// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package font

// okage: this file is not in go-text. It tests the changes of D67 to
// sanitizeLangSys and newCmap4.

import (
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
