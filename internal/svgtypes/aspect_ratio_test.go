// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import "testing"

// The tests of svgtypes src/aspect_ratio.rs.

func TestParseAspectRatio(t *testing.T) {
	tests := []struct {
		name string
		text string
		want AspectRatio
	}{
		{"parse_1", "none", AspectRatio{Defer: false, Align: AlignNone, Slice: false}},
		{"parse_2", "defer none", AspectRatio{Defer: true, Align: AlignNone, Slice: false}},
		{"parse_3", "xMinYMid", AspectRatio{Defer: false, Align: AlignXMinYMid, Slice: false}},
		{"parse_4", "xMinYMid slice", AspectRatio{Defer: false, Align: AlignXMinYMid, Slice: true}},
		{"parse_5", "xMinYMid meet", AspectRatio{Defer: false, Align: AlignXMinYMid, Slice: false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAspectRatio(tt.text)
			if err != nil || got != tt.want {
				t.Errorf("ParseAspectRatio(%q) = %v, %v, want %v", tt.text, got, err, tt.want)
			}
		})
	}
}
