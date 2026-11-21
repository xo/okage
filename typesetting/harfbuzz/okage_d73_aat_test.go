// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package harfbuzz

import (
	"encoding/binary"
	"slices"
	"strings"
	"testing"

	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
	"github.com/xo/okage/typesetting/language"
)

// okage: the tests of the audit of D73 of the commits of HarfBuzz after
// 873dbc1e that change the AAT code. test/harfbuzz compares the same fonts
// with HarfBuzz 15.1.0.

const robotoD GID = 72

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
func ligatureOffset(glyph GID) uint32 {
	return uint32(0x40000000 - int(glyph))
}

// morxLigature builds a ligature subtable that replaces a, b with x. It
// deletes b.
func morxLigature() morxAuditSubtable {
	return morxLigatureAt(robotoB)
}

// morxLigatureAt builds the subtable of morxLigature, whose first action
// reads the component at glyph minus the glyph on the stack.
func morxLigatureAt(glyph GID) morxAuditSubtable {
	return morxLigatureFlags(glyph, 0xA000)
}

// morxLigatureFlags builds the subtable of morxLigatureAt, where the entry
// of b has the flags.
func morxLigatureFlags(glyph GID, flags int) morxAuditSubtable {
	classTable := u16s(8, int(robotoA), 2, 4, 5)
	states := u16s(0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 2)
	entries := u16s(0, 0, 0, 2, 0x8000, 0, 0, flags, 0)
	actions := u32s(ligatureOffset(glyph), 0xC0000000|ligatureOffset(robotoA))
	components := u16s(0)
	ligatures := u16s(int(robotoX))
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
	return morxContextualStates(classTable, u16s(0, 0, 1, 0, 0, 0, 1, 0, 0, 2, 0, 0), lookup)
}

// morxContextualStates builds a contextual subtable with the class table,
// the rows of the states for the four classes and the substitution lookup.
// The entry 0 does nothing, the entry 1 goes to the state 2, and the entry
// 2 replaces the current glyph with the lookup.
func morxContextualStates(classTable, states, lookup []byte) morxAuditSubtable {
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

// TestMorxDeletedGlyphSet shapes abc and abcc. A ligature subtable
// replaces a, b with x and deletes b. A contextual subtable then replaces
// c after the deleted glyph with d. Its class table covers only the glyphs
// 0xFFFE and 0xFFFF, so its set of glyphs holds the deleted glyph. As the
// commit 5a1c0c4660 of HarfBuzz does, the deleted glyph goes into the set
// of glyphs of a buffer of four glyphs or more. go-text did not add it,
// and did not apply the contextual subtable to abcc (D73).
func TestMorxDeletedGlyphSet(t *testing.T) {
	morx := morxAuditTable(morxLigature(), morxContextual(u16s(8, 0xFFFE, 2, 1, 1), u16s(8, int(robotoC), 1, int(robotoD))))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"morx": morx}), []shapedText{
		{"abc", []shapedGlyph{{robotoX, 0, 1015, 0, 0}, {robotoD, 2, 1155, 0, 0}}},
		{"abcc", []shapedGlyph{{robotoX, 0, 1015, 0, 0}, {robotoD, 2, 1155, 0, 0}, {robotoC, 3, 1072, 0, 0}}},
	})
}

// kerxFormat2 builds a subtable of format 2 of kerx for a, b, c and d. The
// rows of the left classes are 0, 6, 12 and 18 bytes from the kerning
// array, and the right classes are 0, 3, 4 and 2 bytes. The array has three
// rows: 0, 0, 0, then 0, -80, 30, then 0, -50, -20.
func kerxFormat2() kerxSubtable {
	return kerxFormat2Lookups(u16s(8, int(robotoA), 4, 0, 6, 12, 18), u16s(8, int(robotoA), 4, 0, 3, 4, 2))
}

// kerxFormat2Lookups builds the subtable of kerxFormat2 with the lookups
// of the left and the right classes.
func kerxFormat2Lookups(left, right []byte) kerxSubtable {
	array := u16s(0, 0, 0, 0, -80, 30, 0, -50, -20)
	data := u32s(6, 28, uint32(28+len(left)), uint32(28+len(left)+len(right)))
	data = append(data, left...)
	data = append(data, right...)
	data = append(data, array...)
	return kerxSubtable{coverage: 2, data: data}
}

// TestKerxFormat2 shapes texts with kerxFormat2. As the commit 88e5077a80
// of HarfBuzz, in 15.1.0, reads it, the sum of the classes is a byte
// offset from the kerning array, rounded down to a value of two bytes. A
// value is read up to the end of the subtable, and up to the end of the
// table for the last subtable. In the first table, d, d reads after the
// end of the first subtable, in the header of the second. In the second
// table, the length of the subtable leaves out the last row. go-text read
// the sum as an offset from the end of the header of the subtable, and
// required it to be after the start of the kerning array. So it did not
// kern these texts (D73).
func TestKerxFormat2(t *testing.T) {
	bb := shapedText{"bb", []shapedGlyph{{robotoB, 0, 1109, 0, 0}, {robotoB, 1, 1109, -40, 0}}}
	bc := shapedText{"bc", []shapedGlyph{{robotoB, 0, 1164, 0, 0}, {robotoC, 1, 1087, 15, 0}}}
	cb := shapedText{"cb", []shapedGlyph{{robotoC, 0, 1047, 0, 0}, {robotoB, 1, 1124, -25, 0}}}
	cc := shapedText{"cc", []shapedGlyph{{robotoC, 0, 1062, 0, 0}, {robotoC, 1, 1062, -10, 0}}}
	bd := shapedText{"bd", []shapedGlyph{{robotoB, 0, 1109, 0, 0}, {robotoD, 1, 1115, -40, 0}}}
	dd := shapedText{"dd", []shapedGlyph{{robotoD, 0, 1155, 0, 0}, {robotoD, 1, 1155, 0, 0}}}
	kerx := kerxTable(kerxFormat2(), kerxPairs(int(robotoX), int(robotoY), -300))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), []shapedText{bb, bc, cb, cc, bd, dd})
	last := kerxFormat2()
	last.length = 12 + len(last.data) - 6
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerxTable(last)}), []shapedText{cb, cc, dd})
}

// TestLookupFormat10ValueSize shapes ab with a table kerx whose first
// subtable has a lookup of format 10 for its left classes, then a subtable
// of format 0 that kerns a, b. The values of the lookup have 0 to 5 bytes.
// As in HarfBuzz 15.1.0, a size of 0 bytes or of more than 4 bytes is an
// error, and the table has no subtable. The commit a763a612d1 of HarfBuzz
// refuses the size 0. go-text read the values as two bytes for each size,
// and kerned a, b (D73).
func TestLookupFormat10ValueSize(t *testing.T) {
	kerned := []shapedGlyph{{robotoA, 0, 964, 0, 0}, {robotoB, 1, 999, -150, 0}}
	plain := []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoB, 1, 1149, 0, 0}}
	for _, test := range []struct {
		size int
		want []shapedGlyph
	}{{0, plain}, {1, kerned}, {2, kerned}, {4, kerned}, {5, plain}} {
		left := u16s(10, test.size, int(robotoA), 3, 0, 6, 12)
		kerx := kerxTable(kerxFormat2Lookups(left, u16s(8, int(robotoA), 3, 0, 2, 4)), kerxPairs(int(robotoA), int(robotoB), -300))
		face := fontWithLayout(t, map[string][]byte{"kerx": kerx})
		testShapedFace(t, face, []shapedText{{"ab", test.want}})
	}
}

// kernNegativeState builds a table kern of Apple with a subtable of format
// 1, whose state array starts after the row of the state -1. The class of
// a and b is 4. In the state 0, a or b pushes the glyph and goes to the
// state -1. In the state -1, a or b pushes the glyph, applies the list -40,
// -29 and goes to the state 0.
func kernNegativeState() []byte {
	data := u16s(5, 10, 21, 26, 38)
	data = append(data, u16s(int(robotoA), 2)...)
	data = append(data, 4, 4)
	data = append(data, 0, 0, 0, 0, 2) // the state -1
	data = append(data, 0, 0, 0, 0, 1) // the state 0
	data = append(data, u16s(21, 0, 16, kerxPush, 21, kerxPush|38)...)
	data = append(data, u16s(-40, -29)...)
	kern := u32s(0x00010000, 1, uint32(8+len(data)), 0x00010000)
	return append(kern, data...)
}

// TestKernNegativeState shapes ab and aab with kernNegativeState. As the
// commit ec4a38c4ab of HarfBuzz, in 15.1.0, reads it, a state before the
// state array is a negative state. go-text gave the state -1 an index out
// of range, so its entries did nothing and the text had no kerning (D73).
func TestKernNegativeState(t *testing.T) {
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kern": kernNegativeState()}), []shapedText{
		{"ab", []shapedGlyph{{robotoA, 0, 1084, -30, 0}, {robotoB, 1, 1109, -40, 0}}},
		{"aab", []shapedGlyph{{robotoA, 0, 1084, -30, 0}, {robotoA, 1, 1074, -40, 0}, {robotoB, 2, 1149, 0, 0}}},
	})
}

// TestLookupSegmentsOutOfOrder shapes c and cccc with a contextual
// subtable whose class table is a lookup of format 2 with the segments x,
// a and z, out of order. In each state, the class 1, out of bounds,
// replaces c with d. As the commit 77be62a733 of HarfBuzz, in 15.1.0,
// collects them, the glyphs from a to z go into the set of the subtable at
// the first segment out of order. So c is in the set, and the subtable
// applies. go-text added only x, a and z, and did not apply the subtable
// (D73).
func TestLookupSegmentsOutOfOrder(t *testing.T) {
	classTable := u16s(2, 6, 3, 12, 1, 6, int(robotoX), int(robotoX), 1, int(robotoA), int(robotoA), 1, int(robotoZ), int(robotoZ), 1)
	states := u16s(0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 0, 0)
	morx := morxAuditTable(morxContextualStates(classTable, states, u16s(8, int(robotoC), 1, int(robotoD))))
	d := shapedGlyph{robotoD, 0, 1155, 0, 0}
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"morx": morx}), []shapedText{
		{"c", []shapedGlyph{d}},
		{"cccc", []shapedGlyph{d, {robotoD, 1, 1155, 0, 0}, {robotoD, 2, 1155, 0, 0}, {robotoD, 3, 1155, 0, 0}}},
	})
}

// TestLookupSegmentsCollect collects the glyphs of lookups of format 2. At
// the first segment out of order, one range from the first to the last
// glyph of the rest of the segments takes their place. A segment whose
// first glyph is after its last glyph adds nothing.
func TestLookupSegmentsCollect(t *testing.T) {
	for _, test := range []struct {
		segments [][2]tables.GlyphID
		want     [][2]uint32
	}{
		{[][2]tables.GlyphID{{1, 3}, {5, 5}, {8, 9}}, [][2]uint32{{1, 3}, {5, 5}, {8, 9}}},
		{[][2]tables.GlyphID{{10, 12}, {5, 6}, {20, 21}}, [][2]uint32{{5, 21}}},
		{[][2]tables.GlyphID{{1, 3}, {9, 4}, {5, 6}, {2, 2}, {7, 7}}, [][2]uint32{{1, 7}}},
		{[][2]tables.GlyphID{{5, 6}, {0, 0}, {9, 4}, {3, 3}}, [][2]uint32{{0, 3}, {5, 6}}},
		{[][2]tables.GlyphID{{1, 3}, {3, 4}, {9, 8}}, [][2]uint32{{1, 4}}},
		{[][2]tables.GlyphID{{0, 0xFFFE}, {0, 0xFFFE}}, [][2]uint32{{0, 0xFFFE}}},
	} {
		var got, want intSet
		collectSegmentGlyphs(&got, test.segments)
		for _, r := range test.want {
			want.addRange(r[0], r[1])
		}
		for g := uint32(0); g < 0x10000; g++ {
			if got.has(g) != want.has(g) {
				t.Errorf("%v: glyph %d is in the set: %v, want %v", test.segments, g, got.has(g), want.has(g))
				break
			}
		}
	}
}

// ltagTable builds a table ltag with the tags.
func ltagTable(tags ...string) []byte {
	out := u32s(1, 0, uint32(len(tags)))
	offset := 12 + 4*len(tags)
	var data []byte
	for _, tag := range tags {
		out = append(out, u16s(offset+len(data), len(tag))...)
		data = append(data, tag...)
	}
	return append(out, data...)
}

// morxLanguages builds a table morx with one chain. Its features of the
// type 39 turn on the flags 2, 4, 8 and 16 for the tags 1, 2, 3 and 9 of
// the table ltag. The subtable of the flags 2 replaces a with b, the
// subtable of the flags 4 replaces a with c, the subtable of the flags 8
// replaces a with d, and the subtable of the flags 16 replaces a with x.
func morxLanguages() []byte {
	features := []struct{ setting, flags int }{{1, 2}, {2, 4}, {3, 8}, {9, 16}}
	replacements := []GID{robotoB, robotoC, robotoD, robotoX}
	length := 16 + 12*len(features) + len(replacements)*(12+8)
	out := u16s(2, 0)
	out = append(out, u32s(1, 1, uint32(length), uint32(len(features)), uint32(len(replacements)))...)
	for _, f := range features {
		out = append(out, u16s(39, f.setting)...)
		out = append(out, u32s(uint32(f.flags), 0xFFFFFFFF)...)
	}
	for i, glyph := range replacements {
		out = append(out, u32s(20, 0x20000004, uint32(features[i].flags))...)
		out = append(out, u16s(8, int(robotoA), 1, int(glyph))...)
	}
	return out
}

// TestLtagLanguage shapes a in languages with a table morx whose features
// of the type 39 turn on subtables for the tags sr, Zh_Hant and en@x of
// the table ltag, and for a tag 9 that the table does not have. As the
// commit cd570b6f54 of HarfBuzz, in 15.1.0, does, the canonical form of a
// tag stops before @, and a tag matches the language, or the start of the
// language up to a '-'. go-text matched the language as the start of the
// tag, read en@x as en-x, and panicked for the tag 9 (D73).
func TestLtagLanguage(t *testing.T) {
	face := fontWithLayout(t, map[string][]byte{"morx": morxLanguages(), "ltag": ltagTable("sr", "Zh_Hant", "en@x")})
	hf := NewFont(face)
	for _, test := range []struct {
		lang string
		want GID
	}{
		{"sr", robotoB}, {"SR", robotoB}, {"sr-Latn-RS", robotoB}, {"srb", robotoA}, {"s", robotoA},
		{"zh-hant", robotoC}, {"zh-hant-tw", robotoC}, {"zh", robotoA},
		{"en", robotoD}, {"en-us", robotoD}, {"enx", robotoA}, {"fr", robotoA},
	} {
		buf := NewBuffer()
		buf.AddRunes([]rune("a"), 0, -1)
		buf.Props.Language = language.NewLanguage(test.lang)
		buf.GuessSegmentProperties()
		buf.Shape(hf, nil)
		if got := buf.Info[0].Glyph; got != test.want {
			t.Errorf("a in %s: got the glyph %d, want %d", test.lang, got, test.want)
		}
	}
}

// featLigatures builds a table feat with the feature type 1, the
// ligatures, and its settings 2 and 3, the common ligatures on and off.
func featLigatures() []byte {
	out := u32s(0x00010000)
	out = append(out, u16s(1, 0, 0, 0)...)
	out = append(out, u16s(1, 2, 0, 24, 0, 256)...)
	return append(out, u16s(2, 256, 3, 256)...)
}

// morxBudget builds a table morx with a chain whose subtable replaces a
// with b when the common ligatures are on, then empty chains. The first
// chain has extra feature records that no text asks for.
func morxBudget(extra, empty int) []byte {
	features := u16s(1, 2)
	features = append(features, u32s(1, 0xFFFFFFFF)...)
	features = append(features, u16s(1, 3)...)
	features = append(features, u32s(0, 0xFFFFFFFE)...)
	for range extra {
		features = append(features, u16s(100, 0)...)
		features = append(features, u32s(0, 0xFFFFFFFF)...)
	}
	out := u16s(2, 0)
	out = append(out, u32s(uint32(1+empty), 0, uint32(16+len(features)+20), uint32(2+extra), 1)...)
	out = append(out, features...)
	out = append(out, u32s(20, 0x20000004, 1)...)
	out = append(out, u16s(8, int(robotoA), 1, int(robotoB))...)
	for range empty {
		out = append(out, u32s(0, 16, 0, 0)...)
	}
	return out
}

// TestMorxMapBudget shapes 2,000 a, whose characters each have the feature
// liga in a range of their own, on at an odd index and off at an even
// index. The table morx is morxBudget with 10,000 extra feature records
// and 1,000 empty chains. As the commit 370a0da5a1 of HarfBuzz, in 15.1.0,
// does, the map extends the last range of a chain when the next range has
// the same flags. Each range costs 11,020 from the budget of 16,777,216,
// and the first range costs 16,000 more for the empty chains. So the
// compilation stops after the range of the index 1,520, which then goes
// to the end of the text. go-text had no budget, and kept a range for each
// chain and each range of the text. Without the merge of the ranges, a
// range costs 27,020 (D73).
func TestMorxMapBudget(t *testing.T) {
	face := fontWithLayout(t, map[string][]byte{"morx": morxBudget(10000, 1000), "feat": featLigatures()})
	const n = 2000
	features := make([]Feature, n)
	for i := range features {
		features[i] = Feature{Tag: ot.NewTag('l', 'i', 'g', 'a'), Value: uint32(i % 2), Start: i, End: i + 1}
	}
	buf := NewBuffer()
	buf.AddRunes([]rune(strings.Repeat("a", n)), 0, -1)
	buf.GuessSegmentProperties()
	buf.Shape(NewFont(face), features)
	for i, info := range buf.Info {
		want := robotoA
		if i%2 == 1 && i <= 1520 {
			want = robotoB
		}
		if info.Glyph != want {
			t.Fatalf("glyph %d: got %d, want %d", i, info.Glyph, want)
		}
	}
}

// TestMorxLigatureNegativeComponent shapes texts with a ligature subtable
// whose first action reads the component at the index -1. HarfBuzz 15.1.0
// stops the action and keeps the glyphs. go-text panicked with an index
// out of range (D73).
func TestMorxLigatureNegativeComponent(t *testing.T) {
	face := fontWithLayout(t, map[string][]byte{"morx": morxAuditTable(morxLigatureAt(robotoB + 1))})
	for _, text := range []string{"ab", "abc", "cab"} {
		buf := NewBuffer()
		buf.AddRunes([]rune(text), 0, -1)
		buf.GuessSegmentProperties()
		buf.Shape(NewFont(face), nil)
		var got []GID
		for _, info := range buf.Info {
			got = append(got, info.Glyph)
		}
		want := map[string][]GID{"ab": {robotoA, robotoB}, "abc": {robotoA, robotoB, robotoC}, "cab": {robotoC, robotoA, robotoB}}[text]
		if len(got) != len(want) {
			t.Fatalf("%s: got the glyphs %v, want %v", text, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: got the glyphs %v, want %v", text, got, want)
			}
		}
	}
}

// TestMorxLigaturePerformAction shapes ab with a ligature subtable that
// replaces a, b with x, where the entry of b has the flag SetComponent and
// other flags. As in HarfBuzz 15.1.0, the entry has an action only when it
// has the flag PerformAction, 0x2000. go-text took any bit of the mask
// 0x3FFF, and replaced a, b with x for the reserved bits 0x1FFF (D73).
func TestMorxLigaturePerformAction(t *testing.T) {
	ligature := []shapedGlyph{{robotoX, 0, 1015, 0, 0}}
	plain := []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoB, 1, 1149, 0, 0}}
	for _, test := range []struct {
		flags int
		want  []shapedGlyph
	}{{0xA000, ligature}, {0x8001, plain}, {0x9000, plain}, {0x8FFF, plain}, {0xB000, ligature}} {
		face := fontWithLayout(t, map[string][]byte{"morx": morxAuditTable(morxLigatureFlags(robotoB, test.flags))})
		testShapedFace(t, face, []shapedText{{"ab", test.want}})
	}
}

// testShapedFeatures shapes the text in the face with the features, and
// compares the glyphs with the glyphs of HarfBuzz.
func testShapedFeatures(t *testing.T, face *font.Face, features []Feature, test shapedText) {
	t.Helper()
	buf := NewBuffer()
	buf.AddRunes([]rune(test.text), 0, -1)
	buf.GuessSegmentProperties()
	buf.Shape(NewFont(face), features)
	if len(buf.Info) != len(test.want) {
		t.Fatalf("%q: got %d glyphs, want %d", test.text, len(buf.Info), len(test.want))
	}
	for i, want := range test.want {
		info, pos := buf.Info[i], buf.Pos[i]
		if got := (shapedGlyph{info.Glyph, info.Cluster, pos.XAdvance, pos.XOffset, pos.YOffset}); got != want {
			t.Errorf("%q, glyph %d: got %+v, want %+v", test.text, i, got, want)
		}
	}
}

// TestKerxFormat4KernOff shapes ab with the feature kern off, and with a
// table kerx whose subtable of format 4 attaches b to a with the anchors
// (50, 60) and (10, 20). As in HarfBuzz 15.1.0, the subtable applies when
// the feature kern is off, also when it is not cross-stream. go-text did
// not apply it (D73).
func TestKerxFormat4KernOff(t *testing.T) {
	marks := stateMachine{
		lookup:   lookup8(robotoA, 4, 5),
		nClasses: 6,
		states:   [][]int{{0, 0, 0, 0, 1, 2}, {0, 0, 0, 0, 1, 2}},
		entries:  [][]int{{0, 0, kerxEnd}, {0, kerxMark, kerxEnd}, {0, 0, 0}},
	}
	kerx := kerxTable(kerxSubtable{coverage: kerxFormat4, data: marks.bytes([]uint32{kerx4Coordinates}, u16s(50, 60, 10, 20))})
	kernOff := []Feature{{Tag: ot.NewTag('k', 'e', 'r', 'n'), End: FeatureGlobalEnd}}
	testShapedFeatures(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), kernOff,
		shapedText{"ab", []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoB, 1, 1149, -1074, 40}}})
}

// binSearchLookup builds a lookup of format 2, 4 or 6 with the units, each
// padded to unitSize bytes, then the tail.
func binSearchLookup(format, unitSize int, units [][]int, tail ...int) []byte {
	out := u16s(format, unitSize, len(units), 0, 0, 0)
	for _, unit := range units {
		out = append(out, u16s(unit...)...)
		out = append(out, make([]byte, unitSize-2*len(unit))...)
	}
	return append(out, u16s(tail...)...)
}

// TestLookupBinarySearchUnits shapes texts with lookups of format 2, 4 and
// 6. As VarSizedBinSearchArrayOf of HarfBuzz 15.1.0 reads them, the last
// unit is dropped only when it ends the search, and the units are unitSize
// bytes apart. A segment of format 4 whose first glyph is after its last
// glyph is an error, and the offset 0 gives the values at the start of the
// lookup:
//   - A contextual subtable replaces c with d. Its class table is a lookup
//     of format 4 whose one segment gives the class 1 to c. go-text dropped
//     the segment, and did not replace c.
//   - In a chain of morx, a noncontextual subtable has a lookup of format 4
//     with no unit, a segment of a, b whose first glyph is after its last
//     glyph, or a segment that replaces a, b with x, y. A second subtable
//     replaces a with b. go-text read 65,535 segments for no unit, and
//     refused the table. It made an array of a negative length for the
//     segment of b, a.
//   - In a chain of morx, a noncontextual subtable deletes a. Then a
//     noncontextual subtable, whose lookup of format 2 replaces c with x,
//     has a last unit that ends the search and gives c to the glyph
//     0xFFFF. A contextual subtable then replaces b with d after the
//     deleted glyph. go-text replaced the deleted glyph with c, so b was
//     not after the deleted glyph.
//   - A noncontextual subtable has a lookup of format 4 whose segment of c
//     has the offset 0. HarfBuzz reads the value 4, the format of the
//     lookup. go-text gave the segment no values.
//
// (D73)
func TestLookupBinarySearchUnits(t *testing.T) {
	cd := u16s(8, int(robotoC), 1, int(robotoD))
	classTable := binSearchLookup(4, 6, [][]int{{int(robotoC), int(robotoC), 18}}, 1)
	morx := morxAuditTable(morxContextualStates(classTable, u16s(0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 0, 0), cd))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"morx": morx}), []shapedText{
		{"c", []shapedGlyph{{robotoD, 0, 1155, 0, 0}}},
	})

	ab := morxAuditSubtable{4, lookup8(robotoA, int(robotoB))}
	for _, test := range []struct {
		lookup []byte
		want   []shapedGlyph
	}{
		{binSearchLookup(4, 6, nil), []shapedGlyph{{robotoB, 0, 1149, 0, 0}}},
		{binSearchLookup(4, 6, [][]int{{int(robotoA), int(robotoB), 18}}, int(robotoX), int(robotoY)),
			[]shapedGlyph{{robotoA, 0, 1114, 0, 0}}},
		{binSearchLookup(4, 6, [][]int{{int(robotoB), int(robotoA), 18}}, int(robotoX), int(robotoY)),
			[]shapedGlyph{{robotoX, 0, 1015, 0, 0}}},
	} {
		morx := morxAuditTable(morxAuditSubtable{4, test.lookup}, ab)
		testShapedFace(t, fontWithLayout(t, map[string][]byte{"morx": morx}), []shapedText{{"a", test.want}})
	}

	cx := binSearchLookup(2, 6, [][]int{{int(robotoC), int(robotoC), int(robotoX)}, {0xFFFF, 0xFFFF, int(robotoC)}})
	morx = morxAuditTable(morxAuditSubtable{4, lookup8(robotoA, 0xFFFF)}, morxAuditSubtable{4, cx},
		morxContextual(u16s(8, 0xFFFE, 2, 1, 1), u16s(8, int(robotoB), 1, int(robotoD))))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"morx": morx}), []shapedText{
		{"abc", []shapedGlyph{{robotoD, 0, 1155, 0, 0}, {robotoX, 2, 1015, 0, 0}}},
	})

	morx = morxAuditTable(morxAuditSubtable{4, binSearchLookup(4, 6, [][]int{{int(robotoC), int(robotoC), 0}})})
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"morx": morx}), []shapedText{
		{"c", []shapedGlyph{{4, 0, 507, 0, 0}}},
	})
}

// lookup10 builds a lookup of format 10 whose values have size bytes, for
// the glyphs from first on.
func lookup10(size int, first GID, values ...uint32) []byte {
	out := u16s(10, size, int(first), len(values))
	for _, v := range values {
		out = append(out, u32s(v)[4-size:]...)
	}
	return out
}

// kerxFormat6 builds a subtable of format 6 of kerx with values of two
// bytes, the lookups of the rows and the columns, and the kerning array.
func kerxFormat6(rows, columns []byte, array ...int) kerxSubtable {
	return kerxFormat6Data(0, 2, rows, columns, u16s(array...), nil)
}

// kerxFormat6Data builds a subtable of format 6 of kerx with the flags, the
// counts of the rows and the columns, the lookups, the kerning array and
// the values of the tuples. The vector of the tuples follows the array.
func kerxFormat6Data(flags uint32, count int, rows, columns, array, vector []byte) kerxSubtable {
	data := u32s(flags)
	data = append(data, u16s(count, count)...)
	arrayOffset := uint32(36 + len(rows) + len(columns))
	data = append(data, u32s(36, uint32(36+len(rows)), arrayOffset, arrayOffset+uint32(len(array)))...)
	data = append(data, rows...)
	data = append(data, columns...)
	data = append(data, array...)
	data = append(data, vector...)
	return kerxSubtable{coverage: 6, data: data}
}

// TestLookupFormat10 shapes texts with lookups of format 10. As in
// HarfBuzz 15.1.0, only the rows and the columns of kerx format 6 read a
// value from such a lookup, with the size that the lookup gives, and keep
// the low 16 bits of a value. A class table, a substitution of morx and
// the classes of kerx format 2 find no value:
//   - The class table of a contextual subtable gives b the class 1, and c
//     the class 3. Both have the class 1, which replaces c with d.
//   - A noncontextual subtable does not replace a with b.
//   - A subtable of format 2 of kerx, whose left classes give 6 to b, does
//     not kern b, b.
//   - A subtable of format 6 of kerx, whose rows give 0 to a and 2 to b,
//     and whose columns give 0 to a and 1 to b, with values of 4 bytes.
//
// go-text read each value as 2 bytes, and in each place (D73).
func TestLookupFormat10(t *testing.T) {
	states := u16s(0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 0, 0)
	morx := morxAuditTable(morxContextualStates(lookup10(1, robotoB, 1, 3), states, u16s(8, int(robotoC), 1, int(robotoD))))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"morx": morx}), []shapedText{
		{"bc", []shapedGlyph{{robotoB, 0, 1149, 0, 0}, {robotoD, 1, 1155, 0, 0}}},
	})

	morx = morxAuditTable(morxAuditSubtable{4, lookup10(2, robotoA, uint32(robotoB))})
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"morx": morx}), []shapedText{
		{"a", []shapedGlyph{{robotoA, 0, 1114, 0, 0}}},
	})

	kerx := kerxTable(kerxFormat2Lookups(lookup10(2, robotoB, 6), u16s(8, int(robotoA), 3, 0, 2, 4)))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), []shapedText{
		{"bb", []shapedGlyph{{robotoB, 0, 1149, 0, 0}, {robotoB, 1, 1149, 0, 0}}},
	})

	kerx = kerxTable(kerxFormat6(lookup10(4, robotoA, 0, 0x10002), lookup10(4, robotoA, 0x30000, 1), 0, -100, -50, -20))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), []shapedText{
		{"ab", []shapedGlyph{{robotoA, 0, 1064, 0, 0}, {robotoB, 1, 1099, -50, 0}}},
		{"ba", []shapedGlyph{{robotoB, 0, 1124, 0, 0}, {robotoA, 1, 1089, -25, 0}}},
		{"bb", []shapedGlyph{{robotoB, 0, 1139, 0, 0}, {robotoB, 1, 1139, -10, 0}}},
	})
}

// kernFormat2 builds a table kern of Apple with a subtable of format 2 for
// a, b and c. The rows of the left classes are 36, 42 and 48 bytes from the
// start of the subtable, where the kerning array starts at 36, and the
// right classes are 0, 3 and 5 bytes. The array has three rows: 0, 0, 0,
// then 0, -80, 30, then 0, -50, -20.
func kernFormat2() []byte {
	data := u16s(6, 16, 26, 36)
	data = append(data, u16s(int(robotoA), 3, 36, 42, 48)...)
	data = append(data, u16s(int(robotoA), 3, 0, 3, 5)...)
	data = append(data, u16s(0, 0, 0, 0, -80, 30, 0, -50, -20)...)
	kern := u32s(0x00010000, 1, uint32(8+len(data)), 0x00020000)
	return append(kern, data...)
}

// TestKernFormat2 shapes texts with kernFormat2. As in HarfBuzz 15.1.0, the
// sum of the classes is a byte offset from the start of the subtable, and
// its distance from the kerning array is rounded down to a value of two
// bytes. go-text read the value at an odd distance, from the bytes of two
// values (D73).
func TestKernFormat2(t *testing.T) {
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kern": kernFormat2()}), []shapedText{
		{"bb", []shapedGlyph{{robotoB, 0, 1109, 0, 0}, {robotoB, 1, 1109, -40, 0}}},
		{"cc", []shapedGlyph{{robotoC, 0, 1062, 0, 0}, {robotoC, 1, 1062, -10, 0}}},
	})
}

// TestKerxTupleCount shapes texts with subtables of format 0 and 2 of kerx
// with a tuple count of 1. As in HarfBuzz 15.1.0, a value is a byte offset
// from the start of the subtable to the values of the tuples, and gives 0
// when the tuples are not in the subtable, or not in the table for the
// last subtable. In format 2, a kerning array that starts after the end of
// the subtable is an error:
//   - A subtable of format 0 kerns a, b with the offset 52, b, c with the
//     offset -2 and c, c with the offset 56. Its values of the tuples follow
//     its pairs: -100, -60, then -40 for the last subtable, which ends
//     there.
//   - A subtable of format 2, whose kerning array gives the offset 58 to
//     a, b, to the value -80 after the array.
//   - A subtable of format 2 whose kerning array starts after its end,
//     then a subtable that kerns a, b.
//
// go-text read a value of format 0 as an offset from the end of the header
// of the subtable, and refused the table when the offset was outside the
// subtable. It ignored the tuple count in format 2, and the start of the
// kerning array (D73).
func TestKerxTupleCount(t *testing.T) {
	pairs := kerxPairs(int(robotoA), int(robotoB), 52, int(robotoA), int(robotoC), 54,
		int(robotoB), int(robotoC), -2, int(robotoC), int(robotoC), 56)
	pairs.data = append(pairs.data, u16s(-100, -60)...)
	pairs.tuples = 1
	last := pairs
	last.data = append(slices.Clone(last.data), u16s(-40)...)
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerxTable(last)}), []shapedText{
		{"ab", []shapedGlyph{{robotoA, 0, 1064, 0, 0}, {robotoB, 1, 1099, -50, 0}}},
		{"bc", []shapedGlyph{{robotoB, 0, 1149, 0, 0}, {robotoC, 1, 1072, 0, 0}}},
		{"cc", []shapedGlyph{{robotoC, 0, 1052, 0, 0}, {robotoC, 1, 1052, -20, 0}}},
	})
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerxTable(pairs, kerxPairs())}), []shapedText{
		{"cc", []shapedGlyph{{robotoC, 0, 1072, 0, 0}, {robotoC, 1, 1072, 0, 0}}},
	})

	left := u16s(8, int(robotoA), 3, 0, 6, 6)
	right := u16s(8, int(robotoA), 3, 0, 2, 4)
	data := u32s(6, 28, uint32(28+len(left)), uint32(28+len(left)+len(right)))
	data = append(data, left...)
	data = append(data, right...)
	data = append(data, u16s(0, 58, 60, -80, -50)...)
	format2 := kerxSubtable{coverage: 2, data: data, tuples: 1}
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerxTable(format2)}), []shapedText{
		{"ab", []shapedGlyph{{robotoA, 0, 1074, 0, 0}, {robotoB, 1, 1109, -40, 0}}},
	})

	late := kerxSubtable{coverage: 2, data: u32s(6, 28, 28, 100)}
	late.data = append(late.data, u16s(8, int(robotoA), 1, 0)...)
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerxTable(late, kerxPairs(int(robotoA), int(robotoB), -300))}), []shapedText{
		{"ab", []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoB, 1, 1149, 0, 0}}},
	})
}

// TestLtagRange shapes a in sr with the table of TestLtagLanguage, whose
// table ltag has a third tag that ends after the end of the table, or has
// the version 0. As in HarfBuzz 15.1.0, the whole table ltag is refused, so
// that no tag matches the language. go-text refused only the third tag,
// and gave b for sr (D73).
func TestLtagRange(t *testing.T) {
	long := ltagTable("sr", "Zh_Hant", "en@x")
	copy(long[12+2*4+2:], u16s(100))
	version0 := ltagTable("sr", "Zh_Hant", "en@x")
	version0[3] = 0
	for _, ltag := range [][]byte{long, version0} {
		hf := NewFont(fontWithLayout(t, map[string][]byte{"morx": morxLanguages(), "ltag": ltag}))
		buf := NewBuffer()
		buf.AddRunes([]rune("a"), 0, -1)
		buf.Props.Language = language.NewLanguage("sr")
		buf.GuessSegmentProperties()
		buf.Shape(hf, nil)
		if got := buf.Info[0].Glyph; got != robotoA {
			t.Errorf("a in sr: got the glyph %d, want %d", got, robotoA)
		}
	}
}

// kerxFormat4Actions builds a subtable of format 4 of kerx with the action
// type, whose class table gives the classes 4, 5 and 6 to a, b and c. a
// marks itself, b marks itself and applies the action 1, and c applies the
// action 0. The actions follow the state table.
func kerxFormat4Actions(actionType uint32, actions []byte) kerxSubtable {
	marks := stateMachine{
		lookup:   lookup8(robotoA, 4, 5, 6),
		nClasses: 7,
		states:   [][]int{{0, 0, 0, 0, 1, 2, 3}, {0, 0, 0, 0, 1, 2, 3}},
		entries:  [][]int{{0, 0, kerxEnd}, {0, kerxMark, kerxEnd}, {0, kerxMark, 1}, {0, 0, 0}},
	}
	return kerxSubtable{coverage: kerxFormat4, data: marks.bytes([]uint32{actionType << 30}, actions)}
}

// TestKerxFormat4Actions shapes texts with subtables of
// kerxFormat4Actions. As in HarfBuzz 15.1.0, an action is read when the
// subtable applies it:
//   - In a subtable of the action type 2 with only the action 0, the
//     action 1 of b is not in the subtable. The transition of b stops, and
//     b does not mark itself, so c attaches to a.
//   - A subtable of the action type 1, and no table ankr. The anchors are
//     at (0, 0).
//   - A subtable of the action type 3 has no data. c attaches to a.
//   - A subtable of the action type 2 whose actions start at its end, then
//     a subtable that kerns a, c. No action applies.
//
// go-text required the action of the largest index of an entry in the
// subtable, and the action types 0 to 2. It refused the table for these
// subtables, and panicked for an action of the type 1 without a table
// ankr (D73).
func TestKerxFormat4Actions(t *testing.T) {
	kerx := kerxTable(kerxFormat4Actions(2, u16s(50, 60, 10, 20)))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), []shapedText{
		{"abc", []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoB, 1, 1149, 0, 0}, {robotoC, 2, 1072, -2223, 40}}},
	})
	kerx = kerxTable(kerxFormat4Actions(1, u16s(0, 1, 0, 1)))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), []shapedText{
		{"ac", []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoC, 1, 1072, -1114, 0}}},
	})
	kerx = kerxTable(kerxFormat4Actions(3, nil))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), []shapedText{
		{"ac", []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoC, 1, 1072, -1114, 0}}},
	})
	kerx = kerxTable(kerxFormat4Actions(2, nil), kerxPairs(int(robotoA), int(robotoC), -300))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), []shapedText{
		{"ac", []shapedGlyph{{robotoA, 0, 964, 0, 0}, {robotoC, 1, 922, -150, 0}}},
	})
}

// TestKerxFormat6Values shapes texts with subtables of format 6 of kerx,
// whose rows give 0 to a and 2 to b, and whose columns give 0 to a and 1 to
// b. As in HarfBuzz 15.1.0, the value at the sum of a row and a column is
// read when the subtable applies, and is 0 when it is not in the subtable.
// The counts of the rows and the columns do not bound it. Long values
// have 32 bits, and so do the values of their lookups of format 8. With a
// tuple count, a value is a byte offset from the vector of the tuples:
//   - The kerning array 0, -100, -50, with counts of 1, kerns a, b and b,
//     a, and not b, b.
//   - The long values 0, -100, -50, -20, with lookups of format 8.
//   - The long values 0, 2, 4, 6, which are offsets to the tuples -10, -20,
//     -30 and -40, with a tuple count of 1.
//
// go-text read rowCount × columnCount values, and the values of a lookup
// of format 8 with long values as 16 bits. It read the values of the tuples
// when it parsed the table (D73).
func TestKerxFormat6Values(t *testing.T) {
	rows, columns := lookup8(robotoA, 0, 2), lookup8(robotoA, 0, 1)
	kerx := kerxTable(kerxFormat6Data(0, 1, rows, columns, u16s(0, -100, -50), nil))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), []shapedText{
		{"ab", []shapedGlyph{{robotoA, 0, 1064, 0, 0}, {robotoB, 1, 1099, -50, 0}}},
		{"ba", []shapedGlyph{{robotoB, 0, 1124, 0, 0}, {robotoA, 1, 1089, -25, 0}}},
		{"bb", []shapedGlyph{{robotoB, 0, 1149, 0, 0}, {robotoB, 1, 1149, 0, 0}}},
	})

	rows = append(u16s(8, int(robotoA), 2), u32s(0, 2)...)
	columns = append(u16s(8, int(robotoA), 2), u32s(0, 1)...)
	kerx = kerxTable(kerxFormat6Data(1, 2, rows, columns, u32s(0, 0xFFFFFF9C, 0xFFFFFFCE, 0xFFFFFFEC), nil))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), []shapedText{
		{"bb", []shapedGlyph{{robotoB, 0, 1139, 0, 0}, {robotoB, 1, 1139, -10, 0}}},
	})

	tuples := kerxFormat6Data(1, 2, rows, columns, u32s(0, 2, 4, 6), u16s(-10, -20, -30, -40))
	tuples.tuples = 1
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerxTable(tuples)}), []shapedText{
		{"ab", []shapedGlyph{{robotoA, 0, 1104, 0, 0}, {robotoB, 1, 1139, -10, 0}}},
	})
}

// morxField gives the subtable with the 32-bit value v at the byte offset
// at of its data.
func morxField(st morxAuditSubtable, at int, v uint32) morxAuditSubtable {
	st.data = slices.Clone(st.data)
	copy(st.data[at:], u32s(v))
	return st
}

// TestMorxLazyData shapes abc with subtables of morx, then a noncontextual
// subtable that replaces c with x. As in HarfBuzz 15.1.0, the data of a
// subtable is read when the subtable applies:
//   - The ligature subtable of morxLigature, whose actions start after the
//     end of the table, does not replace a, b with x. When its components
//     follow its ligatures, it does. When the offset of its components is
//     0, the table is refused.
//   - An insertion subtable whose insertion after a is after the end of the
//     table inserts no glyph. When the offset of its insertions is 0, the
//     table is refused.
//   - A contextual subtable that replaces a with d, whose substitutions are
//     at the offset 0, or whose one substitution is at the offset 0, which
//     is the start of the offsets.
//
// go-text refused the table for an action, a component or an insertion
// that it could not read, and ignored the offsets 0. It panicked when an
// entry read a substitution or an insertion at such an offset (D73).
func TestMorxLazyData(t *testing.T) {
	cx := morxAuditSubtable{4, lookup8(robotoC, int(robotoX))}
	ligature := morxLigature()
	sorted := slices.Clone(ligature.data)
	n := len(sorted)
	sorted[n-4], sorted[n-3], sorted[n-2], sorted[n-1] = sorted[n-2], sorted[n-1], sorted[n-4], sorted[n-3]
	components := binary.BigEndian.Uint32(sorted[20:])
	unsorted := morxField(morxField(morxAuditSubtable{2, sorted}, 20, components+2), 24, components)

	insertion := stateMachine{
		lookup:   lookup8(robotoA, 4),
		nClasses: 5,
		states:   [][]int{{0, 0, 0, 0, 1}, {0, 0, 0, 0, 1}},
		entries:  [][]int{{0, 0, insertNone, insertNone}, {0, insertCurrentOne, 100, insertNone}},
	}
	far := morxAuditSubtable{5, insertion.bytes([]uint32{0}, u16s(int(robotoB)))}

	contextual := morxContextualStates(u16s(8, int(robotoA), 1, 4), u16s(0, 0, 0, 0, 2, 0, 0, 0, 0, 2, 0, 0, 0, 0, 2), u16s(8, int(robotoA), 1, int(robotoD)))
	contextual = morxField(contextual, 0, 5)
	a, b, c, x := shapedGlyph{robotoA, 0, 1114, 0, 0}, shapedGlyph{robotoB, 1, 1149, 0, 0}, shapedGlyph{robotoC, 2, 1072, 0, 0}, shapedGlyph{robotoX, 2, 1015, 0, 0}
	refused := []shapedGlyph{a, b, c}
	for _, test := range []struct {
		st   morxAuditSubtable
		want []shapedGlyph
	}{
		{morxField(ligature, 16, 10000), []shapedGlyph{a, b, x}},
		{unsorted, []shapedGlyph{{robotoX, 0, 1015, 0, 0}, x}},
		{morxField(ligature, 20, 0), refused},
		{far, []shapedGlyph{a, b, x}},
		{morxField(far, 16, 0), refused},
		{morxField(contextual, 16, 0), refused},
		{morxField(contextual, int(binary.BigEndian.Uint32(contextual.data[16:])), 0), refused},
	} {
		testShapedFace(t, fontWithLayout(t, map[string][]byte{"morx": morxAuditTable(test.st, cx)}), []shapedText{{"abc", test.want}})
	}
}

// kernFormat3 builds a table kern of Apple with a subtable of format 3 for
// the glyphs 0 to 70, which leaves out c. The left classes give 1 to a and
// b, and 7 to the glyph 10, more than the 2 left classes. The right classes
// give 1 to b and 2 to the glyph 50. The kerning values are 0 and -100,
// and the indexes of the classes are 0, 1, 5, then 0, 1, 0, where 5 is not
// a kerning value.
func kernFormat3() []byte {
	const glyphs = robotoC
	data := u16s(int(glyphs))
	data = append(data, 2, 2, 3, 0)
	data = append(data, u16s(0, -100)...)
	left := make([]byte, glyphs)
	left[robotoA], left[robotoB], left[10] = 1, 1, 7
	right := make([]byte, glyphs)
	right[robotoB], right[50] = 1, 2
	data = append(data, left...)
	data = append(data, right...)
	data = append(data, 0, 1, 5, 0, 1, 0)
	kern := u32s(0x00010000, 1, uint32(8+len(data)), 0x00030000)
	return append(kern, data...)
}

// TestKernFormat3 shapes texts with kernFormat3. As in HarfBuzz 15.1.0, the
// classes and the kerning values are read when the subtable applies, so a
// class or an index out of range is not an error. c, after the glyphs of
// the subtable, has the class 0, and the subtable applies to it. go-text
// refused the table, and did not kern c (D73).
func TestKernFormat3(t *testing.T) {
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kern": kernFormat3()}), []shapedText{
		{"ab", []shapedGlyph{{robotoA, 0, 1064, 0, 0}, {robotoB, 1, 1099, -50, 0}}},
		{"cb", []shapedGlyph{{robotoC, 0, 1022, 0, 0}, {robotoB, 1, 1099, -50, 0}}},
	})
}
