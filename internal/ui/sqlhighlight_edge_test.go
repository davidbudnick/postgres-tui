package ui

import (
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"charm.land/bubbles/v2/textarea"
)

func TestEdge_HighlightSQLLineState(t *testing.T) {
	// empty line variants
	s, in := highlightSQLLineState("", false, 0, true)
	if !strings.Contains(s, "\x1b[") {
		t.Fatal("cursor on empty")
	}
	if in {
		t.Fatal("empty not in block")
	}
	s, in = highlightSQLLineState("", true, -1, false)
	if s != "" || !in {
		t.Fatalf("block cont empty: %q %v", s, in)
	}
	s, in = highlightSQLLineState("", false, -1, false)
	if s != "" || in {
		t.Fatalf("plain empty: %q %v", s, in)
	}

	// numbers including sci notation and leading-dot
	for _, line := range []string{
		"42",
		"3.14",
		".5",
		"2.5e-3",
		"1E+10",
		"6.02e23",
		"1e-2",
	} {
		out, _ := highlightSQLLineState(line, false, -1, false)
		if !strings.Contains(stripANSI(out), strings.TrimLeft(line, "")) {
			// number body retained
			if !strings.Contains(stripANSI(out), "e") && strings.Contains(line, "e") {
				t.Fatalf("sci missing: line=%q out=%q", line, stripANSI(out))
			}
		}
		if !strings.Contains(out, "\x1b[") {
			t.Fatalf("number not styled: %q", line)
		}
	}

	// double-quoted identifiers
	out, _ := highlightSQLLineState(`SELECT "Id", "Full Name" FROM t`, false, -1, false)
	plain := stripANSI(out)
	if !strings.Contains(plain, "Id") || !strings.Contains(plain, "Full Name") {
		t.Fatalf("dquote: %q", plain)
	}
	// unclosed double quote
	out, _ = highlightSQLLineState(`SELECT "open`, false, 8, true)
	if !strings.Contains(stripANSI(out), "open") {
		t.Fatalf("unclosed dquote: %q", stripANSI(out))
	}

	// block comment continue across lines
	_, in = highlightSQLLineState("SELECT /* start", false, -1, false)
	if !in {
		t.Fatal("should open block")
	}
	out, in = highlightSQLLineState("still inside", true, -1, false)
	if !in || !strings.Contains(stripANSI(out), "still") {
		t.Fatalf("continue block: %q in=%v", stripANSI(out), in)
	}
	out, in = highlightSQLLineState("end */ done", true, 2, true)
	if in {
		t.Fatal("should close block")
	}
	if !strings.Contains(stripANSI(out), "done") {
		t.Fatalf("after close: %q", stripANSI(out))
	}
	// unclosed continue with cursor
	_, in = highlightSQLLineState("still going", true, 3, true)
	if !in {
		t.Fatal("still open with cursor")
	}

	// operators
	out, _ = highlightSQLLineState("a + b - c * d / e % f = g <> h != i | j & k ^ l ~ m", false, 2, true)
	if !strings.Contains(out, "\x1b[") {
		t.Fatal("ops styled")
	}
	// star as operator
	out, _ = highlightSQLLineState("SELECT * FROM t", false, 7, true)
	if !strings.Contains(stripANSI(out), "*") {
		t.Fatal("star")
	}

	// line comment
	out, _ = highlightSQLLineState("SELECT 1 -- trailing", false, 3, true)
	if !strings.Contains(stripANSI(out), "trailing") {
		t.Fatal("line comment")
	}

	// single-quoted with escape and unclosed
	out, _ = highlightSQLLineState(`'it''s'`, false, 2, true)
	if !strings.Contains(stripANSI(out), "it") {
		t.Fatal("sq escape")
	}
	out, _ = highlightSQLLineState(`'oops`, false, 1, true)
	if !strings.Contains(stripANSI(out), "oops") {
		t.Fatal("sq unclosed")
	}

	// keywords + functions + idents
	out, _ = highlightSQLLineState("SELECT COUNT(*), lower(name) FROM users", false, 0, true)
	plain = stripANSI(out)
	for _, tok := range []string{"SELECT", "COUNT", "lower", "users"} {
		if !strings.Contains(plain, tok) {
			t.Fatalf("missing %q in %q", tok, plain)
		}
	}

	// cursor past end of line
	out, _ = highlightSQLLineState("ab", false, 10, true)
	if !strings.Contains(out, "\x1b[") {
		t.Fatal("cursor past end")
	}

	// punctuation / whitespace
	out, _ = highlightSQLLineState("(a, b);", false, 0, true)
	if !strings.Contains(stripANSI(out), "(a, b);") {
		t.Fatalf("punct: %q", stripANSI(out))
	}

	// inline block comment fully closed on same line
	out, _ = highlightSQLLineState("a /*c*/ b", false, 2, true)
	if !strings.Contains(stripANSI(out), "b") {
		t.Fatalf("inline block: %q", stripANSI(out))
	}
}

func TestEdge_PaintSegment(t *testing.T) {
	if paintSegment("", sqlKwStyle, 0, 0, true) != "" {
		t.Fatal("empty seg")
	}

	// unfocused / no cursor col → plain style
	out := paintSegment("abc", sqlKwStyle, 0, -1, false)
	if !strings.Contains(out, "abc") && stripANSI(out) != "abc" {
		// styled or plain still contains letters
		if !strings.Contains(stripANSI(out), "abc") {
			t.Fatalf("plain: %q", out)
		}
	}
	_ = paintSegment("abc", sqlKwStyle, 0, -1, false)

	// cursorLine with cursorCol < 0 → wash whole segment
	out = paintSegment("abc", sqlKwStyle, 0, -1, true)
	if !strings.Contains(stripANSI(out), "abc") {
		t.Fatalf("wash: %q", stripANSI(out))
	}

	// cursor before segment (cursorLine true, cursorCol >= 0 but < start)
	out = paintSegment("abc", sqlKwStyle, 5, 1, true)
	if !strings.Contains(stripANSI(out), "abc") {
		t.Fatalf("before: %q", stripANSI(out))
	}

	// cursor after segment
	out = paintSegment("abc", sqlKwStyle, 0, 99, true)
	if !strings.Contains(stripANSI(out), "abc") {
		t.Fatalf("after: %q", stripANSI(out))
	}

	// cursor at start of segment (off==0, no left)
	out = paintSegment("abc", sqlKwStyle, 0, 0, true)
	if !strings.Contains(stripANSI(out), "abc") {
		t.Fatalf("at start: %q", stripANSI(out))
	}

	// cursor inside (has left + cursor + right)
	out = paintSegment("abc", sqlKwStyle, 0, 1, true)
	if stripANSI(out) != "abc" {
		t.Fatalf("inside: %q", stripANSI(out))
	}

	// cursor at last char (left + cursor, no right)
	out = paintSegment("abc", sqlKwStyle, 0, 2, true)
	if stripANSI(out) != "abc" {
		t.Fatalf("at end: %q", stripANSI(out))
	}

	// single-rune segment with cursor
	out = paintSegment("x", sqlKwStyle, 0, 0, true)
	if stripANSI(out) != "x" {
		t.Fatalf("single: %q", stripANSI(out))
	}

	// cursorLine false with cursorCol >= 0 → early plain path
	out = paintSegment("abc", sqlKwStyle, 0, 1, false)
	if stripANSI(out) != "abc" {
		t.Fatalf("unfocused with col: %q", stripANSI(out))
	}

	// start offset non-zero, cursor maps into segment
	out = paintSegment("xyz", sqlNumStyle, 10, 11, true)
	if stripANSI(out) != "xyz" {
		t.Fatalf("offset start: %q", stripANSI(out))
	}
}

func setTextareaRow(t *testing.T, ta *textarea.Model, row int) {
	t.Helper()
	v := reflect.ValueOf(ta).Elem().FieldByName("row")
	if !v.IsValid() {
		t.Fatal("row field missing")
	}
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().SetInt(int64(row))
}

func setTextareaCol(t *testing.T, ta *textarea.Model, col int) {
	t.Helper()
	v := reflect.ValueOf(ta).Elem().FieldByName("col")
	if !v.IsValid() {
		t.Fatal("col field missing")
	}
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().SetInt(int64(col))
}

func TestEdge_RenderHighlightedSQLEditor(t *testing.T) {
	m := NewModel()

	// nil editor
	out := m.renderHighlightedSQLEditor(40, 5, true)
	if !strings.Contains(out, "no editor") {
		t.Fatalf("nil: %q", out)
	}

	nm, _ := m.openQuery()
	m = nm.(Model)

	// empty value focused + unfocused
	m.QueryArea.SetValue("")
	out = m.renderHighlightedSQLEditor(40, 6, true)
	if !strings.Contains(stripANSI(out), "1") {
		t.Fatalf("empty focused gutter: %q", stripANSI(out))
	}
	out = m.renderHighlightedSQLEditor(40, 6, false)
	if !strings.Contains(stripANSI(out), "1") {
		t.Fatalf("empty unfocused: %q", stripANSI(out))
	}

	// single short line
	m.QueryArea.SetValue("SELECT 1")
	out = m.renderHighlightedSQLEditor(50, 8, true)
	if !strings.Contains(stripANSI(out), "SELECT") {
		t.Fatalf("short: %q", stripANSI(out))
	}
	// tilde padding when height > content
	if !strings.Contains(stripANSI(out), "~") {
		t.Fatalf("tilde pad: %q", stripANSI(out))
	}

	// multi-line + scroll near bottom (viewport start > 0, end clamp)
	var b strings.Builder
	for i := 0; i < 30; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("SELECT 1")
	}
	m.QueryArea.SetValue(b.String())
	for i := 0; i < 28; i++ {
		m.QueryArea.CursorDown()
	}
	out = m.renderHighlightedSQLEditor(40, 6, true)
	if stripANSI(out) == "" {
		t.Fatal("scroll bottom empty")
	}

	// multi-line with cursor at top → start < 0 path
	m.QueryArea.SetValue(b.String())
	setTextareaRow(t, m.QueryArea, 0) // SetValue leaves cursor at end
	setTextareaCol(t, m.QueryArea, 0)
	out = m.renderHighlightedSQLEditor(40, 6, true)
	if !strings.Contains(stripANSI(out), "SELECT") {
		t.Fatalf("scroll top: %q", stripANSI(out))
	}

	// long line truncation + ellipsis on focused cursor line
	long := "SELECT very_long_identifier_name_that_must_be_truncated_in_the_editor_window_xyz_12345"
	m.QueryArea.SetValue(long)
	setTextareaCol(t, m.QueryArea, len(long)) // past truncated width
	out = m.renderHighlightedSQLEditor(30, 4, true)
	if !strings.Contains(out, "…") && !strings.Contains(stripANSI(out), "…") {
		// dim ellipsis may include ANSI; strip and check
		if !strings.Contains(stripANSI(out), "…") {
			t.Fatalf("expected ellipsis: %q", stripANSI(out))
		}
	}

	// long line unfocused (no cursor paint) still truncates
	out = m.renderHighlightedSQLEditor(30, 4, false)
	if !strings.Contains(stripANSI(out), "SELECT") {
		t.Fatalf("long unfocused: %q", stripANSI(out))
	}

	// multi-line with long line visible near cursor mid-scroll
	var b2 strings.Builder
	for i := 0; i < 20; i++ {
		if i > 0 {
			b2.WriteByte('\n')
		}
		if i == 10 {
			b2.WriteString(long)
		} else {
			b2.WriteString("SELECT 1")
		}
	}
	m.QueryArea.SetValue(b2.String())
	for i := 0; i < 10; i++ {
		m.QueryArea.CursorDown()
	}
	out = m.renderHighlightedSQLEditor(28, 6, true)
	if !strings.Contains(stripANSI(out), "SELECT") {
		t.Fatalf("mid long: %q", stripANSI(out))
	}

	// focused current line with col past rune length
	m.QueryArea.SetValue("short")
	setTextareaRow(t, m.QueryArea, 0)
	setTextareaCol(t, m.QueryArea, 500)
	out = m.renderHighlightedSQLEditor(40, 3, true)
	if !strings.Contains(stripANSI(out), "short") {
		t.Fatalf("col over: %q", stripANSI(out))
	}

	// col past truncated plain length on a long focused line
	m.QueryArea.SetValue(long)
	setTextareaRow(t, m.QueryArea, 0)
	setTextareaCol(t, m.QueryArea, len(long)+10)
	out = m.renderHighlightedSQLEditor(24, 3, true)
	if !strings.Contains(stripANSI(out), "SELECT") {
		t.Fatalf("col past truncate: %q", stripANSI(out))
	}

	// height 1 single line
	m.QueryArea.SetValue("x")
	setTextareaCol(t, m.QueryArea, 0)
	_ = m.renderHighlightedSQLEditor(20, 1, true)

	// multi-line block comment spanning before viewport so inBlock tracked
	var b3 strings.Builder
	b3.WriteString("SELECT /* open comment")
	for i := 0; i < 15; i++ {
		b3.WriteString("\nstill commented")
	}
	b3.WriteString("\n*/ SELECT 2")
	m.QueryArea.SetValue(b3.String())
	for i := 0; i < 16; i++ {
		m.QueryArea.CursorDown()
	}
	out = m.renderHighlightedSQLEditor(40, 4, true)
	if stripANSI(out) == "" {
		t.Fatal("block scroll")
	}
}

func TestEdge_HighlightSQLWrappers(t *testing.T) {
	if highlightSQL("") != "" {
		t.Fatal("empty highlightSQL")
	}
	lines := highlightSQLLines("")
	if len(lines) != 1 || lines[0] != "" {
		t.Fatalf("empty lines: %v", lines)
	}
	sql := "SELECT /* open\nstill\n*/ 1 + 2.5e-3\nSELECT \"Id\", 'it''s' FROM t WHERE x >= .5 -- tail"
	out := highlightSQL(sql)
	plain := stripANSI(out)
	for _, tok := range []string{"still", "Id", "it", "tail", "2.5e-3"} {
		if !strings.Contains(plain, tok) {
			t.Fatalf("missing %q in %q", tok, plain)
		}
	}
	_ = highlightSQL("SELECT 'oops")
	_ = highlightSQL("/* only")
	_ = highlightSQL("42 select")
}
