// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package unicodedata

import (
	"testing"
	"unicode"
)

// okage: the tests of D92.

// TestGeneralCategoryTable makes sure that LookupGeneralCategory, which
// reads the table of LookupSegmentProperties, gives the result of gcLookup
// for each rune, and for the runes just outside the range of Unicode.
func TestGeneralCategoryTable(t *testing.T) {
	for r := rune(-2); r <= unicode.MaxRune+2; r++ {
		if got, want := LookupGeneralCategory(r), GeneralCategory(gcLookup(r)); got != want {
			t.Fatalf("LookupGeneralCategory(%U) = %d, want %d", r, got, want)
		}
	}
}
