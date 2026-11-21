// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package harfbuzz

import (
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
