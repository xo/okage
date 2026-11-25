// Copyright 2024 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"strconv"

	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/font/opentype/tables"

	colrpaint "github.com/xo/okage/internal/colr"
	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/internal/xmlwriter"
	"github.com/xo/okage/path"
)

// This file ports usvg src/text/colr.rs.
//
// Port: the trait PaintSink of read-fonts 0.46 becomes the interface Painter
// of internal/colr, whose function PaintModel walks the paint graph of a
// COLR glyph on the tables of go-text (D9, D100). A text that upstream writes with `{}` of an
// f32 is written with fmtF32, which writes the same text.

// colrBuilder is Builder in usvg src/text/colr.rs.
type colrBuilder struct {
	path *[]byte
	minX float32
	minY float32
	maxX float32
	maxY float32
}

// newColrBuilder is Builder::new in usvg src/text/colr.rs.
func newColrBuilder(p *[]byte) *colrBuilder {
	const f32Max = 3.40282346638528859811704183484516925440e+38
	return &colrBuilder{
		path: p,
		minX: f32Max,
		minY: f32Max,
		maxX: -f32Max,
		maxY: -f32Max,
	}
}

// addPoint is Builder::add_point in usvg src/text/colr.rs.
func (b *colrBuilder) addPoint(x, y float32) {
	b.minX = minF32(b.minX, x)
	b.minY = minF32(b.minY, y)
	b.maxX = maxF32(b.maxX, x)
	b.maxY = maxF32(b.maxY, y)
}

// bounds is Builder::bounds in usvg src/text/colr.rs. It returns a
// conservative bounding box of the written path.
// It includes curve control points, so it can be larger than the exact
// bounding box, but never smaller.
func (b *colrBuilder) bounds() (Rect, bool) {
	if b.minX <= b.maxX && b.minY <= b.maxY {
		return path.RectFromLTRB(b.minX, b.minY, b.maxX, b.maxY)
	}
	return Rect{}, false
}

// finish is Builder::finish in usvg src/text/colr.rs.
func (b *colrBuilder) finish() {
	if len(*b.path) != 0 {
		*b.path = (*b.path)[:len(*b.path)-1] // remove trailing space
	}
}

// writeCmd appends a command and its numbers, each followed by a space, to
// the path, as the write! calls of the OutlinePen of Builder do.
func (b *colrBuilder) writeCmd(cmd string, nums ...float32) {
	*b.path = append(*b.path, cmd...)
	*b.path = append(*b.path, ' ')
	for _, n := range nums {
		*b.path = appendF32(*b.path, n)
		*b.path = append(*b.path, ' ')
	}
}

// MoveTo is OutlinePen::move_to for Builder in usvg src/text/colr.rs.
func (b *colrBuilder) MoveTo(x, y float32) {
	b.addPoint(x, y)
	b.writeCmd("M", x, y)
}

// LineTo is OutlinePen::line_to for Builder in usvg src/text/colr.rs.
func (b *colrBuilder) LineTo(x, y float32) {
	b.addPoint(x, y)
	b.writeCmd("L", x, y)
}

// QuadTo is OutlinePen::quad_to for Builder in usvg src/text/colr.rs.
func (b *colrBuilder) QuadTo(cx0, cy0, x, y float32) {
	b.addPoint(cx0, cy0)
	b.addPoint(x, y)
	b.writeCmd("Q", cx0, cy0, x, y)
}

// CurveTo is OutlinePen::curve_to for Builder in usvg src/text/colr.rs.
func (b *colrBuilder) CurveTo(cx0, cy0, cx1, cy1, x, y float32) {
	b.addPoint(cx0, cy0)
	b.addPoint(cx1, cy1)
	b.addPoint(x, y)
	b.writeCmd("C", cx0, cy0, cx1, cy1, x, y)
}

// Close is OutlinePen::close for Builder in usvg src/text/colr.rs.
func (b *colrBuilder) Close() {
	*b.path = append(*b.path, "Z "...)
}

// writeColorAttribute is XmlWriterExt::write_color_attribute in usvg
// src/text/colr.rs.
func writeColorAttribute(w *xmlwriter.XMLWriter, name string, color svgtypes.Color) {
	w.WriteAttributeFmt(name, "rgb(%d, %d, %d)", color.Red, color.Green, color.Blue)
}

// writeTransformAttribute is XmlWriterExt::write_transform_attribute in usvg
// src/text/colr.rs.
func writeTransformAttribute(w *xmlwriter.XMLWriter, name string, ts colrpaint.Transform) {
	if ts == colrpaint.IdentityTransform() {
		return
	}

	w.WriteAttributeFmt(name, "matrix(%s %s %s %s %s %s)",
		fmtF32(ts.XX), fmtF32(ts.YX), fmtF32(ts.XY), fmtF32(ts.YY), fmtF32(ts.DX), fmtF32(ts.DY))
}

// writeSpreadMethodAttribute is
// XmlWriterExt::write_spread_method_attribute in usvg src/text/colr.rs.
func writeSpreadMethodAttribute(w *xmlwriter.XMLWriter, extend tables.Extend) {
	var v string
	switch extend {
	case tables.ExtendPad:
		v = "pad"
	case tables.ExtendRepeat:
		v = "repeat"
	case tables.ExtendReflect:
		v = "reflect"
	default:
		return
	}
	w.WriteAttribute("spreadMethod", v)
}

// glyphPainter is GlyphPainter in usvg src/text/colr.rs.
//
// NOTE: This is only a best-effort translation of COLR into SVG.
//
// Port: the font and the variation location of upstream are the face of
// go-text, whose coordinates are the location. opt holds the logger (D20).
type glyphPainter struct {
	pf   *parsedFace
	face *font.Face
	// The variation location to draw outlines at is the coordinates of face.
	svg              *xmlwriter.XMLWriter
	pathBuf          *[]byte
	gradientIndex    int
	clipPathIndex    int
	foregroundColor  svgtypes.Color
	transform        colrpaint.Transform
	outlineTransform colrpaint.Transform
	transformsStack  []colrpaint.Transform
	// The bounding box of every active clip, in the root coordinate space.
	// `None` means the clip is empty (or its bounds are unknown).
	clipStack []*Rect
	opt       *options
	// palette is the index of the palette of CPAL.
	//
	// Port: usvg always takes the first palette. FontDatabase.ColorGlyph
	// can take another one (D57).
	palette int
}

// writeGradientStops is GlyphPainter::write_gradient_stops in usvg
// src/text/colr.rs.
func (p *glyphPainter) writeGradientStops(stops []colrpaint.ColorStop) {
	for _, stop := range stops {
		color := p.paletteIndexToColor(stop.PaletteIndex, stop.Alpha)
		p.svg.StartElement("stop")
		p.svg.WriteAttribute("offset", fmtF32(stop.Offset))
		writeColorAttribute(p.svg, "stop-color", color)
		opacity := float32(color.Alpha) / 255.0
		p.svg.WriteAttribute("stop-opacity", fmtF32(opacity))
		p.svg.EndElement()
	}
}

// paintSolid is GlyphPainter::paint_solid in usvg src/text/colr.rs.
func (p *glyphPainter) paintSolid(color svgtypes.Color) {
	p.svg.StartElement("path")
	writeColorAttribute(p.svg, "fill", color)
	opacity := float32(color.Alpha) / 255.0
	p.svg.WriteAttribute("fill-opacity", fmtF32(opacity))
	writeTransformAttribute(p.svg, "transform", p.outlineTransform)
	p.svg.WriteAttribute("d", string(*p.pathBuf))
	p.svg.EndElement()
}

// paintLinearGradient is GlyphPainter::paint_linear_gradient in usvg
// src/text/colr.rs.
func (p *glyphPainter) paintLinearGradient(
	p0 colrpaint.Point,
	p1 colrpaint.Point,
	colorStops []colrpaint.ColorStop,
	extend tables.Extend,
) {
	gradientID := "lg" + strconv.Itoa(p.gradientIndex)
	p.gradientIndex++

	gradientTransform := paintTransform(p.outlineTransform, p.transform, p.opt)

	p.svg.StartElement("linearGradient")
	p.svg.WriteAttribute("id", gradientID)
	p.svg.WriteAttribute("x1", fmtF32(p0.X))
	p.svg.WriteAttribute("y1", fmtF32(p0.Y))
	p.svg.WriteAttribute("x2", fmtF32(p1.X))
	p.svg.WriteAttribute("y2", fmtF32(p1.Y))
	p.svg.WriteAttribute("gradientUnits", "userSpaceOnUse")
	writeSpreadMethodAttribute(p.svg, extend)
	writeTransformAttribute(p.svg, "gradientTransform", gradientTransform)
	p.writeGradientStops(colorStops)
	p.svg.EndElement()

	p.svg.StartElement("path")
	p.svg.WriteAttributeFmt("fill", "url(#%s)", gradientID)
	writeTransformAttribute(p.svg, "transform", p.outlineTransform)
	p.svg.WriteAttribute("d", string(*p.pathBuf))
	p.svg.EndElement()
}

// paintRadialGradient is GlyphPainter::paint_radial_gradient in usvg
// src/text/colr.rs.
func (p *glyphPainter) paintRadialGradient(
	c0 colrpaint.Point,
	r0 float32,
	c1 colrpaint.Point,
	r1 float32,
	colorStops []colrpaint.ColorStop,
	extend tables.Extend,
) {
	gradientID := "rg" + strconv.Itoa(p.gradientIndex)
	p.gradientIndex++

	gradientTransform := paintTransform(p.outlineTransform, p.transform, p.opt)

	// TODO: Normalizing the stops into the 0..1 range moves the circles onto the
	// first and last stop, which can make `r0` (and in theory `r1`) negative.
	// SVG cannot express that, so the color line should be cut where the radius
	// reaches zero, with an interpolated stop inserted at the cut and the
	// remaining stops reparameterized into the 0..1 range.
	p.svg.StartElement("radialGradient")
	p.svg.WriteAttribute("id", gradientID)
	p.svg.WriteAttribute("cx", fmtF32(c1.X))
	p.svg.WriteAttribute("cy", fmtF32(c1.Y))
	p.svg.WriteAttribute("r", fmtF32(r1))
	p.svg.WriteAttribute("fr", fmtF32(r0))
	p.svg.WriteAttribute("fx", fmtF32(c0.X))
	p.svg.WriteAttribute("fy", fmtF32(c0.Y))
	p.svg.WriteAttribute("gradientUnits", "userSpaceOnUse")
	writeSpreadMethodAttribute(p.svg, extend)
	writeTransformAttribute(p.svg, "gradientTransform", gradientTransform)
	p.writeGradientStops(colorStops)
	p.svg.EndElement()

	p.svg.StartElement("path")
	p.svg.WriteAttributeFmt("fill", "url(#%s)", gradientID)
	writeTransformAttribute(p.svg, "transform", p.outlineTransform)
	p.svg.WriteAttribute("d", string(*p.pathBuf))
	p.svg.EndElement()
}

// paintSweepGradient is GlyphPainter::paint_sweep_gradient in usvg
// src/text/colr.rs.
func (p *glyphPainter) paintSweepGradient(
	_ colrpaint.Point,
	_ float32,
	_ float32,
	_ []colrpaint.ColorStop,
	_ tables.Extend,
) {
	// Sweep gradients are not supported.
	// TODO: surface warning without printing to stdout
}

// toTspTransform is to_tsp_transform in usvg src/text/colr.rs.
func toTspTransform(ts colrpaint.Transform) Transform {
	return path.TransformFromRow(ts.XX, ts.YX, ts.XY, ts.YY, ts.DX, ts.DY)
}

// paintTransform is paint_transform in usvg src/text/colr.rs.
func paintTransform(outlineTransform, transform colrpaint.Transform, opt *options) colrpaint.Transform {
	outlineTs := toTspTransform(outlineTransform)
	gradientTransform := toTspTransform(transform)

	inv, ok := outlineTs.Invert()
	if !ok {
		opt.warnf("Failed to calculate transform for gradient in glyph.")
		inv = path.DefaultTransform()
	}
	ts := inv.PreConcat(gradientTransform)

	return colrpaint.Transform{XX: ts.SX, YX: ts.KY, XY: ts.KX, YY: ts.SY, DX: ts.TX, DY: ts.TY}
}

// mapRect is map_rect in usvg src/text/colr.rs. It returns the bounding box
// of `rect` transformed by `ts`.
func mapRect(rect Rect, ts Transform) (Rect, bool) {
	points := []path.Point{
		path.PointFromXY(rect.Left(), rect.Top()),
		path.PointFromXY(rect.Right(), rect.Top()),
		path.PointFromXY(rect.Left(), rect.Bottom()),
		path.PointFromXY(rect.Right(), rect.Bottom()),
	}
	ts.MapPoints(points)
	const f32Max = 3.40282346638528859811704183484516925440e+38
	minX, minY := float32(f32Max), float32(f32Max)
	maxX, maxY := float32(-f32Max), float32(-f32Max)
	for _, p := range points {
		minX = minF32(minX, p.X)
		minY = minF32(minY, p.Y)
		maxX = maxF32(maxX, p.X)
		maxY = maxF32(maxY, p.Y)
	}
	return path.RectFromLTRB(minX, minY, maxX, maxY)
}

// intersectRects is intersect_rects in usvg src/text/colr.rs. It returns the
// intersection of two rects, or `None` when they do not overlap.
func intersectRects(a, b Rect) (Rect, bool) {
	return path.RectFromLTRB(
		maxF32(a.Left(), b.Left()),
		maxF32(a.Top(), b.Top()),
		minF32(a.Right(), b.Right()),
		minF32(a.Bottom(), b.Bottom()),
	)
}

// clipWithPath is GlyphPainter::clip_with_path in usvg src/text/colr.rs.
func (p *glyphPainter) clipWithPath(d string) {
	clipID := "cp" + strconv.Itoa(p.clipPathIndex)
	p.clipPathIndex++

	p.svg.StartElement("clipPath")
	p.svg.WriteAttribute("id", clipID)
	p.svg.StartElement("path")
	writeTransformAttribute(p.svg, "transform", p.outlineTransform)
	p.svg.WriteAttribute("d", d)
	p.svg.EndElement()
	p.svg.EndElement()

	p.svg.StartElement("g")
	p.svg.WriteAttributeFmt("clip-path", "url(#%s)", clipID)
}

// outlineGlyph is GlyphPainter::outline_glyph in usvg src/text/colr.rs. It
// outlines a glyph into `path_buf` at the current variation location
// (an empty path on failure), records the current transform as the
// outline transform and returns the outline's conservative bounding box
// in the glyph's local coordinate space.
func (p *glyphPainter) outlineGlyph(glyphID GlyphID) (Rect, bool) {
	*p.pathBuf = (*p.pathBuf)[:0]

	var bounds Rect
	hasBounds := false
	builder := newColrBuilder(p.pathBuf)
	outlined := drawOutline(p.pf, p.face, glyphID, builder)
	if outlined {
		builder.finish()
		bounds, hasBounds = builder.bounds()
	} else {
		// A partial outline may have been written before a draw error.
		*p.pathBuf = (*p.pathBuf)[:0]
	}

	// We have to write outline using the current transform.
	p.outlineTransform = p.transform

	return bounds, hasBounds
}

// paintBrush is GlyphPainter::paint_brush in usvg src/text/colr.rs. It
// paints `path_buf` (positioned by the outline transform) with the given
// brush.
func (p *glyphPainter) paintBrush(brush colrpaint.Brush) {
	switch brush.Kind {
	case colrpaint.BrushSolid:
		color := p.paletteIndexToColor(brush.PaletteIndex, brush.Alpha)
		p.paintSolid(color)
	case colrpaint.BrushLinearGradient:
		p.paintLinearGradient(brush.P0, brush.P1, brush.ColorStops, brush.Extend)
	case colrpaint.BrushRadialGradient:
		p.paintRadialGradient(brush.C0, brush.R0, brush.C1, brush.R1, brush.ColorStops, brush.Extend)
	case colrpaint.BrushSweepGradient:
		p.paintSweepGradient(brush.C0, brush.StartAngle, brush.EndAngle, brush.ColorStops, brush.Extend)
	}
}

// paletteIndexToColor is GlyphPainter::palette_index_to_color in usvg
// src/text/colr.rs.
func (p *glyphPainter) paletteIndexToColor(paletteIndex uint16, alpha float32) svgtypes.Color {
	lookup := func() (svgtypes.Color, bool) {
		// We always use the first palette. `ColorPalettes` handles
		// per-palette record offsets internally.
		//
		// Port: okage takes the palette of the painter, and the first one
		// when the font does not have it (D57).
		palettes := p.face.CPAL
		if len(palettes) == 0 {
			return svgtypes.Color{}, false
		}
		palette := palettes[0]
		if p.palette > 0 && p.palette < len(palettes) {
			palette = palettes[p.palette]
		}
		if int(paletteIndex) >= len(palette) {
			return svgtypes.Color{}, false
		}
		color := palette[paletteIndex]
		return svgtypes.Color{
			Red:   color.Red,
			Blue:  color.Blue,
			Green: color.Green,
			Alpha: color.Alpha,
		}, true
	}

	var color svgtypes.Color
	if paletteIndex == 0xFFFF {
		color = p.foregroundColor
	} else if c, ok := lookup(); ok {
		color = c
	} else {
		color = p.foregroundColor
	}

	// Multiply alpha
	color.Alpha = wide.TruncU8(float32(color.Alpha) * alpha)

	return color
}

// PushTransform is PaintSink::push_transform for GlyphPainter in usvg
// src/text/colr.rs.
func (p *glyphPainter) PushTransform(transform colrpaint.Transform) {
	p.transformsStack = append(p.transformsStack, p.transform)
	p.transform = p.transform.Mul(transform)
}

// PopTransform is PaintSink::pop_transform for GlyphPainter in usvg
// src/text/colr.rs.
func (p *glyphPainter) PopTransform() {
	if n := len(p.transformsStack); n > 0 {
		p.transform = p.transformsStack[n-1]
		p.transformsStack = p.transformsStack[:n-1]
	}
}

// FillGlyph is PaintSink::fill_glyph for GlyphPainter in usvg
// src/text/colr.rs.
func (p *glyphPainter) FillGlyph(glyphID colrpaint.GlyphID, brushTransform *colrpaint.Transform, brush colrpaint.Brush) {
	// Fill the glyph outline directly instead of the default
	// clip-then-fill decomposition. This avoids a redundant clip path
	// per fill and matches the output of the old ttf-parser based painter.
	p.outlineGlyph(GlyphID(glyphID))

	if brushTransform != nil {
		p.PushTransform(*brushTransform)
		p.paintBrush(brush)
		p.PopTransform()
	} else {
		p.paintBrush(brush)
	}
}

// PushClipGlyph is PaintSink::push_clip_glyph for GlyphPainter in usvg
// src/text/colr.rs.
func (p *glyphPainter) PushClipGlyph(glyphID colrpaint.GlyphID) {
	bounds, hasBounds := p.outlineGlyph(GlyphID(glyphID))

	// Clip with the outline. This must always open a clip group - even
	// when outlining failed (an empty path clips everything away) - since
	// the corresponding `pop_clip` will unconditionally close it.
	d := string(*p.pathBuf)
	p.clipWithPath(d)

	var rootBounds *Rect
	if hasBounds {
		if r, ok := mapRect(bounds, toTspTransform(p.outlineTransform)); ok {
			rootBounds = &r
		}
	}
	p.clipStack = append(p.clipStack, rootBounds)
}

// PushClipBox is PaintSink::push_clip_box for GlyphPainter in usvg
// src/text/colr.rs.
func (p *glyphPainter) PushClipBox(clipBox colrpaint.BoundingBox) {
	xMin := clipBox.XMin
	xMax := clipBox.XMax
	yMin := clipBox.YMin
	yMax := clipBox.YMax

	clipPath := "M " + fmtF32(xMin) + " " + fmtF32(yMin) +
		" L " + fmtF32(xMax) + " " + fmtF32(yMin) +
		" L " + fmtF32(xMax) + " " + fmtF32(yMax) +
		" L " + fmtF32(xMin) + " " + fmtF32(yMax) + " Z"

	// The clip box is positioned by the current transform.
	p.outlineTransform = p.transform
	p.clipWithPath(clipPath)

	var bounds *Rect
	if b, ok := path.RectFromLTRB(xMin, yMin, xMax, yMax); ok {
		if r, ok := mapRect(b, toTspTransform(p.outlineTransform)); ok {
			bounds = &r
		}
	}
	p.clipStack = append(p.clipStack, bounds)
}

// PopClip is PaintSink::pop_clip for GlyphPainter in usvg
// src/text/colr.rs.
func (p *glyphPainter) PopClip() {
	p.svg.EndElement()
	if n := len(p.clipStack); n > 0 {
		p.clipStack = p.clipStack[:n-1]
	}
}

// Fill is PaintSink::fill for GlyphPainter in usvg src/text/colr.rs.
func (p *glyphPainter) Fill(brush colrpaint.Brush) {
	// A fill paints the intersection of all currently active clips.
	// Paint a rectangle covering that intersection and let the enclosing
	// clip groups shape it.
	if len(p.clipStack) == 0 {
		p.opt.warnf("Unclipped COLR fills are not supported.")
		return
	}

	var region Rect
	hasRegion := false
	for _, bounds := range p.clipStack {
		// A clip with no (or unknown) bounds clips everything away.
		if bounds == nil {
			return
		}
		if hasRegion {
			r, ok := intersectRects(region, *bounds)
			if !ok {
				// An empty intersection - there is nothing to paint.
				return
			}
			region = r
		} else {
			region = *bounds
			hasRegion = true
		}
	}
	if !hasRegion {
		return
	}

	*p.pathBuf = (*p.pathBuf)[:0]
	*p.pathBuf = append(*p.pathBuf, "M "+fmtF32(region.Left())+" "+fmtF32(region.Top())+
		" L "+fmtF32(region.Right())+" "+fmtF32(region.Top())+
		" L "+fmtF32(region.Right())+" "+fmtF32(region.Bottom())+
		" L "+fmtF32(region.Left())+" "+fmtF32(region.Bottom())+" Z"...)

	// The covering rectangle is in the root coordinate space.
	p.outlineTransform = colrpaint.IdentityTransform()

	p.paintBrush(brush)
}

// compositeModeBlendModes holds the composite modes that
// PaintSink::push_layer for GlyphPainter in usvg src/text/colr.rs
// matches, with the blend mode whose name upstream writes.
//
// Port: BlendMode.String writes the same names.
var compositeModeBlendModes = map[tables.CompositeMode]BlendMode{
	tables.CompositeSrcOver:       BlendModeNormal,
	tables.CompositeScreen:        BlendModeScreen,
	tables.CompositeOverlay:       BlendModeOverlay,
	tables.CompositeDarken:        BlendModeDarken,
	tables.CompositeLighten:       BlendModeLighten,
	tables.CompositeColorDodge:    BlendModeColorDodge,
	tables.CompositeColorBurn:     BlendModeColorBurn,
	tables.CompositeHardLight:     BlendModeHardLight,
	tables.CompositeSoftLight:     BlendModeSoftLight,
	tables.CompositeDifference:    BlendModeDifference,
	tables.CompositeExclusion:     BlendModeExclusion,
	tables.CompositeMultiply:      BlendModeMultiply,
	tables.CompositeHslHue:        BlendModeHue,
	tables.CompositeHslSaturation: BlendModeSaturation,
	tables.CompositeHslColor:      BlendModeColor,
	tables.CompositeHslLuminosity: BlendModeLuminosity,
}

// PushLayer is PaintSink::push_layer for GlyphPainter in usvg
// src/text/colr.rs.
func (p *glyphPainter) PushLayer(compositeMode tables.CompositeMode) {
	// TODO: Need to figure out how to represent the other blend modes in SVG.
	blendMode, ok := compositeModeBlendModes[compositeMode]
	if !ok {
		// Unsupported blend mode
		// TODO: support other blend modes
		// TODO: surface warning without printing to stdout
		blendMode = BlendModeNormal
	}
	mode := blendMode.String()

	p.svg.StartElement("g")
	p.svg.WriteAttributeFmt("style", "mix-blend-mode: %s; isolation: isolate", mode)
}

// PopLayer is PaintSink::pop_layer for GlyphPainter in usvg
// src/text/colr.rs.
func (p *glyphPainter) PopLayer() {
	p.svg.EndElement()
}
