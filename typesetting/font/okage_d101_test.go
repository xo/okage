// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package font

// okage: this file is not in go-text. It tests the normalization of the
// coordinates of D101.

import "testing"

// TestNormalizeVariationsSkrifa checks that NormalizeVariations gives the
// coordinates of Fvar::user_to_normalized of read-fonts 0.46, which skrifa
// 0.49 calls. A small Rust program read each font with the tag
// skrifa-v0.49.0 of fontations and printed the coordinates of the first
// axis, with the other axes at their default. goText is the coordinate of
// the rule of go-text, which normalizes in float32 and rounds to 2.14
// before it maps the coordinate with avar. Each goText differs from skrifa
// by one unit of 2.14.
//
// The program also printed the coordinates of the five fonts over the range
// of the first axis, in steps of 1/8 or 1/16: 31,205 values. NormalizeVariations gives each value of skrifa, and the rule of
// go-text differs on 7,253 of them.
func TestNormalizeVariationsSkrifa(t *testing.T) {
	for _, test := range []struct {
		path   string
		user   float32
		skrifa VarCoord
		goText VarCoord
	}{
		{"toys/CFF2-VF.otf", 200.75, -16350, -16351},
		{"toys/CFF2-VF.otf", 201.375, -16323, -16322},
		{"toys/CFF2-VF.otf", 216, -15672, -15671},
		{"toys/CFF2-VF.otf", 333, -7994, -7995},
		{"common/NotoSansCJKjp-VF.otf", 101.25, 16, 17},
		{"common/NotoSansCJKjp-VF.otf", 301, 2674, 2672},
		{"common/Selawik-VF.ttf", 300.125, -16363, -16364},
		{"common/Commissioner-VF.ttf", 100.375, 5, 6},
		{"common/SourceSans-VF.ttf", 200.5, 8, 9},
		{"common/SourceSans-VF.ttf", 899.75, 16381, 16380},
	} {
		font := loadFont(t, test.path)
		coords := make([]float32, len(font.fvar))
		for i, a := range font.fvar {
			coords[i] = a.Default
		}
		coords[0] = test.user
		if got := font.NormalizeVariations(coords)[0]; got != test.skrifa {
			t.Errorf("%s at %g: the coordinate is %d, want %d", test.path, test.user, got, test.skrifa)
		}
		old := font.fvar.normalizeCoordinates(coords)[0]
		if len(font.avar.AxisSegmentMaps) != 0 {
			old = font.avar.AxisSegmentMaps[0].Map(old)
		}
		if old != test.goText {
			t.Errorf("%s at %g: the rule of go-text gives %d, want %d", test.path, test.user, old, test.goText)
		}
	}
}
