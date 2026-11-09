// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import "testing"

// The tests of svgtypes src/length.rs.

func TestParseLength(t *testing.T) {
	tests := []struct {
		name string
		text string
		want Length
	}{
		{"parse_1", "1", NewLength(1.0, LengthUnitNone)},
		{"parse_2", "1em", NewLength(1.0, LengthUnitEm)},
		{"parse_3", "1ex", NewLength(1.0, LengthUnitEx)},
		{"parse_4", "1px", NewLength(1.0, LengthUnitPx)},
		{"parse_5", "1in", NewLength(1.0, LengthUnitIn)},
		{"parse_6", "1cm", NewLength(1.0, LengthUnitCm)},
		{"parse_7", "1mm", NewLength(1.0, LengthUnitMm)},
		{"parse_8", "1pt", NewLength(1.0, LengthUnitPt)},
		{"parse_9", "1pc", NewLength(1.0, LengthUnitPc)},
		{"parse_10", "1%", NewLength(1.0, LengthUnitPercent)},
		{"parse_11", "1e0", NewLength(1.0, LengthUnitNone)},
		{"parse_12", "1.0e0", NewLength(1.0, LengthUnitNone)},
		{"parse_13", "1.0e0em", NewLength(1.0, LengthUnitEm)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLength(tt.text)
			if err != nil || got != tt.want {
				t.Errorf("ParseLength(%q) = %v, %v, want %v", tt.text, got, err, tt.want)
			}
		})
	}
}

func TestStreamParseLength(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"parse_14", "1,"},
		{"parse_15", "1 ,"},
		{"parse_16", "1 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStream(tt.text)
			if got, err := s.ParseLength(); err != nil || got != NewLength(1.0, LengthUnitNone) {
				t.Errorf("got %v, %v", got, err)
			}
		})
	}
}

func TestLengthErr1(t *testing.T) {
	s := NewStream("1q")
	if got, err := s.ParseLength(); err != nil || got != NewLength(1.0, LengthUnitNone) {
		t.Errorf("got %v, %v", got, err)
	}
	_, err := s.ParseLength()
	errText(t, err, "invalid number at position 2")
}

func TestLengthErr2(t *testing.T) {
	_, err := ParseLength("1mmx")
	errText(t, err, "unexpected data at position 4")
}

// TestLengthListParser is the example of the documentation of
// LengthListParser in svgtypes src/length.rs.
func TestLengthListParser(t *testing.T) {
	p := NewLengthListParser("10px 20% 50mm")
	want := []Length{NewLength(10.0, LengthUnitPx), NewLength(20.0, LengthUnitPercent), NewLength(50.0, LengthUnitMm)}
	var got []Length
	for v, err := range p.All() {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, v)
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}
