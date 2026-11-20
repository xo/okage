// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package shaping

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"runtime"
	"strings"
	"testing"

	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/internal/testfonts"
	"github.com/xo/okage/typesetting/language"
	"github.com/xo/okage/typesetting/segmenter"
	"golang.org/x/image/math/fixed"
)

// okage: the tests of D77.

// paragraphCase is a paragraph of the tests of Paragraph: its input, its
// faces and the function that prepares its runs.
type paragraphCase struct {
	name    string
	input   Input
	faces   Fontmap
	prepare func(*Input)
}

// unitSize sets the size of a run to the units of its face, as ivy does.
func unitSize(in *Input) { in.Size = fixed.I(int(in.Face.Upem())) }

// paragraphFont loads a font of typesetting/testdata. It returns nil when
// the folder is not there.
func paragraphFont(t testing.TB, dir, name string) *font.Face {
	t.Helper()
	data, err := testfonts.NewDir(dir).ReadFile(name)
	if testfonts.Missing(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	return face
}

// paragraphWords returns the first n words of a text of the folder
// perf_reference/texts of typesetting/testdata, joined with spaces, or ""
// when the folder is not there.
func paragraphWords(t testing.TB, name string, n int) string {
	t.Helper()
	data, err := testfonts.NewDir("harfbuzz").ReadFile("perf_reference/texts/" + name)
	if testfonts.Missing(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	words := strings.Fields(string(data))
	return strings.Join(words[:min(n, len(words))], " ")
}

// paragraphCases returns the paragraphs of the tests of Paragraph. The
// cases of the fonts of typesetting/testdata are left out when the folder
// is not there.
func paragraphCases(t testing.TB) []paragraphCase {
	t.Helper()
	roboto := loadOpentypeFont(t, "../font/testdata/Roboto-Regular.ttf")
	amiri := loadOpentypeFont(t, "../font/testdata/Amiri-Regular.ttf")
	mono := loadOpentypeFont(t, "../font/testdata/UbuntuMono-R.ttf")
	newCase := func(name, text string, dir di.Direction, lang string, prepare func(*Input), faces ...*font.Face) paragraphCase {
		runes := []rune(text)
		return paragraphCase{
			name: name,
			input: Input{
				Text: runes, RunEnd: len(runes), Direction: dir, Face: faces[0],
				Size: 16 * 64, Language: language.NewLanguage(lang),
			},
			faces:   fixedFontmap(faces),
			prepare: prepare,
		}
	}
	latin := "AVA Wave, To. Tea office ffl fi \"Yes,\" LT. (Wo) [Ty] " + benchParagraphLatin[:900]
	mixed := "The word سلام (salaam) means peace; 2024 saw ١٢٣ «books» and العربية, 27%. " +
		"He read: \"قال الراوي\" [the narrator] — then 456 went on. "
	cases := []paragraphCase{
		newCase("latin", latin, di.DirectionLTR, "en", nil, roboto),
		newCase("latin-units", latin, di.DirectionLTR, "en", unitSize, roboto),
		newCase("mono", latin, di.DirectionLTR, "en", nil, mono),
		newCase("arabic", benchParagraphArabic[:1200], di.DirectionRTL, "ar", nil, amiri),
		newCase("mixed-ltr", strings.Repeat(mixed, 4), di.DirectionLTR, "en", nil, roboto, amiri),
		newCase("mixed-rtl", strings.Repeat(mixed, 4), di.DirectionRTL, "ar", unitSize, roboto, amiri),
	}
	if face := paragraphFont(t, "harfbuzz", "perf_reference/fonts/NotoSansDevanagari-Regular.ttf"); face != nil {
		text := paragraphWords(t, "hi-words.txt", 220) + " (हिन्दी 123) “क्ष त्र ज्ञ” श्री"
		cases = append(cases, newCase("devanagari", text, di.DirectionLTR, "hi", nil, face, roboto))
	}
	if face := paragraphFont(t, "harfbuzz", "perf_reference/fonts/NotoNastaliqUrdu-Regular.ttf"); face != nil {
		text := paragraphWords(t, "fa-words.txt", 160)
		cases = append(cases, newCase("nastaliq", text, di.DirectionRTL, "fa", nil, face, amiri))
	}
	if face := paragraphFont(t, "opentype", "bitmap/NotoColorEmoji.ttf"); face != nil {
		text := strings.Repeat("Good morning 😀, said the crab 🦀, and the flag 🏳️‍🌈 waved 👍🏽 over 👨‍👩‍👧‍👦 the beach 🇫🇷🇯🇵 ☺️ ok. ", 8)
		cases = append(cases, newCase("emoji", text, di.DirectionLTR, "en", nil, roboto, face))
	}
	return cases
}

// freshLine shapes the runes start to end of the paragraph of c alone, as
// Paragraph.Line must give them.
func freshLine(shaper *HarfbuzzShaper, seg *Segmenter, c *paragraphCase, start, end int) []Output {
	in := c.input
	in.Text, in.RunStart, in.RunEnd = c.input.Text[start:end], 0, end-start
	var outs []Output
	for _, item := range seg.Split(in, c.faces) {
		if c.prepare != nil {
			c.prepare(&item)
		}
		outs = append(outs, shaper.Shape(item))
	}
	return outs
}

// diffOutputs returns the first difference between two lists of outputs,
// or "" when they are the same.
func diffOutputs(got, want []Output) string {
	if len(got) != len(want) {
		return fmt.Sprintf("got %d outputs, want %d", len(got), len(want))
	}
	for i := range got {
		g, w := got[i], want[i]
		switch {
		case g.Advance != w.Advance, g.Size != w.Size, g.LineBounds != w.LineBounds,
			g.GlyphBounds != w.GlyphBounds, g.Direction != w.Direction, g.Runes != w.Runes,
			g.Face != w.Face, g.VisualIndex != w.VisualIndex, g.Level != w.Level:
			return fmt.Sprintf("output %d: got advance %d, runes %v, level %d, want %d, %v, %d",
				i, g.Advance, g.Runes, g.Level, w.Advance, w.Runes, w.Level)
		case len(g.Glyphs) != len(w.Glyphs):
			return fmt.Sprintf("output %d: got %d glyphs, want %d", i, len(g.Glyphs), len(w.Glyphs))
		}
		for j := range g.Glyphs {
			// The flags of a glyph are not compared: Line keeps the flags
			// of the paragraph, and HarfBuzz itself does not keep them
			// when it splits a text (hb-buffer-verify.cc).
			gg, wg := g.Glyphs[j], w.Glyphs[j]
			gg.Mask, wg.Mask = 0, 0
			if gg != wg {
				return fmt.Sprintf("output %d, glyph %d: got %+v, want %+v", i, j, g.Glyphs[j], w.Glyphs[j])
			}
		}
	}
	return ""
}

// checkLine compares the outputs of Line for the runes start to end with a
// new shaping of these runes.
func checkLine(t testing.TB, p *Paragraph, shaper *HarfbuzzShaper, seg *Segmenter, c *paragraphCase, start, end int) {
	t.Helper()
	got := p.Line(start, end)
	want := freshLine(shaper, seg, c, start, end)
	if d := diffOutputs(got, want); d != "" {
		t.Fatalf("%s: runes %d to %d, %q: %s", c.name, start, end, string(c.input.Text[start:end]), d)
	}
}

// wrapLines returns the cuts of the lines of a paragraph that break at the
// last break of p before about width runes, as a program that breaks lines
// does.
func wrapLines(p *Paragraph, width int) []int {
	cuts := []int{0}
	for start := 0; start < p.Len(); {
		end, last := start+1, 0
		for ; end <= p.Len(); end++ {
			ok, mandatory := p.LineBreak(end)
			if ok {
				last = end
			}
			if mandatory || (end-start >= width && last > start) {
				break
			}
		}
		if last <= start {
			last = p.Len()
		}
		cuts = append(cuts, last)
		start = last
	}
	return cuts
}

// TestParagraphLine compares the outputs of Line with a new shaping, for
// the lines of each paragraph at several widths, for its starts and its
// ends, and for random ranges.
func TestParagraphLine(t *testing.T) {
	var (
		p      Paragraph
		shaper HarfbuzzShaper
		seg    Segmenter
	)
	rng := rand.New(rand.NewPCG(77, 2026))
	for _, c := range paragraphCases(t) {
		t.Run(c.name, func(t *testing.T) {
			p.Init(c.input, c.faces, c.prepare)
			n := p.Len()
			if p.ShapedRunes() != n {
				t.Fatalf("got %d shaped runes after Init, want %d", p.ShapedRunes(), n)
			}
			lineRunes := 0
			for _, width := range []int{12, 31, 45, 70} {
				cuts := wrapLines(&p, width)
				for i := 1; i < len(cuts); i++ {
					checkLine(t, &p, &shaper, &seg, &c, cuts[i-1], cuts[i])
					lineRunes += cuts[i] - cuts[i-1]
				}
			}
			reshaped := p.ShapedRunes() - n
			t.Logf("%d runes, %d runes of lines, %d shaped again", n, lineRunes, reshaped)
			for i := 0; i <= n; i += 1 + n/97 {
				checkLine(t, &p, &shaper, &seg, &c, i, n)
				checkLine(t, &p, &shaper, &seg, &c, 0, i)
			}
			for range 400 {
				a, b := rng.IntN(n+1), rng.IntN(n+1)
				checkLine(t, &p, &shaper, &seg, &c, min(a, b), max(a, b))
			}
			checkLine(t, &p, &shaper, &seg, &c, 0, n)
		})
	}
}

// TestParagraphReuse makes sure that Line takes most glyphs of the lines
// of a Latin text from the paragraph, and all the glyphs of the whole
// paragraph.
func TestParagraphReuse(t *testing.T) {
	c := paragraphCases(t)[0]
	var p Paragraph
	p.Init(c.input, c.faces, c.prepare)
	n := p.Len()
	p.Line(0, n)
	if p.ShapedRunes() != n {
		t.Fatalf("got %d shaped runes for the whole paragraph, want %d", p.ShapedRunes(), n)
	}
	cuts := wrapLines(&p, 60)
	for i := 1; i < len(cuts); i++ {
		p.Line(cuts[i-1], cuts[i])
	}
	if again := p.ShapedRunes() - n; again > n/4 {
		t.Fatalf("got %d runes shaped again for the lines of %d runes, want at most %d", again, n, n/4)
	}
}

// TestParagraphLineBreak compares LineBreak with the line iterator of the
// segmenter.
func TestParagraphLineBreak(t *testing.T) {
	var p Paragraph
	var seg segmenter.Segmenter
	for _, c := range paragraphCases(t) {
		p.Init(c.input, c.faces, c.prepare)
		n := p.Len()
		want := make([][2]bool, n+1)
		seg.Init(c.input.Text)
		lines := seg.LineIterator()
		for lines.Next() {
			line := lines.Line()
			want[line.Offset+len(line.Text)] = [2]bool{true, line.IsMandatoryBreak}
		}
		for i := range n + 1 {
			ok, mandatory := p.LineBreak(i)
			if [2]bool{ok, mandatory} != want[i] {
				t.Fatalf("%s: rune %d: got %v and %v, want %v", c.name, i, ok, mandatory, want[i])
			}
		}
		if ok, mandatory := p.LineBreak(n); !ok || !mandatory {
			t.Fatalf("%s: got %v and %v at the end, want a mandatory break", c.name, ok, mandatory)
		}
	}
	roboto := loadOpentypeFont(t, "../font/testdata/Roboto-Regular.ttf")
	p.Init(Input{Face: roboto, Size: 16 * 64}, fixedFontmap{roboto}, nil)
	if ok, _ := p.LineBreak(0); ok || p.Len() != 0 || len(p.Line(0, 0)) != 1 {
		t.Fatal("got a break or no output for an empty paragraph")
	}
}

// TestParagraphAllocs makes sure that a Paragraph reuses its memory
// between paragraphs and lines. It uses the Latin cases, because the bidi
// algorithm and the shaper of some scripts, such as Devanagari, allocate
// memory of their own.
func TestParagraphAllocs(t *testing.T) {
	if runtime.Compiler == "tinygo" {
		t.Skip("skipping: the shaper allocates with TinyGo, whose escape analysis keeps less on the stack")
	}
	cases := paragraphCases(t)
	var p Paragraph
	run := func() {
		for _, c := range cases[:3] {
			p.Init(c.input, c.faces, c.prepare)
			for start := 0; start < p.Len(); start += 50 {
				p.Line(start, min(start+60, p.Len()))
			}
		}
	}
	run()
	if allocs := testing.AllocsPerRun(5, run); allocs > 0 {
		t.Fatalf("got %v allocations, want none", allocs)
	}
	// The line breaks allocate at most the iterator of the segmenter, which
	// stays on the stack only when the compiler inlines LineIterator. It
	// does not with -race.
	breaks := func() {
		p.hasBreaks = false
		p.LineBreak(0)
	}
	breaks()
	if allocs := testing.AllocsPerRun(5, breaks); allocs > 1 {
		t.Fatalf("got %v allocations for the line breaks, want at most 1", allocs)
	}
}

// FuzzParagraphLine compares the outputs of Line with a new shaping, for
// a text that the fuzzer makes and for ranges that it picks. Each byte of
// text picks a character of an alphabet of Latin, Arabic with its marks,
// digits, spaces and paired punctuation, which Roboto or Amiri have. Each
// two bytes of cuts pick a range. The alphabet leaves out the characters
// of FuzzUnsafeToBreak, and the tatweel U+0640: in a run of tatweels and
// spaces, whose script is Common, Amiri gives a ligature of tatweels where
// a run starts, and HarfBuzz does not mark the cluster as unsafe to break.
// uharfbuzz 0.57.0 does the same, so Line takes other glyphs than a new
// shaping there (D70).
func FuzzParagraphLine(f *testing.F) {
	roboto := loadOpentypeFont(f, "../font/testdata/Roboto-Regular.ttf")
	amiri := loadOpentypeFont(f, "../font/testdata/Amiri-Regular.ttf")
	alphabet := []rune("AVWTYLPFKkoaecfilty0147.,;:'\" -()[]«»/" +
		"ابتجحدرسعفقكلمنهويةءأإص،؟ ١٢ًَِّْ")
	f.Add([]byte("AVA Wave, To. office (Ty) fi"), []byte{3, 9, 0, 28, 14, 20}, false)
	f.Add([]byte{40, 41, 42, 43, 38, 44, 45, 46, 47, 38, 1, 2, 3, 38, 48, 49, 50, 51}, []byte{0, 5, 4, 12, 9, 18}, true)
	f.Add([]byte{40, 66, 41, 67, 42, 38, 30, 43, 44, 31, 36, 1, 39, 50, 51, 52}, []byte{1, 6, 2, 9}, false)
	// A hamza of Amiri takes another glyph from the letters before it, so
	// a glyph that is safe to break can change when the text before it
	// changes. A first version of Line, which read only the flag
	// unsafe-to-break, gave other glyphs for this input.
	f.Add([]byte("0090900"), []byte("72"), false)
	faces := fixedFontmap{roboto, amiri}
	var (
		p      Paragraph
		shaper HarfbuzzShaper
		seg    Segmenter
	)
	f.Fuzz(func(t *testing.T, text, cuts []byte, rtl bool) {
		runes := make([]rune, 0, min(len(text), 96))
		for _, b := range text[:min(len(text), 96)] {
			runes = append(runes, alphabet[int(b)%len(alphabet)])
		}
		c := paragraphCase{
			name: "fuzz",
			input: Input{
				Text: runes, RunEnd: len(runes), Direction: di.DirectionLTR, Face: roboto,
				Size: 16 * 64, Language: language.NewLanguage("en"),
			},
			faces: faces,
		}
		if rtl {
			c.input.Direction, c.input.Language = di.DirectionRTL, language.NewLanguage("ar")
		}
		p.Init(c.input, c.faces, nil)
		n := p.Len()
		checkLine(t, &p, &shaper, &seg, &c, 0, n)
		for i := 0; i+1 < min(len(cuts), 32); i += 2 {
			a, b := int(cuts[i])%(n+1), int(cuts[i+1])%(n+1)
			checkLine(t, &p, &shaper, &seg, &c, min(a, b), max(a, b))
		}
		cutsOf := wrapLines(&p, 1+len(cuts)%20)
		for i := 1; i < len(cutsOf); i++ {
			checkLine(t, &p, &shaper, &seg, &c, cutsOf[i-1], cutsOf[i])
		}
	})
}
