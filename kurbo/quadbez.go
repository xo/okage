// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

// This file ports the subset of kurbo kurbo/src/quadbez.rs that svgtypes and
// usvg need: QuadBez, QuadBez::raise, and the impls of ParamCurve,
// ParamCurveDeriv and ParamCurveArclen.

// QuadBez is QuadBez in kurbo kurbo/src/quadbez.rs.
//
// A single quadratic Bézier segment.
type QuadBez struct {
	P0 Point
	P1 Point
	P2 Point
}

// NewQuadBez is QuadBez::new in kurbo kurbo/src/quadbez.rs.
//
// Create a new quadratic Bézier segment.
func NewQuadBez(p0, p1, p2 Point) QuadBez {
	return QuadBez{P0: p0, P1: p1, P2: p2}
}

// Raise is QuadBez::raise in kurbo kurbo/src/quadbez.rs.
//
// Raise the order by 1.
//
// Returns a cubic Bézier segment that exactly represents this quadratic.
func (q QuadBez) Raise() CubicBez {
	return NewCubicBez(
		q.P0,
		q.P0.Add(q.P1.Sub(q.P0).Mul(2.0/3.0)),
		q.P2.Add(q.P1.Sub(q.P2).Mul(2.0/3.0)),
		q.P2,
	)
}

// Eval is ParamCurve::eval for QuadBez in kurbo kurbo/src/quadbez.rs.
func (q QuadBez) Eval(t float64) Point {
	mt := 1.0 - t
	return q.P0.ToVec2().Mul(mt * mt).
		Add(q.P1.ToVec2().Mul(mt * 2.0).Add(q.P2.ToVec2().Mul(t)).Mul(t)).
		ToPoint()
}

// Subsegment is ParamCurve::subsegment for QuadBez in kurbo
// kurbo/src/quadbez.rs.
func (q QuadBez) Subsegment(start, end float64) QuadBez {
	t0, t1 := start, end
	p0 := q.Eval(t0)
	p2 := q.Eval(t1)
	p1 := p0.Add(q.P1.Sub(q.P0).Lerp(q.P2.Sub(q.P1), t0).Mul(t1 - t0))
	return QuadBez{P0: p0, P1: p1, P2: p2}
}

// Subdivide is ParamCurve::subdivide for QuadBez in kurbo
// kurbo/src/quadbez.rs.
//
// Subdivide into halves, using de Casteljau.
func (q QuadBez) Subdivide() (QuadBez, QuadBez) {
	pm := q.Eval(0.5)
	return NewQuadBez(q.P0, q.P0.Midpoint(q.P1), pm),
		NewQuadBez(pm, q.P1.Midpoint(q.P2), q.P2)
}

// Start is ParamCurve::start for QuadBez in kurbo kurbo/src/quadbez.rs.
func (q QuadBez) Start() Point {
	return q.P0
}

// End is ParamCurve::end for QuadBez in kurbo kurbo/src/quadbez.rs.
func (q QuadBez) End() Point {
	return q.P2
}

// Deriv is ParamCurveDeriv::deriv for QuadBez in kurbo kurbo/src/quadbez.rs.
func (q QuadBez) Deriv() Line {
	return NewLine(
		q.P1.ToVec2().Sub(q.P0.ToVec2()).Mul(2.0).ToPoint(),
		q.P2.ToVec2().Sub(q.P1.ToVec2()).Mul(2.0).ToPoint(),
	)
}

// Arclen is ParamCurveArclen::arclen for QuadBez in kurbo
// kurbo/src/quadbez.rs.
//
// Arclength of a quadratic Bézier segment.
//
// This computation is based on an analytical formula. Since that formula suffers
// from numerical instability when the curve is very close to a straight line, we
// detect that case and fall back to Legendre-Gauss quadrature.
//
// Accuracy should be better than 1e-13 over the entire range.
//
// Adapted from <http://www.malczak.linuxpl.com/blog/quadratic-bezier-curve-length/>
// with permission.
func (q QuadBez) Arclen(_ float64) float64 {
	d2 := q.P0.ToVec2().Sub(q.P1.ToVec2().Mul(2.0)).Add(q.P2.ToVec2())
	a := d2.Hypot2()
	d1 := q.P1.Sub(q.P0)
	c := d1.Hypot2()
	if a < float64(5e-4*c) {
		// This case happens for nearly straight Béziers.
		//
		// Calculate arclength using Legendre-Gauss quadrature using formula from Behdad
		// in https://github.com/Pomax/BezierInfo-2/issues/77
		v0 := q.P0.ToVec2().Mul(-0.492943519233745).
			Add(q.P1.ToVec2().Mul(0.430331482911935)).
			Add(q.P2.ToVec2().Mul(0.0626120363218102)).
			Hypot()
		v1 := q.P2.Sub(q.P0).Mul(0.4444444444444444).Hypot()
		v2 := q.P0.ToVec2().Mul(-0.0626120363218102).
			Sub(q.P1.ToVec2().Mul(0.430331482911935)).
			Add(q.P2.ToVec2().Mul(0.492943519233745)).
			Hypot()
		return v0 + v1 + v2
	}
	b := 2.0 * d2.Dot(d1)

	sabc := Sqrt(a + b + c)
	a2 := Powf(a, -0.5)
	a32 := Powi(a2, 3)
	c2 := 2.0 * Sqrt(c)
	baC2 := float64(b*a2) + c2

	v0 := float64(0.25*a2*a2*b*(float64(2.0*sabc)-c2)) + sabc
	// TODO: justify and fine-tune this exact constant.
	// The factor of a2 here is a little arbitrary: we really want
	// to test whether ba_c2 is small, but it's also important for
	// this comparison to be scale-invariant. We chose a2 (instead of,
	// for example, c2 on the rhs) because it's unchanged under
	// reversing the parametrization.
	if baC2*a2 < 1e-13 {
		// This case happens for Béziers with a sharp kink.
		return v0
	}
	return v0 + float64(0.25*
		a32*
		(float64(4.0*c*a)-float64(b*b))*
		Ln((float64((float64(2.0*a)+b)*a2)+float64(2.0*sabc))/baC2))
}

// InvArclen is the default method ParamCurveArclen::inv_arclen for QuadBez,
// in kurbo kurbo/src/param_curve.rs.
func (q QuadBez) InvArclen(arclen, accuracy float64) float64 {
	return invArclenParamCurve(q, arclen, accuracy)
}
