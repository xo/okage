// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/stream.rs.

import (
	"iter"
	"strings"
	"unicode/utf8"

	"github.com/xo/okage"
)

// The functions isSign to isASCIIIdent port the trait ByteExt in svgtypes
// src/stream.rs, which upstream implements for u8. Go cannot add a method to
// byte, so each method is a function.
//
// Extension methods for XML-subset only operations.

// isSign is ByteExt::is_sign in svgtypes src/stream.rs.
//
// Checks if a byte is a numeric sign.
func isSign(c byte) bool {
	return c == '+' || c == '-'
}

// isDigit is ByteExt::is_digit in svgtypes src/stream.rs.
//
// Checks if a byte is a digit.
//
// It accepts `[0-9]`.
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// isHexDigit is ByteExt::is_hex_digit in svgtypes src/stream.rs.
//
// Checks if a byte is a hex digit.
//
// It accepts `[0-9A-Fa-f]`.
func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}

// isSpace is ByteExt::is_space in svgtypes src/stream.rs.
//
// Checks if a byte is a space.
//
// It accepts `[ \r\n\t]`.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// isASCIIIdent is ByteExt::is_ascii_ident in svgtypes src/stream.rs.
//
// Checks if a byte is an ASCII ident char.
func isASCIIIdent(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '-' || c == '_'
}

// The functions isNameStart to isEscape port the private trait CharExt in
// svgtypes src/stream.rs, which upstream implements for char.

// isNameStart is CharExt::is_name_start in svgtypes src/stream.rs.
func isNameStart(c rune) bool {
	switch {
	case c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z'):
		return true
	default:
		return isNonASCIIIdent(c) || isEscape(c)
	}
}

// isNameChar is CharExt::is_name_char in svgtypes src/stream.rs.
func isNameChar(c rune) bool {
	switch {
	case c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-':
		return true
	default:
		return isNonASCIIIdent(c) || isEscape(c)
	}
}

// isNonASCIIIdent is CharExt::is_non_ascii_ident in svgtypes src/stream.rs.
// It reports whether c is a non-ASCII ident code point of CSS:
// <https://www.w3.org/TR/css-syntax-3/#non-ascii-ident-code-point>
func isNonASCIIIdent(c rune) bool {
	switch {
	case c == 0xB7,
		c >= 0xC0 && c <= 0xD6,
		c >= 0xD8 && c <= 0xF6,
		c >= 0xF8 && c <= 0x37D,
		c >= 0x37F && c <= 0x1FFF,
		c == 0x200C,
		c == 0x200D,
		c == 0x203F,
		c == 0x2040,
		c >= 0x2070 && c <= 0x218F,
		c >= 0x2C00 && c <= 0x2FEF,
		c >= 0x3001 && c <= 0xD7FF,
		c >= 0xF900 && c <= 0xFDCF,
		c >= 0xFDF0 && c <= 0xFFFD,
		c >= 0x10000:
		return true
	}
	return false
}

// isEscape is CharExt::is_escape in svgtypes src/stream.rs.
func isEscape(_ rune) bool {
	// TODO: this
	return false
}

// Stream is Stream in svgtypes src/stream.rs.
//
// A streaming text parsing interface.
//
// A Stream is a value, as the Rust type is Copy. A copy parses on from the
// same position without changing the original.
type Stream struct {
	text string
	pos  int
}

// NewStream is the impl From<&str> for Stream in svgtypes src/stream.rs.
func NewStream(text string) Stream {
	return Stream{text: text, pos: 0}
}

// Pos is Stream::pos in svgtypes src/stream.rs.
//
// Returns the current position in bytes.
func (s *Stream) Pos() int {
	return s.pos
}

// CalcCharPos is Stream::calc_char_pos in svgtypes src/stream.rs.
//
// Calculates the current position in chars.
func (s *Stream) CalcCharPos() int {
	return s.CalcCharPosAt(s.pos)
}

// CalcCharPosAt is Stream::calc_char_pos_at in svgtypes src/stream.rs.
//
// Calculates the current position in chars.
func (s *Stream) CalcCharPosAt(bytePos int) int {
	pos := 1
	for idx := range s.text {
		if idx >= bytePos {
			break
		}

		pos++
	}

	return pos
}

// JumpToEnd is Stream::jump_to_end in svgtypes src/stream.rs.
//
// Sets current position equal to the end.
//
// Used to indicate end of parsing on error.
func (s *Stream) JumpToEnd() {
	s.pos = len(s.text)
}

// AtEnd is Stream::at_end in svgtypes src/stream.rs.
//
// Checks if the stream is reached the end.
//
// Any [Stream.Pos] value larger than original text length indicates stream end.
//
// Accessing stream after reaching end via safe methods will produce
// an `UnexpectedEndOfStream` error.
//
// Accessing stream after reaching end via *_unchecked methods will produce
// a bound checking panic.
func (s *Stream) AtEnd() bool {
	return s.pos >= len(s.text)
}

// CurrByte is Stream::curr_byte in svgtypes src/stream.rs.
//
// Returns a byte from a current stream position.
//
// # Errors
//
//   - `UnexpectedEndOfStream`
func (s *Stream) CurrByte() (byte, error) {
	if s.AtEnd() {
		return 0, okage.ErrSVGTypesUnexpectedEndOfStream
	}

	return s.CurrByteUnchecked(), nil
}

// Chars is Stream::chars in svgtypes src/stream.rs. It gives the chars from
// the current position to the end.
func (s *Stream) Chars() iter.Seq[rune] {
	tail := s.text[s.pos:]
	return func(yield func(rune) bool) {
		for _, c := range tail {
			if !yield(c) {
				return
			}
		}
	}
}

// CurrByteUnchecked is Stream::curr_byte_unchecked in svgtypes
// src/stream.rs.
//
// Returns a byte from a current stream position.
//
// # Panics
//
//   - if the current position is after the end of the data
func (s *Stream) CurrByteUnchecked() byte {
	return s.text[s.pos]
}

// IsCurrByteEq is Stream::is_curr_byte_eq in svgtypes src/stream.rs.
//
// Checks that current byte is equal to provided.
//
// Returns `false` if no bytes left.
func (s *Stream) IsCurrByteEq(c byte) bool {
	if !s.AtEnd() {
		return s.CurrByteUnchecked() == c
	}
	return false
}

// NextByte is Stream::next_byte in svgtypes src/stream.rs.
//
// Returns a next byte from a current stream position.
//
// # Errors
//
//   - `UnexpectedEndOfStream`
func (s *Stream) NextByte() (byte, error) {
	if s.pos+1 >= len(s.text) {
		return 0, okage.ErrSVGTypesUnexpectedEndOfStream
	}

	return s.text[s.pos+1], nil
}

// Advance is Stream::advance in svgtypes src/stream.rs.
//
// Advances by `n` bytes.
func (s *Stream) Advance(n int) {
	s.pos += n
}

// SkipSpaces is Stream::skip_spaces in svgtypes src/stream.rs.
//
// Skips whitespaces.
//
// Accepted values: `' ' \n \r \t`.
func (s *Stream) SkipSpaces() {
	for !s.AtEnd() && isSpace(s.CurrByteUnchecked()) {
		s.Advance(1)
	}
}

// StartsWith is Stream::starts_with in svgtypes src/stream.rs.
//
// Checks that the stream starts with a selected text.
func (s *Stream) StartsWith(text string) bool {
	return strings.HasPrefix(s.text[s.pos:], text)
}

// ConsumeByte is Stream::consume_byte in svgtypes src/stream.rs.
//
// Consumes current byte if it's equal to the provided byte.
//
// # Errors
//
//   - `InvalidChar`
//   - `UnexpectedEndOfStream`
func (s *Stream) ConsumeByte(c byte) error {
	b, err := s.CurrByte()
	if err != nil {
		return err
	}
	if b != c {
		return errInvalidChar(
			[]byte{s.CurrByteUnchecked(), c},
			s.CalcCharPos(),
		)
	}

	s.Advance(1)
	return nil
}

// ParseIdent is Stream::parse_ident in svgtypes src/stream.rs.
//
// Parses a single [ident](https://drafts.csswg.org/css-syntax-3/#typedef-ident-token).
//
// # Errors
//
//   - `InvalidIdent`
func (s *Stream) ParseIdent() (string, error) {
	start := s.Pos()

	if b, err := s.CurrByte(); err == nil && b == '-' {
		s.Advance(1)
	}

	tail := s.text[s.pos:]
	if len(tail) > 0 {
		c, size := utf8.DecodeRuneInString(tail)
		if isNameStart(c) {
			s.Advance(size)
		} else {
			return "", okage.ErrSVGTypesInvalidIdent
		}
		tail = tail[size:]
	}

	for _, c := range tail {
		if isNameChar(c) {
			s.Advance(utf8.RuneLen(c))
		} else {
			break
		}
	}

	if start == s.Pos() {
		return "", okage.ErrSVGTypesInvalidIdent
	}

	name := s.SliceBack(start)
	return name, nil
}

// ConsumeASCIIIdent is Stream::consume_ascii_ident in svgtypes
// src/stream.rs.
//
// Consumes a single ident consisting of ASCII characters, if available.
func (s *Stream) ConsumeASCIIIdent() string {
	start := s.pos
	s.SkipBytes(func(_ *Stream, c byte) bool { return isASCIIIdent(c) })
	return s.SliceBack(start)
}

// ParseQuotedString is Stream::parse_quoted_string in svgtypes
// src/stream.rs.
//
// Parses a single [quoted string](https://drafts.csswg.org/css-syntax-3/#typedef-string-token)
//
// # Errors
//
//   - `UnexpectedEndOfStream`
//   - `InvalidValue`
func (s *Stream) ParseQuotedString() (string, error) {
	// Check for opening quote.
	quote, err := s.CurrByte()
	if err != nil {
		return "", err
	}

	if quote != '\'' && quote != '"' {
		return "", okage.ErrSVGTypesInvalidValue
	}

	prev := quote
	s.Advance(1)

	start := s.Pos()

	for !s.AtEnd() {
		curr := s.CurrByteUnchecked()

		// Advance until the closing quote.
		if curr == quote {
			// Check for escaped quote.
			if prev != '\\' {
				break
			}
		}

		prev = curr
		s.Advance(1)
	}

	value := s.SliceBack(start)

	// Check for closing quote.
	if err := s.ConsumeByte(quote); err != nil {
		return "", err
	}

	return value, nil
}

// ConsumeString is Stream::consume_string in svgtypes src/stream.rs.
//
// Consumes selected string.
//
// # Errors
//
//   - `InvalidChar`
//   - `UnexpectedEndOfStream`
func (s *Stream) ConsumeString(text string) error {
	if s.AtEnd() {
		return okage.ErrSVGTypesUnexpectedEndOfStream
	}

	if !s.StartsWith(text) {
		n := min(len(text), len(s.text)-s.pos)
		// Collect chars and do not slice a string,
		// because the `len` can be on the char boundary.
		// Which lead to a panic.
		var actual strings.Builder
		i := 0
		for _, c := range s.text[s.pos:] {
			if i == n {
				break
			}
			actual.WriteRune(c)
			i++
		}

		expected := text

		return errInvalidString(
			[]string{actual.String(), expected},
			s.CalcCharPos(),
		)
	}

	s.Advance(len(text))
	return nil
}

// ConsumeBytes is Stream::consume_bytes in svgtypes src/stream.rs.
//
// Consumes bytes by the predicate and returns them.
//
// The result can be empty.
func (s *Stream) ConsumeBytes(f func(s *Stream, c byte) bool) string {
	start := s.Pos()
	s.SkipBytes(f)
	return s.SliceBack(start)
}

// SkipBytes is Stream::skip_bytes in svgtypes src/stream.rs.
//
// Consumes bytes by the predicate.
func (s *Stream) SkipBytes(f func(s *Stream, c byte) bool) {
	for !s.AtEnd() {
		c := s.CurrByteUnchecked()
		if f(s, c) {
			s.Advance(1)
		} else {
			break
		}
	}
}

// SliceBack is Stream::slice_back in svgtypes src/stream.rs.
//
// Slices data from `pos` to the current position.
func (s *Stream) SliceBack(pos int) string {
	return s.text[pos:s.pos]
}

// SliceTail is Stream::slice_tail in svgtypes src/stream.rs.
//
// Slices data from the current position to the end.
func (s *Stream) SliceTail() string {
	return s.text[s.pos:]
}

// ParseNumberOrPercent is Stream::parse_number_or_percent in svgtypes
// src/stream.rs.
//
// Parses number or percent from the stream.
//
// Percent value will be normalized.
func (s *Stream) ParseNumberOrPercent() (float64, error) {
	s.SkipSpaces()

	n, err := s.ParseNumber()
	if err != nil {
		return 0, err
	}
	if s.StartsWith("%") {
		s.Advance(1)
		return n / 100.0, nil
	}
	return n, nil
}

// ParseListNumberOrPercent is Stream::parse_list_number_or_percent in
// svgtypes src/stream.rs.
//
// Parses number or percent from a list of numbers and/or percents.
func (s *Stream) ParseListNumberOrPercent() (float64, error) {
	if s.AtEnd() {
		return 0, okage.ErrSVGTypesUnexpectedEndOfStream
	}

	l, err := s.ParseNumberOrPercent()
	if err != nil {
		return 0, err
	}
	s.SkipSpaces()
	s.parseListSeparator()
	return l, nil
}

// SkipDigits is Stream::skip_digits in svgtypes src/stream.rs.
//
// Skips digits.
func (s *Stream) SkipDigits() {
	s.SkipBytes(func(_ *Stream, c byte) bool { return isDigit(c) })
}

// parseListSeparator is Stream::parse_list_separator in svgtypes
// src/stream.rs.
func (s *Stream) parseListSeparator() {
	if s.IsCurrByteEq(',') {
		s.Advance(1)
	}
}
