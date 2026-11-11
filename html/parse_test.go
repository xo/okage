package html

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/xo/okage"
)

func TestParseText(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		lines []Line
	}{
		{"Spaces", "  a   b  ", []Line{{Spans: []Span{{Text: "  a   b  "}}}}},
		{"Newline", "a\nb\tc", []Line{{Spans: []Span{{Text: "abc"}}}}},
		{"Entities", "x&eacute;&nbsp;&#65;&#x42;&Alpha;&hellip;&amp;&lt;y", []Line{{Spans: []Span{{Text: "xé ABΑ…&<y"}}}}},
		{"Lines", `a<br align="left"/>b<BR ALIGN="Right"/>c<br/>`, []Line{
			{Spans: []Span{{Text: "a"}}, Align: AlignLeft},
			{Spans: []Span{{Text: "b"}}, Align: AlignRight},
			{Spans: []Span{{Text: "c"}}},
		}},
		{"EmptyLine", "a<br/><br/>b", []Line{
			{Spans: []Span{{Text: "a"}}},
			{},
			{Spans: []Span{{Text: "b"}}},
		}},
		{"Fonts", `a<B>b<i>c</i></B><font color="red" point-size="20" face="Mono">d<u><o><s>e</s></o></u></font><sub>f</sub><sup>g</sup>`, []Line{{Spans: []Span{
			{Text: "a"},
			{Text: "b", Font: Font{Bold: true}},
			{Text: "c", Font: Font{Bold: true, Italic: true}},
			{Text: "d", Font: Font{Color: "red", Size: 20, Face: "Mono"}},
			{Text: "e", Font: Font{Color: "red", Size: 20, Face: "Mono", Underline: true, Overline: true, Strike: true}},
			{Text: "f", Font: Font{Sub: true}},
			{Text: "g", Font: Font{Sup: true}},
		}}}},
		{"Join", "a<b></b>b", []Line{{Spans: []Span{{Text: "ab"}}}}},
		{"Empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, issues, err := Parse(tt.src)
			if err != nil {
				t.Fatal(err)
			}
			if len(issues) != 0 {
				t.Errorf("issues: %v", issues)
			}
			if l.Table != nil {
				t.Fatal("a table")
			}
			if !reflect.DeepEqual(l.Lines, tt.lines) {
				t.Errorf("lines:\n got %#v\nwant %#v", l.Lines, tt.lines)
			}
		})
	}
}

func TestParseTable(t *testing.T) {
	src := `
	<font color="red" point-size="14">
	<TABLE border="0" cellBorder="1" cellspacing="0" cellpadding="4" bgcolor="white" color="blue" port="t" rows="*" columns="*" style="rounded, dashed" width="40" height="20" fixedsize="true" align="left" valign="top">
		junk
		<tr><td bgcolor="lightblue" colspan="2">"authors"</td></tr>
		<hr/>
		<tr><td align="left" PORT="author_id" balign="right" valign="bottom" sides="bl" border="2" cellpadding="3" width="10" height="11" rowspan="2">author_id</td><vr/><td align="text" style="dotted"><table><tr><td>x</td></tr></table></td></tr>
	</TABLE>
	</font>`
	l, issues, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Lines) != 0 || l.Table == nil {
		t.Fatalf("got %#v", l)
	}
	if len(issues) != 1 || issues[0].Msg != "text outside a cell is not read" || issues[0].Pos != (Pos{Line: 4, Column: 3}) {
		t.Errorf("issues: %v", issues)
	}
	tb := l.Table
	font := Font{Color: "red", Size: 14}
	want := Table{
		Font: font, Border: 0, CellBorder: 1, CellSpacing: 0, CellPadding: 4,
		Width: 40, Height: 20, FixedSize: true, BgColor: "white", Color: "blue", Port: "t",
		Align: AlignLeft, VAlign: VAlignTop, Style: Style{Rounded: true, Line: LineDashed},
		RowRules: true, ColumnRules: true,
	}
	got := *tb
	got.Rows = nil
	if !reflect.DeepEqual(got, want) {
		t.Errorf("table:\n got %#v\nwant %#v", got, want)
	}
	if len(tb.Rows) != 2 || !tb.Rows[0].Rule || tb.Rows[1].Rule {
		t.Fatalf("rows: %#v", tb.Rows)
	}
	c := tb.Rows[0].Cells[0]
	if c.BgColor != "lightblue" || c.ColSpan != 2 || c.Border != -1 || c.CellPadding != -1 {
		t.Errorf("cell 0: %#v", c)
	}
	if !reflect.DeepEqual(c.Lines, []Line{{Spans: []Span{{Text: `"authors"`, Font: font}}}}) {
		t.Errorf("cell 0 lines: %#v", c.Lines)
	}
	c = tb.Rows[1].Cells[0]
	wantCell := Cell{
		Lines: []Line{{Spans: []Span{{Text: "author_id", Font: font}}}},
		Align: AlignLeft, BAlign: AlignRight, VAlign: VAlignBottom,
		Border: 2, CellPadding: 3, Width: 10, Height: 11, ColSpan: 1, RowSpan: 2,
		Port: "author_id", Sides: "LB", Rule: true,
	}
	if !reflect.DeepEqual(*c, wantCell) {
		t.Errorf("cell 1:\n got %#v\nwant %#v", *c, wantCell)
	}
	c = tb.Rows[1].Cells[1]
	if c.Align != AlignText || c.Style.Line != LineDotted || c.Table == nil || c.Table.Font != font {
		t.Errorf("cell 2: %#v", c)
	}
}

func TestParseIssues(t *testing.T) {
	src := `<table href="u" title="t" border="x" sides="L"><tr id="r"><td class="c" colspan="0" style="radial" port="p">a<img src="i.png"/></td><td port="p" align="text2">b</td></tr></table>`
	_, issues, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, i := range issues {
		got = append(got, i.String())
	}
	want := []string{
		"1:8: the attribute href of <table> is not read",
		"1:17: the attribute title of <table> is not read",
		`1:27: the value "x" of border is not valid`,
		"1:38: the attribute sides of <table> is not read",
		"1:52: the attribute id of <tr> is not read",
		"1:63: the attribute class of <td> is not read",
		`1:73: the value "0" of colspan is not valid`,
		`1:85: the style "radial" is not read`,
		"1:110: the tag <img> is not read",
		`1:137: the port "p" repeats`,
		`1:146: the value "text2" of align is not valid`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("issues:\n got %q\nwant %q", got, want)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		src string
		pos Pos
		msg string
	}{
		{"a & b", Pos{1, 3}, ""},
		{"a &foo; b", Pos{1, 3}, ""},
		{"x &eacute;\n&nbsp; &bar;", Pos{2, 8}, ""},
		{"<B>x</b>", Pos{1, 5}, ""},
		{"<table><tr><td>x</td></tr></table>tail", Pos{1, 35}, "text next to a table"},
		{"a<table><tr><td>x</td></tr></table>", Pos{1, 2}, "a table next to text"},
		{"<table><tr><td>x</td></tr></table><table><tr><td>y</td></tr></table>", Pos{1, 35}, "a second table"},
		{`<font color="red">a</font><table><tr><td>x</td></tr></table>`, Pos{1, 27}, "a table next to text"},
		{"<table><tr></tr></table>", Pos{1, 8}, "a row with no cell"},
		{"<table></table>", Pos{1, 1}, "a table with no row"},
		{"<table><tr><td>x</td></tr><hr/></table>", Pos{1, 27}, "<hr/> not between two rows"},
		{"<table><hr/><tr><td>x</td></tr></table>", Pos{1, 8}, "<hr/> not between two rows"},
		{"<table><tr><td>x</td><vr/></tr></table>", Pos{1, 22}, "<vr/> not between two cells"},
		{"<table><tr><td>x</td></tr><tr><td>a<table><tr><td>x</td></tr></table></td></tr></table>", Pos{1, 36}, "a table next to text"},
		{"<td>x</td>", Pos{1, 1}, "the tag <td> where text goes"},
		{"<table><td>x</td></table>", Pos{1, 8}, "the tag <td> in a table"},
		{"a<br>b</br>", Pos{1, 2}, "content in <br>"},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			_, _, err := Parse(tt.src)
			if !errors.Is(err, okage.ErrHTMLSyntax) {
				t.Fatalf("got %v", err)
			}
			var se *SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("got %T", err)
			}
			if se.Pos != tt.pos {
				t.Errorf("pos: got %v, want %v (%v)", se.Pos, tt.pos, err)
			}
			if tt.msg != "" && se.Msg != tt.msg {
				t.Errorf("msg: got %q, want %q", se.Msg, tt.msg)
			}
			if strings.Contains(err.Error(), rootName) {
				t.Errorf("the error names the root element: %v", err)
			}
		})
	}
}
