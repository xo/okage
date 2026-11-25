package harfbuzz_test

import "testing"

// The tests of D101 compare harfbuzz with HarfBuzz 15.2.0 on a table trak
// whose offsets are null, and on the space separators that the generated
// USE table gives the category SPACE.

// trakTrack is a track of trakWith: its value and its tracking values for
// each size. nil values give the null offset.
type trakTrack struct {
	track  int32
	values []int
}

// trakWith builds a table trak with horizontal track data only. nil sizes
// give the null offset of the size table, with nSizes sizes.
func trakWith(nSizes int, sizes []int32, tracks ...trakTrack) []byte {
	const data = 12
	out := u32s(0x00010000)
	out = append(out, u16s(0, data, 0, 0)...)
	next := data + 8 + 8*len(tracks)
	sizeOffset := 0
	if sizes != nil {
		sizeOffset = next
		next += 4 * len(sizes)
	}
	out = append(out, u16s(len(tracks), nSizes)...)
	out = append(out, u32s(uint32(sizeOffset))...)
	var tail []byte
	for _, s := range sizes {
		tail = append(tail, u32s(uint32(s)<<16)...)
	}
	for _, tr := range tracks {
		out = append(out, u32s(uint32(tr.track)<<16)...)
		valueOffset := 0
		if tr.values != nil {
			valueOffset = next
			next += 2 * len(tr.values)
			tail = append(tail, u16s(tr.values...)...)
		}
		out = append(out, u16s(0, valueOffset)...)
	}
	return append(out, tail...)
}

// TestTrakNullOffsets: tables trak with a null offset to the values of a
// track or to the sizes, with a table STAT. HarfBuzz 15.2.0 reads the
// offset 0 as the start of the table trak, so the values are the first
// words of the table: 1, 0, 0, 12 and so on. go-text panicked on a null
// offset to the values. The last table has two tracks with the same
// value, which HarfBuzz averages.
func TestTrakNullOffsets(t *testing.T) {
	stat := u16s(1, 1, 8, 0)
	stat = append(stat, u32s(0)...)
	stat = append(stat, u16s(0)...)
	stat = append(stat, u32s(0)...)
	stat = append(stat, u16s(0)...)
	for _, c := range []struct {
		name string
		trak []byte
	}{
		{"one size, null values", trakWith(1, []int32{12}, trakTrack{0, nil})},
		{"four sizes, null values", trakWith(4, []int32{2, 4, 8, 16}, trakTrack{0, nil})},
		{"null sizes", trakWith(2, nil, trakTrack{0, []int{100, 300}})},
		{"null sizes and values", trakWith(3, nil, trakTrack{0, nil})},
		{"two tracks at 0", trakWith(1, []int32{12}, trakTrack{0, []int{100}}, trakTrack{0, []int{300}})},
	} {
		t.Run(c.name, func(t *testing.T) {
			data := robotoWith(t, map[string][]byte{"trak": c.trak, "STAT": stat})
			compareOptions(t, data, shapeOptions{}, "ab", "abc")
		})
	}
}

// TestUSESpaceSeparators: Roboto with the table GSUB of TestUSEWordEnds,
// whose features isol, init, medi and fina for Balinese each replace the
// glyph 0. The USE table that usegen writes gives the category SPACE to
// each space separator, as the table of HarfBuzz 15.2.0 does, where D97
// mapped them by hand. A space separator ends a word and is not the base
// of a vowel or a mark. The no-break space is a consonant placeholder, the
// base of the vowel after it.
func TestUSESpaceSeparators(t *testing.T) {
	gsub := layoutTable(t, []string{"DFLT", "bali"},
		[]layoutFeature{{"fina", []int{3}}, {"init", []int{1}}, {"isol", []int{0}}, {"medi", []int{2}}},
		[][]byte{
			lookupTable(t, 1, 0, singleSubst(robotoA, 0)),
			lookupTable(t, 1, 0, singleSubst(robotoB, 0)),
			lookupTable(t, 1, 0, singleSubst(robotoC, 0)),
			lookupTable(t, 1, 0, singleSubst(robotoX, 0)),
		})
	data := robotoWith(t, map[string][]byte{"GSUB": gsub})
	compareOptions(t, data, shapeOptions{flags: true},
		"ᬓᬓ\u3000ᬓᬓ", "ᬓ\u2002ᬓᬓᬓ", "ᬓ\u1680ᬓ", "ᬓ\u202fᬓ\u205fᬓ",
		"ᬓ\u3000\u1b36", "\u2009\u1b36", "\u200a\u1b34\u1b36", "ᬓ\u00a0\u1b36", "\u00a0\u1b36ᬓ")
}
