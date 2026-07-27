package ui

import (
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
)

// SQL syntax colors (inline with datatype palette).
var (
	sqlKwStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true) // sky — keywords
	sqlFnStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))           // violet — functions
	sqlStrStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))           // green — strings
	sqlNumStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))            // cyan — numbers
	sqlCommentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Italic(true)
	sqlIdentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // identifiers
	sqlOpStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("180")) // gold — ops / *
	sqlPunctStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	sqlCursorStyle  = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("16")).
			Background(lipgloss.Color("39"))
	sqlCursorLineStyle = lipgloss.NewStyle().Background(lipgloss.Color("236"))
)

// Common SQL functions get distinct color from keywords.
var sqlFunctions = map[string]struct{}{
	"count": {}, "sum": {}, "avg": {}, "min": {}, "max": {},
	"coalesce": {}, "nullif": {}, "cast": {}, "now": {}, "current_timestamp": {},
	"current_date": {}, "current_time": {}, "extract": {}, "date_trunc": {},
	"upper": {}, "lower": {}, "length": {}, "trim": {}, "substring": {},
	"round": {}, "abs": {}, "greatest": {}, "least": {}, "exists": {},
	"array_agg": {}, "string_agg": {}, "jsonb_build_object": {}, "json_agg": {},
	"pg_typeof": {}, "to_char": {}, "to_date": {}, "to_timestamp": {},
	"nextval": {}, "currval": {}, "generate_series": {},
}

// highlightSQL returns the full SQL string with lipgloss color spans (no cursor).
func highlightSQL(sql string) string {
	if sql == "" {
		return ""
	}
	lines := highlightSQLLines(sql)
	return strings.Join(lines, "\n")
}

// highlightSQLLines colorizes each line; multi-line comments keep state.
func highlightSQLLines(sql string) []string {
	if sql == "" {
		return []string{""}
	}
	raw := strings.Split(sql, "\n")
	out := make([]string, len(raw))
	inBlock := false
	for i, line := range raw {
		var colored string
		colored, inBlock = highlightSQLLineState(line, inBlock, -1, false)
		out[i] = colored
	}
	return out
}

// highlightSQLLineState colorizes one line.
// cursorCol >= 0 paints that rune as the cursor (and uses cursor-line wash when cursorLine).
func highlightSQLLineState(line string, inBlockComment bool, cursorCol int, cursorLine bool) (string, bool) {
	if line == "" {
		if cursorLine && cursorCol >= 0 {
			return sqlCursorStyle.Render(" "), inBlockComment
		}
		return "", inBlockComment
	}

	runes := []rune(line)
	var b strings.Builder
	i := 0
	for i < len(runes) {
		// Block comment continuation / start
		if inBlockComment {
			start := i
			for i < len(runes) {
				if i+1 < len(runes) && runes[i] == '*' && runes[i+1] == '/' {
					i += 2
					inBlockComment = false
					break
				}
				i++
			}
			b.WriteString(paintSegment(string(runes[start:i]), sqlCommentStyle, start, cursorCol, cursorLine))
			continue
		}
		if i+1 < len(runes) && runes[i] == '/' && runes[i+1] == '*' {
			start := i
			i += 2
			inBlockComment = true
			for i < len(runes) {
				if i+1 < len(runes) && runes[i] == '*' && runes[i+1] == '/' {
					i += 2
					inBlockComment = false
					break
				}
				i++
			}
			b.WriteString(paintSegment(string(runes[start:i]), sqlCommentStyle, start, cursorCol, cursorLine))
			continue
		}
		// Line comment
		if i+1 < len(runes) && runes[i] == '-' && runes[i+1] == '-' {
			b.WriteString(paintSegment(string(runes[i:]), sqlCommentStyle, i, cursorCol, cursorLine))
			break
		}
		// Single-quoted string
		if runes[i] == '\'' {
			start := i
			i++
			for i < len(runes) {
				if runes[i] == '\'' {
					i++
					// escaped ''
					if i < len(runes) && runes[i] == '\'' {
						i++
						continue
					}
					break
				}
				i++
			}
			b.WriteString(paintSegment(string(runes[start:i]), sqlStrStyle, start, cursorCol, cursorLine))
			continue
		}
		// Double-quoted identifier
		if runes[i] == '"' {
			start := i
			i++
			for i < len(runes) {
				if runes[i] == '"' {
					i++
					break
				}
				i++
			}
			b.WriteString(paintSegment(string(runes[start:i]), sqlIdentStyle.Bold(true), start, cursorCol, cursorLine))
			continue
		}
		// Number
		if unicode.IsDigit(runes[i]) || (runes[i] == '.' && i+1 < len(runes) && unicode.IsDigit(runes[i+1])) {
			start := i
			if runes[i] == '.' {
				i++
			}
			for i < len(runes) && (unicode.IsDigit(runes[i]) || runes[i] == '.' || runes[i] == 'e' || runes[i] == 'E' ||
				((runes[i] == '+' || runes[i] == '-') && i > start && (runes[i-1] == 'e' || runes[i-1] == 'E'))) {
				i++
			}
			b.WriteString(paintSegment(string(runes[start:i]), sqlNumStyle, start, cursorCol, cursorLine))
			continue
		}
		// Ident / keyword
		if unicode.IsLetter(runes[i]) || runes[i] == '_' {
			start := i
			for i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			word := string(runes[start:i])
			st := sqlIdentStyle
			low := strings.ToLower(word)
			if isSQLKeyword(word) {
				st = sqlKwStyle
			} else if _, ok := sqlFunctions[low]; ok {
				st = sqlFnStyle
			}
			b.WriteString(paintSegment(word, st, start, cursorCol, cursorLine))
			continue
		}
		// Operators / star
		if strings.ContainsRune("=<>!+-*/%|&^~", runes[i]) || runes[i] == '*' {
			b.WriteString(paintSegment(string(runes[i]), sqlOpStyle, i, cursorCol, cursorLine))
			i++
			continue
		}
		// Punctuation / whitespace
		st := sqlPunctStyle
		if unicode.IsSpace(runes[i]) {
			st = lipgloss.NewStyle()
		}
		b.WriteString(paintSegment(string(runes[i]), st, i, cursorCol, cursorLine))
		i++
	}

	// Cursor past end of line
	if cursorLine && cursorCol >= len(runes) {
		b.WriteString(sqlCursorStyle.Render(" "))
	}
	return b.String(), inBlockComment
}

// paintSegment applies style, splitting out the cursor cell when needed.
func paintSegment(seg string, st lipgloss.Style, start, cursorCol int, cursorLine bool) string {
	if seg == "" {
		return ""
	}
	if !cursorLine || cursorCol < 0 {
		if cursorLine {
			return sqlCursorLineStyle.Foreground(st.GetForeground()).Render(seg)
		}
		return st.Render(seg)
	}
	runes := []rune(seg)
	end := start + len(runes)
	if cursorCol < start || cursorCol >= end {
		if cursorLine {
			return sqlCursorLineStyle.Foreground(st.GetForeground()).Render(seg)
		}
		return st.Render(seg)
	}
	// cursor inside segment
	off := cursorCol - start
	var b strings.Builder
	if off > 0 {
		left := string(runes[:off])
		if cursorLine {
			b.WriteString(sqlCursorLineStyle.Foreground(st.GetForeground()).Render(left))
		} else {
			b.WriteString(st.Render(left))
		}
	}
	cur := string(runes[off])
	b.WriteString(sqlCursorStyle.Render(cur))
	if off+1 < len(runes) {
		right := string(runes[off+1:])
		if cursorLine {
			b.WriteString(sqlCursorLineStyle.Foreground(st.GetForeground()).Render(right))
		} else {
			b.WriteString(st.Render(right))
		}
	}
	return b.String()
}

// renderHighlightedSQLEditor draws a syntax-highlighted editor with line numbers + cursor.
func (m Model) renderHighlightedSQLEditor(width, height int, focused bool) string {
	if m.QueryArea == nil {
		return dimStyle.Render("(no editor)")
	}
	ta := m.QueryArea
	raw := ta.Value()
	lines := strings.Split(raw, "\n")
	if raw == "" {
		lines = []string{""}
	}

	// Keep textarea metrics in sync for input handling.
	lnW := 4
	contentW := max(width-lnW-1, 10)
	ta.SetWidth(contentW + lnW + 1) // include gutter so wrap math stays sane
	ta.SetHeight(height)
	ta.ShowLineNumbers = false
	ta.Prompt = ""

	curLine := ta.Line()
	curCol := ta.Column()
	if curLine < 0 {
		curLine = 0
	}
	if curLine >= len(lines) {
		curLine = len(lines) - 1
	}

	// Simple viewport around cursor
	start := 0
	if len(lines) > height {
		start = curLine - height/2
		if start < 0 {
			start = 0
		}
		if start+height > len(lines) {
			start = len(lines) - height
		}
	}
	end := min(start+height, len(lines))

	// Track block-comment state up to visible start
	inBlock := false
	for i := 0; i < start; i++ {
		_, inBlock = highlightSQLLineState(lines[i], inBlock, -1, false)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		plain := lines[i]
		ellipsis := false
		if displayWidth(plain) > contentW {
			plain = truncate(plain, contentW)
			// truncate adds … ; strip for tokenizer then append dim ellipsis after colorize
			if strings.HasSuffix(plain, "…") {
				plain = strings.TrimSuffix(plain, "…")
				ellipsis = true
			}
		}
		isCur := focused && i == curLine
		col := -1
		if isCur {
			col = curCol
			if col > len([]rune(plain)) {
				col = len([]rune(plain))
			}
		}
		var hi string
		hi, inBlock = highlightSQLLineState(plain, inBlock, col, isCur)
		if ellipsis {
			hi += dimStyle.Render("…")
		}
		lnNum := padLeft(fmtInt(i+1), lnW-1)
		ln := dimStyle.Render(lnNum) + " "
		if isCur && focused {
			ln = accentStyle.Render(lnNum) + " "
		}
		b.WriteString(ln)
		b.WriteString(hi)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	visible := end - start
	for visible < height {
		b.WriteByte('\n')
		b.WriteString(dimStyle.Render(padLeft("~", lnW-1)) + " ")
		visible++
	}
	return b.String()
}
