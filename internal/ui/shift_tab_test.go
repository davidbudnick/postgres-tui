package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestNormalizeShiftTab(t *testing.T) {
	k := normalizeKey(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if k != "shift+tab" {
		t.Fatalf("got %q", k)
	}
}

func TestShiftTabBrowserFocusCycle(t *testing.T) {
	m := NewModel()
	m.Width, m.Height = 120, 40
	m.Screen = types.ScreenBrowser
	m.Focus = focusContent
	nm, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m = nm.(Model)
	if m.Focus != focusSidebar {
		t.Fatalf("focus=%v want sidebar", m.Focus)
	}
	nm, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = nm.(Model)
	if m.Focus != focusContent {
		t.Fatalf("focus=%v want content", m.Focus)
	}
}

func TestShiftTabTableDataFocusCycle(t *testing.T) {
	m := NewModel()
	m.Width, m.Height = 120, 40
	m.Screen = types.ScreenTableData
	m.Focus = focusContent
	nm, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m = nm.(Model)
	if m.Focus != focusSidebar {
		t.Fatalf("focus=%v want sidebar", m.Focus)
	}
	nm, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = nm.(Model)
	if m.Focus != focusContent {
		t.Fatalf("focus=%v want content", m.Focus)
	}
}

func TestShiftTabTableDetailFocusCycle(t *testing.T) {
	m := NewModel()
	m.Width, m.Height = 120, 40
	m.Screen = types.ScreenTableDetail
	m.Focus = focusContent
	nm, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m = nm.(Model)
	if m.Focus != focusSidebar {
		t.Fatalf("focus=%v want sidebar", m.Focus)
	}
}

func TestShiftTabFormFields(t *testing.T) {
	m := NewModel()
	m.Width, m.Height = 120, 40
	m.Screen = types.ScreenAddConnection
	m.ConnFocusIdx = 2
	m.focusConnField()
	nm, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	m = nm.(Model)
	if m.ConnFocusIdx != 1 {
		t.Fatalf("focus=%d want 1", m.ConnFocusIdx)
	}
}

func TestConnectionsRenderAtZeroSize(t *testing.T) {
	m := NewModel()
	// Before WindowSizeMsg — Width/Height are 0; must still paint home, not "too small".
	s := m.render()
	if s == "" {
		t.Fatal("empty render")
	}
	if contains(s, "too small") {
		t.Fatalf("should not flash too-small on launch: %q", truncate(s, 80))
	}
	if !contains(s, "Instances") && !contains(s, "instances") {
		t.Fatalf("expected home screen content: %q", truncate(s, 120))
	}
}

func TestConnectionsRenderNormal(t *testing.T) {
	m := NewModel()
	m.Width, m.Height = 120, 40
	s := m.render()
	if !contains(s, "Instances") && !contains(s, "instances") && !contains(s, "add") {
		t.Fatalf("broken home: %q", truncate(s, 200))
	}
}

func TestConnectionsLogoDoesNotWrap(t *testing.T) {
	m := NewModel()
	m.Width, m.Height = 100, 40
	body := m.viewConnections()
	// Logo must stay on 6 solid lines — Width() wrap previously smashed it mid-glyph.
	lines := splitLines(body)
	blockLines := 0
	for _, ln := range lines {
		if strings.Contains(ln, "█") || strings.Contains(ln, "╗") || strings.Contains(ln, "╝") {
			blockLines++
		}
	}
	if blockLines < 6 {
		t.Fatalf("logo lines wrapped/lost: found %d block-art lines", blockLines)
	}
	// No single logo line should be absurdly short (wrap artifact).
	for _, ln := range lines {
		if !strings.Contains(ln, "█") {
			continue
		}
		if lipglossWidth(ln) < 40 {
			t.Fatalf("logo line looks wrapped (width=%d): %q", lipglossWidth(ln), truncate(ln, 50))
		}
	}
}

func splitLines(s string) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		out = append(out, s[start:])
	}
	return out
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}
