// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports tiny-skia tests/integration/skia_dash.rs.

import (
	"testing"

	"github.com/xo/okage"
)

func TestSkiaDashCrbug140642(t *testing.T) {
	// We used to see this construct, and due to rounding as we accumulated
	// our length, the loop where we apply the phase would run off the end of
	// the array, since it relied on just -= each interval value, which did not
	// behave as "expected". Now the code explicitly checks for walking off the
	// end of that array.
	//
	// A different (better) fix might be to rewrite dashing to do all of its
	// length/phase/measure math using double, but this may need to be
	// coordinated with SkPathMeasure, to be consistent between the two.
	if _, ok := okage.NewStrokeDash([]float32{27734, 35660, 2157846850, 247}, -248.135982067); !ok {
		t.Error("NewStrokeDash returned None, want Some")
	}
}

func TestSkiaDashCrbug124652(t *testing.T) {
	// http://code.google.com/p/chromium/issues/detail?id=124652
	// This particular test/bug only applies to the float case, where
	// large values can "swamp" small ones.
	if _, ok := okage.NewStrokeDash([]float32{837099584, 33450}, -10); !ok {
		t.Error("NewStrokeDash returned None, want Some")
	}
}

// Extremely large path_length/dash_length ratios may cause infinite looping
// due to single precision rounding.
func TestSkiaDashInfiniteDash(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(0, 5)
	pb.LineTo(5000000, 5)
	path := must(pb.Finish())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	stroke := okage.DefaultStroke()
	stroke.Dash, _ = okage.NewStrokeDash([]float32{0.2, 0.2}, 0)

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil) // Doesn't draw anything.
}
