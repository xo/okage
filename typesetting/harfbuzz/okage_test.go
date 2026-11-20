// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package harfbuzz

import (
	"fmt"
	"hash/fnv"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/xo/okage/typesetting/font"
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
	face := openFontFile(t, path)
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
// 15.1.0, stops the shaping.
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
	if depth != 10 || c.buffer.maxOps != -1 {
		t.Errorf("got %d nested lookups and %d operations left, want 10 and -1", depth, c.buffer.maxOps)
	}

	c.buffer.maxOps, depth = 1000, 0
	c.recurse(0)
	if depth != 64 || c.buffer.maxOps >= 0 {
		t.Errorf("got %d nested lookups and %d operations left, want 64 and less than 0", depth, c.buffer.maxOps)
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

	// Without operations left, the merge does not extend.
	b = newBuffer([]int{0, 1, 1, 1, 2}, nil)
	b.maxOps = -1
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
