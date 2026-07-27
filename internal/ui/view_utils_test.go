package ui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/davidbudnick/postgres-tui/internal/types"
)

func TestPadRightPlain(t *testing.T) {
	got := padRight("hi", 5)
	if got != "hi   " {
		t.Fatalf("got %q", got)
	}
	if displayWidth(got) != 5 {
		t.Fatalf("width=%d", displayWidth(got))
	}
}

func TestPadRightTruncates(t *testing.T) {
	got := padRight("order_items", 8)
	if displayWidth(got) != 8 {
		t.Fatalf("width=%d got=%q", displayWidth(got), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis: %q", got)
	}
}

func TestPadRightColoredUsesDisplayWidth(t *testing.T) {
	colored := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("tbl")
	got := padRight(colored, 6)
	if displayWidth(got) != 6 {
		t.Fatalf("width=%d raw=%q", displayWidth(got), got)
	}
	if displayWidth(colored) != 3 {
		t.Fatalf("colored width=%d", displayWidth(colored))
	}
}

func TestTruncateNarrow(t *testing.T) {
	cases := []struct {
		s    string
		w    int
		want int
	}{
		{"product_categories", 10, 10},
		{"order_items", 6, 6},
		{"x", 1, 1},
		{"hello", 0, 0},
		{"hello", 5, 5},
	}
	for _, tc := range cases {
		got := truncate(tc.s, tc.w)
		if displayWidth(got) != tc.want {
			t.Fatalf("truncate(%q,%d)=%q width=%d want %d", tc.s, tc.w, got, displayWidth(got), tc.want)
		}
	}
}

func TestObjectListColsAdaptive(t *testing.T) {
	wide := objectListCols(26)
	if !wide.showRows {
		t.Fatalf("expected rows at 26: %+v", wide)
	}
	if wide.nameW+2+objRowsColW+1 > 26 {
		t.Fatalf("wide columns exceed width: nameW=%d", wide.nameW)
	}

	mid := objectListCols(18)
	if !mid.showRows {
		t.Fatalf("expected rows at 18: %+v", mid)
	}

	narrow := objectListCols(12)
	if narrow.showRows {
		t.Fatalf("expected name-only at 12: %+v", narrow)
	}
}

func TestCompactDurationAndActivityDur(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{50 * time.Millisecond, "50ms"},
		{5 * time.Second, "5s"},
		{90 * time.Second, "1m30s"},
		{90 * time.Hour, "3d18h"},
	}
	for _, tc := range cases {
		if got := compactDuration(tc.d); got != tc.want {
			t.Fatalf("compactDuration(%v)=%q want %q", tc.d, got, tc.want)
		}
	}
	if got := formatActivityDur("idle", 90*time.Hour); got != "—" {
		t.Fatalf("idle dur=%q", got)
	}
	if got := formatActivityDur("active", 50*time.Millisecond); got != "50ms" {
		t.Fatalf("active dur=%q", got)
	}
	if dashEmpty("") != "—" || dashEmpty("pg") != "pg" {
		t.Fatal("dashEmpty")
	}
}

func TestObjectListRowFitsInner(t *testing.T) {
	for _, inner := range []int{12, 16, 18, 20, 22, 26, 40} {
		cols := objectListCols(inner)
		plain, colored := cols.row("order_items", "5.0K", "table")
		if displayWidth(plain) != inner {
			t.Fatalf("inner=%d plain width=%d plain=%q", inner, displayWidth(plain), plain)
		}
		if displayWidth(colored) != inner {
			t.Fatalf("inner=%d colored width=%d", inner, displayWidth(colored))
		}
		plain, colored = cols.row("product_categories_extra", "1.0K", "view")
		if displayWidth(plain) != inner {
			t.Fatalf("long name inner=%d plain width=%d %q", inner, displayWidth(plain), plain)
		}
		if displayWidth(colored) != inner {
			t.Fatalf("long name inner=%d colored width=%d", inner, displayWidth(colored))
		}
	}
}

func TestObjectListRowHasGlyphNotKindColumn(t *testing.T) {
	cols := objectListCols(26)
	plain, _ := cols.row("orders", "250", "table")
	if strings.Contains(plain, "tbl") {
		t.Fatalf("KIND column should not appear: %q", plain)
	}
	if !strings.Contains(plain, "orders") {
		t.Fatalf("NAME missing: %q", plain)
	}
	if !strings.Contains(plain, "250") {
		t.Fatalf("ROWS should remain at width 26: %q", plain)
	}
	// glyph is present (table marker)
	if !strings.Contains(plain, kindGlyph("table")) {
		t.Fatalf("expected kind glyph: %q", plain)
	}
}

func TestKindGlyph(t *testing.T) {
	if kindGlyph("table") == "" || kindGlyph("view") == kindGlyph("table") {
		t.Fatal("glyphs should be distinct non-empty")
	}
}

func TestToolGlyphsDistinct(t *testing.T) {
	seen := map[string]NavSection{}
	for _, n := range toolNavItems() {
		g := toolGlyph(n)
		if g == "" || g == "·" {
			t.Fatalf("empty glyph for %v", n)
		}
		if len([]rune(g)) != 1 {
			t.Fatalf("tool glyph must be single cell, got %q for %v", g, n)
		}
		if prev, ok := seen[g]; ok {
			t.Fatalf("duplicate glyph %q for %v and %v", g, prev, n)
		}
		seen[g] = n
	}
}

func TestERDLayersParentsAboveChildren(t *testing.T) {
	g := types.ERDGraph{
		Tables: []types.ERDTable{
			{Name: "users", Columns: []string{"id"}},
			{Name: "orders", Columns: []string{"id", "user_id"}},
			{Name: "order_items", Columns: []string{"id", "order_id"}},
		},
		Edges: []types.FKEdge{
			{FromTable: "orders", FromCols: []string{"user_id"}, ToTable: "users", ToCols: []string{"id"}},
			{FromTable: "order_items", FromCols: []string{"order_id"}, ToTable: "orders", ToCols: []string{"id"}},
		},
	}
	layers := erdLayers(g)
	if len(layers) < 2 {
		t.Fatalf("layers=%v", layers)
	}
	// users should appear in an earlier (higher) layer than order_items
	pos := map[string]int{}
	for i, layer := range layers {
		for _, n := range layer {
			pos[n] = i
		}
	}
	if pos["users"] >= pos["orders"] || pos["orders"] >= pos["order_items"] {
		t.Fatalf("bad layer order: %v pos=%v", layers, pos)
	}
	out := renderERDDiagram(g, 100)
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, "users") || !strings.Contains(joined, "orders") {
		t.Fatalf("diagram missing tables: %q", joined)
	}
	// Routed edges should leave a vertical/arrow between layers (not only text labels).
	if !strings.Contains(joined, "│") && !strings.Contains(joined, "▼") {
		t.Fatalf("expected routed connectors in diagram:\n%s", joined)
	}
}

func TestERDBarycenterKeepsChildNearParent(t *testing.T) {
	g := types.ERDGraph{
		Tables: []types.ERDTable{
			{Name: "users"}, {Name: "product_categories"},
			{Name: "orders"}, {Name: "products"},
		},
		Edges: []types.FKEdge{
			{FromTable: "orders", FromCols: []string{"user_id"}, ToTable: "users"},
			{FromTable: "products", FromCols: []string{"category_id"}, ToTable: "product_categories"},
		},
	}
	layers := erdBarycenterOrder(g, erdLayers(g))
	if len(layers) < 2 {
		t.Fatalf("layers=%v", layers)
	}
	// After barycenter, orders should be under users side, products under categories.
	top, bot := layers[0], layers[1]
	ui, pi := erdIndexOf(top, "users"), erdIndexOf(top, "product_categories")
	oi, pri := erdIndexOf(bot, "orders"), erdIndexOf(bot, "products")
	if ui < 0 || pi < 0 || oi < 0 || pri < 0 {
		t.Fatalf("missing names top=%v bot=%v", top, bot)
	}
	// Same relative order: if users is left of categories, orders left of products.
	if (ui < pi) != (oi < pri) {
		t.Fatalf("barycenter failed: top=%v bot=%v", top, bot)
	}
}

func erdIndexOf(ss []string, s string) int {
	for i, x := range ss {
		if x == s {
			return i
		}
	}
	return -1
}

func TestPgTypeStyleFamilies(t *testing.T) {
	cases := []struct {
		typ  string
		want lipgloss.Style
	}{
		{"integer", typeIntStyle},
		{"bigint", typeIntStyle},
		{"serial", typeIntStyle},
		{"numeric(10,2)", typeFloatStyle},
		{"double precision", typeFloatStyle},
		{"boolean", typeBoolStyle},
		{"timestamp with time zone", typeTimeStyle},
		{"date", typeTimeStyle},
		{"jsonb", typeJSONStyle},
		{"uuid", typeUUIDStyle},
		{"bytea", typeBinaryStyle},
		{"text", typeTextStyle},
		{"character varying", typeTextStyle},
		{"geometry", typeGeoStyle},
	}
	for _, tc := range cases {
		got := pgTypeStyle(tc.typ)
		if got.GetForeground() != tc.want.GetForeground() {
			t.Fatalf("pgTypeStyle(%q) fg=%v want %v", tc.typ, got.GetForeground(), tc.want.GetForeground())
		}
	}
}

func TestCellValueStyleInference(t *testing.T) {
	cases := []struct {
		raw  string
		want lipgloss.Style
	}{
		{"", nullCellStyle},
		{"NULL", nullCellStyle},
		{"true", boolTrueStyle},
		{"false", boolFalseStyle},
		{"42", numCellStyle},
		{"-3.14", numCellStyle},
		{"2024-01-15", timeCellStyle},
		{"2024-01-15T12:00:00Z", timeCellStyle},
		{"a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11", uuidCellStyle},
		{`{"a":1}`, jsonCellStyle},
		{"[1,2]", jsonCellStyle},
		{"hello", strCellStyle},
	}
	for _, tc := range cases {
		got := cellValueStyle(tc.raw)
		if got.GetForeground() != tc.want.GetForeground() {
			t.Fatalf("cellValueStyle(%q) fg=%v want %v", tc.raw, got.GetForeground(), tc.want.GetForeground())
		}
	}
}

func TestConstraintAndActivityStyles(t *testing.T) {
	if constraintBadgeStyle("PRIMARY KEY").GetForeground() != badgePKStyle.GetForeground() {
		t.Fatal("PRIMARY KEY badge")
	}
	if constraintBadgeStyle("FOREIGN KEY").GetForeground() != badgeFKStyle.GetForeground() {
		t.Fatal("FOREIGN KEY badge")
	}
	if constraintBadgeStyle("UNIQUE").GetForeground() != badgeUniqueStyle.GetForeground() {
		t.Fatal("UNIQUE badge")
	}
	if activityStateStyle("active").GetForeground() != stateActiveStyle.GetForeground() {
		t.Fatal("active state")
	}
	if activityStateStyle("idle in transaction").GetForeground() != stateXactStyle.GetForeground() {
		t.Fatal("idle-xact state")
	}
	if durationStyle(40*time.Second).GetForeground() != durHotStyle.GetForeground() {
		t.Fatal("hot duration")
	}
}
