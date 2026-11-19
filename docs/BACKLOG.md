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

1. Apply the 69 pending commits of `docs/upstream/typesetting.tsv` in
   order, as "typesetting" in [`UPSTREAM.md`](UPSTREAM.md) says. D67 applied
   the commits that fix a panic, a loop or a missing bound on malformed
   input, and D70 applied five commits that remove allocations of the
   shaper. Many of the others fix a wrong result on a malformed font.
2. 33 test files of go-text read the data of
   `github.com/go-text/typesetting-utils`, and the copy leaves them out.
   Copy the fonts and the files that they read, each with its license, and
   put the tests back in their packages, as D63 decides.
3. A test of `shaping` calls a method of `reflect` that TinyGo does not have,
   so the TinyGo command of `AGENTS.md` leaves `typesetting/shaping` out.
4. In Amiri, `harfbuzz` places a mark on some ligatures of a short text in
   another place than HarfBuzz 15.1.0. In `لًكنننننن`, U+064B is at the
   x offset -10,560, and HarfBuzz puts it at -2,176, at a scale of 64 for
   each unit of the font. U+064E in `هلَكو` has the same fault. In a longer
   text that holds the same characters, both give the same offset. go-text
   `v0.3.5` and its commit `bafa8730` have the fault. `FuzzUnsafeToBreak`
   found it, and its Arabic alphabet leaves out the marks until a fix
   (D70). Source: D70, 2026-11-19.
5. Two fuzz targets of go-text fail at the commit `113f0ea` of okage, before
   D70. `FuzzBreakOptions` finds that the line breaker of `shaping` gives 2
   graphemes for `"0! ̽"`, and the segmenter gives 3.
   `FuzzE2EVariableSize` finds that `WrapParagraph` gives 4,787 runes for
   the 2,550 runes of `benchParagraphLatin`, at the size 16, the width 100
   and the packed configuration byte 0x55. A pending commit of go-text can
   fix them. Source: D70, 2026-11-19.

## lottie

From D41, D42, D43 and D44. The port of velato draws each sampled frame of the 879
Noto animated emoji within the tolerance of ThorVG. These parts of Lottie are
not done:

1. Expressions. velato and okage ignore them. 29 files of the corpus have
   expressions, and they still pass, because their values are close to the
   values that the keyframes give.
2. The Gaussian blur, the drop shadow and the effect Set Matte in the
   `PixmapSink`. No file of the corpus uses the first two. 21 layers use Set
   Matte, which velato does not convert, and they still pass.
3. Image assets. `PixmapSink.DrawImage` draws nothing, and velato leaves the
   images to the sink. A dotLottie archive keeps its images in the archive,
   and `DotLottie.FS` gives them (D44).
4. Text layers, which velato does not parse. A file with one is an error.
5. The geometric boolean operations of merge paths, for a stroke or a trim
   after a merge path of mode 2 to 5 (D42).

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

## The PDF writer

From D62.

1. Limit the size of a subset of a font. A subset copies the data of a glyph
   once for each glyph ID that the caller asks for, so a hostile font can
   make a subset up to 65,536 times the size of its table glyf. Upstream has
   the same property. Give `internal/subsetter` a limit on its output, and
   return an error past it. Source: phase 13.
2. `typesetting` computes the deltas of gvar, HVAR and the blends of CFF2 in
   float32, and skrifa in fixed point. In the instances of `test/krilla`
   that are not the same as the instances of subsetter, between 1 point in
   750 and 1 point in 150 differs by 1 unit. One glyph of
   `NotoSans-Regular_var.ttf` differs by 2 units, and 21 of the 31,042
   glyphs of the instances of `Cantarell-VF.otf` have another number of
   quadratic curves. The
   maintainers accept this within the tolerance of D65. To give the
   instances of subsetter, `typesetting` must compute the deltas in fixed
   point, as skrifa does. Source: phase 13, unit 4.
3. `path.Path.Stroke` does not end for some paths with a point that is NaN.
   `PathBuilder` takes such a point, and the bounds of the path leave it
   out. For example, the stroke of width 2 of a path that moves to NaN,
   1e38, draws lines to 1, -1.7e38 and -3e38, -2e37, and closes, does not
   end. tiny-skia has the same fault. `pdf` checks each point before it
   strokes a path, and returns `ErrPDFInvalidPath` (D66). A program that
   renders such a path with okage still waits forever. Source: phase 14,
   the merge of units 5 and 6.
4. `typesetting` drops the whole table GPOS of `NotoSans_variable.ttf` of
   krilla, because it cannot read an anchor of a lookup MarkLigPos. The
   text of that font then has no kerning in `pdf`, and the file differs
   from the file of krilla. D67 applies the commit `31064bfc` of go-text,
   which fixes the bounds of the anchors, but the parse still fails with
   "unsupported Anchor format 281" in the MarkArray of the lookup. Find
   the fault in the parse of MarkLigPos. `test/krilla` compares the
   variable fonts with RobotoFlex until it is fixed. Source: phase 16.
5. `pdf/text_glyph_outline.go` and `svg/usvg_text_cache.go` each hold a
   copy of the code that draws an outline of typesetting in the order of
   skrifa. The copy of `pdf` also drops the line back to the start of a
   contour that skrifa starts at its last point, and follows the rules of
   `NopFilterSink` of read-fonts for CFF. The tests of `svg` compare
   images within a tolerance, so the copy of `svg` does not need them yet.
   Move the code into one internal package for both. Source: phase 16.
7. `pdfsvg` draws the filters of SVG with its own port of `filter.rs` of
   krilla-svg (phase 17), and `Surface.DrawRaster` follows the same code
   (D68). D62 item 10 says that the writer uses one code for both. Make
   `pdfsvg` call `Surface.DrawRaster` when both phases are merged. Source:
   phase 18.
