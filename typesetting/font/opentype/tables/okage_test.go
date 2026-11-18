// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

// okage: this file is not in go-text. It tests the change of D61 to
// VariationRegion.Evaluate, and the changes of D65 to SegmentMaps.Map,
// ParseGlyf, Hmtx.Advance and ParseHmtx.

import "testing"

func TestEvaluateMissingCoords(t *testing.T) {
	// The region peaks at 1 on the second axis. The first axis does not
	// change it.
	region := VariationRegion{RegionAxes: []RegionAxisCoordinates{
		{StartCoord: 0, PeakCoord: 0, EndCoord: 0},
		{StartCoord: 0, PeakCoord: 1 << 14, EndCoord: 1 << 14},
	}}
	tests := []struct {
		coords []Coord
		want   float32
	}{
		{nil, 0},
		{[]Coord{0}, 0},
		{[]Coord{1 << 14}, 0},
		{[]Coord{0, 0}, 0},
		{[]Coord{0, 1 << 13}, 0.5},
		{[]Coord{0, 1 << 14}, 1},
		{[]Coord{0, 1 << 14, 1 << 14}, 1},
	}
	for _, test := range tests {
		if got := region.Evaluate(test.coords); got != test.want {
			t.Errorf("Evaluate(%v) = %v, want %v", test.coords, got, test.want)
		}
	}
}

// TestSegmentMapsRounding checks that Map rounds the interpolation as skrifa
// does: in 16.16, with the result rounded half up to 2.14 (D65).
func TestSegmentMapsRounding(t *testing.T) {
	sm := SegmentMaps{AxisValueMaps: []AxisValueMap{
		{-1 << 14, -10813}, {0, 0}, {1 << 14, 10813},
	}}
	for _, test := range []struct{ in, want Coord }{
		{8192, 5407},   // 5406.5 rounds up
		{-8192, -5406}, // -5406.5 rounds up
		{4096, 2703},   // 2703.25
		{1 << 14, 10813},
		{0, 0},
	} {
		if got := sm.Map(test.in); got != test.want {
			t.Errorf("Map(%d) = %d, want %d", test.in, got, test.want)
		}
	}
}

// TestParseGlyfOutOfRange checks that a glyph whose range of loca is out of
// the glyf table has no outline, and that ParseGlyf does not panic (D65).
func TestParseGlyfOutOfRange(t *testing.T) {
	glyf, err := ParseGlyf(make([]byte, 4), []uint32{0, 0, 100, 2})
	if err != nil {
		t.Fatal(err)
	}
	for i, g := range glyf {
		if g.Data != nil {
			t.Errorf("the glyph %d has the data %v", i, g.Data)
		}
	}
}

// TestHmtxWithoutLongMetric checks that a hmtx table without a long metric
// gives the advance 0, and that ParseHmtx rejects a negative count (D65).
func TestHmtxWithoutLongMetric(t *testing.T) {
	hmtx, _, err := ParseHmtx([]byte{0, 1, 0, 2}, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := hmtx.Advance(1); got != 0 {
		t.Errorf("the advance is %d, want 0", got)
	}
	if _, _, err := ParseHmtx([]byte{0, 1, 0, 2}, 1, -1); err == nil {
		t.Error("a negative count gives no error")
	}
}
