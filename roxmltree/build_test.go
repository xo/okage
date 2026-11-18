package roxmltree

import (
	"errors"
	"slices"
	"testing"

	"github.com/xo/okage"
)

// This file ports no upstream test. It tests ParseRoot and Builder of D58.

// buildTestXML is a document with namespaces, attributes, text and nested
// elements.
const buildTestXML = `<?xml version="1.0"?>
<!-- a comment -->
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="10" height="20">
  <g id="a" xml:space="preserve">text<!-- c --> more<rect xlink:href="#x"/>tail</g>
  <foreignObject><p xmlns="http://www.w3.org/1999/xhtml" xmlns:e="urn:e" e:x="1">html</p></foreignObject>
  <empty xmlns=""/>
</svg>`

// shape returns the tag names, the attributes and the text of the nodes of
// a document, without the comments, in document order.
func shape(d *Document) []string {
	var out []string
	for n := range d.Descendants() {
		switch {
		case n.IsElement():
			out = append(out, "<"+n.TagName().String())
			for a := range n.Attributes() {
				ns, _ := a.Namespace()
				out = append(out, "@{"+ns+"}"+a.Name()+"="+a.Value())
			}
			for ns := range n.Namespaces() {
				out = append(out, "ns "+ns.String())
			}
		case n.IsText():
			t, _ := n.Text()
			out = append(out, "text "+t)
		}
	}
	return out
}

func TestBuilderCopy(t *testing.T) {
	parsed, err := Parse(buildTestXML)
	if err != nil {
		t.Fatal(err)
	}
	b := NewBuilder(DefaultParsingOptions())
	if err := b.Copy(parsed.Root()); err != nil {
		t.Fatal(err)
	}
	built, err := b.Document()
	if err != nil {
		t.Fatal(err)
	}
	// The parsed document splits the text around the comment, and the copy
	// joins it, so compare with the text joined.
	want := shape(parsed)
	i := slices.Index(want, "text text")
	if i < 0 || want[i+1] != "text  more" {
		t.Fatalf("the parsed document is %q", want)
	}
	want = slices.Concat(want[:i], []string{"text text more"}, want[i+2:])
	if got := shape(built); !slices.Equal(got, want) {
		t.Errorf("the copy is\n%q\nwant\n%q", got, want)
	}

	// The navigation of the copy works as in a parsed document.
	g, ok := built.RootElement().FirstElementChild()
	if !ok || g.TagName() != ExpandedNameFromTuple("http://www.w3.org/2000/svg", "g") {
		t.Fatalf("the first child is %v", g)
	}
	if tail, _ := g.Tail(); tail != "\n  " {
		t.Errorf("g has the tail %q, want the white space", tail)
	}
	if v, ok := g.Attribute(ExpandedNameFromTuple(NSXMLURI, "space")); !ok || v != "preserve" {
		t.Errorf("xml:space is %q, %v", v, ok)
	}
	if p, ok := g.LookupPrefix("http://www.w3.org/1999/xlink"); !ok || p != "xlink" {
		t.Errorf("the prefix of xlink is %q, %v", p, ok)
	}
	if last, ok := built.RootElement().LastElementChild(); !ok || last.TagName().Name() != "empty" {
		t.Errorf("the last child is %v", last)
	}
}

func TestBuilderCalls(t *testing.T) {
	const svg = "http://www.w3.org/2000/svg"
	b := NewBuilder(DefaultParsingOptions())
	steps := []error{
		b.StartElement(ExpandedNameFromTuple(svg, "svg")),
		b.Attribute(ExpandedNameFromStr("width"), "0"),
		b.Text("a"),
		b.Text("b"),
		b.StartElement(ExpandedNameFromTuple("http://www.w3.org/1999/xhtml", "p")),
		b.Attribute(ExpandedNameFromTuple("urn:x", "y"), "z"),
		b.EndElement(),
		b.Text("c"),
		b.EndElement(),
	}
	for i, err := range steps {
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	d, err := b.Document()
	if err != nil {
		t.Fatal(err)
	}
	root := d.RootElement()
	if text, _ := root.Text(); text != "ab" {
		t.Errorf("the text is %q, want ab", text)
	}
	p, _ := root.FirstElementChild()
	if tail, _ := p.Tail(); tail != "c" {
		t.Errorf("the tail is %q, want c", tail)
	}
	if v, ok := p.Attribute(ExpandedNameFromTuple("urn:x", "y")); !ok || v != "z" {
		t.Errorf("the attribute is %q, %v", v, ok)
	}
	if _, ok := p.LookupPrefix("urn:x"); ok {
		t.Error("the builder declared a prefix")
	}
}

func TestBuilderErrors(t *testing.T) {
	name := ExpandedNameFromStr("e")
	tests := []struct {
		name  string
		build func(b *Builder) error
		want  error
	}{
		{"no root", func(*Builder) error { return nil }, okage.ErrXMLNoRootNode},
		{"unclosed", func(b *Builder) error { return b.StartElement(name) }, okage.ErrXMLUnclosedRootNode},
		{"two roots", func(b *Builder) error {
			_ = b.StartElement(name)
			_ = b.EndElement()
			return b.StartElement(name)
		}, okage.ErrXMLBuildOrder},
		{"text outside", func(b *Builder) error { return b.Text("x") }, okage.ErrXMLBuildOrder},
		{"end without start", func(b *Builder) error { return b.EndElement() }, okage.ErrXMLBuildOrder},
		{"late attribute", func(b *Builder) error {
			_ = b.StartElement(name)
			_ = b.Text("x")
			return b.Attribute(name, "v")
		}, okage.ErrXMLBuildOrder},
		{"duplicated attribute", func(b *Builder) error {
			_ = b.StartElement(name)
			_ = b.Attribute(name, "1")
			return b.Attribute(name, "2")
		}, okage.ErrXMLDuplicatedAttribute},
		{"duplicated namespace", func(b *Builder) error {
			_ = b.StartElement(name)
			_ = b.Namespace("a", "urn:a")
			return b.Namespace("a", "urn:b")
		}, okage.ErrXMLDuplicatedNamespace},
		{"xmlns URI", func(b *Builder) error {
			_ = b.StartElement(name)
			return b.Namespace("a", NSXMLNSURI)
		}, okage.ErrXMLUnexpectedXMLNSURI},
		{"empty name", func(b *Builder) error { return b.StartElement(ExpandedNameFromStr("")) }, okage.ErrXMLInvalidName},
		{"nodes limit", func(b *Builder) error {
			_ = b.StartElement(name)
			_ = b.StartElement(name)
			return b.StartElement(name)
		}, okage.ErrXMLNodesLimitReached},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := DefaultParsingOptions()
			opt.NodesLimit = 3
			b := NewBuilder(opt)
			err := tt.build(b)
			if err == nil {
				_, err = b.Document()
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("the error is %v, want %v", err, tt.want)
			}
			// The builder keeps its error.
			if _, again := b.Document(); !errors.Is(again, tt.want) {
				t.Errorf("Document after the error returns %v", again)
			}
		})
	}
}

func TestParseRoot(t *testing.T) {
	const text = `<?xml version="1.0"?>
<!DOCTYPE svg [ <!ENTITY w "15"> ]>
<svg xmlns="http://www.w3.org/2000/svg" width="&w;" viewBox="0 0 1 2"><g><broken</svg>`
	opt := DefaultParsingOptions()
	opt.AllowDTD = true
	if _, err := ParseWithOptions(text, opt); err == nil {
		t.Fatal("Parse accepts the broken document")
	}
	d, err := ParseRoot(text, opt)
	if err != nil {
		t.Fatal(err)
	}
	root := d.RootElement()
	if root.TagName() != ExpandedNameFromTuple("http://www.w3.org/2000/svg", "svg") {
		t.Errorf("the root is %v", root.TagName())
	}
	if v, _ := root.Attribute(ExpandedNameFromStr("width")); v != "15" {
		t.Errorf("width is %q, want 15", v)
	}
	if root.HasChildren() {
		t.Error("the root has children")
	}

	for _, bad := range []string{"", "<!-- only a comment -->", "<a b='1' b='2'>"} {
		if _, err := ParseRoot(bad, DefaultParsingOptions()); err == nil {
			t.Errorf("ParseRoot(%q) gives no error", bad)
		}
	}
	if _, err := ParseRoot("<e/>", DefaultParsingOptions()); err != nil {
		t.Errorf("ParseRoot of an empty element: %v", err)
	}
}
