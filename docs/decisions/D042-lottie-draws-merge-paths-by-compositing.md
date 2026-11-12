# D42. `lottie` draws merge paths by compositing

Status: Decided, amends D41, amended by D43.

A merge path is a shape item of Lottie, with the type `mm`. It combines the
paths above it in its group into one shape, and the fills and strokes below
it draw that shape. Its mode tells how:

| Mode | Name | Result |
| --- | --- | --- |
| 1 | Merge | One compound path |
| 2 | Add | The union of the paths |
| 3 | Subtract | The first path without the paths after it |
| 4 | Intersect | The area that all paths cover |
| 5 | Exclude | The area that an odd number of paths cover |

velato skips each merge path. Mode 1 still draws correctly, because velato
joins all paths of one draw into one compound path. Modes 2 to 5 draw each
path as if the merge path was not there.

On 2026-11-12, the 879 Noto animated emoji had 108 merge paths of modes 2 to
4, in 28 files: 90 of mode 4, 12 of mode 3 and 6 of mode 2. Only fills and
gradient fills come after them.
No stroke, trim path, offset path, rounded corners or repeater comes after a
merge path of mode 2 to 5.

A geometric boolean operation on paths makes a new path. okage has no code
for it, and a port of one is large. A fill of the result does not need that
path, because okage can make the same area by compositing.

## The decision

1. `lottie` draws a fill or a gradient fill after a merge path of mode 2 to
   5 with these calls to its `Sink` (D41):
   1. It pushes a layer with the blend mode source-over and an opacity of 1,
      clipped to the bounds of the paths.
   2. It draws the first path with an opaque solid brush.
   3. It draws each next path with an opaque solid brush and the blend mode
      of the mode: source-over for add, destination-out for subtract,
      destination-in for intersect, and xor for exclude.
   4. It draws the bounds with the brush and the opacity of the fill, and
      the blend mode source-in.
   5. It pops the layer.
2. A path is one operand. A group above the merge path is one operand, with
   all its paths after its transform. Each operand draws with the fill rule
   of the fill. The operands go in the order of the group, top to bottom, as
   After Effects combines them. With one operand, the result is that
   operand.
3. Mode 1 needs nothing new. It stays as velato draws it.
4. A stroke, a trim path, an offset path, rounded corners or a repeater
   after a merge path of mode 2 to 5 needs the path of the result, which
   compositing does not give. In that case, `lottie` draws the paths as one
   compound path, as velato does, and warns on the logger (D20).
5. The okage sink of D41 draws each call with the parts of okage that exist:
   a layer is a pixmap that it composites with `DrawPixmap`, as `svg` draws
   a group, and the blend modes are the modes of tiny-skia. okage needs no
   new API.
6. velato is no oracle for merge paths. The test module compares the frames
   of the files with merge paths with ThorVG, through `github.com/xo/lottie`,
   within the tolerance of D23. Only the test module imports it (D6, D22).
   Tests in `lottie` cover each mode with small files that the tests write.
7. This is a deliberate difference from upstream. Each part of the code has
   a comment that starts with "Port:" and names D42.
8. D43 makes the compositing an option, `Renderer.MergePaths`, off by
   default, because ThorVG draws a merge path as velato does.

## When upstream changes

When a velato commit adds merge paths, follow these steps:

1. Remove the okage code of D42 from the importer and from the runtime of
   `lottie`. Find it by the comments that name D42.
2. Port the commit of upstream as `docs/UPSTREAM.md` says.
3. Run the tests of D42. If upstream does not cover a mode or a case that
   D42 covers, a maintainer decides whether okage keeps that part on top of
   the code of upstream. Record the result in D42 or in a new decision.
4. Update the list "Differences from upstream" in `README.md`.

If upstream adds geometric boolean operations, the fallback of item 4 also
goes away, because the stroke then has a path to draw.

## Why

The corpus uses merge paths of modes 2 to 4 only under fills, and a fill of
the result is the same area as a composite of the paths. Every blend mode
that this needs is already in okage, and the `Sink` of velato already takes a
layer with a blend mode. So the change is a few hundred lines in the port,
and okage gets no new API.

Compositing has limits. It gives no path of the result, so a stroke of the
result falls back to the compound path. Where two paths share an edge
exactly, the antialiasing of each edge can leave a faint seam, which a
geometric operation does not leave. No file of the corpus needs a path of
the result. A port of a geometric boolean operation waits until a file
needs one.
