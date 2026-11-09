// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"math"

	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/tree/geom.rs.

// NonZeroRect is the re-export of tiny_skia_path::NonZeroRect in usvg
// src/tree/geom.rs.
type NonZeroRect = path.NonZeroRect

// Rect is the re-export of tiny_skia_path::Rect in usvg src/tree/geom.rs.
type Rect = path.Rect

// Size is the re-export of tiny_skia_path::Size in usvg src/tree/geom.rs.
type Size = path.Size

// Transform is the re-export of tiny_skia_path::Transform in usvg
// src/tree/geom.rs.
type Transform = path.Transform

// ApproxEqUlps32 is the impl ApproxEqUlps for f32 of the crate float-cmp
// 0.9.0, src/ulps_eq.rs, which usvg src/tree/mod.rs exports again through
// strict_num. It reports whether x and other are within ulps units of least
// precision of each other.
//
// Differing signs are always unequal, and zeroes are only equal to zeroes.
func ApproxEqUlps32(x, other float32, ulps int32) bool {
	// -0 and +0 are drastically far in ulps terms, so
	// we need a special case for that.
	if x == other {
		return true
	}

	// Handle differing signs as a special case, even if
	// they are very close, most people consider them
	// unequal.
	if math.Signbit(float64(x)) != math.Signbit(float64(other)) {
		return false
	}

	diff := int32(math.Float32bits(x)) - int32(math.Float32bits(other))
	return diff >= -ulps && diff <= ulps
}

// ApproxEqUlps64 is the impl ApproxEqUlps for f64 of the crate float-cmp
// 0.9.0, src/ulps_eq.rs, which usvg src/tree/mod.rs exports again through
// strict_num. It reports whether x and other are within ulps units of least
// precision of each other.
//
// Differing signs are always unequal, and zeroes are only equal to zeroes.
func ApproxEqUlps64(x, other float64, ulps int64) bool {
	// -0 and +0 are drastically far in ulps terms, so
	// we need a special case for that.
	if x == other {
		return true
	}

	// Handle differing signs as a special case, even if
	// they are very close, most people consider them
	// unequal.
	if math.Signbit(x) != math.Signbit(other) {
		return false
	}

	diff := int64(math.Float64bits(x)) - int64(math.Float64bits(other))
	return diff >= -ulps && diff <= ulps
}

// ApproxZeroUlps32 is the impl ApproxZeroUlps for f32 in usvg
// src/tree/geom.rs. It checks if the number is approximately zero.
//
// Port: the Rust trait ApproxZeroUlps becomes one function for each type
// that implements it.
func ApproxZeroUlps32(x float32, ulps int32) bool {
	return ApproxEqUlps32(x, 0.0, ulps)
}

// ApproxZeroUlps64 is the impl ApproxZeroUlps for f64 in usvg
// src/tree/geom.rs. It checks if the number is approximately zero.
func ApproxZeroUlps64(x float64, ulps int64) bool {
	return ApproxEqUlps64(x, 0.0, ulps)
}

// isValidLength32 is the impl IsValidLength for f32 in usvg
// src/tree/geom.rs. It checks that the current number is > 0.
func isValidLength32(x float32) bool {
	return x > 0.0 && !math.IsInf(float64(x), 0) && !math.IsNaN(float64(x))
}

// isValidLength64 is the impl IsValidLength for f64 in usvg
// src/tree/geom.rs. It checks that the current number is > 0.
func isValidLength64(x float64) bool {
	return x > 0.0 && !math.IsInf(x, 0) && !math.IsNaN(x)
}

// viewBox is ViewBox in usvg src/tree/geom.rs. It is a view box.
type viewBox struct {
	// Value of the `viewBox` attribute.
	rect NonZeroRect

	// Value of the `preserveAspectRatio` attribute.
	aspect svgtypes.AspectRatio
}

// toTransform is ViewBox::to_transform in usvg src/tree/geom.rs. It converts
// `viewBox` into `Transform`.
func (v viewBox) toTransform(imgSize Size) Transform {
	vr := v.rect

	sx := imgSize.Width() / vr.Width()
	sy := imgSize.Height() / vr.Height()

	if v.aspect.Align != svgtypes.AlignNone {
		var s float32
		if v.aspect.Slice {
			if sx < sy {
				s = sy
			} else {
				s = sx
			}
		} else {
			if sx > sy {
				s = sy
			} else {
				s = sx
			}
		}

		sx, sy = s, s
	}

	x := -vr.X() * sx
	y := -vr.Y() * sy
	w := imgSize.Width() - float32(vr.Width()*sx)
	h := imgSize.Height() - float32(vr.Height()*sy)

	tx, ty := alignedPos(v.aspect.Align, x, y, w, h)
	return path.TransformFromRow(sx, 0.0, 0.0, sy, tx, ty)
}

// bbox is BBox in usvg src/tree/geom.rs. It is a bounding box calculator.
//
// Port: the impls From<Rect> and From<NonZeroRect> become bboxFromRect and
// bboxFromNonZeroRect, and expand takes a bbox.
type bbox struct {
	left   float32
	top    float32
	right  float32
	bottom float32
}

// bboxFromRect is the impl From<Rect> for BBox in usvg src/tree/geom.rs.
func bboxFromRect(r Rect) bbox {
	return bbox{
		left:   r.Left(),
		top:    r.Top(),
		right:  r.Right(),
		bottom: r.Bottom(),
	}
}

// bboxFromNonZeroRect is the impl From<NonZeroRect> for BBox in usvg
// src/tree/geom.rs.
func bboxFromNonZeroRect(r NonZeroRect) bbox {
	return bbox{
		left:   r.Left(),
		top:    r.Top(),
		right:  r.Right(),
		bottom: r.Bottom(),
	}
}

// defaultBBox is the impl Default for BBox in usvg src/tree/geom.rs.
func defaultBBox() bbox {
	return bbox{
		left:   math.MaxFloat32,
		top:    math.MaxFloat32,
		right:  -math.MaxFloat32,
		bottom: -math.MaxFloat32,
	}
}

// isDefault is BBox::is_default in usvg src/tree/geom.rs. It checks if the
// bounding box is default, i.e. invalid.
func (b bbox) isDefault() bool {
	return b.left == math.MaxFloat32 &&
		b.top == math.MaxFloat32 &&
		b.right == -math.MaxFloat32 &&
		b.bottom == -math.MaxFloat32
}

// expand is BBox::expand in usvg src/tree/geom.rs. It expands the bounding
// box to the specified bounds.
func (b bbox) expand(r bbox) bbox {
	return b.expandImpl(r)
}

// expandImpl is BBox::expand_impl in usvg src/tree/geom.rs.
func (b bbox) expandImpl(r bbox) bbox {
	return bbox{
		left:   minF32(b.left, r.left),
		top:    minF32(b.top, r.top),
		right:  maxF32(b.right, r.right),
		bottom: maxF32(b.bottom, r.bottom),
	}
}

// toRect is BBox::to_rect in usvg src/tree/geom.rs. It converts a bounding
// box into [`Rect`].
func (b bbox) toRect() (Rect, bool) {
	if !b.isDefault() {
		return path.RectFromLTRB(b.left, b.top, b.right, b.bottom)
	}
	return Rect{}, false
}

// toNonZeroRect is BBox::to_non_zero_rect in usvg src/tree/geom.rs. It
// converts a bounding box into [`NonZeroRect`].
func (b bbox) toNonZeroRect() (NonZeroRect, bool) {
	if !b.isDefault() {
		return path.NonZeroRectFromLTRB(b.left, b.top, b.right, b.bottom)
	}
	return NonZeroRect{}, false
}

// alignedPos is aligned_pos in usvg src/tree/geom.rs. It returns object
// aligned position.
func alignedPos(align svgtypes.Align, x, y, w, h float32) (float32, float32) {
	switch align {
	case svgtypes.AlignNone:
		return x, y
	case svgtypes.AlignXMinYMin:
		return x, y
	case svgtypes.AlignXMidYMin:
		return x + w/2.0, y
	case svgtypes.AlignXMaxYMin:
		return x + w, y
	case svgtypes.AlignXMinYMid:
		return x, y + h/2.0
	case svgtypes.AlignXMidYMid:
		return x + w/2.0, y + h/2.0
	case svgtypes.AlignXMaxYMid:
		return x + w, y + h/2.0
	case svgtypes.AlignXMinYMax:
		return x, y + h
	case svgtypes.AlignXMidYMax:
		return x + w/2.0, y + h
	case svgtypes.AlignXMaxYMax:
		return x + w, y + h
	}
	return x, y
}

// minF32 is f32::min. If one operand is NaN, it returns the other one.
//
// It follows the code that rustc makes for x86-64, as the function minF32 of
// the package path does: the result is x if x < y, and y if not. If that
// result is NaN, the result is x.
func minF32(x, y float32) float32 {
	if y != y {
		return x
	}
	if x < y {
		return x
	}
	return y
}

// maxF32 is f32::max. If one operand is NaN, it returns the other one.
//
// It follows the code that rustc makes for x86-64, as the function maxF32 of
// the package path does: the result is x if x > y, and y if not. If that
// result is NaN, the result is x.
func maxF32(x, y float32) float32 {
	if y != y {
		return x
	}
	if x > y {
		return x
	}
	return y
}
