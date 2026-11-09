// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

// lengthU32 is LengthU32 in tiny-skia-path path/src/lib.rs. It is an integer
// length that is guaranteed to be > 0.
//
// Port: Rust uses core::num::NonZeroU32. Go has no such type, so each
// constructor checks that the value is not zero.
type lengthU32 = uint32

// Point is Point in tiny-skia-path path/src/lib.rs. It is a point.
//
// The point is not guaranteed to be finite.
type Point struct {
	X float32
	Y float32
}

// PointFromXY is Point::from_xy in tiny-skia-path path/src/lib.rs. It creates
// a new Point.
func PointFromXY(x, y float32) Point {
	return Point{X: x, Y: y}
}

// PointFromF32x2 is Point::from_f32x2 in tiny-skia-path path/src/lib.rs. It
// creates a new Point from an F32x2.
func PointFromF32x2(r F32x2) Point {
	return PointFromXY(r.X(), r.Y())
}

// ToF32x2 is Point::to_f32x2 in tiny-skia-path path/src/lib.rs. It converts a
// Point into an F32x2.
func (p Point) ToF32x2() F32x2 {
	return NewF32x2(p.X, p.Y)
}

// PointZero is Point::zero in tiny-skia-path path/src/lib.rs. It creates a
// point at the position 0x0.
func PointZero() Point {
	return Point{X: 0, Y: 0}
}

// IsZero is Point::is_zero in tiny-skia-path path/src/lib.rs. It returns true
// if x and y are both zero.
func (p Point) IsZero() bool {
	return p.X == 0 && p.Y == 0
}

// IsFinite is Point::is_finite in tiny-skia-path path/src/lib.rs. It returns
// true if both x and y are measurable values.
//
// Both values are other than infinities and NaN.
//
// Port: upstream tests the product x*y, so a product that overflows to
// infinity also gives false.
func (p Point) IsFinite() bool {
	return isFinite(float32(p.X * p.Y))
}

// almostEqual is Point::almost_equal in tiny-skia-path path/src/lib.rs. It
// checks that two points are almost equal.
func (p Point) almostEqual(other Point) bool {
	return !p.Sub(other).canNormalize()
}

// equalsWithinTolerance is Point::equals_within_tolerance in tiny-skia-path
// path/src/lib.rs. It checks that two points are almost equal with the given
// tolerance.
func (p Point) equalsWithinTolerance(other Point, tolerance float32) bool {
	return IsNearlyZeroWithinTolerance(p.X-other.X, tolerance) &&
		IsNearlyZeroWithinTolerance(p.Y-other.Y, tolerance)
}

// Normalize is Point::normalize in tiny-skia-path path/src/lib.rs. It scales
// (x, y) so that Length returns one, and keeps the ratio of x to y, if it can.
//
// If the length before is nearly zero, it sets the vector to (0, 0) and
// returns false. Otherwise it returns true.
func (p *Point) Normalize() bool {
	return p.SetLengthFrom(p.X, p.Y, 1)
}

// SetNormalize is Point::set_normalize in tiny-skia-path path/src/lib.rs. It
// sets the vector to (x, y) scaled so that Length returns one, and so that the
// vector is proportional to (x, y).
//
// If the length of (x, y) is nearly zero, it sets the vector to (0, 0) and
// returns false. Otherwise it returns true.
func (p *Point) SetNormalize(x, y float32) bool {
	return p.SetLengthFrom(x, y, 1)
}

// canNormalize is Point::can_normalize in tiny-skia-path path/src/lib.rs.
func (p Point) canNormalize() bool {
	return isFinite(p.X) && isFinite(p.Y) && (p.X != 0 || p.Y != 0)
}

// Length is Point::length in tiny-skia-path path/src/lib.rs. It returns the
// Euclidean distance from the origin.
func (p Point) Length() float32 {
	mag2 := float32(p.X*p.X) + float32(p.Y*p.Y)
	if isFinite(mag2) {
		return sqrtF32(mag2)
	}
	xx := float64(p.X)
	yy := float64(p.Y)
	return float32(sqrtF64(float64(xx*xx) + float64(yy*yy)))
}

// SetLength is Point::set_length in tiny-skia-path path/src/lib.rs. It scales
// the vector so that its distance to the origin is length, if it can.
//
// If the length before is nearly zero, it sets the vector to (0, 0) and
// returns false. Otherwise it returns true.
func (p *Point) SetLength(length float32) bool {
	return p.SetLengthFrom(p.X, p.Y, length)
}

// SetLengthFrom is Point::set_length_from in tiny-skia-path path/src/lib.rs.
// It sets the vector to (x, y) scaled to length, if it can.
//
// If the length before is nearly zero, it sets the vector to (0, 0) and
// returns false. Otherwise it returns true.
func (p *Point) SetLengthFrom(x, y, length float32) bool {
	return setPointLength(p, x, y, length, nil)
}

// Distance is Point::distance in tiny-skia-path path/src/lib.rs. It returns
// the Euclidean distance from other.
func (p Point) Distance(other Point) float32 {
	return p.Sub(other).Length()
}

// Dot is Point::dot in tiny-skia-path path/src/lib.rs. It returns the dot
// product of two points.
func (p Point) Dot(other Point) float32 {
	return float32(p.X*other.X) + float32(p.Y*other.Y)
}

// Cross is Point::cross in tiny-skia-path path/src/lib.rs. It returns the
// cross product of the vector and other.
//
// The vector and other form three-dimensional vectors with a z value of zero.
// The cross product is a three-dimensional vector with x and y values of zero.
// Cross returns its z component.
func (p Point) Cross(other Point) float32 {
	return float32(p.X*other.Y) - float32(p.Y*other.X)
}

// distanceToSqd is Point::distance_to_sqd in tiny-skia-path path/src/lib.rs.
func (p Point) distanceToSqd(pt Point) float32 {
	dx := p.X - pt.X
	dy := p.Y - pt.Y
	return float32(dx*dx) + float32(dy*dy)
}

// lengthSqd is Point::length_sqd in tiny-skia-path path/src/lib.rs.
func (p Point) lengthSqd() float32 {
	return p.Dot(p)
}

// Scale is Point::scale in tiny-skia-path path/src/lib.rs. It scales the point
// in place by scale.
func (p *Point) Scale(scale float32) {
	p.X = float32(p.X * scale)
	p.Y = float32(p.Y * scale)
}

// scaled is Point::scaled in tiny-skia-path path/src/lib.rs.
func (p Point) scaled(scale float32) Point {
	return PointFromXY(float32(p.X*scale), float32(p.Y*scale))
}

// swapCoords is Point::swap_coords in tiny-skia-path path/src/lib.rs.
func (p *Point) swapCoords() {
	p.X, p.Y = p.Y, p.X
}

// rotateCW is Point::rotate_cw in tiny-skia-path path/src/lib.rs.
func (p *Point) rotateCW() {
	p.swapCoords()
	p.X = -p.X
}

// rotateCCW is Point::rotate_ccw in tiny-skia-path path/src/lib.rs.
func (p *Point) rotateCCW() {
	p.swapCoords()
	p.Y = -p.Y
}

// setPointLength is set_point_length in tiny-skia-path path/src/lib.rs.
//
// We have to worry about 2 tricky conditions:
//  1. underflow of mag2 (compared against nearlyzero^2)
//  2. overflow of mag2 (compared w/ isfinite)
//
// If we underflow, we return false. If we overflow, we compute again using
// doubles, which is much slower (3x in a desktop test) but will not overflow.
//
// Port: origLength is the Rust &mut Option<f32>. A nil pointer is None.
func setPointLength(pt *Point, x, y, length float32, origLength *float32) bool {
	// our mag2 step overflowed to infinity, so use doubles instead.
	// much slower, but needed when x or y are very large, other wise we
	// divide by inf. and return (0,0) vector.
	xx := float64(x)
	yy := float64(y)
	dmag := sqrtF64(float64(xx*xx) + float64(yy*yy))
	dscale := float64(length) / dmag
	x = float32(x * float32(dscale))
	y = float32(y * float32(dscale))

	// check if we're not finite, or we're zero-length
	if !isFinite(x) || !isFinite(y) || (x == 0 && y == 0) {
		*pt = PointZero()
		return false
	}

	var mag float32
	if origLength != nil {
		mag = float32(dmag)
	}

	*pt = PointFromXY(x, y)

	if origLength != nil {
		*origLength = mag
	}

	return true
}

// Neg is the core::ops::Neg impl of Point in tiny-skia-path path/src/lib.rs.
func (p Point) Neg() Point {
	return Point{X: -p.X, Y: -p.Y}
}

// Add is the core::ops::Add impl of Point in tiny-skia-path path/src/lib.rs.
func (p Point) Add(other Point) Point {
	return PointFromXY(p.X+other.X, p.Y+other.Y)
}

// AddAssign is the core::ops::AddAssign impl of Point in tiny-skia-path
// path/src/lib.rs.
func (p *Point) AddAssign(other Point) {
	p.X += other.X
	p.Y += other.Y
}

// Sub is the core::ops::Sub impl of Point in tiny-skia-path path/src/lib.rs.
func (p Point) Sub(other Point) Point {
	return PointFromXY(p.X-other.X, p.Y-other.Y)
}

// SubAssign is the core::ops::SubAssign impl of Point in tiny-skia-path
// path/src/lib.rs.
func (p *Point) SubAssign(other Point) {
	p.X -= other.X
	p.Y -= other.Y
}

// Mul is the core::ops::Mul impl of Point in tiny-skia-path path/src/lib.rs.
func (p Point) Mul(other Point) Point {
	return PointFromXY(float32(p.X*other.X), float32(p.Y*other.Y))
}

// MulAssign is the core::ops::MulAssign impl of Point in tiny-skia-path
// path/src/lib.rs.
func (p *Point) MulAssign(other Point) {
	p.X = float32(p.X * other.X)
	p.Y = float32(p.Y * other.Y)
}
