// Copyright 2016 the SimpleCSS Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package simplecss_test

import (
	"fmt"

	"github.com/xo/okage/internal/simplecss"
	"github.com/xo/okage/roxmltree"
)

// This example ports the doc test of DeclarationTokenizer in simplecss
// src/lib.rs.
func ExampleDeclarationTokenizer() {
	t := simplecss.NewDeclarationTokenizer("background: url(\"img.png\"); color:red !important")
	for d := range t.All() {
		fmt.Printf("%s: %s important=%v\n", d.Name, d.Value, d.Important)
	}
	// Output:
	// background: url("img.png") important=false
	// color: red important=true
}

// This example ports the doc test of SelectorTokenizer in simplecss
// src/selector.rs.
func ExampleSelectorTokenizer() {
	t := simplecss.NewSelectorTokenizer("div > p:first-child")
	for tok, err := range t.All() {
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("%v(%q)\n", tok.Kind, tok.Value)
	}
	// Output:
	// TypeSelector("div")
	// ChildCombinator("")
	// TypeSelector("p")
	// PseudoClass("first-child")
}

// This example parses a style sheet. The rules come out sorted by
// specificity.
func ExampleParseStyleSheet() {
	style := simplecss.ParseStyleSheet("#a { fill:red } rect, .b { stroke:blue !important } @media print { g {} }")
	fmt.Println(style)
	// Output:
	// rect { stroke:blue !important; }
	// *[class~='b'] { stroke:blue !important; }
	// *[id='a'] { fill:red; }
}

// xmlNode is XmlNode in simplecss examples/select.rs. It is an Element on a
// node of roxmltree.
type xmlNode struct {
	node roxmltree.Node
}

// selectNode is XmlNode::select in simplecss examples/select.rs. It returns
// the first element in the subtree of n, n included, that matches text.
func (n xmlNode) selectNode(text string) (roxmltree.Node, bool) {
	selector, ok := simplecss.ParseSelector(text)
	if !ok {
		return roxmltree.Node{}, false
	}
	for node := range n.node.Descendants() {
		if node.IsElement() && selector.Matches(xmlNode{node}) {
			return node, true
		}
	}
	return roxmltree.Node{}, false
}

// ParentElement is Element::parent_element for XmlNode in simplecss
// examples/select.rs.
func (n xmlNode) ParentElement() (simplecss.Element, bool) {
	parent, ok := n.node.ParentElement()
	if !ok {
		return nil, false
	}
	return xmlNode{parent}, true
}

// PrevSiblingElement is Element::prev_sibling_element for XmlNode in
// simplecss examples/select.rs.
//
// Port: upstream takes the first element of prev_siblings, and roxmltree
// starts that iterator at the node itself. So the method returns the node
// itself, and :first-child matches no element. The port keeps that fault.
func (n xmlNode) PrevSiblingElement() (simplecss.Element, bool) {
	for node := range n.node.PrevSiblings() {
		if node.IsElement() {
			return xmlNode{node}, true
		}
	}
	return nil, false
}

// HasLocalName is Element::has_local_name for XmlNode in simplecss
// examples/select.rs.
func (n xmlNode) HasLocalName(name string) bool {
	return n.node.TagName().Name() == name
}

// AttributeMatches is Element::attribute_matches for XmlNode in simplecss
// examples/select.rs.
func (n xmlNode) AttributeMatches(localName string, operator simplecss.AttributeOperator) bool {
	value, ok := n.node.Attribute(roxmltree.ExpandedNameFromStr(localName))
	if !ok {
		return false
	}
	return operator.Matches(value)
}

// PseudoClassMatches is Element::pseudo_class_matches for XmlNode in
// simplecss examples/select.rs. The XML is static, so it ignores the other
// pseudo-classes.
func (n xmlNode) PseudoClassMatches(class simplecss.PseudoClass) bool {
	if class.Kind == simplecss.PseudoClassFirstChild {
		_, ok := n.PrevSiblingElement()
		return !ok
	}
	return false
}

// This example ports simplecss examples/select.rs. It matches selectors
// against a tree of roxmltree.
//
// Port: upstream asserts the id of each element that it finds. Its first
// assertion panics, because of the fault in PrevSiblingElement. The example
// prints the id of each element instead, so that its output shows the
// result of upstream.
func Example_select() {
	doc, err := roxmltree.Parse(`<svg>
            <g>
                <rect id='rect1' class='round blue'/>
                <rect id='rect2' color='red'/>
            </g>
        </svg>`)
	if err != nil {
		fmt.Println(err)
		return
	}
	root := xmlNode{doc.RootElement()}
	for _, text := range []string{"rect:first-child", "[color=red]", "svg rect", "svg > g > rect", ".blue"} {
		node, ok := root.selectNode(text)
		if !ok {
			fmt.Printf("%s: none\n", text)
			continue
		}
		id, _ := node.Attribute(roxmltree.ExpandedNameFromStr("id"))
		fmt.Printf("%s: %s\n", text, id)
	}
	// Output:
	// rect:first-child: none
	// [color=red]: rect2
	// svg rect: rect1
	// svg > g > rect: rect1
	// .blue: rect1
}
