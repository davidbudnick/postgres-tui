package pg

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/davidbudnick/postgres-tui/internal/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultTimeout = 30 * time.Second
const maxCellBytes = 4096
const maxQueryRows = 5000

// testConfigurePool mutates pool config after ParseConfig (tests only).
var testConfigurePool func(*pgxpool.Config)

type scannable interface {
	Scan(dest ...any) error
}

var scanRow = func(s scannable, dest ...any) error { return s.Scan(dest...) }
var rowValues = func(r interface{ Values() ([]any, error) }) ([]any, error) { return r.Values() }
var rowsErr = func(r interface{ Err() error }) error { return r.Err() }

// Client is a PostgreSQL client backed by pgx.
type Client struct {
	mu       sync.RWMutex
	pool     *pgxpool.Pool
	conn     types.Connection
	readOnly bool
	database string
}

// NewClient creates a disconnected client.
func NewClient() *Client {
	return &Client{}
}

// Connect opens a connection pool to PostgreSQL.
func (c *Client) Connect(conn types.Connection) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pool != nil {
		c.pool.Close()
		c.pool = nil
	}

	if conn.Port == 0 {
		conn.Port = 5432
	}
	if conn.Database == "" {
		conn.Database = "postgres"
	}
	if conn.SSLMode == "" {
		conn.SSLMode = types.SSLModePrefer
	}
	if conn.Username == "" {
		conn.Username = "postgres"
	}

	cfg, err := pgxpool.ParseConfig(conn.DSN())
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 4
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	if testConfigurePool != nil {
		testConfigurePool(cfg)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("ping: %w", err)
	}

	c.pool = pool
	c.conn = conn
	c.readOnly = conn.ReadOnly
	c.database = conn.Database
	return nil
}

// Disconnect closes the pool.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.pool != nil {
		c.pool.Close()
		c.pool = nil
	}
	c.database = ""
	return nil
}

// TestConnection opens a short-lived connection and measures latency.
func (c *Client) TestConnection(conn types.Connection) (time.Duration, types.ServerInfo, error) {
	tmp := NewClient()
	start := time.Now()
	if err := tmp.Connect(conn); err != nil {
		return 0, types.ServerInfo{}, err
	}
	latency := time.Since(start)
	info, err := tmp.GetServerInfo()
	_ = tmp.Disconnect()
	return latency, info, err
}

// IsConnected reports whether a pool is open.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pool != nil
}

// IsReadOnly reports read-only mode.
func (c *Client) IsReadOnly() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.readOnly
}

// CurrentDatabase returns the active database name.
func (c *Client) CurrentDatabase() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.database
}

// SwitchDatabase reconnects using the same credentials on a different database.
func (c *Client) SwitchDatabase(name string) error {
	c.mu.RLock()
	conn := c.conn
	cur := c.database
	c.mu.RUnlock()
	if name != "" && name == cur {
		return nil
	}
	conn.Database = name
	return c.Connect(conn)
}

func (c *Client) poolOrErr() (*pgxpool.Pool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.pool == nil {
		return nil, fmt.Errorf("not connected")
	}
	return c.pool, nil
}

// GetServerInfo loads basic server metadata.
func (c *Client) GetServerInfo() (types.ServerInfo, error) {
	pool, err := c.poolOrErr()
	if err != nil {
		return types.ServerInfo{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	info := types.ServerInfo{
		Host: c.conn.Host,
		Port: c.conn.Port,
	}
	row := pool.QueryRow(ctx, `
SELECT version(),
       current_user,
       current_database(),
       current_setting('server_encoding'),
       current_setting('TimeZone'),
       pg_postmaster_start_time(),
       current_setting('max_connections')::int,
       (SELECT count(*) FROM pg_stat_activity)`)
	var start time.Time
	var maxConns, active int
	if err := row.Scan(&info.Version, &info.User, &info.Database, &info.Encoding, &info.Timezone, &start, &maxConns, &active); err != nil {
		return types.ServerInfo{}, fmt.Errorf("server info: %w", err)
	}
	info.StartTime = start
	info.Uptime = time.Since(start).Truncate(time.Second).String()
	info.MaxConns = maxConns
	info.ActiveConns = active

	_ = pool.QueryRow(ctx, `SHOW server_version_num`).Scan(&info.VersionNum)
	return info, nil
}

// ListDatabases lists connectable databases.
func (c *Client) ListDatabases() ([]types.DatabaseInfo, error) {
	pool, err := c.poolOrErr()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	rows, err := pool.Query(ctx, `
SELECT d.datname,
       pg_catalog.pg_get_userbyid(d.datdba),
       pg_catalog.pg_encoding_to_char(d.encoding),
       d.datcollate,
       COALESCE(pg_database_size(d.datname), 0),
       pg_size_pretty(COALESCE(pg_database_size(d.datname), 0)),
       d.datconnlimit,
       d.datallowconn
FROM pg_database d
WHERE d.datistemplate = false
ORDER BY d.datname`)
	if err != nil {
		return nil, fmt.Errorf("list databases: %w", err)
	}
	defer rows.Close()

	var out []types.DatabaseInfo
	for rows.Next() {
		var db types.DatabaseInfo
		if err := scanRow(rows, &db.Name, &db.Owner, &db.Encoding, &db.Collate, &db.SizeBytes, &db.SizePretty, &db.ConnLimit, &db.AllowConn); err != nil {
			return nil, err
		}
		out = append(out, db)
	}
	return out, rowsErr(rows)
}

// ListSchemas lists non-system schemas.
func (c *Client) ListSchemas() ([]types.SchemaInfo, error) {
	pool, err := c.poolOrErr()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	rows, err := pool.Query(ctx, `
SELECT n.nspname,
       pg_catalog.pg_get_userbyid(n.nspowner),
       (SELECT count(*) FROM pg_class c WHERE c.relnamespace = n.oid AND c.relkind = 'r'),
       (SELECT count(*) FROM pg_class c WHERE c.relnamespace = n.oid AND c.relkind IN ('v','m'))
FROM pg_namespace n
WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
  AND n.nspname NOT LIKE 'pg_temp_%'
  AND n.nspname NOT LIKE 'pg_toast_temp_%'
ORDER BY n.nspname`)
	if err != nil {
		return nil, fmt.Errorf("list schemas: %w", err)
	}
	defer rows.Close()

	var out []types.SchemaInfo
	for rows.Next() {
		var s types.SchemaInfo
		if err := scanRow(rows, &s.Name, &s.Owner, &s.TableCount, &s.ViewCount); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rowsErr(rows)
}

// ListObjects lists objects of a given kind in a schema (empty schema = all user schemas).
func (c *Client) ListObjects(schema string, kind types.ObjectKind) ([]types.SchemaObject, error) {
	pool, err := c.poolOrErr()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	switch kind {
	case types.ObjectTable, types.ObjectView, types.ObjectMatView, "":
		return c.listRelations(ctx, pool, schema, kind)
	case types.ObjectSequence:
		return c.listSequences(ctx, pool, schema)
	case types.ObjectFunction:
		return c.listFunctions(ctx, pool, schema)
	case types.ObjectType:
		return c.listTypes(ctx, pool, schema)
	case types.ObjectExtension:
		return c.listExtensionsAsObjects(ctx, pool)
	default:
		return c.listRelations(ctx, pool, schema, types.ObjectTable)
	}
}

func (c *Client) listRelations(ctx context.Context, pool *pgxpool.Pool, schema string, kind types.ObjectKind) ([]types.SchemaObject, error) {
	relkinds := []string{"r", "p"}
	switch kind {
	case types.ObjectView:
		relkinds = []string{"v"}
	case types.ObjectMatView:
		relkinds = []string{"m"}
	case "":
		relkinds = []string{"r", "p", "v", "m"}
	}

	placeholders := make([]string, len(relkinds))
	args := make([]any, 0, len(relkinds)+1)
	for i, rk := range relkinds {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, rk)
	}
	schemaFilter := ""
	if schema != "" {
		args = append(args, schema)
		schemaFilter = fmt.Sprintf(" AND n.nspname = $%d", len(args))
	}

	q := fmt.Sprintf(`
SELECT n.nspname,
       c.relname,
       c.relkind::text,
       pg_catalog.pg_get_userbyid(c.relowner),
       COALESCE(c.reltuples, 0)::bigint,
       pg_size_pretty(pg_total_relation_size(c.oid)),
       pg_total_relation_size(c.oid),
       COALESCE(obj_description(c.oid, 'pg_class'), '')
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN (%s)
  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
  %s
ORDER BY n.nspname, c.relname`, strings.Join(placeholders, ","), schemaFilter)

	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list relations: %w", err)
	}
	defer rows.Close()

	var out []types.SchemaObject
	for rows.Next() {
		var o types.SchemaObject
		var relkind string
		if err := scanRow(rows, &o.Schema, &o.Name, &relkind, &o.Owner, &o.RowEstimate, &o.SizePretty, &o.SizeBytes, &o.Comment); err != nil {
			return nil, err
		}
		switch relkind {
		case "v":
			o.Kind = types.ObjectView
		case "m":
			o.Kind = types.ObjectMatView
		default:
			o.Kind = types.ObjectTable
		}
		out = append(out, o)
	}
	return out, rowsErr(rows)
}

func (c *Client) listSequences(ctx context.Context, pool *pgxpool.Pool, schema string) ([]types.SchemaObject, error) {
	args := []any{}
	schemaFilter := ""
	if schema != "" {
		args = append(args, schema)
		schemaFilter = " AND n.nspname = $1"
	}
	q := `
SELECT n.nspname, c.relname, pg_catalog.pg_get_userbyid(c.relowner),
       pg_size_pretty(pg_total_relation_size(c.oid)), pg_total_relation_size(c.oid)
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind = 'S'
  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')` + schemaFilter + `
ORDER BY n.nspname, c.relname`
	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.SchemaObject
	for rows.Next() {
		var o types.SchemaObject
		o.Kind = types.ObjectSequence
		if err := scanRow(rows, &o.Schema, &o.Name, &o.Owner, &o.SizePretty, &o.SizeBytes); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rowsErr(rows)
}

func (c *Client) listFunctions(ctx context.Context, pool *pgxpool.Pool, schema string) ([]types.SchemaObject, error) {
	args := []any{}
	schemaFilter := ""
	if schema != "" {
		args = append(args, schema)
		schemaFilter = " AND n.nspname = $1"
	}
	q := `
SELECT n.nspname, p.proname, pg_catalog.pg_get_userbyid(p.proowner),
       pg_catalog.pg_get_function_identity_arguments(p.oid)
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')` + schemaFilter + `
ORDER BY n.nspname, p.proname
LIMIT 500`
	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.SchemaObject
	for rows.Next() {
		var o types.SchemaObject
		var argsStr string
		o.Kind = types.ObjectFunction
		if err := scanRow(rows, &o.Schema, &o.Name, &o.Owner, &argsStr); err != nil {
			return nil, err
		}
		if argsStr != "" {
			o.Comment = argsStr
		}
		out = append(out, o)
	}
	return out, rowsErr(rows)
}

func (c *Client) listTypes(ctx context.Context, pool *pgxpool.Pool, schema string) ([]types.SchemaObject, error) {
	args := []any{}
	schemaFilter := ""
	if schema != "" {
		args = append(args, schema)
		schemaFilter = " AND n.nspname = $1"
	}
	q := `
SELECT n.nspname, t.typname, pg_catalog.pg_get_userbyid(t.typowner)
FROM pg_type t
JOIN pg_namespace n ON n.oid = t.typnamespace
WHERE t.typtype IN ('e', 'c', 'd')
  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')` + schemaFilter + `
ORDER BY n.nspname, t.typname`
	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.SchemaObject
	for rows.Next() {
		var o types.SchemaObject
		o.Kind = types.ObjectType
		if err := scanRow(rows, &o.Schema, &o.Name, &o.Owner); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rowsErr(rows)
}

func (c *Client) listExtensionsAsObjects(ctx context.Context, pool *pgxpool.Pool) ([]types.SchemaObject, error) {
	rows, err := pool.Query(ctx, `SELECT e.extname, e.extversion, n.nspname
FROM pg_extension e
JOIN pg_namespace n ON n.oid = e.extnamespace
ORDER BY e.extname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.SchemaObject
	for rows.Next() {
		var o types.SchemaObject
		var ver string
		o.Kind = types.ObjectExtension
		if err := scanRow(rows, &o.Name, &ver, &o.Schema); err != nil {
			return nil, err
		}
		o.Comment = ver
		out = append(out, o)
	}
	return out, rowsErr(rows)
}

// GetTableDetail loads columns, indexes, and constraints for a table/view.
func (c *Client) GetTableDetail(schema, name string) (types.TableDetail, error) {
	if schema == "" || name == "" {
		return types.TableDetail{}, fmt.Errorf("schema and name are required")
	}
	pool, err := c.poolOrErr()
	if err != nil {
		return types.TableDetail{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	var relkind string
	err = pool.QueryRow(ctx, `
SELECT c.relkind::text
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1 AND c.relname = $2
  AND c.relkind IN ('r', 'p', 'v', 'm', 'f')`, schema, name).Scan(&relkind)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.TableDetail{}, fmt.Errorf("relation %s.%s not found", schema, name)
		}
		return types.TableDetail{}, fmt.Errorf("lookup relation: %w", err)
	}

	kind := types.ObjectTable
	switch relkind {
	case "v":
		kind = types.ObjectView
	case "m":
		kind = types.ObjectMatView
	}

	detail := types.TableDetail{
		Object: types.SchemaObject{Schema: schema, Name: name, Kind: kind},
	}

	colRows, err := pool.Query(ctx, `
SELECT a.attname,
       pg_catalog.format_type(a.atttypid, a.atttypmod),
       t.typname,
       NOT a.attnotnull,
       COALESCE(pg_get_expr(ad.adbin, ad.adrelid), ''),
       COALESCE(col_description(a.attrelid, a.attnum), ''),
       a.attnum,
       EXISTS (
         SELECT 1 FROM pg_index i
         WHERE i.indrelid = a.attrelid AND a.attnum = ANY(i.indkey) AND i.indisprimary
       )
FROM pg_attribute a
JOIN pg_class c ON c.oid = a.attrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_type t ON t.oid = a.atttypid
LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
WHERE n.nspname = $1 AND c.relname = $2
  AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
  AND a.attnum > 0 AND NOT a.attisdropped
ORDER BY a.attnum`, schema, name)
	if err != nil {
		return types.TableDetail{}, fmt.Errorf("columns: %w", err)
	}
	defer colRows.Close()
	for colRows.Next() {
		var col types.ColumnInfo
		if err := scanRow(colRows, &col.Name, &col.DataType, &col.UDTName, &col.IsNullable, &col.Default, &col.Comment, &col.Position, &col.IsPrimaryKey); err != nil {
			return types.TableDetail{}, err
		}
		detail.Columns = append(detail.Columns, col)
	}
	if err := rowsErr(colRows); err != nil {
		return types.TableDetail{}, err
	}

	idxRows, err := pool.Query(ctx, `
SELECT i.relname,
       pg_get_indexdef(ix.indexrelid),
       ix.indisunique,
       ix.indisprimary,
       pg_size_pretty(pg_relation_size(i.oid))
FROM pg_index ix
JOIN pg_class t ON t.oid = ix.indrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
JOIN pg_class i ON i.oid = ix.indexrelid
WHERE n.nspname = $1 AND t.relname = $2
  AND t.relkind IN ('r', 'p', 'v', 'm', 'f')
ORDER BY i.relname`, schema, name)
	if err != nil {
		return types.TableDetail{}, fmt.Errorf("indexes: %w", err)
	}
	defer idxRows.Close()
	for idxRows.Next() {
		var idx types.IndexInfo
		if err := scanRow(idxRows, &idx.Name, &idx.Definition, &idx.IsUnique, &idx.IsPrimary, &idx.SizePretty); err != nil {
			return types.TableDetail{}, err
		}
		detail.Indexes = append(detail.Indexes, idx)
	}
	if err := rowsErr(idxRows); err != nil {
		return types.TableDetail{}, err
	}

	conRows, err := pool.Query(ctx, `
SELECT con.conname,
       con.contype::text,
       pg_get_constraintdef(con.oid)
FROM pg_constraint con
JOIN pg_class t ON t.oid = con.conrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
WHERE n.nspname = $1 AND t.relname = $2
  AND t.relkind IN ('r', 'p', 'v', 'm', 'f')
ORDER BY con.conname`, schema, name)
	if err != nil {
		return types.TableDetail{}, fmt.Errorf("constraints: %w", err)
	}
	defer conRows.Close()
	for conRows.Next() {
		var ct types.ConstraintInfo
		var code string
		if err := scanRow(conRows, &ct.Name, &code, &ct.Definition); err != nil {
			return types.TableDetail{}, err
		}
		ct.Type = constraintTypeName(code)
		detail.Constraints = append(detail.Constraints, ct)
	}
	if err := rowsErr(conRows); err != nil {
		return types.TableDetail{}, err
	}

	if kind == types.ObjectView || kind == types.ObjectMatView {
		var def string
		err = pool.QueryRow(ctx, `
SELECT pg_get_viewdef(c.oid, true)
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1 AND c.relname = $2
  AND c.relkind IN ('v', 'm')`, schema, name).Scan(&def)
		if err == nil {
			detail.CreateSQL = strings.TrimSpace(def)
		}
	}

	return detail, nil
}

// GetObjectDetail loads kind-specific metadata (sequence/function/type/extension or relation).
func (c *Client) GetObjectDetail(schema, name string, kind types.ObjectKind) (types.TableDetail, error) {
	switch kind {
	case types.ObjectTable, types.ObjectView, types.ObjectMatView, "":
		return c.GetTableDetail(schema, name)
	case types.ObjectSequence:
		return c.getSequenceDetail(schema, name)
	case types.ObjectFunction:
		return c.getFunctionDetail(schema, name)
	case types.ObjectType:
		return c.getTypeDetail(schema, name)
	case types.ObjectExtension:
		return c.getExtensionDetail(schema, name)
	default:
		return c.GetTableDetail(schema, name)
	}
}

func (c *Client) getSequenceDetail(schema, name string) (types.TableDetail, error) {
	pool, err := c.poolOrErr()
	if err != nil {
		return types.TableDetail{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	detail := types.TableDetail{
		Object: types.SchemaObject{Schema: schema, Name: name, Kind: types.ObjectSequence},
	}
	var lastVal, startVal, minVal, maxVal, inc *int64
	var cache *int64
	var cycle, called bool
	var dataType string
	err = pool.QueryRow(ctx, `
SELECT last_value, start_value, min_value, max_value, increment_by,
       cache_size, cycle, data_type
FROM pg_sequences
WHERE schemaname = $1 AND sequencename = $2`, schema, name).Scan(
		&lastVal, &startVal, &minVal, &maxVal, &inc, &cache, &cycle, &dataType,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.TableDetail{}, fmt.Errorf("sequence %s.%s not found", schema, name)
		}
		return types.TableDetail{}, fmt.Errorf("sequence: %w", err)
	}
	_ = called
	addProp := func(k string, v any) {
		switch t := v.(type) {
		case *int64:
			if t != nil {
				detail.Props = append(detail.Props, types.DetailProp{Label: k, Value: fmt.Sprintf("%d", *t)})
			}
		case string:
			if t != "" {
				detail.Props = append(detail.Props, types.DetailProp{Label: k, Value: t})
			}
		case bool:
			detail.Props = append(detail.Props, types.DetailProp{Label: k, Value: fmt.Sprintf("%v", t)})
		}
	}
	addProp("data type", dataType)
	addProp("last value", lastVal)
	addProp("start", startVal)
	addProp("min", minVal)
	addProp("max", maxVal)
	addProp("increment", inc)
	addProp("cache", cache)
	addProp("cycle", cycle)
	detail.CreateSQL = fmt.Sprintf("SELECT nextval('%s.%s');", quoteIdent(schema), quoteIdent(name))
	return detail, nil
}

func (c *Client) getFunctionDetail(schema, name string) (types.TableDetail, error) {
	pool, err := c.poolOrErr()
	if err != nil {
		return types.TableDetail{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	detail := types.TableDetail{
		Object: types.SchemaObject{Schema: schema, Name: name, Kind: types.ObjectFunction},
	}
	var oid uint32
	var lang, ret, identity string
	err = pool.QueryRow(ctx, `
SELECT p.oid,
       l.lanname,
       pg_get_function_result(p.oid),
       pg_get_function_identity_arguments(p.oid)
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
JOIN pg_language l ON l.oid = p.prolang
WHERE n.nspname = $1 AND p.proname = $2
ORDER BY p.oid
LIMIT 1`, schema, name).Scan(&oid, &lang, &ret, &identity)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.TableDetail{}, fmt.Errorf("function %s.%s not found", schema, name)
		}
		return types.TableDetail{}, fmt.Errorf("function: %w", err)
	}
	detail.Props = []types.DetailProp{
		{Label: "language", Value: lang},
		{Label: "returns", Value: ret},
		{Label: "arguments", Value: identity},
	}
	var def string
	if err := pool.QueryRow(ctx, `SELECT pg_get_functiondef($1::oid)`, oid).Scan(&def); err == nil {
		detail.CreateSQL = strings.TrimSpace(def)
	}
	return detail, nil
}

func (c *Client) getTypeDetail(schema, name string) (types.TableDetail, error) {
	pool, err := c.poolOrErr()
	if err != nil {
		return types.TableDetail{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	detail := types.TableDetail{
		Object: types.SchemaObject{Schema: schema, Name: name, Kind: types.ObjectType},
	}
	var typCategory, typType, enumLabels, owner string
	err = pool.QueryRow(ctx, `
SELECT t.typtype::text,
       t.typcategory::text,
       COALESCE(pg_catalog.array_to_string(
         ARRAY(SELECT e.enumlabel FROM pg_enum e WHERE e.enumtypid = t.oid ORDER BY e.enumsortorder),
         ', '), ''),
       pg_get_userbyid(t.typowner)
FROM pg_type t
JOIN pg_namespace n ON n.oid = t.typnamespace
WHERE n.nspname = $1 AND t.typname = $2
  AND t.typtype IN ('c', 'e', 'd', 'r', 'm')`, schema, name).Scan(&typType, &typCategory, &enumLabels, &owner)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.TableDetail{}, fmt.Errorf("type %s.%s not found", schema, name)
		}
		return types.TableDetail{}, fmt.Errorf("type: %w", err)
	}
	typeName := map[string]string{"c": "composite", "e": "enum", "d": "domain", "r": "range", "m": "multirange"}[typType]
	detail.Props = []types.DetailProp{
		{Label: "type", Value: typeName},
		{Label: "category", Value: typCategory},
		{Label: "owner", Value: owner},
	}
	if enumLabels != "" {
		detail.Props = append(detail.Props, types.DetailProp{Label: "labels", Value: enumLabels})
	}
	if typType == "c" {
		rows, qerr := pool.Query(ctx, `
SELECT a.attname,
       pg_catalog.format_type(a.atttypid, a.atttypmod),
       t.typname,
       NOT a.attnotnull,
       '',
       COALESCE(col_description(a.attrelid, a.attnum), ''),
       a.attnum,
       false
FROM pg_type ty
JOIN pg_namespace n ON n.oid = ty.typnamespace
JOIN pg_class c ON c.oid = ty.typrelid
JOIN pg_attribute a ON a.attrelid = c.oid
JOIN pg_type t ON t.oid = a.atttypid
WHERE n.nspname = $1 AND ty.typname = $2
  AND a.attnum > 0 AND NOT a.attisdropped
ORDER BY a.attnum`, schema, name)
		if qerr == nil {
			defer rows.Close()
			for rows.Next() {
				var col types.ColumnInfo
				if err := scanRow(rows, &col.Name, &col.DataType, &col.UDTName, &col.IsNullable, &col.Default, &col.Comment, &col.Position, &col.IsPrimaryKey); err != nil {
					break
				}
				detail.Columns = append(detail.Columns, col)
			}
		}
	}
	return detail, nil
}

func (c *Client) getExtensionDetail(schema, name string) (types.TableDetail, error) {
	pool, err := c.poolOrErr()
	if err != nil {
		return types.TableDetail{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	detail := types.TableDetail{
		Object: types.SchemaObject{Schema: schema, Name: name, Kind: types.ObjectExtension},
	}
	var version, extSchema, relocatable string
	err = pool.QueryRow(ctx, `
SELECT e.extversion,
       n.nspname,
       CASE WHEN e.extrelocatable THEN 'yes' ELSE 'no' END
FROM pg_extension e
JOIN pg_namespace n ON n.oid = e.extnamespace
WHERE e.extname = $1`, name).Scan(&version, &extSchema, &relocatable)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.TableDetail{}, fmt.Errorf("extension %s not found", name)
		}
		return types.TableDetail{}, fmt.Errorf("extension: %w", err)
	}
	detail.Object.Schema = extSchema
	detail.Props = []types.DetailProp{
		{Label: "version", Value: version},
		{Label: "schema", Value: extSchema},
		{Label: "relocatable", Value: relocatable},
	}
	return detail, nil
}

func constraintTypeName(code string) string {
	switch code {
	case "p":
		return "PRIMARY KEY"
	case "f":
		return "FOREIGN KEY"
	case "u":
		return "UNIQUE"
	case "c":
		return "CHECK"
	case "x":
		return "EXCLUDE"
	default:
		return code
	}
}

// GetTableData selects rows from a table with pagination.
func (c *Client) GetTableData(schema, name string, offset, limit int) (types.QueryResult, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	ident := quoteIdent(schema) + "." + quoteIdent(name)
	sql := fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", ident, limit+1, offset)
	return c.RunQuery(sql, limit)
}

// RunQuery executes SQL and returns a tabular result for SELECT-like statements.
func (c *Client) RunQuery(sql string, limit int) (types.QueryResult, error) {
	pool, err := c.poolOrErr()
	if err != nil {
		return types.QueryResult{}, err
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > maxQueryRows {
		limit = maxQueryRows
	}

	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return types.QueryResult{}, fmt.Errorf("empty query")
	}

	if c.IsReadOnly() && isMutatingSQL(trimmed) {
		return types.QueryResult{}, fmt.Errorf("read-only mode: write statements are blocked")
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	start := time.Now()
	result := types.QueryResult{SQL: trimmed, IsSelect: looksLikeSelect(trimmed)}

	if !result.IsSelect {
		tag, err := pool.Exec(ctx, trimmed)
		result.Duration = time.Since(start)
		if err != nil {
			return result, err
		}
		result.RowsAffected = tag.RowsAffected()
		return result, nil
	}

	execSQL := trimmed
	if !hasLimitClause(trimmed) {
		execSQL = fmt.Sprintf("SELECT * FROM (%s) AS _pg_tui_q LIMIT %d", trimmed, limit+1)
	}

	rows, err := pool.Query(ctx, execSQL)
	if err != nil {
		result.Duration = time.Since(start)
		return result, err
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	result.Columns = make([]string, len(fields))
	for i, f := range fields {
		result.Columns[i] = string(f.Name)
	}

	for rows.Next() {
		vals, err := rowValues(rows)
		if err != nil {
			result.Duration = time.Since(start)
			return result, err
		}
		row := make([]string, len(vals))
		for i, v := range vals {
			row[i] = formatCell(v)
		}
		result.Rows = append(result.Rows, row)
		if len(result.Rows) > limit {
			result.Truncated = true
			result.Rows = result.Rows[:limit]
			break
		}
	}
	result.Duration = time.Since(start)
	result.RowsAffected = int64(len(result.Rows))
	return result, rowsErr(rows)
}

// ListActivity returns current client backends (excludes background workers).
func (c *Client) ListActivity() ([]types.ActivityRow, error) {
	pool, err := c.poolOrErr()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	rows, err := pool.Query(ctx, `
SELECT pid,
       COALESCE(usename, ''),
       COALESCE(datname, ''),
       COALESCE(state, ''),
       COALESCE(left(query, 500), ''),
       COALESCE(wait_event, ''),
       COALESCE(wait_event_type, ''),
       backend_start,
       query_start,
       COALESCE(application_name, ''),
       COALESCE(client_addr::text, ''),
       COALESCE(backend_type, '')
FROM pg_stat_activity
WHERE pid <> pg_backend_pid()
  AND COALESCE(backend_type, 'client backend') = 'client backend'
ORDER BY CASE state
           WHEN 'active' THEN 0
           WHEN 'idle in transaction' THEN 1
           WHEN 'idle in transaction (aborted)' THEN 2
           ELSE 3
         END,
         COALESCE(query_start, backend_start) DESC NULLS LAST
LIMIT 200`)
	if err != nil {
		return nil, fmt.Errorf("activity: %w", err)
	}
	defer rows.Close()

	var out []types.ActivityRow
	for rows.Next() {
		var a types.ActivityRow
		var queryStart *time.Time
		if err := scanRow(rows, &a.PID, &a.User, &a.Database, &a.State, &a.Query, &a.WaitEvent, &a.WaitEventType,
			&a.BackendStart, &queryStart, &a.ApplicationName, &a.ClientAddr, &a.BackendType); err != nil {
			return nil, err
		}
		if queryStart != nil {
			a.QueryStart = *queryStart
			a.Duration = time.Since(*queryStart).Truncate(time.Millisecond)
		}
		out = append(out, a)
	}
	return out, rowsErr(rows)
}

// ListERD loads tables and foreign-key edges for a schema ER diagram.
func (c *Client) ListERD(schema string) (types.ERDGraph, error) {
	if schema == "" {
		schema = "public"
	}
	pool, err := c.poolOrErr()
	if err != nil {
		return types.ERDGraph{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	graph := types.ERDGraph{Schema: schema}

	tblRows, err := pool.Query(ctx, `
SELECT c.relname, a.attname
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid
WHERE n.nspname = $1
  AND c.relkind IN ('r', 'p')
  AND a.attnum > 0 AND NOT a.attisdropped
ORDER BY c.relname, a.attnum`, schema)
	if err != nil {
		return types.ERDGraph{}, fmt.Errorf("erd tables: %w", err)
	}
	defer tblRows.Close()

	byName := map[string]*types.ERDTable{}
	var order []string
	for tblRows.Next() {
		var name, col string
		if err := scanRow(tblRows, &name, &col); err != nil {
			return types.ERDGraph{}, err
		}
		t, ok := byName[name]
		if !ok {
			t = &types.ERDTable{Name: name}
			byName[name] = t
			order = append(order, name)
		}
		t.Columns = append(t.Columns, col)
	}
	if err := rowsErr(tblRows); err != nil {
		return types.ERDGraph{}, err
	}
	for _, name := range order {
		graph.Tables = append(graph.Tables, *byName[name])
	}

	edgeRows, err := pool.Query(ctx, `
SELECT con.conname,
       src.relname,
       tgt.relname,
       array_agg(src_att.attname ORDER BY u.ord),
       array_agg(tgt_att.attname ORDER BY u.ord)
FROM pg_constraint con
JOIN pg_class src ON src.oid = con.conrelid
JOIN pg_namespace nsrc ON nsrc.oid = src.relnamespace
JOIN pg_class tgt ON tgt.oid = con.confrelid
JOIN LATERAL unnest(con.conkey, con.confkey) WITH ORDINALITY AS u(src_attnum, tgt_attnum, ord) ON true
JOIN pg_attribute src_att ON src_att.attrelid = con.conrelid AND src_att.attnum = u.src_attnum
JOIN pg_attribute tgt_att ON tgt_att.attrelid = con.confrelid AND tgt_att.attnum = u.tgt_attnum
WHERE con.contype = 'f'
  AND nsrc.nspname = $1
GROUP BY con.conname, src.relname, tgt.relname
ORDER BY src.relname, con.conname`, schema)
	if err != nil {
		return types.ERDGraph{}, fmt.Errorf("erd edges: %w", err)
	}
	defer edgeRows.Close()

	for edgeRows.Next() {
		var e types.FKEdge
		if err := scanRow(edgeRows, &e.Name, &e.FromTable, &e.ToTable, &e.FromCols, &e.ToCols); err != nil {
			return types.ERDGraph{}, err
		}
		graph.Edges = append(graph.Edges, e)
	}
	return graph, rowsErr(edgeRows)
}

// ListExtensions returns installed extensions.
func (c *Client) ListExtensions() ([]types.ExtensionInfo, error) {
	pool, err := c.poolOrErr()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	rows, err := pool.Query(ctx, `
SELECT e.extname, e.extversion, n.nspname
FROM pg_extension e
JOIN pg_namespace n ON n.oid = e.extnamespace
ORDER BY e.extname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []types.ExtensionInfo
	for rows.Next() {
		var e types.ExtensionInfo
		if err := scanRow(rows, &e.Name, &e.Version, &e.Schema); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rowsErr(rows)
}

// CancelQuery cancels a backend by PID.
func (c *Client) CancelQuery(pid int) error {
	if c.IsReadOnly() {
		return fmt.Errorf("read-only mode")
	}
	pool, err := c.poolOrErr()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	_, err = pool.Exec(ctx, `SELECT pg_cancel_backend($1)`, pid)
	return err
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func formatCell(v any) string {
	if v == nil {
		return "NULL"
	}
	switch t := v.(type) {
	case []byte:
		s := string(t)
		if len(s) > maxCellBytes {
			return s[:maxCellBytes] + "…"
		}
		return s
	case time.Time:
		return t.Format(time.RFC3339)
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case string:
		if len(t) > maxCellBytes {
			return t[:maxCellBytes] + "…"
		}
		return t
	default:
		s := fmt.Sprint(t)
		if len(s) > maxCellBytes {
			return s[:maxCellBytes] + "…"
		}
		return s
	}
}

func looksLikeSelect(sql string) bool {
	s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(sql), "("))
	upper := strings.ToUpper(s)
	for _, p := range []string{"SELECT", "WITH", "SHOW", "EXPLAIN", "VALUES", "TABLE"} {
		if strings.HasPrefix(upper, p) {
			return true
		}
	}
	return false
}

func isMutatingSQL(sql string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	for _, p := range []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "CREATE", "TRUNCATE", "GRANT", "REVOKE", "VACUUM", "REINDEX", "CLUSTER", "COMMENT", "COPY", "CALL", "DO ", "SECURITY"} {
		if strings.HasPrefix(upper, p) {
			return true
		}
	}
	return false
}

func hasLimitClause(sql string) bool {
	upper := strings.ToUpper(sql)
	return strings.Contains(upper, " LIMIT ")
}

var _ = pgx.Identifier{}
