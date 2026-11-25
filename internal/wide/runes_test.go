package wide

import (
	"math/rand/v2"
	"testing"
)

// indexOutsideNaive is RuneRanges.IndexOutside with a plain test of each
// range.
func indexOutsideNaive(ranges [][2]rune, s []rune) int {
	for i, r := range s {
		in := false
		for _, rg := range ranges {
			in = in || rg[0] <= r && r <= rg[1]
		}
		if !in {
			return i
		}
	}
	return len(s)
}

// TestRuneRanges compares IndexOutside with a plain test of each range, on
// random texts of each length up to 80, with 1 to 8 ranges, so that the
// first rune outside the ranges is in each lane of a block of 8, in the
// tail and in a text shorter than the blocks of the SIMD code (D92). The
// runes are near the bounds of the ranges, and some are negative or above
// U+10FFFF.
func TestRuneRanges(t *testing.T) {
	rng := rand.New(rand.NewPCG(92, 3))
	for n := range 81 {
		for range 300 {
			ranges := make([][2]rune, 1+rng.IntN(8))
			for k := range ranges {
				lo := rune(rng.IntN(0x110000))
				ranges[k] = [2]rune{lo, lo + rune(rng.IntN(0x2000))}
			}
			rr := NewRuneRanges(ranges...)
			s := make([]rune, n)
			for i := range s {
				rg := ranges[rng.IntN(len(ranges))]
				switch rng.IntN(20) {
				case 0:
					s[i] = rg[0] - 1
				case 1:
					s[i] = rg[1] + 1
				case 2:
					s[i] = -1 - rune(rng.IntN(1000))
				case 3:
					s[i] = 0x110000 + rune(rng.IntN(1000))
				default:
					s[i] = rg[0] + rune(rng.IntN(int(rg[1]-rg[0])+1))
				}
			}
			want := indexOutsideNaive(ranges, s)
			if got := rr.IndexOutside(s); got != want {
				t.Fatalf("IndexOutside(%v) with %v = %d, want %d", s, ranges, got, want)
			}
			if got := indexOutsideRuneRanges(&rr, s); got != want {
				t.Fatalf("indexOutsideRuneRanges(%v) with %v = %d, want %d", s, ranges, got, want)
			}
		}
	}
}

// TestNewRuneRangesPanics makes sure that NewRuneRanges panics for more
// than 8 ranges and for an invalid range.
func TestNewRuneRangesPanics(t *testing.T) {
	for _, ranges := range [][][2]rune{
		make([][2]rune, 9),
		{{5, 4}},
		{{-1, 4}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("NewRuneRanges(%v) did not panic", ranges)
				}
			}()
			NewRuneRanges(ranges...)
		}()
	}
}
