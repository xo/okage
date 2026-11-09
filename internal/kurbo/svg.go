// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

// This file ports the subset of kurbo kurbo/src/svg.rs that svgtypes and usvg
// need: SvgArc, and Arc::from_svg_arc.

// Note: the SVG arc logic is heavily adapted from https://github.com/nical/lyon

// SvgArc is SvgArc in kurbo kurbo/src/svg.rs.
//
// A single SVG arc segment.
type SvgArc struct {
	// The arc's start point.
	From Point
	// The arc's end point.
	To Point
	// The arc's radii, where the vector's x-component is the radius in the
	// positive x direction after applying `x_rotation`.
	Radii Vec2
	// How much the arc is rotated, in radians.
	XRotation float64
	// Does this arc sweep through more than π radians?
	LargeArc bool
	// Determines if the arc should begin moving at positive angles.
	Sweep bool
}

// IsStraightLine is SvgArc::is_straight_line in kurbo kurbo/src/svg.rs.
//
// Checks that arc is actually a straight line.
//
// In this case, it can be replaced with a `LineTo`.
func (a *SvgArc) IsStraightLine() bool {
	return Abs(a.Radii.X) <= 1e-5 || Abs(a.Radii.Y) <= 1e-5 || a.From == a.To
}

// FromSvgArc is Arc::from_svg_arc in kurbo kurbo/src/svg.rs.
//
// Creates an `Arc` from a `SvgArc`.
//
// Returns false if `arc` is actually a straight line.
func FromSvgArc(arc *SvgArc) (Arc, bool) {
	// Have to check this first, otherwise `sum_of_sq` will be 0.
	if arc.IsStraightLine() {
		return Arc{}, false
	}

	rx := Abs(arc.Radii.X)
	ry := Abs(arc.Radii.Y)

	xr := remF64(arc.XRotation, 2.0*pi)
	sinPhi, cosPhi := SinCos(xr)
	hdX := (arc.From.X - arc.To.X) * 0.5
	hdY := (arc.From.Y - arc.To.Y) * 0.5
	hsX := (arc.From.X + arc.To.X) * 0.5
	hsY := (arc.From.Y + arc.To.Y) * 0.5

	// F6.5.1
	p := NewVec2(
		float64(cosPhi*hdX)+float64(sinPhi*hdY),
		float64(-sinPhi*hdX)+float64(cosPhi*hdY),
	)

	// Sanitize the radii.
	// If rf > 1 it means the radii are too small for the arc to
	// possibly connect the end points. In this situation we scale
	// them up according to the formula provided by the SVG spec.

	// F6.6.2
	rf := p.X*p.X/(rx*rx) + p.Y*p.Y/(ry*ry)
	if rf > 1.0 {
		scale := Sqrt(rf)
		rx *= scale
		ry *= scale
	}

	rxry := rx * ry
	rxpy := rx * p.Y
	rypx := ry * p.X
	sumOfSq := float64(rxpy*rxpy) + float64(rypx*rypx)

	// F6.5.2
	signCoe := 1.0
	if arc.LargeArc == arc.Sweep {
		signCoe = -1.0
	}
	coe := signCoe * Sqrt(Abs((float64(rxry*rxry)-sumOfSq)/sumOfSq))
	transformedCx := coe * rxpy / ry
	transformedCy := -coe * rypx / rx

	// F6.5.3
	center := NewPoint(
		float64(cosPhi*transformedCx)-float64(sinPhi*transformedCy)+hsX,
		float64(sinPhi*transformedCx)+float64(cosPhi*transformedCy)+hsY,
	)

	startV := NewVec2((p.X-transformedCx)/rx, (p.Y-transformedCy)/ry)
	endV := NewVec2((-p.X-transformedCx)/rx, (-p.Y-transformedCy)/ry)

	startAngle := startV.Atan2()

	sweepAngle := remF64(endV.Atan2()-startAngle, 2.0*pi)

	if arc.Sweep && sweepAngle < 0.0 {
		sweepAngle += 2.0 * pi
	} else if !arc.Sweep && sweepAngle > 0.0 {
		sweepAngle -= 2.0 * pi
	}

	return Arc{
		Center:     center,
		Radii:      NewVec2(rx, ry),
		StartAngle: startAngle,
		SweepAngle: sweepAngle,
		XRotation:  arc.XRotation,
	}, true
}
