package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"
	"github.com/davidbudnick/postgres-tui/internal/service"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

// Commands wraps service dependencies and returns tea.Cmd factories.
type Commands struct {
	config service.ConfigService
	pg     service.PGService
}

// NewCommands creates a Commands instance.
func NewCommands(config service.ConfigService, pg service.PGService) *Commands {
	return &Commands{config: config, pg: pg}
}

// NewCommandsFromContainer creates Commands from a service container.
func NewCommandsFromContainer(c *service.Container) *Commands {
	return &Commands{config: c.Config, pg: c.PG}
}

// Config returns the config service.
func (c *Commands) Config() service.ConfigService { return c.config }

// PG returns the postgres service.
func (c *Commands) PG() service.PGService { return c.pg }

// LoadConnections loads saved connections.
func (c *Commands) LoadConnections() tea.Cmd {
	return func() tea.Msg {
		conns, err := c.config.ListConnections()
		return types.ConnectionsLoadedMsg{Connections: conns, Err: err}
	}
}

// AddConnection adds a connection.
func (c *Commands) AddConnection(conn types.Connection) tea.Cmd {
	return func() tea.Msg {
		added, err := c.config.AddConnection(conn)
		return types.ConnectionAddedMsg{Connection: added, Err: err}
	}
}

// UpdateConnection updates a connection.
func (c *Commands) UpdateConnection(conn types.Connection) tea.Cmd {
	return func() tea.Msg {
		updated, err := c.config.UpdateConnection(conn)
		return types.ConnectionUpdatedMsg{Connection: updated, Err: err}
	}
}

// DeleteConnection deletes a connection.
func (c *Commands) DeleteConnection(id int64) tea.Cmd {
	return func() tea.Msg {
		err := c.config.DeleteConnection(id)
		return types.ConnectionDeletedMsg{ID: id, Err: err}
	}
}

// Connect connects to a server.
func (c *Commands) Connect(conn types.Connection) tea.Cmd {
	return func() tea.Msg {
		if err := c.pg.Connect(conn); err != nil {
			return types.ConnectedMsg{Err: err}
		}
		info, err := c.pg.GetServerInfo()
		return types.ConnectedMsg{Info: info, Err: err}
	}
}

// Disconnect disconnects.
func (c *Commands) Disconnect() tea.Cmd {
	return func() tea.Msg {
		_ = c.pg.Disconnect()
		return types.DisconnectedMsg{}
	}
}

// TestConnection tests a connection.
func (c *Commands) TestConnection(conn types.Connection) tea.Cmd {
	return func() tea.Msg {
		latency, info, err := c.pg.TestConnection(conn)
		return types.ConnectionTestMsg{
			Success: err == nil,
			Err:     err,
			Latency: latency,
			Info:    info,
		}
	}
}

// LoadDatabases lists databases.
func (c *Commands) LoadDatabases() tea.Cmd {
	return func() tea.Msg {
		dbs, err := c.pg.ListDatabases()
		return types.DatabasesLoadedMsg{Databases: dbs, Err: err}
	}
}

// SelectDatabase switches to a database and reloads server info.
func (c *Commands) SelectDatabase(name string) tea.Cmd {
	return func() tea.Msg {
		if err := c.pg.SwitchDatabase(name); err != nil {
			return types.DatabaseSelectedMsg{Database: name, Err: err}
		}
		info, err := c.pg.GetServerInfo()
		return types.DatabaseSelectedMsg{Database: name, Info: info, Err: err}
	}
}

// LoadSchemas lists schemas.
func (c *Commands) LoadSchemas() tea.Cmd {
	return func() tea.Msg {
		schemas, err := c.pg.ListSchemas()
		return types.SchemasLoadedMsg{Schemas: schemas, Err: err}
	}
}

// LoadObjects lists objects of a kind.
func (c *Commands) LoadObjects(schema string, kind types.ObjectKind) tea.Cmd {
	return c.LoadObjectKinds(schema, []types.ObjectKind{kind})
}

// LoadObjectKinds lists and merges objects for multiple kinds (sidebar filters).
func (c *Commands) LoadObjectKinds(schema string, kinds []types.ObjectKind) tea.Cmd {
	return func() tea.Msg {
		if len(kinds) == 0 {
			return types.ObjectsLoadedMsg{Objects: nil, Err: nil}
		}
		var all []types.SchemaObject
		var lastKind types.ObjectKind
		for _, kind := range kinds {
			lastKind = kind
			objs, err := c.pg.ListObjects(schema, kind)
			if err != nil {
				return types.ObjectsLoadedMsg{Objects: all, Kind: kind, Err: err}
			}
			all = append(all, objs...)
		}
		// Stable sort: kind then name
		sort.SliceStable(all, func(i, j int) bool {
			if all[i].Kind != all[j].Kind {
				return all[i].Kind < all[j].Kind
			}
			if all[i].Schema != all[j].Schema {
				return all[i].Schema < all[j].Schema
			}
			return all[i].Name < all[j].Name
		})
		return types.ObjectsLoadedMsg{Objects: all, Kind: lastKind, Err: nil}
	}
}

// LoadTableDetail loads table/view metadata.
func (c *Commands) LoadTableDetail(schema, name string) tea.Cmd {
	return func() tea.Msg {
		detail, err := c.pg.GetTableDetail(schema, name)
		return types.TableDetailLoadedMsg{Detail: detail, Err: err}
	}
}

// LoadObjectDetail loads kind-specific metadata (sequence/function/type/extension/relation).
func (c *Commands) LoadObjectDetail(schema, name string, kind types.ObjectKind) tea.Cmd {
	return func() tea.Msg {
		detail, err := c.pg.GetObjectDetail(schema, name, kind)
		return types.TableDetailLoadedMsg{Detail: detail, Err: err}
	}
}

// LoadTableData loads paginated table rows.
func (c *Commands) LoadTableData(schema, name string, offset, limit int) tea.Cmd {
	return func() tea.Msg {
		result, err := c.pg.GetTableData(schema, name, offset, limit)
		return types.TableDataLoadedMsg{Result: result, Offset: offset, Err: err}
	}
}

// RunQuery executes SQL.
func (c *Commands) RunQuery(sql string, limit int) tea.Cmd {
	return func() tea.Msg {
		result, err := c.pg.RunQuery(sql, limit)
		return types.QueryResultMsg{Result: result, Err: err}
	}
}

// LoadActivity loads pg_stat_activity.
func (c *Commands) LoadActivity() tea.Cmd {
	return func() tea.Msg {
		rows, err := c.pg.ListActivity()
		return types.ActivityLoadedMsg{Rows: rows, Err: err}
	}
}

// LoadERD loads FK relationships for a schema ER diagram.
func (c *Commands) LoadERD(schema string) tea.Cmd {
	return func() tea.Msg {
		graph, err := c.pg.ListERD(schema)
		return types.ERDLoadedMsg{Graph: graph, Err: err}
	}
}

// LoadServerInfo reloads server info.
func (c *Commands) LoadServerInfo() tea.Cmd {
	return func() tea.Msg {
		info, err := c.pg.GetServerInfo()
		return types.ServerInfoLoadedMsg{Info: info, Err: err}
	}
}

// LoadFavorites loads favorites.
func (c *Commands) LoadFavorites(connID int64) tea.Cmd {
	return func() tea.Msg {
		return types.FavoritesLoadedMsg{Favorites: c.config.ListFavorites(connID)}
	}
}

// ExportCSV writes query rows to a CSV file.
func (c *Commands) ExportCSV(path string, result types.QueryResult) tea.Cmd {
	return func() tea.Msg {
		if path == "" {
			path = fmt.Sprintf("/tmp/postgres-tui-export-%d.csv", time.Now().Unix())
		}
		var b strings.Builder
		b.WriteString(strings.Join(result.Columns, ","))
		b.WriteByte('\n')
		for _, row := range result.Rows {
			escaped := make([]string, len(row))
			for i, cell := range row {
				if strings.ContainsAny(cell, ",\"\n") {
					escaped[i] = `"` + strings.ReplaceAll(cell, `"`, `""`) + `"`
				} else {
					escaped[i] = cell
				}
			}
			b.WriteString(strings.Join(escaped, ","))
			b.WriteByte('\n')
		}
		err := os.WriteFile(path, []byte(b.String()), 0o600)
		return types.ExportDoneMsg{Path: path, Rows: len(result.Rows), Err: err}
	}
}

var clipboardWrite = clipboard.WriteAll

// CopyToClipboard copies text to the system clipboard.
func (c *Commands) CopyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		if err := clipboardWrite(text); err != nil {
			return types.StatusMsg{Text: "clipboard: " + err.Error()}
		}
		return types.StatusMsg{Text: "Copied to clipboard"}
	}
}

// CheckForUpdate is a no-op stub for dev builds (real update in main package).
func CheckForUpdate(version string) tea.Cmd {
	return func() tea.Msg {
		_ = version
		return nil
	}
}
