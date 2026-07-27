package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func key(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: s}
}

func TestKeysConnectionsAndForm(t *testing.T) {
	m := covModel(t)
	m.Screen = types.ScreenConnections
	for _, k := range []string{"j", "k", "g", "G", "a", "e", "d", "t", "enter", "r", "q", "down", "up"} {
		nm, _ := m.keysConnections(k)
		m = nm.(Model)
		m.Screen = types.ScreenConnections
		if len(m.Connections) == 0 {
			m.Connections = covModel(t).Connections
		}
	}
	// empty connections branches
	m.Connections = nil
	for _, k := range []string{"e", "d", "t", "enter", "j", "k", "G"} {
		nm, _ := m.keysConnections(k)
		m = nm.(Model)
	}

	m = covModel(t)
	m.Screen = types.ScreenAddConnection
	m.ConnInputs = createConnectionInputs()
	m.ConnFocusIdx = 0
	m.focusConnField()
	for _, k := range []string{"tab", "down", "shift+tab", "up", "esc"} {
		nm, _ := m.keysConnectionForm(k, key(k))
		m = nm.(Model)
		m.Screen = types.ScreenAddConnection
	}
	m.ConnFocusIdx = connFieldSSL
	_, _ = m.keysConnectionForm("left", key("left"))
	_, _ = m.keysConnectionForm("right", key("right"))
	m.ConnFocusIdx = connFieldReadOnly
	_, _ = m.keysConnectionForm(" ", key(" "))
	// typing into text field
	m.ConnFocusIdx = 0
	m.focusConnField()
	_, _ = m.keysConnectionForm("x", tea.KeyPressMsg{Text: "x"})
	// save validation
	nm, _ := m.saveConnectionForm()
	m = nm.(Model)
	m.ConnInputs[connFieldName].SetValue("n")
	m.ConnInputs[connFieldHost].SetValue("h")
	m.ConnInputs[connFieldPort].SetValue("bad")
	nm, _ = m.saveConnectionForm()
	m = nm.(Model)
	m.ConnInputs[connFieldPort].SetValue("5432")
	nm, _ = m.saveConnectionForm()
	m = nm.(Model)
	// edit path
	m.Screen = types.ScreenEditConnection
	ed := m.Connections[0]
	m.EditingConn = &ed
	m.ConnInputs[connFieldName].SetValue("n2")
	m.ConnInputs[connFieldHost].SetValue("h2")
	_, _ = m.saveConnectionForm()
	// nil cmds
	m.Cmds = nil
	_, _ = m.saveConnectionForm()
}

func TestKeysDatabases(t *testing.T) {
	m := covModel(t)
	m.Screen = types.ScreenDatabases
	for _, k := range []string{"j", "k", "g", "G", "enter", "r", "q", "esc", "down", "up"} {
		nm, _ := m.keysDatabases(k)
		m = nm.(Model)
		m.Screen = types.ScreenDatabases
	}
	m.CurrentDatabase = ""
	_, _ = m.keysDatabases("esc")
	m = covModel(t)
	m.CurrentDatabase = "demo"
	m.Objects = nil
	_, _ = m.keysDatabases("esc")
}

func TestKeysBrowserSidebarObjectSearch(t *testing.T) {
	m := covModel(t)
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	m = m.syncSidebarCursorToObject()

	for _, k := range []string{
		"j", "k", "g", "G", "tab", "shift+tab", "enter", " ", "1", "2", "3", "4", "5", "6",
		"/", ";", ":", "D", "A", "E", "i", "r", "esc", "h", "l", "?", "ctrl+p", "L", "x",
		"down", "up", "left", "right",
	} {
		nm, cmd := m.keysBrowser(k, key(k))
		m = nm.(Model)
		m.Screen = types.ScreenBrowser
		_ = cmd
	}

	// content databases mode
	m.ContentMode = contentDatabases
	m.Focus = focusContent
	for _, k := range []string{"j", "k", "g", "G", "enter", "esc", "r"} {
		nm, _ := m.keysDatabasesInContent(k)
		m = nm.(Model)
	}

	// object search
	nm, _ := m.startObjectSearch()
	m = nm.(Model)
	for _, ch := range "user" {
		nm, _ = m.keysObjectSearch(string(ch), tea.KeyPressMsg{Text: string(ch)})
		m = nm.(Model)
	}
	_, _ = m.keysObjectSearch("backspace", tea.KeyPressMsg{Code: tea.KeyBackspace})
	_, _ = m.keysObjectSearch("enter", tea.KeyPressMsg{Code: tea.KeyEnter})
	nm, _ = m.startObjectSearch()
	m = nm.(Model)
	_, _ = m.keysObjectSearch("esc", tea.KeyPressMsg{Code: tea.KeyEsc})

	// sidebar navigation helpers
	m = covModel(t)
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	for _, k := range []string{"j", "k", "g", "G", "enter", " ", "1"} {
		nm, _ := m.keysSidebar(k)
		m = nm.(Model)
	}
	_, _ = m.toggleKindFilter(navTables)
	_, _ = m.toggleKindFilter(navViews)
	_, _ = m.onSidebarCursorMoved()
	m = m.syncSelectionFromSidebar()
	_, _ = m.openSelectedObject()
	// open non-table kinds
	for i := range m.Objects {
		m.SelectedObjIdx = i
		m.CurrentObject = &m.Objects[i]
		_, _ = m.openSelectedObject()
		_, _ = m.beginObjectDetail(m.Objects[i])
	}
	_, _ = m.openDatabasesContent()
	_, _ = m.openERD()
	_, _ = m.beginTableDetail(m.Objects[0])
	_, _ = m.beginTableData(m.Objects[0], 0, 50)
	_, _ = m.beginTableData(m.Objects[0], 50, 50)
}

func TestKeysTableDataDetailQuery(t *testing.T) {
	m := covModel(t)
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m = m.setCurrentObject(m.Objects[0])
	for _, k := range []string{
		"j", "k", "h", "l", "g", "G", "n", "p", "y", "Y", "D", "x", "esc", "tab", "r", ";",
		"down", "up", "left", "right", "enter",
	} {
		nm, _ := m.keysTableData(k)
		m = nm.(Model)
		m.Screen = types.ScreenTableData
	}
	// empty data
	m.TableData.Rows = nil
	_, _ = m.keysTableData("y")
	_, _ = m.keysTableData("Y")

	m.Screen = types.ScreenTableDetail
	for _, k := range []string{"h", "l", "1", "2", "3", "j", "k", "esc", "tab", "r", "enter", "left", "right"} {
		nm, _ := m.keysTableDetail(k)
		m = nm.(Model)
		m.Screen = types.ScreenTableDetail
	}

	m.Screen = types.ScreenQuery
	m.QueryFocus = "editor"
	nm, _ := m.openQuery()
	m = nm.(Model)
	for _, k := range []string{
		"ctrl+enter", "ctrl+e", "tab", "shift+tab", "esc", "up", "down",
		"ctrl+p", "ctrl+n", "x", "r",
	} {
		nm, _ := m.keysQuery(k, key(k))
		m = nm.(Model)
		m.Screen = types.ScreenQuery
	}
	// type into editor
	_, _ = m.keysQuery("a", tea.KeyPressMsg{Text: "a"})
	m.QueryFocus = "results"
	for _, k := range []string{"j", "k", "h", "l", "g", "G", "y", "Y", "esc", "tab"} {
		nm, _ := m.keysQuery(k, key(k))
		m = nm.(Model)
		m.Screen = types.ScreenQuery
		m.QueryFocus = "results"
	}
	// suggestions
	m.QueryFocus = "editor"
	m.QuerySuggests = []string{"users", "orders"}
	m.QuerySuggestIdx = 0
	_, _ = m.keysQuery("down", key("down"))
	_, _ = m.keysQuery("up", key("up"))
	_, _ = m.keysQuery("tab", key("tab"))
	_, _ = m.keysQuery("esc", key("esc"))
}

func TestKeysActivityERDServerConfirmExportPalette(t *testing.T) {
	m := covModel(t)
	m.Screen = types.ScreenActivity
	for _, k := range []string{"j", "k", "g", "G", "r", "esc", "y", "down", "up"} {
		nm, _ := m.keysActivity(k)
		m = nm.(Model)
		m.Screen = types.ScreenActivity
	}

	m.Screen = types.ScreenERD
	for _, k := range []string{"j", "k", "g", "G", "r", "esc", "down", "up"} {
		nm, _ := m.keysERD(k)
		m = nm.(Model)
		m.Screen = types.ScreenERD
	}

	m.Screen = types.ScreenServerInfo
	for _, k := range []string{"r", "esc", "q"} {
		nm, _ := m.keysServerInfo(k)
		m = nm.(Model)
		m.Screen = types.ScreenServerInfo
	}

	m.Screen = types.ScreenConfirmDelete
	m.ConfirmType = "connection"
	m.ConfirmData = m.Connections[0]
	for _, k := range []string{"esc", "n", "y", "enter"} {
		nm, _ := m.keysConfirm(k)
		m = nm.(Model)
		m.Screen = types.ScreenConfirmDelete
		m.ConfirmType = "connection"
		if len(m.Connections) > 0 {
			m.ConfirmData = m.Connections[0]
		}
	}

	m = covModel(t)
	m.Screen = types.ScreenExport
	m.PrevScreen = types.ScreenTableData
	for _, k := range []string{"esc", "enter"} {
		nm, _ := m.keysExport(k, key(k))
		m = nm.(Model)
		m.Screen = types.ScreenExport
	}
	_, _ = m.keysExport("x", tea.KeyPressMsg{Text: "x"})

	m.Screen = types.ScreenCommandPalette
	m.PaletteItems = defaultPaletteItems()
	m.PaletteIdx = 0
	for _, k := range []string{"esc", "j", "k", "enter", "down", "up"} {
		nm, _ := m.keysPalette(k, key(k))
		m = nm.(Model)
		m.Screen = types.ScreenCommandPalette
	}
	_, _ = m.keysPalette("q", tea.KeyPressMsg{Text: "q"})
}

func TestHandleKeyPressDispatch(t *testing.T) {
	m := covModel(t)
	screens := []types.Screen{
		types.ScreenConnections, types.ScreenAddConnection, types.ScreenEditConnection,
		types.ScreenDatabases, types.ScreenBrowser, types.ScreenTableData, types.ScreenTableDetail,
		types.ScreenQuery, types.ScreenActivity, types.ScreenERD, types.ScreenServerInfo,
		types.ScreenHelp, types.ScreenTestConnection, types.ScreenConfirmDelete,
		types.ScreenLogs, types.ScreenFavorites, types.ScreenExport, types.ScreenCommandPalette,
	}
	for _, s := range screens {
		m.Screen = s
		m.Focus = focusContent
		for _, k := range []string{"esc", "?", "q", "j", "tab"} {
			nm, _ := m.handleKeyPress(key(k))
			if nm != nil {
				m = nm.(Model)
			}
			m.Screen = s
		}
	}
	// global ctrl+c
	_, _ = m.handleKeyPress(tea.KeyPressMsg{Text: "ctrl+c"})
}

func TestObjectListKeys(t *testing.T) {
	m := covModel(t)
	m.Screen = types.ScreenBrowser
	m.Focus = focusContent
	for _, k := range []string{"j", "k", "g", "G", "enter", "D", "/", "esc"} {
		nm, _ := m.keysObjectList(k)
		m = nm.(Model)
	}
}
