// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"math"
	"slices"

	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// The paints of krilla src/graphics/paint.rs, which fill and stroke text and
// paths.

// hashF32Bits writes the bits of a float32, as krilla hashes a float with
// to_bits.
func hashF32Bits(h *hasher, v float32) {
	h.u32(math.Float32bits(v))
}

// hashStops writes the derived Hash of a Vec<Stop>: the length, then each
// stop.
func hashStops(h *hasher, stops []Stop) {
	h.usize(len(stops))
	for _, s := range stops {
		s.hash(h)
	}
}

// LinearGradient is LinearGradient in krilla src/graphics/paint.rs. The zero
// Transform of Go is not the identity, so a caller sets Transform, such as
// to path.IdentityTransform(), the Default of upstream.
//
// A linear gradient.
type LinearGradient struct {
	// X1 is the x coordinate of the first point.
	X1 float32
	// Y1 is the y coordinate of the first point.
	Y1 float32
	// X2 is the x coordinate of the second point.
	X2 float32
	// Y2 is the y coordinate of the second point.
	Y2 float32
	// Transform is a transform that should be applied to the linear
	// gradient.
	Transform Transform
	// SpreadMethod is the spread method of the linear gradient.
	SpreadMethod SpreadMethod
	// Stops holds the color stops of the linear gradient.
	//
	// Note that all stops need to be in the same color space.
	Stops []Stop
	// AntiAlias is whether the gradient should be anti-aliased.
	AntiAlias bool
	// Extend says which ends extend past the stops, for the spread method
	// pad. It is not in krilla (D68). The zero value extends both ends, as
	// krilla does.
	Extend ExtendMode
}

// hash is the impl Hash for LinearGradient in krilla
// src/graphics/paint.rs.
func (g LinearGradient) hash(h *hasher) {
	hashF32Bits(h, g.X1)
	hashF32Bits(h, g.Y1)
	hashF32Bits(h, g.X2)
	hashF32Bits(h, g.Y2)
	h.transform(g.Transform)
	h.usize(int(g.SpreadMethod))
	hashStops(h, g.Stops)
	h.bool(g.AntiAlias)
	g.Extend.hash(h)
}

// hashPaint is the derived Hash of Paint for a linear gradient.
func (g LinearGradient) hashPaint(h *hasher) {
	h.usize(innerPaintLinearGradient)
	g.hash(h)
}

// RadialGradient is RadialGradient in krilla src/graphics/paint.rs. The zero
// Transform of Go is not the identity, so a caller sets Transform.
//
// A radial gradient.
type RadialGradient struct {
	// Fx is the x coordinate of the start circle.
	Fx float32
	// Fy is the y coordinate of the start circle.
	Fy float32
	// Fr is the radius of the start circle.
	Fr float32
	// Cx is the x coordinate of the end circle.
	Cx float32
	// Cy is the y coordinate of the end circle.
	Cy float32
	// Cr is the radius of the end circle.
	Cr float32
	// Transform is a transform that should be applied to the radial
	// gradient.
	Transform Transform
	// SpreadMethod is the spread method of the radial gradient.
	//
	// Note: The spread methods Repeat and Reflect are currently not
	// supported for radial gradients, and will fall back to Pad.
	SpreadMethod SpreadMethod
	// Stops holds the color stops of the radial gradient.
	//
	// Note that all stops need to be in the same color space.
	Stops []Stop
	// AntiAlias is whether the gradient should be anti-aliased.
	AntiAlias bool
	// Extend says which circles extend past the stops. It is not in krilla
	// (D68). The zero value extends both circles, as krilla does.
	Extend ExtendMode
}

// hash is the impl Hash for RadialGradient in krilla
// src/graphics/paint.rs.
func (g RadialGradient) hash(h *hasher) {
	hashF32Bits(h, g.Fx)
	hashF32Bits(h, g.Fy)
	hashF32Bits(h, g.Fr)
	hashF32Bits(h, g.Cx)
	hashF32Bits(h, g.Cy)
	hashF32Bits(h, g.Cr)
	h.transform(g.Transform)
	h.usize(int(g.SpreadMethod))
	hashStops(h, g.Stops)
	h.bool(g.AntiAlias)
	g.Extend.hash(h)
}

// hashPaint is the derived Hash of Paint for a radial gradient.
func (g RadialGradient) hashPaint(h *hasher) {
	h.usize(innerPaintRadialGradient)
	g.hash(h)
}

// SweepGradient is SweepGradient in krilla src/graphics/paint.rs. The zero
// Transform of Go is not the identity, so a caller sets Transform.
//
// A sweep gradient.
//
// Angles start from the right and go counter-clockwise with increasing
// values.
type SweepGradient struct {
	// Cx is the x coordinate of the center.
	Cx float32
	// Cy is the y coordinate of the center.
	Cy float32
	// StartAngle is the start angle.
	StartAngle float32
	// EndAngle is the end angle.
	EndAngle float32
	// Transform is a transform that should be applied to the sweep
	// gradient.
	Transform Transform
	// SpreadMethod is the spread method of the sweep gradient.
	SpreadMethod SpreadMethod
	// Stops holds the color stops of the sweep gradient.
	//
	// Note that all stops need to be in the same color space.
	Stops []Stop
	// AntiAlias is whether the gradient should be anti-aliased.
	AntiAlias bool
}

// hash is the impl Hash for SweepGradient in krilla src/graphics/paint.rs.
func (g SweepGradient) hash(h *hasher) {
	hashF32Bits(h, g.Cx)
	hashF32Bits(h, g.Cy)
	hashF32Bits(h, g.StartAngle)
	hashF32Bits(h, g.EndAngle)
	h.transform(g.Transform)
	h.usize(int(g.SpreadMethod))
	hashStops(h, g.Stops)
	h.bool(g.AntiAlias)
}

// hashPaint is the derived Hash of Paint for a sweep gradient.
func (g SweepGradient) hashPaint(h *hasher) {
	h.usize(innerPaintSweepGradient)
	g.hash(h)
}

// Pattern is Pattern in krilla src/graphics/paint.rs. The zero Transform of
// Go is not the identity, so a caller sets Transform.
//
// A pattern.
//
// IMPORTANT: Note that you must only use a pattern in the document that you
// created it with! If you use it in a different document, you will end up
// with an invalid PDF file.
type Pattern struct {
	// Stream is the stream of the pattern.
	Stream *Stream
	// Transform is a transform that should be applied to the pattern.
	Transform Transform
	// Width is the width of the pattern.
	Width float32
	// Height is the height of the pattern.
	Height float32
}

// hash is the impl Hash for Pattern in krilla src/graphics/paint.rs.
func (p Pattern) hash(h *hasher) {
	p.Stream.hash(h)
	h.transform(p.Transform)
	hashF32Bits(h, p.Width)
	hashF32Bits(h, p.Height)
}

// hashPaint is the derived Hash of Paint for a pattern.
func (p Pattern) hashPaint(h *hasher) {
	h.usize(innerPaintPattern)
	p.hash(h)
}

// The cases of InnerPaint in krilla src/graphics/paint.rs, which the derived
// Hash of Paint writes.
const (
	innerPaintColor = iota
	innerPaintLinearGradient
	innerPaintRadialGradient
	innerPaintSweepGradient
	innerPaintPattern
)

// Paint is Paint in krilla src/graphics/paint.rs, with the enum InnerPaint
// that it wraps. A Rust enum with data becomes an interface with one type for
// each case: ColorRGB, ColorLuma and ColorCMYK, which are each a Color,
// LinearGradient, RadialGradient, SweepGradient and Pattern. The impl From
// of upstream for each type is the conversion of the type into the
// interface. The Arc of the case Pattern is a copy of the value, which holds
// a pointer to its stream. A type switch on a Paint is the match on
// InnerPaint of upstream, with the case Color first.
//
// A paint.
type Paint interface {
	// hashPaint is the derived Hash of Paint: the case of InnerPaint, then
	// the value.
	hashPaint(h *hasher)
}

// paintAsRGB is Paint::as_rgb in krilla src/graphics/paint.rs. It returns
// false for a paint that is not a color, and for a CMYK color.
func paintAsRGB(p Paint) (ColorRGB, bool) {
	if c, ok := p.(Color); ok {
		return c.asRGB()
	}
	return ColorRGB{}, false
}

// SpreadMethod is SpreadMethod in krilla src/graphics/paint.rs. The zero
// value is SpreadMethodPad, the Default of upstream.
//
// A spread method.
type SpreadMethod uint8

// The values of SpreadMethod.
const (
	// SpreadMethodPad is the pad spread method.
	SpreadMethodPad SpreadMethod = iota
	// SpreadMethodReflect is the reflect spread method.
	SpreadMethodReflect
	// SpreadMethodRepeat is the repeat spread method.
	SpreadMethodRepeat
)

// Stop is Stop in krilla src/graphics/paint.rs.
//
// A color stop in a gradient.
type Stop struct {
	// Offset is the normalized offset of the stop.
	Offset NormalizedF32
	// Color is the color of the stop.
	Color Color
	// Opacity is the opacity of the stop.
	Opacity NormalizedF32
}

// hash is the derived Hash of Stop in krilla src/graphics/paint.rs.
func (s Stop) hash(h *hasher) {
	hashF32Bits(h, s.Offset.Get())
	s.Color.hash(h)
	hashF32Bits(h, s.Opacity.Get())
}

// LineCap is LineCap in krilla src/graphics/paint.rs. The zero value is
// LineCapButt, the Default of upstream.
//
// A line cap.
type LineCap uint8

// The values of LineCap.
const (
	// LineCapButt is the butt line cap.
	LineCapButt LineCap = iota
	// LineCapRound is the round line cap.
	LineCapRound
	// LineCapSquare is the square line cap.
	LineCapSquare
)

// toPDFLineCap is LineCap::to_pdf_line_cap in krilla
// src/graphics/paint.rs.
func (c LineCap) toPDFLineCap() pdfwriter.LineCapStyle {
	switch c {
	case LineCapButt:
		return pdfwriter.LineCapStyleButtCap
	case LineCapRound:
		return pdfwriter.LineCapStyleRoundCap
	default:
		return pdfwriter.LineCapStyleProjectingSquareCap
	}
}

// LineJoin is LineJoin in krilla src/graphics/paint.rs. The zero value is
// LineJoinMiter, the Default of upstream.
//
// A line join.
type LineJoin uint8

// The values of LineJoin.
const (
	// LineJoinMiter is the miter line join.
	LineJoinMiter LineJoin = iota
	// LineJoinRound is the round line join.
	LineJoinRound
	// LineJoinBevel is the bevel line join.
	LineJoinBevel
)

// toPDFLineJoin is LineJoin::to_pdf_line_join in krilla
// src/graphics/paint.rs.
func (j LineJoin) toPDFLineJoin() pdfwriter.LineJoinStyle {
	switch j {
	case LineJoinMiter:
		return pdfwriter.LineJoinStyleMiterJoin
	case LineJoinRound:
		return pdfwriter.LineJoinStyleRoundJoin
	default:
		return pdfwriter.LineJoinStyleBevelJoin
	}
}

// StrokeDash is StrokeDash in krilla src/graphics/paint.rs.
//
// A stroke dash.
type StrokeDash struct {
	// Array is the dash array.
	Array []float32
	// Offset is the offset of the dash.
	Offset float32
}

// Stroke is Stroke in krilla src/graphics/paint.rs. DefaultStroke gives the
// Default of upstream.
//
// A stroke.
type Stroke struct {
	// Paint is the paint of the stroke.
	Paint Paint
	// Width is the width of the stroke.
	Width float32
	// MiterLimit is the miter limit of the stroke.
	MiterLimit float32
	// LineCap is the line cap of the stroke.
	LineCap LineCap
	// LineJoin is the line join of the stroke.
	LineJoin LineJoin
	// Opacity is the opacity of the stroke.
	Opacity NormalizedF32
	// Dash is the optional dash of the stroke. Nil is None.
	Dash *StrokeDash
}

// DefaultStroke is the impl Default for Stroke in krilla
// src/graphics/paint.rs: a black stroke of width 1.
func DefaultStroke() Stroke {
	return Stroke{
		Paint:      ColorLumaBlack(),
		Width:      1.0,
		MiterLimit: 10.0,
		LineCap:    LineCapButt,
		LineJoin:   LineJoinMiter,
		Opacity:    NormalizedF32One,
		Dash:       nil,
	}
}

// intoTinySkia is Stroke::into_tiny_skia in krilla src/graphics/paint.rs. It
// gives the stroke of the package path. A dash that path.NewStrokeDash
// does not take gives no dash, as upstream does.
func (s Stroke) intoTinySkia() path.Stroke {
	stroke := path.Stroke{
		Width:      s.Width,
		MiterLimit: s.MiterLimit,
		Dash:       nil,
	}
	switch s.LineCap {
	case LineCapButt:
		stroke.LineCap = path.LineCapButt
	case LineCapRound:
		stroke.LineCap = path.LineCapRound
	default:
		stroke.LineCap = path.LineCapSquare
	}
	switch s.LineJoin {
	case LineJoinMiter:
		stroke.LineJoin = path.LineJoinMiter
	case LineJoinRound:
		stroke.LineJoin = path.LineJoinRound
	default:
		stroke.LineJoin = path.LineJoinBevel
	}

	if s.Dash != nil {
		// Rust moves the array into the dash, so the dash gets a copy.
		stroke.Dash, _ = path.NewStrokeDash(slices.Clone(s.Dash.Array), s.Dash.Offset)
	}

	return stroke
}

// FillRule is FillRule in krilla src/graphics/paint.rs. The zero value is
// FillRuleNonZero, the Default of upstream.
//
// A fill rule.
type FillRule uint8

// The values of FillRule.
const (
	// FillRuleNonZero is the non-zero fill rule.
	FillRuleNonZero FillRule = iota
	// FillRuleEvenOdd is the even-odd fill rule.
	FillRuleEvenOdd
)

// Fill is Fill in krilla src/graphics/paint.rs. DefaultFill gives the
// Default of upstream. The derived Hash of upstream hashes no Fill, Stroke
// or StrokeDash in the cache of the serializer, so they have no method
// hash.
//
// A fill.
type Fill struct {
	// Paint is the paint of the fill.
	Paint Paint
	// Opacity is the opacity of the fill.
	Opacity NormalizedF32
	// Rule is the fill rule that should be used when applying the fill.
	Rule FillRule
}

// DefaultFill is the impl Default for Fill in krilla src/graphics/paint.rs:
// an opaque black fill with the non-zero rule.
func DefaultFill() Fill {
	return Fill{
		Paint:   ColorLumaBlack(),
		Opacity: NormalizedF32One,
		Rule:    FillRuleNonZero,
	}
}
