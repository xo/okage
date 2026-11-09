// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import "github.com/xo/okage/path"

// Port: upstream re-exports chop_cubic_at2, chop_quad_at,
// find_cubic_max_curvature, find_unit_quad_roots, new_t_values, CubicCoeff and
// QuadCoeff of tiny_skia_path::path_geometry here. The Go code calls them in
// the package path.

// chopQuadAtXExtrema is chop_quad_at_x_extrema in tiny-skia
// src/path_geometry.rs.
//
// Returns 0 for 1 quad, and 1 for two quads, either way the answer is stored
// in dst[].
//
// Guarantees that the 1/2 quads will be monotonic.
func chopQuadAtXExtrema(src *[3]Point, dst *[5]Point) int {
	a := src[0].X
	b := src[1].X
	c := src[2].X

	if isNotMonotonic(a, b, c) {
		if tValue, ok := path.ValidUnitDivide(a-b, a-b-b+c); ok {
			path.ChopQuadAt(src[:], tValue, dst)

			// flatten double quad extrema
			dst[1].X = dst[2].X
			dst[3].X = dst[2].X

			return 1
		}

		// if we get here, we need to force dst to be monotonic, even though
		// we cannot compute a unit_divide value (probably underflow).
		if absF32(a-b) < absF32(b-c) {
			b = a
		} else {
			b = c
		}
	}

	dst[0] = path.PointFromXY(a, src[0].Y)
	dst[1] = path.PointFromXY(b, src[1].Y)
	dst[2] = path.PointFromXY(c, src[2].Y)
	return 0
}

// chopQuadAtYExtrema is chop_quad_at_y_extrema in tiny-skia
// src/path_geometry.rs.
//
// Returns 0 for 1 quad, and 1 for two quads, either way the answer is stored
// in dst[].
//
// Guarantees that the 1/2 quads will be monotonic.
func chopQuadAtYExtrema(src *[3]Point, dst *[5]Point) int {
	a := src[0].Y
	b := src[1].Y
	c := src[2].Y

	if isNotMonotonic(a, b, c) {
		if tValue, ok := path.ValidUnitDivide(a-b, a-b-b+c); ok {
			path.ChopQuadAt(src[:], tValue, dst)

			// flatten double quad extrema
			dst[1].Y = dst[2].Y
			dst[3].Y = dst[2].Y

			return 1
		}

		// if we get here, we need to force dst to be monotonic, even though
		// we cannot compute a unit_divide value (probably underflow).
		if absF32(a-b) < absF32(b-c) {
			b = a
		} else {
			b = c
		}
	}

	dst[0] = path.PointFromXY(src[0].X, a)
	dst[1] = path.PointFromXY(src[1].X, b)
	dst[2] = path.PointFromXY(src[2].X, c)
	return 0
}

// isNotMonotonic is is_not_monotonic in tiny-skia src/path_geometry.rs.
func isNotMonotonic(a, b, c float32) bool {
	ab := a - b
	bc := b - c
	if ab < 0.0 {
		bc = -bc
	}

	return ab == 0.0 || bc < 0.0
}

// chopCubicAtXExtrema is chop_cubic_at_x_extrema in tiny-skia
// src/path_geometry.rs.
func chopCubicAtXExtrema(src *[4]Point, dst *[10]Point) int {
	tValuesArr := path.NewTValues()
	tValues := findCubicExtrema(src[0].X, src[1].X, src[2].X, src[3].X, &tValuesArr)

	chopCubicAt(src, tValues, dst[:])
	if len(tValues) != 0 {
		// we do some cleanup to ensure our X extrema are flat
		dst[2].X = dst[3].X
		dst[4].X = dst[3].X
		if len(tValues) == 2 {
			dst[5].X = dst[6].X
			dst[7].X = dst[6].X
		}
	}

	return len(tValues)
}

// chopCubicAtYExtrema is chop_cubic_at_y_extrema in tiny-skia
// src/path_geometry.rs.
//
// Given 4 points on a cubic bezier, chop it into 1, 2, 3 beziers such that
// the resulting beziers are monotonic in Y.
//
// This is called by the scan converter.
//
// Depending on what is returned, dst[] is treated as follows:
//
//   - 0: dst[0..3] is the original cubic
//   - 1: dst[0..3] and dst[3..6] are the two new cubics
//   - 2: dst[0..3], dst[3..6], dst[6..9] are the three new cubics
func chopCubicAtYExtrema(src *[4]Point, dst *[10]Point) int {
	tValuesArr := path.NewTValues()
	tValues := findCubicExtrema(src[0].Y, src[1].Y, src[2].Y, src[3].Y, &tValuesArr)

	chopCubicAt(src, tValues, dst[:])
	if len(tValues) != 0 {
		// we do some cleanup to ensure our Y extrema are flat
		dst[2].Y = dst[3].Y
		dst[4].Y = dst[3].Y
		if len(tValues) == 2 {
			dst[5].Y = dst[6].Y
			dst[7].Y = dst[6].Y
		}
	}

	return len(tValues)
}

// findCubicExtrema is find_cubic_extrema in tiny-skia src/path_geometry.rs.
//
// Cubic'(t) = At^2 + Bt + C, where
// A = 3(-a + 3(b - c) + d)
// B = 6(a - 2b + c)
// C = 3(b - a)
// Solve for t, keeping only those that fit between 0 < t < 1.
func findCubicExtrema(a, b, c, d float32, tValues *[3]path.NormalizedF32Exclusive) []path.NormalizedF32Exclusive {
	// we divide A,B,C by 3 to simplify
	na := d - a + float32(3.0*(b-c))
	nb := 2.0 * (a - b - b + c)
	nc := b - a

	roots := path.FindUnitQuadRoots(na, nb, nc, tValues)
	return tValues[0:roots]
}

// chopCubicAt is chop_cubic_at in tiny-skia src/path_geometry.rs.
//
// http://code.google.com/p/skia/issues/detail?id=32
//
// This test code failed when we did not check the return result of
// valid_unit_divide in SkChopCubicAt(... NormalizedF32Exclusives[], int
// roots). The reason is that after the first chop, the parameters to
// valid_unit_divide are equal (thanks to finite float precision and rounding
// in the subtracts). Thus even though the 2nd NormalizedF32Exclusive looks <
// 1.0, after we renormalize it, we end up with 1.0, hence the need to check
// and just return the last cubic as a degenerate clump of 4 points in the
// same place.
func chopCubicAt(src *[4]Point, tValues []path.NormalizedF32Exclusive, dst []Point) {
	if len(tValues) == 0 {
		// nothing to chop
		dst[0] = src[0]
		dst[1] = src[1]
		dst[2] = src[2]
		dst[3] = src[3]
		return
	}

	t := tValues[0]
	var tmp [4]Point

	dstOffset := 0
	for i := range tValues {
		path.ChopCubicAt2(src, t, dst[dstOffset:])
		if i == len(tValues)-1 {
			break
		}

		dstOffset += 3
		// have src point to the remaining cubic (after the chop)
		tmp[0] = dst[dstOffset+0]
		tmp[1] = dst[dstOffset+1]
		tmp[2] = dst[dstOffset+2]
		tmp[3] = dst[dstOffset+3]
		src = &tmp

		// watch out in case the renormalized t is not in range
		n, ok := path.ValidUnitDivide(
			tValues[i+1].Get()-tValues[i].Get(),
			1.0-tValues[i].Get(),
		)

		if !ok {
			// if we cannot, just create a degenerate cubic
			dst[dstOffset+4] = src[3]
			dst[dstOffset+5] = src[3]
			dst[dstOffset+6] = src[3]
			break
		}
		t = n
	}
}

// chopCubicAtMaxCurvature is chop_cubic_at_max_curvature in tiny-skia
// src/path_geometry.rs.
func chopCubicAtMaxCurvature(src *[4]Point, tValues *[3]path.NormalizedF32Exclusive, dst []Point) int {
	var rootsArr [3]path.NormalizedF32
	roots := path.FindCubicMaxCurvature(src, &rootsArr)

	// Throw out values not inside 0..1.
	count := 0
	for _, root := range roots {
		if 0.0 < root.Get() && root.Get() < 1.0 {
			tValues[count] = path.NormalizedF32ExclusiveBounded(root.Get())
			count++
		}
	}

	if count == 0 {
		copy(dst[0:4], src[:])
	} else {
		chopCubicAt(src, tValues[0:count], dst)
	}

	return count + 1
}

// chopMonoCubicAtXPathGeometry is chop_mono_cubic_at_x in tiny-skia
// src/path_geometry.rs. The name takes the suffix of its module, because
// src/edge_clipper.rs has a function of the same name (D10).
func chopMonoCubicAtXPathGeometry(src *[4]Point, x float32, dst *[7]Point) bool {
	return cubicDchopAtIntercept(src, x, true, dst)
}

// chopMonoCubicAtYPathGeometry is chop_mono_cubic_at_y in tiny-skia
// src/path_geometry.rs. The name takes the suffix of its module, because
// src/edge_clipper.rs has a function of the same name (D10).
func chopMonoCubicAtYPathGeometry(src *[4]Point, y float32, dst *[7]Point) bool {
	return cubicDchopAtIntercept(src, y, false, dst)
}

// cubicDchopAtIntercept is cubic_dchop_at_intercept in tiny-skia
// src/path_geometry.rs.
func cubicDchopAtIntercept(src *[4]Point, intercept float32, isVertical bool, dst *[7]Point) bool {
	src64 := [4]point64{
		point64FromPoint(src[0]),
		point64FromPoint(src[1]),
		point64FromPoint(src[2]),
		point64FromPoint(src[3]),
	}

	cubic := newCubic64(src64)
	var roots [3]float64
	var count int
	if isVertical {
		count = verticalIntersect(&cubic, float64(intercept), &roots)
	} else {
		count = horizontalIntersect(&cubic, float64(intercept), &roots)
	}

	if count > 0 {
		pair := cubic.chopAt(roots[0])
		for i := range 7 {
			dst[i] = pair.points[i].toPoint()
		}

		return true
	}
	return false
}
