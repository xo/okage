// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

import (
	"math"
	"slices"
	"testing"
)

// verifyRoots is verify in the tests of kurbo kurbo/src/common.rs.
func verifyRoots(t *testing.T, roots, expected []float64) {
	t.Helper()
	if len(roots) != len(expected) {
		t.Fatalf("got %v, want %v", roots, expected)
	}
	roots = slices.Clone(roots)
	slices.Sort(roots)
	for i := range expected {
		if math.Abs(roots[i]-expected[i]) >= 1e-12 {
			t.Errorf("got %v, want %v", roots, expected)
		}
	}
}

// TestSolveCubic is test_solve_cubic in kurbo kurbo/src/common.rs.
func TestSolveCubic(t *testing.T) {
	verifyRoots(t, SolveCubic(-5.0, 0.0, 0.0, 1.0), []float64{math.Cbrt(5.0)})
	verifyRoots(t, SolveCubic(-5.0, -1.0, 0.0, 1.0), []float64{1.90416085913492})
	verifyRoots(t, SolveCubic(0.0, -1.0, 0.0, 1.0), []float64{-1.0, 0.0, 1.0})
	verifyRoots(t, SolveCubic(-2.0, -3.0, 0.0, 1.0), []float64{-1.0, 2.0})
	verifyRoots(t, SolveCubic(2.0, -3.0, 0.0, 1.0), []float64{-2.0, 1.0})
	verifyRoots(t, SolveCubic(2.0-1e-12, 5.0, 4.0, 1.0), []float64{
		-1.9999999999989995,
		-1.0000010000848456,
		-0.9999989999161546,
	})
	verifyRoots(t, SolveCubic(2.0+1e-12, 5.0, 4.0, 1.0), []float64{-2.0})
}

// TestSolveQuadratic is test_solve_quadratic in kurbo kurbo/src/common.rs.
func TestSolveQuadratic(t *testing.T) {
	verifyRoots(t, SolveQuadratic(-5.0, 0.0, 1.0), []float64{-math.Sqrt(5.0), math.Sqrt(5.0)})
	verifyRoots(t, SolveQuadratic(5.0, 0.0, 1.0), nil)
	verifyRoots(t, SolveQuadratic(5.0, 1.0, 0.0), []float64{-5.0})
	verifyRoots(t, SolveQuadratic(1.0, 2.0, 1.0), []float64{-1.0})
}

// TestShapeOutlines checks the outlines of an ellipse and a rounded
// rectangle: where they start, how many elements they have, and that they
// end where they start.
func TestShapeOutlines(t *testing.T) {
	e := NewEllipse(NewPoint(10, 20), NewVec2(5, 3), 0).PathElements(0.1)
	if e[0].Kind != PathElMoveTo || e[0].P[0] != NewPoint(15, 20) {
		t.Errorf("the ellipse starts with %+v", e[0])
	}
	if last := e[len(e)-1]; last.Kind != PathElCurveTo || last.P[2].Distance(NewPoint(15, 20)) > 1e-9 {
		t.Errorf("the ellipse ends with %+v", last)
	}
	r := NewRoundedRect(0, 0, 10, 6, 2).PathElements(0.1)
	if r[0].P[0] != NewPoint(0, 2) || r[len(r)-1].Kind != PathElClosePath {
		t.Errorf("the rounded rectangle is %+v", r)
	}
	if got := len(Segments(r)); got != 8 {
		t.Errorf("the rounded rectangle has %d segments, want 8", got)
	}
}
