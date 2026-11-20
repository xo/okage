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
