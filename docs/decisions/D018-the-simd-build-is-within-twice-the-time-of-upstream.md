# D18. The SIMD build is within twice the time of upstream

Status: Decided, amended by D35.

1. The SIMD build of okage takes at most twice the time of upstream with AVX2
   on the ported benchmarks of fills, on amd64.
2. The speed of the scalar build is measured and recorded, with no target.

Phase 4 measures both, with the benchmarks of tiny-skia that okage ports. The
nightly job of CI runs them (D24).

## What was measured

On 2026-11-09, on an AMD Ryzen 9 9950X, with the scenes of the benchmarks of
upstream, which fill a path of 1000 by 1000 pixels. upstream was built with
`--no-default-features --features std` for its scalar code, and with the
feature `simd` and `-C target-cpu=x86-64-v3` for AVX2. The times are in ms for
each fill.

| Benchmark | upstream, AVX2 | upstream, scalar | okage, scalar build | okage, SIMD build |
| --- | --- | --- | --- | --- |
| `BlendSourceOver` | 0.44 | 0.61 | 6.8 | 6.9 |
| `BlendColorDodge` | 0.60 | 3.62 | 53 | 73 |
| `BlendHue` | 1.09 | 4.75 | 75 | 120 |
| `GradientsTwoStopsLinearPad` | 1.23 | 2.47 | 30 | 43 |
| `GradientsTwoStopsLinearPadHQ` | 0.61 | 3.78 | 38 | 47 |

The target is not met. okage is about 10 to 20 times slower than the scalar
code of upstream, so the gap is not only SIMD. Two causes are known:

1. The pipeline keeps its values in arrays, and each operation of a vector is
   a call that returns an array. The Go compiler does not merge these as LLVM
   does (D28).
2. In Go 1.27.3, a build with `GOEXPERIMENT=simd` makes ordinary code slower.
   The same plain function takes about 13 ns in the SIMD build and 9.7 ns in
   the scalar build. So the SIMD build is slower than the scalar build on
   most of these scenes.

Two functions of the scalar code of upstream, the rounding and the floor of a
lane, took most of the time of the blend modes and the gradients. Their Go
form is now a loop on the lanes that gives the same bits for each of the 2^32
values of a lane, and the times above include that change. Before it,
`BlendColorDodge` took 111 ms.

`docs/BACKLOG.md` holds the work that closes the gap.

D35 closes this target. okage has no speed target, and work that makes it
faster stays open.
