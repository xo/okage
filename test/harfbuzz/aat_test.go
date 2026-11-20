package harfbuzz_test

import "testing"

// The tests of the audit of D73 of the commits of HarfBuzz after 873dbc1e
// that change the AAT code. Each test builds a font from
// Roboto-Regular.ttf, as the other tests of D73 do.

const robotoD = 72

// morxAuditSubtable is a subtable of a chain of morx: its type, and its data
// after its header of 12 bytes.
type morxAuditSubtable struct {
	kind int
	data []byte
}

// morxAuditTable builds a table morx of version 2 with one chain. The chain has
// the default flags 1 and no feature. Each subtable applies in each
// direction, with the flags 1.
func morxAuditTable(subtables ...morxAuditSubtable) []byte {
	length := 16
	for _, st := range subtables {
		length += 12 + len(st.data)
	}
	out := u16s(2, 0)
	out = append(out, u32s(1, 1, uint32(length), 0, uint32(len(subtables)))...)
	for _, st := range subtables {
		out = append(out, u32s(uint32(12+len(st.data)), 0x20000000|uint32(st.kind), 1)...)
		out = append(out, st.data...)
	}
	return out
}

// ligatureOffset gives the offset of a ligature action that reads the
// component 0 at the glyph.
func ligatureOffset(glyph int) uint32 {
	return uint32(0x40000000 - glyph)
}

// morxLigature builds a ligature subtable that replaces a, b with x. It
// deletes b.
func morxLigature() morxAuditSubtable {
	classTable := u16s(8, robotoA, 2, 4, 5)
	states := u16s(0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 2)
	entries := u16s(0, 0, 0, 2, 0x8000, 0, 0, 0xA000, 0)
	actions := u32s(ligatureOffset(robotoB), 0xC0000000|ligatureOffset(robotoA))
	components := u16s(0)
	ligatures := u16s(robotoX)
	classOffset := 28
	statesOffset := classOffset + len(classTable)
	entriesOffset := statesOffset + len(states)
	actionsOffset := entriesOffset + len(entries)
	componentsOffset := actionsOffset + len(actions)
	data := u32s(6, uint32(classOffset), uint32(statesOffset), uint32(entriesOffset),
		uint32(actionsOffset), uint32(componentsOffset), uint32(componentsOffset+len(components)))
	for _, part := range [][]byte{classTable, states, entries, actions, components, ligatures} {
		data = append(data, part...)
	}
	return morxAuditSubtable{2, data}
}

// morxContextual builds a contextual subtable with the class table and the
// substitution lookup. In each state, the class 2 of the deleted glyph
// goes to the state 2, where the class 1, out of bounds, replaces the
// current glyph with the lookup.
func morxContextual(classTable, lookup []byte) morxAuditSubtable {
	states := u16s(0, 0, 1, 0, 0, 0, 1, 0, 0, 2, 0, 0)
	entries := u16s(0, 0, 0xFFFF, 0xFFFF, 2, 0, 0xFFFF, 0xFFFF, 0, 0, 0xFFFF, 0)
	substitutions := append(u32s(4), lookup...)
	classOffset := 20
	statesOffset := classOffset + len(classTable)
	entriesOffset := statesOffset + len(states)
	data := u32s(4, uint32(classOffset), uint32(statesOffset), uint32(entriesOffset),
		uint32(entriesOffset+len(entries)))
	for _, part := range [][]byte{classTable, states, entries, substitutions} {
		data = append(data, part...)
	}
	return morxAuditSubtable{1, data}
}

// TestMorxDeletedGlyphSet: a ligature subtable replaces a, b with x and
// deletes b. A contextual subtable then replaces c after the deleted glyph
// with d. Its class table covers only the glyphs 0xFFFE and 0xFFFF, so its
// set of glyphs holds the deleted glyph. HarfBuzz adds the deleted glyph to
// the set of glyphs of a buffer of four glyphs or more, from its commit
// 5a1c0c4660.
func TestMorxDeletedGlyphSet(t *testing.T) {
	morx := morxAuditTable(morxLigature(), morxContextual(u16s(8, 0xFFFE, 2, 1, 1), u16s(8, robotoC, 1, robotoD)))
	testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "abc", "abcc", "abccc", "cabc")
}

// kerxFormat2 builds a subtable of format 2 of kerx for a, b, c and d. The
// rows of the left classes are 0, 6, 12 and 18 bytes from the kerning
// array, and the right classes are 0, 3, 4 and 2 bytes. The array has three
// rows: 0, 0, 0, then 0, -80, 30, then 0, -50, -20.
func kerxFormat2() kerxSubtable {
	left := u16s(8, robotoA, 4, 0, 6, 12, 18)
	right := u16s(8, robotoA, 4, 0, 3, 4, 2)
	array := u16s(0, 0, 0, 0, -80, 30, 0, -50, -20)
	data := u32s(6, 28, uint32(28+len(left)), uint32(28+len(left)+len(right)))
	data = append(data, left...)
	data = append(data, right...)
	data = append(data, array...)
	return kerxSubtable{coverage: 2, data: data}
}

// TestKerxFormat2: a subtable of format 2 of kerx. HarfBuzz reads the sum
// of the classes as a byte offset from the kerning array, rounded down to
// a value of two bytes, from its commit 88e5077a80. It reads a value up to
// the end of the subtable, and up to the end of the table for the last
// subtable, whose length it ignores. In the first table, d, d reads after
// the end of the first subtable, in the header of the second. In the
// second table, the length of the subtable leaves out the last row.
func TestKerxFormat2(t *testing.T) {
	texts := []string{"ab", "ac", "bb", "bc", "cb", "cc", "bd", "dd", "xy", "abcd"}
	kerx := kerxTable(kerxFormat2(), kerxPairs(robotoX, robotoY, -300))
	testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, texts...)
	last := kerxFormat2()
	last.length = 12 + len(last.data) - 6
	testCase(t, robotoWith(t, map[string][]byte{"kerx": kerxTable(last)}), false, texts...)
}
