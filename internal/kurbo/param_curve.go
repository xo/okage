// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

// This file ports the subset of kurbo kurbo/src/param_curve.rs that svgtypes
// and usvg need: the traits ParamCurve, ParamCurveDeriv and ParamCurveArclen.
//
// A Rust trait becomes a Go interface. The trait methods return Self, so each
// interface takes the type that implements it as a type parameter. A default
// method of a trait becomes a generic function, and each type whose impl
// does not override it calls that function from its method.

// DefaultAccuracy is DEFAULT_ACCURACY in kurbo kurbo/src/param_curve.rs.
//
// A default value for methods that take an 'accuracy' argument.
//
// This value is intended to be suitable for general-purpose use, such as
// 2d graphics.
const DefaultAccuracy = 1e-6

// ParamCurve is the trait ParamCurve in kurbo kurbo/src/param_curve.rs. T is
// the type that implements it.
//
// A curve parameterized by a scalar.
//
// If the result is interpreted as a point, this represents a curve.
// But the result can be interpreted as a vector as well.
type ParamCurve[T any] interface {
	// Evaluate the curve at parameter `t`.
	//
	// Generally `t` is in the range [0..1].
	Eval(t float64) Point

	// Get a subsegment of the curve for the given parameter range, from
	// start to end. Rust takes a Range<f64>.
	Subsegment(start, end float64) T

	// Subdivide into (roughly) halves.
	Subdivide() (T, T)

	// The start point.
	Start() Point

	// The end point.
	End() Point
}

// subdivideParamCurve is the default method ParamCurve::subdivide in kurbo
// kurbo/src/param_curve.rs.
//
// Subdivide into (roughly) halves.
func subdivideParamCurve[T ParamCurve[T]](c T) (T, T) {
	return c.Subsegment(0.0, 0.5), c.Subsegment(0.5, 1.0)
}

// startParamCurve is the default method ParamCurve::start in kurbo
// kurbo/src/param_curve.rs.
//
// The start point.
func startParamCurve[T ParamCurve[T]](c T) Point {
	return c.Eval(0.0)
}

// endParamCurve is the default method ParamCurve::end in kurbo
// kurbo/src/param_curve.rs.
//
// The end point.
func endParamCurve[T ParamCurve[T]](c T) Point {
	return c.Eval(1.0)
}

// TODO: I might not want to have separate traits for all these.

// ParamCurveDeriv is the trait ParamCurveDeriv in kurbo
// kurbo/src/param_curve.rs. D is the associated type DerivResult.
//
// A differentiable parameterized curve.
type ParamCurveDeriv[D ParamCurve[D]] interface {
	// The derivative of the curve.
	//
	// Note that the type of the return value is somewhat inaccurate, as
	// the derivative of a curve (mapping of param to point) is a mapping
	// of param to vector. We choose to accept this rather than have a
	// more complex type scheme.
	Deriv() D
}

// GaussArclen is the default method ParamCurveDeriv::gauss_arclen in kurbo
// kurbo/src/param_curve.rs.
//
// Estimate arclength using Gaussian quadrature.
//
// The coefficients are assumed to cover the range (-1..1), which is
// traditional.
func GaussArclen[D ParamCurve[D]](c ParamCurveDeriv[D], coeffs [][2]float64) float64 {
	d := c.Deriv()
	// The sum of an iterator of f64 starts at -0.0 in Rust.
	sum := negZero
	for _, wx := range coeffs {
		wi, xi := wx[0], wx[1]
		sum += float64(wi * d.Eval(0.5*(xi+1.0)).ToVec2().Hypot())
	}
	return sum * 0.5
}

// ParamCurveArclen is the trait ParamCurveArclen in kurbo
// kurbo/src/param_curve.rs. T is the type that implements it.
//
// A parameterized curve that can have its arc length measured.
type ParamCurveArclen[T any] interface {
	ParamCurve[T]

	// The arc length of the curve.
	//
	// The result is accurate to the given accuracy (subject to
	// roundoff errors for ridiculously low values). Compute time
	// may vary with accuracy, if the curve needs to be subdivided.
	Arclen(accuracy float64) float64

	// Solve for the parameter that has the given arc length from the start.
	InvArclen(arclen, accuracy float64) float64
}

// invArclenParamCurve is the default method ParamCurveArclen::inv_arclen in
// kurbo kurbo/src/param_curve.rs.
//
// Solve for the parameter that has the given arc length from the start.
//
// This implementation uses the IPT method, as provided by
// [SolveItp]. This is as robust as bisection but
// typically converges faster. In addition, the method takes
// care to compute arc lengths of increasingly smaller segments
// of the curve, as that is likely faster than repeatedly
// computing the arc length of the segment starting at t=0.
func invArclenParamCurve[T ParamCurveArclen[T]](c T, arclen, accuracy float64) float64 {
	if arclen <= 0.0 {
		return 0.0
	}
	totalArclen := c.Arclen(accuracy)
	if arclen >= totalArclen {
		return 1.0
	}
	tLast := 0.0
	arclenLast := 0.0
	epsilon := accuracy / totalArclen
	n := 1.0 - minF64(Ceil(Log2(epsilon)), 0.0)
	innerAccuracy := accuracy / n
	f := func(t float64) float64 {
		var start, end, dir float64
		if t > tLast {
			start, end, dir = tLast, t, 1.0
		} else {
			start, end, dir = t, tLast, -1.0
		}
		arc := c.Subsegment(start, end).Arclen(innerAccuracy)
		arclenLast += float64(arc * dir)
		tLast = t
		return arclenLast - arclen
	}
	return SolveItp(f, 0.0, 1.0, epsilon, 1, 0.2, -arclen, totalArclen-arclen)
}
