// Copyright 2018 Yevhenii Reizner
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package roxmltree

import (
	"cmp"
	"errors"
	"slices"

	"github.com/xo/okage"
)

// This file ports no upstream file. It holds ParseRoot and Builder of D58.
// Upstream makes a Document only from XML text. ParseRoot reads the root
// element alone, and Builder makes a Document from the tree of another
// parser, such as the HTML tree of ivy, without XML text.

// newContext makes the context of a parse of text, with the root node and
// the namespace xml, as parse does.
func newContext(text string, opt ParsingOptions) *context {
	doc := Document{
		text:   text,
		serial: documentSerial.Add(1),
	}
	doc.nodes = append(doc.nodes, nodeData{
		kind: nodeKind{typ: NodeTypeRoot},
		rng:  Range{Start: 0, End: len(text)},
	})
	// The first namespace of a document is xml, and pushNS cannot fail on
	// it.
	_ = doc.namespaces.pushNS(nsXMLPrefix, NSXMLURI)
	return &context{
		opt:               opt,
		namespaceStartIdx: 1,
		currentAttributes: make([]tempAttributeData, 0, 16),
		parentPrefixes:    []string{""},
		afterText:         make([]string, 0, 1),
		parentID:          NewNodeID(0),
		tagName:           newNullTagNameSpan(),
		doc:               doc,
	}
}

// finish trims the slices of the document of ctx, as parse does, and
// returns the document.
func (ctx *context) finish() *Document {
	d := &ctx.doc
	d.nodes = shrinkToFit(d.nodes)
	d.attributes = shrinkToFit(d.attributes)
	d.namespaces.shrinkToFit()
	return d
}

// rootEvents passes the tokens of a parse to its context, and stops the
// parse at the end of the start tag of the root element.
type rootEvents struct {
	ctx *context
}

// stopParseError is the error that rootEvents returns to stop the
// tokenizer. It never reaches the caller of ParseRoot.
type stopParseError struct{}

// Error satisfies the error interface.
func (stopParseError) Error() string {
	return "parsing xml: stopped after the root element"
}

// token satisfies xmlEvents.
func (r rootEvents) token(t token) error {
	if err := r.ctx.token(t); err != nil {
		return err
	}
	if t.kind == tokenElementEnd {
		return stopParseError{}
	}
	return nil
}

// resolveEntity satisfies xmlEvents.
func (r rootEvents) resolveEntity(pubID string, hasPubID bool, uri string) (string, bool, error) {
	return r.ctx.resolveEntity(pubID, hasPubID, uri)
}

// ParseRoot parses the input XML string up to the end of the start tag of
// the root element, and stops (D58). The document holds the nodes before the
// root element and the root element with its attributes and its namespaces,
// but no child of the root element. The parse reads the DTD, and expands
// its entities in the values of the attributes, as Parse does.
//
// ParseRoot is for a program that needs only the attributes of the root
// element, such as the size of an SVG image. It does not check the text after
// the start tag, so a document that Parse rejects can give no error.
func ParseRoot(text string, opt ParsingOptions) (*Document, error) {
	ctx := newContext(text, opt)
	if err := parseTokenizer(text, opt.AllowDTD, rootEvents{ctx}); err != nil && !errors.Is(err, stopParseError{}) {
		return nil, err
	}
	d := &ctx.doc
	if _, ok := d.Root().FirstElementChild(); !ok {
		return nil, errNoRootNode()
	}
	return ctx.finish(), nil
}

// Builder makes a Document from calls, in place of XML text (D58). A program
// that holds a tree from another parser, such as an HTML parser, builds a
// Document from it with a Builder, and needs no XML text.
//
// Call StartElement for each element, then Namespace and Attribute for its
// namespace declarations and its attributes, then Text and StartElement for
// its content, in the order of the document, and then EndElement. Call
// Document at the end. The document has one root element, and no text
// outside it.
//
// The names are expanded names, with the namespace URI. A Builder does not
// check that a name is a valid XML name, and the document keeps a name as
// it is. It does not declare a prefix for a namespace URI. A program that
// reads the prefixes, with Node.Namespaces or Node.LookupPrefix, declares
// them with Namespace.
//
// The document has no input text. Its nodes and its attributes have empty
// ranges. An error is a sentinel error of the root package, such as
// okage.ErrXMLDuplicatedAttribute or okage.ErrXMLBuildOrder, without a
// position. After an error, each call returns the same error.
type Builder struct {
	ctx *context

	// pending is true from StartElement to the next call that is not
	// Namespace or Attribute. tag and attributes are the name and the
	// attributes of the element.
	pending    bool
	tag        ExpandedName
	attributes []builderAttribute

	// depth is the number of open elements, and rootDone is true after
	// the end of the root element.
	depth    int
	rootDone bool

	done bool
	err  error
}

// builderAttribute is an attribute of the pending element of a Builder.
type builderAttribute struct {
	name  ExpandedName
	value string
}

// NewBuilder returns a Builder of an empty document. Of the options, only
// NodesLimit applies, as it applies to a parse.
func NewBuilder(opt ParsingOptions) *Builder {
	return &Builder{ctx: newContext("", opt)}
}

// fail keeps the first error of the builder, and returns it. An *Error of
// the parser becomes its sentinel error, because its position means nothing
// here.
func (b *Builder) fail(err error) error {
	if e, ok := errors.AsType[*Error](err); ok {
		err = e.kind
	}
	if b.err == nil {
		b.err = err
	}
	return b.err
}

// StartElement starts an element with the given name, as a child of the open
// element, or as the root element.
func (b *Builder) StartElement(name ExpandedName) error {
	if err := b.flush(); err != nil {
		return err
	}
	switch {
	case b.rootDone:
		return b.fail(okage.ErrXMLBuildOrder)
	case name.name == "":
		return b.fail(okage.ErrXMLInvalidName)
	}
	b.ctx.resetAfterText()
	b.pending = true
	b.tag = name
	b.attributes = b.attributes[:0]
	return nil
}

// Namespace declares the namespace prefix for uri on the element that
// StartElement started, as the attribute xmlns:prefix does. The prefix ""
// declares the default namespace, as the attribute xmlns does. The rules of
// the parser for the prefixes xml and xmlns apply.
func (b *Builder) Namespace(prefix, uri string) error {
	if b.err != nil {
		return b.err
	}
	if !b.pending {
		return b.fail(okage.ErrXMLBuildOrder)
	}
	switch {
	case uri == NSXMLNSURI:
		return b.fail(okage.ErrXMLUnexpectedXMLNSURI)
	case prefix == nsXMLPrefix && uri != NSXMLURI:
		return b.fail(okage.ErrXMLInvalidXMLPrefixURI)
	case prefix != nsXMLPrefix && uri == NSXMLURI:
		return b.fail(okage.ErrXMLUnexpectedXMLURI)
	case b.ctx.doc.namespaces.exists(b.ctx.namespaceStartIdx, prefix):
		return b.fail(okage.ErrXMLDuplicatedNamespace)
	case prefix == nsXMLPrefix:
		// The parser does not add the namespace xml either.
		return nil
	}
	if err := b.ctx.doc.namespaces.pushNS(prefix, uri); err != nil {
		return b.fail(err)
	}
	return nil
}

// Attribute adds an attribute to the element that StartElement started.
func (b *Builder) Attribute(name ExpandedName, value string) error {
	if b.err != nil {
		return b.err
	}
	switch {
	case !b.pending:
		return b.fail(okage.ErrXMLBuildOrder)
	case name.name == "":
		return b.fail(okage.ErrXMLInvalidName)
	}
	for _, a := range b.attributes {
		if a.name == name {
			return b.fail(okage.ErrXMLDuplicatedAttribute)
		}
	}
	b.attributes = append(b.attributes, builderAttribute{name, value})
	return nil
}

// Text adds text to the open element. Text next to text joins it, as in a
// parse. An empty text adds nothing.
func (b *Builder) Text(text string) error {
	if err := b.flush(); err != nil {
		return err
	}
	switch {
	case text == "":
		return nil
	case b.depth == 0:
		return b.fail(okage.ErrXMLBuildOrder)
	}
	if err := b.ctx.appendText(text, Range{}); err != nil {
		return b.fail(err)
	}
	return nil
}

// EndElement ends the open element.
func (b *Builder) EndElement() error {
	if err := b.flush(); err != nil {
		return err
	}
	if b.depth == 0 {
		return b.fail(okage.ErrXMLBuildOrder)
	}
	ctx := b.ctx
	ctx.resetAfterText()
	ctx.awaitingSubtree = append(ctx.awaitingSubtree, ctx.parentID)
	parent, _ := ctx.doc.nodes[ctx.parentID.GetUsize()].parent.get()
	ctx.parentID = parent
	b.depth--
	if b.depth == 0 {
		b.rootDone = true
	}
	return nil
}

// Document returns the document. It fails when the document has no root
// element, or when an element is open. The builder cannot be used after it.
func (b *Builder) Document() (*Document, error) {
	if err := b.flush(); err != nil {
		return nil, err
	}
	switch {
	case b.done:
		return nil, b.fail(okage.ErrXMLBuildOrder)
	case b.depth > 0:
		return nil, b.fail(okage.ErrXMLUnclosedRootNode)
	case !b.rootDone:
		return nil, b.fail(okage.ErrXMLNoRootNode)
	}
	b.done = true
	b.ctx.resetAfterText()
	return b.ctx.finish(), nil
}

// flush appends the element that StartElement started, with its namespaces
// and its attributes, as processElement does for a start tag.
func (b *Builder) flush() error {
	if b.err != nil {
		return b.err
	}
	if b.done {
		return b.fail(okage.ErrXMLBuildOrder)
	}
	if !b.pending {
		return nil
	}
	b.pending = false
	ctx := b.ctx
	namespaces := ctx.resolveNamespaces()
	ctx.namespaceStartIdx = len(ctx.doc.namespaces.treeOrder)

	tagName, err := b.indexName(b.tag, namespaces, false)
	if err != nil {
		return b.fail(err)
	}
	attributes := newShortRange(0, 0)
	if len(b.attributes) > 0 {
		if uint64(len(ctx.doc.attributes))+uint64(len(b.attributes)) >= 0xFFFF_FFFF {
			return b.fail(okage.ErrXMLAttributesLimitReached)
		}
		start := len(ctx.doc.attributes)
		for _, a := range b.attributes {
			name, err := b.indexName(a.name, namespaces, true)
			if err != nil {
				return b.fail(err)
			}
			ctx.doc.attributes = append(ctx.doc.attributes, attributeData{name: name, value: a.value})
		}
		attributes = shortRangeFrom(start, len(ctx.doc.attributes))
	}
	id, err := ctx.appendNode(nodeKind{
		typ:        NodeTypeElement,
		tagName:    tagName,
		attributes: attributes,
		namespaces: namespaces,
	}, Range{})
	if err != nil {
		return b.fail(err)
	}
	ctx.parentID = id
	b.depth++
	return nil
}

// indexName returns the indexed form of an expanded name. The namespace is
// a namespace in scope with the same URI, with a prefix for an attribute,
// or else a namespace of the document with no prefix.
func (b *Builder) indexName(name ExpandedName, namespaces shortRange, attribute bool) (expandedNameIndexed, error) {
	if !name.hasURI {
		return expandedNameIndexed{localName: name.name}, nil
	}
	if name.uri == NSXMLURI {
		// The namespace xml is always the first one of the document.
		return expandedNameIndexed{namespaceIdx: 0, hasNamespace: true, localName: name.name}, nil
	}
	ns := &b.ctx.doc.namespaces
	start, end := namespaces.toURange()
	for _, idx := range ns.treeOrder[start:end] {
		v := ns.get(idx)
		if v.uri == name.uri && (!attribute || v.name != "") {
			return expandedNameIndexed{namespaceIdx: idx, hasNamespace: true, localName: name.name}, nil
		}
	}
	idx, err := ns.intern(name.uri)
	if err != nil {
		return expandedNameIndexed{}, err
	}
	return expandedNameIndexed{namespaceIdx: idx, hasNamespace: true, localName: name.name}, nil
}

// intern returns the index of the namespace with no prefix and the given
// URI, and adds it to the values when it is new. Unlike pushNS, it does not
// put the namespace in scope.
func (n *namespaces) intern(uri string) (namespaceIdx, error) {
	key := Namespace{uri: uri}
	i, found := slices.BinarySearchFunc(n.sortedOrder, key, func(idx namespaceIdx, key Namespace) int {
		value := &n.values[idx]
		if c := cmp.Compare(value.name, key.name); c != 0 {
			return c
		}
		return cmp.Compare(value.uri, key.uri)
	})
	if found {
		return n.sortedOrder[i], nil
	}
	if len(n.values) > 0xFFFF {
		return 0, okage.ErrXMLNamespacesLimitReached
	}
	idx := namespaceIdx(len(n.values))
	n.values = append(n.values, key)
	n.sortedOrder = slices.Insert(n.sortedOrder, i, idx)
	return idx, nil
}

// Copy adds a copy of the node n and of its descendants to the open element,
// or as the root element. For the root node of a document, it copies the
// children of the root node. It copies the elements, their attributes and
// their text. It leaves out the comments and the processing instructions.
// The first element that it copies declares all the namespaces in its
// scope, and each other element declares the namespaces that its parent
// does not have.
func (b *Builder) Copy(n Node) error {
	if b.err != nil {
		return b.err
	}
	switch {
	case n.IsText():
		text, _ := n.Text()
		return b.Text(text)
	case !n.IsRoot() && !n.IsElement():
		return nil
	}
	type frame struct {
		next    Node
		hasNext bool
	}
	var stack []frame
	first := true
	enter := func(n Node) error {
		if err := b.StartElement(n.TagName()); err != nil {
			return err
		}
		parent, hasParent := n.ParentElement()
		for ns := range n.Namespaces() {
			if !first && hasParent && hasNamespace(parent, ns) {
				continue
			}
			if err := b.Namespace(ns.name, ns.uri); err != nil {
				return err
			}
		}
		first = false
		for a := range n.Attributes() {
			name := ExpandedNameFromStr(a.Name())
			if uri, ok := a.Namespace(); ok {
				name = ExpandedNameFromTuple(uri, a.Name())
			}
			if err := b.Attribute(name, a.Value()); err != nil {
				return err
			}
		}
		child, ok := n.FirstChild()
		stack = append(stack, frame{child, ok})
		return nil
	}
	if n.IsElement() {
		if err := enter(n); err != nil {
			return err
		}
	} else {
		child, ok := n.FirstChild()
		stack = append(stack, frame{child, ok})
	}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if !top.hasNext {
			stack = stack[:len(stack)-1]
			if len(stack) == 0 && !n.IsElement() {
				break
			}
			if err := b.EndElement(); err != nil {
				return err
			}
			continue
		}
		c := top.next
		top.next, top.hasNext = c.NextSibling()
		switch {
		case c.IsElement():
			if err := enter(c); err != nil {
				return err
			}
		case c.IsText():
			text, _ := c.Text()
			if err := b.Text(text); err != nil {
				return err
			}
		}
	}
	return nil
}

// hasNamespace reports whether the element n has the namespace ns in scope.
func hasNamespace(n Node, ns Namespace) bool {
	for v := range n.Namespaces() {
		if v == ns {
			return true
		}
	}
	return false
}
