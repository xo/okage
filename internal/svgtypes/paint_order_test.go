// Copyright 2022 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import "testing"

// The tests of svgtypes src/paint_order.rs.

func TestParsePaintOrder(t *testing.T) {
	tests := []struct {
		name string
		text string
		want PaintOrder
	}{
		{"parse_1", "normal", DefaultPaintOrder()},
		{"parse_2", "qwe", DefaultPaintOrder()},
		{"parse_3", "", DefaultPaintOrder()},
		{"parse_4", "stroke qwe", DefaultPaintOrder()},
		{"parse_5", "stroke stroke", DefaultPaintOrder()},
		{"parse_6", "stroke", PaintOrderFromArray([3]PaintOrderKind{
			PaintOrderKindStroke, PaintOrderKindFill, PaintOrderKindMarkers,
		})},
		{"parse_7", "stroke markers", PaintOrderFromArray([3]PaintOrderKind{
			PaintOrderKindStroke, PaintOrderKindMarkers, PaintOrderKindFill,
		})},
		{"parse_8", "stroke markers fill", PaintOrderFromArray([3]PaintOrderKind{
			PaintOrderKindStroke, PaintOrderKindMarkers, PaintOrderKindFill,
		})},
		{"parse_9", "markers", PaintOrderFromArray([3]PaintOrderKind{
			PaintOrderKindMarkers, PaintOrderKindFill, PaintOrderKindStroke,
		})},
		{"parse_10", "  stroke\n", PaintOrderFromArray([3]PaintOrderKind{
			PaintOrderKindStroke, PaintOrderKindFill, PaintOrderKindMarkers,
		})},
		{"parse_11", "stroke stroke stroke stroke", DefaultPaintOrder()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParsePaintOrder(tt.text); got != tt.want {
				t.Errorf("ParsePaintOrder(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}
