// Copyright 2014 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import (
	"math"
	"testing"
)

// TestTest is the test test in tiny-skia-path path/src/dash.rs.
func TestTest(t *testing.T) {
	inf := float32(math.Inf(1))
	tests := []struct {
		array  []float32
		offset float32
	}{
		{[]float32{}, 0.0},
		{[]float32{1.0}, 0.0},
		{[]float32{1.0, 2.0, 3.0}, 0.0},
		{[]float32{1.0, -2.0}, 0.0},
		{[]float32{0.0, 0.0}, 0.0},
		{[]float32{1.0, -1.0}, 0.0},
		{[]float32{1.0, 1.0}, inf},
		{[]float32{1.0, inf}, 0.0},
	}
	for _, test := range tests {
		if _, ok := NewStrokeDash(test.array, test.offset); ok {
			t.Errorf("NewStrokeDash(%v, %v) is ok, want false", test.array, test.offset)
		}
	}
}

// TestBug26 is the test bug_26 in tiny-skia-path path/src/dash.rs.
func TestBug26(t *testing.T) {
	pb := NewPathBuilder()
	pb.MoveTo(665.54, 287.3)
	pb.LineTo(675.67, 273.04)
	pb.LineTo(675.52, 271.32)
	pb.LineTo(674.79, 269.61)
	pb.LineTo(674.05, 268.04)
	pb.LineTo(672.88, 266.47)
	pb.LineTo(671.27, 264.9)
	path, ok := pb.Finish()
	if !ok {
		t.Fatalf("Finish is not ok")
	}

	strokeDash, ok := NewStrokeDash([]float32{6.0, 4.5}, 0.0)
	if !ok {
		t.Fatalf("NewStrokeDash is not ok")
	}

	if _, ok := path.Dash(strokeDash, 1.0); !ok {
		t.Errorf("Dash is not ok, want ok")
	}
}
