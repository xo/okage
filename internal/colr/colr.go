// Copyright 2024 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package colr walks the paint graph of a COLR glyph, as
// ColorGlyph::paint of skrifa 0.44 does (D32). svg draws the color glyphs of
// a text with it, as usvg src/text/colr.rs does, and pdf draws the color
// glyphs of a Type 3 font with it, as krilla src/text/glyph/colr.rs does.
// D62 moved it out of svg so that both packages share it.
//
// go-text parses the tables COLR and CPAL, but it does not walk the graph
// (D9). This file walks it as skrifa does: ColorGlyph::paint of
// src/color/mod.rs, traverse.rs, instance.rs and transform.rs of src/color.
// Each item names the item of skrifa that it replaces.
package colr

import (
	"cmp"
	"math"
	"slices"

	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/font/opentype/tables"

	"github.com/xo/okage/internal/crmath"
)

// GlyphID is the ID of a glyph, GlyphId of skrifa.
type GlyphID uint32

// Transform is skrifa::color::Transform, Matrix<f32> of font-types: an
// affine transform with the elements xx, yx, xy, yy, dx and dy.
type Transform struct {
	XX float32
	YX float32
	XY float32
	YY float32
	DX float32
	DY float32
}

// IdentityTransform is the impl Default for Matrix in font-types, the
// identity.
func IdentityTransform() Transform {
	return Transform{XX: 1, YY: 1}
}

// Mul is the impl Mul for Matrix in font-types: t applied after rhs.
func (t Transform) Mul(rhs Transform) Transform {
	return Transform{
		XX: float32(t.XX*rhs.XX) + float32(t.XY*rhs.YX),
		YX: float32(t.YX*rhs.XX) + float32(t.YY*rhs.YX),
		XY: float32(t.XX*rhs.XY) + float32(t.XY*rhs.YY),
		YY: float32(t.YX*rhs.XY) + float32(t.YY*rhs.YY),
		DX: float32(t.XX*rhs.DX) + float32(t.XY*rhs.DY) + t.DX,
		DY: float32(t.YX*rhs.DX) + float32(t.YY*rhs.DY) + t.DY,
	}
}

// Point is Point<f32> of font-types.
type Point struct {
	X float32
	Y float32
}

// BoundingBox is BoundingBox<f32> of font-types.
type BoundingBox struct {
	XMin float32
	YMin float32
	XMax float32
	YMax float32
}

// ColorStop is skrifa::color::ColorStop.
type ColorStop struct {
	Offset       float32
	PaletteIndex uint16
	Alpha        float32
}

// BrushKind is the case of a Brush.
type BrushKind uint8

// The cases of skrifa::color::Brush.
const (
	// BrushSolid is Brush::Solid, with PaletteIndex and Alpha.
	BrushSolid BrushKind = iota
	// BrushLinearGradient is Brush::LinearGradient, with P0, P1,
	// ColorStops and Extend.
	BrushLinearGradient
	// BrushRadialGradient is Brush::RadialGradient, with C0, R0, C1, R1,
	// ColorStops and Extend.
	BrushRadialGradient
	// BrushSweepGradient is Brush::SweepGradient, with C0, StartAngle,
	// EndAngle, ColorStops and Extend.
	BrushSweepGradient
)

// Brush is skrifa::color::Brush. The enum has data, so it becomes a struct
// with a kind (D27). Each case uses the fields that its name gives in
// upstream.
type Brush struct {
	Kind         BrushKind
	PaletteIndex uint16
	Alpha        float32
	P0           Point
	P1           Point
	C0           Point
	R0           float32
	C1           Point
	R1           float32
	StartAngle   float32
	EndAngle     float32
	ColorStops   []ColorStop
	Extend       tables.Extend
}

// Painter is the trait skrifa::color::ColorPainter, with the methods that
// usvg, krilla and the traversal call. FillGlyph has the default method of
// skrifa in a painter that has no method of its own: PushClipGlyph,
// PushTransform of the brush transform when there is one, Fill,
// PopTransform and PopClip.
type Painter interface {
	PushTransform(transform Transform)
	PopTransform()
	FillGlyph(glyphID GlyphID, brushTransform *Transform, brush Brush)
	PushClipGlyph(glyphID GlyphID)
	PushClipBox(clipBox BoundingBox)
	PopClip()
	Fill(brush Brush)
	PushLayer(compositeMode tables.CompositeMode)
	PopLayer()
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
	return float32(float32(v) / 16384)
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
func resolveClipBox(inst *colrInstance, clipBox tables.ClipBox) BoundingBox {
	switch cbox := clipBox.(type) {
	case tables.ClipBoxFormat2:
		deltas := inst.varDeltas(cbox.VarIndexBase, 4)
		return BoundingBox{
			XMin: float32(cbox.XMin) + deltas[0],
			YMin: float32(cbox.YMin) + deltas[1],
			XMax: float32(cbox.XMax) + deltas[2],
			YMax: float32(cbox.YMax) + deltas[3],
		}
	case tables.ClipBoxFormat1:
		return BoundingBox{
			XMin: float32(cbox.XMin),
			YMin: float32(cbox.YMin),
			XMax: float32(cbox.XMax),
			YMax: float32(cbox.YMax),
		}
	}
	return BoundingBox{}
}

// getClipboxFontUnits is get_clipbox_font_units of skrifa
// src/color/traversal.rs.
func getClipboxFontUnits(inst *colrInstance, glyphID GlyphID) (BoundingBox, bool) {
	clip, ok := inst.colr.ClipList.Search(tables.GlyphID(glyphID))
	if !ok || clip == nil {
		return BoundingBox{}, false
	}
	return resolveClipBox(inst, clip), true
}

// resolveColorStops is ColorStops::resolve of skrifa src/color/instance.rs.
func resolveColorStops(inst *colrInstance, stops []tables.ColorStop, varStops []tables.VarColorStop) []ColorStop {
	out := make([]ColorStop, 0, len(stops)+len(varStops))
	for _, stop := range stops {
		out = append(out, ColorStop{
			Offset:       f2dot14(stop.StopOffset),
			PaletteIndex: stop.PaletteIndex,
			Alpha:        f2dot14(stop.Alpha),
		})
	}
	for _, stop := range varStops {
		deltas := inst.varDeltas(stop.VarIndexBase, 2)
		out = append(out, ColorStop{
			Offset:       f2dot14Delta(stop.StopOffset, deltas[0]),
			PaletteIndex: stop.PaletteIndex,
			Alpha:        f2dot14Delta(stop.Alpha, deltas[1]),
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
	colorStops             []ColorStop
	extend                 tables.Extend

	glyphID uint16
	paint   tables.PaintTable

	ts Transform

	scaleX, scaleY float32
	angle          float32
	xSkewAngle     float32
	ySkewAngle     float32
	aroundCenter   *Point

	sourcePaint   tables.PaintTable
	mode          tables.CompositeMode
	backdropPaint tables.PaintTable
}

// resolvePaint is resolve_paint of skrifa src/color/instance.rs. It returns
// false where upstream returns a ReadError.
func resolvePaint(inst *colrInstance, paint tables.PaintTable) (resolvedPaint, bool) {
	center := func(x, y float32) *Point { return &Point{x, y} }
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
			ts: Transform{a.Xx, a.Yx, a.Xy, a.Yy, a.Dx, a.Dy}}, true
	case tables.PaintVarTransform:
		a := p.Transform
		deltas := inst.varDeltas(a.VarIndexBase, 6)
		return resolvedPaint{kind: resolvedTransform, paint: p.Paint, ts: Transform{
			fixedDelta(a.Xx, deltas[0]), fixedDelta(a.Yx, deltas[1]),
			fixedDelta(a.Xy, deltas[2]), fixedDelta(a.Yy, deltas[3]),
			fixedDelta(a.Dx, deltas[4]), fixedDelta(a.Dy, deltas[5]),
		}}, true
	case tables.PaintTranslate:
		return resolvedPaint{kind: resolvedTranslate, paint: p.Paint,
			ts: Transform{DX: float32(p.Dx), DY: float32(p.Dy)}}, true
	case tables.PaintVarTranslate:
		deltas := inst.varDeltas(p.VarIndexBase, 2)
		return resolvedPaint{kind: resolvedTranslate, paint: p.Paint,
			ts: Transform{DX: float32(p.Dx) + deltas[0], DY: float32(p.Dy) + deltas[1]}}, true
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
func paintTransformOf(paint *resolvedPaint) Transform {
	switch paint.kind {
	case resolvedRotate:
		sinV := crmath.Sin32(toRadians32(paint.angle * 180.0))
		cosV := crmath.Cos32(toRadians32(paint.angle * 180.0))
		out := Transform{XX: cosV, XY: -sinV, YX: sinV, YY: cosV}

		scalarDotProduct := func(a, b, c, d float32) float32 {
			return float32(a*b) + float32(c*d)
		}

		if c := paint.aroundCenter; c != nil {
			out.DX = scalarDotProduct(sinV, c.Y, 1.0-cosV, c.X)
			out.DY = scalarDotProduct(-sinV, c.X, 1.0-cosV, c.Y)
		}
		return out
	case resolvedScale:
		out := Transform{XX: paint.scaleX, YY: paint.scaleY}

		if c := paint.aroundCenter; c != nil {
			out.DX = c.X - float32(paint.scaleX*c.X)
			out.DY = c.Y - float32(paint.scaleY*c.Y)
		}
		return out
	case resolvedSkew:
		tanX := crmath.Tan32(toRadians32(paint.xSkewAngle * 180.0))
		tanY := crmath.Tan32(toRadians32(paint.ySkewAngle * 180.0))
		out := Transform{XX: 1, YY: 1, XY: -tanX, YX: tanY}

		if c := paint.aroundCenter; c != nil {
			out.DX = tanX * c.Y
			out.DY = -tanY * c.X
		}
		return out
	case resolvedTransform:
		return paint.ts
	case resolvedTranslate:
		return Transform{XX: 1, YY: 1, DX: paint.ts.DX, DY: paint.ts.DY}
	}
	return IdentityTransform()
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
func makeSortedResolvedStops(stops []ColorStop) []ColorStop {
	out := slices.Clone(stops)
	slices.SortStableFunc(out, func(a, b ColorStop) int {
		if a.Offset != a.Offset || b.Offset != b.Offset {
			return 0
		}
		return cmp.Compare(a.Offset, b.Offset)
	})
	return out
}

// collectFillGlyphPainter is CollectFillGlyphPainter of skrifa
// src/color/traversal.rs.
type collectFillGlyphPainter struct {
	brushTransform      *Transform
	glyphID             GlyphID
	parentPainter       Painter
	optimizationSuccess bool
}

// pushTransform is ColorPainter::push_transform for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) PushTransform(transform Transform) {
	if p.optimizationSuccess {
		if p.brushTransform == nil {
			t := transform
			p.brushTransform = &t
		} else {
			t := p.brushTransform.Mul(transform)
			p.brushTransform = &t
		}
	}
}

// popTransform is ColorPainter::pop_transform for CollectFillGlyphPainter.
//
//nolint:dupword // the comment of skrifa
func (p *collectFillGlyphPainter) PopTransform() {
	// Since we only support fill and and transform operations, we need to
	// ignore a popped transform, as this would be called after traversing
	// the graph backup after a fill was performed, but we want to preserve
	// the transform in order to be able to return it.
}

// fillGlyph is the default ColorPainter::fill_glyph of skrifa
// src/color/mod.rs, for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) FillGlyph(glyphID GlyphID, brushTransform *Transform, brush Brush) {
	p.PushClipGlyph(glyphID)
	if brushTransform != nil {
		p.PushTransform(*brushTransform)
		p.Fill(brush)
		p.PopTransform()
	} else {
		p.Fill(brush)
	}
	p.PopClip()
}

// fill is ColorPainter::fill for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) Fill(brush Brush) {
	if p.optimizationSuccess {
		p.parentPainter.FillGlyph(p.glyphID, p.brushTransform, brush)
	}
}

// pushClipGlyph is ColorPainter::push_clip_glyph for
// CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) PushClipGlyph(GlyphID) {
	p.optimizationSuccess = false
}

// pushClipBox is ColorPainter::push_clip_box for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) PushClipBox(BoundingBox) {
	p.optimizationSuccess = false
}

// popClip is ColorPainter::pop_clip for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) PopClip() {
	p.optimizationSuccess = false
}

// pushLayer is ColorPainter::push_layer for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) PushLayer(tables.CompositeMode) {
	p.optimizationSuccess = false
}

// popLayer is ColorPainter::pop_layer for CollectFillGlyphPainter.
func (p *collectFillGlyphPainter) PopLayer() {
	p.optimizationSuccess = false
}

// projectOnto is the closure project_onto of traverse_with_callbacks in
// skrifa src/color/traversal.rs.
func projectOnto(vector, point Point) Point {
	length := float32(math.Sqrt(float64(float32(point.X*point.X) + float32(point.Y*point.Y))))
	if length == 0.0 {
		return Point{}
	}
	pointNormalized := Point{point.X / length, point.Y / length}
	dot := float32(vector.X*point.X) + float32(vector.Y*point.Y)
	f := dot / length
	pointNormalized.X *= f
	pointNormalized.Y *= f
	return pointNormalized
}

// traverseWithCallbacks is traverse_with_callbacks of skrifa
// src/color/traversal.rs. It returns false where upstream returns a
// PaintError.
func traverseWithCallbacks(
	paint *resolvedPaint,
	inst *colrInstance,
	painter Painter,
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
		painter.Fill(Brush{Kind: BrushSolid, PaletteIndex: paint.paletteIndex, Alpha: paint.alpha})
		return true
	case resolvedLinearGradient:
		p0 := Point{paint.x0, paint.y0}
		p1 := Point{paint.x1, paint.y1}
		p2 := Point{paint.x2, paint.y2}

		sub := func(a, b Point) Point { return Point{a.X - b.X, a.Y - b.Y} }
		crossProduct := func(a, b Point) float32 { return float32(a.X*b.Y) - float32(a.Y*b.X) }

		resolvedStops := makeSortedResolvedStops(paint.colorStops)

		// If p0p1 or p0p2 are degenerate probably nothing should be drawn.
		// If p0p1 and p0p2 are parallel then one side is the first color and the other side is
		// the last color, depending on the direction.
		// For now, just use the first color.
		if p1 == p0 || p2 == p0 || crossProduct(sub(p1, p0), sub(p2, p0)) == 0.0 {
			if len(resolvedStops) != 0 {
				stop := resolvedStops[0]
				painter.Fill(Brush{Kind: BrushSolid, PaletteIndex: stop.PaletteIndex, Alpha: stop.Alpha})
			}
			return true
		}

		perpendicularToP2 := sub(p2, p0)
		perpendicularToP2 = Point{perpendicularToP2.Y, -perpendicularToP2.X}
		proj := projectOnto(sub(p1, p0), perpendicularToP2)
		p3 := Point{p0.X + proj.X, p0.Y + proj.Y}

		if len(resolvedStops) != 0 {
			firstStop := resolvedStops[0]
			lastStop := resolvedStops[len(resolvedStops)-1]
			colorStopRange := lastStop.Offset - firstStop.Offset

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
				extraStop.Offset += 1.0
				resolvedStops = append(resolvedStops, extraStop)

				colorStopRange = 1.0
			}

			if colorStopRange != 1.0 || firstStop.Offset != 0.0 {
				p0p3 := sub(p3, p0)
				p0Offset := Point{float32(p0p3.X * firstStop.Offset), float32(p0p3.Y * firstStop.Offset)}
				p3Offset := Point{float32(p0p3.X * lastStop.Offset), float32(p0p3.Y * lastStop.Offset)}

				p3 = Point{p0.X + p3Offset.X, p0.Y + p3Offset.Y}
				p0 = Point{p0.X + p0Offset.X, p0.Y + p0Offset.Y}

				scaleFactor := 1.0 / colorStopRange
				startOffset := firstStop.Offset

				for i := range resolvedStops {
					resolvedStops[i].Offset = (resolvedStops[i].Offset - startOffset) * scaleFactor
				}
			}

			painter.Fill(Brush{
				Kind:       BrushLinearGradient,
				P0:         p0,
				P1:         p3,
				ColorStops: resolvedStops,
				Extend:     paint.extend,
			})
		}

		return true
	case resolvedRadialGradient:
		c0 := Point{paint.x0, paint.y0}
		c1 := Point{paint.x1, paint.y1}
		radius0 := paint.radius0
		radius1 := paint.radius1

		resolvedStops := makeSortedResolvedStops(paint.colorStops)

		if len(resolvedStops) != 0 {
			firstStop := resolvedStops[0]
			lastStop := resolvedStops[len(resolvedStops)-1]
			colorStopRange := lastStop.Offset - firstStop.Offset
			// Nothing can be drawn for this situation.
			if colorStopRange == 0.0 && paint.extend != tables.ExtendPad {
				return true
			}

			// In the Pad case, for providing normalized stops in the 0 to 1 range to the client,
			// insert a color stop at the end. See LinearGradient for more details.

			if colorStopRange == 0.0 && paint.extend == tables.ExtendPad {
				extraStop := lastStop
				extraStop.Offset += 1.0
				resolvedStops = append(resolvedStops, extraStop)
				colorStopRange = 1.0
			}

			// If the colorline is not normalized, normalize it by modifying the circles.
			if colorStopRange != 1.0 || firstStop.Offset != 0.0 {
				c0ToC1 := Point{c1.X - c0.X, c1.Y - c0.Y}
				radiusDiff := radius1 - radius0
				scaleFactor := 1.0 / colorStopRange

				c0Offset := Point{float32(c0ToC1.X * firstStop.Offset), float32(c0ToC1.Y * firstStop.Offset)}
				c1Offset := Point{float32(c0ToC1.X * lastStop.Offset), float32(c0ToC1.Y * lastStop.Offset)}
				stopsStartOffset := firstStop.Offset

				c1 = Point{c0.X + c1Offset.X, c0.Y + c1Offset.Y}
				c0 = Point{c0.X + c0Offset.X, c0.Y + c0Offset.Y}
				radius1 = radius0 + float32(radiusDiff*lastStop.Offset)
				radius0 += float32(radiusDiff * firstStop.Offset)

				for i := range resolvedStops {
					resolvedStops[i].Offset = (resolvedStops[i].Offset - stopsStartOffset) * scaleFactor
				}
			}

			painter.Fill(Brush{
				Kind:       BrushRadialGradient,
				C0:         c0,
				R0:         radius0,
				C1:         c1,
				R1:         radius1,
				ColorStops: resolvedStops,
				Extend:     paint.extend,
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
		colorStopRange := lastStop.Offset - firstStop.Offset

		startAngleScaled := float32(sectorAngle*firstStop.Offset) + startAngle
		endAngleScaled := float32(sectorAngle*lastStop.Offset) + startAngle

		startOffset := firstStop.Offset

		// Nothing can be drawn for this situation.
		if colorStopRange == 0.0 && paint.extend != tables.ExtendPad {
			return true
		}

		// In the Pad case, if the color_stop_range is 0 insert a color stop at the end.
		if colorStopRange == 0.0 && paint.extend == tables.ExtendPad {
			offsetLast := lastStop
			offsetLast.Offset += 1.0
			resolvedStops = append(resolvedStops, offsetLast)
			colorStopRange = 1.0
		}

		scaleFactor := 1.0 / colorStopRange

		for i := range resolvedStops {
			resolvedStops[i].Offset = (resolvedStops[i].Offset - startOffset) * scaleFactor
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
				resolvedStops[i].Offset = 1.0 - resolvedStops[i].Offset
			}
		}

		// https://learn.microsoft.com/en-us/typography/opentype/spec/colr#sweep-gradients
		// "If the starting and ending angles are the same, a sweep gradient
		// is not drawn."
		if startAngleScaled == endAngleScaled && paint.extend != tables.ExtendPad {
			return true
		}

		painter.Fill(Brush{
			Kind:       BrushSweepGradient,
			C0:         Point{paint.x0, paint.y0},
			StartAngle: startAngleScaled,
			EndAngle:   endAngleScaled,
			ColorStops: resolvedStops,
			Extend:     paint.extend,
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
			painter.PushClipGlyph(glyphID)
			rp, ok := resolvePaint(inst, paint.paint)
			if !ok {
				return false
			}
			result = traverseWithCallbacks(&rp, inst, painter, decycler, recurseDepth+1)
			painter.PopClip()
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
			painter.PushClipBox(clipbox)
		}

		rp, ok := resolvePaint(inst, baseGlyph)
		if !ok {
			return false
		}
		result := traverseWithCallbacks(&rp, inst, painter, decycler, recurseDepth+1)
		if hasClipbox {
			painter.PopClip()
		}
		return result
	case resolvedTransform, resolvedTranslate, resolvedScale, resolvedRotate, resolvedSkew:
		painter.PushTransform(paintTransformOf(paint))
		rp, ok := resolvePaint(inst, paint.paint)
		if !ok {
			return false
		}
		result := traverseWithCallbacks(&rp, inst, painter, decycler, recurseDepth+1)
		painter.PopTransform()
		return result
	case resolvedComposite:
		painter.PushLayer(tables.CompositeSrcOver)
		rp, ok := resolvePaint(inst, paint.backdropPaint)
		if !ok {
			return false
		}
		if !traverseWithCallbacks(&rp, inst, painter, decycler, recurseDepth+1) {
			return false
		}
		painter.PushLayer(paint.mode)
		rp, ok = resolvePaint(inst, paint.sourcePaint)
		if !ok {
			return false
		}
		result := traverseWithCallbacks(&rp, inst, painter, decycler, recurseDepth+1)
		painter.PopLayer()
		painter.PopLayer()
		return result
	}
	return false
}

// Paint is font.color_glyphs().get(glyph_id) and ColorGlyph::paint
// of skrifa src/color/mod.rs, which usvg and krilla call. It paints the color glyph of
// glyphID of ft at coords, preferring a COLRv1 representation over a COLRv0
// one, and returns false when the glyph has none or painting fails.
func Paint(ft *font.Font, coords []tables.Coord, glyphID GlyphID, painter Painter) bool {
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
			painter.FillGlyph(GlyphID(layer.GlyphID), nil, Brush{
				Kind:         BrushSolid,
				PaletteIndex: layer.PaletteIndex,
				Alpha:        1.0,
			})
		}
		return true
	}

	clipbox, hasClipbox := getClipboxFontUnits(inst, glyphID)

	if hasClipbox {
		painter.PushClipBox(clipbox)
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
		painter.PopClip()
	}
	return true
}

// allZero reports whether each coordinate is 0, as
// LocationRef::effective_coords of skrifa finds the default location.
func allZero(coords []tables.Coord) bool {
	for _, c := range coords {
		if c != 0 {
			return false
		}
	}
	return true
}
