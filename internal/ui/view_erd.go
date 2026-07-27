package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/davidbudnick/postgres-tui/internal/types"
)

const (
	erdBoxMinW   = 18
	erdBoxMaxW   = 26
	erdMaxCols   = 5
	erdListMinW  = 36
	erdMaxTables = 40
	erdHGap      = 4
	erdVGap      = 3 // rows reserved for connectors between layers
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
		lines = renderERDList(m.ERD, width)
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

func renderERDList(g types.ERDGraph, width int) []string {
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
	lines = append(lines, dimStyle.Render("  Relationships"))
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
	// Don't overwrite box corners/walls with wire unless empty/wire
	cur := c.cells[y][x]
	if isBoxRune(cur) && isWireRune(r) {
		// merge junctions
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
	// prefer junction when crossing
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
	// keep box geometry; only upgrade T-junctions lightly
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
	// borders
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
	// title
	title := truncate(n.name, inner)
	c.writeStr(n.x+1, n.y+1, padRight(title, inner), 3)
	// separator
	c.put(n.x, n.y+2, '├', 1)
	c.put(n.x+n.w-1, n.y+2, '┤', 1)
	c.hline(n.x+1, n.x+n.w-2, n.y+2, 1)

	cols := orderERDColumns(n.name, n.table.Columns, fkCols)
	if len(cols) > erdMaxCols {
		cols = append(append([]string{}, cols[:erdMaxCols-1]...), "…")
	}
	for i, col := range cols {
		y := n.y + 3 + i
		if y >= n.y+n.h-1 {
			break
		}
		a := byte(1) // dim
		if fkCols[n.name+"."+col] {
			a = 2 // accent FK
		} else if col == "id" {
			a = 0
		}
		c.writeStr(n.x+1, y, padRight(truncate(col, inner), inner), a)
	}
}

func (c *erdCanvas) routeEdge(parent, child erdNode, label string) {
	// Parent bottom-center → child top-center (Manhattan down-across-down).
	//
	//   parent
	//     │
	//     └── label ──┐
	//                 │
	//                 ▼
	//               child
	px := parent.x + parent.w/2
	py := parent.y + parent.h // first free row under parent
	cx := child.x + child.w/2
	cy := child.y // top border of child

	if cy <= py+1 {
		return
	}

	midY := (py + cy) / 2
	if midY <= py {
		midY = py
	}
	if midY >= cy-1 {
		midY = cy - 2
	}
	if midY < py {
		midY = py
	}

	// stem under parent
	for y := py; y < midY; y++ {
		c.put(px, y, '│', 2)
	}
	c.put(px, midY, '│', 2)

	if px == cx {
		// straight down
		for y := midY; y < cy-1; y++ {
			c.put(cx, y, '│', 2)
		}
		c.put(cx, cy-1, '▼', 2)
	} else {
		// corner at parent column
		if cx > px {
			c.put(px, midY, '└', 2)
		} else {
			c.put(px, midY, '┘', 2)
		}
		// horizontal
		lo, hi := px, cx
		if lo > hi {
			lo, hi = hi, lo
		}
		for x := lo + 1; x < hi; x++ {
			c.put(x, midY, '─', 2)
		}
		// corner at child column
		if cx > px {
			c.put(cx, midY, '┐', 2)
		} else {
			c.put(cx, midY, '┌', 2)
		}
		// drop to child
		for y := midY + 1; y < cy-1; y++ {
			c.put(cx, y, '│', 2)
		}
		c.put(cx, cy-1, '▼', 2)
	}

	if label == "" {
		return
	}
	lab := truncate(label, 12)
	labX := (px + cx) / 2
	if labX < 0 {
		labX = 0
	}
	if labX+len(lab) >= c.w {
		labX = max(c.w-len(lab)-1, 0)
	}
	labY := midY - 1
	if labY <= parent.y+parent.h-1 {
		labY = midY
		// sit to the side of the horizontal bar
		if cx > px {
			labX = min(px+1, c.w-len(lab)-1)
		} else {
			labX = max(cx+1, 0)
		}
	}
	if labY >= 0 && labY < c.h {
		c.writeStr(labX, labY, lab, 2)
	}
}

func (c *erdCanvas) lines() []string {
	out := make([]string, 0, c.h)
	// trim trailing empty rows
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
		// trim trailing spaces on line
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
	fkCols := erdFKColumnSet(g)
	layers := erdLayers(g)
	layers = erdBarycenterOrder(g, layers)

	// Box width from densest layer
	nAcross := maxLayerWidth(layers)
	boxW := clamp((width-(nAcross-1)*erdHGap)/max(nAcross, 1), erdBoxMinW, erdBoxMaxW)

	// Build nodes with positions
	nodes := map[string]*erdNode{}
	y := 0
	for li, layer := range layers {
		// box heights may vary — compute each
		type sized struct {
			name string
			h    int
			t    types.ERDTable
		}
		var row []sized
		maxH := 0
		for _, name := range layer {
			t := erdFindTable(g, name)
			cols := orderERDColumns(name, t.Columns, fkCols)
			nCol := len(cols)
			if nCol > erdMaxCols {
				nCol = erdMaxCols
			}
			if nCol == 0 {
				nCol = 1
			}
			// top+title+sep+cols+bot = 3+nCol+1
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
				h:     maxH, // uniform row height for clean edges
				table: s.t,
				layer: li,
				slot:  i,
			}
		}
		y += maxH + erdVGap
	}

	// Canvas size
	maxX := width
	for _, n := range nodes {
		if n.x+n.w > maxX {
			maxX = n.x + n.w
		}
	}
	cv := newERDCanvas(min(max(maxX, width), width+4), y+1)

	// Draw edges first (under boxes), then boxes on top
	for _, e := range g.Edges {
		parent, okP := nodes[e.ToTable]
		child, okC := nodes[e.FromTable]
		if !okP || !okC {
			continue
		}
		// only route consecutive-ish layers to avoid spaghetti
		if child.layer <= parent.layer {
			continue
		}
		label := strings.Join(e.FromCols, ",")
		cv.routeEdge(*parent, *child, label)
	}

	for _, n := range nodes {
		cv.drawBox(*n, fkCols)
	}

	lines := cv.lines()
	if len(g.Edges) > 0 {
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("Relationships"))
		lines = append(lines, renderERDList(g, width)...)
	} else {
		lines = append(lines, "", dimStyle.Render("  (no foreign keys — isolated tables)"))
	}
	return lines
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

func orderERDColumns(table string, cols []string, fkCols map[string]bool) []string {
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
		if d < 0 {
			d = 0
		}
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
	// index of each node in its layer after previous sort
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
