// Package wide holds the vector types of tiny-skia, a port of src/wide, and
// the conversions from a float to an integer that give the result of Rust.
//
// Each vector type has a scalar implementation, which is in every build, and
// a SIMD implementation, which is in a build with GOEXPERIMENT=simd on amd64,
// arm64 and wasm. Both give the same bytes. See docs/SIMD.md and the
// decisions D15 and D16.
//
// A vector type is an array in every build, such as [8]float32. The scalar
// code of each method is a plain function, such as addF32x8, in the file of
// the type. In the SIMD build, a method loads its arrays into vectors of
// simd/archsimd, works on them and stores the result. It does so only if
// useSIMD is true, which the program sets once when it starts. Otherwise it
// calls the plain function. A method whose SIMD code is not faster always
// calls the plain function.
//
// The program widegen in internal/widegen writes the methods and the SIMD
// code from the plain functions and its own table. Its package comment lists
// the files that it writes. Run "go generate ./internal/wide" after a change
// to a plain function or to that table.
package wide

//go:generate go run ./internal/widegen
