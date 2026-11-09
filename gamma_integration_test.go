// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports tiny-skia tests/integration/gamma.rs.

import (
	"testing"

	"github.com/xo/okage"
)

func TestGammaGamma(t *testing.T) {
	paint := okage.DefaultPaint()
	stroke := okage.DefaultStroke()
	wide := okage.DefaultStroke()
	wide.Width = 3

	solid := okage.SolidColor(okage.ColorFromRGBA8(255, 100, 20, 255))
	grad2 := must(okage.NewLinearGradient(
		okage.Point{X: 50, Y: 2},
		okage.Point{X: 450, Y: 2},
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(255, 0, 0, 255)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(0, 255, 0, 255)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	grad3 := must(okage.NewLinearGradient(
		okage.Point{X: 50, Y: 2},
		okage.Point{X: 450, Y: 2},
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(255, 0, 0, 255)),
			okage.NewGradientStop(0.5, okage.ColorFromRGBA8(0, 0, 255, 255)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(128, 128, 128, 128)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	pixmap := must(okage.NewPixmap(500, 60))
	pixmap.Fill(okage.ColorBlack)

	pb := okage.NewPathBuilder()
	pb.MoveTo(20, 2)
	pb.LineTo(480, 3)
	path := must(pb.Finish())

	colors := []okage.ColorSpace{
		okage.ColorSpaceLinear,
		okage.ColorSpaceGamma2,
		okage.ColorSpaceSimpleSRGB,
		okage.ColorSpaceFullSRGBGamma,
	}

	for i, color := range colors {
		xf := okage.TransformFromTranslate(0, 4*float32(i))

		paint.Colorspace = color
		paint.Shader = solid
		pixmap.StrokePath(path, &paint, &stroke, xf, nil)

		xf = okage.TransformFromTranslate(0, 20+10*float32(i))
		paint.Shader = grad2
		pixmap.StrokePath(path, &paint, &wide, xf, nil)

		xf = okage.TransformFromTranslate(0, 22.5+10*float32(i))
		paint.Shader = grad3
		pixmap.StrokePath(path, &paint, &wide, xf, nil)
	}

	// pixmap.SavePNG("testdata/images/gamma.png")
	expected := check(okage.LoadPNG("testdata/images/gamma.png"))
	assertPixmapEqual(t, pixmap, expected)
}
