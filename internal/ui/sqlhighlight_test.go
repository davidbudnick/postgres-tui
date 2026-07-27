package ui

import (
	"strings"
	"testing"
)

func TestHighlightSQLKeywordsAndStrings(t *testing.T) {
	sql := "SELECT id, email FROM users WHERE name = 'Ada' AND n = 42; -- note"
	out := highlightSQL(sql)
	if out == sql || out == "" {
		t.Fatalf("expected colored output, got plain %q", out)
	}
	// Must still contain the source tokens
	plain := stripANSI(out)
	for _, tok := range []string{"SELECT", "FROM", "users", "Ada", "42"} {
		if !strings.Contains(plain, tok) {
			t.Fatalf("missing %q in %q", tok, plain)
		}
	}
	// ANSI escapes present
	if !strings.Contains(out, "\x1b[") {
		t.Fatal("expected ANSI color codes")
	}
}

func TestHighlightSQLBlockComment(t *testing.T) {
	sql := "SELECT /* hide */ 1"
	out := highlightSQL(sql)
	if !strings.Contains(stripANSI(out), "hide") {
		t.Fatalf("comment body missing: %q", stripANSI(out))
	}
}

func TestHighlightSQLCursorOnKeyword(t *testing.T) {
	// Cursor on first char of SELECT
	line, _ := highlightSQLLineState("SELECT 1", false, 0, true)
	if !strings.Contains(line, "\x1b[") {
		t.Fatal("cursor line should be styled")
	}
	if !strings.Contains(stripANSI(line), "SELECT") {
		t.Fatalf("got %q", stripANSI(line))
	}
}

func TestRenderHighlightedEditorEmpty(t *testing.T) {
	m := NewModel()
	nm, _ := m.openQuery()
	m = nm.(Model)
	m.QueryArea.SetValue("")
	s := m.renderHighlightedSQLEditor(60, 6, true)
	if strings.TrimSpace(stripANSI(s)) == "" {
		// empty editor still shows line gutter "1"
	}
	if !strings.Contains(stripANSI(s), "1") {
		t.Fatalf("expected line number gutter: %q", stripANSI(s))
	}
}
