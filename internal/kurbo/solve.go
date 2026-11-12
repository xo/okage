// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

import "math"

// SolveCubic is solve_cubic in kurbo kurbo/src/common.rs.
//
// Find real roots of cubic equation: the values of x for which
// c0 + c1 x + c2 x² + c3 x³ = 0.
//
// Port: Rust computes cbrt with the C library. Go takes math.Cbrt, which can
// differ in the last bit.
func SolveCubic(c0, c1, c2, c3 float64) []float64 {
	result := make([]float64, 0, 3)
	c3Recip := 1 / c3
	const oneThird = 1. / 3.
	scaledC2 := c2 * (oneThird * c3Recip)
	scaledC1 := c1 * (oneThird * c3Recip)
	scaledC0 := c0 * c3Recip
	if !(isFinite(scaledC0) && isFinite(scaledC1) && isFinite(scaledC2)) {
		// cubic coefficient is zero or nearly so.
		return SolveQuadratic(c0, c1, c2)
	}
	c0, c1, c2 = scaledC0, scaledC1, scaledC2
	d0 := math.FMA(-c2, c2, c1)
	d1 := math.FMA(-c1, c2, c0)
	d2 := c2*c0 - c1*c1
	d := 4.0*d0*d2 - d1*d1
	de := math.FMA(-2.0*c2, d0, d1)
	switch {
	case d < 0.0:
		sq := Sqrt(-0.25 * d)
		r := -0.5 * de
		t1 := math.Cbrt(r+sq) + math.Cbrt(r-sq)
		result = append(result, t1-c2)
	case d == 0.0:
		t1 := Copysign(Sqrt(-d0), de)
		result = append(result, t1-c2, -2.0*t1-c2)
	default:
		th := Atan2(Sqrt(d), -de) * oneThird
		thSin, thCos := SinCos(th)
		r0 := thCos
		ss3 := thSin * Sqrt(3.0)
		r1 := 0.5 * (-thCos + ss3)
		r2 := 0.5 * (-thCos - ss3)
		t := 2.0 * Sqrt(-d0)
		result = append(result, math.FMA(t, r0, -c2), math.FMA(t, r1, -c2), math.FMA(t, r2, -c2))
	}
	return result
}

// SolveQuadratic is solve_quadratic in kurbo kurbo/src/common.rs.
//
// Find real roots of quadratic equation: the values of x for which
// c0 + c1 x + c2 x² = 0.
func SolveQuadratic(c0, c1, c2 float64) []float64 {
	result := make([]float64, 0, 2)
	sc0 := c0 * (1 / c2)
	sc1 := c1 * (1 / c2)
	if !isFinite(sc0) || !isFinite(sc1) {
		// c2 is zero or very small, treat as linear eqn
		root := -c0 / c1
		if isFinite(root) {
			result = append(result, root)
		} else if c0 == 0.0 && c1 == 0.0 {
			// Degenerate case
			result = append(result, 0.0)
		}
		return result
	}
	arg := sc1*sc1 - 4.*sc0
	var root1 float64
	if !isFinite(arg) {
		root1 = -sc1
	} else {
		if arg < 0.0 {
			return result
		} else if arg == 0.0 {
			return append(result, -0.5*sc1)
		}
		root1 = -0.5 * (sc1 + Copysign(Sqrt(arg), sc1))
	}
	root2 := sc0 / root1
	if isFinite(root2) {
		// Sort just to be friendly and make results deterministic.
		if root2 > root1 {
			result = append(result, root1, root2)
		} else {
			result = append(result, root2, root1)
		}
	} else {
		result = append(result, root1)
	}
	return result
}

// isFinite is f64::is_finite.
func isFinite(x float64) bool {
	return !math.IsInf(x, 0) && x == x
}

// IsFinite is Vec2::is_finite in kurbo kurbo/src/vec2.rs.
func (v Vec2) IsFinite() bool {
	return isFinite(v.X) && isFinite(v.Y)
}

// Cross is Vec2::cross in kurbo kurbo/src/vec2.rs.
func (v Vec2) Cross(other Vec2) float64 {
	return v.X*other.Y - v.Y*other.X
}

// IsFinite is Point::is_finite in kurbo kurbo/src/point.rs.
func (p Point) IsFinite() bool {
	return isFinite(p.X) && isFinite(p.Y)
}

// IsFinite is CubicBez::is_finite in kurbo kurbo/src/cubicbez.rs.
func (c CubicBez) IsFinite() bool {
	return c.P0.IsFinite() && c.P1.IsFinite() && c.P2.IsFinite() && c.P3.IsFinite()
}

// IsFinite is QuadBez::is_finite in kurbo kurbo/src/quadbez.rs.
func (q QuadBez) IsFinite() bool {
	return q.P0.IsFinite() && q.P1.IsFinite() && q.P2.IsFinite()
}

// Extrema is impl ParamCurveExtrema for QuadBez in kurbo
// kurbo/src/quadbez.rs.
func (q QuadBez) Extrema() []float64 {
	result := make([]float64, 0, 2)
	d0 := q.P1.Sub(q.P0)
	d1 := q.P2.Sub(q.P1)
	dd := d1.Sub(d0)
	if dd.X != 0.0 {
		t := -d0.X / dd.X
		if t > 0.0 && t < 1.0 {
			result = append(result, t)
		}
	}
	if dd.Y != 0.0 {
		t := -d0.Y / dd.Y
		if t > 0.0 && t < 1.0 {
			result = append(result, t)
			if len(result) == 2 && result[0] > t {
				result[0], result[1] = result[1], result[0]
			}
		}
	}
	return result
}

// BoundingBox is ParamCurveExtrema::bounding_box in kurbo
// kurbo/src/param_curve.rs, for QuadBez.
func (q QuadBez) BoundingBox() Rect {
	bbox := RectFromPoints(q.P0, q.P2)
	for _, t := range q.Extrema() {
		bbox = bbox.UnionPt(q.Eval(t))
	}
	return bbox
}
