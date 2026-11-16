// Copyright (c) 2006-2013 James Graham and other contributors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package html

// This file ports tinyhtml5 treebuilder.py (D49). The class Element of
// upstream wraps an element of ElementTree, which holds text as the text
// and the tail of elements. The port builds the tree of document.go
// instead, with a node for each run of text, and each method of Element
// does what its doc comment says on that tree.

import "slices"

// scopeVariant is a variant of element_in_scope in tinyhtml5
// treebuilder.py, a key of _list_elements.
type scopeVariant uint8

// The variants of element_in_scope.
const (
	scopeDefault scopeVariant = iota
	scopeButton
	scopeList
	scopeTable
	scopeSelect
)

// listElement is _list_elements in tinyhtml5 treebuilder.py. It returns if
// the element is in the list of the variant, and if the variant inverts the
// test.
func listElement(variant scopeVariant, n *Node) (in, invert bool) {
	ns, name := n.namespace, n.name
	switch variant {
	case scopeButton:
		return scopingElement(ns, name) || ns == NamespaceHTML && name == "button", false
	case scopeList:
		return scopingElement(ns, name) || ns == NamespaceHTML && (name == "ol" || name == "ul"), false
	case scopeTable:
		return ns == NamespaceHTML && (name == "html" || name == "table"), false
	case scopeSelect:
		return ns == NamespaceHTML && (name == "optgroup" || name == "option"), true
	}
	return scopingElement(ns, name), false
}

// impliedEndTag is _implied_end_tags in tinyhtml5 treebuilder.py. XXX td, th
// and tr are not actually needed.
func impliedEndTag(name string) bool {
	switch name {
	case "dd", "dt", "li", "option", "optgroup", "p", "rp", "rt":
		return true
	}
	return false
}

// attributesEqual is the comparison of the dicts of the attributes of two
// elements in tinyhtml5 treebuilder.py and parser.py. The order does not
// matter.
func attributesEqual(a, b []Attribute) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) <= smallAttrs {
		for _, x := range a {
			i := attrIndex(b, x.Namespace, x.Name)
			if i < 0 || b[i].Value != x.Value {
				return false
			}
		}
		return true
	}
	type key struct{ ns, name string }
	m := make(map[key]string, len(b))
	for _, x := range b {
		m[key{x.Namespace, x.Name}] = x.Value
	}
	for _, x := range a {
		v, ok := m[key{x.Namespace, x.Name}]
		if !ok || v != x.Value {
			return false
		}
	}
	return true
}

// appendChild is Element.append_child in tinyhtml5 treebuilder.py. It
// inserts node as the last child of parent.
func appendChild(parent, node *Node) {
	node.parent = parent
	node.prevSibling = parent.lastChild
	node.nextSibling = nil
	if parent.lastChild != nil {
		parent.lastChild.nextSibling = node
	} else {
		parent.firstChild = node
	}
	parent.lastChild = node
}

// insertBefore is Element.insert_before in tinyhtml5 treebuilder.py. It
// inserts node as a child of parent, before reference.
func insertBefore(parent, node, reference *Node) {
	node.parent = parent
	node.nextSibling = reference
	node.prevSibling = reference.prevSibling
	if reference.prevSibling != nil {
		reference.prevSibling.nextSibling = node
	} else {
		parent.firstChild = node
	}
	reference.prevSibling = node
}

// removeChild is Element.remove_child in tinyhtml5 treebuilder.py. It
// removes node from the children of parent.
func removeChild(parent, node *Node) {
	if node.prevSibling != nil {
		node.prevSibling.nextSibling = node.nextSibling
	} else {
		parent.firstChild = node.nextSibling
	}
	if node.nextSibling != nil {
		node.nextSibling.prevSibling = node.prevSibling
	} else {
		parent.lastChild = node.prevSibling
	}
	node.parent, node.prevSibling, node.nextSibling = nil, nil, nil
}

// hasContent is Element.has_content in tinyhtml5 treebuilder.py. It returns
// true if the node has children or text.
func hasContent(n *Node) bool {
	return n.firstChild != nil
}

// treeBuilder is TreeBuilder in tinyhtml5 treebuilder.py.
type treeBuilder struct {
	defaultNamespace string

	openElements []*Node

	// activeFormattingElements holds a nil Node for each scope marker. The
	// scope markers are inserted when entering object elements, marquees,
	// table cells, and table captions, and are used to prevent formatting
	// from "leaking" into tables, object elements, and marquees.
	activeFormattingElements []*Node

	headElement *Node
	formElement *Node

	insertFromTable bool

	document *Node

	// nodes counts the nodes that the tree builder made, for WithMaxNodes.
	nodes int
}

// newTreeBuilder is TreeBuilder.__init__ in tinyhtml5 treebuilder.py, with
// namespace_html_elements true.
func newTreeBuilder() *treeBuilder {
	tb := &treeBuilder{defaultNamespace: NamespaceHTML}
	tb.reset()
	return tb
}

// reset is TreeBuilder.reset in tinyhtml5 treebuilder.py.
func (tb *treeBuilder) reset() {
	tb.openElements = nil
	tb.activeFormattingElements = nil

	tb.headElement = nil
	tb.formElement = nil

	tb.insertFromTable = false

	tb.document = &Node{typ: DocumentNode}
	tb.nodes = 0
}

// currentNode returns the last open element.
func (tb *treeBuilder) currentNode() *Node {
	return tb.openElements[len(tb.openElements)-1]
}

// pop removes the last open element and returns it.
func (tb *treeBuilder) pop() *Node {
	n := tb.openElements[len(tb.openElements)-1]
	tb.openElements = tb.openElements[:len(tb.openElements)-1]
	return n
}

// removeOpenElement removes a node from the open elements.
func (tb *treeBuilder) removeOpenElement(n *Node) {
	if i := slices.Index(tb.openElements, n); i >= 0 {
		tb.openElements = slices.Delete(tb.openElements, i, i+1)
	}
}

// removeActiveFormattingElement removes a node from the active formatting
// elements.
func (tb *treeBuilder) removeActiveFormattingElement(n *Node) {
	if i := slices.Index(tb.activeFormattingElements, n); i >= 0 {
		tb.activeFormattingElements = slices.Delete(tb.activeFormattingElements, i, i+1)
	}
}

// appendActiveFormattingElement is ActiveFormattingElements.append in
// tinyhtml5 treebuilder.py. It appends a node, or a marker for nil, to the
// end of the list.
func (tb *treeBuilder) appendActiveFormattingElement(node *Node) {
	equalCount := 0
	if node != nil {
		for i, element := range slices.Backward(tb.activeFormattingElements) {
			if element == nil {
				break
			}
			nodesEqual := element.namespace == node.namespace && element.name == node.name &&
				attributesEqual(element.attrs, node.attrs)
			if nodesEqual {
				equalCount++
			}
			if equalCount == 3 {
				tb.activeFormattingElements = slices.Delete(tb.activeFormattingElements, i, i+1)
				break
			}
		}
	}
	tb.activeFormattingElements = append(tb.activeFormattingElements, node)
}

// elementInScope is TreeBuilder.element_in_scope in tinyhtml5
// treebuilder.py, for a name of HTML.
func (tb *treeBuilder) elementInScope(name string, variant scopeVariant) bool {
	return tb.inScope(nil, name, variant)
}

// nodeInScope is TreeBuilder.element_in_scope in tinyhtml5 treebuilder.py,
// for a node.
func (tb *treeBuilder) nodeInScope(target *Node, variant scopeVariant) bool {
	return tb.inScope(target, "", variant)
}

// inScope is TreeBuilder.element_in_scope in tinyhtml5 treebuilder.py. If we
// pass a node in we match that. If we pass a string match any node with that
// name.
func (tb *treeBuilder) inScope(target *Node, name string, variant scopeVariant) bool {
	for _, node := range slices.Backward(tb.openElements) {
		if target != nil && node == target {
			return true
		}
		if target == nil && node.namespace == NamespaceHTML && node.name == name {
			return true
		}
		if in, invert := listElement(variant, node); invert != in {
			return false
		}
	}
	// We should never reach this point. Upstream raises ValueError.
	return false
}

// reconstructActiveFormattingElements is
// TreeBuilder.reconstruct_active_formatting_elements in tinyhtml5
// treebuilder.py.
func (tb *treeBuilder) reconstructActiveFormattingElements() {
	// Within this algorithm the order of steps described in the
	// specification is not quite the same as the order of steps in the
	// code. It should still do the same though.

	// Step 1: stop the algorithm when there's nothing to do.
	afe := tb.activeFormattingElements
	if len(afe) == 0 {
		return
	}

	// Step 2 and step 3: we start with the last element. So i is -1.
	i := len(afe) - 1
	entry := afe[i]
	if entry == nil || slices.Contains(tb.openElements, entry) {
		return
	}

	// Step 6.
	for entry != nil && !slices.Contains(tb.openElements, entry) {
		if i == 0 {
			// This will be reset to 0 below.
			i = -1
			break
		}
		i--
		// Step 5: let entry be one earlier in the list.
		entry = afe[i]
	}

	for {
		// Step 7.
		i++

		// Step 8.
		entry = tb.activeFormattingElements[i]
		clone := tb.clone(entry) // mainly to get a new copy of the attributes

		// Step 9.
		element := tb.insertElement(&token{
			typ:       tokenStartTag,
			name:      clone.name,
			namespace: clone.namespace,
			attrs:     clone.attrs,
		})

		// Step 10.
		tb.activeFormattingElements[i] = element

		// Step 11.
		if element == tb.activeFormattingElements[len(tb.activeFormattingElements)-1] {
			break
		}
	}
}

// clearActiveFormattingElements is
// TreeBuilder.clear_active_formatting_elements in tinyhtml5 treebuilder.py.
func (tb *treeBuilder) clearActiveFormattingElements() {
	afe := tb.activeFormattingElements
	entry := afe[len(afe)-1]
	afe = afe[:len(afe)-1]
	for len(afe) > 0 && entry != nil {
		entry = afe[len(afe)-1]
		afe = afe[:len(afe)-1]
	}
	tb.activeFormattingElements = afe
}

// elementInActiveFormattingElements is
// TreeBuilder.element_in_active_formatting_elements in tinyhtml5
// treebuilder.py. It finds name between the end of the active formatting
// elements and the last marker. If an element with this name exists, it
// returns it. Else it returns nil.
func (tb *treeBuilder) elementInActiveFormattingElements(name string) *Node {
	for _, item := range slices.Backward(tb.activeFormattingElements) {
		// Check for Marker first because if it's a Marker it doesn't have a
		// name attribute.
		if item == nil {
			break
		}
		if item.name == name {
			return item
		}
	}
	return nil
}

// insertRoot is TreeBuilder.insert_root in tinyhtml5 treebuilder.py.
func (tb *treeBuilder) insertRoot(tok *token) {
	element := tb.createElement(tok)
	tb.openElements = append(tb.openElements, element)
	appendChild(tb.document, element)
}

// insertDoctype is TreeBuilder.insert_doctype in tinyhtml5 treebuilder.py.
func (tb *treeBuilder) insertDoctype(tok *token) {
	tb.nodes++
	appendChild(tb.document, &Node{
		typ:      DoctypeNode,
		name:     tok.name,
		publicID: tok.publicID,
		systemID: tok.systemID,
	})
}

// insertComment is TreeBuilder.insert_comment in tinyhtml5 treebuilder.py.
func (tb *treeBuilder) insertComment(tok *token, parent *Node) {
	tb.nodes++
	appendChild(parent, &Node{typ: CommentNode, data: tok.data})
}

// createElement is TreeBuilder.create_element in tinyhtml5 treebuilder.py.
// It creates an element but doesn't insert it anywhere.
func (tb *treeBuilder) createElement(tok *token) *Node {
	namespace := tok.namespace
	if namespace == "" {
		namespace = tb.defaultNamespace
	}
	tb.nodes++
	return &Node{typ: ElementNode, name: tok.name, namespace: namespace, attrs: slices.Clone(tok.attrs)}
}

// clone is Element.clone in tinyhtml5 treebuilder.py. It returns a shallow
// copy of the node: the same name and attributes, but no parent or
// children.
func (tb *treeBuilder) clone(n *Node) *Node {
	tb.nodes++
	return &Node{typ: n.typ, name: n.name, namespace: n.namespace, attrs: slices.Clone(n.attrs)}
}

// insertElement is TreeBuilder.insert_element in tinyhtml5 treebuilder.py,
// which insert_from_table switches.
func (tb *treeBuilder) insertElement(tok *token) *Node {
	if tb.insertFromTable {
		return tb.insertElementTable(tok)
	}
	return tb.insertElementNormal(tok)
}

// insertElementNormal is TreeBuilder.insert_element_normal in tinyhtml5
// treebuilder.py.
func (tb *treeBuilder) insertElementNormal(tok *token) *Node {
	element := tb.createElement(tok)
	appendChild(tb.currentNode(), element)
	tb.openElements = append(tb.openElements, element)
	return element
}

// insertElementTable is TreeBuilder.insert_element_table in tinyhtml5
// treebuilder.py. It creates an element and inserts it into the tree.
func (tb *treeBuilder) insertElementTable(tok *token) *Node {
	if !tableInsertModeElement(tb.currentNode().name) {
		return tb.insertElementNormal(tok)
	}
	element := tb.createElement(tok)
	// We should be in the InTable mode. This means we want to do
	// special magic element rearranging.
	parent, before := tb.getTableMisnestedNodePosition()
	if before == nil {
		appendChild(parent, element)
	} else {
		insertBefore(parent, element, before)
	}
	tb.openElements = append(tb.openElements, element)
	return element
}

// insertText is TreeBuilder.insert_text in tinyhtml5 treebuilder.py. It
// inserts text data into the current node. Upstream also takes a parent,
// which no caller gives.
func (tb *treeBuilder) insertText(data string) {
	parent := tb.currentNode()

	inTable := tb.insertFromTable && tableInsertModeElement(tb.currentNode().name)
	if inTable {
		// We should be in the InTable mode. This means we want to do
		// special magic element rearranging.
		parent, before := tb.getTableMisnestedNodePosition()
		tb.insertTextIn(parent, data, before)
	} else {
		tb.insertTextIn(parent, data, nil)
	}
}

// insertTextIn is Element.insert_text in tinyhtml5 treebuilder.py. It
// inserts data as text in parent, before the node before, or at the
// end when before is nil. Text next to text joins it.
func (tb *treeBuilder) insertTextIn(parent *Node, data string, before *Node) {
	if data == "" {
		return
	}
	prev := parent.lastChild
	if before != nil {
		prev = before.prevSibling
	}
	if prev != nil && prev.typ == TextNode {
		prev.text = append(prev.text, data...)
		return
	}
	tb.nodes++
	text := &Node{typ: TextNode, text: []byte(data)}
	if before != nil {
		insertBefore(parent, text, before)
	} else {
		appendChild(parent, text)
	}
}

// reparentChildren is Element.reparent_children in tinyhtml5
// treebuilder.py. It moves all the children of the node n to parent.
func reparentChildren(n, parent *Node) {
	for c := n.firstChild; c != nil; {
		next := c.nextSibling
		removeChild(n, c)
		if last := parent.lastChild; c.typ == TextNode && last != nil && last.typ == TextNode {
			last.text = append(last.text, c.text...)
		} else {
			appendChild(parent, c)
		}
		c = next
	}
}

// getTableMisnestedNodePosition is
// TreeBuilder.get_table_misnested_node_position in tinyhtml5 treebuilder.py.
// It gets the foster parent element and the sibling, or nil, to insert
// before.
func (tb *treeBuilder) getTableMisnestedNodePosition() (*Node, *Node) {
	// The foster parent element is the one which comes before the most
	// recently opened table element.

	// XXX - this is really inelegant.
	var lastTable, fosterParent, before *Node
	lastIndex := -1
	for i, element := range slices.Backward(tb.openElements) {
		if element.name == "table" {
			lastTable, lastIndex = element, i
			break
		}
	}
	switch {
	case lastTable != nil && lastTable.parent != nil:
		// XXX - we should really check that this parent is actually a
		// node here.
		fosterParent = lastTable.parent
		before = lastTable
	case lastTable != nil:
		// A table at the bottom of the stack gives the index -1, which
		// Python reads as the last element.
		index := lastIndex - 1
		if index < 0 {
			index = len(tb.openElements) - 1
		}
		fosterParent = tb.openElements[index]
	default:
		fosterParent = tb.openElements[0]
	}
	return fosterParent, before
}

// generateImpliedEndTags is TreeBuilder.generate_implied_end_tags in
// tinyhtml5 treebuilder.py. Upstream calls itself again, and the port loops.
func (tb *treeBuilder) generateImpliedEndTags(exclude string) {
	for len(tb.openElements) > 0 {
		name := tb.currentNode().name
		if !impliedEndTag(name) || name == exclude {
			return
		}
		tb.pop()
		// XXX This is not entirely what the specification says. We should
		// investigate it more closely.
	}
}

// getFragment is TreeBuilder.get_fragment in tinyhtml5 treebuilder.py. It
// returns the final fragment.
func (tb *treeBuilder) getFragment() *Node {
	fragment := &Node{typ: DocumentNode}
	reparentChildren(tb.openElements[0], fragment)
	return fragment
}
