// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/paint.rs.

import (
	"strings"

	"github.com/xo/okage"
)

// PaintFallbackKind is the case of the enum PaintFallback in svgtypes
// src/paint.rs (D27).
type PaintFallbackKind uint8

// The cases of PaintFallback.
const (
	// PaintFallbackNone is PaintFallback::None.
	//
	// The `none` value.
	PaintFallbackNone PaintFallbackKind = iota
	// PaintFallbackCurrentColor is PaintFallback::CurrentColor.
	//
	// The `currentColor` value.
	PaintFallbackCurrentColor
	// PaintFallbackColor is PaintFallback::Color.
	//
	// [`<color>`] value.
	//
	// [`<color>`]: https://www.w3.org/TR/css-color-3/
	PaintFallbackColor
)

// paintFallbackKindNames holds the Rust name of each case of PaintFallbackKind.
var paintFallbackKindNames = [...]string{
	PaintFallbackNone:         "None", //nolint:goconst // the Rust name of the case
	PaintFallbackCurrentColor: "CurrentColor",
	PaintFallbackColor:        "Color",
}

// String returns the Rust name of the case.
func (k PaintFallbackKind) String() string {
	if int(k) < len(paintFallbackKindNames) {
		return paintFallbackKindNames[k]
	}
	return "PaintFallbackKind(?)"
}

// PaintFallback is PaintFallback in svgtypes src/paint.rs. The Rust enum has
// data in one case, so it is a struct with a kind (D27). Color holds the
// data of the case Color.
//
// Representation of the fallback part of the [`<paint>`] type.
//
// Used by the [`Paint`] type.
//
// [`<paint>`]: https://www.w3.org/TR/SVG2/painting.html#SpecifyingPaint
type PaintFallback struct {
	Kind  PaintFallbackKind
	Color Color
}

// PaintKind is the case of the enum Paint in svgtypes src/paint.rs (D27).
type PaintKind uint8

// The cases of Paint.
const (
	// PaintNone is Paint::None.
	//
	// The `none` value.
	PaintNone PaintKind = iota
	// PaintInherit is Paint::Inherit.
	//
	// The `inherit` value.
	PaintInherit
	// PaintCurrentColor is Paint::CurrentColor.
	//
	// The `currentColor` value.
	PaintCurrentColor
	// PaintColor is Paint::Color.
	//
	// [`<color>`] value.
	//
	// [`<color>`]: https://www.w3.org/TR/css-color-3/
	PaintColor
	// PaintFuncIRI is Paint::FuncIRI.
	//
	// [`<FuncIRI>`] value with an optional fallback.
	//
	// [`<FuncIRI>`]: https://www.w3.org/TR/SVG11/types.html#DataTypeFuncIRI
	PaintFuncIRI
	// PaintContextFill is Paint::ContextFill.
	//
	// The `context-fill` value.
	PaintContextFill
	// PaintContextStroke is Paint::ContextStroke.
	//
	// The `context-stroke` value.
	PaintContextStroke
)

// paintKindNames holds the Rust name of each case of PaintKind.
var paintKindNames = [...]string{
	PaintNone:          "None",
	PaintInherit:       "Inherit",
	PaintCurrentColor:  "CurrentColor",
	PaintColor:         "Color",
	PaintFuncIRI:       "FuncIRI",
	PaintContextFill:   "ContextFill",
	PaintContextStroke: "ContextStroke",
}

// String returns the Rust name of the case.
func (k PaintKind) String() string {
	if int(k) < len(paintKindNames) {
		return paintKindNames[k]
	}
	return "PaintKind(?)"
}

// Paint is Paint in svgtypes src/paint.rs. The Rust enum has data in two
// cases, so it is a struct with a kind (D27). Color holds the data of the
// case Color. FuncIRI and Fallback hold the data of the case FuncIRI, and
// HasFallback is false where the Option of Rust is None.
//
// Representation of the [`<paint>`] type.
//
// Doesn't own the data. Use only for parsing.
//
// `<icccolor>` isn't supported.
//
// [`<paint>`]: https://www.w3.org/TR/SVG2/painting.html#SpecifyingPaint
type Paint struct {
	Kind        PaintKind
	Color       Color
	FuncIRI     string
	Fallback    PaintFallback
	HasFallback bool
}

// ParsePaint is Paint::from_str in svgtypes src/paint.rs.
//
// Parses a `Paint` from a string.
func ParsePaint(text string) (Paint, error) {
	// str::trim of Rust trims the chars with the property White_Space, as
	// strings.TrimSpace does.
	text = strings.TrimSpace(text)
	switch text {
	case "none": //nolint:goconst // the keyword of upstream
		return Paint{Kind: PaintNone}, nil
	case "inherit":
		return Paint{Kind: PaintInherit}, nil
	case "currentColor":
		return Paint{Kind: PaintCurrentColor}, nil
	case "context-fill":
		return Paint{Kind: PaintContextFill}, nil
	case "context-stroke":
		return Paint{Kind: PaintContextStroke}, nil
	default:
		s := NewStream(text)
		if s.StartsWith("url(") {
			link, err := s.ParseFuncIRI()
			if err != nil {
				return Paint{}, okage.ErrSVGTypesInvalidValue
			}
			s.SkipSpaces()

			// get fallback
			if !s.AtEnd() {
				fallback := s.SliceTail()
				switch fallback {
				case "none":
					return Paint{Kind: PaintFuncIRI, FuncIRI: link, Fallback: PaintFallback{Kind: PaintFallbackNone}, HasFallback: true}, nil
				case "currentColor":
					return Paint{Kind: PaintFuncIRI, FuncIRI: link, Fallback: PaintFallback{Kind: PaintFallbackCurrentColor}, HasFallback: true}, nil
				default:
					color, err := ParseColor(fallback)
					if err != nil {
						return Paint{}, err
					}
					return Paint{Kind: PaintFuncIRI, FuncIRI: link, Fallback: PaintFallback{Kind: PaintFallbackColor, Color: color}, HasFallback: true}, nil
				}
			}
			return Paint{Kind: PaintFuncIRI, FuncIRI: link}, nil
		}
		c, err := ParseColor(text)
		if err != nil {
			return Paint{}, okage.ErrSVGTypesInvalidValue
		}
		return Paint{Kind: PaintColor, Color: c}, nil
	}
}
