// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports no file of velato. It holds the path that a text follows,
// the key p of a text layer, which velato does not parse (D85). It follows
// LottieTextFollowPath in src/loaders/lottie/tvgLottieModel.cpp of ThorVG
// v1.1.1, with the length of a curve of src/common/tvgMath.cpp.

import (
	"math"

	"github.com/xo/okage/kurbo"
)

// textFollow is the path that a text follows, in the units of a glyph at
// the size 100. Each segment is a cubic curve, as ThorVG makes each segment
// of a path of Lottie.
type textFollow struct {
	segs []kurbo.CubicBez
	lens []float64
	// total is the length of the path.
	total float64
	// closed reports if the path ends with a close.
	closed bool
	// start is the first point of the path.
	start kurbo.Point
}

// newTextFollow returns the path of a text that follows path, which is in
// pixels, for glyphs at the given scale.
func newTextFollow(path []kurbo.PathEl, scale float64) *textFollow {
	inv := kurbo.AffineScale(1 / scale)
	f := &textFollow{}
	var first, last kurbo.Point
	add := func(c kurbo.CubicBez) {
		n := bezierLength(c)
		f.segs = append(f.segs, c)
		f.lens = append(f.lens, n)
		f.total += n
		last = c.P3
	}
	for i, el := range path {
		el = inv.MulPathEl(el)
		switch el.Kind {
		case kurbo.PathElMoveTo:
			first, last = el.P[0], el.P[0]
			if i == 0 {
				f.start = first
			}
		case kurbo.PathElLineTo:
			add(kurbo.NewCubicBez(last, last, el.P[0], el.P[0]))
		case kurbo.PathElQuadTo:
			add(kurbo.QuadBez{P0: last, P1: el.P[0], P2: el.P[1]}.Raise())
		case kurbo.PathElCurveTo:
			add(kurbo.NewCubicBez(last, el.P[0], el.P[1], el.P[2]))
		case kurbo.PathElClosePath:
			if last != first {
				add(kurbo.NewCubicBez(last, last, first, first))
			}
			last = first
		}
	}
	f.closed = len(path) > 0 && path[len(path)-1].Kind == kurbo.PathElClosePath
	return f
}

// position returns the point of f at the length at, as
// LottieTextFollowPath::position of ThorVG finds it. Before the start and
// after the end, a closed path wraps around, and an open path goes on in a
// line from its first or its last segment.
func (f *textFollow) position(at float64) kurbo.Point {
	if len(f.segs) == 0 {
		return f.start
	}
	if at <= 0 {
		if !f.closed {
			c := f.segs[0]
			return extend(c.P0, bezierAngle(c, 0.0001), at)
		}
		for at < 0 {
			at += f.total
		}
	}
	if at >= f.total {
		if !f.closed {
			c := f.segs[len(f.segs)-1]
			return extend(c.P3, bezierAngle(c, 0.999), at-f.total)
		}
		for at > f.total {
			at -= f.total
		}
	}
	current := 0.0
	for i, c := range f.segs {
		n := f.lens[i]
		if current+n < at {
			current += n
			continue
		}
		return c.Eval(bezierAt(c, at-current, n))
	}
	return f.segs[len(f.segs)-1].P3
}

// extend returns the point at the distance d from p, in the direction of
// the given angle in radians.
func extend(p kurbo.Point, angle, d float64) kurbo.Point {
	s, c := math.Sincos(angle)
	return kurbo.NewPoint(float64(d*c)+p.X, float64(d*s)+p.Y)
}

// bezierAngle returns the angle of the tangent of c at t, in radians.
func bezierAngle(c kurbo.CubicBez, t float64) float64 {
	return c.Deriv().Eval(t).ToVec2().Atan2()
}

// bezierLength returns the length of c as Bezier::length of ThorVG
// computes it: it splits the curve in halves until its control polygon is
// longer than its chord by at most bezierEpsilon.
func bezierLength(c kurbo.CubicBez) float64 {
	n := c.P0.Distance(c.P1) + c.P1.Distance(c.P2) + c.P2.Distance(c.P3)
	chord := c.P0.Distance(c.P3)
	if math.Abs(n-chord) > bezierEpsilon {
		left, right := c.Subdivide()
		return bezierLength(left) + bezierLength(right)
	}
	return n
}

// bezierAt returns the parameter of c at the length at, for the length n
// of c, as Bezier::at of ThorVG finds it, by bisection.
func bezierAt(c kurbo.CubicBez, at, n float64) float64 {
	if at <= 0 {
		return 0
	}
	if at >= n {
		return 1
	}
	lo, hi, t := 0.0, 1.0, 0.5
	for {
		n = bezierLength(c.Subsegment(0, t))
		if math.Abs(n-at) < bezierEpsilon || math.Abs(lo-hi) < 1e-3 {
			return t
		}
		if n < at {
			lo = t
			t = (t + hi) * 0.5
		} else {
			hi = t
			t = (lo + t) * 0.5
		}
	}
}
