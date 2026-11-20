// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports no file of velato. It holds the text animators of a text
// layer, the grouping of their glyphs and the path of the text, which velato
// does not parse (D85). They follow the Lottie loader of ThorVG v1.1.1:
// parseTextRange in src/loaders/lottie/tvgLottieParser.cpp,
// LottieTextRange::factor in src/loaders/lottie/tvgLottieModel.cpp, and
// updateTextRange and updateURLFont in
// src/loaders/lottie/tvgLottieBuilder.cpp.

import (
	"math"

	"github.com/xo/okage/kurbo"
)

// TextAnimation holds the text animators of a text layer, the grouping of
// the glyphs that they change, and the path that the text follows (D85).
type TextAnimation struct {
	// Animators are the text animators, in the order of the file.
	Animators []TextAnimator
	// Group is the grouping of the glyphs that turn about one point.
	Group TextGroup
	// Anchor is the point of a group that it turns about, in the units of
	// a glyph at the size 100.
	Anchor Value[kurbo.Point]
	// Path is the path of the text, or nil.
	Path *TextPath
}

// TextGroup is the grouping of the glyphs of a text animator, the key g of
// the key m of a text layer.
type TextGroup uint8

// The values of TextGroup. ThorVG gives each glyph a group of its own for
// TextGroupAll, as for TextGroupCharacters.
const (
	TextGroupCharacters TextGroup = 1
	TextGroupWords      TextGroup = 2
	TextGroupLines      TextGroup = 3
	TextGroupAll        TextGroup = 4
)

// TextBased is what a range selector counts, its key b.
type TextBased uint8

// The values of TextBased. ThorVG counts the words as the number of the
// line plus the number of the spaces before the glyph.
const (
	TextBasedCharacters        TextBased = 1
	TextBasedCharactersNoSpace TextBased = 2
	TextBasedWords             TextBased = 3
	TextBasedLines             TextBased = 4
)

// TextSelectorShape is the shape of a range selector, its key sh.
type TextSelectorShape uint8

// The values of TextSelectorShape.
const (
	TextShapeSquare   TextSelectorShape = 1
	TextShapeRampUp   TextSelectorShape = 2
	TextShapeRampDown TextSelectorShape = 3
	TextShapeTriangle TextSelectorShape = 4
	TextShapeRound    TextSelectorShape = 5
	TextShapeSmooth   TextSelectorShape = 6
)

// TextAnimator is a text animator: a range selector, and the style that
// it gives each glyph in proportion to the share of the glyph in the range.
type TextAnimator struct {
	// Start, End and Offset give the range, in percent of the text, or in
	// glyphs when Index is true.
	Start, End, Offset Value[float64]
	Index              bool
	// Amount is the share of the style that the range gives, in percent.
	Amount Value[float64]
	// Smoothness softens the edge of the shape square, in percent.
	Smoothness Value[float64]
	// MinEase and MaxEase ease the share of each glyph, in percent.
	MinEase, MaxEase Value[float64]
	Based            TextBased
	Shape            TextSelectorShape

	// Position moves a glyph, in pixels.
	Position Value[kurbo.Point]
	// Scale scales a glyph, in percent.
	Scale Value[kurbo.Vec2]
	// Rotation turns the group of a glyph, in degrees.
	Rotation Value[float64]
	// Opacity, FillOpacity and StrokeOpacity are in percent.
	Opacity, FillOpacity, StrokeOpacity Value[float64]
	// FillColor, StrokeColor and StrokeWidth are nil when the animator
	// does not change them.
	FillColor, StrokeColor *Value[Color]
	StrokeWidth            *Value[float64]
	// Tracking adds space before a glyph, in the units of a glyph at the
	// size 100.
	Tracking Value[float64]
	// LineSpacing moves the next lines down, in pixels.
	LineSpacing Value[float64]
}

// TextPath is the path that a text follows: the mask of the layer with the
// index Mask, and the margin before the first glyph, in pixels.
type TextPath struct {
	Mask        int
	FirstMargin Value[float64]
}

// convTextAnimation converts the text animators, the grouping and the path
// of a text layer. It returns nil when the layer has none of them.
func (c *converter) convTextAnimation(source *TextData) *TextAnimation {
	var path *TextPath
	if p := source.Path; p != nil && p.Mask != nil && *p.Mask >= 0 {
		path = &TextPath{Mask: *p.Mask, FirstMargin: c.convScalar(orFloat(p.FirstMargin, &floatValueZero))}
	}
	if len(source.Animators) == 0 && path == nil {
		return nil
	}
	anim := &TextAnimation{Group: TextGroupCharacters, Anchor: fixedValue(kurbo.Point{}), Path: path}
	if m := source.Alignment; m != nil {
		if m.Group != nil {
			anim.Group = TextGroup(*m.Group)
		}
		if m.Anchor != nil {
			anim.Anchor = c.convMultiPoint(m.Anchor)
		}
	}
	for i := range source.Animators {
		anim.Animators = append(anim.Animators, c.convTextAnimator(&source.Animators[i]))
	}
	return anim
}

// convTextAnimator converts a text animator with the defaults of
// LottieTextRange of ThorVG. The amount is 0 when the file does not give
// it, as in ThorVG. ThorVG gives the key rn a range at a random place with
// rand of C, which okage does not follow: it reads the range as it is.
func (c *converter) convTextAnimator(source *TextAnimatorSchema) TextAnimator {
	s, st := &source.Selector, &source.Style
	enum := func(v *int, def uint8) uint8 {
		if v == nil {
			return def
		}
		return uint8(*v)
	}
	hundred := staticFloat(100)
	a := TextAnimator{
		Start:      c.convScalar(orFloat(s.Start, &floatValueZero)),
		End:        c.convScalar(orFloat(s.End, &hundred)),
		Offset:     c.convScalar(orFloat(s.Offset, &floatValueZero)),
		Index:      s.Units != nil && *s.Units != 1,
		Amount:     c.convScalar(orFloat(s.Amount, &floatValueZero)),
		Smoothness: c.convScalar(orFloat(s.Smoothness, &hundred)),
		MinEase:    c.convScalar(orFloat(s.MinEase, &floatValueZero)),
		MaxEase:    c.convScalar(orFloat(s.MaxEase, &floatValueZero)),
		Based:      TextBased(enum(s.Based, uint8(TextBasedCharacters))),
		Shape:      TextSelectorShape(enum(s.Shape, uint8(TextShapeSquare))),

		Position:      fixedValue(kurbo.Point{}),
		Scale:         fixedValue(kurbo.Vec2{X: 100, Y: 100}),
		Rotation:      c.convScalar(orFloat(st.Rotation, &floatValueZero)),
		Opacity:       c.convScalar(orFloat(st.Opacity, &hundred)),
		FillOpacity:   c.convScalar(orFloat(st.FillOpacity, &hundred)),
		StrokeOpacity: c.convScalar(orFloat(st.StrokeOpacity, &hundred)),
		Tracking:      c.convScalar(orFloat(st.Tracking, &floatValueZero)),
		LineSpacing:   c.convScalar(orFloat(st.LineSpacing, &floatValueZero)),
	}
	if st.Position != nil {
		a.Position = c.convMultiPoint(st.Position)
	}
	if st.Scale != nil {
		a.Scale = c.convVec2(st.Scale)
	}
	if st.FillColor != nil {
		v := c.convColor(st.FillColor)
		a.FillColor = &v
	}
	if st.StrokeColor != nil {
		v := c.convColor(st.StrokeColor)
		a.StrokeColor = &v
	}
	if st.StrokeWidth != nil {
		v := c.convScalar(st.StrokeWidth)
		a.StrokeWidth = &v
	}
	return a
}

// textEpsilon is FLOAT_EPSILON of ThorVG, for tvg::zero and tvg::equal.
const textEpsilon = 1.1920929e-07

// clamp32 is tvg::clamp: lo when v is below lo, else hi when v is above hi.
func clamp32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// factor returns the share of the glyph idx of a text of total bytes in the
// range of a at frame, from 0 to 1, as LottieTextRange::factor of ThorVG
// computes it, in float32.
func (a *TextAnimator) factor(frame float64, total, idx float32) float32 {
	offset := float32(a.Offset.Evaluate(frame))
	start := float32(a.Start.Evaluate(frame)) + offset
	end := float32(a.End.Evaluate(frame)) + offset
	divisor := float32(1)
	if !a.Index {
		divisor = 100 / total
	}
	start /= divisor
	end /= divisor
	equal := float32(math.Abs(float64(start-end))) < textEpsilon
	f := float32(0)
	switch a.Shape {
	case TextShapeSquare:
		smoothness := float32(a.Smoothness.Evaluate(frame))
		if float32(math.Abs(float64(smoothness))) < textEpsilon {
			if idx >= float32(math.RoundToEven(float64(start))) && idx < float32(math.RoundToEven(float64(end))) {
				f = 1
			}
		} else {
			if idx >= float32(math.Floor(float64(start))) {
				if diff := idx - start; diff < 0 {
					f = min(end, 1) + diff
				} else {
					f = end - idx
				}
			}
			smoothness *= 0.01
			f = (f - float32((1-smoothness)*0.5)) / smoothness
		}
	case TextShapeRampUp:
		switch {
		case !equal:
			f = (0.5 + idx - start) / (end - start)
		case idx >= end:
			f = 1
		}
	case TextShapeRampDown:
		switch {
		case !equal:
			f = 1 - (0.5+idx-start)/(end-start)
		case idx < end:
			f = 1
		}
	case TextShapeTriangle:
		if !equal {
			f = 2 * (0.5 + idx - start) / (end - start)
		}
		if f >= 1 {
			f = 2 - f
		}
	case TextShapeRound:
		idx = clamp32(idx+(0.5-start), 0, end-start)
		r := float32(0.5 * (end - start))
		t := idx - r
		if !equal {
			f = float32(math.Sqrt(float64(1 - t*t/(r*r))))
		}
	case TextShapeSmooth:
		idx = clamp32(idx+(0.5-start), 0, end-start)
		if !equal {
			f = float32(0.5 * float32(1+float32(math.Cos(float64(math.Pi*float32(1+2*idx/(end-start)))))))
		}
	}
	f = clamp32(f, 0, 1)

	minEase := clamp32(float32(a.MinEase.Evaluate(frame)), -100, 100)
	maxEase := clamp32(float32(a.MaxEase.Evaluate(frame)), -100, 100)
	if float32(math.Abs(float64(minEase))) >= textEpsilon || float32(math.Abs(float64(maxEase))) >= textEpsilon {
		in := EasingHandle{X: 1, Y: 1}
		out := EasingHandle{}
		if maxEase > 0 {
			in.X = float64(1 - float32(maxEase*0.01))
		} else {
			in.Y = float64(1 + float32(maxEase*0.01))
		}
		if minEase > 0 {
			out.X = float64(minEase * 0.01)
		} else {
			out.Y = float64(-minEase * 0.01)
		}
		if out.X != out.Y || in.X != in.Y {
			f = float32(tweenF64(0, 1, float64(f), Easing{O: out, I: in}))
		}
	}
	f = clamp32(f, 0, 1)
	return f * float32(a.Amount.Evaluate(frame)) * 0.01
}

// textRGB is a color of a text with 8 bits for each channel, as RGB32 of
// ThorVG.
type textRGB [3]int32

// rgbOf returns the 8-bit channels of c, rounded.
func rgbOf(c Color) textRGB {
	var v textRGB
	for i := range v {
		v[i] = int32(math.RoundToEven(float64(c[i]) * 255))
	}
	return v
}

// lerp is lerp of RGB32 in ThorVG: each channel moves by t toward to, and
// is cut toward 0 and to the range 0 to 255.
func (c textRGB) lerp(to textRGB, t float32) textRGB {
	var v textRGB
	for i := range v {
		v[i] = min(max(int32(float32(c[i])+float32(float32(to[i]-c[i])*t)), 0), 255)
	}
	return v
}

// color returns c as a Color with the alpha a, as textColor converts it.
func (c textRGB) color(a float32) Color {
	return Color{float32(float64(c[0]) / 255), float32(float64(c[1]) / 255), float32(float64(c[2]) / 255), a}
}

// opacity255 is the opacity of a text animator, in percent, as an opacity
// of 8 bits, as getValue(uint8_t&) of ThorVG cuts it.
func opacity255(v float64) float32 {
	return float32(uint8(min(max(float32(v)*2.55, 0), 255)))
}

// mixOpacity moves the opacity o of 8 bits by f toward to, and cuts it, as
// updateTextRange of ThorVG does.
func mixOpacity(o, to, f float32) float32 {
	return float32(uint8(min(max(o-float32(f*(o-to)), 0), 255)))
}

// textStyle is the style of a glyph after the text animators.
type textStyle struct {
	// grouped reports if an animator selects the glyph, so that it draws
	// in the group of its glyph.
	grouped bool
	// translate is the offset in pixels, scale the scale and rotate the
	// angle in degrees of the glyph.
	translate kurbo.Vec2
	scale     kurbo.Vec2
	rotate    float64
	// fill and stroke are the colors, and opacity, fillOpacity and
	// strokeOpacity the opacities of 8 bits.
	fill, stroke                        textRGB
	opacity, fillOpacity, strokeOpacity float32
	// strokeWidth is the width of the outline in pixels.
	strokeWidth float64
}

// newTextStyle returns the style of a glyph of doc without an animator.
func newTextStyle(doc *TextDocument) textStyle {
	return textStyle{
		scale:         kurbo.Vec2{X: 1, Y: 1},
		fill:          rgbOf(doc.Fill),
		stroke:        rgbOf(doc.Stroke),
		opacity:       255,
		fillOpacity:   255,
		strokeOpacity: 255,
		strokeWidth:   doc.StrokeWidth,
	}
}

// brushes returns the fill and the stroke of s, with the alpha alpha.
func (s *textStyle) brushes(alpha float64) (Brush, Brush) {
	a := float32(alpha)
	fill := s.fill.color(s.opacity / 255 * (s.fillOpacity / 255) * a)
	stroke := s.stroke.color(s.opacity / 255 * (s.strokeOpacity / 255) * a)
	return solidBrush(fill), solidBrush(stroke)
}

// wholeTextStyle returns the style of each glyph of a text that the sink
// shapes, as updateURLFont of ThorVG gives it: each animator whose share
// of the first glyph is not 0 sets the opacity and moves the fill color.
// When it has a stroke width, it also sets the width and the color of the
// outline. The other properties of the animators do nothing.
func wholeTextStyle(anim *TextAnimation, doc *TextDocument, frame float64) textStyle {
	s := newTextStyle(doc)
	if anim == nil {
		return s
	}
	total := float32(len(doc.Text))
	colors := s
	for i := range anim.Animators {
		a := &anim.Animators[i]
		f := a.factor(frame, total, 0)
		if float32(math.Abs(float64(f))) < textEpsilon {
			continue
		}
		a.mixColors(&colors, frame, f)
		s.fill = colors.fill
		s.opacity = opacity255(a.Opacity.Evaluate(frame))
		if a.StrokeWidth != nil {
			s.strokeWidth = float64(f) * a.StrokeWidth.Evaluate(frame)
			s.stroke = colors.stroke
		}
	}
	return s
}

// mixColors moves the fill and the stroke colors of s by f toward the
// colors of a, when a has them.
func (a *TextAnimator) mixColors(s *textStyle, frame float64, f float32) {
	if a.FillColor != nil {
		s.fill = s.fill.lerp(rgbOf(a.FillColor.Evaluate(frame)), f)
	}
	if a.StrokeColor != nil {
		s.stroke = s.stroke.lerp(rgbOf(a.StrokeColor.Evaluate(frame)), f)
	}
}
