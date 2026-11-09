// Copyright 2022 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"encoding/binary"
	"slices"
	"strconv"

	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"

	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/internal/xmlwriter"
	"github.com/xo/okage/path"
)

// This file ports usvg src/text/flatten.rs.
//
// Port: go-text takes the place of skrifa (D9). The outline of a glyph comes
// from font.Face.GlyphDataOutline, and the tables CBLC, CBDT, sbix, SVG and
// COLR are read with the package tables of go-text, by the rules of skrifa.

// resolveRenderingMode is resolve_rendering_mode in usvg
// src/text/flatten.rs.
func resolveRenderingMode(text *Text) ShapeRendering {
	switch text.renderingMode {
	case TextRenderingOptimizeSpeed:
		return ShapeRenderingCrispEdges
	case TextRenderingOptimizeLegibility:
		return ShapeRenderingGeometricPrecision
	}
	return ShapeRenderingGeometricPrecision
}

// effectiveVariations is effective_variations in usvg src/text/flatten.rs.
// It returns the effective variation settings for a glyph: the span's
// explicit variations plus an automatically computed `opsz` value when
// `font-optical-sizing: auto` is in effect and the font has an `opsz` axis
// that wasn't set explicitly. This matches browser behavior
// (CSS font-optical-sizing: auto).
func effectiveVariations(cache *converterCache, span *Span, glyph *PositionedGlyph) []FontVariation {
	variations := slices.Clone(span.Variations)
	if span.FontOpticalSizing == FontOpticalSizingAuto &&
		!slices.ContainsFunc(variations, func(v FontVariation) bool { return v.Tag == opsz }) &&
		cache.hasOpszAxis(glyph.Font) {
		variations = append(variations, NewFontVariation(opsz, glyph.FontSize()))
	}
	return variations
}

// pushOutlinePaths is push_outline_paths in usvg src/text/flatten.rs.
func pushOutlinePaths(
	span *Span,
	builder **path.PathBuilder,
	newChildren *[]Node,
	renderingMode ShapeRendering,
	absTransform Transform,
) {
	b := *builder
	*builder = path.NewPathBuilder()

	if p, ok := b.Finish(); ok {
		if p, ok := newPath(
			"",
			span.Visible,
			cloneFill(span.Fill),
			cloneStroke(span.Stroke),
			span.PaintOrder,
			renderingMode,
			p,
			absTransform,
		); ok {
			*newChildren = append(*newChildren, p)
		}
	}
}

// flatten is flatten in usvg src/text/flatten.rs.
func flatten(text *Text, cache *converterCache) (*Group, NonZeroRect, bool) {
	var newChildren []Node

	absTransform := text.absTransform
	renderingMode := resolveRenderingMode(text)

	for i := range text.layouted {
		span := &text.layouted[i]
		if span.Overline != nil {
			p := clonePath(span.Overline)
			p.renderingMode = renderingMode
			newChildren = append(newChildren, p)
		}

		if span.Underline != nil {
			p := clonePath(span.Underline)
			p.renderingMode = renderingMode
			newChildren = append(newChildren, p)
		}

		// Instead of always processing each glyph separately, we always collect
		// as many outline glyphs as possible by pushing them into the span_builder
		// and only if we encounter a different glyph, or we reach the very end of the
		// span to we push the actual outline paths into new_children. This way, we don't need
		// to create a new path for every glyph if we have many consecutive glyphs
		// with just outlines (which is the most common case).
		spanBuilder := path.NewPathBuilder()

		for j := range span.PositionedGlyphs {
			glyph := &span.PositionedGlyphs[j]
			variations := effectiveVariations(cache, span, glyph)

			// A (best-effort conversion of a) COLR glyph.
			if tree, ok := cache.fontdbColr(glyph.Font, glyph.ID, variations); ok {
				group := emptyGroup()
				group.transform = glyph.ColrTransform()
				// TODO: Probably need to update abs_transform of children? Same
				// for SVG and bitmap glyphs.
				root := tree.root.cloneGroup()
				group.children = append(group.children, &root)
				group.calculateBoundingBoxes()

				newChildren = append(newChildren, group)
			} else if node, ok := cache.fontdbSvg(glyph.Font, glyph.ID); ok {
				// An SVG glyph. Will return the usvg node containing the glyph descriptions.
				pushOutlinePaths(
					span,
					&spanBuilder,
					&newChildren,
					renderingMode,
					absTransform,
				)

				group := emptyGroup()
				group.transform = glyph.SvgTransform()
				group.children = append(group.children, cloneNode(node))
				group.calculateBoundingBoxes()

				newChildren = append(newChildren, group)
			} else if img, ok := cache.fontdbRaster(glyph.Font, glyph.ID); ok {
				// A bitmap glyph.
				pushOutlinePaths(
					span,
					&spanBuilder,
					&newChildren,
					renderingMode,
					absTransform,
				)

				var transform Transform
				if img.isSbix {
					var xMin, yMin int16
					if img.glyphBBox != nil {
						xMin, yMin = img.glyphBBox.xMin, img.glyphBBox.yMin
					}
					transform = glyph.SbixTransform(
						float32(img.x),
						float32(img.y),
						float32(xMin),
						float32(yMin),
						float32(img.pixelsPerEm),
						img.image.size.Height(),
					)
				} else {
					transform = glyph.CbdtTransform(float32(img.x), float32(img.y), float32(img.pixelsPerEm))
				}

				group := emptyGroup()
				group.transform = transform
				image := img.image
				group.children = append(group.children, &image)
				group.calculateBoundingBoxes()

				newChildren = append(newChildren, group)
			} else {
				outline, ok := cache.fontdbOutline(glyph.Font, glyph.ID, variations)

				if ok {
					if outline, ok := outline.Clone().Transform(glyph.OutlineTransform()); ok {
						spanBuilder.PushPath(outline)
					}
				}
			}
		}

		pushOutlinePaths(
			span,
			&spanBuilder,
			&newChildren,
			renderingMode,
			absTransform,
		)

		if span.LineThrough != nil {
			p := clonePath(span.LineThrough)
			p.renderingMode = renderingMode
			newChildren = append(newChildren, p)
		}
	}

	group := emptyGroup()
	group.id = text.id

	group.children = append(group.children, newChildren...)

	group.calculateBoundingBoxes()
	groupStrokeBBox, ok := group.StrokeBoundingBox()
	if !ok {
		return nil, NonZeroRect{}, false
	}
	strokeBBox, ok := groupStrokeBBox.ToNonZeroRect()
	if !ok {
		return nil, NonZeroRect{}, false
	}
	return group, strokeBBox, true
}

// cloneNode is the derived Clone for Node in usvg src/tree/mod.rs: a copy of
// the node, with a copy of each fill and stroke, as cloneGroup makes it.
func cloneNode(node Node) Node {
	g := Group{children: []Node{node}}
	return g.cloneGroup().children[0]
}

// outlinePathBuilder is PathBuilder in usvg src/text/flatten.rs, the pen that
// draws an outline into a path.
//
// Port: skrifa draws an outline with the calls of the trait OutlinePen.
// go-text gives the segments of the outline, which drawOutline passes to
// this pen.
type outlinePathBuilder struct {
	builder *path.PathBuilder
}

// moveTo is OutlinePen::move_to for PathBuilder in usvg src/text/flatten.rs.
func (b *outlinePathBuilder) moveTo(x, y float32) {
	b.builder.MoveTo(x, y)
}

// lineTo is OutlinePen::line_to for PathBuilder in usvg src/text/flatten.rs.
func (b *outlinePathBuilder) lineTo(x, y float32) {
	b.builder.LineTo(x, y)
}

// quadTo is OutlinePen::quad_to for PathBuilder in usvg src/text/flatten.rs.
func (b *outlinePathBuilder) quadTo(cx0, cy0, x, y float32) {
	b.builder.QuadTo(cx0, cy0, x, y)
}

// curveTo is OutlinePen::curve_to for PathBuilder in usvg
// src/text/flatten.rs.
func (b *outlinePathBuilder) curveTo(cx0, cy0, cx1, cy1, x, y float32) {
	b.builder.CubicTo(cx0, cy0, cx1, cy1, x, y)
}

// close is OutlinePen::close for PathBuilder in usvg src/text/flatten.rs.
func (b *outlinePathBuilder) close() {
	b.builder.Close()
}

// outlinePen is the trait skrifa::outline::OutlinePen, which usvg
// implements.
type outlinePen interface {
	moveTo(x, y float32)
	lineTo(x, y float32)
	quadTo(cx0, cy0, x, y float32)
	curveTo(cx0, cy0, cx1, cy1, x, y float32)
	close()
}

// drawOutline is OutlineGlyph::draw of skrifa with unhinted settings, which
// usvg calls. It draws the outline of glyph of face, a face of pf, into pen,
// and returns false when the face has no outline for the glyph.
//
// Port: go-text gives the segments of each contour without its close. A
// contour ends where the next one starts, so pen.close is called before each
// move and after the last segment, as skrifa calls it at the end of each
// contour. For a TrueType outline, skrifa starts a contour whose first point
// is off the curve at its last point, or between its last and first points,
// and draws no line back to the start, where go-text starts at the next point
// on the curve and draws a line back. skrifaContour moves the segments of
// go-text to the start of skrifa, so that the paths are the same.
func drawOutline(pf *parsedFace, face *font.Face, glyph GlyphID, pen outlinePen) bool {
	outline, ok := face.GlyphDataOutline(font.GID(glyph))
	if !ok {
		return false
	}
	segments := outline.Segments
	if !pf.hasCFF() {
		if flags, ok := pf.contourFlags(glyph); ok {
			segments = skrifaSegments(segments, flags)
		}
	}
	for i, seg := range segments {
		a := seg.Args
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			if i != 0 {
				pen.close()
			}
			pen.moveTo(a[0].X, a[0].Y)
		case ot.SegmentOpLineTo:
			pen.lineTo(a[0].X, a[0].Y)
		case ot.SegmentOpQuadTo:
			pen.quadTo(a[0].X, a[0].Y, a[1].X, a[1].Y)
		case ot.SegmentOpCubeTo:
			pen.curveTo(a[0].X, a[0].Y, a[1].X, a[1].Y, a[2].X, a[2].Y)
		}
	}
	if len(segments) != 0 {
		pen.close()
	}
	return true
}

// hasCFF reports whether the face has a table CFF or CFF2, which go-text
// reads before the table glyf.
func (f *parsedFace) hasCFF() bool {
	return f.ld.HasTable(ot.MustNewTag("CFF ")) || f.ld.HasTable(ot.MustNewTag("CFF2"))
}

// contourFlags returns, for each contour of the TrueType outline of glyph in
// the order of go-text, whether its first and its last points are on the
// curve. It returns false when the face has no table glyf.
func (f *parsedFace) contourFlags(glyph GlyphID) ([][2]bool, bool) {
	f.glyfOnce.Do(func() {
		head, loca, glyf := f.rawTable("head"), f.rawTable("loca"), f.rawTable("glyf")
		maxp := f.rawTable("maxp")
		if len(head) < headMinSize || loca == nil || glyf == nil || len(maxp) < 6 {
			return
		}
		numGlyphs := int(binary.BigEndian.Uint16(maxp[4:]))
		offsets, err := tables.ParseLoca(loca, numGlyphs, int16(binary.BigEndian.Uint16(head[50:])) == 1)
		if err != nil || len(offsets) == 0 {
			return
		}
		f.glyf, _ = tables.ParseGlyf(glyf, offsets)
	})
	if f.glyf == nil {
		return nil, false
	}
	var flags [][2]bool
	seen := map[tables.GlyphID]bool{}
	var walk func(gid tables.GlyphID, depth int)
	walk = func(gid tables.GlyphID, depth int) {
		// The limit of nesting of go-text.
		if depth > 20 || int(gid) >= len(f.glyf) {
			return
		}
		switch data := f.glyf[gid].Data.(type) {
		case tables.SimpleGlyph:
			start := 0
			for _, end := range data.EndPtsOfContours {
				if int(end) >= len(data.Points) || int(end) < start {
					return
				}
				flags = append(flags, [2]bool{data.Points[start].Flag&1 != 0, data.Points[end].Flag&1 != 0})
				start = int(end) + 1
			}
		case tables.CompositeGlyph:
			for _, item := range data.Glyphs {
				if seen[item.GlyphIndex] {
					continue
				}
				seen[item.GlyphIndex] = true
				walk(item.GlyphIndex, depth+1)
				delete(seen, item.GlyphIndex)
			}
		}
	}
	walk(tables.GlyphID(glyph), 0)
	return flags, true
}

// skrifaSegments moves the segments of each contour of a TrueType outline of
// go-text to the start that skrifa takes, and drops the line back to the
// start that go-text draws and skrifa does not. flags holds, for each
// contour, whether its first and its last points are on the curve. When the
// contours do not match the flags, segments is returned as it is.
func skrifaSegments(segments []font.Segment, flags [][2]bool) []font.Segment {
	var contours [][]font.Segment
	for i, seg := range segments {
		if seg.Op == ot.SegmentOpMoveTo || i == 0 {
			contours = append(contours, nil)
		}
		contours[len(contours)-1] = append(contours[len(contours)-1], seg)
	}
	if len(contours) != len(flags) {
		return segments
	}
	end := func(s font.Segment) font.SegmentPoint {
		switch s.Op {
		case ot.SegmentOpQuadTo:
			return s.Args[1]
		case ot.SegmentOpCubeTo:
			return s.Args[2]
		}
		return s.Args[0]
	}
	out := make([]font.Segment, 0, len(segments))
	for i, c := range contours {
		firstOn, lastOn := flags[i][0], flags[i][1]
		n := len(c)
		switch {
		case n < 2 || c[0].Op != ot.SegmentOpMoveTo:
			out = append(out, c...)
		case firstOn:
			// The line back to the start, when the last point is on the
			// curve too.
			if lastOn && c[n-1].Op == ot.SegmentOpLineTo && c[n-1].Args[0] == c[0].Args[0] {
				c = c[:n-1]
			}
			out = append(out, c...)
		case lastOn && n >= 3:
			// skrifa starts at the last point, which ends the segment
			// before the closing curve of go-text.
			start := end(c[n-2])
			out = append(out, font.Segment{Op: ot.SegmentOpMoveTo, Args: [3]font.SegmentPoint{start}})
			out = append(out, c[n-1])
			out = append(out, c[1:n-1]...)
		case !lastOn && n >= 3:
			// skrifa starts between the last and the first points, which
			// ends the curve before the last curve of go-text.
			start := end(c[n-2])
			out = append(out, font.Segment{Op: ot.SegmentOpMoveTo, Args: [3]font.SegmentPoint{start}})
			out = append(out, c[n-1])
			out = append(out, c[1:n-1]...)
		default:
			out = append(out, c...)
		}
	}
	return out
}

// glyphBoundingBox is BoundingBox<i16> of read-fonts.
type glyphBoundingBox struct {
	xMin int16
	yMin int16
	xMax int16
	yMax int16
}

// bitmapImage is BitmapImage in usvg src/text/flatten.rs.
//
// Port: a nil glyphBBox is None.
type bitmapImage struct {
	image       Image
	x           int16
	y           int16
	pixelsPerEm uint16
	glyphBBox   *glyphBoundingBox
	isSbix      bool
}

// databaseOutline is DatabaseExt::outline for Database in usvg
// src/text/flatten.rs.
//
// Port: skrifa outlines a glyph at the location of the variations. go-text
// does the same with a face whose coordinates are the variations.
func databaseOutline(fontdb *FontDatabase, id FontID, glyphID GlyphID, variations []FontVariation) (*path.Path, bool) {
	pf, ok := fontdb.withFaceData(id)
	if !ok {
		return nil, false
	}
	face := font.NewFace(pf.font)

	// An empty variation list resolves to the default value of every
	// variation axis, which is what we want for non-variable fonts and
	// for variable fonts used without variations.
	face.SetVariations(toGoTextVariations(variations))

	builder := &outlinePathBuilder{builder: path.NewPathBuilder()}
	if !drawOutline(pf, face, glyphID, builder) {
		return nil, false
	}

	return builder.builder.Finish()
}

// databaseHasOpszAxis is DatabaseExt::has_opsz_axis for Database in usvg
// src/text/flatten.rs.
func databaseHasOpszAxis(fontdb *FontDatabase, id FontID) bool {
	pf, ok := fontdb.withFaceData(id)
	if !ok {
		return false
	}
	return hasAxis(pf, opsz)
}

// bitmapGlyph is BitmapGlyph of skrifa src/bitmap.rs, with the fields that
// usvg reads, for a glyph with PNG data.
type bitmapGlyph struct {
	png           []byte
	innerBearingX float32
	innerBearingY float32
	ppemY         float32
	ppemX         float32
	width         uint32
	height        uint32
}

// bitmapStrike is a strike of BitmapStrikes of skrifa src/bitmap.rs: its
// size and the lookup of a glyph in it.
type bitmapStrike struct {
	ppem float32
	get  func(glyphID GlyphID) (bitmapGlyph, bool)
}

// bitmapStrikes is BitmapStrikes::new of skrifa src/bitmap.rs, which usvg
// calls with font.bitmap_strikes. It returns the strikes of the first of
// the tables sbix and CBDT that the face has, and whether that table is
// sbix.
//
// Port: skrifa also reads the table EBDT, whose glyphs are masks with no PNG
// data, which usvg does not draw. The strikes of EBDT are left out, and
// usvg draws the outline of such a glyph, as it does with skrifa.
func bitmapStrikes(pf *parsedFace) ([]bitmapStrike, bool) {
	numGlyphs := 0
	if maxp := pf.rawTable("maxp"); len(maxp) >= 6 {
		numGlyphs = int(binary.BigEndian.Uint16(maxp[4:]))
	}

	if raw := pf.rawTable("sbix"); raw != nil {
		if sbix, _, err := tables.ParseSbix(raw, numGlyphs); err == nil {
			strikes := make([]bitmapStrike, 0, len(sbix.Strikes))
			for _, st := range sbix.Strikes {
				ppem := float32(st.Ppem)
				strikes = append(strikes, bitmapStrike{
					ppem: ppem,
					get: func(glyphID GlyphID) (bitmapGlyph, bool) {
						if int(glyphID) >= len(st.GlyphDatas) {
							return bitmapGlyph{}, false
						}
						g := st.GlyphDatas[glyphID]
						if len(g.Data) == 0 || g.GraphicType != ot.MustNewTag("png ") {
							return bitmapGlyph{}, false
						}
						if len(g.Data) < 24 {
							return bitmapGlyph{}, false
						}
						return bitmapGlyph{
							png:           g.Data,
							innerBearingX: float32(g.OriginOffsetX),
							innerBearingY: float32(g.OriginOffsetY),
							ppemX:         ppem,
							ppemY:         ppem,
							width:         binary.BigEndian.Uint32(g.Data[16:]),
							height:        binary.BigEndian.Uint32(g.Data[20:]),
						}, true
					},
				})
			}
			return strikes, true
		}
	}

	cblcRaw, cbdt := pf.rawTable("CBLC"), pf.rawTable("CBDT")
	if cblcRaw != nil && cbdt != nil {
		if cblc, _, err := tables.ParseCBLC(cblcRaw); err == nil {
			strikes := make([]bitmapStrike, 0, len(cblc.BitmapSizes))
			for i, size := range cblc.BitmapSizes {
				// Port: go-text keeps the bit depth private. It is the byte
				// 46 of the record of the size, which starts at 8.
				var bitDepth uint8
				if off := 8 + 48*i + 46; off < len(cblcRaw) {
					bitDepth = cblcRaw[off]
				}
				subtables := cblc.IndexSubTables[i]
				strikes = append(strikes, bitmapStrike{
					ppem: float32(size.PpemY),
					get: func(glyphID GlyphID) (bitmapGlyph, bool) {
						return cbdtGlyph(size, bitDepth, subtables, cbdt, glyphID)
					},
				})
			}
			return strikes, false
		}
	}

	return nil, false
}

// cbdtGlyph is BitmapStrike::get of skrifa src/bitmap.rs for a strike of
// CBDT, with BitmapGlyph::from_bdt, for a glyph with PNG data.
func cbdtGlyph(size tables.BitmapSize, bitDepth uint8, subtables []tables.BitmapSubtable, cbdt []byte, glyphID GlyphID) (bitmapGlyph, bool) {
	gid := tables.GlyphID(glyphID)
	for _, st := range subtables {
		if gid < st.FirstGlyph || gid > st.LastGlyph {
			continue
		}
		var start, end int
		var bigMetrics *tables.BigGlyphMetrics
		base := int(st.ImageDataOffset)
		index := int(gid - st.FirstGlyph)
		switch data := st.IndexData.(type) {
		case tables.IndexData1:
			if index+1 >= len(data.SbitOffsets) {
				return bitmapGlyph{}, false
			}
			start, end = base+int(data.SbitOffsets[index]), base+int(data.SbitOffsets[index+1])
		case tables.IndexData3:
			if index+1 >= len(data.SbitOffsets) {
				return bitmapGlyph{}, false
			}
			start, end = base+int(data.SbitOffsets[index]), base+int(data.SbitOffsets[index+1])
		case tables.IndexData2:
			start = base + int(data.ImageSize)*index
			end = start + int(data.ImageSize)
			bigMetrics = &data.BigMetrics
		case tables.IndexData4:
			i := slices.IndexFunc(data.GlyphArray, func(p tables.GlyphIdOffsetPair) bool { return p.GlyphID == gid })
			if i < 0 || i+1 >= len(data.GlyphArray) {
				return bitmapGlyph{}, false
			}
			start, end = base+int(data.GlyphArray[i].SbitOffset), base+int(data.GlyphArray[i+1].SbitOffset)
		case tables.IndexData5:
			i := slices.Index(data.GlyphIdArray, gid)
			if i < 0 {
				return bitmapGlyph{}, false
			}
			start = base + int(data.ImageSize)*i
			end = start + int(data.ImageSize)
			bigMetrics = &data.BigMetrics
		default:
			return bitmapGlyph{}, false
		}
		if start < 0 || end > len(cbdt) || start > end {
			return bitmapGlyph{}, false
		}
		b := cbdt[start:end]

		var metrics tables.SmallGlyphMetrics
		var png []byte
		readPNG := func(b []byte) ([]byte, bool) {
			if len(b) < 4 {
				return nil, false
			}
			n := int(binary.BigEndian.Uint32(b))
			if len(b) < 4+n {
				return nil, false
			}
			return b[4 : 4+n], true
		}
		var ok bool
		switch st.ImageFormat {
		case 17:
			if len(b) < 5 {
				return bitmapGlyph{}, false
			}
			metrics = tables.SmallGlyphMetrics{
				Height: b[0], Width: b[1], BearingX: int8(b[2]), BearingY: int8(b[3]), Advance: b[4],
			}
			png, ok = readPNG(b[5:])
		case 18:
			if len(b) < 8 {
				return bitmapGlyph{}, false
			}
			metrics = tables.SmallGlyphMetrics{
				Height: b[0], Width: b[1], BearingX: int8(b[2]), BearingY: int8(b[3]), Advance: b[4],
			}
			png, ok = readPNG(b[8:])
		case 19:
			if bigMetrics == nil {
				return bitmapGlyph{}, false
			}
			metrics = bigMetrics.SmallGlyphMetrics
			png, ok = readPNG(b)
		default:
			// Port: the other formats hold BGRA or mask data, which usvg
			// does not draw.
			return bitmapGlyph{}, false
		}
		if !ok || bitDepth != 32 {
			return bitmapGlyph{}, false
		}
		return bitmapGlyph{
			png:           png,
			innerBearingX: float32(metrics.BearingX),
			innerBearingY: float32(metrics.BearingY),
			ppemX:         float32(size.PpemX),
			ppemY:         float32(size.PpemY),
			width:         uint32(metrics.Width),
			height:        uint32(metrics.Height),
		}, true
	}
	return bitmapGlyph{}, false
}

// glyphForSize is BitmapStrikes::glyph_for_size of skrifa src/bitmap.rs,
// for Size::unscaled(), which asks for the largest image available.
func glyphForSize(strikes []bitmapStrike, glyphID GlyphID) (bitmapGlyph, bool) {
	size := float32(3.40282346638528859811704183484516925440e+38) // f32::MAX
	var best bitmapGlyph
	hasBest := false
	for _, entry := range strikes {
		entrySize := entry.ppem
		if hasBest {
			bestSize := best.ppemY
			if (entrySize >= size && entrySize < bestSize) ||
				(bestSize < size && entrySize > bestSize) {
				if g, ok := entry.get(glyphID); ok {
					best = g
				}
			}
		} else {
			best, hasBest = entry.get(glyphID)
		}
	}
	return best, hasBest
}

// controlBoundsPen is ControlBoundsPen of skrifa: the bounding box of the
// points of an outline, with its control points.
type controlBoundsPen struct {
	box glyphBoundingBox32
	has bool
}

// glyphBoundingBox32 is BoundingBox<f32> of read-fonts.
type glyphBoundingBox32 struct {
	xMin, yMin, xMax, yMax float32
}

// add adds a point to the bounding box.
func (p *controlBoundsPen) add(x, y float32) {
	if !p.has {
		p.box = glyphBoundingBox32{x, y, x, y}
		p.has = true
		return
	}
	p.box.xMin = minF32(p.box.xMin, x)
	p.box.yMin = minF32(p.box.yMin, y)
	p.box.xMax = maxF32(p.box.xMax, x)
	p.box.yMax = maxF32(p.box.yMax, y)
}

func (p *controlBoundsPen) moveTo(x, y float32)           { p.add(x, y) }
func (p *controlBoundsPen) lineTo(x, y float32)           { p.add(x, y) }
func (p *controlBoundsPen) quadTo(cx0, cy0, x, y float32) { p.add(cx0, cy0); p.add(x, y) }
func (p *controlBoundsPen) curveTo(cx0, cy0, cx1, cy1, x, y float32) {
	p.add(cx0, cy0)
	p.add(cx1, cy1)
	p.add(x, y)
}
func (p *controlBoundsPen) close() {}

// glyphBounds is GlyphMetrics::bounds of skrifa src/metrics.rs, for an
// unscaled size and the default location, which usvg calls with
// font.glyph_metrics. A font with a table gvar, CFF or CFF2 takes the
// bounding box of the points of the outline. Another font takes the box of
// the glyph in the table glyf, and an empty glyph has an empty box. It
// returns false for no bounding box.
func glyphBounds(pf *parsedFace, glyphID GlyphID) (glyphBoundingBox32, bool) {
	if pf.ld.HasTable(ot.MustNewTag("gvar")) || pf.hasCFF() {
		var pen controlBoundsPen
		if !drawOutline(pf, font.NewFace(pf.font), glyphID, &pen) {
			return glyphBoundingBox32{}, false
		}
		return pen.box, pen.has
	}
	head, loca, glyf := pf.rawTable("head"), pf.rawTable("loca"), pf.rawTable("glyf")
	if len(head) < headMinSize || loca == nil || glyf == nil {
		return glyphBoundingBox32{}, false
	}
	var start, end int
	if int16(binary.BigEndian.Uint16(head[50:])) == 0 {
		i := int(glyphID) * 2
		if i+4 > len(loca) {
			return glyphBoundingBox32{}, false
		}
		start = int(binary.BigEndian.Uint16(loca[i:])) * 2
		end = int(binary.BigEndian.Uint16(loca[i+2:])) * 2
	} else {
		i := int(glyphID) * 4
		if i+8 > len(loca) {
			return glyphBoundingBox32{}, false
		}
		start = int(binary.BigEndian.Uint32(loca[i:]))
		end = int(binary.BigEndian.Uint32(loca[i+4:]))
	}
	if end == start {
		// Empty glyphs have an empty bounding box
		return glyphBoundingBox32{}, true
	}
	if end < start || start+10 > len(glyf) {
		return glyphBoundingBox32{}, false
	}
	g := glyf[start:]
	i16 := func(off int) float32 { return float32(int16(binary.BigEndian.Uint16(g[off:]))) }
	return glyphBoundingBox32{i16(2), i16(4), i16(6), i16(8)}, true
}

// databaseRaster is DatabaseExt::raster for Database in usvg
// src/text/flatten.rs.
func databaseRaster(fontdb *FontDatabase, id FontID, glyphID GlyphID) (*bitmapImage, bool) {
	pf, ok := fontdb.withFaceData(id)
	if !ok {
		return nil, false
	}
	strikes, isSbix := bitmapStrikes(pf)

	// We set size to unscaled to get the largest image available
	image, ok := glyphForSize(strikes, glyphID)
	if !ok {
		return nil, false
	}

	var boundingBox *glyphBoundingBox
	if bbox, ok := glyphBounds(pf, glyphID); ok {
		boundingBox = &glyphBoundingBox{
			xMin: asI16(bbox.xMin),
			yMin: asI16(bbox.yMin),
			xMax: asI16(bbox.xMax),
			yMax: asI16(bbox.yMax),
		}
	}

	size, ok := path.SizeFromWH(float32(image.width), float32(image.height))
	if !ok {
		return nil, false
	}
	absBoundingBox, ok := path.NonZeroRectFromXYWH(
		0.0,
		0.0,
		float32(image.width),
		float32(image.height),
	)
	if !ok {
		return nil, false
	}

	return &bitmapImage{
		image: Image{
			id:             "",
			visible:        true,
			size:           size,
			renderingMode:  ImageRenderingOptimizeQuality,
			kind:           ImageKindPNG(slices.Clone(image.png)),
			absTransform:   path.DefaultTransform(),
			absBoundingBox: absBoundingBox,
		},
		x:           asI16(image.innerBearingX),
		y:           asI16(image.innerBearingY),
		pixelsPerEm: wideTruncU16(image.ppemX),
		glyphBBox:   boundingBox,
		isSbix:      isSbix,
	}, true
}

// wideTruncU16 is the Rust cast "x as u16" of an f32.
func wideTruncU16(x float32) uint16 {
	switch {
	case x != x || x <= 0:
		return 0
	case x >= 65535:
		return 65535
	}
	return uint16(x)
}

// databaseSvg is DatabaseExt::svg for Database in usvg src/text/flatten.rs.
//
// Port: opt holds the logger (D20).
func databaseSvg(fontdb *FontDatabase, id FontID, glyphID GlyphID, opt *options) (Node, bool) {
	// SEE: https://docs.rs/read-fonts/latest/read_fonts/tables/svg/type.Svg.html

	// TODO: Technically not 100% accurate because the SVG format in a OTF font
	// is actually a subset/superset of a normal SVG, but it seems to work fine
	// for Twitter Color Emoji, so might as well use what we already have.

	// TODO: Glyph records can contain the data for multiple glyphs. We should
	// add a cache so we don't need to reparse the data every time.
	pf, ok := fontdb.withFaceData(id)
	if !ok {
		return nil, false
	}
	raw := pf.rawTable("SVG ")
	if raw == nil {
		return nil, false
	}
	svgTable, _, err := tables.ParseSVG(raw)
	if err != nil {
		return nil, false
	}
	documentList := svgTable.SVGDocumentList
	var docRecord *tables.SVGDocumentRecord
	for i := range documentList.DocumentRecords {
		r := &documentList.DocumentRecords[i]
		if uint32(r.StartGlyphID) <= uint32(glyphID) && uint32(glyphID) <= uint32(r.EndGlyphID) {
			docRecord = r
			break
		}
	}
	if docRecord == nil {
		return nil, false
	}
	start := int(docRecord.SvgDocOffset)
	end := start + int(docRecord.SvgDocLength)
	if end > len(documentList.SVGRawData) || start > end {
		return nil, false
	}
	imageData := documentList.SVGRawData[start:end]
	tree, err := fromData(imageData, newOptions())
	if err != nil {
		return nil, false
	}

	// Twitter Color Emoji seems to always have one SVG record per glyph,
	// while Noto Color Emoji sometimes contains multiple ones. It's kind of hacky,
	// but the best we have for now.
	var node Node
	if docRecord.StartGlyphID == docRecord.EndGlyphID {
		root := tree.root
		node = &root
	} else {
		n, ok := tree.NodeByID("glyph" + strconv.FormatUint(uint64(glyphID), 10))
		if !ok {
			opt.warnf("Failed to find SVG glyph node for glyph %d", glyphID)
			return nil, false
		}
		node = n
	}

	return node, true
}

// databaseColr is DatabaseExt::colr for Database in usvg
// src/text/flatten.rs.
//
// Port: skrifa paints a COLR glyph with ColorGlyph::paint, which
// paintColorGlyph of usvg_text_colr_paint.go does on the tables of go-text.
// opt holds the logger (D20).
func databaseColr(fontdb *FontDatabase, id FontID, glyphID GlyphID, variations []FontVariation, opt *options) (*Tree, bool) {
	pf, ok := fontdb.withFaceData(id)
	if !ok {
		return nil, false
	}
	if pf.font.COLR == nil {
		return nil, false
	}

	face := font.NewFace(pf.font)
	face.SetVariations(toGoTextVariations(variations))

	svg := xmlwriter.New()

	svg.StartElement("svg")
	svg.WriteAttribute("xmlns", "http://www.w3.org/2000/svg")
	svg.WriteAttribute("xmlns:xlink", "http://www.w3.org/1999/xlink")

	pathBuf := make([]byte, 0, 256)
	gradientIndex := 1
	clipPathIndex := 1

	svg.StartElement("g")

	glyphPainter := &glyphPainter{
		pf:               pf,
		face:             face,
		svg:              svg,
		pathBuf:          &pathBuf,
		gradientIndex:    gradientIndex,
		clipPathIndex:    clipPathIndex,
		foregroundColor:  svgtypes.NewColorRGBA(0, 0, 0, 255),
		transform:        identityColrTransform(),
		outlineTransform: identityColrTransform(),
		transformsStack:  []colrTransform{identityColrTransform()},
		clipStack:        nil,
		opt:              opt,
	}

	if !paintColorGlyph(pf.font, face.Coords(), glyphID, glyphPainter) {
		return nil, false
	}
	svg.EndElement()

	tree, err := fromData([]byte(svg.EndDocument()), newOptions())
	if err != nil {
		return nil, false
	}
	return tree, true
}
