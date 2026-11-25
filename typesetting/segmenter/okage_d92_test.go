// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package segmenter

import (
	"math/rand/v2"
	"os"
	"strings"
	"testing"
)

// okage: the tests of D92.

// lineBreaksFromIterator returns the breaks of LineBreaks from the lines of
// the line iterator after Init.
func lineBreaksFromIterator(seg *Segmenter, text []rune) []uint8 {
	seg.Init(text)
	breaks := make([]uint8, len(text)+1)
	iter := seg.LineIterator()
	for iter.Next() {
		line := iter.Line()
		f := LineBreakAllowed
		if line.IsMandatoryBreak {
			f |= LineBreakMandatory
		}
		breaks[line.Offset+len(line.Text)] = f
	}
	return breaks
}

// TestLineBreaks compares LineBreaks with the lines of the line iterator,
// on the texts of the three test files of Unicode, on random texts of
// their runes and on the empty text.
func TestLineBreaks(t *testing.T) {
	var texts [][]rune
	var pool []rune
	for _, file := range []string{"test/LineBreakTest.txt", "test/GraphemeBreakTest.txt", "test/WordBreakTest.txt"} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.SplitSeq(string(b), "\n") {
			if len(line) == 0 || strings.HasPrefix(line, "#") {
				continue
			}
			text, _ := parseUCDTestLineBoundary(t, line)
			texts = append(texts, text)
			pool = append(pool, text...)
		}
	}
	rng := rand.New(rand.NewPCG(92, 2))
	for range 20000 {
		text := make([]rune, rng.IntN(30))
		for i := range text {
			text[i] = pool[rng.IntN(len(pool))]
		}
		texts = append(texts, text)
	}
	texts = append(texts, nil)

	var seg Segmenter
	for _, text := range texts {
		want := lineBreaksFromIterator(&seg, text)
		got := make([]uint8, len(text)+1)
		LineBreaks(text, got)
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%s: got the break %d before the rune %d, want %d", hex(append(text, 0)), got[i], i, want[i])
			}
		}
	}
}
