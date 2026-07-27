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

func TestHighlightSQLEdges(t *testing.T) {
	if highlightSQL("") != "" {
		t.Fatal("empty")
	}
	lines := highlightSQLLines("")
	if len(lines) != 1 {
		t.Fatal(lines)
	}
	sql := "SELECT /* open\nstill\n*/ 1 + 2.5e-3"
	out := highlightSQL(sql)
	if !strings.Contains(stripANSI(out), "still") {
		t.Fatal(stripANSI(out))
	}
	_ = highlightSQL(`SELECT COUNT(*), "Id", 'it''s' FROM t WHERE x >= .5 -- tail`)
	_ = highlightSQL("SELECT 'oops")
	_ = highlightSQL("/* only")
	_ = highlightSQL("42 select")

	s, _ := highlightSQLLineState("", false, 0, true)
	if !strings.Contains(s, "\x1b[") {
		t.Fatal("cursor empty")
	}
	_, inBlock := highlightSQLLineState("", true, -1, false)
	if !inBlock {
		t.Fatal("block cont empty")
	}
	s, _ = highlightSQLLineState("ab", false, 10, true)
	_ = s
	_, _ = highlightSQLLineState("SELECT 'hi'", false, 8, true)
	_, _ = highlightSQLLineState("12345", false, 2, true)
	_, _ = highlightSQLLineState("-- note", false, 3, true)
	_, _ = highlightSQLLineState("a + b", false, 2, true)
	_, _ = highlightSQLLineState(`"abc"`, false, 2, true)
	_, _ = highlightSQLLineState("/*c*/ x", false, 2, true)
	_, inBlock = highlightSQLLineState("still */ done", true, 2, true)
	if inBlock {
		t.Fatal("should close")
	}
	_, inBlock = highlightSQLLineState("still going", true, -1, false)
	if !inBlock {
		t.Fatal("still open")
	}
	if paintSegment("", sqlKwStyle, 0, 0, true) != "" {
		t.Fatal("empty paint")
	}
	_ = paintSegment("abc", sqlKwStyle, 0, -1, false)
	_ = paintSegment("abc", sqlKwStyle, 0, -1, true)
	_ = paintSegment("abc", sqlKwStyle, 0, 99, true)
	_ = paintSegment("abc", sqlKwStyle, 0, 1, true)
	_ = paintSegment("abc", sqlKwStyle, 0, 1, false)
	_ = paintSegment("a", sqlKwStyle, 0, 0, false)
}

func TestRenderHighlightedEditorEdges(t *testing.T) {
	m := NewModel()
	if !strings.Contains(m.renderHighlightedSQLEditor(40, 5, true), "no editor") {
		t.Fatal("nil editor")
	}
	nm, _ := m.openQuery()
	m = nm.(Model)
	m.QueryArea.SetValue("")
	_ = m.renderHighlightedSQLEditor(40, 6, true)
	_ = m.renderHighlightedSQLEditor(40, 6, false)

	var b strings.Builder
	for i := 0; i < 20; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i == 5 {
			b.WriteString("SELECT /* c */ very_long_identifier_name_that_should_truncate_in_the_editor_window_xyz")
		} else {
			b.WriteString("SELECT 1")
		}
	}
	m.QueryArea.SetValue(b.String())
	for i := 0; i < 12; i++ {
		m.QueryArea.CursorDown()
	}
	_ = m.renderHighlightedSQLEditor(30, 6, true)
	m.QueryArea.SetValue("x")
	out := m.renderHighlightedSQLEditor(40, 8, true)
	if !strings.Contains(stripANSI(out), "~") {
		t.Fatalf("tilde pad %q", stripANSI(out))
	}
}
