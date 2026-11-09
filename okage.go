// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import "github.com/xo/okage/path"

// The type aliases below are the re-exports of tiny_skia_path in tiny-skia
// src/lib.rs.

// IntRect is the re-export of tiny_skia_path::IntRect in tiny-skia
// src/lib.rs. It is an integer rectangle.
type IntRect = path.IntRect

// IntSize is the re-export of tiny_skia_path::IntSize in tiny-skia
// src/lib.rs. It is an integer size.
type IntSize = path.IntSize

// NonZeroRect is the re-export of tiny_skia_path::NonZeroRect in tiny-skia
// src/lib.rs. It is a rectangle with a width and a height above zero.
type NonZeroRect = path.NonZeroRect

// Point is the re-export of tiny_skia_path::Point in tiny-skia src/lib.rs. It
// is a point.
type Point = path.Point

// Rect is the re-export of tiny_skia_path::Rect in tiny-skia src/lib.rs. It
// is a rectangle.
type Rect = path.Rect

// Size is the re-export of tiny_skia_path::Size in tiny-skia src/lib.rs. It
// is a size.
type Size = path.Size

// Transform is the re-export of tiny_skia_path::Transform in tiny-skia
// src/lib.rs. It is an affine transform.
type Transform = path.Transform

// LineCap is the re-export of tiny_skia_path::LineCap in tiny-skia
// src/lib.rs. It is what a stroke draws at the ends of an open contour.
type LineCap = path.LineCap

// LineJoin is the re-export of tiny_skia_path::LineJoin in tiny-skia
// src/lib.rs. It is what a stroke draws at the corners of a path.
type LineJoin = path.LineJoin

// Stroke is the re-export of tiny_skia_path::Stroke in tiny-skia src/lib.rs.
// It holds the properties of a stroke.
type Stroke = path.Stroke

// StrokeDash is the re-export of tiny_skia_path::StrokeDash in tiny-skia
// src/lib.rs. It is a dash of a stroke.
type StrokeDash = path.StrokeDash

// Path is the re-export of tiny_skia_path::Path in tiny-skia src/lib.rs. It is
// a Bezier path.
type Path = path.Path

// PathBuilder is the re-export of tiny_skia_path::PathBuilder in tiny-skia
// src/lib.rs. It builds a Path.
type PathBuilder = path.PathBuilder

// PathSegment is the re-export of tiny_skia_path::PathSegment in tiny-skia
// src/lib.rs. It is a segment of a path.
type PathSegment = path.PathSegment

// PathSegmentKind is the kind of a PathSegment. In Rust, it is the variant of
// the enum PathSegment, which tiny-skia src/lib.rs re-exports.
type PathSegmentKind = path.PathSegmentKind

// PathSegmentsIter is the re-export of tiny_skia_path::PathSegmentsIter in
// tiny-skia src/lib.rs. It is an iterator over the segments of a path.
type PathSegmentsIter = path.PathSegmentsIter

// PathStroker is the re-export of tiny_skia_path::PathStroker in tiny-skia
// src/lib.rs. It makes the outline of a stroke.
type PathStroker = path.PathStroker

// The constants below are the variants of the enums that tiny-skia src/lib.rs
// re-exports. In Rust, a variant comes with its enum.
const (
	// LineCapButt is LineCap::Butt.
	LineCapButt = path.LineCapButt
	// LineCapRound is LineCap::Round.
	LineCapRound = path.LineCapRound
	// LineCapSquare is LineCap::Square.
	LineCapSquare = path.LineCapSquare

	// LineJoinMiter is LineJoin::Miter.
	LineJoinMiter = path.LineJoinMiter
	// LineJoinMiterClip is LineJoin::MiterClip.
	LineJoinMiterClip = path.LineJoinMiterClip
	// LineJoinRound is LineJoin::Round.
	LineJoinRound = path.LineJoinRound
	// LineJoinBevel is LineJoin::Bevel.
	LineJoinBevel = path.LineJoinBevel

	// PathSegmentMoveTo is PathSegment::MoveTo.
	PathSegmentMoveTo = path.PathSegmentMoveTo
	// PathSegmentLineTo is PathSegment::LineTo.
	PathSegmentLineTo = path.PathSegmentLineTo
	// PathSegmentQuadTo is PathSegment::QuadTo.
	PathSegmentQuadTo = path.PathSegmentQuadTo
	// PathSegmentCubicTo is PathSegment::CubicTo.
	PathSegmentCubicTo = path.PathSegmentCubicTo
	// PathSegmentClose is PathSegment::Close.
	PathSegmentClose = path.PathSegmentClose
)

// The functions below are the constructors of the types that tiny-skia
// src/lib.rs re-exports. In Rust, an associated function comes with its type.
// Each one calls the function of path with the same name.

// IntRectFromXYWH is IntRect::from_xywh. It calls path.IntRectFromXYWH.
func IntRectFromXYWH(x, y int32, width, height uint32) (IntRect, bool) {
	return path.IntRectFromXYWH(x, y, width, height)
}

// IntRectFromLTRB is IntRect::from_ltrb. It calls path.IntRectFromLTRB.
func IntRectFromLTRB(left, top, right, bottom int32) (IntRect, bool) {
	return path.IntRectFromLTRB(left, top, right, bottom)
}

// IntSizeFromWH is IntSize::from_wh. It calls path.IntSizeFromWH.
func IntSizeFromWH(width, height uint32) (IntSize, bool) {
	return path.IntSizeFromWH(width, height)
}

// NonZeroRectFromLTRB is NonZeroRect::from_ltrb. It calls
// path.NonZeroRectFromLTRB.
func NonZeroRectFromLTRB(left, top, right, bottom float32) (NonZeroRect, bool) {
	return path.NonZeroRectFromLTRB(left, top, right, bottom)
}

// NonZeroRectFromXYWH is NonZeroRect::from_xywh. It calls
// path.NonZeroRectFromXYWH.
func NonZeroRectFromXYWH(x, y, w, h float32) (NonZeroRect, bool) {
	return path.NonZeroRectFromXYWH(x, y, w, h)
}

// PointFromXY is Point::from_xy. It calls path.PointFromXY.
func PointFromXY(x, y float32) Point {
	return path.PointFromXY(x, y)
}

// PointZero is Point::zero. It calls path.PointZero.
func PointZero() Point {
	return path.PointZero()
}

// RectFromLTRB is Rect::from_ltrb. It calls path.RectFromLTRB.
func RectFromLTRB(left, top, right, bottom float32) (Rect, bool) {
	return path.RectFromLTRB(left, top, right, bottom)
}

// RectFromXYWH is Rect::from_xywh. It calls path.RectFromXYWH.
func RectFromXYWH(x, y, w, h float32) (Rect, bool) {
	return path.RectFromXYWH(x, y, w, h)
}

// RectFromPoints is Rect::from_points. It calls path.RectFromPoints.
func RectFromPoints(points []Point) (Rect, bool) {
	return path.RectFromPoints(points)
}

// SizeFromWH is Size::from_wh. It calls path.SizeFromWH.
func SizeFromWH(width, height float32) (Size, bool) {
	return path.SizeFromWH(width, height)
}

// DefaultTransform is Transform::default. It calls path.DefaultTransform.
func DefaultTransform() Transform {
	return path.DefaultTransform()
}

// IdentityTransform is Transform::identity. It calls path.IdentityTransform.
func IdentityTransform() Transform {
	return path.IdentityTransform()
}

// TransformFromRow is Transform::from_row. It calls path.TransformFromRow.
func TransformFromRow(sx, ky, kx, sy, tx, ty float32) Transform {
	return path.TransformFromRow(sx, ky, kx, sy, tx, ty)
}

// TransformFromTranslate is Transform::from_translate. It calls
// path.TransformFromTranslate.
func TransformFromTranslate(tx, ty float32) Transform {
	return path.TransformFromTranslate(tx, ty)
}

// TransformFromScale is Transform::from_scale. It calls
// path.TransformFromScale.
func TransformFromScale(sx, sy float32) Transform {
	return path.TransformFromScale(sx, sy)
}

// TransformFromSkew is Transform::from_skew. It calls path.TransformFromSkew.
func TransformFromSkew(kx, ky float32) Transform {
	return path.TransformFromSkew(kx, ky)
}

// TransformFromRotate is Transform::from_rotate. It calls
// path.TransformFromRotate.
func TransformFromRotate(angle float32) Transform {
	return path.TransformFromRotate(angle)
}

// TransformFromRotateAt is Transform::from_rotate_at. It calls
// path.TransformFromRotateAt.
func TransformFromRotateAt(angle, tx, ty float32) Transform {
	return path.TransformFromRotateAt(angle, tx, ty)
}

// TransformFromBBox is Transform::from_bbox. It calls path.TransformFromBBox.
func TransformFromBBox(bbox NonZeroRect) Transform {
	return path.TransformFromBBox(bbox)
}

// DefaultStroke is Stroke::default. It calls path.DefaultStroke.
func DefaultStroke() Stroke {
	return path.DefaultStroke()
}

// NewStrokeDash is StrokeDash::new. It calls path.NewStrokeDash.
func NewStrokeDash(dashArray []float32, dashOffset float32) (*StrokeDash, bool) {
	return path.NewStrokeDash(dashArray, dashOffset)
}

// NewPathBuilder is PathBuilder::new. It calls path.NewPathBuilder.
func NewPathBuilder() *PathBuilder {
	return path.NewPathBuilder()
}

// NewPathBuilderWithCapacity is PathBuilder::with_capacity. It calls
// path.NewPathBuilderWithCapacity.
func NewPathBuilderWithCapacity(verbsCapacity, pointsCapacity int) *PathBuilder {
	return path.NewPathBuilderWithCapacity(verbsCapacity, pointsCapacity)
}

// PathFromRect is PathBuilder::from_rect. It calls path.PathFromRect.
func PathFromRect(rect Rect) *Path {
	return path.PathFromRect(rect)
}

// PathFromCircle is PathBuilder::from_circle. It calls path.PathFromCircle.
func PathFromCircle(cx, cy, radius float32) (*Path, bool) {
	return path.PathFromCircle(cx, cy, radius)
}

// PathFromOval is PathBuilder::from_oval. It calls path.PathFromOval.
func PathFromOval(oval Rect) (*Path, bool) {
	return path.PathFromOval(oval)
}

// NewPathStroker is PathStroker::new. It calls path.NewPathStroker.
func NewPathStroker() *PathStroker {
	return path.NewPathStroker()
}

// ComputeResolutionScale is PathStroker::compute_resolution_scale. It calls
// path.ComputeResolutionScale.
func ComputeResolutionScale(ts Transform) float32 {
	return path.ComputeResolutionScale(ts)
}

// lengthU32 is LengthU32 in tiny-skia src/lib.rs. It is an integer length
// that is guaranteed to be > 0.
//
// Port: Rust uses core::num::NonZeroU32. Go has no such type, so each
// constructor checks that the value is not zero.
type lengthU32 = uint32
