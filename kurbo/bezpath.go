// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

// PathElKind is the variant of PathEl in kurbo kurbo/src/bezpath.rs (D27).
type PathElKind uint8

// The variants of PathEl.
const (
	PathElMoveTo PathElKind = iota
	PathElLineTo
	PathElQuadTo
	PathElCurveTo
	PathElClosePath
)

// PathEl is PathEl in kurbo kurbo/src/bezpath.rs. A MoveTo and a LineTo use
// P[0], a QuadTo uses P[0] and P[1], and a CurveTo uses all three points.
type PathEl struct {
	Kind PathElKind
	P    [3]Point
}

// MoveTo is PathEl::MoveTo in kurbo kurbo/src/bezpath.rs.
func MoveTo(p Point) PathEl { return PathEl{Kind: PathElMoveTo, P: [3]Point{p}} }

// LineTo is PathEl::LineTo in kurbo kurbo/src/bezpath.rs.
func LineTo(p Point) PathEl { return PathEl{Kind: PathElLineTo, P: [3]Point{p}} }

// QuadTo is PathEl::QuadTo in kurbo kurbo/src/bezpath.rs.
func QuadTo(p1, p2 Point) PathEl { return PathEl{Kind: PathElQuadTo, P: [3]Point{p1, p2}} }

// CurveTo is PathEl::CurveTo in kurbo kurbo/src/bezpath.rs.
func CurveTo(p1, p2, p3 Point) PathEl {
	return PathEl{Kind: PathElCurveTo, P: [3]Point{p1, p2, p3}}
}

// ClosePath is PathEl::ClosePath in kurbo kurbo/src/bezpath.rs.
func ClosePath() PathEl { return PathEl{Kind: PathElClosePath} }

// MulPathEl is impl Mul<PathEl> for Affine in kurbo kurbo/src/bezpath.rs.
func (a Affine) MulPathEl(el PathEl) PathEl {
	switch el.Kind {
	case PathElMoveTo, PathElLineTo:
		el.P[0] = a.MulPoint(el.P[0])
	case PathElQuadTo:
		el.P[0], el.P[1] = a.MulPoint(el.P[0]), a.MulPoint(el.P[1])
	case PathElCurveTo:
		el.P[0], el.P[1], el.P[2] = a.MulPoint(el.P[0]), a.MulPoint(el.P[1]), a.MulPoint(el.P[2])
	}
	return el
}

// toQuadTol is TO_QUAD_TOL in kurbo kurbo/src/bezpath.rs.
//
// The share of the tolerance of Flatten that goes to the conversion of a
// cubic into quadratics.
const toQuadTol float64 = 0.1

// Flatten is flatten in kurbo kurbo/src/bezpath.rs. It appends to dst the
// elements of path with each curve flattened to lines, and returns dst.
//
// The tolerance value controls the maximum distance between the curved input
// segments and their polyline approximations. (In technical terms, this is
// the Hausdorff distance). The algorithm attempts to bound this distance by
// tolerance but this is not absolutely guaranteed. The number of segments
// tends to scale as the inverse square root of tolerance.
//
// Port: kurbo calls a callback with each element. Flatten appends them to a
// slice, so that it takes no closure (D25, D94).
func Flatten(path []PathEl, tolerance float64, dst []PathEl) []PathEl {
	sqrtTol := Sqrt(tolerance)
	var lastPt Point
	hasLast := false
	var quadBuf []quadFlatten
	for _, el := range path {
		switch el.Kind {
		case PathElMoveTo, PathElLineTo:
			lastPt, hasLast = el.P[0], true
			dst = append(dst, el)
		case PathElQuadTo:
			if hasLast {
				q := NewQuadBez(lastPt, el.P[0], el.P[1])
				params := q.estimateSubdiv(sqrtTol)
				n := max(f64ToUsize(Ceil(0.5*params.val/sqrtTol)), 1)
				step := 1.0 / float64(n)
				for i := uint(1); i < n; i++ {
					u := float64(i) * step
					t := q.determineSubdivT(&params, u)
					dst = append(dst, LineTo(q.Eval(t)))
				}
				dst = append(dst, LineTo(el.P[1]))
			}
			lastPt, hasLast = el.P[1], true
		case PathElCurveTo:
			if hasLast {
				c := NewCubicBez(lastPt, el.P[0], el.P[1], el.P[2])

				// Subdivide into quadratics, and estimate the number of
				// subdivisions required for each, summing to arrive at an
				// estimate for the number of subdivisions for the cubic.
				// Also retain these parameters for later.
				iter := c.ToQuads(tolerance * toQuadTol)
				quadBuf = quadBuf[:0]
				sqrtRemainTol := sqrtTol * Sqrt(1.0-toQuadTol)
				sum := 0.0
				for {
					_, _, q, ok := iter.Next()
					if !ok {
						break
					}
					params := q.estimateSubdiv(sqrtRemainTol)
					sum += params.val
					quadBuf = append(quadBuf, quadFlatten{q: q, params: params})
				}
				n := max(f64ToUsize(Ceil(0.5*sum/sqrtRemainTol)), 1)

				// Iterate through the quadratics, outputting the points of
				// subdivisions that fall within that quadratic.
				step := sum / float64(n)
				i := uint(1)
				valSum := 0.0
				for k := range quadBuf {
					qp := &quadBuf[k]
					target := float64(i) * step
					recipVal := 1 / qp.params.val
					for target < valSum+qp.params.val {
						u := (target - valSum) * recipVal
						t := qp.q.determineSubdivT(&qp.params, u)
						dst = append(dst, LineTo(qp.q.Eval(t)))
						i++
						if i == n+1 {
							break
						}
						target = float64(i) * step
					}
					valSum += qp.params.val
				}
				dst = append(dst, LineTo(el.P[2]))
			}
			lastPt, hasLast = el.P[2], true
		case PathElClosePath:
			hasLast = false
			dst = append(dst, el)
		}
	}
	return dst
}

// quadFlatten is a quadratic of a cubic and its parameters, the element of
// quad_buf in flatten in kurbo kurbo/src/bezpath.rs.
type quadFlatten struct {
	q      QuadBez
	params flattenParams
}

// PathSegKind is the variant of PathSeg in kurbo kurbo/src/bezpath.rs (D27).
type PathSegKind uint8

// The variants of PathSeg.
const (
	PathSegLine PathSegKind = iota
	PathSegQuad
	PathSegCubic
)

// PathSeg is PathSeg in kurbo kurbo/src/bezpath.rs. The field of its kind
// holds the segment.
type PathSeg struct {
	Kind  PathSegKind
	Line  Line
	Quad  QuadBez
	Cubic CubicBez
}

// Eval is impl ParamCurve for PathSeg in kurbo kurbo/src/bezpath.rs.
func (s PathSeg) Eval(t float64) Point {
	switch s.Kind {
	case PathSegLine:
		return s.Line.Eval(t)
	case PathSegQuad:
		return s.Quad.Eval(t)
	}
	return s.Cubic.Eval(t)
}

// Subsegment is impl ParamCurve for PathSeg in kurbo kurbo/src/bezpath.rs.
func (s PathSeg) Subsegment(start, end float64) PathSeg {
	switch s.Kind {
	case PathSegLine:
		s.Line = s.Line.Subsegment(start, end)
	case PathSegQuad:
		s.Quad = s.Quad.Subsegment(start, end)
	default:
		s.Cubic = s.Cubic.Subsegment(start, end)
	}
	return s
}

// Start is impl ParamCurve for PathSeg in kurbo kurbo/src/bezpath.rs.
func (s PathSeg) Start() Point {
	switch s.Kind {
	case PathSegLine:
		return s.Line.P0
	case PathSegQuad:
		return s.Quad.P0
	}
	return s.Cubic.P0
}

// End is impl ParamCurve for PathSeg in kurbo kurbo/src/bezpath.rs.
func (s PathSeg) End() Point {
	switch s.Kind {
	case PathSegLine:
		return s.Line.P1
	case PathSegQuad:
		return s.Quad.P2
	}
	return s.Cubic.P3
}

// Arclen is impl ParamCurveArclen for PathSeg in kurbo kurbo/src/bezpath.rs.
func (s PathSeg) Arclen(accuracy float64) float64 {
	switch s.Kind {
	case PathSegLine:
		return s.Line.Arclen(accuracy)
	case PathSegQuad:
		return s.Quad.Arclen(accuracy)
	}
	return s.Cubic.Arclen(accuracy)
}

// InvArclen is impl ParamCurveArclen for PathSeg in kurbo
// kurbo/src/bezpath.rs.
func (s PathSeg) InvArclen(arclen, accuracy float64) float64 {
	switch s.Kind {
	case PathSegLine:
		return s.Line.InvArclen(arclen, accuracy)
	case PathSegQuad:
		return s.Quad.InvArclen(arclen, accuracy)
	}
	return s.Cubic.InvArclen(arclen, accuracy)
}

// AsPathEl is PathSeg::as_path_el in kurbo kurbo/src/bezpath.rs.
func (s PathSeg) AsPathEl() PathEl {
	switch s.Kind {
	case PathSegLine:
		return LineTo(s.Line.P1)
	case PathSegQuad:
		return QuadTo(s.Quad.P1, s.Quad.P2)
	}
	return CurveTo(s.Cubic.P1, s.Cubic.P2, s.Cubic.P3)
}

// Reverse is PathSeg::reverse in kurbo kurbo/src/bezpath.rs.
func (s PathSeg) Reverse() PathSeg {
	switch s.Kind {
	case PathSegLine:
		s.Line = NewLine(s.Line.P1, s.Line.P0)
	case PathSegQuad:
		s.Quad = NewQuadBez(s.Quad.P2, s.Quad.P1, s.Quad.P0)
	default:
		s.Cubic = NewCubicBez(s.Cubic.P3, s.Cubic.P2, s.Cubic.P1, s.Cubic.P0)
	}
	return s
}

// Segments is segments in kurbo kurbo/src/bezpath.rs: the segments of a
// sequence of path elements.
//
// Port: kurbo gives an iterator that panics on a first element ClosePath.
// Segments returns a slice, and a ClosePath before any point gives no
// segment.
func Segments(elements []PathEl) []PathSeg {
	var segs []PathSeg
	var start, last Point
	started := false
	for _, el := range elements {
		if !started {
			switch el.Kind {
			case PathElMoveTo, PathElLineTo:
				start, last = el.P[0], el.P[0]
			case PathElQuadTo:
				start, last = el.P[1], el.P[1]
			case PathElCurveTo:
				start, last = el.P[2], el.P[2]
			case PathElClosePath:
				continue
			}
			started = true
		}
		switch el.Kind {
		case PathElMoveTo:
			start, last = el.P[0], el.P[0]
		case PathElLineTo:
			segs = append(segs, PathSeg{Kind: PathSegLine, Line: NewLine(last, el.P[0])})
			last = el.P[0]
		case PathElQuadTo:
			segs = append(segs, PathSeg{Kind: PathSegQuad, Quad: NewQuadBez(last, el.P[0], el.P[1])})
			last = el.P[1]
		case PathElCurveTo:
			segs = append(segs, PathSeg{Kind: PathSegCubic, Cubic: NewCubicBez(last, el.P[0], el.P[1], el.P[2])})
			last = el.P[2]
		case PathElClosePath:
			if last != start {
				segs = append(segs, PathSeg{Kind: PathSegLine, Line: NewLine(last, start)})
				last = start
			}
		}
	}
	return segs
}
