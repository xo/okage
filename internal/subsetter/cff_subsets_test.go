// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"slices"
	"testing"

	"github.com/xo/okage/typesetting/font/cff"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// This file ports the parts of glyph_metrics, glyph_outlines_ttf_parser and
// glyph_outlines_skrifa of subsetter tests/src/main.rs that read the CFF
// fonts, for TestSubsets. Upstream reads the fonts with ttf-parser and
// skrifa. The port reads them with typesetting (D52), which draws a glyph
// with the FontMatrix of its font DICT.

// cffGlyphTables holds the tables of a CFF font that the tests compare.
type cffGlyphTables struct {
	numGlyphs uint16
	hmtx      tables.Hmtx
	cff       *cff.CFF
}

// isCFFFont reports whether the first font of a font file has a CFF table.
func isCFFFont(t *testing.T, data []byte) bool {
	t.Helper()
	f, err := parse(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	return f.hasTable(tagCFF)
}

// testNumGlyphs returns the number of glyphs of the maxp table of the first
// font of a font file, as Face::number_of_glyphs of ttf-parser.
func testNumGlyphs(t *testing.T, data []byte) uint16 {
	t.Helper()
	f, err := parse(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := f.table(tagMAXP)
	if !ok {
		t.Fatal("the font has no table maxp")
	}
	maxp, _, err := tables.ParseMaxp(b)
	if err != nil {
		t.Fatal(err)
	}
	return maxp.NumGlyphs
}

// readCFFGlyphTables reads the tables of the first font of a CFF font file
// with typesetting.
func readCFFGlyphTables(t *testing.T, data []byte) cffGlyphTables {
	t.Helper()
	f, err := parse(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw := func(tag Tag) []byte {
		b, ok := f.table(tag)
		if !ok {
			t.Fatalf("the font has no table %v", tag)
		}
		return b
	}
	maxp, _, err := tables.ParseMaxp(raw(tagMAXP))
	if err != nil {
		t.Fatal(err)
	}
	hhea, _, err := tables.ParseHhea(raw(tagHHEA))
	if err != nil {
		t.Fatal(err)
	}
	n := int(maxp.NumGlyphs)
	metrics := int(hhea.NumOfLongMetrics)
	hmtx, _, err := tables.ParseHmtx(raw(tagHMTX), metrics, max(0, n-metrics))
	if err != nil {
		t.Fatal(err)
	}
	font, err := cff.Parse(raw(tagCFF))
	if err != nil {
		t.Fatal(err)
	}
	return cffGlyphTables{numGlyphs: maxp.NumGlyphs, hmtx: hmtx, cff: font}
}

// glyphMetricsCFF is glyph_metrics in subsetter tests/src/main.rs for a CFF
// font. It also checks that each glyph has an identity CID-to-GID mapping,
// in the charset that rewriteCharset writes.
func glyphMetricsCFF(t *testing.T, ctx testContext) {
	t.Helper()
	old := readCFFGlyphTables(t, ctx.font)
	sub := readCFFGlyphTables(t, ctx.subset)

	for _, glyph := range ctx.gids {
		if glyph >= old.numGlyphs {
			continue
		}
		mapped, ok := ctx.mapper.Get(glyph)
		if !ok {
			t.Fatalf("the glyph %d is not in the remapper", glyph)
		}

		_, a, errA := old.cff.LoadGlyph(tables.GlyphID(glyph))
		_, b, errB := sub.cff.LoadGlyph(tables.GlyphID(mapped))
		if (errA == nil) != (errB == nil) || a != b {
			t.Errorf("metric glyph bounding box didn't match for glyph %d: %v and %v", glyph, a, b)
		}
		if a, b := old.hmtx.SideBearing(tables.GlyphID(glyph)), sub.hmtx.SideBearing(tables.GlyphID(mapped)); a != b {
			t.Errorf("metric glyph horizontal side bearing didn't match for glyph %d: %d and %d", glyph, a, b)
		}
		if a, b := old.hmtx.Advance(tables.GlyphID(glyph)), sub.hmtx.Advance(tables.GlyphID(mapped)); a != b {
			t.Errorf("metric glyph horizontal advance didn't match for glyph %d: %d and %d", glyph, a, b)
		}
	}

	// Assert that each glyph has an identity CID-to-GID mapping.
	if cid, ok := subsetCharsetCIDs(t, ctx.subset); !ok || len(cid) != int(ctx.mapper.NumGids()) {
		t.Errorf("the charset of the subset has %d CIDs, %v", len(cid), ok)
	} else {
		for gid, c := range cid {
			if int(c) != gid {
				t.Errorf("the glyph %d has the CID %d", gid, c)
				break
			}
		}
	}
}

// subsetCharsetCIDs reads the charset of the CFF table of a subset, which
// has the format 0 or 2, and returns the CID of each glyph.
func subsetCharsetCIDs(t *testing.T, subset []byte) ([]uint16, bool) {
	t.Helper()
	f, err := parse(subset, 0)
	if err != nil {
		t.Fatal(err)
	}
	table, _ := f.table(tagCFF)
	r := newReader(table)
	r.jump(4)
	if !skipIndex(&r) {
		return nil, false
	}
	top, ok := parseTopDictIndex(&r)
	if !ok || !top.hasCharset || !top.hasCharStrings {
		return nil, false
	}
	cr := newReaderAtOffset(table, top.charStrings)
	charStrings, ok := parseIndex(&cr)
	if !ok {
		return nil, false
	}
	n := int(charStrings.len())
	cids := []uint16{0}
	r = newReaderAtOffset(table, top.charset)
	format, ok := r.readU8()
	if !ok {
		return nil, false
	}
	switch format {
	case 0:
		for len(cids) < n {
			v, ok := r.readU16()
			if !ok {
				return nil, false
			}
			cids = append(cids, v)
		}
	case 2:
		for len(cids) < n {
			first, ok := r.readU16()
			if !ok {
				return nil, false
			}
			left, ok := r.readU16()
			if !ok {
				return nil, false
			}
			for i := range int(left) + 1 {
				cids = append(cids, first+uint16(i))
			}
		}
	default:
		return nil, false
	}
	return cids, len(cids) == n
}

// glyphOutlinesCFF is glyph_outlines_ttf_parser and glyph_outlines_skrifa
// in subsetter tests/src/main.rs for a CFF font. Each glyph that
// typesetting draws in the font has the same outline in the subset.
func glyphOutlinesCFF(t *testing.T, ctx testContext) {
	t.Helper()
	old := readCFFGlyphTables(t, ctx.font)
	sub := readCFFGlyphTables(t, ctx.subset)

	for glyph := range old.numGlyphs {
		if !slices.Contains(ctx.gids, glyph) {
			continue
		}
		newGlyph, ok := ctx.mapper.Get(glyph)
		if !ok {
			t.Fatalf("the glyph %d is not in the remapper", glyph)
		}
		a, _, err := old.cff.LoadGlyph(tables.GlyphID(glyph))
		if err != nil {
			continue
		}
		b, _, err := sub.cff.LoadGlyph(tables.GlyphID(newGlyph))
		if err != nil {
			t.Errorf("failed to find glyph %d in new face: %v", glyph, err)
			continue
		}
		if !slices.Equal(a, b) {
			t.Errorf("glyph %d drawn with typesetting didn't match.", glyph)
		}
	}
}
