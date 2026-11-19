# D68. `pdf` adds what the PDF device of ivy needs

Status: Decided, amends D62.

ivy keeps its own device for PDF output, which maps the calls of its display
list onto a `pdf.Surface` (D62 item 11). krilla does not have some features
of PDF that the device needs. Item 11 of D62 lists them, and item 10 adds a
raster fallback for a caller that is not `pdfsvg`. On 2026-11-19 a
maintainer asked for phase 18 of "The PDF writer" in
[`../PLAN.md`](../PLAN.md). This decision records the API of each addition,
and the result of the gate of phase 18. The maintainers accepted it on
2026-11-19, as the answer to question 28.

## The device of ivy

`internal/display/okage.go` of ivy draws the display list with okage. Its
calls show what the PDF device needs:

| Call of the display list | What the PDF device calls |
| --- | --- |
| `FillPath`, `StrokePath` and `FillGlyphs` with an `okage.Shader` | `Surface.PaintFromShader`, then `SetFill` or `SetStroke` and `DrawPath`. A shader that does not convert goes to `Surface.DrawRaster` |
| `StrokeAdjusted`, the stroke with `Paint.StrokeAdjust` (D54) | `Surface.PushStrokeAdjustment` |
| `FillMask`, a stencil of 8 bits with a shader | `Surface.FillStencil`, with the paint of the shader as the fill |
| `BeginGroup` with the opacity, the blend mode, `Isolated` and `Knockout` | `Surface.PushGroup` |
| `Mask` of a group, with `Backdrop` and `Transfer` (D56) | `Mask.WithBackdrop` and `Mask.WithTransfer` |
| a blend mode that PDF does not have, such as `BlendModeDestinationOut` | `Surface.DrawRaster` |

`ClipPath`, `Concat`, `Save`, `Restore` and `DrawImage` need nothing new.
They are `PushClipPath`, `PushTransform`, a mark on the stack of pushes, and
`DrawImage` with `PushOpacity`.

## The additions

Each addition is new API of `pdf`, in a file `pdf/ivy_*.go`. A change to a
file of the port has a comment that starts with `okage:` and names this
decision. Each change keeps the bytes and the hashes of every input of
krilla.

1. A transparency group. `Surface.PushGroup(Group)` starts one form XObject
   with a group of the transparency type and the flags `I` and `K`. `Pop`
   draws it with one graphics state that holds `CA`, `ca`, `BM` and
   `SMask`. krilla draws an opacity, a mask and an isolated group each as
   an XObject of its own, so a group that is not isolated, or that is a
   knockout group, does not keep its meaning there. The zero `Group` has
   the opacity 0, as the `Group` of ivy has.
2. A soft mask with a backdrop and a transfer function.
   `Mask.WithBackdrop(ColorRGB)` writes `BC`, for a mask of the type
   luminosity. The group of a mask is in the RGB color space, so the color
   is RGB. `Mask.WithTransfer(*[256]uint8)` writes `TR` as a sampled
   function of 256 samples of 8 bits, which holds the table of
   `okage.Mask.ApplyTransfer` exactly. Such a mask covers the content that
   it masks with its bbox, because Poppler 26.11 does not apply the
   transfer function outside the bbox of the group of the mask.
3. The extend rule. `LinearGradient` and `RadialGradient` get the field
   `Extend` of the type `ExtendMode`. Its zero value `ExtendBoth` is the
   `Extend [true true]` of krilla. `ExtendStart`, `ExtendEnd` and
   `ExtendNone` write the other flags. This is the decal spread mode (D55)
   and the gradient of `NewRadialGradientExtend` (D64).
4. A mesh. The paint `MeshGradient` holds triangles of `MeshVertex`, each
   with a point, a color and an opacity, and a transform. It is a shading
   of type 4 with coordinates of 32 bits, components of 16 bits and a flag
   of 8 bits, in a shading pattern. ivy splits the patches of types 6 and 7
   into triangles before it makes an `okage.MeshGradient` (D55), so the
   device needs no shading of type 5, 6 or 7. When a vertex has an opacity
   below 1, a second shading of type 4 in gray holds the opacities, as
   krilla does for the stops of a gradient. A fill then draws the opaque
   mesh in a group whose soft mask is that shading, because Poppler 26.11
   does not apply the soft mask of a graphics state to a shading of type 4.
   A stroke sets the soft mask in its graphics state, as krilla does for a
   gradient. A mesh with no triangle, a vertex with no color or a point
   that is not finite records `ErrPDFInvalidGradient`. Colors with
   different numbers of components record `ErrPDFMixedColorSpaces` (D66).
5. A fill through a stencil of 8 bits. `Surface.FillStencil(*image.Alpha,
   Size, interpolate)` fills a rectangle with the fill of the surface
   through a soft mask of the type luminosity. The mask holds the stencil as
   a gray image, so each value is its own luminosity.
6. Stroke adjustment. `Surface.PushStrokeAdjustment(bool)` writes `SA` in
   the graphics state of each operation until its `Pop`.
7. The conversion of a shader. `Surface.PaintFromShader(okage.Shader)`
   returns a `Paint` and an opacity, or false. A `SolidColor`, each linear
   gradient, a radial gradient with the spread mode pad or decal or of
   `NewRadialGradientExtend`, a sweep gradient with the spread mode pad,
   repeat or reflect, a `MeshGradient`, and a `Pattern` with the spread
   mode repeat or reflect convert. A radial gradient with the spread mode
   repeat or reflect does not convert, because krilla draws it with the
   mode pad. A sweep gradient with the decal mode and a `Pattern` with the
   spread mode pad or decal do not convert either. The tile of a reflected
   pattern is one image of four mirrored copies. The colors are RGB colors
   of 8 bits, which is the default `ColorSpaceLinear` of okage.
8. The raster fallback of D62 item 10. `Surface.DrawRaster(Rect, scale,
   draw)` follows `render` of `filter.rs` of krilla-svg. It finds the
   resolution of the rectangle on the page from the current transform,
   multiplies it by the scale, and makes a pixmap of at most 5000 pixels on
   a side. `draw` draws onto the pixmap with okage, and the surface draws
   the pixmap as an image into the rectangle.

## The parameters of the shaders

okage keeps the parameters of its shaders private, as tiny-skia does. The
conversion needs them, so the root package gives them:

1. `GradientStop.Position` and `GradientStop.Color`.
2. `LinearGradient.Parameters`, `RadialGradient.Parameters` and
   `SweepGradient.Parameters` return a `GradientParameters`: the points or
   the center, the radii, the angles, the stops as the shader draws them,
   the spread mode, the flags of `NewRadialGradientExtend` and the
   transform. A radial gradient keeps the two circles of its constructor
   in a new field of `gradientType`, because the focal gradient of
   tiny-skia keeps only a transform from which they do not come back. A
   linear gradient and a sweep gradient compute their points from the
   values that tiny-skia keeps.
3. `MeshGradient.Parameters` returns the triangles that `NewMeshGradient`
   kept, the opacity and the transform. The mesh keeps the points of its
   triangles for it.
4. `Pattern.Parameters` returns the pixmap, the spread mode, the quality,
   the opacity and the transform.

The bytes of each shader do not change.

## A panic of a degenerate linear gradient

`FuzzDocument` found a panic of `NewLinearGradient` that tiny-skia has too.
When the start and the end of a gradient are very close, with the spread
mode repeat or reflect, the gradient is the average color of its stops.
`average_gradient_color` of tiny-skia takes the width of each interval
between two stops. With stops that are not in order, a width is negative,
a channel of the average leaves 0..=1, and the unwrap of `Color::from_rgba`
panics. A position that is NaN is 0 after `NewGradientStop`, so it panics
only through the order. ivy makes gradients from the data of a file, so a
hostile file can reach it. On 2026-11-19 the maintainers decided this fix:

1. `averageGradientColor` gives the stops the positions that `newGradient`
   gives them: in the order of the list, a position below the position
   before it takes that position. Stops in order keep their positions, so
   every input that worked keeps the bytes of tiny-skia.
2. When rounding takes a channel of the average a little out of 0..=1,
   the channel is clamped, where tiny-skia panics.

The comments of the changes start with `okage:` and name this decision. The
radial, sweep and extended radial gradients do not call
`averageGradientColor`, and have no such panic. The Rust oracle and the
benchmarks of the gradients gave the same results after the fix.
`FuzzDocument` gives stops in any order and with NaN.

## The gate

`TestIvyAdditions` of `test/krilla` draws 19 scenes twice: with okage into
a pixmap, and with `pdf` into a file. MuPDF 1.28.6 and Poppler 26.11 render
the file at 72 dots for each inch. A pixel differs when a channel differs by
more than 32 (D31). A scene passes when at most 1 percent of its 40,000
pixels differ, and `qpdf --check` accepts each file. On 2026-11-19 the
results were these:

| Scene | MuPDF | Poppler |
| --- | --- | --- |
| a knockout group, isolated | 15 | 3,984, a fault of Poppler |
| a knockout group over a backdrop | 3 | 3,800, a fault of Poppler |
| a group with an opacity, a blend mode and a mask | 0 | 8 |
| a mask with a backdrop | 0 | 0 |
| a mask with a transfer function | 20 | 20 |
| a mask with both | 0 | 0 |
| colors with alpha | 0 | 0 |
| linear gradients, with each spread mode | 75 | 91 |
| a linear gradient with alpha | 0 | 0 |
| radial gradients, pad and decal | 142 | 128 |
| radial gradients of `NewRadialGradientExtend` | 418 | 441 |
| sweep gradients | 117 | 129 |
| meshes, with an opacity and turned | 83 | 211 |
| patterns, repeat and reflect | 0 | 0 |
| a stencil with a color | 0 | 0 |
| a stencil with a gradient | 0 | 0 |
| thin lines with stroke adjustment | 3,100, a fault of MuPDF | 0 |
| wide lines with stroke adjustment | 3,529, a fault of MuPDF | 580, another rule |
| the raster fallback | 0 | 698, a fault of Poppler |

The scene of `NewRadialGradientExtend` allows 1.5 percent. The edges of
its cones are edges of shadings, which no viewer anti-aliases, and the
viewers put them in other pixels than okage. The test logs the scenes of a
known fault and does not fail them:

1. Poppler draws a knockout group as a group without the flag `K`.
2. MuPDF ignores `SA`. Poppler rounds each edge of a line to the pixels, so
   a line of 1.6 units is 1 or 2 pixels wide by its place. okage rounds the
   width first, to 2 pixels (D54).
3. Poppler smooths an image that it scales up by less than 4 times, even
   with `Interpolate` false, and so the image of the fallback at its own
   size. The patterns and the stencils of the gate scale their pixels by 4
   or more for this reason.

`pdf/ivy_test.go` checks the objects of each addition, and `FuzzDocument`
makes the new calls.

## When upstream changes

Follow "When upstream changes" of D62. If krilla adds one of these
features, port it. If the result is the same, remove the addition and its
line in `README.md`. If it is not the same, a maintainer decides which one
okage keeps.
