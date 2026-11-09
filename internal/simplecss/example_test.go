// Copyright 2016 the SimpleCSS Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package simplecss_test

import (
	"fmt"

	"github.com/xo/okage/internal/simplecss"
)

// This example ports the doc test of DeclarationTokenizer in simplecss
// src/lib.rs.
func ExampleDeclarationTokenizer() {
	t := simplecss.NewDeclarationTokenizer("background: url(\"img.png\"); color:red !important")
	for d := range t.All() {
		fmt.Printf("%s: %s important=%v\n", d.Name, d.Value, d.Important)
	}
	// Output:
	// background: url("img.png") important=false
	// color: red important=true
}

// This example ports the doc test of SelectorTokenizer in simplecss
// src/selector.rs.
func ExampleSelectorTokenizer() {
	t := simplecss.NewSelectorTokenizer("div > p:first-child")
	for tok, err := range t.All() {
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("%v(%q)\n", tok.Kind, tok.Value)
	}
	// Output:
	// TypeSelector("div")
	// ChildCombinator("")
	// TypeSelector("p")
	// PseudoClass("first-child")
}

// This example parses a style sheet. The rules come out sorted by
// specificity.
func ExampleParseStyleSheet() {
	style := simplecss.ParseStyleSheet("#a { fill:red } rect, .b { stroke:blue !important } @media print { g {} }")
	fmt.Println(style)
	// Output:
	// rect { stroke:blue !important; }
	// *[class~='b'] { stroke:blue !important; }
	// *[id='a'] { fill:red; }
}
