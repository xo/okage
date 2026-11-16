// Copyright 2019 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

import (
	"math"
	"testing"
)

// The tests of kurbo kurbo/src/arc.rs.

const (
	fracPi4 = math.Pi / 4
	fracPi6 = math.Pi / 6
)

func assertPointNear(t *testing.T, actual, expected Point) {
	t.Helper()
	epsilon := 1e-12
	if !(actual.Distance(expected) <= epsilon) {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

func assertSubsegmentMatches(t *testing.T, arc Arc, start, end float64) {
	t.Helper()
	subsegment := arc.Subsegment(start, end)
	for _, tt := range []float64{0.0, 0.25, 0.5, 1.0} {
		expectedT := start + (end-start)*tt
		assertPointNear(t, subsegment.Eval(tt), arc.Eval(expectedT))
	}
	assertPointNear(t, subsegment.Start(), arc.Eval(start))
	assertPointNear(t, subsegment.End(), arc.Eval(end))
}

func TestReversedArc(t *testing.T) {
	a := NewArc(NewPoint(0., 0.), NewVec2(1., 0.), 0., pi, 0.)
	f := a.Reversed()

	// Most fields should be unchanged:
	if a.Center != f.Center || a.Radii != f.Radii || a.XRotation != f.XRotation {
		t.Errorf("fields changed: %+v, %+v", a, f)
	}

	// Sweep angle should be in reverse
	if a.SweepAngle != -f.SweepAngle {
		t.Errorf("sweep angle %v, want %v", f.SweepAngle, -a.SweepAngle)
	}

	// Reversing it again should result in the original arc
	if a != f.Reversed() {
		t.Errorf("%+v != %+v", a, f.Reversed())
	}
}

func TestEvalEndpointsAndMidpoint(t *testing.T) {
	arc := NewArc(NewPoint(3.0, -2.0), NewVec2(4.0, 1.5), fracPi6, pi, 0.0)
	assertPointNear(t, arc.Eval(0.0), arc.Start())
	assertPointNear(t, arc.Eval(1.0), arc.End())
	assertPointNear(t,
		arc.Eval(0.5),
		arc.Center.Add(sampleEllipse(arc.Radii, arc.XRotation, arc.angleAt(0.5))),
	)
}

func TestEvalRotatedEllipse(t *testing.T) {
	arc := NewArc(NewPoint(5.0, 7.0), NewVec2(3.0, 2.0), fracPi6, pi/3.0, fracPi4)
	assertPointNear(t,
		arc.Start(),
		arc.Center.Add(sampleEllipse(arc.Radii, arc.XRotation, arc.StartAngle)),
	)
	assertPointNear(t,
		arc.End(),
		arc.Center.Add(sampleEllipse(arc.Radii, arc.XRotation, arc.StartAngle+arc.SweepAngle)),
	)
}

func TestEvalReversedArc(t *testing.T) {
	arc := NewArc(NewPoint(2.0, 3.0), NewVec2(4.0, 1.0), fracPi6, pi/2.0, fracPi4)
	reversed := arc.Reversed()
	assertPointNear(t, reversed.Start(), arc.End())
	assertPointNear(t, reversed.End(), arc.Start())
	assertPointNear(t, reversed.Eval(0.5), arc.Eval(0.5))
}

func TestSubsegmentMatchesOriginal(t *testing.T) {
	arc := NewArc(NewPoint(1.0, -4.0), NewVec2(3.0, 2.0), -fracPi4, pi, fracPi6)
	assertSubsegmentMatches(t, arc, 0.2, 0.7)
}

func TestSubsegmentNegativeSweepMatchesOriginal(t *testing.T) {
	arc := NewArc(NewPoint(1.0, 2.0), NewVec2(5.0, 3.0), pi, -0.75*pi, fracPi6)
	assertSubsegmentMatches(t, arc, 0.1, 0.8)
}

func TestSubsegmentTinySweepMatchesOriginal(t *testing.T) {
	arc := NewArc(NewPoint(0.0, 0.0), NewVec2(2.0, 1.0), 1.0, 1e-9, fracPi4)
	assertSubsegmentMatches(t, arc, 0.25, 0.75)
}

// TestAppendIterLimit makes sure that an arc with a huge radius ends with
// MaxArcSubdivisions curves that reach the end of the arc, where upstream does
// not end (D30), and that an arc just below the limit is not changed.
func TestAppendIterLimit(t *testing.T) {
	for _, r := range []float64{1e50, 1e300, math.MaxFloat64} {
		a := Arc{Center: Point{0, 0}, Radii: Vec2{r, r}, StartAngle: 0, SweepAngle: math.Pi}
		n := 0
		var last Point
		a.ToCubicBeziers(0.1, func(_, _, p3 Point) {
			n++
			last = p3
		})
		if n != MaxArcSubdivisions {
			t.Errorf("radius %g: %d curves, expected %d", r, n, MaxArcSubdivisions)
		}
		end := sampleEllipse(a.Radii, a.XRotation, a.StartAngle+a.SweepAngle)
		if want := a.Center.Add(end); math.Abs(last.X-want.X) > 1e-6*r || math.Abs(last.Y-want.Y) > 1e-6*r {
			t.Errorf("radius %g: the last point is %v, expected %v", r, last, want)
		}
	}
	// A full circle of radius 4e20 needs fewer curves than the limit, and
	// the limit changes nothing for it.
	a := Arc{Center: Point{0, 0}, Radii: Vec2{4e20, 4e20}, StartAngle: 0, SweepAngle: 2 * math.Pi}
	if it := a.AppendIter(0.1); it.n >= MaxArcSubdivisions {
		t.Errorf("radius 4e20: %d curves, expected fewer than %d", it.n, MaxArcSubdivisions)
	}
}
