# D26. The math functions are correctly rounded

Status: Proposed.

tiny-skia calls functions of the C library through the methods of `f32` and
`f64` in Rust: `acos` and `cos` for an `f64` in `src/path64/cubic64.rs`, and
`acos`, `cos`, `sin` and `powf` for an `f32` in tiny-skia-path. D16 asks phase
3 to measure them, and a decision to record what okage does.

## What was measured

On 2026-11-09, on 20,000 random inputs for each function, against a result
computed with `math/big` at 160 bits and rounded:

| Function | glibc 2.44 differs from the rounded result | Go differs from glibc |
| --- | --- | --- |
| `acos`, `f64` | 24 | 6,223 |
| `cos`, `f64` | 20 | 5,345 |
| `acosf`, `f32` | 0 | 0 |
| `cosf`, `f32` | 223 | 223 |
| `sinf`, `f32` | 265 | 265 |
| `powf`, `f32` | 7 | 7 |

The port of the scan converters matched tiny-skia in 8,888 of 9,000 cases of
a recording oracle. The 112 others clip a cubic, which calls `acos` and `cos`.
With the functions of glibc in place of those of Go, all 9,000 matched.

So glibc is not exact either. Its `f32` functions miss the rounded result in
about 1 percent of these inputs, and its `f64` functions in about 0.1
percent. Rust on another platform calls another C library, with other
results. To match glibc bit for bit, okage has to port glibc, whose code is
under the LGPL, into a package under the BSD-3-Clause license.

## The decision

1. okage computes these functions correctly rounded, in the package
   `internal/crmath`: `Acos64`, `Cos64`, `Acos32`, `Cos32`, `Sin32` and
   `Pow32`. A correctly rounded result is the exact value rounded to the
   nearest float, so it is the same on every platform and with TinyGo.
2. A function for an `f32` computes with the package `math` in `float64`, and
   computes again with `math/big` only when that result is too close to the
   middle of two `f32` values to decide the rounding. It takes about 11 ns.
3. A function for an `f64` computes with `math/big` at 160 bits. It takes
   about 9 microseconds. Only the clipping of a cubic calls it.
4. The tests that compare okage with the Rust oracle on Linux expect a
   difference only where glibc is not correctly rounded. A scene with no
   cubic curve must give the same bytes. A scene with a cubic curve can
   differ, and the test counts such scenes and fails if more than 1 percent
   of them differ. On 2026-11-09, 2 of 11,900 random scenes differed, both
   with a clipped cubic, and both matched when the functions of glibc took
   the place of `crmath` in a copy of the port.

The results of `math.Acos` and `math.Cos` of Go are not used, because they
miss the rounded result more often: by three units in the last place for
`math.Acos(0.9831809036248165)`.

A faster `f64` path, with arithmetic in double-double precision and the same
fallback, is in `docs/BACKLOG.md`.
