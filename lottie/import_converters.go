// Copyright 2024 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports velato src/import/converters.rs, src/import/builders.rs
// and src/import/defaults.rs.

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/kurbo"
	"github.com/xo/okage/internal/lottiecompat"
)

// converter holds the first error of an import.
//
// Port: velato panics where a file has a value that it does not support,
// with todo!, unimplemented! or assert!, or where it indexes past the end of
// a slice. The converter records the first such case as an error that wraps
// okage.ErrLottieUnsupported, and the import returns it (D41).
type converter struct {
	err error
	// velato is true when the differences of D42 and D43 are off, for the
	// test against velato (internal/lottiecompat).
	velato bool
}

// failf records the first error.
func (c *converter) failf(format string, args ...any) {
	if c.err == nil {
		c.err = fmt.Errorf("%w: "+format, append([]any{okage.ErrLottieUnsupported}, args...)...)
	}
}

// The defaults of velato src/import/defaults.rs.
var (
	floatValueZero       = staticFloat(0.0)
	floatValueOneHundred = staticFloat(100.0)
	multidimOneHundred   = MultiDimensional{Value: AnimatedPropertyK[[]float64]{Static: []float64{100.0, 100.0, 100.0}}}
	positionZero         = PositionSchema{Value: PositionValueK{Static: []float64{0.0, 0.0}}}
)

// staticFloat returns a static FloatValue.
func staticFloat(v float64) FloatValue {
	return FloatValue{AnimatedProperty[float64]{Value: AnimatedPropertyK[float64]{Static: v}}}
}

// layerEntry is a converted layer with its authored id, matte mode and
// explicit matte index.
type layerEntry struct {
	layer      Layer
	id         uint
	matteMode  *BlendMode
	matteIndex *uint
}

// processLayers is process_layers in velato src/import/converters.rs.
func (c *converter) processLayers(sourceLayers []AnyLayer) []Layer {
	idmap := map[uint]int{}
	var converted []layerEntry
	for i := range sourceLayers {
		e := c.convLayer(&sourceLayers[i])
		idmap[e.id] = len(converted)
		converted = append(converted, e)
	}

	resolve := func(id uint) LayerReference {
		if index, ok := idmap[id]; ok {
			return LayerReference{Index: index, Resolved: true}
		}
		return LayerReference{Index: int(id)}
	}

	layers := make([]Layer, 0, len(converted))
	for idx, e := range converted {
		layer := e.layer
		if layer.Parent != nil && !layer.Parent.Resolved {
			r := resolve(uint(layer.Parent.Index))
			layer.Parent = &r
		}
		if e.matteMode != nil && *e.matteMode != blendMix(MixNormal) {
			var source *LayerReference
			if e.matteIndex != nil {
				r := resolve(*e.matteIndex)
				source = &r
			} else if idx > 0 {
				source = &LayerReference{Index: idx - 1, Resolved: true}
			}
			if source != nil {
				layer.Matte = &LayerMatte{Mode: *e.matteMode, Source: *source}
			}
		}
		layers = append(layers, layer)
	}

	// A layer's matte source is selected by tp, or defaults to the preceding
	// layer when tt enables a matte. Treat resolved sources as matte-only to
	// avoid drawing their content separately. This matches ThorVG.
	for index := range layers {
		if m := layers[index].Matte; m != nil && m.Source.Resolved {
			layers[m.Source.Index].IsMatteSource = true
		}
	}
	return layers
}

// convAnimation is conv_animation in velato src/import/converters.rs.
func convAnimation(source *Animation) (*Composition, error) {
	if math.IsInf(source.FrameRate, 0) || source.FrameRate != source.FrameRate || source.FrameRate <= 0.0 {
		return nil, fmt.Errorf("%w: invalid animation frame rate %v: expected a finite, positive value", okage.ErrLottieInvalid, source.FrameRate)
	}
	target := &Composition{
		Frames:    [2]float64{source.InPoint, source.OutPoint},
		FrameRate: source.FrameRate,
		Width:     source.Width,
		Height:    source.Height,
		Assets:    map[string][]Layer{},
		Images:    map[string]ImageAsset{},
	}
	c := converter{velato: lottiecompat.Velato}
	for _, asset := range source.Assets {
		switch {
		case asset.Precomposition != nil:
			target.Assets[asset.Precomposition.ID] = c.processLayers(asset.Precomposition.Layers)
		case asset.Image != nil:
			file := asset.Image.FileAsset
			target.Images[file.ID] = ImageAsset{
				ID:        file.ID,
				Width:     asset.Image.Width,
				Height:    asset.Image.Height,
				Directory: file.Dir,
				FileName:  file.FileName,
				Embedded:  file.Embedded.isTrue(),
			}
		}
	}
	target.Layers = c.processLayers(source.Layers)
	if c.err != nil {
		return nil, c.err
	}
	return target, nil
}

// convLayer is conv_layer in velato src/import/converters.rs.
func (c *converter) convLayer(source *AnyLayer) layerEntry {
	visual := source.visual()
	layer := Layer{Hidden: visual.Hidden != nil && *visual.Hidden, Transform: defaultTransform()}
	var e layerEntry
	switch {
	case source.Precomposition != nil:
		e = c.setupLayerBase(visual, &layer, source.Precomposition.StartTime)
		layer.Width, layer.Height = source.Precomposition.Width, source.Precomposition.Height
		content := Content{Kind: ContentInstance, Name: source.Precomposition.RefID}
		if tm := source.Precomposition.TimeRemap; tm != nil {
			v := c.convScalar(tm)
			content.TimeRemap = &v
		}
		layer.Content = content
	case source.Shape != nil:
		e = c.setupLayerBase(visual, &layer, startTime(visual))
		var shapes []Shape
		for i := range source.Shape.Shapes {
			if s, ok := c.convShape(&source.Shape.Shapes[i]); ok {
				shapes = append(shapes, s)
			}
		}
		layer.Content = Content{Kind: ContentShape, Shapes: shapes}
	case source.Image != nil:
		e = c.setupLayerBase(visual, &layer, startTime(visual))
		layer.Content = Content{Kind: ContentImage, Name: source.Image.RefID}
	case source.Solid != nil:
		e = c.setupLayerBase(visual, &layer, startTime(visual))
		// Port: velato gives a solid layer no content. okage fills its
		// rectangle with its color, as ThorVG does (D43).
		if color, ok := parseHexColor(source.Solid.Color); ok && !c.velato {
			layer.Content = Content{Kind: ContentSolid, Color: color, Width: source.Solid.Width, Height: source.Solid.Height}
		}
	default:
		// A null layer has no content.
		e = c.setupLayerBase(visual, &layer, startTime(visual))
	}

	for _, raw := range visual.Effects {
		var effect map[string]json.RawMessage
		if json.Unmarshal(raw, &effect) != nil {
			continue
		}
		if en, ok := effect["en"]; ok {
			var b bool
			var n uint64
			if (json.Unmarshal(en, &b) == nil && !b) || (json.Unmarshal(en, &n) == nil && n == 0) {
				continue
			}
		}
		if le, ok := c.convLayerEffect(effect); ok {
			layer.Effects = append(layer.Effects, le)
		} else {
			var name string
			if nm, ok := effect["nm"]; ok {
				_ = json.Unmarshal(nm, &name)
			}
			u := UnsupportedEffect{Name: name}
			if ty, ok := effect["ty"]; ok {
				var n uint64
				if json.Unmarshal(ty, &n) == nil {
					u.EffectType = &n
				}
			}
			layer.UnsupportedEffects = append(layer.UnsupportedEffects, u)
		}
	}
	e.layer = layer
	return e
}

// startTime returns the start time of a layer, 0 when it has none.
func startTime(visual *VisualLayer) float64 {
	if visual.StartTime != nil {
		return *visual.StartTime
	}
	return 0.0
}

// setupLayerBase is setup_layer_base, setup_shape_layer and
// setup_precomp_layer in velato src/import/builders.rs, which differ only in
// the start frame. It sets the common fields of target from source.
func (c *converter) setupLayerBase(source *VisualLayer, target *Layer, startFrame float64) layerEntry {
	if source.Name != nil {
		target.Name = *source.Name
	}
	if source.ParentIndex != nil {
		target.Parent = &LayerReference{Index: int(*source.ParentIndex)}
	}
	target.Transform, target.Opacity = c.convTransform(&source.Transform)
	target.IsMatteSource = source.MatteTarget.isTrue()

	var matteMode *BlendMode
	if source.MatteMode != nil {
		var m BlendMode
		switch *source.MatteMode {
		case MatteModeNormal:
			m = blendMix(MixNormal)
		case MatteModeAlpha, MatteModeLuma:
			m = blendCompose(ComposeSrcIn)
		default:
			m = blendCompose(ComposeSrcOut)
		}
		matteMode = &m
	}
	var matteIndex *uint
	if source.MatteLayerIndex != nil {
		u := castUsize(*source.MatteLayerIndex)
		matteIndex = &u
	}

	bm := BlendModeNormal
	if source.BlendMode != nil {
		bm = *source.BlendMode
	}
	target.BlendMode = c.convBlendMode(bm)
	if target.BlendMode != nil && *target.BlendMode == blendMix(MixNormal) {
		target.BlendMode = nil
	}
	target.Stretch = 1.0
	if source.TimeStretch != nil {
		target.Stretch = *source.TimeStretch
	}
	target.Frames = [2]float64{source.InPoint, source.OutPoint}
	target.StartFrame = startFrame

	for _, mask := range source.MasksProperties {
		if mask.Shape == nil {
			continue
		}
		geometry, ok := c.convShapeGeometry(mask.Shape)
		if !ok {
			continue
		}
		opacity := &floatValueOneHundred
		if mask.Opacity != nil {
			opacity = mask.Opacity
		}
		m := Mask{
			Mode:     blendMix(MixNormal),
			Geometry: geometry,
			Opacity:  c.convScalar(opacity),
			// Port: velato does not read the mode, the inversion and the
			// expansion (D43).
			Inverted: mask.Inverted != nil && *mask.Inverted,
			Expand:   c.convScalar(orFloat(mask.Expand, &floatValueZero)),
		}
		if mask.Mode != nil {
			m.MaskMode = *mask.Mode
		}
		target.Masks = append(target.Masks, m)
	}

	id := uint(0)
	if source.Index != nil {
		id = *source.Index
	}
	return layerEntry{id: id, matteMode: matteMode, matteIndex: matteIndex}
}

// castUsize is the cast "as usize" of an f64: it truncates and saturates,
// and NaN gives 0.
func castUsize(x float64) uint {
	switch {
	case !(x > 0):
		return 0
	case x >= math.MaxUint:
		return math.MaxUint
	}
	return uint(x)
}

// convLayerEffect is conv_layer_effect in velato src/import/converters.rs.
func (c *converter) convLayerEffect(effect map[string]json.RawMessage) (LayerEffect, bool) {
	var parameters []map[string]json.RawMessage
	if raw, ok := effect["ef"]; !ok || json.Unmarshal(raw, &parameters) != nil {
		return LayerEffect{}, false
	}
	param := func(index int) (json.RawMessage, bool) {
		if index >= len(parameters) {
			return nil, false
		}
		v, ok := parameters[index]["v"]
		return v, ok
	}
	scalar := func(index int) (Value[float64], bool) {
		raw, ok := param(index)
		if !ok {
			return Value[float64]{}, false
		}
		var v FloatValue
		if json.Unmarshal(raw, &v) != nil {
			return Value[float64]{}, false
		}
		return c.convScalar(&v), true
	}
	color := func(index int) (Value[Color], bool) {
		raw, ok := param(index)
		if !ok {
			return Value[Color]{}, false
		}
		var v ColorValue
		if json.Unmarshal(raw, &v) != nil {
			return Value[Color]{}, false
		}
		return c.convColor(&v), true
	}
	or := func(v Value[float64], ok bool, def float64) Value[float64] {
		if ok {
			return v
		}
		return fixedValue(def)
	}
	var ty uint64
	if raw, ok := effect["ty"]; !ok || json.Unmarshal(raw, &ty) != nil {
		return LayerEffect{}, false
	}
	var e LayerEffect
	var ok bool
	switch ty {
	case 29:
		e.Kind = EffectGaussianBlur
		if e.Blurriness, ok = scalar(0); !ok {
			return LayerEffect{}, false
		}
		v, ok1 := scalar(1)
		e.Dimensions = or(v, ok1, 1.0)
		v, ok2 := scalar(2)
		e.Wrap = or(v, ok2, 0.0)
	case 25:
		e.Kind = EffectDropShadow
		var ok1, ok2, ok3, ok4, ok5 bool
		e.Color, ok1 = color(0)
		e.Opacity, ok2 = scalar(1)
		e.Angle, ok3 = scalar(2)
		e.Distance, ok4 = scalar(3)
		e.Softness, ok5 = scalar(4)
		if !(ok1 && ok2 && ok3 && ok4 && ok5) {
			return LayerEffect{}, false
		}
	case 21:
		e.Kind = EffectFill
		var ok1, ok2 bool
		e.Color, ok1 = color(2)
		e.Opacity, ok2 = scalar(6)
		if !(ok1 && ok2) {
			return LayerEffect{}, false
		}
	case 20:
		e.Kind = EffectTint
		var ok1, ok2, ok3 bool
		e.Black, ok1 = color(0)
		e.White, ok2 = color(1)
		e.Amount, ok3 = scalar(2)
		if !(ok1 && ok2 && ok3) {
			return LayerEffect{}, false
		}
	default:
		return LayerEffect{}, false
	}
	return e, true
}

// convRotation returns the rotation of a transform, or reports the split
// rotation that velato does not support.
func (c *converter) convRotation(r *AnyTransformR) *FloatValue {
	if r == nil {
		return &floatValueZero
	}
	if r.IsSplit {
		c.failf("split rotation")
		return &floatValueZero
	}
	return &r.Rotation
}

// convTransform is conv_transform in velato src/import/converters.rs.
func (c *converter) convTransform(value *TransformSchema) (Transform, Value[float64]) {
	rotation := c.convRotation(value.Rotation)
	var position Position
	if value.Position.IsSplit {
		position = Position{
			Kind: PositionSplit,
			X:    c.convScalar(&value.Position.SplitPosition.X),
			Y:    c.convScalar(&value.Position.SplitPosition.Y),
		}
	} else {
		position = c.convPosPoint(&value.Position.Position)
	}
	t := &AnimatedTransform{
		Anchor:    c.convPosPoint(orPosition(value.AnchorPoint, &positionZero)),
		Position:  position,
		Scale:     c.convVec2(orMulti(value.Scale, &multidimOneHundred)),
		Rotation:  c.convScalar(rotation),
		Skew:      c.convScalar(orFloat(value.Skew, &floatValueZero)),
		SkewAngle: c.convScalar(orFloat(value.SkewAxis, &floatValueZero)),
	}
	opacity := c.convScalar(orFloat(value.Opacity, &floatValueOneHundred))
	return t.intoModel(), opacity
}

// convShapeTransform is conv_shape_transform in velato
// src/import/converters.rs.
func (c *converter) convShapeTransform(value *TransformShape) GroupTransform {
	rotation := c.convRotation(value.Rotation)
	if value.Position.IsSplit {
		c.failf("split position")
	}
	t := &AnimatedTransform{
		Anchor:    c.convPosPoint(orPosition(value.AnchorPoint, &positionZero)),
		Position:  c.convPosPoint(&value.Position.Position),
		Scale:     c.convVec2(orMulti(value.Scale, &multidimOneHundred)),
		Rotation:  c.convScalar(rotation),
		Skew:      c.convScalar(orFloat(value.Skew, &floatValueZero)),
		SkewAngle: c.convScalar(orFloat(value.SkewAxis, &floatValueZero)),
	}
	return GroupTransform{
		Transform: t.intoModel(),
		Opacity:   c.convScalar(orFloat(value.Opacity, &floatValueOneHundred)),
	}
}

// orFloat returns v, or def when v is nil.
func orFloat(v, def *FloatValue) *FloatValue {
	if v != nil {
		return v
	}
	return def
}

// orMulti returns v, or def when v is nil.
func orMulti(v, def *MultiDimensional) *MultiDimensional {
	if v != nil {
		return v
	}
	return def
}

// orPosition returns v, or def when v is nil.
func orPosition(v, def *PositionSchema) *PositionSchema {
	if v != nil {
		return v
	}
	return def
}

// collectTangents is the closure collect_tangents of conv_keyframes in
// velato src/import/converters.rs.
func collectTangents(handle *KeyframeBezierHandle) []EasingHandle {
	if handle == nil {
		return nil
	}
	x, y := handle.XCoordinate, handle.YCoordinate
	var handles []EasingHandle
	switch {
	case x.IsArray && y.IsArray:
		for i := 0; i < 2 && i < len(x.Values) && i < len(y.Values); i++ {
			handles = append(handles, EasingHandle{X: x.Values[i], Y: y.Values[i]})
		}
	case x.IsArray:
		for i := 0; i < 2 && i < len(x.Values); i++ {
			handles = append(handles, EasingHandle{X: x.Values[i], Y: y.Single})
		}
	case y.IsArray:
		for i := 0; i < 2 && i < len(y.Values); i++ {
			handles = append(handles, EasingHandle{X: x.Single, Y: y.Values[i]})
		}
	default:
		handles = append(handles, EasingHandle{X: x.Single, Y: y.Single})
	}
	return handles
}

// convKeyframes is conv_keyframes in velato src/import/converters.rs.
//
// Port: velato makes one keyframe for each pair of easing handles, so a
// keyframe whose handles hold arrays is two keyframes at one time. okage
// makes one keyframe with the first pair of handles, which every dimension
// takes, as ThorVG does (D43).
func convKeyframes[T any](keyframes []Keyframe, f func(*Keyframe) T, tween func(a, b T, t float64, e Easing) T) Value[T] {
	var frames []Time
	var values []T
	for i := range keyframes {
		k := &keyframes[i]
		tm := Time{Frame: k.Base.Time, Hold: k.Base.Hold.isTrue()}
		inTangents := collectTangents(k.Base.InTangent)
		outTangents := collectTangents(k.Base.OutTangent)
		n := min(len(inTangents), len(outTangents))
		if lottiecompat.Velato {
			// velato makes one keyframe for each pair of handles.
			if n == 0 {
				frames = append(frames, tm)
				values = append(values, f(k))
			}
			for j := range n {
				in, out := inTangents[j], outTangents[j]
				frames = append(frames, Time{Frame: tm.Frame, InTangent: &in, OutTangent: &out, Hold: tm.Hold})
				values = append(values, f(k))
			}
			continue
		}
		if n > 0 {
			tm.InTangent, tm.OutTangent = &inTangents[0], &outTangents[0]
		}
		frames = append(frames, tm)
		values = append(values, f(k))
	}
	return newAnimated(frames, values, tween)
}

// convKeyframeHandle is conv_keyframe_handle in velato
// src/import/converters.rs. An array of another length than 1 fails the
// assert of velato.
func (c *converter) convKeyframeHandle(handle *KeyframeBezierHandle) *EasingHandle {
	if handle == nil {
		return nil
	}
	component := func(k KeyframeComponent) float64 {
		if !k.IsArray {
			return k.Single
		}
		if len(k.Values) != 1 {
			c.failf("an easing handle with %d values", len(k.Values))
			if len(k.Values) == 0 {
				return 0
			}
		}
		return k.Values[0]
	}
	return &EasingHandle{X: component(handle.XCoordinate), Y: component(handle.YCoordinate)}
}

// convGradientColors is conv_gradient_colors in velato
// src/import/converters.rs.
//
// Port: the stops come from gradientStops, and an animated gradient keeps
// the raw data of each keyframe, which it tweens before it makes the stops
// (D43).
func (c *converter) convGradientColors(value *GradientColors) ColorStopsValue {
	count := int(value.Count)
	p := value.Colors.Value
	if c.velato {
		return c.convGradientColorsVelato(value)
	}
	if !p.IsAnimated {
		return ColorStopsValue{Fixed: gradientStops(p.Static, count)}
	}
	var frames []Time
	var values [][]float64
	for i := range p.Keyframes {
		k := &p.Keyframes[i]
		frames = append(frames, Time{
			Frame:      k.Base.Time,
			InTangent:  c.convKeyframeHandle(k.Base.InTangent),
			OutTangent: c.convKeyframeHandle(k.Base.OutTangent),
			Hold:       k.Base.Hold.isTrue(),
		})
		values = append(values, k.Value)
	}
	return ColorStopsValue{Animated: &AnimatedColorStops{Frames: frames, Values: values, Count: count}}
}

// convGradientColorsVelato is conv_gradient_colors in velato
// src/import/converters.rs, without the change of D43.
func (c *converter) convGradientColorsVelato(value *GradientColors) ColorStopsValue {
	count := int(value.Count)
	p := value.Colors.Value
	if !p.IsAnimated {
		var stops ColorStops
		for _, v := range convStops(p.Static, count) {
			stops = append(stops, ColorStop{
				Offset: float32(v[0]),
				Color:  Color{float32(v[1]), float32(v[2]), float32(v[3]), float32(v[4])},
			})
		}
		return ColorStopsValue{Fixed: stops}
	}
	var frames []Time
	var values [][]float64
	for i := range p.Keyframes {
		k := &p.Keyframes[i]
		frames = append(frames, Time{
			Frame:      k.Base.Time,
			InTangent:  c.convKeyframeHandle(k.Base.InTangent),
			OutTangent: c.convKeyframeHandle(k.Base.OutTangent),
			Hold:       k.Base.Hold.isTrue(),
		})
		var flat []float64
		for _, s := range convStops(k.Value, count) {
			flat = append(flat, s[:]...)
		}
		values = append(values, flat)
	}
	return ColorStopsValue{Animated: &AnimatedColorStops{Frames: frames, Values: values, Count: count, velato: true}}
}

// convStops is conv_stops in velato src/import/converters.rs: the stops of
// a gradient, each the offset, the three components of the color and the
// alpha, which comes from the opacity stops at the offsets of the color
// stops. okage uses gradientStops instead (D43).
func convStops(value []float64, count int) [][5]float64 {
	var stops [][5]float64
	for i := 0; i+4 <= len(value); i += 4 {
		chunk := value[i : i+4]
		stops = append(stops, [5]float64{chunk[0], chunk[1], chunk[2], chunk[3], 1.0})
		if len(stops) < count {
			continue
		}
		// There is alpha data at the end of the list, which is a sequence
		// of (offset, alpha) pairs.
		var alphaStops [][2]float64
		for j := count * 4; j+2 <= len(value); j += 2 {
			alphaStops = append(alphaStops, [2]float64{value[j], value[j+1]})
		}
		for s := range stops {
			stop := &stops[s]
			var last *[2]float64
			for k := range alphaStops {
				b, alphaB := alphaStops[k][0], alphaStops[k][1]
				if last != nil {
					a, alphaA := last[0], last[1]
					x := stop[0]
					t := normalizeToRange(a, b, x)
					alphaInterp := tweenF64(alphaA, alphaB, t, easingLerp)
					// todo: this is a hack to get alpha rendering with a
					// falloff similar to LottieFiles
					if x >= a && x <= b && t <= 0.25 && x <= 0.1 {
						alphaInterp = alphaA
					}
					if x >= a && x <= b && t >= 0.75 && x >= 0.9 {
						alphaInterp = alphaB
					}
					stop[4] = rustMin(stop[4], alphaInterp)
				}
				last = &alphaStops[k]
			}
		}
		break
	}
	return stops
}

// convJoin converts a line join, with def when it is nil.
func convJoin(j *LineJoin, def LineJoin) Join {
	v := def
	if j != nil {
		v = *j
	}
	switch v {
	case LineJoinRound:
		return JoinRound
	case LineJoinMiter:
		return JoinMiter
	}
	return JoinBevel
}

// convCap converts a line cap, with def when it is nil.
func convCap(cp *LineCap, def LineCap) Cap {
	v := def
	if cp != nil {
		v = *cp
	}
	switch v {
	case LineCapRound:
		return CapRound
	case LineCapSquare:
		return CapSquare
	}
	return CapButt
}

// convGradient converts the gradient of a gradient fill or stroke.
func (c *converter) convGradient(g *BaseGradientShape) *AnimatedBrush {
	return &AnimatedBrush{IsGradient: true, Gradient: AnimatedGradient{
		IsRadial:   g.GradientType != nil && *g.GradientType == GradientTypeRadial,
		StartPoint: c.convMultiPoint(&g.StartPoint),
		EndPoint:   c.convMultiPoint(&g.EndPoint),
		Stops:      c.convGradientColors(&g.Colors),
		// Port: velato ignores the highlight (D43).
		HighlightLength: c.convHighlight(g.HighlightLength),
		HighlightAngle:  c.convHighlight(g.HighlightAngle),
	}}
}

// convHighlight converts a value of the highlight of a radial gradient, which
// velato ignores (D43).
func (c *converter) convHighlight(v *FloatValue) Value[float64] {
	if c.velato || v == nil {
		return fixedValue(0.0)
	}
	return c.convScalar(v)
}

// gradientOpacity converts the opacity of a gradient, which velato sets to
// 100 (D43).
func (c *converter) gradientOpacity(v *FloatValue) Value[float64] {
	if c.velato {
		return fixedValue(100.0)
	}
	return c.convScalar(v)
}

// convDashes sets the dashes of stroke from the dash items of a stroke
// shape, which velato does not read (D43). The items of the kind "d" and
// "g" give the lengths in their order, and the item of the kind "o" gives
// the offset.
func (c *converter) convDashes(stroke *AnimatedStroke, dashes []StrokeDashShape) {
	if c.velato {
		return
	}
	stroke.DashOffset = fixedValue(0.0)
	for i := range dashes {
		d := &dashes[i]
		if d.Length == nil {
			continue
		}
		v := c.convScalar(d.Length)
		if d.DashType != nil && *d.DashType == "o" {
			stroke.DashOffset = v
			continue
		}
		stroke.Dashes = append(stroke.Dashes, v)
	}
}

// convDraw is conv_draw in velato src/import/converters.rs.
func (c *converter) convDraw(value *AnyShape) (Draw, bool) {
	switch {
	case value.Fill != nil:
		brush := &AnimatedBrush{Solid: c.convColor(&value.Fill.Color)}
		return Draw{Brush: brush.intoModel(), Opacity: c.convScalar(&value.Fill.Opacity)}, true
	case value.Stroke != nil:
		s := value.Stroke
		stroke := &AnimatedStroke{
			Width:      c.convScalar(&s.Width),
			Join:       convJoin(s.LineJoin, LineJoinBevel),
			MiterLimit: s.MiterLimit,
			Cap:        convCap(s.LineCap, LineCapButt),
		}
		c.convDashes(stroke, s.Dashes)
		sv := stroke.intoModel()
		brush := &AnimatedBrush{Solid: c.convColor(&s.StrokeColor)}
		return Draw{Stroke: &sv, Brush: brush.intoModel(), Opacity: c.convScalar(&s.Opacity)}, true
	case value.GradientFill != nil:
		// Port: velato gives a gradient fill the opacity 100. okage takes its
		// opacity, as ThorVG does (D43).
		return Draw{Brush: c.convGradient(&value.GradientFill.BaseGradientShape).intoModel(), Opacity: c.gradientOpacity(&value.GradientFill.Opacity)}, true
	case value.GradientStroke != nil:
		s := value.GradientStroke
		stroke := &AnimatedStroke{
			Width:      c.convScalar(&s.Width),
			Join:       convJoin(s.LineJoin, LineJoinRound),
			MiterLimit: s.MiterLimit,
			Cap:        convCap(s.LineCap, LineCapRound),
		}
		c.convDashes(stroke, s.Dashes)
		sv := stroke.intoModel()
		// Port: velato gives a gradient stroke the opacity 100. okage takes
		// its opacity, as ThorVG does (D43).
		return Draw{Stroke: &sv, Brush: c.convGradient(&s.BaseGradientShape).intoModel(), Opacity: c.gradientOpacity(&s.Opacity)}, true
	}
	return Draw{}, false
}

// convShape is conv_shape in velato src/import/converters.rs.
func (c *converter) convShape(value *AnyShape) (Shape, bool) {
	if draw, ok := c.convDraw(value); ok {
		return Shape{Kind: ShapeDraw, Draw: draw}, true
	}
	if geometry, ok := c.convGeometry(value); ok {
		return Shape{Kind: ShapeGeometry, Geometry: geometry}, true
	}
	switch {
	case value.Group != nil:
		var shapes []Shape
		var groupTransform *GroupTransform
		for i := range value.Group.Shapes {
			item := &value.Group.Shapes[i]
			if item.Transform != nil {
				gt := c.convShapeTransform(item.Transform)
				groupTransform = &gt
				continue
			}
			if s, ok := c.convShape(item); ok {
				shapes = append(shapes, s)
			}
		}
		if len(shapes) == 0 {
			return Shape{}, false
		}
		return Shape{Kind: ShapeGroup, Group: shapes, GroupTransform: groupTransform}, true
	case value.Repeater != nil:
		r := value.Repeater
		if r.Hidden != nil && *r.Hidden {
			return Shape{}, false
		}
		transform := &r.Transform.TransformSchema
		var position Position
		if transform.Position.IsSplit {
			position = Position{Kind: PositionSplit, X: c.convScalar(&transform.Position.SplitPosition.X), Y: c.convScalar(&transform.Position.SplitPosition.Y)}
		} else {
			position = c.convPosPoint(&transform.Position.Position)
		}
		rotation := &floatValueZero
		if transform.Rotation != nil {
			if transform.Rotation.IsSplit {
				rotation = &transform.Rotation.ZRotation
			} else {
				rotation = &transform.Rotation.Rotation
			}
		}
		composite := RepeaterBelow
		if r.Composite != nil && *r.Composite == CompositeAbove {
			composite = RepeaterAbove
		}
		repeater := &AnimatedRepeater{
			Copies:       c.convScalar(&r.Copies),
			Offset:       c.convScalar(orFloat(r.Offset, &floatValueZero)),
			AnchorPoint:  c.convPosPoint(orPosition(transform.AnchorPoint, &positionZero)),
			Position:     position,
			Rotation:     c.convScalar(rotation),
			Scale:        c.convVec2(orMulti(transform.Scale, &multidimOneHundred)),
			StartOpacity: c.convScalar(orFloat(r.Transform.StartOpacity, &floatValueOneHundred)),
			EndOpacity:   c.convScalar(orFloat(r.Transform.EndOpacity, &floatValueOneHundred)),
			Composite:    composite,
		}
		return Shape{Kind: ShapeRepeater, Repeater: repeater.intoModel()}, true
	case value.Merge != nil:
		// Port: velato skips merge paths. okage keeps them (D42). Mode 1
		// joins the paths above it into one operand of a later merge path,
		// and draws as velato draws a compound path.
		if c.velato {
			return Shape{}, false
		}
		mode := MergeModeNormal
		if m := value.Merge.MergeMode; m != nil {
			mode = *m
		}
		return Shape{Kind: ShapeMerge, Merge: mode}, true
	case value.Trim != nil:
		t := value.Trim
		mode := TrimParallel
		if t.Multiple != nil && *t.Multiple == TrimMultipleShapesSequential {
			mode = TrimSequential
		}
		trim := &AnimatedTrim{
			Mode:   mode,
			Start:  c.convScalar(&t.Start),
			End:    c.convScalar(&t.End),
			Offset: c.convScalar(&t.Offset),
		}
		return Shape{Kind: ShapeTrim, Trim: trim.intoModel()}, true
	}
	return Shape{}, false
}

// convGeometry is conv_geometry in velato src/import/converters.rs.
func (c *converter) convGeometry(value *AnyShape) (Geometry, bool) {
	switch {
	case value.Ellipse != nil:
		e := value.Ellipse
		return Geometry{Kind: GeometryEllipse, Ellipse: &AnimatedEllipse{
			IsCCW:    e.Direction != nil && *e.Direction == ShapeDirectionReversed,
			Position: c.convPosPoint(&e.Position),
			Size:     c.convSize(&e.Size),
		}}, true
	case value.Rectangle != nil:
		r := value.Rectangle
		return Geometry{Kind: GeometryRect, Rect: &AnimatedRect{
			IsCCW:        r.Direction != nil && *r.Direction == ShapeDirectionReversed,
			Position:     c.convPosPoint(&r.Position),
			Size:         c.convSize(&r.Size),
			CornerRadius: c.convScalar(&r.RoundedCornerRadius),
		}}, true
	case value.Path != nil:
		return c.convShapeGeometry(&value.Path.ShapeProperty)
	case value.PolyStar != nil:
		return c.convPolystarGeometry(value.PolyStar), true
	}
	return Geometry{}, false
}

// convPolystarGeometry is conv_polystar_geometry in velato
// src/import/converters.rs.
func (c *converter) convPolystarGeometry(value *PolyStarShape) Geometry {
	isPolygon := value.StarType == StarTypePolygon
	star := &AnimatedStar{
		IsPolygon:      isPolygon,
		Direction:      1.0,
		Position:       c.convPosPoint(&value.Position),
		InnerRadius:    fixedValue(0.0),
		InnerRoundness: fixedValue(0.0),
		OuterRadius:    c.convScalar(&value.OuterRadius),
		OuterRoundness: c.convScalar(&value.OuterRoundness),
		Rotation:       c.convScalar(&value.Rotation),
		Points:         c.convScalar(&value.Points),
	}
	if !isPolygon {
		if value.InnerRadius != nil {
			star.InnerRadius = c.convScalar(value.InnerRadius)
		}
		if value.InnerRoundness != nil {
			star.InnerRoundness = c.convScalar(value.InnerRoundness)
		}
	}
	return star.intoModel()
}

// convShapeGeometry is conv_shape_geometry in velato
// src/import/converters.rs.
func (c *converter) convShapeGeometry(value *ShapeProperty) (Geometry, bool) {
	if !value.Value.IsAnimated {
		points, isClosed := convSpline(&value.Value.Static)
		path := splineToPath(len(points), func(i int) kurbo.Point { return points[i] }, isClosed, nil)
		return Geometry{Kind: GeometryFixed, Path: path}, true
	}
	animated := value.Value.Keyframes
	isClosed := false
	var frames []Time
	var values []SplineKeyframeValues
	for idx := range animated {
		v := &animated[idx]
		frames = append(frames, Time{
			Frame:      v.Base.Time,
			InTangent:  c.convKeyframeHandle(v.Base.InTangent),
			OutTangent: c.convKeyframeHandle(v.Base.OutTangent),
			Hold:       v.Base.Hold.isTrue(),
		})
		var startBezier *Bezier
		if v.HasStart {
			if len(v.Start) == 0 {
				return Geometry{}, false
			}
			startBezier = &v.Start[0]
		} else {
			if idx == 0 {
				return Geometry{}, false
			}
			prev := &animated[idx-1]
			switch {
			case prev.HasEnd && len(prev.End) > 0:
				startBezier = &prev.End[0]
			case prev.HasStart && len(prev.Start) > 0:
				startBezier = &prev.Start[0]
			default:
				return Geometry{}, false
			}
		}
		points, frameClosed := convSpline(startBezier)
		var endPoints []kurbo.Point
		var endClosed bool
		if v.HasEnd && len(v.End) > 0 {
			endPoints, endClosed = convSpline(&v.End[0])
			isClosed = isClosed || endClosed
		}
		values = append(values, SplineKeyframeValues{Start: points, End: endPoints, StartClosed: frameClosed, EndClosed: endClosed})
		isClosed = isClosed || frameClosed
	}
	return Geometry{Kind: GeometrySpline, Spline: &AnimatedSpline{IsClosed: isClosed, Times: frames, Values: values}}, true
}

// convSpline is conv_spline in velato src/import/converters.rs: three
// points for each vertex, the vertex, its in tangent and its out tangent.
func convSpline(value *Bezier) ([]kurbo.Point, bool) {
	points := make([]kurbo.Point, 0, len(value.Vertices)*3)
	isClosed := value.Closed != nil && *value.Closed
	for i, v := range value.Vertices {
		var in, out [2]float64
		if i < len(value.InTangents) {
			in = value.InTangents[i]
		}
		if i < len(value.OutTangents) {
			out = value.OutTangents[i]
		}
		points = append(points, kurbo.NewPoint(v[0], v[1]), kurbo.NewPoint(in[0], in[1]), kurbo.NewPoint(out[0], out[1]))
	}
	return points, isClosed
}

// convBlendMode is conv_blend_mode in velato src/import/converters.rs.
func (c *converter) convBlendMode(value BlendModeSchema) *BlendMode {
	if value == BlendModeNormal {
		return nil
	}
	if value == BlendModeAdd || value == BlendModeHardMix {
		c.failf("the blend mode %d", value)
		return nil
	}
	m := blendMix(Mix(value))
	return &m
}

// convScalar is conv_scalar in velato src/import/converters.rs.
func (c *converter) convScalar(value *FloatValue) Value[float64] {
	p := value.Value
	if !p.IsAnimated {
		return fixedValue(p.Static)
	}
	var frames []Time
	var values []float64
	for i := range p.Keyframes {
		k := &p.Keyframes[i]
		if len(k.Value) == 0 {
			c.failf("a keyframe with no value")
			values = append(values, 0)
		} else {
			values = append(values, k.Value[0])
		}
		frames = append(frames, Time{
			Frame:      k.Base.Time,
			InTangent:  c.convKeyframeHandle(k.Base.InTangent),
			OutTangent: c.convKeyframeHandle(k.Base.OutTangent),
			Hold:       k.Base.Hold.isTrue(),
		})
	}
	return newAnimated(frames, values, tweenF64)
}

// convMulti is conv_multi in velato src/import/converters.rs.
func convMulti[T any](p AnimatedPropertyK[[]float64], f func([]float64) T, tween func(a, b T, t float64, e Easing) T) Value[T] {
	if !p.IsAnimated {
		return fixedValue(f(p.Static))
	}
	return convKeyframes(p.Keyframes, func(k *Keyframe) T { return f(k.Value) }, tween)
}

// pointOf is the function point of conv_pos_point in velato
// src/import/converters.rs.
func pointOf(v []float64) kurbo.Point {
	if len(v) < 2 {
		return kurbo.Point{}
	}
	return kurbo.NewPoint(v[0], v[1])
}

// convPosPoint is conv_pos_point in velato src/import/converters.rs.
func (c *converter) convPosPoint(value *PositionSchema) Position {
	if !value.Value.IsAnimated {
		return Position{Kind: PositionValue, Value: fixedValue(pointOf(value.Value.Static))}
	}
	keyframes := value.Value.Keyframes
	spatial := false
	for _, k := range keyframes {
		if k.ValueInTangent != nil || k.ValueOutTangent != nil {
			spatial = true
			break
		}
	}
	if !spatial {
		frames := make([]Keyframe, len(keyframes))
		for i := range keyframes {
			frames[i] = keyframes[i].Keyframe
		}
		return Position{Kind: PositionValue, Value: convKeyframes(frames, func(k *Keyframe) kurbo.Point { return pointOf(k.Value) }, tweenPoint)}
	}
	temporal := func(h *KeyframeBezierHandle) *EasingHandle {
		if h == nil {
			return nil
		}
		first := func(k KeyframeComponent) (float64, bool) {
			if !k.IsArray {
				return k.Single, true
			}
			if len(k.Values) == 0 {
				return 0, false
			}
			return k.Values[0], true
		}
		x, okx := first(h.XCoordinate)
		y, oky := first(h.YCoordinate)
		if !okx || !oky {
			return nil
		}
		return &EasingHandle{X: x, Y: y}
	}
	spatialKeyframes := make([]SpatialKeyframe, len(keyframes))
	for i := range keyframes {
		k := &keyframes[i]
		base := &k.Base
		sk := SpatialKeyframe{
			Time: Time{
				Frame:      base.Time,
				InTangent:  temporal(base.InTangent),
				OutTangent: temporal(base.OutTangent),
				Hold:       base.Hold.isTrue(),
			},
			Value: pointOf(k.Value),
		}
		if k.ValueOutTangent != nil {
			v := pointOf(k.ValueOutTangent).ToVec2()
			sk.OutTangent = &v
		}
		if k.ValueInTangent != nil {
			v := pointOf(k.ValueInTangent).ToVec2()
			sk.InTangent = &v
		}
		spatialKeyframes[i] = sk
	}
	return Position{Kind: PositionSpatial, Spatial: NewSpatialPosition(spatialKeyframes)}
}

// convMultiPoint is conv_multi_point in velato src/import/converters.rs.
func (c *converter) convMultiPoint(value *MultiDimensional) Value[kurbo.Point] {
	return convMulti(value.Value, pointOf, tweenPoint)
}

// convColor is conv_color in velato src/import/converters.rs.
func (c *converter) convColor(value *ColorValue) Value[Color] {
	return convMulti(value.Value, func(x []float64) Color {
		if len(x) < 3 {
			return Color{0, 0, 0, 1}
		}
		return Color{float32(x[0]), float32(x[1]), float32(x[2]), 1.0}
	}, tweenColor)
}

// convVec2 is conv_vec2 in velato src/import/converters.rs.
func (c *converter) convVec2(value *MultiDimensional) Value[kurbo.Vec2] {
	return convMulti(value.Value, func(x []float64) kurbo.Vec2 { return pointOf(x).ToVec2() }, tweenVec2)
}

// convSize is conv_size in velato src/import/converters.rs.
func (c *converter) convSize(value *MultiDimensional) Value[kurbo.Size] {
	return convMulti(value.Value, func(x []float64) kurbo.Size {
		p := pointOf(x)
		return kurbo.NewSize(p.X, p.Y)
	}, tweenSize)
}

// normalizeToRange is normalize_to_range in velato src/import/converters.rs.
func normalizeToRange(a, b, x float64) float64 {
	if a == b {
		// Avoid division by zero if a and b are the same.
		return 0.0
	}
	return (x - a) / (b - a)
}

// rustMin is f64::min: the smaller value, or the other value when one is
// NaN.
func rustMin(a, b float64) float64 {
	switch {
	case a != a:
		return b
	case b != b:
		return a
	case b < a:
		return b
	}
	return a
}

// rustMax is f64::max: the larger value, or the other value when one is
// NaN.
func rustMax(a, b float64) float64 {
	switch {
	case a != a:
		return b
	case b != b:
		return a
	case b > a:
		return b
	}
	return a
}

// parseHexColor parses the color of a solid layer, "#rrggbb", and returns
// false for another form.
func parseHexColor(s string) (Color, bool) {
	if len(s) != 7 || s[0] != '#' {
		return Color{}, false
	}
	var v [3]float32
	for i := range 3 {
		var n int
		for _, ch := range s[1+2*i : 3+2*i] {
			n *= 16
			switch {
			case ch >= '0' && ch <= '9':
				n += int(ch - '0')
			case ch >= 'a' && ch <= 'f':
				n += int(ch-'a') + 10
			case ch >= 'A' && ch <= 'F':
				n += int(ch-'A') + 10
			default:
				return Color{}, false
			}
		}
		v[i] = float32(n) / 255
	}
	return Color{v[0], v[1], v[2], 1}, true
}
