package ui

import (
	"errors"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/cmd"
	"github.com/davidbudnick/postgres-tui/internal/db"
	"github.com/davidbudnick/postgres-tui/internal/testutil"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func navModel(t *testing.T) Model {
	t.Helper()
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	m.Width = 140
	m.Height = 40
	m.Cmds = cmd.NewCommands(cfg, testutil.NewMockPG())
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	m.CurrentDatabase = "demo"
	m.CurrentSchema = "public"
	m.CurrentConn = &types.Connection{Name: "local", Host: "localhost", Port: 5432, Database: "demo"}
	m.Schemas = []types.SchemaInfo{
		{Name: "billing", TableCount: 1},
		{Name: "public", TableCount: 3},
	}
	m.SelectedSchema = 1
	m.KindEnabled = defaultKindFilters()
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "orders", Kind: types.ObjectTable},
		{Schema: "public", Name: "users", Kind: types.ObjectTable},
		{Schema: "public", Name: "active_users", Kind: types.ObjectView},
		{Schema: "public", Name: "id_seq", Kind: types.ObjectSequence},
		{Schema: "public", Name: "do_stuff", Kind: types.ObjectFunction},
		{Schema: "public", Name: "status", Kind: types.ObjectType},
		{Schema: "public", Name: "plpgsql", Kind: types.ObjectExtension},
	}
	m.SelectedObjIdx = 0
	m.Databases = []types.DatabaseInfo{
		{Name: "demo", Owner: "postgres", SizePretty: "12 MB"},
		{Name: "postgres", Owner: "postgres", SizePretty: "8 MB"},
		{Name: "analytics", Owner: "postgres", SizePretty: "20 MB"},
	}
	m.PageSize = 50
	return m.syncSidebarCursorToObject()
}

func navKey(s string) tea.KeyPressMsg {
	if s == "" {
		return tea.KeyPressMsg{}
	}
	return tea.KeyPressMsg{Text: s, Code: []rune(s)[0]}
}

func navPress(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func TestNav_KeysDatabases(t *testing.T) {
	m := navModel(t)
	m.Screen = types.ScreenDatabases
	m.SelectedDBIdx = 0

	for _, k := range []string{"j", "down", "k", "up", "g", "G", "r"} {
		nm, cmd := m.keysDatabases(k)
		m = nm.(Model)
		m.Screen = types.ScreenDatabases
		_ = cmd
	}
	_, cmd := m.keysDatabases("q")
	if cmd == nil {
		t.Fatal("q should quit")
	}

	// enter same database with objects already loaded
	m.SelectedDBIdx = 0
	m.CurrentDatabase = "demo"
	nm, cmd := m.keysDatabases("enter")
	m = nm.(Model)
	if cmd != nil {
		t.Fatal("same db with objects should not load schemas")
	}

	// enter same database with empty objects
	m.Objects = nil
	nm, cmd = m.keysDatabases("enter")
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("same db empty objects loads schemas")
	}

	// enter different database
	m = navModel(t)
	m.Screen = types.ScreenDatabases
	m.SelectedDBIdx = 1
	nm, cmd = m.keysDatabases("enter")
	if cmd == nil {
		t.Fatal("switch db")
	}
	_ = nm

	// esc with open database + empty objects
	m = navModel(t)
	m.Screen = types.ScreenDatabases
	m.Objects = nil
	nm, cmd = m.keysDatabases("esc")
	if cmd == nil {
		t.Fatal("esc reload schemas")
	}
	_ = nm

	// esc with open database + objects present
	m = navModel(t)
	m.Screen = types.ScreenDatabases
	nm, cmd = m.keysDatabases("esc")
	m = nm.(Model)
	if cmd != nil {
		t.Fatal("esc with objects")
	}

	// esc disconnect when no current database
	m.CurrentDatabase = ""
	nm, cmd = m.keysDatabases("esc")
	if cmd == nil {
		t.Fatal("esc disconnect")
	}
	m = nm.(Model)

	// esc without cmds
	m.Cmds = nil
	m.CurrentDatabase = ""
	nm, _ = m.keysDatabases("esc")
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("esc no cmds")
	}

	// empty databases branches
	m = navModel(t)
	m.Screen = types.ScreenDatabases
	m.Databases = nil
	for _, k := range []string{"j", "k", "G", "enter"} {
		nm, cmd = m.keysDatabases(k)
		m = nm.(Model)
		_ = cmd
	}
	m.Cmds = nil
	_, cmd = m.keysDatabases("r")
	if cmd != nil {
		t.Fatal("r no cmds")
	}
	_, cmd = m.keysDatabases("enter")
	if cmd != nil {
		t.Fatal("enter no cmds empty")
	}

	// via Update KeyPressMsg
	m = navModel(t)
	m.Screen = types.ScreenDatabases
	nm, _ = m.Update(navKey("j"))
	m = nm.(Model)
	if m.SelectedDBIdx != 1 {
		t.Fatalf("update j idx=%d", m.SelectedDBIdx)
	}
}

func TestNav_KeysBrowser(t *testing.T) {
	m := navModel(t)

	_, cmd := m.keysBrowser("q", navKey("q"))
	if cmd == nil {
		t.Fatal("q")
	}

	// esc clears object search first
	m.ObjectFilter = "ord"
	m.FilterInput.SetValue("ord")
	nm, _ := m.keysBrowser("esc", navPress(tea.KeyEscape))
	m = nm.(Model)
	if m.objectSearchQuery() != "" {
		t.Fatal("esc clear search")
	}

	// esc leaves databases/schema content
	for _, mode := range []ContentMode{contentDatabases, contentSchema} {
		m.ContentMode = mode
		m.Focus = focusContent
		nm, _ = m.keysBrowser("esc", navPress(tea.KeyEscape))
		m = nm.(Model)
		if m.ContentMode != contentPreview || m.Focus != focusSidebar {
			t.Fatalf("esc mode %v", mode)
		}
	}

	// esc disconnect / no cmds
	nm, cmd = m.keysBrowser("esc", navPress(tea.KeyEscape))
	if cmd == nil {
		t.Fatal("disconnect")
	}
	m.Cmds = nil
	nm, _ = m.keysBrowser("esc", navPress(tea.KeyEscape))
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("esc connections")
	}

	m = navModel(t)
	for _, k := range []string{"tab", "shift+tab", ";", ":", "A", "E", "i", "L", "r", "D", "1", "2", "3", "4", "5", "6"} {
		msg := navKey(k)
		if k == "tab" {
			msg = navPress(tea.KeyTab)
		}
		if k == "shift+tab" {
			msg = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
		}
		nm, cmd = m.keysBrowser(k, msg)
		m = nm.(Model)
		m.Screen = types.ScreenBrowser
		_ = cmd
	}

	// contentDatabases + focusContent routes to keysDatabasesInContent
	m.ContentMode = contentDatabases
	m.Focus = focusContent
	m.SelectedDBIdx = 0
	nm, _ = m.keysBrowser("j", navKey("j"))
	m = nm.(Model)
	if m.SelectedDBIdx != 1 {
		t.Fatal("browser -> databases content")
	}

	// r in databases content with/without cmds
	m.ContentMode = contentDatabases
	nm, cmd = m.keysBrowser("r", navKey("r"))
	if cmd == nil {
		t.Fatal("r databases")
	}
	m.Cmds = nil
	_, cmd = m.keysBrowser("r", navKey("r"))
	if cmd != nil {
		t.Fatal("r databases no cmds")
	}

	// A/i without cmds
	m = navModel(t)
	m.Cmds = nil
	_, cmd = m.keysBrowser("A", navKey("A"))
	if cmd != nil {
		t.Fatal("A no cmds")
	}
	_, cmd = m.keysBrowser("i", navKey("i"))
	if cmd != nil {
		t.Fatal("i no cmds")
	}

	// D with no selection
	m.Objects = nil
	_, cmd = m.keysBrowser("D", navKey("D"))
	if cmd != nil {
		t.Fatal("D empty")
	}

	// via Update
	m = navModel(t)
	nm, _ = m.Update(navKey("j"))
	m = nm.(Model)
	if m.SidebarCursor == 0 {
		t.Fatal("expected sidebar cursor move via Update")
	}
}

func TestNav_KeysDatabasesInContent(t *testing.T) {
	m := navModel(t)
	m.ContentMode = contentDatabases
	m.Focus = focusContent
	m.SelectedDBIdx = 0
	m.CurrentDatabase = "demo"

	for _, k := range []string{"j", "down", "k", "up", "g", "G", "r", "h", "left"} {
		nm, cmd := m.keysDatabasesInContent(k)
		m = nm.(Model)
		_ = cmd
	}

	// enter current database returns to preview
	m.SelectedDBIdx = 0
	m.CurrentDatabase = "demo"
	nm, cmd := m.keysDatabasesInContent("enter")
	m = nm.(Model)
	if m.ContentMode != contentPreview || m.Focus != focusSidebar {
		t.Fatal("enter same db")
	}
	if cmd != nil {
		t.Fatal("no switch cmd")
	}

	// enter other database
	m.ContentMode = contentDatabases
	m.Focus = focusContent
	m.SelectedDBIdx = 1
	nm, cmd = m.keysDatabasesInContent("enter")
	if cmd == nil {
		t.Fatal("switch")
	}
	_ = nm

	// empty / no cmds
	m.Databases = nil
	for _, k := range []string{"j", "k", "G", "enter"} {
		_, _ = m.keysDatabasesInContent(k)
	}
	m.Databases = []types.DatabaseInfo{{Name: "x"}}
	m.Cmds = nil
	_, cmd = m.keysDatabasesInContent("r")
	if cmd != nil {
		t.Fatal("r no cmds")
	}
	_, cmd = m.keysDatabasesInContent("enter")
	if cmd != nil {
		t.Fatal("enter no cmds")
	}
}

func TestNav_KeysSidebar(t *testing.T) {
	// empty-ish model still has kind+tool rows from buildSidebarRows
	m := NewModel()
	nm, cmd := m.keysSidebar("j")
	m = nm.(Model)
	_ = cmd

	m = navModel(t)
	for _, k := range []string{"j", "down", "k", "up", "g", "G"} {
		nm, _ = m.keysSidebar(k)
		m = nm.(Model)
	}

	// space/enter on kind filter
	m = m.pinSidebarCursorToKind(navViews)
	nm, cmd = m.keysSidebar(" ")
	if cmd == nil {
		t.Fatal("space kind")
	}
	m = nm.(Model)
	m = m.pinSidebarCursorToKind(navSequences)
	nm, cmd = m.keysSidebar("enter")
	if cmd == nil {
		t.Fatal("enter kind")
	}

	// activate object rows
	for _, k := range []string{"enter", "l", "right", "D"} {
		mm := navModel(t)
		mm.SelectedObjIdx = 0
		mm = mm.syncSidebarCursorToObject()
		nm, cmd = mm.keysSidebar(k)
		if cmd == nil {
			t.Fatalf("%s object", k)
		}
		_ = nm
	}

	// D on kind row does nothing
	m = navModel(t)
	m = m.pinSidebarCursorToKind(navTables)
	nm, cmd = m.keysSidebar("D")
	if cmd != nil {
		t.Fatal("D on kind")
	}

	// unknown key falls through
	_, _ = m.keysSidebar("x")

	// D on object with empty name
	m = navModel(t)
	m.Objects = []types.SchemaObject{{Schema: "public", Name: "", Kind: types.ObjectTable}}
	m.SelectedObjIdx = 0
	m = m.syncSidebarCursorToObject()
	nm, cmd = m.keysSidebar("D")
	if cmd != nil {
		t.Fatal("D empty name")
	}
	_ = nm
}

func TestNav_ActivateSidebarRowAllKinds(t *testing.T) {
	m := navModel(t)

	// schema row
	nm, cmd := m.activateSidebarRow(sidebarRow{kind: sbSchema, schema: 0})
	m = nm.(Model)
	if m.CurrentSchema != "billing" || cmd == nil {
		t.Fatalf("schema schema=%q cmd=%v", m.CurrentSchema, cmd != nil)
	}
	// schema OOB
	_, cmd = m.activateSidebarRow(sidebarRow{kind: sbSchema, schema: 99})
	if cmd != nil {
		t.Fatal("schema oob")
	}

	// object row
	m = navModel(t)
	nm, cmd = m.activateSidebarRow(sidebarRow{kind: sbObject, objIdx: 0})
	if cmd == nil {
		t.Fatal("object")
	}
	_ = nm

	// kind + nav
	nm, cmd = m.activateSidebarRow(sidebarRow{kind: sbKind, nav: navViews})
	if cmd == nil {
		t.Fatal("kind")
	}
	nm, cmd = m.activateSidebarRow(sidebarRow{kind: sbNav, nav: navFunctions})
	if cmd == nil {
		t.Fatal("nav")
	}

	// tools: query / activity / erd / server / databases
	for _, nav := range []NavSection{navQuery, navActivity, navERD, navServer, navDatabases} {
		mm := navModel(t)
		nm, cmd = mm.activateSidebarRow(sidebarRow{kind: sbTool, nav: nav})
		if nav != navQuery && cmd == nil {
			// query may return cmd for focus; activity/erd/server/databases need cmds
			t.Fatalf("tool %v expected cmd", nav)
		}
		_ = nm
	}

	// tools without cmds
	m = navModel(t)
	m.Cmds = nil
	for _, nav := range []NavSection{navActivity, navServer} {
		_, cmd = m.activateSidebarRow(sidebarRow{kind: sbTool, nav: nav})
		if cmd != nil {
			t.Fatalf("tool %v no cmds", nav)
		}
	}
	nm, cmd = m.activateSidebarRow(sidebarRow{kind: sbTool, nav: navERD})
	m = nm.(Model)
	if m.Screen != types.ScreenERD || cmd != nil {
		t.Fatal("erd no cmds")
	}
	nm, cmd = m.activateSidebarRow(sidebarRow{kind: sbTool, nav: navDatabases})
	m = nm.(Model)
	if m.ContentMode != contentDatabases || cmd != nil {
		t.Fatal("databases no cmds")
	}

	// unknown tool nav
	_, cmd = m.activateSidebarRow(sidebarRow{kind: sbTool, nav: NavSection(99)})
	if cmd != nil {
		t.Fatal("unknown tool")
	}
}

func TestNav_ToggleKindFilter(t *testing.T) {
	m := navModel(t)

	// nil KindEnabled initializes defaults
	m.KindEnabled = nil
	nm, cmd := m.toggleKindFilter(navViews)
	m = nm.(Model)
	if cmd == nil || !m.KindEnabled[navViews] {
		t.Fatal("init + enable views")
	}

	// last filter on rejection
	m.KindEnabled = map[NavSection]bool{navTables: true}
	nm, cmd = m.toggleKindFilter(navTables)
	m = nm.(Model)
	if cmd != nil || !m.KindEnabled[navTables] {
		t.Fatal("last filter must stay on")
	}
	if m.StatusMsg == "" {
		t.Fatal("status for last filter")
	}

	// toggle off when another remains; keep contentSchema
	m.KindEnabled = map[NavSection]bool{navTables: true, navViews: true}
	m.ContentMode = contentSchema
	nm, cmd = m.toggleKindFilter(navViews)
	m = nm.(Model)
	if m.ContentMode != contentSchema {
		t.Fatal("keep schema mode")
	}
	if m.KindEnabled[navViews] {
		t.Fatal("views should toggle off")
	}
	if cmd == nil {
		t.Fatal("reload after toggle")
	}

	// numeric keys via keysBrowser hit toggleKindFilter
	m = navModel(t)
	nm, _ = m.keysBrowser("2", navKey("2"))
	m = nm.(Model)
	if !m.KindEnabled[navViews] {
		t.Fatal("key 2 enables views")
	}
}

func TestNav_SidebarCursorSync(t *testing.T) {
	// on empty NewModel rows still exist (kinds+tools); exercise non-object path
	m := NewModel()
	nm, _ := m.onSidebarCursorMoved()
	m = nm.(Model)
	m = m.syncSelectionFromSidebar()

	m = navModel(t)
	// land on object row -> afterObjectCursorMove
	m.SelectedObjIdx = 0
	m = m.syncSidebarCursorToObject()
	nm, _ = m.onSidebarCursorMoved()
	m = nm.(Model)
	if m.CurrentObject == nil || m.CurrentObject.Name != "orders" {
		t.Fatalf("cursor object=%v", m.CurrentObject)
	}

	// land on kind row -> no afterObjectCursorMove side effect required
	m = m.pinSidebarCursorToKind(navTables)
	prev := m.SelectedObjIdx
	nm, _ = m.onSidebarCursorMoved()
	m = nm.(Model)
	_ = prev

	// syncSelectionFromSidebar updates SelectedObjIdx only for object rows
	m.SelectedObjIdx = 0
	m = m.syncSidebarCursorToObject()
	// move cursor to second object
	rows := m.buildSidebarRows()
	for i, r := range rows {
		if r.kind == sbObject && r.objIdx == 1 {
			m.SidebarCursor = i
			break
		}
	}
	m = m.syncSelectionFromSidebar()
	if m.SelectedObjIdx != 1 {
		t.Fatalf("SelectedObjIdx=%d", m.SelectedObjIdx)
	}

	// non-object cursor leaves SelectedObjIdx alone
	m = m.pinSidebarCursorToKind(navTables)
	before := m.SelectedObjIdx
	m = m.syncSelectionFromSidebar()
	if m.SelectedObjIdx != before {
		t.Fatal("kind row should not change SelectedObjIdx")
	}

	// via keysSidebar j/k which call both helpers
	m = navModel(t)
	m.SelectedObjIdx = 0
	m = m.syncSidebarCursorToObject()
	nm, _ = m.keysSidebar("j")
	m = nm.(Model)
	if m.SelectedObjIdx != 1 {
		t.Fatalf("j idx=%d", m.SelectedObjIdx)
	}
}

func TestNav_OpenSelectedAndPreview(t *testing.T) {
	m := navModel(t)

	// empty selection
	m.Objects = nil
	nm, cmd := m.openSelectedObject()
	if cmd != nil {
		t.Fatal("empty")
	}
	_ = nm

	// relation -> beginTableData
	m = navModel(t)
	m.SelectedObjIdx = 0
	nm, cmd = m.openSelectedObject()
	m = nm.(Model)
	if cmd == nil || !m.Loading {
		t.Fatal("table data")
	}
	_ = cmd()

	// non-relation -> beginObjectDetail
	m.SelectedObjIdx = 3 // sequence
	nm, cmd = m.openSelectedObject()
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("sequence detail")
	}
	_ = cmd()

	// showObjectPreview direct + via beginTableData non-relation
	o := types.SchemaObject{Schema: "public", Name: "do_stuff", Kind: types.ObjectFunction}
	m = m.showObjectPreview(o)
	if m.ContentMode != contentPreview || m.Focus != focusSidebar {
		t.Fatal("preview mode")
	}
	if m.StatusMsg == "" {
		t.Fatal("preview status")
	}
	m = m.showObjectPreview(types.SchemaObject{Name: "x"}) // empty kind status skip
}

func TestNav_OpenDatabasesERDRefreshLoad(t *testing.T) {
	m := navModel(t)

	nm, cmd := m.openDatabasesContent()
	m = nm.(Model)
	if m.ContentMode != contentDatabases || m.Focus != focusContent || cmd == nil {
		t.Fatal("open databases")
	}
	m.Cmds = nil
	nm, cmd = m.openDatabasesContent()
	m = nm.(Model)
	if m.ContentMode != contentDatabases || cmd != nil {
		t.Fatal("databases no cmds")
	}

	m = navModel(t)
	nm, cmd = m.openERD()
	if cmd == nil {
		t.Fatal("erd cmds")
	}
	_ = nm
	m.Cmds = nil
	m.CurrentSchema = ""
	nm, cmd = m.openERD()
	m = nm.(Model)
	if m.Screen != types.ScreenERD || cmd != nil {
		t.Fatal("erd no cmds defaults public")
	}

	m = navModel(t)
	nm, cmd = m.refreshBrowser()
	if cmd == nil {
		t.Fatal("refresh")
	}
	_ = nm
	m.Cmds = nil
	_, cmd = m.refreshBrowser()
	if cmd != nil {
		t.Fatal("refresh no cmds")
	}

	// loadObjectsForNav keeps schema mode + extension-only empties schema
	m = navModel(t)
	m.ContentMode = contentSchema
	nm, cmd = m.loadObjectsForNav()
	m = nm.(Model)
	if m.ContentMode != contentSchema || cmd == nil {
		t.Fatal("keep schema")
	}
	m.KindEnabled = map[NavSection]bool{navExtensions: true}
	nm, cmd = m.loadObjectsForNav()
	if cmd == nil {
		t.Fatal("ext load")
	}
	m.Cmds = nil
	_, cmd = m.loadObjectsForNav()
	if cmd != nil {
		t.Fatal("load no cmds")
	}
	_ = nm
}

func TestNav_BeginDetailAndData(t *testing.T) {
	m := navModel(t)

	// beginObjectDetail guards
	_, cmd := m.beginObjectDetail(types.SchemaObject{})
	if cmd != nil {
		t.Fatal("empty name")
	}
	m.Cmds = nil
	_, cmd = m.beginObjectDetail(types.SchemaObject{Name: "fn", Schema: "public", Kind: types.ObjectFunction})
	if cmd != nil {
		t.Fatal("no cmds")
	}
	m = navModel(t)
	nm, cmd := m.beginObjectDetail(types.SchemaObject{Name: "fn", Schema: "public", Kind: types.ObjectFunction})
	m = nm.(Model)
	if cmd == nil || !m.Loading || m.Focus != focusContent {
		t.Fatal("object detail")
	}
	msg := cmd()
	if _, ok := msg.(sequencedDetailMsg); !ok {
		t.Fatalf("want sequencedDetailMsg got %T", msg)
	}

	// beginTableDetail
	_, cmd = m.beginTableDetail(types.SchemaObject{})
	if cmd != nil {
		t.Fatal("td empty")
	}
	m.Cmds = nil
	_, cmd = m.beginTableDetail(types.SchemaObject{Name: "x"})
	if cmd != nil {
		t.Fatal("td no cmds")
	}
	m = navModel(t)
	// non-relation delegates to beginObjectDetail
	nm2, cmd := m.beginTableDetail(types.SchemaObject{Name: "s", Schema: "public", Kind: types.ObjectSequence})
	m = nm2
	if cmd == nil {
		t.Fatal("td seq")
	}
	_ = cmd()
	// relation wraps LoadObjectDetail
	nm2, cmd = m.beginTableDetail(types.SchemaObject{Name: "orders", Schema: "public", Kind: types.ObjectTable})
	m = nm2
	if cmd == nil {
		t.Fatal("td table")
	}
	msg = cmd()
	if _, ok := msg.(sequencedDetailMsg); !ok {
		t.Fatalf("want sequencedDetailMsg got %T", msg)
	}

	// beginTableData
	_, cmd = m.beginTableData(types.SchemaObject{}, 0, 10)
	if cmd != nil {
		t.Fatal("data empty")
	}
	m.Cmds = nil
	_, cmd = m.beginTableData(types.SchemaObject{Name: "x"}, 0, 10)
	if cmd != nil {
		t.Fatal("data no cmds")
	}
	m = navModel(t)
	// non-relation -> showObjectPreview
	nm2, cmd = m.beginTableData(types.SchemaObject{Name: "fn", Schema: "public", Kind: types.ObjectFunction}, 0, 10)
	m = nm2
	if cmd != nil || m.ContentMode != contentPreview {
		t.Fatal("non-rel preview")
	}
	// relation wraps LoadTableData
	nm2, cmd = m.beginTableData(types.SchemaObject{Name: "orders", Schema: "public", Kind: types.ObjectTable}, 0, 10)
	m = nm2
	if cmd == nil {
		t.Fatal("data table")
	}
	msg = cmd()
	if _, ok := msg.(sequencedDataMsg); !ok {
		t.Fatalf("want sequencedDataMsg got %T", msg)
	}

	// empty kind is treated as relation
	_, cmd = m.beginTableData(types.SchemaObject{Name: "t", Schema: "public", Kind: ""}, 0, 10)
	if cmd == nil {
		t.Fatal("empty kind relation")
	}
}

func TestNav_ApplyTableDetailAndData(t *testing.T) {
	m := navModel(t)

	// error path
	nm, _ := m.applyTableDetail(types.TableDetailLoadedMsg{Err: errors.New("boom")})
	m = nm.(Model)
	if m.Err == nil {
		t.Fatal("detail err")
	}

	// mismatch current object
	cur := types.SchemaObject{Schema: "public", Name: "orders", Kind: types.ObjectTable}
	m.CurrentObject = &cur
	nm, _ = m.applyTableDetail(types.TableDetailLoadedMsg{
		Detail: types.TableDetail{Object: types.SchemaObject{Schema: "public", Name: "users"}},
	})
	m = nm.(Model)
	if m.Screen == types.ScreenTableDetail {
		t.Fatal("mismatch should not open detail")
	}

	// success + fill schema from CurrentSchema when detail schema empty
	m.CurrentObject = nil
	m.CurrentSchema = "public"
	m.SchemaCols = map[string][]string{"x": {"a"}}
	nm, _ = m.applyTableDetail(types.TableDetailLoadedMsg{
		Detail: types.TableDetail{
			Object:  types.SchemaObject{Name: "orders", Schema: ""},
			Columns: []types.ColumnInfo{{Name: "id", DataType: "int"}},
		},
	})
	m = nm.(Model)
	if m.Screen != types.ScreenTableDetail {
		t.Fatal("detail screen")
	}
	if m.CurrentObject == nil || m.CurrentObject.Schema != "public" || m.CurrentObject.Name != "orders" {
		t.Fatalf("schema fill=%v", m.CurrentObject)
	}

	// empty columns/props status; empty name -> "table"
	m.CurrentObject = nil
	nm, _ = m.applyTableDetail(types.TableDetailLoadedMsg{
		Detail: types.TableDetail{Object: types.SchemaObject{}},
	})
	m = nm.(Model)
	if m.StatusMsg == "" {
		t.Fatal("empty columns status")
	}

	// applyTableData error
	nm, _ = m.applyTableData(types.TableDataLoadedMsg{Err: errors.New("e")})
	m = nm.(Model)
	if m.Err == nil {
		t.Fatal("data err")
	}

	// SchemaCols nil + column cache
	m.SchemaCols = nil
	obj := types.SchemaObject{Schema: "public", Name: "orders", Kind: types.ObjectTable}
	m.CurrentObject = &obj
	nm, _ = m.applyTableData(types.TableDataLoadedMsg{
		Result: types.QueryResult{Columns: []string{"id", "total"}, Rows: [][]string{{"1", "2"}}},
		Offset: 0,
	})
	m = nm.(Model)
	if m.SchemaCols == nil || len(m.SchemaCols["orders"]) != 2 {
		t.Fatalf("SchemaCols=%v", m.SchemaCols)
	}
	if m.Screen != types.ScreenTableData {
		t.Fatal("data screen")
	}

	// empty page with offset snaps back
	m.PageSize = 10
	m.SchemaCols = map[string][]string{}
	nm, cmd := m.applyTableData(types.TableDataLoadedMsg{
		Result: types.QueryResult{Columns: []string{"id"}, Rows: nil},
		Offset: 20,
	})
	if cmd == nil {
		t.Fatal("snap back")
	}
	_ = nm

	// empty first page stays
	nm, cmd = m.applyTableData(types.TableDataLoadedMsg{
		Result: types.QueryResult{Columns: []string{"id"}, Rows: nil},
		Offset: 0,
	})
	m = nm.(Model)
	if cmd != nil {
		t.Fatal("no snap at 0")
	}
	if m.DataOffset != 0 {
		t.Fatal("offset")
	}

	// via Update messages
	m = navModel(t)
	m.CurrentObject = &obj
	nm, _ = m.Update(types.TableDetailLoadedMsg{
		Detail: types.TableDetail{
			Object:  obj,
			Columns: []types.ColumnInfo{{Name: "id"}},
		},
	})
	m = nm.(Model)
	nm, _ = m.Update(types.TableDataLoadedMsg{
		Result: types.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}}},
		Offset: 0,
	})
	_ = nm
}

func TestNav_KeysObjectList(t *testing.T) {
	m := navModel(t)
	m.Screen = types.ScreenBrowser
	m.Focus = focusContent
	m.SelectedObjIdx = 0

	// j/down/k/up/G navigation
	nm, _ := m.keysObjectList("j")
	m = nm.(Model)
	if m.SelectedObjIdx != 1 {
		t.Fatal("j")
	}
	nm, _ = m.keysObjectList("down")
	m = nm.(Model)
	nm, _ = m.keysObjectList("k")
	m = nm.(Model)
	nm, _ = m.keysObjectList("up")
	m = nm.(Model)

	// g from non-zero (previously uncovered)
	m.SelectedObjIdx = 3
	nm, _ = m.keysObjectList("g")
	m = nm.(Model)
	if m.SelectedObjIdx != 0 {
		t.Fatalf("g idx=%d", m.SelectedObjIdx)
	}
	// g / k / up at zero are no-ops
	nm, _ = m.keysObjectList("g")
	m = nm.(Model)
	nm, _ = m.keysObjectList("k")
	m = nm.(Model)
	nm, _ = m.keysObjectList("up")
	m = nm.(Model)
	if m.SelectedObjIdx != 0 {
		t.Fatal("stay at 0")
	}

	// G to end, then G again no-op, j at end no-op
	nm, _ = m.keysObjectList("G")
	m = nm.(Model)
	last := m.SelectedObjIdx
	nm, _ = m.keysObjectList("G")
	m = nm.(Model)
	if m.SelectedObjIdx != last {
		t.Fatal("G stay")
	}
	nm, _ = m.keysObjectList("j")
	m = nm.(Model)

	// h/left focus sidebar
	nm, _ = m.keysObjectList("h")
	m = nm.(Model)
	if m.Focus != focusSidebar {
		t.Fatal("h")
	}
	m.Focus = focusContent
	nm, _ = m.keysObjectList("left")
	m = nm.(Model)

	// enter/l/right on table/view opens data
	m.SelectedObjIdx = 0
	nm, cmd := m.keysObjectList("enter")
	if cmd == nil {
		t.Fatal("enter table")
	}
	_ = nm
	m.SelectedObjIdx = 2 // view
	nm, cmd = m.keysObjectList("l")
	if cmd == nil {
		t.Fatal("l view")
	}
	// mat view via synthetic list
	m.Objects = append(m.Objects, types.SchemaObject{Schema: "public", Name: "mv", Kind: types.ObjectMatView})
	m.SelectedObjIdx = len(m.Objects) - 1
	nm, cmd = m.keysObjectList("right")
	if cmd == nil {
		t.Fatal("right matview")
	}

	// enter on non-relation opens detail
	m.SelectedObjIdx = 3 // sequence in original list - rebuild
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "orders", Kind: types.ObjectTable},
		{Schema: "public", Name: "id_seq", Kind: types.ObjectSequence},
	}
	m.SelectedObjIdx = 1
	nm, cmd = m.keysObjectList("enter")
	if cmd == nil {
		t.Fatal("enter seq detail")
	}

	// D opens detail
	m.SelectedObjIdx = 0
	nm, cmd = m.keysObjectList("D")
	if cmd == nil {
		t.Fatal("D")
	}

	// empty objects branches
	m.Objects = nil
	for _, k := range []string{"j", "k", "G", "enter", "D", "g"} {
		nm, cmd = m.keysObjectList(k)
		_ = nm
		if (k == "enter" || k == "D") && cmd != nil {
			t.Fatalf("%s empty", k)
		}
	}

	// k at 0 no-op already covered; j at last covered
	// via Update on browser content focus is not object-list by default —
	// call path exercised above is enough; still drive a KeyPressMsg through Update
	m = navModel(t)
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	nm, _ = m.Update(navKey("j"))
	_ = nm
}

func TestNav_CopyAndAfterObjectCursorMove(t *testing.T) {
	m := navModel(t)

	// copySelectedObject fills empty schema from CurrentSchema
	m.Objects = []types.SchemaObject{{Name: "t", Kind: types.ObjectTable}}
	m.SelectedObjIdx = 0
	m.CurrentSchema = "billing"
	o := m.copySelectedObject()
	if o.Schema != "billing" || o.Name != "t" {
		t.Fatalf("copy=%+v", o)
	}

	// empty copy
	m.Objects = nil
	if m.copySelectedObject().Name != "" {
		t.Fatal("empty copy")
	}

	// afterObjectCursorMove empty name
	nm, cmd := m.afterObjectCursorMove()
	if cmd != nil {
		t.Fatal("empty after")
	}
	_ = nm

	// browser screens: only set CurrentObject
	m = navModel(t)
	m.Screen = types.ScreenBrowser
	m.SelectedObjIdx = 1
	nm, cmd = m.afterObjectCursorMove()
	m = nm.(Model)
	if cmd != nil || m.CurrentObject == nil || m.CurrentObject.Name != "users" {
		t.Fatal("browser after")
	}
	for _, sc := range []types.Screen{
		types.ScreenQuery, types.ScreenActivity, types.ScreenERD, types.ScreenServerInfo,
	} {
		m.Screen = sc
		m.SelectedObjIdx = 0
		nm, cmd = m.afterObjectCursorMove()
		m = nm.(Model)
		if cmd != nil {
			t.Fatalf("%v should not reload", sc)
		}
	}

	// data/structure: cursor move returns to preview, does not auto-open
	m.Screen = types.ScreenTableDetail
	m.SelectedObjIdx = 0
	nm, cmd = m.afterObjectCursorMove()
	m = nm.(Model)
	if cmd != nil || m.Screen != types.ScreenBrowser || m.Focus != focusSidebar {
		t.Fatalf("detail cursor: screen=%v focus=%v cmd=%v", m.Screen, m.Focus, cmd != nil)
	}

	m.Screen = types.ScreenTableData
	m.SelectedObjIdx = 1
	nm, cmd = m.afterObjectCursorMove()
	m = nm.(Model)
	if cmd != nil || m.Screen != types.ScreenBrowser || m.Focus != focusSidebar {
		t.Fatalf("data cursor: screen=%v focus=%v cmd=%v", m.Screen, m.Focus, cmd != nil)
	}
	if m.CurrentObject == nil || m.CurrentObject.Name != "users" {
		t.Fatalf("data cursor object: %+v", m.CurrentObject)
	}

	// data screen + non-relation still stays on preview (enter opens)
	m.Objects = []types.SchemaObject{{Schema: "public", Name: "fn", Kind: types.ObjectFunction}}
	m.SelectedObjIdx = 0
	m.Screen = types.ScreenTableData
	nm, cmd = m.afterObjectCursorMove()
	m = nm.(Model)
	if cmd != nil || m.Screen != types.ScreenBrowser {
		t.Fatalf("data nonrel: screen=%v cmd=%v", m.Screen, cmd != nil)
	}

	// default screen
	m.Screen = types.ScreenConnections
	m.Objects = []types.SchemaObject{{Schema: "public", Name: "orders", Kind: types.ObjectTable}}
	m.SelectedObjIdx = 0
	_, cmd = m.afterObjectCursorMove()
	if cmd != nil {
		t.Fatal("default")
	}

	// stale err cleared
	m = navModel(t)
	m.Err = errors.New("stale")
	m.Screen = types.ScreenBrowser
	nm, _ = m.afterObjectCursorMove()
	m = nm.(Model)
	if m.Err != nil {
		t.Fatal("stale err")
	}
}
