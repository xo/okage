// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import (
	"math"
	"testing"
)

// TestBound is the test bound in tiny-skia-path path/src/scalar.rs.
func TestBound(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	tests := []struct {
		x    float32
		want float32
	}{
		{nan, 1.0},
		{inf, 1.0},
		{-inf, 0.0},
		{epsilonF32, epsilonF32},
		{0.5, 0.5},
		{-1.0, 0.0},
		{2.0, 1.0},
	}
	for _, test := range tests {
		if got := Bound(test.x, 0.0, 1.0); got != test.want {
			t.Errorf("Bound(%v, 0, 1) = %v, want %v", test.x, got, test.want)
		}
	}
}

// TestMinMaxF32 ports no upstream test. It pins the results of minF32 and
// maxF32 that rustc 1.97.1 gives for f32::min and f32::max on x86-64.
func TestMinMaxF32(t *testing.T) {
	nan := float32(math.NaN())
	negZero := float32(math.Copysign(0, -1))
	tests := []struct {
		name     string
		x, y     float32
		max, min float32
	}{
		{"nan first", nan, 1, 1, 1},
		{"nan second", 1, nan, 1, 1},
		{"plus zero first", 0, negZero, negZero, negZero},
		{"minus zero first", negZero, 0, 0, 0},
		{"ordered", 1, 2, 2, 1},
	}
	for _, test := range tests {
		if got := maxF32(test.x, test.y); math.Float32bits(got) != math.Float32bits(test.max) {
			t.Errorf("%s: maxF32 = %v, want %v", test.name, got, test.max)
		}
		if got := minF32(test.x, test.y); math.Float32bits(got) != math.Float32bits(test.min) {
			t.Errorf("%s: minF32 = %v, want %v", test.name, got, test.min)
		}
	}
}
