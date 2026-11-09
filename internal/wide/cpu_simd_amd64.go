//go:build goexperiment.simd && !okage_scalar && !tinygo

package wide

import "simd/archsimd"

// useSIMD reports whether the methods of the vector types run their SIMD
// code. On amd64, each operation of archsimd needs AVX, and the 256-bit
// integer operations need AVX2. So the program reads the CPU once, when it
// starts, and a CPU without AVX2 runs the scalar code (D15). A test can set
// it to false to run the scalar code in the SIMD build.
var useSIMD = archsimd.X86.AVX2()
