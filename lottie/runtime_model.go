// Copyright 2024 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports velato src/runtime/model/mod.rs and
// src/runtime/model/fixed.rs.

import (
	"math"
	"strings"

	"github.com/xo/okage/internal/kurbo"
)

// ImageAsset is ImageAsset in velato src/runtime/model/mod.rs: the metadata
// of a raster image. A Sink loads and decodes it.
type ImageAsset struct {
	// Identifier referenced by an image layer's refId.
	ID string
	// Authored display width in pixels, or nil.
	Width *float64
	// Authored display height in pixels, or nil.
	Height *float64
	// Directory or URL prefix from the asset's u field, or nil.
	Directory *string
	// File name or data URL from the asset's p field.
	FileName string
	// Whether the asset is marked as embedded by its e field.
	Embedded bool
}

// Location is ImageAsset::location in velato src/runtime/model/mod.rs: the
// directory and the file name, or the unchanged data URL.
func (a *ImageAsset) Location() string {
	if a.IsDataURL() {
		return a.FileName
	}
	if a.Directory != nil {
		return *a.Directory + a.FileName
	}
	return a.FileName
}

// IsDataURL is ImageAsset::is_data_url in velato src/runtime/model/mod.rs.
func (a *ImageAsset) IsDataURL() bool {
	return strings.HasPrefix(a.FileName, "data:")
}

// Transform is Transform in velato src/runtime/model/mod.rs: a fixed affine
// transform, or an animated one (D27).
type Transform struct {
	Fixed    kurbo.Affine
	Animated *AnimatedTransform
}

// defaultTransform is impl Default for Transform: the identity.
func defaultTransform() Transform {
	return Transform{Fixed: kurbo.IdentityAffine}
}

// IsFixed is Transform::is_fixed in velato src/runtime/model/mod.rs.
func (t Transform) IsFixed() bool {
	return t.Animated == nil || t.Animated.IsFixed()
}

// Components is Transform::components in velato src/runtime/model/mod.rs:
// the authored components, or false for a matrix-only transform.
func (t Transform) Components(frame float64) (TransformComponents, bool) {
	if t.Animated == nil {
		return TransformComponents{}, false
	}
	return t.Animated.Components(frame), true
}

// Evaluate is Transform::evaluate in velato src/runtime/model/mod.rs.
func (t Transform) Evaluate(frame float64) kurbo.Affine {
	if t.Animated == nil {
		return t.Fixed
	}
	return t.Animated.Evaluate(frame)
}

// StrokeValue is Stroke in velato src/runtime/model/mod.rs: a fixed or an
// animated stroke (D27).
type StrokeValue struct {
	Fixed    Stroke
	Animated *AnimatedStroke
}

// Evaluate is Stroke::evaluate in velato src/runtime/model/mod.rs.
func (s StrokeValue) Evaluate(frame float64) Stroke {
	if s.Animated == nil {
		return s.Fixed
	}
	return s.Animated.Evaluate(frame)
}

// RepeaterValue is Repeater in velato src/runtime/model/mod.rs: a fixed or
// an animated repeater (D27).
type RepeaterValue struct {
	Fixed    FixedRepeater
	Animated *AnimatedRepeater
}

// Evaluate is Repeater::evaluate in velato src/runtime/model/mod.rs.
func (r RepeaterValue) Evaluate(frame float64) FixedRepeater {
	if r.Animated == nil {
		return r.Fixed
	}
	return r.Animated.Evaluate(frame)
}

// RepeaterComposite is RepeaterComposite in velato
// src/runtime/model/mod.rs: how repeater copies stack.
type RepeaterComposite uint8

// The values of RepeaterComposite.
const (
	// RepeaterBelow paints later copies below earlier copies.
	RepeaterBelow RepeaterComposite = iota
	// RepeaterAbove paints later copies above earlier copies.
	RepeaterAbove
)

// ColorStopsValue is ColorStops in velato src/runtime/model/mod.rs: fixed or
// animated color stops (D27).
type ColorStopsValue struct {
	Fixed    ColorStops
	Animated *AnimatedColorStops
}

// IsFixed is ColorStops::is_fixed in velato src/runtime/model/mod.rs.
func (c ColorStopsValue) IsFixed() bool {
	return c.Animated == nil
}

// Evaluate is ColorStops::evaluate in velato src/runtime/model/mod.rs.
func (c ColorStopsValue) Evaluate(frame float64) ColorStops {
	if c.Animated == nil {
		return c.Fixed
	}
	return c.Animated.Evaluate(frame)
}

// BrushValue is Brush in velato src/runtime/model/mod.rs: a fixed or an
// animated brush (D27).
type BrushValue struct {
	Fixed    Brush
	Animated *AnimatedBrush
}

// Evaluate is Brush::evaluate in velato src/runtime/model/mod.rs.
func (b BrushValue) Evaluate(alpha, frame float64) Brush {
	if b.Animated == nil {
		if alpha == 1.0 {
			return b.Fixed
		}
		return b.Fixed.MultiplyAlpha(float32(alpha))
	}
	return b.Animated.Evaluate(alpha, frame)
}

// GeometryKind is the variant of Geometry in velato
// src/runtime/model/mod.rs.
type GeometryKind uint8

// The variants of Geometry.
const (
	GeometryFixed GeometryKind = iota
	GeometryRect
	GeometryEllipse
	GeometrySpline
	GeometryStar
)

// Geometry is Geometry in velato src/runtime/model/mod.rs (D27). The field
// of its kind holds the geometry.
type Geometry struct {
	Kind    GeometryKind
	Path    []kurbo.PathEl
	Rect    *AnimatedRect
	Ellipse *AnimatedEllipse
	Spline  *AnimatedSpline
	Star    *AnimatedStar
}

// Evaluate is Geometry::evaluate in velato src/runtime/model/mod.rs. It
// appends the path at frame to path.
func (g *Geometry) Evaluate(frame float64, path []kurbo.PathEl) []kurbo.PathEl {
	switch g.Kind {
	case GeometryFixed:
		return append(path, g.Path...)
	case GeometryRect:
		return g.Rect.Evaluate(frame, path)
	case GeometryEllipse:
		return g.Ellipse.Evaluate(frame, path)
	case GeometrySpline:
		path, _ = g.Spline.Evaluate(frame, path)
		return path
	}
	return g.Star.Evaluate(frame, path)
}

// Draw is Draw in velato src/runtime/model/mod.rs: a fill or a stroke.
type Draw struct {
	// Parameters for a stroked draw operation, or nil for a fill.
	Stroke *StrokeValue
	// Brush for the draw operation.
	Brush BrushValue
	// Opacity of the draw operation.
	Opacity Value[float64]
}

// ShapeKind is the variant of Shape in velato src/runtime/model/mod.rs.
type ShapeKind uint8

// The variants of Shape.
const (
	ShapeGroup ShapeKind = iota
	ShapeGeometry
	ShapeDraw
	ShapeRepeater
	ShapeTrim
	// ShapeMerge is a merge path.
	//
	// Port: velato skips merge paths. okage keeps them, and draws those
	// that need a boolean operation by compositing (D42).
	ShapeMerge
)

// Shape is Shape in velato src/runtime/model/mod.rs: an element of a shape
// layer (D27).
type Shape struct {
	Kind ShapeKind
	// Group holds the shapes of a group, and GroupTransform its transform,
	// or nil.
	Group          []Shape
	GroupTransform *GroupTransform
	Geometry       Geometry
	Draw           Draw
	Repeater       RepeaterValue
	Trim           TrimValue
	// Merge is the mode of a merge path.
	Merge MergeMode
}

// TransformComponents is TransformComponents in velato
// src/runtime/model/mod.rs: the evaluated authored values of a transform.
type TransformComponents struct {
	Anchor    kurbo.Point
	Position  kurbo.Point
	Scale     kurbo.Vec2
	Rotation  float64
	Skew      float64
	SkewAngle float64
}

// Matrix is TransformComponents::matrix in velato
// src/runtime/model/mod.rs.
func (c TransformComponents) Matrix() kurbo.Affine {
	skew := kurbo.IdentityAffine
	if c.Skew != 0.0 {
		angle := toRadians(c.SkewAngle)
		skew = kurbo.AffineRotate(-angle).
			Mul(kurbo.AffineSkew(-kurbo.Tan(toRadians(c.Skew)), 0.0)).
			Mul(kurbo.AffineRotate(angle))
	}
	return kurbo.AffineTranslate(c.Position.ToVec2()).
		Mul(kurbo.AffineRotate(toRadians(c.Rotation))).
		Mul(skew).
		Mul(kurbo.AffineScaleNonUniform(c.Scale.X/100.0, c.Scale.Y/100.0)).
		Mul(kurbo.AffineTranslate(c.Anchor.ToVec2().Mul(-1)))
}

// toRadians is f64::to_radians.
func toRadians(deg float64) float64 {
	return deg * (math.Pi / 180.0)
}

// GroupTransform is GroupTransform in velato src/runtime/model/mod.rs.
type GroupTransform struct {
	Transform Transform
	Opacity   Value[float64]
}

// LayerEffectKind is the variant of LayerEffect in velato
// src/runtime/model/mod.rs.
type LayerEffectKind uint8

// The variants of LayerEffect.
const (
	// EffectGaussianBlur is the Gaussian Blur effect, ty 29.
	EffectGaussianBlur LayerEffectKind = iota
	// EffectDropShadow is the Drop Shadow effect, ty 25.
	EffectDropShadow
	// EffectFill is the Fill effect, ty 21.
	EffectFill
	// EffectTint is the Tint effect, ty 20.
	EffectTint
)

// LayerEffect is LayerEffect in velato src/runtime/model/mod.rs: a layer
// effect with animated parameters (D27). Each kind uses the fields that its
// Rust variant has.
type LayerEffect struct {
	Kind LayerEffectKind
	// GaussianBlur.
	Blurriness, Dimensions, Wrap Value[float64]
	// DropShadow, and Color for Fill.
	Color                              Value[Color]
	Opacity, Angle, Distance, Softness Value[float64]
	// Tint.
	Black, White Value[Color]
	Amount       Value[float64]
}

// UnsupportedEffect is UnsupportedEffect in velato
// src/runtime/model/mod.rs.
type UnsupportedEffect struct {
	Name       string
	EffectType *uint64
}

// LayerReference is LayerReference in velato src/runtime/model/mod.rs: a
// resolved index in the containing composition, or an authored ind that
// import did not find.
type LayerReference struct {
	Index    int
	Resolved bool
}

// LayerMatte is the matte of a layer: its blend mode and its source.
type LayerMatte struct {
	Mode   BlendMode
	Source LayerReference
}

// Layer is Layer in velato src/runtime/model/mod.rs.
type Layer struct {
	// Excluded from normal rendering, but still usable as a matte source or
	// parent.
	Hidden  bool
	Effects []LayerEffect
	// Effects that import could not convert.
	UnsupportedEffects []UnsupportedEffect
	// Name of the layer.
	Name string
	// Transform parent in the same composition, or nil.
	Parent *LayerReference
	// Transform for the entire layer.
	Transform Transform
	// Opacity for the entire layer.
	Opacity Value[float64]
	// Width and Height of the layer.
	Width, Height float64
	// Blend mode for the layer, or nil.
	BlendMode *BlendMode
	// Range of frames in which the layer is active: [Frames[0], Frames[1]).
	Frames [2]float64
	// Frame time stretch factor.
	Stretch float64
	// Starting frame for the layer (only applied to instances).
	StartFrame float64
	// List of masks applied to the content.
	Masks []Mask
	// True if the layer is used as a matte source.
	IsMatteSource bool
	// Matte blend mode and source layer, or nil.
	Matte *LayerMatte
	// Content of the layer.
	Content Content
}

// Mask is Mask in velato src/runtime/model/mod.rs.
type Mask struct {
	// Blend mode for the mask.
	Mode BlendMode
	// Geometry that defines the shape of the mask.
	Geometry Geometry
	// Opacity of the mask.
	Opacity Value[float64]
	// Port: the mode, the inversion and the expansion of the mask, which
	// velato does not read (D43).
	MaskMode MaskMode
	Inverted bool
	Expand   Value[float64]
}

// isClip reports if the mask works as a clip: it adds, it is not inverted,
// and it has the opacity 100 and no expansion. velato draws each mask as a
// clip (D43).
func (m *Mask) isClip() bool {
	return (m.MaskMode == "" || m.MaskMode == MaskModeAdd) && !m.Inverted &&
		m.Opacity.IsFixed() && m.Opacity.Fixed == 100 && m.Expand.IsFixed() && m.Expand.Fixed == 0
}

// ContentKind is the variant of Content in velato src/runtime/model/mod.rs.
type ContentKind uint8

// The variants of Content.
const (
	ContentNone ContentKind = iota
	ContentInstance
	ContentImage
	ContentShape
	// ContentSolid is a solid layer.
	//
	// Port: velato gives a solid layer no content. okage fills its
	// rectangle with its color, as ThorVG does (D43).
	ContentSolid
)

// Content is Content in velato src/runtime/model/mod.rs (D27).
type Content struct {
	Kind ContentKind
	// Name is the asset of an instance, or the image of an image layer.
	Name string
	// TimeRemap is the time remap of an instance, or nil.
	TimeRemap *Value[float64]
	// Shapes is the collection of shapes of a shape layer.
	Shapes []Shape
	// Color is the color of a solid layer, and Width and Height its size.
	Color         Color
	Width, Height float64
}

// TrimMode is TrimMode in velato src/runtime/model/mod.rs.
type TrimMode uint8

// The values of TrimMode.
const (
	// TrimParallel applies the range to each contour (m: 1).
	TrimParallel TrimMode = iota
	// TrimSequential applies the range across the combined lengths (m: 2).
	TrimSequential
)

// TrimValue is Trim in velato src/runtime/model/mod.rs: a fixed or an
// animated trim (D27).
type TrimValue struct {
	Fixed    FixedTrim
	Animated *AnimatedTrim
}

// Evaluate is Trim::evaluate in velato src/runtime/model/mod.rs.
func (t TrimValue) Evaluate(frame float64) FixedTrim {
	if t.Animated == nil {
		return t.Fixed
	}
	return t.Animated.Evaluate(frame)
}

// FixedRepeater is Repeater in velato src/runtime/model/fixed.rs.
type FixedRepeater struct {
	// Number of times to repeat.
	Copies int
	// Offset of each subsequent repeated element.
	Offset float64
	// Anchor point.
	AnchorPoint kurbo.Point
	// Translation.
	Position kurbo.Point
	// Rotation in degrees.
	Rotation float64
	// Scale.
	Scale kurbo.Vec2
	// Opacity of the first element.
	StartOpacity float64
	// Opacity of the last element.
	EndOpacity float64
	// How copies are stacked relative to the original shape.
	Composite RepeaterComposite
}

// Transform is Repeater::transform in velato src/runtime/model/fixed.rs: the
// transform for the copy index.
func (r *FixedRepeater) Transform(index int) kurbo.Affine {
	t := r.Offset + float64(index)
	scale := func(value float64) float64 {
		base := value / 100.0
		if base < 0.0 && t-math.Trunc(t) != 0.0 {
			lower := math.Floor(t)
			fraction := t - lower
			return kurbo.Powf(base, lower)*(1.0-fraction) + kurbo.Powf(base, lower+1.0)*fraction
		}
		return kurbo.Powf(base, t)
	}
	return kurbo.AffineTranslate(kurbo.NewVec2(
		t*r.Position.X+r.AnchorPoint.X,
		t*r.Position.Y+r.AnchorPoint.Y,
	)).Mul(kurbo.AffineRotate(toRadians(t * r.Rotation))).
		Mul(kurbo.AffineScaleNonUniform(scale(r.Scale.X), scale(r.Scale.Y))).
		Mul(kurbo.AffineTranslate(kurbo.NewVec2(-r.AnchorPoint.X, -r.AnchorPoint.Y)))
}

// FixedTrim is Trim in velato src/runtime/model/fixed.rs.
type FixedTrim struct {
	Mode TrimMode
	// Start of the visible segment (0.0 to 100.0).
	Start float64
	// End of the visible segment (0.0 to 100.0).
	End float64
	// Offset rotation in degrees.
	Offset float64
}

// trimRange is a normalized range of a trim, from 0 to 1.
type trimRange struct {
	Start, End float64
}

// Normalized is Trim::normalized in velato src/runtime/model/fixed.rs. It
// returns the first range, the second range when the offset wraps around,
// and false when the trim shows nothing.
func (t FixedTrim) Normalized() (trimRange, *trimRange, bool) {
	// LottieFiles' ThorVG player also clamps start/end before applying the
	// offset.
	start := clampUpTo(t.Start, 100.0)
	end := clampUpTo(t.End, 100.0)
	startPct, endPct := start, end
	if !(start <= end) {
		startPct, endPct = end, start
	}

	rangePct := endPct - startPct
	if rangePct < 1e-9 {
		return trimRange{}, nil, false
	}
	if rangePct >= 100.0-1e-9 {
		return trimRange{0.0, 1.0}, nil, true
	}

	offsetPct := t.Offset * (100.0 / 360.0)
	startPct = remEuclid(startPct+offsetPct, 100.0)
	endPct = startPct + rangePct

	if endPct <= 100.0 {
		return trimRange{startPct / 100.0, endPct / 100.0}, nil, true
	}
	return trimRange{startPct / 100.0, 1.0}, &trimRange{0.0, (endPct - 100.0) / 100.0}, true
}

// remEuclid is f64::rem_euclid.
func remEuclid(x, y float64) float64 {
	r := math.Mod(x, y)
	if r < 0.0 {
		return r + math.Abs(y)
	}
	return r
}
