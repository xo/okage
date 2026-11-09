# The subset of kurbo

okage ports a subset of kurbo 0.13.1, from the commit `838b69ef` (D7). The
subset holds each type and function that svgtypes and usvg call, and each
item that those items need. The traits `ParamCurve`, `ParamCurveDeriv` and
`ParamCurveArclen` are ported in full for each type of the subset, so that
each type has the methods that the trait gives it.

`svgtypes` means svgtypes 0.16.1. `usvg` means usvg in resvg 0.49.0, and the
paths start at `crates/usvg/src`. A use that is marked "for the subset" comes
from another item of the subset, not from svgtypes or usvg.

The port differs from upstream in one place: `Arc.AppendIter` makes at most
`MaxArcSubdivisions` curves for one arc, where kurbo has no limit (D30).

## point.rs, in point.go

| Item | Go | Used by |
| --- | --- | --- |
| `Point` | `Point` | svgtypes `path.rs`, usvg `parser/shapes.rs`, `parser/text.rs`, `text/layout.rs` |
| `Point::new` | `NewPoint` | svgtypes `path.rs`, usvg `parser/shapes.rs`, `parser/text.rs`, `text/layout.rs` |
| `Point::to_vec2` | `Point.ToVec2` | for the subset |
| `Point::lerp` | `Point.Lerp` | for the subset: `Line::eval` |
| `Point::midpoint` | `Point.Midpoint` | for the subset: `QuadBez::subdivide`, `CubicBez::subdivide` |
| `Point::distance` | `Point.Distance` | the tests of `arc.rs` |
| `impl Add<Vec2> for Point` | `Point.Add` | for the subset |
| `impl Sub<Vec2> for Point` | `Point.SubVec` | for the subset |
| `impl Sub<Point> for Point` | `Point.Sub` | for the subset |

## vec2.rs, in vec2.go

| Item | Go | Used by |
| --- | --- | --- |
| `Vec2` | `Vec2` | svgtypes `path.rs`, usvg `parser/shapes.rs`, `text/layout.rs` |
| `Vec2::new` | `NewVec2` | svgtypes `path.rs`, usvg `parser/shapes.rs`, `text/layout.rs` |
| `Vec2::to_point` | `Vec2.ToPoint` | for the subset |
| `Vec2::dot` | `Vec2.Dot` | for the subset: `QuadBez::arclen` |
| `Vec2::hypot`, `Vec2::hypot2` | `Vec2.Hypot`, `Vec2.Hypot2` | for the subset: the arc lengths |
| `Vec2::atan2` | `Vec2.Atan2` | usvg `text/layout.rs`, and `Arc::from_svg_arc` |
| `Vec2::lerp` | `Vec2.Lerp` | for the subset |
| `impl Add for Vec2`, `impl Sub for Vec2` | `Vec2.Add`, `Vec2.Sub` | for the subset |
| `impl Mul<f64> for Vec2`, `impl Mul<Vec2> for f64` | `Vec2.Mul` | for the subset |

## line.rs, in line.go

| Item | Go | Used by |
| --- | --- | --- |
| `Line` | `Line` | usvg `parser/text.rs`, `text/layout.rs` |
| `Line::new` | `NewLine` | usvg `parser/text.rs`, `text/layout.rs` |
| `impl ParamCurve for Line` | `Line.Eval`, `Subsegment`, `Subdivide`, `Start`, `End` | usvg `parser/text.rs`, `text/layout.rs` (`eval`) |
| `impl ParamCurveDeriv for Line` | `Line.Deriv` | the trait, and the test `quadbez_raise` |
| `impl ParamCurveArclen for Line` | `Line.Arclen`, `Line.InvArclen` | the trait |
| `ConstPoint`, with its impls of the three traits | `ConstPoint` | for the subset: `Line::deriv` |

## quadbez.rs, in quadbez.go

| Item | Go | Used by |
| --- | --- | --- |
| `QuadBez` | `QuadBez` | usvg `parser/text.rs`, `text/layout.rs` |
| `QuadBez::new` | `NewQuadBez` | usvg `parser/text.rs` |
| `QuadBez::raise` | `QuadBez.Raise` | usvg `parser/text.rs`, `text/layout.rs` |
| `impl ParamCurve for QuadBez` | `QuadBez.Eval`, `Subsegment`, `Subdivide`, `Start`, `End` | usvg `text/layout.rs` (`deriv().eval`) |
| `impl ParamCurveDeriv for QuadBez` | `QuadBez.Deriv` | the trait |
| `impl ParamCurveArclen for QuadBez` | `QuadBez.Arclen`, `QuadBez.InvArclen` | the trait |

## cubicbez.rs, in cubicbez.go

| Item | Go | Used by |
| --- | --- | --- |
| `CubicBez` | `CubicBez` | usvg `parser/text.rs`, `text/layout.rs` |
| `CubicBez::new` | `NewCubicBez` | usvg `parser/text.rs` |
| `impl ParamCurve for CubicBez` | `CubicBez.Eval`, `Subsegment`, `Subdivide`, `Start`, `End` | usvg `text/layout.rs` (`eval`), and `inv_arclen` |
| `impl ParamCurveDeriv for CubicBez` | `CubicBez.Deriv` | usvg `text/layout.rs` |
| `arclen_quadrature_core`, `arclen_rec` | `arclenQuadratureCore`, `arclenRec` | for the subset: `CubicBez::arclen` |
| `impl ParamCurveArclen for CubicBez` | `CubicBez.Arclen`, `CubicBez.InvArclen` | usvg `parser/text.rs` (`arclen`), `text/layout.rs` (`arclen`, `inv_arclen`) |

## svg.rs, in svg.go

| Item | Go | Used by |
| --- | --- | --- |
| `SvgArc` | `SvgArc` | svgtypes `path.rs`, usvg `parser/shapes.rs` |
| `SvgArc::is_straight_line` | `SvgArc.IsStraightLine` | for the subset: `Arc::from_svg_arc` |
| `Arc::from_svg_arc` | `FromSvgArc` | svgtypes `path.rs`, usvg `parser/shapes.rs` |

## arc.rs, in arc.go

| Item | Go | Used by |
| --- | --- | --- |
| `Arc` | `Arc` | svgtypes `path.rs`, usvg `parser/shapes.rs` |
| `Arc::new`, `Arc::reversed` | `NewArc`, `Arc.Reversed` | the tests of `arc.rs` |
| `Arc::angle_at` | `Arc.angleAt` | for the subset |
| `Arc::append_iter`, `ArcAppendIter` | `Arc.AppendIter`, `ArcAppendIter`, with the limit `MaxArcSubdivisions` of D30 | for the subset: `Arc::to_cubic_beziers` |
| `Arc::to_cubic_beziers` | `Arc.ToCubicBeziers` | svgtypes `path.rs`, usvg `parser/shapes.rs` |
| `sample_ellipse`, `rotate_pt` | `sampleEllipse`, `rotatePt` | for the subset |
| `impl ParamCurve for Arc` | `Arc.Eval`, `Subsegment`, `Subdivide`, `Start`, `End` | the tests of `arc.rs` |

`ArcAppendIter::next` returns the three points of the only path element that
it makes, `PathEl::CurveTo`, because okage does not port `PathEl`.

## param_curve.rs, in param_curve.go

| Item | Go | Used by |
| --- | --- | --- |
| `DEFAULT_ACCURACY` | `DefaultAccuracy` | the trait file |
| `ParamCurve`, with its default methods | `ParamCurve`, `subdivideParamCurve`, `startParamCurve`, `endParamCurve` | usvg `parser/text.rs`, `text/layout.rs` |
| `ParamCurveDeriv`, with `gauss_arclen` | `ParamCurveDeriv`, `GaussArclen` | usvg `text/layout.rs` |
| `ParamCurveArclen`, with `inv_arclen` | `ParamCurveArclen`, `invArclenParamCurve` | usvg `parser/text.rs`, `text/layout.rs` |

## common.rs, in common.go

| Item | Go | Used by |
| --- | --- | --- |
| `FloatFuncs`: `abs`, `atan2`, `ceil`, `cos`, `copysign`, `floor`, `ln`, `log2`, `powi`, `powf`, `round`, `sin`, `sin_cos`, `sqrt`, `tan`, `trunc`, `signum` | `Abs`, `Atan2`, `Ceil`, `Cos`, `Copysign`, `Floor`, `Ln`, `Log2`, `Powi`, `Powf`, `Round`, `Sin`, `SinCos`, `Sqrt`, `Tan`, `Trunc`, `Signum` | svgtypes `color.rs` and `transform.rs`, and the subset |
| `solve_itp`, `solve_itp_fallible` | `SolveItp`, `solveItpFallible` | for the subset: `inv_arclen` |
| `GAUSS_LEGENDRE_COEFFS_8`, `_8_HALF`, `_16_HALF`, `_24_HALF` | `GaussLegendreCoeffs8`, `GaussLegendreCoeffs8Half`, `GaussLegendreCoeffs16Half`, `GaussLegendreCoeffs24Half` | for the subset: `CubicBez::arclen` |

usvg turns on the feature `std` of kurbo, so the methods of `FloatFuncs` are
the methods of `f64` in the Rust standard library. `common.go` also holds the
helpers `maxF64`, `minF64`, `remF64` and `f64ToUsize`, which follow `f64::max`,
`f64::min`, the operator `%` and the conversion `as usize` of Rust.

## What is not in the subset

The rest of `src/` is Not applicable (D7): among others `BezPath`, `PathEl`,
`Shape`, `Affine`, `Rect`, `Ellipse`, the traits `ParamCurveArea`,
`ParamCurveNearest`, `ParamCurveCurvature` and `ParamCurveExtrema`, the
polynomial solvers and the conversion of a cubic to quadratic Beziers. The
upstream tests of these items are not ported.
