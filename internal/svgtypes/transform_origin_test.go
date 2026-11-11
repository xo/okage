// Copyright 2023 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import "testing"

// The tests of svgtypes src/transform_origin.rs.

func TestParseTransformOrigin(t *testing.T) {
	pct := func(n float64) Length { return NewLength(n, LengthUnitPercent) }
	px := func(n float64) Length { return NewLength(n, LengthUnitPx) }
	tests := []struct {
		name string
		text string
		want TransformOrigin
	}{
		{"parse_1", "center", NewTransformOrigin(pct(50.0), pct(50.0), px(0.0))},
		{"parse_2", "left", NewTransformOrigin(pct(0.0), pct(50.0), px(0.0))},
		{"parse_3", "right", NewTransformOrigin(pct(100.0), pct(50.0), px(0.0))},
		{"parse_4", "top", NewTransformOrigin(pct(50.0), pct(0.0), px(0.0))},
		{"parse_5", "bottom", NewTransformOrigin(pct(50.0), pct(100.0), px(0.0))},
		{"parse_6", "30px", NewTransformOrigin(px(30.0), pct(50.0), px(0.0))},
		{"parse_7", "center left", NewTransformOrigin(pct(0.0), pct(50.0), px(0.0))},
		{"parse_8", "left center", NewTransformOrigin(pct(0.0), pct(50.0), px(0.0))},
		{"parse_9", "center bottom", NewTransformOrigin(pct(50.0), pct(100.0), px(0.0))},
		{"parse_10", "bottom center", NewTransformOrigin(pct(50.0), pct(100.0), px(0.0))},
		{"parse_11", "30%, center", NewTransformOrigin(pct(30.0), pct(50.0), px(0.0))},
		{"parse_12", " center, 30%", NewTransformOrigin(pct(50.0), pct(30.0), px(0.0))},
		{"parse_13", "left top", NewTransformOrigin(pct(0.0), pct(0.0), px(0.0))},
		{"parse_14", "center right 3px", NewTransformOrigin(pct(100.0), pct(50.0), px(3.0))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTransformOrigin(tt.text)
			if err != nil || got != tt.want {
				t.Errorf("ParseTransformOrigin(%q) = %v, %v, want %v", tt.text, got, err, tt.want)
			}
		})
	}
}

func TestParseTransformOriginErr(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"parse_err_1", "", "transform origin doesn't have enough parameters"},
		{"parse_err_2", "some", "transform origin has invalid parameters"},
		{"parse_err_3", "center some", "transform origin has invalid parameters"},
		{"parse_err_4", "left right", "transform origin has invalid parameters"},
		{"parse_err_5", "left top 3%", "z-index cannot be a percentage"},
		{"parse_err_6", " ", "transform origin doesn't have enough parameters"},
		{"parse_err_7", "-", "transform origin has invalid parameters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTransformOrigin(tt.text)
			errText(t, err, tt.want)
		})
	}
}
