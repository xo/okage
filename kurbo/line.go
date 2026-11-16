// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

// This file ports the subset of kurbo kurbo/src/line.rs that svgtypes and
// usvg need: Line and ConstPoint, with their impls of ParamCurve,
// ParamCurveDeriv and ParamCurveArclen.

// Line is Line in kurbo kurbo/src/line.rs.
//
// A single line.
type Line struct {
	// The line's start point.
	P0 Point
	// The line's end point.
	P1 Point
}

// NewLine is Line::new in kurbo kurbo/src/line.rs.
//
// Create a new line.
func NewLine(p0, p1 Point) Line {
	return Line{P0: p0, P1: p1}
}

// Eval is ParamCurve::eval for Line in kurbo kurbo/src/line.rs.
func (l Line) Eval(t float64) Point {
	return l.P0.Lerp(l.P1, t)
}

// Subsegment is ParamCurve::subsegment for Line in kurbo kurbo/src/line.rs.
func (l Line) Subsegment(start, end float64) Line {
	return Line{
		P0: l.Eval(start),
		P1: l.Eval(end),
	}
}

// Subdivide is the default method ParamCurve::subdivide for Line, in kurbo
// kurbo/src/param_curve.rs.
func (l Line) Subdivide() (Line, Line) {
	return subdivideParamCurve(l)
}

// Start is ParamCurve::start for Line in kurbo kurbo/src/line.rs.
func (l Line) Start() Point {
	return l.P0
}

// End is ParamCurve::end for Line in kurbo kurbo/src/line.rs.
func (l Line) End() Point {
	return l.P1
}

// Deriv is ParamCurveDeriv::deriv for Line in kurbo kurbo/src/line.rs.
func (l Line) Deriv() ConstPoint {
	return ConstPoint{l.P1.Sub(l.P0).ToPoint()}
}

// Arclen is ParamCurveArclen::arclen for Line in kurbo kurbo/src/line.rs.
func (l Line) Arclen(_ float64) float64 {
	return l.P1.Sub(l.P0).Hypot()
}

// InvArclen is ParamCurveArclen::inv_arclen for Line in kurbo
// kurbo/src/line.rs.
func (l Line) InvArclen(arclen, _ float64) float64 {
	return arclen / l.P1.Sub(l.P0).Hypot()
}

// ConstPoint is ConstPoint in kurbo kurbo/src/line.rs.
//
// A trivial "curve" that is just a constant.
type ConstPoint struct {
	p Point
}

// Eval is ParamCurve::eval for ConstPoint in kurbo kurbo/src/line.rs.
func (c ConstPoint) Eval(_ float64) Point {
	return c.p
}

// Subsegment is ParamCurve::subsegment for ConstPoint in kurbo
// kurbo/src/line.rs.
func (c ConstPoint) Subsegment(_, _ float64) ConstPoint {
	return c
}

// Subdivide is the default method ParamCurve::subdivide for ConstPoint, in
// kurbo kurbo/src/param_curve.rs.
func (c ConstPoint) Subdivide() (ConstPoint, ConstPoint) {
	return subdivideParamCurve(c)
}

// Start is the default method ParamCurve::start for ConstPoint, in kurbo
// kurbo/src/param_curve.rs.
func (c ConstPoint) Start() Point {
	return startParamCurve(c)
}

// End is the default method ParamCurve::end for ConstPoint, in kurbo
// kurbo/src/param_curve.rs.
func (c ConstPoint) End() Point {
	return endParamCurve(c)
}

// Deriv is ParamCurveDeriv::deriv for ConstPoint in kurbo kurbo/src/line.rs.
func (c ConstPoint) Deriv() ConstPoint {
	return ConstPoint{NewPoint(0.0, 0.0)}
}

// Arclen is ParamCurveArclen::arclen for ConstPoint in kurbo
// kurbo/src/line.rs.
func (c ConstPoint) Arclen(_ float64) float64 {
	return 0.0
}

// InvArclen is ParamCurveArclen::inv_arclen for ConstPoint in kurbo
// kurbo/src/line.rs.
func (c ConstPoint) InvArclen(_, _ float64) float64 {
	return 0.0
}
