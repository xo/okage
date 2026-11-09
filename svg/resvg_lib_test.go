// Copyright 2017 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"math"
	"testing"
)

// This file ports the tests of resvg src/lib.rs.

// TestMaxFilterBBoxIsClamped is max_filter_bbox_is_clamped in resvg
// src/lib.rs.
func TestMaxFilterBBoxIsClamped(t *testing.T) {
	bbox := maxFilterBBox(math.MaxUint32, math.MaxUint32)
	if got, want := bbox.Left(), int32(math.MinInt32/2); got != want {
		t.Errorf("left = %d, want %d", got, want)
	}
	if got, want := bbox.Top(), int32(math.MinInt32/2); got != want {
		t.Errorf("top = %d, want %d", got, want)
	}
	if got, want := bbox.Right(), int32(math.MaxInt32/2); got != want {
		t.Errorf("right = %d, want %d", got, want)
	}
	if got, want := bbox.Bottom(), int32(math.MaxInt32/2); got != want {
		t.Errorf("bottom = %d, want %d", got, want)
	}
}
