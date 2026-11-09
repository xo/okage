// Copyright 2020 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "github.com/xo/okage/internal/wide"

// This file ports resvg src/filter/convolve_matrix.rs.

// applyFilterConvolveMatrix is apply in resvg src/filter/convolve_matrix.rs.
// It applies a convolve matrix.
//
// Input image pixels should have a **premultiplied alpha** when `preserve_alpha=false`.
//
// # Allocations
//
// This method will allocate a copy of the `src` image as a back buffer.
func applyFilterConvolveMatrix(matrix *ConvolveMatrixFilter, src imageRefMut) {
	bound := func(minimum, val, maximum int32) int32 {
		return max(minimum, min(maximum, val))
	}

	widthMax := int32(src.width) - 1
	heightMax := int32(src.height) - 1

	buf := newImageRefMut(src.width, src.height, make([]byte, len(src.data)))
	var x, y uint32
	for j := range len(src.data) / 4 {
		inP := rgba8At(src.data, j)
		var newR, newG, newB, newA float32
		for oy := range matrix.Matrix().Rows() {
			for ox := range matrix.Matrix().Columns() {
				tx := int32(x) - int32(matrix.Matrix().TargetX()) + int32(ox)
				ty := int32(y) - int32(matrix.Matrix().TargetY()) + int32(oy)

				switch matrix.EdgeMode() {
				case EdgeModeFilterNone:
					if tx < 0 || tx > widthMax || ty < 0 || ty > heightMax {
						continue
					}
				case EdgeModeFilterDuplicate:
					tx = bound(0, tx, widthMax)
					ty = bound(0, ty, heightMax)
				case EdgeModeFilterWrap:
					for tx < 0 {
						tx += int32(src.width)
					}
					tx %= int32(src.width)

					for ty < 0 {
						ty += int32(src.height)
					}
					ty %= int32(src.height)
				}

				k := matrix.Matrix().Get(
					matrix.Matrix().Columns()-ox-1,
					matrix.Matrix().Rows()-oy-1,
				)

				p := src.pixelAt(uint32(tx), uint32(ty))
				newR += float32(float32(p.r) / 255.0 * k)
				newG += float32(float32(p.g) / 255.0 * k)
				newB += float32(float32(p.b) / 255.0 * k)

				if !matrix.PreserveAlpha() {
					newA += float32(float32(p.a) / 255.0 * k)
				}
			}
		}

		if matrix.PreserveAlpha() {
			newA = float32(inP.a) / 255.0
		} else {
			newA = newA/matrix.Divisor().Get() + matrix.Bias()
		}

		boundedNewA := f32BoundFilter(0.0, newA, 1.0)

		calc := func(x float32) uint8 {
			x = x/matrix.Divisor().Get() + float32(matrix.Bias()*newA)

			if matrix.PreserveAlpha() {
				x = f32BoundFilter(0.0, x, 1.0) * boundedNewA
			} else {
				x = f32BoundFilter(0.0, x, boundedNewA)
			}

			return wide.TruncU8(float32(x*255.0) + 0.5)
		}

		buf.setPixelAt(x, y, rgba8{
			r: calc(newR),
			g: calc(newG),
			b: calc(newB),
			a: wide.TruncU8(float32(boundedNewA*255.0) + 0.5),
		})

		x++
		if x == src.width {
			x = 0
			y++
		}
	}

	// Do not use `mem::swap` because `data` referenced via FFI.
	copy(src.data, buf.data)
}
