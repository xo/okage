// Copyright 2024 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file holds the types of the crates peniko and color that velato uses
// (D41): a color, the stops of a gradient, a brush, a stroke and a blend
// mode.

import "github.com/xo/okage/internal/kurbo"

// Color is peniko::Color, an AlphaColor of sRGB: the components red, green,
// blue and alpha, from 0 to 1, with straight alpha.
type Color [4]float32

// The colors that velato names.
var (
	ColorTransparent = Color{0, 0, 0, 0}
	ColorBlack       = Color{0, 0, 0, 1}
	ColorWhite       = Color{1, 1, 1, 1}
)

// MultiplyAlpha is AlphaColor::multiply_alpha in the crate color.
func (c Color) MultiplyAlpha(alpha float32) Color {
	c[3] *= alpha
	return c
}

// WithAlpha is AlphaColor::with_alpha in the crate color.
func (c Color) WithAlpha(alpha float32) Color {
	c[3] = alpha
	return c
}

// ColorStop is peniko::ColorStop: a color at an offset of a gradient.
type ColorStop struct {
	Offset float32
	Color  Color
}

// ColorStops is peniko::ColorStops.
type ColorStops []ColorStop

// GradientKind is the kind of a peniko::Gradient.
type GradientKind uint8

// The kinds of a gradient.
const (
	GradientLinear GradientKind = iota
	GradientRadial
)

// Gradient is peniko::Gradient, with the two kinds that velato makes. A
// linear gradient goes from Start to End. A radial gradient has its center
// at Start and its radius in Radius, as Gradient::new_radial makes it. The
// gradient pads its ends.
type Gradient struct {
	Kind   GradientKind
	Start  kurbo.Point
	End    kurbo.Point
	Radius float32
	Stops  ColorStops
	// Focal is the focal point of a radial gradient, which is Start when
	// the gradient has no highlight.
	//
	// Port: peniko's new_radial has no focal point. okage takes the
	// highlight of a Lottie radial gradient, as ThorVG does (D43).
	Focal kurbo.Point
}

// BrushKind is the kind of a peniko::Brush.
type BrushKind uint8

// The kinds of a brush.
const (
	BrushSolid BrushKind = iota
	BrushGradient
)

// Brush is peniko::Brush, with the kinds solid and gradient.
type Brush struct {
	Kind     BrushKind
	Color    Color
	Gradient Gradient
}

// solidBrush returns a solid brush of c.
func solidBrush(c Color) Brush {
	return Brush{Kind: BrushSolid, Color: c}
}

// MultiplyAlpha is Brush::multiply_alpha in peniko.
func (b Brush) MultiplyAlpha(alpha float32) Brush {
	if alpha == 1.0 {
		return b
	}
	switch b.Kind {
	case BrushSolid:
		b.Color = b.Color.MultiplyAlpha(alpha)
	case BrushGradient:
		stops := make(ColorStops, len(b.Gradient.Stops))
		for i, s := range b.Gradient.Stops {
			stops[i] = ColorStop{Offset: s.Offset, Color: s.Color.MultiplyAlpha(alpha)}
		}
		b.Gradient.Stops = stops
	}
	return b
}

// Cap is kurbo::Cap.
type Cap uint8

// The caps of a stroke.
const (
	CapButt Cap = iota
	CapSquare
	CapRound
)

// Join is kurbo::Join.
type Join uint8

// The joins of a stroke.
const (
	JoinBevel Join = iota
	JoinMiter
	JoinRound
)

// Stroke is kurbo::Stroke, the style of a stroke, without dashes, which
// velato does not make.
type Stroke struct {
	Width      float64
	Join       Join
	MiterLimit float64
	StartCap   Cap
	EndCap     Cap
	// Dash holds the lengths of the dashes and the gaps, and DashOffset
	// the offset of the pattern, or Dash is nil for a solid stroke.
	//
	// Port: velato reads the dashes of a stroke, and does not use them.
	// okage dashes the stroke, as ThorVG does (D43).
	Dash       []float64
	DashOffset float64
}

// newStroke is Stroke::new in kurbo: round caps and joins, and a miter
// limit of 4.
func newStroke(width float64) Stroke {
	return Stroke{Width: width, Join: JoinRound, MiterLimit: 4.0, StartCap: CapRound, EndCap: CapRound}
}

// Mix is peniko::Mix, the separable and non-separable blend modes.
type Mix uint8

// The values of Mix, in the order of peniko.
const (
	MixNormal Mix = iota
	MixMultiply
	MixScreen
	MixOverlay
	MixDarken
	MixLighten
	MixColorDodge
	MixColorBurn
	MixHardLight
	MixSoftLight
	MixDifference
	MixExclusion
	MixHue
	MixSaturation
	MixColor
	MixLuminosity
)

// Compose is peniko::Compose, the Porter-Duff modes.
type Compose uint8

// The values of Compose, in the order of peniko.
const (
	ComposeClear Compose = iota
	ComposeCopy
	ComposeDest
	ComposeSrcOver
	ComposeDestOver
	ComposeSrcIn
	ComposeDestIn
	ComposeSrcOut
	ComposeDestOut
	ComposeSrcAtop
	ComposeDestAtop
	ComposeXor
	ComposePlus
	ComposePlusLighter
)

// BlendMode is peniko::BlendMode: a mix and a compose mode.
type BlendMode struct {
	Mix     Mix
	Compose Compose
}

// blendMix is impl From<Mix> for BlendMode in peniko.
func blendMix(m Mix) BlendMode {
	return BlendMode{Mix: m, Compose: ComposeSrcOver}
}

// blendCompose is impl From<Compose> for BlendMode in peniko.
func blendCompose(c Compose) BlendMode {
	return BlendMode{Mix: MixNormal, Compose: c}
}
