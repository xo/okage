// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"errors"
	"fmt"
	"math/bits"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/internal/subsetter"
	"github.com/xo/okage/path"
	ot "github.com/xo/okage/typesetting/font/opentype"
)

// This file ports krilla src/text/cid.rs.

// subsetTagLen is SUBSET_TAG_LEN in krilla src/text/cid.rs.
const subsetTagLen = 6

// identityH is IDENTITY_H in krilla src/text/cid.rs.
const identityH = "Identity-H"

// cmapName is CMAP_NAME in krilla src/text/cid.rs.
const cmapName = pdfwriter.Name("Custom")

// systemInfo is SYSTEM_INFO in krilla src/text/cid.rs.
var systemInfo = pdfwriter.SystemInfo{
	Registry:   pdfwriter.Str("Adobe"),
	Ordering:   pdfwriter.Str("Identity"),
	Supplement: 0,
}

// cmapEntry is the pair (String, Option<Location>) of the entries of the
// map of the codepoints of a font in krilla.
type cmapEntry struct {
	text     string
	location Location
}

// writeCmapEntry is write_cmap_entry in krilla src/text/cid.rs.
//
// A shared function for CID fonts and Type3 fonts to write the cmap entries.
func writeCmapEntry[G uint8 | uint16](font *Font, entry *cmapEntry, sc *serializeContext, cmap *pdfwriter.UnicodeCmap[G], g G) {
	if entry == nil {
		sc.registerValidationError(validationError{kind: validationNoCodepointMapping, font: font.hash, glyph: uint32(g)})
		return
	}
	text, loc := entry.text, entry.location
	invalidCodepoint := text == ""
	var invalidCode rune
	hasInvalidCode := false
	var privateUnicode rune
	hasPrivateUnicode := false

	for _, c := range text {
		switch c {
		case 0x0, 0xFEFF, 0xFFFE:
			invalidCode, hasInvalidCode = c, true
			invalidCodepoint = true
		}
		if (c >= 0xE000 && c <= 0xF8FF) || (c >= 0xF0000 && c <= 0xFFFFD) || (c >= 0x100000 && c <= 0x10FFFD) {
			privateUnicode, hasPrivateUnicode = c, true
		}
	}

	switch {
	case hasInvalidCode:
		sc.registerValidationError(validationError{kind: validationInvalidCodepointMapping, font: font.hash, glyph: uint32(g), char: invalidCode, location: loc})
	case invalidCodepoint:
		sc.registerValidationError(validationError{kind: validationNoCodepointMapping, font: font.hash, glyph: uint32(g), location: loc})
	}

	if hasPrivateUnicode {
		sc.registerValidationError(validationError{kind: validationUnicodePrivateArea, font: font.hash, glyph: uint32(g), char: privateUnicode, location: loc})
	}

	if text != "" {
		cmap.PairWithMultiple(g, []rune(text)...)
	}
}

// cidFont is CIDFont in krilla src/text/cid.rs.
//
// A CID-keyed font.
type cidFont struct {
	// f is the actual underlying OTF font of the CID-keyed font.
	f *Font
	// glyphRemapper maps GIDs from the original font to CIDs, i.e. the
	// corresponding GID in the font subset. The subsetter will ensure that
	// for CID-keyed CFF fonts, the CID-to-GID mapping will be the identity
	// mapping, regardless of what the mapping was in the original font. This
	// allows us to index both, CFF and glyf-based fonts, transparently using
	// GIDs, instead of having to distinguish according to the underlying
	// font. See section 9.7.4.2 for more information on how glyphs are
	// indexed in a CID-keyed font.
	glyphRemapper *subsetter.GlyphRemapper
	// cmapEntries is a mapping from CIDs to their string in the original
	// text.
	cmapEntries map[uint16]cmapEntry
	// widths holds the widths of the glyphs, indexed by their CID.
	widths []float32
	// empty is true until the first glyph.
	empty bool
}

// newCIDFont is CIDFont::new in krilla src/text/cid.rs.
//
// Create a new CID-keyed font.
func newCIDFont(font *Font) *cidFont {
	// Always include the .notdef glyph. Will also always be included by the
	// subsetter in the glyph remapper.
	w, _ := font.advanceWidth(0)
	return &cidFont{
		glyphRemapper: subsetter.NewGlyphRemapper(),
		cmapEntries:   map[uint16]cmapEntry{},
		widths:        []float32{w},
		f:             font,
		empty:         true,
	}
}

// font is CIDFont::font in krilla src/text/cid.rs, and PdfFont::font.
func (f *cidFont) font() *Font {
	return f.f
}

// unitsPerEm is CIDFont::units_per_em in krilla src/text/cid.rs, and
// PdfFont::units_per_em.
//
// Note that this refers to the units per em in PDF (which is always 1000),
// and not the units per em of the underlying font.
func (*cidFont) unitsPerEm() float32 {
	return pdfUnitsPerEm
}

// forceFill is PdfFont::force_fill for CIDFont in krilla src/text/mod.rs.
func (*cidFont) forceFill() bool {
	return false
}

// getCID is CIDFont::get_cid in krilla src/text/cid.rs.
func (f *cidFont) getCID(glyph GlyphID) (uint16, bool) {
	return f.glyphRemapper.Get(uint16(glyph))
}

// getGID is PdfFont::get_gid for CIDFont in krilla src/text/mod.rs.
func (f *cidFont) getGID(glyph coloredGlyph) (pdfGlyph, bool) {
	cid, ok := f.getCID(glyph.glyphID)
	return pdfGlyph{cid: cid}, ok
}

// addGlyph is CIDFont::add_glyph in krilla src/text/cid.rs. The caller
// gives a glyph ID below 65536, which krilla unwraps.
//
// Add a new glyph (if it has not already been added) and return its CID.
func (f *cidFont) addGlyph(glyph GlyphID) uint16 {
	f.empty = false

	newID := f.glyphRemapper.Remap(uint16(glyph))

	// This means that the glyph ID has been newly assigned, and thus we
	// need to add its width.
	if int(newID) >= len(f.widths) {
		w, _ := f.f.advanceWidth(glyph)
		f.widths = append(f.widths, w)
	}

	return newID
}

// getCodepoints is CIDFont::get_codepoints in krilla src/text/cid.rs, and
// PdfFont::get_codepoints.
func (f *cidFont) getCodepoints(g pdfGlyph) (string, bool) {
	if g.isType3 {
		return "", false
	}
	e, ok := f.cmapEntries[g.cid]
	return e.text, ok
}

// setCodepoints is CIDFont::set_codepoints in krilla src/text/cid.rs, and
// PdfFont::set_codepoints.
func (f *cidFont) setCodepoints(g pdfGlyph, text string, location Location) {
	if g.isType3 {
		return
	}
	f.cmapEntries[g.cid] = cmapEntry{text: text, location: location}
}

// identifier is CIDFont::identifier in krilla src/text/cid.rs.
func (f *cidFont) identifier() fontIdentifier {
	return cidIdentifier(f.f)
}

// serialize is CIDFont::serialize in krilla src/text/cid.rs.
func (f *cidFont) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) error {
	chunk := cc.nonStream.fonts
	streamChunk := sc.newChunk()

	cidRef := sc.newRef()
	descriptorRef := sc.newRef()
	cmapRef := sc.newRef()
	cidSetRef := sc.newRef()
	dataRef := sc.newRef()

	glyphRemapper := f.glyphRemapper

	isGlyf := f.f.info.hasGlyf
	isCFF := f.f.info.hasCFF
	isCFF2 := f.f.info.hasCFF2

	// okage: when the font cannot be embedded, the fallback of D80 writes
	// the font without its font program, and Document.Finish draws its
	// glyphs as paths.
	var embedErr error
	if !isGlyf && !isCFF && !isCFF2 {
		embedErr = fontError(f.f, "font is missing an outline table")
	}

	// OS/2 fsType has a few bits that describe which kind of license the
	// font has. Of particular interest is bit 2 "Restricted License
	// embedding", which places restrictions on font embedding that are
	// incompatible with some PDF standards.
	//
	// The OpenType spec intended for the different bits to be mutually
	// exclusive. However, some fonts have multiple of the bits sets. The
	// OpenType spec was thus adjusted to require mutual exclusion, allowing
	// applications to assume the least-restrictive specified variant.
	// Specifically, it also clarifies that "For Restricted License embedding
	// to take effect, the Embedding permissions sub-field must have the
	// value 2 (that is, only bit 1 is set)."
	//
	// Hence, we are not checking `fsType & 2 != 0` (as would be usual for
	// bit flags), but rather `fsType & 0xF == 2`.
	if os2, ok := readOS2(f.f.table("OS/2")); embedErr == nil && ok && os2.fsType&0xF == 2 {
		sc.registerValidationError(validationError{kind: validationRestrictedLicense, font: f.f.hash})
	}

	var subsetted *Font
	var globalBBox Rect
	if embedErr == nil {
		subsetted, globalBBox, embedErr = subsetFont(f.f, glyphRemapper)
	}
	embedded := embedErr == nil
	var numGlyphs uint32
	var fontStream filterStream
	var baseFont string
	if embedded {
		numGlyphs = subsetted.numGlyphs()

		// If we have a CFF font, only embed the standalone CFF program.
		data := subsetted.fontData()
		if subsetted.ld.HasTable(ot.MustNewTag("CFF ")) {
			data = subsetted.table("CFF ")
		}
		fontStream = newFilterStreamFromBinaryData(data).finish(sc.serializeSettings())

		baseFont = cidBaseFontName(f.f, glyphRemapper)
	} else {
		if !sc.serializeSettings().subsetFallback {
			return embedErr
		}
		sc.failFont(f, rootRef, embedErr)
		globalBBox = f.f.bbox()
		baseFont = cidFontName(f.f)
	}
	baseFontType0 := baseFont
	if isCFF {
		baseFontType0 = baseFont + "-" + identityH
	}

	chunk.Type0Font(rootRef).
		BaseFont(pdfwriter.Name(baseFontType0)).
		EncodingPredefined(pdfwriter.Name(identityH)).
		DescendantFont(cidRef).
		ToUnicode(cmapRef).
		Finish()

	cid := chunk.CIDFont(cidRef)
	if isCFF {
		cid.Subtype(pdfwriter.CIDFontType0)
	} else {
		cid.Subtype(pdfwriter.CIDFontType2)
	}
	cid.BaseFont(pdfwriter.Name(baseFont))
	cid.SystemInfo(systemInfo)
	cid.FontDescriptor(descriptorRef)
	cid.DefaultWidth(0.0)

	if !isCFF {
		cid.CIDToGIDMapPredefined(pdfwriter.Name("Identity"))
	}

	// In CID fonts, a upem value of 1000 is assumed for all fonts, so we need
	// to convert.
	toPDFUnits := func(v float32) float32 {
		return v / f.f.UnitsPerEm() * f.unitsPerEm()
	}

	first := 0
	widthWriter := cid.Widths()
	for w, group := range groupByKey(f.widths, func(w float32) float32 { return w }) {
		end := first + len(group)
		if w != 0.0 {
			last := end - 1
			widthWriter.Same(uint16(first), uint16(last), toPDFUnits(w))
		}
		first = end
	}

	widthWriter.Finish()
	cid.Finish()

	// The only reason we write this in the first place is that PDF/A-1b
	// requires a CIDSet.
	if embedded && !sc.serializeSettings().pdfVersion().deprecatesCIDSet() {
		// It's always guaranteed by the subsetter that CIDs start from 0 and
		// are consecutive, so this encoding is very straight-forward.
		cidStreamData := make([]byte, 0, numGlyphs/8+1)
		for range numGlyphs / 8 {
			cidStreamData = append(cidStreamData, 0xFF)
		}
		if padding := numGlyphs % 8; padding != 0 {
			cidStreamData = append(cidStreamData, ^byte(0xFF>>padding))
		}

		cidStream := newFilterStreamFromBinaryData(cidStreamData).finish(sc.serializeSettings())
		cidSet := streamChunk.Stream(cidSetRef, cidStream.encodedData())
		cidStream.writeFilters(cidSet.Dict)
		cidSet.Finish()
	}

	var flags pdfwriter.FontFlags
	if name, ok := f.f.postscriptName(); ok && containsSerif(name) {
		flags |= pdfwriter.FontFlagsSerif
	}
	if f.f.isMonospaced() {
		flags |= pdfwriter.FontFlagsFixedPitch
	}
	if f.f.italicAngle() != 0.0 {
		flags |= pdfwriter.FontFlagsItalic
	}
	flags |= pdfwriter.FontFlagsSymbolic
	flags |= pdfwriter.FontFlagsSmallCap

	bboxRect, _ := rectFromLTRBUnwrap(
		toPDFUnits(globalBBox.Left()),
		toPDFUnits(globalBBox.Top()),
		toPDFUnits(globalBBox.Right()),
		toPDFUnits(globalBBox.Bottom()),
	)
	bbox := rectToPDFRect(bboxRect)

	italicAngle := f.f.italicAngle()
	ascender := toPDFUnits(f.f.ascent())
	descender := toPDFUnits(f.f.descent())
	capHeight := ascender
	if c, ok := f.f.capHeight(); ok {
		capHeight = toPDFUnits(c)
	}
	stemV := 10.0 + float32(0.244*(f.f.weight()-50.0))

	fontDescriptor := chunk.FontDescriptor(descriptorRef)
	fontDescriptor.
		Name(pdfwriter.Name(baseFont)).
		Flags(flags).
		BBox(bbox).
		ItalicAngle(italicAngle).
		Ascent(ascender).
		Descent(descender).
		CapHeight(capHeight).
		StemV(stemV)

	if embedded && !sc.serializeSettings().pdfVersion().deprecatesCIDSet() {
		fontDescriptor.CIDSet(cidSetRef)
	}

	switch {
	case !embedded:
	case isCFF:
		fontDescriptor.FontFile3(dataRef)
	default:
		fontDescriptor.FontFile2(dataRef)
	}

	fontDescriptor.Finish()

	cmap := pdfwriter.NewUnicodeCmap[uint16](cmapName, systemInfo)

	// For the .notdef glyph, it's fine if no mapping exists, since it is
	// included even if it was not referenced in the text.
	for g := uint16(1); g < glyphRemapper.NumGids(); g++ {
		var entry *cmapEntry
		if e, ok := f.cmapEntries[g]; ok {
			entry = &e
		}
		writeCmapEntry(f.f, entry, sc, cmap, g)
	}

	cmapBuf := cmap.Finish()
	cmapStream := newFilterStreamFromContentStream(cmapBuf.AsSlice(), sc.serializeSettings()).finish(sc.serializeSettings())
	cmapWriter := streamChunk.Cmap(cmapRef, cmapStream.encodedData())
	cmapStream.writeFilters(cmapWriter.Dict)
	cmapWriter.WritingMode(pdfwriter.WModeHorizontal)
	cmapWriter.Finish()

	if embedded {
		stream := streamChunk.Stream(dataRef, fontStream.encodedData())
		fontStream.writeFilters(stream.Dict)
		if isCFF {
			stream.Pair(pdfwriter.Name("Subtype"), pdfwriter.Name("CIDFontType0C"))
		}

		stream.Finish()
	}
	cc.streams.fonts = append(cc.streams.fonts, streamChunk)

	return nil
}

// fontError is KrillaError::Font of krilla src/error.rs. It wraps
// ErrPDFFont with the PostScript name of the font, when the font has one,
// and the message, as the impl Display for KrillaError writes them.
func fontError(font *Font, message string) error {
	if name, ok := font.postscriptName(); ok {
		return fmt.Errorf("%w %s: %s", okage.ErrPDFFont, name, message)
	}
	return fmt.Errorf("%w: %s", okage.ErrPDFFont, message)
}

// containsSerif is the test of the flag SERIF of krilla: whether the
// PostScript name contains "Serif".
func containsSerif(name string) bool {
	for i := 0; i+5 <= len(name); i++ {
		if name[i:i+5] == "Serif" {
			return true
		}
	}
	return false
}

// rectFromLTRBUnwrap is Rect::from_ltrb(...).unwrap() of krilla. krilla
// panics for a rectangle that is not valid, which a font whose units per
// em is 0 gives. The port then takes the rectangle from 0 to 1.
func rectFromLTRBUnwrap(left, top, right, bottom float32) (Rect, bool) {
	if r, ok := path.RectFromLTRB(left, top, right, bottom); ok {
		return r, true
	}
	r, _ := path.RectFromLTRB(0, 0, 1, 1)
	return r, false
}

// remapperHasher gives the methods of subsetter.Hasher to a hasher. It
// writes a usize as a u64, as StableHasher of stable_hash128 does.
type remapperHasher struct {
	h *hasher
}

// WriteUsize is Hasher::write_usize of StableHasher.
func (r remapperHasher) WriteUsize(i uint) {
	r.h.usize(int(i))
}

// Write is Hasher::write of StableHasher.
func (r remapperHasher) Write(b []byte) {
	r.h.write(b)
}

// subsetTag is subset_tag in krilla src/text/cid.rs, for the glyph
// remapper that krilla hashes.
//
// Create a tag for a font subset.
func subsetTag(m *subsetter.GlyphRemapper) string {
	const base = 26
	hi, lo := stableHash128(func(h *hasher) { m.Hash(remapperHasher{h}) }).AsU128()
	var letters [subsetTagLen]byte
	for i := range letters {
		var r uint64
		hi, r = bits.Div64(0, hi, base)
		lo, r = bits.Div64(r, lo, base)
		letters[i] = 'A' + byte(r)
	}
	return string(letters[:])
}

// cidBaseFontName is base_font_name in krilla src/text/cid.rs.
func cidBaseFontName(font *Font, m *subsetter.GlyphRemapper) string {
	tag := subsetTag(m)

	return tag + "+" + cidFontName(font)
}

// cidFontName is the part of base_font_name in krilla src/text/cid.rs that
// trims the PostScript name of the font. The name of a font without its
// font program has no tag, because the file has no subset of it (D80).
func cidFontName(font *Font) string {
	const restLen = subsetTagLen + 1 + 1 + len(identityH)

	postscriptName, ok := font.postscriptName()
	if !ok {
		postscriptName = "unknown"
	}
	const maxLen = 127 - restLen
	return postscriptName[:min(len(postscriptName), maxLen)]
}

// subsetFont is subset_font in krilla src/text/cid.rs. It returns the font
// of the subset and the bounding box of its glyphs.
func subsetFont(font *Font, glyphRemapper *subsetter.GlyphRemapper) (*Font, Rect, error) {
	variationCoordinates := make([]subsetter.Variation, len(font.variationCoordinates()))
	for i, v := range font.variationCoordinates() {
		variationCoordinates[i] = subsetter.Variation{Tag: subsetter.NewTag(v.Tag), Value: v.Value}
	}
	data, err := subsetter.SubsetWithVariations(font.fontData(), font.index(), variationCoordinates, glyphRemapper)
	if err != nil {
		return nil, Rect{}, fmt.Errorf("%w: %w", fontError(font, "failed to subset font"), err)
	}
	subsetted, err := NewFont(data, 0)
	if err != nil {
		if errors.Is(err, okage.ErrPDFInvalidFont) {
			return nil, Rect{}, fontError(font, "failed to subset font")
		}
		return nil, Rect{}, err
	}
	globalBBox := subsetted.bbox()

	var bbox Rect
	hasBBox := false
	for g := range subsetted.numGlyphs() {
		if pathBBox, ok := computeBBox(subsetted, GlyphID(g)); ok {
			if hasBBox {
				rectExpand(&bbox, pathBBox)
			} else {
				bbox, hasBBox = pathBBox, true
			}
		}
	}
	if !hasBBox {
		bbox = globalBBox
	}

	return subsetted, bbox, nil
}

// computeBBox is compute_bbox in krilla src/text/cid.rs. The subset has no
// variations, so its location is the default.
func computeBBox(font *Font, glyph GlyphID) (Rect, bool) {
	b := newOutlineBuilder()
	if !font.drawSkrifaOutline(glyph, b) {
		return Rect{}, false
	}
	p, ok := b.finish()
	if !ok {
		return Rect{}, false
	}
	return p.Bounds(), true
}

// isEmpty is CIDFont::is_empty in krilla src/text/cid.rs.
func (f *cidFont) isEmpty() bool {
	return f.empty
}
