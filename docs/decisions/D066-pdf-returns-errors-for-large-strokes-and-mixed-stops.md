# D66. `pdf` returns errors for large strokes and mixed stops

Status: Decided, amends D62.

D62 makes `pdf` return an error where krilla panics on an input of a
program. The merge of units 5 and 6 of phase 14 found two inputs for which
the error was not decided. On 2026-11-19 the maintainers decided both.

## A path to stroke with large coordinates

krilla strokes a path with tiny-skia to compute the bbox of the stroke,
inside a group, a mask, a pattern or a stream, and for a paint that is not a
color. tiny-skia panics in `find_quad_max_curvature` for a quadratic curve
whose products overflow to NaN, such as the curve from 2e37, -2e37 through
-1, 1e20 to 1, 1e38. The stroker also does not end for some paths with a
point that is NaN, because the bounds of the path leave the NaN out.

1. Before `pdf` computes the bbox of a stroke, it checks each point of the
   path. If a coordinate is NaN, or a coordinate plus the width of the
   stroke has a magnitude above 2^60, `pdf` records `ErrPDFInvalidPath`,
   which `Document.Finish` returns, and does not stroke the path. Below
   2^60, no product of the stroker overflows a float32.
2. `findQuadMaxCurvature` of `path` returns 0 for a NaN, where tiny-skia
   panics. The curve then counts as a line. This changes no output of the
   renderer, because each such input panicked before. The comment of the
   change starts with `okage:` and names this decision.
3. The fuzz test of `pdf` reads any float32 as a coordinate.

## The stops of a gradient in two color spaces

krilla panics when two stops of a gradient have colors with different
numbers of components, such as an RGB color and a CMYK color.

1. `pdf` records `ErrPDFMixedColorSpaces`, which `Document.Finish` returns,
   and writes no shading function. A shading of the opacities of the stops
   has one component for each stop, so it has no such error.

## Other panics of the gradients

D62 already covers the other panics of krilla for a gradient that a program
can cause: a gradient with no stop, and a transform that cannot be inverted
where krilla needs its inverse. `pdf` records `ErrPDFInvalidGradient` for
them.
