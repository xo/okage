# D64. Radial gradients with the extend rule of PDF

Status: Decided.

ivy draws the radial shadings of PDF and PostScript with okage. These are
the shadings of type 3. ISO 32000-1, 8.7.4.5.4, gives their rule. The
shading has a circle for each s. Its center and its radius go from the start
circle at s = 0 to the end circle at s = 1. A point takes the color of the
largest s whose circle passes through the point and has a radius of at least
0. The key `Extend` holds two flags. Without the first flag, s cannot be
less than 0. Without the second flag, s cannot be more than 1. A point
without such an s is not painted.

The two-point conical gradient of tiny-skia takes the largest t of all, and
the spread mode then gives its color. When the circles nest and both flags
are true, the two rules agree. In the other cases they can differ. For
example, when the circles do not nest, a point whose largest t is more than
1 can also have a smaller t in 0..=1. PDF paints that smaller t.
`SpreadModeDecal` of D55 paints nothing there. Before this decision, ivy drew such a shading
as a texture or an image. The pages of `pdf` with `internal/hayro/render`
and of `ps` with `ps/internal/graphics` did this.

On 2026-11-18 a maintainer asked for this rule as new API of okage. It is
not a port of tiny-skia. The bytes of every input that worked before do not
change.

## The decision

1. A new constructor makes such a gradient:

   ```go
   func NewRadialGradientExtend(
       startPoint Point, startRadius float32,
       endPoint Point, endRadius float32,
       stops []GradientStop,
       extendStart, extendEnd bool,
       transform Transform,
   ) (Shader, bool)
   ```

   It returns a `RadialGradient`, so the gradient works with `FillPath`,
   `FillRect`, a `Mask`, each blend mode, each `ColorSpace`, `Transform` and
   `ApplyOpacity`. extendStart and extendEnd are the two flags of `Extend`.
   One shader thus covers the four combinations of the flags. The gradient
   is never opaque.
2. The t of a point is the largest t that meets three conditions:
   - The circle of t passes through the point.
   - The radius of the circle, `startRadius + t * (endRadius -
     startRadius)`, is at least 0.
   - t is in 0..=1, or less than 0 with extendStart, or more than 1 with
     extendEnd.

   The equation of the circles has two roots at most. The gradient tries the
   larger root first, then the smaller root. The color of t is the color of
   `SpreadModePad`: a t less than 0 takes the color at 0, and a t more than
   1 takes the color at 1. A point without such a t is transparent black.
   A NaN t is no t.
3. The constructor returns false when the stops are empty, when a radius is
   less than 0, when a point or a radius is not finite, when the transform
   is not invertible, or when the centers and the radii are nearly equal, as
   `NewRadialGradient` tests with `degenerateThreshold`. In the last case
   every circle is the same circle, and the gradient paints no area. Two
   circles of radius 0 with other centers give a gradient that paints
   nothing, as in PDF.
4. A gradient with one stop is not a `SolidColor`, as it is for
   `NewRadialGradient`. It has the color of the stop where a t exists, and
   it is transparent elsewhere.
5. The constructor maps the gradient into a space where the start center is
   the origin, and where the largest of the distance between the centers and
   the two radii is 1. With the end center `(cx, cy)` and the radii `r0` and
   `r0 + dr` in that space, a point `(x, y)` is on the circle of t when
   `a * t * t - 2 * b * t + c = 0`, with:

   ```text
   a = cx * cx + cy * cy - dr * dr
   b = x * cx + y * cy + r0 * dr
   c = x * x + y * y - r0 * r0
   ```

   The roots are `q / a` and `c / q`, with
   `q = b + sign(b) * sqrt(b * b - a * c)`. This form keeps its precision
   when `a` is near 0. When `|a|` is at most 1/4096, `a` is 0. The start
   circle then touches the end circle on the inside, and the radius changes
   as fast as the center moves. The equation has one root, `c / q`, and the
   other root, which is not finite, is no t. tiny-skia makes the same choice
   for a focal point on the end circle.
6. A new stage, `stageXYTo2PtConicalExtend`, computes the t and the mask of
   the lanes that have a t, in the mask of the context of a two-point
   conical gradient. It comes after the stages of D53 and D55, so that the
   other stages keep their values. Then the gradient uses the stages of
   tiny-skia: `stagePadX1` when the stops are evenly spaced, the stage of
   the colors, `stagePremultiply`, and `stageApplyVectorMask`, as the strip
   of a two-point conical gradient does.
7. Only the highp pipeline has the new stage, as only highp has the stages
   of a two-point conical gradient in tiny-skia, and as D55 does for its
   stages. The lowp pipeline of tiny-skia has no vector mask. A lowp stage
   thus needs a lowp mask stage too. A gradient of `NewRadialGradientExtend`
   always uses the highp pipeline.
8. The stage computes each lane in one loop on the lanes (D33), with
   `float32`. Each product that an addition or a subtraction follows has an
   explicit `float32` conversion (D16). The stage takes no vector type and
   calls `sqrtF32`. The scalar build and the SIMD build thus give the same
   bytes. The stage does not allocate.

## For ivy

The pages of `pdf` and of `ps` call the constructor for each radial shading
whose two radii are at least 0, with the six numbers of `Coords`, the two
flags of `Extend`, the stops that sample the function of the shading over
its `Domain`, and the transform of the shading. The texture and the image
are then not needed.

## The tests

tiny-skia has no such rule, so the Rust oracle cannot test it.

1. `shaders_radial_gradient_extend_test.go` computes the t of each pixel in
   `float64` with the rule of PDF, and compares each pixel of okage with the
   color of that t. A channel can differ by 1. The test skips a pixel whose
   t is too near a limit for `float32`: near 0 or 1, near a radius of 0, or
   near the edge of the cone of the circles. It covers 11 pairs of circles,
   each with the four combinations of the flags and two sets of stops. The
   pairs include circles that do not nest, circles that nest, concentric
   circles, equal radii, a start circle that touches the end circle on the
   inside, and two circles of radius 0. Other tests cover a transform, the
   opacity, one stop and the inputs that return false.
2. With both flags true and circles that nest, the gradient gives the
   colors of `NewRadialGradient` in the pad mode, within 1 in each channel.
3. A test checks a hash of the bytes of every case. The scalar build, the
   SIMD build, 386, and arm64, s390x and riscv64 in QEMU gave the same hash
   on 2026-11-18. These three architectures fuse a multiply and an add.
4. `test/shading` writes each case as a PDF file with a shading of type 3,
   renders it with MuPDF `mutool draw`, and counts the pixels that differ
   by more than 8 in a channel. On 2026-11-18, with MuPDF 1.28.6, at most
   281 of 40000 pixels differed, at the edges of the shadings that MuPDF
   anti-aliases. The test allows 2 percent. The two-point conical gradient
   of tiny-skia, with the decal mode or the pad mode, gives up to 24676
   such pixels in the same test.

## Speed

On 2026-11-18, `BenchmarkGradientsTwoPointRadialExtend` fills the path of
`BenchmarkGradientsTwoPointRadial` with the same circles in about 11 ms,
and the gradient of tiny-skia takes about 9 ms. The machine was busy with
other work, so the numbers vary by about 20 percent.
