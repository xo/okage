//go:build !goexperiment.simd || okage_scalar || tinygo || !(amd64 || arm64 || wasm)

package wide

// implName names the code that the methods run, for the names of the
// benchmarks. The scalar build has only the scalar code.
func implName() string {
	return "scalar"
}
