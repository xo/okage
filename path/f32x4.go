// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

// f32x4 is f32x4 in tiny-skia-path path/src/f32x4_t.rs.
//
// Right now, there are no visible benefits of using SIMD for f32x4. So we don't.
type f32x4 [4]float32

// max is f32x4::max in tiny-skia-path path/src/f32x4_t.rs.
func (v f32x4) max(rhs f32x4) f32x4 {
	return f32x4{
		maxF32(v[0], rhs[0]),
		maxF32(v[1], rhs[1]),
		maxF32(v[2], rhs[2]),
		maxF32(v[3], rhs[3]),
	}
}

// min is f32x4::min in tiny-skia-path path/src/f32x4_t.rs.
func (v f32x4) min(rhs f32x4) f32x4 {
	return f32x4{
		minF32(v[0], rhs[0]),
		minF32(v[1], rhs[1]),
		minF32(v[2], rhs[2]),
		minF32(v[3], rhs[3]),
	}
}

// add is the core::ops::Add impl of f32x4 in tiny-skia-path
// path/src/f32x4_t.rs.
//
//nolint:unused // upstream defines it, and the port keeps it
func (v f32x4) add(rhs f32x4) f32x4 {
	return f32x4{
		v[0] + rhs[0],
		v[1] + rhs[1],
		v[2] + rhs[2],
		v[3] + rhs[3],
	}
}

// addAssign is the core::ops::AddAssign impl of f32x4 in tiny-skia-path
// path/src/f32x4_t.rs.
//
//nolint:unused // upstream defines it, and the port keeps it
func (v *f32x4) addAssign(rhs f32x4) {
	*v = v.add(rhs)
}

// sub is the core::ops::Sub impl of f32x4 in tiny-skia-path
// path/src/f32x4_t.rs.
//
//nolint:unused // upstream defines it, and the port keeps it
func (v f32x4) sub(rhs f32x4) f32x4 {
	return f32x4{
		v[0] - rhs[0],
		v[1] - rhs[1],
		v[2] - rhs[2],
		v[3] - rhs[3],
	}
}

// mul is the core::ops::Mul impl of f32x4 in tiny-skia-path
// path/src/f32x4_t.rs.
func (v f32x4) mul(rhs f32x4) f32x4 {
	return f32x4{
		float32(v[0] * rhs[0]),
		float32(v[1] * rhs[1]),
		float32(v[2] * rhs[2]),
		float32(v[3] * rhs[3]),
	}
}

// mulAssign is the core::ops::MulAssign impl of f32x4 in tiny-skia-path
// path/src/f32x4_t.rs.
func (v *f32x4) mulAssign(rhs f32x4) {
	*v = v.mul(rhs)
}
