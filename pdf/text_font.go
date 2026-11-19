// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"unicode"
	"unicode/utf16"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/siphasher"
	"github.com/xo/okage/path"
	tsfont "github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
	"github.com/xo/okage/typesetting/harfbuzz"
)

// This file ports krilla src/text/font.rs. skrifa reads the fonts in
// upstream, and typesetting reads them in the port (D52, D62). The metrics
// of FontInfo::new follow Metrics::new and Attributes::new of skrifa 0.44,
// which krilla calls, on the bytes of the tables head, hhea, OS/2, post,
// name and MVAR. The other parts of skrifa that the text calls, such as the
// outlines and the bitmap strikes, are in the files of text/glyph.
//
// The port leaves out StandardFont, the 14 standard fonts of PDF. Only the
// forms of krilla use them, and the base port has no forms (D62).

// Tag is Tag in krilla src/text/font.rs.
//
// A 4-byte OpenType tag.
type Tag [4]byte

// TagFromString is Tag::try_from_str in krilla src/text/font.rs. It returns
// false if the string is not 4 bytes in size.
//
// Try to create a new tag from a string.
func TagFromString(s string) (Tag, bool) {
	if len(s) != 4 {
		return Tag{}, false
	}
	return Tag([]byte(s)), true
}

// VariationCoord is an item (Tag, f32) of the variation coordinates of
// Font::new_variable in krilla src/text/font.rs: the tag of an axis and a
// value in the coordinates of the user, such as 700 for the axis wght.
type VariationCoord struct {
	Tag   Tag
	Value float32
}

// FontOption is an option of NewFont (D20).
type FontOption func(*fontOptions)

// fontOptions holds the options of NewFont.
type fontOptions struct {
	coords []VariationCoord
}

// WithVariations is the argument variation_coords of Font::new_variable in
// krilla src/text/font.rs. It sets the coordinates of the variations of a
// variable font. A tag that is not an axis of the font has no effect.
func WithVariations(coords ...VariationCoord) FontOption {
	return func(o *fontOptions) {
		o.coords = append(o.coords, coords...)
	}
}

// Font is Font in krilla src/text/font.rs. Two fonts are equal when their
// fontInfo is equal, as in upstream, so two fonts that NewFont makes from
// the same bytes share their subset in a document.
//
// An OpenType font. Can be a TrueType, OpenType font or a TrueType
// collection. It holds a reference to the underlying data as well as some
// basic information about the font.
//
// A Font is safe for concurrent use. Do not change the bytes that NewFont
// takes while the font is in use.
type Font struct {
	data []byte
	info *fontInfo
	// hash is the hash of info, as Prehashed of upstream holds it.
	hash siphasher.Hash128

	ld *ot.Loader
	ft *tsfont.Font

	// mu guards face, hb and glyf, which typesetting does not let two
	// goroutines use at once.
	mu   sync.Mutex
	face *tsfont.Face
	hb   *harfbuzz.Font
	// glyf holds the table glyf, parsed on its first use, for the order of
	// the outlines of skrifa. glyfDone is true after the first use.
	glyf     tables.Glyf
	glyfDone bool

	// svg is the table SVG, which svgOnce reads on its first use. It is
	// nil for a font without a valid table SVG.
	svgOnce sync.Once
	svg     *tables.SVG
	// strikes holds the bitmap strikes, which strikesOnce reads on their
	// first use.
	strikesOnce   sync.Once
	strikesFormat bitmapFormat
	strikes       []bitmapStrike
}

// NewFont is Font::new and Font::new_variable in krilla src/text/font.rs.
// The index indicates the index of a font in a TrueType collection, and is
// 0 for a single font. WithVariations gives the coordinates of a variable
// font.
//
// krilla returns None if the index is invalid or the font cannot be read.
// The port returns an error that wraps okage.ErrPDFInvalidFont. A font that
// is not TrueType or OpenType, such as a Type 1 font or a bare CFF font,
// gives that error. The caller draws its glyphs as paths with
// Surface.DrawPath (D62, question 25). A WOFF, WOFF2 or EOT font is
// converted with the package webfont first.
//
// Create a new font from some data.
func NewFont(data []byte, index int, opts ...FontOption) (*Font, error) {
	var o fontOptions
	for _, opt := range opts {
		opt(&o)
	}
	if index < 0 || int64(index) > math.MaxUint32 {
		return nil, fmt.Errorf("%w: the index %d is not valid", okage.ErrPDFInvalidFont, index)
	}
	ld, ok := fontLoader(data, uint32(index))
	if !ok {
		return nil, fmt.Errorf("%w: the font with the index %d cannot be read", okage.ErrPDFInvalidFont, index)
	}
	f := &Font{data: data, ld: ld}
	ft, err := tsfont.NewFont(ld)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", okage.ErrPDFInvalidFont, err)
	}
	f.ft = ft
	f.face = tsfont.NewFace(ft)
	if len(o.coords) != 0 {
		vars := make([]tsfont.Variation, len(o.coords))
		for i, c := range o.coords {
			vars[i] = tsfont.Variation{Tag: ot.NewTag(c.Tag[0], c.Tag[1], c.Tag[2], c.Tag[3]), Value: c.Value}
		}
		f.face.SetVariations(vars)
	}
	info, ok := newFontInfo(f, uint32(index), len(data), o.coords)
	if !ok {
		return nil, fmt.Errorf("%w: the font with the index %d has no valid table head", okage.ErrPDFInvalidFont, index)
	}
	f.info = info
	f.hash = sipHash(info)
	return f, nil
}

// fontLoader is FontRef::from_index of read-fonts. It reads the font of the
// index in a collection, or a single font with the index 0. read-fonts reads
// a font whose version is 0x00010000, OTTO or true only, so a WOFF file or
// a Type 1 font gives false.
func fontLoader(data []byte, index uint32) (*ot.Loader, bool) {
	if len(data) < 4 {
		return nil, false
	}
	switch string(data[:4]) {
	case "\x00\x01\x00\x00", "OTTO", "true":
		if index != 0 {
			return nil, false
		}
	case "ttcf":
	default:
		return nil, false
	}
	lds, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil || uint64(index) >= uint64(len(lds)) {
		return nil, false
	}
	return lds[index], true
}

// table returns the bytes of a table of the font, or nil.
func (f *Font) table(tag string) []byte {
	b, err := f.ld.RawTable(ot.MustNewTag(tag))
	if err != nil {
		return nil
	}
	return b
}

// postscriptName is Font::postscript_name in krilla src/text/font.rs.
func (f *Font) postscriptName() (string, bool) {
	return f.info.postscriptName, f.info.hasPostscriptName
}

// isAppleColorEmoji is Font::is_apple_color_emoji in krilla
// src/text/font.rs.
func (f *Font) isAppleColorEmoji() bool {
	return f.info.isAppleColorEmoji
}

// index is Font::index in krilla src/text/font.rs.
func (f *Font) index() uint32 {
	return f.info.index
}

// variationCoordinates is Font::variation_coordinates in krilla
// src/text/font.rs.
func (f *Font) variationCoordinates() []VariationCoord {
	return f.info.varCoords
}

// capHeight is Font::cap_height in krilla src/text/font.rs.
func (f *Font) capHeight() (float32, bool) {
	return f.info.capHeight, f.info.hasCapHeight
}

// ascent is Font::ascent in krilla src/text/font.rs.
func (f *Font) ascent() float32 {
	return f.info.ascent
}

// weight is Font::weight in krilla src/text/font.rs.
func (f *Font) weight() float32 {
	return f.info.weight
}

// stretch is Font::stretch in krilla src/text/font.rs.
func (f *Font) stretch() float32 {
	return f.info.stretch
}

// descent is Font::descent in krilla src/text/font.rs.
func (f *Font) descent() float32 {
	return f.info.descent
}

// numGlyphs is Font::num_glyphs in krilla src/text/font.rs.
func (f *Font) numGlyphs() uint32 {
	return f.info.numGlyphs
}

// isMonospaced is Font::is_monospaced in krilla src/text/font.rs.
func (f *Font) isMonospaced() bool {
	return f.info.isMonospaced
}

// italicAngle is Font::italic_angle in krilla src/text/font.rs.
func (f *Font) italicAngle() float32 {
	return f.info.italicAngle
}

// UnitsPerEm is Font::units_per_em in krilla src/text/font.rs.
//
// The units per em of the font.
func (f *Font) UnitsPerEm() float32 {
	return float32(f.info.unitsPerEm)
}

// bbox is Font::bbox in krilla src/text/font.rs.
func (f *Font) bbox() Rect {
	return f.info.globalBBox
}

// fontData is Font::font_data in krilla src/text/font.rs.
func (f *Font) fontData() []byte {
	return f.data
}

// advanceWidth is Font::advance_width in krilla src/text/font.rs, with
// GlyphMetrics::advance_width of skrifa at the location of the font. It
// returns false for a glyph that the font does not have.
//
// skrifa adds the delta of HVAR truncated to an integer, as FreeType does,
// or the delta of the phantom points of gvar rounded to an integer. The
// port takes the advance of typesetting at the location, and rounds the
// delta in the same way. typesetting computes the delta in float32, so a
// width can differ from the width of krilla by 1 unit (D65).
func (f *Font) advanceWidth(glyph GlyphID) (float32, bool) {
	maxp, hhea, hmtx := f.table("maxp"), f.table("hhea"), f.table("hmtx")
	if len(maxp) < 6 || uint32(glyph) >= uint32(binary.BigEndian.Uint16(maxp[4:])) {
		return 0, false
	}
	var advance int32
	if len(hhea) >= 36 && hmtx != nil {
		n := int(binary.BigEndian.Uint16(hhea[34:]))
		if n*4 > len(hmtx) {
			n = len(hmtx) / 4
		}
		switch {
		case int(glyph) < n:
			advance = int32(binary.BigEndian.Uint16(hmtx[int(glyph)*4:]))
		case n > 0:
			advance = int32(binary.BigEndian.Uint16(hmtx[(n-1)*4:]))
		}
	}
	if len(f.info.location) != 0 && (f.ld.HasTable(ot.MustNewTag("HVAR")) || f.ld.HasTable(ot.MustNewTag("gvar"))) {
		f.mu.Lock()
		varied := f.face.HorizontalAdvance(tsfont.GID(glyph))
		f.mu.Unlock()
		// skrifa computes the delta as a Fixed of 16.16 bits. The port
		// rounds the delta of typesetting to the nearest such value first,
		// so that a delta that is an integer in skrifa is one here too. The
		// compiler turns the division into a multiply, and the conversion
		// stops it from fusing that multiply with the add below (D16).
		delta := float64(math.Round((float64(varied)-float64(advance))*65536) / 65536)
		if f.ld.HasTable(ot.MustNewTag("HVAR")) {
			advance += int32(delta)
		} else {
			advance += int32(math.Floor(delta + 0.5))
		}
	}
	return float32(advance), true
}

// fontInfo is FontInfo in krilla src/text/font.rs.
//
// FontInfo holds basic information about the font which is necessary to
// distinguish a Font object from others. The Hash implementation of the Font
// struct solely depends on its FontInfo object. The reason we do this is to
// avoid hashing the whole font, which can be dozens of megabytes. Instead,
// we parse the most basic information as well as additional distinguishing
// information, such as the font name and the checksum, and hash this
// instead.
type fontInfo struct {
	index    uint32
	checksum uint32
	dataLen  int
	// varCoords holds the coordinates of the variations, which the subset
	// takes. location is derived from them.
	varCoords         []VariationCoord
	location          []tables.Coord
	unitsPerEm        uint16
	globalBBox        Rect
	numGlyphs         uint32
	postscriptName    string
	hasPostscriptName bool
	isAppleColorEmoji bool
	ascent            float32
	descent           float32
	capHeight         float32
	hasCapHeight      bool
	isMonospaced      bool
	italicAngle       float32
	weight            float32
	hasGlyf           bool
	hasCFF            bool
	hasCFF2           bool
	stretch           float32
}

// typeID is the name of fontInfo for sipHash.
func (*fontInfo) typeID() string { return "FontInfo" }

// hash is the derived Hash of FontInfo in krilla src/text/font.rs. Only a
// key of a cache takes it, so it writes the fields in their order without
// the exact layout of Rust.
func (i *fontInfo) hash(h *hasher) {
	h.u32(i.index)
	h.u32(i.checksum)
	h.usize(i.dataLen)
	h.usize(len(i.varCoords))
	for _, c := range i.varCoords {
		h.write(c.Tag[:])
		h.u32(math.Float32bits(c.Value))
	}
	h.usize(len(i.location))
	for _, c := range i.location {
		h.u16(uint16(c))
	}
	h.u16(i.unitsPerEm)
	h.rect(i.globalBBox)
	h.u32(i.numGlyphs)
	h.bool(i.hasPostscriptName)
	h.str(i.postscriptName)
	h.bool(i.isAppleColorEmoji)
	h.u32(math.Float32bits(i.ascent))
	h.u32(math.Float32bits(i.descent))
	h.bool(i.hasCapHeight)
	h.u32(math.Float32bits(i.capHeight))
	h.bool(i.isMonospaced)
	h.u32(math.Float32bits(i.italicAngle))
	h.u32(math.Float32bits(i.weight))
	h.bool(i.hasGlyf)
	h.bool(i.hasCFF)
	h.bool(i.hasCFF2)
	h.u32(math.Float32bits(i.stretch))
}

// newFontInfo is FontInfo::new in krilla src/text/font.rs. It returns false
// where upstream returns None: for a font without a valid table head, or
// with a bounding box that is not valid and a units per em of 0.
func newFontInfo(f *Font, index uint32, dataLen int, varCoords []VariationCoord) (*fontInfo, bool) {
	head := f.table("head")
	if len(head) < 54 {
		return nil, false
	}
	checksum := binary.BigEndian.Uint32(head[8:])
	location := f.face.Coords()
	if allCoordsZero(location) {
		location = nil
	}
	var numGlyphs uint32
	if maxp := f.table("maxp"); len(maxp) >= 6 {
		numGlyphs = uint32(binary.BigEndian.Uint16(maxp[4:]))
	}

	m := newSkrifaMetrics(f, location)
	os2, hasOS2 := readOS2(f.table("OS/2"))
	ascent, descent := m.ascent, m.descent
	if hasOS2 {
		ascent = float32(os2.typoAscender)
		descent = float32(os2.typoDescender)
	}
	weight, stretch := skrifaAttributes(f, os2, hasOS2)
	unitsPerEm := m.unitsPerEm

	globalBBox, ok := Rect{}, false
	if m.hasBounds {
		globalBBox, ok = path.RectFromLTRB(m.xMin, m.yMin, m.xMax, m.yMax)
	}
	if !ok {
		globalBBox, ok = path.RectFromXYWH(0, 0, float32(unitsPerEm), float32(unitsPerEm))
		if !ok {
			return nil, false
		}
	}

	psName, hasPSName := postscriptNameOf(f.table("name"))
	isAppleColorEmoji := bitmapStrikesFormat(f) == bitmapFormatSbix &&
		localizedNameIs(f.table("name"), 6, "AppleColorEmoji")

	coords := make([]VariationCoord, len(varCoords))
	for i, c := range varCoords {
		v := c.Value
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			v = 0
		}
		coords[i] = VariationCoord{Tag: c.Tag, Value: v}
	}

	return &fontInfo{
		index:             index,
		dataLen:           dataLen,
		checksum:          checksum,
		varCoords:         coords,
		location:          location,
		numGlyphs:         numGlyphs,
		unitsPerEm:        unitsPerEm,
		postscriptName:    psName,
		hasPostscriptName: hasPSName,
		isAppleColorEmoji: isAppleColorEmoji,
		ascent:            ascent,
		capHeight:         m.capHeight,
		hasCapHeight:      m.hasCapHeight,
		hasGlyf:           f.ld.HasTable(ot.MustNewTag("glyf")),
		hasCFF:            f.ld.HasTable(ot.MustNewTag("CFF ")),
		hasCFF2:           f.ld.HasTable(ot.MustNewTag("CFF2")),
		descent:           descent,
		isMonospaced:      m.isMonospace,
		weight:            weight,
		stretch:           stretch,
		italicAngle:       m.italicAngle,
		globalBBox:        globalBBox,
	}, true
}

// canBeCIDFont is FontInfo::can_be_cid_font in krilla src/text/font.rs.
func (i *fontInfo) canBeCIDFont() bool {
	return i.hasCFF || i.hasGlyf || i.hasCFF2
}

// allCoordsZero reports whether each coordinate is 0, which
// LocationRef::effective_coords of skrifa takes as the default location.
func allCoordsZero(coords []tables.Coord) bool {
	for _, c := range coords {
		if c != 0 {
			return false
		}
	}
	return true
}

// os2Table holds the fields of the table OS/2 that skrifa reads.
type os2Table struct {
	weightClass   uint16
	widthClass    uint16
	fsType        uint16
	fsSelection   uint16
	typoAscender  int16
	typoDescender int16
	typoLineGap   int16
	winAscent     uint16
	winDescent    uint16
	capHeight     int16
	hasCapHeight  bool
}

// readOS2 reads the table OS/2 as read-fonts does. read-fonts reads a table
// of the version 0 from 78 bytes, of the version 1 from 86 bytes, and of a
// later version from 96 bytes, and the field sCapHeight from the version 2.
func readOS2(b []byte) (os2Table, bool) {
	if len(b) < 78 {
		return os2Table{}, false
	}
	version := binary.BigEndian.Uint16(b)
	switch {
	case version == 1 && len(b) < 86:
		return os2Table{}, false
	case version >= 2 && len(b) < 96:
		return os2Table{}, false
	}
	u16 := func(off int) uint16 { return binary.BigEndian.Uint16(b[off:]) }
	t := os2Table{
		weightClass:   u16(4),
		widthClass:    u16(6),
		fsType:        u16(8),
		fsSelection:   u16(62),
		typoAscender:  int16(u16(68)),
		typoDescender: int16(u16(70)),
		typoLineGap:   int16(u16(72)),
		winAscent:     u16(74),
		winDescent:    u16(76),
	}
	if version >= 2 {
		t.capHeight = int16(u16(88))
		t.hasCapHeight = true
	}
	return t, true
}

// skrifaMetrics holds the fields of Metrics of skrifa src/metrics.rs that
// krilla reads, at the size unscaled.
type skrifaMetrics struct {
	unitsPerEm   uint16
	isMonospace  bool
	italicAngle  float32
	ascent       float32
	descent      float32
	capHeight    float32
	hasCapHeight bool
	hasBounds    bool
	xMin         float32
	yMin         float32
	xMax         float32
	yMax         float32
}

// newSkrifaMetrics is Metrics::new of skrifa src/metrics.rs, with the size
// unscaled, at the normalized coordinates of location.
func newSkrifaMetrics(f *Font, location []tables.Coord) skrifaMetrics {
	var m skrifaMetrics
	if head := f.table("head"); len(head) >= 54 {
		m.unitsPerEm = binary.BigEndian.Uint16(head[18:])
		m.hasBounds = true
		m.xMin = float32(int16(binary.BigEndian.Uint16(head[36:])))
		m.yMin = float32(int16(binary.BigEndian.Uint16(head[38:])))
		m.xMax = float32(int16(binary.BigEndian.Uint16(head[40:])))
		m.yMax = float32(int16(binary.BigEndian.Uint16(head[42:])))
	}
	if post := f.table("post"); len(post) >= 32 {
		m.isMonospace = binary.BigEndian.Uint32(post[12:]) != 0
		m.italicAngle = float32(float64(int32(binary.BigEndian.Uint32(post[4:]))) / 65536)
	}
	// Choosing proper line metrics is a challenge due to the changing spec,
	// backward compatibility and broken fonts.
	//
	// We use the same strategy as FreeType:
	// 1. Use the OS/2 metrics if the table exists and the USE_TYPO_METRICS
	//    flag is set.
	// 2. Otherwise, use the hhea metrics.
	// 3. If hhea metrics are zero and the OS/2 table exists:
	//    3a. Use the typo metrics if they are non-zero
	//    3b. Otherwise, use the win metrics
	os2, hasOS2 := readOS2(f.table("OS/2"))
	usedTypoMetrics := false
	if hasOS2 {
		const useTypoMetrics = 1 << 7
		if os2.fsSelection&useTypoMetrics != 0 {
			m.ascent = float32(os2.typoAscender)
			m.descent = float32(os2.typoDescender)
			usedTypoMetrics = true
		}
		m.capHeight, m.hasCapHeight = float32(os2.capHeight), os2.hasCapHeight
	}
	if !usedTypoMetrics {
		if hhea := f.table("hhea"); len(hhea) >= 36 {
			m.ascent = float32(int16(binary.BigEndian.Uint16(hhea[4:])))
			m.descent = float32(int16(binary.BigEndian.Uint16(hhea[6:])))
		}
		if m.ascent == 0 && m.descent == 0 && hasOS2 {
			if os2.typoAscender != 0 || os2.typoDescender != 0 {
				m.ascent = float32(os2.typoAscender)
				m.descent = float32(os2.typoDescender)
			} else {
				m.ascent = float32(os2.winAscent)
				// Win descent is always positive while other descent values
				// are negative. Negate it to ensure we return consistent
				// metrics.
				m.descent = -float32(os2.winDescent)
			}
		}
	}
	if len(location) != 0 {
		if raw := f.table("MVAR"); raw != nil {
			if mvar, _, err := tables.ParseMVAR(raw); err == nil {
				delta := func(tag string) float32 {
					t := ot.MustNewTag(tag)
					for _, r := range mvar.ValueRecords {
						if r.ValueTag == t {
							return mvar.ItemVariationStore.GetDelta(r.Index, location)
						}
					}
					return 0
				}
				m.ascent += delta("hasc")
				m.descent += delta("hdsc")
				if m.hasCapHeight {
					m.capHeight += delta("cpht")
				}
			}
		}
	}
	return m
}

// skrifaAttributes is Attributes::new of skrifa src/attribute.rs. It returns
// the weight and the ratio of the stretch of the font.
func skrifaAttributes(f *Font, os2 os2Table, hasOS2 bool) (float32, float32) {
	if hasOS2 {
		stretch := float32(2)
		switch os2.widthClass {
		case 0, 1:
			stretch = 0.5
		case 2:
			stretch = 0.625
		case 3:
			stretch = 0.75
		case 4:
			stretch = 0.875
		case 5:
			stretch = 1
		case 6:
			stretch = 1.125
		case 7:
			stretch = 1.25
		case 8:
			stretch = 1.5
		}
		return float32(os2.weightClass), stretch
	}
	if head := f.table("head"); len(head) >= 54 {
		const bold = 1 << 0
		if binary.BigEndian.Uint16(head[44:])&bold != 0 {
			return 700, 1
		}
	}
	return 400, 1
}

// nameRecords calls fn for each record of the table name, in the order of
// the table, with the decoded string of the record, as
// NameRecord::string of read-fonts reads it. It stops when fn returns
// false. A record whose string is outside the table is skipped.
func nameRecords(b []byte, fn func(nameID uint16, s string) bool) {
	if len(b) < 6 {
		return
	}
	count := int(binary.BigEndian.Uint16(b[2:]))
	storage := int(binary.BigEndian.Uint16(b[4:]))
	if 6+count*12 > len(b) {
		return
	}
	var data []byte
	if storage <= len(b) {
		data = b[storage:]
	}
	for i := range count {
		r := b[6+i*12:]
		platformID := binary.BigEndian.Uint16(r)
		encodingID := binary.BigEndian.Uint16(r[2:])
		nameID := binary.BigEndian.Uint16(r[6:])
		length := int(binary.BigEndian.Uint16(r[8:]))
		offset := int(binary.BigEndian.Uint16(r[10:]))
		if offset+length > len(data) {
			continue
		}
		if !fn(nameID, decodeName(platformID, encodingID, data[offset:offset+length])) {
			return
		}
	}
}

// decodeName is the Display of NameString of read-fonts: the text of a name
// in UTF-16BE or Mac Roman, or no text for another encoding.
func decodeName(platformID, encodingID uint16, b []byte) string {
	utf16BE := platformID == 0 || (platformID == 3 && (encodingID == 0 || encodingID == 1 || encodingID == 10))
	var out []rune
	switch {
	case utf16BE:
		for i := 0; i+2 <= len(b); i += 2 {
			c1 := binary.BigEndian.Uint16(b[i:])
			if c1 >= 0xD800 && c1 < 0xDC00 {
				if i+4 > len(b) {
					out = append(out, unicode.ReplacementChar)
					break
				}
				c2 := binary.BigEndian.Uint16(b[i+2:])
				if c2 < 0xDC00 || c2 > 0xDFFF {
					out = append(out, unicode.ReplacementChar)
					continue
				}
				out = append(out, utf16.DecodeRune(rune(c1), rune(c2)))
				i += 2
				continue
			}
			if c1 >= 0xDC00 && c1 <= 0xDFFF {
				out = append(out, unicode.ReplacementChar)
				continue
			}
			out = append(out, rune(c1))
		}
	case platformID == 1 && encodingID == 0:
		for _, c := range b {
			out = append(out, tables.DecodeMacintoshByte(c))
		}
	}
	return string(out)
}

// postscriptNameOf is the search of the PostScript name in FontInfo::new in
// krilla src/text/font.rs: the first record with the name ID 6 whose string
// read-fonts can read.
func postscriptNameOf(b []byte) (string, bool) {
	var name string
	found := false
	nameRecords(b, func(nameID uint16, s string) bool {
		if nameID == 6 {
			name, found = s, true
			return false
		}
		return true
	})
	return name, found
}

// localizedNameIs is the test of is_apple_color_emoji in FontInfo::new in
// krilla src/text/font.rs: whether a localized string of the name ID is the
// text. skrifa takes the records of the name ID whose encoding it reads.
func localizedNameIs(b []byte, id uint16, text string) bool {
	found := false
	nameRecords(b, func(nameID uint16, s string) bool {
		if nameID == id && s == text {
			found = true
			return false
		}
		return true
	})
	return found
}
