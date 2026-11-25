// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package filter

import (
	"math"

	"github.com/xo/okage/internal/wide"
)

// This file ports the parts of resvg src/filter/mod.rs that the blurs, the
// color matrix and the transfer functions use. svg keeps its own port of the
// whole module, for the other primitives (D88).
//
// The type RGBA8 of the crate rgb is rgba8. Upstream views the bytes of a
// pixmap as a slice of RGBA8 with bytemuck. Go cannot do that without the
// package unsafe (D25), so imageRefMut holds the bytes, and a loop over the
// pixels steps 4 bytes at a time.

// rgba8 is the type RGBA8 of the crate rgb: a pixel of 4 bytes.
type rgba8 struct {
	r, g, b, a uint8
}

// rgba8At returns the pixel at index i of data, which holds RGBA bytes. It is
// data[i] of a Rust slice of RGBA8. It ports no upstream function.
func rgba8At(data []byte, i int) rgba8 {
	p := data[i*4 : i*4+4 : i*4+4]
	return rgba8{r: p[0], g: p[1], b: p[2], a: p[3]}
}

// setRGBA8At sets the pixel at index i of data to p. It is data[i] = p of a
// Rust slice of RGBA8. It ports no upstream function.
func setRGBA8At(data []byte, i int, p rgba8) {
	d := data[i*4 : i*4+4 : i*4+4]
	d[0], d[1], d[2], d[3] = p.r, p.g, p.b, p.a
}

// imageRefMut is ImageRefMut in resvg src/filter/mod.rs: the RGBA bytes of
// an image and its size.
type imageRefMut struct {
	data   []byte
	width  uint32
	height uint32
}

// newImageRefMut is ImageRefMut::new in resvg src/filter/mod.rs.
func newImageRefMut(width, height uint32, data []byte) imageRefMut {
	return imageRefMut{data: data, width: width, height: height}
}

// multiplyAlpha is multiply_alpha in resvg src/filter/mod.rs. It multiplies
// provided pixels alpha.
func multiplyAlpha(data []byte) {
	for i := 0; i+4 <= len(data); i += 4 {
		p := data[i : i+4 : i+4]
		a := float32(p[3]) / 255.0
		p[2] = wide.TruncU8(float32(float32(p[2])*a) + 0.5)
		p[1] = wide.TruncU8(float32(float32(p[1])*a) + 0.5)
		p[0] = wide.TruncU8(float32(float32(p[0])*a) + 0.5)
	}
}

// demultiplyAlpha is demultiply_alpha in resvg src/filter/mod.rs. It
// demultiplies provided pixels alpha.
func demultiplyAlpha(data []byte) {
	for i := 0; i+4 <= len(data); i += 4 {
		p := data[i : i+4 : i+4]
		a := float32(p[3]) / 255.0
		p[2] = wide.TruncU8(float32(float32(p[2])/a) + 0.5)
		p[1] = wide.TruncU8(float32(float32(p[1])/a) + 0.5)
		p[0] = wide.TruncU8(float32(float32(p[0])/a) + 0.5)
	}
}

// pixmapIntoLinearRGB is PixmapExt::into_linear_rgb in resvg
// src/filter/mod.rs, on the bytes of a pixmap.
func pixmapIntoLinearRGB(data []byte) {
	demultiplyAlpha(data)
	intoLinearRGB(data)
	multiplyAlpha(data)
}

// pixmapIntoSRGB is PixmapExt::into_srgb in resvg src/filter/mod.rs, on the
// bytes of a pixmap.
func pixmapIntoSRGB(data []byte) {
	demultiplyAlpha(data)
	fromLinearRGB(data)
	multiplyAlpha(data)
}

// srgbToLinearRGBTable is SRGB_TO_LINEAR_RGB_TABLE in resvg
// src/filter/mod.rs. It is a precomputed sRGB to LinearRGB table.
//
// Since we are storing the result in `u8`, there is no need to compute those
// values each time. Mainly because it's very expensive.
//
//	if (C_srgb <= 0.04045)
//	    C_lin = C_srgb / 12.92;
//	 else
//	    C_lin = pow((C_srgb + 0.055) / 1.055, 2.4);
//
// Thanks to librsvg for the idea.
var srgbToLinearRGBTable = [256]uint8{ //nolint:dupl // the two tables of upstream
	0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3,
	4, 4, 4, 4, 4, 5, 5, 5, 5, 6, 6, 6, 6, 7, 7, 7,
	8, 8, 8, 8, 9, 9, 9, 10, 10, 10, 11, 11, 12, 12, 12, 13,
	13, 13, 14, 14, 15, 15, 16, 16, 17, 17, 17, 18, 18, 19, 19, 20,
	20, 21, 22, 22, 23, 23, 24, 24, 25, 25, 26, 27, 27, 28, 29, 29,
	30, 30, 31, 32, 32, 33, 34, 35, 35, 36, 37, 37, 38, 39, 40, 41,
	41, 42, 43, 44, 45, 45, 46, 47, 48, 49, 50, 51, 51, 52, 53, 54,
	55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70,
	71, 72, 73, 74, 76, 77, 78, 79, 80, 81, 82, 84, 85, 86, 87, 88,
	90, 91, 92, 93, 95, 96, 97, 99, 100, 101, 103, 104, 105, 107, 108, 109,
	111, 112, 114, 115, 116, 118, 119, 121, 122, 124, 125, 127, 128, 130, 131, 133,
	134, 136, 138, 139, 141, 142, 144, 146, 147, 149, 151, 152, 154, 156, 157, 159,
	161, 163, 164, 166, 168, 170, 171, 173, 175, 177, 179, 181, 183, 184, 186, 188,
	190, 192, 194, 196, 198, 200, 202, 204, 206, 208, 210, 212, 214, 216, 218, 220,
	222, 224, 226, 229, 231, 233, 235, 237, 239, 242, 244, 246, 248, 250, 253, 255,
}

// linearRGBToSRGBTable is LINEAR_RGB_TO_SRGB_TABLE in resvg
// src/filter/mod.rs. It is a precomputed LinearRGB to sRGB table.
//
// Since we are storing the result in `u8`, there is no need to compute those
// values each time. Mainly because it's very expensive.
//
//	if (C_lin <= 0.0031308)
//	    C_srgb = C_lin * 12.92;
//	else
//	    C_srgb = 1.055 * pow(C_lin, 1.0 / 2.4) - 0.055;
//
// Thanks to librsvg for the idea.
var linearRGBToSRGBTable = [256]uint8{ //nolint:dupl // the two tables of upstream
	0, 13, 22, 28, 34, 38, 42, 46, 50, 53, 56, 59, 61, 64, 66, 69,
	71, 73, 75, 77, 79, 81, 83, 85, 86, 88, 90, 92, 93, 95, 96, 98,
	99, 101, 102, 104, 105, 106, 108, 109, 110, 112, 113, 114, 115, 117, 118, 119,
	120, 121, 122, 124, 125, 126, 127, 128, 129, 130, 131, 132, 133, 134, 135, 136,
	137, 138, 139, 140, 141, 142, 143, 144, 145, 146, 147, 148, 148, 149, 150, 151,
	152, 153, 154, 155, 155, 156, 157, 158, 159, 159, 160, 161, 162, 163, 163, 164,
	165, 166, 167, 167, 168, 169, 170, 170, 171, 172, 173, 173, 174, 175, 175, 176,
	177, 178, 178, 179, 180, 180, 181, 182, 182, 183, 184, 185, 185, 186, 187, 187,
	188, 189, 189, 190, 190, 191, 192, 192, 193, 194, 194, 195, 196, 196, 197, 197,
	198, 199, 199, 200, 200, 201, 202, 202, 203, 203, 204, 205, 205, 206, 206, 207,
	208, 208, 209, 209, 210, 210, 211, 212, 212, 213, 213, 214, 214, 215, 215, 216,
	216, 217, 218, 218, 219, 219, 220, 220, 221, 221, 222, 222, 223, 223, 224, 224,
	225, 226, 226, 227, 227, 228, 228, 229, 229, 230, 230, 231, 231, 232, 232, 233,
	233, 234, 234, 235, 235, 236, 236, 237, 237, 238, 238, 238, 239, 239, 240, 240,
	241, 241, 242, 242, 243, 243, 244, 244, 245, 245, 246, 246, 246, 247, 247, 248,
	248, 249, 249, 250, 250, 251, 251, 251, 252, 252, 253, 253, 254, 254, 255, 255,
}

// intoLinearRGB is into_linear_rgb in resvg src/filter/mod.rs. It converts
// input pixel from sRGB into LinearRGB.
//
// Provided pixels should have an **unpremultiplied alpha**.
//
// RGB channels order of the input image doesn't matter, but alpha channel must be the last one.
func intoLinearRGB(data []byte) {
	for i := 0; i+4 <= len(data); i += 4 {
		data[i+0] = srgbToLinearRGBTable[data[i+0]]
		data[i+1] = srgbToLinearRGBTable[data[i+1]]
		data[i+2] = srgbToLinearRGBTable[data[i+2]]
	}
}

// fromLinearRGB is from_linear_rgb in resvg src/filter/mod.rs. It converts
// input pixel from LinearRGB into sRGB.
//
// Provided pixels should have an **unpremultiplied alpha**.
//
// RGB channels order of the input image doesn't matter, but alpha channel must be the last one.
func fromLinearRGB(data []byte) {
	for i := 0; i+4 <= len(data); i += 4 {
		data[i+0] = linearRGBToSRGBTable[data[i+0]]
		data[i+1] = linearRGBToSRGBTable[data[i+1]]
		data[i+2] = linearRGBToSRGBTable[data[i+2]]
	}
}

// f32Bound is f32_bound in resvg src/filter/mod.rs.
//
// TODO: https://github.com/rust-lang/rust/issues/44095
func f32Bound(minimum, val, maximum float32) float32 {
	switch {
	case val > maximum:
		return maximum
	case val >= minimum:
		return val
	}
	// Catches `val < min` as well as a NaN `val`.
	return minimum
}

// resolveStdDev is resolve_std_dev in resvg src/filter/mod.rs, after
// scale_coordinates. It takes the standard deviations that the transform
// already scaled.
//
// If the third result is set, then a box blur should be used. Or IIR
// otherwise.
//
// Port: the last result is false where upstream returns None.
func resolveStdDev(stdDx, stdDy float32) (float64, float64, bool, bool) {
	// 'A negative value or a value of zero disables the effect of the given filter primitive
	// (i.e., the result is the filter input image).'
	if approxEqUlps32(stdDx, 0.0, 4) && approxEqUlps32(stdDy, 0.0, 4) {
		return 0, 0, false, false
	}

	// Ignore tiny sigmas. In case of IIR blur it can lead to a transparent image.
	if stdDx < 0.05 {
		stdDx = 0.0
	}

	if stdDy < 0.05 {
		stdDy = 0.0
	}

	const blurSigmaThreshold float32 = 2.0
	// Check that the current feGaussianBlur filter can be applied using a box blur.
	boxBlur := stdDx >= blurSigmaThreshold || stdDy >= blurSigmaThreshold

	return float64(stdDx), float64(stdDy), boxBlur, true
}

// approxEqUlps32 is the impl ApproxEqUlps for f32 of the crate float-cmp
// 0.9.0, src/ulps_eq.rs, as ApproxEqUlps32 of svg. It reports whether x and
// other are within ulps units of least precision of each other.
func approxEqUlps32(x, other float32, ulps int32) bool {
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
