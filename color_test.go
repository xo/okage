// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"bytes"
	"math"
	"testing"
)

// TestPremultiplyU8 is premultiply_u8 in the tests of tiny-skia src/color.rs.
func TestPremultiplyU8(t *testing.T) {
	got := ColorU8FromRGBA(10, 20, 30, 40).Premultiply()
	want := premultipliedColorU8FromRGBAUnchecked(2, 3, 5, 40)
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestPremultiplyU8Opaque is premultiply_u8_opaque in the tests of tiny-skia
// src/color.rs.
func TestPremultiplyU8Opaque(t *testing.T) {
	got := ColorU8FromRGBA(10, 20, 30, 255).Premultiply()
	want := premultipliedColorU8FromRGBAUnchecked(10, 20, 30, 255)
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestDemultiplyU8_1 is demultiply_u8_1 in the tests of tiny-skia
// src/color.rs.
func TestDemultiplyU8_1(t *testing.T) {
	got := premultipliedColorU8FromRGBAUnchecked(2, 3, 5, 40).Demultiply()
	want := ColorU8FromRGBA(13, 19, 32, 40)
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestDemultiplyU8_2 is demultiply_u8_2 in the tests of tiny-skia
// src/color.rs.
func TestDemultiplyU8_2(t *testing.T) {
	got := premultipliedColorU8FromRGBAUnchecked(10, 20, 30, 255).Demultiply()
	want := ColorU8FromRGBA(10, 20, 30, 255)
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestDemultiplyU8_3 is demultiply_u8_3 in the tests of tiny-skia
// src/color.rs.
func TestDemultiplyU8_3(t *testing.T) {
	got := premultipliedColorU8FromRGBAUnchecked(153, 99, 54, 180).Demultiply()
	want := ColorU8FromRGBA(217, 140, 77, 180)
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestBytemuckCastsRGBA is bytemuck_casts_rgba in the tests of tiny-skia
// src/color.rs.
//
// Port: Go has no bytemuck cast. The test writes the colors into a PixelSlice
// and reads its bytes.
func TestBytemuckCastsRGBA(t *testing.T) {
	s := make(PixelSlice, 8)
	s.Set(0, premultipliedColorU8FromRGBAUnchecked(0, 1, 2, 3))
	s.Set(1, premultipliedColorU8FromRGBAUnchecked(10, 11, 12, 13))
	if want := []byte{0, 1, 2, 3, 10, 11, 12, 13}; !bytes.Equal(s, want) {
		t.Errorf("got %v, want %v", []byte(s), want)
	}
	if s.At(1) != premultipliedColorU8FromRGBAUnchecked(10, 11, 12, 13) {
		t.Errorf("At(1) = %v", s.At(1))
	}
}

// TestApproxPowf checks approxPowf. It ports no upstream test. The expected
// bits came from approx_powf of tiny-skia 0.12.0.
func TestApproxPowf(t *testing.T) {
	if got := approxPowf(0, 2.2); got != 0 {
		t.Errorf("approxPowf(0, 2.2) = %v", got)
	}
	if got := approxPowf(1, 2.2); got != 1 {
		t.Errorf("approxPowf(1, 2.2) = %v", got)
	}
	for _, c := range [][3]uint32{
		{0x3de06ac0, 0x3f4bd0e0, 0x3e301800},
		{0x3f62b841, 0x4020762a, 0x3f3cbf00},
		{0x3ea6b91d, 0x3fd738aa, 0x3e1b3940},
		{0x3f4b3af6, 0x3f96556a, 0x3f433300},
		{0x3f50af98, 0x3f108371, 0x3f6417c0},
		{0x3f7d7052, 0x403cbf39, 0x3f787f80},
		{0x3e2585b3, 0x401e9cf7, 0x3c330900},
		{0x3ef4b637, 0x3f7f8ac6, 0x3ef502c0},
	} {
		x := math.Float32frombits(c[0])
		y := math.Float32frombits(c[1])
		if got := math.Float32bits(approxPowf(x, y)); got != c[2] {
			t.Errorf("approxPowf(%v, %v) = %#08x, want %#08x", x, y, got, c[2])
		}
	}
}

// TestColorSpaceString checks the String method of ColorSpace. It ports no
// upstream test.
func TestColorSpaceString(t *testing.T) {
	for cs, want := range map[ColorSpace]string{
		ColorSpaceLinear:        "Linear",
		ColorSpaceGamma2:        "Gamma2",
		ColorSpaceSimpleSRGB:    "SimpleSRGB",
		ColorSpaceFullSRGBGamma: "FullSRGBGamma",
	} {
		if got := cs.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}
