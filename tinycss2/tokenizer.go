// Copyright 2013-2020 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package tinycss2

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// newlineReplacer is the chain of str.replace calls at the start of
// parse_component_value_list in tinycss2 tinycss2/tokenizer.py.
var newlineReplacer = strings.NewReplacer(
	"\x00", "�",
	"\r\n", "\n",
	"\r", "\n",
	"\f", "\n",
)

// frame is an item of the stack of nested blocks of the tokenizer: the list
// to append to, and the character that closes the block.
type frame struct {
	tokens  *[]Node
	endChar rune
}

// ParseComponentValueList is parse_component_value_list in tinycss2
// tinycss2/tokenizer.py. It parses a list of component values.
//
// With WithSkipComments(true), the result, and recursively its blocks and
// functions, holds no Comment. WithMaxDepth bounds the nesting of the blocks
// and functions (D50).
//
// The tokenizer works on the code points of css, as Python does, so that each
// column counts code points. A byte of css that is not valid UTF-8 reads as
// U+FFFD.
func ParseComponentValueList(input string, opts ...Option) []Node {
	o := newOptions(opts)
	return parseComponentValueList(input, o.skipComments, o.maxDepth)
}

// parseComponentValueList is parse_component_value_list.
func parseComponentValueList(input string, skipComments bool, maxDepth int) []Node {
	css := []rune(newlineReplacer.Replace(input))
	length := len(css)
	tokenStartPos, pos := 0, 0 // Character index in the css source.
	line := 1                  // First line is line 1.
	lastNewline := -1
	root := []Node{}
	tokens := &root
	var endChar rune // Pop the stack when encountering this character.
	var stack []frame
	// sink holds the content of the blocks that are deeper than maxDepth.
	// The tokenizer drops it.
	sink := new([]Node)
	add := func(n Node) {
		if tokens != sink {
			*tokens = append(*tokens, n)
		}
	}
	// push opens a block whose content is content. It returns false when
	// the block is too deep, and the caller must not add the block.
	push := func(content *[]Node, end rune, line, column int) bool {
		stack = append(stack, frame{tokens, endChar})
		endChar = end
		if len(stack) > maxDepth {
			if tokens != sink {
				*tokens = append(*tokens, NewParseError(line, column, "too-deep", "Blocks nested too deep"))
			}
			tokens = sink
			return false
		}
		tokens = content
		return true
	}

	for pos < length {
		if newline := lastIndexRune(css, '\n', tokenStartPos, pos); newline != -1 {
			line += 1 + countRune(css, '\n', tokenStartPos, newline)
			lastNewline = newline
		}
		// First character in a line is in column 1.
		column := pos - lastNewline
		tokenStartPos = pos
		c := css[pos]

		switch {
		case c == ' ' || c == '\n' || c == '\t':
			pos++
			for pos < length && (css[pos] == ' ' || css[pos] == '\n' || css[pos] == '\t') {
				pos++
			}
			add(NewWhitespaceToken(line, column, string(css[tokenStartPos:pos])))
			continue
		case (c == 'U' || c == 'u') && pos+2 < length && css[pos+1] == '+' &&
			(isHexDigit(css[pos+2]) || css[pos+2] == '?'):
			var start, end int
			start, end, pos = consumeUnicodeRange(css, pos+2)
			add(NewUnicodeRangeToken(line, column, start, end))
			continue
		case startsWith(css, pos, "-->"): // Check before identifiers
			add(NewLiteralToken(line, column, "-->"))
			pos += 3
			continue
		case isIdentStart(css, pos):
			var value string
			value, pos = consumeIdent(css, pos)
			if !startsWith(css, pos, "(") { // Not a function
				add(NewIdentToken(line, column, value))
				continue
			}
			pos++ // Skip the '('
			if asciiLower(value) == TypeURL {
				urlPos := pos
				for urlPos < length && (css[urlPos] == ' ' || css[urlPos] == '\n' || css[urlPos] == '\t') {
					urlPos++
				}
				if urlPos >= length || (css[urlPos] != '"' && css[urlPos] != '\'') {
					var (
						urlValue string
						ok       bool
						err      *tokenError
					)
					urlValue, ok, pos, err = consumeURL(css, pos)
					if ok {
						repr := "url(" + serializeURL(urlValue) + ")"
						if err != nil {
							if err.kind == kindEOFInString {
								repr = trimLastRunes(repr, 2)
							} else {
								repr = trimLastRunes(repr, 1)
							}
						}
						add(NewURLToken(line, column, urlValue, repr))
					}
					if err != nil {
						add(NewParseError(line, column, err.kind, err.message))
					}
					continue
				}
			}
			function := NewFunctionBlock(line, column, value, []Node{})
			if parent := tokens; push(&function.Arguments, ')', line, column) {
				*parent = append(*parent, function)
			}
			continue
		}

		if end, isInteger, ok := matchNumber(css, pos); ok {
			pos = end
			repr := string(css[tokenStartPos:pos])
			value, _ := strconv.ParseFloat(repr, 64)
			var intValue int64
			if isInteger {
				intValue = parseIntSaturating(repr)
			}
			switch {
			case pos < length && isIdentStart(css, pos):
				var unit string
				unit, pos = consumeIdent(css, pos)
				add(NewDimensionToken(line, column, value, intValue, isInteger, repr, unit))
			case startsWith(css, pos, "%"):
				pos++
				add(NewPercentageToken(line, column, value, intValue, isInteger, repr))
			default:
				add(NewNumberToken(line, column, value, intValue, isInteger, repr))
			}
			continue
		}

		switch {
		case c == '@':
			pos++
			if pos < length && isIdentStart(css, pos) {
				var value string
				value, pos = consumeIdent(css, pos)
				add(NewAtKeywordToken(line, column, value))
			} else {
				add(NewLiteralToken(line, column, "@"))
			}
		case c == '#':
			pos++
			if pos < length && (isNameChar(css[pos]) ||
				// Valid escape:
				(css[pos] == '\\' && !startsWith(css, pos, "\\\n"))) {
				isIdentifier := isIdentStart(css, pos)
				var value string
				value, pos = consumeIdent(css, pos)
				add(NewHashToken(line, column, value, isIdentifier))
			} else {
				add(NewLiteralToken(line, column, "#"))
			}
		case c == '{':
			block := NewCurlyBracketsBlock(line, column, []Node{})
			if parent := tokens; push(&block.Content, '}', line, column) {
				*parent = append(*parent, block)
			}
			pos++
		case c == '[':
			block := NewSquareBracketsBlock(line, column, []Node{})
			if parent := tokens; push(&block.Content, ']', line, column) {
				*parent = append(*parent, block)
			}
			pos++
		case c == '(':
			block := NewParenthesesBlock(line, column, []Node{})
			if parent := tokens; push(&block.Content, ')', line, column) {
				*parent = append(*parent, block)
			}
			pos++
		case c == endChar && endChar != 0: // Matching }, ] or )
			// The top-level endChar is 0 (never equal to a character), so
			// we never get here if the stack is empty.
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			tokens, endChar = top.tokens, top.endChar
			pos++
		case c == '}' || c == ']' || c == ')':
			add(NewParseError(line, column, string(c), "Unmatched "+string(c)))
			pos++
		case c == '"' || c == '\'':
			var (
				value string
				ok    bool
				err   *tokenError
			)
			value, ok, pos, err = consumeQuotedString(css, pos)
			if ok {
				repr := `"` + serializeStringValue(value) + `"`
				if err != nil {
					repr = repr[:len(repr)-1]
				}
				add(NewStringToken(line, column, value, repr))
			}
			if err != nil {
				add(NewParseError(line, column, err.kind, err.message))
			}
		case startsWith(css, pos, "/*"): // Comment
			pos = indexRunes(css, "*/", pos+2)
			if pos == -1 {
				if !skipComments {
					add(&Comment{Location{line, column}, string(css[tokenStartPos+2:])})
				}
				// The loop ends here, as with break in upstream.
				pos = length
				continue
			}
			if !skipComments {
				add(&Comment{Location{line, column}, string(css[tokenStartPos+2 : pos])})
			}
			pos += 2
		case startsWith(css, pos, "<!--"):
			add(NewLiteralToken(line, column, "<!--"))
			pos += 4
		case startsWith(css, pos, "||"):
			add(NewLiteralToken(line, column, "||"))
			pos += 2
		case strings.ContainsRune("~|^$*", c):
			pos++
			if startsWith(css, pos, "=") {
				pos++
				add(NewLiteralToken(line, column, string(c)+"="))
			} else {
				add(NewLiteralToken(line, column, string(c)))
			}
		default:
			add(NewLiteralToken(line, column, string(c)))
			pos++
		}
	}
	return root
}

// The kinds and the messages of the errors of the tokenizer.
const (
	kindEOFInString = "eof-in-string"
	kindEOFInURL    = "eof-in-url"
	messageEOFInURL = "EOF in URL"
)

// tokenError is the tuple (kind, message) of an error of the tokenizer.
type tokenError struct {
	kind    string
	message string
}

// matchNumber is _NUMBER_RE.match in tinycss2 tinycss2/tokenizer.py, the
// regular expression `[-+]?([0-9]*\.)?[0-9]+([eE][+-]?[0-9]+)?`. It returns
// the end of the match, and whether neither group matched, which makes the
// number an integer.
func matchNumber(css []rune, pos int) (int, bool, bool) {
	n := len(css)
	p := pos
	if p < n && (css[p] == '+' || css[p] == '-') {
		p++
	}
	q := p
	for q < n && isDigit(css[q]) {
		q++
	}
	isInteger := true
	if q < n && css[q] == '.' && q+1 < n && isDigit(css[q+1]) {
		q++
		for q < n && isDigit(css[q]) {
			q++
		}
		isInteger = false
	} else if q == p {
		return 0, false, false
	}
	if q < n && (css[q] == 'e' || css[q] == 'E') {
		r := q + 1
		if r < n && (css[r] == '+' || css[r] == '-') {
			r++
		}
		if r < n && isDigit(css[r]) {
			for r < n && isDigit(css[r]) {
				r++
			}
			q = r
			isInteger = false
		}
	}
	return q, isInteger, true
}

// parseIntSaturating is int() of Python for the representation of an
// integer. A value out of the range of int64 saturates.
func parseIntSaturating(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		if strings.HasPrefix(s, "-") {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return v
}

// isNameStart is _is_name_start in tinycss2 tinycss2/tokenizer.py. It
// reports whether the given character is a name-start code point.
func isNameStart(css []rune, pos int) bool {
	// https://www.w3.org/TR/css-syntax-3/#name-start-code-point
	c := css[pos]
	return isASCIILetter(c) || c == '_' || c > 0x7F
}

// isIdentStart is _is_ident_start in tinycss2 tinycss2/tokenizer.py. It
// reports whether the given position is the start of a CSS identifier.
func isIdentStart(css []rune, pos int) bool {
	// https://drafts.csswg.org/css-syntax/#would-start-an-identifier
	switch {
	case isNameStart(css, pos):
		return true
	case css[pos] == '-':
		pos++
		// Name-start code point or hyphen:
		return (pos < len(css) && (isNameStart(css, pos) || css[pos] == '-')) ||
			// Valid escape:
			(startsWith(css, pos, `\`) && !startsWith(css, pos, "\\\n"))
	case css[pos] == '\\':
		return !startsWith(css, pos, "\\\n")
	}
	return false
}

// consumeIdent is _consume_ident in tinycss2 tinycss2/tokenizer.py. It
// returns the unescaped value and the new position. It assumes that pos
// starts at a valid identifier, as isIdentStart says.
func consumeIdent(css []rune, pos int) (string, int) {
	// http://dev.w3.org/csswg/css-syntax/#consume-a-name
	var chunks strings.Builder
	length := len(css)
	startPos := pos
	for pos < length {
		c := css[pos]
		if isNameChar(c) {
			pos++
		} else if c == '\\' && !startsWith(css, pos, "\\\n") {
			// Valid escape
			chunks.WriteString(string(css[startPos:pos]))
			var r rune
			r, pos = consumeEscape(css, pos+1)
			chunks.WriteRune(r)
			startPos = pos
		} else {
			break
		}
	}
	chunks.WriteString(string(css[startPos:pos]))
	return chunks.String(), pos
}

// consumeQuotedString is _consume_quoted_string in tinycss2
// tinycss2/tokenizer.py. It returns the unescaped value, false for the None
// of upstream, the new position and the error.
func consumeQuotedString(css []rune, pos int) (string, bool, int, *tokenError) {
	// https://drafts.csswg.org/css-syntax/#consume-a-string-token
	quote := css[pos]
	pos++
	var chunks strings.Builder
	length := len(css)
	startPos := pos
	for pos < length {
		c := css[pos]
		switch c {
		case quote:
			chunks.WriteString(string(css[startPos:pos]))
			pos++
			return chunks.String(), true, pos, nil
		case '\\':
			chunks.WriteString(string(css[startPos:pos]))
			pos++
			if pos < length {
				if css[pos] == '\n' { // Ignore escaped newlines
					pos++
				} else {
					var r rune
					r, pos = consumeEscape(css, pos)
					chunks.WriteRune(r)
				}
			}
			// else: Escaped EOF, do nothing
			startPos = pos
		case '\n': // Unescaped newline
			return "", false, pos, &tokenError{"bad-string", "Bad string token"}
		default:
			pos++
		}
	}
	chunks.WriteString(string(css[startPos:pos]))
	return chunks.String(), true, pos, &tokenError{kindEOFInString, "EOF in string"}
}

// consumeEscape is _consume_escape in tinycss2 tinycss2/tokenizer.py. It
// returns the unescaped character and the new position. It assumes a valid
// escape: pos is just after '\' and not followed by '\n'.
//
// Python gives a lone surrogate for an escape from U+D800 to U+DFFF. A Go
// string cannot hold one, so the port gives U+FFFD, as the CSS syntax says
// (D50).
func consumeEscape(css []rune, pos int) (rune, int) {
	// https://drafts.csswg.org/css-syntax/#consume-an-escaped-character
	// _HEX_ESCAPE_RE is ([0-9A-Fa-f]{1,6})[ \n\t]?
	end := pos
	for end < len(css) && end-pos < 6 && isHexDigit(css[end]) {
		end++
	}
	if end > pos {
		codepoint, _ := strconv.ParseUint(string(css[pos:end]), 16, 32)
		if end < len(css) && (css[end] == ' ' || css[end] == '\n' || css[end] == '\t') {
			end++
		}
		if codepoint > 0 && codepoint <= utf8.MaxRune && utf8.ValidRune(rune(codepoint)) {
			return rune(codepoint), end
		}
		return '�', end
	}
	if pos < len(css) {
		return css[pos], pos + 1
	}
	return '�', pos
}

// consumeURL is _consume_url in tinycss2 tinycss2/tokenizer.py. It returns
// the unescaped URL, false for the None of upstream, the new position and the
// error. The given pos is just after the '(' of 'url('.
func consumeURL(css []rune, pos int) (string, bool, int, *tokenError) {
	var err *tokenError
	length := len(css)
	// https://drafts.csswg.org/css-syntax/#consume-a-url-token
	// Skip whitespace
	for pos < length && (css[pos] == ' ' || css[pos] == '\n' || css[pos] == '\t') {
		pos++
	}
	if pos >= length { // EOF
		return "", true, pos, &tokenError{kindEOFInURL, messageEOFInURL}
	}
	var (
		value    string
		hasValue bool
	)
	c := css[pos]
	switch c {
	case '"', '\'':
		value, hasValue, pos, err = consumeQuotedString(css, pos)
	case ')':
		return "", true, pos + 1, err
	default:
		var chunks strings.Builder
		startPos := pos
	loop:
		for {
			if pos >= length { // EOF
				chunks.WriteString(string(css[startPos:pos]))
				return chunks.String(), true, pos, &tokenError{kindEOFInURL, messageEOFInURL}
			}
			c = css[pos]
			switch {
			case c == ')':
				chunks.WriteString(string(css[startPos:pos]))
				pos++
				return chunks.String(), true, pos, err
			case c == ' ' || c == '\n' || c == '\t':
				chunks.WriteString(string(css[startPos:pos]))
				value, hasValue = chunks.String(), true
				pos++
				break loop
			case c == '\\' && !startsWith(css, pos, "\\\n"):
				// Valid escape
				chunks.WriteString(string(css[startPos:pos]))
				var r rune
				r, pos = consumeEscape(css, pos+1)
				chunks.WriteRune(r)
				startPos = pos
			case c == '"' || c == '\'' || c == '(' ||
				// https://drafts.csswg.org/css-syntax/#non-printable-character
				(c <= 0x08 || c == 0x0b || (c >= 0x0e && c <= 0x1f) || c == 0x7f):
				hasValue = false // Parse error
				pos++
				break loop
			default:
				pos++
			}
		}
	}

	if hasValue {
		for pos < length && (css[pos] == ' ' || css[pos] == '\n' || css[pos] == '\t') {
			pos++
		}
		if pos < length {
			if css[pos] == ')' {
				return value, true, pos + 1, err
			}
		} else {
			if err == nil {
				err = &tokenError{kindEOFInURL, messageEOFInURL}
			}
			return value, true, pos, err
		}
	}

	// https://drafts.csswg.org/css-syntax/#consume-the-remnants-of-a-bad-url0
	for pos < length {
		if startsWith(css, pos, `\)`) {
			pos += 2
		} else if css[pos] == ')' {
			pos++
			break
		} else {
			pos++
		}
	}
	return "", false, pos, &tokenError{"bad-url", "bad URL token"}
}

// consumeUnicodeRange is _consume_unicode_range in tinycss2
// tinycss2/tokenizer.py. It returns the range and the new position. The
// given pos is just after the '+' of 'U+' or 'u+'.
func consumeUnicodeRange(css []rune, pos int) (int, int, int) {
	// https://drafts.csswg.org/css-syntax/#consume-a-unicode-range-token
	length := len(css)
	startPos := pos
	maxPos := min(pos+6, length)
	for pos < maxPos && isHexDigit(css[pos]) {
		pos++
	}
	start := string(css[startPos:pos])

	startPos = pos
	// Same maxPos as before: total of hex digits and question marks <= 6
	for pos < maxPos && css[pos] == '?' {
		pos++
	}
	questionMarks := pos - startPos

	var end string
	switch {
	case questionMarks > 0:
		end = start + strings.Repeat("F", questionMarks)
		start += strings.Repeat("0", questionMarks)
	case pos+1 < length && css[pos] == '-' && isHexDigit(css[pos+1]):
		pos++
		startPos = pos
		maxPos = min(pos+6, length)
		for pos < maxPos && isHexDigit(css[pos]) {
			pos++
		}
		end = string(css[startPos:pos])
	default:
		end = start
	}
	s, _ := strconv.ParseInt(start, 16, 64)
	e, _ := strconv.ParseInt(end, 16, 64)
	return int(s), int(e), pos
}

// startsWith is str.startswith of Python, with a start position.
func startsWith(css []rune, pos int, prefix string) bool {
	for _, r := range prefix {
		if pos >= len(css) || css[pos] != r {
			return false
		}
		pos++
	}
	return true
}

// indexRunes is str.find of Python, with a start position.
func indexRunes(css []rune, sub string, start int) int {
	s := []rune(sub)
	for i := start; i+len(s) <= len(css); i++ {
		if startsWith(css, i, sub) {
			return i
		}
	}
	return -1
}

// lastIndexRune is str.rfind of Python, for one character, between start
// and end.
func lastIndexRune(css []rune, r rune, start, end int) int {
	for i := end - 1; i >= start; i-- {
		if css[i] == r {
			return i
		}
	}
	return -1
}

// countRune is str.count of Python, for one character, between start and
// end.
func countRune(css []rune, r rune, start, end int) int {
	n := 0
	for i := start; i < end; i++ {
		if css[i] == r {
			n++
		}
	}
	return n
}

// trimLastRunes removes the last n code points of s, as s[:-n] of Python.
func trimLastRunes(s string, n int) string {
	for range n {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
	}
	return s
}
