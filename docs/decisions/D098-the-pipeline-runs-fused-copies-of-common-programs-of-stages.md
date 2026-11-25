# D98. The pipeline runs fused copies of common programs of stages

Status: Decided, amends D17 and D96.

## Context

Point 4 of D96 measured a prototype of a copy of a program of stages that
keeps its values in `archsimd` registers. For one group of 8 pixels of
`BenchmarkGradientsTwoStopsLinearPadHQ`, the stages took 73 ns and the copy
took 17 ns. Hard rule 8 lets only `internal/wide` hold code with the SIMD
tag, so the copies cannot be stages of the pipeline. Question 42 asked the
maintainers where such copies live, and which programs get one. On
2026-11-25 the maintainers chose answer 1: a tool writes a copy for each of
a few common programs into `internal/wide`, the pipeline uses a copy when
its program matches one, and the stages otherwise. A test compares each
copy with the stages, byte for byte.

A fused program is such a copy. It runs the arithmetic of each stage on
each pixel, in the same order, so it gives the same bytes as the stages
(D16). It follows hard rule 7: it multiplies and adds with
`float32(x*y) + z` in the scalar code, and with `Mul` and then `Add` in the
SIMD code.

## The decision

1. The tool `fusegen`, in `internal/wide/internal/fusegen`, writes the
   fused programs from its table. `go generate ./internal/wide` runs it, as
   it runs `widegen`. The table holds each program in a small language of
   operations on a pixel, such as `madd`, `div255`, `loadi` and `storef`.
   The package comment of `fusegen` describes the language.
2. `fusegen` writes three files of `internal/wide`:
   - `fused_programs.go`, in every build, holds the names of the programs
     and the scalar code of each. The scalar code works on one pixel at a
     time and keeps its values in local variables.
   - `fused_scalar.go` holds `FusedRow` in a build without the SIMD code.
     It runs the scalar code. TinyGo gets this file (D25).
   - `fused_simd_amd64.go` holds `FusedRow` and the SIMD code of each
     program. The SIMD code works on 8 pixels at a time with
     `archsimd.Float32x8` and `archsimd.Uint32x8`, and the scalar code draws
     the pixels that are left. It runs only if `useSIMD` is true, and it
     clears the upper bits of the vector registers when it ends (D35).
   arm64 and WebAssembly run the scalar code in the SIMD build too, because
   their SIMD code was not written or measured.
3. A 16-bit lane of lowp is a lane of 32 bits in the SIMD code. An add, a
   subtract and a multiply keep the low 16 bits right. `fusegen` tracks the
   largest value of each lane, and it clears the high 16 bits before a
   shift where they can be other than 0. So the SIMD code gives the wrapped
   result of a `u16` of Rust.
4. `matchFused`, in `pipeline_fused.go` of the root package, matches the
   stages of a pipeline when the pipeline compiles. `run` then calls
   `wide.FusedRow` for each row of its rect, in place of the stages. A rect
   that goes one column past the right side of the pixmap still draws that
   pixel on the next row, as the blitter of commit `67c37f6` keeps it
   (`blitColumns`).
5. The fused programs are these six:
   - `FusedSolidOverLowp`: `uniform_color`, `source_over_rgba`, or
     `uniform_color`, `load_dst`, `source_over`, `store`. This is a fill
     with a solid color that is not opaque.
   - `FusedSolidCoverageOverLowp`: `uniform_color`, `scale_1_float`,
     `load_dst`, `source_over`, `store`. This is a span of anti-aliasing.
   - `FusedSolidMaskOverLowp` and `FusedSolidMaskCoverageOverLowp`: the
     same with `mask_u8` after `uniform_color`. This is a fill through a
     clip mask, the work of the spreadsheets of ivy.
   - `FusedLinearPad2Highp` and `FusedLinearPad2Lowp`: `seed_shader`,
     `transform`, `pad_x1`, `evenly_spaced_2_stop_gradient`, `premultiply`,
     `source_over_rgba`, in highp and in lowp. The builder leaves out
     `transform` for the identity, and that program matches too.
6. A solid color matches only when each channel of its color is 255 or
   less, and a span runs the fused program only when its coverage gives 255
   or less. The colors and the coverages of the painter always do. The
   others run the stages.
7. `mask_u8` returns without a store when each byte of its mask is 0. The
   fused programs skip such a pixel. With a mask of 0 the source is 0, and
   `source_over` gives the destination back, so the bytes are the same.

## How the programs were chosen

A count of the pixels that each program of stages draws, in one run of
each root benchmark, gave these programs at the top:

| Program of stages | Share of the pixels |
| --- | --- |
| lowp `uniform_color`, `source_over_rgba` | 31.6% |
| lowp `uniform_color`, `mask_u8`, `load_dst`, `source_over`, `store` | 5.7% |
| highp patterns with `bicubic`, `bilinear` or `gather`, each | 1.4 to 1.8% |
| highp `gather_aligned` with each blend mode, each | 1.4% |
| lowp `uniform_color`, `load_dst`, `source_over`, `store` | 1.2% |
| lowp `uniform_color`, `mask_u8`, `scale_1_float`, `load_dst`, `source_over`, `store` | 1.0% |
| lowp and highp gradients of two stops, each | 0.4% |

The benchmarks of `bench_ivy_test.go` alone draw 52% of their pixels with
the program of `mask_u8`, 25% with the patterns of `DrawPixmapUpscale`, 10%
with `uniform_color`, `load_dst`, `source_over`, `store`, and 9% with the
program of `mask_u8` and `scale_1_float`.

The patterns get no fused program. A copy of the program `seed_shader`,
`repeat`, `gather`, `source_over_rgba` of `BenchmarkPatternsPlain` in
scalar Go, one pixel at a time, took 9.3 µs for a row of 990 pixels, where
the stages took 9.0 µs. The time of a pattern is in the reads of the pixels
of the source and in the filters, one lane at a time, and `archsimd` of Go
1.27 has no gather instruction. The other programs of the table, such as
the blend modes of `gather_aligned` and the glyphs of
`uniform_color`, `load_dst`, `lerp_1_float`, `store`, draw few pixels or
spend their time outside the stages, so they get no fused program either.

## The numbers

The root benchmarks ran 10 times in each build, with the old and the new
test binary in turn, pinned to one core with `-test.cpu 1`, on an AMD Ryzen
9 9950X with Go 1.27.4. The old binary is the same code with the fused
programs turned off. benchstat is not on the machine, so a time here is the
median of the 10 runs, in ms:

| Benchmark | Scalar build, old | Scalar build, new | SIMD build, old | SIMD build, new |
| --- | --- | --- | --- | --- |
| `FillRect` | 2.41 | 0.99 | 2.32 | 0.17 |
| `FillRectAA` | 2.46 | 1.11 | 2.42 | 0.27 |
| `FillPathAA` | 1.63 | 0.66 | 1.62 | 0.28 |
| `BlendSourceOver` | 1.68 | 0.71 | 1.61 | 0.13 |
| `Clip` | 3.01 | 1.65 | 2.92 | 0.47 |
| `Hairline` | 3.29 | 1.73 | 3.17 | 1.72 |
| `GradientsTwoStopsLinearPad` | 5.70 | 3.12 | 5.10 | 0.72 |
| `GradientsTwoStopsLinearPadHQ` | 3.23 | 2.39 | 3.03 | 0.51 |
| `ClipSpreadsheet` | 83.6 | 36.5 | 82.1 | 35.4 |
| `ClipSpreadsheetPath` | 82.9 | 38.1 | 87.0 | 37.2 |
| `ClipGrid` | 63.4 | 27.6 | 63.5 | 25.3 |
| `ClipGridRect` | 54.3 | 23.9 | 51.4 | 22.3 |
| `FillMaskZeroRows` | 3.13 | 2.56 | 3.16 | 2.77 |

The scalar build gets its gain from the scalar code of the fused programs,
which keeps its values in local variables and calls no stage. The
geometric mean of the 105 root benchmarks went down by 12.7 percent in
the scalar build and by 19.8 percent in the SIMD build. The benchmarks
whose programs did not change moved by up to 18 percent in either
direction, from the other jobs on the machine. A second run of 8 turns of
`Gamma/Gamma2`, `GradientsTwoStopsLinearReflect` and `DrawPixmapScaled` in
the SIMD build, which moved by 6 to 18 percent in the first run, gave the
old and the new times within 1 percent.

`GradientsTwoStopsLinearPadHQ` fills about 52,000 groups of 8 pixels. Its
fill went from about 58 ns to about 10 ns for each group in the SIMD build,
with the scan of the path in both times. The prototype of D96 gave 73 ns
and 17 ns for one group alone. The spreadsheets of ivy take less than half
of their time in both builds. Their rest is the scan of the paths and the
masks of the clips, which the fused programs do not change.

## Consequences

A new fused program follows the steps of "The fused programs" in
[`../SIMD.md`](../SIMD.md): a line in the table of `fusegen`, a match in
`matchFused`, and a case in `fusedCases` of `pipeline_fused_test.go`.
`TestFusedPrograms` runs each fused program and its stages on random
pixmaps, rects, masks, colors, coverages and gradients, with NaN and
infinities, and makes sure that both give the same bytes.
`TestFusedRowSIMD` makes sure that the SIMD code gives the same bytes as the
scalar code. `TestFusegen` makes sure that the generated files are the
output of `fusegen`. Question 42 is answered. D17 and D96 keep the form of
upstream for the stages, and this decision adds the fused programs beside
them.
