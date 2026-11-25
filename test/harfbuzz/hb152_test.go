package harfbuzz_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/harfbuzz"
)

// The tests of D97 compare harfbuzz with HarfBuzz 15.2.0 on the changes of
// HarfBuzz from 15.1.0 to 15.2.0 that D97 ports. Each test builds its font
// by hand from Roboto-Regular.ttf, or from a font of perf_reference, as the
// tests of D73 do.

// feature is a request of the feature tag with the value over [start, end).
func feature(tag string, value uint32, start, end int) harfbuzz.Feature {
	return harfbuzz.Feature{Tag: ot.MustNewTag(tag), Value: value, Start: start, End: end}
}

// global is a request of the feature tag with the value over the whole text.
func global(tag string, value uint32) harfbuzz.Feature {
	return feature(tag, value, harfbuzz.FeatureGlobalStart, harfbuzz.FeatureGlobalEnd)
}

// glyphOf gives the glyph of the character r in the font data.
func glyphOf(t *testing.T, data []byte, r rune) int {
	t.Helper()
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	g, ok := face.NominalGlyph(r)
	if !ok {
		t.Fatalf("the font has no glyph for %U", r)
	}
	return int(g)
}

// perfFont reads a font of perf_reference, and skips the test when the
// folder typesetting/testdata is missing.
func perfFont(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(perfReference, "fonts", name))
	if err != nil {
		t.Skip("the folder typesetting/testdata is missing")
	}
	return data
}

// singleSubst builds a SingleSubst of format 1 that adds delta to each
// glyph of the coverage.
func singleSubst(delta int, glyphs ...int) []byte {
	out := u16s(1, 6, delta)
	return append(out, coverage1(glyphs...)...)
}

// TestGlobalFeatureOverridesRange: the feature salt replaces a with b. A
// request of salt on a range, then a global request with another value,
// for the same tag. In HarfBuzz 15.2.0, the later global request wins over
// the ranges.
func TestGlobalFeatureOverridesRange(t *testing.T) {
	gsub := layoutTable(t, []string{"DFLT", "latn"}, []layoutFeature{{"salt", []int{0}}},
		[][]byte{lookupTable(t, 1, 0, singleSubst(robotoB-robotoA, robotoA))})
	data := robotoWith(t, map[string][]byte{"GSUB": gsub})
	for _, features := range [][]harfbuzz.Feature{
		{feature("salt", 1, 1, 2), global("salt", 0)},
		{feature("salt", 0, 1, 2), global("salt", 1)},
		{global("salt", 1), feature("salt", 0, 1, 2)},
		{feature("salt", 1, 0, 1), feature("salt", 1, 2, 3)},
	} {
		compareOptions(t, data, shapeOptions{features: features}, "aaa", "aaaa")
	}
}

// TestAATLaterFeatureRequest: the table morx of morxBudget replaces a with
// b when the common ligatures are on. In HarfBuzz 15.2.0, the later of two
// requests for liga over the same glyphs wins, and the end of a request
// removes that request.
func TestAATLaterFeatureRequest(t *testing.T) {
	data := robotoWith(t, map[string][]byte{"morx": morxBudget(0, 0), "feat": featLigatures()})
	for _, features := range [][]harfbuzz.Feature{
		{feature("liga", 0, 0, 3), feature("liga", 1, 1, 2)},
		{feature("liga", 1, 0, 3), feature("liga", 0, 1, 2)},
		{feature("liga", 0, 0, 2), feature("liga", 0, 1, 4), feature("liga", 1, 3, 4)},
		{feature("liga", 0, 1, 3), feature("liga", 0, 0, 2)},
	} {
		compareOptions(t, data, shapeOptions{features: features}, "aaaa")
	}
}

// kernPairsTable builds a table kern of Microsoft with one subtable of format 0
// with the coverage and the pairs, each a left glyph, a right glyph and a
// value. The pairs must be sorted.
func kernPairsTable(coverage int, pairs ...int) []byte {
	n := len(pairs) / 3
	out := u16s(0, 1, 0, 14+6*n, coverage, n, 6*n, 0, 0)
	return append(out, u16s(pairs...)...)
}

// TestKernCrossStream: a table kern with a cross-stream subtable. b after a
// moves up by 200, and c after b takes the value -0x8000, which resets the
// offset across the line. In HarfBuzz 15.2.0, the reset does not move c,
// and the glyphs after the first cross-stream kern are unsafe to break.
func TestKernCrossStream(t *testing.T) {
	data := robotoWith(t, map[string][]byte{"kern": kernPairsTable(0x05, robotoA, robotoB, 200, robotoB, robotoC, -0x8000)})
	compareOptions(t, data, shapeOptions{flags: true}, "ab", "abc", "bc", "xabcx", "abab")
}

// TestKernVertical: a table kern with a vertical subtable that kerns a, b.
// In HarfBuzz 15.2.0, the table applies in vertical text when the caller
// asks for vkrn and GPOS does not have it.
func TestKernVertical(t *testing.T) {
	data := robotoWith(t, map[string][]byte{"kern": kernPairsTable(0x00, robotoA, robotoB, -300)})
	compareOptions(t, data, shapeOptions{direction: "ttb"}, "ab", "aab")
	compareOptions(t, data, shapeOptions{direction: "ttb", features: []harfbuzz.Feature{global("vkrn", 1)}}, "ab", "aab")
}

// TestKerxWithGPOS: a font with GSUB, with a GPOS that has no feature of
// kerning, and with a table kerx of a subtable of pairs, a cross-stream
// subtable of pairs and a subtable of format 1. In HarfBuzz 15.2.0, GPOS
// applies, and then kerx applies its subtable of pairs that is not
// cross-stream.
func TestKerxWithGPOS(t *testing.T) {
	gsub := layoutTable(t, []string{"DFLT", "latn"}, []layoutFeature{{"salt", []int{0}}},
		[][]byte{lookupTable(t, 1, 0, singleSubst(1, robotoZ))})
	single := u16s(1, 8, 0x0004, 50)
	single = append(single, coverage1(robotoC)...)
	gpos := layoutTable(t, []string{"DFLT", "latn"}, []layoutFeature{{"mark", []int{0}}},
		[][]byte{lookupTable(t, 1, 0, single)})
	crossStream := kerxPairs(robotoB, robotoC, 100)
	crossStream.coverage = 0x40000000
	format1 := kerxFormat1([4][2]int{{0, kerxEnd}, {kerxPush, kerxEnd}, {kerxPush, 0}, {0, kerxEnd}}, -40, -30, kerxEnd)
	kerx := kerxTable(kerxPairs(robotoA, robotoB, -300), crossStream, format1)
	data := robotoWith(t, map[string][]byte{"GSUB": gsub, "GPOS": gpos, "kerx": kerx})
	compareOptions(t, data, shapeOptions{flags: true}, "ab", "abc", "aab", "bcc")
}

// TestMarkBaseNotMatching: the feature mark attaches the mark c to the base
// a or b, and is off on b. In HarfBuzz 15.2.0, the search for the base of c
// stops at b, so c does not attach to a.
func TestMarkBaseNotMatching(t *testing.T) {
	gdef := u16s(1, 0, 12, 0, 0, 0)
	gdef = append(gdef, u16s(2, 1, robotoC, robotoC, 3)...)
	markBase := u16s(1, 12, 18, 1, 26, 38)
	markBase = append(markBase, coverage1(robotoC)...)
	markBase = append(markBase, coverage1(robotoA, robotoB)...)
	markBase = append(markBase, u16s(1, 0, 6)...)
	markBase = append(markBase, anchor1(0, 0)...)
	markBase = append(markBase, u16s(2, 6, 12)...)
	markBase = append(markBase, anchor1(200, 400)...)
	markBase = append(markBase, anchor1(300, 500)...)
	gpos := layoutTable(t, []string{"DFLT", "latn"}, []layoutFeature{{"mark", []int{0}}},
		[][]byte{lookupTable(t, 4, 0, markBase)})
	data := robotoWith(t, map[string][]byte{"GDEF": gdef, "GPOS": gpos})
	compareOptions(t, data, shapeOptions{features: []harfbuzz.Feature{feature("mark", 0, 1, 2)}}, "abc", "acbc", "abcc")
	compareOptions(t, data, shapeOptions{}, "abc", "acbc")
}

// TestNotFoundVariationSelector: a character that Roboto does not have,
// followed by a variation selector. In HarfBuzz 15.2.0, it takes the glyph
// not-found of the buffer.
func TestNotFoundVariationSelector(t *testing.T) {
	data := robotoWith(t, nil)
	compareOptions(t, data, shapeOptions{notFound: 3}, "\u4e00\ufe00", "a\u4e00\ufe00b", "\u4e00", "a\ufe00")
}

// TestFallbackMarkOffset: a table kern moves b after a by an offset, and a
// combining acute follows b. Without GPOS, HarfBuzz 15.2.0 places the mark
// from the offset of its base.
func TestFallbackMarkOffset(t *testing.T) {
	data := robotoWith(t, map[string][]byte{"kern": kernPairsTable(0x01, robotoA, robotoB, -300)})
	compareOptions(t, data, shapeOptions{}, "ab\u0301", "ab\u0323", "ab\u0301\u0301", "b\u0301")
}

// TestIndicInitContext: NotoSansDevanagari-Regular.ttf with a table GSUB
// whose feature init replaces the matra i with ka. In HarfBuzz 15.2.0, init
// does not apply when the context before the buffer ends with a letter.
func TestIndicInitContext(t *testing.T) {
	data := perfFont(t, "NotoSansDevanagari-Regular.ttf")
	matra := glyphOf(t, data, 0x093F)
	ka := glyphOf(t, data, 0x0915)
	gsub := layoutTable(t, []string{"DFLT", "dev2"}, []layoutFeature{{"init", []int{0}}},
		[][]byte{lookupTable(t, 1, 0, singleSubst(ka-matra, matra))})
	data = replaceLayout(t, data, map[string][]byte{"GSUB": gsub})
	compareOptions(t, data, shapeOptions{}, "क\u093f", "क क\u093f")
	compareOptions(t, data, shapeOptions{itemOffset: 1}, "कक\u093f", " क\u093f", "\u0902क\u093f")
}

// TestUSEWordEnds: Roboto with a table GSUB whose features isol, init, medi
// and fina for Balinese each replace the glyph 0 with another glyph. In
// HarfBuzz 15.2.0, a space, a ZWNJ and a generic base end a word of the
// USE shaper, and a space is not the base of a vowel.
func TestUSEWordEnds(t *testing.T) {
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
		"ᬓᬓᬓ", "ᬓᬓ ᬓᬓ", "ᬓ\u200cᬓᬓ", "ᬓ\u00a0ᬓ", "ᬓ \u1b36", " \u1b36", "ᬓ\u1b5aᬓ")
}

// TestKhmerThirdCoeng: Roboto with a table GSUB whose feature pref for
// Khmer replaces the glyph 0 with a. In HarfBuzz 15.2.0, only the first or
// the second coeng of a syllable moves with ro before the base and takes
// pref.
func TestKhmerThirdCoeng(t *testing.T) {
	gsub := layoutTable(t, []string{"DFLT", "khmr"}, []layoutFeature{{"pref", []int{0}}},
		[][]byte{lookupTable(t, 1, 0, singleSubst(robotoA, 0))})
	data := robotoWith(t, map[string][]byte{"GSUB": gsub})
	compareOptions(t, data, shapeOptions{}, "ក\u17d2រ", "ក\u17d2ក\u17d2រ", "ក\u17d2ក\u17d2ក\u17d2រ")
}

// TestHangulToneMarks: Roboto, which has the dotted circle and no GDEF. In
// HarfBuzz 15.2.0, a tone mark after a dotted circle takes it as its base,
// and the inserted dotted circle keeps its advance.
func TestHangulToneMarks(t *testing.T) {
	data := robotoWith(t, nil)
	compareOptions(t, data, shapeOptions{flags: true}, "\u302e", "\u25cc\u302e", "\u302e\u302f", "a\u302e")
}

// TestGraphemeMergeFlags: Roboto and Thai text with sara am, which the
// Thai shaper decomposes, and merges as graphemes. In HarfBuzz 15.2.0, the
// glyphs are unsafe to break at the levels of characters, where the
// clusters do not merge.
func TestGraphemeMergeFlags(t *testing.T) {
	data := robotoWith(t, nil)
	for _, level := range []harfbuzz.ClusterLevel{harfbuzz.MonotoneGraphemes, harfbuzz.MonotoneCharacters, harfbuzz.Characters} {
		compareOptions(t, data, shapeOptions{flags: true, clusterLevel: level}, "\u0e01\u0e33", "\u0e01\u0e48\u0e33", "\u0e01\u0e33\u0e01")
	}
}

// TestVowelConstraintIgnorable: NotoSansDevanagari-Regular.ttf. In HarfBuzz
// 15.2.0, a ZWJ or a ZWNJ between the vowels of a prohibited sequence does
// not stop the dotted circle, which keeps its advance, and the glyphs of
// the sequence are unsafe to break.
func TestVowelConstraintIgnorable(t *testing.T) {
	data := perfFont(t, "NotoSansDevanagari-Regular.ttf")
	compareOptions(t, data, shapeOptions{flags: true}, "अ\u093e", "अ\u200d\u093e", "अ\u200c\u093e", "र\u200d\u094dइ", "क अ\u200d\u093e")
}

// TestPerSyllableLigature: NotoSansDevanagari-Regular.ttf with a table GSUB
// whose feature pres makes kha of ka, ka. Word joiners between the two ka
// are syllables of their own, and the ligature skips them. In HarfBuzz
// 15.2.0, a glyph of another syllable ends the match, also when the serial
// of its syllable wraps to the serial of the first ka.
func TestPerSyllableLigature(t *testing.T) {
	data := perfFont(t, "NotoSansDevanagari-Regular.ttf")
	ka := glyphOf(t, data, 0x0915)
	kha := glyphOf(t, data, 0x0916)
	ligature := u16s(1, 8, 1, 14)
	ligature = append(ligature, coverage1(ka)...)
	ligature = append(ligature, u16s(1, 4, kha, 2, ka)...)
	gsub := layoutTable(t, []string{"DFLT", "dev2"}, []layoutFeature{{"pres", []int{0}}},
		[][]byte{lookupTable(t, 4, 0, ligature)})
	data = replaceLayout(t, data, map[string][]byte{"GSUB": gsub})
	var texts []string
	for n := range 18 {
		texts = append(texts, "क"+strings.Repeat("\u2060", n)+"क")
	}
	compareOptions(t, data, shapeOptions{}, texts...)
}

// TestSyllableCut: NotoSansDevanagari-Regular.ttf with syllables of more
// than 64 glyphs. In HarfBuzz 15.2.0, the cut of such a syllable stays the
// same before and after the substitutions, and its pieces are one cluster.
func TestSyllableCut(t *testing.T) {
	data := perfFont(t, "NotoSansDevanagari-Regular.ttf")
	compareOptions(t, data, shapeOptions{flags: true},
		"क"+strings.Repeat("\u0902", 70), "क\u093f"+strings.Repeat("\u093c", 140)+"क", strings.Repeat("क\u094d", 40)+"क")
}

// trakTable builds a table trak with one track 0 for horizontal text and
// for vertical text, with the value at the size 12.
func trakTable(horizontal, vertical int) []byte {
	data := func(value int) []byte {
		// nTracks, nSizes, the offset of the sizes, then the track entry
		// with its offset to the values, the size, and the value.
		out := u16s(1, 1)
		out = append(out, u32s(0)...)
		out = append(out, u32s(0)...)
		out = append(out, u16s(0, 0)...)
		out = append(out, u32s(12<<16)...)
		return append(out, u16s(value)...)
	}
	const header = 12
	h := data(horizontal)
	out := u32s(0x00010000)
	out = append(out, u16s(0, header, header+len(h), 0)...)
	h = fixTrak(h, header)
	v := fixTrak(data(vertical), header+len(h))
	out = append(out, h...)
	return append(out, v...)
}

// fixTrak writes the offsets of the sizes and of the values of the track
// data that starts at the offset start of the table.
func fixTrak(data []byte, start int) []byte {
	sizes := start + 16
	values := sizes + 4
	copy(data[4:8], u32s(uint32(sizes)))
	copy(data[14:16], u16s(values))
	return data
}

// TestTrakVertical: a table trak that adds 100 units to each glyph in
// horizontal text and in vertical text, with a table STAT. In HarfBuzz
// 15.2.0, a positive tracking makes the spacing of vertical text larger.
func TestTrakVertical(t *testing.T) {
	stat := u16s(1, 1, 8, 0)
	stat = append(stat, u32s(0)...)
	stat = append(stat, u16s(0)...)
	stat = append(stat, u32s(0)...)
	stat = append(stat, u16s(0)...)
	data := robotoWith(t, map[string][]byte{"trak": trakTable(100, 100), "STAT": stat})
	compareOptions(t, data, shapeOptions{direction: "ttb"}, "ab", "abc")
	compareOptions(t, data, shapeOptions{}, "ab", "abc")
}

// TestRecomposeWithoutMarks: Roboto and texts with no marks of the Khmer
// script and of Devanagari, where e with acute decomposes to e and a mark.
// In HarfBuzz 15.2.0, the normalizer composes e with acute again, because
// the decomposition added marks to the text.
func TestRecomposeWithoutMarks(t *testing.T) {
	data := robotoWith(t, nil)
	compareOptions(t, data, shapeOptions{}, "\u1780\u00e9", "\u1780\u00e9a", "\u0915\u00e9", "\u1780\u00e9\u0301")
}
