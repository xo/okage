// Copyright 2018 Yevhenii Reizner
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package roxmltree

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/okage"
)

// Error is Error in roxmltree src/parse.rs. It is one of all possible
// errors.
//
// Port: each case of the Rust enum is a sentinel error of the root package
// okage, such as okage.ErrXMLUnknownToken (D21). Error wraps that sentinel
// error with the data of the case, so errors.Is finds the case and errors.As
// finds the position. The message is "parsing xml: " and the Display text of
// upstream. Message gives the Display text alone.
type Error struct {
	kind okage.Error
	pos  TextPos
	// name is the name of DuplicatedNamespace, UnknownNamespace,
	// UnknownEntityReference and DuplicatedAttribute, the expected name of
	// UnexpectedCloseTag, the expected text of InvalidChar2 and
	// InvalidString, and the message of EntityResolver.
	name string
	// actual is the actual name of UnexpectedCloseTag.
	actual string
	// char is the char of NonXmlChar.
	char rune
	// expected is the expected byte of InvalidChar.
	expected byte
	// actualByte is the actual byte of InvalidChar and InvalidChar2.
	actualByte byte
	// cause is the error of the entity resolver, for EntityResolver.
	cause error
}

// errInvalidXMLPrefixURI is Error::InvalidXmlPrefixUri.
//
// The `xmlns:xml` attribute must have an <http://www.w3.org/XML/1998/namespace> URI.
func errInvalidXMLPrefixURI(pos TextPos) error {
	return &Error{kind: okage.ErrXMLInvalidXMLPrefixURI, pos: pos}
}

// errUnexpectedXMLURI is Error::UnexpectedXmlUri.
//
// Only the `xmlns:xml` attribute can have the <http://www.w3.org/XML/1998/namespace> URI.
func errUnexpectedXMLURI(pos TextPos) error {
	return &Error{kind: okage.ErrXMLUnexpectedXMLURI, pos: pos}
}

// errUnexpectedXMLNSURI is Error::UnexpectedXmlnsUri.
//
// The <http://www.w3.org/2000/xmlns/> URI must not be declared.
func errUnexpectedXMLNSURI(pos TextPos) error {
	return &Error{kind: okage.ErrXMLUnexpectedXMLNSURI, pos: pos}
}

// errInvalidElementNamePrefix is Error::InvalidElementNamePrefix.
//
// `xmlns` can't be used as an element prefix.
func errInvalidElementNamePrefix(pos TextPos) error {
	return &Error{kind: okage.ErrXMLInvalidElementNamePrefix, pos: pos}
}

// errDuplicatedNamespace is Error::DuplicatedNamespace.
//
// A namespace was already defined on this element.
func errDuplicatedNamespace(name string, pos TextPos) error {
	return &Error{kind: okage.ErrXMLDuplicatedNamespace, name: name, pos: pos}
}

// errUnknownNamespace is Error::UnknownNamespace.
//
// An unknown namespace.
//
// Indicates that an element or an attribute has an unknown qualified name prefix.
//
// The first value is a prefix.
func errUnknownNamespace(name string, pos TextPos) error {
	return &Error{kind: okage.ErrXMLUnknownNamespace, name: name, pos: pos}
}

// errUnexpectedCloseTag is Error::UnexpectedCloseTag.
//
// Incorrect tree structure.
//
// expected, actual, position.
func errUnexpectedCloseTag(expected, actual string, pos TextPos) error {
	return &Error{kind: okage.ErrXMLUnexpectedCloseTag, name: expected, actual: actual, pos: pos}
}

// errUnexpectedEntityCloseTag is Error::UnexpectedEntityCloseTag.
//
// Entity value starts with a close tag.
//
// Example:
//
//	<!DOCTYPE test [ <!ENTITY p '</p>'> ]>
//	<root>&p;</root>
func errUnexpectedEntityCloseTag(pos TextPos) error {
	return &Error{kind: okage.ErrXMLUnexpectedEntityCloseTag, pos: pos}
}

// errUnknownEntityReference is Error::UnknownEntityReference.
//
// A reference to an entity that was not defined in the DTD.
func errUnknownEntityReference(name string, pos TextPos) error {
	return &Error{kind: okage.ErrXMLUnknownEntityReference, name: name, pos: pos}
}

// errMalformedEntityReference is Error::MalformedEntityReference.
//
// A malformed entity reference.
//
// A `&` character inside an attribute value or text indicates an entity reference.
// Otherwise, the document is not well-formed.
func errMalformedEntityReference(pos TextPos) error {
	return &Error{kind: okage.ErrXMLMalformedEntityReference, pos: pos}
}

// errEntityReferenceLoop is Error::EntityReferenceLoop.
//
// A possible entity reference loop.
//
// The current depth limit is 10. The max number of references per reference is 255.
func errEntityReferenceLoop(pos TextPos) error {
	return &Error{kind: okage.ErrXMLEntityReferenceLoop, pos: pos}
}

// errInvalidAttributeValue is Error::InvalidAttributeValue.
//
// Attribute value cannot have a `<` character.
func errInvalidAttributeValue(pos TextPos) error {
	return &Error{kind: okage.ErrXMLInvalidAttributeValue, pos: pos}
}

// errDuplicatedAttribute is Error::DuplicatedAttribute.
//
// An element has a duplicated attributes.
//
// This also includes namespaces resolving.
// So an element like this will lead to an error.
//
//	<e xmlns:n1='http://www.w3.org' xmlns:n2='http://www.w3.org' n1:a='b1' n2:a='b2'/>
func errDuplicatedAttribute(name string, pos TextPos) error {
	return &Error{kind: okage.ErrXMLDuplicatedAttribute, name: name, pos: pos}
}

// errNoRootNode is Error::NoRootNode.
//
// The XML document must have at least one element.
func errNoRootNode() error {
	return &Error{kind: okage.ErrXMLNoRootNode, pos: NewTextPos(1, 1)}
}

// errUnclosedRootNode is Error::UnclosedRootNode.
//
// The root node was opened but never closed.
func errUnclosedRootNode() error {
	return &Error{kind: okage.ErrXMLUnclosedRootNode, pos: NewTextPos(1, 1)}
}

// errUnexpectedDeclaration is Error::UnexpectedDeclaration.
//
// An XML document can have only one XML declaration
// and it must be at the start of the document.
func errUnexpectedDeclaration(pos TextPos) error {
	return &Error{kind: okage.ErrXMLUnexpectedDeclaration, pos: pos}
}

// errDtdDetected is Error::DtdDetected.
//
// An XML with DTD detected.
//
// This error will be emitted only when `ParsingOptions::allow_dtd` is set to `false`.
func errDtdDetected() error {
	return &Error{kind: okage.ErrXMLDTDDetected, pos: NewTextPos(1, 1)}
}

// errNodesLimitReached is Error::NodesLimitReached.
//
// Indicates that the [`ParsingOptions::nodes_limit`] was reached.
func errNodesLimitReached() error {
	return &Error{kind: okage.ErrXMLNodesLimitReached, pos: NewTextPos(1, 1)}
}

// errAttributesLimitReached is Error::AttributesLimitReached.
//
// Indicates that too many attributes were parsed.
func errAttributesLimitReached() error {
	return &Error{kind: okage.ErrXMLAttributesLimitReached, pos: NewTextPos(1, 1)}
}

// errNamespacesLimitReached is Error::NamespacesLimitReached.
//
// Indicates that too many namespaces were parsed.
func errNamespacesLimitReached() error {
	return &Error{kind: okage.ErrXMLNamespacesLimitReached, pos: NewTextPos(1, 1)}
}

// errInvalidName is Error::InvalidName.
//
// An invalid name.
func errInvalidName(pos TextPos) error {
	return &Error{kind: okage.ErrXMLInvalidName, pos: pos}
}

// errNonXMLChar is Error::NonXmlChar.
//
// A non-XML character has occurred.
//
// Valid characters are: <https://www.w3.org/TR/xml/#char32>
func errNonXMLChar(c rune, pos TextPos) error {
	return &Error{kind: okage.ErrXMLNonXMLChar, char: c, pos: pos}
}

// errInvalidChar is Error::InvalidChar.
//
// An invalid/unexpected character.
//
// expected, actual, position.
func errInvalidChar(expected, actual byte, pos TextPos) error {
	return &Error{kind: okage.ErrXMLInvalidChar, expected: expected, actualByte: actual, pos: pos}
}

// errInvalidChar2 is Error::InvalidChar2.
//
// An invalid/unexpected character.
//
// expected, actual, position.
func errInvalidChar2(expected string, actual byte, pos TextPos) error {
	return &Error{kind: okage.ErrXMLInvalidChar2, name: expected, actualByte: actual, pos: pos}
}

// errInvalidString is Error::InvalidString.
//
// An unexpected string.
//
// Contains what string was expected.
func errInvalidString(expected string, pos TextPos) error {
	return &Error{kind: okage.ErrXMLInvalidString, name: expected, pos: pos}
}

// errInvalidExternalID is Error::InvalidExternalID.
//
// An invalid ExternalID in the DTD.
func errInvalidExternalID(pos TextPos) error {
	return &Error{kind: okage.ErrXMLInvalidExternalID, pos: pos}
}

// errEntityResolver is Error::EntityResolver.
//
// The given entity resolved yielded an errror.
//
// Port: upstream holds the message of the resolver as a String. Go keeps the
// error of the resolver too, so that errors.Is and errors.As find it.
func errEntityResolver(pos TextPos, cause error) error {
	return &Error{kind: okage.ErrXMLEntityResolver, pos: pos, name: cause.Error(), cause: cause}
}

// errInvalidComment is Error::InvalidComment.
//
// A comment cannot contain `--` or end with `-`.
func errInvalidComment(pos TextPos) error {
	return &Error{kind: okage.ErrXMLInvalidComment, pos: pos}
}

// errInvalidCharacterData is Error::InvalidCharacterData.
//
// A Character Data node contains an invalid data.
//
// Currently, only `]]>` is not allowed.
func errInvalidCharacterData(pos TextPos) error {
	return &Error{kind: okage.ErrXMLInvalidCharacterData, pos: pos}
}

// errUnknownToken is Error::UnknownToken.
//
// An unknown token.
func errUnknownToken(pos TextPos) error {
	return &Error{kind: okage.ErrXMLUnknownToken, pos: pos}
}

// errUnexpectedEndOfStream is Error::UnexpectedEndOfStream.
//
// The steam ended earlier than we expected.
//
// Should only appear on invalid input data.
func errUnexpectedEndOfStream() error {
	return &Error{kind: okage.ErrXMLUnexpectedEndOfStream, pos: NewTextPos(1, 1)}
}

// Pos is Error::pos in roxmltree src/parse.rs. It returns the error
// position.
func (e *Error) Pos() TextPos {
	return e.pos
}

// Message is core::fmt::Display for Error in roxmltree src/parse.rs. It
// returns the text that upstream displays.
func (e *Error) Message() string {
	pos := e.pos.String()
	switch e.kind {
	case okage.ErrXMLInvalidXMLPrefixURI:
		return "'xml' namespace prefix mapped to wrong URI at " + pos
	case okage.ErrXMLUnexpectedXMLURI:
		return "the 'xml' namespace URI is used for not 'xml' prefix at " + pos
	case okage.ErrXMLUnexpectedXMLNSURI:
		return "the 'xmlns' URI is used at " + pos + ", but it must not be declared"
	case okage.ErrXMLInvalidElementNamePrefix:
		return "the 'xmlns' prefix is used at " + pos + ", but it must not be"
	case okage.ErrXMLDuplicatedNamespace:
		return "namespace '" + e.name + "' at " + pos + " is already defined"
	case okage.ErrXMLUnknownNamespace:
		return "an unknown namespace prefix '" + e.name + "' at " + pos
	case okage.ErrXMLUnexpectedCloseTag:
		return "expected '" + e.name + "' tag, not '" + e.actual + "' at " + pos
	case okage.ErrXMLUnexpectedEntityCloseTag:
		return "unexpected close tag at " + pos
	case okage.ErrXMLMalformedEntityReference:
		return "malformed entity reference at " + pos
	case okage.ErrXMLUnknownEntityReference:
		return "unknown entity reference '" + e.name + "' at " + pos
	case okage.ErrXMLEntityReferenceLoop:
		return "a possible entity reference loop is detected at " + pos
	case okage.ErrXMLInvalidAttributeValue:
		return "unescaped '<' found at " + pos
	case okage.ErrXMLDuplicatedAttribute:
		return "attribute '" + e.name + "' at " + pos + " is already defined"
	case okage.ErrXMLNoRootNode:
		return "the document does not have a root node"
	case okage.ErrXMLUnclosedRootNode:
		return "the root node was opened but never closed"
	case okage.ErrXMLUnexpectedDeclaration:
		return "unexpected XML declaration at " + pos
	case okage.ErrXMLDTDDetected:
		return "XML with DTD detected"
	case okage.ErrXMLNodesLimitReached:
		return "nodes limit reached"
	case okage.ErrXMLAttributesLimitReached:
		return "more than 2^32 attributes were parsed"
	case okage.ErrXMLNamespacesLimitReached:
		return "more than 2^16 unique namespaces were parsed"
	case okage.ErrXMLInvalidName:
		return "invalid name token at " + pos
	case okage.ErrXMLNonXMLChar:
		return "a non-XML character " + rustDebugChar(e.char) + " found at " + pos
	case okage.ErrXMLInvalidChar:
		// Port: `as char` reads a byte as a char of Latin-1.
		return "expected '" + string(rune(e.expected)) + "' not '" + string(rune(e.actualByte)) + "' at " + pos
	case okage.ErrXMLInvalidChar2:
		return "expected " + e.name + " not '" + string(rune(e.actualByte)) + "' at " + pos
	case okage.ErrXMLInvalidString:
		return "expected '" + e.name + "' at " + pos
	case okage.ErrXMLInvalidExternalID:
		return "invalid ExternalID at " + pos
	case okage.ErrXMLEntityResolver:
		return "entity resolver failed at " + pos + ": " + e.name
	case okage.ErrXMLInvalidComment:
		return "comment at " + pos + " contains '--'"
	case okage.ErrXMLInvalidCharacterData:
		return "']]>' at " + pos + " is not allowed inside a character data"
	case okage.ErrXMLUnknownToken:
		return "unknown token at " + pos
	case okage.ErrXMLUnexpectedEndOfStream:
		return "unexpected end of stream"
	}
	return e.kind.Error()
}

// Error satisfies the error interface. The message is "parsing xml: " and
// the Display text of upstream (D21).
func (e *Error) Error() string {
	return "parsing xml: " + e.Message()
}

// Unwrap returns the sentinel error of the case, and the error of the entity
// resolver for EntityResolver.
func (e *Error) Unwrap() []error {
	if e.cause != nil {
		return []error{e.kind, e.cause}
	}
	return []error{e.kind}
}

// ParsingOptions is ParsingOptions in roxmltree src/parse.rs. It holds the
// parsing options.
//
// Port: the zero value is not the default of upstream, because NodesLimit
// is zero in it. Start from DefaultParsingOptions.
type ParsingOptions struct {
	// Allow DTD parsing.
	//
	// When set to `false`, XML with DTD will cause an error.
	// Empty DTD block is not an error.
	//
	// Currently, there is no option to simply skip DTD.
	// Mainly because you will get `UnknownEntityReference` error later anyway.
	//
	// This flag is set to `false` by default for security reasons,
	// but `roxmltree` still has checks for billion laughs attack,
	// so this is just an extra security measure.
	//
	// Default: false
	AllowDTD bool

	// Sets the maximum number of nodes to parse.
	//
	// Useful when dealing with random input to limit memory usage.
	//
	// Default: u32::MAX (no limit)
	NodesLimit uint32

	// Function to resolve external entities
	//
	// See [`EntityResolver`] for the signature
	// and the expected behaviour.
	//
	// Default: nil (None)
	EntityResolver EntityResolver
}

// EntityResolver is EntityResolver in roxmltree src/parse.rs. It is a
// function to resolve external entities.
//
// This function is passed the optional public ID
// and the mandatory URI of the external entity.
//
// It is expected to yield the content defining
// the URI, possibly preceded by a text declaration.
//
// If it yields [`None`], the entity is discarded
// and will yield errors if referenced during parsing.
//
// Errors must be stringified so they can be propagated
// via [`Error::EntityResolver`].
//
// Port: hasPubID is false when the public ID is None. The function returns
// ok false for None. A non-nil error becomes okage.ErrXMLEntityResolver.
type EntityResolver func(pubID string, hasPubID bool, uri string) (value string, ok bool, err error)

// DefaultParsingOptions is Default for ParsingOptions in roxmltree
// src/parse.rs.
func DefaultParsingOptions() ParsingOptions {
	return ParsingOptions{
		AllowDTD:       false,
		NodesLimit:     0xFFFF_FFFF,
		EntityResolver: nil,
	}
}

// String is fmt::Debug for ParsingOptions in roxmltree src/parse.rs.
func (o ParsingOptions) String() string {
	entityResolver := rustNone
	if o.EntityResolver != nil {
		entityResolver = "Some(..)"
	}

	allowDTD := "false"
	if o.AllowDTD {
		allowDTD = "true"
	}
	return "ParsingOptions { allow_dtd: " + allowDTD +
		", nodes_limit: " + strconv.FormatUint(uint64(o.NodesLimit), 10) +
		", entity_resolver: " + rustDebugStr(entityResolver) + " }"
}

// tempAttributeData is TempAttributeData in roxmltree src/parse.rs.
type tempAttributeData struct {
	prefix   string
	local    string
	value    string
	rng      Range
	qnameLen uint16
	eqLen    uint8
}

// Parse is Document::parse in roxmltree src/parse.rs. It parses the input
// XML string.
//
// We do not support `&[u8]` or `Reader` because the input must be an already allocated
// UTF-8 string.
//
// This is a shorthand for `Document::parse_with_options(data, ParsingOptions::default())`.
//
// Port: a Rust &str is always valid UTF-8, and the caller must check that
// text is, for example with utf8.ValidString. The parser reads a byte of
// invalid UTF-8 as U+FFFD.
func Parse(text string) (*Document, error) {
	return ParseWithOptions(text, DefaultParsingOptions())
}

// ParseWithOptions is Document::parse_with_options in roxmltree
// src/parse.rs. It parses the input XML string using to selected options.
//
// We do not support `&[u8]` or `Reader` because the input must be an already allocated
// UTF-8 string.
func ParseWithOptions(text string, opt ParsingOptions) (*Document, error) {
	return parse(text, opt)
}

// entity is Entity in roxmltree src/parse.rs.
type entity struct {
	name  string
	value strSpan
}

// tagNameSpan is TagNameSpan in roxmltree src/parse.rs.
type tagNameSpan struct {
	prefix    string
	name      string
	pos       int
	prefixPos int
}

// newNullTagNameSpan is TagNameSpan::new_null in roxmltree src/parse.rs.
func newNullTagNameSpan() tagNameSpan {
	return tagNameSpan{
		prefix:    "",
		name:      "",
		pos:       0,
		prefixPos: 0,
	}
}

// loopDetector is LoopDetector in roxmltree src/parse.rs. It is an entity
// loop detector.
//
// Limits:
//   - Entities depth is 10.
//   - Maximum number of entity references per entity reference is 255.
//
// Basically, if a text or an attribute has an entity reference and this reference
// has more than 10 nested references - this is an error.
//
// This is useful for simple loops like:
//
//	<!ENTITY a '&b;'>
//	<!ENTITY b '&a;'>
//
// And, if a text or an attribute has an entity reference and it references more
// than 255 references - this is an error.
//
// This is useful for cases like billion laughs attack, where depth can be pretty small,
// but the number of references is exponentially increasing:
//
//	<!ENTITY lol "lol">
//	<!ENTITY lol1 "&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;">
//	<!ENTITY lol2 "&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;">
//	<!ENTITY lol3 "&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;">
//	<!ENTITY lol4 "&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;">
type loopDetector struct {
	// References depth.
	depth uint8
	// Number of references resolved by the root reference.
	references uint8
}

// incDepth is LoopDetector::inc_depth in roxmltree src/parse.rs.
func (l *loopDetector) incDepth(stream *stream) error {
	if l.depth < 10 {
		l.depth++
		return nil
	}
	return errEntityReferenceLoop(stream.GenTextPos())
}

// decDepth is LoopDetector::dec_depth in roxmltree src/parse.rs.
func (l *loopDetector) decDepth() {
	if l.depth > 0 {
		l.depth--
	}

	// Reset references count after reaching zero depth.
	if l.depth == 0 {
		l.references = 0
	}
}

// incReferences is LoopDetector::inc_references in roxmltree src/parse.rs.
func (l *loopDetector) incReferences(stream *stream) error {
	if l.depth == 0 {
		// Allow infinite amount of references at zero depth.
		return nil
	}
	if l.references == 0xFF {
		return errEntityReferenceLoop(stream.GenTextPos())
	}

	l.references++
	return nil
}

// context is Context in roxmltree src/parse.rs.
type context struct {
	opt               ParsingOptions
	namespaceStartIdx int
	currentAttributes []tempAttributeData
	awaitingSubtree   []NodeID
	parentPrefixes    []string
	entities          []entity
	// afterText is the Rust Vec<Cow<str>>.
	afterText    []string
	parentID     NodeID
	tagName      tagNameSpan
	loopDetector loopDetector
	doc          Document
}

// appendNode is Context::append_node in roxmltree src/parse.rs.
func (ctx *context) appendNode(kind nodeKind, rng Range) (NodeID, error) {
	if uint64(len(ctx.doc.nodes)) >= uint64(ctx.opt.NodesLimit) {
		return 0, errNodesLimitReached()
	}

	newChildID := nodeIDFromUsize(len(ctx.doc.nodes))

	appendingElement := kind.typ == NodeTypeElement
	ctx.doc.nodes = append(ctx.doc.nodes, nodeData{
		parent:      someNodeID(ctx.parentID),
		prevSibling: 0,
		nextSubtree: 0,
		lastChild:   0,
		kind:        kind,
		rng:         rng,
	})

	lastChildID := ctx.doc.nodes[ctx.parentID.GetUsize()].lastChild
	ctx.doc.nodes[newChildID.GetUsize()].prevSibling = lastChildID
	ctx.doc.nodes[ctx.parentID.GetUsize()].lastChild = someNodeID(newChildID)

	for _, id := range ctx.awaitingSubtree {
		ctx.doc.nodes[id.GetUsize()].nextSubtree = someNodeID(newChildID)
	}
	ctx.awaitingSubtree = ctx.awaitingSubtree[:0]

	if !appendingElement {
		ctx.awaitingSubtree = append(ctx.awaitingSubtree, nodeIDFromUsize(len(ctx.doc.nodes)-1))
	}

	return newChildID, nil
}

// appendText is Context::append_text in roxmltree src/parse.rs.
func (ctx *context) appendText(text string, rng Range) error {
	if len(ctx.afterText) == 0 {
		if _, err := ctx.appendNode(nodeKind{typ: NodeTypeText, text: text}, rng); err != nil {
			return err
		}
	}

	ctx.afterText = append(ctx.afterText, text)
	return nil
}

// mergeText is Context::merge_text in roxmltree src/parse.rs.
func (ctx *context) mergeText() {
	node := &ctx.doc.nodes[len(ctx.doc.nodes)-1]

	if node.kind.typ != NodeTypeText {
		panic("internal error: entered unreachable code")
	}

	node.kind.text = strings.Join(ctx.afterText, "")
}

// resetAfterText is Context::reset_after_text in roxmltree src/parse.rs.
func (ctx *context) resetAfterText() {
	if len(ctx.afterText) == 0 {
		return
	}

	if len(ctx.afterText) > 1 {
		ctx.mergeText()
	}

	ctx.afterText = ctx.afterText[:0]
}

// parse is parse in roxmltree src/parse.rs.
func parse(text string, opt ParsingOptions) (*Document, error) {
	// Trying to guess rough nodes and attributes amount.
	nodesCapacity := strings.Count(text, "<")
	attributesCapacity := strings.Count(text, "=")

	// Init document.
	doc := Document{
		text:       text,
		nodes:      make([]nodeData, 0, nodesCapacity),
		attributes: make([]attributeData, 0, attributesCapacity),
		namespaces: namespaces{},
		serial:     documentSerial.Add(1),
	}

	// Add a root node.
	doc.nodes = append(doc.nodes, nodeData{
		parent:      0,
		prevSibling: 0,
		nextSubtree: 0,
		lastChild:   0,
		kind:        nodeKind{typ: NodeTypeRoot},
		rng:         Range{Start: 0, End: len(text)},
	})

	if err := doc.namespaces.pushNS(nsXMLPrefix, NSXMLURI); err != nil {
		return nil, err
	}

	allowDTD := opt.AllowDTD

	ctx := &context{
		opt:               opt,
		namespaceStartIdx: 1,
		currentAttributes: make([]tempAttributeData, 0, 16),
		entities:          nil,
		awaitingSubtree:   nil,
		parentPrefixes:    []string{""},
		afterText:         make([]string, 0, 1),
		parentID:          NewNodeID(0),
		tagName:           newNullTagNameSpan(),
		loopDetector:      loopDetector{},
		doc:               doc,
	}

	if err := parseTokenizer(text, allowDTD, ctx); err != nil {
		return nil, err
	}

	d := &ctx.doc
	hasElement := false
	for n := range d.Root().Children() {
		if n.IsElement() {
			hasElement = true
			break
		}
	}
	if !hasElement {
		return nil, errNoRootNode()
	}

	if len(ctx.parentPrefixes) > 1 {
		return nil, errUnclosedRootNode()
	}

	d.nodes = shrinkToFit(d.nodes)
	d.attributes = shrinkToFit(d.attributes)
	d.namespaces.shrinkToFit()

	return d, nil
}

// token is XmlEvents::token for Context in roxmltree src/parse.rs.
func (ctx *context) token(t token) error {
	switch t.kind {
	case tokenProcessingInstruction:
		ctx.resetAfterText()
		pi := nodeKind{typ: NodeTypePI, pi: PI{Target: t.prefix, Value: t.local}}
		if _, err := ctx.appendNode(pi, t.rng); err != nil {
			return err
		}
	case tokenComment:
		ctx.resetAfterText()
		if _, err := ctx.appendNode(nodeKind{typ: NodeTypeComment, text: t.local}, t.rng); err != nil {
			return err
		}
	case tokenEntityDeclaration:
		ctx.entities = append(ctx.entities, entity{
			name:  t.local,
			value: t.span,
		})
	case tokenElementStart:
		ctx.resetAfterText()

		if t.prefix == xmlns {
			pos := ctx.doc.TextPosAt(t.start + 1)
			return errInvalidElementNamePrefix(pos)
		}

		ctx.tagName = tagNameSpan{
			prefix:    t.prefix,
			name:      t.local,
			pos:       t.start,
			prefixPos: t.start + 1,
		}
	case tokenAttribute:
		if err := processAttribute(t.rng, t.qnameLen, t.eqLen, t.prefix, t.local, t.span, ctx); err != nil {
			return err
		}
	case tokenElementEnd:
		ctx.resetAfterText()
		if err := processElement(t.end, t.rng, ctx); err != nil {
			return err
		}
	case tokenText:
		if err := processText(t.local, t.rng, ctx); err != nil {
			return err
		}
	case tokenCdata:
		if err := processCdata(t.local, t.rng, ctx); err != nil {
			return err
		}
	}

	return nil
}

// resolveEntity is XmlEvents::resolve_entity for Context in roxmltree
// src/parse.rs.
func (ctx *context) resolveEntity(pubID string, hasPubID bool, uri string) (string, bool, error) {
	if ctx.opt.EntityResolver != nil {
		return ctx.opt.EntityResolver(pubID, hasPubID, uri)
	}
	return "", false, nil
}

// processAttribute is process_attribute in roxmltree src/parse.rs.
func processAttribute(
	rng Range,
	qnameLen uint16,
	eqLen uint8,
	prefix string,
	local string,
	span strSpan,
	ctx *context,
) error {
	value, err := normalizeAttribute(span, ctx)
	if err != nil {
		return err
	}

	switch {
	case prefix == xmlns:
		// The xmlns namespace MUST NOT be declared as the default namespace.
		if value == NSXMLNSURI {
			pos := ctx.doc.TextPosAt(rng.Start)
			return errUnexpectedXMLNSURI(pos)
		}

		isXMLNSURI := value == NSXMLURI

		// The prefix 'xml' is by definition bound to the namespace name
		// http://www.w3.org/XML/1998/namespace.
		// It MUST NOT be bound to any other namespace name.
		if local == nsXMLPrefix {
			if !isXMLNSURI {
				pos := ctx.doc.TextPosAt(rng.Start)
				return errInvalidXMLPrefixURI(pos)
			}
		} else {
			// The xml namespace MUST NOT be bound to a non-xml prefix.
			if isXMLNSURI {
				pos := ctx.doc.TextPosAt(rng.Start)
				return errUnexpectedXMLURI(pos)
			}
		}

		// Check for duplicated namespaces.
		if ctx.doc.namespaces.exists(ctx.namespaceStartIdx, local) {
			pos := ctx.doc.TextPosAt(rng.Start)
			return errDuplicatedNamespace(local, pos)
		}

		// Xml namespace should not be added to the namespaces.
		if !isXMLNSURI {
			if err := ctx.doc.namespaces.pushNS(local, value); err != nil {
				return err
			}
		}
	case local == xmlns:
		// The xml namespace MUST NOT be declared as the default namespace.
		if value == NSXMLURI {
			pos := ctx.doc.TextPosAt(rng.Start)
			return errUnexpectedXMLURI(pos)
		}

		// The xmlns namespace MUST NOT be declared as the default namespace.
		if value == NSXMLNSURI {
			pos := ctx.doc.TextPosAt(rng.Start)
			return errUnexpectedXMLNSURI(pos)
		}

		if err := ctx.doc.namespaces.pushNS("", value); err != nil {
			return err
		}
	default:
		ctx.currentAttributes = append(ctx.currentAttributes, tempAttributeData{
			prefix:   prefix,
			local:    local,
			value:    value,
			rng:      rng,
			qnameLen: qnameLen,
			eqLen:    eqLen,
		})
	}

	return nil
}

// processElement is process_element in roxmltree src/parse.rs.
func processElement(endToken elementEnd, tokenRange Range, ctx *context) error {
	if ctx.tagName.name == "" {
		// May occur in XML like this:
		// <!DOCTYPE test [ <!ENTITY p '</p>'> ]>
		// <root>&p;</root>

		if endToken.kind == elementEndClose {
			return errUnexpectedEntityCloseTag(ctx.doc.TextPosAt(tokenRange.Start))
		}
		panic("internal error: entered unreachable code: should be already checked by the tokenizer")
	}

	namespaces := ctx.resolveNamespaces()
	ctx.namespaceStartIdx = len(ctx.doc.namespaces.treeOrder)

	attributes, err := resolveAttributes(namespaces, ctx)
	if err != nil {
		return err
	}

	switch endToken.kind {
	case elementEndEmpty:
		tagNSIdx, hasTagNS, err := getNSIdxByPrefix(
			namespaces,
			ctx.tagName.prefixPos,
			ctx.tagName.prefix,
			&ctx.doc,
		)
		if err != nil {
			return err
		}
		newElementID, err := ctx.appendNode(
			nodeKind{
				typ: NodeTypeElement,
				tagName: expandedNameIndexed{
					namespaceIdx: tagNSIdx,
					hasNamespace: hasTagNS,
					localName:    ctx.tagName.name,
				},
				attributes: attributes,
				namespaces: namespaces,
			},
			Range{Start: ctx.tagName.pos, End: tokenRange.End},
		)
		if err != nil {
			return err
		}
		ctx.awaitingSubtree = append(ctx.awaitingSubtree, newElementID)
	case elementEndClose:
		prefix, local := endToken.prefix, endToken.local
		parentNode := &ctx.doc.nodes[ctx.parentID.GetUsize()]
		// should never panic as we start with the single prefix of the
		// root node and always push another one when changing the parent
		parentPrefix := ctx.parentPrefixes[len(ctx.parentPrefixes)-1]

		parentNode.rng.End = tokenRange.End

		if parentNode.kind.typ == NodeTypeElement {
			tagName := parentNode.kind.tagName
			if prefix != parentPrefix || local != tagName.localName {
				return errUnexpectedCloseTag(
					genQNameString(parentPrefix, tagName.localName),
					genQNameString(prefix, local),
					ctx.doc.TextPosAt(tokenRange.Start),
				)
			}
		}
		ctx.awaitingSubtree = append(ctx.awaitingSubtree, ctx.parentID)

		if id, ok := parentNode.parent.get(); ok {
			ctx.parentID = id
			ctx.parentPrefixes = ctx.parentPrefixes[:len(ctx.parentPrefixes)-1]
		} else {
			// May occur in XML like this:
			// <!DOCTYPE test [ <!ENTITY p '<p></p></p>'> ]>
			// <p>&p;&p;

			return errUnexpectedEntityCloseTag(ctx.doc.TextPosAt(tokenRange.Start))
		}
	case elementEndOpen:
		tagNSIdx, hasTagNS, err := getNSIdxByPrefix(
			namespaces,
			ctx.tagName.prefixPos,
			ctx.tagName.prefix,
			&ctx.doc,
		)
		if err != nil {
			return err
		}
		ctx.parentID, err = ctx.appendNode(
			nodeKind{
				typ: NodeTypeElement,
				tagName: expandedNameIndexed{
					namespaceIdx: tagNSIdx,
					hasNamespace: hasTagNS,
					localName:    ctx.tagName.name,
				},
				attributes: attributes,
				namespaces: namespaces,
			},
			Range{Start: ctx.tagName.pos, End: tokenRange.End},
		)
		if err != nil {
			return err
		}
		ctx.parentPrefixes = append(ctx.parentPrefixes, ctx.tagName.prefix)
	}

	return nil
}

// resolveNamespaces is Context::resolve_namespaces in roxmltree
// src/parse.rs.
func (ctx *context) resolveNamespaces() shortRange {
	parent := &ctx.doc.nodes[ctx.parentID.GetUsize()]
	if parent.kind.typ == NodeTypeElement {
		parentNS := parent.kind.namespaces
		if ctx.namespaceStartIdx == len(ctx.doc.namespaces.treeOrder) {
			return parentNS
		}

		start, end := parentNS.toURange()
		for i := start; i < end; i++ {
			if !ctx.doc.namespaces.exists(
				ctx.namespaceStartIdx,
				ctx.doc.namespaces.get(ctx.doc.namespaces.treeOrder[i]).name,
			) {
				ctx.doc.namespaces.pushRef(i)
			}
		}
	}

	return shortRangeFrom(ctx.namespaceStartIdx, len(ctx.doc.namespaces.treeOrder))
}

// resolveAttributes is resolve_attributes in roxmltree src/parse.rs.
func resolveAttributes(namespaces shortRange, ctx *context) (shortRange, error) {
	if len(ctx.currentAttributes) == 0 {
		return newShortRange(0, 0), nil
	}

	if uint64(len(ctx.doc.attributes))+uint64(len(ctx.currentAttributes)) >= 0xFFFF_FFFF {
		return shortRange{}, errAttributesLimitReached()
	}

	startIdx := len(ctx.doc.attributes)

	// Port: drain(..) empties the list, also when an error returns early.
	defer func() { ctx.currentAttributes = ctx.currentAttributes[:0] }()
	for i := range ctx.currentAttributes {
		attr := &ctx.currentAttributes[i]
		var nsIdx namespaceIdx
		var hasNS bool
		switch attr.prefix {
		case nsXMLPrefix:
			// The prefix 'xml' is by definition bound to the namespace name
			// http://www.w3.org/XML/1998/namespace. This namespace is added
			// to the document on creation and is always element 0.
			nsIdx, hasNS = 0, true
		case "":
			// 'The namespace name for an unprefixed attribute name
			// always has no value.'
		default:
			var err error
			nsIdx, hasNS, err = getNSIdxByPrefix(namespaces, attr.rng.Start, attr.prefix, &ctx.doc)
			if err != nil {
				return shortRange{}, err
			}
		}

		attrName := expandedNameIndexed{
			namespaceIdx: nsIdx,
			hasNamespace: hasNS,
			localName:    attr.local,
		}

		// Check for duplicated attributes.
		for j := startIdx; j < len(ctx.doc.attributes); j++ {
			if ctx.doc.attributes[j].name.asExpandedName(&ctx.doc) == attrName.asExpandedName(&ctx.doc) {
				pos := ctx.doc.TextPosAt(attr.rng.Start)
				return shortRange{}, errDuplicatedAttribute(attr.local, pos)
			}
		}

		ctx.doc.attributes = append(ctx.doc.attributes, attributeData{
			name:     attrName,
			value:    attr.value,
			rng:      attr.rng,
			qnameLen: attr.qnameLen,
			eqLen:    attr.eqLen,
		})
	}

	return shortRangeFrom(startIdx, len(ctx.doc.attributes)), nil
}

// processText is process_text in roxmltree src/parse.rs.
func processText(text string, rng Range, ctx *context) error {
	// Add text as is if it has only valid characters.
	if strings.IndexByte(text, '&') < 0 && strings.IndexByte(text, '\r') < 0 {
		return ctx.appendText(text, rng)
	}

	textBuffer := newTextBuffer()
	isAsIs := false // TODO: explain
	stream := streamFromSubstr(ctx.doc.text, rng)
	for !stream.AtEnd() {
		chunk, err := parseNextChunk(&stream, ctx.entities)
		if err != nil {
			return err
		}
		switch chunk.kind {
		case nextChunkByte:
			if isAsIs {
				textBuffer.pushRaw(chunk.b)
				isAsIs = false
			} else {
				textBuffer.pushFromText(chunk.b, stream.AtEnd())
			}
		case nextChunkChar:
			bytes := newCharToBytes(chunk.c)
			for b, ok := bytes.next(); ok; b, ok = bytes.next() {
				if ctx.loopDetector.depth > 0 {
					textBuffer.pushFromText(b, stream.AtEnd())
				} else {
					// Characters not from entity should be added as is.
					// Not sure why... At least `lxml` produces the same result.
					textBuffer.pushRaw(b)
					isAsIs = true
				}
			}
		case nextChunkText:
			fragment := chunk.text
			isAsIs = false

			if !textBuffer.isEmpty() {
				if err := ctx.appendText(textBuffer.finish(), rng); err != nil {
					return err
				}
			}

			if err := ctx.loopDetector.incReferences(&stream); err != nil {
				return err
			}
			if err := ctx.loopDetector.incDepth(&stream); err != nil {
				return err
			}

			text := ctx.doc.text
			if fragment.Range().Start == 0 {
				text = fragment.AsStr()
			}

			stream := streamFromSubstr(text, fragment.Range())
			prevTagName := ctx.tagName
			ctx.tagName = newNullTagNameSpan()
			if err := parseContent(&stream, ctx); err != nil {
				return err
			}
			ctx.tagName = prevTagName
			textBuffer.clear()

			ctx.loopDetector.decDepth()
		}
	}

	if !textBuffer.isEmpty() {
		return ctx.appendText(textBuffer.finish(), rng)
	}

	return nil
}

// processCdata is process_cdata in roxmltree src/parse.rs.
//
// While the whole purpose of CDATA is to indicate to an XML library that this text
// has to be stored as is, carriage return (`\r`) is still has to be replaced with `\n`.
func processCdata(text string, rng Range, ctx *context) error {
	pos := strings.IndexByte(text, '\r')

	// Add text as is if it has only valid characters.
	if pos < 0 {
		return ctx.appendText(text, rng)
	}

	var buf strings.Builder

	for pos >= 0 {
		line, rest := text[:pos], text[pos:]

		buf.WriteString(line)
		buf.WriteByte('\n')

		if len(rest) > 1 && rest[1] == '\n' {
			text = rest[2:]
		} else {
			text = rest[1:]
		}

		pos = strings.IndexByte(text, '\r')
	}

	buf.WriteString(text)

	return ctx.appendText(buf.String(), rng)
}

// nextChunkKind is the kind of the enum NextChunk in roxmltree
// src/parse.rs.
type nextChunkKind uint8

// The kinds of NextChunk.
const (
	nextChunkByte nextChunkKind = iota
	nextChunkChar
	nextChunkText
)

// nextChunk is NextChunk in roxmltree src/parse.rs. Byte uses b, Char uses c
// and Text uses text (D27).
type nextChunk struct {
	kind nextChunkKind
	b    byte
	c    rune
	text strSpan
}

// parseNextChunk is parse_next_chunk in roxmltree src/parse.rs.
func parseNextChunk(stream *stream, entities []entity) (nextChunk, error) {
	// Safe, because we already checked that stream is not at the end.
	// But we have an additional `debug_assert` above just in case.
	c := stream.CurrByteUnchecked()

	// Check for character/entity references.
	if c == '&' {
		start := stream.Pos()
		ref, ok := stream.ConsumeReference()
		if !ok {
			pos := stream.GenTextPosFrom(start)
			return nextChunk{}, errMalformedEntityReference(pos)
		}
		if ref.kind == referenceChar {
			return nextChunk{kind: nextChunkChar, c: ref.char}, nil
		}
		for i := range entities {
			if entities[i].name == ref.entity {
				return nextChunk{kind: nextChunkText, text: entities[i].value}, nil
			}
		}
		pos := stream.GenTextPosFrom(start)
		return nextChunk{}, errUnknownEntityReference(ref.entity, pos)
	}
	stream.Advance(1)
	return nextChunk{kind: nextChunkByte, b: c}, nil
}

// normalizeAttribute is normalize_attribute in roxmltree src/parse.rs.
//
// https://www.w3.org/TR/REC-xml/#AVNormalize
func normalizeAttribute(text strSpan, ctx *context) (string, error) {
	// We assume that `&` indicates an entity or a character reference.
	// But in rare cases it can be just an another character.
	if strings.ContainsAny(text.AsStr(), "&\t\n\r") {
		textBuffer := newTextBuffer()
		if err := normalizeAttributeInner(text, &textBuffer, ctx); err != nil {
			return "", err
		}
		return textBuffer.finish(), nil
	}
	return text.AsStr(), nil
}

// normalizeAttributeInner is _normalize_attribute in roxmltree src/parse.rs.
//
// Port: upstream reads the range of text in the input text, also for the
// value of an entity that the entity resolver gave. Go does the same.
func normalizeAttributeInner(text strSpan, buffer *textBuffer, ctx *context) error {
	stream := streamFromSubstr(ctx.doc.text, text.Range())
	for !stream.AtEnd() {
		// Safe, because we already checked that the stream is not at the end.
		c := stream.CurrByteUnchecked()

		if c != '&' {
			stream.Advance(1)
			next, err := stream.CurrByte()
			buffer.pushFromAttr(c, next, err == nil)
			continue
		}

		// Check for character/entity references.
		start := stream.Pos()
		ref, ok := stream.ConsumeReference()
		if !ok {
			pos := stream.GenTextPosFrom(start)
			return errMalformedEntityReference(pos)
		}
		switch ref.kind {
		case referenceChar:
			bytes := newCharToBytes(ref.char)
			for b, ok := bytes.next(); ok; b, ok = bytes.next() {
				if ctx.loopDetector.depth > 0 {
					// Escaped `<` inside an ENTITY is an error.
					// Escaped `<` outside an ENTITY is ok.
					if b == '<' {
						return errInvalidAttributeValue(stream.GenTextPosFrom(start))
					}

					buffer.pushFromAttr(b, 0, false)
				} else {
					// Characters not from entity should be added as is.
					// Not sure why... At least `lxml` produces the same results.
					buffer.pushRaw(b)
				}
			}
		case referenceEntity:
			found := -1
			for i := range ctx.entities {
				if ctx.entities[i].name == ref.entity {
					found = i
					break
				}
			}
			if found < 0 {
				pos := stream.GenTextPosFrom(start)
				return errUnknownEntityReference(ref.entity, pos)
			}
			if err := ctx.loopDetector.incReferences(&stream); err != nil {
				return err
			}
			if err := ctx.loopDetector.incDepth(&stream); err != nil {
				return err
			}
			if err := normalizeAttributeInner(ctx.entities[found].value, buffer, ctx); err != nil {
				return err
			}
			ctx.loopDetector.decDepth()
		}
	}

	return nil
}

// getNSIdxByPrefix is get_ns_idx_by_prefix in roxmltree src/parse.rs. ok is
// false for None.
func getNSIdxByPrefix(
	namespaces shortRange,
	prefixPos int,
	prefix string,
	doc *Document,
) (idx namespaceIdx, ok bool, err error) {
	// Prefix CAN be empty when the default namespace was defined.
	//
	// Example:
	// <e xmlns='http://www.w3.org'/>
	//
	// Port: an empty prefix is the None of prefix_opt.
	prefixOpt := prefix

	start, end := namespaces.toURange()
	for _, i := range doc.namespaces.treeOrder[start:end] {
		if doc.namespaces.get(i).name == prefixOpt {
			return i, true, nil
		}
	}

	if prefix != "" {
		// If an URI was not found and prefix IS NOT empty than
		// we have an unknown namespace.
		//
		// Example:
		// <e random:a='b'/>
		pos := doc.TextPosAt(prefixPos)
		return 0, false, errUnknownNamespace(prefix, pos)
	}
	// If an URI was not found and prefix IS empty than
	// an element or an attribute doesn't have a namespace.
	//
	// Example:
	// <e a='b'/>
	return 0, false, nil
}

// genQNameString is gen_qname_string in roxmltree src/parse.rs.
func genQNameString(prefix, local string) string {
	if prefix == "" {
		return local
	}
	return prefix + ":" + local
}

// charToBytes is CharToBytes in roxmltree src/parse.rs. It iterates over a
// `char` by `u8`.
type charToBytes struct {
	buf [4]byte
	idx uint8
}

// newCharToBytes is CharToBytes::new in roxmltree src/parse.rs.
func newCharToBytes(c rune) charToBytes {
	buf := [4]byte{0xFF, 0xFF, 0xFF, 0xFF}
	utf8.EncodeRune(buf[:], c)

	return charToBytes{buf: buf, idx: 0}
}

// next is Iterator::next for CharToBytes in roxmltree src/parse.rs.
func (c *charToBytes) next() (byte, bool) {
	if c.idx < 4 {
		b := c.buf[c.idx]

		if b != 0xFF {
			c.idx++
			return b, true
		}
		c.idx = 4
	}

	return 0, false
}

// textBuffer is TextBuffer in roxmltree src/parse.rs.
type textBuffer struct {
	buffer []byte
}

// newTextBuffer is TextBuffer::new in roxmltree src/parse.rs.
func newTextBuffer() textBuffer {
	return textBuffer{
		buffer: make([]byte, 0, 32),
	}
}

// pushRaw is TextBuffer::push_raw in roxmltree src/parse.rs.
func (t *textBuffer) pushRaw(c byte) {
	t.buffer = append(t.buffer, c)
}

// pushFromAttr is TextBuffer::push_from_attr in roxmltree src/parse.rs.
// hasNext is false when next is None.
func (t *textBuffer) pushFromAttr(current byte, next byte, hasNext bool) {
	// \r in \r\n should be ignored.
	if current == '\r' && hasNext && next == '\n' {
		return
	}

	// \n, \r and \t should be converted into spaces.
	switch current {
	case '\n', '\r', '\t':
		current = ' '
	}

	t.buffer = append(t.buffer, current)
}

// pushFromText is TextBuffer::push_from_text in roxmltree src/parse.rs.
//
// Translate \r\n and any \r that is not followed by \n into a single \n character.
//
// https://www.w3.org/TR/xml/#sec-line-ends
func (t *textBuffer) pushFromText(c byte, atEnd bool) {
	switch {
	case len(t.buffer) > 0 && t.buffer[len(t.buffer)-1] == '\r':
		idx := len(t.buffer) - 1
		t.buffer[idx] = '\n'

		if atEnd && c == '\r' {
			t.buffer = append(t.buffer, '\n')
		} else if c != '\n' {
			t.buffer = append(t.buffer, c)
		}
	case atEnd && c == '\r':
		t.buffer = append(t.buffer, '\n')
	default:
		t.buffer = append(t.buffer, c)
	}
}

// clear is TextBuffer::clear in roxmltree src/parse.rs.
func (t *textBuffer) clear() {
	t.buffer = t.buffer[:0]
}

// isEmpty is TextBuffer::is_empty in roxmltree src/parse.rs.
func (t *textBuffer) isEmpty() bool {
	return len(t.buffer) == 0
}

// finish is TextBuffer::finish in roxmltree src/parse.rs.
func (t *textBuffer) finish() string {
	// `unwrap` is safe, because buffer must contain a valid UTF-8 string.
	s := string(t.buffer)
	t.buffer = nil
	return s
}
