package svg

import (
	"github.com/xo/okage/typesetting/font"
)

// This file ports no upstream file. It gives a program the faces that the
// database parses, their character maps and their metrics, so that a program
// that shapes text itself with typesetting does not parse a face again
// (D57).

// FaceFont returns the font of the face id, as go-text parses it. The
// database parses each font file once, on first use, and keeps the result.
// The layout of text of svg reads the same font. A *font.Font is safe for
// concurrent use. FaceFont returns false when the face does not exist or
// go-text cannot parse it.
func (db *FontDatabase) FaceFont(id FontID) (*font.Font, bool) {
	pf, ok := db.withFaceData(id)
	if !ok {
		return nil, false
	}
	return pf.font, true
}

// NewFace returns a new face of go-text for the face id, at the values of the
// axes in variations. It keeps only the axes that the font has, and limits
// each value to the range of its axis, as the layout of text of svg does for
// a font instance. AutoFontVariations gives the axes of a CSS weight,
// stretch and style.
//
// The face shares the font that FaceFont returns, so NewFace parses nothing.
// A *font.Face is not safe for concurrent use, so each call returns a new
// face, which the caller can keep. NewFace returns false as FaceFont does.
func (db *FontDatabase) NewFace(id FontID, variations []FontVariation) (*font.Face, bool) {
	pf, ok := db.withFaceData(id)
	if !ok {
		return nil, false
	}
	face := font.NewFace(pf.font)
	if key := filterVariations(pf, variations); len(key) != 0 {
		face.SetVariations(toGoTextVariations(key))
	}
	return face, true
}

// NominalGlyph returns the glyph of the character c in the face id. It reads
// the character map that the database keeps for each font file. The database
// reads that map once, from the tables cmap and OS/2 only, and it does not
// parse the whole face. NominalGlyph returns false when the face does not
// exist or has no glyph for c. DefaultFallbackSelector looks up each
// character with it.
func (db *FontDatabase) NominalGlyph(id FontID, c rune) (GlyphID, bool) {
	cmap, ok := db.faceCharmap(id)
	if !ok {
		return 0, false
	}
	g, ok := cmap.Lookup(c)
	return GlyphID(g), ok
}

// FontMetrics holds the metrics of a face that the layout of text of svg
// uses, in font units. A value of y grows up from the baseline, as in the
// font.
type FontMetrics struct {
	// UnitsPerEm is the number of font units in one em.
	UnitsPerEm uint16

	// Ascent is the height above the baseline, and Descent the depth below
	// it, which is negative. The layout puts the box of a line of text from
	// Descent to Ascent.
	Ascent  int16
	Descent int16

	// XHeight is the height of the small letters. A font that does not give
	// it gets 45% of the height from Descent to Ascent.
	XHeight uint16

	// UnderlinePosition is the position of the underline, and
	// UnderlineThickness is its thickness.
	UnderlinePosition  int16
	UnderlineThickness uint16

	// LineThroughPosition is the position of the line through the text.
	LineThroughPosition int16

	// SubscriptOffset and SuperscriptOffset are the shifts of the baseline
	// of a subscript and of a superscript.
	SubscriptOffset   int16
	SuperscriptOffset int16
}

// FaceMetrics returns the metrics of the face id at the values of the axes in
// variations, as the layout of text of svg reads them for a <text> element
// and for ShapeText. It reads the tables hhea, OS/2, post and MVAR by the
// rules of the crate skrifa, which usvg uses. FaceMetrics returns false when
// the face does not exist, cannot be parsed, or has a number of units per
// em outside 16 to 16384.
func (db *FontDatabase) FaceMetrics(id FontID, variations []FontVariation) (FontMetrics, bool) {
	f, ok := resolveFont(id, variations, newFontCache(db, newOptions()))
	if !ok {
		return FontMetrics{}, false
	}
	return FontMetrics{
		UnitsPerEm:          f.unitsPerEm,
		Ascent:              f.ascent,
		Descent:             f.descent,
		XHeight:             f.xHeight,
		UnderlinePosition:   f.underlinePosition,
		UnderlineThickness:  f.underlineThickness,
		LineThroughPosition: f.lineThroughPosition,
		SubscriptOffset:     f.subscriptOffset,
		SuperscriptOffset:   f.superscriptOffset,
	}, true
}
