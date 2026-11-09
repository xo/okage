// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

// This file ports the subset of kurbo kurbo/src/vec2.rs that svgtypes and
// usvg need.
//
// Each product that a sum takes is converted with float64(), so that the Go
// compiler cannot fuse a multiply and an add (D16).

// Vec2 is Vec2 in kurbo kurbo/src/vec2.rs.
//
// A 2D vector.
//
// This is intended primarily for a vector in the mathematical sense,
// but it can be interpreted as a translation, and converted to and
// from a [Point] (vector relative to the origin) and `Size`.
type Vec2 struct {
	// The x-coordinate.
	X float64
	// The y-coordinate.
	Y float64
}

// NewVec2 is Vec2::new in kurbo kurbo/src/vec2.rs.
//
// Create a new vector.
func NewVec2(x, y float64) Vec2 {
	return Vec2{X: x, Y: y}
}

// ToPoint is Vec2::to_point in kurbo kurbo/src/vec2.rs.
//
// Convert this vector into a [Point].
func (v Vec2) ToPoint() Point {
	return NewPoint(v.X, v.Y)
}

// Dot is Vec2::dot in kurbo kurbo/src/vec2.rs.
//
// Dot product of two vectors.
func (v Vec2) Dot(other Vec2) float64 {
	return float64(v.X*other.X) + float64(v.Y*other.Y)
}

// Hypot is Vec2::hypot in kurbo kurbo/src/vec2.rs.
//
// Magnitude of vector.
//
// See [Point.Distance] for the same operation on [Point].
func (v Vec2) Hypot() float64 {
	// Avoid f64::hypot as it calls a slow library function.
	return Sqrt(v.Hypot2())
}

// Hypot2 is Vec2::hypot2 in kurbo kurbo/src/vec2.rs.
//
// Magnitude squared of vector.
func (v Vec2) Hypot2() float64 {
	return v.Dot(v)
}

// Atan2 is Vec2::atan2 in kurbo kurbo/src/vec2.rs.
//
// Find the angle in radians between this vector and the vector `Vec2 { x: 1.0, y: 0.0 }`
// in the positive `y` direction.
//
// If the vector is interpreted as a complex number, this is the argument.
// The angle is expressed in radians.
func (v Vec2) Atan2() float64 {
	return Atan2(v.Y, v.X)
}

// Lerp is Vec2::lerp in kurbo kurbo/src/vec2.rs.
//
// Linearly interpolate between two vectors.
func (v Vec2) Lerp(other Vec2, t float64) Vec2 {
	return v.Add(other.Sub(v).Mul(t))
}

// Add is the operator `Vec2 + Vec2`, the impl Add for Vec2 in kurbo
// kurbo/src/vec2.rs.
func (v Vec2) Add(other Vec2) Vec2 {
	return Vec2{X: v.X + other.X, Y: v.Y + other.Y}
}

// Sub is the operator `Vec2 - Vec2`, the impl Sub for Vec2 in kurbo
// kurbo/src/vec2.rs.
func (v Vec2) Sub(other Vec2) Vec2 {
	return Vec2{X: v.X - other.X, Y: v.Y - other.Y}
}

// Mul is the operator `Vec2 * f64`, the impl Mul<f64> for Vec2 in kurbo
// kurbo/src/vec2.rs. The operator `f64 * Vec2`, the impl Mul<Vec2> for f64,
// is `other * self` in Rust, so it is Mul too.
func (v Vec2) Mul(other float64) Vec2 {
	return Vec2{X: float64(v.X * other), Y: float64(v.Y * other)}
}
