// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"fmt"
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/crmath"
	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// This file ports resvg src/render.rs.
//
// Port: a function of upstream that returns Option<()> returns a bool, and
// false is None. The caller of upstream ignores most of these results.

// context is Context in resvg src/render.rs.
//
// Port: opt is the options of Render, which carry the logger of D20.
// Upstream has no such field, and logs with the crate log.
type context struct {
	maxBBox path.IntRect
	opt     *options
}

// renderNodes is render_nodes in resvg src/render.rs.
func renderNodes(parent *Group, ctx *context, transform Transform, pixmap *okage.PixmapMut) {
	for _, node := range parent.Children() {
		renderNode(node, ctx, transform, pixmap)
	}
}

// renderNode is render_node in resvg src/render.rs.
func renderNode(node Node, ctx *context, transform Transform, pixmap *okage.PixmapMut) {
	switch n := node.(type) {
	case *Group:
		renderGroup(n, ctx, transform, pixmap)
	case *Path:
		renderPath(
			n,
			okage.BlendModeSourceOver,
			ctx,
			transform,
			pixmap,
		)
	case *Image:
		renderImage(n, transform, pixmap, ctx.opt)
	case *Text:
		renderGroup(n.Flattened(), ctx, transform, pixmap)
	}
}

// renderGroup is render_group in resvg src/render.rs.
func renderGroup(group *Group, ctx *context, transform Transform, pixmap *okage.PixmapMut) bool { //nolint:unparam // upstream returns Option<()>
	transform = transform.PreConcat(group.Transform())

	if !group.ShouldIsolate() {
		renderNodes(group, ctx, transform, pixmap)
		return true
	}

	var rest Transform
	if len(group.Filters()) == 0 {
		rest = path.IdentityTransform()
	} else {
		sx := crmath.Hypot32(transform.SX, transform.KY)
		sy := crmath.Hypot32(transform.KX, transform.SY)
		rest = path.TransformFromRow(
			transform.SX/sx,
			transform.KY/sx,
			transform.KX/sy,
			transform.SY/sy,
			0.0,
			0.0,
		)
	}
	invRest, ok := rest.Invert()
	if !ok {
		return false
	}

	maxRect, ok := ctx.maxBBox.ToRect().Transform(invRest)
	if !ok {
		return false
	}
	maxBBox, ok := maxRect.RoundOut()
	if !ok {
		return false
	}
	layer, ibbox, ok := renderLayer(group, &context{maxBBox: maxBBox, opt: ctx.opt}, invRest.PreConcat(transform))
	if !ok {
		return false
	}

	paint := okage.DefaultPaint()
	paint.Shader = okage.NewPattern(
		layer.AsRef(),
		okage.SpreadModePad,
		okage.FilterQualityBilinear,
		group.Opacity().Get(),
		path.TransformFromTranslate(float32(ibbox.X()), float32(ibbox.Y())),
	)
	paint.BlendMode = convertBlendMode(group.BlendMode())
	paint.AntiAlias = true
	rectPath := path.PathFromRect(ibbox.ToRect())
	pixmap.FillPath(rectPath, &paint, okage.FillRuleWinding, rest, nil)

	return true
}

// renderLayer is render_layer in resvg src/render.rs. It renders an isolated
// group into a new layer, returning it and its position.
func renderLayer(group *Group, ctx *context, transform Transform) (*okage.Pixmap, path.IntRect, bool) {
	layerBBox, ok := group.LayerBoundingBox()
	if !ok {
		return nil, path.IntRect{}, false
	}
	bbox, ok := layerBBox.Transform(transform)
	if !ok {
		return nil, path.IntRect{}, false
	}

	var ibbox path.IntRect
	if len(group.Filters()) == 0 {
		// Convert group bbox into an integer one, expanding each side outwards by 2px
		// to make sure that anti-aliased pixels would not be clipped.
		x, ok := checkedSubI32(wide.TruncI32(floorF32(bbox.X())), 2)
		if !ok {
			return nil, path.IntRect{}, false
		}
		y, ok := checkedSubI32(wide.TruncI32(floorF32(bbox.Y())), 2)
		if !ok {
			return nil, path.IntRect{}, false
		}
		w, ok := checkedAddU32(wide.TruncU32(ceilF32(bbox.Width())), 4)
		if !ok {
			return nil, path.IntRect{}, false
		}
		h, ok := checkedAddU32(wide.TruncU32(ceilF32(bbox.Height())), 4)
		if !ok {
			return nil, path.IntRect{}, false
		}
		ibbox, ok = path.IntRectFromXYWH(x, y, w, h)
		if !ok {
			return nil, path.IntRect{}, false
		}
	} else {
		// The bounding box for groups with filters is special and should not be expanded by 2px,
		// because it's already acting as a clipping region.
		r, ok := path.IntRectFromXYWH(
			wide.TruncI32(floorF32(bbox.X())),
			wide.TruncI32(floorF32(bbox.Y())),
			wide.TruncU32(maxF32(ceilF32(bbox.Width()), 1.0)),
			wide.TruncU32(maxF32(ceilF32(bbox.Height()), 1.0)),
		)
		if !ok {
			return nil, path.IntRect{}, false
		}
		// Make sure our filter region is not bigger than 4x the canvas size.
		// This is required mainly to prevent huge filter regions that would tank the performance.
		// It should not affect the final result in any way.
		ibbox, ok = fitToRect(r, ctx.maxBBox)
		if !ok {
			return nil, path.IntRect{}, false
		}
	}

	// Make sure our layer is not bigger than 4x the canvas size.
	// This is required to prevent huge layers.
	if len(group.Filters()) == 0 {
		ibbox, ok = fitToRect(ibbox, ctx.maxBBox)
		if !ok {
			return nil, path.IntRect{}, false
		}
	}

	var shiftTs Transform
	{
		// Original shift.
		dx := bbox.X()
		dy := bbox.Y()

		// Account for subpixel positioned layers.
		dx -= bbox.X() - float32(ibbox.X())
		dy -= bbox.Y() - float32(ibbox.Y())

		shiftTs = path.TransformFromTranslate(-dx, -dy)
	}

	transform = shiftTs.PreConcat(transform)

	subPixmap, ok := newPixmap(ibbox.Width(), ibbox.Height(), ctx.opt)
	subPixmap, ok = logNone(subPixmap, ok, func() {
		ctx.opt.warnf("Failed to allocate a group layer for: %s.", debugIntRect(ibbox))
	})
	if !ok {
		return nil, path.IntRect{}, false
	}

	subMut := subPixmap.AsMut()
	renderNodes(group, ctx, transform, &subMut)

	if len(group.Filters()) != 0 {
		for _, filter := range group.Filters() {
			applyFilter(filter, transform, subPixmap, ctx.opt)
		}
	}

	if clipPath, ok := group.ClipPath(); ok {
		applyClip(clipPath, transform, subPixmap, ctx.opt)
	}

	if mask, ok := group.Mask(); ok {
		applyMask(mask, ctx, transform, subPixmap)
	}

	return subPixmap, ibbox, true
}

// convertBlendMode is convert_blend_mode in resvg src/render.rs.
func convertBlendMode(mode BlendMode) okage.BlendMode {
	switch mode {
	case BlendModeNormal:
		return okage.BlendModeSourceOver
	case BlendModeMultiply:
		return okage.BlendModeMultiply
	case BlendModeScreen:
		return okage.BlendModeScreen
	case BlendModeOverlay:
		return okage.BlendModeOverlay
	case BlendModeDarken:
		return okage.BlendModeDarken
	case BlendModeLighten:
		return okage.BlendModeLighten
	case BlendModeColorDodge:
		return okage.BlendModeColorDodge
	case BlendModeColorBurn:
		return okage.BlendModeColorBurn
	case BlendModeHardLight:
		return okage.BlendModeHardLight
	case BlendModeSoftLight:
		return okage.BlendModeSoftLight
	case BlendModeDifference:
		return okage.BlendModeDifference
	case BlendModeExclusion:
		return okage.BlendModeExclusion
	case BlendModeHue:
		return okage.BlendModeHue
	case BlendModeSaturation:
		return okage.BlendModeSaturation
	case BlendModeColor:
		return okage.BlendModeColor
	case BlendModeLuminosity:
		return okage.BlendModeLuminosity
	case BlendModePlusLighter:
		return okage.BlendModePlus
	}
	return okage.BlendModeSourceOver
}

// The helpers below port no upstream function. They give the result of the
// methods of f32, i32 and u32 in the Rust standard library, which resvg
// calls.

// floorF32 is f32::floor.
func floorF32(x float32) float32 {
	return float32(math.Floor(float64(x)))
}

// ceilF32 is f32::ceil.
func ceilF32(x float32) float32 {
	return float32(math.Ceil(float64(x)))
}

// checkedSubI32 is i32::checked_sub. It reports false on an overflow.
func checkedSubI32(a, b int32) (int32, bool) {
	r := a - b
	if (b > 0 && r > a) || (b < 0 && r < a) {
		return 0, false
	}
	return r, true
}

// checkedAddU32 is u32::checked_add. It reports false on an overflow.
func checkedAddU32(a, b uint32) (uint32, bool) {
	r := a + b
	if r < a {
		return 0, false
	}
	return r, true
}

// debugIntRect is the derived impl Debug for tiny_skia::IntRect, which the
// warning of renderLayer prints with {:?}.
func debugIntRect(r path.IntRect) string {
	return fmt.Sprintf("IntRect { x: %d, y: %d, width: %d, height: %d }", r.X(), r.Y(), r.Width(), r.Height())
}
