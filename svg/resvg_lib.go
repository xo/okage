// Copyright 2017 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// This file ports resvg src/lib.rs. resvg is an SVG rendering library.
//
// Port: upstream logs with the crate log, which has one global logger. Go
// logs to the logger of WithLogger (D20), so Render and RenderNode take the
// options of the parser, and read only WithLogger from them. The private
// type context of resvg src/render.rs carries the options to each function
// that logs, and each pixmap and mask that the renderer makes logs the
// warnings of the root package to the same logger.
//
// The trait OptionLog of upstream is the function logNone in
// usvg_parser.go, which is the same trait of usvg src/parser/mod.rs.

// Render is render in resvg src/lib.rs. It renders a tree onto the pixmap.
//
// `transform` will be used as a root transform.
// Can be used to position SVG inside the `pixmap`.
//
// The produced content is in the sRGB color space.
//
// Port: opts are the options of the parser. Render reads only WithLogger, and
// logs the warnings of upstream to that logger (D20).
func Render(tree *Tree, transform Transform, pixmap *okage.PixmapMut, opts ...Option) {
	opt := newOptions(opts...)
	maxBBox := maxFilterBBox(pixmap.Width(), pixmap.Height())

	ctx := &context{maxBBox: maxBBox, opt: opt}
	renderNodes(tree.Root(), ctx, transform, pixmap)
}

// RenderNode is render_node in resvg src/lib.rs. It renders a node onto the
// pixmap.
//
// `transform` will be used as a root transform.
// Can be used to position SVG inside the `pixmap`.
//
// The expected pixmap size can be retrieved from `usvg::Node::abs_layer_bounding_box()`.
//
// Returns `None` when `node` has a zero size.
//
// The produced content is in the sRGB color space.
//
// Port: it returns false where upstream returns None. opts are the options of
// the parser, as for Render.
func RenderNode(node Node, transform Transform, pixmap *okage.PixmapMut, opts ...Option) bool {
	bbox, ok := NodeAbsLayerBoundingBox(node)
	if !ok {
		return false
	}

	opt := newOptions(opts...)
	maxBBox := maxFilterBBox(pixmap.Width(), pixmap.Height())

	transform = transform.PreTranslate(-bbox.X(), -bbox.Y())

	ctx := &context{maxBBox: maxBBox, opt: opt}
	renderNode(node, ctx, transform, pixmap)

	return true
}

// maxFilterBBox is max_filter_bbox in resvg src/lib.rs.
func maxFilterBBox(width, height uint32) path.IntRect {
	r, ok := path.IntRectFromXYWH(
		saturatingMulI32(i32TryFromU32(width), -2),
		saturatingMulI32(i32TryFromU32(height), -2),
		saturatingMulU32(width, 5),
		saturatingMulU32(height, 5),
	)
	if ok {
		return r
	}
	r, ok = path.IntRectFromLTRB(math.MinInt32/2, math.MinInt32/2, math.MaxInt32/2, math.MaxInt32/2)
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	return r
}

// i32TryFromU32 is i32::try_from(v).unwrap_or(i32::MAX) of Rust, which
// maxFilterBBox calls.
func i32TryFromU32(v uint32) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(v)
}

// saturatingMulI32 is i32::saturating_mul of Rust.
func saturatingMulI32(a, b int32) int32 {
	r := int64(a) * int64(b)
	switch {
	case r > math.MaxInt32:
		return math.MaxInt32
	case r < math.MinInt32:
		return math.MinInt32
	}
	return int32(r)
}

// saturatingMulU32 is u32::saturating_mul of Rust.
func saturatingMulU32(a, b uint32) uint32 {
	r := uint64(a) * uint64(b)
	if r > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(r)
}

// newPixmap is tiny_skia::Pixmap::new, which resvg calls. The pixmap logs to
// the logger of opt. It ports no upstream function.
func newPixmap(width, height uint32, opt *options) (*okage.Pixmap, bool) {
	return okage.NewPixmap(width, height, okage.WithLogger(opt.logger))
}

// mustPixmap is the unwrap of an Option<Pixmap> that upstream relies on. It
// panics when ok is false.
func mustPixmap(p *okage.Pixmap, ok bool) *okage.Pixmap {
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	return p
}
