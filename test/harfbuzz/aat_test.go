package harfbuzz_test

import (
	"bytes"
	"encoding/binary"
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
// empty, the feature off is off for the whole text. When direction is not
// empty, it is the direction of the buffer, such as ttb. features are more
// requests of features, in their order, and flags compares the flag
// unsafe-to-break of each glyph. When notFound is not 0, the buffer gives
// it to a character that the font does not have. clusterLevel is the
// cluster level of the buffer. The first itemOffset characters of a text
// are the context before the text.
type shapeOptions struct {
	language  string
	ranged    string
	period    int
	off       string
	direction string
	features  []harfbuzz.Feature
	flags     bool

	notFound     harfbuzz.GID
	clusterLevel harfbuzz.ClusterLevel
	itemOffset   int
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
	if opts.direction != "" {
		args = append(args, "--direction", opts.direction)
	}
	for _, f := range opts.features {
		end := strconv.Itoa(f.End)
		if f.End == harfbuzz.FeatureGlobalEnd {
			end = "-1"
		}
		args = append(args, "--feature", f.Tag.String(), strconv.Itoa(f.Start), end, strconv.Itoa(int(f.Value)))
	}
	if opts.notFound != 0 {
		args = append(args, "--not-found", strconv.Itoa(int(opts.notFound)))
	}
	if opts.clusterLevel != 0 {
		args = append(args, "--cluster-level", strconv.Itoa(int(opts.clusterLevel)))
	}
	if opts.itemOffset != 0 {
		args = append(args, "--item-offset", strconv.Itoa(opts.itemOffset))
	}
	want := shapeHarfBuzz(t, path, texts, opts.flags, args...)
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	hf := harfbuzz.NewFont(face)
	for i, text := range texts {
		runes := []rune(text)
		buf := harfbuzz.NewBuffer()
		buf.AddRunes(runes, opts.itemOffset, -1)
		buf.NotFound = opts.notFound
		buf.ClusterLevel = opts.clusterLevel
		if opts.language != "" {
			buf.Props.Language = language.NewLanguage(opts.language)
		}
		buf.GuessSegmentProperties()
		if opts.direction != "" {
			buf.Props.Direction = directions[opts.direction]
		}
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
		features = append(features, opts.features...)
		buf.Shape(hf, features)
		var got []string
		for j, info := range buf.Info {
			pos := buf.Pos[j]
			glyph := fmt.Sprintf("%d %d %d %d %d %d", info.Glyph, info.Cluster, pos.XAdvance, pos.YAdvance, pos.XOffset, pos.YOffset)
			if opts.flags {
				unsafe := 0
				if info.Mask&harfbuzz.GlyphUnsafeToBreak != 0 {
					unsafe = 1
				}
				glyph += fmt.Sprintf(" %d", unsafe)
			}
			got = append(got, glyph)
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

// directions maps the directions of shape.py to those of harfbuzz.
var directions = map[string]harfbuzz.Direction{
	"ltr": harfbuzz.LeftToRight,
	"rtl": harfbuzz.RightToLeft,
	"ttb": harfbuzz.TopToBottom,
	"btt": harfbuzz.BottomToTop,
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
// A second table has a subtable of format 2 whose kerning array starts
// after its end, then a subtable that kerns a, b. HarfBuzz refuses that
// table.
func TestKernFormat2(t *testing.T) {
	testCase(t, robotoWith(t, map[string][]byte{"kern": kernFormat2()}), false, "ab", "ac", "bb", "bc", "cb", "cc", "ba", "abc")
	testCase(t, robotoWith(t, map[string][]byte{"kern": kernOT(kernLateArray(), kernPairs(20, robotoA, robotoB, -300))}), false, "ab")
}

// kernLateArray builds a subtable of format 2 of a table kern of Microsoft
// whose kerning array starts after its end.
func kernLateArray() []byte {
	data := u16s(0, 6+8+6, 2<<8|1, 6, 14, 14, 100)
	return append(data, u16s(robotoA, 1, 0)...)
}

// TestKerxTupleCount: subtables of format 0 and 2 of kerx with a tuple
// count. HarfBuzz reads each value as a byte offset from the start of the
// subtable to the values of the tuples, and gives 0 when the tuples are not
// in the subtable, or not in the table for the last subtable. In format 2,
// it refuses the table when the kerning array starts after the end of the
// subtable:
//   - A subtable of format 0 kerns a, b with the offset 52, a, c with the
//     offset 54, b, c with the offset -2 and c, c with the offset 56. Its
//     values of the tuples follow its pairs: -100, -60, then -40 for the
//     last subtable, which ends there.
//   - A subtable of format 2, whose kerning array gives the offsets 58 and
//     60, to the values -80 and -50 after the array.
//   - A subtable of format 2 whose kerning array starts after its end,
//     then a subtable that kerns a, b.
func TestKerxTupleCount(t *testing.T) {
	pairs := kerxPairs(robotoA, robotoB, 52, robotoA, robotoC, 54, robotoB, robotoC, -2, robotoC, robotoC, 56)
	pairs.data = append(pairs.data, u16s(-100, -60)...)
	for _, count := range []int{1, 2} {
		first := pairs
		first.tuples = count
		last := first
		last.data = append(slices.Clone(last.data), u16s(-40)...)
		for _, kerx := range [][]byte{kerxTable(first), kerxTable(first, kerxPairs()), kerxTable(last)} {
			testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, "ab", "ac", "bc", "cc", "abc")
		}
	}

	left := u16s(8, robotoA, 3, 0, 6, 6)
	right := u16s(8, robotoA, 3, 0, 2, 4)
	array := u16s(0, 58, 60)
	data := u32s(6, 28, uint32(28+len(left)), uint32(28+len(left)+len(right)))
	data = append(data, left...)
	data = append(data, right...)
	data = append(data, array...)
	data = append(data, u16s(-80, -50)...)
	format2 := kerxSubtable{coverage: 2, data: data, tuples: 1}
	testCase(t, robotoWith(t, map[string][]byte{"kerx": kerxTable(format2)}), false, "ab", "ac", "bb", "bc", "abc")

	late := kerxSubtable{coverage: 2, data: u32s(6, 28, 28, 100)}
	late.data = append(late.data, u16s(8, robotoA, 1, 0)...)
	for _, kerx := range [][]byte{
		kerxTable(late, kerxPairs(robotoA, robotoB, -300)),
		kerxTable(kerxPairs(robotoA, robotoB, -300), late),
	} {
		testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, "ab")
	}
}

// TestLtagRange: the table of TestLtagLanguage, with a table ltag whose
// third tag ends after the end of the table, or whose version is 0.
// HarfBuzz refuses the whole table ltag, so that no tag matches a
// language.
func TestLtagRange(t *testing.T) {
	long := ltagTable("sr", "Zh_Hant", "en@x")
	copy(long[12+2*4+2:], u16s(100))
	version0 := ltagTable("sr", "Zh_Hant", "en@x")
	version0[3] = 0
	for _, ltag := range [][]byte{long, version0} {
		data := robotoWith(t, map[string][]byte{"morx": morxLanguages(), "ltag": ltag})
		for _, lang := range []string{"sr", "zh-hant", "en"} {
			compareOptions(t, data, shapeOptions{language: lang}, "a")
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
		entries:  [][]int{{0, 0, kerx4NoActionData}, {0, kerxMark, kerx4NoActionData}, {0, kerxMark, 1}, {0, 0, 0}},
	}
	return kerxSubtable{coverage: kerxFormat4, data: marks.bytes([]uint32{actionType << 30}, actions)}
}

// TestKerxFormat4Actions: subtables of kerxFormat4Actions. HarfBuzz reads
// an action when it applies the subtable. When the action is not in the
// subtable, or not in the table for the last subtable, it stops the
// transition, which does not take the flag Mark. An action of the type 3
// has no data, and the glyph is still attached:
//   - Subtables of the action types 0, 1 and 2 with only the action 0.
//   - A subtable of the action type 3.
//   - A subtable of the action type 2 whose actions start at its end, then
//     a subtable that kerns a, c.
func TestKerxFormat4Actions(t *testing.T) {
	texts := []string{"ac", "abc", "bc", "abac", "acb", "cc"}
	for _, st := range []kerxSubtable{
		kerxFormat4Actions(0, u16s(0, 1)),
		kerxFormat4Actions(1, u16s(0, 1)),
		kerxFormat4Actions(2, u16s(50, 60, 10, 20)),
		kerxFormat4Actions(3, nil),
	} {
		testCase(t, robotoWith(t, map[string][]byte{"kerx": kerxTable(st)}), false, texts...)
	}
	kerx := kerxTable(kerxFormat4Actions(2, nil), kerxPairs(robotoA, robotoC, -300))
	testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, texts...)
}

// TestKerxFormat6Values: subtables of format 6 of kerx, whose rows give 0
// to a and 2 to b, and whose columns give 0 to a and 1 to b. HarfBuzz reads
// the value at the sum of a row and a column when it applies the subtable,
// and gives 0 when the value is not in the subtable, or not in the table
// for the last subtable. It ignores the counts of the rows and the
// columns. Long values have 32 bits, and so do the values of their
// lookups. With a tuple count, a value is a byte offset from the vector of
// the tuples. HarfBuzz refuses the table when the kerning array or the
// vector starts after the end of the subtable:
//   - A kerning array of three values 0, -100, -50, without the value of
//     b, b, with counts of 1 and 2, in a subtable before a subtable that
//     kerns x, y, and as the last subtable.
//   - Long values 0, -100, -50, -20 with lookups of format 8, 2, 4 and 6,
//     with counts of 2.
//   - Long values that are the offsets 0, 2, 4 and 6 to the tuples -10,
//     -20, -30 and -40, with a tuple count of 1, and the same with offsets
//     after the tuples.
//   - A subtable whose kerning array starts after its end, and a subtable
//     with a tuple count whose vector starts after its end, then a subtable
//     that kerns a, b.
func TestKerxFormat6Values(t *testing.T) {
	texts := []string{"ab", "ba", "bb", "aa", "abba"}
	rows, columns := lookup8(robotoA, 0, 2), lookup8(robotoA, 0, 1)
	for _, count := range []int{1, 2} {
		short := kerxFormat6Data(0, count, rows, columns, u16s(0, -100, -50), nil)
		for _, kerx := range [][]byte{kerxTable(short, kerxPairs(robotoX, robotoY, -300)), kerxTable(short)} {
			testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, texts...)
		}
	}

	long := u32s(0, 0xFFFFFF9C, 0xFFFFFFCE, 0xFFFFFFEC)
	for _, lookups := range [][2][]byte{
		{append(u16s(8, robotoA, 2), u32s(0, 2)...), append(u16s(8, robotoA, 2), u32s(0, 1)...)},
		{binSearchLookup(2, 8, [][]int{{robotoA, robotoA, 0, 0}, {robotoB, robotoB, 0, 2}}),
			binSearchLookup(2, 8, [][]int{{robotoB, robotoA, 0, 0}}, 0)},
		{binSearchLookup(4, 6, [][]int{{robotoB, robotoA, 18}}, 0, 0, 0, 2), binSearchLookup(4, 6, [][]int{{robotoB, robotoA, 18}}, 0, 0, 0, 1)},
		{binSearchLookup(6, 6, [][]int{{robotoA, 0, 0}, {robotoB, 0, 2}}), binSearchLookup(6, 6, [][]int{{robotoA, 0, 0}, {robotoB, 0, 1}})},
	} {
		kerx := kerxTable(kerxFormat6Data(1, 2, lookups[0], lookups[1], long, nil))
		testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, texts...)
	}

	long8 := [2][]byte{append(u16s(8, robotoA, 2), u32s(0, 2)...), append(u16s(8, robotoA, 2), u32s(0, 1)...)}
	for _, offsets := range [][]byte{u32s(0, 2, 4, 6), u32s(2, 4, 6, 8)} {
		st := kerxFormat6Data(1, 2, long8[0], long8[1], offsets, u16s(-10, -20, -30, -40))
		st.tuples = 1
		testCase(t, robotoWith(t, map[string][]byte{"kerx": kerxTable(st)}), false, texts...)
	}

	late := kerxFormat6Data(0, 2, rows, columns, nil, nil)
	copy(late.data[16:], u32s(1000))
	vector := kerxFormat6Data(0, 2, rows, columns, u16s(0, -100, -50, -20), nil)
	copy(vector.data[20:], u32s(1000))
	vector.tuples = 1
	for _, st := range []kerxSubtable{late, vector} {
		testCase(t, robotoWith(t, map[string][]byte{"kerx": kerxTable(st, kerxPairs(robotoA, robotoB, -300))}), false, "ab")
	}
}

// TestMorxLazyData: subtables of morx whose data HarfBuzz reads when it
// applies the subtable, then a noncontextual subtable that replaces c with
// x. HarfBuzz skips an action, an insertion or a component that is not in
// the table. It refuses the table when the offset of the actions, of the
// components, of the ligatures or of the insertions is 0. The offset 0 of
// the substitutions of a contextual subtable, and of one substitution, is
// the start of the table that holds it:
//   - The ligature subtable of morxLigature, whose actions start after the
//     end of the table, or whose components follow its ligatures, or whose
//     offset of the actions, of the components or of the ligatures is 0.
//   - An insertion subtable that inserts b after a, with an insertion
//     after the end of the table, or with the offset 0.
//   - A contextual subtable whose substitutions are at the offset 0, or
//     whose one substitution is at the offset 0.
func TestMorxLazyData(t *testing.T) {
	cx := morxAuditSubtable{4, lookup8(robotoC, robotoX)}
	texts := []string{"abc", "ab", "c", "aab"}
	ligature := morxLigature()
	field := func(st morxAuditSubtable, at int, v uint32) morxAuditSubtable {
		st.data = slices.Clone(st.data)
		copy(st.data[at:], u32s(v))
		return st
	}
	sorted := slices.Clone(ligature.data)
	n := len(sorted)
	sorted[n-4], sorted[n-3], sorted[n-2], sorted[n-1] = sorted[n-2], sorted[n-1], sorted[n-4], sorted[n-3]
	components := binary.BigEndian.Uint32(sorted[20:])
	unsorted := field(morxAuditSubtable{2, sorted}, 20, components+2)
	unsorted = field(unsorted, 24, components)
	for _, st := range []morxAuditSubtable{
		field(ligature, 16, 10000), unsorted,
		field(ligature, 16, 0), field(ligature, 20, 0), field(ligature, 24, 0),
	} {
		testCase(t, robotoWith(t, map[string][]byte{"morx": morxAuditTable(st, cx)}), false, texts...)
	}

	for _, index := range []int{0, 100} {
		insertion := stateMachine{
			lookup:   lookup8(robotoA, 4),
			nClasses: 5,
			states:   [][]int{{0, 0, 0, 0, 1}, {0, 0, 0, 0, 1}},
			entries:  [][]int{{0, 0, insertNone, insertNone}, {0, insertCurrentOne, index, insertNone}},
		}
		data := insertion.bytes([]uint32{0}, u16s(robotoB))
		for _, st := range []morxAuditSubtable{{5, data}, field(morxAuditSubtable{5, data}, 16, 0)} {
			testCase(t, robotoWith(t, map[string][]byte{"morx": morxAuditTable(st, cx)}), false, texts...)
		}
	}

	contextual := morxContextualStates(u16s(8, robotoA, 1, 4), u16s(0, 0, 0, 0, 2, 0, 0, 0, 0, 2, 0, 0, 0, 0, 2), u16s(8, robotoA, 1, robotoD))
	contextual.data = slices.Clone(contextual.data)
	copy(contextual.data[0:], u32s(5))
	for _, st := range []morxAuditSubtable{
		contextual,
		field(contextual, 16, 0),
		field(contextual, int(binary.BigEndian.Uint32(contextual.data[16:])), 0),
	} {
		testCase(t, robotoWith(t, map[string][]byte{"morx": morxAuditTable(st, cx)}), false, texts...)
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
	data := u16s(glyphs)
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

// TestKernFormat3: the subtable of kernFormat3. HarfBuzz reads the classes
// and the kerning value when it applies the subtable. A glyph after the
// glyphs of the subtable has the class 0, a class too large gives no
// kerning, and so does an index that is not a kerning value. The subtable
// applies to each glyph of the font.
func TestKernFormat3(t *testing.T) {
	testCase(t, robotoWith(t, map[string][]byte{"kern": kernFormat3()}), false, "ab", "bb", "cb", "xb", "ba", "abc", "aa")
}

// TestAATClassOffsetZero: subtables whose class table is at the offset 0,
// which HarfBuzz reads at the start of the state table, or of the
// subtable for kerx format 2 and kern format 2, then a subtable that
// replaces c with x, or kerns a, b. Here HarfBuzz cannot read such a class
// table, and refuses the table:
//   - An insertion subtable of morx.
//   - A subtable of format 2 of kerx, with the offset 0 for the left or the
//     right classes.
//   - A subtable of format 2 of a table kern of Microsoft, with the offset
//     0 for the left classes.
func TestAATClassOffsetZero(t *testing.T) {
	insertion := stateMachine{
		lookup:   lookup8(robotoA, 4),
		nClasses: 5,
		states:   [][]int{{0, 0, 0, 0, 1}, {0, 0, 0, 0, 1}},
		entries:  [][]int{{0, 0, insertNone, insertNone}, {0, insertCurrentOne, 0, insertNone}},
	}
	st := morxAuditSubtable{5, insertion.bytes([]uint32{0}, u16s(robotoB))}
	st.data = slices.Clone(st.data)
	copy(st.data[4:], u32s(0))
	morx := morxAuditTable(st, morxAuditSubtable{4, lookup8(robotoC, robotoX)})
	testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "ac", "c")

	for _, at := range []int{4, 8} {
		format2 := kerxFormat2()
		format2.data = slices.Clone(format2.data)
		copy(format2.data[at:], u32s(0))
		kerx := kerxTable(format2, kerxPairs(robotoA, robotoB, -300))
		testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, "ab", "bc")
	}

	format2 := u16s(0, 6+8+6+2, 2<<8|1, 6, 0, 14, 20)
	format2 = append(format2, u16s(robotoA, 1, 0, 0)...)
	kern := kernOT(format2, kernPairs(20, robotoA, robotoB, -300))
	testCase(t, robotoWith(t, map[string][]byte{"kern": kern}), false, "ab")
}

// TestAATTableVersions: tables with a version that HarfBuzz refuses, or
// reads in another way. HarfBuzz refuses a table morx of version 0 and a
// table kerx of version 1, and then shapes the text without them. A table
// kern whose major version is 1 has a version and a count of subtables of
// 32 bits, also when the next 16 bits are not 0:
//   - A table morx of version 0 whose subtable replaces a with b.
//   - A table kerx of version 1 that kerns a, b.
//   - A table kern of version 1 with a count of 16 bits, 1, and a subtable
//     that kerns a, b. HarfBuzz reads the length of the subtable as the
//     count of subtables, and refuses the table.
func TestAATTableVersions(t *testing.T) {
	morx := morxAuditTable(morxAuditSubtable{4, lookup8(robotoA, robotoB)})
	morx[1] = 0
	testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "a", "ab")

	kerx := kerxTable(kerxPairs(robotoA, robotoB, -300))
	kerx[1] = 1
	testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, "ab")

	pairs := u16s(1, 6, 0, 0, robotoA, robotoB, -300)
	kern := u16s(1, 1)
	kern = append(kern, u32s(uint32(8+len(pairs)), 0)...)
	kern = append(kern, pairs...)
	testCase(t, robotoWith(t, map[string][]byte{"kern": kern}), false, "ab")
}

// ankrTable builds a table ankr with the version and the lookup, whose
// values are offsets in the glyph data. The glyph data holds the anchor
// (100, 200) at the offset 0 and the anchor (10, 20) at the offset 8.
func ankrTable(version int, lookup []byte) []byte {
	out := u16s(version, 0)
	out = append(out, u32s(12, uint32(12+len(lookup)))...)
	out = append(out, lookup...)
	out = append(out, u32s(1)...)
	out = append(out, u16s(100, 200)...)
	out = append(out, u32s(1)...)
	return append(out, u16s(10, 20)...)
}

// TestAnkr: a subtable of format 4 of kerx with anchor point actions,
// where c attaches to a with the anchor 0 of each glyph, and tables ankr.
// HarfBuzz refuses a table ankr whose version is not 0, whose lookup has
// the format 10, or where the anchors of a value of the lookup are not in
// the table. The anchors are then at (0, 0):
//   - A table ankr whose lookup gives the offset 0 to a and 8 to c.
//   - The same table with the version 1.
//   - The same table, where the lookup gives b the offset 1000.
//   - The same table, with a lookup of format 10.
func TestAnkr(t *testing.T) {
	kerx := kerxTable(kerxFormat4Actions(1, u16s(0, 0)))
	for _, ankr := range [][]byte{
		ankrTable(0, lookup8(robotoA, 0, 8, 8)),
		ankrTable(1, lookup8(robotoA, 0, 8, 8)),
		ankrTable(0, lookup8(robotoA, 0, 1000, 8)),
		ankrTable(0, lookup10(2, robotoA, 0, 8, 8)),
	} {
		testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx, "ankr": ankr}), false, "ac", "abc", "cc")
	}
}
