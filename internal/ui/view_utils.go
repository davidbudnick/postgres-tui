package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

type keyDesc struct {
	Key, Desc string
}

func (m Model) fullScreenFrame(body string, keys ...keyDesc) string {
	content := strings.TrimRight(body, "\n")
	if len(keys) > 0 {
		content = content + "\n" + renderKeyHelpWidth(max(m.Width-2, 20), toKeyPairs(keys))
	}
	return content
}

func (m Model) denseListFrame(title, body string, keys ...keyDesc) string {
	var b strings.Builder
	left := " " + titleStyle.Render(title)
	gap := m.Width - lipglossWidth(left)
	if gap < 1 {
		gap = 1
	}
	b.WriteString(left + strings.Repeat(" ", gap))
	b.WriteByte('\n')

	footer := ""
	if len(keys) > 0 {
		footer = renderKeyHelpWidth(max(m.Width-2, 20), toKeyPairs(keys))
	}
	chrome := 2 + lipglossHeight(footer)
	if m.Loading || m.StatusMsg != "" || m.Err != nil {
		chrome++
	}
	panelH := max(m.Height-chrome, 5)
	panelW := max(m.Width, 40)
	innerH := max(panelH-2, 3)
	panelBody := clampLines(strings.TrimRight(body, "\n"), innerH)
	b.WriteString(panelStyle(true, panelW, panelH).Render(panelBody))
	if footer != "" {
		b.WriteByte('\n')
		b.WriteString(footer)
	}
	return b.String()
}

func (m Model) formModal(body string) string {
	w := min(56, max(m.Width-8, 40))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorAccent)).
		Padding(1, 2).
		Width(w).
		Render(strings.TrimRight(body, "\n"))
}

func (m Model) listHeader(title string) string {
	return titleStyle.Render(title) + "\n"
}

func (m Model) tableSep(width int) string {
	return dimStyle.Render(strings.Repeat("─", max(min(width, m.Width-4), 20))) + "\n"
}

func padRight(s string, w int) string {
	dw := displayWidth(s)
	if dw >= w {
		return truncate(s, w)
	}
	return s + strings.Repeat(" ", w-dw)
}

func displayWidth(s string) int {
	return lipgloss.Width(s)
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if displayWidth(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	target := maxLen - 1
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if rw <= 0 {
			b.WriteRune(r)
			continue
		}
		if w+rw > target {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	b.WriteRune('…')
	return b.String()
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampRowCursor(cursor, nRows int) int {
	if nRows <= 0 {
		return 0
	}
	return clamp(cursor, 0, nRows-1)
}

func listWindow(selected, total, maxVisible int) (start, end int) {
	if total <= 0 {
		return 0, 0
	}
	if maxVisible <= 0 {
		maxVisible = 1
	}
	if total <= maxVisible {
		return 0, total
	}
	start = selected - maxVisible/2
	if start < 0 {
		start = 0
	}
	end = start + maxVisible
	if end > total {
		end = total
		start = end - maxVisible
	}
	return start, end
}

func fmtInt(n int) string {
	return fmt.Sprintf("%d", n)
}

func fmtInt64(n int64) string {
	return fmt.Sprintf("%d", n)
}

func toKeyPairs(keys []keyDesc) []struct{ key, desc string } {
	out := make([]struct{ key, desc string }, len(keys))
	for i, k := range keys {
		out[i] = struct{ key, desc string }{k.Key, k.Desc}
	}
	return out
}

func renderKeyHelp(keys []struct{ key, desc string }) string {
	return renderKeyHelpWidth(0, keys)
}

func renderKeyHelpWidth(width int, keys []struct{ key, desc string }) string {
	// Compact chips — muted surface, not loud keycaps.
	chip := lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color(colorMuted)).
		Padding(0, 1)

	if width <= 0 {
		var b strings.Builder
		for i, kb := range keys {
			b.WriteString(chip.Render(kb.key))
			b.WriteString(" ")
			b.WriteString(dimStyle.Render(kb.desc))
			if i < len(keys)-1 {
				b.WriteString("  ")
			}
		}
		return b.String()
	}

	var lines []string
	var cur strings.Builder
	curW := 0
	for _, kb := range keys {
		piece := chip.Render(kb.key) + " " + dimStyle.Render(kb.desc)
		pw := lipgloss.Width(piece)
		need := pw
		if curW > 0 {
			need += 2
		}
		if curW > 0 && curW+need > width {
			lines = append(lines, cur.String())
			cur.Reset()
			curW = 0
			need = pw
		}
		if curW > 0 {
			cur.WriteString("  ")
			curW += 2
		}
		cur.WriteString(piece)
		curW += pw
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return strings.Join(lines, "\n")
}

func clampLines(s string, maxLines int) string {
	if maxLines < 1 {
		maxLines = 1
	}
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}

func packRow(left, right string, inner int) string {
	gap := inner - displayWidth(left) - displayWidth(right)
	if gap < 1 {
		return truncate(left, inner)
	}
	return left + strings.Repeat(" ", gap) + right
}

const objRowsColW = 6

type objectListLayout struct {
	inner    int
	nameW    int
	showRows bool
}

func objectListCols(inner int) objectListLayout {
	if inner < 1 {
		inner = 1
	}
	showRows := inner >= 16
	nameW := inner - 2
	if showRows {
		nameW = max(inner-(2+objRowsColW+1), 6)
	}
	return objectListLayout{inner: inner, nameW: nameW, showRows: showRows}
}

func (c objectListLayout) row(name, rows, kindRaw string) (plain, colored string) {
	glyph := kindGlyph(kindRaw)
	nameCell := padRight(name, c.nameW)
	leftPlain := glyph + " " + nameCell
	leftColored := kindStyle(kindRaw).Render(glyph) + " " + normalStyle.Render(nameCell)
	if !c.showRows || rows == "" {
		return padRight(leftPlain, c.inner), padRight(leftColored, c.inner)
	}
	rowsCell := padLeft(rows, objRowsColW)
	gap := max(c.inner-displayWidth(leftPlain)-objRowsColW, 1)
	spaces := strings.Repeat(" ", gap)
	return padRight(leftPlain+spaces+rowsCell, c.inner),
		padRight(leftColored+spaces+dimStyle.Render(rowsCell), c.inner)
}

func padLeft(s string, w int) string {
	dw := displayWidth(s)
	if dw >= w {
		return truncate(s, w)
	}
	return strings.Repeat(" ", w-dw) + s
}
