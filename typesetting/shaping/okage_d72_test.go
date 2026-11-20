// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package shaping

import (
	"bufio"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage/typesetting/bidi"
	"github.com/xo/okage/typesetting/di"
	ucd "github.com/xo/okage/typesetting/internal/unicodedata"
)

// okage: the tests of D72.

// bidiClassRunes gives a rune for each class of BidiTest.txt, as the tests
// of bidi do.
var bidiClassRunes = map[string]rune{
	"L": 'a', "R": 0x05d0, "EN": '0', "ES": '+', "ET": '#', "AN": 0x0661,
	"CS": '.', "B": '\n', "S": 0x0b, "WS": ' ', "ON": '!', "BN": 0,
	"NSM": 0x0300, "AL": 0x0608, "LRO": 0x202d, "RLO": 0x202e,
	"LRE": 0x202a, "RLE": 0x202b, "PDF": 0x202c, "LRI": 0x2066,
	"RLI": 0x2067, "FSI": 0x2068, "PDI": 0x2069,
}

// readBidiTexts returns the texts of the test files of the bidi algorithm
// in ../bidi/test.
func readBidiTexts(t *testing.T) [][]rune {
	t.Helper()
	var texts [][]rune
	read := func(name string, parse func(line string) []rune) {
		f, err := os.Open("../bidi/test/" + name)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || line[0] == '#' || line[0] == '@' {
				continue
			}
			if text := parse(line); text != nil {
				texts = append(texts, text)
			}
		}
		if err := sc.Err(); err != nil {
			t.Fatal(err)
		}
	}
	read("BidiCharacterTest.txt", func(line string) []rune {
		var text []rune
		for _, f := range strings.Fields(strings.Split(line, ";")[0]) {
			r, err := strconv.ParseUint(f, 16, 32)
			if err != nil {
				t.Fatalf("parsing %q: %v", line, err)
			}
			text = append(text, rune(r))
		}
		return text
	})
	read("BidiTest.txt", func(line string) []rune {
		var text []rune
		for _, f := range strings.Fields(strings.Split(line, ";")[0]) {
			r, ok := bidiClassRunes[f]
			if !ok {
				t.Fatalf("parsing %q: no rune for the class %s", line, f)
			}
			text = append(text, r)
		}
		return text
	})
	return texts
}

// randomBidiTexts returns n random texts. Most of their runes have a bidi
// class that needsBidi accepts, so that many texts skip the algorithm.
func randomBidiTexts(n int) [][]rune {
	rng := rand.New(rand.NewPCG(72, 72))
	// runes of each class but those of bidiClassesRTL, from Latin-1 and
	// beyond: letters, numbers, separators, neutrals, marks, format
	// characters, LRM, CJK, emoji and unassigned code points.
	common := []rune{
		'a', 'Z', '0', '9', '+', '-', '#', '$', '%', '.', ',', ':', '/', ' ',
		'\t', 0x0b, 0x0c, '!', '"', '\'', '(', ')', '[', ']', '{', '}', '<',
		'>', 0x00a0, 0x00ad, 0x00b2, 0x00ab, 0x00bb, 0x00e9, 0x00ff, 0x0300,
		0x0301, 0x200b, 0x200c, 0x200d, 0x200e, 0x2028, 0x2019, 0x201c,
		0x2026, 0x2060, 0x3000, 0x3001, 0x4e00, 0x6587, 0xff01, 0xfe0f,
		0x1f600, 0x1f1fa, 0xe0001, 0x0378, 0x0001, 0x007f, 0x0085,
	}
	// runes of bidiClassesRTL: Hebrew, Arabic, Arabic numbers, RLM and
	// the bidi controls.
	rtl := []rune{
		0x05d0, 0x05ea, 0x0627, 0x0644, 0x0661, 0x06f1, 0x0608, 0x200f,
		0x202a, 0x202b, 0x202c, 0x202d, 0x202e, 0x2066, 0x2067, 0x2068,
		0x2069, 0xfb1d, 0x10800,
	}
	texts := make([][]rune, n)
	for i := range texts {
		text := make([]rune, 1+rng.IntN(60))
		for j := range text {
			switch k := rng.IntN(1000); {
			case k < 3:
				text[j] = rtl[rng.IntN(len(rtl))]
			case k < 10:
				// any code point but the surrogates
				r := rng.Int32N(0x110000 - 0x800)
				if r >= 0xd800 {
					r += 0x800
				}
				text[j] = r
			default:
				text[j] = common[rng.IntN(len(common))]
			}
		}
		texts[i] = text
	}
	return texts
}

// TestBidiSkip makes sure that the full bidi algorithm gives one run at
// level 0 for each paragraph that splitByBidi skips (D72). It splits each
// text into paragraphs as splitByBidi does.
func TestBidiSkip(t *testing.T) {
	texts := readBidiTexts(t)
	texts = append(texts, randomBidiTexts(50000)...)
	var p bidi.Paragraph
	skipped, total := 0, 0
	for _, text := range texts {
		start := 0
		for i, r := range text {
			if !ucd.IsBidiB(r) && i != len(text)-1 {
				continue
			}
			para := text[start : i+1]
			start = i + 1
			total++
			if needsBidi(para) {
				continue
			}
			skipped++
			for _, dir := range []bidi.Direction{bidi.LeftToRight, bidi.Neutral} {
				out := p.Segment(para, dir)
				if out.NumRuns() != 1 {
					t.Fatalf("got %d runs for %U in the direction %d, want 1", out.NumRuns(), para, dir)
				}
				if got, want := out.Run(0), (bidi.Run{Start: 0, End: len(para), Level: 0}); got != want {
					t.Fatalf("got the run %v for %U in the direction %d, want %v", got, para, dir, want)
				}
			}
		}
	}
	// The texts must test both paths.
	if skipped < 10000 || total-skipped < 10000 {
		t.Fatalf("skipped %d of %d paragraphs, want 10,000 or more on each path", skipped, total)
	}
	t.Logf("skipped %d of %d paragraphs", skipped, total)
}

// fullSplitByBidi is splitByBidi without the skip of D72: it runs the
// full bidi algorithm on each paragraph of a text from left to right.
func fullSplitByBidi(p *bidi.Paragraph, text []rune) []Input {
	var runs []Input
	start := 0
	for i, r := range text {
		if !ucd.IsBidiB(r) && i != len(text)-1 {
			continue
		}
		out := p.Segment(text[start:i+1], bidi.LeftToRight)
		for j := 0; j < out.NumRuns(); j++ {
			run := out.Run(j)
			in := Input{Text: text, RunStart: start + run.Start, RunEnd: start + run.End, Level: run.Level}
			if run.IsLeftToRight() {
				in.Direction.SetProgression(di.FromTopLeft)
			} else {
				in.Direction.SetProgression(di.TowardTopLeft)
			}
			runs = append(runs, in)
		}
		start = i + 1
	}
	return runs
}

// TestBidiSkipSplit makes sure that splitByBidi gives the same runs with
// the skip of D72 as with the full bidi algorithm.
func TestBidiSkipSplit(t *testing.T) {
	texts := readBidiTexts(t)
	texts = append(texts, randomBidiTexts(50000)...)
	var seg Segmenter
	var p bidi.Paragraph
	for _, text := range texts {
		seg.reset()
		seg.splitByBidi(Input{Text: text, RunEnd: len(text)})
		want := fullSplitByBidi(&p, text)
		if len(seg.output) != len(want) {
			t.Fatalf("got %d runs for %U, want %d", len(seg.output), text, len(want))
		}
		for i, got := range seg.output {
			w := want[i]
			if got.RunStart != w.RunStart || got.RunEnd != w.RunEnd || got.Level != w.Level || got.Direction != w.Direction {
				t.Fatalf("got the run %d [%d, %d) at the level %d in the direction %v for %U, want [%d, %d) at %d in %v",
					i, got.RunStart, got.RunEnd, got.Level, got.Direction, text, w.RunStart, w.RunEnd, w.Level, w.Direction)
			}
		}
	}
}
