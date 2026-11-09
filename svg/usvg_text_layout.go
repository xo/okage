// Copyright 2022 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"encoding/binary"
	"fmt"
	"iter"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-text/typesetting/bidi"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/harfbuzz"
	"github.com/go-text/typesetting/language"

	"github.com/xo/okage/internal/kurbo"
	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// This file ports usvg src/text/layout.rs.
//
// Port: go-text takes the place of the crates of text (D9). The font database
// of usvg_text_fontdb.go is fontdb, the package harfbuzz of go-text is
// harfrust, the package bidi of go-text is unicode-bidi, the package language
// of go-text is unicode-script, and the table of usvg_text_orientation.go is
// unicode-vo. The metrics of skrifa are read from the tables of the font,
// which go-text loads, by the rules of skrifa.

// PositionedGlyph is PositionedGlyph in usvg src/text/layout.rs. It is a
// glyph that has already been positioned correctly.
//
// Note that the transform already takes the font size into consideration, so applying the
// transform to the outline of the glyphs is all that is necessary to display it correctly.
type PositionedGlyph struct {
	// Returns the transform of the glyph itself within the cluster. For example,
	// for zalgo text, it contains the transform to position the glyphs above/below
	// the main glyph.
	glyphTs Transform
	// Returns the transform of the whole cluster that the glyph is part of.
	clusterTs Transform
	// Returns the transform of the span that the glyph is a part of.
	spanTs Transform
	// The units per em of the font the glyph belongs to.
	unitsPerEm uint16
	// The font size the glyph should be scaled to.
	fontSize float32
	// ID is the ID of the glyph.
	ID GlyphID
	// Text is the text from the original string that corresponds to that
	// glyph.
	Text string
	// Font is the ID of the font the glyph should be taken from. Can be used
	// with the [font database of the tree](crate::Tree::fontdb) this glyph
	// is part of.
	Font FontID
}

// FontSize is PositionedGlyph::font_size in usvg src/text/layout.rs. It
// returns the font size for this glyph.
func (g *PositionedGlyph) FontSize() float32 {
	return g.fontSize
}

// Transform is PositionedGlyph::transform in usvg src/text/layout.rs. It
// returns the transform of glyph.
func (g *PositionedGlyph) Transform() Transform {
	sx := g.fontSize / float32(g.unitsPerEm)

	return g.spanTs.
		PreConcat(g.clusterTs).
		PreConcat(path.TransformFromScale(sx, sx)).
		PreConcat(g.glyphTs)
}

// OutlineTransform is PositionedGlyph::outline_transform in usvg
// src/text/layout.rs. It returns the transform of glyph, assuming that an
// outline glyph is being used (i.e. from the `glyf` or `CFF/CFF2` table).
func (g *PositionedGlyph) OutlineTransform() Transform {
	// Outlines are mirrored by default.
	return g.Transform().
		PreConcat(path.TransformFromScale(1.0, -1.0))
}

// CbdtTransform is PositionedGlyph::cbdt_transform in usvg
// src/text/layout.rs. It returns the transform for the glyph, assuming that
// a CBTD-based raster glyph is being used.
func (g *PositionedGlyph) CbdtTransform(x, y, pixelsPerEm float32) Transform {
	return g.Transform().
		PreConcat(path.TransformFromScale(
			float32(g.unitsPerEm)/pixelsPerEm,
			float32(g.unitsPerEm)/pixelsPerEm,
		)).
		// Right now, the top-left corner of the image would be placed in
		// on the "text cursor", but we want the bottom-left corner to be there,
		// so we need to shift it up and also apply the x/y offset.
		PreTranslate(x, -y)
}

// SbixTransform is PositionedGlyph::sbix_transform in usvg
// src/text/layout.rs. It returns the transform for the glyph, assuming that
// a sbix-based raster glyph is being used.
func (g *PositionedGlyph) SbixTransform(
	x float32,
	y float32,
	xMin float32,
	yMin float32,
	pixelsPerEm float32,
	height float32,
) Transform {
	// In contrast to CBDT, we also need to look at the outline bbox of the glyph and add a shift if necessary.
	bboxXShift := -xMin

	var bboxYShift float32
	if ApproxZeroUlps32(yMin, 4) {
		// For unknown reasons, using Apple Color Emoji will lead to a vertical shift on MacOS, but this shift
		// doesn't seem to be coming from the font and most likely is somehow hardcoded. On Windows,
		// this shift will not be applied. However, if this shift is not applied the emojis are a bit
		// too high up when being together with other text, so we try to imitate this.
		// See also https://github.com/harfbuzz/harfbuzz/issues/2679#issuecomment-1345595425
		// So whenever the y-shift is 0, we approximate this vertical shift that seems to be produced by it.
		// This value seems to be pretty close to what is happening on MacOS.
		// We can still remove this if it turns out to be a problem, but Apple Color Emoji is pretty
		// much the only `sbix` font out there and they all seem to have a y-shift of 0, so it
		// makes sense to keep it.
		bboxYShift = 0.128 * float32(g.unitsPerEm)
	} else {
		bboxYShift = -yMin
	}

	return g.Transform().
		PreConcat(path.TransformFromTranslate(bboxXShift, bboxYShift)).
		PreConcat(path.TransformFromScale(
			float32(g.unitsPerEm)/pixelsPerEm,
			float32(g.unitsPerEm)/pixelsPerEm,
		)).
		// Right now, the top-left corner of the image would be placed in
		// on the "text cursor", but we want the bottom-left corner to be there,
		// so we need to shift it up and also apply the x/y offset.
		PreTranslate(x, -height-y)
}

// SvgTransform is PositionedGlyph::svg_transform in usvg src/text/layout.rs.
// It returns the transform for the glyph, assuming that an SVG glyph is
// being used.
func (g *PositionedGlyph) SvgTransform() Transform {
	return g.Transform()
}

// ColrTransform is PositionedGlyph::colr_transform in usvg
// src/text/layout.rs. It returns the transform for the glyph, assuming that
// a COLR glyph is being used.
func (g *PositionedGlyph) ColrTransform() Transform {
	return g.OutlineTransform()
}

// Span is Span in usvg src/text/layout.rs. A span contains a number of
// layouted glyphs that share the same fill, stroke, paint order and
// visibility.
//
// Port: a nil Fill, Stroke, Underline, Overline or LineThrough is None.
type Span struct {
	// Fill is the fill of the span.
	Fill *Fill
	// Stroke is the stroke of the span.
	Stroke *Stroke
	// PaintOrder is the paint order of the span.
	PaintOrder PaintOrder
	// FontSize is the font size of the span.
	FontSize NonZeroPositiveF32
	// Variations holds the font variation settings for variable fonts.
	Variations []FontVariation
	// FontOpticalSizing is the font optical sizing mode.
	FontOpticalSizing FontOpticalSizing
	// Visible is the visibility of the span.
	Visible bool
	// PositionedGlyphs holds the glyphs that make up the span.
	PositionedGlyphs []PositionedGlyph
	// Underline is an underline text decoration of the span.
	// Needs to be rendered before all glyphs.
	Underline *Path
	// Overline is an overline text decoration of the span.
	// Needs to be rendered before all glyphs.
	Overline *Path
	// LineThrough is a line-through text decoration of the span.
	// Needs to be rendered after all glyphs.
	LineThrough *Path
}

// clone is the derived Clone for Span in usvg src/text/layout.rs. It copies
// the fill, the stroke and the paths of the decorations, which the
// conversion of the paint servers changes in place.
func (s *Span) clone() Span {
	c := *s
	c.Fill = cloneFill(s.Fill)
	c.Stroke = cloneStroke(s.Stroke)
	c.Underline = clonePath(s.Underline)
	c.Overline = clonePath(s.Overline)
	c.LineThrough = clonePath(s.LineThrough)
	return c
}

// cloneFill is the Clone of an Option<Fill> of upstream. A nil fill is None.
func cloneFill(f *Fill) *Fill {
	if f == nil {
		return nil
	}
	c := *f
	return &c
}

// cloneStroke is the Clone of an Option<Stroke> of upstream. A nil stroke is
// None.
func cloneStroke(s *Stroke) *Stroke {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}

// clonePath is the Clone of an Option<Path> of upstream, with a copy of its
// fill and stroke. A nil path is None.
func clonePath(p *Path) *Path {
	if p == nil {
		return nil
	}
	c := *p
	c.fill = cloneFill(p.fill)
	c.stroke = cloneStroke(p.stroke)
	return &c
}

// glyphCluster is GlyphCluster in usvg src/text/layout.rs.
type glyphCluster struct {
	byteIdx           byteIndex
	codepoint         rune
	width             float32
	advance           float32
	textLengthSpacing float32
	ascent            float32
	descent           float32
	hasRelativeShift  bool
	glyphs            []PositionedGlyph
	transform         Transform
	pathTransform     Transform
	visible           bool
}

// height is GlyphCluster::height in usvg src/text/layout.rs.
func (c *glyphCluster) height() float32 {
	return c.ascent - c.descent
}

// fullTransform is GlyphCluster::transform in usvg src/text/layout.rs. It
// takes another name, because the field transform has that name.
func (c *glyphCluster) fullTransform() Transform {
	return c.pathTransform.PostConcat(c.transform)
}

// clone is the derived Clone for GlyphCluster in usvg src/text/layout.rs.
func (c *glyphCluster) clone() glyphCluster {
	n := *c
	n.glyphs = slices.Clone(c.glyphs)
	return n
}

// layoutText is layout_text in usvg src/text/layout.rs.
//
// Port: opt holds the logger (D20).
func layoutText(
	textNode *Text,
	resolver *FontResolver,
	fontdb *FontDatabase,
	opt *options,
) ([]Span, NonZeroRect, bool) {
	fontsCache := fontsCache{}

	for i := range textNode.chunks {
		chunk := &textNode.chunks[i]
		for j := range chunk.spans {
			span := &chunk.spans[j]
			key := span.font.key()
			if _, ok := fontsCache[key]; !ok {
				if id, ok := resolver.selectFont(&span.font, fontdb, opt); ok {
					if font, ok := databaseLoadFont(fontdb, id, span.font.variations); ok {
						fontsCache[key] = font
					}
				}
			}
		}
	}

	var spans []Span
	charOffset := 0
	var lastX float32
	var lastY float32
	bbox := defaultBBox()
	for i := range textNode.chunks {
		chunk := &textNode.chunks[i]
		var x, y float32
		switch chunk.textFlow.Kind {
		case TextFlowLinear:
			x, y = lastX, lastY
			if chunk.x != nil {
				x = *chunk.x
			}
			if chunk.y != nil {
				y = *chunk.y
			}
		case TextFlowPath:
			x, y = 0.0, 0.0
		}

		clusters := processChunk(chunk, fontsCache, resolver, fontdb, opt)
		if len(clusters) == 0 {
			charOffset += utf8.RuneCountInString(chunk.text)
			continue
		}

		applyWritingMode(textNode.writingMode, clusters)
		applyLetterSpacing(chunk, &clusters)
		applyWordSpacing(chunk, clusters)

		applyLengthAdjust(chunk, clusters)
		currX, currY := resolveClustersPositions(
			textNode,
			chunk,
			charOffset,
			textNode.writingMode,
			fontsCache,
			clusters,
		)

		textTs := path.DefaultTransform()
		if textNode.writingMode == WritingModeTopToBottom {
			if chunk.textFlow.Kind == TextFlowLinear {
				textTs = textTs.PreRotateAt(90.0, x, y)
			}
		}

		for j := range chunk.spans {
			span := &chunk.spans[j]
			font, ok := fontsCache[span.font.key()]
			if !ok {
				continue
			}

			decorationSpans := collectDecorationSpans(span, clusters)

			spanTs := textTs
			spanTs = spanTs.PreTranslate(x, y)
			if chunk.textFlow.Kind == TextFlowLinear {
				shift := resolveBaseline(span, font, textNode.writingMode)

				// In case of a horizontal flow, shift transform and not clusters,
				// because clusters can be rotated and an additional shift will lead
				// to invalid results.
				spanTs = spanTs.PreTranslate(0.0, shift)
			}

			var underline *Path
			var overline *Path
			var lineThrough *Path

			if decoration := span.decoration.underline; decoration != nil {
				// TODO: No idea what offset should be used for top-to-bottom layout.
				// There is
				// https://www.w3.org/TR/css-text-decor-3/#text-underline-position-property
				// but it doesn't go into details.
				var offset float32
				switch textNode.writingMode {
				case WritingModeLeftToRight:
					offset = -font.underlinePositionAt(span.fontSize.Get())
				case WritingModeTopToBottom:
					offset = font.height(span.fontSize.Get()) / 2.0
				}

				if p, ok := convertDecoration(offset, span, font, *decoration, decorationSpans, spanTs, opt); ok {
					bbox = bbox.expand(bboxFromRect(p.data.Bounds()))
					underline = p
				}
			}

			if decoration := span.decoration.overline; decoration != nil {
				var offset float32
				switch textNode.writingMode {
				case WritingModeLeftToRight:
					offset = -font.ascentAt(span.fontSize.Get())
				case WritingModeTopToBottom:
					offset = -font.height(span.fontSize.Get()) / 2.0
				}

				if p, ok := convertDecoration(offset, span, font, *decoration, decorationSpans, spanTs, opt); ok {
					bbox = bbox.expand(bboxFromRect(p.data.Bounds()))
					overline = p
				}
			}

			if decoration := span.decoration.lineThrough; decoration != nil {
				var offset float32
				switch textNode.writingMode {
				case WritingModeLeftToRight:
					offset = -font.lineThroughPositionAt(span.fontSize.Get())
				case WritingModeTopToBottom:
					offset = 0.0
				}

				if p, ok := convertDecoration(offset, span, font, *decoration, decorationSpans, spanTs, opt); ok {
					bbox = bbox.expand(bboxFromRect(p.data.Bounds()))
					lineThrough = p
				}
			}

			fill := cloneFill(span.fill)
			if fill != nil {
				// The `fill-rule` should be ignored.
				// https://www.w3.org/TR/SVG2/text.html#TextRenderingOrder
				//
				// 'Since the fill-rule property does not apply to SVG text elements,
				// the specific order of the subpaths within the equivalent path does not matter.'
				fill.rule = FillRuleNonZero
			}

			if spanFragments, spanBBox, ok := convertSpan(span, clusters, spanTs); ok {
				bbox = bbox.expand(bboxFromNonZeroRect(spanBBox))

				var positionedGlyphs []PositionedGlyph
				for k := range spanFragments {
					gc := &spanFragments[k]
					clusterTs := gc.fullTransform()
					for l := range gc.glyphs {
						gc.glyphs[l].clusterTs = clusterTs
						gc.glyphs[l].spanTs = spanTs
					}
					positionedGlyphs = append(positionedGlyphs, gc.glyphs...)
				}

				spans = append(spans, Span{
					Fill:              fill,
					Stroke:            cloneStroke(span.stroke),
					PaintOrder:        span.paintOrder,
					FontSize:          span.fontSize,
					Variations:        slices.Clone(span.font.variations),
					FontOpticalSizing: span.fontOpticalSizing,
					Visible:           span.visible,
					PositionedGlyphs:  positionedGlyphs,
					Underline:         underline,
					Overline:          overline,
					LineThrough:       lineThrough,
				})
			}
		}

		charOffset += utf8.RuneCountInString(chunk.text)

		if textNode.writingMode == WritingModeTopToBottom {
			if chunk.textFlow.Kind == TextFlowLinear {
				currX, currY = currY, currX
			}
		}

		lastX = x + currX
		lastY = y + currY
	}

	r, ok := bbox.toNonZeroRect()
	if !ok {
		return nil, NonZeroRect{}, false
	}

	return spans, r, true
}

// convertSpan is convert_span in usvg src/text/layout.rs.
func convertSpan(
	span *TextSpan,
	clusters []glyphCluster,
	textTs Transform,
) ([]glyphCluster, NonZeroRect, bool) {
	var spanClusters []glyphCluster
	bboxesBuilder := path.NewPathBuilder()

	for i := range clusters {
		cluster := &clusters[i]
		if !cluster.visible {
			continue
		}

		if spanContains(span, cluster.byteIdx) {
			spanClusters = append(spanClusters, cluster.clone())
		}

		advance := cluster.advance
		if advance <= 0.0 {
			advance = 1.0
		}

		// We have to calculate text bbox using font metrics and not glyph shape.
		if r, ok := path.NonZeroRectFromXYWH(0.0, -cluster.ascent, advance, cluster.height()); ok {
			if r, ok := r.Transform(cluster.fullTransform()); ok {
				bboxesBuilder.PushRect(r.ToRect())
			}
		}
	}

	bboxes, ok := bboxesBuilder.Finish()
	if !ok {
		return nil, NonZeroRect{}, false
	}
	bboxes, ok = bboxes.Transform(textTs)
	if !ok {
		return nil, NonZeroRect{}, false
	}
	tight, ok := bboxes.ComputeTightBounds()
	if !ok {
		return nil, NonZeroRect{}, false
	}
	bbox, ok := tight.ToNonZeroRect()
	if !ok {
		return nil, NonZeroRect{}, false
	}

	return spanClusters, bbox, true
}

// collectDecorationSpans is collect_decoration_spans in usvg
// src/text/layout.rs.
func collectDecorationSpans(span *TextSpan, clusters []glyphCluster) []decorationSpan {
	var spans []decorationSpan

	started := false
	var width float32
	transform := path.DefaultTransform()

	for i := range clusters {
		cluster := &clusters[i]
		if spanContains(span, cluster.byteIdx) {
			if started && cluster.hasRelativeShift {
				started = false
				spans = append(spans, decorationSpan{width, transform})
			}

			if !started {
				width = cluster.advance
				started = true
				transform = cluster.transform
			} else {
				width += cluster.advance
			}
		} else if started {
			spans = append(spans, decorationSpan{width, transform})
			started = false
		}
	}

	if started {
		spans = append(spans, decorationSpan{width, transform})
	}

	return spans
}

// convertDecoration is convert_decoration in usvg src/text/layout.rs.
//
// Port: opt holds the logger (D20).
func convertDecoration(
	dy float32,
	span *TextSpan,
	font *resolvedFont,
	decoration TextDecorationStyle,
	decorationSpans []decorationSpan,
	transform Transform,
	opt *options,
) (*Path, bool) {
	thickness := font.underlineThicknessAt(span.fontSize.Get())

	builder := path.NewPathBuilder()
	for _, decSpan := range decorationSpans {
		rect, ok := path.NonZeroRectFromXYWH(0.0, -thickness/2.0, decSpan.width, thickness)
		if !ok {
			opt.warnf("a decoration span has a malformed bbox")
			continue
		}

		ts := decSpan.transform.PreTranslate(0.0, dy)

		p := path.PathFromRect(rect.ToRect())
		p, ok = p.Transform(ts)
		if !ok {
			continue
		}

		builder.PushPath(p)
	}

	pathData, ok := builder.Finish()
	if !ok {
		return nil, false
	}
	pathData, ok = pathData.Transform(transform)
	if !ok {
		return nil, false
	}

	return newPath(
		"",
		span.visible,
		cloneFill(decoration.fill),
		cloneStroke(decoration.stroke),
		PaintOrderFillAndStroke,
		DefaultShapeRendering(),
		pathData,
		path.DefaultTransform(),
	)
}

// decorationSpan is DecorationSpan in usvg src/text/layout.rs. It is a text
// decoration span.
//
// Basically a horizontal line, that will be used for underline, overline and line-through.
// It doesn't have a height, since it depends on the Font metrics.
type decorationSpan struct {
	width     float32
	transform Transform
}

// resolveClustersPositions is resolve_clusters_positions in usvg
// src/text/layout.rs. It resolves clusters positions.
//
// Mainly sets the `transform` property.
//
// Returns the last text position. The next text chunk should start from that position.
func resolveClustersPositions(
	text *Text,
	chunk *TextChunk,
	charOffset int,
	writingMode WritingMode,
	fontsCache fontsCache,
	clusters []glyphCluster,
) (float32, float32) {
	switch chunk.textFlow.Kind {
	case TextFlowPath:
		return resolveClustersPositionsPath(
			text,
			chunk,
			charOffset,
			chunk.textFlow.Path,
			writingMode,
			fontsCache,
			clusters,
		)
	default:
		return resolveClustersPositionsHorizontal(text, chunk, charOffset, writingMode, clusters)
	}
}

// clustersLength is clusters_length in usvg src/text/layout.rs.
func clustersLength(clusters []glyphCluster) float32 {
	var w float32
	for i := range clusters {
		w += clusters[i].advance
	}
	return w
}

// shiftXY is a pair (f32, f32) of a shift in x and y.
type shiftXY struct {
	x float32
	y float32
}

// resolveRelativeShifts is resolve_relative_shifts in usvg
// src/text/layout.rs.
func resolveRelativeShifts(
	text *Text,
	chunk *TextChunk,
	offset int,
	clusters []glyphCluster,
) ([]shiftXY, []int) {
	// `dx` and `dy` shift a character and all the characters after it in logical order,
	// while the clusters are in visual order, which differs in right-to-left text.
	logicalOrder := make([]int, len(clusters))
	for i := range logicalOrder {
		logicalOrder[i] = i
	}
	slices.SortStableFunc(logicalOrder, func(a, b int) int {
		return clusters[a].byteIdx.value() - clusters[b].byteIdx.value()
	})

	shifts := make([]shiftXY, len(clusters))
	var shiftX, shiftY float32
	for _, i := range logicalOrder {
		cp := offset + clusters[i].byteIdx.codePointAt(chunk.text)
		shiftX += f32At(text.dx, cp)
		shiftY += f32At(text.dy, cp)
		shifts[i] = shiftXY{shiftX, shiftY}
	}

	return shifts, logicalOrder
}

// f32At is list.get(i).cloned().unwrap_or(0.0) of upstream.
func f32At(list []float32, i int) float32 {
	if i < len(list) {
		return list[i]
	}
	return 0
}

// resolveClustersPositionsHorizontal is
// resolve_clusters_positions_horizontal in usvg src/text/layout.rs.
func resolveClustersPositionsHorizontal(
	text *Text,
	chunk *TextChunk,
	offset int,
	writingMode WritingMode,
	clusters []glyphCluster,
) (float32, float32) {
	shifts, logicalOrder := resolveRelativeShifts(text, chunk, offset, clusters)
	first := logicalOrder[0]
	last := logicalOrder[len(logicalOrder)-1]

	// Vertical text is laid out horizontally and rotated by 90 degrees afterwards.
	if writingMode == WritingModeTopToBottom {
		for i := range shifts {
			shifts[i] = shiftXY{shifts[i].y, -shifts[i].x}
		}
	}

	for i := 1; i < len(clusters); i++ {
		if shifts[i] != shifts[i-1] {
			clusters[i].hasRelativeShift = true
		}
	}

	left := float32(math.Inf(1))
	right := float32(math.Inf(-1))
	var advance float32
	for i := range clusters {
		cluster := &clusters[i]
		start := advance + shifts[i].x
		end := start + cluster.advance - cluster.textLengthSpacing
		left = minF32(minF32(left, start), end)
		right = maxF32(maxF32(right, start), end)
		advance += cluster.advance
	}

	x := shifts[first].x - left + processAnchor(chunk.anchor, right-left)

	for i := range clusters {
		cluster := &clusters[i]
		cp := offset + cluster.byteIdx.codePointAt(chunk.text)

		cluster.transform = cluster.transform.PreTranslate(x+shifts[i].x, shifts[i].y)

		if cp < len(text.rotate) {
			angle := text.rotate[cp]
			if !ApproxZeroUlps32(angle, 4) {
				cluster.transform = cluster.transform.PreRotate(angle)
				cluster.hasRelativeShift = true
			}
		}

		x += cluster.advance
	}

	return x + shifts[last].x, shifts[last].y
}

// resolveBaseline is resolve_baseline in usvg src/text/layout.rs.
//
// Baseline resolving in SVG is a mess.
// Not only it's poorly documented, but as soon as you start mixing
// `dominant-baseline` and `alignment-baseline` each application/browser will produce
// different results.
//
// For now, resvg simply tries to match Chrome's output and not the mythical SVG spec output.
//
// See `alignment_baseline_shift` method comment for more details.
func resolveBaseline(span *TextSpan, font *resolvedFont, writingMode WritingMode) float32 {
	shift := -resolveBaselineShift(span.baselineShift, font, span.fontSize.Get())

	// TODO: support vertical layout as well
	if writingMode == WritingModeLeftToRight {
		if span.alignmentBaseline == AlignmentBaselineAuto ||
			span.alignmentBaseline == AlignmentBaselineBaseline {
			shift += font.dominantBaselineShift(span.dominantBaseline, span.fontSize.Get())
		} else {
			shift += font.alignmentBaselineShift(span.alignmentBaseline, span.fontSize.Get())
		}
	}

	return shift
}

// resolveBaselineShift is resolve_baseline_shift in usvg
// src/text/layout.rs.
func resolveBaselineShift(baselines []BaselineShift, font *resolvedFont, fontSize float32) float32 {
	var shift float32
	for _, baseline := range slices.Backward(baselines) {
		switch baseline.Kind {
		case BaselineShiftBaseline:
		case BaselineShiftSubscript:
			shift -= font.subscriptOffsetAt(fontSize)
		case BaselineShiftSuperscript:
			shift += font.superscriptOffsetAt(fontSize)
		case BaselineShiftNumber:
			shift += baseline.Number
		}
	}

	return shift
}

// resolveClustersPositionsPath is resolve_clusters_positions_path in usvg
// src/text/layout.rs.
func resolveClustersPositionsPath(
	text *Text,
	chunk *TextChunk,
	charOffset int,
	textPath *TextPath,
	writingMode WritingMode,
	fontsCache fontsCache,
	clusters []glyphCluster,
) (float32, float32) {
	var lastX float32
	var lastY float32

	shifts, _ := resolveRelativeShifts(text, chunk, charOffset, clusters)

	// In the text path mode, chunk's x/y coordinates provide an additional offset along the path.
	// The X coordinate is used in a horizontal mode, and Y in vertical.
	var chunkOffset float32
	switch writingMode {
	case WritingModeLeftToRight:
		if chunk.x != nil {
			chunkOffset = *chunk.x
		}
	case WritingModeTopToBottom:
		if chunk.y != nil {
			chunkOffset = *chunk.y
		}
	}

	startOffset := chunkOffset + textPath.startOffset + processAnchor(chunk.anchor, clustersLength(clusters))

	normals := collectNormals(text, clusters, shifts, textPath.path, startOffset)
	for i := range clusters {
		cluster := &clusters[i]
		normal := normals[i]
		dy := shifts[i].y
		if normal == nil {
			// Hide clusters that are outside the text path.
			cluster.visible = false
			continue
		}
		x, y, angle := normal.x, normal.y, normal.angle

		// We have to break a decoration line for each cluster during text-on-path.
		cluster.hasRelativeShift = true

		origTs := cluster.transform

		// Clusters should be rotated by the x-midpoint x baseline position.
		halfWidth := cluster.width / 2.0
		cluster.transform = path.DefaultTransform()
		cluster.transform = cluster.transform.PreTranslate(x-halfWidth, y)
		cluster.transform = cluster.transform.PreRotateAt(angle, halfWidth, 0.0)

		cp := charOffset + cluster.byteIdx.codePointAt(chunk.text)

		var baselineShift float32
		if span, ok := chunkSpanAt(chunk, cluster.byteIdx); ok {
			if font, ok := fontsCache[span.font.key()]; ok {
				baselineShift = -resolveBaseline(span, font, writingMode)
			}
		}

		// Shift only by `dy` since we already applied `dx`
		// during offset along the path calculation.
		if !ApproxZeroUlps32(dy, 4) || !ApproxZeroUlps32(baselineShift, 4) {
			shift := kurbo.NewVec2(0.0, float64(dy-baselineShift))
			cluster.transform = cluster.transform.
				PreTranslate(float32(shift.X), float32(shift.Y))
		}

		if cp < len(text.rotate) {
			angle := text.rotate[cp]
			if !ApproxZeroUlps32(angle, 4) {
				cluster.transform = cluster.transform.PreRotate(angle)
			}
		}

		// The possible `lengthAdjust` transform should be applied after text-on-path positioning.
		cluster.transform = cluster.transform.PreConcat(origTs)

		lastX = x + cluster.advance
		lastY = y
	}

	return lastX, lastY
}

// processAnchor is process_anchor in usvg src/text/layout.rs.
func processAnchor(a TextAnchor, textWidth float32) float32 {
	switch a {
	case TextAnchorMiddle:
		return -textWidth / 2.0
	case TextAnchorEnd:
		return -textWidth
	}
	return 0.0 // Nothing.
}

// pathNormal is PathNormal in usvg src/text/layout.rs.
type pathNormal struct {
	x     float32
	y     float32
	angle float32
}

// createCurveFromLine is the inner function create_curve_from_line of
// collect_normals in usvg src/text/layout.rs.
func createCurveFromLine(px, py, x, y float32) kurbo.CubicBez {
	line := kurbo.NewLine(
		kurbo.NewPoint(float64(px), float64(py)),
		kurbo.NewPoint(float64(x), float64(y)),
	)
	p1 := line.Eval(0.33)
	p2 := line.Eval(0.66)
	return kurbo.CubicBez{
		P0: line.P0,
		P1: p1,
		P2: p2,
		P3: line.P1,
	}
}

// segmentCurve returns the curve of a segment of a path, as the match in
// collect_normals in usvg src/text/layout.rs and path_length in usvg
// src/parser/text.rs makes it. It updates the start of the subpath and the
// last point, and returns false for a MoveTo.
func segmentCurve(seg path.PathSegment, prevMX, prevMY, prevX, prevY *float32) (kurbo.CubicBez, bool) {
	switch seg.Kind {
	case path.PathSegmentMoveTo:
		p := seg.Points[0]
		*prevMX = p.X
		*prevMY = p.Y
		*prevX = p.X
		*prevY = p.Y
		return kurbo.CubicBez{}, false
	case path.PathSegmentLineTo:
		p := seg.Points[0]
		return createCurveFromLine(*prevX, *prevY, p.X, p.Y), true
	case path.PathSegmentQuadTo:
		p1, p := seg.Points[0], seg.Points[1]
		return kurbo.QuadBez{
			P0: kurbo.NewPoint(float64(*prevX), float64(*prevY)),
			P1: kurbo.NewPoint(float64(p1.X), float64(p1.Y)),
			P2: kurbo.NewPoint(float64(p.X), float64(p.Y)),
		}.Raise(), true
	case path.PathSegmentCubicTo:
		p1, p2, p := seg.Points[0], seg.Points[1], seg.Points[2]
		return kurbo.CubicBez{
			P0: kurbo.NewPoint(float64(*prevX), float64(*prevY)),
			P1: kurbo.NewPoint(float64(p1.X), float64(p1.Y)),
			P2: kurbo.NewPoint(float64(p2.X), float64(p2.Y)),
			P3: kurbo.NewPoint(float64(p.X), float64(p.Y)),
		}, true
	default:
		return createCurveFromLine(*prevX, *prevY, *prevMX, *prevMY), true
	}
}

// collectNormals is collect_normals in usvg src/text/layout.rs.
func collectNormals(
	text *Text,
	clusters []glyphCluster,
	shifts []shiftXY,
	p *path.Path,
	offset float32,
) []*pathNormal {
	offsets := make([]float64, 0, len(clusters))
	normals := make([]*pathNormal, 0, len(clusters))
	{
		advance := offset
		for i := range clusters {
			cluster := &clusters[i]
			// Clusters should be rotated by the x-midpoint x baseline position.
			halfWidth := cluster.width / 2.0

			offset := advance + shifts[i].x + halfWidth

			// Clusters outside the path have no normals.
			if offset < 0.0 {
				normals = append(normals, nil)
			}

			offsets = append(offsets, float64(offset))
			advance += cluster.advance
		}
	}

	prevMX := p.Points()[0].X
	prevMY := p.Points()[0].Y
	prevX := prevMX
	prevY := prevMY

	var length float64
	for seg := range p.Segments().All() {
		curve, ok := segmentCurve(seg, &prevMX, &prevMY, &prevX, &prevY)
		if !ok {
			continue
		}

		var arclenAccuracy float32
		{
			baseArclenAccuracy := float32(0.5)
			// Accuracy depends on a current scale.
			// When we have a tiny path scaled by a large value,
			// we have to increase out accuracy accordingly.
			sx, sy := text.absTransform.GetScale()
			// 1.0 acts as a threshold to prevent division by 0 and/or low accuracy.
			arclenAccuracy = baseArclenAccuracy / maxF32(float32(math.Sqrt(float64(sx*sy))), 1.0)
		}

		curveLen := curve.Arclen(float64(arclenAccuracy))

		for _, offset := range offsets[min(len(normals), len(offsets)):] {
			if offset >= length && offset <= length+curveLen {
				offset := curve.InvArclen(offset-length, float64(arclenAccuracy))
				// some rounding error may occur, so we give offset a little tolerance
				offset = max(0.0, min(offset, 1.0))

				pos := curve.Eval(offset)
				d := curve.Deriv().Eval(offset)
				dv := kurbo.NewVec2(-d.Y, d.X) // tangent
				angle := float64(dv.Atan2()*(180.0/math.Pi)) - 90.0

				normals = append(normals, &pathNormal{
					x:     float32(pos.X),
					y:     float32(pos.Y),
					angle: float32(angle),
				})

				if len(normals) == len(offsets) {
					break
				}
			}
		}

		length += curveLen
		prevX = float32(curve.P3.X)
		prevY = float32(curve.P3.Y)
	}

	// If path ended and we still have unresolved normals - set them to `None`.
	for len(normals) < len(offsets) {
		normals = append(normals, nil)
	}

	return normals
}

// processChunk is process_chunk in usvg src/text/layout.rs. It converts a
// text chunk into a list of outlined clusters.
//
// This function will do the BIDI reordering, text shaping and glyphs outlining,
// but not the text layouting. So all clusters are in the 0x0 position.
//
//nolint:dupword // the comments of upstream
func processChunk(
	chunk *TextChunk,
	fontsCache fontsCache,
	resolver *FontResolver,
	fontdb *FontDatabase,
	opt *options,
) []glyphCluster {
	// The way this function works is a bit tricky.
	//
	// The first problem is BIDI reordering.
	// We cannot shape text span-by-span, because glyph clusters are not guarantee to be continuous.
	//
	// For example:
	// <text>Hel<tspan fill="url(#lg1)">lo של</tspan>ום.</text>
	//
	// Would be shaped as:
	// H e l l o   ש ל  ו  ם .   (characters)
	// 0 1 2 3 4 5 12 10 8 6 14  (cluster indices in UTF-8)
	//       ---         ---     (green span)
	//
	// As you can see, our continuous `lo של` span was split into two separated one.
	// So our 3 spans: black - green - black, become 5 spans: black - green - black - green - black.
	// If we shape `Hel`, then `lo של` an then `ום` separately - we would get an incorrect output.
	// To properly handle this we simply shape the whole chunk.
	//
	// But this introduces another issue - what to do when we have multiple fonts?
	// The easy solution would be to simply shape text with each font,
	// where the first font output is used as a base one and all others overwrite it.
	// This way in case of:
	// <text font-family="Arial">Hello <tspan font-family="Helvetica">world</tspan></text>
	// we would replace Arial glyphs for `world` with Helvetica one. Pretty simple.
	//
	// Well, it would work most of the time, but not always.
	// This is because different fonts can produce different amount of glyphs for the same text.
	// The most common example are ligatures. Some fonts can shape `fi` as two glyphs `f` and `i`,
	// but some can use `ﬁ` (U+FB01) instead.
	// Meaning that during merging we have to overwrite not individual glyphs, but clusters.

	// Glyph splitting assigns distinct glyphs to the same index in the original text, we need to
	// store previously used indices to make sure we do not re-use the same index while overwriting
	// span glyphs.
	positions := map[int]struct{}{}

	var glyphs []glyph
	for i := range chunk.spans {
		span := &chunk.spans[i]
		font, ok := fontsCache[span.font.key()]
		if !ok {
			continue
		}

		tmpGlyphs := shapeText(
			chunk.text,
			font,
			span.smallCaps,
			span.applyKerning,
			span.font.variations,
			span.fontSize.Get(),
			span.fontOpticalSizing,
			resolver,
			fontdb,
			opt,
		)

		// Do nothing with the first run.
		if len(glyphs) == 0 {
			glyphs = tmpGlyphs
			continue
		}

		clear(positions)

		// Overwrite span's glyphs.
		for next := 0; next < len(tmpGlyphs); {
			newGlyph := tmpGlyphs[next]
			next++
			if !spanContains(span, newGlyph.byteIdx) {
				continue
			}

			idx := slices.IndexFunc(glyphs, func(g glyph) bool { return g.byteIdx == newGlyph.byteIdx })
			if idx < 0 {
				continue
			}
			if _, used := positions[idx]; used {
				continue
			}

			positions[idx] = struct{}{}

			prevClusterLen := glyphs[idx].clusterLen
			if prevClusterLen < newGlyph.clusterLen {
				// If the new font represents the same cluster with fewer glyphs
				// then remove remaining glyphs.
				for range newGlyph.clusterLen - 1 {
					// Port: Vec::remove panics on an index out of bounds.
					if idx+1 >= len(glyphs) {
						panic(fmt.Sprintf("removal index (is %d) should be < len (is %d)", idx+1, len(glyphs)))
					}
					glyphs = slices.Delete(glyphs, idx+1, idx+2)
				}
			} else if prevClusterLen > newGlyph.clusterLen {
				// If the new font represents the same cluster with more glyphs
				// then insert them after the current one.
				for j := 1; j < prevClusterLen; j++ {
					if next < len(tmpGlyphs) {
						g := tmpGlyphs[next]
						next++
						glyphs = slices.Insert(glyphs, idx+j, g)
					}
				}
			}

			glyphs[idx] = newGlyph
		}
	}

	// Convert glyphs to clusters.
	var clusters []glyphCluster
	for rng, byteIdx := range glyphClusters(glyphs) {
		if span, ok := chunkSpanAt(chunk, byteIdx); ok {
			clusters = append(clusters, formGlyphClusters(
				glyphs[rng[0]:rng[1]],
				chunk.text,
				span.fontSize.Get(),
			))
		}
	}

	return clusters
}

// applyLengthAdjust is apply_length_adjust in usvg src/text/layout.rs.
func applyLengthAdjust(chunk *TextChunk, clusters []glyphCluster) {
	isHorizontal := chunk.textFlow.Kind == TextFlowLinear

	for i := range chunk.spans {
		span := &chunk.spans[i]
		if span.textLength == nil {
			continue
		}
		targetWidth := *span.textLength

		var width float32
		var clusterIndexes []int
		for i := span.start; i < span.end; i++ {
			if index := slices.IndexFunc(clusters, func(c glyphCluster) bool { return c.byteIdx.value() == i }); index >= 0 {
				clusterIndexes = append(clusterIndexes, index)
			}
		}
		// Complex scripts can have multi-codepoint clusters therefore we have to remove duplicates.
		slices.Sort(clusterIndexes)
		clusterIndexes = slices.Compact(clusterIndexes)

		for _, i := range clusterIndexes {
			// Use the original cluster `width` and not `advance`.
			// This method essentially discards any `word-spacing` and `letter-spacing`.
			width += clusters[i].width
		}

		if len(clusterIndexes) == 0 {
			continue
		}

		if span.lengthAdjust == LengthAdjustSpacing {
			var factor float32
			if len(clusterIndexes) > 1 {
				factor = (targetWidth - width) / float32(len(clusterIndexes)-1)
			}

			for _, i := range clusterIndexes {
				clusters[i].advance = clusters[i].width + factor
				clusters[i].textLengthSpacing = factor
			}
		} else {
			factor := targetWidth / width
			// Prevent multiplying by zero.
			if factor < 0.001 {
				continue
			}

			for _, i := range clusterIndexes {
				clusters[i].transform = clusters[i].transform.PreScale(factor, 1.0)

				// Technically just a hack to support the current text-on-path algorithm.
				if !isHorizontal {
					clusters[i].advance *= factor
					clusters[i].width *= factor
				}
			}
		}
	}
}

// applyWritingMode is apply_writing_mode in usvg src/text/layout.rs. It
// rotates clusters according to
// [Unicode Vertical_Orientation Property](https://www.unicode.org/reports/tr50/tr50-19.html).
func applyWritingMode(writingMode WritingMode, clusters []glyphCluster) {
	if writingMode != WritingModeTopToBottom {
		return
	}

	for i := range clusters {
		cluster := &clusters[i]
		if charOrientationUpright(cluster.codepoint) {
			ts := path.DefaultTransform()
			// Position glyph in the center of vertical axis.
			ts = ts.PreTranslate(0.0, (cluster.ascent+cluster.descent)/2.0)
			// Rotate by 90 degrees in the center.
			ts = ts.PreRotateAt(
				-90.0,
				cluster.width/2.0,
				-(cluster.ascent+cluster.descent)/2.0,
			)

			cluster.pathTransform = ts

			// Move "baseline" to the middle and make height equal to width.
			cluster.ascent = cluster.width / 2.0
			cluster.descent = -cluster.width / 2.0
		} else {
			// Could not find a spec that explains this,
			// but this is how other applications are shifting the "rotated" characters
			// in the top-to-bottom mode.
			cluster.transform = cluster.transform.
				PreTranslate(0.0, (cluster.ascent+cluster.descent)/2.0)
		}
	}
}

// applyLetterSpacing is apply_letter_spacing in usvg src/text/layout.rs. It
// applies the `letter-spacing` property to a text chunk clusters.
//
// [In the CSS spec](https://www.w3.org/TR/css-text-3/#letter-spacing-property).
func applyLetterSpacing(chunk *TextChunk, clusters *[]glyphCluster) {
	// At least one span should have a non-zero spacing.
	if !slices.ContainsFunc(chunk.spans, func(span TextSpan) bool {
		return !ApproxZeroUlps32(span.letterSpacing, 4)
	}) {
		return
	}

	numClusters := len(*clusters)
	for i := range *clusters {
		cluster := &(*clusters)[i]
		// Spacing must be applied only to characters that belongs to the script
		// that supports spacing.
		// We are checking only the first code point, since it should be enough.
		// https://www.w3.org/TR/css-text-3/#cursive-tracking
		script := language.LookupScript(cluster.codepoint)
		if scriptSupportsLetterSpacing(script) {
			if span, ok := chunkSpanAt(chunk, cluster.byteIdx); ok {
				// A space after the last cluster should be ignored,
				// since it affects the bbox and text alignment.
				if i != numClusters-1 {
					cluster.advance += span.letterSpacing
				}

				// If the cluster advance became negative - clear it.
				// This is an UB so we can do whatever we want, and we mimic Chrome's behavior.
				if !isValidLength32(cluster.advance) {
					cluster.width = 0.0
					cluster.advance = 0.0
					cluster.glyphs = nil
				}
			}
		}
	}
}

// applyWordSpacing is apply_word_spacing in usvg src/text/layout.rs. It
// applies the `word-spacing` property to a text chunk clusters.
//
// [In the CSS spec](https://www.w3.org/TR/css-text-3/#propdef-word-spacing).
func applyWordSpacing(chunk *TextChunk, clusters []glyphCluster) {
	// At least one span should have a non-zero spacing.
	if !slices.ContainsFunc(chunk.spans, func(span TextSpan) bool {
		return !ApproxZeroUlps32(span.wordSpacing, 4)
	}) {
		return
	}

	for i := range clusters {
		cluster := &clusters[i]
		if isWordSeparatorCharacters(cluster.codepoint) {
			if span, ok := chunkSpanAt(chunk, cluster.byteIdx); ok {
				// Technically, word spacing 'should be applied half on each
				// side of the character', but it doesn't affect us in any way,
				// so we are ignoring this.
				cluster.advance += span.wordSpacing

				// After word spacing, `advance` can be negative.
			}
		}
	}
}

// formGlyphClusters is form_glyph_clusters in usvg src/text/layout.rs.
func formGlyphClusters(glyphs []glyph, text string, fontSize float32) glyphCluster {
	var x float32
	var width float32
	var advance float32

	positionedGlyphs := make([]PositionedGlyph, 0, len(glyphs))

	for i := range glyphs {
		glyph := &glyphs[i]
		sx := glyph.font.scale(fontSize)

		// Apply offset.
		//
		// The first glyph in the cluster will have an offset from 0x0,
		// but the later one will have an offset from the "current position".
		// So we have to keep an advance.
		// TODO: should be done only inside a single text span
		ts := path.TransformFromTranslate(x+float32(glyph.dx), -float32(glyph.dy))

		positionedGlyphs = append(positionedGlyphs, PositionedGlyph{
			glyphTs: ts,
			// Will be set later.
			clusterTs: path.DefaultTransform(),
			// Will be set later.
			spanTs:     path.DefaultTransform(),
			unitsPerEm: glyph.font.unitsPerEm,
			fontSize:   fontSize,
			Font:       glyph.font.id,
			Text:       glyph.text,
			ID:         glyph.id,
		})

		x += float32(glyph.width)

		glyphWidth := float32(float32(glyph.width) * sx)
		advance += glyphWidth
		if glyphWidth > width {
			width = glyphWidth
		}
	}

	byteIdx := glyphs[0].byteIdx
	font := glyphs[0].font
	return glyphCluster{
		byteIdx:           byteIdx,
		codepoint:         byteIdx.charFrom(text),
		width:             width,
		advance:           advance,
		textLengthSpacing: 0.0,
		ascent:            font.ascentAt(fontSize),
		descent:           font.descentAt(fontSize),
		hasRelativeShift:  false,
		transform:         path.DefaultTransform(),
		pathTransform:     path.DefaultTransform(),
		glyphs:            positionedGlyphs,
		visible:           true,
	}
}

// skrifaDecoration is skrifa::metrics::Decoration: the offset and the
// thickness of a line.
type skrifaDecoration struct {
	offset    float32
	thickness float32
}

// skrifaMetrics is skrifa::metrics::Metrics, with the fields that usvg
// reads. A nil field is None.
type skrifaMetrics struct {
	unitsPerEm uint16
	ascent     float32
	descent    float32
	xHeight    *float32
	strikeout  *skrifaDecoration
	underline  *skrifaDecoration
}

// The minimum sizes of the tables that skrifa reads, below which read-fonts
// fails to read a table.
const (
	headMinSize = 54
	hheaMinSize = 36
	postMinSize = 32
)

// os2MinSize is the size of the OS/2 table for its version, below which
// read-fonts fails to read it.
func os2MinSize(version uint16) int {
	switch version {
	case 0:
		return 78
	case 1:
		return 86
	case 2, 3, 4:
		return 96
	}
	return 100
}

// readOS2 returns the OS/2 table of face, or nil when the face has none or
// read-fonts cannot read it.
func readOS2(face *parsedFace) []byte {
	os2 := face.rawTable("OS/2")
	if len(os2) < 2 || len(os2) < os2MinSize(binary.BigEndian.Uint16(os2)) {
		return nil
	}
	return os2
}

// readMVAR returns the MVAR table of face, or false when it has none.
func readMVAR(face *parsedFace) (tables.MVAR, bool) {
	raw := face.rawTable("MVAR")
	if raw == nil {
		return tables.MVAR{}, false
	}
	mvar, _, err := tables.ParseMVAR(raw)
	return mvar, err == nil
}

// mvarMetricDelta is Mvar::metric_delta of read-fonts, which skrifa and usvg
// call. It returns the delta of the metric tag at coords, or 0.
func mvarMetricDelta(mvar tables.MVAR, tag string, coords []tables.Coord) float32 {
	t := ot.MustNewTag(tag)
	for _, rec := range mvar.ValueRecords {
		if rec.ValueTag == t {
			return mvar.ItemVariationStore.GetDelta(rec.Index, coords)
		}
	}
	return 0
}

// newSkrifaMetrics is Metrics::new of skrifa src/metrics.rs, for an unscaled
// size, which usvg calls with font.metrics. coords is the location of the
// face in its normalized variation space.
//
// Port: skrifa reads the tables head, post, OS/2, hhea and MVAR with
// read-fonts. go-text loads them, and they are read here by the same rules.
func newSkrifaMetrics(face *parsedFace, coords []tables.Coord) skrifaMetrics {
	var metrics skrifaMetrics
	if head := face.rawTable("head"); len(head) >= headMinSize {
		metrics.unitsPerEm = binary.BigEndian.Uint16(head[18:])
	}
	i16 := func(b []byte, off int) float32 {
		return float32(int16(binary.BigEndian.Uint16(b[off:])))
	}
	if post := face.rawTable("post"); len(post) >= postMinSize {
		metrics.underline = &skrifaDecoration{
			offset:    i16(post, 8),
			thickness: i16(post, 10),
		}
	}
	// Choosing proper line metrics is a challenge due to the changing
	// spec, backward compatibility and broken fonts.
	//
	// We use the same strategy as FreeType:
	// 1. Use the OS/2 metrics if the table exists and the USE_TYPO_METRICS
	//    flag is set.
	// 2. Otherwise, use the hhea metrics.
	// 3. If hhea metrics are zero and the OS/2 table exists:
	//    3a. Use the typo metrics if they are non-zero
	//    3b. Otherwise, use the win metrics
	os2 := readOS2(face)
	usedTypoMetrics := false
	if os2 != nil {
		const useTypoMetrics = 1 << 7
		if binary.BigEndian.Uint16(os2[62:])&useTypoMetrics != 0 {
			metrics.ascent = i16(os2, 68)
			metrics.descent = i16(os2, 70)
			usedTypoMetrics = true
		}
		if binary.BigEndian.Uint16(os2) >= 2 {
			xHeight := i16(os2, 86)
			metrics.xHeight = &xHeight
		}
		metrics.strikeout = &skrifaDecoration{
			offset:    i16(os2, 28),
			thickness: i16(os2, 26),
		}
	}
	if !usedTypoMetrics {
		if hhea := face.rawTable("hhea"); len(hhea) >= hheaMinSize {
			metrics.ascent = i16(hhea, 4)
			metrics.descent = i16(hhea, 6)
		}
		if metrics.ascent == 0.0 && metrics.descent == 0.0 {
			if os2 != nil {
				if i16(os2, 68) != 0 || i16(os2, 70) != 0 {
					metrics.ascent = i16(os2, 68)
					metrics.descent = i16(os2, 70)
				} else {
					metrics.ascent = float32(binary.BigEndian.Uint16(os2[74:]))
					// Win descent is always positive while other descent values are negative. Negate it
					// to ensure we return consistent metrics.
					metrics.descent = -float32(binary.BigEndian.Uint16(os2[76:]))
				}
			}
		}
	}
	if mvar, ok := readMVAR(face); ok && !allZero(coords) {
		metricDelta := func(tag string) float32 { return mvarMetricDelta(mvar, tag, coords) }
		metrics.ascent += metricDelta("hasc")
		metrics.descent += metricDelta("hdsc")
		if metrics.xHeight != nil {
			*metrics.xHeight += metricDelta("xhgt")
		}
		if metrics.underline != nil {
			metrics.underline.offset += metricDelta("undo")
			metrics.underline.thickness += metricDelta("unds")
		}
		if metrics.strikeout != nil {
			metrics.strikeout.offset += metricDelta("stro")
			metrics.strikeout.thickness += metricDelta("strs")
		}
	}
	return metrics
}

// allZero reports whether coords is empty or holds only zeros, the case where
// LocationRef::effective_coords of skrifa is empty.
func allZero(coords []tables.Coord) bool {
	for _, c := range coords {
		if c != 0 {
			return false
		}
	}
	return true
}

// variationsLocation is font.axes().location(variations) of skrifa, which
// usvg calls. It returns the normalized coordinates of the variations in
// the variation space of the font.
//
// Port: go-text normalizes the values with the tables fvar and avar, as
// skrifa does.
func variationsLocation(ft *font.Font, variations []FontVariation) []tables.Coord {
	face := font.NewFace(ft)
	face.SetVariations(toGoTextVariations(variations))
	return face.Coords()
}

// toGoTextVariations converts variations to the variations of go-text.
func toGoTextVariations(variations []FontVariation) []font.Variation {
	vs := make([]font.Variation, len(variations))
	for i, v := range variations {
		vs[i] = font.Variation{Tag: ot.NewTag(v.Tag[0], v.Tag[1], v.Tag[2], v.Tag[3]), Value: v.Value}
	}
	return vs
}

// databaseLoadFont is DatabaseExt::load_font for Database in usvg
// src/text/layout.rs.
//
// Port: skrifa::FontRef::from_index becomes the face that go-text parses, and
// font.metrics becomes newSkrifaMetrics.
func databaseLoadFont(fontdb *FontDatabase, id FontID, variations []FontVariation) (*resolvedFont, bool) {
	face, ok := fontdb.withFaceData(id)
	if !ok {
		return nil, false
	}

	// For variable fonts, metrics depend on the position in the design space,
	// so resolve the requested variations to a normalized location first.
	// Metrics are always in font units, so the size stays unscaled.
	coords := variationsLocation(face.font, variations)
	metrics := newSkrifaMetrics(face, coords)

	// Reject fonts with an out-of-range unitsPerEm, like `ttf-parser` did.
	if metrics.unitsPerEm < 16 || metrics.unitsPerEm > 16384 {
		return nil, false
	}
	unitsPerEm := metrics.unitsPerEm

	ascent := metrics.ascent
	descent := metrics.descent

	var xHeight uint16
	if metrics.xHeight != nil && *metrics.xHeight > 0.0 {
		if v := wide.TruncI32(round32(*metrics.xHeight)); v >= 0 && v <= math.MaxUint16 {
			xHeight = uint16(v)
		}
	}
	if xHeight == 0 {
		// If not set - fallback to height * 45%.
		// 45% is what Firefox uses.
		v := wide.TruncI32(round32((ascent - descent) * 0.45))
		if v <= 0 || v > math.MaxUint16 {
			return nil, false
		}
		xHeight = uint16(v)
	}

	var lineThroughPosition int16
	if metrics.strikeout != nil {
		lineThroughPosition = asI16(round32(metrics.strikeout.offset))
	} else {
		lineThroughPosition = int16(xHeight) / 2
	}

	var underlinePosition int16
	var underlineThickness uint16
	if metrics.underline != nil {
		thickness := wide.TruncI32(round32(metrics.underline.thickness))
		if thickness > 0 && thickness <= math.MaxUint16 {
			underlineThickness = uint16(thickness)
		} else {
			// `skrifa` guarantees that units_per_em is > 0
			underlineThickness = unitsPerEm / 12
		}

		underlinePosition = asI16(round32(metrics.underline.offset))
	} else {
		underlinePosition = -int16(unitsPerEm) / 9
		underlineThickness = unitsPerEm / 12
	}

	// 0.2 and 0.4 are generic offsets used by some applications (Inkscape/librsvg).
	subscriptOffset := asI16(round32(float32(unitsPerEm) / 0.2))
	superscriptOffset := asI16(round32(float32(unitsPerEm) / 0.4))

	// TODO: Consider upstreaming into skrifa
	if os2 := readOS2(face); os2 != nil {
		subscriptOffset = int16(binary.BigEndian.Uint16(os2[16:]))
		superscriptOffset = int16(binary.BigEndian.Uint16(os2[24:]))
	}
	if mvar, ok := readMVAR(face); ok && len(coords) != 0 {
		metricDelta := func(tag string) float32 { return mvarMetricDelta(mvar, tag, coords) }

		subscriptOffset += asI16(round32(metricDelta("sbyo")))
		superscriptOffset += asI16(round32(metricDelta("spyo")))
	}

	return &resolvedFont{
		id:                  id,
		unitsPerEm:          unitsPerEm,
		ascent:              asI16(round32(ascent)),
		descent:             asI16(round32(descent)),
		xHeight:             xHeight,
		underlinePosition:   underlinePosition,
		underlineThickness:  underlineThickness,
		lineThroughPosition: lineThroughPosition,
		subscriptOffset:     subscriptOffset,
		superscriptOffset:   superscriptOffset,
	}, true
}

// asI16 is the Rust cast "x as i16" of an f32: it truncates and saturates,
// and NaN gives 0.
func asI16(x float32) int16 {
	switch {
	case x != x:
		return 0
	case x <= math.MinInt16:
		return math.MinInt16
	case x >= math.MaxInt16:
		return math.MaxInt16
	}
	return int16(x)
}

// databaseHasChar is DatabaseExt::has_char for Database in usvg
// src/text/layout.rs.
//
// Port: skrifa::charmap::Charmap::map becomes the cmap of go-text.
func databaseHasChar(fontdb *FontDatabase, id FontID, c rune) bool {
	face, ok := fontdb.withFaceData(id)
	if !ok {
		return false
	}
	_, ok = face.font.NominalGlyph(c)
	return ok
}

// shapeText is shape_text in usvg src/text/layout.rs. It does the text
// shaping with font fallback.
//
// Port: opt holds the logger (D20).
func shapeText(
	text string,
	font *resolvedFont,
	smallCaps bool,
	applyKerning bool,
	variations []FontVariation,
	fontSize float32,
	fontOpticalSizing FontOpticalSizing,
	resolver *FontResolver,
	fontdb *FontDatabase,
	opt *options,
) []glyph {
	glyphs := shapeTextWithFont(
		text,
		font,
		smallCaps,
		applyKerning,
		variations,
		fontSize,
		fontOpticalSizing,
		fontdb,
	)

	// Remember all fonts used for shaping.
	usedFonts := []FontID{font.id}

	// Loop until all glyphs become resolved or until no more fonts are left.
outer:
	for {
		var missing rune
		hasMissing := false
		for i := range glyphs {
			if glyphs[i].isMissing() {
				missing = glyphs[i].byteIdx.charFrom(text)
				hasMissing = true
				break
			}
		}

		if !hasMissing {
			break outer
		}

		id, ok := resolver.selectFallback(missing, usedFonts, fontdb, opt)
		if !ok {
			break outer
		}
		fallbackFont, ok := databaseLoadFont(fontdb, id, variations)
		if !ok {
			break outer
		}

		// Shape again, using a new font.
		fallbackGlyphs := shapeTextWithFont(
			text,
			fallbackFont,
			smallCaps,
			applyKerning,
			variations,
			fontSize,
			fontOpticalSizing,
			fontdb,
		)

		allMatched := !slices.ContainsFunc(fallbackGlyphs, func(g glyph) bool { return g.isMissing() })
		if allMatched {
			// Replace all glyphs when all of them were matched.
			glyphs = fallbackGlyphs
			break outer
		}

		// We assume, that shaping with an any font will produce the same amount of glyphs.
		// This is incorrect, but good enough for now.
		if len(glyphs) != len(fallbackGlyphs) {
			break outer
		}

		// TODO: Replace clusters and not glyphs. This should be more accurate.

		// Copy new glyphs.
		for i := range glyphs {
			if glyphs[i].isMissing() && !fallbackGlyphs[i].isMissing() {
				glyphs[i] = fallbackGlyphs[i]
			}
		}

		// Remember this font.
		usedFonts = append(usedFonts, fallbackFont.id)
	}

	// Warn about missing glyphs.
	for i := range glyphs {
		if glyphs[i].isMissing() {
			c := glyphs[i].byteIdx.charFrom(text)
			// TODO: print a full grapheme
			opt.warnf("No fonts with a %c/U+%s character were found.", c, strings.ToUpper(strconv.FormatUint(uint64(c), 16)))
		}
	}

	return glyphs
}

// hasAxis reports whether the variable font of face has an axis tag. It is
// font.axes().get_by_tag(tag).is_some() of skrifa and harfrust.
//
// Port: go-text keeps the axes of a font private, so the table fvar is read
// from its bytes.
func hasAxis(face *parsedFace, tag [4]byte) bool {
	fvar := face.rawTable("fvar")
	if len(fvar) < 16 {
		return false
	}
	axesOffset := int(binary.BigEndian.Uint16(fvar[4:]))
	axisCount := int(binary.BigEndian.Uint16(fvar[8:]))
	axisSize := int(binary.BigEndian.Uint16(fvar[10:]))
	for i := range axisCount {
		off := axesOffset + i*axisSize
		if off+4 > len(fvar) {
			return false
		}
		if [4]byte(fvar[off:off+4]) == tag {
			return true
		}
	}
	return false
}

// bidiRun is a run of text of one embedding level, as BidiInfo::visual_runs
// of unicode-bidi gives it: a range of bytes and its level.
type bidiRun struct {
	start int
	end   int
	level bidi.Level
}

// visualRuns is BidiInfo::new(text, Some(Level::ltr())) and
// BidiInfo::visual_runs of the first paragraph of unicode-bidi, which usvg
// calls. It returns the runs of the first paragraph of text in visual order.
//
// Port: the package bidi of go-text resolves the levels of the first
// paragraph, with rule L1. It gives runs of one direction, and the level of
// the first character of each run, where unicode-bidi splits the runs at each
// change of level. The runs are then reordered by rule L2, as unicode-bidi
// does. The two differ only where explicit embeddings nest two levels of one
// direction.
func visualRuns(text string) []bidiRun {
	runeOffsets := make([]int, 0, len(text)+1)
	for i := range text {
		runeOffsets = append(runeOffsets, i)
	}
	runeOffsets = append(runeOffsets, len(text))

	var p bidi.Paragraph
	segmented := p.SegmentString(text, bidi.LeftToRight)
	runs := make([]bidiRun, 0, segmented.NumRuns())
	for i := range segmented.NumRuns() {
		r := segmented.Run(i)
		runs = append(runs, bidiRun{start: runeOffsets[r.Start], end: runeOffsets[r.End], level: r.Level})
	}
	if len(runs) == 0 {
		return runs
	}

	// Re-order the odd runs.
	// Stop at the lowest *odd* level.
	maxLevel, minLevel := runs[0].level, runs[0].level
	for _, r := range runs {
		maxLevel = max(maxLevel, r.level)
		minLevel = min(minLevel, r.level)
	}
	if minLevel%2 == 0 {
		minLevel++
	}
	for maxLevel >= minLevel {
		// Look for the start of a sequence of consecutive runs of max_level or higher.
		seqStart := 0
		for seqStart < len(runs) {
			if runs[seqStart].level < maxLevel {
				seqStart++
				continue
			}

			// Found the start of a sequence. Now find the end.
			seqEnd := seqStart + 1
			for seqEnd < len(runs) {
				if runs[seqEnd].level < maxLevel {
					break
				}
				seqEnd++
			}

			// Reverse the runs within this sequence.
			slices.Reverse(runs[seqStart:seqEnd])

			seqStart = seqEnd
		}

		maxLevel--
	}

	return runs
}

// guessScript is the part of UnicodeBuffer::guess_segment_properties of
// harfrust that guesses the script: the script of the first character that
// is not Common, Inherited or Unknown.
//
// Port: the method of go-text also sets the language from the environment,
// where harfrust leaves it unset, so the buffer takes only this script.
func guessScript(runes []rune) language.Script {
	for _, r := range runes {
		switch s := language.LookupScript(r); s {
		case language.Common, language.Inherited, language.Unknown:
		default:
			return s
		}
	}
	return 0
}

// shapeTextWithFont is shape_text_with_font in usvg src/text/layout.rs. It
// converts a text into a list of glyph IDs.
//
// This function will do the BIDI reordering and text shaping.
//
// Port: upstream returns None when the font does not load, which each
// caller takes as no glyphs, so Go returns nil. harfrust becomes the package harfbuzz of go-text, with a face whose
// coordinates are the variations, and unicode-bidi becomes visualRuns.
func shapeTextWithFont(
	text string,
	font *resolvedFont,
	smallCaps bool,
	applyKerning bool,
	variations []FontVariation,
	fontSize float32,
	fontOpticalSizing FontOpticalSizing,
	fontdb *FontDatabase,
) []glyph {
	pf, ok := fontdb.withFaceData(font.id)
	if !ok {
		return nil
	}

	// Build the list of variations to apply
	vars := slices.Clone(variations)

	// Automatic optical sizing: if font-optical-sizing is auto and the font has
	// an 'opsz' axis that isn't explicitly set, auto-set it to match font size.
	// This matches browser behavior (CSS font-optical-sizing: auto).
	if fontOpticalSizing == FontOpticalSizingAuto {
		hasExplicitOpsz := slices.ContainsFunc(vars, func(v FontVariation) bool { return v.Tag == opsz })
		if !hasExplicitOpsz && hasAxis(pf, opsz) {
			vars = append(vars, FontVariation{Tag: opsz, Value: fontSize})
		}
	}

	var glyphs []glyph

	// A shaper instance is only needed to apply variations.
	face := gotextNewFace(pf)
	if len(vars) != 0 {
		face.SetVariations(toGoTextVariations(vars))
	}
	hbFont := harfbuzz.NewFont(face)

	var features []harfbuzz.Feature
	if smallCaps {
		features = append(features, harfbuzz.Feature{
			Tag: ot.MustNewTag("smcp"), Value: 1,
			Start: harfbuzz.FeatureGlobalStart, End: harfbuzz.FeatureGlobalEnd,
		})
	}
	if !applyKerning {
		features = append(features, harfbuzz.Feature{
			Tag: ot.MustNewTag("kern"), Value: 0,
			Start: harfbuzz.FeatureGlobalStart, End: harfbuzz.FeatureGlobalEnd,
		})
	}

	for _, run := range visualRuns(text) {
		subText := text[run.start:run.end]
		if subText == "" {
			continue
		}

		ltr := run.level%2 == 0
		direction := harfbuzz.LeftToRight
		if !ltr {
			direction = harfbuzz.RightToLeft
		}

		// Port: harfrust takes the byte offset of each character as its
		// cluster. go-text takes the index of the rune, which byteOffsets
		// maps back.
		runes := []rune(subText)
		byteOffsets := make([]int, 0, len(runes))
		for i := range subText {
			byteOffsets = append(byteOffsets, i)
		}

		buffer := harfbuzz.NewBuffer()
		buffer.AddRunes(runes, 0, -1)
		buffer.Props.Direction = direction

		// TODO: explicitly set language?
		buffer.Props.Script = guessScript(runes)

		buffer.Shape(hbFont, features)

		positions := buffer.Pos
		infos := buffer.Info
		clusterAt := func(i int) int { return byteOffsets[infos[i].Cluster] }

		for i := range infos {
			pos := positions[i]
			info := infos[i]
			idx := run.start + clusterAt(i)

			start := clusterAt(i)

			last := i + 1
			if !ltr {
				last = i - 1
			}
			end := len(subText)
			if last >= 0 && last < len(infos) {
				end = clusterAt(last)
			}

			clusterLen := 0 // TODO: can fail?
			glyphText := ""
			if end >= start {
				clusterLen = end - start
				glyphText = subText[start:end]
			}

			glyphs = append(glyphs, glyph{
				byteIdx:    newByteIndex(idx),
				clusterLen: clusterLen,
				text:       glyphText,
				id:         GlyphID(info.Glyph),
				dx:         pos.XOffset,
				dy:         pos.YOffset,
				width:      pos.XAdvance,
				font:       font,
			})
		}
	}

	return glyphs
}

// gotextNewFace returns a new face of go-text for the font of pf, with no
// variations.
func gotextNewFace(pf *parsedFace) *font.Face {
	return font.NewFace(pf.font)
}

// glyphClusters is GlyphClusters in usvg src/text/layout.rs. It is an
// iterator over glyph clusters.
//
// Input:  0 2 2 2 3 4 4 5 5
// Result: 0 1     4 5   7
//
// Port: the iterator yields the range of each cluster, with its byte index.
func glyphClusters(data []glyph) iter.Seq2[[2]int, byteIndex] {
	return func(yield func([2]int, byteIndex) bool) {
		idx := 0
		for idx != len(data) {
			start := idx
			cluster := data[idx].byteIdx
			for _, g := range data[idx:] {
				if g.byteIdx != cluster {
					break
				}

				idx++
			}

			if !yield([2]int{start, idx}, cluster) {
				return
			}
		}
	}
}

// scriptSupportsLetterSpacing is script_supports_letter_spacing in usvg
// src/text/layout.rs. It checks that selected script supports letter
// spacing.
//
// [In the CSS spec](https://www.w3.org/TR/css-text-3/#cursive-tracking).
//
// The list itself is from: https://github.com/harfbuzz/harfbuzz/issues/64
//
// Port: unicode_script::Script becomes the script of the package language
// of go-text.
func scriptSupportsLetterSpacing(script language.Script) bool {
	switch script {
	case language.Arabic,
		language.Syriac,
		language.Nko,
		language.Manichaean,
		language.Psalter_Pahlavi,
		language.Mandaic,
		language.Mongolian,
		language.Phags_Pa,
		language.Devanagari,
		language.Bengali,
		language.Gurmukhi,
		language.Modi,
		language.Sharada,
		language.Syloti_Nagri,
		language.Tirhuta,
		language.Ogham:
		return false
	}
	return true
}

// glyph is Glyph in usvg src/text/layout.rs. It is a glyph.
//
// Basically, a glyph ID and it's metrics.
type glyph struct {
	// The glyph ID in the font.
	id GlyphID

	// Position in bytes in the original string.
	//
	// We use it to match a glyph with a character in the text chunk and therefore with the style.
	byteIdx byteIndex

	// The length of the cluster in bytes.
	clusterLen int

	// The text from the original string that corresponds to that glyph.
	text string

	// The glyph offset in font units.
	dx int32

	// The glyph offset in font units.
	dy int32

	// The glyph width / X-advance in font units.
	width int32

	// Reference to the source font.
	//
	// Each glyph can have it's own source font.
	font *resolvedFont
}

// isMissing is Glyph::is_missing in usvg src/text/layout.rs.
func (g *glyph) isMissing() bool {
	return g.id == 0
}

// resolvedFont is ResolvedFont in usvg src/text/layout.rs.
//
//nolint:dupword // the comments of upstream
type resolvedFont struct {
	id FontID

	unitsPerEm uint16

	// All values below are in font units.
	ascent  int16
	descent int16
	xHeight uint16

	underlinePosition  int16
	underlineThickness uint16

	// line-through thickness should be the the same as underline thickness
	// according to the TrueType spec:
	// https://docs.microsoft.com/en-us/typography/opentype/spec/os2#ystrikeoutsize
	lineThroughPosition int16

	subscriptOffset   int16
	superscriptOffset int16
}

// chunkSpanAt is chunk_span_at in usvg src/text/layout.rs.
func chunkSpanAt(chunk *TextChunk, byteOffset byteIndex) (*TextSpan, bool) {
	for i := range chunk.spans {
		if spanContains(&chunk.spans[i], byteOffset) {
			return &chunk.spans[i], true
		}
	}
	return nil, false
}

// spanContains is span_contains in usvg src/text/layout.rs.
func spanContains(span *TextSpan, byteOffset byteIndex) bool {
	return byteOffset.value() >= span.start && byteOffset.value() < span.end
}

// isWordSeparatorCharacters is is_word_separator_characters in usvg
// src/text/layout.rs. It checks that the selected character is a word
// separator.
//
// According to: https://www.w3.org/TR/css-text-3/#word-separator
func isWordSeparatorCharacters(c rune) bool {
	switch c {
	case 0x0020, 0x00A0, 0x1361, 0x010100, 0x010101, 0x01039F, 0x01091F:
		return true
	}
	return false
}

// scale is ResolvedFont::scale in usvg src/text/layout.rs.
func (f *resolvedFont) scale(fontSize float32) float32 {
	return fontSize / float32(f.unitsPerEm)
}

// ascentAt is ResolvedFont::ascent in usvg src/text/layout.rs. It takes
// another name, because the field ascent has that name.
func (f *resolvedFont) ascentAt(fontSize float32) float32 {
	return float32(float32(f.ascent) * f.scale(fontSize))
}

// descentAt is ResolvedFont::descent in usvg src/text/layout.rs.
func (f *resolvedFont) descentAt(fontSize float32) float32 {
	return float32(float32(f.descent) * f.scale(fontSize))
}

// height is ResolvedFont::height in usvg src/text/layout.rs.
func (f *resolvedFont) height(fontSize float32) float32 {
	return f.ascentAt(fontSize) - f.descentAt(fontSize)
}

// xHeightAt is ResolvedFont::x_height in usvg src/text/layout.rs.
func (f *resolvedFont) xHeightAt(fontSize float32) float32 {
	return float32(float32(f.xHeight) * f.scale(fontSize))
}

// underlinePositionAt is ResolvedFont::underline_position in usvg
// src/text/layout.rs.
func (f *resolvedFont) underlinePositionAt(fontSize float32) float32 {
	return float32(float32(f.underlinePosition) * f.scale(fontSize))
}

// underlineThicknessAt is ResolvedFont::underline_thickness in usvg
// src/text/layout.rs.
func (f *resolvedFont) underlineThicknessAt(fontSize float32) float32 {
	return float32(float32(f.underlineThickness) * f.scale(fontSize))
}

// lineThroughPositionAt is ResolvedFont::line_through_position in usvg
// src/text/layout.rs.
func (f *resolvedFont) lineThroughPositionAt(fontSize float32) float32 {
	return float32(float32(f.lineThroughPosition) * f.scale(fontSize))
}

// subscriptOffsetAt is ResolvedFont::subscript_offset in usvg
// src/text/layout.rs.
func (f *resolvedFont) subscriptOffsetAt(fontSize float32) float32 {
	return float32(float32(f.subscriptOffset) * f.scale(fontSize))
}

// superscriptOffsetAt is ResolvedFont::superscript_offset in usvg
// src/text/layout.rs.
func (f *resolvedFont) superscriptOffsetAt(fontSize float32) float32 {
	return float32(float32(f.superscriptOffset) * f.scale(fontSize))
}

// dominantBaselineShift is ResolvedFont::dominant_baseline_shift in usvg
// src/text/layout.rs.
func (f *resolvedFont) dominantBaselineShift(baseline DominantBaseline, fontSize float32) float32 {
	var alignment AlignmentBaseline
	switch baseline {
	case DominantBaselineAuto:
		alignment = AlignmentBaselineAuto
	case DominantBaselineUseScript:
		alignment = AlignmentBaselineAuto // unsupported
	case DominantBaselineNoChange:
		alignment = AlignmentBaselineAuto // already resolved
	case DominantBaselineResetSize:
		alignment = AlignmentBaselineAuto // unsupported
	case DominantBaselineIdeographic:
		alignment = AlignmentBaselineIdeographic
	case DominantBaselineAlphabetic:
		alignment = AlignmentBaselineAlphabetic
	case DominantBaselineHanging:
		alignment = AlignmentBaselineHanging
	case DominantBaselineMathematical:
		alignment = AlignmentBaselineMathematical
	case DominantBaselineCentral:
		alignment = AlignmentBaselineCentral
	case DominantBaselineMiddle:
		alignment = AlignmentBaselineMiddle
	case DominantBaselineTextAfterEdge:
		alignment = AlignmentBaselineTextAfterEdge
	case DominantBaselineTextBeforeEdge:
		alignment = AlignmentBaselineTextBeforeEdge
	}

	return f.alignmentBaselineShift(alignment, fontSize)
}

// alignmentBaselineShift is ResolvedFont::alignment_baseline_shift in usvg
// src/text/layout.rs.
//
// The `alignment-baseline` property is a mess.
//
// The SVG 1.1 spec (https://www.w3.org/TR/SVG11/text.html#BaselineAlignmentProperties)
// goes on and on about what this property suppose to do, but doesn't actually explain
// how it should be implemented. It's just a very verbose overview.
//
// As of Nov 2022, only Chrome and Safari support `alignment-baseline`. Firefox isn't.
// Same goes for basically every SVG library in existence.
// Meaning we have no idea how exactly it should be implemented.
//
// And even Chrome and Safari cannot agree on how to handle `baseline`, `after-edge`,
// `text-after-edge` and `ideographic` variants. Producing vastly different output.
//
// As per spec, a proper implementation should get baseline values from the font itself,
// using `BASE` and `bsln` TrueType tables. If those tables are not present,
// we have to synthesize them (https://drafts.csswg.org/css-inline/#baseline-synthesis-fonts).
// And in the worst case scenario simply fallback to hardcoded values.
//
// Also, most fonts do not provide `BASE` and `bsln` tables to begin with.
//
// Again, as of Nov 2022, Chrome does only the latter:
// https://github.com/chromium/chromium/blob/main/third_party/blink/renderer/platform/fonts/font_metrics.cc#L153
//
// Since baseline TrueType tables parsing and baseline synthesis are pretty hard,
// we do what Chrome does - use hardcoded values. And it seems like Safari does the same.
//
// But that's not all! SVG 2 and CSS Inline Layout 3 did a baseline handling overhaul,
// and it's far more complex now. Not sure if anyone actually supports it.
func (f *resolvedFont) alignmentBaselineShift(alignment AlignmentBaseline, fontSize float32) float32 {
	switch alignment {
	case AlignmentBaselineAuto:
		return 0.0
	case AlignmentBaselineBaseline:
		return 0.0
	case AlignmentBaselineBeforeEdge, AlignmentBaselineTextBeforeEdge:
		return f.ascentAt(fontSize)
	case AlignmentBaselineMiddle:
		return f.xHeightAt(fontSize) * 0.5
	case AlignmentBaselineCentral:
		return f.ascentAt(fontSize) - float32(f.height(fontSize)*0.5)
	case AlignmentBaselineAfterEdge, AlignmentBaselineTextAfterEdge:
		return f.descentAt(fontSize)
	case AlignmentBaselineIdeographic:
		return f.descentAt(fontSize)
	case AlignmentBaselineAlphabetic:
		return 0.0
	case AlignmentBaselineHanging:
		return f.ascentAt(fontSize) * 0.8
	case AlignmentBaselineMathematical:
		return f.ascentAt(fontSize) * 0.5
	}
	return 0.0
}

// fontsCache is FontsCache in usvg src/text/layout.rs.
//
// Port: Go keys the map with Font.key, because a Font holds slices.
type fontsCache map[string]*resolvedFont

// key returns a text that is equal for two fonts when the derived Eq of Font
// in usvg src/tree/text.rs is true, so that it keys a map as the derived
// Hash does.
func (f *Font) key() string {
	var b strings.Builder
	for _, family := range f.families {
		fmt.Fprintf(&b, "%d:%q,", family.Kind, family.Name)
	}
	fmt.Fprintf(&b, "|%d|%d|%d|", f.style, f.stretch, f.weight)
	for _, v := range f.variations {
		fmt.Fprintf(&b, "%q=%08x,", v.Tag[:], math.Float32bits(v.Value))
	}
	return b.String()
}

// byteIndex is ByteIndex in usvg src/text/layout.rs. It is a read-only text
// index in bytes.
//
// Guarantee to be on a char boundary and in text bounds.
type byteIndex int

// newByteIndex is ByteIndex::new in usvg src/text/layout.rs.
func newByteIndex(i int) byteIndex {
	return byteIndex(i)
}

// value is ByteIndex::value in usvg src/text/layout.rs.
func (i byteIndex) value() int {
	return int(i)
}

// codePointAt is ByteIndex::code_point_at in usvg src/text/layout.rs. It
// converts byte position into a code point position.
func (i byteIndex) codePointAt(text string) int {
	n := 0
	for j := range text {
		if j == int(i) {
			break
		}
		n++
	}
	return n
}

// charFrom is ByteIndex::char_from in usvg src/text/layout.rs. It converts
// byte position into a character.
func (i byteIndex) charFrom(text string) rune {
	r, _ := utf8.DecodeRuneInString(text[i:])
	return r
}
