// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

import "math"

// Affine is Affine in kurbo kurbo/src/affine.rs.
//
// A 2D affine transform. The six coefficients are the matrix
//
//	[ a c e ]
//	[ b d f ]
//	[ 0 0 1 ]
//
// in the order a, b, c, d, e, f.
type Affine [6]float64

// IdentityAffine is Affine::IDENTITY in kurbo kurbo/src/affine.rs.
var IdentityAffine = AffineScale(1.0)

// NewAffine is Affine::new in kurbo kurbo/src/affine.rs.
func NewAffine(c [6]float64) Affine {
	return Affine(c)
}

// AffineScale is Affine::scale in kurbo kurbo/src/affine.rs.
func AffineScale(s float64) Affine {
	return Affine{s, 0.0, 0.0, s, 0.0, 0.0}
}

// AffineScaleNonUniform is Affine::scale_non_uniform in kurbo
// kurbo/src/affine.rs.
func AffineScaleNonUniform(sx, sy float64) Affine {
	return Affine{sx, 0.0, 0.0, sy, 0.0, 0.0}
}

// AffineRotate is Affine::rotate in kurbo kurbo/src/affine.rs. th is in
// radians.
func AffineRotate(th float64) Affine {
	s, c := SinCos(th)
	return Affine{c, s, -s, c, 0.0, 0.0}
}

// AffineTranslate is Affine::translate in kurbo kurbo/src/affine.rs.
func AffineTranslate(p Vec2) Affine {
	return Affine{1.0, 0.0, 0.0, 1.0, p.X, p.Y}
}

// AffineSkew is Affine::skew in kurbo kurbo/src/affine.rs.
func AffineSkew(skewX, skewY float64) Affine {
	return Affine{1.0, skewY, skewX, 1.0, 0.0, 0.0}
}

// Mul is impl Mul for Affine in kurbo kurbo/src/affine.rs: the transform
// that applies other first and then a.
func (a Affine) Mul(other Affine) Affine {
	return Affine{
		a[0]*other[0] + a[2]*other[1],
		a[1]*other[0] + a[3]*other[1],
		a[0]*other[2] + a[2]*other[3],
		a[1]*other[2] + a[3]*other[3],
		a[0]*other[4] + a[2]*other[5] + a[4],
		a[1]*other[4] + a[3]*other[5] + a[5],
	}
}

// MulPoint is impl Mul<Point> for Affine in kurbo kurbo/src/affine.rs.
func (a Affine) MulPoint(p Point) Point {
	return NewPoint(
		a[0]*p.X+a[2]*p.Y+a[4],
		a[1]*p.X+a[3]*p.Y+a[5],
	)
}

// Translation is Affine::translation in kurbo kurbo/src/affine.rs.
func (a Affine) Translation() Vec2 {
	return Vec2{X: a[4], Y: a[5]}
}

// Determinant is Affine::determinant in kurbo kurbo/src/affine.rs.
func (a Affine) Determinant() float64 {
	return a[0]*a[3] - a[1]*a[2]
}

// Inverse is Affine::inverse in kurbo kurbo/src/affine.rs. A singular
// transform gives non-finite coefficients.
func (a Affine) Inverse() Affine {
	invDet := 1 / a.Determinant()
	return Affine{
		invDet * a[3],
		-invDet * a[1],
		-invDet * a[2],
		invDet * a[0],
		invDet * (a[2]*a[5] - a[3]*a[4]),
		invDet * (a[1]*a[4] - a[0]*a[5]),
	}
}

// TransformRectBbox is Affine::transform_rect_bbox in kurbo
// kurbo/src/affine.rs.
func (a Affine) TransformRectBbox(rect Rect) Rect {
	p00 := a.MulPoint(NewPoint(rect.X0, rect.Y0))
	p01 := a.MulPoint(NewPoint(rect.X0, rect.Y1))
	p10 := a.MulPoint(NewPoint(rect.X1, rect.Y0))
	p11 := a.MulPoint(NewPoint(rect.X1, rect.Y1))
	return RectFromPoints(p00, p01).Union(RectFromPoints(p10, p11))
}

// IsFinite is Affine::is_finite in kurbo kurbo/src/affine.rs.
func (a Affine) IsFinite() bool {
	for _, c := range a {
		if math.IsInf(c, 0) || c != c {
			return false
		}
	}
	return true
}

// svd is Affine::svd in kurbo kurbo/src/affine.rs. It returns the singular
// values, the larger first, and the angle of the first singular vector.
func (a Affine) svd() (Vec2, float64) {
	aa, b, c, d := a[0], a[1], a[2], a[3]
	a2 := aa * aa
	b2 := b * b
	c2 := c * c
	d2 := d * d
	ab := aa * b
	cd := c * d
	angle := 0.5 * Atan2(2.0*(ab+cd), a2-b2+c2-d2)
	s1 := Sqrt(Powi(aa+d, 2) + Powi(b-c, 2))
	s2 := Sqrt(Powi(aa-d, 2) + Powi(b+c, 2))
	return Vec2{X: 0.5 * (s1 + s2), Y: 0.5 * Abs(s1-s2)}, angle
}
