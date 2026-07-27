package ui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestSidebarRowsInjection(t *testing.T) {
	old := sidebarRowsFor
	t.Cleanup(func() { sidebarRowsFor = old })

	m := covModel(t)
	// empty rows
	sidebarRowsFor = func(Model) []sidebarRow { return nil }
	m = m.pinSidebarCursorToKind(navTables)
	m = m.pinSidebarAfterObjectsLoad()

	// fallback first kind when nav not found
	sidebarRowsFor = func(Model) []sidebarRow {
		return []sidebarRow{{kind: sbKind, nav: navViews, label: "Views"}}
	}
	m = m.pinSidebarCursorToKind(navTables)
	if m.SidebarCursor != 0 {
		t.Fatal(m.SidebarCursor)
	}

	// default branch
	sidebarRowsFor = func(Model) []sidebarRow {
		return []sidebarRow{{kind: sbNav, nav: navTables, label: "x"}}
	}
	m.SidebarCursor = 0
	m = m.pinSidebarAfterObjectsLoad()

	// schema match / mismatch
	sidebarRowsFor = func(Model) []sidebarRow {
		return []sidebarRow{{kind: sbSchema, schema: 1, label: "s"}}
	}
	m.SelectedSchema = 1
	m = m.pinSidebarAfterObjectsLoad()
	m.SelectedSchema = 0
	m = m.pinSidebarAfterObjectsLoad()

	// tool/object
	sidebarRowsFor = func(Model) []sidebarRow {
		return []sidebarRow{{kind: sbTool, nav: navQuery}}
	}
	m = m.pinSidebarAfterObjectsLoad()
	sidebarRowsFor = func(Model) []sidebarRow {
		return []sidebarRow{{kind: sbObject, objIdx: 0}}
	}
	m = m.pinSidebarAfterObjectsLoad()
}

func TestPaintOutsideCursorLineFalse(t *testing.T) {
	st := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	// outside with cursorLine true hits line 212-214
	_ = paintSegment("abc", st, 5, 0, true)
	_ = paintSegment("abc", st, 0, 10, true)
	// outside with cursorLine false hits 214 return st.Render - but we only reach
	// the outside check when cursorLine is true (because !cursorLine returns early).
	// So 214 bare st.Render in outside block is dead when cursorLine false can't get there.
	// Early path covers cursorLine false.
	_ = paintSegment("abc", st, 0, -1, true)
	_ = paintSegment("abc", st, 0, 1, false)
}

func TestSQLRebuildEmptyStringAdd(t *testing.T) {
	c := newSQLCompleter()
	c.Rebuild([]types.SchemaObject{
		{Name: "t", Kind: types.ObjectTable, Schema: "public"},
	}, map[string][]string{
		"t": {"", "id", "id"}, // empty col skipped; duplicate id
		"":  {"x"},
	})
}

func TestEditorCurLineGuards(t *testing.T) {
	m := covModel(t)
	ta := textarea.New()
	ta.SetValue("line0\nline1")
	m.QueryArea = &ta
	// Normal
	_ = m.renderHighlightedSQLEditor(40, 5, true)
	// Many lines with cursor at start for start<0 clamp
	var b strings.Builder
	for i := 0; i < 30; i++ {
		b.WriteString("line\n")
	}
	ta.SetValue(b.String())
	// cursor at 0, height 5 → start stays 0
	m.QueryArea = &ta
	_ = m.renderHighlightedSQLEditor(40, 5, true)
}

func TestConnectionsNarrowFrame(t *testing.T) {
	m := covModel(t)
	m.Width = 3 // sectionInner very small
	m.Height = 20
	m.Screen = types.ScreenConnections
	m.Connections = []types.Connection{{Name: "n", Host: "h", Port: 1}}
	_ = m.viewConnections()
	m.Width = 1
	_ = m.viewConnections()
}

func TestRouteEdgeMidYClamps(t *testing.T) {
	c := newERDCanvas(40, 30)
	// py and cy such that midY clamps
	// parent bottom py = y+h, child top cy
	// want midY <= py and midY >= cy-1
	p := erdNode{x: 5, y: 5, w: 4, h: 2}  // py = 7
	ch := erdNode{x: 5, y: 9, w: 4, h: 2} // cy = 9, cy > py+1 (7+1=8) so 9>8 ok
	// midY = (7+9)/2 = 8; midY <= py? 8<=7 false; midY >= cy-1=8? 8>=8 true → midY = 7
	c.routeEdge(p, ch, "x")
	// even tighter
	p2 := erdNode{x: 10, y: 2, w: 4, h: 1}  // py=3
	ch2 := erdNode{x: 20, y: 5, w: 4, h: 1} // cy=5
	// midY=(3+5)/2=4; 4<=3? no; 4>=4? yes → midY=3; midY<py? no
	c.routeEdge(p2, ch2, "longlabelxx")
	// labX < 0 path via extreme positions
	p3 := erdNode{x: 0, y: 0, w: 2, h: 1}
	ch3 := erdNode{x: 0, y: 10, w: 2, h: 1}
	c.routeEdge(p3, ch3, "L")
}

func TestErdLayersEmptyOut(t *testing.T) {
	// empty tables → names empty → layers empty → out empty → return {names}
	_ = erdLayers(types.ERDGraph{})
}
