package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestResidual_UpdateBranches(t *testing.T) {
	m := khModel(t)
	m.Connections = []types.Connection{
		{ID: 10, Name: "a"},
		{ID: 20, Name: "b"},
	}
	m.SelectedConnIdx = 1
	nm, _ := m.Update(types.ConnectionUpdatedMsg{
		Connection: types.Connection{ID: 10, Name: "a-updated"},
	})
	m = nm.(Model)
	if m.Connections[0].Name != "a-updated" {
		t.Fatalf("updated=%+v", m.Connections[0])
	}

	m.SelectedConnIdx = 1
	nm, _ = m.Update(types.ConnectionDeletedMsg{ID: 20})
	m = nm.(Model)
	if len(m.Connections) != 1 || m.SelectedConnIdx != 0 {
		t.Fatalf("n=%d idx=%d", len(m.Connections), m.SelectedConnIdx)
	}

	// SchemasLoaded fallthrough: not browser
	m.Screen = types.ScreenConnections
	nm, cmd := m.Update(types.SchemasLoadedMsg{Schemas: []types.SchemaInfo{{Name: "public"}}})
	m = nm.(Model)
	if cmd != nil {
		t.Fatal("expected no load when not browser")
	}

	// ObjectsLoaded with cursor on object → default pin
	m = khWorkspace(t)
	m.Screen = types.ScreenBrowser
	m.SelectedObjIdx = 1
	m = m.syncSidebarCursorToObject()
	nm, _ = m.Update(types.ObjectsLoadedMsg{Objects: m.Objects})
	m = nm.(Model)

	// sequenced match applies
	obj := types.SchemaObject{Schema: "public", Name: "orders", Kind: types.ObjectTable}
	m.CurrentObject = &obj
	m.contentSeq = 7
	nm, _ = m.Update(sequencedDetailMsg{
		TableDetailLoadedMsg: types.TableDetailLoadedMsg{
			Detail: types.TableDetail{
				Object:  obj,
				Columns: []types.ColumnInfo{{Name: "id"}},
			},
		},
		seq: 7,
	})
	m = nm.(Model)
	if m.Screen != types.ScreenTableDetail {
		t.Fatalf("detail screen=%v", m.Screen)
	}

	m.contentSeq = 8
	m.CurrentObject = &obj
	nm, _ = m.Update(sequencedDataMsg{
		TableDataLoadedMsg: types.TableDataLoadedMsg{
			Result: types.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}}},
			Offset: 0,
		},
		seq: 8,
	})
	m = nm.(Model)
	if m.Screen != types.ScreenTableData {
		t.Fatalf("data screen=%v", m.Screen)
	}

	// unknown message
	nm, cmd = m.Update(struct{ n int }{1})
	if cmd != nil {
		t.Fatal("unknown")
	}
	_ = nm
}

func TestResidual_ApplySchemaFillAndCols(t *testing.T) {
	m := khModel(t)
	m.CurrentSchema = "public"
	m.CurrentObject = nil
	nm, _ := m.applyTableDetail(types.TableDetailLoadedMsg{
		Detail: types.TableDetail{
			Object:  types.SchemaObject{Name: "t1", Kind: types.ObjectTable},
			Columns: []types.ColumnInfo{{Name: "c"}},
		},
	})
	m = nm.(Model)
	if m.CurrentObject == nil || m.CurrentObject.Schema != "public" {
		t.Fatalf("schema fill=%+v", m.CurrentObject)
	}

	m.SchemaCols = nil
	obj := types.SchemaObject{Schema: "public", Name: "t1", Kind: types.ObjectTable}
	m.CurrentObject = &obj
	nm, _ = m.applyTableData(types.TableDataLoadedMsg{
		Result: types.QueryResult{Columns: []string{"id", "n"}, Rows: [][]string{{"1", "a"}}},
		Offset: 0,
	})
	m = nm.(Model)
	if m.SchemaCols == nil || len(m.SchemaCols["public.t1"]) != 2 {
		t.Fatalf("cols=%v", m.SchemaCols)
	}
}

func TestResidual_ObjectListGAndAfterEmpty(t *testing.T) {
	m := khWorkspace(t)
	m.SelectedObjIdx = 2
	nm, _ := m.keysObjectList("g")
	m = nm.(Model)
	if m.SelectedObjIdx != 0 {
		t.Fatalf("idx=%d", m.SelectedObjIdx)
	}

	m.Objects = nil
	m.SelectedObjIdx = 0
	nm, cmd := m.afterObjectCursorMove()
	if cmd != nil {
		t.Fatal("empty after")
	}
	_ = nm
}

func TestResidual_BeginWrappersExecute(t *testing.T) {
	m := khWorkspace(t)
	_, cmd := m.beginObjectDetail(types.SchemaObject{
		Schema: "public", Name: "users_id_seq", Kind: types.ObjectSequence,
	})
	if cmd == nil {
		t.Fatal("cmd")
	}
	msg := cmd()
	if _, ok := msg.(sequencedDetailMsg); !ok {
		t.Fatalf("got %T", msg)
	}

	_, cmd = m.beginTableDetail(types.SchemaObject{
		Schema: "public", Name: "orders", Kind: types.ObjectTable,
	})
	if cmd == nil {
		t.Fatal("detail cmd")
	}
	msg = cmd()
	if _, ok := msg.(sequencedDetailMsg); !ok {
		t.Fatalf("got %T", msg)
	}

	_, cmd = m.beginTableData(types.SchemaObject{
		Schema: "public", Name: "orders", Kind: types.ObjectTable,
	}, 0, 10)
	if cmd == nil {
		t.Fatal("data cmd")
	}
	msg = cmd()
	if _, ok := msg.(sequencedDataMsg); !ok {
		t.Fatalf("got %T", msg)
	}
}

func TestResidual_SidebarEmptyLike(t *testing.T) {
	// buildSidebarRows always emits kinds+tools; still exercise cursor helpers on sparse model.
	m := NewModel()
	m.Schemas = nil
	m.Objects = nil
	m.KindEnabled = map[NavSection]bool{}
	nm, _ := m.keysSidebar("j")
	m = nm.(Model)
	nm, _ = m.onSidebarCursorMoved()
	m = nm.(Model)
	m = m.syncSelectionFromSidebar()
	_ = m
}

func TestResidual_NormalizeKeyEdges(t *testing.T) {
	// Prefer String path "shift+tab" when Code is not KeyTab (rare terminals).
	// Empty Text + space Text skipped path already exercised; ensure Code-only space.
	if got := normalizeKey(tea.KeyPressMsg{Code: tea.KeySpace}); got != "space" {
		t.Fatalf("space=%q", got)
	}
	// shift+ multi-rune rest falls through
	if got := normalizeKey(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift}); got == "" {
		t.Fatal("shift+enter empty")
	}
}
