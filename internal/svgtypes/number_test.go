// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import (
	"errors"
	"math"
	"testing"

	"github.com/xo/okage"
)

// The tests of svgtypes src/number.rs.

func TestParseNumber(t *testing.T) {
	tests := []struct {
		name string
		text string
		want float64
	}{
		{"parse_1", "0", 0.0},
		{"parse_2", "1", 1.0},
		{"parse_3", "-1", -1.0},
		{"parse_4", " -1 ", -1.0},
		{"parse_5", "  1  ", 1.0},
		{"parse_6", ".4", 0.4},
		{"parse_7", "-.4", -0.4},
		{"parse_8", "-.4text", -0.4},
		{"parse_9", "-.01 text", -0.01},
		{"parse_10", "-.01 4", -0.01},
		{"parse_11", ".0000000000008", 0.0000000000008},
		{"parse_12", "1000000000000", 1000000000000.0},
		{"parse_13", "123456.123456", 123456.123456},
		{"parse_14", "+10", 10.0},
		{"parse_15", "1e2", 100.0},
		{"parse_16", "1e+2", 100.0},
		{"parse_17", "1E2", 100.0},
		{"parse_18", "1e-2", 0.01},
		{"parse_19", "1ex", 1.0},
		{"parse_20", "1em", 1.0},
		{"parse_21", "12345678901234567890", 12345678901234567000.0},
		{"parse_22", "0.", 0.0},
		{"parse_23", "1.3e-2", 0.013},
		// test_number!(parse_24, "1e", 1.0); // TODO: this
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStream(tt.text)
			got, err := s.ParseNumber()
			if err != nil || got != tt.want {
				t.Errorf("ParseNumber(%q) = %v, %v, want %v", tt.text, got, err, tt.want)
			}
		})
	}
}

func TestParseNumberErr(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"parse_err_1", "q"},
		{"parse_err_2", ""},
		{"parse_err_3", "-"},
		{"parse_err_4", "+"},
		{"parse_err_5", "-q"},
		{"parse_err_6", "."},
		{"parse_err_7", "99999999e99999999"},
		{"parse_err_8", "-99999999e99999999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStream(tt.text)
			_, err := s.ParseNumber()
			errText(t, err, "invalid number at position 1")
		})
	}
}

// TestParseNumberForms checks the texts that the scanner passes to
// strconv.ParseFloat, where Go accepts more forms than Rust. These cases are
// not upstream tests. The values come from Rust 1.97 with svgtypes 0.16.1.
func TestParseNumberForms(t *testing.T) {
	tests := []struct {
		text string
		want float64
		ok   bool
	}{
		{"1e-400", 0, true},
		{"-1e-400", math.Copysign(0, -1), true},
		{"1e400", 0, false},
		{"1e", 0, false},
		{"1e+", 0, false},
		{"1e-x", 0, false},
		{"-.", 0, false},
		{".e5", 0, false},
		{"0x10", 0, true},
		{"1_0", 1, true},
		{"inf", 0, false},
		{"nan", 0, false},
		{"4.9e-324", 5e-324, true},
		{"2.2250738585072011e-308", 2.225073858507201e-308, true},
		{"1.7976931348623158e308", math.MaxFloat64, true},
		{"1.7976931348623159e308", 0, false},
	}
	for _, tt := range tests {
		s := NewStream(tt.text)
		got, err := s.ParseNumber()
		if tt.ok != (err == nil) || math.Float64bits(got) != math.Float64bits(tt.want) {
			t.Errorf("ParseNumber(%q) = %v, %v, want %v, ok %v", tt.text, got, err, tt.want, tt.ok)
		}
		if err != nil && !errors.Is(err, okage.ErrSVGTypesInvalidNumber) {
			t.Errorf("ParseNumber(%q) error %v, want ErrSVGTypesInvalidNumber", tt.text, err)
		}
	}
}
