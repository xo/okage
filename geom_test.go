// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"math"
	"testing"
)

// TestScreenIntRectTests is tests in screen_int_rect_tests of tiny-skia
// src/geom.rs.
func TestScreenIntRectTests(t *testing.T) {
	for _, c := range [][4]uint32{
		{0, 0, 0, 0},
		{0, 0, 1, 0},
		{0, 0, 0, 1},
		{0, 0, math.MaxUint32, math.MaxUint32},
		{0, 0, 1, math.MaxUint32},
		{0, 0, math.MaxUint32, 1},
		{math.MaxUint32, 0, 1, 1},
		{0, math.MaxUint32, 1, 1},
		{math.MaxUint32, math.MaxUint32, math.MaxUint32, math.MaxUint32},
	} {
		if _, ok := screenIntRectFromXYWH(c[0], c[1], c[2], c[3]); ok {
			t.Errorf("screenIntRectFromXYWH%v is ok, want not ok", c)
		}
	}

	r, ok := screenIntRectFromXYWH(1, 2, 3, 4)
	if !ok {
		t.Fatal("screenIntRectFromXYWH(1, 2, 3, 4) is not ok")
	}
	if r.X() != 1 || r.Y() != 2 || r.Width() != 3 || r.Height() != 4 || r.Right() != 4 || r.Bottom() != 6 {
		t.Errorf("got %+v", r)
	}
}
