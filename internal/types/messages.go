package types

import "time"

// ConnectionsLoadedMsg is emitted after loading saved connections.
type ConnectionsLoadedMsg struct {
	Connections []Connection
	Err         error
}

// ConnectionAddedMsg is emitted after adding a connection.
type ConnectionAddedMsg struct {
	Connection Connection
	Err        error
}

// ConnectionUpdatedMsg is emitted after updating a connection.
type ConnectionUpdatedMsg struct {
	Connection Connection
	Err        error
}

// ConnectionDeletedMsg is emitted after deleting a connection.
type ConnectionDeletedMsg struct {
	ID  int64
	Err error
}

// ConnectedMsg is emitted after connecting to a server.
type ConnectedMsg struct {
	Info ServerInfo
	Err  error
}

// AutoConnectMsg triggers an automatic connection from CLI flags.
type AutoConnectMsg struct {
	Connection Connection
}

// DisconnectedMsg is emitted after disconnect.
type DisconnectedMsg struct{}

// DatabasesLoadedMsg is emitted after listing databases.
type DatabasesLoadedMsg struct {
	Databases []DatabaseInfo
	Err       error
}

// DatabaseSelectedMsg is emitted after switching databases.
type DatabaseSelectedMsg struct {
	Database string
	Info     ServerInfo
	Err      error
}

// SchemasLoadedMsg is emitted after listing schemas.
type SchemasLoadedMsg struct {
	Schemas []SchemaInfo
	Err     error
}

// ObjectsLoadedMsg is emitted after listing schema objects.
type ObjectsLoadedMsg struct {
	Objects []SchemaObject
	Kind    ObjectKind
	Err     error
}

// TableDetailLoadedMsg is emitted after loading table metadata.
type TableDetailLoadedMsg struct {
	Detail TableDetail
	Err    error
}

// TableDataLoadedMsg is emitted after loading table rows.
type TableDataLoadedMsg struct {
	Result QueryResult
	Offset int
	Err    error
}

// QueryResultMsg is emitted after running a query.
type QueryResultMsg struct {
	Result QueryResult
	Err    error
}

// ActivityLoadedMsg is emitted after loading activity.
type ActivityLoadedMsg struct {
	Rows []ActivityRow
	Err  error
}

// ERDLoadedMsg is emitted after loading schema FK relationships.
type ERDLoadedMsg struct {
	Graph ERDGraph
	Err   error
}

// ServerInfoLoadedMsg is emitted after loading server info.
type ServerInfoLoadedMsg struct {
	Info ServerInfo
	Err  error
}

// ConnectionTestMsg is emitted after testing a connection.
type ConnectionTestMsg struct {
	Success bool
	Err     error
	Latency time.Duration
	Info    ServerInfo
}

// FavoritesLoadedMsg is emitted after loading favorites.
type FavoritesLoadedMsg struct {
	Favorites []Favorite
	Err       error
}

// ExportDoneMsg is emitted after export finishes.
type ExportDoneMsg struct {
	Path string
	Rows int
	Err  error
}

// TickMsg is a periodic tick.
type TickMsg struct{}

// StatusMsg is a transient status string.
type StatusMsg struct {
	Text string
}

// EditorSaveMsg is emitted when the multiline editor saves.
type EditorSaveMsg struct {
	Content string
}

// EditorQuitMsg is emitted when the multiline editor quits.
type EditorQuitMsg struct{}
