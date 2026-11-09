// Copyright 2021 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"iter"
	"strings"

	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/parser/svgtree/mod.rs.
//
// Port: the module svgtree is private to the crate, so its types are
// unexported. A Rust iterator becomes an iter.Seq. The generic trait
// FromValue becomes the function type fromValue, and each impl of it a
// function fromValue<Type> in the order of upstream. The generic methods
// SvgNode::attribute, SvgNode::try_attribute and SvgNode::find_attribute
// become the generic functions nodeAttribute, nodeTryAttribute and
// nodeFindAttribute, which take the fromValue function of the type that the
// Rust code names. The methods attributeStr, attributeF32, attributeLength,
// attributeOpacity, attributeTransform and attributeNode are shorthands of
// nodeAttribute for the types that the parser reads most.
//
// A node does not hold a pointer to its data, as the field d of upstream
// does, because the Go slice of nodes can move while the parser appends to
// it. It finds its data by its id.

// document is Document in usvg src/parser/svgtree/mod.rs. It is an SVG tree
// container.
//
// Contains only element and text nodes.
// Text nodes are present only inside the `text` element.
//
// Port: opt is the options of the parse, for the logger (D20). Upstream logs
// with the crate log.
type document struct {
	nodes []nodeData
	attrs []attribute
	links map[string]nodeID
	opt   *options
}

// root is Document::root in usvg src/parser/svgtree/mod.rs. It returns the
// root node.
func (d *document) root() svgNode {
	return svgNode{id: newNodeID(0), doc: d}
}

// rootElement is Document::root_element in usvg src/parser/svgtree/mod.rs.
// It returns the root element.
func (d *document) rootElement() svgNode {
	// `unwrap` is safe, because `Document` is guarantee to have at least one element.
	n, ok := d.root().firstElementChild()
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	return n
}

// descendants is Document::descendants in usvg src/parser/svgtree/mod.rs.
// It returns an iterator over document's descendant nodes.
//
// Shorthand for `doc.root().descendants()`.
func (d *document) descendants() iter.Seq[svgNode] {
	return d.root().descendants()
}

// elementByID is Document::element_by_id in usvg src/parser/svgtree/mod.rs.
// It returns an element by ID.
//
// Unlike the [`Descendants`] iterator, this is just a HashMap lookup.
// Meaning it's way faster.
func (d *document) elementByID(id string) (svgNode, bool) {
	nodeID, ok := d.links[id]
	if !ok {
		return svgNode{}, false
	}
	return d.get(nodeID), true
}

// get is Document::get in usvg src/parser/svgtree/mod.rs.
func (d *document) get(id nodeID) svgNode {
	return svgNode{id: id, doc: d}
}

// String is the impl Debug for Document in usvg src/parser/svgtree/mod.rs.
//
// Port: a tag name and an attribute name print with their SVG names, where
// upstream prints the name of the Rust enum case.
func (d *document) String() string {
	if !d.root().hasChildren() {
		return "Document []"
	}

	var b strings.Builder
	writelnIndented := func(depth int, text string) {
		for range depth {
			b.WriteString("    ")
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}

	var printChildren func(parent svgNode, depth int)
	printChildren = func(parent svgNode, depth int) {
		for child := range parent.children() {
			if child.isElement() {
				writelnIndented(depth, "Element {")
				tag, _ := child.tagName()
				writelnIndented(depth, "    tag_name: "+tag.String())

				if len(child.attributes()) != 0 {
					writelnIndented(depth+1, "attributes: [")
					for _, attr := range child.attributes() {
						writelnIndented(depth+2, attr.String())
					}
					writelnIndented(depth+1, "]")
				}

				if child.hasChildren() {
					writelnIndented(depth, "    children: [")
					printChildren(child, depth+2)
					writelnIndented(depth, "    ]")
				}

				writelnIndented(depth, "}")
			} else {
				writelnIndented(depth, child.String())
			}
		}
	}

	b.WriteString("Document [\n")
	printChildren(d.root(), 1)
	b.WriteString("]\n")
	return b.String()
}

// shortRange is ShortRange in usvg src/parser/svgtree/mod.rs.
type shortRange struct {
	start uint32
	end   uint32
}

// newShortRange is ShortRange::new in usvg src/parser/svgtree/mod.rs.
func newShortRange(start, end uint32) shortRange {
	return shortRange{start: start, end: end}
}

// toURange is ShortRange::to_urange in usvg src/parser/svgtree/mod.rs.
func (r shortRange) toURange() (int, int) {
	return int(r.start), int(r.end)
}

// nodeID is NodeId in usvg src/parser/svgtree/mod.rs.
//
// Port: upstream stores the id plus one in a NonZeroU32, to make an
// Option<NodeId> small. nodeID stores the index itself, and optNodeID is the
// Option<NodeId>.
type nodeID uint32

// newNodeID is NodeId::new in usvg src/parser/svgtree/mod.rs.
func newNodeID(id uint32) nodeID {
	return nodeID(id)
}

// get is NodeId::get in usvg src/parser/svgtree/mod.rs.
func (id nodeID) get() uint32 {
	return uint32(id)
}

// getUsize is NodeId::get_usize in usvg src/parser/svgtree/mod.rs.
func (id nodeID) getUsize() int {
	return int(id.get())
}

// nodeIDFromUsize is From<usize> for NodeId in usvg
// src/parser/svgtree/mod.rs.
func nodeIDFromUsize(id int) nodeID {
	// We already checked that `id` is limited by u32::MAX.
	return newNodeID(uint32(id))
}

// optNodeID is an Option<NodeId> of upstream. It holds the id plus one, and
// zero is None, as the NonZeroU32 of upstream.
type optNodeID uint32

// someNodeID returns Some(id).
func someNodeID(id nodeID) optNodeID {
	return optNodeID(id + 1)
}

// get returns the id and true, or false for None.
func (o optNodeID) get() (nodeID, bool) {
	if o == 0 {
		return 0, false
	}
	return nodeID(o - 1), true
}

// nodeKindKind is the case of a nodeKind.
type nodeKindKind uint8

// The cases of the enum NodeKind in usvg src/parser/svgtree/mod.rs.
const (
	// nodeKindRoot is NodeKind::Root.
	nodeKindRoot nodeKindKind = iota
	// nodeKindElement is NodeKind::Element.
	nodeKindElement
	// nodeKindText is NodeKind::Text.
	nodeKindText
)

// nodeKind is NodeKind in usvg src/parser/svgtree/mod.rs.
//
// Port: each case holds a part of the same data, so the enum becomes a struct
// with a kind (D27).
type nodeKind struct {
	kind nodeKindKind
	// tagName is the field tag_name of Element.
	tagName EId
	// attributes is the field attributes of Element.
	attributes shortRange
	// text is the string of Text.
	text string
}

// nodeData is NodeData in usvg src/parser/svgtree/mod.rs.
type nodeData struct {
	parent      optNodeID
	nextSibling optNodeID
	// firstChild and lastChild are the field children of upstream, an
	// Option<(NodeId, NodeId)>. Both are None, or both are Some.
	firstChild optNodeID
	lastChild  optNodeID
	kind       nodeKind
}

// attribute is Attribute in usvg src/parser/svgtree/mod.rs. It is an
// attribute.
//
// Port: the value is a string, in place of the roxmltree::StringStorage of
// upstream.
type attribute struct {
	// Attribute's name.
	name AId
	// Attribute's value.
	value string
	// Attribute's importance
	important bool
}

// String is the impl Debug for Attribute in usvg src/parser/svgtree/mod.rs.
//
// Port: the name prints with its SVG name.
func (a attribute) String() string {
	important := "false" //nolint:goconst // an SVG keyword, as upstream writes it
	if a.important {
		important = "true" //nolint:goconst // an SVG keyword, as upstream writes it
	}
	return "Attribute { name: " + a.name.String() + ", value: " + a.value + ", important: " + important + " }"
}

// svgNode is SvgNode in usvg src/parser/svgtree/mod.rs. It is an SVG node.
//
// Port: two nodes are equal with ==, as the impl PartialEq of upstream says.
type svgNode struct {
	id  nodeID
	doc *document
}

// d is the field d of SvgNode in usvg src/parser/svgtree/mod.rs. It returns
// the data of the node.
func (n svgNode) d() *nodeData {
	return &n.doc.nodes[n.id.getUsize()]
}

// isElement is SvgNode::is_element in usvg src/parser/svgtree/mod.rs. It
// checks if the current node is an element.
func (n svgNode) isElement() bool {
	return n.d().kind.kind == nodeKindElement
}

// isText is SvgNode::is_text in usvg src/parser/svgtree/mod.rs. It checks if
// the current node is a text.
func (n svgNode) isText() bool {
	return n.d().kind.kind == nodeKindText
}

// document is SvgNode::document in usvg src/parser/svgtree/mod.rs. It returns
// node's document.
func (n svgNode) document() *document {
	return n.doc
}

// tagName is SvgNode::tag_name in usvg src/parser/svgtree/mod.rs. It returns
// element's tag name, unless the current node is text.
func (n svgNode) tagName() (EId, bool) {
	k := &n.d().kind
	if k.kind == nodeKindElement {
		return k.tagName, true
	}
	return 0, false
}

// hasTagName reports whether the node is an element with the tag name eid. It
// is the Rust comparison `node.tag_name() == Some(eid)`.
func (n svgNode) hasTagName(eid EId) bool {
	tag, ok := n.tagName()
	return ok && tag == eid
}

// elementID is SvgNode::element_id in usvg src/parser/svgtree/mod.rs. It
// returns element's `id` attribute value.
//
// Returns an empty string otherwise.
func (n svgNode) elementID() string {
	v, _ := n.attributeStr(AIdId)
	return v
}

// fromValue is the trait FromValue in usvg src/parser/svgtree/mod.rs. It is
// a function that parses an attribute value.
//
// When false is returned, the attribute value will be logged as a parsing
// failure.
type fromValue[T any] func(node svgNode, aid AId, value string) (T, bool)

// nodeAttribute is SvgNode::attribute in usvg src/parser/svgtree/mod.rs. It
// returns an attribute value, parsed by parse.
func nodeAttribute[T any](n svgNode, aid AId, parse fromValue[T]) (T, bool) {
	var zero T
	value, ok := n.rawAttribute(aid)
	if !ok {
		return zero, false
	}
	// These AId have an initial value of none
	isPossibleNone := false
	switch aid {
	case AIdMask, AIdMarkerStart, AIdMarkerMid, AIdMarkerEnd, AIdClipPath, AIdFilter,
		AIdFontSizeAdjust, AIdTextDecoration, AIdStroke, AIdStrokeDasharray:
		isPossibleNone = true
	}
	if isPossibleNone && value == "none" { //nolint:goconst // an SVG keyword, as upstream writes it
		return zero, false
	}
	v, ok := parse(n, aid, value)
	if !ok {
		// TODO: show position in XML
		n.doc.opt.warnf("Failed to parse %s value: '%s'.", aid, value)
		return zero, false
	}
	return v, true
}

// rawAttribute returns the value of the attribute aid as it is stored. It is
// the lookup that SvgNode::attribute and SvgNode::try_attribute in usvg
// src/parser/svgtree/mod.rs share.
func (n svgNode) rawAttribute(aid AId) (string, bool) {
	for _, a := range n.attributes() {
		if a.name == aid {
			return a.value, true
		}
	}
	return "", false
}

// attributeStr is SvgNode::attribute::<&str> in usvg
// src/parser/svgtree/mod.rs.
func (n svgNode) attributeStr(aid AId) (string, bool) {
	return nodeAttribute(n, aid, fromValueStr)
}

// attributeF32 is SvgNode::attribute::<f32> in usvg
// src/parser/svgtree/mod.rs.
func (n svgNode) attributeF32(aid AId) (float32, bool) {
	return nodeAttribute(n, aid, fromValueF32)
}

// attributeLength is SvgNode::attribute::<svgtypes::Length> in usvg
// src/parser/svgtree/mod.rs.
func (n svgNode) attributeLength(aid AId) (svgtypes.Length, bool) {
	return nodeAttribute(n, aid, fromValueLength)
}

// attributeOpacity is SvgNode::attribute::<Opacity> in usvg
// src/parser/svgtree/mod.rs.
func (n svgNode) attributeOpacity(aid AId) (Opacity, bool) {
	return nodeAttribute(n, aid, fromValueOpacity)
}

// attributeTransform is SvgNode::attribute::<Transform> in usvg
// src/parser/svgtree/mod.rs.
func (n svgNode) attributeTransform(aid AId) (Transform, bool) {
	return nodeAttribute(n, aid, fromValueTransform)
}

// attributeNode is SvgNode::attribute::<SvgNode> in usvg
// src/parser/svgtree/mod.rs.
func (n svgNode) attributeNode(aid AId) (svgNode, bool) {
	return nodeAttribute(n, aid, fromValueSvgNode)
}

// nodeTryAttribute is SvgNode::try_attribute in usvg
// src/parser/svgtree/mod.rs. It returns an attribute value.
//
// Same as `SvgNode::attribute`, but doesn't show a warning.
func nodeTryAttribute[T any](n svgNode, aid AId, parse fromValue[T]) (T, bool) {
	value, ok := n.rawAttribute(aid)
	if !ok {
		var zero T
		return zero, false
	}
	return parse(n, aid, value)
}

// nodeAttribute is SvgNode::node_attribute in usvg
// src/parser/svgtree/mod.rs. It returns the element that the IRI or the
// FuncIRI of the attribute aid links to.
//
// Port: the generic function nodeAttribute is SvgNode::attribute, a
// different item. A method and a function do not collide in Go.
func (n svgNode) nodeAttribute(aid AId) (svgNode, bool) {
	value, ok := n.attributeStr(aid)
	if !ok {
		return svgNode{}, false
	}
	var id string
	if aid == AIdHref {
		v, err := svgtypes.ParseIRI(value)
		if err != nil {
			return svgNode{}, false
		}
		id = string(v)
	} else {
		v, err := svgtypes.ParseFuncIRI(value)
		if err != nil {
			return svgNode{}, false
		}
		id = string(v)
	}

	return n.document().elementByID(id)
}

// hasAttribute is SvgNode::has_attribute in usvg src/parser/svgtree/mod.rs.
// It checks if an attribute is present.
func (n svgNode) hasAttribute(aid AId) bool {
	_, ok := n.rawAttribute(aid)
	return ok
}

// attributes is SvgNode::attributes in usvg src/parser/svgtree/mod.rs. It
// returns a list of all element's attributes.
func (n svgNode) attributes() []attribute {
	k := &n.d().kind
	if k.kind == nodeKindElement {
		start, end := k.attributes.toURange()
		return n.doc.attrs[start:end]
	}
	return nil
}

// attributeID is SvgNode::attribute_id in usvg src/parser/svgtree/mod.rs.
func (n svgNode) attributeID(aid AId) (int, bool) {
	k := &n.d().kind
	if k.kind != nodeKindElement {
		return 0, false
	}
	for idx, attr := range n.attributes() {
		if attr.name == aid {
			return int(k.attributes.start) + idx, true
		}
	}
	return 0, false
}

// nodeFindAttribute is SvgNode::find_attribute in usvg
// src/parser/svgtree/mod.rs. It finds a [`Node`] that contains the required
// attribute, and returns the value parsed by parse.
//
// For inheritable attributes walks over ancestors until a node with
// the specified attribute is found.
//
// For non-inheritable attributes checks only the current node and the parent one.
// As per SVG spec.
func nodeFindAttribute[T any](n svgNode, aid AId, parse fromValue[T]) (T, bool) {
	node, ok := n.findAttributeImpl(aid)
	if !ok {
		var zero T
		return zero, false
	}
	return nodeAttribute(node, aid, parse)
}

// findAttributeImpl is SvgNode::find_attribute_impl in usvg
// src/parser/svgtree/mod.rs.
func (n svgNode) findAttributeImpl(aid AId) (svgNode, bool) {
	if aid.isInheritable() {
		for a := range n.ancestors() {
			if a.hasAttribute(aid) {
				return a, true
			}
		}

		return svgNode{}, false
	}

	if n.hasAttribute(aid) {
		return n, true
	}
	// Non-inheritable attributes can inherit a value only from a direct parent.
	p, ok := n.parentElement()
	if !ok {
		return svgNode{}, false
	}
	if p.hasAttribute(aid) {
		return p, true
	}
	return svgNode{}, false
}

// text is SvgNode::text in usvg src/parser/svgtree/mod.rs. It returns node's
// text data.
//
// For text nodes returns its content. For elements returns the first child node text.
func (n svgNode) text() string {
	k := &n.d().kind
	switch k.kind {
	case nodeKindElement:
		child, ok := n.firstChild()
		if ok && child.isText() {
			return child.d().kind.text
		}
		return ""
	case nodeKindText:
		return k.text
	}
	return ""
}

// parent is SvgNode::parent in usvg src/parser/svgtree/mod.rs. It returns a
// parent node.
func (n svgNode) parent() (svgNode, bool) {
	id, ok := n.d().parent.get()
	if !ok {
		return svgNode{}, false
	}
	return n.doc.get(id), true
}

// parentElement is SvgNode::parent_element in usvg
// src/parser/svgtree/mod.rs. It returns the parent element.
func (n svgNode) parentElement() (svgNode, bool) {
	first := true
	for a := range n.ancestors() {
		if first {
			first = false
			continue
		}
		if a.isElement() {
			return a, true
		}
	}
	return svgNode{}, false
}

// nextSibling is SvgNode::next_sibling in usvg src/parser/svgtree/mod.rs. It
// returns the next sibling.
func (n svgNode) nextSibling() (svgNode, bool) {
	id, ok := n.d().nextSibling.get()
	if !ok {
		return svgNode{}, false
	}
	return n.doc.get(id), true
}

// firstChild is SvgNode::first_child in usvg src/parser/svgtree/mod.rs. It
// returns the first child.
func (n svgNode) firstChild() (svgNode, bool) {
	id, ok := n.d().firstChild.get()
	if !ok {
		return svgNode{}, false
	}
	return n.doc.get(id), true
}

// firstElementChild is SvgNode::first_element_child in usvg
// src/parser/svgtree/mod.rs. It returns the first child element.
func (n svgNode) firstElementChild() (svgNode, bool) {
	for c := range n.children() {
		if c.isElement() {
			return c, true
		}
	}
	return svgNode{}, false
}

// lastChild is SvgNode::last_child in usvg src/parser/svgtree/mod.rs. It
// returns the last child.
func (n svgNode) lastChild() (svgNode, bool) {
	id, ok := n.d().lastChild.get()
	if !ok {
		return svgNode{}, false
	}
	return n.doc.get(id), true
}

// hasChildren is SvgNode::has_children in usvg src/parser/svgtree/mod.rs. It
// checks if the node has child nodes.
func (n svgNode) hasChildren() bool {
	return n.d().firstChild != 0
}

// ancestors is SvgNode::ancestors in usvg src/parser/svgtree/mod.rs. It
// returns an iterator over ancestor nodes starting at this node.
//
// Port: the iterator Ancestors of upstream is the function that the
// iter.Seq calls.
func (n svgNode) ancestors() iter.Seq[svgNode] {
	return func(yield func(svgNode) bool) {
		node, ok := n, true
		for ok {
			if !yield(node) {
				return
			}
			node, ok = node.parent()
		}
	}
}

// children is SvgNode::children in usvg src/parser/svgtree/mod.rs. It
// returns an iterator over children nodes.
//
// Port: the iterator Children of upstream is the function that the iter.Seq
// calls.
func (n svgNode) children() iter.Seq[svgNode] {
	return func(yield func(svgNode) bool) {
		front, hasFront := n.firstChild()
		back, hasBack := n.lastChild()
		for hasFront {
			node := front
			if hasBack && front == back {
				hasFront, hasBack = false, false
			} else {
				front, hasFront = node.nextSibling()
			}
			if !yield(node) {
				return
			}
		}
	}
}

// traverse is SvgNode::traverse in usvg src/parser/svgtree/mod.rs. It
// returns an iterator which traverses the subtree starting at this node.
func (n svgNode) traverse() *traverseIter {
	return &traverseIter{root: n}
}

// descendants is SvgNode::descendants in usvg src/parser/svgtree/mod.rs. It
// returns an iterator over this node and its descendants.
//
// Port: the iterator Descendants of upstream is the function that the
// iter.Seq calls.
func (n svgNode) descendants() iter.Seq[svgNode] {
	return func(yield func(svgNode) bool) {
		t := n.traverse()
		for {
			edge, ok := t.next()
			if !ok {
				return
			}
			if edge.kind == edgeOpen {
				if !yield(edge.node) {
					return
				}
			}
		}
	}
}

// hrefIter is SvgNode::href_iter in usvg src/parser/svgtree/mod.rs. It
// returns an iterator over elements linked via `xlink:href`.
//
// Port: the iterator HrefIter of upstream is the function that the iter.Seq
// calls.
func (n svgNode) hrefIter() iter.Seq[svgNode] {
	return func(yield func(svgNode) bool) {
		doc := n.document()
		origin := n.id
		curr := n.id

		if !yield(doc.get(curr)) {
			return
		}

		for {
			link, ok := doc.get(curr).nodeAttribute(AIdHref)
			if !ok {
				return
			}
			if link.id == curr || link.id == origin {
				doc.opt.warnf(
					"Element '#%s' cannot reference itself via 'xlink:href'.",
					doc.get(origin).elementID(),
				)
				return
			}

			curr = link.id
			if !yield(doc.get(curr)) {
				return
			}
		}
	}
}

// String is the impl Debug for SvgNode in usvg src/parser/svgtree/mod.rs.
//
// Port: a tag name and an attribute name print with their SVG names, and the
// text prints in Go syntax.
func (n svgNode) String() string {
	k := &n.d().kind
	switch k.kind {
	case nodeKindRoot:
		return "Root"
	case nodeKindElement:
		var b strings.Builder
		b.WriteString("Element { tag_name: Some(" + k.tagName.String() + "), attributes: [")
		for i, a := range n.attributes() {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(a.String())
		}
		b.WriteString("] }")
		return b.String()
	}
	return "Text(\"" + k.text + "\")"
}

// edgeKind is the case of an edge.
type edgeKind uint8

// The cases of the enum Edge in usvg src/parser/svgtree/mod.rs.
const (
	// edgeOpen is Edge::Open.
	edgeOpen edgeKind = iota
	// edgeClose is Edge::Close.
	edgeClose
)

// edge is Edge in usvg src/parser/svgtree/mod.rs.
//
// Port: each case holds a node, so the enum becomes a struct with a kind
// (D27).
type edge struct {
	kind edgeKind
	node svgNode
}

// traverseIter is Traverse in usvg src/parser/svgtree/mod.rs.
type traverseIter struct {
	root    svgNode
	edge    edge
	hasEdge bool
}

// next is the impl Iterator for Traverse in usvg src/parser/svgtree/mod.rs.
func (t *traverseIter) next() (edge, bool) {
	switch {
	case t.hasEdge && t.edge.kind == edgeOpen:
		node := t.edge.node
		if firstChild, ok := node.firstChild(); ok {
			t.edge = edge{kind: edgeOpen, node: firstChild}
		} else {
			t.edge = edge{kind: edgeClose, node: node}
		}
	case t.hasEdge && t.edge.kind == edgeClose:
		node := t.edge.node
		if node == t.root {
			t.hasEdge = false
		} else if nextSibling, ok := node.nextSibling(); ok {
			t.edge = edge{kind: edgeOpen, node: nextSibling}
		} else if p, ok := node.parent(); ok {
			t.edge = edge{kind: edgeClose, node: p}
		} else {
			t.hasEdge = false
		}
	default:
		t.edge = edge{kind: edgeOpen, node: t.root}
		t.hasEdge = true
	}

	return t.edge, t.hasEdge
}

// IsGraphic is EId::is_graphic in usvg src/parser/svgtree/mod.rs. It checks
// if this is a
// [graphics element](https://www.w3.org/TR/SVG11/intro.html#TermGraphicsElement).
func (id EId) IsGraphic() bool {
	switch id {
	case EIdCircle, EIdEllipse, EIdImage, EIdLine, EIdPath, EIdPolygon,
		EIdPolyline, EIdRect, EIdText, EIdUse:
		return true
	}
	return false
}

// IsGradient is EId::is_gradient in usvg src/parser/svgtree/mod.rs. It checks
// if this is a
// [gradient element](https://www.w3.org/TR/SVG11/intro.html#TermGradientElement).
func (id EId) IsGradient() bool {
	return id == EIdLinearGradient || id == EIdRadialGradient
}

// IsPaintServer is EId::is_paint_server in usvg src/parser/svgtree/mod.rs.
// It checks if this is a
// [paint server element](https://www.w3.org/TR/SVG11/intro.html#TermPaint).
func (id EId) IsPaintServer() bool {
	return id == EIdLinearGradient || id == EIdRadialGradient || id == EIdPattern
}

// isPresentation is AId::is_presentation in usvg src/parser/svgtree/mod.rs.
func (id AId) isPresentation() bool {
	switch id {
	case AIdAlignmentBaseline,
		AIdBaselineShift,
		AIdBackgroundColor, // non-standard SVG attribute
		AIdClipPath,
		AIdClipRule,
		AIdColor,
		AIdColorInterpolation,
		AIdColorInterpolationFilters,
		AIdColorRendering,
		AIdDirection,
		AIdDisplay,
		AIdDominantBaseline,
		AIdFill,
		AIdFillOpacity,
		AIdFillRule,
		AIdFilter,
		AIdFloodColor,
		AIdFloodOpacity,
		AIdFontFamily,
		AIdFontKerning,       // technically not presentation
		AIdFontOpticalSizing, // technically not presentation
		AIdFontSize,
		AIdFontSizeAdjust,
		AIdFontStretch,
		AIdFontStyle,
		AIdFontVariant,
		AIdFontWeight,
		AIdFontVariationSettings,
		AIdGlyphOrientationHorizontal,
		AIdGlyphOrientationVertical,
		AIdImageRendering,
		AIdIsolation, // technically not presentation
		AIdLetterSpacing,
		AIdLightingColor,
		AIdMarkerEnd,
		AIdMarkerMid,
		AIdMarkerStart,
		AIdMask,
		AIdMaskType,
		AIdMixBlendMode, // technically not presentation
		AIdOpacity,
		AIdOverflow,
		AIdPaintOrder,
		AIdShapeRendering,
		AIdStopColor,
		AIdStopOpacity,
		AIdStroke,
		AIdStrokeDasharray,
		AIdStrokeDashoffset,
		AIdStrokeLinecap,
		AIdStrokeLinejoin,
		AIdStrokeMiterlimit,
		AIdStrokeOpacity,
		AIdStrokeWidth,
		AIdTextAnchor,
		AIdTextDecoration,
		AIdTextOverflow,
		AIdTextRendering,
		AIdTransform,
		AIdTransformOrigin,
		AIdUnicodeBidi,
		AIdVectorEffect,
		AIdVisibility,
		AIdWhiteSpace,
		AIdWordSpacing,
		AIdWritingMode:
		return true
	}
	return false
}

// isInheritable is AId::is_inheritable in usvg src/parser/svgtree/mod.rs. It
// checks if the current attribute is inheritable.
func (id AId) isInheritable() bool {
	if id.isPresentation() {
		return !isNonInheritable(id)
	}
	return false
}

// allowsInheritValue is AId::allows_inherit_value in usvg
// src/parser/svgtree/mod.rs.
func (id AId) allowsInheritValue() bool {
	switch id {
	case AIdAlignmentBaseline,
		AIdBaselineShift,
		AIdClipPath,
		AIdClipRule,
		AIdColor,
		AIdColorInterpolationFilters,
		AIdDirection,
		AIdDisplay,
		AIdDominantBaseline,
		AIdFill,
		AIdFillOpacity,
		AIdFillRule,
		AIdFilter,
		AIdFloodColor,
		AIdFloodOpacity,
		AIdFontFamily,
		AIdFontKerning,
		AIdFontOpticalSizing,
		AIdFontSize,
		AIdFontStretch,
		AIdFontStyle,
		AIdFontVariant,
		AIdFontWeight,
		AIdImageRendering,
		AIdKerning,
		AIdLetterSpacing,
		AIdMarkerEnd,
		AIdMarkerMid,
		AIdMarkerStart,
		AIdMask,
		AIdOpacity,
		AIdOverflow,
		AIdShapeRendering,
		AIdStopColor,
		AIdStopOpacity,
		AIdStroke,
		AIdStrokeDasharray,
		AIdStrokeDashoffset,
		AIdStrokeLinecap,
		AIdStrokeLinejoin,
		AIdStrokeMiterlimit,
		AIdStrokeOpacity,
		AIdStrokeWidth,
		AIdTextAnchor,
		AIdTextDecoration,
		AIdTextRendering,
		AIdVisibility,
		AIdWordSpacing,
		AIdWritingMode:
		return true
	}
	return false
}

// isNonInheritable is is_non_inheritable in usvg src/parser/svgtree/mod.rs.
func isNonInheritable(id AId) bool {
	switch id {
	case AIdAlignmentBaseline,
		AIdBaselineShift,
		AIdClipPath,
		AIdDisplay,
		AIdFilter,
		AIdFloodColor,
		AIdFloodOpacity,
		AIdMask,
		AIdOpacity,
		AIdOverflow,
		AIdLightingColor,
		AIdStopColor,
		AIdStopOpacity,
		AIdTextDecoration,
		AIdTransform,
		AIdTransformOrigin:
		return true
	}
	return false
}

// fromValueStr is the impl FromValue for &str in usvg
// src/parser/svgtree/mod.rs.
func fromValueStr(_ svgNode, _ AId, value string) (string, bool) {
	return value, true
}

// fromValueF32 is the impl FromValue for f32 in usvg
// src/parser/svgtree/mod.rs.
func fromValueF32(_ svgNode, _ AId, value string) (float32, bool) {
	v, err := svgtypes.ParseNumber(value)
	if err != nil {
		return 0, false
	}
	return float32(v), true
}

// fromValueLength is the impl FromValue for svgtypes::Length in usvg
// src/parser/svgtree/mod.rs.
func fromValueLength(_ svgNode, _ AId, value string) (svgtypes.Length, bool) {
	v, err := svgtypes.ParseLength(value)
	return v, err == nil
}

// fromValueOpacity is the impl FromValue for Opacity in usvg
// src/parser/svgtree/mod.rs.
//
// TODO: to svgtypes?
func fromValueOpacity(_ svgNode, _ AId, value string) (Opacity, bool) {
	length, err := svgtypes.ParseLength(value)
	if err != nil {
		return 0, false
	}
	switch length.Unit {
	case svgtypes.LengthUnitPercent:
		return path.NormalizedF32Clamped(float32(length.Number) / 100.0), true
	case svgtypes.LengthUnitNone:
		return path.NormalizedF32Clamped(float32(length.Number)), true
	}
	return 0, false
}

// fromValueTransform is the impl FromValue for Transform in usvg
// src/parser/svgtree/mod.rs.
func fromValueTransform(_ svgNode, _ AId, value string) (Transform, bool) {
	ts, err := svgtypes.ParseTransform(value)
	if err != nil {
		return Transform{}, false
	}

	t := path.TransformFromRow(
		float32(ts.A),
		float32(ts.B),
		float32(ts.C),
		float32(ts.D),
		float32(ts.E),
		float32(ts.F),
	)

	if t.IsValid() {
		return t, true
	}
	return path.DefaultTransform(), true
}

// fromValueTransformOrigin is the impl FromValue for
// svgtypes::TransformOrigin in usvg src/parser/svgtree/mod.rs.
func fromValueTransformOrigin(_ svgNode, _ AId, value string) (svgtypes.TransformOrigin, bool) {
	v, err := svgtypes.ParseTransformOrigin(value)
	return v, err == nil
}

// fromValueViewBox is the impl FromValue for svgtypes::ViewBox in usvg
// src/parser/svgtree/mod.rs.
func fromValueViewBox(_ svgNode, _ AId, value string) (svgtypes.ViewBox, bool) {
	v, err := svgtypes.ParseViewBox(value)
	return v, err == nil
}

// fromValueUnits is the impl FromValue for Units in usvg
// src/parser/svgtree/mod.rs.
func fromValueUnits(_ svgNode, _ AId, value string) (units, bool) {
	switch value {
	case "userSpaceOnUse": //nolint:goconst // an SVG keyword, as upstream writes it
		return unitsUserSpaceOnUse, true
	case "objectBoundingBox":
		return unitsObjectBoundingBox, true
	}
	return 0, false
}

// fromValueAspectRatio is the impl FromValue for svgtypes::AspectRatio in
// usvg src/parser/svgtree/mod.rs.
func fromValueAspectRatio(_ svgNode, _ AId, value string) (svgtypes.AspectRatio, bool) {
	v, err := svgtypes.ParseAspectRatio(value)
	return v, err == nil
}

// fromValuePaintOrder is the impl FromValue for svgtypes::PaintOrder in usvg
// src/parser/svgtree/mod.rs.
func fromValuePaintOrder(_ svgNode, _ AId, value string) (svgtypes.PaintOrder, bool) {
	return svgtypes.ParsePaintOrder(value), true
}

// fromValueColor is the impl FromValue for svgtypes::Color in usvg
// src/parser/svgtree/mod.rs.
func fromValueColor(_ svgNode, _ AId, value string) (svgtypes.Color, bool) {
	v, err := svgtypes.ParseColor(value)
	return v, err == nil
}

// fromValueAngle is the impl FromValue for svgtypes::Angle in usvg
// src/parser/svgtree/mod.rs.
func fromValueAngle(_ svgNode, _ AId, value string) (svgtypes.Angle, bool) {
	v, err := svgtypes.ParseAngle(value)
	return v, err == nil
}

// fromValueEnableBackground is the impl FromValue for
// svgtypes::EnableBackground in usvg src/parser/svgtree/mod.rs.
func fromValueEnableBackground(_ svgNode, _ AId, value string) (svgtypes.EnableBackground, bool) {
	v, err := svgtypes.ParseEnableBackground(value)
	return v, err == nil
}

// fromValuePaint is the impl FromValue for svgtypes::Paint in usvg
// src/parser/svgtree/mod.rs.
func fromValuePaint(_ svgNode, _ AId, value string) (svgtypes.Paint, bool) {
	v, err := svgtypes.ParsePaint(value)
	return v, err == nil
}

// fromValueNumberList is the impl FromValue for Vec<f32> in usvg
// src/parser/svgtree/mod.rs.
func fromValueNumberList(_ svgNode, _ AId, value string) ([]float32, bool) {
	var list []float32
	for n, err := range svgtypes.NewNumberListParser(value).All() {
		if err != nil {
			return nil, false
		}
		list = append(list, float32(n))
	}

	return list, true
}

// fromValueLengthList is the impl FromValue for Vec<svgtypes::Length> in
// usvg src/parser/svgtree/mod.rs.
func fromValueLengthList(_ svgNode, _ AId, value string) ([]svgtypes.Length, bool) {
	var list []svgtypes.Length
	for n, err := range svgtypes.NewLengthListParser(value).All() {
		if err != nil {
			return nil, false
		}
		list = append(list, n)
	}

	return list, true
}

// fromValueVisibility is the impl FromValue for Visibility in usvg
// src/parser/svgtree/mod.rs.
func fromValueVisibility(_ svgNode, _ AId, value string) (visibility, bool) {
	switch value {
	case "visible": //nolint:goconst // an SVG keyword, as upstream writes it
		return visibilityVisible, true
	case "hidden":
		return visibilityHidden, true
	case "collapse":
		return visibilityCollapse, true
	}
	return 0, false
}

// fromValueSpreadMethod is the impl FromValue for SpreadMethod in usvg
// src/parser/svgtree/mod.rs.
func fromValueSpreadMethod(_ svgNode, _ AId, value string) (SpreadMethod, bool) {
	switch value {
	case "pad":
		return SpreadMethodPad, true
	case "reflect":
		return SpreadMethodReflect, true
	case "repeat":
		return SpreadMethodRepeat, true
	}
	return 0, false
}

// fromValueShapeRendering is the impl FromValue for ShapeRendering in usvg
// src/parser/svgtree/mod.rs.
func fromValueShapeRendering(_ svgNode, _ AId, value string) (ShapeRendering, bool) {
	switch value {
	case "optimizeSpeed": //nolint:goconst // an SVG keyword, as upstream writes it
		return ShapeRenderingOptimizeSpeed, true
	case "crispEdges": //nolint:goconst // an SVG keyword, as upstream writes it
		return ShapeRenderingCrispEdges, true
	case "auto", "geometricPrecision": //nolint:goconst // an SVG keyword, as upstream writes it
		return ShapeRenderingGeometricPrecision, true
	}
	return 0, false
}

// fromValueTextRendering is the impl FromValue for TextRendering in usvg
// src/parser/svgtree/mod.rs.
func fromValueTextRendering(_ svgNode, _ AId, value string) (TextRendering, bool) {
	switch value {
	case "optimizeSpeed":
		return TextRenderingOptimizeSpeed, true
	case "auto", "optimizeLegibility": //nolint:goconst // an SVG keyword, as upstream writes it
		return TextRenderingOptimizeLegibility, true
	case "geometricPrecision":
		return TextRenderingGeometricPrecision, true
	}
	return 0, false
}

// fromValueImageRendering is the impl FromValue for ImageRendering in usvg
// src/parser/svgtree/mod.rs.
func fromValueImageRendering(_ svgNode, _ AId, value string) (ImageRendering, bool) {
	switch value {
	case "auto", "optimizeQuality": //nolint:goconst // an SVG keyword, as upstream writes it
		return ImageRenderingOptimizeQuality, true
	case "optimizeSpeed":
		return ImageRenderingOptimizeSpeed, true
	case "smooth": //nolint:goconst // an SVG keyword, as upstream writes it
		return ImageRenderingSmooth, true
	case "high-quality": //nolint:goconst // an SVG keyword, as upstream writes it
		return ImageRenderingHighQuality, true
	case "crisp-edges": //nolint:goconst // an SVG keyword, as upstream writes it
		return ImageRenderingCrispEdges, true
	case "pixelated": //nolint:goconst // an SVG keyword, as upstream writes it
		return ImageRenderingPixelated, true
	}
	return 0, false
}

// fromValueBlendMode is the impl FromValue for BlendMode in usvg
// src/parser/svgtree/mod.rs.
func fromValueBlendMode(_ svgNode, aid AId, value string) (BlendMode, bool) {
	switch value {
	case "normal": //nolint:goconst // an SVG keyword, as upstream writes it
		return BlendModeNormal, true
	case "multiply":
		return BlendModeMultiply, true
	case "screen":
		return BlendModeScreen, true
	case "overlay":
		return BlendModeOverlay, true
	case "darken":
		return BlendModeDarken, true
	case "lighten":
		return BlendModeLighten, true
	case "color-dodge":
		return BlendModeColorDodge, true
	case "color-burn":
		return BlendModeColorBurn, true
	case "hard-light":
		return BlendModeHardLight, true
	case "soft-light":
		return BlendModeSoftLight, true
	case "difference":
		return BlendModeDifference, true
	case "exclusion":
		return BlendModeExclusion, true
	case "hue":
		return BlendModeHue, true
	case "saturation":
		return BlendModeSaturation, true
	case "color": //nolint:goconst // an SVG keyword, as upstream writes it
		return BlendModeColor, true
	case "luminosity":
		return BlendModeLuminosity, true
	case "plus-lighter":
		if aid == AIdMixBlendMode {
			return BlendModePlusLighter, true
		}
	}
	return 0, false
}

// fromValueSvgNode is the impl FromValue for SvgNode in usvg
// src/parser/svgtree/mod.rs.
func fromValueSvgNode(node svgNode, aid AId, value string) (svgNode, bool) {
	var id string
	if aid == AIdHref {
		v, err := svgtypes.ParseIRI(value)
		if err != nil {
			return svgNode{}, false
		}
		id = string(v)
	} else {
		v, err := svgtypes.ParseFuncIRI(value)
		if err != nil {
			return svgNode{}, false
		}
		id = string(v)
	}

	return node.document().elementByID(id)
}
