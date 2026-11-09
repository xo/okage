// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import "testing"

// The tests of svgtypes src/enable_background.rs.

func TestParseEnableBackground(t *testing.T) {
	tests := []struct {
		name string
		text string
		want EnableBackground
	}{
		{"parse_1", "accumulate", EnableBackground{Kind: EnableBackgroundAccumulate}},
		{"parse_2", "  accumulate  ", EnableBackground{Kind: EnableBackgroundAccumulate}},
		{"parse_3", "new", EnableBackground{Kind: EnableBackgroundNew}},
		{"parse_4", "  new  ", EnableBackground{Kind: EnableBackgroundNew}},
		{"parse_5", "new 1 2 3 4", EnableBackground{Kind: EnableBackgroundNewWithRegion, X: 1.0, Y: 2.0, Width: 3.0, Height: 4.0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEnableBackground(tt.text)
			if err != nil || got != tt.want {
				t.Errorf("ParseEnableBackground(%q) = %v, %v, want %v", tt.text, got, err, tt.want)
			}
		})
	}
}

func TestParseEnableBackgroundErr(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"err_1", " accumulate b ", "unexpected data at position 13"},
		{"err_2", " new b ", "invalid number at position 6"},
		{"err_3", "new 1 2 3", "unexpected end of stream"},
		{"err_4", "new 1 2 3 4 5", "unexpected data at position 13"},
		{"err_5", "new 0 0 0 0", "invalid value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseEnableBackground(tt.text)
			errText(t, err, tt.want)
		})
	}
}
