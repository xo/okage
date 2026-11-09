// Copyright 2020 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "github.com/xo/okage/internal/wide"

// This file ports resvg src/filter/composite.rs.

// arithmetic is arithmetic in resvg src/filter/composite.rs. It performs an
// arithmetic composition.
//
//   - `src1` and `src2` image pixels should have a **premultiplied alpha**.
//   - `dest` image pixels will have a **premultiplied alpha**.
//
// `src1`, `src2` and `dest` dimensions must match.
func arithmetic(
	k1 float32,
	k2 float32,
	k3 float32,
	k4 float32,
	src1 imageRef,
	src2 imageRef,
	dest imageRefMut,
) {
	calc := func(i1, i2 uint8, maximum float32) float32 {
		i1f := float32(i1) / 255.0
		i2f := float32(i2) / 255.0
		result := float32(float32(k1*i1f)*i2f) + float32(k2*i1f) + float32(k3*i2f) + k4
		return f32BoundFilter(0.0, result, maximum)
	}

	i := 0
	n := min(len(src1.data), len(src2.data)) / 4
	for j := range n {
		c1 := rgba8At(src1.data, j)
		c2 := rgba8At(src2.data, j)
		a := calc(c1.a, c2.a, 1.0)
		if ApproxZeroUlps32(a, 4) {
			i++
			continue
		}

		r := wide.TruncU8(calc(c1.r, c2.r, a) * 255.0)
		g := wide.TruncU8(calc(c1.g, c2.g, a) * 255.0)
		b := wide.TruncU8(calc(c1.b, c2.b, a) * 255.0)
		a8 := wide.TruncU8(a * 255.0)

		setRGBA8At(dest.data, i, rgba8{r: r, g: g, b: b, a: a8})

		i++
	}
}
