// Copyright (c) 2006-2013 James Graham and other contributors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package html

// This file ports tinyhtml5 parser.py (D49). Each phase of upstream is a
// type that implements the interface phase, and its dispatch tables become a
// switch on the name of the tag. An assert of upstream has no port. Where
// upstream raises an error, such as ValueError for a start tag in the text
// phase, the port adds an issue and goes on, so that no input stops the
// parse.

import (
	"slices"
	"strconv"
	"strings"
)

// htmlParser is HTMLParser in tinyhtml5 parser.py. It generates a tree
// structure from a stream of HTML, which can be malformed.
type htmlParser struct {
	tree      *treeBuilder
	tokenizer *htmlTokenizer
	opts      *documentOptions
	phases    phases

	container string

	phase         phase
	originalPhase phase

	firstStartTag     bool
	framesetOK        bool
	compatibilityMode QuirksMode

	issues []Issue

	// reparse is true when the encoding changed, where upstream raises
	// ReparseError.
	reparse bool

	// stopped is true when the parse stopped at a limit of the options.
	stopped bool
}

// phases holds the phases of the parser, as the dict phases of HTMLParser in
// tinyhtml5 parser.py does.
type phases struct {
	initial            *initialPhase
	beforeHTML         *beforeHTMLPhase
	beforeHead         *beforeHeadPhase
	inHead             *inHeadPhase
	inHeadNoscript     *inHeadNoscriptPhase
	afterHead          *afterHeadPhase
	inBody             *inBodyPhase
	text               *textPhase
	inTable            *inTablePhase
	inTableText        *inTableTextPhase
	inCaption          *inCaptionPhase
	inColumnGroup      *inColumnGroupPhase
	inTableBody        *inTableBodyPhase
	inRow              *inRowPhase
	inCell             *inCellPhase
	inSelect           *inSelectPhase
	inSelectInTable    *inSelectInTablePhase
	inForeignContent   *inForeignContentPhase
	afterBody          *afterBodyPhase
	inFrameset         *inFramesetPhase
	afterFrameset      *afterFramesetPhase
	afterAfterBody     *afterAfterBodyPhase
	afterAfterFrameset *afterAfterFramesetPhase
}

// newHTMLParser is HTMLParser.__init__ in tinyhtml5 parser.py.
func newHTMLParser(opts *documentOptions) *htmlParser {
	p := &htmlParser{tree: newTreeBuilder(), opts: opts}
	b := phaseBase{p: p, tree: p.tree}
	p.phases = phases{
		initial:            &initialPhase{b},
		beforeHTML:         &beforeHTMLPhase{b},
		beforeHead:         &beforeHeadPhase{b},
		inHead:             &inHeadPhase{b},
		inHeadNoscript:     &inHeadNoscriptPhase{b},
		afterHead:          &afterHeadPhase{b},
		inBody:             &inBodyPhase{phaseBase: b},
		text:               &textPhase{b},
		inTable:            &inTablePhase{b},
		inTableText:        &inTableTextPhase{phaseBase: b},
		inCaption:          &inCaptionPhase{b},
		inColumnGroup:      &inColumnGroupPhase{b},
		inTableBody:        &inTableBodyPhase{b},
		inRow:              &inRowPhase{b},
		inCell:             &inCellPhase{b},
		inSelect:           &inSelectPhase{b},
		inSelectInTable:    &inSelectInTablePhase{b},
		inForeignContent:   &inForeignContentPhase{b},
		afterBody:          &afterBodyPhase{b},
		inFrameset:         &inFramesetPhase{b},
		afterFrameset:      &afterFramesetPhase{b},
		afterAfterBody:     &afterAfterBodyPhase{b},
		afterAfterFrameset: &afterAfterFramesetPhase{b},
	}
	return p
}

// parseStream is HTMLParser._parse in tinyhtml5 parser.py. container is the
// name of the container of a fragment, or empty for a document.
func (p *htmlParser) parseStream(stream *htmlInputStream, container string) {
	p.container = container
	p.tokenizer = newHTMLTokenizer(stream, p)
	p.reset()
	if !p.mainLoop() {
		p.reset()
		p.mainLoop()
	}
}

// reset is HTMLParser.reset in tinyhtml5 parser.py.
func (p *htmlParser) reset() {
	p.tree.reset()
	p.firstStartTag = false
	p.issues = nil
	p.reparse = false
	p.stopped = false
	p.compatibilityMode = NoQuirks

	if p.container != "" {
		switch {
		case cdataElement(p.container):
			p.tokenizer.state = rcdataState
		case rcdataElement(p.container):
			p.tokenizer.state = rawtextState
		case p.container == "plaintext":
			p.tokenizer.state = plaintextState
		default:
			// State already is data state.
			p.tokenizer.state = dataState
		}
		p.phase = p.phases.beforeHTML
		p.phases.beforeHTML.insertHTMLElement()
		p.resetInsertionMode()
	} else {
		p.tokenizer.state = dataState
		p.phase = p.phases.initial
	}

	p.originalPhase = nil

	p.framesetOK = true
}

// isHTMLIntegrationPoint is HTMLParser.is_html_integration_point in
// tinyhtml5 parser.py.
func (p *htmlParser) isHTMLIntegrationPoint(element *Node) bool {
	if element.namespace == NamespaceMathML && element.name == "annotation-xml" {
		encoding, ok := element.Attr("encoding")
		if !ok {
			return false
		}
		encoding = asciiUpperToLower(encoding)
		return encoding == "text/html" || encoding == "application/xhtml+xml"
	}
	return htmlIntegrationPointElement(element.namespace, element.name)
}

// isMathMLTextIntegrationPoint is
// HTMLParser.is_mathml_text_integration_point in tinyhtml5 parser.py.
func (p *htmlParser) isMathMLTextIntegrationPoint(element *Node) bool {
	return mathMLTextIntegrationPointElement(element.namespace, element.name)
}

// maxEOFPhases bounds the number of phases that process the end of the
// input. Upstream asserts that no phase comes twice, so the true number is
// at most the number of phases.
const maxEOFPhases = 32

// mainLoop is HTMLParser.main_loop in tinyhtml5 parser.py. It returns false
// when the encoding changed and the parse must start again.
func (p *htmlParser) mainLoop() bool {
	for tok := range p.tokenizer.tokens() {
		var previousToken *token
		newToken := tok
		var typ tokenType
		for newToken != nil {
			previousToken = newToken
			var currentNode *Node
			if len(p.tree.openElements) > 0 {
				currentNode = p.tree.currentNode()
			}

			typ = newToken.typ

			if typ == tokenParseError {
				p.parseError(newToken.data, newToken.datavars...)
				newToken = nil
				continue
			}
			var ph phase = p.phases.inForeignContent
			if currentNode == nil ||
				currentNode.namespace == p.tree.defaultNamespace ||
				(p.isMathMLTextIntegrationPoint(currentNode) &&
					((typ == tokenStartTag && tok.name != "mglyph" && tok.name != "malignmark") ||
						typ == tokenCharacters || typ == tokenSpaceCharacters)) ||
				(currentNode.namespace == NamespaceMathML &&
					currentNode.name == "annotation-xml" &&
					typ == tokenStartTag &&
					tok.name == "svg") ||
				(p.isHTMLIntegrationPoint(currentNode) &&
					(typ == tokenStartTag || typ == tokenCharacters || typ == tokenSpaceCharacters)) {
				ph = p.phase
			}

			switch typ {
			case tokenCharacters:
				newToken = ph.processCharacters(newToken)
			case tokenSpaceCharacters:
				newToken = ph.processSpaceCharacters(newToken)
			case tokenStartTag:
				newToken = ph.processStartTag(newToken)
			case tokenEndTag:
				newToken = ph.processEndTag(newToken)
			case tokenComment:
				newToken = ph.processComment(newToken)
			case tokenDoctype:
				newToken = ph.processDoctype(newToken)
			default:
				newToken = nil
			}
			if p.reparse {
				return false
			}
		}

		if typ == tokenStartTag && previousToken.selfClosing &&
			!previousToken.selfClosingAcknowledged {
			p.parseError("non-void-element-with-trailing-solidus", "name", previousToken.name)
		}

		if p.overLimit() {
			break
		}
	}

	// When the loop finishes it's EOF.
	reprocess := true
	for i := 0; reprocess && i < maxEOFPhases; i++ {
		reprocess = p.phase.processEOF()
	}
	return true
}

// overLimit says if the tree passed a limit of the options. The first time,
// it adds an issue and stops the parse.
func (p *htmlParser) overLimit() bool {
	switch {
	case p.stopped:
		return true
	case p.tree.nodes > p.opts.maxNodes:
		p.limitIssue("too-many-nodes", p.opts.maxNodes)
	case len(p.tree.openElements) > p.opts.maxDepth:
		p.limitIssue("too-deep", p.opts.maxDepth)
	default:
		return false
	}
	p.stopped = true
	return true
}

// limitIssue adds the issue of a limit, even past the limit of the issues.
func (p *htmlParser) limitIssue(code string, limit int) {
	line, column := p.tokenizer.stream.position()
	p.issues = append(p.issues, Issue{
		Pos: Pos{Line: line, Column: column + 1},
		Msg: code + " (limit: " + strconv.Itoa(limit) + "): the parse stopped here",
	})
}

// document is HTMLParser.parse and HTMLParser.parse_fragment in tinyhtml5
// parser.py, after the parse. It returns the whole tree, or the fragment.
func (p *htmlParser) document(fragment bool) *Document {
	root := p.tree.document
	if fragment {
		root = p.tree.getFragment()
	}
	finishText(root)
	return &Document{
		root:     root,
		encoding: p.tokenizer.stream.decoded,
		mode:     p.compatibilityMode,
		issues:   p.issues,
	}
}

// parseError is HTMLParser.parse_error in tinyhtml5 parser.py. datavars
// holds names and values in turn. It keeps at most the issues of
// WithMaxIssues.
func (p *htmlParser) parseError(errorcode string, datavars ...string) {
	if len(p.issues) >= p.opts.maxIssues {
		return
	}
	line, column := p.tokenizer.stream.position()
	msg := errorcode
	if len(datavars) > 1 {
		var b strings.Builder
		b.WriteString(errorcode)
		b.WriteString(" (")
		for i := 0; i+1 < len(datavars); i += 2 {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(datavars[i])
			b.WriteString(": ")
			b.WriteString(strconv.Quote(datavars[i+1]))
		}
		b.WriteString(")")
		msg = b.String()
	}
	p.issues = append(p.issues, Issue{Pos: Pos{Line: line, Column: column + 1}, Msg: msg})
}

// adjustMathMLAttributes is HTMLParser.adjust_mathml_attributes in
// tinyhtml5 parser.py.
func (p *htmlParser) adjustMathMLAttributes(tok *token) {
	for i, a := range tok.attrs {
		if a.Namespace == "" {
			if name, ok := adjustMathMLAttribute(a.Name); ok {
				tok.attrs[i].Name = name
			}
		}
	}
}

// adjustSVGAttributes is HTMLParser.adjust_svg_attributes in tinyhtml5
// parser.py.
func (p *htmlParser) adjustSVGAttributes(tok *token) {
	for i, a := range tok.attrs {
		if a.Namespace == "" {
			if name, ok := adjustSVGAttribute(a.Name); ok {
				tok.attrs[i].Name = name
			}
		}
	}
}

// adjustForeignAttributes is HTMLParser.adjust_foreign_attributes in
// tinyhtml5 parser.py.
func (p *htmlParser) adjustForeignAttributes(tok *token) {
	for i, a := range tok.attrs {
		if a.Namespace == "" {
			if ns, local, ok := adjustForeignAttribute(a.Name); ok {
				tok.attrs[i].Namespace = ns
				tok.attrs[i].Name = local
			}
		}
	}
}

// resetInsertionMode is HTMLParser.reset_insertion_mode in tinyhtml5
// parser.py. The name of this method is mostly historical. (It's also used
// in the specification.)
func (p *htmlParser) resetInsertionMode() {
	last := false
	var newPhase phase
	for i, node := range slices.Backward(p.tree.openElements) {
		nodeName := node.name
		newPhase = nil
		if i == 0 {
			last = true
			nodeName = p.container
		}

		if !last && node.namespace != p.tree.defaultNamespace {
			continue
		}

		newPhase = p.newMode(nodeName)
		if newPhase != nil {
			break
		}
		if last {
			newPhase = p.phases.inBody
			break
		}
	}

	if newPhase == nil {
		newPhase = p.phases.inBody
	}
	p.phase = newPhase
}

// newMode is the dict new_modes of HTMLParser.reset_insertion_mode in
// tinyhtml5 parser.py.
func (p *htmlParser) newMode(name string) phase {
	switch name {
	case "select":
		return p.phases.inSelect
	case "td", "th":
		return p.phases.inCell
	case "tr":
		return p.phases.inRow
	case "tbody", "thead", "tfoot":
		return p.phases.inTableBody
	case "caption":
		return p.phases.inCaption
	case "colgroup":
		return p.phases.inColumnGroup
	case "table":
		return p.phases.inTable
	case "head", "body":
		return p.phases.inBody
	case "frameset":
		return p.phases.inFrameset
	case "html":
		return p.phases.beforeHead
	}
	return nil
}

// parseRcdataRawtext is HTMLParser.parse_rcdata_rawtext in tinyhtml5
// parser.py, the generic RCDATA and RAWTEXT parsing algorithm.
func (p *htmlParser) parseRcdataRawtext(tok *token, rawtext bool) {
	p.tree.insertElement(tok)

	if rawtext {
		p.tokenizer.state = rawtextState
	} else {
		p.tokenizer.state = rcdataState
	}

	p.originalPhase = p.phase

	p.phase = p.phases.text
}

// phase is a phase of the parser: a subclass of Phase in tinyhtml5
// parser.py. A method returns a token for the parser to process again, or
// nil.
type phase interface {
	processEOF() bool
	processComment(tok *token) *token
	processDoctype(tok *token) *token
	processCharacters(tok *token) *token
	processSpaceCharacters(tok *token) *token
	processStartTag(tok *token) *token
	processEndTag(tok *token) *token
}

// phaseBase is Phase in tinyhtml5 parser.py, the base of each phase.
type phaseBase struct {
	p    *htmlParser
	tree *treeBuilder
}

// processComment is Phase.process_comment in tinyhtml5 parser.py. For most
// phases the following is correct. Where it's not it will be overridden.
func (b *phaseBase) processComment(tok *token) *token {
	b.tree.insertComment(tok, b.tree.currentNode())
	return nil
}

// processDoctype is Phase.process_doctype in tinyhtml5 parser.py.
//
//nolint:unparam // It implements phase, whose methods return a token.
func (b *phaseBase) processDoctype(*token) *token {
	b.p.parseError("unexpected-doctype")
	return nil
}

// processCharacters is Phase.process_characters in tinyhtml5 parser.py.
func (b *phaseBase) processCharacters(tok *token) *token {
	b.tree.insertText(tok.data)
	return nil
}

// processSpaceCharacters is Phase.process_space_characters in tinyhtml5
// parser.py.
func (b *phaseBase) processSpaceCharacters(tok *token) *token {
	b.tree.insertText(tok.data)
	return nil
}

// startTagHTML is Phase.start_tag_html in tinyhtml5 parser.py.
func (b *phaseBase) startTagHTML(tok *token) *token {
	if !b.p.firstStartTag && tok.name == "html" {
		b.p.parseError("non-html-root")
	}
	// XXX Need a check here to see if the first start tag token emitted is
	// this token... If it's not, invoke self.parser.parse_error().
	mergeAttributes(b.tree.openElements[0], tok.attrs)
	b.p.firstStartTag = false
	return nil
}

// mergeAttributes adds each attribute of attrs that the element does not
// have, as start_tag_html and start_tag_body in tinyhtml5 parser.py do.
func mergeAttributes(element *Node, attrs []Attribute) {
	if len(element.attrs)+len(attrs) <= smallAttrs {
		for _, a := range attrs {
			if attrIndex(element.attrs, a.Namespace, a.Name) < 0 {
				element.attrs = append(element.attrs, a)
			}
		}
		return
	}
	type key struct{ ns, name string }
	have := make(map[key]bool, len(element.attrs)+len(attrs))
	for _, a := range element.attrs {
		have[key{a.Namespace, a.Name}] = true
	}
	for _, a := range attrs {
		if k := (key{a.Namespace, a.Name}); !have[k] {
			have[k] = true
			element.attrs = append(element.attrs, a)
		}
	}
}

// initialPhase is InitialPhase in tinyhtml5 parser.py.
type initialPhase struct{ phaseBase }

// processSpaceCharacters is InitialPhase.process_space_characters in
// tinyhtml5 parser.py.
func (*initialPhase) processSpaceCharacters(*token) *token {
	return nil
}

// processComment is InitialPhase.process_comment in tinyhtml5 parser.py.
func (ph *initialPhase) processComment(tok *token) *token {
	ph.tree.insertComment(tok, ph.tree.document)
	return nil
}

// quirksPublicIDPrefixes are the prefixes of public identifiers that set
// quirks mode in InitialPhase.process_doctype of tinyhtml5 parser.py.
var quirksPublicIDPrefixes = []string{
	"+//silmaril//dtd html pro v0r11 19970101//",
	"-//advasoft ltd//dtd html 3.0 aswedit + extensions//",
	"-//as//dtd html 3.0 aswedit + extensions//",
	"-//ietf//dtd html 2.0 level 1//",
	"-//ietf//dtd html 2.0 level 2//",
	"-//ietf//dtd html 2.0 strict level 1//",
	"-//ietf//dtd html 2.0 strict level 2//",
	"-//ietf//dtd html 2.0 strict//",
	"-//ietf//dtd html 2.0//",
	"-//ietf//dtd html 2.1e//",
	"-//ietf//dtd html 3.0//",
	"-//ietf//dtd html 3.2 final//",
	"-//ietf//dtd html 3.2//",
	"-//ietf//dtd html 3//",
	"-//ietf//dtd html level 0//",
	"-//ietf//dtd html level 1//",
	"-//ietf//dtd html level 2//",
	"-//ietf//dtd html level 3//",
	"-//ietf//dtd html strict level 0//",
	"-//ietf//dtd html strict level 1//",
	"-//ietf//dtd html strict level 2//",
	"-//ietf//dtd html strict level 3//",
	"-//ietf//dtd html strict//",
	"-//ietf//dtd html//",
	"-//metrius//dtd metrius presentational//",
	"-//microsoft//dtd internet explorer 2.0 html strict//",
	"-//microsoft//dtd internet explorer 2.0 html//",
	"-//microsoft//dtd internet explorer 2.0 tables//",
	"-//microsoft//dtd internet explorer 3.0 html strict//",
	"-//microsoft//dtd internet explorer 3.0 html//",
	"-//microsoft//dtd internet explorer 3.0 tables//",
	"-//netscape comm. corp.//dtd html//",
	"-//netscape comm. corp.//dtd strict html//",
	"-//o'reilly and associates//dtd html 2.0//",
	"-//o'reilly and associates//dtd html extended 1.0//",
	"-//o'reilly and associates//dtd html extended relaxed 1.0//",
	"-//softquad software//dtd hotmetal pro 6.0::19990601::" +
		"extensions to html 4.0//",
	"-//softquad//dtd hotmetal pro 4.0::19971010::" +
		"extensions to html 4.0//",
	"-//spyglass//dtd html 2.0 extended//",
	"-//sq//dtd html 2.0 hotmetal + extensions//",
	"-//sun microsystems corp.//dtd hotjava html//",
	"-//sun microsystems corp.//dtd hotjava strict html//",
	"-//w3c//dtd html 3 1995-03-24//",
	"-//w3c//dtd html 3.2 draft//",
	"-//w3c//dtd html 3.2 final//",
	"-//w3c//dtd html 3.2//",
	"-//w3c//dtd html 3.2s draft//",
	"-//w3c//dtd html 4.0 frameset//",
	"-//w3c//dtd html 4.0 transitional//",
	"-//w3c//dtd html experimental 19960712//",
	"-//w3c//dtd html experimental 970421//",
	"-//w3c//dtd w3 html//",
	"-//w3o//dtd w3 html 3.0//",
	"-//webtechs//dtd mozilla html 2.0//",
	"-//webtechs//dtd mozilla html//",
}

// hasAnyPrefix is str.startswith with a tuple of prefixes.
func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// processDoctype is InitialPhase.process_doctype in tinyhtml5 parser.py.
func (ph *initialPhase) processDoctype(tok *token) *token {
	name := tok.name
	publicID := tok.publicID
	systemID := tok.systemID
	correct := tok.correct

	if name != "html" || tok.hasPublicID ||
		(tok.hasSystemID && systemID != "about:legacy-compat") {
		ph.p.parseError("unknown-doctype")
	}

	ph.tree.insertDoctype(tok)

	if publicID != "" {
		publicID = asciiUpperToLower(publicID)
	}

	switch {
	case !correct || tok.name != "html" ||
		hasAnyPrefix(publicID, quirksPublicIDPrefixes...) ||
		publicID == "-//w3o//dtd w3 html strict 3.0//en//" ||
		publicID == "-/w3c/dtd html 4.0 transitional/en" ||
		publicID == "html" ||
		(hasAnyPrefix(publicID,
			"-//w3c//dtd html 4.01 frameset//",
			"-//w3c//dtd html 4.01 transitional//") &&
			!tok.hasSystemID) ||
		(systemID != "" && strings.ToLower(systemID) ==
			"http://www.ibm.com/data/dtd/v11/ibmxhtml1-transitional.dtd"):
		ph.p.compatibilityMode = Quirks
	case hasAnyPrefix(publicID,
		"-//w3c//dtd xhtml 1.0 frameset//",
		"-//w3c//dtd xhtml 1.0 transitional//") ||
		(hasAnyPrefix(publicID,
			"-//w3c//dtd html 4.01 frameset//",
			"-//w3c//dtd html 4.01 transitional//") &&
			tok.hasSystemID):
		ph.p.compatibilityMode = LimitedQuirks
	}

	ph.p.phase = ph.p.phases.beforeHTML
	return nil
}

// anythingElse is InitialPhase.anything_else in tinyhtml5 parser.py.
func (ph *initialPhase) anythingElse() {
	ph.p.compatibilityMode = Quirks
	ph.p.phase = ph.p.phases.beforeHTML
}

// processCharacters is InitialPhase.process_characters in tinyhtml5
// parser.py.
func (ph *initialPhase) processCharacters(tok *token) *token {
	ph.p.parseError("expected-doctype-but-got-chars")
	ph.anythingElse()
	return tok
}

// processStartTag is InitialPhase.process_start_tag in tinyhtml5 parser.py.
func (ph *initialPhase) processStartTag(tok *token) *token {
	ph.p.parseError("expected-doctype-but-got-start-tag", "name", tok.name)
	ph.anythingElse()
	return tok
}

// processEndTag is InitialPhase.process_end_tag in tinyhtml5 parser.py.
func (ph *initialPhase) processEndTag(tok *token) *token {
	ph.p.parseError("expected-doctype-but-got-end-tag", "name", tok.name)
	ph.anythingElse()
	return tok
}

// processEOF is InitialPhase.process_eof in tinyhtml5 parser.py.
func (ph *initialPhase) processEOF() bool {
	ph.p.parseError("expected-doctype-but-got-eof")
	ph.anythingElse()
	return true
}

// beforeHTMLPhase is BeforeHtmlPhase in tinyhtml5 parser.py.
type beforeHTMLPhase struct{ phaseBase }

// insertHTMLElement is BeforeHtmlPhase._insert_html_element in tinyhtml5
// parser.py.
func (ph *beforeHTMLPhase) insertHTMLElement() {
	ph.tree.insertRoot(impliedTagToken("html", tokenStartTag))
	ph.p.phase = ph.p.phases.beforeHead
}

// processEOF is BeforeHtmlPhase.process_eof in tinyhtml5 parser.py.
func (ph *beforeHTMLPhase) processEOF() bool {
	ph.insertHTMLElement()
	return true
}

// processComment is BeforeHtmlPhase.process_comment in tinyhtml5 parser.py.
func (ph *beforeHTMLPhase) processComment(tok *token) *token {
	ph.tree.insertComment(tok, ph.tree.document)
	return nil
}

// processSpaceCharacters is BeforeHtmlPhase.process_space_characters in
// tinyhtml5 parser.py.
func (*beforeHTMLPhase) processSpaceCharacters(*token) *token {
	return nil
}

// processCharacters is BeforeHtmlPhase.process_characters in tinyhtml5
// parser.py.
func (ph *beforeHTMLPhase) processCharacters(tok *token) *token {
	ph.insertHTMLElement()
	return tok
}

// processStartTag is BeforeHtmlPhase.process_start_tag in tinyhtml5
// parser.py.
func (ph *beforeHTMLPhase) processStartTag(tok *token) *token {
	if tok.name == "html" {
		ph.p.firstStartTag = true
	}
	ph.insertHTMLElement()
	return tok
}

// processEndTag is BeforeHtmlPhase.process_end_tag in tinyhtml5 parser.py.
func (ph *beforeHTMLPhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "head", "body", "html", "br":
		ph.insertHTMLElement()
		return tok
	}
	ph.p.parseError("unexpected-end-tag-before-html", "name", tok.name)
	return nil
}

// beforeHeadPhase is BeforeHeadPhase in tinyhtml5 parser.py.
type beforeHeadPhase struct{ phaseBase }

// processEOF is BeforeHeadPhase.process_eof in tinyhtml5 parser.py.
func (ph *beforeHeadPhase) processEOF() bool {
	ph.startTagHead(impliedTagToken("head", tokenStartTag))
	return true
}

// processSpaceCharacters is BeforeHeadPhase.process_space_characters in
// tinyhtml5 parser.py.
func (*beforeHeadPhase) processSpaceCharacters(*token) *token {
	return nil
}

// processCharacters is BeforeHeadPhase.process_characters in tinyhtml5
// parser.py.
func (ph *beforeHeadPhase) processCharacters(tok *token) *token {
	ph.startTagHead(impliedTagToken("head", tokenStartTag))
	return tok
}

// processStartTag dispatches as BeforeHeadPhase.start_tag_handler in
// tinyhtml5 parser.py.
func (ph *beforeHeadPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.startTagHTML(tok)
	case "head":
		return ph.startTagHead(tok)
	}
	return ph.startTagOther(tok)
}

// startTagHTML is BeforeHeadPhase.start_tag_html in tinyhtml5 parser.py.
func (ph *beforeHeadPhase) startTagHTML(tok *token) *token {
	return ph.p.phases.inBody.processStartTag(tok)
}

// startTagHead is BeforeHeadPhase.start_tag_head in tinyhtml5 parser.py.
func (ph *beforeHeadPhase) startTagHead(tok *token) *token {
	ph.tree.insertElement(tok)
	ph.tree.headElement = ph.tree.currentNode()
	ph.p.phase = ph.p.phases.inHead
	return nil
}

// startTagOther is BeforeHeadPhase.start_tag_other in tinyhtml5 parser.py.
func (ph *beforeHeadPhase) startTagOther(tok *token) *token {
	ph.startTagHead(impliedTagToken("head", tokenStartTag))
	return tok
}

// processEndTag dispatches as BeforeHeadPhase.end_tag_handler in tinyhtml5
// parser.py.
func (ph *beforeHeadPhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "head", "body", "html", "br":
		return ph.endTagImplyHead(tok)
	}
	return ph.endTagOther(tok)
}

// endTagImplyHead is BeforeHeadPhase.end_tag_imply_head in tinyhtml5
// parser.py.
func (ph *beforeHeadPhase) endTagImplyHead(tok *token) *token {
	ph.startTagHead(impliedTagToken("head", tokenStartTag))
	return tok
}

// endTagOther is BeforeHeadPhase.end_tag_other in tinyhtml5 parser.py.
func (ph *beforeHeadPhase) endTagOther(tok *token) *token {
	ph.p.parseError("end-tag-after-implied-root", "name", tok.name)
	return nil
}

// inHeadPhase is InHeadPhase in tinyhtml5 parser.py.
type inHeadPhase struct{ phaseBase }

// processEOF is InHeadPhase.process_eof in tinyhtml5 parser.py, the real
// thing.
func (ph *inHeadPhase) processEOF() bool {
	ph.anythingElse()
	return true
}

// processCharacters is InHeadPhase.process_characters in tinyhtml5
// parser.py.
func (ph *inHeadPhase) processCharacters(tok *token) *token {
	ph.anythingElse()
	return tok
}

// processStartTag dispatches as InHeadPhase.start_tag_handler in tinyhtml5
// parser.py.
func (ph *inHeadPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.p.phases.inBody.processStartTag(tok)
	case "title":
		return ph.startTagTitle(tok)
	case "noframes", "style":
		return ph.startTagNoframesStyle(tok)
	case "noscript":
		return ph.startTagNoscript(tok)
	case "script":
		return ph.startTagScript(tok)
	case "base", "basefont", "bgsound", "command", "link":
		return ph.startTagBaseLinkCommand(tok)
	case "meta":
		return ph.startTagMeta(tok)
	case "head":
		return ph.startTagHead(tok)
	}
	return ph.startTagOther(tok)
}

// startTagHead is InHeadPhase.start_tag_head in tinyhtml5 parser.py.
func (ph *inHeadPhase) startTagHead(*token) *token {
	ph.p.parseError("two-heads-are-not-better-than-one")
	return nil
}

// startTagBaseLinkCommand is InHeadPhase.start_tag_base_link_command in
// tinyhtml5 parser.py.
func (ph *inHeadPhase) startTagBaseLinkCommand(tok *token) *token {
	ph.tree.insertElement(tok)
	ph.tree.pop()
	tok.selfClosingAcknowledged = true
	return nil
}

// startTagMeta is InHeadPhase.start_tag_meta in tinyhtml5 parser.py.
func (ph *inHeadPhase) startTagMeta(tok *token) *token {
	ph.tree.insertElement(tok)
	ph.tree.pop()
	tok.selfClosingAcknowledged = true

	stream := ph.p.tokenizer.stream
	if !stream.certain {
		charset, hasCharset := findAttr(tok.attrs, "charset")
		content, hasContent := findAttr(tok.attrs, "content")
		httpEquiv, hasHTTPEquiv := findAttr(tok.attrs, "http-equiv")
		switch {
		case hasCharset:
			ph.p.reparse = stream.changeEncoding(charset)
		case hasContent && hasHTTPEquiv && strings.ToLower(httpEquiv) == "content-type":
			// Encoding it as UTF-8 here is a hack, as really we should pass
			// the abstract Unicode string, and just use the
			// ContentAttributeParser on that, but using UTF-8 allows all
			// chars to be encoded and as a ASCII-superset works.
			data := newEncodingBytes([]byte(content))
			parser := newContentAttributeParser(data)
			if codec, ok := parser.parse(); ok {
				if !utf8ASCII(codec) {
					codec = ""
				}
				ph.p.reparse = stream.changeEncoding(codec)
			}
		}
	}
	return nil
}

// utf8ASCII says if a label is ASCII, as lookup_encoding of tinyhtml5
// inputstream.py needs for a label in bytes.
func utf8ASCII(label string) bool {
	for i := range len(label) {
		if label[i] >= 0x80 {
			return false
		}
	}
	return true
}

// findAttr returns the value of the attribute of a token with a name and no
// namespace.
func findAttr(attrs []Attribute, name string) (string, bool) {
	if i := attrIndex(attrs, "", name); i >= 0 {
		return attrs[i].Value, true
	}
	return "", false
}

// startTagTitle is InHeadPhase.start_tag_title in tinyhtml5 parser.py.
func (ph *inHeadPhase) startTagTitle(tok *token) *token {
	ph.p.parseRcdataRawtext(tok, false)
	return nil
}

// startTagNoframesStyle is InHeadPhase.start_tag_noframes_style in
// tinyhtml5 parser.py.
func (ph *inHeadPhase) startTagNoframesStyle(tok *token) *token {
	// Need to decide whether to implement the scripting-disabled case.
	ph.p.parseRcdataRawtext(tok, true)
	return nil
}

// startTagNoscript is InHeadPhase.start_tag_noscript in tinyhtml5
// parser.py. The scripting flag of upstream is false.
func (ph *inHeadPhase) startTagNoscript(tok *token) *token {
	ph.tree.insertElement(tok)
	ph.p.phase = ph.p.phases.inHeadNoscript
	return nil
}

// startTagScript is InHeadPhase.start_tag_script in tinyhtml5 parser.py.
func (ph *inHeadPhase) startTagScript(tok *token) *token {
	ph.tree.insertElement(tok)
	ph.p.tokenizer.state = scriptDataState
	ph.p.originalPhase = ph.p.phase
	ph.p.phase = ph.p.phases.text
	return nil
}

// startTagOther is InHeadPhase.start_tag_other in tinyhtml5 parser.py.
func (ph *inHeadPhase) startTagOther(tok *token) *token {
	ph.anythingElse()
	return tok
}

// processEndTag dispatches as InHeadPhase.end_tag_handler in tinyhtml5
// parser.py.
func (ph *inHeadPhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "head":
		return ph.endTagHead(tok)
	case "br", "html", "body":
		return ph.endTagHTMLBodyBr(tok)
	}
	return ph.endTagOther(tok)
}

// endTagHead is InHeadPhase.end_tag_head in tinyhtml5 parser.py.
func (ph *inHeadPhase) endTagHead(*token) *token {
	ph.tree.pop()
	ph.p.phase = ph.p.phases.afterHead
	return nil
}

// endTagHTMLBodyBr is InHeadPhase.end_tag_html_body_br in tinyhtml5
// parser.py.
func (ph *inHeadPhase) endTagHTMLBodyBr(tok *token) *token {
	ph.anythingElse()
	return tok
}

// endTagOther is InHeadPhase.end_tag_other in tinyhtml5 parser.py.
func (ph *inHeadPhase) endTagOther(tok *token) *token {
	ph.p.parseError("unexpected-end-tag", "name", tok.name)
	return nil
}

// anythingElse is InHeadPhase.anything_else in tinyhtml5 parser.py.
func (ph *inHeadPhase) anythingElse() {
	ph.endTagHead(impliedTagToken("head", tokenEndTag))
}

// inHeadNoscriptPhase is InHeadNoscriptPhase in tinyhtml5 parser.py.
type inHeadNoscriptPhase struct{ phaseBase }

// processEOF is InHeadNoscriptPhase.process_eof in tinyhtml5 parser.py.
func (ph *inHeadNoscriptPhase) processEOF() bool {
	ph.p.parseError("eof-in-head-noscript")
	ph.anythingElse()
	return true
}

// processComment is InHeadNoscriptPhase.process_comment in tinyhtml5
// parser.py.
func (ph *inHeadNoscriptPhase) processComment(tok *token) *token {
	return ph.p.phases.inHead.processComment(tok)
}

// processCharacters is InHeadNoscriptPhase.process_characters in tinyhtml5
// parser.py.
func (ph *inHeadNoscriptPhase) processCharacters(tok *token) *token {
	ph.p.parseError("char-in-head-noscript")
	ph.anythingElse()
	return tok
}

// processSpaceCharacters is InHeadNoscriptPhase.process_space_characters in
// tinyhtml5 parser.py.
func (ph *inHeadNoscriptPhase) processSpaceCharacters(tok *token) *token {
	return ph.p.phases.inHead.processSpaceCharacters(tok)
}

// processStartTag dispatches as InHeadNoscriptPhase.start_tag_handler in
// tinyhtml5 parser.py.
func (ph *inHeadNoscriptPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.p.phases.inBody.processStartTag(tok)
	case "basefont", "bgsound", "link", "meta", "noframes", "style":
		return ph.p.phases.inHead.processStartTag(tok)
	case "head", "noscript":
		ph.p.parseError("unexpected-start-tag", "name", tok.name)
		return nil
	}
	ph.p.parseError("unexpected-inhead-noscript-tag", "name", tok.name)
	ph.anythingElse()
	return tok
}

// processEndTag dispatches as InHeadNoscriptPhase.end_tag_handler in
// tinyhtml5 parser.py.
func (ph *inHeadNoscriptPhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "noscript":
		return ph.endTagNoscript(tok)
	case "br":
		ph.p.parseError("unexpected-inhead-noscript-tag", "name", tok.name)
		ph.anythingElse()
		return tok
	}
	ph.p.parseError("unexpected-end-tag", "name", tok.name)
	return nil
}

// endTagNoscript is InHeadNoscriptPhase.end_tag_noscript in tinyhtml5
// parser.py.
func (ph *inHeadNoscriptPhase) endTagNoscript(*token) *token {
	ph.tree.pop()
	ph.p.phase = ph.p.phases.inHead
	return nil
}

// anythingElse is InHeadNoscriptPhase.anything_else in tinyhtml5 parser.py.
// The caller must raise the parse error first.
func (ph *inHeadNoscriptPhase) anythingElse() {
	ph.endTagNoscript(impliedTagToken("noscript", tokenEndTag))
}

// afterHeadPhase is AfterHeadPhase in tinyhtml5 parser.py.
type afterHeadPhase struct{ phaseBase }

// processEOF is AfterHeadPhase.process_eof in tinyhtml5 parser.py.
func (ph *afterHeadPhase) processEOF() bool {
	ph.anythingElse()
	return true
}

// processCharacters is AfterHeadPhase.process_characters in tinyhtml5
// parser.py.
func (ph *afterHeadPhase) processCharacters(tok *token) *token {
	ph.anythingElse()
	return tok
}

// processStartTag dispatches as AfterHeadPhase.start_tag_handler in
// tinyhtml5 parser.py.
func (ph *afterHeadPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.p.phases.inBody.processStartTag(tok)
	case "body":
		ph.p.framesetOK = false
		ph.tree.insertElement(tok)
		ph.p.phase = ph.p.phases.inBody
		return nil
	case "frameset":
		ph.tree.insertElement(tok)
		ph.p.phase = ph.p.phases.inFrameset
		return nil
	case "base", "basefont", "bgsound", "link", "meta", "noframes", "script",
		"style", "title":
		return ph.startTagFromHead(tok)
	case "head":
		ph.p.parseError("unexpected-start-tag", "name", tok.name)
		return nil
	}
	ph.anythingElse()
	return tok
}

// startTagFromHead is AfterHeadPhase.start_tag_from_head in tinyhtml5
// parser.py.
func (ph *afterHeadPhase) startTagFromHead(tok *token) *token {
	ph.p.parseError("unexpected-start-tag-out-of-my-head", "name", tok.name)
	if ph.tree.headElement == nil {
		// Port: upstream appends None and fails. Only a fragment has no
		// head element here.
		return nil
	}
	ph.tree.openElements = append(ph.tree.openElements, ph.tree.headElement)
	ph.p.phases.inHead.processStartTag(tok)
	for i, node := range slices.Backward(ph.tree.openElements) {
		if node.name == "head" {
			ph.tree.openElements = slices.Delete(ph.tree.openElements, i, i+1)
			break
		}
	}
	return nil
}

// processEndTag dispatches as AfterHeadPhase.end_tag_handler in tinyhtml5
// parser.py.
func (ph *afterHeadPhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "body", "html", "br":
		ph.anythingElse()
		return tok
	}
	ph.p.parseError("unexpected-end-tag", "name", tok.name)
	return nil
}

// anythingElse is AfterHeadPhase.anything_else in tinyhtml5 parser.py.
func (ph *afterHeadPhase) anythingElse() {
	ph.tree.insertElement(impliedTagToken("body", tokenStartTag))
	ph.p.phase = ph.p.phases.inBody
	ph.p.framesetOK = true
}

// inBodyPhase is InBodyPhase in tinyhtml5 parser.py, the really crazy mode.
type inBodyPhase struct {
	phaseBase

	// dropNewline is true where upstream sets process_space_characters to
	// process_space_characters_drop_newline.
	dropNewline bool
}

// isMatchingFormattingElement is InBodyPhase.is_matching_formatting_element
// in tinyhtml5 parser.py.
func (ph *inBodyPhase) isMatchingFormattingElement(node1, node2 *Node) bool {
	return node1.name == node2.name &&
		node1.namespace == node2.namespace &&
		attributesEqual(node1.attrs, node2.attrs)
}

// addFormattingElement is InBodyPhase.add_formatting_element in tinyhtml5
// parser.py.
func (ph *inBodyPhase) addFormattingElement(tok *token) {
	ph.tree.insertElement(tok)
	element := ph.tree.currentNode()

	var matchingElements []*Node
	for _, node := range slices.Backward(ph.tree.activeFormattingElements) {
		if node == nil {
			break
		}
		if ph.isMatchingFormattingElement(node, element) {
			matchingElements = append(matchingElements, node)
		}
	}

	if len(matchingElements) == 3 {
		ph.tree.removeActiveFormattingElement(matchingElements[len(matchingElements)-1])
	}
	ph.tree.appendActiveFormattingElement(element)
}

// processEOF is InBodyPhase.process_eof in tinyhtml5 parser.py, the real
// deal.
func (ph *inBodyPhase) processEOF() bool {
	for _, node := range slices.Backward(ph.tree.openElements) {
		switch node.name {
		case "dd", "dt", "li", "p", "tbody", "td", "tfoot", "th", "thead", "tr",
			"body", "html":
			continue
		}
		ph.p.parseError("expected-closing-tag-but-got-eof")
		break
	}
	// Stop parsing.
	return false
}

// processSpaceCharacters is InBodyPhase.process_space_characters in
// tinyhtml5 parser.py, which upstream sets to one of two methods.
func (ph *inBodyPhase) processSpaceCharacters(tok *token) *token {
	if ph.dropNewline {
		ph.processSpaceCharactersDropNewline(tok)
	} else {
		ph.processSpaceCharactersNonPre(tok)
	}
	return nil
}

// processSpaceCharactersDropNewline is
// InBodyPhase.process_space_characters_drop_newline in tinyhtml5 parser.py.
// Sometimes (start of <pre>, <listing>, and <textarea> blocks) we want to
// drop leading newlines.
func (ph *inBodyPhase) processSpaceCharactersDropNewline(tok *token) {
	data := tok.data
	ph.dropNewline = false
	current := ph.tree.currentNode()
	if strings.HasPrefix(data, "\n") &&
		(current.name == "pre" || current.name == "listing" || current.name == "textarea") &&
		!hasContent(current) {
		data = data[1:]
	}
	if data != "" {
		ph.tree.reconstructActiveFormattingElements()
		ph.tree.insertText(data)
	}
}

// processCharacters is InBodyPhase.process_characters in tinyhtml5
// parser.py.
func (ph *inBodyPhase) processCharacters(tok *token) *token {
	if tok.data == "\u0000" {
		// The tokenizer should always emit null on its own.
		return nil
	}
	ph.tree.reconstructActiveFormattingElements()
	ph.tree.insertText(tok.data)
	// This must be bad for performance.
	if ph.p.framesetOK && strings.ContainsFunc(tok.data, func(c rune) bool { return !spaceCharacter(c) }) {
		ph.p.framesetOK = false
	}
	return nil
}

// processSpaceCharactersNonPre is
// InBodyPhase.process_space_characters_non_pre in tinyhtml5 parser.py.
func (ph *inBodyPhase) processSpaceCharactersNonPre(tok *token) {
	ph.tree.reconstructActiveFormattingElements()
	ph.tree.insertText(tok.data)
}

// processStartTag dispatches as InBodyPhase.start_tag_handler in tinyhtml5
// parser.py.
func (ph *inBodyPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.startTagHTML(tok)
	case "base", "basefont", "bgsound", "command", "link", "meta",
		"script", "style", "title":
		return ph.p.phases.inHead.processStartTag(tok)
	case "body":
		ph.startTagBody(tok)
	case "frameset":
		ph.startTagFrameset(tok)
	case "address", "article", "aside", "blockquote", "center", "details",
		"dir", "div", "dl", "fieldset", "figcaption", "figure",
		"footer", "header", "hgroup", "main", "menu", "nav", "ol", "p",
		"section", "summary", "ul":
		ph.startTagCloseP(tok)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		ph.startTagHeading(tok)
	case "pre", "listing":
		ph.startTagPreListing(tok)
	case "form":
		ph.startTagForm(tok)
	case "li", "dd", "dt":
		ph.startTagListItem(tok)
	case "plaintext":
		ph.startTagPlaintext(tok)
	case "a":
		ph.startTagA(tok)
	case "b", "big", "code", "em", "font", "i", "s", "small", "strike",
		"strong", "tt", "u":
		ph.startTagFormatting(tok)
	case "nobr":
		ph.startTagNobr(tok)
	case "button":
		return ph.startTagButton(tok)
	case "applet", "marquee", "object":
		ph.startTagAppletMarqueeObject(tok)
	case "xmp":
		ph.startTagXmp(tok)
	case "table":
		ph.startTagTable(tok)
	case "area", "br", "embed", "img", "keygen", "wbr":
		ph.startTagVoidFormatting(tok)
	case "param", "source", "track":
		ph.startTagParamSource(tok)
	case "input":
		ph.startTagInput(tok)
	case "hr":
		ph.startTagHr(tok)
	case "image":
		ph.startTagImage(tok)
	case "isindex":
		ph.startTagIsindex(tok)
	case "textarea":
		ph.startTagTextarea(tok)
	case "iframe":
		ph.startTagIframe(tok)
	case "noscript":
		ph.startTagNoscript(tok)
	case "noembed", "noframes":
		ph.startTagRawtext(tok)
	case "select":
		ph.startTagSelect(tok)
	case "rp", "rt":
		ph.startTagRpRt(tok)
	case "option", "optgroup":
		ph.startTagOpt(tok)
	case "math":
		ph.startTagMath(tok)
	case "svg":
		ph.startTagSVG(tok)
	case "caption", "col", "colgroup", "frame", "head",
		"tbody", "td", "tfoot", "th", "thead", "tr":
		ph.startTagMisplaced(tok)
	default:
		ph.startTagOther(tok)
	}
	return nil
}

// startTagBody is InBodyPhase.start_tag_body in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagBody(tok *token) {
	ph.p.parseError("unexpected-start-tag", "name", "body")
	open := ph.tree.openElements
	if len(open) == 1 || open[1].name != "body" {
		return
	}
	ph.p.framesetOK = false
	mergeAttributes(open[1], tok.attrs)
}

// startTagFrameset is InBodyPhase.start_tag_frameset in tinyhtml5
// parser.py.
func (ph *inBodyPhase) startTagFrameset(tok *token) {
	ph.p.parseError("unexpected-start-tag", "name", "frameset")
	open := ph.tree.openElements
	switch {
	case len(open) == 1 || open[1].name != "body":
	case !ph.p.framesetOK:
	default:
		if open[1].parent != nil {
			removeChild(open[1].parent, open[1])
		}
		for len(ph.tree.openElements) > 1 && ph.tree.currentNode().name != "html" {
			ph.tree.pop()
		}
		ph.tree.insertElement(tok)
		ph.p.phase = ph.p.phases.inFrameset
	}
}

// startTagCloseP is InBodyPhase.start_tag_close_p in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagCloseP(tok *token) {
	if ph.tree.elementInScope("p", scopeButton) {
		ph.endTagP(impliedTagToken("p", tokenEndTag))
	}
	ph.tree.insertElement(tok)
}

// startTagPreListing is InBodyPhase.start_tag_pre_listing in tinyhtml5
// parser.py.
func (ph *inBodyPhase) startTagPreListing(tok *token) {
	if ph.tree.elementInScope("p", scopeButton) {
		ph.endTagP(impliedTagToken("p", tokenEndTag))
	}
	ph.tree.insertElement(tok)
	ph.p.framesetOK = false
	ph.dropNewline = true
}

// startTagForm is InBodyPhase.start_tag_form in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagForm(tok *token) {
	if ph.tree.formElement != nil {
		ph.p.parseError("unexpected-start-tag", "name", "form")
		return
	}
	if ph.tree.elementInScope("p", scopeButton) {
		ph.endTagP(impliedTagToken("p", tokenEndTag))
	}
	ph.tree.insertElement(tok)
	ph.tree.formElement = ph.tree.currentNode()
}

// startTagListItem is InBodyPhase.start_tag_list_item in tinyhtml5
// parser.py.
func (ph *inBodyPhase) startTagListItem(tok *token) {
	ph.p.framesetOK = false

	stopNames := []string{"li"}
	if tok.name == "dt" || tok.name == "dd" {
		stopNames = []string{"dt", "dd"}
	}
	for _, node := range slices.Backward(ph.tree.openElements) {
		if slices.Contains(stopNames, node.name) {
			ph.p.phase.processEndTag(impliedTagToken(node.name, tokenEndTag))
			break
		}
		if specialElement(node.namespace, node.name) &&
			node.name != "address" && node.name != "div" && node.name != "p" {
			break
		}
	}

	if ph.tree.elementInScope("p", scopeButton) {
		ph.p.phase.processEndTag(impliedTagToken("p", tokenEndTag))
	}

	ph.tree.insertElement(tok)
}

// startTagPlaintext is InBodyPhase.start_tag_plaintext in tinyhtml5
// parser.py.
func (ph *inBodyPhase) startTagPlaintext(tok *token) {
	if ph.tree.elementInScope("p", scopeButton) {
		ph.endTagP(impliedTagToken("p", tokenEndTag))
	}
	ph.tree.insertElement(tok)
	ph.p.tokenizer.state = plaintextState
}

// startTagHeading is InBodyPhase.start_tag_heading in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagHeading(tok *token) {
	if ph.tree.elementInScope("p", scopeButton) {
		ph.endTagP(impliedTagToken("p", tokenEndTag))
	}
	if headingElement(ph.tree.currentNode().name) {
		ph.p.parseError("unexpected-start-tag", "name", tok.name)
		ph.tree.pop()
	}
	ph.tree.insertElement(tok)
}

// startTagA is InBodyPhase.start_tag_a in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagA(tok *token) {
	if afeAElement := ph.tree.elementInActiveFormattingElements("a"); afeAElement != nil {
		ph.p.parseError("unexpected-start-tag-implies-end-tag", "startName", "a", "endName", "a")
		ph.endTagFormatting(impliedTagToken("a", tokenEndTag))
		ph.tree.removeOpenElement(afeAElement)
		ph.tree.removeActiveFormattingElement(afeAElement)
	}
	ph.tree.reconstructActiveFormattingElements()
	ph.addFormattingElement(tok)
}

// startTagFormatting is InBodyPhase.start_tag_formatting in tinyhtml5
// parser.py.
func (ph *inBodyPhase) startTagFormatting(tok *token) {
	ph.tree.reconstructActiveFormattingElements()
	ph.addFormattingElement(tok)
}

// startTagNobr is InBodyPhase.start_tag_nobr in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagNobr(tok *token) {
	ph.tree.reconstructActiveFormattingElements()
	if ph.tree.elementInScope("nobr", scopeDefault) {
		ph.p.parseError("unexpected-start-tag-implies-end-tag", "startName", "nobr", "endName", "nobr")
		ph.processEndTag(impliedTagToken("nobr", tokenEndTag))
		// XXX Need tests that trigger the following.
		ph.tree.reconstructActiveFormattingElements()
	}
	ph.addFormattingElement(tok)
}

// startTagButton is InBodyPhase.start_tag_button in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagButton(tok *token) *token {
	if ph.tree.elementInScope("button", scopeDefault) {
		ph.p.parseError("unexpected-start-tag-implies-end-tag", "startName", "button", "endName", "button")
		ph.processEndTag(impliedTagToken("button", tokenEndTag))
		return tok
	}
	ph.tree.reconstructActiveFormattingElements()
	ph.tree.insertElement(tok)
	ph.p.framesetOK = false
	return nil
}

// startTagAppletMarqueeObject is
// InBodyPhase.start_tag_applet_marquee_object in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagAppletMarqueeObject(tok *token) {
	ph.tree.reconstructActiveFormattingElements()
	ph.tree.insertElement(tok)
	ph.tree.appendActiveFormattingElement(nil)
	ph.p.framesetOK = false
}

// startTagXmp is InBodyPhase.start_tag_xmp in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagXmp(tok *token) {
	if ph.tree.elementInScope("p", scopeButton) {
		ph.endTagP(impliedTagToken("p", tokenEndTag))
	}
	ph.tree.reconstructActiveFormattingElements()
	ph.p.framesetOK = false
	ph.p.parseRcdataRawtext(tok, true)
}

// startTagTable is InBodyPhase.start_tag_table in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagTable(tok *token) {
	if ph.p.compatibilityMode != Quirks {
		if ph.tree.elementInScope("p", scopeButton) {
			ph.processEndTag(impliedTagToken("p", tokenEndTag))
		}
	}
	ph.tree.insertElement(tok)
	ph.p.framesetOK = false
	ph.p.phase = ph.p.phases.inTable
}

// startTagVoidFormatting is InBodyPhase.start_tag_void_formatting in
// tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagVoidFormatting(tok *token) {
	ph.tree.reconstructActiveFormattingElements()
	ph.tree.insertElement(tok)
	ph.tree.pop()
	tok.selfClosingAcknowledged = true
	ph.p.framesetOK = false
}

// startTagInput is InBodyPhase.start_tag_input in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagInput(tok *token) {
	framesetOK := ph.p.framesetOK
	ph.startTagVoidFormatting(tok)
	if typ, ok := findAttr(tok.attrs, "type"); ok && asciiUpperToLower(typ) == "hidden" {
		// input type=hidden doesn't change frameset_ok.
		ph.p.framesetOK = framesetOK
	}
}

// startTagParamSource is InBodyPhase.start_tag_param_source in tinyhtml5
// parser.py.
func (ph *inBodyPhase) startTagParamSource(tok *token) {
	ph.tree.insertElement(tok)
	ph.tree.pop()
	tok.selfClosingAcknowledged = true
}

// startTagHr is InBodyPhase.start_tag_hr in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagHr(tok *token) {
	if ph.tree.elementInScope("p", scopeButton) {
		ph.endTagP(impliedTagToken("p", tokenEndTag))
	}
	ph.tree.insertElement(tok)
	ph.tree.pop()
	tok.selfClosingAcknowledged = true
	ph.p.framesetOK = false
}

// startTagImage is InBodyPhase.start_tag_image in tinyhtml5 parser.py. No
// really...
func (ph *inBodyPhase) startTagImage(tok *token) {
	ph.p.parseError("unexpected-start-tag-treated-as", "originalName", "image", "newName", "img")
	img := impliedTagToken("img", tokenStartTag)
	img.attrs = tok.attrs
	img.selfClosing = tok.selfClosing
	ph.processStartTag(img)
}

// startTagIsindex is InBodyPhase.start_tag_isindex in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagIsindex(tok *token) {
	ph.p.parseError("deprecated-tag", "name", "isindex")
	if ph.tree.formElement != nil {
		return
	}
	form := impliedTagToken("form", tokenStartTag)
	if action, ok := findAttr(tok.attrs, "action"); ok {
		form.attrs = []Attribute{{Name: "action", Value: action}}
	}
	ph.processStartTag(form)
	ph.processStartTag(impliedTagToken("hr", tokenStartTag))
	ph.processStartTag(impliedTagToken("label", tokenStartTag))
	// XXX Localization ...
	prompt, ok := findAttr(tok.attrs, "prompt")
	if !ok {
		prompt = "This is a searchable index. Enter search keywords: "
	}
	ph.processCharacters(&token{typ: tokenCharacters, data: prompt})
	var attributes []Attribute
	for _, a := range tok.attrs {
		if a.Namespace == "" && (a.Name == "action" || a.Name == "prompt") {
			continue
		}
		attributes = append(attributes, a)
	}
	if i := attrIndex(attributes, "", "name"); i >= 0 {
		attributes[i].Value = "isindex"
	} else {
		attributes = append(attributes, Attribute{Name: "name", Value: "isindex"})
	}
	input := impliedTagToken("input", tokenStartTag)
	input.attrs = attributes
	input.selfClosing = tok.selfClosing
	ph.processStartTag(input)
	ph.processEndTag(impliedTagToken("label", tokenEndTag))
	ph.processStartTag(impliedTagToken("hr", tokenStartTag))
	ph.processEndTag(impliedTagToken("form", tokenEndTag))
}

// startTagTextarea is InBodyPhase.start_tag_textarea in tinyhtml5
// parser.py.
func (ph *inBodyPhase) startTagTextarea(tok *token) {
	ph.tree.insertElement(tok)
	ph.p.tokenizer.state = rcdataState
	ph.dropNewline = true
	ph.p.framesetOK = false
}

// startTagIframe is InBodyPhase.start_tag_iframe in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagIframe(tok *token) {
	ph.p.framesetOK = false
	ph.startTagRawtext(tok)
}

// startTagNoscript is InBodyPhase.start_tag_noscript in tinyhtml5
// parser.py. The scripting flag of upstream is false.
func (ph *inBodyPhase) startTagNoscript(tok *token) {
	ph.startTagOther(tok)
}

// startTagRawtext is InBodyPhase.start_tag_rawtext in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagRawtext(tok *token) {
	ph.p.parseRcdataRawtext(tok, true)
}

// startTagOpt is InBodyPhase.start_tag_opt in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagOpt(tok *token) {
	if ph.tree.currentNode().name == "option" {
		ph.p.phase.processEndTag(impliedTagToken("option", tokenEndTag))
	}
	ph.tree.reconstructActiveFormattingElements()
	ph.tree.insertElement(tok)
}

// startTagSelect is InBodyPhase.start_tag_select in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagSelect(tok *token) {
	ph.tree.reconstructActiveFormattingElements()
	ph.tree.insertElement(tok)
	ph.p.framesetOK = false
	switch ph.p.phase {
	case ph.p.phases.inTable, ph.p.phases.inCaption, ph.p.phases.inColumnGroup,
		ph.p.phases.inTableBody, ph.p.phases.inRow, ph.p.phases.inCell:
		ph.p.phase = ph.p.phases.inSelectInTable
	default:
		ph.p.phase = ph.p.phases.inSelect
	}
}

// startTagRpRt is InBodyPhase.start_tag_rp_rt in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagRpRt(tok *token) {
	if ph.tree.elementInScope("ruby", scopeDefault) {
		ph.tree.generateImpliedEndTags("")
		if ph.tree.currentNode().name != "ruby" {
			ph.p.parseError("rp-or-rt-tag-not-in-ruby-scope")
		}
	}
	ph.tree.insertElement(tok)
}

// startTagMath is InBodyPhase.start_tag_math in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagMath(tok *token) {
	ph.tree.reconstructActiveFormattingElements()
	ph.p.adjustMathMLAttributes(tok)
	ph.p.adjustForeignAttributes(tok)
	tok.namespace = NamespaceMathML
	ph.tree.insertElement(tok)
	// Need to get the parse error right for the case where the token has a
	// namespace not equal to the xmlns attribute.
	if tok.selfClosing {
		ph.tree.pop()
		tok.selfClosingAcknowledged = true
	}
}

// startTagSVG is InBodyPhase.start_tag_svg in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagSVG(tok *token) {
	ph.tree.reconstructActiveFormattingElements()
	ph.p.adjustSVGAttributes(tok)
	ph.p.adjustForeignAttributes(tok)
	tok.namespace = NamespaceSVG
	ph.tree.insertElement(tok)
	// Need to get the parse error right for the case where the token has a
	// namespace not equal to the xmlns attribute.
	if tok.selfClosing {
		ph.tree.pop()
		tok.selfClosingAcknowledged = true
	}
}

// startTagMisplaced is InBodyPhase.start_tag_misplaced in tinyhtml5
// parser.py, for the elements that should be children of other elements.
// Here they are ignored: "caption", "col", "colgroup", "frame", "frameset",
// "head", "option", "optgroup", "tbody", "td", "tfoot", "th", "thead", "tr",
// "noscript".
func (ph *inBodyPhase) startTagMisplaced(tok *token) {
	ph.p.parseError("unexpected-start-tag-ignored", "name", tok.name)
}

// startTagOther is InBodyPhase.start_tag_other in tinyhtml5 parser.py.
func (ph *inBodyPhase) startTagOther(tok *token) {
	ph.tree.reconstructActiveFormattingElements()
	ph.tree.insertElement(tok)
}

// processEndTag dispatches as InBodyPhase.end_tag_handler in tinyhtml5
// parser.py.
func (ph *inBodyPhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "body":
		ph.endTagBody(tok)
	case "html":
		return ph.endTagHTML(tok)
	case "address", "article", "aside", "blockquote", "button", "center",
		"details", "dialog", "dir", "div", "dl", "fieldset", "figcaption", "figure",
		"footer", "header", "hgroup", "listing", "main", "menu", "nav", "ol", "pre",
		"section", "summary", "ul":
		ph.endTagBlock(tok)
	case "form":
		ph.endTagForm(tok)
	case "p":
		ph.endTagP(tok)
	case "dd", "dt", "li":
		ph.endTagListItem(tok)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		ph.endTagHeading(tok)
	case "a", "b", "big", "code", "em", "font", "i", "nobr", "s", "small",
		"strike", "strong", "tt", "u":
		ph.endTagFormatting(tok)
	case "applet", "marquee", "object":
		ph.endTagAppletMarqueeObject(tok)
	case "br":
		ph.endTagBr(tok)
	default:
		ph.endTagOther(tok)
	}
	return nil
}

// endTagP is InBodyPhase.end_tag_p in tinyhtml5 parser.py.
func (ph *inBodyPhase) endTagP(*token) {
	if !ph.tree.elementInScope("p", scopeButton) {
		ph.startTagCloseP(impliedTagToken("p", tokenStartTag))
		ph.p.parseError("unexpected-end-tag", "name", "p")
		ph.endTagP(impliedTagToken("p", tokenEndTag))
		return
	}
	ph.tree.generateImpliedEndTags("p")
	if ph.tree.currentNode().name != "p" {
		ph.p.parseError("unexpected-end-tag", "name", "p")
	}
	node := ph.tree.pop()
	for node.name != "p" && len(ph.tree.openElements) > 0 {
		node = ph.tree.pop()
	}
}

// endTagBody is InBodyPhase.end_tag_body in tinyhtml5 parser.py.
func (ph *inBodyPhase) endTagBody(*token) {
	if !ph.tree.elementInScope("body", scopeDefault) {
		ph.p.parseError("unexpected-end-tag", "name", "body")
		return
	}
	if ph.tree.currentNode().name != "body" {
		for _, node := range ph.tree.openElements[2:] {
			switch node.name {
			case "dd", "dt", "li", "optgroup", "option", "p", "rp", "rt",
				"tbody", "td", "tfoot", "th", "thead", "tr", "body", "html":
				continue
			}
			// Not sure this is the correct name for the parse error.
			ph.p.parseError("expected-one-end-tag-but-got-another", "gotName", "body", "expectedName", node.name)
			break
		}
	}
	ph.p.phase = ph.p.phases.afterBody
}

// endTagHTML is InBodyPhase.end_tag_html in tinyhtml5 parser.py.
func (ph *inBodyPhase) endTagHTML(tok *token) *token {
	// We repeat the test for the body end tag token being ignored here.
	if ph.tree.elementInScope("body", scopeDefault) {
		ph.endTagBody(impliedTagToken("body", tokenEndTag))
		return tok
	}
	return nil
}

// popUntil pops the open elements up to and including the first one with a
// name, as the loops "node = pop(); while node.name != name" of tinyhtml5
// parser.py do.
func (tb *treeBuilder) popUntil(name string) {
	for len(tb.openElements) > 0 {
		if tb.pop().name == name {
			return
		}
	}
}

// endTagBlock is InBodyPhase.end_tag_block in tinyhtml5 parser.py.
func (ph *inBodyPhase) endTagBlock(tok *token) {
	// Put us back in the right whitespace handling mode.
	if tok.name == "pre" {
		ph.dropNewline = false
	}
	inScope := ph.tree.elementInScope(tok.name, scopeDefault)
	if inScope {
		ph.tree.generateImpliedEndTags("")
	}
	if ph.tree.currentNode().name != tok.name {
		ph.p.parseError("end-tag-too-early", "name", tok.name)
	}
	if inScope {
		ph.tree.popUntil(tok.name)
	}
}

// endTagForm is InBodyPhase.end_tag_form in tinyhtml5 parser.py.
func (ph *inBodyPhase) endTagForm(*token) {
	node := ph.tree.formElement
	ph.tree.formElement = nil
	if node == nil || !ph.tree.nodeInScope(node, scopeDefault) {
		ph.p.parseError("unexpected-end-tag", "name", "form")
		return
	}
	ph.tree.generateImpliedEndTags("")
	if ph.tree.currentNode() != node {
		ph.p.parseError("end-tag-too-early-ignored", "name", "form")
	}
	ph.tree.removeOpenElement(node)
}

// endTagListItem is InBodyPhase.end_tag_list_item in tinyhtml5 parser.py.
func (ph *inBodyPhase) endTagListItem(tok *token) {
	variant := scopeDefault
	if tok.name == "li" {
		variant = scopeList
	}
	if !ph.tree.elementInScope(tok.name, variant) {
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
		return
	}
	ph.tree.generateImpliedEndTags(tok.name)
	if ph.tree.currentNode().name != tok.name {
		ph.p.parseError("end-tag-too-early", "name", tok.name)
	}
	ph.tree.popUntil(tok.name)
}

// endTagHeading is InBodyPhase.end_tag_heading in tinyhtml5 parser.py.
func (ph *inBodyPhase) endTagHeading(tok *token) {
	for _, item := range headingElements {
		if ph.tree.elementInScope(item, scopeDefault) {
			ph.tree.generateImpliedEndTags("")
			break
		}
	}
	if ph.tree.currentNode().name != tok.name {
		ph.p.parseError("end-tag-too-early", "name", tok.name)
	}

	for _, item := range headingElements {
		if ph.tree.elementInScope(item, scopeDefault) {
			node := ph.tree.pop()
			for !headingElement(node.name) && len(ph.tree.openElements) > 0 {
				node = ph.tree.pop()
			}
			break
		}
	}
}

// endTagFormatting is InBodyPhase.end_tag_formatting in tinyhtml5 parser.py,
// the much-feared adoption agency algorithm.
// http://svn.whatwg.org/webapps/complete.html#adoptionAgency revision 7867
// XXX Better parseError messages appreciated.
func (ph *inBodyPhase) endTagFormatting(tok *token) {
	tree := ph.tree

	// Step 1.
	outerLoopCounter := 0

	// Step 2.
	for outerLoopCounter < 8 {
		// Step 3.
		outerLoopCounter++

		// Step 4.

		// Let the formatting element be the last element in
		// the list of active formatting elements that:
		// - is between the end of the list and the last scope
		// marker in the list, if any, or the start of the list
		// otherwise, and
		// - has the same tag name as the token.
		formattingElement := tree.elementInActiveFormattingElements(tok.name)
		inOpen := formattingElement != nil && slices.Contains(tree.openElements, formattingElement)
		switch {
		case formattingElement == nil ||
			(inOpen && !tree.elementInScope(formattingElement.name, scopeDefault)):
			// If there is no such node, then abort these steps
			// and instead act as described in the "any other
			// end tag" entry below.
			ph.endTagOther(tok)
			return

		case !inOpen:
			// Otherwise, if there is such a node, but that node is
			// not in the stack of open elements, then this is a
			// parse error; remove the element from the list, and
			// abort these steps.
			ph.p.parseError("adoption-agency-1.2", "name", tok.name)
			tree.removeActiveFormattingElement(formattingElement)
			return

		case !tree.elementInScope(formattingElement.name, scopeDefault):
			// Otherwise, if there is such a node, and that node is
			// also in the stack of open elements, but the element
			// is not in scope, then this is a parse error; ignore
			// the token, and abort these steps.
			ph.p.parseError("adoption-agency-4.4", "name", tok.name)
			return

		default:
			// Otherwise, there is a formatting element and that
			// element is in the stack and is in scope. If the
			// element is not the current node, this is a parse
			// error. In any case, proceed with the algorithm as
			// written in the following steps.
			if formattingElement != tree.currentNode() {
				ph.p.parseError("adoption-agency-1.3", "name", tok.name)
			}
		}

		// Step 5.

		// Let the furthest block be the topmost node in the
		// stack of open elements that is lower in the stack
		// than the formatting element, and is an element in
		// the special category. There might not be one.
		afeIndex := slices.Index(tree.openElements, formattingElement)
		var furthestBlock *Node
		for _, element := range tree.openElements[afeIndex:] {
			if specialElement(element.namespace, element.name) {
				furthestBlock = element
				break
			}
		}

		// Step 6.

		// If there is no furthest block, then the UA must
		// first pop all the nodes from the bottom of the stack
		// of open elements, from the current node up to and
		// including the formatting element, then remove the
		// formatting element from the list of active
		// formatting elements, and finally abort these steps.
		if furthestBlock == nil {
			element := tree.pop()
			for element != formattingElement && len(tree.openElements) > 0 {
				element = tree.pop()
			}
			tree.removeActiveFormattingElement(element)
			return
		}

		// Step 7.
		commonAncestor := tree.openElements[pyIndex(afeIndex-1, len(tree.openElements))]

		// Step 8.

		// The bookmark is supposed to help us identify where to reinsert
		// nodes in step 15. We have to ensure that we reinsert nodes after
		// the node before the active formatting element. Note the bookmark
		// can move in step 9.7.
		bookmark := slices.Index(tree.activeFormattingElements, formattingElement)

		// Step 9.
		lastNode := furthestBlock
		node := furthestBlock
		innerLoopCounter := 0

		index := slices.Index(tree.openElements, node)
		for innerLoopCounter < 3 {
			innerLoopCounter++
			// Node is element before node in open elements.
			index--
			node = tree.openElements[pyIndex(index, len(tree.openElements))]
			if !slices.Contains(tree.activeFormattingElements, node) {
				tree.removeOpenElement(node)
				continue
			}
			// Step 9.6.
			if node == formattingElement {
				break
			}
			// Step 9.7.
			if lastNode == furthestBlock {
				bookmark = slices.Index(tree.activeFormattingElements, node) + 1
			}
			// Step 9.8.
			clone := tree.clone(node)
			// Replace node with clone.
			tree.activeFormattingElements[slices.Index(tree.activeFormattingElements, node)] = clone
			tree.openElements[slices.Index(tree.openElements, node)] = clone
			node = clone
			// Step 9.9.
			// Remove lastNode from its parents, if any.
			if lastNode.parent != nil {
				removeChild(lastNode.parent, lastNode)
			}
			appendChild(node, lastNode)
			// Step 9.10.
			lastNode = node
		}

		// Step 10.

		// Foster parent lastNode if commonAncestor is a
		// table, tbody, tfoot, thead, or tr we need to foster
		// parent the lastNode.
		if lastNode.parent != nil {
			removeChild(lastNode.parent, lastNode)
		}

		if tableInsertModeElement(commonAncestor.name) {
			parent, before := tree.getTableMisnestedNodePosition()
			if before == nil {
				// Port: upstream fails without a node to insert before.
				appendChild(parent, lastNode)
			} else {
				insertBefore(parent, lastNode, before)
			}
		} else {
			appendChild(commonAncestor, lastNode)
		}

		// Step 11.
		clone := tree.clone(formattingElement)

		// Step 12.
		reparentChildren(furthestBlock, clone)

		// Step 13.
		appendChild(furthestBlock, clone)

		// Step 14.
		tree.removeActiveFormattingElement(formattingElement)
		bookmark = min(max(bookmark, 0), len(tree.activeFormattingElements))
		tree.activeFormattingElements = slices.Insert(tree.activeFormattingElements, bookmark, clone)

		// Step 15.
		tree.removeOpenElement(formattingElement)
		i := slices.Index(tree.openElements, furthestBlock) + 1
		tree.openElements = slices.Insert(tree.openElements, i, clone)
	}
}

// pyIndex is the index i of a list of length n as Python reads it: a
// negative index counts from the end.
func pyIndex(i, n int) int {
	if i < 0 {
		i += n
	}
	return min(max(i, 0), n-1)
}

// endTagAppletMarqueeObject is InBodyPhase.end_tag_applet_marquee_object in
// tinyhtml5 parser.py.
func (ph *inBodyPhase) endTagAppletMarqueeObject(tok *token) {
	if ph.tree.elementInScope(tok.name, scopeDefault) {
		ph.tree.generateImpliedEndTags("")
	}
	if ph.tree.currentNode().name != tok.name {
		ph.p.parseError("end-tag-too-early", "name", tok.name)
	}

	if ph.tree.elementInScope(tok.name, scopeDefault) {
		ph.tree.popUntil(tok.name)
		ph.tree.clearActiveFormattingElements()
	}
}

// endTagBr is InBodyPhase.end_tag_br in tinyhtml5 parser.py.
func (ph *inBodyPhase) endTagBr(*token) {
	ph.p.parseError("unexpected-end-tag-treated-as", "originalName", "br", "newName", "br element")
	ph.tree.reconstructActiveFormattingElements()
	ph.tree.insertElement(impliedTagToken("br", tokenStartTag))
	ph.tree.pop()
}

// endTagOther is InBodyPhase.end_tag_other in tinyhtml5 parser.py.
func (ph *inBodyPhase) endTagOther(tok *token) {
	for _, node := range slices.Backward(ph.tree.openElements) {
		if node.name == tok.name {
			ph.tree.generateImpliedEndTags(tok.name)
			if ph.tree.currentNode().name != tok.name {
				ph.p.parseError("unexpected-end-tag", "name", tok.name)
			}
			for len(ph.tree.openElements) > 0 && ph.tree.pop() != node {
			}
			break
		}
		if specialElement(node.namespace, node.name) {
			ph.p.parseError("unexpected-end-tag", "name", tok.name)
			break
		}
	}
}

// textPhase is TextPhase in tinyhtml5 parser.py.
type textPhase struct{ phaseBase }

// processCharacters is TextPhase.process_characters in tinyhtml5 parser.py.
func (ph *textPhase) processCharacters(tok *token) *token {
	ph.tree.insertText(tok.data)
	return nil
}

// processEOF is TextPhase.process_eof in tinyhtml5 parser.py.
func (ph *textPhase) processEOF() bool {
	ph.p.parseError("expected-named-closing-tag-but-got-eof", "name", ph.tree.currentNode().name)
	ph.tree.pop()
	ph.p.phase = ph.p.originalPhase
	return true
}

// processStartTag is TextPhase.start_tag_other in tinyhtml5 parser.py.
// Upstream raises ValueError, and the port ignores the tag.
func (ph *textPhase) processStartTag(tok *token) *token {
	ph.p.parseError("unexpected-start-tag-in-text", "name", tok.name)
	return nil
}

// processEndTag dispatches as TextPhase.end_tag_handler in tinyhtml5
// parser.py. The rest of end_tag_script is all stuff that only happens if
// document.write works.
func (ph *textPhase) processEndTag(*token) *token {
	ph.tree.pop()
	ph.p.phase = ph.p.originalPhase
	return nil
}

// inTablePhase is InTablePhase in tinyhtml5 parser.py.
// http://www.whatwg.org/specs/web-apps/current-work/#in-table
type inTablePhase struct{ phaseBase }

// clearStackToTableContext is InTablePhase._clear_stack_to_table_context in
// tinyhtml5 parser.py, "clear the stack back to a table context".
func (ph *inTablePhase) clearStackToTableContext() {
	for len(ph.tree.openElements) > 1 {
		if name := ph.tree.currentNode().name; name == "table" || name == "html" {
			break
		}
		ph.tree.pop()
	}
	// When the current node is <html> it's a fragment case.
}

// processEOF is InTablePhase.process_eof in tinyhtml5 parser.py.
func (ph *inTablePhase) processEOF() bool {
	if ph.tree.currentNode().name != "html" {
		ph.p.parseError("eof-in-table")
	}
	// Stop parsing.
	return false
}

// processSpaceCharacters is InTablePhase.process_space_characters in
// tinyhtml5 parser.py.
func (ph *inTablePhase) processSpaceCharacters(tok *token) *token {
	originalPhase := ph.p.phase
	ph.p.phase = ph.p.phases.inTableText
	ph.p.phases.inTableText.originalPhase = originalPhase
	return ph.p.phases.inTableText.processSpaceCharacters(tok)
}

// processCharacters is InTablePhase.process_characters in tinyhtml5
// parser.py.
func (ph *inTablePhase) processCharacters(tok *token) *token {
	originalPhase := ph.p.phase
	ph.p.phase = ph.p.phases.inTableText
	ph.p.phases.inTableText.originalPhase = originalPhase
	return ph.p.phases.inTableText.processCharacters(tok)
}

// insertText is InTablePhase.insert_text in tinyhtml5 parser.py. If we get
// here there must be at least one non-whitespace character. Do the table
// magic!
func (ph *inTablePhase) insertText(tok *token) {
	ph.tree.insertFromTable = true
	ph.p.phases.inBody.processCharacters(tok)
	ph.tree.insertFromTable = false
}

// processStartTag dispatches as InTablePhase.start_tag_handler in tinyhtml5
// parser.py.
func (ph *inTablePhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.startTagHTML(tok)
	case "caption":
		ph.clearStackToTableContext()
		ph.tree.appendActiveFormattingElement(nil)
		ph.tree.insertElement(tok)
		ph.p.phase = ph.p.phases.inCaption
		return nil
	case "colgroup":
		ph.startTagColgroup(tok)
		return nil
	case "col":
		ph.startTagColgroup(impliedTagToken("colgroup", tokenStartTag))
		return tok
	case "tbody", "tfoot", "thead":
		ph.startTagRowgroup(tok)
		return nil
	case "td", "th", "tr":
		ph.startTagRowgroup(impliedTagToken("tbody", tokenStartTag))
		return tok
	case "table":
		ph.p.parseError("unexpected-start-tag-implies-end-tag", "startName", "table", "endName", "table")
		ph.p.phase.processEndTag(impliedTagToken("table", tokenEndTag))
		if ph.p.container == "" {
			return tok
		}
		return nil
	case "style", "script":
		return ph.p.phases.inHead.processStartTag(tok)
	case "input":
		if typ, ok := findAttr(tok.attrs, "type"); ok && asciiUpperToLower(typ) == "hidden" {
			ph.p.parseError("unexpected-hidden-input-in-table")
			ph.tree.insertElement(tok)
			// XXX associate with form.
			ph.tree.pop()
			return nil
		}
		ph.startTagOther(tok)
		return nil
	case "form":
		ph.p.parseError("unexpected-form-in-table")
		if ph.tree.formElement == nil {
			ph.tree.insertElement(tok)
			ph.tree.formElement = ph.tree.currentNode()
			ph.tree.pop()
		}
		return nil
	}
	ph.startTagOther(tok)
	return nil
}

// startTagColgroup is InTablePhase.start_tag_colgroup in tinyhtml5
// parser.py.
func (ph *inTablePhase) startTagColgroup(tok *token) {
	ph.clearStackToTableContext()
	ph.tree.insertElement(tok)
	ph.p.phase = ph.p.phases.inColumnGroup
}

// startTagRowgroup is InTablePhase.start_tag_rowgroup in tinyhtml5
// parser.py.
func (ph *inTablePhase) startTagRowgroup(tok *token) {
	ph.clearStackToTableContext()
	ph.tree.insertElement(tok)
	ph.p.phase = ph.p.phases.inTableBody
}

// startTagOther is InTablePhase.start_tag_other in tinyhtml5 parser.py.
func (ph *inTablePhase) startTagOther(tok *token) {
	ph.p.parseError("unexpected-start-tag-implies-table-voodoo", "name", tok.name)
	// Do the table magic!
	ph.tree.insertFromTable = true
	ph.p.phases.inBody.processStartTag(tok)
	ph.tree.insertFromTable = false
}

// processEndTag dispatches as InTablePhase.end_tag_handler in tinyhtml5
// parser.py.
func (ph *inTablePhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "table":
		ph.endTagTable(tok)
		return nil
	case "body", "caption", "col", "colgroup", "html", "tbody", "td",
		"tfoot", "th", "thead", "tr":
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
		return nil
	}
	ph.endTagOther(tok)
	return nil
}

// endTagTable is InTablePhase.end_tag_table in tinyhtml5 parser.py.
func (ph *inTablePhase) endTagTable(tok *token) {
	if !ph.tree.elementInScope("table", scopeTable) {
		// Fragment case.
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
		return
	}
	ph.tree.generateImpliedEndTags("")
	if ph.tree.currentNode().name != "table" {
		ph.p.parseError("end-tag-too-early-named", "gotName", "table", "expectedName", ph.tree.currentNode().name)
	}
	ph.tree.popUntil("table")
	ph.p.resetInsertionMode()
}

// endTagOther is InTablePhase.end_tag_other in tinyhtml5 parser.py.
func (ph *inTablePhase) endTagOther(tok *token) {
	ph.p.parseError("unexpected-end-tag-implies-table-voodoo", "name", tok.name)
	// Do the table magic!
	ph.tree.insertFromTable = true
	ph.p.phases.inBody.processEndTag(tok)
	ph.tree.insertFromTable = false
}

// inTableTextPhase is InTableTextPhase in tinyhtml5 parser.py.
type inTableTextPhase struct {
	phaseBase

	originalPhase   phase
	characterTokens []string
}

// flushCharacters is InTableTextPhase.flush_characters in tinyhtml5
// parser.py.
func (ph *inTableTextPhase) flushCharacters() {
	data := strings.Join(ph.characterTokens, "")
	switch {
	case strings.ContainsFunc(data, func(c rune) bool { return !spaceCharacter(c) }):
		ph.p.phases.inTable.insertText(&token{typ: tokenCharacters, data: data})
	case data != "":
		ph.tree.insertText(data)
	}
	ph.characterTokens = ph.characterTokens[:0]
}

// processComment is InTableTextPhase.process_comment in tinyhtml5
// parser.py.
func (ph *inTableTextPhase) processComment(tok *token) *token {
	ph.flushCharacters()
	ph.p.phase = ph.originalPhase
	return tok
}

// processEOF is InTableTextPhase.process_eof in tinyhtml5 parser.py.
func (ph *inTableTextPhase) processEOF() bool {
	ph.flushCharacters()
	ph.p.phase = ph.originalPhase
	return true
}

// processCharacters is InTableTextPhase.process_characters in tinyhtml5
// parser.py.
func (ph *inTableTextPhase) processCharacters(tok *token) *token {
	if tok.data == "\u0000" {
		return nil
	}
	ph.characterTokens = append(ph.characterTokens, tok.data)
	return nil
}

// processSpaceCharacters is InTableTextPhase.process_space_characters in
// tinyhtml5 parser.py. Pretty sure we should never reach here.
func (ph *inTableTextPhase) processSpaceCharacters(tok *token) *token {
	ph.characterTokens = append(ph.characterTokens, tok.data)
	return nil
}

// processStartTag is InTableTextPhase.process_start_tag in tinyhtml5
// parser.py.
func (ph *inTableTextPhase) processStartTag(tok *token) *token {
	ph.flushCharacters()
	ph.p.phase = ph.originalPhase
	return tok
}

// processEndTag is InTableTextPhase.process_end_tag in tinyhtml5 parser.py.
func (ph *inTableTextPhase) processEndTag(tok *token) *token {
	ph.flushCharacters()
	ph.p.phase = ph.originalPhase
	return tok
}

// inCaptionPhase is InCaptionPhase in tinyhtml5 parser.py.
// http://www.whatwg.org/specs/web-apps/current-work/#in-caption
type inCaptionPhase struct{ phaseBase }

// ignoreEndTagCaption is InCaptionPhase.ignore_end_tag_caption in tinyhtml5
// parser.py.
func (ph *inCaptionPhase) ignoreEndTagCaption() bool {
	return !ph.tree.elementInScope("caption", scopeTable)
}

// processEOF is InCaptionPhase.process_eof in tinyhtml5 parser.py.
func (ph *inCaptionPhase) processEOF() bool {
	ph.p.phases.inBody.processEOF()
	return false
}

// processCharacters is InCaptionPhase.process_characters in tinyhtml5
// parser.py.
func (ph *inCaptionPhase) processCharacters(tok *token) *token {
	return ph.p.phases.inBody.processCharacters(tok)
}

// processStartTag dispatches as InCaptionPhase.start_tag_handler in
// tinyhtml5 parser.py.
func (ph *inCaptionPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.startTagHTML(tok)
	case "caption", "col", "colgroup", "tbody", "td", "tfoot", "th",
		"thead", "tr":
		ph.p.parseError("unexpected-table-start-tag-in-caption")
		// XXX Have to duplicate logic here to find out if the tag is ignored.
		ignoreEndTag := ph.ignoreEndTagCaption()
		ph.p.phase.processEndTag(impliedTagToken("caption", tokenEndTag))
		if !ignoreEndTag {
			return tok
		}
		return nil
	}
	return ph.p.phases.inBody.processStartTag(tok)
}

// processEndTag dispatches as InCaptionPhase.end_tag_handler in tinyhtml5
// parser.py.
func (ph *inCaptionPhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "caption":
		ph.endTagCaption(tok)
		return nil
	case "table":
		ph.p.parseError("unexpected-table-end-tag-in-caption")
		ignoreEndTag := ph.ignoreEndTagCaption()
		ph.p.phase.processEndTag(impliedTagToken("caption", tokenEndTag))
		if !ignoreEndTag {
			return tok
		}
		return nil
	case "body", "col", "colgroup", "html", "tbody", "td", "tfoot", "th",
		"thead", "tr":
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
		return nil
	}
	return ph.p.phases.inBody.processEndTag(tok)
}

// endTagCaption is InCaptionPhase.end_tag_caption in tinyhtml5 parser.py.
func (ph *inCaptionPhase) endTagCaption(tok *token) {
	if ph.ignoreEndTagCaption() {
		// Fragment case.
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
		return
	}
	// AT this code is quite similar to end_tag_table in "InTable".
	ph.tree.generateImpliedEndTags("")
	if ph.tree.currentNode().name != "caption" {
		ph.p.parseError("expected-one-end-tag-but-got-another", "gotName", "caption", "expectedName", ph.tree.currentNode().name)
	}
	ph.tree.popUntil("caption")
	ph.tree.clearActiveFormattingElements()
	ph.p.phase = ph.p.phases.inTable
}

// inColumnGroupPhase is InColumnGroupPhase in tinyhtml5 parser.py.
// http://www.whatwg.org/specs/web-apps/current-work/#in-column
type inColumnGroupPhase struct{ phaseBase }

// ignoreEndTagColgroup is InColumnGroupPhase.ignore_end_tag_colgroup in
// tinyhtml5 parser.py.
func (ph *inColumnGroupPhase) ignoreEndTagColgroup() bool {
	return ph.tree.currentNode().name == "html"
}

// processEOF is InColumnGroupPhase.process_eof in tinyhtml5 parser.py.
func (ph *inColumnGroupPhase) processEOF() bool {
	if ph.tree.currentNode().name == "html" {
		return false
	}
	ignoreEndTag := ph.ignoreEndTagColgroup()
	ph.endTagColgroup(impliedTagToken("colgroup", tokenEndTag))
	return !ignoreEndTag
}

// processCharacters is InColumnGroupPhase.process_characters in tinyhtml5
// parser.py.
func (ph *inColumnGroupPhase) processCharacters(tok *token) *token {
	return ph.anythingElse(tok)
}

// anythingElse is the body that InColumnGroupPhase.process_characters,
// start_tag_other and end_tag_other share in tinyhtml5 parser.py.
func (ph *inColumnGroupPhase) anythingElse(tok *token) *token {
	ignoreEndTag := ph.ignoreEndTagColgroup()
	ph.endTagColgroup(impliedTagToken("colgroup", tokenEndTag))
	if !ignoreEndTag {
		return tok
	}
	return nil
}

// processStartTag dispatches as InColumnGroupPhase.start_tag_handler in
// tinyhtml5 parser.py.
func (ph *inColumnGroupPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.startTagHTML(tok)
	case "col":
		ph.tree.insertElement(tok)
		ph.tree.pop()
		tok.selfClosingAcknowledged = true
		return nil
	}
	return ph.anythingElse(tok)
}

// processEndTag dispatches as InColumnGroupPhase.end_tag_handler in
// tinyhtml5 parser.py.
func (ph *inColumnGroupPhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "colgroup":
		ph.endTagColgroup(tok)
		return nil
	case "col":
		ph.p.parseError("no-end-tag", "name", "col")
		return nil
	}
	return ph.anythingElse(tok)
}

// endTagColgroup is InColumnGroupPhase.end_tag_colgroup in tinyhtml5
// parser.py.
func (ph *inColumnGroupPhase) endTagColgroup(tok *token) {
	if ph.ignoreEndTagColgroup() {
		// Fragment case.
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
		return
	}
	ph.tree.pop()
	ph.p.phase = ph.p.phases.inTable
}

// inTableBodyPhase is InTableBodyPhase in tinyhtml5 parser.py.
// http://www.whatwg.org/specs/web-apps/current-work/#in-table0
type inTableBodyPhase struct{ phaseBase }

// clearStackToTableBodyContext is
// InTableBodyPhase._clear_stack_to_table_body_context in tinyhtml5
// parser.py.
func (ph *inTableBodyPhase) clearStackToTableBodyContext() {
	for len(ph.tree.openElements) > 1 {
		switch ph.tree.currentNode().name {
		case "tbody", "tfoot", "thead", "html":
			return
		}
		ph.tree.pop()
	}
}

// processEOF is InTableBodyPhase.process_eof in tinyhtml5 parser.py.
func (ph *inTableBodyPhase) processEOF() bool {
	ph.p.phases.inTable.processEOF()
	return false
}

// processSpaceCharacters is InTableBodyPhase.process_space_characters in
// tinyhtml5 parser.py.
func (ph *inTableBodyPhase) processSpaceCharacters(tok *token) *token {
	return ph.p.phases.inTable.processSpaceCharacters(tok)
}

// processCharacters is InTableBodyPhase.process_characters in tinyhtml5
// parser.py.
func (ph *inTableBodyPhase) processCharacters(tok *token) *token {
	return ph.p.phases.inTable.processCharacters(tok)
}

// rowGroupInScope is the test that InTableBodyPhase.start_tag_table_other
// and end_tag_table share in tinyhtml5 parser.py.
func (ph *inTableBodyPhase) rowGroupInScope() bool {
	return ph.tree.elementInScope("tbody", scopeTable) ||
		ph.tree.elementInScope("thead", scopeTable) ||
		ph.tree.elementInScope("tfoot", scopeTable)
}

// processStartTag dispatches as InTableBodyPhase.start_tag_handler in
// tinyhtml5 parser.py.
func (ph *inTableBodyPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.startTagHTML(tok)
	case "tr":
		ph.startTagTr(tok)
		return nil
	case "td", "th":
		ph.p.parseError("unexpected-cell-in-table-body", "name", tok.name)
		ph.startTagTr(impliedTagToken("tr", tokenStartTag))
		return tok
	case "caption", "col", "colgroup", "tbody", "tfoot", "thead":
		// XXX AT Any ideas on how to share this with end_tag_table?
		if ph.rowGroupInScope() {
			ph.clearStackToTableBodyContext()
			ph.endTagTableRowgroup(impliedTagToken(ph.tree.currentNode().name, tokenEndTag))
			return tok
		}
		// Fragment case.
		ph.p.parseError("unexpected-start-tag-out-of-table", "name", tok.name)
		return nil
	}
	return ph.p.phases.inTable.processStartTag(tok)
}

// startTagTr is InTableBodyPhase.start_tag_tr in tinyhtml5 parser.py.
func (ph *inTableBodyPhase) startTagTr(tok *token) {
	ph.clearStackToTableBodyContext()
	ph.tree.insertElement(tok)
	ph.p.phase = ph.p.phases.inRow
}

// processEndTag dispatches as InTableBodyPhase.end_tag_handler in tinyhtml5
// parser.py.
func (ph *inTableBodyPhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "tbody", "tfoot", "thead":
		ph.endTagTableRowgroup(tok)
		return nil
	case "table":
		if ph.rowGroupInScope() {
			ph.clearStackToTableBodyContext()
			ph.endTagTableRowgroup(impliedTagToken(ph.tree.currentNode().name, tokenEndTag))
			return tok
		}
		// Fragment case.
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
		return nil
	case "body", "caption", "col", "colgroup", "html", "td", "th", "tr":
		ph.p.parseError("unexpected-end-tag-in-table-body", "name", tok.name)
		return nil
	}
	return ph.p.phases.inTable.processEndTag(tok)
}

// endTagTableRowgroup is InTableBodyPhase.end_tag_table_rowgroup in
// tinyhtml5 parser.py.
func (ph *inTableBodyPhase) endTagTableRowgroup(tok *token) {
	if ph.tree.elementInScope(tok.name, scopeTable) {
		ph.clearStackToTableBodyContext()
		ph.tree.pop()
		ph.p.phase = ph.p.phases.inTable
	} else {
		ph.p.parseError("unexpected-end-tag-in-table-body", "name", tok.name)
	}
}

// inRowPhase is InRowPhase in tinyhtml5 parser.py.
// http://www.whatwg.org/specs/web-apps/current-work/#in-row
type inRowPhase struct{ phaseBase }

// clearStackToTableRowContext is InRowPhase._clear_stack_to_table_row_context
// in tinyhtml5 parser.py.
func (ph *inRowPhase) clearStackToTableRowContext() {
	for len(ph.tree.openElements) > 1 {
		name := ph.tree.currentNode().name
		if name == "tr" || name == "html" {
			return
		}
		ph.p.parseError("unexpected-implied-end-tag-in-table-row", "name", name)
		ph.tree.pop()
	}
}

// ignoreEndTagTr is InRowPhase.ignore_end_tag_tr in tinyhtml5 parser.py.
func (ph *inRowPhase) ignoreEndTagTr() bool {
	return !ph.tree.elementInScope("tr", scopeTable)
}

// processEOF is InRowPhase.process_eof in tinyhtml5 parser.py.
func (ph *inRowPhase) processEOF() bool {
	ph.p.phases.inTable.processEOF()
	return false
}

// processSpaceCharacters is InRowPhase.process_space_characters in
// tinyhtml5 parser.py.
func (ph *inRowPhase) processSpaceCharacters(tok *token) *token {
	return ph.p.phases.inTable.processSpaceCharacters(tok)
}

// processCharacters is InRowPhase.process_characters in tinyhtml5
// parser.py.
func (ph *inRowPhase) processCharacters(tok *token) *token {
	return ph.p.phases.inTable.processCharacters(tok)
}

// processStartTag dispatches as InRowPhase.start_tag_handler in tinyhtml5
// parser.py.
func (ph *inRowPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.startTagHTML(tok)
	case "td", "th":
		ph.clearStackToTableRowContext()
		ph.tree.insertElement(tok)
		ph.p.phase = ph.p.phases.inCell
		ph.tree.appendActiveFormattingElement(nil)
		return nil
	case "caption", "col", "colgroup", "tbody", "tfoot", "thead", "tr":
		ignoreEndTag := ph.ignoreEndTagTr()
		ph.endTagTr(impliedTagToken("tr", tokenEndTag))
		// XXX how are we sure it's always ignored in the fragment case?
		if !ignoreEndTag {
			return tok
		}
		return nil
	}
	return ph.p.phases.inTable.processStartTag(tok)
}

// processEndTag dispatches as InRowPhase.end_tag_handler in tinyhtml5
// parser.py.
func (ph *inRowPhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "tr":
		ph.endTagTr(tok)
		return nil
	case "table":
		ignoreEndTag := ph.ignoreEndTagTr()
		ph.endTagTr(impliedTagToken("tr", tokenEndTag))
		// Reprocess the current tag if the tr end tag was not ignored.
		// XXX how are we sure it's always ignored in the fragment case?
		if !ignoreEndTag {
			return tok
		}
		return nil
	case "tbody", "tfoot", "thead":
		if ph.tree.elementInScope(tok.name, scopeTable) {
			ph.endTagTr(impliedTagToken("tr", tokenEndTag))
			return tok
		}
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
		return nil
	case "body", "caption", "col", "colgroup", "html", "td", "th":
		ph.p.parseError("unexpected-end-tag-in-table-row", "name", tok.name)
		return nil
	}
	return ph.p.phases.inTable.processEndTag(tok)
}

// endTagTr is InRowPhase.end_tag_tr in tinyhtml5 parser.py.
func (ph *inRowPhase) endTagTr(tok *token) {
	if ph.ignoreEndTagTr() {
		// Fragment case.
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
		return
	}
	ph.clearStackToTableRowContext()
	ph.tree.pop()
	ph.p.phase = ph.p.phases.inTableBody
}

// inCellPhase is InCellPhase in tinyhtml5 parser.py.
// http://www.whatwg.org/specs/web-apps/current-work/#in-cell
type inCellPhase struct{ phaseBase }

// closeCell is InCellPhase._close_cell in tinyhtml5 parser.py.
func (ph *inCellPhase) closeCell() {
	if ph.tree.elementInScope("td", scopeTable) {
		ph.endTagTableCell(impliedTagToken("td", tokenEndTag))
	} else if ph.tree.elementInScope("th", scopeTable) {
		ph.endTagTableCell(impliedTagToken("th", tokenEndTag))
	}
}

// processEOF is InCellPhase.process_eof in tinyhtml5 parser.py.
func (ph *inCellPhase) processEOF() bool {
	ph.p.phases.inBody.processEOF()
	return false
}

// processCharacters is InCellPhase.process_characters in tinyhtml5
// parser.py.
func (ph *inCellPhase) processCharacters(tok *token) *token {
	return ph.p.phases.inBody.processCharacters(tok)
}

// processStartTag dispatches as InCellPhase.start_tag_handler in tinyhtml5
// parser.py.
func (ph *inCellPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.startTagHTML(tok)
	case "caption", "col", "colgroup", "tbody", "td", "tfoot", "th",
		"thead", "tr":
		if ph.tree.elementInScope("td", scopeTable) || ph.tree.elementInScope("th", scopeTable) {
			ph.closeCell()
			return tok
		}
		// Fragment case.
		ph.p.parseError("unexpected-start-tag-out-of-table-cell", "name", tok.name)
		return nil
	}
	return ph.p.phases.inBody.processStartTag(tok)
}

// processEndTag dispatches as InCellPhase.end_tag_handler in tinyhtml5
// parser.py.
func (ph *inCellPhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "td", "th":
		ph.endTagTableCell(tok)
		return nil
	case "body", "caption", "col", "colgroup", "html":
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
		return nil
	case "table", "tbody", "tfoot", "thead", "tr":
		if ph.tree.elementInScope(tok.name, scopeTable) {
			ph.closeCell()
			return tok
		}
		// Sometimes fragment case.
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
		return nil
	}
	return ph.p.phases.inBody.processEndTag(tok)
}

// endTagTableCell is InCellPhase.end_tag_table_cell in tinyhtml5 parser.py.
func (ph *inCellPhase) endTagTableCell(tok *token) {
	if !ph.tree.elementInScope(tok.name, scopeTable) {
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
		return
	}
	ph.tree.generateImpliedEndTags(tok.name)
	if ph.tree.currentNode().name != tok.name {
		ph.p.parseError("unexpected-cell-end-tag", "name", tok.name)
		ph.tree.popUntil(tok.name)
	} else {
		ph.tree.pop()
	}
	ph.tree.clearActiveFormattingElements()
	ph.p.phase = ph.p.phases.inRow
}

// inSelectPhase is InSelectPhase in tinyhtml5 parser.py.
// http://www.whatwg.org/specs/web-apps/current-work/#in-select
type inSelectPhase struct{ phaseBase }

// processEOF is InSelectPhase.process_eof in tinyhtml5 parser.py.
func (ph *inSelectPhase) processEOF() bool {
	if ph.tree.currentNode().name != "html" {
		ph.p.parseError("eof-in-select")
	}
	return false
}

// processCharacters is InSelectPhase.process_characters in tinyhtml5
// parser.py.
func (ph *inSelectPhase) processCharacters(tok *token) *token {
	if tok.data == "\u0000" {
		return nil
	}
	ph.tree.insertText(tok.data)
	return nil
}

// processStartTag dispatches as InSelectPhase.start_tag_handler in
// tinyhtml5 parser.py.
func (ph *inSelectPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.startTagHTML(tok)
	case "option":
		// We need to imply </option> if <option> is the current node.
		if ph.tree.currentNode().name == "option" {
			ph.tree.pop()
		}
		ph.tree.insertElement(tok)
		return nil
	case "optgroup":
		if ph.tree.currentNode().name == "option" {
			ph.tree.pop()
		}
		if ph.tree.currentNode().name == "optgroup" {
			ph.tree.pop()
		}
		ph.tree.insertElement(tok)
		return nil
	case "select":
		ph.p.parseError("unexpected-select-in-select")
		ph.endTagSelect(impliedTagToken("select", tokenEndTag))
		return nil
	case "input", "keygen", "textarea":
		ph.p.parseError("unexpected-input-in-select")
		if ph.tree.elementInScope("select", scopeSelect) {
			ph.endTagSelect(impliedTagToken("select", tokenEndTag))
			return tok
		}
		return nil
	case "script":
		return ph.p.phases.inHead.processStartTag(tok)
	}
	ph.p.parseError("unexpected-start-tag-in-select", "name", tok.name)
	return nil
}

// processEndTag dispatches as InSelectPhase.end_tag_handler in tinyhtml5
// parser.py.
func (ph *inSelectPhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "option":
		if ph.tree.currentNode().name == "option" {
			ph.tree.pop()
		} else {
			ph.p.parseError("unexpected-end-tag-in-select", "name", "option")
		}
	case "optgroup":
		// </optgroup> implicitly closes <option>.
		open := ph.tree.openElements
		if ph.tree.currentNode().name == "option" && len(open) > 1 &&
			open[len(open)-2].name == "optgroup" {
			ph.tree.pop()
		}
		// It also closes </optgroup>.
		if ph.tree.currentNode().name == "optgroup" {
			ph.tree.pop()
		} else {
			// But nothing else.
			ph.p.parseError("unexpected-end-tag-in-select", "name", "optgroup")
		}
	case "select":
		ph.endTagSelect(tok)
	default:
		ph.p.parseError("unexpected-end-tag-in-select", "name", tok.name)
	}
	return nil
}

// endTagSelect is InSelectPhase.end_tag_select in tinyhtml5 parser.py.
func (ph *inSelectPhase) endTagSelect(tok *token) {
	if !ph.tree.elementInScope("select", scopeSelect) {
		// Fragment case.
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
		return
	}
	ph.tree.popUntil("select")
	ph.p.resetInsertionMode()
}

// inSelectInTablePhase is InSelectInTablePhase in tinyhtml5 parser.py.
type inSelectInTablePhase struct{ phaseBase }

// processEOF is InSelectInTablePhase.process_eof in tinyhtml5 parser.py.
func (ph *inSelectInTablePhase) processEOF() bool {
	ph.p.phases.inSelect.processEOF()
	return false
}

// processCharacters is InSelectInTablePhase.process_characters in tinyhtml5
// parser.py.
func (ph *inSelectInTablePhase) processCharacters(tok *token) *token {
	return ph.p.phases.inSelect.processCharacters(tok)
}

// processStartTag dispatches as InSelectInTablePhase.start_tag_handler in
// tinyhtml5 parser.py.
func (ph *inSelectInTablePhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "caption", "table", "tbody", "tfoot", "thead", "tr", "td", "th":
		ph.p.parseError("unexpected-table-element-start-tag-in-select-in-table", "name", tok.name)
		ph.p.phases.inSelect.processEndTag(impliedTagToken("select", tokenEndTag))
		return tok
	}
	return ph.p.phases.inSelect.processStartTag(tok)
}

// processEndTag dispatches as InSelectInTablePhase.end_tag_handler in
// tinyhtml5 parser.py.
func (ph *inSelectInTablePhase) processEndTag(tok *token) *token {
	switch tok.name {
	case "caption", "table", "tbody", "tfoot", "thead", "tr", "td", "th":
		ph.p.parseError("unexpected-table-element-end-tag-in-select-in-table", "name", tok.name)
		if ph.tree.elementInScope(tok.name, scopeTable) {
			ph.p.phases.inSelect.processEndTag(impliedTagToken("select", tokenEndTag))
			return tok
		}
		return nil
	}
	return ph.p.phases.inSelect.processEndTag(tok)
}

// inForeignContentPhase is InForeignContentPhase in tinyhtml5 parser.py.
type inForeignContentPhase struct{ phaseBase }

// breakoutElement is InForeignContentPhase.breakout_elements in tinyhtml5
// parser.py.
func breakoutElement(name string) bool {
	switch name {
	case "b", "big", "blockquote", "body", "br", "center", "code", "dd", "div", "dl",
		"dt", "em", "embed", "h1", "h2", "h3", "h4", "h5", "h6", "head", "hr", "i",
		"img", "li", "listing", "menu", "meta", "nobr", "ol", "p", "pre", "ruby", "s",
		"small", "span", "strong", "strike", "sub", "sup", "table", "tt", "u", "ul",
		"var":
		return true
	}
	return false
}

// adjustSVGTagNames is InForeignContentPhase.adjust_svg_tag_names in
// tinyhtml5 parser.py.
func (ph *inForeignContentPhase) adjustSVGTagNames(tok *token) {
	if name, ok := svgTagName(tok.name); ok {
		tok.name = name
	}
}

// svgTagName is the dict replacements of
// InForeignContentPhase.adjust_svg_tag_names in tinyhtml5 parser.py.
func svgTagName(name string) (string, bool) {
	switch name {
	case "altglyph":
		return "altGlyph", true
	case "altglyphdef":
		return "altGlyphDef", true
	case "altglyphitem":
		return "altGlyphItem", true
	case "animatecolor":
		return "animateColor", true
	case "animatemotion":
		return "animateMotion", true
	case "animatetransform":
		return "animateTransform", true
	case "clippath":
		return "clipPath", true
	case "feblend":
		return "feBlend", true
	case "fecolormatrix":
		return "feColorMatrix", true
	case "fecomponenttransfer":
		return "feComponentTransfer", true
	case "fecomposite":
		return "feComposite", true
	case "feconvolvematrix":
		return "feConvolveMatrix", true
	case "fediffuselighting":
		return "feDiffuseLighting", true
	case "fedisplacementmap":
		return "feDisplacementMap", true
	case "fedistantlight":
		return "feDistantLight", true
	case "feflood":
		return "feFlood", true
	case "fefunca":
		return "feFuncA", true
	case "fefuncb":
		return "feFuncB", true
	case "fefuncg":
		return "feFuncG", true
	case "fefuncr":
		return "feFuncR", true
	case "fegaussianblur":
		return "feGaussianBlur", true
	case "feimage":
		return "feImage", true
	case "femerge":
		return "feMerge", true
	case "femergenode":
		return "feMergeNode", true
	case "femorphology":
		return "feMorphology", true
	case "feoffset":
		return "feOffset", true
	case "fepointlight":
		return "fePointLight", true
	case "fespecularlighting":
		return "feSpecularLighting", true
	case "fespotlight":
		return "feSpotLight", true
	case "fetile":
		return "feTile", true
	case "feturbulence":
		return "feTurbulence", true
	case "foreignobject":
		return "foreignObject", true
	case "glyphref":
		return "glyphRef", true
	case "lineargradient":
		return "linearGradient", true
	case "radialgradient":
		return "radialGradient", true
	case "textpath":
		return "textPath", true
	}
	return "", false
}

// processEOF has no port: the parser never gives the end of the input to
// InForeignContentPhase in tinyhtml5 parser.py.
func (*inForeignContentPhase) processEOF() bool {
	return false
}

// processCharacters is InForeignContentPhase.process_characters in
// tinyhtml5 parser.py.
func (ph *inForeignContentPhase) processCharacters(tok *token) *token {
	if tok.data == "\u0000" {
		tok.data = "�"
	} else if ph.p.framesetOK && strings.ContainsFunc(tok.data, func(c rune) bool { return !spaceCharacter(c) }) {
		ph.p.framesetOK = false
	}
	return ph.phaseBase.processCharacters(tok)
}

// processStartTag is InForeignContentPhase.process_start_tag in tinyhtml5
// parser.py.
func (ph *inForeignContentPhase) processStartTag(tok *token) *token {
	currentNode := ph.tree.currentNode()
	fontAttr := false
	if tok.name == "font" {
		for _, a := range tok.attrs {
			if a.Namespace == "" && (a.Name == "color" || a.Name == "face" || a.Name == "size") {
				fontAttr = true
				break
			}
		}
	}
	if breakoutElement(tok.name) || fontAttr {
		ph.p.parseError("unexpected-html-element-in-foreign-content", "name", tok.name)
		for len(ph.tree.openElements) > 1 {
			current := ph.tree.currentNode()
			if current.namespace == ph.tree.defaultNamespace ||
				ph.p.isHTMLIntegrationPoint(current) ||
				ph.p.isMathMLTextIntegrationPoint(current) {
				break
			}
			ph.tree.pop()
		}
		return tok
	}

	switch currentNode.namespace {
	case NamespaceMathML:
		ph.p.adjustMathMLAttributes(tok)
	case NamespaceSVG:
		ph.adjustSVGTagNames(tok)
		ph.p.adjustSVGAttributes(tok)
	}
	ph.p.adjustForeignAttributes(tok)
	tok.namespace = currentNode.namespace
	ph.tree.insertElement(tok)
	if tok.selfClosing {
		ph.tree.pop()
		tok.selfClosingAcknowledged = true
	}
	return nil
}

// processEndTag is InForeignContentPhase.process_end_tag in tinyhtml5
// parser.py.
func (ph *inForeignContentPhase) processEndTag(tok *token) *token {
	nodeIndex := len(ph.tree.openElements) - 1
	node := ph.tree.currentNode()
	if asciiUpperToLower(node.name) != tok.name {
		ph.p.parseError("unexpected-end-tag", "name", tok.name)
	}

	for {
		if asciiUpperToLower(node.name) == tok.name {
			// XXX this isn't in the spec but it seems necessary.
			if ph.p.phase == ph.p.phases.inTableText {
				ph.p.phases.inTableText.flushCharacters()
				ph.p.phase = ph.p.phases.inTableText.originalPhase
			}
			for len(ph.tree.openElements) > 0 && ph.tree.pop() != node {
			}
			return nil
		}
		nodeIndex--
		if nodeIndex < 0 {
			return nil
		}

		node = ph.tree.openElements[nodeIndex]
		if node.namespace != ph.tree.defaultNamespace {
			continue
		}
		return ph.p.phase.processEndTag(tok)
	}
}

// afterBodyPhase is AfterBodyPhase in tinyhtml5 parser.py.
type afterBodyPhase struct{ phaseBase }

// processEOF is AfterBodyPhase.process_eof in tinyhtml5 parser.py. Stop
// parsing.
func (*afterBodyPhase) processEOF() bool {
	return false
}

// processComment is AfterBodyPhase.process_comment in tinyhtml5 parser.py.
// This is needed because data is to be appended to the <html> element here
// and not to whatever is currently open.
func (ph *afterBodyPhase) processComment(tok *token) *token {
	ph.tree.insertComment(tok, ph.tree.openElements[0])
	return nil
}

// processCharacters is AfterBodyPhase.process_characters in tinyhtml5
// parser.py.
func (ph *afterBodyPhase) processCharacters(tok *token) *token {
	ph.p.parseError("unexpected-char-after-body")
	ph.p.phase = ph.p.phases.inBody
	return tok
}

// processStartTag dispatches as AfterBodyPhase.start_tag_handler in
// tinyhtml5 parser.py.
func (ph *afterBodyPhase) processStartTag(tok *token) *token {
	if tok.name == "html" {
		return ph.p.phases.inBody.processStartTag(tok)
	}
	ph.p.parseError("unexpected-start-tag-after-body", "name", tok.name)
	ph.p.phase = ph.p.phases.inBody
	return tok
}

// processEndTag dispatches as AfterBodyPhase.end_tag_handler in tinyhtml5
// parser.py.
func (ph *afterBodyPhase) processEndTag(tok *token) *token {
	if tok.name == "html" {
		if ph.p.container != "" {
			ph.p.parseError("unexpected-end-tag-after-body-innerhtml")
		} else {
			ph.p.phase = ph.p.phases.afterAfterBody
		}
		return nil
	}
	ph.p.parseError("unexpected-end-tag-after-body", "name", tok.name)
	ph.p.phase = ph.p.phases.inBody
	return tok
}

// inFramesetPhase is InFramesetPhase in tinyhtml5 parser.py.
// http://www.whatwg.org/specs/web-apps/current-work/#in-frameset
type inFramesetPhase struct{ phaseBase }

// processEOF is InFramesetPhase.process_eof in tinyhtml5 parser.py.
func (ph *inFramesetPhase) processEOF() bool {
	if ph.tree.currentNode().name != "html" {
		ph.p.parseError("eof-in-frameset")
	}
	return false
}

// processCharacters is InFramesetPhase.process_characters in tinyhtml5
// parser.py.
func (ph *inFramesetPhase) processCharacters(*token) *token {
	ph.p.parseError("unexpected-char-in-frameset")
	return nil
}

// processStartTag dispatches as InFramesetPhase.start_tag_handler in
// tinyhtml5 parser.py.
func (ph *inFramesetPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.startTagHTML(tok)
	case "frameset":
		ph.tree.insertElement(tok)
		return nil
	case "frame":
		ph.tree.insertElement(tok)
		ph.tree.pop()
		return nil
	case "noframes":
		return ph.p.phases.inBody.processStartTag(tok)
	}
	ph.p.parseError("unexpected-start-tag-in-frameset", "name", tok.name)
	return nil
}

// processEndTag dispatches as InFramesetPhase.end_tag_handler in tinyhtml5
// parser.py.
func (ph *inFramesetPhase) processEndTag(tok *token) *token {
	if tok.name != "frameset" {
		ph.p.parseError("unexpected-end-tag-in-frameset", "name", tok.name)
		return nil
	}
	if ph.tree.currentNode().name == "html" {
		// Fragment case.
		ph.p.parseError("unexpected-frameset-in-frameset-innerhtml")
	} else {
		ph.tree.pop()
	}
	if ph.p.container == "" && ph.tree.currentNode().name != "frameset" {
		// If we're not in fragment mode and the current node is not a
		// "frameset" element (anymore) then switch.
		ph.p.phase = ph.p.phases.afterFrameset
	}
	return nil
}

// afterFramesetPhase is AfterFramesetPhase in tinyhtml5 parser.py.
// http://www.whatwg.org/specs/web-apps/current-work/#after3
type afterFramesetPhase struct{ phaseBase }

// processEOF is AfterFramesetPhase.process_eof in tinyhtml5 parser.py. Stop
// parsing.
func (*afterFramesetPhase) processEOF() bool {
	return false
}

// processCharacters is AfterFramesetPhase.process_characters in tinyhtml5
// parser.py.
func (ph *afterFramesetPhase) processCharacters(*token) *token {
	ph.p.parseError("unexpected-char-after-frameset")
	return nil
}

// processStartTag dispatches as AfterFramesetPhase.start_tag_handler in
// tinyhtml5 parser.py.
func (ph *afterFramesetPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.startTagHTML(tok)
	case "noframes":
		return ph.p.phases.inHead.processStartTag(tok)
	}
	ph.p.parseError("unexpected-start-tag-after-frameset", "name", tok.name)
	return nil
}

// processEndTag dispatches as AfterFramesetPhase.end_tag_handler in
// tinyhtml5 parser.py.
func (ph *afterFramesetPhase) processEndTag(tok *token) *token {
	if tok.name == "html" {
		ph.p.phase = ph.p.phases.afterAfterFrameset
		return nil
	}
	ph.p.parseError("unexpected-end-tag-after-frameset", "name", tok.name)
	return nil
}

// afterAfterBodyPhase is AfterAfterBodyPhase in tinyhtml5 parser.py.
type afterAfterBodyPhase struct{ phaseBase }

// processEOF is AfterAfterBodyPhase.process_eof in tinyhtml5 parser.py.
func (*afterAfterBodyPhase) processEOF() bool {
	return false
}

// processComment is AfterAfterBodyPhase.process_comment in tinyhtml5
// parser.py.
func (ph *afterAfterBodyPhase) processComment(tok *token) *token {
	ph.tree.insertComment(tok, ph.tree.document)
	return nil
}

// processSpaceCharacters is AfterAfterBodyPhase.process_space_characters in
// tinyhtml5 parser.py.
func (ph *afterAfterBodyPhase) processSpaceCharacters(tok *token) *token {
	return ph.p.phases.inBody.processSpaceCharacters(tok)
}

// processCharacters is AfterAfterBodyPhase.process_characters in tinyhtml5
// parser.py.
func (ph *afterAfterBodyPhase) processCharacters(tok *token) *token {
	ph.p.parseError("expected-eof-but-got-char")
	ph.p.phase = ph.p.phases.inBody
	return tok
}

// processStartTag dispatches as AfterAfterBodyPhase.start_tag_handler in
// tinyhtml5 parser.py.
func (ph *afterAfterBodyPhase) processStartTag(tok *token) *token {
	if tok.name == "html" {
		return ph.p.phases.inBody.processStartTag(tok)
	}
	ph.p.parseError("expected-eof-but-got-start-tag", "name", tok.name)
	ph.p.phase = ph.p.phases.inBody
	return tok
}

// processEndTag is AfterAfterBodyPhase.process_end_tag in tinyhtml5
// parser.py.
func (ph *afterAfterBodyPhase) processEndTag(tok *token) *token {
	ph.p.parseError("expected-eof-but-got-end-tag", "name", tok.name)
	ph.p.phase = ph.p.phases.inBody
	return tok
}

// afterAfterFramesetPhase is AfterAfterFramesetPhase in tinyhtml5
// parser.py.
type afterAfterFramesetPhase struct{ phaseBase }

// processEOF is AfterAfterFramesetPhase.process_eof in tinyhtml5 parser.py.
func (*afterAfterFramesetPhase) processEOF() bool {
	return false
}

// processComment is AfterAfterFramesetPhase.process_comment in tinyhtml5
// parser.py.
func (ph *afterAfterFramesetPhase) processComment(tok *token) *token {
	ph.tree.insertComment(tok, ph.tree.document)
	return nil
}

// processSpaceCharacters is AfterAfterFramesetPhase.process_space_characters
// in tinyhtml5 parser.py.
func (ph *afterAfterFramesetPhase) processSpaceCharacters(tok *token) *token {
	return ph.p.phases.inBody.processSpaceCharacters(tok)
}

// processCharacters is AfterAfterFramesetPhase.process_characters in
// tinyhtml5 parser.py.
func (ph *afterAfterFramesetPhase) processCharacters(*token) *token {
	ph.p.parseError("expected-eof-but-got-char")
	return nil
}

// processStartTag dispatches as AfterAfterFramesetPhase.start_tag_handler in
// tinyhtml5 parser.py.
func (ph *afterAfterFramesetPhase) processStartTag(tok *token) *token {
	switch tok.name {
	case "html":
		return ph.p.phases.inBody.processStartTag(tok)
	case "noframes":
		return ph.p.phases.inHead.processStartTag(tok)
	}
	ph.p.parseError("expected-eof-but-got-start-tag", "name", tok.name)
	return nil
}

// processEndTag is AfterAfterFramesetPhase.process_end_tag in tinyhtml5
// parser.py.
func (ph *afterAfterFramesetPhase) processEndTag(tok *token) *token {
	ph.p.parseError("expected-eof-but-got-end-tag", "name", tok.name)
	return nil
}

// impliedTagToken is implied_tag_token in tinyhtml5 parser.py.
func impliedTagToken(name string, typ tokenType) *token {
	return &token{typ: typ, name: name}
}
