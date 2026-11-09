// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/viewbox.rs. The enum ViewBoxError becomes the
// sentinel errors ErrSVGTypesViewBoxInvalidNumber and
// ErrSVGTypesViewBoxInvalidSize of the root package (D21).

import "github.com/xo/okage"

// ViewBox is ViewBox in svgtypes src/viewbox.rs.
//
// Representation of the [`<viewBox>`] type.
//
// [`<viewBox>`]: https://www.w3.org/TR/SVG2/coords.html#ViewBoxAttribute
type ViewBox struct {
	X float64
	Y float64
	W float64
	H float64
}

// NewViewBox is ViewBox::new in svgtypes src/viewbox.rs.
//
// Creates a new `ViewBox`.
func NewViewBox(x, y, w, h float64) ViewBox {
	return ViewBox{X: x, Y: y, W: w, H: h}
}

// ParseViewBox is the impl FromStr for ViewBox in svgtypes src/viewbox.rs.
func ParseViewBox(text string) (ViewBox, error) {
	s := NewStream(text)

	x, err := s.ParseListNumber()
	if err != nil {
		return ViewBox{}, okage.ErrSVGTypesViewBoxInvalidNumber
	}
	y, err := s.ParseListNumber()
	if err != nil {
		return ViewBox{}, okage.ErrSVGTypesViewBoxInvalidNumber
	}
	w, err := s.ParseListNumber()
	if err != nil {
		return ViewBox{}, okage.ErrSVGTypesViewBoxInvalidNumber
	}
	h, err := s.ParseListNumber()
	if err != nil {
		return ViewBox{}, okage.ErrSVGTypesViewBoxInvalidNumber
	}

	if w <= 0.0 || h <= 0.0 {
		return ViewBox{}, okage.ErrSVGTypesViewBoxInvalidSize
	}

	return NewViewBox(x, y, w, h), nil
}
