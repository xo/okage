// Copyright 2012 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"math"
	"slices"

	"github.com/xo/okage/internal/crmath"
)

// pointCount is POINT_COUNT in tiny-skia src/path64/cubic64.rs.
const pointCount = 4

// pi is PI in tiny-skia src/path64/cubic64.rs.
const pi float64 = 3.141592653589793

// cubic64Pair is Cubic64Pair in tiny-skia src/path64/cubic64.rs.
type cubic64Pair struct {
	points [7]point64
}

// cubic64 is Cubic64 in tiny-skia src/path64/cubic64.rs.
type cubic64 struct {
	points [pointCount]point64
}

// newCubic64 is Cubic64::new in tiny-skia src/path64/cubic64.rs.
func newCubic64(points [pointCount]point64) cubic64 {
	return cubic64{points: points}
}

// asF64Slice is Cubic64::as_f64_slice in tiny-skia src/path64/cubic64.rs.
func (c *cubic64) asF64Slice() [pointCount * 2]float64 {
	return [pointCount * 2]float64{
		c.points[0].x,
		c.points[0].y,
		c.points[1].x,
		c.points[1].y,
		c.points[2].x,
		c.points[2].y,
		c.points[3].x,
		c.points[3].y,
	}
}

// pointAtT is Cubic64::point_at_t in tiny-skia src/path64/cubic64.rs.
func (c *cubic64) pointAtT(t float64) point64 {
	if t == 0.0 {
		return c.points[0]
	}

	if t == 1.0 {
		return c.points[3]
	}

	oneT := 1.0 - t
	oneT2 := oneT * oneT
	a := oneT2 * oneT
	b := float64(3.0*oneT2) * t
	t2 := t * t
	cc := float64(3.0*oneT) * t2
	d := t2 * t
	return point64FromXY(
		float64(a*c.points[0].x)+
			float64(b*c.points[1].x)+
			float64(cc*c.points[2].x)+
			float64(d*c.points[3].x),
		float64(a*c.points[0].y)+
			float64(b*c.points[1].y)+
			float64(cc*c.points[2].y)+
			float64(d*c.points[3].y),
	)
}

// searchRoots is Cubic64::search_roots in tiny-skia src/path64/cubic64.rs.
func (c *cubic64) searchRoots(extrema int, axisIntercept float64, xAxis searchAxis, extremeTs *[6]float64, validRoots []float64) int {
	extrema += c.findInflections(extremeTs[extrema:])
	extremeTs[extrema] = 0.0
	extrema++
	extremeTs[extrema] = 1.0
	slices.SortStableFunc(extremeTs[0:extrema], cmpF64)
	validCount := 0
	index := 0
	for index < extrema {
		min := extremeTs[index]
		index++
		max := extremeTs[index]
		if min == max {
			continue
		}

		newT := c.binarySearch(min, max, axisIntercept, xAxis)
		if newT >= 0.0 {
			if validCount >= 3 {
				return 0
			}

			validRoots[validCount] = newT
			validCount++
		}
	}

	return validCount
}

// findInflections is Cubic64::find_inflections in tiny-skia
// src/path64/cubic64.rs.
func (c *cubic64) findInflections(tValues []float64) int {
	ax := c.points[1].x - c.points[0].x
	ay := c.points[1].y - c.points[0].y
	bx := c.points[2].x - float64(2.0*c.points[1].x) + c.points[0].x
	by := c.points[2].y - float64(2.0*c.points[1].y) + c.points[0].y
	cx := c.points[3].x + float64(3.0*(c.points[1].x-c.points[2].x)) - c.points[0].x
	cy := c.points[3].y + float64(3.0*(c.points[1].y-c.points[2].y)) - c.points[0].y
	return rootsValidTQuad64(
		float64(bx*cy)-float64(by*cx),
		float64(ax*cy)-float64(ay*cx),
		float64(ax*by)-float64(ay*bx),
		tValues,
	)
}

// binarySearch is Cubic64::binary_search in tiny-skia src/path64/cubic64.rs.
//
// give up when changing t no longer moves point
// also, copy point rather than recompute it when it does change.
func (c *cubic64) binarySearch(min, max, axisIntercept float64, xAxis searchAxis) float64 {
	t := (min + max) / 2.0
	step := (t - min) / 2.0
	cubicAtT := c.pointAtT(t)
	calcPos := cubicAtT.axisCoord(xAxis)
	calcDist := calcPos - axisIntercept
	for {
		priorT := maxF64(min, t-step)
		lessPt := c.pointAtT(priorT)
		if approximatelyEqualHalf(lessPt.x, cubicAtT.x) &&
			approximatelyEqualHalf(lessPt.y, cubicAtT.y) {
			return -1.0 // binary search found no point at this axis intercept
		}

		lessDist := lessPt.axisCoord(xAxis) - axisIntercept
		lastStep := step
		step /= 2.0
		var ok bool
		if calcDist > 0.0 {
			ok = calcDist > lessDist
		} else {
			ok = calcDist < lessDist
		}
		if ok {
			t = priorT
		} else {
			nextT := t + lastStep
			if nextT > max {
				return -1.0
			}

			morePt := c.pointAtT(nextT)
			if approximatelyEqualHalf(morePt.x, cubicAtT.x) &&
				approximatelyEqualHalf(morePt.y, cubicAtT.y) {
				return -1.0 // binary search found no point at this axis intercept
			}

			moreDist := morePt.axisCoord(xAxis) - axisIntercept
			var ok2 bool
			if calcDist > 0.0 {
				ok2 = calcDist <= moreDist
			} else {
				ok2 = calcDist >= moreDist
			}
			if ok2 {
				continue
			}

			t = nextT
		}

		testAtT := c.pointAtT(t)
		cubicAtT = testAtT
		calcPos = cubicAtT.axisCoord(xAxis)
		calcDist = calcPos - axisIntercept

		if approximatelyEqual(calcPos, axisIntercept) {
			break
		}
	}

	return t
}

// chopAt is Cubic64::chop_at in tiny-skia src/path64/cubic64.rs.
func (c *cubic64) chopAt(t float64) cubic64Pair {
	var dst [7]point64
	if t == 0.5 {
		p := &c.points
		dst[0] = p[0]
		dst[1].x = (p[0].x + p[1].x) / 2.0
		dst[1].y = (p[0].y + p[1].y) / 2.0
		dst[2].x = (p[0].x + float64(2.0*p[1].x) + p[2].x) / 4.0
		dst[2].y = (p[0].y + float64(2.0*p[1].y) + p[2].y) / 4.0
		dst[3].x = (p[0].x + float64(3.0*(p[1].x+p[2].x)) + p[3].x) / 8.0
		dst[3].y = (p[0].y + float64(3.0*(p[1].y+p[2].y)) + p[3].y) / 8.0
		dst[4].x = (p[1].x + float64(2.0*p[2].x) + p[3].x) / 4.0
		dst[4].y = (p[1].y + float64(2.0*p[2].y) + p[3].y) / 4.0
		dst[5].x = (p[2].x + p[3].x) / 2.0
		dst[5].y = (p[2].y + p[3].y) / 2.0
		dst[6] = p[3]

		return cubic64Pair{points: dst}
	}

	interpCubicCoordsX(&c.points, t, &dst)
	interpCubicCoordsY(&c.points, t, &dst)
	return cubic64Pair{points: dst}
}

// coefficients is coefficients in tiny-skia src/path64/cubic64.rs.
func coefficients(src []float64) (float64, float64, float64, float64) {
	a := src[6]                          // d
	b := src[4] * 3.0                    // 3*c
	c := src[2] * 3.0                    // 3*b
	d := src[0]                          // a
	a -= d - c + b                       // A =   -a + 3*b - 3*c + d
	b += float64(3.0*d) - float64(2.0*c) // B =  3*a - 6*b + 3*c
	c -= float64(3.0 * d)                // C = -3*a + 3*b

	return a, b, c, d
}

// rootsValidTCubic64 is roots_valid_t in tiny-skia src/path64/cubic64.rs. The
// name takes the suffix of its module, because src/path64/quad64.rs has a
// function of the same name (D10).
//
// from SkGeometry.cpp (and Numeric Solutions, 5.6).
func rootsValidTCubic64(a, b, c, d float64, t *[3]float64) int {
	var s [3]float64
	realRoots := rootsRealCubic64(a, b, c, d, &s)
	foundRoots := pushValidTs(s[:], realRoots, t[:])
outer:
	for index := range realRoots {
		tValue := s[index]
		if !approximatelyOneOrLess(tValue) && between(tValue, 1.0, 1.00005) {
			for idx2 := range foundRoots {
				if approximatelyEqual(t[idx2], 1.0) {
					continue outer
				}
			}

			t[foundRoots] = 1.0
			foundRoots++
		} else if !approximatelyZeroOrMore(tValue) && between(tValue, -0.00005, 0.0) {
			for idx2 := range foundRoots {
				if approximatelyEqual(t[idx2], 0.0) {
					continue outer
				}
			}

			t[foundRoots] = 0.0
			foundRoots++
		}
	}

	return foundRoots
}

// rootsRealCubic64 is roots_real in tiny-skia src/path64/cubic64.rs. The name
// takes the suffix of its module, because src/path64/quad64.rs has a function
// of the same name (D10).
//
// Port: upstream calls f64::acos and f64::cos, which call the C library. okage
// calls the correctly rounded crmath.Acos64 and crmath.Cos64, which are the
// same on every platform (D26).
func rootsRealCubic64(a, b, c, d float64, s *[3]float64) int {
	if approximatelyZero(a) &&
		approximatelyZeroWhenComparedTo(a, b) &&
		approximatelyZeroWhenComparedTo(a, c) &&
		approximatelyZeroWhenComparedTo(a, d) {
		// we are just a quadratic
		return rootsRealQuad64(b, c, d, s)
	}

	if approximatelyZeroWhenComparedTo(d, a) &&
		approximatelyZeroWhenComparedTo(d, b) &&
		approximatelyZeroWhenComparedTo(d, c) {
		// 0 is one root
		num := rootsRealQuad64(a, b, c, s)
		for i := range num {
			if approximatelyZero(s[i]) {
				return num
			}
		}

		s[num] = 0.0
		num++

		return num
	}

	if approximatelyZero(a + b + c + d) {
		// 1 is one root
		num := rootsRealQuad64(a, a+b, -d, s)
		for i := range num {
			if almostDequalULPs64(s[i], 1.0) {
				return num
			}
		}
		s[num] = 1.0
		num++
		return num
	}

	{
		invA := 1.0 / a
		na := b * invA
		nb := c * invA
		nc := d * invA
		a, b, c = na, nb, nc
	}

	a2 := a * a
	q := (a2 - float64(b*3.0)) / 9.0
	r := (float64(float64(2.0*a2)*a) - float64(float64(9.0*a)*b) + float64(27.0*c)) / 54.0
	r2 := r * r
	q3 := float64(q*q) * q
	r2MinusQ3 := r2 - q3
	adiv3 := a / 3.0
	offset := 0
	if r2MinusQ3 < 0.0 {
		// we have 3 real roots

		// the divide/root can, due to finite precisions, be slightly outside of -1...1
		theta := crmath.Acos64(boundScalar64(r/math.Sqrt(q3), -1.0, 1.0))
		neg2RootQ := -2.0 * math.Sqrt(q)

		rr := float64(neg2RootQ*crmath.Cos64(theta/3.0)) - adiv3
		s[offset] = rr
		offset++

		rr = float64(neg2RootQ*crmath.Cos64((theta+2.0*pi)/3.0)) - adiv3
		if !almostDequalULPs64(s[0], rr) {
			s[offset] = rr
			offset++
		}

		rr = float64(neg2RootQ*crmath.Cos64((theta-2.0*pi)/3.0)) - adiv3
		if !almostDequalULPs64(s[0], rr) && (offset == 1 || !almostDequalULPs64(s[1], rr)) {
			s[offset] = rr
			offset++
		}
	} else {
		// we have 1 real root
		sqrtR2MinusQ3 := math.Sqrt(r2MinusQ3)
		a := math.Abs(r) + sqrtR2MinusQ3
		a = cubeRoot(a)
		if r > 0.0 {
			a = -a
		}

		if a != 0.0 {
			a += q / a
		}

		r2 := a - adiv3
		s[offset] = r2
		offset++
		if almostDequalULPs64(r2, q3) {
			r2 = -a/2.0 - adiv3
			if !almostDequalULPs64(s[0], r2) {
				s[offset] = r2
				offset++
			}
		}
	}

	return offset
}

// findExtrema is find_extrema in tiny-skia src/path64/cubic64.rs.
//
// Cubic64'(t) = At^2 + Bt + C, where
// A = 3(-a + 3(b - c) + d)
// B = 6(a - 2b + c)
// C = 3(b - a)
// Solve for t, keeping only those that fit between 0 < t < 1.
func findExtrema(src []float64, tValues []float64) int {
	// we divide A,B,C by 3 to simplify
	a := src[0]
	b := src[2]
	c := src[4]
	d := src[6]
	a2 := d - a + float64(3.0*(b-c))
	b2 := 2.0 * (a - b - b + c)
	c2 := b - a

	return rootsValidTQuad64(a2, b2, c2, tValues)
}

// cmpF64 is cmp_f64 in tiny-skia src/path64/cubic64.rs.
//
// Skia does not seem to care about NaN/inf during sorting, so we do not either.
func cmpF64(a, b float64) int {
	if a < b {
		return -1
	} else if a > b {
		return 1
	}
	return 0
}

// interpCubicCoordsX is interp_cubic_coords_x in tiny-skia
// src/path64/cubic64.rs. It is the classic one t subdivision.
func interpCubicCoordsX(src *[4]point64, t float64, dst *[7]point64) {
	ab := interp(src[0].x, src[1].x, t)
	bc := interp(src[1].x, src[2].x, t)
	cd := interp(src[2].x, src[3].x, t)
	abc := interp(ab, bc, t)
	bcd := interp(bc, cd, t)
	abcd := interp(abc, bcd, t)

	dst[0].x = src[0].x
	dst[1].x = ab
	dst[2].x = abc
	dst[3].x = abcd
	dst[4].x = bcd
	dst[5].x = cd
	dst[6].x = src[3].x
}

// interpCubicCoordsY is interp_cubic_coords_y in tiny-skia
// src/path64/cubic64.rs.
func interpCubicCoordsY(src *[4]point64, t float64, dst *[7]point64) {
	ab := interp(src[0].y, src[1].y, t)
	bc := interp(src[1].y, src[2].y, t)
	cd := interp(src[2].y, src[3].y, t)
	abc := interp(ab, bc, t)
	bcd := interp(bc, cd, t)
	abcd := interp(abc, bcd, t)

	dst[0].y = src[0].y
	dst[1].y = ab
	dst[2].y = abc
	dst[3].y = abcd
	dst[4].y = bcd
	dst[5].y = cd
	dst[6].y = src[3].y
}
