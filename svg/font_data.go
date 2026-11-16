package svg

import (
	"github.com/xo/okage/webfont"
)

// This file ports no upstream file. It holds the APIs of D45 for fonts in a
// web format and for the strings of the name table of a face.

// ConvertFontData returns the TrueType or OpenType data of a font in WOFF,
// WOFF2 or EOT, or of a collection in WOFF2. It returns data as it is when
// data is already TrueType or OpenType data, or a collection of them.
// FontDatabase.LoadFontData does the same conversion, so a program does not
// need this function to load such a font.
//
// ConvertFontData returns an error that wraps okage.ErrFontData for
// malformed data. It returns okage.ErrFontUnsupported for data in another
// format, and for an EOT file that uses MicroType Express compression.
func ConvertFontData(data []byte) ([]byte, error) {
	return webfont.Convert(data)
}

// FaceName returns the string of the name ID nameID of the name table of the
// face id, such as 4 for the full name, 5 for the version and 19 for the
// sample text. It takes the record in English (United States) first, then
// another Unicode record, then a Macintosh record in Roman. It returns false
// when the face does not exist or has no such record.
func (db *FontDatabase) FaceName(id FontID, nameID uint16) (string, bool) {
	face, ok := db.withFaceData(id)
	if !ok {
		return "", false
	}
	names, ok := parseNameTable(face.rawTable("name"))
	if !ok {
		return "", false
	}
	// rank orders the records: 0 is the best, and -1 is a record that is
	// not used.
	rank := func(n nameRecord) int {
		switch {
		case n.nameID != nameID:
			return -1
		case n.isUnicode() && n.language() == FontLanguageEnglishUnitedStates:
			return 0
		case n.isUnicode():
			return 1
		case n.isMacRoman() && n.language() == FontLanguageEnglishUnitedStates:
			return 2
		case n.isMacRoman():
			return 3
		}
		return -1
	}
	best, bestRank := "", -1
	for _, n := range names {
		r := rank(n)
		if r < 0 || (bestRank >= 0 && r >= bestRank) {
			continue
		}
		if s, ok := nameToUnicode(n); ok {
			best, bestRank = s, r
		}
	}
	return best, bestRank >= 0
}
