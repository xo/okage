// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"sort"
	"testing"
)

// An entry of the symbol table names a character. A character without a
// glyph in the face is an empty box, with a warning of WithLogger, and the
// formula then misses a symbol without an error. So each character of the
// table has a glyph in STIX Two Math. The source tests the same with the
// name TestEverySymbolHasAGlyphInTheEmbeddedFont, where a missing glyph is
// the box .notdef.
func TestEverySymbolOfTheTableHasAGlyph(t *testing.T) {
	face := testFace(t)
	names := make([]string, 0, len(symbols))
	for name := range symbols {
		names = append(names, name)
	}
	sort.Strings(names)
	missing := 0
	for _, name := range names {
		r := symbols[name].r
		if _, ok := face.NominalGlyph(r); !ok {
			t.Errorf("\\%s: U+%04X has no glyph in STIX Two Math", name, r)
			missing++
		}
	}
	t.Logf("%d symbols checked, %d without a glyph", len(names), missing)
}
