// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package shaping

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/language"
)

// okage: the tests of D87.

// blockFontmap gives a face to a rune by its block, as a program that has a
// font for each script does: Latin and the punctuation of ASCII in the
// first face, Greek and Cyrillic in the second, and the rest in the third.
type blockFontmap [3]*font.Face

func (bf blockFontmap) ResolveFace(r rune) *font.Face {
	switch {
	case r < 0x370:
		return bf[0]
	case r < 0x530:
		return bf[1]
	default:
		return bf[2]
	}
}

// TestParagraphSplitSkip compares the split of Paragraph with
// Segmenter.Split on long random texts of words, each in one script, so
// that splitRange skips long spans of runes that cannot change its runs.
func TestParagraphSplitSkip(t *testing.T) {
	var (
		p   Paragraph
		seg Segmenter
	)
	roboto := loadOpentypeFont(t, "../font/testdata/Roboto-Regular.ttf")
	amiri := loadOpentypeFont(t, "../font/testdata/Amiri-Regular.ttf")
	mono := loadOpentypeFont(t, "../font/testdata/UbuntuMono-R.ttf")
	alphabets := [][]rune{
		[]rune("AVWTaefilotyz"),
		[]rune("αβγδεΩЖжЯяфы"),
		[]rune("漢字中文書国語"),
		[]rune("ひらがなカタカナ"),
		[]rune("0123456789"),
		[]rune("  　"),
		[]rune(".,;:!?'\"-/。、"),
		[]rune("()[]{}«»「」『』（）【】“”"),
		[]rune("́̈‍‌️"),
	}
	fontmaps := []Fontmap{
		blockFontmap{roboto, amiri, mono},
		blockFontmap{roboto, roboto, mono},
		blockFontmap{mono, amiri, roboto},
		fixedFontmap{roboto},
	}
	rng := rand.New(rand.NewPCG(87, 2026))
	tested := 0
	for i := range 600 {
		var runes []rune
		for len(runes) < 400 {
			// The words of letters are long, and the others are short.
			k := rng.IntN(len(alphabets))
			n := 1 + rng.IntN(3)
			if k < 4 {
				n = 1 + rng.IntN(30)
			}
			for range n {
				runes = append(runes, alphabets[k][rng.IntN(len(alphabets[k]))])
			}
		}
		var face *font.Face
		if i%2 == 0 {
			face = mono
		}
		in := Input{
			Text: runes, RunEnd: len(runes), Direction: di.DirectionLTR, Face: face,
			Size: 16 * 64, Language: language.NewLanguage([]string{"", "en", "ja"}[i%3]),
		}
		p.Init(in, fontmaps[i%len(fontmaps)], nil)
		if checkSplits(t, &p, &seg, rng, fmt.Sprintf("words %d", i), 40) {
			tested++
		}
	}
	if tested < 600 {
		t.Fatalf("got the split of Paragraph for %d texts, want 600", tested)
	}
}

// TestParagraphNext makes sure that nextScript and nextFace give, for each
// rune, the next rune that can change the split, as initNext says.
func TestParagraphNext(t *testing.T) {
	roboto := loadOpentypeFont(t, "../font/testdata/Roboto-Regular.ttf")
	mono := loadOpentypeFont(t, "../font/testdata/UbuntuMono-R.ttf")
	var p Paragraph
	text := []rune("ab (c) 漢字 d「e」 f‍g 中")
	p.Init(Input{Text: text, RunEnd: len(text), Direction: di.DirectionLTR, Size: 16 * 64},
		blockFontmap{roboto, roboto, mono}, nil)
	if !p.ownSplit {
		t.Fatal("got Segmenter.Split, want the split of Paragraph")
	}
	// The runes that can change the split by script: a, each paired
	// delimiter, 漢 and d after the Latin and the Han runes, and 中.
	// The runes that can change the split by face: a, then each rune that
	// is not a space or a joiner, and whose face is not the face of the
	// last such rune: 漢, d, 「, e, 」, f and 中.
	wantScript := []int{0, 3, 5, 7, 10, 11, 13, 19}
	wantFace := []int{0, 7, 10, 11, 12, 13, 15, 19}
	check := func(name string, next []int32, want []int) {
		t.Helper()
		k := 0
		for i := range text {
			for k < len(want) && want[k] < i {
				k++
			}
			w := len(text)
			if k < len(want) {
				w = want[k]
			}
			if int(next[i]) != w {
				t.Fatalf("%s[%d] = %d, want %d", name, i, next[i], w)
			}
		}
	}
	check("nextScript", p.nextScript, wantScript)
	check("nextFace", p.nextFace, wantFace)
}
