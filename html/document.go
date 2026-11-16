package html

// This file ports no upstream file. It holds the tree that ParseDocument and
// ParseFragment return, and their options (D49). tinyhtml5 builds an
// ElementTree of Python. The port builds this tree, with a node for each
// run of text.

import (
	"fmt"
	"io"
	"iter"
	"slices"
)

// NodeType is the type of a Node.
type NodeType uint8

// The values of NodeType.
const (
	// DocumentNode is the root of a document or of a fragment.
	DocumentNode NodeType = iota
	// ElementNode is an element.
	ElementNode
	// TextNode is a run of text.
	TextNode
	// CommentNode is a comment.
	CommentNode
	// DoctypeNode is a doctype.
	DoctypeNode
)

// String returns the name of the type.
func (t NodeType) String() string {
	switch t {
	case DocumentNode:
		return "document"
	case ElementNode:
		return "element"
	case TextNode:
		return "text"
	case CommentNode:
		return "comment"
	case DoctypeNode:
		return "doctype"
	}
	return fmt.Sprintf("NodeType(%d)", uint8(t))
}

// Attribute is an attribute of an element.
type Attribute struct {
	// Namespace is empty for most attributes. An attribute of an element of
	// SVG or MathML, such as xlink:href, has NamespaceXLink, NamespaceXML or
	// NamespaceXMLNS.
	Namespace string

	// Name is the local name, in lower case for an element of HTML. An
	// attribute of SVG or MathML keeps the case of the standard, such as
	// viewBox.
	Name string

	// Value is the value, with its character references decoded.
	Value string
}

// Node is a node of a parsed document. A node does not change after the
// parse.
type Node struct {
	typ       NodeType
	namespace string
	name      string
	data      string
	publicID  string
	systemID  string
	attrs     []Attribute

	parent      *Node
	firstChild  *Node
	lastChild   *Node
	prevSibling *Node
	nextSibling *Node

	// text holds the text of a text node while the parse runs.
	text []byte
}

// Type returns the type of the node.
func (n *Node) Type() NodeType {
	return n.typ
}

// Namespace returns the namespace of an element: NamespaceHTML,
// NamespaceSVG or NamespaceMathML.
func (n *Node) Namespace() string {
	return n.namespace
}

// Name returns the local name of an element, or the name of a doctype. The
// name of an element of HTML is in lower case. An element of SVG keeps the
// case of the standard, such as foreignObject.
func (n *Node) Name() string {
	return n.name
}

// Data returns the text of a text node or of a comment.
func (n *Node) Data() string {
	return n.data
}

// PublicID returns the public identifier of a doctype, or an empty string.
func (n *Node) PublicID() string {
	return n.publicID
}

// SystemID returns the system identifier of a doctype, or an empty string.
func (n *Node) SystemID() string {
	return n.systemID
}

// Attributes returns a copy of the attributes of an element, in the order of
// the source.
func (n *Node) Attributes() []Attribute {
	return slices.Clone(n.attrs)
}

// Attr returns the value of the attribute of an element with a name and no
// namespace, and true if the element has it.
func (n *Node) Attr(name string) (string, bool) {
	return n.AttrNS("", name)
}

// AttrNS returns the value of the attribute of an element with a namespace
// and a local name, and true if the element has it.
func (n *Node) AttrNS(namespace, name string) (string, bool) {
	if i := attrIndex(n.attrs, namespace, name); i >= 0 {
		return n.attrs[i].Value, true
	}
	return "", false
}

// Parent returns the parent of the node, or nil for the root.
func (n *Node) Parent() *Node {
	return n.parent
}

// FirstChild returns the first child of the node, or nil.
func (n *Node) FirstChild() *Node {
	return n.firstChild
}

// LastChild returns the last child of the node, or nil.
func (n *Node) LastChild() *Node {
	return n.lastChild
}

// PrevSibling returns the node before this one in its parent, or nil.
func (n *Node) PrevSibling() *Node {
	return n.prevSibling
}

// NextSibling returns the node after this one in its parent, or nil.
func (n *Node) NextSibling() *Node {
	return n.nextSibling
}

// Children returns the children of the node, in order.
func (n *Node) Children() iter.Seq[*Node] {
	return func(yield func(*Node) bool) {
		for c := n.firstChild; c != nil; c = c.nextSibling {
			if !yield(c) {
				return
			}
		}
	}
}

// attrIndex returns the index of the attribute with a namespace and a name,
// or -1.
func attrIndex(attrs []Attribute, namespace, name string) int {
	for i, a := range attrs {
		if a.Name == name && a.Namespace == namespace {
			return i
		}
	}
	return -1
}

// QuirksMode is the mode of a document, which its doctype sets. A
// renderer lays out a document in quirks mode with the rules of old
// browsers.
type QuirksMode uint8

// The values of QuirksMode.
const (
	NoQuirks QuirksMode = iota
	Quirks
	LimitedQuirks
)

// String returns the name of the mode, as tinyhtml5 names it.
func (m QuirksMode) String() string {
	switch m {
	case NoQuirks:
		return "no quirks"
	case Quirks:
		return "quirks"
	case LimitedQuirks:
		return "limited quirks"
	}
	return fmt.Sprintf("QuirksMode(%d)", uint8(m))
}

// Document is a parsed HTML document, or a parsed fragment.
type Document struct {
	root     *Node
	encoding string
	mode     QuirksMode
	issues   []Issue
}

// Root returns the root of the tree, a node of type DocumentNode. The root
// of a document holds the doctype, the comments around the html element and
// the html element. The root of a fragment holds the nodes of the fragment.
func (d *Document) Root() *Node {
	return d.root
}

// DocumentElement returns the html element of a document, or nil for a
// fragment.
func (d *Document) DocumentElement() *Node {
	for c := range d.root.Children() {
		if c.typ == ElementNode && c.name == "html" && c.namespace == NamespaceHTML {
			return c
		}
	}
	return nil
}

// Encoding returns the name of the encoding that decoded the input, such as
// "utf-8" or "windows-1252", as the WHATWG Encoding Standard names it.
func (d *Document) Encoding() string {
	return d.encoding
}

// Mode returns the quirks mode of the document.
func (d *Document) Mode() QuirksMode {
	return d.mode
}

// Issues returns a copy of the parse errors of the document, in order. HTML
// recovers from each one, so an issue is not an error. The list stops at the
// limit of WithMaxIssues.
func (d *Document) Issues() []Issue {
	return slices.Clone(d.issues)
}

// CharsetReader decodes an input in an encoding that html does not decode
// itself. It returns a reader of the text in UTF-8. charset is the name of the
// encoding in the WHATWG Encoding Standard, such as "shift_jis".
type CharsetReader func(charset string, input io.Reader) (io.Reader, error)

// The default limits of ParseDocument and ParseFragment.
const (
	// DefaultMaxDepth is the default of WithMaxDepth.
	DefaultMaxDepth = 512
	// DefaultMaxNodes is the default of WithMaxNodes.
	DefaultMaxNodes = 1 << 20
	// DefaultMaxIssues is the default of WithMaxIssues.
	DefaultMaxIssues = 1000
)

// DocumentOption is an option of ParseDocument and ParseFragment.
type DocumentOption func(*documentOptions)

// documentOptions holds the options of ParseDocument and ParseFragment.
type documentOptions struct {
	charset        string
	defaultCharset string
	charsetReader  CharsetReader
	maxDepth       int
	maxNodes       int
	maxIssues      int
}

// newDocumentOptions returns the options with their defaults.
func newDocumentOptions(opts []DocumentOption) *documentOptions {
	o := &documentOptions{
		defaultCharset: encodingWindows1252,
		maxDepth:       DefaultMaxDepth,
		maxNodes:       DefaultMaxNodes,
		maxIssues:      DefaultMaxIssues,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithCharset gives the encoding of the input, such as "utf-8", which wins
// over a meta element. Only a byte order mark wins over it. An unknown label
// has no effect.
func WithCharset(label string) DocumentOption {
	return func(o *documentOptions) {
		o.charset = label
	}
}

// WithDefaultCharset gives the encoding of an input with no byte order mark
// and no meta element that declares one. Its default is windows-1252.
func WithDefaultCharset(label string) DocumentOption {
	return func(o *documentOptions) {
		o.defaultCharset = label
	}
}

// WithCharsetReader gives the decoder of the encodings that html does not
// decode itself. html decodes UTF-8, UTF-16LE, UTF-16BE, windows-1252 and
// x-user-defined. Without a reader, or when the reader fails, html decodes
// another encoding as windows-1252, and adds an issue.
func WithCharsetReader(r CharsetReader) DocumentOption {
	return func(o *documentOptions) {
		o.charsetReader = r
	}
}

// WithMaxDepth sets the largest number of open elements. The tree is about as
// deep as the largest number of open elements. When the input opens more, the
// parse stops there, as at the end of the input, and adds an issue. A value
// below 1 keeps the default, DefaultMaxDepth.
func WithMaxDepth(n int) DocumentOption {
	return func(o *documentOptions) {
		if n > 0 {
			o.maxDepth = n
		}
	}
}

// WithMaxNodes sets the largest number of nodes in the tree. When the input
// makes more, the parse stops there, as at the end of the input, and adds an
// issue. A value below 1 keeps the default, DefaultMaxNodes.
func WithMaxNodes(n int) DocumentOption {
	return func(o *documentOptions) {
		if n > 0 {
			o.maxNodes = n
		}
	}
}

// WithMaxIssues sets the largest number of issues that a document keeps. A
// value below 1 keeps the default, DefaultMaxIssues.
func WithMaxIssues(n int) DocumentOption {
	return func(o *documentOptions) {
		if n > 0 {
			o.maxIssues = n
		}
	}
}

// ParseDocument reads and parses an HTML document with the parsing algorithm
// of the HTML standard, as a port of tinyhtml5. It finds the encoding from a
// byte order mark, WithCharset, a meta element or WithDefaultCharset, in that
// order. It never fails for HTML that is not valid: it recovers as a browser
// does, and it lists each parse error as an issue of the document. It
// returns an error only when r fails.
func ParseDocument(r io.Reader, opts ...DocumentOption) (*Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("parsing html: reading the document: %w", err)
	}
	o := newDocumentOptions(opts)
	p := newHTMLParser(o)
	p.parseStream(newBinaryInputStream(data, o), "")
	return p.document(false), nil
}

// ParseFragment reads and parses a fragment of HTML, as the content of an
// element with the name container, such as "div". The root of the returned
// document holds the nodes of the fragment. It returns an error only when r
// fails.
func ParseFragment(r io.Reader, container string, opts ...DocumentOption) (*Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("parsing html: reading the fragment: %w", err)
	}
	o := newDocumentOptions(opts)
	p := newHTMLParser(o)
	p.parseStream(newBinaryInputStream(data, o), container)
	return p.document(true), nil
}

// finishText sets the text of each text node in the tree under root from the
// bytes that the parse appended.
func finishText(root *Node) {
	n := root
	for n != nil {
		if n.typ == TextNode {
			n.data = string(n.text)
			n.text = nil
		}
		if n.firstChild != nil {
			n = n.firstChild
			continue
		}
		for n != root && n.nextSibling == nil {
			n = n.parent
		}
		if n == root {
			return
		}
		n = n.nextSibling
	}
}
