package svg

import (
	"bytes"
	"compress/gzip"
	"errors"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/roxmltree"
)

// This file ports no upstream test. It tests the API of D58.

func TestReadDimensions(t *testing.T) {
	tests := []struct {
		data string
		want Dimensions
	}{
		{
			`<svg xmlns="http://www.w3.org/2000/svg" width="10mm" height="5" viewBox="0 0 20 10"><g>`,
			Dimensions{
				Width: Length{Number: 10, Unit: LengthUnitMm}, HasWidth: true,
				Height: Length{Number: 5, Unit: LengthUnitNone}, HasHeight: true,
				ViewBox: mustNonZeroRect(path.NonZeroRectFromXYWH(0, 0, 20, 10)), HasViewBox: true,
			},
		},
		{
			`<?xml version="1.0"?><!DOCTYPE svg [<!ENTITY w "50%">]><svg width="&w;" height="0em" viewBox="0 0 0 10"/>`,
			Dimensions{
				Width: Length{Number: 50, Unit: LengthUnitPercent}, HasWidth: true,
				Height: Length{Number: 0, Unit: LengthUnitEm}, HasHeight: true,
			},
		},
		{
			`<svg xmlns="http://www.w3.org/2000/svg" width="wide"/>`,
			Dimensions{},
		},
	}
	for _, tt := range tests {
		got, err := ReadDimensions([]byte(tt.data))
		if err != nil {
			t.Errorf("ReadDimensions(%q): %v", tt.data, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ReadDimensions(%q) = %+v, want %+v", tt.data, got, tt.want)
		}
	}

	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="7"/>`))
	_ = w.Close()
	if got, err := ReadDimensions(gz.Bytes()); err != nil || !got.HasWidth || got.Width.Number != 7 {
		t.Errorf("ReadDimensions of SVGZ data = %+v, %v", got, err)
	}

	for _, c := range []struct {
		data string
		want error
	}{
		{`<html/>`, okage.ErrXMLNoRootNode},
		{`<html/>`, okage.ErrSVGParsingFailed},
		{`<svg`, okage.ErrSVGParsingFailed},
		{"<svg width='\xff'/>", okage.ErrSVGNotAnUTF8Str},
		{"\x1f\x8bnot gzip", okage.ErrSVGMalformedGZip},
	} {
		if _, err := ReadDimensions([]byte(c.data)); !errors.Is(err, c.want) {
			t.Errorf("ReadDimensions(%q) = %v, want %v", c.data, err, c.want)
		}
	}
}

func TestFromXMLTree(t *testing.T) {
	doc, err := roxmltree.Parse(parserTestSVG)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := FromXMLTree(doc)
	if err != nil {
		t.Fatal(err)
	}
	if got := tree.ToString(); got != parserTestWant {
		t.Errorf("got\n%s", got)
	}

	// A document of a Builder, as the tree of an HTML parser makes it,
	// gives the same tree as the text.
	const svgNS = "http://www.w3.org/2000/svg"
	b := roxmltree.NewBuilder(roxmltree.DefaultParsingOptions())
	steps := []error{
		b.StartElement(roxmltree.ExpandedNameFromTuple(svgNS, "svg")),
		b.Attribute(roxmltree.ExpandedNameFromStr("width"), "20"),
		b.Attribute(roxmltree.ExpandedNameFromStr("height"), "10"),
		b.StartElement(roxmltree.ExpandedNameFromTuple(svgNS, "style")),
		b.Text("rect { fill: green }"),
		b.EndElement(),
		b.StartElement(roxmltree.ExpandedNameFromTuple(svgNS, "rect")),
		b.Attribute(roxmltree.ExpandedNameFromStr("id"), "r"),
		b.Attribute(roxmltree.ExpandedNameFromStr("width"), "5"),
		b.Attribute(roxmltree.ExpandedNameFromStr("height"), "5"),
		b.EndElement(),
		b.StartElement(roxmltree.ExpandedNameFromTuple(svgNS, "use")),
		b.Attribute(roxmltree.ExpandedNameFromTuple("http://www.w3.org/1999/xlink", "href"), "#r"),
		b.Attribute(roxmltree.ExpandedNameFromStr("x"), "10"),
		b.EndElement(),
		b.EndElement(),
	}
	for i, err := range steps {
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	built, err := b.Document()
	if err != nil {
		t.Fatal(err)
	}
	fromBuilder, err := FromXMLTree(built)
	if err != nil {
		t.Fatal(err)
	}
	fromText, err := FromStr(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="20" height="10">` +
		`<style>rect { fill: green }</style><rect id="r" width="5" height="5"/><use xlink:href="#r" x="10"/></svg>`)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fromBuilder.ToString(), fromText.ToString(); got != want {
		t.Errorf("the tree of the builder is\n%s\nwant\n%s", got, want)
	}
}

func TestFromXMLNode(t *testing.T) {
	const page = `<html xmlns="http://www.w3.org/1999/xhtml"><head><style>rect { fill: red }</style></head>
<body><p id="r">text</p><svg xmlns="http://www.w3.org/2000/svg" width="20" height="10"><rect width="5" height="5"/></svg></body></html>`
	doc, err := roxmltree.Parse(page)
	if err != nil {
		t.Fatal(err)
	}
	var node roxmltree.Node
	for n := range doc.Descendants() {
		if n.TagName().Name() == "svg" {
			node = n
		}
	}
	tree, err := FromXMLNode(node)
	if err != nil {
		t.Fatal(err)
	}
	want, err := FromStr(`<svg xmlns="http://www.w3.org/2000/svg" width="20" height="10"><rect width="5" height="5"/></svg>`)
	if err != nil {
		t.Fatal(err)
	}
	// The style sheet of the page does not apply to the image.
	if got := tree.ToString(); got != want.ToString() {
		t.Errorf("got\n%s\nwant\n%s", got, want.ToString())
	}
	if d := NodeDimensions(node); !d.HasWidth || d.Width.Number != 20 || d.HasViewBox {
		t.Errorf("NodeDimensions = %+v", d)
	}
	if _, err := FromXMLNode(doc.Root()); !errors.Is(err, okage.ErrXMLNoRootNode) {
		t.Errorf("FromXMLNode of the root node: %v", err)
	}
}

func TestWithZeroSize(t *testing.T) {
	zero := []string{
		`<svg xmlns="http://www.w3.org/2000/svg" width="0" height="10"><rect width="5" height="5"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="0%"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="0" viewBox="0 0 10 10"/>`,
	}
	for _, data := range zero {
		if _, err := FromStr(data); !errors.Is(err, okage.ErrSVGInvalidSize) {
			t.Errorf("FromStr(%q) without the option: %v", data, err)
		}
		tree, err := FromStr(data, WithZeroSize(true))
		if err != nil {
			t.Errorf("FromStr(%q): %v", data, err)
			continue
		}
		if tree.Size() != (Size{}) || tree.Size().Width() != 0 || tree.Size().Height() != 0 {
			t.Errorf("FromStr(%q) has the size %v", data, tree.Size())
		}
		if tree.Root().HasChildren() {
			t.Errorf("FromStr(%q) has children", data)
		}
		r := New(WithZeroSize(true), WithLoadSystemFonts(false))
		if _, err := r.Render([]byte(data)); !errors.Is(err, okage.ErrSVGInvalidRenderSize) {
			t.Errorf("Render(%q): %v", data, err)
		}
	}
	for _, data := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg" width="-1" height="10"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="0" height="-1"/>`,
	} {
		if _, err := FromStr(data, WithZeroSize(true)); !errors.Is(err, okage.ErrSVGInvalidSize) {
			t.Errorf("FromStr(%q): %v", data, err)
		}
	}
	if !strings.Contains(mustTree(t, `<svg xmlns="http://www.w3.org/2000/svg" width="0" height="10"/>`).ToString(), `width="0"`) {
		t.Error("the writer does not write the width 0")
	}
}

// mustTree parses data with WithZeroSize(true).
func mustTree(t *testing.T, data string) *Tree {
	t.Helper()
	tree, err := FromStr(data, WithZeroSize(true))
	if err != nil {
		t.Fatal(err)
	}
	return tree
}
