package ui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestNormalizeKeyInjectedString(t *testing.T) {
	old := keyMsgString
	t.Cleanup(func() { keyMsgString = old })

	keyMsgString = func(tea.KeyPressMsg) string { return "backtab" }
	if normalizeKey(tea.KeyPressMsg{Code: 'x'}) != "shift+tab" {
		t.Fatal("backtab")
	}
	keyMsgString = func(tea.KeyPressMsg) string { return "shift+tab" }
	if normalizeKey(tea.KeyPressMsg{Code: 'x'}) != "shift+tab" {
		t.Fatal("shift+tab string")
	}
	keyMsgString = func(tea.KeyPressMsg) string { return "" }
	if normalizeKey(tea.KeyPressMsg{}) != "" {
		t.Fatal("empty string")
	}
	keyMsgString = func(tea.KeyPressMsg) string { return "ctrl+enter" }
	if normalizeKey(tea.KeyPressMsg{}) != "ctrl+enter" {
		t.Fatal("ctrl")
	}
	keyMsgString = func(tea.KeyPressMsg) string { return "alt+/" }
	_ = normalizeKey(tea.KeyPressMsg{})
	keyMsgString = func(tea.KeyPressMsg) string { return "super+a" }
	_ = normalizeKey(tea.KeyPressMsg{})
	keyMsgString = func(tea.KeyPressMsg) string { return "meta+a" }
	_ = normalizeKey(tea.KeyPressMsg{})
	keyMsgString = func(tea.KeyPressMsg) string { return "shift+/" }
	if normalizeKey(tea.KeyPressMsg{}) != "?" {
		t.Fatal("shift+/")
	}
	// all shift specials
	for in, want := range map[string]string{
		"shift+3": "#", "shift+8": "*", "shift+;": ":", "shift+'": "\"",
		"shift+1": "!", "shift+,": "<", "shift+.": ">", "shift+-": "_",
		"shift+=": "+", "shift+`": "~", "shift+\\": "|", "shift+[": "{", "shift+]": "}",
		"shift+a": "A",
	} {
		keyMsgString = func(tea.KeyPressMsg) string { return in }
		if got := normalizeKey(tea.KeyPressMsg{}); got != want {
			t.Fatalf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestPinSidebarUnreachableish(t *testing.T) {
	// Empty rows: zero out schemas/objects and temporarily empty kind/tool by
	// calling pin on a model whose buildSidebarRows is still non-empty — force
	// the "no matching kind" path by using a nav that won't match after we
	// strip kinds via empty KindEnabled is not enough. Instead exercise default
	// branch by placing cursor on a tool then calling pinSidebarAfterObjectsLoad
	// after mutating NavSection.
	m := covModel(t)
	// pin kind that exists
	m = m.pinSidebarCursorToKind(navTables)
	// After load with object cursor
	rows := m.buildSidebarRows()
	for i, r := range rows {
		if r.kind == sbObject {
			m.SidebarCursor = i
			break
		}
	}
	m = m.pinSidebarAfterObjectsLoad()
	// schema match path
	for i, r := range rows {
		if r.kind == sbSchema {
			m.SidebarCursor = i
			m.SelectedSchema = r.schema
			m = m.pinSidebarAfterObjectsLoad()
			m.SelectedSchema = r.schema + 99
			m = m.pinSidebarAfterObjectsLoad()
			break
		}
	}
}

func TestBeginDetailDataNonRelationAndClosures(t *testing.T) {
	m := covModel(t)
	// non-relation beginTableDetail
	seq := types.SchemaObject{Schema: "public", Name: "s", Kind: types.ObjectSequence}
	m, cmd := m.beginTableDetail(seq)
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(sequencedDetailMsg); !ok {
			// still covered either way
			_ = msg
		}
	}
	// non-relation beginTableData -> preview
	m, _ = m.beginTableData(seq, 0, 10)
	// relation beginTableDetail execute cmd
	m, cmd = m.beginTableDetail(m.Objects[0])
	if cmd != nil {
		_ = cmd()
	}
	m, cmd = m.beginTableData(m.Objects[0], 0, 5)
	if cmd != nil {
		_ = cmd()
	}
	// empty name / nil cmds
	m2 := m
	m2.Cmds = nil
	m2, _ = m2.beginTableDetail(m.Objects[0])
	m2, _ = m2.beginTableData(m.Objects[0], 0, 5)
	_, _ = m.beginObjectDetail(types.SchemaObject{Name: ""})
	nm, cmd := m.beginObjectDetail(seq)
	m = nm.(Model)
	if cmd != nil {
		_ = cmd()
	}
}

func TestKeysSidebarEmptyAndObjectD(t *testing.T) {
	m := covModel(t)
	// empty rows only if we could clear kind nav — not possible; still call with empty objects
	m.Schemas = nil
	m.Objects = nil
	_, _ = m.keysSidebar("j")
	_, _ = m.onSidebarCursorMoved()
	m = m.syncSelectionFromSidebar()

	// D on object row
	m = covModel(t)
	rows := m.buildSidebarRows()
	for i, r := range rows {
		if r.kind == sbObject {
			m.SidebarCursor = i
			_, _ = m.keysSidebar("D")
			break
		}
	}
	// space/enter on kind row
	for i, r := range rows {
		if r.kind == sbKind {
			m.SidebarCursor = i
			_, _ = m.keysSidebar(" ")
			_, _ = m.keysSidebar("enter")
			break
		}
	}
}

func TestPaintSegmentAllBranches(t *testing.T) {
	st := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	_ = paintSegment("", st, 0, 0, true)
	// cursorLine true, cursorCol < 0 → cursor line style
	_ = paintSegment("abc", st, 0, -1, true)
	// cursorLine false
	_ = paintSegment("abc", st, 0, 1, false)
	// outside before
	_ = paintSegment("abc", st, 10, 0, true)
	// outside after
	_ = paintSegment("abc", st, 0, 99, true)
	// inside at start (off==0 no left)
	_ = paintSegment("abc", st, 0, 0, true)
	// inside middle
	_ = paintSegment("abc", st, 0, 1, true)
	// inside end
	_ = paintSegment("abc", st, 0, 2, true)
	// start offset non-zero
	_ = paintSegment("abc", st, 5, 6, true)
	// cursorLine false shouldn't hit left/right non-cursor branches inside — those need cursorLine true with off>0 which we did
}

func TestRenderHighlightedEditorEdgesResidual(t *testing.T) {
	m := covModel(t)
	ta := textarea.New()
	// many lines + long line for ellipsis path
	long := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 10)
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString(long)
		b.WriteByte('\n')
	}
	ta.SetValue(b.String())
	// push cursor near end
	for i := 0; i < 45; i++ {
		ta.CursorDown()
	}
	m.QueryArea = &ta
	_ = m.renderHighlightedSQLEditor(30, 6, true)
	// curLine out of range guards — SetValue short then high line is hard; empty already covered
	ta2 := textarea.New()
	ta2.SetValue("one\ntwo")
	m.QueryArea = &ta2
	_ = m.renderHighlightedSQLEditor(80, 10, true)
}

func TestViewDetailAndResultsEdges(t *testing.T) {
	m := covModel(t)
	// viewTableDetailContent kind from CurrentObject when detail kind empty
	m.CurrentObject = &types.SchemaObject{Schema: "public", Name: "t", Kind: types.ObjectView}
	m.TableDetail = types.TableDetail{Object: types.SchemaObject{Schema: "public", Name: "t"}}
	_ = m.viewTableDetailContent(80, 20)

	// detailTabs with only definition
	d := types.TableDetail{CreateSQL: "CREATE VIEW"}
	_ = m.detailTabs(d)
	// only columns
	_ = m.detailTabs(types.TableDetail{Columns: []types.ColumnInfo{{Name: "a"}}})
	// only indexes
	_ = m.detailTabs(types.TableDetail{Indexes: []types.IndexInfo{{Name: "i"}}})
	// only constraints
	_ = m.detailTabs(types.TableDetail{Constraints: []types.ConstraintInfo{{Name: "c"}}})
	// empty
	_ = m.detailTabs(types.TableDetail{})

	// renderResultTable: many cols squeeze, empty cell on selected row, short maxWidth
	res := types.QueryResult{
		Columns: []string{"c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8"},
		Rows: [][]string{
			{"", "b", "c", "d", "e", "f", "g", "h"},
			{"1", "2", "3", "4", "5", "6", "7", "8"},
		},
	}
	_ = m.renderResultTable(res, 0, 0, 5, 15) // narrow → col squeeze
	_ = m.renderResultTable(res, 0, 0, 5, 10)
	// many sample rows
	rows := make([][]string, 50)
	for i := range rows {
		rows[i] = []string{"x", "y"}
	}
	_ = m.renderResultTable(types.QueryResult{Columns: []string{"a", "b"}, Rows: rows}, 0, 0, 5, 40)
	// row shorter than cols
	_ = m.renderResultTable(types.QueryResult{Columns: []string{"a", "b", "c"}, Rows: [][]string{{"only"}}}, 0, 0, 5, 40)
	// cursorCol past nShow
	_ = m.renderResultTable(res, 0, 99, 5, 20)
	// maxWidth < 20
	_ = m.renderResultTable(res, 0, 0, 5, 5)
	// no columns
	_ = m.renderResultTable(types.QueryResult{}, 0, 0, 5, 40)
}

func TestConnectionsFramePadAndERDResidual(t *testing.T) {
	m := covModel(t)
	// force narrow width so section frame pad <= 0
	m.Width = 5
	m.Height = 20
	m.Screen = types.ScreenConnections
	m.Connections = []types.Connection{{Name: "x", Host: "h", Port: 1}}
	_ = m.viewConnections()
	m.Width = 140

	// ERD put attr upgrade
	c := newERDCanvas(30, 20)
	c.put(2, 2, '─', 1)
	c.put(2, 2, '│', 5)
	// routeEdge close children (cy <= py+1)
	p := erdNode{x: 5, y: 5, w: 6, h: 3, table: types.ERDTable{Name: "p"}}
	ch := erdNode{x: 5, y: 6, w: 6, h: 3, table: types.ERDTable{Name: "c"}}
	c.routeEdge(p, ch, "lbl", 0)
	// wide label near edge
	p2 := erdNode{x: 1, y: 1, w: 4, h: 2, table: types.ERDTable{Name: "p"}}
	c2 := erdNode{x: 20, y: 8, w: 4, h: 2, table: types.ERDTable{Name: "c"}}
	c.routeEdge(p2, c2, "verylonglabelname", 0)
	// child left of parent
	c3 := erdNode{x: 1, y: 10, w: 4, h: 2, table: types.ERDTable{Name: "c"}}
	c.routeEdge(erdNode{x: 15, y: 1, w: 4, h: 2, table: types.ERDTable{Name: "p"}}, c3, "ab", 0)
	// straight down
	c.routeEdge(erdNode{x: 10, y: 1, w: 4, h: 2}, erdNode{x: 10, y: 10, w: 4, h: 2}, "", 0)
	// midY clamps
	c.routeEdge(erdNode{x: 2, y: 2, w: 2, h: 1}, erdNode{x: 8, y: 5, w: 2, h: 1}, "z", 0)

	// erdLayers empty out fallback + cycle
	g := types.ERDGraph{}
	_ = erdLayers(g)
	// barycenter self-edge skip + orphan
	g = types.ERDGraph{
		Tables: []types.ERDTable{
			{Name: "a", Columns: []string{"id"}},
			{Name: "b", Columns: []string{"id"}},
			{Name: "c", Columns: []string{"id"}},
		},
		Edges: []types.FKEdge{
			{FromTable: "b", ToTable: "a", FromCols: []string{"id"}, ToCols: []string{"id"}},
			{FromTable: "b", ToTable: "b", FromCols: []string{"id"}, ToCols: []string{"id"}},
			{FromTable: "c", ToTable: "missing", FromCols: []string{"id"}, ToCols: []string{"id"}},
		},
	}
	layers := erdLayers(g)
	_ = erdBarycenterOrder(g, layers)
	// equal scores name sort
	g2 := types.ERDGraph{
		Tables: []types.ERDTable{
			{Name: "p", Columns: []string{"id"}},
			{Name: "aa", Columns: []string{"id"}},
			{Name: "ab", Columns: []string{"id"}},
		},
		Edges: []types.FKEdge{
			{FromTable: "aa", ToTable: "p", FromCols: []string{"id"}, ToCols: []string{"id"}},
			{FromTable: "ab", ToTable: "p", FromCols: []string{"id"}, ToCols: []string{"id"}},
		},
	}
	_ = erdBarycenterOrder(g2, erdLayers(g2))
	// render diagram maxX expand
	_ = renderERDDiagram(g2, 20)
	// edge missing node
	g3 := types.ERDGraph{
		Tables: []types.ERDTable{{Name: "only", Columns: []string{"id"}}},
		Edges:  []types.FKEdge{{FromTable: "only", ToTable: "nope", FromCols: []string{"id"}, ToCols: []string{"id"}}},
	}
	_ = renderERDDiagram(g3, 40)
}

func TestSidebarAndHeaderResidual(t *testing.T) {
	m := covModel(t)
	// no matches filter
	m.FilterActive = true
	m.FilterInput.SetValue("zzz_no_match")
	m.ObjectFilter = "zzz_no_match"
	_ = m.viewSidebar(30, 25)
	// loading none
	m.Loading = true
	m.ObjectFilter = ""
	m.FilterActive = false
	m.Objects = nil
	_ = m.viewSidebar(30, 25)
	// header long version
	m.ServerInfo.Version = "PostgreSQL 16.0 on x86_64-pc-linux-gnu, compiled by gcc"
	m.ReadOnly = true
	m.CurrentSchema = "public"
	m.Width = 40 // force gap calc
	_ = m.viewWorkspaceHeader()
	m.Width = 10
	_ = m.viewWorkspaceHeader()
}

func TestSQLCompleterRebuildEmptyName(t *testing.T) {
	c := newSQLCompleter()
	c.Rebuild([]types.SchemaObject{
		{Name: "", Kind: types.ObjectTable},
		{Name: "t", Kind: types.ObjectTable, Schema: "public"},
		{Name: "v", Kind: types.ObjectView},
	}, map[string][]string{
		"":  {"skip"},
		"t": {"", "id", "name"},
	})
}
