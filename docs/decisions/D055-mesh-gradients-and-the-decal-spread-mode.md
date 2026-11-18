# D55. Mesh gradients and the decal spread mode

Status: Decided.

ivy draws PDF, PostScript and the layout of HTML and CSS with okage. It needs
two things that tiny-skia does not have:

1. A fill of a mesh of triangles with a color at each corner, which the fill
   interpolates. This is Gouraud shading. The shadings of the types 4 to 7 of
   PDF and PostScript need it. ivy splits the patches of the types 6 and 7
   into triangles itself. Before this decision, ivy sampled each mesh into a
   texture and drew the texture as a `Pattern`.
2. A spread mode of a gradient that paints nothing before the start and after
   the end of the gradient. A PDF shading with `Extend` false needs it. Skia
   calls this mode the tile mode `kDecal`.

On 2026-11-18 a maintainer asked for both as new API of okage. They are not a
port of tiny-skia. The bytes of every input that worked before do not change.

## The decision

### The decal spread mode

1. `SpreadMode` has a fourth value, `SpreadModeDecal`, after
   `SpreadModeRepeat`. The three values of tiny-skia keep their numbers. Its
   `String` is `Decal`.
2. A linear, radial or sweep gradient in this mode paints transparent black
   where its t is outside 0..=1, and a NaN t is outside. Inside, it paints the
   color of `SpreadModePad`. The stops do not set the limits: with a first
   stop at 0.2, t from 0 to 0.2 paints the color of the first stop, as in
   Skia.
3. The pipeline follows the stages `decal_x` and `check_decal_mask` of Skia
   (BSD-3-Clause). `stageDecalX1` keeps a mask of the lanes whose t is in
   0..=1. Then, as in Skia, the mode clamps t as `SpreadModePad` does. After
   the gradient stage, and before the premultiplication, `stageCheckDecalMask`
   makes the lanes outside the mask transparent. The mask is a field of its
   own in the context, so it works with the mask of a two-point conical
   gradient.
4. A gradient in this mode is never opaque, as in Skia. The blitter thus does
   not change `SourceOver` into `Source` for it.
5. A linear gradient whose start and end are very close returns a
   transparent `SolidColor` in this mode. Skia returns an empty shader, which
   draws nothing. The transparent color is the limit of a band of colors
   whose width goes to 0, and it acts as the mode does with the blend mode
   `Source`.
6. These cases keep the behavior of tiny-skia for each mode:
   - A gradient with one stop returns the `SolidColor` of that stop.
   - A degenerate radial or sweep gradient returns false, as it does for
     `SpreadModeRepeat` and `SpreadModeReflect`.
   - A sweep gradient over the full circle uses `SpreadModePad`. Its t is
     always in 0..=1, so the result is the same.
7. A `Pattern` treats `SpreadModeDecal` as `SpreadModePad`. ivy did not ask
   for decal images, and the decal of Skia for a filtered image needs more
   work.

### MeshGradient

8. A new shader, `MeshGradient`, fills a mesh of triangles. The new API is:

   ```go
   type MeshVertex struct {
       Point Point
       Color Color
   }
   type MeshTriangle [3]MeshVertex
   type MeshGradient struct{ /* unexported */ }
   func NewMeshGradient(triangles []MeshTriangle, transform Transform) (Shader, bool)
   ```

   `MeshGradient` implements `Shader`, so it works with `FillPath`,
   `FillRect`, a `Mask`, each blend mode, `Transform` and `ApplyOpacity`.
   `transform` maps the space of the triangles to the space of the user, as
   the transform of a gradient does.
9. `NewMeshGradient` skips a triangle with no area or with a point that is
   not finite. It returns false when no triangle is left, or when the
   transform is not invertible.
10. The shader takes the center of each pixel, in the space of the mesh:
    - If triangles cover the center, the last of them in the list gives the
      color. A later triangle thus paints over an earlier one, as in PDF.
    - If no triangle covers the center, but the square of the pixel overlaps
      a triangle with an area that is not 0, the last such triangle gives
      the color of its point near the center. The test of the overlap uses
      the axes x and y and the normals of the three edges.
    - Elsewhere, the color is transparent black.

    With the second rule, an antialiased fill of the outline of the mesh has
    the colors of the mesh at its edge, and no dark border. A fill of a mesh
    that is aligned to the pixels does not grow by a pixel.
11. The fill has no seams. Each edge has a function that is
    `(x1 - x0) * (y - y0) - (y1 - y0) * (x - x0)`, where `(x0, y0)` is the
    first point of the edge in the order of x and then y. Two triangles that
    share an edge thus compute the same value at a point, with the opposite
    sign, and a point on the shared edge is in both. Each pixel takes its
    color from one triangle. A caller fills a path that covers the mesh,
    such as its outline, and the path gives the antialiasing.
12. The shader interpolates the colors without premultiplied alpha, in the
    `ColorSpace` of the `Paint`, and then premultiplies them, as the gradients
    of tiny-skia do. The color is `c0 + t1 * (c1 - c0) + t2 * (c2 - c0)`, with
    the weights of the corners from the functions of the edges. A mesh of one
    color thus gives that color exactly. Each channel is clamped to 0..=1.
    `ApplyOpacity` multiplies the alpha after the interpolation.
13. A `MeshGradient` is never opaque, because it is transparent outside its
    triangles.
14. `NewMeshGradient` puts the triangles in a grid of cells, with about one
    cell for each triangle and at most 1024 cells on a side. The grid gets
    coarser until it holds at most 16 entries for each triangle. A pixel
    tests only the triangles of its cell, from the last to the first. The
    triangles and the grid do not change after `NewMeshGradient`, so the
    copies of a shader share them, and many goroutines can draw with one
    shader.

### The pipeline

15. The new stages `stageDecalX1`, `stageCheckDecalMask` and
    `stageMeshGradient` come after the stages of tiny-skia, so those keep
    their values. Only the highp pipeline has them. A decal gradient and a
    mesh thus always use the highp pipeline. The pipelines of every other
    shader do not change.
16. The new stages compute each lane in one loop on the lanes (D33). Each
    product that an addition follows has an explicit `float32` conversion
    (D16). The scalar build and the SIMD build thus give the same bytes. The
    stages do not allocate. `pushStages` of a `MeshGradient` allocates the
    colors of the mesh only for a `ColorSpace` other than `ColorSpaceLinear`.

## The tests

tiny-skia has neither feature, so the Rust oracle cannot test them. The tests
in `shaders_decal_test.go` and `shaders_mesh_gradient_test.go` compare the
pixels with values that the test knows:

1. A decal gradient gives the bytes of the same gradient in the pad mode
   where t is in 0..=1, and transparent pixels elsewhere. The tests cover
   linear, radial, concentric, two-point conical and sweep gradients, with
   two stops and with three stops that are not evenly spaced.
2. A mesh of one color, turned and scaled, gives the bytes of a fill of its
   outline with that color, antialiased or not, with each blend mode, a mask,
   a color space and an opacity.
3. A turned mesh of many colors gives each pixel the alpha of the coverage
   of its outline. No pixel inside the mesh lets the background through.
4. A triangle with a red that grows with x and a green that grows with y
   gives each pixel inside the color at its center, and each pixel outside
   is transparent.
5. A later triangle paints over an earlier one, and a mesh of 3000 triangles
   that overlap fills in a short time.
