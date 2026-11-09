// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import "testing"

// The tests of svgtypes src/points.rs.

func TestPointsParse1(t *testing.T) {
	parser := NewPointsParser("10 20 30 40")
	if x, y, ok := parser.Next(); !ok || x != 10.0 || y != 20.0 {
		t.Errorf("got %v, %v, %v", x, y, ok)
	}
	if x, y, ok := parser.Next(); !ok || x != 30.0 || y != 40.0 {
		t.Errorf("got %v, %v, %v", x, y, ok)
	}
	if _, _, ok := parser.Next(); ok {
		t.Error("got a value, want the end")
	}
}

func TestPointsParse2(t *testing.T) {
	parser := NewPointsParser("10 20 30 40 50")
	if x, y, ok := parser.Next(); !ok || x != 10.0 || y != 20.0 {
		t.Errorf("got %v, %v, %v", x, y, ok)
	}
	if x, y, ok := parser.Next(); !ok || x != 30.0 || y != 40.0 {
		t.Errorf("got %v, %v, %v", x, y, ok)
	}
	if _, _, ok := parser.Next(); ok {
		t.Error("got a value, want the end")
	}
}
