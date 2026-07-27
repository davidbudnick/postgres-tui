package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func sampleObjects() []types.SchemaObject {
	return []types.SchemaObject{
		{Schema: "public", Name: "orders", Kind: types.ObjectTable},
		{Schema: "public", Name: "order_items", Kind: types.ObjectTable},
		{Schema: "public", Name: "products", Kind: types.ObjectTable},
		{Schema: "public", Name: "users", Kind: types.ObjectTable},
		{Schema: "public", Name: "project_jobs", Kind: types.ObjectTable},
	}
}

func workspaceWithObjects(screen types.Screen) Model {
	m := NewModel()
	m.Screen = screen
	m.Width = 120
	m.Height = 40
	m.Objects = sampleObjects()
	m.CurrentDatabase = "demo"
	m.CurrentSchema = "public"
	m.Focus = focusSidebar
	return m
}

func TestObjectSearchTypesLetterJ(t *testing.T) {
	m := workspaceWithObjects(types.ScreenBrowser)
	nm, _ := m.startObjectSearch()
	m = nm.(Model)
	if !m.FilterActive {
		t.Fatal("expected FilterActive")
	}

	// The old bug bound "j" to commit — typing project must keep the query open.
	nm, _ = m.keysObjectSearch("p", tea.KeyPressMsg{Text: "p"})
	m = nm.(Model)
	nm, _ = m.keysObjectSearch("r", tea.KeyPressMsg{Text: "r"})
	m = nm.(Model)
	nm, _ = m.keysObjectSearch("o", tea.KeyPressMsg{Text: "o"})
	m = nm.(Model)
	nm, _ = m.keysObjectSearch("j", tea.KeyPressMsg{Text: "j"})
	m = nm.(Model)

	if !m.FilterActive {
		t.Fatal("j must not commit search")
	}
	if got := m.FilterInput.Value(); got != "proj" {
		t.Fatalf("value=%q want proj", got)
	}
	objs := m.filteredObjects()
	if len(objs) != 1 || objs[0].Name != "project_jobs" {
		t.Fatalf("filtered=%v", objs)
	}
}

func TestObjectSearchFiltersLive(t *testing.T) {
	m := workspaceWithObjects(types.ScreenBrowser)
	nm, _ := m.startObjectSearch()
	m = nm.(Model)
	for _, ch := range "order" {
		nm, _ = m.keysObjectSearch(string(ch), tea.KeyPressMsg{Text: string(ch)})
		m = nm.(Model)
	}
	names := make([]string, 0)
	for _, o := range m.filteredObjects() {
		names = append(names, o.Name)
	}
	if len(names) != 2 {
		t.Fatalf("want 2 matches got %v", names)
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "orders") || !strings.Contains(joined, "order_items") {
		t.Fatalf("unexpected %v", names)
	}
}

func TestObjectSearchFromTableData(t *testing.T) {
	m := workspaceWithObjects(types.ScreenTableData)
	nm, _ := m.Update(tea.KeyPressMsg{Text: "/", Code: '/'})
	m = nm.(Model)
	if !m.FilterActive {
		t.Fatal(" / should open search from table data")
	}
	if m.Focus != focusSidebar {
		t.Fatalf("focus=%v want sidebar", m.Focus)
	}

	for _, ch := range "users" {
		nm, _ = m.keysObjectSearch(string(ch), tea.KeyPressMsg{Text: string(ch)})
		m = nm.(Model)
	}
	objs := m.filteredObjects()
	if len(objs) != 1 || objs[0].Name != "users" {
		t.Fatalf("filtered=%v query=%q", objs, m.objectSearchQuery())
	}
}

func TestObjectSearchEscClears(t *testing.T) {
	m := workspaceWithObjects(types.ScreenBrowser)
	nm, _ := m.startObjectSearch()
	m = nm.(Model)
	for _, ch := range "ord" {
		nm, _ = m.keysObjectSearch(string(ch), tea.KeyPressMsg{Text: string(ch)})
		m = nm.(Model)
	}
	if m.FilterInput.Value() != "ord" {
		t.Fatalf("value=%q", m.FilterInput.Value())
	}
	nm, _ = m.keysObjectSearch("esc", tea.KeyPressMsg{})
	m = nm.(Model)
	if m.FilterActive {
		t.Fatal("esc should leave search")
	}
	if m.objectSearchQuery() != "" {
		t.Fatalf("query should clear got %q", m.objectSearchQuery())
	}
	if len(m.filteredObjects()) != len(sampleObjects()) {
		t.Fatalf("expected full list")
	}
}

func TestObjectSearchCommitKeepsFilter(t *testing.T) {
	m := workspaceWithObjects(types.ScreenBrowser)
	nm, _ := m.startObjectSearch()
	m = nm.(Model)
	m.FilterInput.SetValue("order")
	m.ObjectFilter = "order"
	m = m.commitObjectSearch()
	if m.FilterActive {
		t.Fatal("committed search should not stay active")
	}
	if m.objectSearchQuery() != "order" {
		t.Fatalf("query=%q", m.objectSearchQuery())
	}
	if len(m.filteredObjects()) != 2 {
		t.Fatalf("want 2 filtered objects")
	}
	// esc on browser clears applied filter
	nm, _ = m.keysBrowser("esc", tea.KeyPressMsg{})
	m = nm.(Model)
	if m.objectSearchQuery() != "" {
		t.Fatalf("esc should clear applied filter, got %q", m.objectSearchQuery())
	}
}

func TestObjectSearchEmptyTextFallback(t *testing.T) {
	// Terminals sometimes omit Key.Text; we synthesize from normalized key.
	m := workspaceWithObjects(types.ScreenBrowser)
	nm, _ := m.startObjectSearch()
	m = nm.(Model)
	nm, _ = m.keysObjectSearch("x", tea.KeyPressMsg{Code: 'x'}) // Text empty
	m = nm.(Model)
	if m.FilterInput.Value() != "x" {
		t.Fatalf("got %q want x", m.FilterInput.Value())
	}
}
