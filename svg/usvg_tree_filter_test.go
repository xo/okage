// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "testing"

// usvg src/tree/filter.rs has no tests.

func TestKindHasInput(t *testing.T) {
	ref := InputFilter{Kind: InputFilterReference, Reference: "a"}
	graphic := InputFilter{Kind: InputFilterSourceGraphic}
	alpha := InputFilter{Kind: InputFilterSourceAlpha}
	tests := []struct {
		kind KindFilter
		in   InputFilter
		want bool
	}{
		{&BlendFilter{input1: graphic, input2: ref}, ref, true},
		{&BlendFilter{input1: graphic, input2: ref}, alpha, false},
		{&OffsetFilter{input: ref}, InputFilter{Kind: InputFilterReference, Reference: "b"}, false},
		{&MergeFilter{inputs: []InputFilter{alpha, ref}}, ref, true},
		{&FloodFilter{}, graphic, false},
		{&ImageFilter{}, graphic, false},
		{&TurbulenceFilter{}, graphic, false},
		{&GaussianBlurFilter{}, graphic, true},
	}
	for i, test := range tests {
		if got := KindFilterHasInput(test.kind, test.in); got != test.want {
			t.Errorf("%d: KindHasInput(%T, %v) = %v, want %v", i, test.kind, test.in, got, test.want)
		}
	}
}

func TestConvolveMatrixData(t *testing.T) {
	if _, ok := newConvolveMatrixDataFilter(0, 0, 2, 2, []float32{1, 2, 3}); ok {
		t.Error("newConvolveMatrixData accepted a short matrix")
	}
	if _, ok := newConvolveMatrixDataFilter(2, 0, 2, 2, []float32{1, 2, 3, 4}); ok {
		t.Error("newConvolveMatrixData accepted targetX == columns")
	}
	if _, ok := newConvolveMatrixDataFilter(0, 2, 2, 2, []float32{1, 2, 3, 4}); ok {
		t.Error("newConvolveMatrixData accepted targetY == rows")
	}
	if _, ok := newConvolveMatrixDataFilter(0, 0, 1, 1, []float32{1}); !ok {
		t.Error("newConvolveMatrixData failed for a 1x1 matrix")
	}
	d, ok := newConvolveMatrixDataFilter(1, 0, 3, 2, []float32{1, 2, 3, 4, 5, 6})
	if !ok {
		t.Fatal("newConvolveMatrixData failed")
	}
	if d.Get(2, 1) != 6 || d.Get(0, 1) != 4 || d.Columns() != 3 || d.Rows() != 2 || d.TargetX() != 1 {
		t.Error("ConvolveMatrixData is wrong")
	}
}

func TestDefaultColorMatrixKind(t *testing.T) {
	m, ok := DefaultColorMatrixKindFilter().(ColorMatrixKindFilterMatrix)
	if !ok || len(m) != 20 || m[0] != 1 || m[6] != 1 || m[12] != 1 || m[18] != 1 {
		t.Errorf("DefaultColorMatrixKind() = %v", m)
	}
	if DefaultColorInterpolationFilter() != ColorInterpolationFilterLinearRGB {
		t.Error("DefaultColorInterpolation is wrong")
	}
}
