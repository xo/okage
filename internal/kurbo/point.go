// Copyright 2019 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

// This file ports the subset of kurbo kurbo/src/point.rs that svgtypes and
// usvg need.

// Point is Point in kurbo kurbo/src/point.rs.
//
// A 2D point.
//
// This type represents a point in 2D space. It has the same layout as [Vec2], but
// its meaning is different: `Vec2` represents a change in location (for example velocity).
//
// In general, `kurbo` overloads math operators where it makes sense, for example implementing
// `Affine * Point` as the point under the affine transformation. However `Point + Point` and
// `f64 * Point` are not implemented, because the operations do not make geometric sense. If you
// need to apply these operations, then 1) check what you're doing makes geometric sense, then 2)
// use [Point.ToVec2] to convert the point to a `Vec2`.
type Point struct {
	// The x coordinate.
	X float64
	// The y coordinate.
	Y float64
}

// NewPoint is Point::new in kurbo kurbo/src/point.rs.
//
// Create a new `Point` with the provided `x` and `y` coordinates.
func NewPoint(x, y float64) Point {
	return Point{X: x, Y: y}
}

// ToVec2 is Point::to_vec2 in kurbo kurbo/src/point.rs.
//
// Convert this point into a `Vec2`.
func (p Point) ToVec2() Vec2 {
	return NewVec2(p.X, p.Y)
}

// Lerp is Point::lerp in kurbo kurbo/src/point.rs.
//
// Linearly interpolate between two points.
func (p Point) Lerp(other Point, t float64) Point {
	return p.ToVec2().Lerp(other.ToVec2(), t).ToPoint()
}

// Midpoint is Point::midpoint in kurbo kurbo/src/point.rs.
//
// Determine the midpoint of two points.
func (p Point) Midpoint(other Point) Point {
	return NewPoint(0.5*(p.X+other.X), 0.5*(p.Y+other.Y))
}

// Distance is Point::distance in kurbo kurbo/src/point.rs.
//
// Euclidean distance.
//
// See [Vec2.Hypot] for the same operation on [Vec2].
func (p Point) Distance(other Point) float64 {
	return p.Sub(other).Hypot()
}

// Add is the operator `Point + Vec2`, the impl Add<Vec2> for Point in kurbo
// kurbo/src/point.rs.
func (p Point) Add(other Vec2) Point {
	return NewPoint(p.X+other.X, p.Y+other.Y)
}

// SubVec is the operator `Point - Vec2`, the impl Sub<Vec2> for Point in
// kurbo kurbo/src/point.rs.
func (p Point) SubVec(other Vec2) Point {
	return NewPoint(p.X-other.X, p.Y-other.Y)
}

// Sub is the operator `Point - Point`, the impl Sub<Point> for Point in kurbo
// kurbo/src/point.rs.
func (p Point) Sub(other Point) Vec2 {
	return NewVec2(p.X-other.X, p.Y-other.Y)
}
