package html

// This file ports no upstream file. It lays out a parsed label as dot of
// Graphviz 16.1.0 does (D38).

import (
	"fmt"

	"github.com/xo/okage"
)

// Rect is a box in points, from its top left, with y down.
type Rect struct {
	X, Y, Width, Height float64
}

// Layout is a label laid out, in points from the top left of the label, with
// y down. Draw the boxes in order, then the rules, then the runs.
type Layout struct {
	// Width and Height are the size of the label.
	Width, Height float64

	// Boxes are the boxes of the tables and the cells, in the order that
	// they draw.
	Boxes []Box

	// Rules are the rules between rows and cells, and the sides of the
	// cells with a sides attribute.
	Rules []Rule

	// Runs are the runs of text.
	Runs []Run

	// Ports maps each port name to the box of its table or cell.
	Ports map[string]Rect
}

// Box is the box of a table or a cell. Fill it, then draw its border inside
// it: the path of the border is the box inset by half of StrokeWidth.
type Box struct {
	Rect

	// Fill is the raw fill, or empty for no fill.
	Fill string

	// Stroke is the raw color of the border, or empty for the default
	// color. StrokeWidth is the width of the border, or 0 for no border.
	Stroke      string
	StrokeWidth float64

	// Style rounds the corners and sets the line of the border.
	Style Style

	// Table is the table of the box, or nil for a cell.
	Table *Table

	// Cell is the cell of the box, or nil for a table.
	Cell *Cell
}

// Rule is a line: a rule between rows or cells, or a side of a cell.
type Rule struct {
	X1, Y1, X2, Y2 float64

	// Stroke is the raw color of the line, or empty for the default color.
	Stroke string

	// Width is the width of the line.
	Width float64

	// Line is the line style.
	Line LineStyle

	// Cell is the cell of a side, or nil for a rule between rows or cells.
	Cell *Cell
}

// Run is a run of text in one font.
type Run struct {
	// X is the left end of the run, and Y its baseline.
	X, Y float64

	// Width is the advance width of the run, as the measure gives it.
	Width float64

	// Top and Height are the top and the height of the line of the run. The
	// middle of the line is Top + Height/2.
	Top, Height float64

	// Text is the text of the run.
	Text string

	// Style is the font of the run, with the default family and size of the
	// options where the label gives none. The size of a run in sub or sup is
	// already smaller.
	Style TextStyle

	// Color is the raw color of the text, or empty for the default color.
	Color string

	Underline bool
	Overline  bool
	Strike    bool
	Sub       bool
	Sup       bool

	// Shift is the distance that sub or sup moves the baseline, up for a
	// positive value. Y already holds it.
	Shift float64

	// Under is the raw fill of the nearest box under the run, or empty.
	Under string
}

// The size and the shifts of a run in sub or sup, relative to the size of the
// font around it.
const (
	subSupScale = 5.0 / 6.0
	supShift    = 1.0 / 3.0
	subShift    = -1.0 / 5.0
)

// Layout lays out the label with the options. It returns an error that wraps
// okage.ErrHTMLMeasure when the measure returns an error.
func (l *Label) Layout(opts ...Option) (*Layout, error) {
	lay := &layouter{
		o:      newOptions(opts),
		out:    &Layout{Ports: map[string]Rect{}},
		grids:  map[*Table]*grid{},
		blocks: map[*Cell]*block{},
	}
	if l.Table != nil {
		w, h := lay.tableSize(l.Table)
		lay.out.Width, lay.out.Height = w, h
		lay.placeTable(l.Table, Rect{Width: w, Height: h}, true, "", "")
	} else {
		b := lay.block(nil, l.Lines)
		lay.out.Width, lay.out.Height = b.w, b.h
		lay.placeText(l.Lines, b, Rect{Width: b.w, Height: b.h}, AlignDefault, AlignDefault, VAlignDefault, "")
	}
	if lay.err != nil {
		return nil, lay.err
	}
	return lay.out, nil
}

// layouter holds the state of Layout.
type layouter struct {
	o      *options
	out    *Layout
	grids  map[*Table]*grid
	blocks map[*Cell]*block
	err    error
}

// lineSize is the size of a line of text, with the metrics of its runs.
type lineSize struct {
	w, h, ascent float64
	spans        []Metrics
	styles       []TextStyle
}

// block is the size of the lines of a label or of a cell.
type block struct {
	w, h  float64
	lines []lineSize
}

// style returns the style of a font, with the defaults of the options.
func (lay *layouter) style(f Font) TextStyle {
	s := lay.o.style
	if f.Face != "" {
		s.Family = f.Face
	}
	if f.Size > 0 {
		s.Size = f.Size
	}
	s.Bold, s.Italic = f.Bold, f.Italic
	if f.Sub || f.Sup {
		s.Size *= subSupScale
	}
	return s
}

// measure measures a run, and keeps the first error.
func (lay *layouter) measure(text string, style TextStyle) Metrics {
	if lay.err != nil {
		return Metrics{}
	}
	m, err := lay.o.measure(text, style)
	if err != nil {
		lay.err = fmt.Errorf("%w: %q: %w", okage.ErrHTMLMeasure, text, err)
		return Metrics{}
	}
	return m
}

// block measures the lines of a cell, or of the label when c is nil. A line
// is as high as its highest run, and its baseline is below its top by the
// largest ascent. An empty line has the metrics of a space.
func (lay *layouter) block(c *Cell, lines []Line) *block {
	if c != nil {
		if b, ok := lay.blocks[c]; ok {
			return b
		}
	}
	b := &block{}
	for _, ln := range lines {
		var ls lineSize
		for _, sp := range ln.Spans {
			st := lay.style(sp.Font)
			m := lay.measure(sp.Text, st)
			ls.spans = append(ls.spans, m)
			ls.styles = append(ls.styles, st)
			ls.w += m.Width
			ls.h = max(ls.h, m.Height)
			ls.ascent = max(ls.ascent, m.Ascent)
		}
		if len(ln.Spans) == 0 {
			m := lay.measure(" ", lay.style(Font{}))
			ls.h, ls.ascent = m.Height, m.Ascent
		}
		b.lines = append(b.lines, ls)
		b.w = max(b.w, ls.w)
		b.h += ls.h
	}
	if c != nil {
		lay.blocks[c] = b
	}
	return b
}

// cellBorder returns the border of a cell in a table.
func cellBorder(c *Cell, t *Table) float64 {
	if c.Border >= 0 {
		return float64(c.Border)
	}
	if t.CellBorder >= 0 {
		return float64(t.CellBorder)
	}
	return float64(t.Border)
}

// cellPadding returns the padding of a cell in a table.
func cellPadding(c *Cell, t *Table) float64 {
	if c.CellPadding >= 0 {
		return float64(c.CellPadding)
	}
	return float64(t.CellPadding)
}

// contentSize returns the size of the content of a cell.
func (lay *layouter) contentSize(c *Cell) (float64, float64) {
	if c.Table != nil {
		return lay.tableSize(c.Table)
	}
	b := lay.block(c, c.Lines)
	return b.w, b.h
}

// cellSize returns the size of a cell: its content, its border and its
// padding, at least its width and height, or its width and height with
// fixedsize.
func (lay *layouter) cellSize(c *Cell, t *Table) (float64, float64) {
	cw, ch := lay.contentSize(c)
	in := 2 * (cellBorder(c, t) + cellPadding(c, t))
	return fitSize(cw+in, c.Width, c.FixedSize), fitSize(ch+in, c.Height, c.FixedSize)
}

// fitSize returns a size of content with a width or a height attribute: at
// least the attribute, or the attribute with fixedsize. An attribute of 0 is
// not given.
func fitSize(size float64, attr int, fixed bool) float64 {
	if attr <= 0 {
		return size
	}
	if fixed {
		return float64(attr)
	}
	return max(size, float64(attr))
}

// grid is the columns and the rows of a table.
type grid struct {
	// at holds the first row and the first column of each cell.
	at map[*Cell][2]int

	// cols and rows are the widths of the columns and the heights of the
	// rows.
	cols, rows []float64

	// w and h are the size of the cells, the spacing and the border. A
	// width and a height of the table can make the table larger.
	w, h float64
}

// grid lays out the cells of a table. Each cell takes the next free column
// of its row. A column is as wide as its widest cell of one column, and a
// cell that spans columns adds the width that it lacks to them in equal
// parts, in the order of the rows and the cells. Rows work the same way.
func (lay *layouter) grid(t *Table) *grid {
	if g, ok := lay.grids[t]; ok {
		return g
	}
	g := &grid{at: map[*Cell][2]int{}}
	used := map[[2]int]bool{}
	ncols, nrows := 0, len(t.Rows)
	for r, row := range t.Rows {
		col := 0
		for _, c := range row.Cells {
			for used[[2]int{r, col}] {
				col++
			}
			g.at[c] = [2]int{r, col}
			for dr := range c.RowSpan {
				for dc := range c.ColSpan {
					used[[2]int{r + dr, col + dc}] = true
				}
			}
			col += c.ColSpan
			ncols = max(ncols, col)
			nrows = max(nrows, r+c.RowSpan)
		}
	}
	g.cols, g.rows = make([]float64, ncols), make([]float64, nrows)
	sp := float64(t.CellSpacing)
	for _, spans := range []bool{false, true} {
		for _, row := range t.Rows {
			for _, c := range row.Cells {
				w, h := lay.cellSize(c, t)
				at := g.at[c]
				fit(g.cols, at[1], c.ColSpan, w, sp, spans)
				fit(g.rows, at[0], c.RowSpan, h, sp, spans)
			}
		}
	}
	b := float64(t.Border)
	g.w = sum(g.cols) + float64(ncols+1)*sp + 2*b
	g.h = sum(g.rows) + float64(nrows+1)*sp + 2*b
	lay.grids[t] = g
	return g
}

// fit makes n sizes from i, with the spacing between them, hold a size: in
// the first pass for n of 1, and in the second pass for n of more than 1.
func fit(sizes []float64, i, n int, size, sp float64, spans bool) {
	if n == 1 {
		if !spans {
			sizes[i] = max(sizes[i], size)
		}
		return
	}
	if !spans {
		return
	}
	n = min(n, len(sizes)-i)
	have := sum(sizes[i:i+n]) + float64(n-1)*sp
	if size <= have {
		return
	}
	for j := i; j < i+n; j++ {
		sizes[j] += (size - have) / float64(n)
	}
}

// sum returns the sum of sizes.
func sum(sizes []float64) float64 {
	var s float64
	for _, v := range sizes {
		s += v
	}
	return s
}

// tableSize returns the size of a table: its grid, at least its width and
// height, or its width and height with fixedsize.
func (lay *layouter) tableSize(t *Table) (float64, float64) {
	g := lay.grid(t)
	return fitSize(g.w, t.Width, t.FixedSize), fitSize(g.h, t.Height, t.FixedSize)
}

// placeTable places a table in a box. A table fills the box, and its columns
// and rows share the extra space in equal parts. A table with fixedsize, or
// the table of the label when exact is set, keeps its size, and its align and
// valign place it. The extra space of a width or a height of the table goes
// to no column or row: the cells stay at the top left.
func (lay *layouter) placeTable(t *Table, box Rect, exact bool, under, stroke string) {
	g := lay.grid(t)
	tw, th := lay.tableSize(t)
	var dx, dy float64
	if exact || t.FixedSize {
		box = placeBox(box, tw, th, t.Align, t.VAlign)
	} else {
		if len(g.cols) > 0 {
			dx = (box.Width - tw) / float64(len(g.cols))
		}
		if len(g.rows) > 0 {
			dy = (box.Height - th) / float64(len(g.rows))
		}
	}
	if t.Color != "" {
		stroke = t.Color
	}
	b := float64(t.Border)
	lay.addBox(Box{Rect: box, Fill: t.BgColor, Stroke: stroke, StrokeWidth: b, Style: t.Style, Table: t})
	if t.BgColor != "" {
		under = t.BgColor
	}
	if t.Port != "" {
		lay.out.Ports[t.Port] = box
	}
	sp := float64(t.CellSpacing)
	colX := make([]float64, len(g.cols)+1)
	colX[0] = box.X + b + sp
	for i, w := range g.cols {
		colX[i+1] = colX[i] + w + dx + sp
	}
	rowY := make([]float64, len(g.rows)+1)
	rowY[0] = box.Y + b + sp
	for i, h := range g.rows {
		rowY[i+1] = rowY[i] + h + dy + sp
	}
	ncols, nrows := len(g.cols), len(g.rows)
	// edge returns the distance from a cell to the end of a rule: to the
	// outer edge of the table at the end of the grid, and to the middle of
	// the spacing elsewhere.
	edge := func(end bool) float64 {
		if end {
			return sp + b
		}
		return sp / 2
	}
	for _, row := range t.Rows {
		for _, c := range row.Cells {
			at := g.at[c]
			r0, c0 := at[0], at[1]
			r1, c1 := min(r0+c.RowSpan, nrows), min(c0+c.ColSpan, ncols)
			cb := Rect{X: colX[c0], Y: rowY[r0], Width: colX[c1] - sp - colX[c0], Height: rowY[r1] - sp - rowY[r0]}
			lay.placeCell(c, t, cb, under, stroke)
			if (t.ColumnRules || c.Rule) && c1 < ncols {
				x := cb.X + cb.Width + sp/2
				lay.addRule(x, cb.Y-edge(r0 == 0), x, cb.Y+cb.Height+edge(r1 == nrows), stroke)
			}
			if (t.RowRules || t.Rows[r1-1].Rule) && r1 < nrows {
				y := cb.Y + cb.Height + sp/2
				lay.addRule(cb.X-edge(c0 == 0), y, cb.X+cb.Width+edge(c1 == ncols), y, stroke)
			}
		}
	}
}

// placeBox places a box of a size in a larger box by an align and a valign,
// in the center by default.
func placeBox(box Rect, w, h float64, a Align, v VAlign) Rect {
	r := Rect{X: box.X + (box.Width-w)/2, Y: box.Y + (box.Height-h)/2, Width: w, Height: h}
	switch a {
	case AlignLeft:
		r.X = box.X
	case AlignRight:
		r.X = box.X + box.Width - w
	}
	switch v {
	case VAlignTop:
		r.Y = box.Y
	case VAlignBottom:
		r.Y = box.Y + box.Height - h
	}
	return r
}

// addRule adds a rule of width 1.
func (lay *layouter) addRule(x1, y1, x2, y2 float64, stroke string) {
	lay.out.Rules = append(lay.out.Rules, Rule{X1: x1, Y1: y1, X2: x2, Y2: y2, Stroke: stroke, Width: 1})
}

// addBox adds a box with a fill or a border.
func (lay *layouter) addBox(b Box) {
	if b.StrokeWidth <= 0 {
		b.StrokeWidth, b.Stroke = 0, ""
	}
	if b.Fill == "" && b.StrokeWidth == 0 {
		return
	}
	lay.out.Boxes = append(lay.out.Boxes, b)
}

// placeCell places a cell of a table in its box, then its content.
func (lay *layouter) placeCell(c *Cell, t *Table, box Rect, under, stroke string) {
	b, pad := cellBorder(c, t), cellPadding(c, t)
	if c.Color != "" {
		stroke = c.Color
	}
	if len(c.Sides) > 0 && len(c.Sides) < 4 {
		lay.addBox(Box{Rect: box, Fill: c.BgColor, Style: c.Style, Cell: c})
		if b > 0 {
			lay.sides(c, box, b, stroke)
		}
	} else {
		lay.addBox(Box{Rect: box, Fill: c.BgColor, Stroke: stroke, StrokeWidth: b, Style: c.Style, Cell: c})
	}
	if c.BgColor != "" {
		under = c.BgColor
	}
	if c.Port != "" {
		lay.out.Ports[c.Port] = box
	}
	in := b + pad
	content := Rect{X: box.X + in, Y: box.Y + in, Width: max(box.Width-2*in, 0), Height: max(box.Height-2*in, 0)}
	if c.Table != nil {
		lay.placeTable(c.Table, content, false, under, stroke)
		return
	}
	lay.placeText(c.Lines, lay.block(c, c.Lines), content, c.Align, c.BAlign, c.VAlign, under)
}

// sides adds a rule for each side of a cell with a sides attribute, inside
// the box of the cell, as a border is.
func (lay *layouter) sides(c *Cell, box Rect, b float64, stroke string) {
	x1, y1 := box.X+b/2, box.Y+b/2
	x2, y2 := box.X+box.Width-b/2, box.Y+box.Height-b/2
	for _, s := range c.Sides {
		r := Rule{Stroke: stroke, Width: b, Line: c.Style.Line, Cell: c}
		switch s {
		case 'L':
			r.X1, r.Y1, r.X2, r.Y2 = x1, y1, x1, y2
		case 'T':
			r.X1, r.Y1, r.X2, r.Y2 = x1, y1, x2, y1
		case 'R':
			r.X1, r.Y1, r.X2, r.Y2 = x2, y1, x2, y2
		case 'B':
			r.X1, r.Y1, r.X2, r.Y2 = x1, y2, x2, y2
		}
		lay.out.Rules = append(lay.out.Rules, r)
	}
}

// placeText places lines of text in a box. The lines go inside the box by
// align and valign, as one block that is as wide as the widest line, or as
// the box with AlignText. Each line goes inside the block by its own align,
// then by balign, then in the center.
func (lay *layouter) placeText(lines []Line, b *block, box Rect, align, balign Align, valign VAlign, under string) {
	top := box.Y + (box.Height-b.h)/2
	switch valign {
	case VAlignTop:
		top = box.Y
	case VAlignBottom:
		top = box.Y + box.Height - b.h
	}
	bx, bw := box.X+(box.Width-b.w)/2, b.w
	switch align {
	case AlignLeft:
		bx = box.X
	case AlignRight:
		bx = box.X + box.Width - b.w
	case AlignText:
		bx, bw = box.X, box.Width
	}
	for i, ln := range lines {
		ls := b.lines[i]
		a := ln.Align
		if a == AlignDefault {
			a = balign
		}
		x := bx + (bw-ls.w)/2
		switch a {
		case AlignLeft:
			x = bx
		case AlignRight:
			x = bx + bw - ls.w
		}
		baseline := top + ls.ascent
		for j, sp := range ln.Spans {
			m, st := ls.spans[j], ls.styles[j]
			var shift float64
			switch {
			case sp.Font.Sup:
				shift = supShift * st.Size / subSupScale
			case sp.Font.Sub:
				shift = subShift * st.Size / subSupScale
			}
			lay.out.Runs = append(lay.out.Runs, Run{
				X: x, Y: baseline - shift, Width: m.Width,
				Top: top, Height: ls.h,
				Text: sp.Text, Style: st, Color: sp.Font.Color,
				Underline: sp.Font.Underline, Overline: sp.Font.Overline, Strike: sp.Font.Strike,
				Sub: sp.Font.Sub, Sup: sp.Font.Sup, Shift: shift,
				Under: under,
			})
			x += m.Width
		}
		top += ls.h
	}
}
