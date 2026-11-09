// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"math"

	"github.com/xo/okage/internal/wide"
)

// lengthU32One is LENGTH_U32_ONE in tiny-skia src/math.rs.
const lengthU32One lengthU32 = 1

// leftShift is left_shift in tiny-skia src/math.rs.
//
// Port: Rust masks the shift count to 0..31 in a release build. Go panics on
// a negative count, so the count is masked here.
func leftShift(value, shift int32) int32 {
	return int32(uint32(value) << (uint32(shift) & 31))
}

// leftShift64 is left_shift64 in tiny-skia src/math.rs.
//
// Port: the shift count is masked to 0..63, as Rust does in a release build.
func leftShift64(value int64, shift int32) int64 {
	return int64(uint64(value) << (uint32(shift) & 63))
}

// bound is bound in tiny-skia src/math.rs, for i64.
//
// Port: upstream is generic over Ord. Only fixed_point.rs calls it, with i64.
func bound(min, value, max int64) int64 {
	return maxI64(minI64(max, value), min)
}

// approxPowf is approx_powf in tiny-skia src/math.rs.
//
// Skia cites
// http://www.machinedlearnings.com/2011/06/fast-approximate-logarithm-exponential.html
func approxPowf(x, y float32) float32 {
	if x == 0.0 || x == 1.0 {
		return x
	}

	e := float32(float32(math.Float32bits(x)) * (1.0 / float32(1<<23)))
	m := math.Float32frombits((math.Float32bits(x) & 0x007fffff) | 0x3f000000)

	log2X := e - 124.225514990 - float32(1.498030302*m) - 1.725879990/(0.3520887068+m)

	x = float32(log2X * y)

	f := x - floorF32(x)

	a := x + 121.274057500
	a -= float32(f * 1.490129070)
	a += 27.728023300 / (4.84252568 - f)
	a = float32(a * float32(1<<23))

	if a < float32(math.Float32bits(float32(math.Inf(1)))) {
		if a > 0.0 {
			return math.Float32frombits(wide.TruncU32(roundF32(a)))
		}
		return 0.0
	}
	return float32(math.Inf(1))
}

// The helpers below port no upstream function. They give the result of the
// methods of f32, f64 and the integer types in the Rust standard library,
// which upstream calls. The helpers of the package path are not exported, so
// the root package has its own.

// sqrtF32 is f32::sqrt. The square root in float64, rounded to float32, is
// the correctly rounded square root in float32.
func sqrtF32(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
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

// maxF32 is f32::max. If one operand is NaN, it returns the other one.
//
// It follows the code that rustc makes for x86-64, as the helper of the
// package path does: the result is x if x > y, and y if not. If y is NaN, the
// result is x.
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
// It follows the code that rustc makes for x86-64, as the helper of the
// package path does: the result is x if x < y, and y if not. If y is NaN, the
// result is x.
func minF32(x, y float32) float32 {
	if y != y {
		return x
	}
	if x < y {
		return x
	}
	return y
}

// maxF64 is f64::max. It has the same rules as maxF32.
func maxF64(x, y float64) float64 {
	if y != y {
		return x
	}
	if x > y {
		return x
	}
	return y
}

// minF64 is f64::min. It has the same rules as minF32.
func minF64(x, y float64) float64 {
	if y != y {
		return x
	}
	if x < y {
		return x
	}
	return y
}

// absI32 is i32::abs in a release build. The absolute value of i32::MIN
// wraps to i32::MIN.
func absI32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// minI32 is i32::min, the method of the trait Ord.
func minI32(x, y int32) int32 {
	if y < x {
		return y
	}
	return x
}

// maxI32 is i32::max, the method of the trait Ord.
func maxI32(x, y int32) int32 {
	if y < x {
		return x
	}
	return y
}

// minI64 is i64::min, the method of the trait Ord.
func minI64(x, y int64) int64 {
	if y < x {
		return y
	}
	return x
}

// maxI64 is i64::max, the method of the trait Ord.
func maxI64(x, y int64) int64 {
	if y < x {
		return x
	}
	return y
}

// maxU32 is u32::max, the method of the trait Ord.
func maxU32(x, y uint32) uint32 { //nolint:unparam // the signature of upstream is kept
	if y < x {
		return x
	}
	return y
}

// checkedSubU32 is u32::checked_sub. It returns false if the result is below
// zero.
func checkedSubU32(x, y uint32) (uint32, bool) {
	if y > x {
		return 0, false
	}
	return x - y, true
}

// checkedAddU32 is u32::checked_add. It returns false if the result does not
// fit in a uint32.
func checkedAddU32(x, y uint32) (uint32, bool) {
	s := x + y
	if s < x {
		return 0, false
	}
	return s, true
}

// checkedMulU32 is u32::checked_mul. It returns false if the result does not
// fit in a uint32.
func checkedMulU32(x, y uint32) (uint32, bool) {
	p := uint64(x) * uint64(y)
	if p > math.MaxUint32 {
		return 0, false
	}
	return uint32(p), true
}
