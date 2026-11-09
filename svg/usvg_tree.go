// Copyright 2019 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"bytes"
	"image"
	_ "image/gif"  // Registers the GIF decoder for imageKindActualSize.
	_ "image/jpeg" // Registers the JPEG decoder for imageKindActualSize.
	_ "image/png"  // Registers the PNG decoder for imageKindActualSize.
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	_ "golang.org/x/image/webp" // Registers the WebP decoder for imageKindActualSize.

	"github.com/xo/okage/path"
)

// This file ports usvg src/tree/mod.rs.
//
// Port: a Rust Arc<T> that the tree shares becomes a Go pointer *T, and the
// tree shares the same pointer where upstream shares the same Arc. A private
// field of type Option<T> becomes a pointer *T, or a nil slice or interface,
// and nil is the Rust None. A getter that returns an Option returns a second
// result that is false for None.

// NonZeroPositiveF32 is the re-export of strict_num::NonZeroPositiveF32 in
// usvg src/tree/mod.rs.
type NonZeroPositiveF32 = path.NonZeroPositiveF32

// NormalizedF32 is the re-export of strict_num::NormalizedF32 in usvg
// src/tree/mod.rs.
type NormalizedF32 = path.NormalizedF32

// PositiveF32 is the re-export of strict_num::PositiveF32 in usvg
// src/tree/mod.rs.
type PositiveF32 = path.PositiveF32

// Opacity is Opacity in usvg src/tree/mod.rs. It is an alias to
// `NormalizedF32`.
type Opacity = path.NormalizedF32

// nonEmptyString is NonEmptyString in usvg src/tree/mod.rs.
//
// Must not be clone-able to preserve ID uniqueness.
//
// Port: Go cannot forbid a copy, so the parser must not copy an ID to two
// elements.
type nonEmptyString string

// newNonEmptyString is NonEmptyString::new in usvg src/tree/mod.rs.
func newNonEmptyString(s string) (nonEmptyString, bool) {
	if strings.TrimSpace(s) == "" {
		return "", false
	}

	return nonEmptyString(s), true
}

// get is NonEmptyString::get in usvg src/tree/mod.rs.
func (s nonEmptyString) get() string {
	return string(s)
}

// take is NonEmptyString::take in usvg src/tree/mod.rs.
func (s nonEmptyString) take() string {
	return string(s)
}

// NonZeroF32 is NonZeroF32 in usvg src/tree/mod.rs. It is a non-zero `f32`.
//
// Just like `f32` but immutable and guarantee to never be zero.
type NonZeroF32 float32

// NewNonZeroF32 is NonZeroF32::new in usvg src/tree/mod.rs. It creates a new
// `NonZeroF32` value.
func NewNonZeroF32(n float32) (NonZeroF32, bool) {
	if ApproxEqUlps32(n, 0.0, 4) {
		return 0, false
	}
	return NonZeroF32(n), true
}

// Get is NonZeroF32::get in usvg src/tree/mod.rs. It returns an underlying
// value.
func (n NonZeroF32) Get() float32 {
	return float32(n)
}

// units is Units in usvg src/tree/mod.rs.
//
// `Units` cannot have a default value, because it changes depending on an
// element.
type units uint8

// The values of units.
const (
	unitsUserSpaceOnUse units = iota
	unitsObjectBoundingBox
)

// String is the impl Display for Units in usvg src/tree/mod.rs.
func (u units) String() string {
	switch u {
	case unitsUserSpaceOnUse:
		return "userSpaceOnUse" //nolint:goconst // an SVG keyword, as upstream writes it
	case unitsObjectBoundingBox:
		return "objectBoundingBox"
	}
	return "Units(?)"
}

// visibility is Visibility in usvg src/tree/mod.rs. It is a visibility
// property.
//
// `visibility` attribute in the SVG.
//
// The zero value is visibilityVisible, the Default of upstream.
type visibility uint8

// The values of visibility.
const (
	visibilityVisible visibility = iota
	visibilityHidden
	visibilityCollapse
)

// String returns the Rust name of the value.
func (v visibility) String() string {
	switch v {
	case visibilityVisible:
		return "Visible"
	case visibilityHidden:
		return "Hidden"
	case visibilityCollapse:
		return "Collapse"
	}
	return "Visibility(?)"
}

// ShapeRendering is ShapeRendering in usvg src/tree/mod.rs. It is a shape
// rendering method.
//
// `shape-rendering` attribute in the SVG.
//
// The Default of upstream is ShapeRenderingGeometricPrecision, which
// DefaultShapeRendering returns.
type ShapeRendering uint8

// The values of ShapeRendering.
const (
	ShapeRenderingOptimizeSpeed ShapeRendering = iota
	ShapeRenderingCrispEdges
	ShapeRenderingGeometricPrecision
)

// String returns the SVG name of the value, as the impl FromStr for
// ShapeRendering in usvg src/tree/mod.rs reads it.
func (r ShapeRendering) String() string {
	switch r {
	case ShapeRenderingOptimizeSpeed:
		return "optimizeSpeed" //nolint:goconst // the SVG name of the value
	case ShapeRenderingCrispEdges:
		return "crispEdges" //nolint:goconst // an SVG keyword, as upstream writes it
	case ShapeRenderingGeometricPrecision:
		return "geometricPrecision" //nolint:goconst // the SVG name of the value
	}
	return "ShapeRendering(?)"
}

// UseShapeAntialiasing is ShapeRendering::use_shape_antialiasing in usvg
// src/tree/mod.rs. It checks if anti-aliasing should be enabled.
func (r ShapeRendering) UseShapeAntialiasing() bool {
	switch r {
	case ShapeRenderingOptimizeSpeed:
		return false
	case ShapeRenderingCrispEdges:
		return false
	case ShapeRenderingGeometricPrecision:
		return true
	}
	return true
}

// DefaultShapeRendering is the impl Default for ShapeRendering in usvg
// src/tree/mod.rs.
func DefaultShapeRendering() ShapeRendering {
	return ShapeRenderingGeometricPrecision
}

// ParseShapeRendering is the impl FromStr for ShapeRendering in usvg
// src/tree/mod.rs.
//
// Port: upstream returns the error "invalid" for an unknown name. Go returns
// false, because the error holds no data.
func ParseShapeRendering(s string) (ShapeRendering, bool) {
	switch s {
	case "optimizeSpeed":
		return ShapeRenderingOptimizeSpeed, true
	case "crispEdges":
		return ShapeRenderingCrispEdges, true
	case "geometricPrecision":
		return ShapeRenderingGeometricPrecision, true
	}
	return 0, false
}

// TextRendering is TextRendering in usvg src/tree/mod.rs. It is a text
// rendering method.
//
// `text-rendering` attribute in the SVG.
//
// The Default of upstream is TextRenderingOptimizeLegibility, which
// DefaultTextRendering returns.
type TextRendering uint8

// The values of TextRendering.
const (
	TextRenderingOptimizeSpeed TextRendering = iota
	TextRenderingOptimizeLegibility
	TextRenderingGeometricPrecision
)

// String returns the SVG name of the value, as the impl FromStr for
// TextRendering in usvg src/tree/mod.rs reads it.
func (r TextRendering) String() string {
	switch r {
	case TextRenderingOptimizeSpeed:
		return "optimizeSpeed"
	case TextRenderingOptimizeLegibility:
		return "optimizeLegibility" //nolint:goconst // an SVG keyword, as upstream writes it
	case TextRenderingGeometricPrecision:
		return "geometricPrecision"
	}
	return "TextRendering(?)"
}

// DefaultTextRendering is the impl Default for TextRendering in usvg
// src/tree/mod.rs.
func DefaultTextRendering() TextRendering {
	return TextRenderingOptimizeLegibility
}

// ParseTextRendering is the impl FromStr for TextRendering in usvg
// src/tree/mod.rs.
//
// Port: upstream returns the error "invalid" for an unknown name. Go returns
// false, because the error holds no data.
func ParseTextRendering(s string) (TextRendering, bool) {
	switch s {
	case "optimizeSpeed":
		return TextRenderingOptimizeSpeed, true
	case "optimizeLegibility":
		return TextRenderingOptimizeLegibility, true
	case "geometricPrecision":
		return TextRenderingGeometricPrecision, true
	}
	return 0, false
}

// ImageRendering is ImageRendering in usvg src/tree/mod.rs. It is an image
// rendering method.
//
// `image-rendering` attribute in the SVG.
//
// The zero value is ImageRenderingOptimizeQuality, the Default of upstream.
type ImageRendering uint8

// The values of ImageRendering.
const (
	ImageRenderingOptimizeQuality ImageRendering = iota
	ImageRenderingOptimizeSpeed
	// ImageRenderingSmooth and the values after it follow a comment of
	// upstream, which says that they can only appear as presentation
	// attributes.
	ImageRenderingSmooth
	ImageRenderingHighQuality
	ImageRenderingCrispEdges
	ImageRenderingPixelated
)

// String returns the SVG name of the value, as the impl FromStr for
// ImageRendering in usvg src/tree/mod.rs reads it.
func (r ImageRendering) String() string {
	switch r {
	case ImageRenderingOptimizeQuality:
		return "optimizeQuality" //nolint:goconst // an SVG keyword, as upstream writes it
	case ImageRenderingOptimizeSpeed:
		return "optimizeSpeed"
	case ImageRenderingSmooth:
		return "smooth" //nolint:goconst // an SVG keyword, as upstream writes it
	case ImageRenderingHighQuality:
		return "high-quality" //nolint:goconst // an SVG keyword, as upstream writes it
	case ImageRenderingCrispEdges:
		return "crisp-edges" //nolint:goconst // an SVG keyword, as upstream writes it
	case ImageRenderingPixelated:
		return "pixelated" //nolint:goconst // an SVG keyword, as upstream writes it
	}
	return "ImageRendering(?)"
}

// ParseImageRendering is the impl FromStr for ImageRendering in usvg
// src/tree/mod.rs.
//
// Port: upstream returns the error "invalid" for an unknown name. Go returns
// false, because the error holds no data.
func ParseImageRendering(s string) (ImageRendering, bool) {
	switch s {
	case "optimizeQuality":
		return ImageRenderingOptimizeQuality, true
	case "optimizeSpeed":
		return ImageRenderingOptimizeSpeed, true
	case "smooth":
		return ImageRenderingSmooth, true
	case "high-quality":
		return ImageRenderingHighQuality, true
	case "crisp-edges":
		return ImageRenderingCrispEdges, true
	case "pixelated":
		return ImageRenderingPixelated, true
	}
	return 0, false
}

// BlendMode is BlendMode in usvg src/tree/mod.rs. It is a blending mode
// property.
//
// `mix-blend-mode` attribute in the SVG.
//
// The zero value is BlendModeNormal, the Default of upstream.
type BlendMode uint8

// The values of BlendMode.
const (
	BlendModeNormal BlendMode = iota
	BlendModeMultiply
	BlendModeScreen
	BlendModeOverlay
	BlendModeDarken
	BlendModeLighten
	BlendModeColorDodge
	BlendModeColorBurn
	BlendModeHardLight
	BlendModeSoftLight
	BlendModeDifference
	BlendModeExclusion
	BlendModeHue
	BlendModeSaturation
	BlendModeColor
	BlendModeLuminosity
	BlendModePlusLighter
)

// String is the impl Display for BlendMode in usvg src/tree/mod.rs.
func (m BlendMode) String() string {
	switch m {
	case BlendModeNormal:
		return "normal" //nolint:goconst // an SVG keyword, as upstream writes it
	case BlendModeMultiply:
		return "multiply"
	case BlendModeScreen:
		return "screen"
	case BlendModeOverlay:
		return "overlay"
	case BlendModeDarken:
		return "darken"
	case BlendModeLighten:
		return "lighten"
	case BlendModeColorDodge:
		return "color-dodge"
	case BlendModeColorBurn:
		return "color-burn"
	case BlendModeHardLight:
		return "hard-light"
	case BlendModeSoftLight:
		return "soft-light"
	case BlendModeDifference:
		return "difference"
	case BlendModeExclusion:
		return "exclusion"
	case BlendModeHue:
		return "hue"
	case BlendModeSaturation:
		return "saturation"
	case BlendModeColor:
		return "color" //nolint:goconst // an SVG keyword, as upstream writes it
	case BlendModeLuminosity:
		return "luminosity"
	case BlendModePlusLighter:
		return "plus-lighter"
	}
	return "BlendMode(?)"
}

// SpreadMethod is SpreadMethod in usvg src/tree/mod.rs. It is a spread
// method.
//
// `spreadMethod` attribute in the SVG.
//
// The zero value is SpreadMethodPad, the Default of upstream.
type SpreadMethod uint8

// The values of SpreadMethod.
const (
	SpreadMethodPad SpreadMethod = iota
	SpreadMethodReflect
	SpreadMethodRepeat
)

// String returns the Rust name of the value.
func (m SpreadMethod) String() string {
	switch m {
	case SpreadMethodPad:
		return "Pad"
	case SpreadMethodReflect:
		return "Reflect"
	case SpreadMethodRepeat:
		return "Repeat"
	}
	return "SpreadMethod(?)"
}

// BaseGradient is BaseGradient in usvg src/tree/mod.rs. It is a generic
// gradient.
type BaseGradient struct {
	id           nonEmptyString
	units        units // used only during parsing
	transform    Transform
	spreadMethod SpreadMethod
	stops        []Stop
}

// ID is BaseGradient::id in usvg src/tree/mod.rs. It returns the element's
// ID.
//
// Taken from the SVG itself.
// Used only during SVG writing. `resvg` doesn't rely on this property.
func (g *BaseGradient) ID() string {
	return g.id.get()
}

// Transform is BaseGradient::transform in usvg src/tree/mod.rs. It returns
// the gradient transform.
//
// `gradientTransform` in SVG.
func (g *BaseGradient) Transform() Transform {
	return g.transform
}

// SpreadMethod is BaseGradient::spread_method in usvg src/tree/mod.rs. It
// returns the gradient spreading method.
//
// `spreadMethod` in SVG.
func (g *BaseGradient) SpreadMethod() SpreadMethod {
	return g.spreadMethod
}

// Stops is BaseGradient::stops in usvg src/tree/mod.rs. It returns a list of
// `stop` elements.
func (g *BaseGradient) Stops() []Stop {
	return g.stops
}

// LinearGradient is LinearGradient in usvg src/tree/mod.rs. It is a linear
// gradient.
//
// `linearGradient` element in SVG.
//
// Port: the impl Deref<Target = BaseGradient> becomes an embedded
// BaseGradient.
type LinearGradient struct {
	BaseGradient

	x1 float32
	y1 float32
	x2 float32
	y2 float32
}

// X1 is LinearGradient::x1 in usvg src/tree/mod.rs. It returns the `x1`
// coordinate.
func (g *LinearGradient) X1() float32 {
	return g.x1
}

// Y1 is LinearGradient::y1 in usvg src/tree/mod.rs. It returns the `y1`
// coordinate.
func (g *LinearGradient) Y1() float32 {
	return g.y1
}

// X2 is LinearGradient::x2 in usvg src/tree/mod.rs. It returns the `x2`
// coordinate.
func (g *LinearGradient) X2() float32 {
	return g.x2
}

// Y2 is LinearGradient::y2 in usvg src/tree/mod.rs. It returns the `y2`
// coordinate.
func (g *LinearGradient) Y2() float32 {
	return g.y2
}

// RadialGradient is RadialGradient in usvg src/tree/mod.rs. It is a radial
// gradient.
//
// `radialGradient` element in SVG.
//
// Port: the impl Deref<Target = BaseGradient> becomes an embedded
// BaseGradient.
type RadialGradient struct {
	BaseGradient

	cx float32
	cy float32
	r  PositiveF32
	fx float32
	fy float32
	fr PositiveF32
}

// Cx is RadialGradient::cx in usvg src/tree/mod.rs. It returns the `cx`
// coordinate.
func (g *RadialGradient) Cx() float32 {
	return g.cx
}

// Cy is RadialGradient::cy in usvg src/tree/mod.rs. It returns the `cy`
// coordinate.
func (g *RadialGradient) Cy() float32 {
	return g.cy
}

// R is RadialGradient::r in usvg src/tree/mod.rs. It returns the gradient
// radius.
func (g *RadialGradient) R() PositiveF32 {
	return g.r
}

// Fx is RadialGradient::fx in usvg src/tree/mod.rs. It returns the `fx`
// coordinate.
func (g *RadialGradient) Fx() float32 {
	return g.fx
}

// Fy is RadialGradient::fy in usvg src/tree/mod.rs. It returns the `fy`
// coordinate.
func (g *RadialGradient) Fy() float32 {
	return g.fy
}

// Fr is RadialGradient::fr in usvg src/tree/mod.rs. It returns the focal
// radius.
func (g *RadialGradient) Fr() PositiveF32 {
	return g.fr
}

// StopOffset is StopOffset in usvg src/tree/mod.rs. It is an alias to
// `NormalizedF32`.
type StopOffset = path.NormalizedF32

// Stop is Stop in usvg src/tree/mod.rs. It is a gradient's stop element.
//
// `stop` element in SVG.
type Stop struct {
	offset  StopOffset
	color   Color
	opacity Opacity
}

// Offset is Stop::offset in usvg src/tree/mod.rs. It returns the gradient
// stop offset.
//
// `offset` in SVG.
func (s Stop) Offset() StopOffset {
	return s.offset
}

// Color is Stop::color in usvg src/tree/mod.rs. It returns the gradient stop
// color.
//
// `stop-color` in SVG.
func (s Stop) Color() Color {
	return s.color
}

// Opacity is Stop::opacity in usvg src/tree/mod.rs. It returns the gradient
// stop opacity.
//
// `stop-opacity` in SVG.
func (s Stop) Opacity() Opacity {
	return s.opacity
}

// Pattern is Pattern in usvg src/tree/mod.rs. It is a pattern element.
//
// `pattern` element in SVG.
type Pattern struct {
	id           nonEmptyString
	units        units // used only during parsing
	contentUnits units // used only during parsing
	transform    Transform
	rect         NonZeroRect
	viewBox      *viewBox
	root         Group
}

// ID is Pattern::id in usvg src/tree/mod.rs. It returns the element's ID.
//
// Taken from the SVG itself.
// Used only during SVG writing. `resvg` doesn't rely on this property.
func (p *Pattern) ID() string {
	return p.id.get()
}

// Transform is Pattern::transform in usvg src/tree/mod.rs. It returns the
// pattern transform.
//
// `patternTransform` in SVG.
func (p *Pattern) Transform() Transform {
	return p.transform
}

// Rect is Pattern::rect in usvg src/tree/mod.rs. It returns the pattern
// rectangle.
//
// `x`, `y`, `width` and `height` in SVG.
func (p *Pattern) Rect() NonZeroRect {
	return p.rect
}

// Root is Pattern::root in usvg src/tree/mod.rs. It returns the pattern
// children.
func (p *Pattern) Root() *Group {
	return &p.root
}

// StrokeWidth is StrokeWidth in usvg src/tree/mod.rs. It is an alias to
// `NonZeroPositiveF32`.
type StrokeWidth = path.NonZeroPositiveF32

// StrokeMiterlimit is StrokeMiterlimit in usvg src/tree/mod.rs. It is a
// `stroke-miterlimit` value.
//
// Just like `f32` but immutable and guarantee to be >=1.0.
//
// Port: the impl PartialEq compares two values within 4 units of least
// precision. Go compares them with Eq, not with ==.
type StrokeMiterlimit float32

// NewStrokeMiterlimit is StrokeMiterlimit::new in usvg src/tree/mod.rs. It
// creates a new `StrokeMiterlimit` value.
//
// Port: upstream checks with debug_assert! that n is finite and >= 1. The
// release build does not check, and Go does not check either.
func NewStrokeMiterlimit(n float32) StrokeMiterlimit {
	if !(n >= 1.0) {
		n = 1.0
	}

	return StrokeMiterlimit(n)
}

// Get is StrokeMiterlimit::get in usvg src/tree/mod.rs. It returns an
// underlying value.
func (m StrokeMiterlimit) Get() float32 {
	return float32(m)
}

// DefaultStrokeMiterlimit is the impl Default for StrokeMiterlimit in usvg
// src/tree/mod.rs.
func DefaultStrokeMiterlimit() StrokeMiterlimit {
	return NewStrokeMiterlimit(4.0)
}

// Eq is the impl PartialEq for StrokeMiterlimit in usvg src/tree/mod.rs.
func (m StrokeMiterlimit) Eq(other StrokeMiterlimit) bool {
	return ApproxEqUlps32(float32(m), float32(other), 4)
}

// LineCap is LineCap in usvg src/tree/mod.rs. It is a line cap.
//
// `stroke-linecap` attribute in the SVG.
//
// The zero value is LineCapButt, the Default of upstream.
type LineCap uint8

// The values of LineCap.
const (
	LineCapButt LineCap = iota
	LineCapRound
	LineCapSquare
)

// String returns the Rust name of the value.
func (c LineCap) String() string {
	switch c {
	case LineCapButt:
		return "Butt"
	case LineCapRound:
		return "Round"
	case LineCapSquare:
		return "Square"
	}
	return "LineCap(?)"
}

// LineJoin is LineJoin in usvg src/tree/mod.rs. It is a line join.
//
// `stroke-linejoin` attribute in the SVG.
//
// The zero value is LineJoinMiter, the Default of upstream.
type LineJoin uint8

// The values of LineJoin.
const (
	LineJoinMiter LineJoin = iota
	LineJoinMiterClip
	LineJoinRound
	LineJoinBevel
)

// String returns the Rust name of the value.
func (j LineJoin) String() string {
	switch j {
	case LineJoinMiter:
		return "Miter"
	case LineJoinMiterClip:
		return "MiterClip"
	case LineJoinRound:
		return "Round"
	case LineJoinBevel:
		return "Bevel"
	}
	return "LineJoin(?)"
}

// Stroke is Stroke in usvg src/tree/mod.rs. It is a stroke style.
type Stroke struct {
	paint      Paint
	dasharray  []float32 // nil is None
	dashoffset float32
	miterlimit StrokeMiterlimit
	opacity    Opacity
	width      StrokeWidth
	linecap    LineCap
	linejoin   LineJoin
	// Whether the current stroke needs to be resolved relative
	// to a context element.
	contextElement *contextElement
}

// Paint is Stroke::paint in usvg src/tree/mod.rs. It returns the stroke
// paint.
func (s *Stroke) Paint() Paint {
	return s.paint
}

// Dasharray is Stroke::dasharray in usvg src/tree/mod.rs. It returns the
// stroke dash array.
func (s *Stroke) Dasharray() ([]float32, bool) {
	return s.dasharray, s.dasharray != nil
}

// Dashoffset is Stroke::dashoffset in usvg src/tree/mod.rs. It returns the
// stroke dash offset.
func (s *Stroke) Dashoffset() float32 {
	return s.dashoffset
}

// Miterlimit is Stroke::miterlimit in usvg src/tree/mod.rs. It returns the
// stroke miter limit.
func (s *Stroke) Miterlimit() StrokeMiterlimit {
	return s.miterlimit
}

// Opacity is Stroke::opacity in usvg src/tree/mod.rs. It returns the stroke
// opacity.
func (s *Stroke) Opacity() Opacity {
	return s.opacity
}

// Width is Stroke::width in usvg src/tree/mod.rs. It returns the stroke
// width.
func (s *Stroke) Width() StrokeWidth {
	return s.width
}

// Linecap is Stroke::linecap in usvg src/tree/mod.rs. It returns the stroke
// linecap.
func (s *Stroke) Linecap() LineCap {
	return s.linecap
}

// Linejoin is Stroke::linejoin in usvg src/tree/mod.rs. It returns the
// stroke linejoin.
func (s *Stroke) Linejoin() LineJoin {
	return s.linejoin
}

// ToTinySkia is Stroke::to_tiny_skia in usvg src/tree/mod.rs. It converts
// into a `tiny_skia_path::Stroke` type.
func (s *Stroke) ToTinySkia() path.Stroke {
	stroke := path.Stroke{
		Width:      s.width.Get(),
		MiterLimit: s.miterlimit.Get(),
		LineCap:    path.LineCapButt,
		LineJoin:   path.LineJoinMiter,
		// According to the spec, dash should not be accounted during
		// bbox calculation.
		Dash: nil,
	}
	switch s.linecap {
	case LineCapButt:
		stroke.LineCap = path.LineCapButt
	case LineCapRound:
		stroke.LineCap = path.LineCapRound
	case LineCapSquare:
		stroke.LineCap = path.LineCapSquare
	}
	switch s.linejoin {
	case LineJoinMiter:
		stroke.LineJoin = path.LineJoinMiter
	case LineJoinMiterClip:
		stroke.LineJoin = path.LineJoinMiterClip
	case LineJoinRound:
		stroke.LineJoin = path.LineJoinRound
	case LineJoinBevel:
		stroke.LineJoin = path.LineJoinBevel
	}

	if s.dasharray != nil {
		if dash, ok := path.NewStrokeDash(slices.Clone(s.dasharray), s.dashoffset); ok {
			stroke.Dash = dash
		}
	}

	return stroke
}

// FillRule is FillRule in usvg src/tree/mod.rs. It is a fill rule.
//
// `fill-rule` attribute in the SVG.
//
// The zero value is FillRuleNonZero, the Default of upstream.
type FillRule uint8

// The values of FillRule.
const (
	FillRuleNonZero FillRule = iota
	FillRuleEvenOdd
)

// String returns the Rust name of the value.
func (r FillRule) String() string {
	switch r {
	case FillRuleNonZero:
		return "NonZero"
	case FillRuleEvenOdd:
		return "EvenOdd"
	}
	return "FillRule(?)"
}

// contextElementKind is the case of a contextElement.
type contextElementKind uint8

// The cases of the enum ContextElement in usvg src/tree/mod.rs.
const (
	// contextElementUseNode is ContextElement::UseNode.
	//
	// The current context element is a use node. Since we can get
	// the bounding box of a use node only once we have converted
	// all elements, we need to fix the transform and units of
	// the stroke/fill after converting the whole tree.
	contextElementUseNode contextElementKind = iota
	// contextElementPathNode is ContextElement::PathNode.
	//
	// The current context element is a path node (i.e. only applicable
	// if we draw the marker of a path). Since we already know the bounding
	// box of the path when rendering the markers, we can convert them directly,
	// so we do it while parsing.
	contextElementPathNode
)

// contextElement is ContextElement in usvg src/tree/mod.rs.
//
// Port: the enum holds no data. It stays a struct with a kind, so that a
// nil *contextElement is None, as in the fields of Fill and Stroke.
type contextElement struct {
	kind contextElementKind
}

// Fill is Fill in usvg src/tree/mod.rs. It is a fill style.
type Fill struct {
	paint   Paint
	opacity Opacity
	rule    FillRule
	// Whether the current fill needs to be resolved relative
	// to a context element.
	contextElement *contextElement
}

// Paint is Fill::paint in usvg src/tree/mod.rs. It returns the fill paint.
func (f *Fill) Paint() Paint {
	return f.paint
}

// Opacity is Fill::opacity in usvg src/tree/mod.rs. It returns the fill
// opacity.
func (f *Fill) Opacity() Opacity {
	return f.opacity
}

// Rule is Fill::rule in usvg src/tree/mod.rs. It returns the fill rule.
func (f *Fill) Rule() FillRule {
	return f.rule
}

// DefaultFill is the impl Default for Fill in usvg src/tree/mod.rs.
func DefaultFill() Fill {
	return Fill{
		paint:          ColorBlack(),
		opacity:        path.NormalizedF32One,
		rule:           FillRuleNonZero,
		contextElement: nil,
	}
}

// Color is Color in usvg src/tree/mod.rs. It is a 8-bit RGB color.
type Color struct {
	Red   uint8
	Green uint8
	Blue  uint8
}

// NewColorRGB is Color::new_rgb in usvg src/tree/mod.rs. It constructs a new
// `Color` from RGB values.
func NewColorRGB(red, green, blue uint8) Color {
	return Color{Red: red, Green: green, Blue: blue}
}

// ColorBlack is Color::black in usvg src/tree/mod.rs. It constructs a new
// `Color` set to black.
func ColorBlack() Color {
	return NewColorRGB(0, 0, 0)
}

// ColorWhite is Color::white in usvg src/tree/mod.rs. It constructs a new
// `Color` set to white.
func ColorWhite() Color {
	return NewColorRGB(255, 255, 255)
}

// Paint is Paint in usvg src/tree/mod.rs. It is a paint style.
//
// `paint` value type in the SVG.
//
// Port: the cases hold different data, so the enum becomes an interface
// (D27). Its cases are the types Color, *LinearGradient, *RadialGradient and
// *Pattern, and a type switch takes the place of the Rust match. The ==
// operator on two Paint values is the impl PartialEq of upstream: it compares
// two colors by value, and two gradients or two patterns by pointer, as
// Arc::ptr_eq does.
type Paint interface {
	isPaint()
}

// isPaint makes Color a case of Paint.
func (Color) isPaint() {}

// isPaint makes *LinearGradient a case of Paint.
func (*LinearGradient) isPaint() {}

// isPaint makes *RadialGradient a case of Paint.
func (*RadialGradient) isPaint() {}

// isPaint makes *Pattern a case of Paint.
func (*Pattern) isPaint() {}

// ClipPath is ClipPath in usvg src/tree/mod.rs. It is a clip-path element.
//
// `clipPath` element in SVG.
type ClipPath struct {
	id        nonEmptyString
	transform Transform
	clipPath  *ClipPath
	root      Group
}

// emptyClipPath is ClipPath::empty in usvg src/tree/mod.rs.
func emptyClipPath(id nonEmptyString) *ClipPath {
	return &ClipPath{
		id:        id,
		transform: path.DefaultTransform(),
		clipPath:  nil,
		root:      emptyGroupValue(),
	}
}

// ID is ClipPath::id in usvg src/tree/mod.rs. It returns the element's ID.
//
// Taken from the SVG itself.
// Used only during SVG writing. `resvg` doesn't rely on this property.
func (c *ClipPath) ID() string {
	return c.id.get()
}

// Transform is ClipPath::transform in usvg src/tree/mod.rs. It returns the
// clip path transform.
//
// `transform` in SVG.
func (c *ClipPath) Transform() Transform {
	return c.transform
}

// ClipPath is ClipPath::clip_path in usvg src/tree/mod.rs. It returns the
// additional clip path.
//
// `clip-path` in SVG.
func (c *ClipPath) ClipPath() (*ClipPath, bool) {
	return c.clipPath, c.clipPath != nil
}

// Root is ClipPath::root in usvg src/tree/mod.rs. It returns the clip path
// children.
func (c *ClipPath) Root() *Group {
	return &c.root
}

// MaskType is MaskType in usvg src/tree/mod.rs. It is a mask type.
//
// The zero value is MaskTypeLuminance, the Default of upstream.
type MaskType uint8

// The values of MaskType.
const (
	// MaskTypeLuminance indicates that the luminance values of the mask
	// should be used.
	MaskTypeLuminance MaskType = iota
	// MaskTypeAlpha indicates that the alpha values of the mask should be
	// used.
	MaskTypeAlpha
)

// String returns the Rust name of the value.
func (t MaskType) String() string {
	switch t {
	case MaskTypeLuminance:
		return "Luminance"
	case MaskTypeAlpha:
		return "Alpha"
	}
	return "MaskType(?)"
}

// Mask is Mask in usvg src/tree/mod.rs. It is a mask element.
//
// `mask` element in SVG.
type Mask struct {
	id   nonEmptyString
	rect NonZeroRect
	kind MaskType
	mask *Mask
	root Group
}

// ID is Mask::id in usvg src/tree/mod.rs. It returns the element's ID.
//
// Taken from the SVG itself.
// Used only during SVG writing. `resvg` doesn't rely on this property.
func (m *Mask) ID() string {
	return m.id.get()
}

// Rect is Mask::rect in usvg src/tree/mod.rs. It returns the mask rectangle.
//
// `x`, `y`, `width` and `height` in SVG.
func (m *Mask) Rect() NonZeroRect {
	return m.rect
}

// Kind is Mask::kind in usvg src/tree/mod.rs. It returns the mask type.
//
// `mask-type` in SVG.
func (m *Mask) Kind() MaskType {
	return m.kind
}

// Mask is Mask::mask in usvg src/tree/mod.rs. It returns the additional
// mask.
//
// `mask` in SVG.
func (m *Mask) Mask() (*Mask, bool) {
	return m.mask, m.mask != nil
}

// Root is Mask::root in usvg src/tree/mod.rs. It returns the mask children.
//
// A mask can have no children, in which case the whole element should be masked out.
func (m *Mask) Root() *Group {
	return &m.root
}

// Node is Node in usvg src/tree/mod.rs. It is the node's kind.
//
// Port: the cases hold different data, and the code calls methods on each,
// so the enum becomes an interface (D27). Its cases are *Group, *Path, *Image
// and *Text, as the Box of each case in upstream, and a type switch takes the
// place of the Rust match. The methods of the interface are the methods of
// Node in upstream. Node::bounding_box, Node::abs_bounding_box,
// Node::stroke_bounding_box, Node::abs_stroke_bounding_box and
// Node::abs_layer_bounding_box become the functions NodeBoundingBox,
// NodeAbsBoundingBox, NodeStrokeBoundingBox, NodeAbsStrokeBoundingBox and
// NodeAbsLayerBoundingBox, because a case has a method of each name with
// another result.
type Node interface {
	// ID is Node::id in usvg src/tree/mod.rs. It returns node's ID.
	ID() string

	// AbsTransform is Node::abs_transform in usvg src/tree/mod.rs. It
	// returns node's absolute transform.
	//
	// This method is cheap since absolute transforms are already resolved.
	AbsTransform() Transform

	// Subroots is Node::subroots in usvg src/tree/mod.rs. It calls a
	// closure for each subroot this `Node` has.
	//
	// The [`Tree::root`](Tree::root) field contain only render-able SVG elements.
	// But some elements, specifically clip paths, masks, patterns and feImage
	// can store their own SVG subtrees.
	// And while one can access them manually, it's pretty verbose.
	// This methods allows looping over _all_ SVG elements present in the `Tree`.
	//
	// # Example
	//
	//	func allNodes(parent *svg.Group) {
	//		for _, node := range parent.Children() {
	//			// do stuff...
	//
	//			if g, ok := node.(*svg.Group); ok {
	//				allNodes(g)
	//			}
	//
	//			// handle subroots as well
	//			node.Subroots(allNodes)
	//		}
	//	}
	Subroots(f func(*Group))

	isNode()
}

// isNode makes *Group a case of Node.
func (*Group) isNode() {}

// isNode makes *Path a case of Node.
func (*Path) isNode() {}

// isNode makes *Image a case of Node.
func (*Image) isNode() {}

// isNode makes *Text a case of Node.
func (*Text) isNode() {}

// NodeBoundingBox is Node::bounding_box in usvg src/tree/mod.rs. It returns
// node's bounding box in object coordinates, if any.
func NodeBoundingBox(node Node) (Rect, bool) {
	switch n := node.(type) {
	case *Group:
		return n.BoundingBox()
	case *Path:
		return n.BoundingBox()
	case *Image:
		return n.BoundingBox(), true
	case *Text:
		return n.BoundingBox(), true
	}
	return Rect{}, false
}

// NodeAbsBoundingBox is Node::abs_bounding_box in usvg src/tree/mod.rs. It
// returns node's bounding box in canvas coordinates, if any.
func NodeAbsBoundingBox(node Node) (Rect, bool) {
	switch n := node.(type) {
	case *Group:
		return n.AbsBoundingBox()
	case *Path:
		return n.AbsBoundingBox()
	case *Image:
		return n.AbsBoundingBox(), true
	case *Text:
		return n.AbsBoundingBox(), true
	}
	return Rect{}, false
}

// NodeStrokeBoundingBox is Node::stroke_bounding_box in usvg
// src/tree/mod.rs. It returns node's bounding box, including stroke, in
// object coordinates, if any.
func NodeStrokeBoundingBox(node Node) (Rect, bool) {
	switch n := node.(type) {
	case *Group:
		return n.StrokeBoundingBox()
	case *Path:
		return n.StrokeBoundingBox()
	// Image cannot be stroked.
	case *Image:
		return n.BoundingBox(), true
	case *Text:
		return n.StrokeBoundingBox(), true
	}
	return Rect{}, false
}

// NodeAbsStrokeBoundingBox is Node::abs_stroke_bounding_box in usvg
// src/tree/mod.rs. It returns node's bounding box, including stroke, in
// canvas coordinates, if any.
func NodeAbsStrokeBoundingBox(node Node) (Rect, bool) {
	switch n := node.(type) {
	case *Group:
		return n.AbsStrokeBoundingBox()
	case *Path:
		return n.AbsStrokeBoundingBox()
	// Image cannot be stroked.
	case *Image:
		return n.AbsBoundingBox(), true
	case *Text:
		return n.AbsStrokeBoundingBox(), true
	}
	return Rect{}, false
}

// NodeAbsLayerBoundingBox is Node::abs_layer_bounding_box in usvg
// src/tree/mod.rs. It returns the element's "layer" bounding box in canvas
// units, if any.
//
// For most nodes this is just `abs_bounding_box`,
// but for groups this is `abs_layer_bounding_box`.
//
// See [`Group::layer_bounding_box`] for details.
func NodeAbsLayerBoundingBox(node Node) (NonZeroRect, bool) {
	switch n := node.(type) {
	case *Group:
		return n.AbsLayerBoundingBox()
	// Hor/ver path without stroke can return None. This is expected.
	case *Path:
		r, ok := n.AbsBoundingBox()
		if !ok {
			return NonZeroRect{}, false
		}
		return r.ToNonZeroRect()
	case *Image:
		return n.AbsBoundingBox().ToNonZeroRect()
	case *Text:
		return n.AbsBoundingBox().ToNonZeroRect()
	}
	return NonZeroRect{}, false
}

// nodeToParentCoordinates is Node::to_parent_coordinates in usvg
// src/tree/mod.rs. It maps a rect in this node's object coordinates into its
// parent's coordinates.
func nodeToParentCoordinates(node Node, rect Rect) Rect {
	if group, ok := node.(*Group); ok {
		if r, ok := rect.Transform(group.transform); ok {
			return r
		}
	}
	return rect
}

// Group is Group in usvg src/tree/mod.rs. It is a group container.
//
// The preprocessor will remove all groups that don't impact rendering.
// Those that left is just an indicator that a new canvas should be created.
//
// `g` element in SVG.
type Group struct {
	id           string
	transform    Transform
	absTransform Transform
	opacity      Opacity
	blendMode    BlendMode
	isolate      bool
	clipPath     *ClipPath
	// Whether the group is a context element (i.e. a use node)
	isContextElement bool
	mask             *Mask
	filters          []*Filter
	// Calculated from `children` on first access and cached, so they must
	// only be accessed once the group is complete.
	boundingBox          onceLock[optional[Rect]]
	absBoundingBox       onceLock[optional[Rect]]
	strokeBoundingBox    onceLock[optional[Rect]]
	absStrokeBoundingBox onceLock[optional[Rect]]
	layerBoundingBox     onceLock[optional[NonZeroRect]]
	absLayerBoundingBox  onceLock[optional[NonZeroRect]]
	children             []Node
}

// cloneGroup is the derived Clone for Group in usvg src/tree/mod.rs.
//
// Port: the children are copied, with a copy of each fill and stroke, as the
// Box of each Node and the fields of each Path are in upstream. A clip path,
// a mask, a filter, a paint server and the tree of an SVG image stay shared,
// as an Arc is. Upstream holds the Tree of an SVG image by value, and Go
// shares it, because nothing changes it after a clone.
func (g *Group) cloneGroup() *Group {
	c := new(Group)
	g.cloneGroupInto(c)
	return c
}

// cloneGroupInto is cloneGroup for a destination that the caller holds,
// such as the root of a pattern.
//
// Port: the caches of the bounding boxes hold a lock, so a Group is not
// copied by value. Each field and each set cache is copied by name.
func (g *Group) cloneGroupInto(c *Group) {
	g.moveInto(c)
	c.filters = slices.Clone(g.filters)
	c.children = make([]Node, len(g.children))
	for i, child := range g.children {
		switch n := child.(type) {
		case *Group:
			c.children[i] = n.cloneGroup()
		case *Path:
			np := n.clone()
			if n.fill != nil {
				f := *n.fill
				np.fill = &f
			}
			if n.stroke != nil {
				s := *n.stroke
				np.stroke = &s
			}
			c.children[i] = np
		case *Image:
			ni := *n
			c.children[i] = &ni
		case *Text:
			c.children[i] = n.clone()
		default:
			c.children[i] = child
		}
	}
}

// moveInto sets each field of c to the field of g, and each cache of c to
// the cache of g that is set. The filters and the children stay shared.
//
// Port: Rust moves a Group by value. Go copies its fields by name, because
// the caches of the bounding boxes hold a lock.
func (g *Group) moveInto(c *Group) {
	c.id = g.id
	c.transform = g.transform
	c.absTransform = g.absTransform
	c.opacity = g.opacity
	c.blendMode = g.blendMode
	c.isolate = g.isolate
	c.clipPath = g.clipPath
	c.isContextElement = g.isContextElement
	c.mask = g.mask
	c.filters = g.filters
	c.children = g.children
	g.boundingBox.cloneInto(&c.boundingBox)
	g.absBoundingBox.cloneInto(&c.absBoundingBox)
	g.strokeBoundingBox.cloneInto(&c.strokeBoundingBox)
	g.absStrokeBoundingBox.cloneInto(&c.absStrokeBoundingBox)
	g.layerBoundingBox.cloneInto(&c.layerBoundingBox)
	g.absLayerBoundingBox.cloneInto(&c.absLayerBoundingBox)
}

// emptyGroup is Group::empty in usvg src/tree/mod.rs.
//
// Port: it returns a pointer, which most callers need. emptyGroupValue
// returns the value, for a field that holds a Group.
func emptyGroup() *Group {
	g := emptyGroupValue()
	return &g
}

// emptyGroupValue is Group::empty in usvg src/tree/mod.rs. It returns the
// value.
func emptyGroupValue() Group {
	return Group{
		id:               "",
		transform:        path.DefaultTransform(),
		absTransform:     path.DefaultTransform(),
		opacity:          path.NormalizedF32One,
		blendMode:        BlendModeNormal,
		isolate:          false,
		clipPath:         nil,
		mask:             nil,
		filters:          nil,
		isContextElement: false,
		children:         nil,
	}
}

// ID is Group::id in usvg src/tree/mod.rs. It returns the element's ID.
//
// Taken from the SVG itself.
// Isn't automatically generated.
// Can be empty.
func (g *Group) ID() string {
	return g.id
}

// Transform is Group::transform in usvg src/tree/mod.rs. It returns the
// element's transform.
//
// This is a relative transform. The one that is set via the `transform` attribute in SVG.
func (g *Group) Transform() Transform {
	return g.transform
}

// AbsTransform is Group::abs_transform in usvg src/tree/mod.rs. It returns
// the element's absolute transform.
//
// Contains all ancestors transforms including group's transform.
//
// Note that subroots, like clipPaths, masks and patterns, have their own root transform,
// which isn't affected by the node that references this subroot.
func (g *Group) AbsTransform() Transform {
	return g.absTransform
}

// Opacity is Group::opacity in usvg src/tree/mod.rs. It returns the group
// opacity.
//
// After the group is rendered we should combine
// it with a parent group using the specified opacity.
func (g *Group) Opacity() Opacity {
	return g.opacity
}

// BlendMode is Group::blend_mode in usvg src/tree/mod.rs. It returns the
// group blend mode.
//
// `mix-blend-mode` in SVG.
func (g *Group) BlendMode() BlendMode {
	return g.blendMode
}

// Isolate is Group::isolate in usvg src/tree/mod.rs. It returns the group
// isolation.
//
// `isolation` in SVG.
func (g *Group) Isolate() bool {
	return g.isolate
}

// ClipPath is Group::clip_path in usvg src/tree/mod.rs. It returns the
// element's clip path.
func (g *Group) ClipPath() (*ClipPath, bool) {
	return g.clipPath, g.clipPath != nil
}

// Mask is Group::mask in usvg src/tree/mod.rs. It returns the element's
// mask.
func (g *Group) Mask() (*Mask, bool) {
	return g.mask, g.mask != nil
}

// Filters is Group::filters in usvg src/tree/mod.rs. It returns the
// element's filters.
func (g *Group) Filters() []*Filter {
	return g.filters
}

// BoundingBox is Group::bounding_box in usvg src/tree/mod.rs. It returns the
// element's object bounding box.
//
// `objectBoundingBox` in SVG terms. Meaning it doesn't affected by parent transforms.
//
// Can be set to `None` in case of an empty group.
func (g *Group) BoundingBox() (Rect, bool) {
	return g.boundingBox.getOrInit(func() optional[Rect] {
		return optionalOf(g.unionOfChildren(func(child Node) (Rect, bool) {
			r, ok := NodeBoundingBox(child)
			if !ok {
				return Rect{}, false
			}
			return nodeToParentCoordinates(child, r), true
		}))
	}).get()
}

// AbsBoundingBox is Group::abs_bounding_box in usvg src/tree/mod.rs. It
// returns the element's bounding box in canvas coordinates.
//
// `userSpaceOnUse` in SVG terms.
func (g *Group) AbsBoundingBox() (Rect, bool) {
	return g.absBoundingBox.getOrInit(func() optional[Rect] {
		return optionalOf(g.unionOfChildren(NodeAbsBoundingBox))
	}).get()
}

// StrokeBoundingBox is Group::stroke_bounding_box in usvg src/tree/mod.rs. It
// returns the element's object bounding box including stroke.
//
// Similar to `bounding_box`, but includes stroke.
func (g *Group) StrokeBoundingBox() (Rect, bool) {
	return g.strokeBoundingBox.getOrInit(func() optional[Rect] {
		return optionalOf(g.unionOfChildren(func(child Node) (Rect, bool) {
			r, ok := NodeStrokeBoundingBox(child)
			if !ok {
				return Rect{}, false
			}
			return nodeToParentCoordinates(child, r), true
		}))
	}).get()
}

// AbsStrokeBoundingBox is Group::abs_stroke_bounding_box in usvg
// src/tree/mod.rs. It returns the element's bounding box including stroke in
// user coordinates.
//
// Similar to `abs_bounding_box`, but includes stroke.
func (g *Group) AbsStrokeBoundingBox() (Rect, bool) {
	return g.absStrokeBoundingBox.getOrInit(func() optional[Rect] {
		return optionalOf(g.unionOfChildren(NodeAbsStrokeBoundingBox))
	}).get()
}

// LayerBoundingBox is Group::layer_bounding_box in usvg src/tree/mod.rs. It
// returns the element's "layer" bounding box in object units.
//
// Conceptually, this is `stroke_bounding_box` expanded and/or clipped
// by `filters_bounding_box`, but also including all the children.
// Groups without filters are additionally clipped by their clip path.
// This is the bounding box `resvg` will later use to allocate layers/pixmaps
// during isolated groups rendering.
//
// Only groups have it, because only groups can have filters.
// For other nodes layer bounding box is the same as stroke bounding box.
//
// Unlike other bounding boxes, cannot have zero size.
func (g *Group) LayerBoundingBox() (NonZeroRect, bool) {
	return g.layerBoundingBox.getOrInit(func() optional[NonZeroRect] {
		return optionalOf(g.calculateLayerBoundingBox())
	}).get()
}

// AbsLayerBoundingBox is Group::abs_layer_bounding_box in usvg
// src/tree/mod.rs. It returns the element's "layer" bounding box in canvas
// units.
func (g *Group) AbsLayerBoundingBox() (NonZeroRect, bool) {
	return g.absLayerBoundingBox.getOrInit(func() optional[NonZeroRect] {
		r, ok := g.LayerBoundingBox()
		if !ok {
			return optional[NonZeroRect]{}
		}
		return optionalOf(r.Transform(g.absTransform))
	}).get()
}

// Children is Group::children in usvg src/tree/mod.rs. It returns the
// group's children.
func (g *Group) Children() []Node {
	return g.children
}

// ShouldIsolate is Group::should_isolate in usvg src/tree/mod.rs. It checks
// if this group should be isolated during rendering.
func (g *Group) ShouldIsolate() bool {
	return g.isolate ||
		g.opacity != path.NormalizedF32One ||
		g.clipPath != nil ||
		g.mask != nil ||
		len(g.filters) != 0 ||
		g.blendMode != BlendModeNormal // TODO: probably not needed?
}

// HasChildren is Group::has_children in usvg src/tree/mod.rs. It returns
// `true` if the group has any children.
func (g *Group) HasChildren() bool {
	return len(g.children) != 0
}

// FiltersBoundingBox is Group::filters_bounding_box in usvg src/tree/mod.rs.
// It calculates a node's filter bounding box.
//
// Filters with `objectBoundingBox` and missing or zero `bounding_box` would be ignored.
//
// Note that a filter region can act like a clipping rectangle,
// therefore this function can produce a bounding box smaller than `bounding_box`.
//
// Returns `None` when then group has no filters.
//
// This function is very fast, that's why we do not store this bbox as a `Group` field.
func (g *Group) FiltersBoundingBox() (NonZeroRect, bool) {
	fullRegion := defaultBBox()
	for _, filter := range g.filters {
		fullRegion = fullRegion.expand(bboxFromNonZeroRect(filter.rect))
	}

	return fullRegion.toNonZeroRect()
}

// Subroots is the private Group::subroots in usvg src/tree/mod.rs, which
// Node::subroots calls for a group.
func (g *Group) Subroots(f func(*Group)) {
	if g.clipPath != nil {
		clip := g.clipPath
		f(&clip.root)

		if clip.clipPath != nil {
			f(&clip.clipPath.root)
		}
	}

	if g.mask != nil {
		mask := g.mask
		f(&mask.root)

		if mask.mask != nil {
			f(&mask.mask.root)
		}
	}

	for _, filter := range g.filters {
		for i := range filter.primitives {
			if image, ok := filter.primitives[i].kind.(*ImageFilter); ok {
				f(image.Root())
			}
		}
	}
}

// PaintOrder is PaintOrder in usvg src/tree/mod.rs. It is the representation
// of the [`paint-order`] property.
//
// `usvg` will handle `markers` automatically,
// therefore we provide only `fill` and `stroke` variants.
//
// The zero value is PaintOrderFillAndStroke, the Default of upstream.
//
// [`paint-order`]: https://www.w3.org/TR/SVG2/painting.html#PaintOrder
type PaintOrder uint8

// The values of PaintOrder.
const (
	PaintOrderFillAndStroke PaintOrder = iota
	PaintOrderStrokeAndFill
)

// String returns the Rust name of the value.
func (o PaintOrder) String() string {
	switch o {
	case PaintOrderFillAndStroke:
		return "FillAndStroke"
	case PaintOrderStrokeAndFill:
		return "StrokeAndFill"
	}
	return "PaintOrder(?)"
}

// Path is Path in usvg src/tree/mod.rs. It is a path element.
type Path struct {
	id                   string
	visible              bool
	fill                 *Fill
	stroke               *Stroke
	paintOrder           PaintOrder
	renderingMode        ShapeRendering
	data                 *path.Path
	absTransform         Transform
	boundingBox          onceLock[optional[Rect]]
	absBoundingBox       onceLock[optional[Rect]]
	strokeBoundingBox    onceLock[optional[Rect]]
	absStrokeBoundingBox onceLock[optional[Rect]]
}

// newSimplePath is Path::new_simple in usvg src/tree/mod.rs.
func newSimplePath(data *path.Path) *Path {
	return newPath(
		"",
		true,
		nil,
		nil,
		PaintOrderFillAndStroke,
		DefaultShapeRendering(),
		data,
		path.DefaultTransform(),
	)
}

// newPath is Path::new in usvg src/tree/mod.rs.
func newPath(
	id string,
	visible bool,
	fill *Fill,
	stroke *Stroke,
	paintOrder PaintOrder,
	renderingMode ShapeRendering,
	data *path.Path,
	absTransform Transform,
) *Path {
	return &Path{
		id:            id,
		visible:       visible,
		fill:          fill,
		stroke:        stroke,
		paintOrder:    paintOrder,
		renderingMode: renderingMode,
		data:          data,
		absTransform:  absTransform,
	}
}

// clone is the derived Clone for Path in usvg src/tree/mod.rs.
//
// Port: the caches of the bounding boxes hold a lock, so the clone copies
// each field and each set cache by name. The fill and the stroke stay
// shared, and the caller copies them where it needs to.
func (p *Path) clone() *Path {
	c := newPath(p.id, p.visible, p.fill, p.stroke, p.paintOrder, p.renderingMode, p.data, p.absTransform)
	p.boundingBox.cloneInto(&c.boundingBox)
	p.absBoundingBox.cloneInto(&c.absBoundingBox)
	p.strokeBoundingBox.cloneInto(&c.strokeBoundingBox)
	p.absStrokeBoundingBox.cloneInto(&c.absStrokeBoundingBox)
	return c
}

// ID is Path::id in usvg src/tree/mod.rs. It returns the element's ID.
//
// Taken from the SVG itself.
// Isn't automatically generated.
// Can be empty.
func (p *Path) ID() string {
	return p.id
}

// IsVisible is Path::is_visible in usvg src/tree/mod.rs. It returns the
// element visibility.
func (p *Path) IsVisible() bool {
	return p.visible
}

// Fill is Path::fill in usvg src/tree/mod.rs. It returns the fill style.
func (p *Path) Fill() (*Fill, bool) {
	return p.fill, p.fill != nil
}

// Stroke is Path::stroke in usvg src/tree/mod.rs. It returns the stroke
// style.
func (p *Path) Stroke() (*Stroke, bool) {
	return p.stroke, p.stroke != nil
}

// PaintOrder is Path::paint_order in usvg src/tree/mod.rs. It returns the
// fill and stroke paint order.
//
// Since markers will be replaced with regular nodes automatically,
// `usvg` doesn't provide the `markers` order type. It's was already done.
//
// `paint-order` in SVG.
func (p *Path) PaintOrder() PaintOrder {
	return p.paintOrder
}

// RenderingMode is Path::rendering_mode in usvg src/tree/mod.rs. It returns
// the rendering mode.
//
// `shape-rendering` in SVG.
func (p *Path) RenderingMode() ShapeRendering {
	return p.renderingMode
}

// Data is Path::data in usvg src/tree/mod.rs. It returns the segments list.
//
// All segments are in absolute coordinates.
func (p *Path) Data() *path.Path {
	// TODO: find a better name
	return p.data
}

// AbsTransform is Path::abs_transform in usvg src/tree/mod.rs. It returns the
// element's absolute transform.
//
// Contains all ancestors transforms including elements's transform.
//
// Note that this is not the relative transform present in SVG.
// The SVG one would be set only on groups.
func (p *Path) AbsTransform() Transform {
	return p.absTransform
}

// BoundingBox is Path::bounding_box in usvg src/tree/mod.rs. It returns the
// element's object bounding box.
//
// `objectBoundingBox` in SVG terms. Meaning it doesn't affected by parent transforms.
func (p *Path) BoundingBox() (Rect, bool) {
	return p.boundingBox.getOrInit(func() optional[Rect] {
		return optionalOf(p.data.ComputeTightBounds())
	}).get()
}

// AbsBoundingBox is Path::abs_bounding_box in usvg src/tree/mod.rs. It
// returns the element's bounding box in canvas coordinates.
//
// `userSpaceOnUse` in SVG terms.
func (p *Path) AbsBoundingBox() (Rect, bool) {
	return p.absBoundingBox.getOrInit(func() optional[Rect] {
		if p.absTransform.HasSkew() {
			data, ok := p.absData()
			if !ok {
				return optional[Rect]{}
			}
			return optionalOf(data.ComputeTightBounds())
		}
		// A transform without a skew can be performed just on a bbox.
		r, ok := p.BoundingBox()
		if !ok {
			return optional[Rect]{}
		}
		return optionalOf(r.Transform(p.absTransform))
	}).get()
}

// StrokeBoundingBox is Path::stroke_bounding_box in usvg src/tree/mod.rs. It
// returns the element's object bounding box including stroke.
//
// Will have the same value as `bounding_box` when path has no stroke.
func (p *Path) StrokeBoundingBox() (Rect, bool) {
	return p.strokeBoundingBox.getOrInit(func() optional[Rect] {
		if r, ok := calculateStrokeBBox(p.stroke, p.data); ok {
			return some(r)
		}
		return optionalOf(p.BoundingBox())
	}).get()
}

// AbsStrokeBoundingBox is Path::abs_stroke_bounding_box in usvg
// src/tree/mod.rs. It returns the element's bounding box including stroke in
// canvas coordinates.
//
// Will have the same value as `abs_bounding_box` when path has no stroke.
func (p *Path) AbsStrokeBoundingBox() (Rect, bool) {
	return p.absStrokeBoundingBox.getOrInit(func() optional[Rect] {
		switch {
		case p.stroke == nil:
			return optionalOf(p.AbsBoundingBox())
		case p.absTransform.HasSkew():
			data, ok := p.absData()
			if !ok {
				return optional[Rect]{}
			}
			if r, ok := calculateStrokeBBox(p.stroke, data); ok {
				return some(r)
			}
			return optionalOf(p.AbsBoundingBox())
		default:
			r, ok := p.StrokeBoundingBox()
			if !ok {
				return optional[Rect]{}
			}
			return optionalOf(r.Transform(p.absTransform))
		}
	}).get()
}

// absData is Path::abs_data in usvg src/tree/mod.rs.
func (p *Path) absData() (*path.Path, bool) {
	return p.data.Clone().Transform(p.absTransform)
}

// calculateStrokeBBox is Path::calculate_stroke_bbox in usvg
// src/tree/mod.rs.
func calculateStrokeBBox(s *Stroke, path *path.Path) (Rect, bool) {
	if s == nil {
		return Rect{}, false
	}
	stroke := s.ToTinySkia()
	// According to the spec, dash should not be accounted during bbox calculation.
	stroke.Dash = nil

	// TODO: avoid for round and bevel caps

	// Expensive, but there is not much we can do about it.
	if strokedPath, ok := path.Stroke(&stroke, 1.0); ok {
		return strokedPath.ComputeTightBounds()
	}

	return Rect{}, false
}

// Subroots is the private Path::subroots in usvg src/tree/mod.rs, which
// Node::subroots calls for a path.
func (p *Path) Subroots(f func(*Group)) {
	if p.fill != nil {
		if patt, ok := p.fill.paint.(*Pattern); ok {
			f(patt.Root())
		}
	}
	if p.stroke != nil {
		if patt, ok := p.stroke.paint.(*Pattern); ok {
			f(patt.Root())
		}
	}
}

// ImageKind is ImageKind in usvg src/tree/mod.rs. It is an embedded image
// kind.
//
// Port: the cases hold different data, so the enum becomes an interface
// (D27). Its cases are ImageKindJPEG, ImageKindPNG, ImageKindGIF,
// ImageKindWEBP and ImageKindSVG. The Arc<Vec<u8>> of upstream becomes a
// byte slice, which shares the bytes as the Arc does.
type ImageKind interface {
	isImageKind()
}

// ImageKindJPEG is ImageKind::JPEG in usvg src/tree/mod.rs. It is a reference
// to raw JPEG data. Should be decoded by the caller.
type ImageKindJPEG []byte

// ImageKindPNG is ImageKind::PNG in usvg src/tree/mod.rs. It is a reference
// to raw PNG data. Should be decoded by the caller.
type ImageKindPNG []byte

// ImageKindGIF is ImageKind::GIF in usvg src/tree/mod.rs. It is a reference
// to raw GIF data. Should be decoded by the caller.
type ImageKindGIF []byte

// ImageKindWEBP is ImageKind::WEBP in usvg src/tree/mod.rs. It is a reference
// to raw WebP data. Should be decoded by the caller.
type ImageKindWEBP []byte

// ImageKindSVG is ImageKind::SVG in usvg src/tree/mod.rs. It is a
// preprocessed SVG tree. Can be rendered as is.
//
// Port: upstream holds the Tree by value. Go holds a pointer.
type ImageKindSVG struct {
	Tree *Tree
}

// isImageKind makes ImageKindJPEG a case of ImageKind.
func (ImageKindJPEG) isImageKind() {}

// isImageKind makes ImageKindPNG a case of ImageKind.
func (ImageKindPNG) isImageKind() {}

// isImageKind makes ImageKindGIF a case of ImageKind.
func (ImageKindGIF) isImageKind() {}

// isImageKind makes ImageKindWEBP a case of ImageKind.
func (ImageKindWEBP) isImageKind() {}

// isImageKind makes ImageKindSVG a case of ImageKind.
func (ImageKindSVG) isImageKind() {}

// imageKindActualSize is ImageKind::actual_size in usvg src/tree/mod.rs.
//
// Port: upstream reads the size with the crate imagesize. Go reads it with
// image.DecodeConfig, the decoders of the standard library and
// golang.org/x/image/webp (D7). Upstream logs to the global logger of the
// crate log. Go takes the options, and logs to the logger of WithLogger
// (D20).
func imageKindActualSize(kind ImageKind, opt *options) (Size, bool) {
	var data []byte
	switch k := kind.(type) {
	case ImageKindJPEG:
		data = k
	case ImageKindPNG:
		data = k
	case ImageKindGIF:
		data = k
	case ImageKindWEBP:
		data = k
	case ImageKindSVG:
		return k.Tree.size, true
	default:
		return Size{}, false
	}

	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		if size, ok := path.SizeFromWH(float32(cfg.Width), float32(cfg.Height)); ok {
			return size, true
		}
	}
	opt.warnf("Image has an invalid size. Skipped.")
	return Size{}, false
}

// String is the impl Debug for ImageKind in usvg src/tree/mod.rs.
func (ImageKindJPEG) String() string { return "ImageKind::JPEG(..)" }

// String is the impl Debug for ImageKind in usvg src/tree/mod.rs.
func (ImageKindPNG) String() string { return "ImageKind::PNG(..)" }

// String is the impl Debug for ImageKind in usvg src/tree/mod.rs.
func (ImageKindGIF) String() string { return "ImageKind::GIF(..)" }

// String is the impl Debug for ImageKind in usvg src/tree/mod.rs.
func (ImageKindWEBP) String() string { return "ImageKind::WEBP(..)" }

// String is the impl Debug for ImageKind in usvg src/tree/mod.rs.
func (ImageKindSVG) String() string { return "ImageKind::SVG(..)" }

// Image is Image in usvg src/tree/mod.rs. It is a raster image element.
//
// `image` element in SVG.
type Image struct {
	id             string
	visible        bool
	size           Size
	renderingMode  ImageRendering
	kind           ImageKind
	absTransform   Transform
	absBoundingBox NonZeroRect
}

// ID is Image::id in usvg src/tree/mod.rs. It returns the element's ID.
//
// Taken from the SVG itself.
// Isn't automatically generated.
// Can be empty.
func (i *Image) ID() string {
	return i.id
}

// IsVisible is Image::is_visible in usvg src/tree/mod.rs. It returns the
// element visibility.
func (i *Image) IsVisible() bool {
	return i.visible
}

// Size is Image::size in usvg src/tree/mod.rs. It returns the actual image
// size.
//
// This is not `width` and `height` attributes,
// but rather the actual PNG/JPEG/GIF/SVG image size.
func (i *Image) Size() Size {
	return i.size
}

// RenderingMode is Image::rendering_mode in usvg src/tree/mod.rs. It returns
// the rendering mode.
//
// `image-rendering` in SVG.
func (i *Image) RenderingMode() ImageRendering {
	return i.renderingMode
}

// Kind is Image::kind in usvg src/tree/mod.rs. It returns the image data.
func (i *Image) Kind() ImageKind {
	return i.kind
}

// AbsTransform is Image::abs_transform in usvg src/tree/mod.rs. It returns
// the element's absolute transform.
//
// Contains all ancestors transforms including elements's transform.
//
// Note that this is not the relative transform present in SVG.
// The SVG one would be set only on groups.
func (i *Image) AbsTransform() Transform {
	return i.absTransform
}

// BoundingBox is Image::bounding_box in usvg src/tree/mod.rs. It returns the
// element's object bounding box.
//
// `objectBoundingBox` in SVG terms. Meaning it doesn't affected by parent transforms.
func (i *Image) BoundingBox() Rect {
	return mustRect(i.size.ToRect(0.0, 0.0))
}

// AbsBoundingBox is Image::abs_bounding_box in usvg src/tree/mod.rs. It
// returns the element's bounding box in canvas coordinates.
//
// `userSpaceOnUse` in SVG terms.
func (i *Image) AbsBoundingBox() Rect {
	return i.absBoundingBox.ToRect()
}

// Subroots is the private Image::subroots in usvg src/tree/mod.rs, which
// Node::subroots calls for an image.
func (i *Image) Subroots(f func(*Group)) {
	if svg, ok := i.kind.(ImageKindSVG); ok {
		f(&svg.Tree.root)
	}
}

// Tree is Tree in usvg src/tree/mod.rs. It is a nodes tree container.
type Tree struct {
	size            Size
	root            Group
	linearGradients []*LinearGradient
	radialGradients []*RadialGradient
	patterns        []*Pattern
	clipPaths       []*ClipPath
	masks           []*Mask
	filters         []*Filter
	fontdb          *FontDatabase
}

// Size is Tree::size in usvg src/tree/mod.rs. It returns the image size.
//
// Size of an image that should be created to fit the SVG.
//
// `width` and `height` in SVG.
//
// Note that this does not necessarily represent the bounding box of the
// rendered contents. Use
// [`self.root().abs_layer_bounding_box()`](Group::abs_layer_bounding_box)
// to retrieve it instead.
func (t *Tree) Size() Size {
	return t.size
}

// Root is Tree::root in usvg src/tree/mod.rs. It returns the root element of
// the SVG tree.
func (t *Tree) Root() *Group {
	return &t.root
}

// NodeByID is Tree::node_by_id in usvg src/tree/mod.rs. It returns a
// renderable node by ID.
//
// If an empty ID is provided, than this method will always return `None`.
func (t *Tree) NodeByID(id string) (Node, bool) {
	if id == "" {
		return nil, false
	}

	return nodeByID(&t.root, id)
}

// FontDB is Tree::fontdb in usvg src/tree/mod.rs. It returns the font
// database that applies to all text nodes in the tree.
func (t *Tree) FontDB() *FontDatabase {
	return t.fontdb
}

// HasTextNodes is Tree::has_text_nodes in usvg src/tree/mod.rs. It checks if
// the current tree has any text nodes.
func (t *Tree) HasTextNodes() bool {
	return hasTextNodes(&t.root)
}

// HasDefsNodes is Tree::has_defs_nodes in usvg src/tree/mod.rs. It checks if
// the current tree has any `defs` nodes.
func (t *Tree) HasDefsNodes() bool {
	return len(t.LinearGradients()) != 0 ||
		len(t.RadialGradients()) != 0 ||
		len(t.Patterns()) != 0 ||
		len(t.Filters()) != 0 ||
		len(t.ClipPaths()) != 0 ||
		len(t.Masks()) != 0
}

// LinearGradients is Tree::linear_gradients in usvg src/tree/mod.rs. It
// returns a list of all unique [`LinearGradient`]s in the tree.
func (t *Tree) LinearGradients() []*LinearGradient {
	return t.linearGradients
}

// RadialGradients is Tree::radial_gradients in usvg src/tree/mod.rs. It
// returns a list of all unique [`RadialGradient`]s in the tree.
func (t *Tree) RadialGradients() []*RadialGradient {
	return t.radialGradients
}

// Patterns is Tree::patterns in usvg src/tree/mod.rs. It returns a list of
// all unique [`Pattern`]s in the tree.
func (t *Tree) Patterns() []*Pattern {
	return t.patterns
}

// ClipPaths is Tree::clip_paths in usvg src/tree/mod.rs. It returns a list of
// all unique [`ClipPath`]s in the tree.
func (t *Tree) ClipPaths() []*ClipPath {
	return t.clipPaths
}

// Masks is Tree::masks in usvg src/tree/mod.rs. It returns a list of all
// unique [`Mask`]s in the tree.
func (t *Tree) Masks() []*Mask {
	return t.masks
}

// Filters is Tree::filters in usvg src/tree/mod.rs. It returns a list of all
// unique [`Filter`](filter::Filter)s in the tree.
func (t *Tree) Filters() []*Filter {
	return t.filters
}

// collectPaintServers is Tree::collect_paint_servers in usvg
// src/tree/mod.rs.
func (t *Tree) collectPaintServers() {
	loopOverPaintServers(&t.root, func(paint Paint) {
		switch p := paint.(type) {
		case Color:
		case *LinearGradient:
			if !slices.Contains(t.linearGradients, p) {
				t.linearGradients = append(t.linearGradients, p)
			}
		case *RadialGradient:
			if !slices.Contains(t.radialGradients, p) {
				t.radialGradients = append(t.radialGradients, p)
			}
		case *Pattern:
			if !slices.Contains(t.patterns, p) {
				t.patterns = append(t.patterns, p)
			}
		}
	})
}

// nodeByID is node_by_id in usvg src/tree/mod.rs.
func nodeByID(parent *Group, id string) (Node, bool) {
	for _, child := range parent.children {
		if child.ID() == id {
			return child, true
		}

		if g, ok := child.(*Group); ok {
			if n, ok := nodeByID(g, id); ok {
				return n, true
			}
		}
	}

	return nil, false
}

// hasTextNodes is has_text_nodes in usvg src/tree/mod.rs.
func hasTextNodes(root *Group) bool {
	for _, node := range root.children {
		if _, ok := node.(*Text); ok {
			return true
		}

		hasText := false

		if image, ok := node.(*Image); ok {
			if tree, ok := image.kind.(ImageKindSVG); ok {
				if hasTextNodes(&tree.Tree.root) {
					hasText = true
				}
			}
		}

		node.Subroots(func(subroot *Group) {
			hasText = hasTextNodes(subroot) || hasText
		})

		if hasText {
			return true
		}
	}

	return false
}

// loopOverPaintServers is loop_over_paint_servers in usvg src/tree/mod.rs.
func loopOverPaintServers(parent *Group, f func(Paint)) {
	push := func(paint Paint) {
		if paint != nil {
			f(paint)
		}
	}

	for _, node := range parent.children {
		switch n := node.(type) {
		case *Group:
			loopOverPaintServers(n, f)
		case *Path:
			if n.fill != nil {
				push(n.fill.paint)
			}
			if n.stroke != nil {
				push(n.stroke.paint)
			}
		case *Image:
		// Flattened text would be used instead.
		case *Text:
		}

		node.Subroots(func(subroot *Group) { loopOverPaintServers(subroot, f) })
	}
}

// collectClipPaths is Group::collect_clip_paths in usvg src/tree/mod.rs.
func (g *Group) collectClipPaths(clipPaths *[]*ClipPath) {
	for _, node := range g.Children() {
		if g, ok := node.(*Group); ok {
			if clip := g.clipPath; clip != nil {
				if !slices.Contains(*clipPaths, clip) {
					*clipPaths = append(*clipPaths, clip)
				}

				if subClip := clip.clipPath; subClip != nil {
					if !slices.Contains(*clipPaths, subClip) {
						*clipPaths = append(*clipPaths, subClip)
					}
				}
			}
		}

		node.Subroots(func(subroot *Group) { subroot.collectClipPaths(clipPaths) })

		if g, ok := node.(*Group); ok {
			g.collectClipPaths(clipPaths)
		}
	}
}

// collectMasks is Group::collect_masks in usvg src/tree/mod.rs.
func (g *Group) collectMasks(masks *[]*Mask) {
	for _, node := range g.Children() {
		if g, ok := node.(*Group); ok {
			if mask := g.mask; mask != nil {
				if !slices.Contains(*masks, mask) {
					*masks = append(*masks, mask)
				}

				if subMask := mask.mask; subMask != nil {
					if !slices.Contains(*masks, subMask) {
						*masks = append(*masks, subMask)
					}
				}
			}
		}

		node.Subroots(func(subroot *Group) { subroot.collectMasks(masks) })

		if g, ok := node.(*Group); ok {
			g.collectMasks(masks)
		}
	}
}

// collectFilters is Group::collect_filters in usvg src/tree/mod.rs.
func (g *Group) collectFilters(filters *[]*Filter) {
	for _, node := range g.Children() {
		if g, ok := node.(*Group); ok {
			for _, filter := range g.Filters() {
				if !slices.Contains(*filters, filter) {
					*filters = append(*filters, filter)
				}
			}
		}

		node.Subroots(func(subroot *Group) { subroot.collectFilters(filters) })

		if g, ok := node.(*Group); ok {
			g.collectFilters(filters)
		}
	}
}

// unionOfChildren is Group::union_of_children in usvg src/tree/mod.rs.
func (g *Group) unionOfChildren(f func(Node) (Rect, bool)) (Rect, bool) {
	bbox := defaultBBox()
	for _, child := range g.children {
		if cBBox, ok := f(child); ok {
			bbox = bbox.expand(bboxFromRect(cBBox))
		}
	}
	return bbox.toRect()
}

// calculateLayerBoundingBox is Group::calculate_layer_bounding_box in usvg
// src/tree/mod.rs.
func (g *Group) calculateLayerBoundingBox() (NonZeroRect, bool) {
	// Filter bbox has a higher priority than layers bbox.
	if filterBBox, ok := g.FiltersBoundingBox(); ok {
		return filterBBox, true
	}
	layerBBox, ok := g.unionOfChildren(func(child Node) (Rect, bool) {
		if group, ok := child.(*Group); ok {
			r, ok := group.LayerBoundingBox()
			if !ok {
				return Rect{}, false
			}
			r, ok = r.Transform(group.transform)
			if !ok {
				return Rect{}, false
			}
			return r.ToRect(), true
		}
		// Not a group - no need to transform.
		return NodeStrokeBoundingBox(child)
	})
	if !ok {
		return NonZeroRect{}, false
	}
	// Nothing outside the clip path is visible. Filtered groups use their filter
	// region instead, because filters are applied before clipping.
	// The clip's stroke bounding box is used because, unlike the object one,
	// it covers the glyph outlines of text.
	if clip := g.clipPath; clip != nil {
		clipStrokeBBox, ok := clip.root.StrokeBoundingBox()
		if !ok {
			return NonZeroRect{}, false
		}
		clipBBox, ok := clipStrokeBBox.Transform(clip.transform)
		if !ok {
			return NonZeroRect{}, false
		}
		if layerBBox, ok = layerBBox.Intersect(clipBBox); !ok {
			return NonZeroRect{}, false
		}
	}
	return layerBBox.ToNonZeroRect()
}

// onceLock is std::sync::OnceLock of Rust. The first call of getOrInit sets
// its value, and each later call returns that value.
//
// Port: a mutex guards the first set, and done lets each later read go
// without the lock, so that a tree is safe to read from several goroutines,
// as a Tree of upstream is Sync. A onceLock must not be copied, so a clone
// copies a set value with cloneInto.
type onceLock[T any] struct {
	mu    sync.Mutex
	done  atomic.Bool
	value T
}

// getOrInit is OnceLock::get_or_init. It returns the value, and sets it to
// the result of f first when it is not set.
func (l *onceLock[T]) getOrInit(f func() T) T {
	if l.done.Load() {
		return l.value
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.done.Load() {
		l.value = f()
		l.done.Store(true)
	}
	return l.value
}

// get is OnceLock::get. It returns the value and whether it is set.
func (l *onceLock[T]) get() (T, bool) {
	if l.done.Load() {
		return l.value, true
	}
	var zero T
	return zero, false
}

// set is the assignment of OnceLock::from(value) to a field of upstream. It
// sets the value, also when it is already set.
func (l *onceLock[T]) set(value T) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.value = value
	l.done.Store(true)
}

// cloneInto is the Clone of OnceLock. It sets dst to the value of l, when l
// has one.
func (l *onceLock[T]) cloneInto(dst *onceLock[T]) {
	if v, ok := l.get(); ok {
		dst.set(v)
	}
}

// optional is an Option<T> of upstream that a struct holds as a field.
//
// Port: Go has no Option type. A function returns (T, bool), and a field
// holds an optional, so that a struct does not need a pointer for it.
type optional[T any] struct {
	value T
	ok    bool
}

// optionalOf makes an optional from the result of a function that returns
// (T, bool).
func optionalOf[T any](value T, ok bool) optional[T] {
	if !ok {
		return optional[T]{}
	}
	return optional[T]{value: value, ok: true}
}

// some is Some(value) of upstream.
func some[T any](value T) optional[T] {
	return optional[T]{value: value, ok: true}
}

// get returns the value and whether it is set.
func (o optional[T]) get() (T, bool) {
	return o.value, o.ok
}

// mustRect is the unwrap of an Option<Rect> that upstream relies on. It
// panics when ok is false.
func mustRect(r Rect, ok bool) Rect {
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	return r
}

// mustNonZeroRect is the unwrap of an Option<NonZeroRect> that upstream
// relies on. It panics when ok is false.
func mustNonZeroRect(r NonZeroRect, ok bool) NonZeroRect {
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	return r
}
