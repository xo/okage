// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"reflect"
	"testing"

	"github.com/xo/okage/typesetting/font"
)

// The first tests read the tree. The tests after them count the glyph items
// of the box, as the tests of the source count the glyphs of the SVG text,
// and compare boxes, as the tests of the source compare the SVG text.

func TestFontSwitches(t *testing.T) {
	for _, c := range [][2]string{
		{`{\rm th}`, `\mathrm{th}`},
		{`{\bf v}\cdot w`, `\mathbf{v}\cdot w`},
		{`{\it a}`, `\mathit{a}`},
		{`{\sf y}`, `\mathsf{y}`},
		{`{\tt z}`, `\mathtt{z}`},
		{`{\cal A}`, `\mathcal{A}`},
		{`{\sl a}`, `\mathit{a}`},
		{`{\rm a}b`, `\mathrm{a}b`},
	} {
		if !sameTree(t, c[0], c[1]) {
			t.Errorf("%q does not parse as %q", c[0], c[1])
		}
	}
	for _, c := range [][2]string{
		{`{\rm ab}`, `ab`},
		{`{\bf v}`, `{\rm v}`},
		{`{\rm a}b`, `{\rm ab}`},
		{`x^{\rm th}`, `x^{th}`},
	} {
		if sameTree(t, c[0], c[1]) {
			t.Errorf("%q parses as %q", c[0], c[1])
		}
	}
}

func TestNamedOperators(t *testing.T) {
	for name, op := range namedOps {
		a, ok := mustParse(t, `\`+name)[0].(Atom)
		if !ok || a.Class != Op || a.Variant != Upright {
			t.Errorf(`\%s = %#v, want an upright operator`, name, a)
			continue
		}
		want := LimitsSide
		if op.limits {
			want = LimitsAuto
		}
		if a.Limits != want {
			t.Errorf(`\%s limits = %d, want %d`, name, a.Limits, want)
		}
	}
	lim := as[Atom](t, mustParse(t, `\limsup`)[0])
	if b, ok := lim.Body.(List); !ok || len(b) != 3 || b[1] != (Space{Width: Length{3, Mu}}) {
		t.Errorf(`\limsup = %#v, want lim, a thin space and sup`, lim)
	}
}

func TestOperatorname(t *testing.T) {
	if !sameTree(t, `\operatorname{sin}`, `\sin`) {
		t.Error(`\operatorname{sin} should equal \sin`)
	}
	if a := as[Atom](t, mustParse(t, `\operatorname*{argmax}`)[0]); a.Limits != LimitsAuto || a.Text != "argmax" {
		t.Errorf(`\operatorname* = %#v, want limits in display`, a)
	}
	for _, tex := range []string{`\operatorname{a{b}c}`, `\operatorname{a\,b}`, `\operatorname{a\nope b}`} {
		mustParse(t, tex)
	}
	// okage: \operatorname x is \operatorname{x}, as in KaTeX (D95).
	if !sameTree(t, `\operatorname x`, `\operatorname{x}`) {
		t.Error(`\operatorname x is not \operatorname{x}`)
	}
	for _, tex := range []string{`\operatorname`, `\operatorname*`, `\operatorname{argmax`, `\operatorname*{argmax`} {
		if _, err := Parse(tex); err == nil {
			t.Errorf("Parse(%q) should fail", tex)
		}
	}
}

func TestModular(t *testing.T) {
	if a := as[Atom](t, mustParse(t, `\bmod`)[0]); a.Class != Bin || a.Text != "mod" {
		t.Errorf(`\bmod = %#v`, a)
	}
	for _, c := range []struct {
		tex   string
		items int
	}{
		{`\pmod n`, 6}, // quad ( mod thin n )
		{`\mod n`, 4},  // quad mod thin n
		{`\pod n`, 4},  // quad ( n )
	} {
		if a := as[Atom](t, mustParse(t, c.tex)[0]); len(as[List](t, a.Body)) != c.items {
			t.Errorf("%s = %#v, want %d items", c.tex, a, c.items)
		}
	}
	for _, tex := range []string{`\pmod`, `\mod`, `\pod`} {
		if _, err := Parse(tex); err == nil {
			t.Errorf("Parse(%q) should fail", tex)
		}
	}
}

func TestOpNameHelpers(t *testing.T) {
	for _, sw := range []string{"rm", "bf", "it", "sf", "tt", "cal", "sl"} {
		if !fontSwitch[sw].set {
			t.Errorf("fontSwitch[%q] is not set", sw)
		}
	}
	p := &parser{}
	if _, _, err := p.readOpName(nil); err == nil {
		t.Error("readOpName(nil) should fail")
	}
	if _, _, err := p.readOpName([]token{{kind: tChar, text: "x", r: 'x'}}); err == nil {
		t.Error("readOpName without a brace should fail")
	}
	if _, _, err := p.readOpName([]token{{kind: tLBrace}, {kind: tChar, text: "a", r: 'a'}}); err == nil {
		t.Error("readOpName unterminated should fail")
	}
	name, rest, err := p.readOpName([]token{
		{kind: tLBrace},
		{kind: tChar, text: "a", r: 'a'},
		{kind: tCtrl, text: ","},
		{kind: tCtrl, text: "nope"},
		{kind: tChar, text: "b", r: 'b'},
		{kind: tRBrace},
		{kind: tChar, text: "z", r: 'z'},
	})
	if err != nil || name != "a b" || len(rest) != 1 {
		t.Errorf("readOpName = (%q, %d, %v), want (%q, 1, nil)", name, len(rest), err, "a b")
	}
	if !reflect.DeepEqual(rest[0].r, 'z') {
		t.Errorf("rest = %v", rest)
	}
}

// glyphCount returns the number of glyph items of tex. The source counts the
// <path> elements of its SVG text, one for each glyph with ink.
func glyphCount(t *testing.T, face *font.Face, tex string) int {
	t.Helper()
	return len(itemsOf(onFace(t, face, tex, 32, Text), ItemGlyph))
}

// The font switches change the glyphs: \rm and \bf give other glyphs than
// math italic and roman.
func TestFontSwitchesChangeTheGlyphs(t *testing.T) {
	face := testFace(t)
	gids := func(tex string) []font.GID {
		t.Helper()
		var out []font.GID
		for _, it := range itemsOf(onFace(t, face, tex, 32, Text), ItemGlyph) {
			out = append(out, it.Glyph)
		}
		return out
	}
	for _, c := range [][2]string{{`{\rm ab}`, `ab`}, {`{\bf v}`, `{\rm v}`}} {
		if reflect.DeepEqual(gids(c[0]), gids(c[1])) {
			t.Errorf("%q has the glyphs of %q", c[0], c[1])
		}
	}
	for _, sw := range []string{`\rm`, `\bf`, `\it`, `\sf`, `\tt`, `\cal`, `\sl`} {
		for _, st := range []Style{Display, Text} {
			onFace(t, face, `{`+sw+` xyz}`, 32, st)
		}
	}
}

// The first half of TestNamedOperators of the source: each operator sets one
// upright glyph for each letter. The space of "lim sup" is a Space, not a
// glyph.
func TestNamedOperatorsSetOneGlyphForEachLetter(t *testing.T) {
	face := testFace(t)
	for op, want := range map[string]int{
		`\log`: 3, `\ln`: 2, `\lg`: 2, `\exp`: 3,
		`\sin`: 3, `\cos`: 3, `\tan`: 3, `\cot`: 3, `\sec`: 3, `\csc`: 3,
		`\sinh`: 4, `\cosh`: 4, `\tanh`: 4, `\coth`: 4,
		`\arcsin`: 6, `\arccos`: 6, `\arctan`: 6,
		`\min`: 3, `\max`: 3, `\inf`: 3, `\sup`: 3, `\lim`: 3,
		`\limsup`: 6, `\liminf`: 6,
		`\det`: 3, `\dim`: 3, `\ker`: 3, `\deg`: 3, `\gcd`: 3, `\hom`: 3,
		`\arg`: 3, `\Pr`: 2,
	} {
		if got := glyphCount(t, face, op); got != want {
			t.Errorf("%s sets %d glyphs, want %d", op, got, want)
		}
	}
	if equalBoxes(onFace(t, face, `\sin`, 32, Text), onFace(t, face, `sin`, 32, Text)) {
		t.Error(`\sin is not upright`)
	}
	// An operator with limits sets its subscript under it in display style,
	// and an operator without them sets it at the side in both styles.
	if equalBoxes(onFace(t, face, `\lim_{x} a`, 32, Display), onFace(t, face, `\lim_{x} a`, 32, Text)) {
		t.Error(`\lim does not take limits in display style`)
	}
	if !equalBoxes(onFace(t, face, `\log_{x} a`, 32, Display), onFace(t, face, `\log_{x} a`, 32, Text)) {
		t.Error(`\log takes limits`)
	}
}

// \operatorname sets its name upright, as the named operator of the same
// name, and the starred form takes limits in display style.
func TestOperatornameInTheBox(t *testing.T) {
	face := testFace(t)
	if got := glyphCount(t, face, `\operatorname{argmax}`); got != 6 {
		t.Errorf(`\operatorname{argmax} sets %d glyphs, want 6`, got)
	}
	if !equalBoxes(onFace(t, face, `\operatorname{sin}`, 32, Text), onFace(t, face, `\sin`, 32, Text)) {
		t.Error(`\operatorname{sin} does not lay out as \sin`)
	}
	if equalBoxes(onFace(t, face, `\operatorname{argmax}`, 32, Text), onFace(t, face, `argmax`, 32, Text)) {
		t.Error(`\operatorname is not upright`)
	}
	for _, tex := range []string{`\operatorname{a{b}c}`, `\operatorname{a\,b}`, `\operatorname{a\nope b}`} {
		onFace(t, face, tex, 32, Text)
	}
	if equalBoxes(onFace(t, face, `\operatorname*{argmax}_i f`, 32, Display), onFace(t, face, `\operatorname{argmax}_i f`, 32, Display)) {
		t.Error(`\operatorname* does not take limits in display style`)
	}
}

// The glyphs of \bmod, \pmod, \mod and \pod. The quad and the thin space are
// spaces, not glyphs.
func TestModularGlyphs(t *testing.T) {
	face := testFace(t)
	for _, c := range []struct {
		tex  string
		want int
	}{
		{`\bmod`, 3},   // m o d
		{`\pmod n`, 6}, // ( m o d n )
		{`\mod n`, 4},  // m o d n
		{`\pod n`, 3},  // ( n )
	} {
		if got := glyphCount(t, face, c.tex); got != c.want {
			t.Errorf("%s sets %d glyphs, want %d", c.tex, got, c.want)
		}
	}
	if equalBoxes(onFace(t, face, `\pmod n`, 32, Text), onFace(t, face, `\mod n`, 32, Text)) {
		t.Error(`\pmod has no parentheses`)
	}
	// \pmod{n} starts with a quad, one em, before the parenthesis.
	g := itemsOf(onFace(t, face, `\pmod n`, 32, Text), ItemGlyph)
	if g[0].X != 32 {
		t.Errorf(`the parenthesis of \pmod n is at %v, want 32 after the quad`, g[0].X)
	}
	onFace(t, face, `a \bmod b`, 32, Text)
	onFace(t, face, `x \pmod n`, 32, Text)
}

// Each long arrow is one glyph, and the double arrow is another glyph than
// the single one.
func TestLongArrowsAreOneGlyph(t *testing.T) {
	face := testFace(t)
	for _, tex := range []string{`\Longrightarrow`, `\Longleftarrow`, `\Longleftrightarrow`, `\longmapsto`} {
		if got := glyphCount(t, face, tex); got != 1 {
			t.Errorf("%s sets %d glyphs, want 1", tex, got)
		}
	}
	a := itemsOf(onFace(t, face, `\Longrightarrow`, 32, Text), ItemGlyph)
	b := itemsOf(onFace(t, face, `\longrightarrow`, 32, Text), ItemGlyph)
	if a[0].Glyph == b[0].Glyph {
		t.Error(`\Longrightarrow is the glyph of \longrightarrow`)
	}
	onFace(t, face, `A \Longrightarrow B`, 32, Text)
}
