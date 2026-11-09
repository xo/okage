// Copyright 2020 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"github.com/xo/okage/internal/wide"
)

// This file ports resvg src/filter/box_blur.rs.
//
// Based on https://github.com/fschutt/fastblur
//
// Port: a Rust usize stays an int, and an isize an int, which has 64 bits on
// the platforms of Rust that resvg runs on and 32 bits with GOARCH=386. The
// sums of a column or a row stay far below 2^31.

// steps is STEPS in resvg src/filter/box_blur.rs.
const steps = 5

// applyFilterBoxBlur is apply in resvg src/filter/box_blur.rs. It applies a
// box blur.
//
// Input image pixels should have a **premultiplied alpha**.
//
// A negative or zero `sigma_x`/`sigma_y` will disable the blur along that axis.
//
// # Allocations
//
// This method will allocate a copy of the `src` image as a back buffer.
func applyFilterBoxBlur(sigmaX, sigmaY float64, src imageRefMut) {
	boxesHorz := createBoxGauss(float32(sigmaX))
	boxesVert := createBoxGauss(float32(sigmaY))
	backbufData := append([]byte(nil), src.data...)
	backbuf := newImageRefMut(src.width, src.height, backbufData)

	for i := range boxesHorz {
		boxSizeHorz, boxSizeVert := boxesHorz[i], boxesVert[i]
		radiusHorz := int((boxSizeHorz - 1) / 2)
		radiusVert := int((boxSizeVert - 1) / 2)
		boxBlurImpl(radiusHorz, radiusVert, backbuf, src)
	}
}

// createBoxGauss is create_box_gauss in resvg src/filter/box_blur.rs.
func createBoxGauss(sigma float32) [steps]int32 {
	if sigma > 0.0 {
		const nFloat = float32(steps)

		// Ideal averaging filter width
		wIdeal := sqrtF32(float32(12.0*sigma*sigma)/nFloat) + 1.0
		wl := wide.TruncI32(floorF32(wIdeal))
		if wl%2 == 0 {
			wl--
		}

		wu := wl + 2

		wlFloat := float32(wl)
		mIdeal := (float32(12.0*sigma*sigma) -
			float32(nFloat*wlFloat*wlFloat) -
			float32(4.0*nFloat*wlFloat) -
			float32(3.0*nFloat)) /
			(float32(-4.0*wlFloat) - 4.0)
		m := wide.TruncI64(round32(mIdeal))
		m = max(m, 0)

		var sizes [steps]int32
		for i := range steps {
			if int64(i) < m {
				sizes[i] = wl
			} else {
				sizes[i] = wu
			}
		}

		return sizes
	}
	return [steps]int32{1, 1, 1, 1, 1}
}

// boxBlurImpl is box_blur_impl in resvg src/filter/box_blur.rs.
func boxBlurImpl(
	blurRadiusHorz int,
	blurRadiusVert int,
	backbuf imageRefMut,
	frontbuf imageRefMut,
) {
	boxBlurVert(blurRadiusVert, frontbuf, backbuf)
	boxBlurHorz(blurRadiusHorz, backbuf, frontbuf)
}

// boxBlurVert is box_blur_vert in resvg src/filter/box_blur.rs.
func boxBlurVert(blurRadius int, backbuf imageRefMut, frontbuf imageRefMut) {
	if blurRadius == 0 {
		copy(frontbuf.data, backbuf.data)
		return
	}

	width := int(backbuf.width)
	height := int(backbuf.height)

	iarr := 1.0 / float32(blurRadius+blurRadius+1)
	blurRadiusPrev := blurRadius - height
	blurRadiusNext := blurRadius + 1

	for i := range width {
		colStart := i                  // inclusive
		colEnd := i + width*(height-1) // inclusive
		ti := i
		li := ti
		ri := ti + blurRadius*width

		fv := rgba8{}
		lv := rgba8{}

		valR := blurRadiusNext * int(fv.r)
		valG := blurRadiusNext * int(fv.g)
		valB := blurRadiusNext * int(fv.b)
		valA := blurRadiusNext * int(fv.a)

		// Get the pixel at the specified index, or the first pixel of the column
		// if the index is beyond the top edge of the image
		getTop := func(i int) rgba8 {
			if i < colStart {
				return fv
			}
			return rgba8At(backbuf.data, i)
		}

		// Get the pixel at the specified index, or the last pixel of the column
		// if the index is beyond the bottom edge of the image
		getBottom := func(i int) rgba8 {
			if i > colEnd {
				return lv
			}
			return rgba8At(backbuf.data, i)
		}

		for j := range min(blurRadius, height) {
			bb := rgba8At(backbuf.data, ti+j*width)
			valR += int(bb.r)
			valG += int(bb.g)
			valB += int(bb.b)
			valA += int(bb.a)
		}
		if blurRadius > height {
			valR += blurRadiusPrev * int(lv.r)
			valG += blurRadiusPrev * int(lv.g)
			valB += blurRadiusPrev * int(lv.b)
			valA += blurRadiusPrev * int(lv.a)
		}

		for range min(height, blurRadius+1) {
			bb := getBottom(ri)
			ri += width
			valR += sub(bb.r, fv.r)
			valG += sub(bb.g, fv.g)
			valB += sub(bb.b, fv.b)
			valA += sub(bb.a, fv.a)

			setRGBA8At(frontbuf.data, ti, rgba8{
				r: wide.TruncU8(round(float32(float32(valR) * iarr))),
				g: wide.TruncU8(round(float32(float32(valG) * iarr))),
				b: wide.TruncU8(round(float32(float32(valB) * iarr))),
				a: wide.TruncU8(round(float32(float32(valA) * iarr))),
			})
			ti += width
		}

		if height <= blurRadius {
			// otherwise `(height - blur_radius)` will underflow
			continue
		}

		for range (height - blurRadius) - (blurRadius + 1) {
			bb1 := rgba8At(backbuf.data, ri)
			ri += width
			bb2 := rgba8At(backbuf.data, li)
			li += width

			valR += sub(bb1.r, bb2.r)
			valG += sub(bb1.g, bb2.g)
			valB += sub(bb1.b, bb2.b)
			valA += sub(bb1.a, bb2.a)

			setRGBA8At(frontbuf.data, ti, rgba8{
				r: wide.TruncU8(round(float32(float32(valR) * iarr))),
				g: wide.TruncU8(round(float32(float32(valG) * iarr))),
				b: wide.TruncU8(round(float32(float32(valB) * iarr))),
				a: wide.TruncU8(round(float32(float32(valA) * iarr))),
			})
			ti += width
		}

		for range min(height-blurRadius-1, blurRadius) {
			bb := getTop(li)
			li += width

			valR += sub(lv.r, bb.r)
			valG += sub(lv.g, bb.g)
			valB += sub(lv.b, bb.b)
			valA += sub(lv.a, bb.a)

			setRGBA8At(frontbuf.data, ti, rgba8{
				r: wide.TruncU8(round(float32(float32(valR) * iarr))),
				g: wide.TruncU8(round(float32(float32(valG) * iarr))),
				b: wide.TruncU8(round(float32(float32(valB) * iarr))),
				a: wide.TruncU8(round(float32(float32(valA) * iarr))),
			})
			ti += width
		}
	}
}

// boxBlurHorz is box_blur_horz in resvg src/filter/box_blur.rs.
func boxBlurHorz(blurRadius int, backbuf imageRefMut, frontbuf imageRefMut) {
	if blurRadius == 0 {
		copy(frontbuf.data, backbuf.data)
		return
	}

	width := int(backbuf.width)
	height := int(backbuf.height)

	iarr := 1.0 / float32(blurRadius+blurRadius+1)
	blurRadiusPrev := blurRadius - width
	blurRadiusNext := blurRadius + 1

	for i := range height {
		rowStart := i * width     // inclusive
		rowEnd := (i+1)*width - 1 // inclusive
		ti := i * width           // VERTICAL: $i;
		li := ti
		ri := ti + blurRadius

		fv := rgba8{}
		lv := rgba8{}

		valR := blurRadiusNext * int(fv.r)
		valG := blurRadiusNext * int(fv.g)
		valB := blurRadiusNext * int(fv.b)
		valA := blurRadiusNext * int(fv.a)

		// Get the pixel at the specified index, or the first pixel of the row
		// if the index is beyond the left edge of the image
		getLeft := func(i int) rgba8 {
			if i < rowStart {
				return fv
			}
			return rgba8At(backbuf.data, i)
		}

		// Get the pixel at the specified index, or the last pixel of the row
		// if the index is beyond the right edge of the image
		getRight := func(i int) rgba8 {
			if i > rowEnd {
				return lv
			}
			return rgba8At(backbuf.data, i)
		}

		for j := range min(blurRadius, width) {
			bb := rgba8At(backbuf.data, ti+j) // VERTICAL: ti + j * width
			valR += int(bb.r)
			valG += int(bb.g)
			valB += int(bb.b)
			valA += int(bb.a)
		}
		if blurRadius > width {
			valR += blurRadiusPrev * int(lv.r)
			valG += blurRadiusPrev * int(lv.g)
			valB += blurRadiusPrev * int(lv.b)
			valA += blurRadiusPrev * int(lv.a)
		}

		// Process the left side where we need pixels from beyond the left edge
		for range min(width, blurRadius+1) {
			bb := getRight(ri)
			ri++
			valR += sub(bb.r, fv.r)
			valG += sub(bb.g, fv.g)
			valB += sub(bb.b, fv.b)
			valA += sub(bb.a, fv.a)

			setRGBA8At(frontbuf.data, ti, rgba8{
				r: wide.TruncU8(round(float32(float32(valR) * iarr))),
				g: wide.TruncU8(round(float32(float32(valG) * iarr))),
				b: wide.TruncU8(round(float32(float32(valB) * iarr))),
				a: wide.TruncU8(round(float32(float32(valA) * iarr))),
			})
			ti++ // VERTICAL : ti += width, same with the other areas
		}

		if width <= blurRadius {
			// otherwise `(width - blur_radius)` will underflow
			continue
		}

		// Process the middle where we know we won't bump into borders
		// without the extra indirection of get_left/get_right. This is faster.
		for range (width - blurRadius) - (blurRadius + 1) {
			bb1 := rgba8At(backbuf.data, ri)
			ri++
			bb2 := rgba8At(backbuf.data, li)
			li++

			valR += sub(bb1.r, bb2.r)
			valG += sub(bb1.g, bb2.g)
			valB += sub(bb1.b, bb2.b)
			valA += sub(bb1.a, bb2.a)

			setRGBA8At(frontbuf.data, ti, rgba8{
				r: wide.TruncU8(round(float32(float32(valR) * iarr))),
				g: wide.TruncU8(round(float32(float32(valG) * iarr))),
				b: wide.TruncU8(round(float32(float32(valB) * iarr))),
				a: wide.TruncU8(round(float32(float32(valA) * iarr))),
			})
			ti++
		}

		// Process the right side where we need pixels from beyond the right edge
		for range min(width-blurRadius-1, blurRadius) {
			bb := getLeft(li)
			li++

			valR += sub(lv.r, bb.r)
			valG += sub(lv.g, bb.g)
			valB += sub(lv.b, bb.b)
			valA += sub(lv.a, bb.a)

			setRGBA8At(frontbuf.data, ti, rgba8{
				r: wide.TruncU8(round(float32(float32(valR) * iarr))),
				g: wide.TruncU8(round(float32(float32(valG) * iarr))),
				b: wide.TruncU8(round(float32(float32(valB) * iarr))),
				a: wide.TruncU8(round(float32(float32(valA) * iarr))),
			})
			ti++
		}
	}
}

// round is round in resvg src/filter/box_blur.rs. It is a fast rounding for
// x <= 2^23.
// This is orders of magnitude faster than built-in rounding intrinsic.
//
// Source: https://stackoverflow.com/a/42386149/585725
//
// Port: each step is a float32 operation that rounds, as in Rust. The
// conversions keep the Go compiler from folding the two steps.
func round(x float32) float32 {
	x = float32(x + 12582912.0)
	x = float32(x - 12582912.0)
	return x
}

// sub is sub in resvg src/filter/box_blur.rs.
func sub(c1, c2 uint8) int {
	return int(c1) - int(c2)
}
