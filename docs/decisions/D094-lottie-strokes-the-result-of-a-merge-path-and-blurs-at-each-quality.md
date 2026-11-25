# D94. lottie strokes the result of a merge path, and blurs at each quality

Status: Decided, amends D42 and D79.

Items 2 and 3 of "lottie" in [`../BACKLOG.md`](../BACKLOG.md) named two
parts of Lottie that `lottie` did not draw:

1. A stroke or a trim path after a merge path of mode 2 to 5. D42 draws a
   fill of the result of a merge path by compositing, which gives no path of
   the result. So a stroke or a trim path after it drew the operands as one
   compound path.
2. The quality of the Gaussian blur and the drop shadow. The `PixmapSink`
   drew them as ThorVG does at its default quality, 50, and had no option
   for another quality (D79).

The maintainers asked for both on 2026-11-25. velato and ThorVG are the
oracles of `lottie` (D41, D43). Where velato has no behavior, `lottie`
follows ThorVG.

## What the oracles do

The test module ran ThorVG through `github.com/xo/lottie` v0.1.0, the build
of ThorVG 1.1.1 that D82 names. The source of ThorVG is not on the machine,
so these results come from files that the tests make.

1. Merge paths. velato skips a merge path (D42). ThorVG draws the same image
   for the modes 1 to 5, for a fill and for a stroke: the operands as one
   compound path, as D43 says for a fill. So neither oracle has a geometric
   boolean operation, and kurbo has none either.
2. The masks of a layer. ThorVG draws a mask with the mode add, subtract or
   intersect as the merge modes 2, 3 and 4 combine their operands. ThorVG
   fills a path with the even-odd rule, which covers the area that an odd
   number of operands covers, as the merge mode 5 does. With three masks or
   more, the mode difference of ThorVG does not give that area.
3. The quality. ThorVG applies one box blur up to the quality 34, two up to
   67, and three above, to the blur and to the shadow. That is
   `int(3 * (quality - 1) / 100) + 1` boxes, the formula of D79 at the
   quality 50. The test drew a blur and a shadow at the qualities 1, 20, 34,
   35, 50, 67, 68 and 100 with ThorVG, and with okage at the qualities 1, 50
   and 100. okage matched ThorVG only with the same number of boxes, in each
   case. With another number of boxes, a blur differed in up to 2,868
   pixels of 120,000 and a shadow in up to 1,340.

## The decision

1. With `Renderer.MergePaths`, a stroke after a merge path of mode 2 to 5
   draws the path of the result of the merge path. A trim path after it
   trims that path, and a later stroke or fill draws the trimmed path. The
   result of a merge path is one operand of a later merge path, in the same
   group or in a group around it. A fill of the result still draws by
   compositing, as D42 says. Without `Renderer.MergePaths`, nothing changes,
   so `lottie` still draws as ThorVG does by default (D43).
2. The new package `internal/pathbool` computes the result. It is written
   for okage, because no oracle and no upstream has the operation (hard
   rule 3, D6). `pathbool.Combine` takes the operands in their order and the
   mode, and returns closed contours of lines:
   1. It flattens the curves of each operand with `kurbo.Flatten`, the port
      of `flatten` of kurbo, which the subset of kurbo now holds, with
      `QuadBez::estimate_subdiv` and the parts that it needs.
   2. It snaps the points to a grid of integers, 16 steps in the tolerance,
      so that each test of the geometry is exact.
   3. It splits each edge where another edge crosses or touches it, and it
      merges the edges that lie on each other. A point of a crossing snaps
      to the grid, so it repeats the splits until no edge crosses another.
   4. For each edge, it counts the winding number of each operand on both
      sides, with a vertical ray. Each operand fills with the nonzero rule,
      as the sink fills each path. An edge is part of the result when the
      mode gives the inside on one side and the outside on the other.
   5. It links these edges into closed contours, with the inside on their
      left. Where two contours touch at a point, each one keeps to the
      boundary of its own area.
3. The tolerance is 0.1 pixels, the tolerance of an ellipse in velato.
   `lottie` divides it by the largest scale of the transform from the
   operands to the pixels.
4. The `PixmapSink` takes the option `WithQuality`, from 0 to 100, which
   follows `tvg_lottie_animation_set_quality` of ThorVG (D20). It applies
   the number of box blurs of ThorVG at that quality, as part 3 of "What the
   oracles do" says. The default is 50. A value below 0 is 0, and a value
   above 100 is 100.
5. Each part of the code has a comment that names D94.

## Which oracle each part follows

| Part | Oracle | Test |
| --- | --- | --- |
| The area of the result of a merge path | ThorVG, through the masks and the even-odd rule of part 2 of "What the oracles do" | `TestThorVGMergeArea` fills the result of `pathbool` for four sets of two to four operands, rectangles, rounded rectangles and ellipses, and each of the modes 2 to 5. ThorVG draws the same area with masks or with the even-odd rule. 0% of the pixels differ in each of the 80 frames |
| The stroke and the trim path of the result | ThorVG, which strokes and trims the contours of the result as paths of a file | `TestThorVGMergeStroke`: 28 strokes and 56 trim paths, as the section of 2026-11-25 says. 0% of the pixels differ in each of the 420 frames |
| The start point, the direction and the order of the contours of the result | Neither. No oracle gives them, so a trim path of the result follows the rules of `pathbool`, which the section of 2026-11-25 lists | `TestMergePathsStroke` in `lottie`, and `TestThorVGMergeStroke` |
| The use of the result as an operand of a later merge path | Neither. D42 says how After Effects combines the operands | `TestMergePathsNested` in `lottie` |
| The quality of the blur and the shadow | ThorVG | `TestThorVGQuality`: a blur and a shadow at the qualities 1, 20, 34, 35, 67, 68 and 100. At most 0.008% of the pixels differ in each of the 70 frames |

At the qualities up to 34, the blur of ThorVG is one box, and the sink
blurs with the box blur of `svg` and the same standard deviation (D79). So
the largest difference of a channel is 35 there, and 4 at the other
qualities. The tolerance of D43 sees 10 pixels of 120,000.

## The results

Without `Renderer.MergePaths`, the bytes of each frame stay the same. With
it, the test against ThorVG gives the same share of pixels that differ as
before, in each frame of the 25 files of the corpus that change with
`Renderer.MergePaths`. No file of the corpus has a stroke or a trim path
after a merge path of mode 2 to 5, or a merge path whose operand is the
result of another one.

`pathbool.Combine` takes about 61 microseconds for two circles and a square
at the tolerance 0.1, in `BenchmarkCombine`.

## When upstream changes

If velato adds merge paths, follow the steps of D42. If it adds geometric
boolean operations, port them, and remove `internal/pathbool` and the code
of this decision that they replace. If ThorVG adds them, as
`github.com/xo/lottie` runs it, compare the tests of this decision with it,
and make the result follow ThorVG where the two differ, or record a
decision that keeps `pathbool`.

If a new version of ThorVG changes the number of box blurs at a quality,
`TestThorVGQuality` fails. Change `thorvgBlurLevels` to follow it.

## Why

A stroke of a merge path is the outline of the combined shape in After
Effects. Without a path of the result, the stroke drew each operand whole,
and a trim path trimmed each operand. A boolean operation on flattened
paths is small, and a fill of the result by compositing already exists, so
only the stroke and the trim path need it. The masks and the even-odd rule
of ThorVG give the same area as the merge modes, so ThorVG can still test
the result.

The quality is a setting of ThorVG that a program can set, for example to
make the blurs faster. Each quality draws as ThorVG draws it.

## A trim path of a result with several contours (2026-11-25)

The test of the trim path covered only a result with one contour. The
maintainers asked on 2026-11-25 for a test of a result with several
contours. `TestThorVGMergeStroke` now has seven sets of operands. The three
new sets are a bar across the square, a circle inside the square, and a
path in the shape of a U with a bar across its arms. The test strokes the
result of each set at each mode from 2 to 5. It trims each result once with
the trim mode simultaneously and once with the mode individually. 14 of the
28 results have two contours or more, and one has ten. 0% of the pixels
differ in each of the 420 frames. `pathbool` and `lottie` did not change.

The test pins these rules of `pathbool.Combine`:

1. The order of the contours. A contour comes before another when its
   first edge comes first. The edges come in the order of the operands, and
   in each operand in the order of its points after the flattening.
2. The start point of a contour. It is the start of its first edge.
3. The direction of a contour. It has the inside on its left where y goes
   up. On the canvas, where y goes down, an outer contour turns clockwise
   and a hole turns counterclockwise.

With the mode simultaneously, okage trims each contour of the result on its
own, as velato trims each contour of a path. ThorVG trims each path of the
file on its own. So that trim pins the start points and the directions.
With the mode individually, okage trims the contours as one outline. So
that trim also pins the order.

ThorVG 1.1.1 trims the paths of a group with the mode individually in
another order than velato: the first path, then the last, and then the
others backwards. The maintainers chose on 2026-11-25 that `lottie`
follows that order, as the section of 2026-11-25 of D43 says. A trim path
with the mode individually trims each contour of the result of a merge
path as a path of its own in that order. So the file of the test gives
ThorVG the contours in the order of `pathbool`, and the two draw the same.
