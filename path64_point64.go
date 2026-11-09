// Copyright 2012 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import "github.com/xo/okage/path"

// searchAxis is SearchAxis in tiny-skia src/path64/point64.rs.
type searchAxis uint8

// The values of searchAxis.
const (
	searchAxisX searchAxis = iota
	searchAxisY
)

// point64 is Point64 in tiny-skia src/path64/point64.rs.
type point64 struct {
	x float64
	y float64
}

// point64FromXY is Point64::from_xy in tiny-skia src/path64/point64.rs.
func point64FromXY(x, y float64) point64 {
	return point64{x: x, y: y}
}

// point64FromPoint is Point64::from_point in tiny-skia
// src/path64/point64.rs.
func point64FromPoint(p Point) point64 {
	return point64{x: float64(p.X), y: float64(p.Y)}
}

// point64Zero is Point64::zero in tiny-skia src/path64/point64.rs.
//
//nolint:unused // upstream defines it, and the port keeps it
func point64Zero() point64 {
	return point64{x: 0.0, y: 0.0}
}

// toPoint is Point64::to_point in tiny-skia src/path64/point64.rs.
func (p point64) toPoint() Point {
	return path.PointFromXY(float32(p.x), float32(p.y))
}

// axisCoord is Point64::axis_coord in tiny-skia src/path64/point64.rs.
func (p point64) axisCoord(axis searchAxis) float64 {
	if axis == searchAxisX {
		return p.x
	}
	return p.y
}
