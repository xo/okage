// Copyright 2019 the SimpleCSS Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package simplecss

import (
	"slices"
	"testing"
)

// TestDeclarationTokenizer ports the tests of simplecss
// tests/declaration_tokenizer.rs, which the macro tokenize! makes. Each
// subtest has the name of the upstream test.
func TestDeclarationTokenizer(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []Declaration
	}{
		{"tokenize_01", "", []Declaration{}},
		{"tokenize_02", " ", []Declaration{}},
		{"tokenize_03", "/**/", []Declaration{}},
		{"tokenize_04", "color:red", []Declaration{{Name: "color", Value: "red", Important: false}}},
		{"tokenize_05", "color:red;", []Declaration{{Name: "color", Value: "red", Important: false}}},
		{"tokenize_06", "color:red ", []Declaration{{Name: "color", Value: "red", Important: false}}},
		{"tokenize_07", " color: red; ", []Declaration{{Name: "color", Value: "red", Important: false}}},
		{"tokenize_08", "  color  :  red  ; ", []Declaration{{Name: "color", Value: "red", Important: false}}},
		{"tokenize_09", "  color:red;;;;color:red; ", []Declaration{{Name: "color", Value: "red", Important: false}, {Name: "color", Value: "red", Important: false}}},
		{"tokenize_10", "background: url(\"img.png\");", []Declaration{{Name: "background", Value: "url(\"img.png\")", Important: false}}},
		{"tokenize_11", "background: url(\"{}\");", []Declaration{{Name: "background", Value: "url(\"{}\")", Important: false}}},
		{"tokenize_12", "color: red ! important", []Declaration{{Name: "color", Value: "red", Important: true}}},
		{"tokenize_13", "color: red !important", []Declaration{{Name: "color", Value: "red", Important: true}}},
		{"tokenize_14", "color: red!important", []Declaration{{Name: "color", Value: "red", Important: true}}},
		{"tokenize_15", "color: red !/**/important", []Declaration{{Name: "color", Value: "red", Important: true}}},
		{"tokenize_16", "border: 1em solid blue", []Declaration{{Name: "border", Value: "1em solid blue", Important: false}}},
		{"tokenize_17", "background: navy url(support/diamond.png) -2em -2em no-repeat", []Declaration{{Name: "background", Value: "navy url(support/diamond.png) -2em -2em no-repeat", Important: false}}},
		{"tokenize_18", "/**/color:red", []Declaration{{Name: "color", Value: "red", Important: false}}},
		{"tokenize_19", "/* *\\/*/color: red;", []Declaration{{Name: "color", Value: "red", Important: false}}},
		{"tokenize_20", "/**/color/**/:/**/red/**/;/**/", []Declaration{{Name: "color", Value: "red", Important: false}}},
		{"tokenize_21", "\ncolor\n:\nred\n;\n", []Declaration{{Name: "color", Value: "red", Important: false}}},
		{"tokenize_22", "{color:red}", []Declaration{}},
		{"tokenize_23", "(color:red)", []Declaration{}},
		{"tokenize_24", "[color:red]", []Declaration{}},
		{"tokenize_25", "color:", []Declaration{}},
		{"tokenize_26", "value:\"text\"", []Declaration{{Name: "value", Value: "\"text\"", Important: false}}},
		{"tokenize_27", "value:'text'", []Declaration{{Name: "value", Value: "'text'", Important: false}}},
		{"tokenize_28", "color:#fff", []Declaration{{Name: "color", Value: "#fff", Important: false}}},
		{"tokenize_29", "color:0.5", []Declaration{{Name: "color", Value: "0.5", Important: false}}},
		{"tokenize_30", "color:.5", []Declaration{{Name: "color", Value: ".5", Important: false}}},
		{"tokenize_31", "color:#FFF", []Declaration{{Name: "color", Value: "#FFF", Important: false}}},
		{"tokenize_32", "content: counter(chapno, upper-roman) \". \"", []Declaration{{Name: "content", Value: "counter(chapno, upper-roman) \". \"", Important: false}}},
		{"tokenize_33", "font-family:'Noto Serif','DejaVu Serif',serif", []Declaration{{Name: "font-family", Value: "'Noto Serif','DejaVu Serif',serif", Important: false}}},
		{"tokenize_34", "*zoom:1;", []Declaration{{Name: "zoom", Value: "1", Important: false}}}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tok := NewDeclarationTokenizer(test.text)
			for i, want := range test.want {
				got, ok := tok.Next()
				if !ok {
					t.Fatalf("token %d: got none, want %+v", i, want)
				}
				if got != want {
					t.Fatalf("token %d: got %+v, want %+v", i, got, want)
				}
			}
			if got, ok := tok.Next(); ok {
				t.Fatalf("got %+v, want none", got)
			}
		})
	}
}

// TestDeclarationTokenizerAll checks that All gives the same declarations as
// Next.
func TestDeclarationTokenizerAll(t *testing.T) {
	got := slices.Collect(NewDeclarationTokenizer("color:red; fill:blue !important; [bad]").All())
	want := []Declaration{
		{Name: "color", Value: "red"},
		{Name: "fill", Value: "blue", Important: true},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
