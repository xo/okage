// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports the examples of tiny-skia, examples/*.rs. Each upstream
// example is one Example function, in the order of the names of the files.
//
// Each upstream example saves its image to image.png. These examples do not
// write a file, so that a test run does not change the repository. Call
// Pixmap.SavePNG to save the image. In place of the file, an example prints
// one pixel, which go test compares with its Output comment.

import (
	"fmt"
	"math"

	"github.com/xo/okage"
)

// printPixel prints the premultiplied RGBA value of the pixel at x, y.
func printPixel(pixmap *okage.Pixmap, x, y uint32) {
	c, _ := pixmap.Pixel(x, y)
	fmt.Println(c.Red(), c.Green(), c.Blue(), c.Alpha())
}

// This example ports tiny-skia examples/fill.rs. It fills two paths, one
// with anti-aliasing and one without.
func Example_fill() {
	paint1 := okage.DefaultPaint()
	paint1.SetColorRGBA8(50, 127, 150, 200)
	paint1.AntiAlias = true

	paint2 := okage.DefaultPaint()
	paint2.SetColorRGBA8(220, 140, 75, 180)
	paint2.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(60, 60)
	pb.LineTo(160, 940)
	pb.CubicTo(380, 840, 660, 800, 940, 800)
	pb.CubicTo(740, 460, 440, 160, 60, 60)
	pb.Close()
	path1, _ := pb.Finish()

	pb = okage.NewPathBuilder()
	pb.MoveTo(940, 60)
	pb.LineTo(840, 940)
	pb.CubicTo(620, 840, 340, 800, 60, 800)
	pb.CubicTo(260, 460, 560, 160, 940, 60)
	pb.Close()
	path2, _ := pb.Finish()

	pixmap, _ := okage.NewPixmap(1000, 1000)
	pixmap.FillPath(
		path1,
		&paint1,
		okage.FillRuleWinding,
		okage.IdentityTransform(),
		nil,
	)
	pixmap.FillPath(
		path2,
		&paint2,
		okage.FillRuleWinding,
		okage.IdentityTransform(),
		nil,
	)
	printPixel(pixmap, 500, 600)
	// Output:
	// 167 129 88 239
}

// This example ports tiny-skia examples/gamma.rs. It strokes the same lines
// in each color space.
func Example_gamma() {
	paint := okage.DefaultPaint()
	paint.Shader = okage.SolidColor(okage.ColorFromRGBA8(255, 100, 20, 255))
	paint.AntiAlias = true
	stroke := okage.DefaultStroke()

	pixmap, _ := okage.NewPixmap(1000, 1000)
	pixmap.Fill(okage.ColorBlack)

	pb := okage.NewPathBuilder()
	for i := range 10 {
		pb.MoveTo(50, 45+float32(i)*20)
		pb.LineTo(450, 45+float32(i)*21)
	}
	path, _ := pb.Finish()

	colors := []okage.ColorSpace{
		okage.ColorSpaceLinear,
		okage.ColorSpaceGamma2,
		okage.ColorSpaceSimpleSRGB,
		okage.ColorSpaceFullSRGBGamma,
	}

	for i, color := range colors {
		paint.Colorspace = color

		xf := okage.IdentityTransform()
		xf = xf.PreTranslate(0, 240*float32(i))

		pixmap.StrokePath(path, &paint, &stroke, xf, nil)

		// Move down 0.5 pixel so lines start in the middle of the pixel, not the edge
		xf = xf.PreTranslate(500, 0.5)

		pixmap.StrokePath(path, &paint, &stroke, xf, nil)
	}

	printPixel(pixmap, 250, 285)
	// Output:
	// 181 71 14 255
}

// This example ports tiny-skia examples/hairline.rs.
//
// This example demonstrates thin paths rendering.
func Example_hairline() {
	pb := okage.NewPathBuilder()
	pb.MoveTo(50, 100)
	pb.CubicTo(130, 20, 390, 120, 450, 30)
	path, _ := pb.Finish()

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pixmap, _ := okage.NewPixmap(500, 500)
	transform := okage.IdentityTransform()
	for i := range 20 {
		stroke := okage.DefaultStroke()
		stroke.Width = 2 - (float32(i) / 10)
		pixmap.StrokePath(path, &paint, &stroke, transform, nil)
		transform = transform.PreTranslate(0, 20)
	}

	printPixel(pixmap, 50, 100)
	// Output:
	// 13 32 37 63
}

// This example ports tiny-skia examples/image_on_image.rs. It draws a pixmap
// on another pixmap with a transform.
//
// Upstream also prints the time that the drawing takes. This example does
// not, because the time is different on each run.
func Example_imageOnImage() {
	triangle := createTriangle()

	pixmap, _ := okage.NewPixmap(400, 400)

	paint := okage.DefaultPixmapPaint()
	paint.Quality = okage.FilterQualityBicubic

	pixmap.DrawPixmap(
		20,
		20,
		triangle.AsRef(),
		&paint,
		okage.TransformFromRow(1.2, 0.5, 0.5, 1.2, 0, 0),
		nil,
	)

	printPixel(pixmap, 200, 200)
	// Output:
	// 39 100 118 200
}

// createTriangle is create_triangle in tiny-skia examples/image_on_image.rs.
func createTriangle() *okage.Pixmap {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pb := okage.NewPathBuilder()
	pb.MoveTo(0, 200)
	pb.LineTo(200, 200)
	pb.LineTo(100, 0)
	pb.Close()
	path, _ := pb.Finish()

	pixmap, _ := okage.NewPixmap(200, 200)

	pixmap.FillPath(
		path,
		&paint,
		okage.FillRuleWinding,
		okage.IdentityTransform(),
		nil,
	)

	rect, _ := okage.RectFromLTRB(0, 0, 200, 200)
	path = okage.PathFromRect(rect)
	stroke := okage.DefaultStroke()
	paint.SetColorRGBA8(200, 0, 0, 220)

	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil) // TODO: stroke_rect

	return pixmap
}

// This example ports tiny-skia examples/large_image.rs.
//
// This example will crate a 20_000x20_000px image, which can take a while in a debug mode.
// This example is used mainly to tests that our tiling algorithm actually works and doesn't panic.
//
// The pixmap and the mask need about 2 GB of memory, so this example has no
// Output comment, and go test compiles it but does not run it.
func Example_largeImage() { //nolint:testableexamples // It needs about 2 GB of memory, so go test must not run it.
	pb := okage.NewPathBuilder()
	pb.MoveTo(1200, 1200)
	pb.LineTo(3200, 18800)
	pb.CubicTo(7600, 16800, 13200, 16000, 18800, 16000)
	pb.CubicTo(14800, 9200, 8800, 3200, 1200, 1200)
	pb.Close()
	path1, _ := pb.Finish()

	pb = okage.NewPathBuilder()
	pb.MoveTo(18800, 1200)
	pb.LineTo(16800, 18800)
	pb.CubicTo(12400, 16800, 6800, 16000, 1200, 16000)
	pb.CubicTo(5200, 9200, 11200, 3200, 18800, 1200)
	pb.Close()
	path2, _ := pb.Finish()

	pixmap, _ := okage.NewPixmap(20000, 20000)

	pb = okage.NewPathBuilder()
	pb.PushCircle(10000, 10000, 7000)
	clipPath, _ := pb.Finish()

	mask, _ := okage.NewMask(20000, 20000)
	mask.FillPath(clipPath, okage.FillRuleWinding, true, okage.DefaultTransform())

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(90, 175, 100, 150)
	paint.AntiAlias = true
	largeRect, _ := okage.RectFromXYWH(500, 500, 19000, 19000)
	pixmap.FillRect(largeRect, &paint, okage.IdentityTransform(), nil)

	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true
	pixmap.FillPath(
		path1,
		&paint,
		okage.FillRuleWinding,
		okage.DefaultTransform(),
		mask,
	)

	paint.SetColorRGBA8(220, 140, 75, 180)
	paint.AntiAlias = false
	pixmap.FillPath(
		path2,
		&paint,
		okage.FillRuleWinding,
		okage.DefaultTransform(),
		nil,
	)

	paint.SetColorRGBA8(255, 10, 15, 180)
	paint.AntiAlias = true
	stroke := okage.DefaultStroke()
	stroke.Width = 0.8 // hairline
	pixmap.StrokePath(path2, &paint, &stroke, okage.DefaultTransform(), nil)
}

// This example ports tiny-skia examples/linear_gradient.rs. It fills a path
// with a linear gradient.
func Example_linearGradient() {
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader, _ = okage.NewLinearGradient(
		okage.PointFromXY(100, 100),
		okage.PointFromXY(900, 900),
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	)

	pb := okage.NewPathBuilder()
	pb.MoveTo(60, 60)
	pb.LineTo(160, 940)
	pb.CubicTo(380, 840, 660, 800, 940, 800)
	pb.CubicTo(740, 460, 440, 160, 60, 60)
	pb.Close()
	path, _ := pb.Finish()

	pixmap, _ := okage.NewPixmap(1000, 1000)
	pixmap.FillPath(
		path,
		&paint,
		okage.FillRuleWinding,
		okage.IdentityTransform(),
		nil,
	)
	printPixel(pixmap, 400, 600)
	// Output:
	// 101 100 84 190
}

// This example ports tiny-skia examples/mask.rs. It fills a rectangle through
// a mask of two circles.
func Example_mask() {
	pb := okage.NewPathBuilder()
	pb.PushCircle(250, 250, 200)
	pb.PushCircle(250, 250, 100)
	clipPath, _ := pb.Finish()

	clipPath, _ = clipPath.Transform(okage.TransformFromRow(1, -0.3, 0, 1, 0, 75))

	mask, _ := okage.NewMask(500, 500)
	mask.FillPath(clipPath, okage.FillRuleEvenOdd, true, okage.DefaultTransform())

	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.SetColorRGBA8(50, 127, 150, 200)

	pixmap, _ := okage.NewPixmap(500, 500)
	rect, _ := okage.RectFromXYWH(0, 0, 500, 500)
	pixmap.FillRect(
		rect,
		&paint,
		okage.IdentityTransform(),
		mask,
	)
	printPixel(pixmap, 250, 100)
	// Output:
	// 39 100 118 200
}

// This example ports tiny-skia examples/pattern.rs. It fills a circle with a
// pattern of a small pixmap. The function create_triangle of the example is
// the same as crate_triangle of tests/integration/pattern.rs, so the example
// calls crateTriangle of pattern_integration_test.go.
func Example_pattern() {
	triangle := crateTriangle()

	paint := okage.DefaultPaint()
	paint.AntiAlias = true
	paint.Shader = okage.NewPattern(
		triangle.AsRef(),
		okage.SpreadModeRepeat,
		okage.FilterQualityBicubic,
		1,
		okage.TransformFromRow(1.5, -0.4, 0, -0.8, 5, 1),
	)

	path, _ := okage.PathFromCircle(200, 200, 180)

	pixmap, _ := okage.NewPixmap(400, 400)
	pixmap.FillPath(
		path,
		&paint,
		okage.FillRuleWinding,
		okage.IdentityTransform(),
		nil,
	)
	printPixel(pixmap, 200, 200)
	// Output:
	// 39 100 118 200
}

// This example ports tiny-skia examples/stroke.rs. It strokes a star with a
// dash and round caps.
//
// Based on https://fiddle.skia.org/c/@compose_path
func Example_stroke() {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(0, 127, 0, 200)
	paint.AntiAlias = true

	const radius float32 = 250
	const center float32 = 250
	pb := okage.NewPathBuilder()
	pb.MoveTo(center+radius, center)
	for i := 1; i < 8; i++ {
		a := 2.6927937 * float32(i)
		// Rust calls cosf and sinf of the C library. The functions of the
		// package math can give another last bit for a rare input.
		cos := float32(math.Cos(float64(a)))
		sin := float32(math.Sin(float64(a)))
		pb.LineTo(center+float32(radius*cos), center+float32(radius*sin))
	}
	path, _ := pb.Finish()

	stroke := okage.DefaultStroke()
	stroke.Width = 6
	stroke.LineCap = okage.LineCapRound
	stroke.Dash, _ = okage.NewStrokeDash([]float32{20, 40}, 0)

	pixmap, _ := okage.NewPixmap(500, 500)
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)
	printPixel(pixmap, 497, 250)
	// Output:
	// 0 100 0 200
}
