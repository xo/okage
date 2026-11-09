// Copyright 2026 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"encoding/binary"
	"math"
	"slices"
	"strconv"

	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
	"github.com/go-text/typesetting/harfbuzz"

	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/internal/xmlwriter"
	"github.com/xo/okage/path"
)

// This file ports usvg src/text/cache.rs.
//
// Port: go-text takes the place of skrifa (D9). skrifa::Font becomes the
// face that go-text parses, and a font instance becomes a face of go-text
// whose coordinates are the variations. The outline of a glyph comes from
// font.Face.GlyphDataOutline, and the tables CBLC, CBDT, sbix, SVG and COLR
// are read with the package tables of go-text, by the rules of skrifa.

// fontCache is FontCache in usvg src/text/cache.rs.
//
// Port: opt holds the logger (D20).
type fontCache struct {
	// This fontdb is initialized from [`Options::fontdb`](crate::Options::fontdb) and then
	// populated over the course of conversion.
	fontdb   *FontDatabase
	resolved fontsCache
	// `None` marks faces that failed to load, so they are not read again.
	//
	// Port: a nil value is None.
	loaded map[FontID]*loadedFont
	opt    *options
}

// loadedFont is LoadedFont in usvg src/text/cache.rs.
//
// Port: the parsed face of go-text takes the place of skrifa::Font. A nil
// value of svg or raster is None. skrifa reads the table SVG and the bitmap
// strikes on demand, and Go does the same with svgChecked and strikesLoaded.
type loadedFont struct {
	pf        *parsedFace
	instances map[string]*fontInstance
	svg       map[GlyphID]Node
	raster    map[GlyphID]*bitmapImage

	svgChecked    bool
	hasSVG        bool
	strikesLoaded bool
	strikes       []bitmapStrike
}

// fontInstance is FontInstance in usvg src/text/cache.rs.
//
// Port: skrifa::Font at a location becomes a face of go-text whose
// coordinates are the variations. The shaper of go-text is a harfbuzz font,
// which holds the accelerators of the lookups, so it is made on the first
// shaping only. go-text keeps its shape plans in a buffer, keyed by the
// face, so plans is the buffer that shapes each run with the font. A nil
// value of outlines or colr is None.
type fontInstance struct {
	face     *font.Face
	hbFont   *harfbuzz.Font
	plans    *harfbuzz.Buffer
	outlines map[GlyphID]*path.Path
	colr     map[GlyphID]*Tree
}

// bitmapImage is BitmapImage in usvg src/text/cache.rs.
type bitmapImage struct {
	image       Image
	x           int16
	y           int16
	pixelsPerEm uint16
	bearingX    float32
	bearingY    float32
	isSbix      bool
}

// newFontCache is FontCache::new in usvg src/text/cache.rs.
func newFontCache(fontdb *FontDatabase, opt *options) *fontCache {
	return &fontCache{
		fontdb:   fontdb,
		resolved: fontsCache{},
		loaded:   map[FontID]*loadedFont{},
		opt:      opt,
	}
}

// load is FontCache::load in usvg src/text/cache.rs.
func (c *fontCache) load(id FontID) (*loadedFont, bool) {
	l, ok := c.loaded[id]
	if !ok {
		l, _ = newLoadedFont(id, c.fontdb)
		c.loaded[id] = l
	}
	return l, l != nil
}

// outline is FontCache::outline in usvg src/text/cache.rs.
func (c *fontCache) outline(id FontID, glyph GlyphID, variations []FontVariation) (*path.Path, bool) {
	l, ok := c.load(id)
	if !ok {
		return nil, false
	}
	instance := l.instance(variations)
	p, ok := instance.outlines[glyph]
	if !ok {
		p, _ = outline(l.pf, instance.face, glyph)
		instance.outlines[glyph] = p
	}
	return p, p != nil
}

// colr is FontCache::colr in usvg src/text/cache.rs.
//
// Port: font.color_glyphs().is_empty() of skrifa reports a font without a
// COLR table, which go-text parses with the font.
func (c *fontCache) colr(id FontID, glyph GlyphID, variations []FontVariation) (*Tree, bool) {
	l, ok := c.load(id)
	if !ok {
		return nil, false
	}
	if l.pf.font.COLR == nil {
		return nil, false
	}

	instance := l.instance(variations)
	t, ok := instance.colr[glyph]
	if !ok {
		t, _ = colr(l.pf, instance.face, glyph, c.opt)
		instance.colr[glyph] = t
	}
	return t, t != nil
}

// svg is FontCache::svg in usvg src/text/cache.rs.
func (c *fontCache) svg(id FontID, glyph GlyphID) (Node, bool) {
	l, ok := c.load(id)
	if !ok {
		return nil, false
	}
	if !l.svgTableOK() {
		return nil, false
	}

	n, ok := l.svg[glyph]
	if !ok {
		n, _ = svg(l.pf, glyph, c.opt)
		l.svg[glyph] = n
	}
	return n, n != nil
}

// raster is FontCache::raster in usvg src/text/cache.rs.
func (c *fontCache) raster(id FontID, glyph GlyphID) (*bitmapImage, bool) {
	l, ok := c.load(id)
	if !ok {
		return nil, false
	}
	if len(l.bitmapStrikes()) == 0 {
		return nil, false
	}

	img, ok := l.raster[glyph]
	if !ok {
		img, _ = raster(l, glyph)
		l.raster[glyph] = img
	}
	return img, img != nil
}

// newLoadedFont is LoadedFont::new in usvg src/text/cache.rs.
//
// Port: go-text shares the font data of the parsed face, so Go copies
// nothing for any source.
func newLoadedFont(id FontID, fontdb *FontDatabase) (*loadedFont, bool) {
	// Share the font data if it is already in memory, otherwise copy it.
	pf, ok := fontdb.withFaceData(id)
	if !ok {
		return nil, false
	}

	return &loadedFont{
		pf:        pf,
		instances: map[string]*fontInstance{},
		svg:       map[GlyphID]Node{},
		raster:    map[GlyphID]*bitmapImage{},
	}, true
}

// instance is LoadedFont::instance in usvg src/text/cache.rs.
//
// Port: the key is the text of the filtered variations, because a Go map
// cannot have a slice as its key.
func (l *loadedFont) instance(variations []FontVariation) *fontInstance {
	key := filterVariations(l.pf, variations)

	keyText := make([]byte, 0, len(key)*8)
	for _, v := range key {
		keyText = append(keyText, v.Tag[:]...)
		keyText = binary.BigEndian.AppendUint32(keyText, math.Float32bits(v.Value))
	}
	if instance, ok := l.instances[string(keyText)]; ok {
		return instance
	}

	face := font.NewFace(l.pf.font)
	if len(key) != 0 {
		face.SetVariations(toGoTextVariations(key))
	}
	instance := &fontInstance{
		face:     face,
		outlines: map[GlyphID]*path.Path{},
		colr:     map[GlyphID]*Tree{},
	}
	l.instances[string(keyText)] = instance
	return instance
}

// shaper returns the harfbuzz font and the buffer of the plans of the
// instance, and makes them on the first call.
//
// Port: upstream makes the ShaperFont of harfrust from the instance at each
// call, which is cheap. A harfbuzz font of go-text holds the accelerators of
// the lookups, so Go makes it once.
func (i *fontInstance) shaper() (*harfbuzz.Font, *harfbuzz.Buffer) {
	if i.hbFont == nil {
		i.hbFont = harfbuzz.NewFont(i.face)
		i.plans = harfbuzz.NewBuffer()
	}
	return i.hbFont, i.plans
}

// svgTableOK reports whether the table SVG of the font can be read, as
// font.tables().svg().is_ok() of skrifa does.
func (l *loadedFont) svgTableOK() bool {
	if !l.svgChecked {
		l.svgChecked = true
		if raw := l.pf.rawTable("SVG "); raw != nil {
			_, _, err := tables.ParseSVG(raw)
			l.hasSVG = err == nil
		}
	}
	return l.hasSVG
}

// bitmapStrikes returns the bitmap strikes of the font, as
// font.bitmap_strikes() of skrifa does.
func (l *loadedFont) bitmapStrikes() []bitmapStrike {
	if !l.strikesLoaded {
		l.strikesLoaded = true
		l.strikes = bitmapStrikes(l.pf)
	}
	return l.strikes
}

// fontAxis is the record of an axis of the table fvar, with the fields that
// Axis of read-fonts reads.
type fontAxis struct {
	tag      [4]byte
	minValue float32
	maxValue float32
}

// fontAxes is font.axes() of skrifa. It returns the axes of the table fvar,
// or none when the table cannot be read.
func fontAxes(pf *parsedFace) []fontAxis {
	fvar := pf.rawTable("fvar")
	if len(fvar) < 16 {
		return nil
	}
	axesOffset := int(binary.BigEndian.Uint16(fvar[4:]))
	axisCount := int(binary.BigEndian.Uint16(fvar[8:]))
	axisSize := int(binary.BigEndian.Uint16(fvar[10:]))
	if axisSize < 20 || axesOffset+axisCount*axisSize > len(fvar) {
		return nil
	}
	fixed := func(b []byte) float32 { return float32(int32(binary.BigEndian.Uint32(b))) / 65536.0 }
	axes := make([]fontAxis, axisCount)
	for i := range axes {
		r := fvar[axesOffset+i*axisSize:]
		axes[i] = fontAxis{tag: [4]byte(r[:4]), minValue: fixed(r[4:]), maxValue: fixed(r[12:])}
	}
	return axes
}

// clamp is Axis::clamp of read-fonts. It clamps `value` to the range of the
// axis, as a font instance does.
//
// A maximum below the minimum is raised to it, as in normalization.
func (a fontAxis) clamp(value float32) float32 {
	minV := a.minValue
	maxV := maxF32(a.maxValue, minV)
	return minF32(maxF32(value, minV), maxV)
}

// filterVariations is font.axes().filter(variations) of skrifa, which
// LoadedFont::instance calls. It returns one variation for each axis that a
// variation names, in font order, with the last value given for it,
// clamped to the range of the axis.
func filterVariations(pf *parsedFace, variations []FontVariation) []FontVariation {
	var key []FontVariation
	for _, axis := range fontAxes(pf) {
		value, has := float32(0), false
		for _, v := range variations {
			if v.Tag == axis.tag {
				value, has = axis.clamp(v.Value), true
			}
		}
		if has {
			key = append(key, NewFontVariation(axis.tag, value))
		}
	}
	return key
}

// withOpticalSizing is with_optical_sizing in usvg src/text/cache.rs. It
// adds the `opsz` value that `font-optical-sizing: auto` implies, like
// browsers do.
//
// It comes first, so that an `opsz` in `variations` overrides it.
func withOpticalSizing(variations []FontVariation, opticalSizing FontOpticalSizing, fontSize float32) []FontVariation {
	if opticalSizing != FontOpticalSizingAuto {
		return variations
	}
	out := make([]FontVariation, 0, len(variations)+1)
	out = append(out, NewFontVariation([4]byte{'o', 'p', 's', 'z'}, fontSize))
	return append(out, variations...)
}

// outlinePathBuilder is PathBuilder in usvg src/text/cache.rs, the pen that
// draws an outline into a path.
//
// Port: skrifa draws an outline with the calls of the trait OutlinePen.
// go-text gives the segments of the outline, which drawOutline passes to
// this pen.
type outlinePathBuilder struct {
	builder *path.PathBuilder
}

// moveTo is OutlinePen::move_to for PathBuilder in usvg src/text/cache.rs.
func (b *outlinePathBuilder) moveTo(x, y float32) {
	b.builder.MoveTo(x, y)
}

// lineTo is OutlinePen::line_to for PathBuilder in usvg src/text/cache.rs.
func (b *outlinePathBuilder) lineTo(x, y float32) {
	b.builder.LineTo(x, y)
}

// quadTo is OutlinePen::quad_to for PathBuilder in usvg src/text/cache.rs.
func (b *outlinePathBuilder) quadTo(cx0, cy0, x, y float32) {
	b.builder.QuadTo(cx0, cy0, x, y)
}

// curveTo is OutlinePen::curve_to for PathBuilder in usvg
// src/text/cache.rs.
func (b *outlinePathBuilder) curveTo(cx0, cy0, cx1, cy1, x, y float32) {
	b.builder.CubicTo(cx0, cy0, cx1, cy1, x, y)
}

// close is OutlinePen::close for PathBuilder in usvg src/text/cache.rs.
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

// outline is outline in usvg src/text/cache.rs.
//
// Port: skrifa outlines a glyph of a font instance. go-text does the same
// with a face whose coordinates are the variations.
func outline(pf *parsedFace, face *font.Face, glyphID GlyphID) (*path.Path, bool) {
	builder := &outlinePathBuilder{builder: path.NewPathBuilder()}
	if !drawOutline(pf, face, glyphID, builder) {
		return nil, false
	}

	return builder.builder.Finish()
}

// bitmapGlyph is BitmapGlyph of read-fonts src/model/glyph/bitmap.rs, with
// the fields that usvg reads, for a glyph with PNG data.
type bitmapGlyph struct {
	png           []byte
	isSbix        bool
	width         uint32
	height        uint32
	xBearing      int32
	yBearing      int32
	innerXBearing int32
	innerYBearing int32
	ppemX         uint16
	ppemY         uint16
}

// ppem is BitmapGlyph::ppem of read-fonts. It returns the larger of the
// horizontal and vertical pixels per em.
func (g *bitmapGlyph) ppem() uint16 {
	return max(g.ppemX, g.ppemY)
}

// bitmapStrike is Strike of read-fonts src/model/glyph/bitmap.rs: its size
// and the lookup of a glyph in it.
type bitmapStrike struct {
	ppemX  uint16
	ppemY  uint16
	lookup func(glyphID GlyphID) (bitmapGlyph, bool)
}

// ppem is Strike::ppem of read-fonts. It returns the larger of the
// horizontal and vertical pixels per em.
func (s *bitmapStrike) ppem() uint16 {
	return max(s.ppemX, s.ppemY)
}

// get is Strike::get of read-fonts. It returns the bitmap for `glyph`, or
// false if the strike does not contain it.
//
// A strike with no size has no glyphs, because its bitmaps cannot be
// placed in font units.
func (s *bitmapStrike) get(glyphID GlyphID) (bitmapGlyph, bool) {
	if s.ppemX == 0 || s.ppemY == 0 {
		return bitmapGlyph{}, false
	}
	return s.lookup(glyphID)
}

// pngSignature is PNG_SIGNATURE of read-fonts: the eight-byte signature at
// the start of every PNG file.
var pngSignature = [8]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}

// maxSbixDupes is MAX_SBIX_DUPES of read-fonts: the maximum number of `dupe`
// entries to follow.
//
// This limit stops a cycle of entries from looping forever.
const maxSbixDupes = 8

// bitmapStrikes is BitmapStrikes::new of read-fonts
// src/model/glyph/bitmap.rs, which usvg calls with font.bitmap_strikes. It
// returns the strikes of the first of the tables sbix and CBDT that the face
// has.
//
// Port: read-fonts also reads the table EBDT, whose glyphs are masks with no
// PNG data, which usvg does not draw. The strikes of EBDT are left out, and
// usvg draws the outline of such a glyph, as it does with read-fonts.
func bitmapStrikes(pf *parsedFace) []bitmapStrike {
	numGlyphs := 0
	if maxp := pf.rawTable("maxp"); len(maxp) >= 6 {
		numGlyphs = int(binary.BigEndian.Uint16(maxp[4:]))
	}

	if raw := pf.rawTable("sbix"); raw != nil {
		if sbix, _, err := tables.ParseSbix(raw, numGlyphs); err == nil {
			strikes := make([]bitmapStrike, 0, len(sbix.Strikes))
			for _, st := range sbix.Strikes {
				strikes = append(strikes, bitmapStrike{
					ppemX:  st.Ppem,
					ppemY:  st.Ppem,
					lookup: func(glyphID GlyphID) (bitmapGlyph, bool) { return sbixGlyph(st, pf, glyphID) },
				})
			}
			return strikes
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
					ppemX: uint16(size.PpemX),
					ppemY: uint16(size.PpemY),
					lookup: func(glyphID GlyphID) (bitmapGlyph, bool) {
						return cbdtGlyph(size, bitDepth, subtables, cbdt, glyphID)
					},
				})
			}
			return strikes
		}
	}

	return nil
}

// sbixGlyph is sbix_glyph in read-fonts src/model/glyph/bitmap.rs. It reads
// a PNG glyph from an `sbix` strike.
//
// A `dupe` entry resolves to the image of the glyph it names.
func sbixGlyph(strike tables.Strike, pf *parsedFace, glyph GlyphID) (bitmapGlyph, bool) {
	// The bearings come from the glyph asked for, not the one a `dupe`
	// entry points at, and are only read once a PNG is found.
	requested := glyph
	for range maxSbixDupes + 1 {
		if int(glyph) >= len(strike.GlyphDatas) {
			return bitmapGlyph{}, false
		}
		entry := strike.GlyphDatas[glyph]
		if len(entry.Data) == 0 {
			return bitmapGlyph{}, false
		}
		switch entry.GraphicType {
		case ot.MustNewTag("dupe"):
			if len(entry.Data) < 2 {
				return bitmapGlyph{}, false
			}
			glyph = GlyphID(binary.BigEndian.Uint16(entry.Data))
		case ot.MustNewTag("png "):
			data := entry.Data
			if len(data) < 8 || [8]byte(data[:8]) != pngSignature {
				return bitmapGlyph{}, false
			}
			// The IHDR chunk follows the signature: four bytes of length,
			// four of type, then the width and height.
			if len(data) < 24 {
				return bitmapGlyph{}, false
			}
			width := binary.BigEndian.Uint32(data[16:])
			height := binary.BigEndian.Uint32(data[20:])
			// HarfBuzz refuses a dimension that an i32 of pixels would
			// not hold once scaled.
			if width >= 1<<16 || height >= 1<<16 {
				return bitmapGlyph{}, false
			}
			xBearing, yBearing := sbixBearings(pf, requested)
			return bitmapGlyph{
				png:           data,
				isSbix:        true,
				width:         width,
				height:        height,
				xBearing:      xBearing,
				yBearing:      yBearing,
				innerXBearing: int32(entry.OriginOffsetX),
				innerYBearing: int32(entry.OriginOffsetY),
				ppemX:         strike.Ppem,
				ppemY:         strike.Ppem,
			}, true
		default:
			return bitmapGlyph{}, false
		}
	}
	return bitmapGlyph{}, false
}

// cbdtGlyph is Strike::get of read-fonts src/model/glyph/bitmap.rs for a
// strike of CBDT, with BitmapGlyph::from_bdt, for a glyph with PNG data.
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
			width:         uint32(metrics.Width),
			height:        uint32(metrics.Height),
			innerXBearing: int32(metrics.BearingX),
			innerYBearing: int32(metrics.BearingY),
			ppemX:         uint16(size.PpemX),
			ppemY:         uint16(size.PpemY),
		}, true
	}
	return bitmapGlyph{}, false
}

// selectBitmap is BitmapStrikes::select_bitmap of read-fonts
// src/model/glyph/bitmap.rs, for no size, which selects the largest image
// available.
//
// Strikes are ranked the same way as in `select`, but only strikes that
// contain the glyph are considered.
func selectBitmap(strikes []bitmapStrike, glyphID GlyphID) (bitmapGlyph, bool) {
	var best bitmapGlyph
	hasBest := false
	for i := range strikes {
		strike := &strikes[i]
		// closer_to with no size prefers a larger strike.
		if !hasBest || strike.ppem() > best.ppem() {
			if bitmap, ok := strike.get(glyphID); ok {
				best, hasBest = bitmap, true
			}
		}
	}
	return best, hasBest
}

// glyfGlyph returns the data of a glyph in the table glyf, as Loca::get_glyf
// of read-fonts finds it. The data is empty for an empty glyph, and holds at
// least the header of 10 bytes otherwise.
func glyfGlyph(pf *parsedFace, glyphID GlyphID) ([]byte, bool) {
	head, loca, glyf := pf.rawTable("head"), pf.rawTable("loca"), pf.rawTable("glyf")
	if len(head) < headMinSize || loca == nil || glyf == nil {
		return nil, false
	}
	var start, end int
	if int16(binary.BigEndian.Uint16(head[50:])) == 0 {
		i := int(glyphID) * 2
		if i+4 > len(loca) {
			return nil, false
		}
		start = int(binary.BigEndian.Uint16(loca[i:])) * 2
		end = int(binary.BigEndian.Uint16(loca[i+2:])) * 2
	} else {
		i := int(glyphID) * 4
		if i+8 > len(loca) {
			return nil, false
		}
		start = int(binary.BigEndian.Uint32(loca[i:]))
		end = int(binary.BigEndian.Uint32(loca[i+4:]))
	}
	if end == start {
		return []byte{}, true
	}
	if end < start || start+10 > len(glyf) {
		return nil, false
	}
	return glyf[start:], true
}

// sbixBearings is sbix_bearings in read-fonts src/model/glyph/bitmap.rs. It
// returns the bearings used to position an `sbix` bitmap, in font units.
//
// The specification places the image at the glyph origin. Skia and
// CoreText offset it by the `hmtx` bearing and the bottom of the outline's
// bounding box. This places the image where the outline would be. A glyph
// without an outline is placed at the origin.
func sbixBearings(pf *parsedFace, glyph GlyphID) (int32, int32) {
	g, ok := glyfGlyph(pf, glyph)
	if !ok || len(g) == 0 {
		return 0, 0
	}
	if int16(binary.BigEndian.Uint16(g)) == 0 {
		return 0, 0
	}
	lsb, _ := statedSideBearing(pf, glyph)
	return lsb, int32(int16(binary.BigEndian.Uint16(g[4:])))
}

// statedSideBearing is HMetricRecords::stated_side_bearing of read-fonts:
// the left side bearing of the table hmtx, in font units.
func statedSideBearing(pf *parsedFace, glyphID GlyphID) (int32, bool) {
	hhea, hmtx := pf.rawTable("hhea"), pf.rawTable("hmtx")
	if len(hhea) < hheaMinSize || hmtx == nil {
		return 0, false
	}
	numberOfHMetrics := int(binary.BigEndian.Uint16(hhea[34:]))
	var off int
	if int(glyphID) < numberOfHMetrics {
		off = int(glyphID)*4 + 2
	} else {
		off = numberOfHMetrics*4 + (int(glyphID)-numberOfHMetrics)*2
	}
	if off+2 > len(hmtx) {
		return 0, false
	}
	return int32(int16(binary.BigEndian.Uint16(hmtx[off:]))), true
}

// raster is raster in usvg src/text/cache.rs.
//
// Port: it takes the loaded font, which holds the bitmap strikes.
func raster(l *loadedFont, glyphID GlyphID) (*bitmapImage, bool) {
	// No size selects the largest image available.
	image, ok := selectBitmap(l.bitmapStrikes(), glyphID)
	if !ok {
		return nil, false
	}

	// Follows vello:
	// https://github.com/linebender/vello/blob/717a485dabd16249fae739a755a3da3ea4a8f69a/glifo/src/glyph.rs#L1322-L1367
	var bearingX, bearingY float32
	if isAppleColorEmoji(l.pf) {
		bearingX, bearingY = 0.0, -float32(unitsPerEmOf(l.pf))/8.0
	} else {
		bearingX, bearingY = float32(image.xBearing), float32(image.yBearing)
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
		x:           int16(image.innerXBearing),
		y:           int16(image.innerYBearing),
		pixelsPerEm: image.ppemX,
		bearingX:    bearingX,
		bearingY:    bearingY,
		isSbix:      image.isSbix,
	}, true
}

// unitsPerEmOf is font.metrics().units_per_em of read-fonts: the units per
// em of the table head when it is between 16 and 16384, and 1000 otherwise.
func unitsPerEmOf(pf *parsedFace) uint16 {
	var stated uint16
	if head := pf.rawTable("head"); len(head) >= headMinSize {
		stated = binary.BigEndian.Uint16(head[18:])
	}
	if stated >= 16 && stated <= 16384 {
		return stated
	}
	return 1000
}

// isAppleColorEmoji is is_apple_color_emoji in usvg src/text/cache.rs.
func isAppleColorEmoji(pf *parsedFace) bool {
	names, ok := parseNameTable(pf.rawTable("name"))
	if !ok {
		return false
	}
	for _, name := range names {
		if name.nameID != nameIDPostScriptName {
			continue
		}
		if s, ok := nameToUnicode(name); ok && s == "AppleColorEmoji" {
			return true
		}
	}
	return false
}

// svg is svg in usvg src/text/cache.rs.
//
// Port: opt holds the logger (D20).
func svg(pf *parsedFace, glyphID GlyphID, opt *options) (Node, bool) {
	// SEE: https://docs.rs/read-fonts/latest/read_fonts/tables/svg/type.Svg.html

	// TODO: Technically not 100% accurate because the SVG format in a OTF font
	// is actually a subset/superset of a normal SVG, but it seems to work fine
	// for Twitter Color Emoji, so might as well use what we already have.

	// TODO: Glyph records can contain the data for multiple glyphs. We should
	// add a cache so we don't need to reparse the data every time.
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
		node = &tree.root
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

// colr is colr in usvg src/text/cache.rs.
//
// Port: skrifa paints a COLR glyph with ColorGlyph::paint, which
// paintColorGlyph of usvg_text_colr_paint.go does on the tables of go-text.
// face is the font instance, whose coordinates are the variations. opt
// holds the logger (D20).
func colr(pf *parsedFace, face *font.Face, glyphID GlyphID, opt *options) (*Tree, bool) {
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
