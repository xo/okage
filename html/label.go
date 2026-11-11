package html

// This file ports no upstream file. It holds the tree that Parse returns
// (D38).

// Label is a parsed HTML-like label. It holds lines of text or one table.
type Label struct {
	// Lines are the lines of a label of text. They are empty when Table is
	// set.
	Lines []Line

	// Table is the table of the label, or nil for a label of text.
	Table *Table
}

// Line is a line of text, with the runs of text in it.
type Line struct {
	// Spans are the runs of text of the line, each in one font. An empty
	// line has no span.
	Spans []Span

	// Align is the align of the <br/> that ends the line, or AlignDefault.
	Align Align
}

// Span is a run of text in one font.
type Span struct {
	// Text is the text of the run, with its spaces and its entities decoded.
	Text string

	// Font is the font of the run, from the tags around it.
	Font Font
}

// Font is the font that the tags b, i, u, o, s, sub, sup and font set.
type Font struct {
	// Face is the face of the innermost font tag with a face, or empty for
	// the default family.
	Face string

	// Color is the raw color of the innermost font tag with a color, or
	// empty for the default color.
	Color string

	// Size is the point-size of the innermost font tag with a point-size, or
	// 0 for the default size.
	Size float64

	Bold      bool
	Italic    bool
	Underline bool
	Overline  bool
	Strike    bool
	Sub       bool
	Sup       bool
}

// Align is a horizontal place: an align, a balign, or the align of a <br/>.
type Align uint8

// The values of Align.
const (
	// AlignDefault is an align that is not given.
	AlignDefault Align = iota
	AlignLeft
	AlignRight
	AlignCenter
	// AlignText is align="text" of a cell: the lines go inside the whole
	// width of the cell.
	AlignText
)

// VAlign is a vertical place: the valign of a table or a cell.
type VAlign uint8

// The values of VAlign.
const (
	// VAlignDefault is a valign that is not given, which is the middle.
	VAlignDefault VAlign = iota
	VAlignTop
	VAlignBottom
	VAlignMiddle
)

// LineStyle is the line of a border or a rule.
type LineStyle uint8

// The values of LineStyle.
const (
	LineSolid LineStyle = iota
	LineDashed
	LineDotted
)

// Style is the style attribute of a table or a cell.
type Style struct {
	// Rounded rounds the corners of the border.
	Rounded bool

	// Line is the line of the border.
	Line LineStyle
}

// Table is a <table>.
type Table struct {
	// Rows are the rows of the table, in order.
	Rows []*Row

	// Font is the font around the table. Its cells take it.
	Font Font

	// Border is the width of the border of the table. Its default is 1.
	Border int

	// CellBorder is the width of the border of each cell, or -1 for the
	// width of Border.
	CellBorder int

	// CellSpacing is the space between the cells, and between the cells and
	// the border. Its default is 2.
	CellSpacing int

	// CellPadding is the space inside each cell, around its content. Its
	// default is 2.
	CellPadding int

	// Width and Height are the least size of the table, or 0. With
	// FixedSize, they are its size.
	Width     int
	Height    int
	FixedSize bool

	// BgColor is the raw fill, and Color the raw color of the border. An
	// empty Color takes the color of the cell around the table.
	BgColor string
	Color   string

	// Port names the box of the table.
	Port string

	// Align and VAlign place a table with FixedSize in its cell.
	Align  Align
	VAlign VAlign

	Style Style

	// RowRules and ColumnRules draw a rule between each two rows and each
	// two columns, as rows="*" and columns="*" do.
	RowRules    bool
	ColumnRules bool
}

// Row is a <tr>.
type Row struct {
	// Cells are the cells of the row, in order.
	Cells []*Cell

	// Rule draws a rule under the row, as an <hr/> after it does.
	Rule bool
}

// Cell is a <td>. It holds lines of text, one table, or nothing.
type Cell struct {
	// Lines are the lines of the text of the cell.
	Lines []Line

	// Table is the table of the cell, or nil.
	Table *Table

	// Align places the content across the cell, and BAlign places each line
	// that has no align of its own. VAlign places the content down the
	// cell.
	Align  Align
	BAlign Align
	VAlign VAlign

	// Border is the width of the border, or -1 for the cellborder of the
	// table. CellPadding is the space inside the border, or -1 for the
	// cellpadding of the table.
	Border      int
	CellPadding int

	// Width and Height are the least size of the cell, or 0. With
	// FixedSize, they are its size.
	Width     int
	Height    int
	FixedSize bool

	// ColSpan and RowSpan are the number of columns and rows of the cell,
	// 1 by default.
	ColSpan int
	RowSpan int

	// BgColor is the raw fill, and Color the raw color of the border. An
	// empty Color takes the color of the table.
	BgColor string
	Color   string

	// Port names the box of the cell.
	Port string

	// Sides lists the sides of the border, as some of "L", "T", "R" and "B"
	// in that order. Empty is all four.
	Sides string

	Style Style

	// Rule draws a rule after the cell, as a <vr/> after it does.
	Rule bool
}
