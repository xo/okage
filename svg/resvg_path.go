// Copyright 2019 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"github.com/xo/okage"
	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// This file ports resvg src/path.rs.
//
// Port: the function render of upstream collides with the function render of
// other modules, so it takes the name of its module, renderPath (D10). A
// function that returns Option<()> returns a bool, and false is None.

// renderPath is render in resvg src/path.rs.
func renderPath(
	p *Path,
	blendMode okage.BlendMode,
	ctx *context,
	transform Transform,
	pixmap *okage.PixmapMut,
) {
	if !p.IsVisible() {
		return
	}

	if p.PaintOrder() == PaintOrderFillAndStroke {
		fillPath(p, blendMode, ctx, transform, pixmap)
		strokePath(p, blendMode, ctx, transform, pixmap)
	} else {
		strokePath(p, blendMode, ctx, transform, pixmap)
		fillPath(p, blendMode, ctx, transform, pixmap)
	}
}

// fillPath is fill_path in resvg src/path.rs.
func fillPath(
	p *Path,
	blendMode okage.BlendMode,
	ctx *context,
	transform Transform,
	pixmap *okage.PixmapMut,
) bool { //nolint:unparam // upstream returns Option<()>
	fill, ok := p.Fill()
	if !ok {
		return false
	}

	// Horizontal and vertical lines cannot be filled. Skip.
	if p.Data().Bounds().Width() == 0.0 || p.Data().Bounds().Height() == 0.0 {
		return false
	}

	var rule okage.FillRule
	switch fill.Rule() {
	case FillRuleNonZero:
		rule = okage.FillRuleWinding
	case FillRuleEvenOdd:
		rule = okage.FillRuleEvenOdd
	}

	paint := okage.DefaultPaint()
	switch c := fill.Paint().(type) {
	case Color:
		paint.SetColorRGBA8(c.Red, c.Green, c.Blue, fill.Opacity().ToU8())
	case *LinearGradient:
		shader, ok := convertLinearGradient(c, fill.Opacity())
		if !ok {
			return false
		}
		paint.Shader = shader
	case *RadialGradient:
		shader, ok := convertRadialGradient(c, fill.Opacity())
		if !ok {
			return false
		}
		paint.Shader = shader
	case *Pattern:
		pattPix, pattTs, ok := renderPatternPixmap(c, ctx, transform)
		if !ok {
			return false
		}

		patternPixmap := pattPix
		paint.Shader = okage.NewPattern(
			patternPixmap.AsRef(),
			okage.SpreadModeRepeat,
			okage.FilterQualityBicubic,
			fill.Opacity().Get(),
			pattTs,
		)
	}
	paint.AntiAlias = p.RenderingMode().UseShapeAntialiasing()
	paint.BlendMode = blendMode

	pixmap.FillPath(p.Data(), &paint, rule, transform, nil)
	return true
}

// strokePath is stroke_path in resvg src/path.rs.
func strokePath(
	p *Path,
	blendMode okage.BlendMode,
	ctx *context,
	transform Transform,
	pixmap *okage.PixmapMut,
) bool { //nolint:unparam // upstream returns Option<()>
	stroke, ok := p.Stroke()
	if !ok {
		return false
	}
	paint := okage.DefaultPaint()
	switch c := stroke.Paint().(type) {
	case Color:
		paint.SetColorRGBA8(c.Red, c.Green, c.Blue, stroke.Opacity().ToU8())
	case *LinearGradient:
		shader, ok := convertLinearGradient(c, stroke.Opacity())
		if !ok {
			return false
		}
		paint.Shader = shader
	case *RadialGradient:
		shader, ok := convertRadialGradient(c, stroke.Opacity())
		if !ok {
			return false
		}
		paint.Shader = shader
	case *Pattern:
		pattPix, pattTs, ok := renderPatternPixmap(c, ctx, transform)
		if !ok {
			return false
		}

		patternPixmap := pattPix
		paint.Shader = okage.NewPattern(
			patternPixmap.AsRef(),
			okage.SpreadModeRepeat,
			okage.FilterQualityBicubic,
			stroke.Opacity().Get(),
			pattTs,
		)
	}
	paint.AntiAlias = p.RenderingMode().UseShapeAntialiasing()
	paint.BlendMode = blendMode

	ts := stroke.ToTinySkia()
	pixmap.StrokePath(p.Data(), &paint, &ts, transform, nil)

	return true
}

// convertLinearGradient is convert_linear_gradient in resvg src/path.rs.
func convertLinearGradient(gradient *LinearGradient, opacity Opacity) (okage.Shader, bool) {
	mode, points, ok := convertBaseGradient(&gradient.BaseGradient, opacity)
	if !ok {
		return nil, false
	}

	shader, ok := okage.NewLinearGradient(
		path.Point{X: gradient.X1(), Y: gradient.Y1()},
		path.Point{X: gradient.X2(), Y: gradient.Y2()},
		points,
		mode,
		gradient.Transform(),
	)
	if !ok {
		return nil, false
	}

	return shader, true
}

// convertRadialGradient is convert_radial_gradient in resvg src/path.rs.
func convertRadialGradient(gradient *RadialGradient, opacity Opacity) (okage.Shader, bool) {
	mode, points, ok := convertBaseGradient(&gradient.BaseGradient, opacity)
	if !ok {
		return nil, false
	}

	shader, ok := okage.NewRadialGradient(
		path.Point{X: gradient.Fx(), Y: gradient.Fy()},
		gradient.Fr().Get(),
		path.Point{X: gradient.Cx(), Y: gradient.Cy()},
		gradient.R().Get(),
		points,
		mode,
		gradient.Transform(),
	)
	if !ok {
		return nil, false
	}

	return shader, true
}

// convertBaseGradient is convert_base_gradient in resvg src/path.rs.
func convertBaseGradient(gradient *BaseGradient, opacity Opacity) (okage.SpreadMode, []okage.GradientStop, bool) {
	var mode okage.SpreadMode
	switch gradient.SpreadMethod() {
	case SpreadMethodPad:
		mode = okage.SpreadModePad
	case SpreadMethodReflect:
		mode = okage.SpreadModeReflect
	case SpreadMethodRepeat:
		mode = okage.SpreadModeRepeat
	}

	points := make([]okage.GradientStop, 0, len(gradient.Stops()))
	for _, stop := range gradient.Stops() {
		alpha := stop.Opacity().Mul(opacity)
		color := okage.ColorFromRGBA8(
			stop.Color().Red,
			stop.Color().Green,
			stop.Color().Blue,
			alpha.ToU8(),
		)
		points = append(points, okage.NewGradientStop(stop.Offset().Get(), color))
	}

	return mode, points, true
}

// renderPatternPixmap is render_pattern_pixmap in resvg src/path.rs.
func renderPatternPixmap(
	pattern *Pattern,
	ctx *context,
	transform Transform,
) (*okage.Pixmap, Transform, bool) {
	var sx, sy float32
	{
		ts2 := transform.PreConcat(pattern.Transform())
		sx, sy = ts2.GetScale()
	}

	rect := pattern.Rect()
	imgSize, ok := path.IntSizeFromWH(
		wide.TruncU32(round32(rect.Width()*sx)),
		wide.TruncU32(round32(rect.Height()*sy)),
	)
	if !ok {
		return nil, Transform{}, false
	}
	pixmap, ok := newPixmap(imgSize.Width(), imgSize.Height(), ctx.opt)
	if !ok {
		return nil, Transform{}, false
	}

	transform = path.TransformFromScale(sx, sy)
	m := pixmap.AsMut()
	renderNodes(pattern.Root(), ctx, transform, &m)

	ts := path.DefaultTransform()
	ts = ts.PreConcat(pattern.Transform())
	ts = ts.PreTranslate(rect.X(), rect.Y())
	ts = ts.PreScale(1.0/sx, 1.0/sy)

	return pixmap, ts, true
}
