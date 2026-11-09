// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports tiny-skia tests/integration/pixmap.rs.

import (
	"testing"

	"github.com/xo/okage"
)

func TestPixmapCloneRect1(t *testing.T) {
	pixmap := must(okage.NewPixmap(200, 200))

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pixmap.FillPath(
		must(okage.PathFromCircle(100, 100, 80)),
		&paint,
		okage.FillRuleWinding,
		okage.IdentityTransform(),
		nil,
	)

	part := must(pixmap.AsRef().CloneRect(must(okage.IntRectFromXYWH(10, 15, 80, 90))))

	expected := check(okage.LoadPNG("testdata/images/pixmap/clone-rect-1.png"))
	assertPixmapEqual(t, part, expected)
}

func TestPixmapCloneRect2(t *testing.T) {
	pixmap := must(okage.NewPixmap(200, 200))

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pixmap.FillPath(
		must(okage.PathFromCircle(100, 100, 80)),
		&paint,
		okage.FillRuleWinding,
		okage.IdentityTransform(),
		nil,
	)

	part := must(pixmap.AsRef().CloneRect(must(okage.IntRectFromXYWH(130, 120, 80, 90))))

	expected := check(okage.LoadPNG("testdata/images/pixmap/clone-rect-2.png"))
	assertPixmapEqual(t, part, expected)
}

func TestPixmapCloneRectOutOfBound(t *testing.T) {
	pixmap := must(okage.NewPixmap(200, 200))

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pixmap.FillPath(
		must(okage.PathFromCircle(100, 100, 80)),
		&paint,
		okage.FillRuleWinding,
		okage.IdentityTransform(),
		nil,
	)

	if _, ok := pixmap.AsRef().CloneRect(must(okage.IntRectFromXYWH(250, 15, 80, 90))); ok {
		t.Error("CloneRect returned Some, want None")
	}
	if _, ok := pixmap.AsRef().CloneRect(must(okage.IntRectFromXYWH(10, 250, 80, 90))); ok {
		t.Error("CloneRect returned Some, want None")
	}
	if _, ok := pixmap.AsRef().CloneRect(must(okage.IntRectFromXYWH(10, -250, 80, 90))); ok {
		t.Error("CloneRect returned Some, want None")
	}
}

func TestPixmapFill(t *testing.T) {
	c := okage.ColorFromRGBA8(50, 100, 150, 200)
	pixmap := must(okage.NewPixmap(10, 10))
	pixmap.Fill(c)
	if got, want := must(pixmap.Pixel(1, 1)), c.Premultiply().ToColorU8(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPixmapDrawPixmap(t *testing.T) {
	// Tests that painting algorithm will switch `Bicubic`/`Bilinear` to `Nearest`.
	// Otherwise we will get a blurry image.

	// A pixmap with the bottom half filled with solid color.
	subPixmap := func() *okage.Pixmap {
		paint := okage.DefaultPaint()
		paint.SetColorRGBA8(50, 127, 150, 200)
		paint.AntiAlias = false

		rect := must(okage.RectFromXYWH(0, 50, 100, 50))

		pixmap := must(okage.NewPixmap(100, 100))
		pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)
		return pixmap
	}()

	paint := okage.DefaultPixmapPaint()
	paint.Quality = okage.FilterQualityBicubic

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.DrawPixmap(20, 20, subPixmap.AsRef(), &paint, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/canvas/draw-pixmap.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestPixmapDrawPixmapTs(t *testing.T) {
	triangle := func() *okage.Pixmap {
		paint := okage.DefaultPaint()
		paint.SetColorRGBA8(50, 127, 150, 200)
		paint.AntiAlias = true

		pb := okage.NewPathBuilder()
		pb.MoveTo(0, 100)
		pb.LineTo(100, 100)
		pb.LineTo(50, 0)
		pb.Close()
		path := must(pb.Finish())

		pixmap := must(okage.NewPixmap(100, 100))
		pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
		return pixmap
	}()

	paint := okage.DefaultPixmapPaint()
	paint.Quality = okage.FilterQualityBicubic

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.DrawPixmap(
		5, 10,
		triangle.AsRef(),
		&paint,
		okage.TransformFromRow(1.2, 0.5, 0.5, 1.2, 0, 0),
		nil,
	)

	expected := check(okage.LoadPNG("testdata/images/canvas/draw-pixmap-ts.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestPixmapDrawPixmapOpacity(t *testing.T) {
	triangle := func() *okage.Pixmap {
		paint := okage.DefaultPaint()
		paint.SetColorRGBA8(50, 127, 150, 200)
		paint.AntiAlias = true

		pb := okage.NewPathBuilder()
		pb.MoveTo(0, 100)
		pb.LineTo(100, 100)
		pb.LineTo(50, 0)
		pb.Close()
		path := must(pb.Finish())

		pixmap := must(okage.NewPixmap(100, 100))
		pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
		return pixmap
	}()

	paint := okage.DefaultPixmapPaint()
	paint.Quality = okage.FilterQualityBicubic
	paint.Opacity = 0.5

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.DrawPixmap(
		5, 10,
		triangle.AsRef(),
		&paint,
		okage.TransformFromRow(1.2, 0.5, 0.5, 1.2, 0, 0),
		nil,
	)

	expected := check(okage.LoadPNG("testdata/images/canvas/draw-pixmap-opacity.png"))
	assertPixmapEqual(t, pixmap, expected)
}
