// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package harfbuzz

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/xo/okage/typesetting/font"
)

// okage: the tests of D87.

// TestSetDigestMayHave compares mayHave with the three tests of HarfBuzz,
// for random digests and for each glyph.
func TestSetDigestMayHave(t *testing.T) {
	rng := rand.New(rand.NewPCG(87, 2026))
	for range 64 {
		var d setDigest
		for range 1 + rng.IntN(40) {
			d.add(gID(rng.IntN(1 << 16)))
		}
		if rng.IntN(4) == 0 {
			a := gID(rng.IntN(1 << 16))
			d.addRange(a, a+gID(rng.IntN(200)))
		}
		for g := range 1 << 16 {
			want := d[0]&(one<<((gID(g)>>shift0)&mb1)) != 0 &&
				d[1]&(one<<((gID(g)>>shift1)&mb1)) != 0 &&
				d[2]&(one<<((gID(g)>>shift2)&mb1)) != 0
			if got := d.mayHave(gID(g)); got != want {
				t.Fatalf("digest %x: mayHave(%d) = %v, want %v", d, g, got, want)
			}
		}
	}
}

// TestGlyphHOriginIsZero makes sure that the faces give the origin 0, 0
// for each glyph, which hOriginIsZero trusts.
func TestGlyphHOriginIsZero(t *testing.T) {
	for _, name := range []string{"common/Roboto-BoldItalic.ttf", "common/mplus-1p-regular.ttf", "common/Selawik-VF.ttf"} {
		ft := openFontFileTT(t, name)
		f := NewFont(font.NewFace(ft))
		if !f.hOriginIsZero() {
			t.Fatalf("%s: hOriginIsZero is false for a font with no embolden", name)
		}
		for g := range GID(1 << 16) {
			if x, y := f.getGlyphHOriginWithFallback(g); x != 0 || y != 0 {
				t.Fatalf("%s: the origin of glyph %d is %d, %d, want 0, 0", name, g, x, y)
			}
		}
	}
}

// uncachedExtents returns the extents of glyph as GlyphExtents gives them
// without the cache.
func uncachedExtents(f *Font, glyph GID) (GlyphExtents, bool) {
	ext, ok := f.face.GlyphExtents(glyph)
	if !ok {
		return GlyphExtents{}, false
	}
	syntheticGlyphExtents(&ext, f.slant, f.xEmbolden)
	return GlyphExtents{
		XBearing: f.emScalefX(ext.XBearing),
		YBearing: f.emScalefY(ext.YBearing),
		Width:    f.emScalefX(ext.Width),
		Height:   f.emScalefY(ext.Height),
	}, true
}

// checkExtents compares GlyphExtents with uncachedExtents for each glyph of
// f, twice, so that the second pass reads the cache. It returns the
// extents of each glyph.
func checkExtents(t *testing.T, name string, f *Font) map[GID]GlyphExtents {
	t.Helper()
	all := make(map[GID]GlyphExtents)
	for pass := range 2 {
		for g := range GID(1 << 16) {
			got, gotOK := f.GlyphExtents(g)
			want, wantOK := uncachedExtents(f, g)
			if got != want || gotOK != wantOK {
				t.Fatalf("%s, scale %d, %d, pass %d: GlyphExtents(%d) = %v, %v, want %v, %v",
					name, f.XScale, f.YScale, pass, g, got, gotOK, want, wantOK)
			}
			if gotOK {
				all[g] = got
			}
		}
	}
	if len(all) < 100 {
		t.Fatalf("%s: got the extents of %d glyphs, want more", name, len(all))
	}
	return all
}

// TestExtentsCache compares the extents of the cache with the extents
// without it, for each glyph of some fonts, at several scales, with a
// slant and an embolden, and at other variations of a variable font.
func TestExtentsCache(t *testing.T) {
	for _, name := range []string{"common/Roboto-BoldItalic.ttf", "common/mplus-1p-regular.ttf", "common/Selawik-VF.ttf"} {
		ft := openFontFileTT(t, name)
		face := font.NewFace(ft)
		f := NewFont(face)
		checkExtents(t, name, f)
		for _, scale := range []int32{16 << 6, 13 << 6, 1000, 16 << 6} {
			f.XScale, f.YScale = scale, scale
			checkExtents(t, name, f)
		}
		f.YScale = 11 << 6
		checkExtents(t, name, f)
		f.slant, f.xEmbolden = 0.2, 0.03
		checkExtents(t, name, f)
		f.slant, f.xEmbolden = 0, 0
		checkExtents(t, name, f)
		if name == "common/Selawik-VF.ttf" {
			// The face gives other extents at other variations, and the
			// cache must not give the old ones.
			face.SetCoords([]font.VarCoord{-1 << 14})
			light := checkExtents(t, name, f)
			face.SetCoords([]font.VarCoord{1 << 14})
			bold := checkExtents(t, name, f)
			changed := 0
			for g, e := range bold {
				if light[g] != e {
					changed++
				}
			}
			if changed < 100 {
				t.Fatalf("%s: %d glyphs have other extents at other variations, want more", name, changed)
			}
		}
	}
}

// TestExtentsCacheGen makes sure that the cache empties itself when its gen
// goes round, so that an old item does not serve a glyph.
func TestExtentsCacheGen(t *testing.T) {
	ft := openFontFileTT(t, "common/Roboto-BoldItalic.ttf")
	f := NewFont(font.NewFace(ft))
	checkExtents(t, "Roboto", f)
	f.scaledExtents.gen = math.MaxUint32
	for i := range f.scaledExtents.items {
		f.scaledExtents.items[i].gen = 1
	}
	f.XScale = 7 << 6
	f.scaledExtents.sync(f)
	if f.scaledExtents.gen != 1 {
		t.Fatalf("got the gen %d after it went round, want 1", f.scaledExtents.gen)
	}
	for i, item := range f.scaledExtents.items {
		if item.gen != 0 {
			t.Fatalf("item %d has the gen %d after the gen went round, want 0", i, item.gen)
		}
	}
	checkExtents(t, "Roboto", f)
}
