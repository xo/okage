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
	order := DefaultPaintOrder().Order

	s := NewStream(text)
	count := 0
	for !s.AtEnd() && count < 3 {
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

		index := slices.Index(order[count:], name)
		if index < 0 {
			return DefaultPaintOrder()
		}
		// Port: slice::rotate_right(1) of order[count..=count+index] moves
		// the found kind to the position count.
		copy(order[count+1:count+index+1], order[count:count+index])
		order[count] = name
		count++
	}

	s.SkipSpaces()
	if !s.AtEnd() {
		// Any trailing data is an error.
		return DefaultPaintOrder()
	}

	return PaintOrder{Order: order}
}
