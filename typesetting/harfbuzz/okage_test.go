// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package harfbuzz

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	td "github.com/xo/okage/typesetting/internal/testfonts/harfbuzz"
)

// okage: the tests of D73.

// shapedGlyph is a glyph of the output of HarfBuzz 15.1.0, at the scale of
// the units of the font.
type shapedGlyph struct {
	glyph            GID
	cluster          int
	advance          Position
	offsetX, offsetY Position
}

// shapedText is a text and the glyphs that HarfBuzz 15.1.0 gives for it.
type shapedText struct {
	text string
	want []shapedGlyph
}

// testShapedGlyphs shapes each text in the font of the file, and compares
// the glyphs with the glyphs of HarfBuzz.
func testShapedGlyphs(t *testing.T, path string, texts []shapedText) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	face, err := font.ParseTTF(file)
	if err != nil {
		t.Fatal(err)
	}
	testShapedFace(t, face, texts)
}

// testShapedFace shapes each text in the face, and compares the glyphs with
// the glyphs of HarfBuzz.
func testShapedFace(t *testing.T, face *font.Face, texts []shapedText) {
	t.Helper()
	for _, test := range texts {
		buf := NewBuffer()
		buf.AddRunes([]rune(test.text), 0, -1)
		buf.GuessSegmentProperties()
		buf.Shape(NewFont(face), nil)
		if len(buf.Info) != len(test.want) {
			t.Errorf("%q: got %d glyphs, want %d", test.text, len(buf.Info), len(test.want))
			continue
		}
		for i, want := range test.want {
			info, pos := buf.Info[i], buf.Pos[i]
			got := shapedGlyph{info.Glyph, info.Cluster, pos.XAdvance, pos.XOffset, pos.YOffset}
			if got != want {
				t.Errorf("%q, glyph %d: got %+v, want %+v", test.text, i, got, want)
			}
		}
	}
}

// TestMarkBaseOfLookup shapes two short texts in Amiri. In each text, GSUB
// adds a tatweel after the lam. One lookup of GPOS attaches the mark to
// the tatweel, and a later lookup attaches it to the lam. go-text kept the
// base of the first lookup for the second, which then did not attach the
// mark.
func TestMarkBaseOfLookup(t *testing.T) {
	testShapedGlyphs(t, "../font/testdata/Amiri-Regular.ttf", []shapedText{
		{"لًكنننننن", []shapedGlyph{
			{3982, 8, 542, 0, 0}, {3964, 7, 218, 0, 0}, {2140, 6, 244, 0, 0}, {2140, 5, 244, 0, 0},
			{4511, 4, 244, 0, 0}, {2140, 3, 244, 0, 0}, {2307, 2, 588, 0, 0}, {427, 0, 0, -34, 0},
			{5992, 0, 185, 0, 0}, {2329, 0, 175, 0, 0},
		}},
		{"هلَكو", []shapedGlyph{
			{2448, 4, 345, 0, 0}, {2307, 3, 588, 0, 0}, {430, 1, 0, -23, 0}, {5992, 1, 185, 0, 0},
			{2335, 1, 197, 0, 0}, {2263, 0, 452, 0, 0},
		}},
	})
}

// TestPairPosClassCounts shapes two texts in Amiri-Regular.ttf of
// perf_reference. A PairPos of format 2 of lookup 74 of its GPOS has 80
// classes for the first glyph and 79 in ClassDef1, and go-text dropped the
// whole table GPOS, so the texts had no kerning.
func TestPairPosClassCounts(t *testing.T) {
	face := font.NewFace(openFontFile(t, "perf_reference/fonts/Amiri-Regular.ttf"))
	testShapedFace(t, face, []shapedText{
		{"ه کو", []shapedGlyph{{2440, 3, 708, 0, 0}, {2290, 2, 850, 0, 0}, {3, 1, 600, 0, 0}, {418, 0, 756, 0, 0}}},
		{"برگ", []shapedGlyph{{522, 2, 2463, 0, 0}, {2518, 1, 313, -300, 0}, {2484, 0, 362, 0, 0}}},
	})
}

// TestMarkWithoutRecord shapes a text in NotoSans-Regular_var.ttf, the
// same file as NotoSans_variable.ttf of krilla. Its lookup 35, a
// MarkLigPos, covers 287 marks with 281 mark records, and six records have
// a null anchor offset. U+0334 has such a record, and U+1ACB has no record.
// go-text dropped the whole table GPOS, so the text had no kerning.
func TestMarkWithoutRecord(t *testing.T) {
	testShapedGlyphs(t, "../../internal/subsetter/testdata/NotoSans-Regular_var.ttf", []shapedText{
		{"fi\u0334 fl\u1acb AV", []shapedGlyph{
			{1654, 0, 602, 0, 0}, {2712, 0, 0, -129, 0}, {3, 3, 260, 0, 0}, {1655, 4, 602, 0, 0},
			{3767, 4, 0, -130, 765}, {3, 7, 260, 0, 0}, {36, 8, 599, 0, 0}, {57, 9, 600, 0, 0},
		}},
	})
}

// shapeWithin shapes the text in the font, and fails the test when the
// shaping does not end in 30 seconds.
func shapeWithin(t *testing.T, face *font.Font, text string) *Buffer {
	t.Helper()
	done := make(chan *Buffer, 1)
	go func() {
		buf := NewBuffer()
		buf.AddRunes([]rune(text), 0, -1)
		buf.GuessSegmentProperties()
		buf.Shape(NewFont(font.NewFace(face)), nil)
		done <- buf
	}()
	select {
	case buf := <-done:
		return buf
	case <-time.After(30 * time.Second):
		t.Fatalf("%+q: the shaping did not end in 30 seconds", text)
		return nil
	}
}

// TestShapeDuployan shapes two texts in NotoSansDuployan-Regular.otf, which
// makes about 60 glyphs of each character. HarfBuzz 15.1.0 gives the glyph
// count and the hash of the glyphs of each text. The lower limits of go-text stopped the shaping of the first
// text too early, and its loop then did not end. Its reverse chaining
// lookups read stale glyphs, which gave other glyphs for the second text.
func TestShapeDuployan(t *testing.T) {
	testShapedHashes(t, "perf_reference/fonts/NotoSansDuployan-Regular.otf", []shapedHash{
		{"\U0001bc47\U0001bc23 \U0001bc02\U0001bc46 \U0001bc06\U0001bc44\U0001bc09\U0001bc05\U0001bc0b\U0001bc41\U0001bc04\U0001bc03", 1182, 0x19f62eca43f458ed},
		{"\U0001bc02\U0001bc41", 161, 0x660029025185b5f9},
	})
}

// shapedHash is a text, and the glyph count and the hash of the glyphs that
// HarfBuzz 15.1.0 gives for it. The hash is FNV-1a of 64 bits, over the line
// "glyph cluster xAdvance yAdvance xOffset yOffset\n" of each glyph.
type shapedHash struct {
	text   string
	glyphs int
	hash   uint64
}

// testShapedHashes shapes each text in the font of the file of
// typesetting/testdata/harfbuzz, and compares the glyph count and the hash
// of the glyphs with HarfBuzz.
func testShapedHashes(t *testing.T, path string, tests []shapedHash) {
	t.Helper()
	testFontHashes(t, openFontFile(t, path), tests)
}

// testFontHashes shapes each text in the font, and compares the glyph
// count and the hash of the glyphs with HarfBuzz.
func testFontHashes(t *testing.T, face *font.Font, tests []shapedHash) {
	t.Helper()
	for _, test := range tests {
		buf := shapeWithin(t, face, test.text)
		h := fnv.New64a()
		for i, info := range buf.Info {
			pos := buf.Pos[i]
			fmt.Fprintf(h, "%d %d %d %d %d %d\n", info.Glyph, info.Cluster, pos.XAdvance, pos.YAdvance, pos.XOffset, pos.YOffset)
		}
		if len(buf.Info) != test.glyphs || h.Sum64() != test.hash {
			t.Errorf("%+q: got %d glyphs with the hash %#x, want %d with %#x", test.text, len(buf.Info), h.Sum64(), test.glyphs, test.hash)
		}
	}
}

// TestShapeWithoutOperations shapes a text that uses all the operations of
// the buffer. A lookup then applies without a move in the buffer, and the
// loop of go-text did not end. HarfBuzz 15.1.0 also uses all the operations
// for this text. The earlier text of this test no longer uses all the
// operations, because harfbuzz now takes them as HarfBuzz does (D73).
func TestShapeWithoutOperations(t *testing.T) {
	face := openFontFile(t, "perf_reference/fonts/NotoSansDuployan-Regular.otf")
	shapeWithin(t, face, "\U0001bc2c\U0001bc24\U0001bc00\U0001bc2c\U0001bc4e\U0001bc2d\U0001bc26\U0001bc35\U0001bc22\U0001bc3e\U0001bc7a\U0001bc23\U0001bc15\U0001bc84\U0001bc1d\U0001bc01")
}

// TestShapeDuployanOperations shapes two texts in
// NotoSansDuployan-Regular.otf that used all the operations of the buffer
// in go-text, but not in HarfBuzz 15.1.0. go-text set the glyph flags of a
// range whose start, in the output buffer, was after its end, and took the
// operations for them. HarfBuzz returns for such a range.
func TestShapeDuployanOperations(t *testing.T) {
	testShapedHashes(t, "perf_reference/fonts/NotoSansDuployan-Regular.otf", []shapedHash{
		{"\U0001bc99\U0001bc12\U0001bc4d\U0001bc97\U0001bc16", 995, 0x8ccafe247ee09a27},
		{"\U0001bc91\U0001bc85\U0001bc94\U0001bc49\U0001bc00\U0001bc21", 1223, 0x63dcede550394d8b},
	})
}

// TestSetGlyphFlagsBeforeStart sets the flags of a range that starts in the
// output buffer after its end in the buffer. HarfBuzz 15.1.0 computes the
// length of the range without a sign, and sets no flag.
func TestSetGlyphFlagsBeforeStart(t *testing.T) {
	b := NewBuffer()
	b.Info = make([]GlyphInfo, 4)
	b.outInfo = make([]GlyphInfo, 5)
	for i := range b.outInfo {
		b.outInfo[i].Cluster = i
	}
	b.haveOutput = true
	b.idx = 2
	b.maxOps = 100
	b.unsafeToBreakFromOutbuffer(4, 3)
	for i, info := range append(b.outInfo, b.Info...) {
		if info.Mask != 0 {
			t.Errorf("glyph %d has the mask %#x, want 0", i, info.Mask)
		}
	}
	if b.maxOps != 100 {
		t.Errorf("got %d operations left, want 100", b.maxOps)
	}
}

// TestRecurseOperations checks that each nested lookup costs one operation,
// and that a nesting deeper than HB_MAX_NESTING_LEVEL, 64 in HarfBuzz
// 15.1.0, puts the buffer in error, which stops the shaping.
func TestRecurseOperations(t *testing.T) {
	var c otApplyContext
	c.buffer = NewBuffer()
	c.buffer.maxOps = 10
	c.nestingLevelLeft = maxNestingLevel
	depth := 0
	c.recurseFunc = func(c *otApplyContext, _ uint16) bool {
		depth++
		return c.recurse(0)
	}
	if c.recurse(0) {
		t.Error("the nested lookup applied without operations")
	}
	if depth != 10 || c.buffer.maxOps != -1 || !c.buffer.inError {
		t.Errorf("got %d nested lookups, %d operations left and the error %v, want 10, -1 and true", depth, c.buffer.maxOps, c.buffer.inError)
	}

	c.buffer.maxOps, c.buffer.inError, depth = 1000, false, 0
	c.recurse(0)
	if depth != 64 || c.buffer.maxOps != 936 || !c.buffer.inError {
		t.Errorf("got %d nested lookups, %d operations left and the error %v, want 64, 936 and true", depth, c.buffer.maxOps, c.buffer.inError)
	}
}

// TestMergeClustersOperations checks the operations of the merges of
// clusters. As in HarfBuzz 15.1.0, a merge takes one operation for each
// glyph that it changes or extends over, and it does not extend when the
// buffer has no operations left.
func TestMergeClustersOperations(t *testing.T) {
	newBuffer := func(clusters, outClusters []int) *Buffer {
		b := NewBuffer()
		b.ClusterLevel = MonotoneGraphemes
		b.Info = make([]GlyphInfo, len(clusters))
		for i, cluster := range clusters {
			b.Info[i].Cluster = cluster
		}
		b.outInfo = make([]GlyphInfo, len(outClusters))
		for i, cluster := range outClusters {
			b.outInfo[i].Cluster = cluster
		}
		b.haveOutput = outClusters != nil
		b.maxOps = 100
		return b
	}
	clustersOf := func(infos []GlyphInfo) []int {
		var out []int
		for _, info := range infos {
			out = append(out, info.Cluster)
		}
		return out
	}

	// The merge extends to the end of the cluster 1.
	b := newBuffer([]int{0, 1, 1, 1, 2}, nil)
	b.mergeClusters(0, 2)
	if got := clustersOf(b.Info); !slices.Equal(got, []int{0, 0, 0, 0, 2}) || b.maxOps != 96 {
		t.Errorf("mergeClusters: got the clusters %v and %d operations left, want [0 0 0 0 2] and 96", got, b.maxOps)
	}

	// Without operations left, the buffer is in error, and the merge does
	// not extend.
	b = newBuffer([]int{0, 1, 1, 1, 2}, nil)
	b.maxOps, b.inError = -1, true
	b.mergeClusters(0, 2)
	if got := clustersOf(b.Info); !slices.Equal(got, []int{0, 0, 1, 1, 2}) || b.maxOps != -3 {
		t.Errorf("mergeClusters without operations: got the clusters %v and %d operations left, want [0 0 1 1 2] and -3", got, b.maxOps)
	}

	// The merge of the output buffer extends to the start of the cluster 3.
	b = newBuffer([]int{4}, []int{3, 3, 1})
	b.mergeOutClusters(1, 3)
	if got := clustersOf(b.outInfo); !slices.Equal(got, []int{1, 1, 1}) || b.maxOps != 97 {
		t.Errorf("mergeOutClusters: got the clusters %v and %d operations left, want [1 1 1] and 97", got, b.maxOps)
	}

	// The deleted glyph merges its cluster into the two glyphs before it.
	b = newBuffer([]int{2, 6}, []int{5, 5})
	b.deleteGlyph()
	if got := clustersOf(b.outInfo); !slices.Equal(got, []int{2, 2}) || b.maxOps != 98 {
		t.Errorf("deleteGlyph: got the clusters %v and %d operations left, want [2 2] and 98", got, b.maxOps)
	}

	b = newBuffer([]int{5, 5, 2}, nil)
	b.Pos = make([]GlyphPosition, 3)
	b.deleteGlyphsInplace(func(info *GlyphInfo) bool { return info.Cluster == 2 })
	if got := clustersOf(b.Info); !slices.Equal(got, []int{2, 2}) || b.maxOps != 98 {
		t.Errorf("deleteGlyphsInplace: got the clusters %v and %d operations left, want [2 2] and 98", got, b.maxOps)
	}
}

// TestAttachmentNesting checks that the offsets of a chain of attachments
// propagate to at most HB_MAX_NESTING_LEVEL glyphs, 64 in HarfBuzz 15.1.0.
// Each of 70 glyphs has the offset 1 and attaches to the next glyph. The
// glyph 64 does not take the offset of its parent, so the first glyph has
// 65, not 70. go-text had a nesting level of 6 for the cursive attachments,
// and no limit here.
func TestAttachmentNesting(t *testing.T) {
	pos := make([]GlyphPosition, 70)
	for i := range pos {
		pos[i].YOffset = 1
		if i < len(pos)-1 {
			pos[i].attachChain = 1
			pos[i].attachType = attachTypeCursive
		}
	}
	for i := range pos {
		if pos[i].attachChain != 0 {
			propagateAttachmentOffsets(NewBuffer(), pos, i, LeftToRight)
		}
	}
	if pos[0].YOffset != 65 || pos[63].YOffset != 2 || pos[64].YOffset != 1 || pos[65].YOffset != 5 {
		t.Errorf("got the offsets %d, %d, %d and %d, want 65, 2, 1 and 5", pos[0].YOffset, pos[63].YOffset, pos[64].YOffset, pos[65].YOffset)
	}
}

// The glyphs of Roboto-Regular.ttf of font/testdata, for the fonts that
// fontWithLayout builds.
const (
	robotoA GID = 69
	robotoB GID = 70
	robotoC GID = 71
	robotoX GID = 92
	robotoY GID = 93
	robotoZ GID = 94
)

// u16s gives the values as 16-bit integers in big-endian order.
func u16s(values ...int) []byte {
	out := make([]byte, 0, 2*len(values))
	for _, v := range values {
		out = binary.BigEndian.AppendUint16(out, uint16(v))
	}
	return out
}

// offsetTo gives a 16-bit offset, and fails the test when it does not fit.
func offsetTo(t testing.TB, offset int) int {
	t.Helper()
	if offset > 0xFFFF {
		t.Fatalf("the offset %d does not fit in 16 bits", offset)
	}
	return offset
}

// layoutFeature is a feature of a table GSUB or GPOS of fontWithLayout.
type layoutFeature struct {
	tag     string
	lookups []int
}

// layoutTable builds a table GSUB or GPOS. Each script has one default
// LangSys with all the features. A lookup is the bytes of a whole Lookup
// table. The offsets have 16 bits, so only the last lookup can be larger
// than 64 KiB.
func layoutTable(t testing.TB, scripts []string, features []layoutFeature, lookups [][]byte) []byte {
	t.Helper()
	// All the scripts share one Script table, with one default LangSys.
	scriptList := u16s(len(scripts))
	for _, tag := range scripts {
		scriptList = append(scriptList, tag...)
		scriptList = append(scriptList, u16s(2+6*len(scripts))...)
	}
	scriptList = append(scriptList, u16s(4, 0)...)
	scriptList = append(scriptList, u16s(0, 0xFFFF, len(features))...)
	for i := range features {
		scriptList = append(scriptList, u16s(i)...)
	}

	featureList := u16s(len(features))
	var featureTables []byte
	for _, f := range features {
		featureList = append(featureList, f.tag...)
		featureList = append(featureList, u16s(offsetTo(t, 2+6*len(features)+len(featureTables)))...)
		featureTables = append(featureTables, u16s(0, len(f.lookups))...)
		featureTables = append(featureTables, u16s(f.lookups...)...)
	}
	featureList = append(featureList, featureTables...)

	lookupList := u16s(len(lookups))
	var lookupTables []byte
	for _, lookup := range lookups {
		lookupList = append(lookupList, u16s(offsetTo(t, 2+2*len(lookups)+len(lookupTables)))...)
		lookupTables = append(lookupTables, lookup...)
	}
	lookupList = append(lookupList, lookupTables...)

	out := []byte{0, 1, 0, 0}
	out = append(out, u16s(10, offsetTo(t, 10+len(scriptList)), offsetTo(t, 10+len(scriptList)+len(featureList)))...)
	out = append(out, scriptList...)
	out = append(out, featureList...)
	return append(out, lookupList...)
}

// lookupTable builds a Lookup table with its subtables. Only the last
// subtable can be larger than 64 KiB.
func lookupTable(t testing.TB, lookupType, flag int, subtables ...[]byte) []byte {
	t.Helper()
	out := u16s(lookupType, flag, len(subtables))
	var data []byte
	for _, subtable := range subtables {
		out = append(out, u16s(offsetTo(t, 6+2*len(subtables)+len(data)))...)
		data = append(data, subtable...)
	}
	return append(out, data...)
}

// coverage1 builds a Coverage table of format 1. The glyphs must be sorted.
func coverage1(glyphs ...GID) []byte {
	out := u16s(1, len(glyphs))
	for _, g := range glyphs {
		out = append(out, u16s(int(g))...)
	}
	return out
}

// fontWithLayout gives Roboto-Regular.ttf of font/testdata with the tables
// of layout in place of its tables GDEF, GSUB and GPOS.
func fontWithLayout(t testing.TB, layout map[string][]byte) *font.Face {
	t.Helper()
	ld, err := ot.NewLoader(bytes.NewReader(fontDataWithLayout(t, layout)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := font.NewFont(ld)
	if err != nil {
		t.Fatal(err)
	}
	return font.NewFace(f)
}

// fontDataWithLayout gives the bytes of the font of fontWithLayout.
func fontDataWithLayout(t testing.TB, layout map[string][]byte) []byte {
	t.Helper()
	data, err := os.ReadFile("../font/testdata/Roboto-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	return replaceLayout(t, data, layout)
}

// replaceLayout gives the bytes of the font data with the tables of layout
// in place of its tables GDEF, GSUB and GPOS.
func replaceLayout(t testing.TB, data []byte, layout map[string][]byte) []byte {
	t.Helper()
	ld, err := ot.NewLoader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var tabs []ot.Table
	for _, tag := range ld.Tables() {
		switch tag.String() {
		case "GDEF", "GSUB", "GPOS":
			continue
		}
		raw, err := ld.RawTable(tag)
		if err != nil {
			t.Fatal(err)
		}
		tabs = append(tabs, ot.Table{Tag: tag, Content: raw})
	}
	for tag, content := range layout {
		tabs = append(tabs, ot.Table{Tag: ot.MustNewTag(tag), Content: content})
	}
	slices.SortFunc(tabs, func(a, b ot.Table) int { return cmp.Compare(a.Tag, b.Tag) })
	return ot.WriteOpentype(tabs, ot.TrueType)
}

// gposPairClassZero builds a table GPOS whose feature kern has one lookup
// of two subtables. The first is a PairPos of format 2 for a, whose
// ClassDef2 holds only c. It gives the advance -100 to the class 0 and -200
// to the class 1. The second is a PairPos of format 1 for the pair a, b,
// with -300.
func gposPairClassZero(t testing.TB) []byte {
	pair2 := u16s(2, 20, 4, 0, 26, 30, 1, 2, -100, -200)
	pair2 = append(pair2, coverage1(robotoA)...)
	pair2 = append(pair2, u16s(2, 0)...)                  // ClassDef1: no range, so a has the class 0
	pair2 = append(pair2, u16s(1, int(robotoC), 1, 1)...) // ClassDef2: c has the class 1
	pair1 := u16s(1, 18, 4, 0, 1, 12, 1, int(robotoB), -300)
	pair1 = append(pair1, coverage1(robotoA)...)
	return layoutTable(t, []string{"DFLT"}, []layoutFeature{{"kern", []int{0}}},
		[][]byte{lookupTable(t, 2, 0, pair2, pair1)})
}

// TestPairPosClassZero shapes two texts with gposPairClassZero. In
// HarfBuzz 15.1.0, b has the class 0 in the first subtable, which applies
// with the advance -100. go-text did not apply it, and the second subtable
// applied -300.
func TestPairPosClassZero(t *testing.T) {
	face := fontWithLayout(t, map[string][]byte{"GPOS": gposPairClassZero(t)})
	if got := len(face.GPOS.Lookups); got != 1 {
		t.Fatalf("got %d lookups of GPOS, want 1", got)
	}
	testShapedFace(t, face, []shapedText{
		{"ab", []shapedGlyph{{robotoA, 0, 1014, 0, 0}, {robotoB, 1, 1149, 0, 0}}},
		{"ac", []shapedGlyph{{robotoA, 0, 914, 0, 0}, {robotoC, 1, 1072, 0, 0}}},
	})
}

// anchor1 builds an Anchor table of format 1.
func anchor1(x, y int) []byte {
	return u16s(1, x, y)
}

// gposCursiveCycle builds a table GDEF that makes b a mark, and a table
// GPOS whose feature kern has two cursive lookups, and whose feature mark
// attaches d to c. The first cursive lookup has the flag RightToLeft, and
// attaches a to b and b to c. The second has the flag IgnoreMarks, skips
// b, and attaches c to a. So the cursive attachments of a, b and c form a
// cycle.
func gposCursiveCycle(t testing.TB) (gdef, gpos []byte) {
	t.Helper()
	gdef = u16s(1, 0, 12, 0, 0, 0)
	gdef = append(gdef, u16s(2, 1, int(robotoB), int(robotoB), 3)...)
	rtl := u16s(1, 18, 3, 0, 28, 34, 40, 46, 0)
	rtl = append(rtl, coverage1(robotoA, robotoB, robotoC)...)
	rtl = append(rtl, anchor1(500, 100)...)
	rtl = append(rtl, anchor1(0, 0)...)
	rtl = append(rtl, anchor1(300, 200)...)
	rtl = append(rtl, anchor1(0, 50)...)
	skip := u16s(1, 14, 2, 0, 22, 28, 0)
	skip = append(skip, coverage1(robotoA, robotoC)...)
	skip = append(skip, anchor1(500, 300)...)
	skip = append(skip, anchor1(0, 0)...)
	markBase := u16s(1, 12, 18, 1, 24, 36)
	markBase = append(markBase, coverage1(robotoD)...)
	markBase = append(markBase, coverage1(robotoC)...)
	markBase = append(markBase, u16s(1, 0, 6)...)
	markBase = append(markBase, anchor1(0, 0)...)
	markBase = append(markBase, u16s(1, 4)...)
	markBase = append(markBase, anchor1(200, 400)...)
	gpos = layoutTable(t, []string{"DFLT", "latn"},
		[]layoutFeature{{"kern", []int{0, 1}}, {"mark", []int{2}}},
		[][]byte{lookupTable(t, 3, 1, rtl), lookupTable(t, 3, 8, skip), lookupTable(t, 4, 0, markBase)})
	return gdef, gpos
}

// TestCursiveCycle shapes abcd with gposCursiveCycle. The cursive
// attachments of a, b and c form a cycle, and d attaches to c. As in
// HarfBuzz 15.1.0, the walk of the attachments of c for the offset of d
// stops after 3 attachments, one less than the number of glyphs, and
// test/harfbuzz gives the same glyphs. go-text did not stop (D73).
func TestCursiveCycle(t *testing.T) {
	gdef, gpos := gposCursiveCycle(t)
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"GDEF": gdef, "GPOS": gpos}), []shapedText{
		{"abcd", []shapedGlyph{
			{robotoA, 0, 500, 0, -50}, {robotoB, 1, 0, 0, 50},
			{robotoC, 2, 1072, 0, 200}, {robotoD, 3, 1155, -872, 750},
		}},
	})
}

// TestCrossOffsetBounds checks two bounds of resolveCrossOffset, as in
// HarfBuzz 15.1.0. A parent before the start of the buffer ends the walk,
// where go-text read the glyph at a negative index. A sum of the offsets
// is clamped to the range of a position, where go-text let it wrap (D73).
func TestCrossOffsetBounds(t *testing.T) {
	pos := make([]GlyphPosition, 2)
	pos[1].YOffset = 7
	pos[1].attachChain = -2
	pos[1].attachType = attachTypeCursive
	if got := resolveCrossOffset(pos, 1, LeftToRight, false); got != 7 {
		t.Errorf("a parent before the start: got the offset %d, want 7", got)
	}
	pos[0].YOffset = math.MaxInt32
	pos[1].YOffset = math.MaxInt32
	pos[1].attachChain = -1
	if got := resolveCrossOffset(pos, 1, LeftToRight, false); got != math.MaxInt32 {
		t.Errorf("a sum past the range: got the offset %d, want %d", got, math.MaxInt32)
	}
}

// TestReverseCursiveBounds checks reverseCursiveMinorOffset, as in
// HarfBuzz 15.1.0. The negated offset of a glyph is clamped to the range
// of a position, where go-text let -MinInt32 wrap. A chain of -32,768,
// whose reverse does not fit in 16 bits, leaves the parent detached, where
// go-text stored a chain that wrapped (D73).
func TestReverseCursiveBounds(t *testing.T) {
	pos := make([]GlyphPosition, 2)
	pos[1].YOffset = math.MinInt32
	pos[1].attachChain = -1
	pos[1].attachType = attachTypeCursive
	reverseCursiveMinorOffset(pos, 1, LeftToRight, len(pos), 0)
	if pos[0].YOffset != math.MaxInt32 || pos[0].attachChain != 1 || pos[1].attachChain != 0 {
		t.Errorf("negated offset: got the offset %d and the chains %d, %d, want %d, 1 and 0",
			pos[0].YOffset, pos[0].attachChain, pos[1].attachChain, math.MaxInt32)
	}
	pos = make([]GlyphPosition, 32769)
	pos[32768].attachChain = math.MinInt16
	pos[32768].attachType = attachTypeCursive
	reverseCursiveMinorOffset(pos, 32768, LeftToRight, len(pos), 0)
	if pos[0].attachChain != 0 || pos[0].attachType != 0 || pos[32768].attachChain != 0 {
		t.Errorf("reversed chain: got the chains %d and %d, want 0 and 0", pos[0].attachChain, pos[32768].attachChain)
	}
}

// TestAttachmentClamp checks that propagateAttachmentOffsets clamps each
// sum to the range of a position, as HarfBuzz 15.1.0 does. A cursive
// glyph adds the offset of its parent, and a mark subtracts the advances
// of the glyphs from its base. go-text let each sum wrap (D73).
func TestAttachmentClamp(t *testing.T) {
	pos := make([]GlyphPosition, 2)
	pos[0].YOffset = math.MaxInt32
	pos[1].YOffset = math.MaxInt32
	pos[1].attachChain = -1
	pos[1].attachType = attachTypeCursive
	propagateAttachmentOffsets(NewBuffer(), pos, 1, LeftToRight)
	if got := pos[1].YOffset; got != math.MaxInt32 {
		t.Errorf("cursive: got the offset %d, want %d", got, math.MaxInt32)
	}
	pos = make([]GlyphPosition, 3)
	pos[0].XAdvance = math.MaxInt32
	pos[1].XAdvance = math.MaxInt32
	pos[2].attachChain = -2
	pos[2].attachType = attachTypeMark
	propagateAttachmentOffsets(NewBuffer(), pos, 2, LeftToRight)
	if got := pos[2].XOffset; got != math.MinInt32 {
		t.Errorf("mark: got the offset %d, want %d", got, math.MinInt32)
	}
}

// gsubNestedRecords builds a table GSUB with two lookups. The feature ccmp
// has lookup 1, a context of one glyph. For x, it calls lookup 0 once, and
// for a, it calls lookup 0 the given number of times. Lookup 0, a
// MultipleSubst, replaces a with b, b with a, and x with z, y. So each call
// for a replaces the glyph, and the next call moves back to it first. Each
// call then takes two operations.
func gsubNestedRecords(t testing.TB, records int) []byte {
	multiple := u16s(1, 26, 3, 12, 16, 20, 1, int(robotoB), 1, int(robotoA), 2, int(robotoZ), int(robotoY))
	multiple = append(multiple, coverage1(robotoA, robotoB, robotoX)...)
	// The rule set of a has one rule with the records, and the rule set of
	// x has one rule with one record.
	context := u16s(1, 10, 2, 30, 18)
	context = append(context, coverage1(robotoA, robotoX)...)
	context = append(context, u16s(1, 4, 1, 1, 0, 0)...)
	context = append(context, u16s(1, 4, 1, records)...)
	for range records {
		context = append(context, u16s(0, 0)...)
	}
	return layoutTable(t, []string{"DFLT"}, []layoutFeature{{"ccmp", []int{1}}},
		[][]byte{lookupTable(t, 2, 0, multiple), lookupTable(t, 5, 0, context)})
}

// gsubMultiple builds a table GSUB whose feature ccmp replaces x with count
// glyphs y.
func gsubMultiple(t testing.TB, count int) []byte {
	multiple := u16s(1, 8, 1, 14)
	multiple = append(multiple, coverage1(robotoX)...)
	multiple = append(multiple, u16s(count)...)
	for range count {
		multiple = append(multiple, u16s(int(robotoY))...)
	}
	return layoutTable(t, []string{"DFLT"}, []layoutFeature{{"ccmp", []int{0}}},
		[][]byte{lookupTable(t, 2, 0, multiple)})
}

// TestDropOutputWithoutOperations shapes xaaaaaaa with gsubNestedRecords
// and 65,535 records. Each a takes about 130,000 operations, so the buffer
// has no operations left in the middle of the lookup, at the fifth a. The
// output of the lookup has its own array from x on, because z, y is longer
// than x. HarfBuzz then drops the output, and keeps the array of the input,
// where each move back of the lookup wrote a glyph: x, then b at the
// fifth a. go-text kept the output (D73). The text has seven a, so that it
// takes more than the 524,288 operations of HarfBuzz 15.2.0 (D97).
func TestDropOutputWithoutOperations(t *testing.T) {
	face := fontWithLayout(t, map[string][]byte{"GSUB": gsubNestedRecords(t, 65535)})
	if got := len(face.GSUB.Lookups); got != 2 {
		t.Fatalf("got %d lookups of GSUB, want 2", got)
	}
	testShapedFace(t, face, []shapedText{
		{"xaaaaaaa", []shapedGlyph{
			{robotoX, 0, 1015, 0, 0}, {robotoA, 1, 1114, 0, 0}, {robotoA, 2, 1114, 0, 0}, {robotoA, 3, 1114, 0, 0},
			{robotoA, 4, 1114, 0, 0}, {robotoB, 5, 1149, 0, 0}, {robotoA, 6, 1114, 0, 0}, {robotoA, 7, 1114, 0, 0},
		}},
	})
}

// TestBufferLengthLimit shapes xx with gsubMultiple. The limit of the
// length of the buffer is 65,536 glyphs for a text of two characters. As in
// HarfBuzz 15.1.0, harfbuzz compares a length with the limit only when the
// arrays of HarfBuzz must grow, and they grow from 62,922 to 94,415 glyphs.
// So two times 40,000 glyphs do not reach the limit. Two times 50,000
// glyphs do, the buffer is in error, and it keeps xx. go-text compared the
// length with the limit only before each lookup, and kept the 100,000
// glyphs.
func TestBufferLengthLimit(t *testing.T) {
	for _, test := range []struct {
		count, glyphs int
		hash          uint64
	}{
		{40000, 80000, 0xfe5097d9d51cb0e5},
		{50000, 2, 0xbe00a6b1239c748e},
	} {
		face := fontWithLayout(t, map[string][]byte{"GSUB": gsubMultiple(t, test.count)})
		buf := NewBuffer()
		buf.AddRunes([]rune("xx"), 0, -1)
		buf.GuessSegmentProperties()
		buf.Shape(NewFont(face), nil)
		h := fnv.New64a()
		for i, info := range buf.Info {
			pos := buf.Pos[i]
			fmt.Fprintf(h, "%d %d %d %d %d %d\n", info.Glyph, info.Cluster, pos.XAdvance, pos.YAdvance, pos.XOffset, pos.YOffset)
		}
		if len(buf.Info) != test.glyphs || h.Sum64() != test.hash {
			t.Errorf("%d glyphs for x: got %d glyphs with the hash %#x, want %d with %#x", test.count, len(buf.Info), h.Sum64(), test.glyphs, test.hash)
		}
	}
}

// TestSwapBuffersInError ends a lookup with and without an error. As
// hb_buffer_t::sync of HarfBuzz 15.1.0, swapBuffers keeps the output when
// the buffer is not in error. In error, it drops the output and keeps Info,
// with the glyph that the lookup wrote into the array of Info before the
// output took an array of its own.
func TestSwapBuffersInError(t *testing.T) {
	for _, inError := range []bool{false, true} {
		b := NewBuffer()
		b.AddRunes([]rune("abc"), 0, -1)
		b.clearOutput()
		b.replaceGlyphIndex(7) // in the array of Info
		b.outputGlyphIndex(8)  // longer than the input, so in an array of its own
		b.nextGlyph()
		b.inError = inError
		ok := b.swapBuffers()
		var got []GID
		for _, info := range b.Info {
			got = append(got, info.Glyph)
		}
		want := []GID{7, 8, 0, 0}
		if inError {
			want = []GID{7, 0, 0}
		}
		if ok == inError || !slices.Equal(got, want) {
			t.Errorf("in error %v: got %v and %v, want %v and %v", inError, ok, got, !inError, want)
		}
	}
}

// TestBufferReuse shapes a short text, then puts a text of 70,000
// characters in the same buffer. The limit of the length for the short
// text is 65,536 glyphs. As in HarfBuzz 15.1.0, the limit goes back to its
// default after the shaping, so the buffer takes each character.
func TestBufferReuse(t *testing.T) {
	face := fontWithLayout(t, nil)
	b := NewBuffer()
	b.AddRunes([]rune("ab"), 0, -1)
	b.GuessSegmentProperties()
	b.Shape(NewFont(face), nil)
	b.Clear()
	b.AddRunes([]rune(strings.Repeat("a", 70000)), 0, -1)
	if len(b.Info) != 70000 || b.inError {
		t.Errorf("got %d glyphs and the error %v, want 70000 and false", len(b.Info), b.inError)
	}
}

// u32s gives the values as 32-bit integers in big-endian order.
func u32s(values ...uint32) []byte {
	out := make([]byte, 0, 4*len(values))
	for _, v := range values {
		out = binary.BigEndian.AppendUint32(out, v)
	}
	return out
}

// morxInsertions builds a table morx with one chain of count insertion
// subtables, then a noncontextual subtable. Each insertion subtable inserts
// 31 times a after each a, and the noncontextual subtable replaces a with b.
func morxInsertions(count int) []byte {
	// The state table: the class 4 for a, two states that give the entry
	// 1 for a, and the entry 1 inserts 31 glyphs after the current glyph.
	insertion := u32s(5, 20, 28, 48, 64)
	insertion = append(insertion, u16s(8, int(robotoA), 1, 4)...)
	insertion = append(insertion, u16s(0, 0, 0, 0, 1, 0, 0, 0, 0, 1)...)
	insertion = append(insertion, u16s(0, 0, 0xFFFF, 0xFFFF, 0, 31<<5, 0, 0xFFFF)...)
	for range 31 {
		insertion = append(insertion, u16s(int(robotoA))...)
	}
	// costly is insertion, whose entry 1 inserts 31 glyphs before and 31
	// glyphs after the current glyph, from the index 0xFFFE, which is not
	// in the subtable.
	costly := u32s(5, 20, 28, 48, 64)
	costly = append(costly, u16s(8, int(robotoA), 1, 4)...)
	costly = append(costly, u16s(0, 0, 0, 0, 1, 0, 0, 0, 0, 1)...)
	costly = append(costly, u16s(0, 0, 0xFFFF, 0xFFFF, 0, 31<<5|31, 0xFFFE, 0xFFFE)...)
	noncontextual := u16s(8, int(robotoA), 1, int(robotoB)) // a lookup of format 8

	length := 16 + count*(12+len(insertion)) + 12 + len(costly) + 12 + len(noncontextual)
	out := u16s(2, 0)
	out = append(out, u32s(1, 1, uint32(length), 0, uint32(count+2))...) // one chain
	for range count {
		// The length, the coverage for all the directions and the type,
		// and the flags.
		out = append(out, u32s(uint32(12+len(insertion)), 0x20000005, 1)...)
		out = append(out, insertion...)
	}
	out = append(out, u32s(uint32(12+len(costly)), 0x20000005, 1)...)
	out = append(out, costly...)
	out = append(out, u32s(uint32(12+len(noncontextual)), 0x20000004, 1)...)
	return append(out, noncontextual...)
}

// TestMorxSubtableOperations shapes a with morxInsertions and three
// insertion subtables, which make 32, 1,024, then 32,768 glyphs a. The
// buffer has 524,288 operations. The fourth subtable takes 62 operations
// for each a, for the insertions that are not in the subtable, as
// HarfBuzz 15.2.0 does. The buffer is then in error, and the noncontextual
// subtable does not apply. go-text took no operation for an insertion,
// and gave 32,768 glyphs b (D73, D97).
func TestMorxSubtableOperations(t *testing.T) {
	face := fontWithLayout(t, map[string][]byte{"morx": morxInsertions(3)})
	if got := len(face.Morx); got != 1 || len(face.Morx[0].Subtables) != 5 {
		t.Fatalf("got %d chains of morx, want 1 with 5 subtables", got)
	}
	testFontHashes(t, face.Font, []shapedHash{{"a", 32768, 0x9458d2032c2f2325}})
}

// gsubIndicRules builds a table GSUB for the script dev2 of
// NotoSansDevanagari-Regular.ttf, from the glyphs of ka, kha and the virama.
// The feature pres replaces ka with kha. The features blwf, vatu, pstf and
// pref have a context of format 1 for the virama, with 32,000 rules that
// match no text. The rules share their bytes.
func gsubIndicRules(t testing.TB, ka, kha, virama GID) []byte {
	const rules = 32000
	single := u16s(1, 6, int(kha-ka))
	single = append(single, coverage1(ka)...)
	context := u16s(1, 8, 1, 14)
	context = append(context, coverage1(virama)...)
	context = append(context, u16s(rules)...)
	for range rules {
		context = append(context, u16s(2+2*rules)...)
	}
	context = append(context, u16s(2, 0, 1)...) // the rule: the virama, then the glyph 1
	return layoutTable(t, []string{"DFLT", "dev2"},
		[]layoutFeature{{"blwf", []int{1}}, {"pref", []int{1}}, {"pres", []int{0}}, {"pstf", []int{1}}, {"vatu", []int{1}}},
		[][]byte{lookupTable(t, 1, 0, single), lookupTable(t, 5, 0, context)})
}

// TestIndicWouldSubstituteOperations shapes six times ka with
// gsubIndicRules. To find the position of each consonant, the Indic shaper
// asks whether blwf, vatu, pstf and pref substitute the virama and the
// consonant. As in HarfBuzz, each question takes an operation for each rule
// of the rule set of the virama (D73). Before the last consonant, the
// buffer has no operations left of the 524,288 of HarfBuzz 15.2.0, and it
// is in error, so pres does not apply (D97). go-text took no operation,
// and pres replaced each ka with kha.
func TestIndicWouldSubstituteOperations(t *testing.T) {
	data, err := td.Files.ReadFile("perf_reference/fonts/NotoSansDevanagari-Regular.ttf")
	if err != nil {
		t.Skip(err)
	}
	const ka, kha, virama = 25, 26, 81
	gsub := gsubIndicRules(t, ka, kha, virama)
	ld, err := ot.NewLoader(bytes.NewReader(replaceLayout(t, data, map[string][]byte{"GSUB": gsub})))
	if err != nil {
		t.Fatal(err)
	}
	f, err := font.NewFont(ld)
	if err != nil {
		t.Fatal(err)
	}
	face := font.NewFace(f)
	if got := len(face.GSUB.Lookups); got != 2 {
		t.Fatalf("got %d lookups of GSUB, want 2", got)
	}
	testShapedFace(t, face, []shapedText{{"कककककक", []shapedGlyph{
		{ka, 0, 762, 0, 0}, {ka, 1, 762, 0, 0}, {ka, 2, 762, 0, 0}, {ka, 3, 762, 0, 0},
		{ka, 4, 762, 0, 0}, {ka, 5, 762, 0, 0},
	}}})
}

// gsubIndicFirstCoverage builds a table GSUB for the script dev2 of
// NotoSansDevanagari-Regular.ttf, from the glyphs of ka, kha and the
// virama. The feature blwf has one lookup of two chained contexts. The
// first, of format 1, covers ka, and its one rule matches no text, so ka
// is in the digest of the lookup. The second, of format 3, matches kha and
// the virama, with no nested lookup.
func gsubIndicFirstCoverage(t testing.TB, ka, kha, virama GID) []byte {
	format1 := u16s(1, 8, 1, 14)
	format1 = append(format1, coverage1(ka)...)
	format1 = append(format1, u16s(1, 4, 0, 2, 1, 0, 0)...) // one rule: ka, then the glyph 1
	format3 := u16s(3, 0, 2, 14, 20, 0, 0)
	format3 = append(format3, coverage1(kha)...)
	format3 = append(format3, coverage1(virama)...)
	return layoutTable(t, []string{"DFLT", "dev2"}, []layoutFeature{{"blwf", []int{0}}},
		[][]byte{lookupTable(t, 6, 0, format1, format3)})
}

// TestIndicFirstGlyphCoverage shapes ka, virama, ka, i with
// gsubIndicFirstCoverage. To find the position of the second ka, the Indic
// shaper asks whether blwf substitutes ka and the virama. In HarfBuzz
// 15.1.0, the context of format 3 does not test the coverage of ka, so the
// answer is yes. The second ka is then below the base, and the matra i
// moves before the first ka. go-text tested the coverage, so the second ka
// was the base, and i moved before it.
func TestIndicFirstGlyphCoverage(t *testing.T) {
	data, err := td.Files.ReadFile("perf_reference/fonts/NotoSansDevanagari-Regular.ttf")
	if err != nil {
		t.Skip(err)
	}
	const ka, kha, virama, i = 25, 26, 81, 67
	gsub := gsubIndicFirstCoverage(t, ka, kha, virama)
	ld, err := ot.NewLoader(bytes.NewReader(replaceLayout(t, data, map[string][]byte{"GSUB": gsub})))
	if err != nil {
		t.Fatal(err)
	}
	f, err := font.NewFont(ld)
	if err != nil {
		t.Fatal(err)
	}
	testShapedFace(t, font.NewFace(f), []shapedText{{"क्कि", []shapedGlyph{
		{i, 0, 259, 0, 0}, {ka, 0, 762, 0, 0}, {virama, 0, 0, 0, 0}, {ka, 0, 762, 0, 0},
	}}})
}

// kerxSubtable is a subtable of a table kerx: its coverage, with the format
// in the low byte, and its data after its header of 12 bytes. When length
// is not 0, it is the length that the header gives, in place of the real
// length. tuples is its tuple count.
type kerxSubtable struct {
	coverage uint32
	data     []byte
	length   int
	tuples   int
}

// kerxTable builds a table kerx of version 2 with the subtables.
func kerxTable(subtables ...kerxSubtable) []byte {
	out := u16s(2, 0)
	out = append(out, u32s(uint32(len(subtables)))...)
	for _, st := range subtables {
		length := 12 + len(st.data)
		if st.length != 0 {
			length = st.length
		}
		out = append(out, u32s(uint32(length), st.coverage, uint32(st.tuples))...)
		out = append(out, st.data...)
	}
	return out
}

// kerxPairs builds a subtable of format 0 of kerx with the pairs, each a
// left glyph, a right glyph and a value. The pairs must be sorted.
func kerxPairs(pairs ...int) kerxSubtable {
	n := uint32(len(pairs) / 3)
	data := u32s(n, 6*n, 0, 0)
	data = append(data, u16s(pairs...)...)
	return kerxSubtable{coverage: 0, data: data}
}

// aatSubtables builds a table kerx, a table kern and a table morx with
// count subtables. In kerx and kern, the last subtable kerns a, b by -300,
// and the other subtables are empty. morx has one chain, whose subtables
// each replace a with b.
func aatSubtables(count int) (kerx, kern, morx []byte) {
	subtables := make([]kerxSubtable, count)
	for i := range count - 1 {
		subtables[i] = kerxPairs()
	}
	subtables[count-1] = kerxPairs(int(robotoA), int(robotoB), -300)
	kerx = kerxTable(subtables...)

	kern = u16s(0, count)
	for range count - 1 {
		kern = append(kern, u16s(0, 14, 1, 0, 0, 0, 0)...)
	}
	kern = append(kern, u16s(0, 20, 1, 1, 6, 0, 0, int(robotoA), int(robotoB), -300)...)

	noncontextual := u16s(8, int(robotoA), 1, int(robotoB))
	morx = u16s(2, 0)
	morx = append(morx, u32s(1, 1, uint32(16+count*(12+len(noncontextual))), 0, uint32(count))...)
	for range count {
		morx = append(morx, u32s(uint32(12+len(noncontextual)), 0x20000004, 1)...)
		morx = append(morx, noncontextual...)
	}
	return kerx, kern, morx
}

// TestAATSubtableLimit shapes ab with the tables of aatSubtables. HarfBuzz
// 15.1.0 refuses a table kern, kerx or morx with more than 1,024
// subtables, HB_AAT_MAX_SUBTABLES of hb-limits.hh, and so does font.
// go-text read the tables with 1,025 subtables (D73).
func TestAATSubtableLimit(t *testing.T) {
	kerned := []shapedGlyph{{robotoA, 0, 964, 0, 0}, {robotoB, 1, 999, -150, 0}}
	plain := []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoB, 1, 1149, 0, 0}}
	for _, test := range []struct {
		count        int
		kerned, morx []shapedGlyph
	}{
		{1024, kerned, []shapedGlyph{{robotoB, 0, 1149, 0, 0}, {robotoB, 1, 1149, 0, 0}}},
		{1025, plain, plain},
	} {
		want := test.count
		if want > 1024 {
			want = 0
		}
		kerx, kern, morx := aatSubtables(test.count)
		kerxFace := fontWithLayout(t, map[string][]byte{"kerx": kerx})
		kernFace := fontWithLayout(t, map[string][]byte{"kern": kern})
		morxFace := fontWithLayout(t, map[string][]byte{"morx": morx})
		morxCount := 0
		for _, c := range morxFace.Morx {
			morxCount += len(c.Subtables)
		}
		if len(kerxFace.Kerx) != want || len(kernFace.Kern) != want || morxCount != want {
			t.Errorf("%d subtables: got %d subtables of kerx, %d of kern and %d of morx, want %d",
				test.count, len(kerxFace.Kerx), len(kernFace.Kern), morxCount, want)
		}
		testShapedFace(t, kerxFace, []shapedText{{"ab", test.kerned}})
		testShapedFace(t, kernFace, []shapedText{{"ab", test.kerned}})
		testShapedFace(t, morxFace, []shapedText{{"ab", test.morx}})
	}
}

// The flags of an entry of a subtable of format 1 of kerx.
const (
	kerxPush  = 0x8000
	kerxReset = 0x2000
	kerxEnd   = 0xFFFF // the index of no list of actions, and the end of a list
)

// kerxFormat1 builds a subtable of format 1 of kerx. Its class table gives
// the classes 4, 5 and 6 to a, b and c. An entry is its flags and the index
// of its list of actions in the values. entries[0] is the entry of the
// classes 0 to 3, and entries[1], entries[2] and entries[3] are the
// entries of a, b and c, in each of the two states.
func kerxFormat1(entries [4][2]int, values ...int) kerxSubtable {
	const classes = 7
	classTable := u16s(8, int(robotoA), 3, 4, 5, 6)
	var states []byte
	for range 2 {
		states = append(states, u16s(0, 0, 0, 0, 1, 2, 3)...)
	}
	var entryTable []byte
	for _, e := range entries {
		entryTable = append(entryTable, u16s(0, e[0], e[1])...)
	}
	data := u32s(classes, 20, uint32(20+len(classTable)), uint32(20+len(classTable)+len(states)),
		uint32(20+len(classTable)+len(states)+len(entryTable)))
	data = append(data, classTable...)
	data = append(data, states...)
	data = append(data, entryTable...)
	data = append(data, u16s(values...)...)
	return kerxSubtable{coverage: 1, data: data}
}

// TestKerxFormat1History shapes texts with a subtable of format 1 of kerx.
// It pushes a, pushes b and applies the list 51, 41, -31 at b, and applies
// a list of ten values at c, with the flag Reset and without a push. As in
// HarfBuzz 15.1.0, the stack is a history of eight glyphs, which each list
// walks again from the last glyph, and a slot that no glyph filled holds
// the first glyph. A list ends with 0xFFFF, not with an odd value, the
// flag Reset is ignored, and the glyphs before the current glyph that a
// list changes are unsafe to break. go-text popped the stack, ended a list
// with an odd value, and emptied the stack on the flag Reset (D73).
func TestKerxFormat1History(t *testing.T) {
	kerx := kerxTable(kerxFormat1([4][2]int{{0, kerxEnd}, {kerxPush, kerxEnd}, {kerxPush, 0}, {kerxReset, 4}},
		51, 41, -31, kerxEnd,
		-20, 15, 7, 3, 5, 9, 11, 13, 17, 19, kerxEnd))
	face := fontWithLayout(t, map[string][]byte{"kerx": kerx})
	testShapedFace(t, face, []shapedText{
		{"ab", []shapedGlyph{{robotoA, 0, 1124, 10, 0}, {robotoB, 1, 1200, 51, 0}}},
		{"abc", []shapedGlyph{{robotoA, 0, 1206, 92, 0}, {robotoB, 1, 1197, 48, 0}, {robotoC, 2, 1072, 0, 0}}},
		{"bc", []shapedGlyph{{robotoB, 0, 1289, 140, 0}, {robotoC, 1, 1072, 0, 0}}},
		{"aaaaaaaaab", []shapedGlyph{
			{robotoA, 0, 1114, 0, 0}, {robotoA, 1, 1114, 0, 0}, {robotoA, 2, 1114, 0, 0}, {robotoA, 3, 1114, 0, 0},
			{robotoA, 4, 1114, 0, 0}, {robotoA, 5, 1114, 0, 0}, {robotoA, 6, 1114, 0, 0}, {robotoA, 7, 1083, -31, 0},
			{robotoA, 8, 1155, 41, 0}, {robotoB, 9, 1200, 51, 0},
		}},
	})

	// The list of b changes x at 0 with the slots that no glyph filled, so
	// each glyph after it is unsafe to break.
	buf := shapeWithin(t, face.Font, "xxxxab")
	for i, info := range buf.Info {
		if got, want := info.Mask&GlyphUnsafeToBreak != 0, i != 0; got != want {
			t.Errorf("xxxxab, glyph %d: got unsafe to break %v, want %v", i, got, want)
		}
	}
}

// TestKerxFormat1Operations shapes aab and aa with a subtable of format 1
// of kerx, which applies at b a list of 600,000 values 0 without an end,
// then a subtable of format 0, which kerns a, a. As in HarfBuzz, each
// action takes one operation, the buffer runs out of its 524,288
// operations in the first subtable, and the second subtable does not
// apply. go-text took no operation for an action, and kerned a, a (D73,
// D97).
func TestKerxFormat1Operations(t *testing.T) {
	values := make([]int, 600000)
	kerx := kerxTable(kerxFormat1([4][2]int{{0, kerxEnd}, {kerxPush, kerxEnd}, {kerxPush, 0}, {0, kerxEnd}}, values...),
		kerxPairs(int(robotoA), int(robotoA), -300))
	face := fontWithLayout(t, map[string][]byte{"kerx": kerx})
	testShapedFace(t, face, []shapedText{
		{"aab", []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoA, 1, 1114, 0, 0}, {robotoB, 2, 1149, 0, 0}}},
		{"aa", []shapedGlyph{{robotoA, 0, 964, 0, 0}, {robotoA, 1, 964, -150, 0}}},
	})
	if buf := shapeWithin(t, face.Font, "aab"); !buf.inError {
		t.Error("aab: the buffer is not in error")
	}
}

// TestKerxFormat1Bounds shapes texts with two subtables of format 1 of
// kerx. In the first, the list of b, 10, 20, has no end. The header of the
// second subtable gives a length without its values, and its list of b is
// 5. As in HarfBuzz 15.1.0, a list ends at the end of its subtable, and
// the list of the last subtable ends at the end of the table (D73).
func TestKerxFormat1Bounds(t *testing.T) {
	entries := [4][2]int{{0, kerxEnd}, {kerxPush, kerxEnd}, {kerxPush, 0}, {0, kerxEnd}}
	last := kerxFormat1(entries, 5, kerxEnd)
	last.length = 12 + len(last.data) - 4
	kerx := kerxTable(kerxFormat1(entries, 10, 20), last)
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), []shapedText{
		{"ab", []shapedGlyph{{robotoA, 0, 1134, 20, 0}, {robotoB, 1, 1164, 15, 0}}},
		{"aab", []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoA, 1, 1134, 20, 0}, {robotoB, 2, 1164, 15, 0}}},
	})
}

// TestKernFormat1Offset shapes ab with a subtable of format 1 of a table
// kern of Apple. It pushes a, and pushes b with a list of actions at the
// byte offset 0x2000, which gives -40 to b and -30 to a. In kern, the bit
// 0x2000 is a bit of the offset. go-text read it as the flag Reset of
// kerx, emptied the stack, and kerned only b (D73).
func TestKernFormat1Offset(t *testing.T) {
	const values = 0x2000
	data := u16s(7, 10, 18, 32, values)
	data = append(data, u16s(int(robotoA), 3)...)
	data = append(data, 4, 5, 6, 0)
	for range 2 {
		data = append(data, 0, 0, 0, 0, 1, 2, 0)
	}
	data = append(data, u16s(18, 0, 18, kerxPush, 18, kerxPush|values)...)
	data = append(data, make([]byte, values-len(data))...)
	data = append(data, u16s(-40, -29)...)
	kern := u32s(0x00010000, 1, uint32(8+len(data)), 0x00010000)
	kern = append(kern, data...)
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kern": kern}), []shapedText{
		{"ab", []shapedGlyph{{robotoA, 0, 1084, -30, 0}, {robotoB, 1, 1109, -40, 0}}},
	})
}

// stateMachine is an extended state table of morx or kerx. lookup is its
// class table. Each state is a row of nClasses entry indexes, and each
// entry is its new state, its flags and its data.
type stateMachine struct {
	lookup   []byte
	nClasses int
	states   [][]int
	entries  [][]int
}

// bytes gives the state table, with one 32-bit field after its header for
// each tail, and the tails after the table. A field is fields[i] OR the
// offset of tails[i] from the start of the state table.
func (m stateMachine) bytes(fields []uint32, tails ...[]byte) []byte {
	var states, entries []byte
	for _, row := range m.states {
		states = append(states, u16s(row...)...)
	}
	for _, e := range m.entries {
		entries = append(entries, u16s(e...)...)
	}
	header := 16 + 4*len(tails)
	offset := header + len(m.lookup)
	out := u32s(uint32(m.nClasses), uint32(header), uint32(offset), uint32(offset+len(states)))
	offset += len(states) + len(entries)
	for i, tail := range tails {
		out = append(out, u32s(fields[i]|uint32(offset))...)
		offset += len(tail)
	}
	out = append(out, m.lookup...)
	out = append(out, states...)
	out = append(out, entries...)
	for _, tail := range tails {
		out = append(out, tail...)
	}
	return out
}

// lookup8 builds a lookup table of format 8, which gives the values to the
// glyphs from first on.
func lookup8(first GID, values ...int) []byte {
	return append(u16s(8, int(first), len(values)), u16s(values...)...)
}

// morxSubtable is a subtable of a chain of morx: its coverage, with the
// type in the low byte, and its data after its header of 12 bytes.
type morxSubtable struct {
	coverage uint32
	data     []byte
}

// morxChain builds a chain of morx with the subtables, whose default flags
// and subtable flags are 1.
func morxChain(subtables ...morxSubtable) []byte {
	var data []byte
	for _, st := range subtables {
		data = append(data, u32s(uint32(12+len(st.data)), st.coverage, 1)...)
		data = append(data, st.data...)
	}
	out := u32s(1, uint32(16+len(data)), 0, uint32(len(subtables)))
	return append(out, data...)
}

// morxTable builds a table morx of version 2 with the chains.
func morxTable(chains ...[]byte) []byte {
	out := u16s(2, 0)
	out = append(out, u32s(uint32(len(chains)))...)
	for _, c := range chains {
		out = append(out, c...)
	}
	return out
}

// The coverages of the subtables of morx and kerx that the tests use.
const (
	morxNoncontextual = 0x20000004
	morxInsertion     = 0x20000005
	kerxFormat4       = 4
	kerxCrossStream   = 0x40000000
	kerxBackwards     = 0x10000000
)

// The flags and the data of the entries of the state machines.
const (
	kerxMark         = 0x8000 // the flag Mark of format 4 of kerx
	insertCurrentOne = 1 << 5 // insert one glyph at the current glyph
	insertNone       = 0xFFFF // the index of no insertion
	kerx4Coordinates = 2 << 30
)

// TestAATInitialGlyphs shapes texts with subtables with a state machine,
// whose class table gives the class 4 to a, and from whose start state the
// class 4 starts no action. As in HarfBuzz 15.1.0, such a subtable applies
// only when the buffer holds a glyph whose class can start an action from
// the start state, and a glyph outside the class table is never one of
// them:
//   - In a subtable of format 1 of kerx, x is outside the class table, and
//     pushes itself and kerns itself by -100. The subtable does not apply.
//   - A subtable of morx inserts b at the end of the text. It does not
//     apply.
//   - In a chain of morx, a noncontextual subtable deletes a. Then an
//     insertion subtable, whose class table holds only c, goes to the
//     state 1 on the deleted glyph, and inserts b after x in the state 1.
//     It applies to the deleted glyph.
//
// go-text collected each glyph of the class table (D73).
func TestAATInitialGlyphs(t *testing.T) {
	kerx1 := stateMachine{
		lookup:   lookup8(robotoA, 4),
		nClasses: 5,
		states:   [][]int{{0, 1, 0, 0, 0}, {0, 1, 0, 0, 0}},
		entries:  [][]int{{0, 0, kerxEnd}, {0, kerxPush, 0}},
	}
	kerx := kerxTable(kerxSubtable{coverage: 1, data: kerx1.bytes([]uint32{0}, u16s(-100, kerxEnd))})
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), []shapedText{
		{"xa", []shapedGlyph{{robotoX, 0, 1015, 0, 0}, {robotoA, 1, 1114, 0, 0}}},
	})

	insertion := stateMachine{
		lookup:   lookup8(robotoA, 4),
		nClasses: 5,
		states:   [][]int{{1, 0, 0, 0, 0}, {1, 0, 0, 0, 0}},
		entries:  [][]int{{0, 0, insertNone, insertNone}, {0, insertCurrentOne, 0, insertNone}},
	}
	morx := morxTable(morxChain(morxSubtable{morxInsertion, insertion.bytes([]uint32{0}, u16s(int(robotoB)))}))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"morx": morx}), []shapedText{
		{"a", []shapedGlyph{{robotoA, 0, 1114, 0, 0}}},
	})

	insertion = stateMachine{
		lookup:   lookup8(robotoC, 4),
		nClasses: 5,
		states:   [][]int{{0, 0, 1, 0, 0}, {0, 2, 1, 0, 0}},
		entries:  [][]int{{0, 0, insertNone, insertNone}, {1, 0, insertNone, insertNone}, {0, insertCurrentOne, 0, insertNone}},
	}
	morx = morxTable(morxChain(
		morxSubtable{morxNoncontextual, lookup8(robotoA, 0xFFFF)},
		morxSubtable{morxInsertion, insertion.bytes([]uint32{0}, u16s(int(robotoB)))},
	))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"morx": morx}), []shapedText{
		{"ax", []shapedGlyph{{robotoX, 0, 1015, 0, 0}, {robotoB, 0, 1149, 0, 0}}},
		{"xa", []shapedGlyph{{robotoX, 0, 1015, 0, 0}}},
	})
}

// kerxMarks builds a table kerx with two subtables. The first is of format
// 1 and cross-stream: a pushes itself and applies the list of values. The
// second is of format 4, with the coverage: a marks itself, and b attaches
// to the marked glyph with the coordinates of the anchors.
func kerxMarks(coverage uint32, values, anchors []int) []byte {
	cross := stateMachine{
		lookup:   lookup8(robotoA, 4, 5),
		nClasses: 6,
		states:   [][]int{{0, 0, 0, 0, 1, 0}, {0, 0, 0, 0, 1, 0}},
		entries:  [][]int{{0, 0, kerxEnd}, {0, kerxPush, 0}},
	}
	marks := stateMachine{
		lookup:   lookup8(robotoA, 4, 5),
		nClasses: 6,
		states:   [][]int{{0, 0, 0, 0, 1, 2}, {0, 0, 0, 0, 1, 2}},
		entries:  [][]int{{0, 0, kerxEnd}, {0, kerxMark, kerxEnd}, {0, 0, 0}},
	}
	return kerxTable(
		kerxSubtable{coverage: kerxCrossStream | 1, data: cross.bytes([]uint32{0}, u16s(append(values, kerxEnd)...))},
		kerxSubtable{coverage: coverage, data: marks.bytes([]uint32{kerx4Coordinates}, u16s(anchors...))},
	)
}

// TestKerxFormat4CrossOffset shapes texts with kerxMarks, which moves each
// a by 100 across the line, and attaches b to the last a with the anchors
// (50, 60) and (10, 20). As in HarfBuzz 15.1.0, the offset of b across the
// line takes the offsets of the a, which are attached to each other by the
// cross-stream subtable. It does so also when the subtable of format 4
// goes backwards, which reverses the buffer. go-text gave b the offset 40
// (D73).
func TestKerxFormat4CrossOffset(t *testing.T) {
	kerx := kerxMarks(kerxFormat4, []int{100}, []int{50, 60, 10, 20})
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), []shapedText{
		{"aab", []shapedGlyph{{robotoA, 0, 1114, 0, 100}, {robotoA, 1, 1114, 0, 200}, {robotoB, 2, 1149, -1074, 240}}},
	})
	kerx = kerxMarks(kerxFormat4|kerxBackwards, []int{100}, []int{50, 60, 10, 20})
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}), []shapedText{
		{"aaba", []shapedGlyph{
			{robotoA, 0, 1114, 0, 100}, {robotoA, 1, 1114, 0, 200},
			{robotoB, 2, 1149, 1189, 380}, {robotoA, 3, 1114, 0, 480},
		}},
	})
}

// TestKerxFormat4Clamp shapes a text with kerxMarks, which moves a across
// the line by 65,537 times 32,767, and attaches b to a with anchors 65,535
// apart. As in HarfBuzz 15.1.0, the offset of b is clamped to the range of
// a position. The glyphs x give the buffer the operations that the actions
// take (D73).
func TestKerxFormat4Clamp(t *testing.T) {
	values := make([]int, 65537)
	for i := range values {
		values[i] = 32767
	}
	kerx := kerxMarks(kerxFormat4, values, []int{0, 32767, 0, -32768})
	buf := shapeWithin(t, fontWithLayout(t, map[string][]byte{"kerx": kerx}).Font, "ab"+strings.Repeat("x", 38))
	if buf.inError {
		t.Fatal("the buffer is in error")
	}
	if got, want := buf.Pos[0].YOffset, Position(65537*32767); got != want {
		t.Errorf("a: got the offset %d, want %d", got, want)
	}
	if got, want := buf.Pos[1].YOffset, Position(math.MaxInt32); got != want {
		t.Errorf("b: got the offset %d, want %d", got, want)
	}
}

// kernFormat1 builds the data of a subtable of format 1 of kern, after its
// header. Its class table gives the classes 4, 5 and 6 to a, b and c, and
// its two states give them the entries 1, 2 and 3. Each entry stays in the
// state 0 and has the flags of its index, which hold the byte offset of a
// list of values from the start of the state table. The values start at
// the byte offset valueTable, which is 48 or more.
func kernFormat1(flags [4]int, valueTable int, values ...int) []byte {
	data := u16s(7, 10, 18, 32, valueTable)
	data = append(data, u16s(int(robotoA), 3)...)
	data = append(data, 4, 5, 6, 0)
	for range 2 {
		data = append(data, 0, 0, 0, 0, 1, 2, 3)
	}
	for _, f := range flags {
		data = append(data, u16s(18, f)...)
	}
	data = append(data, make([]byte, valueTable-len(data))...)
	return append(data, u16s(values...)...)
}

// kernTable builds a table kern of Apple of version 1 with subtables of
// format 1, each the data after its header.
func kernTable(subtables ...[]byte) []byte {
	out := u32s(0x00010000, uint32(len(subtables)))
	for _, data := range subtables {
		out = append(out, u32s(uint32(8+len(data)), 0x00010000)...)
		out = append(out, data...)
	}
	return out
}

// TestKernFormat1ValueTable shapes texts with subtables of format 1 of
// kern, where a pushes itself:
//   - b has a list of values at the byte offset 2, before the value table,
//     and c has the list -40, -29 at the start of the value table. As in
//     HarfBuzz 15.1.0, b empties the stack. go-text gave b no list, and c
//     kerned a in abc.
//   - b and c push themselves too, and the list of c is 11, 20, 30. As in
//     HarfBuzz, the values for the three glyphs of the stack are in the
//     subtable, and the first value ends the list. go-text kept the values
//     up to the first odd value, and did not apply the list.
//   - The same subtable with the list 20, 30, which ends with the
//     subtable, before a second subtable with no list. As in HarfBuzz, the
//     list does not apply to a stack of three glyphs, and applies to a
//     stack of two. go-text read the values of the second subtable after
//     the end of the table, and refused the table.
//
// (D73)
func TestKernFormat1ValueTable(t *testing.T) {
	kern := kernTable(kernFormat1([4]int{0, kerxPush, 2, 48}, 48, -40, -29))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kern": kern}), []shapedText{
		{"abc", []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoB, 1, 1149, 0, 0}, {robotoC, 2, 1072, 0, 0}}},
		{"ac", []shapedGlyph{{robotoA, 0, 1074, -40, 0}, {robotoC, 1, 1072, 0, 0}}},
	})

	push := [4]int{0, kerxPush, kerxPush, kerxPush | 48}
	kern = kernTable(kernFormat1(push, 48, 11, 20, 30))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kern": kern}), []shapedText{
		{"abc", []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoB, 1, 1149, 0, 0}, {robotoC, 2, 1082, 10, 0}}},
	})

	kern = kernTable(kernFormat1(push, 48, 20, 30), kernFormat1([4]int{}, 48))
	testShapedFace(t, fontWithLayout(t, map[string][]byte{"kern": kern}), []shapedText{
		{"abc", []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoB, 1, 1149, 0, 0}, {robotoC, 2, 1072, 0, 0}}},
		{"ac", []shapedGlyph{{robotoA, 0, 1144, 30, 0}, {robotoC, 1, 1092, 20, 0}}},
	})
}

// kernPairs builds a subtable of format 0 of a table kern of Microsoft,
// with the length and the pairs, each a left glyph, a right glyph and a
// value.
func kernPairs(length int, pairs ...int) []byte {
	n := len(pairs) / 3
	out := u16s(0, length, 1, n, 6*n, 0, 0)
	return append(out, u16s(pairs...)...)
}

// kernOT builds a table kern of Microsoft with the subtables.
func kernOT(subtables ...[]byte) []byte {
	out := u16s(0, len(subtables))
	for _, st := range subtables {
		out = append(out, st...)
	}
	return out
}

// withLength gives the chain of morx or the subtable with the length at
// the byte offset at in place of its length.
func withLength(data []byte, at, length int) []byte {
	out := slices.Clone(data)
	copy(out[at:], u32s(uint32(length)))
	return out
}

// withCoverage gives the chain of morx with a glyph coverage of version 3
// after its subtables, which holds the offsets and room for one bitfield,
// and with its length.
func withCoverage(chain []byte, offsets ...uint32) []byte {
	out := append(slices.Clone(chain), u32s(offsets...)...)
	out = append(out, make([]byte, 4096)...)
	return withLength(out, 4, len(out))
}

// TestAATTableErrors shapes ab or ac with tables kern, kerx and morx with
// an error, or with a subtable that HarfBuzz 15.1.0 does not apply. A
// subtable kerns a, b by -300, or replaces a with b or c with x. As in
// HarfBuzz:
//   - A table kern or kerx is refused when a subtable that is not the last
//     has more pairs than its length holds. The length of the last subtable
//     is ignored.
//   - A table kerx or morx is refused when a subtable or a chain after the
//     first one is longer than the table, when a chain or a subtable is
//     shorter than its header, or when the glyph coverage of version 3 is
//     not in the table. go-text kept the subtables or the chains before
//     the error.
//   - A subtable of an unknown format or type does not apply, and the
//     subtables after it apply. go-text refused it and the subtables after
//     it.
//   - The next chain of morx starts at the length of a chain. go-text read
//     it after the subtables.
//
// (D73)
func TestAATTableErrors(t *testing.T) {
	kerned := []shapedText{{"ab", []shapedGlyph{{robotoA, 0, 964, 0, 0}, {robotoB, 1, 999, -150, 0}}}}
	plain := []shapedText{{"ab", []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoB, 1, 1149, 0, 0}}}}
	for _, test := range []struct {
		kern []byte
		want []shapedText
	}{
		{kernOT(kernPairs(14, int(robotoA), int(robotoB), -300)), kerned},
		{kernOT(u16s(0, 14, 1, 2, 12, 1, 0), kernPairs(20, int(robotoA), int(robotoB), -300)), plain},
		{kernOT(kernPairs(0), kernPairs(20, int(robotoA), int(robotoB), -300)), plain},
		{kernOT(u16s(0, 8, 5<<8|1, 0), kernPairs(20, int(robotoA), int(robotoB), -300)), kerned},
	} {
		testShapedFace(t, fontWithLayout(t, map[string][]byte{"kern": test.kern}), test.want)
	}

	pairs := kerxPairs(int(robotoA), int(robotoB), -300)
	version3 := func(kerx []byte, offsets ...uint32) []byte {
		out := append(slices.Clone(kerx), u32s(offsets...)...)
		out = append(out, make([]byte, 4096)...)
		out[1] = 3
		return out
	}
	for _, test := range []struct {
		kerx []byte
		want []shapedText
	}{
		{kerxTable(pairs, kerxSubtable{data: u32s(0, 0, 0, 0), length: 1000}), plain},
		{kerxTable(kerxSubtable{coverage: 3, data: u32s(0)}, pairs), kerned},
		{kerxTable(kerxSubtable{data: u32s(1, 6, 0, 0)}, pairs), plain},
		{version3(kerxTable(kerxPairs(), pairs), 0, 8), kerned},
		{version3(kerxTable(kerxPairs(), pairs), 0, 4096), plain},
	} {
		testShapedFace(t, fontWithLayout(t, map[string][]byte{"kerx": test.kerx}), test.want)
	}

	ab := morxChain(morxSubtable{morxNoncontextual, lookup8(robotoA, int(robotoB))})
	cx := morxChain(morxSubtable{morxNoncontextual, lookup8(robotoC, int(robotoX))})
	padded := withLength(append(slices.Clone(ab), 0, 0, 0, 0), 4, len(ab)+4)
	for _, test := range []struct {
		version int
		chains  [][]byte
		want    []shapedGlyph
	}{
		{2, [][]byte{ab, withLength(cx, 4, 1000)}, []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoC, 1, 1072, 0, 0}}},
		{2, [][]byte{cx, withLength(ab, 4, 8)}, []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoC, 1, 1072, 0, 0}}},
		{2, [][]byte{morxChain(morxSubtable{0x20000003, u32s(0)}, morxSubtable{morxNoncontextual, lookup8(robotoA, int(robotoB))})},
			[]shapedGlyph{{robotoB, 0, 1149, 0, 0}, {robotoC, 1, 1072, 0, 0}}},
		{2, [][]byte{padded, cx}, []shapedGlyph{{robotoB, 0, 1149, 0, 0}, {robotoX, 1, 1015, 0, 0}}},
		{3, [][]byte{withCoverage(ab, 0), withCoverage(cx, 4)}, []shapedGlyph{{robotoB, 0, 1149, 0, 0}, {robotoX, 1, 1015, 0, 0}}},
		{3, [][]byte{withCoverage(ab, 0), withCoverage(cx, 4096)}, []shapedGlyph{{robotoA, 0, 1114, 0, 0}, {robotoC, 1, 1072, 0, 0}}},
	} {
		morx := morxTable(test.chains...)
		morx[1] = byte(test.version)
		testShapedFace(t, fontWithLayout(t, map[string][]byte{"morx": morx}), []shapedText{{"ac", test.want}})
	}
}
