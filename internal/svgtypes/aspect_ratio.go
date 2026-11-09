// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/aspect_ratio.rs.

// Align is Align in svgtypes src/aspect_ratio.rs.
//
// Representation of the `align` value of the [`preserveAspectRatio`] attribute.
//
// [`preserveAspectRatio`]: https://www.w3.org/TR/SVG11/coords.html#PreserveAspectRatioAttribute
type Align uint8

// The values of Align.
const (
	AlignNone Align = iota
	AlignXMinYMin
	AlignXMidYMin
	AlignXMaxYMin
	AlignXMinYMid
	AlignXMidYMid
	AlignXMaxYMid
	AlignXMinYMax
	AlignXMidYMax
	AlignXMaxYMax
)

// alignNames holds the Rust name of each value of Align.
var alignNames = [...]string{
	AlignNone:     "None", //nolint:goconst // the Rust name of the case
	AlignXMinYMin: "XMinYMin",
	AlignXMidYMin: "XMidYMin",
	AlignXMaxYMin: "XMaxYMin",
	AlignXMinYMid: "XMinYMid",
	AlignXMidYMid: "XMidYMid",
	AlignXMaxYMid: "XMaxYMid",
	AlignXMinYMax: "XMinYMax",
	AlignXMidYMax: "XMidYMax",
	AlignXMaxYMax: "XMaxYMax",
}

// String returns the Rust name of the value.
func (a Align) String() string {
	if int(a) < len(alignNames) {
		return alignNames[a]
	}
	return "Align(?)"
}

// AspectRatio is AspectRatio in svgtypes src/aspect_ratio.rs.
//
// Representation of the [`preserveAspectRatio`] attribute.
//
// SVG 2 removed the `defer` keyword, but we still support it.
//
// [`preserveAspectRatio`]: https://www.w3.org/TR/SVG11/coords.html#PreserveAspectRatioAttribute
type AspectRatio struct {
	// `<defer>` value.
	//
	// Set to `true` when `defer` value is present.
	Defer bool
	// `<align>` value.
	Align Align
	// `<meetOrSlice>` value.
	//
	// - Set to `true` when `slice` value is present.
	// - Set to `false` when `meet` value is present or value is not set at all.
	Slice bool
}

// ParseAspectRatio is the impl FromStr for AspectRatio in svgtypes
// src/aspect_ratio.rs.
func ParseAspectRatio(text string) (AspectRatio, error) {
	s := NewStream(text)

	s.SkipSpaces()

	isDefer := s.StartsWith("defer")
	if isDefer {
		s.Advance(5)
		if err := s.ConsumeByte(' '); err != nil {
			return AspectRatio{}, err
		}
		s.SkipSpaces()
	}

	start := s.Pos()
	var align Align
	switch s.ConsumeASCIIIdent() {
	case "none": //nolint:goconst // the keyword of upstream
		align = AlignNone
	case "xMinYMin":
		align = AlignXMinYMin
	case "xMidYMin":
		align = AlignXMidYMin
	case "xMaxYMin":
		align = AlignXMaxYMin
	case "xMinYMid":
		align = AlignXMinYMid
	case "xMidYMid":
		align = AlignXMidYMid
	case "xMaxYMid":
		align = AlignXMaxYMid
	case "xMinYMax":
		align = AlignXMinYMax
	case "xMidYMax":
		align = AlignXMidYMax
	case "xMaxYMax":
		align = AlignXMaxYMax
	default:
		return AspectRatio{}, errUnexpectedData(s.CalcCharPosAt(start))
	}

	s.SkipSpaces()

	slice := false
	if !s.AtEnd() {
		start := s.Pos()
		v := s.ConsumeASCIIIdent()
		switch v {
		case "meet":
		case "slice":
			slice = true
		case "":
		default:
			return AspectRatio{}, errUnexpectedData(s.CalcCharPosAt(start))
		}
	}

	return AspectRatio{
		Defer: isDefer,
		Align: align,
		Slice: slice,
	}, nil
}

// DefaultAspectRatio is the impl Default for AspectRatio in svgtypes
// src/aspect_ratio.rs.
func DefaultAspectRatio() AspectRatio {
	return AspectRatio{
		Defer: false,
		Align: AlignXMidYMid,
		Slice: false,
	}
}
