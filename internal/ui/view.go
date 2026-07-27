package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func joinHorizontalLipgloss(a, b string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, a, b)
}

func joinHorizontalLipgloss3(a, b, c string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, a, b, c)
}

func (m Model) getScreenView() string {
	switch m.Screen {
	case types.ScreenConnections:
		return m.viewConnections()
	case types.ScreenAddConnection, types.ScreenEditConnection:
		return m.viewConnectionForm()
	case types.ScreenDatabases:
		return m.viewDatabases()
	case types.ScreenBrowser, types.ScreenTableData, types.ScreenTableDetail,
		types.ScreenQuery, types.ScreenActivity, types.ScreenERD, types.ScreenServerInfo:
		return m.viewWorkspace()
	case types.ScreenHelp:
		return m.viewHelp()
	case types.ScreenConfirmDelete:
		return m.viewConfirmDelete()
	case types.ScreenTestConnection:
		return m.viewTestConnection()
	case types.ScreenLogs:
		return m.viewLogs()
	case types.ScreenFavorites:
		return m.viewFavorites()
	case types.ScreenExport:
		return m.viewExport()
	case types.ScreenCommandPalette:
		return m.viewCommandPalette()
	default:
		return ""
	}
}

// View implements tea.Model.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
	// Before the first WindowSizeMsg, Width/Height are 0 — still draw the home
	// screen so launch isn't a blank "too small" flash.
	if m.Width > 0 && m.Height > 0 && (m.Width < 50 || m.Height < 15) {
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center,
			"Terminal too small.\nResize to at least 50x15.")
	}

	content := m.getScreenView()
	status := m.getStatusBar()
	fullContent := content
	if status != "" {
		switch m.Screen {
		case types.ScreenBrowser, types.ScreenTableData, types.ScreenTableDetail,
			types.ScreenQuery, types.ScreenActivity, types.ScreenERD, types.ScreenServerInfo,
			types.ScreenDatabases, types.ScreenLogs, types.ScreenFavorites:
			fullContent = content + "\n" + status
		default:
			fullContent = content + "\n\n" + status
		}
	}

	vPos := lipgloss.Position(lipgloss.Top)
	hPos := lipgloss.Left
	switch m.Screen {
	case types.ScreenConnections, types.ScreenAddConnection, types.ScreenEditConnection,
		types.ScreenConfirmDelete, types.ScreenTestConnection, types.ScreenHelp,
		types.ScreenCommandPalette, types.ScreenExport:
		vPos = lipgloss.Center
		hPos = lipgloss.Center
	case types.ScreenBrowser, types.ScreenTableData, types.ScreenTableDetail,
		types.ScreenQuery, types.ScreenActivity, types.ScreenERD, types.ScreenServerInfo,
		types.ScreenDatabases, types.ScreenLogs, types.ScreenFavorites:
		vPos = lipgloss.Top
		hPos = lipgloss.Left
	}

	// Avoid Place(0,0) on first frame (undefined layout / huge empty padding).
	if m.Width <= 0 || m.Height <= 0 {
		return fullContent
	}
	return lipgloss.Place(m.Width, m.Height, hPos, vPos, fullContent)
}

func (m Model) getStatusBar() string {
	if m.Screen == types.ScreenConnections || m.Screen == types.ScreenAddConnection ||
		m.Screen == types.ScreenEditConnection || m.Screen == types.ScreenHelp {
		return ""
	}

	if m.Screen == types.ScreenDatabases ||
		m.Screen == types.ScreenLogs || m.Screen == types.ScreenFavorites ||
		m.isWorkspaceScreen() {
		parts := []string{}
		if m.Loading {
			parts = append(parts, dimStyle.Render("loading…"))
		}
		if m.StatusMsg != "" {
			parts = append(parts, successStyle.Render(m.StatusMsg))
		}
		if m.Err != nil {
			parts = append(parts, errorStyle.Render(truncate(m.Err.Error(), max(m.Width-4, 20))))
		}
		if len(parts) == 0 {
			return ""
		}
		return " " + strings.Join(parts, dimStyle.Render(" · "))
	}
	parts := []string{}
	if m.Loading {
		parts = append(parts, dimStyle.Render("loading…"))
	}
	if m.StatusMsg != "" {
		parts = append(parts, successStyle.Render(m.StatusMsg))
	}
	if m.Err != nil {
		parts = append(parts, errorStyle.Render(m.Err.Error()))
	}
	if m.CurrentConn != nil {
		parts = append(parts, dimStyle.Render(m.CurrentConn.Name))
	}
	if m.CurrentDatabase != "" {
		parts = append(parts, accentStyle.Render(m.CurrentDatabase))
	}
	if m.CurrentSchema != "" {
		parts = append(parts, dimStyle.Render(m.CurrentSchema))
	}
	if m.ReadOnly {
		parts = append(parts, badgeROStyle.Render("RO"))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, dimStyle.Render(" · "))
}

func (m Model) viewTableDataContent(width, height int) string {
	var b strings.Builder
	title := "Data"
	if m.CurrentObject != nil {
		title = m.CurrentObject.FullName()
	}
	b.WriteString(headerStyle.Render(truncate(title, max(width-22, 8))))
	b.WriteString("  ")
	b.WriteString(tabActiveStyle.Render("Data"))
	b.WriteString(tabInactiveStyle.Render("Structure"))
	b.WriteByte('\n')

	res := m.TableData
	if m.Loading {
		b.WriteString(dimStyle.Render("Loading rows…"))
		b.WriteByte('\n')
		return b.String()
	}
	if res.Columns == nil {
		b.WriteString(dimStyle.Render("No data loaded."))
		b.WriteByte('\n')
		return b.String()
	}

	var meta string
	if len(res.Rows) == 0 {
		meta = "0 rows"
	} else {
		endRow := m.DataOffset + len(res.Rows)
		meta = fmt.Sprintf("%d–%d", m.DataOffset+1, endRow)
	}
	if m.CurrentObject != nil && m.CurrentObject.RowEstimate > 0 {
		meta += fmt.Sprintf(" of %s", compactInt(m.CurrentObject.RowEstimate))
	}
	meta += "  ·  " + res.Duration.Round(1e6).String()
	atFirst := m.DataOffset <= 0
	atLast := !m.tableDataHasMore()
	switch {
	case atFirst && !atLast:
		meta += "  · first page"
	case atLast && !atFirst:
		meta += "  · last page"
	}
	if res.Truncated {
		meta += "  · more"
	}
	b.WriteString(dimStyle.Render(meta))
	b.WriteByte('\n')
	b.WriteString(m.renderResultTable(res, m.DataCursor, m.DataCol, max(height-3, 4), width))
	return b.String()
}

func (m Model) viewTableDetailContent(width, height int) string {
	var b strings.Builder
	d := m.TableDetail
	name := ""
	kind := d.Object.Kind
	if m.CurrentObject != nil {
		name = m.CurrentObject.FullName()
		if kind == "" {
			kind = m.CurrentObject.Kind
		}
	}
	if name == "" || name == "." {
		name = d.Object.FullName()
	}
	if name == "" || name == "." {
		name = "Object"
	}
	b.WriteString(headerStyle.Render(truncate(name, max(width-36, 8))))
	b.WriteString("  ")
	if kind != "" {
		b.WriteString(kindStyle(string(kind)).Render(kindGlyph(string(kind)) + " " + string(kind)))
		b.WriteString("  ")
	}
	rel := isRelationObject(types.SchemaObject{Kind: kind})
	if rel {
		b.WriteString(tabInactiveStyle.Render("Data"))
		b.WriteString(tabActiveStyle.Render("Structure"))
	} else {
		b.WriteString(tabActiveStyle.Render("Detail"))
	}
	b.WriteByte('\n')

	detailMatches := m.CurrentObject == nil || objectIdentityMatch(m.CurrentObject, d.Object)
	if m.Loading || !detailMatches {
		if m.Loading {
			b.WriteString(dimStyle.Render("Loading…"))
		} else {
			b.WriteString(dimStyle.Render("Detail not loaded for selection."))
		}
		b.WriteByte('\n')
		return b.String()
	}

	tabs := m.detailTabs(d)
	tab := clamp(m.DetailTab, 0, max(len(tabs)-1, 0))
	for i, t := range tabs {
		if i == tab {
			b.WriteString(tabActiveStyle.Render(t))
		} else {
			b.WriteString(tabInactiveStyle.Render(t))
		}
	}
	b.WriteByte('\n')
	b.WriteString(gridSepStyle.Render(strings.Repeat("─", min(width, 80))))
	b.WriteByte('\n')

	switch tabs[tab] {
	case "Info":
		b.WriteString(m.renderDetailProps(d, width))
	case "Definition":
		b.WriteString(m.renderDetailDefinition(d, width, height))
	case "Indexes":
		if len(d.Indexes) == 0 {
			b.WriteString(dimStyle.Render("No indexes."))
			b.WriteByte('\n')
		}
		for _, idx := range d.Indexes {
			b.WriteString(accentStyle.Render(idx.Name))
			if idx.IsPrimary {
				b.WriteString(" ")
				b.WriteString(badgePKStyle.Render("PK"))
			} else if idx.IsUnique {
				b.WriteString(" ")
				b.WriteString(badgeUniqueStyle.Render("UNIQUE"))
			}
			if idx.SizePretty != "" {
				b.WriteString(" ")
				b.WriteString(sizeStyle.Render(idx.SizePretty))
			}
			b.WriteByte('\n')
			b.WriteString(dimStyle.Render("  " + truncate(idx.Definition, width-2)))
			b.WriteByte('\n')
		}
	case "Constraints":
		if len(d.Constraints) == 0 {
			b.WriteString(dimStyle.Render("No constraints."))
			b.WriteByte('\n')
		}
		for _, c := range d.Constraints {
			b.WriteString(accentStyle.Render(c.Name))
			b.WriteString(" ")
			b.WriteString(constraintBadgeStyle(c.Type).Render(c.Type))
			b.WriteByte('\n')
			b.WriteString(dimStyle.Render("  " + truncate(c.Definition, width-2)))
			b.WriteByte('\n')
		}
	default: // Columns
		if len(d.Columns) == 0 {
			b.WriteString(dimStyle.Render("No columns."))
			b.WriteByte('\n')
			if len(d.Props) > 0 {
				b.WriteByte('\n')
				b.WriteString(m.renderDetailProps(d, width))
			}
		} else {
			b.WriteString(m.renderDetailColumns(d, width, height))
		}
	}
	return b.String()
}

func (m Model) detailTabs(d types.TableDetail) []string {
	kind := d.Object.Kind
	if m.CurrentObject != nil && kind == "" {
		kind = m.CurrentObject.Kind
	}
	if !isRelationObject(types.SchemaObject{Kind: kind}) {
		tabs := []string{"Info"}
		if d.CreateSQL != "" {
			tabs = append(tabs, "Definition")
		}
		if len(d.Columns) > 0 {
			tabs = append([]string{"Columns"}, tabs...)
		}
		return tabs
	}
	tabs := []string{"Columns", "Indexes", "Constraints"}
	if d.CreateSQL != "" {
		tabs = append(tabs, "Definition")
	}
	return tabs
}

func (m Model) renderDetailProps(d types.TableDetail, width int) string {
	var b strings.Builder
	if len(d.Props) == 0 {
		b.WriteString(dimStyle.Render("No properties."))
		b.WriteByte('\n')
		return b.String()
	}
	labelW := 14
	for _, p := range d.Props {
		if w := displayWidth(p.Label); w+1 > labelW {
			labelW = min(w+1, 22)
		}
	}
	for _, p := range d.Props {
		b.WriteString(dimStyle.Render(padRight(p.Label, labelW)))
		b.WriteString(normalStyle.Render(truncate(p.Value, max(width-labelW-1, 8))))
		b.WriteByte('\n')
	}
	return b.String()
}

func (m Model) renderDetailDefinition(d types.TableDetail, width, height int) string {
	var b strings.Builder
	if d.CreateSQL == "" {
		b.WriteString(dimStyle.Render("No definition."))
		b.WriteByte('\n')
		return b.String()
	}
	maxLines := max(height-6, 4)
	lines := strings.Split(d.CreateSQL, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, "…")
	}
	for _, line := range lines {
		b.WriteString(typeJSONStyle.Render(truncate(line, width)))
		b.WriteByte('\n')
	}
	return b.String()
}

func (m Model) renderDetailColumns(d types.TableDetail, width, height int) string {
	var b strings.Builder
	nameW := min(max(width/4, 12), 28)
	typeW := min(max(width/3, 14), 28)
	defW := max(width-nameW-typeW-12, 6)
	var hdr strings.Builder
	hdr.WriteString(padRight("#", 3))
	hdr.WriteString("│")
	hdr.WriteString(padRight("COLUMN", nameW))
	hdr.WriteString("│")
	hdr.WriteString(padRight("TYPE", typeW))
	hdr.WriteString("│")
	hdr.WriteString(padRight("NULL", 4))
	hdr.WriteString("│")
	hdr.WriteString(padRight("DEFAULT", defW))
	b.WriteString(tableHeaderStyle.Render(padRight(hdr.String(), width)))
	b.WriteByte('\n')
	sep := gridSepStyle.Render("│")
	maxVisible := max(height-5, 4)
	end := min(len(d.Columns), maxVisible)
	for i, col := range d.Columns[:end] {
		nullStyled := nullNoStyle.Render(padRight("NO", 4))
		if col.IsNullable {
			nullStyled = nullYesStyle.Render(padRight("YES", 4))
		}
		var nameStyled string
		switch {
		case col.IsPrimaryKey:
			nameStyled = accentStyle.Bold(true).Render(padRight("◆ "+col.Name, nameW))
		case strings.HasSuffix(col.Name, "_id"):
			nameStyled = fkColStyle.Render(padRight(col.Name, nameW))
		default:
			nameStyled = normalStyle.Render(padRight(col.Name, nameW))
		}
		typeStyled := pgTypeStyle(col.DataType).Render(padRight(col.DataType, typeW))
		defPlain := padRight(truncate(col.Default, defW), defW)
		defStyled := dimStyle.Render(defPlain)
		if col.Default != "" {
			defStyled = cellValueStyle(col.Default).Render(defPlain)
		}
		posPlain := padRight(fmtInt(col.Position), 3)
		pos := dimStyle.Render(posPlain)
		if i%2 == 1 {
			pos = zebraStyle.Foreground(lipgloss.Color(colorDim)).Render(posPlain)
		}
		b.WriteString(pos)
		b.WriteString(sep)
		b.WriteString(nameStyled)
		b.WriteString(sep)
		b.WriteString(typeStyled)
		b.WriteString(sep)
		b.WriteString(nullStyled)
		b.WriteString(sep)
		b.WriteString(defStyled)
		b.WriteByte('\n')
	}
	return b.String()
}

func (m Model) viewQueryContent(width, height int) string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("SQL Query"))
	b.WriteString(dimStyle.Render("  ·  ; open  ·  ctrl+enter / ctrl+e run  ·  tab complete  ·  ↑↓ cycle  ·  esc back"))
	b.WriteByte('\n')

	editorFocused := m.QueryFocus == "editor" && m.Focus == focusContent
	// Leave room for help + suggestions + results.
	editorH := min(max(height/4, 5), 12)
	if m.QueryArea != nil {
		m.QueryArea.SetWidth(max(width-6, 20))
		m.QueryArea.SetHeight(editorH)
		m.QueryArea.ShowLineNumbers = false
		m.QueryArea.Prompt = ""
		label := "editor"
		if editorFocused {
			label = "EDITOR"
		}
		b.WriteString(sectionStyle.Render(label))
		b.WriteString(dimStyle.Render("  kw "))
		b.WriteString(sqlKwStyle.Render("SELECT"))
		b.WriteString(dimStyle.Render("  str "))
		b.WriteString(sqlStrStyle.Render("'…'"))
		b.WriteString(dimStyle.Render("  num "))
		b.WriteString(sqlNumStyle.Render("42"))
		b.WriteString(dimStyle.Render("  fn "))
		b.WriteString(sqlFnStyle.Render("count"))
		b.WriteByte('\n')
		border := colorBorder
		if editorFocused {
			border = colorAccent
		}
		// Syntax-highlighted view (textarea still owns input/cursor state).
		body := m.renderHighlightedSQLEditor(max(width-4, 20), editorH, editorFocused)
		box := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(border)).
			Padding(0, 1).
			Width(max(width-2, 20)).
			Render(body)
		b.WriteString(box)
		b.WriteByte('\n')
	}

	// Autocomplete strip (schema tables/columns + keywords).
	if editorFocused && len(m.QuerySuggests) > 0 {
		b.WriteString(m.renderQuerySuggestions(width))
		b.WriteByte('\n')
	} else if editorFocused {
		b.WriteString(dimStyle.Render(m.queryHelpLine(width)))
		b.WriteByte('\n')
	}

	resLabel := "results"
	if m.QueryFocus == "results" {
		resLabel = "RESULTS"
	}
	b.WriteString(sectionStyle.Render(resLabel))
	b.WriteByte('\n')

	used := lipglossHeight(b.String())
	resH := max(height-used-1, 3)

	if m.Loading {
		b.WriteString(dimStyle.Render("Running…"))
		b.WriteByte('\n')
	} else if m.QueryResult.Columns == nil && m.QueryResult.RowsAffected == 0 && m.QueryResult.SQL == "" {
		b.WriteString(dimStyle.Render("Type SQL above · ctrl+enter to run · examples:"))
		b.WriteByte('\n')
		b.WriteString(dimStyle.Render("  SELECT * FROM public.users LIMIT 50;"))
		b.WriteByte('\n')
		b.WriteString(dimStyle.Render("  SELECT u.email, o.total_cents FROM users u JOIN orders o ON o.user_id = u.id;"))
		b.WriteByte('\n')
		b.WriteString(dimStyle.Render("  EXPLAIN ANALYZE SELECT …;"))
		b.WriteByte('\n')
	} else if !m.QueryResult.IsSelect && m.QueryResult.SQL != "" {
		b.WriteString(successStyle.Render(fmt.Sprintf("OK — %d rows affected · %s",
			m.QueryResult.RowsAffected, m.QueryResult.Duration.Round(1e6))))
		b.WriteByte('\n')
	} else {
		b.WriteString(dimStyle.Render(fmt.Sprintf("%d rows · %s",
			len(m.QueryResult.Rows), m.QueryResult.Duration.Round(1e6))))
		if m.QueryResult.Truncated {
			b.WriteString(yellowStyle.Render("  truncated"))
		}
		b.WriteByte('\n')
		b.WriteString(m.renderResultTable(m.QueryResult, m.DataCursor, m.DataCol, resH-1, width))
	}
	return b.String()
}

func (m Model) queryHelpLine(width int) string {
	nTbl, nCol := 0, 0
	if m.SQLCompleter != nil {
		nTbl = len(m.SQLCompleter.tables.disp)
		nCol = len(m.SQLCompleter.columns.disp)
	}
	msg := fmt.Sprintf("smart complete · %d tables · %d cols · clause-aware · tab accept", nTbl, nCol)
	return truncate(msg, max(width-1, 20))
}

func (m Model) renderQuerySuggestions(width int) string {
	if len(m.QuerySuggests) == 0 {
		return ""
	}
	var parts []string
	idx := clamp(m.QuerySuggestIdx, 0, len(m.QuerySuggests)-1)
	for i, s := range m.QuerySuggests {
		chip := s
		if i == idx {
			parts = append(parts, selectedStyle.Render(" "+chip+" "))
		} else {
			parts = append(parts, badgeStyle.Render(chip))
		}
	}
	line := " " + strings.Join(parts, " ")
	// Keep single visual line.
	if lipglossWidth(line) > width {
		// show selected + neighbors only
		lo := max(idx-2, 0)
		hi := min(idx+3, len(m.QuerySuggests))
		parts = parts[:0]
		for i := lo; i < hi; i++ {
			s := m.QuerySuggests[i]
			if i == idx {
				parts = append(parts, selectedStyle.Render(" "+s+" "))
			} else {
				parts = append(parts, badgeStyle.Render(s))
			}
		}
		line = " " + strings.Join(parts, " ") + dimStyle.Render(" …")
	}
	return dimStyle.Render("suggest") + line
}

// renderResultTable draws a dense spreadsheet-like grid with cell cursor.
func (m Model) renderResultTable(res types.QueryResult, cursorRow, cursorCol, maxRows, maxWidth int) string {
	if len(res.Columns) == 0 {
		return dimStyle.Render("(no columns)\n")
	}
	if maxWidth < 20 {
		maxWidth = 20
	}

	const maxColW = 24
	rnW := max(len(fmt.Sprintf("%d", max(m.DataOffset+len(res.Rows), 1))), 3)

	avail := maxWidth - rnW - 1
	nCol := len(res.Columns)
	colW := make([]int, nCol)
	for i, c := range res.Columns {
		colW[i] = min(max(displayWidth(c), 4), maxColW)
	}
	sample := res.Rows
	if len(sample) > 40 {
		sample = sample[:40]
	}
	for _, row := range sample {
		for i, cell := range row {
			if i >= nCol {
				break
			}
			disp := cell
			if disp == "" {
				disp = "NULL"
			}
			w := min(displayWidth(disp), maxColW)
			if w > colW[i] {
				colW[i] = w
			}
		}
	}

	if avail > nCol {
		used := 0
		visible := 0
		for i := 0; i < nCol; i++ {
			need := colW[i] + 1
			if used+need > avail && visible > 0 {
				colW = colW[:visible]
				break
			}
			if used+need > avail {
				colW[i] = max(avail-used-1, 3)
				visible = i + 1
				colW = colW[:visible]
				break
			}
			used += need
			visible++
		}
	}
	nShow := len(colW)
	if cursorCol >= nShow {
		cursorCol = nShow - 1
	}
	if cursorCol < 0 {
		cursorCol = 0
	}

	sep := gridSepStyle.Render("│")
	var b strings.Builder

	var hdr strings.Builder
	hdr.WriteString(padRight("#", rnW))
	for i := 0; i < nShow; i++ {
		hdr.WriteString("│")
		name := res.Columns[i]
		hdr.WriteString(padRight(name, colW[i]))
	}
	b.WriteString(tableHeaderStyle.Render(padRight(hdr.String(), maxWidth)))
	b.WriteByte('\n')

	var rule strings.Builder
	rule.WriteString(strings.Repeat("─", rnW))
	for i := 0; i < nShow; i++ {
		rule.WriteString("┼")
		rule.WriteString(strings.Repeat("─", colW[i]))
	}
	b.WriteString(gridSepStyle.Render(truncate(rule.String(), maxWidth)))
	b.WriteByte('\n')

	start, end := listWindow(cursorRow, len(res.Rows), maxRows)
	for i := start; i < end; i++ {
		row := res.Rows[i]
		rowNum := fmt.Sprintf("%d", m.DataOffset+i+1)

		rawCells := make([]string, nShow)
		plainCells := make([]string, 0, nShow+1)
		plainCells = append(plainCells, padRight(rowNum, rnW))
		for j := 0; j < nShow; j++ {
			cell := ""
			if j < len(row) {
				cell = row[j]
			}
			rawCells[j] = cell
			disp := cell
			if cell == "" {
				disp = "NULL"
			}
			plainCells = append(plainCells, padRight(truncate(disp, colW[j]), colW[j]))
		}

		if i == cursorRow {
			// Continuous blue selection band (es-tui style), including separators.
			selSep := selectedStyle.Render("│")
			parts := make([]string, 0, len(plainCells)*2)
			parts = append(parts, selectedStyle.Render(plainCells[0]))
			for j := 0; j < nShow; j++ {
				parts = append(parts, selSep)
				text := plainCells[j+1]
				if j == cursorCol {
					parts = append(parts, cellSelectedStyle.Render(text))
				} else if rawCells[j] == "" {
					parts = append(parts, selectedStyle.Italic(true).Render(text))
				} else {
					parts = append(parts, selectedStyle.Render(text))
				}
			}
			b.WriteString(strings.Join(parts, ""))
		} else {
			parts := make([]string, 0, len(plainCells)*2)
			num := dimStyle.Render(plainCells[0])
			if i%2 == 1 {
				num = zebraStyle.Foreground(lipgloss.Color(colorDim)).Render(plainCells[0])
			}
			parts = append(parts, num)
			for j := 0; j < nShow; j++ {
				parts = append(parts, sep)
				text := plainCells[j+1]
				raw := rawCells[j]
				st := cellValueStyle(raw)
				if raw == "" {
					if i%2 == 1 {
						parts = append(parts, zebraStyle.Foreground(lipgloss.Color(colorDim)).Italic(true).Render(text))
					} else {
						parts = append(parts, nullCellStyle.Render(text))
					}
					continue
				}
				if i%2 == 1 {
					parts = append(parts, zebraStyle.Foreground(st.GetForeground()).Render(text))
				} else {
					parts = append(parts, st.Render(text))
				}
			}
			b.WriteString(strings.Join(parts, ""))
		}
		b.WriteByte('\n')
	}

	if nShow < len(res.Columns) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("… +%d cols", len(res.Columns)-nShow)))
		b.WriteByte('\n')
	}
	return b.String()
}

func (m Model) viewActivityContent(width, height int) string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Activity"))
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %d backends", len(m.Activity))))
	b.WriteByte('\n')
	b.WriteString(gridSepStyle.Render(strings.Repeat("─", min(width, 80))))
	b.WriteByte('\n')

	if m.Loading && len(m.Activity) == 0 {
		b.WriteString(dimStyle.Render("Loading…"))
		b.WriteByte('\n')
		return b.String()
	}
	if len(m.Activity) == 0 {
		b.WriteString(dimStyle.Render("No client backends."))
		b.WriteByte('\n')
		return b.String()
	}

	pidW, userW, dbW, stateW, durW := 7, 10, 12, 10, 8
	fixed := pidW + userW + dbW + stateW + durW + 5
	qW := max(width-fixed, 10)

	hdr := fmt.Sprintf("%s %s %s %s %s %s",
		padRight("PID", pidW),
		padRight("USER", userW),
		padRight("DB", dbW),
		padRight("STATE", stateW),
		padRight("DUR", durW),
		padRight("QUERY", qW),
	)
	b.WriteString(tableHeaderStyle.Render(padRight(hdr, width)))
	b.WriteByte('\n')

	maxVisible := max(height-4, 4)
	sel := clamp(m.SelectedActIdx, 0, len(m.Activity)-1)
	start, end := listWindow(sel, len(m.Activity), maxVisible)
	for i := start; i < end; i++ {
		a := m.Activity[i]
		user := dashEmpty(a.User)
		db := dashEmpty(a.Database)
		state := dashEmpty(shortState(a.State))
		dur := formatActivityDur(a.State, a.Duration)
		query := strings.ReplaceAll(a.Query, "\n", " ")
		query = strings.TrimSpace(query)
		if query == "" {
			query = "—"
		}
		plain := fmt.Sprintf("%s %s %s %s %s %s",
			padRight(fmtInt(a.PID), pidW),
			padRight(user, userW),
			padRight(db, dbW),
			padRight(state, stateW),
			padRight(dur, durW),
			padRight(truncate(query, qW), qW),
		)
		if i == sel {
			b.WriteString(selectedStyle.Render(padRight(plain, width)))
			b.WriteByte('\n')
			continue
		}

		pidPlain := padRight(fmtInt(a.PID), pidW)
		userPlain := padRight(user, userW)
		dbPlain := padRight(db, dbW)
		statePlain := padRight(state, stateW)
		durPlain := padRight(dur, durW)
		queryPlain := padRight(truncate(query, qW), qW)

		stateSt := activityStateStyle(a.State)
		durSt := dimStyle
		if dur != "—" {
			durSt = durationStyle(a.Duration)
		}
		querySt := normalStyle
		if strings.EqualFold(strings.TrimSpace(a.State), "idle") {
			querySt = dimStyle
		}

		zebra := i%2 == 1
		paint := func(st lipgloss.Style, text string) string {
			if zebra {
				return zebraStyle.Foreground(st.GetForeground()).Render(text)
			}
			return st.Render(text)
		}

		var line strings.Builder
		line.WriteString(paint(dimStyle, pidPlain))
		line.WriteByte(' ')
		line.WriteString(paint(normalStyle, userPlain))
		line.WriteByte(' ')
		line.WriteString(paint(schemaNameStyle, dbPlain))
		line.WriteByte(' ')
		line.WriteString(paint(stateSt, statePlain))
		line.WriteByte(' ')
		line.WriteString(paint(durSt, durPlain))
		line.WriteByte(' ')
		line.WriteString(paint(querySt, queryPlain))
		b.WriteString(line.String())
		b.WriteByte('\n')
	}
	return b.String()
}

func dashEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func shortState(state string) string {
	s := strings.TrimSpace(state)
	switch strings.ToLower(s) {
	case "idle in transaction":
		return "idle-xact"
	case "idle in transaction (aborted)":
		return "xact-abort"
	case "fastpath function call":
		return "fastpath"
	default:
		return s
	}
}

func formatActivityDur(state string, d time.Duration) string {
	st := strings.ToLower(strings.TrimSpace(state))
	if st == "" || st == "idle" {
		return "—"
	}
	return compactDuration(d)
}

func compactDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Millisecond:
		return "0ms"
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		if s == 0 {
			return fmt.Sprintf("%dm", m)
		}
		return fmt.Sprintf("%dm%02ds", m, s)
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh%02dm", h, m)
	default:
		days := int(d.Hours()) / 24
		h := int(d.Hours()) % 24
		if h == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd%dh", days, h)
	}
}

func (m Model) viewServerInfoContent(width, height int) string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Server"))
	b.WriteByte('\n')
	b.WriteString(gridSepStyle.Render(strings.Repeat("─", min(width, 80))))
	b.WriteByte('\n')

	info := m.ServerInfo
	rows := []struct {
		k, v string
		st   lipgloss.Style
	}{
		{"Version", info.Version, accentStyle},
		{"User", info.User, normalStyle},
		{"Database", info.Database, schemaNameStyle},
		{"Host", fmt.Sprintf("%s:%d", info.Host, info.Port), normalStyle},
		{"Encoding", info.Encoding, typeTextStyle},
		{"Timezone", info.Timezone, timeCellStyle},
		{"Uptime", info.Uptime, sizeStyle},
		{"Connections", fmt.Sprintf("%d / %d", info.ActiveConns, info.MaxConns), numCellStyle},
	}
	_ = height
	for _, r := range rows {
		if r.v == "" {
			continue
		}
		b.WriteString(dimStyle.Render(padRight(r.k, 14)))
		b.WriteString(r.st.Render(truncate(r.v, max(width-15, 10))))
		b.WriteByte('\n')
	}
	return b.String()
}

// Legacy full-screen wrappers (tests / direct calls).
func (m Model) viewTableData() string   { return m.viewWorkspace() }
func (m Model) viewTableDetail() string { return m.viewWorkspace() }
func (m Model) viewQuery() string       { return m.viewWorkspace() }
func (m Model) viewActivity() string    { return m.viewWorkspace() }
func (m Model) viewERD() string         { return m.viewWorkspace() }
func (m Model) viewServerInfo() string  { return m.viewWorkspace() }

func (m Model) viewHelp() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Help"))
	b.WriteString(dimStyle.Render("  ·  postgres-tui"))
	b.WriteString("\n\n")

	sections := []struct {
		title string
		keys  [][2]string
	}{
		{"Global", [][2]string{
			{"q", "Quit"},
			{"ctrl+c", "Force quit"},
			{"?", "Toggle help"},
			{"esc", "Back"},
			{"ctrl+p", "Palette"},
			{"j/k", "Move"},
			{"g/G", "Top/bottom"},
		}},
		{"Connections", [][2]string{
			{"enter", "Connect"},
			{"a", "Add"},
			{"e", "Edit"},
			{"d", "Delete"},
			{"t", "Test"},
			{"up/down", "Navigate"},
		}},
		{"Workspace", [][2]string{
			{"tab", "Cycle panes"},
			{"enter", "Open item"},
			{"space", "Toggle filter"},
			{"1-6", "Kind filters"},
			{"/", "Search"},
			{";", "SQL editor"},
			{":", "SQL editor"},
			{"D", "Structure"},
			{"A", "Activity"},
			{"E", "ER diagram"},
			{"i", "Server info"},
			{"r", "Refresh"},
			{"esc", "Back"},
		}},
		{"Table data", [][2]string{
			{"h/l", "Move cell"},
			{"y", "Copy cell"},
			{"Y", "Copy row"},
			{"n/p", "Page"},
			{"D", "Structure"},
			{"x", "Export CSV"},
		}},
		{"Query", [][2]string{
			{";", "Open editor"},
			{"ctrl+enter", "Run SQL"},
			{"ctrl+e", "Run SQL"},
			{"tab", "Complete/focus"},
			{"↑↓", "Cycle suggests"},
			{"esc", "Dismiss/back"},
			{"x", "Export CSV"},
		}},
	}

	// Two-column binding rows; keep desc short so lines never wrap.
	const keyCol = 12
	const bindCol = 32
	colStyle := lipgloss.NewStyle().Width(bindCol)

	for _, sec := range sections {
		b.WriteString(accentStyle.Bold(true).Render(sec.title))
		b.WriteString("\n")
		half := (len(sec.keys) + 1) / 2
		var left, right strings.Builder
		for i, binding := range sec.keys {
			line := helpBindingLine(binding[0], binding[1], keyCol)
			if i < half {
				left.WriteString(line)
				left.WriteString("\n")
			} else {
				right.WriteString(line)
				right.WriteString("\n")
			}
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			colStyle.Render(strings.TrimRight(left.String(), "\n")),
			colStyle.Render(strings.TrimRight(right.String(), "\n")),
		))
		b.WriteString("\n\n")
	}

	b.WriteString(renderKeyHelp([]struct{ key, desc string }{
		{"?", "close"},
		{"esc", "close"},
	}))

	modalWidth := min(80, max(m.Width-6, 50))
	if m.Width <= 0 {
		modalWidth = 72
	}
	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorSelectBg)).
		Padding(1, 2).
		Width(modalWidth).
		Render(strings.TrimRight(b.String(), "\n"))
	return modal
}

// helpBindingLine renders a fixed-width key chip + single-line description.
func helpBindingLine(key, desc string, keyWidth int) string {
	chip := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(lipgloss.Color("255")).
		Padding(0, 1).
		Render(key)
	pad := keyWidth - lipgloss.Width(chip)
	if pad < 1 {
		pad = 1
	}
	// Hard-cap desc so JoinHorizontal columns never wrap mid-row.
	return chip + strings.Repeat(" ", pad) + descStyle.Render(truncate(desc, 18))
}

func (m Model) viewLogs() string {
	var b strings.Builder
	var logs []string
	if m.Logs != nil {
		logs = m.Logs.GetLogs()
	}
	innerW := max(m.Width-2, 20)
	if len(logs) == 0 {
		b.WriteString(dimStyle.Render(" No log entries."))
		b.WriteByte('\n')
	} else {
		hdr := padRight("LOG", innerW)
		b.WriteString(tableHeaderStyle.Render(hdr))
		b.WriteByte('\n')
		start := 0
		maxVisible := max(m.Height-8, 5)
		if len(logs) > maxVisible {
			start = len(logs) - maxVisible
		}
		for i, line := range logs[start:] {
			text := " " + truncate(strings.TrimSpace(line), innerW-1)
			padded := padRight(text, innerW)
			if i%2 == 1 {
				b.WriteString(zebraStyle.Render(padded))
			} else {
				b.WriteString(dimStyle.Render(padded))
			}
			b.WriteByte('\n')
		}
	}
	return m.denseListFrame("Logs", b.String(), keyDesc{"esc", "back"})
}

func (m Model) viewFavorites() string {
	var b strings.Builder
	innerW := max(m.Width-2, 20)
	if len(m.Favorites) == 0 {
		b.WriteString(dimStyle.Render(" No favorites yet."))
		b.WriteByte('\n')
	} else {
		nameW := min(max(innerW/2, 16), 36)
		hdr := fmt.Sprintf("%s  %s  %s",
			padRight("OBJECT", nameW),
			padRight("SCHEMA", 16),
			padRight("DATABASE", 16),
		)
		b.WriteString(tableHeaderStyle.Render(padRight(hdr, innerW)))
		b.WriteByte('\n')
		for i, f := range m.Favorites {
			line := fmt.Sprintf("%s  %s  %s",
				padRight(f.Object, nameW),
				padRight(f.Schema, 16),
				padRight(f.Database, 16),
			)
			padded := padRight(line, innerW)
			switch {
			case i == m.SelectedFavIdx:
				b.WriteString(selectedStyle.Render(padded))
			case i%2 == 1:
				b.WriteString(zebraStyle.Render(padded))
			default:
				b.WriteString(normalStyle.Render(padded))
			}
			b.WriteByte('\n')
		}
	}
	return m.denseListFrame("Favorites", b.String(),
		keyDesc{"↑/↓", "navigate"},
		keyDesc{"enter", "open"},
		keyDesc{"esc", "back"},
	)
}

func (m Model) viewExport() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Export CSV"))
	b.WriteString("\n\n")
	if m.Inputs != nil {
		b.WriteString(m.Inputs.ExportInput.View())
	}
	b.WriteString("\n\n")
	b.WriteString(renderKeyHelp([]struct{ key, desc string }{
		{"enter", "export"},
		{"esc", "cancel"},
	}))
	return m.formModal(b.String())
}

func (m Model) viewCommandPalette() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Command Palette"))
	b.WriteString("\n\n")
	if m.Inputs != nil {
		b.WriteString(m.Inputs.PaletteInput.View())
		b.WriteString("\n\n")
	}
	filter := ""
	if m.Inputs != nil {
		filter = strings.ToLower(m.Inputs.PaletteInput.Value())
	}
	var items []PaletteItem
	for _, it := range m.PaletteItems {
		if filter == "" || strings.Contains(strings.ToLower(it.Label), filter) ||
			strings.Contains(strings.ToLower(it.Group), filter) {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		b.WriteString(dimStyle.Render("No matches"))
	} else {
		idx := clamp(m.PaletteIdx, 0, len(items)-1)
		for i, it := range items {
			line := fmt.Sprintf("%s  %s", padRight(it.Label, 36), it.Keys)
			if i == idx {
				b.WriteString(selectedStyle.Render(padRight(line, 48)))
			} else {
				b.WriteString(dimStyle.Render(line))
			}
			b.WriteString("\n")
		}
	}
	return m.formModal(b.String())
}
