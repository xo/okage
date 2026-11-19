// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"image"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// This file ports no file of krilla. It converts a shader of okage into a
// paint of pdf, for the device of ivy (D62 item 11, D68).

// PaintFromShader converts a shader of okage into a paint and an opacity.
// The caller sets them as the Paint and the Opacity of a Fill or a Stroke. It
// returns false for a shader that a paint cannot hold. The caller then draws
// that part with DrawRaster. It is not in krilla (D68).
//
// The shaders convert in this way:
//
//   - A SolidColor is a ColorRGB, and its alpha is the opacity.
//   - A LinearGradient is a LinearGradient with the same spread mode. The
//     decal spread mode is SpreadMethodPad with ExtendNone.
//   - A RadialGradient with the spread mode pad or decal, or of
//     NewRadialGradientExtend, is a RadialGradient with the ExtendMode of
//     its flags. With the spread mode repeat or reflect, it does not
//     convert, because krilla draws such a gradient with the mode pad.
//   - A SweepGradient with the spread mode pad, repeat or reflect is a
//     SweepGradient. With the decal spread mode, it does not convert.
//   - A MeshGradient is a MeshGradient.
//   - A Pattern with the spread mode repeat is a Pattern whose tile is its
//     pixmap, and with the spread mode reflect a Pattern whose tile holds
//     four mirrored copies of its pixmap. Its opacity is the opacity. A
//     Pattern with the spread mode pad or decal does not convert.
//
// A gradient keeps the alpha of each stop as the opacity of the stop. The
// colors are RGB colors of 8 bits. The conversion takes the colors as they
// are, which is the color space okage.ColorSpaceLinear of a Paint of okage,
// the default.
func (s *Surface) PaintFromShader(shader okage.Shader) (Paint, NormalizedF32, bool) {
	switch sh := shader.(type) {
	case okage.SolidColor:
		c, o := rgbOf(okage.Color(sh))
		return c, o, true
	case okage.LinearGradient:
		p := sh.Parameters()
		g := LinearGradient{
			X1: p.Start.X, Y1: p.Start.Y, X2: p.End.X, Y2: p.End.Y,
			Transform: p.Transform,
			Stops:     stopsOf(p.Stops),
		}
		switch p.Mode {
		case okage.SpreadModeRepeat:
			g.SpreadMethod = SpreadMethodRepeat
		case okage.SpreadModeReflect:
			g.SpreadMethod = SpreadMethodReflect
		case okage.SpreadModeDecal:
			g.Extend = ExtendNone
		}
		return g, NormalizedF32One, true
	case okage.RadialGradient:
		p := sh.Parameters()
		g := RadialGradient{
			Fx: p.Start.X, Fy: p.Start.Y, Fr: p.StartRadius,
			Cx: p.End.X, Cy: p.End.Y, Cr: p.EndRadius,
			Transform: p.Transform,
			Stops:     stopsOf(p.Stops),
		}
		switch {
		case p.Extend:
			g.Extend = extendModeOf(p.ExtendStart, p.ExtendEnd)
		case p.Mode == okage.SpreadModeDecal:
			g.Extend = ExtendNone
		case p.Mode != okage.SpreadModePad:
			return nil, 0, false
		}
		return g, NormalizedF32One, true
	case okage.SweepGradient:
		p := sh.Parameters()
		g := SweepGradient{
			Cx: p.Start.X, Cy: p.Start.Y,
			StartAngle: p.StartAngle, EndAngle: p.EndAngle,
			Transform: p.Transform,
			Stops:     stopsOf(p.Stops),
		}
		switch p.Mode {
		case okage.SpreadModeRepeat:
			g.SpreadMethod = SpreadMethodRepeat
		case okage.SpreadModeReflect:
			g.SpreadMethod = SpreadMethodReflect
		case okage.SpreadModeDecal:
			return nil, 0, false
		}
		return g, NormalizedF32One, true
	case okage.MeshGradient:
		p := sh.Parameters()
		if len(p.Triangles) == 0 {
			return nil, 0, false
		}
		m := MeshGradient{Triangles: make([]MeshTriangle, len(p.Triangles)), Transform: p.Transform}
		for i, t := range p.Triangles {
			for k, v := range t {
				c := v.Color
				c.ApplyOpacity(p.Opacity)
				rgb, o := rgbOf(c)
				m.Triangles[i][k] = MeshVertex{Point: v.Point, Color: rgb, Opacity: o}
			}
		}
		return m, NormalizedF32One, true
	case okage.Pattern:
		p := sh.Parameters()
		if p.Mode != okage.SpreadModeRepeat && p.Mode != okage.SpreadModeReflect {
			return nil, 0, false
		}
		pat, ok := s.patternOf(p)
		if !ok {
			return nil, 0, false
		}
		return pat, path.NormalizedF32Clamped(p.Opacity), true
	}
	return nil, 0, false
}

// rgbOf converts a color of okage into an RGB color of 8 bits and its
// opacity.
func rgbOf(c okage.Color) (ColorRGB, NormalizedF32) {
	u := c.ToColorU8()
	return NewColorRGB(u.Red(), u.Green(), u.Blue()), path.NormalizedF32Clamped(c.Alpha())
}

// stopsOf converts the stops of a gradient of okage.
func stopsOf(stops []okage.GradientStop) []Stop {
	out := make([]Stop, len(stops))
	for i, st := range stops {
		c, o := rgbOf(st.Color())
		out[i] = Stop{Offset: path.NormalizedF32Clamped(st.Position()), Color: c, Opacity: o}
	}
	return out
}

// patternOf makes the tiling pattern of a pattern of okage, for the spread
// mode repeat or reflect. For the mode reflect, the tile is an image of twice
// the width and twice the height, with the pixmap, its mirror in x, its
// mirror in y and its mirror in both. One image has no seams at the mirrors,
// which Poppler 26.11 draws for an image with a transform that mirrors it.
func (s *Surface) patternOf(p okage.PatternParameters) (Pattern, bool) {
	w, h := int(p.Pixmap.Width()), int(p.Pixmap.Height())
	if w == 0 || h == 0 {
		return Pattern{}, false
	}
	tw, th := w, h
	if p.Mode == okage.SpreadModeReflect {
		tw, th = 2*w, 2*h
	}
	if err := checkImageBytes(tw, th, 4); err != nil {
		s.sc.setError(imageError(err))
		return Pattern{}, false
	}
	img := image.NewNRGBA(image.Rect(0, 0, tw, th))
	pixels := p.Pixmap.Pixels()
	for y := range th {
		sy := y
		if sy >= h {
			sy = th - 1 - y
		}
		for x := range tw {
			sx := x
			if sx >= w {
				sx = tw - 1 - x
			}
			c := pixels.At(sy*w + sx).Demultiply()
			copy(img.Pix[y*img.Stride+4*x:], []byte{c.Red(), c.Green(), c.Blue(), c.Alpha()})
		}
	}
	pdfImage, err := ImageFromImage(img, p.Quality != okage.FilterQualityNearest)
	if err != nil {
		s.sc.setError(err)
		return Pattern{}, false
	}
	size, ok := path.SizeFromWH(float32(tw), float32(th))
	if !ok {
		return Pattern{}, false
	}
	b := s.StreamBuilder()
	ps := b.Surface()
	ps.DrawImage(pdfImage, size)
	ps.Finish()
	return Pattern{Stream: b.Finish(), Transform: p.Transform, Width: size.Width(), Height: size.Height()}, true
}
