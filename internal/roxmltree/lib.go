// Copyright 2018 Yevhenii Reizner
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package roxmltree

import (
	"cmp"
	"iter"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
)

// NSXMLURI is NS_XML_URI in roxmltree src/lib.rs, the
// <http://www.w3.org/XML/1998/namespace> URI.
const NSXMLURI = "http://www.w3.org/XML/1998/namespace"

// nsXMLPrefix is NS_XML_PREFIX in roxmltree src/lib.rs. The prefix 'xml',
// which is by definition bound to NS_XML_URI.
const nsXMLPrefix = "xml"

// NSXMLNSURI is NS_XMLNS_URI in roxmltree src/lib.rs, the
// <http://www.w3.org/2000/xmlns/> URI.
const NSXMLNSURI = "http://www.w3.org/2000/xmlns/"

// xmlns is XMLNS in roxmltree src/lib.rs. The string 'xmlns', which is used
// to declare new namespaces.
const xmlns = "xmlns"

// TextPos is TextPos in roxmltree src/lib.rs. It is a position in text.
//
// Position indicates a row/line and a column in the original text. Starting from 1:1.
type TextPos struct {
	Row uint32
	Col uint32
}

// NewTextPos is TextPos::new in roxmltree src/lib.rs. It constructs a new
// `TextPos`.
func NewTextPos(row, col uint32) TextPos {
	return TextPos{Row: row, Col: col}
}

// String is fmt::Display for TextPos in roxmltree src/lib.rs.
func (p TextPos) String() string {
	return strconv.FormatUint(uint64(p.Row), 10) + ":" + strconv.FormatUint(uint64(p.Col), 10)
}

// Range is core::ops::Range<usize> of the Rust core library: the byte
// offsets from Start to End, with End excluded.
type Range struct {
	Start int
	End   int
}

// documentSerial gives each Document a number, so that Node.Cmp can order
// the nodes of two documents. Upstream compares the addresses of the
// documents.
var documentSerial atomic.Uint64

// Document is Document in roxmltree src/lib.rs. It is an XML tree container.
//
// A tree consists of [`Nodes`].
// There are no separate structs for each node type.
// So you should check the current node type yourself via [`Node::node_type()`].
// There are only [5 types](enum.NodeType.html):
// Root, Element, PI, Comment and Text.
//
// As you can see there are no XML declaration and CDATA types.
// The XML declaration is basically skipped, since it doesn't contain any
// valuable information (we support only UTF-8 anyway).
// And CDATA will be converted into a Text node as is, without
// any preprocessing (you can read more about it
// [here](https://github.com/RazrFalcon/roxmltree/blob/master/docs/parsing.md)).
//
// Also, the Text node data can be accessed from the text node itself or from
// the parent element via [`Node::text()`] or [`Node::tail()`].
type Document struct {
	// An original data.
	//
	// Required for `text_pos_at` methods.
	text       string
	nodes      []nodeData
	attributes []attributeData
	namespaces namespaces
	// serial is the port of the address of the document (see
	// documentSerial).
	serial uint64
}

// Root is Document::root in roxmltree src/lib.rs. It returns the root node.
func (d *Document) Root() Node {
	return Node{
		id:  NewNodeID(0),
		d:   &d.nodes[0],
		doc: d,
	}
}

// GetNode is Document::get_node in roxmltree src/lib.rs. It returns the node
// of the tree with the given NodeID.
//
// Note: NewNodeID(0) represents the root node.
func (d *Document) GetNode(id NodeID) (Node, bool) {
	i := id.GetUsize()
	if i >= len(d.nodes) {
		return Node{}, false
	}
	return Node{
		id:  id,
		d:   &d.nodes[i],
		doc: d,
	}, true
}

// getNodeUnwrap is Document::get_node(id).unwrap() in roxmltree src/lib.rs.
// It panics when the node does not exist, as the unwrap of upstream does.
func (d *Document) getNodeUnwrap(id NodeID) Node {
	n, ok := d.GetNode(id)
	if !ok {
		panic("roxmltree: called `Option::unwrap()` on a `None` value")
	}
	return n
}

// RootElement is Document::root_element in roxmltree src/lib.rs. It returns
// the root element of the document.
//
// Unlike `root`, will return a first element node.
//
// The root element always exists.
func (d *Document) RootElement() Node {
	// `expect` is safe, because the `Document` is guarantee to have at least one element.
	n, ok := d.Root().FirstElementChild()
	if !ok {
		panic("XML documents must contain a root element")
	}
	return n
}

// Descendants is Document::descendants in roxmltree src/lib.rs. It returns an
// iterator over document's descendant nodes.
//
// Shorthand for `doc.root().descendants()`.
func (d *Document) Descendants() iter.Seq[Node] {
	return d.Root().Descendants()
}

// TextPosAt is Document::text_pos_at in roxmltree src/lib.rs. It calculates
// `TextPos` in the original document from position in bytes.
//
// **Note:** this operation is expensive.
func (d *Document) TextPosAt(pos int) TextPos {
	s := newStream(d.text)
	return s.GenTextPosFrom(pos)
}

// InputText is Document::input_text in roxmltree src/lib.rs. It returns the
// input text of the original document.
func (d *Document) InputText() string {
	return d.text
}

// String is fmt::Debug for Document in roxmltree src/lib.rs.
//
// Port: an attribute value and a namespace URI print as a quoted string.
// Upstream prints the StringStorage around it, such as Borrowed("b").
func (d *Document) String() string {
	if !d.Root().HasChildren() {
		return "Document []"
	}

	var f strings.Builder

	writelnIndented := func(depth int, s string) {
		for range depth {
			f.WriteString("    ")
		}
		f.WriteString(s)
		f.WriteByte('\n')
	}

	printIntoIter := func(prefix string, data []string, depth int) {
		if len(data) == 0 {
			return
		}

		writelnIndented(depth, prefix+": [")
		for _, v := range data {
			writelnIndented(depth+1, v)
		}
		writelnIndented(depth, "]")
	}

	var printChildren func(parent Node, depth int)
	printChildren = func(parent Node, depth int) {
		for child := range parent.Children() {
			if child.IsElement() {
				writelnIndented(depth, "Element {")
				writelnIndented(depth, "    tag_name: "+child.TagName().String())
				var attrs []string
				for a := range child.Attributes() {
					attrs = append(attrs, a.String())
				}
				printIntoIter("attributes", attrs, depth+1)
				var nss []string
				for ns := range child.Namespaces() {
					nss = append(nss, ns.String())
				}
				printIntoIter("namespaces", nss, depth+1)

				if child.HasChildren() {
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

	f.WriteString("Document [\n")
	printChildren(d.Root(), 1)
	f.WriteString("]\n")

	return f.String()
}

// NodeType is NodeType in roxmltree src/lib.rs. It is a list of supported
// node types.
type NodeType uint8

// The node types.
const (
	// NodeTypeRoot is the root node of the `Document`.
	NodeTypeRoot NodeType = iota
	// NodeTypeElement is an element node.
	//
	// Only an element can have a tag name and attributes.
	NodeTypeElement
	// NodeTypePI is a processing instruction.
	NodeTypePI
	// NodeTypeComment is a comment node.
	NodeTypeComment
	// NodeTypeText is a text node.
	NodeTypeText
)

// String is fmt::Debug for NodeType in roxmltree src/lib.rs.
func (t NodeType) String() string {
	switch t {
	case NodeTypeRoot:
		return "Root"
	case NodeTypeElement:
		return "Element"
	case NodeTypePI:
		return "PI"
	case NodeTypeComment:
		return "Comment"
	case NodeTypeText:
		return "Text"
	}
	return "NodeType(" + strconv.Itoa(int(t)) + ")"
}

// PI is PI in roxmltree src/lib.rs. It is a processing instruction.
//
// Port: Value is the Rust Option<&str>, and an empty Value is None. The
// tokenizer never gives Some(""), so no value is lost.
type PI struct {
	Target string
	Value  string
}

// shortRange is ShortRange in roxmltree src/lib.rs. It is a short range.
//
// Just like Range, but only for `u32` and copyable.
type shortRange struct {
	start uint32
	end   uint32
}

// shortRangeFrom is From<Range<usize>> for ShortRange in roxmltree
// src/lib.rs.
func shortRangeFrom(start, end int) shortRange {
	return newShortRange(uint32(start), uint32(end))
}

// newShortRange is ShortRange::new in roxmltree src/lib.rs.
func newShortRange(start, end uint32) shortRange {
	return shortRange{start: start, end: end}
}

// toURange is ShortRange::to_urange in roxmltree src/lib.rs.
func (r shortRange) toURange() (int, int) {
	return int(r.start), int(r.end)
}

// NodeID is NodeId in roxmltree src/lib.rs. It is a node ID stored as
// `u32`.
//
// An index into a `Tree`-internal `Vec`.
//
// Note that this value should be used with care since `roxmltree` doesn't
// check that `NodeId` actually belongs to a selected `Document`.
// So you can end up in a situation, when `NodeId` produced by one `Document`
// is used to select a node in another `Document`.
//
// Port: upstream stores id+1 in a NonZeroU32, so that Option<NodeId> is
// small. NodeID holds the id itself, and optNodeID holds id+1 for the
// fields of a node.
type NodeID uint32

// NewNodeID is NodeId::new in roxmltree src/lib.rs. It constructs a new
// `NodeId` from a `u32`.
func NewNodeID(id uint32) NodeID {
	// We are using `NonZeroU32` to reduce overhead of `Option<NodeId>`.
	if id+1 == 0 {
		panic("roxmltree: called `Option::unwrap()` on a `None` value")
	}
	return NodeID(id)
}

// Get is NodeId::get in roxmltree src/lib.rs. It returns the `u32`
// representation of the `NodeId`.
func (id NodeID) Get() uint32 {
	return uint32(id)
}

// GetUsize is NodeId::get_usize in roxmltree src/lib.rs. It returns the
// `usize` representation of the `NodeId`.
func (id NodeID) GetUsize() int {
	return int(id.Get())
}

// nodeIDFromUsize is From<usize> for NodeId in roxmltree src/lib.rs.
func nodeIDFromUsize(id int) NodeID {
	// We already checked that `id` is limited by u32::MAX.
	return NewNodeID(uint32(id))
}

// optNodeID is Option<NodeId> of roxmltree src/lib.rs. It holds id+1, and
// zero is None, as the NonZeroU32 of upstream does.
type optNodeID uint32

// someNodeID is Some(id).
func someNodeID(id NodeID) optNodeID {
	return optNodeID(id + 1)
}

// get returns the id, and false for None.
func (o optNodeID) get() (NodeID, bool) {
	if o == 0 {
		return 0, false
	}
	return NodeID(o - 1), true
}

// nodeKind is NodeKind in roxmltree src/lib.rs. Its kind is the NodeType
// (D27). Only Element uses tagName, attributes and namespaces, only PI uses
// pi, and only Comment and Text use text.
//
// Port: the StringStorage of a comment and a text is a Go string.
type nodeKind struct {
	typ        NodeType
	tagName    expandedNameIndexed
	attributes shortRange
	namespaces shortRange
	pi         PI
	text       string
}

// nodeData is NodeData in roxmltree src/lib.rs.
type nodeData struct {
	parent      optNodeID
	prevSibling optNodeID
	nextSubtree optNodeID
	lastChild   optNodeID
	kind        nodeKind
	rng         Range
}

// attributeData is AttributeData in roxmltree src/lib.rs.
type attributeData struct {
	name     expandedNameIndexed
	value    string
	rng      Range
	qnameLen uint16
	eqLen    uint8 // includes any surrounding spaces
}

// Attribute is Attribute in roxmltree src/lib.rs. It is an attribute.
type Attribute struct {
	doc  *Document
	data *attributeData
}

// Namespace is Attribute::namespace in roxmltree src/lib.rs. It returns
// attribute's namespace URI.
func (a Attribute) Namespace() (string, bool) {
	ns, ok := a.data.name.namespace(a.doc)
	if !ok {
		return "", false
	}
	return ns.URI(), true
}

// Name is Attribute::name in roxmltree src/lib.rs. It returns attribute's
// name.
func (a Attribute) Name() string {
	return a.data.name.localName
}

// Value is Attribute::value in roxmltree src/lib.rs. It returns attribute's
// value.
func (a Attribute) Value() string {
	return a.data.value
}

// ValueStorage is Attribute::value_storage in roxmltree src/lib.rs. It
// returns attribute's value storage.
//
// Useful when you need a more low-level access to an allocated string.
//
// Port: StringStorage is a Go string, so this is Value.
func (a Attribute) ValueStorage() string {
	return a.data.value
}

// Position is Attribute::position in roxmltree src/lib.rs. It returns
// attribute's position in bytes in the original document.
//
// You can calculate a human-readable text position via [Document.TextPosAt].
//
//	<e attr='value'/>
//	   ^
//
// Deprecated: replaced by `range`.
func (a Attribute) Position() int {
	return a.data.rng.Start
}

// Range is Attribute::range in roxmltree src/lib.rs. It returns attribute's
// range in bytes in the original document.
//
//	<e n:attr='value'/>
//	   ^^^^^^^^^^^^^^
func (a Attribute) Range() Range {
	return a.data.rng
}

// RangeQName is Attribute::range_qname in roxmltree src/lib.rs. It returns
// attribute's qname's range in bytes in the original document.
//
//	<e n:attr='value'/>
//	   ^^^^^^
//
// To reduce memory usage the qname length is limited by u16::MAX.
// If the attribute exceeds that limit then the end of the returned range will be incorrect.
func (a Attribute) RangeQName() Range {
	end := a.data.rng.Start + int(a.data.qnameLen)
	return Range{Start: a.data.rng.Start, End: end}
}

// RangeValue is Attribute::range_value in roxmltree src/lib.rs. It returns
// attribute's value's range in bytes in the original document, excluding the
// surrounding quotes.
//
// If the attribute's value is an empty string then the `start` and `end` of this `Range` are equal, and indicate the closing quote.
//
//	<e n:attr='value'/>
//	           ^^^^^
//
// To reduce memory usage the qname length is limited by u16::MAX,
// and the number of spaces around the equal sign is limited by u8::MAX.
// If the attribute exceeds those limits then the start of the returned range will be incorrect.
func (a Attribute) RangeValue() Range {
	// +1 on start and -1 on end are to exclude the quotes around the value (all valid quotes are 1 byte)
	start := a.data.rng.Start + int(a.data.qnameLen) + int(a.data.eqLen) + 1
	end := a.data.rng.End - 1
	return Range{Start: start, End: end}
}

// Equal is PartialEq for Attribute in roxmltree src/lib.rs. Two attributes
// are equal when their expanded names and their values are equal.
func (a Attribute) Equal(other Attribute) bool {
	return a.data.name.asExpandedName(a.doc) == other.data.name.asExpandedName(other.doc) &&
		a.data.value == other.data.value
}

// String is fmt::Debug for Attribute in roxmltree src/lib.rs.
//
// Port: the value prints as a quoted string. Upstream prints the
// StringStorage around it, such as Borrowed("b").
func (a Attribute) String() string {
	return "Attribute { name: " + a.data.name.asExpandedName(a.doc).String() +
		", value: " + rustDebugStr(a.data.value) + " }"
}

// Namespace is Namespace in roxmltree src/lib.rs. It is a namespace.
//
// Contains URI and *prefix* pair.
//
// Port: name is the Rust Option<&str>, and an empty name is None. Upstream
// never stores Some("").
type Namespace struct {
	name string
	uri  string
}

// Name is Namespace::name in roxmltree src/lib.rs. It returns namespace
// name/prefix.
func (n Namespace) Name() (string, bool) {
	return n.name, n.name != ""
}

// URI is Namespace::uri in roxmltree src/lib.rs. It returns namespace URI.
func (n Namespace) URI() string {
	return n.uri
}

// String is the derived fmt::Debug for Namespace in roxmltree src/lib.rs.
//
// Port: the URI prints as a quoted string. Upstream prints the StringStorage
// around it, such as Borrowed("b").
func (n Namespace) String() string {
	return "Namespace { name: " + rustDebugOptionStr(n.Name()) + ", uri: " + rustDebugStr(n.uri) + " }"
}

// namespaces is Namespaces in roxmltree src/lib.rs.
type namespaces struct {
	// Deduplicated namespace values used throughout the document
	values []Namespace
	// Indices into the above in tree order as the document is parsed
	treeOrder []namespaceIdx
	// Indices into the above sorted by value used for deduplication
	sortedOrder []namespaceIdx
}

// pushNS is Namespaces::push_ns in roxmltree src/lib.rs. An empty name is
// None.
func (n *namespaces) pushNS(name string, uri string) error {
	// binary_search_by: (value.name, value.uri) compared with (name, uri).
	// None is less than any Some, as an empty name is less than any other
	// name.
	sortedIdx, found := slices.BinarySearchFunc(n.sortedOrder, Namespace{name: name, uri: uri}, func(idx namespaceIdx, key Namespace) int {
		value := &n.values[idx]
		if c := cmp.Compare(value.name, key.name); c != 0 {
			return c
		}
		return cmp.Compare(value.uri, key.uri)
	})

	var idx namespaceIdx
	if found {
		idx = n.sortedOrder[sortedIdx]
	} else {
		if len(n.values) > 0xFFFF {
			return errNamespacesLimitReached()
		}
		idx = namespaceIdx(len(n.values))
		n.values = append(n.values, Namespace{name: name, uri: uri})
		n.sortedOrder = slices.Insert(n.sortedOrder, sortedIdx, idx)
	}

	n.treeOrder = append(n.treeOrder, idx)

	return nil
}

// pushRef is Namespaces::push_ref in roxmltree src/lib.rs.
func (n *namespaces) pushRef(treeIdx int) {
	idx := n.treeOrder[treeIdx]
	n.treeOrder = append(n.treeOrder, idx)
}

// exists is Namespaces::exists in roxmltree src/lib.rs. An empty prefix is
// None.
func (n *namespaces) exists(start int, prefix string) bool {
	for _, idx := range n.treeOrder[start:] {
		if n.values[idx].name == prefix {
			return true
		}
	}
	return false
}

// shrinkToFit is Namespaces::shrink_to_fit in roxmltree src/lib.rs.
func (n *namespaces) shrinkToFit() {
	n.values = shrinkToFit(n.values)
	n.treeOrder = shrinkToFit(n.treeOrder)
	// Only needed to deduplicate namespaces during parsing.
	n.sortedOrder = nil
}

// get is Namespaces::get in roxmltree src/lib.rs.
func (n *namespaces) get(idx namespaceIdx) *Namespace {
	return &n.values[idx]
}

// shrinkToFit is Vec::shrink_to_fit of the Rust alloc library. It copies the
// slice into one of its length when the slice has spare capacity.
func shrinkToFit[S ~[]E, E any](s S) S {
	if cap(s) == len(s) {
		return s
	}
	c := make(S, len(s))
	copy(c, s)
	return c
}

// namespaceIdx is NamespaceIdx in roxmltree src/lib.rs.
type namespaceIdx uint16

// expandedNameIndexed is ExpandedNameIndexed in roxmltree src/lib.rs.
// hasNamespace is false when namespace_idx is None.
type expandedNameIndexed struct {
	namespaceIdx namespaceIdx
	hasNamespace bool
	localName    string
}

// namespace is ExpandedNameIndexed::namespace in roxmltree src/lib.rs.
func (e expandedNameIndexed) namespace(doc *Document) (*Namespace, bool) {
	if !e.hasNamespace {
		return nil, false
	}
	return doc.namespaces.get(e.namespaceIdx), true
}

// asExpandedName is ExpandedNameIndexed::as_expanded_name in roxmltree
// src/lib.rs.
func (e expandedNameIndexed) asExpandedName(doc *Document) ExpandedName {
	ns, ok := e.namespace(doc)
	if !ok {
		return ExpandedName{name: e.localName}
	}
	return ExpandedName{
		uri:    ns.URI(),
		hasURI: true,
		name:   e.localName,
	}
}

// ExpandedName is ExpandedName in roxmltree src/lib.rs. It is an expanded
// name.
//
// Contains an namespace URI and name pair.
//
// Port: hasURI is false when the URI is None. A URI can be Some(""), for
// xmlns="". Two values are equal with ==, as the PartialEq of upstream says.
type ExpandedName struct {
	uri    string
	hasURI bool
	name   string
}

// Namespace is ExpandedName::namespace in roxmltree src/lib.rs. It returns a
// namespace URI.
func (e ExpandedName) Namespace() (string, bool) {
	return e.uri, e.hasURI
}

// Name is ExpandedName::name in roxmltree src/lib.rs. It returns a local
// name.
func (e ExpandedName) Name() string {
	return e.name
}

// ExpandedNameFromStatic is ExpandedName::from_static in roxmltree
// src/lib.rs. It creates a new instance from static data.
func ExpandedNameFromStatic(uri, name string) ExpandedName {
	return ExpandedName{
		uri:    uri,
		hasURI: true,
		name:   name,
	}
}

// String is fmt::Debug for ExpandedName in roxmltree src/lib.rs.
func (e ExpandedName) String() string {
	if ns, ok := e.Namespace(); ok {
		return "{" + ns + "}" + e.name
	}
	return e.name
}

// ExpandedNameFromStr is From<&str> for ExpandedName in roxmltree
// src/lib.rs. The name has no namespace.
func ExpandedNameFromStr(v string) ExpandedName {
	return ExpandedName{name: v}
}

// ExpandedNameFromTuple is From<(&str, &str)> for ExpandedName in roxmltree
// src/lib.rs. The name has the namespace URI uri.
func ExpandedNameFromTuple(uri, name string) ExpandedName {
	return ExpandedName{
		uri:    uri,
		hasURI: true,
		name:   name,
	}
}

// Node is Node in roxmltree src/lib.rs. It is a node in a document.
//
// # Document Order
//
// The implementation of the `Ord` traits for `Node` is based on the concept of *document-order*.
// In layman's terms, document-order is the order in which one would see each element if
// one opened a document in a text editor or web browser and scrolled down.
// Document-order convention is followed in XPath, CSS Counters, and DOM selectors API
// to ensure consistent results from selection.
// One difference in `roxmltree` is that there is the notion of more than one document
// in existence at a time. While Nodes within the same document are in document-order,
// Nodes in different documents will be grouped together, but not in any particular
// order.
//
// As an example, if we have a Document `a` with Nodes `[a0, a1, a2]` and a
// Document `b` with Nodes `[b0, b1]`, these Nodes in order could be either
// `[a0, a1, a2, b0, b1]` or `[b0, b1, a0, a1, a2]` and roxmltree makes no
// guarantee which it will be.
//
// Document-order is defined here in the
// [W3C XPath Recommendation](https://www.w3.org/TR/xpath-3/#id-document-order)
// The use of document-order in DOM Selectors is described here in the
// [W3C Selectors API Level 1](https://www.w3.org/TR/selectors-api/#the-apis)
//
// Port: two nodes are equal with ==, as the PartialEq of upstream says, and
// Cmp is the Ord of upstream.
type Node struct {
	// Node's ID.
	id NodeID

	// The tree containing the node.
	doc *Document

	// Node's data.
	d *nodeData
}

// Cmp is Ord for Node in roxmltree src/lib.rs. It orders the nodes by their
// id, then by their document.
func (n Node) Cmp(other Node) int {
	if c := cmp.Compare(n.id, other.id); c != 0 {
		return c
	}
	return cmp.Compare(n.doc.serial, other.doc.serial)
}

// NodeType is Node::node_type in roxmltree src/lib.rs. It returns node's
// type.
func (n Node) NodeType() NodeType {
	return n.d.kind.typ
}

// IsRoot is Node::is_root in roxmltree src/lib.rs. It checks that node is a
// root node.
func (n Node) IsRoot() bool {
	return n.NodeType() == NodeTypeRoot
}

// IsElement is Node::is_element in roxmltree src/lib.rs. It checks that node
// is an element node.
func (n Node) IsElement() bool {
	return n.NodeType() == NodeTypeElement
}

// IsPI is Node::is_pi in roxmltree src/lib.rs. It checks that node is a
// processing instruction node.
func (n Node) IsPI() bool {
	return n.NodeType() == NodeTypePI
}

// IsComment is Node::is_comment in roxmltree src/lib.rs. It checks that node
// is a comment node.
func (n Node) IsComment() bool {
	return n.NodeType() == NodeTypeComment
}

// IsText is Node::is_text in roxmltree src/lib.rs. It checks that node is a
// text node.
func (n Node) IsText() bool {
	return n.NodeType() == NodeTypeText
}

// Document is Node::document in roxmltree src/lib.rs. It returns node's
// document.
func (n Node) Document() *Document {
	return n.doc
}

// TagName is Node::tag_name in roxmltree src/lib.rs. It returns node's tag
// name.
//
// Returns an empty name with no namespace if the current node is not an element.
func (n Node) TagName() ExpandedName {
	if n.d.kind.typ == NodeTypeElement {
		return n.d.kind.tagName.asExpandedName(n.doc)
	}
	return ExpandedNameFromStr("")
}

// HasTagName is Node::has_tag_name in roxmltree src/lib.rs. It checks that
// node has a specified tag name.
func (n Node) HasTagName(name ExpandedName) bool {
	if n.d.kind.typ != NodeTypeElement {
		return false
	}
	tagName := n.d.kind.tagName
	if _, ok := name.Namespace(); ok {
		return tagName.asExpandedName(n.doc) == name
	}
	return tagName.localName == name.name
}

// DefaultNamespace is Node::default_namespace in roxmltree src/lib.rs. It
// returns node's default namespace URI.
func (n Node) DefaultNamespace() (string, bool) {
	for ns := range n.Namespaces() {
		if ns.name == "" {
			return ns.uri, true
		}
	}
	return "", false
}

// LookupPrefix is Node::lookup_prefix in roxmltree src/lib.rs. It returns a
// prefix for a given namespace URI.
func (n Node) LookupPrefix(uri string) (string, bool) {
	if uri == NSXMLURI {
		return nsXMLPrefix, true
	}

	for ns := range n.Namespaces() {
		if ns.uri == uri {
			return ns.Name()
		}
	}
	return "", false
}

// LookupNamespaceURI is Node::lookup_namespace_uri in roxmltree src/lib.rs.
// It returns an URI for a given prefix.
//
// Port: an empty prefix is None. Upstream returns None for Some(""), and Go
// returns the default namespace for "".
func (n Node) LookupNamespaceURI(prefix string) (string, bool) {
	for ns := range n.Namespaces() {
		if ns.name == prefix {
			return ns.uri, true
		}
	}
	return "", false
}

// Attribute is Node::attribute in roxmltree src/lib.rs. It returns element's
// attribute value.
func (n Node) Attribute(name ExpandedName) (string, bool) {
	a, ok := n.AttributeNode(name)
	if !ok {
		return "", false
	}
	return a.Value(), true
}

// AttributeNode is Node::attribute_node in roxmltree src/lib.rs. It returns
// element's attribute object.
//
// The same as [`attribute()`], but returns the `Attribute` itself instead of a value string.
func (n Node) AttributeNode(name ExpandedName) (Attribute, bool) {
	if _, ok := name.Namespace(); ok {
		for a := range n.Attributes() {
			if a.data.name.asExpandedName(n.doc) == name {
				return a, true
			}
		}
		return Attribute{}, false
	}
	for a := range n.Attributes() {
		if a.data.name.localName == name.name {
			return a, true
		}
	}
	return Attribute{}, false
}

// HasAttribute is Node::has_attribute in roxmltree src/lib.rs. It checks that
// element has a specified attribute.
func (n Node) HasAttribute(name ExpandedName) bool {
	_, ok := n.AttributeNode(name)
	return ok
}

// Attributes is Node::attributes in roxmltree src/lib.rs, with
// Attributes::new and the Iterator of Attributes. It returns element's
// attributes.
//
// Port: upstream also gives a double ended iterator with a length. A caller
// that needs them collects the sequence.
func (n Node) Attributes() iter.Seq[Attribute] {
	var attrs []attributeData
	if n.d.kind.typ == NodeTypeElement {
		start, end := n.d.kind.attributes.toURange()
		attrs = n.doc.attributes[start:end]
	}
	doc := n.doc
	return func(yield func(Attribute) bool) {
		for i := range attrs {
			if !yield(Attribute{doc: doc, data: &attrs[i]}) {
				return
			}
		}
	}
}

// Namespaces is Node::namespaces in roxmltree src/lib.rs, with the Iterator
// of NamespaceIter. It returns element's namespaces.
//
// Port: upstream also gives a double ended iterator with a length. A caller
// that needs them collects the sequence. The next_back of upstream returns
// the next item from the front, and Go has no port of it.
func (n Node) Namespaces() iter.Seq[Namespace] {
	var nss []namespaceIdx
	if n.d.kind.typ == NodeTypeElement {
		start, end := n.d.kind.namespaces.toURange()
		nss = n.doc.namespaces.treeOrder[start:end]
	}
	doc := n.doc
	return func(yield func(Namespace) bool) {
		for _, idx := range nss {
			if !yield(*doc.namespaces.get(idx)) {
				return
			}
		}
	}
}

// Text is Node::text in roxmltree src/lib.rs. It returns node's text.
//
//   - for an element will return a first text child
//   - for a comment will return a self text
//   - for a text node will return a self text
func (n Node) Text() (string, bool) {
	return n.TextStorage()
}

// TextStorage is Node::text_storage in roxmltree src/lib.rs. It returns
// node's text storage.
//
// Useful when you need a more low-level access to an allocated string.
//
// Port: StringStorage is a Go string, so this is Text.
func (n Node) TextStorage() (string, bool) {
	switch n.d.kind.typ {
	case NodeTypeElement:
		child, ok := n.FirstChild()
		if ok && child.IsText() {
			kind := &n.doc.nodes[child.id.GetUsize()].kind
			if kind.typ == NodeTypeText {
				return kind.text, true
			}
		}
		return "", false
	case NodeTypeComment, NodeTypeText:
		return n.d.kind.text, true
	}
	return "", false
}

// Tail is Node::tail in roxmltree src/lib.rs. It returns element's tail
// text.
func (n Node) Tail() (string, bool) {
	return n.TailStorage()
}

// TailStorage is Node::tail_storage in roxmltree src/lib.rs. It returns
// element's tail text storage.
//
// Useful when you need a more low-level access to an allocated string.
//
// Port: StringStorage is a Go string, so this is Tail.
func (n Node) TailStorage() (string, bool) {
	if !n.IsElement() {
		return "", false
	}

	next, ok := n.NextSibling()
	if !ok {
		return "", false
	}
	kind := &n.doc.nodes[next.id.GetUsize()].kind
	if kind.typ == NodeTypeText {
		return kind.text, true
	}
	return "", false
}

// PI is Node::pi in roxmltree src/lib.rs. It returns node as Processing
// Instruction.
func (n Node) PI() (PI, bool) {
	if n.d.kind.typ == NodeTypePI {
		return n.d.kind.pi, true
	}
	return PI{}, false
}

// Parent is Node::parent in roxmltree src/lib.rs. It returns the parent of
// this node.
func (n Node) Parent() (Node, bool) {
	id, ok := n.d.parent.get()
	if !ok {
		return Node{}, false
	}
	return n.doc.getNodeUnwrap(id), true
}

// ParentElement is Node::parent_element in roxmltree src/lib.rs. It returns
// the parent element of this node.
func (n Node) ParentElement() (Node, bool) {
	return findSkip1(n.Ancestors(), Node.IsElement)
}

// PrevSibling is Node::prev_sibling in roxmltree src/lib.rs. It returns the
// previous sibling of this node.
func (n Node) PrevSibling() (Node, bool) {
	id, ok := n.d.prevSibling.get()
	if !ok {
		return Node{}, false
	}
	return n.doc.getNodeUnwrap(id), true
}

// PrevSiblingElement is Node::prev_sibling_element in roxmltree src/lib.rs.
// It returns the previous sibling element of this node.
func (n Node) PrevSiblingElement() (Node, bool) {
	return findSkip1(n.PrevSiblings(), Node.IsElement)
}

// NextSibling is Node::next_sibling in roxmltree src/lib.rs. It returns the
// next sibling of this node.
func (n Node) NextSibling() (Node, bool) {
	id, ok := n.d.nextSubtree.get()
	if !ok {
		return Node{}, false
	}
	node := n.doc.getNodeUnwrap(id)
	possiblySelf, ok := node.d.prevSibling.get()
	if !ok {
		panic("next_subtree will always have a previous sibling")
	}
	if possiblySelf == n.id {
		return node, true
	}
	return Node{}, false
}

// NextSiblingElement is Node::next_sibling_element in roxmltree src/lib.rs.
// It returns the next sibling element of this node.
func (n Node) NextSiblingElement() (Node, bool) {
	return findSkip1(n.NextSiblings(), Node.IsElement)
}

// FirstChild is Node::first_child in roxmltree src/lib.rs. It returns the
// first child of this node.
func (n Node) FirstChild() (Node, bool) {
	if _, ok := n.d.lastChild.get(); !ok {
		return Node{}, false
	}
	return n.doc.getNodeUnwrap(NewNodeID(n.id.Get() + 1)), true
}

// FirstElementChild is Node::first_element_child in roxmltree src/lib.rs. It
// returns the first element child of this node.
func (n Node) FirstElementChild() (Node, bool) {
	for c := range n.Children() {
		if c.IsElement() {
			return c, true
		}
	}
	return Node{}, false
}

// LastChild is Node::last_child in roxmltree src/lib.rs. It returns the last
// child of this node.
func (n Node) LastChild() (Node, bool) {
	id, ok := n.d.lastChild.get()
	if !ok {
		return Node{}, false
	}
	return n.doc.getNodeUnwrap(id), true
}

// LastElementChild is Node::last_element_child in roxmltree src/lib.rs. It
// returns the last element child of this node.
//
// Port: upstream walks the children from the back. Go walks them from the
// front and keeps the last element, which gives the same node.
func (n Node) LastElementChild() (Node, bool) {
	var last Node
	found := false
	for c := range n.Children() {
		if c.IsElement() {
			last, found = c, true
		}
	}
	return last, found
}

// HasSiblings is Node::has_siblings in roxmltree src/lib.rs. It returns true
// if this node has siblings.
func (n Node) HasSiblings() bool {
	if _, ok := n.d.prevSibling.get(); ok {
		return true
	}
	_, ok := n.NextSibling()
	return ok
}

// HasChildren is Node::has_children in roxmltree src/lib.rs. It returns true
// if this node has children.
func (n Node) HasChildren() bool {
	_, ok := n.d.lastChild.get()
	return ok
}

// Ancestors is Node::ancestors in roxmltree src/lib.rs. It returns an
// iterator over ancestor nodes starting at this node.
func (n Node) Ancestors() iter.Seq[Node] {
	return axisIter(n, Node.Parent)
}

// PrevSiblings is Node::prev_siblings in roxmltree src/lib.rs. It returns an
// iterator over previous sibling nodes starting at this node.
func (n Node) PrevSiblings() iter.Seq[Node] {
	return axisIter(n, Node.PrevSibling)
}

// NextSiblings is Node::next_siblings in roxmltree src/lib.rs. It returns an
// iterator over next sibling nodes starting at this node.
func (n Node) NextSiblings() iter.Seq[Node] {
	return axisIter(n, Node.NextSibling)
}

// FirstChildren is Node::first_children in roxmltree src/lib.rs. It returns
// an iterator over first children nodes starting at this node.
func (n Node) FirstChildren() iter.Seq[Node] {
	return axisIter(n, Node.FirstChild)
}

// LastChildren is Node::last_children in roxmltree src/lib.rs. It returns an
// iterator over last children nodes starting at this node.
func (n Node) LastChildren() iter.Seq[Node] {
	return axisIter(n, Node.LastChild)
}

// Children is Node::children in roxmltree src/lib.rs, with the Iterator of
// Children. It returns an iterator over children nodes.
//
// Port: upstream also gives a double ended iterator. A caller that needs it
// collects the sequence.
func (n Node) Children() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		front, hasFront := n.FirstChild()
		back, hasBack := n.LastChild()
		for hasFront {
			node := front
			if hasFront == hasBack && front == back {
				hasFront, hasBack = false, false
			} else {
				front, hasFront = node.NextSibling()
			}
			if !yield(node) {
				return
			}
		}
	}
}

// Descendants is Node::descendants in roxmltree src/lib.rs, with
// Descendants::new and the Iterator of Descendants. It returns an iterator
// over this node and its descendants.
//
// Port: upstream also gives a double ended iterator with a length. A caller
// that needs them collects the sequence.
func (n Node) Descendants() iter.Seq[Node] {
	from := n.id.GetUsize()

	until := len(n.doc.nodes)
	if id, ok := n.d.nextSubtree.get(); ok {
		until = id.GetUsize()
	}

	nodes := n.doc.nodes[from:until]
	doc := n.doc
	return func(yield func(Node) bool) {
		for idx := range nodes {
			if !yield(Node{
				id:  nodeIDFromUsize(from + idx),
				d:   &nodes[idx],
				doc: doc,
			}) {
				return
			}
		}
	}
}

// Range is Node::range in roxmltree src/lib.rs. It returns node's range in
// bytes in the original document.
func (n Node) Range() Range {
	return n.d.rng
}

// ID is Node::id in roxmltree src/lib.rs. It returns node's NodeID.
func (n Node) ID() NodeID {
	return n.id
}

// String is fmt::Debug for Node in roxmltree src/lib.rs.
//
// Port: the attributes and the namespaces of an element print as a list of
// their own Debug texts. Upstream prints the slice iterators that hold them.
func (n Node) String() string {
	switch n.d.kind.typ {
	case NodeTypeRoot:
		return "Root"
	case NodeTypeElement:
		var attrs, nss []string
		for a := range n.Attributes() {
			attrs = append(attrs, a.String())
		}
		for ns := range n.Namespaces() {
			nss = append(nss, ns.String())
		}
		return "Element { tag_name: " + n.TagName().String() +
			", attributes: [" + strings.Join(attrs, ", ") +
			"], namespaces: [" + strings.Join(nss, ", ") + "] }"
	case NodeTypePI:
		pi := n.d.kind.pi
		return "PI { target: " + rustDebugStr(pi.Target) +
			", value: " + rustDebugOptionStr(pi.Value, pi.Value != "") + " }"
	case NodeTypeComment:
		return "Comment(" + rustDebugStr(n.d.kind.text) + ")"
	case NodeTypeText:
		return "Text(" + rustDebugStr(n.d.kind.text) + ")"
	}
	return ""
}

// axisIter is the Iterator of AxisIter in roxmltree src/lib.rs, an iterator
// over the specified axis.
func axisIter(start Node, next func(Node) (Node, bool)) iter.Seq[Node] {
	return func(yield func(Node) bool) {
		node, ok := start, true
		for ok {
			cur := node
			node, ok = next(cur)
			if !yield(cur) {
				return
			}
		}
	}
}

// findSkip1 is iter.skip(1).find(f) of the Rust core library.
func findSkip1(seq iter.Seq[Node], f func(Node) bool) (Node, bool) {
	first := true
	for n := range seq {
		if first {
			first = false
			continue
		}
		if f(n) {
			return n, true
		}
	}
	return Node{}, false
}
