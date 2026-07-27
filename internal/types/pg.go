package types

import "time"

// ServerInfo is basic PostgreSQL server metadata.
type ServerInfo struct {
	Version     string
	VersionNum  int
	User        string
	Database    string
	Host        string
	Port        int
	Encoding    string
	Timezone    string
	Uptime      string
	StartTime   time.Time
	MaxConns    int
	ActiveConns int
}

// DatabaseInfo describes a database on the server.
type DatabaseInfo struct {
	Name       string
	Owner      string
	Encoding   string
	Collate    string
	SizeBytes  int64
	SizePretty string
	ConnLimit  int
	AllowConn  bool
}

// SchemaInfo describes a schema.
type SchemaInfo struct {
	Name       string
	Owner      string
	TableCount int
	ViewCount  int
}

// ObjectKind classifies a schema object.
type ObjectKind string

const (
	ObjectTable     ObjectKind = "table"
	ObjectView      ObjectKind = "view"
	ObjectMatView   ObjectKind = "matview"
	ObjectSequence  ObjectKind = "sequence"
	ObjectFunction  ObjectKind = "function"
	ObjectType      ObjectKind = "type"
	ObjectExtension ObjectKind = "extension"
)

// SchemaObject is a browsable database object.
type SchemaObject struct {
	Schema      string
	Name        string
	Kind        ObjectKind
	Owner       string
	RowEstimate int64
	SizePretty  string
	SizeBytes   int64
	Comment     string
}

// FullName returns schema.name.
func (o SchemaObject) FullName() string {
	if o.Schema == "" {
		return o.Name
	}
	return o.Schema + "." + o.Name
}

// ColumnInfo describes a table/view column.
type ColumnInfo struct {
	Name         string
	DataType     string
	UDTName      string
	IsNullable   bool
	Default      string
	IsPrimaryKey bool
	Comment      string
	Position     int
}

// IndexInfo describes a table index.
type IndexInfo struct {
	Name       string
	Definition string
	IsUnique   bool
	IsPrimary  bool
	Columns    string
	SizePretty string
}

// ConstraintInfo describes a table constraint.
type ConstraintInfo struct {
	Name       string
	Type       string
	Definition string
}

// DetailProp is a labeled fact on object detail (sequence/function/type/extension).
type DetailProp struct {
	Label string
	Value string
}

// TableDetail aggregates metadata for a table/view or other schema object.
type TableDetail struct {
	Object      SchemaObject
	Columns     []ColumnInfo
	Indexes     []IndexInfo
	Constraints []ConstraintInfo
	Triggers    []string
	CreateSQL   string // view/function definition SQL
	Props       []DetailProp
}

// QueryResult is the outcome of a SQL statement.
type QueryResult struct {
	Columns      []string
	Rows         [][]string
	RowsAffected int64
	Duration     time.Duration
	Truncated    bool
	IsSelect     bool
	SQL          string
}

// ActivityRow is one pg_stat_activity entry.
type ActivityRow struct {
	PID             int
	User            string
	Database        string
	State           string
	Query           string
	WaitEvent       string
	WaitEventType   string
	BackendStart    time.Time
	QueryStart      time.Time
	ApplicationName string
	ClientAddr      string
	Duration        time.Duration
	BackendType     string
}

// FKEdge is a foreign-key relationship between two tables.
type FKEdge struct {
	Name      string
	FromTable string
	FromCols  []string
	ToTable   string
	ToCols    []string
}

// ERDTable is a table node in an ER diagram.
type ERDTable struct {
	Name    string
	Columns []string
}

// ERDGraph is schema-scoped tables and FK edges for the ER diagram.
type ERDGraph struct {
	Schema string
	Tables []ERDTable
	Edges  []FKEdge
}

// ExtensionInfo is an installed extension.
type ExtensionInfo struct {
	Name    string
	Version string
	Schema  string
}
