// Copyright (c) 2006-2013 James Graham and other contributors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package html

// This file ports tinyhtml5 inputstream.py (D49). Upstream reads its input in
// chunks of 10240 characters. The port decodes the whole input at once, so it
// is one chunk: the line breaks are made the same at the start, and each
// "invalid-codepoint" error comes after the first read.

import (
	"bytes"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

// htmlInputStream is HTMLUnicodeInputStream and HTMLBinaryInputStream in
// tinyhtml5 inputstream.py. It provides a Unicode stream of characters to the
// tokenizer. It takes care of the character encoding, and it replaces the
// sequences of bytes that are not valid.
type htmlInputStream struct {
	// chunk holds the characters of the input, with each line break as '\n'.
	chunk []rune

	// chunkOffset is the offset of the next character in chunk.
	chunkOffset int

	// lines holds the offset of each '\n' in chunk, in order.
	lines []int

	// errors are the errors that the stream found and the tokenizer has not
	// given yet.
	errors []string

	// encoding is the name of the encoding, and certain says if it is
	// certain or tentative.
	encoding string
	certain  bool

	// decoded is the name of the encoding that decoded the input. It differs
	// from encoding when html has no decoder for encoding.
	decoded string

	// source is the input of HTMLUnicodeInputStream.
	source string

	// The binary stream only.
	binary          bool
	rawStream       []byte
	rawPos          int
	numberBytesMeta int
	overrideEnc     string
	defaultEnc      string
	charsetReader   CharsetReader
}

// newUnicodeInputStream is HTMLUnicodeInputStream.__init__ in tinyhtml5
// inputstream.py, for a string.
func newUnicodeInputStream(source string) *htmlInputStream {
	s := &htmlInputStream{source: source, encoding: encodingUTF8, certain: true}
	s.reset()
	return s
}

// reset is HTMLUnicodeInputStream.reset and HTMLBinaryInputStream.reset in
// tinyhtml5 inputstream.py. It decodes the input again from the start.
func (s *htmlInputStream) reset() {
	text := s.source
	s.errors = nil
	s.decoded = s.encoding
	if s.binary {
		text = s.decode()
	}
	s.chunk, s.lines = s.readChunk(text)
	s.chunkOffset = 0
}

// position is HTMLUnicodeInputStream.position in tinyhtml5 inputstream.py.
// It returns the line, from 1, and the column, from 0, of the current
// position in the stream.
func (s *htmlInputStream) position() (int, int) {
	offset := s.chunkOffset
	numberLines := sort.SearchInts(s.lines, offset)
	if numberLines == 0 {
		return 1, offset
	}
	return numberLines + 1, offset - (s.lines[numberLines-1] + 1)
}

// character is HTMLUnicodeInputStream.character in tinyhtml5
// inputstream.py. It reads one character from the stream, and it returns eof
// at the end.
func (s *htmlInputStream) character() rune {
	if s.chunkOffset >= len(s.chunk) {
		return eof
	}
	c := s.chunk[s.chunkOffset]
	s.chunkOffset++
	return c
}

// readChunk is HTMLUnicodeInputStream.read_chunk in tinyhtml5
// inputstream.py, for the whole input. It reports the characters that are
// not valid, and it makes each "\r\n" and each "\r" a "\n".
func (s *htmlInputStream) readChunk(data string) ([]rune, []int) {
	chunk := make([]rune, 0, utf8.RuneCountInString(data))
	var lines []int
	cr := false
	for _, c := range data {
		// Report character errors.
		if invalidUnicode(c) {
			s.errors = append(s.errors, "invalid-codepoint")
		}
		// Replace invalid characters.
		switch {
		case c == '\r':
			cr = true
			c = '\n'
		case c == '\n' && cr:
			cr = false
			continue
		default:
			cr = false
		}
		if c == '\n' {
			lines = append(lines, len(chunk))
		}
		chunk = append(chunk, c)
	}
	return chunk, lines
}

// invalidUnicode is invalid_unicode_re in tinyhtml5 inputstream.py. A
// surrogate cannot be in a Go string, so the port does not test for one.
func invalidUnicode(c rune) bool {
	switch {
	case 0x0001 <= c && c <= 0x0008, c == 0x000B, 0x000E <= c && c <= 0x001F,
		0x007F <= c && c <= 0x009F, 0xFDD0 <= c && c <= 0xFDEF:
		return true
	case c >= 0xFFFE:
		return c&0xFFFE == 0xFFFE
	}
	return false
}

// charsUntil is HTMLUnicodeInputStream.chars_until in tinyhtml5
// inputstream.py. It returns the characters from the stream up to the first
// character in characters, or up to the end. With opposite, it returns the
// characters up to the first character that is not in characters.
func (s *htmlInputStream) charsUntil(characters string, opposite bool) string {
	start := s.chunkOffset
	for s.chunkOffset < len(s.chunk) {
		if strings.ContainsRune(characters, s.chunk[s.chunkOffset]) != opposite {
			break
		}
		s.chunkOffset++
	}
	return string(s.chunk[start:s.chunkOffset])
}

// unget is HTMLUnicodeInputStream.unget in tinyhtml5 inputstream.py. It puts
// back the last character that the stream gave.
func (s *htmlInputStream) unget(c rune) {
	if c != eof {
		s.chunkOffset--
	}
}

// newBinaryInputStream is HTMLBinaryInputStream.__init__ in tinyhtml5
// inputstream.py. It finds the encoding of source and decodes it.
func newBinaryInputStream(source []byte, o *documentOptions) *htmlInputStream {
	s := &htmlInputStream{
		binary:    true,
		rawStream: source,
		// Number of bytes to use when looking for a meta element with
		// encoding information.
		numberBytesMeta: 1024,
		// Encodings given as arguments.
		overrideEnc:   o.charset,
		defaultEnc:    o.defaultCharset,
		charsetReader: o.charsetReader,
	}
	// Determine encoding.
	s.encoding, s.certain = s.determineEncoding()
	// Reset and set Unicode stream.
	s.reset()
	return s
}

// decode is the stream reader that HTMLBinaryInputStream.reset in tinyhtml5
// inputstream.py makes. It decodes the raw stream from its position with
// the encoding of the stream.
func (s *htmlInputStream) decode() string {
	data := s.rawStream[min(s.rawPos, len(s.rawStream)):]
	if text, ok := decodeBytes(s.encoding, data); ok {
		return text
	}
	if s.charsetReader != nil {
		r, err := s.charsetReader(s.encoding, bytes.NewReader(data))
		if err == nil && r != nil {
			b, err := io.ReadAll(r)
			if err == nil {
				return decodeUTF8(b)
			}
		}
		s.errors = append(s.errors, "charset-reader-failed")
	} else {
		s.errors = append(s.errors, "unsupported-encoding")
	}
	s.decoded = encodingWindows1252
	return decodeWindows1252(data)
}

// determineEncoding is HTMLBinaryInputStream.determine_encoding in tinyhtml5
// inputstream.py. It returns the name of the encoding, and true when the
// encoding is certain.
func (s *htmlInputStream) determineEncoding() (string, bool) {
	// BOMs take precedence over everything. This will also read past the
	// BOM if present.
	if encoding, ok := s.detectBOM(); ok {
		return encoding, true
	}

	// If we've been overridden, we've been overridden.
	if encoding, ok := lookupEncoding(s.overrideEnc); ok {
		return encoding, true
	}

	// Look for meta elements with encoding information.
	if encoding, ok := s.detectEncodingMeta(); ok {
		return encoding, false
	}

	// Try the default encoding.
	if encoding, ok := lookupEncoding(s.defaultEnc); ok {
		return encoding, false
	}

	// Fallback to tinyhtml5's default if even that hasn't worked.
	return encodingWindows1252, false
}

// changeEncoding is HTMLBinaryInputStream.change_encoding in tinyhtml5
// inputstream.py. It returns true when the parser must parse the input again,
// where upstream raises ReparseError.
func (s *htmlInputStream) changeEncoding(label string) bool {
	newEncoding, ok := lookupEncoding(label)
	if !ok || !s.binary {
		return false
	}
	switch newEncoding {
	case encodingUTF16BE, encodingUTF16LE:
		// Port: upstream also sets the new encoding to utf-8 here, and then
		// does nothing with it.
	case s.encoding:
		s.certain = true
	default:
		s.rawPos = 0
		s.encoding, s.certain = newEncoding, true
		s.reset()
		return true
	}
	return false
}

// detectBOM is HTMLBinaryInputStream.detect_bom in tinyhtml5 inputstream.py.
// It returns the encoding of the BOM at the start of the stream, and false
// when no BOM gives an encoding.
func (s *htmlInputStream) detectBOM() (string, bool) {
	// Go to beginning of file and read in 4 bytes.
	str := s.rawStream[:min(4, len(s.rawStream))]

	// Try detecting the BOM using bytes from the string.
	for _, seek := range [...]int{3, 4, 2} { // UTF-8, UTF-32, UTF-16
		if encoding, ok := bomEncoding(str[:min(seek, len(str))]); ok {
			// Set the read position past the BOM if one was found.
			s.rawPos = seek
			return lookupEncoding(encoding)
		}
	}

	// Otherwise, set it to the start of the stream.
	s.rawPos = 0
	return "", false
}

// bomEncoding is the dict boms of HTMLBinaryInputStream.detect_bom in
// tinyhtml5 inputstream.py.
func bomEncoding(b []byte) (string, bool) {
	switch string(b) {
	case "\xEF\xBB\xBF":
		return "utf-8", true
	case "\xFF\xFE":
		return "utf-16le", true
	case "\xFE\xFF":
		return "utf-16be", true
	case "\xFF\xFE\x00\x00":
		return "utf-32le", true
	case "\x00\x00\xFE\xFF":
		return "utf-32be", true
	}
	return "", false
}

// detectEncodingMeta is HTMLBinaryInputStream.detect_encoding_meta in
// tinyhtml5 inputstream.py. It returns the encoding that a meta element
// declares.
func (s *htmlInputStream) detectEncodingMeta() (string, bool) {
	start := min(s.rawPos, len(s.rawStream))
	buffer := s.rawStream[start:min(start+s.numberBytesMeta, len(s.rawStream))]
	parser := newEncodingParser(buffer)
	s.rawPos = 0
	encoding, ok := parser.getEncoding()

	if ok && (encoding == encodingUTF16BE || encoding == encodingUTF16LE) {
		encoding = encodingUTF8
	}

	return encoding, ok
}

// encodingBytes is EncodingBytes in tinyhtml5 inputstream.py: bytes with a
// position and various extra methods. A method that returns false where
// upstream raises StopIteration stops the parse of the encoding.
type encodingBytes struct {
	data      []byte
	_position int
}

// newEncodingBytes is EncodingBytes.__new__ and EncodingBytes.__init__ in
// tinyhtml5 inputstream.py. It holds the bytes in lower case.
func newEncodingBytes(value []byte) *encodingBytes {
	return &encodingBytes{data: bytes.ToLower(value), _position: -1}
}

// next is EncodingBytes.__next__ in tinyhtml5 inputstream.py.
func (e *encodingBytes) next() (int, bool) {
	e._position++
	if e._position >= len(e.data) {
		return 0, false
	}
	return int(e.data[e._position]), true
}

// previous is EncodingBytes.previous in tinyhtml5 inputstream.py. No caller
// reads the byte that upstream returns.
func (e *encodingBytes) previous() {
	e._position--
}

// setPosition is EncodingBytes.set_position in tinyhtml5 inputstream.py.
func (e *encodingBytes) setPosition(position int) bool {
	if e._position >= len(e.data) {
		return false
	}
	e._position = max(0, position)
	return true
}

// position is EncodingBytes.get_position in tinyhtml5 inputstream.py.
// Upstream returns None before the first byte, and each use of None reads it
// as 0.
func (e *encodingBytes) position() (int, bool) {
	if e._position >= len(e.data) {
		return 0, false
	}
	return max(0, e._position), true
}

// currentByte is EncodingBytes.current_byte in tinyhtml5 inputstream.py.
func (e *encodingBytes) currentByte() (int, bool) {
	position, ok := e.position()
	if !ok {
		return 0, false
	}
	return int(e.data[position]), true
}

// skip is EncodingBytes.skip in tinyhtml5 inputstream.py. It skips past the
// characters, and returns the next byte or -1.
func (e *encodingBytes) skip(characters func(int) bool) (int, bool) {
	position, ok := e.position() // Use property for the error-checking
	if !ok {
		return 0, false
	}
	for position < len(e.data) {
		character := int(e.data[position])
		if !characters(character) {
			e._position = position
			return character, true
		}
		position++
	}
	e._position = position
	return -1, true
}

// skipUntil is EncodingBytes.skip_until in tinyhtml5 inputstream.py.
func (e *encodingBytes) skipUntil(characters func(int) bool) (int, bool) {
	position, ok := e.position()
	if !ok {
		return 0, false
	}
	for position < len(e.data) {
		character := int(e.data[position])
		if characters(character) {
			e._position = position
			return character, true
		}
		position++
	}
	e._position = position
	return -1, true
}

// matchBytes is EncodingBytes.match_bytes in tinyhtml5 inputstream.py. It
// looks for a sequence of bytes at the position. If the bytes are found, it
// returns true and moves the position past them.
func (e *encodingBytes) matchBytes(b string) (bool, bool) {
	position, ok := e.position()
	if !ok {
		return false, false
	}
	result := strings.HasPrefix(string(e.data[position:]), b)
	if result {
		e.setPosition(position + len(b))
	}
	return result, true
}

// jumpTo is EncodingBytes.jump_to in tinyhtml5 inputstream.py. It looks for
// the next sequence of bytes that matches b, and moves the position to the
// last byte of the match.
func (e *encodingBytes) jumpTo(b string) bool {
	position, ok := e.position()
	if !ok {
		return false
	}
	i := bytes.Index(e.data[position:], []byte(b))
	if i < 0 {
		return false
	}
	e._position = position + i + len(b) - 1
	return true
}

// spaceCharacterByte is space_characters_bytes in tinyhtml5 inputstream.py.
func spaceCharacterByte(c int) bool {
	return c >= 0 && spaceCharacter(rune(c))
}

// spaceOrSlash is space_characters_bytes | frozenset([b"/"]) in tinyhtml5
// inputstream.py.
func spaceOrSlash(c int) bool {
	return spaceCharacterByte(c) || c == '/'
}

// spacesAngleBrackets is spaces_angle_brackets in tinyhtml5 inputstream.py.
func spacesAngleBrackets(c int) bool {
	return spaceCharacterByte(c) || c == '>' || c == '<'
}

// asciiUppercaseByte is ascii_uppercase_bytes in tinyhtml5 inputstream.py.
func asciiUppercaseByte(c int) bool {
	return 'A' <= c && c <= 'Z'
}

// asciiLetterByte is ascii_letters_bytes in tinyhtml5 inputstream.py.
func asciiLetterByte(c int) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// encodingParser is EncodingParser in tinyhtml5 inputstream.py, the mini
// parser that finds the character encoding in the meta elements.
type encodingParser struct {
	data     *encodingBytes
	encoding string
}

// newEncodingParser is EncodingParser.__init__ in tinyhtml5 inputstream.py.
func newEncodingParser(data []byte) *encodingParser {
	return &encodingParser{data: newEncodingBytes(data)}
}

// getEncoding is EncodingParser.get_encoding in tinyhtml5 inputstream.py.
func (p *encodingParser) getEncoding() (string, bool) {
	if !bytes.Contains(p.data.data, []byte("<meta")) {
		return "", false
	}

	methodDispatch := [...]struct {
		key    string
		method func() (bool, bool)
	}{
		{"<!--", p.handleComment},
		{"<meta", p.handleMeta},
		{"</", p.handlePossibleEndTag},
		{"<!", p.handleOther},
		{"<?", p.handleOther},
		{"<", p.handlePossibleStartTag},
	}
	for range len(p.data.data) {
		keepParsing := true
		if !p.data.jumpTo("<") {
			break
		}
		for _, m := range methodDispatch {
			matched, ok := p.data.matchBytes(m.key)
			if !ok {
				keepParsing = false
				break
			}
			if matched {
				keep, ok := m.method()
				keepParsing = keep && ok
				break
			}
		}
		if !keepParsing {
			break
		}
	}

	return p.encoding, p.encoding != ""
}

// handleComment is EncodingParser.handle_comment in tinyhtml5
// inputstream.py. It skips over comments.
func (p *encodingParser) handleComment() (bool, bool) {
	ok := p.data.jumpTo("-->")
	return ok, ok
}

// handleMeta is EncodingParser.handle_meta in tinyhtml5 inputstream.py.
func (p *encodingParser) handleMeta() (bool, bool) {
	c, ok := p.data.currentByte()
	if !ok {
		return false, false
	}
	if !spaceCharacterByte(c) {
		// If we have <meta not followed by a space so just keep going.
		return true, true
	}
	// We have a valid meta element we want to search for attributes.
	hasPragma := false
	pendingEncoding := ""
	for {
		// Try to find the next attribute after the current position.
		name, value, found, ok := p.getAttribute()
		if !ok {
			return false, false
		}
		if !found {
			return true, true
		}

		switch name {
		case "http-equiv":
			hasPragma = value == "content-type"
			if hasPragma && pendingEncoding != "" {
				p.encoding = pendingEncoding
				return false, true
			}
		case "charset":
			if codec, ok := lookupEncodingBytes(value); ok {
				p.encoding = codec
				return false, true
			}
		case "content":
			contentParser := newContentAttributeParser(newEncodingBytes([]byte(value)))
			if tentativeEncoding, ok := contentParser.parse(); ok {
				if codec, ok := lookupEncodingBytes(tentativeEncoding); ok {
					if hasPragma {
						p.encoding = codec
						return false, true
					}
					pendingEncoding = codec
				}
			}
		}
	}
}

// handlePossibleStartTag is EncodingParser.handle_possible_start_tag in
// tinyhtml5 inputstream.py.
func (p *encodingParser) handlePossibleStartTag() (bool, bool) {
	return p.handlePossibleTag(false)
}

// handlePossibleEndTag is EncodingParser.handle_possible_end_tag in
// tinyhtml5 inputstream.py.
func (p *encodingParser) handlePossibleEndTag() (bool, bool) {
	if _, ok := p.data.next(); !ok {
		return false, false
	}
	return p.handlePossibleTag(true)
}

// handlePossibleTag is EncodingParser.handle_possible_tag in tinyhtml5
// inputstream.py.
func (p *encodingParser) handlePossibleTag(endTag bool) (bool, bool) {
	data := p.data
	c, ok := data.currentByte()
	if !ok {
		return false, false
	}
	if !asciiLetterByte(c) {
		// If the next byte is not an ASCII letter either ignore this
		// fragment (possible start tag case) or treat it according to
		// handle_other.
		if endTag {
			data.previous()
			if _, ok := p.handleOther(); !ok {
				return false, false
			}
		}
		return true, true
	}

	character, ok := data.skipUntil(spacesAngleBrackets)
	if !ok {
		return false, false
	}
	if character == '<' {
		// Return to the first step in the overall "two step" algorithm
		// reprocessing the < byte.
		data.previous()
	} else {
		// Read all attributes.
		for {
			_, _, found, ok := p.getAttribute()
			if !ok {
				return false, false
			}
			if !found {
				break
			}
		}
	}
	return true, true
}

// handleOther is EncodingParser.handle_other in tinyhtml5 inputstream.py.
func (p *encodingParser) handleOther() (bool, bool) {
	ok := p.data.jumpTo(">")
	return ok, ok
}

// getAttribute is EncodingParser.get_attribute in tinyhtml5 inputstream.py.
// It returns the name and the value of the next attribute in the stream, and
// false when it finds no attribute. The last result is false where upstream
// raises StopIteration.
func (p *encodingParser) getAttribute() (name, value string, found, ok bool) {
	data := p.data
	// Step 1 (skip characters).
	character, ok := data.skip(spaceOrSlash)
	if !ok {
		return "", "", false, false
	}
	// Step 2.
	if character == '>' || character == -1 {
		return "", "", false, true
	}
	// Step 3.
	var attributeName, attributeValue []byte
	// Step 4 attribute name.
name:
	for {
		switch {
		case character == '=' && len(attributeName) > 0:
			break name
		case spaceCharacterByte(character):
			// Step 6!
			if character, ok = data.skip(spaceCharacterByte); !ok {
				return "", "", false, false
			}
			break name
		case character == '/' || character == '>':
			return string(attributeName), "", true, true
		case asciiUppercaseByte(character):
			attributeName = append(attributeName, byte(character)+'a'-'A')
		case character == -1:
			return "", "", false, true
		default:
			attributeName = append(attributeName, byte(character))
		}
		// Step 5.
		if character, ok = data.next(); !ok {
			return "", "", false, false
		}
	}
	// Step 7.
	if character != '=' {
		data.previous()
		return string(attributeName), "", true, true
	}
	// Step 8.
	if _, ok := data.next(); !ok {
		return "", "", false, false
	}
	// Step 9.
	if character, ok = data.skip(spaceCharacterByte); !ok {
		return "", "", false, false
	}
	// Step 10.
	switch quote := character; {
	case quote == '\'' || quote == '"':
		// 10.1.
		for {
			// 10.2.
			if character, ok = data.next(); !ok {
				return "", "", false, false
			}
			switch {
			case character == quote:
				// 10.3.
				if _, ok := data.next(); !ok {
					return "", "", false, false
				}
				return string(attributeName), string(attributeValue), true, true
			case asciiUppercaseByte(character):
				// 10.4.
				attributeValue = append(attributeValue, byte(character)+'a'-'A')
			default:
				// 10.5.
				attributeValue = append(attributeValue, byte(character))
			}
		}
	case character == '>':
		return string(attributeName), "", true, true
	case asciiUppercaseByte(character):
		attributeValue = append(attributeValue, byte(character)+'a'-'A')
	case character == -1:
		return "", "", false, true
	default:
		attributeValue = append(attributeValue, byte(character))
	}
	// Step 11.
	for {
		if character, ok = data.next(); !ok {
			return "", "", false, false
		}
		switch {
		case spacesAngleBrackets(character):
			return string(attributeName), string(attributeValue), true, true
		case asciiUppercaseByte(character):
			attributeValue = append(attributeValue, byte(character)+'a'-'A')
		default:
			attributeValue = append(attributeValue, byte(character))
		}
	}
}

// contentAttributeParser is ContentAttributeParser in tinyhtml5
// inputstream.py.
type contentAttributeParser struct {
	data *encodingBytes
}

// newContentAttributeParser is ContentAttributeParser.__init__ in tinyhtml5
// inputstream.py.
func newContentAttributeParser(data *encodingBytes) *contentAttributeParser {
	return &contentAttributeParser{data: data}
}

// parse is ContentAttributeParser.parse in tinyhtml5 inputstream.py. It
// returns the charset of the value of a content attribute.
func (p *contentAttributeParser) parse() (string, bool) {
	data := p.data
	// Check if the attribute name is charset, otherwise return.
	if !data.jumpTo("charset") {
		return "", false
	}
	if !data.setPosition(data._position + 1) {
		return "", false
	}
	if _, ok := data.skip(spaceCharacterByte); !ok {
		return "", false
	}
	if c, ok := data.currentByte(); !ok || c != '=' {
		// If there is no = sign, keep looking for attributes.
		return "", false
	}
	position, _ := data.position()
	if !data.setPosition(position + 1) {
		return "", false
	}
	if _, ok := data.skip(spaceCharacterByte); !ok {
		return "", false
	}
	// Look for an encoding between matching quote marks.
	c, ok := data.currentByte()
	if !ok {
		return "", false
	}
	if c == '"' || c == '\'' {
		quote := c
		position, _ := data.position()
		if !data.setPosition(position + 1) {
			return "", false
		}
		oldPosition, ok := data.position()
		if !ok {
			return "", false
		}
		if data.jumpTo(string(rune(quote))) {
			position, _ := data.position()
			return string(data.data[oldPosition:position]), true
		}
		return "", false
	}
	// Unquoted value.
	oldPosition, _ := data.position()
	if _, ok := data.skipUntil(spaceCharacterByte); ok {
		position, ok := data.position()
		if !ok {
			// Return the whole remaining value.
			return string(data.data[oldPosition:]), true
		}
		return string(data.data[oldPosition:position]), true
	}
	// Return the whole remaining value.
	return string(data.data[oldPosition:]), true
}

// lookupEncodingBytes is lookup_encoding in tinyhtml5 inputstream.py, for a
// label in bytes. A label that is not ASCII has no encoding.
func lookupEncodingBytes(label string) (string, bool) {
	for i := range len(label) {
		if label[i] >= 0x80 {
			return "", false
		}
	}
	return lookupEncoding(label)
}
