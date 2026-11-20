# Backlog

This document holds the work that is known and not done, with where each item
came from. When an item is done, delete it, and record anything that was
decided in [`decisions/`](decisions/README.md).

Work on the port goes in units: one upstream file with its tests. Claim a unit
under "Units in progress" before you start, with the name of your session.
When the change is committed, delete the claim.

## Units in progress


## typesetting

From D52.

1. Make the text code faster for a long text. A profile of ivy on 2026-11-20,
   after D70, put 23% of the CPU time of the layout of
   `gutenberg/html/pg1342-images.html` and 26% of `gutenberg/txt/pg24264.txt`
   in `typesetting`. D72 then made four changes: the skip of the bidi
   algorithm, the fast paths of the lookups below U+0100, the context of the
   normalizer of `harfbuzz` in its buffer, and the breaker of `LineWrapper`
   as a value. On 2026-11-20 `test/textbench` measured them with 10 runs on
   4 CPUs. `BenchmarkEndToEnd` went from 352 to 305 ns/char on `pg24264`
   (-13%) and from 414 to 350 ns/char on `pg1342` (-15%). Its allocations
   went from 628,035 to 95 and from 187,416 to 62 for each run of a book.
   The skip of the bidi algorithm gave most of the time. The fast paths gave
   -8% on `pg1342`, but +1.7% on `BenchmarkEndToEndWrap` of `pg24264`,
   because a rune above U+00FF now takes one more call. The two changes of
   the buffers removed the allocations. Their change of the time is less
   than 2%.
   D77 then added `shaping.Paragraph`, which shapes a paragraph once, gives
   the glyphs of each line from it, and computes the line breaks of the
   paragraph once. `BenchmarkEndToEndOnce` is the path of ivy with
   `Paragraph`. On 2026-11-20 it took 247 ns/char on `pg24264` and 214
   ns/char on `pg1342`, where `BenchmarkEndToEnd` took 296 and 342 (-17%
   and -38%), with 10 runs on 4 CPUs. It shapes 1.00
   character for each character of each book, where `BenchmarkEndToEnd`
   shapes 1.52 and 1.73. It gives the same lines. ivy does not use
   `Paragraph` yet.
   A profile of `BenchmarkEndToEndOnce` then put 28% of the time of
   `pg24264` in `Segmenter.Split`. Of that time, 69% came from
   `Paragraph.Line`, which splits a line again when its runes are in more
   than one run of the paragraph. The lookups of the script and of the face
   of each rune took most of the split. Item 6 of D72 made two changes:
   - `language.LookupScript` finds the script of a rune above U+00FF with a
     table of the blocks of 256 runes. The table gives the few ranges of
     `ScriptRanges` that can hold a rune of the block, and the binary
     search reads only them. A test compares it with the old lookup for
     every rune.
   - For a paragraph from left to right with no bidi runs, `Paragraph.Init`
     keeps the script, the paired delimiter and the face of each rune. It
     splits the paragraph from them with the steps of `Split`, and `Line`
     splits each line from them too. A test compares the runs with the runs
     of `Split`. `Split` and `SplitByFace` themselves do not change.

   On 2026-11-20 `test/textbench` measured them with 10 runs on 4 CPUs.
   The machine had other work, so each run of the new code ran next to a
   run of the old code, and the numbers are the medians of the pairs. The
   new lookup made `BenchmarkSplit` 22% faster on `pg24264`, 12% on
   `devanagari` and 8% on `arabic`. It made `BenchmarkEndToEndOnce` 5%
   faster and `BenchmarkEndToEnd` 3% faster on `pg24264`. The new split of
   `Paragraph` then made `BenchmarkEndToEndOnce` 15% faster on `pg24264`
   and 13% on `emoji`. Together, `BenchmarkEndToEndOnce` went from 254 to
   203 ns/char on `pg24264` (-20%), and from 271 to 233 ns/char on `emoji`
   (-14%). The other workloads of `BenchmarkEndToEndOnce` and
   `BenchmarkEndToEnd` changed by 3% or less. Most of their runes are below
   U+0100, or most of their lines are in one run. The lines are the same.
   After these changes, a profile of `BenchmarkEndToEndOnce` puts 53% of
   the time of `pg24264` and 67% of `pg1342` in the shaper. The line
   breaks take 20% and 13%, and the split takes 12% and 5%.
   D81 makes six changes to the shaper, which the maintainers accepted on
   2026-11-20. Together they make `BenchmarkEndToEndOnce` 17% faster on
   `pg24264` and 12% on `pg1342`, with the same output. The text is not
   decoded from UTF-8 in `typesetting`, whose API takes runes, so a decode
   once is work for ivy. D83 makes five more changes, which the maintainers
   accepted on 2026-11-21: one table for the Unicode properties that the
   segmenter reads, and caches of the kerning and of the glyph properties
   of GDEF. Together they make `BenchmarkEndToEndOnce` 10% faster on
   `pg24264` and 17% on `pg1342`, with the same output. Next, try the
   candidates of "What remains" in D83. Keep the output of each test and
   the hashes of ivy the same.
   A change to the copy of go-text needs a decision first, as D65, D67, D70,
   D72, D77, D81 and D83 had. A change at the calls in `shaping` keeps the
   copy closer to go-text. Source: D72, D77, D81, D83, `test/textbench`,
   2026-11-20.
2. Use SIMD in `typesetting` where the work is a scan or a loop over an array
   (D17, D28). Most of the text code is a table lookup or a state machine for
   each rune, and SIMD does not help there. These places can use it:
   - A scan of the bytes of a text for any byte that can start a right-to-left
     character or a bidi control, for the skip of the bidi algorithm of D72.
   - A scan for a run of ASCII, for the fast paths of D72.
   - The scale and the rounding of the advances and the offsets of the glyphs
     in `harfbuzz` (`emScalef`, `roundf`). After D72 they take 9% of the
     time of `BenchmarkEndToEnd` on `pg24264` and 8% on `pg1342`. The
     rounding must give the same numbers, and D16 and D71 still apply.
3. Give the flag unsafe-to-concat of HarfBuzz in the glyphs of `Shape` when
   a program needs it. D70 gives only the flag unsafe-to-break. Only the
   shaper inside `shaping.Paragraph` sets `ProduceUnsafeToConcat`, and
   `Paragraph` removes the flag from the glyphs that it gives (D77).
   Source: D70.
4. Make `harfbuzz` follow HarfBuzz 15.1.0 in the tables kern, kerx and
   morx at the places that the work on fixes 13 to 19 of D73 found. No
   text of `perf_reference` and no workload of `test/textbench` uses these
   tables:
   - The parse of `typesetting` refuses some data of a subtable that
     HarfBuzz reads only when it applies the subtable. Since fix 19 of D73,
     such an error refuses the whole table kerx or morx, where HarfBuzz
     applies the table. For example, `typesetting` requires the anchor of
     each entry of a subtable of format 4 of kerx, an action type from 0
     to 2, and the whole kerning array of a subtable of format 6. When an
     anchor is not in the subtable, HarfBuzz skips the transition, and
     does not take the flag Mark.
   - In a ligature subtable of morx, an entry has an action when a bit of
     the mask 0x3FFF of its flags is set. In HarfBuzz, it has an action
     only when its flag PerformAction, 0x2000, is set.
   - A subtable of format 4 of kerx does not apply when the feature kern
     is off and the subtable is not cross-stream. HarfBuzz applies it.
   - A lookup of format 4 in the class table of a state machine drops its
     last segment, also when that segment is not the segment 0xFFFF that
     ends the search. HarfBuzz drops only that segment.
   - Outside these tables, `resolveCrossOffset` of GPOS has neither the
     limit of HarfBuzz on the number of attachments that it follows, from
     703509a9d9, nor its clamp, from bcce9d4f55.
     `propagateAttachmentOffsets` also adds the offsets without the clamp
     of HarfBuzz. If cursive attachments form a cycle, which HarfBuzz says
     lookups with other flags can do, the loop of `resolveCrossOffset`
     does not end.
   - Other commits of HarfBuzz after 873dbc1e change the AAT code, and no
     one compared them with `harfbuzz` yet: 5a1c0c4660, 88e5077a80,
     a763a612d1, ec4a38c4ab, 77be62a733, cd570b6f54 and 370a0da5a1. For
     example, 88e5077a80 reads the class values of a subtable of format 2
     of kerx as byte offsets from the kerning array, and `typesetting`
     reads them in another way.
   Source: D73, 2026-11-20 and 2026-11-21.
5. In a run of only tatweels and spaces, which has the script Common, Amiri
   forms a ligature of tatweels where a run starts, and the shaper does not
   mark that cluster as unsafe to break. uharfbuzz 0.57.0 gives the same
   flags and glyphs, so `Paragraph.Line` of D77 inherits a fault of HarfBuzz,
   as D70 records for U+01EC. The fuzz target of `Paragraph` leaves the
   tatweel out of its alphabet. Source: D77, 2026-11-20.


## lottie

From D41, D42, D43, D44, D75, D79, D82 and D85. The port of velato draws each
sampled frame of the 879 Noto animated emoji within the tolerance of
ThorVG. These parts of Lottie are not done:

1. Expressions. velato and okage ignore them. 29 files of the corpus have
   expressions, and they still pass, because their values are close to the
   values that the keyframes give.
2. The geometric boolean operations of merge paths, for a stroke or a trim
   after a merge path of mode 2 to 5 (D42).
3. The quality of the Gaussian blur and the drop shadow (D79). The sink
   draws them as ThorVG does at its default quality, 50. It has no option
   for another quality.

## Math

1. Lay out math in okage for ivy and zuzu. D78 decides `texmath`, a fork of
   go-tex/math on `typesetting`, with a reader of the OpenType table MATH
   in `typesetting/font` and the font STIX Two Math in
   `texmath/stixtwomath`. The phases 20 to 27 of D78 are the plan, and they
   started on 2026-11-20. Source: zuzu and ivy, 2026-11-20.

   Phases 25 and 26 are done. The snapshots in
   `texmath/testdata/snapshots` give the same bytes on amd64, arm64, s390x,
   wasip1 and TinyGo. The parse gate and the layout gate of KaTeX pass with
   the lists `katex_parse.txt` and `katex_layout.txt` in `test/texmath`.
   Phase 26 added the text face, the color commands, the italic correction,
   the top accent attachment and the math kerning, the brace and the wide
   accents that grow, and the cheap missing commands of D78 section 10.
   It also added the commands that the formulas of the corpus of
   mermaid-go-renderer need, the last of them `\phase`, at the request of
   zuzu. This work is left:

   - Phase 27, the commits of go-tex/math after the base commit.
   - The helpers `KernPair` and `LigaturePair` of D78 section 8, if ivy asks
     for them.
   - `\underrightarrow`, `\sideset` and the environments `align`,
     `equation` and `gather`, which wait for a request. The other commands
     of `katex_parse.txt`, most of them symbols and operators of AMS and
     accents of text, wait for one too.
   - The alignment of `\cfrac[l]` and `\cfrac[r]`, which `texmath` reads and
     drops, a default value of the first parameter of `\newcommand`, and a
     dashed rule for `\hdashline`, which is solid.
   - `\tag` puts its label a quad after the formula, because `Layout` has
     no line width. A label at the right margin needs an option that gives
     the line width, if ivy or zuzu asks for it.

## Fonts in a web format

From D45. `webfont` (D51) does not read an EOT file that uses MicroType
Express compression, and returns `okage.ErrFontUnsupported` for it. Such files
are rare. They come from old versions of Internet Explorer.

## Phase 4

### Make the pipeline faster

From D18, D28, D33 and D35. okage has no speed target (D35), and this work
stays open. After D33, okage is about 1.2 to 3 times slower than the scalar
code of upstream, and about 4 to 11 times slower than its AVX2 code, on the
benchmarks of D18. Measure and try, in this order, keeping the bytes of every
test:

1. The forms of D17 that are not measured: one loop with a `switch` on the
   stage, and the color channels in local variables. The call of each stage
   through a function value is now a large part of a fill. On 2026-11-20 a
   simpler form was measured and not kept: `nextStage` only marked that the
   next stage runs, and one loop called the stages through their function
   values, so that the stages did not nest on the stack. It was slower in
   both builds, by 1.6 percent on the geometric mean of 15 benchmarks and by
   12 percent on `BenchmarkGradientsTwoStopsLinearPadHQ`.
2. Lane functions that are just over the budget of the inliner, and bounds
   checks in the stages (`go build -gcflags='-m -d=ssa/check_bce'`). On
   2026-11-20, `colorBurnHighpLane` and `colorDodgeHighpLane` fit the budget
   of 80, and `BenchmarkBlendColorDodge` went from 4.84 ms to 4.65 ms (D35).
   `setSat` (cost 114), `exclusiveReflect` (91) and `gatherIxLane` (127) are
   still over the budget. On 2026-11-20 a version of `exclusiveReflect` that
   takes `limit + limit` and `inv_limit * 0.5` from its caller still cost
   86, because `wide.LaneFloor` alone costs 44, and it was not kept. The
   bounds checks that remain in the stages are
   one for each stage, on the index of the next stage or on the slice of its
   pixels, and the checks in the loops on `p.tail` of the mask stages.
3. A copy of the stages that keeps its values in `archsimd` registers through
   a run of stages, written by a tool, for the SIMD build (D17, D28).
4. The stages of sRGB and `Powf`, which D33 did not change, because no
   benchmark covers them.
5. The bilinear sampling of highp is still the largest cost of a raster
   page of ivy that okage scales up. Each time in items 5 to 7 is the median
   CPU time of one operation, with 10 runs of the old and the new code in
   turn. On 2026-11-20, `sampler2x2` and `sampler4x4` tile and clamp each
   coordinate once for its column or its row, in one loop on the lanes. If
   the 8 lanes of a group share their row, as they do when the image is not
   turned, `bilinear` and `bicubic` find the row and its weights once for the
   group. A pixel loads without a call, and its bytes become floats from a
   table (D35). `BenchmarkDrawPixmapUpscale` scales an image up by 8 onto a
   page of 1600 by 893 pixels. In the scalar build it went from 62 ms to
   36 ms and then to 27 ms with the bilinear filter, and from 182 ms to
   92 ms and then to 55 ms with the bicubic filter. In the SIMD build it went
   from 36 ms to 28 ms with the bilinear filter, and from 93 ms to 56 ms with
   the bicubic filter. Then a lane whose columns are the columns of the lane
   before it takes the pixels of that lane, which are already floats, when
   the lanes share their row. In the scalar build it went from 28.1 ms to
   26.2 ms with the bilinear filter, and from 57.8 ms to 53.8 ms with the
   bicubic filter. In the SIMD build it went from 28.1 ms to 25.2 ms and
   from 55.8 ms to 52.4 ms. Each lane still finds its columns, then
   compares them with the columns of the lane before it. Most of the time
   that is left is the multiplications and the additions of each lane.
   Source: the measurement of ivy, 2026-11-18, and `bench_ivy_test.go`,
   2026-11-20.
6. The blend modes of highp. On 2026-11-20, each stage of `blend_fn!`,
   `blend_fn2!` and `blend_fn3!` calls its function directly, `store` rounds
   a channel with `LaneRoundIntByte`, and `load_8888`, `mask_u8` and the
   fast path of D53 read the value of a byte from a table (D35). The
   composite of a group of 1600 by 893 pixels in `BenchmarkCompositeBlend`
   went from 22 ms to 15 ms and then to 14.2 ms with Multiply, and from
   23 ms to 18 ms and then to 17.0 ms with Overlay. With Hue it went from
   24.1 ms to 21.6 ms, and with Color from 18.9 ms to 16.4 ms. The SIMD build
   was slower than the scalar build on the fast path of D53, which is plain
   Go, because of the upper bits of the vector registers. "Measure the cost
   of GOEXPERIMENT=simd" below gives the cause. Since 2026-11-20 the fast
   path and the pipelines call `wide.ClearUpperBits` for each row. The
   composite with SourceOver went from 9.4 ms to 6.8 ms in the SIMD build,
   and it takes 6.6 ms in the scalar build. Source: the measurement of ivy,
   2026-11-18, and `bench_ivy_test.go`, 2026-11-20.
7. Make a clip cheaper. Since 2026-11-20, a `Mask` holds only the rows and
   the columns that its paths cover, with a margin of 15 columns on each
   side for the chunks of `maskBounds`. `NewMask` allocates nothing, and
   `Clone` shares the pixels until the first write (D35, D60). A page of a
   spreadsheet in `BenchmarkClipSpreadsheet`, with 496 cells that each
   clone a clip and intersect it with the rectangle of the cell, went from
   121 ms to 81 ms, and from 689 MiB to 38 MiB that it allocates, with the
   rows. With the columns it allocates 14 MiB in place of 40 MiB. Its time
   changed by less than the noise of the machine, from -6 to +2 percent in
   the scalar build and from -4 to -2 percent in the SIMD build, because
   the fills of the cells take most of it. `BenchmarkDrawPixmapLayer`,
   whose mask is a circle in the middle of the page, went from 6.2 ms to
   5.2 ms in the scalar build. One step remains:
   - A clip of a rectangle on the grid of the pixels still makes a mask. ivy
     can draw such a clip without a mask, as `PixmapSink` of `lottie` does,
     or the API of okage can take a clip rectangle. That is a decision of
     the maintainers.

   Source: ivy, item 5 of "Left by the speed of 2026-11-18", 2026-11-19,
   and `bench_ivy_test.go`, 2026-11-20.

### Measure the cost of GOEXPERIMENT=simd in each release of Go

From D18 and D28. In Go 1.27.3 the plain functions are slower in a build with
`GOEXPERIMENT=simd`. Measure it again with each release of Go, as the steps
of "A new release of Go" in `docs/SIMD.md` say, and turn the SIMD paths of
`internal/wide/internal/widegen` on or off again from the measurements.
Also measure whether `GODEBUG=simd` changes what `archsimd` runs, or what
`archsimd.X86` reports (D15, D17).

On 2026-11-20 the cause of the slow plain functions was found, on an AMD
Ryzen 9 9950X with Go 1.27.4. In the SIMD build, `runtime.asyncPreempt`
restores the whole vector registers, Z0 to Z31 and K0 to K7 with AVX-512,
when it preempts a goroutine, and it does not run `VZEROUPPER` after. The
code of `archsimd` does not run it either. A plain float operation of Go,
such as `MULSS` or `CVTSI2SS`, keeps the upper bits of its register, so the
CPU then waits for the last write to the whole register. The next signal,
such as the one of the CPU profiler, clears the state, so a profile does
not show the cause. `wide.ClearUpperBits` now clears the bits for each row
of the pipelines and of the fast path of D53 (`docs/SIMD.md`). The fast path
with SourceOver went from 9.4 ms to 6.8 ms in the SIMD build, against 6.6 ms
in the scalar build. The calls in the pipelines made the root benchmarks of
the SIMD build 1 to 3 percent faster on the geometric mean.

The cause is in Go, and no report was posted. This program shows it:

```go
func work(x []float32, b []byte) {
	for i := range x {
		x[i] = float32(x[i]*0.5) + float32(b[i])
	}
}
```

With `x` and `b` of 4096 values, and 200,000 calls of `work` in a row, a call
took 1.6 µs in the normal build and 6.3 µs in the SIMD build. With a call of
`archsimd.ClearAVXUpperBits` for each 64 calls of `work`, it took 1.6 µs in
the SIMD build. Two steps remain:

- The SIMD code of `internal/wide` leaves the upper bits as they are after
  each method, and the next plain float code of a stage waits. Clear the
  bits at the end of each method on amd64, and measure the table of
  `widegen` again. The rule of 1.2 times of that table was measured before
  this cause was known.
- Remove the calls of `wide.ClearUpperBits` if a release of Go clears the
  bits itself, as the documentation of `archsimd.ClearAVXUpperBits` says
  the compiler can do.

## Phase 6

### Port the example select of simplecss

From phase 6. `examples/select.rs` of simplecss matches selectors against a
tree of roxmltree. It was not ported while `roxmltree` was being
written. Port it as an example of `internal/simplecss`, with its output.

## Phase 10

### Follow the font model of read-fonts 0.46 more closely

From phase 10, resvg commit `04291b74`, and D9 rule 3. usvg now reads fonts
through the owned font model of read-fonts 0.46, which skrifa 0.49 exports.
`svg/usvg_text_cache.go` and `resolveFont` follow its rules on go-text. Three
differences stay:

1. read-fonts keeps a metric with its MVAR delta as an F48Dot16. Go keeps it
   as a float32, so a rounded metric can differ at a location with a delta.
2. The walk of the paint graph of COLRv1 in `internal/colr` ports skrifa
   0.44 (D32, D62). usvg now walks the graph with the model of read-fonts
   0.46. No test shows a difference.
3. read-fonts reads the strikes of the table EBDT. usvg draws none of its
   glyphs, so Go leaves the strikes out. The result is the same.

## Filters

1. Give the blur and the color matrix of the filters of `svg` an API on a
   `Mask` and on a `Pixmap`. ivy blurs the masks of the shadows, the glows and
   the soft edges of DrawingML with box blurs of its own, and changes the
   colors of pictures for the picture effects of Word itself. Both are in
   `internal/drawingml` of ivy, in `blur.go` and `recolor.go`. Source: ivy,
   2026-11-19.

## The PDF writer

From D62.

1. Make a Type 3 font from images that the caller gives, each with its text.
   The writer makes a Type 3 font only from the glyphs of a font file. ivy
   then writes the glyphs of a Type 3 font of bitmaps in a PDF file as image
   masks of the page, which each viewer scales down in another way, and it
   writes a color glyph or a bitmap glyph, such as an emoji, as an image
   without its text. Source: ivy, "Left by phase 7a" item 3 and "Left by
   phase 7b" item 2, 2026-11-19.
