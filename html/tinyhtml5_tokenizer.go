// Copyright (c) 2006-2013 James Graham and other contributors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package html

// This file ports tinyhtml5 tokenizer.py (D49). Each state of upstream is a
// method of htmlTokenizer and a value of tokenizerState. Upstream builds the
// name, the attributes and the text of a token with "+=". The port appends
// them to slices of bytes, so that a long token takes linear time.

import (
	"iter"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// hasKeysWithPrefix is has_keys_with_prefix in tinyhtml5 tokenizer.py.
func hasKeysWithPrefix(prefix string) bool {
	i, found := slices.BinarySearchFunc(entities, prefix, func(e entity, p string) int {
		return strings.Compare(e.name, p)
	})
	if found {
		return true
	}
	if i == len(entities) {
		return false
	}
	return strings.HasPrefix(entities[i].name, prefix)
}

// entityValue returns the text of a named character reference.
func entityValue(name string) (string, bool) {
	i, found := slices.BinarySearchFunc(entities, name, func(e entity, p string) int {
		return strings.Compare(e.name, p)
	})
	if !found {
		return "", false
	}
	return entities[i].value, true
}

// longestPrefix is longest_prefix in tinyhtml5 tokenizer.py. It returns
// false where upstream raises KeyError.
func longestPrefix(prefix string) (string, bool) {
	if _, ok := entityValue(prefix); ok {
		return prefix, true
	}
	for i := 1; i <= len(prefix); i++ {
		if _, ok := entityValue(prefix[:len(prefix)-i]); ok {
			return prefix[:len(prefix)-i], true
		}
	}
	return "", false
}

// rawAttr is an attribute of a tag that the tokenizer builds.
type rawAttr struct {
	name  []byte
	value []byte
}

// token is a token of tinyhtml5, which upstream holds in a dict.
type token struct {
	typ tokenType

	// name is the name of a tag or of a doctype.
	name string

	// data is the text of characters or of a comment, or the code of a parse
	// error.
	data string

	// attrs are the attributes of a start tag, in order, with no name twice.
	attrs []Attribute

	// namespace is the namespace that the parser gives a foreign element, or
	// empty for the default namespace.
	namespace string

	selfClosing             bool
	selfClosingAcknowledged bool

	// publicID and systemID are the identifiers of a doctype. hasPublicID
	// and hasSystemID are false where upstream holds None.
	publicID    string
	systemID    string
	hasPublicID bool
	hasSystemID bool
	correct     bool

	// datavars are the names and the values of the data of a parse error.
	datavars []string

	// The parts of the token that the tokenizer builds.
	nameB    []byte
	dataB    []byte
	publicB  []byte
	systemB  []byte
	rawAttrs []rawAttr
	attrSet  map[string]bool
}

// lastAttr returns the attribute that the tokenizer builds.
func (t *token) lastAttr() *rawAttr {
	return &t.rawAttrs[len(t.rawAttrs)-1]
}

// finishDoctype sets the strings of a doctype from its parts.
func (t *token) finishDoctype() {
	t.name = string(t.nameB)
	t.publicID = string(t.publicB)
	t.systemID = string(t.systemB)
}

// htmlTokenizer is HTMLTokenizer in tinyhtml5 tokenizer.py.
type htmlTokenizer struct {
	stream *htmlInputStream
	parser *htmlParser

	// state is the state of the tokenizer.
	state tokenizerState

	// currentToken is the token being processed.
	currentToken *token

	temporaryBuffer []byte

	tokenQueue []*token
}

// newHTMLTokenizer is HTMLTokenizer.__init__ in tinyhtml5 tokenizer.py.
func newHTMLTokenizer(stream *htmlInputStream, parser *htmlParser) *htmlTokenizer {
	// Setup the initial tokenizer state.
	return &htmlTokenizer{stream: stream, parser: parser, state: dataState}
}

// tokens is HTMLTokenizer.__iter__ in tinyhtml5 tokenizer.py. It runs the
// states and yields each token. It stops when a state returns false at the
// end of the input.
func (t *htmlTokenizer) tokens() iter.Seq[*token] {
	return func(yield func(*token) bool) {
		t.tokenQueue = t.tokenQueue[:0]
		for t.run() {
			for len(t.stream.errors) > 0 {
				code := t.stream.errors[0]
				t.stream.errors = t.stream.errors[1:]
				if !yield(&token{typ: tokenParseError, data: code}) {
					return
				}
			}
			for len(t.tokenQueue) > 0 {
				tok := t.tokenQueue[0]
				t.tokenQueue[0] = nil
				t.tokenQueue = t.tokenQueue[1:]
				if !yield(tok) {
					return
				}
			}
		}
	}
}

// parseError is HTMLTokenizer.parse_error in tinyhtml5 tokenizer.py. It adds
// a parse error to the token queue.
func (t *htmlTokenizer) parseError(data string, datavars ...string) {
	t.tokenQueue = append(t.tokenQueue, &token{typ: tokenParseError, data: data, datavars: datavars})
}

// characters is HTMLTokenizer.characters in tinyhtml5 tokenizer.py. It adds
// a characters string to the token queue.
func (t *htmlTokenizer) characters(data string) {
	t.tokenQueue = append(t.tokenQueue, &token{typ: tokenCharacters, data: data})
}

// consumeNumberEntity is HTMLTokenizer.consume_number_entity in tinyhtml5
// tokenizer.py. It returns either U+FFFD or the character based on the
// representation. It also discards ";" if present. If not present, it adds a
// parse error.
func (t *htmlTokenizer) consumeNumberEntity(isHex bool) string {
	allowed := digit
	radix := 10
	if isHex {
		allowed = hexDigit
		radix = 16
	}

	// Consume all the characters that are in range while making sure we
	// don't hit an EOF. The value stops above 0x10FFFF, where upstream
	// computes a larger integer.
	integer := 0
	character := t.stream.character()
	for character != eof && allowed(character) {
		v, _ := strconv.ParseInt(string(character), radix, 32)
		integer = min(integer*radix+int(v), 0x110000)
		character = t.stream.character()
	}

	var replacement rune
	// Certain characters get replaced with others.
	if r, ok := replacementCharacter(integer); ok {
		replacement = r
		t.parseError("illegal-codepoint-for-numeric-entity", "integer", strconv.Itoa(integer))
	} else if (0xD800 <= integer && integer <= 0xDFFF) || integer > 0x10FFFF {
		replacement = '�'
		t.parseError("illegal-codepoint-for-numeric-entity", "integer", strconv.Itoa(integer))
	} else {
		if (0x0001 <= integer && integer <= 0x0008) ||
			(0x000E <= integer && integer <= 0x001F) ||
			(0x007F <= integer && integer <= 0x009F) ||
			(0xFDD0 <= integer && integer <= 0xFDEF) ||
			integer == 0x000B || integer&0xFFFE == 0xFFFE {
			t.parseError("illegal-codepoint-for-numeric-entity", "integer", strconv.Itoa(integer))
		}
		replacement = rune(integer)
	}

	// Discard the ; if present. Otherwise, put it back on the queue and
	// invoke parse_error on parser.
	if character != ';' {
		t.parseError("numeric-entity-without-semicolon")
		t.stream.unget(character)
	}

	return string(replacement)
}

// consumeEntity is HTMLTokenizer.consume_entity in tinyhtml5 tokenizer.py.
// allowed is -1 where upstream gives None.
func (t *htmlTokenizer) consumeEntity(allowed rune, fromAttribute bool) {
	// Initialise to the default output for when no entity is matched.
	output := "&"

	stack := []rune{t.stream.character()}
	unget := stack[0] == eof || stack[0] == '<' || stack[0] == '&' ||
		spaceCharacter(stack[0]) || (allowed != -1 && allowed == stack[0])
	switch {
	case unget:
		t.stream.unget(stack[0])

	case stack[0] == '#':
		// Read the next character to see if it's hex or decimal.
		isHex := false
		stack = append(stack, t.stream.character())
		if stack[len(stack)-1] == 'x' || stack[len(stack)-1] == 'X' {
			isHex = true
			stack = append(stack, t.stream.character())
		}

		// stack[-1] should be the first digit.
		last := stack[len(stack)-1]
		if last != eof && (isHex && hexDigit(last) || !isHex && digit(last)) {
			// At least one digit found, so consume the whole number.
			t.stream.unget(last)
			output = t.consumeNumberEntity(isHex)
		} else {
			// No digits found.
			t.parseError("expected-numeric-entity")
			t.stream.unget(last)
			stack = stack[:len(stack)-1]
			output = "&" + string(stack)
		}

	default:
		// At this point in the process might have named entity. Entities
		// are stored in the global variable "entities". Consume characters
		// and compare to these to a substring of the entity names in the
		// list until the substring no longer matches.
		for stack[len(stack)-1] != eof {
			if !hasKeysWithPrefix(string(stack)) {
				break
			}
			stack = append(stack, t.stream.character())
		}

		// At this point we have a string that starts with some characters
		// that may match an entity.
		// Try to find the longest entity the string will match to take care
		// of &noti for instance.
		entityName, ok := longestPrefix(string(stack[:len(stack)-1]))
		if !ok {
			t.parseError("expected-named-entity")
			t.stream.unget(stack[len(stack)-1])
			stack = stack[:len(stack)-1]
			output = "&" + string(stack)
			break
		}
		if entityName[len(entityName)-1] != ';' {
			t.parseError("named-entity-without-semicolon")
		}
		entityLength := len(entityName)
		next := stack[entityLength]
		allowedCharacter := next != eof && (asciiLetter(next) || digit(next) || next == '=')
		t.stream.unget(stack[len(stack)-1])
		stack = stack[:len(stack)-1]
		if entityName[len(entityName)-1] != ';' && fromAttribute && allowedCharacter {
			output = "&" + string(stack)
		} else {
			value, _ := entityValue(entityName)
			output = value + string(stack[entityLength:])
		}
	}

	if fromAttribute {
		a := t.currentToken.lastAttr()
		a.value = append(a.value, output...)
	} else {
		typ := tokenCharacters
		if r, size := utf8.DecodeRuneInString(output); size == len(output) && spaceCharacter(r) {
			typ = tokenSpaceCharacters
		}
		t.tokenQueue = append(t.tokenQueue, &token{typ: typ, data: output})
	}
}

// processEntityInAttribute is HTMLTokenizer.process_entity_in_attribute in
// tinyhtml5 tokenizer.py. It replaces the need for
// entity_in_attribute_value_state.
func (t *htmlTokenizer) processEntityInAttribute(allowed rune) {
	t.consumeEntity(allowed, true)
}

// emitCurrentToken is HTMLTokenizer.emit_current_token in tinyhtml5
// tokenizer.py. It is a generic handler for emitting the tags. It also sets
// the state to "data" because that's what's needed after a token has been
// emitted.
func (t *htmlTokenizer) emitCurrentToken() {
	tok := t.currentToken
	// Add token to the queue to be yielded.
	if tok.typ == tokenStartTag || tok.typ == tokenEndTag || tok.typ == tokenEmptyTag {
		tok.name = asciiUpperToLower(string(tok.nameB))
		if tok.typ == tokenStartTag {
			// We had some duplicated attribute, fix so first wins.
			tok.attrs = make([]Attribute, 0, len(tok.rawAttrs))
			var seen map[string]bool
			if len(tok.rawAttrs) > smallAttrs {
				seen = make(map[string]bool, len(tok.rawAttrs))
			}
			for _, a := range tok.rawAttrs {
				name := string(a.name)
				if seen != nil {
					if seen[name] {
						continue
					}
					seen[name] = true
				} else if slices.ContainsFunc(tok.attrs, func(b Attribute) bool { return b.Name == name }) {
					continue
				}
				tok.attrs = append(tok.attrs, Attribute{Name: name, Value: string(a.value)})
			}
		}

		if tok.typ == tokenEndTag {
			if len(tok.rawAttrs) > 0 {
				t.parseError("attributes-in-end-tag")
			}
			if tok.selfClosing {
				t.parseError("self-closing-flag-on-end-tag")
			}
		}
		tok.rawAttrs = nil
		tok.attrSet = nil
	}
	t.tokenQueue = append(t.tokenQueue, tok)
	t.state = dataState
}

// emitComment adds the comment or the doctype that the tokenizer builds to
// the token queue.
func (t *htmlTokenizer) emitComment() {
	tok := t.currentToken
	if tok.typ == tokenComment {
		tok.data = string(tok.dataB)
	} else {
		tok.finishDoctype()
	}
	t.tokenQueue = append(t.tokenQueue, tok)
}

// smallAttrs is the number of attributes up to which a tag finds an
// attribute by a search of its list.
const smallAttrs = 16

// tokenizerState is a state of the tokenizer: a method of HTMLTokenizer in
// tinyhtml5 tokenizer.py, which upstream holds in self.state.
type tokenizerState uint8

// The states of the tokenizer.
const (
	dataState tokenizerState = iota
	entityDataState
	rcdataState
	characterReferenceInRcdataState
	rawtextState
	scriptDataState
	plaintextState
	tagOpenState
	closeTagOpenState
	tagNameState
	rcdataLessThanSignState
	rcdataEndTagOpenState
	rcdataEndTagNameState
	rawtextLessThanSignState
	rawtextEndTagOpenState
	rawtextEndTagNameState
	scriptDataLessThanSignState
	scriptDataEndTagOpenState
	scriptDataEndTagNameState
	scriptDataEscapeStartState
	scriptDataEscapeStartDashState
	scriptDataEscapedState
	scriptDataEscapedDashState
	scriptDataEscapedDashDashState
	scriptDataEscapedLessThanSignState
	scriptDataEscapedEndTagOpenState
	scriptDataEscapedEndTagNameState
	scriptDataDoubleEscapeStartState
	scriptDataDoubleEscapedState
	scriptDataDoubleEscapedDashState
	scriptDataDoubleEscapedDashDashState
	scriptDataDoubleEscapedLessThanSignState
	scriptDataDoubleEscapeEndState
	beforeAttributeNameState
	attributeNameState
	afterAttributeNameState
	beforeAttributeValueState
	attributeValueDoubleQuotedState
	attributeValueSingleQuotedState
	attributeValueUnquotedState
	afterAttributeValueState
	selfClosingStartTagState
	bogusCommentState
	markupDeclarationOpenState
	commentStartState
	commentStartDashState
	commentState
	commentEndDashState
	commentEndState
	commentEndBangState
	doctypeState
	beforeDoctypeNameState
	doctypeNameState
	afterDoctypeNameState
	afterDoctypePublicKeywordState
	beforeDoctypePublicIdentifierState
	doctypePublicIdentifierDoubleQuotedState
	doctypePublicIdentifierSingleQuotedState
	afterDoctypePublicIdentifierState
	betweenDoctypePublicAndSystemIdentifiersState
	afterDoctypeSystemKeywordState
	beforeDoctypeSystemIdentifierState
	doctypeSystemIdentifierDoubleQuotedState
	doctypeSystemIdentifierSingleQuotedState
	afterDoctypeSystemIdentifierState
	bogusDoctypeState
	cdataSectionState
)

// run calls the method of the current state, as self.state() does in
// HTMLTokenizer.__iter__ of tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) run() bool {
	switch t.state {
	case dataState:
		return t.dataState()
	case entityDataState:
		return t.entityDataState()
	case rcdataState:
		return t.rcdataState()
	case characterReferenceInRcdataState:
		return t.characterReferenceInRcdataState()
	case rawtextState:
		return t.rawtextState()
	case scriptDataState:
		return t.scriptDataState()
	case plaintextState:
		return t.plaintextState()
	case tagOpenState:
		return t.tagOpenState()
	case closeTagOpenState:
		return t.closeTagOpenState()
	case tagNameState:
		return t.tagNameState()
	case rcdataLessThanSignState:
		return t.rcdataLessThanSignState()
	case rcdataEndTagOpenState:
		return t.rcdataEndTagOpenState()
	case rcdataEndTagNameState:
		return t.rcdataEndTagNameState()
	case rawtextLessThanSignState:
		return t.rawtextLessThanSignState()
	case rawtextEndTagOpenState:
		return t.rawtextEndTagOpenState()
	case rawtextEndTagNameState:
		return t.rawtextEndTagNameState()
	case scriptDataLessThanSignState:
		return t.scriptDataLessThanSignState()
	case scriptDataEndTagOpenState:
		return t.scriptDataEndTagOpenState()
	case scriptDataEndTagNameState:
		return t.scriptDataEndTagNameState()
	case scriptDataEscapeStartState:
		return t.scriptDataEscapeStartState()
	case scriptDataEscapeStartDashState:
		return t.scriptDataEscapeStartDashState()
	case scriptDataEscapedState:
		return t.scriptDataEscapedState()
	case scriptDataEscapedDashState:
		return t.scriptDataEscapedDashState()
	case scriptDataEscapedDashDashState:
		return t.scriptDataEscapedDashDashState()
	case scriptDataEscapedLessThanSignState:
		return t.scriptDataEscapedLessThanSignState()
	case scriptDataEscapedEndTagOpenState:
		return t.scriptDataEscapedEndTagOpenState()
	case scriptDataEscapedEndTagNameState:
		return t.scriptDataEscapedEndTagNameState()
	case scriptDataDoubleEscapeStartState:
		return t.scriptDataDoubleEscapeStartState()
	case scriptDataDoubleEscapedState:
		return t.scriptDataDoubleEscapedState()
	case scriptDataDoubleEscapedDashState:
		return t.scriptDataDoubleEscapedDashState()
	case scriptDataDoubleEscapedDashDashState:
		return t.scriptDataDoubleEscapedDashDashState()
	case scriptDataDoubleEscapedLessThanSignState:
		return t.scriptDataDoubleEscapedLessThanSignState()
	case scriptDataDoubleEscapeEndState:
		return t.scriptDataDoubleEscapeEndState()
	case beforeAttributeNameState:
		return t.beforeAttributeNameState()
	case attributeNameState:
		return t.attributeNameState()
	case afterAttributeNameState:
		return t.afterAttributeNameState()
	case beforeAttributeValueState:
		return t.beforeAttributeValueState()
	case attributeValueDoubleQuotedState:
		return t.attributeValueDoubleQuotedState()
	case attributeValueSingleQuotedState:
		return t.attributeValueSingleQuotedState()
	case attributeValueUnquotedState:
		return t.attributeValueUnquotedState()
	case afterAttributeValueState:
		return t.afterAttributeValueState()
	case selfClosingStartTagState:
		return t.selfClosingStartTagState()
	case bogusCommentState:
		return t.bogusCommentState()
	case markupDeclarationOpenState:
		return t.markupDeclarationOpenState()
	case commentStartState:
		return t.commentStartState()
	case commentStartDashState:
		return t.commentStartDashState()
	case commentState:
		return t.commentState()
	case commentEndDashState:
		return t.commentEndDashState()
	case commentEndState:
		return t.commentEndState()
	case commentEndBangState:
		return t.commentEndBangState()
	case doctypeState:
		return t.doctypeState()
	case beforeDoctypeNameState:
		return t.beforeDoctypeNameState()
	case doctypeNameState:
		return t.doctypeNameState()
	case afterDoctypeNameState:
		return t.afterDoctypeNameState()
	case afterDoctypePublicKeywordState:
		return t.afterDoctypePublicKeywordState()
	case beforeDoctypePublicIdentifierState:
		return t.beforeDoctypePublicIdentifierState()
	case doctypePublicIdentifierDoubleQuotedState:
		return t.doctypePublicIdentifierDoubleQuotedState()
	case doctypePublicIdentifierSingleQuotedState:
		return t.doctypePublicIdentifierSingleQuotedState()
	case afterDoctypePublicIdentifierState:
		return t.afterDoctypePublicIdentifierState()
	case betweenDoctypePublicAndSystemIdentifiersState:
		return t.betweenDoctypePublicAndSystemIdentifiersState()
	case afterDoctypeSystemKeywordState:
		return t.afterDoctypeSystemKeywordState()
	case beforeDoctypeSystemIdentifierState:
		return t.beforeDoctypeSystemIdentifierState()
	case doctypeSystemIdentifierDoubleQuotedState:
		return t.doctypeSystemIdentifierDoubleQuotedState()
	case doctypeSystemIdentifierSingleQuotedState:
		return t.doctypeSystemIdentifierSingleQuotedState()
	case afterDoctypeSystemIdentifierState:
		return t.afterDoctypeSystemIdentifierState()
	case bogusDoctypeState:
		return t.bogusDoctypeState()
	case cdataSectionState:
		return t.cdataSectionState()
	}
	return false
}

// Below are the various tokenizer states worked out.

// dataState is HTMLTokenizer.data_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) dataState() bool {
	data := t.stream.character()
	switch {
	case data == '&':
		t.state = entityDataState
	case data == '<':
		t.state = tagOpenState
	case data == '\u0000':
		t.parseError("invalid-codepoint")
		t.characters("\u0000")
	case data == eof:
		return false
	case spaceCharacter(data):
		// Directly after emitting a token you switch back to the "data
		// state". At that point space characters are important so they are
		// emitted separately.
		t.tokenQueue = append(t.tokenQueue, &token{
			typ:  tokenSpaceCharacters,
			data: string(data) + t.stream.charsUntil(spaceCharacters, true),
		})
		// No need to update lastFourChars here, since the first space will
		// have already been appended to lastFourChars and will have broken
		// any <!-- or --> sequences.
	default:
		characters := t.stream.charsUntil("&<\u0000", false)
		t.characters(string(data) + characters)
	}
	return true
}

// entityDataState is HTMLTokenizer.entity_data_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) entityDataState() bool {
	t.consumeEntity(-1, false)
	t.state = dataState
	return true
}

// rcdataState is HTMLTokenizer.rcdata_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) rcdataState() bool {
	data := t.stream.character()
	switch {
	case data == '&':
		t.state = characterReferenceInRcdataState
	case data == '<':
		t.state = rcdataLessThanSignState
	case data == eof:
		// Tokenization ends.
		return false
	case data == '\u0000':
		t.parseError("invalid-codepoint")
		t.characters("�")
	case spaceCharacter(data):
		// Directly after emitting a token you switch back to the "data
		// state". At that point space_characters are important so they are
		// emitted separately.
		t.tokenQueue = append(t.tokenQueue, &token{
			typ:  tokenSpaceCharacters,
			data: string(data) + t.stream.charsUntil(spaceCharacters, true),
		})
		// No need to update lastFourChars here, since the first space will
		// have already been appended to lastFourChars and will have broken
		// any <!-- or --> sequences.
	default:
		chars := t.stream.charsUntil("&<\u0000", false)
		t.characters(string(data) + chars)
	}
	return true
}

// characterReferenceInRcdataState is
// HTMLTokenizer.character_reference_in_rc_data_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) characterReferenceInRcdataState() bool {
	t.consumeEntity(-1, false)
	t.state = rcdataState
	return true
}

// rawtextState is HTMLTokenizer.rawtext_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) rawtextState() bool {
	data := t.stream.character()
	switch data {
	case '<':
		t.state = rawtextLessThanSignState
	case '\u0000':
		t.parseError("invalid-codepoint")
		t.characters("�")
	case eof:
		return false
	default:
		characters := t.stream.charsUntil("<\u0000", false)
		t.characters(string(data) + characters)
	}
	return true
}

// scriptDataState is HTMLTokenizer.script_data_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) scriptDataState() bool {
	data := t.stream.character()
	switch data {
	case '<':
		t.state = scriptDataLessThanSignState
	case '\u0000':
		t.parseError("invalid-codepoint")
		t.characters("�")
	case eof:
		return false
	default:
		characters := t.stream.charsUntil("<\u0000", false)
		t.characters(string(data) + characters)
	}
	return true
}

// plaintextState is HTMLTokenizer.plaintext_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) plaintextState() bool {
	data := t.stream.character()
	switch data {
	case eof:
		return false
	case '\u0000':
		t.parseError("invalid-codepoint")
		t.characters("�")
	default:
		t.characters(string(data) + t.stream.charsUntil("\u0000", false))
	}
	return true
}

// tagOpenState is HTMLTokenizer.tag_open_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) tagOpenState() bool {
	data := t.stream.character()
	switch {
	case data == '!':
		t.state = markupDeclarationOpenState
	case data == '/':
		t.state = closeTagOpenState
	case asciiLetter(data):
		t.currentToken = &token{typ: tokenStartTag, nameB: utf8.AppendRune(nil, data)}
		t.state = tagNameState
	case data == '>':
		// XXX In theory it could be something besides a tag name. But
		// do we really care?
		t.parseError("expected-tag-name-but-got-right-bracket")
		t.characters("<>")
		t.state = dataState
	case data == '?':
		// XXX In theory it could be something besides a tag name. But
		// do we really care?
		t.parseError("expected-tag-name-but-got-question-mark")
		t.stream.unget(data)
		t.state = bogusCommentState
	default:
		// XXX
		t.parseError("expected-tag-name")
		t.characters("<")
		t.stream.unget(data)
		t.state = dataState
	}
	return true
}

// closeTagOpenState is HTMLTokenizer.close_tag_open_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) closeTagOpenState() bool {
	data := t.stream.character()
	switch {
	case asciiLetter(data):
		t.currentToken = &token{typ: tokenEndTag, nameB: utf8.AppendRune(nil, data)}
		t.state = tagNameState
	case data == '>':
		t.parseError("expected-closing-tag-but-got-right-bracket")
		t.state = dataState
	case data == eof:
		t.parseError("expected-closing-tag-but-got-eof")
		t.characters("</")
		t.state = dataState
	default:
		// XXX data can be _'_...
		t.parseError("expected-closing-tag-but-got-char", "data", string(data))
		t.stream.unget(data)
		t.state = bogusCommentState
	}
	return true
}

// tagNameState is HTMLTokenizer.tag_name_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) tagNameState() bool {
	data := t.stream.character()
	switch {
	case spaceCharacter(data):
		t.state = beforeAttributeNameState
	case data == '>':
		t.emitCurrentToken()
	case data == eof:
		t.parseError("eof-in-tag-name")
		t.state = dataState
	case data == '/':
		t.state = selfClosingStartTagState
	case data == '\u0000':
		t.parseError("invalid-codepoint")
		t.currentToken.nameB = utf8.AppendRune(t.currentToken.nameB, '�')
	default:
		t.currentToken.nameB = utf8.AppendRune(t.currentToken.nameB, data)
		// (Don't use chars_until here, because tag names are
		// very short and it's faster to not do anything fancy.)
	}
	return true
}

// appropriate is the test for an appropriate end tag of the states for the
// names of end tags in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) appropriate() bool {
	return t.currentToken != nil &&
		strings.EqualFold(t.currentToken.name, string(t.temporaryBuffer))
}

// endTagToken returns the end tag that the states for the names of end tags
// in tinyhtml5 tokenizer.py make from the temporary buffer.
func (t *htmlTokenizer) endTagToken() *token {
	return &token{typ: tokenEndTag, nameB: slices.Clone(t.temporaryBuffer)}
}

// endTagName is the body of the states for the names of end tags in
// tinyhtml5 tokenizer.py, with the state that each one goes back to.
func (t *htmlTokenizer) endTagName(back tokenizerState) bool {
	appropriate := t.appropriate()
	data := t.stream.character()
	switch {
	case spaceCharacter(data) && appropriate:
		t.currentToken = t.endTagToken()
		t.state = beforeAttributeNameState
	case data == '/' && appropriate:
		t.currentToken = t.endTagToken()
		t.state = selfClosingStartTagState
	case data == '>' && appropriate:
		t.currentToken = t.endTagToken()
		t.emitCurrentToken()
		t.state = dataState
	case asciiLetter(data):
		t.temporaryBuffer = utf8.AppendRune(t.temporaryBuffer, data)
	default:
		t.characters("</" + string(t.temporaryBuffer))
		t.stream.unget(data)
		t.state = back
	}
	return true
}

// lessThanSign is the body of rcdata_less_than_sign_state and
// rawtext_less_than_sign_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) lessThanSign(open, back tokenizerState) bool {
	data := t.stream.character()
	if data == '/' {
		t.temporaryBuffer = t.temporaryBuffer[:0]
		t.state = open
	} else {
		t.characters("<")
		t.stream.unget(data)
		t.state = back
	}
	return true
}

// endTagOpen is the body of rcdata_end_tag_open_state,
// rawtext_end_tag_open_state and script_data_end_tag_open_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) endTagOpen(name, back tokenizerState) bool {
	data := t.stream.character()
	if asciiLetter(data) {
		t.temporaryBuffer = utf8.AppendRune(t.temporaryBuffer, data)
		t.state = name
	} else {
		t.characters("</")
		t.stream.unget(data)
		t.state = back
	}
	return true
}

// rcdataLessThanSignState is HTMLTokenizer.rcdata_less_than_sign_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) rcdataLessThanSignState() bool {
	return t.lessThanSign(rcdataEndTagOpenState, rcdataState)
}

// rcdataEndTagOpenState is HTMLTokenizer.rcdata_end_tag_open_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) rcdataEndTagOpenState() bool {
	return t.endTagOpen(rcdataEndTagNameState, rcdataState)
}

// rcdataEndTagNameState is HTMLTokenizer.rcdata_end_tag_name_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) rcdataEndTagNameState() bool {
	return t.endTagName(rcdataState)
}

// rawtextLessThanSignState is HTMLTokenizer.rawtext_less_than_sign_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) rawtextLessThanSignState() bool {
	return t.lessThanSign(rawtextEndTagOpenState, rawtextState)
}

// rawtextEndTagOpenState is HTMLTokenizer.rawtext_end_tag_open_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) rawtextEndTagOpenState() bool {
	return t.endTagOpen(rawtextEndTagNameState, rawtextState)
}

// rawtextEndTagNameState is HTMLTokenizer.rawtext_end_tag_name_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) rawtextEndTagNameState() bool {
	return t.endTagName(rawtextState)
}

// scriptDataLessThanSignState is
// HTMLTokenizer.script_data_less_than_sign_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) scriptDataLessThanSignState() bool {
	data := t.stream.character()
	switch data {
	case '/':
		t.temporaryBuffer = t.temporaryBuffer[:0]
		t.state = scriptDataEndTagOpenState
	case '!':
		t.characters("<!")
		t.state = scriptDataEscapeStartState
	default:
		t.characters("<")
		t.stream.unget(data)
		t.state = scriptDataState
	}
	return true
}

// scriptDataEndTagOpenState is HTMLTokenizer.script_data_end_tag_open_state
// in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) scriptDataEndTagOpenState() bool {
	return t.endTagOpen(scriptDataEndTagNameState, scriptDataState)
}

// scriptDataEndTagNameState is HTMLTokenizer.script_data_end_tag_name_state
// in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) scriptDataEndTagNameState() bool {
	return t.endTagName(scriptDataState)
}

// scriptDataEscapeStartState is
// HTMLTokenizer.script_data_escape_start_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) scriptDataEscapeStartState() bool {
	data := t.stream.character()
	if data == '-' {
		t.characters("-")
		t.state = scriptDataEscapeStartDashState
	} else {
		t.stream.unget(data)
		t.state = scriptDataState
	}
	return true
}

// scriptDataEscapeStartDashState is
// HTMLTokenizer.script_data_escape_start_dash_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) scriptDataEscapeStartDashState() bool {
	data := t.stream.character()
	if data == '-' {
		t.characters("-")
		t.state = scriptDataEscapedDashDashState
	} else {
		t.stream.unget(data)
		t.state = scriptDataState
	}
	return true
}

// scriptDataEscapedState is HTMLTokenizer.script_data_escaped_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) scriptDataEscapedState() bool {
	data := t.stream.character()
	switch data {
	case '-':
		t.characters("-")
		t.state = scriptDataEscapedDashState
	case '<':
		t.state = scriptDataEscapedLessThanSignState
	case '\u0000':
		t.parseError("invalid-codepoint")
		t.characters("�")
	case eof:
		t.state = dataState
	default:
		t.characters(string(data) + t.stream.charsUntil("<-\u0000", false))
	}
	return true
}

// scriptDataEscapedDashState is HTMLTokenizer.script_data_escaped_dash_state
// in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) scriptDataEscapedDashState() bool {
	data := t.stream.character()
	switch data {
	case '-':
		t.characters("-")
		t.state = scriptDataEscapedDashDashState
	case '<':
		t.state = scriptDataEscapedLessThanSignState
	case '\u0000':
		t.parseError("invalid-codepoint")
		t.characters("�")
		t.state = scriptDataEscapedState
	case eof:
		t.state = dataState
	default:
		t.characters(string(data))
		t.state = scriptDataEscapedState
	}
	return true
}

// scriptDataEscapedDashDashState is
// HTMLTokenizer.script_data_escaped_dash_dash_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) scriptDataEscapedDashDashState() bool {
	data := t.stream.character()
	switch data {
	case '-':
		t.characters("-")
	case '<':
		t.state = scriptDataEscapedLessThanSignState
	case '>':
		t.characters(">")
		t.state = scriptDataState
	case '\u0000':
		t.parseError("invalid-codepoint")
		t.characters("�")
		t.state = scriptDataEscapedState
	case eof:
		t.state = dataState
	default:
		t.characters(string(data))
		t.state = scriptDataEscapedState
	}
	return true
}

// scriptDataEscapedLessThanSignState is
// HTMLTokenizer.script_data_escaped_less_than_sign_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) scriptDataEscapedLessThanSignState() bool {
	data := t.stream.character()
	switch {
	case data == '/':
		t.temporaryBuffer = t.temporaryBuffer[:0]
		t.state = scriptDataEscapedEndTagOpenState
	case asciiLetter(data):
		t.characters("<" + string(data))
		t.temporaryBuffer = utf8.AppendRune(t.temporaryBuffer[:0], data)
		t.state = scriptDataDoubleEscapeStartState
	default:
		t.characters("<")
		t.stream.unget(data)
		t.state = scriptDataEscapedState
	}
	return true
}

// scriptDataEscapedEndTagOpenState is
// HTMLTokenizer.script_data_escaped_end_tag_open_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) scriptDataEscapedEndTagOpenState() bool {
	data := t.stream.character()
	if asciiLetter(data) {
		t.temporaryBuffer = utf8.AppendRune(t.temporaryBuffer[:0], data)
		t.state = scriptDataEscapedEndTagNameState
	} else {
		t.characters("</")
		t.stream.unget(data)
		t.state = scriptDataEscapedState
	}
	return true
}

// scriptDataEscapedEndTagNameState is
// HTMLTokenizer.script_data_escaped_end_tag_name_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) scriptDataEscapedEndTagNameState() bool {
	return t.endTagName(scriptDataEscapedState)
}

// doubleEscape is the body of script_data_double_escape_start_state and
// script_data_double_escape_end_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) doubleEscape(script, other tokenizerState) bool {
	data := t.stream.character()
	switch {
	case spaceCharacter(data) || data == '/' || data == '>':
		t.characters(string(data))
		if strings.EqualFold(string(t.temporaryBuffer), "script") {
			t.state = script
		} else {
			t.state = other
		}
	case asciiLetter(data):
		t.characters(string(data))
		t.temporaryBuffer = utf8.AppendRune(t.temporaryBuffer, data)
	default:
		t.stream.unget(data)
		t.state = other
	}
	return true
}

// scriptDataDoubleEscapeStartState is
// HTMLTokenizer.script_data_double_escape_start_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) scriptDataDoubleEscapeStartState() bool {
	return t.doubleEscape(scriptDataDoubleEscapedState, scriptDataEscapedState)
}

// scriptDataDoubleEscapedState is
// HTMLTokenizer.script_data_double_escaped_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) scriptDataDoubleEscapedState() bool {
	data := t.stream.character()
	switch data {
	case '-':
		t.characters("-")
		t.state = scriptDataDoubleEscapedDashState
	case '<':
		t.characters("<")
		t.state = scriptDataDoubleEscapedLessThanSignState
	case '\u0000':
		t.parseError("invalid-codepoint")
		t.characters("�")
	case eof:
		t.parseError("eof-in-script-in-script")
		t.state = dataState
	default:
		t.characters(string(data))
	}
	return true
}

// scriptDataDoubleEscapedDashState is
// HTMLTokenizer.script_data_double_escaped_dash_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) scriptDataDoubleEscapedDashState() bool {
	data := t.stream.character()
	switch data {
	case '-':
		t.characters("-")
		t.state = scriptDataDoubleEscapedDashDashState
	case '<':
		t.characters("<")
		t.state = scriptDataDoubleEscapedLessThanSignState
	case '\u0000':
		t.parseError("invalid-codepoint")
		t.characters("�")
		t.state = scriptDataDoubleEscapedState
	case eof:
		t.parseError("eof-in-script-in-script")
		t.state = dataState
	default:
		t.characters(string(data))
		t.state = scriptDataDoubleEscapedState
	}
	return true
}

// scriptDataDoubleEscapedDashDashState is
// HTMLTokenizer.script_data_double_escaped_dash_dash_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) scriptDataDoubleEscapedDashDashState() bool {
	data := t.stream.character()
	switch data {
	case '-':
		t.characters("-")
	case '<':
		t.characters("<")
		t.state = scriptDataDoubleEscapedLessThanSignState
	case '>':
		t.characters(">")
		t.state = scriptDataState
	case '\u0000':
		t.parseError("invalid-codepoint")
		t.characters("�")
		t.state = scriptDataDoubleEscapedState
	case eof:
		t.parseError("eof-in-script-in-script")
		t.state = dataState
	default:
		t.characters(string(data))
		t.state = scriptDataDoubleEscapedState
	}
	return true
}

// scriptDataDoubleEscapedLessThanSignState is
// HTMLTokenizer.script_data_double_escaped_less_than_sign_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) scriptDataDoubleEscapedLessThanSignState() bool {
	data := t.stream.character()
	if data == '/' {
		t.characters("/")
		t.temporaryBuffer = t.temporaryBuffer[:0]
		t.state = scriptDataDoubleEscapeEndState
	} else {
		t.stream.unget(data)
		t.state = scriptDataDoubleEscapedState
	}
	return true
}

// scriptDataDoubleEscapeEndState is
// HTMLTokenizer.script_data_double_escape_end_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) scriptDataDoubleEscapeEndState() bool {
	return t.doubleEscape(scriptDataEscapedState, scriptDataDoubleEscapedState)
}

// newAttribute starts a new attribute of the current tag, with the first
// character of its name.
func (t *htmlTokenizer) newAttribute(c rune) {
	t.currentToken.rawAttrs = append(t.currentToken.rawAttrs, rawAttr{name: utf8.AppendRune(nil, c)})
}

// beforeAttributeNameState is HTMLTokenizer.before_attribute_name_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) beforeAttributeNameState() bool {
	data := t.stream.character()
	switch {
	case spaceCharacter(data):
		t.stream.charsUntil(spaceCharacters, true)
	case asciiLetter(data):
		t.newAttribute(data)
		t.state = attributeNameState
	case data == '>':
		t.emitCurrentToken()
	case data == '/':
		t.state = selfClosingStartTagState
	case data == '\'' || data == '"' || data == '=' || data == '<':
		t.parseError("invalid-character-in-attribute-name")
		t.newAttribute(data)
		t.state = attributeNameState
	case data == '\u0000':
		t.parseError("invalid-codepoint")
		t.newAttribute('�')
		t.state = attributeNameState
	case data == eof:
		t.parseError("expected-attribute-name-but-got-eof")
		t.state = dataState
	default:
		t.newAttribute(data)
		t.state = attributeNameState
	}
	return true
}

// attributeNameState is HTMLTokenizer.attribute_name_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) attributeNameState() bool {
	data := t.stream.character()
	leavingThisState := true
	emitToken := false
	a := t.currentToken.lastAttr()
	switch {
	case data == '=':
		t.state = beforeAttributeValueState
	case asciiLetter(data):
		a.name = utf8.AppendRune(a.name, data)
		a.name = append(a.name, t.stream.charsUntil(asciiLetters, true)...)
		leavingThisState = false
	case data == '>':
		// XXX If we emit here the attributes are converted to a dict
		// without being checked and when the code below runs we error
		// because data is a dict not a list.
		emitToken = true
	case spaceCharacter(data):
		t.state = afterAttributeNameState
	case data == '/':
		t.state = selfClosingStartTagState
	case data == '\u0000':
		t.parseError("invalid-codepoint")
		a.name = utf8.AppendRune(a.name, '�')
		leavingThisState = false
	case data == '\'' || data == '"' || data == '<':
		t.parseError("invalid-character-in-attribute-name")
		a.name = utf8.AppendRune(a.name, data)
		leavingThisState = false
	case data == eof:
		t.parseError("eof-in-attribute-name")
		t.state = dataState
	default:
		a.name = utf8.AppendRune(a.name, data)
		leavingThisState = false
	}

	if leavingThisState {
		// Attributes are not dropped at this stage. That happens when the
		// start tag token is emitted so values can still be safely appended
		// to attributes, but we do want to report the parse error in time.
		for i, c := range a.name {
			if 'A' <= c && c <= 'Z' {
				a.name[i] = c + 'a' - 'A'
			}
		}
		if t.duplicateAttribute() {
			t.parseError("duplicate-attribute")
		}
		// XXX Fix for above.
		if emitToken {
			t.emitCurrentToken()
		}
	}
	return true
}

// duplicateAttribute says if the name of the last attribute of the current
// tag is the name of an attribute before it. A tag with many attributes keeps
// their names in a set, so that the test takes constant time.
func (t *htmlTokenizer) duplicateAttribute() bool {
	tok := t.currentToken
	attrs := tok.rawAttrs
	name := attrs[len(attrs)-1].name
	if len(attrs) <= smallAttrs {
		for _, a := range attrs[:len(attrs)-1] {
			if string(a.name) == string(name) {
				return true
			}
		}
		return false
	}
	if tok.attrSet == nil {
		tok.attrSet = make(map[string]bool, len(attrs))
		for _, a := range attrs[:len(attrs)-1] {
			tok.attrSet[string(a.name)] = true
		}
	}
	if tok.attrSet[string(name)] {
		return true
	}
	tok.attrSet[string(name)] = true
	return false
}

// asciiLetters is ascii_letters in tinyhtml5 constants.py, as a string for
// charsUntil.
const asciiLetters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// afterAttributeNameState is HTMLTokenizer.after_attribute_name_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) afterAttributeNameState() bool {
	data := t.stream.character()
	switch {
	case spaceCharacter(data):
		t.stream.charsUntil(spaceCharacters, true)
	case data == '=':
		t.state = beforeAttributeValueState
	case data == '>':
		t.emitCurrentToken()
	case asciiLetter(data):
		t.newAttribute(data)
		t.state = attributeNameState
	case data == '/':
		t.state = selfClosingStartTagState
	case data == '\u0000':
		t.parseError("invalid-codepoint")
		t.newAttribute('�')
		t.state = attributeNameState
	case data == '\'' || data == '"' || data == '<':
		t.parseError("invalid-character-after-attribute-name")
		t.newAttribute(data)
		t.state = attributeNameState
	case data == eof:
		t.parseError("expected-end-of-tag-but-got-eof")
		t.state = dataState
	default:
		t.newAttribute(data)
		t.state = attributeNameState
	}
	return true
}

// beforeAttributeValueState is HTMLTokenizer.before_attribute_value_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) beforeAttributeValueState() bool {
	data := t.stream.character()
	switch {
	case spaceCharacter(data):
		t.stream.charsUntil(spaceCharacters, true)
	case data == '"':
		t.state = attributeValueDoubleQuotedState
	case data == '&':
		t.state = attributeValueUnquotedState
		t.stream.unget(data)
	case data == '\'':
		t.state = attributeValueSingleQuotedState
	case data == '>':
		t.parseError("expected-attribute-value-but-got-right-bracket")
		t.emitCurrentToken()
	case data == '\u0000':
		t.parseError("invalid-codepoint")
		a := t.currentToken.lastAttr()
		a.value = utf8.AppendRune(a.value, '�')
		t.state = attributeValueUnquotedState
	case data == '=' || data == '<' || data == '`':
		t.parseError("equals-in-unquoted-attribute-value")
		a := t.currentToken.lastAttr()
		a.value = utf8.AppendRune(a.value, data)
		t.state = attributeValueUnquotedState
	case data == eof:
		t.parseError("expected-attribute-value-but-got-eof")
		t.state = dataState
	default:
		a := t.currentToken.lastAttr()
		a.value = utf8.AppendRune(a.value, data)
		t.state = attributeValueUnquotedState
	}
	return true
}

// attributeValueQuoted is the body of attribute_value_double_quoted_state
// and attribute_value_single_quoted_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) attributeValueQuoted(quote rune, until, errEOF string) bool {
	data := t.stream.character()
	switch data {
	case quote:
		t.state = afterAttributeValueState
	case '&':
		t.processEntityInAttribute(quote)
	case '\u0000':
		t.parseError("invalid-codepoint")
		a := t.currentToken.lastAttr()
		a.value = utf8.AppendRune(a.value, '�')
	case eof:
		t.parseError(errEOF)
		t.state = dataState
	default:
		a := t.currentToken.lastAttr()
		a.value = utf8.AppendRune(a.value, data)
		a.value = append(a.value, t.stream.charsUntil(until, false)...)
	}
	return true
}

// attributeValueDoubleQuotedState is
// HTMLTokenizer.attribute_value_double_quoted_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) attributeValueDoubleQuotedState() bool {
	return t.attributeValueQuoted('"', "\"&\u0000", "eof-in-attribute-value-double-quote")
}

// attributeValueSingleQuotedState is
// HTMLTokenizer.attribute_value_single_quoted_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) attributeValueSingleQuotedState() bool {
	return t.attributeValueQuoted('\'', "'&\u0000", "eof-in-attribute-value-single-quote")
}

// attributeValueUnquotedState is
// HTMLTokenizer.attribute_value_unquoted_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) attributeValueUnquotedState() bool {
	data := t.stream.character()
	switch {
	case spaceCharacter(data):
		t.state = beforeAttributeNameState
	case data == '&':
		t.processEntityInAttribute('>')
	case data == '>':
		t.emitCurrentToken()
	case data == '"' || data == '\'' || data == '=' || data == '<' || data == '`':
		t.parseError("unexpected-character-in-unquoted-attribute-value")
		a := t.currentToken.lastAttr()
		a.value = utf8.AppendRune(a.value, data)
	case data == '\u0000':
		t.parseError("invalid-codepoint")
		a := t.currentToken.lastAttr()
		a.value = utf8.AppendRune(a.value, '�')
	case data == eof:
		t.parseError("eof-in-attribute-value-no-quotes")
		t.state = dataState
	default:
		a := t.currentToken.lastAttr()
		a.value = utf8.AppendRune(a.value, data)
		a.value = append(a.value, t.stream.charsUntil("&>\"'=<`\u0000"+spaceCharacters, false)...)
	}
	return true
}

// afterAttributeValueState is HTMLTokenizer.after_attribute_value_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) afterAttributeValueState() bool {
	data := t.stream.character()
	switch {
	case spaceCharacter(data):
		t.state = beforeAttributeNameState
	case data == '>':
		t.emitCurrentToken()
	case data == '/':
		t.state = selfClosingStartTagState
	case data == eof:
		t.parseError("unexpected-eof-after-attribute-value")
		t.stream.unget(data)
		t.state = dataState
	default:
		t.parseError("unexpected-character-after-attribute-value")
		t.stream.unget(data)
		t.state = beforeAttributeNameState
	}
	return true
}

// selfClosingStartTagState is HTMLTokenizer.self_closing_start_tag_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) selfClosingStartTagState() bool {
	data := t.stream.character()
	switch data {
	case '>':
		t.currentToken.selfClosing = true
		t.emitCurrentToken()
	case eof:
		t.parseError("unexpected-eof-after-solidus-in-tag")
		t.stream.unget(data)
		t.state = dataState
	default:
		t.parseError("unexpected-character-after-solidus-in-tag")
		t.stream.unget(data)
		t.state = beforeAttributeNameState
	}
	return true
}

// bogusCommentState is HTMLTokenizer.bogus_comment_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) bogusCommentState() bool {
	// Make a new comment token and give it as value all the characters
	// until the first > or EOF (chars_until checks for EOF automatically)
	// and emit it.
	data := t.stream.charsUntil(">", false)
	data = strings.ReplaceAll(data, "\u0000", "�")
	t.tokenQueue = append(t.tokenQueue, &token{typ: tokenComment, data: data})

	// Eat the character directly after the bogus comment which is either a
	// ">" or an EOF.
	t.stream.character()
	t.state = dataState
	return true
}

// markupDeclarationOpenState is HTMLTokenizer.markup_declaration_open_state
// in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) markupDeclarationOpenState() bool {
	stack := []rune{t.stream.character()}
	last := func() rune { return stack[len(stack)-1] }
	switch {
	case last() == '-':
		stack = append(stack, t.stream.character())
		if last() == '-' {
			t.currentToken = &token{typ: tokenComment}
			t.state = commentStartState
			return true
		}
	case last() == 'd' || last() == 'D':
		matched := true
		for _, expected := range [...]string{"oO", "cC", "tT", "yY", "pP", "eE"} {
			stack = append(stack, t.stream.character())
			if last() == eof || !strings.ContainsRune(expected, last()) {
				matched = false
				break
			}
		}
		if matched {
			t.currentToken = &token{typ: tokenDoctype, correct: true}
			t.state = doctypeState
			return true
		}
	case last() == '[' && t.parser != nil && len(t.parser.tree.openElements) > 0 &&
		t.parser.tree.openElements[len(t.parser.tree.openElements)-1].namespace != t.parser.tree.defaultNamespace:
		matched := true
		for _, expected := range "CDATA[" {
			stack = append(stack, t.stream.character())
			if last() != expected {
				matched = false
				break
			}
		}
		if matched {
			t.state = cdataSectionState
			return true
		}
	}

	t.parseError("expected-dashes-or-doctype")

	for len(stack) > 0 {
		t.stream.unget(last())
		stack = stack[:len(stack)-1]
	}
	t.state = bogusCommentState
	return true
}

// commentStartState is HTMLTokenizer.comment_start_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) commentStartState() bool {
	data := t.stream.character()
	tok := t.currentToken
	switch data {
	case '-':
		t.state = commentStartDashState
	case '\u0000':
		t.parseError("invalid-codepoint")
		tok.dataB = utf8.AppendRune(tok.dataB, '�')
	case '>':
		t.parseError("incorrect-comment")
		t.emitComment()
		t.state = dataState
	case eof:
		t.parseError("eof-in-comment")
		t.emitComment()
		t.state = dataState
	default:
		tok.dataB = utf8.AppendRune(tok.dataB, data)
		t.state = commentState
	}
	return true
}

// commentStartDashState is HTMLTokenizer.comment_start_dash_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) commentStartDashState() bool {
	data := t.stream.character()
	tok := t.currentToken
	switch data {
	case '-':
		t.state = commentEndState
	case '\u0000':
		t.parseError("invalid-codepoint")
		tok.dataB = append(tok.dataB, "-�"...)
	case '>':
		t.parseError("incorrect-comment")
		t.emitComment()
		t.state = dataState
	case eof:
		t.parseError("eof-in-comment")
		t.emitComment()
		t.state = dataState
	default:
		tok.dataB = utf8.AppendRune(append(tok.dataB, '-'), data)
		t.state = commentState
	}
	return true
}

// commentState is HTMLTokenizer.comment_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) commentState() bool {
	data := t.stream.character()
	tok := t.currentToken
	switch data {
	case '-':
		t.state = commentEndDashState
	case '\u0000':
		t.parseError("invalid-codepoint")
		tok.dataB = utf8.AppendRune(tok.dataB, '�')
	case eof:
		t.parseError("eof-in-comment")
		t.emitComment()
		t.state = dataState
	default:
		tok.dataB = utf8.AppendRune(tok.dataB, data)
		tok.dataB = append(tok.dataB, t.stream.charsUntil("-\u0000", false)...)
	}
	return true
}

// commentEndDashState is HTMLTokenizer.comment_end_dash_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) commentEndDashState() bool {
	data := t.stream.character()
	tok := t.currentToken
	switch data {
	case '-':
		t.state = commentEndState
	case '\u0000':
		t.parseError("invalid-codepoint")
		tok.dataB = append(tok.dataB, "-�"...)
		t.state = commentState
	case eof:
		t.parseError("eof-in-comment-end-dash")
		t.emitComment()
		t.state = dataState
	default:
		tok.dataB = utf8.AppendRune(append(tok.dataB, '-'), data)
		t.state = commentState
	}
	return true
}

// commentEndState is HTMLTokenizer.comment_end_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) commentEndState() bool {
	data := t.stream.character()
	tok := t.currentToken
	switch data {
	case '>':
		t.emitComment()
		t.state = dataState
	case '\u0000':
		t.parseError("invalid-codepoint")
		tok.dataB = append(tok.dataB, "--�"...)
		t.state = commentState
	case '!':
		t.parseError("unexpected-bang-after-double-dash-in-comment")
		t.state = commentEndBangState
	case '-':
		t.parseError("unexpected-dash-after-double-dash-in-comment")
		tok.dataB = append(tok.dataB, '-')
	case eof:
		t.parseError("eof-in-comment-double-dash")
		t.emitComment()
		t.state = dataState
	default:
		// XXX
		t.parseError("unexpected-char-in-comment")
		tok.dataB = utf8.AppendRune(append(tok.dataB, "--"...), data)
		t.state = commentState
	}
	return true
}

// commentEndBangState is HTMLTokenizer.comment_end_bang_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) commentEndBangState() bool {
	data := t.stream.character()
	tok := t.currentToken
	switch data {
	case '>':
		t.emitComment()
		t.state = dataState
	case '-':
		tok.dataB = append(tok.dataB, "--!"...)
		t.state = commentEndDashState
	case '\u0000':
		t.parseError("invalid-codepoint")
		tok.dataB = append(tok.dataB, "--!�"...)
		t.state = commentState
	case eof:
		t.parseError("eof-in-comment-end-bang-state")
		t.emitComment()
		t.state = dataState
	default:
		tok.dataB = utf8.AppendRune(append(tok.dataB, "--!"...), data)
		t.state = commentState
	}
	return true
}

// doctypeState is HTMLTokenizer.doctype_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) doctypeState() bool {
	data := t.stream.character()
	switch {
	case spaceCharacter(data):
		t.state = beforeDoctypeNameState
	case data == eof:
		t.parseError("expected-doctype-name-but-got-eof")
		t.currentToken.correct = false
		t.emitComment()
		t.state = dataState
	default:
		t.parseError("need-space-after-doctype")
		t.stream.unget(data)
		t.state = beforeDoctypeNameState
	}
	return true
}

// beforeDoctypeNameState is HTMLTokenizer.before_doctype_name_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) beforeDoctypeNameState() bool {
	data := t.stream.character()
	tok := t.currentToken
	switch {
	case spaceCharacter(data):
	case data == '>':
		t.parseError("expected-doctype-name-but-got-right-bracket")
		tok.correct = false
		t.emitComment()
		t.state = dataState
	case data == '\u0000':
		t.parseError("invalid-codepoint")
		tok.nameB = utf8.AppendRune(tok.nameB[:0], '�')
		t.state = doctypeNameState
	case data == eof:
		t.parseError("expected-doctype-name-but-got-eof")
		tok.correct = false
		t.emitComment()
		t.state = dataState
	default:
		tok.nameB = utf8.AppendRune(tok.nameB[:0], data)
		t.state = doctypeNameState
	}
	return true
}

// lowerDoctypeName lowers the ASCII letters of the name of the current
// doctype.
func (t *htmlTokenizer) lowerDoctypeName() {
	for i, c := range t.currentToken.nameB {
		if 'A' <= c && c <= 'Z' {
			t.currentToken.nameB[i] = c + 'a' - 'A'
		}
	}
}

// doctypeNameState is HTMLTokenizer.doctype_name_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) doctypeNameState() bool {
	data := t.stream.character()
	tok := t.currentToken
	switch {
	case spaceCharacter(data):
		t.lowerDoctypeName()
		t.state = afterDoctypeNameState
	case data == '>':
		t.lowerDoctypeName()
		t.emitComment()
		t.state = dataState
	case data == '\u0000':
		t.parseError("invalid-codepoint")
		tok.nameB = utf8.AppendRune(tok.nameB, '�')
		t.state = doctypeNameState
	case data == eof:
		t.parseError("eof-in-doctype-name")
		tok.correct = false
		t.lowerDoctypeName()
		t.emitComment()
		t.state = dataState
	default:
		tok.nameB = utf8.AppendRune(tok.nameB, data)
	}
	return true
}

// afterDoctypeNameState is HTMLTokenizer.after_doctype_name_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) afterDoctypeNameState() bool {
	data := t.stream.character()
	tok := t.currentToken
	switch {
	case spaceCharacter(data):
	case data == '>':
		t.emitComment()
		t.state = dataState
	case data == eof:
		tok.correct = false
		t.stream.unget(data)
		t.parseError("eof-in-doctype")
		t.emitComment()
		t.state = dataState
	default:
		if data == 'p' || data == 'P' {
			matched := true
			for _, expected := range [...]string{"uU", "bB", "lL", "iI", "cC"} {
				data = t.stream.character()
				if data == eof || !strings.ContainsRune(expected, data) {
					matched = false
					break
				}
			}
			if matched {
				t.state = afterDoctypePublicKeywordState
				return true
			}
		} else if data == 's' || data == 'S' {
			matched := true
			for _, expected := range [...]string{"yY", "sS", "tT", "eE", "mM"} {
				data = t.stream.character()
				if data == eof || !strings.ContainsRune(expected, data) {
					matched = false
					break
				}
			}
			if matched {
				t.state = afterDoctypeSystemKeywordState
				return true
			}
		}

		// All the characters read before the current 'data' will be
		// [a-zA-Z], so they're garbage in the bogus doctype and can be
		// discarded; only the latest character might be '>' or EOF and
		// needs to be ungetted.
		t.stream.unget(data)
		t.parseError("expected-space-or-right-bracket-in-doctype", "data", string(data))
		tok.correct = false
		t.state = bogusDoctypeState
	}
	return true
}

// afterDoctypeKeyword is the body of after_doctype_public_keyword_state and
// after_doctype_system_keyword_state in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) afterDoctypeKeyword(before tokenizerState) bool {
	data := t.stream.character()
	switch {
	case spaceCharacter(data):
		t.state = before
	case data == '\'' || data == '"':
		t.parseError("unexpected-char-in-doctype")
		t.stream.unget(data)
		t.state = before
	case data == eof:
		t.parseError("eof-in-doctype")
		t.currentToken.correct = false
		t.emitComment()
		t.state = dataState
	default:
		t.stream.unget(data)
		t.state = before
	}
	return true
}

// afterDoctypePublicKeywordState is
// HTMLTokenizer.after_doctype_public_keyword_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) afterDoctypePublicKeywordState() bool {
	return t.afterDoctypeKeyword(beforeDoctypePublicIdentifierState)
}

// beforeDoctypePublicIdentifierState is
// HTMLTokenizer.before_doctype_public_identifier_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) beforeDoctypePublicIdentifierState() bool {
	data := t.stream.character()
	tok := t.currentToken
	switch {
	case spaceCharacter(data):
	case data == '"':
		tok.hasPublicID, tok.publicB = true, tok.publicB[:0]
		t.state = doctypePublicIdentifierDoubleQuotedState
	case data == '\'':
		tok.hasPublicID, tok.publicB = true, tok.publicB[:0]
		t.state = doctypePublicIdentifierSingleQuotedState
	case data == '>':
		t.parseError("unexpected-end-of-doctype")
		tok.correct = false
		t.emitComment()
		t.state = dataState
	case data == eof:
		t.parseError("eof-in-doctype")
		tok.correct = false
		t.emitComment()
		t.state = dataState
	default:
		t.parseError("unexpected-char-in-doctype")
		tok.correct = false
		t.state = bogusDoctypeState
	}
	return true
}

// doctypeIdentifierQuoted is the body of the states of the quoted public and
// system identifiers of a doctype in tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) doctypeIdentifierQuoted(quote rune, id *[]byte, after tokenizerState) bool {
	data := t.stream.character()
	tok := t.currentToken
	switch data {
	case quote:
		t.state = after
	case '\u0000':
		t.parseError("invalid-codepoint")
		*id = utf8.AppendRune(*id, '�')
	case '>':
		t.parseError("unexpected-end-of-doctype")
		tok.correct = false
		t.emitComment()
		t.state = dataState
	case eof:
		t.parseError("eof-in-doctype")
		tok.correct = false
		t.emitComment()
		t.state = dataState
	default:
		*id = utf8.AppendRune(*id, data)
	}
	return true
}

// doctypePublicIdentifierDoubleQuotedState is
// HTMLTokenizer.doctype_public_identifier_double_quoted_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) doctypePublicIdentifierDoubleQuotedState() bool {
	return t.doctypeIdentifierQuoted('"', &t.currentToken.publicB, afterDoctypePublicIdentifierState)
}

// doctypePublicIdentifierSingleQuotedState is
// HTMLTokenizer.doctype_public_identifier_single_quoted_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) doctypePublicIdentifierSingleQuotedState() bool {
	return t.doctypeIdentifierQuoted('\'', &t.currentToken.publicB, afterDoctypePublicIdentifierState)
}

// afterDoctypePublicIdentifierState is
// HTMLTokenizer.after_doctype_public_identifier_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) afterDoctypePublicIdentifierState() bool {
	data := t.stream.character()
	tok := t.currentToken
	switch {
	case spaceCharacter(data):
		t.state = betweenDoctypePublicAndSystemIdentifiersState
	case data == '>':
		t.emitComment()
		t.state = dataState
	case data == '"':
		t.parseError("unexpected-char-in-doctype")
		tok.hasSystemID, tok.systemB = true, tok.systemB[:0]
		t.state = doctypeSystemIdentifierDoubleQuotedState
	case data == '\'':
		t.parseError("unexpected-char-in-doctype")
		tok.hasSystemID, tok.systemB = true, tok.systemB[:0]
		t.state = doctypeSystemIdentifierSingleQuotedState
	case data == eof:
		t.parseError("eof-in-doctype")
		tok.correct = false
		t.emitComment()
		t.state = dataState
	default:
		t.parseError("unexpected-char-in-doctype")
		tok.correct = false
		t.state = bogusDoctypeState
	}
	return true
}

// betweenDoctypePublicAndSystemIdentifiersState is
// HTMLTokenizer.between_doctype_public_and_system_identifiers_state in
// tinyhtml5 tokenizer.py.
func (t *htmlTokenizer) betweenDoctypePublicAndSystemIdentifiersState() bool {
	data := t.stream.character()
	tok := t.currentToken
	switch {
	case spaceCharacter(data):
	case data == '>':
		t.emitComment()
		t.state = dataState
	case data == '"':
		tok.hasSystemID, tok.systemB = true, tok.systemB[:0]
		t.state = doctypeSystemIdentifierDoubleQuotedState
	case data == '\'':
		tok.hasSystemID, tok.systemB = true, tok.systemB[:0]
		t.state = doctypeSystemIdentifierSingleQuotedState
	case data == eof:
		t.parseError("eof-in-doctype")
		tok.correct = false
		t.emitComment()
		t.state = dataState
	default:
		t.parseError("unexpected-char-in-doctype")
		tok.correct = false
		t.state = bogusDoctypeState
	}
	return true
}

// afterDoctypeSystemKeywordState is
// HTMLTokenizer.after_doctype_system_keyword_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) afterDoctypeSystemKeywordState() bool {
	return t.afterDoctypeKeyword(beforeDoctypeSystemIdentifierState)
}

// beforeDoctypeSystemIdentifierState is
// HTMLTokenizer.before_doctype_system_identifier_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) beforeDoctypeSystemIdentifierState() bool {
	data := t.stream.character()
	tok := t.currentToken
	switch {
	case spaceCharacter(data):
	case data == '"':
		tok.hasSystemID, tok.systemB = true, tok.systemB[:0]
		t.state = doctypeSystemIdentifierDoubleQuotedState
	case data == '\'':
		tok.hasSystemID, tok.systemB = true, tok.systemB[:0]
		t.state = doctypeSystemIdentifierSingleQuotedState
	case data == '>':
		t.parseError("unexpected-char-in-doctype")
		tok.correct = false
		t.emitComment()
		t.state = dataState
	case data == eof:
		t.parseError("eof-in-doctype")
		tok.correct = false
		t.emitComment()
		t.state = dataState
	default:
		t.parseError("unexpected-char-in-doctype")
		tok.correct = false
		t.state = bogusDoctypeState
	}
	return true
}

// doctypeSystemIdentifierDoubleQuotedState is
// HTMLTokenizer.doctype_system_identifier_double_quoted_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) doctypeSystemIdentifierDoubleQuotedState() bool {
	return t.doctypeIdentifierQuoted('"', &t.currentToken.systemB, afterDoctypeSystemIdentifierState)
}

// doctypeSystemIdentifierSingleQuotedState is
// HTMLTokenizer.doctype_system_identifier_single_quoted_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) doctypeSystemIdentifierSingleQuotedState() bool {
	return t.doctypeIdentifierQuoted('\'', &t.currentToken.systemB, afterDoctypeSystemIdentifierState)
}

// afterDoctypeSystemIdentifierState is
// HTMLTokenizer.after_doctype_system_identifier_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) afterDoctypeSystemIdentifierState() bool {
	data := t.stream.character()
	switch {
	case spaceCharacter(data):
	case data == '>':
		t.emitComment()
		t.state = dataState
	case data == eof:
		t.parseError("eof-in-doctype")
		t.currentToken.correct = false
		t.emitComment()
		t.state = dataState
	default:
		t.parseError("unexpected-char-in-doctype")
		t.state = bogusDoctypeState
	}
	return true
}

// bogusDoctypeState is HTMLTokenizer.bogus_doctype_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) bogusDoctypeState() bool {
	data := t.stream.character()
	switch data {
	case '>':
		t.emitComment()
		t.state = dataState
	case eof:
		// XXX EMIT
		t.stream.unget(data)
		t.emitComment()
		t.state = dataState
	}
	return true
}

// cdataSectionState is HTMLTokenizer.cdata_section_state in tinyhtml5
// tokenizer.py.
func (t *htmlTokenizer) cdataSectionState() bool {
	var data []string
	for {
		data = append(data, t.stream.charsUntil("]", false))
		data = append(data, t.stream.charsUntil(">", false))
		char := t.stream.character()
		if char == eof {
			break
		}
		last := data[len(data)-1]
		if strings.HasSuffix(last, "]]") {
			data[len(data)-1] = last[:len(last)-2]
			break
		}
		data = append(data, string(char))
	}

	text := strings.Join(data, "")
	// Deal with null here rather than in the parser.
	if nullCount := strings.Count(text, "\u0000"); nullCount > 0 {
		for range nullCount {
			t.parseError("invalid-codepoint")
		}
		text = strings.ReplaceAll(text, "\u0000", "�")
	}
	if text != "" {
		t.characters(text)
	}
	t.state = dataState
	return true
}
