// Copyright 2019 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

// RoundedRect is RoundedRect in kurbo kurbo/src/rounded_rect.rs, with one
// radius for the four corners, as RoundedRectRadii::from_single_radius
// gives.
type RoundedRect struct {
	Rect   Rect
	Radius float64
}

// NewRoundedRect is RoundedRect::new in kurbo kurbo/src/rounded_rect.rs.
func NewRoundedRect(x0, y0, x1, y1, radius float64) RoundedRect {
	rect := NewRect(x0, y0, x1, y1).Abs()
	shortest := minF64(rect.Width(), rect.Height())
	return RoundedRect{Rect: rect, Radius: minF64(Abs(radius), shortest/2.0)}
}

// PathElements is impl Shape for RoundedRect in kurbo
// kurbo/src/rounded_rect.rs. It goes clockwise in a y-down coordinate
// system, from the top of the left side.
func (r RoundedRect) PathElements(tolerance float64) []PathEl {
	rad := r.Radius
	rc := r.Rect
	arc := func(i int, center Point) Arc {
		return Arc{
			Center:     center,
			Radii:      Vec2{X: rad, Y: rad},
			StartAngle: fracPi2 * float64(i),
			SweepAngle: fracPi2,
		}
	}
	els := []PathEl{MoveTo(NewPoint(rc.X0, rc.Y0+rad))}
	els = arc(2, NewPoint(rc.X0+rad, rc.Y0+rad)).appendElements(els, tolerance)
	els = append(els, LineTo(NewPoint(rc.X1-rad, rc.Y0)))
	els = arc(3, NewPoint(rc.X1-rad, rc.Y0+rad)).appendElements(els, tolerance)
	els = append(els, LineTo(NewPoint(rc.X1, rc.Y1-rad)))
	els = arc(0, NewPoint(rc.X1-rad, rc.Y1-rad)).appendElements(els, tolerance)
	els = append(els, LineTo(NewPoint(rc.X0+rad, rc.Y1)))
	els = arc(1, NewPoint(rc.X0+rad, rc.Y1-rad)).appendElements(els, tolerance)
	return append(els, ClosePath())
}
