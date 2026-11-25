# D82. lottie follows ThorVG at the edges and in the tracking

Status: Decided, amends D41, D75 and D79.

ThorVG is the second oracle of `lottie` (D43). The items 2, 6, 7 and 8 of
"lottie" in [`../BACKLOG.md`](../BACKLOG.md) name places where the
`PixmapSink` draws in another way than ThorVG. On 2026-11-20 the source of
ThorVG was read at the tag `v1.1.1`. That is the version of the build
`@thorvg/webcanvas` 1.1.1 that `github.com/xo/lottie` v0.1.0 runs, and that
the test module compares with. Each item has a prototype in files of okage
that start with `okage_`, and a test in `test/lottie/compat_test.go` that
makes its own file and compares it with ThorVG within the tolerance of D43.
On 2026-11-20 the maintainers accepted the parts 1 to 3 below, answer 2 of
question 34. They did not accept the prototype of Set Matte.

## What ThorVG does

1. The Gaussian blur and the drop shadow. `effectGaussianBlur` in
   `src/renderer/cpu_engine/tvgSwPostEffect.cpp` blurs the pixels of the
   layer in a region: the bounds of the layer, grown by the size of the
   blur, and cut to the canvas. Each box blur repeats the pixel at the edge
   of that region outside it (`_gaussianEdgeExtend`). The key "Repeat Edge
   Pixels" of the effect reaches the renderer, but the renderer uses it only
   in a log line. The region holds transparent pixels around the layer, so
   the repeat shows only where the layer reaches the edge of the canvas.
   There, the blur does not fade the layer. The drop shadow blurs in the
   same way. Item 8 of the backlog says that ThorVG tiles the pixels. That
   is not the case: ThorVG repeats the pixels at the edge.
2. The tracking of a text that ThorVG shapes with a font of its own.
   `updateText` in `src/loaders/lottie/tvgLottieBuilder.cpp` gives the text
   the spacing factor 1 plus the tracking times the size over the ascent of
   the font. The tracking is the key `tr` in tenths, and the size the key
   `s` in hundredths. The ascent is the ascent of the table `hhea` at that
   size. `src/loaders/sfnt/tvgSfntLoader.cpp` multiplies each advance, with
   its kerning, by the factor. A negative factor is refused, and then the
   factor stays 1. A text of the list `chars` still adds the tracking to
   each advance, as the `PixmapSink` already does.
3. An image that turns by an angle that is not a multiple of 90 degrees.
   `rasterTexmapPolygon` in `src/renderer/cpu_engine/tvgSwRasterTexmap.h`
   draws a pixel when the image holds the bottom-right corner of the pixel.
   It samples the image there with a bilinear filter that puts the center of
   an image pixel at its top-left corner. `_feathering` fades the first and
   the last row and column of the image, from transparent at the edge of the
   image to opaque one image pixel inside it. The edge has no other
   antialiasing. An image that does not turn, or that turns by a multiple of
   90 degrees, does not fade.
4. The effect Set Matte, ADBE Set Matte3, of the type 28. `getEffect` in
   `src/loaders/lottie/tvgLottieParser.cpp` knows the types 5, 20 to 23, 25
   and 29, and skips any other effect. So ThorVG skips Set Matte, as velato
   does. Item 2 of the backlog is not a difference from ThorVG. The SVG
   renderer of lottie-web draws the effect: the layer with the index `ind`
   of the parameter "Take Matte From Layer", in the same composition, is an
   alpha matte of the layer, and it still draws at its own place.

## The prototypes

| Item | Prototype | Test file of `compat_test.go` | Pixels that differ from ThorVG, before and after |
| --- | --- | --- | --- |
| 8 | The blur and the shadow read the pixels outside the canvas from a copy that repeats the edge of the canvas | A rectangle and a circle that reach past the edges, with a blur in both directions, a blur along y, and a shadow | The blur 4.0% to 7.2% before, 0% after. The shadow was within the tolerance before |
| 7 | A cluster that the sink shapes multiplies its advance, and the offsets of its glyphs after the first, by the factor of ThorVG, and does not add the tracking | Noto Sans with the tracking -60, -30, 100, 150, 200 and 400, centered, aligned to the left and to the right | 4.6% to 6.1% before, at most 0.03% after |
| 6 | The sink draws a turned image pixel by pixel, as `rasterTexmapPolygon` does, in `okage_image.go` | An image of 16 by 12 pixels scaled by about 3.3 and turned to 50 and to -125 degrees, and an image that turns at its own size | 3.5% to 4.3% before, 0% after |

A prototype of item 2 drew Set Matte as lottie-web does. With the
equivalent track matte, it differed from ThorVG in at most 0.01% of the
pixels. The 21 layers of the corpus that use the effect changed only at the
antialiasing of a few edges, which the tolerance of D43 does not see. The
maintainers did not accept it, so okage does not have it.

With the parts 1 to 3, each of the 4,395 frames of the test against ThorVG
has the same share of pixels that differ as before, to three decimals. The
parts are in the sink, which the test against velato does not use, and the
test against velato gives the same calls for each of the 879 files.

## The decision

1. The `PixmapSink` draws the blur and the shadow of a layer as if the
   pixels at the edge of the canvas repeat outside it, as ThorVG does. The
   key "Repeat Edge Pixels" changes nothing, as in ThorVG.
2. The `PixmapSink` multiplies the advance of a cluster that it shapes with
   the fonts of `WithFontDatabase` by 1 plus the tracking over the ascent of
   the font, as ThorVG does. A glyph of the list `chars` keeps the tracking
   that it adds.
3. The `PixmapSink` draws an image that turns by an angle that is not a
   multiple of 90 degrees as `rasterTexmapPolygon` of ThorVG does, with the
   fade of its edge over one pixel of the image.
4. `lottie` skips the effect Set Matte, as velato and ThorVG do. Item 2 of
   "lottie" in the backlog is deleted.
5. Each part of the code has a comment that names D82.
6. The statuses of D41, D75 and D79 say "amended by D82", and the item of
   `lottie` in [`docs/DIFFERENCES.md`](../DIFFERENCES.md) that names D79
   names the parts 1 to 3.

## When upstream changes

If velato draws Set Matte, port its commit as `docs/UPSTREAM.md` says.

If a new version of ThorVG, as `github.com/xo/lottie` runs it, changes one
of the behaviors of "What ThorVG does", run the tests of
`test/lottie/compat_test.go`. If a test fails, change the part of the
prototype to follow ThorVG, or record a decision that keeps it.

## Why

D43 makes ThorVG the reference where velato leaves a feature to the sink.
The parts 1 to 3 each make the sink draw as ThorVG draws, and each one
makes a test against ThorVG pass that failed before. A text with tracking,
a blur at the edge of the canvas and a turned picture are common in Lottie
files outside the corpus.

Set Matte is not like them. Neither oracle draws it, so a test can only
compare it with a file that gives the same image in another way, and no file
of the corpus changes in a way that the tolerance of D43 sees.
