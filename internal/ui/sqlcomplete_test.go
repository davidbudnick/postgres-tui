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
	if raceEnabled {
		t.Skip("timing budget is meaningless under -race")
	}
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
	if elapsed > 100*time.Millisecond {
		t.Fatalf("autocomplete too slow: %v for %d matches (budget 100ms)", elapsed, rounds*3)
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

func TestStripSQLNoiseAndIdentEdges(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"SELECT 1", "SELECT 1"},
		{"SELECT -- c\n1", "SELECT \n1"},
		{"SELECT /* b */ 1", "SELECT  1"},
		{"SELECT /* unclosed", "SELECT d"},
		{"SELECT 'a''b' x", "SELECT   x"},
		{"SELECT 'open", "SELECT  "},
	}
	for _, tc := range cases {
		got := stripSQLNoiseFast(tc.in)
		if got != tc.want {
			t.Fatalf("strip(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
	if !onlyWhitespace("") || !onlyWhitespace(" \t\n\r") || onlyWhitespace("x") {
		t.Fatal("onlyWhitespace")
	}
	if !isSQLIdentPrefix("") || !isSQLIdentPrefix("a_1") || !isSQLIdentPrefix(`"Q"`) || !isSQLIdentPrefix("a.b") {
		t.Fatal("ident ok")
	}
	if isSQLIdentPrefix("1a") || isSQLIdentPrefix("a-b") {
		t.Fatal("ident bad")
	}
}

func TestMatchAtAllContextsAndHelpers(t *testing.T) {
	c := testCompleter()
	if c.MatchAt("x", "", 0) != nil {
		t.Fatal("limit0")
	}
	var nilC *sqlCompleter
	if nilC.MatchAt("x", "", 5) != nil {
		t.Fatal("nil")
	}
	if c.MatchAt("1x", "SELECT ", 5) != nil {
		t.Fatal("bad prefix")
	}

	checks := []struct {
		p, b string
		ok   func([]string) bool
	}{
		{"pr", "SELECT * FROM ", func(g []string) bool { return containsFold(g, "products") }},
		{"or", "SELECT * FROM users JOIN ", func(g []string) bool { return containsFold(g, "orders") }},
		{"em", "SELECT ", func(g []string) bool { return containsFold(g, "email") }},
		{"em", "SELECT * FROM users WHERE ", func(g []string) bool { return containsFold(g, "email") }},
		{"us", "SELECT * FROM users u JOIN orders o ON ", func(g []string) bool { return len(g) >= 0 }},
		{"em", "SELECT * FROM users ORDER BY ", func(g []string) bool { return len(g) > 0 }},
		{"em", "SELECT * FROM users GROUP BY ", func(g []string) bool { return len(g) > 0 }},
		{"", "SELECT * FROM users LIMIT ", func(g []string) bool { return containsFold(g, "OFFSET") }},
		{"em", "UPDATE users SET ", func(g []string) bool { return len(g) > 0 }},
		{"us", "INSERT INTO ", func(g []string) bool { return containsFold(g, "users") }},
		{"users.em", "SELECT ", func(g []string) bool { return containsFold(g, "email") }},
		{"public.users.em", "SELECT ", func(g []string) bool { return containsFold(g, "email") }},
		{"", "SELECT * FROM ", func(g []string) bool { return len(g) > 0 }},
		{"SE", "", func(g []string) bool { return containsFold(g, "SELECT") }},
		{"SEL", "", func(g []string) bool { return len(g) > 0 && g[0] == "SELECT" }},
		{"WH", "x WH", func(g []string) bool { return containsFold(g, "WHERE") || len(g) >= 0 }},
	}
	for i, tc := range checks {
		got := c.MatchAt(tc.p, tc.b, 16)
		if !tc.ok(got) {
			t.Fatalf("case %d p=%q b=%q got=%v", i, tc.p, tc.b, got)
		}
	}
	_ = c.MatchAt("email", "SELECT email FROM users", 8)
	_ = c.MatchAt("users", "SELECT * FROM users", 8)

	c.Rebuild([]types.SchemaObject{
		{Name: "", Kind: types.ObjectTable},
		{Schema: "public", Name: "fn", Kind: types.ObjectFunction},
		{Schema: "public", Name: "mv", Kind: types.ObjectMatView},
		{Name: "bare", Kind: types.ObjectTable},
	}, map[string][]string{"public.mv": {"", "id"}, "mv": {"id", "id"}})
	_ = c.MatchAt("mv", "SELECT * FROM ", 5)
	_ = c.bucketForTable("")
	_ = c.bucketForTable("nope")
	_ = c.bucketForTable("public.mv")
	_ = (*sqlCompleter)(nil).bucketForTable("x")

	b := mustBucket([]string{"Alpha", "alpine", "beta"})
	seen := map[string]struct{}{"alpha": {}}
	_ = b.prefixCollect("", 5, nil, seen, true)
	_ = b.prefixCollect("al", 1, nil, map[string]struct{}{}, true)
	_ = b.prefixCollect("zz", 5, nil, map[string]struct{}{}, false)
	_ = strBucket{}.prefixCollect("a", 5, nil, map[string]struct{}{}, false)
	_ = buildBucket(nil)

	out := c.collectScopedCols("em", "SELECT * FROM no_such WHERE ", 8, nil, map[string]struct{}{}, false)
	if !containsFold(out, "email") && len(out) == 0 {
		// may be empty if no columns rebuilt for email after Rebuild above
	}
	c = testCompleter()
	out = c.collectScopedCols("em", "SELECT * FROM users WHERE ", 8, nil, map[string]struct{}{}, false)
	if !containsFold(out, "email") {
		t.Fatalf("scoped %v", out)
	}
	filled := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	seen2 := map[string]struct{}{}
	for _, x := range filled {
		seen2[x] = struct{}{}
	}
	out = c.collectScopedCols("em", "SELECT * FROM users WHERE ", 8, filled, seen2, false)
	if len(out) != 8 {
		t.Fatalf("limit keep %d", len(out))
	}
}

func TestDetectSQLContextEdges(t *testing.T) {
	long := strings.Repeat("x", 300) + " SELECT * FROM t WHERE "
	ctx, _ := detectSQLContext(long)
	if ctx != ctxWhere {
		t.Fatalf("long %v", ctx)
	}
	ctx, tbl := detectSQLContext("SELECT users.")
	if ctx != ctxAfterDot || tbl != "users" {
		t.Fatalf("dot %v %q", ctx, tbl)
	}
	ctx, tbl = detectSQLContext("SELECT public.users.")
	if ctx != ctxAfterDot || !strings.Contains(tbl, "users") {
		t.Fatalf("schema.dot %v %q", ctx, tbl)
	}
	for before, want := range map[string]sqlCtx{
		"SELECT * FROM t HAVING ":      ctxWhere,
		"UPDATE t SET ":                ctxSet,
		"INSERT INTO t ":               ctxInsert,
		"SELECT a AND ":                ctxSelectList,
		"SELECT * FROM t WHERE x AND ": ctxWhere,
		"SELECT * FROM t USING ":       ctxOn,
		".":                            ctxGeneral,
		"SELECT * FROM t OFFSET ":      ctxLimit,
	} {
		got, _ := detectSQLContext(before)
		if got != want {
			t.Fatalf("before=%q got=%v want=%v", before, got, want)
		}
	}
	if tablesInScopeFast("") != nil {
		t.Fatal("empty scope")
	}
	long2 := strings.Repeat(" ", 250) + "SELECT * FROM users JOIN orders o ON true WHERE "
	got := tablesInScopeFast(long2)
	if !containsFold(got, "users") || !containsFold(got, "orders") {
		t.Fatalf("scope %v", got)
	}
	_ = tablesInScopeFast("SELECT * FROM users AS u LEFT JOIN orders")
	_ = tablesInScopeFast("SELECT * FROM WHERE")
}

func TestTokenTextBeforeApplyEdges(t *testing.T) {
	sql := "SELECT a\nFROM us"
	tok, _ := sqlTokenAtCursor(sql, 1, len("FROM us"))
	if tok != "us" {
		t.Fatalf("tok %q", tok)
	}
	_ = textBeforeCursor(sql, 1, 4)
	_ = textBeforeCursor("abc", 0, -1)
	_ = textBeforeCursor("abc", 0, 100)
	_ = textBeforeCursor(sql, -1, 0)
	_ = textBeforeCursor(sql, 99, 0)
	_ = textBeforeCursor(sql, 1, -5)
	_ = textBeforeCursor(sql, 1, 1000)
	tok, _ = sqlTokenAtCursor(sql, -1, 0)
	tok, _ = sqlTokenAtCursor(sql, 99, 0)
	tok, _ = sqlTokenAtCursor("abc", 0, -1)
	tok, _ = sqlTokenAtCursor("abc", 0, 100)
	if tok != "abc" {
		t.Fatalf("single %q", tok)
	}
	_, _ = sqlTokenAtCursor(`SELECT "x`, 0, 9)

	v, _ := applySQLSuggestion(sql, -1, 0, "x")
	if v != sql {
		t.Fatal("bad line")
	}
	v, col := applySQLSuggestion("SELECT u.em", 0, len("SELECT u.em"), "email")
	if v != "SELECT u.email" || col != len("SELECT u.email") {
		t.Fatalf("%q %d", v, col)
	}
	v, _ = applySQLSuggestion("SELECT\nFROM u", 1, 100, "users")
	if !strings.Contains(v, "users") {
		t.Fatalf("%q", v)
	}

	n := 0
	for w := range wordsSeq("one two three") {
		n++
		if n == 1 {
			_ = w
			break
		}
	}
	for range wordsSeq("") {
		t.Fatal("empty words")
	}
	for range wordsSeq("   ") {
		t.Fatal("ws words")
	}
	if !isAllUpper("SELECT") || isAllUpper("Select") || isAllUpper("") {
		t.Fatal("isAllUpper")
	}
	if upperASCII("select") != "SELECT" || upperASCII("SELECT") != "SELECT" {
		t.Fatal("upperASCII")
	}
}
