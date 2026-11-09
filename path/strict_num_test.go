// Copyright (c) 2022 Yevhenii Reizner
//
// Use of this source code is governed by the MIT license that can be found in
// the NOTICE file.

package path

import (
	"math"
	"testing"
)

// The tests positive_f64, norm_f64 and clamped_norm_f64 of strict-num are not
// ported, because the f64 types are not ported.

var (
	nanF32    = float32(math.NaN())
	infF32    = float32(math.Inf(1))
	negInfF32 = float32(math.Inf(-1))
)

// TestFiniteF32 is the test finite_f32 in strict-num src/lib.rs.
func TestFiniteF32(t *testing.T) {
	if n, ok := NewFiniteF32(0.0); !ok || n.Get() != 0.0 {
		t.Errorf("NewFiniteF32(0) = %v, %v, want 0, true", n, ok)
	}
	for _, x := range []float32{nanF32, infF32, negInfF32} {
		if _, ok := NewFiniteF32(x); ok {
			t.Errorf("NewFiniteF32(%v) is ok, want false", x)
		}
	}
}

// TestPositiveF32 is the test positive_f32 in strict-num src/lib.rs.
func TestPositiveF32(t *testing.T) {
	for _, x := range []float32{-1.0, 0.0, -epsilonF32, nanF32, infF32, negInfF32} {
		if _, ok := NewNonZeroPositiveF32(x); ok {
			t.Errorf("NewNonZeroPositiveF32(%v) is ok, want false", x)
		}
	}
	for _, x := range []float32{1.0, epsilonF32} {
		if n, ok := NewNonZeroPositiveF32(x); !ok || n.Get() != x {
			t.Errorf("NewNonZeroPositiveF32(%v) = %v, %v, want %v, true", x, n, ok, x)
		}
	}
}

// TestNormF32 is the test norm_f32 in strict-num src/lib.rs.
func TestNormF32(t *testing.T) {
	for _, x := range []float32{-0.5, -epsilonF32, 1.5, nanF32, infF32, negInfF32} {
		if _, ok := NewNormalizedF32(x); ok {
			t.Errorf("NewNormalizedF32(%v) is ok, want false", x)
		}
	}
	for _, x := range []float32{0.0, 0.5, 1.0} {
		if n, ok := NewNormalizedF32(x); !ok || n.Get() != x {
			t.Errorf("NewNormalizedF32(%v) = %v, %v, want %v, true", x, n, ok, x)
		}
	}
}

// TestClampedNormF32 is the test clamped_norm_f32 in strict-num src/lib.rs.
func TestClampedNormF32(t *testing.T) {
	tests := []struct {
		x    float32
		want float32
	}{
		{-0.5, 0.0},
		{0.5, 0.5},
		{1.5, 1.0},
		{nanF32, 0.0},
		{infF32, 0.0},
		{negInfF32, 0.0},
	}
	for _, test := range tests {
		if got := NormalizedF32Clamped(test.x).Get(); got != test.want {
			t.Errorf("NormalizedF32Clamped(%v) = %v, want %v", test.x, got, test.want)
		}
	}
}
