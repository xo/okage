# D96. The speed work of phase 4, and the cost of the SIMD build

Status: Decided, amends D17, amended by D98.

## Context

`docs/BACKLOG.md` held two items of phase 4. "Make the pipeline faster"
had seven steps, from D17, D18, D28, D33 and D35. "Measure the cost of
GOEXPERIMENT=simd in each release of Go" came from D18 and D28. On
2026-11-25 the maintainers asked to clear both items: to try each step,
to keep a change only if it is faster, and to record each result here.

Each change keeps the bytes of the renderer in both builds (D16). The
measurements ran on an AMD Ryzen 9 9950X with Go 1.27.4, with the old and
the new test binary in turn, pinned to one core with `-test.cpu 1`, and
compared with benchstat. Other jobs ran on the machine at the same time,
so a single time varied by up to 30 percent. A result here is a median of
10 runs, unless it says otherwise, and benchstat marks it as significant
(p < 0.05).

## The decision

1. The stages keep the form of upstream: a table of functions, and each
   stage calls the next. Two forms of D17 were measured and not kept:
   - One loop with a `switch` on the stage, in place of the calls through
     function values. The stages no longer call `nextStage`, and the Go
     compiler inlines 76 calls of stages into the `switch`. On 17
     benchmarks the geometric mean changed by -3.1 percent in the scalar
     build and by +2.2 percent in the SIMD build, and
     `BenchmarkHairlineAA` was 20 percent slower in the SIMD build.
   - The same loop with the channels in local variables: the pipeline is a
     local variable of `run`, which stays on the stack. The geometric mean
     changed by +3.0 percent in the scalar build and by +1.9 percent in the
     SIMD build. Each change is within the noise, and none is faster in
     both builds.
2. `clipColor` of the non-separable blend modes returns early when its
   channels need no clip: when `mx` is 0 or more and not more than `a`, it
   gives `c.max(0)` for each channel without `lum`. The Go compiler still
   does not inline it (cost 132), but the common case is shorter. The
   composite of a group in `BenchmarkCompositeBlend` went from 24.5 ms to
   22.2 ms with Hue and from 18.1 ms to 16.9 ms with Color in the scalar
   build, and from 23.7 ms to 21.6 ms, 23.6 ms to 20.5 ms, 19.0 ms to 17.6
   ms and 21.2 ms to 19.6 ms with Hue, Saturation, Color and Luminosity in
   the SIMD build. The geometric mean of the four went down by 8.1 percent
   in the scalar build and by 9.4 percent in the SIMD build. This is step 2
   and step 6 of the item.
3. Step 2 left these as they are:
   - `exclusiveReflect` (cost 91) and `softLightHighpLane` (cost 163).
     No benchmark runs the stage `reflect`, and the soft light function
     does the work of upstream for each lane, so a smaller cost needs a
     change of the arithmetic.
   - The one bounds check for each stage, on the index of the next stage
     in `nextStage`. A check that is always true costs about one cycle, and
     the noise of the machine hides a change of that size.
4. Step 3, a copy of the stages that keeps its values in `archsimd`
   registers, written by a tool, is not built. A prototype for one group
   of 8 pixels ran the program of `BenchmarkGradientsTwoStopsLinearPadHQ`:
   `seed_shader`, `transform`, `pad_x1`, `evenly_spaced_2_stop_gradient`,
   `premultiply`, `load_dst`, `source_over` and `store`. It gave the same
   bytes as the stages on 200,000 random groups:
   - The stages, each a function of its own: 73 ns for each group.
   - The same stages in one loop on the lanes, with scalar Go: 47 to 64 ns.
   - The same stages in one function with `archsimd.Float32x8`: 17 ns.

   The fill of the benchmark covers 415,411 pixels, about 52,000 groups,
   so the stages take most of its 3.1 ms, and the copy in `archsimd` can
   save about 2 ms of it. The tool is still large. Each program of stages
   needs its own copy, and hard rule 8 lets only `internal/wide` hold code
   with the SIMD tag, so the copies cannot be stages of the pipeline. Where
   the copies live, and which programs get one, is a choice for the
   maintainers.
5. The stages of the color spaces, which no benchmark of tiny-skia covers,
   now have `BenchmarkGamma`. It fills a rect of 812 by 777 pixels with a
   color that is not opaque, on a pixmap that is not empty.
   - `LanePowf` of `internal/wide` gives one lane of `F32x8.Powf`, with the
     operations of the chain of methods of upstream in the same order.
     The scalar code of `Powf` calls it for each lane. `TestLanePowf`
     compares it with the chain.
   - The stages of the gamma 2 and of sRGB work on one lane at a time, as
     D33 says. The stages of the gamma 2.2 still call `Powf`, because its
     SIMD code was faster than `LanePowf` for each lane in the SIMD build:
     16.8 ms against 13.7 ms with `ColorSpaceSimpleSRGB`.
   - In the scalar build, `ColorSpaceSimpleSRGB` went from 85.4 ms to
     15.7 ms, and `ColorSpaceFullSRGBGamma` from 101.5 ms to 18.8 ms. In the
     SIMD build, `ColorSpaceFullSRGBGamma` went from 36.1 ms to 17.5 ms,
     and `ColorSpaceSimpleSRGB` stayed at 13 to 14 ms. `ColorSpaceGamma2`
     stayed at 5 to 6 ms in both builds. These times are medians of 5 or 6
     runs.
6. `sampler4x4` of the bicubic filter reads the weights of its lane once,
   before its loops on the rows and the columns.
   `BenchmarkDrawPixmapUpscale` with the bicubic filter went from 45.7 ms
   to 43.9 ms in the scalar build and from 46.8 ms to 44.5 ms in the SIMD
   build. `BenchmarkPatternsHQ` went from 28.3 ms to 27.8 ms in the scalar
   build, and did not change in the SIMD build. This is step 5.
7. Step 7, a cheaper clip, gets no change. A CPU profile of
   `BenchmarkClipGridRect` and `BenchmarkClipSpreadsheet` shows
   `Mask.IntersectPath` at 5 percent of the time. The fills of the cells
   take the rest.

## The cost of the SIMD build

On 2026-11-25 the current release of Go is 1.27.4, the release of the
measurement of 2026-11-21 in [`../SIMD.md`](../SIMD.md). That measurement
of the table of `widegen` still holds. The other steps were measured
again:

1. Plain float code is still slow after `archsimd` code in the SIMD build.
   The program of "The upper bits of the vector registers" in
   [`../SIMD.md`](../SIMD.md) takes 1.6 µs for each call in the normal
   build, 6.6 µs in the SIMD build, and 1.6 µs in the SIMD build with a
   call of `archsimd.ClearAVXUpperBits` for each 64 calls. Go 1.27.4 does
   not clear the bits itself, so okage keeps `wide.ClearUpperBits`.
2. `GODEBUG=simd=0`, `simd=128`, `simd=256` and `simd=512` do not change
   what `archsimd.X86` reports, and do not change the time of a loop of
   `Float32x8.Mul` and `Float32x8.Sub`, about 1.2 ns for each step.
3. The root benchmarks of the old code, without `BenchmarkGamma`, ran 6
   times in each build in turn. The geometric mean of the SIMD build was
   12.7 percent less than the geometric mean of the scalar build. The
   times of the scalar build varied by more than 100 percent in that run,
   because of the other jobs, so the run does not show a cost or a gain.
   The SIMD build is not slower than the scalar build on these
   benchmarks. On 2026-11-21 the difference was 0.1 percent without the
   five benchmarks that fill a rect with one color.

These measurements are now step 7 of "A new release of Go" in
[`../SIMD.md`](../SIMD.md), so they need no item in the backlog.

## The root benchmarks

The root benchmarks, without `BenchmarkGamma`, ran 5 times in each build,
the old code and the new code in turn. The geometric mean went down by 2.0
percent in the scalar build and by 0.8 percent in the SIMD build. The
benchmarks of points 2, 5 and 6 changed as those points say. Two
gradients of highp and `BenchmarkPatternsPlain`, whose stages did not
change, were 1 to 2 percent slower in the SIMD build, which is within the
effect of the place of the code in the binary ("The upper bits of the
vector registers" in [`../SIMD.md`](../SIMD.md)).

## Consequences

`docs/BACKLOG.md` no longer holds phase 4. Work that makes okage faster is
still welcome (D35). The next large step is the copy of the stages in
`archsimd` registers of point 4, after the maintainers choose where such
copies live. D17 recorded that the forms of point 1 were not measured.
This decision measures them.
