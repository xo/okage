// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"encoding/binary"
	"testing"
)

// subsetter tests/fuzz subsets the fonts of a folder with random sets of
// glyphs. FuzzSubset gives malformed fonts and random sets of glyphs to
// Subset and SubsetWithVariations instead (item 8 of "Testing" under "The
// PDF writer" in docs/PLAN.md). Neither one must panic or hang.

// maxFuzzFont is the largest font that FuzzSubset subsets. The memory of a
// subset has a bound that grows with the size of the font: a composite
// glyph can refer to each glyph ID, and the subset then holds a copy of the
// data of each one.
const maxFuzzFont = 1 << 14

// FuzzSubset subsets malformed fonts. The first two bytes of glyphs tell
// whether to call SubsetWithVariations and with which value of the axis
// wght, and the other bytes give the glyphs, two bytes each.
func FuzzSubset(f *testing.F) {
	for _, name := range []string{"TestTTC.ttc"} {
		data := readTestFont(f, name)
		f.Add(data, []byte{0, 0, 0, 1, 0, 2})
		f.Add(data, []byte{1, 0, 0, 3})
	}
	small, err := Subset(readTestFont(f, "NotoSans-Regular.ttf"), 0, NewGlyphRemapperFromGlyphs([]uint16{36, 120, 121, 122}))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(small, []byte{0, 0, 0, 1, 0, 2, 0, 3})
	f.Add(small, []byte{1, 200, 0, 4})
	f.Add(buildTrueType([][]byte{nil, compositeGlyph(2, 3, 1), simpleGlyph(), compositeGlyph(1, 2)}), []byte{0, 0, 0, 1})

	f.Fuzz(func(t *testing.T, data, glyphs []byte) {
		if len(data) > maxFuzzFont || len(glyphs) < 2 || len(glyphs) > 512 {
			return
		}
		m := NewGlyphRemapper()
		for i := 2; i+1 < len(glyphs); i += 2 {
			m.Remap(binary.BigEndian.Uint16(glyphs[i:]))
		}
		before := m.Clone()
		var out []byte
		var err error
		if glyphs[0]&1 == 0 {
			out, err = Subset(data, 0, m)
		} else {
			vars := []Variation{{Tag{'w', 'g', 'h', 't'}, float32(glyphs[1]) * 4}}
			out, err = SubsetWithVariations(data, uint32(glyphs[0]>>1), vars, m)
		}
		if !m.Equal(before) {
			t.Error("the subset changes the remapper")
		}
		if err != nil {
			return
		}
		if len(out) < 12 {
			t.Errorf("the subset has %d bytes", len(out))
		}
		// The result is a font that the subsetter reads again.
		if _, err := parse(out, 0); err != nil {
			t.Errorf("the subset does not parse: %v", err)
		}
	})
}
