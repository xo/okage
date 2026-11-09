// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import "testing"

// The tests of svgtypes src/angle.rs.

func TestParseAngle(t *testing.T) {
	tests := []struct {
		name string
		text string
		want Angle
	}{
		{"parse_1", "1", NewAngle(1.0, AngleUnitDegrees)},
		{"parse_2", "1deg", NewAngle(1.0, AngleUnitDegrees)},
		{"parse_3", "1grad", NewAngle(1.0, AngleUnitGradians)},
		{"parse_4", "1rad", NewAngle(1.0, AngleUnitRadians)},
		{"parse_5", "1turn", NewAngle(1.0, AngleUnitTurns)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAngle(tt.text)
			if err != nil || got != tt.want {
				t.Errorf("ParseAngle(%q) = %v, %v, want %v", tt.text, got, err, tt.want)
			}
		})
	}
}

func TestAngleErr1(t *testing.T) {
	s := NewStream("1q")
	if got, err := s.ParseAngle(); err != nil || got != NewAngle(1.0, AngleUnitDegrees) {
		t.Errorf("got %v, %v", got, err)
	}
	_, err := s.ParseAngle()
	errText(t, err, "invalid number at position 2")
}

func TestAngleErr2(t *testing.T) {
	_, err := ParseAngle("1degq")
	errText(t, err, "unexpected data at position 5")
}
