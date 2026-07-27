package ui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestOpenDatabasePicker(t *testing.T) {
	m := khModel(t)
	nm, cmd := m.openDatabasePicker()
	m = nm.(Model)
	if m.StatusMsg != "Connect first" || cmd != nil {
		t.Fatalf("disconnected: status=%q cmd=%v", m.StatusMsg, cmd != nil)
	}

	m = khWorkspace(t)
	m.Databases = nil
	nm, cmd = m.openDatabasePicker()
	m = nm.(Model)
	if m.Screen != types.ScreenDatabasePicker {
		t.Fatalf("screen=%v", m.Screen)
	}
	if cmd == nil || !m.Loading {
		t.Fatal("expected LoadDatabases when empty")
	}

	m.Databases = []types.DatabaseInfo{
		{Name: "analytics", Owner: "pg", SizePretty: "1 MB"},
		{Name: "demo", Owner: "app", SizePretty: "12 MB"},
		{Name: "postgres", Owner: "pg", SizePretty: "8 MB"},
	}
	m.CurrentDatabase = "demo"
	m.Loading = false
	nm, cmd = m.openDatabasePicker()
	m = nm.(Model)
	if cmd != nil {
		t.Fatal("no load when list present")
	}
	if m.PaletteIdx != 1 {
		t.Fatalf("preselect idx=%d", m.PaletteIdx)
	}
	if m.Inputs == nil || m.Inputs.PaletteInput.Placeholder != "Filter databases…" {
		t.Fatal("placeholder")
	}

	// Re-open while already on picker keeps PrevScreen.
	prev := m.PrevScreen
	nm, _ = m.openDatabasePicker()
	m = nm.(Model)
	if m.PrevScreen != prev {
		t.Fatalf("prev changed: %v -> %v", prev, m.PrevScreen)
	}
}

func TestFilteredDatabases(t *testing.T) {
	m := khWorkspace(t)
	m.Databases = []types.DatabaseInfo{
		{Name: "analytics", Owner: "reader"},
		{Name: "demo", Owner: "app"},
		{Name: "postgres", Owner: "pg"},
	}
	m.Inputs.PaletteInput.SetValue("")
	if n := len(m.filteredDatabases()); n != 3 {
		t.Fatalf("all=%d", n)
	}
	m.Inputs.PaletteInput.SetValue("DEMO")
	got := m.filteredDatabases()
	if len(got) != 1 || got[0].Name != "demo" {
		t.Fatalf("name filter: %+v", got)
	}
	m.Inputs.PaletteInput.SetValue("reader")
	got = m.filteredDatabases()
	if len(got) != 1 || got[0].Name != "analytics" {
		t.Fatalf("owner filter: %+v", got)
	}
	m.Inputs.PaletteInput.SetValue("zzzz")
	if len(m.filteredDatabases()) != 0 {
		t.Fatal("no match")
	}
}

func TestKeysDatabasePicker(t *testing.T) {
	m := khWorkspace(t)
	m.Databases = []types.DatabaseInfo{
		{Name: "analytics", SizePretty: "1 MB"},
		{Name: "demo", SizePretty: "2 MB"},
		{Name: "postgres", SizePretty: "3 MB"},
	}
	m.CurrentDatabase = "demo"
	nm, _ := m.openDatabasePicker()
	m = nm.(Model)

	nm, _ = m.keysDatabasePicker("down", tea.KeyPressMsg{})
	m = nm.(Model)
	if m.PaletteIdx != 2 {
		t.Fatalf("down idx=%d", m.PaletteIdx)
	}
	nm, _ = m.keysDatabasePicker("j", tea.KeyPressMsg{})
	m = nm.(Model)
	if m.PaletteIdx != 2 {
		t.Fatalf("clamp bottom idx=%d", m.PaletteIdx)
	}
	nm, _ = m.keysDatabasePicker("up", tea.KeyPressMsg{})
	m = nm.(Model)
	if m.PaletteIdx != 1 {
		t.Fatalf("up idx=%d", m.PaletteIdx)
	}
	nm, _ = m.keysDatabasePicker("k", tea.KeyPressMsg{})
	m = nm.(Model)
	if m.PaletteIdx != 0 {
		t.Fatalf("k idx=%d", m.PaletteIdx)
	}
	nm, _ = m.keysDatabasePicker("ctrl+n", tea.KeyPressMsg{})
	m = nm.(Model)
	nm, _ = m.keysDatabasePicker("ctrl+p", tea.KeyPressMsg{})
	m = nm.(Model)

	// enter current DB closes without select cmd
	m.PaletteIdx = 1
	nm, cmd := m.keysDatabasePicker("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	m = nm.(Model)
	if cmd != nil || m.Screen != types.ScreenBrowser {
		t.Fatalf("current db: screen=%v cmd=%v", m.Screen, cmd != nil)
	}

	// enter other DB issues SelectDatabase
	nm, _ = m.openDatabasePicker()
	m = nm.(Model)
	m.PaletteIdx = 0
	nm, cmd = m.keysDatabasePicker("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	m = nm.(Model)
	if cmd == nil || !m.Loading {
		t.Fatal("expected SelectDatabase")
	}
	if m.Screen != types.ScreenBrowser {
		t.Fatalf("screen after select=%v", m.Screen)
	}

	// esc
	nm, _ = m.openDatabasePicker()
	m = nm.(Model)
	nm, _ = m.keysDatabasePicker("esc", tea.KeyPressMsg{Code: tea.KeyEsc})
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatalf("esc screen=%v", m.Screen)
	}

	// empty list enter / filter typing
	m.Databases = nil
	m.Screen = types.ScreenDatabasePicker
	m.PrevScreen = types.ScreenBrowser
	nm, cmd = m.keysDatabasePicker("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("empty enter")
	}
	m.Databases = []types.DatabaseInfo{{Name: "demo"}, {Name: "other"}}
	m.Inputs.PaletteInput.SetValue("")
	nm, _ = m.keysDatabasePicker("o", tea.KeyPressMsg{Text: "o", Code: 'o'})
	m = nm.(Model)
	if m.PaletteIdx != 0 {
		t.Fatalf("type resets idx=%d", m.PaletteIdx)
	}

	// nil cmds + empty items branches
	m.Cmds = nil
	m.Databases = []types.DatabaseInfo{{Name: "x"}}
	m.PaletteIdx = 0
	nm, cmd = m.keysDatabasePicker("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("nil cmds")
	}
	_ = nm

	// PrevScreen self-heal on esc
	m = khWorkspace(t)
	m.Screen = types.ScreenDatabasePicker
	m.PrevScreen = types.ScreenDatabasePicker
	nm, _ = m.keysDatabasePicker("esc", tea.KeyPressMsg{Code: tea.KeyEsc})
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatalf("esc self-heal=%v", m.Screen)
	}

	// nil inputs path
	m.Inputs = nil
	m.Databases = []types.DatabaseInfo{{Name: "a"}, {Name: "b"}}
	m.Screen = types.ScreenDatabasePicker
	nm, _ = m.keysDatabasePicker("down", tea.KeyPressMsg{})
	m = nm.(Model)
	nm, _ = m.keysDatabasePicker("x", tea.KeyPressMsg{Text: "x"})
	_ = nm
}

func TestDatabasePickerHotkeyAndView(t *testing.T) {
	m := khWorkspace(t)
	m.Databases = []types.DatabaseInfo{
		{Name: "analytics", SizePretty: "1 MB"},
		{Name: "demo", SizePretty: "2 MB"},
		{Name: "postgres", SizePretty: "3 MB"},
	}
	m.CurrentDatabase = "demo"
	m.Width = 100
	m.Height = 30

	nm, cmd := m.Update(tea.KeyPressMsg{Text: "b", Code: 'b'})
	m = nm.(Model)
	if m.Screen != types.ScreenDatabasePicker || cmd != nil {
		t.Fatalf("hotkey b: screen=%v cmd=%v", m.Screen, cmd != nil)
	}
	view := m.viewDatabasePicker()
	if view == "" {
		t.Fatal("empty view")
	}
	_ = m.render()
	_ = m.getScreenView()

	// loading empty list
	m.Databases = nil
	m.Loading = true
	_ = m.viewDatabasePicker()

	// no matches
	m.Databases = []types.DatabaseInfo{{Name: "demo"}}
	m.Loading = false
	m.Inputs.PaletteInput.SetValue("zzz")
	_ = m.viewDatabasePicker()

	// many rows for windowing
	m.Inputs.PaletteInput.SetValue("")
	m.Databases = nil
	for i := 0; i < 20; i++ {
		m.Databases = append(m.Databases, types.DatabaseInfo{
			Name: "db" + string(rune('a'+i%26)) + string(rune('0'+i%10)), SizePretty: "1k",
		})
	}
	m.PaletteIdx = 15
	_ = m.viewDatabasePicker()

	// palette path
	m = khWorkspace(t)
	m.Databases = []types.DatabaseInfo{{Name: "demo"}}
	nm, cmd = m.runPalette("databases")
	m = nm.(Model)
	if m.Screen != types.ScreenDatabasePicker {
		t.Fatalf("palette databases screen=%v", m.Screen)
	}
	_ = cmd

	// loaded msg while picker open preselects
	m.CurrentDatabase = "app"
	nm, _ = m.Update(types.DatabasesLoadedMsg{Databases: []types.DatabaseInfo{
		{Name: "analytics"},
		{Name: "app"},
	}})
	m = nm.(Model)
	if m.PaletteIdx != 1 || m.Loading {
		t.Fatalf("loaded preselect idx=%d loading=%v", m.PaletteIdx, m.Loading)
	}

	// error while picker stays quiet
	nm, _ = m.Update(types.DatabasesLoadedMsg{Err: errors.New("load failed")})
	m = nm.(Model)
	if m.Err != nil {
		t.Fatalf("picker load err should not clobber: %v", m.Err)
	}

	// typingContext includes picker
	m.Screen = types.ScreenDatabasePicker
	if !m.typingContext() {
		t.Fatal("picker is typing context")
	}

	// handleKeyPress routes picker keys
	m.Databases = []types.DatabaseInfo{
		{Name: "analytics", SizePretty: "1 MB"},
		{Name: "demo", SizePretty: "2 MB"},
	}
	m.CurrentDatabase = "demo"
	m.Screen = types.ScreenDatabasePicker
	m.PrevScreen = types.ScreenBrowser
	m.PaletteIdx = 1
	// green branch: current DB visible but not selected
	m.PaletteIdx = 0
	_ = m.viewDatabasePicker()

	nm, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = nm.(Model)
	if m.PaletteIdx != 1 {
		t.Fatalf("routed down idx=%d", m.PaletteIdx)
	}

	// enter with PrevScreen stuck on picker
	m.Screen = types.ScreenDatabasePicker
	m.PrevScreen = types.ScreenDatabasePicker
	m.PaletteIdx = 0
	m.CurrentDatabase = "demo"
	m.Databases = []types.DatabaseInfo{{Name: "analytics"}, {Name: "demo"}}
	nm, cmd = m.keysDatabasePicker("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	m = nm.(Model)
	if cmd == nil || m.Screen != types.ScreenBrowser {
		t.Fatalf("enter self-heal prev: screen=%v cmd=%v", m.Screen, cmd != nil)
	}

	// screen string
	if types.ScreenDatabasePicker.String() != "Switch Database" {
		t.Fatal(types.ScreenDatabasePicker.String())
	}
}
