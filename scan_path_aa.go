// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"log/slog"

	"github.com/xo/okage/path"
)

// supersampleShift is SUPERSAMPLE_SHIFT in tiny-skia src/scan/path_aa.rs. It
// controls how much we super-sample (when we use that scan conversion).
const supersampleShift uint32 = 2

// shiftPathAA is SHIFT in tiny-skia src/scan/path_aa.rs.
//
// Port: the constants SHIFT, SCALE and MASK take the suffix of their module,
// because local variables of the package have the names shift and scale, and
// blitter.rs has the type Mask.
const shiftPathAA uint32 = supersampleShift

// scalePathAA is SCALE in tiny-skia src/scan/path_aa.rs.
const scalePathAA uint32 = 1 << shiftPathAA

// maskPathAA is MASK in tiny-skia src/scan/path_aa.rs.
const maskPathAA uint32 = scalePathAA - 1

// fillPathPathAA is fill_path in tiny-skia src/scan/path_aa.rs. The name takes
// the suffix of its module, because src/scan/path.rs has a function of the
// same name (D10).
//
// Port: logger is the logger of D20, which buildEdges logs to. A nil logger
// logs nothing.
func fillPathPathAA(p *Path, fillRule FillRule, clip *screenIntRect, b blitter, logger *slog.Logger) {
	// Unlike `path.bounds.to_rect()?.round_out()`,
	// this method rounds out first and then converts into a Rect.
	bounds := p.Bounds()
	r, ok := path.RectFromLTRB(
		floorF32(bounds.Left()),
		floorF32(bounds.Top()),
		ceilF32(bounds.Right()),
		ceilF32(bounds.Bottom()),
	)
	if !ok {
		return
	}
	ir, ok := r.RoundOut()
	if !ok {
		return
	}

	// TODO: remove
	// If the intersection of the path bounds and the clip bounds
	// will overflow 32767 when << by SHIFT, we cannot supersample,
	// so draw without antialiasing.
	clippedIR, ok := ir.Intersect(clip.ToIntRect())
	if !ok {
		return
	}
	if rectOverflowsShortShift(&clippedIR, int32(shiftPathAA)) != 0 {
		fillPathPath(p, fillRule, clip, b, logger)
		return
	}

	// TODO: remove
	// Our antialiasing cannot handle a clip larger than 32767.
	// TODO: skia actually limits the clip to 32767
	{
		const maxClipCoord uint32 = 32767
		if clip.Right() > maxClipCoord || clip.Bottom() > maxClipCoord {
			return
		}
	}

	// TODO: SkScanClipper
	// TODO: AAA

	fillPathImplPathAA(p, fillRule, &ir, clip, b, logger)
}

// rectOverflowsShortShift is rect_overflows_short_shift in tiny-skia
// src/scan/path_aa.rs.
//
// Does any of the coordinates of this rectangle not fit in a short, when
// left-shifted by shift?
func rectOverflowsShortShift(rect *IntRect, shift int32) int32 {
	// Since we expect these to succeed, we bit-or together
	// for a tiny extra bit of speed.
	return overflowsShortShift(rect.Left(), shift) |
		overflowsShortShift(rect.Top(), shift) |
		overflowsShortShift(rect.Right(), shift) |
		overflowsShortShift(rect.Bottom(), shift)
}

// overflowsShortShift is overflows_short_shift in tiny-skia
// src/scan/path_aa.rs.
func overflowsShortShift(value, shift int32) int32 {
	s := 16 + shift
	return (leftShift(value, s) >> s) - value
}

// fillPathImplPathAA is fill_path_impl in tiny-skia src/scan/path_aa.rs. The
// name takes the suffix of its module, because src/scan/path.rs has a function
// of the same name (D10).
func fillPathImplPathAA(p *Path, fillRule FillRule, bounds *IntRect, clip *screenIntRect, b blitter, logger *slog.Logger) {
	// TODO: MaskSuperBlitter

	// TODO: 15% slower than skia, find out why
	sb, ok := newSuperBlitter(bounds, clip, b)
	if !ok {
		return // clipped out, nothing else to do
	}

	pathContainedInClip := false
	if sbounds, ok := intRectToScreenIntRect(*bounds); ok {
		pathContainedInClip = clip.Contains(sbounds)
	}
	// If bounds cannot be converted into ScreenIntRect,
	// the path is out of clip.

	fillPathImplPath(
		p,
		fillRule,
		clip,
		bounds.Top(),
		bounds.Bottom(),
		int32(shiftPathAA),
		pathContainedInClip,
		sb,
		logger,
	)

	// Port: this is the Drop impl of SuperBlitter, which Rust runs when sb
	// goes out of scope.
	sb.flush()
}

// baseSuperBlitter is BaseSuperBlitter in tiny-skia src/scan/path_aa.rs.
type baseSuperBlitter struct {
	realBlitter blitter

	// currIY is the current y coordinate, in destination coordinates.
	currIY int32
	// width is the widest row of region to be blitted, in destination
	// coordinates.
	width lengthU32
	// left is the leftmost x coordinate in any row, in destination
	// coordinates.
	left uint32
	// superLeft is the leftmost x coordinate in any row, in supersampled
	// coordinates.
	superLeft uint32

	// currY is the current y coordinate in supersampled coordinates.
	currY int32
	// top is the initial y coordinate (top of bounds).
	top int32
}

// newBaseSuperBlitter is BaseSuperBlitter::new in tiny-skia
// src/scan/path_aa.rs.
func newBaseSuperBlitter(bounds *IntRect, clipRect *screenIntRect, b blitter) (baseSuperBlitter, bool) {
	r, ok := bounds.Intersect(clipRect.ToIntRect())
	if !ok {
		return baseSuperBlitter{}, false
	}
	sect, ok := intRectToScreenIntRect(r)
	if !ok {
		return baseSuperBlitter{}, false
	}
	return baseSuperBlitter{
		realBlitter: b,
		currIY:      int32(sect.Top()) - 1,
		width:       sect.WidthSafe(),
		left:        sect.Left(),
		superLeft:   sect.Left() << shiftPathAA,
		currY:       int32(sect.Top()<<shiftPathAA) - 1,
		top:         int32(sect.Top()),
	}, true
}

// superBlitter is SuperBlitter in tiny-skia src/scan/path_aa.rs.
type superBlitter struct {
	unreachableBlitter

	base    baseSuperBlitter
	runs    *alphaRuns
	offsetX int
}

// newSuperBlitter is SuperBlitter::new in tiny-skia src/scan/path_aa.rs.
func newSuperBlitter(bounds *IntRect, clipRect *screenIntRect, b blitter) (*superBlitter, bool) {
	base, ok := newBaseSuperBlitter(bounds, clipRect, b)
	if !ok {
		return nil, false
	}
	runsWidth := base.width
	return &superBlitter{
		base:    base,
		runs:    newAlphaRuns(runsWidth),
		offsetX: 0,
	}, true
}

// flush is SuperBlitter::flush in tiny-skia src/scan/path_aa.rs.
//
// Once `runs` contains a complete supersampled row, flush() blits it out
// through the wrapped blitter.
func (sb *superBlitter) flush() {
	if sb.base.currIY >= sb.base.top {
		if !sb.runs.isEmpty() {
			if sb.base.currIY < 0 {
				panic("called `Result::unwrap()` on an `Err` value")
			}
			sb.base.realBlitter.blitAntiH(
				sb.base.left,
				uint32(sb.base.currIY),
				sb.runs.alpha,
				sb.runs.runs,
			)
			sb.runs.reset(sb.base.width)
			sb.offsetX = 0
		}

		sb.base.currIY = sb.base.top - 1
	}
}

// blitH is the Blitter impl of SuperBlitter in tiny-skia src/scan/path_aa.rs.
// It blits a row of pixels, with location and width specified in
// supersampled coordinates.
func (sb *superBlitter) blitH(x, y uint32, width lengthU32) {
	iy := int32(y >> shiftPathAA)

	// hack, until I figure out why my cubics (I think) go beyond the bounds
	if n, ok := checkedSubU32(x, sb.base.superLeft); ok {
		x = n
	} else {
		width = x + width
		if width == 0 {
			panic("called `Option::unwrap()` on a `None` value")
		}
		x = 0
	}

	if sb.base.currY != int32(y) {
		sb.offsetX = 0
		sb.base.currY = int32(y)
	}

	if iy != sb.base.currIY {
		// new scanline
		sb.flush()
		sb.base.currIY = iy
	}

	start := x
	stop := x + width

	// integer-pixel-aligned ends of blit, rounded out
	fb := start & maskPathAA
	fe := stop & maskPathAA
	n := (int32(stop) >> shiftPathAA) - (int32(start) >> shiftPathAA) - 1

	if n < 0 {
		fb = fe - fb
		n = 0
		fe = 0
	} else {
		if fb == 0 {
			n++
		} else {
			fb = scalePathAA - fb
		}
	}

	mv := (uint32(1) << (8 - shiftPathAA)) - (((y & maskPathAA) + 1) >> shiftPathAA)
	if mv > 255 {
		panic("called `Result::unwrap()` on an `Err` value")
	}
	maxValue := uint8(mv)
	sb.offsetX = sb.runs.add(
		x>>shiftPathAA,
		coverageToPartialAlpha(fb),
		int(n),
		coverageToPartialAlpha(fe),
		maxValue,
		sb.offsetX,
	)
}

// coverageToPartialAlpha is coverage_to_partial_alpha in tiny-skia
// src/scan/path_aa.rs.
//
// coverage_to_partial_alpha() is being used by AlphaRuns, which *accumulates*
// SCALE pixels worth of "alpha" in [0,(256/SCALE)] to produce a final value in
// [0, 255] and handles clamping 256->255 itself, with the same (alpha - (alpha
// >> 8)) correction as coverage_to_exact_alpha().
func coverageToPartialAlpha(aa uint32) alphaU8 {
	aa <<= 8 - 2*shiftPathAA
	return uint8(aa)
}
