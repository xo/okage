# D107. The bounds of a path check each point, and a hairline stays in the pixmap

Status: Decided, amends D16 and D74.

## The problem

ivy found a crash on 2026-11-28, in its fuzz test `FuzzEncode` of
`pdf/pdfenc`. This path, filled with anti-aliasing on a pixmap of 60 by 45
pixels, panics in the alpha runs of the scan converter:

```
MoveTo(109, 19)
QuadTo(-15, NaN, -6, 125)
CubicTo(18, 3, -9, 36, -5, 228)
Close
```

okage v0.23.0 and v0.24.0 have the fault. `PathBuilder.Finish` computes the
bounds of the points with `RectFromPoints`, which must return false for a
point that is NaN or infinite. Skia, in `SkRect::setBoundsCheck`, starts its
check with the first one or two points times 0. That product is NaN when one
of them is not finite. tiny-skia starts the check at 0, so it checks only
the points after the first one or two. In the path above, the second point
holds the NaN. The minimum and the maximum leave the NaN out, the bounds are
finite, and `Finish` takes the path.

An infinite point does not get through, because it makes the bounds
infinite, and `RectFromLTRB` refuses them.

When the fuzz test `FuzzFillPathHuge` of okage took NaN and infinities too,
it found a second crash, with finite points. A hairline without
anti-aliasing can give a pixel one column past the right side of the
pixmap. tiny-skia draws that pixel on the first column of the next row, and
okage keeps that, because it gives the bytes of tiny-skia (D16). On the
last row, the next row is past the end of the pixmap, and tiny-skia and
okage panic. The Rust oracle of `test/oracle` stops on the same input.
tiny-skia limits the row of the mostly horizontal lines for the same
reason, but not the column of the mostly vertical ones.

## The rule

1. `RectFromPoints` starts its check with the first points times 0, as
   Skia does. It returns false for a NaN or an infinity in any point. So
   `PathBuilder.Finish`, `Path.Transform`, `Rect.Transform` and the edge
   clipper of the scan converter refuse such points. The result is the
   same as tiny-skia for each set of finite points.
2. `hairLineRgn` skips a pixel past the right side of the clip on the last
   row of the clip. On every other row, it draws the pixel on the first
   column of the next row, as tiny-skia does. tiny-skia panics on that
   pixel, so the bytes of each image that tiny-skia draws do not change.
3. The check of D74 in `Path.Stroke` stays. No public function makes a
   path with a NaN any more, but a path of the package can still hold one.

Each change has a comment that starts with `okage:` and names this
decision.

## Other checks of points

`Point.IsFinite` of tiny-skia tests the product of x and y, where
`SkPoint::isFinite` of Skia tests each coordinate times 0. So tiny-skia
treats a point such as (1e20, 1e20) as not finite, and the edge builder
leaves out its line. That is stricter than Skia, and lets no NaN through.
okage keeps it, to keep the bytes of tiny-skia. No other function of the
port of tiny-skia computes bounds with a check like `RectFromPoints`.

## Tests

`path/rect_finite_test.go` checks `RectFromPoints` with a NaN and an
infinity in each point of 1 to 6 points. It also checks `Finish` with the
path above, and `Path.Transform` with a transform that makes the first point
NaN.
`scan_path_finite_test.go` fills both paths, and
`TestHairlinePastTheLastRow` of `hairline_overflow_test.go` strokes the
hairline that the fuzz test found. `FuzzFillPathHuge` now also takes NaN
and infinities, and ran 31 million inputs without a panic.
`TestStrokeNotFinite` of `path` makes its paths by hand, because `Finish`
refuses them now.

## When upstream changes

If tiny-skia checks the first points in `Rect::from_points`, or limits the
column of a hairline, port the commit as `docs/UPSTREAM.md` says. If the
result is the same, remove the change of this decision.
