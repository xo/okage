//go:build !goexperiment.simd || okage_scalar || tinygo || !(amd64 || arm64 || wasm)

package wide

// IndexOutside returns the index of the first rune of s that is in none of
// the ranges, or len(s) if each rune is in a range (D92).
func (rr *RuneRanges) IndexOutside(s []rune) int {
	return indexOutsideRuneRanges(rr, s)
}
