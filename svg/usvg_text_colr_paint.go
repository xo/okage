// Copyright 2024 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"cmp"
	"math"
	"slices"

	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype/tables"

	"github.com/xo/okage/internal/crmath"
)

// This file ports no file of usvg. usvg src/text/flatten.rs paints a COLR
// glyph with ColorGlyph::paint of skrifa 0.44, which walks the paint graph
// of the glyph and calls the methods of the trait ColorPainter, which
// GlyphPainter of usvg src/text/colr.rs implements. go-text parses the
// tables COLR and CPAL, but it does not walk the graph (D9). This file walks
// it as skrifa does (D32): ColorGlyph::paint of src/color/mod.rs,
// traverse.rs, instance.rs and transform.rs of src/color. Each item names
// the item of skrifa that it replaces.

// colrTransform is skrifa::color::Transform, Matrix<f32> of font-types: an
// affine transform with the elements xx, yx, xy, yy, dx and dy.
type colrTransform struct {
	xx float32
	yx float32
	xy float32
	yy float32
	dx float32
	dy float32
}

// identityColrTransform is the impl Default for Matrix in font-types, the
// identity.
func identityColrTransform() colrTransform {
	return colrTransform{xx: 1, yy: 1}
}

// mul is the impl Mul for Matrix in font-types: t applied after rhs.
func (t colrTransform) mul(rhs colrTransform) colrTransform {
	return colrTransform{
		xx: float32(t.xx*rhs.xx) + float32(t.xy*rhs.yx),
		yx: float32(t.yx*rhs.xx) + float32(t.yy*rhs.yx),
		xy: float32(t.xx*rhs.xy) + float32(t.xy*rhs.yy),
		yy: float32(t.yx*rhs.xy) + float32(t.yy*rhs.yy),
		dx: float32(t.xx*rhs.dx) + float32(t.xy*rhs.dy) + t.dx,
		dy: float32(t.yx*rhs.dx) + float32(t.yy*rhs.dy) + t.dy,
	}
}

// colrPoint is Point<f32> of font-types.
type colrPoint struct {
	x float32
	y float32
}

// colrBoundingBox is BoundingBox<f32> of font-types.
type colrBoundingBox struct {
	xMin float32
	yMin float32
	xMax float32
	yMax float32
}

// colrColorStop is skrifa::color::ColorStop.
type colrColorStop struct {
	offset       float32
	paletteIndex uint16
	alpha        float32
}

// colrBrushKind is the case of a colrBrush.
type colrBrushKind uint8

// The cases of skrifa::color::Brush.
const (
	colrBrushSolid colrBrushKind = iota
	colrBrushLinearGradient
	colrBrushRadialGradient
	colrBrushSweepGradient
)

// colrBrush is skrifa::color::Brush. The enum has data, so it becomes a
// struct with a kind (D27). Each case uses the fields that its name gives in
// upstream.
type colrBrush struct {
	kind         colrBrushKind
	paletteIndex uint16
	alpha        float32
	p0           colrPoint
	p1           colrPoint
	c0           colrPoint
	r0           float32
	c1           colrPoint
	r1           float32
	startAngle   float32
	endAngle     float32
	colorStops   []colrColorStop
	extend       tables.Extend
}

// colorPainter is the trait skrifa::color::ColorPainter, with the methods
// that usvg and the traversal call.
type colorPainter interface {
	pushTransform(transform colrTransform)
	popTransform()
	fillGlyph(glyphID GlyphID, brushTransform *colrTransform, brush colrBrush)
	pushClipGlyph(glyphID GlyphID)
	pushClipBox(clipBox colrBoundingBox)
	popClip()
	fill(brush colrBrush)
	pushLayer(compositeMode tables.CompositeMode)
	popLayer()
}

// colrInstance is ColrInstance of skrifa src/color/instance.rs: the table
// COLR at a location in the variation space of the font.
type colrInstance struct {
	colr   *tables.COLR1
	coords []tables.Coord
}

// varDeltas is ColrInstance::var_deltas of skrifa src/color/instance.rs.
func (inst *colrInstance) varDeltas(varIndexBase uint32, n int) []float32 {
	const noVariationDeltas = 0xFFFFFFFF
	deltas := make([]float32, n)
	if len(inst.coords) == 0 ||
		inst.colr.ItemVariationStore == nil ||
		varIndexBase == noVariationDeltas {
		return deltas
	}
	actualCount := min(int(math.MaxUint32-varIndexBase), n)
	varStore := inst.colr.ItemVariationStore
	for i := range actualCount {
		varIndex := varIndexBase + uint32(i)
		var deltaIx tables.VariationStoreIndex
		if inst.colr.VarIndexMap != nil {
			deltaIx = inst.colr.VarIndexMap.Index(tables.GlyphID(varIndex))
		} else {
			deltaIx = tables.VariationStoreIndex{DeltaSetOuter: 0, DeltaSetInner: uint16(varIndex)}
		}
		deltas[i] = varStore.GetDelta(deltaIx, inst.coords)
	}
	return deltas
}

// f2dot14 is F2Dot14::to_f32 of font-types.
func f2dot14(v tables.Fixed214) float32 {
	return float32(v) / 16384
}

// f2dot14Delta is F2Dot14::apply_float_delta of font-types.
func f2dot14Delta(v tables.Fixed214, delta float32) float32 {
	return f2dot14(v) + float32(delta*(1.0/16384))
}

// fixedDelta is Fixed::apply_float_delta of font-types, for a 16.16 value.
func fixedDelta(v tables.Float1616, delta float32) float32 {
	return v + float32(delta*(1.0/65536))
}

// resolveClipBox is resolve_clip_box of skrifa src/color/instance.rs.
func resolveClipBox(inst *colrInstance, clipBox tables.ClipBox) colrBoundingBox {
	switch cbox := clipBox.(type) {
	case tables.ClipBoxFormat2:
		deltas := inst.varDeltas(cbox.VarIndexBase, 4)
		return colrBoundingBox{
			xMin: float32(cbox.XMin) + deltas[0],
			yMin: float32(cbox.YMin) + deltas[1],
			xMax: float32(cbox.XMax) + deltas[2],
			yMax: float32(cbox.YMax) + deltas[3],
		}
	case tables.ClipBoxFormat1:
		return colrBoundingBox{
			xMin: float32(cbox.XMin),
			yMin: float32(cbox.YMin),
			xMax: float32(cbox.XMax),
			yMax: float32(cbox.YMax),
		}
	}
	return colrBoundingBox{}
}

// getClipboxFontUnits is get_clipbox_font_units of skrifa
// src/color/traversal.rs.
func getClipboxFontUnits(inst *colrInstance, glyphID GlyphID) (colrBoundingBox, bool) {
	clip, ok := inst.colr.ClipList.Search(tables.GlyphID(glyphID))
	if !ok || clip == nil {
		return colrBoundingBox{}, false
	}
	return resolveClipBox(inst, clip), true
}

// resolveColorStops is ColorStops::resolve of skrifa src/color/instance.rs.
func resolveColorStops(inst *colrInstance, stops []tables.ColorStop, varStops []tables.VarColorStop) []colrColorStop {
	out := make([]colrColorStop, 0, len(stops)+len(varStops))
	for _, stop := range stops {
		out = append(out, colrColorStop{
			offset:       f2dot14(stop.StopOffset),
			paletteIndex: stop.PaletteIndex,
			alpha:        f2dot14(stop.Alpha),
		})
	}
	for _, stop := range varStops {
		deltas := inst.varDeltas(stop.VarIndexBase, 2)
		out = append(out, colrColorStop{
			offset:       f2dot14Delta(stop.StopOffset, deltas[0]),
			paletteIndex: stop.PaletteIndex,
			alpha:        f2dot14Delta(stop.Alpha, deltas[1]),
		})
	}
	return out
}

// resolvedPaintKind is the case of a resolvedPaint.
type resolvedPaintKind uint8

// The cases of ResolvedPaint of skrifa src/color/instance.rs.
const (
	resolvedColrLayers resolvedPaintKind = iota
	resolvedSolid
	resolvedLinearGradient
	resolvedRadialGradient
	resolvedSweepGradient
	resolvedGlyph
	resolvedColrGlyph
	resolvedTransform
	resolvedTranslate
	resolvedScale
	resolvedRotate
	resolvedSkew
	resolvedComposite
)

// resolvedPaint is ResolvedPaint of skrifa src/color/instance.rs. The enum
// has data, so it becomes a struct with a kind (D27). The points of a
// gradient are x0, y0, x1, y1, x2 and y2, and a transform keeps its six
// elements in ts.
type resolvedPaint struct {
	kind resolvedPaintKind

	layers     []tables.PaintTable
	firstLayer uint32

	paletteIndex uint16
	alpha        float32

	x0, y0, x1, y1, x2, y2 float32
	radius0, radius1       float32
	startAngle, endAngle   float32
	colorStops             []colrColorStop
	extend                 tables.Extend

	glyphID uint16
	paint   tables.PaintTable

	ts colrTransform

	scaleX, scaleY float32
	angle          float32
	xSkewAngle     float32
	ySkewAngle     float32
	aroundCenter   *colrPoint

	sourcePaint   tables.PaintTable
	mode          tables.CompositeMode
	backdropPaint tables.PaintTable
}

// resolvePaint is resolve_paint of skrifa src/color/instance.rs. It returns
// false where upstream returns a ReadError.
func resolvePaint(inst *colrInstance, paint tables.PaintTable) (resolvedPaint, bool) {
	center := func(x, y float32) *colrPoint { return &colrPoint{x, y} }
	switch p := paint.(type) {
	case tables.PaintColrLayers:
		layers, err := inst.colr.LayerList.Resolve(p)
		if err != nil {
			return resolvedPaint{}, false
		}
		return resolvedPaint{kind: resolvedColrLayers, layers: layers, firstLayer: p.FirstLayerIndex}, true
	case tables.PaintSolid:
		return resolvedPaint{kind: resolvedSolid, paletteIndex: p.PaletteIndex, alpha: f2dot14(p.Alpha)}, true
	case tables.PaintVarSolid:
		deltas := inst.varDeltas(p.VarIndexBase, 1)
		return resolvedPaint{kind: resolvedSolid, paletteIndex: p.PaletteIndex, alpha: f2dot14Delta(p.Alpha, deltas[0])}, true
	case tables.PaintLinearGradient:
		return resolvedPaint{
			kind: resolvedLinearGradient,
			x0:   float32(p.X0), y0: float32(p.Y0),
			x1: float32(p.X1), y1: float32(p.Y1),
			x2: float32(p.X2), y2: float32(p.Y2),
			colorStops: resolveColorStops(inst, p.ColorLine.ColorStops, nil),
			extend:     p.ColorLine.Extend,
		}, true
	case tables.PaintVarLinearGradient:
		deltas := inst.varDeltas(p.VarIndexBase, 6)
		return resolvedPaint{
			kind: resolvedLinearGradient,
			x0:   float32(p.X0) + deltas[0], y0: float32(p.Y0) + deltas[1],
			x1: float32(p.X1) + deltas[2], y1: float32(p.Y1) + deltas[3],
			x2: float32(p.X2) + deltas[4], y2: float32(p.Y2) + deltas[5],
			colorStops: resolveColorStops(inst, nil, p.ColorLine.ColorStops),
			extend:     p.ColorLine.Extend,
		}, true
	case tables.PaintRadialGradient:
		return resolvedPaint{
			kind: resolvedRadialGradient,
			x0:   float32(p.X0), y0: float32(p.Y0), radius0: float32(p.Radius0),
			x1: float32(p.X1), y1: float32(p.Y1), radius1: float32(p.Radius1),
			colorStops: resolveColorStops(inst, p.ColorLine.ColorStops, nil),
			extend:     p.ColorLine.Extend,
		}, true
	case tables.PaintVarRadialGradient:
		deltas := inst.varDeltas(p.VarIndexBase, 6)
		return resolvedPaint{
			kind: resolvedRadialGradient,
			x0:   float32(p.X0) + deltas[0], y0: float32(p.Y0) + deltas[1], radius0: float32(p.Radius0) + deltas[2],
			x1: float32(p.X1) + deltas[3], y1: float32(p.Y1) + deltas[4], radius1: float32(p.Radius1) + deltas[5],
			colorStops: resolveColorStops(inst, nil, p.ColorLine.ColorStops),
			extend:     p.ColorLine.Extend,
		}, true
	case tables.PaintSweepGradient:
		return resolvedPaint{
			kind: resolvedSweepGradient,
			x0:   float32(p.CenterX), y0: float32(p.CenterY),
			startAngle: f2dot14(p.StartAngle), endAngle: f2dot14(p.EndAngle),
			colorStops: resolveColorStops(inst, p.ColorLine.ColorStops, nil),
			extend:     p.ColorLine.Extend,
		}, true
	case tables.PaintVarSweepGradient:
		deltas := inst.varDeltas(p.VarIndexBase, 4)
		return resolvedPaint{
			kind: resolvedSweepGradient,
			x0:   float32(p.CenterX) + deltas[0], y0: float32(p.CenterY) + deltas[1],
			startAngle: f2dot14Delta(p.StartAngle, deltas[2]), endAngle: f2dot14Delta(p.EndAngle, deltas[3]),
			colorStops: resolveColorStops(inst, nil, p.ColorLine.ColorStops),
			extend:     p.ColorLine.Extend,
		}, true
	case tables.PaintGlyph:
		return resolvedPaint{kind: resolvedGlyph, glyphID: p.GlyphID, paint: p.Paint}, true
	case tables.PaintColrGlyph:
		return resolvedPaint{kind: resolvedColrGlyph, glyphID: p.GlyphID}, true
	case tables.PaintTransform:
		a := p.Transform
		return resolvedPaint{kind: resolvedTransform, paint: p.Paint,
			ts: colrTransform{a.Xx, a.Yx, a.Xy, a.Yy, a.Dx, a.Dy}}, true
	case tables.PaintVarTransform:
		a := p.Transform
		deltas := inst.varDeltas(a.VarIndexBase, 6)
		return resolvedPaint{kind: resolvedTransform, paint: p.Paint, ts: colrTransform{
			fixedDelta(a.Xx, deltas[0]), fixedDelta(a.Yx, deltas[1]),
			fixedDelta(a.Xy, deltas[2]), fixedDelta(a.Yy, deltas[3]),
			fixedDelta(a.Dx, deltas[4]), fixedDelta(a.Dy, deltas[5]),
		}}, true
	case tables.PaintTranslate:
		return resolvedPaint{kind: resolvedTranslate, paint: p.Paint,
			ts: colrTransform{dx: float32(p.Dx), dy: float32(p.Dy)}}, true
	case tables.PaintVarTranslate:
		deltas := inst.varDeltas(p.VarIndexBase, 2)
		return resolvedPaint{kind: resolvedTranslate, paint: p.Paint,
			ts: colrTransform{dx: float32(p.Dx) + deltas[0], dy: float32(p.Dy) + deltas[1]}}, true
	case tables.PaintScale:
		return resolvedPaint{kind: resolvedScale, paint: p.Paint,
			scaleX: f2dot14(p.ScaleX), scaleY: f2dot14(p.ScaleY)}, true
	case tables.PaintVarScale:
		deltas := inst.varDeltas(p.VarIndexBase, 2)
		return resolvedPaint{kind: resolvedScale, paint: p.Paint,
			scaleX: f2dot14Delta(p.ScaleX, deltas[0]), scaleY: f2dot14Delta(p.ScaleY, deltas[1])}, true
	case tables.PaintScaleAroundCenter:
		return resolvedPaint{kind: resolvedScale, paint: p.Paint,
			scaleX: f2dot14(p.ScaleX), scaleY: f2dot14(p.ScaleY),
			aroundCenter: center(float32(p.CenterX), float32(p.CenterY))}, true
	case tables.PaintVarScaleAroundCenter:
		deltas := inst.varDeltas(p.VarIndexBase, 4)
		return resolvedPaint{kind: resolvedScale, paint: p.Paint,
			scaleX: f2dot14Delta(p.ScaleX, deltas[0]), scaleY: f2dot14Delta(p.ScaleY, deltas[1]),
			aroundCenter: center(float32(p.CenterX)+deltas[2], float32(p.CenterY)+deltas[3])}, true
	case tables.PaintScaleUniform:
		s := f2dot14(p.Scale)
		return resolvedPaint{kind: resolvedScale, paint: p.Paint, scaleX: s, scaleY: s}, true
	case tables.PaintVarScaleUniform:
		deltas := inst.varDeltas(p.VarIndexBase, 1)
		s := f2dot14Delta(p.Scale, deltas[0])
		return resolvedPaint{kind: resolvedScale, paint: p.Paint, scaleX: s, scaleY: s}, true
	case tables.PaintScaleUniformAroundCenter:
		s := f2dot14(p.Scale)
		return resolvedPaint{kind: resolvedScale, paint: p.Paint, scaleX: s, scaleY: s,
			aroundCenter: center(float32(p.CenterX), float32(p.CenterY))}, true
	case tables.PaintVarScaleUniformAroundCenter:
		deltas := inst.varDeltas(p.VarIndexBase, 3)
		s := f2dot14Delta(p.Scale, deltas[0])
		return resolvedPaint{kind: resolvedScale, paint: p.Paint, scaleX: s, scaleY: s,
			aroundCenter: center(float32(p.CenterX)+deltas[1], float32(p.CenterY)+deltas[2])}, true
	case tables.PaintRotate:
		return resolvedPaint{kind: resolvedRotate, paint: p.Paint, angle: f2dot14(p.Angle)}, true
	case tables.PaintVarRotate:
		deltas := inst.varDeltas(p.VarIndexBase, 1)
		return resolvedPaint{kind: resolvedRotate, paint: p.Paint, angle: f2dot14Delta(p.Angle, deltas[0])}, true
	case tables.PaintRotateAroundCenter:
		return resolvedPaint{kind: resolvedRotate, paint: p.Paint, angle: f2dot14(p.Angle),
			aroundCenter: center(float32(p.CenterX), float32(p.CenterY))}, true
	case tables.PaintVarRotateAroundCenter:
		deltas := inst.varDeltas(p.VarIndexBase, 3)
		return resolvedPaint{kind: resolvedRotate, paint: p.Paint, angle: f2dot14Delta(p.Angle, deltas[0]),
			aroundCenter: center(float32(p.CenterX)+deltas[1], float32(p.CenterY)+deltas[2])}, true
	case tables.PaintSkew:
		return resolvedPaint{kind: resolvedSkew, paint: p.Paint,
			xSkewAngle: f2dot14(p.XSkewAngle), ySkewAngle: f2dot14(p.YSkewAngle)}, true
	case tables.PaintVarSkew:
		deltas := inst.varDeltas(p.VarIndexBase, 2)
		return resolvedPaint{kind: resolvedSkew, paint: p.Paint,
			xSkewAngle: f2dot14Delta(p.XSkewAngle, deltas[0]), ySkewAngle: f2dot14Delta(p.YSkewAngle, deltas[1])}, true
	case tables.PaintSkewAroundCenter:
		return resolvedPaint{kind: resolvedSkew, paint: p.Paint,
			xSkewAngle: f2dot14(p.XSkewAngle), ySkewAngle: f2dot14(p.YSkewAngle),
			aroundCenter: center(float32(p.CenterX), float32(p.CenterY))}, true
	case tables.PaintVarSkewAroundCenter:
		deltas := inst.varDeltas(p.VarIndexBase, 4)
		return resolvedPaint{kind: resolvedSkew, paint: p.Paint,
			xSkewAngle: f2dot14Delta(p.XSkewAngle, deltas[0]), ySkewAngle: f2dot14Delta(p.YSkewAngle, deltas[1]),
			aroundCenter: center(float32(p.CenterX)+deltas[2], float32(p.CenterY)+deltas[3])}, true
	case tables.PaintComposite:
		return resolvedPaint{kind: resolvedComposite, sourcePaint: p.SourcePaint, mode: p.CompositeMode,
			backdropPaint: p.BackdropPaint}, true
	}
	return resolvedPaint{}, false
}

// toRadians32 is f32::to_radians of Rust.
func toRadians32(x float32) float32 {
	return x * (math.Pi / 180)
}

// paintTransformOf is the impl TryFrom<&ResolvedPaint> for Transform of
// skrifa src/color/transform.rs.
func paintTransformOf(paint *resolvedPaint) colrTransform {
	switch paint.kind {
	case resolvedRotate:
		sinV := crmath.Sin32(toRadians32(paint.angle * 180.0))
		cosV := crmath.Cos32(toRadians32(paint.angle * 180.0))
		out := colrTransform{xx: cosV, xy: -sinV, yx: sinV, yy: cosV}

		scalarDotProduct := func(a, b, c, d float32) float32 {
			return float32(a*b) + float32(c*d)
		}

		if c := paint.aroundCenter; c != nil {
			out.dx = scalarDotProduct(sinV, c.y, 1.0-cosV, c.x)
			out.dy = scalarDotProduct(-sinV, c.x, 1.0-cosV, c.y)
		}
		return out
	case resolvedScale:
		out := colrTransform{xx: paint.scaleX, yy: paint.scaleY}

		if c := paint.aroundCenter; c != nil {
			out.dx = c.x - float32(paint.scaleX*c.x)
			out.dy = c.y - float32(paint.scaleY*c.y)
		}
		return out
	case resolvedSkew:
		tanX := crmath.Tan32(toRadians32(paint.xSkewAngle * 180.0))
		tanY := crmath.Tan32(toRadians32(paint.ySkewAngle * 180.0))
		out := colrTransform{xx: 1, yy: 1, xy: -tanX, yx: tanY}

		if c := paint.aroundCenter; c != nil {
			out.dx = tanX * c.y
			out.dy = -tanY * c.x
		}
		return out
	case resolvedTransform:
		return paint.ts
	case resolvedTranslate:
		return colrTransform{xx: 1, yy: 1, dx: paint.ts.dx, dy: paint.ts.dy}
	}
	return identityColrTransform()
}

// maxTraversalDepth is MAX_TRAVERSAL_DEPTH of skrifa
// src/color/traversal.rs.
const maxTraversalDepth = 64

// paintDecycler is PaintDecycler of skrifa src/color/traversal.rs: the ids
// of the paints that the traversal is in, to find a cycle.
//
// Port: skrifa takes the offset of a paint as its id. go-text gives no
// offsets, so the traversal takes the index of a layer in the LayerList, or
// the glyph of a base glyph above 1<<32. The index is in the whole LayerList,
// not in one PaintColrLayers, because a PaintColrLayers inside another one,
// as in a ZWJ sequence of Noto Color Emoji, starts again at 0.
type paintDecycler struct {
	stack []uint64
}

// enter is Decycler::enter of skrifa src/decycler.rs. It returns false for a
// cycle or a traversal that is too deep, and else a function that leaves.
func (d *paintDecycler) enter(id uint64) (func(), bool) {
	if len(d.stack) >= maxTraversalDepth || slices.Contains(d.stack, id) {
		return nil, false
	}
	d.stack = append(d.stack, id)
	return func() { d.stack = d.stack[:len(d.stack)-1] }, true
}

// makeSortedResolvedStops is make_sorted_resolved_stops of skrifa
// src/color/traversal.rs.
func makeSortedResolvedStops(stops []colrColorStop) []colrColorStop {
	out := slices.Clone(stops)
	slices.SortStableFunc(out, func(a, b colrColorStop) int {
		if a.offset != a.offset || b.offset != b.offset {
			return 0
		}
		return cmp.Compare(a.offset, b.offset)
	})
	return out
}

// collectFillGlyphPainter is CollectFillGlyphPainter of skrifa
// src/color/traversal.rs.
type collectFillGlyphPainter struct {
	brushTransform      *colrTransform
	glyphID             GlyphID
	parentPainter       colorPainter
	optimizationSuccess bool
}

// pushTransform is ColorPainter::push_transform for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) pushTransform(transform colrTransform) {
	if p.optimizationSuccess {
		if p.brushTransform == nil {
			t := transform
			p.brushTransform = &t
		} else {
			t := p.brushTransform.mul(transform)
			p.brushTransform = &t
		}
	}
}

// popTransform is ColorPainter::pop_transform for CollectFillGlyphPainter.
//
//nolint:dupword // the comment of skrifa
func (p *collectFillGlyphPainter) popTransform() {
	// Since we only support fill and and transform operations, we need to
	// ignore a popped transform, as this would be called after traversing
	// the graph backup after a fill was performed, but we want to preserve
	// the transform in order to be able to return it.
}

// fillGlyph is the default ColorPainter::fill_glyph of skrifa
// src/color/mod.rs, for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) fillGlyph(glyphID GlyphID, brushTransform *colrTransform, brush colrBrush) {
	p.pushClipGlyph(glyphID)
	if brushTransform != nil {
		p.pushTransform(*brushTransform)
		p.fill(brush)
		p.popTransform()
	} else {
		p.fill(brush)
	}
	p.popClip()
}

// fill is ColorPainter::fill for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) fill(brush colrBrush) {
	if p.optimizationSuccess {
		p.parentPainter.fillGlyph(p.glyphID, p.brushTransform, brush)
	}
}

// pushClipGlyph is ColorPainter::push_clip_glyph for
// CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) pushClipGlyph(GlyphID) {
	p.optimizationSuccess = false
}

// pushClipBox is ColorPainter::push_clip_box for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) pushClipBox(colrBoundingBox) {
	p.optimizationSuccess = false
}

// popClip is ColorPainter::pop_clip for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) popClip() {
	p.optimizationSuccess = false
}

// pushLayer is ColorPainter::push_layer for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) pushLayer(tables.CompositeMode) {
	p.optimizationSuccess = false
}

// popLayer is ColorPainter::pop_layer for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) popLayer() {
	p.optimizationSuccess = false
}

// projectOnto is the closure project_onto of traverse_with_callbacks in
// skrifa src/color/traversal.rs.
func projectOnto(vector, point colrPoint) colrPoint {
	length := float32(math.Sqrt(float64(float32(point.x*point.x) + float32(point.y*point.y))))
	if length == 0.0 {
		return colrPoint{}
	}
	pointNormalized := colrPoint{point.x / length, point.y / length}
	dot := float32(vector.x*point.x) + float32(vector.y*point.y)
	f := dot / length
	pointNormalized.x *= f
	pointNormalized.y *= f
	return pointNormalized
}

// traverseWithCallbacks is traverse_with_callbacks of skrifa
// src/color/traversal.rs. It returns false where upstream returns a
// PaintError.
func traverseWithCallbacks(
	paint *resolvedPaint,
	inst *colrInstance,
	painter colorPainter,
	decycler *paintDecycler,
	recurseDepth int,
) bool {
	if recurseDepth >= maxTraversalDepth {
		return false
	}
	switch paint.kind {
	case resolvedColrLayers:
		for i, layerPaint := range paint.layers {
			leave, ok := decycler.enter(uint64(paint.firstLayer) + uint64(i))
			if !ok {
				return false
			}
			rp, ok := resolvePaint(inst, layerPaint)
			if !ok {
				leave()
				return false
			}
			ok = traverseWithCallbacks(&rp, inst, painter, decycler, recurseDepth+1)
			leave()
			if !ok {
				return false
			}
		}
		return true
	case resolvedSolid:
		painter.fill(colrBrush{kind: colrBrushSolid, paletteIndex: paint.paletteIndex, alpha: paint.alpha})
		return true
	case resolvedLinearGradient:
		p0 := colrPoint{paint.x0, paint.y0}
		p1 := colrPoint{paint.x1, paint.y1}
		p2 := colrPoint{paint.x2, paint.y2}

		sub := func(a, b colrPoint) colrPoint { return colrPoint{a.x - b.x, a.y - b.y} }
		crossProduct := func(a, b colrPoint) float32 { return float32(a.x*b.y) - float32(a.y*b.x) }

		resolvedStops := makeSortedResolvedStops(paint.colorStops)

		// If p0p1 or p0p2 are degenerate probably nothing should be drawn.
		// If p0p1 and p0p2 are parallel then one side is the first color and the other side is
		// the last color, depending on the direction.
		// For now, just use the first color.
		if p1 == p0 || p2 == p0 || crossProduct(sub(p1, p0), sub(p2, p0)) == 0.0 {
			if len(resolvedStops) != 0 {
				stop := resolvedStops[0]
				painter.fill(colrBrush{kind: colrBrushSolid, paletteIndex: stop.paletteIndex, alpha: stop.alpha})
			}
			return true
		}

		perpendicularToP2 := sub(p2, p0)
		perpendicularToP2 = colrPoint{perpendicularToP2.y, -perpendicularToP2.x}
		proj := projectOnto(sub(p1, p0), perpendicularToP2)
		p3 := colrPoint{p0.x + proj.x, p0.y + proj.y}

		if len(resolvedStops) != 0 {
			firstStop := resolvedStops[0]
			lastStop := resolvedStops[len(resolvedStops)-1]
			colorStopRange := lastStop.offset - firstStop.offset

			// Nothing can be drawn for this situation.
			if colorStopRange == 0.0 && paint.extend != tables.ExtendPad {
				return true
			}

			// In the Pad case, for providing normalized stops in the 0 to 1 range to the client,
			// insert a color stop at the end. Adding this stop will paint the equivalent gradient,
			// because: All font-specified color stops are in the same spot, mode is pad, so
			// everything before this spot is painted with the first color, everything after this spot
			// is painted with the last color. Not adding this stop would skip the projection below along
			// the p0-p3 axis and result in specifying non-normalized color stops to the shader.

			if colorStopRange == 0.0 && paint.extend == tables.ExtendPad {
				extraStop := lastStop
				extraStop.offset += 1.0
				resolvedStops = append(resolvedStops, extraStop)

				colorStopRange = 1.0
			}

			if colorStopRange != 1.0 || firstStop.offset != 0.0 {
				p0p3 := sub(p3, p0)
				p0Offset := colrPoint{p0p3.x * firstStop.offset, p0p3.y * firstStop.offset}
				p3Offset := colrPoint{p0p3.x * lastStop.offset, p0p3.y * lastStop.offset}

				p3 = colrPoint{p0.x + p3Offset.x, p0.y + p3Offset.y}
				p0 = colrPoint{p0.x + p0Offset.x, p0.y + p0Offset.y}

				scaleFactor := 1.0 / colorStopRange
				startOffset := firstStop.offset

				for i := range resolvedStops {
					resolvedStops[i].offset = (resolvedStops[i].offset - startOffset) * scaleFactor
				}
			}

			painter.fill(colrBrush{
				kind:       colrBrushLinearGradient,
				p0:         p0,
				p1:         p3,
				colorStops: resolvedStops,
				extend:     paint.extend,
			})
		}

		return true
	case resolvedRadialGradient:
		c0 := colrPoint{paint.x0, paint.y0}
		c1 := colrPoint{paint.x1, paint.y1}
		radius0 := paint.radius0
		radius1 := paint.radius1

		resolvedStops := makeSortedResolvedStops(paint.colorStops)

		if len(resolvedStops) != 0 {
			firstStop := resolvedStops[0]
			lastStop := resolvedStops[len(resolvedStops)-1]
			colorStopRange := lastStop.offset - firstStop.offset
			// Nothing can be drawn for this situation.
			if colorStopRange == 0.0 && paint.extend != tables.ExtendPad {
				return true
			}

			// In the Pad case, for providing normalized stops in the 0 to 1 range to the client,
			// insert a color stop at the end. See LinearGradient for more details.

			if colorStopRange == 0.0 && paint.extend == tables.ExtendPad {
				extraStop := lastStop
				extraStop.offset += 1.0
				resolvedStops = append(resolvedStops, extraStop)
				colorStopRange = 1.0
			}

			// If the colorline is not normalized, normalize it by modifying the circles.
			if colorStopRange != 1.0 || firstStop.offset != 0.0 {
				c0ToC1 := colrPoint{c1.x - c0.x, c1.y - c0.y}
				radiusDiff := radius1 - radius0
				scaleFactor := 1.0 / colorStopRange

				c0Offset := colrPoint{c0ToC1.x * firstStop.offset, c0ToC1.y * firstStop.offset}
				c1Offset := colrPoint{c0ToC1.x * lastStop.offset, c0ToC1.y * lastStop.offset}
				stopsStartOffset := firstStop.offset

				c1 = colrPoint{c0.x + c1Offset.x, c0.y + c1Offset.y}
				c0 = colrPoint{c0.x + c0Offset.x, c0.y + c0Offset.y}
				radius1 = radius0 + float32(radiusDiff*lastStop.offset)
				radius0 += float32(radiusDiff * firstStop.offset)

				for i := range resolvedStops {
					resolvedStops[i].offset = (resolvedStops[i].offset - stopsStartOffset) * scaleFactor
				}
			}

			painter.fill(colrBrush{
				kind:       colrBrushRadialGradient,
				c0:         c0,
				r0:         radius0,
				c1:         c1,
				r1:         radius1,
				colorStops: resolvedStops,
				extend:     paint.extend,
			})
		}
		return true
	case resolvedSweepGradient:
		// OpenType 1.9.1 adds a shift to the angle to ease specifying a 0 to 360
		// degree sweep.
		sweepAngleToDegrees := func(angle float32) float32 { return float32(angle*180.0) + 180.0 }

		startAngle := sweepAngleToDegrees(paint.startAngle)
		endAngle := sweepAngleToDegrees(paint.endAngle)

		// Stop normalization for sweep:

		sectorAngle := endAngle - startAngle

		resolvedStops := makeSortedResolvedStops(paint.colorStops)
		if len(resolvedStops) == 0 {
			return true
		}

		firstStop := resolvedStops[0]
		lastStop := resolvedStops[len(resolvedStops)-1]
		colorStopRange := lastStop.offset - firstStop.offset

		startAngleScaled := float32(sectorAngle*firstStop.offset) + startAngle
		endAngleScaled := float32(sectorAngle*lastStop.offset) + startAngle

		startOffset := firstStop.offset

		// Nothing can be drawn for this situation.
		if colorStopRange == 0.0 && paint.extend != tables.ExtendPad {
			return true
		}

		// In the Pad case, if the color_stop_range is 0 insert a color stop at the end.
		if colorStopRange == 0.0 && paint.extend == tables.ExtendPad {
			offsetLast := lastStop
			offsetLast.offset += 1.0
			resolvedStops = append(resolvedStops, offsetLast)
			colorStopRange = 1.0
		}

		scaleFactor := 1.0 / colorStopRange

		for i := range resolvedStops {
			resolvedStops[i].offset = (resolvedStops[i].offset - startOffset) * scaleFactor
		}

		// /* https://docs.microsoft.com/en-us/typography/opentype/spec/colr#sweep-gradients
		//  * "The angles are expressed in counter-clockwise degrees from
		//  * the direction of the positive x-axis on the design
		//  * grid. [...]  The color line progresses from the start angle
		//  * to the end angle in the counter-clockwise direction;" -
		//  * Convert angles and stops from counter-clockwise to clockwise
		//  * for the shader if the gradient is not already reversed due to
		//  * start angle being larger than end angle. */
		startAngleScaled = 360.0 - startAngleScaled
		endAngleScaled = 360.0 - endAngleScaled

		if startAngleScaled >= endAngleScaled {
			startAngleScaled, endAngleScaled = endAngleScaled, startAngleScaled
			slices.Reverse(resolvedStops)
			for i := range resolvedStops {
				resolvedStops[i].offset = 1.0 - resolvedStops[i].offset
			}
		}

		// https://learn.microsoft.com/en-us/typography/opentype/spec/colr#sweep-gradients
		// "If the starting and ending angles are the same, a sweep gradient
		// is not drawn."
		if startAngleScaled == endAngleScaled && paint.extend != tables.ExtendPad {
			return true
		}

		painter.fill(colrBrush{
			kind:       colrBrushSweepGradient,
			c0:         colrPoint{paint.x0, paint.y0},
			startAngle: startAngleScaled,
			endAngle:   endAngleScaled,
			colorStops: resolvedStops,
			extend:     paint.extend,
		})
		return true
	case resolvedGlyph:
		glyphID := GlyphID(paint.glyphID)
		optimizer := &collectFillGlyphPainter{glyphID: glyphID, parentPainter: painter, optimizationSuccess: true}
		rp, ok := resolvePaint(inst, paint.paint)
		if !ok {
			return false
		}
		result := traverseWithCallbacks(&rp, inst, optimizer, decycler, recurseDepth+1)

		if !optimizer.optimizationSuccess {
			painter.pushClipGlyph(glyphID)
			rp, ok := resolvePaint(inst, paint.paint)
			if !ok {
				return false
			}
			result = traverseWithCallbacks(&rp, inst, painter, decycler, recurseDepth+1)
			painter.popClip()
		}

		return result
	case resolvedColrGlyph:
		glyphID := GlyphID(paint.glyphID)
		baseGlyph, ok := inst.colr.Search(tables.GlyphID(glyphID))
		if _, v0 := baseGlyph.(tables.PaintColrLayersResolved); !ok || v0 {
			// PaintError::GlyphNotFound
			return false
		}
		leave, ok := decycler.enter(1<<32 | uint64(glyphID))
		if !ok {
			return false
		}
		defer leave()
		clipbox, hasClipbox := getClipboxFontUnits(inst, glyphID)

		if hasClipbox {
			painter.pushClipBox(clipbox)
		}

		rp, ok := resolvePaint(inst, baseGlyph)
		if !ok {
			return false
		}
		result := traverseWithCallbacks(&rp, inst, painter, decycler, recurseDepth+1)
		if hasClipbox {
			painter.popClip()
		}
		return result
	case resolvedTransform, resolvedTranslate, resolvedScale, resolvedRotate, resolvedSkew:
		painter.pushTransform(paintTransformOf(paint))
		rp, ok := resolvePaint(inst, paint.paint)
		if !ok {
			return false
		}
		result := traverseWithCallbacks(&rp, inst, painter, decycler, recurseDepth+1)
		painter.popTransform()
		return result
	case resolvedComposite:
		painter.pushLayer(tables.CompositeSrcOver)
		rp, ok := resolvePaint(inst, paint.backdropPaint)
		if !ok {
			return false
		}
		if !traverseWithCallbacks(&rp, inst, painter, decycler, recurseDepth+1) {
			return false
		}
		painter.pushLayer(paint.mode)
		rp, ok = resolvePaint(inst, paint.sourcePaint)
		if !ok {
			return false
		}
		result := traverseWithCallbacks(&rp, inst, painter, decycler, recurseDepth+1)
		painter.popLayer()
		painter.popLayer()
		return result
	}
	return false
}

// paintColorGlyph is font.color_glyphs().get(glyph_id) and ColorGlyph::paint
// of skrifa src/color/mod.rs, which usvg calls. It paints the color glyph of
// glyphID of ft at coords, preferring a COLRv1 representation over a COLRv0
// one, and returns false when the glyph has none or painting fails.
func paintColorGlyph(ft *font.Font, coords []tables.Coord, glyphID GlyphID, painter colorPainter) bool {
	if ft.COLR == nil {
		return false
	}
	paint, ok := ft.COLR.Search(tables.GlyphID(glyphID))
	if !ok {
		return false
	}
	var effective []tables.Coord
	if !allZero(coords) {
		effective = coords
	}
	inst := &colrInstance{colr: ft.COLR, coords: effective}

	if layers, ok := paint.(tables.PaintColrLayersResolved); ok {
		// traverse_v0_range
		for _, layer := range layers {
			painter.fillGlyph(GlyphID(layer.GlyphID), nil, colrBrush{
				kind:         colrBrushSolid,
				paletteIndex: layer.PaletteIndex,
				alpha:        1.0,
			})
		}
		return true
	}

	clipbox, hasClipbox := getClipboxFontUnits(inst, glyphID)

	if hasClipbox {
		painter.pushClipBox(clipbox)
	}

	var decycler paintDecycler
	leave, ok := decycler.enter(1<<32 | uint64(glyphID))
	if !ok {
		return false
	}
	defer leave()
	rp, ok := resolvePaint(inst, paint)
	if !ok {
		return false
	}
	if !traverseWithCallbacks(&rp, inst, painter, &decycler, 0) {
		return false
	}

	if hasClipbox {
		painter.popClip()
	}
	return true
}
