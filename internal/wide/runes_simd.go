//go:build goexperiment.simd && !okage_scalar && !tinygo && (amd64 || arm64 || wasm)

package wide

// IndexOutside returns the index of the first rune of s that is in none of
// the ranges, or len(s) if each rune is in a range (D92). It tests 8 runes at
// a time with SIMD code if useSIMD is true and s holds 32 runes or more. It
// tests the runes of a block that holds a rune outside the ranges, the
// runes of the tail and a shorter s with the scalar code, which is faster
// for a few runes.
func (rr *RuneRanges) IndexOutside(s []rune) int {
	if !useSIMD || len(s) < 32 {
		return indexOutsideRuneRanges(rr, s)
	}
	i := indexOutsideRuneRangesSIMD(rr, s)
	return i + indexOutsideRuneRanges(rr, s[i:])
}
