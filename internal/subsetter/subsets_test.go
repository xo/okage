// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// This file ports the subset tests of subsetter tests/src/main.rs and
// tests/src/subsets.rs. tests/data/subsets.tests is copied into testdata,
// byte for byte. Each line gives a font and a set of glyphs. The tests
// subset the font, and check that each glyph of the set keeps its metrics
// and its outline.
//
// Upstream reads the fonts with ttf-parser and skrifa. The port reads the
// tables with typesetting (D52), and compares the outlines as the glyf
// table holds them: the points of a simple glyph, and the components of a
// composite glyph with their new glyph IDs.
//
// A line whose font is not in testdata is skipped. The CFF fonts come with
// unit 3 of phase 13.

// testContext is TestContext in subsetter tests/src/main.rs.
type testContext struct {
	font   []byte
	subset []byte
	mapper *GlyphRemapper
	gids   []uint16
}

// TestSubsets runs glyph_metrics and the outline tests of subsetter
// tests/src/subsets.rs for each line of testdata/subsets.tests.
func TestSubsets(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "subsets.tests"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		fontFile, gids, _ := strings.Cut(line, ";")
		if _, err := os.Stat(filepath.Join("testdata", fontFile)); err != nil {
			continue
		}
		n++
		t.Run(line, func(t *testing.T) {
			if testing.Short() && gids == "*" {
				t.Skip("every glyph of the font")
			}
			ctx := getTestContext(t, fontFile, gids)
			glyphMetrics(t, ctx)
			glyphOutlines(t, ctx)
		})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Error("no line of subsets.tests has a font in testdata")
	}
}

// getTestContext is get_test_context in subsetter tests/src/main.rs.
func getTestContext(t *testing.T, fontFile, gids string) testContext {
	t.Helper()
	data := readTestFont(t, fontFile)
	old := readGlyphTables(t, data)
	glyphs := parseGids(gids, old.numGlyphs)
	glyphRemapper := NewGlyphRemapperFromGlyphs(glyphs)
	subset, err := Subset(data, 0, glyphRemapper)
	if err != nil {
		t.Fatal(err)
	}
	return testContext{font: data, subset: subset, mapper: glyphRemapper, gids: glyphs}
}

// parseGids is parse_gids in subsetter tests/src/main.rs.
func parseGids(gids string, maxGlyph uint16) []uint16 {
	if gids == "*" {
		out := make([]uint16, maxGlyph)
		for i := range out {
			out[i] = uint16(i)
		}
		return out
	}

	var out []uint16
	for el := range strings.SplitSeq(gids, ",") {
		if el == "" {
			continue
		}
		if first, second, ok := strings.Cut(el, "-"); ok {
			a, _ := strconv.ParseUint(first, 10, 16)
			b, _ := strconv.ParseUint(second, 10, 16)
			for g := a; g <= b; g++ {
				out = append(out, uint16(g))
			}
		} else {
			g, _ := strconv.ParseUint(el, 10, 16)
			out = append(out, uint16(g))
		}
	}
	return out
}

// glyphMetrics is glyph_metrics in subsetter tests/src/main.rs.
func glyphMetrics(t *testing.T, ctx testContext) {
	t.Helper()
	old := readGlyphTables(t, ctx.font)
	sub := readGlyphTables(t, ctx.subset)

	for _, glyph := range ctx.gids {
		if glyph >= old.numGlyphs {
			continue
		}
		mapped, ok := ctx.mapper.Get(glyph)
		if !ok {
			t.Fatalf("the glyph %d is not in the remapper", glyph)
		}

		if a, b := old.bbox(glyph), sub.bbox(mapped); a != b {
			t.Errorf("metric glyph bounding box didn't match for glyph %d: %v and %v", glyph, a, b)
		}
		if a, b := old.hmtx.SideBearing(tables.GlyphID(glyph)), sub.hmtx.SideBearing(tables.GlyphID(mapped)); a != b {
			t.Errorf("metric glyph horizontal side bearing didn't match for glyph %d: %d and %d", glyph, a, b)
		}
		if a, b := old.hmtx.Advance(tables.GlyphID(glyph)), sub.hmtx.Advance(tables.GlyphID(mapped)); a != b {
			t.Errorf("metric glyph horizontal advance didn't match for glyph %d: %d and %d", glyph, a, b)
		}
	}
}

// glyphOutlines is glyph_outlines_ttf_parser and glyph_outlines_skrifa in
// subsetter tests/src/main.rs.
func glyphOutlines(t *testing.T, ctx testContext) {
	t.Helper()
	old := readGlyphTables(t, ctx.font)
	sub := readGlyphTables(t, ctx.subset)
	// The subset holds the components of its composite glyphs, which
	// Subset adds to a copy of the remapper.
	face, err := parse(ctx.font, 0)
	if err != nil {
		t.Fatal(err)
	}
	withComponents := ctx.mapper.Clone()
	if err := closure(face, withComponents); err != nil {
		t.Fatal(err)
	}

	for glyph := range old.numGlyphs {
		if !slices.Contains(ctx.gids, glyph) {
			continue
		}
		newGlyph, ok := ctx.mapper.Get(glyph)
		if !ok {
			t.Fatalf("the glyph %d is not in the remapper", glyph)
		}
		a, b := old.glyf[glyph], sub.glyf[newGlyph]
		if !sameOutline(a, b, withComponents) {
			t.Errorf("glyph %d didn't match.", glyph)
		}
	}
}

// sameOutline reports whether the glyph b of the subset is the glyph a of
// the font, with the new IDs of its components.
func sameOutline(a, b tables.Glyph, m *GlyphRemapper) bool {
	switch a := a.Data.(type) {
	case nil:
		return b.Data == nil
	case tables.SimpleGlyph:
		b, ok := b.Data.(tables.SimpleGlyph)
		return ok && slices.Equal(a.EndPtsOfContours, b.EndPtsOfContours) &&
			slices.Equal(a.Points, b.Points) && slices.Equal(a.Instructions, b.Instructions)
	case tables.CompositeGlyph:
		b, ok := b.Data.(tables.CompositeGlyph)
		if !ok || len(a.Glyphs) != len(b.Glyphs) {
			return false
		}
		for i, pa := range a.Glyphs {
			pb := b.Glyphs[i]
			want, ok := m.Get(uint16(pa.GlyphIndex))
			ax, ay := pa.ArgsAsTranslation()
			bx, by := pb.ArgsAsTranslation()
			if !ok || uint16(pb.GlyphIndex) != want || pa.Flags != pb.Flags || ax != bx || ay != by || pa.Scale != pb.Scale {
				return false
			}
		}
		return true
	}
	return false
}

// glyphTables holds the tables of a TrueType font that the tests compare.
type glyphTables struct {
	numGlyphs uint16
	hmtx      tables.Hmtx
	glyf      tables.Glyf
}

// bbox returns the bounding box of a glyph, which is empty for a glyph
// without outline, as ttf-parser gives it.
func (g glyphTables) bbox(glyph uint16) [4]int16 {
	gl := g.glyf[glyph]
	if gl.Data == nil {
		return [4]int16{}
	}
	return [4]int16{gl.XMin, gl.YMin, gl.XMax, gl.YMax}
}

// readGlyphTables reads the tables of the first font of a font file with
// typesetting.
func readGlyphTables(t *testing.T, data []byte) glyphTables {
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
	head, _, err := tables.ParseHead(raw(tagHEAD))
	if err != nil {
		t.Fatal(err)
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
	loca, err := tables.ParseLoca(raw(tagLOCA), n, head.IndexToLocFormat == 1)
	if err != nil {
		t.Fatal(err)
	}
	glyf, err := tables.ParseGlyf(raw(tagGLYF), loca)
	if err != nil {
		t.Fatal(err)
	}
	return glyphTables{numGlyphs: maxp.NumGlyphs, hmtx: hmtx, glyf: glyf}
}
