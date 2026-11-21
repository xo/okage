// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package harfbuzz

import (
	"math/rand/v2"
	"testing"
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
