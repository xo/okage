// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"math"
	"math/bits"

	"github.com/xo/okage/path"
)

// floatPi is FLOAT_PI in tiny-skia src/scan/hairline.rs.
const floatPi float32 = 3.14159265

// lineProc is LineProc in tiny-skia src/scan/hairline.rs.
//
// Port: a nil clip is the Rust None.
type lineProc func(points []Point, clip *screenIntRect, b blitter)

// maxCubicSubdivideLevel is MAX_CUBIC_SUBDIVIDE_LEVEL in tiny-skia
// src/scan/hairline.rs.
const maxCubicSubdivideLevel uint8 = 9

// maxQuadSubdivideLevel is MAX_QUAD_SUBDIVIDE_LEVEL in tiny-skia
// src/scan/hairline.rs.
const maxQuadSubdivideLevel uint8 = 5

// strokePathHairline is stroke_path in tiny-skia src/scan/hairline.rs. The
// name takes the suffix of its module, because src/scan/hairline_aa.rs has a
// function of the same name (D10).
func strokePathHairline(p *Path, lineCap LineCap, clip *screenIntRect, b blitter) {
	strokePathImpl(p, lineCap, clip, hairLineRgn, b)
}

// hairLineRgn is hair_line_rgn in tiny-skia src/scan/hairline.rs.
func hairLineRgn(points []Point, clip *screenIntRect, b blitter) {
	const max = 32767.0
	fixedBounds, ok := path.RectFromLTRB(-max, -max, max, max)
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}

	var clipBounds *Rect
	if clip != nil {
		r := clip.ToRect()
		clipBounds = &r
	}

	for i := range len(points) - 1 {
		var pts [2]Point

		// We have to pre-clip the line to fit in a Fixed, so we just chop the line.
		if !intersect(&[2]Point{points[i], points[i+1]}, &fixedBounds, &pts) {
			continue
		}

		if clipBounds != nil {
			tmp := pts
			// Perform a clip in scalar space, so we catch huge values which can
			// be missed after we convert to FDot6 (overflow).
			if !intersect(&tmp, clipBounds, &pts) {
				continue
			}
		}

		x0 := fdot6FromF32(pts[0].X)
		y0 := fdot6FromF32(pts[0].Y)
		x1 := fdot6FromF32(pts[1].X)
		y1 := fdot6FromF32(pts[1].Y)

		dx := x1 - x0
		dy := y1 - y0

		if absI32(dx) > absI32(dy) {
			// mostly horizontal

			if x0 > x1 {
				// we want to go left-to-right
				x0, x1 = x1, x0
				y0, y1 = y1, y0 //nolint:ineffassign,wastedassign // the swap of upstream is kept
			}

			ix0 := fdot6Round(x0)
			ix1 := fdot6Round(x1)
			if ix0 == ix1 {
				// too short to draw
				continue
			}

			slope := fdot16Div(dy, dx)
			startY := fdot6ToFDot16(y0) + ((slope * ((32 - x0) & 63)) >> 6)

			// In some cases, probably due to precision/rounding issues,
			// `start_y` can become equal to the image height,
			// which will lead to panic, because we will access pixels outside
			// the current memory buffer.
			// This is tiny-skia specific issue. Skia handles this part differently.
			maxY := int32(math.MaxInt32)
			if clipBounds != nil {
				maxY = fdot16FromF32(clipBounds.Bottom())
			}

			for {
				if ix0 >= 0 && startY >= 0 && startY < maxY {
					b.blitH(uint32(ix0), uint32(startY>>16), lengthU32One)
				}

				startY += slope
				ix0++
				if ix0 >= ix1 {
					break
				}
			}
		} else {
			// mostly vertical

			if y0 > y1 {
				// we want to go top-to-bottom
				x0, x1 = x1, x0 //nolint:ineffassign,wastedassign // the swap of upstream is kept
				y0, y1 = y1, y0
			}

			iy0 := fdot6Round(y0)
			iy1 := fdot6Round(y1)
			if iy0 == iy1 {
				// too short to draw
				continue
			}

			slope := fdot16Div(dx, dy)
			startX := fdot6ToFDot16(x0) + ((slope * ((32 - y0) & 63)) >> 6)

			for {
				if startX >= 0 && iy0 >= 0 {
					b.blitH(uint32(startX>>16), uint32(iy0), lengthU32One)
				}

				startX += slope
				iy0++
				if iy0 >= iy1 {
					break
				}
			}
		}
	}
}

// strokePathImpl is stroke_path_impl in tiny-skia src/scan/hairline.rs.
func strokePathImpl(p *Path, lineCap LineCap, clip *screenIntRect, lp lineProc, b blitter) {
	var insetClip *IntRect
	var outsetClip *IntRect

	{
		capOut := float32(2.0)
		if lineCap == path.LineCapButt {
			capOut = 1.0
		}
		r, ok := p.Bounds().Outset(capOut, capOut)
		if !ok {
			return
		}
		ibounds, ok := r.RoundOut()
		if !ok {
			return
		}
		if _, ok := clip.ToIntRect().Intersect(ibounds); !ok {
			return
		}

		if !clip.ToIntRect().Contains(ibounds) {
			// We now cache two scalar rects, to use for culling per-segment (e.g. cubic).
			// Since we are hairlining, the "bounds" of the control points is not necessarily the
			// limit of where a segment can draw (it can draw up to 1 pixel beyond in aa-hairs).
			//
			// Compute the pt-bounds per segment is easy, so we do that, and then inversely adjust
			// the culling bounds so we can just do a straight compare per segment.
			//
			// insetClip is use for quick-accept (i.e. the segment is not clipped), so we inset
			// it from the clip-bounds (since segment bounds can be off by 1).
			//
			// outsetClip is used for quick-reject (i.e. the segment is entirely outside), so we
			// outset it from the clip-bounds.
			v, ok := clip.ToIntRect().MakeOutset(1, 1)
			if !ok {
				return
			}
			outsetClip = &v
			v2, ok := clip.ToIntRect().Inset(1, 1)
			if !ok {
				return
			}
			insetClip = &v2
		}
	}

	prevVerb := path.PathVerbMove
	firstPt := path.PointZero()
	lastPt := path.PointZero()

	iter := p.Segments()
	for {
		segment, ok := iter.Next()
		if !ok {
			break
		}
		verb := iter.CurrVerb()
		nextVerb, hasNextVerb := iter.NextVerb()
		var lastPt2 Point
		switch segment.Kind {
		case path.PathSegmentMoveTo:
			pt := segment.Points[0]
			firstPt = pt
			lastPt = pt
			lastPt2 = pt
		case path.PathSegmentLineTo:
			pt := segment.Points[0]
			points := [2]Point{lastPt, pt}
			if lineCap != path.LineCapButt {
				extendPts(lineCap, prevVerb, nextVerb, hasNextVerb, points[:])
			}

			lp(points[:], clip, b)
			lastPt = pt
			lastPt2 = points[0]
		case path.PathSegmentQuadTo:
			p0, p1 := segment.Points[0], segment.Points[1]
			points := [3]Point{lastPt, p0, p1}
			if lineCap != path.LineCapButt {
				extendPts(lineCap, prevVerb, nextVerb, hasNextVerb, points[:])
			}

			hairQuad(
				&points,
				clip,
				insetClip,
				outsetClip,
				computeQuadLevel(&points),
				lp,
				b,
			)

			lastPt = p1
			lastPt2 = points[0]
		case path.PathSegmentCubicTo:
			p0, p1, p2 := segment.Points[0], segment.Points[1], segment.Points[2]
			points := [4]Point{lastPt, p0, p1, p2}
			if lineCap != path.LineCapButt {
				extendPts(lineCap, prevVerb, nextVerb, hasNextVerb, points[:])
			}

			hairCubic(
				&points,
				clip,
				insetClip,
				outsetClip,
				lp,
				b,
			)

			lastPt = p2
			lastPt2 = points[0]
		case path.PathSegmentClose:
			points := [2]Point{lastPt, firstPt}
			if lineCap != path.LineCapButt && prevVerb == path.PathVerbMove {
				// cap moveTo/close to match svg expectations for degenerate segments
				extendPts(lineCap, prevVerb, nextVerb, hasNextVerb, points[:])
			}
			lp(points[:], clip, b)
			lastPt2 = points[0]
		}

		if lineCap != path.LineCapButt {
			if prevVerb == path.PathVerbMove &&
				(verb == path.PathVerbLine || verb == path.PathVerbQuad || verb == path.PathVerbCubic) {
				firstPt = lastPt2 // the curve moved the initial point, so close to it instead
			}

			prevVerb = verb
		}
	}
}

// extendPts is extend_pts in tiny-skia src/scan/hairline.rs.
//
// Extend the points in the direction of the starting or ending tangent by 1/2
// unit to account for a round or square cap.
//
// If there is no distance between the end point and the control point, use the
// next control point to create a tangent. If the curve is degenerate, move the
// cap out 1/2 unit horizontally.
//
// Port: nextVerb and hasNextVerb are the Rust Option<PathVerb>. hasNextVerb is
// false for None.
func extendPts(lineCap LineCap, prevVerb, nextVerb path.PathVerb, hasNextVerb bool, points []Point) {
	// The area of a circle is PI*R*R. For a unit circle, R=1/2, and the cap covers half of that.
	capOutset := floatPi / 8.0
	if lineCap == path.LineCapSquare {
		capOutset = 0.5
	}
	if prevVerb == path.PathVerbMove {
		first := points[0]
		offset := 0
		controls := len(points) - 1
		var tangent Point
		for {
			offset++
			tangent = first.Sub(points[offset])

			if !tangent.IsZero() {
				break
			}

			controls--
			if controls == 0 {
				break
			}
		}

		if tangent.IsZero() {
			tangent = path.PointFromXY(1.0, 0.0)
			controls = len(points) - 1 // If all points are equal, move all but one.
		} else {
			tangent.Normalize()
		}

		offset = 0
		for {
			// If the end point and control points are equal, loop to move them in tandem.
			points[offset].X += float32(tangent.X * capOutset)
			points[offset].Y += float32(tangent.Y * capOutset)

			offset++
			controls++
			if controls >= len(points) {
				break
			}
		}
	}

	if !hasNextVerb || nextVerb == path.PathVerbMove || nextVerb == path.PathVerbClose {
		last := points[len(points)-1]
		offset := len(points) - 1
		controls := len(points) - 1
		var tangent Point
		for {
			offset--
			tangent = last.Sub(points[offset])

			if !tangent.IsZero() {
				break
			}

			controls--
			if controls == 0 {
				break
			}
		}

		if tangent.IsZero() {
			tangent = path.PointFromXY(-1.0, 0.0)
			controls = len(points) - 1
		} else {
			tangent.Normalize()
		}

		offset = len(points) - 1
		for {
			points[offset].X += float32(tangent.X * capOutset)
			points[offset].Y += float32(tangent.Y * capOutset)

			offset--
			controls++
			if controls >= len(points) {
				break
			}
		}
	}
}

// hairQuad is hair_quad in tiny-skia src/scan/hairline.rs.
func hairQuad(points *[3]Point, clip *screenIntRect, insetClip, outsetClip *IntRect, level uint8, lp lineProc, b blitter) {
	if insetClip != nil {
		inset := insetClip.ToRect()
		if outsetClip == nil {
			return
		}
		outset := outsetClip.ToRect()

		bounds, ok := computeNocheckQuadBounds(points)
		if !ok {
			return
		}
		if !geometricOverlap(&outset, &bounds) {
			return // nothing to do
		} else if geometricContains(&inset, &bounds) {
			clip = nil
		}
	}

	hairQuad2(points, clip, level, lp, b)
}

// computeNocheckQuadBounds is compute_nocheck_quad_bounds in tiny-skia
// src/scan/hairline.rs.
func computeNocheckQuadBounds(points *[3]Point) (Rect, bool) {
	min := points[0].ToF32x2()
	max := min
	for i := 1; i < 3; i++ {
		pair := points[i].ToF32x2()
		min = min.Min(pair)
		max = max.Max(pair)
	}

	return path.RectFromLTRB(min.X(), min.Y(), max.X(), max.Y())
}

// geometricOverlap is geometric_overlap in tiny-skia src/scan/hairline.rs.
func geometricOverlap(a, b *Rect) bool {
	return a.Left() < b.Right() && b.Left() < a.Right() && a.Top() < b.Bottom() && b.Top() < a.Bottom()
}

// geometricContains is geometric_contains in tiny-skia src/scan/hairline.rs.
func geometricContains(outer, inner *Rect) bool {
	return inner.Right() <= outer.Right() &&
		inner.Left() >= outer.Left() &&
		inner.Bottom() <= outer.Bottom() &&
		inner.Top() >= outer.Top()
}

// hairQuad2 is hair_quad2 in tiny-skia src/scan/hairline.rs.
func hairQuad2(points *[3]Point, clip *screenIntRect, level uint8, lp lineProc, b blitter) {
	coeff := path.QuadCoeffFromPoints(points)

	const maxPointsQuad = (1 << maxQuadSubdivideLevel) + 1
	lines := 1 << level

	var tmp [maxPointsQuad]Point
	tmp[0] = points[0]

	var t path.F32x2
	dt := path.F32x2Splat(1.0 / float32(lines))
	for i := 1; i < lines; i++ {
		t = t.Add(dt)
		v := coeff.A.Mul(t).Add(coeff.B).Mul(t).Add(coeff.C)
		tmp[i] = path.PointFromXY(v.X(), v.Y())
	}

	tmp[lines] = points[2]
	lp(tmp[0:lines+1], clip, b)
}

// computeQuadLevel is compute_quad_level in tiny-skia src/scan/hairline.rs.
func computeQuadLevel(points *[3]Point) uint8 {
	d := computeIntQuadDist(points)
	// Quadratics approach the line connecting their start and end points
	// 4x closer with each subdivision, so we compute the number of
	// subdivisions to be the minimum need to get that distance to be less
	// than a pixel.
	level := (33 - uint32(bits.LeadingZeros32(d))) >> 1
	// sanity check on level (from the previous version)
	if level > uint32(maxQuadSubdivideLevel) { //nolint:modernize // the shape of upstream is kept
		level = uint32(maxQuadSubdivideLevel)
	}

	return uint8(level)
}

// computeIntQuadDist is compute_int_quad_dist in tiny-skia
// src/scan/hairline.rs.
func computeIntQuadDist(points *[3]Point) uint32 {
	// compute the vector between the control point ([1]) and the middle of the
	// line connecting the start and end ([0] and [2])
	dx := absF32(path.Half(points[0].X+points[2].X) - points[1].X)
	dy := absF32(path.Half(points[0].Y+points[2].Y) - points[1].Y)

	// convert to whole pixel values (use ceiling to be conservative).
	// assign to unsigned so we can safely add 1/2 of the smaller and still fit in
	// u32, since T::saturate_from() returns 31 bits at most.
	idx := uint32(path.SaturateFrom(ceilF32(dx)))
	idy := uint32(path.SaturateFrom(ceilF32(dy)))

	// use the cheap approx for distance
	if idx > idy {
		return idx + (idy >> 1)
	}
	return idy + (idx >> 1)
}

// hairCubic is hair_cubic in tiny-skia src/scan/hairline.rs.
func hairCubic(points *[4]Point, clip *screenIntRect, insetClip, outsetClip *IntRect, lp lineProc, b blitter) {
	if insetClip != nil {
		inset := insetClip.ToRect()
		if outsetClip == nil {
			return
		}
		outset := outsetClip.ToRect()

		bounds, ok := computeNocheckCubicBounds(points)
		if !ok {
			return
		}
		if !geometricOverlap(&outset, &bounds) {
			return // noting to do
		} else if geometricContains(&inset, &bounds) {
			clip = nil
		}
	}

	if quickCubicNicenessCheck(points) {
		hairCubic2(points, clip, lp, b)
	} else {
		var tmp [13]Point
		tValues := path.NewTValues()

		count := chopCubicAtMaxCurvature(points, &tValues, tmp[:])
		for i := range count {
			offset := i * 3
			newPoints := [4]Point(tmp[offset : offset+4])
			hairCubic2(&newPoints, clip, lp, b)
		}
	}
}

// computeNocheckCubicBounds is compute_nocheck_cubic_bounds in tiny-skia
// src/scan/hairline.rs.
func computeNocheckCubicBounds(points *[4]Point) (Rect, bool) {
	min := points[0].ToF32x2()
	max := min
	for i := 1; i < 4; i++ {
		pair := points[i].ToF32x2()
		min = min.Min(pair)
		max = max.Max(pair)
	}

	return path.RectFromLTRB(min.X(), min.Y(), max.X(), max.Y())
}

// quickCubicNicenessCheck is quick_cubic_niceness_check in tiny-skia
// src/scan/hairline.rs.
//
// The off-curve points are "inside" the limits of the on-curve points.
func quickCubicNicenessCheck(points *[4]Point) bool {
	return lt90(points[1], points[0], points[3]) &&
		lt90(points[2], points[0], points[3]) &&
		lt90(points[1], points[3], points[0]) &&
		lt90(points[2], points[3], points[0])
}

// lt90 is lt_90 in tiny-skia src/scan/hairline.rs.
func lt90(p0, pivot, p2 Point) bool {
	return p0.Sub(pivot).Dot(p2.Sub(pivot)) >= 0.0
}

// hairCubic2 is hair_cubic2 in tiny-skia src/scan/hairline.rs.
func hairCubic2(points *[4]Point, clip *screenIntRect, lp lineProc, b blitter) {
	lines := computeCubicSegments(points)
	if lines == 1 {
		lp([]Point{points[0], points[3]}, clip, b)
		return
	}

	coeff := path.CubicCoeffFromPoints(points)

	const maxPointsCubic = (1 << maxCubicSubdivideLevel) + 1
	var tmp [maxPointsCubic]Point

	dt := path.F32x2Splat(1.0 / float32(lines))
	var t path.F32x2

	tmp[0] = points[0]
	for i := 1; i < lines; i++ {
		t = t.Add(dt)
		tmp[i] = path.PointFromF32x2(coeff.A.Mul(t).Add(coeff.B).Mul(t).Add(coeff.C).Mul(t).Add(coeff.D))
	}

	allFinite := true
	for _, p := range tmp {
		if !p.IsFinite() {
			allFinite = false
			break
		}
	}
	if allFinite {
		tmp[lines] = points[3]
		lp(tmp[0:lines+1], clip, b)
	}
	// else some point(s) are non-finite, so do not draw
}

// computeCubicSegments is compute_cubic_segments in tiny-skia
// src/scan/hairline.rs.
func computeCubicSegments(points *[4]Point) int {
	p0 := points[0].ToF32x2()
	p1 := points[1].ToF32x2()
	p2 := points[2].ToF32x2()
	p3 := points[3].ToF32x2()

	oneThird := path.F32x2Splat(1.0 / 3.0)
	twoThird := path.F32x2Splat(2.0 / 3.0)

	p13 := oneThird.Mul(p3).Add(twoThird.Mul(p0))
	p23 := oneThird.Mul(p0).Add(twoThird.Mul(p3))

	diff := p1.Sub(p13).Abs().Max(p2.Sub(p23).Abs()).MaxComponent()
	tol := float32(1.0 / 8.0)

	for i := range maxCubicSubdivideLevel {
		if diff < tol {
			return 1 << i
		}

		tol *= 4.0
	}

	return 1 << maxCubicSubdivideLevel
}
