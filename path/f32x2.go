// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

// F32x2 is f32x2 in tiny-skia-path path/src/f32x2_t.rs. It is a pair of
// float32 numbers.
//
// Upstream says that it is mainly for internal use, and that a user must not
// rely on it.
//
// Right now, there are no visible benefits of using SIMD for f32x2. So we don't.
type F32x2 [2]float32

// NewF32x2 is f32x2::new in tiny-skia-path path/src/f32x2_t.rs. It creates a
// new pair.
func NewF32x2(a, b float32) F32x2 {
	return F32x2{a, b}
}

// F32x2Splat is f32x2::splat in tiny-skia-path path/src/f32x2_t.rs. It creates
// a new pair from a single value.
func F32x2Splat(x float32) F32x2 {
	return F32x2{x, x}
}

// Abs is f32x2::abs in tiny-skia-path path/src/f32x2_t.rs. It returns an
// absolute value.
func (v F32x2) Abs() F32x2 {
	return F32x2{absF32(v.X()), absF32(v.Y())}
}

// Min is f32x2::min in tiny-skia-path path/src/f32x2_t.rs. It returns a
// minimum value.
func (v F32x2) Min(other F32x2) F32x2 {
	return F32x2{pmin(v.X(), other.X()), pmin(v.Y(), other.Y())}
}

// Max is f32x2::max in tiny-skia-path path/src/f32x2_t.rs. It returns a
// maximum value.
func (v F32x2) Max(other F32x2) F32x2 {
	return F32x2{pmax(v.X(), other.X()), pmax(v.Y(), other.Y())}
}

// MaxComponent is f32x2::max_component in tiny-skia-path path/src/f32x2_t.rs.
// It returns the maximum of both values.
func (v F32x2) MaxComponent() float32 {
	return pmax(v.X(), v.Y())
}

// X is f32x2::x in tiny-skia-path path/src/f32x2_t.rs. It returns the first
// value.
func (v F32x2) X() float32 {
	return v[0]
}

// Y is f32x2::y in tiny-skia-path path/src/f32x2_t.rs. It returns the second
// value.
func (v F32x2) Y() float32 {
	return v[1]
}

// Add is the core::ops::Add impl of f32x2 in tiny-skia-path
// path/src/f32x2_t.rs.
func (v F32x2) Add(other F32x2) F32x2 {
	return F32x2{v.X() + other.X(), v.Y() + other.Y()}
}

// Sub is the core::ops::Sub impl of f32x2 in tiny-skia-path
// path/src/f32x2_t.rs.
func (v F32x2) Sub(other F32x2) F32x2 {
	return F32x2{v.X() - other.X(), v.Y() - other.Y()}
}

// Mul is the core::ops::Mul impl of f32x2 in tiny-skia-path
// path/src/f32x2_t.rs.
func (v F32x2) Mul(other F32x2) F32x2 {
	return F32x2{float32(v.X() * other.X()), float32(v.Y() * other.Y())}
}

// Div is the core::ops::Div impl of f32x2 in tiny-skia-path
// path/src/f32x2_t.rs.
func (v F32x2) Div(other F32x2) F32x2 {
	return F32x2{v.X() / other.X(), v.Y() / other.Y()}
}

// A faster and more forgiving f32 min/max implementation.
//
// Unlike std one, we do not care about NaN.

// pmax is pmax in tiny-skia-path path/src/f32x2_t.rs.
func pmax(a, b float32) float32 {
	if a < b {
		return b
	}
	return a
}

// pmin is pmin in tiny-skia-path path/src/f32x2_t.rs.
func pmin(a, b float32) float32 {
	if b < a {
		return b
	}
	return a
}
