// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

import (
	"math"
	"testing"
)

// The tests of kurbo kurbo/src/cubicbez.rs. Only the tests of the subset are
// ported: cubicbez_deriv, cubicbez_arclen, cubicbez_inv_arclen,
// cubicbez_inv_arclen_accuracy, degenerate_to_quads and cubicbez_toquads.
// The others test the signed area, the nearest point, the quadratic splines,
// the extrema and the inflections, which the subset does not have.

func TestCubicBezDeriv(t *testing.T) {
	// y = x^2
	c := NewCubicBez(
		NewPoint(0.0, 0.0),
		NewPoint(1.0/3.0, 0.0),
		NewPoint(2.0/3.0, 1.0/3.0),
		NewPoint(1.0, 1.0),
	)
	deriv := c.Deriv()

	n := 10
	for i := 0; i <= n; i++ {
		tt := float64(i) * (1 / float64(n))
		delta := 1e-6
		p := c.Eval(tt)
		p1 := c.Eval(tt + delta)
		dApprox := p1.Sub(p).Mul(1 / delta)
		d := deriv.Eval(tt).ToVec2()
		if !(d.Sub(dApprox).Hypot() < delta*2.0) {
			t.Errorf("t = %v: deriv %v, approx %v", tt, d, dApprox)
		}
	}
}

func TestCubicBezArclen(t *testing.T) {
	// y = x^2
	c := NewCubicBez(
		NewPoint(0.0, 0.0),
		NewPoint(1.0/3.0, 0.0),
		NewPoint(2.0/3.0, 1.0/3.0),
		NewPoint(1.0, 1.0),
	)
	trueArclen := 0.5*math.Sqrt(5.0) + 0.25*math.Log(2.0+math.Sqrt(5.0))
	for i := range int32(12) {
		accuracy := Powi(0.1, i)
		err := c.Arclen(accuracy) - trueArclen
		if !(math.Abs(err) < accuracy) {
			t.Errorf("accuracy %v: error %v", accuracy, err)
		}
	}
}

func TestCubicBezInvArclen(t *testing.T) {
	// y = x^2 / 100
	c := NewCubicBez(
		NewPoint(0.0, 0.0),
		NewPoint(100.0/3.0, 0.0),
		NewPoint(200.0/3.0, 100.0/3.0),
		NewPoint(100.0, 100.0),
	)
	trueArclen := 100.0 * (0.5*math.Sqrt(5.0) + 0.25*math.Log(2.0+math.Sqrt(5.0)))
	for i := range int32(12) {
		accuracy := Powi(0.1, i)
		n := 10
		for j := 0; j <= n; j++ {
			arc := float64(j) * ((1 / float64(n)) * trueArclen)
			tt := c.InvArclen(arc, accuracy*0.5)
			actualArc := c.Subsegment(0.0, tt).Arclen(accuracy * 0.5)
			if !(math.Abs(arc-actualArc) < accuracy) {
				t.Errorf("at accuracy %e, wanted %v got %v", accuracy, actualArc, arc)
			}
		}
	}
	// corner case: user passes accuracy larger than total arc length
	accuracy := trueArclen * 1.1
	arc := trueArclen * 0.5
	tt := c.InvArclen(arc, accuracy)
	actualArc := c.Subsegment(0.0, tt).Arclen(accuracy)
	if !(math.Abs(arc-actualArc) < 2.0*accuracy) {
		t.Errorf("at accuracy %e, want %v got %v", accuracy, actualArc, arc)
	}
}

func TestCubicBezInvArclenAccuracy(t *testing.T) {
	c := NewCubicBez(NewPoint(0.2, 0.73), NewPoint(0.35, 1.08), NewPoint(0.85, 1.08), NewPoint(1.0, 0.73))
	trueT := c.InvArclen(0.5, 1e-12)
	for i := int32(1); i < 12; i++ {
		accuracy := Powi(0.1, i)
		approxT := c.InvArclen(0.5, accuracy)
		if !(math.Abs(approxT-trueT) <= accuracy) {
			t.Errorf("accuracy %v: t = %v, want %v", accuracy, approxT, trueT)
		}
	}
}

// TestDegenerateToQuads is degenerate_to_quads: ToQuads returns something
// for collinear points.
func TestDegenerateToQuads(t *testing.T) {
	c := NewCubicBez(NewPoint(0., 9.), NewPoint(6., 6.), NewPoint(12., 3.0), NewPoint(18., 0.0))
	it := c.ToQuads(1e-6)
	if n := it.Len(); n != 1 {
		t.Fatalf("ToQuads gives %d quadratics, want 1", n)
	}
	if _, _, _, ok := it.Next(); !ok {
		t.Fatal("ToQuads gives no quadratic")
	}
	if _, _, q, ok := it.Next(); ok {
		t.Errorf("ToQuads gives a second quadratic %v", q)
	}
}

// TestCubicBezToQuads is cubicbez_toquads.
func TestCubicBezToQuads(t *testing.T) {
	// y = x^3
	c := NewCubicBez(NewPoint(0.0, 0.0), NewPoint(1.0/3.0, 0.0), NewPoint(2.0/3.0, 0.0), NewPoint(1.0, 1.0))
	for i := range int32(10) {
		accuracy := Powi(0.1, i)
		it := c.ToQuads(accuracy)
		for {
			t0, t1, q, ok := it.Next()
			if !ok {
				break
			}
			epsilon := 1e-12
			if d := q.Start().Sub(c.Eval(t0)).Hypot(); !(d < epsilon) {
				t.Errorf("accuracy %v: the start of %v is %v from the cubic", accuracy, q, d)
			}
			if d := q.End().Sub(c.Eval(t1)).Hypot(); !(d < epsilon) {
				t.Errorf("accuracy %v: the end of %v is %v from the cubic", accuracy, q, d)
			}
			n := 4
			for j := 0; j <= n; j++ {
				tt := float64(j) * (1 / float64(n))
				p := q.Eval(tt)
				err := math.Abs(p.Y - Powi(p.X, 3))
				if !(err < accuracy) {
					t.Errorf("got %v wanted %v", err, accuracy)
				}
			}
		}
	}
}
