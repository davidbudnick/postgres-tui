package ui

import (
	"sort"
	"strings"
	"unicode"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

// SQL keywords offered as completions (uppercase insert).
var sqlKeywords = []string{
	"SELECT", "FROM", "WHERE", "AND", "OR", "NOT", "IN", "IS", "NULL",
	"ORDER", "BY", "GROUP", "HAVING", "LIMIT", "OFFSET", "JOIN", "LEFT",
	"RIGHT", "INNER", "OUTER", "FULL", "CROSS", "ON", "AS", "DISTINCT",
	"COUNT", "SUM", "AVG", "MIN", "MAX", "INSERT", "INTO", "VALUES",
	"UPDATE", "SET", "DELETE", "CREATE", "TABLE", "VIEW", "INDEX", "DROP",
	"ALTER", "EXPLAIN", "ANALYZE", "WITH", "UNION", "ALL", "CASE", "WHEN",
	"THEN", "ELSE", "END", "BETWEEN", "LIKE", "ILIKE", "RETURNING", "TRUE",
	"FALSE", "ASC", "DESC", "EXISTS", "CAST", "COALESCE", "NULLIF", "NOW",
}

var (
	kwAfterSelect = []string{"DISTINCT", "COUNT", "SUM", "AVG", "MIN", "MAX", "CASE", "CAST", "COALESCE", "FROM", "AS", "NULL"}
	kwAfterFrom   = []string{"WHERE", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "FULL", "CROSS", "ON", "AS", "ORDER", "GROUP", "LIMIT", "OFFSET"}
	kwAfterWhere  = []string{"AND", "OR", "NOT", "IN", "IS", "NULL", "BETWEEN", "LIKE", "ILIKE", "EXISTS", "ORDER", "GROUP", "LIMIT", "HAVING"}
	kwAfterOrder  = []string{"BY", "ASC", "DESC", "LIMIT", "OFFSET"}
	kwAfterGroup  = []string{"BY", "HAVING", "ORDER", "LIMIT"}
	kwStart       = []string{"SELECT", "WITH", "INSERT", "UPDATE", "DELETE", "EXPLAIN", "CREATE", "DROP", "ALTER"}
	kwSelectFns   = []string{"COUNT", "SUM", "AVG", "MIN", "MAX", "COALESCE", "NULLIF", "CAST", "NOW"}
)

// Pre-sorted keyword buckets with lowercase companions (built once).
var (
	kwAfterSelectB = mustBucket(kwAfterSelect)
	kwAfterFromB   = mustBucket(kwAfterFrom)
	kwAfterWhereB  = mustBucket(kwAfterWhere)
	kwAfterOrderB  = mustBucket(kwAfterOrder)
	kwAfterGroupB  = mustBucket(kwAfterGroup)
	kwStartB       = mustBucket(kwStart)
	kwSelectFnsB   = mustBucket(kwSelectFns)
	kwAllB         = mustBucket(sqlKeywords)
)

type sqlCtx int

const (
	ctxGeneral sqlCtx = iota
	ctxSelectList
	ctxFrom
	ctxJoin
	ctxOn
	ctxWhere
	ctxOrder
	ctxGroup
	ctxLimit
	ctxSet
	ctxInsert
	ctxAfterDot
)

// strBucket is a sorted display+lower pair for O(log n) prefix search.
type strBucket struct {
	disp  []string
	lower []string
}

func mustBucket(items []string) strBucket {
	disp := append([]string(nil), items...)
	sort.Slice(disp, func(i, j int) bool {
		return strings.ToLower(disp[i]) < strings.ToLower(disp[j])
	})
	low := make([]string, len(disp))
	for i, s := range disp {
		low[i] = strings.ToLower(s)
	}
	return strBucket{disp: disp, lower: low}
}

func buildBucket(items []string) strBucket {
	if len(items) == 0 {
		return strBucket{}
	}
	return mustBucket(items)
}

// prefixCollect appends up to remaining matches into out (no alloc for empty).
func (b strBucket) prefixCollect(p string, limit int, out []string, seen map[string]struct{}, upperKW bool) []string {
	if limit <= 0 || len(b.disp) == 0 {
		return out
	}
	if p == "" {
		for i := 0; i < len(b.disp) && len(out) < limit; i++ {
			k := b.lower[i]
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			cand := b.disp[i]
			if upperKW && isSQLKeywordFast(k) {
				cand = strings.ToUpper(cand)
			}
			out = append(out, cand)
		}
		return out
	}
	// Binary search first index with lower >= p
	i := sort.Search(len(b.lower), func(i int) bool {
		return b.lower[i] >= p
	})
	for ; i < len(b.lower) && len(out) < limit; i++ {
		if !strings.HasPrefix(b.lower[i], p) {
			break
		}
		if b.lower[i] == p {
			continue // exact already typed
		}
		k := b.lower[i]
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		cand := b.disp[i]
		if upperKW && isSQLKeywordFast(k) {
			cand = strings.ToUpper(cand)
		}
		out = append(out, cand)
	}
	return out
}

// sqlCompleter is a context-aware in-memory SQL autocomplete index.
type sqlCompleter struct {
	keywords  strBucket
	tables    strBucket
	columns   strBucket
	qualified strBucket
	colByTbl  map[string]strBucket // lower table key → column bucket
}

func newSQLCompleter() *sqlCompleter {
	c := &sqlCompleter{colByTbl: map[string]strBucket{}}
	c.Rebuild(nil, nil)
	return c
}

// Rebuild rebuilds candidate buckets from schema objects + column cache.
func (c *sqlCompleter) Rebuild(objs []types.SchemaObject, cols map[string][]string) {
	seenT := make(map[string]struct{}, 64)
	seenC := make(map[string]struct{}, 64)
	seenQ := make(map[string]struct{}, 64)
	var tables, columns, qualified []string
	colBy := map[string][]string{}

	add := func(dst *[]string, seen map[string]struct{}, s string) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		*dst = append(*dst, s)
	}

	for _, o := range objs {
		if o.Name == "" {
			continue
		}
		switch o.Kind {
		case types.ObjectTable, types.ObjectView, types.ObjectMatView, "":
			add(&tables, seenT, o.Name)
			if o.Schema != "" {
				add(&tables, seenT, o.Schema+"."+o.Name)
			}
		}
	}
	for qual, list := range cols {
		bare := qual
		if i := strings.LastIndexByte(qual, '.'); i >= 0 {
			bare = qual[i+1:]
		}
		for _, col := range list {
			add(&columns, seenC, col)
			if col == "" {
				continue
			}
			add(&qualified, seenQ, bare+"."+col)
			if bare != qual {
				add(&qualified, seenQ, qual+"."+col)
			}
		}
		lb, lq := strings.ToLower(bare), strings.ToLower(qual)
		colBy[lb] = appendUnique(colBy[lb], list)
		if lq != lb {
			colBy[lq] = appendUnique(colBy[lq], list)
		}
	}

	c.keywords = kwAllB
	c.tables = buildBucket(tables)
	c.columns = buildBucket(columns)
	c.qualified = buildBucket(qualified)
	c.colByTbl = make(map[string]strBucket, len(colBy))
	for k, v := range colBy {
		c.colByTbl[k] = buildBucket(v)
	}
}

func appendUnique(dst, src []string) []string {
	seen := make(map[string]struct{}, len(dst)+len(src))
	for _, s := range dst {
		seen[strings.ToLower(s)] = struct{}{}
	}
	for _, s := range src {
		k := strings.ToLower(s)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		dst = append(dst, s)
	}
	return dst
}

// Match is context-free (tests / fallback).
func (c *sqlCompleter) Match(prefix string, limit int) []string {
	return c.MatchAt(prefix, "", limit)
}

// MatchAt returns clause-ranked suggestions. Hot path: binary search only, tiny allocs.
func (c *sqlCompleter) MatchAt(prefix, before string, limit int) []string {
	if c == nil || limit <= 0 {
		return nil
	}
	if prefix != "" && !isSQLIdentPrefix(prefix) {
		return nil
	}

	ctx, dotTable := detectSQLContext(before)
	fullPrefix := prefix
	if i := strings.LastIndexByte(prefix, '.'); i >= 0 {
		switch ctx {
		case ctxSelectList, ctxWhere, ctxOn, ctxOrder, ctxGroup, ctxSet:
			dotTable = prefix[:i]
			prefix = prefix[i+1:]
			ctx = ctxAfterDot
		}
	}

	p := strings.ToLower(prefix)
	upperKW := isAllUpper(prefix) && prefix != ""
	seen := make(map[string]struct{}, limit*2)
	out := make([]string, 0, limit)

	collect := func(b strBucket) {
		out = b.prefixCollect(p, limit, out, seen, upperKW)
	}
	collectList := func(b strBucket) {
		if len(out) >= limit {
			return
		}
		out = b.prefixCollect(p, limit, out, seen, upperKW)
	}

	switch ctx {
	case ctxFrom, ctxJoin:
		collect(c.tables)
		collectList(kwAfterFromB)
	case ctxSelectList:
		// scoped columns first
		out = c.collectScopedCols(p, before, limit, out, seen, upperKW)
		collectList(kwSelectFnsB)
		collectList(c.columns)
		collectList(kwAfterSelectB)
		collectList(c.tables)
	case ctxWhere, ctxOn, ctxSet:
		out = c.collectScopedCols(p, before, limit, out, seen, upperKW)
		collectList(c.columns)
		collectList(c.qualified)
		collectList(kwAfterWhereB)
	case ctxOrder:
		out = c.collectScopedCols(p, before, limit, out, seen, upperKW)
		collectList(c.columns)
		collectList(kwAfterOrderB)
	case ctxGroup:
		out = c.collectScopedCols(p, before, limit, out, seen, upperKW)
		collectList(c.columns)
		collectList(kwAfterGroupB)
	case ctxLimit:
		collectList(mustBucket([]string{"OFFSET"}))
	case ctxAfterDot:
		b := c.bucketForTable(dotTable)
		// When user typed table.xxx, suggest bare columns; applySQLSuggestion re-qualifies.
		n := len(out)
		out = b.prefixCollect(p, limit, out, seen, false)
		_ = n
		_ = fullPrefix
	case ctxInsert:
		collect(c.tables)
		collectList(mustBucket([]string{"INTO", "VALUES", "SELECT"}))
	default:
		if onlyWhitespace(before) {
			collectList(kwStartB)
			collectList(c.tables)
			collectList(c.columns)
		} else {
			collectList(c.keywords)
			collectList(c.tables)
			collectList(c.columns)
		}
	}
	return out
}

func (c *sqlCompleter) bucketForTable(table string) strBucket {
	if table == "" || c == nil {
		return strBucket{}
	}
	t := strings.ToLower(table)
	if b, ok := c.colByTbl[t]; ok {
		return b
	}
	if i := strings.LastIndexByte(t, '.'); i >= 0 {
		if b, ok := c.colByTbl[t[i+1:]]; ok {
			return b
		}
	}
	return strBucket{}
}

func (c *sqlCompleter) collectScopedCols(p, before string, limit int, out []string, seen map[string]struct{}, upperKW bool) []string {
	// Fast path: grab tables from a short window at the end of before.
	scope := tablesInScopeFast(before)
	if len(scope) == 0 {
		return c.columns.prefixCollect(p, limit, out, seen, upperKW)
	}
	for _, t := range scope {
		if len(out) >= limit {
			break
		}
		out = c.bucketForTable(t).prefixCollect(p, limit, out, seen, upperKW)
	}
	if len(out) == 0 {
		return c.columns.prefixCollect(p, limit, out, seen, upperKW)
	}
	return out
}

// detectSQLContext — reverse scan last ~200 bytes for the active clause. O(n) tiny n.
func detectSQLContext(before string) (ctx sqlCtx, dotTable string) {
	if before == "" {
		return ctxGeneral, ""
	}
	// Only inspect a tail window (keystrokes don't need full history).
	if len(before) > 240 {
		before = before[len(before)-240:]
	}
	// Skip trailing incomplete token for keyword detection.
	s := stripSQLNoiseFast(before)

	trim := strings.TrimRight(s, " \t\n\r")
	if strings.HasSuffix(trim, ".") {
		i := len(trim) - 1
		j := i
		for j > 0 {
			c := trim[j-1]
			if isIdentByte(c) || c == '.' {
				j--
				continue
			}
			break
		}
		if j < i {
			return ctxAfterDot, trim[j:i]
		}
	}

	// Walk words left-to-right on the tail (small).
	ctx = ctxGeneral
	var prev string
	for w := range wordsSeq(s) {
		u := upperASCII(w)
		switch u {
		case "SELECT":
			ctx = ctxSelectList
		case "FROM":
			ctx = ctxFrom
		case "JOIN", "INNER", "CROSS", "LEFT", "RIGHT", "FULL":
			ctx = ctxJoin
		case "ON", "USING":
			ctx = ctxOn
		case "WHERE", "HAVING":
			ctx = ctxWhere
		case "AND", "OR":
			if ctx != ctxOn && ctx != ctxSelectList {
				ctx = ctxWhere
			}
		case "GROUP":
			ctx = ctxGroup
		case "ORDER":
			ctx = ctxOrder
		case "BY":
			if prev == "ORDER" {
				ctx = ctxOrder
			} else if prev == "GROUP" {
				ctx = ctxGroup
			}
		case "LIMIT", "OFFSET":
			ctx = ctxLimit
		case "SET":
			ctx = ctxSet
		case "INSERT", "INTO":
			ctx = ctxInsert
		case "UPDATE":
			ctx = ctxFrom
		}
		prev = u
	}
	return ctx, ""
}

// tablesInScopeFast finds relation names after FROM/JOIN in a short tail.
func tablesInScopeFast(before string) []string {
	if before == "" {
		return nil
	}
	if len(before) > 240 {
		before = before[len(before)-240:]
	}
	s := stripSQLNoiseFast(before)
	var out []string
	want := false
	for w := range wordsSeq(s) {
		u := upperASCII(w)
		switch u {
		case "FROM", "JOIN", "INTO", "UPDATE":
			want = true
			continue
		case "LEFT", "RIGHT", "INNER", "OUTER", "FULL", "CROSS", "AS":
			continue
		case "ON", "WHERE", "GROUP", "ORDER", "LIMIT", "HAVING", "SET",
			"RETURNING", "SELECT", "AND", "OR", "USING":
			want = false
			continue
		}
		if want {
			if !isSQLKeywordFast(strings.ToLower(w)) {
				out = append(out, w)
			}
			want = false
		}
	}
	return out
}

// wordsSeq yields identifier tokens without allocating a slice of all words.
func wordsSeq(s string) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		i := 0
		for i < len(s) {
			// skip non-ident
			for i < len(s) && !isIdentStart(s[i]) {
				i++
			}
			if i >= len(s) {
				return
			}
			j := i + 1
			for j < len(s) && isIdentByte(s[j]) {
				j++
			}
			if !yield(s[i:j]) {
				return
			}
			i = j
		}
	}
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isIdentByte(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '.'
}

// stripSQLNoiseFast drops strings/comments; works in-place on a builder, tail-sized.
func stripSQLNoiseFast(s string) string {
	if !strings.ContainsAny(s, "'\"/-") {
		return s // common path: no literals
	}
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '-' && s[i+1] == '-' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			if i+1 < len(s) {
				i += 2
			}
			continue
		}
		if s[i] == '\'' {
			i++
			for i < len(s) {
				if s[i] == '\'' {
					i++
					if i < len(s) && s[i] == '\'' {
						i++
						continue
					}
					break
				}
				i++
			}
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func onlyWhitespace(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' {
			return false
		}
	}
	return true
}

// keyword set for O(1) checks
var sqlKeywordSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(sqlKeywords))
	for _, k := range sqlKeywords {
		m[strings.ToLower(k)] = struct{}{}
	}
	return m
}()

func isSQLKeyword(s string) bool {
	return isSQLKeywordFast(strings.ToLower(s))
}

func isSQLKeywordFast(lower string) bool {
	_, ok := sqlKeywordSet[lower]
	return ok
}

func isAllUpper(s string) bool {
	has := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			return false
		}
		if c >= 'A' && c <= 'Z' {
			has = true
		}
	}
	return has
}

func isSQLIdentPrefix(s string) bool {
	if s == "" {
		return true
	}
	for i, r := range s {
		if i == 0 {
			if unicode.IsLetter(r) || r == '_' || r == '"' {
				continue
			}
			return false
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.' || r == '"' {
			continue
		}
		return false
	}
	return true
}

func upperASCII(s string) string {
	// Hot path: most SQL keywords are already short ASCII.
	for i := 0; i < len(s); i++ {
		if s[i] >= 'a' && s[i] <= 'z' {
			return strings.ToUpper(s)
		}
	}
	return s
}

// sqlTokenAtCursor returns the identifier-like prefix immediately before the cursor.
func sqlTokenAtCursor(value string, line, col int) (token string, startCol int) {
	// Fast path: single-line queries (common)
	if line == 0 && !strings.Contains(value, "\n") {
		if col < 0 {
			col = 0
		}
		if col > len(value) {
			col = len(value)
		}
		start := col
		for start > 0 {
			c := value[start-1]
			if isIdentByte(c) || c == '"' {
				start--
				continue
			}
			break
		}
		return value[start:col], start
	}
	lines := strings.Split(value, "\n")
	if line < 0 || line >= len(lines) {
		return "", col
	}
	row := lines[line]
	if col < 0 {
		col = 0
	}
	if col > len(row) {
		col = len(row)
	}
	start := col
	for start > 0 {
		c := row[start-1]
		if isIdentByte(c) || c == '"' {
			start--
			continue
		}
		break
	}
	return row[start:col], start
}

// textBeforeCursor returns all SQL before the cursor position.
func textBeforeCursor(value string, line, col int) string {
	if line == 0 && !strings.Contains(value, "\n") {
		if col < 0 {
			return ""
		}
		if col > len(value) {
			return value
		}
		return value[:col]
	}
	lines := strings.Split(value, "\n")
	if line < 0 {
		return ""
	}
	if line >= len(lines) {
		return value
	}
	var b strings.Builder
	for i := 0; i < line; i++ {
		b.WriteString(lines[i])
		b.WriteByte('\n')
	}
	row := lines[line]
	if col > len(row) {
		col = len(row)
	}
	if col < 0 {
		col = 0
	}
	b.WriteString(row[:col])
	return b.String()
}

// applySQLSuggestion replaces the token before cursor with suggestion.
func applySQLSuggestion(value string, line, col int, suggestion string) (newValue string, newCol int) {
	lines := strings.Split(value, "\n")
	if line < 0 || line >= len(lines) {
		return value, col
	}
	token, start := sqlTokenAtCursor(value, line, col)
	row := lines[line]
	if col > len(row) {
		col = len(row)
	}
	if i := strings.LastIndexByte(token, '.'); i >= 0 && strings.IndexByte(suggestion, '.') < 0 {
		suggestion = token[:i+1] + suggestion
	}
	lines[line] = row[:start] + suggestion + row[col:]
	newCol = start + len(suggestion)
	return strings.Join(lines, "\n"), newCol
}
