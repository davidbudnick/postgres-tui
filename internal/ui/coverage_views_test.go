package ui

import (
	"testing"

	"charm.land/bubbles/v2/textarea"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestAllScreenViews(t *testing.T) {
	m := covModel(t)
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
	for _, s := range screens {
		m.Screen = s
		m.ContentMode = contentPreview
		m.Focus = focusContent
		_ = m.getScreenView()
		_ = m.render()
		_ = m.View()
		_ = m.getStatusBar()
	}

	// too small terminal
	m.Width, m.Height = 40, 10
	_ = m.render()
	// zero size
	m.Width, m.Height = 0, 0
	m.Screen = types.ScreenConnections
	_ = m.render()
	m.Width, m.Height = 140, 40

	// connections empty + with items
	m.Screen = types.ScreenConnections
	m.Connections = nil
	_ = m.viewConnections()
	m.Connections = []types.Connection{
		{Name: "A", Host: "h", Port: 1, Username: "u", Database: "d", ReadOnly: true, Color: "red", Group: "g"},
		{Name: "B", Host: "h2", Port: 2},
	}
	m.SelectedConnIdx = 0
	_ = m.viewConnections()
	m.SelectedConnIdx = 1
	m.Loading = true
	m.ConnectionError = "boom"
	_ = m.viewConnections()
	_ = m.connectionsKeyHelp(80)
	_ = m.connectionsKeyHelp(20)
	_ = m.connectionsKeyHelp(0)
	_ = m.buildStatsBar()
	m.Version = "dev"
	_ = m.buildStatsBar()
	m.Version = "v0.0.2-2-g3a8997a"
	_ = m.buildStatsBar()
	m.Version = "v1.2.3"
	_ = m.buildStatsBar()
	_ = m.renderConnectionCard(m.Connections[0], true, 40)
	_ = m.renderConnectionCard(m.Connections[1], false, 20)

	// form
	m.Screen = types.ScreenAddConnection
	m.ConnFocusIdx = 0
	_ = m.viewConnectionForm()
	m.Screen = types.ScreenEditConnection
	m.ConnFocusIdx = 3
	m.ConnReadOnly = true
	m.ConnSSLIdx = 2
	_ = m.viewConnectionForm()
	_ = m.viewTestConnection()
	m.TestConnResult = "OK"
	_ = m.viewTestConnection()
	m.ConfirmType = "delete"
	m.ConfirmData = m.Connections[0]
	_ = m.viewConfirmDelete()

	// databases
	m.Screen = types.ScreenDatabases
	_ = m.viewDatabases()
	_ = m.viewDatabasesHeader()
	_ = m.viewDatabasesChips()
	_ = m.viewDatabasesPanel(40, 20)
	m.Databases = nil
	_ = m.viewDatabasesPanel(40, 20)
	_ = colWidth(10, 4, 0, 0)
	_ = colWidth(10, 4, 3, 8)

	// workspace modes
	m = covModel(t)
	m.Screen = types.ScreenBrowser
	for _, mode := range []ContentMode{contentPreview, contentDatabases, contentSchema} {
		m.ContentMode = mode
		_ = m.viewWorkspace()
		_ = m.viewSidebar(30, 20)
		_ = m.viewContentPane(80, 20)
		_ = m.viewBrowserPreview(40, 20)
		_ = m.viewBrowserPreviewContent(40, 20)
		_ = m.viewSchemaOverviewContent(40, 20)
		_ = m.viewDatabasesContent(40, 20)
	}
	_ = m.viewWorkspaceHeader()
	_ = m.viewWorkspaceFooter()
	_ = m.viewBrowserHeader()
	_ = m.viewBrowserFooter()
	_ = lipglossHeight("a\nb\nc")
	_ = stripAnsiRough("\x1b[31mred\x1b[0m")
	_ = joinH("a", "b")
	_ = joinHorizontal("a", "b")
	_ = joinHorizontalLipgloss("a", "b")
	_ = joinHorizontalLipgloss3("a", "b", "c")
	_ = compactInt(0)
	_ = compactInt(999)
	_ = compactInt(1500)
	_ = compactInt(2_000_000)

	// object preview kinds
	for _, o := range m.Objects {
		m.CurrentObject = &o
		m.TableDetail.Object = o
		_ = m.renderObjectPreviewBody(o, 40, 20)
		_ = m.cachedColumnsFor(o)
	}
	m.CurrentObject = nil
	_ = m.renderObjectPreviewBody(types.SchemaObject{}, 40, 10)

	// table data / detail / query
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m = m.setCurrentObject(m.Objects[0])
	_ = m.viewTableDataContent(60, 20)
	_ = m.viewTableData()
	m.DataCursor = 0
	m.DataCol = 1
	_ = m.viewTableDataContent(60, 20)
	m.TableData.Rows = nil
	_ = m.viewTableDataContent(60, 20)

	m.Screen = types.ScreenTableDetail
	m.TableDetail = types.TableDetail{
		Object: m.Objects[0],
		Columns: []types.ColumnInfo{
			{Name: "id", DataType: "integer", IsPrimaryKey: true, IsNullable: false, Default: "nextval", Comment: "pk"},
			{Name: "email", DataType: "text", IsNullable: true},
		},
		Indexes: []types.IndexInfo{
			{Name: "users_pkey", IsPrimary: true, IsUnique: true, Definition: "CREATE UNIQUE INDEX", SizePretty: "8 kB"},
			{Name: "users_email_key", IsUnique: true, Definition: "CREATE UNIQUE INDEX email", SizePretty: "8 kB"},
		},
		Constraints: []types.ConstraintInfo{
			{Name: "users_pkey", Type: "PRIMARY KEY", Definition: "PRIMARY KEY (id)"},
			{Name: "fk", Type: "FOREIGN KEY", Definition: "FOREIGN KEY (x) REFERENCES t(id)"},
			{Name: "u", Type: "UNIQUE", Definition: "UNIQUE (email)"},
			{Name: "c", Type: "CHECK", Definition: "CHECK (true)"},
		},
		Triggers:  []string{"trg"},
		CreateSQL: "CREATE VIEW v AS SELECT 1",
		Props:     []types.DetailProp{{Label: "owner", Value: "postgres"}},
	}
	for tab := 0; tab < 4; tab++ {
		m.DetailTab = tab
		_ = m.viewTableDetailContent(70, 25)
	}
	_ = m.viewTableDetail()
	_ = m.detailTabs(m.TableDetail)
	_ = m.renderDetailProps(m.TableDetail, 40)
	_ = m.renderDetailDefinition(m.TableDetail, 40, 20)
	_ = m.renderDetailColumns(m.TableDetail, 40, 20)
	// sequence-like props only
	m.TableDetail.Columns = nil
	m.DetailTab = 0
	_ = m.viewTableDetailContent(70, 25)

	m.Screen = types.ScreenQuery
	m.QueryFocus = "editor"
	_ = m.viewQueryContent(70, 25)
	m.QueryFocus = "results"
	_ = m.viewQueryContent(70, 25)
	m.QuerySuggests = []string{"users", "orders", "products"}
	m.QuerySuggestIdx = 1
	_ = m.renderQuerySuggestions(40)
	_ = m.viewQuery()
	_ = m.renderResultTable(m.QueryResult, 60, 10, 0, 0)
	_ = m.renderResultTable(types.QueryResult{}, 60, 10, 0, 0)
	_ = m.renderResultTable(types.QueryResult{
		Columns: []string{"a", "b"},
		Rows:    [][]string{{"1", "2"}, {"3", "null"}, {"true", "2024-01-01"}},
	}, 40, 5, 1, 1)

	m.Screen = types.ScreenActivity
	_ = m.viewActivityContent(70, 20)
	m.Activity = nil
	_ = m.viewActivityContent(70, 20)
	_ = m.viewActivity()
	_ = shortState("active")
	_ = shortState("idle in transaction (aborted)")
	_ = shortState("")
	_ = compactDuration(0)
	_ = compactDuration(500 * 1e6) // nanoseconds? Duration is time.Duration
	_ = compactDuration(2e9)
	_ = compactDuration(60e9)
	_ = compactDuration(3600e9)

	m.Screen = types.ScreenERD
	_ = m.viewERDContent(80, 30)
	m.ERD = types.ERDGraph{}
	_ = m.viewERDContent(80, 30)
	_ = m.viewERD()

	m.Screen = types.ScreenServerInfo
	_ = m.viewServerInfoContent(60, 20)
	m.ServerInfo = types.ServerInfo{}
	_ = m.viewServerInfoContent(60, 20)
	_ = m.viewServerInfo()

	m.Screen = types.ScreenHelp
	m.Width = 100
	_ = m.viewHelp()
	m.Width = 0
	_ = m.viewHelp()
	_ = helpBindingLine("enter", "select", 12)
	_ = helpBindingLine("verylongkeyname", "desc", 4)

	m.Width = 120
	m.Screen = types.ScreenLogs
	_ = m.viewLogs()
	m.Logs = nil
	_ = m.viewLogs()
	m.Logs = types.NewLogWriter()
	for i := 0; i < 30; i++ {
		_, _ = m.Logs.Write([]byte("line"))
	}
	_ = m.viewLogs()

	m.Screen = types.ScreenFavorites
	m.Favorites = nil
	_ = m.viewFavorites()
	m.Favorites = []types.Favorite{
		{Object: "users", Schema: "public", Database: "demo"},
		{Object: "orders", Schema: "public", Database: "demo"},
	}
	m.SelectedFavIdx = 0
	_ = m.viewFavorites()
	m.SelectedFavIdx = 1
	_ = m.viewFavorites()

	m.Screen = types.ScreenExport
	_ = m.viewExport()
	m.Inputs = nil
	_ = m.viewExport()

	m.Screen = types.ScreenCommandPalette
	m.Inputs = &ModelInputs{}
	m.Inputs.PaletteInput = createTextInput("cmd", 30)
	m.PaletteItems = defaultPaletteItems()
	_ = m.viewCommandPalette()
	m.Inputs.PaletteInput.SetValue("query")
	_ = m.viewCommandPalette()
	m.Inputs = nil
	_ = m.viewCommandPalette()

	// status bar variants
	m = covModel(t)
	m.Screen = types.ScreenBrowser
	m.Loading = true
	m.StatusMsg = "hi"
	m.Err = errString("e")
	_ = m.getStatusBar()
	m.Loading = false
	m.Err = nil
	m.ReadOnly = true
	_ = m.getStatusBar()
	m.Screen = types.ScreenConnections
	_ = m.getStatusBar()
	m.Screen = types.ScreenTableData
	m.StatusMsg = ""
	_ = m.getStatusBar()
}

type errString string

func (e errString) Error() string { return string(e) }

func TestOpenQueryAndCompleterViews(t *testing.T) {
	m := covModel(t)
	m.Screen = types.ScreenBrowser
	nm, _ := m.openQuery()
	m = nm.(Model)
	_ = m.viewQueryContent(80, 30)
	m.QuerySuggests = []string{"a", "b"}
	m.QuerySuggestIdx = 0
	_ = m.renderQuerySuggestions(30)
	// force QueryArea path
	ta := textarea.New()
	ta.SetValue("SELECT * FROM ")
	m.QueryArea = &ta
	m.refreshQuerySuggestions()
	_ = m.viewQueryContent(80, 30)
}

func TestViewBranchesRemaining(t *testing.T) {
	m := covModel(t)

	// connection form SSL/RO focus + err + test result variants + confirm variants
	m.Screen = types.ScreenAddConnection
	m.ConnFocusIdx = connFieldSSL
	_ = m.viewConnectionForm()
	m.ConnFocusIdx = connFieldReadOnly
	m.ConnReadOnly = true
	m.Err = errString("bad")
	_ = m.viewConnectionForm()
	m.Screen = types.ScreenTestConnection
	m.Loading = true
	_ = m.viewTestConnection()
	m.Loading = false
	m.TestConnResult = ""
	_ = m.viewTestConnection()
	m.TestConnResult = "fail"
	_ = m.viewTestConnection()
	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "connection"
	m.ConfirmData = types.Connection{Name: "x"}
	_ = m.viewConfirmDelete()
	m.ConfirmData = "nope"
	_ = m.viewConfirmDelete()
	m.ConfirmType = "other"
	_ = m.viewConfirmDelete()

	// databases chips/header/panel edges
	m = covModel(t)
	m.Screen = types.ScreenDatabases
	m.ReadOnly = true
	m.Loading = true
	_ = m.viewDatabases()
	_ = m.viewDatabasesHeader()
	_ = m.viewDatabasesChips()
	_ = m.viewDatabasesPanel(40, 10)
	m.Loading = false
	m.Databases = nil
	_ = m.viewDatabases()
	m.Databases = []types.DatabaseInfo{
		{Name: "demo", Owner: "o", SizePretty: "1", Encoding: "UTF8"},
		{Name: "other", Owner: "o", SizePretty: "1", Encoding: "UTF8"},
		{Name: "z", Owner: "o", SizePretty: "1", Encoding: "UTF8"},
	}
	m.CurrentDatabase = "demo"
	m.SelectedDBIdx = 2
	_ = m.viewDatabasesPanel(50, 12)
	_ = colWidth(100, 10, 16, 48)
	_ = colWidth(20, 10, 16, 48)

	// sidebar preview / schema / databases content empty/loading
	m = covModel(t)
	m.Screen = types.ScreenBrowser
	m.ContentMode = contentPreview
	m.Objects = nil
	_ = m.viewBrowserPreview(40, 20)
	_ = m.viewBrowserPreviewContent(40, 20)
	m.Objects = seededWorkspace().Objects
	m.CurrentObject = &m.Objects[0]
	_ = m.viewBrowserPreview(40, 20)
	m.ContentMode = contentSchema
	m.Loading = true
	m.Objects = nil
	_ = m.viewSchemaOverviewContent(40, 20)
	m.Loading = false
	_ = m.viewSchemaOverviewContent(40, 20)
	m.Objects = seededWorkspace().Objects
	m.KindEnabled[navViews] = true
	m.Focus = focusContent
	_ = m.viewSchemaOverviewContent(60, 25)
	m.ContentMode = contentDatabases
	m.Databases = nil
	_ = m.viewDatabasesContent(40, 20)
	m.Loading = true
	_ = m.viewDatabasesContent(40, 20)
	m.Loading = false
	m.Databases = seededWorkspace().Databases
	m.SelectedDBIdx = 1
	_ = m.viewDatabasesContent(40, 20)

	// object preview kinds + cached columns
	m = covModel(t)
	for _, k := range []types.ObjectKind{
		types.ObjectTable, types.ObjectView, types.ObjectMatView,
		types.ObjectSequence, types.ObjectFunction, types.ObjectType, types.ObjectExtension, "",
	} {
		o := types.SchemaObject{Schema: "public", Name: "x", Kind: k, Owner: "o", SizePretty: "1 kB"}
		_ = m.renderObjectPreviewBody(o, 60, 20)
		o.RowEstimate = 100
		_ = m.renderObjectPreviewBody(o, 60, 20)
	}
	m.SchemaCols = nil
	o := types.SchemaObject{Schema: "public", Name: "x", Kind: types.ObjectTable}
	_ = m.cachedColumnsFor(o)
	m.SchemaCols = map[string][]string{"public.x": {"id"}}
	_ = m.cachedColumnsFor(o)
	m.SchemaCols = map[string][]string{"x": {"id"}}
	_ = m.cachedColumnsFor(o)
	m.SchemaCols = map[string][]string{}
	m.CurrentObject = &o
	m.TableDetail.Object = o
	m.TableDetail.Columns = []types.ColumnInfo{{Name: "id"}, {Name: ""}}
	_ = m.cachedColumnsFor(o)

	// workspace header RO / long version / narrow
	m = covModel(t)
	m.ReadOnly = true
	m.ServerInfo.Version = "PostgreSQL 16.13 on x86_64-pc-linux-gnu, compiled by gcc"
	m.Width = 40
	_ = m.viewWorkspaceHeader()
	m.CurrentConn = nil
	m.CurrentDatabase = ""
	_ = m.viewWorkspaceHeader()
	for _, sc := range []types.Screen{
		types.ScreenBrowser, types.ScreenTableData, types.ScreenTableDetail,
		types.ScreenQuery, types.ScreenActivity, types.ScreenERD, types.ScreenServerInfo,
	} {
		m.Screen = sc
		m.ContentMode = contentPreview
		_ = m.viewWorkspaceFooter()
	}
	m.Screen = types.ScreenBrowser
	m.ContentMode = contentSchema
	_ = m.viewWorkspaceFooter()
	m.ContentMode = contentDatabases
	_ = m.viewWorkspaceFooter()

	// ERD empty / list / route
	m.Screen = types.ScreenERD
	m.ERD = types.ERDGraph{}
	_ = m.viewERDContent(80, 30)
	m.ERD = types.ERDGraph{Tables: []types.ERDTable{{Name: "only", Columns: []string{"id", "a", "b", "c", "d", "e", "f"}}}}
	_ = m.viewERDContent(40, 12)
	m.ERD = types.ERDGraph{
		Tables: []types.ERDTable{
			{Name: "p", Columns: []string{"id"}},
			{Name: "c", Columns: []string{"id", "p_id"}},
		},
		Edges: []types.FKEdge{{FromTable: "c", FromCols: []string{"p_id"}, ToTable: "p", ToCols: []string{"id"}}},
	}
	_ = m.viewERDContent(80, 30)
	_ = renderERDList(m.ERD, 5, true)
	_ = maxLayerWidth([][]string{{"aa"}, {"b"}})
	_ = maxLayerWidth(nil)
	layers := erdLayers(m.ERD)
	_ = erdBarycenterOrder(m.ERD, layers)
	// put wire-on-wire attr bump
	c := newERDCanvas(20, 10)
	c.put(1, 1, '│', 1)
	c.put(1, 1, '─', 2)
	c.put(2, 2, 'A', 0)
	c.put(2, 2, '│', 1)
	parent := erdNode{x: 1, y: 1, w: 8, h: 4, name: "p", table: types.ERDTable{Name: "p", Columns: []string{"id"}}}
	child := erdNode{x: 1, y: 7, w: 8, h: 3, name: "c", table: types.ERDTable{Name: "c", Columns: []string{"id"}}}
	c.drawBox(parent, nil)
	c.drawBox(child, map[string]bool{"c.id": true})
	c.routeEdge(parent, child, "fk", 0)
	c.routeEdge(parent, erdNode{x: 1, y: 2, w: 4, h: 2}, "early", 0)
	_ = c.lines()
}
