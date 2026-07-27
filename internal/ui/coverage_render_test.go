package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/textarea"
	"github.com/davidbudnick/postgres-tui/internal/cmd"
	"github.com/davidbudnick/postgres-tui/internal/db"
	"github.com/davidbudnick/postgres-tui/internal/testutil"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

// fullySeededModel builds a Model with every content pane populated.
func fullySeededModel(t *testing.T) Model {
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
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	m.CurrentDatabase = "demo"
	m.CurrentSchema = "public"
	m.CurrentConn = &types.Connection{
		ID: 1, Name: "Local Demo", Host: "localhost", Port: 5432,
		Database: "demo", Username: "postgres", SSLMode: types.SSLModePrefer,
	}
	m.Schemas = []types.SchemaInfo{
		{Name: "analytics", Owner: "postgres", TableCount: 2, ViewCount: 1},
		{Name: "billing", Owner: "app", TableCount: 1},
		{Name: "public", Owner: "postgres", TableCount: 8, ViewCount: 2},
	}
	m.SelectedSchema = 2
	m.KindEnabled = map[NavSection]bool{
		navTables: true, navViews: true, navSequences: true,
		navFunctions: true, navTypes: true, navExtensions: true,
	}
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "users", Kind: types.ObjectTable, Owner: "postgres", RowEstimate: 1000, SizePretty: "128 kB", Comment: "app users"},
		{Schema: "public", Name: "orders", Kind: types.ObjectTable, Owner: "postgres", RowEstimate: 5000, SizePretty: "800 kB"},
		{Schema: "public", Name: "order_items", Kind: types.ObjectTable, RowEstimate: 17500, SizePretty: "1.2 MB"},
		{Schema: "public", Name: "active_users", Kind: types.ObjectView, Owner: "postgres", RowEstimate: 0, SizePretty: "0"},
		{Schema: "public", Name: "sales_mv", Kind: types.ObjectMatView, RowEstimate: 200, SizePretty: "64 kB"},
		{Schema: "public", Name: "user_id_seq", Kind: types.ObjectSequence, Owner: "postgres"},
		{Schema: "public", Name: "calc_total", Kind: types.ObjectFunction, Owner: "postgres"},
		{Schema: "public", Name: "order_status", Kind: types.ObjectType, Owner: "postgres"},
		{Schema: "public", Name: "pgcrypto", Kind: types.ObjectExtension},
	}
	m.SelectedObjIdx = 0
	m = m.setCurrentObject(m.Objects[0])
	m = m.syncSidebarCursorToObject()

	m.TableDetail = types.TableDetail{
		Object: m.Objects[0],
		Columns: []types.ColumnInfo{
			{Name: "id", DataType: "integer", IsPrimaryKey: true, IsNullable: false, Position: 1, Default: "nextval('user_id_seq')"},
			{Name: "email", DataType: "text", IsNullable: false, Position: 2},
			{Name: "user_id", DataType: "integer", IsNullable: true, Position: 3},
			{Name: "meta", DataType: "jsonb", IsNullable: true, Position: 4},
			{Name: "created_at", DataType: "timestamp with time zone", IsNullable: false, Position: 5, Default: "now()"},
		},
		Indexes: []types.IndexInfo{
			{Name: "users_pkey", IsPrimary: true, SizePretty: "16 kB", Definition: "CREATE UNIQUE INDEX users_pkey ON users USING btree (id)"},
			{Name: "users_email_key", IsUnique: true, SizePretty: "8 kB", Definition: "CREATE UNIQUE INDEX users_email_key ON users (email)"},
		},
		Constraints: []types.ConstraintInfo{
			{Name: "users_pkey", Type: "PRIMARY KEY", Definition: "PRIMARY KEY (id)"},
			{Name: "users_email_key", Type: "UNIQUE", Definition: "UNIQUE (email)"},
		},
		CreateSQL: "CREATE TABLE public.users (\n  id integer PRIMARY KEY,\n  email text NOT NULL\n);",
		Props: []types.DetailProp{
			{Label: "Owner", Value: "postgres"},
			{Label: "Tablespace", Value: "pg_default"},
		},
	}
	m.TableData = types.QueryResult{
		Columns: []string{"id", "email", "active", "created_at", "meta", "score", "extra1", "extra2"},
		Rows: [][]string{
			{"1", "alice@example.com", "true", "2024-01-15T10:00:00Z", `{"role":"admin"}`, "1.5", "a", "b"},
			{"2", "bob@example.com", "false", "2024-02-01T12:30:00Z", `{}`, "2.0", "c", "d"},
			{"3", "", "true", "2024-03-01T00:00:00Z", `[1,2]`, "0", "e", "f"},
		},
		Duration:  2 * time.Millisecond,
		IsSelect:  true,
		Truncated: true,
	}
	m.DataOffset = 0
	m.DataCursor = 1
	m.DataCol = 1
	m.PageSize = 100

	m.QueryResult = types.QueryResult{
		Columns:  []string{"id", "email"},
		Rows:     [][]string{{"1", "alice@example.com"}, {"2", "bob@example.com"}},
		Duration: time.Millisecond,
		IsSelect: true,
		SQL:      "SELECT id, email FROM users LIMIT 100",
	}
	m.QueryFocus = "results"
	m.QueryHistory = []string{"SELECT 1", "SELECT 2"}
	m.HistoryIdx = -1
	ta := textarea.New()
	ta.SetValue("SELECT * FROM users WHERE ")
	ta.SetWidth(60)
	ta.SetHeight(6)
	ta.SetCursorColumn(len("SELECT * FROM users WHERE "))
	m.QueryArea = &ta
	m.QuerySuggests = []string{"email", "id", "active", "created_at", "meta"}
	m.QuerySuggestIdx = 0
	m.SchemaCols = map[string][]string{
		"public.users": {"id", "email", "active", "created_at", "meta"},
		"users":        {"id", "email", "active", "created_at", "meta"},
	}
	m.rebuildSQLCompleter()

	m.Activity = []types.ActivityRow{
		{PID: 100, User: "postgres", Database: "demo", State: "active", Query: "SELECT 1", Duration: 2 * time.Second},
		{PID: 101, User: "postgres", Database: "demo", State: "idle", Query: "", Duration: 0},
		{PID: 102, User: "app", Database: "demo", State: "idle in transaction", Query: "UPDATE t", Duration: 40 * time.Second},
		{PID: 103, User: "", Database: "", State: "idle in transaction (aborted)", Query: "x", Duration: 5 * time.Minute},
		{PID: 104, User: "app", Database: "demo", State: "fastpath function call", Query: "fn()", Duration: 90 * time.Millisecond},
		{PID: 105, User: "app", Database: "demo", State: "active", Query: "long", Duration: 2 * time.Hour},
		{PID: 106, User: "app", Database: "demo", State: "active", Query: "day", Duration: 26 * time.Hour},
	}
	m.SelectedActIdx = 0

	m.ERD = types.ERDGraph{
		Schema: "public",
		Tables: []types.ERDTable{
			{Name: "users", Columns: []string{"id", "email", "created_at"}},
			{Name: "orders", Columns: []string{"id", "user_id", "total_cents"}},
			{Name: "order_items", Columns: []string{"id", "order_id", "product_id", "qty"}},
			{Name: "products", Columns: []string{"id", "name", "price"}},
		},
		Edges: []types.FKEdge{
			{FromTable: "orders", FromCols: []string{"user_id"}, ToTable: "users", ToCols: []string{"id"}},
			{FromTable: "order_items", FromCols: []string{"order_id"}, ToTable: "orders", ToCols: []string{"id"}},
			{FromTable: "order_items", FromCols: []string{"product_id"}, ToTable: "products", ToCols: []string{"id"}},
		},
	}
	m.ERDOffset = 0

	m.ServerInfo = types.ServerInfo{
		Version: "PostgreSQL 16.13 on x86_64-pc-linux-gnu", User: "postgres", Database: "demo",
		Host: "localhost", Port: 5432, Encoding: "UTF8", Timezone: "UTC",
		Uptime: "2 days", ActiveConns: 3, MaxConns: 100,
	}
	m.Databases = []types.DatabaseInfo{
		{Name: "demo", Owner: "postgres", SizePretty: "12 MB", Encoding: "UTF8"},
		{Name: "postgres", Owner: "postgres", SizePretty: "8 MB", Encoding: "UTF8"},
		{Name: "template1", Owner: "postgres", SizePretty: "7 MB", Encoding: "UTF8"},
	}
	m.SelectedDBIdx = 0

	m.Connections = []types.Connection{
		{ID: 1, Name: "Local Demo", Host: "localhost", Port: 5432, Database: "demo", Username: "postgres", SSLMode: types.SSLModePrefer},
		{ID: 2, Name: "Prod RO", Host: "db.example.com", Port: 5432, Database: "app", Username: "reader", SSLMode: types.SSLModeRequire, ReadOnly: true},
		{ID: 3, Name: "NoSSL", Host: "127.0.0.1", Port: 5433, Username: "postgres", SSLMode: types.SSLModeDisable},
	}
	m.SelectedConnIdx = 0

	m.Favorites = []types.Favorite{
		{ConnectionID: 1, Database: "demo", Schema: "public", Object: "users", Kind: "table"},
		{ConnectionID: 1, Database: "demo", Schema: "public", Object: "orders", Kind: "table"},
	}
	m.SelectedFavIdx = 0

	m.Logs = types.NewLogWriter()
	_, _ = m.Logs.Write([]byte("line one\n"))
	_, _ = m.Logs.Write([]byte("line two\n"))
	_, _ = m.Logs.Write([]byte("line three\n"))

	m.StatusMsg = "ready"
	m.ReadOnly = false
	return m
}

func TestCoverageRenderAllScreens(t *testing.T) {
	base := fullySeededModel(t)

	screens := []struct {
		name   string
		screen types.Screen
		setup  func(Model) Model
	}{
		{"connections", types.ScreenConnections, nil},
		{"add_connection", types.ScreenAddConnection, func(m Model) Model {
			m.ConnFocusIdx = connFieldSSL
			m.ConnSSLIdx = 2
			m.ConnReadOnly = true
			return m
		}},
		{"edit_connection", types.ScreenEditConnection, func(m Model) Model {
			m.ConnFocusIdx = connFieldReadOnly
			m.ConnReadOnly = false
			m.Err = errors.New("validation failed")
			return m
		}},
		{"databases_fullscreen", types.ScreenDatabases, nil},
		{"browser_preview", types.ScreenBrowser, func(m Model) Model {
			m.ContentMode = contentPreview
			return m
		}},
		{"browser_schema", types.ScreenBrowser, func(m Model) Model {
			m.ContentMode = contentSchema
			m.Focus = focusContent
			return m
		}},
		{"browser_databases", types.ScreenBrowser, func(m Model) Model {
			m.ContentMode = contentDatabases
			m.Focus = focusContent
			return m
		}},
		{"table_data", types.ScreenTableData, func(m Model) Model {
			m.Focus = focusContent
			return m
		}},
		{"table_detail", types.ScreenTableDetail, func(m Model) Model {
			m.Focus = focusContent
			m.DetailTab = 0
			return m
		}},
		{"query", types.ScreenQuery, func(m Model) Model {
			m.Focus = focusContent
			m.QueryFocus = "results"
			return m
		}},
		{"activity", types.ScreenActivity, func(m Model) Model {
			m.Focus = focusContent
			return m
		}},
		{"erd", types.ScreenERD, func(m Model) Model {
			m.Focus = focusContent
			return m
		}},
		{"server", types.ScreenServerInfo, func(m Model) Model {
			m.Focus = focusContent
			return m
		}},
		{"help", types.ScreenHelp, nil},
		{"confirm_delete", types.ScreenConfirmDelete, func(m Model) Model {
			m.ConfirmType = "connection"
			m.ConfirmData = m.Connections[0]
			return m
		}},
		{"confirm_delete_default", types.ScreenConfirmDelete, func(m Model) Model {
			m.ConfirmType = "other"
			return m
		}},
		{"test_connection_ok", types.ScreenTestConnection, func(m Model) Model {
			m.TestConnResult = "OK — PostgreSQL 16"
			return m
		}},
		{"test_connection_err", types.ScreenTestConnection, func(m Model) Model {
			m.TestConnResult = "connection refused"
			return m
		}},
		{"test_connection_loading", types.ScreenTestConnection, func(m Model) Model {
			m.Loading = true
			return m
		}},
		{"test_connection_empty", types.ScreenTestConnection, func(m Model) Model {
			m.TestConnResult = ""
			return m
		}},
		{"logs", types.ScreenLogs, nil},
		{"logs_empty", types.ScreenLogs, func(m Model) Model {
			m.Logs = nil
			return m
		}},
		{"favorites", types.ScreenFavorites, nil},
		{"favorites_empty", types.ScreenFavorites, func(m Model) Model {
			m.Favorites = nil
			return m
		}},
		{"export", types.ScreenExport, nil},
		{"command_palette", types.ScreenCommandPalette, nil},
		{"command_palette_filter", types.ScreenCommandPalette, func(m Model) Model {
			m.Inputs.PaletteInput.SetValue("query")
			m.PaletteIdx = 0
			return m
		}},
		{"command_palette_nomatch", types.ScreenCommandPalette, func(m Model) Model {
			m.Inputs.PaletteInput.SetValue("zzzz-no-match")
			return m
		}},
	}

	for _, tc := range screens {
		t.Run(tc.name, func(t *testing.T) {
			m := base
			m.Screen = tc.screen
			if tc.setup != nil {
				m = tc.setup(m)
			}
			view := m.getScreenView()
			if strings.TrimSpace(view) == "" && tc.screen <= types.ScreenCommandPalette {
				t.Fatal("empty getScreenView")
			}
			_ = m.View()
			_ = m.render()
		})
	}

	m := base
	m.Screen = types.Screen(99)
	if got := m.getScreenView(); got != "" {
		t.Fatalf("unknown screen want empty got %q", got)
	}
}

func TestCoverageRenderDetailTabs(t *testing.T) {
	m := fullySeededModel(t)
	m.Screen = types.ScreenTableDetail
	m.Focus = focusContent

	tabs := m.detailTabs(m.TableDetail)
	if len(tabs) < 3 {
		t.Fatalf("tabs=%v", tabs)
	}
	for i := 0; i < len(tabs)+1; i++ {
		m.DetailTab = i
		out := m.viewTableDetailContent(100, 30)
		if strings.TrimSpace(out) == "" {
			t.Fatalf("tab %d empty", i)
		}
	}

	m.TableDetail.Indexes = nil
	m.DetailTab = 1
	_ = m.viewTableDetailContent(80, 20)
	m.TableDetail.Constraints = nil
	m.DetailTab = 2
	_ = m.viewTableDetailContent(80, 20)

	m.TableDetail.Columns = nil
	m.DetailTab = 0
	_ = m.viewTableDetailContent(80, 20)

	fn := types.SchemaObject{Schema: "public", Name: "calc_total", Kind: types.ObjectFunction}
	m = m.setCurrentObject(fn)
	m.TableDetail = types.TableDetail{
		Object:    fn,
		CreateSQL: "CREATE FUNCTION calc_total() RETURNS int AS $$ SELECT 1 $$ LANGUAGE sql;",
		Props: []types.DetailProp{
			{Label: "Language", Value: "sql"},
			{Label: "Returns", Value: "integer"},
			{Label: "VeryLongPropertyLabel", Value: "value"},
		},
	}
	for i := 0; i < 4; i++ {
		m.DetailTab = i
		_ = m.viewTableDetailContent(90, 25)
	}
	m.TableDetail.Props = nil
	m.DetailTab = 0
	_ = m.viewTableDetailContent(80, 20)
	m.TableDetail.CreateSQL = ""
	tabs = m.detailTabs(m.TableDetail)
	for i := range tabs {
		m.DetailTab = i
		_ = m.viewTableDetailContent(80, 20)
	}
	_ = m.renderDetailDefinition(types.TableDetail{}, 80, 20)

	m.Loading = true
	_ = m.viewTableDetailContent(80, 20)
	m.Loading = false
	other := types.SchemaObject{Schema: "public", Name: "orders", Kind: types.ObjectTable}
	m = m.setCurrentObject(other)
	m.TableDetail.Object = fn
	_ = m.viewTableDetailContent(80, 20)

	m.CurrentObject = nil
	m.TableDetail = types.TableDetail{}
	_ = m.viewTableDetailContent(80, 20)
}

func TestCoverageRenderQueryAndData(t *testing.T) {
	m := fullySeededModel(t)
	m.Screen = types.ScreenQuery
	m.Focus = focusContent

	m.QueryFocus = "results"
	_ = m.viewQueryContent(100, 30)
	_ = m.viewQuery()

	m.QueryFocus = "editor"
	m.QuerySuggests = []string{"email", "id", "active", "created_at", "meta", "role", "score", "extra"}
	m.QuerySuggestIdx = 3
	_ = m.viewQueryContent(40, 30)
	_ = m.renderQuerySuggestions(30)
	_ = m.renderQuerySuggestions(200)
	m.QuerySuggests = nil
	_ = m.viewQueryContent(100, 30)
	_ = m.renderQuerySuggestions(80)

	m.Loading = true
	_ = m.viewQueryContent(80, 20)
	m.Loading = false
	m.QueryResult = types.QueryResult{}
	_ = m.viewQueryContent(80, 20)
	m.QueryResult = types.QueryResult{SQL: "UPDATE t SET x=1", RowsAffected: 3, Duration: time.Millisecond, IsSelect: false}
	_ = m.viewQueryContent(80, 20)
	m.QueryResult = types.QueryResult{
		Columns: []string{"a"}, Rows: [][]string{{"1"}}, IsSelect: true, Truncated: true, Duration: time.Millisecond,
	}
	_ = m.viewQueryContent(80, 20)

	m.QueryArea = nil
	_ = m.viewQueryContent(80, 20)

	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m = m.setCurrentObject(m.Objects[0])
	m.TableData = types.QueryResult{
		Columns: []string{"id", "email"}, Rows: [][]string{{"1", "a"}, {"2", "b"}},
		Duration: time.Millisecond, Truncated: true,
	}
	m.DataOffset = 0
	_ = m.viewTableDataContent(80, 20)
	_ = m.viewTableData()
	m.DataOffset = 10
	m.TableData.Truncated = false
	m.TableData.Rows = [][]string{{"11", "z"}}
	_ = m.viewTableDataContent(80, 20)
	m.Loading = true
	_ = m.viewTableDataContent(80, 20)
	m.Loading = false
	m.TableData.Columns = nil
	_ = m.viewTableDataContent(80, 20)
	m.TableData = types.QueryResult{Columns: []string{"id"}, Rows: nil, Duration: time.Millisecond}
	m.DataOffset = 0
	_ = m.viewTableDataContent(80, 20)
	m.CurrentObject = nil
	_ = m.viewTableDataContent(80, 20)

	_ = m.renderResultTable(types.QueryResult{}, 0, 0, 10, 60)
	_ = m.renderResultTable(types.QueryResult{Columns: []string{"a"}}, 0, 0, 10, 10)
	wide := types.QueryResult{
		Columns: []string{"c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8", "c9", "c10"},
		Rows: [][]string{
			{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
			{"", "x", "y", "z", "a", "b", "c", "d", "e", "f"},
		},
	}
	_ = m.renderResultTable(wide, 0, 5, 10, 40)
	_ = m.renderResultTable(wide, 1, 0, 10, 80)
	many := types.QueryResult{Columns: []string{"id", "v"}, Rows: make([][]string, 50)}
	for i := range many.Rows {
		many.Rows[i] = []string{fmt.Sprintf("%d", i), strings.Repeat("x", 30)}
	}
	_ = m.renderResultTable(many, 25, 1, 8, 50)
}

func TestCoverageRenderActivityERDServer(t *testing.T) {
	m := fullySeededModel(t)

	m.Screen = types.ScreenActivity
	_ = m.viewActivityContent(120, 30)
	_ = m.viewActivity()
	m.Loading = true
	m.Activity = nil
	_ = m.viewActivityContent(80, 20)
	m.Loading = false
	_ = m.viewActivityContent(80, 20)
	_ = shortState("idle in transaction")
	_ = shortState("idle in transaction (aborted)")
	_ = shortState("fastpath function call")
	_ = shortState("active")
	_ = compactDuration(-1)
	_ = compactDuration(0)
	_ = compactDuration(500 * time.Microsecond)
	_ = compactDuration(5 * time.Millisecond)
	_ = compactDuration(3 * time.Second)
	_ = compactDuration(90 * time.Second)
	_ = compactDuration(2 * time.Minute)
	_ = compactDuration(90 * time.Minute)
	_ = compactDuration(2 * time.Hour)
	_ = compactDuration(26 * time.Hour)
	_ = compactDuration(48 * time.Hour)
	_ = formatActivityDur("idle", time.Second)
	_ = formatActivityDur("active", time.Second)
	_ = dashEmpty("")
	_ = dashEmpty("x")

	m = fullySeededModel(t)
	m.Screen = types.ScreenERD
	_ = m.viewERDContent(120, 30)
	_ = m.viewERD()
	_ = m.viewERDContent(20, 20)
	m.ERDOffset = 5
	_ = m.viewERDContent(40, 8)
	m.ERD.Schema = ""
	m.CurrentSchema = ""
	_ = m.viewERDContent(80, 20)
	m.Loading = true
	m.ERD.Tables = nil
	_ = m.viewERDContent(80, 20)
	m.Loading = false
	_ = m.viewERDContent(80, 20)
	_ = renderERDList(types.ERDGraph{Tables: []types.ERDTable{{Name: "t"}}}, 80, true)
	big := types.ERDGraph{Schema: "s"}
	for i := 0; i < erdMaxTables+5; i++ {
		big.Tables = append(big.Tables, types.ERDTable{Name: fmt.Sprintf("t%d", i), Columns: []string{"id"}})
	}
	big.Edges = []types.FKEdge{{FromTable: "t0", FromCols: []string{"id"}, ToTable: "t1", ToCols: []string{"id"}}}
	m.ERD = big
	_ = m.viewERDContent(120, 40)

	c := newERDCanvas(4, 2)
	c = newERDCanvas(20, 12)
	c.hline(5, 2, 3, 1)
	c.vline(3, 8, 2, 1)
	c.vline(3, 2, 8, 1)
	c.put(0, 0, '─', 1)
	c.put(0, 0, '│', 1)
	c.put(1, 1, '┌', 1)
	c.put(1, 1, '─', 1)
	_ = mergeWires('─', '│')
	_ = mergeWires('│', '─')
	_ = mergeWires('─', '─')
	_ = mergeWires('│', '│')
	_ = mergeWires('┌', '┐')
	_ = mergeWires('┬', '├')
	_ = mergeWireBox('┌', '│')
	_ = mergeWireBox('┌', '─')
	_ = mergeWireBox('┌', '┼')
	_ = isBoxRune('┼')
	_ = isBoxRune('x')
	_ = isWireRune('▼')
	_ = isWireRune('x')
	g := types.ERDGraph{
		Tables: []types.ERDTable{
			{Name: "a", Columns: []string{"id", "x"}},
			{Name: "b", Columns: []string{"id", "a_id"}},
		},
		Edges: []types.FKEdge{{FromTable: "b", FromCols: []string{"a_id"}, ToTable: "a", ToCols: []string{"id"}}},
	}
	_ = renderERDDiagram(g, 100)
	_ = erdFindTable(g, "missing")
	_ = erdFindTable(g, "a")
	_ = orderERDColumns("a", []string{"x", "id", "z"}, map[string]bool{"a.z": true}, false)
	_ = maxLayerWidth([][]string{{"a", "bb"}, {"ccc"}})
	_ = maxLayerWidth(nil)

	m = fullySeededModel(t)
	m.Screen = types.ScreenServerInfo
	_ = m.viewServerInfoContent(80, 20)
	_ = m.viewServerInfo()
	m.ServerInfo = types.ServerInfo{}
	_ = m.viewServerInfoContent(80, 20)
}

func TestCoverageRenderConnectionsDatabases(t *testing.T) {
	m := fullySeededModel(t)

	m.Screen = types.ScreenConnections
	_ = m.viewConnections()
	m.Loading = true
	m.ConnectionError = "timeout dialing host"
	m.Version = "1.0.0"
	_ = m.viewConnections()
	m.Loading = false
	m.ConnectionError = ""
	for i := 0; i < 12; i++ {
		m.Connections = append(m.Connections, types.Connection{
			ID: int64(10 + i), Name: fmt.Sprintf("c%d", i), Host: "h", Port: 5432,
		})
	}
	m.SelectedConnIdx = 8
	m.Height = 25
	_ = m.viewConnections()
	_ = m.connectionsKeyHelp(20)
	_ = m.connectionsKeyHelp(200)
	_ = m.buildStatsBar()
	_ = m.renderConnectionCard(types.Connection{Name: "x", Host: "h", Port: 1, SSLMode: types.SSLModeDisable}, false, 55)
	_ = m.renderConnectionCard(types.Connection{Name: "y", Host: "h", Port: 1, Database: "d", Username: "u", ReadOnly: true}, true, 55)

	m.Screen = types.ScreenAddConnection
	m.ConnFocusIdx = 0
	_ = m.viewConnectionForm()
	m.Screen = types.ScreenEditConnection
	m.ConnFocusIdx = connFieldSSL
	m.ConnSSLIdx = 0
	_ = m.viewConnectionForm()
	m.ConnFocusIdx = connFieldReadOnly
	m.ConnReadOnly = true
	m.Err = errors.New("bad")
	_ = m.viewConnectionForm()

	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "connection"
	m.ConfirmData = "not-a-conn"
	_ = m.viewConfirmDelete()

	m = fullySeededModel(t)
	m.Screen = types.ScreenDatabases
	m.ReadOnly = true
	_ = m.viewDatabases()
	m.Loading = true
	_ = m.viewDatabases()
	_ = m.viewDatabasesPanel(60, 10)
	m.Loading = false
	m.Databases = nil
	_ = m.viewDatabases()
	_ = m.viewDatabasesPanel(60, 10)
	m.Databases = []types.DatabaseInfo{
		{Name: "demo", Owner: "postgres", SizePretty: "1 MB", Encoding: "UTF8"},
		{Name: "other", Owner: "app", SizePretty: "2 MB", Encoding: "UTF8"},
		{Name: "third", Owner: "app", SizePretty: "3 MB", Encoding: "UTF8"},
	}
	m.CurrentDatabase = "demo"
	m.SelectedDBIdx = 1
	_ = m.viewDatabasesPanel(60, 12)
	m.CurrentConn = nil
	_ = m.viewDatabasesHeader()
	_ = m.viewDatabasesChips()
	_ = colWidth(10, 42, 16, 48)
	_ = colWidth(100, 42, 16, 48)
	_ = colWidth(200, 42, 16, 0)
}

func TestCoverageRenderSidebarAndWorkspace(t *testing.T) {
	m := fullySeededModel(t)
	m.Screen = types.ScreenBrowser
	m.ContentMode = contentPreview
	_ = m.viewWorkspace()
	_ = m.viewBrowser()
	_ = m.viewBrowserHeader()
	_ = m.viewBrowserFooter()
	_ = m.viewBrowserPreview(50, 20)
	_ = m.viewBrowserPreviewContent(50, 20)
	_ = m.viewSchemaOverviewContent(80, 25)
	_ = m.viewDatabasesContent(80, 20)

	m.Objects = nil
	_ = m.viewBrowserPreview(40, 15)
	_ = m.viewBrowserPreviewContent(40, 15)
	_ = m.viewSchemaOverviewContent(60, 15)
	m.Objects = fullySeededModel(t).Objects
	m.FilterInput.SetValue("zzz-none")
	m.ObjectFilter = "zzz-none"
	_ = m.viewSidebar(32, 30)
	m.ObjectFilter = ""
	m.FilterInput.SetValue("")
	m.FilterActive = true
	_ = m.viewSidebar(32, 30)
	m.FilterActive = false

	for _, o := range m.Objects {
		_ = m.renderObjectPreviewBody(o, 40, 25)
	}
	_ = m.renderObjectPreviewBody(types.SchemaObject{Schema: "public", Name: "x", Comment: "hi"}, 40, 20)
	m.SchemaCols = map[string][]string{"public.users": {"id", "email", "a", "b", "c", "d", "e", "f", "g"}}
	_ = m.renderObjectPreviewBody(m.Objects[0], 40, 18)
	_ = m.cachedColumnsFor(m.Objects[0])
	_ = m.cachedColumnsFor(types.SchemaObject{Schema: "public", Name: "users"})
	m.SchemaCols = nil
	m = m.setCurrentObject(m.Objects[0])
	m.TableDetail.Object = m.Objects[0]
	m.TableDetail.Columns = []types.ColumnInfo{{Name: "id"}, {Name: ""}, {Name: "email"}}
	_ = m.cachedColumnsFor(m.Objects[0])
	_ = m.cachedColumnsFor(types.SchemaObject{Schema: "other", Name: "nope"})
	m.SchemaCols = map[string][]string{}
	m.CurrentObject = nil
	m.TableDetail = types.TableDetail{}
	_ = m.renderObjectPreviewBody(types.SchemaObject{Schema: "public", Name: "t", Kind: types.ObjectTable}, 40, 20)

	m = fullySeededModel(t)
	m.ContentMode = contentSchema
	m.Focus = focusContent
	_ = m.viewSchemaOverviewContent(90, 30)
	m.Focus = focusSidebar
	_ = m.viewSchemaOverviewContent(90, 30)
	m.Loading = true
	_ = m.viewSchemaOverviewContent(80, 20)
	m.Loading = false
	m.KindEnabled = map[NavSection]bool{navTables: true}
	_ = m.viewSchemaOverviewContent(80, 20)
	m.Schemas = nil
	m.CurrentSchema = ""
	_ = m.viewSchemaOverviewContent(80, 20)

	m = fullySeededModel(t)
	m.ContentMode = contentDatabases
	_ = m.viewDatabasesContent(80, 20)
	m.Loading = true
	m.Databases = nil
	_ = m.viewDatabasesContent(80, 20)
	m.Loading = false
	_ = m.viewDatabasesContent(80, 20)

	for _, sc := range []types.Screen{
		types.ScreenBrowser, types.ScreenTableData, types.ScreenTableDetail,
		types.ScreenQuery, types.ScreenActivity, types.ScreenERD, types.ScreenServerInfo,
	} {
		m.Screen = sc
		if sc == types.ScreenBrowser {
			for _, mode := range []ContentMode{contentPreview, contentSchema, contentDatabases} {
				m.ContentMode = mode
				_ = m.viewWorkspaceFooter()
			}
		} else {
			_ = m.viewWorkspaceFooter()
		}
	}

	m.CurrentConn = nil
	m.CurrentDatabase = ""
	m.ReadOnly = true
	m.ServerInfo.Version = "PostgreSQL 16.13 on x86_64-pc-linux-gnu very long suffix"
	_ = m.viewWorkspaceHeader()

	m = fullySeededModel(t)
	m.Loading = true
	m.Screen = types.ScreenBrowser
	m.ContentMode = contentPreview
	_ = m.viewSidebar(32, 30)
	m.Loading = false
	m.Schemas = nil
	_ = m.viewSidebar(32, 30)
	m.Screen = types.ScreenQuery
	_ = m.viewSidebar(32, 30)
	m.Screen = types.ScreenActivity
	_ = m.viewSidebar(32, 30)
	m.Screen = types.ScreenERD
	_ = m.viewSidebar(32, 30)
	m.Screen = types.ScreenServerInfo
	_ = m.viewSidebar(32, 30)
	m.Screen = types.ScreenBrowser
	m.ContentMode = contentDatabases
	_ = m.viewSidebar(32, 30)
	m.Focus = focusContent
	_ = m.viewSidebar(32, 30)
	_ = m.viewSidebar(8, 10)
	m.Width = 50
	m.Height = 10
	_ = m.viewWorkspace()
	_ = joinH("a", "b")
	_ = joinHorizontal3("a", "b", "c")
	_ = joinHorizontalLipgloss3("a", "b", "c")
	_ = stripAnsiRough("x")
	_ = lipglossHeight("")
	_ = compactInt(5)
	_ = compactInt(1500)
	_ = compactInt(2_000_000)
}

func TestCoverageRenderStatusAndRender(t *testing.T) {
	m := fullySeededModel(t)

	m.Screen = types.ScreenConnections
	_ = m.getStatusBar()
	m.Screen = types.ScreenHelp
	_ = m.getStatusBar()
	m.Screen = types.ScreenBrowser
	m.Loading = true
	m.StatusMsg = "ok"
	m.Err = errors.New("boom " + strings.Repeat("x", 200))
	_ = m.getStatusBar()
	m.Loading = false
	m.StatusMsg = ""
	m.Err = nil
	_ = m.getStatusBar()
	m.Screen = types.ScreenExport
	m.Loading = true
	m.StatusMsg = "hi"
	m.Err = errors.New("e")
	m.ReadOnly = true
	_ = m.getStatusBar()
	m.Screen = types.ScreenLogs
	_ = m.getStatusBar()
	m.Screen = types.ScreenFavorites
	_ = m.getStatusBar()
	m.Screen = types.ScreenDatabases
	_ = m.getStatusBar()

	m.Width = 40
	m.Height = 10
	_ = m.render()
	m.Width = 0
	m.Height = 0
	m.Screen = types.ScreenConnections
	_ = m.render()
	m.Width = 120
	m.Height = 40
	m.Screen = types.ScreenBrowser
	m.StatusMsg = "status"
	_ = m.render()
	m.Screen = types.ScreenHelp
	_ = m.render()
	m.Screen = types.ScreenExport
	_ = m.render()
	_ = m.View()
	m.Width = 0
	_ = m.viewHelp()
}

func TestCoverageModelHelpers(t *testing.T) {
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := fullySeededModel(t)
	m.Cmds = cmd.NewCommands(cfg, testutil.NewMockPG())

	cli := types.Connection{Name: "cli", Host: "localhost", Port: 5432}
	m.CLIConnection = &cli
	m.Version = "0.1.0"
	cmdInit := m.Init()
	if cmdInit == nil {
		t.Fatal("Init should return cmds")
	}
	m2 := NewModel()
	m2.Version = "dev"
	_ = m2.Init()

	for _, mode := range []types.SSLMode{
		types.SSLModeDisable, types.SSLModeAllow, types.SSLModePrefer,
		types.SSLModeRequire, types.SSLModeVerifyCA, types.SSLModeVerifyFull,
		"", "unknown",
	} {
		_ = sslIndex(mode)
	}

	m.pushQueryHistory("")
	m.pushQueryHistory("  SELECT 1  ")
	m.pushQueryHistory("SELECT 1")
	m.pushQueryHistory("SELECT 2")
	for i := 0; i < 35; i++ {
		m.pushQueryHistory(fmt.Sprintf("q%d", i))
	}
	if len(m.QueryHistory) > 30 {
		t.Fatalf("history cap: %d", len(m.QueryHistory))
	}

	m.SQLCompleter = nil
	m.rebuildSQLCompleter()
	if m.SQLCompleter == nil {
		t.Fatal("rebuild should allocate")
	}

	m.SchemaCols = nil
	m.cacheDetailColumns(types.TableDetail{})
	m.cacheDetailColumns(types.TableDetail{
		Object:  types.SchemaObject{Schema: "public", Name: "t"},
		Columns: []types.ColumnInfo{{Name: "a"}, {Name: ""}, {Name: "b"}},
	})
	if len(m.SchemaCols["public.t"]) != 2 {
		t.Fatalf("cols=%v", m.SchemaCols["public.t"])
	}

	m.QueryArea = nil
	if m.acceptQuerySuggestion() {
		t.Fatal("nil area")
	}
	ta := textarea.New()
	ta.SetValue("SELECT * FROM pro")
	ta.SetCursorColumn(len("SELECT * FROM pro"))
	m.QueryArea = &ta
	m.QuerySuggests = nil
	if m.acceptQuerySuggestion() {
		t.Fatal("empty suggests")
	}
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "products", Kind: types.ObjectTable},
		{Schema: "public", Name: "users", Kind: types.ObjectTable},
	}
	m.rebuildSQLCompleter()
	m.QuerySuggests = []string{"products", "profiles"}
	m.QuerySuggestIdx = 0
	if !m.acceptQuerySuggestion() {
		t.Fatal("expected accept")
	}

	m.KindEnabled = nil
	if !m.kindChecked(navTables) {
		t.Fatal("default tables")
	}
	if m.kindChecked(navViews) {
		t.Fatal("views off by default")
	}
	m.KindEnabled = defaultKindFilters()
	_ = m.enabledObjectKinds()
	m.KindEnabled[navViews] = true
	_ = m.enabledObjectKinds()

	for _, n := range []NavSection{
		navTables, navViews, navSequences, navFunctions, navTypes, navExtensions,
		navQuery, navActivity, navERD, navServer, navDatabases, NavSection(99),
	} {
		_ = n.String()
		_ = n.ObjectKind()
	}
	_ = sidebarNavItems()

	m.Objects = fullySeededModel(t).Objects
	m.ObjectFilter = ""
	m.FilterInput.SetValue("")
	m.FilterActive = false
	if len(m.filteredObjects()) != len(m.Objects) {
		t.Fatal("no filter")
	}
	m.ObjectFilter = "user"
	_ = m.filteredObjects()
	m.ObjectFilter = "table"
	_ = m.filteredObjects()
	m.ObjectFilter = "public.user"
	objs := m.filteredObjects()
	if len(objs) == 0 {
		t.Fatal("dot filter should match full name")
	}
	m.FilterActive = true
	m.FilterInput.SetValue("order")
	_ = m.objectSearchQuery()
	_ = m.filteredObjects()

	m.Objects = nil
	m.ObjectFilter = ""
	m.FilterActive = false
	m.FilterInput.SetValue("")
	if _, ok := m.selectedObject(); ok {
		t.Fatal("empty")
	}
	m.Objects = []types.SchemaObject{{Schema: "public", Name: "t", Kind: types.ObjectTable}}
	m.SelectedObjIdx = 99
	if o, ok := m.selectedObject(); !ok || o.Name != "t" {
		t.Fatalf("clamp select: %v %v", o, ok)
	}

	cur := &types.SchemaObject{Schema: "public", Name: "orders", Kind: types.ObjectTable}
	_ = objectIdentityMatch(nil, *cur)
	_ = objectIdentityMatch(cur, types.SchemaObject{Schema: "public", Name: "orders", Kind: types.ObjectType})
	_ = objectIdentityMatch(cur, types.SchemaObject{Schema: "public", Name: "orders", Kind: types.ObjectTable})
	_ = objectIdentityMatch(cur, types.SchemaObject{Schema: "other", Name: "orders"})
	_ = objectIdentityMatch(&types.SchemaObject{Schema: "public", Name: "orders"}, types.SchemaObject{Schema: "public", Name: "orders"})

	m = m.setCurrentObject(m.Objects[0])
	m = m.clearObjectContent()
	if m.CurrentObject != nil {
		t.Fatal("cleared")
	}
	m = m.resetToBrowserList()
	if m.Screen != types.ScreenBrowser || m.Focus != focusSidebar {
		t.Fatal("reset")
	}

	m = fullySeededModel(t)
	m = m.syncSidebarCursorToObject()
	m.SelectedObjIdx = 999
	m = m.syncSidebarCursorToObject()
	m = m.pinSidebarCursorToKind(navViews)
	m = m.pinSidebarCursorToKind(navTables)
	m.Schemas = nil
	m = m.pinSidebarCursorToKind(navFunctions)
	m = m.pinSidebarCursorToKind(NavSection(99))

	m = fullySeededModel(t)
	m = m.pinSidebarCursorToSchema(0)
	m = m.pinSidebarCursorToSchema(2)
	m = m.pinSidebarCursorToSchema(99)

	m = m.pinSidebarCursorToKind(navTables)
	m = m.pinSidebarAfterObjectsLoad()
	m = m.pinSidebarCursorToSchema(m.SelectedSchema)
	m = m.pinSidebarAfterObjectsLoad()
	m.SelectedSchema = 0
	m = m.pinSidebarCursorToSchema(2)
	m = m.pinSidebarAfterObjectsLoad()
	m = m.syncSidebarCursorToObject()
	m = m.pinSidebarAfterObjectsLoad()
	rows := m.buildSidebarRows()
	for i, r := range rows {
		if r.kind == sbTool {
			m.SidebarCursor = i
			break
		}
	}
	m = m.pinSidebarAfterObjectsLoad()
	m.Schemas = nil
	m.Objects = nil
	m = m.pinSidebarAfterObjectsLoad()

	m = fullySeededModel(t)
	if _, ok := m.currentSchemaInfo(); !ok {
		t.Fatal("current schema")
	}
	m.CurrentSchema = "missing"
	m.SelectedSchema = 1
	if s, ok := m.currentSchemaInfo(); !ok || s.Name != "billing" {
		t.Fatalf("fallback schema: %v %v", s, ok)
	}
	m.Schemas = nil
	if _, ok := m.currentSchemaInfo(); ok {
		t.Fatal("no schemas")
	}

	objs = []types.SchemaObject{
		{Schema: "public", Name: "users", Kind: types.ObjectTable},
	}
	if objectInList(objs, nil) {
		t.Fatal("nil")
	}
	if !objectInList(objs, &objs[0]) {
		t.Fatal("found")
	}
	missing := types.SchemaObject{Schema: "public", Name: "nope", Kind: types.ObjectTable}
	if objectInList(objs, &missing) {
		t.Fatal("missing")
	}

	for _, sc := range []types.Screen{
		types.ScreenBrowser, types.ScreenTableData, types.ScreenConnections, types.ScreenHelp,
	} {
		m.Screen = sc
		_ = m.isWorkspaceScreen()
	}
}

func TestCoverageRefreshQuerySuggestions(t *testing.T) {
	m := fullySeededModel(t)
	m.QueryArea = nil
	m.refreshQuerySuggestions()
	ta := textarea.New()
	m.QueryArea = &ta
	m.SQLCompleter = nil
	m.refreshQuerySuggestions()
	m.rebuildSQLCompleter()

	ta.SetValue("xyz")
	ta.SetCursorColumn(3)
	m.refreshQuerySuggestions()

	ta.SetValue("SELECT * FROM ")
	ta.SetCursorColumn(len("SELECT * FROM "))
	m.refreshQuerySuggestions()

	ta.SetValue("   ")
	ta.SetCursorColumn(3)
	m.refreshQuerySuggestions()

	ta.SetValue("SELECT * FROM use")
	ta.SetCursorColumn(len("SELECT * FROM use"))
	m.refreshQuerySuggestions()
}

func TestCoverageCloseRemainingBranches(t *testing.T) {
	m := fullySeededModel(t)

	// detailTabs: non-relation with columns + CreateSQL
	ext := types.SchemaObject{Schema: "public", Name: "pgcrypto", Kind: types.ObjectExtension}
	d := types.TableDetail{
		Object:    ext,
		Columns:   []types.ColumnInfo{{Name: "x", DataType: "text"}},
		CreateSQL: "CREATE EXTENSION pgcrypto;",
		Props:     []types.DetailProp{{Label: "v", Value: "1"}},
	}
	tabs := m.detailTabs(d)
	if len(tabs) < 2 {
		t.Fatalf("tabs=%v", tabs)
	}
	m = m.setCurrentObject(ext)
	m.TableDetail = d
	for i := range tabs {
		m.DetailTab = i
		_ = m.viewTableDetailContent(80, 20)
	}

	// renderDetailDefinition line truncation
	longSQL := strings.Repeat("line\n", 40)
	_ = m.renderDetailDefinition(types.TableDetail{CreateSQL: longSQL}, 40, 8)

	// Init: cmds only, no CLI
	m3 := NewModel()
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c2.json"))
	if err != nil {
		t.Fatal(err)
	}
	m3.Cmds = cmd.NewCommands(cfg, testutil.NewMockPG())
	m3.CLIConnection = nil
	_ = m3.Init()

	// getStatusBar empty parts on workspace
	m.Screen = types.ScreenBrowser
	m.Loading = false
	m.StatusMsg = ""
	m.Err = nil
	if bar := m.getStatusBar(); bar != "" {
		// may still be empty
		_ = bar
	}
	// default status branch empty
	m.Screen = types.ScreenExport
	m.Loading = false
	m.StatusMsg = ""
	m.Err = nil
	m.CurrentConn = nil
	m.CurrentDatabase = ""
	m.CurrentSchema = ""
	m.ReadOnly = false
	_ = m.getStatusBar()

	// viewLogs with many lines (scroll window)
	m.Logs = types.NewLogWriter()
	for i := 0; i < 40; i++ {
		_, _ = m.Logs.Write([]byte(fmt.Sprintf("log line %d\n", i)))
	}
	m.Height = 12
	m.Width = 80
	_ = m.viewLogs()

	// viewDatabasesHeader version truncation + " on "
	m.CurrentConn = &types.Connection{Name: "c", Host: "h", Port: 1}
	m.ServerInfo.Version = "PostgreSQL 16.13 on x86_64-pc-linux-gnu " + strings.Repeat("x", 40)
	m.ReadOnly = true
	m.Width = 40
	_ = m.viewDatabasesHeader()

	// viewContentPane default screen
	m.Screen = types.ScreenHelp
	_ = m.viewContentPane(40, 20)

	// viewConnections empty after having cards — section frame pad path
	m.Screen = types.ScreenConnections
	m.Connections = nil
	m.Width = 30 // pad path
	_ = m.viewConnections()

	// renderResultTable: cursor col clamp, empty nShow edge, short rows
	res := types.QueryResult{
		Columns: []string{"a"},
		Rows:    [][]string{{}, {"1"}},
	}
	_ = m.renderResultTable(res, 0, 5, 5, 5)
	_ = m.renderResultTable(res, -1, -1, 5, 100)

	// pinSidebarAfterObjectsLoad on schema row matching SelectedSchema
	m = fullySeededModel(t)
	m.SelectedSchema = 2
	m = m.pinSidebarCursorToSchema(2)
	m = m.pinSidebarAfterObjectsLoad()

	// cachedColumnsFor name-only key
	m.SchemaCols = map[string][]string{"users": {"id"}}
	_ = m.cachedColumnsFor(types.SchemaObject{Schema: "other", Name: "users"})

	// renderObjectPreviewBody default kind actions
	_ = m.renderObjectPreviewBody(types.SchemaObject{Schema: "s", Name: "x", Kind: "weird"}, 40, 15)

	// refreshQuerySuggestions WHERE/SELECT contexts
	ta := textarea.New()
	m.QueryArea = &ta
	m.rebuildSQLCompleter()
	ta.SetValue("SELECT ")
	ta.SetCursorColumn(len("SELECT "))
	m.refreshQuerySuggestions()
	ta.SetValue("SELECT * FROM users WHERE ")
	ta.SetCursorColumn(len("SELECT * FROM users WHERE "))
	m.refreshQuerySuggestions()
	ta.SetValue("INSERT INTO ")
	ta.SetCursorColumn(len("INSERT INTO "))
	m.refreshQuerySuggestions()
}
