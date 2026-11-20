// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package language

import "testing"

// okage: the tests of D72.

// TestLatin1Scripts makes sure that the fast path of LookupScript gives
// the result of the general lookup for each rune below U+0100, and around
// the limit of the fast path.
func TestLatin1Scripts(t *testing.T) {
	for r := rune(-2); r < 0x180; r++ {
		if got, want := LookupScript(r), lookupScript(r); got != want {
			t.Errorf("LookupScript(%U) = %s, want %s", r, got, want)
		}
	}
}

// TestScriptBlocks makes sure that LookupScript gives the result of the
// binary search over all of ScriptRanges for each rune, and for runes
// that are not valid.
func TestScriptBlocks(t *testing.T) {
	check := func(r rune) {
		if got, want := LookupScript(r), lookupScript(r); got != want {
			t.Fatalf("LookupScript(%U) = %s, want %s", r, got, want)
		}
	}
	for r := rune(0); r <= 0x10FFFF+0x1000; r++ {
		check(r)
	}
	for _, r := range []rune{-1 << 31, -0x10000, -1, 0x7FFFFFFF} {
		check(r)
	}
}
