# Backlog

This document holds the work that is known and not done, with where each item
came from. When an item is done, delete it, and record anything that was
decided in [`decisions/`](decisions/README.md).

Work on the port goes in units: one upstream file with its tests. Claim a unit
under "Units in progress" before you start, with the name of your session.
When the change is committed, delete the claim.

## Units in progress

1. The copy of go-text in `typesetting`, with the change of `svg` and
   `webfont` to it (D52). Claimed by the session of group G of ivy on
   2026-11-17.

## typesetting

From D52.

1. Apply the 110 pending commits of `docs/upstream/typesetting.tsv` in
   order, as "typesetting" in [`UPSTREAM.md`](UPSTREAM.md) says. Many of them
   fix a panic on a malformed font, and ivy reads fonts from strangers.
2. 33 test files of go-text read the data of
   `github.com/go-text/typesetting-utils`, and the copy leaves them out.
   Question 19 asks what they become.
3. A test of `shaping` calls a method of `reflect` that TinyGo does not have,
   so the TinyGo command of `AGENTS.md` leaves `typesetting/shaping` out.

## The coverage of an edge

From the review of the text of ivy on 2026-11-17. okage computes the
coverage of a pixel from a grid of 4 by 4 samples, as tiny-skia does
(`SUPERSAMPLE_SHIFT` of `scan_path_aa.go`). The coverage of an edge moves in
steps of a quarter, so most pixels of a line of text have one of the
coverages 0.25, 0.5, 0.75 or 1. FreeType, MuPDF, Poppler and Cairo give
every coverage. For a page of text at 96 dpi, the mean difference of a pixel
of okage from a pixel of Cairo is 0.064. A grid of 8 by 8 samples gives
0.052, and one of 16 by 16 gives 0.051. ivy draws small glyphs at twice
their size and averages the samples, which costs about half again the time
of the drawing of a page of text. A finer grid, or an exact coverage, in
okage itself is a difference from tiny-skia, and a decision of the
maintainers.

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
2. The walk of the paint graph of COLRv1 in `svg/usvg_text_colr_paint.go`
   ports skrifa 0.44 (D32). usvg now walks the graph with the model of read-fonts
   0.46. No test shows a difference.
3. read-fonts reads the strikes of the table EBDT. usvg draws none of its
   glyphs, so Go leaves the strikes out. The result is the same.

## The API of roxmltree

D47 makes `roxmltree` a public package. `svg` ported `Tree::from_xmltree` of
usvg as the unexported function `fromXMLTree`, because a user was not able to
make a document of roxmltree. Decide whether `svg` exports it now. Source: D47.
