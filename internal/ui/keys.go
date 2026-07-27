package ui

import (
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

var keyMsgString = func(msg tea.KeyPressMsg) string { return msg.String() }

func normalizeKey(msg tea.KeyPressMsg) string {
	// Tab chords must win over Text/String quirks across terminals.
	if msg.Code == tea.KeyTab {
		if msg.Mod&tea.ModShift != 0 {
			return "shift+tab"
		}
		return "tab"
	}
	s := keyMsgString(msg)
	if s == "backtab" || s == "shift+tab" {
		return "shift+tab"
	}
	// Prefer the full chord string when modifiers are present so ctrl/alt/meta
	// combos (ctrl+enter, ctrl+p, …) are not collapsed to bare Text ("\n", "p").
	if strings.Contains(s, "ctrl+") || strings.Contains(s, "alt+") || strings.Contains(s, "super+") || strings.Contains(s, "meta+") {
		return s
	}
	if t := msg.Text; t != "" && t != " " {
		return t
	}
	if strings.HasPrefix(s, "shift+") {
		rest := strings.TrimPrefix(s, "shift+")
		if len(rest) == 1 {
			r := rune(rest[0])
			if r >= 'a' && r <= 'z' {
				return string(unicode.ToUpper(r))
			}
			switch rest {
			case "/":
				return "?"
			case "3":
				return "#"
			case "8":
				return "*"
			case ";":
				return ":"
			case "'":
				return "\""
			case "1":
				return "!"
			case ",":
				return "<"
			case ".":
				return ">"
			case "-":
				return "_"
			case "=":
				return "+"
			case "`":
				return "~"
			case "\\":
				return "|"
			case "[":
				return "{"
			case "]":
				return "}"
			}
		}
	}
	return s
}

// cycleFocus toggles sidebar tree ↔ content (2-pane workspace).
func cycleFocus(cur focusPane, reverse bool) focusPane {
	_ = reverse
	if cur == focusContent {
		return focusSidebar
	}
	return focusContent
}

func (m Model) typingContext() bool {
	if m.FilterActive || m.FilterInput.Focused() {
		return true
	}
	switch m.Screen {
	case types.ScreenAddConnection, types.ScreenEditConnection,
		types.ScreenExport, types.ScreenCommandPalette:
		return true
	case types.ScreenQuery:
		return m.Focus == focusContent && m.QueryFocus == "editor" && m.QueryArea != nil
	default:
		return false
	}
}
