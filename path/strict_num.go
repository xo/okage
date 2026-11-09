// Copyright (c) 2022 Yevhenii Reizner
//
// Use of this source code is governed by the MIT license that can be found in
// the NOTICE file.

package path

import "github.com/xo/okage/internal/wide"

// This file is a port of the crate strict-num 0.1.1, src/lib.rs. The crate
// holds a collection of bounded numeric types.
//
// Port: okage ports only the f32 types, FiniteF32, PositiveF32,
// NonZeroPositiveF32 and NormalizedF32, because tiny-skia uses only them. The
// f64 types, the impls of Display and the feature approx-eq are not ported.
// The unsafe constructors new_unchecked are not ported, because a Go
// conversion such as FiniteF32(x) does the same thing.
//
// Unlike f32, the types of strict-num implement Ord and Hash in Rust, because
// each value is finite. In Go, the operators == and < on the values do the
// same thing.

// FiniteF32 is FiniteF32 in strict-num src/lib.rs. It is an immutable, finite
// float32.
type FiniteF32 float32

// NewFiniteF32 is FiniteF32::new in strict-num src/lib.rs. It creates a finite
// float32.
//
// It returns false for NaN and infinity.
func NewFiniteF32(n float32) (FiniteF32, bool) {
	if isFinite(n) {
		return FiniteF32(n), true
	}
	return 0, false
}

// Get is FiniteF32::get in strict-num src/lib.rs. It returns the value as a
// primitive type.
func (n FiniteF32) Get() float32 {
	return float32(n)
}

// PositiveF32 is PositiveF32 in strict-num src/lib.rs. It is an immutable,
// finite float32 that is known to be >= 0.
type PositiveF32 float32

// PositiveF32Zero is PositiveF32::ZERO in strict-num src/lib.rs. It is a
// PositiveF32 with the value zero.
const PositiveF32Zero PositiveF32 = 0

// NewPositiveF32 is PositiveF32::new in strict-num src/lib.rs. It creates a
// new PositiveF32 if the given value is >= 0.
//
// It returns false for a negative value, NaN and infinity.
func NewPositiveF32(n float32) (PositiveF32, bool) {
	if isFinite(n) && n >= 0 {
		return PositiveF32(n), true
	}
	return 0, false
}

// Get is PositiveF32::get in strict-num src/lib.rs. It returns the value as a
// primitive type.
func (n PositiveF32) Get() float32 {
	return float32(n)
}

// GetFinite is PositiveF32::get_finite in strict-num src/lib.rs. It returns
// the value as a FiniteF32.
func (n PositiveF32) GetFinite() FiniteF32 {
	return FiniteF32(n)
}

// NonZeroPositiveF32 is NonZeroPositiveF32 in strict-num src/lib.rs. It is an
// immutable, finite float32 that is known to be > 0.
type NonZeroPositiveF32 float32

// NewNonZeroPositiveF32 is NonZeroPositiveF32::new in strict-num src/lib.rs.
// It creates a new NonZeroPositiveF32 if the given value is > 0.
//
// It returns false for a negative value, zero, NaN and infinity.
func NewNonZeroPositiveF32(n float32) (NonZeroPositiveF32, bool) {
	if isFinite(n) && n > 0 {
		return NonZeroPositiveF32(n), true
	}
	return 0, false
}

// Get is NonZeroPositiveF32::get in strict-num src/lib.rs. It returns the
// value as a primitive type.
func (n NonZeroPositiveF32) Get() float32 {
	return float32(n)
}

// GetFinite is NonZeroPositiveF32::get_finite in strict-num src/lib.rs. It
// returns the value as a FiniteF32.
func (n NonZeroPositiveF32) GetFinite() FiniteF32 {
	return FiniteF32(n)
}

// NormalizedF32 is NormalizedF32 in strict-num src/lib.rs. It is an immutable,
// finite float32 in the range 0..=1.
type NormalizedF32 float32

// NormalizedF32Zero is NormalizedF32::ZERO in strict-num src/lib.rs. It is a
// NormalizedF32 with the value zero.
const NormalizedF32Zero NormalizedF32 = 0

// NormalizedF32One is NormalizedF32::ONE in strict-num src/lib.rs. It is a
// NormalizedF32 with the value one.
const NormalizedF32One NormalizedF32 = 1

// NewNormalizedF32 is NormalizedF32::new in strict-num src/lib.rs. It creates
// a NormalizedF32 if the given value is in the range 0..=1.
func NewNormalizedF32(n float32) (NormalizedF32, bool) {
	if isFinite(n) && n >= 0 && n <= 1 {
		return NormalizedF32(n), true
	}
	return 0, false
}

// NormalizedF32Clamped is NormalizedF32::new_clamped in strict-num
// src/lib.rs. It creates a NormalizedF32 and clamps the given value to the
// range 0..=1.
//
// It returns zero for NaN or infinity.
func NormalizedF32Clamped(n float32) NormalizedF32 {
	if isFinite(n) {
		return NormalizedF32(clampF32(0, n, 1))
	}
	return NormalizedF32Zero
}

// NormalizedF32FromU8 is NormalizedF32::new_u8 in strict-num src/lib.rs. It
// creates a NormalizedF32 by dividing the given value by 255.
func NormalizedF32FromU8(n uint8) NormalizedF32 {
	return NormalizedF32(float32(n) / 255.0)
}

// NormalizedF32FromU16 is NormalizedF32::new_u16 in strict-num src/lib.rs. It
// creates a NormalizedF32 by dividing the given value by 65535.
func NormalizedF32FromU16(n uint16) NormalizedF32 {
	return NormalizedF32(float32(n) / 65535.0)
}

// Get is NormalizedF32::get in strict-num src/lib.rs. It returns the value as
// a primitive type.
func (n NormalizedF32) Get() float32 {
	return float32(n)
}

// GetFinite is NormalizedF32::get_finite in strict-num src/lib.rs. It returns
// the value as a FiniteF32.
func (n NormalizedF32) GetFinite() FiniteF32 {
	return FiniteF32(n)
}

// ToU8 is NormalizedF32::to_u8 in strict-num src/lib.rs. It returns the value
// as a uint8.
func (n NormalizedF32) ToU8() uint8 {
	return wide.TruncU8(float32(float32(n)*255.0) + 0.5)
}

// ToU16 is NormalizedF32::to_u16 in strict-num src/lib.rs. It returns the
// value as a uint16.
func (n NormalizedF32) ToU16() uint16 {
	return wide.TruncU16(float32(float32(n)*65535.0) + 0.5)
}

// Mul is the core::ops::Mul impl of NormalizedF32 in strict-num src/lib.rs.
// The product is clamped to the range 0..=1.
func (n NormalizedF32) Mul(rhs NormalizedF32) NormalizedF32 {
	return NormalizedF32Clamped(float32(float32(n) * float32(rhs)))
}

// clampF32 is clamp_f32 in strict-num src/lib.rs.
func clampF32(min, val, max float32) float32 {
	return maxF32(minF32(max, val), min)
}
