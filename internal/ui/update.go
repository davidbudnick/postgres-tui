package ui

import (
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

type sequencedDetailMsg struct {
	types.TableDetailLoadedMsg
	seq int
}

type sequencedDataMsg struct {
	types.TableDataLoadedMsg
	seq int
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		if m.QueryArea != nil {
			// Content pane is roughly half width once the workspace shell is open.
			m.QueryArea.SetWidth(max(msg.Width/2-6, 30))
			m.QueryArea.SetHeight(6)
		}
		return m, nil

	case types.StatusMsg:
		m.StatusMsg = msg.Text
		return m, nil

	case types.ConnectionsLoadedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err
			return m, nil
		}
		m.Connections = msg.Connections
		if m.SelectedConnIdx >= len(m.Connections) {
			m.SelectedConnIdx = max(0, len(m.Connections)-1)
		}
		return m, nil

	case types.ConnectionAddedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err
			return m, nil
		}
		m.Connections = append(m.Connections, msg.Connection)
		m.Screen = types.ScreenConnections
		m.StatusMsg = "Instance added"
		m.Err = nil
		return m, nil

	case types.ConnectionUpdatedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err
			return m, nil
		}
		for i, c := range m.Connections {
			if c.ID == msg.Connection.ID {
				m.Connections[i] = msg.Connection
				break
			}
		}
		m.Screen = types.ScreenConnections
		m.StatusMsg = "Instance updated"
		return m, nil

	case types.ConnectionDeletedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err
			return m, nil
		}
		filtered := m.Connections[:0]
		for _, c := range m.Connections {
			if c.ID != msg.ID {
				filtered = append(filtered, c)
			}
		}
		m.Connections = filtered
		if m.SelectedConnIdx >= len(m.Connections) {
			m.SelectedConnIdx = max(0, len(m.Connections)-1)
		}
		m.StatusMsg = "Instance deleted"
		m.Screen = types.ScreenConnections
		return m, nil

	case types.AutoConnectMsg:
		m.Loading = true
		m.ConnectionError = ""
		m.CurrentConn = &msg.Connection
		m.CLIConnection = nil
		if m.Cmds != nil {
			return m, m.Cmds.Connect(msg.Connection)
		}
		return m, nil

	case types.ConnectedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.ConnectionError = msg.Err.Error()
			m.Err = msg.Err
			m.Screen = types.ScreenConnections
			return m, nil
		}
		m.ServerInfo = msg.Info
		m.CurrentDatabase = msg.Info.Database
		if m.CurrentConn != nil {
			m.ReadOnly = m.CurrentConn.ReadOnly
			if m.CurrentDatabase == "" {
				m.CurrentDatabase = m.CurrentConn.Database
			}
		}
		if m.Cmds != nil {
			m.ReadOnly = m.Cmds.PG().IsReadOnly()
		}
		m.Err = nil
		m.ConnectionError = ""
		// Connect already opened the connection's database — go straight to workspace.
		// Never Batch LoadDatabases + SelectDatabase (SwitchDatabase closes the pool).
		if m.CurrentDatabase != "" && m.Cmds != nil {
			m = m.resetToBrowserList()
			m.NavSection = navTables
			m.SidebarCursor = 0
			m.SelectedObjIdx = 0
			m.Objects = nil
			m.CurrentSchema = "public"
			m.StatusMsg = "Opened " + m.CurrentDatabase
			m.Loading = true
			return m, m.Cmds.LoadSchemas()
		}
		// No default database on the connection — show the switcher.
		m.Screen = types.ScreenDatabases
		m.StatusMsg = "Connected"
		if m.Cmds != nil {
			m.Loading = true
			return m, m.Cmds.LoadDatabases()
		}
		return m, nil

	case types.DisconnectedMsg:
		m.CurrentConn = nil
		m.CurrentDatabase = ""
		m.ServerInfo = types.ServerInfo{}
		m.Screen = types.ScreenConnections
		m.StatusMsg = "Disconnected"
		return m, nil

	case types.DatabasesLoadedMsg:
		m.Loading = false
		if msg.Err != nil {
			// Don't clobber a healthy workspace with a stale list-databases race.
			if !m.isWorkspaceScreen() {
				m.Err = msg.Err
			}
			return m, nil
		}
		m.Databases = msg.Databases
		m.Err = nil
		for i, db := range m.Databases {
			if db.Name == m.CurrentDatabase {
				m.SelectedDBIdx = i
				break
			}
		}
		return m, nil

	case types.DatabaseSelectedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err
			return m, nil
		}
		m.CurrentDatabase = msg.Database
		m.ServerInfo = msg.Info
		m.Err = nil
		m = m.resetToBrowserList()
		m.NavSection = navTables
		m.SidebarCursor = 0
		m.SelectedObjIdx = 0
		m.Objects = nil
		m.CurrentSchema = "public"
		m.StatusMsg = "Opened " + msg.Database
		if m.Cmds != nil {
			m.Loading = true
			return m, m.Cmds.LoadSchemas()
		}
		return m, nil

	case types.SchemasLoadedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err
			return m, nil
		}
		m.Schemas = msg.Schemas
		if len(m.Schemas) > 0 {
			m.SelectedSchema = 0
			m.CurrentSchema = m.Schemas[0].Name
			for i, s := range m.Schemas {
				if s.Name == "public" {
					m.SelectedSchema = i
					m.CurrentSchema = "public"
					break
				}
			}
		}
		// Always refresh objects for enabled filters + schema after schemas land.
		if m.Screen == types.ScreenBrowser && m.Cmds != nil {
			m.Loading = true
			return m, m.Cmds.LoadObjectKinds(m.CurrentSchema, m.enabledObjectKinds())
		}
		return m, nil

	case types.ObjectsLoadedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err
			return m, nil
		}
		// Remember what the user was highlighting before the row list reshuffles.
		prevKind := sidebarRowKind(-1)
		prevNav := m.NavSection
		prevSchema := m.SelectedSchema
		if rows := m.buildSidebarRows(); len(rows) > 0 {
			r := rows[clamp(m.SidebarCursor, 0, len(rows)-1)]
			prevKind = r.kind
			if r.kind == sbKind {
				prevNav = r.nav
			}
			if r.kind == sbSchema {
				prevSchema = r.schema
			}
		}

		m.Objects = msg.Objects
		m.rebuildSQLCompleter()
		m.SelectedObjIdx = 0
		if m.CurrentObject != nil && !objectInList(m.Objects, m.CurrentObject) {
			m = m.clearObjectContent()
			if m.Screen == types.ScreenTableDetail || m.Screen == types.ScreenTableData {
				m.Screen = types.ScreenBrowser
				m.Focus = focusSidebar
			}
		}
		// Never auto-jump to first table — that made KIND unselectable ("keeps flipping down").
		switch prevKind {
		case sbKind:
			m = m.pinSidebarCursorToKind(prevNav)
		case sbSchema:
			m = m.pinSidebarCursorToSchema(prevSchema)
		default:
			m = m.pinSidebarAfterObjectsLoad()
		}
		return m, nil

	case sequencedDetailMsg:
		if msg.seq != m.contentSeq {
			return m, nil
		}
		return m.applyTableDetail(msg.TableDetailLoadedMsg)

	case types.TableDetailLoadedMsg:
		return m.applyTableDetail(msg)

	case sequencedDataMsg:
		if msg.seq != m.contentSeq {
			return m, nil
		}
		return m.applyTableData(msg.TableDataLoadedMsg)

	case types.TableDataLoadedMsg:
		return m.applyTableData(msg)

	case types.QueryResultMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err
			m.StatusMsg = ""
			return m, nil
		}
		m.QueryResult = msg.Result
		m.DataCursor = 0
		m.DataCol = 0
		m.Err = nil
		m.StatusMsg = fmt.Sprintf("OK · %s", msg.Result.Duration.Round(1e6))
		return m, nil

	case types.ActivityLoadedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err
			return m, nil
		}
		m.Activity = msg.Rows
		m.SelectedActIdx = clamp(m.SelectedActIdx, 0, max(len(m.Activity)-1, 0))
		m.Screen = types.ScreenActivity
		m.Focus = focusContent
		return m, nil

	case types.ERDLoadedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err
			return m, nil
		}
		m.ERD = msg.Graph
		m.ERDOffset = 0
		m.Screen = types.ScreenERD
		m.Focus = focusContent
		return m, nil

	case types.ServerInfoLoadedMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err
			return m, nil
		}
		m.ServerInfo = msg.Info
		m.Screen = types.ScreenServerInfo
		m.Focus = focusContent
		return m, nil

	case types.ConnectionTestMsg:
		m.Loading = false
		if msg.Success {
			m.TestConnResult = fmt.Sprintf("OK · %s · %s", msg.Latency.Round(1e6), shortVersion(msg.Info.Version))
		} else if msg.Err != nil {
			m.TestConnResult = msg.Err.Error()
		} else {
			m.TestConnResult = "failed"
		}
		m.Screen = types.ScreenTestConnection
		return m, nil

	case types.FavoritesLoadedMsg:
		m.Favorites = msg.Favorites
		return m, nil

	case types.ExportDoneMsg:
		m.Loading = false
		if msg.Err != nil {
			m.Err = msg.Err
			return m, nil
		}
		m.StatusMsg = fmt.Sprintf("Exported %d rows → %s", msg.Rows, msg.Path)
		m.Screen = m.PrevScreen
		if m.Screen == 0 {
			m.Screen = types.ScreenBrowser
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

func shortVersion(v string) string {
	if i := strings.Index(v, " on "); i > 0 {
		return v[:i]
	}
	if len(v) > 40 {
		return v[:40]
	}
	return v
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := normalizeKey(msg)

	// Global quit
	if key == "ctrl+c" {
		return m, tea.Quit
	}

	// Help toggle
	if key == "?" && !m.typingContext() {
		if m.Screen == types.ScreenHelp {
			m.Screen = m.PrevScreen
			return m, nil
		}
		m.PrevScreen = m.Screen
		m.Screen = types.ScreenHelp
		return m, nil
	}

	// Command palette
	if key == "ctrl+p" && !m.typingContext() {
		m.PrevScreen = m.Screen
		m.Screen = types.ScreenCommandPalette
		m.PaletteIdx = 0
		if m.Inputs != nil {
			m.Inputs.PaletteInput.SetValue("")
			m.Inputs.PaletteInput.Focus()
		}
		return m, nil
	}

	// Object tree search is workspace-global (not just ScreenBrowser).
	if m.isWorkspaceScreen() {
		if m.FilterActive {
			return m.keysObjectSearch(key, msg)
		}
		if key == "/" && !m.typingContext() {
			return m.startObjectSearch()
		}
	}

	switch m.Screen {
	case types.ScreenConnections:
		return m.keysConnections(key)
	case types.ScreenAddConnection, types.ScreenEditConnection:
		return m.keysConnectionForm(key, msg)
	case types.ScreenDatabases:
		return m.keysDatabases(key)
	case types.ScreenBrowser:
		return m.keysBrowser(key, msg)
	case types.ScreenTableData:
		return m.keysTableData(key)
	case types.ScreenTableDetail:
		return m.keysTableDetail(key)
	case types.ScreenQuery:
		return m.keysQuery(key, msg)
	case types.ScreenActivity:
		return m.keysActivity(key)
	case types.ScreenERD:
		return m.keysERD(key)
	case types.ScreenServerInfo:
		return m.keysServerInfo(key)
	case types.ScreenHelp, types.ScreenTestConnection:
		if key == "esc" || key == "enter" || key == "q" {
			if m.Screen == types.ScreenHelp {
				m.Screen = m.PrevScreen
			} else {
				m.Screen = types.ScreenConnections
			}
			return m, nil
		}
	case types.ScreenConfirmDelete:
		return m.keysConfirm(key)
	case types.ScreenLogs, types.ScreenFavorites:
		if key == "esc" || key == "q" {
			if m.PrevScreen != types.ScreenLogs && m.PrevScreen != types.ScreenFavorites {
				m.Screen = m.PrevScreen
			} else {
				m.Screen = types.ScreenBrowser
			}
			return m, nil
		}
	case types.ScreenExport:
		return m.keysExport(key, msg)
	case types.ScreenCommandPalette:
		return m.keysPalette(key, msg)
	}

	if key == "q" && !m.typingContext() {
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) keysConnections(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q":
		return m, tea.Quit
	case "j", "down":
		if len(m.Connections) > 0 {
			m.SelectedConnIdx = (m.SelectedConnIdx + 1) % len(m.Connections)
		}
	case "k", "up":
		if len(m.Connections) > 0 {
			m.SelectedConnIdx = (m.SelectedConnIdx - 1 + len(m.Connections)) % len(m.Connections)
		}
	case "g":
		m.SelectedConnIdx = 0
	case "G":
		if len(m.Connections) > 0 {
			m.SelectedConnIdx = len(m.Connections) - 1
		}
	case "a":
		m.Screen = types.ScreenAddConnection
		m.EditingConn = nil
		m.ConnInputs = createConnectionInputs()
		m.ConnFocusIdx = 0
		m.ConnReadOnly = false
		m.ConnSSLIdx = sslIndex(types.SSLModePrefer)
		m.ConnInputs[0].Focus()
		m.Err = nil
	case "e":
		if len(m.Connections) == 0 {
			return m, nil
		}
		conn := m.Connections[m.SelectedConnIdx]
		m.EditingConn = &conn
		m.Screen = types.ScreenEditConnection
		m.ConnInputs = createConnectionInputs()
		m.ConnInputs[connFieldName].SetValue(conn.Name)
		m.ConnInputs[connFieldHost].SetValue(conn.Host)
		m.ConnInputs[connFieldPort].SetValue(strconv.Itoa(conn.Port))
		m.ConnInputs[connFieldUser].SetValue(conn.Username)
		m.ConnInputs[connFieldPass].SetValue(conn.Password)
		m.ConnInputs[connFieldDatabase].SetValue(conn.Database)
		m.ConnReadOnly = conn.ReadOnly
		m.ConnSSLIdx = sslIndex(conn.SSLMode)
		m.ConnFocusIdx = 0
		m.ConnInputs[0].Focus()
		m.Err = nil
	case "d":
		if len(m.Connections) == 0 {
			return m, nil
		}
		m.ConfirmType = "connection"
		m.ConfirmData = m.Connections[m.SelectedConnIdx]
		m.Screen = types.ScreenConfirmDelete
	case "t":
		if len(m.Connections) == 0 || m.Cmds == nil {
			return m, nil
		}
		m.Loading = true
		m.TestConnResult = ""
		return m, m.Cmds.TestConnection(m.Connections[m.SelectedConnIdx])
	case "enter":
		if len(m.Connections) == 0 || m.Cmds == nil {
			return m, nil
		}
		conn := m.Connections[m.SelectedConnIdx]
		m.CurrentConn = &conn
		m.Loading = true
		m.ConnectionError = ""
		return m, m.Cmds.Connect(conn)
	case "r":
		if m.Cmds != nil {
			m.Loading = true
			return m, m.Cmds.LoadConnections()
		}
	}
	return m, nil
}

func (m Model) keysConnectionForm(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.Screen = types.ScreenConnections
		m.Err = nil
		return m, nil
	case "tab", "down":
		m.ConnFocusIdx = (m.ConnFocusIdx + 1) % connFieldCount
		m.focusConnField()
		return m, nil
	case "shift+tab", "up":
		m.ConnFocusIdx = (m.ConnFocusIdx - 1 + connFieldCount) % connFieldCount
		m.focusConnField()
		return m, nil
	case "left":
		if m.ConnFocusIdx == connFieldSSL {
			m.ConnSSLIdx = (m.ConnSSLIdx - 1 + len(sslModeOptions)) % len(sslModeOptions)
			return m, nil
		}
	case "right":
		if m.ConnFocusIdx == connFieldSSL {
			m.ConnSSLIdx = (m.ConnSSLIdx + 1) % len(sslModeOptions)
			return m, nil
		}
	case " ":
		if m.ConnFocusIdx == connFieldReadOnly {
			m.ConnReadOnly = !m.ConnReadOnly
			return m, nil
		}
	case "enter":
		return m.saveConnectionForm()
	}

	if m.ConnFocusIdx < connTextCount {
		var cmd tea.Cmd
		m.ConnInputs[m.ConnFocusIdx], cmd = m.ConnInputs[m.ConnFocusIdx].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) focusConnField() {
	for i := range m.ConnInputs {
		m.ConnInputs[i].Blur()
	}
	if m.ConnFocusIdx < connTextCount {
		m.ConnInputs[m.ConnFocusIdx].Focus()
	}
}

func (m Model) saveConnectionForm() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.ConnInputs[connFieldName].Value())
	host := strings.TrimSpace(m.ConnInputs[connFieldHost].Value())
	portStr := strings.TrimSpace(m.ConnInputs[connFieldPort].Value())
	user := strings.TrimSpace(m.ConnInputs[connFieldUser].Value())
	pass := m.ConnInputs[connFieldPass].Value()
	database := strings.TrimSpace(m.ConnInputs[connFieldDatabase].Value())
	if name == "" || host == "" {
		m.Err = fmt.Errorf("name and host are required")
		return m, nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		port = 5432
	}
	conn := types.Connection{
		Name:     name,
		Host:     host,
		Port:     port,
		Username: user,
		Password: pass,
		Database: database,
		SSLMode:  sslModeOptions[clamp(m.ConnSSLIdx, 0, len(sslModeOptions)-1)],
		ReadOnly: m.ConnReadOnly,
	}
	if m.Cmds == nil {
		m.Err = fmt.Errorf("not initialized")
		return m, nil
	}
	m.Loading = true
	if m.Screen == types.ScreenEditConnection && m.EditingConn != nil {
		conn.ID = m.EditingConn.ID
		return m, m.Cmds.UpdateConnection(conn)
	}
	return m, m.Cmds.AddConnection(conn)
}

func (m Model) keysDatabases(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q":
		return m, tea.Quit
	case "esc":
		// Switcher only: return to workspace if we already have a DB open.
		if m.CurrentDatabase != "" {
			m = m.resetToBrowserList()
			m.Err = nil
			m.StatusMsg = ""
			if m.Cmds != nil && len(m.Objects) == 0 {
				m.Loading = true
				return m, m.Cmds.LoadSchemas()
			}
			return m, nil
		}
		if m.Cmds != nil {
			return m, m.Cmds.Disconnect()
		}
		m.Screen = types.ScreenConnections
		return m, nil
	case "j", "down":
		if len(m.Databases) > 0 {
			m.SelectedDBIdx = (m.SelectedDBIdx + 1) % len(m.Databases)
		}
	case "k", "up":
		if len(m.Databases) > 0 {
			m.SelectedDBIdx = (m.SelectedDBIdx - 1 + len(m.Databases)) % len(m.Databases)
		}
	case "g":
		m.SelectedDBIdx = 0
	case "G":
		if len(m.Databases) > 0 {
			m.SelectedDBIdx = len(m.Databases) - 1
		}
	case "r":
		if m.Cmds != nil {
			m.Loading = true
			return m, m.Cmds.LoadDatabases()
		}
	case "enter":
		if len(m.Databases) == 0 || m.Cmds == nil {
			return m, nil
		}
		db := m.Databases[m.SelectedDBIdx]
		// Already on this DB — just return to workspace.
		if db.Name == m.CurrentDatabase {
			m = m.resetToBrowserList()
			m.Err = nil
			if len(m.Objects) == 0 {
				m.Loading = true
				return m, m.Cmds.LoadSchemas()
			}
			return m, nil
		}
		m.Loading = true
		m.Err = nil
		return m, m.Cmds.SelectDatabase(db.Name)
	}
	return m, nil
}

// startObjectSearch focuses the sidebar search box (/).
func (m Model) startObjectSearch() (tea.Model, tea.Cmd) {
	m.Focus = focusSidebar
	m.FilterActive = true
	m.FilterInput.SetWidth(max(sidebarTreeWidth-4, 16))
	// Keep existing query so / re-opens for edit; cursor at end.
	m.FilterInput.CursorEnd()
	cmd := m.FilterInput.Focus()
	m.ObjectFilter = m.FilterInput.Value()
	m.SelectedObjIdx = 0
	m = m.pinSidebarToFirstObject()
	return m, cmd
}

// clearObjectSearch exits search and shows the full object list.
func (m Model) clearObjectSearch() Model {
	m.FilterActive = false
	m.FilterInput.Blur()
	m.FilterInput.SetValue("")
	m.ObjectFilter = ""
	m.SelectedObjIdx = 0
	m = m.syncSidebarCursorToObject()
	return m
}

// commitObjectSearch keeps the query applied and returns focus to the tree.
func (m Model) commitObjectSearch() Model {
	m.ObjectFilter = m.FilterInput.Value()
	m.FilterActive = false
	m.FilterInput.Blur()
	m.SelectedObjIdx = 0
	m = m.pinSidebarToFirstObject()
	return m
}

// pinSidebarToFirstObject puts the tree cursor on the first filtered object.
func (m Model) pinSidebarToFirstObject() Model {
	rows := m.buildSidebarRows()
	for i, r := range rows {
		if r.kind == sbObject {
			m.SidebarCursor = i
			m.SelectedObjIdx = r.objIdx
			return m
		}
	}
	return m
}

// keysObjectSearch handles keystrokes while the object search box is active.
func (m Model) keysObjectSearch(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		return m.clearObjectSearch(), nil
	case "enter":
		m = m.commitObjectSearch()
		if len(m.filteredObjects()) > 0 {
			return m.openSelectedObject()
		}
		return m, nil
	case "down", "tab":
		// Leave the box and land on the first match (do NOT bind j — users type it).
		return m.commitObjectSearch(), nil
	case "ctrl+u":
		m.FilterInput.SetValue("")
		m.ObjectFilter = ""
		m.SelectedObjIdx = 0
		m = m.pinSidebarToFirstObject()
		return m, nil
	case "backspace":
		if m.FilterInput.Value() == "" {
			return m.clearObjectSearch(), nil
		}
	}

	// textinput only inserts Key.Text; fill it when the terminal only set Code.
	if msg.Text == "" && len([]rune(key)) == 1 && key != "\n" && key != "\t" {
		k := tea.Key(msg)
		k.Text = key
		msg = tea.KeyPressMsg(k)
	}
	var cmd tea.Cmd
	m.FilterInput, cmd = m.FilterInput.Update(msg)
	m.ObjectFilter = m.FilterInput.Value()
	m.SelectedObjIdx = 0
	m = m.pinSidebarToFirstObject()
	return m, cmd
}

func (m Model) keysBrowser(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	_ = msg
	switch key {
	case "q":
		return m, tea.Quit
	case "esc":
		// Clear applied filter first, then leave databases/schema, then disconnect.
		if m.objectSearchQuery() != "" {
			return m.clearObjectSearch(), nil
		}
		if m.ContentMode == contentDatabases || m.ContentMode == contentSchema {
			m.ContentMode = contentPreview
			m.Focus = focusSidebar
			m.StatusMsg = ""
			m.Err = nil
			return m, nil
		}
		if m.Cmds != nil {
			return m, m.Cmds.Disconnect()
		}
		m.Screen = types.ScreenConnections
		return m, nil
	case "tab", "shift+tab":
		m.Focus = cycleFocus(m.Focus, key == "shift+tab")
		return m, nil
	case ";", ":":
		// ; is primary open-query (SQL-ish); : kept as alias
		return m.openQuery()
	case "A":
		if m.Cmds != nil {
			m.Loading = true
			return m, m.Cmds.LoadActivity()
		}
	case "E":
		return m.openERD()
	case "i":
		if m.Cmds != nil {
			m.Loading = true
			return m, m.Cmds.LoadServerInfo()
		}
	case "L":
		m.PrevScreen = m.Screen
		m.Screen = types.ScreenLogs
		return m, nil
	case "r":
		if m.ContentMode == contentDatabases {
			if m.Cmds != nil {
				m.Loading = true
				return m, m.Cmds.LoadDatabases()
			}
			return m, nil
		}
		return m.refreshBrowser()
	case "D":
		o := m.copySelectedObject()
		if o.Name != "" {
			return m.beginTableDetail(o)
		}
	case "1", "2", "3", "4", "5", "6":
		items := objectKindNavItems()
		i := int(key[0] - '1')
		if i >= 0 && i < len(items) {
			return m.toggleKindFilter(items[i])
		}
	}

	if m.ContentMode == contentDatabases && m.Focus == focusContent {
		return m.keysDatabasesInContent(key)
	}

	return m.keysSidebar(key)
}

func (m Model) keysDatabasesInContent(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if len(m.Databases) > 0 {
			m.SelectedDBIdx = (m.SelectedDBIdx + 1) % len(m.Databases)
		}
	case "k", "up":
		if len(m.Databases) > 0 {
			m.SelectedDBIdx = (m.SelectedDBIdx - 1 + len(m.Databases)) % len(m.Databases)
		}
	case "g":
		m.SelectedDBIdx = 0
	case "G":
		if len(m.Databases) > 0 {
			m.SelectedDBIdx = len(m.Databases) - 1
		}
	case "r":
		if m.Cmds != nil {
			m.Loading = true
			return m, m.Cmds.LoadDatabases()
		}
	case "enter":
		if len(m.Databases) == 0 || m.Cmds == nil {
			return m, nil
		}
		db := m.Databases[clamp(m.SelectedDBIdx, 0, len(m.Databases)-1)]
		if db.Name == m.CurrentDatabase {
			m.ContentMode = contentPreview
			m.Focus = focusSidebar
			m.StatusMsg = m.CurrentDatabase
			return m, nil
		}
		m.Loading = true
		m.Err = nil
		return m, m.Cmds.SelectDatabase(db.Name)
	case "h", "left":
		m.Focus = focusSidebar
	}
	return m, nil
}

func (m Model) keysSidebar(key string) (tea.Model, tea.Cmd) {
	rows := m.buildSidebarRows()
	row := rows[clamp(m.SidebarCursor, 0, len(rows)-1)]
	switch key {
	case "j", "down":
		m.SidebarCursor = min(m.SidebarCursor+1, len(rows)-1)
		m = m.syncSelectionFromSidebar()
		return m.onSidebarCursorMoved()
	case "k", "up":
		m.SidebarCursor = max(m.SidebarCursor-1, 0)
		m = m.syncSelectionFromSidebar()
		return m.onSidebarCursorMoved()
	case "g":
		m.SidebarCursor = 0
		m = m.syncSelectionFromSidebar()
		return m.onSidebarCursorMoved()
	case "G":
		m.SidebarCursor = len(rows) - 1
		m = m.syncSelectionFromSidebar()
		return m.onSidebarCursorMoved()
	case " ", "enter", "l", "right":
		if row.kind == sbKind {
			return m.toggleKindFilter(row.nav)
		}
		return m.activateSidebarRow(row)
	case "D":
		if row.kind == sbObject {
			m = m.syncSelectionFromSidebar()
			o := m.copySelectedObject()
			if o.Name != "" {
				return m.beginTableDetail(o)
			}
		}
	}
	return m, nil
}

// toggleKindFilter flips a kind checkbox and reloads the object list.
func (m Model) toggleKindFilter(nav NavSection) (tea.Model, tea.Cmd) {
	if m.KindEnabled == nil {
		m.KindEnabled = defaultKindFilters()
	}
	m.KindEnabled[nav] = !m.KindEnabled[nav]
	// Keep at least one kind enabled.
	if len(m.enabledObjectKinds()) == 0 {
		m.KindEnabled[nav] = true
		m.StatusMsg = "At least one filter must stay on"
		return m, nil
	}
	m.NavSection = nav
	m.SelectedObjIdx = 0
	if m.ContentMode != contentSchema {
		m.ContentMode = contentPreview
	}
	m.StatusMsg = ""
	return m.loadObjectsForNav()
}

// onSidebarCursorMoved updates preview for objects; schema/kind apply on enter
// only (auto-load on hover re-ordered rows and flung the cursor downward).
func (m Model) onSidebarCursorMoved() (tea.Model, tea.Cmd) {
	rows := m.buildSidebarRows()
	row := rows[clamp(m.SidebarCursor, 0, len(rows)-1)]
	if row.kind == sbObject {
		return m.afterObjectCursorMove()
	}
	return m, nil
}

func (m Model) syncSelectionFromSidebar() Model {
	rows := m.buildSidebarRows()
	row := rows[clamp(m.SidebarCursor, 0, len(rows)-1)]
	if row.kind == sbObject {
		m.SelectedObjIdx = row.objIdx
	}
	return m
}

func (m Model) openSelectedObject() (tea.Model, tea.Cmd) {
	o := m.copySelectedObject()
	if o.Name == "" {
		return m, nil
	}
	m.Err = nil
	m = m.setCurrentObject(o)
	// Tables/views open data grid; everything else opens kind-specific detail.
	if isRelationObject(o) {
		return m.beginTableData(o, 0, m.PageSize)
	}
	return m.beginObjectDetail(o)
}

// isRelationObject is true for kinds that support data grid + structure tabs.
func isRelationObject(o types.SchemaObject) bool {
	switch o.Kind {
	case types.ObjectTable, types.ObjectView, types.ObjectMatView, "":
		return true
	default:
		return false
	}
}

// showObjectPreview keeps the tree focused and shows the lightweight preview pane.
func (m Model) showObjectPreview(o types.SchemaObject) Model {
	m = m.setCurrentObject(o)
	m.Screen = types.ScreenBrowser
	m.ContentMode = contentPreview
	m.Focus = focusSidebar
	m.TableDetail = types.TableDetail{}
	m.TableData = types.QueryResult{}
	m.contentSeq++
	m.Loading = false
	m.Err = nil
	if o.Kind != "" {
		m.StatusMsg = string(o.Kind) + " · " + o.FullName()
	}
	return m
}

// beginObjectDetail loads sequence/function/type/extension metadata into Structure.
func (m Model) beginObjectDetail(o types.SchemaObject) (tea.Model, tea.Cmd) {
	if m.Cmds == nil || o.Name == "" {
		return m, nil
	}
	m.Err = nil
	m = m.setCurrentObject(o)
	m.contentSeq++
	seq := m.contentSeq
	m.Loading = true
	m.DetailTab = 0
	m.Focus = focusContent
	base := m.Cmds.LoadObjectDetail(o.Schema, o.Name, o.Kind)
	return m, func() tea.Msg {
		d := base().(types.TableDetailLoadedMsg)
		return sequencedDetailMsg{TableDetailLoadedMsg: d, seq: seq}
	}
}

func (m Model) activateSidebarRow(row sidebarRow) (tea.Model, tea.Cmd) {
	switch row.kind {
	case sbKind:
		return m.toggleKindFilter(row.nav)
	case sbTool:
		switch row.nav {
		case navQuery:
			return m.openQuery()
		case navActivity:
			m = m.clearObjectContent()
			m.ContentMode = contentPreview
			if m.Cmds != nil {
				m.Loading = true
				return m, m.Cmds.LoadActivity()
			}
		case navERD:
			return m.openERD()
		case navServer:
			m = m.clearObjectContent()
			m.ContentMode = contentPreview
			if m.Cmds != nil {
				m.Loading = true
				return m, m.Cmds.LoadServerInfo()
			}
		case navDatabases:
			return m.openDatabasesContent()
		}
	case sbSchema:
		if row.schema >= 0 && row.schema < len(m.Schemas) {
			m.SelectedSchema = row.schema
			m.CurrentSchema = m.Schemas[row.schema].Name
			m.SelectedObjIdx = 0
			m.ContentMode = contentSchema
			return m.loadObjectsForNav()
		}
	case sbObject:
		m.SelectedObjIdx = row.objIdx
		return m.openSelectedObject()
	case sbNav:
		m.NavSection = row.nav
		m.ContentMode = contentPreview
		return m.loadObjectsForNav()
	}
	return m, nil
}

func (m Model) openDatabasesContent() (tea.Model, tea.Cmd) {
	m = m.clearObjectContent()
	m.Screen = types.ScreenBrowser
	m.ContentMode = contentDatabases
	m.Focus = focusContent
	m.StatusMsg = "Switch database"
	if m.Cmds != nil {
		m.Loading = true
		return m, m.Cmds.LoadDatabases()
	}
	return m, nil
}

func (m Model) openERD() (tea.Model, tea.Cmd) {
	m = m.clearObjectContent()
	m.ContentMode = contentPreview
	m.ERDOffset = 0
	// Prefer neighborhood of the selected sidebar table when available.
	m.ERDFocusAll = false
	schema := m.CurrentSchema
	if schema == "" {
		schema = "public"
	}
	if m.Cmds == nil {
		m.Screen = types.ScreenERD
		m.Focus = focusContent
		return m, nil
	}
	m.Loading = true
	return m, m.Cmds.LoadERD(schema)
}

func (m Model) keysERD(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "tab":
		m.Focus = cycleFocus(m.Focus, false)
		return m, nil
	case "shift+tab":
		m.Focus = cycleFocus(m.Focus, true)
		return m, nil
	case "esc":
		m.Screen = types.ScreenBrowser
		m.Focus = focusSidebar
		m.ContentMode = contentPreview
		return m, nil
	}

	if m.Focus == focusSidebar || m.Focus == focusMain {
		return m.keysSidebar(key)
	}

	switch key {
	case "j", "down":
		m.ERDOffset++
	case "k", "up":
		m.ERDOffset = max(m.ERDOffset-1, 0)
	case "g":
		m.ERDOffset = 0
	case "r":
		return m.openERD()
	case "a":
		m.ERDFocusAll = true
		m.ERDOffset = 0
		m.StatusMsg = "ERD: all tables"
	case "f":
		if m.erdFocusCandidate() == "" {
			m.StatusMsg = "Select a table to focus"
			return m, nil
		}
		m.ERDFocusAll = false
		m.ERDOffset = 0
		m.StatusMsg = "ERD: focus " + m.erdFocusCandidate()
	}
	return m, nil
}

// loadObjectsForNav lists objects for the active nav/schema, preserving schema overview mode.
func (m Model) loadObjectsForNav() (tea.Model, tea.Cmd) {
	keepSchema := m.ContentMode == contentSchema
	m = m.clearObjectContent()
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	if keepSchema {
		m.ContentMode = contentSchema
	} else {
		m.ContentMode = contentPreview
	}
	if m.Cmds == nil {
		return m, nil
	}
	kinds := m.enabledObjectKinds()
	schema := m.CurrentSchema
	onlyExt := len(kinds) == 1 && len(kinds) > 0 && kinds[0] == types.ObjectExtension
	if onlyExt {
		schema = ""
	}
	m.Loading = true
	return m, m.Cmds.LoadObjectKinds(schema, kinds)
}

func (m Model) refreshBrowser() (tea.Model, tea.Cmd) {
	if m.Cmds == nil {
		return m, nil
	}
	m = m.resetToBrowserList()
	m.Loading = true
	return m, tea.Batch(m.Cmds.LoadSchemas(), m.Cmds.LoadObjectKinds(m.CurrentSchema, m.enabledObjectKinds()))
}

func (m Model) applyTableDetail(msg types.TableDetailLoadedMsg) (tea.Model, tea.Cmd) {
	m.Loading = false
	if msg.Err != nil {
		m.Err = msg.Err
		return m, nil
	}
	if m.CurrentObject != nil &&
		(m.CurrentObject.Schema != msg.Detail.Object.Schema || m.CurrentObject.Name != msg.Detail.Object.Name) {
		return m, nil
	}
	m.TableDetail = msg.Detail
	m.cacheDetailColumns(msg.Detail)
	m.rebuildSQLCompleter()
	if msg.Detail.Object.Name != "" {
		o := msg.Detail.Object
		if o.Schema == "" {
			o.Schema = m.CurrentSchema
		}
		m = m.setCurrentObject(o)
	}
	m.Screen = types.ScreenTableDetail
	m.DetailTab = 0
	m.Focus = focusContent
	if len(msg.Detail.Columns) == 0 && len(msg.Detail.Props) == 0 {
		name := msg.Detail.Object.FullName()
		if name == "" || name == "." {
			name = "table"
		}
		m.StatusMsg = fmt.Sprintf("no columns found for %s", name)
	}
	return m, nil
}

func (m Model) applyTableData(msg types.TableDataLoadedMsg) (tea.Model, tea.Cmd) {
	m.Loading = false
	if msg.Err != nil {
		m.Err = msg.Err
		return m, nil
	}
	// Cache column names from grid for ultra-fast SQL autocomplete.
	if m.CurrentObject != nil && len(msg.Result.Columns) > 0 {
		if m.SchemaCols == nil {
			m.SchemaCols = map[string][]string{}
		}
		cols := append([]string(nil), msg.Result.Columns...)
		m.SchemaCols[m.CurrentObject.FullName()] = cols
		m.SchemaCols[m.CurrentObject.Name] = cols
		m.rebuildSQLCompleter()
	}
	offset := max(msg.Offset, 0)
	if len(msg.Result.Rows) == 0 && offset > 0 && m.CurrentObject != nil {
		prev := max(offset-m.PageSize, 0)
		if prev != offset {
			return m.beginTableData(*m.CurrentObject, prev, m.PageSize)
		}
	}
	m.TableData = msg.Result
	m.DataOffset = offset
	m.DataCursor = clampRowCursor(0, len(msg.Result.Rows))
	m.DataCol = 0
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	return m, nil
}

func (m Model) tableDataHasMore() bool {
	if m.TableData.Truncated {
		return true
	}
	return m.PageSize > 0 && len(m.TableData.Rows) == m.PageSize
}

func (m Model) beginTableDetail(o types.SchemaObject) (Model, tea.Cmd) {
	if m.Cmds == nil || o.Name == "" {
		return m, nil
	}
	// Non-relations use the same Structure screen with kind-specific props/SQL.
	if !isRelationObject(o) {
		nm, cmd := m.beginObjectDetail(o)
		return nm.(Model), cmd
	}
	m.Err = nil
	m = m.setCurrentObject(o)
	m.contentSeq++
	seq := m.contentSeq
	m.Loading = true
	m.DetailTab = 0
	base := m.Cmds.LoadObjectDetail(o.Schema, o.Name, o.Kind)
	return m, func() tea.Msg {
		d := base().(types.TableDetailLoadedMsg)
		return sequencedDetailMsg{TableDetailLoadedMsg: d, seq: seq}
	}
}

func (m Model) beginTableData(o types.SchemaObject, offset, limit int) (Model, tea.Cmd) {
	if m.Cmds == nil || o.Name == "" {
		return m, nil
	}
	if !isRelationObject(o) {
		return m.showObjectPreview(o), nil
	}
	m.Err = nil
	m = m.setCurrentObject(o)
	m.contentSeq++
	seq := m.contentSeq
	m.Loading = true
	base := m.Cmds.LoadTableData(o.Schema, o.Name, offset, limit)
	return m, func() tea.Msg {
		d := base().(types.TableDataLoadedMsg)
		return sequencedDataMsg{TableDataLoadedMsg: d, seq: seq}
	}
}

func (m Model) keysObjectList(key string) (tea.Model, tea.Cmd) {
	objs := m.filteredObjects()
	switch key {
	case "j", "down":
		if len(objs) == 0 {
			return m, nil
		}
		next := min(m.SelectedObjIdx+1, len(objs)-1)
		if next == m.SelectedObjIdx {
			return m, nil
		}
		m.SelectedObjIdx = next
		return m.afterObjectCursorMove()
	case "k", "up":
		if len(objs) == 0 {
			return m, nil
		}
		next := max(m.SelectedObjIdx-1, 0)
		if next == m.SelectedObjIdx {
			return m, nil
		}
		m.SelectedObjIdx = next
		return m.afterObjectCursorMove()
	case "g":
		if m.SelectedObjIdx == 0 {
			return m, nil
		}
		m.SelectedObjIdx = 0
		return m.afterObjectCursorMove()
	case "G":
		if len(objs) == 0 {
			return m, nil
		}
		next := len(objs) - 1
		if next == m.SelectedObjIdx {
			return m, nil
		}
		m.SelectedObjIdx = next
		return m.afterObjectCursorMove()
	case "h", "left":
		m.Focus = focusSidebar
	case "enter", "l", "right":
		o := m.copySelectedObject()
		if o.Name == "" {
			return m, nil
		}
		if o.Kind == types.ObjectTable || o.Kind == types.ObjectView || o.Kind == types.ObjectMatView {
			m.DataOffset = 0
			return m.beginTableData(o, 0, m.PageSize)
		}
		return m.beginTableDetail(o)
	case "D":
		o := m.copySelectedObject()
		if o.Name == "" {
			return m, nil
		}
		return m.beginTableDetail(o)
	}
	return m, nil
}

func (m Model) copySelectedObject() types.SchemaObject {
	o, ok := m.selectedObject()
	if !ok {
		return types.SchemaObject{}
	}
	if o.Schema == "" {
		o.Schema = m.CurrentSchema
	}
	return o
}

func (m Model) afterObjectCursorMove() (tea.Model, tea.Cmd) {
	o := m.copySelectedObject()
	if o.Name == "" {
		return m, nil
	}
	m.Err = nil // drop stale "relation not found" when moving off a bad row
	m = m.setCurrentObject(o)

	// Browser/preview: only update CurrentObject — never auto-open.
	if m.Screen == types.ScreenBrowser || m.Screen == types.ScreenQuery ||
		m.Screen == types.ScreenActivity || m.Screen == types.ScreenERD ||
		m.Screen == types.ScreenServerInfo {
		return m, nil
	}

	// Structure / data screens: reload for relations; kind detail for others.
	switch m.Screen {
	case types.ScreenTableDetail:
		return m.beginTableDetail(o)
	case types.ScreenTableData:
		if !isRelationObject(o) {
			return m.beginObjectDetail(o)
		}
		return m.beginTableData(o, 0, m.PageSize)
	default:
		return m, nil
	}
}

func (m Model) openQuery() (tea.Model, tea.Cmd) {
	m.Screen = types.ScreenQuery
	m.QueryFocus = "editor"
	m.Focus = focusContent
	m.rebuildSQLCompleter()
	prefill := ""
	if m.CurrentObject != nil && isRelationObject(*m.CurrentObject) {
		prefill = fmt.Sprintf("SELECT * FROM %s.%s LIMIT 100;\n",
			m.CurrentObject.Schema, m.CurrentObject.Name)
	}
	editorW := max(m.Width/2-6, 40)
	if m.QueryArea == nil {
		ta := textarea.New()
		ta.Placeholder = "SELECT col FROM schema.table WHERE … LIMIT 100;"
		ta.SetWidth(editorW)
		ta.SetHeight(8)
		ta.CharLimit = 0
		ta.ShowLineNumbers = true
		ta.Focus()
		if prefill != "" {
			ta.SetValue(prefill)
		}
		m.QueryArea = &ta
	} else {
		if prefill != "" && strings.TrimSpace(m.QueryArea.Value()) == "" {
			m.QueryArea.SetValue(prefill)
		}
		m.QueryArea.SetWidth(editorW)
		m.QueryArea.Focus()
	}
	m.refreshQuerySuggestions()
	// Auto-run prefilled object queries so the results pane is useful immediately.
	if prefill != "" && m.Cmds != nil {
		m.Loading = true
		m.pushQueryHistory(prefill)
		return m, m.Cmds.RunQuery(prefill, m.PageSize)
	}
	return m, nil
}

func (m Model) keysTableData(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "tab":
		m.Focus = cycleFocus(m.Focus, false)
		return m, nil
	case "shift+tab":
		m.Focus = cycleFocus(m.Focus, true)
		return m, nil
	case "esc":
		m.Screen = types.ScreenBrowser
		m.Focus = focusMain
		return m, nil
	}

	if m.Focus == focusSidebar || m.Focus == focusMain {
		return m.keysSidebar(key)
	}

	nCols := len(m.TableData.Columns)
	nRows := len(m.TableData.Rows)
	switch key {
	case "j", "down":
		m.DataCursor = clampRowCursor(m.DataCursor+1, nRows)
	case "k", "up":
		m.DataCursor = clampRowCursor(m.DataCursor-1, nRows)
	case "h", "left":
		m.DataCol = max(m.DataCol-1, 0)
	case "l", "right":
		if nCols > 0 {
			m.DataCol = min(m.DataCol+1, nCols-1)
		}
	case "g":
		m.DataCursor = clampRowCursor(0, nRows)
	case "G":
		m.DataCursor = clampRowCursor(nRows-1, nRows)
	case "0":
		m.DataCol = 0
	case "$":
		if nCols > 0 {
			m.DataCol = nCols - 1
		}
	case "n":
		if m.CurrentObject != nil && m.tableDataHasMore() {
			return m.beginTableData(*m.CurrentObject, m.DataOffset+m.PageSize, m.PageSize)
		}
	case "p":
		if m.CurrentObject != nil && m.DataOffset > 0 {
			off := max(m.DataOffset-m.PageSize, 0)
			return m.beginTableData(*m.CurrentObject, off, m.PageSize)
		}
	case "D":
		o := m.copySelectedObject()
		if o.Name == "" && m.CurrentObject != nil {
			o = *m.CurrentObject
		}
		if o.Name != "" {
			return m.beginTableDetail(o)
		}
	case "x":
		m.PrevScreen = types.ScreenTableData
		m.Screen = types.ScreenExport
		if m.Inputs != nil {
			m.Inputs.ExportInput.SetValue("/tmp/postgres-tui-export.csv")
			m.Inputs.ExportInput.Focus()
		}
	case "y":
		if m.Cmds != nil && len(m.TableData.Rows) > 0 {
			row := m.TableData.Rows[clamp(m.DataCursor, 0, len(m.TableData.Rows)-1)]
			col := clamp(m.DataCol, 0, max(len(row)-1, 0))
			cell := ""
			if col < len(row) {
				cell = row[col]
			}
			return m, m.Cmds.CopyToClipboard(cell)
		}
	case "Y":
		if m.Cmds != nil && len(m.TableData.Rows) > 0 {
			row := m.TableData.Rows[clamp(m.DataCursor, 0, len(m.TableData.Rows)-1)]
			return m, m.Cmds.CopyToClipboard(strings.Join(row, "\t"))
		}
	case "r":
		if m.CurrentObject != nil {
			return m.beginTableData(*m.CurrentObject, m.DataOffset, m.PageSize)
		}
	case "E":
		return m.openERD()
	case ";", ":":
		return m.openQuery()
	}
	return m, nil
}

func (m Model) keysTableDetail(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "tab":
		m.Focus = cycleFocus(m.Focus, false)
		return m, nil
	case "shift+tab":
		m.Focus = cycleFocus(m.Focus, true)
		return m, nil
	case "esc":
		m.Screen = types.ScreenBrowser
		m.Focus = focusMain
		return m, nil
	}

	if m.Focus == focusSidebar || m.Focus == focusMain {
		return m.keysSidebar(key)
	}

	nTabs := max(len(m.detailTabs(m.TableDetail))-1, 0)
	switch key {
	case "h", "left":
		m.DetailTab = max(m.DetailTab-1, 0)
	case "l", "right":
		m.DetailTab = min(m.DetailTab+1, nTabs)
	case "1":
		m.DetailTab = 0
	case "2":
		m.DetailTab = min(1, nTabs)
	case "3":
		m.DetailTab = min(2, nTabs)
	case "4":
		m.DetailTab = min(3, nTabs)
	case "enter":
		o := m.copySelectedObject()
		if o.Name == "" && m.CurrentObject != nil {
			o = *m.CurrentObject
		}
		if o.Name != "" && isRelationObject(o) {
			return m.beginTableData(o, 0, m.PageSize)
		}
	case "D":
		o := m.copySelectedObject()
		if o.Name == "" && m.CurrentObject != nil {
			o = *m.CurrentObject
		}
		if o.Name != "" {
			return m.beginTableDetail(o)
		}
	case "E":
		return m.openERD()
	case ";", ":":
		return m.openQuery()
	}
	return m, nil
}

func (m Model) keysQuery(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Bubble Tea / terminal variants for "run query".
	runQuery := key == "ctrl+enter" || key == "ctrl+j" || key == "ctrl+r" ||
		key == "f5" || key == "ctrl+m" || key == "alt+enter" ||
		key == "ctrl+e" ||
		strings.EqualFold(key, "ctrl+return")
	if !runQuery {
		// Kitty/VHS sometimes encode as Mod+Code rather than a stable String().
		if msg.String() == "ctrl+enter" || msg.String() == "ctrl+j" {
			runQuery = true
		}
	}

	switch {
	case key == "esc":
		if m.Focus == focusContent && m.QueryFocus == "editor" && len(m.QuerySuggests) > 0 {
			m.QuerySuggests = nil
			m.QuerySuggestIdx = 0
			return m, nil
		}
		m.Screen = types.ScreenBrowser
		m.Focus = focusMain
		if m.QueryArea != nil {
			m.QueryArea.Blur()
		}
		return m, nil
	case key == "tab":
		// Accept autocomplete first when suggestions are open in the editor.
		if m.Focus == focusContent && m.QueryFocus == "editor" && len(m.QuerySuggests) > 0 {
			m.acceptQuerySuggestion()
			return m, nil
		}
		// Within query content: editor ↔ results; from outer panes land on content.
		if m.Focus != focusContent {
			m.Focus = focusContent
			m.QueryFocus = "editor"
			if m.QueryArea != nil {
				m.QueryArea.Focus()
			}
			return m, nil
		}
		if m.QueryFocus == "editor" {
			m.QueryFocus = "results"
			if m.QueryArea != nil {
				m.QueryArea.Blur()
			}
			m.QuerySuggests = nil
		} else {
			m.QueryFocus = "editor"
			if m.QueryArea != nil {
				m.QueryArea.Focus()
			}
			m.refreshQuerySuggestions()
		}
		return m, nil
	case key == "shift+tab":
		if m.Focus == focusContent && m.QueryFocus == "results" {
			m.QueryFocus = "editor"
			if m.QueryArea != nil {
				m.QueryArea.Focus()
			}
			return m, nil
		}
		if m.Focus == focusContent {
			m.Focus = focusMain
			if m.QueryArea != nil {
				m.QueryArea.Blur()
			}
			return m, nil
		}
		if m.Focus == focusMain {
			m.Focus = focusSidebar
			return m, nil
		}
		m.Focus = focusContent
		return m, nil
	case runQuery:
		if m.QueryArea == nil || m.Cmds == nil {
			return m, nil
		}
		sql := m.QueryArea.Value()
		m.pushQueryHistory(sql)
		m.Loading = true
		m.Err = nil
		m.StatusMsg = "Running…"
		return m, m.Cmds.RunQuery(sql, m.PageSize)
	case key == "x":
		if len(m.QueryResult.Rows) > 0 {
			m.PrevScreen = types.ScreenQuery
			m.Screen = types.ScreenExport
			if m.Inputs != nil {
				m.Inputs.ExportInput.SetValue("/tmp/postgres-tui-export.csv")
				m.Inputs.ExportInput.Focus()
			}
		}
		return m, nil
	}

	if m.Focus == focusSidebar || m.Focus == focusMain {
		return m.keysSidebar(key)
	}

	if m.QueryFocus == "results" {
		nCols := len(m.QueryResult.Columns)
		nRows := len(m.QueryResult.Rows)
		switch key {
		case "j", "down":
			m.DataCursor = clampRowCursor(m.DataCursor+1, nRows)
		case "k", "up":
			m.DataCursor = clampRowCursor(m.DataCursor-1, nRows)
		case "h", "left":
			m.DataCol = max(m.DataCol-1, 0)
		case "l", "right":
			if nCols > 0 {
				m.DataCol = min(m.DataCol+1, nCols-1)
			}
		case "y":
			if m.Cmds != nil && len(m.QueryResult.Rows) > 0 {
				row := m.QueryResult.Rows[clamp(m.DataCursor, 0, len(m.QueryResult.Rows)-1)]
				col := clamp(m.DataCol, 0, max(len(row)-1, 0))
				cell := ""
				if col < len(row) {
					cell = row[col]
				}
				return m, m.Cmds.CopyToClipboard(cell)
			}
		case "Y":
			if m.Cmds != nil && len(m.QueryResult.Rows) > 0 {
				row := m.QueryResult.Rows[clamp(m.DataCursor, 0, len(m.QueryResult.Rows)-1)]
				return m, m.Cmds.CopyToClipboard(strings.Join(row, "\t"))
			}
		}
		return m, nil
	}

	if m.QueryArea != nil && m.QueryFocus == "editor" {
		// Cycle suggestions without leaving the editor.
		if len(m.QuerySuggests) > 0 {
			switch key {
			case "down", "ctrl+n":
				m.QuerySuggestIdx = (m.QuerySuggestIdx + 1) % len(m.QuerySuggests)
				return m, nil
			case "up", "ctrl+p":
				m.QuerySuggestIdx = (m.QuerySuggestIdx - 1 + len(m.QuerySuggests)) % len(m.QuerySuggests)
				return m, nil
			case "enter":
				// Plain enter inserts newline via textarea; ctrl+enter runs (above).
			}
		}
		// ctrl+space forces suggestion refresh / show catalog help
		if key == "ctrl+@" || key == "ctrl+ " || key == "alt+/" || key == "ctrl+space" {
			m.refreshQuerySuggestions()
			return m, nil
		}
		var cmd tea.Cmd
		*m.QueryArea, cmd = m.QueryArea.Update(msg)
		m.refreshQuerySuggestions()
		return m, cmd
	}
	return m, nil
}

func (m Model) keysActivity(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "tab":
		m.Focus = cycleFocus(m.Focus, false)
		return m, nil
	case "shift+tab":
		m.Focus = cycleFocus(m.Focus, true)
		return m, nil
	case "esc":
		m.Screen = types.ScreenBrowser
		m.Focus = focusMain
		return m, nil
	}

	if m.Focus == focusSidebar || m.Focus == focusMain {
		return m.keysSidebar(key)
	}

	switch key {
	case "j", "down":
		if len(m.Activity) > 0 {
			m.SelectedActIdx = min(m.SelectedActIdx+1, len(m.Activity)-1)
		}
	case "k", "up":
		m.SelectedActIdx = max(m.SelectedActIdx-1, 0)
	case "r":
		if m.Cmds != nil {
			m.Loading = true
			return m, m.Cmds.LoadActivity()
		}
	}
	return m, nil
}

func (m Model) keysServerInfo(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "tab":
		m.Focus = cycleFocus(m.Focus, false)
		return m, nil
	case "shift+tab":
		m.Focus = cycleFocus(m.Focus, true)
		return m, nil
	case "esc":
		m.Screen = types.ScreenBrowser
		m.Focus = focusMain
		return m, nil
	}

	if m.Focus == focusSidebar || m.Focus == focusMain {
		return m.keysSidebar(key)
	}

	switch key {
	case "r":
		if m.Cmds != nil {
			m.Loading = true
			return m, m.Cmds.LoadServerInfo()
		}
	}
	return m, nil
}

func (m Model) keysConfirm(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "n", "esc":
		m.Screen = types.ScreenConnections
		return m, nil
	case "y", "enter":
		if m.ConfirmType == "connection" {
			if conn, ok := m.ConfirmData.(types.Connection); ok && m.Cmds != nil {
				m.Loading = true
				return m, m.Cmds.DeleteConnection(conn.ID)
			}
		}
		m.Screen = types.ScreenConnections
	}
	return m, nil
}

func (m Model) keysExport(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.Screen = m.PrevScreen
		if m.Screen == 0 {
			m.Screen = types.ScreenBrowser
		}
		return m, nil
	case "enter":
		if m.Cmds == nil || m.Inputs == nil {
			return m, nil
		}
		path := strings.TrimSpace(m.Inputs.ExportInput.Value())
		res := m.TableData
		if m.PrevScreen == types.ScreenQuery {
			res = m.QueryResult
		}
		m.Loading = true
		return m, m.Cmds.ExportCSV(path, res)
	}
	if m.Inputs != nil {
		var cmd tea.Cmd
		m.Inputs.ExportInput, cmd = m.Inputs.ExportInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) keysPalette(key string, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	filter := ""
	if m.Inputs != nil {
		filter = strings.ToLower(m.Inputs.PaletteInput.Value())
	}
	var items []PaletteItem
	for _, it := range m.PaletteItems {
		if filter == "" || strings.Contains(strings.ToLower(it.Label), filter) {
			items = append(items, it)
		}
	}

	switch key {
	case "esc":
		m.Screen = m.PrevScreen
		return m, nil
	case "down", "ctrl+n":
		if len(items) > 0 {
			m.PaletteIdx = min(m.PaletteIdx+1, len(items)-1)
		}
		return m, nil
	case "up", "ctrl+p":
		m.PaletteIdx = max(m.PaletteIdx-1, 0)
		return m, nil
	case "enter":
		if len(items) == 0 {
			return m, nil
		}
		it := items[clamp(m.PaletteIdx, 0, len(items)-1)]
		m.Screen = m.PrevScreen
		return m.runPalette(it.ID)
	}
	if m.Inputs != nil {
		var cmd tea.Cmd
		m.Inputs.PaletteInput, cmd = m.Inputs.PaletteInput.Update(msg)
		m.PaletteIdx = 0
		return m, cmd
	}
	return m, nil
}

func (m Model) runPalette(id string) (tea.Model, tea.Cmd) {
	switch id {
	case "query", "activity", "erd", "server", "tables", "views", "databases":
		if m.CurrentConn == nil {
			m.StatusMsg = "Connect first"
			return m, nil
		}
	}
	switch id {
	case "query":
		return m.openQuery()
	case "activity":
		if m.Cmds != nil {
			m.Loading = true
			return m, m.Cmds.LoadActivity()
		}
	case "erd":
		return m.openERD()
	case "server":
		if m.Cmds != nil {
			m.Loading = true
			return m, m.Cmds.LoadServerInfo()
		}
	case "tables":
		m.Screen = types.ScreenBrowser
		m.NavSection = navTables
		m.ContentMode = contentPreview
		return m.loadObjectsForNav()
	case "views":
		m.Screen = types.ScreenBrowser
		m.NavSection = navViews
		m.ContentMode = contentPreview
		return m.loadObjectsForNav()
	case "databases":
		return m.openDatabasesContent()
	case "disconnect":
		if m.Cmds != nil {
			return m, m.Cmds.Disconnect()
		}
	case "help":
		m.PrevScreen = types.ScreenBrowser
		m.Screen = types.ScreenHelp
	case "logs":
		m.Screen = types.ScreenLogs
	}
	return m, nil
}

// silence unused import if textarea only used in openQuery
var _ = textinput.New
