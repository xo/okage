// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"testing"

	"github.com/xo/okage/path"
)

// TestChopCubicAtYExtrema1 is chop_cubic_at_y_extrema_1 in the tests of
// tiny-skia src/path_geometry.rs.
func TestChopCubicAtYExtrema1(t *testing.T) {
	src := [4]Point{
		path.PointFromXY(10.0, 20.0),
		path.PointFromXY(67.0, 437.0),
		path.PointFromXY(298.0, 213.0),
		path.PointFromXY(401.0, 214.0),
	}

	var dst [10]Point
	n := chopCubicAtYExtrema(&src, &dst)
	if n != 2 {
		t.Fatalf("n = %d, want 2", n)
	}
	want := [10]Point{
		path.PointFromXY(10.0, 20.0),
		path.PointFromXY(37.508274, 221.24475),
		path.PointFromXY(105.541855, 273.19803),
		path.PointFromXY(180.15599, 273.19803),
		path.PointFromXY(259.80502, 273.19803),
		path.PointFromXY(346.9527, 213.99666),
		path.PointFromXY(400.30844, 213.99666),
		path.PointFromXY(400.53958, 213.99666),
		path.PointFromXY(400.7701, 213.99777),
		path.PointFromXY(401.0, 214.0),
	}
	for i := range dst {
		if dst[i] != want[i] {
			t.Errorf("dst[%d] = %v, want %v", i, dst[i], want[i])
		}
	}
}
