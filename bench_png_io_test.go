// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports the benchmarks of tiny-skia in benches/src/png_io.rs that
// measure the code of tiny-skia: decode_rgb, decode_rgba and encode_rgba.
// decode_raw_rgb, decode_raw_rgba and encode_raw_rgba measure only the crate
// png, which okage does not port (D7), so they are not ported.

import (
	"os"
	"testing"

	"github.com/xo/okage"
)

func BenchmarkPNGIODecodeRGB(b *testing.B) {
	data := check(os.ReadFile("testdata/images/pngs/rgb.png"))
	for b.Loop() {
		_ = check(okage.DecodePNG(data))
	}
}

func BenchmarkPNGIODecodeRGBA(b *testing.B) {
	data := check(os.ReadFile("testdata/images/pngs/rgba.png"))
	for b.Loop() {
		_ = check(okage.DecodePNG(data))
	}
}

func BenchmarkPNGIOEncodeRGBA(b *testing.B) {
	pixmap := check(okage.LoadPNG("testdata/images/pngs/rgba.png"))
	for b.Loop() {
		_ = check(pixmap.EncodePNG())
	}
}
