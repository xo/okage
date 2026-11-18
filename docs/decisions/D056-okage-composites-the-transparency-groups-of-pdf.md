# D56. okage composites the transparency groups of PDF

Status: Decided.

ivy draws PDF and XPS with okage. PDF has soft masks, isolated groups,
groups that are not isolated, and knockout groups (ISO 32000-2, 11.4 and
11.6). tiny-skia has the pieces for an isolated group and a soft mask, but
not for the other two kinds of group. The maintainers decided on
2026-11-18 that okage adds the missing pieces as new public API in the root
package. ivy asked for them in its backlog, as item 3 of "Left by phase 3".

## What tiny-skia already gives

1. `MaskFromPixmap` with `MaskTypeLuminance` makes a soft mask of
   luminosity. Each value is the luminance of the demultiplied color, times
   the alpha. This is the same as a backdrop of black.
2. An isolated group draws into a transparent pixmap. `DrawPixmap` then
   draws that pixmap onto its parent with the blend mode, the opacity and
   the mask of the group.

The Rust oracle already compares both with tiny-skia. The new tests in
`transparency_test.go` pin them with values that are computed by hand.

## The new API

The file `transparency.go` holds the new API. It ports no upstream file.

1. `MaskFromLuminance(pixmap PixmapRef, backdrop Color, opts ...Option)
   *Mask` makes a soft mask of luminosity over an opaque backdrop of the
   color `backdrop`. This is the backdrop color BC of a soft mask of PDF.
   The luminance is linear, so the function adds the luminance of the
   backdrop times one minus the alpha of the pixel. The function ignores the
   alpha of `backdrop`, because the backdrop of PDF is opaque. A backdrop
   with a luminance of zero, such as `ColorBlack`, gives the same bytes as
   `MaskFromPixmap` with `MaskTypeLuminance`. The function uses the same
   steps in `float32` as `MaskFromPixmap` for that reason.
2. `(*Mask).ApplyTransfer(table *[256]uint8)` changes each value `v` of
   the mask to `table[v]`. This is the transfer function TR of a soft mask.
   A mask holds bytes, so a table of 256 values holds every transfer
   function exactly.
3. `(*Pixmap).RemoveBackdrop(backdrop, group PixmapRef)` and the same
   method of `PixmapMut` finish a group that is not isolated. The group
   starts as a copy of its backdrop, and its elements draw onto that copy.
   The alpha channel of `group` holds the alpha of the group alone. A device
   gets it when it draws each element a second time, onto a transparent
   pixmap. The method removes the backdrop from the result, with the
   formula of 11.6.3, C = Cn + (Cn - C0) * (a0/ag - a0). On premultiplied
   colors this is c = cn - c0 * (1 - ag), and the alpha is ag. A device then
   draws the result onto the backdrop with `DrawPixmap`.
4. `(*Pixmap).Knockout(element, backdrop PixmapRef, shape *Mask)` and the
   same method of `PixmapMut` draw one element of a knockout group. The
   element is drawn onto a copy of the initial backdrop of the group, and
   `shape` holds its coverage without its opacity. On premultiplied colors,
   the result is element + (result - backdrop) * (1 - shape). The zero
   `PixmapRef` is a transparent backdrop, for a knockout group that is
   isolated. A nil shape covers the whole pixmap, as a nil mask does in
   tiny-skia.

A device draws a knockout group that is not isolated with both methods.
It calls `Knockout` once on the copy of the backdrop, and once on the
pixmap that holds the alpha of the group alone, with a transparent
backdrop. At the end of the group, it calls `RemoveBackdrop`.

## The arithmetic

1. `RemoveBackdrop` and `Knockout` compute on integers. Each product of
   two bytes is divided by 255 and rounded to the nearest integer. No
   product falls half way between two integers, so the rounding has no
   ties. The bytes are the same in the scalar build, in the SIMD build and
   with TinyGo, and the functions do not allocate.
2. Each function clamps the alpha between 0 and 255, and each color between
   0 and the alpha, so that the pixels stay premultiplied. Rounding in the
   pipeline can give a pixel that is a little out of range.
3. `RemoveBackdrop` takes the alpha of the result from `group`, as PDF
   does. Over an opaque backdrop, a group with `SourceOver`, an opacity of
   1 and no mask gives the same bytes as its elements drawn straight onto
   the backdrop. Over a backdrop that is not opaque, the colors are the
   same and the alpha can differ by 1. The pipeline of tiny-skia rounds the
   alpha of each element, and the union of the two alphas does not round
   the same way.
4. A function that gets pixmaps or a mask of different sizes logs a warning
   to the logger of the pixmap and does nothing, as `ApplyMask` does.

## The differences from tiny-skia

tiny-skia has none of these functions, so none of them changes a result of
tiny-skia. Every input that worked before gives the same bytes. The one
overlap is `MaskFromLuminance` with a black backdrop, which gives the bytes
of `MaskFromPixmap`, and a test makes sure of it.

## What okage leaves to the device

1. The device chooses the shape of an element for `Knockout`. For a fill,
   it is the coverage of the path, from `Mask.FillPath`, times the clip.
2. The device draws each element twice in a group that is not isolated:
   once onto the copy of the backdrop, and once onto a transparent pixmap
   for the alpha of the group. okage has no separate channel for the shape
   or the alpha of a group.
3. A soft mask of a group that is not isolated blends with its backdrop
   color. The device fills the pixmap with the backdrop color, draws the
   group straight onto it, and calls `MaskFromPixmap`. For an isolated soft
   mask, it draws onto a transparent pixmap and calls `MaskFromLuminance`.
