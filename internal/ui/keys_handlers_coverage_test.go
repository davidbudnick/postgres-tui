package ui

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/cmd"
	"github.com/davidbudnick/postgres-tui/internal/db"
	"github.com/davidbudnick/postgres-tui/internal/testutil"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func khModel(t *testing.T) Model {
	t.Helper()
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	m.Width = 120
	m.Height = 40
	m.Cmds = cmd.NewCommands(cfg, testutil.NewMockPG())
	return m
}

func khWorkspace(t *testing.T) Model {
	t.Helper()
	m := khModel(t)
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
		{Schema: "public", Name: "users_id_seq", Kind: types.ObjectSequence},
	}
	m.SelectedObjIdx = 0
	return m.syncSidebarCursorToObject()
}

func khPress(code rune, mod tea.KeyMod) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: mod}
}

func khText(s string) tea.KeyPressMsg {
	if s == "" {
		return tea.KeyPressMsg{}
	}
	return tea.KeyPressMsg{Text: s, Code: []rune(s)[0]}
}

func khArea(val string) textarea.Model {
	ta := textarea.New()
	ta.SetWidth(40)
	ta.SetHeight(6)
	ta.SetValue(val)
	ta.Focus()
	return ta
}

func TestKH_NormalizeKeyFull(t *testing.T) {
	if normalizeKey(khPress(tea.KeyTab, 0)) != "tab" {
		t.Fatal("tab")
	}
	if normalizeKey(khPress(tea.KeyTab, tea.ModShift)) != "shift+tab" {
		t.Fatal("shift+tab")
	}
	if normalizeKey(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}) != "ctrl+c" {
		t.Fatal("ctrl+c")
	}
	if normalizeKey(tea.KeyPressMsg{Code: 'x', Mod: tea.ModAlt}) != "alt+x" {
		t.Fatal("alt")
	}
	if normalizeKey(tea.KeyPressMsg{Code: 'x', Mod: tea.ModSuper}) != "super+x" {
		t.Fatal("super")
	}
	if normalizeKey(tea.KeyPressMsg{Code: 'x', Mod: tea.ModMeta}) != "meta+x" {
		t.Fatal("meta")
	}
	if normalizeKey(khText("j")) != "j" {
		t.Fatal("text")
	}
	_ = normalizeKey(tea.KeyPressMsg{})
	shiftMap := map[rune]string{
		'a': "A", 'z': "Z", '/': "?", '3': "#", '8': "*", ';': ":",
		'\'': "\"", '1': "!", ',': "<", '.': ">", '-': "_", '=': "+",
		'`': "~", '\\': "|", '[': "{", ']': "}",
	}
	for code, want := range shiftMap {
		if got := normalizeKey(khPress(code, tea.ModShift)); got != want {
			t.Fatalf("shift+%q=%q want %q", code, got, want)
		}
	}
	if normalizeKey(khPress('9', tea.ModShift)) != "shift+9" {
		t.Fatal("shift+9")
	}
	if normalizeKey(tea.KeyPressMsg{Code: 'q'}) != "q" {
		t.Fatal("code q")
	}
}

func TestKH_TypingContextFull(t *testing.T) {
	m := NewModel()
	if m.typingContext() {
		t.Fatal("default")
	}
	m.FilterActive = true
	if !m.typingContext() {
		t.Fatal("filter active")
	}
	m.FilterActive = false
	m.FilterInput.Focus()
	if !m.typingContext() {
		t.Fatal("filter focused")
	}
	m.FilterInput.Blur()
	for _, sc := range []types.Screen{
		types.ScreenAddConnection, types.ScreenEditConnection,
		types.ScreenExport, types.ScreenCommandPalette,
	} {
		m.Screen = sc
		if !m.typingContext() {
			t.Fatalf("%v", sc)
		}
	}
	m.Screen = types.ScreenQuery
	m.Focus = focusContent
	m.QueryFocus = "editor"
	if m.typingContext() {
		t.Fatal("no area")
	}
	area := khArea("select 1")
	m.QueryArea = &area
	if !m.typingContext() {
		t.Fatal("editor")
	}
	m.QueryFocus = "results"
	if m.typingContext() {
		t.Fatal("results")
	}
	m.Focus = focusSidebar
	m.QueryFocus = "editor"
	if m.typingContext() {
		t.Fatal("sidebar")
	}
	m.Screen = types.ScreenBrowser
	if m.typingContext() {
		t.Fatal("browser")
	}
}

func TestKH_HandleKeyPressRoutes(t *testing.T) {
	m := khModel(t)
	_, cmd := m.handleKeyPress(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+c")
	}

	nm, _ := m.handleKeyPress(khText("?"))
	m = nm.(Model)
	if m.Screen != types.ScreenHelp {
		t.Fatal("help")
	}
	nm, _ = m.handleKeyPress(khText("?"))
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("help close")
	}

	m.Screen = types.ScreenAddConnection
	nm, _ = m.handleKeyPress(khText("?"))
	m = nm.(Model)
	if m.Screen != types.ScreenAddConnection {
		t.Fatal("? typing")
	}

	m.Screen = types.ScreenBrowser
	nm, _ = m.handleKeyPress(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	m = nm.(Model)
	if m.Screen != types.ScreenCommandPalette {
		t.Fatal("palette")
	}
	m.Screen = types.ScreenExport
	nm, _ = m.handleKeyPress(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	m = nm.(Model)
	if m.Screen != types.ScreenExport {
		t.Fatal("palette typing")
	}

	m = khWorkspace(t)
	m.FilterActive = true
	nm, _ = m.handleKeyPress(khPress(tea.KeyEscape, 0))
	m = nm.(Model)
	if m.FilterActive {
		t.Fatal("filter esc")
	}
	nm, _ = m.handleKeyPress(khText("/"))
	m = nm.(Model)
	if !m.FilterActive {
		t.Fatal("start search")
	}

	routes := []struct {
		screen types.Screen
		key    tea.KeyPressMsg
		prep   func(Model) Model
	}{
		{types.ScreenConnections, khText("j"), nil},
		{types.ScreenAddConnection, khPress(tea.KeyEscape, 0), nil},
		{types.ScreenEditConnection, khPress(tea.KeyEscape, 0), nil},
		{types.ScreenDatabases, khText("j"), func(m Model) Model {
			m.Databases = []types.DatabaseInfo{{Name: "demo"}}
			return m
		}},
		{types.ScreenBrowser, khText("j"), nil},
		{types.ScreenTableData, khPress(tea.KeyEscape, 0), func(m Model) Model {
			m.Focus = focusContent
			return m
		}},
		{types.ScreenTableDetail, khPress(tea.KeyEscape, 0), func(m Model) Model {
			m.Focus = focusContent
			return m
		}},
		{types.ScreenQuery, khPress(tea.KeyEscape, 0), func(m Model) Model {
			a := khArea("x")
			m.QueryArea = &a
			m.Focus = focusContent
			m.QueryFocus = "editor"
			return m
		}},
		{types.ScreenActivity, khPress(tea.KeyEscape, 0), nil},
		{types.ScreenERD, khPress(tea.KeyEscape, 0), nil},
		{types.ScreenServerInfo, khPress(tea.KeyEscape, 0), nil},
		{types.ScreenConfirmDelete, khText("n"), nil},
		{types.ScreenExport, khPress(tea.KeyEscape, 0), nil},
		{types.ScreenCommandPalette, khPress(tea.KeyEscape, 0), nil},
	}
	for _, r := range routes {
		m = khModel(t)
		m.Screen = r.screen
		m.PrevScreen = types.ScreenBrowser
		m.CurrentDatabase = "demo"
		m.Connections = []types.Connection{{ID: 1, Name: "a"}}
		if r.prep != nil {
			m = r.prep(m)
		}
		nm, _ = m.handleKeyPress(r.key)
		_ = nm
	}

	m = khModel(t)
	m.Screen = types.ScreenHelp
	m.PrevScreen = types.ScreenBrowser
	nm, _ = m.handleKeyPress(khPress(tea.KeyEscape, 0))
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatal("help esc")
	}
	m.Screen = types.ScreenTestConnection
	nm, _ = m.handleKeyPress(khPress(tea.KeyEnter, 0))
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("test enter")
	}
	m.Screen = types.ScreenTestConnection
	nm, _ = m.handleKeyPress(khText("q"))
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("test q")
	}

	m.Screen = types.ScreenLogs
	m.PrevScreen = types.ScreenBrowser
	nm, _ = m.handleKeyPress(khPress(tea.KeyEscape, 0))
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatal("logs")
	}
	m.Screen = types.ScreenFavorites
	m.PrevScreen = types.ScreenLogs
	nm, _ = m.handleKeyPress(khText("q"))
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatal("favorites nested")
	}

	m.Screen = types.Screen(99)
	_, cmd = m.handleKeyPress(khText("q"))
	if cmd == nil {
		t.Fatal("global q")
	}
}

func TestKH_KeysConnections(t *testing.T) {
	m := khModel(t)
	m.Connections = []types.Connection{
		{ID: 1, Name: "a", Host: "h", Port: 5432, Username: "u", Password: "p", Database: "d", SSLMode: types.SSLModePrefer},
		{ID: 2, Name: "b", Host: "h2", Port: 5433},
	}
	m.SelectedConnIdx = 0

	_, cmd := m.keysConnections("q")
	if cmd == nil {
		t.Fatal("q")
	}
	nm, _ := m.keysConnections("j")
	m = nm.(Model)
	nm, _ = m.keysConnections("down")
	m = nm.(Model)
	nm, _ = m.keysConnections("k")
	m = nm.(Model)
	nm, _ = m.keysConnections("up")
	m = nm.(Model)
	nm, _ = m.keysConnections("G")
	m = nm.(Model)
	if m.SelectedConnIdx != 1 {
		t.Fatal("G")
	}
	nm, _ = m.keysConnections("g")
	m = nm.(Model)
	if m.SelectedConnIdx != 0 {
		t.Fatal("g")
	}
	nm, _ = m.keysConnections("a")
	m = nm.(Model)
	if m.Screen != types.ScreenAddConnection {
		t.Fatal("a")
	}
	m.Screen = types.ScreenConnections
	nm, _ = m.keysConnections("e")
	m = nm.(Model)
	if m.Screen != types.ScreenEditConnection || m.EditingConn == nil {
		t.Fatal("e")
	}
	m.Screen = types.ScreenConnections
	nm, _ = m.keysConnections("d")
	m = nm.(Model)
	if m.Screen != types.ScreenConfirmDelete {
		t.Fatal("d")
	}
	m.Screen = types.ScreenConnections
	nm, cmd = m.keysConnections("t")
	if cmd == nil {
		t.Fatal("t")
	}
	nm, cmd = m.keysConnections("enter")
	if cmd == nil {
		t.Fatal("enter")
	}
	nm, cmd = m.keysConnections("r")
	if cmd == nil {
		t.Fatal("r")
	}

	m.Connections = nil
	for _, k := range []string{"j", "k", "G", "e", "d", "t", "enter"} {
		nm, cmd = m.keysConnections(k)
		_ = nm
		if k == "e" || k == "d" || k == "t" || k == "enter" {
			if cmd != nil {
				t.Fatalf("%s empty", k)
			}
		}
	}
	m.Connections = []types.Connection{{ID: 1}}
	m.Cmds = nil
	for _, k := range []string{"t", "enter", "r"} {
		_, cmd = m.keysConnections(k)
		if cmd != nil {
			t.Fatalf("%s no cmds", k)
		}
	}
}

func TestKH_ConnectionFormAndSave(t *testing.T) {
	m := khModel(t)
	m.Screen = types.ScreenAddConnection
	m.ConnInputs = createConnectionInputs()
	m.ConnFocusIdx = 0
	m.focusConnField()

	nm, _ := m.keysConnectionForm("esc", khPress(tea.KeyEscape, 0))
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("esc")
	}
	m.Screen = types.ScreenAddConnection
	nm, _ = m.keysConnectionForm("tab", khPress(tea.KeyTab, 0))
	m = nm.(Model)
	nm, _ = m.keysConnectionForm("down", khPress(tea.KeyDown, 0))
	m = nm.(Model)
	nm, _ = m.keysConnectionForm("shift+tab", khPress(tea.KeyTab, tea.ModShift))
	m = nm.(Model)
	nm, _ = m.keysConnectionForm("up", khPress(tea.KeyUp, 0))
	m = nm.(Model)

	m.ConnFocusIdx = connFieldSSL
	m.ConnSSLIdx = 1
	nm, _ = m.keysConnectionForm("left", khPress(tea.KeyLeft, 0))
	m = nm.(Model)
	if m.ConnSSLIdx != 0 {
		t.Fatal("ssl left")
	}
	nm, _ = m.keysConnectionForm("right", khPress(tea.KeyRight, 0))
	m = nm.(Model)
	m.ConnFocusIdx = 0
	nm, _ = m.keysConnectionForm("left", khPress(tea.KeyLeft, 0))
	m = nm.(Model)

	m.ConnFocusIdx = connFieldReadOnly
	nm, _ = m.keysConnectionForm(" ", khText(" "))
	m = nm.(Model)
	if !m.ConnReadOnly {
		t.Fatal("readonly")
	}
	m.ConnFocusIdx = 0
	nm, _ = m.keysConnectionForm(" ", khText(" "))
	m = nm.(Model)
	nm, _ = m.keysConnectionForm("x", khText("x"))
	m = nm.(Model)

	m.ConnInputs = createConnectionInputs()
	m.ConnInputs[connFieldName].SetValue("")
	m.ConnInputs[connFieldHost].SetValue("")
	nm, cmd := m.saveConnectionForm()
	m = nm.(Model)
	if m.Err == nil {
		t.Fatal("required")
	}
	m.ConnInputs[connFieldName].SetValue("n")
	m.ConnInputs[connFieldHost].SetValue("h")
	m.ConnInputs[connFieldPort].SetValue("bad")
	m.Cmds = nil
	nm, cmd = m.saveConnectionForm()
	m = nm.(Model)
	if m.Err == nil {
		t.Fatal("uninit")
	}

	m = khModel(t)
	m.Screen = types.ScreenAddConnection
	m.ConnInputs = createConnectionInputs()
	m.ConnInputs[connFieldName].SetValue("n")
	m.ConnInputs[connFieldHost].SetValue("h")
	m.ConnInputs[connFieldPort].SetValue("0")
	nm, cmd = m.saveConnectionForm()
	if cmd == nil {
		t.Fatal("add")
	}
	nm, cmd = m.keysConnectionForm("enter", khPress(tea.KeyEnter, 0))
	if cmd == nil {
		t.Fatal("enter save")
	}

	m = khModel(t)
	m.Screen = types.ScreenEditConnection
	conn := types.Connection{ID: 7, Name: "old"}
	m.EditingConn = &conn
	m.ConnInputs = createConnectionInputs()
	m.ConnInputs[connFieldName].SetValue("new")
	m.ConnInputs[connFieldHost].SetValue("h")
	m.ConnInputs[connFieldPort].SetValue("5433")
	nm, cmd = m.saveConnectionForm()
	if cmd == nil {
		t.Fatal("update")
	}
	m.ConnFocusIdx = connFieldSSL
	nm, _ = m.keysConnectionForm("x", khText("x"))
	_ = nm
}

func TestKH_KeysDatabases(t *testing.T) {
	m := khModel(t)
	m.Screen = types.ScreenDatabases
	m.Databases = []types.DatabaseInfo{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	m.SelectedDBIdx = 0
	m.CurrentDatabase = "a"

	_, cmd := m.keysDatabases("q")
	if cmd == nil {
		t.Fatal("q")
	}
	nm, _ := m.keysDatabases("j")
	m = nm.(Model)
	nm, _ = m.keysDatabases("down")
	m = nm.(Model)
	nm, _ = m.keysDatabases("k")
	m = nm.(Model)
	nm, _ = m.keysDatabases("up")
	m = nm.(Model)
	nm, _ = m.keysDatabases("G")
	m = nm.(Model)
	if m.SelectedDBIdx != 2 {
		t.Fatal("G")
	}
	nm, _ = m.keysDatabases("g")
	m = nm.(Model)
	nm, cmd = m.keysDatabases("r")
	if cmd == nil {
		t.Fatal("r")
	}

	m.Objects = []types.SchemaObject{{Name: "t"}}
	nm, cmd = m.keysDatabases("enter")
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser || cmd != nil {
		t.Fatal("same db objects")
	}
	m.Screen = types.ScreenDatabases
	m.CurrentDatabase = "a"
	m.SelectedDBIdx = 0
	m.Objects = nil
	nm, cmd = m.keysDatabases("enter")
	if cmd == nil {
		t.Fatal("same db empty")
	}
	m.Screen = types.ScreenDatabases
	m.SelectedDBIdx = 1
	nm, cmd = m.keysDatabases("enter")
	if cmd == nil {
		t.Fatal("switch")
	}

	m.Screen = types.ScreenDatabases
	m.CurrentDatabase = "a"
	m.Objects = []types.SchemaObject{{Name: "t"}}
	nm, _ = m.keysDatabases("esc")
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatal("esc back")
	}
	m.Screen = types.ScreenDatabases
	m.CurrentDatabase = "a"
	m.Objects = nil
	nm, cmd = m.keysDatabases("esc")
	if cmd == nil {
		t.Fatal("esc reload")
	}
	m.Screen = types.ScreenDatabases
	m.CurrentDatabase = ""
	nm, cmd = m.keysDatabases("esc")
	if cmd == nil {
		t.Fatal("esc disconnect")
	}
	m.Cmds = nil
	m.CurrentDatabase = ""
	nm, _ = m.keysDatabases("esc")
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("esc no cmds")
	}

	m.Databases = nil
	for _, k := range []string{"j", "k", "G"} {
		nm, _ = m.keysDatabases(k)
	}
	_, cmd = m.keysDatabases("enter")
	if cmd != nil {
		t.Fatal("enter empty")
	}
	m.Databases = []types.DatabaseInfo{{Name: "a"}}
	m.Cmds = nil
	_, cmd = m.keysDatabases("r")
	if cmd != nil {
		t.Fatal("r no cmds")
	}
	_, cmd = m.keysDatabases("enter")
	if cmd != nil {
		t.Fatal("enter no cmds")
	}
	_ = nm
}

func TestKH_ObjectSearchKeys(t *testing.T) {
	m := khWorkspace(t)
	nm, _ := m.startObjectSearch()
	m = nm.(Model)
	m.FilterInput.SetValue("order")
	m.ObjectFilter = "order"
	nm, cmd := m.keysObjectSearch("enter", khPress(tea.KeyEnter, 0))
	if cmd == nil {
		t.Fatal("enter open")
	}
	m = khWorkspace(t)
	nm, _ = m.startObjectSearch()
	m = nm.(Model)
	m.FilterInput.SetValue("zzz")
	m.ObjectFilter = "zzz"
	nm, cmd = m.keysObjectSearch("enter", khPress(tea.KeyEnter, 0))
	if cmd != nil {
		t.Fatal("enter nomatch")
	}
	nm, _ = m.startObjectSearch()
	m = nm.(Model)
	nm, _ = m.keysObjectSearch("down", khPress(tea.KeyDown, 0))
	m = nm.(Model)
	if m.FilterActive {
		t.Fatal("down")
	}
	nm, _ = m.startObjectSearch()
	m = nm.(Model)
	nm, _ = m.keysObjectSearch("tab", khPress(tea.KeyTab, 0))
	m = nm.(Model)
	if m.FilterActive {
		t.Fatal("tab")
	}
	nm, _ = m.startObjectSearch()
	m = nm.(Model)
	m.FilterInput.SetValue("ab")
	m.ObjectFilter = "ab"
	nm, _ = m.keysObjectSearch("ctrl+u", tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	m = nm.(Model)
	if m.FilterInput.Value() != "" {
		t.Fatal("ctrl+u")
	}
	nm, _ = m.keysObjectSearch("backspace", khPress(tea.KeyBackspace, 0))
	m = nm.(Model)
	if m.FilterActive {
		t.Fatal("backspace empty")
	}
	nm, _ = m.startObjectSearch()
	m = nm.(Model)
	m.FilterInput.SetValue("ab")
	nm, _ = m.keysObjectSearch("backspace", khPress(tea.KeyBackspace, 0))
	m = nm.(Model)
	if !m.FilterActive {
		t.Fatal("backspace text")
	}
}

func TestKH_KeysBrowser(t *testing.T) {
	m := khWorkspace(t)
	_, cmd := m.keysBrowser("q", khText("q"))
	if cmd == nil {
		t.Fatal("q")
	}
	m.ObjectFilter = "x"
	nm, _ := m.keysBrowser("esc", khPress(tea.KeyEscape, 0))
	m = nm.(Model)
	if m.objectSearchQuery() != "" {
		t.Fatal("esc filter")
	}
	m.ContentMode = contentSchema
	nm, _ = m.keysBrowser("esc", khPress(tea.KeyEscape, 0))
	m = nm.(Model)
	if m.ContentMode != contentPreview {
		t.Fatal("esc schema")
	}
	m.ContentMode = contentDatabases
	nm, _ = m.keysBrowser("esc", khPress(tea.KeyEscape, 0))
	m = nm.(Model)
	nm, cmd = m.keysBrowser("esc", khPress(tea.KeyEscape, 0))
	if cmd == nil {
		t.Fatal("disconnect")
	}
	m.Cmds = nil
	nm, _ = m.keysBrowser("esc", khPress(tea.KeyEscape, 0))
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("esc no cmds")
	}

	m = khWorkspace(t)
	nm, _ = m.keysBrowser("tab", khPress(tea.KeyTab, 0))
	m = nm.(Model)
	nm, _ = m.keysBrowser("shift+tab", khPress(tea.KeyTab, tea.ModShift))
	m = nm.(Model)
	nm, _ = m.keysBrowser(";", khText(";"))
	m = nm.(Model)
	if m.Screen != types.ScreenQuery {
		t.Fatal(";")
	}
	m.Screen = types.ScreenBrowser
	nm, _ = m.keysBrowser(":", khText(":"))
	m = nm.(Model)

	m = khWorkspace(t)
	nm, cmd = m.keysBrowser("A", khText("A"))
	if cmd == nil {
		t.Fatal("A")
	}
	nm, cmd = m.keysBrowser("E", khText("E"))
	if cmd == nil {
		t.Fatal("E")
	}
	nm, cmd = m.keysBrowser("i", khText("i"))
	if cmd == nil {
		t.Fatal("i")
	}
	nm, _ = m.keysBrowser("L", khText("L"))
	m = nm.(Model)
	if m.Screen != types.ScreenLogs {
		t.Fatal("L")
	}
	m.Screen = types.ScreenBrowser
	m.ContentMode = contentDatabases
	nm, cmd = m.keysBrowser("r", khText("r"))
	if cmd == nil {
		t.Fatal("r db")
	}
	m.ContentMode = contentDatabases
	m.Cmds = nil
	_, cmd = m.keysBrowser("r", khText("r"))
	if cmd != nil {
		t.Fatal("r db no cmds")
	}
	m = khWorkspace(t)
	nm, cmd = m.keysBrowser("r", khText("r"))
	if cmd == nil {
		t.Fatal("refresh")
	}
	nm, cmd = m.keysBrowser("D", khText("D"))
	if cmd == nil {
		t.Fatal("D")
	}
	for _, k := range []string{"1", "2", "3", "4", "5", "6"} {
		nm, _ = m.keysBrowser(k, khText(k))
		m = nm.(Model)
	}
	m.ContentMode = contentDatabases
	m.Focus = focusContent
	m.Databases = []types.DatabaseInfo{{Name: "demo"}, {Name: "other"}}
	nm, _ = m.keysBrowser("j", khText("j"))
	m = nm.(Model)
	if m.SelectedDBIdx != 1 {
		t.Fatal("db content")
	}
	m.Cmds = nil
	m.Focus = focusSidebar
	m.ContentMode = contentPreview
	_, cmd = m.keysBrowser("A", khText("A"))
	if cmd != nil {
		t.Fatal("A no cmds")
	}
	_, cmd = m.keysBrowser("i", khText("i"))
	if cmd != nil {
		t.Fatal("i no cmds")
	}
	_ = nm
}

func TestKH_DatabasesInContent(t *testing.T) {
	m := khWorkspace(t)
	m.ContentMode = contentDatabases
	m.Focus = focusContent
	m.Databases = []types.DatabaseInfo{{Name: "demo"}, {Name: "other"}, {Name: "z"}}
	m.CurrentDatabase = "demo"
	m.SelectedDBIdx = 0

	nm, _ := m.keysDatabasesInContent("j")
	m = nm.(Model)
	nm, _ = m.keysDatabasesInContent("down")
	m = nm.(Model)
	nm, _ = m.keysDatabasesInContent("k")
	m = nm.(Model)
	nm, _ = m.keysDatabasesInContent("up")
	m = nm.(Model)
	nm, _ = m.keysDatabasesInContent("G")
	m = nm.(Model)
	nm, _ = m.keysDatabasesInContent("g")
	m = nm.(Model)
	nm, cmd := m.keysDatabasesInContent("r")
	if cmd == nil {
		t.Fatal("r")
	}
	m.SelectedDBIdx = 0
	nm, _ = m.keysDatabasesInContent("enter")
	m = nm.(Model)
	if m.ContentMode != contentPreview {
		t.Fatal("same db")
	}
	m.ContentMode = contentDatabases
	m.Focus = focusContent
	m.SelectedDBIdx = 1
	nm, cmd = m.keysDatabasesInContent("enter")
	if cmd == nil {
		t.Fatal("switch")
	}
	nm, _ = m.keysDatabasesInContent("h")
	m = nm.(Model)
	nm, _ = m.keysDatabasesInContent("left")
	m = nm.(Model)

	m.Databases = nil
	for _, k := range []string{"j", "k", "G"} {
		nm, _ = m.keysDatabasesInContent(k)
	}
	_, cmd = m.keysDatabasesInContent("enter")
	if cmd != nil {
		t.Fatal("empty enter")
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
	_ = nm
}

func TestKH_SidebarActivateToggle(t *testing.T) {
	m := NewModel()
	nm, _ := m.keysSidebar("j")
	_ = nm

	m = khWorkspace(t)
	for _, k := range []string{"j", "down", "k", "up", "g", "G"} {
		nm, _ = m.keysSidebar(k)
		m = nm.(Model)
	}
	m = m.pinSidebarCursorToKind(navViews)
	nm, cmd := m.keysSidebar(" ")
	if cmd == nil {
		t.Fatal("space kind")
	}
	m = khWorkspace(t)
	m.SelectedObjIdx = 0
	m = m.syncSidebarCursorToObject()
	for _, k := range []string{"enter", "l", "right", "D"} {
		mm := khWorkspace(t)
		mm.SelectedObjIdx = 0
		mm = mm.syncSidebarCursorToObject()
		nm, cmd = mm.keysSidebar(k)
		if cmd == nil {
			t.Fatalf("%s object", k)
		}
	}
	m = m.pinSidebarCursorToKind(navTables)
	nm, cmd = m.keysSidebar("D")
	if cmd != nil {
		t.Fatal("D kind")
	}

	for _, nav := range []NavSection{navQuery, navActivity, navERD, navServer, navDatabases} {
		mm := khWorkspace(t)
		nm, cmd = mm.activateSidebarRow(sidebarRow{kind: sbTool, nav: nav})
		_ = nm
		_ = cmd
	}
	m = khWorkspace(t)
	m.Cmds = nil
	_, cmd = m.activateSidebarRow(sidebarRow{kind: sbTool, nav: navActivity})
	if cmd != nil {
		t.Fatal("activity no cmds")
	}
	_, cmd = m.activateSidebarRow(sidebarRow{kind: sbTool, nav: navServer})
	if cmd != nil {
		t.Fatal("server no cmds")
	}
	m = khWorkspace(t)
	nm, cmd = m.activateSidebarRow(sidebarRow{kind: sbNav, nav: navViews})
	if cmd == nil {
		t.Fatal("sbNav")
	}
	nm, cmd = m.activateSidebarRow(sidebarRow{kind: sbKind, nav: navSequences})
	if cmd == nil {
		t.Fatal("sbKind")
	}
	_, cmd = m.activateSidebarRow(sidebarRow{kind: sbSchema, schema: 99})
	if cmd != nil {
		t.Fatal("schema oob")
	}
	nm, cmd = m.activateSidebarRow(sidebarRow{kind: sbObject, objIdx: 0})
	if cmd == nil {
		t.Fatal("object")
	}
	_, cmd = m.activateSidebarRow(sidebarRow{kind: sbTool, nav: NavSection(99)})
	if cmd != nil {
		t.Fatal("unknown tool")
	}

	m = khModel(t)
	m.KindEnabled = nil
	nm, _ = m.toggleKindFilter(navTables)
	m = nm.(Model)
	m.KindEnabled = map[NavSection]bool{navTables: true}
	nm, _ = m.toggleKindFilter(navTables)
	m = nm.(Model)
	if !m.KindEnabled[navTables] || m.StatusMsg == "" {
		t.Fatal("last filter")
	}
	nm, cmd = m.toggleKindFilter(navViews)
	if cmd == nil {
		t.Fatal("enable views")
	}
	m.ContentMode = contentSchema
	nm, _ = m.toggleKindFilter(navViews)
	m = nm.(Model)
	if m.ContentMode != contentSchema {
		t.Fatal("keep schema")
	}

	m.KindEnabled = map[NavSection]bool{navExtensions: true}
	m.CurrentSchema = "public"
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

func TestKH_OpenBeginHelpers(t *testing.T) {
	m := khWorkspace(t)
	m.Objects = nil
	m.SelectedObjIdx = 0
	_, cmd := m.openSelectedObject()
	if cmd != nil {
		t.Fatal("empty sel")
	}
	m = khWorkspace(t)
	m.SelectedObjIdx = 2
	m = m.syncSidebarCursorToObject()
	nm, cmd := m.openSelectedObject()
	if cmd == nil {
		t.Fatal("seq")
	}
	_ = nm

	m = khWorkspace(t)
	o := types.SchemaObject{Schema: "public", Name: "fn", Kind: types.ObjectFunction}
	m = m.showObjectPreview(o)
	if m.ContentMode != contentPreview {
		t.Fatal("preview")
	}
	m = m.showObjectPreview(types.SchemaObject{Name: "x"})

	_, cmd = m.beginObjectDetail(types.SchemaObject{})
	if cmd != nil {
		t.Fatal("empty name")
	}
	m.Cmds = nil
	_, cmd = m.beginObjectDetail(types.SchemaObject{Name: "x", Schema: "public", Kind: types.ObjectFunction})
	if cmd != nil {
		t.Fatal("no cmds")
	}
	m = khWorkspace(t)
	nm, cmd = m.beginObjectDetail(types.SchemaObject{Name: "fn", Schema: "public", Kind: types.ObjectFunction})
	if cmd == nil {
		t.Fatal("detail")
	}
	_ = cmd()

	m = khWorkspace(t)
	_, cmd = m.beginTableDetail(types.SchemaObject{})
	if cmd != nil {
		t.Fatal("td empty")
	}
	m.Cmds = nil
	_, cmd = m.beginTableDetail(types.SchemaObject{Name: "x"})
	if cmd != nil {
		t.Fatal("td no cmds")
	}
	m = khWorkspace(t)
	_, cmd = m.beginTableDetail(types.SchemaObject{Name: "s", Schema: "public", Kind: types.ObjectSequence})
	if cmd == nil {
		t.Fatal("td seq")
	}
	_, cmd = m.beginTableDetail(types.SchemaObject{Name: "orders", Schema: "public", Kind: types.ObjectTable})
	if cmd == nil {
		t.Fatal("td table")
	}
	_ = cmd()

	m = khWorkspace(t)
	_, cmd = m.beginTableData(types.SchemaObject{}, 0, 10)
	if cmd != nil {
		t.Fatal("data empty")
	}
	m.Cmds = nil
	_, cmd = m.beginTableData(types.SchemaObject{Name: "x"}, 0, 10)
	if cmd != nil {
		t.Fatal("data no cmds")
	}
	m = khWorkspace(t)
	nm2, cmd := m.beginTableData(types.SchemaObject{Name: "fn", Schema: "public", Kind: types.ObjectFunction}, 0, 10)
	m = nm2
	if cmd != nil || m.ContentMode != contentPreview {
		t.Fatal("non-rel preview")
	}
	_, cmd = m.beginTableData(types.SchemaObject{Name: "orders", Schema: "public", Kind: types.ObjectTable}, 0, 10)
	if cmd == nil {
		t.Fatal("data table")
	}
	_ = cmd()

	m.Cmds = nil
	nm, cmd = m.openDatabasesContent()
	m = nm.(Model)
	if cmd != nil || m.ContentMode != contentDatabases {
		t.Fatal("db no cmds")
	}
	m.Cmds = nil
	m.CurrentSchema = ""
	nm, cmd = m.openERD()
	m = nm.(Model)
	if m.Screen != types.ScreenERD || cmd != nil {
		t.Fatal("erd no cmds")
	}
	m = khWorkspace(t)
	nm, cmd = m.openERD()
	if cmd == nil {
		t.Fatal("erd cmds")
	}
	m.Cmds = nil
	_, cmd = m.refreshBrowser()
	if cmd != nil {
		t.Fatal("refresh no cmds")
	}
	m = khWorkspace(t)
	_, cmd = m.refreshBrowser()
	if cmd == nil {
		t.Fatal("refresh")
	}
	_ = nm
}

func TestKH_AfterCursorAndObjectList(t *testing.T) {
	m := NewModel()
	nm, _ := m.onSidebarCursorMoved()
	_ = nm
	m = m.syncSelectionFromSidebar()

	m = khWorkspace(t)
	m.SelectedObjIdx = 99
	nm, _ = m.afterObjectCursorMove()
	_ = nm

	m = khWorkspace(t)
	m.Screen = types.ScreenBrowser
	m.SelectedObjIdx = 0
	nm, _ = m.afterObjectCursorMove()
	m = nm.(Model)

	m.Screen = types.ScreenTableDetail
	nm, cmd := m.afterObjectCursorMove()
	m = nm.(Model)
	if cmd != nil || m.Screen != types.ScreenBrowser {
		t.Fatalf("detail cursor should preview, screen=%v cmd=%v", m.Screen, cmd != nil)
	}
	m.Screen = types.ScreenTableData
	nm, cmd = m.afterObjectCursorMove()
	m = nm.(Model)
	if cmd != nil || m.Screen != types.ScreenBrowser {
		t.Fatalf("data cursor should preview, screen=%v cmd=%v", m.Screen, cmd != nil)
	}
	m.Objects = []types.SchemaObject{{Schema: "public", Name: "fn", Kind: types.ObjectFunction}}
	m.SelectedObjIdx = 0
	m.Screen = types.ScreenTableData
	nm, cmd = m.afterObjectCursorMove()
	m = nm.(Model)
	if cmd != nil || m.Screen != types.ScreenBrowser {
		t.Fatalf("data nonrel should preview, screen=%v cmd=%v", m.Screen, cmd != nil)
	}
	m.Screen = types.ScreenConnections
	m.Objects = []types.SchemaObject{{Schema: "public", Name: "orders", Kind: types.ObjectTable}}
	m.SelectedObjIdx = 0
	_, cmd = m.afterObjectCursorMove()
	if cmd != nil {
		t.Fatal("default")
	}

	m = khWorkspace(t)
	m.SelectedObjIdx = 0
	nm, _ = m.keysObjectList("j")
	m = nm.(Model)
	nm, _ = m.keysObjectList("down")
	m = nm.(Model)
	m.SelectedObjIdx = len(m.filteredObjects()) - 1
	nm, _ = m.keysObjectList("j")
	m = nm.(Model)
	nm, _ = m.keysObjectList("k")
	m = nm.(Model)
	nm, _ = m.keysObjectList("up")
	m = nm.(Model)
	m.SelectedObjIdx = 0
	nm, _ = m.keysObjectList("k")
	m = nm.(Model)
	nm, _ = m.keysObjectList("g")
	m = nm.(Model)
	nm, _ = m.keysObjectList("G")
	m = nm.(Model)
	nm, _ = m.keysObjectList("G")
	m = nm.(Model)
	nm, _ = m.keysObjectList("h")
	m = nm.(Model)
	nm, _ = m.keysObjectList("left")
	m = nm.(Model)
	m.SelectedObjIdx = 0
	nm, cmd = m.keysObjectList("enter")
	if cmd == nil {
		t.Fatal("enter")
	}
	m.SelectedObjIdx = 2
	nm, cmd = m.keysObjectList("l")
	if cmd == nil {
		t.Fatal("l")
	}
	m.SelectedObjIdx = 0
	nm, cmd = m.keysObjectList("right")
	if cmd == nil {
		t.Fatal("right")
	}
	nm, cmd = m.keysObjectList("D")
	if cmd == nil {
		t.Fatal("D")
	}
	m.Objects = nil
	for _, k := range []string{"j", "k", "G", "enter", "D"} {
		nm, cmd = m.keysObjectList(k)
		_ = nm
		if (k == "enter" || k == "D") && cmd != nil {
			t.Fatalf("%s empty", k)
		}
	}
	m.Objects = []types.SchemaObject{{Name: "a", Schema: "public", Kind: types.ObjectTable}}
	m.SelectedObjIdx = 0
	nm, _ = m.keysObjectList("g")
	_ = nm

	m = NewModel()
	if m.copySelectedObject().Name != "" {
		t.Fatal("copy empty")
	}
}

func TestKH_OpenQuery(t *testing.T) {
	m := khWorkspace(t)
	m.CurrentObject = &m.Objects[0]
	nm, cmd := m.openQuery()
	m = nm.(Model)
	if m.QueryArea == nil || cmd == nil {
		t.Fatal("prefill run")
	}
	area := khArea("")
	m.QueryArea = &area
	m.CurrentObject = &m.Objects[0]
	nm, cmd = m.openQuery()
	m = nm.(Model)
	if m.QueryArea.Value() == "" {
		t.Fatal("prefill empty")
	}
	area2 := khArea("select 1")
	m.QueryArea = &area2
	nm, _ = m.openQuery()
	m = nm.(Model)
	if m.QueryArea.Value() != "select 1" {
		t.Fatal("keep")
	}
	m.Cmds = nil
	m.QueryArea = nil
	m.CurrentObject = &m.Objects[0]
	nm, cmd = m.openQuery()
	if cmd != nil {
		t.Fatal("no cmds")
	}
	seq := m.Objects[2]
	m.CurrentObject = &seq
	m.QueryArea = nil
	nm, _ = m.openQuery()
	m = nm.(Model)
	if m.QueryArea == nil {
		t.Fatal("area")
	}
	_ = nm
}

func TestKH_TableDataKeys(t *testing.T) {
	m := khWorkspace(t)
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	obj := m.Objects[0]
	m.CurrentObject = &obj
	m.TableData = types.QueryResult{
		Columns: []string{"id", "name", "x"}, Rows: [][]string{{"1", "a", "x"}, {"2", "b", "y"}}, Truncated: true,
	}
	m.DataCursor, m.DataCol, m.DataOffset, m.PageSize = 0, 1, 0, 2

	nm, _ := m.keysTableData("tab")
	m = nm.(Model)
	nm, _ = m.keysTableData("shift+tab")
	m = nm.(Model)
	nm, _ = m.keysTableData("esc")
	m = nm.(Model)
	m.Screen = types.ScreenTableData
	m.Focus = focusSidebar
	nm, _ = m.keysTableData("j")
	m = nm.(Model)
	// Sidebar j/k leaves data view for preview; restore grid for content keys.
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m.CurrentObject = &obj
	m.TableData = types.QueryResult{
		Columns: []string{"id", "name", "x"}, Rows: [][]string{{"1", "a", "x"}, {"2", "b", "y"}}, Truncated: true,
	}
	m.DataCursor, m.DataCol, m.DataOffset, m.PageSize = 0, 1, 0, 2
	nm, _ = m.keysTableData("0")
	m = nm.(Model)
	nm, _ = m.keysTableData("$")
	m = nm.(Model)
	nm, cmd := m.keysTableData("n")
	if cmd == nil {
		t.Fatal("n")
	}
	m.DataOffset = 2
	nm, cmd = m.keysTableData("p")
	if cmd == nil {
		t.Fatal("p")
	}
	nm, cmd = m.keysTableData("D")
	if cmd == nil {
		t.Fatal("D")
	}
	m.Objects = nil
	m.CurrentObject = &obj
	nm, cmd = m.keysTableData("D")
	if cmd == nil {
		t.Fatal("D current")
	}
	m.Objects = khWorkspace(t).Objects
	nm, _ = m.keysTableData("x")
	m = nm.(Model)
	if m.Screen != types.ScreenExport {
		t.Fatal("x")
	}
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	nm, cmd = m.keysTableData("y")
	if cmd == nil {
		t.Fatal("y")
	}
	nm, cmd = m.keysTableData("Y")
	if cmd == nil {
		t.Fatal("Y")
	}
	nm, cmd = m.keysTableData("r")
	if cmd == nil {
		t.Fatal("r")
	}
	nm, cmd = m.keysTableData("E")
	if cmd == nil {
		t.Fatal("E")
	}
	nm, _ = m.keysTableData(";")
	m = nm.(Model)
	m.Screen = types.ScreenTableData
	nm, _ = m.keysTableData(":")
	m = nm.(Model)

	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m.TableData = types.QueryResult{}
	nm, _ = m.keysTableData("$")
	nm, _ = m.keysTableData("l")
	_, cmd = m.keysTableData("y")
	if cmd != nil {
		t.Fatal("y empty")
	}
	_, cmd = m.keysTableData("Y")
	if cmd != nil {
		t.Fatal("Y empty")
	}
	m.CurrentObject = nil
	for _, k := range []string{"n", "p", "r"} {
		_, cmd = m.keysTableData(k)
		if cmd != nil {
			t.Fatalf("%s no obj", k)
		}
	}
	m.Inputs = nil
	m.TableData = types.QueryResult{Rows: [][]string{{"1"}}}
	nm, _ = m.keysTableData("x")
	_ = nm
}

func TestKH_TableDetailKeys(t *testing.T) {
	m := khWorkspace(t)
	m.Screen = types.ScreenTableDetail
	m.Focus = focusContent
	obj := m.Objects[0]
	m.CurrentObject = &obj
	m.TableDetail = types.TableDetail{
		Object: obj, Columns: []types.ColumnInfo{{Name: "id"}},
		Indexes: []types.IndexInfo{{Name: "pk"}}, Constraints: []types.ConstraintInfo{{Name: "c"}},
	}
	for _, k := range []string{"tab", "shift+tab", "esc"} {
		nm, _ := m.keysTableDetail(k)
		m = nm.(Model)
	}
	m.Screen = types.ScreenTableDetail
	m.Focus = focusSidebar
	nm, _ := m.keysTableDetail("j")
	m = nm.(Model)
	m.Focus = focusContent
	for _, k := range []string{"l", "right", "h", "left", "1", "2", "3", "4"} {
		nm, _ = m.keysTableDetail(k)
		m = nm.(Model)
	}
	nm, cmd := m.keysTableDetail("enter")
	if cmd == nil {
		t.Fatal("enter")
	}
	seq := m.Objects[2]
	m.CurrentObject = &seq
	m.Objects = nil
	_, cmd = m.keysTableDetail("enter")
	if cmd != nil {
		t.Fatal("nonrel")
	}
	m.CurrentObject = &obj
	m.Objects = khWorkspace(t).Objects
	m.SelectedObjIdx = 0
	_, cmd = m.keysTableDetail("D")
	if cmd == nil {
		t.Fatal("D")
	}
	m.Objects = nil
	m.CurrentObject = &obj
	_, cmd = m.keysTableDetail("D")
	if cmd == nil {
		t.Fatal("D cur")
	}
	_, cmd = m.keysTableDetail("E")
	if cmd == nil {
		t.Fatal("E")
	}
	nm, _ = m.keysTableDetail(";")
	m = nm.(Model)
	m.Screen = types.ScreenTableDetail
	m.Focus = focusContent
	nm, _ = m.keysTableDetail(":")
	m = nm.(Model)
	m.Screen = types.ScreenTableDetail
	m.Focus = focusContent
	m.Objects = nil
	m.CurrentObject = nil
	_, cmd = m.keysTableDetail("enter")
	if cmd != nil {
		t.Fatal("empty enter")
	}
	_, cmd = m.keysTableDetail("D")
	if cmd != nil {
		t.Fatal("empty D")
	}
	_ = nm
}

func TestKH_QueryKeys(t *testing.T) {
	m := khWorkspace(t)
	m.Screen = types.ScreenQuery
	m.Focus = focusContent
	m.QueryFocus = "editor"
	area := khArea("SELECT 1")
	m.QueryArea = &area

	nm, _ := m.keysQuery("esc", khPress(tea.KeyEscape, 0))
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatal("esc")
	}

	m.Screen = types.ScreenQuery
	m.Focus = focusContent
	m.QueryFocus = "editor"
	area = khArea("SELECT * FROM ")
	m.QueryArea = &area
	m.QuerySuggests = []string{"orders", "users"}
	m.QuerySuggestIdx = 0
	nm, _ = m.keysQuery("tab", khPress(tea.KeyTab, 0))
	m = nm.(Model)

	m.QuerySuggests = nil
	m.QueryFocus = "editor"
	nm, _ = m.keysQuery("tab", khPress(tea.KeyTab, 0))
	m = nm.(Model)
	if m.QueryFocus != "results" {
		t.Fatal("to results")
	}
	nm, _ = m.keysQuery("tab", khPress(tea.KeyTab, 0))
	m = nm.(Model)
	if m.QueryFocus != "editor" {
		t.Fatal("to editor")
	}
	m.Focus = focusSidebar
	nm, _ = m.keysQuery("tab", khPress(tea.KeyTab, 0))
	m = nm.(Model)
	if m.Focus != focusContent {
		t.Fatal("from sidebar")
	}

	m.Focus = focusContent
	m.QueryFocus = "results"
	nm, _ = m.keysQuery("shift+tab", khPress(tea.KeyTab, tea.ModShift))
	m = nm.(Model)
	nm, _ = m.keysQuery("shift+tab", khPress(tea.KeyTab, tea.ModShift))
	m = nm.(Model)
	nm, _ = m.keysQuery("shift+tab", khPress(tea.KeyTab, tea.ModShift))
	m = nm.(Model)
	nm, _ = m.keysQuery("shift+tab", khPress(tea.KeyTab, tea.ModShift))
	m = nm.(Model)

	m.Focus = focusContent
	m.QueryFocus = "editor"
	for _, msg := range []tea.KeyPressMsg{
		{Code: tea.KeyEnter, Mod: tea.ModCtrl},
		{Code: 'j', Mod: tea.ModCtrl},
		{Code: 'r', Mod: tea.ModCtrl},
		{Code: tea.KeyF5},
		{Code: 'm', Mod: tea.ModCtrl},
		{Code: tea.KeyEnter, Mod: tea.ModAlt},
		{Code: 'e', Mod: tea.ModCtrl},
	} {
		nm, cmd := m.keysQuery(normalizeKey(msg), msg)
		m = nm.(Model)
		if cmd == nil {
			t.Fatalf("run %q", normalizeKey(msg))
		}
	}
	// String() path for ctrl+enter without normalize recognizing
	msg := tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl}
	nm, cmd := m.keysQuery("unknown", msg)
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("string run")
	}

	m.QueryArea = nil
	_, cmd = m.keysQuery("ctrl+enter", tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	if cmd != nil {
		t.Fatal("no area")
	}
	area = khArea("x")
	m.QueryArea = &area
	m.Cmds = nil
	_, cmd = m.keysQuery("ctrl+enter", tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	if cmd != nil {
		t.Fatal("no cmds")
	}

	m = khWorkspace(t)
	m.Screen = types.ScreenQuery
	m.Focus = focusContent
	m.QueryFocus = "results"
	m.QueryResult = types.QueryResult{Columns: []string{"id", "n"}, Rows: [][]string{{"1", "a"}, {"2", "b"}}}
	area = khArea("x")
	m.QueryArea = &area
	nm, _ = m.keysQuery("x", khText("x"))
	m = nm.(Model)
	if m.Screen != types.ScreenExport {
		t.Fatal("export")
	}
	m.Screen = types.ScreenQuery
	m.QueryResult.Rows = nil
	nm, _ = m.keysQuery("x", khText("x"))
	m = nm.(Model)

	m.Focus = focusSidebar
	nm, _ = m.keysQuery("j", khText("j"))
	m = nm.(Model)

	m.Focus = focusContent
	m.QueryFocus = "results"
	m.QueryResult = types.QueryResult{Columns: []string{"id", "n"}, Rows: [][]string{{"1", "a"}, {"2", "b"}}}
	for _, k := range []string{"j", "down", "k", "up", "h", "left", "l", "right"} {
		nm, _ = m.keysQuery(k, khText(k))
		m = nm.(Model)
	}
	_, cmd = m.keysQuery("y", khText("y"))
	if cmd == nil {
		t.Fatal("y")
	}
	_, cmd = m.keysQuery("Y", khText("Y"))
	if cmd == nil {
		t.Fatal("Y")
	}
	m.QueryResult.Rows = nil
	_, cmd = m.keysQuery("y", khText("y"))
	if cmd != nil {
		t.Fatal("y empty")
	}
	m.QueryResult = types.QueryResult{Rows: [][]string{{"1"}}}
	nm, _ = m.keysQuery("l", khText("l"))
	m = nm.(Model)

	m.QueryFocus = "editor"
	m.QuerySuggests = []string{"a", "b", "c"}
	m.QuerySuggestIdx = 0
	nm, _ = m.keysQuery("down", khPress(tea.KeyDown, 0))
	m = nm.(Model)
	nm, _ = m.keysQuery("ctrl+n", tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	m = nm.(Model)
	nm, _ = m.keysQuery("up", khPress(tea.KeyUp, 0))
	m = nm.(Model)
	nm, _ = m.keysQuery("ctrl+p", tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	m = nm.(Model)
	nm, _ = m.keysQuery("enter", khPress(tea.KeyEnter, 0))
	m = nm.(Model)
	nm, _ = m.keysQuery("ctrl+@", tea.KeyPressMsg{Code: '@', Mod: tea.ModCtrl})
	m = nm.(Model)
	nm, _ = m.keysQuery("ctrl+space", tea.KeyPressMsg{Code: ' ', Mod: tea.ModCtrl})
	m = nm.(Model)
	nm, _ = m.keysQuery("alt+/", tea.KeyPressMsg{Code: '/', Mod: tea.ModAlt})
	m = nm.(Model)
	nm, _ = m.keysQuery("z", khText("z"))
	m = nm.(Model)
	m.QueryFocus = "other"
	nm, _ = m.keysQuery("j", khText("j"))
	m = nm.(Model)
	m.QueryFocus = "editor"
	m.QueryArea = nil
	nm, _ = m.keysQuery("a", khText("a"))
	_ = nm
}

func TestKH_ActivityServerConfirmExportPaletteERD(t *testing.T) {
	m := khWorkspace(t)
	m.Screen = types.ScreenActivity
	m.Focus = focusContent
	m.Activity = []types.ActivityRow{{PID: 1}, {PID: 2}, {PID: 3}}
	for _, k := range []string{"tab", "shift+tab", "esc"} {
		nm, _ := m.keysActivity(k)
		m = nm.(Model)
	}
	m.Screen = types.ScreenActivity
	m.Focus = focusSidebar
	nm, _ := m.keysActivity("j")
	m = nm.(Model)
	m.Focus = focusContent
	for _, k := range []string{"j", "down", "k", "up"} {
		nm, _ = m.keysActivity(k)
		m = nm.(Model)
	}
	nm, cmd := m.keysActivity("r")
	if cmd == nil {
		t.Fatal("act r")
	}
	m.Activity = nil
	nm, _ = m.keysActivity("j")
	m = nm.(Model)
	m.Cmds = nil
	_, cmd = m.keysActivity("r")
	if cmd != nil {
		t.Fatal("act r no cmds")
	}

	m = khWorkspace(t)
	m.Screen = types.ScreenServerInfo
	m.Focus = focusContent
	for _, k := range []string{"tab", "shift+tab", "esc"} {
		nm, _ = m.keysServerInfo(k)
		m = nm.(Model)
	}
	m.Screen = types.ScreenServerInfo
	m.Focus = focusMain
	nm, _ = m.keysServerInfo("j")
	m = nm.(Model)
	m.Focus = focusContent
	nm, cmd = m.keysServerInfo("r")
	if cmd == nil {
		t.Fatal("srv r")
	}
	m.Cmds = nil
	_, cmd = m.keysServerInfo("r")
	if cmd != nil {
		t.Fatal("srv r no cmds")
	}

	m = khModel(t)
	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "connection"
	m.ConfirmData = types.Connection{ID: 5}
	for _, k := range []string{"n", "esc"} {
		nm, _ = m.keysConfirm(k)
		m = nm.(Model)
		m.Screen = types.ScreenConfirmDelete
	}
	m.ConfirmType = "connection"
	m.ConfirmData = types.Connection{ID: 5}
	nm, cmd = m.keysConfirm("y")
	if cmd == nil {
		t.Fatal("y")
	}
	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "connection"
	m.ConfirmData = types.Connection{ID: 5}
	nm, cmd = m.keysConfirm("enter")
	if cmd == nil {
		t.Fatal("enter")
	}
	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "other"
	nm, _ = m.keysConfirm("y")
	m = nm.(Model)
	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "connection"
	m.ConfirmData = "bad"
	nm, _ = m.keysConfirm("y")
	m = nm.(Model)
	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "connection"
	m.ConfirmData = types.Connection{ID: 1}
	m.Cmds = nil
	_, cmd = m.keysConfirm("y")
	if cmd != nil {
		t.Fatal("no cmds")
	}

	m = khModel(t)
	m.Screen = types.ScreenExport
	m.PrevScreen = types.ScreenTableData
	m.TableData = types.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}}}
	nm, _ = m.keysExport("esc", khPress(tea.KeyEscape, 0))
	m = nm.(Model)
	m.Screen = types.ScreenExport
	m.PrevScreen = 0
	nm, _ = m.keysExport("esc", khPress(tea.KeyEscape, 0))
	m = nm.(Model)
	m.Screen = types.ScreenExport
	m.PrevScreen = types.ScreenQuery
	m.QueryResult = types.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}}}
	m.Inputs.ExportInput.SetValue("/tmp/out.csv")
	nm, cmd = m.keysExport("enter", khPress(tea.KeyEnter, 0))
	if cmd == nil {
		t.Fatal("export")
	}
	nm, _ = m.keysExport("a", khText("a"))
	m = nm.(Model)
	m.Cmds = nil
	_, cmd = m.keysExport("enter", khPress(tea.KeyEnter, 0))
	if cmd != nil {
		t.Fatal("export no cmds")
	}
	m = khModel(t)
	m.Screen = types.ScreenExport
	m.Inputs = nil
	_, cmd = m.keysExport("enter", khPress(tea.KeyEnter, 0))
	if cmd != nil {
		t.Fatal("export no inputs")
	}
	nm, _ = m.keysExport("z", khText("z"))
	_ = nm

	m = khWorkspace(t)
	m.Screen = types.ScreenCommandPalette
	m.PrevScreen = types.ScreenBrowser
	m.PaletteItems = defaultPaletteItems()
	nm, _ = m.keysPalette("esc", khPress(tea.KeyEscape, 0))
	m = nm.(Model)
	m.Screen = types.ScreenCommandPalette
	for _, k := range []string{"down", "up"} {
		nm, _ = m.keysPalette(k, khText(k))
		m = nm.(Model)
	}
	nm, _ = m.keysPalette("ctrl+n", tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl})
	m = nm.(Model)
	nm, _ = m.keysPalette("ctrl+p", tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	m = nm.(Model)
	nm, _ = m.keysPalette("q", khText("q"))
	m = nm.(Model)
	m.Inputs.PaletteInput.SetValue("")
	m.PaletteIdx = 0
	nm, cmd = m.keysPalette("enter", khPress(tea.KeyEnter, 0))
	_ = nm
	_ = cmd
	m.Screen = types.ScreenCommandPalette
	m.Inputs.PaletteInput.SetValue("zzzz-no-match")
	_, cmd = m.keysPalette("enter", khPress(tea.KeyEnter, 0))
	if cmd != nil {
		t.Fatal("empty filter enter")
	}
	m.PaletteItems = nil
	nm, _ = m.keysPalette("down", khPress(tea.KeyDown, 0))
	m = nm.(Model)
	m.Inputs = nil
	m.PaletteItems = defaultPaletteItems()
	nm, _ = m.keysPalette("x", khText("x"))
	_ = nm

	for _, id := range []string{"query", "activity", "erd", "server", "tables", "views", "databases", "disconnect", "help", "logs", "unknown"} {
		mm := khWorkspace(t)
		nm, cmd = mm.runPalette(id)
		_ = nm
		_ = cmd
	}
	m = khWorkspace(t)
	m.CurrentConn = nil
	nm, _ = m.runPalette("query")
	m = nm.(Model)
	if m.StatusMsg != "Connect first" {
		t.Fatal("connect first")
	}
	m = khWorkspace(t)
	m.Cmds = nil
	for _, id := range []string{"activity", "server", "disconnect"} {
		_, cmd = m.runPalette(id)
		if cmd != nil {
			t.Fatalf("%s no cmds", id)
		}
	}

	m = khWorkspace(t)
	m.Screen = types.ScreenERD
	m.Focus = focusContent
	for _, k := range []string{"tab", "shift+tab", "esc"} {
		nm, _ = m.keysERD(k)
		m = nm.(Model)
	}
	m.Screen = types.ScreenERD
	m.Focus = focusSidebar
	nm, _ = m.keysERD("j")
	m = nm.(Model)
	m.Focus = focusMain
	nm, _ = m.keysERD("k")
	m = nm.(Model)
	m.Focus = focusContent
	for _, k := range []string{"j", "down", "k", "up", "g"} {
		nm, _ = m.keysERD(k)
		m = nm.(Model)
	}
	nm, cmd = m.keysERD("r")
	if cmd == nil {
		t.Fatal("erd r")
	}
	_ = nm
}

func TestKH_ApplyDetailMismatchAndUpdateEdges(t *testing.T) {
	m := khModel(t)
	cur := types.SchemaObject{Schema: "public", Name: "a", Kind: types.ObjectTable}
	m.CurrentObject = &cur
	nm, _ := m.applyTableDetail(types.TableDetailLoadedMsg{
		Detail: types.TableDetail{Object: types.SchemaObject{Schema: "public", Name: "b"}},
	})
	m = nm.(Model)
	if m.Screen == types.ScreenTableDetail {
		t.Fatal("mismatch")
	}
	nm, _ = m.applyTableDetail(types.TableDetailLoadedMsg{
		Detail: types.TableDetail{Object: types.SchemaObject{Name: "a"}, Props: []types.DetailProp{{Label: "x", Value: "y"}}},
	})
	m = nm.(Model)
	m.CurrentObject = nil
	nm, _ = m.applyTableDetail(types.TableDetailLoadedMsg{Detail: types.TableDetail{}})
	m = nm.(Model)
	if m.StatusMsg == "" {
		t.Fatal("empty status")
	}
	nm, _ = m.applyTableDetail(types.TableDetailLoadedMsg{Err: errors.New("e")})
	m = nm.(Model)
	if m.Err == nil {
		t.Fatal("err")
	}
	nm, _ = m.applyTableData(types.TableDataLoadedMsg{Err: errors.New("e")})
	m = nm.(Model)
	if m.Err == nil {
		t.Fatal("data err")
	}

	// Connected with empty info db but CurrentConn.Database fills it
	m = khModel(t)
	m.CurrentConn = &types.Connection{Database: "fromconn", ReadOnly: true}
	nm, cmd := m.Update(types.ConnectedMsg{Info: types.ServerInfo{Version: "v"}})
	m = nm.(Model)
	if m.CurrentDatabase != "fromconn" || cmd == nil {
		t.Fatalf("db fill %q cmd=%v", m.CurrentDatabase, cmd != nil)
	}

	// DatabaseSelected without cmds
	m = khModel(t)
	m.Cmds = nil
	nm, cmd = m.Update(types.DatabaseSelectedMsg{Database: "x", Info: types.ServerInfo{Database: "x"}})
	if cmd != nil {
		t.Fatal("no cmds")
	}
	_ = nm
	_ = time.Millisecond
}
