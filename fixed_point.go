// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"math"

	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// Skia uses fixed points pretty chaotically, therefore we cannot use
// strongly typed wrappers. Which is unfortunate.
//
// Port: the functions of the modules fdot6, fdot8 and fdot16 take the name of
// their module as a prefix, because Rust calls them as fdot6::div, and both
// fdot6 and fdot16 have a div.

// fDot6 is FDot6 in tiny-skia src/fixed_point.rs. It is a 26.6 fixed point.
type fDot6 = int32

// fDot8 is FDot8 in tiny-skia src/fixed_point.rs. It is a 24.8 fixed point.
type fDot8 = int32

// fDot16 is FDot16 in tiny-skia src/fixed_point.rs. It is a 16.16 fixed point.
type fDot16 = int32

// fdot6One is fdot6::ONE in tiny-skia src/fixed_point.rs.
const fdot6One fDot6 = 64

// fdot6FromI32 is fdot6::from_i32 in tiny-skia src/fixed_point.rs.
func fdot6FromI32(n int32) fDot6 {
	return n << 6
}

// fdot6FromF32 is fdot6::from_f32 in tiny-skia src/fixed_point.rs.
func fdot6FromF32(n float32) fDot6 {
	return wide.TruncI32(float32(n * 64.0))
}

// fdot6Floor is fdot6::floor in tiny-skia src/fixed_point.rs.
func fdot6Floor(n fDot6) fDot6 {
	return n >> 6
}

// fdot6Ceil is fdot6::ceil in tiny-skia src/fixed_point.rs.
func fdot6Ceil(n fDot6) fDot6 {
	return (n + 63) >> 6
}

// fdot6Round is fdot6::round in tiny-skia src/fixed_point.rs.
func fdot6Round(n fDot6) fDot6 {
	return (n + 32) >> 6
}

// fdot6ToFDot16 is fdot6::to_fdot16 in tiny-skia src/fixed_point.rs.
func fdot6ToFDot16(n fDot6) fDot16 {
	return leftShift(n, 10)
}

// fdot6Div is fdot6::div in tiny-skia src/fixed_point.rs.
func fdot6Div(a, b fDot6) fDot16 {
	if a >= math.MinInt16 && a <= math.MaxInt16 {
		return leftShift(a, 16) / b
	}
	return fdot16Div(a, b)
}

// fdot6CanConvertToFDot16 is fdot6::can_convert_to_fdot16 in tiny-skia
// src/fixed_point.rs.
//
//nolint:unused // upstream calls it only in a debug_assert
func fdot6CanConvertToFDot16(n fDot6) bool {
	maxDot6 := int32(math.MaxInt32 >> (16 - 6))
	return absI32(n) <= maxDot6
}

// fdot6SmallScale is fdot6::small_scale in tiny-skia src/fixed_point.rs.
func fdot6SmallScale(value uint8, dot6 fDot6) uint8 {
	return uint8((int32(value) * dot6) >> 6)
}

// fdot8FromFDot16 is fdot8::from_fdot16 in tiny-skia src/fixed_point.rs.
//
// Extracted from SkScan_Antihair.cpp.
func fdot8FromFDot16(x fDot16) fDot8 {
	return (x + 0x80) >> 8
}

// fdot16Half is fdot16::HALF in tiny-skia src/fixed_point.rs.
const fdot16Half fDot16 = (1 << 16) / 2

// fdot16One is fdot16::ONE in tiny-skia src/fixed_point.rs.
const fdot16One fDot16 = 1 << 16

// fdot16FromF32 is fdot16::from_f32 in tiny-skia src/fixed_point.rs.
//
// `from_f32` seems to lack a rounding step. For all fixed-point values, this
// version is as accurate as possible for (fixed -> float -> fixed). Rounding
// reduces accuracy if the intermediate floats are in the range that only
// holds integers (adding 0.5 to an odd integer then snaps to nearest even).
// Using double for the rounding math gives maximum accuracy for
// (float -> fixed -> float), but that is usually overkill.
func fdot16FromF32(x float32) fDot16 {
	return path.SaturateFrom(float32(x * float32(fdot16One)))
}

// fdot16FloorToI32 is fdot16::floor_to_i32 in tiny-skia src/fixed_point.rs.
func fdot16FloorToI32(x fDot16) int32 {
	return x >> 16
}

// fdot16CeilToI32 is fdot16::ceil_to_i32 in tiny-skia src/fixed_point.rs.
func fdot16CeilToI32(x fDot16) int32 {
	return (x + fdot16One - 1) >> 16
}

// fdot16RoundToI32 is fdot16::round_to_i32 in tiny-skia src/fixed_point.rs.
func fdot16RoundToI32(x fDot16) int32 {
	return (x + fdot16Half) >> 16
}

// fdot16Mul is fdot16::mul in tiny-skia src/fixed_point.rs.
//
// The divide can exceed 32 bits. Clamp to a signed 32 bit result.
func fdot16Mul(a, b fDot16) fDot16 {
	return fDot16((int64(a) * int64(b)) >> 16)
}

// fdot16Div is fdot16::div in tiny-skia src/fixed_point.rs.
//
// The divide can exceed 32 bits. Clamp to a signed 32 bit result.
func fdot16Div(numer, denom fDot6) fDot16 {
	v := leftShift64(int64(numer), 16) / int64(denom)
	n := bound(math.MinInt32, v, math.MaxInt32)
	return int32(n)
}

// fdot16FastDiv is fdot16::fast_div in tiny-skia src/fixed_point.rs.
func fdot16FastDiv(a, b fDot6) fDot16 {
	return leftShift(a, 16) / b
}
