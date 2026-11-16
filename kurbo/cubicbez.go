// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

// This file ports the subset of kurbo kurbo/src/cubicbez.rs that svgtypes
// and usvg need: CubicBez, and the impls of ParamCurve, ParamCurveDeriv and
// ParamCurveArclen with the arc length helpers.

// CubicBez is CubicBez in kurbo kurbo/src/cubicbez.rs.
//
// A single cubic Bézier segment.
type CubicBez struct {
	P0 Point
	P1 Point
	P2 Point
	P3 Point
}

// NewCubicBez is CubicBez::new in kurbo kurbo/src/cubicbez.rs.
//
// Create a new cubic Bézier segment.
func NewCubicBez(p0, p1, p2, p3 Point) CubicBez {
	return CubicBez{P0: p0, P1: p1, P2: p2, P3: p3}
}

// Eval is ParamCurve::eval for CubicBez in kurbo kurbo/src/cubicbez.rs.
func (c CubicBez) Eval(t float64) Point {
	mt := 1.0 - t
	v := c.P0.ToVec2().Mul(mt * mt * mt).
		Add(c.P1.ToVec2().Mul(mt * mt * 3.0).
			Add(c.P2.ToVec2().Mul(mt * 3.0).Add(c.P3.ToVec2().Mul(t)).Mul(t)).
			Mul(t))
	return v.ToPoint()
}

// Subsegment is ParamCurve::subsegment for CubicBez in kurbo
// kurbo/src/cubicbez.rs.
func (c CubicBez) Subsegment(start, end float64) CubicBez {
	t0, t1 := start, end
	p0 := c.Eval(t0)
	p3 := c.Eval(t1)
	d := c.Deriv()
	scale := (t1 - t0) * (1.0 / 3.0)
	p1 := p0.Add(d.Eval(t0).ToVec2().Mul(scale))
	p2 := p3.SubVec(d.Eval(t1).ToVec2().Mul(scale))
	return CubicBez{P0: p0, P1: p1, P2: p2, P3: p3}
}

// Subdivide is ParamCurve::subdivide for CubicBez in kurbo
// kurbo/src/cubicbez.rs.
//
// Subdivide into halves, using de Casteljau.
func (c CubicBez) Subdivide() (CubicBez, CubicBez) {
	pm := c.Eval(0.5)
	return NewCubicBez(
			c.P0,
			c.P0.Midpoint(c.P1),
			c.P0.ToVec2().Add(c.P1.ToVec2().Mul(2.0)).Add(c.P2.ToVec2()).Mul(0.25).
				ToPoint(),
			pm,
		),
		NewCubicBez(
			pm,
			c.P1.ToVec2().Add(c.P2.ToVec2().Mul(2.0)).Add(c.P3.ToVec2()).Mul(0.25).
				ToPoint(),
			c.P2.Midpoint(c.P3),
			c.P3,
		)
}

// Start is ParamCurve::start for CubicBez in kurbo kurbo/src/cubicbez.rs.
func (c CubicBez) Start() Point {
	return c.P0
}

// End is ParamCurve::end for CubicBez in kurbo kurbo/src/cubicbez.rs.
func (c CubicBez) End() Point {
	return c.P3
}

// Deriv is ParamCurveDeriv::deriv for CubicBez in kurbo
// kurbo/src/cubicbez.rs.
func (c CubicBez) Deriv() QuadBez {
	return NewQuadBez(
		c.P1.Sub(c.P0).Mul(3.0).ToPoint(),
		c.P2.Sub(c.P1).Mul(3.0).ToPoint(),
		c.P3.Sub(c.P2).Mul(3.0).ToPoint(),
	)
}

// arclenQuadratureCore is arclen_quadrature_core in kurbo
// kurbo/src/cubicbez.rs.
func arclenQuadratureCore(coeffs [][2]float64, dm, dm1, dm2 Vec2) float64 {
	// The sum of an iterator of f64 starts at -0.0 in Rust.
	sum := negZero
	for _, wx := range coeffs {
		wi, xi := wx[0], wx[1]
		d := dm.Add(dm2.Mul(xi * xi))
		dpx := d.Add(dm1.Mul(xi)).Hypot()
		dmx := d.Sub(dm1.Mul(xi)).Hypot()
		// 2.25f64.sqrt() is exactly 1.5.
		sum += float64(float64(Sqrt(2.25)*wi) * (dpx + dmx))
	}
	return sum
}

// arclenRec is arclen_rec in kurbo kurbo/src/cubicbez.rs.
func arclenRec(c *CubicBez, accuracy float64, depth uint) float64 {
	d03 := c.P3.Sub(c.P0)
	d01 := c.P1.Sub(c.P0)
	d12 := c.P2.Sub(c.P1)
	d23 := c.P3.Sub(c.P2)
	lpLc := d01.Hypot() + d12.Hypot() + d23.Hypot() - d03.Hypot()
	dd1 := d12.Sub(d01)
	dd2 := d23.Sub(d12)
	// It might be faster to do direct multiplies, the data dependencies would be shorter.
	// The following values don't have the factor of 3 for first deriv
	dm := d01.Add(d23).Mul(0.25).Add(d12.Mul(0.5)) // first derivative at midpoint
	dm1 := dd2.Add(dd1).Mul(0.5)                   // second derivative at midpoint
	dm2 := dd2.Sub(dd1).Mul(0.25)                  // 0.5 * (third derivative at midpoint)

	// The sum of an iterator of f64 starts at -0.0 in Rust.
	est := negZero
	for _, wx := range GaussLegendreCoeffs8 {
		wi, xi := wx[0], wx[1]
		dNorm2 := dm.Add(dm1.Mul(xi)).Add(dm2.Mul(xi * xi)).Hypot2()
		ddNorm2 := dm1.Add(dm2.Mul(2.0 * xi)).Hypot2()
		est += float64(wi * (ddNorm2 / dNorm2))
	}
	estGauss8Error := minF64(float64(Powi(est, 3)*2.5e-6), 3e-2) * lpLc
	if estGauss8Error < accuracy {
		return arclenQuadratureCore(GaussLegendreCoeffs8Half, dm, dm1, dm2)
	}
	estGauss16Error := minF64(float64(Powi(est, 6)*1.5e-11), 9e-3) * lpLc
	if estGauss16Error < accuracy {
		return arclenQuadratureCore(GaussLegendreCoeffs16Half, dm, dm1, dm2)
	}
	estGauss24Error := minF64(float64(Powi(est, 9)*3.5e-16), 3.5e-3) * lpLc
	if estGauss24Error < accuracy || depth >= 20 {
		return arclenQuadratureCore(GaussLegendreCoeffs24Half, dm, dm1, dm2)
	}
	c0, c1 := c.Subdivide()
	return arclenRec(&c0, accuracy*0.5, depth+1) + arclenRec(&c1, accuracy*0.5, depth+1)
}

// Arclen is ParamCurveArclen::arclen for CubicBez in kurbo
// kurbo/src/cubicbez.rs.
//
// Arclength of a cubic Bézier segment.
//
// This is an adaptive subdivision approach using Legendre-Gauss quadrature
// in the base case, and an error estimate to decide when to subdivide.
func (c CubicBez) Arclen(accuracy float64) float64 {
	return arclenRec(&c, accuracy, 0)
}

// InvArclen is the default method ParamCurveArclen::inv_arclen for CubicBez,
// in kurbo kurbo/src/param_curve.rs.
func (c CubicBez) InvArclen(arclen, accuracy float64) float64 {
	return invArclenParamCurve(c, arclen, accuracy)
}
