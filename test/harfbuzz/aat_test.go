package harfbuzz_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/harfbuzz"
	"github.com/xo/okage/typesetting/language"
)

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
	return morxLigatureAt(robotoB)
}

// morxLigatureAt builds the subtable of morxLigature, whose first action
// reads the component at glyph minus the glyph on the stack.
func morxLigatureAt(glyph int) morxAuditSubtable {
	return morxLigatureFlags(glyph, 0xA000)
}

// morxLigatureFlags builds the subtable of morxLigatureAt, where the entry
// of b has the flags.
func morxLigatureFlags(glyph, flags int) morxAuditSubtable {
	classTable := u16s(8, robotoA, 2, 4, 5)
	states := u16s(0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 2)
	entries := u16s(0, 0, 0, 2, 0x8000, 0, 0, flags, 0)
	actions := u32s(ligatureOffset(glyph), 0xC0000000|ligatureOffset(robotoA))
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
	return kerxFormat2Lookups(u16s(8, robotoA, 4, 0, 6, 12, 18), u16s(8, robotoA, 4, 0, 3, 4, 2))
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

// TestLookupFormat10ValueSize: a table kerx whose first subtable has a
// lookup of format 10 for its left classes, then a subtable of format 0
// that kerns a, b. HarfBuzz refuses a lookup of format 10 whose values
// have 0 bytes, from its commit a763a612d1, or more than 4 bytes, and then
// refuses the whole table. HarfBuzz reads no class from a lookup of format
// 10 in a subtable of format 2.
func TestLookupFormat10ValueSize(t *testing.T) {
	for _, size := range []int{0, 1, 2, 4, 5} {
		left := u16s(10, size, robotoA, 3, 0, 6, 12)
		kerx := kerxTable(kerxFormat2Lookups(left, u16s(8, robotoA, 3, 0, 2, 4)), kerxPairs(robotoA, robotoB, -300))
		testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, "ab", "aab", "bb", "cb", "bc")
	}
}

// kernNegativeState builds a table kern of Apple with a subtable of format
// 1, whose state array starts after the row of the state -1. The class of
// a and b is 4. In the state 0, a or b pushes the glyph and goes to the
// state -1. In the state -1, a or b pushes the glyph, applies the list -40,
// -29 and goes to the state 0.
func kernNegativeState() []byte {
	data := u16s(5, 10, 21, 26, 38)
	data = append(data, u16s(robotoA, 2)...)
	data = append(data, 4, 4)
	data = append(data, 0, 0, 0, 0, 2) // the state -1
	data = append(data, 0, 0, 0, 0, 1) // the state 0
	data = append(data, u16s(21, 0, 16, kerxPush, 21, kerxPush|38)...)
	data = append(data, u16s(-40, -29)...)
	kern := u32s(0x00010000, 1, uint32(8+len(data)), 0x00010000)
	return append(kern, data...)
}

// TestKernNegativeState: the subtable of kernNegativeState. HarfBuzz reads
// a state before the state array as a negative state, from its commit
// ec4a38c4ab. Before it, HarfBuzz refused the table.
func TestKernNegativeState(t *testing.T) {
	testCase(t, robotoWith(t, map[string][]byte{"kern": kernNegativeState()}), false, "ab", "ba", "aab", "abab", "abc")
}

// TestLookupSegmentsOutOfOrder: a contextual subtable whose class table is
// a lookup of format 2 with the segments x, a and z, out of order. In each
// state, the class 1, out of bounds, replaces c with d. From its commit
// 77be62a733, HarfBuzz adds the glyphs from a to z to the set of the
// subtable, at the first segment out of order. So c is in the set, and
// the subtable applies.
func TestLookupSegmentsOutOfOrder(t *testing.T) {
	classTable := u16s(2, 6, 3, 12, 1, 6, robotoX, robotoX, 1, robotoA, robotoA, 1, robotoZ, robotoZ, 1)
	states := u16s(0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 0, 0)
	morx := morxAuditTable(morxContextualStates(classTable, states, u16s(8, robotoC, 1, robotoD)))
	testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "c", "bcb", "cccc", "bbbb")
}

// shapeOptions are the options of compareOptions. When language is not
// empty, it is the language of the buffer. When ranged is not empty, each
// character i of a text has the feature ranged in a range of its own, on
// when i is not a multiple of period and off when it is. When off is not
// empty, the feature off is off for the whole text.
type shapeOptions struct {
	language string
	ranged   string
	period   int
	off      string
}

// compareOptions shapes each text in the font data with harfbuzz and with
// HarfBuzz, with the options, and fails the test for each difference.
func compareOptions(t *testing.T, data []byte, opts shapeOptions, texts ...string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "font.ttf")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var args []string
	if opts.language != "" {
		args = append(args, "--language", opts.language)
	}
	if opts.ranged != "" {
		args = append(args, "--ranged", opts.ranged, strconv.Itoa(opts.period))
	}
	if opts.off != "" {
		args = append(args, "--off", opts.off)
	}
	want := shapeHarfBuzz(t, path, texts, false, args...)
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	hf := harfbuzz.NewFont(face)
	for i, text := range texts {
		runes := []rune(text)
		buf := harfbuzz.NewBuffer()
		buf.AddRunes(runes, 0, -1)
		if opts.language != "" {
			buf.Props.Language = language.NewLanguage(opts.language)
		}
		buf.GuessSegmentProperties()
		var features []harfbuzz.Feature
		if opts.ranged != "" {
			tag := ot.MustNewTag(opts.ranged)
			for j := range runes {
				value := uint32(0)
				if j%opts.period != 0 {
					value = 1
				}
				features = append(features, harfbuzz.Feature{Tag: tag, Value: value, Start: j, End: j + 1})
			}
		}
		if opts.off != "" {
			features = append(features, harfbuzz.Feature{Tag: ot.MustNewTag(opts.off), End: harfbuzz.FeatureGlobalEnd})
		}
		buf.Shape(hf, features)
		var got []string
		for j, info := range buf.Info {
			pos := buf.Pos[j]
			got = append(got, fmt.Sprintf("%d %d %d %d %d %d", info.Glyph, info.Cluster, pos.XAdvance, pos.YAdvance, pos.XOffset, pos.YOffset))
		}
		if !slices.Equal(got, want[i].glyphs) {
			t.Errorf("%+q with %+v: got %d glyphs, HarfBuzz gives %d", trim(text), opts, len(got), len(want[i].glyphs))
			for j := range min(len(got), len(want[i].glyphs)) {
				if got[j] != want[i].glyphs[j] {
					t.Errorf("  glyph %d: got %q, HarfBuzz gives %q", j, got[j], want[i].glyphs[j])
					break
				}
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
	replacements := []int{robotoB, robotoC, robotoD, robotoX}
	length := 16 + 12*len(features) + len(replacements)*(12+8)
	out := u16s(2, 0)
	out = append(out, u32s(1, 1, uint32(length), uint32(len(features)), uint32(len(replacements)))...)
	for _, f := range features {
		out = append(out, u16s(39, f.setting)...)
		out = append(out, u32s(uint32(f.flags), 0xFFFFFFFF)...)
	}
	for i, glyph := range replacements {
		out = append(out, u32s(20, 0x20000004, uint32(features[i].flags))...)
		out = append(out, u16s(8, robotoA, 1, glyph)...)
	}
	return out
}

// TestLtagLanguage: a table morx whose features of the type 39 turn on
// subtables for the tags sr, Zh_Hant and en@x of the table ltag, and for a
// tag 9 that the table does not have. From its commit cd570b6f54,
// HarfBuzz compares the canonical form of a tag with the language of the
// buffer. The canonical form stops before @. A tag matches the language,
// or the start of the language up to a '-'.
func TestLtagLanguage(t *testing.T) {
	data := robotoWith(t, map[string][]byte{"morx": morxLanguages(), "ltag": ltagTable("sr", "Zh_Hant", "en@x")})
	for _, lang := range []string{"sr", "SR", "sr-Latn-RS", "srb", "s", "zh-hant", "zh-hant-tw", "zh", "en", "en-us", "enx", "fr"} {
		compareOptions(t, data, shapeOptions{language: lang}, "a", "aa")
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
	out = append(out, u16s(8, robotoA, 1, robotoB)...)
	for range empty {
		out = append(out, u32s(0, 16, 0, 0)...)
	}
	return out
}

// TestMorxMapBudget: a text of 2,000 a, whose characters each have the
// feature liga in a range of their own, on at an odd index and off at an
// even index. The table morx is morxBudget with 10,000 extra feature
// records and 1,000 empty chains. From its commit 370a0da5a1, HarfBuzz
// extends the last range of a chain when the next range has the same
// flags, and stops the compilation of the map when it runs out of its
// budget. The last range then goes to the end of the text. The extra
// feature records and the chains take from the budget, and so do the
// ranges that the map keeps.
func TestMorxMapBudget(t *testing.T) {
	data := robotoWith(t, map[string][]byte{"morx": morxBudget(10000, 1000), "feat": featLigatures()})
	compareOptions(t, data, shapeOptions{ranged: "liga", period: 2}, "aaaa", strings.Repeat("a", 2000))
}

// TestMorxLigatureNegativeComponent shapes texts with a ligature subtable
// whose first action reads the component at the index -1. HarfBuzz stops
// the action, and harfbuzz panicked before fix 27 of D73.
func TestMorxLigatureNegativeComponent(t *testing.T) {
	morx := morxAuditTable(morxLigatureAt(robotoB + 1))
	testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "ab", "abc", "cab")
}

// TestMorxLigaturePerformAction shapes texts with a ligature subtable that
// replaces a, b with x, where the entry of b has the flag SetComponent and
// other flags. HarfBuzz performs the action of an entry only when it has
// the flag PerformAction, 0x2000, and ignores the reserved bits 0x1FFF.
func TestMorxLigaturePerformAction(t *testing.T) {
	for _, flags := range []int{0xA000, 0x8001, 0x9000, 0x8FFF, 0xB000} {
		morx := morxAuditTable(morxLigatureFlags(robotoB, flags))
		testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "ab", "abc", "cab", "aab")
	}
}

// TestKerxFormat4KernOff: the table of TestKerxFormat4CrossOffset, and a
// table with only its subtable of format 4, with the feature kern off.
// HarfBuzz applies a subtable of format 4 of kerx when the feature kern is
// off, also when the subtable is not cross-stream.
func TestKerxFormat4KernOff(t *testing.T) {
	marks := stateMachine{
		lookup:   lookup8(robotoA, 4, 5),
		nClasses: 6,
		states:   [][]int{{0, 0, 0, 0, 1, 2}, {0, 0, 0, 0, 1, 2}},
		entries:  [][]int{{0, 0, kerx4NoActionData}, {0, kerxMark, kerx4NoActionData}, {0, 0, 0}},
	}
	only := kerxTable(kerxSubtable{coverage: kerxFormat4, data: marks.bytes([]uint32{kerx4Coordinates}, u16s(50, 60, 10, 20))})
	for _, kerx := range [][]byte{kerxMarks(kerxFormat4, []int{100}, []int{50, 60, 10, 20}), only} {
		data := robotoWith(t, map[string][]byte{"kerx": kerx})
		for _, off := range []string{"", "kern"} {
			compareOptions(t, data, shapeOptions{off: off}, "ab", "aab", "abb", "ba")
		}
	}
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

// TestLookupBinarySearchUnits: lookups of format 2, 4 and 6, as HarfBuzz
// reads VarSizedBinSearchArrayOf. It drops the last unit only when that
// unit ends the search, with two words 0xFFFF in formats 2 and 4, and one
// word in format 6. The units are unitSize bytes apart. It refuses a
// segment of format 4 whose first glyph is after its last glyph, and reads
// the values of the offset 0 at the start of the lookup:
//   - The class table of a contextual subtable is a lookup of format 4
//     that gives the class 1 to c, without a unit that ends the search,
//     with one, and with units of 8 bytes. In each state, the class 1
//     replaces c with d.
//   - In a chain of morx, the first subtable is noncontextual with a
//     lookup of format 4 with no unit, with a segment whose first glyph is
//     after its last glyph, or with a segment that replaces a, b with x, y.
//     The second subtable replaces a with b.
//   - In a chain of morx, a noncontextual subtable deletes a, then a
//     noncontextual subtable replaces c with x, and its last unit, which
//     ends the search, gives c to the glyph 0xFFFF. A contextual subtable
//     then replaces b with d after the deleted glyph.
//   - A noncontextual subtable whose lookup of format 4 has a segment for
//     c with the offset 0.
func TestLookupBinarySearchUnits(t *testing.T) {
	states := u16s(0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 0, 0)
	cd := u16s(8, robotoC, 1, robotoD)
	for _, classTable := range [][]byte{
		binSearchLookup(4, 6, [][]int{{robotoC, robotoC, 18}}, 1),
		binSearchLookup(4, 6, [][]int{{robotoC, robotoC, 24}, {0xFFFF, 0xFFFF, 0}}, 1),
		binSearchLookup(4, 8, [][]int{{robotoC, robotoC, 28}, {0xFFFF, 0xFFFF, 0}}, 1),
	} {
		morx := morxAuditTable(morxContextualStates(classTable, states, cd))
		testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "c", "bcb", "cccc")
	}

	ab := morxAuditSubtable{4, lookup8(robotoA, robotoB)}
	for _, lookup := range [][]byte{
		binSearchLookup(4, 6, nil),
		binSearchLookup(4, 6, [][]int{{robotoA, robotoB, 18}}, robotoX, robotoY),
		binSearchLookup(4, 6, [][]int{{robotoB, robotoA, 18}}, robotoX, robotoY),
		binSearchLookup(4, 6, [][]int{{robotoB, robotoA, 24}, {0xFFFF, 0xFFFF, 0}}, robotoX, robotoY),
	} {
		morx := morxAuditTable(morxAuditSubtable{4, lookup}, ab)
		testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "a", "ab", "ba")
	}

	deleteA := morxAuditSubtable{4, lookup8(robotoA, 0xFFFF)}
	afterDeleted := morxContextual(u16s(8, 0xFFFE, 2, 1, 1), u16s(8, robotoB, 1, robotoD))
	for _, lookup := range [][]byte{
		binSearchLookup(2, 6, [][]int{{robotoC, robotoC, robotoX}, {0xFFFF, 0xFFFF, robotoC}}),
		binSearchLookup(6, 4, [][]int{{robotoC, robotoX}, {0xFFFF, robotoC}}),
		binSearchLookup(6, 6, [][]int{{robotoC, robotoX}, {0xFFFF, robotoC}}),
	} {
		morx := morxAuditTable(deleteA, morxAuditSubtable{4, lookup}, afterDeleted)
		testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "ab", "ba", "abab", "b", "abc")
	}

	morx := morxAuditTable(morxAuditSubtable{4, binSearchLookup(4, 6, [][]int{{robotoC, robotoC, 0}})})
	testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "c", "ac")
}

// lookup10 builds a lookup of format 10 whose values have size bytes, for
// the glyphs from first on.
func lookup10(size, first int, values ...uint32) []byte {
	out := u16s(10, size, first, len(values))
	for _, v := range values {
		out = append(out, u32s(v)[4-size:]...)
	}
	return out
}

// kerxFormat6 builds a subtable of format 6 of kerx with values of two
// bytes, the lookups of the rows and the columns, and the kerning array.
func kerxFormat6(rows, columns []byte, array ...int) kerxSubtable {
	data := u32s(0)
	data = append(data, u16s(2, 2)...)
	data = append(data, u32s(36, uint32(36+len(rows)), uint32(36+len(rows)+len(columns)), 0)...)
	data = append(data, rows...)
	data = append(data, columns...)
	data = append(data, u16s(array...)...)
	return kerxSubtable{coverage: 6, data: data}
}

// TestLookupFormat10: lookups of format 10. HarfBuzz reads a value from
// such a lookup only for the rows and the columns of kerx format 6, with
// the size that the lookup gives, and keeps the low 16 bits of a value.
// For a class table, a substitution of morx or the classes of kerx format
// 2, it finds no value. It still collects the glyphs of the lookup, with
// their values for the class table of a state machine:
//   - The class table of a contextual subtable gives b the class 1, and c
//     the class 3. HarfBuzz gives both the class 1, which replaces c with
//     d.
//   - A noncontextual subtable replaces a with b.
//   - A subtable of format 2 of kerx, whose left classes give 6 to b, then
//     a subtable that kerns b, c.
//   - A subtable of format 6 of kerx, whose rows give 0 to a and 2 to b,
//     and whose columns give 0 to a and 1 to b, with values of 1 to 4
//     bytes. The kerning array is 0, -100, -50, -20.
func TestLookupFormat10(t *testing.T) {
	states := u16s(0, 2, 0, 0, 0, 2, 0, 0, 0, 2, 0, 0)
	morx := morxAuditTable(morxContextualStates(lookup10(1, robotoB, 1, 3), states, u16s(8, robotoC, 1, robotoD)))
	testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "bc", "cb", "c", "b")

	morx = morxAuditTable(morxAuditSubtable{4, lookup10(2, robotoA, robotoB)})
	testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "a", "ab")

	left := lookup10(2, robotoB, 6)
	kerx := kerxTable(kerxFormat2Lookups(left, u16s(8, robotoA, 3, 0, 2, 4)), kerxPairs(robotoB, robotoC, -300))
	testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, "bb", "bc", "ba")

	for _, size := range []int{1, 2, 3, 4} {
		rows := lookup10(size, robotoA, 0, 0x10002)
		columns := lookup10(size, robotoA, 0x30000, 1)
		kerx := kerxTable(kerxFormat6(rows, columns, 0, -100, -50, -20))
		testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, "ab", "ba", "bb", "aa")
	}
}

// kernFormat2 builds a table kern of Apple with a subtable of format 2 for
// a, b and c. The rows of the left classes are 36, 42 and 48 bytes from the
// start of the subtable, where the kerning array starts at 36, and the
// right classes are 0, 3 and 5 bytes. The array has three rows: 0, 0, 0,
// then 0, -80, 30, then 0, -50, -20.
func kernFormat2() []byte {
	data := u16s(6, 16, 26, 36)
	data = append(data, u16s(robotoA, 3, 36, 42, 48)...)
	data = append(data, u16s(robotoA, 3, 0, 3, 5)...)
	data = append(data, u16s(0, 0, 0, 0, -80, 30, 0, -50, -20)...)
	kern := u32s(0x00010000, 1, uint32(8+len(data)), 0x00020000)
	return append(kern, data...)
}

// TestKernFormat2: the subtable of kernFormat2. HarfBuzz reads the sum of
// the classes as a byte offset from the start of the subtable, and rounds
// its distance from the kerning array down to a value of two bytes.
func TestKernFormat2(t *testing.T) {
	testCase(t, robotoWith(t, map[string][]byte{"kern": kernFormat2()}), false, "ab", "ac", "bb", "bc", "cb", "cc", "ba", "abc")
}
