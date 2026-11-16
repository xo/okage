// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

import (
	"math"
	"testing"
)

// The tests of kurbo kurbo/src/quadbez.rs. The tests quadbez_signed_area,
// quadbez_nearest, quadbez_nearest_low_order,
// quadbez_nearest_rounding_panic, quadbez_extrema and perimeter_not_nan are
// not ported, because the subset has no ParamCurveArea, ParamCurveNearest,
// ParamCurveExtrema and Shape.

func assertNear(t *testing.T, p0, p1 Point, epsilon float64) {
	t.Helper()
	if !(p1.Sub(p0).Hypot() < epsilon) {
		t.Errorf("%v != %v", p0, p1)
	}
}

func TestQuadBezDeriv(t *testing.T) {
	q := NewQuadBez(NewPoint(0.0, 0.0), NewPoint(0.0, 0.5), NewPoint(1.0, 1.0))
	deriv := q.Deriv()

	n := 10
	for i := 0; i <= n; i++ {
		tt := float64(i) * (1 / float64(n))
		delta := 1e-6
		p := q.Eval(tt)
		p1 := q.Eval(tt + delta)
		dApprox := p1.Sub(p).Mul(1 / delta)
		d := deriv.Eval(tt).ToVec2()
		if !(d.Sub(dApprox).Hypot() < delta*2.0) {
			t.Errorf("t = %v: deriv %v, approx %v", tt, d, dApprox)
		}
	}
}

func TestQuadBezArclen(t *testing.T) {
	q := NewQuadBez(NewPoint(0.0, 0.0), NewPoint(0.0, 0.5), NewPoint(1.0, 1.0))
	trueArclen := 0.5*math.Sqrt(5.0) + 0.25*math.Log(2.0+math.Sqrt(5.0))
	for i := range int32(12) {
		accuracy := Powi(0.1, i)
		est := q.Arclen(accuracy)
		err := est - trueArclen
		if !(math.Abs(err) < accuracy) {
			t.Errorf("%v != %v", est, trueArclen)
		}
	}
}

func TestQuadBezArclenPathological(t *testing.T) {
	q := NewQuadBez(NewPoint(-1.0, 0.0), NewPoint(1.03, 0.0), NewPoint(1.0, 0.0))
	trueArclen := 2.0008737864167325 // A rough empirical calculation
	accuracy := 1e-11
	est := q.Arclen(accuracy)
	if !(math.Abs(est-trueArclen) < accuracy) {
		t.Errorf("%v != %v", est, trueArclen)
	}
}

func TestQuadBezSubsegment(t *testing.T) {
	q := NewQuadBez(NewPoint(3.1, 4.1), NewPoint(5.9, 2.6), NewPoint(5.3, 5.8))
	t0 := 0.1
	t1 := 0.8
	qs := q.Subsegment(t0, t1)
	epsilon := 1e-12
	n := 10
	for i := 0; i <= n; i++ {
		tt := float64(i) * (1 / float64(n))
		ts := t0 + tt*(t1-t0)
		assertNear(t, q.Eval(ts), qs.Eval(tt), epsilon)
	}
}

func TestQuadBezRaise(t *testing.T) {
	q := NewQuadBez(NewPoint(3.1, 4.1), NewPoint(5.9, 2.6), NewPoint(5.3, 5.8))
	c := q.Raise()
	qd := q.Deriv()
	cd := c.Deriv()
	epsilon := 1e-12
	n := 10
	for i := 0; i <= n; i++ {
		tt := float64(i) * (1 / float64(n))
		assertNear(t, q.Eval(tt), c.Eval(tt), epsilon)
		assertNear(t, qd.Eval(tt), cd.Eval(tt), epsilon)
	}
}
