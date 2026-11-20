// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

// okage: this file is not in go-text (D84).

import "testing"

// TestFixedArithmetic checks the products and the quotients of Fixed
// against the rules of Fixed of font-types src/fixed.rs.
func TestFixedArithmetic(t *testing.T) {
	half := FixedOne / 2
	for _, tc := range []struct {
		name      string
		got, want Fixed
	}{
		{"3 * 0.5", FixedFromInt(3).Mul(half), 0x18000},
		{"-3 * 0.5", FixedFromInt(-3).Mul(half), -0x18000},
		// 1/65536 * 0.5 is a tie, which goes away from zero.
		{"tie", Fixed(1).Mul(half), 1},
		{"negative tie", Fixed(-1).Mul(half), -1},
		{"1 / 3", FixedOne.Div(FixedFromInt(3)), 21845},
		{"-2 / 3", FixedFromInt(-2).Div(FixedFromInt(3)), -43691},
		{"1 / 0", FixedOne.Div(0), 0x7FFFFFFF},
		{"-1 / 0", (-FixedOne).Div(0), -0x7FFFFFFF},
		{"1 * 2 / 3", FixedOne.MulDiv(FixedFromInt(2), FixedFromInt(3)), 43691},
		{"1 * -2 / 3", FixedOne.MulDiv(FixedFromInt(-2), FixedFromInt(3)), -43691},
		{"1 * -2 / -3", FixedOne.MulDiv(FixedFromInt(-2), FixedFromInt(-3)), 43691},
		{"0.6 in 2.14", Coord(9830).Fixed(), 39320},
		{"0.6 to 2.14", Fixed(Fixed(39322).ToCoord()), 9831},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

// TestEvaluateFixed compares the scalars of a region in 16.16 with the
// scalars in float32.
func TestEvaluateFixed(t *testing.T) {
	region := VariationRegion{RegionAxes: []RegionAxisCoordinates{
		{StartCoord: -1 << 14, PeakCoord: -1 << 13, EndCoord: 0},
		{StartCoord: 0, PeakCoord: 0, EndCoord: 1 << 14},
	}}
	for _, tc := range []struct {
		coord Coord
		want  Fixed
	}{
		{-1 << 14, 0},
		{-3 << 12, FixedOne / 2},
		{-1 << 13, FixedOne},
		{-1 << 12, FixedOne / 2},
		{0, 0},
		{1 << 13, 0},
		// 2/3 of the way from the start to the peak.
		{-10923, 43688},
	} {
		got := region.EvaluateFixed([]Coord{tc.coord})
		if got != tc.want {
			t.Errorf("coordinate %d: got %d, want %d", tc.coord, got, tc.want)
		}
		if f := region.Evaluate([]Coord{tc.coord}); f-float32(got)/(1<<16) > 1e-4 || float32(got)/(1<<16)-f > 1e-4 {
			t.Errorf("coordinate %d: the scalar %d is far from the scalar %g in float32", tc.coord, got, f)
		}
	}
}

// TestMapFixed checks that MapFixed gives the coordinates of Map for the
// coordinates in 2.14.
func TestMapFixed(t *testing.T) {
	sm := SegmentMaps{AxisValueMaps: []AxisValueMap{
		{FromCoordinate: -1 << 14, ToCoordinate: -1 << 14},
		{FromCoordinate: 0, ToCoordinate: 0},
		{FromCoordinate: 1 << 13, ToCoordinate: 3 << 12},
		{FromCoordinate: 1 << 14, ToCoordinate: 1 << 14},
	}}
	for c := Coord(-1 << 14); c <= 1<<14; c += 97 {
		if got, want := sm.MapFixed(c.Fixed()).ToCoord(), sm.Map(c); got != want {
			t.Errorf("coordinate %d: MapFixed gives %d, and Map gives %d", c, got, want)
		}
	}
	// 0.6 is 39322 in 16.16. It is 1/5 of the way from 0.5 to 1, which map
	// to 0.75 and 1, so it maps to 0.8, which is 13107 in 2.14.
	if got := sm.MapFixed(39322).ToCoord(); got != 13107 {
		t.Errorf("0.6: got %d, want 13107", got)
	}
}
