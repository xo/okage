// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import (
	"math"
	"testing"
)

func mustIntRect(t *testing.T, x, y int32, w, h uint32) IntRect {
	t.Helper()
	r, ok := IntRectFromXYWH(x, y, w, h)
	if !ok {
		t.Fatalf("IntRectFromXYWH(%d, %d, %d, %d) is not ok", x, y, w, h)
	}
	return r
}

func mustRect(t *testing.T, l, tp, r, b float32) Rect {
	t.Helper()
	rect, ok := RectFromLTRB(l, tp, r, b)
	if !ok {
		t.Fatalf("RectFromLTRB(%v, %v, %v, %v) is not ok", l, tp, r, b)
	}
	return rect
}

// TestIntRectTests is the test tests of the module int_rect_tests in
// tiny-skia-path path/src/rect.rs.
func TestIntRectTests(t *testing.T) {
	bad := []struct {
		x, y int32
		w, h uint32
	}{
		{0, 0, 0, 0},
		{0, 0, 1, 0},
		{0, 0, 0, 1},
		{0, 0, math.MaxUint32, math.MaxUint32},
		{0, 0, 1, math.MaxUint32},
		{0, 0, math.MaxUint32, 1},
		{math.MaxInt32, 0, 1, 1},
		{0, math.MaxInt32, 1, 1},
	}
	for _, b := range bad {
		if _, ok := IntRectFromXYWH(b.x, b.y, b.w, b.h); ok {
			t.Errorf("IntRectFromXYWH(%d, %d, %d, %d) is ok, want false", b.x, b.y, b.w, b.h)
		}
	}

	{
		// No intersection.
		r1 := mustIntRect(t, 1, 2, 3, 4)
		r2 := mustIntRect(t, 11, 12, 13, 14)
		if _, ok := r1.Intersect(r2); ok {
			t.Errorf("no intersection: Intersect is ok, want false")
		}
	}

	{
		// Second inside the first one.
		r1 := mustIntRect(t, 1, 2, 30, 40)
		r2 := mustIntRect(t, 11, 12, 13, 14)
		got, ok := r1.Intersect(r2)
		if want := mustIntRect(t, 11, 12, 13, 14); !ok || got != want {
			t.Errorf("second inside: Intersect = %+v, %v, want %+v, true", got, ok, want)
		}
	}

	{
		// Partial overlap.
		r1 := mustIntRect(t, 1, 2, 30, 40)
		r2 := mustIntRect(t, 11, 12, 50, 60)
		got, ok := r1.Intersect(r2)
		if want := mustIntRect(t, 11, 12, 20, 30); !ok || got != want {
			t.Errorf("partial overlap: Intersect = %+v, %v, want %+v, true", got, ok, want)
		}
	}
}

// TestRectTests is the test tests of the module rect_tests in tiny-skia-path
// path/src/rect.rs.
func TestRectTests(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	bad := [][4]float32{
		{10.0, 10.0, 5.0, 10.0},
		{10.0, 10.0, 10.0, 5.0},
		{nan, 10.0, 10.0, 10.0},
		{10.0, nan, 10.0, 10.0},
		{10.0, 10.0, nan, 10.0},
		{10.0, 10.0, 10.0, nan},
		{10.0, 10.0, 10.0, inf},
	}
	for _, b := range bad {
		if _, ok := RectFromLTRB(b[0], b[1], b[2], b[3]); ok {
			t.Errorf("RectFromLTRB(%v) is ok, want false", b)
		}
	}

	rect := mustRect(t, 10.0, 20.0, 30.0, 40.0)
	if rect.Left() != 10.0 || rect.Top() != 20.0 || rect.Right() != 30.0 || rect.Bottom() != 40.0 {
		t.Errorf("edges = %v %v %v %v, want 10 20 30 40", rect.Left(), rect.Top(), rect.Right(), rect.Bottom())
	}
	if rect.Width() != 20.0 || rect.Height() != 20.0 {
		t.Errorf("size = %v x %v, want 20 x 20", rect.Width(), rect.Height())
	}

	rect = mustRect(t, -30.0, 20.0, -10.0, 40.0)
	if rect.Width() != 20.0 || rect.Height() != 20.0 {
		t.Errorf("size = %v x %v, want 20 x 20", rect.Width(), rect.Height())
	}
}

// TestRectTransform is the test transform of the module rect_tests in
// tiny-skia-path path/src/rect.rs. Its name differs from the Rust name,
// because transform.rs also has a test transform.
func TestRectTransform(t *testing.T) {
	// Tests based on 2x2 rectangle
	rect := mustRect(t, 1.0, 2.0, 3.0, 4.0)

	tests := []struct {
		name string
		ts   Transform
		want Rect
	}{
		{"identity", IdentityTransform(), rect},
		// Scale x by 1x, y by 2x
		{"scale", TransformFromScale(1.0, 2.0), mustRect(t, 1.0, 4.0, 3.0, 8.0)},
		// Skew box along x-axis - vertical lines at y=c go to y=c+x and
		// horizontal lines stay put. Result is bounding box
		{"skew x", TransformFromSkew(1.0, 0.0), mustRect(t, 3.0, 2.0, 7.0, 4.0)},
		// Skew box along y-axis - horizontal lines at x=c go to x=c+2y
		{"skew y", TransformFromSkew(0.0, 2.0), mustRect(t, 1.0, 4.0, 3.0, 10.0)},
	}
	for _, test := range tests {
		got, ok := rect.Transform(test.ts)
		if !ok || got != test.want {
			t.Errorf("%s: Transform = %+v, %v, want %+v, true", test.name, got, ok, test.want)
		}
	}
}
