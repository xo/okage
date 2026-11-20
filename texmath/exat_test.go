// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"math"
	"testing"
)

func TestExAtMatchesTheReference(t *testing.T) {
	for _, c := range []struct{ size, want float64 }{
		{5, 0.7374}, {6, 0.7837}, {7, 0.8329}, {8, 0.8853}, {9, 0.9409},
		{10, 1.0000}, {10.95, 1.0591}, {12, 1.1147}, {14.4, 1.2398},
		{17.28, 1.3668}, {19, 1.4222}, {20, 1.4563},
		{20.74, 1.5000}, {24.88, 1.5000}, {30, 1.5000},
	} {
		if v := exAt(c.size); math.Abs(v-c.want) > 0.0001 {
			t.Errorf("exAt(%g) = %.4f, tectonic gives %.4f", c.size, v, c.want)
		}
	}
}

func TestExAtIsNotProportional(t *testing.T) {
	if r10, r20 := exAt(10)/10, exAt(20)/20; math.Abs(r10-r20) < 0.02 {
		t.Errorf("ex@/size: %.4f at 10pt and %.4f at 20pt - too close for this test "+
			"to tell a proportional stand-in from the real function", r10, r20)
	}
	if exAt(10) == exAt(20) {
		t.Error("ex@ is constant, so this test could not tell a constant from the real function")
	}
}

// okage: exAt ends, and returns a value, for any size.
func TestExAtTakesAnySize(t *testing.T) {
	for _, p := range []float64{0, -1, -1e15, -1e300, math.Inf(-1), math.Inf(1), math.NaN(), 1e300} {
		if v := exAt(p); math.IsNaN(v) || v < 0 || v > 1.5 {
			t.Errorf("exAt(%g) = %g", p, v)
		}
	}
}
