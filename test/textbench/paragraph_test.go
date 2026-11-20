package textbench_test

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"golang.org/x/image/math/fixed"

	"github.com/xo/okage/typesetting/shaping"
)

// unitSize sets the size of a run to the units of its face, as ivy does.
func unitSize(in *shaping.Input) { in.Size = fixed.I(int(in.Face.Upem())) }

// BenchmarkEndToEndOnce measures the model of the layout of ivy with
// shaping.Paragraph, in onceLayout, on each paragraph (D77).
func BenchmarkEndToEndOnce(b *testing.B) {
	for _, spec := range workloadSpecs {
		b.Run(spec.name, func(b *testing.B) {
			w := spec.get(b)
			l := &onceLayout{w: w}
			b.ReportAllocs()
			for b.Loop() {
				l.shaped, l.lines = 0, 0
				for _, text := range w.runes {
					l.layOut(text)
				}
			}
			reportPerChar(b, w)
			b.ReportMetric(float64(l.shaped)/float64(w.chars), "shaped/char")
			b.ReportMetric(float64(l.lines), "lines/op")
		})
	}
}

// onceLayout is ivyLayout with shaping.Paragraph, as ivy can use it (D77).
// It shapes each paragraph once with Paragraph.Init. For each line:
//
//  1. It takes the short text of ivyLayout from the start of the rest of
//     the paragraph, and makes it longer up to the next line break, so
//     that its end is safe to break in most texts. The first line does not
//     change, since it breaks before the end of the short text.
//  2. It takes the glyphs of the short text from Paragraph.Line, which
//     gives the glyphs of a new shaping of the short text.
//  3. It asks Paragraph.LineBreak for the line breaks, not the segmenter.
//  4. It finds the first line as ivyLayout does, and takes the whole rest
//     of the paragraph when the short text fits.
//  5. It takes the glyphs of the first line from Paragraph.Line.
type onceLayout struct {
	w    *workload
	para shaping.Paragraph
	// adv is the advance in pixels of the cluster that starts at each rune
	// of the last text, with one more item for the end.
	adv []float64

	// shaped is the number of characters that the model shaped, and lines
	// the number of lines.
	shaped, lines int
}

// layOut breaks a paragraph into lines.
func (l *onceLayout) layOut(text []rune) {
	l.para.Init(l.w.input(text), l.w.faces, unitSize)
	n := len(text)
	for off := 0; off < n; {
		end := off + l.shortRunes(text[off:])
		for end < n {
			if ok, _ := l.para.LineBreak(end); ok {
				break
			}
			end++
		}
		l.measure(off, end)
		cut, fits := l.firstLine(text, off, end)
		if fits && end < n {
			end = n
			l.measure(off, end)
			cut, fits = l.firstLine(text, off, end)
		}
		l.lines++
		if fits {
			break
		}
		lineEnd := cut
		if l.w.collapse {
			for lineEnd > off && text[lineEnd-1] == ' ' {
				lineEnd--
			}
		}
		_ = l.para.Line(off, lineEnd)
		off = cut
	}
	l.shaped += l.para.ShapedRunes()
}

// shortRunes returns the number of runes of the short text of the start of
// rest, as shortText of ivyLayout does with a string.
func (l *onceLayout) shortRunes(rest []rune) int {
	width := l.w.width / l.w.fontSize
	short := min(len(rest), int(width*ratio))
	sum, after := 0.0, 0
	for i, r := range rest[:short] {
		if r == '\n' {
			break
		}
		if sum <= width {
			sum += minAdvance(r)
			continue
		}
		if after++; after > wideTextMargin {
			return i
		}
	}
	return short
}

// measure keeps the advances of the clusters of the runes start to end of
// the paragraph, from Paragraph.Line.
func (l *onceLayout) measure(start, end int) {
	l.adv = resize(l.adv, end-start+1)
	for _, out := range l.para.Line(start, end) {
		scale := l.w.fontSize / float64(out.Face.Upem()) / 64
		for _, g := range out.Glyphs {
			l.adv[g.TextIndex()] += float64(g.Advance) * scale
		}
	}
}

// firstLine returns the end of the first line of the runes start to end
// of text, and whether they all fit in the width, as firstLine of
// ivyLayout does with the breaks of Paragraph.LineBreak.
func (l *onceLayout) firstLine(text []rune, start, end int) (cut int, fits bool) {
	x, last := 0.0, 0
	for i := start; i < end; i++ {
		if i > start {
			if ok, _ := l.para.LineBreak(i); ok {
				last = i
			}
		}
		x += l.adv[i-start]
		if x <= l.w.width || text[i] == ' ' {
			continue
		}
		if last > 0 {
			return last, false
		}
		for j := i + 1; j < end; j++ {
			if ok, _ := l.para.LineBreak(j); ok {
				return j, false
			}
		}
		return end, true
	}
	return end, true
}

// TestEndToEndOnce makes sure that onceLayout breaks the paragraphs of
// each workload into the lines of ivyLayout, with fewer shaped characters.
func TestEndToEndOnce(t *testing.T) {
	for _, spec := range workloadSpecs {
		t.Run(spec.name, func(t *testing.T) {
			w, err := spec.load()
			if err != nil {
				t.Skipf("skipping %s: %v", spec.name, err)
			}
			old, once := &ivyLayout{w: w}, &onceLayout{w: w}
			for i, p := range w.paragraphs[:min(len(w.paragraphs), 300)] {
				lines := old.lines
				old.layOut(p)
				lines = old.lines - lines
				onceLines := once.lines
				once.layOut(w.runes[i])
				if onceLines = once.lines - onceLines; onceLines != lines {
					t.Fatalf("paragraph %d: got %d lines, want %d", i, onceLines, lines)
				}
			}
			if once.shaped >= old.shaped {
				t.Fatalf("got %d shaped characters, want fewer than %d", once.shaped, old.shaped)
			}
			t.Logf("%d lines, %d and %d shaped characters", old.lines, old.shaped, once.shaped)
		})
	}
}

// TestParagraphLine compares the glyphs of Paragraph.Line with a new
// shaping of the same runes, on the paragraphs of each workload: for the
// short texts and the lines of onceLayout, and for random ranges.
func TestParagraphLine(t *testing.T) {
	rng := rand.New(rand.NewPCG(77, 1120))
	for _, spec := range workloadSpecs {
		t.Run(spec.name, func(t *testing.T) {
			w, err := spec.load()
			if err != nil {
				t.Skipf("skipping %s: %v", spec.name, err)
			}
			c := &lineChecker{l: &onceLayout{w: w}}
			l := c.l
			for i, text := range w.runes[:min(len(w.runes), 300)] {
				l.para.Init(w.input(text), w.faces, unitSize)
				n := len(text)
				for off := 0; off < n; {
					end := off + l.shortRunes(text[off:])
					c.check(t, i, text, off, end)
					l.measure(off, end)
					cut, fits := l.firstLine(text, off, end)
					if fits {
						c.check(t, i, text, off, n)
						break
					}
					c.check(t, i, text, off, cut)
					off = cut
				}
				for range 10 {
					a, b := rng.IntN(n+1), rng.IntN(n+1)
					c.check(t, i, text, min(a, b), max(a, b))
				}
			}
		})
	}
}

// lineChecker compares Paragraph.Line of a onceLayout with a new shaping.
type lineChecker struct {
	l      *onceLayout
	seg    shaping.Segmenter
	shaper shaping.HarfbuzzShaper
}

// check compares the outputs of Paragraph.Line for the runes start to end
// of the paragraph i, whose runes are text, with a new shaping.
func (c *lineChecker) check(t *testing.T, i int, text []rune, start, end int) {
	t.Helper()
	got := c.l.para.Line(start, end)
	in := c.l.w.input(text[start:end])
	var want []shaping.Output
	for _, item := range c.seg.Split(in, c.l.w.faces) {
		unitSize(&item)
		want = append(want, c.shaper.Shape(item))
	}
	if d := diffOutputs(got, want); d != "" {
		t.Fatalf("paragraph %d, runes %d to %d, %q: %s", i, start, end, string(text[start:end]), d)
	}
}

// diffOutputs returns the first difference between two lists of outputs,
// or "" when they are the same.
func diffOutputs(got, want []shaping.Output) string {
	if len(got) != len(want) {
		return fmt.Sprintf("got %d outputs, want %d", len(got), len(want))
	}
	for i := range got {
		g, w := got[i], want[i]
		switch {
		case g.Advance != w.Advance, g.Size != w.Size, g.LineBounds != w.LineBounds,
			g.GlyphBounds != w.GlyphBounds, g.Direction != w.Direction, g.Runes != w.Runes,
			g.Face != w.Face, g.VisualIndex != w.VisualIndex, g.Level != w.Level:
			return fmt.Sprintf("output %d: got advance %d and runes %v, want %d and %v", i, g.Advance, g.Runes, w.Advance, w.Runes)
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
