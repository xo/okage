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

	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
	"github.com/xo/okage/typesetting/harfbuzz"

	colrpaint "github.com/xo/okage/internal/colr"
	"github.com/xo/okage/internal/glyphoutline"
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
// instanceKey holds the text of the key that instance makes, not the
// variations, since the text is the key of instances.
type loadedFont struct {
	pf          *parsedFace
	instances   map[string]*fontInstance
	instanceKey []byte
	svg         map[GlyphID]Node
	raster      map[GlyphID]*bitmapImage

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
		t, _ = colr(l.pf, instance.face, glyph, 0, svgtypes.NewColorRGBA(0, 0, 0, 255), c.opt)
		instance.colr[glyph] = t
	}
	return t, t != nil
}

// hasColorGlyph reports if glyph of the font id is a glyph of the tables
// COLR, SVG, CBDT or sbix, which flatten draws with its own colors.
//
// Port: usvg has no such function. The fallback of D40 uses it.
func (c *fontCache) hasColorGlyph(id FontID, glyph GlyphID) bool {
	l, ok := c.load(id)
	if !ok {
		return false
	}
	if colr := l.pf.font.COLR; colr != nil {
		if _, ok := colr.Search(tables.GlyphID(glyph)); ok {
			return true
		}
	}
	if _, ok := c.svg(id, glyph); ok {
		return true
	}
	_, ok = c.raster(id, glyph)
	return ok
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
		pf:          pf,
		instances:   map[string]*fontInstance{},
		instanceKey: nil,
		svg:         map[GlyphID]Node{},
		raster:      map[GlyphID]*bitmapImage{},
	}, true
}

// instance is LoadedFont::instance in usvg src/text/cache.rs.
//
// Port: the key is the text of the filtered variations, because a Go map
// cannot have a slice as its key. The buffer instanceKey holds that text
// between the calls.
func (l *loadedFont) instance(variations []FontVariation) *fontInstance {
	key := filterVariations(l.pf, variations)

	keyText := l.instanceKey[:0]
	for _, v := range key {
		keyText = append(keyText, v.Tag[:]...)
		keyText = binary.BigEndian.AppendUint32(keyText, math.Float32bits(v.Value))
	}
	l.instanceKey = keyText
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

// MoveTo is OutlinePen::move_to for PathBuilder in usvg src/text/cache.rs.
func (b *outlinePathBuilder) MoveTo(x, y float32) {
	b.builder.MoveTo(x, y)
}

// LineTo is OutlinePen::line_to for PathBuilder in usvg src/text/cache.rs.
func (b *outlinePathBuilder) LineTo(x, y float32) {
	b.builder.LineTo(x, y)
}

// QuadTo is OutlinePen::quad_to for PathBuilder in usvg src/text/cache.rs.
func (b *outlinePathBuilder) QuadTo(cx0, cy0, x, y float32) {
	b.builder.QuadTo(cx0, cy0, x, y)
}

// CurveTo is OutlinePen::curve_to for PathBuilder in usvg
// src/text/cache.rs.
func (b *outlinePathBuilder) CurveTo(cx0, cy0, cx1, cy1, x, y float32) {
	b.builder.CubicTo(cx0, cy0, cx1, cy1, x, y)
}

// Close is OutlinePen::close for PathBuilder in usvg src/text/cache.rs.
func (b *outlinePathBuilder) Close() {
	b.builder.Close()
}

// drawOutline is OutlineGlyph::draw of skrifa with unhinted settings, which
// usvg calls. It draws the outline of glyph of face, a face of pf, into pen,
// and returns false when the face has no outline for the glyph.
//
// Port: the package glyphoutline puts the segments of go-text in the order
// of skrifa, as it does for pdf.
func drawOutline(pf *parsedFace, face *font.Face, glyph GlyphID, pen glyphoutline.Pen) bool {
	return pf.outline.Draw(face, font.GID(glyph), pen)
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

// bitmapPixels is the case of Pixels of read-fonts
// src/model/glyph/bitmap.rs.
type bitmapPixels uint8

// The cases of Pixels of read-fonts.
const (
	// bitmapPixelsPNG is Pixels::Png.
	bitmapPixelsPNG bitmapPixels = iota
	// bitmapPixelsBGRA is Pixels::Bgra.
	bitmapPixelsBGRA
	// bitmapPixelsMask is Pixels::Mask.
	bitmapPixelsMask
)

// bitmapGlyph is BitmapGlyph of read-fonts src/model/glyph/bitmap.rs, with
// the fields that usvg reads. Only a glyph of PNG pixels keeps its data,
// because usvg draws no other glyph.
type bitmapGlyph struct {
	pixels        bitmapPixels
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
// returns the strikes of the first of the tables sbix, CBDT and EBDT that
// the face has.
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

	if strikes, ok := bdtStrikes(pf.rawTable("CBLC"), pf.rawTable("CBDT"), true); ok {
		return strikes
	}
	// The glyphs of EBDT are masks, which usvg does not draw. Their strikes
	// still count, as in read-fonts (D100).
	if strikes, ok := bdtStrikes(pf.rawTable("EBLC"), pf.rawTable("EBDT"), false); ok {
		return strikes
	}

	return nil
}

// bdtStrikes returns the strikes of the location table blc and the data
// table bdt: CBLC and CBDT when isColor is true, EBLC and EBDT otherwise. It
// returns false when the font does not have both tables, or when go-text
// cannot parse the location table.
func bdtStrikes(blc, bdt []byte, isColor bool) ([]bitmapStrike, bool) {
	if blc == nil || bdt == nil {
		return nil, false
	}
	parsed, _, err := tables.ParseCBLC(blc)
	if err != nil {
		return nil, false
	}
	strikes := make([]bitmapStrike, 0, len(parsed.BitmapSizes))
	for i, size := range parsed.BitmapSizes {
		// Port: go-text keeps the glyph range and the bit depth private.
		// They are the bytes 40 to 43 and the byte 46 of the record of the
		// size, which starts at 8.
		var r bdtSizeRange
		if off := 8 + 48*i; off+47 <= len(blc) {
			r.startGlyph = tables.GlyphID(binary.BigEndian.Uint16(blc[off+40:]))
			r.endGlyph = tables.GlyphID(binary.BigEndian.Uint16(blc[off+42:]))
			r.bitDepth = blc[off+46]
		}
		subtables := parsed.IndexSubTables[i]
		strikes = append(strikes, bitmapStrike{
			ppemX: uint16(size.PpemX),
			ppemY: uint16(size.PpemY),
			lookup: func(glyphID GlyphID) (bitmapGlyph, bool) {
				return bdtGlyph(size, r, subtables, bdt, isColor, glyphID)
			},
		})
	}
	return strikes, true
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

// bdtSizeRange holds the fields of BitmapSize that go-text keeps private:
// the range of the glyphs of the size and its bit depth.
type bdtSizeRange struct {
	startGlyph tables.GlyphID
	endGlyph   tables.GlyphID
	bitDepth   uint8
}

// bdtLocation is BitmapLocation of read-fonts src/tables/bitmap.rs: where the
// data of a glyph is in the data table, and its format.
type bdtLocation struct {
	format     uint16
	start, end int
	metrics    *tables.BigGlyphMetrics
}

// locate is BitmapSize::location of read-fonts src/tables/bitmap.rs. It
// finds the data of gid in the first index subtable whose range holds it.
func (r bdtSizeRange) locate(subtables []tables.BitmapSubtable, gid tables.GlyphID) (bdtLocation, bool) {
	if gid < r.startGlyph || gid > r.endGlyph {
		return bdtLocation{}, false
	}
	for _, st := range subtables {
		if gid < st.FirstGlyph || gid > st.LastGlyph {
			continue
		}
		loc := bdtLocation{format: st.ImageFormat}
		base := int(st.ImageDataOffset)
		index := int(gid - st.FirstGlyph)
		switch data := st.IndexData.(type) {
		case tables.IndexData1:
			if index+1 >= len(data.SbitOffsets) {
				return bdtLocation{}, false
			}
			loc.start, loc.end = base+int(data.SbitOffsets[index]), base+int(data.SbitOffsets[index+1])
		case tables.IndexData3:
			if index+1 >= len(data.SbitOffsets) {
				return bdtLocation{}, false
			}
			loc.start, loc.end = base+int(data.SbitOffsets[index]), base+int(data.SbitOffsets[index+1])
		case tables.IndexData2:
			loc.start = base + int(data.ImageSize)*index
			loc.end = loc.start + int(data.ImageSize)
			loc.metrics = &data.BigMetrics
		case tables.IndexData4:
			i := slices.IndexFunc(data.GlyphArray, func(p tables.GlyphIdOffsetPair) bool { return p.GlyphID == gid })
			if i < 0 || i+1 >= len(data.GlyphArray) {
				return bdtLocation{}, false
			}
			loc.start, loc.end = base+int(data.GlyphArray[i].SbitOffset), base+int(data.GlyphArray[i+1].SbitOffset)
		case tables.IndexData5:
			i := slices.Index(data.GlyphIdArray, gid)
			if i < 0 {
				return bdtLocation{}, false
			}
			loc.start = base + int(data.ImageSize)*i
			loc.end = loc.start + int(data.ImageSize)
			loc.metrics = &data.BigMetrics
		default:
			return bdtLocation{}, false
		}
		if loc.end < loc.start {
			return bdtLocation{}, false
		}
		return loc, true
	}
	return bdtLocation{}, false
}

// bdtFormat is BitmapDataFormat of read-fonts src/tables/bitmap.rs: how the
// pixels of a glyph are stored.
type bdtFormat uint8

// The cases of BitmapDataFormat of read-fonts.
const (
	// bdtByteAligned is BitmapDataFormat::ByteAligned.
	bdtByteAligned bdtFormat = iota
	// bdtBitAligned is BitmapDataFormat::BitAligned.
	bdtBitAligned
	// bdtPNG is BitmapDataFormat::Png.
	bdtPNG
)

// bdtGlyph is Strike::get of read-fonts src/model/glyph/bitmap.rs for a
// strike of CBDT or EBDT, with bitmap_data of src/tables/bitmap.rs and
// BitmapGlyph::from_bdt. isColor is true for CBDT, the one table whose
// glyphs can hold PNG data. A glyph of BGRA or mask pixels has no PNG data,
// but it is a glyph all the same, as in read-fonts (D100).
func bdtGlyph(size tables.BitmapSize, r bdtSizeRange, subtables []tables.BitmapSubtable, bdt []byte, isColor bool, glyphID GlyphID) (bitmapGlyph, bool) {
	loc, ok := r.locate(subtables, tables.GlyphID(glyphID))
	if !ok || loc.start < 0 || loc.end > len(bdt) {
		return bitmapGlyph{}, false
	}
	b := bdt[loc.start:loc.end]
	smallMetrics := func() (tables.SmallGlyphMetrics, bool) {
		if len(b) < 5 {
			return tables.SmallGlyphMetrics{}, false
		}
		m := tables.SmallGlyphMetrics{
			Height: b[0], Width: b[1], BearingX: int8(b[2]), BearingY: int8(b[3]), Advance: b[4],
		}
		b = b[5:]
		return m, true
	}
	// The big metrics start with the fields of the small metrics. Their
	// three vertical fields follow, which usvg does not read.
	bigMetrics := func() (tables.SmallGlyphMetrics, bool) {
		if len(b) < 8 {
			return tables.SmallGlyphMetrics{}, false
		}
		m, _ := smallMetrics()
		b = b[3:]
		return m, true
	}
	read := func(n int) ([]byte, bool) {
		if n > len(b) {
			return nil, false
		}
		return b[:n], true
	}
	depth := int(r.bitDepth)

	var metrics tables.SmallGlyphMetrics
	var format bdtFormat
	var data []byte
	switch loc.format {
	case 1, 6:
		if loc.format == 1 {
			metrics, ok = smallMetrics()
		} else {
			metrics, ok = bigMetrics()
		}
		if !ok {
			return bitmapGlyph{}, false
		}
		// The data for each row is padded to a byte boundary
		pitch := (int(metrics.Width)*depth + 7) / 8
		format = bdtByteAligned
		data, ok = read(pitch * int(metrics.Height))
	case 2, 5, 7:
		switch loc.format {
		case 2:
			metrics, ok = smallMetrics()
		case 7:
			metrics, ok = bigMetrics()
		default:
			if loc.metrics == nil {
				return bitmapGlyph{}, false
			}
			metrics, ok = loc.metrics.SmallGlyphMetrics, true
		}
		if !ok {
			return bitmapGlyph{}, false
		}
		// The data is tightly packed
		format = bdtBitAligned
		data, ok = read((int(metrics.Width)*depth*int(metrics.Height) + 7) / 8)
	case 17, 18, 19:
		if !isColor {
			return bitmapGlyph{}, false
		}
		switch loc.format {
		case 17:
			metrics, ok = smallMetrics()
		case 18:
			metrics, ok = bigMetrics()
		default:
			if loc.metrics == nil {
				return bitmapGlyph{}, false
			}
			metrics, ok = loc.metrics.SmallGlyphMetrics, true
		}
		if !ok || len(b) < 4 {
			return bitmapGlyph{}, false
		}
		n := int(binary.BigEndian.Uint32(b))
		b = b[4:]
		format = bdtPNG
		data, ok = read(n)
	default:
		// Composite bitmaps, formats 8 and 9, are assembled from others,
		// which nothing here does yet. No other format is valid.
		return bitmapGlyph{}, false
	}
	if !ok {
		return bitmapGlyph{}, false
	}

	width, height := int(metrics.Width), int(metrics.Height)
	glyph := bitmapGlyph{
		width:         uint32(width),
		height:        uint32(height),
		innerXBearing: int32(metrics.BearingX),
		innerYBearing: int32(metrics.BearingY),
		ppemX:         uint16(size.PpemX),
		ppemY:         uint16(size.PpemY),
	}
	switch {
	case depth == 32 && format == bdtPNG:
		glyph.png = data
	case depth == 32 && format == bdtByteAligned:
		// Bgra::new needs four bytes for each pixel.
		if len(data) < width*height*4 {
			return bitmapGlyph{}, false
		}
		glyph.pixels = bitmapPixelsBGRA
	case (depth == 1 || depth == 2 || depth == 4 || depth == 8) && format != bdtPNG:
		// Mask::new needs the bytes that bitmap_data reads, so it does not
		// fail here.
		glyph.pixels = bitmapPixelsMask
	default:
		return bitmapGlyph{}, false
	}
	return glyph, true
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
	// TODO: implement other bitmap formats
	if image.pixels != bitmapPixelsPNG {
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
// colrpaint.Paint of internal/colr does on the tables of go-text.
// face is the font instance, whose coordinates are the variations. opt
// holds the logger (D20). palette and foreground are the palette of CPAL
// and the color of the text, which FontDatabase.ColorGlyph gives (D57).
// usvg takes the palette 0 and black, as FontCache.colr does.
func colr(pf *parsedFace, face *font.Face, glyphID GlyphID, palette int, foreground svgtypes.Color, opt *options) (*Tree, bool) {
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
		palette:          palette,
		foregroundColor:  foreground,
		transform:        colrpaint.IdentityTransform(),
		outlineTransform: colrpaint.IdentityTransform(),
		transformsStack:  []colrpaint.Transform{colrpaint.IdentityTransform()},
		clipStack:        nil,
		opt:              opt,
	}

	// usvg walks the graph with the font model of read-fonts 0.46 (D100).
	if !colrpaint.PaintModel(pf.font, face.Coords(), colrpaint.GlyphID(glyphID), glyphPainter) {
		return nil, false
	}
	svg.EndElement()

	tree, err := fromData([]byte(svg.EndDocument()), newOptions())
	if err != nil {
		return nil, false
	}
	return tree, true
}
