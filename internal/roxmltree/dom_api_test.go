// Copyright 2018 Yevhenii Reizner
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package roxmltree

// This file ports roxmltree tests/dom-api.rs.

import (
	"testing"
)

// countNodes counts the nodes of seq for which f is true.
func countNodes(seq func(func(Node) bool), f func(Node) bool) int {
	n := 0
	for node := range seq {
		if f(node) {
			n++
		}
	}
	return n
}

// TestGetElementsByTagName is the test get_elements_by_tag_name in
// roxmltree tests/dom-api.rs, for Document.getElementsByTagName().
func TestGetElementsByTagName(t *testing.T) {
	data := `<!-- comment -->
<svg>
    <rect/>
    <text>Text</text>
    <g>
        <!-- comment -->
        <rect/>
    </g>
</svg>
`

	doc := mustParse(t, data)

	n := countNodes(doc.Descendants(), func(n Node) bool { return n.HasTagName(ExpandedNameFromStr("rect")) })
	if n != 2 {
		t.Errorf("len = %d, want 2", n)
	}
}

// TestGetElementsByTagNameNS is the test get_elements_by_tag_name_ns in
// roxmltree tests/dom-api.rs, for Document.getElementsByTagNameNS().
func TestGetElementsByTagNameNS(t *testing.T) {
	data := `<!-- comment -->
<svg xmlns:q='http://www.w3.org/'>
    <rect/>
    <text>Text</text>
    <g>
        <!-- comment -->
        <q:rect/>
    </g>
</svg>
`

	doc := mustParse(t, data)

	n := countNodes(doc.Descendants(), func(n Node) bool {
		return n.HasTagName(ExpandedNameFromTuple("http://www.w3.org/", "rect"))
	})
	if n != 1 {
		t.Errorf("len = %d, want 1", n)
	}
}

// TestChildElementCount is the test child_element_count in roxmltree
// tests/dom-api.rs, for ParentNode.childElementCount.
func TestChildElementCount(t *testing.T) {
	data := `<svg>
    <rect/>
    <!-- comment -->
    <rect/>
    <!-- comment -->
    <rect/>
</svg>
`

	doc := mustParse(t, data)
	svgElem := doc.RootElement()

	if count := countNodes(svgElem.Children(), Node.IsElement); count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

// TestChildren is the test children in roxmltree tests/dom-api.rs, for
// ParentNode.children.
func TestChildren(t *testing.T) {
	data := `<svg>
    <rect/>
    <!-- comment -->
    <rect/>
    <!-- comment -->
    <rect/>
</svg>
`

	doc := mustParse(t, data)
	svgElem := doc.RootElement()

	if count := countNodes(svgElem.Children(), Node.IsElement); count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

// TestFirstElementChild is the test first_element_child in roxmltree
// tests/dom-api.rs, for ParentNode.firstElementChild.
func TestFirstElementChild(t *testing.T) {
	data := `<svg>
    <!-- comment -->
    <rect/>
</svg>
`

	doc := mustParse(t, data)
	svgElem := doc.RootElement()

	elem, _ := svgElem.FirstElementChild()
	if !elem.HasTagName(ExpandedNameFromStr("rect")) {
		t.Errorf("FirstElementChild() = %v", elem)
	}

	// or

	var found Node
	for n := range svgElem.Children() {
		if n.IsElement() {
			found = n
			break
		}
	}
	if !found.HasTagName(ExpandedNameFromStr("rect")) {
		t.Errorf("first element of Children() = %v", found)
	}
}

// TestLastElementChild is the test last_element_child in roxmltree
// tests/dom-api.rs, for ParentNode.lastElementChild.
func TestLastElementChild(t *testing.T) {
	data := `<svg>
    <!-- comment -->
    <rect/>
    <!-- comment -->
</svg>
`

	doc := mustParse(t, data)
	svgElem := doc.RootElement()

	elem, _ := svgElem.LastElementChild()
	if !elem.HasTagName(ExpandedNameFromStr("rect")) {
		t.Errorf("LastElementChild() = %v", elem)
	}

	// or

	var elems []Node
	for n := range svgElem.Children() {
		if n.IsElement() {
			elems = append(elems, n)
		}
	}
	if last := elems[len(elems)-1]; !last.HasTagName(ExpandedNameFromStr("rect")) {
		t.Errorf("last element of Children() = %v", last)
	}
}

// TestGetElementByID is the test get_element_by_id in roxmltree
// tests/dom-api.rs, for Document.getElementById.
func TestGetElementByID(t *testing.T) {
	data := `<svg id='svg1'>
    <circle id='circle1'/>
    <g>
        <rect id='rect1'/>
    </g>
</svg>
`

	doc := mustParse(t, data)
	var elem Node
	for n := range doc.Descendants() {
		if v, ok := n.Attribute(ExpandedNameFromStr("id")); ok && v == "rect1" {
			elem = n
			break
		}
	}
	if !elem.HasTagName(ExpandedNameFromStr("rect")) {
		t.Errorf("element = %v", elem)
	}
}

// TestOwnerDocument is the test owner_document in roxmltree
// tests/dom-api.rs, for Node.ownerDocument.
func TestOwnerDocument(t *testing.T) {
	doc := mustParse(t, "<svg/>")
	_ = doc.RootElement()
}

// TestParentElement is the test parent_element in roxmltree
// tests/dom-api.rs, for Node.parentElement.
func TestParentElement(t *testing.T) {
	data := `<svg>
    <!-- comment -->
    <rect/>
</svg>
`

	doc := mustParse(t, data)
	var rect Node
	for n := range doc.Descendants() {
		if n.HasTagName(ExpandedNameFromStr("rect")) {
			rect = n
			break
		}
	}
	parent, _ := rect.ParentElement()
	if !parent.HasTagName(ExpandedNameFromStr("svg")) {
		t.Errorf("ParentElement() = %v", parent)
	}

	// or

	n, _ := findSkip1(rect.Ancestors(), Node.IsElement)
	if !n.HasTagName(ExpandedNameFromStr("svg")) {
		t.Errorf("ancestors().skip(1).find(is_element) = %v", n)
	}
}

// TestContains is the test contains in roxmltree tests/dom-api.rs, for
// Node.contains.
func TestContains(t *testing.T) {
	data := `<svg>
    <rect/>
</svg>
`

	doc := mustParse(t, data)
	svg := doc.RootElement()
	rect, _ := svg.FirstChild()

	if countNodes(svg.Descendants(), func(n Node) bool { return n == rect }) == 0 {
		t.Error("svg.Descendants() does not contain rect")
	}
}
