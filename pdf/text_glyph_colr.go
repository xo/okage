// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"github.com/xo/okage/internal/colr"
	"github.com/xo/okage/path"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// This file ports krilla src/text/glyph/colr.rs. skrifa walks the paint
// graph of a COLR glyph and calls the methods of the trait ColorPainter,
// which ColrBuilder implements. The port walks it with colr.Paint of
// internal/colr, the walk of D32, which colrBuilder implements as
// colr.Painter.

// hasColrData is has_colr_data in krilla src/text/glyph/colr.rs.
func hasColrData(font *Font, glyph GlyphID) bool {
	if font.ft.COLR == nil || glyph > 0xFFFF {
		return false
	}
	_, ok := font.ft.COLR.Search(tables.GlyphID(glyph))
	return ok
}

// drawColrGlyph is draw_glyph in krilla src/text/glyph/colr.rs. It returns
// false where upstream returns None.
//
// Draw a COLR-based glyph on a surface.
func drawColrGlyph(font *Font, contextColor ColorRGB, glyph GlyphID, surface *Surface) bool {
	// Drawing COLR glyphs is a bit tricky, because it's possible that an
	// error occurs while we are drawing, in which case we cannot revert it
	// anymore since we already drew the instructions onto the surface.
	// Because of this, we first convert the glyph into a more accessible
	// bytecode representation and only if that succeeds do we iterate over
	// the bytecode to draw onto the canvas.
	if !hasColrData(font, glyph) {
		return false
	}

	colrCanvas := newColrBuilder(font, contextColor)
	if !colr.Paint(font.ft, font.info.location, colr.GlyphID(glyph), colrCanvas) {
		return false
	}
	instructions, bounds, hasBounds, ok := colrCanvas.finish()
	if !ok {
		return false
	}
	if !hasBounds || bounds.IsEmpty() {
		return true
	}
	boundsPath := path.PathFromRect(bounds)

	surface.PushTransform(path.TransformFromScale(1.0, -1.0))
	interpretColr(instructions, boundsPath, surface)
	surface.Pop()

	return true
}

// interpretColr is interpret in krilla src/text/glyph/colr.rs.
//
// Interpret the glyph bytecode.
func interpretColr(instructions []colrInstruction, bounds *Path, surface *Surface) {
	for _, instruction := range instructions {
		switch instruction.kind {
		case colrInstructionLayer:
			blend, ok := compositeModeToBlendMode(instruction.mode)
			if !ok {
				blend = BlendModeNormal
			}
			surface.PushBlendMode(blend)
			surface.PushIsolated()
			interpretColr(instruction.source, bounds, surface)
			surface.Pop()
			surface.Pop()
		case colrInstructionComposite:
			interpretComposite(instruction.source, instruction.mode, instruction.backdrop, bounds, surface)
		case colrInstructionFilled:
			clips := instruction.clips
			filled := bounds
			if n := len(clips); n != 0 {
				filled = clips[n-1]
				clips = clips[:n-1]
			}

			numClips := len(clips)
			for _, p := range clips {
				surface.PushClipPath(p, FillRuleNonZero)
			}

			oldFill := surface.GetFill()
			oldStroke := surface.GetStroke()

			fill := instruction.fill
			surface.SetFill(&fill)
			surface.SetStroke(nil)

			surface.DrawPath(filled)

			surface.SetFill(oldFill)
			surface.SetStroke(oldStroke)

			for range numClips {
				surface.Pop()
			}
		}
	}
}

// interpretComposite is interpret_composite in krilla
// src/text/glyph/colr.rs.
func interpretComposite(source []colrInstruction, mode tables.CompositeMode, backdrop []colrInstruction, bounds *Path, surface *Surface) {
	switch mode {
	case tables.CompositeClear:
	case tables.CompositeSrc:
		surface.PushIsolated()
		interpretColr(source, bounds, surface)
		surface.Pop()
	case tables.CompositeDest:
		surface.PushIsolated()
		interpretColr(backdrop, bounds, surface)
		surface.Pop()
	case tables.CompositeSrcOver:
		surface.PushIsolated()
		interpretColr(backdrop, bounds, surface)
		interpretColr(source, bounds, surface)
		surface.Pop()
	case tables.CompositeDestOver:
		surface.PushIsolated()
		interpretColr(source, bounds, surface)
		interpretColr(backdrop, bounds, surface)
		surface.Pop()
	case tables.CompositeSrcIn:
		interpretMasked(source, backdrop, bounds, surface)
	case tables.CompositeDestIn:
		interpretMasked(backdrop, source, bounds, surface)
	default:
		blend, ok := compositeModeToBlendMode(mode)
		if !ok {
			surface.PushIsolated()
			interpretColr(backdrop, bounds, surface)
			interpretColr(source, bounds, surface)
			surface.Pop()
			return
		}

		surface.PushIsolated()
		interpretColr(backdrop, bounds, surface)
		surface.PushBlendMode(blend)
		surface.PushIsolated()
		interpretColr(source, bounds, surface)
		surface.Pop()
		surface.Pop()
		surface.Pop()
	}
}

// interpretMasked is interpret_masked in krilla src/text/glyph/colr.rs.
func interpretMasked(visible, mask []colrInstruction, bounds *Path, surface *Surface) {
	builder := surface.StreamBuilder()
	maskSurface := builder.Surface()
	interpretColr(mask, bounds, maskSurface)
	maskSurface.Finish()
	maskStream := builder.Finish()

	surface.PushMask(NewMask(maskStream, MaskTypeAlpha))
	interpretColr(visible, bounds, surface)
	surface.Pop()
}

// colrInstructionKind is the case of a colrInstruction.
type colrInstructionKind uint8

// The cases of Instruction in krilla src/text/glyph/colr.rs.
const (
	colrInstructionLayer colrInstructionKind = iota
	colrInstructionComposite
	colrInstructionFilled
)

// colrInstruction is Instruction in krilla src/text/glyph/colr.rs. The enum
// has data, so it becomes a struct with a kind. Layer has mode and source,
// Composite has source, mode and backdrop, and Filled has fill and clips.
//
// A bytecode instruction for drawing a COLR glyph.
type colrInstruction struct {
	kind     colrInstructionKind
	mode     tables.CompositeMode
	source   []colrInstruction
	backdrop []colrInstruction
	fill     Fill
	clips    []*Path
}

// compositeModeToBlendMode is CompositeModeExt::to_blend_mode in krilla
// src/text/glyph/colr.rs.
func compositeModeToBlendMode(mode tables.CompositeMode) (BlendMode, bool) {
	switch mode {
	case tables.CompositeSrcOver:
		return BlendModeNormal, true
	case tables.CompositeScreen:
		return BlendModeScreen, true
	case tables.CompositeOverlay:
		return BlendModeOverlay, true
	case tables.CompositeDarken:
		return BlendModeDarken, true
	case tables.CompositeLighten:
		return BlendModeLighten, true
	case tables.CompositeColorDodge:
		return BlendModeColorDodge, true
	case tables.CompositeColorBurn:
		return BlendModeColorBurn, true
	case tables.CompositeHardLight:
		return BlendModeHardLight, true
	case tables.CompositeSoftLight:
		return BlendModeSoftLight, true
	case tables.CompositeDifference:
		return BlendModeDifference, true
	case tables.CompositeExclusion:
		return BlendModeExclusion, true
	case tables.CompositeMultiply:
		return BlendModeMultiply, true
	case tables.CompositeHslHue:
		return BlendModeHue, true
	case tables.CompositeHslColor:
		return BlendModeColor, true
	case tables.CompositeHslLuminosity:
		return BlendModeLuminosity, true
	case tables.CompositeHslSaturation:
		return BlendModeSaturation, true
	}
	return 0, false
}

// colrStackEntry is an item (Vec<Instruction>, bool) of the stack of
// ColrBuilder: the instructions of a layer and whether they are bounded.
type colrStackEntry struct {
	instructions []colrInstruction
	bounded      bool
}

// colrBuilder is ColrBuilder in krilla src/text/glyph/colr.rs.
//
// The context necessary for creating the bytecode of a COLR-based glyph.
type colrBuilder struct {
	font         *Font
	contextColor ColorRGB
	clips        [][]*Path
	stack        []colrStackEntry
	bounds       Rect
	hasBounds    bool
	layers       []tables.CompositeMode
	transforms   []Transform
	err          bool
}

// newColrBuilder is ColrBuilder::new in krilla src/text/glyph/colr.rs.
func newColrBuilder(font *Font, contextColor ColorRGB) *colrBuilder {
	return &colrBuilder{
		font:         font,
		contextColor: contextColor,
		stack:        []colrStackEntry{{bounded: true}},
		transforms:   []Transform{path.IdentityTransform()},
		clips:        [][]*Path{nil},
	}
}

// finish is ColrBuilder::finish in krilla src/text/glyph/colr.rs. The last
// result is false where upstream returns None.
func (b *colrBuilder) finish() ([]colrInstruction, Rect, bool, bool) {
	if b.err || len(b.stack) == 0 {
		return nil, Rect{}, false, false
	}
	e := b.stack[len(b.stack)-1]
	b.stack = b.stack[:len(b.stack)-1]
	// A color glyph needs to be bounded to be considered valid.
	if !e.bounded {
		return nil, Rect{}, false, false
	}
	return e.instructions, b.bounds, b.hasBounds, true
}

// pushClip is ColrBuilder::push_clip in krilla src/text/glyph/colr.rs.
func (b *colrBuilder) pushClip(p *Path) {
	if len(b.clips) == 0 {
		b.err = true
		return
	}
	last := b.clips[len(b.clips)-1]
	clips := make([]*Path, len(last), len(last)+1)
	copy(clips, last)
	if b.hasBounds {
		joined, ok := b.bounds.Join(p.Bounds())
		if !ok {
			b.err = true
			return
		}
		b.bounds = joined
	} else {
		b.bounds, b.hasBounds = p.Bounds(), true
	}
	clips = append(clips, p)
	b.clips = append(b.clips, clips)
}

// paletteIndexToColor is ColrBuilder::palette_index_to_color in krilla
// src/text/glyph/colr.rs.
func (b *colrBuilder) paletteIndexToColor(paletteIndex uint16, alpha float32) (ColorRGB, NormalizedF32, bool) {
	alpha = min(max(alpha, 0), 1)

	if paletteIndex != 0xFFFF {
		palettes := b.font.ft.CPAL
		if len(palettes) == 0 {
			return ColorRGB{}, NormalizedF32(0), false
		}
		palette := palettes[0]
		if int(paletteIndex) >= len(palette) {
			return ColorRGB{}, NormalizedF32(0), false
		}
		color := palette[paletteIndex]
		a, ok := NewNormalizedF32(float32(alpha*float32(color.Alpha)) / 255.0)
		if !ok {
			return ColorRGB{}, NormalizedF32(0), false
		}
		return NewColorRGB(color.Red, color.Green, color.Blue), a, true
	}
	a, ok := NewNormalizedF32(alpha)
	if !ok {
		return ColorRGB{}, NormalizedF32(0), false
	}
	return b.contextColor, a, true
}

// stops is ColrBuilder::stops in krilla src/text/glyph/colr.rs.
func (b *colrBuilder) stops(stops []colr.ColorStop) ([]Stop, bool) {
	converted := make([]Stop, 0, len(stops))
	for _, stop := range stops {
		color, alpha, ok := b.paletteIndexToColor(stop.PaletteIndex, stop.Alpha)
		if !ok {
			return nil, false
		}
		offset, ok := NewNormalizedF32(stop.Offset)
		if !ok {
			return nil, false
		}
		converted = append(converted, Stop{Offset: offset, Color: color, Opacity: alpha})
	}
	return converted, true
}

// extendToSpreadMethod is ExtendExt::to_spread_method in krilla
// src/text/glyph/colr.rs.
func extendToSpreadMethod(e tables.Extend) SpreadMethod {
	switch e {
	case tables.ExtendRepeat:
		return SpreadMethodRepeat
	case tables.ExtendReflect:
		return SpreadMethodReflect
	}
	return SpreadMethodPad
}

// colrTransformToTransform is Transform::from_row with the elements of a
// transform of skrifa, as ColrBuilder::push_transform calls it.
func colrTransformToTransform(t colr.Transform) Transform {
	return path.TransformFromRow(t.XX, t.YX, t.XY, t.YY, t.DX, t.DY)
}

// PushTransform is ColorPainter::push_transform for ColrBuilder in krilla
// src/text/glyph/colr.rs.
func (b *colrBuilder) PushTransform(transform colr.Transform) {
	if len(b.transforms) == 0 {
		b.err = true
		return
	}
	last := b.transforms[len(b.transforms)-1]
	b.transforms = append(b.transforms, last.PreConcat(colrTransformToTransform(transform)))
}

// PopTransform is ColorPainter::pop_transform for ColrBuilder in krilla
// src/text/glyph/colr.rs.
func (b *colrBuilder) PopTransform() {
	if len(b.transforms) == 0 {
		b.err = true
		return
	}
	b.transforms = b.transforms[:len(b.transforms)-1]
}

// FillGlyph is the default ColorPainter::fill_glyph of skrifa
// src/color/mod.rs, which ColrBuilder does not override.
func (b *colrBuilder) FillGlyph(glyphID colr.GlyphID, brushTransform *colr.Transform, brush colr.Brush) {
	b.PushClipGlyph(glyphID)
	if brushTransform != nil {
		b.PushTransform(*brushTransform)
		b.Fill(brush)
		b.PopTransform()
	} else {
		b.Fill(brush)
	}
	b.PopClip()
}

// PushClipGlyph is ColorPainter::push_clip_glyph for ColrBuilder in krilla
// src/text/glyph/colr.rs.
func (b *colrBuilder) PushClipGlyph(glyphID colr.GlyphID) {
	glyphBuilder := newOutlineBuilder()
	if !b.font.drawSkrifaOutline(GlyphID(glyphID), glyphBuilder) {
		b.err = true
		return
	}
	p, ok := glyphBuilder.finish()
	if !ok || len(b.transforms) == 0 {
		b.err = true
		return
	}
	p, ok = p.Transform(b.transforms[len(b.transforms)-1])
	if !ok {
		b.err = true
		return
	}
	b.pushClip(p)
}

// PushClipBox is ColorPainter::push_clip_box for ColrBuilder in krilla
// src/text/glyph/colr.rs.
func (b *colrBuilder) PushClipBox(clipBox colr.BoundingBox) {
	pb := path.NewPathBuilder()
	pb.MoveTo(clipBox.XMin, clipBox.YMin)
	pb.LineTo(clipBox.XMin, clipBox.YMax)
	pb.LineTo(clipBox.XMax, clipBox.YMax)
	pb.LineTo(clipBox.XMax, clipBox.YMin)
	pb.Close()

	p, ok := pb.Finish()
	if !ok || len(b.transforms) == 0 {
		b.err = true
		return
	}
	p, ok = p.Transform(b.transforms[len(b.transforms)-1])
	if !ok {
		b.err = true
		return
	}
	b.pushClip(p)
}

// PopClip is ColorPainter::pop_clip for ColrBuilder in krilla
// src/text/glyph/colr.rs.
func (b *colrBuilder) PopClip() {
	if len(b.clips) == 0 {
		b.err = true
		return
	}
	b.clips = b.clips[:len(b.clips)-1]
}

// Fill is ColorPainter::fill for ColrBuilder in krilla
// src/text/glyph/colr.rs.
func (b *colrBuilder) Fill(brush colr.Brush) {
	var fill Fill
	switch brush.Kind {
	case colr.BrushSolid:
		color, alpha, ok := b.paletteIndexToColor(brush.PaletteIndex, brush.Alpha)
		if !ok {
			b.err = true
			return
		}
		fill = Fill{Paint: color, Opacity: alpha, Rule: FillRuleNonZero}
	case colr.BrushLinearGradient:
		stops, ok := b.stops(brush.ColorStops)
		if !ok || len(b.transforms) == 0 {
			b.err = true
			return
		}
		fill = Fill{
			Paint: LinearGradient{
				X1:           brush.P0.X,
				Y1:           brush.P0.Y,
				X2:           brush.P1.X,
				Y2:           brush.P1.Y,
				Stops:        stops,
				SpreadMethod: extendToSpreadMethod(brush.Extend),
				Transform:    b.transforms[len(b.transforms)-1],
			},
			Opacity: NormalizedF32One,
			Rule:    FillRuleNonZero,
		}
	case colr.BrushRadialGradient:
		stops, ok := b.stops(brush.ColorStops)
		if !ok || len(b.transforms) == 0 {
			b.err = true
			return
		}
		fill = Fill{
			Paint: RadialGradient{
				Fx:           brush.C0.X,
				Fy:           brush.C0.Y,
				Fr:           brush.R0,
				Cx:           brush.C1.X,
				Cy:           brush.C1.Y,
				Cr:           brush.R1,
				Stops:        stops,
				SpreadMethod: extendToSpreadMethod(brush.Extend),
				Transform:    b.transforms[len(b.transforms)-1],
			},
			Opacity: NormalizedF32One,
			Rule:    FillRuleNonZero,
		}
	case colr.BrushSweepGradient:
		stops, ok := b.stops(brush.ColorStops)
		if !ok || len(b.transforms) == 0 {
			b.err = true
			return
		}
		// krilla sweep gradients go in a different direction than COLR, so
		// we need to invert y-axis.
		transform := b.transforms[len(b.transforms)-1].PreConcat(path.TransformFromScale(1.0, -1.0))
		fill = Fill{
			Paint: SweepGradient{
				Cx:           brush.C0.X,
				Cy:           -brush.C0.Y,
				StartAngle:   brush.StartAngle,
				EndAngle:     brush.EndAngle,
				Stops:        stops,
				SpreadMethod: extendToSpreadMethod(brush.Extend),
				Transform:    transform,
			},
			Opacity: NormalizedF32One,
			Rule:    FillRuleNonZero,
		}
	default:
		return
	}

	// The proper implementation would be to apply all clip paths and then
	// draw the whole "visible" area with the fill. However, this seems to
	// produce artifacts in Google Chrome when zooming. So instead, what we
	// do is that we apply all clip paths except for the last one, and the
	// last one we use to actually perform the fill.
	if len(b.clips) == 0 || len(b.stack) == 0 {
		b.err = true
		return
	}
	clips := append([]*Path(nil), b.clips[len(b.clips)-1]...)
	top := &b.stack[len(b.stack)-1]
	top.bounded = top.bounded && len(clips) != 0
	top.instructions = append(top.instructions, colrInstruction{kind: colrInstructionFilled, fill: fill, clips: clips})
}

// PushLayer is ColorPainter::push_layer for ColrBuilder in krilla
// src/text/glyph/colr.rs.
func (b *colrBuilder) PushLayer(compositeMode tables.CompositeMode) {
	b.layers = append(b.layers, compositeMode)
	b.stack = append(b.stack, colrStackEntry{bounded: true})
}

// PopLayer is ColorPainter::pop_layer for ColrBuilder in krilla
// src/text/glyph/colr.rs.
func (b *colrBuilder) PopLayer() {
	if len(b.layers) == 0 || len(b.stack) == 0 {
		b.err = true
		return
	}
	composite := b.layers[len(b.layers)-1]
	b.layers = b.layers[:len(b.layers)-1]
	e := b.stack[len(b.stack)-1]
	b.stack = b.stack[:len(b.stack)-1]
	instructions, bounded := e.instructions, e.bounded

	if len(b.stack) == 0 {
		b.err = true
		return
	}
	parent := &b.stack[len(b.stack)-1]

	// See the skrifa code to see how it handles composite operations.
	// Everything is wrapped in a SrcOver base layer.
	switch composite {
	case tables.CompositeClear:
		parent.bounded = true
	case tables.CompositeSrc, tables.CompositeSrcOut:
		parent.bounded = bounded
	case tables.CompositeDest, tables.CompositeDestOut:
	case tables.CompositeSrcIn, tables.CompositeDestIn:
		parent.bounded = bounded || parent.bounded
	default:
		parent.bounded = bounded && parent.bounded
	}
	if composite == tables.CompositeSrcOver {
		if n := len(instructions); n != 0 && instructions[n-1].kind == colrInstructionLayer {
			last := instructions[n-1]
			parent.instructions = append(parent.instructions, colrInstruction{
				kind:     colrInstructionComposite,
				source:   last.source,
				mode:     last.mode,
				backdrop: instructions[:n-1],
			})
			return
		}
	}

	parent.instructions = append(parent.instructions, colrInstruction{kind: colrInstructionLayer, mode: composite, source: instructions})
}
