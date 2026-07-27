package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

// Match redis-tui home: natural-width block, outer Place() centers once.
// Never Width()+Align the whole block — that wraps the logo mid-glyph.
const connCardW = 55

func (m Model) viewConnections() string {
	var b strings.Builder
	w := connCardW
	if m.Width > 0 {
		w = min(connCardW, max(m.Width-10, 40))
	}

	b.WriteString(m.connectionsLogo())
	b.WriteString("\n\n")
	b.WriteString(m.buildStatsBar())
	b.WriteString("\n\n")

	if m.Loading {
		b.WriteString(dimStyle.Render("Connecting…"))
		b.WriteString("\n\n")
	}
	if m.ConnectionError != "" {
		errBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorRed)).
			Foreground(lipgloss.Color(colorRed)).
			Padding(0, 2).
			Width(w).
			Render(fmt.Sprintf("Connection Failed\n%s", dimStyle.Render(m.ConnectionError)))
		b.WriteString(errBox)
		b.WriteString("\n\n")
	}

	n := len(m.Connections)
	// Frame width matches redis (fixed 55-char section).
	sectionInner := w - 1
	top := fmt.Sprintf("╭─ Saved Instances (%d) ", n)
	if pad := sectionInner - len([]rune(top)); pad > 0 {
		top += strings.Repeat("─", pad)
	}
	top += "╮"
	bot := "╰" + strings.Repeat("─", sectionInner) + "╯"

	b.WriteString(accentStyle.Render(top))
	b.WriteString("\n")

	if n == 0 {
		b.WriteString("\n")
		empty := lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim)).
			Padding(1, 2).
			Render("  No instances saved.\n\n  Press a to add your first PostgreSQL connection.")
		b.WriteString(empty)
		b.WriteString("\n")
	} else {
		b.WriteString("\n")
		maxVisible := max((m.Height-20)/3, 3)
		sel := clamp(m.SelectedConnIdx, 0, n-1)
		start, end := listWindow(sel, n, maxVisible)
		for i := start; i < end; i++ {
			b.WriteString(m.renderConnectionCard(m.Connections[i], i == sel, w))
			b.WriteString("\n")
		}
		if n > maxVisible {
			b.WriteString(dimStyle.Render(fmt.Sprintf("  ↕ %d–%d of %d", start+1, end, n)))
			b.WriteString("\n")
		}
	}

	b.WriteString(accentStyle.Render(bot))
	b.WriteString("\n\n")
	b.WriteString(m.connectionsKeyHelp(w))

	return strings.TrimRight(b.String(), "\n")
}

func (m Model) connectionsLogo() string {
	// Same scale as redis-tui REDIS logo (~48–50 cols). One style, no per-line wrap.
	logo := `
██████╗  ██████╗ ███████╗████████╗ ██████╗ ██████╗ ███████╗
██╔══██╗██╔═══██╗██╔════╝╚══██╔══╝██╔════╝ ██╔══██╗██╔════╝
██████╔╝██║   ██║███████╗   ██║   ██║  ███╗██████╔╝█████╗  
██╔═══╝ ██║   ██║╚════██║   ██║   ██║   ██║██╔══██╗██╔══╝  
██║     ╚██████╔╝███████║   ██║   ╚██████╔╝██║  ██║███████╗
╚═╝      ╚═════╝ ╚══════╝   ╚═╝    ╚═════╝ ╚═╝  ╚═╝╚══════╝`
	// Trim leading newline from raw string; keep lines intact (no Width wrap).
	logo = strings.TrimPrefix(logo, "\n")
	return logoStyle.Render(logo) + "\n" +
		dimStyle.Render("PostgreSQL TUI  ·  keyboard-first instance manager")
}

func (m Model) connectionsKeyHelp(width int) string {
	keys := []struct{ key, desc string }{
		{"↑/↓", "navigate"},
		{"enter", "connect"},
		{"a", "add"},
		{"e", "edit"},
		{"d", "delete"},
		{"t", "test"},
		{"?", "help"},
		{"q", "quit"},
	}
	// Prefer one line when it fits; otherwise wrap without expanding past logo.
	line := renderKeyHelp(keys)
	if width > 0 && lipgloss.Width(line) > width {
		return renderKeyHelpWidth(width, keys)
	}
	return line
}

func (m Model) buildStatsBar() string {
	boxes := []struct {
		label string
		value string
		color string
	}{
		{"Instances", fmt.Sprintf("%d saved", len(m.Connections)), colorAccent},
		{"Time", time.Now().Format("15:04:05"), "245"},
	}
	if m.Version != "" && m.Version != "dev" {
		boxes = append(boxes, struct{ label, value, color string }{"Version", m.Version, colorPGBlueHi})
	}

	var statsBoxes []string
	for _, box := range boxes {
		content := fmt.Sprintf("%s\n%s",
			dimStyle.Render(box.label),
			lipgloss.NewStyle().Foreground(lipgloss.Color(box.color)).Bold(true).Render(box.value),
		)
		statsBoxes = append(statsBoxes, statsBoxStyle.Width(16).Render(content))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, statsBoxes...)
}

func (m Model) renderConnectionCard(conn types.Connection, selected bool, width int) string {
	var card strings.Builder

	icon := "○"
	nameStyle := normalStyle
	if selected {
		icon = "●"
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen)).Bold(true)
	}

	fmt.Fprintf(&card, " %s %s", icon, nameStyle.Render(conn.Name))
	card.WriteString("\n")

	hostPort := fmt.Sprintf("%s:%d", conn.Host, conn.Port)
	if conn.Database != "" {
		hostPort += "/" + conn.Database
	}
	card.WriteString(dimStyle.Render("   " + hostPort))

	if conn.Username != "" {
		card.WriteString("  ")
		card.WriteString(badgeStyle.Render(conn.Username))
	}
	ssl := string(conn.SSLMode)
	if ssl == "" {
		ssl = "prefer"
	}
	if ssl != "disable" {
		card.WriteString(" ")
		card.WriteString(badgeSSLStyle.Render("SSL"))
	}
	if conn.ReadOnly {
		card.WriteString(" ")
		card.WriteString(badgeROStyle.Render("RO"))
	}

	style := connCardStyle
	if selected {
		style = connCardSelectedStyle
	}
	return style.Width(width).Render(card.String())
}

func (m Model) viewConnectionForm() string {
	title := "Add Instance"
	if m.Screen == types.ScreenEditConnection {
		title = "Edit Instance"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	for i, label := range connTextLabels {
		if i >= len(m.ConnInputs) {
			break
		}
		labelStyle := keyStyle
		if m.ConnFocusIdx == i {
			labelStyle = accentStyle
		}
		b.WriteString(labelStyle.Render(label))
		b.WriteString("\n")
		b.WriteString(m.ConnInputs[i].View())
		b.WriteString("\n\n")
	}

	sslFocused := m.ConnFocusIdx == connFieldSSL
	sslLabel := keyStyle
	if sslFocused {
		sslLabel = accentStyle
	}
	b.WriteString(sslLabel.Render("SSL Mode"))
	b.WriteString("\n")
	cur := string(sslModeOptions[clamp(m.ConnSSLIdx, 0, len(sslModeOptions)-1)])
	sslLine := fmt.Sprintf("  %-14s ◂ ▸", cur)
	if sslFocused {
		b.WriteString(accentStyle.Render(sslLine))
	} else {
		b.WriteString(normalStyle.Render(sslLine))
	}
	b.WriteString("\n\n")

	roFocused := m.ConnFocusIdx == connFieldReadOnly
	roLabel := keyStyle
	if roFocused {
		roLabel = accentStyle
	}
	b.WriteString(roLabel.Render("Read-only"))
	b.WriteString("\n")
	check := "[ ] Browse only — block writes"
	if m.ConnReadOnly {
		check = "[x] Browse only — block writes"
	}
	checkStyle := normalStyle
	if roFocused {
		checkStyle = accentStyle
	}
	b.WriteString(checkStyle.Render(check))
	b.WriteString("\n\n")

	if m.Err != nil && (m.Screen == types.ScreenAddConnection || m.Screen == types.ScreenEditConnection) {
		b.WriteString(errorStyle.Render(m.Err.Error()))
		b.WriteString("\n\n")
	}

	b.WriteString(renderKeyHelp([]struct{ key, desc string }{
		{"tab", "next"},
		{"shift+tab", "prev"},
		{"space", "toggle"},
		{"←/→", "ssl"},
		{"enter", "save"},
		{"esc", "cancel"},
	}))

	return m.formModal(b.String())
}

func (m Model) viewTestConnection() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Test Connection"))
	b.WriteString("\n\n")
	if m.Loading {
		b.WriteString(dimStyle.Render("Testing…"))
	} else if m.TestConnResult != "" {
		if strings.HasPrefix(m.TestConnResult, "OK") {
			b.WriteString(successStyle.Render(m.TestConnResult))
		} else {
			b.WriteString(errorStyle.Render(m.TestConnResult))
		}
	} else {
		b.WriteString(dimStyle.Render("No result"))
	}
	b.WriteString("\n\n")
	b.WriteString(renderKeyHelp([]struct{ key, desc string }{{"esc", "back"}, {"enter", "back"}}))
	return m.formModal(b.String())
}

func (m Model) viewConfirmDelete() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Confirm Delete"))
	b.WriteString("\n\n")
	switch m.ConfirmType {
	case "connection":
		if conn, ok := m.ConfirmData.(types.Connection); ok {
			b.WriteString(fmt.Sprintf("Delete instance %s?", accentStyle.Render(conn.Name)))
		} else {
			b.WriteString("Delete this instance?")
		}
	default:
		b.WriteString("Are you sure?")
	}
	b.WriteString("\n\n")
	b.WriteString(renderKeyHelp([]struct{ key, desc string }{
		{"y", "yes"},
		{"n/esc", "cancel"},
	}))
	return m.formModal(b.String())
}
