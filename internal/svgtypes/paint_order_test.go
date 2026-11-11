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
		{"parse_12", "stroke stroke markers", DefaultPaintOrder()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParsePaintOrder(tt.text); got != tt.want {
				t.Errorf("ParsePaintOrder(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// TestAllPaintOrderCombinations is test_all_paint_order_combinations in
// svgtypes src/paint_order.rs.
func TestAllPaintOrderCombinations(t *testing.T) {
	fill, stroke, markers := PaintOrderKindFill, PaintOrderKindStroke, PaintOrderKindMarkers
	for _, c := range []struct {
		text string
		want [3]PaintOrderKind
	}{
		{"fill stroke markers", [3]PaintOrderKind{fill, stroke, markers}},
		{"fill markers stroke", [3]PaintOrderKind{fill, markers, stroke}},
		{"stroke fill markers", [3]PaintOrderKind{stroke, fill, markers}},
		{"stroke markers fill", [3]PaintOrderKind{stroke, markers, fill}},
		{"markers fill stroke", [3]PaintOrderKind{markers, fill, stroke}},
		{"markers stroke fill", [3]PaintOrderKind{markers, stroke, fill}},
	} {
		if got := ParsePaintOrder(c.text); got != PaintOrderFromArray(c.want) {
			t.Errorf("ParsePaintOrder(%q) = %v, want %v", c.text, got, PaintOrderFromArray(c.want))
		}
	}
}
