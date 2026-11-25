// Copyright 2020 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package filter

import (
	"github.com/xo/okage/internal/crmath"
	"github.com/xo/okage/internal/wide"
)

// This file ports resvg src/filter/component_transfer.rs. It moved from svg
// for D88.

// applyComponentTransfer is apply in resvg
// src/filter/component_transfer.rs. It applies component transfer functions
// for each `src` image channel.
//
// Input image pixels should have an **unpremultiplied alpha**.
//
// Port: upstream takes the filter of usvg, and this function takes its four
// functions. src holds only the RGBA bytes.
func applyComponentTransfer(funcR, funcG, funcB, funcA *TransferFunction, src []byte) {
	for i := 0; i+4 <= len(src); i += 4 {
		pixel := src[i : i+4 : i+4]
		if !isDummy(funcR) {
			pixel[0] = transfer(funcR, pixel[0])
		}

		if !isDummy(funcB) {
			pixel[2] = transfer(funcB, pixel[2])
		}

		if !isDummy(funcG) {
			pixel[1] = transfer(funcG, pixel[1])
		}

		if !isDummy(funcA) {
			pixel[3] = transfer(funcA, pixel[3])
		}
	}
}

// isDummy is is_dummy in resvg src/filter/component_transfer.rs.
func isDummy(fn *TransferFunction) bool {
	switch fn.kind {
	case transferIdentity:
		return true
	case transferTable:
		return len(fn.values) == 0
	case transferDiscrete:
		return len(fn.values) == 0
	case transferLinear:
		return false
	case transferGamma:
		return false
	}
	return true
}

// transfer is transfer in resvg src/filter/component_transfer.rs.
func transfer(fn *TransferFunction, c8 uint8) uint8 {
	c := float32(c8) / 255.0
	switch fn.kind {
	case transferIdentity:
	case transferTable:
		values := fn.values
		n := len(values) - 1
		k := asUsize(floorF32(c * float32(n)))
		k = min(k, n)
		if k == n {
			c = values[k]
		} else {
			vk := values[k]
			vk1 := values[k+1]
			kf := float32(k)
			nf := float32(n)
			c = vk + float32(float32((c-kf/nf)*nf)*(vk1-vk))
		}
	case transferDiscrete:
		values := fn.values
		n := len(values)
		k := asUsize(floorF32(c * float32(n)))
		c = values[min(k, n-1)]
	case transferLinear:
		c = float32(fn.slope*c) + fn.intercept
	case transferGamma:
		c = float32(fn.amplitude*crmath.Pow32(c, fn.exponent)) + fn.offset
	}

	return wide.TruncU8(f32Bound(0.0, c, 1.0) * 255.0)
}

// asUsize is the Rust cast "f as usize" for an f32. It truncates toward zero
// and saturates, and NaN gives 0.
func asUsize(f float32) int {
	switch {
	case f != f || f <= 0:
		return 0
	case f >= 2147483647:
		// The callers take the minimum of the result and the length of a
		// slice, so this limit gives the result of the limit of usize.
		return 2147483647
	}
	return int(f)
}
