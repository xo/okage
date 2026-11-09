// Copyright 2020 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"github.com/xo/okage/internal/crmath"
	"github.com/xo/okage/internal/wide"
)

// This file ports resvg src/filter/color_matrix.rs.
//
// Port: each sum of products of upstream converts each product to float32, so
// that the Go compiler cannot fuse a multiply and an add (D16).

// applyFilterColorMatrix is apply in resvg src/filter/color_matrix.rs. It
// applies a color matrix filter.
//
// Input image pixels should have an **unpremultiplied alpha**.
func applyFilterColorMatrix(matrix ColorMatrixKindFilter, src imageRefMut) {
	switch mk := matrix.(type) {
	case ColorMatrixKindFilterMatrix:
		m := mk
		for i := 0; i+4 <= len(src.data); i += 4 {
			pixel := src.data[i : i+4 : i+4]
			r, g, b, a := toNormalizedComponents(rgba8{r: pixel[0], g: pixel[1], b: pixel[2], a: pixel[3]})

			newR := float32(r*m[0]) + float32(g*m[1]) + float32(b*m[2]) + float32(a*m[3]) + m[4]
			newG := float32(r*m[5]) + float32(g*m[6]) + float32(b*m[7]) + float32(a*m[8]) + m[9]
			newB := float32(r*m[10]) + float32(g*m[11]) + float32(b*m[12]) + float32(a*m[13]) + m[14]
			newA := float32(r*m[15]) + float32(g*m[16]) + float32(b*m[17]) + float32(a*m[18]) + m[19]

			pixel[0] = fromNormalized(newR)
			pixel[1] = fromNormalized(newG)
			pixel[2] = fromNormalized(newB)
			pixel[3] = fromNormalized(newA)
		}
	case ColorMatrixKindFilterSaturate:
		v := maxF32(PositiveF32(mk).Get(), 0.0)
		m := [9]float32{
			0.213 + float32(0.787*v),
			0.715 - float32(0.715*v),
			0.072 - float32(0.072*v),
			0.213 - float32(0.213*v),
			0.715 + float32(0.285*v),
			0.072 - float32(0.072*v),
			0.213 - float32(0.213*v),
			0.715 - float32(0.715*v),
			0.072 + float32(0.928*v),
		}

		for i := 0; i+4 <= len(src.data); i += 4 { //nolint:dupl // upstream repeats the loop for Saturate and HueRotate
			pixel := src.data[i : i+4 : i+4]
			r, g, b, _ := toNormalizedComponents(rgba8{r: pixel[0], g: pixel[1], b: pixel[2], a: pixel[3]})

			newR := float32(r*m[0]) + float32(g*m[1]) + float32(b*m[2])
			newG := float32(r*m[3]) + float32(g*m[4]) + float32(b*m[5])
			newB := float32(r*m[6]) + float32(g*m[7]) + float32(b*m[8])

			pixel[0] = fromNormalized(newR)
			pixel[1] = fromNormalized(newG)
			pixel[2] = fromNormalized(newB)
		}
	case ColorMatrixKindFilterHueRotate:
		angle := toRadians(float32(mk))
		a1 := crmath.Cos32(angle)
		a2 := crmath.Sin32(angle)
		m := [9]float32{
			0.213 + float32(0.787*a1) - float32(0.213*a2),
			0.715 - float32(0.715*a1) - float32(0.715*a2),
			0.072 - float32(0.072*a1) + float32(0.928*a2),
			0.213 - float32(0.213*a1) + float32(0.143*a2),
			0.715 + float32(0.285*a1) + float32(0.140*a2),
			0.072 - float32(0.072*a1) - float32(0.283*a2),
			0.213 - float32(0.213*a1) - float32(0.787*a2),
			0.715 - float32(0.715*a1) + float32(0.715*a2),
			0.072 + float32(0.928*a1) + float32(0.072*a2),
		}

		for i := 0; i+4 <= len(src.data); i += 4 { //nolint:dupl // upstream repeats the loop for Saturate and HueRotate
			pixel := src.data[i : i+4 : i+4]
			r, g, b, _ := toNormalizedComponents(rgba8{r: pixel[0], g: pixel[1], b: pixel[2], a: pixel[3]})

			newR := float32(r*m[0]) + float32(g*m[1]) + float32(b*m[2])
			newG := float32(r*m[3]) + float32(g*m[4]) + float32(b*m[5])
			newB := float32(r*m[6]) + float32(g*m[7]) + float32(b*m[8])

			pixel[0] = fromNormalized(newR)
			pixel[1] = fromNormalized(newG)
			pixel[2] = fromNormalized(newB)
		}
	case ColorMatrixKindFilterLuminanceToAlpha:
		for i := 0; i+4 <= len(src.data); i += 4 {
			pixel := src.data[i : i+4 : i+4]
			r, g, b, _ := toNormalizedComponents(rgba8{r: pixel[0], g: pixel[1], b: pixel[2], a: pixel[3]})

			newA := float32(r*0.2125) + float32(g*0.7154) + float32(b*0.0721)

			pixel[0] = 0
			pixel[1] = 0
			pixel[2] = 0
			pixel[3] = fromNormalized(newA)
		}
	}
}

// toNormalizedComponents is to_normalized_components in resvg
// src/filter/color_matrix.rs.
func toNormalizedComponents(pixel rgba8) (float32, float32, float32, float32) {
	return float32(pixel.r) / 255.0,
		float32(pixel.g) / 255.0,
		float32(pixel.b) / 255.0,
		float32(pixel.a) / 255.0
}

// fromNormalized is from_normalized in resvg src/filter/color_matrix.rs.
func fromNormalized(c float32) uint8 {
	return wide.TruncU8(f32BoundFilter(0.0, c, 1.0) * 255.0)
}

// radsPerDeg is the constant of f32::to_radians in Rust: consts::PI / 180.0,
// computed in f32. A Go conversion of a constant rounds it, so
// float32(math.Pi) is the f32 PI of Rust, and the division of two typed
// constants rounds once, as the f32 division does.
const radsPerDeg = float32(pi32) / 180.0

// toRadians is f32::to_radians.
func toRadians(x float32) float32 {
	return x * radsPerDeg
}
