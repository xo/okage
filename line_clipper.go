// Copyright 2011 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import "github.com/xo/okage/path"

// maxPoints is MAX_POINTS in tiny-skia src/line_clipper.rs.
const maxPoints = 4

// clip is clip in tiny-skia src/line_clipper.rs.
//
// Clip the line pts[0]...pts[1] against clip, ignoring segments that lie
// completely above or below the clip. For portions to the left or right, turn
// those into vertical line segments that are aligned to the edge of the clip.
//
// Return the number of line segments that result, and store the end-points of
// those segments sequentially in lines as follows:
//
//	1st segment: lines[0]..lines[1]
//	2nd segment: lines[1]..lines[2]
//	3rd segment: lines[2]..lines[3]
func clip(src *[2]Point, clip *Rect, canCullToTheRight bool, points *[maxPoints]Point) []Point {
	index0, index1 := 1, 0
	if src[0].Y < src[1].Y {
		index0, index1 = 0, 1
	}

	// Check if we are completely clipped out in Y (above or below)

	if src[index1].Y <= clip.Top() {
		// we are above the clip
		return nil
	}

	if src[index0].Y >= clip.Bottom() {
		// we are below the clip
		return nil
	}

	// Chop in Y to produce a single segment, stored in tmp[0..1]

	tmp := *src

	// now compute intersections
	if src[index0].Y < clip.Top() {
		tmp[index0] = path.PointFromXY(sectWithHorizontal(src, clip.Top()), clip.Top())
	}

	if tmp[index1].Y > clip.Bottom() {
		tmp[index1] = path.PointFromXY(sectWithHorizontal(src, clip.Bottom()), clip.Bottom())
	}

	// Chop it into 1..3 segments that are wholly within the clip in X.

	// temp storage for up to 3 segments
	var resultStorage [maxPoints]Point
	lineCount := 1
	var reverse bool

	if src[0].X < src[1].X {
		index0 = 0
		index1 = 1
		reverse = false
	} else {
		index0 = 1
		index1 = 0
		reverse = true
	}

	var result []Point
	switch {
	case tmp[index1].X <= clip.Left():
		// wholly to the left
		tmp[0].X = clip.Left()
		tmp[1].X = clip.Left()
		reverse = false
		result = tmp[:]
	case tmp[index0].X >= clip.Right():
		// wholly to the right
		if canCullToTheRight {
			return nil
		}

		tmp[0].X = clip.Right()
		tmp[1].X = clip.Right()
		reverse = false
		result = tmp[:]
	default:
		offset := 0

		if tmp[index0].X < clip.Left() {
			resultStorage[offset] = path.PointFromXY(clip.Left(), tmp[index0].Y)
			offset++
			resultStorage[offset] = path.PointFromXY(clip.Left(), sectClampWithVertical(&tmp, clip.Left()))
		} else {
			resultStorage[offset] = tmp[index0]
		}
		offset++

		if tmp[index1].X > clip.Right() {
			resultStorage[offset] = path.PointFromXY(clip.Right(), sectClampWithVertical(&tmp, clip.Right()))
			offset++
			resultStorage[offset] = path.PointFromXY(clip.Right(), tmp[index1].Y)
		} else {
			resultStorage[offset] = tmp[index1]
		}

		lineCount = offset
		result = resultStorage[:]
	}

	// Now copy the results into the caller's lines[] parameter
	if reverse {
		// copy the pts in reverse order to maintain winding order
		for i := 0; i <= lineCount; i++ {
			points[lineCount-i] = result[i]
		}
	} else {
		n := lineCount + 1
		copy(points[0:n], result[0:n])
	}

	return points[0 : lineCount+1]
}

// sectWithHorizontal is sect_with_horizontal in tiny-skia
// src/line_clipper.rs. It returns the X coordinate of the intersection with a
// horizontal line at Y.
func sectWithHorizontal(src *[2]Point, y float32) float32 {
	dy := src[1].Y - src[0].Y
	if path.IsNearlyZero(dy) {
		return path.Ave(src[0].X, src[1].X)
	}

	// need the extra precision so we do not compute a value that exceeds
	// our original limits
	x0 := float64(src[0].X)
	y0 := float64(src[0].Y)
	x1 := float64(src[1].X)
	y1 := float64(src[1].Y)
	result := x0 + float64((float64(y)-y0)*(x1-x0))/(y1-y0)

	// The computed X value can still exceed [X0..X1] due to quantum flux
	// when the doubles were added and subtracted, so we have to pin the
	// answer :(
	return float32(pinUnsortedF64(result, x0, x1))
}

// isBetweenUnsorted is is_between_unsorted in tiny-skia src/line_clipper.rs.
// It returns true if value is between the two limits, where the limits are
// either ascending or descending.
//
// Port: upstream calls it only in debug_assert!, which okage does not port.
// It is kept for the order of the file.
//
//nolint:unused // upstream calls it only in a debug_assert
func isBetweenUnsorted(value, limit0, limit1 float32) bool {
	if limit0 < limit1 {
		return limit0 <= value && value <= limit1
	}
	return limit1 <= value && value <= limit0
}

// sectClampWithVertical is sect_clamp_with_vertical in tiny-skia
// src/line_clipper.rs.
func sectClampWithVertical(src *[2]Point, x float32) float32 {
	y := sectWithVertical(src, x)
	// Our caller expects y to be between src[0].y and src[1].y (unsorted), but due to the
	// numerics of floats/doubles, we can compute a value slightly outside of that,
	// so we have to manually clamp afterwards.
	// See skbug.com/7491
	return pinUnsortedF32(y, src[0].Y, src[1].Y)
}

// sectWithVertical is sect_with_vertical in tiny-skia src/line_clipper.rs. It
// returns the Y coordinate of the intersection with a vertical line at X.
func sectWithVertical(src *[2]Point, x float32) float32 {
	dx := src[1].X - src[0].X
	if path.IsNearlyZero(dx) {
		return path.Ave(src[0].Y, src[1].Y)
	}

	// need the extra precision so we do not compute a value that exceeds
	// our original limits
	x0 := float64(src[0].X)
	y0 := float64(src[0].Y)
	x1 := float64(src[1].X)
	y1 := float64(src[1].Y)
	result := y0 + float64((float64(x)-x0)*(y1-y0))/(x1-x0)
	return float32(result)
}

// pinUnsortedF32 is pin_unsorted_f32 in tiny-skia src/line_clipper.rs.
func pinUnsortedF32(value, limit0, limit1 float32) float32 {
	if limit1 < limit0 {
		limit0, limit1 = limit1, limit0
	}
	// now the limits are sorted

	if value < limit0 {
		return limit0
	} else if value > limit1 {
		return limit1
	}
	return value
}

// pinUnsortedF64 is pin_unsorted_f64 in tiny-skia src/line_clipper.rs.
func pinUnsortedF64(value, limit0, limit1 float64) float64 {
	if limit1 < limit0 {
		limit0, limit1 = limit1, limit0
	}
	// now the limits are sorted

	if value < limit0 {
		return limit0
	} else if value > limit1 {
		return limit1
	}
	return value
}

// intersect is intersect in tiny-skia src/line_clipper.rs.
//
// Intersect the line segment against the rect. If there is a non-empty
// resulting segment, return true and set dst[] to that segment. If not,
// return false and ignore dst[].
//
// `clip` is specialized for scan-conversion, as it adds vertical segments on
// the sides to show where the line extended beyond the left or right sides.
// `intersect` does not.
func intersect(src *[2]Point, clip *Rect, dst *[2]Point) bool {
	bounds, ok := path.RectFromLTRB(
		minF32(src[0].X, src[1].X),
		minF32(src[0].Y, src[1].Y),
		maxF32(src[0].X, src[1].X),
		maxF32(src[0].Y, src[1].Y),
	)

	if ok {
		if containsNoEmptyCheck(clip, &bounds) {
			*dst = *src
			return true
		}

		// check for no overlap, and only permit coincident edges if the line
		// and the edge are collinear
		if nestedLT(bounds.Right(), clip.Left(), bounds.Width()) ||
			nestedLT(clip.Right(), bounds.Left(), bounds.Width()) ||
			nestedLT(bounds.Bottom(), clip.Top(), bounds.Height()) ||
			nestedLT(clip.Bottom(), bounds.Top(), bounds.Height()) {
			return false
		}
	}

	index0, index1 := 1, 0
	if src[0].Y < src[1].Y {
		index0, index1 = 0, 1
	}

	tmp := *src

	// now compute Y intersections
	if tmp[index0].Y < clip.Top() {
		tmp[index0] = path.PointFromXY(sectWithHorizontal(src, clip.Top()), clip.Top())
	}

	if tmp[index1].Y > clip.Bottom() {
		tmp[index1] = path.PointFromXY(sectWithHorizontal(src, clip.Bottom()), clip.Bottom())
	}

	index0, index1 = 1, 0
	if tmp[0].X < tmp[1].X {
		index0, index1 = 0, 1
	}

	// check for quick-reject in X again, now that we can have been chopped
	if tmp[index1].X <= clip.Left() || tmp[index0].X >= clip.Right() {
		// usually we return false, but we do not if the line is vertical and coincident
		// with the clip.
		if tmp[0].X != tmp[1].X || tmp[0].X < clip.Left() || tmp[0].X > clip.Right() {
			return false
		}
	}

	if tmp[index0].X < clip.Left() {
		tmp[index0] = path.PointFromXY(clip.Left(), sectWithVertical(src, clip.Left()))
	}

	if tmp[index1].X > clip.Right() {
		tmp[index1] = path.PointFromXY(clip.Right(), sectWithVertical(src, clip.Right()))
	}

	*dst = tmp
	return true
}

// nestedLT is nested_lt in tiny-skia src/line_clipper.rs.
func nestedLT(a, b, dim float32) bool {
	return a <= b && (a < b || dim > 0.0)
}

// containsNoEmptyCheck is contains_no_empty_check in tiny-skia
// src/line_clipper.rs. It returns true if outer contains inner, even if inner
// is empty.
func containsNoEmptyCheck(outer, inner *Rect) bool {
	return outer.Left() <= inner.Left() &&
		outer.Top() <= inner.Top() &&
		outer.Right() >= inner.Right() &&
		outer.Bottom() >= inner.Bottom()
}
