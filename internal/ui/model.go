package ui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"github.com/davidbudnick/postgres-tui/internal/cmd"
	"github.com/davidbudnick/postgres-tui/internal/types"

	tea "charm.land/bubbletea/v2"
)

// Focus pane within the workspace layout.
type focusPane int

const (
	focusSidebar focusPane = iota
	focusMain
	focusContent
)

// ContentMode selects what ScreenBrowser shows in the content pane.
type ContentMode int

const (
	contentPreview ContentMode = iota
	contentSchema
	contentDatabases
)

// NavSection is a sidebar navigation section.
type NavSection int

const (
	navTables NavSection = iota
	navViews
	navSequences
	navFunctions
	navTypes
	navExtensions
	navQuery
	navActivity
	navERD
	navServer
	navDatabases
)

func (n NavSection) String() string {
	switch n {
	case navTables:
		return "Tables"
	case navViews:
		return "Views"
	case navSequences:
		return "Sequences"
	case navFunctions:
		return "Functions"
	case navTypes:
		return "Types"
	case navExtensions:
		return "Extensions"
	case navQuery:
		return "Query"
	case navActivity:
		return "Activity"
	case navERD:
		return "ERD"
	case navServer:
		return "Server"
	case navDatabases:
		return "Databases"
	default:
		return ""
	}
}

func (n NavSection) ObjectKind() types.ObjectKind {
	switch n {
	case navTables:
		return types.ObjectTable
	case navViews:
		return types.ObjectView
	case navSequences:
		return types.ObjectSequence
	case navFunctions:
		return types.ObjectFunction
	case navTypes:
		return types.ObjectType
	case navExtensions:
		return types.ObjectExtension
	default:
		return types.ObjectTable
	}
}

// Model is the Bubble Tea application model.
type Model struct {
	Cmds    *cmd.Commands
	Version string
	Screen  types.Screen

	Connections     []types.Connection
	SelectedConnIdx int
	ConnInputs      []textinput.Model
	ConnFocusIdx    int
	ConnReadOnly    bool
	ConnSSLIdx      int
	EditingConn     *types.Connection
	CurrentConn     *types.Connection
	ServerInfo      types.ServerInfo
	ReadOnly        bool

	Databases       []types.DatabaseInfo
	SelectedDBIdx   int
	CurrentDatabase string

	Schemas        []types.SchemaInfo
	SelectedSchema int
	CurrentSchema  string
	Objects        []types.SchemaObject
	SelectedObjIdx int
	ObjectFilter   string
	NavSection     NavSection // last-focused kind row (for pin/cursor)
	KindEnabled    map[NavSection]bool
	Focus          focusPane
	SidebarCursor  int
	ContentMode    ContentMode

	CurrentObject *types.SchemaObject
	TableDetail   types.TableDetail
	TableData     types.QueryResult
	DataOffset    int
	DataCursor    int
	DataCol       int
	PageSize      int
	DetailTab     int // 0 columns, 1 indexes, 2 constraints
	contentSeq    int

	QueryArea       *textarea.Model
	QueryResult     types.QueryResult
	QueryFocus      string // "editor" | "results"
	QueryHistory    []string
	HistoryIdx      int
	SQLCompleter    *sqlCompleter
	SchemaCols      map[string][]string // "schema.table" → column names
	QuerySuggests   []string
	QuerySuggestIdx int

	Activity       []types.ActivityRow
	SelectedActIdx int

	ERD       types.ERDGraph
	ERDOffset int

	Favorites      []types.Favorite
	SelectedFavIdx int

	Width           int
	Height          int
	Err             error
	StatusMsg       string
	Loading         bool
	ConfirmType     string
	ConfirmData     any
	Logs            *types.LogWriter
	SendFunc        *func(tea.Msg)
	ConnectionError string
	TestConnResult  string
	CLIConnection   *types.Connection
	KeyBindings     types.KeyBindings
	PrevScreen      types.Screen
	LogCursor       int
	FilterInput     textinput.Model
	FilterActive    bool
	ExportPath      string
	PaletteFilter   string
	PaletteIdx      int
	PaletteItems    []PaletteItem
	Inputs          *ModelInputs
}

// PaletteItem is one command-palette action.
type PaletteItem struct {
	ID    string
	Label string
	Keys  string
	Group string
}

// ModelInputs holds secondary text inputs.
type ModelInputs struct {
	ExportInput  textinput.Model
	PaletteInput textinput.Model
}

// NewModel creates a default model.
func NewModel() Model {
	fi := textinput.New()
	fi.Placeholder = "search tables…"
	fi.CharLimit = 128
	fi.SetWidth(24)
	fi.Prompt = ""

	return Model{
		Screen:       types.ScreenConnections,
		Connections:  []types.Connection{},
		ConnInputs:   createConnectionInputs(),
		KeyBindings:  types.DefaultKeyBindings(),
		PageSize:     100,
		NavSection:   navTables,
		KindEnabled:  defaultKindFilters(),
		Focus:        focusSidebar,
		QueryFocus:   "editor",
		FilterInput:  fi,
		SQLCompleter: newSQLCompleter(),
		SchemaCols:   map[string][]string{},
		Inputs: &ModelInputs{
			ExportInput:  createTextInput("Export path (e.g. /tmp/out.csv)", 50),
			PaletteInput: createTextInput("Filter commands…", 40),
		},
		PaletteItems: defaultPaletteItems(),
	}
}

// rebuildSQLCompleter refreshes autocomplete candidates from objects + column cache.
func (m *Model) rebuildSQLCompleter() {
	if m.SQLCompleter == nil {
		m.SQLCompleter = newSQLCompleter()
	}
	m.SQLCompleter.Rebuild(m.Objects, m.SchemaCols)
}

// cacheDetailColumns stores columns from a loaded table/view detail for completions.
func (m *Model) cacheDetailColumns(d types.TableDetail) {
	if d.Object.Name == "" || len(d.Columns) == 0 {
		return
	}
	if m.SchemaCols == nil {
		m.SchemaCols = map[string][]string{}
	}
	names := make([]string, 0, len(d.Columns))
	for _, c := range d.Columns {
		if c.Name != "" {
			names = append(names, c.Name)
		}
	}
	key := d.Object.FullName()
	m.SchemaCols[key] = names
	if d.Object.Name != "" {
		m.SchemaCols[d.Object.Name] = names
	}
}

// refreshQuerySuggestions recomputes context-aware autocomplete at the cursor.
func (m *Model) refreshQuerySuggestions() {
	m.QuerySuggests = nil
	m.QuerySuggestIdx = 0
	if m.QueryArea == nil || m.SQLCompleter == nil {
		return
	}
	val := m.QueryArea.Value()
	line, col := m.QueryArea.Line(), m.QueryArea.Column()
	token, _ := sqlTokenAtCursor(val, line, col)
	before := textBeforeCursor(val, line, col)
	// Allow empty prefix after FROM/SELECT/etc. so a bare space still lists tables/cols.
	if token == "" {
		ctx, _ := detectSQLContext(before)
		switch ctx {
		case ctxFrom, ctxJoin, ctxSelectList, ctxWhere, ctxOn, ctxOrder, ctxGroup, ctxInsert:
			// ok — show contextual list
		default:
			if !onlyWhitespace(before) {
				return
			}
		}
	}
	m.QuerySuggests = m.SQLCompleter.MatchAt(token, before, 8)
}

// acceptQuerySuggestion inserts the highlighted completion into the editor.
func (m *Model) acceptQuerySuggestion() bool {
	if m.QueryArea == nil || len(m.QuerySuggests) == 0 {
		return false
	}
	idx := clamp(m.QuerySuggestIdx, 0, len(m.QuerySuggests)-1)
	sug := m.QuerySuggests[idx]
	line, col := m.QueryArea.Line(), m.QueryArea.Column()
	newVal, newCol := applySQLSuggestion(m.QueryArea.Value(), line, col, sug)
	m.QueryArea.SetValue(newVal)
	m.QueryArea.SetCursorColumn(newCol)
	m.QuerySuggests = nil
	m.QuerySuggestIdx = 0
	m.refreshQuerySuggestions()
	return true
}

// defaultKindFilters enables Tables only (views etc. opt-in).
func defaultKindFilters() map[NavSection]bool {
	return map[NavSection]bool{
		navTables:     true,
		navViews:      false,
		navSequences:  false,
		navFunctions:  false,
		navTypes:      false,
		navExtensions: false,
	}
}

func (m Model) enabledObjectKinds() []types.ObjectKind {
	var kinds []types.ObjectKind
	for _, n := range objectKindNavItems() {
		if m.KindEnabled[n] {
			kinds = append(kinds, n.ObjectKind())
		}
	}
	return kinds
}

func (m Model) kindChecked(n NavSection) bool {
	if m.KindEnabled == nil {
		return n == navTables
	}
	return m.KindEnabled[n]
}

func defaultPaletteItems() []PaletteItem {
	return []PaletteItem{
		{ID: "query", Label: "SQL Query editor", Keys: "; / :", Group: "Browse"},
		{ID: "activity", Label: "Activity (pg_stat_activity)", Keys: "A", Group: "Server"},
		{ID: "erd", Label: "ER diagram (FK relationships)", Keys: "E", Group: "Browse"},
		{ID: "server", Label: "Server info", Keys: "i", Group: "Server"},
		{ID: "tables", Label: "Browse tables", Keys: "", Group: "Browse"},
		{ID: "views", Label: "Browse views", Keys: "", Group: "Browse"},
		{ID: "databases", Label: "Switch database", Keys: "", Group: "Connection"},
		{ID: "disconnect", Label: "Disconnect", Keys: "", Group: "Connection"},
		{ID: "help", Label: "Help", Keys: "?", Group: "App"},
		{ID: "logs", Label: "App logs", Keys: "L", Group: "App"},
	}
}

func createTextInput(placeholder string, width int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 4096
	ti.SetWidth(width)
	return ti
}

const (
	connFieldName = iota
	connFieldHost
	connFieldPort
	connFieldUser
	connFieldPass
	connFieldDatabase
	connFieldSSL
	connFieldReadOnly
	connFieldCount
)

const connTextCount = 6

var sslModeOptions = []types.SSLMode{
	types.SSLModeDisable,
	types.SSLModeAllow,
	types.SSLModePrefer,
	types.SSLModeRequire,
	types.SSLModeVerifyCA,
	types.SSLModeVerifyFull,
}

var connTextLabels = []string{
	"Name",
	"Host",
	"Port",
	"Username",
	"Password",
	"Database",
}

func createConnectionInputs() []textinput.Model {
	placeholders := []string{
		"local-pg",
		"localhost",
		"5432",
		"postgres",
		"optional",
		"postgres",
	}
	inputs := make([]textinput.Model, connTextCount)
	for i, ph := range placeholders {
		ti := textinput.New()
		ti.Placeholder = ph
		ti.CharLimit = 2048
		ti.SetWidth(42)
		switch i {
		case connFieldPort:
			ti.SetValue("5432")
		case connFieldHost:
			ti.SetValue("localhost")
		case connFieldUser:
			ti.SetValue("postgres")
		case connFieldDatabase:
			ti.SetValue("postgres")
		case connFieldPass:
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}
		inputs[i] = ti
	}
	return inputs
}

func sslIndex(m types.SSLMode) int {
	for i, opt := range sslModeOptions {
		if opt == m || (m == "" && opt == types.SSLModePrefer) {
			return i
		}
	}
	return 2 // prefer
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.Cmds != nil {
		cmds = append(cmds, m.Cmds.LoadConnections())
	}
	if m.CLIConnection != nil {
		conn := *m.CLIConnection
		cmds = append(cmds, func() tea.Msg {
			return types.AutoConnectMsg{Connection: conn}
		})
	}
	cmds = append(cmds, cmd.CheckForUpdate(m.Version))
	return tea.Batch(cmds...)
}

func (m *Model) pushQueryHistory(q string) {
	q = strings.TrimSpace(q)
	if q == "" {
		return
	}
	if len(m.QueryHistory) > 0 && m.QueryHistory[0] == q {
		return
	}
	m.QueryHistory = append([]string{q}, m.QueryHistory...)
	if len(m.QueryHistory) > 30 {
		m.QueryHistory = m.QueryHistory[:30]
	}
	m.HistoryIdx = -1
}

// objectSearchQuery is the live filter string for the object tree.
func (m Model) objectSearchQuery() string {
	if m.FilterActive {
		return strings.ToLower(strings.TrimSpace(m.FilterInput.Value()))
	}
	if q := strings.ToLower(strings.TrimSpace(m.ObjectFilter)); q != "" {
		return q
	}
	return strings.ToLower(strings.TrimSpace(m.FilterInput.Value()))
}

func (m Model) filteredObjects() []types.SchemaObject {
	f := m.objectSearchQuery()
	if f == "" {
		return m.Objects
	}
	var out []types.SchemaObject
	for _, o := range m.Objects {
		name := strings.ToLower(o.Name)
		kind := strings.ToLower(string(o.Kind))
		// Prefer name match. Full "schema.name" only when the query has a dot
		// so "u" does not match every public.* object via the schema segment.
		match := strings.Contains(name, f) || strings.Contains(kind, f)
		if !match && strings.Contains(f, ".") {
			match = strings.Contains(strings.ToLower(o.FullName()), f)
		}
		if match {
			out = append(out, o)
		}
	}
	return out
}

func (m Model) selectedObject() (types.SchemaObject, bool) {
	objs := m.filteredObjects()
	if len(objs) == 0 {
		return types.SchemaObject{}, false
	}
	return objs[clamp(m.SelectedObjIdx, 0, len(objs)-1)], true
}

// objectIdentityMatch is true when cur refers to the same schema object as o.
// Kind is compared when present so table "orders" does not match type "orders".
func objectIdentityMatch(cur *types.SchemaObject, o types.SchemaObject) bool {
	if cur == nil {
		return false
	}
	if cur.Name != o.Name || cur.Schema != o.Schema {
		return false
	}
	if cur.Kind != "" && o.Kind != "" {
		return cur.Kind == o.Kind
	}
	return true
}

func (m Model) setCurrentObject(o types.SchemaObject) Model {
	cp := o
	m.CurrentObject = &cp
	return m
}

func (m Model) clearObjectContent() Model {
	m.contentSeq++
	m.CurrentObject = nil
	m.TableDetail = types.TableDetail{}
	m.TableData = types.QueryResult{}
	m.DataOffset = 0
	m.DataCursor = 0
	m.DataCol = 0
	m.DetailTab = 0
	return m
}

func (m Model) resetToBrowserList() Model {
	m = m.clearObjectContent()
	m.Screen = types.ScreenBrowser
	m.Focus = focusSidebar
	m.ContentMode = contentPreview
	return m
}

// syncSidebarCursorToObject moves the tree cursor onto the selected object row.
func (m Model) syncSidebarCursorToObject() Model {
	rows := m.buildSidebarRows()
	for i, r := range rows {
		if r.kind == sbObject && r.objIdx == m.SelectedObjIdx {
			m.SidebarCursor = i
			return m
		}
	}
	return m
}

// pinSidebarCursorToKind keeps the cursor on a specific FILTERS checkbox row.
var sidebarRowsFor = func(m Model) []sidebarRow { return m.buildSidebarRows() }

func (m Model) pinSidebarCursorToKind(nav NavSection) Model {
	for i, r := range sidebarRowsFor(m) {
		if r.kind == sbKind && r.nav == nav {
			m.SidebarCursor = i
			return m
		}
	}
	// Fallback: first kind row
	for i, r := range sidebarRowsFor(m) {
		if r.kind == sbKind {
			m.SidebarCursor = i
			return m
		}
	}
	return m
}

func (m Model) currentSchemaInfo() (types.SchemaInfo, bool) {
	for _, s := range m.Schemas {
		if s.Name == m.CurrentSchema {
			return s, true
		}
	}
	if m.SelectedSchema >= 0 && m.SelectedSchema < len(m.Schemas) {
		return m.Schemas[m.SelectedSchema], true
	}
	return types.SchemaInfo{}, false
}

func (m Model) pinSidebarCursorToSchema(schemaIdx int) Model {
	for i, r := range m.buildSidebarRows() {
		if r.kind == sbSchema && r.schema == schemaIdx {
			m.SidebarCursor = i
			return m
		}
	}
	return m
}

// pinSidebarAfterObjectsLoad restores cursor after async LoadObjects.
func (m Model) pinSidebarAfterObjectsLoad() Model {
	rows := sidebarRowsFor(m)
	if len(rows) == 0 {
		return m
	}
	cur := rows[clamp(m.SidebarCursor, 0, len(rows)-1)]
	switch cur.kind {
	case sbKind:
		return m.pinSidebarCursorToKind(m.NavSection)
	case sbSchema:
		if cur.schema == m.SelectedSchema {
			return m
		}
		return m.pinSidebarCursorToSchema(m.SelectedSchema)
	case sbTool, sbObject:
		return m
	default:
		return m.pinSidebarCursorToKind(m.NavSection)
	}
}

func objectInList(objs []types.SchemaObject, o *types.SchemaObject) bool {
	if o == nil {
		return false
	}
	for _, x := range objs {
		if x.Schema == o.Schema && x.Name == o.Name && x.Kind == o.Kind {
			return true
		}
	}
	return false
}

// sidebarNavItems returns object-kind + tool nav (legacy callers).
func sidebarNavItems() []NavSection {
	return append(objectKindNavItems(), toolNavItems()...)
}
