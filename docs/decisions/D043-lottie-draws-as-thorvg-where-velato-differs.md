# D43. `lottie` draws as ThorVG where velato differs

Status: Decided, amends D41 and D42.

The maintainers asked for a port of velato that draws the corpus of `xo/lottie`
as ThorVG draws it, within a tolerance (D41). The corpus is the 879 Noto
animated emoji. The port of velato draws most of the corpus as ThorVG does,
but velato differs from ThorVG in the places that this decision lists. A
change of the file of `xo/lottie` that runs ThorVG is not part of okage, so
ThorVG is the reference.

## The decision

1. These are deliberate differences from velato. Each one follows ThorVG, as
   `xo/lottie` runs it. Each part of the code has a comment that starts with
   "Port:" and names D43.

   | Feature | velato | okage |
   | --- | --- | --- |
   | Opacity of a gradient fill or a gradient stroke | Always 100 | The key `o` |
   | Opacity stops of a gradient | The alpha only at the offsets of the color stops, with a hack | A stop at each offset of a color stop and of an opacity stop, with the color and the alpha of each list there |
   | Radius and highlight of a radial gradient | The distance from the start to the end, and no highlight | The radius that ThorVG computes, the longer side plus 0.375 of the shorter side, and the focal point at the highlight length and angle |
   | Solid layer | Draws nothing | Fills its rectangle with its color |
   | Time remap of a precomposition | Samples `tm` at the frame divided by the stretch, minus the start | Samples `tm` at the frame of the parent, takes 0 as the least time, and computes the frame in float32, as ThorVG does. The stretch does not apply, because `tm` gives the time itself |
   | Easing of a value with a handle for each dimension | One keyframe for each pair of handles, at the same time | One keyframe, and every dimension takes the first pair |
   | Spatial position | The point at the exact arc length | The point at the approximate arc length of ThorVG, which measures each distance as the longer side plus 0.375 of the shorter side |
| Easing of an animated shape | Ignored: the shapes blend with the linear time | The eased time |
| Closing of an animated shape | Closed when any keyframe is closed, and any two keyframes blend | Closed as its keyframe is closed. Between a closed and an open keyframe, the first keyframe stays, because ThorVG cannot blend points that differ in number |
   | Opacity of a layer | Multiplied into each draw, so overlapping shapes add up, except for a layer with effects | Applied to the layer as a whole |
   | Blend mode of a layer | Read and not used | The layer composites with its blend mode. A matte source with the mode multiply gives an empty matte, because ThorVG multiplies its colors into the empty matte |
   | Masks | A clip for each mask, so that masks that add give their intersection, and no mode, inversion, opacity or expansion | A clip for one mask that adds. Any other masks build their coverage by compositing, in their order and with their mode, inversion, opacity and expansion |

2. The masks combine in their order. A mask that adds draws its shape, one
   that subtracts takes it out, one that intersects keeps the common area,
   and one of the mode difference keeps the area that only one of them
   covers. A first mask that subtracts starts from the whole canvas. A mask of
   the mode none does nothing. The modes lighten and darken work as add and
   intersect. A positive expansion grows the shape by a round stroke of twice
   its amount, and a negative one shrinks it in the same way.
3. The `PixmapSink` draws the effects Fill and Tint. It does not draw the
   Gaussian blur and the drop shadow, as the sinks of velato for Vello do not.
   This is not a difference from velato, because velato leaves the effects to
   the sink.
4. The test switch `internal/lottiecompat.Velato` turns the differences of D42
   and D43 off. Only the test against velato sets it, so that the test
   compares the port itself on every file of the corpus (D41). A program never
   sets it.
5. The test module compares `lottie` with ThorVG through `xo/lottie`. It
   renders five frames of each file of the corpus, with the colors
   premultiplied. A pixel differs when one of its channels lies more than 16
   below the least or above the most of that channel in the pixels of the
   other image at it and next to it, in either direction. This leaves out the
   antialiasing of an edge, which two rasterizers compute in other ways, and
   it keeps a wrong color or a missing shape. A frame passes when at most 1
   percent of its pixels differ. The gate is that every frame passes.
6. ThorVG draws a merge path as a compound path, as velato does. So
   `Renderer.MergePaths` turns the compositing of D42 on, and it is off by
   default. In the corpus, one file changes with it, and with it that file
   differs from ThorVG.

## When upstream changes

When a velato commit changes one of the features of item 1, follow these
steps:

1. Remove the okage code of that row of item 1. Find it by the comments that
   name D43.
2. Port the commit of upstream as `docs/UPSTREAM.md` says.
3. Run the test against ThorVG. If upstream draws a file of the corpus in
   another way than ThorVG, a maintainer decides whether okage keeps that
   part on top of the code of upstream. Record the result here or in a new
   decision.
4. Update the list "Differences from upstream" in `README.md`.

## Why

The corpus uses each feature of item 1. Before the changes, 710 of the 4,395
frames of the test differed by more than 1 percent from ThorVG. velato itself
marks several of these features as missing, such as the color stops and the
time remap. ThorVG is the renderer of the LottieFiles player, and velato
follows ThorVG where it says what it follows, so ThorVG is the reference for
the cases that velato leaves out.
