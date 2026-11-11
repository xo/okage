package html

// This file ports no upstream file. It parses an HTML-like label of Graphviz
// into the tree of label.go (D38).

import (
	"errors"
	"strconv"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/roxmltree"
)

// SyntaxError is the error of Parse for a label that is not valid. It wraps
// okage.ErrHTMLSyntax, and the error of internal/roxmltree when the label is
// not valid XML.
type SyntaxError struct {
	// Pos is the position of the error in the text of the label.
	Pos Pos

	// Msg says what is wrong.
	Msg string

	// err is the error of internal/roxmltree, or nil.
	err error
}

// Error returns the message, with the position.
func (e *SyntaxError) Error() string {
	return "parsing html: " + e.Pos.String() + ": " + e.Msg
}

// Unwrap returns okage.ErrHTMLSyntax, and the error of the XML parser when
// there is one.
func (e *SyntaxError) Unwrap() []error {
	if e.err != nil {
		return []error{okage.ErrHTMLSyntax, e.err}
	}
	return []error{okage.ErrHTMLSyntax}
}

// Issue is a part of a label that html does not read, or reads with its
// default: an unknown attribute or tag, an invalid value, or a port name that
// repeats.
type Issue struct {
	// Pos is the position of the part in the text of the label.
	Pos Pos

	// Msg says what html does not read.
	Msg string
}

// String returns the message, with the position.
func (i Issue) String() string {
	return i.Pos.String() + ": " + i.Msg
}

// Parse parses the text of an HTML-like label of Graphviz: the text between
// the outer < and > of label=<...> in DOT. It returns the label and the
// issues, which are the parts that it does not read. It returns a
// *SyntaxError, which wraps okage.ErrHTMLSyntax, for a label that is not
// valid.
func Parse(src string) (*Label, []Issue, error) {
	s := newSource(src)
	doc, err := roxmltree.Parse(s.doc)
	if err != nil {
		if xe, ok := errors.AsType[*roxmltree.Error](err); ok {
			at := xe.Pos()
			msg := strings.TrimSuffix(xe.Message(), " at "+at.String())
			return nil, nil, &SyntaxError{Pos: s.pos(s.docOffset(at.Row, at.Col)), Msg: msg, err: err}
		}
		return nil, nil, &SyntaxError{Pos: Pos{Line: 1, Column: 1}, Msg: err.Error(), err: err}
	}
	p := &parser{s: s, ports: map[string]bool{}}
	lines, table, err := p.content(doc.RootElement(), Font{})
	if err != nil {
		return nil, nil, err
	}
	return &Label{Lines: lines, Table: table}, p.issues, nil
}

// parser holds the state of Parse.
type parser struct {
	s      *source
	issues []Issue
	ports  map[string]bool
}

// syntax returns a *SyntaxError at an offset of the document.
func (p *parser) syntax(at int, msg string) error {
	return &SyntaxError{Pos: p.s.pos(at), Msg: msg}
}

// issue adds an issue at an offset of the document.
func (p *parser) issue(at int, msg string) {
	p.issues = append(p.issues, Issue{Pos: p.s.pos(at), Msg: msg})
}

// The names that repeat in the parser.
const (
	nameAlign = "align"
	nameFont  = "font"
	nameColor = "color"
)

// tagName returns the name of an element in lower case.
func tagName(n roxmltree.Node) string {
	return strings.ToLower(n.TagName().Name())
}

// textOf returns the text of a text node without the characters below
// U+0020, as Graphviz drops them.
func textOf(n roxmltree.Node) string {
	t, _ := n.Text()
	if strings.IndexFunc(t, func(r rune) bool { return r < ' ' }) < 0 {
		return t
	}
	return strings.Map(func(r rune) rune {
		if r < ' ' {
			return -1
		}
		return r
	}, t)
}

// textStart returns the offset of the first character of a text node that
// is not white space, so that an error or an issue points at the text.
func textStart(n roxmltree.Node) int {
	r := n.Range()
	raw := n.Document().InputText()[r.Start:r.End]
	i := strings.IndexFunc(raw, func(c rune) bool { return c != ' ' && c != '\t' && c != '\n' && c != '\r' })
	if i < 0 {
		return r.Start
	}
	return r.Start + i
}

// isSpace reports whether a text is empty or only spaces.
func isSpace(t string) bool {
	return strings.Trim(t, " ") == ""
}

// content holds the state of the content of a label or of a cell.
type content struct {
	lines []Line
	cur   Line
	table *Table
	// text is the offset of the first text that is not a space, or of the
	// first <br/>, or -1.
	text int
}

// content reads the content of a label or of a cell: lines of text, one
// table, or nothing. The tags of a font can wrap the table.
func (p *parser) content(n roxmltree.Node, f Font) ([]Line, *Table, error) {
	c := &content{text: -1}
	if err := p.walk(n, f, c); err != nil {
		return nil, nil, err
	}
	if c.table != nil {
		return nil, c.table, nil
	}
	if len(c.cur.Spans) > 0 {
		c.lines = append(c.lines, c.cur)
	}
	return c.lines, nil, nil
}

// walk reads the children of an element of text in a font.
func (p *parser) walk(n roxmltree.Node, f Font, c *content) error {
	for child := range n.Children() {
		switch {
		case child.IsText():
			t := textOf(child)
			if t == "" {
				continue
			}
			if !isSpace(t) {
				if c.table != nil {
					return p.syntax(textStart(child), "text next to a table")
				}
				if c.text < 0 {
					c.text = textStart(child)
				}
			}
			c.addSpan(Span{Text: t, Font: f})
		case child.IsElement():
			if err := p.element(child, f, c); err != nil {
				return err
			}
		}
	}
	return nil
}

// addSpan adds a run of text to the current line. A run in the same font as
// the run before it joins that run.
func (c *content) addSpan(s Span) {
	if k := len(c.cur.Spans) - 1; k >= 0 && c.cur.Spans[k].Font == s.Font {
		c.cur.Spans[k].Text += s.Text
		return
	}
	c.cur.Spans = append(c.cur.Spans, s)
}

// element reads an element of text: a <br/>, a tag of a font, or a table.
func (p *parser) element(n roxmltree.Node, f Font, c *content) error {
	at := n.Range().Start
	name := tagName(n)
	switch name {
	case "br":
		if n.HasChildren() {
			return p.syntax(at, "content in <br>")
		}
		if c.table != nil {
			return p.syntax(at, "<br/> next to a table")
		}
		if c.text < 0 {
			c.text = at
		}
		c.cur.Align = AlignDefault
		for a := range n.Attributes() {
			if strings.ToLower(a.Name()) != nameAlign {
				p.unknownAttr(a, name)
				continue
			}
			c.cur.Align = p.align(a, false)
		}
		c.lines = append(c.lines, c.cur)
		c.cur = Line{}
		return nil
	case "b", "i", "u", "o", "s", "sub", "sup", nameFont:
		switch name {
		case "b":
			f.Bold = true
		case "i":
			f.Italic = true
		case "u":
			f.Underline = true
		case "o":
			f.Overline = true
		case "s":
			f.Strike = true
		case "sub":
			f.Sub = true
		case "sup":
			f.Sup = true
		case nameFont:
			p.fontAttrs(n, &f)
		}
		if name != nameFont {
			for a := range n.Attributes() {
				p.unknownAttr(a, name)
			}
		}
		return p.walk(n, f, c)
	case "table":
		if c.text >= 0 {
			return p.syntax(at, "a table next to text")
		}
		if c.table != nil {
			return p.syntax(at, "a second table")
		}
		t, err := p.table(n, f)
		if err != nil {
			return err
		}
		c.table = t
		return nil
	case "img":
		p.issue(at, "the tag <img> is not read")
		return nil
	default:
		return p.syntax(at, "the tag <"+name+"> where text goes")
	}
}

// fontAttrs reads the attributes of a <font>.
func (p *parser) fontAttrs(n roxmltree.Node, f *Font) {
	for a := range n.Attributes() {
		switch strings.ToLower(a.Name()) {
		case nameColor:
			f.Color = a.Value()
		case "face":
			f.Face = a.Value()
		case "point-size":
			v, err := strconv.ParseFloat(strings.TrimSpace(a.Value()), 64)
			if err != nil || v <= 0 || v > 255 {
				p.badValue(a)
				continue
			}
			f.Size = v
		default:
			p.unknownAttr(a, nameFont)
		}
	}
}

// table reads a <table> in a font.
func (p *parser) table(n roxmltree.Node, f Font) (*Table, error) {
	t := &Table{Font: f, Border: 1, CellBorder: -1, CellSpacing: 2, CellPadding: 2}
	for a := range n.Attributes() {
		p.tableAttr(t, a)
	}
	err := p.items(n, group{item: "tr", rule: "hr", in: "a table", between: "two rows"},
		func(child roxmltree.Node) error {
			r, err := p.row(child, f)
			if err != nil {
				return err
			}
			t.Rows = append(t.Rows, r)
			return nil
		},
		func() bool {
			if len(t.Rows) == 0 {
				return false
			}
			t.Rows[len(t.Rows)-1].Rule = true
			return true
		})
	if err != nil {
		return nil, err
	}
	if len(t.Rows) == 0 {
		return nil, p.syntax(n.Range().Start, "a table with no row")
	}
	return t, nil
}

// row reads a <tr> in a font.
func (p *parser) row(n roxmltree.Node, f Font) (*Row, error) {
	r := &Row{}
	for a := range n.Attributes() {
		p.unknownAttr(a, "tr")
	}
	err := p.items(n, group{item: "td", rule: "vr", in: "a row", between: "two cells"},
		func(child roxmltree.Node) error {
			c, err := p.cell(child, f)
			if err != nil {
				return err
			}
			r.Cells = append(r.Cells, c)
			return nil
		},
		func() bool {
			if len(r.Cells) == 0 {
				return false
			}
			r.Cells[len(r.Cells)-1].Rule = true
			return true
		})
	if err != nil {
		return nil, err
	}
	if len(r.Cells) == 0 {
		return nil, p.syntax(n.Range().Start, "a row with no cell")
	}
	return r, nil
}

// group names the children of a <table> or a <tr>: the item elements, the
// rule elements between them, and the words of the messages.
type group struct {
	item, rule, in, between string
}

// items reads the children of a <table> or a <tr>: the items, with a rule
// between two of them. add reads an item. mark sets a rule after the last
// item, and reports false when there is no item yet.
func (p *parser) items(n roxmltree.Node, g group, add func(roxmltree.Node) error, mark func() bool) error {
	rule := -1
	notBetween := "<" + g.rule + "/> not between " + g.between
	for child := range n.Children() {
		at := child.Range().Start
		switch {
		case child.IsText():
			if t := textOf(child); !isSpace(t) {
				p.issue(textStart(child), "text outside a cell is not read")
			}
		case child.IsElement():
			switch name := tagName(child); name {
			case g.item:
				if err := add(child); err != nil {
					return err
				}
				rule = -1
			case g.rule:
				if child.HasChildren() {
					return p.syntax(at, "content in <"+name+">")
				}
				if rule >= 0 || !mark() {
					return p.syntax(at, notBetween)
				}
				for a := range child.Attributes() {
					p.unknownAttr(a, name)
				}
				rule = at
			default:
				return p.syntax(at, "the tag <"+name+"> in "+g.in)
			}
		}
	}
	if rule >= 0 {
		return p.syntax(rule, notBetween)
	}
	return nil
}

// cell reads a <td> in a font.
func (p *parser) cell(n roxmltree.Node, f Font) (*Cell, error) {
	c := &Cell{Border: -1, CellPadding: -1, ColSpan: 1, RowSpan: 1}
	for a := range n.Attributes() {
		p.cellAttr(c, a)
	}
	lines, table, err := p.content(n, f)
	if err != nil {
		return nil, err
	}
	c.Lines, c.Table = lines, table
	return c, nil
}

// tableAttr reads an attribute of a <table>.
func (p *parser) tableAttr(t *Table, a roxmltree.Attribute) {
	switch strings.ToLower(a.Name()) {
	case "border":
		p.intAttr(a, &t.Border, 0, 255)
	case "cellborder":
		p.intAttr(a, &t.CellBorder, 0, 127)
	case "cellspacing":
		p.intAttr(a, &t.CellSpacing, -128, 127)
	case "cellpadding":
		p.intAttr(a, &t.CellPadding, 0, 255)
	case "width":
		p.intAttr(a, &t.Width, 0, 65535)
	case "height":
		p.intAttr(a, &t.Height, 0, 65535)
	case "fixedsize":
		p.boolAttr(a, &t.FixedSize)
	case "bgcolor":
		t.BgColor = a.Value()
	case nameColor:
		t.Color = a.Value()
	case "port":
		p.port(a, &t.Port)
	case nameAlign:
		t.Align = p.align(a, false)
	case "valign":
		t.VAlign = p.valign(a)
	case "style":
		t.Style = p.style(a)
	case "rows":
		t.RowRules = p.rules(a)
	case "columns":
		t.ColumnRules = p.rules(a)
	default:
		p.unknownAttr(a, "table")
	}
}

// cellAttr reads an attribute of a <td>.
func (p *parser) cellAttr(c *Cell, a roxmltree.Attribute) {
	switch strings.ToLower(a.Name()) {
	case nameAlign:
		c.Align = p.align(a, true)
	case "balign":
		c.BAlign = p.align(a, false)
	case "valign":
		c.VAlign = p.valign(a)
	case "border":
		p.intAttr(a, &c.Border, 0, 127)
	case "cellpadding":
		p.intAttr(a, &c.CellPadding, 0, 255)
	case "width":
		p.intAttr(a, &c.Width, 0, 65535)
	case "height":
		p.intAttr(a, &c.Height, 0, 65535)
	case "fixedsize":
		p.boolAttr(a, &c.FixedSize)
	case "colspan":
		p.intAttr(a, &c.ColSpan, 1, 65535)
	case "rowspan":
		p.intAttr(a, &c.RowSpan, 1, 65535)
	case "bgcolor":
		c.BgColor = a.Value()
	case nameColor:
		c.Color = a.Value()
	case "port":
		p.port(a, &c.Port)
	case "sides":
		c.Sides = p.sides(a)
	case "style":
		c.Style = p.style(a)
	default:
		p.unknownAttr(a, "td")
	}
}

// unknownAttr adds the issue of an attribute that html does not read.
func (p *parser) unknownAttr(a roxmltree.Attribute, tag string) {
	p.issue(a.Range().Start, "the attribute "+strings.ToLower(a.Name())+" of <"+tag+"> is not read")
}

// badValue adds the issue of an invalid value. The attribute keeps its
// default.
func (p *parser) badValue(a roxmltree.Attribute) {
	p.issue(a.Range().Start, "the value "+strconv.Quote(a.Value())+" of "+strings.ToLower(a.Name())+" is not valid")
}

// intAttr reads an integer from lo to hi.
func (p *parser) intAttr(a roxmltree.Attribute, dst *int, lo, hi int) {
	v, err := strconv.Atoi(strings.TrimSpace(a.Value()))
	if err != nil || v < lo || v > hi {
		p.badValue(a)
		return
	}
	*dst = v
}

// boolAttr reads true or false.
func (p *parser) boolAttr(a roxmltree.Attribute, dst *bool) {
	switch strings.ToLower(strings.TrimSpace(a.Value())) {
	case "true":
		*dst = true
	case "false":
		*dst = false
	default:
		p.badValue(a)
	}
}

// port reads a port name. A name that repeats is an issue, and the first box
// keeps it.
func (p *parser) port(a roxmltree.Attribute, dst *string) {
	v := a.Value()
	if p.ports[v] {
		p.issue(a.Range().Start, "the port "+strconv.Quote(v)+" repeats")
		return
	}
	p.ports[v] = true
	*dst = v
}

// align reads left, right or center, and text when text is set.
func (p *parser) align(a roxmltree.Attribute, text bool) Align {
	switch strings.ToLower(strings.TrimSpace(a.Value())) {
	case "left":
		return AlignLeft
	case "right":
		return AlignRight
	case "center":
		return AlignCenter
	case "text":
		if text {
			return AlignText
		}
	}
	p.badValue(a)
	return AlignDefault
}

// valign reads top, bottom or middle.
func (p *parser) valign(a roxmltree.Attribute) VAlign {
	switch strings.ToLower(strings.TrimSpace(a.Value())) {
	case "top":
		return VAlignTop
	case "bottom":
		return VAlignBottom
	case "middle":
		return VAlignMiddle
	}
	p.badValue(a)
	return VAlignDefault
}

// style reads a list of styles, split by commas or spaces: rounded, solid,
// dashed and dotted. Another style, such as radial or invis, is an issue.
func (p *parser) style(a roxmltree.Attribute) Style {
	var s Style
	for _, v := range strings.FieldsFunc(strings.ToLower(a.Value()), func(r rune) bool { return r == ',' || r == ' ' }) {
		switch v {
		case "rounded":
			s.Rounded = true
		case "solid":
			s.Line = LineSolid
		case "dashed":
			s.Line = LineDashed
		case "dotted":
			s.Line = LineDotted
		default:
			p.issue(a.Range().Start, "the style "+strconv.Quote(v)+" is not read")
		}
	}
	return s
}

// rules reads rows="*" or columns="*".
func (p *parser) rules(a roxmltree.Attribute) bool {
	if strings.TrimSpace(a.Value()) == "*" {
		return true
	}
	p.badValue(a)
	return false
}

// sides reads some of the letters L, T, R and B, in any case and order.
func (p *parser) sides(a roxmltree.Attribute) string {
	var has [4]bool
	for _, r := range strings.ToUpper(a.Value()) {
		i := strings.IndexRune("LTRB", r)
		if i < 0 {
			p.badValue(a)
			return ""
		}
		has[i] = true
	}
	var b strings.Builder
	for i, ok := range has {
		if ok {
			b.WriteByte("LTRB"[i])
		}
	}
	if b.Len() == 0 {
		p.badValue(a)
	}
	return b.String()
}
