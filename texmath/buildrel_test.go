// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"strings"
	"testing"
)

// TestBuildrelLaysOutAsStackrel, TestLefteqnIsZeroWidthAndKeepsItsExtent and
// TestLefteqnSetsItsContentInDisplayStyle port the tests of the source that
// measure the layout.

func TestBuildrelIsStackrel(t *testing.T) {
	for _, c := range [][2]string{
		{`\buildrel \rm def \over =`, `\stackrel{\rm def}{=}`},
		{`a \buildrel x \over \to b`, `a \stackrel{x}{\to} b`},
		{`a \buildrel \sim \over = b`, `a \stackrel{\sim}{=} b`},
		{`\frac{a}{b} \buildrel p \over \to c`, `\frac{a}{b} \stackrel{p}{\to} c`},
	} {
		if !sameTree(t, c[0], c[1]) {
			t.Errorf("%q does not parse as %q", c[0], c[1])
		}
	}
	// \buildrel reads its top as a list, and \stackrel as a group, so here
	// the top of \buildrel is the group {a \over b}, as one Ord atom, and the
	// top of \stackrel the fraction. The layout of the two is the same.
	ou := findOverUnder(mustParse(t, `\buildrel {a \over b} \over =`))
	if ou == nil || ou.Class != Rel || ou.Over == nil {
		t.Errorf("\\buildrel {a \\over b} \\over = is not a relation with a top")
	}
}

func findOverUnder(l List) *OverUnder {
	for _, n := range l {
		if ou, ok := n.(OverUnder); ok {
			return &ou
		}
	}
	return nil
}

func TestBuildrelWithoutOverIsRefused(t *testing.T) {
	for _, tex := range []string{`\buildrel x`, `\buildrel`, `a \buildrel b c`} {
		_, err := Parse(tex)
		if err == nil || !strings.Contains(err.Error(), `\over`) {
			t.Errorf("%s: error %v does not name \\over", tex, err)
		}
	}
}

func TestBuildrelAndLefteqnReportArgumentErrors(t *testing.T) {
	for _, c := range []struct{ tex, want string }{
		{`\buildrel x \over`, "expected an atom"},
		{`\lefteqn`, "expected an atom"},
		{`\buildrel \unknowncmd \over =`, `unknown command \unknowncmd`},
		{`\buildrel \frac{a \over =`, "missing }"},
		{`\lefteqn{\sqrt[}`, `unexpected "}"`},
	} {
		_, err := Parse(c.tex)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v does not contain %q", c.tex, err, c.want)
		}
	}
}

func TestLefteqnIsALapInDisplayStyle(t *testing.T) {
	l, ok := mustParse(t, `\lefteqn{abc}`)[0].(Lap)
	if !ok {
		t.Fatal(`\lefteqn is not a Lap`)
	}
	if s, ok := l.Body.(StyleChange); !ok || s.Style != Display {
		t.Errorf(`\lefteqn body = %#v, want display style`, l.Body)
	}
}

// \buildrel <top> \over <base> is the \stackrel of plain TeX, and latex.ltx
// gives the two the same expansion:
//
//	:11228  \def\buildrel#1\over#2{\mathrel{\mathop{\kern\z@#2}\limits^{#1}}}
//
// So whatever \stackrel does, \buildrel does. The braced top {a \over b} is
// one Ord atom here and a fraction in \stackrel, and the two lay out the same.
func TestBuildrelLaysOutAsStackrel(t *testing.T) {
	for _, c := range [][2]string{
		{`\buildrel \rm def \over =`, `\stackrel{\rm def}{=}`},
		{`a \buildrel x \over \to b`, `a \stackrel{x}{\to} b`},
		{`a \buildrel \sim \over = b`, `a \stackrel{\sim}{=} b`},
		// A fraction before it: \buildrel is a relation, as \mathrel in the
		// definition says.
		{`\frac{a}{b} \buildrel p \over \to c`, `\frac{a}{b} \stackrel{p}{\to} c`},
		{`\buildrel {a \over b} \over =`, `\stackrel{a \over b}{=}`},
	} {
		got := mustLayout(t, c[0], 10, Text)
		want := mustLayout(t, c[1], 10, Text)
		if got.Width != want.Width || got.Height != want.Height || got.Depth != want.Depth {
			t.Errorf("%s = %.3f/%.3f/%.3f, %s gives %.3f/%.3f/%.3f",
				c[0], got.Width, got.Height, got.Depth, c[1], want.Width, want.Height, want.Depth)
		}
	}
}

// \lefteqn{X} sets X in display style with no width, so that it overhangs to
// the right (latex.ltx:11392, \def\lefteqn#1{\rlap{$\displaystyle #1$}}). The
// width is zero, and the height and the depth are those of the content.
func TestLefteqnIsZeroWidthAndKeepsItsExtent(t *testing.T) {
	plain := mustLayout(t, `abc`, 10, Text)
	lap := mustLayout(t, `\lefteqn{abc}`, 10, Text)
	if lap.Width != 0 {
		t.Errorf(`\lefteqn width = %.3f, want 0`, lap.Width)
	}
	if lap.Height != plain.Height || lap.Depth != plain.Depth {
		t.Errorf(`\lefteqn extent %.3f/%.3f, the content alone is %.3f/%.3f`,
			lap.Height, lap.Depth, plain.Height, plain.Depth)
	}
	if len(lap.Items) != len(plain.Items) {
		t.Errorf(`\lefteqn draws %d items, the content %d`, len(lap.Items), len(plain.Items))
	}
}

// The display style matters: \lefteqn is for the cells of eqnarray, which
// are in display style, and a fraction in it is set at the size of display
// style.
func TestLefteqnSetsItsContentInDisplayStyle(t *testing.T) {
	disp := mustLayout(t, `\displaystyle\frac{a}{b}`, 10, Text)
	lap := mustLayout(t, `\lefteqn{\frac{a}{b}}`, 10, Text)
	if lap.Height != disp.Height || lap.Depth != disp.Depth {
		t.Errorf(`\lefteqn{\frac{a}{b}} extent %.3f/%.3f, display style gives %.3f/%.3f`,
			lap.Height, lap.Depth, disp.Height, disp.Depth)
	}
	// The control: text style differs, so the test proves something.
	text := mustLayout(t, `\frac{a}{b}`, 10, Text)
	if text.Height == disp.Height && text.Depth == disp.Depth {
		t.Error("display and text style agree on this fraction, so this test proves nothing")
	}
}
