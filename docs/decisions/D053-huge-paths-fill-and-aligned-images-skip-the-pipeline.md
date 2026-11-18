# D53. Huge paths fill, and aligned images skip the pipeline

Status: Decided.

ivy draws metafiles, PDF files and DjVu files with okage. On 2026-11-18 it
asked for two changes, and a maintainer asked for them in okage:

1. An antialiased fill or stroke of a path with a point near 1e12 pixels
   panicked with "slice bounds out of range" in `slice16AtXY`. ivy left out
   such paths to work around it.
2. `DrawPixmap` took about 40 percent of the time of a page of a DjVu file,
   and 62 percent of the time of a page of an Illustrator file. ivy draws a
   page of a DjVu file at a move of whole pixels, and it draws each
   transparency group of a PDF file back onto its page with the identity
   transform, an opacity, a blend mode and a mask.

## The fault of huge paths

`fill_path` of `src/scan/path_aa.rs` rounds the bounds of the path out to an
`IntRect`, and each side saturates to the range of an i32. Two faults follow:

1. The width saturates too. A path from x = -1.79e10 to x = 1.67e9 gets the
   left side -2147483520 and the width 2147483647, so its right side is 127.
   The rows and the columns past 127 are lost, and the blitter of the
   supersampling gets spans that are wider than its runs. `AlphaRuns` then
   panics.
2. `fill_path_impl` of `src/scan/path.rs` shifts the top and the bottom left
   by 2 for the supersampling. A top below about -2^29 wraps around to a
   positive row, such as 512. The scan converter then starts below the clip,
   and the pipeline reads a row past the end of the pixmap.

tiny-skia 0.12.0 has the same faults. A Rust program that fills such a
quadrilateral on a pixmap of 32 by 32 with tiny-skia 0.12.0 panics with
"range start index 4096 out of range for slice of length 1024" in
`src/pipeline/mod.rs`. The fill without antialiasing does not panic, but
`conservative_round_to_int` of `src/scan/path.rs` fails for a path that is
wider or taller than 2^31 pixels, so that fill draws nothing.

## The decision for huge paths

1. `fillPathPath` and `fillPathPathAA` clamp each side of the bounds of the
   path to the range -2^26 to 2^26 before they round the bounds (the function
   `scanBounds`). A side of 2^26, shifted left by 3 for the grid of 8 by 8
   samples of D54, fits in an i32, and so does a width of 2^27.
2. The clip of a fill is never larger than 32767 pixels, because the painter
   splits a larger pixmap into tiles. A side past 2^26 is out of the clip
   before and after the clamp. The edges are still clipped to the clip as
   before. So the clamp gives the same bytes for each fill that worked
   before, and changes only these fills:
   - an antialiased fill that panicked, or that left out rows or columns,
   - a fill without antialiasing of a path wider or taller than 2^31 pixels,
     which drew nothing.
3. The edge clipper still finds the point where an edge enters the clip in
   float32, as upstream does. Past about 1e12 pixels that point can be off
   by a pixel or more. The tests compare a diagonal edge only up to 1e12.
4. `scan_path_bounds_test.go` fills and strokes such paths. It compares a
   fill whose far sides are at 3e8 to 1e30 pixels with the same fill whose far
   sides are just out of the pixmap, and the bytes are the same. The fuzz
   test `FuzzFillPathHuge` fills and strokes random paths with huge
   coordinates.

This is a difference from upstream, in the list of `README.md`.

## The decision for aligned images

An image is aligned to the pixels when the transform of the pattern only
moves it by whole pixels. Its pixels then map one to one onto the pixels of
the destination.

1. The raster pipeline has one stage that upstream does not have,
   `stageGatherAligned`. `Pattern` pushes it in place of the stages
   `SeedShader`, `Transform` and `Gather` when these are all true:
   - The spread mode is `SpreadModePad`.
   - The inverse of the transform of the pattern only moves by whole pixels,
     at most 2^21 pixels on each axis.

   The stage leaves the same values in the registers as the three stages, so
   each stage after it, each blend mode and each mask gives the same bytes.
   The sum of a coordinate and a move of up to 2^21 each is less than 2^23,
   so its pixel center is exact in a float32. For a pixel past 2^21 the stage
   computes as the three stages do. `TestGatherAligned` compares the stage
   with the three stages bit for bit on random states.
2. The pipeline stays high precision (highp). The low precision pipeline
   (lowp) is not used for an image. Upstream runs a pattern in highp,
   because `gather` has no lowp stage, and lowp computes with integers of 16
   bits that round in other places. lowp gives other bytes.
3. `DrawPixmap` draws an aligned image without the pipeline when the blend
   mode is `SourceOver`, the pixmap needs no tiles, and the mask is nil or
   has the size of the pixmap. It does the same float32 operations on each
   pixel as the stages `gather`, `scale_1_float`, `mask_u8`, `load_dst`,
   `source_over` and `store`. It handles three kinds of pixels faster:
   - A pixel with a mask of 0 keeps its bytes.
   - A pixel whose source is 0 in each channel keeps its bytes.
   - A pixel whose source alpha is 255, with no opacity and a mask of 255 or
     no mask, takes the bytes of the source.

   `TestDrawPixmapAlignedBytes` tests the two facts that these need: a byte
   loads and stores as the same byte, and an alpha of 255 loads as exactly 1.
   `DrawPixmap` reads the pixels in groups of eight that start where the
   groups of the pipeline start, so that an image that shares its bytes with
   the destination gives the same bytes too.
4. `DrawPixmap` fills the same pixels as the pipeline. With the identity
   transform, upstream rounds the rectangle of the image with `Rect::round`,
   which moves a negative side by one pixel and keeps a width of at least
   one. So an image of one column at x = -1 fills the column x = 0 with its
   padded pixels. The fast path does the same.
   `TestDrawPixmapAlignedEdges` draws small images at each position near the
   edges.
5. Every other draw takes the pipeline as before.
   `TestDrawPixmapAlignedSameAsPipeline` and `TestDrawPixmapAlignedExhaustive`
   compare the two paths byte for byte, with and without a mask and an
   opacity.
6. okage adds no public API for this.

On 2026-11-18 the benchmarks of `bench_draw_pixmap_test.go` gave these times
for one draw onto a page of 1600 by 893 pixels, with `-cpu 1`. Each time is
the best of three runs, in the scalar build and in the SIMD build:

| Benchmark | Before | After | Before, SIMD | After, SIMD |
| --- | --- | --- | --- | --- |
| `DrawPixmapLayer`: a group with a mask and the opacity 0.5 | 30.2 ms | 12.0 ms | 30.2 ms | 12.2 ms |
| `DrawPixmapLayerMultiply`: the same with the blend mode Multiply | 30.7 ms | 20.3 ms | 30.7 ms | 20.3 ms |
| `DrawPixmapLayerNoMask`: a group with the default paint | 27.9 ms | 13.0 ms | 28.3 ms | 13.1 ms |
| `DrawPixmapOpaque`: an opaque image, moved by whole pixels | 30.0 ms | 4.5 ms | 30.9 ms | 4.5 ms |
| `DrawPixmapScaled`: an image with a scale, which takes the pipeline | 29.4 ms | 29.4 ms | 29.7 ms | 29.8 ms |

These are not differences from upstream, because the bytes are the same.
