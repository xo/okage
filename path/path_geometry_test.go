// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import (
	"slices"
	"testing"
)

// TestEvalCubicAt1 is the test eval_cubic_at_1 in tiny-skia-path
// path/src/path_geometry.rs.
func TestEvalCubicAt1(t *testing.T) {
	src := [4]Point{
		PointFromXY(30.0, 40.0),
		PointFromXY(30.0, 40.0),
		PointFromXY(171.0, 45.0),
		PointFromXY(180.0, 155.0),
	}

	if got, want := evalCubicPosAt(&src, NormalizedF32Zero), PointFromXY(30.0, 40.0); got != want {
		t.Errorf("evalCubicPosAt = %+v, want %+v", got, want)
	}
	if got, want := evalCubicTangentAt(&src, NormalizedF32Zero), PointFromXY(141.0, 5.0); got != want {
		t.Errorf("evalCubicTangentAt = %+v, want %+v", got, want)
	}
}

// TestFindCubicMaxCurvature1 is the test find_cubic_max_curvature_1 in
// tiny-skia-path path/src/path_geometry.rs.
func TestFindCubicMaxCurvature1(t *testing.T) {
	src := [4]Point{
		PointFromXY(20.0, 160.0),
		PointFromXY(20.0001, 160.0),
		PointFromXY(160.0, 20.0),
		PointFromXY(160.0001, 20.0),
	}

	var buf [3]NormalizedF32
	got := FindCubicMaxCurvature(&src, &buf)

	want := []NormalizedF32{
		NormalizedF32Zero,
		NormalizedF32Clamped(0.5),
		NormalizedF32One,
	}
	if !slices.Equal(got, want) {
		t.Errorf("FindCubicMaxCurvature = %v, want %v", got, want)
	}
}
