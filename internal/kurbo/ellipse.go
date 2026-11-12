// Copyright 2019 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

// Ellipse is Ellipse in kurbo kurbo/src/ellipse.rs: a unit circle under an
// affine transform.
type Ellipse struct {
	inner Affine
}

// NewEllipse is Ellipse::new in kurbo kurbo/src/ellipse.rs.
func NewEllipse(center Point, radii Vec2, xRotation float64) Ellipse {
	return Ellipse{
		inner: AffineTranslate(center.ToVec2()).
			Mul(AffineRotate(xRotation)).
			Mul(AffineScaleNonUniform(Abs(radii.X), Abs(radii.Y))),
	}
}

// Center is Ellipse::center in kurbo kurbo/src/ellipse.rs.
func (e Ellipse) Center() Point {
	return e.inner.Translation().ToPoint()
}

// PathElements is impl Shape for Ellipse in kurbo kurbo/src/ellipse.rs. The
// outline has no ClosePath, as in kurbo.
func (e Ellipse) PathElements(tolerance float64) []PathEl {
	radii, xRotation := e.inner.svd()
	return Arc{
		Center:     e.Center(),
		Radii:      radii,
		StartAngle: 0.0,
		SweepAngle: 2.0 * pi,
		XRotation:  xRotation,
	}.PathElements(tolerance)
}

// PathElements is impl Shape for Arc in kurbo kurbo/src/arc.rs: a MoveTo to
// the start, and the curves of AppendIter.
func (a Arc) PathElements(tolerance float64) []PathEl {
	els := []PathEl{MoveTo(a.Start())}
	return a.appendElements(els, tolerance)
}

// appendElements appends the curves of AppendIter to els.
func (a Arc) appendElements(els []PathEl, tolerance float64) []PathEl {
	it := a.AppendIter(tolerance)
	for {
		p1, p2, p3, ok := it.Next()
		if !ok {
			return els
		}
		els = append(els, CurveTo(p1, p2, p3))
	}
}
