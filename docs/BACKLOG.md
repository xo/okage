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
   breaks take 20% and 13%, and the split takes 12% and 5%. Try this next,
   and keep the output of each test and the hashes of ivy the same:
   - Decode the UTF-8 of a text once. D72 and D77 do not do this.
   A change to the copy of go-text needs a decision first, as D65, D67, D70,
   D72 and D77 had. A change at the calls in `shaping` keeps the copy closer
   to go-text. Source: D72, D77, `test/textbench`, 2026-11-20.
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
4. Make `harfbuzz` follow HarfBuzz 15.1.0 where the fix of faults 7 and 8
   of D73 found it different, but no test text shows it:
   - When the buffer has no operations left in the middle of a lookup,
     HarfBuzz drops the output of that lookup. `harfbuzz` keeps it.
   - HarfBuzz stops the shaping when the buffer grows past its limit
     of length. `harfbuzz` compares the length with the limit only before
     each lookup.
   - The AAT code takes the operations in other ways than HarfBuzz. For
     example, HarfBuzz takes the length of the buffer for each pass of a
     subtable of `morx`.
   - HarfBuzz takes operations for the rules that it tests when the
     Indic shaper asks whether a lookup substitutes glyphs. `harfbuzz` takes
     none.
   - In HarfBuzz, a PairPos of format 2 gives the class 0 to a second
     glyph that its ClassDef2 does not hold, and the subtable applies.
     `harfbuzz` does not apply the subtable, and tries the next one.
   Source: D73, 2026-11-20.
5. In a run of only tatweels and spaces, which has the script Common, Amiri
   forms a ligature of tatweels where a run starts, and the shaper does not
   mark that cluster as unsafe to break. uharfbuzz 0.57.0 gives the same
   flags and glyphs, so `Paragraph.Line` of D77 inherits a fault of HarfBuzz,
   as D70 records for U+01EC. The fuzz target of `Paragraph` leaves the
   tatweel out of its alphabet. Source: D77, 2026-11-20.


## lottie

From D41, D42, D43, D44, D75 and D79. The port of velato draws each sampled
frame of the 879 Noto animated emoji within the tolerance of ThorVG. These
parts of Lottie are not done:

1. Expressions. velato and okage ignore them. 29 files of the corpus have
   expressions, and they still pass, because their values are close to the
   values that the keyframes give.
2. The effect Set Matte in the `PixmapSink`. 21 layers use it, which velato
   does not convert, and they still pass.
3. The text animators of a text layer, its key `a`, and the alignment of
   their groups, its key `m` (D79). A text layer draws as if it had none.
   The capitals of the key `ca` of a text document are not drawn either.
4. The text on a path of a text layer, its key `p` (D79). The text draws on
   its lines, as if it had no path.
5. The geometric boolean operations of merge paths, for a stroke or a trim
   after a merge path of mode 2 to 5 (D42).
6. The edges of an image that is scaled up and turned (D75). ThorVG fades
   the edge of such an image into the background over the width of one
   pixel of the image. The `PixmapSink` antialiases the edge over one pixel
   of the target. An image of 16 by 12 pixels that is scaled by 3.3 and
   turned on a canvas of 100 by 100 differs from ThorVG in 4% of the pixels,
   all at its edges. A turned image at about its own size, and a scaled
   image that is not turned, are within the tolerance of D43.
7. The tracking of a text that the `PixmapSink` shapes with the fonts of
   `WithFontDatabase` (D79). The sink adds the tracking to each advance, as
   for the glyphs of the list `chars`. ThorVG multiplies each advance of a
   font that it shapes itself by a factor. So a line with tracking has
   another width than in ThorVG, and the test compares such a font only
   without tracking.
8. The edge mode wrap of the Gaussian blur (D79). The `PixmapSink` blurs
   the layer as if the pixels outside it were transparent. ThorVG tiles
   the pixels of the layer outside it. The sink draws a blur and a shadow
   as ThorVG does at its default quality, 50. It has no option for another
   quality.

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
   This work is left:

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

## Phase 3

### Make the f64 functions of crmath faster

From D26. `crmath.Acos64`, `crmath.Ln64` and `crmath.Log2_64` compute with
`math/big` only, about 9 microseconds for each call. Give them the fast path
in double-double precision that `Sin64`, `Cos64`, `Tan64`, `Atan2_64` and
`Pow64` have. Only the clipping of a cubic calls `Acos64`.

## Phase 4

### Make the pipeline faster

From D18, D28, D33 and D35. okage has no speed target (D35), and this work
stays open. After D33, okage is about 1.2 to 3 times slower than the scalar
code of upstream, and about 4 to 11 times slower than its AVX2 code, on the
benchmarks of D18. Measure and try, in this order, keeping the bytes of every
test:

1. The forms of D17 that are not measured: one loop with a `switch` on the
   stage, and the color channels in local variables. The call of each stage
   through a function value is now a large part of a fill.
2. Lane functions that are just over the budget of the inliner, the calls of
   `blendFn3` through a function value, and bounds checks in the stages
   (`go build -gcflags='-m -d=ssa/check_bce'`).
3. A copy of the stages that keeps its values in `archsimd` registers through
   a run of stages, written by a tool, for the SIMD build (D17, D28).
4. The stages of sRGB and `Powf`, which D33 did not change, because no
   benchmark covers them.
5. The bilinear sampling of highp is still the largest cost of a raster
   page of ivy that okage scales up. Each time in items 5 to 8 is the median
   CPU time of one operation, in the scalar build. On 2026-11-20,
   `sampler2x2` and `sampler4x4` tile and clamp each coordinate once for its
   column or its row, in one loop on the lanes (D35).
   `BenchmarkDrawPixmapUpscale` scales an image up by 8 onto a page of 1600
   by 893 pixels. It went from 62 ms to 36 ms with the bilinear filter, and
   from 182 ms to 92 ms with the bicubic filter. The image is not turned, so the 8 lanes of a group
   share their row. A stage for such a transform can find the row once for
   each group. Source: the measurement of ivy, 2026-11-18, and
   `bench_ivy_test.go`, 2026-11-20.
6. The blend modes of highp. On 2026-11-20, each stage of `blend_fn!` and
   `blend_fn2!` calls its function directly, so that the Go compiler inlines
   it, and `store` rounds a channel with `LaneRoundIntByte` (D35). The
   composite of a group of 1600 by 893 pixels in `BenchmarkCompositeBlend`
   went from 22 ms to 15 ms with Multiply, and from 23 ms to 18 ms with
   Overlay. The stages of `blend_fn3!`, `loadDst` and `gatherAligned` remain.
   Source: the measurement of ivy, 2026-11-18, and `bench_ivy_test.go`,
   2026-11-20.
7. `callHLineBlitter` of `scan_hairline_aa.go` gives its runs to the blitter
   through an interface, so they escape to the heap on each call, about 300
   bytes for each horizontal hairline. Since 2026-11-20, `blitAntiH` and
   `blitMask` skip the rows and the chunks where the mask is 0 (D60), and
   `rectClipBlitter` no longer allocates on each call of `blitAntiH2` and
   `blitAntiV2`. `BenchmarkFillMaskZeroRowsDense` went from 8.8 ms to
   3.7 ms, and from 66000 to 265 allocations. Source: `bench_ivy_test.go`,
   2026-11-20.
8. Make a clip cheaper. Since 2026-11-20, a `Mask` holds only the rows that
   its paths cover, `NewMask` allocates nothing, and `Clone` shares the rows
   until the first write (D35). A page of a spreadsheet in
   `BenchmarkClipSpreadsheet`, with 496 cells that each clone a clip and
   intersect it with the rectangle of the cell, went from 121 ms to 81 ms,
   and from 689 MiB to 38 MiB that it allocates. Two steps remain:
   - A mask still holds each column of its rows. A mask that holds only the
     columns of its path must give the stage `mask_u8` the chunks of 16
     pixels that `maskBounds` keeps (D60).
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

1. `typesetting` computes the deltas of gvar, HVAR and the blends of CFF2 in
   float32, and skrifa in fixed point. In the instances of `test/krilla`
   that are not the same as the instances of subsetter, between 1 point in
   750 and 1 point in 150 differs by 1 unit. One glyph of
   `NotoSans-Regular_var.ttf` differs by 2 units, and 21 of the 31,042
   glyphs of the instances of `Cantarell-VF.otf` have another number of
   quadratic curves. The
   maintainers accept this within the tolerance of D65. To give the
   instances of subsetter, `typesetting` must compute the deltas in fixed
   point, as skrifa does. Source: phase 13, unit 4.
2. `pdf/text_glyph_outline.go` and `svg/usvg_text_cache.go` each hold a
   copy of the code that draws an outline of typesetting in the order of
   skrifa. The copy of `pdf` also drops the line back to the start of a
   contour that skrifa starts at its last point, and follows the rules of
   `NopFilterSink` of read-fonts for CFF. The tests of `svg` compare
   images within a tolerance, so the copy of `svg` does not need them yet.
   Move the code into one internal package for both. Source: phase 16.
3. `pdfsvg` draws the filters of SVG with its own port of `filter.rs` of
   krilla-svg (phase 17), and `Surface.DrawRaster` follows the same code
   (D68). D62 item 10 says that the writer uses one code for both. Make
   `pdfsvg` call `Surface.DrawRaster` when both phases are merged. Source:
   phase 18.
4. Make a Type 3 font from images that the caller gives, each with its text.
   The writer makes a Type 3 font only from the glyphs of a font file. ivy
   then writes the glyphs of a Type 3 font of bitmaps in a PDF file as image
   masks of the page, which each viewer scales down in another way, and it
   writes a color glyph or a bitmap glyph, such as an emoji, as an image
   without its text. Source: ivy, "Left by phase 7a" item 3 and "Left by
   phase 7b" item 2, 2026-11-19.
