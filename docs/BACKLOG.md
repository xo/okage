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
   than 2%. A profile of `BenchmarkEndToEnd` after D72 shows the work that
   remains.
   Try these, in this order, and keep the output of each test and the
   hashes of ivy the same:
   - Shape a paragraph once, as item 3 says. The shaper takes 51% of the
     time of `pg24264` and 69% of `pg1342`. The benchmark shapes 1.52 and
     1.73 characters for each character of the book, so this saves about
     17% and 29% of the time.
   - Keep the break attributes of a paragraph for all its lines.
     `segmenter.computeBreakAttributes` takes 19% of the time of `pg24264`
     and 13% of `pg1342`, and a program that breaks a paragraph into lines
     asks for the attributes of the same text again for each line.
   - Measure `splitByFace` and `splitByScript`, which take 8% and 6% of the
     time of `pg24264`. `language.LookupScript` still does a binary search
     for each rune above U+00FF, about 5% of the time of `pg24264`.
   - Decode the UTF-8 of a text once. `runtime.decoderune` takes 3% of the
     time of `pg24264`. D72 does not do this.
   A change to the copy of go-text needs a decision first, as D65, D67, D70
   and D72 had. A change at the calls in `shaping` keeps the copy closer to
   go-text. Source: D72, `test/textbench`, 2026-11-20.
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
3. Give `shaping` a way to shape a paragraph once and to give the glyphs of a
   line from it, as HarfBuzz suggests with the flag unsafe-to-break. Only the
   glyphs next to a cut that is not safe to break need a new shaping. ivy does
   this itself since D70, but it still shapes about 1.3 characters for each
   character of the lines of `pg1342-images.html`, and about 1.6 for
   `pg24264.txt`, because a text that starts or ends inside an earlier text
   shapes again. Source: ivy, item 9 of "Left by the speed of 2026-11-18",
   2026-11-20.
4. Give the flag unsafe-to-concat of HarfBuzz when a program needs it. D70
   gives only the flag unsafe-to-break, and the shaper never sets
   `ProduceUnsafeToConcat`. Source: D70.
5. `typesetting` drops the whole table GSUB or GPOS when one subtable of a
   lookup has an error. HarfBuzz sets the offset that fails to null, up to
   32 offsets for each table, and keeps the rest of the table. To do the
   same, the parse of each table and the sanitizers of `font` must drop
   only the part that fails, and `harfbuzz` must accept a missing part.
   Source: D73, 2026-11-20.
6. `BenchmarkShaping` of `harfbuzz` does not end on the text
   `perf_reference/texts/duployan.txt` in the font
   `NotoSansDuployan-Regular.otf`. At `-benchtime 1x`, one shaping takes
   more than 90 seconds in okage and in go-text `bafa8730`. The other texts
   take 5 seconds or less. Find out whether the shaper loops. Source: D76,
   2026-11-20.

## lottie

From D41, D42, D43, D44 and D75. The port of velato draws each sampled
frame of the 879 Noto animated emoji within the tolerance of ThorVG. These
parts of Lottie are not done:

1. Expressions. velato and okage ignore them. 29 files of the corpus have
   expressions, and they still pass, because their values are close to the
   values that the keyframes give.
2. The Gaussian blur, the drop shadow and the effect Set Matte in the
   `PixmapSink`. No file of the corpus uses the first two. 21 layers use Set
   Matte, which velato does not convert, and they still pass.
3. Text layers, which velato does not parse. A file with one is an error.
4. The geometric boolean operations of merge paths, for a stroke or a trim
   after a merge path of mode 2 to 5 (D42).
5. The edges of an image that is scaled up and turned (D75). ThorVG fades
   the edge of such an image into the background over the width of one
   pixel of the image. The `PixmapSink` antialiases the edge over one pixel
   of the target. An image of 16 by 12 pixels that is scaled by 3.3 and
   turned on a canvas of 100 by 100 differs from ThorVG in 4% of the pixels,
   all at its edges. A turned image at about its own size, and a scaled
   image that is not turned, are within the tolerance of D43.

## Math

1. Read the OpenType table MATH in `typesetting/font`: the constants, the
   glyph info and the variants of stretchy glyphs, as the API `hb-ot-math`
   of HarfBuzz gives them. zuzu needs them to lay out the KaTeX math of the
   labels of Mermaid. The parser of TeX and the layout of math stay out of
   okage. A reader of the table is an addition to the copy of go-text, so it
   needs a decision first. The maintainers deferred it on 2026-11-20.
   Source: zuzu, 2026-11-20.

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
   `blendFn` through a function value, and bounds checks in the stages
   (`go build -gcflags='-m -d=ssa/check_bce'`).
3. A copy of the stages that keeps its values in `archsimd` registers through
   a run of stages, written by a tool, for the SIMD build (D17, D28).
4. The stages of sRGB and `Powf`, which D33 did not change, because no
   benchmark covers them.
5. The bilinear sampling of highp, `sampler2x2` and `sample`, costs about
   80 ns for each pixel. A raster page of ivy that okage scales up spends
   almost all its time there: a page of a PSD file at 96 dpi takes 2.5 s.
   Source: the measurement of ivy, 2026-11-18.
6. The blend modes of highp (`loadDst`, `store`, `blendFn2` and
   `blendFn3`) take most of the time of the composite of a group in ivy.
   Source: the measurement of ivy, 2026-11-18.
7. `blitAntiH` and `blitMask` can skip the rows where the mask is 0, as
   `blitRect` does (D60).
8. Make a clip cheaper. A clip of ivy makes a `Mask` of the size of the band
   that it draws, and a clip inside a clip clones the mask. On some pages of
   spreadsheets in ivy, the clips take about a quarter of the time, and
   `NewMask` and `Mask.Clone` make about 80% of the bytes that the pages
   allocate. A mask that covers only the bounds of its path, or a clip of a
   rectangle without a turn that needs no mask, removes most of this cost.
   Source: ivy, item 5 of "Left by the speed of 2026-11-18", 2026-11-19.

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
4. When the subset of a font fails in `Document.Finish`, draw the glyphs of
   that font as paths and write the file, or give a method that makes the
   subset of a font before `Finish`. Today one font that the subsetter refuses
   makes the whole file fail. ivy then makes the subset of each font in a file
   of its own, and draws every page again without the font that fails. Source:
   ivy, `pdf/pdfenc`, 2026-11-19.
5. Export the test of the ranges of `DrawGlyphs`, so that a caller knows that
   the ranges of its glyphs are valid before the call. Today a range that is
   not valid makes `Document.Finish` fail, and ivy tests the ranges itself.
   Source: ivy, `pdf/pdfenc`, 2026-11-19.
6. Make a Type 3 font from images that the caller gives, each with its text.
   The writer makes a Type 3 font only from the glyphs of a font file. ivy
   then writes the glyphs of a Type 3 font of bitmaps in a PDF file as image
   masks of the page, which each viewer scales down in another way, and it
   writes a color glyph or a bitmap glyph, such as an emoji, as an image
   without its text. Source: ivy, "Left by phase 7a" item 3 and "Left by
   phase 7b" item 2, 2026-11-19.
