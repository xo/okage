// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

import (
	"math"
	"testing"
)

// The tests of kurbo kurbo/src/common.rs. The tests test_solve_cubic,
// test_solve_quadratic and test_solve_quartic are not ported, because the
// subset has no polynomial solvers.

func TestSolveItp(t *testing.T) {
	f := func(x float64) float64 { return Powi(x, 3) - x - 2.0 }
	x := SolveItp(f, 1., 2., 1e-12, 0, 0.2, f(1.), f(2.))
	if !(math.Abs(f(x)) < 6e-12) {
		t.Errorf("f(%v) = %v", x, f(x))
	}
}

func TestInvArclen(t *testing.T) {
	c := NewCubicBez(
		NewPoint(0.0, 0.0),
		NewPoint(100.0/3.0, 0.0),
		NewPoint(200.0/3.0, 100.0/3.0),
		NewPoint(100.0, 100.0),
	)
	target := 100.0
	_ = SolveItp(
		func(tt float64) float64 { return c.Subsegment(0.0, tt).Arclen(1e-9) - target },
		0.,
		1.,
		1e-6,
		1,
		0.2,
		-target,
		c.Arclen(1e-9)-target,
	)
}

// TestPowi checks that Powi multiplies in the order of __powidf2.
func TestPowi(t *testing.T) {
	x := 1.1
	if got, want := Powi(x, 2), x*x; got != want {
		t.Errorf("Powi(x, 2) = %v, want %v", got, want)
	}
	if got, want := Powi(x, 3), float64(x*x)*x; got != want {
		t.Errorf("Powi(x, 3) = %v, want %v", got, want)
	}
	x2 := x * x
	x4 := x2 * x2
	if got, want := Powi(x, 6), x2*x4; got != want {
		t.Errorf("Powi(x, 6) = %v, want %v", got, want)
	}
	if got, want := Powi(x, -2), 1/(x*x); got != want {
		t.Errorf("Powi(x, -2) = %v, want %v", got, want)
	}
	if got := Powi(x, 0); got != 1 {
		t.Errorf("Powi(x, 0) = %v, want 1", got)
	}
}

// TestMinMaxF64 checks the rules of f64::min and f64::max for NaN.
func TestMinMaxF64(t *testing.T) {
	nan := math.NaN()
	if got := maxF64(nan, 1); got != 1 {
		t.Errorf("maxF64(NaN, 1) = %v", got)
	}
	if got := maxF64(1, nan); got != 1 {
		t.Errorf("maxF64(1, NaN) = %v", got)
	}
	if got := minF64(nan, 1); got != 1 {
		t.Errorf("minF64(NaN, 1) = %v", got)
	}
	if got := minF64(2, 1); got != 1 {
		t.Errorf("minF64(2, 1) = %v", got)
	}
}
