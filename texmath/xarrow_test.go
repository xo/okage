// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"math"
	"testing"

	"github.com/xo/okage/typesetting/font"
)

// This file ports the layout parts of xarrow_test.go of the source. The
// source counts the <path> and <rect> elements of the SVG text. These tests
// count the items of the kinds ItemPath and ItemRule. The test of the parse
// of the arrows is TestXArrowSpec in censussymbols_test.go.

// Each extensible arrow lays out in each style, with a head, a path item,
// and a shaft, a rule item. An empty label still gives a bare arrow.
func TestXArrowRenders(t *testing.T) {
	for _, tex := range []string{
		`\xrightarrow{f}`,
		`\xrightarrow[g]{f}`,
		`\xleftarrow{n\to\infty}`,
		`\xleftarrow[a]{b}`,
		`\xleftrightarrow{x}`,
		`\xhookrightarrow{i}`,
		`\xhookleftarrow{j}`,
		`\xmapsto{\phi}`,
		`\xRightarrow{p}`,
		`\xLeftarrow{q}`,
		`\xrightarrow{}`,
		`A \xrightarrow{f} B \xleftarrow[u]{v} C`,
	} {
		for _, st := range allStyles {
			b := mustLayout(t, tex, 32, st)
			if len(itemsOf(b, ItemPath)) == 0 || len(itemsOf(b, ItemRule)) == 0 {
				t.Errorf("%q in style %d: no head or no shaft: %d paths, %d rules",
					tex, st, len(itemsOf(b, ItemPath)), len(itemsOf(b, ItemRule)))
			}
		}
	}
}

// The shaft grows with the width of the label over it, and of the label
// under it.
func TestXArrowStretchesToLabel(t *testing.T) {
	short := mustLayout(t, `\xrightarrow{a}`, 40, Text).Width
	long := mustLayout(t, `\xrightarrow{aaaaaaaa}`, 40, Text).Width
	if !(long > short) {
		t.Errorf("the arrow did not stretch: short %.2f, long %.2f", short, long)
	}
	if under := mustLayout(t, `\xrightarrow[wwwwwwww]{a}`, 40, Text).Width; !(under > short) {
		t.Errorf("the label under the arrow did not stretch it: %.2f against %.2f", under, short)
	}
}

// The label over the arrow raises the box, and the label under it deepens
// it.
func TestXArrowLabelsPlaced(t *testing.T) {
	bare := mustLayout(t, `\xrightarrow{}`, 40, Text)
	over := mustLayout(t, `\xrightarrow{X}`, 40, Text)
	if !(over.Height > bare.Height) {
		t.Errorf("the label over the arrow did not raise it: %.2f against %.2f", over.Height, bare.Height)
	}
	under := mustLayout(t, `\xrightarrow[Y]{X}`, 40, Text)
	if !(under.Depth > over.Depth) {
		t.Errorf("the label under the arrow did not deepen it: %.2f against %.2f", under.Depth, over.Depth)
	}
}

// \xmapsto adds a bar at the tail, a rule, and the hooked arrows add a hook,
// which the plain arrows do not have.
//
// okage: the hook is a filled outline, a path item, so that Box.Path holds
// each item. The source strokes it with fill="none".
func TestXArrowDecorations(t *testing.T) {
	plain := mustLayout(t, `\xrightarrow{f}`, 40, Text)
	hooked := mustLayout(t, `\xhookrightarrow{f}`, 40, Text)
	if got, want := len(itemsOf(hooked, ItemPath)), len(itemsOf(plain, ItemPath))+1; got != want {
		t.Errorf("the hooked arrow has %d paths, want %d: the head and the hook", got, want)
	}
	mapsto := mustLayout(t, `\xmapsto{f}`, 40, Text)
	if a, b := len(itemsOf(mapsto, ItemRule)), len(itemsOf(plain, ItemRule)); a <= b {
		t.Errorf("\\xmapsto has %d rules, the plain arrow %d: the bar at the tail is missing", a, b)
	}
	if hooked.Path() == nil {
		t.Error("the hooked arrow has no path")
	}
}

// The metrics of an arrow, computed again from the font. The arrow is
// labelW + 2 pad wide, or 1.4 em, with a pad of 0.3 em. Its shaft is on the
// math axis, and the label over it sits a gap of 0.12 em over the half width
// of a head, 0.24 em.
//
// okage: the shaft, the heads and the axis are those of the style in force,
// text style here, at the size 40. The source draws them at the size of the
// script style of the labels, 28 here, and on the axis of that size.
func TestXArrowMetrics(t *testing.T) {
	face := testFace(t)
	const size = 40.0
	ss := scriptSize(face, size)
	aw, ah, ad := glyphMetrics(t, face, mathItalic('a'), ss)
	labelW := 0.0
	for range 8 {
		labelW += aw
	}
	pad := float64(size * 0.30)
	b := mustLayout(t, `\xrightarrow{aaaaaaaa}`, size, Text)
	if want := max(labelW+float64(2*pad), float64(size*1.4)); !near(b.Width, want) {
		t.Errorf("width %.6f, want %.6f", b.Width, want)
	}
	axis := mathConst(face, font.AxisHeight, size)
	headHW := float64(size * 0.24)
	gap := float64(size * 0.12)
	if want := axis + headHW + gap + ad + ah; !near(b.Height, want) {
		t.Errorf("height %.6f, want %.6f", b.Height, want)
	}
	rt := max(mathConst(face, font.FractionRuleThickness, size), float64(size*0.045))
	rules := itemsOf(b, ItemRule)
	if len(rules) != 1 {
		t.Fatalf("%d rules, want the shaft", len(rules))
	}
	if want := -axis - rt/2; !near(rules[0].Y, want) || !near(rules[0].H, rt) {
		t.Errorf("the shaft is at %.6f, %.6f thick, want %.6f, %.6f: on the axis of text style",
			rules[0].Y, rules[0].H, want, rt)
	}
}

// The hook of \xhookrightarrow hangs from the shaft, down by its diameter
// and half the thickness of the shaft, and the depth of the box holds it.
// The source leaves the hook out of the depth, so the box was 0 deep.
func TestXArrowHookIsInTheDepth(t *testing.T) {
	face := testFace(t)
	const size = 40.0
	axis := mathConst(face, font.AxisHeight, size)
	rt := max(mathConst(face, font.FractionRuleThickness, size), float64(size*0.045))
	hr := float64(float64(size*0.24) * 0.85)
	want := -axis + float64(2*hr) + rt/2
	for _, tex := range []string{`\xhookrightarrow{a}`, `\xhookleftarrow{a}`} {
		b := mustLayout(t, tex, size, Text)
		if !near(b.Depth, want) {
			t.Errorf("%s: the depth is %.6f, want %.6f", tex, b.Depth, want)
		}
		paths := itemsOf(b, ItemPath)
		if len(paths) != 2 {
			t.Fatalf("%s: %d paths, want a head and a hook", tex, len(paths))
		}
		got := math.Inf(-1)
		for i := range b.Items {
			if b.Items[i].Kind == ItemPath {
				got = max(got, float64(b.ItemPath(i).Bounds().Bottom()))
			}
		}
		if math.Abs(got-want) > 1e-3 {
			t.Errorf("%s: the hook ends at %.4f, want %.4f", tex, got, want)
		}
	}
}

// xArrowSpec gives the heads and the ornaments of each arrow, and false for
// a character that is not an arrow. The source classifies the names of the
// commands, and the port the character of XArrow.Arrow.
func TestXArrowSpecOfTheArrow(t *testing.T) {
	for _, c := range []struct {
		arrow                       rune
		left, right, hooked, mapsto bool
	}{
		{'→', false, true, false, false},
		{'←', true, false, false, false},
		{'↔', true, true, false, false},
		{'⇒', false, true, false, false},
		{'⇐', true, false, false, false},
		{'↪', false, true, true, false},
		{'↩', true, false, true, false},
		{'↦', false, true, false, true},
	} {
		l, r, h, m, ok := xArrowSpec(c.arrow)
		if !ok || l != c.left || r != c.right || h != c.hooked || m != c.mapsto {
			t.Errorf("xArrowSpec(%q) = %v %v %v %v %v", c.arrow, l, r, h, m, ok)
		}
	}
	if _, _, _, _, ok := xArrowSpec('x'); ok {
		t.Error("x is not an extensible arrow")
	}
}
