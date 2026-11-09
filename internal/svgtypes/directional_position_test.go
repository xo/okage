// Copyright 2023 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import "testing"

// The tests of svgtypes src/directional_position.rs.

func TestParseDirectionalPosition(t *testing.T) {
	tests := []struct {
		name string
		text string
		want DirectionalPosition
	}{
		{"parse_1", "left", DirectionalPositionLeft},
		{"parse_2", "right", DirectionalPositionRight},
		{"parse_3", "center", DirectionalPositionCenter},
		{"parse_4", "top", DirectionalPositionTop},
		{"parse_5", "bottom", DirectionalPositionBottom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDirectionalPosition(tt.text)
			if err != nil || got != tt.want {
				t.Errorf("ParseDirectionalPosition(%q) = %v, %v, want %v", tt.text, got, err, tt.want)
			}
		})
	}
}

func TestStreamParseDirectionalPosition(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"parse_6", "left,"},
		{"parse_7", "left ,"},
		{"parse_16", "left center"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStream(tt.text)
			if got, err := s.ParseDirectionalPosition(); err != nil || got != DirectionalPositionLeft {
				t.Errorf("got %v, %v", got, err)
			}
		})
	}
}

func TestDirectionalPositionErr1(t *testing.T) {
	s := NewStream("something")
	_, err := s.ParseDirectionalPosition()
	errText(t, err, "expected 'left', 'right', 'top', 'bottom', 'center' not 'something' at position 1")
}
