// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import "testing"

// The tests of svgtypes src/color.rs.

func TestParseColor(t *testing.T) {
	tests := []struct {
		name string
		text string
		want Color
	}{
		{"rrggbb", "#ff0000", NewColorRGB(255, 0, 0)},
		{"rrggbb_upper", "#FF0000", NewColorRGB(255, 0, 0)},
		{"rgb_hex", "#f00", NewColorRGB(255, 0, 0)},
		{"rrggbbaa", "#ff0000ff", NewColorRGBA(255, 0, 0, 255)},
		{"rrggbbaa_upper", "#FF0000FF", NewColorRGBA(255, 0, 0, 255)},
		{"rgba_hex", "#f00f", NewColorRGBA(255, 0, 0, 255)},
		{"rrggbb_spaced", "  #ff0000  ", NewColorRGB(255, 0, 0)},
		{"rgb_numeric", "rgb(254, 203, 231)", NewColorRGB(254, 203, 231)},
		{"rgb_numeric_spaced", " rgb( 77 , 77 , 77 ) ", NewColorRGB(77, 77, 77)},
		{"rgb_percentage", "rgb(50%, 50%, 50%)", NewColorRGB(128, 128, 128)},
		{"rgb_percentage_overflow", "rgb(140%, -10%, 130%)", NewColorRGB(255, 0, 255)},
		{"rgb_percentage_float", "rgb(33.333%,46.666%,93.333%)", NewColorRGB(85, 119, 238)},
		{"rgb_numeric_upper_case", "RGB(254, 203, 231)", NewColorRGB(254, 203, 231)},
		{"rgb_numeric_mixed_case", "RgB(254, 203, 231)", NewColorRGB(254, 203, 231)},
		{"rgb_numeric_red_float", "rgb(3.141592653, 110, 201)", NewColorRGB(3, 110, 201)},
		{"rgb_numeric_green_float", "rgb(254, 150.829521289232389, 210)", NewColorRGB(254, 151, 210)},
		{"rgb_numeric_blue_float", "rgb(96, 255, 0.2)", NewColorRGB(96, 255, 0)},
		{"rgb_numeric_all_float", "rgb(0.0, 129.82, 231.092)", NewColorRGB(0, 130, 231)},
		{"rgb_numeric_all_float_with_alpha", "rgb(0.0, 129.82, 231.092, 0.5)", NewColorRGBA(0, 130, 231, 128)},
		{"rgb_numeric_all_float_overflow", "rgb(290.2, 255.9, 300.0)", NewColorRGB(255, 255, 255)},
		{"name_red", "red", NewColorRGB(255, 0, 0)},
		{"name_red_spaced", " red ", NewColorRGB(255, 0, 0)},
		{"name_red_upper_case", "RED", NewColorRGB(255, 0, 0)},
		{"name_red_mixed_case", "ReD", NewColorRGB(255, 0, 0)},
		{"name_cornflowerblue", "cornflowerblue", NewColorRGB(100, 149, 237)},
		{"transparent", "transparent", NewColorRGBA(0, 0, 0, 0)},
		{"rgba_half", "rgba(10, 20, 30, 0.5)", NewColorRGBA(10, 20, 30, 128)},
		{"rgba_half_no_comma", "rgba(10 20 30 0.5)", NewColorRGBA(10, 20, 30, 128)},
		{"rgba_half_slash", "rgba(10 20 30 / 0.5)", NewColorRGBA(10, 20, 30, 128)},
		{"rgba_numeric_red_float", "rgba(3.141592653, 110, 201, 1.0)", NewColorRGBA(3, 110, 201, 255)},
		{"rgba_numeric_all_float", "rgba(0.0, 129.82, 231.092, 1.5)", NewColorRGBA(0, 130, 231, 255)},
		{"rgba_negative", "rgba(10, 20, 30, -2)", NewColorRGBA(10, 20, 30, 0)},
		{"rgba_large_alpha", "rgba(10, 20, 30, 2)", NewColorRGBA(10, 20, 30, 255)},
		{"rgb_with_alpha", "rgb(10, 20, 30, 0.5)", NewColorRGBA(10, 20, 30, 128)},
		{"hsl_green", "hsl(120, 100%, 75%)", NewColorRGBA(128, 255, 128, 255)},
		{"hsl_yellow", "hsl(60, 100%, 50%)", NewColorRGBA(255, 255, 0, 255)},
		{"hsl_hue_360", "hsl(360, 100%, 100%)", NewColorRGBA(255, 255, 255, 255)},
		{"hsl_out_of_bounds", "hsl(800, 150%, -50%)", NewColorRGBA(0, 0, 0, 255)},
		{"hsla_green", "hsla(120, 100%, 75%, 0.5)", NewColorRGBA(128, 255, 128, 128)},
		{"hsl_with_alpha", "hsl(120, 100%, 75%, 0.5)", NewColorRGBA(128, 255, 128, 128)},
		{"hsl_with_alpha_slash", "hsl(120 100% 75% / 0.5)", NewColorRGBA(128, 255, 128, 128)},
		{"hsl_to_rgb_red_round_up", "hsl(230, 57%, 54%)", NewColorRGBA(71, 93, 205, 255)},
		{"hsl_with_hue_float", "hsl(120.152, 100%, 75%)", NewColorRGBA(128, 255, 128, 255)},
		{"hsla_with_hue_float", "hsla(120.152, 100%, 75%, 0.5)", NewColorRGBA(128, 255, 128, 128)},
		{"hsl_hue_1turn", "hsl(1turn, 100%, 100%)", NewColorRGBA(255, 255, 255, 255)},
		{"hsl_with_hue_turn_and_percentages", "hsl(0.25turn, 100%, 50%)", NewColorRGBA(128, 255, 0, 255)},
		{"hsl_with_hue_deg_and_percentages", "hsl(90deg, 100%, 50%)", NewColorRGBA(128, 255, 0, 255)},
		{"hsl_with_hue_deg_and_numbers", "hsl(90deg, 1, 0.5)", NewColorRGBA(128, 255, 0, 255)},
		{"hsla_with_hue_deg_percentages", "hsla(120deg, 75%, 75%, 100%)", NewColorRGBA(143, 239, 143, 255)},
		{"hsla_with_hue_deg_above_360", "hsla(480deg, 75%, 75%, 100%)", NewColorRGBA(143, 239, 143, 255)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseColor(tt.text)
			if err != nil || got != tt.want {
				t.Errorf("ParseColor(%q) = %v, %v, want %v", tt.text, got, err, tt.want)
			}
		})
	}
}

func TestParseColorErr(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"not_a_color_1", "text", "invalid value"},
		{"icc_color_not_supported_1", "#CD853F icc-color(acmecmyk, 0.11, 0.48, 0.83, 0.00)", "unexpected data at position 9"},
		{"icc_color_not_supported_2", "red icc-color(acmecmyk, 0.11, 0.48, 0.83, 0.00)", "unexpected data at position 5"},
		{"invalid_input_1", "rgb(-0\r", "unexpected end of stream"},
		{"invalid_input_2", "#9ߞpx! ;", "invalid value"},
		{"rgba_with_percent_alpha", "rgba(10, 20, 30, 5%)", "expected ')' not '%' at position 19"},
		{"rgb_mixed_units", "rgb(140%, -10mm, 130pt)", "invalid number at position 14"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseColor(tt.text)
			errText(t, err, tt.want)
		})
	}
}
