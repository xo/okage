// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

// maskBlitter is Mask in tiny-skia src/blitter.rs. It describes an alpha
// bitmap.
//
// Port: src/mask.rs has a type of the same name, so this one takes the suffix
// of its module (D10).
type maskBlitter struct {
	image    [2]uint8
	bounds   screenIntRect
	rowBytes uint32
}

// blitter is Blitter in tiny-skia src/blitter.rs.
//
// A blitter is responsible for actually writing pixels into memory.
//
// Besides efficiency, blitters handle clipping and antialiasing. An object
// that implements blitter contains all the context needed to generate pixels
// for the destination and how src/generated pixels map to the destination.
// The coordinates passed to the blit* calls are in destination pixel space.
//
// Port: each method of the Rust trait has a default body that is
// unreachable!(), and each implementation overrides only the methods that it
// needs. In Go, a type that implements blitter embeds unreachableBlitter,
// whose methods panic, and declares only the methods that its Rust impl
// declares. A method of the type hides the method of the embedded value.
type blitter interface {
	// blitH is Blitter::blit_h. It blits a horizontal run of one or more
	// pixels.
	blitH(x, y uint32, width lengthU32)

	// blitAntiH is Blitter::blit_anti_h. It blits a horizontal run of
	// antialiased pixels.
	//
	// runs[] is a *sparse* zero-terminated run-length encoding of spans of
	// constant alpha values.
	//
	// The runs[] and antialias[] work together to represent long runs of
	// pixels with the same alphas. The runs[] contains the number of pixels
	// with the same alpha, and antialias[] contain the coverage value for that
	// number of pixels. The runs[] (and antialias[]) are encoded in a clever
	// way. The runs array is zero terminated, and has enough entries for each
	// pixel plus one, in most cases some of the entries will not contain valid
	// data. An entry in the runs array contains the number of pixels (np) that
	// have the same alpha value. The next np value is found np entries away.
	// For example, if runs[0] = 7, then the next valid entry will by at
	// runs[7]. The runs array and antialias[] are coupled by index. So, if the
	// np entry is at runs[45] = 12 then the alpha value can be found at
	// antialias[45] = 0x88. This means to use an alpha value of 0x88 for
	// the next 12 pixels starting at pixel 45.
	blitAntiH(x, y uint32, antialias []alphaU8, runs []alphaRun)

	// blitV is Blitter::blit_v. It blits a vertical run of pixels with a
	// constant alpha value.
	blitV(x, y uint32, height lengthU32, alpha alphaU8)

	// blitAntiH2 is Blitter::blit_anti_h2.
	blitAntiH2(x, y uint32, alpha0, alpha1 alphaU8)

	// blitAntiV2 is Blitter::blit_anti_v2.
	blitAntiV2(x, y uint32, alpha0, alpha1 alphaU8)

	// blitRect is Blitter::blit_rect. It blits a solid rectangle one or more
	// pixels wide.
	blitRect(rect *screenIntRect)

	// blitMask is Blitter::blit_mask. It blits a pattern of pixels defined by
	// a rectangle-clipped mask.
	blitMask(mask *maskBlitter, clip *screenIntRect)
}

// unreachableBlitter holds the default methods of the trait Blitter in
// tiny-skia src/blitter.rs. Each one panics, as unreachable!() does.
type unreachableBlitter struct{}

// blitH is the default Blitter::blit_h. It panics.
func (unreachableBlitter) blitH(_, _ uint32, _ lengthU32) {
	panic("internal error: entered unreachable code")
}

// blitAntiH is the default Blitter::blit_anti_h. It panics.
func (unreachableBlitter) blitAntiH(_, _ uint32, _ []alphaU8, _ []alphaRun) {
	panic("internal error: entered unreachable code")
}

// blitV is the default Blitter::blit_v. It panics.
func (unreachableBlitter) blitV(_, _ uint32, _ lengthU32, _ alphaU8) {
	panic("internal error: entered unreachable code")
}

// blitAntiH2 is the default Blitter::blit_anti_h2. It panics.
func (unreachableBlitter) blitAntiH2(_, _ uint32, _, _ alphaU8) {
	panic("internal error: entered unreachable code")
}

// blitAntiV2 is the default Blitter::blit_anti_v2. It panics.
func (unreachableBlitter) blitAntiV2(_, _ uint32, _, _ alphaU8) {
	panic("internal error: entered unreachable code")
}

// blitRect is the default Blitter::blit_rect. It panics.
func (unreachableBlitter) blitRect(_ *screenIntRect) {
	panic("internal error: entered unreachable code")
}

// blitMask is the default Blitter::blit_mask. It panics.
func (unreachableBlitter) blitMask(_ *maskBlitter, _ *screenIntRect) {
	panic("internal error: entered unreachable code")
}
