// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/enable_background.rs.

import "github.com/xo/okage"

// EnableBackgroundKind is the case of the enum EnableBackground in svgtypes
// src/enable_background.rs (D27).
type EnableBackgroundKind uint8

// The cases of EnableBackground.
const (
	EnableBackgroundAccumulate EnableBackgroundKind = iota
	EnableBackgroundNew
	EnableBackgroundNewWithRegion
)

// enableBackgroundKindNames holds the Rust name of each case of EnableBackgroundKind.
var enableBackgroundKindNames = [...]string{
	EnableBackgroundAccumulate:    "Accumulate",
	EnableBackgroundNew:           "New",
	EnableBackgroundNewWithRegion: "NewWithRegion",
}

// String returns the Rust name of the case.
func (k EnableBackgroundKind) String() string {
	if int(k) < len(enableBackgroundKindNames) {
		return enableBackgroundKindNames[k]
	}
	return "EnableBackgroundKind(?)"
}

// EnableBackground is EnableBackground in svgtypes src/enable_background.rs.
// The Rust enum has data in one case, so it is a struct with a kind (D27).
// X, Y, Width and Height hold the data of NewWithRegion, and are zero for
// the other cases.
//
// Representation of the [`enable-background`] attribute.
//
// [`enable-background`]: https://www.w3.org/TR/SVG11/filters.html#EnableBackgroundProperty
type EnableBackground struct {
	Kind   EnableBackgroundKind
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// ParseEnableBackground is the impl FromStr for EnableBackground in svgtypes
// src/enable_background.rs.
func ParseEnableBackground(text string) (EnableBackground, error) {
	s := NewStream(text)
	s.SkipSpaces()
	switch {
	case s.StartsWith("accumulate"):
		s.Advance(10)
		s.SkipSpaces()
		if !s.AtEnd() {
			return EnableBackground{}, errUnexpectedData(s.CalcCharPos())
		}

		return EnableBackground{Kind: EnableBackgroundAccumulate}, nil
	case s.StartsWith("new"):
		s.Advance(3)
		s.SkipSpaces()
		if s.AtEnd() {
			return EnableBackground{Kind: EnableBackgroundNew}, nil
		}

		x, err := s.ParseListNumber()
		if err != nil {
			return EnableBackground{}, err
		}
		y, err := s.ParseListNumber()
		if err != nil {
			return EnableBackground{}, err
		}
		width, err := s.ParseListNumber()
		if err != nil {
			return EnableBackground{}, err
		}
		height, err := s.ParseListNumber()
		if err != nil {
			return EnableBackground{}, err
		}

		s.SkipSpaces()
		if !s.AtEnd() {
			return EnableBackground{}, errUnexpectedData(s.CalcCharPos())
		}

		// Region size must be valid;
		if !(width > 0.0 && height > 0.0) {
			return EnableBackground{}, okage.ErrSVGTypesInvalidValue
		}

		return EnableBackground{
			Kind:   EnableBackgroundNewWithRegion,
			X:      x,
			Y:      y,
			Width:  width,
			Height: height,
		}, nil
	default:
		return EnableBackground{}, okage.ErrSVGTypesInvalidValue
	}
}
