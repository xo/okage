// Copyright 2018 Yevhenii Reizner
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package roxmltree

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// isXMLNameStartChar is XmlCharExt::is_xml_name_start for char in roxmltree
// src/tokenizer.rs. It checks if the value is within the
// [NameStartChar](https://www.w3.org/TR/xml/#NT-NameStartChar) range.
func isXMLNameStartChar(c rune) bool {
	// Check for ASCII first.
	if uint32(c) <= 128 {
		return isXMLNameStartByte(byte(c))
	}

	switch {
	case c >= 0x0000C0 && c <= 0x0000D6,
		c >= 0x0000D8 && c <= 0x0000F6,
		c >= 0x0000F8 && c <= 0x0002FF,
		c >= 0x000370 && c <= 0x00037D,
		c >= 0x00037F && c <= 0x001FFF,
		c >= 0x00200C && c <= 0x00200D,
		c >= 0x002070 && c <= 0x00218F,
		c >= 0x002C00 && c <= 0x002FEF,
		c >= 0x003001 && c <= 0x00D7FF,
		c >= 0x00F900 && c <= 0x00FDCF,
		c >= 0x00FDF0 && c <= 0x00FFFD,
		c >= 0x010000 && c <= 0x0EFFFF:
		return true
	}
	return false
}

// isXMLNameChar is XmlCharExt::is_xml_name for char in roxmltree
// src/tokenizer.rs. It checks if the value is within the
// [NameChar](https://www.w3.org/TR/xml/#NT-NameChar) range.
func isXMLNameChar(c rune) bool {
	// Check for ASCII first.
	if uint32(c) <= 128 {
		return isXMLNameByte(byte(c))
	}

	switch {
	case c == 0x0000B7,
		c >= 0x0000C0 && c <= 0x0000D6,
		c >= 0x0000D8 && c <= 0x0000F6,
		c >= 0x0000F8 && c <= 0x0002FF,
		c >= 0x000300 && c <= 0x00036F,
		c >= 0x000370 && c <= 0x00037D,
		c >= 0x00037F && c <= 0x001FFF,
		c >= 0x00200C && c <= 0x00200D,
		c >= 0x00203F && c <= 0x002040,
		c >= 0x002070 && c <= 0x00218F,
		c >= 0x002C00 && c <= 0x002FEF,
		c >= 0x003001 && c <= 0x00D7FF,
		c >= 0x00F900 && c <= 0x00FDCF,
		c >= 0x00FDF0 && c <= 0x00FFFD,
		c >= 0x010000 && c <= 0x0EFFFF:
		return true
	}
	return false
}

// isXMLCharRune is XmlCharExt::is_xml_char for char in roxmltree
// src/tokenizer.rs. It checks if the value is within the
// [Char](https://www.w3.org/TR/xml/#NT-Char) range.
func isXMLCharRune(c rune) bool {
	// Does not check for surrogate code points U+D800-U+DFFF,
	// since that check was performed by Rust when the `&str` was constructed.
	//
	// Port: Go decodes a surrogate, like any invalid UTF-8, as U+FFFD.
	if uint32(c) < 0x20 {
		return isXMLSpace(byte(c))
	}

	return c != 0xFFFF && c != 0xFFFE
}

// isXMLSpace is XmlByteExt::is_xml_space in roxmltree src/tokenizer.rs. It
// checks if byte is a space.
//
// `[ \r\n\t]`.
func isXMLSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// isXMLNameStartByte is XmlByteExt::is_xml_name_start in roxmltree
// src/tokenizer.rs. It checks if the value is within the
// [NameStartChar](https://www.w3.org/TR/xml/#NT-NameStartChar) range.
func isXMLNameStartByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == ':' || b == '_'
}

// isXMLNameByte is XmlByteExt::is_xml_name in roxmltree src/tokenizer.rs. It
// checks if byte is within the ASCII
// [Char](https://www.w3.org/TR/xml/#NT-Char) range.
func isXMLNameByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') ||
		b == ':' || b == '_' || b == '-' || b == '.'
}

// isXMLCharByte is XmlByteExt::is_xml_char in roxmltree src/tokenizer.rs. It
// checks if the value is within the
// [Char](https://www.w3.org/TR/xml/#NT-Char) range.
func isXMLCharByte(b byte) bool {
	return b > 0x20 || isXMLSpace(b)
}

// isXMLStr is is_xml_str in roxmltree src/tokenizer.rs.
func isXMLStr(s string, valueStart int, stream *stream) error {
	if isASCII(s) {
		for i := range len(s) {
			b := s[i]
			if !isXMLCharByte(b) {
				return errNonXMLChar(rune(b), stream.GenTextPosFrom(valueStart+i))
			}
		}

		return nil
	}
	return isXMLStrUnicode(s, valueStart, stream)
}

// isASCII is <[u8]>::is_ascii of the Rust core library.
func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// isXMLStrUnicode is is_xml_str_unicode in roxmltree src/tokenizer.rs.
func isXMLStrUnicode(s string, valueStart int, stream *stream) error {
	for i, ch := range s {
		if !isXMLCharRune(ch) {
			return errNonXMLChar(ch, stream.GenTextPosFrom(valueStart+i))
		}
	}

	return nil
}

// strSpan is StrSpan in roxmltree src/tokenizer.rs. It is a string slice.
//
// Like `&str`, but also contains the position in the input XML
// from which it was parsed.
type strSpan struct {
	text  string
	start int
}

// strSpanFrom is From<&str> for strSpan in roxmltree src/tokenizer.rs.
func strSpanFrom(text string) strSpan {
	return strSpan{text: text, start: 0}
}

// strSpanFromSubstr is StrSpan::from_substr in roxmltree src/tokenizer.rs.
func strSpanFromSubstr(text string, start, end int) strSpan {
	return strSpan{
		text:  text[start:end],
		start: start,
	}
}

// Range is StrSpan::range in roxmltree src/tokenizer.rs.
func (s strSpan) Range() Range {
	return Range{Start: s.start, End: s.start + len(s.text)}
}

// AsStr is StrSpan::as_str in roxmltree src/tokenizer.rs.
func (s strSpan) AsStr() string {
	return s.text
}

// sliceRegion is StrSpan::slice_region in roxmltree src/tokenizer.rs.
func (s strSpan) sliceRegion(start, end int) string {
	return s.text[start:end]
}

// tokenKind is the kind of the enum Token in roxmltree src/tokenizer.rs.
type tokenKind uint8

// The kinds of Token.
const (
	// <?target content?>.
	tokenProcessingInstruction tokenKind = iota

	// <!-- text -->.
	tokenComment

	// <!ENTITY ns_extend "http://test.com">.
	tokenEntityDeclaration

	// <ns:elem.
	tokenElementStart

	// ns:attr="value".
	tokenAttribute

	tokenElementEnd

	// Contains text between elements including whitespaces.
	// Basically everything between `>` and `<`.
	// Except `]]>`, which is not allowed and will lead to an error.
	tokenText

	// <![CDATA[text]]>.
	tokenCdata
)

// token is Token in roxmltree src/tokenizer.rs. Each case holds a part of the
// same fields, so the enum is a struct with a kind (D27).
//
//   - ProcessingInstruction(target, content, range) uses prefix for the
//     target, local for the content and rng. An empty content is None,
//     because the tokenizer never gives Some("").
//   - Comment(text, range), Text(text, range) and Cdata(text, range) use
//     local for the text and rng.
//   - EntityDeclaration(name, value) uses local for the name and span.
//   - ElementStart(prefix, local, start) uses prefix, local and start.
//   - Attribute(range, qname_len, eq_len, prefix, local, value) uses rng,
//     qnameLen, eqLen, prefix, local and span.
//   - ElementEnd(end, range) uses end and rng.
type token struct {
	kind     tokenKind
	prefix   string
	local    string
	span     strSpan
	rng      Range
	start    int
	qnameLen uint16
	eqLen    uint8
	end      elementEnd
}

// elementEndKind is the kind of the enum ElementEnd in roxmltree
// src/tokenizer.rs.
type elementEndKind uint8

// The kinds of ElementEnd.
const (
	// Indicates `>`.
	elementEndOpen elementEndKind = iota
	// Indicates `</ns:name>`.
	elementEndClose
	// Indicates `/>`.
	elementEndEmpty
)

// elementEnd is ElementEnd in roxmltree src/tokenizer.rs, the `ElementEnd`
// token. Only the case Close uses prefix and local (D27).
type elementEnd struct {
	kind   elementEndKind
	prefix string
	local  string
}

// xmlEvents is XmlEvents in roxmltree src/tokenizer.rs.
type xmlEvents interface {
	token(t token) error

	// resolveEntity is XmlEvents::resolve_entity. The default of upstream
	// returns Ok(None). The value is None when ok is false.
	resolveEntity(pubID string, hasPubID bool, uri string) (value string, ok bool, err error)
}

// parseTokenizer is parse in roxmltree src/tokenizer.rs.
//
//	document ::= prolog element Misc*
func parseTokenizer(text string, allowDTD bool, events xmlEvents) error {
	s := newStream(text)

	if err := parseDeclaration(&s); err != nil {
		return err
	}

	if err := parseMisc(&s, events); err != nil {
		return err
	}

	s.skipSpaces()
	if s.startsWith("<!DOCTYPE") {
		if !allowDTD {
			return errDtdDetected()
		}

		if err := parseDoctype(&s, events); err != nil {
			return err
		}
		if err := parseMisc(&s, events); err != nil {
			return err
		}
	}

	s.skipSpaces()
	if c, err := s.CurrByte(); err == nil && c == '<' {
		if err := parseElement(&s, events); err != nil {
			return err
		}
	}

	if err := parseMisc(&s, events); err != nil {
		return err
	}

	if !s.AtEnd() {
		return errUnknownToken(s.GenTextPos())
	}

	return nil
}

// parseMisc is parse_misc in roxmltree src/tokenizer.rs.
//
//	Misc ::= Comment | PI | S
func parseMisc(s *stream, events xmlEvents) error {
	for !s.AtEnd() {
		s.skipSpaces()
		switch {
		case s.startsWith("<!--"):
			if err := parseComment(s, events); err != nil {
				return err
			}
		case s.startsWith("<?"):
			if err := parsePI(s, events); err != nil {
				return err
			}
		default:
			return nil
		}
	}

	return nil
}

// parseDeclaration is parse_declaration in roxmltree src/tokenizer.rs.
//
//	XMLDecl ::= '<?xml' VersionInfo EncodingDecl? SDDecl? S? '?>'
//
// We don't actually return a token for the XML declaration and only validate it.
func parseDeclaration(s *stream) error {
	// Skip UTF-8 BOM.
	if s.startsWith("\xEF\xBB\xBF") {
		s.Advance(3)
	}

	if !s.startsWith("<?xml ") {
		return nil
	}

	// consumeSpaces is the function consume_spaces inside
	// parse_declaration.
	consumeSpaces := func(s *stream) error {
		if s.startsWithSpace() {
			s.skipSpaces()
		} else if !s.startsWith("?>") && !s.AtEnd() {
			return errInvalidChar2(
				"a whitespace",
				s.CurrByteUnchecked(),
				s.GenTextPos(),
			)
		}

		return nil
	}

	s.Advance(5) // <?xml
	if err := consumeSpaces(s); err != nil {
		return err
	}

	// The `version` "attribute" is mandatory.
	if !s.startsWith("version") {
		// Will trigger the InvalidString error, which is what we want.
		return s.skipString("version")
	}
	if _, _, _, err := parseAttribute(s); err != nil {
		return err
	}
	if err := consumeSpaces(s); err != nil {
		return err
	}

	if s.startsWith("encoding") {
		if _, _, _, err := parseAttribute(s); err != nil {
			return err
		}
		if err := consumeSpaces(s); err != nil {
			return err
		}
	}

	if s.startsWith("standalone") {
		if _, _, _, err := parseAttribute(s); err != nil {
			return err
		}
	}

	s.skipSpaces()
	return s.skipString("?>")
}

// parseComment is parse_comment in roxmltree src/tokenizer.rs.
//
//	'<!--' ((Char - '-') | ('-' (Char - '-')))* '-->'
func parseComment(s *stream, events xmlEvents) error {
	start := s.Pos()
	s.Advance(4)
	text, err := s.consumeChars(func(s *stream, c rune) bool { return !(c == '-' && s.startsWith("-->")) })
	if err != nil {
		return err
	}
	if err := s.skipString("-->"); err != nil {
		return err
	}

	if strings.Contains(text, "--") {
		return errInvalidComment(s.GenTextPosFrom(start))
	}

	if strings.HasSuffix(text, "-") {
		return errInvalidComment(s.GenTextPosFrom(start))
	}

	rng := s.rangeFrom(start)
	return events.token(token{kind: tokenComment, local: text, rng: rng})
}

// parsePI is parse_pi in roxmltree src/tokenizer.rs.
//
//	PI       ::= '<?' PITarget (S (Char* - (Char* '?>' Char*)))? '?>'
//	PITarget ::= Name - (('X' | 'x') ('M' | 'm') ('L' | 'l'))
func parsePI(s *stream, events xmlEvents) error {
	if s.startsWith("<?xml ") {
		return errUnexpectedDeclaration(s.GenTextPos())
	}

	start := s.Pos()
	s.Advance(2)
	target, err := s.consumeName()
	if err != nil {
		return err
	}
	s.skipSpaces()
	// Port: an empty content is None, as upstream gives.
	content, err := s.consumeChars(func(s *stream, c rune) bool { return !(c == '?' && s.startsWith("?>")) })
	if err != nil {
		return err
	}

	if err := s.skipString("?>"); err != nil {
		return err
	}

	rng := s.rangeFrom(start)
	return events.token(token{kind: tokenProcessingInstruction, prefix: target, local: content, rng: rng})
}

// parseDoctype is parse_doctype in roxmltree src/tokenizer.rs.
func parseDoctype(s *stream, events xmlEvents) error {
	start := s.Pos()
	if err := parseDoctypeStart(s); err != nil {
		return err
	}
	s.skipSpaces()

	if c, err := s.CurrByte(); err == nil && c == '>' {
		s.Advance(1)
		return nil
	}

	s.Advance(1) // [
	for !s.AtEnd() {
		s.skipSpaces()
		switch {
		case s.startsWith("<!ENTITY"):
			if err := parseEntityDecl(s, events); err != nil {
				return err
			}
		case s.startsWith("<!--"):
			if err := parseComment(s, events); err != nil {
				return err
			}
		case s.startsWith("<?"):
			if err := parsePI(s, events); err != nil {
				return err
			}
		case s.startsWith("]"):
			// DTD ends with ']' S? '>', therefore we have to skip possible spaces.
			s.Advance(1)
			s.skipSpaces()
			c, err := s.CurrByte()
			if err != nil {
				return errUnexpectedEndOfStream()
			}
			if c == '>' {
				s.Advance(1)
				return nil
			}
			return errInvalidChar2("'>'", c, s.GenTextPos())
		case s.startsWith("<!ELEMENT") ||
			s.startsWith("<!ATTLIST") ||
			s.startsWith("<!NOTATION"):
			if consumeDecl(s) != nil {
				pos := s.GenTextPosFrom(start)
				return errUnknownToken(pos)
			}
		default:
			return errUnknownToken(s.GenTextPos())
		}
	}

	return nil
}

// parseDoctypeStart is parse_doctype_start in roxmltree src/tokenizer.rs.
//
//	doctypedecl ::= '<!DOCTYPE' S Name (S ExternalID)? S? ('[' intSubset ']' S?)? '>'
func parseDoctypeStart(s *stream) error {
	s.Advance(9)

	if err := s.consumeSpaces(); err != nil {
		return err
	}
	if err := s.skipName(); err != nil {
		return err
	}
	s.skipSpaces()

	if _, err := parseExternalID(s); err != nil {
		return err
	}
	s.skipSpaces()

	c, err := s.CurrByte()
	if err != nil {
		return err
	}
	if c != '[' && c != '>' {
		return errInvalidChar2("'[' or '>'", c, s.GenTextPos())
	}

	return nil
}

// externalID is the value of parse_external_id in roxmltree
// src/tokenizer.rs: Option<(Option<&str>, &str)>. ok is false for None, and
// hasPubID is false when the public ID is None.
type externalID struct {
	ok       bool
	pubID    string
	hasPubID bool
	uri      string
}

// parseExternalID is parse_external_id in roxmltree src/tokenizer.rs.
//
//	ExternalID ::= 'SYSTEM' S SystemLiteral | 'PUBLIC' S PubidLiteral S SystemLiteral
func parseExternalID(s *stream) (externalID, error) {
	var v externalID
	if s.startsWith("SYSTEM") || s.startsWith("PUBLIC") {
		start := s.Pos()
		s.Advance(6)
		id := s.sliceBack(start)

		if err := s.consumeSpaces(); err != nil {
			return externalID{}, err
		}
		quote, err := s.consumeQuote()
		if err != nil {
			return externalID{}, err
		}
		first := s.consumeBytes(func(c byte) bool { return c != quote })
		if err := s.consumeByte(quote); err != nil {
			return externalID{}, err
		}

		if id == "SYSTEM" {
			v = externalID{ok: true, uri: first}
		} else {
			if err := s.consumeSpaces(); err != nil {
				return externalID{}, err
			}
			quote, err := s.consumeQuote()
			if err != nil {
				return externalID{}, err
			}
			second := s.consumeBytes(func(c byte) bool { return c != quote })
			if err := s.consumeByte(quote); err != nil {
				return externalID{}, err
			}

			v = externalID{ok: true, pubID: first, hasPubID: true, uri: second}
		}
	}

	return v, nil
}

// parseEntityDecl is parse_entity_decl in roxmltree src/tokenizer.rs.
//
//	EntityDecl  ::= GEDecl | PEDecl
//	GEDecl      ::= '<!ENTITY' S Name S EntityDef S? '>'
//	PEDecl      ::= '<!ENTITY' S '%' S Name S PEDef S? '>'
func parseEntityDecl(s *stream, events xmlEvents) error {
	s.Advance(8)
	if err := s.consumeSpaces(); err != nil {
		return err
	}

	isGE := true
	if s.tryConsumeByte('%') {
		if err := s.consumeSpaces(); err != nil {
			return err
		}
		isGE = false
	}

	name, err := s.consumeName()
	if err != nil {
		return err
	}
	if err := s.consumeSpaces(); err != nil {
		return err
	}
	definition, ok, err := parseEntityDef(s, events, isGE)
	if err != nil {
		return err
	}
	if ok {
		if err := events.token(token{kind: tokenEntityDeclaration, local: name, span: definition}); err != nil {
			return err
		}
	}
	s.skipSpaces()
	return s.consumeByte('>')
}

// parseEntityDef is parse_entity_def in roxmltree src/tokenizer.rs.
//
//	EntityDef   ::= EntityValue | (ExternalID NDataDecl?)
//	PEDef       ::= EntityValue | ExternalID
//	EntityValue ::= '"' ([^%&"] | PEReference | Reference)* '"' |  "'" ([^%&']
//	                            | PEReference | Reference)* "'"
//	ExternalID  ::= 'SYSTEM' S SystemLiteral | 'PUBLIC' S PubidLiteral S SystemLiteral
//	NDataDecl   ::= S 'NDATA' S Name
func parseEntityDef(s *stream, events xmlEvents, isGE bool) (strSpan, bool, error) {
	c, err := s.CurrByte()
	if err != nil {
		return strSpan{}, false, err
	}
	switch c {
	case '"', '\'':
		quote, err := s.consumeQuote()
		if err != nil {
			return strSpan{}, false, err
		}
		start := s.Pos()
		s.skipBytes(func(c byte) bool { return c != quote })
		value := s.sliceBackSpan(start)
		if err := s.consumeByte(quote); err != nil {
			return strSpan{}, false, err
		}
		return value, true, nil
	case 'S', 'P':
		id, err := parseExternalID(s)
		if err != nil {
			return strSpan{}, false, err
		}
		if !id.ok {
			return strSpan{}, false, errInvalidExternalID(s.GenTextPos())
		}
		if isGE {
			s.skipSpaces()
			if s.startsWith("NDATA") {
				s.Advance(5)
				if err := s.consumeSpaces(); err != nil {
					return strSpan{}, false, err
				}
				if err := s.skipName(); err != nil {
					return strSpan{}, false, err
				}
				// TODO: NDataDecl is not supported
			}
		}

		value, ok, err := events.resolveEntity(id.pubID, id.hasPubID, id.uri)
		if err != nil {
			return strSpan{}, false, errEntityResolver(s.GenTextPos(), err)
		}

		if !ok {
			return strSpan{}, false, nil
		}
		stream := newStream(value)
		if err := parseDeclaration(&stream); err != nil {
			return strSpan{}, false, err
		}
		return strSpanFrom(value[stream.pos:]), true, nil
	default:
		pos := s.GenTextPos()
		return strSpan{}, false, errInvalidChar2("a quote, SYSTEM or PUBLIC", c, pos)
	}
}

// consumeDecl is consume_decl in roxmltree src/tokenizer.rs.
func consumeDecl(s *stream) error {
	s.skipBytes(func(c byte) bool { return c != '>' })
	return s.consumeByte('>')
}

// parseElement is parse_element in roxmltree src/tokenizer.rs.
//
//	element ::= EmptyElemTag | STag content ETag
//	'<' Name (S Attribute)* S? '>'
func parseElement(s *stream, events xmlEvents) error {
	start := s.Pos()
	s.Advance(1) // <
	prefix, local, err := s.consumeQName()
	if err != nil {
		return err
	}
	if err := events.token(token{kind: tokenElementStart, prefix: prefix, local: local, start: start}); err != nil {
		return err
	}

	open := false
loop:
	for !s.AtEnd() {
		hasSpace := s.startsWithSpace()
		s.skipSpaces()
		start := s.Pos()
		c, err := s.CurrByte()
		if err != nil {
			return err
		}
		switch c {
		case '/':
			s.Advance(1)
			if err := s.consumeByte('>'); err != nil {
				return err
			}
			rng := s.rangeFrom(start)
			if err := events.token(token{kind: tokenElementEnd, end: elementEnd{kind: elementEndEmpty}, rng: rng}); err != nil {
				return err
			}
			break loop
		case '>':
			s.Advance(1)
			rng := s.rangeFrom(start)
			if err := events.token(token{kind: tokenElementEnd, end: elementEnd{kind: elementEndOpen}, rng: rng}); err != nil {
				return err
			}
			open = true
			break loop
		default:
			// An attribute must be preceded with a whitespace.
			if !hasSpace {
				// Will always trigger an error. Which is what we want.
				if err := s.consumeSpaces(); err != nil {
					return err
				}
			}

			// Manual inlining of `parse_attribute` for performance.
			// We cannot mark `parse_attribute` as `#[inline(always)]`
			// because it will blow up the binary size.
			prefix, local, err := s.consumeQName()
			if err != nil {
				return err
			}
			qnameEnd := s.Pos()
			qnameLen := uint16(min(qnameEnd-start, 0xFFFF))
			if err := s.consumeEq(); err != nil {
				return err
			}
			eqLen := uint8(min(s.Pos()-qnameEnd, 0xFF))
			quote, err := s.consumeQuote()
			if err != nil {
				return err
			}
			// The attribute value must not contain the < character.
			valueStart := s.Pos()
			if err := s.advanceUntil2(quote, '<'); err != nil {
				return err
			}
			value := s.sliceBackSpan(valueStart)
			if err := isXMLStr(value.AsStr(), valueStart, s); err != nil {
				return err
			}
			if err := s.consumeByte(quote); err != nil {
				return err
			}
			end := s.Pos()
			if err := events.token(token{
				kind:     tokenAttribute,
				rng:      Range{Start: start, End: end},
				qnameLen: qnameLen,
				eqLen:    eqLen,
				prefix:   prefix,
				local:    local,
				span:     value,
			}); err != nil {
				return err
			}
		}
	}

	if open {
		return parseContent(s, events)
	}

	return nil
}

// parseAttribute is parse_attribute in roxmltree src/tokenizer.rs.
//
//	Attribute ::= Name Eq AttValue
//
//nolint:unparam // the port keeps the results of upstream, which no caller reads.
func parseAttribute(s *stream) (string, string, strSpan, error) {
	prefix, local, err := s.consumeQName()
	if err != nil {
		return "", "", strSpan{}, err
	}
	if err := s.consumeEq(); err != nil {
		return "", "", strSpan{}, err
	}
	quote, err := s.consumeQuote()
	if err != nil {
		return "", "", strSpan{}, err
	}
	quoteC := rune(quote)
	// The attribute value must not contain the < character.
	valueStart := s.Pos()
	if err := s.skipChars(func(_ *stream, c rune) bool { return c != quoteC && c != '<' }); err != nil {
		return "", "", strSpan{}, err
	}
	value := s.sliceBackSpan(valueStart)
	if err := s.consumeByte(quote); err != nil {
		return "", "", strSpan{}, err
	}
	return prefix, local, value, nil
}

// parseContent is parse_content in roxmltree src/tokenizer.rs.
//
//	content ::= CharData? ((element | Reference | CDSect | PI | Comment) CharData?)*
func parseContent(s *stream, events xmlEvents) error {
	for !s.AtEnd() {
		c, err := s.CurrByte()
		if err != nil {
			return errUnknownToken(s.GenTextPos())
		}
		if c != '<' {
			if err := parseText(s, events); err != nil {
				return err
			}
			continue
		}
		next, err := s.nextByte()
		if err != nil {
			return errUnknownToken(s.GenTextPos())
		}
		switch next {
		case '!':
			switch {
			case s.startsWith("<!--"):
				if err := parseComment(s, events); err != nil {
					return err
				}
			case s.startsWith("<![CDATA["):
				if err := parseCdata(s, events); err != nil {
					return err
				}
			default:
				return errUnknownToken(s.GenTextPos())
			}
		case '?':
			if err := parsePI(s, events); err != nil {
				return err
			}
		case '/':
			return parseCloseElement(s, events)
		default:
			if err := parseElement(s, events); err != nil {
				return err
			}
		}
	}

	return nil
}

// parseCdata is parse_cdata in roxmltree src/tokenizer.rs.
//
//	CDSect  ::= CDStart CData CDEnd
//	CDStart ::= '<![CDATA['
//	CData   ::= (Char* - (Char* ']]>' Char*))
//	CDEnd   ::= ']]>'
func parseCdata(s *stream, events xmlEvents) error {
	start := s.Pos()
	s.Advance(9) // <![CDATA[
	text, err := s.consumeChars(func(s *stream, c rune) bool { return !(c == ']' && s.startsWith("]]>")) })
	if err != nil {
		return err
	}
	if err := s.skipString("]]>"); err != nil {
		return err
	}
	rng := s.rangeFrom(start)
	return events.token(token{kind: tokenCdata, local: text, rng: rng})
}

// parseCloseElement is parse_close_element in roxmltree src/tokenizer.rs.
//
//	'</' Name S? '>'
func parseCloseElement(s *stream, events xmlEvents) error {
	start := s.Pos()
	s.Advance(2) // </

	prefix, tagName, err := s.consumeQName()
	if err != nil {
		return err
	}
	s.skipSpaces()
	if err := s.consumeByte('>'); err != nil {
		return err
	}

	rng := s.rangeFrom(start)
	return events.token(token{
		kind: tokenElementEnd,
		end:  elementEnd{kind: elementEndClose, prefix: prefix, local: tagName},
		rng:  rng,
	})
}

// parseText is parse_text in roxmltree src/tokenizer.rs.
func parseText(s *stream, events xmlEvents) error {
	start := s.Pos()
	text, err := s.consumeChars(func(_ *stream, c rune) bool { return c != '<' })
	if err != nil {
		return err
	}

	// According to the spec, `]]>` must not appear inside a Text node.
	// https://www.w3.org/TR/xml/#syntax
	//
	// Search for `>` first, since it's a bit faster than looking for `]]>`.
	if strings.IndexByte(text, '>') >= 0 && strings.Contains(text, "]]>") {
		return errInvalidCharacterData(s.GenTextPos())
	}

	rng := s.rangeFrom(start)
	return events.token(token{kind: tokenText, local: text, rng: rng})
}

// referenceKind is the kind of the enum reference in roxmltree
// src/tokenizer.rs.
type referenceKind uint8

// The kinds of reference.
const (
	// An entity reference.
	//
	// <https://www.w3.org/TR/xml/#NT-EntityRef>
	referenceEntity referenceKind = iota

	// A character reference.
	//
	// <https://www.w3.org/TR/xml/#NT-CharRef>
	referenceChar
)

// reference is Reference in roxmltree src/tokenizer.rs, the representation
// of the [Reference](https://www.w3.org/TR/xml/#NT-Reference) value. The
// case Entity uses entity and the case Char uses char (D27).
type reference struct {
	kind   referenceKind
	entity string
	char   rune
}

// stream is Stream in roxmltree src/tokenizer.rs.
type stream struct {
	pos  int
	end  int
	span strSpan
}

// newStream is Stream::new in roxmltree src/tokenizer.rs.
func newStream(text string) stream {
	return stream{
		pos:  0,
		end:  len(text),
		span: strSpanFrom(text),
	}
}

// streamFromSubstr is Stream::from_substr in roxmltree src/tokenizer.rs.
func streamFromSubstr(text string, fragment Range) stream {
	return stream{
		pos:  fragment.Start,
		end:  fragment.End,
		span: strSpanFrom(text),
	}
}

// Pos is Stream::pos in roxmltree src/tokenizer.rs.
func (s *stream) Pos() int {
	return s.pos
}

// AtEnd is Stream::at_end in roxmltree src/tokenizer.rs.
func (s *stream) AtEnd() bool {
	return s.pos >= s.end
}

// CurrByte is Stream::curr_byte in roxmltree src/tokenizer.rs.
func (s *stream) CurrByte() (byte, error) {
	if s.AtEnd() {
		return 0, errUnexpectedEndOfStream()
	}

	return s.CurrByteUnchecked(), nil
}

// CurrByteUnchecked is Stream::curr_byte_unchecked in roxmltree
// src/tokenizer.rs.
func (s *stream) CurrByteUnchecked() byte {
	return s.span.text[s.pos]
}

// nextByte is Stream::next_byte in roxmltree src/tokenizer.rs.
func (s *stream) nextByte() (byte, error) {
	if s.pos+1 >= s.end {
		return 0, errUnexpectedEndOfStream()
	}

	return s.span.AsStr()[s.pos+1], nil
}

// asBytes is Stream::as_bytes in roxmltree src/tokenizer.rs.
func (s *stream) asBytes() string {
	return s.span.text[s.pos:s.end]
}

// Advance is Stream::advance in roxmltree src/tokenizer.rs.
func (s *stream) Advance(n int) {
	s.pos += n
}

// startsWith is Stream::starts_with in roxmltree src/tokenizer.rs.
func (s *stream) startsWith(text string) bool {
	return strings.HasPrefix(s.span.text[s.pos:s.end], text)
}

// consumeByte is Stream::consume_byte in roxmltree src/tokenizer.rs.
func (s *stream) consumeByte(c byte) error {
	curr, err := s.CurrByte()
	if err != nil {
		return err
	}
	if curr != c {
		return errInvalidChar(c, curr, s.GenTextPos())
	}

	s.Advance(1)
	return nil
}

// tryConsumeByte is Stream::try_consume_byte in roxmltree src/tokenizer.rs.
//
// Unlike `consume_byte()` will not return any errors.
func (s *stream) tryConsumeByte(c byte) bool {
	b, err := s.CurrByte()
	if err == nil && b == c {
		s.Advance(1)
		return true
	}
	return false
}

// skipString is Stream::skip_string in roxmltree src/tokenizer.rs.
func (s *stream) skipString(text string) error {
	if !s.startsWith(text) {
		pos := s.GenTextPos()

		// Assume that all input `text` are valid UTF-8 strings, so unwrap is safe.
		expected := text

		return errInvalidString(expected, pos)
	}

	s.Advance(len(text))
	return nil
}

// consumeBytes is Stream::consume_bytes in roxmltree src/tokenizer.rs.
func (s *stream) consumeBytes(f func(byte) bool) string {
	start := s.pos
	s.skipBytes(f)
	return s.sliceBack(start)
}

// skipBytes is Stream::skip_bytes in roxmltree src/tokenizer.rs.
func (s *stream) skipBytes(f func(byte) bool) {
	for !s.AtEnd() && f(s.CurrByteUnchecked()) {
		s.Advance(1)
	}
}

// consumeChars is Stream::consume_chars in roxmltree src/tokenizer.rs.
func (s *stream) consumeChars(f func(*stream, rune) bool) (string, error) {
	start := s.pos
	if err := s.skipChars(f); err != nil {
		return "", err
	}
	return s.sliceBack(start), nil
}

// skipChars is Stream::skip_chars in roxmltree src/tokenizer.rs.
//
// Port: upstream iterates over the chars of the rest of the stream. Each char
// that it does not stop at moves the position by its length, so the loop
// decodes the char at the position each time. A byte of invalid UTF-8, which
// a Rust &str cannot hold, decodes as U+FFFD with a length of one.
func (s *stream) skipChars(f func(*stream, rune) bool) error {
	for !s.AtEnd() {
		c, size := utf8.DecodeRuneInString(s.span.text[s.pos:s.end])
		if !isXMLCharRune(c) {
			return errNonXMLChar(c, s.GenTextPos())
		} else if f(s, c) {
			s.Advance(size)
		} else {
			break
		}
	}

	return nil
}

// advanceUntil2 is Stream::advance_until2 in roxmltree src/tokenizer.rs.
func (s *stream) advanceUntil2(needle1, needle2 byte) error {
	b := s.asBytes()
	for i := range len(b) {
		if b[i] == needle1 || b[i] == needle2 {
			s.Advance(i)
			return nil
		}
	}
	return errUnexpectedEndOfStream()
}

// chars is Stream::chars in roxmltree src/tokenizer.rs. Go ranges over the
// string that it returns.
func (s *stream) chars() string {
	return s.span.AsStr()[s.pos:s.end]
}

// sliceBack is Stream::slice_back in roxmltree src/tokenizer.rs.
func (s *stream) sliceBack(pos int) string {
	return s.span.sliceRegion(pos, s.pos)
}

// sliceBackSpan is Stream::slice_back_span in roxmltree src/tokenizer.rs.
func (s *stream) sliceBackSpan(pos int) strSpan {
	return strSpanFromSubstr(s.span.text, pos, s.pos)
}

// rangeFrom is Stream::range_from in roxmltree src/tokenizer.rs.
func (s *stream) rangeFrom(start int) Range {
	return Range{Start: start, End: s.pos}
}

// skipSpaces is Stream::skip_spaces in roxmltree src/tokenizer.rs.
func (s *stream) skipSpaces() {
	for s.startsWithSpace() {
		s.Advance(1)
	}
}

// startsWithSpace is Stream::starts_with_space in roxmltree
// src/tokenizer.rs.
func (s *stream) startsWithSpace() bool {
	return !s.AtEnd() && isXMLSpace(s.CurrByteUnchecked())
}

// consumeSpaces is Stream::consume_spaces in roxmltree src/tokenizer.rs.
//
// Like `skip_spaces()`, but checks that first char is actually a space.
func (s *stream) consumeSpaces() error {
	if s.AtEnd() {
		return errUnexpectedEndOfStream()
	}

	if !s.startsWithSpace() {
		return errInvalidChar2(
			"a whitespace",
			s.CurrByteUnchecked(),
			s.GenTextPos(),
		)
	}

	s.skipSpaces()
	return nil
}

// ConsumeReference is Stream::consume_reference in roxmltree
// src/tokenizer.rs.
//
// Consumes according to: <https://www.w3.org/TR/xml/#NT-Reference>
func (s *stream) ConsumeReference() (reference, bool) {
	if !s.tryConsumeByte('&') {
		return reference{}, false
	}

	// Port: the Rust local reference is ref, because reference is the type.
	var ref reference
	if s.tryConsumeByte('#') {
		var value string
		var radix int
		if s.tryConsumeByte('x') {
			value = s.consumeBytes(isASCIIHexDigit)
			radix = 16
		} else {
			value = s.consumeBytes(isASCIIDigit)
			radix = 10
		}

		n, err := strconv.ParseUint(value, radix, 32)
		if err != nil {
			return reference{}, false
		}

		c := rune(n)
		if !utf8.ValidRune(c) {
			c = '�'
		}
		if !isXMLCharRune(c) {
			return reference{}, false
		}

		ref = reference{kind: referenceChar, char: c}
	} else {
		name, err := s.consumeName()
		if err != nil {
			return reference{}, false
		}
		switch name {
		case "quot":
			ref = reference{kind: referenceChar, char: '"'}
		case "amp":
			ref = reference{kind: referenceChar, char: '&'}
		case "apos":
			ref = reference{kind: referenceChar, char: '\''}
		case "lt":
			ref = reference{kind: referenceChar, char: '<'}
		case "gt":
			ref = reference{kind: referenceChar, char: '>'}
		default:
			ref = reference{kind: referenceEntity, entity: name}
		}
	}

	if s.consumeByte(';') != nil {
		return reference{}, false
	}

	return ref, true
}

// isASCIIHexDigit is u8::is_ascii_hexdigit of the Rust core library.
func isASCIIHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// isASCIIDigit is u8::is_ascii_digit of the Rust core library.
func isASCIIDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// consumeName is Stream::consume_name in roxmltree src/tokenizer.rs.
//
// Consumes according to: <https://www.w3.org/TR/xml/#NT-Name>
func (s *stream) consumeName() (string, error) {
	start := s.Pos()
	if err := s.skipName(); err != nil {
		return "", err
	}

	name := s.sliceBack(start)
	if name == "" {
		return "", errInvalidName(s.GenTextPosFrom(start))
	}

	return name, nil
}

// skipName is Stream::skip_name in roxmltree src/tokenizer.rs.
//
// The same as `consume_name()`, but does not return a consumed name.
func (s *stream) skipName() error {
	start := s.Pos()
	rest := s.chars()
	if rest != "" {
		c, size := utf8.DecodeRuneInString(rest)
		if !isXMLNameStartChar(c) {
			return errInvalidName(s.GenTextPosFrom(start))
		}
		s.Advance(size)
		rest = rest[size:]
	}

	for rest != "" {
		c, size := utf8.DecodeRuneInString(rest)
		if !isXMLNameChar(c) {
			break
		}
		s.Advance(size)
		rest = rest[size:]
	}

	return nil
}

// consumeQName is Stream::consume_qname in roxmltree src/tokenizer.rs.
//
// Consumes a qualified XML name and returns it.
//
// Consumes according to: <https://www.w3.org/TR/xml-names/#ns-qualnames>
func (s *stream) consumeQName() (string, string, error) {
	start := s.Pos()

	splitter, hasSplitter := 0, false

	for !s.AtEnd() {
		// Check for ASCII first for performance reasons.
		b := s.CurrByteUnchecked()
		if b < 128 {
			if b == ':' {
				if !hasSplitter {
					splitter, hasSplitter = s.Pos(), true
					s.Advance(1)
				} else {
					// Multiple `:` is an error.
					return "", "", errInvalidName(s.GenTextPosFrom(start))
				}
			} else if isXMLNameByte(b) {
				s.Advance(1)
			} else {
				break
			}
		} else {
			// Fallback to Unicode code point.
			c, size := utf8.DecodeRuneInString(s.chars())
			if !isXMLNameChar(c) {
				break
			}
			s.Advance(size)
		}
	}

	var prefix, local string
	if hasSplitter {
		prefix = s.span.sliceRegion(start, splitter)
		local = s.sliceBack(splitter + 1)
	} else {
		local = s.sliceBack(start)
		// Slice an empty prefix. This way we can preserve attribute start position.
		prefix = s.span.sliceRegion(start, start)
	}

	// isXMLNameStart is the function is_xml_name_start inside consume_qname.
	isXMLNameStart := func(name string) bool {
		if name != "" {
			b := name[0]
			if b < 128 {
				return isXMLNameStartByte(b)
			}
			c, _ := utf8.DecodeRuneInString(name)
			return isXMLNameStartChar(c)
		}

		return false
	}

	// Prefix must be empty or start with a `NameStartChar`.
	if prefix != "" && !isXMLNameStart(prefix) {
		return "", "", errInvalidName(s.GenTextPosFrom(start))
	}

	// Local name must start with a `NameStartChar`.
	if !isXMLNameStart(local) {
		return "", "", errInvalidName(s.GenTextPosFrom(start))
	}

	return prefix, local, nil
}

// consumeEq is Stream::consume_eq in roxmltree src/tokenizer.rs.
func (s *stream) consumeEq() error {
	s.skipSpaces()
	if err := s.consumeByte('='); err != nil {
		return err
	}
	s.skipSpaces()

	return nil
}

// consumeQuote is Stream::consume_quote in roxmltree src/tokenizer.rs.
func (s *stream) consumeQuote() (byte, error) {
	c, err := s.CurrByte()
	if err != nil {
		return 0, err
	}
	if c == '\'' || c == '"' {
		s.Advance(1)
		return c, nil
	}
	return 0, errInvalidChar2("a quote", c, s.GenTextPos())
}

// GenTextPos is Stream::gen_text_pos in roxmltree src/tokenizer.rs.
//
// Calculates a current absolute position.
//
// This operation is very expensive. Use only for errors.
func (s *stream) GenTextPos() TextPos {
	text := s.span.AsStr()
	end := s.pos

	row := calcCurrRow(text, end)
	col := calcCurrCol(text, end)
	return NewTextPos(row, col)
}

// GenTextPosFrom is Stream::gen_text_pos_from in roxmltree src/tokenizer.rs.
//
// Calculates an absolute position at `pos`.
//
// This operation is very expensive. Use only for errors.
func (s *stream) GenTextPosFrom(pos int) TextPos {
	c := *s
	c.pos = min(pos, len(c.span.AsStr()))
	return c.GenTextPos()
}

// calcCurrRow is Stream::calc_curr_row in roxmltree src/tokenizer.rs.
func calcCurrRow(text string, end int) uint32 {
	row := uint32(1)
	for i := range end {
		if text[i] == '\n' {
			row++
		}
	}

	return row
}

// calcCurrCol is Stream::calc_curr_col in roxmltree src/tokenizer.rs.
func calcCurrCol(text string, end int) uint32 {
	col := uint32(1)
	t := text[:end]
	for t != "" {
		c, size := utf8.DecodeLastRuneInString(t)
		if c == '\n' {
			break
		}
		col++
		t = t[:len(t)-size]
	}

	return col
}
