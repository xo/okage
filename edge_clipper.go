// Copyright 2009 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import "github.com/xo/okage/path"

// Port: upstream has the macros copy_3_points and copy_4_points, which copy 3
// or 4 points of an array into a new array. Go copies a part of an array into
// an array with a conversion, such as [3]Point(arr[i : i+3]).

// maxVerbs is MAX_VERBS in tiny-skia src/edge_clipper.rs.
//
// Max curvature in X and Y split cubic into 9 pieces, * (line + cubic).
const maxVerbs = 18

// clippedEdges is ClippedEdges in tiny-skia src/edge_clipper.rs.
//
// Port: upstream uses an ArrayVec<PathEdge, MAX_VERBS>. Go uses a fixed
// array with a length. push panics when the array is full, as ArrayVec::push
// does.
type clippedEdges struct {
	buf [maxVerbs]pathEdge
	n   int
}

// push is ArrayVec::push.
func (ce *clippedEdges) push(e pathEdge) {
	if ce.n >= maxVerbs {
		panic("ArrayVec: capacity exceeded in push")
	}
	ce.buf[ce.n] = e
	ce.n++
}

// isEmpty is ArrayVec::is_empty.
func (ce *clippedEdges) isEmpty() bool {
	return ce.n == 0
}

// slice returns the edges of ce.
func (ce *clippedEdges) slice() []pathEdge {
	return ce.buf[:ce.n]
}

// edgeClipper is EdgeClipper in tiny-skia src/edge_clipper.rs.
type edgeClipper struct {
	clip              Rect
	canCullToTheRight bool
	edges             clippedEdges
}

// newEdgeClipper is EdgeClipper::new in tiny-skia src/edge_clipper.rs.
func newEdgeClipper(clip Rect, canCullToTheRight bool) edgeClipper {
	return edgeClipper{
		clip:              clip,
		canCullToTheRight: canCullToTheRight,
	}
}

// clipLine is EdgeClipper::clip_line in tiny-skia src/edge_clipper.rs.
func (ec *edgeClipper) clipLine(p0, p1 Point) (clippedEdges, bool) {
	var pointsArr [maxPoints]Point
	points := clip(&[2]Point{p0, p1}, &ec.clip, ec.canCullToTheRight, &pointsArr)
	if len(points) != 0 {
		for i := range len(points) - 1 {
			ec.pushLine(points[i], points[i+1])
		}
	}

	if ec.edges.isEmpty() {
		return clippedEdges{}, false
	}
	return ec.edges, true
}

// pushLine is EdgeClipper::push_line in tiny-skia src/edge_clipper.rs.
func (ec *edgeClipper) pushLine(p0, p1 Point) {
	ec.edges.push(pathEdgeLineTo(p0, p1))
}

// pushVLine is EdgeClipper::push_vline in tiny-skia src/edge_clipper.rs.
func (ec *edgeClipper) pushVLine(x, y0, y1 float32, reverse bool) {
	if reverse {
		y0, y1 = y1, y0
	}

	ec.edges.push(pathEdgeLineTo(
		path.PointFromXY(x, y0),
		path.PointFromXY(x, y1),
	))
}

// clipQuad is EdgeClipper::clip_quad in tiny-skia src/edge_clipper.rs.
func (ec *edgeClipper) clipQuad(p0, p1, p2 Point) (clippedEdges, bool) {
	pts := [3]Point{p0, p1, p2}
	bounds, ok := path.RectFromPoints(pts[:])
	if !ok {
		return clippedEdges{}, false
	}

	if !quickReject(&bounds, &ec.clip) {
		var monoY [5]Point
		countY := chopQuadAtYExtrema(&pts, &monoY)
		for y := 0; y <= countY; y++ {
			var monoX [5]Point
			yPoints := [3]Point(monoY[y*2 : y*2+3])
			countX := chopQuadAtXExtrema(&yPoints, &monoX)
			for x := 0; x <= countX; x++ {
				xPoints := [3]Point(monoX[x*2 : x*2+3])
				ec.clipMonoQuad(&xPoints)
			}
		}
	}

	if ec.edges.isEmpty() {
		return clippedEdges{}, false
	}
	return ec.edges, true
}

// clipMonoQuad is EdgeClipper::clip_mono_quad in tiny-skia
// src/edge_clipper.rs.
//
// src[] must be monotonic in X and Y.
func (ec *edgeClipper) clipMonoQuad(src *[3]Point) {
	var pts [3]Point
	reverse := sortIncreasingY(src[:], pts[:])

	// are we completely above or below
	if pts[2].Y <= ec.clip.Top() || pts[0].Y >= ec.clip.Bottom() {
		return
	}

	// Now chop so that pts is contained within clip in Y
	chopQuadInY(&ec.clip, &pts)

	if pts[0].X > pts[2].X {
		pts[0], pts[2] = pts[2], pts[0]
		reverse = !reverse
	}

	// Now chop in X has needed, and record the segments

	if pts[2].X <= ec.clip.Left() {
		// wholly to the left
		ec.pushVLine(ec.clip.Left(), pts[0].Y, pts[2].Y, reverse)
		return
	}

	if pts[0].X >= ec.clip.Right() {
		// wholly to the right
		if !ec.canCullToTheRight {
			ec.pushVLine(ec.clip.Right(), pts[0].Y, pts[2].Y, reverse)
		}

		return
	}

	t := path.NormalizedF32ExclusiveAny
	var tmp [5]Point

	// are we partially to the left
	if pts[0].X < ec.clip.Left() {
		if chopMonoQuadAtX(&pts, ec.clip.Left(), &t) {
			path.ChopQuadAt(pts[:], t, &tmp)
			ec.pushVLine(ec.clip.Left(), tmp[0].Y, tmp[2].Y, reverse)
			// clamp to clean up imprecise numerics in the chop
			tmp[2].X = ec.clip.Left()
			tmp[3].X = maxF32(tmp[3].X, ec.clip.Left())

			pts[0] = tmp[2]
			pts[1] = tmp[3]
		} else {
			// if chopMonoQuadAtY failed, then we can have hit inexact numerics
			// so we just clamp against the left
			ec.pushVLine(ec.clip.Left(), pts[0].Y, pts[2].Y, reverse)
			return
		}
	}

	// are we partially to the right
	if pts[2].X > ec.clip.Right() {
		if chopMonoQuadAtX(&pts, ec.clip.Right(), &t) {
			path.ChopQuadAt(pts[:], t, &tmp)
			// clamp to clean up imprecise numerics in the chop
			tmp[1].X = minF32(tmp[1].X, ec.clip.Right())
			tmp[2].X = ec.clip.Right()

			ec.pushQuad(&[3]Point{tmp[0], tmp[1], tmp[2]}, reverse)
			ec.pushVLine(ec.clip.Right(), tmp[2].Y, tmp[4].Y, reverse)
		} else {
			// if chopMonoQuadAtY failed, then we can have hit inexact numerics
			// so we just clamp against the right
			pts[1].X = minF32(pts[1].X, ec.clip.Right())
			pts[2].X = minF32(pts[2].X, ec.clip.Right())
			ec.pushQuad(&pts, reverse)
		}
	} else {
		// wholly inside the clip
		ec.pushQuad(&pts, reverse)
	}
}

// pushQuad is EdgeClipper::push_quad in tiny-skia src/edge_clipper.rs.
func (ec *edgeClipper) pushQuad(pts *[3]Point, reverse bool) {
	if reverse {
		ec.edges.push(pathEdgeQuadTo(pts[2], pts[1], pts[0]))
	} else {
		ec.edges.push(pathEdgeQuadTo(pts[0], pts[1], pts[2]))
	}
}

// clipCubic is EdgeClipper::clip_cubic in tiny-skia src/edge_clipper.rs.
func (ec *edgeClipper) clipCubic(p0, p1, p2, p3 Point) (clippedEdges, bool) {
	pts := [4]Point{p0, p1, p2, p3}
	bounds, ok := path.RectFromPoints(pts[:])
	if !ok {
		return clippedEdges{}, false
	}

	// check if we are clipped out vertically
	if bounds.Bottom() > ec.clip.Top() && bounds.Top() < ec.clip.Bottom() {
		if tooBigForReliableFloatMath(&bounds) {
			// cannot safely clip the cubic, so we give up and draw a line (which we can safely clip)
			//
			// If we rewrote chopcubicat*extrema and chopmonocubic using doubles, we can very
			// likely always handle the cubic safely, but (it seems) at a big loss in speed, so
			// we only want to take that alternate impl if needed.
			return ec.clipLine(p0, p3)
		}

		var monoY [10]Point
		countY := chopCubicAtYExtrema(&pts, &monoY)
		for y := 0; y <= countY; y++ {
			var monoX [10]Point
			yPoints := [4]Point(monoY[y*3 : y*3+4])
			countX := chopCubicAtXExtrema(&yPoints, &monoX)
			for x := 0; x <= countX; x++ {
				xPoints := [4]Point(monoX[x*3 : x*3+4])
				ec.clipMonoCubic(&xPoints)
			}
		}
	}

	if ec.edges.isEmpty() {
		return clippedEdges{}, false
	}
	return ec.edges, true
}

// clipMonoCubic is EdgeClipper::clip_mono_cubic in tiny-skia
// src/edge_clipper.rs.
//
// src[] must be monotonic in X and Y.
func (ec *edgeClipper) clipMonoCubic(src *[4]Point) {
	var pts [4]Point
	reverse := sortIncreasingY(src[:], pts[:])

	// are we completely above or below
	if pts[3].Y <= ec.clip.Top() || pts[0].Y >= ec.clip.Bottom() {
		return
	}

	// Now chop so that pts is contained within clip in Y
	chopCubicInY(&ec.clip, &pts)

	if pts[0].X > pts[3].X {
		pts[0], pts[3] = pts[3], pts[0]
		pts[1], pts[2] = pts[2], pts[1]
		reverse = !reverse
	}

	// Now chop in X has needed, and record the segments

	if pts[3].X <= ec.clip.Left() {
		// wholly to the left
		ec.pushVLine(ec.clip.Left(), pts[0].Y, pts[3].Y, reverse)
		return
	}

	if pts[0].X >= ec.clip.Right() {
		// wholly to the right
		if !ec.canCullToTheRight {
			ec.pushVLine(ec.clip.Right(), pts[0].Y, pts[3].Y, reverse)
		}

		return
	}

	// are we partially to the left
	if pts[0].X < ec.clip.Left() {
		var tmp [7]Point
		chopMonoCubicAtXEdgeClipper(&pts, ec.clip.Left(), &tmp)
		ec.pushVLine(ec.clip.Left(), tmp[0].Y, tmp[3].Y, reverse)

		// tmp[3, 4].fX must all be to the right of clip.left().
		// Since we cannot trust the numerics of
		// the chopper, we force those conditions now
		tmp[3].X = ec.clip.Left()
		tmp[4].X = maxF32(tmp[4].X, ec.clip.Left())

		pts[0] = tmp[3]
		pts[1] = tmp[4]
		pts[2] = tmp[5]
	}

	// are we partially to the right
	if pts[3].X > ec.clip.Right() {
		var tmp [7]Point
		chopMonoCubicAtXEdgeClipper(&pts, ec.clip.Right(), &tmp)
		tmp[3].X = ec.clip.Right()
		tmp[2].X = minF32(tmp[2].X, ec.clip.Right())

		ec.pushCubic(&[4]Point{tmp[0], tmp[1], tmp[2], tmp[3]}, reverse)
		ec.pushVLine(ec.clip.Right(), tmp[3].Y, tmp[6].Y, reverse)
	} else {
		// wholly inside the clip
		ec.pushCubic(&pts, reverse)
	}
}

// pushCubic is EdgeClipper::push_cubic in tiny-skia src/edge_clipper.rs.
func (ec *edgeClipper) pushCubic(pts *[4]Point, reverse bool) {
	if reverse {
		ec.edges.push(pathEdgeCubicTo(pts[3], pts[2], pts[1], pts[0]))
	} else {
		ec.edges.push(pathEdgeCubicTo(pts[0], pts[1], pts[2], pts[3]))
	}
}

// edgeClipperIter is EdgeClipperIter in tiny-skia src/edge_clipper.rs.
type edgeClipperIter struct {
	edgeIter          pathEdgeIter
	clip              Rect
	canCullToTheRight bool
}

// newEdgeClipperIter is EdgeClipperIter::new in tiny-skia
// src/edge_clipper.rs.
func newEdgeClipperIter(p *Path, clip Rect, canCullToTheRight bool) edgeClipperIter {
	return edgeClipperIter{
		edgeIter:          edgeIter(p),
		clip:              clip,
		canCullToTheRight: canCullToTheRight,
	}
}

// next is the Iterator impl of EdgeClipperIter in tiny-skia
// src/edge_clipper.rs.
func (it *edgeClipperIter) next() (clippedEdges, bool) {
	for {
		e, ok := it.edgeIter.next()
		if !ok {
			break
		}

		clipper := newEdgeClipper(it.clip, it.canCullToTheRight)

		switch e.kind {
		case pathEdgeKindLineTo:
			if edges, ok := clipper.clipLine(e.points[0], e.points[1]); ok {
				return edges, true
			}
		case pathEdgeKindQuadTo:
			if edges, ok := clipper.clipQuad(e.points[0], e.points[1], e.points[2]); ok {
				return edges, true
			}
		case pathEdgeKindCubicTo:
			if edges, ok := clipper.clipCubic(e.points[0], e.points[1], e.points[2], e.points[3]); ok {
				return edges, true
			}
		}
	}

	return clippedEdges{}, false
}

// quickReject is quick_reject in tiny-skia src/edge_clipper.rs.
func quickReject(bounds, clip *Rect) bool {
	return bounds.Top() >= clip.Bottom() || bounds.Bottom() <= clip.Top()
}

// sortIncreasingY is sort_increasing_y in tiny-skia src/edge_clipper.rs.
//
// src[] must be monotonic in Y. This routine copies src into dst, and sorts
// it to be increasing in Y. If it had to reverse the order of the points, it
// returns true, otherwise it returns false.
func sortIncreasingY(src, dst []Point) bool {
	// We need the data to be monotonically increasing in Y.
	// Never fails, because src is always non-empty.
	if src[0].Y > src[len(src)-1].Y {
		for i := range src {
			dst[i] = src[len(src)-1-i]
		}

		return true
	}
	copy(dst[0:len(src)], src)
	return false
}

// chopQuadInY is chop_quad_in_y in tiny-skia src/edge_clipper.rs. It modifies
// pts[] in place so that it is clipped in Y to the clip rect.
func chopQuadInY(clip *Rect, pts *[3]Point) {
	t := path.NormalizedF32ExclusiveAny
	var tmp [5]Point

	// are we partially above
	if pts[0].Y < clip.Top() {
		if chopMonoQuadAtY(pts, clip.Top(), &t) {
			// take the 2nd chopped quad
			path.ChopQuadAt(pts[:], t, &tmp)
			// clamp to clean up imprecise numerics in the chop
			tmp[2].Y = clip.Top()
			tmp[3].Y = maxF32(tmp[3].Y, clip.Top())

			pts[0] = tmp[2]
			pts[1] = tmp[3]
		} else {
			// if chop_mono_quad_at_y failed, then we can have hit inexact numerics
			// so we just clamp against the top
			for i := range pts {
				if pts[i].Y < clip.Top() {
					pts[i].Y = clip.Top()
				}
			}
		}
	}

	// are we partially below
	if pts[2].Y > clip.Bottom() {
		if chopMonoQuadAtY(pts, clip.Bottom(), &t) {
			path.ChopQuadAt(pts[:], t, &tmp)
			// clamp to clean up imprecise numerics in the chop
			tmp[1].Y = minF32(tmp[1].Y, clip.Bottom())
			tmp[2].Y = clip.Bottom()

			pts[1] = tmp[1]
			pts[2] = tmp[2]
		} else {
			// if chop_mono_quad_at_y failed, then we can have hit inexact numerics
			// so we just clamp against the bottom
			for i := range pts {
				if pts[i].Y > clip.Bottom() {
					pts[i].Y = clip.Bottom()
				}
			}
		}
	}
}

// chopMonoQuadAtX is chop_mono_quad_at_x in tiny-skia src/edge_clipper.rs.
func chopMonoQuadAtX(pts *[3]Point, x float32, t *path.NormalizedF32Exclusive) bool {
	return chopMonoQuadAt(pts[0].X, pts[1].X, pts[2].X, x, t)
}

// chopMonoQuadAtY is chop_mono_quad_at_y in tiny-skia src/edge_clipper.rs.
func chopMonoQuadAtY(pts *[3]Point, y float32, t *path.NormalizedF32Exclusive) bool {
	return chopMonoQuadAt(pts[0].Y, pts[1].Y, pts[2].Y, y, t)
}

// chopMonoQuadAt is chop_mono_quad_at in tiny-skia src/edge_clipper.rs.
func chopMonoQuadAt(c0, c1, c2, target float32, t *path.NormalizedF32Exclusive) bool {
	// Solve F(t) = y where F(t) := [0](1-t)^2 + 2[1]t(1-t) + [2]t^2
	// We solve for t, using quadratic equation, hence we have to rearrange
	// our coefficients to look like At^2 + Bt + C
	a := c0 - c1 - c1 + c2
	b := 2.0 * (c1 - c0)
	c := c0 - target

	roots := path.NewTValues()
	count := path.FindUnitQuadRoots(a, b, c, &roots)
	if count != 0 {
		*t = roots[0]
		return true
	}
	return false
}

// tooBigForReliableFloatMath is too_big_for_reliable_float_math in tiny-skia
// src/edge_clipper.rs.
func tooBigForReliableFloatMath(r *Rect) bool {
	// limit set as the largest float value for which we can still reliably compute things like
	// - chopping at XY extrema
	// - chopping at Y or X values for clipping
	//
	// Current value chosen just by experiment. Larger (and still succeeds) is always better.

	limit := float32(1 << 22)
	return r.Left() < -limit || r.Top() < -limit || r.Right() > limit || r.Bottom() > limit
}

// chopCubicInY is chop_cubic_in_y in tiny-skia src/edge_clipper.rs. It
// modifies pts[] in place so that it is clipped in Y to the clip rect.
func chopCubicInY(clip *Rect, pts *[4]Point) {
	// are we partially above
	if pts[0].Y < clip.Top() {
		var tmp [7]Point
		chopMonoCubicAtYEdgeClipper(pts, clip.Top(), &tmp)

		// For a large range in the points, we can do a poor job of chopping, such that the t
		// we computed resulted in the lower cubic still being partly above the clip.
		//
		// If just the first or first 2 Y values are above the fTop, we can just smash them
		// down. If the first 3 Ys are above fTop, we cannot smash all 3, as that can really
		// distort the cubic. In this case, we take the first output (tmp[3..6] and treat it as
		// a guess, and re-chop against fTop. Then we fall through to checking if we need to
		// smash the first 1 or 2 Y values.
		if tmp[3].Y < clip.Top() && tmp[4].Y < clip.Top() && tmp[5].Y < clip.Top() {
			tmp2 := [4]Point(tmp[3:7])
			chopMonoCubicAtYEdgeClipper(&tmp2, clip.Top(), &tmp)
		}

		// tmp[3, 4].y must all be below clip.fTop.
		// Since we cannot trust the numerics of the chopper, we force those conditions now
		tmp[3].Y = clip.Top()
		tmp[4].Y = maxF32(tmp[4].Y, clip.Top())

		pts[0] = tmp[3]
		pts[1] = tmp[4]
		pts[2] = tmp[5]
	}

	// are we partially below
	if pts[3].Y > clip.Bottom() {
		var tmp [7]Point
		chopMonoCubicAtYEdgeClipper(pts, clip.Bottom(), &tmp)
		tmp[3].Y = clip.Bottom()
		tmp[2].Y = minF32(tmp[2].Y, clip.Bottom())

		pts[1] = tmp[1]
		pts[2] = tmp[2]
		pts[3] = tmp[3]
	}
}

// chopMonoCubicAtXEdgeClipper is chop_mono_cubic_at_x in tiny-skia
// src/edge_clipper.rs. The name takes the suffix of its module, because
// src/path_geometry.rs has a function of the same name (D10).
func chopMonoCubicAtXEdgeClipper(src *[4]Point, x float32, dst *[7]Point) {
	if chopMonoCubicAtXPathGeometry(src, x, dst) {
		return
	}

	srcValues := [4]float32{src[0].X, src[1].X, src[2].X, src[3].X}
	path.ChopCubicAt2(src, monoCubicClosestT(&srcValues, x), dst[:])
}

// chopMonoCubicAtYEdgeClipper is chop_mono_cubic_at_y in tiny-skia
// src/edge_clipper.rs. The name takes the suffix of its module, because
// src/path_geometry.rs has a function of the same name (D10).
func chopMonoCubicAtYEdgeClipper(src *[4]Point, y float32, dst *[7]Point) {
	if chopMonoCubicAtYPathGeometry(src, y, dst) {
		return
	}

	srcValues := [4]float32{src[0].Y, src[1].Y, src[2].Y, src[3].Y}
	path.ChopCubicAt2(src, monoCubicClosestT(&srcValues, y), dst[:])
}

// monoCubicClosestT is mono_cubic_closest_t in tiny-skia src/edge_clipper.rs.
func monoCubicClosestT(src *[4]float32, x float32) path.NormalizedF32Exclusive {
	t := float32(0.5)
	var lastT float32
	bestT := t
	step := float32(0.25)
	d := src[0]
	a := src[3] + float32(3.0*(src[1]-src[2])) - d
	b := float32(3.0 * (src[2] - src[1] - src[1] + d))
	c := float32(3.0 * (src[1] - d))
	x -= d
	closest := path.ScalarMax
	for {
		loc := float32((float32(float32(float32(a*t)+b)*t) + c) * t)
		dist := absF32(loc - x)
		if closest > dist {
			closest = dist
			bestT = t
		}

		lastT = t
		if loc < x {
			t += step
		} else {
			t += -step
		}
		step *= 0.5

		if !(closest > 0.25 && lastT != t) {
			break
		}
	}

	n, ok := path.NewNormalizedF32Exclusive(bestT)
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}
	return n
}
