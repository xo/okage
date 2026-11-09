// Copyright 2019 the SimpleCSS Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package simplecss

import (
	"strings"
	"testing"
)

// testNode is an element of a small XML tree. Upstream tests with the tree
// of roxmltree. The port has its own tree, so that the tests of simplecss do
// not depend on internal/roxmltree. parseTestXML reads only the XML of these
// tests: elements, attributes in single quotes, and white space.
type testNode struct {
	name     string
	attrs    map[string]string
	parent   *testNode
	children []*testNode
}

// parseTestXML parses doc into a tree and returns its root element. It
// panics on XML that it does not read.
func parseTestXML(doc string) *testNode {
	var root, cur *testNode
	i := 0
	skip := func() {
		for i < len(doc) && strings.IndexByte(" \t\r\n", doc[i]) >= 0 {
			i++
		}
	}
	name := func() string {
		start := i
		for i < len(doc) && strings.IndexByte(" \t\r\n/>='", doc[i]) < 0 {
			i++
		}
		return doc[start:i]
	}
	for {
		skip()
		if i >= len(doc) {
			return root
		}
		if doc[i] != '<' {
			panic("text is not supported")
		}
		i++
		if doc[i] == '/' {
			i++
			name()
			i++ // '>'
			cur = cur.parent
			continue
		}
		n := &testNode{name: name(), attrs: map[string]string{}, parent: cur}
		if cur != nil {
			cur.children = append(cur.children, n)
		} else {
			root = n
		}
		for {
			skip()
			switch doc[i] {
			case '/':
				i += 2 // "/>"
			case '>':
				i++
				cur = n
			default:
				k := name()
				i += 2 // "='"
				end := strings.IndexByte(doc[i:], '\'')
				n.attrs[k] = doc[i : i+end]
				i += end + 1
				continue
			}
			break
		}
	}
}

// ParentElement is Element::parent_element of XmlNode in simplecss
// tests/select.rs.
func (n *testNode) ParentElement() (Element, bool) {
	if n.parent == nil {
		return nil, false
	}
	return n.parent, true
}

// PrevSiblingElement is Element::prev_sibling_element of XmlNode in
// simplecss tests/select.rs.
func (n *testNode) PrevSiblingElement() (Element, bool) {
	if n.parent == nil {
		return nil, false
	}
	var prev *testNode
	for _, c := range n.parent.children {
		if c == n {
			break
		}
		prev = c
	}
	if prev == nil {
		return nil, false
	}
	return prev, true
}

// HasLocalName is Element::has_local_name of XmlNode in simplecss
// tests/select.rs.
func (n *testNode) HasLocalName(localName string) bool {
	return n.name == localName
}

// AttributeMatches is Element::attribute_matches of XmlNode in simplecss
// tests/select.rs.
func (n *testNode) AttributeMatches(localName string, operator AttributeOperator) bool {
	value, ok := n.attrs[localName]
	if !ok {
		return false
	}
	return operator.Matches(value)
}

// PseudoClassMatches is Element::pseudo_class_matches of XmlNode in
// simplecss tests/select.rs.
func (n *testNode) PseudoClassMatches(class PseudoClass) bool {
	switch class.Kind {
	case PseudoClassFirstChild:
		_, ok := n.PrevSiblingElement()
		return !ok
	default:
		return false
	}
}

// descendants returns n and the elements under it, in document order.
func (n *testNode) descendants() []*testNode {
	nodes := []*testNode{n}
	for _, c := range n.children {
		nodes = append(nodes, c.descendants()...)
	}
	return nodes
}

// selectAll is XmlNode::select in simplecss tests/select.rs.
func (n *testNode) selectAll(text string) []*testNode {
	selectors, ok := ParseSelector(text)
	if !ok {
		panic("no selector")
	}
	var nodes []*testNode
	for _, node := range n.descendants() {
		if selectors.Matches(node) {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

// matchSingle is the macro match_single! of simplecss tests/select.rs.
func matchSingle(t *testing.T, doc *testNode, selector, want string) {
	t.Helper()
	nodes := doc.selectAll(selector)
	if len(nodes) != 1 {
		t.Fatalf("%q: got %d nodes, want 1", selector, len(nodes))
	}
	if got := nodes[0].attrs["id"]; got != want {
		t.Fatalf("%q: got %q, want %q", selector, got, want)
	}
}

// matchNone is the macro match_none! of simplecss tests/select.rs.
func matchNone(t *testing.T, doc *testNode, selector string) {
	t.Helper()
	if nodes := doc.selectAll(selector); len(nodes) != 0 {
		t.Fatalf("%q: got %d nodes, want 0", selector, len(nodes))
	}
}

// TestSelect ports the tests select_01 to select_30 of simplecss
// tests/select.rs. Each subtest has the name of the upstream test.
func TestSelect(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		test func(t *testing.T, doc *testNode)
	}{
		{"select_01", "<div id='div1'/>", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "*", "div1")
		}},
		{"select_02", "<div id='div1'/>", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "div", "div1")
			matchNone(t, doc, "p")
		}},
		{"select_03", "<div id='div1'/>", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "#div1", "div1")
			matchNone(t, doc, "#d1")
		}},
		{"select_04", "<div id='div1'/>", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchNone(t, doc, "p#div1")
		}},
		{"select_05", "<div id='div1'>\n    <p id='p1'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "div p", "p1")
		}},
		{"select_06", "<div id='div1'>\n    <g id='g1'>\n        <p id='p1'/>\n    </g>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "div p", "p1")
		}},
		{"select_07", "<div id='div1'>\n    <div id='div2'>\n        <g id='g1'>\n            <p id='p1'/>\n        </g>\n    </div>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "div p", "p1")
		}},
		{"select_08", "<div id='div1'>\n    <g id='g1'>\n        <p id='p1'>\n            <div/>\n        </p>\n    </g>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "div p", "p1")
		}},
		{"select_09", "<div id='div1'>\n    <g id='g1'>\n        <p id='p1'/>\n    </g>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "div g p", "p1")
		}},
		{"select_10", "<div id='div1'>\n    <q id='g1'>\n        <p id='p1'/>\n    </q>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchNone(t, doc, "div g p")
		}},
		{"select_11", "<div id='div1'>\n    <g id='g1'>\n        <p id='p1'/>\n    </g>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "div * p", "p1")
		}},
		{"select_12", "<div id='div1'>\n    <p id='p1'>\n        <rect id='rect1'/>\n        <rect id='rect2' color='green'/>\n    </p>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "div p *[color]", "rect2")
			matchSingle(t, doc, "div p [color]", "rect2")
		}},
		{"select_13", "<div id='div1'>\n    <p id='p1'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "div > p", "p1")
		}},
		{"select_14", "<p id='p1'/>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchNone(t, doc, "div > p")
		}},
		{"select_15", "<div id='div1'>\n    <g id='g1'>\n        <p id='p1'/>\n    </g>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchNone(t, doc, "div > p")
		}},
		{"select_16", "<div id='div1'>\n    <p>\n        <ol>\n            <li>\n                <g>\n                    <p id='p1'/>\n                </g>\n            </li>\n        </ol>\n    </p>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "div ol>li p", "p1")
		}},
		{"select_17", "<div id='div1'>\n    <p>\n        <ol>\n            <g>\n                <li>\n                    <g>\n                        <p id='p1'/>\n                    </g>\n                </li>\n            </g>\n        </ol>\n    </p>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchNone(t, doc, "div ol>li p")
		}},
		{"select_18", "<div id='div1'>\n    <g/>\n    <p id='p1'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "g + p", "p1")
		}},
		{"select_19", "<div id='div1'>\n    <test/>\n    <g/>\n    <p id='p1'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "g + p", "p1")
		}},
		{"select_20", "<div id='div1'>\n    <p id='p1'/>\n    <g/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchNone(t, doc, "g + p")
		}},
		{"select_21", "<div id='div1'>\n    <p id='p1'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchNone(t, doc, "div + p")
		}},
		{"select_22", "<div id='div1'>\n    <p id='p1'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "[id=p1]", "p1")
		}},
		{"select_23", "<div id='div1'>\n    <p id='p1' class='test warn'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "[class~=warn]", "p1")
		}},
		{"select_24", "<div id='div1'>\n    <p id='p1' class='test warn'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchNone(t, doc, "[class~='test warn']")
		}},
		{"select_25", "<div id='div1'>\n    <p id='p1' lang='en'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "[lang=en]", "p1")
			matchSingle(t, doc, "[lang|=en]", "p1")
		}},
		{"select_26", "<div id='div1'>\n    <p id='p1' lang='en-US'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "[lang='en-US']", "p1")
			matchSingle(t, doc, "[lang|=en]", "p1")
		}},
		{"select_27", "<div id='div1'>\n    <p id='p1' class='pastoral blue aqua marine'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, ".marine.pastoral", "p1")
		}},
		{"select_28", "<div id='div1'>\n    <p id='p1'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchSingle(t, doc, "p:first-child", "p1")
		}},
		{"select_29", "<div id='div1'>\n    <rect/>\n    <p id='p1'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			matchNone(t, doc, "p:first-child")
		}},
		{"select_30", "<div id='div1'>\n    <p id='p1'/>\n    <p id='p2'/>\n</div>\n", func(t *testing.T, doc *testNode) {
			t.Helper()
			nodes := doc.selectAll(":first-child")
			if len(nodes) != 2 || nodes[0].attrs["id"] != "div1" || nodes[1].attrs["id"] != "p1" {
				t.Fatalf("got %v", nodes)
			}
		}}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.test(t, parseTestXML(test.doc))
		})
	}
}

// TestToString ports the test to_string of simplecss tests/select.rs.
func TestToString(t *testing.T) {
	selectors, ok := ParseSelector("a > b")
	if !ok {
		t.Fatal("no selector")
	}
	if got := selectors.String(); got != "a > b" {
		t.Fatalf("got %q, want %q", got, "a > b")
	}
}
