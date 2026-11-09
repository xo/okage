// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports tiny-skia tests/integration/fill.rs.

import (
	"testing"

	"github.com/xo/okage"
)

func TestFillHorizontalLine(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(10, 10)
	pb.LineTo(90, 10)
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/empty.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillVerticalLine(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(10, 10)
	pb.LineTo(10, 90)
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/empty.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillSingleLine(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(10, 10)
	pb.LineTo(90, 90)
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/empty.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillIntRect(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	rect := must(okage.RectFromXYWH(10, 15, 80, 70))

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/int-rect.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillFloatRect(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	rect := must(okage.RectFromXYWH(10.3, 15.4, 80.5, 70.6))

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/float-rect.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillIntRectAa(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	rect := must(okage.RectFromXYWH(10, 15, 80, 70))

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/int-rect-aa.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillFloatRectAa(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	rect := must(okage.RectFromXYWH(10.3, 15.4, 80.5, 70.6))

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/float-rect-aa.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillFloatRectAaHighp(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true
	paint.ForceHQPipeline = true

	rect := must(okage.RectFromXYWH(10.3, 15.4, 80.5, 70.6))

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/float-rect-aa-highp.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillTinyFloatRect(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	rect := must(okage.RectFromXYWH(1.3, 1.4, 0.5, 0.6))
	pixmap := must(okage.NewPixmap(3, 3))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)

	assertPixelsEqual(t, pixmap.Pixels(), []okage.PremultipliedColorU8{
		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),
		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),
		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),

		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),
		okage.ColorU8FromRGBA(50, 127, 150, 200).Premultiply(),
		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),

		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),
		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),
		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),
	})
}

func TestFillTinyFloatRectAa(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	rect := must(okage.RectFromXYWH(1.3, 1.4, 0.5, 0.6))

	pixmap := must(okage.NewPixmap(3, 3))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)

	assertPixelsEqual(t, pixmap.Pixels(), []okage.PremultipliedColorU8{
		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),
		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),
		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),

		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),
		okage.ColorU8FromRGBA(51, 128, 153, 60).Premultiply(),
		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),

		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),
		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),
		okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(),
	})
}

func TestFillTinyRectAa(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(0, 0, 0, 0)
	paint.AntiAlias = true
	rect := must(okage.RectFromXYWH(0.7, 0, 1, 2))
	pixmap := must(okage.NewPixmap(10, 10))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)
}

func TestFillFloatRectClipTopLeftAa(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	rect := must(okage.RectFromXYWH(-10.3, -20.4, 100.5, 70.2))

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/float-rect-clip-top-left-aa.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillFloatRectClipTopRightAa(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	rect := must(okage.RectFromXYWH(60.3, -20.4, 100.5, 70.2))

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/float-rect-clip-top-right-aa.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillFloatRectClipBottomRightAa(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	rect := must(okage.RectFromXYWH(60.3, 40.4, 100.5, 70.2))

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/float-rect-clip-bottom-right-aa.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillIntRectWithTsClipRight(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	rect := must(okage.RectFromXYWH(0, 0, 100, 100))

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillRect(rect, &paint, okage.TransformFromRow(1, 0, 0, 1, 0.5, 0.5), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/int-rect-with-ts-clip-right.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillOpenPolygon(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(75.160671, 88.756136)
	pb.LineTo(24.797274, 88.734053)
	pb.LineTo(9.255130, 40.828792)
	pb.LineTo(50.012955, 11.243795)
	pb.LineTo(90.744819, 40.864522)
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/polygon.png"))
	assertPixmapEqual(t, pixmap, expected)
}

// Must be the same a open.
func TestFillClosedPolygon(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(75.160671, 88.756136)
	pb.LineTo(24.797274, 88.734053)
	pb.LineTo(9.255130, 40.828792)
	pb.LineTo(50.012955, 11.243795)
	pb.LineTo(90.744819, 40.864522)
	pb.Close() // the only difference
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/polygon.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillWindingStar(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(50, 7.5)
	pb.LineTo(75, 87.5)
	pb.LineTo(10, 37.5)
	pb.LineTo(90, 37.5)
	pb.LineTo(25, 87.5)
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/winding-star.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillEvenOddStar(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(50, 7.5)
	pb.LineTo(75, 87.5)
	pb.LineTo(10, 37.5)
	pb.LineTo(90, 37.5)
	pb.LineTo(25, 87.5)
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleEvenOdd, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/even-odd-star.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillQuadCurve(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(10, 15)
	pb.QuadTo(95, 35, 75, 90)
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleEvenOdd, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/quad.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillCubicCurve(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(10, 15)
	pb.CubicTo(95, 35, 0, 75, 75, 90)
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleEvenOdd, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/cubic.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillMemset2d(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 255) // Must be opaque to trigger memset2d.
	paint.AntiAlias = false

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 90, 90)))

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/memset2d.png"))
	assertPixmapEqual(t, pixmap, expected)
}

// Make sure we do not write past pixmap memory.
func TestFillMemset2dOutOfBounds(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 255) // Must be opaque to trigger memset2d.
	paint.AntiAlias = false

	path := okage.PathFromRect(must(okage.RectFromLTRB(50, 50, 120, 120)))

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/memset2d-2.png"))
	assertPixmapEqual(t, pixmap, expected)
}

// Not sure how to properly test anti-aliasing,
// so for now simply check that it actually applied.
func TestFillFillAa(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pb := okage.NewPathBuilder()
	pb.MoveTo(50, 7.5)
	pb.LineTo(75, 87.5)
	pb.LineTo(10, 37.5)
	pb.LineTo(90, 37.5)
	pb.LineTo(25, 87.5)
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleEvenOdd, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/star-aa.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillOverflowInWalkEdges1(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.CubicTo(39, 163, 117, 61, 130, 70)
	path := must(pb.Finish())

	// Must not panic.
	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
}

func TestFillClipLine1(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(50, -15)
	pb.LineTo(-15, 50)
	pb.LineTo(50, 115)
	pb.LineTo(115, 50)
	pb.Close()
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/clip-line-1.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillClipLine2(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	// This strange path forces `line_clipper::clip` to return an empty array.
	// And we're checking that this case is handled correctly.
	pb := okage.NewPathBuilder()
	pb.MoveTo(0, -1)
	pb.LineTo(50, 0)
	pb.LineTo(0, 50)
	pb.Close()
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/clip-line-2.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillClipQuad(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(10, 85)
	pb.QuadTo(150, 150, 85, 15)
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/clip-quad.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillClipCubic1(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	// `line_clipper::clip` produces 2 points for this path.
	pb := okage.NewPathBuilder()
	pb.MoveTo(10, 50)
	pb.CubicTo(0, 175, 195, 70, 75, 20)
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/clip-cubic-1.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillClipCubic2(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	// `line_clipper::clip` produces 3 points for this path.
	pb := okage.NewPathBuilder()
	pb.MoveTo(10, 50)
	pb.CubicTo(10, 40, 90, 120, 125, 20)
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/clip-cubic-2.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillAaEndlessLoop(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = true

	// This path was causing an endless loop before.
	pb := okage.NewPathBuilder()
	pb.MoveTo(2.1537175, 11.560721)
	pb.QuadTo(1.9999998, 10.787931, 2, 10)
	path := must(pb.Finish())

	// Must not loop.
	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
}

func TestFillClearAa(t *testing.T) {
	// Make sure that Clear with AA doesn't fallback to memset.
	paint := okage.DefaultPaint()
	paint.AntiAlias = true
	paint.BlendMode = okage.BlendModeClear

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.Fill(okage.ColorFromRGBA8(50, 127, 150, 200))
	pixmap.FillPath(
		must(okage.PathFromCircle(50, 50, 40)),
		&paint,
		okage.FillRuleWinding,
		okage.IdentityTransform(),
		nil,
	)

	expected := check(okage.LoadPNG("testdata/images/fill/clear-aa.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillLineCurve(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = true

	path := func() *okage.Path {
		pb := okage.NewPathBuilder()
		pb.MoveTo(100, 20)
		pb.CubicTo(100, 40, 100, 160, 100, 180) // Just a line.
		return must(pb.Finish())
	}()

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	// Must not panic.
}

func TestFillVerticalLinesMergingBug(t *testing.T) {
	// This path must not trigger edge_builder::combine_vertical,
	// otherwise AlphaRuns::add will crash later.
	pb := okage.NewPathBuilder()
	pb.MoveTo(765.56, 158.56)
	pb.LineTo(754.4, 168.28)
	pb.CubicTo(754.4, 168.28, 754.4, 168.24, 754.4, 168.17)
	pb.CubicTo(754.4, 168.09, 754.4, 168.02, 754.4, 167.95)
	pb.LineTo(754.4, 168.06)
	path := must(pb.Finish())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	// Must not panic.
	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.TransformFromRow(5.4, 0, 0, 5.4, -4050, -840), nil)

	expected := check(okage.LoadPNG("testdata/images/fill/vertical-lines-merging-bug.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestFillFillRect(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillRect(
		must(okage.RectFromXYWH(20.3, 10.4, 50.5, 30.2)),
		&paint,
		okage.TransformFromRow(1.2, 0.3, -0.7, 0.8, 12, 15.3),
		nil,
	)

	expected := check(okage.LoadPNG("testdata/images/canvas/fill-rect.png"))
	assertPixmapEqual(t, pixmap, expected)
}
