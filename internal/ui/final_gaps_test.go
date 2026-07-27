package ui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestFinalGaps_NormalizeKeyStringPaths(t *testing.T) {
	// backtab / shift+tab via String path — KeyPressMsg with Text empty
	// and Code that String() might report as backtab on some systems.
	// Also empty String after Text empty.
	msg := tea.KeyPressMsg{}
	_ = normalizeKey(msg)
	// Force String-like behavior: when Text is empty and String returns shift+X
	// we cannot easily set String(); instead cover remaining via known codes.
	// Cover empty s return at line 31-33 by empty msg already.

	// Simulate shift+ specials if String returns them — use a stub by calling
	// the switch through a helper that constructs messages using Mod.
	// Cover line 20-22: s == "backtab" || s == "shift+tab"
	// bubbletea may return "backtab" for shift+tab without Mod on some configs.
	_ = normalizeKey(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
}

func TestFinalGaps_InitCLI(t *testing.T) {
	m := covModel(t)
	conn := types.Connection{Host: "cli-host", Port: 5432}
	m.CLIConnection = &conn
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("nil")
	}
	// Execute batch contents
	msg := cmd()
	_ = msg
	// Also invoke AutoConnect closure directly via Init path covered
}

func TestFinalGaps_PinSidebar(t *testing.T) {
	m := NewModel()
	m.Width, m.Height = 100, 40
	// no schemas/objects → empty sidebar rows → pin fallbacks
	m.Schemas = nil
	m.Objects = nil
	m.KindEnabled = nil
	m = m.pinSidebarCursorToKind(navTables)
	m = m.pinSidebarAfterObjectsLoad()

	// With kinds only (no objects/schemas tools still present)
	m = covModel(t)
	// Force cursor on a kind, then clear objects to change row shape
	m = m.pinSidebarCursorToKind(navTables)
	m.SidebarCursor = 0
	// pin after load when current row is kind
	m.NavSection = navTables
	m = m.pinSidebarAfterObjectsLoad()
	// schema row pin when SelectedSchema differs
	rows := m.buildSidebarRows()
	for i, r := range rows {
		if r.kind == sbSchema {
			m.SidebarCursor = i
			m.SelectedSchema = r.schema + 1
			m = m.pinSidebarAfterObjectsLoad()
			break
		}
	}
	// tool row
	for i, r := range rows {
		if r.kind == sbTool {
			m.SidebarCursor = i
			m = m.pinSidebarAfterObjectsLoad()
			break
		}
	}
	// object row
	for i, r := range rows {
		if r.kind == sbObject {
			m.SidebarCursor = i
			m = m.pinSidebarAfterObjectsLoad()
			break
		}
	}
	// default branch: corrupt cursor to invalid kind via empty rows after object clear mid-flight
	m.SidebarCursor = 0
	m.Objects = nil
	// still has tools/kinds
	m = m.pinSidebarAfterObjectsLoad()
}

func TestFinalGaps_UpdateMsgEdges(t *testing.T) {
	m := covModel(t)
	// ConnectionUpdated matching ID
	conn := m.Connections[0]
	conn.Name = "renamed"
	nm, _ := m.Update(types.ConnectionUpdatedMsg{Connection: conn})
	m = nm.(Model)
	// ConnectionDeleted last item with SelectedConnIdx past end
	m.Connections = []types.Connection{{ID: 1, Name: "only"}}
	m.SelectedConnIdx = 5
	nm, _ = m.Update(types.ConnectionDeletedMsg{ID: 1})
	m = nm.(Model)
	// SchemasLoaded without browser screen (no load objects)
	m.Screen = types.ScreenConnections
	nm, _ = m.Update(types.SchemasLoadedMsg{Schemas: []types.SchemaInfo{{Name: "public"}}})
	m = nm.(Model)
	// ObjectsLoaded default pin path
	m.Screen = types.ScreenBrowser
	m.SidebarCursor = 0
	// clear rows kind by setting empty before build - just load
	nm, _ = m.Update(types.ObjectsLoadedMsg{Objects: m.Objects})
	// sequenced msg wrong seq already covered
	// default Update fallthrough
	nm, _ = m.Update(struct{}{})
	_ = nm

	// keysSidebar empty rows
	m.Schemas = nil
	m.Objects = nil
	m.KindEnabled = map[NavSection]bool{}
	_, _ = m.keysSidebar("j")
	_, _ = m.onSidebarCursorMoved()
	m = m.syncSelectionFromSidebar()

	// beginObjectDetail non-detail msg path — LoadObjectDetail always returns TableDetailLoadedMsg
	// applyTableDetail empty schema object fill
	m = covModel(t)
	detail := m.TableDetail
	detail.Object.Schema = ""
	detail.Object.Name = "x"
	detail.Columns = nil
	detail.Props = nil
	_, _ = m.applyTableDetail(types.TableDetailLoadedMsg{Detail: detail})
	// applyTableData with CurrentObject and empty SchemaCols
	m.SchemaCols = nil
	m.CurrentObject = &m.Objects[0]
	_, _ = m.applyTableData(types.TableDataLoadedMsg{Result: types.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}}}})

	// keysObjectList g when idx already 0 — covered; empty name enter
	m.Objects = []types.SchemaObject{{Name: ""}}
	m.SelectedObjIdx = 0
	_, _ = m.keysObjectList("enter")
	_, _ = m.keysObjectList("D")
	// afterObjectCursorMove empty name
	_, _ = m.afterObjectCursorMove()

	// keysQuery suggest esc + force refresh
	m = covModel(t)
	nm, _ = m.openQuery()
	m = nm.(Model)
	m.QueryFocus = "editor"
	m.QuerySuggests = []string{"a", "b"}
	m.QuerySuggestIdx = 0
	_, _ = m.keysQuery("esc", key("esc"))
	m.QuerySuggests = []string{"a"}
	_, _ = m.keysQuery("ctrl+space", key("ctrl+space"))
	_, _ = m.keysQuery("ctrl+@", key("ctrl+@"))
	_, _ = m.keysQuery("ctrl+ ", key("ctrl+ "))
	_, _ = m.keysQuery("alt+/", key("alt+/"))
	// enter with suggests (falls through to textarea)
	m.QuerySuggests = []string{"a"}
	_, _ = m.keysQuery("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
}

func TestFinalGaps_PaintAndEditor(t *testing.T) {
	st := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	// cursorLine true, cursorCol < 0
	_ = paintSegment("abc", st, 0, -1, true)
	// cursor outside segment on cursor line
	_ = paintSegment("abc", st, 5, 0, true)  // cursor before start
	_ = paintSegment("abc", st, 0, 10, true) // cursor after end
	// cursor inside, cursorLine false branches on left/right — need cursorLine true for inside path
	_ = paintSegment("abcdef", st, 0, 2, true)
	_ = paintSegment("a", st, 0, 0, true)
	// cursorLine false with cursorCol>=0 uses early path
	_ = paintSegment("abc", st, 0, 1, false)

	// renderHighlightedSQLEditor: many lines for viewport, long lines for truncate
	m := covModel(t)
	ta := textarea.New()
	long := strings.Repeat("SELECT long_column_name_here ", 20)
	var lines []string
	for i := 0; i < 40; i++ {
		lines = append(lines, long)
	}
	ta.SetValue(strings.Join(lines, "\n"))
	ta.SetWidth(40)
	ta.SetHeight(8)
	m.QueryArea = &ta
	// move cursor far
	for i := 0; i < 30; i++ {
		ta.CursorDown()
	}
	m.QueryArea = &ta
	_ = m.renderHighlightedSQLEditor(40, 8, true)
	_ = m.renderHighlightedSQLEditor(40, 8, false)
	// empty value
	ta2 := textarea.New()
	ta2.SetValue("")
	m.QueryArea = &ta2
	_ = m.renderHighlightedSQLEditor(40, 5, true)
}

func TestFinalGaps_ERDComplex(t *testing.T) {
	// empty tables
	_ = renderERDDiagram(types.ERDGraph{}, 80, "")
	// tables with empty columns
	g := types.ERDGraph{
		Tables: []types.ERDTable{
			{Name: "a", Columns: nil},
			{Name: "b", Columns: []string{}},
			{Name: "c", Columns: []string{"id"}},
		},
		Edges: []types.FKEdge{
			{FromTable: "b", FromCols: []string{"a_id"}, ToTable: "a", ToCols: []string{"id"}},
			{FromTable: "missing", FromCols: []string{"x"}, ToTable: "a", ToCols: []string{"id"}},
			{FromTable: "c", FromCols: []string{"id"}, ToTable: "missing", ToCols: []string{"id"}},
			// reverse layer edge skipped
			{FromTable: "a", FromCols: []string{"id"}, ToTable: "c", ToCols: []string{"id"}},
			// self edge
			{FromTable: "c", FromCols: []string{"id"}, ToTable: "c", ToCols: []string{"id"}},
		},
	}
	_ = renderERDDiagram(g, 100, "")
	_ = renderERDDiagram(g, 30, "")

	// cycle for stack detection in erdLayers
	g2 := types.ERDGraph{
		Tables: []types.ERDTable{
			{Name: "x", Columns: []string{"id"}},
			{Name: "y", Columns: []string{"id"}},
		},
		Edges: []types.FKEdge{
			{FromTable: "x", FromCols: []string{"id"}, ToTable: "y", ToCols: []string{"id"}},
			{FromTable: "y", FromCols: []string{"id"}, ToTable: "x", ToCols: []string{"id"}},
		},
	}
	layers := erdLayers(g2)
	_ = erdBarycenterOrder(g2, layers)
	// single layer
	_ = erdBarycenterOrder(g2, [][]string{{"x"}})
	// orphan child without parent in pos
	g3 := types.ERDGraph{
		Tables: []types.ERDTable{
			{Name: "p", Columns: []string{"id"}},
			{Name: "c1", Columns: []string{"id"}},
			{Name: "c2", Columns: []string{"id"}},
			{Name: "c3", Columns: []string{"id"}},
		},
		Edges: []types.FKEdge{
			{FromTable: "c1", FromCols: []string{"id"}, ToTable: "p", ToCols: []string{"id"}},
			// c2 has parent not in previous layer pos
			{FromTable: "c2", FromCols: []string{"id"}, ToTable: "ghost", ToCols: []string{"id"}},
		},
	}
	L := erdLayers(g3)
	_ = erdBarycenterOrder(g3, L)
	// edges outside schema
	g4 := types.ERDGraph{
		Tables: []types.ERDTable{{Name: "only", Columns: []string{"id"}}},
		Edges:  []types.FKEdge{{FromTable: "only", FromCols: []string{"id"}, ToTable: "out", ToCols: []string{"id"}}},
	}
	_ = erdLayers(g4)

	// put with higher attr on wire merge
	c := newERDCanvas(20, 10)
	c.put(5, 5, '─', 1)
	c.put(5, 5, '│', 3) // a > attr
	c.put(5, 5, '│', 1) // a not greater
	// routeEdge orientations
	n1 := erdNode{name: "a", x: 1, y: 1, w: 8, h: 4, table: types.ERDTable{Name: "a", Columns: []string{"id"}}, layer: 0}
	n2 := erdNode{name: "b", x: 12, y: 1, w: 8, h: 4, table: types.ERDTable{Name: "b", Columns: []string{"id"}}, layer: 0}
	// same layer may still route
	c.routeEdge(n1, n2, "lbl", 0)
	n3 := erdNode{name: "c", x: 1, y: 6, w: 8, h: 4, table: types.ERDTable{Name: "c", Columns: []string{"id"}}, layer: 1}
	c.routeEdge(n1, n3, "", 0)
	// child left of parent
	n4 := erdNode{name: "d", x: 0, y: 6, w: 6, h: 3, table: types.ERDTable{Name: "d"}, layer: 1}
	c.routeEdge(n2, n4, "x", 0)
}

func TestFinalGaps_Views(t *testing.T) {
	m := covModel(t)
	// detail with no columns and no props status already via apply; view empty structure
	m.TableDetail = types.TableDetail{Object: types.SchemaObject{Name: "x", Schema: "public", Kind: types.ObjectSequence}}
	m.Screen = types.ScreenTableDetail
	_ = m.viewTableDetailContent(80, 20)
	_ = m.detailTabs(m.TableDetail)
	// props only tabs
	m.TableDetail.Props = []types.DetailProp{{Label: "a", Value: "b"}}
	_ = m.detailTabs(m.TableDetail)
	// definition tab
	m.TableDetail.CreateSQL = "CREATE"
	_ = m.detailTabs(m.TableDetail)

	// renderResultTable edges — zero cols, wide cells
	_ = m.renderResultTable(types.QueryResult{Columns: []string{"a"}, Rows: nil}, 0, 0, 5, 20)
	_ = m.renderResultTable(types.QueryResult{
		Columns: []string{"a"},
		Rows:    [][]string{{strings.Repeat("x", 100)}},
	}, 0, 0, 5, 20)

	// connections empty error branch at line 50
	m.Screen = types.ScreenConnections
	m.Connections = nil
	m.ConnectionError = ""
	m.Loading = false
	_ = m.viewConnections()
	// with SelectedConnIdx out of range
	m.Connections = []types.Connection{{Name: "a", Host: "h", Port: 1}}
	m.SelectedConnIdx = 10
	_ = m.viewConnections()

	// sidebar selected object kind rendering
	m = covModel(t)
	m.SidebarCursor = 0
	_ = m.viewSidebar(25, 30)
	// walk all cursors
	rows := m.buildSidebarRows()
	for i := range rows {
		m.SidebarCursor = i
		_ = m.viewSidebar(25, 30)
	}

	// workspace header read-only + no database
	m.ReadOnly = true
	m.CurrentDatabase = ""
	_ = m.viewWorkspaceHeader()
	m.CurrentConn = &types.Connection{Name: "n", Host: "h", Port: 1, ReadOnly: true}
	_ = m.viewWorkspaceHeader()

	// Rebuild completer with empty objects that still has cols
	m.Objects = nil
	m.SchemaCols = map[string][]string{"t": {"a"}}
	m.rebuildSQLCompleter()
}

func TestFinalGaps_BeginTableCmdNils(t *testing.T) {
	m := covModel(t)
	var cmd tea.Cmd
	m, cmd = m.beginTableDetail(m.Objects[0])
	if cmd != nil {
		_ = cmd()
	}
	m, cmd = m.beginTableData(m.Objects[0], 0, 10)
	if cmd != nil {
		_ = cmd()
	}
	nm, cmd := m.beginObjectDetail(m.Objects[3])
	m = nm.(Model)
	if cmd != nil {
		_ = cmd()
	}
}
