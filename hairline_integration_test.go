// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports tiny-skia tests/integration/hairline.rs.

import (
	"testing"

	"github.com/xo/okage"
)

func drawLine(x0, y0, x1, y1 float32, antiAlias bool, width float32, lineCap okage.LineCap) *okage.Pixmap {
	pixmap := must(okage.NewPixmap(100, 100))

	pb := okage.NewPathBuilder()
	pb.MoveTo(x0, y0)
	pb.LineTo(x1, y1)
	path := must(pb.Finish())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = antiAlias

	stroke := okage.DefaultStroke()
	stroke.Width = width
	stroke.LineCap = lineCap
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)

	return pixmap
}

func TestHairlineHline05(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/hline-05.png"))
	assertPixmapEqual(t, drawLine(10, 10, 90, 10, false, 0.5, okage.LineCapButt), expected)
}

func TestHairlineHline05Aa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/hline-05-aa.png"))
	assertPixmapEqual(t, drawLine(10, 10, 90, 10, true, 0.5, okage.LineCapButt), expected)
}

func TestHairlineHline05AaRound(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/hline-05-aa-round.png"))
	assertPixmapEqual(t, drawLine(10, 10, 90, 10, true, 0.5, okage.LineCapRound), expected)
}

func TestHairlineVline05(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/vline-05.png"))
	assertPixmapEqual(t, drawLine(10, 10, 10, 90, false, 0.5, okage.LineCapButt), expected)
}

func TestHairlineVline05Aa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/vline-05-aa.png"))
	assertPixmapEqual(t, drawLine(10, 10, 10, 90, true, 0.5, okage.LineCapButt), expected)
}

func TestHairlineVline05AaRound(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/vline-05-aa-round.png"))
	assertPixmapEqual(t, drawLine(10, 10, 10, 90, true, 0.5, okage.LineCapRound), expected)
}

func TestHairlineHorish05Aa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/horish-05-aa.png"))
	assertPixmapEqual(t, drawLine(10, 10, 90, 70, true, 0.5, okage.LineCapButt), expected)
}

func TestHairlineVertish05Aa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/vertish-05-aa.png"))
	assertPixmapEqual(t, drawLine(10, 10, 70, 90, true, 0.5, okage.LineCapButt), expected)
}

func TestHairlineClipLine05Aa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/clip-line-05-aa.png"))
	assertPixmapEqual(t, drawLine(-10, 10, 110, 70, true, 0.5, okage.LineCapButt), expected)
}

func TestHairlineClipLine00(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/clip-line-00.png"))
	assertPixmapEqual(t, drawLine(-10, 10, 110, 70, false, 0, okage.LineCapButt), expected)
}

func TestHairlineClipLine00V2(t *testing.T) {
	pixmap := must(okage.NewPixmap(512, 512))

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	stroke := okage.DefaultStroke()
	stroke.Width = 0

	builder := okage.NewPathBuilder()
	builder.MoveTo(369.26462, 577.8069)
	builder.LineTo(488.0846, 471.04388)
	path := must(builder.Finish())
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/hairline/clip-line-00-v2.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestHairlineClipHlineTopAa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/clip-hline-top-aa.png"))
	assertPixmapEqual(t, drawLine(-1, 0, 101, 0, true, 1, okage.LineCapButt), expected)
}

func TestHairlineClipHlineBottomAa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/clip-hline-bottom-aa.png"))
	assertPixmapEqual(t, drawLine(-1, 100, 101, 100, true, 1, okage.LineCapButt), expected)
}

func TestHairlineClipVlineLeftAa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/clip-vline-left-aa.png"))
	assertPixmapEqual(t, drawLine(0, -1, 0, 101, true, 1, okage.LineCapButt), expected)
}

func TestHairlineClipVlineRightAa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/clip-vline-right-aa.png"))
	assertPixmapEqual(t, drawLine(100, -1, 100, 101, true, 1, okage.LineCapButt), expected)
}

func drawQuad(antiAlias bool, width float32, lineCap okage.LineCap) *okage.Pixmap {
	pixmap := must(okage.NewPixmap(200, 100))

	pb := okage.NewPathBuilder()
	pb.MoveTo(25, 80)
	pb.QuadTo(155, 75, 175, 20)
	path := must(pb.Finish())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = antiAlias

	stroke := okage.DefaultStroke()
	stroke.Width = width
	stroke.LineCap = lineCap
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)

	return pixmap
}

func TestHairlineQuadWidth05Aa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/quad-width-05-aa.png"))
	assertPixmapEqual(t, drawQuad(true, 0.5, okage.LineCapButt), expected)
}

func TestHairlineQuadWidth05AaRound(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/quad-width-05-aa-round.png"))
	assertPixmapEqual(t, drawQuad(true, 0.5, okage.LineCapRound), expected)
}

func TestHairlineQuadWidth00(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/quad-width-00.png"))
	assertPixmapEqual(t, drawQuad(false, 0, okage.LineCapButt), expected)
}

func drawCubic(points *[8]float32, antiAlias bool, width float32, lineCap okage.LineCap) *okage.Pixmap {
	pixmap := must(okage.NewPixmap(200, 100))

	pb := okage.NewPathBuilder()
	pb.MoveTo(points[0], points[1])
	pb.CubicTo(points[2], points[3], points[4], points[5], points[6], points[7])
	path := must(pb.Finish())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = antiAlias

	stroke := okage.DefaultStroke()
	stroke.Width = width
	stroke.LineCap = lineCap
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)

	return pixmap
}

func TestHairlineCubicWidth10Aa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/cubic-width-10-aa.png"))
	assertPixmapEqual(t, drawCubic(&[8]float32{25, 80, 55, 25, 155, 75, 175, 20}, true, 1, okage.LineCapButt), expected)
}

func TestHairlineCubicWidth05Aa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/cubic-width-05-aa.png"))
	assertPixmapEqual(t, drawCubic(&[8]float32{25, 80, 55, 25, 155, 75, 175, 20}, true, 0.5, okage.LineCapButt), expected)
}

func TestHairlineCubicWidth00Aa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/cubic-width-00-aa.png"))
	assertPixmapEqual(t, drawCubic(&[8]float32{25, 80, 55, 25, 155, 75, 175, 20}, true, 0, okage.LineCapButt), expected)
}

func TestHairlineCubicWidth00(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/cubic-width-00.png"))
	assertPixmapEqual(t, drawCubic(&[8]float32{25, 80, 55, 25, 155, 75, 175, 20}, false, 0, okage.LineCapButt), expected)
}

func TestHairlineCubicWidth05AaRound(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/cubic-width-05-aa-round.png"))
	assertPixmapEqual(t, drawCubic(&[8]float32{25, 80, 55, 25, 155, 75, 175, 20}, true, 0.5, okage.LineCapRound), expected)
}

func TestHairlineCubicWidth00Round(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/cubic-width-00-round.png"))
	assertPixmapEqual(t, drawCubic(&[8]float32{25, 80, 55, 25, 155, 75, 175, 20}, false, 0, okage.LineCapRound), expected)
}

func TestHairlineChopCubic01(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/chop-cubic-01.png"))
	// This curve will invoke `path_geometry::chop_cubic_at_max_curvature` branch of `hair_cubic`.
	assertPixmapEqual(t, drawCubic(&[8]float32{57, 13, 17, 15, 55, 97, 89, 62}, true, 0.5, okage.LineCapButt), expected)
}

func TestHairlineClipCubic05Aa(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/clip-cubic-05-aa.png"))
	assertPixmapEqual(t, drawCubic(&[8]float32{-25, 80, 55, 25, 155, 75, 175, 20}, true, 0.5, okage.LineCapButt), expected)
}

func TestHairlineClipCubic00(t *testing.T) {
	expected := check(okage.LoadPNG("testdata/images/hairline/clip-cubic-00.png"))
	assertPixmapEqual(t, drawCubic(&[8]float32{-25, 80, 55, 25, 155, 75, 175, 20}, false, 0, okage.LineCapButt), expected)
}

func TestHairlineClippedCircleAa(t *testing.T) {
	pixmap := must(okage.NewPixmap(100, 100))

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	stroke := okage.DefaultStroke()
	stroke.Width = 0.5

	path := must(okage.PathFromCircle(50, 50, 55))
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/hairline/clipped-circle-aa.png"))
	assertPixmapEqual(t, pixmap, expected)
}
