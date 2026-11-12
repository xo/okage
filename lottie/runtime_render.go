// Copyright 2024 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports velato src/runtime/render.rs.

import (
	"slices"

	"github.com/xo/okage/internal/kurbo"
	"github.com/xo/okage/internal/lottiecompat"
)

// Sink is the trait RenderSink in velato src/runtime/render.rs: what a
// Renderer draws onto. A shape is a list of path elements. A layer and a
// clip layer last until the PopLayer that matches them.
type Sink interface {
	// PushLayer starts a layer that composites with blend and alpha, clipped
	// to shape under transform.
	PushLayer(blend BlendMode, alpha float32, transform kurbo.Affine, shape []kurbo.PathEl)
	// PushClipLayer starts a layer clipped to shape under transform.
	PushClipLayer(transform kurbo.Affine, shape []kurbo.PathEl)
	// PopLayer ends the last layer.
	PopLayer()
	// PushFilter starts a layer that applies effect, and returns
	// FilterPushed, or returns FilterUnsupported and starts no layer.
	PushFilter(transform kurbo.Affine, effect *FilterEffect) FilterLayerResult
	// Draw fills shape, or strokes it when stroke is not nil, with brush
	// under transform.
	Draw(stroke *Stroke, transform kurbo.Affine, brush *Brush, shape []kurbo.PathEl)
	// DrawImage draws a raster image asset.
	DrawImage(image *ImageAsset, transform kurbo.Affine, alpha float64)
	// BeginLayerGroup is called before a Lottie layer, with its name and
	// its index in its composition.
	BeginLayerGroup(name string, index int)
	// EndLayerGroup is called after a Lottie layer.
	EndLayerGroup()
}

// Renderer is Renderer in velato src/runtime/render.rs: it renders a
// composition onto a Sink.
type Renderer struct {
	// MergePaths draws the merge paths of modes 2 to 5 by compositing, as
	// After Effects combines them (D42). It is false by default, because
	// ThorVG and velato draw a merge path as a compound path, and ThorVG is
	// the reference of the tests (D43).
	MergePaths bool

	batch        batch
	maskElements []kurbo.PathEl
}

// NewRenderer is Renderer::new in velato src/runtime/render.rs.
func NewRenderer() *Renderer {
	return &Renderer{}
}

// Append is Renderer::try_append in velato src/runtime/render.rs: it
// renders the animation at frame onto sink. It returns the error of the
// evaluation, and then leaves sink unchanged.
//
// Port: velato's append panics on an evaluation error. Go returns the error
// (D41).
func (r *Renderer) Append(animation *Composition, frame float64, transform kurbo.Affine, alpha float64, sink Sink) error {
	evaluated, err := animation.Evaluate(frame)
	if err != nil {
		return err
	}
	r.AppendEvaluated(evaluated, transform, alpha, sink)
	return nil
}

// AppendEvaluated is Renderer::append_evaluated in velato
// src/runtime/render.rs: it draws a saved evaluation.
func (r *Renderer) AppendEvaluated(evaluated *EvaluatedComposition, transform kurbo.Affine, alpha float64, sink Sink) {
	animation := evaluated.composition
	r.batch.clear()
	clip := kurbo.NewRect(0.0, 0.0, float64(animation.Width), float64(animation.Height))
	clipBounds := transform.TransformRectBbox(clip)
	sink.PushClipLayer(transform, clip.PathElements(0.1))
	for layerIndex := len(evaluated.Layers) - 1; layerIndex >= 0; layerIndex-- {
		layer := &evaluated.Layers[layerIndex]
		if !layer.Visible {
			continue
		}
		r.renderLayer(animation, evaluated.Layers, layer, layerIndex, transform, alpha, clipBounds, sink)
	}
	sink.PopLayer()
}

// renderLayer is Renderer::render_layer in velato src/runtime/render.rs.
func (r *Renderer) renderLayer(animation *Composition, layerSet []EvaluatedLayer, evaluated *EvaluatedLayer, layerIndex int, outputTransform kurbo.Affine, alpha float64, clipBounds kurbo.Rect, sink Sink) {
	if !evaluated.InRange {
		return
	}
	layer := evaluated.Layer
	frame := evaluated.Frame
	sink.BeginLayerGroup(layer.Name, layerIndex)
	parentTransform := outputTransform
	transform := outputTransform.Mul(evaluated.FullTransform)
	fullRect := kurbo.NewRect(0.0, 0.0, float64(animation.Width), float64(animation.Height)).PathElements(0.1)
	if m := evaluated.Matte; m != nil {
		sink.PushLayer(blendMix(MixNormal), 1.0, parentTransform, fullRect)
		// Port: ThorVG multiplies the colors of a matte source with the
		// blend mode multiply into the empty matte, which gives an empty
		// matte. okage leaves the matte empty for such a source (D43).
		source := layerSet[m.Source].Layer
		multiply := source.BlendMode != nil && source.BlendMode.Mix == MixMultiply
		if !multiply || lottiecompat.Velato {
			r.renderLayer(animation, layerSet, &layerSet[m.Source], m.Source, outputTransform, alpha, clipBounds, sink)
		}
		sink.PushLayer(m.Mode, 1.0, parentTransform, fullRect)
	}
	alpha *= evaluated.Opacity

	// Important for isolating opacity so it applies to the layer effect
	// output.
	//
	// Port: velato isolates the opacity only for a layer with effects, and
	// else multiplies it into each draw, so that overlapping shapes add up.
	// okage isolates the opacity of each layer that has content, so that it
	// applies to the layer as a whole, as ThorVG does (D43).
	isolateOpacity := alpha != 1.0 && (len(layer.Effects) > 0 || layer.Content.Kind != ContentNone)
	if lottiecompat.Velato {
		isolateOpacity = len(layer.Effects) > 0 && alpha != 1.0
	}
	// Port: velato reads the blend mode of a layer, and does not use it.
	// okage composites the layer with its blend mode, as ThorVG does (D43).
	if layer.BlendMode != nil && layer.Content.Kind != ContentNone && !lottiecompat.Velato {
		isolateOpacity = true
	}
	if isolateOpacity {
		mode := blendMix(MixNormal)
		if layer.BlendMode != nil && !lottiecompat.Velato {
			mode = *layer.BlendMode
		}
		sink.PushLayer(mode, float32(alpha), parentTransform, fullRect)
		alpha = 1.0
	}
	filterLayers := 0
	if isolateOpacity {
		filterLayers = 1
	}

	for i := len(layer.Effects) - 1; i >= 0; i-- {
		effect, ok := layer.Effects[i].Evaluate(frame)
		if !ok {
			continue
		}
		if sink.PushFilter(transform, &effect) == FilterPushed {
			filterLayers++
		}
	}
	// Port: velato pushes a clip for each mask, so that two masks that add
	// give their intersection, not their union. okage keeps the clip for one
	// mask that adds, and draws any other masks by compositing after the
	// content (D43).
	clipMasks := len(layer.Masks) <= 1 || lottiecompat.Velato
	for i := range layer.Masks {
		if !layer.Masks[i].isClip() && !lottiecompat.Velato {
			clipMasks = false
		}
	}
	if clipMasks {
		for i := range layer.Masks {
			r.maskElements = layer.Masks[i].Geometry.Evaluate(frame, r.maskElements[:0])
			sink.PushClipLayer(transform, slices.Clone(r.maskElements))
			r.maskElements = r.maskElements[:0]
		}
	} else {
		sink.PushLayer(blendMix(MixNormal), 1.0, kurbo.IdentityAffine, clipBounds.PathElements(0.1))
	}
	switch layer.Content.Kind {
	case ContentInstance:
		for index := len(evaluated.Children) - 1; index >= 0; index-- {
			child := &evaluated.Children[index]
			if child.Layer.IsMatteSource || child.Layer.Hidden {
				continue
			}
			r.renderLayer(animation, evaluated.Children, child, index, outputTransform, alpha, clipBounds, sink)
		}
	case ContentImage:
		if image, ok := animation.Images[layer.Content.Name]; ok {
			// Lottie requires visuals to stay within authored image bounds.
			bounded := image.Width != nil && image.Height != nil && *image.Width > 0.0 && *image.Height > 0.0
			if bounded {
				sink.PushClipLayer(transform, kurbo.NewRect(0.0, 0.0, *image.Width, *image.Height).PathElements(0.1))
			}
			sink.DrawImage(&image, transform, alpha)
			if bounded {
				sink.PopLayer()
			}
		}
	case ContentSolid:
		// Port: velato draws nothing for a solid layer. okage fills its
		// rectangle with its color (D43).
		brush := solidBrush(layer.Content.Color.MultiplyAlpha(float32(alpha)))
		sink.Draw(nil, transform, &brush, kurbo.NewRect(0, 0, layer.Content.Width, layer.Content.Height).PathElements(0.1))
	case ContentShape:
		r.batch.mergePaths = r.MergePaths
		r.batch.evaluate(layer.Content.Shapes, transform, alpha, frame, true)
		r.batch.render(sink, clipBounds)
		r.batch.clear()
	}
	pops := len(layer.Masks) + filterLayers
	if !clipMasks {
		r.renderMasks(layer.Masks, frame, transform, clipBounds, sink)
		pops = 1 + filterLayers
	}
	if evaluated.Matte != nil {
		pops += 2
	}
	for range pops {
		sink.PopLayer()
	}
	sink.EndLayerGroup()
}

// renderMasks draws the coverage of masks in a layer with the blend mode
// destination-in, so that it cuts the content of the layer below (D43). It
// ports no velato function. The masks combine in their order: a mask that
// adds draws its shape, one that subtracts takes it out, one that
// intersects keeps the common area, and one of the mode difference keeps the
// area that one of them covers. A first mask that subtracts starts from the
// whole canvas. An inverted mask uses the canvas without its shape. A
// positive expansion grows the shape by a round stroke, and a negative one
// shrinks it. The modes lighten and darken work as add and intersect.
func (r *Renderer) renderMasks(masks []Mask, frame float64, transform kurbo.Affine, clipBounds kurbo.Rect, sink Sink) {
	full := clipBounds.PathElements(0.1)
	effective := false
	for i := range masks {
		if masks[i].MaskMode != MaskModeNone {
			effective = true
		}
	}
	if !effective {
		// Masks of the mode none do not mask the content.
		return
	}
	sink.PushLayer(blendCompose(ComposeDestIn), 1.0, kurbo.IdentityAffine, full)
	first := true
	for i := range masks {
		m := &masks[i]
		var compose Compose
		switch m.MaskMode {
		case MaskModeNone:
			continue
		case MaskModeSubtract:
			compose = ComposeDestOut
			if first {
				opaque := solidBrush(ColorBlack)
				sink.Draw(nil, kurbo.IdentityAffine, &opaque, full)
			}
		case MaskModeIntersect, MaskModeDarken:
			compose = ComposeDestIn
			if first {
				compose = ComposeSrcOver
			}
		case MaskModeDifference:
			compose = ComposeXor
		default:
			compose = ComposeSrcOver
		}
		first = false
		alpha := clampUpTo(m.Opacity.Evaluate(frame)/100.0, 1)
		shape := m.Geometry.Evaluate(frame, nil)
		expand := m.Expand.Evaluate(frame)
		opaque := solidBrush(ColorBlack)
		sink.PushLayer(blendCompose(compose), float32(alpha), kurbo.IdentityAffine, full)
		if m.Inverted {
			sink.Draw(nil, kurbo.IdentityAffine, &opaque, full)
			sink.PushLayer(blendCompose(ComposeDestOut), 1.0, kurbo.IdentityAffine, full)
		}
		sink.Draw(nil, transform, &opaque, slices.Clone(shape))
		if expand > 0 {
			stroke := newStroke(2 * expand)
			sink.Draw(&stroke, transform, &opaque, slices.Clone(shape))
		} else if expand < 0 {
			stroke := newStroke(-2 * expand)
			sink.PushLayer(blendCompose(ComposeDestOut), 1.0, kurbo.IdentityAffine, full)
			sink.Draw(&stroke, transform, &opaque, shape)
			sink.PopLayer()
		}
		if m.Inverted {
			sink.PopLayer()
		}
		sink.PopLayer()
	}
	sink.PopLayer()
}

// drawData is DrawData in velato src/runtime/render.rs.
type drawData struct {
	stroke    *Stroke
	brush     Brush
	alpha     float64
	transform kurbo.Affine
	// Range into batch.geometries.
	geometry [2]int
	// Only repeaters that copied this style can share its opacity layers
	// with other styles.
	repeaterDepth int
}

// newDrawData is DrawData::new in velato src/runtime/render.rs.
func newDrawData(draw *Draw, alpha float64, geometry [2]int, frame float64) drawData {
	d := drawData{
		brush:     draw.Brush.Evaluate(1.0, frame),
		alpha:     alpha * draw.Opacity.Evaluate(frame) / 100.0,
		transform: kurbo.IdentityAffine,
		geometry:  geometry,
	}
	if draw.Stroke != nil {
		s := draw.Stroke.Evaluate(frame)
		d.stroke = &s
	}
	return d
}

// geometryData is GeometryData in velato src/runtime/render.rs.
type geometryData struct {
	// Range into batch.elements.
	elements  [2]int
	transform kurbo.Affine
	// Sequential trimming follows copy indices independently of paint
	// stacking.
	trimOrder int
	// Inherited styles need copy opacity without inheriting the repeated
	// styles' stacking position.
	copies []copyLayer
	// Port: the merge path that combines this geometry, with mergeID 0 for
	// none, and the operand of the merge that it is part of (D42).
	mergeID   int
	mergeMode MergeMode
	operand   int
}

// opacityCopies is GeometryData::opacity_copies in velato
// src/runtime/render.rs.
func (g *geometryData) opacityCopies() []copyLayer {
	end := 0
	for i, v := range slices.Backward(g.copies) {
		if v.alpha != 1.0 {
			end = i + 1
			break
		}
	}
	return g.copies[:end]
}

// copyLayer is CopyLayer in velato src/runtime/render.rs.
type copyLayer struct {
	id    int
	alpha float64
}

// batch is Batch in velato src/runtime/render.rs.
type batch struct {
	elements         []kurbo.PathEl
	geometries       []geometryData
	draws            []drawData
	repeatGeometries []geometryData
	repeatDraws      []drawData
	nextCopy         int
	trimElements     []kurbo.PathEl
	// Port: the counters of the merge paths and of their operands, and
	// whether the batch draws merge paths (D42).
	nextMerge   int
	nextOperand int
	mergePaths  bool
}

// evaluatePaths is evaluate_paths in velato src/runtime/render.rs.
func evaluatePaths(shapes []Shape, transform kurbo.Affine, frame float64) [][]kurbo.PathEl {
	var b batch
	b.evaluate(shapes, transform, 1.0, frame, false)
	var paths [][]kurbo.PathEl
	for _, g := range b.geometries {
		if g.elements[0] == g.elements[1] {
			continue
		}
		path := make([]kurbo.PathEl, 0, g.elements[1]-g.elements[0])
		for _, el := range b.elements[g.elements[0]:g.elements[1]] {
			path = append(path, g.transform.MulPathEl(el))
		}
		paths = append(paths, path)
	}
	return paths
}

// evaluate is Batch::evaluate in velato src/runtime/render.rs.
func (b *batch) evaluate(shapes []Shape, transform kurbo.Affine, alpha, frame float64, collectDraws bool) {
	geometryStart := len(b.geometries)
	drawStart := len(b.draws)
	for i := range shapes {
		shape := &shapes[i]
		switch shape.Kind {
		case ShapeGroup:
			groupTransform, groupAlpha := kurbo.IdentityAffine, 1.0
			if gt := shape.GroupTransform; gt != nil {
				groupTransform = gt.Transform.Evaluate(frame)
				if collectDraws {
					groupAlpha = gt.Opacity.Evaluate(frame) / 100.0
				}
			}
			groupStart := len(b.geometries)
			b.evaluate(shape.Group, groupTransform, alpha*groupAlpha, frame, collectDraws)
			// Port: a group is one operand of a merge path around it (D42).
			b.nextOperand++
			for i := groupStart; i < len(b.geometries); i++ {
				if b.geometries[i].mergeID == 0 {
					b.geometries[i].operand = b.nextOperand
				}
			}
		case ShapeGeometry:
			b.pushGeometry(&shape.Geometry, kurbo.IdentityAffine, frame)
			// Port: each geometry is one operand of a merge path (D42).
			b.nextOperand++
			b.geometries[len(b.geometries)-1].operand = b.nextOperand
		case ShapeMerge:
			// Port: a merge path combines the paths above it in its group
			// (D42). Mode 1 makes them one operand of a later merge path.
			if !b.mergePaths {
				continue
			}
			if shape.Merge == MergeModeNormal {
				b.nextOperand++
				for i := geometryStart; i < len(b.geometries); i++ {
					if b.geometries[i].mergeID == 0 {
						b.geometries[i].operand = b.nextOperand
					}
				}
				continue
			}
			b.nextMerge++
			for i := geometryStart; i < len(b.geometries); i++ {
				b.geometries[i].mergeID = b.nextMerge
				b.geometries[i].mergeMode = shape.Merge
			}
		case ShapeDraw:
			if collectDraws {
				b.pushDraw(&shape.Draw, alpha, geometryStart, frame)
			}
		case ShapeRepeater:
			repeater := shape.Repeater.Evaluate(frame)
			b.repeat(&repeater, geometryStart, drawStart)
		case ShapeTrim:
			trim := shape.Trim.Evaluate(frame)
			b.applyTrim(&trim, geometryStart)
			// Port: a trim path after a merge path needs the path of the
			// result, so the merge falls back to a compound path (D42).
			for i := geometryStart; i < len(b.geometries); i++ {
				b.geometries[i].mergeID = 0
			}
		}
	}
	for i := geometryStart; i < len(b.geometries); i++ {
		b.geometries[i].transform = transform.Mul(b.geometries[i].transform)
	}
	for i := drawStart; i < len(b.draws); i++ {
		b.draws[i].transform = transform.Mul(b.draws[i].transform)
	}
}

// pushGeometry is Batch::push_geometry in velato src/runtime/render.rs.
func (b *batch) pushGeometry(geometry *Geometry, transform kurbo.Affine, frame float64) {
	start := len(b.elements)
	b.elements = geometry.Evaluate(frame, b.elements)
	b.geometries = append(b.geometries, geometryData{
		elements:  [2]int{start, len(b.elements)},
		transform: transform,
		trimOrder: len(b.geometries),
	})
}

// pushDraw is Batch::push_draw in velato src/runtime/render.rs.
func (b *batch) pushDraw(draw *Draw, alpha float64, geometryStart int, frame float64) {
	b.draws = append(b.draws, newDrawData(draw, alpha, [2]int{geometryStart, len(b.geometries)}, frame))
}

// repeat is Batch::repeat in velato src/runtime/render.rs.
func (b *batch) repeat(repeater *FixedRepeater, geometryStart, drawStart int) {
	if repeater.Copies == 0 {
		b.geometries = b.geometries[:geometryStart]
		b.draws = b.draws[:drawStart]
		return
	}
	if geometryStart == len(b.geometries) {
		return
	}

	// First move the relevant ranges of geometries and draws into side
	// buffers.
	b.repeatGeometries = append(b.repeatGeometries, b.geometries[geometryStart:]...)
	b.geometries = b.geometries[:geometryStart]
	b.repeatDraws = append(b.repeatDraws, b.draws[drawStart:]...)
	b.draws = b.draws[:drawStart]
	geometryCount := len(b.repeatGeometries)
	startAlpha := repeater.StartOpacity / 100.0
	endAlpha := repeater.EndOpacity / 100.0
	deltaAlpha := 0.0
	if repeater.Copies > 1 {
		deltaAlpha = (endAlpha - startAlpha) / float64(repeater.Copies-1)
	}
	index := func(copy int) int {
		if repeater.Composite == RepeaterBelow {
			return repeater.Copies - 1 - copy
		}
		return copy
	}
	// Later styles traverse geometry directly, so it must be in paint order
	// too.
	for copy := range repeater.Copies {
		i := index(copy)
		transform := repeater.Transform(i)
		layer := copyLayer{id: b.nextCopy, alpha: startAlpha + deltaAlpha*float64(i)}
		b.nextCopy++
		for _, g := range b.repeatGeometries {
			g.trimOrder += i * geometryCount
			g.transform = transform.Mul(g.transform)
			g.copies = append([]copyLayer{layer}, g.copies...)
			b.geometries = append(b.geometries, g)
		}
	}

	// Draws are consumed in reverse, unlike geometries.
	for copy := repeater.Copies - 1; copy >= 0; copy-- {
		copyStart := geometryStart + copy*geometryCount
		transform := repeater.Transform(index(copy))
		for _, d := range b.repeatDraws {
			start := d.geometry[0] - geometryStart
			end := d.geometry[1] - geometryStart
			d.geometry = [2]int{copyStart + start, copyStart + end}
			d.transform = transform.Mul(d.transform)
			d.repeaterDepth++
			b.draws = append(b.draws, d)
		}
	}
	// Clear the side buffers.
	b.repeatGeometries = b.repeatGeometries[:0]
	b.repeatDraws = b.repeatDraws[:0]
}

// applyTrim is Batch::apply_trim in velato src/runtime/render.rs.
func (b *batch) applyTrim(trim *FixedTrim, geometryStart int) {
	first, second, ok := trim.Normalized()
	if !ok {
		for i := geometryStart; i < len(b.geometries); i++ {
			b.geometries[i].elements = [2]int{0, 0}
		}
		return
	}
	if first.Start <= 1e-9 && first.End >= 1.0-1e-9 && second == nil {
		return
	}

	for _, g := range b.geometries[:geometryStart] {
		b.trimElements = append(b.trimElements, b.elements[g.elements[0]:g.elements[1]]...)
	}

	sequential := trim.Mode == TrimSequential
	tail := b.geometries[geometryStart:]
	paths := make([][]kurbo.PathEl, len(tail))
	for i, g := range tail {
		for _, el := range b.elements[g.elements[0]:g.elements[1]] {
			if !sequential {
				el = g.transform.MulPathEl(el)
			}
			paths[i] = append(paths[i], el)
		}
	}
	intervals := make([][2]float64, len(paths))
	if sequential {
		lengths := make([]float64, len(paths))
		total := 0.0
		for i, p := range paths {
			lengths[i] = pathLength(p)
			total += lengths[i]
		}
		order := make([]int, len(paths))
		for i := range order {
			order[i] = i
		}
		slices.SortFunc(order, func(a, c int) int { return tail[a].trimOrder - tail[c].trimOrder })
		offset := 0.0
		for _, index := range order {
			intervals[index] = [2]float64{offset, total}
			offset += lengths[index]
		}
	}
	ranges := []trimRange{first}
	if second != nil {
		ranges = append(ranges, *second)
	}
	for i := range tail {
		newStart := len(b.trimElements)
		var seq *[2]float64
		if sequential {
			seq = &intervals[i]
		}
		b.trimElements = trimPath(paths[i], ranges, seq, b.trimElements)
		tail[i].elements = [2]int{newStart, len(b.trimElements)}
		if !sequential {
			tail[i].transform = kurbo.IdentityAffine
		}
	}

	offset := 0
	for i := range b.geometries[:geometryStart] {
		n := b.geometries[i].elements[1] - b.geometries[i].elements[0]
		b.geometries[i].elements = [2]int{offset, offset + n}
		offset += n
	}

	b.elements, b.trimElements = b.trimElements, b.elements
	b.trimElements = b.trimElements[:0]
}

// copiesEqual reports if two lists of copy layers are equal.
func copiesEqual(a, b []copyLayer) bool {
	return slices.Equal(a, b)
}

// render is Batch::render in velato src/runtime/render.rs.
func (b *batch) render(sink Sink, clipBounds kurbo.Rect) {
	var activeCopies []copyLayer
	activeDraw := -1
	activeRepeaterDepth := 0
	var path []kurbo.PathEl
	clip := clipBounds.PathElements(0.1)

	for drawIndex := len(b.draws) - 1; drawIndex >= 0; drawIndex-- {
		draw := &b.draws[drawIndex]
		if draw.alpha <= 0.0 || !draw.transform.IsFinite() {
			continue
		}
		styleInverse := draw.transform.Inverse()
		// A singular style transform collapses both fill and stroke to zero
		// area.
		if !styleInverse.IsFinite() {
			continue
		}
		// Compound paths must be assembled after group transforms and
		// modifiers are resolved.
		geometries := b.geometries[draw.geometry[0]:draw.geometry[1]]
		for len(geometries) > 0 {
			n := 1
			for n < len(geometries) && copiesEqual(geometries[n-1].opacityCopies(), geometries[n].opacityCopies()) {
				n++
			}
			chunk := geometries[:n]
			geometries = geometries[n:]

			copies := chunk[0].opacityCopies()
			skip := false
			for _, c := range copies {
				if c.alpha <= 0.0 {
					skip = true
					break
				}
			}
			if skip {
				continue
			}

			shared := 0
			for shared < len(activeCopies) && shared < len(copies) {
				active, next := activeCopies[shared], copies[shared]
				if !(active.id == next.id && (activeDraw == drawIndex || (shared < activeRepeaterDepth && shared < draw.repeaterDepth))) {
					break
				}
				shared++
			}
			for i := len(activeCopies) - 1; i >= shared; i-- {
				if activeCopies[i].alpha != 1.0 {
					sink.PopLayer()
				}
			}
			for _, c := range copies[shared:] {
				if c.alpha != 1.0 {
					sink.PushLayer(blendMix(MixNormal), float32(c.alpha), kurbo.IdentityAffine, clip)
				}
			}
			activeCopies = copies
			activeDraw = drawIndex
			activeRepeaterDepth = draw.repeaterDepth
			path = b.renderDraw(draw, styleInverse, chunk, path, sink, clipBounds)
		}
	}
	for _, activeCopie := range slices.Backward(activeCopies) {
		if activeCopie.alpha != 1.0 {
			sink.PopLayer()
		}
	}
}

// renderDraw is Batch::render_draw in velato src/runtime/render.rs. It
// returns path, the buffer that it reuses.
func (b *batch) renderDraw(draw *drawData, styleInverse kurbo.Affine, geometries []geometryData, path []kurbo.PathEl, sink Sink, clipBounds kurbo.Rect) []kurbo.PathEl {
	brush := draw.brush
	if draw.alpha != 1.0 {
		brush = draw.brush.MultiplyAlpha(float32(draw.alpha))
	}
	if draw.stroke == nil && geometries[0].mergeID != 0 {
		allMerged := true
		for _, g := range geometries {
			if g.mergeID != geometries[0].mergeID {
				allMerged = false
				break
			}
		}
		if allMerged {
			return b.renderMerge(draw, styleInverse, geometries, path, sink, clipBounds, &brush)
		}
	}
	g := &geometries[0]
	var shape []kurbo.PathEl
	if len(geometries) == 1 && g.transform == draw.transform {
		shape = b.elements[g.elements[0]:g.elements[1]]
	} else {
		path = path[:0]
		for _, g := range geometries {
			elements := b.elements[g.elements[0]:g.elements[1]]
			if g.transform == draw.transform {
				path = append(path, elements...)
			} else {
				transform := styleInverse.Mul(g.transform)
				if transform.IsFinite() {
					for _, el := range elements {
						path = append(path, transform.MulPathEl(el))
					}
				}
			}
		}
		shape = path
	}
	if len(shape) > 0 {
		sink.Draw(draw.stroke, draw.transform, &brush, slices.Clone(shape))
	}
	return path
}

// operandPath appends the elements of geometries to path, in the
// coordinates of the style of draw, as renderDraw assembles a compound path.
func (b *batch) operandPath(draw *drawData, styleInverse kurbo.Affine, geometries []geometryData, path []kurbo.PathEl) []kurbo.PathEl {
	for _, g := range geometries {
		elements := b.elements[g.elements[0]:g.elements[1]]
		if g.transform == draw.transform {
			path = append(path, elements...)
			continue
		}
		transform := styleInverse.Mul(g.transform)
		if transform.IsFinite() {
			for _, el := range elements {
				path = append(path, transform.MulPathEl(el))
			}
		}
	}
	return path
}

// renderMerge draws a fill of the result of a merge path by compositing
// (D42). It pushes a layer, draws the first operand as opaque coverage,
// draws each next operand in a layer with the blend mode of the merge, fills
// the bounds with brush in a layer with the blend mode source-in, and pops
// the layer. It ports no velato function.
func (b *batch) renderMerge(draw *drawData, styleInverse kurbo.Affine, geometries []geometryData, path []kurbo.PathEl, sink Sink, clipBounds kurbo.Rect, brush *Brush) []kurbo.PathEl {
	clip := clipBounds.PathElements(0.1)
	coverage := solidBrush(ColorBlack)
	var compose Compose
	switch geometries[0].mergeMode {
	case MergeModeSubtract:
		compose = ComposeDestOut
	case MergeModeIntersect:
		compose = ComposeDestIn
	case MergeModeExcludeIntersections:
		compose = ComposeXor
	default:
		compose = ComposeSrcOver
	}
	sink.PushLayer(blendMix(MixNormal), 1.0, kurbo.IdentityAffine, clip)
	for first, start := true, 0; start < len(geometries); first = false {
		end := start + 1
		for end < len(geometries) && geometries[end].operand == geometries[start].operand {
			end++
		}
		path = b.operandPath(draw, styleInverse, geometries[start:end], path[:0])
		start = end
		if len(path) == 0 {
			continue
		}
		if first || compose == ComposeSrcOver {
			sink.Draw(nil, draw.transform, &coverage, slices.Clone(path))
			continue
		}
		sink.PushLayer(blendCompose(compose), 1.0, kurbo.IdentityAffine, clip)
		sink.Draw(nil, draw.transform, &coverage, slices.Clone(path))
		sink.PopLayer()
	}
	// The bounds of the clip, in the coordinates of the style.
	inv := styleInverse
	bounds := []kurbo.PathEl{
		kurbo.MoveTo(inv.MulPoint(kurbo.NewPoint(clipBounds.X0, clipBounds.Y0))),
		kurbo.LineTo(inv.MulPoint(kurbo.NewPoint(clipBounds.X1, clipBounds.Y0))),
		kurbo.LineTo(inv.MulPoint(kurbo.NewPoint(clipBounds.X1, clipBounds.Y1))),
		kurbo.LineTo(inv.MulPoint(kurbo.NewPoint(clipBounds.X0, clipBounds.Y1))),
		kurbo.ClosePath(),
	}
	sink.PushLayer(blendCompose(ComposeSrcIn), 1.0, kurbo.IdentityAffine, clip)
	sink.Draw(nil, draw.transform, brush, bounds)
	sink.PopLayer()
	sink.PopLayer()
	return path
}

// clear is Batch::clear in velato src/runtime/render.rs.
func (b *batch) clear() {
	b.elements = b.elements[:0]
	b.geometries = b.geometries[:0]
	b.draws = b.draws[:0]
	b.repeatGeometries = b.repeatGeometries[:0]
	b.repeatDraws = b.repeatDraws[:0]
	b.nextCopy = 0
	b.trimElements = b.trimElements[:0]
	b.nextMerge = 0
	b.nextOperand = 0
}
