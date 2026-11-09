// Copyright 2020 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "math"

// This file ports resvg src/filter/iir_blur.rs.
//
// An IIR blur.
//
// Based on http://www.getreuer.info/home/gaussianiir
//
// Licensed under 'Simplified BSD License'.
//
// Implements the fast Gaussian convolution algorithm of Alvarez and Mazorra,
// where the Gaussian is approximated by a cascade of first-order infinite
// impulsive response (IIR) filters.  Boundaries are handled with half-sample
// symmetric extension.
//
// Gaussian convolution is approached as approximating the heat equation and
// each timestep is performed with an efficient recursive computation.  Using
// more steps yields a more accurate approximation of the Gaussian.  A
// reasonable default value for `numsteps` is 4.
//
// Reference:
// Alvarez, Mazorra, "Signal and Image Restoration using Shock Filters and
// Anisotropic Diffusion," SIAM J. on Numerical Analysis, vol. 31, no. 2,
// pp. 590-605, 1994.

// TODO: Blurs right and bottom sides twice for some reason.

// blurData is BlurData in resvg src/filter/iir_blur.rs.
type blurData struct {
	width  int
	height int
	sigmaX float64
	sigmaY float64
	steps  int
}

// applyFilterIIRBlur is apply in resvg src/filter/iir_blur.rs. It applies an
// IIR blur.
//
// Input image pixels should have a **premultiplied alpha**.
//
// A negative or zero `sigma_x`/`sigma_y` will disable the blur along that axis.
//
// # Allocations
//
// This method will allocate a 2x `src` buffer.
func applyFilterIIRBlur(sigmaX, sigmaY float64, src imageRefMut) {
	bufSize := int(src.width * src.height)
	buf := make([]float64, bufSize)

	d := blurData{
		width:  int(src.width),
		height: int(src.height),
		sigmaX: sigmaX,
		sigmaY: sigmaY,
		steps:  4,
	}

	data := src.data
	gaussianChannel(data, &d, 0, buf)
	gaussianChannel(data, &d, 1, buf)
	gaussianChannel(data, &d, 2, buf)
	gaussianChannel(data, &d, 3, buf)
}

// gaussianChannel is gaussian_channel in resvg src/filter/iir_blur.rs.
func gaussianChannel(data []byte, d *blurData, channel int, buf []float64) {
	for i := range len(data) / 4 {
		buf[i] = float64(data[i*4+channel]) / 255.0
	}

	gaussianiir2d(d, buf)

	for i := range len(data) / 4 {
		data[i*4+channel] = asU8F64(buf[i] * 255.0)
	}
}

// gaussianiir2d is gaussianiir2d in resvg src/filter/iir_blur.rs.
func gaussianiir2d(d *blurData, buf []float64) {
	// Filter horizontally along each row.
	var lambdaX, dnuX float64
	if d.sigmaX > 0.0 {
		lambda, dnu := genCoefficients(d.sigmaX, d.steps)

		for y := range d.height {
			for range d.steps {
				idx := d.width * y

				// Filter rightwards.
				for x := 1; x < d.width; x++ {
					buf[idx+x] += float64(dnu * buf[idx+x-1])
				}

				x := d.width - 1

				// Filter leftwards.
				for x > 0 {
					buf[idx+x-1] += float64(dnu * buf[idx+x])
					x--
				}
			}
		}

		lambdaX, dnuX = lambda, dnu
	} else {
		lambdaX, dnuX = 1.0, 1.0
	}

	// Filter vertically along each column.
	var lambdaY, dnuY float64
	if d.sigmaY > 0.0 {
		lambda, dnu := genCoefficients(d.sigmaY, d.steps)
		for x := range d.width {
			for range d.steps {
				idx := x

				// Filter downwards.
				y := d.width
				for y < len(buf) {
					buf[idx+y] += float64(dnu * buf[idx+y-d.width])
					y += d.width
				}

				y = len(buf) - d.width

				// Filter upwards.
				for y > 0 {
					buf[idx+y-d.width] += float64(dnu * buf[idx+y])
					y -= d.width
				}
			}
		}

		lambdaY, dnuY = lambda, dnu
	} else {
		lambdaY, dnuY = 1.0, 1.0
	}

	postScale := powi64(math.Sqrt(dnuX*dnuY)/math.Sqrt(lambdaX*lambdaY), int32(2*d.steps))
	for i := range buf {
		buf[i] *= postScale
	}
}

// genCoefficients is gen_coefficients in resvg src/filter/iir_blur.rs.
func genCoefficients(sigma float64, steps int) (float64, float64) {
	lambda := (sigma * sigma) / (2.0 * float64(steps))
	dnu := (1.0 + float64(2.0*lambda) - math.Sqrt(float64(1.0+float64(4.0*lambda)))) / (2.0 * lambda)
	return lambda, dnu
}

// powi64 is f64::powi of Rust. Rust calls the intrinsic llvm.powi, which
// compiler-rt computes as __powidf2: a product of squares in f64, and the
// reciprocal of the product for a negative exponent.
func powi64(a float64, b int32) float64 {
	recip := b < 0
	r := float64(1)
	for {
		if b&1 != 0 {
			r *= a
		}
		b /= 2
		if b == 0 {
			break
		}
		a *= a
	}
	if recip {
		return 1 / r
	}
	return r
}
