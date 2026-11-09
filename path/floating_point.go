// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import (
	"math"

	"github.com/xo/okage/internal/wide"
)

// Port: upstream re-exports FiniteF32, NonZeroPositiveF32 and NormalizedF32
// of strict-num here. They are in strict_num.go of this package.

// floatPi is FLOAT_PI in tiny-skia-path path/src/floating_point.rs.
const floatPi float32 = 3.14159265

// maxI32FitsInF32 is MAX_I32_FITS_IN_F32 in tiny-skia-path
// path/src/floating_point.rs.
const maxI32FitsInF32 float32 = 2147483520.0

// minI32FitsInF32 is MIN_I32_FITS_IN_F32 in tiny-skia-path
// path/src/floating_point.rs.
const minI32FitsInF32 float32 = -maxI32FitsInF32

// The functions SaturateFrom to SaturateRound port the traits SaturateCast
// and SaturateRound in tiny-skia-path path/src/floating_point.rs, which hold
// custom routines that convert a float to an integer. Upstream implements
// them only for i32.

// SaturateFrom is i32::saturate_from of the trait SaturateCast<f32> in
// tiny-skia-path path/src/floating_point.rs. It returns the closest integer
// for the given float.
//
// It returns MAX_I32_FITS_IN_F32 for NaN.
func SaturateFrom(x float32) int32 {
	// Port: the comparisons keep the form of upstream, so that NaN gives
	// MAX_I32_FITS_IN_F32.
	if !(x < maxI32FitsInF32) {
		x = maxI32FitsInF32
	}
	if !(x > minI32FitsInF32) {
		x = minI32FitsInF32
	}
	return wide.TruncI32(x)
}

// SaturateFromF64 is i32::saturate_from of the trait SaturateCast<f64> in
// tiny-skia-path path/src/floating_point.rs. It returns the closest integer
// for the given double.
//
// It returns i32::MAX for NaN.
func SaturateFromF64(x float64) int32 {
	// Port: the comparisons keep the form of upstream, so that NaN gives
	// i32::MAX.
	if !(x < float64(math.MaxInt32)) {
		x = float64(math.MaxInt32)
	}
	if !(x > float64(math.MinInt32)) {
		x = float64(math.MinInt32)
	}
	return wide.Trunc64I32(x)
}

// SaturateFloor is i32::saturate_floor of the trait SaturateRound<f32> in
// tiny-skia-path path/src/floating_point.rs.
func SaturateFloor(x float32) int32 {
	return SaturateFrom(floorF32(x))
}

// SaturateCeil is i32::saturate_ceil of the trait SaturateRound<f32> in
// tiny-skia-path path/src/floating_point.rs.
func SaturateCeil(x float32) int32 {
	return SaturateFrom(ceilF32(x))
}

// SaturateRound is i32::saturate_round of the trait SaturateRound<f32> in
// tiny-skia-path path/src/floating_point.rs.
func SaturateRound(x float32) int32 {
	return SaturateFrom(floorF32(x) + 0.5)
}

// f32As2sCompliment is f32_as_2s_compliment in tiny-skia-path
// path/src/floating_point.rs.
//
// Return the float as a 2s compliment int. Just to be used to compare floats
// to each other or against positive float-bit-constants (like 0). This does
// not return the int equivalent of the float, just something cheaper for
// compares-only.
func f32As2sCompliment(x float32) int32 {
	return signBitTo2sCompliment(int32(math.Float32bits(x)))
}

// signBitTo2sCompliment is sign_bit_to_2s_compliment in tiny-skia-path
// path/src/floating_point.rs.
//
// Convert a sign-bit int (i.e. float interpreted as int) into a 2s compliement
// int. This also converts -0 (0x80000000) to 0. Doing this to a float allows
// it to be compared using normal C operators (<, <=, etc.)
func signBitTo2sCompliment(x int32) int32 {
	if x < 0 {
		x &= 0x7FFFFFFF
		x = -x
	}

	return x
}

// NormalizedF32Exclusive is NormalizedF32Exclusive in tiny-skia-path
// path/src/floating_point.rs. It is an immutable float32 that is larger than 0
// and less than 1.
//
// Port: upstream wraps a FiniteF32. A conversion such as
// NormalizedF32Exclusive(x) does not check the value, as new_unchecked does
// not in Rust. Use NewNormalizedF32Exclusive to check it.
type NormalizedF32Exclusive float32

// NormalizedF32ExclusiveAny is NormalizedF32Exclusive::ANY in tiny-skia-path
// path/src/floating_point.rs. It is just a random, valid number.
const NormalizedF32ExclusiveAny = NormalizedF32ExclusiveHalf

// NormalizedF32ExclusiveHalf is NormalizedF32Exclusive::HALF in tiny-skia-path
// path/src/floating_point.rs. It is a predefined 0.5 value.
const NormalizedF32ExclusiveHalf NormalizedF32Exclusive = 0.5

// NewNormalizedF32Exclusive is NormalizedF32Exclusive::new in tiny-skia-path
// path/src/floating_point.rs. It creates a NormalizedF32Exclusive.
func NewNormalizedF32Exclusive(n float32) (NormalizedF32Exclusive, bool) {
	if n > 0 && n < 1 {
		// `n` is guarantee to be finite after the bounds check.
		f, ok := NewFiniteF32(n)
		if !ok {
			return 0, false
		}
		return NormalizedF32Exclusive(f), true
	}
	return 0, false
}

// NormalizedF32ExclusiveBounded is NormalizedF32Exclusive::new_bounded in
// tiny-skia-path path/src/floating_point.rs. It creates a
// NormalizedF32Exclusive and clamps the given value.
//
// Upstream says that it returns zero for NaN or infinity. The code returns
// the upper bound, 1 - f32::EPSILON, for NaN and infinity.
func NormalizedF32ExclusiveBounded(n float32) NormalizedF32Exclusive {
	n = Bound(n, epsilonF32, 1.0-epsilonF32)
	// `n` is guarantee to be finite after clamping.
	return NormalizedF32Exclusive(n)
}

// Get is NormalizedF32Exclusive::get in tiny-skia-path
// path/src/floating_point.rs. It returns the value as a primitive type.
func (n NormalizedF32Exclusive) Get() float32 {
	return float32(n)
}

// ToNormalized is NormalizedF32Exclusive::to_normalized in tiny-skia-path
// path/src/floating_point.rs. It returns the value as a NormalizedF32.
func (n NormalizedF32Exclusive) ToNormalized() NormalizedF32 {
	// NormalizedF32 is (0,1), while NormalizedF32 is [0,1], so it will always fit.
	return NormalizedF32(n)
}

// epsilonF32 is f32::EPSILON in Rust, 2^-23. It ports no upstream item.
const epsilonF32 float32 = 0x1p-23
