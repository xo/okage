// Copyright 2018 Yevhenii Reizner
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package roxmltree

// This file ports roxmltree tests/api.rs.

import (
	"slices"
	"strings"
	"testing"
	"unsafe"
)

// mustParse is Document::parse(data).unwrap().
func mustParse(t *testing.T, data string) *Document {
	t.Helper()
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse(%q): %v", data, err)
	}
	return doc
}

// some is Some(v) of an Option<&str>, for a comparison.
type some struct {
	v  string
	ok bool
}

func checkOpt(t *testing.T, what string, v string, ok bool, want some) {
	t.Helper()
	if got := (some{v, ok}); got != want {
		t.Errorf("%s = %+v, want %+v", what, got, want)
	}
}

func checkPos(t *testing.T, what string, got, want TextPos) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

// TestErrorSize is the test error_size in roxmltree tests/api.rs.
//
// Port: a function returns an error as the interface error, which points
// to the *Error. The test checks the size of that interface.
func TestErrorSize(t *testing.T) {
	if size := unsafe.Sizeof(error(nil)); size > 64 {
		t.Errorf("unsafe.Sizeof(error(nil)) = %d, want <= 64", size)
	}
}

// TestRootElement01 is the test root_element_01 in roxmltree tests/api.rs.
func TestRootElement01(t *testing.T) {
	data := `<!-- comment -->
<e/>
`

	doc := mustParse(t, data)
	node := doc.RootElement()
	if got := node.TagName().Name(); got != "e" {
		t.Errorf("TagName().Name() = %q, want %q", got, "e")
	}
}

// TestGetText01 is the test get_text_01 in roxmltree tests/api.rs.
func TestGetText01(t *testing.T) {
	data := `<root>
    Text1
    <item>
        Text2
    </item>
    Text3
</root>
`

	doc := mustParse(t, data)
	root := doc.RootElement()

	v, ok := root.Text()
	checkOpt(t, "root.Text()", v, ok, some{"\n    Text1\n    ", true})
	v, ok = root.Tail()
	checkOpt(t, "root.Tail()", v, ok, some{})

	item := slices.Collect(root.Children())[1]

	v, ok = item.Text()
	checkOpt(t, "item.Text()", v, ok, some{"\n        Text2\n    ", true})
	v, ok = item.Tail()
	checkOpt(t, "item.Tail()", v, ok, some{"\n    Text3\n", true})
}

// TestGetText02 is the test get_text_02 in roxmltree tests/api.rs.
func TestGetText02(t *testing.T) {
	data := "<root>&apos;</root>"

	doc := mustParse(t, data)
	root := doc.RootElement()

	v, ok := root.Text()
	checkOpt(t, "root.Text()", v, ok, some{"'", true})
}

// TestAPI01 is the test api_01 in roxmltree tests/api.rs.
func TestAPI01(t *testing.T) {
	data := "<e a:attr='a_ns' b:attr='b_ns' attr='no_ns' xmlns:b='http://www.ietf.org' " +
		"xmlns:a='http://www.w3.org' xmlns='http://www.uvic.ca'/>\n"

	doc := mustParse(t, data)
	p := doc.RootElement()

	var found some
	for a := range p.Attributes() {
		if _, ok := a.Namespace(); !ok && a.Name() == "attr" {
			found = some{a.Value(), true}
			break
		}
	}
	if found != (some{"no_ns", true}) {
		t.Errorf("attribute attr without a namespace = %+v", found)
	}
	if !p.HasAttribute(ExpandedNameFromStr("attr")) {
		t.Error(`HasAttribute("attr") is false`)
	}

	v, ok := p.Attribute(ExpandedNameFromTuple("http://www.w3.org", "attr"))
	checkOpt(t, `Attribute(("http://www.w3.org", "attr"))`, v, ok, some{"a_ns", true})
	if !p.HasAttribute(ExpandedNameFromTuple("http://www.w3.org", "attr")) {
		t.Error(`HasAttribute(("http://www.w3.org", "attr")) is false`)
	}

	v, ok = p.Attribute(ExpandedNameFromStr("attr2"))
	checkOpt(t, `Attribute("attr2")`, v, ok, some{})
	if p.HasAttribute(ExpandedNameFromStr("attr2")) {
		t.Error(`HasAttribute("attr2") is true`)
	}

	v, ok = p.Attribute(ExpandedNameFromTuple("http://www.w2.org", "attr"))
	checkOpt(t, `Attribute(("http://www.w2.org", "attr"))`, v, ok, some{})
	if p.HasAttribute(ExpandedNameFromTuple("http://www.w2.org", "attr")) {
		t.Error(`HasAttribute(("http://www.w2.org", "attr")) is true`)
	}

	v, ok = p.Attribute(ExpandedNameFromStr("b"))
	checkOpt(t, `Attribute("b")`, v, ok, some{})
	if p.HasAttribute(ExpandedNameFromStr("b")) {
		t.Error(`HasAttribute("b") is true`)
	}

	v, ok = p.Attribute(ExpandedNameFromStr("xmlns"))
	checkOpt(t, `Attribute("xmlns")`, v, ok, some{})
	if p.HasAttribute(ExpandedNameFromStr("xmlns")) {
		t.Error(`HasAttribute("xmlns") is true`)
	}
}

// TestHasLocalName is the test has_local_name in roxmltree tests/api.rs.
func TestHasLocalName(t *testing.T) {
	doc := mustParse(t, `<root xmlns:foo="http://example.com/foo"><foo:bar foo:baz="qux"/></root>`)

	all := slices.Collect(doc.Descendants())
	node := all[len(all)-1]

	if !node.HasTagName(ExpandedNameFromStr("bar")) {
		t.Error(`HasTagName("bar") is false`)
	}

	if !node.HasAttribute(ExpandedNameFromStr("baz")) {
		t.Error(`HasAttribute("baz") is false`)
	}
	v, ok := node.Attribute(ExpandedNameFromStr("baz"))
	checkOpt(t, `Attribute("baz")`, v, ok, some{"qux", true})
}

// TestGetPI is the test get_pi in roxmltree tests/api.rs.
func TestGetPI(t *testing.T) {
	data := `<?target value?>
<root/>
`

	doc := mustParse(t, data)
	node, _ := doc.Root().FirstChild()
	pi, ok := node.PI()
	if !ok || pi != (PI{Target: "target", Value: "value"}) {
		t.Errorf("PI() = %+v, %v", pi, ok)
	}
}

// TestLookupPrefix01 is the test lookup_prefix_01 in roxmltree tests/api.rs.
func TestLookupPrefix01(t *testing.T) {
	data := "<e xmlns:n1='http://www.w3.org' n1:a='b1'/>"

	doc := mustParse(t, data)
	node := doc.RootElement()
	v, ok := node.LookupPrefix("http://www.w3.org")
	checkOpt(t, `LookupPrefix("http://www.w3.org")`, v, ok, some{"n1", true})
	v, ok = node.LookupPrefix("http://www.w4.org")
	checkOpt(t, `LookupPrefix("http://www.w4.org")`, v, ok, some{})
}

// TestLookupPrefix02 is the test lookup_prefix_02 in roxmltree tests/api.rs.
func TestLookupPrefix02(t *testing.T) {
	data := "<e xml:space='preserve'/>"

	doc := mustParse(t, data)
	node := doc.RootElement()
	v, ok := node.LookupPrefix(NSXMLURI)
	checkOpt(t, "LookupPrefix(NSXMLURI)", v, ok, some{"xml", true})
}

// TestLookupNamespaceURI is the test lookup_namespace_uri in roxmltree
// tests/api.rs.
func TestLookupNamespaceURI(t *testing.T) {
	data := "<e xmlns:n1='http://www.w3.org' xmlns='http://www.w4.org'/>"

	doc := mustParse(t, data)
	node := doc.RootElement()
	v, ok := node.LookupNamespaceURI("n1")
	checkOpt(t, `LookupNamespaceURI(Some("n1"))`, v, ok, some{"http://www.w3.org", true})
	v, ok = node.LookupNamespaceURI("")
	checkOpt(t, "LookupNamespaceURI(None)", v, ok, some{"http://www.w4.org", true})
	v, ok = node.LookupNamespaceURI("n2")
	checkOpt(t, `LookupNamespaceURI(Some("n2"))`, v, ok, some{})
}

// TestTextPos01 is the test text_pos_01 in roxmltree tests/api.rs.
func TestTextPos01(t *testing.T) {
	data := `<e a='b'>
    <!-- comment -->
    <p>Text</p>
</e>
`

	doc := mustParse(t, data)
	node := doc.RootElement()
	attr, _ := node.AttributeNode(ExpandedNameFromStr("a"))

	checkPos(t, "root start", doc.TextPosAt(doc.Root().Range().Start), NewTextPos(1, 1))
	checkPos(t, "root end", doc.TextPosAt(doc.Root().Range().End), NewTextPos(5, 1))

	checkPos(t, "node start", doc.TextPosAt(node.Range().Start), NewTextPos(1, 1))
	checkPos(t, "node end", doc.TextPosAt(node.Range().End), NewTextPos(4, 5))

	checkPos(t, "attr start", doc.TextPosAt(attr.Range().Start), NewTextPos(1, 4))
	checkPos(t, "attr end", doc.TextPosAt(attr.Range().End), NewTextPos(1, 9))
	checkPos(t, "qname start", doc.TextPosAt(attr.RangeQName().Start), NewTextPos(1, 4))
	checkPos(t, "qname end", doc.TextPosAt(attr.RangeQName().End), NewTextPos(1, 5))
	checkPos(t, "value start", doc.TextPosAt(attr.RangeValue().Start), NewTextPos(1, 7))
	checkPos(t, "value end", doc.TextPosAt(attr.RangeValue().End), NewTextPos(1, 8))

	// first child is a text/whitespace, not a comment
	first, _ := node.FirstChild()
	comm, _ := first.NextSibling()
	checkPos(t, "comment start", doc.TextPosAt(comm.Range().Start), NewTextPos(2, 5))

	next, _ := comm.NextSibling()
	p, _ := next.NextSibling()
	checkPos(t, "p start", doc.TextPosAt(p.Range().Start), NewTextPos(3, 5))

	text, _ := p.FirstChild()
	checkPos(t, "text start", doc.TextPosAt(text.Range().Start), NewTextPos(3, 8))
}

// TestTextPos02 is the test text_pos_02 in roxmltree tests/api.rs.
func TestTextPos02(t *testing.T) {
	data := "<n1:e xmlns:n1='http://www.w3.org' n1:a='b'/>"

	doc := mustParse(t, data)
	node := doc.RootElement()
	attr, _ := node.AttributeNode(ExpandedNameFromTuple("http://www.w3.org", "a"))

	checkPos(t, "node start", doc.TextPosAt(node.Range().Start), NewTextPos(1, 1))

	checkPos(t, "attr start", doc.TextPosAt(attr.Range().Start), NewTextPos(1, 36))
	checkPos(t, "attr end", doc.TextPosAt(attr.Range().End), NewTextPos(1, 44))
	checkPos(t, "qname start", doc.TextPosAt(attr.RangeQName().Start), NewTextPos(1, 36))
	checkPos(t, "qname end", doc.TextPosAt(attr.RangeQName().End), NewTextPos(1, 40))
	checkPos(t, "value start", doc.TextPosAt(attr.RangeValue().Start), NewTextPos(1, 42))
	checkPos(t, "value end", doc.TextPosAt(attr.RangeValue().End), NewTextPos(1, 43))
}

// TestTextPos03 is the test text_pos_03 in roxmltree tests/api.rs.
func TestTextPos03(t *testing.T) {
	data := `<!-- comment -->
<e/>
`

	doc := mustParse(t, data)
	node := doc.RootElement()

	checkPos(t, "node start", doc.TextPosAt(node.Range().Start), NewTextPos(2, 1))
	checkPos(t, "node end", doc.TextPosAt(node.Range().End), NewTextPos(2, 5))
}

// TestTextPos04 is the test text_pos_04 in roxmltree tests/api.rs.
func TestTextPos04(t *testing.T) {
	data := "<n1:e xmlns:n1='http://www.w3.org' n1:a=''/>"

	doc := mustParse(t, data)
	node := doc.RootElement()
	attr, _ := node.AttributeNode(ExpandedNameFromStr("a"))

	checkPos(t, "attr start", doc.TextPosAt(attr.Range().Start), NewTextPos(1, 36))
	checkPos(t, "attr end", doc.TextPosAt(attr.Range().End), NewTextPos(1, 43))
	checkPos(t, "qname start", doc.TextPosAt(attr.RangeQName().Start), NewTextPos(1, 36))
	checkPos(t, "qname end", doc.TextPosAt(attr.RangeQName().End), NewTextPos(1, 40))
	checkPos(t, "value start", doc.TextPosAt(attr.RangeValue().Start), NewTextPos(1, 42))
	checkPos(t, "value end", doc.TextPosAt(attr.RangeValue().End), NewTextPos(1, 42))
}

// TestTextPos05 is the test text_pos_05 in roxmltree tests/api.rs.
func TestTextPos05(t *testing.T) {
	data := "<n1:e xmlns:n1='http://www.w3.org' n1:a  =   'b'/>"

	doc := mustParse(t, data)
	node := doc.RootElement()
	attr, _ := node.AttributeNode(ExpandedNameFromStr("a"))

	checkPos(t, "attr start", doc.TextPosAt(attr.Range().Start), NewTextPos(1, 36))
	checkPos(t, "attr end", doc.TextPosAt(attr.Range().End), NewTextPos(1, 49))
	checkPos(t, "qname start", doc.TextPosAt(attr.RangeQName().Start), NewTextPos(1, 36))
	checkPos(t, "qname end", doc.TextPosAt(attr.RangeQName().End), NewTextPos(1, 40))
	checkPos(t, "value start", doc.TextPosAt(attr.RangeValue().Start), NewTextPos(1, 47))
	checkPos(t, "value end", doc.TextPosAt(attr.RangeValue().End), NewTextPos(1, 48))
}

// TestTextPos06 is the test text_pos_06 in roxmltree tests/api.rs.
func TestTextPos06(t *testing.T) {
	data := "<e a" + strings.Repeat(" ", 100) + "=" + strings.Repeat(" ", 160) + "'b'/>"

	doc := mustParse(t, data)
	node := doc.RootElement()
	attr, _ := node.AttributeNode(ExpandedNameFromStr("a"))

	checkPos(t, "attr start", doc.TextPosAt(attr.Range().Start), NewTextPos(1, 4))
	checkPos(t, "attr end", doc.TextPosAt(attr.Range().End), NewTextPos(1, 269))
	checkPos(t, "qname start", doc.TextPosAt(attr.RangeQName().Start), NewTextPos(1, 4))
	checkPos(t, "qname end", doc.TextPosAt(attr.RangeQName().End), NewTextPos(1, 5))
	attr.RangeValue() // unreliable since >254 spaces around equal sign, but still shouldn't panic
}

// TestNextSiblingElement01 is the test next_sibling_element_01 in roxmltree
// tests/api.rs.
func TestNextSiblingElement01(t *testing.T) {
	data := "<root><a/><b/><c/></root>"

	doc := mustParse(t, data)

	root := doc.RootElement()
	a, _ := root.FirstElementChild()
	b, _ := a.NextSiblingElement()
	c, _ := b.NextSiblingElement()
	if _, ok := c.NextSiblingElement(); ok {
		t.Error("c.NextSiblingElement() is not None")
	}

	for _, tc := range []struct {
		n    Node
		want string
	}{{root, "root"}, {a, "a"}, {b, "b"}, {c, "c"}} {
		if got := tc.n.TagName().Name(); got != tc.want {
			t.Errorf("TagName().Name() = %q, want %q", got, tc.want)
		}
	}
}

// TestNextPrevElement01 is the test next_prev_element_01 in roxmltree
// tests/api.rs.
func TestNextPrevElement01(t *testing.T) {
	data := "<root><a/><b/><c/></root>"

	doc := mustParse(t, data)

	root := doc.RootElement()
	c, _ := root.LastElementChild()
	b, _ := c.PrevSiblingElement()
	a, _ := b.PrevSiblingElement()
	if _, ok := a.PrevSiblingElement(); ok {
		t.Error("a.PrevSiblingElement() is not None")
	}

	for _, tc := range []struct {
		n    Node
		want string
	}{{root, "root"}, {a, "a"}, {b, "b"}, {c, "c"}} {
		if got := tc.n.TagName().Name(); got != tc.want {
			t.Errorf("TagName().Name() = %q, want %q", got, tc.want)
		}
	}
}

// TestNodesDocumentOrder is the test nodes_document_order in roxmltree
// tests/api.rs.
func TestNodesDocumentOrder(t *testing.T) {
	data := "<root><a/><b/><c/></root>"

	doc := mustParse(t, data)
	root := doc.RootElement()
	a, _ := root.FirstElementChild()
	b, _ := a.NextSiblingElement()
	c, _ := b.NextSiblingElement()

	elems := []Node{b, c, a}
	slices.SortFunc(elems, Node.Cmp)
	if elems[0] != a || elems[1] != b || elems[2] != c {
		t.Errorf("sorted = %v, want [a b c]", elems)
	}
}

// TestLifetimes is the test lifetimes in roxmltree tests/api.rs.
//
// Port: the test of upstream checks the lifetimes of the API at compile
// time. Go has no lifetimes, so the test calls each function.
func TestLifetimes(t *testing.T) {
	f := func(doc *Document, fun func(*Document)) {
		fun(doc)
	}

	doc := mustParse(t, "<e xmlns='http://www.w3.org'/>")

	f(doc, func(d *Document) { _ = d.Root() })
	f(doc, func(d *Document) { _ = d.Root().Document() })
	f(doc, func(d *Document) { _ = d.Root().TagName() })
	f(doc, func(d *Document) { _, _ = d.Root().TagName().Namespace() })
	f(doc, func(d *Document) { _ = d.Root().TagName().Name() })
	f(doc, func(d *Document) { _, _ = d.Root().DefaultNamespace() })
	f(doc, func(d *Document) { _, _ = d.Root().LookupPrefix("") })
	f(doc, func(d *Document) { _, _ = d.Root().LookupNamespaceURI("") })
	f(doc, func(d *Document) { _, _ = d.Root().Attribute(ExpandedNameFromStr("a")) })
	f(doc, func(d *Document) { _, _ = d.Root().AttributeNode(ExpandedNameFromStr("a")) })
	f(doc, func(d *Document) { _ = d.Root().Attributes() })
	f(doc, func(d *Document) { _ = d.Root().Namespaces() })
	f(doc, func(d *Document) { _, _ = d.Root().Text() })
	f(doc, func(d *Document) { _, _ = d.Root().Tail() })
	f(doc, func(d *Document) { _, _ = d.Root().PI() })
}

// TestTagNameLifetime is the test tag_name_lifetime in roxmltree
// tests/api.rs.
func TestTagNameLifetime(t *testing.T) {
	getTagName := func(node Node) string {
		return node.TagName().Name()
	}

	data := "<e xmlns='http://www.w3.org' />"
	doc := mustParse(t, data)
	root := doc.RootElement()
	if got := getTagName(root); got != "e" {
		t.Errorf("getTagName(root) = %q, want %q", got, "e")
	}
}

// TestEntityResolverWorks is the test entity_resolver_works in roxmltree
// tests/api.rs.
func TestEntityResolverWorks(t *testing.T) {
	text := `<!DOCTYPE foo [<!ENTITY bar SYSTEM "baz.xml">]> <qux>&bar;</qux>`

	entity := `<?xml version="1.0"?><foobar/>`

	entityResolver := func(_ string, _ bool, _ string) (string, bool, error) {
		return entity, true, nil
	}

	opts := DefaultParsingOptions()
	opts.AllowDTD = true
	opts.EntityResolver = entityResolver

	doc, err := ParseWithOptions(text, opts)
	if err != nil {
		t.Fatal(err)
	}

	child, ok := doc.RootElement().FirstChild()
	if !ok || !child.HasTagName(ExpandedNameFromStr("foobar")) {
		t.Errorf("first child = %v, want foobar", child)
	}
}
