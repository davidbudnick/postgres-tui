package ui

import (
	"path/filepath"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/cmd"
	"github.com/davidbudnick/postgres-tui/internal/db"
	"github.com/davidbudnick/postgres-tui/internal/testutil"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func extraModel(t *testing.T) Model {
	t.Helper()
	cfg, err := db.NewConfig(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	m.Width = 120
	m.Height = 40
	m.Cmds = cmd.NewCommands(cfg, testutil.NewMockPG())
	m.Connections = []types.Connection{
		{ID: 1, Name: "local", Host: "localhost", Port: 5432, Username: "u", Password: "p", Database: "demo", SSLMode: types.SSLModePrefer},
		{ID: 2, Name: "other", Host: "h2", Port: 5433, Database: "d2"},
	}
	m.SelectedConnIdx = 0
	m.CurrentConn = &m.Connections[0]
	m.CurrentDatabase = "demo"
	m.CurrentSchema = "public"
	m.Schemas = []types.SchemaInfo{{Name: "public", TableCount: 2}}
	m.SelectedSchema = 0
	m.KindEnabled = defaultKindFilters()
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "orders", Kind: types.ObjectTable},
		{Schema: "public", Name: "users", Kind: types.ObjectTable},
		{Schema: "public", Name: "id_seq", Kind: types.ObjectSequence},
	}
	m.SelectedObjIdx = 0
	m.FilterActive = false
	m.PageSize = 50
	return m
}

func extraArea(val string) textarea.Model {
	ta := textarea.New()
	ta.SetWidth(40)
	ta.SetHeight(6)
	ta.SetValue(val)
	ta.Focus()
	return ta
}

func extraKey(s string) tea.KeyPressMsg {
	if s == "" {
		return tea.KeyPressMsg{}
	}
	return tea.KeyPressMsg{Text: s, Code: []rune(s)[0]}
}

func TestExtra_KeysConnections(t *testing.T) {
	m := extraModel(t)
	m.Screen = types.ScreenConnections

	_, cmd := m.keysConnections("q")
	if cmd == nil {
		t.Fatal("q quit")
	}
	for _, k := range []string{"j", "down", "k", "up", "g", "G"} {
		nm, _ := m.keysConnections(k)
		m = nm.(Model)
	}
	nm, _ := m.keysConnections("a")
	m = nm.(Model)
	if m.Screen != types.ScreenAddConnection {
		t.Fatal("add")
	}
	m.Screen = types.ScreenConnections
	nm, _ = m.keysConnections("e")
	m = nm.(Model)
	if m.Screen != types.ScreenEditConnection || m.EditingConn == nil {
		t.Fatal("edit")
	}
	m.Screen = types.ScreenConnections
	nm, _ = m.keysConnections("d")
	m = nm.(Model)
	if m.Screen != types.ScreenConfirmDelete {
		t.Fatal("delete confirm")
	}
	m.Screen = types.ScreenConnections
	_, cmd = m.keysConnections("t")
	if cmd == nil {
		t.Fatal("test")
	}
	_, cmd = m.keysConnections("enter")
	if cmd == nil {
		t.Fatal("connect")
	}
	_, cmd = m.keysConnections("r")
	if cmd == nil {
		t.Fatal("reload")
	}

	m.Connections = nil
	for _, k := range []string{"j", "k", "G", "e", "d", "t", "enter"} {
		_, cmd = m.keysConnections(k)
		if (k == "e" || k == "d" || k == "t" || k == "enter") && cmd != nil {
			t.Fatalf("%s empty should nil", k)
		}
	}
	m.Connections = []types.Connection{{ID: 1, Name: "x"}}
	m.Cmds = nil
	for _, k := range []string{"t", "enter", "r"} {
		_, cmd = m.keysConnections(k)
		if cmd != nil {
			t.Fatalf("%s no cmds", k)
		}
	}
}

func TestExtra_KeysConnectionFormAndSave(t *testing.T) {
	m := extraModel(t)
	m.Screen = types.ScreenAddConnection
	m.ConnInputs = createConnectionInputs()
	m.ConnFocusIdx = 0
	m.focusConnField()

	nm, _ := m.keysConnectionForm("esc", extraKey("esc"))
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("esc")
	}
	m.Screen = types.ScreenAddConnection
	for _, k := range []string{"tab", "down", "shift+tab", "up"} {
		nm, _ = m.keysConnectionForm(k, extraKey(k))
		m = nm.(Model)
	}

	m.ConnFocusIdx = connFieldSSL
	m.ConnSSLIdx = 1
	nm, _ = m.keysConnectionForm("left", extraKey("left"))
	m = nm.(Model)
	nm, _ = m.keysConnectionForm("right", extraKey("right"))
	m = nm.(Model)
	m.ConnFocusIdx = 0
	nm, _ = m.keysConnectionForm("left", extraKey("left"))
	m = nm.(Model)
	nm, _ = m.keysConnectionForm("right", extraKey("right"))
	m = nm.(Model)

	m.ConnFocusIdx = connFieldReadOnly
	nm, _ = m.keysConnectionForm(" ", extraKey(" "))
	m = nm.(Model)
	if !m.ConnReadOnly {
		t.Fatal("readonly toggle")
	}
	m.ConnFocusIdx = 0
	m.focusConnField()
	nm, _ = m.keysConnectionForm(" ", extraKey(" "))
	m = nm.(Model)
	nm, _ = m.keysConnectionForm("x", extraKey("x"))
	m = nm.(Model)

	m.ConnInputs = createConnectionInputs()
	nm, _ = m.saveConnectionForm()
	m = nm.(Model)
	if m.Err == nil {
		t.Fatal("name/host required")
	}
	m.ConnInputs[connFieldName].SetValue("n")
	m.ConnInputs[connFieldHost].SetValue("h")
	m.ConnInputs[connFieldPort].SetValue("bad")
	nm, cmd := m.saveConnectionForm()
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("add with default port")
	}

	m.ConnInputs[connFieldPort].SetValue("5432")
	m.ConnInputs[connFieldUser].SetValue("u")
	m.ConnInputs[connFieldPass].SetValue("p")
	m.ConnInputs[connFieldDatabase].SetValue("d")
	nm, cmd = m.keysConnectionForm("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter save")
	}

	m.Screen = types.ScreenEditConnection
	ed := m.Connections[0]
	m.EditingConn = &ed
	m.ConnInputs = createConnectionInputs()
	m.ConnInputs[connFieldName].SetValue("n2")
	m.ConnInputs[connFieldHost].SetValue("h2")
	m.ConnInputs[connFieldPort].SetValue("5433")
	nm, cmd = m.saveConnectionForm()
	if cmd == nil {
		t.Fatal("update")
	}

	m.Cmds = nil
	m.ConnInputs[connFieldName].SetValue("n")
	m.ConnInputs[connFieldHost].SetValue("h")
	nm, _ = m.saveConnectionForm()
	m = nm.(Model)
	if m.Err == nil {
		t.Fatal("uninit")
	}

	m.ConnFocusIdx = connFieldSSL
	nm, _ = m.keysConnectionForm("x", extraKey("x"))
	_ = nm
}

func TestExtra_OpenQuery(t *testing.T) {
	m := extraModel(t)
	m.CurrentObject = &m.Objects[0]
	nm, cmd := m.openQuery()
	m = nm.(Model)
	if m.Screen != types.ScreenQuery || m.QueryFocus != "editor" || m.QueryArea == nil || cmd == nil {
		t.Fatal("prefill + run")
	}

	area := extraArea("")
	m.QueryArea = &area
	m.CurrentObject = &m.Objects[0]
	nm, cmd = m.openQuery()
	m = nm.(Model)
	if m.QueryArea.Value() == "" || cmd == nil {
		t.Fatal("prefill empty area")
	}

	kept := extraArea("SELECT 1")
	m.QueryArea = &kept
	nm, _ = m.openQuery()
	m = nm.(Model)
	if m.QueryArea.Value() != "SELECT 1" {
		t.Fatal("keep existing")
	}

	m.Cmds = nil
	m.QueryArea = nil
	m.CurrentObject = &m.Objects[0]
	nm, cmd = m.openQuery()
	m = nm.(Model)
	if cmd != nil || m.QueryArea == nil {
		t.Fatal("no cmds still creates area")
	}

	seq := m.Objects[2]
	m.CurrentObject = &seq
	m.QueryArea = nil
	m.Cmds = extraModel(t).Cmds
	nm, cmd = m.openQuery()
	m = nm.(Model)
	if cmd != nil {
		t.Fatal("non-relation no autorun")
	}
	if m.QueryArea == nil {
		t.Fatal("area for non-rel")
	}
}

func TestExtra_KeysTableData(t *testing.T) {
	m := extraModel(t)
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	obj := m.Objects[0]
	m.CurrentObject = &obj
	m.TableData = types.QueryResult{
		Columns:   []string{"id", "name", "x"},
		Rows:      [][]string{{"1", "a", "x"}, {"2", "b", "y"}},
		Truncated: true,
	}
	m.DataCursor, m.DataCol, m.DataOffset, m.PageSize = 0, 1, 0, 2

	for _, k := range []string{"tab", "shift+tab"} {
		nm, _ := m.keysTableData(k)
		m = nm.(Model)
		m.Screen = types.ScreenTableData
	}
	nm, _ := m.keysTableData("esc")
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatal("esc")
	}
	m.Screen = types.ScreenTableData
	m.Focus = focusSidebar
	nm, _ = m.keysTableData("j")
	m = nm.(Model)
	m.Focus = focusMain
	nm, _ = m.keysTableData("k")
	m = nm.(Model)

	m.Focus = focusContent
	for _, k := range []string{"j", "down", "k", "up", "h", "left", "l", "right", "g", "G", "0", "$"} {
		nm, _ = m.keysTableData(k)
		m = nm.(Model)
	}
	_, cmd := m.keysTableData("n")
	if cmd == nil {
		t.Fatal("n page")
	}
	m.DataOffset = 2
	_, cmd = m.keysTableData("p")
	if cmd == nil {
		t.Fatal("p page")
	}
	_, cmd = m.keysTableData("D")
	if cmd == nil {
		t.Fatal("D detail")
	}
	m.Objects = nil
	m.CurrentObject = &obj
	_, cmd = m.keysTableData("D")
	if cmd == nil {
		t.Fatal("D from current")
	}
	m.Objects = extraModel(t).Objects
	nm, _ = m.keysTableData("x")
	m = nm.(Model)
	if m.Screen != types.ScreenExport {
		t.Fatal("export")
	}
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	_, cmd = m.keysTableData("y")
	if cmd == nil {
		t.Fatal("y cell")
	}
	_, cmd = m.keysTableData("Y")
	if cmd == nil {
		t.Fatal("Y row")
	}
	_, cmd = m.keysTableData("r")
	if cmd == nil {
		t.Fatal("r reload")
	}
	_, cmd = m.keysTableData("E")
	if cmd == nil {
		t.Fatal("E erd")
	}
	nm, _ = m.keysTableData(";")
	m = nm.(Model)
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m.CurrentObject = &obj
	nm, _ = m.keysTableData(":")
	m = nm.(Model)

	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m.TableData = types.QueryResult{}
	_, _ = m.keysTableData("$")
	_, _ = m.keysTableData("l")
	_, cmd = m.keysTableData("y")
	if cmd != nil {
		t.Fatal("y empty")
	}
	_, cmd = m.keysTableData("Y")
	if cmd != nil {
		t.Fatal("Y empty")
	}
	m.CurrentObject = nil
	m.Objects = nil
	for _, k := range []string{"n", "p", "r", "D"} {
		_, cmd = m.keysTableData(k)
		if cmd != nil {
			t.Fatalf("%s no obj", k)
		}
	}
	m.DataOffset = 0
	m.TableData.Truncated = false
	m.CurrentObject = &obj
	_, cmd = m.keysTableData("n")
	if cmd != nil {
		t.Fatal("n no more")
	}
	m.Inputs = nil
	m.TableData = types.QueryResult{Rows: [][]string{{"1"}}}
	nm, _ = m.keysTableData("x")
	_ = nm
}

func TestExtra_KeysTableDetail(t *testing.T) {
	m := extraModel(t)
	m.Screen = types.ScreenTableDetail
	m.Focus = focusContent
	obj := m.Objects[0]
	m.CurrentObject = &obj
	m.TableDetail = types.TableDetail{
		Object:      obj,
		Columns:     []types.ColumnInfo{{Name: "id"}},
		Indexes:     []types.IndexInfo{{Name: "pk"}},
		Constraints: []types.ConstraintInfo{{Name: "c"}},
		CreateSQL:   "CREATE TABLE",
	}
	for _, k := range []string{"tab", "shift+tab"} {
		nm, _ := m.keysTableDetail(k)
		m = nm.(Model)
		m.Screen = types.ScreenTableDetail
	}
	nm, _ := m.keysTableDetail("esc")
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatal("esc")
	}
	m.Screen = types.ScreenTableDetail
	m.Focus = focusSidebar
	nm, _ = m.keysTableDetail("j")
	m = nm.(Model)
	m.Focus = focusMain
	nm, _ = m.keysTableDetail("k")
	m = nm.(Model)

	m.Focus = focusContent
	for _, k := range []string{"h", "left", "l", "right", "1", "2", "3", "4"} {
		nm, _ = m.keysTableDetail(k)
		m = nm.(Model)
	}
	_, cmd := m.keysTableDetail("enter")
	if cmd == nil {
		t.Fatal("enter data")
	}
	seq := m.Objects[2]
	m.CurrentObject = &seq
	m.Objects = nil
	_, cmd = m.keysTableDetail("enter")
	if cmd != nil {
		t.Fatal("nonrel enter")
	}
	m.CurrentObject = &obj
	m.Objects = extraModel(t).Objects
	m.SelectedObjIdx = 0
	_, cmd = m.keysTableDetail("D")
	if cmd == nil {
		t.Fatal("D")
	}
	m.Objects = nil
	m.CurrentObject = &obj
	_, cmd = m.keysTableDetail("D")
	if cmd == nil {
		t.Fatal("D current")
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
}

func TestExtra_KeysQuery(t *testing.T) {
	m := extraModel(t)
	m.Screen = types.ScreenQuery
	m.Focus = focusContent
	m.QueryFocus = "editor"
	m.FilterActive = false
	area := extraArea("SELECT 1")
	m.QueryArea = &area

	nm, _ := m.keysQuery("esc", tea.KeyPressMsg{Code: tea.KeyEsc})
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatal("esc leaves")
	}

	m.Screen = types.ScreenQuery
	m.Focus = focusContent
	m.QueryFocus = "editor"
	area = extraArea("SELECT * FROM ")
	m.QueryArea = &area
	m.QuerySuggests = []string{"orders", "users"}
	m.QuerySuggestIdx = 0
	nm, _ = m.keysQuery("tab", tea.KeyPressMsg{Code: tea.KeyTab})
	m = nm.(Model)

	m.QuerySuggests = nil
	m.QueryFocus = "editor"
	nm, _ = m.keysQuery("tab", tea.KeyPressMsg{Code: tea.KeyTab})
	m = nm.(Model)
	if m.QueryFocus != "results" {
		t.Fatal("tab to results")
	}
	nm, _ = m.keysQuery("tab", tea.KeyPressMsg{Code: tea.KeyTab})
	m = nm.(Model)
	if m.QueryFocus != "editor" {
		t.Fatal("tab to editor")
	}
	m.Focus = focusSidebar
	nm, _ = m.keysQuery("tab", tea.KeyPressMsg{Code: tea.KeyTab})
	m = nm.(Model)
	if m.Focus != focusContent || m.QueryFocus != "editor" {
		t.Fatal("tab from sidebar")
	}
	m.Focus = focusMain
	area = extraArea("x")
	m.QueryArea = &area
	nm, _ = m.keysQuery("tab", tea.KeyPressMsg{Code: tea.KeyTab})
	m = nm.(Model)

	m.Focus = focusContent
	m.QueryFocus = "results"
	nm, _ = m.keysQuery("shift+tab", tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m = nm.(Model)
	if m.QueryFocus != "editor" {
		t.Fatal("shift+tab results→editor")
	}
	nm, _ = m.keysQuery("shift+tab", tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m = nm.(Model)
	if m.Focus != focusMain {
		t.Fatal("shift+tab content→main")
	}
	nm, _ = m.keysQuery("shift+tab", tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m = nm.(Model)
	if m.Focus != focusSidebar {
		t.Fatal("shift+tab main→sidebar")
	}
	nm, _ = m.keysQuery("shift+tab", tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m = nm.(Model)
	if m.Focus != focusContent {
		t.Fatal("shift+tab sidebar→content")
	}

	m.Focus = focusContent
	m.QueryFocus = "editor"
	area = extraArea("SELECT 1")
	m.QueryArea = &area
	for _, key := range []string{"ctrl+enter", "ctrl+j", "ctrl+r", "f5", "ctrl+m", "alt+enter", "ctrl+e", "ctrl+return"} {
		nm, cmd := m.keysQuery(key, extraKey(key))
		m = nm.(Model)
		if cmd == nil {
			t.Fatalf("run %q", key)
		}
	}
	msg := tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl}
	nm, cmd := m.keysQuery("unknown", msg)
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("msg.String run path")
	}

	m.QueryArea = nil
	_, cmd = m.keysQuery("ctrl+enter", tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	if cmd != nil {
		t.Fatal("nil area")
	}
	area = extraArea("x")
	m.QueryArea = &area
	m.Cmds = nil
	_, cmd = m.keysQuery("ctrl+enter", tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModCtrl})
	if cmd != nil {
		t.Fatal("nil cmds")
	}
	m.Cmds = extraModel(t).Cmds

	m.QueryResult = types.QueryResult{Rows: [][]string{{"1", "a"}}, Columns: []string{"id", "name"}}
	nm, _ = m.keysQuery("x", extraKey("x"))
	m = nm.(Model)
	if m.Screen != types.ScreenExport {
		t.Fatal("export with rows")
	}
	m.Screen = types.ScreenQuery
	m.QueryResult.Rows = nil
	nm, _ = m.keysQuery("x", extraKey("x"))
	m = nm.(Model)
	if m.Screen != types.ScreenQuery {
		t.Fatal("export without rows")
	}
	m.Inputs = nil
	m.QueryResult.Rows = [][]string{{"1"}}
	nm, _ = m.keysQuery("x", extraKey("x"))
	m = nm.(Model)

	m.Screen = types.ScreenQuery
	m.Focus = focusSidebar
	m.Inputs = NewModel().Inputs
	nm, _ = m.keysQuery("j", extraKey("j"))
	m = nm.(Model)
	m.Focus = focusMain
	nm, _ = m.keysQuery("k", extraKey("k"))
	m = nm.(Model)

	m.Focus = focusContent
	m.QueryFocus = "results"
	m.QueryResult = types.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "a"}, {"2", "b"}},
	}
	m.DataCursor, m.DataCol = 0, 0
	for _, k := range []string{"j", "down", "k", "up", "h", "left", "l", "right"} {
		nm, _ = m.keysQuery(k, extraKey(k))
		m = nm.(Model)
		m.QueryFocus = "results"
	}
	_, cmd = m.keysQuery("y", extraKey("y"))
	if cmd == nil {
		t.Fatal("y cell")
	}
	_, cmd = m.keysQuery("Y", extraKey("Y"))
	if cmd == nil {
		t.Fatal("Y row")
	}
	m.QueryResult.Rows = nil
	_, cmd = m.keysQuery("y", extraKey("y"))
	if cmd != nil {
		t.Fatal("y empty")
	}
	_, cmd = m.keysQuery("Y", extraKey("Y"))
	if cmd != nil {
		t.Fatal("Y empty")
	}
	m.QueryResult.Rows = [][]string{{"1"}}
	m.Cmds = nil
	_, cmd = m.keysQuery("y", extraKey("y"))
	if cmd != nil {
		t.Fatal("y no cmds")
	}
	m.Cmds = extraModel(t).Cmds
	m.QueryResult = types.QueryResult{Columns: nil, Rows: [][]string{{"only"}}}
	m.DataCol = 0
	_, _ = m.keysQuery("l", extraKey("l"))
	m.QueryResult = types.QueryResult{
		Columns: []string{"a"},
		Rows:    [][]string{{}},
	}
	m.DataCursor, m.DataCol = 0, 0
	_, cmd = m.keysQuery("y", extraKey("y"))
	if cmd == nil {
		t.Fatal("y empty cell")
	}

	m.QueryFocus = "editor"
	area = extraArea("SELECT u.")
	m.QueryArea = &area
	m.QuerySuggests = []string{"users", "orders"}
	m.QuerySuggestIdx = 0
	nm, _ = m.keysQuery("down", extraKey("down"))
	m = nm.(Model)
	if m.QuerySuggestIdx != 1 {
		t.Fatal("suggest down")
	}
	nm, _ = m.keysQuery("ctrl+n", extraKey("ctrl+n"))
	m = nm.(Model)
	nm, _ = m.keysQuery("up", extraKey("up"))
	m = nm.(Model)
	nm, _ = m.keysQuery("ctrl+p", extraKey("ctrl+p"))
	m = nm.(Model)
	nm, _ = m.keysQuery("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	m = nm.(Model)
	m.QuerySuggests = []string{"users", "orders"}
	m.QuerySuggestIdx = 1
	nm, _ = m.keysQuery("esc", tea.KeyPressMsg{Code: tea.KeyEsc})
	m = nm.(Model)
	if len(m.QuerySuggests) != 0 || m.Screen != types.ScreenQuery {
		t.Fatal("esc dismisses suggestions first")
	}

	for _, k := range []string{"ctrl+@", "ctrl+ ", "alt+/", "ctrl+space"} {
		nm, _ = m.keysQuery(k, extraKey(k))
		m = nm.(Model)
		m.QueryFocus = "editor"
		m.QuerySuggests = []string{"users"}
	}
	nm, _ = m.keysQuery("a", extraKey("a"))
	m = nm.(Model)

	m.QueryFocus = "editor"
	m.QueryArea = nil
	nm, _ = m.keysQuery("a", extraKey("a"))
	_ = nm
}

func TestExtra_KeysActivity(t *testing.T) {
	m := extraModel(t)
	m.Screen = types.ScreenActivity
	m.Focus = focusContent
	m.Activity = []types.ActivityRow{
		{PID: 1, State: "active"},
		{PID: 2, State: "idle"},
	}
	m.SelectedActIdx = 0

	for _, k := range []string{"tab", "shift+tab"} {
		nm, _ := m.keysActivity(k)
		m = nm.(Model)
		m.Screen = types.ScreenActivity
	}
	nm, _ := m.keysActivity("esc")
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatal("esc")
	}
	m.Screen = types.ScreenActivity
	m.Focus = focusSidebar
	nm, _ = m.keysActivity("j")
	m = nm.(Model)
	m.Focus = focusMain
	nm, _ = m.keysActivity("k")
	m = nm.(Model)

	m.Focus = focusContent
	for _, k := range []string{"j", "down", "k", "up"} {
		nm, _ = m.keysActivity(k)
		m = nm.(Model)
	}
	_, cmd := m.keysActivity("r")
	if cmd == nil {
		t.Fatal("r")
	}
	m.Activity = nil
	nm, _ = m.keysActivity("j")
	m = nm.(Model)
	m.Cmds = nil
	_, cmd = m.keysActivity("r")
	if cmd != nil {
		t.Fatal("r no cmds")
	}
	_, _ = m.keysActivity("z")
}

func TestExtra_KeysServerInfo(t *testing.T) {
	m := extraModel(t)
	m.Screen = types.ScreenServerInfo
	m.Focus = focusContent

	for _, k := range []string{"tab", "shift+tab"} {
		nm, _ := m.keysServerInfo(k)
		m = nm.(Model)
		m.Screen = types.ScreenServerInfo
	}
	nm, _ := m.keysServerInfo("esc")
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatal("esc")
	}
	m.Screen = types.ScreenServerInfo
	m.Focus = focusSidebar
	nm, _ = m.keysServerInfo("j")
	m = nm.(Model)
	m.Focus = focusMain
	nm, _ = m.keysServerInfo("k")
	m = nm.(Model)

	m.Focus = focusContent
	_, cmd := m.keysServerInfo("r")
	if cmd == nil {
		t.Fatal("r")
	}
	m.Cmds = nil
	_, cmd = m.keysServerInfo("r")
	if cmd != nil {
		t.Fatal("r no cmds")
	}
	_, _ = m.keysServerInfo("z")
}

func TestExtra_KeysERD(t *testing.T) {
	m := extraModel(t)
	m.Screen = types.ScreenERD
	m.Focus = focusContent
	m.ERDOffset = 0

	for _, k := range []string{"tab", "shift+tab"} {
		nm, _ := m.keysERD(k)
		m = nm.(Model)
		m.Screen = types.ScreenERD
	}
	nm, _ := m.keysERD("esc")
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser || m.Focus != focusSidebar {
		t.Fatal("esc")
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
	_, cmd := m.keysERD("r")
	if cmd == nil {
		t.Fatal("r openERD")
	}
	_, _ = m.keysERD("z")
}

func TestExtra_KeysConfirm(t *testing.T) {
	m := extraModel(t)
	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "connection"
	m.ConfirmData = m.Connections[0]

	nm, _ := m.keysConfirm("n")
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("n")
	}
	m.Screen = types.ScreenConfirmDelete
	nm, _ = m.keysConfirm("esc")
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("esc")
	}

	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "connection"
	m.ConfirmData = m.Connections[0]
	_, cmd := m.keysConfirm("y")
	if cmd == nil {
		t.Fatal("y delete")
	}
	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "connection"
	m.ConfirmData = m.Connections[0]
	_, cmd = m.keysConfirm("enter")
	if cmd == nil {
		t.Fatal("enter delete")
	}

	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "connection"
	m.ConfirmData = "not-a-conn"
	nm, _ = m.keysConfirm("y")
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("bad data")
	}
	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "other"
	m.ConfirmData = m.Connections[0]
	nm, _ = m.keysConfirm("y")
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("other type")
	}
	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "connection"
	m.ConfirmData = m.Connections[0]
	m.Cmds = nil
	nm, _ = m.keysConfirm("y")
	m = nm.(Model)
	if m.Screen != types.ScreenConnections {
		t.Fatal("no cmds")
	}
	_, _ = m.keysConfirm("z")
}

func TestExtra_KeysExport(t *testing.T) {
	m := extraModel(t)
	m.Screen = types.ScreenExport
	m.PrevScreen = types.ScreenTableData
	m.TableData = types.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}}}
	m.QueryResult = types.QueryResult{Columns: []string{"q"}, Rows: [][]string{{"2"}}}
	m.Inputs.ExportInput.SetValue("/tmp/out.csv")
	m.Inputs.ExportInput.Focus()

	nm, _ := m.keysExport("esc", tea.KeyPressMsg{Code: tea.KeyEsc})
	m = nm.(Model)
	if m.Screen != types.ScreenTableData {
		t.Fatal("esc prev")
	}
	m.Screen = types.ScreenExport
	m.PrevScreen = 0
	nm, _ = m.keysExport("esc", tea.KeyPressMsg{Code: tea.KeyEsc})
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatal("esc default")
	}

	m.Screen = types.ScreenExport
	m.PrevScreen = types.ScreenTableData
	_, cmd := m.keysExport("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("export table")
	}
	m.PrevScreen = types.ScreenQuery
	_, cmd = m.keysExport("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("export query")
	}

	m.Cmds = nil
	_, cmd = m.keysExport("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("no cmds")
	}
	m.Cmds = extraModel(t).Cmds
	m.Inputs = nil
	_, cmd = m.keysExport("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("no inputs")
	}
	_, _ = m.keysExport("x", extraKey("x"))

	m.Inputs = NewModel().Inputs
	nm, _ = m.keysExport("x", extraKey("x"))
	_ = nm
}

func TestExtra_KeysPaletteAndRun(t *testing.T) {
	m := extraModel(t)
	m.Screen = types.ScreenCommandPalette
	m.PrevScreen = types.ScreenBrowser
	m.PaletteItems = defaultPaletteItems()
	m.PaletteIdx = 0
	m.Inputs.PaletteInput.SetValue("")
	m.Inputs.PaletteInput.Focus()

	nm, _ := m.keysPalette("esc", tea.KeyPressMsg{Code: tea.KeyEsc})
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatal("esc")
	}
	m.Screen = types.ScreenCommandPalette
	for _, k := range []string{"down", "ctrl+n", "up", "ctrl+p"} {
		nm, _ = m.keysPalette(k, extraKey(k))
		m = nm.(Model)
		m.Screen = types.ScreenCommandPalette
	}

	m.Inputs.PaletteInput.SetValue("zzzz-no-match")
	_, cmd := m.keysPalette("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("enter empty filter")
	}
	m.Inputs.PaletteInput.SetValue("")
	m.PaletteIdx = 0
	nm, cmd = m.keysPalette("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	m = nm.(Model)
	if m.Screen == types.ScreenCommandPalette {
		t.Fatal("enter should leave palette")
	}

	m.Screen = types.ScreenCommandPalette
	m.PaletteItems = defaultPaletteItems()
	nm, _ = m.keysPalette("q", extraKey("q"))
	m = nm.(Model)

	m.Inputs = nil
	nm, _ = m.keysPalette("down", extraKey("down"))
	m = nm.(Model)
	nm, _ = m.keysPalette("x", extraKey("x"))
	_ = nm
	m.PaletteItems = nil
	_, cmd = m.keysPalette("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("no items")
	}

	// runPalette — connected paths
	m = extraModel(t)
	for _, id := range []string{"query", "activity", "erd", "server", "tables", "views", "databases", "disconnect", "help", "logs"} {
		mm := extraModel(t)
		nm, cmd = mm.runPalette(id)
		_ = nm
		_ = cmd
	}
	// connect-required without conn
	m.CurrentConn = nil
	for _, id := range []string{"query", "activity", "erd", "server", "tables", "views", "databases"} {
		nm, _ = m.runPalette(id)
		mm := nm.(Model)
		if mm.StatusMsg != "Connect first" {
			t.Fatalf("%s wants connect first", id)
		}
	}
	// no cmds branches
	m = extraModel(t)
	m.Cmds = nil
	for _, id := range []string{"activity", "server", "disconnect"} {
		_, cmd = m.runPalette(id)
		if cmd != nil {
			t.Fatalf("%s no cmds", id)
		}
	}
	// unknown id
	_, _ = m.runPalette("nope")
}
