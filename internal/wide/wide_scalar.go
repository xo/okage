//go:build !goexperiment.simd || okage_scalar || tinygo || !(amd64 || arm64 || wasm)

package wide

// ClearUpperBits does nothing. In the SIMD build on amd64 it clears the upper
// bits of the vector registers (see wide_simd_amd64.go).
func ClearUpperBits() {}
