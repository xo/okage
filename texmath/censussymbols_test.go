// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"math"
	"strings"
	"testing"
)

// This file ports the tests of the symbol table of censussymbols_test.go,
// greekvariants_test.go, mathchar_test.go, alphabets_test.go and
// xarrow_test.go of the source. The tests at the end of the file lay out:
// they port the tests of censussymbols_test.go that read the SVG text or the
// metrics, and TestEverySymbolHasAGlyphInTheEmbeddedFont of
// symboltable_test.go, as TestEverySymbolHasAGlyphInTheFace.

func TestEverySymbolParses(t *testing.T) {
	for name, s := range symbols {
		if name == "sqrt" || name == "{" || name == "}" || name == " " {
			continue // a command that comes first, or a brace
		}
		l, err := Parse(`\` + name)
		if err != nil {
			t.Errorf(`\%s: %v`, name, err)
			continue
		}
		if a, ok := l[0].(Atom); !ok || a.Text != string(s.r) || a.Class != s.cls {
			t.Errorf(`\%s = %#v, want %q of class %d`, name, l, s.r, s.cls)
		}
	}
}

func TestTheDeclaringPackageDecidesTheClassNotUnicodeMath(t *testing.T) {
	for _, c := range []struct {
		name string
		want Class
	}{
		{"blacktriangleleft", Rel}, {"blacktriangleright", Rel}, {"Join", Rel}, {"Bbbk", Ord},
		{"lhd", Bin}, {"rhd", Bin}, {"unlhd", Bin}, {"unrhd", Bin}, {"bigcirc", Bin},
		{"blacklozenge", Ord}, {"lightning", Ord}, {"smallsmile", Rel}, {"smallfrown", Rel},
		{"intop", Op},
	} {
		if s, ok := symbols[c.name]; !ok || s.cls != c.want {
			t.Errorf(`\%s class = %d, want %d`, c.name, s.cls, c.want)
		}
	}
}

func TestRestrictionIsUpharpoonrightUnderAnotherName(t *testing.T) {
	a, b := symbols["restriction"], symbols["upharpoonright"]
	if a != b || a.r != 0x21BE {
		t.Errorf(`\restriction = %v, \upharpoonright = %v`, a, b)
	}
}

func TestTheBagsAreOpenAndCloseDelimiters(t *testing.T) {
	if got := symbols["Lbag"]; got.r != 0x27C5 || got.cls != Open {
		t.Errorf(`\Lbag = %v`, got)
	}
	if got := symbols["Rbag"]; got.r != 0x27C6 || got.cls != Close {
		t.Errorf(`\Rbag = %v`, got)
	}
}

func TestTheAccentsAreNotInTheSymbolTable(t *testing.T) {
	for _, name := range []string{"widecheck", "underrightarrow", "underleftarrow", "dddot"} {
		if s, ok := symbols[name]; ok {
			t.Errorf(`\%s is in the symbol table as %U`, name, s.r)
		}
	}
}

func TestTheAmsfontsBinaryAliasesShareTheGlyphAndNotTheClass(t *testing.T) {
	for _, c := range []struct{ bin, rel string }{
		{"lhd", "vartriangleleft"}, {"rhd", "vartriangleright"},
		{"unlhd", "trianglelefteq"}, {"unrhd", "trianglerighteq"},
	} {
		b, r := symbols[c.bin], symbols[c.rel]
		if b.r != r.r || b.cls != Bin || r.cls != Rel {
			t.Errorf(`\%s = %v, \%s = %v`, c.bin, b, c.rel, r)
		}
	}
	if sameTree(t, `a\lhd b`, `a\vartriangleleft b`) {
		t.Error(`\lhd and \vartriangleleft parse alike`)
	}
}

func TestTheSymbolsWithNoCodepointStayOut(t *testing.T) {
	// okage: \llcorner is in the table since D95, as U+231E, which KaTeX
	// gives it, and \nsubseteqq is \not\subseteqq, a macro.
	for _, name := range []string{"moo", "fatsemi", "lhook", "nsubseteqq", "llceil"} {
		if s, ok := symbols[name]; ok {
			t.Errorf(`\%s is in the table as %U`, name, s.r)
		}
	}
	for name, s := range symbols {
		if s.r == 0x2AC5 && strings.HasPrefix(name, "n") {
			t.Errorf(`\%s = U+2AC5, which is \subseteqq itself`, name)
		}
	}
}

func TestGreekVariantsMatchTheirCodepoints(t *testing.T) {
	for name, want := range map[string]rune{
		"epsilon": 0x03F5, "varepsilon": 0x03B5,
		"phi": 0x03D5, "varphi": 0x03C6,
		"theta": 0x03B8, "vartheta": 0x03D1,
		"rho": 0x03C1, "varrho": 0x03F1,
		"pi": 0x03C0, "varpi": 0x03D6,
		"kappa": 0x03BA, "varkappa": 0x03F0,
		"sigma": 0x03C3, "varsigma": 0x03C2,
	} {
		if s := symbols[name]; s.r != want {
			t.Errorf(`\%s = U+%04X, want U+%04X`, name, s.r, want)
		}
	}
}

func TestMathCharMapsMinusAndAsterisk(t *testing.T) {
	for in, want := range map[rune]rune{'-': 0x2212, '*': 0x2217, 'a': 'a', '+': '+'} {
		if got := mathChar(in); got != want {
			t.Errorf("mathChar(%q) = %U, want %U", in, got, want)
		}
	}
	if a, ok := mustParse(t, `*`)[0].(Atom); !ok || a.Text != "∗" || a.Class != Bin {
		t.Errorf(`* = %#v, want the asterisk operator`, a)
	}
}

func TestTheISOMathAlphabetsMapToTheirOwnBlocks(t *testing.T) {
	for _, c := range []struct {
		name            string
		upA, lowA, dig0 rune
	}{
		{"mathbfit", 0x1D468, 0x1D482, 0x1D7CE},
		{"mathbold", 0x1D468, 0x1D482, 0x1D7CE},
		{"boldsymbol", 0x1D468, 0x1D482, 0x1D7CE},
		{"mathsfit", 0x1D608, 0x1D622, 0x1D7E2},
		{"mathsans", 0x1D608, 0x1D622, 0x1D7E2},
		{"mathsfbfit", 0x1D63C, 0x1D656, 0x1D7EC},
		{"mathboldsans", 0x1D63C, 0x1D656, 0x1D7EC},
	} {
		a := alphabetFor(c.name)
		if !a.set {
			t.Fatalf("%s has no alphabet", c.name)
		}
		for _, k := range []struct{ in, want rune }{
			{'A', c.upA}, {'Z', c.upA + 25}, {'a', c.lowA}, {'z', c.lowA + 25}, {'0', c.dig0}, {'9', c.dig0 + 9},
		} {
			if got := mapVariant(k.in, a.variant); got != k.want {
				t.Errorf("%s: %q -> U+%04X, want U+%04X", c.name, k.in, got, k.want)
			}
		}
	}
}

func TestXArrowSpec(t *testing.T) {
	for name, want := range map[string]rune{
		"xrightarrow": '→', "xleftarrow": '←', "xleftrightarrow": '↔',
		"xhookrightarrow": '↪', "xhookleftarrow": '↩', "xmapsto": '↦',
	} {
		x, ok := mustParse(t, `\`+name+`[u]{o}`)[0].(XArrow)
		if !ok || x.Arrow != want || x.Under == nil || x.Over == nil {
			t.Errorf(`\%s = %#v, want %q with two labels`, name, x, want)
		}
	}
	if x, ok := mustParse(t, `\xrightarrow{f}`)[0].(XArrow); !ok || x.Under != nil {
		t.Errorf(`\xrightarrow{f} has an under label: %#v`, x.Under)
	}
}

// censusSame reports whether a and b have the same metrics and items.
func censusSame(a, b *Box) bool {
	if a.Width != b.Width || a.Height != b.Height || a.Depth != b.Depth || len(a.Items) != len(b.Items) {
		return false
	}
	for i, x := range a.Items {
		y := b.Items[i]
		if x.Kind != y.Kind || x.Glyph != y.Glyph || x.X != y.X || x.Y != y.Y || x.W != y.W || x.H != y.H {
			return false
		}
	}
	return true
}

// A symbol of the table whose character has no glyph in the font is set as
// an empty box, which looks like a lost character. Each symbol of the table
// has a glyph in STIX Two Math.
func TestEverySymbolHasAGlyphInTheFace(t *testing.T) {
	face := testFace(t)
	for name, s := range symbols {
		if _, ok := face.NominalGlyph(s.r); !ok {
			t.Errorf("\\%s: U+%04X has no glyph in STIX Two Math", name, s.r)
		}
	}
}

// The class is the space. A relation takes \thickmuskip on each side, and an
// ordinary symbol none.
//
// okage: the widths are computed again from the font: the advances of the
// three glyphs, and for the relation two thick spaces of 5mu at the size 32,
// with the italic corrections of a and b (phase 26 of D78).
func TestARelationIsSetWiderThanAnOrdinarySymbol(t *testing.T) {
	face := testFace(t)
	const size = 32.0
	wa, _, _ := glyphMetrics(t, face, mathItalic('a'), size)
	wb, _, _ := glyphMetrics(t, face, mathItalic('b'), size)
	wrel, _, _ := glyphMetrics(t, face, symbols["blacktriangleleft"].r, size)
	word, _, _ := glyphMetrics(t, face, symbols["blacktriangle"].r, size)
	thick := float64(5 * (size / 18))
	wa += italicOf(t, face, mathItalic('a'), size)
	wb += italicOf(t, face, mathItalic('b'), size)
	rel := mustLayout(t, `a\blacktriangleleft b`, size, Text).Width
	ord := mustLayout(t, `a\blacktriangle b`, size, Text).Width
	if want := wa + thick + wrel + thick + wb; !near(rel, want) {
		t.Errorf("relation %.4f, want %.4f", rel, want)
	}
	if want := wa + word + wb; !near(ord, want) {
		t.Errorf("ordinary %.4f, want %.4f", ord, want)
	}
	if rel <= ord {
		t.Errorf("the relation %.4f is not wider than the ordinary symbol %.4f", rel, ord)
	}
}

// Each of the 57 symbols of the census lays out in each style with a finite
// width above zero.
func TestEveryCensusSymbolRenders(t *testing.T) {
	for _, name := range []string{
		"blacktriangleleft", "Bbbk", "lrcorner", "blacktriangle", "triangledown",
		"smallsetminus", "blacktriangleright", "rightharpoonup", "blacktriangledown",
		"upharpoonright", "restriction", "longleftrightarrow", "Downarrow", "frown",
		"Uparrow", "looparrowright", "eqdef", "llparenthesis", "Lbag", "Rbag",
		"rightleftharpoons", "sslash", "nrightarrow", "backsim", "bigsqcap", "mho",
		"rightleftarrows", "ulcorner", "succsim", "nsubseteq", "rightarrowtail",
		"multimap", "geqq", "Cup", "nvDash", "mapsfrom", "nleq", "dotminus", "Join",
		"leftrightarrows", "nprec", "leqq", "nRightarrow", "Subset", "precneqq",
		"lll", "smile", "leftrightharpoons", "ncong", "lnsim", "lneq", "lessapprox",
		"lbrack", "gtreqqless", "gtrapprox", "fint", "approxeq",
	} {
		if _, ok := symbols[name]; !ok {
			t.Errorf(`\%s is not in the table`, name)
			continue
		}
		for _, st := range allStyles {
			w := mustLayout(t, `x`+`\`+name+` y`, 32, st).Width
			if math.IsNaN(w) || math.IsInf(w, 0) || w <= 0 {
				t.Errorf(`\%s in style %d: width = %v`, name, st, w)
			}
		}
	}
}

// \lhd is a binary operation and \vartriangleleft a relation, with the same
// glyph, so a\lhd b and a\vartriangleleft b do not lay out alike.
func TestTheAmsfontsBinaryAliasesTakeTheirOwnSpace(t *testing.T) {
	if censusSame(mustLayout(t, `a\lhd b`, 32, Display), mustLayout(t, `a\vartriangleleft b`, 32, Display)) {
		t.Error(`\lhd and \vartriangleleft lay out alike: the class is not reaching the space`)
	}
}

// The disputed classes of the table reach the layout: each pair below differs
// in its class only.
func TestTheDisputedClassesAreVisibleInTheRender(t *testing.T) {
	l := func(tex string) *Box { return mustLayout(t, tex, 32, Display) }
	if censusSame(l(`a\blacklozenge b`), l(`a\bigcirc b`)) {
		t.Error(`\blacklozenge (Ord) spaces like \bigcirc (Bin)`)
	}
	if censusSame(l(`a\lightning b`), l(`a\rightarrow b`)) {
		t.Error(`\lightning (Ord) spaces like \rightarrow (Rel)`)
	}
	if !censusSame(l(`\intop x`), l(`\int x`)) {
		t.Error(`\intop does not lay out as \int: fontmath.ltx:253 makes them the same glyph and class`)
	}
	if censusSame(l(`a\intop b`), l(`a\blacklozenge b`)) {
		t.Error(`\intop (Op) spaces like \blacklozenge (Ord)`)
	}
}

// \amsmathbb is the AMSb font, the alphabet of \mathbb, and not an
// approximation.
func TestAmsmathbbIsMathbbAndNotAnApproximation(t *testing.T) {
	l := func(tex string) *Box { return mustLayout(t, tex, 32, Display) }
	for _, body := range []string{"P", "R", "ZQ", "1"} {
		if !censusSame(l(`\amsmathbb{`+body+`}`), l(`\mathbb{`+body+`}`)) {
			t.Errorf(`\amsmathbb{%s} does not lay out as \mathbb{%s}`, body, body)
		}
	}
	if censusSame(l(`\amsmathbb{P}`), l(`P`)) {
		t.Error(`\amsmathbb{P} lays out as a plain P: the alphabet is not applied`)
	}
}
