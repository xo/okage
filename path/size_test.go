// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import "testing"

// TestIntSizeTests is the test int_size_tests in tiny-skia-path
// path/src/size.rs.
func TestIntSizeTests(t *testing.T) {
	if _, ok := IntSizeFromWH(0, 0); ok {
		t.Errorf("IntSizeFromWH(0, 0) is ok, want false")
	}
	if _, ok := IntSizeFromWH(1, 0); ok {
		t.Errorf("IntSizeFromWH(1, 0) is ok, want false")
	}
	if _, ok := IntSizeFromWH(0, 1); ok {
		t.Errorf("IntSizeFromWH(0, 1) is ok, want false")
	}

	size, ok := IntSizeFromWH(3, 4)
	if !ok {
		t.Fatalf("IntSizeFromWH(3, 4) is not ok")
	}
	want, ok := IntRectFromXYWH(1, 2, 3, 4)
	if !ok {
		t.Fatalf("IntRectFromXYWH(1, 2, 3, 4) is not ok")
	}
	if got := size.ToIntRect(1, 2); got != want {
		t.Errorf("ToIntRect(1, 2) = %+v, want %+v", got, want)
	}
}
