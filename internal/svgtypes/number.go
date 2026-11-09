// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/number.rs.

import (
	"iter"
	"math"
	"strconv"

	"github.com/xo/okage"
)

// Number is Number in svgtypes src/number.rs.
//
// An [SVG number](https://www.w3.org/TR/SVG2/types.html#InterfaceSVGNumber).
type Number float64

// ParseNumber is the impl FromStr for Number in svgtypes src/number.rs.
func ParseNumber(text string) (Number, error) {
	s := NewStream(text)
	n, err := s.ParseNumber()
	if err != nil {
		return 0, err
	}
	s.SkipSpaces()
	if !s.AtEnd() {
		return 0, errUnexpectedData(s.CalcCharPos())
	}

	return Number(n), nil
}

// ParseNumber is Stream::parse_number in svgtypes src/number.rs.
//
// Parses number from the stream.
//
// This method will detect a number length and then
// will pass a substring to the `f64::from_str` method.
//
// <https://www.w3.org/TR/SVG2/types.html#InterfaceSVGNumber>
//
// # Errors
//
// Returns only `InvalidNumber`.
func (s *Stream) ParseNumber() (float64, error) {
	// Strip off leading whitespaces.
	s.SkipSpaces()

	start := s.Pos()

	if s.AtEnd() {
		return 0, errInvalidNumber(s.CalcCharPosAt(start))
	}

	n, err := s.parseNumberImpl()
	if err != nil {
		return 0, errInvalidNumber(s.CalcCharPosAt(start))
	}
	return n, nil
}

// parseNumberImpl is Stream::parse_number_impl in svgtypes src/number.rs.
func (s *Stream) parseNumberImpl() (float64, error) {
	start := s.Pos()

	c, err := s.CurrByte()
	if err != nil {
		return 0, err
	}

	// Consume sign.
	if isSign(c) {
		s.Advance(1)
		c, err = s.CurrByte()
		if err != nil {
			return 0, err
		}
	}

	// Consume integer.
	switch {
	case c >= '0' && c <= '9':
		s.SkipDigits()
	case c == '.':
	default:
		return 0, errInvalidNumber(0)
	}

	// Consume fraction.
	if c, err := s.CurrByte(); err == nil && c == '.' {
		s.Advance(1)
		s.SkipDigits()
	}

	if c, err := s.CurrByte(); err == nil {
		if c == 'e' || c == 'E' {
			c2, err := s.NextByte()
			if err != nil {
				return 0, err
			}
			// Check for `em`/`ex`.
			if c2 != 'm' && c2 != 'x' {
				s.Advance(1)

				c, err := s.CurrByte()
				if err != nil {
					return 0, err
				}
				switch {
				case c == '+' || c == '-':
					s.Advance(1)
					s.SkipDigits()
				case c >= '0' && c <= '9':
					s.SkipDigits()
				default:
					return 0, errInvalidNumber(0)
				}
			}
		}
	}

	str := s.SliceBack(start)

	// Use the default f64 parser now.
	if n, ok := parseF64(str); ok {
		// inf, nan, etc. are an error.
		if !math.IsInf(n, 0) && !math.IsNaN(n) {
			return n, nil
		}
	}

	return 0, errInvalidNumber(0)
}

// parseF64 is `f64::from_str` of Rust, for the text that parseNumberImpl
// scans: an optional sign, digits, an optional fraction and an optional
// exponent. Both parsers round correctly, so they give the same f64.
//
// strconv.ParseFloat accepts more forms than Rust: hex floats, underscores,
// "inf", "infinity" and "nan". None of them can reach it here, because the
// text holds only signs, digits, '.', 'e' and 'E'. For the forms that reach
// it, the two parsers fail on the same texts, such as ".", "-." and "1e+".
// An overflow gives an infinity and the error ErrRange in Go, and an
// infinity in Rust. parseF64 returns the infinity, and the caller rejects it.
// An underflow gives zero, with no error, in both.
func parseF64(str string) (float64, bool) {
	n, err := strconv.ParseFloat(str, 64)
	if err != nil && !math.IsInf(n, 0) {
		return 0, false
	}
	return n, true
}

// ParseListNumber is Stream::parse_list_number in svgtypes src/number.rs.
//
// Parses number from a list of numbers.
func (s *Stream) ParseListNumber() (float64, error) {
	if s.AtEnd() {
		return 0, okage.ErrSVGTypesUnexpectedEndOfStream
	}

	n, err := s.ParseNumber()
	if err != nil {
		return 0, err
	}
	s.SkipSpaces()
	s.parseListSeparator()
	return n, nil
}

// NumberListParser is NumberListParser in svgtypes src/number.rs.
//
// A pull-based [`<list-of-numbers>`] parser.
//
// [`<list-of-numbers>`]: https://www.w3.org/TR/SVG2/types.html#InterfaceSVGNumberList
type NumberListParser struct {
	s Stream
}

// NewNumberListParser is the impl From<&str> for NumberListParser in
// svgtypes src/number.rs.
func NewNumberListParser(v string) *NumberListParser {
	return &NumberListParser{s: NewStream(v)}
}

// Next is Iterator::next for NumberListParser in svgtypes src/number.rs.
// ok is false at the end, where Rust returns None. When ok is true, the
// value and err are the Result of Rust.
func (p *NumberListParser) Next() (float64, bool, error) {
	if p.s.AtEnd() {
		return 0, false, nil
	}
	v, err := p.s.ParseListNumber()
	if err != nil {
		p.s.JumpToEnd()
	}

	return v, true, err
}

// All returns an iterator over the results of Next, as the Iterator impl of
// NumberListParser in svgtypes src/number.rs gives them.
func (p *NumberListParser) All() iter.Seq2[float64, error] {
	return func(yield func(float64, error) bool) {
		for {
			v, ok, err := p.Next()
			if !ok || !yield(v, err) {
				return
			}
		}
	}
}
