// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import "testing"

// The tests of svgtypes src/paint.rs.

func TestParsePaint(t *testing.T) {
	tests := []struct {
		name string
		text string
		want Paint
	}{
		{"parse_1", "none", Paint{Kind: PaintNone}},
		{"parse_2", "  none   ", Paint{Kind: PaintNone}},
		{"parse_3", " inherit ", Paint{Kind: PaintInherit}},
		{"parse_4", " currentColor ", Paint{Kind: PaintCurrentColor}},
		{"parse_5", " red ", Paint{Kind: PaintColor, Color: ColorRed()}},
		{"parse_6", " url(#qwe) ", Paint{Kind: PaintFuncIRI, FuncIRI: "qwe"}},
		{"parse_7", " url(#qwe) none ", Paint{Kind: PaintFuncIRI, FuncIRI: "qwe", Fallback: PaintFallback{Kind: PaintFallbackNone}, HasFallback: true}},
		{"parse_8", " url(#qwe) currentColor ", Paint{Kind: PaintFuncIRI, FuncIRI: "qwe", Fallback: PaintFallback{Kind: PaintFallbackCurrentColor}, HasFallback: true}},
		{"parse_9", " url(#qwe) red ", Paint{Kind: PaintFuncIRI, FuncIRI: "qwe", Fallback: PaintFallback{Kind: PaintFallbackColor, Color: ColorRed()}, HasFallback: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePaint(tt.text)
			if err != nil || got != tt.want {
				t.Errorf("ParsePaint(%q) = %v, %v, want %v", tt.text, got, err, tt.want)
			}
		})
	}
}

func TestParsePaintErr(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"parse_err_1", "qwe", "invalid value"},
		{"parse_err_2", "red icc-color(acmecmyk, 0.11, 0.48, 0.83, 0.00)", "invalid value"},
		// TODO: this
		// test_err!(parse_err_3, "url(#qwe) red icc-color(acmecmyk, 0.11, 0.48, 0.83, 0.00)", "invalid color at 1:15");
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePaint(tt.text)
			errText(t, err, tt.want)
		})
	}
}
