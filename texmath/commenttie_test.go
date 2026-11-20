// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"testing"

	"github.com/xo/okage/typesetting/font"
)

// The first tests read the tree. The tests after them measure the widths, as
// the tests of the source do, with the exact widths of the face.

func TestACommentIsNotTypeset(t *testing.T) {
	for _, tex := range []string{
		"a%commentaire\nb",
		"a%b\nb",
		"a%\nb",
		"ab%commentaire",
		"ab%",
	} {
		if !sameTree(t, tex, `ab`) {
			t.Errorf("%q does not parse as ab: the comment is kept", tex)
		}
	}
	if sameTree(t, `a\%b`, `ab`) {
		t.Errorf("%s parses as ab: the literal percent was lost", `a\%b`)
	}
}

func TestATieIsAnInterwordSpaceAndNotATilde(t *testing.T) {
	if !sameTree(t, `a~b`, `a\ b`) {
		t.Error(`a~b is not a\ b: a tie is the control space`)
	}
	l := mustParse(t, `a~b`)
	if s, ok := l[1].(Space); !ok || s.Width != (Length{Value: 6, Unit: Mu}) {
		t.Errorf(`a~b = %#v, want a space of 6mu between a and b`, l)
	}
}

func TestTheCommentAndTieComposeInsideWrappers(t *testing.T) {
	for _, tex := range []string{
		"\\scalebox{0.5}{$a%c\nb$}",
		`\raisebox{1pt}{a~b}`,
		`\text{a~b}`,
		"\\frac{a%c\nb}{c}",
		`x^{a~b}`,
	} {
		mustParse(t, tex)
	}
	if !sameTree(t, "\\scalebox{0.5}{a%c\nb}", `\scalebox{0.5}{ab}`) {
		t.Error("the comment in a scaled box is kept")
	}
	// okage: in \text, the tie is a space of the text.
	l := textOfGroup(t, `\text{a~b}`)
	if r, ok := l[0].(TextRun); !ok || r.Text != "a b" {
		t.Errorf(`\text{a~b} = %#v, want the text "a b"`, l)
	}
}

// ctWidth returns the width of tex at the size 32 in text style.
func ctWidth(t *testing.T, face *font.Face, tex string) float64 {
	t.Helper()
	return onFace(t, face, tex, 32, Text).Width
}

// A comment carries no ink, and a literal percent keeps its glyph.
func TestACommentAddsNoWidth(t *testing.T) {
	face := testFace(t)
	plain := ctWidth(t, face, `ab`)
	for _, tex := range []string{"a%commentaire\nb", "a%b\nb", "a%\nb", "ab%commentaire", "ab%"} {
		if got := ctWidth(t, face, tex); got != plain {
			t.Errorf("%q is %v wide, ab is %v: the comment carries ink", tex, got, plain)
		}
	}
	if got := ctWidth(t, face, `a\%b`); got <= plain {
		t.Errorf(`a\%%b is %v wide, ab is %v: the literal percent was lost`, got, plain)
	}
}

// A tie is the control space, 6mu: at the size 32, a~b is 6*32/18 wider than
// ab, as wide as a\ b, and not as wide as a\sim b.
func TestATieIsSixMathUnitsWide(t *testing.T) {
	face := testFace(t)
	plain, tie, ctrl := ctWidth(t, face, `ab`), ctWidth(t, face, `a~b`), ctWidth(t, face, `a\ b`)
	if tie != ctrl {
		t.Errorf(`a~b is %v wide, a\ b is %v`, tie, ctrl)
	}
	if want := float64(6*32.0) / 18; !near(tie-plain, want) {
		t.Errorf(`a~b is %v wider than ab, want %v`, tie-plain, want)
	}
	if sim := ctWidth(t, face, `a\sim b`); tie == sim {
		t.Errorf(`a~b is as wide as a\sim b (%v): the tie is a glyph`, sim)
	}
}

// The comment and the tie lay out inside the wrappers that lift text into a
// formula, and a scaled comment is half of ab.
func TestTheCommentAndTieLayOutInsideWrappers(t *testing.T) {
	face := testFace(t)
	for _, tex := range []string{
		"\\scalebox{0.5}{$a%c\nb$}",
		`\raisebox{1pt}{a~b}`,
		`\text{a~b}`,
		"\\frac{a%c\nb}{c}",
		`x^{a~b}`,
	} {
		onFace(t, face, tex, 32, Display)
	}
	plain := ctWidth(t, face, `ab`)
	if got, want := ctWidth(t, face, "\\scalebox{0.5}{a%c\nb}"), float64(0.5*plain); got != want {
		t.Errorf("the scaled comment is %v wide, want %v, half of ab", got, want)
	}
}
