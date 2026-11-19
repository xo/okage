// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"math"

	"github.com/xo/okage/internal/pdfwriter"
)

// This file ports krilla src/interchange/outline.rs, with the methods of
// SerializeContext of src/serialize.rs and of Document of src/document.rs
// that set the outline.
//
// Building outlines for the navigation of the document.
//
// An outline is a tree-like structure that stores the hierarchical structure
// of a document. In particular, in most cases it is used to define the
// hierarchy of headings in the document. The Outline allows you to encode
// this behavior, and makes it easier to navigate the document in PDF viewers
// that support this feature.
//
// The intended usage is to create a new, empty outline in the very
// beginning. It represents the root of the tree. Then, you can create new
// outline nodes, which you can recursively nest in any way you wish while
// processing your document. In the end, you need to push the first layer of
// children to the Outline object.
//
// Finally, once you are done building your outline tree, you can use
// Document.SetOutline to store the outline in the document.

// Outline is Outline in krilla src/interchange/outline.rs. The zero value is
// an empty outline, as Outline::new and the Default of upstream.
//
// An outline.
//
// This represents the root of the outline tree.
type Outline struct {
	children []OutlineNode
}

// NewOutline is Outline::new in krilla src/interchange/outline.rs.
//
// Create a new, empty outline.
func NewOutline() Outline {
	return Outline{}
}

// isEmpty is Outline::is_empty in krilla src/interchange/outline.rs.
func (o *Outline) isEmpty() bool {
	return len(o.children) == 0
}

// PushChild is Outline::push_child in krilla src/interchange/outline.rs.
//
// Push a new child (which may in turn contain other children) to the
// outline.
func (o *Outline) PushChild(node OutlineNode) {
	o.children = append(o.children, node)
}

// outlineable is the trait Outlineable in krilla
// src/interchange/outline.rs. The writers Outline and OutlineItem of
// pdfwriter satisfy it through the adapters below.
type outlineable interface {
	first(item pdfwriter.Ref)
	last(item pdfwriter.Ref)
	count(count int32)
}

// outlineWriter is the impl Outlineable for the writer Outline of
// pdf-writer.
type outlineWriter struct{ w *pdfwriter.Outline }

func (o outlineWriter) first(item pdfwriter.Ref) { o.w.First(item) }
func (o outlineWriter) last(item pdfwriter.Ref)  { o.w.Last(item) }
func (o outlineWriter) count(count int32)        { o.w.Count(count) }

// outlineItemWriter is the impl Outlineable for the writer OutlineItem of
// pdf-writer.
type outlineItemWriter struct{ w *pdfwriter.OutlineItem }

func (o outlineItemWriter) first(item pdfwriter.Ref) { o.w.First(item) }
func (o outlineItemWriter) last(item pdfwriter.Ref)  { o.w.Last(item) }
func (o outlineItemWriter) count(count int32)        { o.w.Count(count) }

// serializedChildren is SerializedChildren in krilla
// src/interchange/outline.rs.
type serializedChildren struct {
	first        pdfwriter.Ref
	last         pdfwriter.Ref
	visibleCount int
}

// write is SerializedChildren::write in krilla src/interchange/outline.rs.
// krilla unwraps the conversion of the count into an i32. The port clamps a
// count above the largest i32, which needs more nodes than a program can
// hold in memory.
func (c serializedChildren) write(o outlineable, negateCount bool) {
	o.first(c.first)
	o.last(c.last)

	count := int32(min(c.visibleCount, math.MaxInt32))
	if negateCount {
		count = -count
	}
	o.count(count)
}

// serialize is Outline::serialize in krilla src/interchange/outline.rs.
func (o *Outline) serialize(sc *serializeContext, cc *chunkContainer, root pdfwriter.Ref) {
	chunk := sc.newChunk()
	children, ok := serializeOutlineChildren(o.children, root, chunk, sc)

	outline := chunk.Outline(root)
	if ok {
		children.write(outlineWriter{outline}, false)
	}
	outline.Finish()

	cc.nonStream.outline = &refChunk{ref: root, chunk: chunk}
}

// OutlineNode is OutlineNode in krilla src/interchange/outline.rs.
//
// An outline node.
//
// This represents either an intermediate node in the outline tree, or a leaf
// node if it does not contain any further children itself.
type OutlineNode struct {
	// children holds the children of the outline node.
	children []OutlineNode
	// text is the text of the outline entry.
	text string
	// destination is the destination of the outline entry.
	destination XYZDestination
	// open says whether this node is initially open (children shown
	// expanded).
	open bool
}

// NewOutlineNode is OutlineNode::new in krilla src/interchange/outline.rs.
//
// Create a new outline node.
//
// text is the string that should be displayed in the outline tree, and
// destination is the destination that should be jumped to when clicking on
// the outline entry.
func NewOutlineNode(text string, destination XYZDestination) OutlineNode {
	return OutlineNode{text: text, destination: destination}
}

// WithOpen is OutlineNode::with_open in krilla src/interchange/outline.rs.
//
// Set whether this node is initially open (children shown expanded).
//
// If open is true, the node's children will be shown expanded when the
// document is opened in a PDF viewer. If false, the children will be
// collapsed. Leaf nodes (nodes without children) are unaffected by this flag.
//
// By default, this flag is set to false.
func (n OutlineNode) WithOpen(open bool) OutlineNode {
	n.open = open
	return n
}

// PushChild is OutlineNode::push_child in krilla
// src/interchange/outline.rs.
//
// Add a new child to the outline node.
func (n *OutlineNode) PushChild(node OutlineNode) {
	n.children = append(n.children, node)
}

// serialize is OutlineNode::serialize in krilla src/interchange/outline.rs. A
// zero next or prev is None. It returns the number of visible entries.
func (n *OutlineNode) serialize(sc *serializeContext, parent, root, next, prev pdfwriter.Ref, chunk *pdfwriter.Chunk) int {
	children, ok := serializeOutlineChildren(n.children, root, chunk, sc)
	outlineEntry := chunk.OutlineItem(root)
	outlineEntry.Parent(parent)

	if next.Get() != 0 {
		outlineEntry.Next(next)
	}

	if prev.Get() != 0 {
		outlineEntry.Prev(prev)
	}

	if ok {
		children.write(outlineItemWriter{outlineEntry}, !n.open)
	}

	outlineEntry.Title(pdfwriter.TextStr(n.text))

	destRef := sc.registerXYZDestination(n.destination)
	outlineEntry.Pair(pdfwriter.Name("Dest"), destRef)

	outlineEntry.Finish()

	// See the algorithm described in the PDF spec. When recursing down, we
	// do not go into nodes that whose count is negative (i.e. nodes that are
	// closed). Therefore, in case the node is closed, the visible count is
	// just the node itself, so 1.
	if n.open && ok {
		return 1 + children.visibleCount
	}
	return 1
}

// serializeOutlineChildren is serialize_children in krilla
// src/interchange/outline.rs. It returns false for no children, where
// upstream returns None.
func serializeOutlineChildren(children []OutlineNode, parent pdfwriter.Ref, chunk *pdfwriter.Chunk, sc *serializeContext) (serializedChildren, bool) {
	if len(children) == 0 {
		return serializedChildren{}, false
	}

	visibleCount := 0
	first := sc.newRef()
	last := first

	var prev pdfwriter.Ref
	cur := first

	for i := range children {
		var next pdfwriter.Ref
		if i < len(children)-1 {
			next = sc.newRef()
		}

		last = cur

		visibleCount += children[i].serialize(sc, parent, last, next, prev, chunk)

		prev = cur
		cur = next
	}

	return serializedChildren{first: first, last: last, visibleCount: visibleCount}, true
}

// clone gives a deep copy of the node, so that a later call of PushChild on
// a node of the caller does not change the outline of a document.
func (n *OutlineNode) clone() OutlineNode {
	c := *n
	c.children = make([]OutlineNode, len(n.children))
	for i := range n.children {
		c.children[i] = n.children[i].clone()
	}
	return c
}

// setOutline is SerializeContext::set_outline in krilla src/serialize.rs.
func (sc *serializeContext) setOutline(outline Outline) {
	// Only set it if it's not empty or if the current validator requires an
	// outline.
	_, prohibited := sc.serializeSettings().validators().prohibits(validationError{kind: validationMissingDocumentOutline})
	if !outline.isEmpty() || prohibited {
		c := Outline{children: make([]OutlineNode, len(outline.children))}
		for i := range outline.children {
			c.children[i] = outline.children[i].clone()
		}
		sc.globalObjects.outline = &c
	}
}

// SetOutline is Document::set_outline in krilla src/document.rs.
//
// Set the outline of the document.
func (d *Document) SetOutline(outline Outline) {
	d.sc.setOutline(outline)
}
