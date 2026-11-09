// Copyright 2012 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import "math"

// pushValidTs is push_valid_ts in tiny-skia src/path64/quad64.rs.
func pushValidTs(s []float64, realRoots int, t []float64) int {
	foundRoots := 0
outer:
	for index := range realRoots {
		tValue := s[index]
		if approximatelyZeroOrMore(tValue) && approximatelyOneOrLess(tValue) {
			tValue = boundScalar64(tValue, 0.0, 1.0)

			for idx2 := range foundRoots {
				if approximatelyEqual(t[idx2], tValue) {
					continue outer
				}
			}

			t[foundRoots] = tValue
			foundRoots++
		}
	}

	return foundRoots
}

// rootsValidTQuad64 is roots_valid_t in tiny-skia src/path64/quad64.rs. The
// name takes the suffix of its module, because src/path64/cubic64.rs has a
// function of the same name (D10).
//
// note: caller expects multiple results to be sorted smaller first
// note: http://en.wikipedia.org/wiki/Loss_of_significance has an interesting
// analysis of the quadratic equation, suggesting why the following looks at
// the sign of B -- and further suggesting that the greatest loss of precision
// is in b squared less two a c.
func rootsValidTQuad64(a, b, c float64, t []float64) int {
	var s [3]float64
	realRoots := rootsRealQuad64(a, b, c, &s)
	return pushValidTs(s[:], realRoots, t)
}

// rootsRealQuad64 is roots_real in tiny-skia src/path64/quad64.rs. The name
// takes the suffix of its module, because src/path64/cubic64.rs has a function
// of the same name (D10).
//
// Numeric Solutions (5.6) suggests to solve the quadratic by computing
//
//	Q = -1/2(B + sgn(B)Sqrt(B^2 - 4 A C))
//
// and using the roots
//
//	t1 = Q / A
//	t2 = C / Q
//
// this does not discard real roots <= 0 or >= 1.
func rootsRealQuad64(a, b, c float64, s *[3]float64) int {
	if a == 0.0 {
		return handleZero(b, c, s)
	}

	p := b / (2.0 * a)
	q := c / a
	if approximatelyZero(a) && (approximatelyZeroInverse(p) || approximatelyZeroInverse(q)) {
		return handleZero(b, c, s)
	}

	// normal form: x^2 + px + q = 0
	p2 := p * p
	if !almostDequalULPs64(p2, q) && p2 < q {
		return 0
	}

	sqrtD := 0.0
	if p2 > q {
		sqrtD = math.Sqrt(p2 - q)
	}

	s[0] = sqrtD - p
	s[1] = -sqrtD - p
	if !almostDequalULPs64(s[0], s[1]) {
		return 2
	}
	return 1
}

// handleZero is handle_zero in tiny-skia src/path64/quad64.rs.
func handleZero(b, c float64, s *[3]float64) int {
	if approximatelyZero(b) {
		s[0] = 0.0
		if c == 0.0 {
			return 1
		}
		return 0
	}
	s[0] = -c / b
	return 1
}
