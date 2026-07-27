package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/textarea"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

// TestCoverageViewEdgeCases pushes remaining view*.go branches toward 100%.
func TestCoverageViewEdgeCases(t *testing.T) {
	m := covModel(t)

	// ── getScreenView for every Screen* (+ unknown) ──────────────────
	screens := []types.Screen{
		types.ScreenConnections,
		types.ScreenAddConnection,
		types.ScreenEditConnection,
		types.ScreenDatabases,
		types.ScreenBrowser,
		types.ScreenTableData,
		types.ScreenTableDetail,
		types.ScreenQuery,
		types.ScreenActivity,
		types.ScreenERD,
		types.ScreenServerInfo,
		types.ScreenHelp,
		types.ScreenConfirmDelete,
		types.ScreenTestConnection,
		types.ScreenLogs,
		types.ScreenFavorites,
		types.ScreenExport,
		types.ScreenCommandPalette,
		types.Screen(99),
	}
	for _, sc := range screens {
		m.Screen = sc
		m.Width, m.Height = 120, 40
		_ = m.getScreenView()
	}

	// Weird sizes: width 0, tiny height, empty lists, loading flags.
	for _, sz := range [][2]int{{0, 0}, {0, 10}, {40, 5}, {10, 40}, {50, 15}, {200, 60}} {
		m.Width, m.Height = sz[0], sz[1]
		m.Screen = types.ScreenBrowser
		m.ContentMode = contentPreview
		_ = m.getScreenView()
		_ = m.viewWorkspace()
		_ = m.viewSidebar(sz[0]/4, max(sz[1]-4, 1))
		_ = m.viewContentPane(max(sz[0]/2, 8), max(sz[1]-4, 1))
	}

	// ── viewConnections: empty / many / loading / error / narrow top ─
	m = covModel(t)
	m.Screen = types.ScreenConnections
	m.Width = 1
	m.Connections = nil
	_ = m.viewConnections()
	m.Width = 40
	m.Connections = nil
	_ = m.viewConnections()
	// Many cards so n > maxVisible (scroll indicator).
	m.Connections = make([]types.Connection, 40)
	for i := range m.Connections {
		m.Connections[i] = types.Connection{
			Name: fmt.Sprintf("c%02d", i), Host: "h", Port: 5432 + i,
			Username: "u", Database: "d", ReadOnly: i%2 == 0,
			SSLMode: types.SSLModeDisable,
		}
	}
	m.SelectedConnIdx = 35
	m.Height = 12
	m.Loading = true
	m.ConnectionError = "dial tcp: refused"
	_ = m.viewConnections()
	m.Loading = false
	m.ConnectionError = ""
	// Force pad<=0 branch: top string longer than sectionInner via huge len.
	// Practical ceiling: huge digit count isn't feasible via slice len, but
	// exercise Width extremes and empty+selected edges.
	m.Width = 0
	_ = m.viewConnections()
	m.Width = 100
	m.SelectedConnIdx = 0
	_ = m.viewConnections()

	// ── viewConnectionForm: SSL/RO focus, edit title, err, short inputs ─
	m.Screen = types.ScreenAddConnection
	m.ConnInputs = createConnectionInputs()
	m.ConnFocusIdx = 0
	_ = m.viewConnectionForm()
	m.ConnFocusIdx = connFieldSSL
	m.ConnSSLIdx = 0
	_ = m.viewConnectionForm()
	m.ConnFocusIdx = connFieldReadOnly
	m.ConnReadOnly = false
	_ = m.viewConnectionForm()
	m.ConnReadOnly = true
	m.Err = errString("validation")
	_ = m.viewConnectionForm()
	m.Screen = types.ScreenEditConnection
	m.ConnFocusIdx = 1
	_ = m.viewConnectionForm()
	// Truncated ConnInputs slice
	m.ConnInputs = m.ConnInputs[:1]
	_ = m.viewConnectionForm()
	m.ConnInputs = nil
	_ = m.viewConnectionForm()

	// ── viewDatabasesHeader: RO + long version + no conn ─────────────
	m = covModel(t)
	m.Screen = types.ScreenDatabases
	m.ReadOnly = true
	m.ServerInfo.Version = "PostgreSQL 16.13 on x86_64-pc-linux-gnu, compiled by gcc (GCC) 11.2.0, 64-bit"
	m.Width = 30
	_ = m.viewDatabasesHeader()
	m.Width = 200
	_ = m.viewDatabasesHeader()
	m.CurrentConn = nil
	m.ServerInfo.Version = strings.Repeat("V", 50) // no " on " substring, still truncate
	_ = m.viewDatabasesHeader()
	m.ReadOnly = false
	m.ServerInfo.Version = ""
	_ = m.viewDatabasesHeader()
	_ = m.viewDatabases()
	_ = m.viewDatabasesChips()
	_ = m.viewDatabasesPanel(0, 0)
	_ = m.viewDatabasesPanel(80, 20)

	// ── denseListFrame / help / activity / query edges ───────────────
	m = covModel(t)
	m.Width = 0
	_ = m.denseListFrame("Title", "body line\nline2", keyDesc{"q", "quit"})
	m.Width = 80
	m.Height = 10
	m.Loading = true
	m.StatusMsg = "ok"
	m.Err = errString("e")
	_ = m.denseListFrame("Logs", "a\nb\nc", keyDesc{"esc", "back"})
	_ = m.denseListFrame("Empty", "")
	m.Loading = false
	m.StatusMsg = ""
	m.Err = nil

	m.Screen = types.ScreenHelp
	m.Width = 0
	_ = m.viewHelp()
	m.Width = 40
	_ = m.viewHelp()
	m.Width = 120
	_ = m.viewHelp()

	m.Screen = types.ScreenActivity
	m.Loading = true
	m.Activity = nil
	_ = m.viewActivityContent(80, 20)
	m.Loading = false
	_ = m.viewActivityContent(40, 10)
	m.Activity = []types.ActivityRow{
		{PID: 1, User: "u", Database: "d", State: "active", Query: "SELECT 1", Duration: time.Second},
		{PID: 2, User: "v", Database: "d", State: "idle in transaction", Query: "", Duration: 90 * time.Second},
		{PID: 3, State: "idle", Duration: 0},
	}
	_ = m.viewActivityContent(100, 30)
	_ = m.viewActivityContent(20, 5)

	// Query: truncated non-select, empty, loading, suggestions, tiny width
	m.Screen = types.ScreenQuery
	m.Focus = focusContent
	m.QueryFocus = "results"
	m.QueryResult = types.QueryResult{
		SQL: "UPDATE t SET x=1", IsSelect: false, RowsAffected: 3,
		Duration: time.Millisecond, Truncated: true,
	}
	_ = m.viewQueryContent(80, 25)
	m.QueryResult = types.QueryResult{
		SQL: "SELECT 1", IsSelect: true, Columns: []string{"?column?"},
		Rows: [][]string{{"1"}}, Truncated: true, Duration: time.Millisecond,
	}
	_ = m.viewQueryContent(80, 25)
	m.QueryResult = types.QueryResult{}
	m.Loading = true
	_ = m.viewQueryContent(60, 15)
	m.Loading = false
	_ = m.viewQueryContent(60, 15)
	ta := textarea.New()
	ta.SetValue("SELECT ")
	m.QueryArea = &ta
	m.QueryFocus = "editor"
	m.Focus = focusContent
	m.QuerySuggests = []string{"users", "orders", "products", "items", "meta"}
	m.QuerySuggestIdx = 2
	_ = m.viewQueryContent(30, 20)
	_ = m.viewQueryContent(0, 0)
	_ = m.viewQueryContent(100, 40)

	// ── viewTableDetailContent / detailTabs edge branches ────────────
	m = covModel(t)
	m.Screen = types.ScreenTableDetail
	m.Focus = focusContent

	// Kind empty on detail, filled from CurrentObject.
	cur := types.SchemaObject{Schema: "public", Name: "users", Kind: types.ObjectTable}
	m.CurrentObject = &cur
	m.TableDetail = types.TableDetail{
		Object: types.SchemaObject{Schema: "public", Name: "users"}, // Kind ""
		Columns: []types.ColumnInfo{
			{Name: "id", DataType: "int", IsPrimaryKey: true},
		},
		Indexes: []types.IndexInfo{
			{Name: "users_pkey", IsPrimary: true, SizePretty: "8 kB", Definition: "CREATE UNIQUE INDEX"},
			{Name: "users_email_key", IsUnique: true, Definition: "CREATE UNIQUE INDEX email"},
			{Name: "plain_idx", Definition: "CREATE INDEX"},
		},
		Constraints: []types.ConstraintInfo{
			{Name: "c1", Type: "PRIMARY KEY", Definition: "PRIMARY KEY (id)"},
		},
		CreateSQL: "CREATE TABLE users (id int);",
	}
	_ = m.detailTabs(m.TableDetail)
	for tab := 0; tab < 6; tab++ {
		m.DetailTab = tab
		_ = m.viewTableDetailContent(80, 25)
	}
	// Empty indexes / constraints messages
	m.TableDetail.Indexes = nil
	m.DetailTab = 1 // Indexes
	_ = m.viewTableDetailContent(80, 20)
	m.TableDetail.Constraints = nil
	m.DetailTab = 2
	_ = m.viewTableDetailContent(80, 20)

	// Non-relation (function): Info / Definition / optional Columns
	fn := types.SchemaObject{Schema: "public", Name: "fn", Kind: types.ObjectFunction}
	m.CurrentObject = &fn
	m.TableDetail = types.TableDetail{
		Object:    types.SchemaObject{Schema: "public", Name: "fn"}, // Kind "" → from CurrentObject
		CreateSQL: "CREATE FUNCTION fn() RETURNS int AS $$ SELECT 1 $$ LANGUAGE sql;",
		Props:     []types.DetailProp{{Label: "lang", Value: "sql"}},
	}
	_ = m.detailTabs(m.TableDetail)
	for tab := 0; tab < 4; tab++ {
		m.DetailTab = tab
		_ = m.viewTableDetailContent(90, 20)
	}
	// Non-relation with columns prepended
	m.TableDetail.Columns = []types.ColumnInfo{{Name: "ret", DataType: "int"}}
	_ = m.detailTabs(m.TableDetail)
	m.DetailTab = 0
	_ = m.viewTableDetailContent(80, 20)
	// Mismatched CurrentObject → "Detail not loaded"
	other := types.SchemaObject{Schema: "public", Name: "other", Kind: types.ObjectTable}
	m.CurrentObject = &other
	_ = m.viewTableDetailContent(80, 20)
	// Loading
	m.Loading = true
	_ = m.viewTableDetailContent(80, 20)
	m.Loading = false
	// No CurrentObject, empty names → "Object"
	m.CurrentObject = nil
	m.TableDetail = types.TableDetail{}
	_ = m.viewTableDetailContent(80, 20)
	// Sequence-like: columns empty + props on Columns tab
	seq := types.SchemaObject{Schema: "public", Name: "s", Kind: types.ObjectSequence}
	m.CurrentObject = &seq
	m.TableDetail = types.TableDetail{
		Object: seq,
		Props:  []types.DetailProp{{Label: "last_value", Value: "99"}},
	}
	m.DetailTab = 0
	_ = m.viewTableDetailContent(70, 15)
	_ = m.detailTabs(m.TableDetail)

	// ── renderResultTable branches ───────────────────────────────────
	m = covModel(t)
	// empty columns
	_ = m.renderResultTable(types.QueryResult{}, 0, 0, 10, 80)
	// tiny maxWidth (<20 → clamped)
	_ = m.renderResultTable(types.QueryResult{
		Columns: []string{"a"},
		Rows:    [][]string{{"1"}},
	}, 0, 0, 5, 5)
	// Row with more cells than columns (i >= nCol break)
	_ = m.renderResultTable(types.QueryResult{
		Columns: []string{"a", "b"},
		Rows:    [][]string{{"1", "2", "extra", "x"}},
	}, 0, 0, 10, 80)
	// Wide first column forces used+need > avail with visible==0
	wide := strings.Repeat("W", 30)
	_ = m.renderResultTable(types.QueryResult{
		Columns: []string{wide, "b", "c"},
		Rows:    [][]string{{wide, "1", "2"}},
	}, 0, 0, 8, 22)
	// Many columns → nShow < len(Columns) overflow note
	cols := make([]string, 20)
	row := make([]string, 20)
	for i := range cols {
		cols[i] = fmt.Sprintf("col%d", i)
		row[i] = fmt.Sprintf("v%d", i)
	}
	_ = m.renderResultTable(types.QueryResult{Columns: cols, Rows: [][]string{row, row}}, 0, 5, 10, 40)
	// Sample rows > 40 for width sampling
	many := make([][]string, 50)
	for i := range many {
		many[i] = []string{"x", "", "true"}
	}
	// Cursor on empty cell (not selected col) + zebra empty cells
	_ = m.renderResultTable(types.QueryResult{
		Columns: []string{"a", "b", "c"},
		Rows:    many,
	}, 1, 0, 12, 60)
	// cursorCol out of range / negative
	_ = m.renderResultTable(types.QueryResult{
		Columns: []string{"a", "b"},
		Rows:    [][]string{{"", "2"}, {"3", ""}, {"5", "6"}},
	}, 0, 99, 8, 40)
	_ = m.renderResultTable(types.QueryResult{
		Columns: []string{"a", "b"},
		Rows:    [][]string{{"1", ""}, {"", "2"}},
	}, 0, -3, 8, 40)
	// Selected row with empty non-cursor cell (italic branch)
	_ = m.renderResultTable(types.QueryResult{
		Columns: []string{"a", "b", "c"},
		Rows:    [][]string{{"1", "", "3"}},
	}, 0, 0, 6, 50)
	// Huge DataOffset → large rnW, tiny avail; still draw
	m.DataOffset = 1_000_000_000
	_ = m.renderResultTable(types.QueryResult{
		Columns: []string{"only"},
		Rows:    [][]string{{"v"}},
	}, 0, 0, 4, 20)
	m.DataOffset = 0
	// Single very wide column on narrow table (first-col shrink path already covered;
	// defensive nShow==0 / "?" header are effectively unreachable via public API).
	_ = m.renderResultTable(types.QueryResult{
		Columns: []string{strings.Repeat("Z", 40)},
		Rows:    [][]string{{strings.Repeat("z", 40)}},
	}, 0, 0, 5, 20)

	// ── sidebar / browser preview / content pane / schema / dbs ─────
	m = covModel(t)
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	m.Width, m.Height = 140, 40

	// Filter active search bar
	m.FilterActive = true
	m.FilterInput.SetValue("us")
	_ = m.viewSidebar(32, 30)
	m.FilterActive = false
	m.ObjectFilter = "zzz_nomatch"
	m.Objects = seededWorkspace().Objects
	_ = m.viewSidebar(32, 30) // (no matches)
	m.ObjectFilter = "user"
	_ = m.viewSidebar(32, 30)
	m.ObjectFilter = ""
	// Empty schemas
	m.Schemas = nil
	_ = m.viewSidebar(32, 20)
	m.Schemas = []types.SchemaInfo{{Name: "public", TableCount: 1}}
	// Empty objects + empty search → "(none)" (not no-matches)
	m.Objects = nil
	m.ObjectFilter = ""
	m.FilterActive = false
	m.FilterInput.SetValue("")
	m.Loading = false
	_ = m.viewSidebar(32, 25)
	// Single kind enabled → title = kind name
	m.KindEnabled = map[NavSection]bool{navTables: true}
	m.Objects = seededWorkspace().Objects
	_ = m.viewSidebar(32, 25)
	// Loading objects title
	m.Loading = true
	m.ContentMode = contentPreview
	_ = m.viewSidebar(32, 25)
	m.Loading = false
	// unfocused sidebar cursor (selectedDimStyle)
	m.Focus = focusContent
	m.SidebarCursor = 0
	_ = m.viewSidebar(32, 25)
	// width tiny (inner < 10)
	_ = m.viewSidebar(4, 10)
	// empty CurrentDatabase
	m.CurrentDatabase = ""
	_ = m.viewSidebar(28, 20)
	m.CurrentDatabase = "demo"

	// Active tool highlighting for each screen
	for _, sc := range []types.Screen{
		types.ScreenQuery, types.ScreenActivity, types.ScreenERD, types.ScreenServerInfo,
	} {
		m.Screen = sc
		_ = m.viewSidebar(32, 25)
	}
	m.Screen = types.ScreenBrowser
	m.ContentMode = contentDatabases
	_ = m.viewSidebar(32, 25)

	// viewBrowserPreview / Content with CurrentObject match + empty
	m.ContentMode = contentPreview
	m.Focus = focusContent
	m.Objects = seededWorkspace().Objects
	m.CurrentObject = &m.Objects[0]
	_ = m.viewBrowserPreview(40, 20)
	_ = m.viewBrowserPreviewContent(40, 20)
	_ = m.viewBrowserPreview(5, 5) // inner < 8
	_ = m.viewBrowserPreviewContent(5, 5)
	m.Objects = nil
	_ = m.viewBrowserPreview(40, 15)
	_ = m.viewBrowserPreviewContent(40, 15)

	// Content pane for every workspace screen + default
	m.Objects = seededWorkspace().Objects
	for _, sc := range []types.Screen{
		types.ScreenBrowser, types.ScreenTableData, types.ScreenTableDetail,
		types.ScreenQuery, types.ScreenActivity, types.ScreenERD, types.ScreenServerInfo,
		types.Screen(50),
	} {
		m.Screen = sc
		for _, mode := range []ContentMode{contentPreview, contentSchema, contentDatabases} {
			m.ContentMode = mode
			_ = m.viewContentPane(80, 25)
			_ = m.viewContentPane(10, 2) // innerH < 3
		}
	}

	// Schema overview: owner, loading, empty, single kind, focus variants
	m.Screen = types.ScreenBrowser
	m.ContentMode = contentSchema
	m.Schemas = []types.SchemaInfo{{Name: "public", Owner: "postgres", TableCount: 3, ViewCount: 1}}
	m.SelectedSchema = 0
	m.CurrentSchema = "public"
	m.Objects = seededWorkspace().Objects
	m.KindEnabled = defaultKindFilters()
	m.Focus = focusContent
	_ = m.viewSchemaOverviewContent(80, 30)
	m.Focus = focusSidebar
	_ = m.viewSchemaOverviewContent(80, 30)
	m.KindEnabled = map[NavSection]bool{navTables: true}
	_ = m.viewSchemaOverviewContent(80, 25)
	m.Loading = true
	_ = m.viewSchemaOverviewContent(60, 20)
	m.Loading = false
	m.Objects = nil
	_ = m.viewSchemaOverviewContent(60, 20)
	m.CurrentSchema = ""
	m.Schemas = nil
	_ = m.viewSchemaOverviewContent(60, 15)
	// objects with empty SizePretty / RowEstimate
	m.Schemas = []types.SchemaInfo{{Name: "public"}}
	m.CurrentSchema = "public"
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "a", Kind: types.ObjectTable},
		{Schema: "public", Name: "b", Kind: types.ObjectView, RowEstimate: 10, SizePretty: "1 kB"},
	}
	m.KindEnabled = defaultKindFilters()
	m.SelectedObjIdx = 1
	m.Focus = focusContent
	_ = m.viewSchemaOverviewContent(70, 20)

	// Databases content: empty / loading / selected
	m.ContentMode = contentDatabases
	m.Databases = nil
	_ = m.viewDatabasesContent(60, 20)
	m.Loading = true
	_ = m.viewDatabasesContent(60, 20)
	m.Loading = false
	m.Databases = seededWorkspace().Databases
	m.SelectedDBIdx = 0
	_ = m.viewDatabasesContent(80, 25)
	_ = m.viewDatabasesContent(5, 5)

	// ── viewWorkspaceHeader: RO, long version, narrow, no schema ─────
	m = covModel(t)
	m.ReadOnly = true
	m.ServerInfo.Version = "PostgreSQL 16.13 on x86_64-pc-linux-gnu, compiled by gcc"
	m.Width = 20
	_ = m.viewWorkspaceHeader()
	m.Width = 200
	_ = m.viewWorkspaceHeader()
	// Version without " on " but longer than 28
	m.ServerInfo.Version = strings.Repeat("PostgreSQL16.13-extra-build-info", 2)
	_ = m.viewWorkspaceHeader()
	m.CurrentSchema = ""
	m.CurrentDatabase = ""
	m.CurrentConn = nil
	m.ReadOnly = false
	m.ServerInfo.Version = ""
	_ = m.viewWorkspaceHeader()
	_ = m.viewWorkspaceFooter()
	_ = m.viewBrowserHeader()
	_ = m.viewBrowserFooter()

	// ── ERD: put / routeEdge / renderERDDiagram / layers / barycenter ─
	// put: out-of-bounds, box+wire, wire-on-wire attr bump (▼ is wire-only)
	c := newERDCanvas(12, 8)
	c.put(-1, -1, 'x', 0)
	c.put(100, 100, 'x', 0)
	c.put(0, 0, '┌', 1)
	c.put(0, 0, '│', 2) // box + wire → mergeWireBox
	c.put(1, 1, '▼', 1) // wire-only (not box)
	c.put(1, 1, '│', 3) // wire on wire + a > attr
	c.put(1, 1, '─', 1) // wire on wire, a <= attr (no bump)
	c.put(2, 2, 'A', 0)
	_ = c.lines()

	// routeEdge: straight down, left/right corners, label clamps, early exit
	parent := erdNode{x: 4, y: 0, w: 8, h: 4, name: "p", table: types.ERDTable{Name: "p", Columns: []string{"id"}}}
	// same x center → straight down
	childStraight := erdNode{x: 4, y: 8, w: 8, h: 3, name: "c", table: types.ERDTable{Name: "c", Columns: []string{"id"}}}
	c2 := newERDCanvas(20, 14)
	c2.drawBox(parent, nil)
	c2.drawBox(childStraight, nil)
	c2.routeEdge(parent, childStraight, "fk", 0)
	// early exit: child too close
	c2.routeEdge(parent, erdNode{x: 4, y: 3, w: 4, h: 2}, "early", 0)
	// rightward child + label
	childR := erdNode{x: 12, y: 9, w: 6, h: 3, name: "r"}
	c2.routeEdge(parent, childR, "user_id", 0)
	// leftward child
	childL := erdNode{x: 0, y: 9, w: 6, h: 3, name: "l"}
	c2.routeEdge(parent, childL, "x", 0)
	// empty label
	c2.routeEdge(parent, childStraight, "", 0)
	// label overflow near right edge + tight vertical gap (labY side path, cx > px)
	c3 := newERDCanvas(16, 12)
	pTight := erdNode{x: 10, y: 0, w: 6, h: 3, name: "pt"}
	chTight := erdNode{x: 12, y: 6, w: 4, h: 3, name: "ct"}
	c3.routeEdge(pTight, chTight, "long_label_name", 0)
	// label with negative-ish labX (negative node x)
	c3.routeEdge(
		erdNode{x: -4, y: 0, w: 4, h: 2},
		erdNode{x: -2, y: 6, w: 4, h: 2},
		"ab", 0)
	// cy = py+2, cx > px → labY side path (352-354) + midY clamps
	// parent h=2,y=0 → py=2; child y=4 → cy=py+2
	c3.routeEdge(
		erdNode{x: 0, y: 0, w: 6, h: 2, name: "a"},
		erdNode{x: 8, y: 4, w: 6, h: 2, name: "b"},
		"id", 0)
	// cx < px labY side path with same tight gap
	c3.routeEdge(
		erdNode{x: 8, y: 0, w: 6, h: 2},
		erdNode{x: 0, y: 4, w: 6, h: 2},
		"id", 0)
	// Force midY edge clamps with extreme coords (py high, cy just past)
	c3.routeEdge(
		erdNode{x: 2, y: 1, w: 4, h: 1}, // py=2
		erdNode{x: 2, y: 4, w: 4, h: 2}, // cy=4=py+2, same center x
		"z", 0)

	// renderERDDiagram: empty, isolated, empty cols, missing edge ends, reverse layer edge
	_ = renderERDDiagram(types.ERDGraph{}, 80)
	_ = renderERDDiagram(types.ERDGraph{
		Tables: []types.ERDTable{{Name: "solo", Columns: nil}, {Name: "wide", Columns: []string{"id", "a", "b", "c", "d", "e", "f"}}},
	}, 80)
	// max tables → list mode via viewERDContent
	m.Screen = types.ScreenERD
	m.Loading = true
	m.ERD = types.ERDGraph{}
	_ = m.viewERDContent(80, 20)
	m.Loading = false
	_ = m.viewERDContent(80, 20)
	// Many tables for list mode + scroll
	tabs := make([]types.ERDTable, erdMaxTables+2)
	edges := make([]types.FKEdge, 0, len(tabs))
	for i := range tabs {
		tabs[i] = types.ERDTable{Name: fmt.Sprintf("t%02d", i), Columns: []string{"id", "x_id"}}
		if i > 0 {
			edges = append(edges, types.FKEdge{
				FromTable: tabs[i].Name, FromCols: []string{"x_id"},
				ToTable: tabs[i-1].Name, ToCols: []string{"id"},
			})
		}
	}
	m.ERD = types.ERDGraph{Schema: "public", Tables: tabs, Edges: edges}
	m.ERDOffset = 5
	_ = m.viewERDContent(100, 12)
	_ = m.viewERDContent(20, 8) // list min width
	// Diagram mode with multi-layer graph + reverse/self/external edges
	g := types.ERDGraph{
		Schema: "public",
		Tables: []types.ERDTable{
			{Name: "users", Columns: []string{"id", "email"}},
			{Name: "orders", Columns: []string{"id", "user_id", "total"}},
			{Name: "items", Columns: []string{"id", "order_id"}},
			{Name: "orphan", Columns: []string{"id"}},
			{Name: "emptycols"},
		},
		Edges: []types.FKEdge{
			{FromTable: "orders", FromCols: []string{"user_id"}, ToTable: "users", ToCols: []string{"id"}},
			{FromTable: "items", FromCols: []string{"order_id"}, ToTable: "orders", ToCols: []string{"id"}},
			// reverse / same-layer-ish
			{FromTable: "users", FromCols: []string{"id"}, ToTable: "orders", ToCols: []string{"user_id"}},
			// self
			{FromTable: "users", FromCols: []string{"id"}, ToTable: "users", ToCols: []string{"id"}},
			// external table not in graph
			{FromTable: "orders", FromCols: []string{"user_id"}, ToTable: "ghost", ToCols: []string{"id"}},
			{FromTable: "ghost", FromCols: []string{"id"}, ToTable: "users", ToCols: []string{"id"}},
		},
	}
	_ = renderERDDiagram(g, 100)
	_ = renderERDDiagram(g, 40)
	_ = renderERDList(g, 5, true)
	_ = renderERDList(types.ERDGraph{Tables: g.Tables}, 40, true) // no edges

	// erdLayers: cycles (empty layer + stack hit), self/external skip, empty graph
	cycle := types.ERDGraph{
		Tables: []types.ERDTable{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		Edges: []types.FKEdge{
			{FromTable: "a", ToTable: "b"},
			{FromTable: "b", ToTable: "a"},
			{FromTable: "c", ToTable: "c"},       // self
			{FromTable: "a", ToTable: "missing"}, // external
			{FromTable: "missing", ToTable: "a"},
		},
	}
	layers := erdLayers(cycle)
	_ = erdBarycenterOrder(cycle, layers)
	_ = erdLayers(types.ERDGraph{})
	_ = erdLayers(types.ERDGraph{Tables: []types.ERDTable{{Name: "only"}}})

	// Multi-parent / no-parent / same-score barycenter ordering
	multi := types.ERDGraph{
		Tables: []types.ERDTable{
			{Name: "p1", Columns: []string{"id"}},
			{Name: "p2", Columns: []string{"id"}},
			{Name: "c1", Columns: []string{"id", "p1_id", "p2_id"}},
			{Name: "c2", Columns: []string{"id"}}, // no parents
			{Name: "c3", Columns: []string{"id", "p1_id"}},
		},
		Edges: []types.FKEdge{
			{FromTable: "c1", FromCols: []string{"p1_id"}, ToTable: "p1", ToCols: []string{"id"}},
			{FromTable: "c1", FromCols: []string{"p2_id"}, ToTable: "p2", ToCols: []string{"id"}},
			{FromTable: "c3", FromCols: []string{"p1_id"}, ToTable: "p1", ToCols: []string{"id"}},
			{FromTable: "c3", FromCols: []string{"p1_id"}, ToTable: "c3", ToCols: []string{"id"}}, // self skip
			// parent not in pos (edge to unknown) — n==0 score path
			{FromTable: "c2", FromCols: []string{"x"}, ToTable: "ghost", ToCols: []string{"id"}},
		},
	}
	ly := erdLayers(multi)
	_ = erdBarycenterOrder(multi, ly)
	// len(layers) < 2 early return
	_ = erdBarycenterOrder(multi, [][]string{{"p1"}})
	_ = erdBarycenterOrder(multi, nil)
	// Hand-built layers: parentless node in layer 1 → score = len(row)
	_ = erdBarycenterOrder(types.ERDGraph{
		Tables: []types.ERDTable{{Name: "a"}, {Name: "b"}, {Name: "c"}},
	}, [][]string{{"a"}, {"b", "c"}})
	// Equal scores → name tiebreak (two children of same single parent)
	tie := types.ERDGraph{
		Tables: []types.ERDTable{{Name: "p"}, {Name: "za"}, {Name: "aa"}},
		Edges: []types.FKEdge{
			{FromTable: "za", ToTable: "p"},
			{FromTable: "aa", ToTable: "p"},
		},
	}
	_ = erdBarycenterOrder(tie, erdLayers(tie))

	// Wide diagram: many nodes in one layer to stress maxX / box width
	wideG := types.ERDGraph{
		Tables: []types.ERDTable{
			{Name: "a1", Columns: []string{"id"}},
			{Name: "a2", Columns: []string{"id"}},
			{Name: "a3", Columns: []string{"id"}},
			{Name: "a4", Columns: []string{"id"}},
			{Name: "b1", Columns: []string{"id", "a_id"}},
		},
		Edges: []types.FKEdge{
			{FromTable: "b1", FromCols: []string{"a_id"}, ToTable: "a1", ToCols: []string{"id"}},
		},
	}
	_ = renderERDDiagram(wideG, 30)
	_ = renderERDDiagram(wideG, 120)

	m.ERD = g
	m.CurrentSchema = ""
	m.ERD.Schema = ""
	_ = m.viewERDContent(90, 30)
	m.ERDOffset = 100
	_ = m.viewERDContent(90, 8)

	// Final sweep: getScreenView again with seeded edge model
	m = covModel(t)
	m.ReadOnly = true
	m.ServerInfo.Version = "PostgreSQL 16.13 on x86_64-pc-linux-gnu, compiled by gcc"
	for _, sc := range screens {
		m.Screen = sc
		m.Width, m.Height = 100, 30
		switch sc {
		case types.ScreenBrowser:
			for _, mode := range []ContentMode{contentPreview, contentSchema, contentDatabases} {
				m.ContentMode = mode
				_ = m.getScreenView()
			}
		case types.ScreenTableDetail:
			for tab := 0; tab < 4; tab++ {
				m.DetailTab = tab
				_ = m.getScreenView()
			}
		default:
			_ = m.getScreenView()
		}
	}
}
