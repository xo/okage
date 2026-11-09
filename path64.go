// Copyright 2012 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"math"

	"github.com/xo/okage/path"
)

// The code below is from SkPathOpsTypes.

// epsilonF64 is f64::EPSILON in Rust, 2^-52. It ports no upstream item.
const epsilonF64 float64 = 0x1p-52

// epsilonF32 is f32::EPSILON in Rust, 2^-23. It ports no upstream item.
const epsilonF32 float32 = 0x1p-23

// dblEpsilonErr is DBL_EPSILON_ERR in tiny-skia src/path64/mod.rs.
//
//nolint:unused // upstream defines it, and the port keeps it
const dblEpsilonErr float64 = epsilonF64 * 4.0

// fltEpsilonHalf is FLT_EPSILON_HALF in tiny-skia src/path64/mod.rs.
const fltEpsilonHalf = float64(epsilonF32 / 2.0)

// fltEpsilonCubed is FLT_EPSILON_CUBED in tiny-skia src/path64/mod.rs.
//
// Port: upstream computes the cube in f32. 2^-69 is exact in f32, so the
// constant is exact.
const fltEpsilonCubed = float64(epsilonF32 * epsilonF32 * epsilonF32)

// fltEpsilonInverse is FLT_EPSILON_INVERSE in tiny-skia src/path64/mod.rs.
const fltEpsilonInverse float64 = 1.0 / float64(epsilonF32)

// The functions boundScalar64 to almostDequalULPs64 port the trait Scalar64
// in tiny-skia src/path64/mod.rs, which upstream implements for f64. Go
// cannot add a method to float64, so each method is a function.

// boundScalar64 is Scalar64::bound in tiny-skia src/path64/mod.rs.
//
// Works just like SkTPin, returning `max` for NaN/inf.
func boundScalar64(x, min, max float64) float64 {
	return maxF64(minF64(max, x), min)
}

// between is Scalar64::between in tiny-skia src/path64/mod.rs. It returns true
// if (a <= x <= b) || (a >= x >= b).
func between(x, a, b float64) bool {
	return float64((a-x)*(b-x)) <= 0.0
}

// preciselyZero is Scalar64::precisely_zero in tiny-skia src/path64/mod.rs.
//
//nolint:unused // upstream defines it, and the port keeps it
func preciselyZero(x float64) bool {
	return math.Abs(x) < dblEpsilonErr
}

// approximatelyZeroOrMore is Scalar64::approximately_zero_or_more in
// tiny-skia src/path64/mod.rs.
func approximatelyZeroOrMore(x float64) bool {
	return x > -epsilonF64
}

// approximatelyOneOrLess is Scalar64::approximately_one_or_less in tiny-skia
// src/path64/mod.rs.
func approximatelyOneOrLess(x float64) bool {
	return x < 1.0+epsilonF64
}

// approximatelyZero is Scalar64::approximately_zero in tiny-skia
// src/path64/mod.rs.
func approximatelyZero(x float64) bool {
	return math.Abs(x) < epsilonF64
}

// approximatelyZeroInverse is Scalar64::approximately_zero_inverse in
// tiny-skia src/path64/mod.rs.
func approximatelyZeroInverse(x float64) bool {
	return math.Abs(x) > fltEpsilonInverse
}

// approximatelyZeroCubed is Scalar64::approximately_zero_cubed in tiny-skia
// src/path64/mod.rs.
func approximatelyZeroCubed(x float64) bool {
	return math.Abs(x) < fltEpsilonCubed
}

// approximatelyZeroHalf is Scalar64::approximately_zero_half in tiny-skia
// src/path64/mod.rs.
func approximatelyZeroHalf(x float64) bool {
	return x < fltEpsilonHalf
}

// approximatelyZeroWhenComparedTo is
// Scalar64::approximately_zero_when_compared_to in tiny-skia
// src/path64/mod.rs.
func approximatelyZeroWhenComparedTo(x, other float64) bool {
	return x == 0.0 || math.Abs(x) < math.Abs(float64(other*float64(epsilonF32)))
}

// approximatelyEqual is Scalar64::approximately_equal in tiny-skia
// src/path64/mod.rs.
//
// Use this for comparing Ts in the range of 0 to 1. For general numbers (larger and smaller) use
// AlmostEqualUlps instead.
func approximatelyEqual(x, other float64) bool {
	return approximatelyZero(x - other)
}

// approximatelyEqualHalf is Scalar64::approximately_equal_half in tiny-skia
// src/path64/mod.rs.
func approximatelyEqualHalf(x, other float64) bool {
	return approximatelyZeroHalf(x - other)
}

// almostDequalULPs64 is Scalar64::almost_dequal_ulps in tiny-skia
// src/path64/mod.rs.
func almostDequalULPs64(x, other float64) bool {
	if math.Abs(x) < float64(path.ScalarMax) && math.Abs(other) < float64(path.ScalarMax) {
		return path.AlmostDequalULPs(float32(x), float32(other))
	}
	return math.Abs(x-other)/maxF64(math.Abs(x), math.Abs(other)) < float64(epsilonF32*16.0)
}

// cubeRoot is cube_root in tiny-skia src/path64/mod.rs.
func cubeRoot(x float64) float64 {
	if approximatelyZeroCubed(x) {
		return 0.0
	}

	result := halleyCbrt3d(math.Abs(x))
	if x < 0.0 {
		return -result
	}
	return result
}

// halleyCbrt3d is halley_cbrt3d in tiny-skia src/path64/mod.rs. It is a cube
// root approximation using 3 iterations of Halley's method (double).
func halleyCbrt3d(d float64) float64 {
	a := cbrt5d(d)
	a = cbrtaHalleyd(a, d)
	a = cbrtaHalleyd(a, d)
	return cbrtaHalleyd(a, d)
}

// cbrt5d is cbrt_5d in tiny-skia src/path64/mod.rs. It is a cube root
// approximation using a bit hack for a 64-bit float, adapted from Kahan's
// cbrt.
//
// Port: upstream casts the f64 to [u32; 2] on a little-endian machine, so
// pt[1] and px[1] are the high 32 bits of the float. The low 32 bits of t are
// zero.
func cbrt5d(d float64) float64 {
	const b1 = 715094163
	px1 := uint32(math.Float64bits(d) >> 32)
	pt1 := px1/3 + b1
	return math.Float64frombits(uint64(pt1) << 32)
}

// cbrtaHalleyd is cbrta_halleyd in tiny-skia src/path64/mod.rs. It is an
// iterative cube root approximation using Halley's method (double).
func cbrtaHalleyd(a, r float64) float64 {
	a3 := float64(a*a) * a
	return float64(a*(a3+r+r)) / (a3 + a3 + r)
}

// interp is interp in tiny-skia src/path64/mod.rs.
func interp(a, b, t float64) float64 {
	return a + float64((b-a)*t)
}
