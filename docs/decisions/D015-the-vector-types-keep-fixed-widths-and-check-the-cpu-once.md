# D15. The vector types keep fixed widths and check the CPU once

Status: Decided, amended by D28.

## Fixed widths

`internal/wide` declares `F32x4`, `F32x8`, `F32x16`, `I32x4`, `I32x8`,
`U32x4`, `U32x8` and `U16x16`, with the methods of the Rust types. The
pipeline uses `lowp` with 16 pixels and `highp` with 8 pixels, as upstream
does. The pipeline calls the methods and never indexes a lane, so its code
does not change with the build.

okage does not use the portable package `simd`, whose types have a length that
depends on the machine, for three reasons:

1. The stages, the loads and stores of a tail, and the choice between `lowp`
   and `highp` read the widths of upstream. A width that changes with the
   machine changes all of them.
2. The portable package of Go 1.27 lacks floor, round and the any and all
   tests of a mask, which the pipeline needs.
3. A fixed width gives the same operations in the same order on every
   machine, which D16 needs.

## Two implementations

1. The scalar implementation is an array, such as `[8]float32`. It is in every
   build.
2. The SIMD implementation uses `simd/archsimd`. On amd64 an `F32x8` is one
   `archsimd.Float32x8`. On arm64 and WebAssembly, where the widest vector is
   128 bits, it is two `archsimd.Float32x4`. It has the tag
   `goexperiment.simd && !okage_scalar && !tinygo && (amd64 || arm64 || wasm)`.

The tag `okage_scalar` leaves the SIMD implementation out of a program that
sets `GOEXPERIMENT=simd`, so that a test can build both forms in one checkout.
The part `!tinygo` keeps it out of a build with TinyGo (D25). The list of
architectures keeps it out of a platform that `archsimd` does not support,
such as riscv64, where the build fails otherwise. Gemini Pro advised on
2026-11-09 to drop the list, and that is not taken for this reason.

Only the files of `internal/wide` carry these tags. dbmeta hard rule 11 bans a
build constraint on an architecture, because SQL queries do not depend on the
machine. Vector code does, and only `internal/wide` has such a constraint.

## The CPU check

On amd64, every operation of `archsimd` needs AVX, including the 128-bit
types, and the 256-bit integer operations, such as `Uint16x16.Add`, need
AVX2. `archsimd` does not choose a copy of a function for the CPU, and its
documentation asks a program to read `archsimd.X86` first. So the SIMD build
reads the CPU once, when the program starts:

1. On amd64 with AVX2, the pipeline runs the SIMD implementation.
2. On amd64 without AVX2, the pipeline runs the scalar implementation.
3. On arm64, NEON is always present, and the pipeline runs the SIMD
   implementation.
4. On WebAssembly, the host must support SIMD128. The prototype of D17
   measures what a host without it does.

The prototype of D17 chooses how the pipeline holds the two implementations.
Astra raised the CPU check on 2026-11-09, and the documentation of `archsimd`
in Go 1.27.3 confirmed it.

When Go makes SIMD a standard feature, the tag of the SIMD implementation
changes, and nothing else does. [`docs/SIMD.md`](../SIMD.md) maps each method
to its Go form.
