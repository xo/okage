// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/filter_functions.rs. The enum
// FilterValueListParserError becomes the sentinel errors
// ErrSVGTypesFilter* of the root package, wrapped with the position (D21).
// Its case StreamErrors holds an Error of src/error.rs, and the port returns
// that error as it is, as the Display of upstream does.

import (
	"fmt"
	"iter"
	"math"

	"github.com/xo/okage"
)

// FilterValueKind is the case of the enum FilterValue in svgtypes
// src/filter_functions.rs (D27).
type FilterValueKind uint8

// The cases of FilterValue.
const (
	// FilterValueBlur is FilterValue::Blur.
	//
	// Cannot be negative and/or have a percentage units.
	FilterValueBlur FilterValueKind = iota
	FilterValueDropShadow
	// FilterValueBrightness is FilterValue::Brightness.
	//
	// Normalized value. Cannot be negative.
	FilterValueBrightness
	// FilterValueContrast is FilterValue::Contrast.
	//
	// Normalized value. Cannot be negative.
	FilterValueContrast
	// FilterValueGrayscale is FilterValue::Grayscale.
	//
	// Normalized value. Cannot be negative.
	FilterValueGrayscale
	FilterValueHueRotate
	// FilterValueInvert is FilterValue::Invert.
	//
	// Normalized value. Cannot be negative.
	FilterValueInvert
	// FilterValueOpacity is FilterValue::Opacity.
	//
	// Normalized value. Cannot be negative.
	FilterValueOpacity
	// FilterValueSepia is FilterValue::Sepia.
	//
	// Normalized value. Cannot be negative.
	FilterValueSepia
	// FilterValueSaturate is FilterValue::Saturate.
	//
	// Normalized value. Cannot be negative.
	FilterValueSaturate
	// FilterValueURL is FilterValue::Url.
	//
	// Cannot be empty.
	FilterValueURL
)

// filterValueKindNames holds the Rust name of each case of FilterValueKind.
var filterValueKindNames = [...]string{
	FilterValueBlur:       "Blur",
	FilterValueDropShadow: "DropShadow",
	FilterValueBrightness: "Brightness",
	FilterValueContrast:   "Contrast",
	FilterValueGrayscale:  "Grayscale",
	FilterValueHueRotate:  "HueRotate",
	FilterValueInvert:     "Invert",
	FilterValueOpacity:    "Opacity",
	FilterValueSepia:      "Sepia",
	FilterValueSaturate:   "Saturate",
	FilterValueURL:        "Url",
}

// String returns the Rust name of the case.
func (k FilterValueKind) String() string {
	if int(k) < len(filterValueKindNames) {
		return filterValueKindNames[k]
	}
	return "FilterValueKind(?)"
}

// FilterValue is FilterValue in svgtypes src/filter_functions.rs. The cases
// hold different data and the code calls no method on them, so the enum is a
// struct with a kind and the fields of every case (D27). A field that a case
// does not have is zero.
//
//   - Blur: Length.
//   - DropShadow: Color and HasColor, which is false where the Option of
//     Rust is None, Dx, Dy and StdDev.
//   - Brightness, Contrast, Grayscale, Invert, Opacity, Sepia and Saturate:
//     Number.
//   - HueRotate: Angle.
//   - Url: URL.
//
// Representation of the [`<filter-function>`] | [`<url>`] type.
//
// Note that [Length] values in this enum do not contain % values.
// They are disallowed by the spec.
//
// [`<filter-function>`]: https://www.w3.org/TR/filter-effects-1/#filter-functions
// [`<url>`]: https://www.w3.org/TR/filter-effects-1/#typedef-filter-url
type FilterValue struct {
	Kind FilterValueKind

	Length Length

	// `currentColor` will be resolved as `None`,
	// because it should be processed in the same way.
	Color    Color
	HasColor bool
	// Cannot have a percentage units.
	Dx Length
	// Cannot have a percentage units.
	Dy Length
	// Cannot be negative and/or have a percentage units.
	StdDev Length

	Number float64

	Angle Angle

	URL string
}

// The functions errPercentageValue to errInvalidURL are the cases with data
// of FilterValueListParserError in svgtypes src/filter_functions.rs.

// errPercentageValue is FilterValueListParserError::PercentageValue.
//
// Lengths with percentage values are not allowed.
func errPercentageValue(pos int) error {
	return fmt.Errorf("%w at position %d", okage.ErrSVGTypesFilterPercentageValue, pos)
}

// errNegativeValue is FilterValueListParserError::NegativeValue.
//
// Some values cannot be negative.
func errNegativeValue(pos int) error {
	return fmt.Errorf("%w at position %d", okage.ErrSVGTypesFilterNegativeValue, pos)
}

// errInvalidAngle is FilterValueListParserError::InvalidAngle.
//
// An invalid angle value.
func errInvalidAngle(pos int) error {
	return fmt.Errorf("%w at position %d", okage.ErrSVGTypesFilterInvalidAngle, pos)
}

// errMissingDropShadowOffset is
// FilterValueListParserError::MissingDropShadowOffset.
//
// Drop shadow offset values must be set.
func errMissingDropShadowOffset(pos int) error {
	return fmt.Errorf("%w at position %d", okage.ErrSVGTypesFilterMissingDropShadowOffset, pos)
}

// errInvalidURL is FilterValueListParserError::InvalidUrl.
//
// Usually indicates an empty url.
func errInvalidURL(pos int) error {
	return fmt.Errorf("%w at position %d", okage.ErrSVGTypesFilterInvalidURL, pos)
}

// FilterValueListParser is FilterValueListParser in svgtypes
// src/filter_functions.rs.
//
// A pull-based [`<filter-value-list>`] parser.
//
// When value is set to `none`, the parser will return `None` immediately.
// Meaning that an empty string and `none` will produce the same results.
//
// [`<filter-value-list>`]: https://www.w3.org/TR/filter-effects-1/#typedef-filter-value-list
type FilterValueListParser struct {
	stream Stream
}

// NewFilterValueListParser is the impl From<&str> for FilterValueListParser
// in svgtypes src/filter_functions.rs.
func NewFilterValueListParser(text string) *FilterValueListParser {
	return &FilterValueListParser{
		stream: NewStream(text),
	}
}

// Next is Iterator::next for FilterValueListParser in svgtypes
// src/filter_functions.rs. ok is false at the end, where Rust returns None.
// When ok is true, the value and err are the Result of Rust.
func (p *FilterValueListParser) Next() (FilterValue, bool, error) {
	p.stream.SkipSpaces()

	if p.stream.AtEnd() {
		// an empty attribute is still a valid value
		return FilterValue{}, false, nil
	}

	if p.stream.StartsWith("none") {
		p.stream.Advance(4)
		p.stream.SkipSpaces()

		if p.stream.AtEnd() {
			return FilterValue{}, false, nil
		}
		p.stream.JumpToEnd()
		return FilterValue{}, true, okage.ErrSVGTypesInvalidValue
	}

	res, err := p.parseNext()
	if err != nil {
		p.stream.JumpToEnd()
	}

	return res, true, err
}

// All returns an iterator over the results of Next, as the Iterator impl of
// FilterValueListParser in svgtypes src/filter_functions.rs gives them.
func (p *FilterValueListParser) All() iter.Seq2[FilterValue, error] {
	return func(yield func(FilterValue, error) bool) {
		for {
			v, ok, err := p.Next()
			if !ok || !yield(v, err) {
				return
			}
		}
	}
}

// parseNext is FilterValueListParser::parse_next in svgtypes
// src/filter_functions.rs.
func (p *FilterValueListParser) parseNext() (FilterValue, error) {
	s := &p.stream

	start := s.Pos()
	name := s.ConsumeASCIIIdent()
	s.SkipSpaces()
	if err := s.ConsumeByte('('); err != nil {
		return FilterValue{}, err
	}
	s.SkipSpaces()

	var value FilterValue
	switch name {
	case "blur":
		if s.IsCurrByteEq(')') {
			value = FilterValue{Kind: FilterValueBlur, Length: LengthZero()}
		} else {
			v, err := parseFilterPositiveLength(s)
			if err != nil {
				return FilterValue{}, err
			}
			value = FilterValue{Kind: FilterValueBlur, Length: v}
		}
	case "drop-shadow":
		v, err := parseDropShadowFunc(s)
		if err != nil {
			return FilterValue{}, err
		}
		value = v
	case "hue-rotate":
		if s.IsCurrByteEq(')') {
			value = FilterValue{Kind: FilterValueHueRotate, Angle: NewAngle(0.0, AngleUnitDegrees)}
		} else {
			v, err := parseFilterAngle(s)
			if err != nil {
				return FilterValue{}, err
			}
			value = FilterValue{Kind: FilterValueHueRotate, Angle: v}
		}
	case "brightness":
		v, err := parseGenericColorFunc(s)
		if err != nil {
			return FilterValue{}, err
		}
		value = FilterValue{Kind: FilterValueBrightness, Number: v}
	case "contrast":
		v, err := parseGenericColorFunc(s)
		if err != nil {
			return FilterValue{}, err
		}
		value = FilterValue{Kind: FilterValueContrast, Number: v}
	case "grayscale":
		v, err := parseGenericColorFunc(s)
		if err != nil {
			return FilterValue{}, err
		}
		value = FilterValue{Kind: FilterValueGrayscale, Number: v}
	case "invert":
		v, err := parseGenericColorFunc(s)
		if err != nil {
			return FilterValue{}, err
		}
		value = FilterValue{Kind: FilterValueInvert, Number: v}
	case "opacity":
		v, err := parseGenericColorFunc(s)
		if err != nil {
			return FilterValue{}, err
		}
		value = FilterValue{Kind: FilterValueOpacity, Number: v}
	case "saturate":
		v, err := parseGenericColorFunc(s)
		if err != nil {
			return FilterValue{}, err
		}
		value = FilterValue{Kind: FilterValueSaturate, Number: v}
	case "sepia":
		v, err := parseGenericColorFunc(s)
		if err != nil {
			return FilterValue{}, err
		}
		value = FilterValue{Kind: FilterValueSepia, Number: v}
	case "url":
		if err := s.ConsumeByte('#'); err != nil {
			return FilterValue{}, err
		}
		link := s.ConsumeBytes(func(_ *Stream, c byte) bool { return c != ' ' && c != ')' })
		if link == "" {
			return FilterValue{}, errInvalidURL(s.CalcCharPosAt(start))
		}
		value = FilterValue{Kind: FilterValueURL, URL: link}
	default:
		return FilterValue{}, errUnexpectedData(s.CalcCharPosAt(start))
	}

	s.SkipSpaces()
	if err := s.ConsumeByte(')'); err != nil {
		return FilterValue{}, err
	}
	s.SkipSpaces()

	return value, nil
}

// parseDropShadowFunc is parse_drop_shadow_func in svgtypes
// src/filter_functions.rs.
func parseDropShadowFunc(s *Stream) (FilterValue, error) {
	if s.IsCurrByteEq(')') {
		pos := s.CalcCharPos()
		return FilterValue{}, errMissingDropShadowOffset(pos)
	}

	// Color can be set before and after lengths.
	var color Color
	hasColor := false
	isCurrentColor := false
	if c, ok := s.TryParseColor(); ok {
		color, hasColor = c, true
		s.SkipSpaces()
	} else if s.StartsWith("currentColor") {
		isCurrentColor = true
		s.Advance(12)
		s.SkipSpaces()
	}

	// Offset is the only mandatory value.
	dx, err := parseFilterLength(s)
	if err != nil {
		return FilterValue{}, err
	}
	s.SkipSpaces()
	dy, err := parseFilterLength(s)
	if err != nil {
		return FilterValue{}, err
	}
	s.SkipSpaces()

	// std_dev is optional
	stdDev := LengthZero()
	if v, err := parseFilterPositiveLength(s); err == nil {
		stdDev = v
		s.SkipSpaces()
	}

	// Try to parse a color after length, if it wasn't set before.
	if !hasColor && !isCurrentColor {
		if c, ok := s.TryParseColor(); ok {
			color, hasColor = c, true
			s.SkipSpaces()
		} else if s.StartsWith("currentColor") {
			s.Advance(12)
		}
	}

	return FilterValue{
		Kind:     FilterValueDropShadow,
		Color:    color,
		HasColor: hasColor,
		Dx:       dx,
		Dy:       dy,
		StdDev:   stdDev,
	}, nil
}

// parseGenericColorFunc is parse_generic_color_func in svgtypes
// src/filter_functions.rs.
func parseGenericColorFunc(s *Stream) (float64, error) {
	if s.IsCurrByteEq(')') {
		return 1.0, nil
	}
	start := s.Pos()
	value, err := s.ParseNumberOrPercent()
	if err != nil {
		return 0, err
	}

	if math.Signbit(value) {
		pos := s.CalcCharPosAt(start)
		return 0, errNegativeValue(pos)
	}

	return value, nil
}

// parseFilterLength is parse_filter_length in svgtypes
// src/filter_functions.rs.
func parseFilterLength(s *Stream) (Length, error) {
	start := s.Pos()
	value, err := s.ParseLength()
	if err != nil {
		return Length{}, err
	}

	if value.Unit == LengthUnitPercent {
		pos := s.CalcCharPosAt(start)
		return Length{}, errPercentageValue(pos)
	}

	return value, nil
}

// parseFilterPositiveLength is parse_filter_positive_length in svgtypes
// src/filter_functions.rs.
func parseFilterPositiveLength(s *Stream) (Length, error) {
	start := s.Pos()
	value, err := s.ParseLength()
	if err != nil {
		return Length{}, err
	}

	if math.Signbit(value.Number) {
		pos := s.CalcCharPosAt(start)
		return Length{}, errNegativeValue(pos)
	}

	if value.Unit == LengthUnitPercent {
		pos := s.CalcCharPosAt(start)
		return Length{}, errPercentageValue(pos)
	}

	return value, nil
}

// parseFilterAngle is parse_filter_angle in svgtypes src/filter_functions.rs.
//
// Just like a normal angle, but units are mandatory.
func parseFilterAngle(s *Stream) (Angle, error) {
	s.SkipSpaces()

	start := s.Pos()
	n, err := s.ParseNumber()
	if err != nil {
		return Angle{}, err
	}

	var u AngleUnit
	switch {
	case s.StartsWith("deg"):
		s.Advance(3)
		u = AngleUnitDegrees
	case s.StartsWith("grad"):
		s.Advance(4)
		u = AngleUnitGradians
	case s.StartsWith("rad"):
		s.Advance(3)
		u = AngleUnitRadians
	case s.StartsWith("turn"):
		s.Advance(4)
		u = AngleUnitTurns
	default:
		// Only zero value allowed to be unit-less.
		if n == 0.0 {
			u = AngleUnitDegrees
		} else {
			pos := s.CalcCharPosAt(start)
			return Angle{}, errInvalidAngle(pos)
		}
	}

	return NewAngle(n, u), nil
}
