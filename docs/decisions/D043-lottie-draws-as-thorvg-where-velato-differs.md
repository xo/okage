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
   | Masks | A clip for each mask, so that masks that add give their intersection, and no mode, inversion, opacity or expansion | A clip for one mask that adds. Any other masks build their coverage by compositing, as the section of 2026-11-25 says, with their mode, inversion, opacity and expansion |
   | Order of a trim path with the mode individually | The paths of the group in their order | The paths of the first item that holds a path backwards, then the paths of the other items backwards, as the section of 2026-11-25 says |

2. The masks combine in their order. A mask that adds draws its shape, one
   that subtracts takes it out, one that intersects keeps the common area,
   and one of the mode difference keeps the area that only one of them
   covers. A first mask that subtracts starts from the whole canvas. A mask of
   the mode none does nothing. The modes lighten and darken work as add and
   intersect. A positive expansion grows the shape by a round stroke of twice
   its amount, and a negative one shrinks it in the same way. The section
   of 2026-11-25 below changes how the masks combine.
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
4. Update the list of differences in `docs/DIFFERENCES.md`.

## Why

The corpus uses each feature of item 1. Before the changes, 710 of the 4,395
frames of the test differed by more than 1 percent from ThorVG. velato itself
marks several of these features as missing, such as the color stops and the
time remap. ThorVG is the renderer of the LottieFiles player, and velato
follows ThorVG where it says what it follows, so ThorVG is the reference for
the cases that velato leaves out.

## How the masks combine (2026-11-25)

D94 found that the masks of the mode difference of ThorVG do not give the
area that an odd number of masks covers, when there are three masks or
more. The maintainers asked on 2026-11-25 for a test of three and four
masks of the mode difference. `TestThorVGMaskDifference` in the test
module draws 25 sets of two to four overlapping masks with ThorVG through
`xo/lottie` and with okage. Before this change, 19 of the 25 sets differed
from ThorVG by more than 1 percent of the pixels, up to 87 percent. Two
masks of the mode difference differed, and so did a first mask that
subtracts with a mask that adds or intersects after it. Now 0% of the
pixels differ in each of the 125 frames.

ThorVG combines the masks in this way, and item 2 now follows it:

1. ThorVG skips a mask of the mode none.
2. A mask joins the shape of the mask before it when the two have the same
   mode and the same opacity, and the mode is subtract or difference. ThorVG
   fills the paths of a shape as one path with the nonzero rule, with the
   opacity of its masks. So two masks of the mode difference give the area
   of both, not the area that only one of them covers.
3. Each shape combines with the result of all the shapes after it, by the
   mode of the next shape. For the shapes s1, s2 and s3, the result is s1
   combined with the result of s2 and s3. So the masks add, subtract,
   intersect and take the difference from the last one, not in their order.
4. When the first mask subtracts, the coverage is the canvas without that
   result.

With an expansion, each mask of a shape draws its own grown or shrunk
shape, and the shape is the area of all of them. The test has one mask
that grows and one that shrinks in a shape of two masks.

ThorVG 1.1.1 does not invert a mask. The test drew an inverted mask with
the modes add, intersect, difference and subtract, first and later, and
ThorVG drew each one as a mask that is not inverted. velato ignores the
flag too. The maintainers chose on 2026-11-25 to keep the inversion,
because the Lottie format defines the flag. So no oracle covers it. The
shape of an inverted mask is the canvas without the shape of the mask, and
an inverted mask joins no other mask.

The corpus has no mask of the mode difference and no inverted mask. Its
masks are one mask, three masks that subtract, or two or three masks that
add, and these give the same area with the new rules. The gate of item 5
still passes. 3,709 of the 4,395 frames have 0% of their pixels that
differ, and the others 0.5% at most.

## The order of a trim path with the mode individually (2026-11-25)

A trim path with the mode individually trims the paths of its group as one
outline. velato trims the paths in their order. ThorVG 1.1.1 trims them in
another order, and the maintainers chose on 2026-11-25 that `lottie`
follows it. ThorVG and `lottie` now trim in this order:

1. The paths of the first item of the group that holds a path, backwards.
   That item is a path or a group with a path. A fill, a stroke or a group
   with no path before it does not count.
2. The paths of the other items, backwards.

The paths of a group inside the group come in their order before they go
backwards, so a group inside a group does not change the order. With the
paths 1 to 5 in one group, the order is 1, 5, 4, 3, 2. With the groups
(1, 2) and (3, 4) and the path 5, it is 2, 1, 5, 4, 3.

With `Renderer.MergePaths`, the contours of the result of a merge path are
paths of their own in this order (D94). So the result of a merge path of
the first operands gives the first item one contour, its first one.

`TestThorVGTrimIndividually` in the test module draws seven files of five
squares, in groups or not, with four trims each, with ThorVG and with
`lottie`. 0.5% of the pixels differ at most in each of the 140 frames. The
outline of a rectangle of ThorVG starts at another point than the one of
velato, which gives that difference. Before the change, frames differed in
up to 3.9% of the pixels. No file of the corpus has a trim path with the
mode individually.

Two cases do not follow ThorVG:

1. ThorVG counts a group with no item as the first item. `lottie` drops
   such a group when it reads the file, as velato does.
2. After a repeater in the group, ThorVG trimmed each copy of each path on
   its own in one test. `lottie` trims the paths in the order of velato
   there.

