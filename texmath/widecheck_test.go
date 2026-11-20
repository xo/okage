// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"testing"
)

// widecheckSameItems reports whether a and b draw the same items. The face of a glyph
// item is not compared, because each layout of the tests takes its own face.
func widecheckSameItems(a, b *Box) bool {
	if len(a.Items) != len(b.Items) {
		return false
	}
	for i := range a.Items {
		x, y := a.Items[i], b.Items[i]
		x.Face, y.Face = nil, nil
		x.Path, y.Path = nil, nil
		if x != y {
			return false
		}
	}
	return true
}

// \widecheck is the caron that grows of stix.sty, U+030C in
// unicode-math-table.tex, in the class mathaccentwide of \widehat and
// \widetilde, which the accent table sets with a fixed glyph.
func TestWidecheckIsTheCaronAndPairsWithCheck(t *testing.T) {
	// Over one character, 91% of the uses of the corpus, it is \check.
	if !widecheckSameItems(mustLayout(t, `\widecheck{S}`, 32, Display), mustLayout(t, `\check{S}`, 32, Display)) {
		t.Error(`\widecheck{S} does not set as \check{S}`)
	}
	// It is an accent: the box is taller than the letter.
	m1 := mustLayout(t, `\widecheck{S}`, 32, Text)
	m2 := mustLayout(t, `S`, 32, Text)
	if m1.Height <= m2.Height {
		t.Errorf(`\widecheck{S} is %v high, S alone %v: no accent was placed`, m1.Height, m2.Height)
	}
	// An accent does not widen its box.
	if m1.Width != m2.Width {
		t.Errorf(`\widecheck{S} is %v wide, S alone %v`, m1.Width, m2.Width)
	}
}

// okage: the wide accents grow, as phase 26 of D78 adds. The source sets them
// at one size, and this test checked that. Over a wide nucleus the accent is
// the widest horizontal variant of the combining mark that is not wider than
// the nucleus, so it is wider than over a narrow nucleus, and never wider than
// its nucleus unless the narrowest one is.
func TestTheWideAccentsStretch(t *testing.T) {
	face := testFace(t)
	accent := func(tex string, mark rune) (accW, nucW float64) {
		b := mustLayout(t, tex, 32, Text)
		for _, it := range b.Items[1:] {
			if it.Kind == ItemGlyph && it.Text == string(mark) {
				w, _, _ := gidMetrics(face, it.Glyph, 32)
				return w, b.Width
			}
		}
		t.Fatalf("%s has no accent", tex)
		return 0, 0
	}
	for _, c := range []struct {
		wide, narrow string
		mark         rune
	}{
		{`\widecheck{CMNOP}`, `\widecheck{C}`, 0x30C},
		{`\widehat{CMNOP}`, `\widehat{C}`, 0x302},
		{`\widetilde{CMNOP}`, `\widetilde{C}`, 0x303},
	} {
		w, nw := accent(c.wide, c.mark)
		n, _ := accent(c.narrow, c.mark)
		if !(w > n) || w > nw {
			t.Errorf("%s has an accent %v wide over %v, and %s one %v wide", c.wide, w, nw, c.narrow, n)
		}
	}
}

// It lays out where the papers write it.
func TestWidecheckComposes(t *testing.T) {
	for _, tex := range []string{
		`\widecheck{S}`, `\widecheck S`, `\widecheck{S\cup Z}`, `\widecheck{CM}`,
		`\widecheck{S}' = \widecheck{m}`, `x^{\widecheck{S}}`, `\frac{\widecheck{S}}{2}`,
	} {
		mathLayoutOK(t, tex)
	}
}
