// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import "testing"

// TestTransform is the test transform in tiny-skia-path
// path/src/transform.rs.
func TestTransform(t *testing.T) {
	eq := []struct {
		name      string
		got, want Transform
	}{
		{"identity", IdentityTransform(), TransformFromRow(1.0, 0.0, 0.0, 1.0, 0.0, 0.0)},
		{"scale", TransformFromScale(1.0, 2.0), TransformFromRow(1.0, 0.0, 0.0, 2.0, 0.0, 0.0)},
		{"skew", TransformFromSkew(2.0, 3.0), TransformFromRow(1.0, 3.0, 2.0, 1.0, 0.0, 0.0)},
		{"translate", TransformFromTranslate(5.0, 6.0), TransformFromRow(1.0, 0.0, 0.0, 1.0, 5.0, 6.0)},
	}
	for _, test := range eq {
		if test.got != test.want {
			t.Errorf("%s: %+v, want %+v", test.name, test.got, test.want)
		}
	}

	// The flags are in the order: is_identity, is_scale, is_skew,
	// is_translate, is_scale_translate, has_scale, has_skew, has_translate.
	flags := func(ts Transform) [8]bool {
		return [8]bool{
			ts.IsIdentity(),
			ts.IsScale(),
			ts.IsSkew(),
			ts.IsTranslate(),
			ts.IsScaleTranslate(),
			ts.HasScale(),
			ts.HasSkew(),
			ts.HasTranslate(),
		}
	}
	tests := []struct {
		name string
		ts   Transform
		want [8]bool
	}{
		{"identity", IdentityTransform(), [8]bool{true, false, false, false, false, false, false, false}},
		{"scale", TransformFromScale(2.0, 3.0), [8]bool{false, true, false, false, true, true, false, false}},
		{"skew", TransformFromSkew(2.0, 3.0), [8]bool{false, false, true, false, false, false, true, false}},
		{"translate", TransformFromTranslate(2.0, 3.0), [8]bool{false, false, false, true, true, false, false, true}},
		{"row", TransformFromRow(1.0, 2.0, 3.0, 4.0, 5.0, 6.0), [8]bool{false, false, false, false, false, true, true, true}},
	}
	for _, test := range tests {
		if got := flags(test.ts); got != test.want {
			t.Errorf("%s: flags = %v, want %v", test.name, got, test.want)
		}
	}

	if TransformFromScale(1.0, 1.0).HasScale() {
		t.Errorf("TransformFromScale(1, 1).HasScale() = true, want false")
	}
	if TransformFromSkew(0.0, 0.0).HasSkew() {
		t.Errorf("TransformFromSkew(0, 0).HasSkew() = true, want false")
	}
	if TransformFromTranslate(0.0, 0.0).HasTranslate() {
		t.Errorf("TransformFromTranslate(0, 0).HasTranslate() = true, want false")
	}
}

// TestConcat is the test concat in tiny-skia-path path/src/transform.rs.
func TestConcat(t *testing.T) {
	ts := TransformFromRow(1.2, 3.4, -5.6, -7.8, 1.2, 3.4)
	ts = ts.PreScale(2.0, -4.0)
	if want := TransformFromRow(2.4, 6.8, 22.4, 31.2, 1.2, 3.4); ts != want {
		t.Errorf("PreScale = %+v, want %+v", ts, want)
	}

	ts = TransformFromRow(1.2, 3.4, -5.6, -7.8, 1.2, 3.4)
	ts = ts.PostScale(2.0, -4.0)
	if want := TransformFromRow(2.4, -13.6, -11.2, 31.2, 2.4, -13.6); ts != want {
		t.Errorf("PostScale = %+v, want %+v", ts, want)
	}
}
