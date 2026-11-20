// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"fmt"
	"testing"
)

// This file ports the tests of alphabets_test.go of the source that read the
// face and the layout. TestTheISOMathAlphabetsMapToTheirOwnBlocks, which
// reads the parser, is in censussymbols_test.go.
//
// The three alphabets of ISO 80000-2, which isomath selects with \vectorsym,
// \matrixsym and \tensorsym, are bold italic, sans-serif italic and
// sans-serif bold italic. Unicode has no italic digits, so each one takes
// the digits of its upright block.

// Each letter and digit of the three blocks is in the face. A code point
// that the face does not have gives an empty box, and the formula then
// misses a letter without an error.
func TestTheISOMathBlocksAreCoveredByTheFace(t *testing.T) {
	face := testFace(t)
	for _, b := range []struct {
		name string
		base rune
		n    int
	}{
		{"bold italic capitals", 0x1D468, 26},
		{"bold italic smalls", 0x1D482, 26},
		{"sans-serif italic capitals", 0x1D608, 26},
		{"sans-serif italic smalls", 0x1D622, 26},
		{"sans-serif bold italic capitals", 0x1D63C, 26},
		{"sans-serif bold italic smalls", 0x1D656, 26},
		{"bold digits", 0x1D7CE, 10},
		{"sans-serif digits", 0x1D7E2, 10},
		{"sans-serif bold digits", 0x1D7EC, 10},
	} {
		for i := range b.n {
			if _, ok := face.NominalGlyph(b.base + rune(i)); !ok {
				t.Errorf("%s: U+%04X is not in the face", b.name, b.base+rune(i))
			}
		}
	}
}

// The three alphabets give different glyphs. A table of code points can be
// right while the layout sets two of them with the same glyphs. The source
// compares the SVG text. Here the glyphs of the items are compared, and the
// text of each item is the character of its alphabet.
func TestTheISOMathAlphabetsRenderDifferently(t *testing.T) {
	seen := map[string]string{}
	for _, c := range []struct {
		name string
		a    rune // the code point of A
	}{
		{"mathbfit", 0x1D468},
		{"mathsfit", 0x1D608},
		{"mathsfbfit", 0x1D63C},
	} {
		b := mustLayout(t, `\`+c.name+`{Axz}`, 10, Text)
		glyphs := itemsOf(b, ItemGlyph)
		if len(glyphs) != 3 {
			t.Fatalf(`\%s{Axz}: %d glyphs, want 3`, c.name, len(glyphs))
		}
		if glyphs[0].Text != string(c.a) {
			t.Errorf(`\%s{Axz}: the first glyph is for %q, want %q`, c.name, glyphs[0].Text, string(c.a))
		}
		key := fmt.Sprint(glyphs[0].Glyph, glyphs[1].Glyph, glyphs[2].Glyph)
		if other, dup := seen[key]; dup {
			t.Errorf(`\%s sets the glyphs of \%s`, c.name, other)
		}
		seen[key] = c.name
	}
}
