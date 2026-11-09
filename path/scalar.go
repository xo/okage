// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import "math"

// ScalarMax is SCALAR_MAX in tiny-skia-path path/src/scalar.rs.
const ScalarMax float32 = 3.402823466e+38

// ScalarNearlyZero is SCALAR_NEARLY_ZERO in tiny-skia-path path/src/scalar.rs.
const ScalarNearlyZero float32 = 1.0 / (1 << 12)

// ScalarRoot2Over2 is SCALAR_ROOT_2_OVER_2 in tiny-skia-path
// path/src/scalar.rs.
const ScalarRoot2Over2 float32 = 0.707106781

// The functions Half to AlmostDequalULPs are the methods of the trait Scalar
// in tiny-skia-path path/src/scalar.rs, which upstream implements for f32.
// The trait holds float extension methods. Upstream says that they are mainly
// for internal use, and that a user must not rely on them.

// Half is Scalar::half in tiny-skia-path path/src/scalar.rs.
func Half(x float32) float32 {
	return float32(x * 0.5)
}

// Ave is Scalar::ave in tiny-skia-path path/src/scalar.rs.
func Ave(x, other float32) float32 {
	return float32((x + other) * 0.5)
}

// Sqr is Scalar::sqr in tiny-skia-path path/src/scalar.rs.
func Sqr(x float32) float32 {
	return float32(x * x)
}

// Invert is Scalar::invert in tiny-skia-path path/src/scalar.rs.
func Invert(x float32) float32 {
	return 1.0 / x
}

// Bound is Scalar::bound in tiny-skia-path path/src/scalar.rs. It is a clamp
// that does not panic.
//
// It works like SkTPin, and returns max for NaN and infinity.
func Bound(x, min, max float32) float32 {
	return maxF32(minF32(max, x), min)
}

// IsNearlyEqual is Scalar::is_nearly_equal in tiny-skia-path
// path/src/scalar.rs.
func IsNearlyEqual(x, other float32) bool {
	return absF32(x-other) <= ScalarNearlyZero
}

// IsNearlyEqualWithinTolerance is Scalar::is_nearly_equal_within_tolerance in
// tiny-skia-path path/src/scalar.rs.
func IsNearlyEqualWithinTolerance(x, other, tolerance float32) bool {
	return absF32(x-other) <= tolerance
}

// IsNearlyZero is Scalar::is_nearly_zero in tiny-skia-path path/src/scalar.rs.
func IsNearlyZero(x float32) bool {
	return IsNearlyZeroWithinTolerance(x, ScalarNearlyZero)
}

// IsNearlyZeroWithinTolerance is Scalar::is_nearly_zero_within_tolerance in
// tiny-skia-path path/src/scalar.rs.
func IsNearlyZeroWithinTolerance(x, tolerance float32) bool {
	return absF32(x) <= tolerance
}

// AlmostDequalULPs is Scalar::almost_dequal_ulps in tiny-skia-path
// path/src/scalar.rs. It comes from SkPathOpsTypes.
func AlmostDequalULPs(x, other float32) bool {
	const ulpsEpsilon int32 = 16
	aBits := f32As2sCompliment(x)
	bBits := f32As2sCompliment(other)
	// Find the difference in ULPs.
	return aBits < bBits+ulpsEpsilon && bBits < aBits+ulpsEpsilon
}

// The helpers below port no upstream function. They give the result of the
// methods of f32 and f64 in the Rust standard library, which upstream calls.
// The trait NoStdFloat of upstream, for a build without std, is not ported.

// isFinite is f32::is_finite.
func isFinite(x float32) bool {
	return !math.IsInf(float64(x), 0) && !math.IsNaN(float64(x))
}

// sqrtF32 is f32::sqrt. The square root in float64, rounded to float32, is
// the correctly rounded square root in float32.
func sqrtF32(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}

// sqrtF64 is f64::sqrt.
func sqrtF64(x float64) float64 {
	return math.Sqrt(x)
}

// absF32 is f32::abs.
func absF32(x float32) float32 {
	return float32(math.Abs(float64(x)))
}

// floorF32 is f32::floor.
func floorF32(x float32) float32 {
	return float32(math.Floor(float64(x)))
}

// ceilF32 is f32::ceil.
func ceilF32(x float32) float32 {
	return float32(math.Ceil(float64(x)))
}

// roundF32 is f32::round. It rounds half away from zero.
func roundF32(x float32) float32 {
	return float32(math.Round(float64(x)))
}

// remF32 is the operator % of f32, which is fmodf in C. The result of fmod is
// exact, so the float64 result is the float32 result.
func remF32(x, y float32) float32 {
	return float32(math.Mod(float64(x), float64(y)))
}

// maxF32 is f32::max. If one operand is NaN, it returns the other one.
//
// It follows the code that rustc 1.97.1 makes for x86-64, which was read on
// 2026-11-09: the result is x if x > y, and y if not. If that result is NaN,
// the result is x. So for two zeros of different signs, the result is y.
func maxF32(x, y float32) float32 {
	if y != y {
		return x
	}
	if x > y {
		return x
	}
	return y
}

// minF32 is f32::min. If one operand is NaN, it returns the other one.
//
// It follows the code that rustc 1.97.1 makes for x86-64, which was read on
// 2026-11-09: the result is x if x < y, and y if not. If that result is NaN,
// the result is x. So for two zeros of different signs, the result is y.
func minF32(x, y float32) float32 {
	if y != y {
		return x
	}
	if x < y {
		return x
	}
	return y
}
