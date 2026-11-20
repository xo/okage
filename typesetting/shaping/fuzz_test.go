package shaping

import (
	"reflect"
	"testing"

	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/language"
	"github.com/xo/okage/typesetting/segmenter"
	"golang.org/x/image/math/fixed"
)

// FuzzE2E shapes and wraps large strings looking for unshapable text or failures
// in rune accounting.
func FuzzE2E(f *testing.F) {
	face := loadOpentypeFont(f, "../font/testdata/Amiri-Regular.ttf")
	f.Add(benchParagraphLatin)
	f.Add(benchParagraphArabic)
	f.Fuzz(func(t *testing.T, input string) {
		textInput := []rune(input)
		var shaper HarfbuzzShaper
		out := []Output{shaper.Shape(Input{
			Text:      textInput,
			RunStart:  0,
			RunEnd:    len(textInput),
			Direction: di.DirectionRTL,
			Face:      face,
			Size:      16 * 72,
			Script:    language.Arabic,
			Language:  language.NewLanguage("AR"),
		})}
		if out[0].Runes.Count != len(textInput) {
			t.Errorf("expected %d shaped runes, got %d", len(textInput), out[0].Runes.Count)
		}
		var l LineWrapper
		outs, _ := l.WrapParagraph(WrapConfig{}, 100, textInput, NewSliceIterator(out))
		totalRunes := 0
		for _, l := range outs {
			for _, run := range l {
				if run.Runes.Offset != totalRunes {
					t.Errorf("expected rune offset %d, got %d", totalRunes, run.Runes.Offset)
				}
				totalRunes += run.Runes.Count
			}
		}
		if totalRunes != len(textInput) {
			t.Errorf("mismatched runes! expected %d, but wrapped output only contains %d", len(textInput), totalRunes)
		}
		_ = outs
	})
}

// FuzzE2EVariableSize shapes and wraps large strings at varying text sizes and line widths.
func FuzzE2EVariableSize(f *testing.F) {
	face := loadOpentypeFont(f, "../font/testdata/Amiri-Regular.ttf")
	f.Add(benchParagraphLatin, byte(16), byte(100), byte(0))
	f.Add(benchParagraphArabic, byte(16), byte(100), byte(0))
	// okage: the input of D73 that truncates the text after 21 lines with
	// the policy Always.
	f.Add(benchParagraphLatin, byte(16), byte(100), byte(0x55))
	f.Fuzz(func(t *testing.T, input string, textSize byte, lineWidth byte, truncateBreakCont byte) {
		// We pack the wrapconfig fields into a byte in order to ensure that the fuzzer spends more time
		// exploring interesting parts of the space instead of varying bits that don't matter.
		wc := WrapConfig{
			TruncateAfterLines: int(truncateBreakCont & 0b11111),
			TextContinues:      truncateBreakCont&0b100000 != 0,
			BreakPolicy:        LineBreakPolicy((truncateBreakCont & 0b11000000 >> 5) % 3),
		}
		textInput := []rune(input)
		var shaper HarfbuzzShaper
		out := []Output{shaper.Shape(Input{
			Text:      textInput,
			RunStart:  0,
			RunEnd:    len(textInput),
			Direction: di.DirectionRTL,
			Face:      face,
			Size:      fixed.I(int(textSize)),
			Script:    language.Arabic,
			Language:  language.NewLanguage("AR"),
		})}
		if out[0].Runes.Count != len(textInput) {
			t.Errorf("expected %d shaped runes, got %d", len(textInput), out[0].Runes.Count)
		}
		var l LineWrapper
		outs, truncated := l.WrapParagraph(wc, int(lineWidth), textInput, NewSliceIterator(out))
		totalRunes := 0
		for i, l := range outs {
			for j, run := range l {
				// okage: the last run of a truncated text is the truncator,
				// and its count is the number of truncated runes. The test
				// adds that number after the loop, so it does not count the
				// truncator here (D73).
				if truncated > 0 && i == len(outs)-1 && j == len(l)-1 {
					if want := (Range{Offset: totalRunes, Count: truncated}); run.Runes != want {
						t.Errorf("expected truncator runes %v, got %v", want, run.Runes)
					}
					continue
				}
				if run.Runes.Offset != totalRunes {
					if reflect.DeepEqual(run, wc.Truncator) {
						continue
					}
					t.Errorf("expected rune offset %d, got %d", totalRunes, run.Runes.Offset)
				}
				totalRunes += run.Runes.Count
			}
		}
		totalRunes += truncated
		if totalRunes != len(textInput) {
			t.Errorf("mismatched runes! expected %d, but wrapped output only contains %d", len(textInput), totalRunes)
		}
		_ = outs
	})
}

func FuzzBreakOptions(f *testing.F) {
	f.Add(string([]rune{183067, 318808839, 476266048}))
	f.Add(benchParagraphArabic)
	f.Add(benchParagraphLatin)
	// okage: the input of D73, a combining mark after a space.
	f.Add("0! \u033d")
	f.Fuzz(func(t *testing.T, input string) {
		runes := []rune(input)
		var seg segmenter.Segmenter
		seg.Init(runes)
		breaker := newBreaker(&seg)
		// okage: the last rune of each grapheme cluster of the whole text
		// (D73).
		var graphemeEnds []int
		var whole segmenter.Segmenter
		whole.Init(runes)
		for iter := whole.GraphemeIterator(); iter.Next(); {
			g := iter.Grapheme()
			graphemeEnds = append(graphemeEnds, g.Offset+len(g.Text)-1)
		}
		var wordOptions []breakOption
		for b, ok := breaker.nextWordBreak(); ok; b, ok = breaker.nextWordBreak() {
			prevRuneIndex := 0
			if len(wordOptions) > 0 {
				prevRuneIndex = wordOptions[len(wordOptions)-1].breakAtRune + 1
			}
			segmentRunes := runes[prevRuneIndex : b.breakAtRune+1]
			segmentGraphemes := []breakOption{}
			for b, ok := breaker.nextGraphemeBreak(); ok; b, ok = breaker.nextGraphemeBreak() {
				// Adjust break offset to be relative to the start of the segment.
				b.breakAtRune -= prevRuneIndex
				segmentGraphemes = append(segmentGraphemes, b)
			}
			// okage: the graphemes of a segment are the grapheme clusters
			// of the whole text that end in the segment. A segmenter of the
			// segment alone ends a cluster at the end of the segment, but a
			// line can break inside a cluster. UAX #14 breaks after a space
			// before a combining mark (LB10 and LB18), and UAX #29 keeps
			// the two in one cluster (GB9). The test checks each segment,
			// not only the first one (D73).
			correctGraphemes := []int{}
			for _, end := range graphemeEnds {
				if end >= prevRuneIndex && end <= b.breakAtRune {
					correctGraphemes = append(correctGraphemes, end-prevRuneIndex)
				}
			}
			same := len(segmentGraphemes) == len(correctGraphemes)
			for i := 0; same && i < len(correctGraphemes); i++ {
				same = segmentGraphemes[i].breakAtRune == correctGraphemes[i]
			}
			if !same {
				t.Errorf("runes[%d:%d] expected %d graphemes, got %d", prevRuneIndex, b.breakAtRune+1, len(correctGraphemes), len(segmentGraphemes))
				t.Errorf("correct graphemes: %v\ngot graphemes: %v", correctGraphemes, segmentGraphemes)
			}
			checkOptions(t, segmentRunes, segmentGraphemes)
			wordOptions = append(wordOptions, b)
		}
		checkOptions(t, runes, wordOptions)
	})
}
