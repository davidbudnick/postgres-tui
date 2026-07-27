package testutil

import (
	"fmt"
	"time"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

// MockPG is an in-memory PGService for tests.
type MockPG struct {
	Connected bool
	ReadOnly  bool
	Database  string
	FailNext  string
	Info      types.ServerInfo
	Databases []types.DatabaseInfo
	Schemas   []types.SchemaInfo
	Objects   []types.SchemaObject
	Detail    types.TableDetail
	Data      types.QueryResult
	Activity  []types.ActivityRow
	ERD       types.ERDGraph
	Exts      []types.ExtensionInfo
}

// NewMockPG returns a mock with demo data.
func NewMockPG() *MockPG {
	return &MockPG{
		Database: "postgres",
		Info: types.ServerInfo{
			Version: "PostgreSQL 16.4", VersionNum: 160004, User: "postgres",
			Database: "postgres", Host: "localhost", Port: 5432, Encoding: "UTF8",
			Timezone: "UTC", Uptime: "1h0m0s", MaxConns: 100, ActiveConns: 3,
		},
		Databases: []types.DatabaseInfo{
			{Name: "postgres", Owner: "postgres", Encoding: "UTF8", SizePretty: "8 MB", AllowConn: true},
			{Name: "app", Owner: "postgres", Encoding: "UTF8", SizePretty: "42 MB", AllowConn: true},
			{Name: "analytics", Owner: "postgres", Encoding: "UTF8", SizePretty: "120 MB", AllowConn: true},
		},
		Schemas: []types.SchemaInfo{
			{Name: "public", Owner: "postgres", TableCount: 3, ViewCount: 1},
			{Name: "billing", Owner: "postgres", TableCount: 2, ViewCount: 0},
		},
		Objects: []types.SchemaObject{
			{Schema: "public", Name: "users", Kind: types.ObjectTable, Owner: "postgres", RowEstimate: 1250, SizePretty: "256 kB"},
			{Schema: "public", Name: "orders", Kind: types.ObjectTable, Owner: "postgres", RowEstimate: 8900, SizePretty: "1.2 MB"},
			{Schema: "public", Name: "products", Kind: types.ObjectTable, Owner: "postgres", RowEstimate: 340, SizePretty: "96 kB"},
			{Schema: "public", Name: "active_users", Kind: types.ObjectView, Owner: "postgres"},
		},
		Detail: types.TableDetail{
			Object: types.SchemaObject{Schema: "public", Name: "users", Kind: types.ObjectTable},
			Columns: []types.ColumnInfo{
				{Name: "id", DataType: "integer", IsPrimaryKey: true, IsNullable: false, Position: 1},
				{Name: "email", DataType: "text", IsNullable: false, Position: 2},
				{Name: "name", DataType: "text", IsNullable: true, Position: 3},
				{Name: "created_at", DataType: "timestamp with time zone", IsNullable: false, Default: "now()", Position: 4},
			},
			Indexes: []types.IndexInfo{
				{Name: "users_pkey", IsPrimary: true, IsUnique: true, Definition: "CREATE UNIQUE INDEX users_pkey ON public.users USING btree (id)", SizePretty: "16 kB"},
				{Name: "users_email_key", IsUnique: true, Definition: "CREATE UNIQUE INDEX users_email_key ON public.users USING btree (email)", SizePretty: "16 kB"},
			},
			Constraints: []types.ConstraintInfo{
				{Name: "users_pkey", Type: "PRIMARY KEY", Definition: "PRIMARY KEY (id)"},
				{Name: "users_email_key", Type: "UNIQUE", Definition: "UNIQUE (email)"},
			},
		},
		Data: types.QueryResult{
			Columns: []string{"id", "email", "name", "created_at"},
			Rows: [][]string{
				{"1", "alice@example.com", "Alice", "2024-01-01T10:00:00Z"},
				{"2", "bob@example.com", "Bob", "2024-01-02T11:30:00Z"},
				{"3", "carol@example.com", "Carol", "2024-01-03T09:15:00Z"},
			},
			RowsAffected: 3,
			Duration:     2 * time.Millisecond,
			IsSelect:     true,
		},
		Activity: []types.ActivityRow{
			{PID: 101, User: "postgres", Database: "app", State: "active", Query: "SELECT * FROM users", Duration: 50 * time.Millisecond, BackendType: "client backend"},
			{PID: 102, User: "app", Database: "app", State: "idle", Query: "COMMIT", Duration: time.Second, BackendType: "client backend"},
		},
		ERD: types.ERDGraph{
			Schema: "public",
			Tables: []types.ERDTable{
				{Name: "users", Columns: []string{"id", "email", "name", "created_at"}},
				{Name: "orders", Columns: []string{"id", "user_id", "total_cents", "status", "created_at"}},
				{Name: "products", Columns: []string{"id", "sku", "name", "price_cents", "category_id"}},
				{Name: "order_items", Columns: []string{"id", "order_id", "product_id", "quantity", "unit_price_cents"}},
				{Name: "product_categories", Columns: []string{"id", "slug", "name"}},
			},
			Edges: []types.FKEdge{
				{Name: "orders_user_id_fkey", FromTable: "orders", FromCols: []string{"user_id"}, ToTable: "users", ToCols: []string{"id"}},
				{Name: "products_category_id_fkey", FromTable: "products", FromCols: []string{"category_id"}, ToTable: "product_categories", ToCols: []string{"id"}},
				{Name: "order_items_order_id_fkey", FromTable: "order_items", FromCols: []string{"order_id"}, ToTable: "orders", ToCols: []string{"id"}},
				{Name: "order_items_product_id_fkey", FromTable: "order_items", FromCols: []string{"product_id"}, ToTable: "products", ToCols: []string{"id"}},
			},
		},
		Exts: []types.ExtensionInfo{{Name: "plpgsql", Version: "1.0", Schema: "pg_catalog"}},
	}
}

func (m *MockPG) fail(op string) error {
	if m.FailNext == op || m.FailNext == "*" {
		m.FailNext = ""
		return fmt.Errorf("mock fail: %s", op)
	}
	return nil
}

// Connect implements PGService.
func (m *MockPG) Connect(conn types.Connection) error {
	if err := m.fail("connect"); err != nil {
		return err
	}
	m.Connected = true
	m.ReadOnly = conn.ReadOnly
	if conn.Database != "" {
		m.Database = conn.Database
	} else {
		m.Database = "postgres"
	}
	m.Info.Database = m.Database
	m.Info.Host = conn.Host
	m.Info.Port = conn.Port
	if m.Info.Port == 0 {
		m.Info.Port = 5432
	}
	return nil
}

// Disconnect implements PGService.
func (m *MockPG) Disconnect() error {
	m.Connected = false
	return nil
}

// TestConnection implements PGService.
func (m *MockPG) TestConnection(conn types.Connection) (time.Duration, types.ServerInfo, error) {
	if err := m.fail("test"); err != nil {
		return 0, types.ServerInfo{}, err
	}
	_ = conn
	return 5 * time.Millisecond, m.Info, nil
}

// IsConnected implements PGService.
func (m *MockPG) IsConnected() bool { return m.Connected }

// IsReadOnly implements PGService.
func (m *MockPG) IsReadOnly() bool { return m.ReadOnly }

// CurrentDatabase implements PGService.
func (m *MockPG) CurrentDatabase() string { return m.Database }

// SwitchDatabase implements PGService.
func (m *MockPG) SwitchDatabase(name string) error {
	if err := m.fail("switch"); err != nil {
		return err
	}
	m.Database = name
	m.Info.Database = name
	return nil
}

// GetServerInfo implements PGService.
func (m *MockPG) GetServerInfo() (types.ServerInfo, error) {
	if err := m.fail("info"); err != nil {
		return types.ServerInfo{}, err
	}
	return m.Info, nil
}

// ListDatabases implements PGService.
func (m *MockPG) ListDatabases() ([]types.DatabaseInfo, error) {
	if err := m.fail("databases"); err != nil {
		return nil, err
	}
	return m.Databases, nil
}

// ListSchemas implements PGService.
func (m *MockPG) ListSchemas() ([]types.SchemaInfo, error) {
	if err := m.fail("schemas"); err != nil {
		return nil, err
	}
	return m.Schemas, nil
}

// ListObjects implements PGService.
func (m *MockPG) ListObjects(schema string, kind types.ObjectKind) ([]types.SchemaObject, error) {
	if err := m.fail("objects"); err != nil {
		return nil, err
	}
	var out []types.SchemaObject
	for _, o := range m.Objects {
		if schema != "" && o.Schema != schema {
			continue
		}
		if kind != "" && o.Kind != kind {
			// allow listing all tables+views when kind empty handled by caller
			if kind == types.ObjectTable && o.Kind != types.ObjectTable {
				continue
			}
			if kind != types.ObjectTable && o.Kind != kind {
				continue
			}
		}
		out = append(out, o)
	}
	return out, nil
}

// GetTableDetail implements PGService.
func (m *MockPG) GetTableDetail(schema, name string) (types.TableDetail, error) {
	if err := m.fail("detail"); err != nil {
		return types.TableDetail{}, err
	}
	d := m.Detail
	d.Object.Schema = schema
	d.Object.Name = name
	return d, nil
}

// GetObjectDetail implements PGService.
func (m *MockPG) GetObjectDetail(schema, name string, kind types.ObjectKind) (types.TableDetail, error) {
	if err := m.fail("object_detail"); err != nil {
		return types.TableDetail{}, err
	}
	d := m.Detail
	d.Object.Schema = schema
	d.Object.Name = name
	if kind != "" {
		d.Object.Kind = kind
	}
	if len(d.Props) == 0 && kind != "" && kind != types.ObjectTable && kind != types.ObjectView {
		d.Props = []types.DetailProp{{Label: "kind", Value: string(kind)}}
	}
	return d, nil
}

// GetTableData implements PGService.
func (m *MockPG) GetTableData(schema, name string, offset, limit int) (types.QueryResult, error) {
	if err := m.fail("data"); err != nil {
		return types.QueryResult{}, err
	}
	_ = schema
	_ = name
	_ = offset
	_ = limit
	return m.Data, nil
}

// RunQuery implements PGService.
func (m *MockPG) RunQuery(sql string, limit int) (types.QueryResult, error) {
	if err := m.fail("query"); err != nil {
		return types.QueryResult{}, err
	}
	r := m.Data
	r.SQL = sql
	_ = limit
	return r, nil
}

// ListActivity implements PGService.
func (m *MockPG) ListActivity() ([]types.ActivityRow, error) {
	if err := m.fail("activity"); err != nil {
		return nil, err
	}
	return m.Activity, nil
}

// ListERD implements PGService.
func (m *MockPG) ListERD(schema string) (types.ERDGraph, error) {
	if err := m.fail("erd"); err != nil {
		return types.ERDGraph{}, err
	}
	g := m.ERD
	if schema != "" {
		g.Schema = schema
	}
	return g, nil
}

// ListExtensions implements PGService.
func (m *MockPG) ListExtensions() ([]types.ExtensionInfo, error) {
	if err := m.fail("ext"); err != nil {
		return nil, err
	}
	return m.Exts, nil
}

// CancelQuery implements PGService.
func (m *MockPG) CancelQuery(pid int) error {
	if err := m.fail("cancel"); err != nil {
		return err
	}
	_ = pid
	return nil
}
