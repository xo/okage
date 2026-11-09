// Copyright 2008 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

// Based on SkStroke.cpp

// swappableBuilders is SwappableBuilders in tiny-skia-path
// path/src/stroker.rs.
type swappableBuilders struct {
	inner *PathBuilder
	outer *PathBuilder
}

// swap is SwappableBuilders::swap in tiny-skia-path path/src/stroker.rs.
func (b *swappableBuilders) swap() {
	// Skia swaps pointers to inner and outer builders during joining,
	// but not builders itself. So a simple `core::mem::swap` will produce invalid results.
	// And if we try to use `core::mem::swap` on references, like below,
	// borrow checker will be unhappy.
	// That's why we need this wrapper. Maybe there is a better solution.
	b.inner, b.outer = b.outer, b.inner
}

// Stroke is Stroke in tiny-skia-path path/src/stroker.rs. It holds the
// properties of a stroke.
type Stroke struct {
	// Width is the thickness of the stroke.
	//
	// It must be >= 0.
	//
	// When it is 0, the stroke is a hairline.
	//
	// Default: 1.0
	Width float32

	// MiterLimit is the limit at which a sharp corner is drawn beveled.
	//
	// Default: 4.0
	MiterLimit float32

	// LineCap is the line cap of the stroke.
	//
	// Default: Butt
	LineCap LineCap

	// LineJoin is the line join of the stroke.
	//
	// Default: Miter
	LineJoin LineJoin

	// Dash holds the properties of the stroke dash. Nil is the Rust None.
	//
	// Default: None
	Dash *StrokeDash
}

// DefaultStroke is the Default impl of Stroke in tiny-skia-path
// path/src/stroker.rs.
func DefaultStroke() Stroke {
	return Stroke{
		Width:      1.0,
		MiterLimit: 4.0,
		LineCap:    LineCapButt,
		LineJoin:   LineJoinMiter,
		Dash:       nil,
	}
}

// LineCap is LineCap in tiny-skia-path path/src/stroker.rs. It is what a
// stroke draws at the start and the end of an open contour of a path.
//
// The zero value is LineCapButt, the Default of upstream.
type LineCap uint8

const (
	// LineCapButt adds no extension to the stroke.
	LineCapButt LineCap = iota
	// LineCapRound adds a circle.
	LineCapRound
	// LineCapSquare adds a square.
	LineCapSquare
)

// String satisfies the fmt.Stringer interface.
func (c LineCap) String() string {
	switch c {
	case LineCapButt:
		return "Butt"
	case LineCapRound:
		return "Round"
	case LineCapSquare:
		return "Square"
	}
	return "LineCap(?)"
}

// LineJoin is LineJoin in tiny-skia-path path/src/stroker.rs. It sets how a
// stroke draws the corners of a shape.
//
// The join affects the four corners of a stroked rectangle, and the connected
// segments of a stroked path.
//
// Choose the miter join to draw sharp corners. Choose the round join to draw a
// circle with a radius equal to the stroke width on top of the corner. Choose
// the bevel join to connect the thick strokes with the smallest shape.
//
// The fill path made to describe the stroked path uses the join setting, but
// it can omit the actual join. For example, a fill path made with round joins
// does not always include circles at each connected segment.
//
// The zero value is LineJoinMiter, the Default of upstream.
type LineJoin uint8

const (
	// LineJoinMiter extends to the miter limit, then switches to bevel.
	LineJoinMiter LineJoin = iota
	// LineJoinMiterClip extends to the miter limit, then clips the corner.
	LineJoinMiterClip
	// LineJoinRound adds a circle.
	LineJoinRound
	// LineJoinBevel connects the outside edges.
	LineJoinBevel
)

// String satisfies the fmt.Stringer interface.
func (j LineJoin) String() string {
	switch j {
	case LineJoinMiter:
		return "Miter"
	case LineJoinMiterClip:
		return "MiterClip"
	case LineJoinRound:
		return "Round"
	case LineJoinBevel:
		return "Bevel"
	}
	return "LineJoin(?)"
}

// quadRecursiveLimit is QUAD_RECURSIVE_LIMIT in tiny-skia-path
// path/src/stroker.rs.
const quadRecursiveLimit = 3

// recursiveLimits is RECURSIVE_LIMITS in tiny-skia-path path/src/stroker.rs.
//
// quads with extreme widths (e.g. (0,1) (1,6) (0,3) width=5e7) recurse to point of failure
// largest seen for normal cubics: 5, 26
// largest seen for normal quads: 11.
var recursiveLimits = [4]int32{5 * 3, 26 * 3, 11 * 3, 11 * 3} // 3x limits seen in practice

// capProc is CapProc in tiny-skia-path path/src/stroker.rs.
//
// Port: upstream holds a function pointer and compares two of them with
// fn_ptr_eq. Go cannot compare two func values, so a capProc names the
// function, and its method call calls it.
type capProc uint8

const (
	capProcButt capProc = iota
	capProcRound
	capProcSquare
)

// call calls the function that c names.
func (c capProc) call(pivot, normal, stop Point, otherPath *PathBuilder, path *PathBuilder) {
	switch c {
	case capProcButt:
		buttCapper(pivot, normal, stop, otherPath, path)
	case capProcRound:
		roundCapper(pivot, normal, stop, otherPath, path)
	case capProcSquare:
		squareCapper(pivot, normal, stop, otherPath, path)
	}
}

// joinProc is JoinProc in tiny-skia-path path/src/stroker.rs.
//
// Port: a joinProc names the function, as a capProc does.
type joinProc uint8

const (
	joinProcMiter joinProc = iota
	joinProcMiterClip
	joinProcRound
	joinProcBevel
)

// call calls the function that j names.
func (j joinProc) call(
	beforeUnitNormal Point,
	pivot Point,
	afterUnitNormal Point,
	radius float32,
	invMiterLimit float32,
	prevIsLine bool,
	currIsLine bool,
	builders swappableBuilders,
) {
	switch j {
	case joinProcMiter:
		miterJoiner(beforeUnitNormal, pivot, afterUnitNormal, radius, invMiterLimit, prevIsLine, currIsLine, builders)
	case joinProcMiterClip:
		miterClipJoiner(beforeUnitNormal, pivot, afterUnitNormal, radius, invMiterLimit, prevIsLine, currIsLine, builders)
	case joinProcRound:
		roundJoiner(beforeUnitNormal, pivot, afterUnitNormal, radius, invMiterLimit, prevIsLine, currIsLine, builders)
	case joinProcBevel:
		bevelJoiner(beforeUnitNormal, pivot, afterUnitNormal, radius, invMiterLimit, prevIsLine, currIsLine, builders)
	}
}

// reductionType is ReductionType in tiny-skia-path path/src/stroker.rs.
type reductionType uint8

const (
	reductionTypePoint       reductionType = iota // all curve points are practically identical
	reductionTypeLine                             // the control point is on the line between the ends
	reductionTypeQuad                             // the control point is outside the line between the ends
	reductionTypeDegenerate                       // the control point is on the line but outside the ends
	reductionTypeDegenerate2                      // two control points are on the line but outside ends (cubic)
	reductionTypeDegenerate3                      // three areas of max curvature found (for cubic)
)

// strokeType is StrokeType in tiny-skia-path path/src/stroker.rs.
type strokeType int32

const (
	strokeTypeOuter strokeType = 1 // use sign-opposite values later to flip perpendicular axis
	strokeTypeInner strokeType = -1
)

// resultType is ResultType in tiny-skia-path path/src/stroker.rs.
type resultType uint8

const (
	resultTypeSplit      resultType = iota // the caller should split the quad stroke in two
	resultTypeDegenerate                   // the caller should add a line
	resultTypeQuad                         // the caller should (continue to try to) add a quad stroke
)

// intersectRayType is IntersectRayType in tiny-skia-path path/src/stroker.rs.
type intersectRayType uint8

const (
	intersectRayTypeCtrlPt intersectRayType = iota
	intersectRayTypeResultType
)

// Stroke is Path::stroke in tiny-skia-path path/src/stroker.rs. It returns a
// stroked path.
//
// ComputeResolutionScale gives resolutionScale.
//
// If you plan to stroke many paths, you can use a PathStroker. It keeps the
// temporary allocations that a stroke needs, and this can make it a bit
// faster.
func (p *Path) Stroke(stroke *Stroke, resolutionScale float32) (*Path, bool) {
	return NewPathStroker().Stroke(p, stroke, resolutionScale)
}

// PathStroker is PathStroker in tiny-skia-path path/src/stroker.rs. It is a
// path stroker.
type PathStroker struct {
	radius             float32
	invMiterLimit      float32
	resScale           float32
	invResScale        float32
	invResScaleSquared float32

	firstNormal     Point
	prevNormal      Point
	firstUnitNormal Point
	prevUnitNormal  Point

	// on original path
	firstPt Point
	prevPt  Point

	firstOuterPt               Point
	firstOuterPtIndexInContour int
	segmentCount               int32
	prevIsLine                 bool

	capper capProc
	joiner joinProc

	// outer is our working answer, inner is temp
	inner  PathBuilder
	outer  PathBuilder
	cusper PathBuilder

	strokeType strokeType

	recursionDepth int32 // track stack depth to abort if numerics run amok
	foundTangents  bool  // do less work until tangents meet (cubic)
	joinCompleted  bool  // previous join was not degenerate
}

// NewPathStroker is PathStroker::new in tiny-skia-path path/src/stroker.rs. It
// creates a new PathStroker. The Default impl of upstream is the same.
func NewPathStroker() *PathStroker {
	return &PathStroker{
		radius:             0.0,
		invMiterLimit:      0.0,
		resScale:           1.0,
		invResScale:        1.0,
		invResScaleSquared: 1.0,

		firstNormal:     PointZero(),
		prevNormal:      PointZero(),
		firstUnitNormal: PointZero(),
		prevUnitNormal:  PointZero(),

		firstPt: PointZero(),
		prevPt:  PointZero(),

		firstOuterPt:               PointZero(),
		firstOuterPtIndexInContour: 0,
		segmentCount:               -1,
		prevIsLine:                 false,

		capper: capProcButt,
		joiner: joinProcMiter,

		inner:  *NewPathBuilder(),
		outer:  *NewPathBuilder(),
		cusper: *NewPathBuilder(),

		strokeType: strokeTypeOuter,

		recursionDepth: 0,
		foundTangents:  false,
		joinCompleted:  false,
	}
}

// ComputeResolutionScale is PathStroker::compute_resolution_scale in
// tiny-skia-path path/src/stroker.rs. It computes a resolution scale.
//
// The resolution scale is the "intended" resolution for the output. The
// default is 1.0.
//
// A larger value (res > 1) tells that the result must be more precise,
// because it will be zoomed up, and small errors will be magnified.
//
// A smaller value (0 < res < 1) tells that the result can be less precise,
// because it will be zoomed down, and small errors can be invisible.
func ComputeResolutionScale(ts Transform) float32 {
	sx := PointFromXY(ts.SX, ts.KX).Length()
	sy := PointFromXY(ts.KY, ts.SY).Length()
	if isFinite(sx) && isFinite(sy) {
		scale := maxF32(sx, sy)
		if scale > 0.0 {
			return scale
		}
	}

	return 1.0
}

// Stroke is PathStroker::stroke in tiny-skia-path path/src/stroker.rs. It
// strokes the path.
//
// It can be called many times, to reuse the allocated buffers.
//
// ComputeResolutionScale gives resolutionScale.
func (s *PathStroker) Stroke(path *Path, stroke *Stroke, resolutionScale float32) (*Path, bool) {
	width, ok := NewNonZeroPositiveF32(stroke.Width)
	if !ok {
		return nil, false
	}
	return s.strokeInner(
		path,
		width,
		stroke.MiterLimit,
		stroke.LineCap,
		stroke.LineJoin,
		resolutionScale,
	)
}

// strokeInner is PathStroker::stroke_inner in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) strokeInner(
	path *Path,
	width NonZeroPositiveF32,
	miterLimit float32,
	lineCap LineCap,
	lineJoin LineJoin,
	resScale float32,
) (*Path, bool) {
	// TODO: stroke_rect optimization

	var invMiterLimit float32

	if lineJoin == LineJoinMiter {
		if miterLimit <= 1.0 {
			lineJoin = LineJoinBevel
		} else {
			invMiterLimit = Invert(miterLimit)
		}
	}

	if lineJoin == LineJoinMiterClip {
		invMiterLimit = Invert(miterLimit)
	}

	s.resScale = resScale
	// The '4' below matches the fill scan converter's error term.
	s.invResScale = Invert(float32(resScale * 4.0))
	s.invResScaleSquared = Sqr(s.invResScale)

	s.radius = Half(width.Get())
	s.invMiterLimit = invMiterLimit

	s.firstNormal = PointZero()
	s.prevNormal = PointZero()
	s.firstUnitNormal = PointZero()
	s.prevUnitNormal = PointZero()

	s.firstPt = PointZero()
	s.prevPt = PointZero()

	s.firstOuterPt = PointZero()
	s.firstOuterPtIndexInContour = 0
	s.segmentCount = -1
	s.prevIsLine = false

	s.capper = capFactory(lineCap)
	s.joiner = joinFactory(lineJoin)

	// Need some estimate of how large our final result (fOuter)
	// and our per-contour temp (fInner) will be, so we don't spend
	// extra time repeatedly growing these arrays.
	//
	// 1x for inner == 'wag' (worst contour length would be better guess)
	s.inner.Clear()
	s.inner.reserve(len(path.verbs), len(path.points))

	// 3x for result == inner + outer + join (swag)
	s.outer.Clear()
	s.outer.reserve(len(path.verbs)*3, len(path.points)*3)

	s.cusper.Clear()

	s.strokeType = strokeTypeOuter

	s.recursionDepth = 0
	s.foundTangents = false
	s.joinCompleted = false

	lastSegmentIsLine := false
	iter := path.Segments()
	iter.SetAutoClose(true)
	for {
		segment, ok := iter.Next()
		if !ok {
			break
		}
		switch segment.Kind {
		case PathSegmentMoveTo:
			s.moveTo(segment.Points[0])
		case PathSegmentLineTo:
			s.lineTo(segment.Points[0], &iter)
			lastSegmentIsLine = true
		case PathSegmentQuadTo:
			s.quadTo(segment.Points[0], segment.Points[1])
			lastSegmentIsLine = false
		case PathSegmentCubicTo:
			s.cubicTo(segment.Points[0], segment.Points[1], segment.Points[2])
			lastSegmentIsLine = false
		case PathSegmentClose:
			if lineCap != LineCapButt {
				// If the stroke consists of a moveTo followed by a close, treat it
				// as if it were followed by a zero-length line. Lines without length
				// can have square and round end caps.
				if s.hasOnlyMoveTo() {
					s.lineTo(s.moveToPt(), nil)
					lastSegmentIsLine = true
					continue
				}

				// If the stroke consists of a moveTo followed by one or more zero-length
				// verbs, then followed by a close, treat is as if it were followed by a
				// zero-length line. Lines without length can have square & round end caps.
				if s.isCurrentContourEmpty() {
					lastSegmentIsLine = true
					continue
				}
			}

			s.close(lastSegmentIsLine)
		}
	}

	return s.finish(lastSegmentIsLine)
}

// builders is PathStroker::builders in tiny-skia-path path/src/stroker.rs.
func (s *PathStroker) builders() swappableBuilders {
	return swappableBuilders{
		inner: &s.inner,
		outer: &s.outer,
	}
}

// moveToPt is PathStroker::move_to_pt in tiny-skia-path path/src/stroker.rs.
func (s *PathStroker) moveToPt() Point {
	return s.firstPt
}

// moveTo is PathStroker::move_to in tiny-skia-path path/src/stroker.rs.
func (s *PathStroker) moveTo(p Point) {
	if s.segmentCount > 0 {
		s.finishContour(false, false)
	}

	s.segmentCount = 0
	s.firstPt = p
	s.prevPt = p
	s.joinCompleted = false
}

// lineTo is PathStroker::line_to in tiny-skia-path path/src/stroker.rs.
//
// Port: a nil iter is the Rust None.
func (s *PathStroker) lineTo(p Point, iter *PathSegmentsIter) {
	teenyLine := s.prevPt.equalsWithinTolerance(p, float32(ScalarNearlyZero*s.invResScale))
	if fnPtrEq(s.capper, capProcButt) && teenyLine {
		return
	}

	if teenyLine && (s.joinCompleted || (iter != nil && iter.hasValidTangent())) {
		return
	}

	normal := PointZero()
	unitNormal := PointZero()
	if !s.preJoinTo(p, true, &normal, &unitNormal) {
		return
	}

	s.outer.LineTo(p.X+normal.X, p.Y+normal.Y)
	s.inner.LineTo(p.X-normal.X, p.Y-normal.Y)

	s.postJoinTo(p, normal, unitNormal)
}

// quadTo is PathStroker::quad_to in tiny-skia-path path/src/stroker.rs.
func (s *PathStroker) quadTo(p1, p2 Point) {
	quad := [3]Point{s.prevPt, p1, p2}
	reduction, reductionType := checkQuadLinear(&quad)
	if reductionType == reductionTypePoint {
		// If the stroke consists of a moveTo followed by a degenerate curve, treat it
		// as if it were followed by a zero-length line. Lines without length
		// can have square and round end caps.
		s.lineTo(p2, nil)
		return
	}

	if reductionType == reductionTypeLine {
		s.lineTo(p2, nil)
		return
	}

	if reductionType == reductionTypeDegenerate {
		s.lineTo(reduction, nil)
		saveJoiner := s.joiner
		s.joiner = joinProcRound
		s.lineTo(p2, nil)
		s.joiner = saveJoiner
		return
	}

	normalAB := PointZero()
	unitAB := PointZero()
	normalBC := PointZero()
	unitBC := PointZero()
	if !s.preJoinTo(p1, false, &normalAB, &unitAB) {
		s.lineTo(p2, nil)
		return
	}

	var quadPoints quadConstruct
	s.initQuad(
		strokeTypeOuter,
		NormalizedF32Zero,
		NormalizedF32One,
		&quadPoints,
	)
	s.quadStroke(&quad, &quadPoints)
	s.initQuad(
		strokeTypeInner,
		NormalizedF32Zero,
		NormalizedF32One,
		&quadPoints,
	)
	s.quadStroke(&quad, &quadPoints)

	ok := setNormalUnitNormal(
		quad[1],
		quad[2],
		s.resScale,
		s.radius,
		&normalBC,
		&unitBC,
	)
	if !ok {
		normalBC = normalAB
		unitBC = unitAB
	}

	s.postJoinTo(p2, normalBC, unitBC)
}

// cubicTo is PathStroker::cubic_to in tiny-skia-path path/src/stroker.rs.
func (s *PathStroker) cubicTo(pt1, pt2, pt3 Point) {
	cubic := [4]Point{s.prevPt, pt1, pt2, pt3}
	var reduction [3]Point
	tangentPt := PointZero()
	reductionType := checkCubicLinear(&cubic, &reduction, &tangentPt)
	if reductionType == reductionTypePoint {
		// If the stroke consists of a moveTo followed by a degenerate curve, treat it
		// as if it were followed by a zero-length line. Lines without length
		// can have square and round end caps.
		s.lineTo(pt3, nil)
		return
	}

	if reductionType == reductionTypeLine {
		s.lineTo(pt3, nil)
		return
	}

	if reductionTypeDegenerate <= reductionType &&
		reductionTypeDegenerate3 >= reductionType {
		s.lineTo(reduction[0], nil)
		saveJoiner := s.joiner
		s.joiner = joinProcRound
		if reductionTypeDegenerate2 <= reductionType {
			s.lineTo(reduction[1], nil)
		}

		if reductionTypeDegenerate3 == reductionType {
			s.lineTo(reduction[2], nil)
		}

		s.lineTo(pt3, nil)
		s.joiner = saveJoiner
		return
	}

	normalAB := PointZero()
	unitAB := PointZero()
	normalCD := PointZero()
	unitCD := PointZero()
	if !s.preJoinTo(tangentPt, false, &normalAB, &unitAB) {
		s.lineTo(pt3, nil)
		return
	}

	tValuesBuf := NewTValues()
	tValues := findCubicInflections(&cubic, &tValuesBuf)
	lastT := NormalizedF32Zero
	for index := 0; index <= len(tValues); index++ {
		nextT := NormalizedF32One
		if index < len(tValues) {
			nextT = tValues[index].ToNormalized()
		}

		var quadPoints quadConstruct
		s.initQuad(strokeTypeOuter, lastT, nextT, &quadPoints)
		s.cubicStroke(&cubic, &quadPoints)
		s.initQuad(strokeTypeInner, lastT, nextT, &quadPoints)
		s.cubicStroke(&cubic, &quadPoints)
		lastT = nextT
	}

	if cusp, ok := findCubicCusp(&cubic); ok {
		cuspLoc := evalCubicPosAt(&cubic, cusp.ToNormalized())
		s.cusper.PushCircle(cuspLoc.X, cuspLoc.Y, s.radius)
	}

	// emit the join even if one stroke succeeded but the last one failed
	// this avoids reversing an inner stroke with a partial path followed by another moveto
	s.setCubicEndNormal(&cubic, normalAB, unitAB, &normalCD, &unitCD)

	s.postJoinTo(pt3, normalCD, unitCD)
}

// cubicStroke is PathStroker::cubic_stroke in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) cubicStroke(cubic *[4]Point, quadPoints *quadConstruct) bool {
	if !s.foundTangents {
		resultType := s.tangentsMeet(cubic, quadPoints)
		if resultType != resultTypeQuad {
			ok := pointsWithinDist(
				quadPoints.quad[0],
				quadPoints.quad[2],
				s.invResScale,
			)
			if (resultType == resultTypeDegenerate || ok) &&
				s.cubicMidOnLine(cubic, quadPoints) {
				s.addDegenerateLine(quadPoints)
				return true
			}
		} else {
			s.foundTangents = true
		}
	}

	if s.foundTangents {
		resultType := s.compareQuadCubic(cubic, quadPoints)
		if resultType == resultTypeQuad {
			stroke := &quadPoints.quad
			if s.strokeType == strokeTypeOuter {
				s.outer.QuadTo(stroke[1].X, stroke[1].Y, stroke[2].X, stroke[2].Y)
			} else {
				s.inner.QuadTo(stroke[1].X, stroke[1].Y, stroke[2].X, stroke[2].Y)
			}

			return true
		}

		if resultType == resultTypeDegenerate {
			if !quadPoints.oppositeTangents {
				s.addDegenerateLine(quadPoints)
				return true
			}
		}
	}

	// Port: upstream tests quad[2].x twice. The fault is kept.
	if !isFinite(quadPoints.quad[2].X) || !isFinite(quadPoints.quad[2].X) { //nolint:staticcheck // the fault of upstream is kept (D1)
		return false // just abort if projected quad isn't representable
	}

	s.recursionDepth++
	foundTangentsIndex := 0
	if s.foundTangents {
		foundTangentsIndex = 1
	}
	if s.recursionDepth > recursiveLimits[foundTangentsIndex] {
		return false // just abort if projected quad isn't representable
	}

	var half quadConstruct
	if !half.initWithStart(quadPoints) {
		s.addDegenerateLine(quadPoints)
		s.recursionDepth--
		return true
	}

	if !s.cubicStroke(cubic, &half) {
		return false
	}

	if !half.initWithEnd(quadPoints) {
		s.addDegenerateLine(quadPoints)
		s.recursionDepth--
		return true
	}

	if !s.cubicStroke(cubic, &half) {
		return false
	}

	s.recursionDepth--
	return true
}

// cubicMidOnLine is PathStroker::cubic_mid_on_line in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) cubicMidOnLine(cubic *[4]Point, quadPoints *quadConstruct) bool {
	strokeMid := PointZero()
	s.cubicQuadMid(cubic, quadPoints, &strokeMid)
	dist := ptToLine(strokeMid, quadPoints.quad[0], quadPoints.quad[2])
	return dist < s.invResScaleSquared
}

// cubicQuadMid is PathStroker::cubic_quad_mid in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) cubicQuadMid(cubic *[4]Point, quadPoints *quadConstruct, mid *Point) {
	cubicMidPt := PointZero()
	s.cubicPerpRay(cubic, quadPoints.midT, &cubicMidPt, mid, nil)
}

// cubicPerpRay is PathStroker::cubic_perp_ray in tiny-skia-path
// path/src/stroker.rs.
//
// Given a cubic and t, return the point on curve,
// its perpendicular, and the perpendicular tangent.
func (s *PathStroker) cubicPerpRay(
	cubic *[4]Point,
	t NormalizedF32,
	tPt *Point,
	onPt *Point,
	tangent *Point,
) {
	*tPt = evalCubicPosAt(cubic, t)
	dxy := evalCubicTangentAt(cubic, t)

	var chopped [7]Point
	if dxy.X == 0.0 && dxy.Y == 0.0 {
		cPoints := cubic[:]
		if IsNearlyZero(t.Get()) {
			dxy = cubic[2].Sub(cubic[0])
		} else if IsNearlyZero(1.0 - t.Get()) {
			dxy = cubic[3].Sub(cubic[1])
		} else {
			// If the cubic inflection falls on the cusp, subdivide the cubic
			// to find the tangent at that point.
			//
			// Unwrap never fails, because we already checked that `t` is not 0/1,
			tt, ok := NewNormalizedF32Exclusive(t.Get())
			if !ok {
				panic("path: PathStroker::cubic_perp_ray: NormalizedF32Exclusive::new returned None")
			}
			ChopCubicAt2(cubic, tt, chopped[:])
			dxy = chopped[3].Sub(chopped[2])
			if dxy.X == 0.0 && dxy.Y == 0.0 {
				dxy = chopped[3].Sub(chopped[1])
				cPoints = chopped[:]
			}
		}

		if dxy.X == 0.0 && dxy.Y == 0.0 {
			dxy = cPoints[3].Sub(cPoints[0])
		}
	}

	s.setRayPoints(*tPt, &dxy, onPt, tangent)
}

// setCubicEndNormal is PathStroker::set_cubic_end_normal in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) setCubicEndNormal(
	cubic *[4]Point,
	normalAB Point,
	unitNormalAB Point,
	normalCD *Point,
	unitNormalCD *Point,
) {
	ab := cubic[1].Sub(cubic[0])
	cd := cubic[3].Sub(cubic[2])

	degenerateAB := degenerateVector(ab)
	degenerateCB := degenerateVector(cd)

	if degenerateAB && degenerateCB {
		*normalCD = normalAB
		*unitNormalCD = unitNormalAB
		return
	}

	if degenerateAB {
		ab = cubic[2].Sub(cubic[0])
		degenerateAB = degenerateVector(ab)
	}

	if degenerateCB {
		cd = cubic[3].Sub(cubic[1])
		degenerateCB = degenerateVector(cd)
	}

	if degenerateAB || degenerateCB {
		*normalCD = normalAB
		*unitNormalCD = unitNormalAB
		return
	}

	setNormalUnitNormal2(cd, s.radius, normalCD, unitNormalCD)
}

// compareQuadCubic is PathStroker::compare_quad_cubic in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) compareQuadCubic(cubic *[4]Point, quadPoints *quadConstruct) resultType {
	// get the quadratic approximation of the stroke
	s.cubicQuadEnds(cubic, quadPoints)
	resultType := s.intersectRay(intersectRayTypeCtrlPt, quadPoints)
	if resultType != resultTypeQuad {
		return resultType
	}

	// project a ray from the curve to the stroke
	// points near midpoint on quad, midpoint on cubic
	ray0 := PointZero()
	ray1 := PointZero()
	s.cubicPerpRay(cubic, quadPoints.midT, &ray1, &ray0, nil)
	quad := quadPoints.quad
	return s.strokeCloseEnough(&quad, &[2]Point{ray0, ray1}, quadPoints)
}

// cubicQuadEnds is PathStroker::cubic_quad_ends in tiny-skia-path
// path/src/stroker.rs.
//
// Given a cubic and a t range, find the start and end if they haven't been found already.
func (s *PathStroker) cubicQuadEnds(cubic *[4]Point, quadPoints *quadConstruct) {
	if !quadPoints.startSet {
		cubicStartPt := PointZero()
		s.cubicPerpRay(
			cubic,
			quadPoints.startT,
			&cubicStartPt,
			&quadPoints.quad[0],
			&quadPoints.tangentStart,
		)
		quadPoints.startSet = true
	}

	if !quadPoints.endSet {
		cubicEndPt := PointZero()
		s.cubicPerpRay(
			cubic,
			quadPoints.endT,
			&cubicEndPt,
			&quadPoints.quad[2],
			&quadPoints.tangentEnd,
		)
		quadPoints.endSet = true
	}
}

// close is PathStroker::close in tiny-skia-path path/src/stroker.rs.
func (s *PathStroker) close(isLine bool) {
	s.finishContour(true, isLine)
}

// finishContour is PathStroker::finish_contour in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) finishContour(close, currIsLine bool) {
	if s.segmentCount > 0 {
		if close {
			s.joiner.call(
				s.prevUnitNormal,
				s.prevPt,
				s.firstUnitNormal,
				s.radius,
				s.invMiterLimit,
				s.prevIsLine,
				currIsLine,
				s.builders(),
			)
			s.outer.Close()

			// now add inner as its own contour
			pt, _ := s.inner.LastPoint()
			s.outer.MoveTo(pt.X, pt.Y)
			s.outer.reversePathTo(&s.inner)
			s.outer.Close()
		} else {
			// add caps to start and end

			// cap the end
			pt, _ := s.inner.LastPoint()
			var otherPath *PathBuilder
			if currIsLine {
				otherPath = &s.inner
			}
			s.capper.call(
				s.prevPt,
				s.prevNormal,
				pt,
				otherPath,
				&s.outer,
			)
			s.outer.reversePathTo(&s.inner)

			// cap the start
			otherPath = nil
			if s.prevIsLine {
				otherPath = &s.inner
			}
			s.capper.call(
				s.firstPt,
				s.firstNormal.Neg(),
				s.firstOuterPt,
				otherPath,
				&s.outer,
			)
			s.outer.Close()
		}

		if !s.cusper.IsEmpty() {
			s.outer.pushPathBuilder(&s.cusper)
			s.cusper.Clear()
		}
	}

	// since we may re-use `inner`, we rewind instead of reset, to save on
	// reallocating its internal storage.
	s.inner.Clear()
	s.segmentCount = -1
	s.firstOuterPtIndexInContour = len(s.outer.points)
}

// preJoinTo is PathStroker::pre_join_to in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) preJoinTo(p Point, currIsLine bool, normal, unitNormal *Point) bool {
	prevX := s.prevPt.X
	prevY := s.prevPt.Y

	normalSet := setNormalUnitNormal(
		s.prevPt,
		p,
		s.resScale,
		s.radius,
		normal,
		unitNormal,
	)
	if !normalSet {
		if fnPtrEq(s.capper, capProcButt) {
			return false
		}

		// Square caps and round caps draw even if the segment length is zero.
		// Since the zero length segment has no direction, set the orientation
		// to upright as the default orientation.
		*normal = PointFromXY(s.radius, 0.0)
		*unitNormal = PointFromXY(1.0, 0.0)
	}

	if s.segmentCount == 0 {
		s.firstNormal = *normal
		s.firstUnitNormal = *unitNormal
		s.firstOuterPt = PointFromXY(prevX+normal.X, prevY+normal.Y)

		s.outer.MoveTo(s.firstOuterPt.X, s.firstOuterPt.Y)
		s.inner.MoveTo(prevX-normal.X, prevY-normal.Y)
	} else {
		// we have a previous segment
		s.joiner.call(
			s.prevUnitNormal,
			s.prevPt,
			*unitNormal,
			s.radius,
			s.invMiterLimit,
			s.prevIsLine,
			currIsLine,
			s.builders(),
		)
	}
	s.prevIsLine = currIsLine
	return true
}

// postJoinTo is PathStroker::post_join_to in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) postJoinTo(p, normal, unitNormal Point) {
	s.joinCompleted = true
	s.prevPt = p
	s.prevUnitNormal = unitNormal
	s.prevNormal = normal
	s.segmentCount++
}

// initQuad is PathStroker::init_quad in tiny-skia-path path/src/stroker.rs.
func (s *PathStroker) initQuad(
	strokeType strokeType,
	start NormalizedF32,
	end NormalizedF32,
	quadPoints *quadConstruct,
) {
	s.strokeType = strokeType
	s.foundTangents = false
	quadPoints.init(start, end)
}

// quadStroke is PathStroker::quad_stroke in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) quadStroke(quad *[3]Point, quadPoints *quadConstruct) bool {
	resultType := s.compareQuadQuad(quad, quadPoints)
	if resultType == resultTypeQuad {
		path := &s.inner
		if s.strokeType == strokeTypeOuter {
			path = &s.outer
		}

		path.QuadTo(
			quadPoints.quad[1].X,
			quadPoints.quad[1].Y,
			quadPoints.quad[2].X,
			quadPoints.quad[2].Y,
		)

		return true
	}

	if resultType == resultTypeDegenerate {
		s.addDegenerateLine(quadPoints)
		return true
	}

	s.recursionDepth++
	if s.recursionDepth > recursiveLimits[quadRecursiveLimit] {
		return false // just abort if projected quad isn't representable
	}

	var half quadConstruct
	half.initWithStart(quadPoints)
	if !s.quadStroke(quad, &half) {
		return false
	}

	half.initWithEnd(quadPoints)
	if !s.quadStroke(quad, &half) {
		return false
	}

	s.recursionDepth--
	return true
}

// compareQuadQuad is PathStroker::compare_quad_quad in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) compareQuadQuad(quad *[3]Point, quadPoints *quadConstruct) resultType {
	// get the quadratic approximation of the stroke
	if !quadPoints.startSet {
		quadStartPt := PointZero()
		s.quadPerpRay(
			quad,
			quadPoints.startT,
			&quadStartPt,
			&quadPoints.quad[0],
			&quadPoints.tangentStart,
		)
		quadPoints.startSet = true
	}

	if !quadPoints.endSet {
		quadEndPt := PointZero()
		s.quadPerpRay(
			quad,
			quadPoints.endT,
			&quadEndPt,
			&quadPoints.quad[2],
			&quadPoints.tangentEnd,
		)
		quadPoints.endSet = true
	}

	resultType := s.intersectRay(intersectRayTypeCtrlPt, quadPoints)
	if resultType != resultTypeQuad {
		return resultType
	}

	// project a ray from the curve to the stroke
	ray0 := PointZero()
	ray1 := PointZero()
	s.quadPerpRay(quad, quadPoints.midT, &ray1, &ray0, nil)
	stroke := quadPoints.quad
	return s.strokeCloseEnough(&stroke, &[2]Point{ray0, ray1}, quadPoints)
}

// setRayPoints is PathStroker::set_ray_points in tiny-skia-path
// path/src/stroker.rs.
//
// Given a point on the curve and its derivative, scale the derivative by the radius, and
// compute the perpendicular point and its tangent.
func (s *PathStroker) setRayPoints(tp Point, dxy *Point, onP *Point, tangent *Point) {
	if !dxy.SetLength(s.radius) {
		*dxy = PointFromXY(s.radius, 0.0)
	}

	axisFlip := float32(int32(s.strokeType)) // go opposite ways for outer, inner
	onP.X = tp.X + float32(axisFlip*dxy.Y)
	onP.Y = tp.Y - float32(axisFlip*dxy.X)

	if tangent != nil {
		tangent.X = onP.X + dxy.X
		tangent.Y = onP.Y + dxy.Y
	}
}

// quadPerpRay is PathStroker::quad_perp_ray in tiny-skia-path
// path/src/stroker.rs.
//
// Given a quad and t, return the point on curve,
// its perpendicular, and the perpendicular tangent.
func (s *PathStroker) quadPerpRay(
	quad *[3]Point,
	t NormalizedF32,
	tp *Point,
	onP *Point,
	tangent *Point,
) {
	*tp = evalQuadAt(quad, t)
	dxy := evalQuadTangentAt(quad, t)

	if dxy.IsZero() {
		dxy = quad[2].Sub(quad[0])
	}

	s.setRayPoints(*tp, &dxy, onP, tangent)
}

// addDegenerateLine is PathStroker::add_degenerate_line in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) addDegenerateLine(quadPoints *quadConstruct) {
	if s.strokeType == strokeTypeOuter {
		s.outer.LineTo(quadPoints.quad[2].X, quadPoints.quad[2].Y)
	} else {
		s.inner.LineTo(quadPoints.quad[2].X, quadPoints.quad[2].Y)
	}
}

// strokeCloseEnough is PathStroker::stroke_close_enough in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) strokeCloseEnough(
	stroke *[3]Point,
	ray *[2]Point,
	quadPoints *quadConstruct,
) resultType {
	half := NormalizedF32Clamped(0.5)
	strokeMid := evalQuadAt(stroke, half)
	// measure the distance from the curve to the quad-stroke midpoint, compare to radius
	if pointsWithinDist(ray[0], strokeMid, s.invResScale) {
		// if the difference is small
		if sharpAngle(&quadPoints.quad) {
			return resultTypeSplit
		}

		return resultTypeQuad
	}

	// measure the distance to quad's bounds (quick reject)
	// an alternative : look for point in triangle
	if !ptInQuadBounds(stroke, ray[0], s.invResScale) {
		// if far, subdivide
		return resultTypeSplit
	}

	// measure the curve ray distance to the quad-stroke
	rootsBuf := NewTValues()
	roots := intersectQuadRay(ray, stroke, &rootsBuf)
	if len(roots) != 1 {
		return resultTypeSplit
	}

	quadPt := evalQuadAt(stroke, roots[0].ToNormalized())
	err := float32(s.invResScale * (1.0 - float32(absF32(roots[0].Get()-0.5)*2.0)))
	if pointsWithinDist(ray[0], quadPt, err) {
		// if the difference is small, we're done
		if sharpAngle(&quadPoints.quad) {
			return resultTypeSplit
		}

		return resultTypeQuad
	}

	// otherwise, subdivide
	return resultTypeSplit
}

// intersectRay is PathStroker::intersect_ray in tiny-skia-path
// path/src/stroker.rs.
//
// Find the intersection of the stroke tangents to construct a stroke quad.
// Return whether the stroke is a degenerate (a line), a quad, or must be split.
// Optionally compute the quad's control point.
func (s *PathStroker) intersectRay(
	intersectRayType intersectRayType,
	quadPoints *quadConstruct,
) resultType {
	start := quadPoints.quad[0]
	end := quadPoints.quad[2]
	aLen := quadPoints.tangentStart.Sub(start)
	bLen := quadPoints.tangentEnd.Sub(end)

	// Slopes match when denom goes to zero:
	//
	//nolint:dupword // the equations of upstream repeat a name
	//                   axLen / ayLen ==                   bxLen / byLen
	// (ayLen * byLen) * axLen / ayLen == (ayLen * byLen) * bxLen / byLen
	//          byLen  * axLen         ==  ayLen          * bxLen
	//          byLen  * axLen         -   ayLen          * bxLen         ( == denom )
	denom := aLen.Cross(bLen)
	if denom == 0.0 || !isFinite(denom) {
		quadPoints.oppositeTangents = aLen.Dot(bLen) < 0.0
		return resultTypeDegenerate
	}

	quadPoints.oppositeTangents = false
	ab0 := start.Sub(end)
	numerA := bLen.Cross(ab0)
	numerB := aLen.Cross(ab0)
	if (numerA >= 0.0) == (numerB >= 0.0) {
		// if the control point is outside the quad ends

		// if the perpendicular distances from the quad points to the opposite tangent line
		// are small, a straight line is good enough
		dist1 := ptToLine(start, end, quadPoints.tangentEnd)
		dist2 := ptToLine(end, start, quadPoints.tangentStart)
		if maxF32(dist1, dist2) <= s.invResScaleSquared {
			return resultTypeDegenerate
		}

		return resultTypeSplit
	}

	// check to see if the denominator is teeny relative to the numerator
	// if the offset by one will be lost, the ratio is too large
	numerA /= denom
	validDivide := numerA > numerA-1.0
	if validDivide {
		if intersectRayType == intersectRayTypeCtrlPt {
			// the intersection of the tangents need not be on the tangent segment
			// so 0 <= numer_a <= 1 is not necessarily true
			quadPoints.quad[1].X =
				float32(start.X*(1.0-numerA)) + float32(quadPoints.tangentStart.X*numerA)
			quadPoints.quad[1].Y =
				float32(start.Y*(1.0-numerA)) + float32(quadPoints.tangentStart.Y*numerA)
		}

		return resultTypeQuad
	}

	quadPoints.oppositeTangents = aLen.Dot(bLen) < 0.0

	// if the lines are parallel, straight line is good enough
	return resultTypeDegenerate
}

// tangentsMeet is PathStroker::tangents_meet in tiny-skia-path
// path/src/stroker.rs.
//
// Given a cubic and a t-range, determine if the stroke can be described by a quadratic.
func (s *PathStroker) tangentsMeet(cubic *[4]Point, quadPoints *quadConstruct) resultType {
	s.cubicQuadEnds(cubic, quadPoints)
	return s.intersectRay(intersectRayTypeResultType, quadPoints)
}

// finish is PathStroker::finish in tiny-skia-path path/src/stroker.rs.
func (s *PathStroker) finish(isLine bool) (*Path, bool) {
	s.finishContour(false, isLine)

	// Swap out the outer builder.
	buf := s.outer
	s.outer = *NewPathBuilder()

	return buf.Finish()
}

// hasOnlyMoveTo is PathStroker::has_only_move_to in tiny-skia-path
// path/src/stroker.rs.
func (s *PathStroker) hasOnlyMoveTo() bool {
	return s.segmentCount == 0
}

// isCurrentContourEmpty is PathStroker::is_current_contour_empty in
// tiny-skia-path path/src/stroker.rs.
func (s *PathStroker) isCurrentContourEmpty() bool {
	return s.inner.isZeroLengthSincePoint(0) &&
		s.outer.isZeroLengthSincePoint(s.firstOuterPtIndexInContour)
}

// capFactory is cap_factory in tiny-skia-path path/src/stroker.rs.
func capFactory(lineCap LineCap) capProc {
	switch lineCap {
	case LineCapRound:
		return capProcRound
	case LineCapSquare:
		return capProcSquare
	}
	return capProcButt
}

// buttCapper is butt_capper in tiny-skia-path path/src/stroker.rs.
func buttCapper(_, _, stop Point, _ *PathBuilder, path *PathBuilder) {
	path.LineTo(stop.X, stop.Y)
}

// roundCapper is round_capper in tiny-skia-path path/src/stroker.rs.
func roundCapper(pivot, normal, stop Point, _ *PathBuilder, path *PathBuilder) {
	parallel := normal
	parallel.rotateCW()

	projectedCenter := pivot.Add(parallel)

	path.conicPointsTo(
		projectedCenter.Add(normal),
		projectedCenter,
		ScalarRoot2Over2,
	)
	path.conicPointsTo(projectedCenter.Sub(normal), stop, ScalarRoot2Over2)
}

// squareCapper is square_capper in tiny-skia-path path/src/stroker.rs.
func squareCapper(pivot, normal, stop Point, otherPath *PathBuilder, path *PathBuilder) {
	parallel := normal
	parallel.rotateCW()

	if otherPath != nil {
		path.setLastPoint(PointFromXY(
			pivot.X+normal.X+parallel.X,
			pivot.Y+normal.Y+parallel.Y,
		))
		path.LineTo(
			pivot.X-normal.X+parallel.X,
			pivot.Y-normal.Y+parallel.Y,
		)
	} else {
		path.LineTo(
			pivot.X+normal.X+parallel.X,
			pivot.Y+normal.Y+parallel.Y,
		)
		path.LineTo(
			pivot.X-normal.X+parallel.X,
			pivot.Y-normal.Y+parallel.Y,
		)
		path.LineTo(stop.X, stop.Y)
	}
}

// joinFactory is join_factory in tiny-skia-path path/src/stroker.rs.
func joinFactory(join LineJoin) joinProc {
	switch join {
	case LineJoinMiterClip:
		return joinProcMiterClip
	case LineJoinRound:
		return joinProcRound
	case LineJoinBevel:
		return joinProcBevel
	}
	return joinProcMiter
}

// isClockwise is is_clockwise in tiny-skia-path path/src/stroker.rs.
func isClockwise(before, after Point) bool {
	return float32(before.X*after.Y) > float32(before.Y*after.X)
}

// angleType is AngleType in tiny-skia-path path/src/stroker.rs.
type angleType uint8

const (
	angleTypeNearly180 angleType = iota
	angleTypeSharp
	angleTypeShallow
	angleTypeNearlyLine
)

// dotToAngleType is dot_to_angle_type in tiny-skia-path path/src/stroker.rs.
func dotToAngleType(dot float32) angleType {
	if dot >= 0.0 {
		// shallow or line
		if IsNearlyZero(1.0 - dot) {
			return angleTypeNearlyLine
		}
		return angleTypeShallow
	}
	// sharp or 180
	if IsNearlyZero(1.0 + dot) {
		return angleTypeNearly180
	}
	return angleTypeSharp
}

// handleInnerJoin is handle_inner_join in tiny-skia-path path/src/stroker.rs.
func handleInnerJoin(pivot, after Point, inner *PathBuilder) {
	// In the degenerate case that the stroke radius is larger than our segments
	// just connecting the two inner segments may "show through" as a funny
	// diagonal. To pseudo-fix this, we go through the pivot point. This adds
	// an extra point/edge, but I can't see a cheap way to know when this is
	// not needed :(
	inner.LineTo(pivot.X, pivot.Y)

	inner.LineTo(pivot.X-after.X, pivot.Y-after.Y)
}

// bevelJoiner is bevel_joiner in tiny-skia-path path/src/stroker.rs.
func bevelJoiner(
	beforeUnitNormal Point,
	pivot Point,
	afterUnitNormal Point,
	radius float32,
	_ float32,
	_ bool,
	_ bool,
	builders swappableBuilders,
) {
	after := afterUnitNormal.scaled(radius)

	if !isClockwise(beforeUnitNormal, afterUnitNormal) {
		builders.swap()
		after = after.Neg()
	}

	builders.outer.LineTo(pivot.X+after.X, pivot.Y+after.Y)
	handleInnerJoin(pivot, after, builders.inner)
}

// roundJoiner is round_joiner in tiny-skia-path path/src/stroker.rs.
func roundJoiner(
	beforeUnitNormal Point,
	pivot Point,
	afterUnitNormal Point,
	radius float32,
	_ float32,
	_ bool,
	_ bool,
	builders swappableBuilders,
) {
	dotProd := beforeUnitNormal.Dot(afterUnitNormal)
	angleType := dotToAngleType(dotProd)

	if angleType == angleTypeNearlyLine {
		return
	}

	before := beforeUnitNormal
	after := afterUnitNormal
	dir := pathDirectionCW

	if !isClockwise(before, after) {
		builders.swap()
		before = before.Neg()
		after = after.Neg()
		dir = pathDirectionCCW
	}

	ts := TransformFromRow(radius, 0.0, 0.0, radius, pivot.X, pivot.Y)

	var conicsBuf [5]conic
	conics, ok := buildUnitArc(before, after, dir, ts, &conicsBuf)
	if ok {
		for _, conic := range conics {
			builders.outer.conicPointsTo(conic.points[1], conic.points[2], conic.weight)
		}

		after.Scale(radius)
		handleInnerJoin(pivot, after, builders.inner)
	}
}

// miterJoiner is miter_joiner in tiny-skia-path path/src/stroker.rs.
func miterJoiner(
	beforeUnitNormal Point,
	pivot Point,
	afterUnitNormal Point,
	radius float32,
	invMiterLimit float32,
	prevIsLine bool,
	currIsLine bool,
	builders swappableBuilders,
) {
	miterJoinerInner(
		beforeUnitNormal,
		pivot,
		afterUnitNormal,
		radius,
		invMiterLimit,
		false,
		prevIsLine,
		currIsLine,
		builders,
	)
}

// miterClipJoiner is miter_clip_joiner in tiny-skia-path path/src/stroker.rs.
func miterClipJoiner(
	beforeUnitNormal Point,
	pivot Point,
	afterUnitNormal Point,
	radius float32,
	invMiterLimit float32,
	prevIsLine bool,
	currIsLine bool,
	builders swappableBuilders,
) {
	miterJoinerInner(
		beforeUnitNormal,
		pivot,
		afterUnitNormal,
		radius,
		invMiterLimit,
		true,
		prevIsLine,
		currIsLine,
		builders,
	)
}

// doBluntOrClipped is do_blunt_or_clipped, a function inside miter_joiner_inner
// in tiny-skia-path path/src/stroker.rs.
func doBluntOrClipped(
	builders swappableBuilders,
	pivot Point,
	radius float32,
	prevIsLine bool,
	currIsLine bool,
	before Point,
	mid Point,
	after Point,
	invMiterLimit float32,
	miterClip bool,
) {
	after.Scale(radius)

	if miterClip {
		mid.Normalize()

		cosBeta := before.Dot(mid)
		sinBeta := before.Cross(mid)

		var x float32
		if absF32(sinBeta) <= ScalarNearlyZero {
			x = 1.0 / invMiterLimit
		} else {
			x = ((1.0 / invMiterLimit) - cosBeta) / sinBeta
		}

		before.Scale(radius)

		beforeTangent := before
		beforeTangent.rotateCW()

		afterTangent := after
		afterTangent.rotateCCW()

		c1 := pivot.Add(before).Add(beforeTangent.scaled(x))
		c2 := pivot.Add(after).Add(afterTangent.scaled(x))

		if prevIsLine {
			builders.outer.setLastPoint(c1)
		} else {
			builders.outer.LineTo(c1.X, c1.Y)
		}

		builders.outer.LineTo(c2.X, c2.Y)
	}

	if !currIsLine {
		builders.outer.LineTo(pivot.X+after.X, pivot.Y+after.Y)
	}

	handleInnerJoin(pivot, after, builders.inner)
}

// doMiter is do_miter, a function inside miter_joiner_inner in tiny-skia-path
// path/src/stroker.rs.
func doMiter(
	builders swappableBuilders,
	pivot Point,
	radius float32,
	prevIsLine bool,
	currIsLine bool,
	mid Point,
	after Point,
) {
	after.Scale(radius)

	if prevIsLine {
		builders.outer.setLastPoint(PointFromXY(pivot.X+mid.X, pivot.Y+mid.Y))
	} else {
		builders.outer.LineTo(pivot.X+mid.X, pivot.Y+mid.Y)
	}

	if !currIsLine {
		builders.outer.LineTo(pivot.X+after.X, pivot.Y+after.Y)
	}

	handleInnerJoin(pivot, after, builders.inner)
}

// miterJoinerInner is miter_joiner_inner in tiny-skia-path
// path/src/stroker.rs.
//
// Port: its two inner functions, do_blunt_or_clipped and do_miter, are the
// functions doBluntOrClipped and doMiter above.
func miterJoinerInner(
	beforeUnitNormal Point,
	pivot Point,
	afterUnitNormal Point,
	radius float32,
	invMiterLimit float32,
	miterClip bool,
	prevIsLine bool,
	currIsLine bool,
	builders swappableBuilders,
) {
	// negate the dot since we're using normals instead of tangents
	dotProd := beforeUnitNormal.Dot(afterUnitNormal)
	angleType := dotToAngleType(dotProd)
	before := beforeUnitNormal
	after := afterUnitNormal
	var mid Point

	if angleType == angleTypeNearlyLine {
		return
	}

	if angleType == angleTypeNearly180 {
		currIsLine = false
		mid = after.Sub(before).scaled(radius / 2.0)
		doBluntOrClipped(
			builders,
			pivot,
			radius,
			prevIsLine,
			currIsLine,
			before,
			mid,
			after,
			invMiterLimit,
			miterClip,
		)
		return
	}

	ccw := !isClockwise(before, after)
	if ccw {
		builders.swap()
		before = before.Neg()
		after = after.Neg()
	}

	// Before we enter the world of square-roots and divides,
	// check if we're trying to join an upright right angle
	// (common case for stroking rectangles). If so, special case
	// that (for speed an accuracy).
	// Note: we only need to check one normal if dot==0
	if dotProd == 0.0 && invMiterLimit <= ScalarRoot2Over2 {
		mid = before.Add(after).scaled(radius)
		doMiter(
			builders,
			pivot,
			radius,
			prevIsLine,
			currIsLine,
			mid,
			after,
		)
		return
	}

	// choose the most accurate way to form the initial mid-vector
	if angleType == angleTypeSharp {
		mid = PointFromXY(after.Y-before.Y, before.X-after.X)
		if ccw {
			mid = mid.Neg()
		}
	} else {
		mid = PointFromXY(before.X+after.X, before.Y+after.Y)
	}

	// midLength = radius / sinHalfAngle
	// if (midLength > miterLimit * radius) abort
	// if (radius / sinHalf > miterLimit * radius) abort
	// if (1 / sinHalf > miterLimit) abort
	// if (1 / miterLimit > sinHalf) abort
	// My dotProd is opposite sign, since it is built from normals and not tangents
	// hence 1 + dot instead of 1 - dot in the formula
	sinHalfAngle := sqrtF32(Half(1.0 + dotProd))
	if sinHalfAngle < invMiterLimit {
		currIsLine = false
		doBluntOrClipped(
			builders,
			pivot,
			radius,
			prevIsLine,
			currIsLine,
			before,
			mid,
			after,
			invMiterLimit,
			miterClip,
		)
		return
	}

	mid.SetLength(radius / sinHalfAngle)
	doMiter(
		builders,
		pivot,
		radius,
		prevIsLine,
		currIsLine,
		mid,
		after,
	)
}

// setNormalUnitNormal is set_normal_unit_normal in tiny-skia-path
// path/src/stroker.rs.
func setNormalUnitNormal(
	before Point,
	after Point,
	scale float32,
	radius float32,
	normal *Point,
	unitNormal *Point,
) bool {
	if !unitNormal.SetNormalize(float32((after.X-before.X)*scale), float32((after.Y-before.Y)*scale)) {
		return false
	}

	unitNormal.rotateCCW()
	*normal = unitNormal.scaled(radius)
	return true
}

// setNormalUnitNormal2 is set_normal_unit_normal2 in tiny-skia-path
// path/src/stroker.rs.
func setNormalUnitNormal2(vec Point, radius float32, normal, unitNormal *Point) bool {
	if !unitNormal.SetNormalize(vec.X, vec.Y) {
		return false
	}

	unitNormal.rotateCCW()
	*normal = unitNormal.scaled(radius)
	return true
}

// fnPtrEq is fn_ptr_eq in tiny-skia-path path/src/stroker.rs.
func fnPtrEq(f1, f2 capProc) bool {
	return f1 == f2
}

// quadConstruct is QuadConstruct in tiny-skia-path path/src/stroker.rs.
//
// The zero value is the Default impl of upstream.
type quadConstruct struct {
	// The state of the quad stroke under construction.
	quad             [3]Point      // the stroked quad parallel to the original curve
	tangentStart     Point         // a point tangent to quad[0]
	tangentEnd       Point         // a point tangent to quad[2]
	startT           NormalizedF32 // a segment of the original curve
	midT             NormalizedF32
	endT             NormalizedF32
	startSet         bool // state to share common points across structs
	endSet           bool
	oppositeTangents bool // set if coincident tangents have opposite directions
}

// init is QuadConstruct::init in tiny-skia-path path/src/stroker.rs.
//
// return false if start and end are too close to have a unique middle.
func (q *quadConstruct) init(start, end NormalizedF32) bool {
	q.startT = start
	q.midT = NormalizedF32Clamped(Half(start.Get() + end.Get()))
	q.endT = end
	q.startSet = false
	q.endSet = false
	return q.startT < q.midT && q.midT < q.endT
}

// initWithStart is QuadConstruct::init_with_start in tiny-skia-path
// path/src/stroker.rs.
func (q *quadConstruct) initWithStart(parent *quadConstruct) bool {
	if !q.init(parent.startT, parent.midT) {
		return false
	}

	q.quad[0] = parent.quad[0]
	q.tangentStart = parent.tangentStart
	q.startSet = true
	return true
}

// initWithEnd is QuadConstruct::init_with_end in tiny-skia-path
// path/src/stroker.rs.
func (q *quadConstruct) initWithEnd(parent *quadConstruct) bool {
	if !q.init(parent.midT, parent.endT) {
		return false
	}

	q.quad[2] = parent.quad[2]
	q.tangentEnd = parent.tangentEnd
	q.endSet = true
	return true
}

// checkQuadLinear is check_quad_linear in tiny-skia-path path/src/stroker.rs.
func checkQuadLinear(quad *[3]Point) (Point, reductionType) {
	degenerateAB := degenerateVector(quad[1].Sub(quad[0]))
	degenerateBC := degenerateVector(quad[2].Sub(quad[1]))
	if degenerateAB && degenerateBC {
		return PointZero(), reductionTypePoint
	}

	if degenerateAB || degenerateBC {
		return PointZero(), reductionTypeLine
	}

	if !quadInLine(quad) {
		return PointZero(), reductionTypeQuad
	}

	t := findQuadMaxCurvature(quad)
	if t == NormalizedF32Zero || t == NormalizedF32One {
		return PointZero(), reductionTypeLine
	}

	return evalQuadAt(quad, t), reductionTypeDegenerate
}

// degenerateVector is degenerate_vector in tiny-skia-path
// path/src/stroker.rs.
func degenerateVector(v Point) bool {
	return !v.canNormalize()
}

// quadInLine is quad_in_line in tiny-skia-path path/src/stroker.rs.
//
// Given quad, see if all there points are in a line.
// Return true if the inside point is close to a line connecting the outermost points.
//
// Find the outermost point by looking for the largest difference in X or Y.
// Since the XOR of the indices is 3  (0 ^ 1 ^ 2)
// the missing index equals: outer_1 ^ outer_2 ^ 3.
func quadInLine(quad *[3]Point) bool {
	var ptMax float32 = -1.0
	outer1 := 0
	outer2 := 0
	for index := range 2 {
		for inner := index + 1; inner < 3; inner++ {
			testDiff := quad[inner].Sub(quad[index])
			testMax := maxF32(absF32(testDiff.X), absF32(testDiff.Y))
			if ptMax < testMax {
				outer1 = index
				outer2 = inner
				ptMax = testMax
			}
		}
	}

	mid := outer1 ^ outer2 ^ 3
	const curvatureSlop float32 = 0.000005 // this multiplier is pulled out of the air
	lineSlop := float32(float32(ptMax*ptMax) * curvatureSlop)
	return ptToLine(quad[mid], quad[outer1], quad[outer2]) <= lineSlop
}

// ptToLine is pt_to_line in tiny-skia-path path/src/stroker.rs.
//
// returns the distance squared from the point to the line.
func ptToLine(pt, lineStart, lineEnd Point) float32 {
	dxy := lineEnd.Sub(lineStart)
	ab0 := pt.Sub(lineStart)
	numer := dxy.Dot(ab0)
	denom := dxy.Dot(dxy)
	t := numer / denom
	if t >= 0.0 && t <= 1.0 {
		hit := PointFromXY(
			float32(lineStart.X*(1.0-t))+float32(lineEnd.X*t),
			float32(lineStart.Y*(1.0-t))+float32(lineEnd.Y*t),
		)
		return hit.distanceToSqd(pt)
	}
	return pt.distanceToSqd(lineStart)
}

// intersectQuadRay is intersect_quad_ray in tiny-skia-path
// path/src/stroker.rs.
//
// Intersect the line with the quad and return the t values on the quad where the line crosses.
func intersectQuadRay(
	line *[2]Point,
	quad *[3]Point,
	roots *[3]NormalizedF32Exclusive,
) []NormalizedF32Exclusive {
	vec := line[1].Sub(line[0])
	var r [3]float32
	for n := range 3 {
		r[n] = float32((quad[n].Y-line[0].Y)*vec.X) - float32((quad[n].X-line[0].X)*vec.Y)
	}
	a := r[2]
	b := r[1]
	c := r[0]
	a += c - float32(2.0*b) // A = a - 2*b + c
	b -= c                  // B = -(b - c)

	n := FindUnitQuadRoots(a, float32(2.0*b), c, roots)
	return roots[0:n]
}

// pointsWithinDist is points_within_dist in tiny-skia-path
// path/src/stroker.rs.
func pointsWithinDist(nearPt, farPt Point, limit float32) bool {
	return nearPt.distanceToSqd(farPt) <= float32(limit*limit)
}

// sharpAngle is sharp_angle in tiny-skia-path path/src/stroker.rs.
func sharpAngle(quad *[3]Point) bool {
	smaller := quad[1].Sub(quad[0])
	larger := quad[1].Sub(quad[2])
	smallerLen := smaller.lengthSqd()
	largerLen := larger.lengthSqd()
	if smallerLen > largerLen {
		smaller, larger = larger, smaller
		largerLen = smallerLen
	}

	if !smaller.SetLength(largerLen) {
		return false
	}

	dot := smaller.Dot(larger)
	return dot > 0.0
}

// ptInQuadBounds is pt_in_quad_bounds in tiny-skia-path path/src/stroker.rs.
//
// Return true if the point is close to the bounds of the quad. This is used as a quick reject.
func ptInQuadBounds(quad *[3]Point, pt Point, invResScale float32) bool {
	xMin := minF32(minF32(quad[0].X, quad[1].X), quad[2].X)
	if pt.X+invResScale < xMin {
		return false
	}

	xMax := maxF32(maxF32(quad[0].X, quad[1].X), quad[2].X)
	if pt.X-invResScale > xMax {
		return false
	}

	yMin := minF32(minF32(quad[0].Y, quad[1].Y), quad[2].Y)
	if pt.Y+invResScale < yMin {
		return false
	}

	yMax := maxF32(maxF32(quad[0].Y, quad[1].Y), quad[2].Y)
	return pt.Y-invResScale <= yMax
}

// checkCubicLinear is check_cubic_linear in tiny-skia-path
// path/src/stroker.rs.
//
// Port: a nil tangentPt is the Rust None.
func checkCubicLinear(
	cubic *[4]Point,
	reduction *[3]Point,
	tangentPt *Point,
) reductionType {
	degenerateAB := degenerateVector(cubic[1].Sub(cubic[0]))
	degenerateBC := degenerateVector(cubic[2].Sub(cubic[1]))
	degenerateCD := degenerateVector(cubic[3].Sub(cubic[2]))
	if degenerateAB && degenerateBC && degenerateCD {
		return reductionTypePoint
	}

	if boolToInt32(degenerateAB)+boolToInt32(degenerateBC)+boolToInt32(degenerateCD) == 2 {
		return reductionTypeLine
	}

	if !cubicInLine(cubic) {
		if tangentPt != nil {
			if degenerateAB {
				*tangentPt = cubic[2]
			} else {
				*tangentPt = cubic[1]
			}
		}

		return reductionTypeQuad
	}

	tValuesBuf := [3]NormalizedF32{NormalizedF32Zero, NormalizedF32Zero, NormalizedF32Zero}
	tValues := FindCubicMaxCurvature(cubic, &tValuesBuf)
	rCount := 0
	// Now loop over the t-values, and reject any that evaluate to either end-point
	for _, t := range tValues {
		if 0.0 >= t.Get() || t.Get() >= 1.0 {
			continue
		}

		reduction[rCount] = evalCubicPosAt(cubic, t)
		if reduction[rCount] != cubic[0] && reduction[rCount] != cubic[3] {
			rCount++
		}
	}

	switch rCount {
	case 0:
		return reductionTypeLine
	case 1:
		return reductionTypeDegenerate
	case 2:
		return reductionTypeDegenerate2
	case 3:
		return reductionTypeDegenerate3
	}
	panic("path: check_cubic_linear: unreachable")
}

// boolToInt32 ports no upstream function. It is the Rust cast "b as i32".
func boolToInt32(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// cubicInLine is cubic_in_line in tiny-skia-path path/src/stroker.rs.
//
// Given a cubic, determine if all four points are in a line.
//
// Return true if the inner points is close to a line connecting the outermost points.
//
// Find the outermost point by looking for the largest difference in X or Y.
// Given the indices of the outermost points, and that outer_1 is greater than outer_2,
// this table shows the index of the smaller of the remaining points:
//
//	                outer_2
//	            0    1    2    3
//	outer_1     ----------------
//	   0     |  -    2    1    1
//	   1     |  -    -    0    0
//	   2     |  -    -    -    0
//	   3     |  -    -    -    -
//
// If outer_1 == 0 and outer_2 == 1, the smaller of the remaining indices (2 and 3) is 2.
//
// This table can be collapsed to: (1 + (2 >> outer_2)) >> outer_1
//
// Given three indices (outer_1 outer_2 mid_1) from 0..3, the remaining index is:
//
//	mid_2 == (outer_1 ^ outer_2 ^ mid_1)
func cubicInLine(cubic *[4]Point) bool {
	var ptMax float32 = -1.0
	outer1 := 0
	outer2 := 0
	for index := range 3 {
		for inner := index + 1; inner < 4; inner++ {
			testDiff := cubic[inner].Sub(cubic[index])
			testMax := maxF32(absF32(testDiff.X), absF32(testDiff.Y))
			if ptMax < testMax {
				outer1 = index
				outer2 = inner
				ptMax = testMax
			}
		}
	}
	mid1 := (1 + (2 >> outer2)) >> outer1
	mid2 := outer1 ^ outer2 ^ mid1
	lineSlop := float32(float32(ptMax*ptMax) * 0.00001) // this multiplier is pulled out of the air

	return ptToLine(cubic[mid1], cubic[outer1], cubic[outer2]) <= lineSlop &&
		ptToLine(cubic[mid2], cubic[outer1], cubic[outer2]) <= lineSlop
}
