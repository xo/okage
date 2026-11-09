// Copyright 2011 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

// fillRect is fill_rect in tiny-skia src/scan/mod.rs.
func fillRect(rect *Rect, clip *screenIntRect, b blitter) {
	if r, ok := rect.Round(); ok {
		fillIntRect(&r, clip, b)
	}
}

// fillIntRect is fill_int_rect in tiny-skia src/scan/mod.rs.
func fillIntRect(rect *IntRect, clip *screenIntRect, b blitter) {
	r, ok := rect.Intersect(clip.ToIntRect())
	if !ok {
		return // everything was clipped out
	}

	sr, ok := intRectToScreenIntRect(r)
	if !ok {
		return
	}

	b.blitRect(&sr)
}

// fillRectAA is fill_rect_aa in tiny-skia src/scan/mod.rs.
func fillRectAA(rect *Rect, clip *screenIntRect, b blitter) {
	fillRectHairlineAA(rect, clip, b)
}
