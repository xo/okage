//go:build goexperiment.simd && !okage_scalar && !tinygo

package wide

// useSIMD reports whether the methods of the vector types run their SIMD
// code. The SIMD build for wasm uses SIMD128, and a host without SIMD128
// cannot run the module, so it is true (D15). A test can set it to false to
// run the scalar code in the SIMD build.
var useSIMD = true
