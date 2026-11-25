package shaping

import (
	"math/rand/v2"
	"testing"

	ucd "github.com/xo/okage/typesetting/internal/unicodedata"
)

// TestNoBidiRanges makes sure that no rune of noBidiRanges has a class of
// bidiClassesRTL or is a paragraph separator (D92).
func TestNoBidiRanges(t *testing.T) {
	for r := rune(0); r <= 0x10FFFF; r++ {
		if !noBidiRanges.Contains(r) {
			continue
		}
		if class, _ := ucd.LookupBidiClass(r); class&bidiClassesRTL != 0 || class == ucd.BD_B {
			t.Fatalf("%U is in noBidiRanges, but has the bidi class %d", r, class)
		}
	}
}

// needsBidiLookup is needsBidi with a lookup for each rune, as before D92.
func needsBidiLookup(text []rune) bool {
	for i, r := range text {
		class, _ := ucd.LookupBidiClass(r)
		if class&bidiClassesRTL != 0 || (class == ucd.BD_B && i != len(text)-1) {
			return true
		}
	}
	return false
}

// TestNeedsBidiScan compares needsBidi with needsBidiLookup on random texts
// of runes inside and outside noBidiRanges, at each length up to 40, so that
// the rune that needs the bidi algorithm is in each lane of a block of 8 and
// in the tail (D92).
func TestNeedsBidiScan(t *testing.T) {
	rng := rand.New(rand.NewPCG(92, 1))
	pool := []rune{'a', ' ', '\n', '\r', 0x85, 0x5D0, 0x627, 0x660, 0x4E2D, 0x2014, 0x200F, 0x202B, 0x2069, 0xFB1D, 0x10900, 0x1F600}
	for n := range 41 {
		for range 2000 {
			text := make([]rune, n)
			for i := range text {
				if rng.IntN(8) == 0 {
					text[i] = pool[rng.IntN(len(pool))]
				} else {
					text[i] = 'a' + rune(rng.IntN(26))
				}
			}
			if got, want := needsBidi(text), needsBidiLookup(text); got != want {
				t.Fatalf("needsBidi(%U) = %t, want %t", text, got, want)
			}
		}
	}
}
