# SIMD

This document says how the vector code of okage is built, how it maps to the
SIMD packages of Go, and how okage follows the changes of those packages. It
was written on 2026-11-09, before any code existed (D3). The facts about Go
were measured in the toolchain of this machine, Go 1.27.3.

SIMD (single instruction, multiple data) means one instruction that works on
several values at once. A lane is one value of a vector.

## What Go 1.27 gives

Go 1.27 has two SIMD packages. Both exist only when a program is built with
`GOEXPERIMENT=simd`, and Go makes no promise that their API stays the same.

1. `simd` is portable. Its types, such as `Float32s` and `Uint16s`, have a
   length that depends on the machine: 4 lanes of `float32` with NEON, 8 with
   AVX2 and 16 with AVX-512. A missing instruction is emulated.
2. `simd/archsimd` has a fixed width for each architecture. amd64 has
   `Float32x4`, `Float32x8` and `Float32x16`. arm64 has 128-bit types only,
   because Go 1.27 supports NEON and not SVE. WebAssembly has 128-bit types.

The compiler makes one copy of a function that uses `simd` for each vector
width: `@simd128`, `@simd256`, `@simd512` and `@simd0`, which emulates. The
function chooses its copy when it starts, from the level that the program
found when it started. `GODEBUG=simd=0`, `simd=128`, `simd=256` and
`simd=512` force a level.

A library cannot set `GOEXPERIMENT` for its users. A program that imports
okage gets the SIMD build only if its own build sets `GOEXPERIMENT=simd`.

The copies of a function for each width belong to the portable package
`simd`. The package `archsimd`, which okage uses, makes no copies. Phase 1
measures whether `GODEBUG=simd` changes what `archsimd` runs, or what
`archsimd.X86` reports.

## Why the widths stay fixed

okage keeps the fixed-width types of upstream, and it does not use the
portable types with a stage width equal to their length (D15). The reasons:

1. The pipeline of upstream is written for 16 pixels in `lowp` and 8 pixels
   in `highp`. The stages, the loads and stores of a tail, and the choice
   between `lowp` and `highp` all read these numbers. A width that changes
   with the machine changes every one of them, and the port no longer follows
   upstream.
2. The portable package lacks floor, round, an exact any or all on a mask, and
   shuffles. The pipeline needs the first three.
3. A fixed width gives the same bytes on every machine, because each lane
   gets the same operations in the same order (D16).

The cost is that the SIMD build does not use AVX-512 for wider vectors.
`F32x16` can be one `archsimd.Float32x16` on amd64 with AVX-512, because
upstream already has that type.

## The build tags and the CPU check

D15 sets how a build gets each implementation:

| Implementation | Tag | When it runs |
| --- | --- | --- |
| scalar | none. It is in every build | in every build without the SIMD implementation, and on an amd64 CPU without AVX2 |
| SIMD | `goexperiment.simd && !okage_scalar && !tinygo && (amd64 \|\| arm64 \|\| wasm)` | on arm64, on WebAssembly, and on amd64 with AVX2 |

The tag `okage_scalar` leaves the SIMD implementation out of a program that
sets `GOEXPERIMENT=simd`. It lets a test compare both builds in one checkout.

On a platform that Go 1.27 does not support with `archsimd`, such as riscv64,
a program that sets `GOEXPERIMENT=simd` gets only the scalar implementation.

TinyGo gets only the scalar implementation (D25). It sets the tags `tinygo`
and `purego`, and it does not support the SIMD packages of Go. TinyGo 0.44.0
does not turn on SIMD128 for its targets for WebAssembly. The flag
`-llvm-features` of `tinygo build` can turn it on, and the comparison of D22
measures that build.

On amd64, every operation of `archsimd` needs AVX, including the 128-bit
types, and the 256-bit integer operations, such as `Uint16x16.Add`, need
AVX2. `archsimd` does not choose a copy of a function for the CPU, and its
documentation asks a program to read `archsimd.X86` first. So the SIMD build
reads `archsimd.X86.AVX2()` once, when the program starts, and the pipeline
runs the scalar implementation if it returns false (D15). The prototype of
phase 1 chooses how the pipeline holds the two implementations (D17).

Only the files of `internal/wide` carry the tag. The pipeline, the shaders
and the rest of okage have no tag. When Go makes SIMD a standard feature, the
change is to this tag, and to the names that the release of Go gives the
packages.

Each type has its files in this form:

```text
internal/wide/f32x8.go          the methods that both implementations share
internal/wide/f32x8_scalar.go   the scalar implementation, in every build
internal/wide/f32x8_simd.go     the SIMD implementation, with the tag
```

## The operations

This table maps each operation that `src/wide` uses to its Go form in each
build. A row with "helper" uses a function of `internal/wide` that gives the
same bytes in both builds.

| Operation | Upstream on x86 | Scalar build | SIMD build |
| --- | --- | --- | --- |
| add, subtract, multiply, divide | `_mm256_add_ps` and others | one statement for each lane | `Add`, `Sub`, `Mul`, `Div` |
| minimum and maximum | `_mm256_min_ps`, `_mm256_max_ps` | helper, as `fast_min` and `fast_max` of upstream | `Min`, `Max`, if a test shows the same NaN rule, else helper |
| square root | `_mm256_sqrt_ps` | `math.Sqrt` on a `float64`, converted to `float32` | `Sqrt` |
| absolute value | `_mm256_and_ps` with a mask | helper | `Abs` |
| compare | `_mm256_cmp_ps` | a lane of all ones or all zeros | `Less` and the others, which give a mask |
| select | `_mm256_blendv_ps` | helper | `IfElse` |
| float to integer, truncate | `_mm256_cvttps_epi32` | helper, which saturates as Rust does | `ConvertToInt32`, if a test shows the same result for a value that does not fit, else helper |
| float to integer, round | `_mm256_cvtps_epi32` | helper, which rounds half to even | `Round`, then the truncating conversion |
| integer to float | `_mm256_cvtepi32_ps` | one statement for each lane | `ConvertToFloat32` |
| floor | `_mm256_round_ps` | `math.Floor` | `Floor` in `archsimd` |
| reciprocal | `_mm256_rcp_ps` | `1 / x` | `Div` (D16) |
| reciprocal square root | `_mm256_rsqrt_ps` | `1 / sqrt(x)` | `Div` and `Sqrt` (D16) |
| shifts | `_mm256_sll_epi32`, `_mm256_srl_epi32` | one statement for each lane | `ShiftAllLeft`, `ShiftAllRight` |
| and, or, xor | `_mm256_and_si256` and others | one statement for each lane | `And`, `Or`, `Xor` |
| any lane, all lanes | `_mm256_movemask_ps` | a loop | `ToBits` of the mask, or a store and a loop, until Go gives mask operations |
| multiply and add | `f * m + a`, not fused | `float32(f*m) + a` | `Mul`, then `Add`. Never `MulAdd` |

Two rows need a test before the SIMD build uses the instruction of Go: the
NaN rule of `Min` and `Max`, and the result of `ConvertToInt32` for a value
that does not fit. A unit test of `internal/wide` compares each method of the
SIMD build with the scalar build on special values: zero, negative zero, NaN,
the infinities, the largest and smallest floats, and the values at each
boundary of `int32`.

## U16x16

Upstream uses a plain array for `u16x16` on x86, and NEON on arm64. okage
uses `archsimd` in the SIMD build on every platform that has it: one
`Uint16x16` on amd64, and two `Uint16x8` on arm64 and WebAssembly. The
benchmarks of phase 4 decide whether the scalar form is faster on amd64. If
it is, the SIMD build keeps the scalar form there, as upstream does.

## The same bytes

The SIMD build gives the same bytes as the scalar build (D16). The rules are
in "The same bytes in every build" in [`PLAN.md`](PLAN.md). The tests of phase
4 compare both builds on every test and on random scenes from the oracle.

## A new release of Go

The SIMD packages are an experiment, and each release of Go can change them.
When a new release of Go comes out, follow these steps:

1. Read the release notes of the release, and the blog of the Go team, for
   changes to `simd` and `simd/archsimd`.
2. Build the SIMD build with the new toolchain, and run every test in it and
   with the fallback of D15.
3. If a name changed, change the files with the SIMD tag.
4. If the release adds `archsimd` for a new architecture, add it to the SIMD
   tag of D15, with its implementation.
5. If the release adds an operation that the table above marks as a helper or
   as a store and a loop, measure it. Use it only if the tests show the same
   bytes.
6. If the release adds SVE on arm64, or makes SIMD a standard feature, write
   a decision for the change.
7. Record the release and the result in this document.

| Release of Go | Date | Result |
| --- | --- | --- |
| 1.27.3 | 2026-11-09 | the toolchain of the plan. No code exists yet |
