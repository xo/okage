// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"math"

	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// The types of krilla src/geom.rs wrap the types of tiny-skia-path. The port
// takes the types of the package path in their place, as the root package of
// okage does, so that a program passes an okage.Path or an okage.Transform to
// pdf without a conversion (D62). The functions of upstream that make and
// read them, such as Rect::from_xywh and Transform::invert, are the functions
// and the methods of path. The methods that krilla adds for the crate are the
// functions below.

// Point is Point in krilla src/geom.rs.
//
// A point.
type Point = path.Point

// Size is Size in krilla src/geom.rs. path.SizeFromWH makes one.
//
// A size.
type Size = path.Size

// Rect is Rect in krilla src/geom.rs. path.RectFromLTRB and
// path.RectFromXYWH make one.
//
// A rectangle defined by left, top, right and bottom edges.
type Rect = path.Rect

// Transform is Transform in krilla src/geom.rs.
//
// An affine transformation matrix.
//
// Unlike other types, doesn't guarantee to be valid. This is Skia quirk.
// Meaning Transform(0, 0, 0, 0, 0, 0) is ok, while it's technically not.
// Non-finite values are also not an error.
type Transform = path.Transform

// Path is Path in krilla src/geom.rs.
//
// A path.
type Path = path.Path

// PathBuilder is PathBuilder in krilla src/geom.rs.
//
// A path builder.
type PathBuilder = path.PathBuilder

// rectExpand is Rect::expand in krilla src/geom.rs. It grows r to hold
// other. krilla unwraps the rectangle of the union. A union that is not a
// valid rectangle, which takes a value that is not finite, leaves r as it is
// in the port.
func rectExpand(r *Rect, other Rect) {
	left := min(r.Left(), other.Left())
	top := min(r.Top(), other.Top())
	right := max(r.Right(), other.Right())
	bottom := max(r.Bottom(), other.Bottom())
	if u, ok := path.RectFromLTRB(left, top, right, bottom); ok {
		*r = u
	}
}

// rectToPDFRect is Rect::to_pdf_rect in krilla src/geom.rs. It adds the width
// to the left edge, as upstream does, which can round to another value than
// the right edge.
func rectToPDFRect(r Rect) pdfwriter.Rect {
	return pdfwriter.NewRect(
		r.Left(),
		r.Top(),
		r.Left()+r.Width(),
		r.Top()+r.Height(),
	)
}

// Quadrilateral is Quadrilateral in krilla src/geom.rs.
//
// A quadrilateral.
//
// The points should be given in the following order:
//  1. bottom-left
//  2. bottom-right
//  3. top-right
//  4. top-left
//
// Where the coordinate system is Y-down.
type Quadrilateral [4]Point

// QuadrilateralFromRect is the impl From<Rect> for Quadrilateral in krilla
// src/geom.rs.
func QuadrilateralFromRect(r Rect) Quadrilateral {
	return Quadrilateral{
		path.PointFromXY(r.Left(), r.Bottom()),
		path.PointFromXY(r.Right(), r.Bottom()),
		path.PointFromXY(r.Right(), r.Top()),
		path.PointFromXY(r.Left(), r.Top()),
	}
}

// transformToPDF is Transform::to_pdf_transform in krilla src/geom.rs. It
// gives the six numbers of the operator cm.
func transformToPDF(t Transform) [6]float32 {
	return [6]float32{t.SX, t.KY, t.KX, t.SY, t.TX, t.TY}
}

// rect is the impl Hash for Rect in krilla src/geom.rs. It hashes the bits of
// the edges.
func (h *hasher) rect(r Rect) {
	h.u32(math.Float32bits(r.Left()))
	h.u32(math.Float32bits(r.Top()))
	h.u32(math.Float32bits(r.Right()))
	h.u32(math.Float32bits(r.Bottom()))
}

// transform is the impl Hash for Transform in krilla src/geom.rs. It hashes
// the bits of the six numbers, in the order of upstream.
func (h *hasher) transform(t Transform) {
	h.u32(math.Float32bits(t.TX))
	h.u32(math.Float32bits(t.TY))
	h.u32(math.Float32bits(t.SX))
	h.u32(math.Float32bits(t.SY))
	h.u32(math.Float32bits(t.KX))
	h.u32(math.Float32bits(t.KY))
}
