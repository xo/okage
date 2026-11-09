// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import (
	"math"

	"github.com/xo/okage/internal/crmath"
)

// Transform is Transform in tiny-skia-path path/src/transform.rs. It is an
// affine transformation matrix.
//
// Unlike other types, a Transform is not guaranteed to be valid. This is a
// quirk of Skia. So Transform(0, 0, 0, 0, 0, 0) is allowed, though it is not
// valid. Values that are not finite are also not an error.
type Transform struct {
	SX float32
	KX float32
	KY float32
	SY float32
	TX float32
	TY float32
}

// DefaultTransform is the Default impl of Transform in tiny-skia-path
// path/src/transform.rs. It returns the identity transform.
func DefaultTransform() Transform {
	return Transform{
		SX: 1.0,
		KX: 0.0,
		KY: 0.0,
		SY: 1.0,
		TX: 0.0,
		TY: 0.0,
	}
}

// IdentityTransform is Transform::identity in tiny-skia-path
// path/src/transform.rs. It creates an identity transform.
func IdentityTransform() Transform {
	return DefaultTransform()
}

// TransformFromRow is Transform::from_row in tiny-skia-path
// path/src/transform.rs. It creates a new Transform.
//
// We are using column-major-column-vector matrix notation, therefore it's ky-kx, not kx-ky.
func TransformFromRow(sx, ky, kx, sy, tx, ty float32) Transform {
	return Transform{
		SX: sx,
		KY: ky,
		KX: kx,
		SY: sy,
		TX: tx,
		TY: ty,
	}
}

// TransformFromTranslate is Transform::from_translate in tiny-skia-path
// path/src/transform.rs. It creates a new translating Transform.
func TransformFromTranslate(tx, ty float32) Transform {
	return TransformFromRow(1.0, 0.0, 0.0, 1.0, tx, ty)
}

// TransformFromScale is Transform::from_scale in tiny-skia-path
// path/src/transform.rs. It creates a new scaling Transform.
func TransformFromScale(sx, sy float32) Transform {
	return TransformFromRow(sx, 0.0, 0.0, sy, 0.0, 0.0)
}

// TransformFromSkew is Transform::from_skew in tiny-skia-path
// path/src/transform.rs. It creates a new skewing Transform.
func TransformFromSkew(kx, ky float32) Transform {
	return TransformFromRow(1.0, ky, kx, 1.0, 0.0, 0.0)
}

// TransformFromRotate is Transform::from_rotate in tiny-skia-path
// path/src/transform.rs. It creates a new rotating Transform.
//
// The angle is in degrees.
func TransformFromRotate(angle float32) Transform {
	v := toRadians(angle)
	a := cosF32(v)
	b := sinF32(v)
	c := -b
	d := a
	return TransformFromRow(a, b, c, d, 0.0, 0.0)
}

// TransformFromRotateAt is Transform::from_rotate_at in tiny-skia-path
// path/src/transform.rs. It creates a new rotating Transform at the given
// position.
//
// The angle is in degrees.
func TransformFromRotateAt(angle, tx, ty float32) Transform {
	ts := DefaultTransform()
	ts = ts.PreTranslate(tx, ty)
	ts = ts.PreConcat(TransformFromRotate(angle))
	ts = ts.PreTranslate(-tx, -ty)
	return ts
}

// TransformFromBBox is Transform::from_bbox in tiny-skia-path
// path/src/transform.rs. It converts a rectangle into a bounding box
// Transform.
func TransformFromBBox(bbox NonZeroRect) Transform {
	return TransformFromRow(bbox.Width(), 0.0, 0.0, bbox.Height(), bbox.X(), bbox.Y())
}

// IsFinite is Transform::is_finite in tiny-skia-path path/src/transform.rs. It
// checks that the transform is finite.
func (ts Transform) IsFinite() bool {
	return isFinite(ts.SX) &&
		isFinite(ts.KY) &&
		isFinite(ts.KX) &&
		isFinite(ts.SY) &&
		isFinite(ts.TX) &&
		isFinite(ts.TY)
}

// IsValid is Transform::is_valid in tiny-skia-path path/src/transform.rs. It
// checks that the transform is finite and has a scale that is not zero.
func (ts Transform) IsValid() bool {
	if ts.IsFinite() {
		sx, sy := ts.GetScale()
		return !(IsNearlyZeroWithinTolerance(sx, epsilonF32) ||
			IsNearlyZeroWithinTolerance(sy, epsilonF32))
	}
	return false
}

// IsIdentity is Transform::is_identity in tiny-skia-path
// path/src/transform.rs. It checks that the transform is the identity.
func (ts Transform) IsIdentity() bool {
	return ts == DefaultTransform()
}

// IsScale is Transform::is_scale in tiny-skia-path path/src/transform.rs. It
// checks that the transform only scales.
func (ts Transform) IsScale() bool {
	return ts.HasScale() && !ts.HasSkew() && !ts.HasTranslate()
}

// IsSkew is Transform::is_skew in tiny-skia-path path/src/transform.rs. It
// checks that the transform only skews.
func (ts Transform) IsSkew() bool {
	return !ts.HasScale() && ts.HasSkew() && !ts.HasTranslate()
}

// IsTranslate is Transform::is_translate in tiny-skia-path
// path/src/transform.rs. It checks that the transform only translates.
func (ts Transform) IsTranslate() bool {
	return !ts.HasScale() && !ts.HasSkew() && ts.HasTranslate()
}

// IsScaleTranslate is Transform::is_scale_translate in tiny-skia-path
// path/src/transform.rs. It checks that the transform holds only a scale and a
// translation.
func (ts Transform) IsScaleTranslate() bool {
	return (ts.HasScale() || ts.HasTranslate()) && !ts.HasSkew()
}

// HasScale is Transform::has_scale in tiny-skia-path path/src/transform.rs. It
// checks that the transform holds a scale part.
func (ts Transform) HasScale() bool {
	return ts.SX != 1.0 || ts.SY != 1.0
}

// HasSkew is Transform::has_skew in tiny-skia-path path/src/transform.rs. It
// checks that the transform holds a skew part.
func (ts Transform) HasSkew() bool {
	return ts.KX != 0.0 || ts.KY != 0.0
}

// HasTranslate is Transform::has_translate in tiny-skia-path
// path/src/transform.rs. It checks that the transform holds a translate part.
func (ts Transform) HasTranslate() bool {
	return ts.TX != 0.0 || ts.TY != 0.0
}

// GetScale is Transform::get_scale in tiny-skia-path path/src/transform.rs. It
// returns the scale part of the transform.
func (ts Transform) GetScale() (float32, float32) {
	xScale := sqrtF32(float32(ts.SX*ts.SX) + float32(ts.KX*ts.KX))
	yScale := sqrtF32(float32(ts.KY*ts.KY) + float32(ts.SY*ts.SY))
	return xScale, yScale
}

// PreScale is Transform::pre_scale in tiny-skia-path path/src/transform.rs. It
// pre-scales the transform.
func (ts Transform) PreScale(sx, sy float32) Transform {
	return ts.PreConcat(TransformFromScale(sx, sy))
}

// PostScale is Transform::post_scale in tiny-skia-path path/src/transform.rs.
// It post-scales the transform.
func (ts Transform) PostScale(sx, sy float32) Transform {
	return ts.PostConcat(TransformFromScale(sx, sy))
}

// PreTranslate is Transform::pre_translate in tiny-skia-path
// path/src/transform.rs. It pre-translates the transform.
func (ts Transform) PreTranslate(tx, ty float32) Transform {
	return ts.PreConcat(TransformFromTranslate(tx, ty))
}

// PostTranslate is Transform::post_translate in tiny-skia-path
// path/src/transform.rs. It post-translates the transform.
func (ts Transform) PostTranslate(tx, ty float32) Transform {
	return ts.PostConcat(TransformFromTranslate(tx, ty))
}

// PreRotate is Transform::pre_rotate in tiny-skia-path path/src/transform.rs.
// It pre-rotates the transform.
//
// The angle is in degrees.
func (ts Transform) PreRotate(angle float32) Transform {
	return ts.PreConcat(TransformFromRotate(angle))
}

// PostRotate is Transform::post_rotate in tiny-skia-path
// path/src/transform.rs. It post-rotates the transform.
//
// The angle is in degrees.
func (ts Transform) PostRotate(angle float32) Transform {
	return ts.PostConcat(TransformFromRotate(angle))
}

// PreRotateAt is Transform::pre_rotate_at in tiny-skia-path
// path/src/transform.rs. It pre-rotates the transform around the given
// position.
//
// The angle is in degrees.
func (ts Transform) PreRotateAt(angle, tx, ty float32) Transform {
	return ts.PreConcat(TransformFromRotateAt(angle, tx, ty))
}

// PostRotateAt is Transform::post_rotate_at in tiny-skia-path
// path/src/transform.rs. It post-rotates the transform around the given
// position.
//
// The angle is in degrees.
func (ts Transform) PostRotateAt(angle, tx, ty float32) Transform {
	return ts.PostConcat(TransformFromRotateAt(angle, tx, ty))
}

// PreConcat is Transform::pre_concat in tiny-skia-path path/src/transform.rs.
// It pre-concatenates the transform.
func (ts Transform) PreConcat(other Transform) Transform {
	return concat(ts, other)
}

// PostConcat is Transform::post_concat in tiny-skia-path
// path/src/transform.rs. It post-concatenates the transform.
func (ts Transform) PostConcat(other Transform) Transform {
	return concat(other, ts)
}

// transformFromSinCos is Transform::from_sin_cos in tiny-skia-path
// path/src/transform.rs.
func transformFromSinCos(sin, cos float32) Transform {
	return TransformFromRow(cos, sin, -sin, cos, 0.0, 0.0)
}

// MapPoint is Transform::map_point in tiny-skia-path path/src/transform.rs. It
// transforms a point with the transform.
func (ts Transform) MapPoint(point *Point) {
	if ts.IsIdentity() {
		// Do nothing.
	} else if ts.IsTranslate() {
		point.X += ts.TX
		point.Y += ts.TY
	} else if ts.IsScaleTranslate() {
		point.X = float32(point.X*ts.SX) + ts.TX
		point.Y = float32(point.Y*ts.SY) + ts.TY
	} else {
		x := float32(point.X*ts.SX) + float32(point.Y*ts.KX) + ts.TX
		y := float32(point.X*ts.KY) + float32(point.Y*ts.SY) + ts.TY
		point.X = x
		point.Y = y
	}
}

// MapPoints is Transform::map_points in tiny-skia-path path/src/transform.rs.
// It transforms a slice of points with the transform.
func (ts Transform) MapPoints(points []Point) {
	if len(points) == 0 {
		return
	}

	// TODO: simd

	if ts.IsIdentity() {
		// Do nothing.
	} else if ts.IsTranslate() {
		for i := range points {
			p := &points[i]
			p.X += ts.TX
			p.Y += ts.TY
		}
	} else if ts.IsScaleTranslate() {
		for i := range points {
			p := &points[i]
			p.X = float32(p.X*ts.SX) + ts.TX
			p.Y = float32(p.Y*ts.SY) + ts.TY
		}
	} else {
		for i := range points {
			p := &points[i]
			x := float32(p.X*ts.SX) + float32(p.Y*ts.KX) + ts.TX
			y := float32(p.X*ts.KY) + float32(p.Y*ts.SY) + ts.TY
			p.X = x
			p.Y = y
		}
	}
}

// Invert is Transform::invert in tiny-skia-path path/src/transform.rs. It
// returns the inverted transform.
func (ts Transform) Invert() (Transform, bool) {
	// Allow the trivial case to be inlined.
	if ts.IsIdentity() {
		return ts, true
	}

	return invert(ts)
}

// invert is invert in tiny-skia-path path/src/transform.rs.
func invert(ts Transform) (Transform, bool) {
	if ts.IsScaleTranslate() {
		if ts.HasScale() {
			invX := Invert(ts.SX)
			invY := Invert(ts.SY)
			return TransformFromRow(
				invX,
				0.0,
				0.0,
				invY,
				float32(-ts.TX*invX),
				float32(-ts.TY*invY),
			), true
		}
		// translate only
		return TransformFromTranslate(-ts.TX, -ts.TY), true
	}
	invDet, ok := invDeterminant(ts)
	if !ok {
		return Transform{}, false
	}
	invTs := computeInv(ts, invDet)

	if invTs.IsFinite() {
		return invTs, true
	}
	return Transform{}, false
}

// invDeterminant is inv_determinant in tiny-skia-path path/src/transform.rs.
func invDeterminant(ts Transform) (float64, bool) {
	det := dcross(float64(ts.SX), float64(ts.SY), float64(ts.KX), float64(ts.KY))

	// Since the determinant is on the order of the cube of the matrix members,
	// compare to the cube of the default nearly-zero constant (although an
	// estimate of the condition number would be better if it wasn't so expensive).
	tolerance := ScalarNearlyZero * ScalarNearlyZero * ScalarNearlyZero
	if IsNearlyZeroWithinTolerance(float32(det), tolerance) {
		return 0, false
	}
	return 1.0 / det, true
}

// computeInv is compute_inv in tiny-skia-path path/src/transform.rs.
func computeInv(ts Transform, invDet float64) Transform {
	return TransformFromRow(
		float32(float64(ts.SY)*invDet),
		float32(float64(-ts.KY)*invDet),
		float32(float64(-ts.KX)*invDet),
		float32(float64(ts.SX)*invDet),
		dcrossDscale(ts.KX, ts.TY, ts.SY, ts.TX, invDet),
		dcrossDscale(ts.KY, ts.TX, ts.SX, ts.TY, invDet),
	)
}

// dcross is dcross in tiny-skia-path path/src/transform.rs.
func dcross(a, b, c, d float64) float64 {
	return float64(a*b) - float64(c*d)
}

// dcrossDscale is dcross_dscale in tiny-skia-path path/src/transform.rs.
func dcrossDscale(a, b, c, d float32, scale float64) float32 {
	return float32(dcross(float64(a), float64(b), float64(c), float64(d)) * scale)
}

// concat is concat in tiny-skia-path path/src/transform.rs.
func concat(a, b Transform) Transform {
	if a.IsIdentity() {
		return b
	} else if b.IsIdentity() {
		return a
	} else if !a.HasSkew() && !b.HasSkew() {
		// just scale and translate
		return TransformFromRow(
			float32(a.SX*b.SX),
			0.0,
			0.0,
			float32(a.SY*b.SY),
			float32(a.SX*b.TX)+a.TX,
			float32(a.SY*b.TY)+a.TY,
		)
	}
	return TransformFromRow(
		mulAddMul(a.SX, b.SX, a.KX, b.KY),
		mulAddMul(a.KY, b.SX, a.SY, b.KY),
		mulAddMul(a.SX, b.KX, a.KX, b.SY),
		mulAddMul(a.KY, b.KX, a.SY, b.SY),
		mulAddMul(a.SX, b.TX, a.KX, b.TY)+a.TX,
		mulAddMul(a.KY, b.TX, a.SY, b.TY)+a.TY,
	)
}

// mulAddMul is mul_add_mul in tiny-skia-path path/src/transform.rs.
func mulAddMul(a, b, c, d float32) float32 {
	return float32(float64(float64(a)*float64(b)) + float64(float64(c)*float64(d)))
}

// The helpers below port no upstream function. They give the result of the
// methods of f32 in the Rust standard library, which upstream calls.

// radsPerDeg is the constant RADS_PER_DEG of f32::to_radians in Rust:
// consts::PI / 180.0, computed in f32. A Go conversion of a constant rounds
// it, so float32(math.Pi) is the f32 PI of Rust, and the division of two
// typed constants rounds once, as the f32 division does.
const radsPerDeg = float32(math.Pi) / 180.0

// toRadians is f32::to_radians.
func toRadians(x float32) float32 {
	return float32(x * radsPerDeg)
}

// cosF32 is f32::cos. Rust calls cosf of the C library. okage gives the
// correctly rounded result, which is the same on every platform (D26).
func cosF32(x float32) float32 {
	return crmath.Cos32(x)
}

// sinF32 is f32::sin. Rust calls sinf of the C library. okage gives the
// correctly rounded result, which is the same on every platform (D26).
func sinF32(x float32) float32 {
	return crmath.Sin32(x)
}
