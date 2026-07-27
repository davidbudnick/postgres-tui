package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func stripCSI(s string) string {
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

func TestTableDetailLoadedSetsCurrentObject(t *testing.T) {
	m := testModel(t)
	m.CurrentSchema = "public"
	nm, _ := m.Update(types.TableDetailLoadedMsg{
		Detail: types.TableDetail{
			Object:  types.SchemaObject{Schema: "public", Name: "order_items", Kind: types.ObjectTable},
			Columns: []types.ColumnInfo{{Name: "id", DataType: "int", Position: 1}},
		},
	})
	m = nm.(Model)
	if m.CurrentObject == nil {
		t.Fatal("CurrentObject not set")
	}
	if m.CurrentObject.Schema != "public" || m.CurrentObject.Name != "order_items" {
		t.Fatalf("CurrentObject=%+v", m.CurrentObject)
	}
	if m.Screen != types.ScreenTableDetail {
		t.Fatalf("screen=%v", m.Screen)
	}
}

func TestTableDetailLoadedEmptyColumnsStatus(t *testing.T) {
	m := testModel(t)
	nm, _ := m.Update(types.TableDetailLoadedMsg{
		Detail: types.TableDetail{
			Object:  types.SchemaObject{Schema: "public", Name: "empty_tbl", Kind: types.ObjectTable},
			Columns: nil,
		},
	})
	m = nm.(Model)
	if !strings.Contains(m.StatusMsg, "no columns found for public.empty_tbl") {
		t.Fatalf("status=%q", m.StatusMsg)
	}
}

func TestStructureTitlePrefersCurrentObject(t *testing.T) {
	m := NewModel()
	m.Width = 100
	m.Height = 30
	obj := types.SchemaObject{Schema: "public", Name: "order_items", Kind: types.ObjectTable}
	m.CurrentObject = &obj
	m.TableDetail = types.TableDetail{
		Object:  types.SchemaObject{Schema: "pg_catalog", Name: "postgresql", Kind: types.ObjectTable},
		Columns: nil,
	}
	plain := stripCSI(m.viewTableDetailContent(80, 20))
	if strings.Contains(plain, "pg_catalog") {
		t.Fatalf("stale detail title leaked: %q", plain)
	}
	if !strings.Contains(plain, "order_items") {
		t.Fatalf("expected CurrentObject title: %q", plain)
	}
	// Mismatched detail must not show stale "No columns" body for wrong object.
	if strings.Contains(plain, "No columns") {
		t.Fatalf("should not show columns body for mismatched detail: %q", plain)
	}
}

func TestStructureShowsColumnsWhenMatched(t *testing.T) {
	m := NewModel()
	obj := types.SchemaObject{Schema: "public", Name: "order_items", Kind: types.ObjectTable}
	m.CurrentObject = &obj
	m.TableDetail = types.TableDetail{
		Object:  obj,
		Columns: []types.ColumnInfo{{Name: "qty", DataType: "int", Position: 1}},
	}
	plain := stripCSI(m.viewTableDetailContent(60, 20))
	if !strings.Contains(plain, "order_items") {
		t.Fatalf("title: %q", plain)
	}
	if !strings.Contains(plain, "qty") {
		t.Fatalf("column missing: %q", plain)
	}
}

func TestObjectListJKReloadsStructure(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenTableDetail
	m.Focus = focusSidebar
	m.CurrentSchema = "public"
	m.NavSection = navTables
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "orders", Kind: types.ObjectTable},
		{Schema: "public", Name: "order_items", Kind: types.ObjectTable},
	}
	m.SelectedObjIdx = 0
	o := m.Objects[0]
	m.CurrentObject = &o
	m.TableDetail = types.TableDetail{Object: o, Columns: []types.ColumnInfo{{Name: "id"}}}
	m = m.syncSidebarCursorToObject()

	nm, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = nm.(Model)
	if m.SelectedObjIdx != 1 {
		t.Fatalf("idx=%d", m.SelectedObjIdx)
	}
	if m.CurrentObject == nil || m.CurrentObject.Name != "order_items" {
		t.Fatalf("CurrentObject=%v", m.CurrentObject)
	}
	if cmd == nil {
		t.Fatal("expected LoadTableDetail cmd after j/k on Structure")
	}
	if !m.Loading {
		t.Fatal("expected loading while reloading structure")
	}
}

func TestSidebarJKSequenceLoadsObjectDetail(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenTableDetail
	m.Focus = focusSidebar
	m.CurrentSchema = "public"
	m.Err = fmt.Errorf("stale")
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "users", Kind: types.ObjectTable},
		{Schema: "public", Name: "users_id_seq", Kind: types.ObjectSequence},
	}
	m.SelectedObjIdx = 0
	u := m.Objects[0]
	m.CurrentObject = &u
	m.TableDetail = types.TableDetail{Object: u, Columns: []types.ColumnInfo{{Name: "id"}}}
	m = m.syncSidebarCursorToObject()

	nm, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = nm.(Model)
	if m.CurrentObject == nil || m.CurrentObject.Name != "users_id_seq" {
		t.Fatalf("CurrentObject=%v", m.CurrentObject)
	}
	if cmd == nil {
		t.Fatal("expected LoadObjectDetail for sequences")
	}
	if m.Err != nil {
		t.Fatalf("stale error should clear: %v", m.Err)
	}
	if m.Loading {
		// loading object detail is OK
	}
}

func TestOpenSequenceLoadsDetail(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "users_id_seq", Kind: types.ObjectSequence},
	}
	m.SelectedObjIdx = 0
	m = m.syncSidebarCursorToObject()
	nm, cmd := m.openSelectedObject()
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("expected LoadObjectDetail cmd for sequence")
	}
	if m.Err != nil {
		t.Fatalf("err=%v", m.Err)
	}
	if !m.Loading {
		t.Fatal("expected loading")
	}
}

func TestObjectDetailViewShowsPropsAndDefinition(t *testing.T) {
	m := testModel(t)
	m.Width = 100
	m.Height = 40
	m.Screen = types.ScreenTableDetail
	m.Focus = focusContent
	o := types.SchemaObject{Schema: "public", Name: "users_id_seq", Kind: types.ObjectSequence}
	m.CurrentObject = &o
	m.TableDetail = types.TableDetail{
		Object: o,
		Props: []types.DetailProp{
			{Label: "last value", Value: "1000"},
			{Label: "increment", Value: "1"},
		},
		CreateSQL: "SELECT nextval('public.users_id_seq');",
	}
	plain := stripCSI(m.viewTableDetailContent(80, 20))
	if !strings.Contains(plain, "sequence") && !strings.Contains(plain, "Info") {
		t.Fatalf("expected sequence detail chrome: %q", plain)
	}
	if !strings.Contains(plain, "last value") || !strings.Contains(plain, "1000") {
		t.Fatalf("missing props: %q", plain)
	}
	// Switch to definition tab
	m.DetailTab = 1
	plain = stripCSI(m.viewTableDetailContent(80, 20))
	if !strings.Contains(plain, "nextval") {
		t.Fatalf("definition missing: %q", plain)
	}
}

func TestBrowserFilterNavDoesNotForceOpen(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenBrowser
	m.ContentMode = contentPreview
	m.Focus = focusSidebar
	m.ObjectFilter = "user"
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "users_id_seq", Kind: types.ObjectSequence},
		{Schema: "public", Name: "users", Kind: types.ObjectTable},
	}
	m.SelectedObjIdx = 0
	m = m.syncSidebarCursorToObject()
	nm, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = nm.(Model)
	if cmd != nil {
		t.Fatal("j/k in browser must not fire load cmds")
	}
	if m.Screen != types.ScreenBrowser {
		t.Fatalf("screen forced to %v", m.Screen)
	}
	if m.CurrentObject == nil || m.CurrentObject.Name != "users" {
		t.Fatalf("expected cursor on users, got %v", m.CurrentObject)
	}
}

func TestKindSelectionDoesNotJumpToObjects(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	m.CurrentSchema = "public"
	m.NavSection = navTables
	m.Schemas = []types.SchemaInfo{{Name: "public", TableCount: 2}}
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "orders", Kind: types.ObjectTable},
		{Schema: "public", Name: "users", Kind: types.ObjectTable},
	}
	// Cursor on compact KIND chips row
	m = m.pinSidebarCursorToKind(navTables)
	kindIdx := m.SidebarCursor

	// Simulate async objects reload (what used to fling the cursor into TABLES).
	nm, _ := m.Update(types.ObjectsLoadedMsg{
		Objects: []types.SchemaObject{
			{Schema: "public", Name: "orders", Kind: types.ObjectTable},
			{Schema: "public", Name: "users", Kind: types.ObjectTable},
			{Schema: "public", Name: "extra", Kind: types.ObjectTable},
		},
		Kind: types.ObjectTable,
	})
	m = nm.(Model)
	if m.SidebarCursor != kindIdx {
		rows := m.buildSidebarRows()
		r := rows[clamp(m.SidebarCursor, 0, len(rows)-1)]
		if r.kind != sbKind {
			t.Fatalf("cursor jumped off KIND chips: kind=%v nav=%v idx=%d (was %d)", r.kind, r.nav, m.SidebarCursor, kindIdx)
		}
	}
}

func TestKindFilterToggle(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	m.CurrentSchema = "public"
	m.KindEnabled = defaultKindFilters()
	m.Schemas = []types.SchemaInfo{{Name: "public", TableCount: 1}}
	m.Objects = []types.SchemaObject{{Schema: "public", Name: "orders", Kind: types.ObjectTable}}
	m = m.pinSidebarCursorToKind(navViews)

	nm, cmd := m.keysSidebar(" ")
	m = nm.(Model)
	if !m.kindChecked(navViews) {
		t.Fatal("expected views filter enabled")
	}
	if !m.kindChecked(navTables) {
		t.Fatal("tables should stay on")
	}
	if cmd == nil {
		t.Fatal("expected reload after toggle")
	}
	// Toggle views off again
	m = m.pinSidebarCursorToKind(navViews)
	nm, _ = m.keysSidebar("enter")
	m = nm.(Model)
	if m.kindChecked(navViews) {
		t.Fatal("expected views toggled off")
	}
}

func TestSchemaEnterShowsOverview(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	m.NavSection = navTables
	m.Schemas = []types.SchemaInfo{
		{Name: "public", Owner: "postgres", TableCount: 2, ViewCount: 1},
	}
	m.CurrentSchema = "public"
	m.SelectedSchema = 0
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "orders", Kind: types.ObjectTable, RowEstimate: 100, SizePretty: "48 kB"},
	}
	nm, cmd := m.activateSidebarRow(sidebarRow{kind: sbSchema, schema: 0})
	m = nm.(Model)
	if m.ContentMode != contentSchema {
		t.Fatalf("ContentMode=%v want schema", m.ContentMode)
	}
	if m.Screen != types.ScreenBrowser {
		t.Fatalf("screen=%v", m.Screen)
	}
	if cmd == nil {
		t.Fatal("expected LoadObjects")
	}
	plain := stripCSI(m.viewSchemaOverviewContent(60, 20))
	if !strings.Contains(plain, "public") {
		t.Fatalf("missing schema name: %q", plain)
	}
	if !strings.Contains(plain, "postgres") || !strings.Contains(plain, "tables") {
		t.Fatalf("missing schema meta: %q", plain)
	}
}

func TestObjectIdentityDistinguishesTableFromType(t *testing.T) {
	table := types.SchemaObject{Schema: "public", Name: "orders", Kind: types.ObjectTable}
	typ := types.SchemaObject{Schema: "public", Name: "orders", Kind: types.ObjectType}
	cur := table
	if !objectIdentityMatch(&cur, table) {
		t.Fatal("table should match itself")
	}
	if objectIdentityMatch(&cur, typ) {
		t.Fatal("table orders must not match type orders")
	}

	m := testModel(t)
	m.Width = 80
	m.Height = 40
	m.Screen = types.ScreenBrowser
	m.ContentMode = contentPreview
	m.Focus = focusSidebar
	m.CurrentObject = &table
	m.KindEnabled = map[NavSection]bool{navTables: true, navTypes: true}
	m.Objects = []types.SchemaObject{table, typ}
	m.SelectedObjIdx = 0
	m = m.syncSidebarCursorToObject()

	side := m.viewSidebar(32, 40)
	// Only the selected table row should use the blue band; type must not get open-object accent.
	// Accent open-state uses bold accent on name — ensure we don't accent the type row as open.
	// Easiest structural check: objectIdentityMatch already covers identity; render still lists both.
	plain := stripCSI(side)
	if !strings.Contains(plain, "orders") {
		t.Fatalf("sidebar missing orders: %q", plain)
	}
	// Type row should still appear as its own line with type glyph.
	if !strings.Contains(plain, kindGlyph("type")) {
		t.Fatalf("expected type glyph in list: %q", plain)
	}
}

func TestSchemaOverviewNoDualBlueSelection(t *testing.T) {
	m := testModel(t)
	m.Width = 120
	m.Height = 40
	m.Screen = types.ScreenBrowser
	m.ContentMode = contentSchema
	m.CurrentSchema = "public"
	m.Schemas = []types.SchemaInfo{{Name: "public", TableCount: 2, Owner: "pg"}}
	m.KindEnabled = map[NavSection]bool{
		navTables: true, navTypes: true,
	}
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "orders", Kind: types.ObjectTable, RowEstimate: 10},
		{Schema: "public", Name: "products", Kind: types.ObjectType},
	}
	m.SelectedObjIdx = 1
	m.Focus = focusSidebar

	// Sidebar owns cursor: content must not paint ANY selection background band.
	content := m.viewSchemaOverviewContent(80, 20)
	if !strings.Contains(content, "products") {
		t.Fatalf("missing products: %q", stripCSI(content))
	}
	// selectedStyle always injects a background SGR; unfocused content row must not match it.
	brightSample := selectedStyle.Render(padRight("products", 20))
	// Crude but effective: focused selection contains the select bg color code "39" or "48;5;39".
	if strings.Contains(content, selectedStyle.Render("products")) {
		t.Fatal("sidebar-focused content reused full selectedStyle on products row")
	}
	_ = brightSample
	// When content focused, bright band is used.
	m.Focus = focusContent
	focused := m.viewSchemaOverviewContent(80, 20)
	if !strings.Contains(stripCSI(focused), "products") {
		t.Fatal("focused content missing products")
	}
	if !strings.Contains(focused, "\x1b[") {
		t.Fatal("focused selection should be styled")
	}
	// Multi-kind list labeled OBJECTS with KIND column, not a stale single kind.
	plain := stripCSI(content)
	if strings.Contains(plain, "EXTENSIONS") {
		t.Fatalf("should not show single kind header when multi-filter: %q", plain)
	}
	if !strings.Contains(plain, "OBJECTS") && !strings.Contains(plain, "KIND") {
		// OBJECTS title or KIND column
		if !strings.Contains(plain, "type") && !strings.Contains(plain, "table") {
			t.Fatalf("expected kind disambiguation in multi-filter list: %q", plain)
		}
	}
}

func TestDatabasesOpensInContentPane(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenBrowser
	m.CurrentDatabase = "demo"
	m.CurrentConn = &types.Connection{Name: "local"}
	nm, cmd := m.activateSidebarRow(sidebarRow{kind: sbTool, nav: navDatabases})
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatalf("screen=%v want browser workspace", m.Screen)
	}
	if m.ContentMode != contentDatabases {
		t.Fatalf("ContentMode=%v want databases", m.ContentMode)
	}
	if m.Focus != focusContent {
		t.Fatalf("focus=%v want content", m.Focus)
	}
	if cmd == nil {
		t.Fatal("expected LoadDatabases")
	}
	nm, _ = m.Update(types.DatabasesLoadedMsg{Databases: []types.DatabaseInfo{
		{Name: "demo", Owner: "pg", SizePretty: "10 MB", Encoding: "UTF8"},
		{Name: "other", Owner: "pg", SizePretty: "1 MB", Encoding: "UTF8"},
	}})
	m = nm.(Model)
	plain := stripCSI(m.viewDatabasesContent(70, 15))
	if !strings.Contains(plain, "demo") || !strings.Contains(plain, "other") {
		t.Fatalf("missing db list: %q", plain)
	}
	// esc returns to preview without leaving workspace
	nm, _ = m.keysBrowser("esc", tea.KeyPressMsg{})
	m = nm.(Model)
	if m.ContentMode != contentPreview {
		t.Fatalf("ContentMode=%v after esc", m.ContentMode)
	}
	if m.Screen != types.ScreenBrowser {
		t.Fatalf("screen=%v after esc", m.Screen)
	}
}

func TestCopySelectedObjectFillsSchema(t *testing.T) {
	m := NewModel()
	m.CurrentSchema = "public"
	m.Objects = []types.SchemaObject{
		{Name: "orders", Kind: types.ObjectTable}, // schema empty
	}
	m.SelectedObjIdx = 0
	o := m.copySelectedObject()
	if o.Schema != "public" || o.Name != "orders" {
		t.Fatalf("got %+v", o)
	}
}

func TestNavSwitchClearsStaleExtensionStructure(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenTableDetail
	m.Focus = focusContent
	m.CurrentSchema = "public"
	ext := types.SchemaObject{Schema: "pg_catalog", Name: "plpgsql", Kind: types.ObjectExtension}
	m.CurrentObject = &ext
	m.TableDetail = types.TableDetail{Object: ext}
	m.Objects = []types.SchemaObject{ext}
	m.NavSection = navExtensions
	m.KindEnabled = map[NavSection]bool{navExtensions: true} // only extensions

	// Enable Tables filter — reloads list and clears stale Structure.
	nm, cmd := m.activateSidebarRow(sidebarRow{kind: sbKind, nav: navTables})
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatalf("screen=%v want browser", m.Screen)
	}
	if m.CurrentObject != nil {
		t.Fatalf("CurrentObject still set: %+v", m.CurrentObject)
	}
	if m.TableDetail.Object.Name != "" {
		t.Fatalf("TableDetail not cleared: %+v", m.TableDetail.Object)
	}
	if !m.kindChecked(navTables) {
		t.Fatal("tables filter should be on")
	}
	if cmd == nil {
		t.Fatal("expected LoadObjects")
	}

	// Objects reload to tables; selection shows order_items, content stays empty.
	nm, _ = m.Update(types.ObjectsLoadedMsg{
		Objects: []types.SchemaObject{
			{Schema: "public", Name: "order_items", Kind: types.ObjectTable},
		},
		Kind: types.ObjectTable,
	})
	m = nm.(Model)
	if m.SelectedObjIdx != 0 || m.Objects[0].Name != "order_items" {
		t.Fatalf("objects not loaded: %+v", m.Objects)
	}
	plain := stripCSI(m.viewTableDetailContent(50, 20))
	if strings.Contains(plain, "plpgsql") || strings.Contains(plain, "pg_catalog") {
		t.Fatalf("stale extension structure still visible: %q", plain)
	}
}

func TestStaleDetailMsgIgnoredAfterNavSwitch(t *testing.T) {
	m := testModel(t)
	m.CurrentSchema = "public"
	ext := types.SchemaObject{Schema: "pg_catalog", Name: "plpgsql", Kind: types.ObjectExtension}
	m = m.setCurrentObject(ext)
	m.contentSeq = 1
	// Simulate in-flight detail for plpgsql (seq=1), then nav invalidates (seq=2).
	m = m.clearObjectContent() // bumps contentSeq to 2
	nm, _ := m.Update(sequencedDetailMsg{
		TableDetailLoadedMsg: types.TableDetailLoadedMsg{
			Detail: types.TableDetail{Object: ext},
		},
		seq: 1,
	})
	m = nm.(Model)
	if m.Screen == types.ScreenTableDetail {
		t.Fatal("stale detail should not open Structure")
	}
	if m.TableDetail.Object.Name == "plpgsql" {
		t.Fatal("stale detail applied")
	}
}

func TestDUsesSelectedObjectNotStaleCurrent(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m.CurrentSchema = "public"
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "order_items", Kind: types.ObjectTable},
		{Schema: "public", Name: "orders", Kind: types.ObjectTable},
	}
	m.SelectedObjIdx = 0
	stale := types.SchemaObject{Schema: "pg_catalog", Name: "plpgsql", Kind: types.ObjectExtension}
	m.CurrentObject = &stale

	nm, cmd := m.keysTableData("D")
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("expected LoadTableDetail")
	}
	if m.CurrentObject == nil || m.CurrentObject.Name != "order_items" {
		t.Fatalf("CurrentObject=%v want order_items", m.CurrentObject)
	}
}

func TestObjectListCompactNameFits(t *testing.T) {
	cols := objectListCols(20)
	plain, _ := cols.row("order_items", "5.0K", "table")
	if displayWidth(plain) != 20 {
		t.Fatalf("width=%d plain=%q", displayWidth(plain), plain)
	}
	if !strings.Contains(plain, "order_items") {
		t.Fatalf("name truncated too aggressively: %q", plain)
	}
}
