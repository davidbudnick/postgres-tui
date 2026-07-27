package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

// TablePlus-style workspace: wide left tree (~28–34) + dominant content pane.
const sidebarTreeWidth = 32

type sidebarRowKind int

const (
	sbSchema sidebarRowKind = iota
	sbObject
	sbTool
	sbKind // kind filter rows (Tables / Views / …)
	sbNav  // legacy kind (same as sbKind)
)

type sidebarRow struct {
	kind   sidebarRowKind
	nav    NavSection
	schema int
	objIdx int // index into filteredObjects()
	label  string
}

func (m Model) buildSidebarRows() []sidebarRow {
	var rows []sidebarRow
	for i, s := range m.Schemas {
		rows = append(rows, sidebarRow{kind: sbSchema, schema: i, label: s.Name})
	}
	// Multi-select kind filters ([x] Tables [ ] Views …)
	for _, n := range objectKindNavItems() {
		rows = append(rows, sidebarRow{kind: sbKind, nav: n, label: n.String()})
	}
	objs := m.filteredObjects()
	for i, o := range objs {
		rows = append(rows, sidebarRow{kind: sbObject, objIdx: i, label: o.Name})
	}
	for _, n := range toolNavItems() {
		rows = append(rows, sidebarRow{kind: sbTool, nav: n, label: n.String()})
	}
	return rows
}

func objectKindNavItems() []NavSection {
	return []NavSection{navTables, navViews, navSequences, navFunctions, navTypes, navExtensions}
}

func toolNavItems() []NavSection {
	return []NavSection{navQuery, navActivity, navERD, navServer, navDatabases}
}

func (m Model) isWorkspaceScreen() bool {
	switch m.Screen {
	case types.ScreenBrowser, types.ScreenTableData, types.ScreenTableDetail,
		types.ScreenQuery, types.ScreenActivity, types.ScreenERD, types.ScreenServerInfo:
		return true
	default:
		return false
	}
}

func (m Model) viewSidebar(width, height int) string {
	focused := m.Focus == focusSidebar || m.Focus == focusMain
	inner := width - 2
	if inner < 10 {
		inner = 10
	}

	var b strings.Builder
	if m.CurrentDatabase != "" {
		b.WriteString(dimStyle.Render(" " + truncate(m.CurrentDatabase, inner-1)))
		b.WriteByte('\n')
	}

	// TablePlus-style search field across the tree.
	searchFocused := m.FilterActive
	m.FilterInput.SetWidth(max(inner-3, 8))
	m.FilterInput.Placeholder = "search…"
	searchVal := m.FilterInput.Value()
	if searchVal == "" {
		searchVal = m.ObjectFilter
	}
	if searchFocused {
		// View() already includes cursor; don't pad/truncate ANSI.
		b.WriteString(searchBarActiveStyle.Width(inner).Render("/" + m.FilterInput.View()))
	} else if searchVal != "" {
		b.WriteString(searchBarStyle.Width(inner).Render(padRight(truncate("/"+searchVal, inner), inner)))
	} else {
		b.WriteString(searchBarStyle.Width(inner).Render(padRight("/ search…", inner)))
	}
	b.WriteByte('\n')

	rows := m.buildSidebarRows()
	cursor := clamp(m.SidebarCursor, 0, max(len(rows)-1, 0))

	idx := 0
	emitSection := func(title string) {
		b.WriteByte('\n')
		b.WriteString(sectionStyle.Render(title))
		b.WriteByte('\n')
	}
	emit := func(colored, plain string) {
		atCursor := idx == cursor && !searchFocused
		switch {
		case atCursor && focused:
			b.WriteString(selectedStyle.Render(padRight(truncate(plain, inner), inner)))
		case atCursor:
			b.WriteString(selectedDimStyle.Render(padRight(truncate(plain, inner), inner)))
		default:
			b.WriteString(padRight(truncate(colored, inner), inner))
		}
		b.WriteByte('\n')
		idx++
	}

	// SCHEMAS
	emitSection("SCHEMAS")
	if len(m.Schemas) == 0 {
		b.WriteString(dimStyle.Render(" (none)"))
		b.WriteByte('\n')
	}
	for i, s := range m.Schemas {
		active := i == m.SelectedSchema || s.Name == m.CurrentSchema
		icon := " "
		if active {
			icon = "▸"
		}
		count := fmt.Sprintf("%d", s.TableCount)
		plain := packRow(fmt.Sprintf("%s %s", icon, s.Name), count, inner)
		var colored string
		if active {
			colored = packRow(accentStyle.Render(icon+" ")+schemaNameStyle.Bold(true).Render(s.Name), rowCountStyle.Render(count), inner)
		} else {
			colored = packRow(dimStyle.Render(icon+" ")+schemaNameStyle.Render(s.Name), rowCountStyle.Render(count), inner)
		}
		emit(colored, plain)
	}

	// FILTERS — multi-select kinds shown in the object list
	emitSection("FILTERS")
	for _, n := range objectKindNavItems() {
		check := "[ ]"
		if m.kindChecked(n) {
			check = "[x]"
		}
		plain := check + " " + n.String()
		var colored string
		if m.kindChecked(n) {
			colored = accentStyle.Render(check) + " " + normalStyle.Render(n.String())
		} else {
			colored = dimStyle.Render(plain)
		}
		emit(colored, plain)
	}

	// OBJECTS (union of enabled filters)
	objs := m.filteredObjects()
	title := "OBJECTS"
	enabled := 0
	for _, n := range objectKindNavItems() {
		if m.kindChecked(n) {
			enabled++
			if enabled == 1 {
				title = strings.ToUpper(n.String())
			}
		}
	}
	if enabled != 1 {
		title = "OBJECTS"
	}
	if m.Loading && m.Screen == types.ScreenBrowser && m.ContentMode != contentDatabases {
		emitSection(title + " …")
	} else {
		emitSection(fmt.Sprintf("%s (%d)", title, len(objs)))
	}
	if len(objs) == 0 && !m.Loading {
		if searchVal != "" {
			b.WriteString(dimStyle.Render(" (no matches)"))
		} else {
			b.WriteString(dimStyle.Render(" (none)"))
		}
		b.WriteByte('\n')
	}
	for i, o := range objs {
		name := o.Name
		rowsEst := ""
		if o.RowEstimate > 0 {
			rowsEst = compactInt(o.RowEstimate)
		}
		glyph := kindGlyph(string(o.Kind))
		plainLeft := glyph + " " + name
		plain := packRow(plainLeft, rowsEst, inner)
		colored := packRow(
			kindStyle(string(o.Kind)).Render(glyph)+" "+normalStyle.Render(name),
			rowCountStyle.Render(rowsEst),
			inner,
		)
		// Match schema+name+kind — bare name match double-highlights table+type pairs (orders).
		if objectIdentityMatch(m.CurrentObject, o) {
			colored = packRow(
				kindStyle(string(o.Kind)).Render(glyph)+" "+accentStyle.Bold(true).Render(name),
				sizeStyle.Render(rowsEst),
				inner,
			)
		}
		_ = i
		emit(colored, plain)
	}

	// TOOLS — compact "tag name" rows (ASCII tags stay aligned in every font)
	emitSection("TOOLS")
	for _, n := range toolNavItems() {
		tag := toolGlyph(n)
		active := false
		switch n {
		case navQuery:
			active = m.Screen == types.ScreenQuery
		case navActivity:
			active = m.Screen == types.ScreenActivity
		case navERD:
			active = m.Screen == types.ScreenERD
		case navServer:
			active = m.Screen == types.ScreenServerInfo
		case navDatabases:
			active = m.ContentMode == contentDatabases
		}
		// Fixed 3-col tag gutter: " > " / " * " …
		plain := fmt.Sprintf(" %s %s", tag, n.String())
		colored := " " + toolStyle(n).Render(tag) + " " + normalStyle.Render(n.String())
		if active {
			plain = fmt.Sprintf("▸%s %s", tag, n.String())
			colored = accentStyle.Bold(true).Render("▸"+tag) + " " + accentStyle.Bold(true).Render(n.String())
		}
		emit(colored, plain)
	}

	return panelStyle(focused || searchFocused, width, height).Render(clampLines(b.String(), height-2))
}

func compactInt(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func (m Model) viewBrowser() string {
	return m.viewWorkspace()
}

func (m Model) viewWorkspace() string {
	header := m.viewWorkspaceHeader()
	footer := m.viewWorkspaceFooter()
	status := m.getStatusBar()
	chrome := lipglossHeight(header) + lipglossHeight(footer)
	if status != "" {
		chrome += 2
	}
	bodyHeight := m.Height - chrome
	if bodyHeight < 6 {
		bodyHeight = 6
	}

	// Two panes: tree | content (TablePlus proportions — content dominates).
	sw := sidebarTreeWidth
	if m.Width > 0 {
		// ~28% of width, clamped
		sw = clamp(m.Width*28/100, 24, 40)
	}
	cw := m.Width - sw
	if cw < 30 {
		cw = max(m.Width/2, 20)
		sw = m.Width - cw
	}

	sidebar := m.viewSidebar(sw, bodyHeight)
	content := m.viewContentPane(cw, bodyHeight)
	body := joinHorizontal(sidebar, content)
	return header + "\n" + body + "\n" + footer
}

func (m Model) viewBrowserPreview(width, height int) string {
	// Used as default content when browsing (nothing open yet).
	focused := m.Focus == focusContent
	inner := width - 2
	if inner < 8 {
		inner = 8
	}
	var b strings.Builder
	b.WriteString(headerStyle.Render("Preview"))
	b.WriteByte('\n')
	b.WriteString(gridSepStyle.Render(strings.Repeat("─", min(inner, 80))))
	b.WriteByte('\n')

	objs := m.filteredObjects()
	if len(objs) == 0 {
		b.WriteString(dimStyle.Render(" Select an object in the left tree"))
		b.WriteByte('\n')
		b.WriteString(dimStyle.Render(" enter open · D detail · ; query"))
		b.WriteByte('\n')
	} else {
		sel := clamp(m.SelectedObjIdx, 0, len(objs)-1)
		// Prefer CurrentObject if set
		o := objs[sel]
		if m.CurrentObject != nil {
			for _, x := range objs {
				if objectIdentityMatch(m.CurrentObject, x) {
					o = x
					break
				}
			}
		}
		b.WriteString(m.renderObjectPreviewBody(o, inner, height-2))
	}

	return panelStyle(focused, width, height).Render(clampLines(b.String(), height-2))
}

func (m Model) viewContentPane(width, height int) string {
	focused := m.Focus == focusContent
	innerH := height - 2
	if innerH < 3 {
		innerH = 3
	}
	var body string
	switch m.Screen {
	case types.ScreenBrowser:
		switch m.ContentMode {
		case contentSchema:
			body = m.viewSchemaOverviewContent(width-2, innerH)
		case contentDatabases:
			body = m.viewDatabasesContent(width-2, innerH)
		default:
			body = m.viewBrowserPreviewContent(width-2, innerH)
		}
	case types.ScreenTableData:
		body = m.viewTableDataContent(width-2, innerH)
	case types.ScreenTableDetail:
		body = m.viewTableDetailContent(width-2, innerH)
	case types.ScreenQuery:
		body = m.viewQueryContent(width-2, innerH)
	case types.ScreenActivity:
		body = m.viewActivityContent(width-2, innerH)
	case types.ScreenERD:
		body = m.viewERDContent(width-2, innerH)
	case types.ScreenServerInfo:
		body = m.viewServerInfoContent(width-2, innerH)
	default:
		body = dimStyle.Render(" ")
	}
	return panelStyle(focused, width, height).Render(clampLines(body, innerH))
}

func (m Model) viewBrowserPreviewContent(width, height int) string {
	inner := width
	if inner < 8 {
		inner = 8
	}
	var b strings.Builder
	objs := m.filteredObjects()
	if len(objs) == 0 {
		b.WriteString(headerStyle.Render("Preview"))
		b.WriteByte('\n')
		b.WriteString(gridSepStyle.Render(strings.Repeat("─", min(inner, 80))))
		b.WriteByte('\n')
		b.WriteString(dimStyle.Render("Select a table in the left tree"))
		b.WriteByte('\n')
		b.WriteString(dimStyle.Render("enter open · D structure · space filter · ; query"))
		b.WriteByte('\n')
		_ = height
		return b.String()
	}
	sel := clamp(m.SelectedObjIdx, 0, len(objs)-1)
	o := objs[sel]
	if m.CurrentObject != nil {
		for _, x := range objs {
			if objectIdentityMatch(m.CurrentObject, x) {
				o = x
				break
			}
		}
	}
	b.WriteString(m.renderObjectPreviewBody(o, inner, height))
	return b.String()
}

// renderObjectPreviewBody is a dense TablePlus-style object card (not a sparse void).
func (m Model) renderObjectPreviewBody(o types.SchemaObject, inner, height int) string {
	var b strings.Builder

	// Title row: kind badge + full name
	kind := string(o.Kind)
	if kind == "" {
		kind = "table"
	}
	b.WriteString(kindStyle(kind).Bold(true).Render(kindGlyph(kind) + " " + kind))
	b.WriteString(dimStyle.Render("  ·  "))
	b.WriteString(titleStyle.Render(truncate(o.FullName(), max(inner-12, 8))))
	b.WriteByte('\n')
	b.WriteString(gridSepStyle.Render(strings.Repeat("─", min(inner, 80))))
	b.WriteByte('\n')

	// Meta as compact key/value rows
	type kv struct{ k, v string }
	rows := make([]kv, 0, 6)
	if o.Schema != "" {
		rows = append(rows, kv{"schema", o.Schema})
	}
	if o.Owner != "" {
		rows = append(rows, kv{"owner", o.Owner})
	}
	if o.RowEstimate > 0 {
		rows = append(rows, kv{"rows", fmtInt64(o.RowEstimate)})
	} else if isRelationObject(o) {
		rows = append(rows, kv{"rows", "—"})
	}
	if o.SizePretty != "" {
		rows = append(rows, kv{"size", o.SizePretty})
	}
	rows = append(rows, kv{"kind", kind})

	labelW := 8
	for _, r := range rows {
		b.WriteString(dimStyle.Render(padRight(r.k, labelW)))
		switch r.k {
		case "rows":
			b.WriteString(rowCountStyle.Render(r.v))
		case "size":
			b.WriteString(sizeStyle.Render(r.v))
		case "schema":
			b.WriteString(schemaNameStyle.Render(r.v))
		case "kind":
			b.WriteString(kindStyle(kind).Render(r.v))
		default:
			b.WriteString(normalStyle.Render(r.v))
		}
		b.WriteByte('\n')
	}

	if o.Comment != "" {
		b.WriteByte('\n')
		b.WriteString(sectionStyle.Render("COMMENT"))
		b.WriteByte('\n')
		b.WriteString(dimStyle.Render(truncate(o.Comment, inner)))
		b.WriteByte('\n')
	}

	// Column peek from cache (filled after structure/data loads) — fills empty pane usefully.
	cols := m.cachedColumnsFor(o)
	if len(cols) > 0 {
		b.WriteByte('\n')
		b.WriteString(sectionStyle.Render(fmt.Sprintf("COLUMNS (%d)", len(cols))))
		b.WriteByte('\n')
		maxShow := min(len(cols), max(height-14, 4))
		for i := 0; i < maxShow; i++ {
			c := cols[i]
			b.WriteString(dimStyle.Render(fmt.Sprintf("  %2d  ", i+1)))
			b.WriteString(normalStyle.Render(truncate(c, max(inner-8, 6))))
			b.WriteByte('\n')
		}
		if len(cols) > maxShow {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  … +%d more  ·  D structure", len(cols)-maxShow)))
			b.WriteByte('\n')
		}
	} else if isRelationObject(o) {
		b.WriteByte('\n')
		b.WriteString(dimStyle.Render("Columns not cached yet — press D for structure or enter for data."))
		b.WriteByte('\n')
	}

	// Action chips (same style as footer help)
	b.WriteByte('\n')
	b.WriteString(sectionStyle.Render("ACTIONS"))
	b.WriteByte('\n')
	var actions []struct{ key, desc string }
	switch o.Kind {
	case types.ObjectTable, types.ObjectView, types.ObjectMatView, "":
		actions = []struct{ key, desc string }{
			{"enter", "open data"},
			{"D", "structure"},
			{";", "query"},
			{"E", "ERD"},
		}
	case types.ObjectSequence, types.ObjectFunction, types.ObjectType, types.ObjectExtension:
		actions = []struct{ key, desc string }{
			{"enter", "open detail"},
			{"D", "detail"},
			{";", "query"},
		}
	default:
		actions = []struct{ key, desc string }{
			{"enter", "open"},
			{";", "query"},
		}
	}
	b.WriteString(renderKeyHelpWidth(max(inner, 20), actions))
	b.WriteByte('\n')
	return b.String()
}

func (m Model) cachedColumnsFor(o types.SchemaObject) []string {
	if m.SchemaCols == nil {
		return nil
	}
	if cols, ok := m.SchemaCols[o.FullName()]; ok {
		return cols
	}
	if cols, ok := m.SchemaCols[o.Name]; ok {
		return cols
	}
	// Detail already loaded for this object
	if objectIdentityMatch(m.CurrentObject, o) && len(m.TableDetail.Columns) > 0 {
		out := make([]string, 0, len(m.TableDetail.Columns))
		for _, c := range m.TableDetail.Columns {
			if c.Name != "" {
				out = append(out, c.Name)
			}
		}
		return out
	}
	return nil
}

func (m Model) viewSchemaOverviewContent(width, height int) string {
	inner := width
	if inner < 8 {
		inner = 8
	}
	var b strings.Builder
	s, ok := m.currentSchemaInfo()
	name := m.CurrentSchema
	if ok {
		name = s.Name
	}
	if name == "" {
		name = "—"
	}
	b.WriteString(headerStyle.Render("Schema · " + name))
	b.WriteByte('\n')
	b.WriteString(gridSepStyle.Render(strings.Repeat("─", min(inner, 80))))
	b.WriteByte('\n')

	if ok {
		if s.Owner != "" {
			b.WriteString(dimStyle.Render("owner   ") + normalStyle.Render(s.Owner))
			b.WriteByte('\n')
		}
		b.WriteString(dimStyle.Render("tables  ") + normalStyle.Render(fmtInt(s.TableCount)))
		b.WriteByte('\n')
		b.WriteString(dimStyle.Render("views   ") + normalStyle.Render(fmtInt(s.ViewCount)))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	// Multi-select filters share one list — don't label it with a single kind.
	listTitle := "OBJECTS"
	enabledKinds := 0
	var onlyKind NavSection
	for _, n := range objectKindNavItems() {
		if m.kindChecked(n) {
			enabledKinds++
			onlyKind = n
		}
	}
	if enabledKinds == 1 {
		listTitle = strings.ToUpper(onlyKind.String())
	}
	b.WriteString(sectionStyle.Render(listTitle))
	b.WriteByte('\n')

	objs := m.filteredObjects()
	if m.Loading {
		b.WriteString(dimStyle.Render("Loading objects…"))
		b.WriteByte('\n')
		return b.String()
	}
	if len(objs) == 0 {
		b.WriteString(dimStyle.Render("(none)"))
		b.WriteByte('\n')
		b.WriteString(dimStyle.Render("enter on a table in the sidebar to open"))
		b.WriteByte('\n')
		_ = height
		return b.String()
	}

	nameW := min(max(inner/2, 14), 36)
	// KIND column when multiple kinds are mixed so types ≠ tables at a glance.
	showKind := enabledKinds != 1
	kindW := 0
	if showKind {
		kindW = 10
	}
	var hdr string
	if showKind {
		hdr = fmt.Sprintf("%s  %s  %s  %s",
			padRight("NAME", nameW),
			padRight("KIND", kindW),
			padRight("ROWS", 10),
			padRight("SIZE", 12),
		)
	} else {
		hdr = fmt.Sprintf("%s  %s  %s",
			padRight("NAME", nameW),
			padRight("ROWS", 10),
			padRight("SIZE", 12),
		)
	}
	b.WriteString(tableHeaderStyle.Render(padRight(hdr, inner)))
	b.WriteByte('\n')

	// Only the focused pane gets the bright blue cursor band — avoids dual selection
	// when sidebar and schema overview share SelectedObjIdx.
	contentFocused := m.Focus == focusContent
	sel := clamp(m.SelectedObjIdx, 0, len(objs)-1)
	maxVisible := max(height-10, 4)
	start, end := listWindow(sel, len(objs), maxVisible)
	for i := start; i < end; i++ {
		o := objs[i]
		rowsEst := "—"
		if o.RowEstimate > 0 {
			rowsEst = compactInt(o.RowEstimate)
		}
		size := o.SizePretty
		if size == "" {
			size = "—"
		}
		namePlain := padRight(o.Name, nameW)
		rowsPlain := padRight(rowsEst, 10)
		sizePlain := padRight(size, 12)
		kindPlain := ""
		if showKind {
			kindPlain = padRight(string(o.Kind), kindW)
		}
		var plain string
		if showKind {
			plain = fmt.Sprintf("%s  %s  %s  %s", namePlain, kindPlain, rowsPlain, sizePlain)
		} else {
			plain = fmt.Sprintf("%s  %s  %s", namePlain, rowsPlain, sizePlain)
		}
		ks := kindStyle(string(o.Kind))
		zebra := i%2 == 1
		paint := func(st lipgloss.Style, text string) string {
			if zebra {
				return zebraStyle.Foreground(st.GetForeground()).Render(text)
			}
			return st.Render(text)
		}
		// Bright blue band ONLY when the content pane owns focus.
		// Sidebar-focused: no bg at all (dim blue still looked like dual select).
		if i == sel && contentFocused {
			b.WriteString(selectedStyle.Render(padRight(plain, inner)))
			b.WriteByte('\n')
			continue
		}
		nameSt := ks
		if i == sel {
			nameSt = accentStyle.Bold(true)
		}
		b.WriteString(paint(nameSt, namePlain))
		if showKind {
			b.WriteString("  ")
			b.WriteString(paint(ks, kindPlain))
		}
		b.WriteString("  ")
		b.WriteString(paint(rowCountStyle, rowsPlain))
		b.WriteString("  ")
		b.WriteString(paint(sizeStyle, sizePlain))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(dimStyle.Render("enter on table in sidebar · [ ] cycle kind · esc preview"))
	b.WriteByte('\n')
	return b.String()
}

func (m Model) viewDatabasesContent(width, height int) string {
	inner := width
	if inner < 8 {
		inner = 8
	}
	var b strings.Builder
	b.WriteString(headerStyle.Render("Databases"))
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d on server", len(m.Databases))))
	b.WriteByte('\n')
	b.WriteString(gridSepStyle.Render(strings.Repeat("─", min(inner, 80))))
	b.WriteByte('\n')

	if m.Loading && len(m.Databases) == 0 {
		b.WriteString(dimStyle.Render("Loading databases…"))
		b.WriteByte('\n')
		return b.String()
	}
	if len(m.Databases) == 0 {
		b.WriteString(dimStyle.Render("No databases found."))
		b.WriteByte('\n')
		b.WriteString(dimStyle.Render("r refresh · esc back"))
		b.WriteByte('\n')
		return b.String()
	}

	nameW := colWidth(inner, 42, 16, 48)
	hdr := fmt.Sprintf("%s  %s  %s  %s",
		padRight("DATABASE", nameW),
		padRight("OWNER", 16),
		padRight("SIZE", 12),
		padRight("ENCODING", 10),
	)
	b.WriteString(tableHeaderStyle.Render(padRight(hdr, inner)))
	b.WriteByte('\n')

	maxVisible := max(height-5, 4)
	sel := clamp(m.SelectedDBIdx, 0, len(m.Databases)-1)
	start, end := listWindow(sel, len(m.Databases), maxVisible)
	for i := start; i < end; i++ {
		db := m.Databases[i]
		namePlain := padRight(db.Name, nameW)
		ownerPlain := padRight(db.Owner, 16)
		sizePlain := padRight(db.SizePretty, 12)
		encPlain := padRight(db.Encoding, 10)
		line := fmt.Sprintf("%s  %s  %s  %s", namePlain, ownerPlain, sizePlain, encPlain)
		cur := db.Name == m.CurrentDatabase
		if i == sel {
			b.WriteString(selectedStyle.Render(padRight(line, inner)))
			b.WriteByte('\n')
			continue
		}
		if cur {
			b.WriteString(greenStyle.Render(padRight(line, inner)))
			b.WriteByte('\n')
			continue
		}
		zebra := i%2 == 1
		paint := func(st lipgloss.Style, text string) string {
			if zebra {
				return zebraStyle.Foreground(st.GetForeground()).Render(text)
			}
			return st.Render(text)
		}
		b.WriteString(paint(normalStyle, namePlain))
		b.WriteString("  ")
		b.WriteString(paint(dimStyle, ownerPlain))
		b.WriteString("  ")
		b.WriteString(paint(sizeStyle, sizePlain))
		b.WriteString("  ")
		b.WriteString(paint(typeTextStyle, encPlain))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(dimStyle.Render("enter open · r refresh · esc back"))
	b.WriteByte('\n')
	return b.String()
}

func (m Model) viewWorkspaceHeader() string {
	conn := "—"
	if m.CurrentConn != nil {
		conn = m.CurrentConn.Name
	}
	db := m.CurrentDatabase
	if db == "" {
		db = "—"
	}

	left := " " + statusPillStyle.Render("●") + " " + normalStyle.Render(conn) +
		dimStyle.Render(" : ") + accentStyle.Render(db)
	if m.CurrentSchema != "" {
		left += dimStyle.Render(" · ") + dimStyle.Render(m.CurrentSchema)
	}

	right := ""
	if m.ReadOnly {
		right = badgeROStyle.Render("RO") + " "
	}
	if m.ServerInfo.Version != "" {
		v := m.ServerInfo.Version
		if i := strings.Index(v, " on "); i > 0 {
			v = v[:i]
		}
		if len(v) > 28 {
			v = v[:28]
		}
		right += dimStyle.Render(v) + " "
	}
	gap := m.Width - lipglossWidth(left) - lipglossWidth(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) viewWorkspaceFooter() string {
	keys := []struct{ key, desc string }{
		{"tab", "pane"},
		{"j/k", "move"},
	}
	switch m.Screen {
	case types.ScreenBrowser:
		switch m.ContentMode {
		case contentDatabases:
			keys = append(keys,
				struct{ key, desc string }{"enter", "open db"},
				struct{ key, desc string }{"r", "refresh"},
				struct{ key, desc string }{"esc", "back"},
			)
		case contentSchema:
			keys = append(keys,
				struct{ key, desc string }{"enter", "open"},
				struct{ key, desc string }{"space", "filter"},
				struct{ key, desc string }{"/", "search"},
				struct{ key, desc string }{"esc", "preview"},
			)
		default:
			keys = append(keys,
				struct{ key, desc string }{"enter", "open"},
				struct{ key, desc string }{"space", "filter"},
				struct{ key, desc string }{"D", "struct"},
				struct{ key, desc string }{"/", "search"},
				struct{ key, desc string }{";", "query"},
				struct{ key, desc string }{"b", "db"},
				struct{ key, desc string }{"esc", "disconnect"},
			)
		}
	case types.ScreenTableData:
		keys = append(keys,
			struct{ key, desc string }{"h/l", "cell"},
			struct{ key, desc string }{"n/p", "page"},
			struct{ key, desc string }{"y", "copy"},
			struct{ key, desc string }{"Y", "row"},
			struct{ key, desc string }{"x", "export"},
			struct{ key, desc string }{"D", "struct"},
			struct{ key, desc string }{"b", "db"},
			struct{ key, desc string }{"esc", "tree"},
		)
	case types.ScreenTableDetail:
		keys = append(keys,
			struct{ key, desc string }{"h/l", "tabs"},
			struct{ key, desc string }{"enter", "data"},
			struct{ key, desc string }{"b", "db"},
			struct{ key, desc string }{"esc", "tree"},
		)
	case types.ScreenQuery:
		keys = append(keys,
			struct{ key, desc string }{"ctrl+↵", "run"},
			struct{ key, desc string }{"ctrl+e", "run"},
			struct{ key, desc string }{"tab", "complete"},
			struct{ key, desc string }{"↑↓", "suggest"},
			struct{ key, desc string }{"x", "export"},
			struct{ key, desc string }{"b", "db"},
			struct{ key, desc string }{"esc", "tree"},
		)
	case types.ScreenActivity:
		keys = append(keys,
			struct{ key, desc string }{"r", "refresh"},
			struct{ key, desc string }{"b", "db"},
			struct{ key, desc string }{"esc", "tree"},
		)
	case types.ScreenERD:
		keys = append(keys,
			struct{ key, desc string }{"j/k", "scroll"},
			struct{ key, desc string }{"r", "refresh"},
			struct{ key, desc string }{"b", "db"},
			struct{ key, desc string }{"esc", "tree"},
		)
	case types.ScreenServerInfo:
		keys = append(keys,
			struct{ key, desc string }{"r", "refresh"},
			struct{ key, desc string }{"b", "db"},
			struct{ key, desc string }{"esc", "tree"},
		)
	}
	if m.Screen == types.ScreenBrowser || m.Screen == types.ScreenTableData || m.Screen == types.ScreenTableDetail {
		keys = append(keys, struct{ key, desc string }{"E", "ERD"})
	}
	keys = append(keys, struct{ key, desc string }{"?", "help"})
	return renderKeyHelpWidth(max(m.Width-2, 20), keys)
}

func (m Model) viewBrowserHeader() string { return m.viewWorkspaceHeader() }
func (m Model) viewBrowserFooter() string { return m.viewWorkspaceFooter() }

func lipglossHeight(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func lipglossWidth(s string) int {
	return displayWidth(s)
}

func stripAnsiRough(s string) string { return s }

func joinHorizontal(left, right string) string {
	return joinHorizontalLipgloss(left, right)
}

func joinH(a, b string) string {
	return joinHorizontalLipgloss(a, b)
}

func joinHorizontal3(a, b, c string) string {
	return joinHorizontalLipgloss3(a, b, c)
}
