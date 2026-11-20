// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package unicodedata

import "testing"

// okage: the tests of D72.

// TestLatin1Lookups makes sure that the fast paths of the lookups give the
// result of the general lookups for each rune below U+0100, and around
// the limit of the fast paths.
func TestLatin1Lookups(t *testing.T) {
	for r := rune(-2); r < 0x180; r++ {
		if got, want := LookupGeneralCategory(r), GeneralCategory(gcLookup(r)); got != want {
			t.Errorf("LookupGeneralCategory(%U) = %d, want %d", r, got, want)
		}
		if got, want := LookupGraphemeBreak(r), lookupGraphemeBreak(r); got != want {
			t.Errorf("LookupGraphemeBreak(%U) = %d, want %d", r, got, want)
		}
		gotClass, gotBracket := LookupBidiClass(r)
		wantClass, wantBracket := lookupBidiClass(r)
		if gotClass != wantClass || gotBracket != wantBracket {
			t.Errorf("LookupBidiClass(%U) = %d, %d, want %d, %d", r, gotClass, gotBracket, wantClass, wantBracket)
		}
	}
}
