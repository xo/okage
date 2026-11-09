// Copyright 2020 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "github.com/xo/okage/internal/wide"

// This file ports resvg src/filter/displacement_map.rs.

// applyFilterDisplacementMap is apply in resvg
// src/filter/displacement_map.rs. It applies a displacement map.
//
//   - `map` pixels should have a **unpremultiplied alpha**.
//   - `src` pixels can have any alpha method.
//
// `scale_x` and `scale_y` are the `scale` attribute already converted
// into the canvas coordinate system, i.e. the maximum displacement in pixels.
//
// `src`, `map` and `dest` dimensions must match.
func applyFilterDisplacementMap(
	fe *DisplacementMapFilter,
	scaleX float32,
	scaleY float32,
	src imageRef,
	m imageRef,
	dest imageRefMut,
) {
	w := int32(src.width)
	h := int32(src.height)

	var x, y uint32
	for j := range len(m.data) / 4 {
		pixel := rgba8At(m.data, j)
		calcOffset := func(channel ColorChannelFilter) float32 {
			var c uint8
			switch channel {
			case ColorChannelFilterB:
				c = pixel.b
			case ColorChannelFilterG:
				c = pixel.g
			case ColorChannelFilterR:
				c = pixel.r
			case ColorChannelFilterA:
				c = pixel.a
			}

			return float32(c)/255.0 - 0.5
		}

		dx := calcOffset(fe.XChannelSelector())
		dy := calcOffset(fe.YChannelSelector())
		ox := wide.TruncI32(round32(float32(x) + float32(dx*scaleX)))
		oy := wide.TruncI32(round32(float32(y) + float32(dy*scaleY)))

		// TODO: we should use some kind of anti-aliasing when offset is on a pixel border

		if x < uint32(w) && y < uint32(h) && ox >= 0 && ox < w && oy >= 0 && oy < h {
			idx := int(oy*w + ox)
			idx1 := int(y*uint32(w) + x)
			setRGBA8At(dest.data, idx1, rgba8At(src.data, idx))
		}

		x++
		if x == src.width {
			x = 0
			y++
		}
	}
}
