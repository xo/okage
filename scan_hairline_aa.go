// Copyright 2011 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import "github.com/xo/okage/path"

// fixedRect is FixedRect in tiny-skia src/scan/hairline_aa.rs.
type fixedRect struct {
	left   fDot16
	top    fDot16
	right  fDot16
	bottom fDot16
}

// fixedRectFromRect is FixedRect::from_rect in tiny-skia
// src/scan/hairline_aa.rs.
func fixedRectFromRect(src *Rect) fixedRect {
	return fixedRect{
		left:   fdot16FromF32(src.Left()),
		top:    fdot16FromF32(src.Top()),
		right:  fdot16FromF32(src.Right()),
		bottom: fdot16FromF32(src.Bottom()),
	}
}

// alphaMul is alpha_mul in tiny-skia src/scan/hairline_aa.rs.
//
// Multiplies value by 0..256, and shift the result down 8 (i.e. return (value
// * alpha256) >> 8).
func alphaMul(value alphaU8, alpha256 int32) uint8 {
	a := (int32(value) * alpha256) >> 8
	return uint8(a)
}

// fillRectHairlineAA is fill_rect in tiny-skia src/scan/hairline_aa.rs. The
// name takes the suffix of its module, because src/scan/mod.rs has a function
// of the same name (D10).
func fillRectHairlineAA(rect *Rect, clip *screenIntRect, b blitter) {
	r, ok := rect.Intersect(clip.ToRect())
	if !ok {
		return // everything was clipped out
	}

	fr := fixedRectFromRect(&r)
	fillFixedRect(&fr, b)
}

// fillFixedRect is fill_fixed_rect in tiny-skia src/scan/hairline_aa.rs.
func fillFixedRect(rect *fixedRect, b blitter) {
	fillDot8(
		fdot8FromFDot16(rect.left),
		fdot8FromFDot16(rect.top),
		fdot8FromFDot16(rect.right),
		fdot8FromFDot16(rect.bottom),
		true,
		b,
	)
}

// toAlpha is to_alpha, an inner function of fill_dot8 in tiny-skia
// src/scan/hairline_aa.rs.
func toAlpha(a int32) uint8 {
	return uint8(a)
}

// fillDot8 is fill_dot8 in tiny-skia src/scan/hairline_aa.rs.
func fillDot8(l, t, r, bt fDot8, fillInner bool, b blitter) {
	// check for empty now that we are in our reduced precision space
	if l >= r || t >= bt {
		return
	}

	top := t >> 8
	if top == ((bt - 1) >> 8) {
		// just one scanline high
		doScanline(l, top, r, toAlpha(bt-t-1), b)
		return
	}

	if t&0xFF != 0 {
		doScanline(l, top, r, toAlpha(256-(t&0xFF)), b)
		top++
	}

	bottom := bt >> 8
	height := bottom - top
	if height > 0 {
		heightU := uint32(height)
		left := l >> 8
		if left == ((r - 1) >> 8) {
			// just 1-pixel wide
			if left >= 0 && top >= 0 {
				b.blitV(uint32(left), uint32(top), heightU, toAlpha(r-l-1))
			}
		} else {
			if l&0xFF != 0 {
				if left >= 0 && top >= 0 {
					b.blitV(uint32(left), uint32(top), heightU, toAlpha(256-(l&0xFF)))
				}

				left++
			}

			right := r >> 8
			width := right - left
			if fillInner {
				if width > 0 {
					if left >= 0 && top >= 0 {
						rect := screenIntRectFromXYWHSafe(uint32(left), uint32(top), uint32(width), heightU)
						b.blitRect(&rect)
					}
				}
			}

			if r&0xFF != 0 {
				if right >= 0 && top >= 0 {
					b.blitV(uint32(right), uint32(top), heightU, toAlpha(r&0xFF))
				}
			}
		}
	}

	if bt&0xFF != 0 {
		doScanline(l, bottom, r, toAlpha(bt&0xFF), b)
	}
}

// doScanline is do_scanline in tiny-skia src/scan/hairline_aa.rs.
func doScanline(l fDot8, top int32, r fDot8, alpha alphaU8, b blitter) {
	oneLen := lengthU32One
	if top < 0 {
		return
	}
	topU := uint32(top)

	if (l >> 8) == ((r - 1) >> 8) {
		// 1x1 pixel
		if left := l >> 8; left >= 0 {
			b.blitV(uint32(left), topU, oneLen, alphaMul(alpha, r-l))
		}

		return
	}

	left := l >> 8

	if l&0xFF != 0 {
		if left := l >> 8; left >= 0 {
			b.blitV(uint32(left), topU, oneLen, alphaMul(alpha, 256-(l&0xFF)))
		}

		left++
	}

	right := r >> 8
	width := right - left
	if width > 0 {
		if left >= 0 {
			y := topU
			callHLineBlitter(uint32(left), &y, uint32(width), alpha, b)
		}
	}

	if r&0xFF != 0 {
		if right >= 0 {
			b.blitV(uint32(right), topU, oneLen, alphaMul(alpha, r&0xFF))
		}
	}
}

// hlineStackBuffer is HLINE_STACK_BUFFER, a constant of call_hline_blitter in
// tiny-skia src/scan/hairline_aa.rs.
const hlineStackBuffer = 100

// callHLineBlitter is call_hline_blitter in tiny-skia
// src/scan/hairline_aa.rs.
//
// Port: a nil y is the Rust None.
func callHLineBlitter(x uint32, y *uint32, count lengthU32, alpha alphaU8, b blitter) {
	var runs [hlineStackBuffer + 1]alphaRun
	var aa [hlineStackBuffer]uint8

	for {
		// In theory, we can just do this once (outside of the loop),
		// since aa[] and runs[] are supposed" to be const when we call the blitter.
		// In reality, some wrapper-blitters (e.g. RgnClipBlitter) cast away that
		// constness, and modify the buffers in-place. Hence the need to be defensive
		// here and reseed the aa value.
		aa[0] = alpha

		n := count
		if n > hlineStackBuffer { //nolint:modernize // the shape of upstream is kept
			n = hlineStackBuffer
		}

		runs[0] = uint16(n)
		runs[n] = 0
		if y != nil {
			b.blitAntiH(x, *y, aa[:], runs[:])
		}
		x += n

		if n >= count || count == 0 {
			break
		}

		count -= n
	}
}

// strokePathHairlineAA is stroke_path in tiny-skia src/scan/hairline_aa.rs.
// The name takes the suffix of its module, because src/scan/hairline.rs has a
// function of the same name (D10).
func strokePathHairlineAA(p *Path, lineCap LineCap, clip *screenIntRect, b blitter) {
	strokePathImpl(p, lineCap, clip, antiHairLineRgn, b)
}

// antiHairLineRgn is anti_hair_line_rgn in tiny-skia src/scan/hairline_aa.rs.
func antiHairLineRgn(points []Point, clip *screenIntRect, b blitter) {
	const max = 32767.0
	fixedBounds, ok := path.RectFromLTRB(-max, -max, max, max)
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}

	var clipBounds *Rect
	if clip != nil {
		// We perform integral clipping later on, but we do a scalar clip first
		// to ensure that our coordinates are expressible in fixed/integers.
		//
		// antialiased hairlines can draw up to 1/2 of a pixel outside of
		// their bounds, so we need to outset the clip before calling the
		// clipper. To make the numerics safer, we outset by a whole pixel,
		// since the 1/2 pixel boundary is important to the antihair blitter,
		// we do not want to risk numerical fate by chopping on that edge.
		if r, ok := clip.ToRect().Outset(1.0, 1.0); ok {
			clipBounds = &r
		}
	}

	for i := range len(points) - 1 {
		var pts [2]Point

		// We have to pre-clip the line to fit in a Fixed, so we just chop the line.
		if !intersect(&[2]Point{points[i], points[i+1]}, &fixedBounds, &pts) {
			continue
		}

		if clipBounds != nil {
			tmp := pts
			if !intersect(&tmp, clipBounds, &pts) {
				continue
			}
		}

		x0 := fdot6FromF32(pts[0].X)
		y0 := fdot6FromF32(pts[0].Y)
		x1 := fdot6FromF32(pts[1].X)
		y1 := fdot6FromF32(pts[1].Y)

		if clip != nil {
			left := minI32(x0, x1)
			top := minI32(y0, y1)
			right := maxI32(x0, x1)
			bottom := maxI32(y0, y1)

			ir, ok := path.IntRectFromLTRB(
				fdot6Floor(left)-1,
				fdot6Floor(top)-1,
				fdot6Ceil(right)+1,
				fdot6Ceil(bottom)+1,
			)
			if !ok {
				return
			}

			if _, ok := clip.ToIntRect().Intersect(ir); !ok {
				continue
			}

			if !clip.ToIntRect().Contains(ir) {
				if r, ok := ir.Intersect(clip.ToIntRect()); ok {
					if subclip, ok := intRectToScreenIntRect(r); ok {
						doAntiHairline(x0, y0, x1, y1, &subclip, b)
					}
				}

				continue
			}

			// fall through to no-clip case
		}

		doAntiHairline(x0, y0, x1, y1, nil, b)
	}
}

// blitterKind is BlitterKind in tiny-skia src/scan/hairline_aa.rs.
type blitterKind uint8

// The values of blitterKind.
const (
	blitterKindHLine blitterKind = iota
	blitterKindHorish
	blitterKindVLine
	blitterKindVertish
)

// doAntiHairline is do_anti_hairline in tiny-skia src/scan/hairline_aa.rs.
//
// Port: a nil clipOpt is the Rust None.
func doAntiHairline(x0, y0, x1, y1 fDot6, clipOpt *screenIntRect, b blitter) {
	// check for integer NaN (0x80000000) which we cannot handle (cannot negate it)
	// It appears typically from a huge float (inf or nan) being converted to int.
	// If we see it, just do not draw.
	if anyBadInts(x0, y0, x1, y1) != 0 {
		return
	}

	// The caller must clip the line to [-32767.0 ... 32767.0] ahead of time  (in dot6 format)

	if absI32(x1-x0) > fdot6FromI32(511) || absI32(y1-y0) > fdot6FromI32(511) {
		// instead of (x0 + x1) >> 1, we shift each separately. This is less
		// precise, but avoids overflowing the intermediate result if the
		// values are huge. A better fix is to clip the original pts
		// directly (i.e. do the divide), so we do not spend time subdividing
		// huge lines at all.
		hx := (x0 >> 1) + (x1 >> 1)
		hy := (y0 >> 1) + (y1 >> 1)
		doAntiHairline(x0, y0, hx, hy, clipOpt, b)
		doAntiHairline(hx, hy, x1, y1, clipOpt, b)
		return // we are done
	}

	var scaleStart int32
	var scaleStop int32
	var istart int32
	var istop int32
	var fstart fDot16
	var slope fDot16
	var kind blitterKind

	if absI32(x1-x0) > absI32(y1-y0) {
		// mostly horizontal

		if x0 > x1 {
			// we want to go left-to-right
			x0, x1 = x1, x0
			y0, y1 = y1, y0
		}

		istart = fdot6Floor(x0)
		istop = fdot6Ceil(x1)
		fstart = fdot6ToFDot16(y0)
		if y0 == y1 {
			// completely horizontal, take fast case
			slope = 0
			kind = blitterKindHLine
		} else {
			slope = fdot16FastDiv(y1-y0, x1-x0)
			fstart += (slope*(32-(x0&63)) + 32) >> 6
			kind = blitterKindHorish
		}

		if istop-istart == 1 {
			// we are within a single pixel
			scaleStart = x1 - x0
			scaleStop = 0
		} else {
			scaleStart = 64 - (x0 & 63)
			scaleStop = x1 & 63
		}

		if clipOpt != nil { //nolint:dupl // upstream has the same shape for each direction
			clip := clipOpt.ToIntRect()

			if istart >= clip.Right() || istop <= clip.Left() {
				return // we are done
			}

			if istart < clip.Left() {
				fstart += slope * (clip.Left() - istart)
				istart = clip.Left()
				scaleStart = 64
				if istop-istart == 1 {
					// we are within a single pixel
					scaleStart = contribution64(x1)
					scaleStop = 0
				}
			}

			if istop > clip.Right() {
				istop = clip.Right()
				scaleStop = 0 // so we do not draw this last column
			}

			if istart == istop {
				return // we are done
			}

			// now test if our Y values are completely inside the clip
			var top, bottom int32
			if slope >= 0 {
				// T2B
				top = fdot16FloorToI32(fstart - fdot16Half)
				bottom = fdot16CeilToI32(fstart + (istop-istart-1)*slope + fdot16Half)
			} else {
				// B2T
				bottom = fdot16CeilToI32(fstart + fdot16Half)
				top = fdot16FloorToI32(fstart + (istop-istart-1)*slope - fdot16Half)
			}

			top--
			bottom++

			if top >= clip.Bottom() || bottom <= clip.Top() {
				return // we are done
			}

			if clip.Top() <= top && clip.Bottom() >= bottom {
				clipOpt = nil
			}
		}
	} else {
		// mostly vertical

		if y0 > y1 {
			// we want to go top-to-bottom
			x0, x1 = x1, x0
			y0, y1 = y1, y0
		}

		istart = fdot6Floor(y0)
		istop = fdot6Ceil(y1)
		fstart = fdot6ToFDot16(x0)
		if x0 == x1 {
			if y0 == y1 {
				// are we zero length? nothing to do
				return // we are done
			}

			slope = 0
			kind = blitterKindVLine
		} else {
			slope = fdot16FastDiv(x1-x0, y1-y0)
			fstart += (slope*(32-(y0&63)) + 32) >> 6
			kind = blitterKindVertish
		}

		if istop-istart == 1 {
			// we are within a single pixel
			scaleStart = y1 - y0
			scaleStop = 0
		} else {
			scaleStart = 64 - (y0 & 63)
			scaleStop = y1 & 63
		}

		if clipOpt != nil { //nolint:dupl // upstream has the same shape for each direction
			clip := clipOpt.ToIntRect()

			if istart >= clip.Bottom() || istop <= clip.Top() {
				return // we are done
			}

			if istart < clip.Top() {
				fstart += slope * (clip.Top() - istart)
				istart = clip.Top()
				scaleStart = 64
				if istop-istart == 1 {
					// we are within a single pixel
					scaleStart = contribution64(y1)
					scaleStop = 0
				}
			}
			if istop > clip.Bottom() {
				istop = clip.Bottom()
				scaleStop = 0 // so we do not draw this last row
			}

			if istart == istop {
				return // we are done
			}

			// now test if our X values are completely inside the clip
			var left, right int32
			if slope >= 0 {
				// L2R
				left = fdot16FloorToI32(fstart - fdot16Half)
				right = fdot16CeilToI32(fstart + (istop-istart-1)*slope + fdot16Half)
			} else {
				// R2L
				right = fdot16CeilToI32(fstart + fdot16Half)
				left = fdot16FloorToI32(fstart + (istop-istart-1)*slope - fdot16Half)
			}

			left--
			right++

			if left >= clip.Right() || right <= clip.Left() {
				return // we are done
			}

			if clip.Left() <= left && clip.Right() >= right {
				clipOpt = nil
			}
		}
	}

	if clipOpt != nil {
		b = &rectClipBlitter{blitter: b, clip: *clipOpt}
	}

	// A bit ugly, but looks like this is the only way to have stack allocated object trait.
	var hairBlitter antiHairBlitter
	switch kind {
	case blitterKindHLine:
		hairBlitter = hlineAntiHairBlitter{b}
	case blitterKindHorish:
		hairBlitter = horishAntiHairBlitter{b}
	case blitterKindVLine:
		hairBlitter = vlineAntiHairBlitter{b}
	case blitterKindVertish:
		hairBlitter = vertishAntiHairBlitter{b}
	}

	istartU := uint32(istart)

	istopU := uint32(istop)

	fstart = hairBlitter.drawCap(istartU, fstart, slope, scaleStart)
	istartU++
	var stopAdj uint32
	if scaleStop > 0 {
		stopAdj = 1
	}
	fullSpans := istopU - istartU - stopAdj
	if fullSpans > 0 {
		fstart = hairBlitter.drawLine(istartU, istartU+fullSpans, fstart, slope)
	}

	if scaleStop > 0 {
		hairBlitter.drawCap(istopU-1, fstart, slope, scaleStop)
	}
}

// badInt is bad_int in tiny-skia src/scan/hairline_aa.rs. It returns the high
// bit set if x == 0x8000.
func badInt(x int32) int32 {
	return x & -x
}

// anyBadInts is any_bad_ints in tiny-skia src/scan/hairline_aa.rs.
func anyBadInts(a, b, c, d int32) int32 {
	return (badInt(a) | badInt(b) | badInt(c) | badInt(d)) >> ((4 << 3) - 1)
}

// contribution64 is contribution_64 in tiny-skia src/scan/hairline_aa.rs.
//
// We want the fractional part of ordinate, but we want multiples of 64 to
// return 64, not 0, so we cannot just say (ordinate & 63). We basically want to
// compute those bits, and if they are 0, return 64. We can do that w/o a branch
// with an extra sub and add.
func contribution64(ordinate fDot6) int32 {
	return ((ordinate - 1) & 63) + 1
}

// antiHairBlitter is AntiHairBlitter in tiny-skia src/scan/hairline_aa.rs.
type antiHairBlitter interface {
	drawCap(x uint32, fy, slope fDot16, mod64 int32) fDot16
	drawLine(x, stopX uint32, fy, slope fDot16) fDot16
}

// hlineAntiHairBlitter is HLineAntiHairBlitter in tiny-skia
// src/scan/hairline_aa.rs.
type hlineAntiHairBlitter struct {
	b blitter
}

// drawCap is the AntiHairBlitter impl of HLineAntiHairBlitter in tiny-skia
// src/scan/hairline_aa.rs.
func (h hlineAntiHairBlitter) drawCap(x uint32, fy, _ fDot16, mod64 int32) fDot16 {
	fy += fdot16One / 2
	fy = maxI32(fy, 0)

	y := uint32(fy >> 16)
	a := i32ToAlpha(fy >> 8)

	// lower line
	ma := fdot6SmallScale(a, mod64)
	if ma != 0 {
		callHLineBlitter(x, &y, lengthU32One, ma, h.b)
	}

	// upper line
	ma = fdot6SmallScale(255-a, mod64)
	if ma != 0 {
		var yp *uint32
		if y1, ok := checkedSubU32(y, 1); ok {
			yp = &y1
		}
		callHLineBlitter(x, yp, lengthU32One, ma, h.b)
	}

	return fy - fdot16One/2
}

// drawLine is the AntiHairBlitter impl of HLineAntiHairBlitter in tiny-skia
// src/scan/hairline_aa.rs.
func (h hlineAntiHairBlitter) drawLine(x, stopX uint32, fy, _ fDot16) fDot16 {
	count := stopX - x
	if count == 0 {
		return fy
	}

	fy += fdot16One / 2
	fy = maxI32(fy, 0)

	y := uint32(fy >> 16)
	a := i32ToAlpha(fy >> 8)

	// lower line
	if a != 0 {
		callHLineBlitter(x, &y, count, a, h.b)
	}

	// upper line
	a = 255 - a
	if a != 0 {
		var yp *uint32
		if y1, ok := checkedSubU32(y, 1); ok {
			yp = &y1
		}
		callHLineBlitter(x, yp, count, a, h.b)
	}

	return fy - fdot16One/2
}

// horishAntiHairBlitter is HorishAntiHairBlitter in tiny-skia
// src/scan/hairline_aa.rs.
type horishAntiHairBlitter struct {
	b blitter
}

// drawCap is the AntiHairBlitter impl of HorishAntiHairBlitter in tiny-skia
// src/scan/hairline_aa.rs.
func (h horishAntiHairBlitter) drawCap(x uint32, fy, dy fDot16, mod64 int32) fDot16 {
	fy += fdot16One / 2
	fy = maxI32(fy, 0)

	lowerY := uint32(fy >> 16)
	a := i32ToAlpha(fy >> 8)
	a0 := fdot6SmallScale(255-a, mod64)
	a1 := fdot6SmallScale(a, mod64)
	h.b.blitAntiV2(x, maxU32(lowerY, 1)-1, a0, a1)

	return fy + dy - fdot16One/2
}

// drawLine is the AntiHairBlitter impl of HorishAntiHairBlitter in tiny-skia
// src/scan/hairline_aa.rs.
func (h horishAntiHairBlitter) drawLine(x, stopX uint32, fy, dy fDot16) fDot16 {
	fy += fdot16One / 2
	for {
		fy = maxI32(fy, 0)
		lowerY := uint32(fy >> 16)
		a := i32ToAlpha(fy >> 8)
		h.b.blitAntiV2(x, maxU32(lowerY, 1)-1, 255-a, a)
		fy += dy

		x++
		if x >= stopX {
			break
		}
	}

	return fy - fdot16One/2
}

// vlineAntiHairBlitter is VLineAntiHairBlitter in tiny-skia
// src/scan/hairline_aa.rs.
type vlineAntiHairBlitter struct {
	b blitter
}

// drawCap is the AntiHairBlitter impl of VLineAntiHairBlitter in tiny-skia
// src/scan/hairline_aa.rs.
func (h vlineAntiHairBlitter) drawCap(y uint32, fx, _ fDot16, mod64 int32) fDot16 {
	fx += fdot16One / 2
	fx = maxI32(fx, 0)

	x := uint32(fx >> 16)
	a := i32ToAlpha(fx >> 8)

	ma := fdot6SmallScale(a, mod64)
	if ma != 0 {
		h.b.blitV(x, y, lengthU32One, ma)
	}

	ma = fdot6SmallScale(255-a, mod64)
	if ma != 0 {
		h.b.blitV(maxU32(x, 1)-1, y, lengthU32One, ma)
	}

	return fx - fdot16One/2
}

// drawLine is the AntiHairBlitter impl of VLineAntiHairBlitter in tiny-skia
// src/scan/hairline_aa.rs.
func (h vlineAntiHairBlitter) drawLine(y, stopY uint32, fx, _ fDot16) fDot16 {
	height := stopY - y
	if height == 0 {
		return fx
	}

	fx += fdot16One / 2
	fx = maxI32(fx, 0)

	x := uint32(fx >> 16)
	a := i32ToAlpha(fx >> 8)

	if a != 0 {
		h.b.blitV(x, y, height, a)
	}

	a = 255 - a
	if a != 0 {
		h.b.blitV(maxU32(x, 1)-1, y, height, a)
	}

	return fx - fdot16One/2
}

// vertishAntiHairBlitter is VertishAntiHairBlitter in tiny-skia
// src/scan/hairline_aa.rs.
type vertishAntiHairBlitter struct {
	b blitter
}

// drawCap is the AntiHairBlitter impl of VertishAntiHairBlitter in tiny-skia
// src/scan/hairline_aa.rs.
func (h vertishAntiHairBlitter) drawCap(y uint32, fx, dx fDot16, mod64 int32) fDot16 {
	fx += fdot16One / 2
	fx = maxI32(fx, 0)

	x := uint32(fx >> 16)
	a := i32ToAlpha(fx >> 8)
	h.b.blitAntiH2(
		maxU32(x, 1)-1,
		y,
		fdot6SmallScale(255-a, mod64),
		fdot6SmallScale(a, mod64),
	)

	return fx + dx - fdot16One/2
}

// drawLine is the AntiHairBlitter impl of VertishAntiHairBlitter in tiny-skia
// src/scan/hairline_aa.rs.
func (h vertishAntiHairBlitter) drawLine(y, stopY uint32, fx, dx fDot16) fDot16 {
	fx += fdot16One / 2
	for {
		fx = maxI32(fx, 0)
		x := uint32(fx >> 16)
		a := i32ToAlpha(fx >> 8)
		h.b.blitAntiH2(maxU32(x, 1)-1, y, 255-a, a)
		fx += dx

		y++
		if y >= stopY {
			break
		}
	}

	return fx - fdot16One/2
}

// i32ToAlpha is i32_to_alpha in tiny-skia src/scan/hairline_aa.rs.
func i32ToAlpha(a int32) uint8 {
	return uint8(a & 0xFF)
}

// rectClipBlitter is RectClipBlitter in tiny-skia src/scan/hairline_aa.rs.
type rectClipBlitter struct {
	unreachableBlitter

	blitter blitter
	clip    screenIntRect
}

// yInRect is y_in_rect, an inner function of RectClipBlitter::blit_anti_h in
// tiny-skia src/scan/hairline_aa.rs.
func yInRect(y uint32, rect screenIntRect) bool {
	return (y - rect.Top()) < rect.Height()
}

// blitAntiH is the Blitter impl of RectClipBlitter in tiny-skia
// src/scan/hairline_aa.rs.
func (rc *rectClipBlitter) blitAntiH(x, y uint32, antialias []alphaU8, runs []alphaRun) {
	if !yInRect(y, rc.clip) || x >= rc.clip.Right() {
		return
	}

	x0 := x
	x1 := x + computeAntiWidth(runs)

	if x1 <= rc.clip.Left() {
		return
	}

	if x0 < rc.clip.Left() {
		dx := rc.clip.Left() - x0
		breakAt(antialias, runs, int32(dx))
		antialias = antialias[dx:]
		runs = runs[dx:]
		x0 = rc.clip.Left()
	}

	if x1 > rc.clip.Right() {
		x1 = rc.clip.Right()
		breakAt(antialias, runs, int32(x1-x0))
		runs[x1-x0] = 0
	}

	rc.blitter.blitAntiH(x0, y, antialias, runs)
}

// xInRect is x_in_rect, an inner function of RectClipBlitter::blit_v in
// tiny-skia src/scan/hairline_aa.rs.
func xInRect(x uint32, rect screenIntRect) bool {
	return (x - rect.Left()) < rect.Width()
}

// blitV is the Blitter impl of RectClipBlitter in tiny-skia
// src/scan/hairline_aa.rs.
func (rc *rectClipBlitter) blitV(x, y uint32, height lengthU32, alpha alphaU8) {
	if !xInRect(x, rc.clip) {
		return
	}

	y0 := y
	y1 := y + height

	if y0 < rc.clip.Top() {
		y0 = rc.clip.Top()
	}

	if y1 > rc.clip.Bottom() {
		y1 = rc.clip.Bottom()
	}

	if y0 < y1 {
		if h := y1 - y0; h != 0 {
			rc.blitter.blitV(x, y0, h, alpha)
		}
	}
}

// blitAntiH2 is the Blitter impl of RectClipBlitter in tiny-skia
// src/scan/hairline_aa.rs.
func (rc *rectClipBlitter) blitAntiH2(x, y uint32, alpha0, alpha1 alphaU8) {
	rc.blitAntiH(
		x,
		y,
		[]alphaU8{alpha0, alpha1},
		[]alphaRun{1, 1, 0},
	)
}

// blitAntiV2 is the Blitter impl of RectClipBlitter in tiny-skia
// src/scan/hairline_aa.rs.
func (rc *rectClipBlitter) blitAntiV2(x, y uint32, alpha0, alpha1 alphaU8) {
	rc.blitAntiH(x, y, []alphaU8{alpha0}, []alphaRun{1, 0})

	rc.blitAntiH(x, y+1, []alphaU8{alpha1}, []alphaRun{1, 0})
}

// computeAntiWidth is compute_anti_width in tiny-skia src/scan/hairline_aa.rs.
func computeAntiWidth(runs []alphaRun) uint32 {
	i := 0
	width := uint32(0)
	for runs[i] != 0 {
		count := runs[i]
		width += uint32(count)
		i += int(count)
	}

	return width
}
