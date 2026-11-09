// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import (
	"iter"
	"math"
	"slices"
	"strconv"
	"strings"
)

// PathVerb is PathVerb in tiny-skia-path path/src/path.rs. It is a path verb.
type PathVerb uint8

// The values of PathVerb.
const (
	PathVerbMove PathVerb = iota
	PathVerbLine
	PathVerbQuad
	PathVerbCubic
	PathVerbClose
)

// String satisfies the fmt.Stringer interface.
func (v PathVerb) String() string {
	switch v {
	case PathVerbMove:
		return "Move"
	case PathVerbLine:
		return "Line"
	case PathVerbQuad:
		return "Quad"
	case PathVerbCubic:
		return "Cubic"
	case PathVerbClose:
		return "Close"
	}
	return "PathVerb(?)"
}

// Path is Path in tiny-skia-path path/src/path.rs. It is a Bezier path.
//
// A PathBuilder creates a Path. PathBuilder.Finish returns the path, and
// Path.Clear returns a PathBuilder that reuses the memory of the path.
//
// A Path is immutable and uses compact storage: it stores the types of the
// segments and the points apart. Path.Verbs and Path.Points give access to
// them, and so does Path.Segments.
//
// The type guarantees these facts:
//   - It has valid bounds, computed in advance.
//   - All points are finite.
//   - It has at least two segments.
//   - Each contour starts with a MoveTo.
//   - No Move follows a Move.
//   - No Close follows a Close.
//   - Contours of zero length are allowed.
type Path struct {
	verbs  []PathVerb
	points []Point
	bounds Rect
}

// Len is Path::len in tiny-skia-path path/src/path.rs. It returns the number
// of segments in the path.
func (p *Path) Len() int {
	return len(p.verbs)
}

// IsEmpty is Path::is_empty in tiny-skia-path path/src/path.rs. It returns
// true if the path is empty.
func (p *Path) IsEmpty() bool {
	return len(p.verbs) == 0
}

// Bounds is Path::bounds in tiny-skia-path path/src/path.rs. It returns the
// bounds of the points of the path.
//
// The value is already computed.
func (p *Path) Bounds() Rect {
	return p.bounds
}

// String is the Debug impl of Path in tiny-skia-path path/src/path.rs. It
// satisfies the fmt.Stringer interface. It writes the segments as the
// commands of an SVG path, and the bounds.
//
// Port: Rust writes the struct with the Debug format of Rust. Go writes the
// same text.
func (p *Path) String() string {
	var b strings.Builder
	for s := range p.Segments().All() {
		pt := s.Points
		switch s.Kind {
		case PathSegmentMoveTo:
			b.WriteString("M " + displayF32(pt[0].X) + " " + displayF32(pt[0].Y) + " ")
		case PathSegmentLineTo:
			b.WriteString("L " + displayF32(pt[0].X) + " " + displayF32(pt[0].Y) + " ")
		case PathSegmentQuadTo:
			b.WriteString("Q " + displayF32(pt[0].X) + " " + displayF32(pt[0].Y) + " " +
				displayF32(pt[1].X) + " " + displayF32(pt[1].Y) + " ")
		case PathSegmentCubicTo:
			b.WriteString("C " + displayF32(pt[0].X) + " " + displayF32(pt[0].Y) + " " +
				displayF32(pt[1].X) + " " + displayF32(pt[1].Y) + " " +
				displayF32(pt[2].X) + " " + displayF32(pt[2].Y) + " ")
		case PathSegmentClose:
			b.WriteString("Z ")
		}
	}
	segments := strings.TrimSuffix(b.String(), " ")
	return "Path { segments: " + strconv.Quote(segments) + ", bounds: " + p.bounds.String() + " }"
}

// displayF32 writes v as the Display impl of f32 in Rust does: the shortest
// decimal that gives back v, with no exponent.
func displayF32(v float32) string {
	switch {
	case v != v:
		return "NaN"
	case math.IsInf(float64(v), 1):
		return "inf"
	case math.IsInf(float64(v), -1):
		return "-inf"
	}
	return strconv.FormatFloat(float64(v), 'f', -1, 32)
}

// debugF32 writes v as the Debug impl of f32 in Rust does. It is displayF32
// with ".0" after an integer. A value whose magnitude is less than 1e-4 or at
// least 1e16 has an exponent, such as "1e-7" or "1.5e16".
func debugF32(v float32) string {
	a := math.Abs(float64(v))
	if v != v || math.IsInf(a, 0) {
		return displayF32(v)
	}
	if a != 0 && (a < 1e-4 || a >= 1e16) {
		s := strconv.FormatFloat(float64(v), 'e', -1, 32)
		mant, exp, _ := strings.Cut(s, "e")
		neg := strings.HasPrefix(exp, "-")
		exp = strings.TrimLeft(exp, "+-")
		exp = strings.TrimLeft(exp, "0")
		if neg {
			exp = "-" + exp
		}
		return mant + "e" + exp
	}
	s := displayF32(v)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// ComputeTightBounds is Path::compute_tight_bounds in tiny-skia-path
// path/src/path.rs. It computes the tight bounds of the path.
//
// This operation can be expensive.
func (p *Path) ComputeTightBounds() (Rect, bool) {
	// big enough to hold worst-case curve type (cubic) extremas + 1
	var extremas [5]Point

	min := p.points[0]
	max := p.points[0]
	iter := p.Segments()
	lastPoint := PointZero()
	for {
		segment, ok := iter.Next()
		if !ok {
			break
		}
		count := 0
		switch segment.Kind {
		case PathSegmentMoveTo:
			extremas[0] = segment.Points[0]
			count = 1
		case PathSegmentLineTo:
			extremas[0] = segment.Points[0]
			count = 1
		case PathSegmentQuadTo:
			count = computeQuadExtremas(lastPoint, segment.Points[0], segment.Points[1], &extremas)
		case PathSegmentCubicTo:
			count = computeCubicExtremas(lastPoint, segment.Points[0], segment.Points[1], segment.Points[2], &extremas)
		case PathSegmentClose:
		}

		lastPoint = iter.lastPoint
		for _, tmp := range extremas[0:count] {
			min.X = minF32(min.X, tmp.X)
			min.Y = minF32(min.Y, tmp.Y)
			max.X = maxF32(max.X, tmp.X)
			max.Y = maxF32(max.Y, tmp.Y)
		}
	}

	return RectFromLTRB(min.X, min.Y, max.X, max.Y)
}

// Verbs is Path::verbs in tiny-skia-path path/src/path.rs. It returns the
// internal slice of verbs.
func (p *Path) Verbs() []PathVerb {
	return p.verbs
}

// Points is Path::points in tiny-skia-path path/src/path.rs. It returns the
// internal slice of points.
func (p *Path) Points() []Point {
	return p.points
}

// Transform is Path::transform in tiny-skia-path path/src/path.rs. It
// transforms the path in place and returns it.
//
// Some points can become NaN or infinity, so this method can fail.
//
// Port: Rust consumes the path. Go changes p in place and returns p. When it
// returns false, p holds the transformed points and the old bounds, and it is
// not valid.
func (p *Path) Transform(ts Transform) (*Path, bool) {
	if ts.IsIdentity() {
		return p, true
	}

	ts.MapPoints(p.points)

	// Update bounds.
	bounds, ok := RectFromPoints(p.points)
	if !ok {
		return nil, false
	}
	p.bounds = bounds

	return p, true
}

// Segments is Path::segments in tiny-skia-path path/src/path.rs. It returns an
// iterator over the segments of the path.
func (p *Path) Segments() PathSegmentsIter {
	return PathSegmentsIter{
		path:        p,
		verbIndex:   0,
		pointsIndex: 0,
		isAutoClose: false,
		lastMoveTo:  PointZero(),
		lastPoint:   PointZero(),
	}
}

// Clear is Path::clear in tiny-skia-path path/src/path.rs. It clears the path
// and returns a PathBuilder that reuses the allocated memory.
//
// Port: Rust consumes the path. Do not use p after Clear.
func (p *Path) Clear() *PathBuilder {
	p.verbs = p.verbs[:0]
	p.points = p.points[:0]

	return &PathBuilder{
		verbs:           p.verbs,
		points:          p.points,
		lastMoveToIndex: 0,
		moveToRequired:  true,
	}
}

// Clone is the derived Clone impl of Path in tiny-skia-path path/src/path.rs.
// It returns a deep copy of the path.
func (p *Path) Clone() *Path {
	return &Path{
		verbs:  slices.Clone(p.verbs),
		points: slices.Clone(p.points),
		bounds: p.bounds,
	}
}

// computeQuadExtremas is compute_quad_extremas in tiny-skia-path
// path/src/path.rs.
func computeQuadExtremas(p0, p1, p2 Point, extremas *[5]Point) int {
	src := [3]Point{p0, p1, p2}
	extremaIdx := 0
	if t, ok := findQuadExtrema(p0.X, p1.X, p2.X); ok {
		extremas[extremaIdx] = evalQuadAt(&src, t.ToNormalized())
		extremaIdx++
	}
	if t, ok := findQuadExtrema(p0.Y, p1.Y, p2.Y); ok {
		extremas[extremaIdx] = evalQuadAt(&src, t.ToNormalized())
		extremaIdx++
	}
	extremas[extremaIdx] = p2
	return extremaIdx + 1
}

// computeCubicExtremas is compute_cubic_extremas in tiny-skia-path
// path/src/path.rs.
func computeCubicExtremas(p0, p1, p2, p3 Point, extremas *[5]Point) int {
	ts0 := NewTValues()
	ts1 := NewTValues()
	n0 := findCubicExtrema(p0.X, p1.X, p2.X, p3.X, &ts0)
	n1 := findCubicExtrema(p0.Y, p1.Y, p2.Y, p3.Y, &ts1)
	totalLen := n0 + n1

	src := [4]Point{p0, p1, p2, p3}
	extremaIdx := 0
	for _, t := range ts0[0:n0] {
		extremas[extremaIdx] = evalCubicPosAt(&src, t.ToNormalized())
		extremaIdx++
	}
	for _, t := range ts1[0:n1] {
		extremas[extremaIdx] = evalCubicPosAt(&src, t.ToNormalized())
		extremaIdx++
	}
	extremas[totalLen] = p3
	return totalLen + 1
}

// PathSegmentKind is the kind of a PathSegment. It is the variant of the enum
// PathSegment in tiny-skia-path path/src/path.rs.
type PathSegmentKind uint8

// The values of PathSegmentKind.
const (
	PathSegmentMoveTo PathSegmentKind = iota
	PathSegmentLineTo
	PathSegmentQuadTo
	PathSegmentCubicTo
	PathSegmentClose
)

// String satisfies the fmt.Stringer interface.
func (k PathSegmentKind) String() string {
	switch k {
	case PathSegmentMoveTo:
		return "MoveTo"
	case PathSegmentLineTo:
		return "LineTo"
	case PathSegmentQuadTo:
		return "QuadTo"
	case PathSegmentCubicTo:
		return "CubicTo"
	case PathSegmentClose:
		return "Close"
	}
	return "PathSegmentKind(?)"
}

// PathSegment is PathSegment in tiny-skia-path path/src/path.rs. It is a path
// segment.
//
// Port: the Rust enum carries data, so it becomes a struct with a kind field.
// Points holds the points of the variant in their order: one for MoveTo and
// LineTo, two for QuadTo, three for CubicTo and none for Close. The other
// points are zero, so that two segments compare equal with ==.
type PathSegment struct {
	Kind   PathSegmentKind
	Points [3]Point
}

// PathSegmentsIter is PathSegmentsIter in tiny-skia-path path/src/path.rs. It
// is an iterator over the segments of a path.
type PathSegmentsIter struct {
	path        *Path
	verbIndex   int
	pointsIndex int

	isAutoClose bool
	lastMoveTo  Point
	lastPoint   Point
}

// SetAutoClose is PathSegmentsIter::set_auto_close in tiny-skia-path
// path/src/path.rs. It sets the auto closing mode. It is off by default.
//
// When it is on, the iterator returns an extra LineTo segment from the current
// position to the previous MoveTo, and only then a Close segment.
func (it *PathSegmentsIter) SetAutoClose(flag bool) {
	it.isAutoClose = flag
}

// autoClose is PathSegmentsIter::auto_close in tiny-skia-path
// path/src/path.rs.
func (it *PathSegmentsIter) autoClose() PathSegment {
	if it.isAutoClose && it.lastPoint != it.lastMoveTo {
		it.verbIndex--
		return PathSegment{Kind: PathSegmentLineTo, Points: [3]Point{it.lastMoveTo}}
	}
	return PathSegment{Kind: PathSegmentClose}
}

// hasValidTangent is PathSegmentsIter::has_valid_tangent in tiny-skia-path
// path/src/path.rs.
func (it *PathSegmentsIter) hasValidTangent() bool {
	iter := *it
	for {
		segment, ok := iter.Next()
		if !ok {
			break
		}
		switch segment.Kind {
		case PathSegmentMoveTo:
			return false
		case PathSegmentLineTo:
			if iter.lastPoint == segment.Points[0] {
				continue
			}

			return true
		case PathSegmentQuadTo:
			if iter.lastPoint == segment.Points[0] && iter.lastPoint == segment.Points[1] { //nolint:gocritic // upstream compares each point with the last point
				continue
			}

			return true
		case PathSegmentCubicTo:
			if iter.lastPoint == segment.Points[0] && //nolint:gocritic // upstream compares each point with the last point
				iter.lastPoint == segment.Points[1] &&
				iter.lastPoint == segment.Points[2] {
				continue
			}

			return true
		case PathSegmentClose:
			return false
		}
	}

	return false
}

// CurrVerb is PathSegmentsIter::curr_verb in tiny-skia-path path/src/path.rs.
// It returns the current verb.
func (it *PathSegmentsIter) CurrVerb() PathVerb {
	return it.path.verbs[it.verbIndex-1]
}

// NextVerb is PathSegmentsIter::next_verb in tiny-skia-path path/src/path.rs.
// It returns the next verb.
func (it *PathSegmentsIter) NextVerb() (PathVerb, bool) {
	if it.verbIndex < len(it.path.verbs) {
		return it.path.verbs[it.verbIndex], true
	}
	return 0, false
}

// Next is Iterator::next of PathSegmentsIter in tiny-skia-path
// path/src/path.rs. It returns the next segment, or false at the end.
func (it *PathSegmentsIter) Next() (PathSegment, bool) {
	if it.verbIndex < len(it.path.verbs) {
		verb := it.path.verbs[it.verbIndex]
		it.verbIndex++

		switch verb {
		case PathVerbMove:
			it.pointsIndex++
			it.lastMoveTo = it.path.points[it.pointsIndex-1]
			it.lastPoint = it.lastMoveTo
			return PathSegment{Kind: PathSegmentMoveTo, Points: [3]Point{it.lastMoveTo}}, true
		case PathVerbLine:
			it.pointsIndex++
			it.lastPoint = it.path.points[it.pointsIndex-1]
			return PathSegment{Kind: PathSegmentLineTo, Points: [3]Point{it.lastPoint}}, true
		case PathVerbQuad:
			it.pointsIndex += 2
			it.lastPoint = it.path.points[it.pointsIndex-1]
			return PathSegment{Kind: PathSegmentQuadTo, Points: [3]Point{
				it.path.points[it.pointsIndex-2],
				it.lastPoint,
			}}, true
		case PathVerbCubic:
			it.pointsIndex += 3
			it.lastPoint = it.path.points[it.pointsIndex-1]
			return PathSegment{Kind: PathSegmentCubicTo, Points: [3]Point{
				it.path.points[it.pointsIndex-3],
				it.path.points[it.pointsIndex-2],
				it.lastPoint,
			}}, true
		case PathVerbClose:
			seg := it.autoClose()
			it.lastPoint = it.lastMoveTo
			return seg, true
		}
	}
	return PathSegment{}, false
}

// All returns an iterator over the remaining segments, for use with range. It
// iterates over a copy of it, so it does not change it.
//
// Port: it ports no upstream function. It is the Go form of the Rust Iterator
// impl of PathSegmentsIter.
func (it PathSegmentsIter) All() iter.Seq[PathSegment] {
	return func(yield func(PathSegment) bool) {
		for {
			seg, ok := it.Next()
			if !ok || !yield(seg) {
				return
			}
		}
	}
}
