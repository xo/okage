// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/points.rs.

import "iter"

// PointsParser is PointsParser in svgtypes src/points.rs.
//
// A pull-based [`<list-of-points>`] parser.
//
// Use it for the `points` attribute of the `polygon` and `polyline` elements.
//
// # Errors
//
//   - Stops on a first invalid character. Follows the same rules as paths parser.
//
// # Notes
//
//   - If data contains an odd number of coordinates - the last one will be ignored.
//     As SVG spec states.
//   - It doesn't validate that there are more than two coordinate pairs,
//     which is required by the SVG spec.
//
// [`<list-of-points>`]: https://www.w3.org/TR/SVG11/shapes.html#PointsBNF
type PointsParser struct {
	s Stream
}

// NewPointsParser is the impl From<&str> for PointsParser in svgtypes
// src/points.rs.
func NewPointsParser(v string) *PointsParser {
	return &PointsParser{s: NewStream(v)}
}

// Next is Iterator::next for PointsParser in svgtypes src/points.rs. It
// returns the next pair (x, y), or false at the end, where Rust returns None.
func (p *PointsParser) Next() (float64, float64, bool) {
	if p.s.AtEnd() {
		return 0, 0, false
	}
	x, err := p.s.ParseListNumber()
	if err != nil {
		return 0, 0, false
	}

	y, err := p.s.ParseListNumber()
	if err != nil {
		return 0, 0, false
	}

	return x, y, true
}

// All returns an iterator over the pairs of Next, as the Iterator impl of
// PointsParser in svgtypes src/points.rs gives them.
func (p *PointsParser) All() iter.Seq2[float64, float64] {
	return func(yield func(float64, float64) bool) {
		for {
			x, y, ok := p.Next()
			if !ok || !yield(x, y) {
				return
			}
		}
	}
}
