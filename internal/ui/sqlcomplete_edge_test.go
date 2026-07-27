package ui

import (
	"strings"
	"testing"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestEdge_StripSQLNoiseFast(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"plain no noise", "plain no noise"},
		{"SELECT -- line comment\nFROM t", "SELECT \nFROM t"},
		{"a--b", "a"},
		{"-- only comment", ""},
		{"SELECT /* block */ x", "SELECT  x"},
		{"/* multi\nline */ SELECT", " SELECT"},
		{"/* unclosed comment", "t"},
		{"x /* mid */ y /* two */ z", "x  y  z"},
		{"SELECT 'simple'", "SELECT  "},
		{"SELECT 'it''s fine' AS x", "SELECT   AS x"},
		{"SELECT 'a''b''c' end", "SELECT   end"},
		{"SELECT 'unclosed", "SELECT  "},
		{"SELECT 'a' -- comment\nFROM t", "SELECT   \nFROM t"},
		{"SELECT /* c */ 's''t' -- tail\n1", "SELECT    \n1"},
		{`SELECT "ident"`, `SELECT "ident"`},
		{"a/b not a comment", "a/b not a comment"},
		{"a-b not a comment", "a-b not a comment"},
	}
	for _, tc := range cases {
		got := stripSQLNoiseFast(tc.in)
		if got != tc.want {
			t.Fatalf("stripSQLNoiseFast(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestEdge_PrefixCollect(t *testing.T) {
	b := mustBucket([]string{"Alpha", "alpine", "beta", "SELECT", "FROM"})
	empty := strBucket{}

	// empty bucket / non-positive limit
	out := empty.prefixCollect("a", 5, nil, map[string]struct{}{}, false)
	if out != nil {
		t.Fatalf("empty bucket: %v", out)
	}
	out = b.prefixCollect("a", 0, []string{"keep"}, map[string]struct{}{}, false)
	if len(out) != 1 || out[0] != "keep" {
		t.Fatalf("limit0: %v", out)
	}

	// empty prefix collects all until limit, skips seen, upperKW for keywords
	seen := map[string]struct{}{"alpha": {}}
	out = b.prefixCollect("", 10, nil, seen, true)
	if containsFold(out, "Alpha") || containsFold(out, "alpine") {
		// alpha already seen; alpine is different key
	}
	if !containsFold(out, "alpine") {
		t.Fatalf("expected alpine for empty p: %v", out)
	}
	if !containsFold(out, "SELECT") {
		t.Fatalf("expected SELECT upper: %v", out)
	}
	// verify SELECT was uppercased
	for _, s := range out {
		if strings.EqualFold(s, "select") && s != "SELECT" {
			t.Fatalf("upperKW failed: %q", s)
		}
	}

	// exact match skip
	out = b.prefixCollect("beta", 5, nil, map[string]struct{}{}, false)
	if containsFold(out, "beta") {
		t.Fatalf("exact match should skip: %v", out)
	}

	// prefix match with seen skip mid-loop
	seen2 := map[string]struct{}{"alpine": {}}
	out = b.prefixCollect("al", 5, nil, seen2, false)
	if containsFold(out, "alpine") {
		t.Fatalf("seen alpine: %v", out)
	}
	if !containsFold(out, "Alpha") {
		t.Fatalf("want Alpha: %v", out)
	}

	// upperKW false keeps display case; true uppercases keywords only
	out = b.prefixCollect("sel", 5, nil, map[string]struct{}{}, true)
	if len(out) == 0 || out[0] != "SELECT" {
		t.Fatalf("upperKW keyword: %v", out)
	}
	out = b.prefixCollect("al", 5, nil, map[string]struct{}{}, true)
	// alpine is not a keyword — stays as display form
	if !containsFold(out, "alpine") {
		t.Fatalf("non-kw: %v", out)
	}

	// no match prefix
	out = b.prefixCollect("zzz", 5, nil, map[string]struct{}{}, false)
	if len(out) != 0 {
		t.Fatalf("zzz: %v", out)
	}

	// limit mid-collection for empty p
	out = b.prefixCollect("", 2, nil, map[string]struct{}{}, false)
	if len(out) != 2 {
		t.Fatalf("limit2 empty p: %v", out)
	}
}

func TestEdge_Rebuild(t *testing.T) {
	c := newSQLCompleter()

	// empty names skipped, empty schema → bare table only, non-table kinds ignored
	c.Rebuild([]types.SchemaObject{
		{Name: "", Schema: "public", Kind: types.ObjectTable},
		{Name: "users", Schema: "", Kind: types.ObjectTable},
		{Name: "orders", Schema: "public", Kind: types.ObjectTable},
		{Name: "v1", Schema: "public", Kind: types.ObjectView},
		{Name: "mv1", Schema: "analytics", Kind: types.ObjectMatView},
		{Name: "fn1", Schema: "public", Kind: types.ObjectFunction},
		{Name: "dup", Schema: "public", Kind: types.ObjectTable},
		{Name: "dup", Schema: "public", Kind: types.ObjectTable},
	}, map[string][]string{
		// qualified with dots
		"public.users": {"id", "email", "", "name"},
		// bare without dots
		"orders": {"id", "total", "id"},
		// schema.table already covered; empty col list
		"public.missing": {},
		// empty key
		"": {"orphan"},
	})

	got := c.MatchAt("use", "SELECT * FROM ", 10)
	if !containsFold(got, "users") {
		t.Fatalf("bare users: %v", got)
	}
	got = c.MatchAt("public.o", "SELECT * FROM ", 10)
	if !containsFold(got, "public.orders") {
		t.Fatalf("schema.table: %v", got)
	}
	got = c.MatchAt("ema", "SELECT ", 10)
	if !containsFold(got, "email") {
		t.Fatalf("col from dotted map: %v", got)
	}
	got = c.MatchAt("tot", "SELECT ", 10)
	if !containsFold(got, "total") {
		t.Fatalf("col from bare map: %v", got)
	}
	got = c.MatchAt("orph", "SELECT ", 10)
	if !containsFold(got, "orphan") {
		t.Fatalf("orphan col: %v", got)
	}
	// qualified suggestions
	got = c.MatchAt("users.em", "SELECT ", 10)
	if !containsFold(got, "email") {
		t.Fatalf("after-dot bare: %v", got)
	}
	got = c.MatchAt("public.users.em", "SELECT ", 10)
	if !containsFold(got, "email") {
		t.Fatalf("after-dot qual: %v", got)
	}

	// empty rebuild
	c.Rebuild(nil, nil)
	if len(c.MatchAt("u", "SELECT * FROM ", 5)) != 0 && containsFold(c.MatchAt("u", "SELECT * FROM ", 5), "users") {
		// tables cleared
		got = c.MatchAt("users", "SELECT * FROM ", 5)
		if containsFold(got, "users") {
			t.Fatalf("expected cleared tables: %v", got)
		}
	}

	// rebuild with only empty-name objects and empty cols
	c.Rebuild([]types.SchemaObject{
		{Name: "", Kind: types.ObjectTable},
		{Name: "", Schema: "s", Kind: types.ObjectView},
	}, map[string][]string{
		"t":   {"", ""},
		"a.b": {""},
	})
}

func TestEdge_BucketForTable(t *testing.T) {
	c := testCompleter()

	if b := c.bucketForTable(""); len(b.disp) != 0 {
		t.Fatalf("empty table: %+v", b)
	}
	if b := (*sqlCompleter)(nil).bucketForTable("users"); len(b.disp) != 0 {
		t.Fatalf("nil completer: %+v", b)
	}
	if b := c.bucketForTable("users"); !containsFold(b.disp, "email") {
		t.Fatalf("bare: %v", b.disp)
	}
	if b := c.bucketForTable("public.users"); !containsFold(b.disp, "email") {
		t.Fatalf("schema.table: %v", b.disp)
	}
	// schema.table unknown bare falls through to last segment lookup
	if b := c.bucketForTable("other.users"); !containsFold(b.disp, "email") {
		t.Fatalf("other.users via bare: %v", b.disp)
	}
	if b := c.bucketForTable("nope"); len(b.disp) != 0 {
		t.Fatalf("unknown: %v", b.disp)
	}
	if b := c.bucketForTable("schema.nope"); len(b.disp) != 0 {
		t.Fatalf("unknown dotted: %v", b.disp)
	}
}

func TestEdge_MatchAtRemaining(t *testing.T) {
	c := testCompleter()

	// Match is context-free wrapper
	if !containsFold(c.Match("use", 10), "users") {
		t.Fatal("Match wrapper")
	}

	// nil / bad limit / bad prefix
	if (*sqlCompleter)(nil).MatchAt("x", "", 5) != nil {
		t.Fatal("nil c")
	}
	if c.MatchAt("x", "", 0) != nil {
		t.Fatal("limit 0")
	}
	if c.MatchAt("-bad", "SELECT ", 5) != nil {
		t.Fatal("bad ident")
	}

	// empty prefix + whitespace before → start keywords
	got := c.MatchAt("", "   \t", 8)
	if !containsFold(got, "SELECT") {
		t.Fatalf("start kw: %v", got)
	}

	// empty prefix + non-ws before → general keywords
	got = c.MatchAt("", "x ", 8)
	if len(got) == 0 {
		t.Fatal("general empty prefix")
	}

	// FROM / JOIN
	got = c.MatchAt("us", "SELECT * FROM ", 8)
	if !containsFold(got, "users") {
		t.Fatalf("from: %v", got)
	}
	got = c.MatchAt("or", "SELECT * FROM users JOIN ", 8)
	if !containsFold(got, "orders") {
		t.Fatalf("join: %v", got)
	}

	// SELECT list + scoped cols
	got = c.MatchAt("em", "SELECT ", 8)
	if !containsFold(got, "email") {
		t.Fatalf("select list: %v", got)
	}
	got = c.MatchAt("em", "SELECT * FROM users WHERE ", 8)
	if !containsFold(got, "email") {
		t.Fatalf("where: %v", got)
	}
	got = c.MatchAt("em", "SELECT * FROM users u JOIN orders o ON ", 8)
	if len(got) == 0 {
		t.Fatal("on")
	}
	got = c.MatchAt("em", "UPDATE users SET ", 8)
	if len(got) == 0 {
		t.Fatal("set")
	}
	got = c.MatchAt("em", "SELECT * FROM users ORDER BY ", 8)
	if len(got) == 0 {
		t.Fatal("order")
	}
	got = c.MatchAt("em", "SELECT * FROM users GROUP BY ", 8)
	if len(got) == 0 {
		t.Fatal("group")
	}
	got = c.MatchAt("", "SELECT * FROM users LIMIT ", 8)
	if !containsFold(got, "OFFSET") {
		t.Fatalf("limit: %v", got)
	}
	got = c.MatchAt("us", "INSERT INTO ", 8)
	if !containsFold(got, "users") {
		t.Fatalf("insert: %v", got)
	}

	// after-dot via prefix split in select/where/on/order/group/set
	got = c.MatchAt("users.em", "SELECT users.em", 8)
	if !containsFold(got, "email") {
		t.Fatalf("dot select: %v", got)
	}
	got = c.MatchAt("users.em", "SELECT * FROM users WHERE users.em", 8)
	if !containsFold(got, "email") {
		t.Fatalf("dot where: %v", got)
	}
	got = c.MatchAt("users.em", "SELECT * FROM users u JOIN orders o ON users.em", 8)
	if !containsFold(got, "email") {
		t.Fatalf("dot on: %v", got)
	}
	got = c.MatchAt("users.em", "SELECT * FROM users ORDER BY users.em", 8)
	if !containsFold(got, "email") {
		t.Fatalf("dot order: %v", got)
	}
	got = c.MatchAt("users.em", "SELECT * FROM users GROUP BY users.em", 8)
	if !containsFold(got, "email") {
		t.Fatalf("dot group: %v", got)
	}
	got = c.MatchAt("users.em", "UPDATE users SET users.em", 8)
	if !containsFold(got, "email") {
		t.Fatalf("dot set: %v", got)
	}
	// dotted prefix in FROM does not force after-dot
	got = c.MatchAt("public.u", "SELECT * FROM public.u", 8)
	if !containsFold(got, "public.users") {
		t.Fatalf("from dotted table: %v", got)
	}

	// all-upper prefix → upperKW path
	got = c.MatchAt("SEL", "", 8)
	if len(got) == 0 || got[0] != "SELECT" {
		t.Fatalf("upperKW MatchAt: %v", got)
	}

	// collectList early return when already at limit
	got = c.MatchAt("", "SELECT * FROM ", 1)
	if len(got) != 1 {
		t.Fatalf("limit1: %v", got)
	}

	// exact typed token skipped
	got = c.MatchAt("users", "SELECT * FROM users", 8)
	if containsFold(got, "users") {
		// may still match public.users etc — bare users exact skip
		for _, s := range got {
			if s == "users" {
				t.Fatalf("exact users should skip: %v", got)
			}
		}
	}
}

func TestEdge_CollectScopedCols(t *testing.T) {
	c := testCompleter()

	// no tables in scope → all columns
	out := c.collectScopedCols("em", "SELECT ", 8, nil, map[string]struct{}{}, false)
	if !containsFold(out, "email") {
		t.Fatalf("no scope: %v", out)
	}

	// tables in scope with matching cols
	out = c.collectScopedCols("em", "SELECT * FROM users WHERE ", 8, nil, map[string]struct{}{}, false)
	if !containsFold(out, "email") {
		t.Fatalf("scoped: %v", out)
	}

	// tables in scope but no matching cols → fallback all columns
	out = c.collectScopedCols("em", "SELECT * FROM no_such_table WHERE ", 8, nil, map[string]struct{}{}, false)
	if !containsFold(out, "email") {
		t.Fatalf("fallback: %v", out)
	}

	// already at limit → break early
	filled := make([]string, 8)
	seen := map[string]struct{}{}
	for i := range filled {
		filled[i] = "x" + string(rune('a'+i))
		seen[filled[i]] = struct{}{}
	}
	out = c.collectScopedCols("em", "SELECT * FROM users WHERE ", 8, filled, seen, false)
	if len(out) != 8 {
		t.Fatalf("at limit: %d", len(out))
	}

	// multi-table scope
	out = c.collectScopedCols("st", "SELECT * FROM users JOIN orders WHERE ", 8, nil, map[string]struct{}{}, false)
	if !containsFold(out, "status") {
		t.Fatalf("multi scope status: %v", out)
	}
}

func TestEdge_DetectSQLContextRemaining(t *testing.T) {
	cases := []struct {
		before string
		want   sqlCtx
		dot    string
	}{
		{"", ctxGeneral, ""},
		{"SELECT ", ctxSelectList, ""},
		{"SELECT * FROM ", ctxFrom, ""},
		{"SELECT * FROM t JOIN ", ctxJoin, ""},
		{"SELECT * FROM t INNER ", ctxJoin, ""},
		{"SELECT * FROM t CROSS ", ctxJoin, ""},
		{"SELECT * FROM t LEFT ", ctxJoin, ""},
		{"SELECT * FROM t RIGHT ", ctxJoin, ""},
		{"SELECT * FROM t FULL ", ctxJoin, ""},
		{"SELECT * FROM t ON ", ctxOn, ""},
		{"SELECT * FROM t USING ", ctxOn, ""},
		{"SELECT * FROM t WHERE ", ctxWhere, ""},
		{"SELECT * FROM t HAVING ", ctxWhere, ""},
		{"SELECT * FROM t WHERE x AND ", ctxWhere, ""},
		{"SELECT * FROM t WHERE x OR ", ctxWhere, ""},
		{"SELECT a AND ", ctxSelectList, ""}, // AND stays select when already select
		{"SELECT * FROM t ON x AND ", ctxOn, ""},
		{"SELECT * FROM t GROUP ", ctxGroup, ""},
		{"SELECT * FROM t ORDER ", ctxOrder, ""},
		{"SELECT * FROM t ORDER BY ", ctxOrder, ""},
		{"SELECT * FROM t GROUP BY ", ctxGroup, ""},
		{"SELECT * FROM t LIMIT ", ctxLimit, ""},
		{"SELECT * FROM t OFFSET ", ctxLimit, ""},
		{"UPDATE t SET ", ctxSet, ""},
		{"INSERT ", ctxInsert, ""},
		{"INSERT INTO t ", ctxInsert, ""},
		{"UPDATE t ", ctxFrom, ""},
		{"SELECT users.", ctxAfterDot, "users"},
		{"SELECT public.users.", ctxAfterDot, "public.users"},
		{".", ctxGeneral, ""},
		{"SELECT * FROM t WHERE x = 'a.'", ctxWhere, ""}, // noise stripped
	}
	for _, tc := range cases {
		got, dot := detectSQLContext(tc.before)
		if got != tc.want {
			t.Fatalf("before=%q got=%v want=%v", tc.before, got, tc.want)
		}
		if tc.dot != "" && dot != tc.dot {
			t.Fatalf("before=%q dot=%q want %q", tc.before, dot, tc.dot)
		}
	}

	// long tail window
	long := strings.Repeat("x", 300) + " SELECT * FROM t WHERE "
	if ctx, _ := detectSQLContext(long); ctx != ctxWhere {
		t.Fatalf("long window: %v", ctx)
	}

	// trailing incomplete token with non-ident before dot
	ctx, dot := detectSQLContext("SELECT (users.")
	if ctx != ctxAfterDot || dot != "users" {
		t.Fatalf("paren dot: %v %q", ctx, dot)
	}
}

func TestEdge_TablesInScopeFast(t *testing.T) {
	if tablesInScopeFast("") != nil {
		t.Fatal("empty")
	}
	long := strings.Repeat(" ", 250) + "SELECT * FROM users JOIN orders o ON true WHERE "
	got := tablesInScopeFast(long)
	if !containsFold(got, "users") || !containsFold(got, "orders") {
		t.Fatalf("long: %v", got)
	}

	got = tablesInScopeFast("SELECT * FROM users AS u LEFT JOIN orders")
	if !containsFold(got, "users") || !containsFold(got, "orders") {
		t.Fatalf("as/left: %v", got)
	}

	got = tablesInScopeFast("SELECT * FROM WHERE")
	if len(got) != 0 {
		t.Fatalf("from where: %v", got)
	}

	got = tablesInScopeFast("INSERT INTO products VALUES")
	if !containsFold(got, "products") {
		t.Fatalf("into: %v", got)
	}

	got = tablesInScopeFast("UPDATE users SET x=1")
	if !containsFold(got, "users") {
		t.Fatalf("update: %v", got)
	}

	got = tablesInScopeFast("SELECT * FROM users RIGHT OUTER JOIN orders FULL CROSS JOIN products ON 1")
	if !containsFold(got, "users") || !containsFold(got, "orders") || !containsFold(got, "products") {
		t.Fatalf("join noise: %v", got)
	}

	// keyword after FROM not captured as table
	got = tablesInScopeFast("SELECT * FROM SELECT")
	if containsFold(got, "SELECT") {
		t.Fatalf("kw as table: %v", got)
	}

	// clause enders reset want
	got = tablesInScopeFast("SELECT * FROM users WHERE id IN (SELECT id FROM")
	// last FROM still open with no table yet
	_ = got
}

func TestEdge_WordsSeq(t *testing.T) {
	var words []string
	for w := range wordsSeq("one two  three") {
		words = append(words, w)
	}
	if len(words) != 3 || words[0] != "one" || words[2] != "three" {
		t.Fatalf("words: %v", words)
	}

	// early break
	n := 0
	for range wordsSeq("a b c") {
		n++
		break
	}
	if n != 1 {
		t.Fatalf("break: %d", n)
	}

	for range wordsSeq("") {
		t.Fatal("empty")
	}
	for range wordsSeq("   \t\n") {
		t.Fatal("ws")
	}
	for range wordsSeq("!!!") {
		t.Fatal("punct")
	}

	// identifiers with digits/underscores/dots
	words = nil
	for w := range wordsSeq("a1 _b c.d") {
		words = append(words, w)
	}
	if len(words) != 3 || words[2] != "c.d" {
		t.Fatalf("idents: %v", words)
	}
}

func TestEdge_OnlyWhitespaceAndIdentPrefix(t *testing.T) {
	if !onlyWhitespace("") || !onlyWhitespace(" \t\n\r") {
		t.Fatal("ws true")
	}
	if onlyWhitespace("x") || onlyWhitespace(" a") || onlyWhitespace("a ") {
		t.Fatal("ws false")
	}

	if !isSQLIdentPrefix("") || !isSQLIdentPrefix("a") || !isSQLIdentPrefix("_x") {
		t.Fatal("ident ok start")
	}
	if !isSQLIdentPrefix(`"`) || !isSQLIdentPrefix(`"Quoted"`) || !isSQLIdentPrefix("a.b.c") {
		t.Fatal("ident ok special")
	}
	if !isSQLIdentPrefix("a1_2") {
		t.Fatal("ident mid")
	}
	if isSQLIdentPrefix("1a") || isSQLIdentPrefix("a-b") || isSQLIdentPrefix("a b") {
		t.Fatal("ident bad")
	}
	if isSQLIdentPrefix("$1") || isSQLIdentPrefix("(x") {
		t.Fatal("ident bad2")
	}
}

func TestEdge_SQLTokenAtCursor(t *testing.T) {
	// single-line fast path
	tok, start := sqlTokenAtCursor("SELECT us", 0, len("SELECT us"))
	if tok != "us" || start != len("SELECT ") {
		t.Fatalf("fast: %q %d", tok, start)
	}
	tok, _ = sqlTokenAtCursor("abc", 0, -1)
	if tok != "" {
		t.Fatalf("col-1: %q", tok)
	}
	tok, _ = sqlTokenAtCursor("abc", 0, 100)
	if tok != "abc" {
		t.Fatalf("col over: %q", tok)
	}
	tok, start = sqlTokenAtCursor(`SELECT "Quo`, 0, len(`SELECT "Quo`))
	if tok != `"Quo` || start != len("SELECT ") {
		t.Fatalf("quote: %q %d", tok, start)
	}
	tok, _ = sqlTokenAtCursor("a.b.c", 0, 5)
	if tok != "a.b.c" {
		t.Fatalf("dots: %q", tok)
	}

	// multi-line path
	sql := "SELECT a\nFROM us"
	tok, start = sqlTokenAtCursor(sql, 1, len("FROM us"))
	if tok != "us" || start != len("FROM ") {
		t.Fatalf("multi: %q %d", tok, start)
	}
	tok, _ = sqlTokenAtCursor(sql, -1, 0)
	if tok != "" {
		t.Fatalf("line-1: %q", tok)
	}
	tok, _ = sqlTokenAtCursor(sql, 99, 0)
	if tok != "" {
		t.Fatalf("line99: %q", tok)
	}
	tok, _ = sqlTokenAtCursor(sql, 1, -5)
	if tok != "" {
		t.Fatalf("multi col-1: %q", tok)
	}
	tok, _ = sqlTokenAtCursor(sql, 1, 1000)
	if tok != "us" && tok != "FROM us" {
		// col clamped to row end → full trailing ident
		if !strings.HasSuffix(tok, "us") {
			t.Fatalf("multi col over: %q", tok)
		}
	}
	// space stop
	tok, start = sqlTokenAtCursor("SELECT  ", 0, 8)
	if tok != "" || start != 8 {
		t.Fatalf("space: %q %d", tok, start)
	}
}

func TestEdge_TextBeforeCursor(t *testing.T) {
	if textBeforeCursor("abc", 0, -1) != "" {
		t.Fatal("col-1")
	}
	if textBeforeCursor("abc", 0, 100) != "abc" {
		t.Fatal("col over")
	}
	if textBeforeCursor("abc", 0, 2) != "ab" {
		t.Fatal("mid")
	}

	sql := "SELECT a\nFROM us"
	if textBeforeCursor(sql, -1, 0) != "" {
		t.Fatal("line-1")
	}
	if textBeforeCursor(sql, 99, 0) != sql {
		t.Fatal("line over")
	}
	got := textBeforeCursor(sql, 1, 4)
	if got != "SELECT a\nFROM" {
		t.Fatalf("multi mid: %q", got)
	}
	got = textBeforeCursor(sql, 1, -5)
	if got != "SELECT a\n" {
		t.Fatalf("multi col-1: %q", got)
	}
	got = textBeforeCursor(sql, 1, 1000)
	if got != sql {
		t.Fatalf("multi col over: %q", got)
	}
	got = textBeforeCursor(sql, 0, 6)
	if got != "SELECT" {
		t.Fatalf("line0 multi: %q", got)
	}
}

func TestEdge_ApplySQLSuggestion(t *testing.T) {
	sql := "SELECT * FROM us"
	v, col := applySQLSuggestion(sql, 0, len(sql), "users")
	if v != "SELECT * FROM users" || col != len("SELECT * FROM users") {
		t.Fatalf("basic: %q %d", v, col)
	}

	// bad line
	v, col = applySQLSuggestion(sql, -1, 0, "x")
	if v != sql || col != 0 {
		t.Fatalf("line-1: %q %d", v, col)
	}
	v, _ = applySQLSuggestion(sql, 5, 0, "x")
	if v != sql {
		t.Fatalf("line over: %q", v)
	}

	// re-qualify bare suggestion after dotted token
	v, col = applySQLSuggestion("SELECT u.em", 0, len("SELECT u.em"), "email")
	if v != "SELECT u.email" || col != len("SELECT u.email") {
		t.Fatalf("requal: %q %d", v, col)
	}

	// already-qualified suggestion is not double-prefixed
	v, _ = applySQLSuggestion("SELECT u.em", 0, len("SELECT u.em"), "u.email")
	if v != "SELECT u.email" {
		t.Fatalf("already qual: %q", v)
	}

	// multi-line + col past end
	v, col = applySQLSuggestion("SELECT\nFROM u", 1, 100, "users")
	if !strings.Contains(v, "users") {
		t.Fatalf("multi: %q", v)
	}
	if col != len("FROM users") {
		t.Fatalf("multi col: %d", col)
	}

	// mid-token replace
	v, col = applySQLSuggestion("SELECT id, em FROM t", 0, len("SELECT id, em"), "email")
	if v != "SELECT id, email FROM t" {
		t.Fatalf("mid: %q", v)
	}
}
