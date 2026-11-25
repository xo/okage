# D88. The blur and the color matrix of the filters get a public API

Status: Decided, amends D5 and D79.

## Context

On 2026-11-19 ivy asked okage for the blur and the color matrix of the
filters of `svg`, on a `Mask` and on a `Pixmap`. Item 1 of "Filters" in
[`../BACKLOG.md`](../BACKLOG.md) holds the request. Today ivy does this
work with code of its own, in `internal/drawingml` of ivy:

1. `blur.go` fills the silhouette of a shape into an `okage.Mask`. It wraps
   the bytes of the mask in an `image.Alpha`, and blurs them with three box
   blurs of its own in each direction. A glow grows the silhouette before
   the blur, and a soft edge makes it smaller. The device of ivy then fills
   the mask with a color. The shadows, the glows and the soft edges of
   DrawingML draw in this way.
2. `recolor.go` changes the colors of an `image.NRGBA`, or of a pixmap that
   ivy demultiplies first, for the picture effects of Word. It applies four
   steps in order. The contrast and the brightness go through a table of
   256 values. The gray takes the luminance with the weights 0.299, 0.587
   and 0.114. The black and white compares the luminance with a threshold.
   The duotone mixes two colors in a straight line by the luminance.

okage already holds this work in the filters of `svg`, the port of
`src/filter/` of resvg:

1. `internal/boxblur` is the box blur of resvg: five box blurs in each
   direction. `svg` takes it for a standard deviation of 2 or more, and the
   IIR blur of `svg/resvg_filter_iir_blur.go` for a smaller one. An IIR blur
   (infinite impulse response) is a blur that runs a recursive filter along
   each row and each column. `lottie` takes the box blur for the Gaussian
   blur and the drop shadow of a layer (D79). For the shadow, `lottie`
   copies the alpha of the layer into a buffer of RGBA bytes, because the
   box blur reads only RGBA.
2. `svg/resvg_filter_color_matrix.go` applies the four kinds of
   `feColorMatrix`: a matrix of 20 numbers, saturate, hue-rotate and
   luminanceToAlpha.
3. `svg/resvg_filter_component_transfer.go` applies the five transfer
   functions of `feComponentTransfer` to each channel: identity, table,
   discrete, linear and gamma.

None of this is public, so a caller cannot use it on its own masks and
pixmaps.

## Facts that hold for each option

1. The box blur blurs each channel alone. So a blur of one channel gives
   the same bytes as the alpha channel of a blur of RGBA.
2. The box blur reads 0 outside the image. Take the smallest box that holds
   each pixel that is not 0. Grow it by the sum of the radii of the boxes of
   the blur. A blur of that box gives the same bytes as a blur of the whole
   image.
3. The IIR blur reflects the image at its edges. So it has no such
   shortcut, and it blurs the whole image.
4. The color matrix and the transfer functions read colors that are not
   premultiplied. On a pixmap, `svg` demultiplies the pixels, applies the
   filter, and multiplies the pixels again.
5. A matrix covers the gray and the duotone of ivy. The table of the
   contrast and the brightness is a linear transfer function. The black and
   white is a discrete transfer function of two values after the gray. So
   the matrix alone does not cover `recolor.go`, but the matrix and the
   transfer functions together do.
6. resvg truncates each channel after a color filter, and ivy rounds it.
   So a channel of ivy can change by 1.
7. ivy blurs with three boxes, and resvg with five. Five boxes are closer
   to a Gaussian blur. With the same standard deviation, the edge of a
   shadow of ivy changes a little.

## The options

### Option 1. No change to okage

ivy keeps `blur.go` and `recolor.go`.

- Cost: none in okage.
- Trade-off: okage and ivy each keep a blur, and `lottie` keeps its copy
  into RGBA for the alpha of a shadow. The blurs of ivy and of `svg` differ.

### Option 2. Methods in the root package

The root package gets methods, and imports the blur from an internal
package:

```go
func (m *Mask) Blur(sigmaX, sigmaY float64, opts ...BlurOption)
func (p *Pixmap) Blur(sigmaX, sigmaY float64, opts ...BlurOption)
func (p *Pixmap) ApplyColorMatrix(m ColorMatrix, opts ...ColorOption)
func (p *Pixmap) ApplyComponentTransfer(r, g, b, a TransferFunction, opts ...ColorOption)
```

The IIR blur, the color matrix and the transfer functions move from `svg`
into internal packages, as the box blur moved for D79.

- Cost: about 2 days of one agent. The code moves, and the API and its
  tests are new.
- Trade-off: a caller finds the methods on the types that it already has.
  A `Mask` method can keep the rows and the columns of D35 and D60. But the
  root package is the port of tiny-skia, and tiny-skia has no filters. This
  is a deliberate addition to the API of tiny-skia, as D86 is, and it puts
  code of resvg into the package of tiny-skia.

### Option 3. A new public package `filter`

A new package `filter`, at `github.com/xo/okage/filter`, holds the filters
of resvg that a caller can apply alone. They are the box blur, the IIR blur,
the color matrix and the transfer functions. Its functions take an
`*okage.Mask` or an `*okage.Pixmap`. `internal/boxblur` moves into it. `svg`
and `lottie` call it. A file of a port keeps the name of its file in
`src/filter/` of resvg, such as `box_blur.go` and `color_matrix.go`. The
functions that resvg does not have go in files that start with `okage_`.

```go
package filter

// Option configures one call of a function of this package.
type Option func(*options)

// Blur blurs the premultiplied pixels of p in place, as feGaussianBlur of
// resvg after resolve_std_dev. sigmaX and sigmaY are the standard
// deviations in pixels. A sigma below 0.05 does not blur on its axis.
// A sigma of 2 or more on either axis takes the box blur, and a smaller
// one the IIR blur.
func Blur(p *okage.Pixmap, sigmaX, sigmaY float64, opts ...Option)

// BlurMask blurs the values of m in place, as Blur blurs the alpha of a
// pixmap.
func BlurMask(m *okage.Mask, sigmaX, sigmaY float64, opts ...Option)

// WithBoxBlur makes Blur and BlurMask take the box blur for each sigma
// above 0, as lottie does (D79).
func WithBoxBlur() Option

// WithLinearRGB makes ApplyColorMatrix and ApplyComponentTransfer work in
// linearRGB, as color-interpolation-filters="linearRGB" does. Without it,
// they work in sRGB.
func WithLinearRGB() Option

// ColorMatrix is one of the four kinds of feColorMatrix.
type ColorMatrix struct{ /* private */ }

func NewColorMatrix(m [20]float32) ColorMatrix
func SaturateMatrix(v float32) ColorMatrix
func HueRotateMatrix(degrees float32) ColorMatrix
func LuminanceToAlphaMatrix() ColorMatrix

// ApplyColorMatrix demultiplies the pixels of p, applies m, and
// multiplies them again, as resvg does.
func ApplyColorMatrix(p *okage.Pixmap, m ColorMatrix, opts ...Option)

// ApplyColorMatrixNRGBA applies m to pixels that are not premultiplied,
// in place.
func ApplyColorMatrixNRGBA(img *image.NRGBA, m ColorMatrix, opts ...Option)

// TransferFunction is one of the five transfer functions of
// feComponentTransfer. The zero value is the identity.
type TransferFunction struct{ /* private */ }

func TransferTable(values []float32) TransferFunction
func TransferDiscrete(values []float32) TransferFunction
func TransferLinear(slope, intercept float32) TransferFunction
func TransferGamma(amplitude, exponent, offset float32) TransferFunction

func ApplyComponentTransfer(p *okage.Pixmap, r, g, b, a TransferFunction, opts ...Option)
func ApplyComponentTransferNRGBA(img *image.NRGBA, r, g, b, a TransferFunction, opts ...Option)
```

The details of the API:

1. `BlurMask` takes the bytes of the mask with `DataMut`, so the mask has
   all its pixels after the blur. A mask of `NewRectMask` (D86) turns into
   pixels in the same way.
2. With the box blur, `Blur` and `BlurMask` blur only the box of fact 2,
   cut to the image. This gives the same bytes as a blur of the whole
   image, in less time for a small shape on a large mask.
3. `BlurMask` blurs one channel. So `lottie` blurs the alpha of a drop
   shadow with a quarter of the memory and the same bytes (fact 1).
4. `ColorMatrix` keeps the kind of resvg, so saturate and hue-rotate keep
   the alpha and the arithmetic of resvg.
5. `ApplyColorMatrixNRGBA` and `ApplyComponentTransferNRGBA` serve a
   picture that ivy decodes into an `image.NRGBA`. They skip the round trip
   through premultiplied bytes, which loses precision where the alpha is
   low.
6. No function returns an error. A sigma that is negative or NaN does not
   blur on its axis, as in resvg. A value out of range is clamped, as in
   resvg. So the package adds no sentinel error (D21).
7. The options configure one call, and hold no package variable (D20).

What the package does not hold:

1. A region of a pixmap to blur. `lottie` blurs a region and repeats the
   edge pixels outside the canvas, as ThorVG does (D82). It keeps that code.
2. The other primitives of the filters, such as morphology, lighting and
   turbulence. They read a filter region and inputs of the tree of `svg`,
   and ivy did not ask for them.

- Cost: about 2 to 3 days of one agent. The code moves, the API and its
  tests are new, and `svg` and `lottie` call the new package.
- Trade-off: the root package stays the port of tiny-skia. A program that
  blurs or changes colors imports a small package, and not all of `svg`.
  But a caller must find a second package, and the functions are not
  methods. The name `filter` is new, and a later decision can add more
  primitives to it.

### Option 4. Functions in `svg`

`svg` exports functions on the types that it already has:

```go
func BlurPixmap(p *okage.Pixmap, sigmaX, sigmaY float64)
func BlurMask(m *okage.Mask, sigmaX, sigmaY float64)
func ApplyColorMatrix(p *okage.Pixmap, kind ColorMatrixKindFilter)
func ApplyComponentTransfer(p *okage.Pixmap, fe *ComponentTransferFilter)
```

- Cost: about 1 day of one agent. No code moves.
- Trade-off: the functions take the types of the tree of usvg, which a
  caller outside SVG does not have. A program that only blurs imports all
  of `svg`, with its parser and its text layout. That costs size most of
  all with TinyGo. `lottie` already imports `svg` for its fonts, and ivy
  imports it for SVG, so neither pays more.

## What holds for options 2, 3 and 4

1. Same bytes (D16): the functions run the code that `svg` runs for
   `feGaussianBlur`, `feColorMatrix` and `feComponentTransfer`. The code is
   scalar, so the scalar build and the SIMD build give the same bytes. The
   images of the tests of resvg do not change, and the tests of `lottie`
   give the same bytes.
2. Tests: a test compares `BlurMask` with the alpha of `Blur`, byte for
   byte, for random shapes and sigmas. Another test compares the blur of
   the box of fact 2 with a blur of the whole image. A test compares each color
   function on a pixmap with the same filter in an SVG file that `svg`
   renders.
3. TinyGo (D25): the code uses no `reflect`, and the blur allocates its
   buffer once for each call, as resvg does. The blur is not part of the
   pipeline, so hard rule 9 does not apply to that buffer.

## The recommendation

The recommendation is option 3, with the color matrix and the transfer
functions. The root package stays the port of tiny-skia. ivy can then delete
`blur.go` and most of `recolor.go`. `lottie` blurs the alpha of a shadow
without its copy into RGBA. `svg` keeps its bytes. The transfer functions
cost about 100 lines more, and without them ivy keeps its table and its
threshold.

## The decision

On 2026-11-25 the maintainers chose answer 1 of question 39: option 3, with
the color matrix and the transfer functions. The statuses of D5 and D79 say
"amended by D88". The table "Layout" of `AGENTS.md` names `filter` in place
of `internal/boxblur`.

The package holds the API of option 3, with these details:

1. The files `box_blur.go`, `iir_blur.go`, `color_matrix.go` and
   `component_transfer.go` moved from `internal/boxblur` and `svg`.
   `mod.go` ports the parts of `src/filter/mod.rs` that they use, and `svg`
   keeps its own port of that module for the other primitives.
   `okage_filter.go` holds the API, and `okage_blur_alpha.go` the box blur
   and the IIR blur of one channel for `BlurMask`. The package has the MIT
   license of resvg in `filter/LICENSE` (D13).
2. `svg` calls `Blur` for `feGaussianBlur` and `feDropShadow`,
   `ApplyColorMatrix` for `feColorMatrix` and `ApplyComponentTransfer` for
   `feComponentTransfer`. `svg` still converts the colors into the color
   space of the primitive itself, so it does not pass `WithLinearRGB`.
3. `lottie` calls `Blur` with `WithBoxBlur` for the Gaussian blur of a
   layer. It calls `BlurMask` with `WithBoxBlur` for the alpha of a drop
   shadow, which it no longer copies into RGBA bytes.
4. `WithLinearRGB` also applies to `Blur`, as
   `color-interpolation-filters` applies to `feGaussianBlur` in resvg.
   `BlurMask` ignores it, because the conversion does not change the alpha.
5. `Blur` and `BlurMask` take the sigmas as float64, and round them to
   float32, as resvg holds them. A sigma above 2^20 blurs as 2^20. Above
   about 1.4 x 10^9 the size of a box of resvg overflows an int32. A blur
   with a sigma of 2^20 has boxes more than a million pixels wide.
6. The box blur of a column starts its index on the right at the minimum
   of the radius and the height, times the width. Upstream starts it at the
   radius times the width, which can overflow an int of 32 bits. Both
   indexes are past the column, where the blur reads 0, so the bytes are the
   same.
7. The zero `ColorMatrix` changes no color, and the zero `TransferFunction`
   is the identity. On a pixmap, `ApplyColorMatrix` and
   `ApplyComponentTransfer` still demultiply and multiply the pixels again,
   as resvg does for each color primitive.

What ivy needs is covered:

1. `blur.go` fills the silhouette into an `okage.Mask`, and can call
   `BlurMask` on it in place of its three box blurs. The glow and the soft
   edge stay in ivy, because they change the silhouette before the blur.
2. `recolor.go` can apply its four steps on an `image.NRGBA` with
   `ApplyComponentTransferNRGBA` and `ApplyColorMatrixNRGBA`. The contrast
   and the brightness are `TransferLinear(k, 0.5 - k/2 + b)`, with the
   factor k and the brightness b of ivy. The gray is a matrix with the
   weights 0.299, 0.587 and 0.114 in each of the rows R, G and B. The black
   and white is `TransferDiscrete` after the gray. Two values put the
   threshold at 0.5. 256 values map each byte to its own value, so they put
   the threshold at any byte. The duotone is a matrix whose rows R, G and B
   take the gray of the channel R, times the difference of the two colors,
   plus the first color. The examples of the package show the gray, a
   duotone, the contrast and the black and white.

The tests:

1. The images of the tests of resvg do not change. All 1,798 test files of
   resvg, rendered at the scales 0.21, 0.5, 1 and 2.3, give the same bytes
   as before, for 7,192 images in all.
2. The tests of `lottie` against ThorVG for the blurs, the shadows, the
   qualities and the edges give the same numbers as before. The corpus of
   `TestThorVG` was not on the machine.
3. A test compares the blur of the box of fact 2 with a blur of the whole
   image, for RGBA and for one channel. A test compares `BlurMask` with the
   alpha of `Blur`, with the box blur and with the IIR blur.
4. A test compares each function on a pixmap with the same primitive in an
   SVG file that `svg` renders, in sRGB and in linearRGB. Another test
   compares each function on an `image.NRGBA` with the same function on an
   opaque pixmap.

The speed of `svg` on 2026-11-25, as the median of 10 runs of each build in
turns, for a canvas of 800 by 800 pixels:

| Scene | Before | After |
| --- | --- | --- |
| a blurred circle of radius 40, `stdDeviation="6"` | 39.5 ms | 13.4 ms |
| a drop shadow of a rectangle of 120 by 60, `stdDeviation="5"` | 51.8 ms | 26.1 ms |
| a blurred rectangle that fills the canvas | 41.9 ms | 41.6 ms |
