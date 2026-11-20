package texmath

import (
	"testing"
)

// This file tests \overbrace and \underbrace, and the assemblies of the MATH
// table, which phase 26 of D78 adds in the horizontal direction.

// TestBraceGrows checks that a brace grows to the width of its base with
// the variants and the assembly, and that its scripts are limits.
func TestBraceGrows(t *testing.T) {
	const size = 10
	face := testFace(t)
	over := glyphOf(t, face, '⏞')
	asm, ok := face.MathGlyphAssembly(over, false)
	if !ok {
		t.Fatal("STIX Two Math has no horizontal assembly for U+23DE")
	}
	for _, tex := range []string{`a`, `a+b`, `a+b+c+d+e+f+g+h+i+j+k+l`} {
		base := mustLayout(t, tex, size, Text)
		b := mustLayout(t, `\overbrace{`+tex+`}`, size, Text)
		// The items of the base come first, and those of the brace after.
		brace := b.Items[len(base.Items):]
		// The parts span the base, within the bearings of the end parts.
		left, right := brace[0].X, 0.0
		for _, it := range brace {
			w, _, _ := gidMetrics(face, it.Glyph, size)
			right = max(right, it.X+w)
		}
		if right-left < base.Width-0.1 && len(brace) > 1 {
			t.Errorf(`\overbrace{%s}: the brace spans %v, the base %v`, tex, right-left, base.Width)
		}
		if b.Height <= base.Height {
			t.Errorf(`\overbrace{%s} is %v high, the base %v`, tex, b.Height, base.Height)
		}
	}
	// A wide base takes the assembly, from the left part to the right part.
	b := mustLayout(t, `\overbrace{a+b+c+d+e+f+g+h+i+j+k+l}`, size, Text)
	g := itemsOf(b, ItemGlyph)
	first, last := asm.Parts[0].Glyph, asm.Parts[len(asm.Parts)-1].Glyph
	var parts []Item
	for _, it := range g {
		if it.Glyph == first || it.Glyph == last || it.Glyph == asm.Parts[1].Glyph || it.Glyph == asm.Parts[2].Glyph {
			parts = append(parts, it)
		}
	}
	if len(parts) < len(asm.Parts) || parts[0].Glyph != first || parts[len(parts)-1].Glyph != last || parts[0].X > parts[len(parts)-1].X {
		t.Errorf("the parts of the wide brace are %+v", parts)
	}
	// The label of \overbrace is over the brace, and that of \underbrace
	// under it, centered as limits in each style.
	for _, st := range []Style{Display, Text} {
		ob := mustLayout(t, `\overbrace{a+b}^{n}`, size, st)
		n := glyphItem(t, ob, glyphOf(t, face, mathItalic('n')))
		if !(n.Y < 0) || -n.Y < mustLayout(t, `\overbrace{a+b}`, size, st).Height {
			t.Errorf("the label of \\overbrace is at %v", n.Y)
		}
		ub := mustLayout(t, `\underbrace{a+b}_{n}`, size, st)
		n = glyphItem(t, ub, glyphOf(t, face, mathItalic('n')))
		if n.Y < mustLayout(t, `\underbrace{a+b}`, size, st).Depth {
			t.Errorf("the label of \\underbrace is at %v", n.Y)
		}
	}
}

// TestAssemblyGoesBottomUp checks that the first part of a vertical assembly,
// which the OpenType specification puts at the bottom, is at the bottom.
func TestAssemblyGoesBottomUp(t *testing.T) {
	face := testFace(t)
	asm, ok := face.MathGlyphAssembly(glyphOf(t, face, '('), true)
	if !ok {
		t.Fatal("STIX Two Math has no assembly for (")
	}
	b := mustLayout(t, `\left(\begin{matrix}a\\b\\c\\d\\e\\f\\g\end{matrix}\right)`, 10, Text)
	bottom, top := asm.Parts[0].Glyph, asm.Parts[len(asm.Parts)-1].Glyph
	var yb, yt float64
	for _, it := range b.Items {
		switch it.Glyph {
		case bottom:
			yb = it.Y
		case top:
			yt = it.Y
		}
	}
	if !(yb > yt) {
		t.Errorf("the bottom part is at %v and the top part at %v, with y down", yb, yt)
	}
}
