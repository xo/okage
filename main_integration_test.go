// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports tiny-skia tests/integration/main.rs. In Rust, that file
// only names the modules of the tests. Each module is a file
// <name>_integration_test.go in Go. This file holds the helpers that the
// tests share.

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/xo/okage"
)

// must is the unwrap of an Option in Rust. It returns v, and it panics if ok
// is false, as unwrap does.
func must[T any](v T, ok bool) T {
	if !ok {
		panic("called unwrap on a None value")
	}
	return v
}

// check is the unwrap of a Result in Rust. It returns v, and it panics if err
// is not nil, as unwrap does.
func check[T any](v T, err error) T {
	if err != nil {
		panic(fmt.Sprintf("called unwrap on an Err value: %v", err))
	}
	return v
}

// assertPixmapEqual is assert_eq! for two pixmaps. Two pixmaps are equal when
// they have the same size and the same bytes, as the derived PartialEq of
// Pixmap says. If they differ, it reports the first pixel that differs.
func assertPixmapEqual(t *testing.T, got, want *okage.Pixmap) {
	t.Helper()
	if got.Width() != want.Width() || got.Height() != want.Height() {
		t.Fatalf("got a pixmap of %dx%d, want %dx%d", got.Width(), got.Height(), want.Width(), want.Height())
	}
	assertBytesEqual(t, got.Data(), want.Data(), got.Width(), 4)
}

// assertMaskEqual is assert_eq! for two masks. Two masks are equal when they
// have the same size and the same bytes, as the derived PartialEq of Mask
// says.
func assertMaskEqual(t *testing.T, got, want *okage.Mask) {
	t.Helper()
	if got.Width() != want.Width() || got.Height() != want.Height() {
		t.Fatalf("got a mask of %dx%d, want %dx%d", got.Width(), got.Height(), want.Width(), want.Height())
	}
	assertBytesEqual(t, got.Data(), want.Data(), got.Width(), 1)
}

// assertBytesEqual compares the bytes of two images with the same size. Each
// pixel has n bytes, and a row has width pixels.
func assertBytesEqual(t *testing.T, got, want []byte, width uint32, n int) {
	t.Helper()
	if bytes.Equal(got, want) {
		return
	}
	count, first := 0, -1
	for i := 0; i+n <= len(got) && i+n <= len(want); i += n {
		if !bytes.Equal(got[i:i+n], want[i:i+n]) {
			count++
			if first < 0 {
				first = i
			}
		}
	}
	if first < 0 {
		t.Fatalf("got %d bytes, want %d", len(got), len(want))
	}
	p := uint32(first / n)
	t.Fatalf("%d pixels differ. The first is at (%d, %d): got %v, want %v",
		count, p%width, p/width, got[first:first+n], want[first:first+n])
}

// assertPixelsEqual is assert_eq! for the pixels of a pixmap and a list of
// colors.
func assertPixelsEqual(t *testing.T, got okage.PixelSlice, want []okage.PremultipliedColorU8) {
	t.Helper()
	if got.Len() != len(want) {
		t.Fatalf("got %d pixels, want %d", got.Len(), len(want))
	}
	for i, w := range want {
		if g := got.At(i); g != w {
			t.Errorf("pixel %d: got %v, want %v", i, g, w)
		}
	}
}
