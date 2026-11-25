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

// TestFlatten checks Flatten on a circle of cubic curves and on a quadratic
// curve. kurbo has no test of flatten, so okage adds this one (D94). Each
// point of the result lies on the curve, and the middle of each line lies
// within the tolerance of the curve.
func TestFlatten(t *testing.T) {
	circle := NewEllipse(NewPoint(3, -2), NewVec2(50, 50), 0).PathElements(1e-3)
	last := 0
	for _, tolerance := range []float64{1, 0.25, 0.1, 0.01} {
		flat := Flatten(circle, tolerance, nil)
		if len(flat) <= last {
			t.Errorf("the tolerance %v gives %d elements, the larger one %d", tolerance, len(flat), last)
		}
		last = len(flat)
		var prev Point
		for i, el := range flat {
			if i == 0 && el.Kind != PathElMoveTo || i > 0 && el.Kind != PathElLineTo {
				t.Fatalf("the tolerance %v gives the element %d of the kind %v", tolerance, i, el.Kind)
			}
			p := el.P[0]
			if d := math.Abs(math.Hypot(p.X-3, p.Y+2) - 50); d > 0.01 {
				t.Errorf("the tolerance %v: the point %v lies %v from the circle", tolerance, p, d)
			}
			if i > 0 {
				m := prev.Midpoint(p)
				if d := 50 - math.Hypot(m.X-3, m.Y+2); d > tolerance+0.01 {
					t.Errorf("the tolerance %v: the middle %v lies %v inside the circle", tolerance, m, d)
				}
			}
			prev = p
		}
	}

	q := NewQuadBez(NewPoint(0, 0), NewPoint(50, 100), NewPoint(100, 0))
	flat := Flatten([]PathEl{MoveTo(q.P0), QuadTo(q.P1, q.P2)}, 0.1, nil)
	if len(flat) < 10 || flat[len(flat)-1] != LineTo(q.P2) {
		t.Fatalf("the quadratic gives %v", flat)
	}
	for _, el := range flat[1:] {
		// The curve is y = 2x - x^2 / 50.
		p := el.P[0]
		if d := math.Abs(p.Y - (2*p.X - p.X*p.X/50)); d > 1e-9 {
			t.Errorf("the point %v lies %v from the quadratic", p, d)
		}
	}

	lines := []PathEl{MoveTo(NewPoint(0, 0)), LineTo(NewPoint(1, 0)), LineTo(NewPoint(1, 1)), ClosePath()}
	if flat := Flatten(lines, 0.1, nil); !slices.Equal(flat, lines) {
		t.Errorf("the lines give %v", flat)
	}
}
