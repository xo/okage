// Copyright 2019 the SimpleCSS Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package simplecss

import "testing"

// TestSpecificity ports the tests of simplecss tests/specificity.rs. Each
// subtest has the name of the upstream test.
func TestSpecificity(t *testing.T) {
	tests := []struct {
		name string
		text string
		want [3]uint8
	}{
		{"spec_01", "*", [3]uint8{0, 0, 0}},
		{"spec_02", "li", [3]uint8{0, 0, 1}},
		{"spec_03", "ul li", [3]uint8{0, 0, 2}},
		{"spec_04", "ul ol + li", [3]uint8{0, 0, 3}},
		{"spec_05", "h1 + *[rel=up]", [3]uint8{0, 1, 1}},
		{"spec_06", "ul ol li.red", [3]uint8{0, 1, 3}},
		{"spec_07", "li.red.level", [3]uint8{0, 2, 1}},
		{"spec_08", "#x34y", [3]uint8{1, 0, 0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selectors, ok := ParseSelector(test.text)
			if !ok {
				t.Fatal("no selector")
			}
			if got := selectors.Specificity(); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}
