// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports tiny-skia tests/integration/mask.rs.

import (
	"testing"

	"github.com/xo/okage"
)

func TestMaskRect(t *testing.T) {
	clipPath := okage.PathFromRect(must(okage.RectFromXYWH(10, 10, 80, 80)))
	mask := must(okage.NewMask(100, 100))
	mask.FillPath(clipPath, okage.FillRuleWinding, false, okage.DefaultTransform())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pixmap := must(okage.NewPixmap(100, 100))
	rect := must(okage.RectFromXYWH(0, 0, 100, 100))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), mask)

	expected := check(okage.LoadPNG("testdata/images/mask/rect.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestMaskRectAa(t *testing.T) {
	clipPath := okage.PathFromRect(must(okage.RectFromXYWH(10.5, 10, 80, 80.5)))
	mask := must(okage.NewMask(100, 100))
	mask.FillPath(clipPath, okage.FillRuleWinding, true, okage.DefaultTransform())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pixmap := must(okage.NewPixmap(100, 100))
	rect := must(okage.RectFromXYWH(0, 0, 100, 100))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), mask)

	expected := check(okage.LoadPNG("testdata/images/mask/rect-aa.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestMaskRectTs(t *testing.T) {
	pixmap := must(okage.NewPixmap(100, 100))

	clipPath := okage.PathFromRect(must(okage.RectFromXYWH(10, 10, 80, 80)))
	clipPath = must(clipPath.Transform(okage.TransformFromRow(1, -0.3, 0, 1, 0, 15)))

	mask := must(okage.NewMask(100, 100))
	mask.FillPath(clipPath, okage.FillRuleWinding, false, okage.DefaultTransform())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	rect := must(okage.RectFromXYWH(0, 0, 100, 100))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), mask)

	expected := check(okage.LoadPNG("testdata/images/mask/rect-ts.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestMaskCircleBottomRightAa(t *testing.T) {
	pixmap := must(okage.NewPixmap(100, 100))

	clipPath := must(okage.PathFromCircle(100, 100, 50))
	mask := must(okage.NewMask(100, 100))
	mask.FillPath(clipPath, okage.FillRuleWinding, true, okage.DefaultTransform())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	rect := must(okage.RectFromXYWH(0, 0, 100, 100))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), mask)

	expected := check(okage.LoadPNG("testdata/images/mask/circle-bottom-right-aa.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestMaskStroke(t *testing.T) {
	pixmap := must(okage.NewPixmap(100, 100))

	clipPath := okage.PathFromRect(must(okage.RectFromXYWH(10, 10, 80, 80)))
	mask := must(okage.NewMask(100, 100))
	mask.FillPath(clipPath, okage.FillRuleWinding, false, okage.DefaultTransform())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	stroke := okage.DefaultStroke()
	stroke.Width = 10

	path := okage.PathFromRect(must(okage.RectFromXYWH(10, 10, 80, 80)))
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), mask)

	expected := check(okage.LoadPNG("testdata/images/mask/stroke.png"))
	assertPixmapEqual(t, pixmap, expected)
}

// Make sure we're clipping only source and not source and destination.
func TestMaskSkipDest(t *testing.T) {
	pixmap := must(okage.NewPixmap(100, 100))

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pixmap.FillPath(
		okage.PathFromRect(must(okage.RectFromXYWH(5, 5, 60, 60))),
		&paint,
		okage.FillRuleWinding,
		okage.IdentityTransform(),
		nil,
	)

	pixmap2 := must(okage.NewPixmap(200, 200))
	pixmap2Mut := pixmap2.AsMut()
	pixmap2Mut.FillPath(
		okage.PathFromRect(must(okage.RectFromXYWH(35, 35, 60, 60))),
		&paint,
		okage.FillRuleWinding,
		okage.IdentityTransform(),
		nil,
	)

	clipPath := okage.PathFromRect(must(okage.RectFromXYWH(40, 40, 40, 40)))
	mask := must(okage.NewMask(100, 100))
	mask.FillPath(clipPath, okage.FillRuleWinding, true, okage.DefaultTransform())

	pixmapPaint := okage.DefaultPixmapPaint()
	pixmap.DrawPixmap(0, 0, pixmap2.AsRef(), &pixmapPaint,
		okage.IdentityTransform(), mask)

	expected := check(okage.LoadPNG("testdata/images/mask/skip-dest.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestMaskIntersectAa(t *testing.T) {
	circle1 := must(okage.PathFromCircle(75, 75, 50))
	circle2 := must(okage.PathFromCircle(125, 125, 50))

	mask := must(okage.NewMask(200, 200))
	mask.FillPath(circle1, okage.FillRuleWinding, true, okage.DefaultTransform())
	mask.IntersectPath(circle2, okage.FillRuleWinding, true, okage.DefaultTransform())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillRect(
		must(okage.RectFromXYWH(0, 0, 200, 200)),
		&paint,
		okage.IdentityTransform(),
		mask,
	)

	expected := check(okage.LoadPNG("testdata/images/mask/intersect-aa.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestMaskIgnoreMemset(t *testing.T) {
	clipPath := okage.PathFromRect(must(okage.RectFromXYWH(10, 10, 80, 80)))

	mask := must(okage.NewMask(100, 100))
	mask.FillPath(clipPath, okage.FillRuleWinding, false, okage.DefaultTransform())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 255)
	paint.AntiAlias = false

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillRect(
		must(okage.RectFromXYWH(0, 0, 100, 100)),
		&paint,
		okage.IdentityTransform(),
		mask,
	)

	expected := check(okage.LoadPNG("testdata/images/mask/ignore-memset.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestMaskIgnoreSource(t *testing.T) {
	clipPath := okage.PathFromRect(must(okage.RectFromXYWH(10, 10, 80, 80)))

	mask := must(okage.NewMask(100, 100))
	mask.FillPath(clipPath, okage.FillRuleWinding, false, okage.DefaultTransform())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 255) // Must be opaque.
	paint.BlendMode = okage.BlendModeSourceOver
	paint.AntiAlias = false

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.Fill(okage.ColorWhite)
	pixmap.FillRect(
		must(okage.RectFromXYWH(0, 0, 100, 100)),
		&paint,
		okage.IdentityTransform(),
		mask,
	)

	expected := check(okage.LoadPNG("testdata/images/mask/ignore-source.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestMaskApplyMask(t *testing.T) {
	pixmap := must(okage.NewPixmap(100, 100))

	clipPath := must(okage.PathFromCircle(100, 100, 50))
	mask := must(okage.NewMask(100, 100))
	mask.FillPath(clipPath, okage.FillRuleWinding, true, okage.DefaultTransform())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	rect := must(okage.RectFromXYWH(0, 0, 100, 100))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)
	pixmap.ApplyMask(mask)

	expected := check(okage.LoadPNG("testdata/images/mask/apply-mask.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestMaskMaskFromAlpha(t *testing.T) {
	path := must(okage.PathFromCircle(100, 100, 50))

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.DefaultTransform(), nil)

	mask := okage.MaskFromPixmap(pixmap.AsRef(), okage.MaskTypeAlpha)

	expected := check(okage.LoadMaskPNG("testdata/images/mask/mask-from-alpha.png"))
	assertMaskEqual(t, mask, expected)
}

func TestMaskMaskFromLuma(t *testing.T) {
	path := must(okage.PathFromCircle(100, 100, 50))

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.DefaultTransform(), nil)

	mask := okage.MaskFromPixmap(pixmap.AsRef(), okage.MaskTypeLuminance)

	expected := check(okage.LoadMaskPNG("testdata/images/mask/mask-from-luma.png"))
	assertMaskEqual(t, mask, expected)
}
