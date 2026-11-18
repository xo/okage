# D54. `Paint` takes a finer coverage and stroke adjustment

Status: Decided, amends D16.

ivy draws the pages of PDF, Office and ebook files with okage. A visual
review of ivy against Cairo, MuPDF and Poppler on 2026-11-17 found three
faults that come from the renderer:

1. The edges of small text have other grays. okage computes the coverage of
   a pixel from a grid of 4 by 4 samples, as tiny-skia does
   (`SUPERSAMPLE_SHIFT` of `scan_path_aa.go`). The coverage of an edge moves
   in steps of a quarter of a pixel. For a page of text at 96 dpi, the mean
   difference of a pixel of okage from a pixel of Cairo is 0.064. A grid of 8
   by 8 samples gives 0.052, and one of 16 by 16 gives 0.051. ivy fills small
   glyphs at twice their size and averages the samples (ivy D28).
2. Thin lines are light. A line of 1 pixel or less that lies across the
   border of two pixels spreads over two light pixels. MuPDF and Poppler
   draw such a line dark. PDF calls their method automatic stroke
   adjustment, the entry SA of a graphics state.
3. A thin horizontal or vertical line in a small image draws nothing. ivy
   renders an SVG image into a pixmap that is only as large as the image. For
   a straight line with a stroke under 1 pixel, that pixmap is 2 pixels high,
   or less.

On 2026-11-18 a maintainer asked for a change in okage for each fault. The
first two are new features that a program turns on. The third is a fix.

## The decision

1. `Paint.Coverage`, of the new type `Coverage`, sets the grid of samples of
   an antialiased fill. `Coverage4x4` is the zero value and the default. It
   is the grid of tiny-skia, and it gives the same bytes as tiny-skia.
   `Coverage8x8` takes 8 by 8 samples in each pixel, so the coverage moves
   in steps of 1/64.
   - The coverage applies to `FillPath`, and to a stroke that `StrokePath`
     fills. It does not apply to `FillRect`, to a hairline or to a `Mask`.
     These compute their coverage in another way, or have no `Paint`.
   - The code is the code of tiny-skia, with the shift 3 in place of the
     shift 2. The shift is a field of the super blitter. `SCALE` and `MASK`
     are local values of `superBlitter.blitH`. The 4 by 4 grid takes the same
     steps as before, so its bytes do not change.
   - With 8 by 8 samples, the coordinates of the samples fit in 16 bits only
     for an area of 4095 pixels or less. A fill whose area on the pixmap is
     wider or higher takes the 4 by 4 grid. tiny-skia fills such an area
     without antialiasing at its own limit, and okage keeps that limit.
   - A fill with 8 by 8 samples took about 13 percent more time than the same
     fill with 4 by 4 samples in `BenchmarkFillPathAA8x8`, and about 23
     percent more for 800 small glyphs in `BenchmarkFillGlyphs8x8`. The fill
     at twice the size of ivy D28 adds about half the time of a page of
     text.
   - okage does not add a grid of 16 by 16 samples or an exact coverage. The
     grid of 16 by 16 gives little more than the grid of 8 by 8, and its
     coordinates fit in 16 bits only up to 2047 pixels.
2. `Paint.StrokeAdjust` turns on the stroke adjustment of PDF. It is false by
   default. When it is true, `StrokePath` does these steps:
   1. It dashes the path in user space, as before.
   2. It maps the path onto the device.
   3. It rounds the width of the stroke on the device to a whole number of
      pixels, at least 1. The width on the device is the width times the
      square root of the absolute determinant of the transform. This is the
      mean scale of the transform.
   4. It moves each horizontal or vertical line of the path, with its ends,
      onto the grid of the pixels. For an odd width, the line goes to the
      center of a pixel. For an even width, it goes to the border of two
      pixels. Then the edges of the stroke fall on the borders of the pixels.
      A point that ends a horizontal line moves only up or down, and a point
      that ends a vertical line moves only left or right. A line is
      horizontal or vertical when its ends differ by less than 1/64 of a
      pixel. A curve does not move, except at the points that it shares with
      such a line.
   5. It strokes the path on the device and fills the stroke. A stroke of 1
      pixel is filled too, and is not a hairline, so that its joins cover
      whole pixels.

   `FillPath` ignores `StrokeAdjust`. The PDF specification asks for a width
   that is no more than half a pixel from the width of the stroke, and at
   least 1 pixel, and for edges on the borders of the pixels. Step 3 and step
   4 give that for the lines that are horizontal or vertical on the device.
3. A hairline draws on a clip that is 2 pixels wide or high, or less. This
   is a fix of a fault of tiny-skia and of resvg.
   - The root cause is in `stroke_path_impl` of `scan/hairline.rs`. When the
     bounds of the path, with an outset of 1 or 2 pixels, are not inside the
     clip, it makes an inset clip, the clip less 1 pixel on each side, for
     the quick test of a curve. `IntRect::inset` gives `None` for a clip of 2
     pixels or less, and then the function returns and draws nothing. Skia
     keeps the empty inset clip, and clips each segment.
   - okage leaves out the inset clip and the outset clip in that case. Then
     each segment is clipped to the clip, and the hairline draws the pixels
     that it draws on a larger pixmap. The comment of the code starts with
     "Port (D54)".
   - resvg strokes with tiny-skia, and so it shares the fault. `svg` needs no
     change.
   - The fix changes only the pixmaps where tiny-skia draws nothing. The
     Rust oracle draws on pixmaps of 8 pixels or more, so it does not see
     the fix.
4. A test of each part compares bytes. The tests of the default compare the
   bytes of tiny-skia. The tests of `Coverage8x8` compare a hash of the
   bytes, the same in the scalar build and in the SIMD build. A test checks
   that a hairline on a pixmap of 1 or 2 rows, or of 1 or 2 columns, gives
   the pixels of the same hairline on a larger pixmap.
