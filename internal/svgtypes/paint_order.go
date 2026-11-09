// Copyright 2022 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/paint_order.rs.

import "slices"

// PaintOrderKind is PaintOrderKind in svgtypes src/paint_order.rs.
//
// [`paint-order`] property variants.
//
// [`paint-order`]: https://www.w3.org/TR/SVG2/painting.html#PaintOrder
type PaintOrderKind uint8

// The values of PaintOrderKind.
const (
	PaintOrderKindFill PaintOrderKind = iota
	PaintOrderKindStroke
	PaintOrderKindMarkers
)

// paintOrderKindNames holds the Rust name of each value of PaintOrderKind.
var paintOrderKindNames = [...]string{
	PaintOrderKindFill:    "Fill",
	PaintOrderKindStroke:  "Stroke",
	PaintOrderKindMarkers: "Markers",
}

// String returns the Rust name of the value.
func (k PaintOrderKind) String() string {
	if int(k) < len(paintOrderKindNames) {
		return paintOrderKindNames[k]
	}
	return "PaintOrderKind(?)"
}

// PaintOrder is PaintOrder in svgtypes src/paint_order.rs.
//
// Representation of the [`paint-order`] property.
//
// [`paint-order`]: https://www.w3.org/TR/SVG2/painting.html#PaintOrder
type PaintOrder struct {
	// The order.
	//
	// Guarantee to not have duplicates.
	Order [3]PaintOrderKind
}

// DefaultPaintOrder is the impl Default for PaintOrder in svgtypes
// src/paint_order.rs.
func DefaultPaintOrder() PaintOrder {
	return PaintOrder{
		Order: [3]PaintOrderKind{
			PaintOrderKindFill,
			PaintOrderKindStroke,
			PaintOrderKindMarkers,
		},
	}
}

// PaintOrderFromArray is the impl From<[PaintOrderKind; 3]> for PaintOrder in
// svgtypes src/paint_order.rs.
func PaintOrderFromArray(order [3]PaintOrderKind) PaintOrder {
	return PaintOrder{Order: order}
}

// ParsePaintOrder is the impl FromStr for PaintOrder in svgtypes
// src/paint_order.rs. The error type of upstream is (), and upstream never
// returns it, so ParsePaintOrder returns no error.
//
// Parses `PaintOrder` from a string.
//
// Never returns an error and fallbacks to the default value instead.
func ParsePaintOrder(text string) PaintOrder {
	var order []PaintOrderKind

	left := []PaintOrderKind{
		PaintOrderKindFill,
		PaintOrderKindStroke,
		PaintOrderKindMarkers,
	}

	s := NewStream(text)
	for !s.AtEnd() && len(order) < 3 {
		s.SkipSpaces()
		ident := s.ConsumeASCIIIdent()
		s.SkipSpaces()
		var name PaintOrderKind
		switch ident {
		// `normal` is the special value that should short-circuit.
		case "normal":
			return DefaultPaintOrder()
		case "fill":
			name = PaintOrderKindFill
		case "stroke":
			name = PaintOrderKindStroke
		case "markers":
			name = PaintOrderKindMarkers
		default:
			return DefaultPaintOrder()
		}

		if index := slices.Index(left, name); index >= 0 {
			left = slices.Delete(left, index, index+1)
		}

		order = append(order, name)
	}

	s.SkipSpaces()
	if !s.AtEnd() {
		// Any trailing data is an error.
		return DefaultPaintOrder()
	}

	if len(order) == 0 {
		return DefaultPaintOrder()
	}

	// Any missing values should be added in the original order.
	for len(order) < 3 && len(left) != 0 {
		order = append(order, left[0])
		left = left[1:]
	}

	// Any duplicates is an error.
	if order[0] == order[1] || order[0] == order[2] || order[1] == order[2] {
		// Any trailing data is an error.
		return DefaultPaintOrder()
	}

	return PaintOrder{
		Order: [3]PaintOrderKind{order[0], order[1], order[2]},
	}
}
