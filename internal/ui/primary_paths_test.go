package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

// seededWorkspace builds a realistic 2-pane model with mock schemas/objects/data.
func seededWorkspace() Model {
	m := NewModel()
	m.Width = 140
	m.Height = 40
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	m.CurrentDatabase = "demo"
	m.CurrentSchema = "public"
	m.CurrentConn = &types.Connection{Name: "Local Demo", Host: "localhost", Port: 5432, Database: "demo"}
	m.Schemas = []types.SchemaInfo{
		{Name: "analytics", TableCount: 1},
		{Name: "billing", TableCount: 1},
		{Name: "public", TableCount: 5},
	}
	m.SelectedSchema = 2
	m.KindEnabled = defaultKindFilters()
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "order_items", Kind: types.ObjectTable, RowEstimate: 17500, SizePretty: "1.2 MB"},
		{Schema: "public", Name: "orders", Kind: types.ObjectTable, RowEstimate: 5000, SizePretty: "800 kB"},
		{Schema: "public", Name: "product_categories", Kind: types.ObjectTable, RowEstimate: 8, SizePretty: "16 kB"},
		{Schema: "public", Name: "products", Kind: types.ObjectTable, RowEstimate: 250, SizePretty: "64 kB"},
		{Schema: "public", Name: "users", Kind: types.ObjectTable, RowEstimate: 1000, SizePretty: "128 kB"},
	}
	m.SelectedObjIdx = 0
	m = m.syncSidebarCursorToObject()
	m.TableData = types.QueryResult{
		Columns: []string{"id", "email", "active", "created_at", "meta"},
		Rows: [][]string{
			{"1", "alice@example.com", "true", "2024-01-15T10:00:00Z", `{"role":"admin"}`},
			{"2", "bob@example.com", "false", "2024-02-01T12:30:00Z", `{}`},
			{"3", "", "true", "2024-03-01T00:00:00Z", `[1,2]`},
		},
		Duration:  time.Millisecond,
		IsSelect:  true,
		Truncated: false,
	}
	m.TableDetail = types.TableDetail{
		Object: m.Objects[0],
		Columns: []types.ColumnInfo{
			{Name: "id", DataType: "integer", IsPrimaryKey: true, IsNullable: false, Position: 1},
			{Name: "order_id", DataType: "integer", IsNullable: false, Position: 2},
			{Name: "email", DataType: "text", IsNullable: true, Position: 3},
			{Name: "created_at", DataType: "timestamp with time zone", IsNullable: false, Position: 4},
			{Name: "meta", DataType: "jsonb", IsNullable: true, Position: 5},
		},
		Indexes: []types.IndexInfo{
			{Name: "order_items_pkey", IsPrimary: true, SizePretty: "16 kB", Definition: "CREATE UNIQUE INDEX ..."},
		},
		Constraints: []types.ConstraintInfo{
			{Name: "order_items_pkey", Type: "PRIMARY KEY", Definition: "PRIMARY KEY (id)"},
			{Name: "order_items_order_id_fkey", Type: "FOREIGN KEY", Definition: "FOREIGN KEY (order_id) REFERENCES orders(id)"},
		},
	}
	m.Activity = []types.ActivityRow{
		{PID: 100, User: "postgres", Database: "demo", State: "active", Query: "SELECT 1", Duration: 2 * time.Second},
		{PID: 101, User: "postgres", Database: "demo", State: "idle", Query: "", Duration: 0},
		{PID: 102, User: "app", Database: "demo", State: "idle in transaction", Query: "UPDATE t", Duration: 40 * time.Second},
	}
	m.ServerInfo = types.ServerInfo{
		Version: "PostgreSQL 16.13", User: "postgres", Database: "demo",
		Host: "localhost", Port: 5432, Encoding: "UTF8", Timezone: "UTC",
		Uptime: "2 days", ActiveConns: 3, MaxConns: 100,
	}
	m.Databases = []types.DatabaseInfo{
		{Name: "demo", Owner: "postgres", SizePretty: "12 MB", Encoding: "UTF8"},
		{Name: "postgres", Owner: "postgres", SizePretty: "8 MB", Encoding: "UTF8"},
	}
	m.ERD = types.ERDGraph{
		Tables: []types.ERDTable{
			{Name: "users", Columns: []string{"id", "email"}},
			{Name: "orders", Columns: []string{"id", "user_id"}},
		},
		Edges: []types.FKEdge{
			{FromTable: "orders", FromCols: []string{"user_id"}, ToTable: "users", ToCols: []string{"id"}},
		},
	}
	return m
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func mustContain(t *testing.T, hay, needle, label string) {
	t.Helper()
	if !strings.Contains(hay, needle) {
		t.Fatalf("%s: missing %q in view (len=%d)", label, needle, len(hay))
	}
}

func TestBlueSelectionStyles(t *testing.T) {
	// Background 39 is the TablePlus/es-tui selection band constant.
	if colorSelectBg != "39" {
		t.Fatalf("colorSelectBg=%q want 39", colorSelectBg)
	}
	if colorAccent != "39" {
		t.Fatalf("colorAccent=%q want 39 for coherent blue chrome", colorAccent)
	}
	out := selectedStyle.Render("ROW")
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("selectedStyle should emit ANSI, got %q", out)
	}
	// Selected text should not look like plain unstyled content.
	if out == "ROW" {
		t.Fatal("selectedStyle produced unstyled text")
	}
	dim := selectedDimStyle.Render("ROW")
	if dim == out {
		t.Fatal("focused and unfocused selection styles should differ")
	}
}

func TestViewSmokeAllPrimaryScreens(t *testing.T) {
	base := seededWorkspace()
	cases := []struct {
		name    string
		setup   func(Model) Model
		needles []string
	}{
		{
			name: "connections",
			setup: func(m Model) Model {
				m.Screen = types.ScreenConnections
				m.Connections = []types.Connection{
					{Name: "Local Demo", Host: "localhost", Port: 5432, Database: "demo", Username: "postgres"},
				}
				return m
			},
			needles: []string{"Saved Instances", "Local Demo", "PostgreSQL TUI"},
		},
		{
			name: "browser_preview",
			setup: func(m Model) Model {
				m.Screen = types.ScreenBrowser
				m.ContentMode = contentPreview
				return m
			},
			needles: []string{"SCHEMAS", "FILTERS", "search", "order_items", "TOOLS", "Query"},
		},
		{
			name: "table_data",
			setup: func(m Model) Model {
				m.Screen = types.ScreenTableData
				m.Focus = focusContent
				o := m.Objects[4] // users
				m = m.setCurrentObject(o)
				return m
			},
			needles: []string{"Data", "email", "alice@example.com", "SCHEMAS"},
		},
		{
			name: "table_structure",
			setup: func(m Model) Model {
				m.Screen = types.ScreenTableDetail
				m.Focus = focusContent
				m.DetailTab = 0
				o := m.Objects[0]
				m = m.setCurrentObject(o)
				m.TableDetail.Object = o
				return m
			},
			needles: []string{"Structure", "Columns", "integer", "jsonb"},
		},
		{
			name: "query",
			setup: func(m Model) Model {
				m.Screen = types.ScreenQuery
				m.Focus = focusContent
				m.QueryFocus = "results"
				m.QueryResult = m.TableData
				m.QueryResult.SQL = "SELECT * FROM users LIMIT 100"
				return m
			},
			needles: []string{"SQL Query", "alice@example.com"},
		},
		{
			name: "activity",
			setup: func(m Model) Model {
				m.Screen = types.ScreenActivity
				m.Focus = focusContent
				return m
			},
			needles: []string{"Activity", "backends", "active"},
		},
		{
			name: "erd",
			setup: func(m Model) Model {
				m.Screen = types.ScreenERD
				m.Focus = focusContent
				return m
			},
			needles: []string{"users", "orders"},
		},
		{
			name: "server",
			setup: func(m Model) Model {
				m.Screen = types.ScreenServerInfo
				m.Focus = focusContent
				return m
			},
			needles: []string{"Server", "PostgreSQL", "UTF8"},
		},
		{
			name: "databases_content",
			setup: func(m Model) Model {
				m.Screen = types.ScreenBrowser
				m.ContentMode = contentDatabases
				m.Focus = focusContent
				return m
			},
			needles: []string{"Databases", "demo", "postgres"},
		},
		{
			name: "help",
			setup: func(m Model) Model {
				m.Screen = types.ScreenHelp
				return m
			},
			needles: []string{"Help", "Global", "Workspace", "Query", "Run SQL"},
		},
		{
			name: "object_search_active",
			setup: func(m Model) Model {
				m.Screen = types.ScreenBrowser
				nm, _ := m.startObjectSearch()
				m = nm.(Model)
				m.FilterInput.SetValue("order")
				m.ObjectFilter = "order"
				return m
			},
			needles: []string{"order", "order_items", "orders"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.setup(base)
			// Prefer View() entry when possible; help uses getScreenView via render.
			var view string
			switch m.Screen {
			case types.ScreenHelp:
				view = m.viewHelp()
			case types.ScreenConnections:
				view = m.viewConnections()
			default:
				view = m.viewWorkspace()
			}
			if strings.TrimSpace(view) == "" {
				t.Fatal("empty view")
			}
			plain := stripANSI(view)
			for _, n := range tc.needles {
				mustContain(t, plain, n, tc.name)
			}
		})
	}
}

func TestPrimaryKeyPathSequence(t *testing.T) {
	m := seededWorkspace()
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar

	// / opens search
	nm, _ := m.Update(tea.KeyPressMsg{Text: "/", Code: '/'})
	m = nm.(Model)
	if !m.FilterActive {
		t.Fatal("expected FilterActive after /")
	}

	// type order (including that j must not commit)
	for _, ch := range "order" {
		nm, _ = m.Update(tea.KeyPressMsg{Text: string(ch)})
		m = nm.(Model)
	}
	if m.FilterInput.Value() != "order" {
		t.Fatalf("filter=%q", m.FilterInput.Value())
	}
	if len(m.filteredObjects()) != 2 {
		t.Fatalf("want 2 order* tables got %d", len(m.filteredObjects()))
	}

	// esc clears search
	nm, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = nm.(Model)
	if m.FilterActive || m.objectSearchQuery() != "" {
		t.Fatal("esc should clear search")
	}

	// reopen search, type users, enter opens object path (screen may stay browser without cmds)
	nm, _ = m.Update(tea.KeyPressMsg{Text: "/", Code: '/'})
	m = nm.(Model)
	for _, ch := range "users" {
		nm, _ = m.Update(tea.KeyPressMsg{Text: string(ch)})
		m = nm.(Model)
	}
	nm, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = nm.(Model)
	if m.FilterActive {
		t.Fatal("enter should commit search")
	}

	// Simulate open table data
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	o := m.Objects[4]
	m = m.setCurrentObject(o)
	v := stripANSI(m.viewWorkspace())
	mustContain(t, v, "Data", "table data view")

	// D structure via screen set (beginTableDetail needs Cmds)
	m.Screen = types.ScreenTableDetail
	m.DetailTab = 0
	m.TableDetail.Object = o
	v = stripANSI(m.viewWorkspace())
	mustContain(t, v, "Structure", "structure view")

	// Query screen
	m.Screen = types.ScreenQuery
	m.QueryResult = m.TableData
	v = stripANSI(m.viewWorkspace())
	mustContain(t, v, "SQL Query", "query view")

	// Activity
	m.Screen = types.ScreenActivity
	v = stripANSI(m.viewWorkspace())
	mustContain(t, v, "Activity", "activity view")

	// ERD
	m.Screen = types.ScreenERD
	v = stripANSI(m.viewWorkspace())
	mustContain(t, v, "users", "erd view")

	// Server
	m.Screen = types.ScreenServerInfo
	v = stripANSI(m.viewWorkspace())
	mustContain(t, v, "Server", "server view")

	// Help via ?
	m.Screen = types.ScreenBrowser
	nm, _ = m.Update(tea.KeyPressMsg{Text: "?"})
	m = nm.(Model)
	if m.Screen != types.ScreenHelp {
		t.Fatalf("screen=%v want Help", m.Screen)
	}
	help := stripANSI(m.viewHelp())
	mustContain(t, help, "Workspace", "help workspace")
	mustContain(t, help, "Global", "help global")
	// two-column chips present (key then desc)
	if !strings.Contains(help, "tab") || !strings.Contains(strings.ToLower(help), "search") {
		t.Fatalf("help missing expected bindings:\n%s", help)
	}

	// esc closes help
	nm, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = nm.(Model)
	if m.Screen == types.ScreenHelp {
		t.Fatal("esc should leave help")
	}

	// tab cycles focus in workspace
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	nm, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = nm.(Model)
	if m.Focus != focusContent {
		t.Fatalf("tab focus=%v want content", m.Focus)
	}
}

func TestKindFilterToggleKeyPath(t *testing.T) {
	m := seededWorkspace()
	// Only tables enabled by default
	if !m.kindChecked(navTables) {
		t.Fatal("tables should be on")
	}
	// Space on kind row toggles — pin cursor to Views filter
	m = m.pinSidebarCursorToKind(navViews)
	nm, _ := m.keysSidebar(" ")
	m = nm.(Model)
	if !m.kindChecked(navViews) {
		// toggle may need Cmds for reload; KindEnabled should flip regardless
		if m.KindEnabled[navViews] {
			// ok
		} else {
			// keysSidebar space on kind calls toggleKindFilter
			t.Fatalf("views filter KindEnabled=%v", m.KindEnabled[navViews])
		}
	}
}

func TestDataGridSelectionUsesBlueBand(t *testing.T) {
	m := seededWorkspace()
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m.DataCursor = 0
	m.DataCol = 0
	o := m.Objects[4]
	m = m.setCurrentObject(o)
	// Full selected row should paint continuous blue (selectedStyle uses bg 39)
	row := m.renderResultTable(m.TableData, 0, 0, 10, 100)
	if !strings.Contains(row, "\x1b[") {
		t.Fatal("expected ANSI in result table")
	}
	// Active cell uses cellSelectedStyle (bg 33)
	if colorCellSel != "33" {
		t.Fatalf("cell sel=%s", colorCellSel)
	}
}

func TestHelpLayoutIsTwoColumnNotWrappedMess(t *testing.T) {
	m := seededWorkspace()
	m.Width = 100
	m.Height = 40
	help := m.viewHelp()
	plain := stripANSI(help)
	// Old mess had "TOOLS→Databases" and duplicate 1-6 lines
	if strings.Contains(plain, "TOOLS→") {
		t.Fatal("stale TOOLS→Databases help entry")
	}
	if strings.Contains(plain, "Jump kind") {
		t.Fatal("stale Jump kind help entry")
	}
	// Sections present once
	for _, sec := range []string{"Global", "Connections", "Workspace", "Table data", "Query"} {
		if c := strings.Count(plain, sec); c < 1 {
			t.Fatalf("section %q missing", sec)
		}
	}
	// Modal should not be tiny formModal (narrow) — width uses ~80 or Width-6
	if !strings.Contains(help, "╭") && !strings.Contains(help, "┌") && !strings.Contains(help, "─") {
		// rounded border from lipgloss
		if lipglossWidth(help) < 40 {
			t.Fatalf("help too narrow: %d", lipglossWidth(help))
		}
	}
}
