// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"reflect"
	"testing"

	"github.com/xo/okage/typesetting/font"
)

// The first tests read the tree. The tests after them measure which branch
// the layout chooses, and the width of a space in math units. The source has
// a level of scripts and a display flag in its style, and the port has the
// four styles of TeX, so the tests of the level read the styles.

func TestMathchoiceKeepsTheFourBranches(t *testing.T) {
	c, ok := mustParse(t, `\mathchoice{a}{ab}{abc}{abcd}`)[0].(Choice)
	if !ok {
		t.Fatalf("not a Choice")
	}
	for i, b := range []List{c.Display, c.Text, c.Script, c.ScriptScript} {
		if len(b) != i+1 {
			t.Errorf("branch %d has %d atoms, want %d", i, len(b), i+1)
		}
	}
}

func TestMathchoiceConsumesAllFourGroups(t *testing.T) {
	l := mustParse(t, `\mathchoice{}{a}{b}{c}z`)
	if len(l) != 2 {
		t.Fatalf("got %d items, want the Choice and z", len(l))
	}
	if c := as[Choice](t, l[0]); len(c.Display) != 0 {
		t.Errorf("the display branch is %#v, want it empty", c.Display)
	}
	if !reflect.DeepEqual(l[1], mustParse(t, `z`)[0]) {
		t.Error("the unchosen branches leaked into the formula")
	}
}

func TestMathchoiceWithTooFewGroupsIsAnError(t *testing.T) {
	for _, tex := range []string{
		`\mathchoice{a}{b}{c}`,
		`\mathchoice{a}{b}`,
		`\mathchoice{a}`,
		`\mathchoice`,
		`\mathchoice{a}{b}{c}x`,
		`\mathchoice{a}{b}{c}{d`,
	} {
		if _, err := Parse(tex); err == nil {
			t.Errorf("%s did not fail", tex)
		}
	}
}

// okage: a fault in any branch is an error, because the parser does not know
// the style. The source reports a fault of the chosen branch only.
func TestMathchoiceReportsAFaultInAnyBranch(t *testing.T) {
	for _, tex := range []string{
		`\mathchoice{\nosuchmaththing}{b}{c}{d}`,
		`\mathchoice{a}{b}{c}{\nosuchmaththing}`,
	} {
		if _, err := Parse(tex); err == nil {
			t.Errorf("%s did not fail", tex)
		}
	}
}

func TestTheCorpusMathchoiceFormsRender(t *testing.T) {
	for _, tex := range []string{
		`\mathchoice{\mkern-3mu}{}{}{}`,
		`\mathchoice{}{}{\mskip-0.5mu}{\mskip-1mu}`,
		`{\mathrm{1}\mkern-4mu{\mathchoice{}{}{\mskip-0.5mu}{\mskip-1mu}}}`,
		`\mathchoice{\sum}{\sum}{\textstyle\sum}{\textstyle\sum}`,
		`x^{\mathchoice{a}{b}{c}{d}}`,
		`\frac{\mathchoice{a}{b}{c}{d}}{2}`,
	} {
		mustParse(t, tex)
	}
}

func TestMuKernMeasuresInMathUnits(t *testing.T) {
	for _, c := range []struct {
		tex  string
		want float64
	}{
		{`a\mkern18mu b`, 18},
		{`a\mkern3mu b`, 3},
		{`a\mkern-3mu b`, -3},
		{`a\mskip3mu b`, 3},
		{`a\mskip3mu plus 1mu b`, 3},
		{`a\mskip3mu plus 1mu minus 2mu b`, 3},
		{`a\mskip3mu plus1fil b`, 3},
	} {
		l := mustParse(t, c.tex)
		if len(l) != 3 {
			t.Errorf("%s: %d items, want a, the space and b", c.tex, len(l))
			continue
		}
		if s, ok := l[1].(Space); !ok || s.Width != (Length{Value: c.want, Unit: Mu}) {
			t.Errorf("%s: %#v, want %gmu", c.tex, l[1], c.want)
		}
	}
	if !sameTree(t, `a\mkern3mu b`, `a\,b`) {
		t.Error(`\mkern3mu is not \,`)
	}
}

func TestMuKernWithoutAReadableDimensionIsAnError(t *testing.T) {
	for _, tex := range []string{`a\mkern b`, `a\mkern`, `a\mkern3pt b`, `a\mkern3 b`, `a\mkern.mu b`} {
		if _, err := Parse(tex); err == nil {
			t.Errorf("%s did not fail", tex)
		}
	}
}

func TestGlueUnitLengthMatchesLongestFirst(t *testing.T) {
	for _, c := range []struct {
		text string
		want int
	}{
		{"filll", 5}, {"fill", 4}, {"fil", 3},
		{"mu", 2}, {"pt", 2}, {"sp", 2},
		{"fillx", 4},
		{"b", 0}, {"", 0}, {"xyz", 0},
	} {
		if got := glueUnitLen(tokenize(c.text), 0); got != c.want {
			t.Errorf("glueUnitLen(%q) = %d, want %d", c.text, got, c.want)
		}
	}
}

// mcWidth returns the width of tex at the size 32 in the style st.
func mcWidth(t *testing.T, face *font.Face, tex string, st Style) float64 {
	t.Helper()
	return onFace(t, face, tex, 32, st).Width
}

// Display and text style choose the branches 0 and 1, and the two differ.
// Script and scriptscript style choose the branches 2 and 3.
func TestMathchoicePicksDisplayAndTextByTheFlag(t *testing.T) {
	face := testFace(t)
	const mc = `\mathchoice{a}{ab}{abc}{abcd}`
	for _, c := range []struct {
		st   Style
		want string
	}{
		{Display, `a`}, {Text, `ab`}, {Script, `abc`}, {ScriptScript, `abcd`},
	} {
		if got, want := mcWidth(t, face, mc, c.st), mcWidth(t, face, c.want, c.st); got != want {
			t.Errorf("style %d chose a body %v wide, want %v, the branch %s", c.st, got, want, c.want)
		}
	}
	if mcWidth(t, face, mc, Display) == mcWidth(t, face, mc, Text) {
		t.Error("display and text style chose the same branch")
	}
}

// TestMathchoiceBranchFollowsTheStyle is
// TestMathchoiceBranchIndexFollowsTheScriptLevel of the source. The source
// chooses by its level of scripts and its display flag, and Choice.branch
// chooses by the style.
func TestMathchoiceBranchFollowsTheStyle(t *testing.T) {
	c := as[Choice](t, mustParse(t, `\mathchoice{a}{ab}{abc}{abcd}`)[0])
	for st, want := range map[Style]int{Display: 1, Text: 2, Script: 3, ScriptScript: 4} {
		if got := len(c.branch(st)); got != want {
			t.Errorf("style %d: the branch has %d atoms, want %d", st, got, want)
		}
	}
}

// TestTheScriptConstructorsLowerTheStyle is
// TestTheScriptConstructorsRaiseTheLevel of the source. A script is one style
// down, and a script of a script, or of scriptscript style, is scriptscript.
// A cell of a grid drops display style. A fraction in display style sets its
// parts in text style, and one in text style in script style (Appendix G,
// rule 15b).
func TestTheScriptConstructorsLowerTheStyle(t *testing.T) {
	for _, c := range []struct {
		name      string
		got, want Style
	}{
		{"a script of display style", scriptOf(Display), Script},
		{"a script of text style", scriptOf(Text), Script},
		{"a script of a script", scriptOf(Script), ScriptScript},
		{"a script of scriptscript style", scriptOf(ScriptScript), ScriptScript},
		{"a cell in display style", cellStyle(Display), Text},
		{"a cell in script style", cellStyle(Script), Script},
		{"the parts of a fraction in display style", fracInner(Display), Text},
		{"the parts of a fraction in text style", fracInner(Text), Script},
		{"the parts of a fraction in script style", fracInner(Script), ScriptScript},
	} {
		if c.got != c.want {
			t.Errorf("%s: style %d, want %d", c.name, c.got, c.want)
		}
	}
}

// The branch changes inside a script: display style takes the empty branch,
// which makes no atom, and a script takes the branch with the x.
func TestMathchoiceChoosesADifferentBranchInsideAScript(t *testing.T) {
	face := testFace(t)
	const mc = `\mathchoice{}{}{x}{}`
	if !equalBoxes(onFace(t, face, `a`+mc, 32, Display), onFace(t, face, `a`, 32, Display)) {
		t.Error("display style did not choose the empty branch 0")
	}
	if equalBoxes(onFace(t, face, `a^{`+mc+`}`, 32, Display), onFace(t, face, `a^{}`, 32, Display)) {
		t.Error("a script did not choose the branch 2, which holds the x")
	}
}

// The unchosen branches do not leak, and the z lands where it lands without
// the \mathchoice.
func TestMathchoiceLaysOutOneBranch(t *testing.T) {
	face := testFace(t)
	if !equalBoxes(onFace(t, face, `\mathchoice{}{a}{b}{c}z`, 32, Display), onFace(t, face, `z`, 32, Display)) {
		t.Error("the unchosen branches leaked into the box")
	}
}

// The forms of the corpus lay out in each style.
func TestTheCorpusMathchoiceFormsLayOut(t *testing.T) {
	face := testFace(t)
	for _, tex := range []string{
		`\mathchoice{\mkern-3mu}{}{}{}`,
		`\mathchoice{}{}{\mskip-0.5mu}{\mskip-1mu}`,
		`{\mathrm{1}\mkern-4mu{\mathchoice{}{}{\mskip-0.5mu}{\mskip-1mu}}}`,
		`\mathchoice{\sum}{\sum}{\textstyle\sum}{\textstyle\sum}`,
		`x^{\mathchoice{a}{b}{c}{d}}`,
		`\frac{\mathchoice{a}{b}{c}{d}}{2}`,
	} {
		for _, st := range allStyles {
			onFace(t, face, tex, 32, st)
		}
	}
	// The \intkern@ of amsmath in display style is a kern of -3mu.
	if got, want := mcWidth(t, face, `\mathchoice{\mkern-3mu}{}{}{}`, Display), float64(-3*32.0)/18; got != want {
		t.Errorf(`\mathchoice{\mkern-3mu}{}{}{} in display style is %v wide, want %v`, got, want)
	}
}

// \mkern and \mskip are in math units, 18mu to the em. At the size 32 in text
// style, 18mu is 32. The stretch of \mskip is dropped.
func TestMuKernWidensByMathUnits(t *testing.T) {
	face := testFace(t)
	base := mcWidth(t, face, `ab`, Text)
	if got := mcWidth(t, face, `a\mkern18mu b`, Text) - base; !near(got, 32) {
		t.Errorf(`\mkern18mu added %v, want 32, one em`, got)
	}
	if got, want := mcWidth(t, face, `a\mkern3mu b`, Text), mcWidth(t, face, `a\,b`, Text); got != want {
		t.Errorf(`\mkern3mu gives %v, \, gives %v`, got, want)
	}
	if got := mcWidth(t, face, `a\mkern-3mu b`, Text); got >= base {
		t.Errorf(`\mkern-3mu gave %v, not less than %v`, got, base)
	}
	plain := mcWidth(t, face, `a\mskip3mu b`, Text)
	for _, tex := range []string{`a\mskip3mu plus 1mu b`, `a\mskip3mu plus 1mu minus 2mu b`, `a\mskip3mu plus1fil b`} {
		if got := mcWidth(t, face, tex, Text); got != plain {
			t.Errorf("%s gives %v, want %v: the stretch is dropped", tex, got, plain)
		}
	}
	// In script style a mu is 1/18 of the script size.
	if got := mcWidth(t, face, `a\mkern18mu b`, Script) - mcWidth(t, face, `ab`, Script); !near(got, scriptSize(face, 32)) {
		t.Errorf(`\mkern18mu in script style added %v, want the script size %v`, got, scriptSize(face, 32))
	}
}

// okage: a kern is not an atom, as in TeX. \int\mkern-6mu\int keeps the thin
// space between two Op atoms, so it is exactly 6mu narrower than \int\int.
// The source makes the kern an Ord atom, which takes a thin space on each
// side, so it was only 3mu narrower.
func TestAKernIsNotAnAtom(t *testing.T) {
	face := testFace(t)
	got := mcWidth(t, face, `\int\int`, Text) - mcWidth(t, face, `\int\mkern-6mu\int`, Text)
	if want := float64(6*32.0) / 18; !near(got, want) {
		t.Errorf(`\int\mkern-6mu\int is %v narrower than \int\int, want %v`, got, want)
	}
}
