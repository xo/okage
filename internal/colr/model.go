// Copyright 2024 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package colr

import (
	"math"
	"slices"

	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// This file walks the paint graph of a COLR glyph as the font model of
// read-fonts 0.46 does, which usvg uses through skrifa 0.49 (D100): the
// function ColorGlyph::paint of src/model/glyph/color.rs, traversal.rs,
// instance.rs and decycler.rs of src/model/glyph/color. krilla still uses
// skrifa 0.44, so pdf keeps the walk of colr.go. The two walks share the
// types of colr.go, the transforms of a paint and the brush of a fill, which
// read-fonts 0.46 computes as skrifa 0.44 does.

// f48Dot16 is F48Dot16 of font-types src/fixed.rs: a number in the 48.16
// fixed-point format, an int64 with 16 bits of fraction.
type f48Dot16 int64

// toF64 is F48Dot16::to_f64 of font-types.
func (f f48Dot16) toF64() float64 { return float64(f) * (1.0 / 65536) }

// fwordDelta is FWord::apply_delta and UfWord::apply_delta of font-types: a
// value in font units with an unscaled delta.
func fwordDelta(v float32, delta f48Dot16) float32 {
	return v + float32(delta.toF64())
}

// f2dot14ApplyDelta is F2Dot14::apply_delta of font-types. A delta of a
// 2.14 value counts units of 1/16384.
func f2dot14ApplyDelta(v tables.Fixed214, delta f48Dot16) float32 {
	return f2dot14(v) + float32(delta.toF64()/16384.0)
}

// fixedApplyDelta is Fixed::apply_delta of font-types, for a 16.16 value. A
// delta of a 16.16 value counts units of 1/65536.
func fixedApplyDelta(v tables.Float1616, delta f48Dot16) float32 {
	return v + float32(delta.toF64()/65536.0)
}

// computeDelta is ItemVariationStore::compute_delta of read-fonts
// src/tables/variations.rs. It adds the product of each delta and the
// scalar of its region in the 48.16 format, and does not round the sum. An
// index or a region that is not in the store gives 0, as the
// unwrap_or_default of its callers does.
//
// Port: tables.ItemVarStore.GetDelta computes the same sum, but returns it
// as a float32, which loses bits that read-fonts keeps. svg has the same
// function for the metrics of MVAR.
func computeDelta(store *tables.ItemVarStore, index tables.VariationStoreIndex, coords []tables.Coord) f48Dot16 {
	const noVariationIndex = 0xFFFF
	if len(coords) == 0 || index.DeltaSetOuter == noVariationIndex && index.DeltaSetInner == noVariationIndex {
		return 0
	}
	if int(index.DeltaSetOuter) >= len(store.ItemVariationDatas) {
		return 0
	}
	data := store.ItemVariationDatas[index.DeltaSetOuter]
	if int(index.DeltaSetInner) >= len(data.DeltaSets) {
		return 0
	}
	deltaSet := data.DeltaSets[index.DeltaSetInner]
	regions := store.VariationRegionList.VariationRegions
	var accum f48Dot16
	for i, regionIndex := range data.RegionIndexes {
		if i >= len(deltaSet) {
			break
		}
		if int(regionIndex) >= len(regions) {
			return 0
		}
		if deltaSet[i] == 0 {
			continue
		}
		accum += f48Dot16(int64(regions[regionIndex].EvaluateFixed(coords)) * int64(deltaSet[i]))
	}
	return accum
}

// modelInstance is ColrInstance of read-fonts
// src/model/glyph/color/instance.rs: the table COLR at a location in the
// variation space of the font.
type modelInstance struct {
	colr   *tables.COLR1
	coords []tables.Coord
}

// varDeltas is ColrInstance::var_deltas of read-fonts
// src/model/glyph/color/instance.rs.
func (inst *modelInstance) varDeltas(varIndexBase uint32, n int) []f48Dot16 {
	const noVariationIndex = 0xFFFFFFFF
	// FreeType never fails these lookups, so neither does this: an
	// unreadable delta is zero.
	deltas := make([]f48Dot16, n)
	varStore := inst.colr.ItemVariationStore
	if varStore == nil {
		return deltas
	}
	if len(inst.coords) == 0 || varIndexBase == noVariationIndex {
		return deltas
	}
	// Avoid overflow if var_index_base + N > u32::MAX
	// Port: the difference is a uint64, because an int of 32 bits cannot
	// hold it, and a negative count then read no delta (D101).
	actualCount := int(min(uint64(math.MaxUint32-varIndexBase), uint64(max(n, 0))))
	for i := range actualCount {
		varIndex := varIndexBase + uint32(i)
		var deltaIx tables.VariationStoreIndex
		if m := inst.colr.VarIndexMap; m != nil {
			deltaIx = deltaSetIndexMapGet(m, varIndex)
		} else {
			// Without a var index map, the mapping is implicit: the index
			// is a delta set index with the outer component in the high
			// word and the inner component in the low word.
			deltaIx = tables.VariationStoreIndex{DeltaSetOuter: uint16(varIndex >> 16), DeltaSetInner: uint16(varIndex)}
		}
		deltas[i] = computeDelta(varStore, deltaIx, inst.coords)
	}
	return deltas
}

// deltaSetIndexMapGet is DeltaSetIndexMap::get of read-fonts
// src/tables/variations.rs. An index past the end of the map takes its last
// entry.
//
// Port: tables.DeltaSetMapping.Index takes a glyph ID of 16 bits. A
// variation index has 32 bits.
func deltaSetIndexMapGet(m *tables.DeltaSetMapping, index uint32) tables.VariationStoreIndex {
	if len(m.Map) == 0 {
		return tables.VariationStoreIndex{DeltaSetOuter: uint16(index >> 16), DeltaSetInner: uint16(index)}
	}
	return m.Map[min(index, uint32(len(m.Map)-1))]
}

// modelResolveClipBox is resolve_clip_box of read-fonts
// src/model/glyph/color/instance.rs.
func modelResolveClipBox(inst *modelInstance, clipBox tables.ClipBox) BoundingBox {
	switch cbox := clipBox.(type) {
	case tables.ClipBoxFormat1:
		return BoundingBox{
			XMin: float32(cbox.XMin),
			YMin: float32(cbox.YMin),
			XMax: float32(cbox.XMax),
			YMax: float32(cbox.YMax),
		}
	case tables.ClipBoxFormat2:
		deltas := inst.varDeltas(cbox.VarIndexBase, 4)
		return BoundingBox{
			XMin: fwordDelta(float32(cbox.XMin), deltas[0]),
			YMin: fwordDelta(float32(cbox.YMin), deltas[1]),
			XMax: fwordDelta(float32(cbox.XMax), deltas[2]),
			YMax: fwordDelta(float32(cbox.YMax), deltas[3]),
		}
	}
	return BoundingBox{}
}

// modelClipboxFontUnits is get_clipbox_font_units of read-fonts
// src/model/glyph/color/traversal.rs.
func modelClipboxFontUnits(inst *modelInstance, glyphID GlyphID) (BoundingBox, bool) {
	clip, ok := inst.colr.ClipList.Search(tables.GlyphID(glyphID))
	if !ok || clip == nil {
		return BoundingBox{}, false
	}
	return modelResolveClipBox(inst, clip), true
}

// modelResolveColorStops is ColorStops::resolve of read-fonts
// src/model/glyph/color/instance.rs.
func modelResolveColorStops(inst *modelInstance, stops []tables.ColorStop, varStops []tables.VarColorStop) []ColorStop {
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
			Offset:       f2dot14ApplyDelta(stop.StopOffset, deltas[0]),
			PaletteIndex: stop.PaletteIndex,
			Alpha:        f2dot14ApplyDelta(stop.Alpha, deltas[1]),
		})
	}
	return out
}

// modelResolvePaint is resolve_paint of read-fonts
// src/model/glyph/color/instance.rs. It returns false where read-fonts
// returns an error.
//
// Port: read-fonts resolves the stops of a gradient in as_brush. The port
// resolves them here, which gives the same stops, because the location does
// not change during a walk. A PaintColrLayers keeps the range of its layers,
// which the walk reads one at a time, as v1_layer of read-fonts does.
func modelResolvePaint(inst *modelInstance, paint tables.PaintTable) (resolvedPaint, bool) {
	center := func(x, y float32) *Point { return &Point{x, y} }
	switch p := paint.(type) {
	case tables.PaintColrLayers:
		// A range that overflows, as it can on a 32-bit target, names no
		// layer the table could hold.
		if uint64(p.FirstLayerIndex)+uint64(p.NumLayers) > math.MaxInt32 {
			return resolvedPaint{}, false
		}
		return resolvedPaint{kind: resolvedColrLayers, firstLayer: p.FirstLayerIndex, numLayers: uint32(p.NumLayers)}, true
	case tables.PaintSolid:
		return resolvedPaint{kind: resolvedSolid, paletteIndex: p.PaletteIndex, alpha: f2dot14(p.Alpha)}, true
	case tables.PaintVarSolid:
		deltas := inst.varDeltas(p.VarIndexBase, 1)
		return resolvedPaint{kind: resolvedSolid, paletteIndex: p.PaletteIndex, alpha: f2dot14ApplyDelta(p.Alpha, deltas[0])}, true
	case tables.PaintLinearGradient:
		return resolvedPaint{
			kind: resolvedLinearGradient,
			x0:   float32(p.X0), y0: float32(p.Y0),
			x1: float32(p.X1), y1: float32(p.Y1),
			x2: float32(p.X2), y2: float32(p.Y2),
			colorStops: modelResolveColorStops(inst, p.ColorLine.ColorStops, nil),
			extend:     p.ColorLine.Extend,
		}, true
	case tables.PaintVarLinearGradient:
		deltas := inst.varDeltas(p.VarIndexBase, 6)
		return resolvedPaint{
			kind: resolvedLinearGradient,
			x0:   fwordDelta(float32(p.X0), deltas[0]), y0: fwordDelta(float32(p.Y0), deltas[1]),
			x1: fwordDelta(float32(p.X1), deltas[2]), y1: fwordDelta(float32(p.Y1), deltas[3]),
			x2: fwordDelta(float32(p.X2), deltas[4]), y2: fwordDelta(float32(p.Y2), deltas[5]),
			colorStops: modelResolveColorStops(inst, nil, p.ColorLine.ColorStops),
			extend:     p.ColorLine.Extend,
		}, true
	case tables.PaintRadialGradient:
		return resolvedPaint{
			kind: resolvedRadialGradient,
			x0:   float32(p.X0), y0: float32(p.Y0), radius0: float32(p.Radius0),
			x1: float32(p.X1), y1: float32(p.Y1), radius1: float32(p.Radius1),
			colorStops: modelResolveColorStops(inst, p.ColorLine.ColorStops, nil),
			extend:     p.ColorLine.Extend,
		}, true
	case tables.PaintVarRadialGradient:
		deltas := inst.varDeltas(p.VarIndexBase, 6)
		return resolvedPaint{
			kind: resolvedRadialGradient,
			x0:   fwordDelta(float32(p.X0), deltas[0]), y0: fwordDelta(float32(p.Y0), deltas[1]),
			radius0: fwordDelta(float32(p.Radius0), deltas[2]),
			x1:      fwordDelta(float32(p.X1), deltas[3]), y1: fwordDelta(float32(p.Y1), deltas[4]),
			radius1:    fwordDelta(float32(p.Radius1), deltas[5]),
			colorStops: modelResolveColorStops(inst, nil, p.ColorLine.ColorStops),
			extend:     p.ColorLine.Extend,
		}, true
	case tables.PaintSweepGradient:
		return resolvedPaint{
			kind: resolvedSweepGradient,
			x0:   float32(p.CenterX), y0: float32(p.CenterY),
			startAngle: f2dot14(p.StartAngle), endAngle: f2dot14(p.EndAngle),
			colorStops: modelResolveColorStops(inst, p.ColorLine.ColorStops, nil),
			extend:     p.ColorLine.Extend,
		}, true
	case tables.PaintVarSweepGradient:
		deltas := inst.varDeltas(p.VarIndexBase, 4)
		return resolvedPaint{
			kind: resolvedSweepGradient,
			x0:   fwordDelta(float32(p.CenterX), deltas[0]), y0: fwordDelta(float32(p.CenterY), deltas[1]),
			startAngle: f2dot14ApplyDelta(p.StartAngle, deltas[2]), endAngle: f2dot14ApplyDelta(p.EndAngle, deltas[3]),
			colorStops: modelResolveColorStops(inst, nil, p.ColorLine.ColorStops),
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
			fixedApplyDelta(a.Xx, deltas[0]), fixedApplyDelta(a.Yx, deltas[1]),
			fixedApplyDelta(a.Xy, deltas[2]), fixedApplyDelta(a.Yy, deltas[3]),
			fixedApplyDelta(a.Dx, deltas[4]), fixedApplyDelta(a.Dy, deltas[5]),
		}}, true
	case tables.PaintTranslate:
		return resolvedPaint{kind: resolvedTranslate, paint: p.Paint,
			ts: Transform{DX: float32(p.Dx), DY: float32(p.Dy)}}, true
	case tables.PaintVarTranslate:
		deltas := inst.varDeltas(p.VarIndexBase, 2)
		return resolvedPaint{kind: resolvedTranslate, paint: p.Paint,
			ts: Transform{DX: fwordDelta(float32(p.Dx), deltas[0]), DY: fwordDelta(float32(p.Dy), deltas[1])}}, true
	case tables.PaintScale:
		return resolvedPaint{kind: resolvedScale, paint: p.Paint,
			scaleX: f2dot14(p.ScaleX), scaleY: f2dot14(p.ScaleY)}, true
	case tables.PaintVarScale:
		deltas := inst.varDeltas(p.VarIndexBase, 2)
		return resolvedPaint{kind: resolvedScale, paint: p.Paint,
			scaleX: f2dot14ApplyDelta(p.ScaleX, deltas[0]), scaleY: f2dot14ApplyDelta(p.ScaleY, deltas[1])}, true
	case tables.PaintScaleAroundCenter:
		return resolvedPaint{kind: resolvedScale, paint: p.Paint,
			scaleX: f2dot14(p.ScaleX), scaleY: f2dot14(p.ScaleY),
			aroundCenter: center(float32(p.CenterX), float32(p.CenterY))}, true
	case tables.PaintVarScaleAroundCenter:
		deltas := inst.varDeltas(p.VarIndexBase, 4)
		return resolvedPaint{kind: resolvedScale, paint: p.Paint,
			scaleX: f2dot14ApplyDelta(p.ScaleX, deltas[0]), scaleY: f2dot14ApplyDelta(p.ScaleY, deltas[1]),
			aroundCenter: center(fwordDelta(float32(p.CenterX), deltas[2]), fwordDelta(float32(p.CenterY), deltas[3]))}, true
	case tables.PaintScaleUniform:
		s := f2dot14(p.Scale)
		return resolvedPaint{kind: resolvedScale, paint: p.Paint, scaleX: s, scaleY: s}, true
	case tables.PaintVarScaleUniform:
		deltas := inst.varDeltas(p.VarIndexBase, 1)
		s := f2dot14ApplyDelta(p.Scale, deltas[0])
		return resolvedPaint{kind: resolvedScale, paint: p.Paint, scaleX: s, scaleY: s}, true
	case tables.PaintScaleUniformAroundCenter:
		s := f2dot14(p.Scale)
		return resolvedPaint{kind: resolvedScale, paint: p.Paint, scaleX: s, scaleY: s,
			aroundCenter: center(float32(p.CenterX), float32(p.CenterY))}, true
	case tables.PaintVarScaleUniformAroundCenter:
		deltas := inst.varDeltas(p.VarIndexBase, 3)
		s := f2dot14ApplyDelta(p.Scale, deltas[0])
		return resolvedPaint{kind: resolvedScale, paint: p.Paint, scaleX: s, scaleY: s,
			aroundCenter: center(fwordDelta(float32(p.CenterX), deltas[1]), fwordDelta(float32(p.CenterY), deltas[2]))}, true
	case tables.PaintRotate:
		return resolvedPaint{kind: resolvedRotate, paint: p.Paint, angle: f2dot14(p.Angle)}, true
	case tables.PaintVarRotate:
		deltas := inst.varDeltas(p.VarIndexBase, 1)
		return resolvedPaint{kind: resolvedRotate, paint: p.Paint, angle: f2dot14ApplyDelta(p.Angle, deltas[0])}, true
	case tables.PaintRotateAroundCenter:
		return resolvedPaint{kind: resolvedRotate, paint: p.Paint, angle: f2dot14(p.Angle),
			aroundCenter: center(float32(p.CenterX), float32(p.CenterY))}, true
	case tables.PaintVarRotateAroundCenter:
		deltas := inst.varDeltas(p.VarIndexBase, 3)
		return resolvedPaint{kind: resolvedRotate, paint: p.Paint, angle: f2dot14ApplyDelta(p.Angle, deltas[0]),
			aroundCenter: center(fwordDelta(float32(p.CenterX), deltas[1]), fwordDelta(float32(p.CenterY), deltas[2]))}, true
	case tables.PaintSkew:
		return resolvedPaint{kind: resolvedSkew, paint: p.Paint,
			xSkewAngle: f2dot14(p.XSkewAngle), ySkewAngle: f2dot14(p.YSkewAngle)}, true
	case tables.PaintVarSkew:
		deltas := inst.varDeltas(p.VarIndexBase, 2)
		return resolvedPaint{kind: resolvedSkew, paint: p.Paint,
			xSkewAngle: f2dot14ApplyDelta(p.XSkewAngle, deltas[0]), ySkewAngle: f2dot14ApplyDelta(p.YSkewAngle, deltas[1])}, true
	case tables.PaintSkewAroundCenter:
		return resolvedPaint{kind: resolvedSkew, paint: p.Paint,
			xSkewAngle: f2dot14(p.XSkewAngle), ySkewAngle: f2dot14(p.YSkewAngle),
			aroundCenter: center(float32(p.CenterX), float32(p.CenterY))}, true
	case tables.PaintVarSkewAroundCenter:
		deltas := inst.varDeltas(p.VarIndexBase, 4)
		return resolvedPaint{kind: resolvedSkew, paint: p.Paint,
			xSkewAngle: f2dot14ApplyDelta(p.XSkewAngle, deltas[0]), ySkewAngle: f2dot14ApplyDelta(p.YSkewAngle, deltas[1]),
			aroundCenter: center(fwordDelta(float32(p.CenterX), deltas[2]), fwordDelta(float32(p.CenterY), deltas[3]))}, true
	case tables.PaintComposite:
		return resolvedPaint{kind: resolvedComposite, sourcePaint: p.SourcePaint, mode: p.CompositeMode,
			backdropPaint: p.BackdropPaint}, true
	}
	return resolvedPaint{}, false
}

// maybeBrush is the case of MaybeBrush of read-fonts
// src/model/glyph/color/instance.rs.
type maybeBrush uint8

// The cases of MaybeBrush of read-fonts.
const (
	// maybeBrushSome is MaybeBrush::Some: a brush to fill with.
	maybeBrushSome maybeBrush = iota
	// maybeBrushNonRendering is MaybeBrush::NonRendering: a valid brush
	// that paints nothing.
	maybeBrushNonRendering
	// maybeBrushNone is MaybeBrush::None: the paint is not a brush.
	maybeBrushNone
)

// asTransform is ResolvedPaint::as_transform of read-fonts
// src/model/glyph/color/instance.rs. It returns the transform of a paint of
// a transform and its child, or false for another paint.
func (paint *resolvedPaint) asTransform() (Transform, tables.PaintTable, bool) {
	switch paint.kind {
	case resolvedTransform, resolvedTranslate, resolvedScale, resolvedRotate, resolvedSkew:
		return paintTransformOf(paint), paint.paint, true
	}
	return Transform{}, nil, false
}

// asBrush is ResolvedPaint::as_brush of read-fonts
// src/model/glyph/color/instance.rs. It gives the brush of a paint of a
// fill, with its stops sorted and normalized. The walk of colr.go, of skrifa
// 0.44, makes the same brush in traverse_with_callbacks, so it calls this
// function too.
func (paint *resolvedPaint) asBrush() (Brush, maybeBrush) {
	switch paint.kind {
	case resolvedSolid:
		return Brush{Kind: BrushSolid, PaletteIndex: paint.paletteIndex, Alpha: paint.alpha}, maybeBrushSome
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
				return Brush{Kind: BrushSolid, PaletteIndex: stop.PaletteIndex, Alpha: stop.Alpha}, maybeBrushSome
			}
			return Brush{}, maybeBrushNonRendering
		}

		// Follow implementation note in nanoemoji:
		// https://github.com/googlefonts/nanoemoji/blob/0ac6e7bb4d8202db692574d8530a9b643f1b3b3c/src/nanoemoji/svg.py#L188
		// to compute a new gradient end point P3 as the orthogonal
		// projection of the vector from p0 to p1 onto a line perpendicular
		// to line p0p2 and passing through p0.
		perpendicularToP2 := sub(p2, p0)
		perpendicularToP2 = Point{perpendicularToP2.Y, -perpendicularToP2.X}
		proj := projectOnto(sub(p1, p0), perpendicularToP2)
		p3 := Point{p0.X + proj.X, p0.Y + proj.Y}

		if len(resolvedStops) == 0 {
			return Brush{}, maybeBrushNonRendering
		}
		firstStop := resolvedStops[0]
		lastStop := resolvedStops[len(resolvedStops)-1]
		colorStopRange := lastStop.Offset - firstStop.Offset

		// Nothing can be drawn for this situation.
		if colorStopRange == 0.0 && paint.extend != tables.ExtendPad {
			return Brush{}, maybeBrushNonRendering
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

		return Brush{
			Kind:       BrushLinearGradient,
			P0:         p0,
			P1:         p3,
			ColorStops: resolvedStops,
			Extend:     paint.extend,
		}, maybeBrushSome
	case resolvedRadialGradient:
		c0 := Point{paint.x0, paint.y0}
		c1 := Point{paint.x1, paint.y1}
		radius0 := paint.radius0
		radius1 := paint.radius1

		resolvedStops := makeSortedResolvedStops(paint.colorStops)
		if len(resolvedStops) == 0 {
			return Brush{}, maybeBrushNonRendering
		}
		firstStop := resolvedStops[0]
		lastStop := resolvedStops[len(resolvedStops)-1]
		colorStopRange := lastStop.Offset - firstStop.Offset
		// Nothing can be drawn for this situation.
		if colorStopRange == 0.0 && paint.extend != tables.ExtendPad {
			return Brush{}, maybeBrushNonRendering
		}

		// In the Pad case, for providing normalized stops in the 0 to 1 range to the client,
		// insert a color stop at the end. See LinearGradient for more details.
		if colorStopRange == 0.0 && paint.extend == tables.ExtendPad {
			extraStop := lastStop
			extraStop.Offset += 1.0
			resolvedStops = append(resolvedStops, extraStop)
			colorStopRange = 1.0
		}

		// If the colorStopRange is 0 at this point, the default behavior of the shader is to
		// clamp to 1 color stops that are above 1, clamp to 0 for color stops that are below 0,
		// and repeat the outer color stops at 0 and 1 if the color stops are inside the
		// range. That will result in the correct rendering.
		if colorStopRange != 1.0 || firstStop.Offset != 0.0 {
			c0ToC1 := Point{c1.X - c0.X, c1.Y - c0.Y}
			radiusDiff := radius1 - radius0
			scaleFactor := 1.0 / colorStopRange

			c0Offset := Point{float32(c0ToC1.X * firstStop.Offset), float32(c0ToC1.Y * firstStop.Offset)}
			c1Offset := Point{float32(c0ToC1.X * lastStop.Offset), float32(c0ToC1.Y * lastStop.Offset)}
			stopsStartOffset := firstStop.Offset

			// Order of reassignments is important to avoid shadowing variables.
			c1 = Point{c0.X + c1Offset.X, c0.Y + c1Offset.Y}
			c0 = Point{c0.X + c0Offset.X, c0.Y + c0Offset.Y}
			radius1 = radius0 + float32(radiusDiff*lastStop.Offset)
			radius0 += float32(radiusDiff * firstStop.Offset)

			for i := range resolvedStops {
				resolvedStops[i].Offset = (resolvedStops[i].Offset - stopsStartOffset) * scaleFactor
			}
		}

		return Brush{
			Kind:       BrushRadialGradient,
			C0:         c0,
			R0:         radius0,
			C1:         c1,
			R1:         radius1,
			ColorStops: resolvedStops,
			Extend:     paint.extend,
		}, maybeBrushSome
	case resolvedSweepGradient:
		// OpenType 1.9.1 adds a shift to the angle to ease specification of a 0 to 360
		// degree sweep.
		sweepAngleToDegrees := func(angle float32) float32 { return float32(angle*180.0) + 180.0 }

		startAngle := sweepAngleToDegrees(paint.startAngle)
		endAngle := sweepAngleToDegrees(paint.endAngle)

		// Stop normalization for sweep:

		sectorAngle := endAngle - startAngle

		resolvedStops := makeSortedResolvedStops(paint.colorStops)
		if len(resolvedStops) == 0 {
			return Brush{}, maybeBrushNonRendering
		}

		firstStop := resolvedStops[0]
		lastStop := resolvedStops[len(resolvedStops)-1]
		colorStopRange := lastStop.Offset - firstStop.Offset

		startAngleScaled := startAngle + float32(sectorAngle*firstStop.Offset)
		endAngleScaled := startAngle + float32(sectorAngle*lastStop.Offset)

		startOffset := firstStop.Offset

		// Nothing can be drawn for this situation.
		if colorStopRange == 0.0 && paint.extend != tables.ExtendPad {
			return Brush{}, maybeBrushNonRendering
		}

		// In the Pad case, if the color_stop_range is 0 insert a color stop at the end before
		// normalizing. Adding this stop will paint the equivalent gradient, because: All font
		// specified color stops are in the same spot, mode is pad, so everything before this
		// spot is painted with the first color, everything after this spot is painted with
		// the last color. Not adding this stop will skip the projection and result in
		// specifying non-normalized color stops to the shader.
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
		// "If the color line's extend mode is reflect or repeat
		// and start and end angle are equal, nothing shall be drawn."
		if startAngleScaled == endAngleScaled && paint.extend != tables.ExtendPad {
			return Brush{}, maybeBrushNonRendering
		}

		return Brush{
			Kind:       BrushSweepGradient,
			C0:         Point{paint.x0, paint.y0},
			StartAngle: startAngleScaled,
			EndAngle:   endAngleScaled,
			ColorStops: resolvedStops,
			Extend:     paint.extend,
		}, maybeBrushSome
	}
	return Brush{}, maybeBrushNone
}

// maxNodes is MAX_NODES of read-fonts src/model/glyph/color/traversal.rs:
// the largest number of nodes that one walk visits. It stops a graph with
// a high fan-out from taking too long.
const maxNodes = 16384

// modelDecycler is Decycler of read-fonts
// src/model/glyph/color/decycler.rs, with the depth MAX_RECURSION_DEPTH of
// read-fonts src/limits.rs. It finds a cycle when the id of a node is the
// id of the node at half the depth, so a cycle is found after it repeats
// at most twice.
//
// Port: read-fonts takes the offset of a paint as its id. go-text gives no
// offsets, so the walk takes the index of a layer in the LayerList, or the
// glyph of a base glyph above 1<<32, as the walk of colr.go does.
type modelDecycler struct {
	nodeIDs [maxTraversalDepth]uint64
	depth   int
}

// enter is Decycler::enter of read-fonts. It returns false for a cycle or a
// walk that is too deep. The caller calls leave when it returns, as the
// guard of read-fonts does when it drops.
func (d *modelDecycler) enter(id uint64) bool {
	if d.depth >= len(d.nodeIDs) {
		return false
	}
	if d.depth != 0 && d.nodeIDs[d.depth/2] == id {
		return false
	}
	d.nodeIDs[d.depth] = id
	d.depth++
	return true
}

// leave is the Drop of DecyclerGuard of read-fonts.
func (d *modelDecycler) leave() {
	d.depth--
}

// traversalState is TraversalState of read-fonts
// src/model/glyph/color/traversal.rs.
type traversalState struct {
	inst      *modelInstance
	sink      Painter
	nodesLeft uint32
}

// resolvePaint is TraversalState::resolve_paint of read-fonts. Each paint
// that it resolves counts against the limit of maxNodes.
func (s *traversalState) resolvePaint(paint tables.PaintTable) (resolvedPaint, bool) {
	if s.nodesLeft == 0 {
		return resolvedPaint{}, false
	}
	s.nodesLeft--
	return modelResolvePaint(s.inst, paint)
}

// modelTraverse is traverse_with_callbacks of read-fonts
// src/model/glyph/color/traversal.rs. It returns false where read-fonts
// returns an error. A walk that fails still pops what it pushed.
func modelTraverse(paint *resolvedPaint, state *traversalState, decycler *modelDecycler, recurseDepth int) bool {
	if recurseDepth >= maxTraversalDepth {
		return false
	}
	switch paint.kind {
	case resolvedColrLayers:
		for i := range paint.numLayers {
			layerIndex := paint.firstLayer + i
			layers, err := state.inst.colr.LayerList.Resolve(tables.PaintColrLayers{FirstLayerIndex: layerIndex, NumLayers: 1})
			if err != nil {
				return false
			}
			// Perform cycle detection with paint id here.
			if !decycler.enter(uint64(layerIndex)) {
				return false
			}
			rp, ok := state.resolvePaint(layers[0])
			ok = ok && modelTraverse(&rp, state, decycler, recurseDepth+1)
			decycler.leave()
			if !ok {
				return false
			}
		}
		return true
	case resolvedSolid, resolvedLinearGradient, resolvedRadialGradient, resolvedSweepGradient:
		if brush, kind := paint.asBrush(); kind == maybeBrushSome {
			state.sink.Fill(brush)
		}
		return true
	case resolvedGlyph:
		glyph := GlyphID(paint.glyphID)
		// Look for the pattern `(transform)* fill` and optimize it to a
		// single paint call
		nextPaint, ok := state.resolvePaint(paint.paint)
		if !ok {
			return false
		}
		// Collect any chain of intermediate transforms
		var intermediateTransform *Transform
		for {
			transform, childPaint, isTransform := nextPaint.asTransform()
			if !isTransform {
				break
			}
			if intermediateTransform != nil {
				t := intermediateTransform.Mul(transform)
				intermediateTransform = &t
			} else {
				intermediateTransform = &transform
			}
			if nextPaint, ok = state.resolvePaint(childPaint); !ok {
				return false
			}
		}
		// If the next paint is a brush, we can optimize the traversal to a
		// single fill_glyph call
		switch brush, kind := nextPaint.asBrush(); kind {
		case maybeBrushSome:
			state.sink.FillGlyph(glyph, intermediateTransform, brush)
			return true
		case maybeBrushNonRendering:
			// Valid brush but doesn't produce any rendering, so we can
			// skip the glyph entirely
			return true
		}
		// In case the optimization was not successful, just push a clip,
		// and continue unoptimized traversal.
		state.sink.PushClipGlyph(glyph)
		if intermediateTransform != nil {
			state.sink.PushTransform(*intermediateTransform)
		}
		result := modelTraverse(&nextPaint, state, decycler, recurseDepth+1)
		if intermediateTransform != nil {
			state.sink.PopTransform()
		}
		state.sink.PopClip()
		return result
	case resolvedColrGlyph:
		glyph := GlyphID(paint.glyphID)
		baseGlyph, ok := state.inst.colr.Search(tables.GlyphID(glyph))
		if _, v0 := baseGlyph.(tables.PaintColrLayersResolved); !ok || v0 {
			// Error::MissingResource
			return false
		}
		if !decycler.enter(1<<32 | uint64(glyph)) {
			return false
		}
		defer decycler.leave()
		// Port: the PaintSink of usvg paints no cached color glyph, so the
		// walk always takes the case PaintCachedColorGlyph::Unimplemented.
		clipbox, hasClipbox := modelClipboxFontUnits(state.inst, glyph)
		if hasClipbox {
			state.sink.PushClipBox(clipbox)
		}
		result := modelTraverseUnresolved(baseGlyph, state, decycler, recurseDepth+1)
		if hasClipbox {
			state.sink.PopClip()
		}
		return result
	case resolvedTransform, resolvedTranslate, resolvedScale, resolvedRotate, resolvedSkew:
		transform, nextPaint, _ := paint.asTransform()
		state.sink.PushTransform(transform)
		result := modelTraverseUnresolved(nextPaint, state, decycler, recurseDepth+1)
		state.sink.PopTransform()
		return result
	case resolvedComposite:
		// Port: the PaintSink of usvg has no pop_layer_with_mode, so the
		// default of read-fonts calls pop_layer.
		state.sink.PushLayer(tables.CompositeSrcOver)
		if !modelTraverseUnresolved(paint.backdropPaint, state, decycler, recurseDepth+1) {
			state.sink.PopLayer()
			return false
		}
		state.sink.PushLayer(paint.mode)
		result := modelTraverseUnresolved(paint.sourcePaint, state, decycler, recurseDepth+1)
		state.sink.PopLayer()
		state.sink.PopLayer()
		return result
	}
	return false
}

// modelTraverseUnresolved is traverse_unresolved_paint of read-fonts
// src/model/glyph/color/traversal.rs.
func modelTraverseUnresolved(paint tables.PaintTable, state *traversalState, decycler *modelDecycler, recurseDepth int) bool {
	rp, ok := state.resolvePaint(paint)
	if !ok {
		return false
	}
	return modelTraverse(&rp, state, decycler, recurseDepth)
}

// PaintModel is font.color_glyphs().get(glyph_id) and ColorGlyph::paint of
// read-fonts 0.46 src/model/glyph/color.rs, which usvg calls through skrifa
// 0.49 (D100). It paints the color glyph of glyphID of ft at coords,
// preferring a COLRv1 representation over a COLRv0 one, and returns false
// when the glyph has none or painting fails.
func PaintModel(ft *font.Font, coords []tables.Coord, glyphID GlyphID, painter Painter) bool {
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
	inst := &modelInstance{colr: ft.COLR, coords: effective}

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

	clipbox, hasClipbox := modelClipboxFontUnits(inst, glyphID)
	if hasClipbox {
		painter.PushClipBox(clipbox)
	}
	var decycler modelDecycler
	if !decycler.enter(1<<32 | uint64(glyphID)) {
		return false
	}
	state := &traversalState{inst: inst, sink: painter, nodesLeft: maxNodes}
	rp, ok := state.resolvePaint(paint)
	result := ok && modelTraverse(&rp, state, &decycler, 0)
	decycler.leave()
	if hasClipbox {
		painter.PopClip()
	}
	return result
}
