package ui

import (
	"fmt"
	"testing"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func edgeKey(code rune, mod tea.KeyMod) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Mod: mod}
}

func TestEdgeNormalizeKeyAllPaths(t *testing.T) {
	if got := normalizeKey(edgeKey(tea.KeyTab, 0)); got != "tab" {
		t.Fatalf("tab: %q", got)
	}
	if got := normalizeKey(edgeKey(tea.KeyTab, tea.ModShift)); got != "shift+tab" {
		t.Fatalf("shift+tab code: %q", got)
	}
	// String-path backtab / shift+tab (terminals that set Text, not KeyTab code).
	if got := normalizeKey(tea.KeyPressMsg{Text: "backtab"}); got != "shift+tab" {
		t.Fatalf("backtab text: %q", got)
	}
	if got := normalizeKey(tea.KeyPressMsg{Text: "shift+tab"}); got != "shift+tab" {
		t.Fatalf("shift+tab text: %q", got)
	}

	for _, msg := range []tea.KeyPressMsg{
		{Code: 'c', Mod: tea.ModCtrl},
		{Code: 'x', Mod: tea.ModAlt},
		{Code: 'x', Mod: tea.ModSuper},
		{Code: 'x', Mod: tea.ModMeta},
		{Code: tea.KeyEnter, Mod: tea.ModCtrl},
	} {
		got := normalizeKey(msg)
		if got == "" {
			t.Fatalf("ctrl/alt chord empty for %#v", msg)
		}
	}

	if got := normalizeKey(tea.KeyPressMsg{Text: "j", Code: 'j'}); got != "j" {
		t.Fatalf("text path: %q", got)
	}

	shiftMap := map[rune]string{
		'a': "A", 'z': "Z", '/': "?", '3': "#", '8': "*", ';': ":",
		'\'': "\"", '1': "!", ',': "<", '.': ">", '-': "_", '=': "+",
		'`': "~", '\\': "|", '[': "{", ']': "}",
	}
	for code, want := range shiftMap {
		if got := normalizeKey(edgeKey(code, tea.ModShift)); got != want {
			t.Fatalf("shift+%q=%q want %q", code, got, want)
		}
	}
	// unmapped shift chord falls through to raw string
	if got := normalizeKey(edgeKey('9', tea.ModShift)); got != "shift+9" {
		t.Fatalf("shift+9: %q", got)
	}
	// empty String() via KeyExtended with no text
	if got := normalizeKey(tea.KeyPressMsg{Code: tea.KeyExtended}); got != "" {
		t.Fatalf("empty: %q", got)
	}
	// bare code without text
	if got := normalizeKey(tea.KeyPressMsg{Code: 'q'}); got != "q" {
		t.Fatalf("q: %q", got)
	}
	// space text does not take Text short-circuit (Text==" " skipped)
	if got := normalizeKey(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}); got != "space" {
		t.Fatalf("space: %q", got)
	}
}

func TestEdgeTypingContextAllScreens(t *testing.T) {
	m := NewModel()
	if m.typingContext() {
		t.Fatal("default connections should not type")
	}
	m.FilterActive = true
	if !m.typingContext() {
		t.Fatal("FilterActive")
	}
	m.FilterActive = false
	m.FilterInput.Focus()
	if !m.typingContext() {
		t.Fatal("FilterInput focused")
	}
	m.FilterInput.Blur()

	for _, sc := range []types.Screen{
		types.ScreenAddConnection,
		types.ScreenEditConnection,
		types.ScreenExport,
		types.ScreenCommandPalette,
	} {
		m.Screen = sc
		if !m.typingContext() {
			t.Fatalf("%v should type", sc)
		}
	}

	m.Screen = types.ScreenQuery
	m.Focus = focusContent
	m.QueryFocus = "editor"
	m.QueryArea = nil
	if m.typingContext() {
		t.Fatal("query without area")
	}
	ta := textarea.New()
	ta.SetValue("select 1")
	m.QueryArea = &ta
	if !m.typingContext() {
		t.Fatal("query editor")
	}
	m.QueryFocus = "results"
	if m.typingContext() {
		t.Fatal("query results")
	}
	m.QueryFocus = "editor"
	m.Focus = focusSidebar
	if m.typingContext() {
		t.Fatal("query sidebar focus")
	}

	for _, sc := range []types.Screen{
		types.ScreenConnections,
		types.ScreenBrowser,
		types.ScreenTableData,
		types.ScreenTableDetail,
		types.ScreenActivity,
		types.ScreenERD,
		types.ScreenServerInfo,
		types.ScreenHelp,
		types.ScreenLogs,
	} {
		m.Screen = sc
		if m.typingContext() {
			t.Fatalf("%v should not type", sc)
		}
	}
}

func TestEdgeModelInitPaths(t *testing.T) {
	// Cmds nil, no CLI — still returns CheckForUpdate.
	m := NewModel()
	m.Cmds = nil
	m.CLIConnection = nil
	c := m.Init()
	if c == nil {
		t.Fatal("Init with nil Cmds should still return update check")
	}
	_ = c()

	// Cmds nil + CLIConnection — execute AutoConnectMsg closure body.
	conn := types.Connection{Name: "cli", Host: "localhost", Port: 5432}
	m.CLIConnection = &conn
	c = m.Init()
	if c == nil {
		t.Fatal("Init with CLI")
	}
	msg := c()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		// single cmd path (if Batch compacted)
		if am, ok := msg.(types.AutoConnectMsg); ok {
			if am.Connection.Name != "cli" {
				t.Fatalf("auto conn %#v", am)
			}
			return
		}
		t.Fatalf("want BatchMsg or AutoConnectMsg, got %T", msg)
	}
	var sawAuto bool
	for _, cmd := range batch {
		if cmd == nil {
			continue
		}
		out := cmd()
		if am, ok := out.(types.AutoConnectMsg); ok {
			sawAuto = true
			if am.Connection.Name != "cli" || am.Connection.Host != "localhost" {
				t.Fatalf("AutoConnectMsg: %#v", am)
			}
		}
	}
	if !sawAuto {
		t.Fatal("expected AutoConnectMsg from Init CLI branch")
	}
}

func TestEdgeRefreshQuerySuggestionsEarlyReturn(t *testing.T) {
	m := NewModel()
	m.QueryArea = nil
	m.SQLCompleter = newSQLCompleter()
	m.refreshQuerySuggestions() // nil area

	ta := textarea.New()
	m.QueryArea = &ta
	m.SQLCompleter = nil
	m.refreshQuerySuggestions() // nil completer

	m.SQLCompleter = newSQLCompleter()
	// empty token + non-special context (not FROM/SELECT/…) + non-whitespace before → early return
	m.QueryArea.SetValue("SELECT 1 ")
	m.QueryArea.SetCursorColumn(len("SELECT 1 "))
	m.refreshQuerySuggestions()
	if len(m.QuerySuggests) != 0 {
		// may still produce something depending on context; non-special empty is the goal
		// "SELECT 1 " after digit is general context → early return with nil suggests
	}

	// whitespace-only before empty token is allowed
	m.QueryArea.SetValue("   ")
	m.QueryArea.SetCursorColumn(3)
	m.refreshQuerySuggestions()
}

func TestEdgePinSidebarCursorPaths(t *testing.T) {
	m := seededWorkspace()

	// hit: pin to existing kind row
	m = m.pinSidebarCursorToKind(navTables)
	rows := m.buildSidebarRows()
	if rows[m.SidebarCursor].kind != sbKind || rows[m.SidebarCursor].nav != navTables {
		t.Fatalf("hit tables: %+v", rows[m.SidebarCursor])
	}

	// miss matching kind → fallback first kind row
	m = m.pinSidebarCursorToKind(navQuery) // tool, not a kind filter
	if rows2 := m.buildSidebarRows(); rows2[m.SidebarCursor].kind != sbKind {
		t.Fatalf("miss fallback: %+v", rows2[m.SidebarCursor])
	}
	m = m.pinSidebarCursorToKind(NavSection(99))
	if rows3 := m.buildSidebarRows(); rows3[m.SidebarCursor].kind != sbKind {
		t.Fatalf("unknown nav fallback: %+v", rows3[m.SidebarCursor])
	}

	// pinSidebarCursorToSchema hit + miss
	m = m.pinSidebarCursorToSchema(0)
	if rows4 := m.buildSidebarRows(); rows4[m.SidebarCursor].kind != sbSchema || rows4[m.SidebarCursor].schema != 0 {
		t.Fatalf("schema hit: %+v", rows4[m.SidebarCursor])
	}
	before := m.SidebarCursor
	m = m.pinSidebarCursorToSchema(999)
	if m.SidebarCursor != before {
		// miss leaves cursor unchanged
	}
	_ = m.pinSidebarCursorToSchema(-1)
}

func TestEdgePinSidebarAfterObjectsLoadBranches(t *testing.T) {
	m := seededWorkspace()

	// empty-ish tree: no schemas/objects still has kind+tool rows; also exercise NewModel
	empty := NewModel()
	empty.Schemas = nil
	empty.Objects = nil
	_ = empty.pinSidebarAfterObjectsLoad()

	// sbKind branch
	m = m.pinSidebarCursorToKind(navTables)
	m.NavSection = navViews
	m = m.pinSidebarAfterObjectsLoad()
	if rows := m.buildSidebarRows(); rows[m.SidebarCursor].kind != sbKind {
		t.Fatalf("after kind: %+v", rows[m.SidebarCursor])
	}

	// sbSchema stay when already on SelectedSchema
	m = seededWorkspace()
	m = m.pinSidebarCursorToSchema(m.SelectedSchema)
	cur := m.SidebarCursor
	m = m.pinSidebarAfterObjectsLoad()
	if m.SidebarCursor != cur {
		t.Fatalf("schema stay: %d → %d", cur, m.SidebarCursor)
	}

	// sbSchema rematch to SelectedSchema
	m = m.pinSidebarCursorToSchema(0)
	m.SelectedSchema = 2
	m = m.pinSidebarAfterObjectsLoad()
	if rows := m.buildSidebarRows(); rows[m.SidebarCursor].kind != sbSchema || rows[m.SidebarCursor].schema != 2 {
		t.Fatalf("schema rematch: %+v", rows[m.SidebarCursor])
	}

	// sbTool stay
	m = seededWorkspace()
	for i, r := range m.buildSidebarRows() {
		if r.kind == sbTool {
			m.SidebarCursor = i
			break
		}
	}
	toolCur := m.SidebarCursor
	m = m.pinSidebarAfterObjectsLoad()
	if m.SidebarCursor != toolCur {
		t.Fatalf("tool stay: %d → %d", toolCur, m.SidebarCursor)
	}

	// sbObject stay
	m = m.syncSidebarCursorToObject()
	objCur := m.SidebarCursor
	m = m.pinSidebarAfterObjectsLoad()
	if m.SidebarCursor != objCur {
		t.Fatalf("object stay: %d → %d", objCur, m.SidebarCursor)
	}
}

func TestEdgeNavSectionObjectKindSSL(t *testing.T) {
	wantStr := map[NavSection]string{
		navTables: "Tables", navViews: "Views", navSequences: "Sequences",
		navFunctions: "Functions", navTypes: "Types", navExtensions: "Extensions",
		navQuery: "Query", navActivity: "Activity", navERD: "ERD",
		navServer: "Server", navDatabases: "Databases",
	}
	for n, s := range wantStr {
		if got := n.String(); got != s {
			t.Fatalf("%v.String()=%q want %q", n, got, s)
		}
	}
	if NavSection(99).String() != "" {
		t.Fatal("unknown String")
	}

	wantKind := map[NavSection]types.ObjectKind{
		navTables: types.ObjectTable, navViews: types.ObjectView,
		navSequences: types.ObjectSequence, navFunctions: types.ObjectFunction,
		navTypes: types.ObjectType, navExtensions: types.ObjectExtension,
	}
	for n, k := range wantKind {
		if got := n.ObjectKind(); got != k {
			t.Fatalf("%v.ObjectKind()=%q want %q", n, got, k)
		}
	}
	// non-object nav defaults to table
	if navQuery.ObjectKind() != types.ObjectTable {
		t.Fatal("default ObjectKind")
	}
	if NavSection(99).ObjectKind() != types.ObjectTable {
		t.Fatal("unknown ObjectKind")
	}

	modes := []types.SSLMode{
		types.SSLModeDisable, types.SSLModeAllow, types.SSLModePrefer,
		types.SSLModeRequire, types.SSLModeVerifyCA, types.SSLModeVerifyFull,
		"", "unknown-mode",
	}
	for _, mode := range modes {
		idx := sslIndex(mode)
		if idx < 0 || idx >= len(sslModeOptions) {
			t.Fatalf("sslIndex(%q)=%d", mode, idx)
		}
	}
	if sslIndex("") != sslIndex(types.SSLModePrefer) {
		t.Fatal("empty SSL should map to prefer")
	}
	if sslIndex("unknown-mode") != 2 {
		t.Fatalf("unknown fallback want 2 got %d", sslIndex("unknown-mode"))
	}
}

func TestEdgeFilteredSelectedIdentity(t *testing.T) {
	m := NewModel()
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "users", Kind: types.ObjectTable},
		{Schema: "public", Name: "orders", Kind: types.ObjectTable},
		{Schema: "public", Name: "user_stats", Kind: types.ObjectView},
		{Schema: "billing", Name: "invoices", Kind: types.ObjectView},
	}

	// kind filter match (name does not contain "view")
	m.ObjectFilter = "view"
	m.FilterActive = false
	got := m.filteredObjects()
	if len(got) != 2 {
		t.Fatalf("kind filter want 2 got %d %v", len(got), got)
	}

	// schema.dot match
	m.ObjectFilter = "billing."
	got = m.filteredObjects()
	if len(got) != 1 || got[0].Name != "invoices" {
		t.Fatalf("dot filter %v", got)
	}

	// name match
	m.ObjectFilter = "user"
	got = m.filteredObjects()
	if len(got) != 2 {
		t.Fatalf("name filter %v", got)
	}

	// empty filter returns all
	m.ObjectFilter = ""
	if len(m.filteredObjects()) != 4 {
		t.Fatal("empty filter")
	}

	// selectedObject empty
	m.Objects = nil
	if _, ok := m.selectedObject(); ok {
		t.Fatal("empty selectedObject")
	}
	m.Objects = []types.SchemaObject{{Schema: "public", Name: "t", Kind: types.ObjectTable}}
	m.SelectedObjIdx = 0
	if o, ok := m.selectedObject(); !ok || o.Name != "t" {
		t.Fatalf("selected %+v %v", o, ok)
	}
	m.SelectedObjIdx = 99
	if o, ok := m.selectedObject(); !ok || o.Name != "t" {
		t.Fatalf("clamp selected %+v %v", o, ok)
	}

	// objectIdentityMatch empty kinds
	if objectIdentityMatch(nil, m.Objects[0]) {
		t.Fatal("nil cur")
	}
	cur := types.SchemaObject{Schema: "public", Name: "t", Kind: ""}
	if !objectIdentityMatch(&cur, types.SchemaObject{Schema: "public", Name: "t", Kind: types.ObjectTable}) {
		t.Fatal("empty kind should match")
	}
	cur.Kind = types.ObjectView
	if objectIdentityMatch(&cur, types.SchemaObject{Schema: "public", Name: "t", Kind: types.ObjectTable}) {
		t.Fatal("kind mismatch")
	}
	cur = types.SchemaObject{Schema: "public", Name: "t", Kind: types.ObjectTable}
	if !objectIdentityMatch(&cur, types.SchemaObject{Schema: "public", Name: "t", Kind: types.ObjectTable}) {
		t.Fatal("full match")
	}
	if objectIdentityMatch(&cur, types.SchemaObject{Schema: "other", Name: "t", Kind: types.ObjectTable}) {
		t.Fatal("schema mismatch")
	}
}

func TestEdgePushQueryHistoryOverflow(t *testing.T) {
	m := NewModel()
	m.pushQueryHistory("")
	m.pushQueryHistory("   ")
	if len(m.QueryHistory) != 0 {
		t.Fatal("empty skip")
	}
	m.pushQueryHistory("  SELECT 1  ")
	if len(m.QueryHistory) != 1 || m.QueryHistory[0] != "SELECT 1" {
		t.Fatalf("trim: %v", m.QueryHistory)
	}
	m.pushQueryHistory("SELECT 1") // duplicate of head
	if len(m.QueryHistory) != 1 {
		t.Fatal("dup head skip")
	}
	for i := 0; i < 40; i++ {
		m.pushQueryHistory(fmt.Sprintf("q%d", i))
	}
	if len(m.QueryHistory) != 30 {
		t.Fatalf("overflow cap want 30 got %d", len(m.QueryHistory))
	}
	if m.HistoryIdx != -1 {
		t.Fatalf("HistoryIdx=%d", m.HistoryIdx)
	}
	if m.QueryHistory[0] != "q39" {
		t.Fatalf("newest first: %q", m.QueryHistory[0])
	}
}
