// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports tiny-skia tests/integration/dash.rs.

import (
	"testing"

	"github.com/xo/okage"
)

func TestDashLine(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.LineTo(90, 80)
	path := must(pb.Finish())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	stroke := okage.DefaultStroke()
	stroke.Dash, _ = okage.NewStrokeDash([]float32{5, 10}, 0)
	stroke.Width = 2

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/dash/line.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestDashQuad(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.QuadTo(35, 75, 90, 80)
	path := must(pb.Finish())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	stroke := okage.DefaultStroke()
	stroke.Dash, _ = okage.NewStrokeDash([]float32{5, 10}, 0)
	stroke.Width = 2

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/dash/quad.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestDashCubic(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.CubicTo(95, 35, 0, 75, 75, 90)
	path := must(pb.Finish())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	stroke := okage.DefaultStroke()
	stroke.Dash, _ = okage.NewStrokeDash([]float32{5, 10}, 0)
	stroke.Width = 2

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/dash/cubic.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestDashHairline(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.CubicTo(95, 35, 0, 75, 75, 90)
	path := must(pb.Finish())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	stroke := okage.DefaultStroke()
	stroke.Dash, _ = okage.NewStrokeDash([]float32{5, 10}, 0)
	stroke.Width = 0.5

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/dash/hairline.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestDashComplex(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(28.7, 23.9)
	pb.LineTo(177.4, 35.2)
	pb.LineTo(177.4, 68)
	pb.LineTo(129.7, 68)
	pb.CubicTo(81.6, 59.3, 41.8, 63.3, 33.4, 115.2)
	pb.CubicTo(56.8, 128.7, 77.3, 143.8, 53.3, 183.8)
	pb.CubicTo(113.8, 185.7, 91, 109.7, 167.3, 111.8)
	pb.CubicTo(-56.2, 90.3, 177.3, 68, 110.2, 95.5)
	path := must(pb.Finish())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	stroke := okage.DefaultStroke()
	stroke.Dash, _ = okage.NewStrokeDash([]float32{10, 5}, 2)
	stroke.Width = 2

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/dash/complex.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestDashMultiSubpaths(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(49, 76)
	pb.CubicTo(22, 150, 11, 213, 186, 151)
	pb.CubicTo(194, 106, 195, 64, 169, 26)
	pb.MoveTo(124, 41)
	pb.LineTo(162, 105)
	pb.CubicTo(135, 175, 97, 166, 53, 128)
	pb.LineTo(93, 71)
	pb.MoveTo(24, 52)
	pb.LineTo(108, 20)
	path := must(pb.Finish())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	stroke := okage.DefaultStroke()
	stroke.Dash, _ = okage.NewStrokeDash([]float32{10, 5}, 2)
	stroke.Width = 2

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/dash/multi_subpaths.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestDashClosed(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(22, 22)
	pb.CubicTo(63, 16, 82, 24, 84, 46)
	pb.CubicTo(86, 73, 15, 58, 16, 89)
	pb.Close()
	path := must(pb.Finish())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	stroke := okage.DefaultStroke()
	stroke.Dash, _ = okage.NewStrokeDash([]float32{10, 5}, 2)
	stroke.Width = 2

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/dash/closed.png"))
	assertPixmapEqual(t, pixmap, expected)
}
