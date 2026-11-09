// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import "testing"

// The tests of svgtypes src/viewbox.rs.

func TestParseViewBox(t *testing.T) {
	got, err := ParseViewBox("-20 30 100 500")
	if want := NewViewBox(-20.0, 30.0, 100.0, 500.0); err != nil || got != want {
		t.Errorf("got %v, %v, want %v", got, err, want)
	}
}

func TestParseViewBoxErr(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"parse_err_1", "qwe", "viewBox contains an invalid number"},
		{"parse_err_2", "10 20 30 0", "viewBox has a negative or zero size"},
		{"parse_err_3", "10 20 0 40", "viewBox has a negative or zero size"},
		{"parse_err_4", "10 20 0 0", "viewBox has a negative or zero size"},
		{"parse_err_5", "10 20 -30 0", "viewBox has a negative or zero size"},
		{"parse_err_6", "10 20 30 -40", "viewBox has a negative or zero size"},
		{"parse_err_7", "10 20 -30 -40", "viewBox has a negative or zero size"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseViewBox(tt.text)
			errText(t, err, tt.want)
		})
	}
}
