//go:build goexperiment.simd && !okage_scalar && !tinygo && (arm64 || wasm)

package wide

import "simd/archsimd"

// indexOutsideRuneRangesSIMD returns the index of the first block of 8 runes
// of s that holds a rune outside the ranges, or the index of the tail of
// fewer than 8 runes. It uses two Int32x4 on arm64 and wasm (D92).
func indexOutsideRuneRangesSIMD(rr *RuneRanges, s []rune) int {
	if rr.n == 0 {
		return 0
	}
	var lo, hi [8]archsimd.Int32x4
	for k := range rr.n {
		lo[k] = archsimd.BroadcastInt32x4(rr.lo[k])
		hi[k] = archsimd.BroadcastInt32x4(rr.hi[k])
	}
	i := 0
	for ; i+8 <= len(s); i += 8 {
		a := archsimd.LoadInt32x4Array((*[4]int32)(s[i : i+4]))
		b := archsimd.LoadInt32x4Array((*[4]int32)(s[i+4 : i+8]))
		inA := a.GreaterEqual(lo[0]).And(a.LessEqual(hi[0]))
		inB := b.GreaterEqual(lo[0]).And(b.LessEqual(hi[0]))
		for k := 1; k < rr.n; k++ {
			inA = inA.Or(a.GreaterEqual(lo[k]).And(a.LessEqual(hi[k])))
			inB = inB.Or(b.GreaterEqual(lo[k]).And(b.LessEqual(hi[k])))
		}
		var lanes [4]int32
		inA.And(inB).ToInt32x4().StoreArray(&lanes)
		if lanes[0]&lanes[1]&lanes[2]&lanes[3] != -1 {
			break
		}
	}
	return i
}
