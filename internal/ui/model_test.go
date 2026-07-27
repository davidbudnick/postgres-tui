package ui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/cmd"
	"github.com/davidbudnick/postgres-tui/internal/db"
	"github.com/davidbudnick/postgres-tui/internal/testutil"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func testModel(t *testing.T) Model {
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

func TestNewModel(t *testing.T) {
	m := NewModel()
	if m.Screen != types.ScreenConnections {
		t.Fatal(m.Screen)
	}
	if len(m.ConnInputs) != connTextCount {
		t.Fatal(len(m.ConnInputs))
	}
}

func TestViewConnectionsRenders(t *testing.T) {
	m := testModel(t)
	m.Connections = []types.Connection{
		{ID: 1, Name: "local", Host: "localhost", Port: 5432, Username: "postgres"},
	}
	s := m.viewConnections()
	if s == "" {
		t.Fatal("empty view")
	}
	if !contains(s, "local") {
		t.Fatalf("missing connection name in %q", s)
	}
	if !contains(s, "Saved Instances") {
		t.Fatalf("missing section frame in %q", s)
	}
	full := m.render()
	if full == "" {
		t.Fatal("empty render")
	}
}

func TestViewConnectionsEmptyState(t *testing.T) {
	m := testModel(t)
	m.Connections = nil
	s := m.viewConnections()
	if !contains(s, "No instances saved") {
		t.Fatalf("missing empty state in %q", s)
	}
	if !contains(s, "Press a") {
		t.Fatalf("missing add hint in %q", s)
	}
}

func TestGetStatusBar_ConnectionsSuppressed(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenConnections
	m.Loading = true
	m.StatusMsg = "hello"
	if bar := m.getStatusBar(); bar != "" {
		t.Fatalf("expected empty status on connections, got %q", bar)
	}
}

func TestHandleAutoConnectClearsCLI(t *testing.T) {
	m := testModel(t)
	conn := types.Connection{Host: "localhost", Port: 5432, Name: "cli"}
	m.CLIConnection = &conn
	nm, _ := m.Update(types.AutoConnectMsg{Connection: conn})
	m = nm.(Model)
	if m.CLIConnection != nil {
		t.Fatal("CLIConnection should be consumed")
	}
	if !m.Loading {
		t.Fatal("expected loading during auto-connect")
	}
}

func TestUpdateConnectFlow(t *testing.T) {
	m := testModel(t)
	m.Connections = []types.Connection{{ID: 1, Name: "local", Host: "localhost", Port: 5432}}
	m.SelectedConnIdx = 0

	nm, c := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	// enter triggers connect cmd
	_ = c
	m = nm.(Model)
	// simulate ConnectedMsg — default DB opens workspace (skips databases hop)
	nm, c = m.Update(types.ConnectedMsg{Info: types.ServerInfo{Version: "PostgreSQL 16", Database: "postgres"}})
	m = nm.(Model)
	// SelectDatabase + LoadDatabases batch; screen may still be connections until selected
	if c == nil {
		t.Fatal("expected load/select cmds")
	}

	nm, _ = m.Update(types.DatabasesLoadedMsg{Databases: []types.DatabaseInfo{
		{Name: "postgres"}, {Name: "demo"},
	}})
	m = nm.(Model)
	if len(m.Databases) != 2 {
		t.Fatal(len(m.Databases))
	}

	nm, _ = m.Update(types.DatabaseSelectedMsg{
		Database: "demo",
		Info:     types.ServerInfo{Database: "demo", Version: "PostgreSQL 16"},
	})
	m = nm.(Model)
	if m.Screen != types.ScreenBrowser {
		t.Fatalf("screen=%v", m.Screen)
	}

	// views should not panic
	_ = m.viewBrowser()
	_ = m.viewDatabases()
	_ = m.viewHelp()
	_ = m.viewServerInfo()
	_ = m.viewActivity()
}

func TestNavSection(t *testing.T) {
	if navTables.String() != "Tables" {
		t.Fatal(navTables.String())
	}
	if navTables.ObjectKind() != types.ObjectTable {
		t.Fatal(navTables.ObjectKind())
	}
}

func TestWorkspaceShellRenders(t *testing.T) {
	m := testModel(t)
	m.CurrentConn = &types.Connection{Name: "local", Host: "localhost", Port: 5432}
	m.CurrentDatabase = "demo"
	m.CurrentSchema = "public"
	m.Schemas = []types.SchemaInfo{{Name: "public", TableCount: 2}}
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "users", Kind: types.ObjectTable, RowEstimate: 1500},
		{Schema: "public", Name: "orders", Kind: types.ObjectTable, RowEstimate: 40000},
	}
	m.Screen = types.ScreenBrowser
	m.Focus = focusMain

	s := m.viewWorkspace()
	if s == "" {
		t.Fatal("empty workspace")
	}
	if !contains(s, "users") || !contains(s, "orders") {
		t.Fatalf("missing objects in workspace: %q", s)
	}
	if !contains(s, "local") || !contains(s, "demo") {
		t.Fatalf("missing header conn/db: %q", s)
	}

	// Table data opens in the content pane of the same shell.
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m.CurrentObject = &m.Objects[0]
	m.TableData = types.QueryResult{
		Columns: []string{"id", "email", "created_at"},
		Rows: [][]string{
			{"1", "a@x.com", "2024-01-01"},
			{"2", "b@x.com", "2024-01-02"},
		},
	}
	m.DataCursor = 0
	m.DataCol = 1
	s = m.viewWorkspace()
	if !contains(s, "email") || !contains(s, "a@x.com") {
		t.Fatalf("missing grid content: %q", s)
	}
	if !contains(s, "users") {
		t.Fatalf("object list should stay visible: %q", s)
	}
}

func TestDenseResultTableCellCursor(t *testing.T) {
	m := testModel(t)
	res := types.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "alice"}, {"2", "bob"}},
	}
	out := m.renderResultTable(res, 1, 1, 10, 60)
	if !contains(out, "alice") || !contains(out, "bob") {
		t.Fatalf("missing rows: %q", out)
	}
	if !contains(out, "│") {
		t.Fatalf("expected cell separators: %q", out)
	}
}

func TestTableDataCellCopy(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m.TableData = types.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "alice"}, {"2", "bob"}},
	}
	m.DataCursor = 0
	m.DataCol = 1

	nm, cmd := m.keysTableData("l")
	m = nm.(Model)
	if m.DataCol != 1 {
		// already at col 1; move left then right
	}
	nm, _ = m.keysTableData("h")
	m = nm.(Model)
	if m.DataCol != 0 {
		t.Fatalf("DataCol=%d want 0", m.DataCol)
	}
	nm, _ = m.keysTableData("l")
	m = nm.(Model)
	if m.DataCol != 1 {
		t.Fatalf("DataCol=%d want 1", m.DataCol)
	}
	_, cmd = m.keysTableData("y")
	if cmd == nil {
		t.Fatal("expected copy command")
	}
}

func TestTableDataCursorBounds(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m.TableData = types.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}, {"2"}, {"3"}},
	}
	m.DataCursor = 0

	nm, _ := m.keysTableData("k")
	m = nm.(Model)
	if m.DataCursor != 0 {
		t.Fatalf("k at top: DataCursor=%d want 0", m.DataCursor)
	}

	for i := 0; i < 10; i++ {
		nm, _ = m.keysTableData("j")
		m = nm.(Model)
	}
	if m.DataCursor != 2 {
		t.Fatalf("j past end: DataCursor=%d want 2", m.DataCursor)
	}

	nm, _ = m.keysTableData("g")
	m = nm.(Model)
	if m.DataCursor != 0 {
		t.Fatalf("g: DataCursor=%d want 0", m.DataCursor)
	}
	nm, _ = m.keysTableData("G")
	m = nm.(Model)
	if m.DataCursor != 2 {
		t.Fatalf("G: DataCursor=%d want 2", m.DataCursor)
	}

	m.TableData.Rows = nil
	m.DataCursor = 5
	nm, _ = m.keysTableData("j")
	m = nm.(Model)
	if m.DataCursor != 0 {
		t.Fatalf("j empty: DataCursor=%d want 0", m.DataCursor)
	}
	nm, _ = m.keysTableData("k")
	m = nm.(Model)
	if m.DataCursor != 0 {
		t.Fatalf("k empty: DataCursor=%d want 0", m.DataCursor)
	}
	nm, _ = m.keysTableData("G")
	m = nm.(Model)
	if m.DataCursor != 0 {
		t.Fatalf("G empty: DataCursor=%d want 0", m.DataCursor)
	}
}

func TestTableDataPaginationBounds(t *testing.T) {
	obj := types.SchemaObject{Schema: "public", Name: "users"}
	m := testModel(t)
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m.CurrentObject = &obj
	m.PageSize = 2
	m.DataOffset = 0
	m.TableData = types.QueryResult{
		Columns:   []string{"id"},
		Rows:      [][]string{{"1"}},
		Truncated: false,
	}

	_, cmd := m.keysTableData("n")
	if cmd != nil {
		t.Fatal("n on partial last page should not load")
	}
	_, cmd = m.keysTableData("p")
	if cmd != nil {
		t.Fatal("p at offset 0 should not load")
	}

	m.TableData.Rows = [][]string{{"1"}, {"2"}}
	m.TableData.Truncated = true
	_, cmd = m.keysTableData("n")
	if cmd == nil {
		t.Fatal("n when Truncated should load next page")
	}

	m.DataOffset = 4
	m.TableData.Truncated = false
	m.TableData.Rows = [][]string{{"5"}}
	_, cmd = m.keysTableData("p")
	if cmd == nil {
		t.Fatal("p with DataOffset>0 should load previous page")
	}

	m.DataOffset = 0
	m.TableData.Rows = [][]string{{"1"}, {"2"}}
	m.TableData.Truncated = false
	_, cmd = m.keysTableData("n")
	if cmd == nil {
		t.Fatal("n on full page without Truncated should still attempt next")
	}
}

func TestApplyTableDataEmptyPageSnapBack(t *testing.T) {
	obj := types.SchemaObject{Schema: "public", Name: "users"}
	m := testModel(t)
	m.CurrentObject = &obj
	m.PageSize = 10
	m.DataOffset = 20

	nm, cmd := m.applyTableData(types.TableDataLoadedMsg{
		Result: types.QueryResult{Columns: []string{"id"}, Rows: nil},
		Offset: 30,
	})
	if cmd == nil {
		t.Fatal("expected snap-back reload command")
	}
	// Model stays at prior offset until the snap-back load completes.
	_ = nm
}

func TestApplyTableDataEmptyAtStart(t *testing.T) {
	obj := types.SchemaObject{Schema: "public", Name: "users"}
	m := testModel(t)
	m.CurrentObject = &obj
	m.PageSize = 10

	nm, cmd := m.applyTableData(types.TableDataLoadedMsg{
		Result: types.QueryResult{Columns: []string{"id"}, Rows: nil},
		Offset: 0,
	})
	m = nm.(Model)
	if cmd != nil {
		t.Fatal("empty first page should not snap back")
	}
	if m.DataOffset != 0 {
		t.Fatalf("DataOffset=%d want 0", m.DataOffset)
	}
	if m.DataCursor != 0 {
		t.Fatalf("DataCursor=%d want 0", m.DataCursor)
	}
}

func TestTableDataPageMetaHints(t *testing.T) {
	obj := types.SchemaObject{Schema: "public", Name: "users", RowEstimate: 100}
	m := testModel(t)
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	m.CurrentObject = &obj
	m.PageSize = 2
	m.DataOffset = 0
	m.TableData = types.QueryResult{
		Columns:   []string{"id"},
		Rows:      [][]string{{"1"}, {"2"}},
		Truncated: true,
	}

	s := m.viewTableDataContent(80, 20)
	if !contains(s, "first page") {
		t.Fatalf("expected first page hint: %q", s)
	}
	if !contains(s, "more") {
		t.Fatalf("expected more hint: %q", s)
	}

	m.DataOffset = 4
	m.TableData.Truncated = false
	m.TableData.Rows = [][]string{{"5"}}
	s = m.viewTableDataContent(80, 20)
	if !contains(s, "last page") {
		t.Fatalf("expected last page hint: %q", s)
	}
	if contains(s, "first page") {
		t.Fatalf("should not show first page on offset>0: %q", s)
	}
}

func TestQueryResultsCursorBounds(t *testing.T) {
	m := testModel(t)
	m.Screen = types.ScreenQuery
	m.Focus = focusContent
	m.QueryFocus = "results"
	m.QueryResult = types.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}, {"2"}},
	}
	m.DataCursor = 0

	msg := tea.KeyPressMsg{}
	nm, _ := m.keysQuery("k", msg)
	m = nm.(Model)
	if m.DataCursor != 0 {
		t.Fatalf("k at top: DataCursor=%d want 0", m.DataCursor)
	}
	for i := 0; i < 5; i++ {
		nm, _ = m.keysQuery("j", msg)
		m = nm.(Model)
	}
	if m.DataCursor != 1 {
		t.Fatalf("j past end: DataCursor=%d want 1", m.DataCursor)
	}

	m.QueryResult.Rows = nil
	m.DataCursor = 3
	nm, _ = m.keysQuery("j", msg)
	m = nm.(Model)
	if m.DataCursor != 0 {
		t.Fatalf("j empty results: DataCursor=%d want 0", m.DataCursor)
	}
}

func TestClampRowCursor(t *testing.T) {
	if got := clampRowCursor(5, 0); got != 0 {
		t.Fatalf("empty: %d", got)
	}
	if got := clampRowCursor(-1, 3); got != 0 {
		t.Fatalf("below: %d", got)
	}
	if got := clampRowCursor(10, 3); got != 2 {
		t.Fatalf("above: %d", got)
	}
	if got := clampRowCursor(1, 3); got != 1 {
		t.Fatalf("mid: %d", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
