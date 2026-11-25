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
`simd`. The package `archsimd`, which okage uses, makes no copies. On
2026-11-25, with Go 1.27.4 on an AMD Ryzen 9 9950X, `GODEBUG=simd=0`,
`simd=128`, `simd=256` and `simd=512` did not change what `archsimd.X86`
reports, and did not change the time of a loop of `Float32x8.Mul` and
`Float32x8.Sub` (D96).

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

A vector type is an array in both implementations, such as `[8]float32`. A
method of the SIMD implementation loads its arrays into vectors of
`archsimd`, works on them, and stores the result in an array. Each type has
its files in this form:

```text
internal/wide/f32x8.go              the scalar code of each method, as a plain function, in every build
internal/wide/f32x8_scalar.go       the methods without the SIMD implementation, which call the plain functions
internal/wide/f32x8_simd.go         the methods of the SIMD build, with the tag
internal/wide/f32x8_simd_amd64.go   the SIMD code on amd64, with Float32x8
internal/wide/f32x8_simd_128.go     the SIMD code on arm64 and wasm, with two Float32x4
```

The program `widegen`, in `internal/wide/internal/widegen`, writes the files
`<type>_scalar.go`, `<type>_simd.go`, `<type>_simd_amd64.go` and
`<type>_simd_128.go`. Each of them starts with the line
`// Code generated by widegen. DO NOT EDIT.` Never edit such a file by hand.
Edit the plain function in `<type>.go`, or the table of `widegen`, and run
`go generate ./internal/wide`. A test of `internal/wide` runs `widegen` and
fails if a generated file differs from its output. The package comment of
`widegen` lists the generated files and the files that a person writes.

`widegen` reads the plain functions. The doc comment of a plain function
starts with a line such as `// addF32x8 is the scalar code of F32x8.Add.`,
and the doc comment of the method follows after a blank comment line. The
table of `widegen` holds the SIMD code of each method, and it turns the SIMD
code of each method on or off. A method whose SIMD code is off calls the plain
function in the SIMD build too. The SIMD code is on only where a benchmark on
amd64 showed it at least 1.2 times as fast as the scalar build. A single
operation, such as `Add`, is slower in the SIMD build, because the load and
the store of the arrays cost more than the operation saves.

To measure the table, run `widegen -all`, and run a benchmark of each method
of the table in both builds, in the form of `bench_test.go`. Then set the
table and run `widegen` without `-all`. The table was last measured on
2026-11-21, with Go 1.27.4 on an AMD Ryzen 9 9950X, with 10 runs of each
benchmark. The SIMD code is on for these methods:

- `F32x4.Round` and `F32x4.Not`.
- `F32x8.Round`, `RoundInt`, `Fract`, `Normalize`, `IsFinite` and `Powf`.
- `F32x16.Normalize`, `Floor` and `Round`.
- `U16x16.Blend`.

That measurement turned the SIMD code of five methods off. The times are the
median ns for each call:

| Method | Scalar build | SIMD code | Scalar build over SIMD code |
| --- | --- | --- | --- |
| `F32x4.Floor` | 13.4 | 14.8 | 0.91 |
| `F32x4.Abs` | 7.2 | 10.1 | 0.71 |
| `F32x4.RoundInt` | 17.0 | 18.0 | 0.95 |
| `F32x8.Floor` | 13.7 | 18.8 | 0.73 |
| `F32x16.Sqrt` | 16.9 | 14.2 | 1.19 |

The file `<type>_scalar.go` has the tag
`!goexperiment.simd || okage_scalar || tinygo || !(amd64 || arm64 || wasm)`,
which is the opposite of the tag of the SIMD implementation. In the SIMD
build, a method with SIMD code runs it if `useSIMD` is true, and calls the
plain function if it is false. `useSIMD` is set once, when the program
starts, in `cpu_simd_amd64.go`, `cpu_simd_arm64.go` and `cpu_simd_wasm.go`.
The helpers of the SIMD code, such as the loads and the minimum with the NaN
rule of upstream, are in `wide_simd_amd64.go` and `wide_simd_128.go`.
`wide_scalar.go` holds the helpers of the other builds. A person writes these
six files.

## The upper bits of the vector registers

A plain float operation of Go on amd64, such as `MULSS`, keeps the upper bits
of its vector register. If those bits can be other than 0, the CPU waits for
the last write to the whole register, and plain float code runs slower. Code
with `archsimd` leaves the bits so, and so does the runtime of the SIMD build
of Go 1.27 each time that it preempts a goroutine, because it restores the
whole vector registers. `wide.ClearUpperBits` clears the bits with
`VZEROUPPER` in the SIMD build on amd64, and does nothing in the other
builds. The pipeline calls it once for each row of a rect, and the fast path
of D53 once for each row of the image (D35). A new loop of plain float code
that runs for a long time calls it in the same way.

This program shows the cost:

```go
func work(x []float32, b []byte) {
	for i := range x {
		x[i] = float32(x[i]*0.5) + float32(b[i])
	}
}
```

With `x` and `b` of 4096 values, after one loop of `archsimd` code, a call
of `work` takes 1.6 µs in the normal build and 6.6 µs in the SIMD build of
Go 1.27.4. With a call of `archsimd.ClearAVXUpperBits` for each 64 calls of
`work`, it takes 1.6 µs in the SIMD build. The documentation of
`archsimd.ClearAVXUpperBits` says that the compiler can clear the bits
itself. When a release of Go does, the calls of `wide.ClearUpperBits` can
go.

The SIMD code of the methods of `internal/wide` also leaves the upper bits
set. On 2026-11-21 two places for the clear were measured with the root
benchmarks of the SIMD build, with 10 runs of each benchmark, on an AMD Ryzen
9 9950X with Go 1.27.4:

1. At the end of the SIMD code of each method, which `widegen` writes. The
   geometric mean of the times changed by +0.4 percent.
2. At the end of each stage, in `nextStage` of `lowp` and of `highp`. The
   geometric mean changed by +1.0 percent.

Neither place is faster, so okage clears the bits in neither place. The hot
stages run no SIMD code of a method, because they hold the loops on the
lanes of D33. A CPU profile of the root benchmarks shows no SIMD code of a
method. With the SIMD code of every method on, from `widegen -all`, the
stages call that code, and both places help. The gradients took 11 to 32
percent less time, and the geometric mean changed by -1.0 percent with the
clear in each method, and by -0.8 percent with the clear at the end of each
stage. If the table of `widegen` turns on a method that a hot stage calls,
measure both places again.

A comparison of two binaries must allow for the place of the code in each
binary. The loop of `blitRect` that fills a rect with one color has the same
instructions in each build. It takes 1.5 times as long when it starts 32
bytes after a boundary of 64 bytes, and five root benchmarks run it:
`FillSource`, `FillOpaque`, `BlendClear`, `BlendSource` and
`CompositeBlend/Clear`. On 2026-11-21 that loop started 32 bytes after such
a boundary in the SIMD build and on the boundary in the scalar build. That
made the geometric mean of the SIMD build 2.0 percent slower than the
scalar build. Without those five benchmarks, the difference was 0.1 percent.

## Copies of a vector

A vector type is an array in both builds (D28). Go copies an array of 32
bytes with loads and stores of 16 bytes. A stage makes such a copy when it
assigns a channel, such as `p.dr = p.r`, or passes a vector as a value. It
also makes one when it takes the vector that a function returns, such as
`wide.SplatF32x8`. A store buffer holds the stores that a CPU did not write
to its cache yet, and a load reads from it when it can. If the stage before
wrote the vector lane by lane, a load of 16 bytes spans 4 stores of 4
bytes, or 8 stores of 2 bytes for a `U16x16`. An AMD Ryzen 9 9950X cannot
forward such a load from its store buffer. The load waits until the stores
are in the cache.

So a hot stage reads and writes the lanes of the channels one by one,
takes a vector as a pointer, and writes a splat lane by lane. A function
that returns a vector writes it through a pointer, as `join` of lowp does.
On 2026-11-21 these changes made the composite of a group 15 to 32 percent
faster with most blend modes, and `BenchmarkDrawPixmapUpscale` 15 to 23
percent faster. The geometric mean of the root benchmarks went down by 12
percent in each build.

## The scan of runes

`RuneRanges` of `internal/wide` is the one code of the package that is not
a vector type of tiny-skia. It holds up to 8 ranges of runes, and its
method `IndexOutside` gives the first rune of a text that is in none of
them. The text code of `typesetting` uses it for the skip of the bidi
algorithm (D92). A person writes its five files: `runes.go` holds the type
and the scalar code, `runes_scalar.go` and `runes_simd.go` hold the method
in each build, and `runes_simd_amd64.go` and `runes_simd_128.go` hold the
SIMD code. The SIMD code tests 8 runes at a time, with `Int32x8` on amd64
and two `Int32x4` on arm64 and wasm. It runs only for a text of 32 runes or
more, and on amd64 it calls `ClearAVXUpperBits` when it ends, because the
text code after it runs plain float code.

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
| floor | `_mm256_round_ps` | the scalar code of upstream, which truncates to `i32` and converts back, so NaN gives 0 and a value out of the range of `i32` gives the bound | `Floor` in `archsimd` where a test shows the same result, else the scalar code |
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
7. Measure the cost of the SIMD build:
   1. Run the program of "The upper bits of the vector registers" in both
      builds, with and without the call of `archsimd.ClearAVXUpperBits`. If
      the SIMD build is as fast without the call, the release clears the
      bits itself. In that case, remove the calls of `wide.ClearUpperBits`, and
      measure the root benchmarks of the SIMD build before and after.
   2. Run the root benchmarks in both builds, 10 runs of each build in
      turn, and compare the geometric means of the times.
   3. Measure the table of `widegen` again, as "The build tags and the CPU
      check" says, and turn the SIMD code of each method on or off from the
      result.
   4. Run a program in the SIMD build with `GODEBUG=simd=0`, `simd=128`,
      `simd=256` and `simd=512`. Find whether each value changes what
      `archsimd.X86` reports, or the time of `archsimd` code.
8. Record the release and the result in this document.

| Release of Go | Date | Result |
| --- | --- | --- |
| 1.27.3 | 2026-11-09 | the toolchain of the plan. No code exists yet |
| 1.27.4 | 2026-11-21 | the table of `widegen` measured again, after the cause in "The upper bits of the vector registers" was found. Five methods turned off, and no method turned on. No change in the bytes |
| 1.27.4 | 2026-11-25 | the steps of step 7 measured again (D96). The release still does not clear the upper bits, so `wide.ClearUpperBits` stays. `GODEBUG=simd` changes nothing for `archsimd`. The table of `widegen` of 2026-11-21 still holds |
