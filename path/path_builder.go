// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import "slices"

// NOTE: this is not SkPathBuilder, but rather a reimplementation of SkPath.

// pathDirection is PathDirection in tiny-skia-path path/src/path_builder.rs.
type pathDirection uint8

const (
	// pathDirectionCW is the clockwise direction for adding closed contours.
	pathDirectionCW pathDirection = iota
	// pathDirectionCCW is the counter-clockwise direction for adding closed
	// contours.
	pathDirectionCCW
)

// PathBuilder is PathBuilder in tiny-skia-path path/src/path_builder.rs. It is
// a path builder.
//
// The zero value is the Default impl of upstream. It differs from
// NewPathBuilder: it does not require a MoveTo first, so a LineTo on an empty
// zero value adds no Move(0, 0).
type PathBuilder struct {
	verbs           []PathVerb
	points          []Point
	lastMoveToIndex int
	moveToRequired  bool
}

// NewPathBuilder is PathBuilder::new in tiny-skia-path
// path/src/path_builder.rs. It creates a new builder.
func NewPathBuilder() *PathBuilder {
	return &PathBuilder{
		verbs:           nil,
		points:          nil,
		lastMoveToIndex: 0,
		moveToRequired:  true,
	}
}

// NewPathBuilderWithCapacity is PathBuilder::with_capacity in tiny-skia-path
// path/src/path_builder.rs. It creates a new builder with the given capacity.
//
// The number of points depends on the type of the verb:
//   - Move: 1
//   - Line: 1
//   - Quad: 2
//   - Cubic: 3
//   - Close: 0
func NewPathBuilderWithCapacity(verbsCapacity, pointsCapacity int) *PathBuilder {
	return &PathBuilder{
		verbs:           make([]PathVerb, 0, verbsCapacity),
		points:          make([]Point, 0, pointsCapacity),
		lastMoveToIndex: 0,
		moveToRequired:  true,
	}
}

// PathFromRect is PathBuilder::from_rect in tiny-skia-path
// path/src/path_builder.rs. It creates a new Path from a Rect.
//
// It never fails, because a Rect is always valid.
//
// It creates the segments clockwise: TopLeft -> TopRight -> BottomRight ->
// BottomLeft.
//
// The contour is closed.
func PathFromRect(rect Rect) *Path {
	verbs := []PathVerb{
		PathVerbMove,
		PathVerbLine,
		PathVerbLine,
		PathVerbLine,
		PathVerbClose,
	}

	points := []Point{
		PointFromXY(rect.Left(), rect.Top()),
		PointFromXY(rect.Right(), rect.Top()),
		PointFromXY(rect.Right(), rect.Bottom()),
		PointFromXY(rect.Left(), rect.Bottom()),
	}

	return &Path{
		bounds: rect,
		verbs:  verbs,
		points: points,
	}
}

// PathFromCircle is PathBuilder::from_circle in tiny-skia-path
// path/src/path_builder.rs. It creates a new Path from a circle.
//
// See PathBuilder.PushCircle for details.
func PathFromCircle(cx, cy, radius float32) (*Path, bool) {
	b := NewPathBuilder()
	b.PushCircle(cx, cy, radius)
	return b.Finish()
}

// PathFromOval is PathBuilder::from_oval in tiny-skia-path
// path/src/path_builder.rs. It creates a new Path from an oval.
//
// See PathBuilder.PushOval for details.
func PathFromOval(oval Rect) (*Path, bool) {
	b := NewPathBuilder()
	b.PushOval(oval)
	return b.Finish()
}

// reserve is PathBuilder::reserve in tiny-skia-path path/src/path_builder.rs.
func (pb *PathBuilder) reserve(additionalVerbs, additionalPoints int) {
	pb.verbs = slices.Grow(pb.verbs, additionalVerbs)
	pb.points = slices.Grow(pb.points, additionalPoints)
}

// Len is PathBuilder::len in tiny-skia-path path/src/path_builder.rs. It
// returns the current number of segments in the builder.
func (pb *PathBuilder) Len() int {
	return len(pb.verbs)
}

// IsEmpty is PathBuilder::is_empty in tiny-skia-path path/src/path_builder.rs.
// It checks if the builder has no segments.
func (pb *PathBuilder) IsEmpty() bool {
	return len(pb.verbs) == 0
}

// MoveTo is PathBuilder::move_to in tiny-skia-path path/src/path_builder.rs.
// It adds the start of a contour.
//
// A MoveTo segment cannot follow a MoveTo segment. If the previous segment
// was also a MoveTo, the current one overwrites it.
func (pb *PathBuilder) MoveTo(x, y float32) {
	if n := len(pb.verbs); n > 0 && pb.verbs[n-1] == PathVerbMove {
		lastIdx := len(pb.points) - 1
		pb.points[lastIdx] = PointFromXY(x, y)
	} else {
		pb.lastMoveToIndex = len(pb.points)
		pb.moveToRequired = false

		pb.verbs = append(pb.verbs, PathVerbMove)
		pb.points = append(pb.points, PointFromXY(x, y))
	}
}

// injectMoveToIfNeeded is PathBuilder::inject_move_to_if_needed in
// tiny-skia-path path/src/path_builder.rs.
func (pb *PathBuilder) injectMoveToIfNeeded() {
	if pb.moveToRequired {
		if pb.lastMoveToIndex < len(pb.points) {
			p := pb.points[pb.lastMoveToIndex]
			pb.MoveTo(p.X, p.Y)
		} else {
			pb.MoveTo(0.0, 0.0)
		}
	}
}

// LineTo is PathBuilder::line_to in tiny-skia-path path/src/path_builder.rs.
// It adds a line from the last point.
//
//   - If the Path is empty, it adds Move(0, 0) first.
//   - If the Path ends with Close, it adds Move(last_x, last_y) first.
func (pb *PathBuilder) LineTo(x, y float32) {
	pb.injectMoveToIfNeeded()

	pb.verbs = append(pb.verbs, PathVerbLine)
	pb.points = append(pb.points, PointFromXY(x, y))
}

// QuadTo is PathBuilder::quad_to in tiny-skia-path path/src/path_builder.rs.
// It adds a quad curve from the last point to x, y.
//
//   - If the Path is empty, it adds Move(0, 0) first.
//   - If the Path ends with Close, it adds Move(last_x, last_y) first.
func (pb *PathBuilder) QuadTo(x1, y1, x, y float32) {
	pb.injectMoveToIfNeeded()

	pb.verbs = append(pb.verbs, PathVerbQuad)
	pb.points = append(pb.points, PointFromXY(x1, y1))
	pb.points = append(pb.points, PointFromXY(x, y))
}

// quadToPt is PathBuilder::quad_to_pt in tiny-skia-path
// path/src/path_builder.rs.
func (pb *PathBuilder) quadToPt(p1, p Point) {
	pb.QuadTo(p1.X, p1.Y, p.X, p.Y)
}

// conicTo is PathBuilder::conic_to in tiny-skia-path
// path/src/path_builder.rs.
//
// We do not support conic segments, but Skia still relies on them from time to time.
// This method will simply convert the input data into quad segments.
func (pb *PathBuilder) conicTo(x1, y1, x, y, weight float32) {
	// check for <= 0 or NaN with this test
	if !(weight > 0.0) {
		pb.LineTo(x, y)
	} else if !isFinite(weight) {
		pb.LineTo(x1, y1)
		pb.LineTo(x, y)
	} else if weight == 1.0 {
		pb.QuadTo(x1, y1, x, y)
	} else {
		pb.injectMoveToIfNeeded()

		last, ok := pb.LastPoint()
		if !ok {
			panic("path: PathBuilder::conic_to: last_point returned None")
		}
		quadder, ok := computeAutoConicToQuads(
			last,
			PointFromXY(x1, y1),
			PointFromXY(x, y),
			weight,
		)
		if ok {
			// Points are ordered as: 0 - 1 2 - 3 4 - 5 6 - ..
			// `count` is a number of pairs +1
			offset := 1
			for range quadder.len {
				pt1 := quadder.points[offset+0]
				pt2 := quadder.points[offset+1]
				pb.QuadTo(pt1.X, pt1.Y, pt2.X, pt2.Y)
				offset += 2
			}
		}
	}
}

// conicPointsTo is PathBuilder::conic_points_to in tiny-skia-path
// path/src/path_builder.rs.
func (pb *PathBuilder) conicPointsTo(pt1, pt2 Point, weight float32) {
	pb.conicTo(pt1.X, pt1.Y, pt2.X, pt2.Y, weight)
}

// CubicTo is PathBuilder::cubic_to in tiny-skia-path path/src/path_builder.rs.
// It adds a cubic curve from the last point to x, y.
//
//   - If the Path is empty, it adds Move(0, 0) first.
//   - If the Path ends with Close, it adds Move(last_x, last_y) first.
func (pb *PathBuilder) CubicTo(x1, y1, x2, y2, x, y float32) {
	pb.injectMoveToIfNeeded()

	pb.verbs = append(pb.verbs, PathVerbCubic)
	pb.points = append(pb.points, PointFromXY(x1, y1))
	pb.points = append(pb.points, PointFromXY(x2, y2))
	pb.points = append(pb.points, PointFromXY(x, y))
}

// cubicToPt is PathBuilder::cubic_to_pt in tiny-skia-path
// path/src/path_builder.rs.
func (pb *PathBuilder) cubicToPt(p1, p2, p Point) {
	pb.CubicTo(p1.X, p1.Y, p2.X, p2.Y, p.X, p.Y)
}

// Close is PathBuilder::close in tiny-skia-path path/src/path_builder.rs. It
// closes the current contour.
//
// A closed contour connects the first and the last point with a line, and so
// forms a continuous loop.
//
// It does nothing when the Path is empty or already closed.
//
// An open contour and a closed contour are filled the same way. A stroke of an
// open contour adds a LineCap at the start and the end of the contour. A
// stroke of a closed contour adds a LineJoin at the start and the end of the
// contour.
func (pb *PathBuilder) Close() {
	// don't add a close if it's the first verb or a repeat
	if len(pb.verbs) != 0 {
		if pb.verbs[len(pb.verbs)-1] != PathVerbClose {
			pb.verbs = append(pb.verbs, PathVerbClose)
		}
	}

	pb.moveToRequired = true
}

// LastPoint is PathBuilder::last_point in tiny-skia-path
// path/src/path_builder.rs. It returns the last point, if there is one.
func (pb *PathBuilder) LastPoint() (Point, bool) {
	if len(pb.points) == 0 {
		return Point{}, false
	}
	return pb.points[len(pb.points)-1], true
}

// setLastPoint is PathBuilder::set_last_point in tiny-skia-path
// path/src/path_builder.rs.
func (pb *PathBuilder) setLastPoint(pt Point) {
	if len(pb.points) != 0 {
		pb.points[len(pb.points)-1] = pt
	} else {
		pb.MoveTo(pt.X, pt.Y)
	}
}

// isZeroLengthSincePoint is PathBuilder::is_zero_length_since_point in
// tiny-skia-path path/src/path_builder.rs.
func (pb *PathBuilder) isZeroLengthSincePoint(startPtIndex int) bool {
	count := len(pb.points) - startPtIndex
	// Port: count is a usize in Rust. If startPtIndex is past the end, it
	// wraps to a large value, and the index below panics. A negative count
	// in Go reaches the same index and the same panic.
	if count >= 0 && count < 2 {
		return true
	}

	first := pb.points[startPtIndex]
	for i := 1; i < count; i++ {
		if first != pb.points[startPtIndex+i] {
			return false
		}
	}

	return true
}

// PushRect is PathBuilder::push_rect in tiny-skia-path
// path/src/path_builder.rs. It adds a rectangle contour.
//
// The contour is closed and has a clockwise direction.
func (pb *PathBuilder) PushRect(rect Rect) {
	pb.MoveTo(rect.Left(), rect.Top())
	pb.LineTo(rect.Right(), rect.Top())
	pb.LineTo(rect.Right(), rect.Bottom())
	pb.LineTo(rect.Left(), rect.Bottom())
	pb.Close()
}

// PushOval is PathBuilder::push_oval in tiny-skia-path
// path/src/path_builder.rs. It adds an oval contour bounded by the given
// rectangle.
//
// The contour is closed and has a clockwise direction.
func (pb *PathBuilder) PushOval(oval Rect) {
	cx := Half(oval.Left()) + Half(oval.Right())
	cy := Half(oval.Top()) + Half(oval.Bottom())

	ovalPoints := [4]Point{
		PointFromXY(cx, oval.Bottom()),
		PointFromXY(oval.Left(), cy),
		PointFromXY(cx, oval.Top()),
		PointFromXY(oval.Right(), cy),
	}

	rectPoints := [4]Point{
		PointFromXY(oval.Right(), oval.Bottom()),
		PointFromXY(oval.Left(), oval.Bottom()),
		PointFromXY(oval.Left(), oval.Top()),
		PointFromXY(oval.Right(), oval.Top()),
	}

	weight := ScalarRoot2Over2
	pb.MoveTo(ovalPoints[3].X, ovalPoints[3].Y)
	for i := range rectPoints {
		pb.conicPointsTo(rectPoints[i], ovalPoints[i], weight)
	}
	pb.Close()
}

// PushCircle is PathBuilder::push_circle in tiny-skia-path
// path/src/path_builder.rs. It adds a circle contour.
//
// The contour is closed and has a clockwise direction.
//
// It does nothing when:
//   - radius <= 0
//   - a value is not finite or is very large
func (pb *PathBuilder) PushCircle(x, y, r float32) {
	if rect, ok := RectFromXYWH(x-r, y-r, r+r, r+r); ok {
		pb.PushOval(rect)
	}
}

// PushPath is PathBuilder::push_path in tiny-skia-path
// path/src/path_builder.rs. It adds a path.
func (pb *PathBuilder) PushPath(other *Path) {
	pb.lastMoveToIndex = len(pb.points)

	pb.verbs = append(pb.verbs, other.verbs...)
	pb.points = append(pb.points, other.points...)
}

// pushPathBuilder is PathBuilder::push_path_builder in tiny-skia-path
// path/src/path_builder.rs.
func (pb *PathBuilder) pushPathBuilder(other *PathBuilder) {
	if other.IsEmpty() {
		return
	}

	if pb.lastMoveToIndex != 0 {
		pb.lastMoveToIndex = len(pb.points) + other.lastMoveToIndex
	}

	pb.verbs = append(pb.verbs, other.verbs...)
	pb.points = append(pb.points, other.points...)
}

// reversePathTo is PathBuilder::reverse_path_to in tiny-skia-path
// path/src/path_builder.rs.
//
// Appends, in a reverse order, the first contour of path ignoring path's last point.
func (pb *PathBuilder) reversePathTo(other *PathBuilder) {
	if other.IsEmpty() {
		return
	}

	pointsOffset := len(other.points) - 1
	for i := len(other.verbs) - 1; i >= 0; i-- { //nolint:modernize // the loop of upstream is kept
		switch other.verbs[i] {
		case PathVerbMove:
			// if the path has multiple contours, stop after reversing the last
			return
		case PathVerbLine:
			// We're moving one point back manually, to prevent points_offset overflow.
			pt := other.points[pointsOffset-1]
			pointsOffset--
			pb.LineTo(pt.X, pt.Y)
		case PathVerbQuad:
			pt1 := other.points[pointsOffset-1]
			pt2 := other.points[pointsOffset-2]
			pointsOffset -= 2
			pb.QuadTo(pt1.X, pt1.Y, pt2.X, pt2.Y)
		case PathVerbCubic:
			pt1 := other.points[pointsOffset-1]
			pt2 := other.points[pointsOffset-2]
			pt3 := other.points[pointsOffset-3]
			pointsOffset -= 3
			pb.CubicTo(pt1.X, pt1.Y, pt2.X, pt2.Y, pt3.X, pt3.Y)
		case PathVerbClose:
		}
	}
}

// Clear is PathBuilder::clear in tiny-skia-path path/src/path_builder.rs. It
// resets the builder.
//
// It keeps the allocated memory.
func (pb *PathBuilder) Clear() {
	pb.verbs = pb.verbs[:0]
	pb.points = pb.points[:0]
	pb.lastMoveToIndex = 0
	pb.moveToRequired = true
}

// Finish is PathBuilder::finish in tiny-skia-path path/src/path_builder.rs. It
// finishes the builder and returns a Path.
//
// It returns false when the Path is empty or has invalid bounds.
//
// Port: Rust consumes the builder. Go moves the memory of pb into the Path
// and leaves pb as a zero value, in each case.
func (pb *PathBuilder) Finish() (*Path, bool) {
	b := *pb
	*pb = PathBuilder{}

	if b.IsEmpty() {
		return nil, false
	}

	// Just a move to? Bail.
	if len(b.verbs) == 1 {
		return nil, false
	}

	bounds, ok := RectFromPoints(b.points)
	if !ok {
		return nil, false
	}

	return &Path{
		bounds: bounds,
		verbs:  b.verbs,
		points: b.points,
	}, true
}

// Clone is the derived Clone impl of PathBuilder in tiny-skia-path
// path/src/path_builder.rs. It returns a deep copy of the builder.
func (pb *PathBuilder) Clone() *PathBuilder {
	return &PathBuilder{
		verbs:           slices.Clone(pb.verbs),
		points:          slices.Clone(pb.points),
		lastMoveToIndex: pb.lastMoveToIndex,
		moveToRequired:  pb.moveToRequired,
	}
}
