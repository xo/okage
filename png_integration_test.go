// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports tiny-skia tests/integration/png.rs.

import (
	"testing"

	"github.com/xo/okage"
)

func TestPngDecodeGrayscale(t *testing.T) {
	pixmap := check(okage.LoadPNG("testdata/images/pngs/grayscale.png"))
	if got, want := must(pixmap.Pixel(10, 10)), okage.ColorU8FromRGBA(255, 255, 255, 255).Premultiply(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := must(pixmap.Pixel(50, 50)), okage.ColorU8FromRGBA(0, 0, 0, 255).Premultiply(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPngDecodeGrayscaleAlpha(t *testing.T) {
	pixmap := check(okage.LoadPNG("testdata/images/pngs/grayscale-alpha.png"))
	if got, want := must(pixmap.Pixel(10, 10)), okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := must(pixmap.Pixel(50, 50)), okage.ColorU8FromRGBA(0, 0, 0, 255).Premultiply(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPngDecodeRgb(t *testing.T) {
	pixmap := check(okage.LoadPNG("testdata/images/pngs/rgb.png"))
	if got, want := must(pixmap.Pixel(10, 10)), okage.ColorU8FromRGBA(255, 255, 255, 255).Premultiply(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := must(pixmap.Pixel(50, 50)), okage.ColorU8FromRGBA(36, 191, 49, 255).Premultiply(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPngDecodeRgba(t *testing.T) {
	pixmap := check(okage.LoadPNG("testdata/images/pngs/rgba.png"))
	if got, want := must(pixmap.Pixel(10, 10)), okage.ColorU8FromRGBA(0, 0, 0, 0).Premultiply(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := must(pixmap.Pixel(25, 25)), okage.ColorU8FromRGBA(161, 227, 165, 108).Premultiply(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := must(pixmap.Pixel(50, 50)), okage.ColorU8FromRGBA(33, 190, 47, 252).Premultiply(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TODO: test encoding, somehow
