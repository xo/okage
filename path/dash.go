// Copyright 2014 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

// This module is a mix of SkDashPath, SkDashPathEffect, SkContourMeasure and SkPathMeasure.

// StrokeDash is StrokeDash in tiny-skia-path path/src/dash.rs. It holds the
// properties of a stroke dash.
//
// It holds an array of pairs: the first number of a pair is an "on" interval,
// and the second one is an "off" interval. It also holds a dash offset and
// internal properties.
//
// The type guarantees these facts:
//   - The dash array always has an even number of values.
//   - All values of the dash array are finite and >= 0.
//   - The dash array has at least two values.
//   - The sum of all values of the dash array is positive and finite.
//   - The dash offset is finite.
type StrokeDash struct {
	array       []float32
	offset      float32
	intervalLen NonZeroPositiveF32
	firstLen    float32 // TODO: PositiveF32
	firstIndex  int
}

// NewStrokeDash is StrokeDash::new in tiny-skia-path path/src/dash.rs. It
// creates a new stroke dash.
//
// Port: Rust takes ownership of the dash array. Go keeps dashArray, so do not
// change it after the call.
func NewStrokeDash(dashArray []float32, dashOffset float32) (*StrokeDash, bool) {
	offset, ok := NewFiniteF32(dashOffset)
	if !ok {
		return nil, false
	}

	if len(dashArray) < 2 || len(dashArray)%2 != 0 {
		return nil, false
	}

	for _, n := range dashArray {
		if n < 0.0 {
			return nil, false
		}
	}

	var sum float32
	for _, n := range dashArray {
		sum += n
	}
	intervalLen, ok := NewNonZeroPositiveF32(sum)
	if !ok {
		return nil, false
	}

	adjusted := adjustDashOffset(offset.Get(), intervalLen.Get())

	firstLen, firstIndex := findFirstInterval(dashArray, adjusted)

	return &StrokeDash{
		array:       dashArray,
		offset:      adjusted,
		intervalLen: intervalLen,
		firstLen:    firstLen,
		firstIndex:  firstIndex,
	}, true
}

// adjustDashOffset is adjust_dash_offset in tiny-skia-path path/src/dash.rs.
//
// Adjust phase to be between 0 and len, "flipping" phase if negative.
// e.g., if len is 100, then phase of -20 (or -120) is equivalent to 80.
func adjustDashOffset(offset, length float32) float32 {
	if offset < 0.0 {
		offset = -offset
		if offset > length {
			offset = remF32(offset, length)
		}

		offset = length - offset

		// Due to finite precision, it's possible that phase == len,
		// even after the subtract (if len >>> phase), so fix that here.
		if offset == length {
			offset = 0.0
		}

		return offset
	} else if offset >= length {
		return remF32(offset, length)
	}
	return offset
}

// findFirstInterval is find_first_interval in tiny-skia-path
// path/src/dash.rs.
func findFirstInterval(dashArray []float32, dashOffset float32) (float32, int) {
	for i, gap := range dashArray {
		if dashOffset > gap || (dashOffset == gap && gap != 0.0) {
			dashOffset -= gap
		} else {
			return gap - dashOffset, i
		}
	}

	// If we get here, phase "appears" to be larger than our length. This
	// shouldn't happen with perfect precision, but we can accumulate errors
	// during the initial length computation (rounding can make our sum be too
	// big or too small. In that event, we just have to eat the error here.
	return dashArray[0], 0
}

// Dash is Path::dash in tiny-skia-path path/src/dash.rs. It converts the path
// into a dashed one.
//
// ComputeResolutionScale gives resolutionScale.
//
// It returns false when it has to make more than 1_000_000 dashes, or when the
// final path has an invalid bounding box.
func (p *Path) Dash(dash *StrokeDash, resolutionScale float32) (*Path, bool) {
	return dashImpl(p, dash, resolutionScale)
}

// dashImpl is dash_impl in tiny-skia-path path/src/dash.rs.
func dashImpl(src *Path, dash *StrokeDash, resScale float32) (*Path, bool) {
	// We do not support the `cull_path` branch here.
	// Skia has a lot of code for cases when a path contains only a single zero-length line
	// or when a path is a rect. Not sure why.
	// We simply ignoring it for the sake of simplicity.

	// We also doesn't support the `SpecialLineRec` case.
	// I have no idea what the point in it.

	isEven := func(x int) bool {
		return x%2 == 0
	}

	pb := NewPathBuilder()
	var dashCount float32
	it := newContourMeasureIter(src, resScale)
	for {
		contour, ok := it.next()
		if !ok {
			break
		}
		skipFirstSegment := contour.isClosed
		addedSegment := false
		length := contour.length
		index := dash.firstIndex

		// Since the path length / dash length ratio may be arbitrarily large, we can exert
		// significant memory pressure while attempting to build the filtered path. To avoid this,
		// we simply give up dashing beyond a certain threshold.
		//
		// The original bug report (http://crbug.com/165432) is based on a path yielding more than
		// 90 million dash segments and crashing the memory allocator. A limit of 1 million
		// segments seems reasonable: at 2 verbs per segment * 9 bytes per verb, this caps the
		// maximum dash memory overhead at roughly 17MB per path.
		const maxDashCount = 1000000
		dashCount += float32(length*float32(len(dash.array)>>1)) / dash.intervalLen.Get()
		if dashCount > float32(maxDashCount) {
			return nil, false
		}

		// Using double precision to avoid looping indefinitely due to single precision rounding
		// (for extreme path_length/dash_length ratios). See test_infinite_dash() unittest.
		//
		// Port: despite the comment, upstream computes in f32.
		var distance float32
		dLen := dash.firstLen

		for distance < length {
			addedSegment = false
			if isEven(index) && !skipFirstSegment {
				addedSegment = true
				contour.pushSegment(distance, distance+dLen, true, pb)
			}

			distance += dLen

			// clear this so we only respect it the first time around
			skipFirstSegment = false

			// wrap around our intervals array if necessary
			index++
			if index == len(dash.array) {
				index = 0
			}

			// fetch our next d_len
			dLen = dash.array[index]
		}

		// extend if we ended on a segment and we need to join up with the (skipped) initial segment
		if contour.isClosed && isEven(dash.firstIndex) && dash.firstLen >= 0.0 {
			contour.pushSegment(0.0, dash.firstLen, !addedSegment, pb)
		}
	}

	return pb.Finish()
}

// maxTValue is MAX_T_VALUE in tiny-skia-path path/src/dash.rs.
const maxTValue uint32 = 0x3FFFFFFF

// contourMeasureIter is ContourMeasureIter in tiny-skia-path
// path/src/dash.rs.
type contourMeasureIter struct {
	iter      PathSegmentsIter
	tolerance float32
}

// newContourMeasureIter is ContourMeasureIter::new in tiny-skia-path
// path/src/dash.rs.
func newContourMeasureIter(p *Path, resScale float32) contourMeasureIter {
	// can't use tangents, since we need [0..1..................2] to be seen
	// as definitely not a line (it is when drawn, but not parametrically)
	// so we compare midpoints
	const cheapDistLimit float32 = 0.5 // just made this value up

	return contourMeasureIter{
		iter:      p.Segments(),
		tolerance: float32(cheapDistLimit * Invert(resScale)),
	}
}

// next is Iterator::next of ContourMeasureIter in tiny-skia-path
// path/src/dash.rs.
//
// If it encounters a zero-length contour, it is skipped.
func (it *contourMeasureIter) next() (*contourMeasure, bool) {
	// Note:
	// as we accumulate distance, we have to check that the result of +=
	// actually made it larger, since a very small delta might be > 0, but
	// still have no effect on distance (if distance >>> delta).
	//
	// We do this check below, and in compute_quad_segs and compute_cubic_segs

	contour := &contourMeasure{}

	pointIndex := 0
	var distance float32
	haveSeenClose := false
	prevP := PointZero()
	for {
		seg, ok := it.iter.Next()
		if !ok {
			break
		}
		switch seg.Kind {
		case PathSegmentMoveTo:
			p0 := seg.Points[0]
			contour.points = append(contour.points, p0)
			prevP = p0
		case PathSegmentLineTo:
			p0 := seg.Points[0]
			prevD := distance
			distance = contour.computeLineSeg(prevP, p0, distance, pointIndex)

			if distance > prevD {
				contour.points = append(contour.points, p0)
				pointIndex++
			}

			prevP = p0
		case PathSegmentQuadTo:
			p0, p1 := seg.Points[0], seg.Points[1]
			prevD := distance
			distance = contour.computeQuadSegs(
				prevP,
				p0,
				p1,
				distance,
				0,
				maxTValue,
				pointIndex,
				it.tolerance,
			)

			if distance > prevD {
				contour.points = append(contour.points, p0)
				contour.points = append(contour.points, p1)
				pointIndex += 2
			}

			prevP = p1
		case PathSegmentCubicTo:
			p0, p1, p2 := seg.Points[0], seg.Points[1], seg.Points[2]
			prevD := distance
			distance = contour.computeCubicSegs(
				prevP,
				p0,
				p1,
				p2,
				distance,
				0,
				maxTValue,
				pointIndex,
				it.tolerance,
			)

			if distance > prevD {
				contour.points = append(contour.points, p0)
				contour.points = append(contour.points, p1)
				contour.points = append(contour.points, p2)
				pointIndex += 3
			}

			prevP = p2
		case PathSegmentClose:
			haveSeenClose = true
		}

		// TODO: to contour iter?
		if v, ok := it.iter.NextVerb(); ok && v == PathVerbMove {
			break
		}
	}

	if !isFinite(distance) {
		return nil, false
	}

	if haveSeenClose {
		prevD := distance
		firstPt := contour.points[0]
		distance = contour.computeLineSeg(
			contour.points[pointIndex],
			firstPt,
			distance,
			pointIndex,
		)

		if distance > prevD {
			contour.points = append(contour.points, firstPt)
		}
	}

	contour.length = distance
	contour.isClosed = haveSeenClose

	if len(contour.points) == 0 {
		return nil, false
	}
	return contour, true
}

// segmentType is SegmentType in tiny-skia-path path/src/dash.rs.
type segmentType uint8

const (
	segmentTypeLine segmentType = iota
	segmentTypeQuad
	segmentTypeCubic
)

// segment is Segment in tiny-skia-path path/src/dash.rs.
type segment struct {
	distance   float32 // total distance up to this point
	pointIndex int     // index into the ContourMeasure::points array
	tValue     uint32
	kind       segmentType
}

// scalarT is Segment::scalar_t in tiny-skia-path path/src/dash.rs.
func (s segment) scalarT() float32 {
	// 1/kMaxTValue can't be represented as a float, but it's close and the limits work fine.
	const maxTReciprocal float32 = 1.0 / float32(maxTValue)
	return float32(float32(s.tValue) * maxTReciprocal)
}

// contourMeasure is ContourMeasure in tiny-skia-path path/src/dash.rs.
type contourMeasure struct {
	segments []segment
	points   []Point
	length   float32
	isClosed bool
}

// pushSegment is ContourMeasure::push_segment in tiny-skia-path
// path/src/dash.rs.
func (c *contourMeasure) pushSegment(startD, stopD float32, startWithMoveTo bool, pb *PathBuilder) {
	if startD < 0.0 {
		startD = 0.0
	}

	if stopD > c.length {
		stopD = c.length
	}

	if !(startD <= stopD) {
		// catch NaN values as well
		return
	}

	if len(c.segments) == 0 {
		return
	}

	segIndex, startT, ok := c.distanceToSegment(startD)
	if !ok {
		return
	}
	seg := c.segments[segIndex]

	stopSegIndex, stopT, ok := c.distanceToSegment(stopD)
	if !ok {
		return
	}
	stopSeg := c.segments[stopSegIndex]

	p := PointZero()
	if startWithMoveTo {
		computePosTan(
			c.points[seg.pointIndex:],
			seg.kind,
			startT,
			&p,
			nil,
		)
		pb.MoveTo(p.X, p.Y)
	}

	if seg.pointIndex == stopSeg.pointIndex {
		segmentTo(
			c.points[seg.pointIndex:],
			seg.kind,
			startT,
			stopT,
			pb,
		)
	} else {
		newSegIndex := segIndex
		for {
			segmentTo(
				c.points[seg.pointIndex:],
				seg.kind,
				startT,
				NormalizedF32One,
				pb,
			)

			oldPointIndex := seg.pointIndex
			for {
				newSegIndex++
				if c.segments[newSegIndex].pointIndex != oldPointIndex {
					break
				}
			}
			seg = c.segments[newSegIndex]

			startT = NormalizedF32Zero

			if seg.pointIndex >= stopSeg.pointIndex {
				break
			}
		}

		segmentTo(
			c.points[seg.pointIndex:],
			seg.kind,
			NormalizedF32Zero,
			stopT,
			pb,
		)
	}
}

// distanceToSegment is ContourMeasure::distance_to_segment in tiny-skia-path
// path/src/dash.rs.
func (c *contourMeasure) distanceToSegment(distance float32) (int, NormalizedF32, bool) {
	idx := findSegment(c.segments, distance)
	// don't care if we hit an exact match or not, so we xor index if it is negative
	idx ^= idx >> 31
	index := int(idx)
	seg := c.segments[index]

	// now interpolate t-values with the prev segment (if possible)
	var startT float32
	var startD float32
	// check if the prev segment is legal, and references the same set of points
	if index > 0 {
		startD = c.segments[index-1].distance
		if c.segments[index-1].pointIndex == seg.pointIndex {
			startT = c.segments[index-1].scalarT()
		}
	}

	t := startT + float32((seg.scalarT()-startT)*(distance-startD))/(seg.distance-startD)
	nt, ok := NewNormalizedF32(t)
	if !ok {
		return 0, 0, false
	}
	return index, nt, true
}

// computeLineSeg is ContourMeasure::compute_line_seg in tiny-skia-path
// path/src/dash.rs.
func (c *contourMeasure) computeLineSeg(p0, p1 Point, distance float32, pointIndex int) float32 {
	d := p0.Distance(p1)
	prevD := distance
	distance += d
	if distance > prevD {
		c.segments = append(c.segments, segment{
			distance:   distance,
			pointIndex: pointIndex,
			tValue:     maxTValue,
			kind:       segmentTypeLine,
		})
	}

	return distance
}

// computeQuadSegs is ContourMeasure::compute_quad_segs in tiny-skia-path
// path/src/dash.rs.
func (c *contourMeasure) computeQuadSegs(
	p0, p1, p2 Point,
	distance float32,
	minT, maxT uint32,
	pointIndex int,
	tolerance float32,
) float32 {
	if tSpanBigEnough(maxT-minT) != 0 && quadTooCurvy(p0, p1, p2, tolerance) {
		var tmp [5]Point
		halfT := (minT + maxT) >> 1

		ChopQuadAt([]Point{p0, p1, p2}, NormalizedF32ExclusiveHalf, &tmp)
		distance = c.computeQuadSegs(
			tmp[0],
			tmp[1],
			tmp[2],
			distance,
			minT,
			halfT,
			pointIndex,
			tolerance,
		)
		distance = c.computeQuadSegs(
			tmp[2],
			tmp[3],
			tmp[4],
			distance,
			halfT,
			maxT,
			pointIndex,
			tolerance,
		)
	} else {
		d := p0.Distance(p2)
		prevD := distance
		distance += d
		if distance > prevD {
			c.segments = append(c.segments, segment{
				distance:   distance,
				pointIndex: pointIndex,
				tValue:     maxT,
				kind:       segmentTypeQuad,
			})
		}
	}

	return distance
}

// computeCubicSegs is ContourMeasure::compute_cubic_segs in tiny-skia-path
// path/src/dash.rs.
func (c *contourMeasure) computeCubicSegs(
	p0, p1, p2, p3 Point,
	distance float32,
	minT, maxT uint32,
	pointIndex int,
	tolerance float32,
) float32 {
	if tSpanBigEnough(maxT-minT) != 0 && cubicTooCurvy(p0, p1, p2, p3, tolerance) {
		var tmp [7]Point
		halfT := (minT + maxT) >> 1

		ChopCubicAt2(
			&[4]Point{p0, p1, p2, p3},
			NormalizedF32ExclusiveHalf,
			tmp[:],
		)
		distance = c.computeCubicSegs(
			tmp[0],
			tmp[1],
			tmp[2],
			tmp[3],
			distance,
			minT,
			halfT,
			pointIndex,
			tolerance,
		)
		distance = c.computeCubicSegs(
			tmp[3],
			tmp[4],
			tmp[5],
			tmp[6],
			distance,
			halfT,
			maxT,
			pointIndex,
			tolerance,
		)
	} else {
		d := p0.Distance(p3)
		prevD := distance
		distance += d
		if distance > prevD {
			c.segments = append(c.segments, segment{
				distance:   distance,
				pointIndex: pointIndex,
				tValue:     maxT,
				kind:       segmentTypeCubic,
			})
		}
	}

	return distance
}

// findSegment is find_segment in tiny-skia-path path/src/dash.rs.
func findSegment(base []segment, key float32) int32 {
	var lo uint32
	hi := uint32(len(base) - 1)

	for lo < hi {
		mid := (hi + lo) >> 1
		if base[mid].distance < key {
			lo = mid + 1
		} else {
			hi = mid
		}
	}

	if base[hi].distance < key {
		hi++
		hi = ^hi
	} else if key < base[hi].distance {
		hi = ^hi
	}

	return int32(hi)
}

// computePosTan is compute_pos_tan in tiny-skia-path path/src/dash.rs.
//
// Port: a nil pos or tangent is the Rust None.
func computePosTan(
	points []Point,
	segKind segmentType,
	t NormalizedF32,
	pos *Point,
	tangent *Point,
) {
	switch segKind {
	case segmentTypeLine:
		if pos != nil {
			*pos = PointFromXY(
				interpDash(points[0].X, points[1].X, t),
				interpDash(points[0].Y, points[1].Y, t),
			)
		}

		if tangent != nil {
			tangent.SetNormalize(points[1].X-points[0].X, points[1].Y-points[0].Y)
		}
	case segmentTypeQuad:
		src := (*[3]Point)(points[0:3])
		if pos != nil {
			*pos = evalQuadAt(src, t)
		}

		if tangent != nil {
			*tangent = evalQuadTangentAt(src, t)
			tangent.Normalize()
		}
	case segmentTypeCubic:
		src := (*[4]Point)(points[0:4])
		if pos != nil {
			*pos = evalCubicPosAt(src, t)
		}

		if tangent != nil {
			*tangent = evalCubicTangentAt(src, t)
			tangent.Normalize()
		}
	}
}

// segmentTo is segment_to in tiny-skia-path path/src/dash.rs.
func segmentTo(
	points []Point,
	segKind segmentType,
	startT NormalizedF32,
	stopT NormalizedF32,
	pb *PathBuilder,
) {
	if startT == stopT {
		if pt, ok := pb.LastPoint(); ok {
			// If the dash as a zero-length on segment, add a corresponding zero-length line.
			// The stroke code will add end caps to zero length lines as appropriate.
			pb.LineTo(pt.X, pt.Y)
		}

		return
	}

	switch segKind {
	case segmentTypeLine:
		if stopT == NormalizedF32One {
			pb.LineTo(points[1].X, points[1].Y)
		} else {
			pb.LineTo(
				interpDash(points[0].X, points[1].X, stopT),
				interpDash(points[0].Y, points[1].Y, stopT),
			)
		}
	case segmentTypeQuad:
		var tmp0, tmp1 [5]Point
		if startT == NormalizedF32Zero {
			if stopT == NormalizedF32One {
				pb.quadToPt(points[1], points[2])
			} else {
				stopT := NormalizedF32ExclusiveBounded(stopT.Get())
				ChopQuadAt(points, stopT, &tmp0)
				pb.quadToPt(tmp0[1], tmp0[2])
			}
		} else {
			startTT := NormalizedF32ExclusiveBounded(startT.Get())
			ChopQuadAt(points, startTT, &tmp0)
			if stopT == NormalizedF32One {
				pb.quadToPt(tmp0[3], tmp0[4])
			} else {
				newT := (stopT.Get() - startT.Get()) / (1.0 - startT.Get())
				newTT := NormalizedF32ExclusiveBounded(newT)
				ChopQuadAt(tmp0[2:], newTT, &tmp1)
				pb.quadToPt(tmp1[1], tmp1[2])
			}
		}
	case segmentTypeCubic:
		var tmp0, tmp1 [7]Point
		if startT == NormalizedF32Zero {
			if stopT == NormalizedF32One {
				pb.cubicToPt(points[1], points[2], points[3])
			} else {
				stopT := NormalizedF32ExclusiveBounded(stopT.Get())
				ChopCubicAt2((*[4]Point)(points[0:4]), stopT, tmp0[:])
				pb.cubicToPt(tmp0[1], tmp0[2], tmp0[3])
			}
		} else {
			startTT := NormalizedF32ExclusiveBounded(startT.Get())
			ChopCubicAt2((*[4]Point)(points[0:4]), startTT, tmp0[:])
			if stopT == NormalizedF32One {
				pb.cubicToPt(tmp0[4], tmp0[5], tmp0[6])
			} else {
				newT := (stopT.Get() - startT.Get()) / (1.0 - startT.Get())
				newTT := NormalizedF32ExclusiveBounded(newT)
				ChopCubicAt2((*[4]Point)(tmp0[3:7]), newTT, tmp1[:])
				pb.cubicToPt(tmp1[1], tmp1[2], tmp1[3])
			}
		}
	}
}

// tSpanBigEnough is t_span_big_enough in tiny-skia-path path/src/dash.rs.
func tSpanBigEnough(tSpan uint32) uint32 {
	return tSpan >> 10
}

// quadTooCurvy is quad_too_curvy in tiny-skia-path path/src/dash.rs.
func quadTooCurvy(p0, p1, p2 Point, tolerance float32) bool {
	// diff = (a/4 + b/2 + c/4) - (a/2 + c/2)
	// diff = -a/4 + b/2 - c/4
	dx := Half(p1.X) - Half(Half(p0.X+p2.X))
	dy := Half(p1.Y) - Half(Half(p0.Y+p2.Y))

	dist := maxF32(absF32(dx), absF32(dy))
	return dist > tolerance
}

// cubicTooCurvy is cubic_too_curvy in tiny-skia-path path/src/dash.rs.
func cubicTooCurvy(p0, p1, p2, p3 Point, tolerance float32) bool {
	n0 := cheapDistExceedsLimit(
		p1,
		interpSafe(p0.X, p3.X, 1.0/3.0),
		interpSafe(p0.Y, p3.Y, 1.0/3.0),
		tolerance,
	)

	n1 := cheapDistExceedsLimit(
		p2,
		interpSafe(p0.X, p3.X, 2.0/3.0),
		interpSafe(p0.Y, p3.Y, 2.0/3.0),
		tolerance,
	)

	return n0 || n1
}

// cheapDistExceedsLimit is cheap_dist_exceeds_limit in tiny-skia-path
// path/src/dash.rs.
func cheapDistExceedsLimit(pt Point, x, y, tolerance float32) bool {
	dist := maxF32(absF32(x-pt.X), absF32(y-pt.Y))
	// just made up the 1/2
	return dist > tolerance
}

// interpDash is interp in tiny-skia-path path/src/dash.rs. The name takes the
// suffix of its module, because path_geometry.rs also has an interp (D10).
//
// Linearly interpolate between A and B, based on t.
//
// If t is 0, return A. If t is 1, return B else interpolate.
func interpDash(a, b float32, t NormalizedF32) float32 {
	return a + float32((b-a)*t.Get())
}

// interpSafe is interp_safe in tiny-skia-path path/src/dash.rs.
func interpSafe(a, b, t float32) float32 {
	return a + float32((b-a)*t)
}
