// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package harfbuzz

import (
	"testing"

	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// okage: the tests of D83.

// TestMappingCache makes sure that an empty cache holds no glyph, and that
// the cache gives the last value of each glyph that it holds.
func TestMappingCache(t *testing.T) {
	var mc mappingCache
	mc.clear()
	for g := range 1 << 16 {
		if _, ok := mc.get(gID(g)); ok {
			t.Fatalf("an empty cache holds glyph %d", g)
		}
	}
	for g := range 1 << 16 {
		mc.set(gID(g), uint16(g*7))
		if v, ok := mc.get(gID(g)); !ok || v != uint16(g*7) {
			t.Fatalf("get(%d) = %d, %v, want %d, true", g, v, ok, uint16(g*7))
		}
		if _, ok := mc.get(gID(g) ^ 0x100); ok {
			t.Fatalf("the cache holds glyph %d after set(%d)", g^0x100, g)
		}
	}
}

// TestPairPosCache compares the coverage index and the classes of a
// pairPosCache with the lookups in the tables, for each PairPos subtable
// of format 2 of some fonts and for each glyph, twice, so that the second
// pass reads the cache.
func TestPairPosCache(t *testing.T) {
	subtables := 0
	for _, name := range []string{"common/Roboto-BoldItalic.ttf", "common/DejaVuSans.ttf", "common/mplus-1p-regular.ttf", "common/SourceSans-VF.ttf"} {
		ft := openFontFileTT(t, name)
		for _, lookup := range ft.GPOS.Lookups {
			for _, table := range lookup.Subtables {
				pair, ok := table.(tables.PairPos)
				if !ok {
					continue
				}
				inner, ok := pair.Data.(tables.PairPosData2)
				if !ok {
					continue
				}
				subtables++
				pc := newPairPosCache()
				for pass := range 2 {
					for g := range 1 << 16 {
						gotIndex, gotOK := pc.index(pair.Cov(), gID(g))
						wantIndex, wantOK := pair.Cov().Index(gID(g))
						if gotOK != wantOK || (gotOK && gotIndex != wantIndex) {
							t.Fatalf("%s, pass %d: index(%d) = %d, %v, want %d, %v", name, pass, g, gotIndex, gotOK, wantIndex, wantOK)
						}
						h := gID(g*31 + 7)
						got1, got2 := pc.classes(&inner, gID(g), h)
						want1, want2 := (*pairPosCache)(nil).classes(&inner, gID(g), h)
						if got1 != want1 || got2 != want2 {
							t.Fatalf("%s, pass %d: classes(%d, %d) = %d, %d, want %d, %d", name, pass, g, h, got1, got2, want1, want2)
						}
					}
				}
			}
		}
	}
	if subtables == 0 {
		t.Fatal("found no PairPos subtable of format 2")
	}
}

// TestGlyphPropsCache compares Font.glyphProps with GDEF.GlyphProps for
// each glyph of some fonts, twice, so that the second pass reads the
// cache.
func TestGlyphPropsCache(t *testing.T) {
	for _, name := range []string{"common/Roboto-BoldItalic.ttf", "common/mplus-1p-regular.ttf", "common/NotoSansArabic.ttf", "common/DejaVuSans.ttf"} {
		ft := openFontFileTT(t, name)
		if ft.GDEF.GlyphClassDef == nil {
			t.Fatalf("%s has no glyph classes", name)
		}
		f := NewFont(&font.Face{Font: ft})
		for pass := range 2 {
			for g := range 1 << 16 {
				if got, want := f.glyphProps(gID(g)), ft.GDEF.GlyphProps(gID(g)); got != want {
					t.Fatalf("%s, pass %d: glyphProps(%d) = %d, want %d", name, pass, g, got, want)
				}
			}
		}
	}
}
