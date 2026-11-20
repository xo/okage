# D86. A mask can hold a rectangle and no pixels

Status: Decided, amends D60.

## Context

ivy and the `PixmapSink` of `lottie` clip much of what they draw to a
rectangle on the grid of the pixels: a cell of a spreadsheet, the box of a
layer, a part of a page. tiny-skia clips only with a `Mask`. So each such
clip makes a mask, fills the rectangle into it, and then each draw runs the
stage `mask_u8` on each chunk of pixels to multiply them by 255. Since
2026-11-20 a `Mask` holds only the rows and the columns that its paths
cover (D35, D60), but a clip of a rectangle still holds its pixels and
still runs the stage. Item 7 of "Make the pipeline faster" in
[`../BACKLOG.md`](../BACKLOG.md) left this step to the maintainers.

## The options

The maintainers looked at four options:

1. No change to okage. ivy draws such a clip without a mask, as
   `PixmapSink` did for a layer with the normal blend mode.
2. A clip rectangle on each draw call, as an argument or an option next to
   the mask.
3. A view of a rectangle of a `PixmapMut`, with a stride, that a caller
   draws into in place of the whole pixmap.
4. A `Mask` that holds only a rectangle and no pixels, made by one new
   function. Each method that takes a mask gives the same bytes as with a
   mask whose pixels are 255 inside the rectangle and 0 outside.

On 2026-11-21 the maintainers chose option 4. It changes no signature of
tiny-skia, and a caller that already passes a mask passes the new one in
the same place. It is a deliberate addition to the API of tiny-skia.

## The decision

1. `NewRectMask(width, height uint32, rect IntRect, opts ...Option)
   (*Mask, bool)` makes a mask of the size width by height that is 255 on
   the pixels of rect inside the mask and 0 on the other pixels. It returns
   false where `NewMask` returns false. A rect that has no pixel inside the
   mask gives the mask of `NewMask`, whose pixels are all 0.
2. A mask of `NewRectMask` holds its rectangle and one row of bytes, the
   row of the mask from column 0 to 15 columns past the right side of the
   rectangle. It holds no pixels.
3. `FillRect`, `FillPath`, `StrokePath` and `DrawPixmap` of `Pixmap` and
   `PixmapMut` draw only on the rows of the rectangle. On those rows the
   blitter splits each span into the chunks of 16 pixels of `maskBounds`
   (D60). A chunk with no pixel of the rectangle does not run. A chunk
   whose pixels are all inside the rectangle runs the pipeline without the
   stage `mask_u8`. That stage multiplies each channel by 255 and divides
   by 255, which gives the same bytes in lowp and in highp, so the stage
   changes nothing there. A chunk that holds a side of the rectangle runs
   the pipeline with `mask_u8`, which reads the row of item 2.
4. Item 3 keeps the stage for the chunks at the sides because of a fault of
   tiny-skia. In a chunk with a coverage that is not 0, `mask_u8` makes the
   source 0 on the pixels whose mask is 0, and the next stages still store
   those pixels. With `BlendModeSource`, for example, a pixel up to 15
   columns outside the rectangle becomes transparent. The bytes stay the
   same as the bytes of tiny-skia with the mask of pixels.
5. The blitter makes the same choices as for a mask of pixels: no
   `memset`, and no change from `SourceOver` to `Source` for an opaque
   paint. A fill of a path whose pixels are all outside the rectangle
   returns before the scan converter.
6. `ApplyMask` with such a mask clears the pixels outside the rectangle
   and keeps the pixels inside it, which are the bytes of the stages
   `load_mask_u8` and `destination_in` of lowp. `Knockout` reads a shape of
   255 inside the rectangle and 0 outside, and makes no pixels for it.
   The fast path of `DrawPixmap` (D53) reads a coverage of 255 inside the
   rectangle and 0 outside.
7. The mask turns into a mask of pixels, with the layout of D35, the first
   time a method reads or writes its pixels: `Data`, `DataMut`, `Take`,
   `FillPath`, `IntersectPath`, `Invert`, `ApplyTransfer`, `EncodePNG` and
   `SavePNG`. `Clear` makes it a mask of `NewMask`. `Clone` copies the
   rectangle and shares the row, and a write to the copy turns only the
   copy into pixels. So each other method of `Mask` keeps its meaning.
8. Tests compare the bytes of each draw method with a mask of
   `NewRectMask` and with the same mask in pixels, for many random
   rectangles, in both builds, in lowp and in highp, for each blend mode.
   They also test each method of item 7.

## Consequences

A clip of a rectangle on the grid of the pixels allocates one row in place
of its pixels, and its draws skip the stage `mask_u8` inside the
rectangle. `PixmapSink` of `lottie` makes its masks of a rectangle with
`NewRectMask`, and draws the same bytes. This decision amends D60: a
`Mask` now has two forms, the rows and the columns of D35 and D60, and the
rectangle of this decision.
