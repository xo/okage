//go:build goexperiment.simd && !okage_scalar && !tinygo

package wide

import "simd/archsimd"

// indexOutsideRuneRangesSIMD returns the index of the first block of 8 runes
// of s that holds a rune outside the ranges, or the index of the tail of
// fewer than 8 runes. It uses Int32x8 on amd64 (D92).
func indexOutsideRuneRangesSIMD(rr *RuneRanges, s []rune) int {
	if rr.n == 0 {
		return 0
	}
	var lo, hi [8]archsimd.Int32x8
	for k := range rr.n {
		lo[k] = archsimd.BroadcastInt32x8(rr.lo[k])
		hi[k] = archsimd.BroadcastInt32x8(rr.hi[k])
	}
	i := 0
	for ; i+8 <= len(s); i += 8 {
		x := archsimd.LoadInt32x8Array((*[8]int32)(s[i : i+8]))
		in := x.GreaterEqual(lo[0]).And(x.LessEqual(hi[0]))
		for k := 1; k < rr.n; k++ {
			in = in.Or(x.GreaterEqual(lo[k]).And(x.LessEqual(hi[k])))
		}
		if in.ToBits() != 0xff {
			break
		}
	}
	// The text code that calls it runs plain float code, so the upper
	// bits of the vector registers must be 0 again.
	archsimd.ClearAVXUpperBits()
	return i
}
