// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import "math"

// alphaRun is AlphaRun in tiny-skia src/alpha_runs.rs.
//
// Port: upstream uses Option<NonZeroU16>. Go uses a uint16, and zero is None.
type alphaRun = uint16

// alphaRuns is AlphaRuns in tiny-skia src/alpha_runs.rs. It is a sparse array
// of run-length-encoded alpha (supersampling coverage) values.
//
// Sparseness allows us to independently compose several paths into the same
// AlphaRuns buffer.
type alphaRuns struct {
	runs  []alphaRun
	alpha []uint8
}

// newAlphaRuns is AlphaRuns::new in tiny-skia src/alpha_runs.rs.
func newAlphaRuns(width lengthU32) *alphaRuns {
	runs := &alphaRuns{
		runs:  make([]alphaRun, width+1),
		alpha: make([]uint8, width+1),
	}
	runs.reset(width)
	return runs
}

// catchOverflow is AlphaRuns::catch_overflow in tiny-skia src/alpha_runs.rs.
// It returns 0-255 given 0-256.
func catchOverflow(alpha uint16) alphaU8 {
	return uint8(alpha - (alpha >> 8))
}

// isEmpty is AlphaRuns::is_empty in tiny-skia src/alpha_runs.rs. It returns
// true if the scanline contains only a single run, of alpha value 0.
func (ar *alphaRuns) isEmpty() bool {
	if run := ar.runs[0]; run != 0 {
		return ar.alpha[0] == 0 && ar.runs[run] == 0
	}
	return true
}

// reset is AlphaRuns::reset in tiny-skia src/alpha_runs.rs. It
// reinitializes the runs for a new scanline.
func (ar *alphaRuns) reset(width lengthU32) {
	if width > math.MaxUint16 {
		panic("called `Result::unwrap()` on an `Err` value")
	}
	run := uint16(width)
	ar.runs[0] = run
	ar.runs[width] = 0
	ar.alpha[0] = 0
}

// add is AlphaRuns::add in tiny-skia src/alpha_runs.rs.
//
// Insert into the buffer a run starting at (x-offset_x).
//
//	if start_alpha > 0
//	    one pixel with value += start_alpha,
//	        max 255
//	if middle_count > 0
//	    middle_count pixels with value += max_value
//	if stop_alpha > 0
//	    one pixel with value += stop_alpha
//
// Returns the offset_x value to pass on the next call, if the next call is
// on the same scanline. If the caller switches scanlines, offset_x must be 0
// when this is called.
func (ar *alphaRuns) add(x uint32, startAlpha alphaU8, middleCount int, stopAlpha alphaU8, maxValue uint8, offsetX int) int {
	xx := int(x)

	runsOffset := offsetX
	alphaOffset := offsetX
	lastAlphaOffset := offsetX
	xx -= offsetX

	if startAlpha != 0 {
		breakRun(ar.runs[runsOffset:], ar.alpha[alphaOffset:], xx, 1)
		// In theory, I can just add alpha[x] + start_alpha.
		// However, if the trailing edge of the previous span and the leading
		// edge of the current span round to the same super-sampled x value,
		// this add can overflow to 256, hence the funny subtract (crud).
		tmp := uint16(ar.alpha[alphaOffset+xx]) + uint16(startAlpha)
		// was (tmp >> 7), but that seems wrong if we try to catch 256
		ar.alpha[alphaOffset+xx] = uint8(tmp - (tmp >> 8))

		runsOffset += xx + 1
		alphaOffset += xx + 1
		xx = 0
	}

	if middleCount != 0 {
		breakRun(ar.runs[runsOffset:], ar.alpha[alphaOffset:], xx, middleCount)
		alphaOffset += xx
		runsOffset += xx
		xx = 0
		for {
			a := catchOverflow(uint16(ar.alpha[alphaOffset]) + uint16(maxValue))
			ar.alpha[alphaOffset] = a

			n := int(unwrapRun(ar.runs[runsOffset]))
			alphaOffset += n
			runsOffset += n
			middleCount -= n

			if middleCount == 0 {
				break
			}
		}

		lastAlphaOffset = alphaOffset
	}

	if stopAlpha != 0 {
		breakRun(ar.runs[runsOffset:], ar.alpha[alphaOffset:], xx, 1)
		alphaOffset += xx
		ar.alpha[alphaOffset] += stopAlpha
		lastAlphaOffset = alphaOffset
	}

	// new offset_x
	return lastAlphaOffset
}

// unwrapRun is Option::unwrap for an AlphaRun. It panics if the run is None.
// It ports no upstream function.
func unwrapRun(r alphaRun) uint16 {
	if r == 0 {
		panic("called `Option::unwrap()` on a `None` value")
	}
	return r
}

// breakRun is AlphaRuns::break_run in tiny-skia src/alpha_runs.rs.
//
// Break the runs in the buffer at offsets x and x+count, properly updating
// the runs to the right and left.
//
// i.e. from the state AAAABBBB, run-length encoded as A4B4, break_run(..., 2,
// 5) produces AAAABBBB rle as A2A2B3B1. Allows add() to sum another run
// to some of the new sub-runs. i.e. adding ..CCCCC. produces AADDEEEB,
// rle as A2D2E3B1.
func breakRun(runs []alphaRun, alpha []uint8, x, count int) {
	origX := x
	runsOffset := 0
	alphaOffset := 0

	for x > 0 {
		n := int(unwrapRun(runs[runsOffset]))

		if x < n {
			alpha[alphaOffset+x] = alpha[alphaOffset]
			runs[runsOffset+0] = uint16(x)
			runs[runsOffset+x] = uint16(n - x)
			break
		}
		runsOffset += n
		alphaOffset += n
		x -= n
	}

	runsOffset = origX
	alphaOffset = origX
	x = count

	for {
		n := int(unwrapRun(runs[runsOffset]))

		if x < n {
			alpha[alphaOffset+x] = alpha[alphaOffset]
			runs[runsOffset+0] = uint16(x)
			runs[runsOffset+x] = uint16(n - x)
			break
		}

		x -= n
		if x == 0 {
			break
		}

		runsOffset += n
		alphaOffset += n
	}
}

// breakAt is AlphaRuns::break_at in tiny-skia src/alpha_runs.rs.
//
// Cut (at offset x in the buffer) a run into two shorter runs with matching
// alpha values.
//
// Used by the RectClipBlitter to trim a RLE encoding to match the clipping
// rectangle.
func breakAt(alpha []alphaU8, runs []alphaRun, x int32) {
	alphaI := 0
	runI := 0
	for x > 0 {
		n := unwrapRun(runs[runI])
		nUsize := int(n)
		nI32 := int32(n)
		if x < nI32 {
			alpha[alphaI+int(x)] = alpha[alphaI]
			runs[0] = uint16(x)
			runs[x] = uint16(nI32 - x)
			break
		}

		runI += nUsize
		alphaI += nUsize
		x -= nI32
	}
}
