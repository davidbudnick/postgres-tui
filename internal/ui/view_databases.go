package ui

import (
	"fmt"
	"strings"
)

// Databases is a compact switcher (TOOLS → Databases), not a mandatory hop.
func (m Model) viewDatabases() string {
	var b strings.Builder
	footerKeys := []keyDesc{
		{"↑/↓", "navigate"},
		{"enter", "open"},
		{"r", "refresh"},
		{"esc", "workspace"},
		{"?", "help"},
	}
	if m.Loading {
		footerKeys = []keyDesc{{"esc", "back"}, {"q", "quit"}}
	} else if len(m.Databases) == 0 {
		footerKeys = []keyDesc{{"esc", "back"}, {"r", "refresh"}}
	}

	header := m.viewDatabasesHeader()
	footer := renderKeyHelpWidth(max(m.Width-2, 20), toKeyPairs(footerKeys))
	chips := m.viewDatabasesChips()

	// Hug content: panel only as tall as the list needs (TablePlus-ish density).
	rows := len(m.Databases)
	if rows == 0 {
		rows = 1
	}
	// header row + data rows + padding
	panelH := min(max(rows+3, 6), max(m.Height-8, 6))
	panelW := min(max(m.Width-4, 40), 90)

	b.WriteString(header)
	b.WriteByte('\n')
	if chips != "" {
		b.WriteString(chips)
		b.WriteByte('\n')
	}
	b.WriteString(m.viewDatabasesPanel(panelW, panelH))
	b.WriteByte('\n')
	b.WriteString(dimStyle.Render(" Switch database — enter opens workspace"))
	b.WriteByte('\n')
	b.WriteString(footer)
	return b.String()
}

func (m Model) viewDatabasesHeader() string {
	conn := "—"
	host := ""
	if m.CurrentConn != nil {
		conn = m.CurrentConn.Name
		host = m.CurrentConn.Address()
	}
	left := " " + titleStyle.Render("Switch database") + dimStyle.Render("  ") +
		statusPillStyle.Render("●") + " " + headerStyle.Render(conn)
	if host != "" {
		left += dimStyle.Render("  " + host)
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
		if len(v) > 36 {
			v = v[:36] + "…"
		}
		right += dimStyle.Render(v) + " "
	}
	gap := m.Width - lipglossWidth(left) - lipglossWidth(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) viewDatabasesChips() string {
	chips := []string{badgeStyle.Render(fmt.Sprintf("%d databases", len(m.Databases)))}
	if m.ServerInfo.User != "" {
		chips = append(chips, badgeStyle.Render("user "+m.ServerInfo.User))
	}
	if m.ServerInfo.ActiveConns > 0 {
		chips = append(chips, badgeStyle.Render(fmt.Sprintf("%d conns", m.ServerInfo.ActiveConns)))
	}
	if m.ReadOnly {
		chips = append(chips, badgeROStyle.Render("READ-ONLY"))
	}
	if m.Loading {
		chips = append(chips, badgeStyle.Render("loading…"))
	}
	return " " + strings.Join(chips, " ")
}

func (m Model) viewDatabasesPanel(width, height int) string {
	innerW := max(width-2, 20)
	innerH := max(height-2, 3)
	var b strings.Builder

	if m.Loading {
		b.WriteString(dimStyle.Render(" Loading databases…"))
		b.WriteByte('\n')
		return panelStyle(true, width, height).Render(clampLines(b.String(), innerH))
	}
	if len(m.Databases) == 0 {
		b.WriteString(dimStyle.Render(" No databases found."))
		b.WriteByte('\n')
		return panelStyle(true, width, height).Render(clampLines(b.String(), innerH))
	}

	nameW := colWidth(innerW, 42, 16, 48)
	hdr := fmt.Sprintf("%s  %s  %s  %s",
		padRight("DATABASE", nameW),
		padRight("OWNER", 16),
		padRight("SIZE", 12),
		padRight("ENCODING", 10),
	)
	b.WriteString(tableHeaderStyle.Render(padRight(hdr, innerW)))
	b.WriteByte('\n')

	maxVisible := max(innerH-2, 3)
	sel := clamp(m.SelectedDBIdx, 0, len(m.Databases)-1)
	start, end := listWindow(sel, len(m.Databases), maxVisible)

	for i := start; i < end; i++ {
		db := m.Databases[i]
		line := fmt.Sprintf("%s  %s  %s  %s",
			padRight(db.Name, nameW),
			padRight(db.Owner, 16),
			padRight(db.SizePretty, 12),
			padRight(db.Encoding, 10),
		)
		cur := db.Name == m.CurrentDatabase
		switch {
		case i == sel:
			b.WriteString(selectedStyle.Render(padRight(line, innerW)))
		case cur:
			b.WriteString(greenStyle.Render(padRight(line, innerW)))
		case i%2 == 1:
			b.WriteString(zebraStyle.Render(padRight(line, innerW)))
		default:
			b.WriteString(padRight(line, innerW))
		}
		b.WriteByte('\n')
	}

	return panelStyle(true, width, height).Render(clampLines(b.String(), innerH))
}

func colWidth(total, fixed, minW, maxW int) int {
	w := total - fixed
	if w < minW {
		return minW
	}
	if maxW > 0 && w > maxW {
		return maxW
	}
	return w
}
