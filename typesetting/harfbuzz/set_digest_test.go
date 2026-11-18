package harfbuzz

import (
	"bytes"
	"os"
	"testing"

	"github.com/xo/okage/typesetting/font"
)

func TestDigest(t *testing.T) {
	const (
		setTypeSize = 2
		numBits     = 3 + 1 + 1
	)
	if shift0 >= setTypeSize*8 {
		t.Error()
	}
	if shift0+numBits > setTypeSize*8 {
		t.Error()
	}
	if shift1 >= setTypeSize*8 {
		t.Error()
	}
	if shift1+numBits > setTypeSize*8 {
		t.Error()
	}
	if shift2 >= setTypeSize*8 {
		t.Error()
	}
	if shift2+numBits > setTypeSize*8 {
		t.Error()
	}
}

func TestDigestHas(t *testing.T) {
	var d setDigest
	for i := setType(10); i < 65_000; i += 7 {
		d.add(i)
	}
	for i := setType(10); i < 65_000; i += 7 {
		if !d.mayHave(i) {
			t.Errorf("expected <may have> for %d", i)
		}
	}
	for i := setType(0); i < 0xFFFF; i++ { // care with overflow
		// if the filter is negative, then the glyph must not be in the set
		if !d.mayHave(i) {
			if (i-10)%7 == 0 {
				t.Errorf("<not have> for glyph %d present in set", i)
			}
		}
	}
}

func TestDigestRangeAfterFlood(t *testing.T) {
	// A range spanning 63 glyphs or more fills the shift-0 sub-digest; the ranges
	// added after it must still reach the other two (Noto Sans Arabic's ccmp
	// coverage: [577-718] first, the noon at 759 in a later range).
	var d setDigest
	d.addRange(577, 718)
	d.addRange(737, 762)
	for _, g := range []setType{577, 700, 718, 737, 759, 762} {
		if !d.mayHave(g) {
			t.Errorf("expected <may have> for %d, which is in an added range", g)
		}
	}
	if d.mayHave(730) {
		t.Errorf("<may have> for 730, which no range covers, means the filter flooded")
	}
}

// okage: D59 leaves out TestDigestKerningAfterFlood and
// TestDigestArabicAfterFlood of go-text, because they read the fonts of
// typesetting-utils (D52). TestDigestKerningRoboto tests the same fault with
// a font of font/testdata.

// TestDigestKerningRoboto shapes pairs whose first glyph the kern lookup of
// Roboto lists after a range of 63 glyphs or more. Before the fix, these pairs
// had no kerning. The advances are the advance of hmtx plus the value of the
// pair in GPOS, in font units.
func TestDigestKerningRoboto(t *testing.T) {
	data, err := os.ReadFile("../font/testdata/Roboto-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	fnt := NewFont(face)
	for _, test := range []struct {
		text    string
		advance Position // of the first glyph
	}{
		{"AV", 1249}, // 1336 - 87, in the first range
		{"ýf", 982},  // 969 + 13
		{"Āw", 1303}, // 1336 - 33
		{"Ěw", 1142}, // 1164 - 22
		{"Ĺw", 1010}, // 1102 - 92
		{"ŕt", 743},  // 693 + 50
	} {
		buffer := NewBuffer()
		buffer.AddRunes([]rune(test.text), 0, -1)
		buffer.GuessSegmentProperties()
		buffer.Shape(fnt, nil)
		if got := buffer.Pos[0].XAdvance; got != test.advance {
			t.Errorf("%q: first advance %d, expected %d", test.text, got, test.advance)
		}
	}
}
