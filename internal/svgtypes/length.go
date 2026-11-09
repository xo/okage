// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/length.rs.

import (
	"iter"

	"github.com/xo/okage"
)

// LengthUnit is LengthUnit in svgtypes src/length.rs.
//
// List of all SVG length units.
type LengthUnit uint8

// The values of LengthUnit.
const (
	LengthUnitNone LengthUnit = iota
	LengthUnitEm
	LengthUnitEx
	LengthUnitPx
	LengthUnitIn
	LengthUnitCm
	LengthUnitMm
	LengthUnitPt
	LengthUnitPc
	LengthUnitPercent
)

// lengthUnitNames holds the Rust name of each unit of LengthUnit.
var lengthUnitNames = [...]string{
	LengthUnitNone:    "None", //nolint:goconst // the Rust name of the case
	LengthUnitEm:      "Em",
	LengthUnitEx:      "Ex",
	LengthUnitPx:      "Px",
	LengthUnitIn:      "In",
	LengthUnitCm:      "Cm",
	LengthUnitMm:      "Mm",
	LengthUnitPt:      "Pt",
	LengthUnitPc:      "Pc",
	LengthUnitPercent: "Percent",
}

// String returns the Rust name of the unit.
func (u LengthUnit) String() string {
	if int(u) < len(lengthUnitNames) {
		return lengthUnitNames[u]
	}
	return "LengthUnit(?)"
}

// Length is Length in svgtypes src/length.rs.
//
// Representation of the [`<length>`] type.
//
// [`<length>`]: https://www.w3.org/TR/SVG2/types.html#InterfaceSVGLength
type Length struct {
	Number float64
	Unit   LengthUnit
}

// NewLength is Length::new in svgtypes src/length.rs.
//
// Constructs a new length.
func NewLength(number float64, unit LengthUnit) Length {
	return Length{Number: number, Unit: unit}
}

// NewLengthNumber is Length::new_number in svgtypes src/length.rs.
//
// Constructs a new length with `LengthUnit::None`.
func NewLengthNumber(number float64) Length {
	return Length{
		Number: number,
		Unit:   LengthUnitNone,
	}
}

// LengthZero is Length::zero in svgtypes src/length.rs.
//
// Constructs a new length with a zero number.
//
// Shorthand for: `Length::new(0.0, Unit::None)`.
func LengthZero() Length {
	return Length{
		Number: 0.0,
		Unit:   LengthUnitNone,
	}
}

// DefaultLength is the impl Default for Length in svgtypes src/length.rs.
func DefaultLength() Length {
	return LengthZero()
}

// ParseLength is the impl FromStr for Length in svgtypes src/length.rs.
func ParseLength(text string) (Length, error) {
	s := NewStream(text)
	l, err := s.ParseLength()
	if err != nil {
		return Length{}, err
	}

	if !s.AtEnd() {
		return Length{}, errUnexpectedData(s.CalcCharPos())
	}

	return NewLength(l.Number, l.Unit), nil
}

// ParseLength is Stream::parse_length in svgtypes src/length.rs.
//
// Parses length from the stream.
//
// <https://www.w3.org/TR/SVG2/types.html#InterfaceSVGLength>
//
// # Notes
//
//   - Suffix must be lowercase, otherwise it will be an error.
func (s *Stream) ParseLength() (Length, error) {
	s.SkipSpaces()

	n, err := s.ParseNumber()
	if err != nil {
		return Length{}, err
	}

	if s.AtEnd() {
		return NewLength(n, LengthUnitNone), nil
	}

	var u LengthUnit
	switch {
	case s.StartsWith("%"):
		u = LengthUnitPercent
	case s.StartsWith("em"):
		u = LengthUnitEm
	case s.StartsWith("ex"):
		u = LengthUnitEx
	case s.StartsWith("px"):
		u = LengthUnitPx
	case s.StartsWith("in"):
		u = LengthUnitIn
	case s.StartsWith("cm"):
		u = LengthUnitCm
	case s.StartsWith("mm"):
		u = LengthUnitMm
	case s.StartsWith("pt"):
		u = LengthUnitPt
	case s.StartsWith("pc"):
		u = LengthUnitPc
	default:
		u = LengthUnitNone
	}

	switch u {
	case LengthUnitPercent:
		s.Advance(1)
	case LengthUnitNone:
	default:
		s.Advance(2)
	}

	return NewLength(n, u), nil
}

// ParseListLength is Stream::parse_list_length in svgtypes src/length.rs.
//
// Parses length from a list of lengths.
func (s *Stream) ParseListLength() (Length, error) {
	if s.AtEnd() {
		return Length{}, okage.ErrSVGTypesUnexpectedEndOfStream
	}

	l, err := s.ParseLength()
	if err != nil {
		return Length{}, err
	}
	s.SkipSpaces()
	s.parseListSeparator()
	return l, nil
}

// LengthListParser is LengthListParser in svgtypes src/length.rs.
//
// A pull-based [`<list-of-length>`] parser.
//
// [`<list-of-length>`]: https://www.w3.org/TR/SVG2/types.html#InterfaceSVGLengthList
type LengthListParser struct {
	s Stream
}

// NewLengthListParser is the impl From<&str> for LengthListParser in
// svgtypes src/length.rs.
func NewLengthListParser(v string) *LengthListParser {
	return &LengthListParser{s: NewStream(v)}
}

// Next is Iterator::next for LengthListParser in svgtypes src/length.rs.
// ok is false at the end, where Rust returns None. When ok is true, the
// value and err are the Result of Rust.
func (p *LengthListParser) Next() (Length, bool, error) {
	if p.s.AtEnd() {
		return Length{}, false, nil
	}
	v, err := p.s.ParseListLength()
	if err != nil {
		p.s.JumpToEnd()
	}

	return v, true, err
}

// All returns an iterator over the results of Next, as the Iterator impl of
// LengthListParser in svgtypes src/length.rs gives them.
func (p *LengthListParser) All() iter.Seq2[Length, error] {
	return func(yield func(Length, error) bool) {
		for {
			v, ok, err := p.Next()
			if !ok || !yield(v, err) {
				return
			}
		}
	}
}
