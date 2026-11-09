// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports tiny-skia tests/integration/stroke.rs.

import (
	"testing"

	"github.com/xo/okage"
)

func TestStrokeRoundCapsAndLargeScale(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	path := func() *okage.Path {
		pb := okage.NewPathBuilder()
		pb.MoveTo(60.0/16.0, 100.0/16.0)
		pb.LineTo(140.0/16.0, 100.0/16.0)
		return must(pb.Finish())
	}()

	stroke := okage.DefaultStroke()
	stroke.Width = 6
	stroke.LineCap = okage.LineCapRound

	transform := okage.TransformFromScale(16, 16)

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.StrokePath(path, &paint, &stroke, transform, nil)

	expected := check(okage.LoadPNG("testdata/images/stroke/round-caps-and-large-scale.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestStrokeCircle(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	path := must(okage.PathFromCircle(100, 100, 50))
	stroke := okage.DefaultStroke()
	stroke.Width = 2

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.StrokePath(path, &paint, &stroke, okage.DefaultTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/stroke/circle.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestStrokeZeroLenSubpathButtCap(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pb := okage.NewPathBuilder()
	pb.MoveTo(50, 50)
	pb.LineTo(50, 50)
	path := must(pb.Finish())

	stroke := okage.DefaultStroke()
	stroke.Width = 20
	stroke.LineCap = okage.LineCapButt

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.StrokePath(path, &paint, &stroke, okage.DefaultTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/stroke/zero-len-subpath-butt-cap.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestStrokeZeroLenSubpathRoundCap(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pb := okage.NewPathBuilder()
	pb.MoveTo(50, 50)
	pb.LineTo(50, 50)
	path := must(pb.Finish())

	stroke := okage.DefaultStroke()
	stroke.Width = 20
	stroke.LineCap = okage.LineCapRound

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.StrokePath(path, &paint, &stroke, okage.DefaultTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/stroke/zero-len-subpath-round-cap.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestStrokeZeroLenSubpathSquareCap(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pb := okage.NewPathBuilder()
	pb.MoveTo(50, 50)
	pb.LineTo(50, 50)
	path := must(pb.Finish())

	stroke := okage.DefaultStroke()
	stroke.Width = 20
	stroke.LineCap = okage.LineCapSquare

	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.StrokePath(path, &paint, &stroke, okage.DefaultTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/stroke/zero-len-subpath-square-cap.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestStrokeRoundCapJoin(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pb := okage.NewPathBuilder()
	pb.MoveTo(170, 30)
	pb.LineTo(30.553378, 99.048418)
	pb.CubicTo(30.563658, 99.066835, 30.546308, 99.280724, 30.557592, 99.305282)
	path := must(pb.Finish())

	stroke := okage.DefaultStroke()
	stroke.Width = 30
	stroke.LineCap = okage.LineCapRound
	stroke.LineJoin = okage.LineJoinRound

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.StrokePath(path, &paint, &stroke, okage.DefaultTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/stroke/round-cap-join.png"))
	assertPixmapEqual(t, pixmap, expected)
}
