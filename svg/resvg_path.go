// Copyright 2019 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"github.com/xo/okage"
	"github.com/xo/okage/internal/crmath"
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

	points := make([]gradientPoint, 0, len(gradient.Stops()))
	for _, stop := range gradient.Stops() {
		alpha := stop.Opacity().Mul(opacity)
		color := okage.ColorFromRGBA8(
			stop.Color().Red,
			stop.Color().Green,
			stop.Color().Blue,
			alpha.ToU8(),
		)
		points = append(points, gradientPoint{stop.Offset().Get(), color})
	}

	if gradient.ColorInterpolation() == ColorInterpolationFilterLinearRGB {
		points = linearRGBStops(points)
	}

	stops := make([]okage.GradientStop, 0, len(points))
	for _, p := range points {
		stops = append(stops, okage.NewGradientStop(p.offset, p.color))
	}

	return mode, stops, true
}

// gradientPoint is the tuple (f32, tiny_skia::Color) of a stop offset and a
// stop color in resvg src/path.rs.
type gradientPoint struct {
	offset float32
	color  okage.Color
}

// rgba is the type Rgba of linear_rgb_stops in resvg src/path.rs.
type rgba = [4]float32

// linearRGBStops is linear_rgb_stops in resvg src/path.rs.
func linearRGBStops(stops []gradientPoint) []gradientPoint {
	rgbaOf := func(c okage.Color) rgba { return rgba{c.Red(), c.Green(), c.Blue(), c.Alpha()} }

	out := make([]gradientPoint, 0, len(stops))
	for i := 0; i+1 < len(stops); i++ {
		c0, c1 := rgbaOf(stops[i].color), rgbaOf(stops[i+1].color)
		posA, posB := stops[i].offset, stops[i+1].offset
		out = append(out, stops[i])
		if posA < posB {
			a, b := toLinear(c0), toLinear(c1)
			out = split(a, b, posA, posB, 0, c0, 1, c1, 0, out)
		}
	}
	if len(stops) != 0 {
		out = append(out, stops[len(stops)-1])
	}
	return out
}

// mix is the inner function mix of linear_rgb_stops in resvg src/path.rs.
func mix(a, b rgba, t float32) rgba {
	var r rgba
	for i := range r {
		r[i] = a[i] + float32((b[i]-a[i])*t)
	}
	return r
}

// toLinear is the inner function to_linear of linear_rgb_stops in resvg
// src/path.rs.
func toLinear(c rgba) rgba {
	f := func(v float32) float32 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return crmath.Pow32((v+0.055)/1.055, 2.4)
	}
	return rgba{f(c[0]), f(c[1]), f(c[2]), c[3]}
}

// toSRGB is the inner function to_srgb of linear_rgb_stops in resvg
// src/path.rs.
func toSRGB(c rgba) rgba {
	f := func(v float32) float32 {
		if v <= 0.0031308 {
			return v * 12.92
		}
		return float32(1.055*crmath.Pow32(v, 1.0/2.4)) - 0.055
	}
	return rgba{f(c[0]), f(c[1]), f(c[2]), c[3]}
}

// split is the inner function split of linear_rgb_stops in resvg
// src/path.rs. It appends the stops that bring the interpolation in sRGB
// within the tolerance of the interpolation in linear RGB, and returns out.
//
// Port: the tuples of the arguments become separate arguments, and out is
// returned, since a Go slice cannot grow through its argument. The constants
// MAX_DEPTH and TOLERANCE of linear_rgb_stops are constants of split, the one
// function that reads them.
func split(
	a, b rgba,
	posA, posB float32,
	t0 float32, c0 rgba,
	t1 float32, c1 rgba,
	depth uint8,
	out []gradientPoint,
) []gradientPoint {
	const maxDepth uint8 = 8
	const tolerance float32 = 0.5 / 255.0

	t := (t0 + t1) / 2.0
	exact := toSRGB(mix(a, b, t))
	approx := mix(c0, c1, 0.5)
	close := true
	for i := range 3 {
		if !(abs32(exact[i]-approx[i]) <= tolerance) {
			close = false
			break
		}
	}
	if close || depth == maxDepth {
		return out
	}

	out = split(a, b, posA, posB, t0, c0, t, exact, depth+1, out)
	if color, ok := okage.ColorFromRGBA(exact[0], exact[1], exact[2], exact[3]); ok {
		out = append(out, gradientPoint{posA + float32((posB-posA)*t), color})
	}
	out = split(a, b, posA, posB, t, exact, t1, c1, depth+1, out)
	return out
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

	sx = float32(imgSize.Width()) / rect.Width()
	sy = float32(imgSize.Height()) / rect.Height()
	transform = path.TransformFromScale(sx, sy)
	neededBBox, ok := path.IntRectFromXYWH(0, 0, pixmap.Width(), pixmap.Height())
	if !ok {
		return nil, Transform{}, false
	}
	ctx = &context{
		maxBBox:    ctx.maxBBox,
		neededBBox: neededBBox,
		opt:        ctx.opt,
	}
	m := pixmap.AsMut()
	renderNodes(pattern.Root(), ctx, transform, &m)

	ts := path.DefaultTransform()
	ts = ts.PreConcat(pattern.Transform())
	ts = ts.PreTranslate(rect.X(), rect.Y())
	ts = ts.PreScale(1.0/sx, 1.0/sy)

	return pixmap, ts, true
}
