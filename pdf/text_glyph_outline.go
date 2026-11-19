// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"encoding/binary"

	tsfont "github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"

	"github.com/xo/okage/path"
)

// This file ports krilla src/text/glyph/outline.rs. skrifa draws an outline
// with the calls of the trait OutlinePen. typesetting gives the segments of
// the outline, which drawSkrifaOutline passes to the pen in the order of
// skrifa, as drawOutline of svg does for usvg.

// glyphPath is glyph_path in krilla src/text/glyph/outline.rs. It draws the
// outline of a glyph at the location of the font, unhinted and unscaled. It
// returns false where upstream returns None: for a glyph without an outline,
// and for an outline without a segment.
func glyphPath(font *Font, glyph GlyphID) (*Path, bool) {
	b := newOutlineBuilder()
	font.drawSkrifaOutline(glyph, b)
	return b.finish()
}

// drawOutlineGlyph is draw_glyph in krilla src/text/glyph/outline.rs.
//
// Draw an outline-based glyph on a surface.
func drawOutlineGlyph(font *Font, glyph GlyphID, baseTransform Transform, surface *Surface) bool {
	p, ok := glyphPath(font, glyph)
	if !ok {
		return false
	}
	p, ok = p.Transform(baseTransform)
	if !ok {
		return false
	}
	surface.DrawPath(p)
	return true
}

// outlineBuilder is OutlineBuilder in krilla src/text/glyph/outline.rs.
//
// A wrapper struct for implementing the OutlinePen trait.
type outlineBuilder struct {
	b *path.PathBuilder
}

// newOutlineBuilder is OutlineBuilder::new in krilla
// src/text/glyph/outline.rs.
func newOutlineBuilder() *outlineBuilder {
	return &outlineBuilder{b: path.NewPathBuilder()}
}

// finish is OutlineBuilder::finish in krilla src/text/glyph/outline.rs.
func (o *outlineBuilder) finish() (*Path, bool) {
	return o.b.Finish()
}

// moveTo is OutlinePen::move_to for OutlineBuilder.
func (o *outlineBuilder) moveTo(x, y float32) { o.b.MoveTo(x, y) }

// lineTo is OutlinePen::line_to for OutlineBuilder.
func (o *outlineBuilder) lineTo(x, y float32) { o.b.LineTo(x, y) }

// quadTo is OutlinePen::quad_to for OutlineBuilder.
func (o *outlineBuilder) quadTo(cx0, cy0, x, y float32) { o.b.QuadTo(cx0, cy0, x, y) }

// curveTo is OutlinePen::curve_to for OutlineBuilder.
func (o *outlineBuilder) curveTo(cx0, cy0, cx1, cy1, x, y float32) {
	o.b.CubicTo(cx0, cy0, cx1, cy1, x, y)
}

// close is OutlinePen::close for OutlineBuilder.
func (o *outlineBuilder) close() { o.b.Close() }

// drawSkrifaOutline is OutlineGlyph::draw of skrifa with unhinted settings
// at the size unscaled and the location of the font, as krilla calls it. It
// draws the outline of the glyph into the builder, and returns false when
// the font has no outline for the glyph.
//
// typesetting gives the segments of each contour without its close. A
// contour ends where the next one starts, so close is called before each
// move and after the last segment, as skrifa calls it at the end of each
// contour. For a CFF or CFF2 outline, nopFilterPen drops the degenerate
// commands, as skrifa does. For a TrueType outline, skrifa starts a contour
// whose first point is off the curve at its last point, or between its last
// and first points, and draws no line back to the start, where typesetting
// starts at the next point on the curve and draws a line back.
// skrifaSegments moves the segments of typesetting to the start of skrifa,
// so that the paths are the same. drawOutline of svg does the same for
// usvg.
func (f *Font) drawSkrifaOutline(glyph GlyphID, pen *outlineBuilder) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if uint64(glyph) > 0xFFFF {
		return false
	}
	outline, ok := f.face.GlyphDataOutline(tsfont.GID(glyph))
	if !ok {
		return false
	}
	segments := outline.Segments
	if f.ld.HasTable(ot.MustNewTag("CFF ")) || f.ld.HasTable(ot.MustNewTag("CFF2")) {
		nop := &nopFilterPen{pen: pen}
		for i, seg := range segments {
			a := seg.Args
			switch seg.Op {
			case ot.SegmentOpMoveTo:
				if i != 0 {
					nop.close()
				}
				nop.moveTo(a[0].X, a[0].Y)
			case ot.SegmentOpLineTo:
				nop.lineTo(a[0].X, a[0].Y)
			case ot.SegmentOpQuadTo:
				// A CFF outline has no quadratic curve.
				nop.lineTo(a[1].X, a[1].Y)
			case ot.SegmentOpCubeTo:
				nop.curveTo(a[0].X, a[0].Y, a[1].X, a[1].Y, a[2].X, a[2].Y)
			}
		}
		nop.close()
		return true
	}
	if flags, ok := f.contourFlags(glyph); ok {
		segments = skrifaSegments(segments, flags)
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

// pendingKind is the case of PendingElement of read-fonts src/ps/cs.rs.
// pendingNone is no pending element.
type pendingKind uint8

// The cases of PendingElement.
const (
	pendingNone pendingKind = iota
	pendingMove
	pendingLine
	pendingCurve
)

// nopFilterPen is NopFilterSink of read-fonts 0.41 src/ps/cs.rs, which
// skrifa puts between the charstrings of a CFF or CFF2 font and the pen. It
// suppresses degenerate move and line commands: a move that a close
// follows, a line to the current point, and a closing line that ends at the
// start of its contour. skrifa compares the points as Fixed values, and the
// port compares the float32 values of typesetting.
type nopFilterPen struct {
	pen      *outlineBuilder
	isOpen   bool
	start    [2]float32
	hasStart bool
	pending  pendingKind
	p        [6]float32
}

// target is PendingElement::target_point.
func (n *nopFilterPen) target() [2]float32 {
	if n.pending == pendingCurve {
		return [2]float32{n.p[4], n.p[5]}
	}
	return [2]float32{n.p[0], n.p[1]}
}

// flushPending is NopFilterSink::flush_pending.
func (n *nopFilterPen) flushPending(forClose bool) {
	p := n.p
	switch n.pending {
	case pendingMove:
		if !forClose {
			n.isOpen = true
			n.pen.moveTo(p[0], p[1])
			n.start, n.hasStart = [2]float32{p[0], p[1]}, true
		}
	case pendingLine:
		if !forClose || !n.hasStart || n.start != [2]float32{p[0], p[1]} {
			n.pen.lineTo(p[0], p[1])
		}
	case pendingCurve:
		n.pen.curveTo(p[0], p[1], p[2], p[3], p[4], p[5])
	}
	n.pending = pendingNone
}

// moveTo is CommandSink::move_to for NopFilterSink.
func (n *nopFilterPen) moveTo(x, y float32) {
	n.pending, n.p = pendingMove, [6]float32{x, y}
}

// lineTo is CommandSink::line_to for NopFilterSink.
func (n *nopFilterPen) lineTo(x, y float32) {
	// Omit the line if we're already at the given position.
	if n.pending != pendingNone && n.target() == [2]float32{x, y} {
		return
	}
	n.flushPending(false)
	n.pending, n.p = pendingLine, [6]float32{x, y}
}

// curveTo is CommandSink::curve_to for NopFilterSink.
func (n *nopFilterPen) curveTo(cx1, cy1, cx2, cy2, x, y float32) {
	n.flushPending(false)
	n.pending, n.p = pendingCurve, [6]float32{cx1, cy1, cx2, cy2, x, y}
}

// close is CommandSink::close for NopFilterSink.
func (n *nopFilterPen) close() {
	n.flushPending(true)
	if n.isOpen {
		n.pen.close()
		n.isOpen = false
	}
}

// contourFlags returns, for each contour of the TrueType outline of glyph in
// the order of typesetting, whether its first and its last points are on
// the curve. It returns false when the font has no table glyf. The caller
// holds f.mu.
func (f *Font) contourFlags(glyph GlyphID) ([][2]bool, bool) {
	if !f.glyfDone {
		f.glyfDone = true
		head, loca, glyf, maxp := f.table("head"), f.table("loca"), f.table("glyf"), f.table("maxp")
		if len(head) >= 54 && loca != nil && glyf != nil && len(maxp) >= 6 {
			numGlyphs := int(binary.BigEndian.Uint16(maxp[4:]))
			offsets, err := tables.ParseLoca(loca, numGlyphs, int16(binary.BigEndian.Uint16(head[50:])) == 1)
			if err == nil && len(offsets) != 0 {
				f.glyf, _ = tables.ParseGlyf(glyf, offsets)
			}
		}
	}
	if f.glyf == nil {
		return nil, false
	}
	var flags [][2]bool
	seen := map[tables.GlyphID]bool{}
	var walk func(gid tables.GlyphID, depth int)
	walk = func(gid tables.GlyphID, depth int) {
		// The limit of nesting of typesetting.
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

// skrifaSegments moves the segments of each contour of a TrueType outline
// of typesetting to the start that skrifa takes, and drops the line back to
// the start that typesetting draws and skrifa does not. flags holds, for
// each contour, whether its first and its last points are on the curve.
// When the contours do not match the flags, segments is returned as it is.
func skrifaSegments(segments []tsfont.Segment, flags [][2]bool) []tsfont.Segment {
	var contours [][]tsfont.Segment
	for i, seg := range segments {
		if seg.Op == ot.SegmentOpMoveTo || i == 0 {
			contours = append(contours, nil)
		}
		contours[len(contours)-1] = append(contours[len(contours)-1], seg)
	}
	if len(contours) != len(flags) {
		return segments
	}
	end := func(s tsfont.Segment) tsfont.SegmentPoint {
		switch s.Op {
		case ot.SegmentOpQuadTo:
			return s.Args[1]
		case ot.SegmentOpCubeTo:
			return s.Args[2]
		}
		return s.Args[0]
	}
	out := make([]tsfont.Segment, 0, len(segments))
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
		case n >= 3:
			// skrifa starts at the last point when it is on the curve, or
			// between the last and the first points. Both end the segment
			// before the closing curve of typesetting. skrifa draws no line
			// back to that start, which the close draws.
			start := end(c[n-2])
			out = append(out, tsfont.Segment{Op: ot.SegmentOpMoveTo, Args: [3]tsfont.SegmentPoint{start}})
			out = append(out, c[n-1])
			rest := c[1 : n-1]
			if k := len(rest); k != 0 && rest[k-1].Op == ot.SegmentOpLineTo && rest[k-1].Args[0] == start {
				rest = rest[:k-1]
			}
			out = append(out, rest...)
		default:
			out = append(out, c...)
		}
	}
	return out
}
