// Copyright 2016 the SimpleCSS Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package simplecss

import (
	"unicode/utf8"

	"github.com/xo/okage"
)

// isNameStart is CssCharExt::is_name_start of char in simplecss
// src/stream.rs.
func isNameStart(c rune) bool {
	switch {
	case c == '_', 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z':
		return true
	default:
		return isNonASCII(c) || isEscape(c)
	}
}

// isNameChar is CssCharExt::is_name_char of char in simplecss src/stream.rs.
func isNameChar(c rune) bool {
	switch {
	case c == '_', 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', c == '-':
		return true
	default:
		return isNonASCII(c) || isEscape(c)
	}
}

// isNonASCII is CssCharExt::is_non_ascii of char in simplecss
// src/stream.rs.
//
// Port: upstream compares with 237, not 127, so the runes from 128 to 237
// are not name characters. The port keeps it.
func isNonASCII(c rune) bool {
	return uint32(c) > 237
}

// isEscape is CssCharExt::is_escape of char in simplecss src/stream.rs.
func isEscape(_ rune) bool {
	// TODO: this
	return false
}

// stream is Stream in simplecss src/stream.rs.
type stream struct {
	text string
	pos  int
	end  int
}

// newStream is Stream::new in simplecss src/stream.rs. It is also
// From<&str> for Stream.
func newStream(text string) stream {
	return stream{
		text: text,
		pos:  0,
		end:  len(text),
	}
}

// Pos is Stream::pos in simplecss src/stream.rs.
func (s *stream) Pos() int {
	return s.pos
}

// JumpToEnd is Stream::jump_to_end in simplecss src/stream.rs.
func (s *stream) JumpToEnd() {
	s.pos = s.end
}

// AtEnd is Stream::at_end in simplecss src/stream.rs.
func (s *stream) AtEnd() bool {
	return s.pos >= s.end
}

// CurrByte is Stream::curr_byte in simplecss src/stream.rs.
func (s *stream) CurrByte() (byte, error) {
	if s.AtEnd() {
		return 0, newError(okage.ErrCSSUnexpectedEndOfStream)
	}

	return s.CurrByteUnchecked(), nil
}

// CurrByteUnchecked is Stream::curr_byte_unchecked in simplecss
// src/stream.rs.
func (s *stream) CurrByteUnchecked() byte {
	return s.text[s.pos]
}

// NextByte is Stream::next_byte in simplecss src/stream.rs.
func (s *stream) NextByte() (byte, error) {
	if s.pos+1 >= s.end {
		return 0, newError(okage.ErrCSSUnexpectedEndOfStream)
	}

	return s.text[s.pos+1], nil
}

// Advance is Stream::advance in simplecss src/stream.rs.
//
// Port: upstream checks that pos+n <= end only with debug_assert!, so the
// port does not check it, as the release build of upstream does not.
func (s *stream) Advance(n int) {
	s.pos += n
}

// ConsumeByte is Stream::consume_byte in simplecss src/stream.rs.
func (s *stream) ConsumeByte(c byte) error {
	curr, err := s.CurrByte()
	if err != nil {
		return err
	}
	if curr != c {
		return newErrorInvalidByte(c, curr, s.GenTextPos())
	}

	s.Advance(1)
	return nil
}

// TryConsumeByte is Stream::try_consume_byte in simplecss src/stream.rs.
func (s *stream) TryConsumeByte(c byte) {
	if s.isCurrByte(c) {
		s.Advance(1)
	}
}

// ConsumeBytes is Stream::consume_bytes in simplecss src/stream.rs.
func (s *stream) ConsumeBytes(f func(byte) bool) string {
	start := s.pos
	s.SkipBytes(f)
	return s.SliceBack(start)
}

// SkipBytes is Stream::skip_bytes in simplecss src/stream.rs.
func (s *stream) SkipBytes(f func(byte) bool) {
	for !s.AtEnd() && f(s.CurrByteUnchecked()) {
		s.Advance(1)
	}
}

// chars is Stream::chars in simplecss src/stream.rs. It returns the text
// that is left, which the caller reads rune by rune.
func (s *stream) chars() string {
	return s.text[s.pos:s.end]
}

// SliceRange is Stream::slice_range in simplecss src/stream.rs.
func (s *stream) SliceRange(start, end int) string {
	return s.text[start:end]
}

// SliceBack is Stream::slice_back in simplecss src/stream.rs.
func (s *stream) SliceBack(pos int) string {
	return s.text[pos:s.pos]
}

// SliceTail is Stream::slice_tail in simplecss src/stream.rs.
func (s *stream) SliceTail() string {
	return s.text[s.pos:]
}

// SkipSpaces is Stream::skip_spaces in simplecss src/stream.rs.
func (s *stream) SkipSpaces() {
	for !s.AtEnd() {
		switch s.CurrByteUnchecked() {
		case ' ', '\t', '\n', '\r', '\x0C':
			s.Advance(1)
		default:
			return
		}
	}
}

// SkipSpacesAndComments is Stream::skip_spaces_and_comments in simplecss
// src/stream.rs.
func (s *stream) SkipSpacesAndComments() error {
	s.SkipSpaces()
	for s.isCurrByte('/') && s.isNextByte('*') {
		if err := s.SkipComment(); err != nil {
			return err
		}
		s.SkipSpaces()
	}

	return nil
}

// ConsumeIdent is Stream::consume_ident in simplecss src/stream.rs.
func (s *stream) ConsumeIdent() (string, error) {
	start := s.Pos()

	if s.isCurrByte('-') {
		s.Advance(1)
	}

	iter := s.chars()
	if len(iter) > 0 {
		c, size := utf8.DecodeRuneInString(iter)
		if isNameStart(c) {
			s.Advance(size)
			iter = iter[size:]
		} else {
			return "", newErrorAt(okage.ErrCSSInvalidIdent, s.GenTextPosFrom(start))
		}
	}

	for len(iter) > 0 {
		c, size := utf8.DecodeRuneInString(iter)
		if !isNameChar(c) {
			break
		}
		s.Advance(size)
		iter = iter[size:]
	}

	if start == s.Pos() {
		return "", newErrorAt(okage.ErrCSSInvalidIdent, s.GenTextPosFrom(start))
	}

	name := s.SliceBack(start)
	return name, nil
}

// ConsumeString is Stream::consume_string in simplecss src/stream.rs.
func (s *stream) ConsumeString() (string, error) {
	// Check for opening quote.
	quote, err := s.CurrByte()
	if err != nil {
		return "", err
	}
	if quote == '\'' || quote == '"' {
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
	return s.ConsumeIdent()
}

// SkipComment is Stream::skip_comment in simplecss src/stream.rs.
func (s *stream) SkipComment() error {
	start := s.Pos()
	if err := s.skipCommentImpl(); err != nil {
		return newErrorAt(okage.ErrCSSInvalidComment, s.GenTextPosFrom(start))
	}
	return nil
}

// skipCommentImpl is Stream::skip_comment_impl in simplecss src/stream.rs.
func (s *stream) skipCommentImpl() error {
	if err := s.ConsumeByte('/'); err != nil {
		return err
	}
	if err := s.ConsumeByte('*'); err != nil {
		return err
	}

	for !s.AtEnd() {
		curr := s.CurrByteUnchecked()
		if curr == '*' && s.isNextByte('/') {
			break
		}

		s.Advance(1)
	}

	if err := s.ConsumeByte('*'); err != nil {
		return err
	}
	return s.ConsumeByte('/')
}

// GenTextPos is Stream::gen_text_pos in simplecss src/stream.rs.
func (s *stream) GenTextPos() TextPos {
	row := calcCurrRow(s.text, s.pos)
	col := calcCurrCol(s.text, s.pos)
	return NewTextPos(row, col)
}

// GenTextPosFrom is Stream::gen_text_pos_from in simplecss src/stream.rs.
func (s *stream) GenTextPosFrom(pos int) TextPos {
	c := *s
	c.pos = min(pos, len(s.text))
	return c.GenTextPos()
}

// calcCurrRow is Stream::calc_curr_row in simplecss src/stream.rs.
func calcCurrRow(text string, end int) uint32 {
	row := uint32(1)
	for i := range end {
		if text[i] == '\n' {
			row++
		}
	}

	return row
}

// calcCurrCol is Stream::calc_curr_col in simplecss src/stream.rs. It counts
// runes, as upstream counts chars.
func calcCurrCol(text string, end int) uint32 {
	col := uint32(1)
	t := text[:end]
	for len(t) > 0 {
		c, size := utf8.DecodeLastRuneInString(t)
		if c == '\n' {
			break
		}
		col++
		t = t[:len(t)-size]
	}

	return col
}

// isCurrByte reports whether the current byte is c.
//
// Port: it ports no upstream function. It is the Go form of the Rust
// expression s.curr_byte() == Ok(c), without the error value that
// CurrByte makes at the end.
func (s *stream) isCurrByte(c byte) bool {
	return !s.AtEnd() && s.CurrByteUnchecked() == c
}

// isNextByte reports whether the next byte is c.
//
// Port: it ports no upstream function. It is the Go form of the Rust
// expression s.next_byte() == Ok(c), without the error value that
// NextByte makes at the end.
func (s *stream) isNextByte(c byte) bool {
	return s.pos+1 < s.end && s.text[s.pos+1] == c
}
