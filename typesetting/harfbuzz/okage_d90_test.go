// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package harfbuzz

import "testing"

// okage: the tests of D90. test/harfbuzz compares the glyphs with HarfBuzz
// at other scales.

// TestEmMult compares emMult with values of em_mult of HarfBuzz 15.1.0,
// (v * (scale * 0x10000 / upem) + 32768) >> 16. The values with a note
// differ from the division of go-text, which rounded toward 0.
func TestEmMult(t *testing.T) {
	for _, tt := range []struct {
		v, scale, upem int32
		want           Position
	}{
		{2, 768, 2048, 1}, // go-text: 0
		{-1, 768, 2048, 0},
		{-3, 768, 2048, -1},
		{-68, 768, 2048, -25},
		{-301, 1000, 2048, -147}, // go-text: -146
		{438, 768, 1000, 336},
		{100000, 768, 2048, 37500},
		{-100000, 1000, 2048, -48828},
		{32767, 1500, 1000, 49151}, // go-text: 49150
		{1234, 2048, 2048, 1234},
		{-1234, 1000, 1000, -1234},
		{5, 0, 1000, 0},
	} {
		if got := emMult(tt.v, emMultiplier(tt.scale, tt.upem)); got != tt.want {
			t.Errorf("emMult(%d, %d, %d): got %d, want %d", tt.v, tt.scale, tt.upem, got, tt.want)
		}
	}

	// A Font keeps the multiplier of its scale, and computes it again
	// when the scale changes.
	f := &Font{faceUpem: 2048}
	for _, tt := range []struct {
		scale int32
		want  Position
	}{{0, 0}, {768, 1}, {768, 1}, {2048, 2}, {4096, 4}, {0, 0}} {
		f.XScale, f.YScale = tt.scale, tt.scale
		if x, y := f.emScaleX(2), f.emScaleY(2); x != tt.want || y != tt.want {
			t.Errorf("scale %d: got %d and %d, want %d", tt.scale, x, y, tt.want)
		}
	}
}
