// Copyright 2020 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"github.com/xo/okage/internal/crmath"
	"github.com/xo/okage/internal/wide"
)

// This file ports resvg src/filter/component_transfer.rs.

// applyFilterComponentTransfer is apply in resvg
// src/filter/component_transfer.rs. It applies component transfer functions
// for each `src` image channel.
//
// Input image pixels should have an **unpremultiplied alpha**.
func applyFilterComponentTransfer(fe *ComponentTransferFilter, src imageRefMut) {
	for i := 0; i+4 <= len(src.data); i += 4 {
		pixel := src.data[i : i+4 : i+4]
		if !isDummy(fe.FuncR()) {
			pixel[0] = transfer(fe.FuncR(), pixel[0])
		}

		if !isDummy(fe.FuncB()) {
			pixel[2] = transfer(fe.FuncB(), pixel[2])
		}

		if !isDummy(fe.FuncG()) {
			pixel[1] = transfer(fe.FuncG(), pixel[1])
		}

		if !isDummy(fe.FuncA()) {
			pixel[3] = transfer(fe.FuncA(), pixel[3])
		}
	}
}

// isDummy is is_dummy in resvg src/filter/component_transfer.rs.
func isDummy(fn TransferFunctionFilter) bool {
	switch f := fn.(type) {
	case TransferFunctionFilterIdentity:
		return true
	case TransferFunctionFilterTable:
		return len(f) == 0
	case TransferFunctionFilterDiscrete:
		return len(f) == 0
	case TransferFunctionFilterLinear:
		return false
	case TransferFunctionFilterGamma:
		return false
	}
	return true
}

// transfer is transfer in resvg src/filter/component_transfer.rs.
func transfer(fn TransferFunctionFilter, c8 uint8) uint8 {
	c := float32(c8) / 255.0
	switch f := fn.(type) {
	case TransferFunctionFilterIdentity:
	case TransferFunctionFilterTable:
		values := f
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
	case TransferFunctionFilterDiscrete:
		values := f
		n := len(values)
		k := asUsize(floorF32(c * float32(n)))
		c = values[min(k, n-1)]
	case TransferFunctionFilterLinear:
		c = float32(f.Slope*c) + f.Intercept
	case TransferFunctionFilterGamma:
		c = float32(f.Amplitude*crmath.Pow32(c, f.Exponent)) + f.Offset
	}

	return wide.TruncU8(f32BoundFilter(0.0, c, 1.0) * 255.0)
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
