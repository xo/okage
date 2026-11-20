// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package font

// okage: this file is not in go-text. It tests the changes of D67 to
// sanitizeLangSys and newCmap4, and the changes of D73 to the anchors of a
// MarkArray and to the class counts of a PairPos of format 2.

import (
	"os"
	"slices"
	"testing"

	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// TestSanitizeLangSysFeatureIndices checks that a language system keeps
// only the feature indexes that are in the feature list, so that harfbuzz
// cannot index the list out of range (D67).
func TestSanitizeLangSysFeatureIndices(t *testing.T) {
	for _, test := range []struct {
		in, want []uint16
	}{
		{nil, nil},
		{[]uint16{0, 1}, []uint16{0, 1}},
		{[]uint16{2, 0, 5, 1, 0xFFFF}, []uint16{0, 1}},
		{[]uint16{3}, []uint16{}},
	} {
		langSys := tables.LangSys{RequiredFeatureIndex: 7, FeatureIndices: slices.Clone(test.in)}
		sanitizeLangSys(&langSys, 2)
		if !slices.Equal(langSys.FeatureIndices, test.want) {
			t.Errorf("the indexes %v give %v, want %v", test.in, langSys.FeatureIndices, test.want)
		}
		if langSys.RequiredFeatureIndex != 0xFFFF {
			t.Errorf("the required feature is %d, want 0xFFFF", langSys.RequiredFeatureIndex)
		}
	}
}

// TestCmap4FullSegment checks that a segment of format 4 from 0 to 0xFFFF
// with a glyph array maps each code, and that a segment that ends before it
// starts gives an error (D67).
func TestCmap4FullSegment(t *testing.T) {
	array := make([]byte, 2*0x10000)
	array[2*79+1] = 5
	cm, err := newCmap4(tables.CmapSubtable4{
		EndCode:        []uint16{0xFFFF},
		StartCode:      []uint16{0},
		IdDelta:        []uint16{0},
		IdRangeOffsets: []uint16{2},
		GlyphIDArray:   array,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gid, ok := cm.Lookup(79); !ok || gid != 5 {
		t.Errorf("the code 79 gives the glyph %d, %v, want 5", gid, ok)
	}
	it := cm.Iter()
	n := 0
	for it.Next() {
		it.Char()
		n++
	}
	if n != 0x10000 {
		t.Errorf("the iterator gives %d codes, want %d", n, 0x10000)
	}
	if _, err := newCmap4(tables.CmapSubtable4{
		EndCode:        []uint16{5},
		StartCode:      []uint16{10},
		IdDelta:        []uint16{0},
		IdRangeOffsets: []uint16{2},
		GlyphIDArray:   array,
	}); err == nil {
		t.Error("a segment that ends before it starts gives no error")
	}
}

// TestMarkArrayNullAnchor checks that a mark record with a null offset has
// an anchor at (0, 0), as the Null anchor of HarfBuzz (D73).
func TestMarkArrayNullAnchor(t *testing.T) {
	src := []byte{
		0, 2, // two mark records
		0, 1, 0, 0, // the class 1 and a null offset
		0, 0, 0, 10, // the class 0 and the anchor at 10
		0, 1, 0, 7, 0xFF, 0xFE, // the anchor of format 1 at (7, -2)
	}
	marks, _, err := tables.ParseMarkArray(src)
	if err != nil {
		t.Fatal(err)
	}
	want := [][2]int16{{0, 0}, {7, -2}}
	if len(marks.MarkAnchors) != len(want) {
		t.Fatalf("got %d anchors, want %d", len(marks.MarkAnchors), len(want))
	}
	for i, anchor := range marks.MarkAnchors {
		got, ok := anchor.(tables.AnchorFormat1)
		if !ok || got.XCoordinate != want[i][0] || got.YCoordinate != want[i][1] {
			t.Errorf("anchor %d: got %+v, want an anchor of format 1 at %v", i, anchor, want[i])
		}
	}
}

// TestNotoSansVariableGPOS loads NotoSans-Regular_var.ttf, the same file
// as NotoSans_variable.ttf of krilla. Six mark records of its lookup 35, a
// MarkLigPos, have a null anchor offset, and go-text dropped the whole
// table GPOS of the font (D73).
func TestNotoSansVariableGPOS(t *testing.T) {
	file, err := os.Open("../../internal/subsetter/testdata/NotoSans-Regular_var.ttf")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	face, err := ParseTTF(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(face.GPOS.Lookups); got != 40 {
		t.Fatalf("got %d lookups of GPOS, want 40", got)
	}
	subtables := face.GPOS.Lookups[35].Subtables
	if len(subtables) != 1 {
		t.Fatalf("lookup 35 has %d subtables, want 1", len(subtables))
	}
	lig, ok := subtables[0].(tables.MarkLigPos)
	if !ok {
		t.Fatalf("lookup 35 is a %T, want a MarkLigPos", subtables[0])
	}
	if got := len(lig.MarkArray.MarkAnchors); got != 281 {
		t.Errorf("got %d mark anchors, want 281", got)
	}
}

// TestPairPosClassCounts checks that the class counts of a PairPos of
// format 2 can be larger than the classes of its class definitions, as in
// HarfBuzz, and that the records of the counts must be in the subtable.
// Lookup 74 of Amiri-Regular.ttf of perf_reference has 80 classes for the
// first glyph and 79 in ClassDef1, and go-text dropped the whole table GPOS
// (D73).
func TestPairPosClassCounts(t *testing.T) {
	src := []byte{
		0, 2, // format 2
		0, 28, // the coverage
		0, 4, 0, 0, // an XAdvance for the first glyph
		0, 34, 0, 42, // ClassDef1 and ClassDef2
		0, 3, 0, 2, // three classes for the first glyph, but ClassDef1 has two
		0, 0, 0, 0, 0, 0, 0xFE, 0x0C, 0, 0, 0, 0, // the records, with -500 for the classes 1 and 1
		0, 1, 0, 1, 0, 5, // the coverage of the glyph 5
		0, 1, 0, 5, 0, 1, 0, 1, // ClassDef1: the glyph 5 has the class 1
		0, 1, 0, 6, 0, 1, 0, 1, // ClassDef2: the glyph 6 has the class 1
	}
	pp, _, err := tables.ParsePairPos(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := pp.Sanitize(); err != nil {
		t.Fatalf("the counts larger than the classes give the error %q", err)
	}
	data := pp.Data.(tables.PairPosData2)
	if data.Class1Count() != 3 || data.Class2Count() != 2 {
		t.Errorf("got the counts %d and %d, want 3 and 2", data.Class1Count(), data.Class2Count())
	}
	if got := data.Record(1, 1).ValueRecord1.XAdvance; got != -500 {
		t.Errorf("got the advance %d, want -500", got)
	}

	src[15] = 10 // the records of ten classes for the second glyph are not in the subtable
	pp, _, err = tables.ParsePairPos(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := pp.Sanitize(); err == nil {
		t.Error("the records outside the subtable give no error")
	}
}
