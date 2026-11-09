// Copyright 2020 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "github.com/xo/okage/internal/wide"

// This file ports resvg src/filter/morphology.rs.

// applyFilterMorphology is apply in resvg src/filter/morphology.rs. It
// applies a morphology filter.
//
// `src` pixels should have a **premultiplied alpha**.
//
// # Allocations
//
// This method will allocate a copy of the `src` image as a back buffer.
func applyFilterMorphology(operator MorphologyOperatorFilter, rx, ry float32, src imageRefMut) {
	widthMax := src.width - 1
	heightMax := src.height - 1
	rxU := min(wide.TruncU32(round32(rx)), widthMax)
	ryU := min(wide.TruncU32(round32(ry)), heightMax)

	buf := newImageRefMut(src.width, src.height, make([]byte, len(src.data)))
	var x, y uint32
	for range len(src.data) / 4 {
		var newP rgba8
		if operator == MorphologyOperatorFilterErode {
			newP.r = 255
			newP.g = 255
			newP.b = 255
			newP.a = 255
		}

		xStart, xEnd := saturatingSubU32(x, rxU), min(x+rxU, widthMax)
		yStart, yEnd := saturatingSubU32(y, ryU), min(y+ryU, heightMax)

		for ty := yStart; ty <= yEnd; ty++ {
			for tx := xStart; tx <= xEnd; tx++ {
				p := src.pixelAt(tx, ty)
				if operator == MorphologyOperatorFilterErode {
					newP.r = min(p.r, newP.r)
					newP.g = min(p.g, newP.g)
					newP.b = min(p.b, newP.b)
					newP.a = min(p.a, newP.a)
				} else {
					newP.r = max(p.r, newP.r)
					newP.g = max(p.g, newP.g)
					newP.b = max(p.b, newP.b)
					newP.a = max(p.a, newP.a)
				}
			}
		}

		buf.setPixelAt(x, y, newP)

		x++
		if x == src.width {
			x = 0
			y++
		}
	}

	// Do not use `mem::swap` because `data` referenced via FFI.
	copy(src.data, buf.data)
}

// saturatingSubU32 is u32::saturating_sub of Rust.
func saturatingSubU32(a, b uint32) uint32 {
	if b > a {
		return 0
	}
	return a - b
}
