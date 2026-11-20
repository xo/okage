// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package shaping

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/font"
	ucd "github.com/xo/okage/typesetting/internal/unicodedata"
	"github.com/xo/okage/typesetting/language"
)

// okage: the tests of the split of Paragraph, item 6 of D72.

// runeFontmap gives the face faces[r%len(faces)] to the rune r, so that
// the face changes often in a text.
type runeFontmap []*font.Face

func (rf runeFontmap) ResolveFace(r rune) *font.Face { return rf[int(r)%len(rf)] }

// scriptFontmap is a fontmap that takes a script, for which Paragraph
// splits with Segmenter.Split.
type scriptFontmap struct{ runeFontmap }

func (scriptFontmap) SetScript(language.Script) {}

// diffInputs returns the first difference between two lists of runs, or
// "" when they are the same.
func diffInputs(got, want []Input) string {
	if len(got) != len(want) {
		return fmt.Sprintf("got %d runs, want %d", len(got), len(want))
	}
	for i := range got {
		g, w := got[i], want[i]
		same := len(g.Text) == len(w.Text) && (len(g.Text) == 0 || &g.Text[0] == &w.Text[0]) &&
			g.RunStart == w.RunStart && g.RunEnd == w.RunEnd && g.Direction == w.Direction &&
			g.Face == w.Face && g.Size == w.Size && g.Script == w.Script &&
			g.Language == w.Language && g.Level == w.Level && len(g.FontFeatures) == len(w.FontFeatures)
		for j := 0; same && j < len(g.FontFeatures); j++ {
			same = g.FontFeatures[j] == w.FontFeatures[j]
		}
		if !same {
			return fmt.Sprintf("run %d: got %d to %d, script %s, language %s, level %d, face %p, "+
				"want %d to %d, %s, %s, %d, %p", i, g.RunStart, g.RunEnd, g.Script, g.Language, g.Level, g.Face,
				w.RunStart, w.RunEnd, w.Script, w.Language, w.Level, w.Face)
		}
	}
	return ""
}

// checkSplit compares splitRange with Segmenter.Split for the runes start
// to end of p.
func checkSplit(t *testing.T, p *Paragraph, seg *Segmenter, name string, start, end int) {
	t.Helper()
	in := p.input
	in.Text, in.RunStart, in.RunEnd = p.text[start:end], 0, end-start
	want := seg.Split(in, p.faces)
	if d := diffInputs(p.splitRange(start, end), want); d != "" {
		t.Fatalf("%s: runes %d to %d, %q: %s", name, start, end, string(p.text[start:end]), d)
	}
}

// checkSplits compares splitRange with Segmenter.Split for the whole
// paragraph of p and for random ranges of it. It returns false when p does
// not use splitRange.
func checkSplits(t *testing.T, p *Paragraph, seg *Segmenter, rng *rand.Rand, name string, ranges int) bool {
	t.Helper()
	if !p.ownSplit {
		return false
	}
	n := p.Len()
	checkSplit(t, p, seg, name, 0, n)
	for range ranges {
		a, b := rng.IntN(n+1), rng.IntN(n+1)
		if a != b {
			checkSplit(t, p, seg, name, min(a, b), max(a, b))
		}
	}
	return true
}

// TestParagraphSplit compares the split of Paragraph with Segmenter.Split
// on the paragraphs of the tests of Paragraph, and on random texts of
// Latin, Greek, Cyrillic, Han, Hiragana, Hangul, marks, digits, spaces and
// paired punctuation, with faces that change often.
func TestParagraphSplit(t *testing.T) {
	var (
		p   Paragraph
		seg Segmenter
	)
	rng := rand.New(rand.NewPCG(72, 2026))
	own := 0
	for _, c := range paragraphCases(t) {
		p.Init(c.input, c.faces, c.prepare)
		if got := checkSplits(t, &p, &seg, rng, c.name, 500); got != p.level0 {
			t.Fatalf("%s: got the split of Paragraph %v, want %v", c.name, got, p.level0)
		} else if got {
			own++
		}
	}
	if own < 3 {
		t.Fatalf("got the split of Paragraph for %d paragraphs, want at least the 3 Latin ones", own)
	}

	roboto := loadOpentypeFont(t, "../font/testdata/Roboto-Regular.ttf")
	amiri := loadOpentypeFont(t, "../font/testdata/Amiri-Regular.ttf")
	mono := loadOpentypeFont(t, "../font/testdata/UbuntuMono-R.ttf")
	alphabet := []rune("AVWTaefiloty0147.,;:!?'\"-/ \u00a0\u3000\t" +
		"()[]{}«»‹›「」『』（）【】〈〉“”" +
		"αβγδΩЖжЯя漢字中文書。、ひらがなカタカナ한국어" +
		"\u0301\u0308\u200d\u200c\u00ad\ufe0f\u3099\u3164\u3164\u17b4")
	fontmaps := []Fontmap{
		fixedFontmap{roboto, amiri, mono},
		runeFontmap{roboto, amiri, mono},
		runeFontmap{mono, roboto},
	}
	languages := []string{"", "en", "ja", "ru", "zz-unknown"}
	tested := 0
	for i := range 3000 {
		runes := make([]rune, 1+rng.IntN(60))
		for k := range runes {
			runes[k] = alphabet[rng.IntN(len(alphabet))]
		}
		var face *font.Face
		if i%3 != 0 {
			face = []*font.Face{roboto, amiri, mono}[i%3]
		}
		in := Input{
			Text: runes, RunEnd: len(runes), Direction: di.DirectionLTR, Face: face,
			Size: 16 * 64, Language: language.NewLanguage(languages[i%len(languages)]),
			Level: 1, Script: language.Arabic,
		}
		p.Init(in, fontmaps[i%len(fontmaps)], nil)
		if checkSplits(t, &p, &seg, rng, fmt.Sprintf("random %d", i), 20) {
			tested++
		}
	}
	if tested < 2900 {
		t.Fatalf("got the split of Paragraph for %d random texts, want most of 3000", tested)
	}

	p.Init(Input{Text: []rune("abc"), RunEnd: 3, Face: roboto}, scriptFontmap{runeFontmap{roboto}}, nil)
	if p.ownSplit {
		t.Fatal("got the split of Paragraph for a FontmapScript, want Segmenter.Split")
	}
}

// TestParagraphSplitSeparators makes sure that each rune that ends a bidi
// paragraph for Segmenter.Split has the bidi class B, which needsBidi
// rejects before the end of a text. So the split of Paragraph has one bidi
// paragraph.
func TestParagraphSplitSeparators(t *testing.T) {
	for r := rune(0); r <= 0x10FFFF; r++ {
		if !ucd.IsBidiB(r) {
			continue
		}
		if class, _ := ucd.LookupBidiClass(r); class != ucd.BD_B {
			t.Errorf("%U ends a bidi paragraph, and its bidi class is %v, not B", r, class)
		}
	}
}
