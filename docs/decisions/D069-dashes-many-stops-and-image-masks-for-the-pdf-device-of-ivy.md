# D69. Dashes, many stops and image masks for the PDF device of ivy

Status: Decided, amends D68.

The PDF device of ivy maps the calls of its display list onto a
`pdf.Surface` (D62 item 11, D68). On 2026-11-19 ivy asked okage for three
more changes, in items 2, 3 and 4 of "Work for okage" of the backlog of
ivy. This decision records the API and the behavior of each change, and the
results of their checks. On 2026-11-19 the maintainers answered question 29
with the first answer of each change: the moved offset, a tree of stitching
functions, and an image mask only for a stencil of 0 and 255 with a color.

## The dash of a stroke

The device writes a stroke of ivy, an `okage.Stroke`, as a stroke of `pdf`.
The dash array and the dash offset of a `StrokeDash` are private, as they
are in tiny-skia. So the device cuts a dashed path into dashes with
`Path.Dash`, and writes the dashes as one path. A viewer then cannot draw the
caps and the joins of the dashes as PDF draws them.

1. `path.StrokeDash` gets two methods, which `okage.StrokeDash` shares:
   - `Array` returns the dash array. It is the slice that `NewStrokeDash`
     took, as the struct keeps it.
   - `Offset` returns the dash offset. `NewStrokeDash` moves the offset into
     the range from 0 to the sum of the dash array, as `adjust_dash_offset`
     of tiny-skia does. It keeps only that value. So `Offset` can differ
     from the offset of the caller, but it gives the same dashes.
2. The device makes a `pdf.StrokeDash` from the two values. `pdf` writes the
   dash with the operator `d`, as krilla does. `NewStrokeDash` takes the
   offset that `Offset` returns and keeps it, so the stroker of `pdf` gives
   the same bbox for the stroke.

The methods add no state, and the bytes of the renderer do not change.

## A gradient with more than 257 stops

krilla writes the function of a linear or a radial gradient with more than 2
stops as one stitching function. It holds one exponential function for each
pair of stops. MuPDF 1.28 reads at most 256 functions of a stitching function. For
more, it reports "too many sub-functions in stitching function" and draws no
shading. A gradient with more than 257 stops after the padding of krilla has
more than 256 functions. The device of ivy draws such a gradient with the
raster fallback, for example for `hayro/pdf/pattern-shading-type2-many.pdf`
of the corpus of the review of ivy.

1. When a stitching function has more than 256 functions, `pdf` puts them
   into a tree of stitching functions. Each stitching function of the tree
   holds at most 256 functions. The domain of a stitching function in the
   tree goes from the offset of its first stop to the offset of its last
   stop. Its parent encodes its interval to that domain. So each exponential
   function gets the same input as in the single stitching function of
   krilla, and the tree draws the same gradient. A tree of depth 2 holds
   65,536 functions.
2. A stitching function of at most 256 functions does not change, so every
   gradient that MuPDF reads keeps the bytes of krilla.
3. The shading of the opacities of the stops uses the same tree.

The backlog of ivy offers a sampled function of type 0 as another answer. A
sampled function holds the colors at points that are evenly spaced, so it
moves a hard stop, where two stops have the same offset. The tree keeps each
stop exactly, and it uses only the functions that krilla writes.

The comment of the change in `graphics_shading_function.go` starts with
`okage:` and names this decision. The tree is in `pdf/ivy_function.go`.

## A stencil of 0 and 255 with a color

`Surface.FillStencil` writes a stencil of 8 bits as a gray image in a soft
mask (D68 item 5). The glyph of a Type 3 font is often an image mask of 1
bit. ivy gives it to `FillStencil` as a stencil of 0 and 255.

1. If the fill of the surface is a color and each pixel of the stencil is 0
   or 255, `FillStencil` writes an image mask. This is an image XObject with
   `ImageMask` true and 1 bit for each pixel. The content stream sets the
   fill color, and the opacity of the fill in the graphics state, and draws
   the image mask with `Do`. The image mask is placed as `DrawImage` places
   an image.
2. A stencil with another value, or a fill that is not a color, keeps the
   soft mask of D68. An image mask of 1 bit cannot hold the coverage of an edge
   that is anti-aliased, and an image mask paints only the current color.
3. `Interpolate` true writes the entry `Interpolate`, as for an image.

## The checks

On 2026-11-19 the results were these:

1. `TestIvyAdditions` of `test/krilla` has two more scenes. The scene
   `shader_many_stops` fills a linear gradient and a radial gradient with
   600 stops, and a linear gradient with 300 stops and an alpha. MuPDF
   1.28.6 and Poppler 26.11 differ from okage in 0 of its 40,000 pixels.
   Without the tree, MuPDF reports the error above. The scene
   `stencil_binary` fills a stencil of 0 and 255 with a color with an
   alpha. It differs in 0 pixels in each viewer.
2. In a copy of ivy with no limit on the stops, `TestPDFEnc` of ivy wrote
   `hayro/pdf/pattern-shading-type2-many.pdf` with the tree. MuPDF drew the
   written page within a mean difference of 1.45 of ivy, and Poppler within
   3.18. With okage v0.17.0, MuPDF reported the error above.
3. In the same test, `repo/pdf/font-type3-widths-with-matrix.pdf` became 22
   image masks and no soft mask. The mean difference at 72 dots for each
   inch changed little. It went from 16.71 to 16.69 through ivy and from
   23.09 to 22.98 through MuPDF. It stayed at 38.16 through Poppler. The
   stencils that ivy gives are not the image masks of the file. For
   example, the glyph `l` is an image mask of 4 by 10 pixels in the file.
   The stencils that ivy gives have sizes such as 12 by 14 pixels. So the difference comes
   from the stencils of ivy, not from the soft mask.
4. `pdf/ivy_test.go` checks the objects of the image mask and the tree.
   `path/dash_test.go` checks the two methods.
5. `test/cmd/perf -pages 3 -hash` of ivy gave the same hashes for the 269
   documents with okage v0.17.0 and with this change.

## When upstream changes

Follow "When upstream changes" of D62. If tiny-skia gives the dash array
and the offset, use its methods, and remove these. If krilla writes the
functions of many stops in another way, or writes image masks, a maintainer
decides which one okage keeps.

When the maintainers decide this decision, the status of D68 gets "amended
by D69".
