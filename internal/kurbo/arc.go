// Copyright 2019 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

// This file ports the subset of kurbo kurbo/src/arc.rs that svgtypes and
// usvg need: Arc, its conversion to cubic Bezier segments, and its impl of
// ParamCurve.

import "math"

// pi is core::f64::consts::PI. A typed constant holds the f64 value, so that
// a constant expression with it rounds as the Rust expression does.
const pi float64 = math.Pi

// fracPi2 is core::f64::consts::FRAC_PI_2.
const fracPi2 float64 = math.Pi / 2

// Arc is Arc in kurbo kurbo/src/arc.rs.
//
// A single elliptical arc segment.
type Arc struct {
	// The arc's centre point.
	Center Point
	// The arc's radii, where the vector's x-component is the radius in the
	// positive x direction after applying `x_rotation`.
	Radii Vec2
	// The start angle in radians.
	StartAngle float64
	// The angle between the start and end of the arc, in radians.
	SweepAngle float64
	// How much the arc is rotated, in radians.
	XRotation float64
}

// NewArc is Arc::new in kurbo kurbo/src/arc.rs.
//
// Create a new `Arc`.
func NewArc(center Point, radii Vec2, startAngle, sweepAngle, xRotation float64) Arc {
	return Arc{
		Center:     center,
		Radii:      radii,
		StartAngle: startAngle,
		SweepAngle: sweepAngle,
		XRotation:  xRotation,
	}
}

// Reversed is Arc::reversed in kurbo kurbo/src/arc.rs.
//
// Returns a copy of this `Arc` in the opposite direction.
//
// The new `Arc` will sweep towards the original `Arc`s
// start angle.
func (a Arc) Reversed() Arc {
	return Arc{
		Center:     a.Center,
		Radii:      a.Radii,
		StartAngle: a.StartAngle + a.SweepAngle,
		SweepAngle: -a.SweepAngle,
		XRotation:  a.XRotation,
	}
}

// angleAt is Arc::angle_at in kurbo kurbo/src/arc.rs.
func (a Arc) angleAt(t float64) float64 {
	return a.StartAngle + float64(a.SweepAngle*t)
}

// AppendIter is Arc::append_iter in kurbo kurbo/src/arc.rs.
//
// Create an iterator generating Bezier path elements.
//
// The generated elements can be appended to an existing bezier path.
func (a Arc) AppendIter(tolerance float64) *ArcAppendIter {
	sign := Signum(a.SweepAngle)
	scaledErr := maxF64(a.Radii.X, a.Radii.Y) / tolerance
	// Number of subdivisions per ellipse based on error tolerance.
	// Note: this may slightly underestimate the error for quadrants.
	nErr := maxF64(Powf(1.1163*scaledErr, 1.0/6.0), 3.999_999)
	nf := Ceil(nErr * Abs(a.SweepAngle) * (1.0 / (2.0 * pi)))
	angleStep := a.SweepAngle / nf
	n := f64ToUsize(nf)
	armLen := (4.0 / 3.0) * Abs(Tan(0.25*angleStep)) * sign
	angle0 := a.StartAngle
	p0 := sampleEllipse(a.Radii, a.XRotation, angle0)

	return &ArcAppendIter{
		idx: 0,

		center:    a.Center,
		radii:     a.Radii,
		xRotation: a.XRotation,
		n:         n,
		armLen:    armLen,
		angleStep: angleStep,

		p0:     p0,
		angle0: angle0,
	}
}

// ToCubicBeziers is Arc::to_cubic_beziers in kurbo kurbo/src/arc.rs.
//
// Converts an `Arc` into a series of cubic bezier segments.
//
// The closure `p` will be invoked with the control points for each segment.
func (a Arc) ToCubicBeziers(tolerance float64, p func(p1, p2, p3 Point)) {
	path := a.AppendIter(tolerance)
	for {
		p1, p2, p3, ok := path.Next()
		if !ok {
			break
		}
		p(p1, p2, p3)
	}
}

// ArcAppendIter is ArcAppendIter in kurbo kurbo/src/arc.rs. Upstream hides
// it from the documentation.
type ArcAppendIter struct {
	idx uint

	center    Point
	radii     Vec2
	xRotation float64
	n         uint
	armLen    float64
	angleStep float64

	p0     Vec2
	angle0 float64
}

// Next is Iterator::next for ArcAppendIter in kurbo kurbo/src/arc.rs.
//
// Upstream returns the path element PathEl::CurveTo(p1, p2, p3), the only
// element that it makes. okage does not port PathEl, so Next returns the
// three points, and false at the end.
func (it *ArcAppendIter) Next() (Point, Point, Point, bool) {
	if it.idx >= it.n {
		return Point{}, Point{}, Point{}, false
	}

	angle1 := it.angle0 + it.angleStep
	p0 := it.p0
	p1 := p0.Add(sampleEllipse(it.radii, it.xRotation, it.angle0+fracPi2).Mul(it.armLen))
	p3 := sampleEllipse(it.radii, it.xRotation, angle1)
	p2 := p3.Sub(sampleEllipse(it.radii, it.xRotation, angle1+fracPi2).Mul(it.armLen))

	it.angle0 = angle1
	it.p0 = p3
	it.idx++

	return it.center.Add(p1), it.center.Add(p2), it.center.Add(p3), true
}

// sampleEllipse is sample_ellipse in kurbo kurbo/src/arc.rs.
//
// Take the ellipse radii, how the radii are rotated, and the sweep angle, and return a point on
// the ellipse.
func sampleEllipse(radii Vec2, xRotation, angle float64) Vec2 {
	angleSin, angleCos := SinCos(angle)
	u := radii.X * angleCos
	v := radii.Y * angleSin
	return rotatePt(NewVec2(u, v), xRotation)
}

// rotatePt is rotate_pt in kurbo kurbo/src/arc.rs.
//
// Rotate `pt` about the origin by `angle` radians.
func rotatePt(pt Vec2, angle float64) Vec2 {
	angleSin, angleCos := SinCos(angle)
	return NewVec2(
		float64(pt.X*angleCos)-float64(pt.Y*angleSin),
		float64(pt.X*angleSin)+float64(pt.Y*angleCos),
	)
}

// Eval is ParamCurve::eval for Arc in kurbo kurbo/src/arc.rs.
func (a Arc) Eval(t float64) Point {
	return a.Center.Add(sampleEllipse(a.Radii, a.XRotation, a.angleAt(t)))
}

// Subsegment is ParamCurve::subsegment for Arc in kurbo kurbo/src/arc.rs.
func (a Arc) Subsegment(start, end float64) Arc {
	return Arc{
		Center:     a.Center,
		Radii:      a.Radii,
		StartAngle: a.angleAt(start),
		SweepAngle: a.SweepAngle * (end - start),
		XRotation:  a.XRotation,
	}
}

// Subdivide is the default method ParamCurve::subdivide for Arc, in kurbo
// kurbo/src/param_curve.rs.
func (a Arc) Subdivide() (Arc, Arc) {
	return subdivideParamCurve(a)
}

// Start is the default method ParamCurve::start for Arc, in kurbo
// kurbo/src/param_curve.rs.
func (a Arc) Start() Point {
	return startParamCurve(a)
}

// End is the default method ParamCurve::end for Arc, in kurbo
// kurbo/src/param_curve.rs.
func (a Arc) End() Point {
	return endParamCurve(a)
}
