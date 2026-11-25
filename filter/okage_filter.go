// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package filter applies the filters of resvg that a caller can use alone:
// the Gaussian blur of feGaussianBlur, the four kinds of feColorMatrix and
// the five transfer functions of feComponentTransfer. They work on an
// okage.Mask, an okage.Pixmap and an image.NRGBA (D88).
//
// The functions run the code that svg runs for these filter primitives, so
// they give the same bytes as resvg. svg and lottie call this package for
// their blurs and their colors.
//
// No function returns an error. A value out of range is clamped, as in
// resvg.
package filter

import (
	"image"

	"github.com/xo/okage"
)

// This file ports no file of resvg. It holds the API of the package (D88).

// maxSigma is the largest standard deviation that Blur and BlurMask take. A
// larger one blurs as maxSigma, so that the sizes of the boxes of the box
// blur do not overflow an int32, where resvg overflows. The boxes of
// maxSigma are more than a million pixels wide, far wider than any image.
const maxSigma = 1 << 20

// Option configures one call of a function of this package (D20).
type Option func(*options)

// options holds the configuration of one call.
type options struct {
	boxBlur   bool
	linearRGB bool
}

// newOptions applies opts to the default configuration.
func newOptions(opts []Option) options {
	var o options
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

// WithBoxBlur makes Blur and BlurMask take the box blur for each standard
// deviation, as lottie does for the blurs of ThorVG (D79). Without it, a
// standard deviation below 2 on both axes takes the IIR blur, as resvg does.
// resvg has no such field.
func WithBoxBlur() Option {
	return func(o *options) {
		o.boxBlur = true
	}
}

// WithLinearRGB makes Blur and the color functions work in linearRGB, as
// the attribute color-interpolation-filters="linearRGB" of resvg does. The
// function converts the colors from sRGB to linearRGB, applies its filter,
// and converts the colors back to sRGB. Without it, the functions work in
// sRGB.
func WithLinearRGB() Option {
	return func(o *options) {
		o.linearRGB = true
	}
}

// Blur blurs the premultiplied pixels of p in place, as feGaussianBlur of
// resvg does after resolve_std_dev. sigmaX and sigmaY are the standard
// deviations in pixels, and the function rounds them to float32, as resvg
// holds them.
//
// A sigma below 0.05, a negative sigma or a NaN does not blur on its axis. A
// sigma of 2 or more on either axis takes the box blur of resvg, which runs
// five box blurs on each axis. A smaller one takes the IIR blur of resvg, a
// recursive filter that runs along each row and each column. The box blur
// reads 0 outside the pixmap, and the IIR blur reflects the pixmap at its
// edges.
//
// The box blur blurs only the smallest box that holds each pixel that is not
// 0, grown by the sizes of the boxes and cut to the pixmap. This gives the
// same bytes as a blur of the whole pixmap. The IIR blur blurs the whole
// pixmap.
//
// With WithLinearRGB, Blur converts the colors to linearRGB before the blur
// and back to sRGB after it, as resvg does.
func Blur(p *okage.Pixmap, sigmaX, sigmaY float64, opts ...Option) {
	if p == nil {
		return
	}
	o := newOptions(opts)
	sx, sy, box, ok := resolveSigma(sigmaX, sigmaY, o)
	if !ok {
		return
	}
	data := p.DataMut()
	if o.linearRGB {
		pixmapIntoLinearRGB(data)
	}
	if box {
		boxBlurRegion(data, 4, int(p.Width()), int(p.Height()), sx, sy)
	} else {
		applyIIRBlur(sx, sy, newImageRefMut(p.Width(), p.Height(), data))
	}
	if o.linearRGB {
		pixmapIntoSRGB(data)
	}
}

// BlurMask blurs the values of m in place, as Blur blurs the alpha of a
// pixmap. Each value of the result is the alpha of the same pixel after
// Blur on a pixmap whose alpha is m, with the same sigmas and options.
// WithLinearRGB changes no alpha, so BlurMask ignores it.
//
// BlurMask takes the bytes of the mask with DataMut, so the mask holds all
// its pixels after the blur. A mask of okage.NewRectMask turns into pixels
// in the same way.
func BlurMask(m *okage.Mask, sigmaX, sigmaY float64, opts ...Option) {
	if m == nil {
		return
	}
	sx, sy, box, ok := resolveSigma(sigmaX, sigmaY, newOptions(opts))
	if !ok {
		return
	}
	if box {
		boxBlurRegion(m.DataMut(), 1, int(m.Width()), int(m.Height()), sx, sy)
		return
	}
	applyIIRBlurAlpha(sx, sy, m.DataMut(), int(m.Width()), int(m.Height()))
}

// resolveSigma clamps the standard deviations and resolves them, as
// resolveStdDev does. The third result reports a box blur. The last result
// is false where the blur does nothing.
func resolveSigma(sigmaX, sigmaY float64, o options) (float64, float64, bool, bool) {
	sx, sy, box, ok := resolveStdDev(clampSigma(sigmaX), clampSigma(sigmaY))
	return sx, sy, box || o.boxBlur, ok
}

// clampSigma returns sigma as a float32, at most maxSigma. A NaN stays NaN.
func clampSigma(sigma float64) float32 {
	if sigma > maxSigma {
		return maxSigma
	}
	return float32(sigma)
}

// boxBlurRegion applies the box blur of resvg to data, which holds the
// pixels of an image of width by height pixels with bpp bytes for each
// pixel: 4 for RGBA and 1 for one channel. It blurs only the smallest box
// that holds each pixel that is not 0, grown by the sum of the radii of the
// boxes on each axis and cut to the image.
//
// Each step of the box blur gives a pixel the mean of the pixels within the
// radius of its box, so after all the steps a pixel out of that box is
// still 0. The blur reads 0 outside the image, and the pixels just outside
// the box are 0, so a pixel inside the box gets the same value as in a blur
// of the whole image.
func boxBlurRegion(data []byte, bpp, width, height int, sigmaX, sigmaY float64) {
	x0, y0, x1, y1, ok := nonZeroBounds(data, bpp, width, height)
	if !ok {
		return
	}
	rx, ry := boxRadii(sigmaX), boxRadii(sigmaY)
	x0, y0 = max(x0-rx, 0), max(y0-ry, 0)
	x1, y1 = min(x1+rx, width), min(y1+ry, height)
	if x0 == 0 && y0 == 0 && x1 == width && y1 == height {
		boxBlur(data, bpp, width, height, sigmaX, sigmaY)
		return
	}
	bw, bh := x1-x0, y1-y0
	stride := width * bpp
	buf := make([]byte, bw*bh*bpp)
	for y := range bh {
		copy(buf[y*bw*bpp:(y+1)*bw*bpp], data[(y0+y)*stride+x0*bpp:])
	}
	boxBlur(buf, bpp, bw, bh, sigmaX, sigmaY)
	for y := range bh {
		copy(data[(y0+y)*stride+x0*bpp:(y0+y)*stride+x1*bpp], buf[y*bw*bpp:(y+1)*bw*bpp])
	}
}

// boxBlur applies the box blur of resvg to the whole image in data, with
// bpp bytes for each pixel.
func boxBlur(data []byte, bpp, width, height int, sigmaX, sigmaY float64) {
	if bpp == 1 {
		applyAlpha(sigmaX, sigmaY, data, width, height)
		return
	}
	apply(sigmaX, sigmaY, newImageRefMut(uint32(width), uint32(height), data))
}

// boxRadii returns the sum of the radii of the boxes of the box blur of
// resvg for the standard deviation sigma: the number of pixels that the
// blur spreads a pixel by on its axis.
func boxRadii(sigma float64) int {
	n := 0
	for _, size := range createBoxGauss(float32(sigma)) {
		n += int((size - 1) / 2)
	}
	return n
}

// nonZeroBounds returns the smallest box that holds each pixel of data that
// is not 0, with x1 and y1 past its last column and row. data holds width
// by height pixels of bpp bytes. It returns false when each pixel is 0.
func nonZeroBounds(data []byte, bpp, width, height int) (x0, y0, x1, y1 int, ok bool) {
	stride := width * bpp
	x0, y0 = width, height
	for y := range height {
		row := data[y*stride : (y+1)*stride : (y+1)*stride]
		first := -1
		for i, v := range row {
			if v != 0 {
				first = i / bpp
				break
			}
		}
		if first < 0 {
			continue
		}
		last := first
		for i := len(row) - 1; i >= first*bpp; i-- {
			if row[i] != 0 {
				last = i / bpp
				break
			}
		}
		if y0 == height {
			y0 = y
		}
		y1 = y + 1
		x0 = min(x0, first)
		x1 = max(x1, last+1)
	}
	return x0, y0, x1, y1, y0 < height
}

// ColorMatrix is one of the four kinds of feColorMatrix of resvg, the enum
// ColorMatrixKind of usvg. The zero value changes no color: ApplyColorMatrix
// then only demultiplies and multiplies the pixels again, as resvg does for
// each color filter.
type ColorMatrix struct {
	kind   colorMatrixKind
	values [20]float32
	value  float32
}

// colorMatrixKind is the kind of a ColorMatrix.
type colorMatrixKind uint8

// The kinds of a ColorMatrix.
const (
	colorMatrixIdentity colorMatrixKind = iota
	colorMatrixMatrix
	colorMatrixSaturate
	colorMatrixHueRotate
	colorMatrixLuminanceToAlpha
)

// NewColorMatrix returns the matrix m of 4 rows of 5 numbers, as
// type="matrix" gives it, row by row. Each row gives a channel of R, G, B
// and A, from the channels R, G, B and A from 0 to 1, and a constant.
func NewColorMatrix(m [20]float32) ColorMatrix {
	return ColorMatrix{kind: colorMatrixMatrix, values: m}
}

// SaturateMatrix returns the matrix of type="saturate" for the value v. 0
// makes the colors gray, and 1 changes nothing. A value below 0 is 0. The
// matrix keeps the alpha.
func SaturateMatrix(v float32) ColorMatrix {
	return ColorMatrix{kind: colorMatrixSaturate, value: v}
}

// HueRotateMatrix returns the matrix of type="hueRotate", which turns the
// hue of the colors by degrees. The matrix keeps the alpha.
func HueRotateMatrix(degrees float32) ColorMatrix {
	return ColorMatrix{kind: colorMatrixHueRotate, value: degrees}
}

// LuminanceToAlphaMatrix returns the matrix of type="luminanceToAlpha",
// which sets the alpha to the luminance of the color and the color to black.
func LuminanceToAlphaMatrix() ColorMatrix {
	return ColorMatrix{kind: colorMatrixLuminanceToAlpha}
}

// ApplyColorMatrix demultiplies the pixels of p, applies m, and multiplies
// the pixels again, in place, as resvg does. Each channel of the result is
// cut to 0 and 1, and truncated to 8 bits.
func ApplyColorMatrix(p *okage.Pixmap, m ColorMatrix, opts ...Option) {
	if p == nil {
		return
	}
	o := newOptions(opts)
	data := p.DataMut()
	if o.linearRGB {
		pixmapIntoLinearRGB(data)
	}
	demultiplyAlpha(data)
	applyColorMatrix(&m, data)
	multiplyAlpha(data)
	if o.linearRGB {
		pixmapIntoSRGB(data)
	}
}

// ApplyColorMatrixNRGBA applies m to the pixels of img, which are not
// premultiplied, in place. It gives the bytes of ApplyColorMatrix without
// its round trip through premultiplied pixels, which loses precision where
// the alpha is low.
func ApplyColorMatrixNRGBA(img *image.NRGBA, m ColorMatrix, opts ...Option) {
	o := newOptions(opts)
	forEachRow(img, func(row []byte) {
		if o.linearRGB {
			intoLinearRGB(row)
		}
		applyColorMatrix(&m, row)
		if o.linearRGB {
			fromLinearRGB(row)
		}
	})
}

// TransferFunction is one of the five transfer functions of
// feComponentTransfer of resvg, the enum TransferFunction of usvg. It maps
// one channel of a color, from 0 to 1. The zero value is the identity.
type TransferFunction struct {
	kind   transferKind
	values []float32

	slope, intercept            float32
	amplitude, exponent, offset float32
}

// transferKind is the kind of a TransferFunction.
type transferKind uint8

// The kinds of a TransferFunction.
const (
	transferIdentity transferKind = iota
	transferTable
	transferDiscrete
	transferLinear
	transferGamma
)

// TransferTable returns the function of type="table". It maps the channel
// along the straight lines between the values, which split the range from 0
// to 1 in equal parts. No value gives the identity. The function holds a
// copy of values.
func TransferTable(values []float32) TransferFunction {
	return TransferFunction{kind: transferTable, values: append([]float32(nil), values...)}
}

// TransferDiscrete returns the function of type="discrete". It splits the
// range from 0 to 1 into as many equal steps as values, and maps each step
// to its value. No value gives the identity. The function holds a copy of
// values.
func TransferDiscrete(values []float32) TransferFunction {
	return TransferFunction{kind: transferDiscrete, values: append([]float32(nil), values...)}
}

// TransferLinear returns the function of type="linear": slope * c +
// intercept.
func TransferLinear(slope, intercept float32) TransferFunction {
	return TransferFunction{kind: transferLinear, slope: slope, intercept: intercept}
}

// TransferGamma returns the function of type="gamma": amplitude *
// pow(c, exponent) + offset.
func TransferGamma(amplitude, exponent, offset float32) TransferFunction {
	return TransferFunction{kind: transferGamma, amplitude: amplitude, exponent: exponent, offset: offset}
}

// ApplyComponentTransfer demultiplies the pixels of p, applies the
// functions r, g, b and a to their channels, and multiplies the pixels
// again, in place, as resvg does. Each channel of the result is cut to 0 and
// 1, and truncated to 8 bits.
func ApplyComponentTransfer(p *okage.Pixmap, r, g, b, a TransferFunction, opts ...Option) {
	if p == nil {
		return
	}
	o := newOptions(opts)
	data := p.DataMut()
	if o.linearRGB {
		pixmapIntoLinearRGB(data)
	}
	demultiplyAlpha(data)
	applyComponentTransfer(&r, &g, &b, &a, data)
	multiplyAlpha(data)
	if o.linearRGB {
		pixmapIntoSRGB(data)
	}
}

// ApplyComponentTransferNRGBA applies the functions r, g, b and a to the
// channels of the pixels of img, which are not premultiplied, in place.
func ApplyComponentTransferNRGBA(img *image.NRGBA, r, g, b, a TransferFunction, opts ...Option) {
	o := newOptions(opts)
	forEachRow(img, func(row []byte) {
		if o.linearRGB {
			intoLinearRGB(row)
		}
		applyComponentTransfer(&r, &g, &b, &a, row)
		if o.linearRGB {
			fromLinearRGB(row)
		}
	})
}

// forEachRow calls fn with the bytes of each row of the pixels of img.
func forEachRow(img *image.NRGBA, fn func(row []byte)) {
	if img == nil || img.Rect.Empty() {
		return
	}
	n := img.Rect.Dx() * 4
	for y := range img.Rect.Dy() {
		fn(img.Pix[y*img.Stride : y*img.Stride+n : y*img.Stride+n])
	}
}
