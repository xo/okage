// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package filter

import "github.com/xo/okage/internal/wide"

// This file ports no file of resvg. It holds the box blur and the IIR blur
// of resvg for an image of one channel, such as a mask (D88). Each function
// is the function of box_blur.rs or iir_blur.rs that its comment names, with
// one byte for each pixel in place of four. Each channel of the blurs of
// resvg is blurred alone, so a channel gets the same bytes.

// applyAlpha is apply of box_blur.rs for an image of one channel.
func applyAlpha(sigmaX, sigmaY float64, data []byte, width, height int) {
	boxesHorz := createBoxGauss(float32(sigmaX))
	boxesVert := createBoxGauss(float32(sigmaY))
	backbuf := append([]byte(nil), data...)

	for i := range boxesHorz {
		boxSizeHorz, boxSizeVert := boxesHorz[i], boxesVert[i]
		radiusHorz := int((boxSizeHorz - 1) / 2)
		radiusVert := int((boxSizeVert - 1) / 2)
		// box_blur_impl
		boxBlurVertAlpha(radiusVert, data, backbuf, width, height)
		boxBlurHorzAlpha(radiusHorz, backbuf, data, width, height)
	}
}

// boxBlurVertAlpha is box_blur_vert of box_blur.rs for an image of one
// channel. The pixels outside the image are 0.
func boxBlurVertAlpha(blurRadius int, backbuf, frontbuf []byte, width, height int) {
	if blurRadius == 0 {
		copy(frontbuf, backbuf)
		return
	}

	iarr := 1.0 / float32(blurRadius+blurRadius+1)

	for i := range width {
		colEnd := i + width*(height-1) // inclusive
		ti := i
		li := ti
		// As in boxBlurVert, the minimum keeps the product from an overflow.
		ri := ti + min(blurRadius, height)*width

		val := 0
		for j := range min(blurRadius, height) {
			val += int(backbuf[ti+j*width])
		}

		for range min(height, blurRadius+1) {
			if ri <= colEnd {
				val += int(backbuf[ri])
			}
			ri += width
			frontbuf[ti] = wide.TruncU8(round(float32(float32(val) * iarr)))
			ti += width
		}

		if height <= blurRadius {
			continue
		}

		for range (height - blurRadius) - (blurRadius + 1) {
			val += int(backbuf[ri]) - int(backbuf[li])
			ri += width
			li += width
			frontbuf[ti] = wide.TruncU8(round(float32(float32(val) * iarr)))
			ti += width
		}

		for range min(height-blurRadius-1, blurRadius) {
			if li >= i {
				val -= int(backbuf[li])
			}
			li += width
			frontbuf[ti] = wide.TruncU8(round(float32(float32(val) * iarr)))
			ti += width
		}
	}
}

// boxBlurHorzAlpha is box_blur_horz of box_blur.rs for an image of one
// channel. The pixels outside the image are 0.
func boxBlurHorzAlpha(blurRadius int, backbuf, frontbuf []byte, width, height int) {
	if blurRadius == 0 {
		copy(frontbuf, backbuf)
		return
	}

	iarr := 1.0 / float32(blurRadius+blurRadius+1)

	for i := range height {
		rowStart := i * width     // inclusive
		rowEnd := (i+1)*width - 1 // inclusive
		ti := rowStart
		li := ti
		ri := ti + blurRadius

		val := 0
		for j := range min(blurRadius, width) {
			val += int(backbuf[ti+j])
		}

		for range min(width, blurRadius+1) {
			if ri <= rowEnd {
				val += int(backbuf[ri])
			}
			ri++
			frontbuf[ti] = wide.TruncU8(round(float32(float32(val) * iarr)))
			ti++
		}

		if width <= blurRadius {
			continue
		}

		for range (width - blurRadius) - (blurRadius + 1) {
			val += int(backbuf[ri]) - int(backbuf[li])
			ri++
			li++
			frontbuf[ti] = wide.TruncU8(round(float32(float32(val) * iarr)))
			ti++
		}

		for range min(width-blurRadius-1, blurRadius) {
			if li >= rowStart {
				val -= int(backbuf[li])
			}
			li++
			frontbuf[ti] = wide.TruncU8(round(float32(float32(val) * iarr)))
			ti++
		}
	}
}

// applyIIRBlurAlpha is apply of iir_blur.rs for an image of one channel.
func applyIIRBlurAlpha(sigmaX, sigmaY float64, data []byte, width, height int) {
	buf := make([]float64, width*height)

	d := blurData{
		width:  width,
		height: height,
		sigmaX: sigmaX,
		sigmaY: sigmaY,
		steps:  4,
	}

	// gaussian_channel
	for i := range buf {
		buf[i] = float64(data[i]) / 255.0
	}

	gaussianiir2d(&d, buf)

	for i := range buf {
		data[i] = asU8F64(buf[i] * 255.0)
	}
}
