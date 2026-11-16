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
