// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"math"

	"github.com/xo/okage/path"
)

// screenIntRect is ScreenIntRect in tiny-skia src/geom.rs. It is a screen
// IntRect.
//
// These facts are guaranteed:
//   - X and Y are in the range 0..=i32::MAX.
//   - The width and the height are in the range 1..=i32::MAX.
//   - x+width and y+height do not overflow.
type screenIntRect struct {
	x      uint32
	y      uint32
	width  lengthU32
	height lengthU32
}

// screenIntRectFromXYWH is ScreenIntRect::from_xywh in tiny-skia src/geom.rs.
// It creates a new screenIntRect.
func screenIntRectFromXYWH(x, y, width, height uint32) (screenIntRect, bool) {
	if x > math.MaxInt32 || y > math.MaxInt32 || width > math.MaxInt32 || height > math.MaxInt32 {
		return screenIntRect{}, false
	}

	if _, ok := checkedAddU32(x, width); !ok {
		return screenIntRect{}, false
	}
	if _, ok := checkedAddU32(y, height); !ok {
		return screenIntRect{}, false
	}

	if width == 0 || height == 0 {
		return screenIntRect{}, false
	}

	return screenIntRect{x: x, y: y, width: width, height: height}, true
}

// screenIntRectFromXYWHSafe is ScreenIntRect::from_xywh_safe in tiny-skia
// src/geom.rs. It creates a new screenIntRect.
func screenIntRectFromXYWHSafe(x, y uint32, width, height lengthU32) screenIntRect {
	return screenIntRect{x: x, y: y, width: width, height: height}
}

// X is ScreenIntRect::x in tiny-skia src/geom.rs. It returns the X position of
// the rect.
func (r screenIntRect) X() uint32 {
	return r.x
}

// Y is ScreenIntRect::y in tiny-skia src/geom.rs. It returns the Y position of
// the rect.
func (r screenIntRect) Y() uint32 {
	return r.y
}

// Width is ScreenIntRect::width in tiny-skia src/geom.rs. It returns the width
// of the rect.
func (r screenIntRect) Width() uint32 {
	return r.width
}

// Height is ScreenIntRect::height in tiny-skia src/geom.rs. It returns the
// height of the rect.
func (r screenIntRect) Height() uint32 {
	return r.height
}

// WidthSafe is ScreenIntRect::width_safe in tiny-skia src/geom.rs. It returns
// the width of the rect.
func (r screenIntRect) WidthSafe() lengthU32 {
	return r.width
}

// Left is ScreenIntRect::left in tiny-skia src/geom.rs. It returns the left
// edge of the rect.
func (r screenIntRect) Left() uint32 {
	return r.x
}

// Top is ScreenIntRect::top in tiny-skia src/geom.rs. It returns the top edge
// of the rect.
func (r screenIntRect) Top() uint32 {
	return r.y
}

// Right is ScreenIntRect::right in tiny-skia src/geom.rs. It returns the right
// edge of the rect.
//
// The right edge is at least 1.
func (r screenIntRect) Right() uint32 {
	// No overflow is guaranteed by constructors.
	return r.x + r.width
}

// Bottom is ScreenIntRect::bottom in tiny-skia src/geom.rs. It returns the
// bottom edge of the rect.
//
// The bottom edge is at least 1.
func (r screenIntRect) Bottom() uint32 {
	// No overflow is guaranteed by constructors.
	return r.y + r.height
}

// Size is ScreenIntRect::size in tiny-skia src/geom.rs. It returns the size of
// the rect.
func (r screenIntRect) Size() path.IntSize {
	s, ok := path.IntSizeFromWH(r.Width(), r.Height())
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}
	return s
}

// Contains is ScreenIntRect::contains in tiny-skia src/geom.rs. It checks that
// the rect completely includes other.
func (r screenIntRect) Contains(other screenIntRect) bool {
	return r.x <= other.x &&
		r.y <= other.y &&
		r.Right() >= other.Right() &&
		r.Bottom() >= other.Bottom()
}

// ToIntRect is ScreenIntRect::to_int_rect in tiny-skia src/geom.rs. It
// converts the rect into an IntRect.
func (r screenIntRect) ToIntRect() path.IntRect {
	// Everything is already checked by constructors.
	ir, ok := path.IntRectFromXYWH(int32(r.x), int32(r.y), r.width, r.height)
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}
	return ir
}

// ToRect is ScreenIntRect::to_rect in tiny-skia src/geom.rs. It converts the
// rect into a Rect.
func (r screenIntRect) ToRect() path.Rect {
	// Cannot fail, because `ScreenIntRect` is always valid.
	// And u32 always fits into f32.
	rect, ok := path.RectFromLTRB(
		float32(r.x),
		float32(r.y),
		float32(r.x)+float32(r.width),
		float32(r.y)+float32(r.height),
	)
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}
	return rect
}

// intSizeToScreenIntRect is IntSizeExt::to_screen_int_rect in tiny-skia
// src/geom.rs. It converts the size into a screenIntRect at the given
// position.
//
// Port: Go cannot add a method to a type of another package, so the method of
// the trait is a function.
func intSizeToScreenIntRect(s path.IntSize, x, y uint32) screenIntRect { //nolint:unparam // the signature of upstream is kept
	r, ok := screenIntRectFromXYWH(x, y, s.Width(), s.Height())
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}
	return r
}

// intRectToScreenIntRect is IntRectExt::to_screen_int_rect in tiny-skia
// src/geom.rs. It converts the rect into a screenIntRect.
//
// It checks that x >= 0 and y >= 0.
//
// Port: Go cannot add a method to a type of another package, so the method of
// the trait is a function.
func intRectToScreenIntRect(r path.IntRect) (screenIntRect, bool) {
	if r.X() < 0 || r.Y() < 0 {
		return screenIntRect{}, false
	}
	return screenIntRectFromXYWH(uint32(r.X()), uint32(r.Y()), r.Width(), r.Height())
}
