// Copyright 2019 the SimpleCSS Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package simplecss

import "testing"

// TestStyleSheet ports the tests of simplecss tests/stylesheet.rs. Each
// subtest has the name of the upstream test.
func TestStyleSheet(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"style_01", "", ""},
		{"style_02", "a {}", ""},
		{"style_03", "a { color:red }", "a { color:red; }"},
		{"style_04", "/**/", ""},
		{"style_05", "a { color:red } /**/", "a { color:red; }"},
		{"style_06", "a, b { color:red }", "a { color:red; }\nb { color:red; }"},
		{"style_07", "a, { color:red }", "a { color:red; }"},
		{"style_08", "a,, { color:red }", "a { color:red; }"},
		{"style_09", "a,,b { color:red }", "a { color:red; }\nb { color:red; }"},
		{"style_10", ",a { color:red }", "a { color:red; }"},
		{"style_11", "@import \"subs.css\";\na { color:red }", "a { color:red; }"},
		{"style_12", "@media screen {\n    p:before { content: 'Hello'; }\n}\na { color:red }", "a { color:red; }"},
		{"style_13", "a > { color:red }", ""},
		{"style_14", "p { color:green; color }", "p { color:green; }"},
		{"style_15", "p { color; color:green }", ""}, // TODO: should be 'p { color:green; }'
		{"style_16", "p { color:green; color: }", "p { color:green; }"},
		{"style_17", "p { color:green; color:; color:red; }", "p { color:green; }"},
		{"style_18", "p { color:green; color{;color:maroon} }", "p { color:green; }"},
		{"style_19", "p { color{;color:maroon} color:green; }", ""}, // TODO: should be 'p { color:green; }'
		{"style_20", "h1 { color: green }\n        h2 & h3 { color: red }\n        h4 { color: black }\n    ", "h1 { color:green; }\nh4 { color:black; }"},
		{"style_21", ":le>*", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			style := ParseStyleSheet(test.text)
			if got := style.String(); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

// TestStyleSheetParseMore checks that ParseMore adds to the rules and sorts
// all of them by specificity, with a stable sort.
func TestStyleSheetParseMore(t *testing.T) {
	style := ParseStyleSheet("#a { fill:red } b { fill:green }")
	style.ParseMore("c { fill:blue } .d { fill:black }")
	want := "b { fill:green; }\nc { fill:blue; }\n*[class~='d'] { fill:black; }\n*[id='a'] { fill:red; }"
	if got := style.String(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
