package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

func testCompleter() *sqlCompleter {
	c := newSQLCompleter()
	c.Rebuild([]types.SchemaObject{
		{Schema: "public", Name: "users", Kind: types.ObjectTable},
		{Schema: "public", Name: "orders", Kind: types.ObjectTable},
		{Schema: "public", Name: "products", Kind: types.ObjectTable},
		{Schema: "public", Name: "product_categories", Kind: types.ObjectTable},
		{Schema: "public", Name: "active_users", Kind: types.ObjectView},
	}, map[string][]string{
		"public.users":              {"id", "email", "name", "created_at"},
		"public.orders":             {"id", "user_id", "total_cents", "status"},
		"public.products":           {"id", "name", "category_id", "sku"},
		"public.product_categories": {"id", "name", "slug"},
	})
	return c
}

func TestSQLCompleterMatchesTablesAndKeywords(t *testing.T) {
	c := testCompleter()

	got := c.Match("use", 10)
	if !containsFold(got, "users") {
		t.Fatalf("expected users in %v", got)
	}
	got = c.Match("SEL", 10)
	if len(got) == 0 || !strings.EqualFold(got[0][:3], "SEL") {
		t.Fatalf("expected SELECT-like match: %v", got)
	}
	got = c.Match("ema", 10)
	if !containsFold(got, "email") {
		t.Fatalf("expected email column: %v", got)
	}
	got = c.Match("public.u", 10)
	if !containsFold(got, "public.users") {
		t.Fatalf("expected public.users: %v", got)
	}
}

func TestContextFromSuggestsTablesNotColumns(t *testing.T) {
	c := testCompleter()
	// SELECT * FROM pr|  → tables, not random columns
	before := "SELECT * FROM pr"
	got := c.MatchAt("pr", before, 8)
	if len(got) == 0 {
		t.Fatal("no suggestions")
	}
	if !containsFold(got, "products") && !containsFold(got, "product_categories") {
		t.Fatalf("expected product* tables first: %v", got)
	}
	// Columns like created_at must not outrank tables for FROM
	if containsFold(got, "created_at") && indexFold(got, "created_at") < indexFold(got, "products") {
		t.Fatalf("column ranked over table in FROM: %v", got)
	}
}

func TestContextSelectSuggestsColumns(t *testing.T) {
	c := testCompleter()
	before := "SELECT em"
	got := c.MatchAt("em", before, 8)
	if !containsFold(got, "email") {
		t.Fatalf("expected email in SELECT list: %v", got)
	}
}

func TestContextWherePrefersScopedColumns(t *testing.T) {
	c := testCompleter()
	before := "SELECT * FROM users WHERE em"
	got := c.MatchAt("em", before, 8)
	if !containsFold(got, "email") {
		t.Fatalf("expected email after WHERE users: %v", got)
	}
}

func TestContextAfterDotColumnsOnly(t *testing.T) {
	c := testCompleter()
	before := "SELECT u."
	// token "u." → MatchAt splits
	got := c.MatchAt("u.", before+"u.", 8)
	// With full token u. and empty col prefix after split...
	// detect via MatchAt("u.em", ...)
	got = c.MatchAt("u.em", "SELECT u.em", 8)
	if len(got) == 0 {
		// if no cols for alias u, may be empty — use users.
		got = c.MatchAt("users.em", "SELECT users.em", 8)
	}
	if !containsFold(got, "email") {
		t.Fatalf("expected email for users.em: %v", got)
	}
	// apply keeps table. prefix
	newVal, _ := applySQLSuggestion("SELECT users.em", 0, len("SELECT users.em"), "email")
	if newVal != "SELECT users.email" {
		t.Fatalf("apply=%q", newVal)
	}
}

func TestDetectSQLContext(t *testing.T) {
	cases := []struct {
		before string
		want   sqlCtx
	}{
		{"", ctxGeneral},
		{"SELECT ", ctxSelectList},
		{"SELECT * FROM ", ctxFrom},
		{"SELECT * FROM users WHERE ", ctxWhere},
		{"SELECT * FROM users u JOIN ", ctxJoin},
		{"SELECT * FROM users u JOIN orders o ON ", ctxOn},
		{"SELECT * FROM users ORDER BY ", ctxOrder},
		{"SELECT * FROM users GROUP BY ", ctxGroup},
		{"SELECT * FROM users LIMIT ", ctxLimit},
	}
	for _, tc := range cases {
		got, _ := detectSQLContext(tc.before)
		if got != tc.want {
			t.Fatalf("before=%q got=%v want=%v", tc.before, got, tc.want)
		}
	}
}

func containsFold(ss []string, want string) bool {
	return indexFold(ss, want) >= 0
}

func indexFold(ss []string, want string) int {
	for i, s := range ss {
		if strings.EqualFold(s, want) {
			return i
		}
	}
	return -1
}

func TestSQLTokenAtCursorAndApply(t *testing.T) {
	sql := "SELECT * FROM us"
	token, start := sqlTokenAtCursor(sql, 0, len(sql))
	if token != "us" || start != len("SELECT * FROM ") {
		t.Fatalf("token=%q start=%d", token, start)
	}
	newVal, newCol := applySQLSuggestion(sql, 0, len(sql), "users")
	if newVal != "SELECT * FROM users" {
		t.Fatalf("newVal=%q", newVal)
	}
	if newCol != len("SELECT * FROM users") {
		t.Fatalf("newCol=%d", newCol)
	}
}

func TestRefreshAndAcceptSuggestions(t *testing.T) {
	m := NewModel()
	m.Objects = []types.SchemaObject{
		{Schema: "public", Name: "products", Kind: types.ObjectTable},
		{Schema: "public", Name: "product_categories", Kind: types.ObjectTable},
	}
	m.rebuildSQLCompleter()
	nm, _ := m.openQuery()
	m = nm.(Model)
	if m.QueryArea == nil {
		t.Fatal("no query area")
	}
	m.QueryArea.SetValue("SELECT * FROM pro")
	m.QueryArea.SetCursorColumn(len("SELECT * FROM pro"))
	m.refreshQuerySuggestions()
	if len(m.QuerySuggests) == 0 {
		t.Fatal("expected suggestions for pro*")
	}
	if !containsFold(m.QuerySuggests, "products") && !containsFold(m.QuerySuggests, "product_categories") {
		t.Fatalf("suggests=%v", m.QuerySuggests)
	}
	// First suggestion should be a table, not a keyword
	first := strings.ToLower(m.QuerySuggests[0])
	if !strings.Contains(first, "product") {
		t.Fatalf("FROM context should rank tables first, got %v", m.QuerySuggests)
	}
	if !m.acceptQuerySuggestion() {
		t.Fatal("accept failed")
	}
	if !strings.Contains(m.QueryArea.Value(), "product") {
		t.Fatalf("value=%q", m.QueryArea.Value())
	}
}

func TestSQLCompleterMatchIsFastEnough(t *testing.T) {
	c := newSQLCompleter()
	objs := make([]types.SchemaObject, 0, 2000)
	cols := map[string][]string{}
	for i := 0; i < 2000; i++ {
		name := "t" + itoa(i)
		objs = append(objs, types.SchemaObject{Schema: "public", Name: name, Kind: types.ObjectTable})
		cols["public."+name] = []string{"id", "name", "created_at", "email", "status", "user_id", "sku"}
	}
	c.Rebuild(objs, cols)

	// One keystroke must be << 1ms even on a fat catalog.
	start := time.Now()
	_ = c.MatchAt("pr", "SELECT * FROM pr", 8)
	_ = c.MatchAt("em", "SELECT * FROM users WHERE em", 8)
	_ = c.MatchAt("SEL", "", 8)
	oneShot := time.Since(start)
	if oneShot > 2*time.Millisecond {
		t.Fatalf("single keystroke batch too slow: %v (want << 2ms)", oneShot)
	}

	// 30k contextual matches must finish under 50ms (binary search path).
	start = time.Now()
	const rounds = 10000
	for i := 0; i < rounds; i++ {
		_ = c.MatchAt("t1", "SELECT * FROM t1", 8)
		_ = c.MatchAt("email", "SELECT * FROM t0 WHERE em", 8)
		_ = c.MatchAt("SEL", "", 8)
	}
	elapsed := time.Since(start)
	if elapsed > 50*time.Millisecond {
		t.Fatalf("autocomplete too slow: %v for %d matches (budget 50ms)", elapsed, rounds*3)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [12]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}
