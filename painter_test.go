// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import "testing"

// The tests of this file are the module tests in tiny-skia src/painter.rs.

const maxDim = drawTilerMaxDimensions

// checkTiles reads every tile of the tiler and compares them with want.
func checkTiles(t *testing.T, tiler *drawTiler, want []screenIntRect) {
	t.Helper()
	for i, w := range want {
		got, ok := tiler.next()
		if !ok {
			t.Fatalf("tile %d: got none, want %v", i, w)
		}
		if got != w {
			t.Errorf("tile %d: got %v, want %v", i, got, w)
		}
	}
	if got, ok := tiler.next(); ok {
		t.Errorf("got %v after the last tile, want none", got)
	}
}

// rectXYWH is ScreenIntRect::from_xywh(...).unwrap() of the tests.
func rectXYWH(t *testing.T, x, y, w, h uint32) screenIntRect {
	t.Helper()
	r, ok := screenIntRectFromXYWH(x, y, w, h)
	if !ok {
		t.Fatalf("screenIntRectFromXYWH(%d, %d, %d, %d) failed", x, y, w, h)
	}
	return r
}

// TestDrawTilerSkip is the test skip in tiny-skia src/painter.rs.
func TestDrawTilerSkip(t *testing.T) {
	if _, ok := newDrawTiler(100, 500); ok {
		t.Error("newDrawTiler(100, 500) is ok")
	}
}

// TestDrawTilerHorizontal is the test horizontal in tiny-skia
// src/painter.rs.
func TestDrawTilerHorizontal(t *testing.T) {
	tiler, ok := newDrawTiler(10000, 500)
	if !ok {
		t.Fatal("newDrawTiler failed")
	}
	checkTiles(t, tiler, []screenIntRect{
		rectXYWH(t, 0, 0, maxDim, 500),
		rectXYWH(t, maxDim, 0, 10000-maxDim, 500),
	})
}

// TestDrawTilerVertical is the test vertical in tiny-skia src/painter.rs.
func TestDrawTilerVertical(t *testing.T) {
	tiler, ok := newDrawTiler(500, 10000)
	if !ok {
		t.Fatal("newDrawTiler failed")
	}
	checkTiles(t, tiler, []screenIntRect{
		rectXYWH(t, 0, 0, 500, maxDim),
		rectXYWH(t, 0, maxDim, 500, 10000-maxDim),
	})
}

// TestDrawTilerRect is the test rect in tiny-skia src/painter.rs.
func TestDrawTilerRect(t *testing.T) {
	tiler, ok := newDrawTiler(10000, 10000)
	if !ok {
		t.Fatal("newDrawTiler failed")
	}
	checkTiles(t, tiler, []screenIntRect{
		// Row 1
		rectXYWH(t, 0, 0, maxDim, maxDim),
		rectXYWH(t, maxDim, 0, 10000-maxDim, maxDim),
		// Row 2
		rectXYWH(t, 0, maxDim, maxDim, 10000-maxDim),
		rectXYWH(t, maxDim, maxDim, 10000-maxDim, 10000-maxDim),
	})
}
