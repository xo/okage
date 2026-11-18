// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

// okage: this file is not in go-text. It tests the change of D61 to
// VariationRegion.Evaluate.

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
