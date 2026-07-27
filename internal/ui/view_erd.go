package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

const (
	erdBoxMinW     = 16
	erdBoxMaxW     = 24
	erdMaxCols     = 4 // PK + FKs only in boxes
	erdListMinW    = 36
	erdMaxTables   = 40
	erdHGap        = 3
	erdVGap        = 4 // rows reserved for connectors between layers
	erdMaxPerRow   = 4 // wrap wide layers so boxes stay readable
	erdIsolateChip = 18
)

func (m Model) viewERDContent(width, height int) string {
	var b strings.Builder
	schema := m.ERD.Schema
	if schema == "" {
		schema = m.CurrentSchema
	}
	if schema == "" {
		schema = "public"
	}

	b.WriteString(headerStyle.Render("ERD"))
	b.WriteString("  ")
	b.WriteString(accentStyle.Render(schema))
	b.WriteString(dimStyle.Render(fmt.Sprintf("  ·  %d tables  ·  %d FKs",
		len(m.ERD.Tables), len(m.ERD.Edges))))
	b.WriteByte('\n')
	b.WriteString(gridSepStyle.Render(strings.Repeat("─", min(width, 100))))
	b.WriteByte('\n')

	if m.Loading && len(m.ERD.Tables) == 0 {
		b.WriteString(dimStyle.Render("Loading schema graph…"))
		b.WriteByte('\n')
		return b.String()
	}
	if len(m.ERD.Tables) == 0 {
		b.WriteString(dimStyle.Render("No tables in this schema."))
		b.WriteByte('\n')
		return b.String()
	}

	bodyH := max(height-3, 4)
	var lines []string
	if width < erdListMinW || len(m.ERD.Tables) > erdMaxTables {
		lines = renderERDList(m.ERD, width, true)
	} else {
		lines = renderERDDiagram(m.ERD, width)
	}

	maxOff := max(len(lines)-bodyH, 0)
	offset := clamp(m.ERDOffset, 0, maxOff)
	end := min(offset+bodyH, len(lines))
	for i := offset; i < end; i++ {
		b.WriteString(lines[i])
		b.WriteByte('\n')
	}
	if len(lines) > bodyH {
		b.WriteString(dimStyle.Render(fmt.Sprintf("↕ %d–%d of %d  ·  j/k scroll  ·  r refresh",
			offset+1, end, len(lines))))
		b.WriteByte('\n')
	}
	return b.String()
}

// renderERDList renders the FK edge list. withHeader controls the section title.
func renderERDList(g types.ERDGraph, width int, withHeader bool) []string {
	if len(g.Edges) == 0 {
		return []string{dimStyle.Render("  No foreign keys in this schema.")}
	}
	edges := append([]types.FKEdge(nil), g.Edges...)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].FromTable != edges[j].FromTable {
			return edges[i].FromTable < edges[j].FromTable
		}
		return edges[i].ToTable < edges[j].ToTable
	})
	leftW := min(max(width*42/100, 18), 44)
	rightW := min(max(width-leftW-10, 14), 44)
	var lines []string
	if withHeader {
		lines = append(lines, dimStyle.Render("  Relationships"))
	}
	for _, e := range edges {
		from := e.FromTable + "." + strings.Join(e.FromCols, ",")
		to := e.ToTable + "." + strings.Join(e.ToCols, ",")
		lines = append(lines,
			"  "+normalStyle.Render(padRight(truncate(from, leftW), leftW))+
				accentStyle.Render(" ──► ")+
				dimStyle.Render(truncate(to, rightW)))
	}
	return lines
}

// ── canvas layout with routed edges ──────────────────────────────────

type erdNode struct {
	name  string
	x, y  int // top-left
	w, h  int
	table types.ERDTable
	layer int
	slot  int // index within layer
}

type erdCanvas struct {
	w, h  int
	cells [][]rune
	attr  [][]byte // 0=plain 1=dim 2=accent 3=title
}

func newERDCanvas(w, h int) *erdCanvas {
	if w < 8 {
		w = 8
	}
	if h < 4 {
		h = 4
	}
	c := &erdCanvas{w: w, h: h, cells: make([][]rune, h), attr: make([][]byte, h)}
	for y := 0; y < h; y++ {
		c.cells[y] = make([]rune, w)
		c.attr[y] = make([]byte, w)
		for x := 0; x < w; x++ {
			c.cells[y][x] = ' '
		}
	}
	return c
}

func (c *erdCanvas) in(x, y int) bool {
	return x >= 0 && y >= 0 && x < c.w && y < c.h
}

func (c *erdCanvas) put(x, y int, r rune, a byte) {
	if !c.in(x, y) {
		return
	}
	cur := c.cells[y][x]
	if isBoxRune(cur) && isWireRune(r) {
		c.cells[y][x] = mergeWireBox(cur, r)
		return
	}
	if isWireRune(cur) && isWireRune(r) {
		c.cells[y][x] = mergeWires(cur, r)
		if a > c.attr[y][x] {
			c.attr[y][x] = a
		}
		return
	}
	c.cells[y][x] = r
	c.attr[y][x] = a
}

func isBoxRune(r rune) bool {
	switch r {
	case '┌', '┐', '└', '┘', '├', '┤', '┬', '┴', '┼', '─', '│':
		return true
	}
	return false
}

func isWireRune(r rune) bool {
	switch r {
	case '│', '─', '┌', '┐', '└', '┘', '├', '┤', '┬', '┴', '┼', '▼', '▲', '►', '◄', '↓', '↑':
		return true
	}
	return false
}

func mergeWires(a, b rune) rune {
	if a == b {
		return a
	}
	h := a == '─' || b == '─' || a == '┬' || b == '┬' || a == '┴' || b == '┴'
	v := a == '│' || b == '│' || a == '├' || b == '├' || a == '┤' || b == '┤'
	if h && v {
		return '┼'
	}
	if a == '│' || b == '│' {
		return '│'
	}
	if a == '─' || b == '─' {
		return '─'
	}
	return b
}

func mergeWireBox(box, wire rune) rune {
	if wire == '│' || wire == '─' {
		return box
	}
	return box
}

func (c *erdCanvas) hline(x1, x2, y int, a byte) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	for x := x1; x <= x2; x++ {
		c.put(x, y, '─', a)
	}
}

func (c *erdCanvas) vline(x, y1, y2 int, a byte) {
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	for y := y1; y <= y2; y++ {
		c.put(x, y, '│', a)
	}
}

func (c *erdCanvas) writeStr(x, y int, s string, a byte) {
	for i, r := range s {
		c.put(x+i, y, r, a)
	}
}

func (c *erdCanvas) drawBox(n erdNode, fkCols map[string]bool) {
	inner := n.w - 2
	c.put(n.x, n.y, '┌', 1)
	c.put(n.x+n.w-1, n.y, '┐', 1)
	c.put(n.x, n.y+n.h-1, '└', 1)
	c.put(n.x+n.w-1, n.y+n.h-1, '┘', 1)
	c.hline(n.x+1, n.x+n.w-2, n.y, 1)
	c.hline(n.x+1, n.x+n.w-2, n.y+n.h-1, 1)
	for y := n.y + 1; y < n.y+n.h-1; y++ {
		c.put(n.x, y, '│', 1)
		c.put(n.x+n.w-1, y, '│', 1)
	}
	title := truncate(n.name, inner)
	c.writeStr(n.x+1, n.y+1, padRight(title, inner), 3)
	c.put(n.x, n.y+2, '├', 1)
	c.put(n.x+n.w-1, n.y+2, '┤', 1)
	c.hline(n.x+1, n.x+n.w-2, n.y+2, 1)

	cols := orderERDColumns(n.name, n.table.Columns, fkCols, true)
	if len(cols) > erdMaxCols {
		cols = append(append([]string{}, cols[:erdMaxCols-1]...), "…")
	}
	for i, col := range cols {
		y := n.y + 3 + i
		if y >= n.y+n.h-1 {
			break
		}
		a := byte(1)
		if fkCols[n.name+"."+col] {
			a = 2
		} else if col == "id" {
			a = 0
		}
		c.writeStr(n.x+1, y, padRight(truncate(col, inner), inner), a)
	}
}

// routeEdge draws parent bottom → child top with a label near the child stem.
func (c *erdCanvas) routeEdge(parent, child erdNode, label string, busOffset int) {
	px := parent.x + parent.w/2
	py := parent.y + parent.h
	cx := child.x + child.w/2
	cy := child.y

	if cy <= py+1 {
		return
	}

	midY := (py + cy) / 2
	// Stagger buses when many edges share the same inter-layer gap.
	if busOffset != 0 {
		midY += busOffset
	}
	if midY <= py {
		midY = py + 1
	}
	if midY >= cy-1 {
		midY = cy - 2
	}
	if midY <= py {
		return
	}

	for y := py; y < midY; y++ {
		c.put(px, y, '│', 2)
	}

	if px == cx {
		for y := midY; y < cy-1; y++ {
			c.put(cx, y, '│', 2)
		}
		c.put(cx, cy-1, '▼', 2)
	} else {
		if cx > px {
			c.put(px, midY, '└', 2)
		} else {
			c.put(px, midY, '┘', 2)
		}
		lo, hi := px, cx
		if lo > hi {
			lo, hi = hi, lo
		}
		for x := lo + 1; x < hi; x++ {
			c.put(x, midY, '─', 2)
		}
		if cx > px {
			c.put(cx, midY, '┐', 2)
		} else {
			c.put(cx, midY, '┌', 2)
		}
		for y := midY + 1; y < cy-1; y++ {
			c.put(cx, y, '│', 2)
		}
		c.put(cx, cy-1, '▼', 2)
	}

	if label == "" {
		return
	}
	lab := truncate(label, 14)
	// Pin label next to the child stem so multiple edges don't pile up mid-bus.
	labX := cx - len(lab)/2
	if labX < 0 {
		labX = 0
	}
	if labX+len(lab) >= c.w {
		labX = max(c.w-len(lab)-1, 0)
	}
	// Prefer just below the bus; fall back onto the bus if the gap is tight.
	labY := midY + 1
	if labY >= cy-1 {
		labY = midY
	}
	if labY >= 0 && labY < c.h {
		c.writeStr(labX, labY, lab, 2)
	}
}

func (c *erdCanvas) lines() []string {
	out := make([]string, 0, c.h)
	last := c.h - 1
	for last > 0 {
		empty := true
		for x := 0; x < c.w; x++ {
			if c.cells[last][x] != ' ' {
				empty = false
				break
			}
		}
		if !empty {
			break
		}
		last--
	}
	for y := 0; y <= last; y++ {
		end := c.w - 1
		for end > 0 && c.cells[y][end] == ' ' {
			end--
		}
		var b strings.Builder
		for x := 0; x <= end; x++ {
			r := c.cells[y][x]
			switch c.attr[y][x] {
			case 2:
				b.WriteString(accentStyle.Render(string(r)))
			case 3:
				b.WriteString(accentStyle.Bold(true).Render(string(r)))
			case 1:
				b.WriteString(dimStyle.Render(string(r)))
			default:
				b.WriteRune(r)
			}
		}
		out = append(out, b.String())
	}
	return out
}

func renderERDDiagram(g types.ERDGraph, width int) []string {
	if len(g.Tables) == 0 {
		return nil
	}

	linked, isolates := erdPartition(g)
	fkCols := erdFKColumnSet(linked)

	var lines []string
	if len(linked.Tables) > 0 {
		layers := erdWrapLayers(erdBarycenterOrder(linked, erdLayers(linked)), erdMaxPerRow)
		lines = append(lines, layoutERDCanvas(linked, layers, fkCols, width)...)
	}

	if len(g.Edges) > 0 {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("Relationships"))
		lines = append(lines, renderERDList(g, width, false)...)
	}

	if len(isolates) > 0 {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render(fmt.Sprintf("Unconnected (%d)", len(isolates))))
		lines = append(lines, renderERDIsolates(isolates, width)...)
	}

	return lines
}

// layoutERDCanvas places layered boxes and routes FK edges onto a canvas.
func layoutERDCanvas(g types.ERDGraph, layers [][]string, fkCols map[string]bool, width int) []string {
	nAcross := maxLayerWidth(layers)
	boxW := clamp((width-(nAcross-1)*erdHGap)/max(nAcross, 1), erdBoxMinW, erdBoxMaxW)

	nodes := map[string]*erdNode{}
	// Count outgoing edges per parent for bus staggering.
	outDegree := map[string]int{}
	edgeIdx := map[string]int{} // parent->running index for busOffset
	for _, e := range g.Edges {
		if e.FromTable != e.ToTable {
			outDegree[e.ToTable]++ // edges leave the parent (ToTable is parent)
		}
	}

	y := 0
	for li, layer := range layers {
		type sized struct {
			name string
			h    int
			t    types.ERDTable
		}
		var row []sized
		maxH := 0
		for _, name := range layer {
			t := erdFindTable(g, name)
			cols := orderERDColumns(name, t.Columns, fkCols, true)
			nCol := len(cols)
			if nCol > erdMaxCols {
				nCol = erdMaxCols
			}
			if nCol == 0 {
				nCol = 1
			}
			h := 4 + nCol
			if h > maxH {
				maxH = h
			}
			row = append(row, sized{name, h, t})
		}
		totalW := len(row)*boxW + (len(row)-1)*erdHGap
		x0 := max((width-totalW)/2, 0)
		for i, s := range row {
			x := x0 + i*(boxW+erdHGap)
			nodes[s.name] = &erdNode{
				name:  s.name,
				x:     x,
				y:     y,
				w:     boxW,
				h:     maxH,
				table: s.t,
				layer: li,
				slot:  i,
			}
		}
		y += maxH + erdVGap
	}

	maxX := width
	for _, n := range nodes {
		if n.x+n.w > maxX {
			maxX = n.x + n.w
		}
	}
	cv := newERDCanvas(min(max(maxX, width), width+4), y+1)

	for _, e := range g.Edges {
		parent, okP := nodes[e.ToTable]
		child, okC := nodes[e.FromTable]
		if !okP || !okC {
			continue
		}
		if child.layer <= parent.layer {
			continue
		}
		idx := edgeIdx[e.ToTable]
		edgeIdx[e.ToTable] = idx + 1
		// Stagger mid-bus when a parent has several children.
		bus := 0
		if outDegree[e.ToTable] > 1 {
			bus = (idx % 3) - 1 // -1, 0, 1
		}
		label := strings.Join(e.FromCols, ",")
		cv.routeEdge(*parent, *child, label, bus)
	}

	for _, n := range nodes {
		cv.drawBox(*n, fkCols)
	}
	return cv.lines()
}

// renderERDIsolates shows tables with no FKs as a compact wrapped chip list.
func renderERDIsolates(tables []types.ERDTable, width int) []string {
	if len(tables) == 0 {
		return nil
	}
	names := make([]string, len(tables))
	for i, t := range tables {
		names[i] = t.Name
	}
	sort.Strings(names)

	var lines []string
	var row strings.Builder
	rowW := 0
	maxW := max(width-2, 20)
	for _, name := range names {
		chip := " " + truncate(name, erdIsolateChip-2) + " "
		need := len(chip) + 1
		if rowW > 0 && rowW+need > maxW {
			lines = append(lines, "  "+dimStyle.Render(row.String()))
			row.Reset()
			rowW = 0
		}
		if rowW > 0 {
			row.WriteByte(' ')
			rowW++
		}
		row.WriteString("[" + strings.TrimSpace(chip) + "]")
		rowW += need
	}
	if row.Len() > 0 {
		lines = append(lines, "  "+dimStyle.Render(row.String()))
	}
	return lines
}

// erdPartition splits tables into those on an FK path and pure isolates.
func erdPartition(g types.ERDGraph) (linked types.ERDGraph, isolates []types.ERDTable) {
	inSchema := map[string]bool{}
	for _, t := range g.Tables {
		inSchema[t.Name] = true
	}
	linkedSet := map[string]bool{}
	var edges []types.FKEdge
	for _, e := range g.Edges {
		if !inSchema[e.FromTable] || !inSchema[e.ToTable] {
			continue
		}
		if e.FromTable == e.ToTable {
			continue
		}
		linkedSet[e.FromTable] = true
		linkedSet[e.ToTable] = true
		edges = append(edges, e)
	}
	var linkedTables []types.ERDTable
	for _, t := range g.Tables {
		if linkedSet[t.Name] {
			linkedTables = append(linkedTables, t)
		} else {
			isolates = append(isolates, t)
		}
	}
	sort.Slice(isolates, func(i, j int) bool { return isolates[i].Name < isolates[j].Name })
	return types.ERDGraph{Schema: g.Schema, Tables: linkedTables, Edges: edges}, isolates
}

// erdWrapLayers splits layers wider than maxPerRow into multiple rows.
func erdWrapLayers(layers [][]string, maxPerRow int) [][]string {
	if maxPerRow < 1 {
		maxPerRow = erdMaxPerRow
	}
	var out [][]string
	for _, layer := range layers {
		if len(layer) <= maxPerRow {
			out = append(out, layer)
			continue
		}
		for i := 0; i < len(layer); i += maxPerRow {
			end := min(i+maxPerRow, len(layer))
			out = append(out, layer[i:end])
		}
	}
	return out
}

func erdFKColumnSet(g types.ERDGraph) map[string]bool {
	out := map[string]bool{}
	for _, e := range g.Edges {
		for _, c := range e.FromCols {
			out[e.FromTable+"."+c] = true
		}
		for _, c := range e.ToCols {
			out[e.ToTable+"."+c] = true
		}
	}
	return out
}

func erdFindTable(g types.ERDGraph, name string) types.ERDTable {
	for _, t := range g.Tables {
		if t.Name == name {
			return t
		}
	}
	return types.ERDTable{Name: name}
}

// orderERDColumns ranks id and FK columns first. When compact, only those are kept.
func orderERDColumns(table string, cols []string, fkCols map[string]bool, compact bool) []string {
	if len(cols) == 0 {
		return nil
	}
	var id, fks, rest []string
	for _, c := range cols {
		switch {
		case c == "id":
			id = append(id, c)
		case fkCols[table+"."+c] || strings.HasSuffix(c, "_id"):
			fks = append(fks, c)
		default:
			rest = append(rest, c)
		}
	}
	out := append(id, fks...)
	if compact {
		if len(out) == 0 && len(rest) > 0 {
			// Keep a single sample column so empty boxes aren't blank.
			return rest[:1]
		}
		return out
	}
	return append(out, rest...)
}

func maxLayerWidth(layers [][]string) int {
	m := 1
	for _, l := range layers {
		if len(l) > m {
			m = len(l)
		}
	}
	return m
}

// erdLayers ranks parent tables above children (longest path depth).
func erdLayers(g types.ERDGraph) [][]string {
	names := make([]string, 0, len(g.Tables))
	inSchema := map[string]bool{}
	for _, t := range g.Tables {
		names = append(names, t.Name)
		inSchema[t.Name] = true
	}
	sort.Strings(names)

	parentsOf := map[string][]string{}
	for _, e := range g.Edges {
		if !inSchema[e.FromTable] || !inSchema[e.ToTable] || e.FromTable == e.ToTable {
			continue
		}
		parentsOf[e.FromTable] = append(parentsOf[e.FromTable], e.ToTable)
	}

	depth := map[string]int{}
	var visit func(n string, stack map[string]bool) int
	visit = func(n string, stack map[string]bool) int {
		if d, ok := depth[n]; ok {
			return d
		}
		if stack[n] {
			return 0
		}
		stack[n] = true
		maxParent := -1
		for _, parent := range parentsOf[n] {
			pd := visit(parent, stack)
			if pd > maxParent {
				maxParent = pd
			}
		}
		d := maxParent + 1
		depth[n] = d
		delete(stack, n)
		return d
	}
	for _, n := range names {
		visit(n, map[string]bool{})
	}

	maxD := 0
	for _, d := range depth {
		if d > maxD {
			maxD = d
		}
	}
	layers := make([][]string, maxD+1)
	for _, n := range names {
		d := depth[n]
		layers[d] = append(layers[d], n)
	}
	var out [][]string
	for _, l := range layers {
		if len(l) == 0 {
			continue
		}
		sort.Strings(l)
		out = append(out, l)
	}
	if len(out) == 0 {
		return [][]string{names}
	}
	return out
}

// erdBarycenterOrder sorts each layer so children sit under their parents.
func erdBarycenterOrder(g types.ERDGraph, layers [][]string) [][]string {
	if len(layers) < 2 {
		return layers
	}
	pos := map[string]float64{}
	for i, name := range layers[0] {
		pos[name] = float64(i)
	}

	parentsOf := map[string][]string{}
	for _, e := range g.Edges {
		if e.FromTable == e.ToTable {
			continue
		}
		parentsOf[e.FromTable] = append(parentsOf[e.FromTable], e.ToTable)
	}

	out := make([][]string, len(layers))
	out[0] = append([]string{}, layers[0]...)
	for li := 1; li < len(layers); li++ {
		type scored struct {
			name  string
			score float64
		}
		var row []scored
		for _, name := range layers[li] {
			ps := parentsOf[name]
			if len(ps) == 0 {
				row = append(row, scored{name, float64(len(row))})
				continue
			}
			sum := 0.0
			n := 0
			for _, p := range ps {
				if v, ok := pos[p]; ok {
					sum += v
					n++
				}
			}
			sc := float64(len(row))
			if n > 0 {
				sc = sum / float64(n)
			}
			row = append(row, scored{name, sc})
		}
		sort.SliceStable(row, func(i, j int) bool {
			if row[i].score != row[j].score {
				return row[i].score < row[j].score
			}
			return row[i].name < row[j].name
		})
		names := make([]string, len(row))
		for i, s := range row {
			names[i] = s.name
			pos[s.name] = float64(i)
		}
		out[li] = names
	}
	return out
}
