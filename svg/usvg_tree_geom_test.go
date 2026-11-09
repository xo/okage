// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"math"
	"testing"

	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// usvg src/tree/geom.rs has no tests. The first tests port the tests of
// ApproxEqUlps in the crate float-cmp 0.9.0, src/ulps_eq.rs.

func TestApproxEqUlps32(t *testing.T) {
	// f32_approx_eq_ulps_test1
	f := float32(0.1)
	var sum float32
	for range 10 {
		sum += f
	}
	product := f * 10.0
	if sum == product {
		t.Fatal("sum == product")
	}
	if !ApproxEqUlps32(sum, product, 1) || ApproxEqUlps32(sum, product, 0) {
		t.Error("f32_approx_eq_ulps_test1 failed")
	}

	// f32_approx_eq_ulps_test2
	x := float32(1000000)
	y := float32(1000000.1)
	if x == y {
		t.Fatal("x == y")
	}
	if !ApproxEqUlps32(x, y, 2) || ApproxEqUlps32(x, y, 1) {
		t.Error("f32_approx_eq_ulps_test2 failed")
	}

	// f32_approx_eq_ulps_test_zeroes
	if !ApproxEqUlps32(0.0, float32(math.Copysign(0, -1)), 0) {
		t.Error("f32_approx_eq_ulps_test_zeroes failed")
	}
}

func TestApproxEqUlps64(t *testing.T) {
	// f64_approx_eq_ulps_test1
	f := 0.1
	var sum float64
	for range 10 {
		sum += f
	}
	product := f * 10.0
	if sum == product {
		t.Fatal("sum == product")
	}
	if !ApproxEqUlps64(sum, product, 1) || ApproxEqUlps64(sum, product, 0) {
		t.Error("f64_approx_eq_ulps_test1 failed")
	}

	// f64_approx_eq_ulps_test2
	x := 1000000.0
	y := 1000000.0000000003
	if x == y {
		t.Fatal("x == y")
	}
	if !ApproxEqUlps64(x, y, 3) || ApproxEqUlps64(x, y, 2) {
		t.Error("f64_approx_eq_ulps_test2 failed")
	}

	// f64_approx_eq_ulps_test_zeroes
	if !ApproxEqUlps64(0.0, math.Copysign(0, -1), 0) {
		t.Error("f64_approx_eq_ulps_test_zeroes failed")
	}
}

func TestApproxZeroUlps(t *testing.T) {
	if !ApproxZeroUlps32(math.Float32frombits(4), 4) || ApproxZeroUlps32(math.Float32frombits(5), 4) {
		t.Error("ApproxZeroUlps32 is wrong")
	}
	if !ApproxZeroUlps64(math.Float64frombits(4), 4) || ApproxZeroUlps64(-1e-300, 4) {
		t.Error("ApproxZeroUlps64 is wrong")
	}
}

func TestIsValidLength(t *testing.T) {
	inf := math.Inf(1)
	for _, x := range []float64{0, -1, inf, math.NaN()} {
		if isValidLength32(float32(x)) || isValidLength64(x) {
			t.Errorf("isValidLength(%v) = true", x)
		}
	}
	if !isValidLength32(0.5) || !isValidLength64(0.5) {
		t.Error("isValidLength(0.5) = false")
	}
}

func TestViewBoxToTransform(t *testing.T) {
	rect, _ := path.NonZeroRectFromXYWH(10, 20, 100, 50)
	size, _ := path.SizeFromWH(200, 200)
	tests := []struct {
		aspect svgtypes.AspectRatio
		want   Transform
	}{
		{svgtypes.AspectRatio{Align: svgtypes.AlignNone}, path.TransformFromRow(2, 0, 0, 4, -20, -80)},
		{svgtypes.AspectRatio{Align: svgtypes.AlignXMidYMid}, path.TransformFromRow(2, 0, 0, 2, -20, 10)},
		{svgtypes.AspectRatio{Align: svgtypes.AlignXMaxYMax}, path.TransformFromRow(2, 0, 0, 2, -20, 60)},
		{svgtypes.AspectRatio{Align: svgtypes.AlignXMinYMin, Slice: true}, path.TransformFromRow(4, 0, 0, 4, -40, -80)},
		{svgtypes.AspectRatio{Align: svgtypes.AlignXMidYMid, Slice: true}, path.TransformFromRow(4, 0, 0, 4, -140, -80)},
	}
	for _, test := range tests {
		vb := viewBox{rect: rect, aspect: test.aspect}
		if got := vb.toTransform(size); got != test.want {
			t.Errorf("%+v: got %v, want %v", test.aspect, got, test.want)
		}
	}
}

func TestBBox(t *testing.T) {
	b := defaultBBox()
	if !b.isDefault() {
		t.Fatal("defaultBBox is not default")
	}
	if _, ok := b.toRect(); ok {
		t.Error("toRect of the default succeeded")
	}
	if _, ok := b.toNonZeroRect(); ok {
		t.Error("toNonZeroRect of the default succeeded")
	}
	r1, _ := path.RectFromXYWH(0, 0, 10, 0)
	b = b.expand(bboxFromRect(r1))
	if r, ok := b.toRect(); !ok || r != r1 {
		t.Errorf("toRect() = %v, %v", r, ok)
	}
	if _, ok := b.toNonZeroRect(); ok {
		t.Error("toNonZeroRect of a zero height succeeded")
	}
	r2, _ := path.NonZeroRectFromXYWH(-5, 5, 10, 10)
	b = b.expand(bboxFromNonZeroRect(r2))
	want, _ := path.NonZeroRectFromLTRB(-5, 0, 10, 15)
	if r, ok := b.toNonZeroRect(); !ok || r != want {
		t.Errorf("toNonZeroRect() = %v, %v, want %v", r, ok, want)
	}
}

func TestMinMaxF32(t *testing.T) {
	nan := float32(math.NaN())
	if minF32(nan, 1) != 1 || minF32(1, nan) != 1 || minF32(1, 2) != 1 {
		t.Error("minF32 is wrong")
	}
	if maxF32(nan, 1) != 1 || maxF32(1, nan) != 1 || maxF32(1, 2) != 2 {
		t.Error("maxF32 is wrong")
	}
}
