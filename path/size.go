// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import "github.com/xo/okage/internal/wide"

// IntSize is IntSize in tiny-skia-path path/src/size.rs. It is an integer
// size.
//
// The width and the height are guaranteed to be positive and not zero.
type IntSize struct {
	width  lengthU32
	height lengthU32
}

// IntSizeFromWH is IntSize::from_wh in tiny-skia-path path/src/size.rs. It
// creates a new IntSize from a width and a height.
func IntSizeFromWH(width, height uint32) (IntSize, bool) {
	if width == 0 || height == 0 {
		return IntSize{}, false
	}
	return IntSize{width: width, height: height}, true
}

// intSizeFromWHSafe is IntSize::from_wh_safe in tiny-skia-path
// path/src/size.rs.
func intSizeFromWHSafe(width, height lengthU32) IntSize {
	return IntSize{width: width, height: height}
}

// Width is IntSize::width in tiny-skia-path path/src/size.rs. It returns the
// width.
func (s IntSize) Width() uint32 {
	return s.width
}

// Height is IntSize::height in tiny-skia-path path/src/size.rs. It returns the
// height.
func (s IntSize) Height() uint32 {
	return s.height
}

// Dimensions is IntSize::dimensions in tiny-skia-path path/src/size.rs. It
// returns the width and the height.
func (s IntSize) Dimensions() (uint32, uint32) {
	return s.Width(), s.Height()
}

// ScaleBy is IntSize::scale_by in tiny-skia-path path/src/size.rs. It scales
// the size by the given factor.
func (s IntSize) ScaleBy(factor float32) (IntSize, bool) {
	return IntSizeFromWH(
		wide.TruncU32(roundF32(float32(float32(s.Width())*factor))),
		wide.TruncU32(roundF32(float32(float32(s.Height())*factor))),
	)
}

// ScaleTo is IntSize::scale_to in tiny-skia-path path/src/size.rs. It scales
// the size to the given size.
func (s IntSize) ScaleTo(to IntSize) IntSize {
	return sizeScale(s, to, false)
}

// ScaleToWidth is IntSize::scale_to_width in tiny-skia-path path/src/size.rs.
// It scales the size to the given width.
func (s IntSize) ScaleToWidth(newWidth uint32) (IntSize, bool) {
	newHeight := ceilF32(float32(float32(newWidth)*float32(s.Height())) / float32(s.Width()))
	return IntSizeFromWH(newWidth, wide.TruncU32(newHeight))
}

// ScaleToHeight is IntSize::scale_to_height in tiny-skia-path
// path/src/size.rs. It scales the size to the given height.
func (s IntSize) ScaleToHeight(newHeight uint32) (IntSize, bool) {
	newWidth := ceilF32(float32(float32(newHeight)*float32(s.Width())) / float32(s.Height()))
	return IntSizeFromWH(wide.TruncU32(newWidth), newHeight)
}

// ToSize is IntSize::to_size in tiny-skia-path path/src/size.rs. It converts
// the size into a Size.
func (s IntSize) ToSize() Size {
	size, ok := SizeFromWH(float32(s.Width()), float32(s.Height()))
	if !ok {
		panic("path: IntSize.ToSize: Size::from_wh returned None")
	}
	return size
}

// ToIntRect is IntSize::to_int_rect in tiny-skia-path path/src/size.rs. It
// converts the size into an IntRect at the given position.
func (s IntSize) ToIntRect(x, y int32) IntRect {
	r, ok := IntRectFromXYWH(x, y, s.Width(), s.Height())
	if !ok {
		panic("path: IntSize.ToIntRect: IntRect::from_xywh returned None")
	}
	return r
}

// sizeScale is size_scale in tiny-skia-path path/src/size.rs.
func sizeScale(s1, s2 IntSize, expand bool) IntSize {
	rw := wide.TruncU32(ceilF32(float32(float32(s2.Height())*float32(s1.Width())) / float32(s1.Height())))
	var withH bool
	if expand {
		withH = rw <= s2.Width()
	} else {
		withH = rw >= s2.Width()
	}

	if !withH {
		r, ok := IntSizeFromWH(rw, s2.Height())
		if !ok {
			panic("path: size_scale: IntSize::from_wh returned None")
		}
		return r
	}
	h := wide.TruncU32(ceilF32(float32(float32(s2.Width())*float32(s1.Height())) / float32(s1.Width())))
	r, ok := IntSizeFromWH(s2.Width(), h)
	if !ok {
		panic("path: size_scale: IntSize::from_wh returned None")
	}
	return r
}

// Size is Size in tiny-skia-path path/src/size.rs. It is a size.
//
// The width and the height are guaranteed to be positive, not zero and
// finite.
type Size struct {
	width  NonZeroPositiveF32
	height NonZeroPositiveF32
}

// SizeFromWH is Size::from_wh in tiny-skia-path path/src/size.rs. It creates a
// new Size from a width and a height.
func SizeFromWH(width, height float32) (Size, bool) {
	w, ok := NewNonZeroPositiveF32(width)
	if !ok {
		return Size{}, false
	}
	h, ok := NewNonZeroPositiveF32(height)
	if !ok {
		return Size{}, false
	}
	return Size{width: w, height: h}, true
}

// Width is Size::width in tiny-skia-path path/src/size.rs. It returns the
// width.
func (s Size) Width() float32 {
	return s.width.Get()
}

// Height is Size::height in tiny-skia-path path/src/size.rs. It returns the
// height.
func (s Size) Height() float32 {
	return s.height.Get()
}

// ScaleTo is Size::scale_to in tiny-skia-path path/src/size.rs. It scales the
// size to the given size.
func (s Size) ScaleTo(to Size) Size {
	return sizeScaleF64(s, to, false)
}

// ExpandTo is Size::expand_to in tiny-skia-path path/src/size.rs. It expands
// the size to the given size.
func (s Size) ExpandTo(to Size) Size {
	return sizeScaleF64(s, to, true)
}

// ScaleBy is Size::scale_by in tiny-skia-path path/src/size.rs. It scales the
// size by the given factor.
func (s Size) ScaleBy(factor float32) (Size, bool) {
	return SizeFromWH(float32(s.Width()*factor), float32(s.Height()*factor))
}

// ScaleToWidth is Size::scale_to_width in tiny-skia-path path/src/size.rs. It
// scales the size to the given width.
func (s Size) ScaleToWidth(newWidth float32) (Size, bool) {
	newHeight := float32(newWidth*s.Height()) / s.Width()
	return SizeFromWH(newWidth, newHeight)
}

// ScaleToHeight is Size::scale_to_height in tiny-skia-path path/src/size.rs.
// It scales the size to the given height.
func (s Size) ScaleToHeight(newHeight float32) (Size, bool) {
	newWidth := float32(newHeight*s.Width()) / s.Height()
	return SizeFromWH(newWidth, newHeight)
}

// ToIntSize is Size::to_int_size in tiny-skia-path path/src/size.rs. It
// converts the size into an IntSize.
func (s Size) ToIntSize() IntSize {
	r, ok := IntSizeFromWH(
		max(1, wide.TruncU32(roundF32(s.Width()))),
		max(1, wide.TruncU32(roundF32(s.Height()))),
	)
	if !ok {
		panic("path: Size.ToIntSize: IntSize::from_wh returned None")
	}
	return r
}

// ToRect is Size::to_rect in tiny-skia-path path/src/size.rs. It converts the
// size into a Rect at the given position.
func (s Size) ToRect(x, y float32) (Rect, bool) {
	return RectFromXYWH(x, y, s.width.Get(), s.height.Get())
}

// ToNonZeroRect is Size::to_non_zero_rect in tiny-skia-path path/src/size.rs.
// It converts the size into a NonZeroRect at the given position.
func (s Size) ToNonZeroRect(x, y float32) NonZeroRect {
	r, ok := NonZeroRectFromXYWH(x, y, s.width.Get(), s.height.Get())
	if !ok {
		panic("path: Size.ToNonZeroRect: NonZeroRect::from_xywh returned None")
	}
	return r
}

// sizeScaleF64 is size_scale_f64 in tiny-skia-path path/src/size.rs.
//
// Port: despite its name, upstream computes in f32.
func sizeScaleF64(s1, s2 Size, expand bool) Size {
	rw := float32(s2.height.Get()*s1.width.Get()) / s1.height.Get()
	var withH bool
	if expand {
		withH = rw <= s2.width.Get()
	} else {
		withH = rw >= s2.width.Get()
	}
	if !withH {
		r, ok := SizeFromWH(rw, s2.height.Get())
		if !ok {
			panic("path: size_scale_f64: Size::from_wh returned None")
		}
		return r
	}
	h := float32(s2.width.Get()*s1.height.Get()) / s1.width.Get()
	r, ok := SizeFromWH(s2.width.Get(), h)
	if !ok {
		panic("path: size_scale_f64: Size::from_wh returned None")
	}
	return r
}
