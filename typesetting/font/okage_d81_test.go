// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package font

import (
	"os"
	"testing"
)

// okage: the tests of D81.

// TestCmapCache compares NominalGlyph, which reads the cache of the cmap
// of the face, with a lookup in the cmap, for each rune. The second pass
// asks for the runes in another order, so that the cache holds other runes
// when it is read.
func TestCmapCache(t *testing.T) {
	for _, name := range []string{"testdata/Roboto-Regular.ttf", "testdata/Amiri-Regular.ttf"} {
		file, err := os.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		face, err := ParseTTF(file)
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		const last = 0x10ffff
		check := func(r rune) {
			t.Helper()
			gotG, gotOK := face.NominalGlyph(r)
			wantG, wantOK := face.Cmap.Lookup(r)
			if gotG != wantG || gotOK != wantOK {
				t.Fatalf("%s, rune %#x: got %d %v, want %d %v", name, r, gotG, gotOK, wantG, wantOK)
			}
		}
		for r := rune(0); r <= last; r++ {
			check(r)
		}
		for i := rune(0); i <= last; i++ {
			check(i * 4099 % (last + 1))
		}
	}
}
