// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports tiny-skia tests/integration/pattern.rs.

import (
	"testing"

	"github.com/xo/okage"
)

func crateTriangle() *okage.Pixmap {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	pb := okage.NewPathBuilder()
	pb.MoveTo(0, 20)
	pb.LineTo(20, 20)
	pb.LineTo(10, 0)
	pb.Close()
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(20, 20))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
	return pixmap
}

func TestPatternPadNearest(t *testing.T) {
	triangle := crateTriangle()

	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = okage.NewPattern(
		triangle.AsRef(),
		okage.SpreadModePad,
		okage.FilterQualityNearest,
		1,
		okage.IdentityTransform(),
	)

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/pattern/pad-nearest.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestPatternRepeatNearest(t *testing.T) {
	triangle := crateTriangle()

	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = okage.NewPattern(
		triangle.AsRef(),
		okage.SpreadModeRepeat,
		okage.FilterQualityNearest,
		1,
		okage.IdentityTransform(),
	)

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/pattern/repeat-nearest.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestPatternReflectNearest(t *testing.T) {
	triangle := crateTriangle()

	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = okage.NewPattern(
		triangle.AsRef(),
		okage.SpreadModeReflect,
		okage.FilterQualityNearest,
		1,
		okage.IdentityTransform(),
	)

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/pattern/reflect-nearest.png"))
	assertPixmapEqual(t, pixmap, expected)
}

// We have to test tile mode for bilinear/bicubic separately,
// because they're using a different algorithm from nearest.
func TestPatternPadBicubic(t *testing.T) {
	triangle := crateTriangle()

	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = okage.NewPattern(
		triangle.AsRef(),
		okage.SpreadModePad,
		okage.FilterQualityBicubic,
		1,
		// Transform must be set, otherwise we will fallback to Nearest.
		okage.TransformFromRow(1.1, 0.3, 0, 1.4, 0, 0),
	)

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/pattern/pad-bicubic.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestPatternRepeatBicubic(t *testing.T) {
	triangle := crateTriangle()

	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = okage.NewPattern(
		triangle.AsRef(),
		okage.SpreadModeRepeat,
		okage.FilterQualityBicubic,
		1,
		// Transform must be set, otherwise we will fallback to Nearest.
		okage.TransformFromRow(1.1, 0.3, 0, 1.4, 0, 0),
	)

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/pattern/repeat-bicubic.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestPatternReflectBicubic(t *testing.T) {
	triangle := crateTriangle()

	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = okage.NewPattern(
		triangle.AsRef(),
		okage.SpreadModeReflect,
		okage.FilterQualityBicubic,
		1,
		// Transform must be set, otherwise we will fallback to Nearest.
		okage.TransformFromRow(1.1, 0.3, 0, 1.4, 0, 0),
	)

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/pattern/reflect-bicubic.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestPatternFilterNearestNoTs(t *testing.T) {
	triangle := crateTriangle()

	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = okage.NewPattern(
		triangle.AsRef(),
		okage.SpreadModeRepeat,
		okage.FilterQualityNearest,
		1,
		okage.IdentityTransform(),
	)

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/pattern/filter-nearest-no-ts.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestPatternFilterNearest(t *testing.T) {
	triangle := crateTriangle()

	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = okage.NewPattern(
		triangle.AsRef(),
		okage.SpreadModeRepeat,
		okage.FilterQualityNearest,
		1,
		okage.TransformFromRow(1.5, 0, -0.4, -0.8, 5, 1),
	)

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/pattern/filter-nearest.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestPatternFilterBilinear(t *testing.T) {
	triangle := crateTriangle()

	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = okage.NewPattern(
		triangle.AsRef(),
		okage.SpreadModeRepeat,
		okage.FilterQualityBilinear,
		1,
		okage.TransformFromRow(1.5, 0, -0.4, -0.8, 5, 1),
	)

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/pattern/filter-bilinear.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestPatternFilterBicubic(t *testing.T) {
	triangle := crateTriangle()

	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = okage.NewPattern(
		triangle.AsRef(),
		okage.SpreadModeRepeat,
		okage.FilterQualityBicubic,
		1,
		okage.TransformFromRow(1.5, 0, -0.4, -0.8, 5, 1),
	)

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/pattern/filter-bicubic.png"))
	assertPixmapEqual(t, pixmap, expected)
}
