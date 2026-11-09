// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import (
	"strconv"
	"testing"
)

// The tests of svgtypes src/transform.rs.

// displayF64 is the Display of f64 in Rust: the shortest text that parses
// back to the same value, with no exponent.
func displayF64(x float64) string {
	return strconv.FormatFloat(x, 'f', -1, 64)
}

func TestParseTransform(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"parse_1", "matrix(1 0 0 1 10 20)", "matrix(1 0 0 1 10 20)"},
		{"parse_2", "translate(10 20)", "matrix(1 0 0 1 10 20)"},
		{"parse_3", "scale(2 3)", "matrix(2 0 0 3 0 0)"},
		{"parse_4", "rotate(30)", "matrix(0.8660254037844387 0.49999999999999994 -0.49999999999999994 0.8660254037844387 0 0)"},
		{"parse_5", "rotate(30 10 20)", "matrix(0.8660254037844387 0.49999999999999994 -0.49999999999999994 0.8660254037844387 11.339745962155611 -2.3205080756887746)"},
		{"parse_6", "translate(10 15) translate(0 5)", "matrix(1 0 0 1 10 20)"},
		{"parse_7", "translate(10) scale(2)", "matrix(2 0 0 2 10 0)"},
		{"parse_8", "translate(25 215) scale(2) skewX(45)", "matrix(2 0 1.9999999999999998 2 25 215)"},
		{"parse_9", "skewX(45)", "matrix(1 0 0.9999999999999999 1 0 0)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := ParseTransform(tt.text)
			if err != nil {
				t.Fatal(err)
			}
			s := "matrix(" + displayF64(ts.A) + " " + displayF64(ts.B) + " " + displayF64(ts.C) + " " +
				displayF64(ts.D) + " " + displayF64(ts.E) + " " + displayF64(ts.F) + ")"
			if s != tt.want {
				t.Errorf("got %s, want %s", s, tt.want)
			}
		})
	}
}

func TestParseTransformErr(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"parse_err_1", "text", "unexpected end of stream"},
		{"parse_err_3", "???G", "expected '(' not '?' at position 1"},
		{"parse_err_6", "rect()", "unexpected data at position 1"},
		{"parse_err_7", "scale(2) rect()", "unexpected data at position 10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseTransform(tt.text)
			errText(t, err, tt.want)
		})
	}
}

func TestTransformParseErr2(t *testing.T) {
	ts := NewTransformListParser("scale(2) text")
	if _, ok, _ := ts.Next(); !ok {
		t.Fatal("got the end")
	}
	_, ok, err := ts.Next()
	if !ok {
		t.Fatal("got the end")
	}
	errText(t, err, "unexpected end of stream")
}

func TestTransformParseErr4(t *testing.T) {
	ts := NewTransformListParser(" ")
	if _, ok, _ := ts.Next(); ok {
		t.Error("got a value, want the end")
	}
}

func TestTransformParseErr5(t *testing.T) {
	ts := NewTransformListParser("\x01")
	if _, ok, err := ts.Next(); !ok || err == nil {
		t.Errorf("got %v, %v, want an error", ok, err)
	}
}

// TestTransformListParser is the example of the documentation of
// TransformListParser in svgtypes src/transform.rs.
func TestTransformListParser(t *testing.T) {
	p := NewTransformListParser("scale(2) translate(10, -20)")
	if v, ok, err := p.Next(); !ok || err != nil || v != (TransformListToken{Kind: TransformListTokenScale, Sx: 2.0, Sy: 2.0}) {
		t.Errorf("got %v, %v, %v", v, ok, err)
	}
	if v, ok, err := p.Next(); !ok || err != nil || v != (TransformListToken{Kind: TransformListTokenTranslate, Tx: 10.0, Ty: -20.0}) {
		t.Errorf("got %v, %v, %v", v, ok, err)
	}
	if _, ok, _ := p.Next(); ok {
		t.Error("got a value, want the end")
	}
}
