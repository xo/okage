// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package harfbuzz

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"fmt"
	"hash/fnv"
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
			propagateAttachmentOffsets(pos, i, LeftToRight)
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

// TestDropOutputWithoutOperations shapes xaaa with gsubNestedRecords and
// 20,000 records. Each a takes about 40,000 operations, so the buffer has
// no operations left in the middle of the lookup, at the second a. The
// output of the lookup has its own array from x on, because z, y is longer
// than x. HarfBuzz 15.1.0 then drops the output, and keeps the array of the
// input, where each move back of the lookup wrote a glyph: x, b, a, a.
// go-text kept the output, z, y, a, a, a.
func TestDropOutputWithoutOperations(t *testing.T) {
	face := fontWithLayout(t, map[string][]byte{"GSUB": gsubNestedRecords(t, 20000)})
	if got := len(face.GSUB.Lookups); got != 2 {
		t.Fatalf("got %d lookups of GSUB, want 2", got)
	}
	testShapedFace(t, face, []shapedText{
		{"xaaa", []shapedGlyph{
			{robotoX, 0, 1015, 0, 0}, {robotoB, 1, 1149, 0, 0}, {robotoA, 2, 1114, 0, 0}, {robotoA, 3, 1114, 0, 0},
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
	noncontextual := u16s(8, int(robotoA), 1, int(robotoB)) // a lookup of format 8

	length := 16 + count*(12+len(insertion)) + 12 + len(noncontextual)
	out := u16s(2, 0)
	out = append(out, u32s(1, 1, uint32(length), 0, uint32(count+1))...) // one chain
	for range count {
		// The length, the coverage for all the directions and the type,
		// and the flags.
		out = append(out, u32s(uint32(12+len(insertion)), 0x20000005, 1)...)
		out = append(out, insertion...)
	}
	out = append(out, u32s(uint32(12+len(noncontextual)), 0x20000004, 1)...)
	return append(out, noncontextual...)
}

// TestMorxSubtableOperations shapes a with morxInsertions and three
// insertion subtables, which make 32, 1,024, then 32,768 glyphs a. The
// buffer has 65,536 operations, and the insertions take 33,792. As in
// HarfBuzz 15.1.0, each subtable also takes the length of the buffer in
// operations, so the noncontextual subtable takes 32,768. The buffer is
// then in error, and the subtable does not apply. go-text took no
// operation for a subtable, and gave 32,768 glyphs b.
func TestMorxSubtableOperations(t *testing.T) {
	face := fontWithLayout(t, map[string][]byte{"morx": morxInsertions(3)})
	if got := len(face.Morx); got != 1 || len(face.Morx[0].Subtables) != 4 {
		t.Fatalf("got %d chains of morx, want 1 with 4 subtables", got)
	}
	testFontHashes(t, face.Font, []shapedHash{{"a", 32768, 0x9458d2032c2f2325}})
}

// gsubIndicRules builds a table GSUB for the script dev2 of
// NotoSansDevanagari-Regular.ttf, from the glyphs of ka, kha and the virama.
// The feature pres replaces ka with kha. The feature blwf has a context of
// format 1 for the virama, with 32,000 rules that match no text. The rules
// share their bytes.
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
	return layoutTable(t, []string{"DFLT", "dev2"}, []layoutFeature{{"blwf", []int{1}}, {"pres", []int{0}}},
		[][]byte{lookupTable(t, 1, 0, single), lookupTable(t, 5, 0, context)})
}

// TestIndicWouldSubstituteOperations shapes four times ka with
// gsubIndicRules. To find the position of each consonant, the Indic shaper
// asks whether blwf substitutes the virama and the consonant. As in
// HarfBuzz 15.1.0, the question takes an operation for each rule of the
// rule set of the virama. At the third consonant, the buffer has no
// operations left, and it is in error, so pres does not apply. go-text took
// no operation, and pres replaced each ka with kha.
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
	testShapedFace(t, face, []shapedText{{"कककक", []shapedGlyph{
		{ka, 0, 762, 0, 0}, {ka, 1, 762, 0, 0}, {ka, 2, 762, 0, 0}, {ka, 3, 762, 0, 0},
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
// length.
type kerxSubtable struct {
	coverage uint32
	data     []byte
	length   int
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
		out = append(out, u32s(uint32(length), st.coverage, 0)...)
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
// of kerx, which applies at b a list of 100,000 values 0 without an end,
// then a subtable of format 0, which kerns a, a. As in HarfBuzz 15.1.0,
// each action takes one operation, the buffer runs out of operations in
// the first subtable, and the second subtable does not apply. go-text
// took no operation for an action, and kerned a, a (D73).
func TestKerxFormat1Operations(t *testing.T) {
	values := make([]int, 100000)
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
