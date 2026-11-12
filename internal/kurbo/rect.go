// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

// Rect is Rect in kurbo kurbo/src/rect.rs.
//
// A rectangle, with the minimum x and y in X0 and Y0 and the maximum in X1 and
// Y1.
type Rect struct {
	X0, Y0, X1, Y1 float64
}

// NewRect is Rect::new in kurbo kurbo/src/rect.rs.
func NewRect(x0, y0, x1, y1 float64) Rect {
	return Rect{X0: x0, Y0: y0, X1: x1, Y1: y1}
}

// RectFromPoints is Rect::from_points in kurbo kurbo/src/rect.rs.
func RectFromPoints(p0, p1 Point) Rect {
	return NewRect(p0.X, p0.Y, p1.X, p1.Y).Abs()
}

// Width is Rect::width in kurbo kurbo/src/rect.rs.
func (r Rect) Width() float64 {
	return r.X1 - r.X0
}

// Height is Rect::height in kurbo kurbo/src/rect.rs.
func (r Rect) Height() float64 {
	return r.Y1 - r.Y0
}

// Abs is Rect::abs in kurbo kurbo/src/rect.rs.
func (r Rect) Abs() Rect {
	return NewRect(minF64(r.X0, r.X1), minF64(r.Y0, r.Y1), maxF64(r.X0, r.X1), maxF64(r.Y0, r.Y1))
}

// Union is Rect::union in kurbo kurbo/src/rect.rs.
func (r Rect) Union(other Rect) Rect {
	return NewRect(
		minF64(r.X0, other.X0),
		minF64(r.Y0, other.Y0),
		maxF64(r.X1, other.X1),
		maxF64(r.Y1, other.Y1),
	)
}

// UnionPt is Rect::union_pt in kurbo kurbo/src/rect.rs.
func (r Rect) UnionPt(pt Point) Rect {
	return NewRect(
		minF64(r.X0, pt.X),
		minF64(r.Y0, pt.Y),
		maxF64(r.X1, pt.X),
		maxF64(r.Y1, pt.Y),
	)
}

// PathElements is impl Shape for Rect in kurbo kurbo/src/rect.rs: the
// outline of the rectangle, clockwise in a y-down coordinate system.
func (r Rect) PathElements(_ float64) []PathEl {
	return []PathEl{
		MoveTo(NewPoint(r.X0, r.Y0)),
		LineTo(NewPoint(r.X1, r.Y0)),
		LineTo(NewPoint(r.X1, r.Y1)),
		LineTo(NewPoint(r.X0, r.Y1)),
		ClosePath(),
	}
}

// Size is Size in kurbo kurbo/src/size.rs.
type Size struct {
	Width, Height float64
}

// NewSize is Size::new in kurbo kurbo/src/size.rs.
func NewSize(width, height float64) Size {
	return Size{Width: width, Height: height}
}
