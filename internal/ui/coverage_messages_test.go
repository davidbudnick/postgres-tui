package ui

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/cmd"
	"github.com/davidbudnick/postgres-tui/internal/db"
	"github.com/davidbudnick/postgres-tui/internal/testutil"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func covModel(t *testing.T) Model {
	t.Helper()
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	m.Width = 140
	m.Height = 40
	m.Version = "1.2.3"
	m.Cmds = cmd.NewCommands(cfg, testutil.NewMockPG())
	m.Connections = []types.Connection{
		{ID: 1, Name: "Local", Host: "localhost", Port: 5432, Database: "demo", Username: "postgres"},
	}
	m.CurrentConn = &m.Connections[0]
	m.CurrentDatabase = "demo"
	m.CurrentSchema = "public"
	m.Schemas = []types.SchemaInfo{{Name: "public", TableCount: 3}, {Name: "billing", TableCount: 1}}
	m.SelectedSchema = 0
	m.KindEnabled = defaultKindFilters()
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "users", Kind: types.ObjectTable, RowEstimate: 10, SizePretty: "8 kB"},
		{Schema: "public", Name: "orders", Kind: types.ObjectTable, RowEstimate: 20, SizePretty: "16 kB"},
		{Schema: "public", Name: "active_users", Kind: types.ObjectView},
		{Schema: "public", Name: "id_seq", Kind: types.ObjectSequence},
		{Schema: "public", Name: "do_stuff", Kind: types.ObjectFunction},
		{Schema: "public", Name: "status", Kind: types.ObjectType},
		{Schema: "public", Name: "plpgsql", Kind: types.ObjectExtension},
	}
	m.SelectedObjIdx = 0
	m.TableDetail = types.TableDetail{
		Object: m.Objects[0],
		Columns: []types.ColumnInfo{
			{Name: "id", DataType: "integer", IsPrimaryKey: true},
			{Name: "email", DataType: "text", IsNullable: true},
		},
		Indexes:     []types.IndexInfo{{Name: "users_pkey", IsPrimary: true, SizePretty: "8 kB"}},
		Constraints: []types.ConstraintInfo{{Name: "users_pkey", Type: "PRIMARY KEY", Definition: "PRIMARY KEY (id)"}},
		CreateSQL:   "SELECT 1",
		Props:       []types.DetailProp{{Label: "k", Value: "v"}},
	}
	m.TableData = types.QueryResult{
		Columns:  []string{"id", "email"},
		Rows:     [][]string{{"1", "a@b.c"}, {"2", "d@e.f"}},
		IsSelect: true, Duration: time.Millisecond,
	}
	m.QueryResult = m.TableData
	m.Activity = []types.ActivityRow{
		{PID: 1, User: "u", Database: "demo", State: "active", Query: "SELECT 1", Duration: time.Second},
		{PID: 2, State: "idle", Duration: 0},
		{PID: 3, State: "idle in transaction", Duration: 40 * time.Second},
	}
	m.ERD = types.ERDGraph{
		Schema: "public",
		Tables: []types.ERDTable{
			{Name: "users", Columns: []string{"id", "email"}},
			{Name: "orders", Columns: []string{"id", "user_id"}},
		},
		Edges: []types.FKEdge{{FromTable: "orders", FromCols: []string{"user_id"}, ToTable: "users", ToCols: []string{"id"}}},
	}
	m.Databases = []types.DatabaseInfo{
		{Name: "demo", Owner: "postgres", SizePretty: "12 MB", Encoding: "UTF8"},
		{Name: "postgres", Owner: "postgres", SizePretty: "8 MB", Encoding: "UTF8"},
	}
	m.ServerInfo = types.ServerInfo{
		Version: "PostgreSQL 16.0 on x86_64-pc-linux-gnu", User: "postgres", Database: "demo",
		Host: "localhost", Port: 5432, Encoding: "UTF8", Timezone: "UTC", Uptime: "1h",
		ActiveConns: 2, MaxConns: 100,
	}
	m.Favorites = []types.Favorite{
		{ConnectionID: 1, Database: "demo", Schema: "public", Object: "users", Kind: "table"},
	}
	m.Logs = types.NewLogWriter()
	_, _ = m.Logs.Write([]byte(`{"level":"INFO","msg":"hi"}`))
	m.PaletteItems = defaultPaletteItems()
	m.PageSize = 50
	ta := textarea.New()
	ta.SetWidth(60)
	ta.SetHeight(6)
	ta.SetValue("SELECT * FROM users")
	m.QueryArea = &ta
	m.rebuildSQLCompleter()
	return m
}

func TestUpdate_AllMessages(t *testing.T) {
	m := covModel(t)

	msgs := []tea.Msg{
		tea.WindowSizeMsg{Width: 100, Height: 30},
		types.StatusMsg{Text: "status"},
		types.ConnectionsLoadedMsg{Connections: m.Connections},
		types.ConnectionsLoadedMsg{Err: errors.New("x")},
		types.ConnectionsLoadedMsg{Connections: nil},
		types.ConnectionAddedMsg{Connection: types.Connection{ID: 9, Name: "n"}},
		types.ConnectionAddedMsg{Err: errors.New("x")},
		types.ConnectionUpdatedMsg{Connection: m.Connections[0]},
		types.ConnectionUpdatedMsg{Connection: types.Connection{ID: 999}},
		types.ConnectionUpdatedMsg{Err: errors.New("x")},
		types.ConnectionDeletedMsg{ID: 1},
		types.ConnectionDeletedMsg{ID: 999},
		types.ConnectionDeletedMsg{Err: errors.New("x")},
		types.AutoConnectMsg{Connection: m.Connections[0]},
		types.ConnectedMsg{Info: m.ServerInfo},
		types.ConnectedMsg{Err: errors.New("bad")},
		types.DisconnectedMsg{},
		types.DatabasesLoadedMsg{Databases: m.Databases},
		types.DatabasesLoadedMsg{Err: errors.New("x")},
		types.DatabaseSelectedMsg{Database: "demo", Info: m.ServerInfo},
		types.DatabaseSelectedMsg{Err: errors.New("x")},
		types.SchemasLoadedMsg{Schemas: m.Schemas},
		types.SchemasLoadedMsg{Schemas: nil},
		types.SchemasLoadedMsg{Err: errors.New("x")},
		types.ObjectsLoadedMsg{Objects: m.Objects},
		types.ObjectsLoadedMsg{Err: errors.New("x")},
		types.TableDetailLoadedMsg{Detail: m.TableDetail},
		types.TableDetailLoadedMsg{Err: errors.New("x")},
		types.TableDataLoadedMsg{Result: m.TableData, Offset: 0},
		types.TableDataLoadedMsg{Result: types.QueryResult{}, Offset: 50},
		types.TableDataLoadedMsg{Err: errors.New("x")},
		types.QueryResultMsg{Result: m.QueryResult},
		types.QueryResultMsg{Err: errors.New("x")},
		types.ActivityLoadedMsg{Rows: m.Activity},
		types.ActivityLoadedMsg{Err: errors.New("x")},
		types.ERDLoadedMsg{Graph: m.ERD},
		types.ERDLoadedMsg{Err: errors.New("x")},
		types.ServerInfoLoadedMsg{Info: m.ServerInfo},
		types.ServerInfoLoadedMsg{Err: errors.New("x")},
		types.ConnectionTestMsg{Success: true, Latency: time.Millisecond, Info: m.ServerInfo},
		types.ConnectionTestMsg{Success: false, Err: errors.New("no")},
		types.ConnectionTestMsg{Success: false},
		types.FavoritesLoadedMsg{Favorites: m.Favorites},
		types.ExportDoneMsg{Path: "/tmp/x.csv", Rows: 2},
		types.ExportDoneMsg{Err: errors.New("x")},
		types.ExportDoneMsg{Path: "/tmp/x.csv", Rows: 1}, // PrevScreen 0
		sequencedDetailMsg{TableDetailLoadedMsg: types.TableDetailLoadedMsg{Detail: m.TableDetail}, seq: m.contentSeq},
		sequencedDetailMsg{TableDetailLoadedMsg: types.TableDetailLoadedMsg{Detail: m.TableDetail}, seq: -1},
		sequencedDataMsg{TableDataLoadedMsg: types.TableDataLoadedMsg{Result: m.TableData}, seq: m.contentSeq},
		sequencedDataMsg{TableDataLoadedMsg: types.TableDataLoadedMsg{Result: m.TableData}, seq: -1},
		tea.KeyPressMsg{Text: "ctrl+c"},
	}

	for i, msg := range msgs {
		nm, _ := m.Update(msg)
		m = nm.(Model)
		// re-seed cmds/size after disconnect-like msgs
		if m.Width == 0 {
			m.Width, m.Height = 140, 40
		}
		if m.Cmds == nil {
			m = covModel(t)
		}
		_ = i
	}

	// Connected without database -> databases screen
	m = covModel(t)
	m.CurrentConn = &types.Connection{Name: "x", Host: "h"}
	m.CurrentDatabase = ""
	nm, _ := m.Update(types.ConnectedMsg{Info: types.ServerInfo{Version: "PostgreSQL 16"}})
	m = nm.(Model)

	// Connected with cmds nil
	m = NewModel()
	m.Width, m.Height = 80, 24
	_, _ = m.Update(types.ConnectedMsg{Info: types.ServerInfo{Database: "d"}})
	_, _ = m.Update(types.AutoConnectMsg{Connection: types.Connection{Host: "h"}})

	// DatabasesLoaded error on workspace shouldn't clobber
	m = covModel(t)
	m.Screen = types.ScreenBrowser
	_, _ = m.Update(types.DatabasesLoadedMsg{Err: errors.New("race")})

	// ObjectsLoaded clears current object not in list
	m = covModel(t)
	m.Screen = types.ScreenTableData
	cur := types.SchemaObject{Schema: "public", Name: "gone", Kind: types.ObjectTable}
	m.CurrentObject = &cur
	_, _ = m.Update(types.ObjectsLoadedMsg{Objects: m.Objects})

	// ObjectsLoaded with sidebar on kind/schema rows
	m = covModel(t)
	m.Screen = types.ScreenBrowser
	m = m.pinSidebarCursorToKind(navTables)
	_, _ = m.Update(types.ObjectsLoadedMsg{Objects: m.Objects})
	m = covModel(t)
	m = m.pinSidebarCursorToSchema(0)
	_, _ = m.Update(types.ObjectsLoadedMsg{Objects: m.Objects})

	// ExportDone with PrevScreen set
	m = covModel(t)
	m.PrevScreen = types.ScreenTableData
	_, _ = m.Update(types.ExportDoneMsg{Path: "p", Rows: 1})

	// WindowSize with QueryArea
	m = covModel(t)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
}

func TestShortVersion(t *testing.T) {
	if shortVersion("PostgreSQL 16.0 on x86") != "PostgreSQL 16.0" {
		t.Fatal(shortVersion("PostgreSQL 16.0 on x86"))
	}
	long := string(make([]byte, 50))
	for i := range long {
		long = long[:i] + "a" + long[i+1:]
	}
	if len(shortVersion(long)) != 40 {
		t.Fatal(len(shortVersion(long)))
	}
	if shortVersion("short") != "short" {
		t.Fatal(shortVersion("short"))
	}
}

func TestInitAndCLIConnection(t *testing.T) {
	m := covModel(t)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("nil init")
	}
	_ = cmd()

	conn := types.Connection{Host: "h", Port: 1}
	m.CLIConnection = &conn
	cmd = m.Init()
	if cmd == nil {
		t.Fatal("nil init cli")
	}
	// Execute batch-ish cmd
	_ = cmd()
}
