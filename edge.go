// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"math/bits"

	"github.com/xo/okage/internal/wide"
)

// maxCoeffShift is MAX_COEFF_SHIFT in tiny-skia src/edge.rs.
//
// We store 1<<shift in a (signed) byte, so its maximum value is 1<<6 == 64.
//
// Note that this limits the number of lines we use to approximate a curve.
// If we need to increase this, we need to store curve_count in something
// larger than i8.
const maxCoeffShift int32 = 6

// edgeKind is the variant of the enum Edge in tiny-skia src/edge.rs.
type edgeKind uint8

// The values of edgeKind.
const (
	edgeKindLine edgeKind = iota
	edgeKindQuadratic
	edgeKindCubic
)

// edge is Edge in tiny-skia src/edge.rs.
//
// Port: the Rust enum carries data, so it becomes a struct with a kind field,
// as path.PathSegment does. line is the data of Edge::Line, quad the data of
// Edge::Quadratic and cubic the data of Edge::Cubic. The other two are zero.
type edge struct {
	kind  edgeKind
	line  lineEdge
	quad  quadraticEdge
	cubic cubicEdge
}

// asLine is Edge::as_line and Edge::as_line_mut in tiny-skia src/edge.rs.
//
// Port: the Deref and DerefMut impls of Edge also call it, so Go code writes
// e.asLine().x where Rust writes e.x.
func (e *edge) asLine() *lineEdge {
	switch e.kind {
	case edgeKindQuadratic:
		return &e.quad.line
	case edgeKindCubic:
		return &e.cubic.line
	}
	return &e.line
}

// optionU32 is the Rust Option<u32>. It ports no upstream item.
type optionU32 struct {
	v  uint32
	ok bool
}

// someU32 is Some(v) for an Option<u32>.
func someU32(v uint32) optionU32 {
	return optionU32{v: v, ok: true}
}

// unwrap is Option::unwrap. It panics if o is None.
func (o optionU32) unwrap() uint32 {
	if !o.ok {
		panic("called `Option::unwrap()` on a `None` value")
	}
	return o.v
}

// lineEdge is LineEdge in tiny-skia src/edge.rs.
type lineEdge struct {
	// Imitate a linked list.
	prev optionU32
	next optionU32

	x       fDot16
	dx      fDot16
	firstY  int32
	lastY   int32
	winding int8 // 1 or -1
}

// newLineEdge is LineEdge::new in tiny-skia src/edge.rs.
func newLineEdge(p0, p1 Point, shift int32) (lineEdge, bool) {
	scale := float32(int32(1) << (shift + 6))
	x0 := wide.TruncI32(float32(p0.X * scale))
	y0 := wide.TruncI32(float32(p0.Y * scale))
	x1 := wide.TruncI32(float32(p1.X * scale))
	y1 := wide.TruncI32(float32(p1.Y * scale))

	winding := int8(1)

	if y0 > y1 {
		x0, x1 = x1, x0
		y0, y1 = y1, y0
		winding = -1
	}

	top := fdot6Round(y0)
	bottom := fdot6Round(y1)

	// are we a zero-height line?
	if top == bottom {
		return lineEdge{}, false
	}

	slope := fdot6Div(x1-x0, y1-y0)
	dy := computeDy(top, y0)

	return lineEdge{
		x:       fdot6ToFDot16(x0 + fdot16Mul(slope, dy)),
		dx:      slope,
		firstY:  top,
		lastY:   bottom - 1,
		winding: winding,
	}, true
}

// isVertical is LineEdge::is_vertical in tiny-skia src/edge.rs.
func (e *lineEdge) isVertical() bool {
	return e.dx == 0
}

// update is LineEdge::update in tiny-skia src/edge.rs.
func (e *lineEdge) update(x0, y0, x1, y1 fDot16) bool {
	y0 >>= 10
	y1 >>= 10

	top := fdot6Round(y0)
	bottom := fdot6Round(y1)

	// are we a zero-height line?
	if top == bottom {
		return false
	}

	x0 >>= 10
	x1 >>= 10

	slope := fdot6Div(x1-x0, y1-y0)
	dy := computeDy(top, y0)

	e.x = fdot6ToFDot16(x0 + fdot16Mul(slope, dy))
	e.dx = slope
	e.firstY = top
	e.lastY = bottom - 1

	return true
}

// quadraticEdge is QuadraticEdge in tiny-skia src/edge.rs.
type quadraticEdge struct {
	line       lineEdge
	curveCount int8
	curveShift uint8 // applied to all dx/ddx/dddx
	qx         fDot16
	qy         fDot16
	qdx        fDot16
	qdy        fDot16
	qddx       fDot16
	qddy       fDot16
	qLastX     fDot16
	qLastY     fDot16
}

// newQuadraticEdge is QuadraticEdge::new in tiny-skia src/edge.rs.
func newQuadraticEdge(points []Point, shift int32) (quadraticEdge, bool) {
	quad, ok := newQuadraticEdge2(points, shift)
	if !ok {
		return quadraticEdge{}, false
	}
	if quad.update() {
		return quad, true
	}
	return quadraticEdge{}, false
}

// newQuadraticEdge2 is QuadraticEdge::new2 in tiny-skia src/edge.rs.
func newQuadraticEdge2(points []Point, shift int32) (quadraticEdge, bool) {
	scale := float32(int32(1) << (shift + 6))
	x0 := wide.TruncI32(float32(points[0].X * scale))
	y0 := wide.TruncI32(float32(points[0].Y * scale))
	x1 := wide.TruncI32(float32(points[1].X * scale))
	y1 := wide.TruncI32(float32(points[1].Y * scale))
	x2 := wide.TruncI32(float32(points[2].X * scale))
	y2 := wide.TruncI32(float32(points[2].Y * scale))

	winding := int8(1)
	if y0 > y2 {
		x0, x2 = x2, x0
		y0, y2 = y2, y0
		winding = -1
	}

	top := fdot6Round(y0)
	bottom := fdot6Round(y2)

	// are we a zero-height quad (line)?
	if top == bottom {
		return quadraticEdge{}, false
	}

	// compute number of steps needed (1 << shift)
	{
		dx := (leftShift(x1, 1) - x0 - x2) >> 2
		dy := (leftShift(y1, 1) - y0 - y2) >> 2
		// This is a little confusing:
		// before this line, shift is the scale up factor for AA,
		// after this line, shift is the fCurveShift.
		shift = diffToShift(dx, dy, shift)
	}

	// need at least 1 subdivision for our bias trick
	if shift == 0 {
		shift = 1
	} else if shift > maxCoeffShift {
		shift = maxCoeffShift
	}

	curveCount := int8(int32(1) << shift)

	// We want to reformulate into polynomial form, to make it clear how we
	// forward-difference.
	//
	// p0 (1 - t)^2 + p1 t(1 - t) + p2 t^2 ==> At^2 + Bt + C
	//
	// A = p0 - 2p1 + p2
	// B = 2(p1 - p0)
	// C = p0
	//
	// Our caller must have constrained our inputs (p0..p2) to all fit into
	// 16.16. However, as seen above, we sometimes compute values that can be
	// larger (e.g. B = 2*(p1 - p0)). To guard against overflow, we will store
	// A and B at 1/2 of their actual value, and just apply a 2x scale during
	// application in updateQuadratic(). Hence we store (shift - 1) in
	// curve_shift.

	curveShift := uint8(shift - 1)

	a := fdot6ToFixedDiv2(x0 - x1 - x1 + x2) // 1/2 the real value
	b := fdot6ToFDot16(x1 - x0)              // 1/2 the real value

	qx := fdot6ToFDot16(x0)
	qdx := b + (a >> shift)  // biased by shift
	qddx := a >> (shift - 1) // biased by shift

	a = fdot6ToFixedDiv2(y0 - y1 - y1 + y2) // 1/2 the real value
	b = fdot6ToFDot16(y1 - y0)              // 1/2 the real value

	qy := fdot6ToFDot16(y0)
	qdy := b + (a >> shift)  // biased by shift
	qddy := a >> (shift - 1) // biased by shift

	qLastX := fdot6ToFDot16(x2)
	qLastY := fdot6ToFDot16(y2)

	return quadraticEdge{
		line: lineEdge{
			winding: winding,
		},
		curveCount: curveCount,
		curveShift: curveShift,
		qx:         qx,
		qy:         qy,
		qdx:        qdx,
		qdy:        qdy,
		qddx:       qddx,
		qddy:       qddy,
		qLastX:     qLastX,
		qLastY:     qLastY,
	}, true
}

// update is QuadraticEdge::update in tiny-skia src/edge.rs.
func (q *quadraticEdge) update() bool {
	var success bool
	count := q.curveCount
	oldx := q.qx
	oldy := q.qy
	dx := q.qdx
	dy := q.qdy
	var newx, newy fDot16
	shift := q.curveShift

	for {
		count--
		if count > 0 {
			newx = oldx + (dx >> shift)
			dx += q.qddx
			newy = oldy + (dy >> shift)
			dy += q.qddy
		} else {
			// last segment
			newx = q.qLastX
			newy = q.qLastY
		}
		success = q.line.update(oldx, oldy, newx, newy)
		oldx = newx
		oldy = newy

		if count == 0 || success {
			break
		}
	}

	q.qx = newx
	q.qy = newy
	q.qdx = dx
	q.qdy = dy
	q.curveCount = count

	return success
}

// cubicEdge is CubicEdge in tiny-skia src/edge.rs.
type cubicEdge struct {
	line       lineEdge
	curveCount int8
	curveShift uint8 // applied to all dx/ddx/dddx except for dshift exception
	dshift     uint8 // applied to cdx and cdy
	cx         fDot16
	cy         fDot16
	cdx        fDot16
	cdy        fDot16
	cddx       fDot16
	cddy       fDot16
	cdddx      fDot16
	cdddy      fDot16
	cLastX     fDot16
	cLastY     fDot16
}

// newCubicEdge is CubicEdge::new in tiny-skia src/edge.rs.
func newCubicEdge(points []Point, shift int32) (cubicEdge, bool) {
	cubic, ok := newCubicEdge2(points, shift, true)
	if !ok {
		return cubicEdge{}, false
	}
	if cubic.update() {
		return cubic, true
	}
	return cubicEdge{}, false
}

// newCubicEdge2 is CubicEdge::new2 in tiny-skia src/edge.rs.
func newCubicEdge2(points []Point, shift int32, sortY bool) (cubicEdge, bool) {
	scale := float32(int32(1) << (shift + 6))
	x0 := wide.TruncI32(float32(points[0].X * scale))
	y0 := wide.TruncI32(float32(points[0].Y * scale))
	x1 := wide.TruncI32(float32(points[1].X * scale))
	y1 := wide.TruncI32(float32(points[1].Y * scale))
	x2 := wide.TruncI32(float32(points[2].X * scale))
	y2 := wide.TruncI32(float32(points[2].Y * scale))
	x3 := wide.TruncI32(float32(points[3].X * scale))
	y3 := wide.TruncI32(float32(points[3].Y * scale))

	winding := int8(1)
	if sortY && y0 > y3 {
		x0, x3 = x3, x0
		x1, x2 = x2, x1
		y0, y3 = y3, y0
		y1, y2 = y2, y1
		winding = -1
	}

	top := fdot6Round(y0)
	bot := fdot6Round(y3)

	// are we a zero-height cubic (line)?
	if sortY && top == bot {
		return cubicEdge{}, false
	}

	// compute number of steps needed (1 << shift)
	{
		// Cannot use (center of curve - center of baseline), since center-of-curve
		// need not be the max delta from the baseline (it can even be coincident)
		// so we try just looking at the two off-curve points
		dx := cubicDeltaFromLine(x0, x1, x2, x3)
		dy := cubicDeltaFromLine(y0, y1, y2, y3)
		// add 1 (by observation)
		shift = diffToShift(dx, dy, 2) + 1
	}
	// need at least 1 subdivision for our bias trick
	if shift > maxCoeffShift {
		shift = maxCoeffShift
	}

	// Since our in coming data is initially shifted down by 10 (or 8 in
	// antialias). That means the most we can shift up is 8. However, we
	// compute coefficients with a 3*, so the safest upshift is really 6
	upShift := int32(6) // largest safe value
	downShift := shift + upShift - 10
	if downShift < 0 {
		downShift = 0
		upShift = 10 - shift
	}

	curveCount := int8(leftShift(-1, shift))
	curveShift := uint8(shift)
	dshift := uint8(downShift)

	b := fdot6UpShift(3*(x1-x0), upShift)
	c := fdot6UpShift(3*(x0-x1-x1+x2), upShift)
	d := fdot6UpShift(x3+3*(x1-x2)-x0, upShift)

	cx := fdot6ToFDot16(x0)
	cdx := b + (c >> shift) + (d >> (2 * shift)) // biased by shift
	cddx := 2*c + ((3 * d) >> (shift - 1))       // biased by 2*shift
	cdddx := (3 * d) >> (shift - 1)              // biased by 2*shift

	b = fdot6UpShift(3*(y1-y0), upShift)
	c = fdot6UpShift(3*(y0-y1-y1+y2), upShift)
	d = fdot6UpShift(y3+3*(y1-y2)-y0, upShift)

	cy := fdot6ToFDot16(y0)
	cdy := b + (c >> shift) + (d >> (2 * shift)) // biased by shift
	cddy := 2*c + ((3 * d) >> (shift - 1))       // biased by 2*shift
	cdddy := (3 * d) >> (shift - 1)              // biased by 2*shift

	cLastX := fdot6ToFDot16(x3)
	cLastY := fdot6ToFDot16(y3)

	return cubicEdge{
		line: lineEdge{
			winding: winding,
		},
		curveCount: curveCount,
		curveShift: curveShift,
		dshift:     dshift,
		cx:         cx,
		cy:         cy,
		cdx:        cdx,
		cdy:        cdy,
		cddx:       cddx,
		cddy:       cddy,
		cdddx:      cdddx,
		cdddy:      cdddy,
		cLastX:     cLastX,
		cLastY:     cLastY,
	}, true
}

// update is CubicEdge::update in tiny-skia src/edge.rs.
func (c *cubicEdge) update() bool {
	var success bool
	count := c.curveCount
	oldx := c.cx
	oldy := c.cy
	var newx, newy fDot16
	ddshift := c.curveShift
	dshift := c.dshift

	for {
		count++
		if count < 0 {
			newx = oldx + (c.cdx >> dshift)
			c.cdx += c.cddx >> ddshift
			c.cddx += c.cdddx

			newy = oldy + (c.cdy >> dshift)
			c.cdy += c.cddy >> ddshift
			c.cddy += c.cdddy
		} else {
			// last segment
			newx = c.cLastX
			newy = c.cLastY
		}

		// we want to say debug_assert(oldy <= newy), but our finite fixedpoint
		// does not always achieve that, so we have to explicitly pin it here.
		if newy < oldy {
			newy = oldy
		}

		success = c.line.update(oldx, oldy, newx, newy)
		oldx = newx
		oldy = newy

		if count == 0 || success {
			break
		}
	}

	c.cx = newx
	c.cy = newy
	c.curveCount = count

	return success
}

// computeDy is compute_dy in tiny-skia src/edge.rs.
//
// This correctly favors the lower-pixel when y0 is on a 1/2 pixel boundary.
func computeDy(top, y0 fDot6) fDot6 {
	return leftShift(top, 6) + 32 - y0
}

// diffToShift is diff_to_shift in tiny-skia src/edge.rs.
func diffToShift(dx, dy fDot6, shiftAA int32) int32 {
	// cheap calc of distance from center of p0-p2 to the center of the curve
	dist := cheapDistance(dx, dy)

	// shift down dist (it is currently in dot6)
	// down by 3 gives us 1/8 pixel accuracy (assuming our dist is accurate...)
	// this is chosen by heuristic: make it as big as possible (to minimize segments)
	// ... but small enough so that our curves still look smooth
	// When shift > 0, we are using AA and everything is scaled up so we can
	// lower the accuracy.
	dist = (dist + (int32(1) << (2 + shiftAA))) >> (3 + shiftAA)

	// each subdivision (shift value) cuts this dist (error) by 1/4
	return (32 - int32(bits.LeadingZeros32(uint32(dist)))) >> 1
}

// cheapDistance is cheap_distance in tiny-skia src/edge.rs.
func cheapDistance(dx, dy fDot6) fDot6 {
	dx = absI32(dx)
	dy = absI32(dy)
	// return max + min/2
	if dx > dy {
		return dx + (dy >> 1)
	}
	return dy + (dx >> 1)
}

// fdot6ToFixedDiv2 is fdot6_to_fixed_div2 in tiny-skia src/edge.rs.
//
// In LineEdge::new, QuadraticEdge::new, CubicEdge::new, the first thing we do
// is to convert the points into FDot6. This is modulated by the shift
// parameter, which will either be 0, or something like 2 for antialiasing.
//
// In the float case, we want to turn the float into .6 by saying pt * 64, or
// pt * 256 for antialiasing. This is implemented as 1 << (shift + 6).
//
// In the fixed case, we want to turn the fixed into .6 by saying pt >> 10, or
// pt >> 8 for antialiasing. This is implemented as pt >> (10 - shift).
func fdot6ToFixedDiv2(value fDot6) fDot16 {
	// we want to return SkFDot6ToFixed(value >> 1), but we do not want to throw
	// away data in value, so just perform a modify up-shift
	return leftShift(value, 16-6-1)
}

// fdot6UpShift is fdot6_up_shift in tiny-skia src/edge.rs.
func fdot6UpShift(x fDot6, upShift int32) int32 {
	return leftShift(x, upShift)
}

// cubicDeltaFromLine is cubic_delta_from_line in tiny-skia src/edge.rs.
//
// f(1/3) = (8a + 12b + 6c + d) / 27
// f(2/3) = (a + 6b + 12c + 8d) / 27
//
// f(1/3)-b = (8a - 15b + 6c + d) / 27
// f(2/3)-c = (a + 6b - 15c + 8d) / 27
//
// use 16/512 to approximate 1/27.
func cubicDeltaFromLine(a, b, c, d fDot6) fDot6 {
	// since our parameters can be negative, we do not use <<
	oneThird := ((a*8 - b*15 + 6*c + d) * 19) >> 9
	twoThird := ((a + 6*b - c*15 + d*8) * 19) >> 9

	return maxI32(absI32(oneThird), absI32(twoThird))
}
