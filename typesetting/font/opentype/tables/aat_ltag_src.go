// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

import "github.com/xo/okage/typesetting/language"

// Ltag is the language tags table
// See https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6ltag.html
type Ltag struct {
	version    uint32        //	Table version; currently 1
	flags      uint32        //	Table flags; currently none defined
	numTags    uint32        //	Number of language tags which follow
	tagRange   []stringRange `arrayCount:"ComputedField-numTags"` //	Range for each tag's string
	stringData []byte        `subsliceStart:"AtStart" arrayCount:"ToEnd"`
}

type stringRange struct {
	offset uint16 // Offset from the start of the table to the beginning of the string
	length uint16 // String length (in bytes)
}

func (lt Ltag) Language(i uint16) language.Language {
	r := lt.tagRange[i]
	return language.NewLanguage(string(lt.stringData[r.offset : r.offset+r.length]))
}

// MatchesLanguage returns true when the tag i matches the language
// specific. The tag matches when its canonical form is specific, or the
// start of specific up to a '-'. The canonical form has at most 63 bytes,
// in lowercase, with '-' for '_', and it stops before the first byte that
// is not a letter, a digit or '-'.
//
// okage: MatchesLanguage follows ltag::matches_language of HarfBuzz
// 15.1.0, from its commit cd570b6f54. go-text matched a tag when specific
// was the start of the tag, read the whole tag with language.NewLanguage,
// which does not stop before such a byte, and panicked for a tag outside
// the table. Such a tag now matches no language. ParseLtag refuses a table
// with a string outside the table, as HarfBuzz does (D73).
func (lt Ltag) MatchesLanguage(i int, specific language.Language) bool {
	if i < 0 || i >= len(lt.tagRange) {
		return false
	}
	r := lt.tagRange[i]
	end := int(r.offset) + int(r.length)
	if end > len(lt.stringData) {
		return false
	}
	tag, s := lt.stringData[r.offset:end], string(specific)
	l := 0
	for ; l < min(len(tag), 63); l++ {
		c := tag[l]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c == '_' {
			c = '-'
		}
		if !('a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '-') {
			break
		}
		if l >= len(s) || c != s[l] {
			return false
		}
	}
	return l > 0 && (l == len(s) || s[l] == '-')
}
