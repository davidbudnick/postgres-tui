package service

import (
	"time"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

// ConfigService defines configuration management.
type ConfigService interface {
	ListConnections() ([]types.Connection, error)
	AddConnection(conn types.Connection) (types.Connection, error)
	UpdateConnection(conn types.Connection) (types.Connection, error)
	DeleteConnection(id int64) error

	AddFavorite(f types.Favorite) (types.Favorite, error)
	RemoveFavorite(connID int64, database, schema, object string) error
	ListFavorites(connID int64) []types.Favorite
	IsFavorite(connID int64, database, schema, object string) bool

	AddRecentObject(r types.RecentObject)
	ListRecentObjects(connID int64) []types.RecentObject

	ListSavedQueries() []types.SavedQuery
	AddSavedQuery(q types.SavedQuery) (types.SavedQuery, error)
	DeleteSavedQuery(name string) error

	GetKeyBindings() types.KeyBindings
	SetKeyBindings(kb types.KeyBindings) error
	ResetKeyBindings() error
	GetPageSize() int

	Close() error
}

// PGService defines PostgreSQL operations.
type PGService interface {
	Connect(conn types.Connection) error
	Disconnect() error
	TestConnection(conn types.Connection) (time.Duration, types.ServerInfo, error)
	IsConnected() bool
	IsReadOnly() bool
	CurrentDatabase() string
	SwitchDatabase(name string) error

	GetServerInfo() (types.ServerInfo, error)
	ListDatabases() ([]types.DatabaseInfo, error)
	ListSchemas() ([]types.SchemaInfo, error)
	ListObjects(schema string, kind types.ObjectKind) ([]types.SchemaObject, error)
	GetTableDetail(schema, name string) (types.TableDetail, error)
	GetObjectDetail(schema, name string, kind types.ObjectKind) (types.TableDetail, error)
	GetTableData(schema, name string, offset, limit int) (types.QueryResult, error)
	RunQuery(sql string, limit int) (types.QueryResult, error)
	ListActivity() ([]types.ActivityRow, error)
	ListERD(schema string) (types.ERDGraph, error)
	ListExtensions() ([]types.ExtensionInfo, error)
	CancelQuery(pid int) error
}
