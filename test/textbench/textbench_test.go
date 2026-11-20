package textbench_test

import (
	"testing"
	"unicode/utf8"

	"golang.org/x/image/math/fixed"

	"github.com/xo/okage/typesetting/bidi"
	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/segmenter"
	"github.com/xo/okage/typesetting/shaping"
)

// reportPerChar reports the time of the benchmark for each character of
// the workload, as ns/char. Call it after the loop of b.Loop.
func reportPerChar(b *testing.B, w *workload) {
	b.Helper()
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(w.chars), "ns/char")
}

// input returns the input of the shaper for a paragraph of the workload,
// at the size of the units of the primary face, as ivy gives it.
func (w *workload) input(text []rune) shaping.Input {
	primary := w.faces.primary()
	return shaping.Input{
		Text:      text,
		RunEnd:    len(text),
		Direction: w.dir,
		Face:      primary,
		Size:      fixed.I(int(primary.Upem())),
		Language:  w.lang,
	}
}

// items returns the runs of Split for each paragraph, copied. The size of
// each run is the size of the units of its face, as ivy gives it, or the
// size of the font in pixels when px is set.
func (w *workload) items(px bool) [][]shaping.Input {
	var seg shaping.Segmenter
	out := make([][]shaping.Input, len(w.runes))
	for i, text := range w.runes {
		for _, item := range seg.Split(w.input(text), w.faces) {
			item.Size = fixed.I(int(item.Face.Upem()))
			if px {
				item.Size = fixed.Int26_6(w.fontSize * 64)
			}
			out[i] = append(out[i], item)
		}
	}
	return out
}

// BenchmarkSegmenter measures the break attributes of the segmenter and
// the line breaks of its iterator, as ivy asks for them.
func BenchmarkSegmenter(b *testing.B) {
	for _, spec := range workloadSpecs {
		b.Run(spec.name, func(b *testing.B) {
			w := spec.get(b)
			var seg segmenter.Segmenter
			b.ReportAllocs()
			for b.Loop() {
				for _, text := range w.runes {
					seg.Init(text)
					lines := seg.LineIterator()
					for lines.Next() {
						_ = lines.Line()
					}
				}
			}
			reportPerChar(b, w)
		})
	}
}

// BenchmarkBidi measures the bidi algorithm on each paragraph.
func BenchmarkBidi(b *testing.B) {
	for _, spec := range workloadSpecs {
		b.Run(spec.name, func(b *testing.B) {
			w := spec.get(b)
			dir := bidi.LeftToRight
			if w.dir == di.DirectionRTL {
				dir = bidi.RightToLeft
			}
			var p bidi.Paragraph
			b.ReportAllocs()
			for b.Loop() {
				for _, text := range w.runes {
					_ = p.Segment(text, dir)
				}
			}
			reportPerChar(b, w)
		})
	}
}

// BenchmarkSplit measures shaping.Segmenter.Split, which splits each
// paragraph by direction, script and face.
func BenchmarkSplit(b *testing.B) {
	for _, spec := range workloadSpecs {
		b.Run(spec.name, func(b *testing.B) {
			w := spec.get(b)
			var seg shaping.Segmenter
			b.ReportAllocs()
			for b.Loop() {
				for _, text := range w.runes {
					_ = seg.Split(w.input(text), w.faces)
				}
			}
			reportPerChar(b, w)
		})
	}
}

// BenchmarkSplitByFace measures shaping.SplitByFace alone.
func BenchmarkSplitByFace(b *testing.B) {
	for _, spec := range workloadSpecs {
		b.Run(spec.name, func(b *testing.B) {
			w := spec.get(b)
			b.ReportAllocs()
			for b.Loop() {
				for _, text := range w.runes {
					_ = shaping.SplitByFace(w.input(text), w.faces)
				}
			}
			reportPerChar(b, w)
		})
	}
}

// BenchmarkShape measures the shaper on the runs of Split, with Shape,
// which allocates the glyphs of each output.
func BenchmarkShape(b *testing.B) {
	for _, spec := range workloadSpecs {
		b.Run(spec.name, func(b *testing.B) {
			w := spec.get(b)
			items := w.items(false)
			var shaper shaping.HarfbuzzShaper
			b.ReportAllocs()
			for b.Loop() {
				for _, paragraph := range items {
					for _, item := range paragraph {
						_ = shaper.Shape(item)
					}
				}
			}
			reportPerChar(b, w)
		})
	}
}

// BenchmarkShapeInto measures the shaper on the runs of Split, with
// ShapeInto, which reuses the glyphs of the last output (D70).
func BenchmarkShapeInto(b *testing.B) {
	for _, spec := range workloadSpecs {
		b.Run(spec.name, func(b *testing.B) {
			w := spec.get(b)
			items := w.items(false)
			var shaper shaping.HarfbuzzShaper
			var glyphs []shaping.Glyph
			b.ReportAllocs()
			for b.Loop() {
				for _, paragraph := range items {
					for _, item := range paragraph {
						glyphs = shaper.ShapeInto(item, glyphs[:0]).Glyphs
					}
				}
			}
			reportPerChar(b, w)
		})
	}
}

// resetter is the method of the RunIterator of shaping.NewSliceIterator
// that gives it new outputs.
type resetter interface {
	Reset(outs []shaping.Output)
}

// BenchmarkWrap measures shaping.LineWrapper, which breaks each shaped
// paragraph into lines of the width of the workload.
func BenchmarkWrap(b *testing.B) {
	for _, spec := range workloadSpecs {
		b.Run(spec.name, func(b *testing.B) {
			w := spec.get(b)
			var shaper shaping.HarfbuzzShaper
			items := w.items(true)
			outs := make([][]shaping.Output, len(items))
			for i, paragraph := range items {
				for _, item := range paragraph {
					outs[i] = append(outs[i], shaper.Shape(item))
				}
			}
			config := shaping.WrapConfig{Direction: w.dir}
			var wrapper shaping.LineWrapper
			iter := shaping.NewSliceIterator(nil)
			reset, ok := iter.(resetter)
			if !ok {
				b.Fatalf("got a RunIterator of the type %T, want a Reset method", iter)
			}
			lines := 0
			b.ReportAllocs()
			for b.Loop() {
				lines = 0
				for i, text := range w.runes {
					reset.Reset(outs[i])
					out, _ := wrapper.WrapParagraph(config, int(w.width), text, iter)
					lines += len(out)
				}
			}
			reportPerChar(b, w)
			b.ReportMetric(float64(lines), "lines/op")
		})
	}
}

// BenchmarkEndToEndWrap measures the path of go-text itself on each
// paragraph: Split, Shape and LineWrapper.
func BenchmarkEndToEndWrap(b *testing.B) {
	for _, spec := range workloadSpecs {
		b.Run(spec.name, func(b *testing.B) {
			w := spec.get(b)
			var seg shaping.Segmenter
			var shaper shaping.HarfbuzzShaper
			var wrapper shaping.LineWrapper
			config := shaping.WrapConfig{Direction: w.dir}
			var outs []shaping.Output
			b.ReportAllocs()
			for b.Loop() {
				for _, text := range w.runes {
					outs = outs[:0]
					for _, item := range seg.Split(w.input(text), w.faces) {
						item.Size = fixed.Int26_6(w.fontSize * 64)
						outs = append(outs, shaper.Shape(item))
					}
					_, _ = wrapper.WrapParagraph(config, int(w.width), text, shaping.NewSliceIterator(outs))
				}
			}
			reportPerChar(b, w)
		})
	}
}

// BenchmarkEndToEnd measures the model of the layout of ivy in
// ivyLayout on each paragraph.
func BenchmarkEndToEnd(b *testing.B) {
	for _, spec := range workloadSpecs {
		b.Run(spec.name, func(b *testing.B) {
			w := spec.get(b)
			l := &ivyLayout{w: w}
			b.ReportAllocs()
			for b.Loop() {
				l.shaped, l.lines = 0, 0
				for _, p := range w.paragraphs {
					l.layOut(p)
				}
			}
			reportPerChar(b, w)
			b.ReportMetric(float64(l.shaped)/float64(w.chars), "shaped/char")
			b.ReportMetric(float64(l.lines), "lines/op")
		})
	}
}

// The constants of SplitFirstLine and shortWideText in
// internal/layout/text_line_break.go of ivy.
const (
	// ratio is the least ratio of the height of a character to its width.
	ratio = 4
	// wideTextMargin is the number of characters that the short text holds
	// after the width of the line.
	wideTextMargin = 24
)

// ivyLayout is a model of the calls of ivy to typesetting, in
// internal/layout of ivy after D70, when it breaks a paragraph into lines.
// It leaves out the work of ivy itself, such as its boxes and its cache of
// shaped texts. For each line:
//
//  1. It takes a short text from the start of the rest of the paragraph,
//     as SplitFirstLine and shortWideText of ivy do.
//  2. It shapes the short text: it decodes the UTF-8, splits the runes
//     with shaping.Segmenter.Split, and shapes each run with ShapeInto.
//  3. It asks the segmenter for the line breaks of the short text.
//  4. It finds the first line in the advances of the clusters. If the
//     whole short text fits and the paragraph goes on, it does steps 2 to 4
//     again on the whole rest of the paragraph.
//  5. It lays out the first line again, as firstLineMetrics of ivy does.
//     ivy takes the glyphs of the line from the short text when the cut is
//     the start of a cluster that is safe to break. To make sure of the
//     runs, it splits the text of the line again (shapeSlice and sameRuns
//     of ivy). Else it shapes the line again.
//
// ivy also lays out the first line with the next word in some cases, which
// the model leaves out.
type ivyLayout struct {
	w       *workload
	seg     shaping.Segmenter
	shaper  shaping.HarfbuzzShaper
	breaker segmenter.Segmenter

	runes  []rune
	glyphs []shaping.Glyph
	// adv is the advance in pixels of the cluster that starts at each rune,
	// start tells which runes start a cluster, and unsafe tells which
	// clusters are unsafe to break. They have one more item for the end.
	adv    []float64
	start  []bool
	unsafe []bool
	// breaks tells before which runes a line can break.
	breaks []bool

	// shaped is the number of characters that the model shaped, and lines
	// the number of lines.
	shaped, lines int
}

// layOut breaks a paragraph into lines.
func (l *ivyLayout) layOut(p string) {
	for off := 0; off < len(p); {
		rest := p[off:]
		short := l.shortText(rest)
		n := l.shape(short)
		l.lineBreaks(short)
		cut, fits := l.firstLine(n)
		if fits && len(short) < len(rest) {
			short = rest
			n = l.shape(short)
			l.lineBreaks(short)
			cut, fits = l.firstLine(n)
		}
		l.lines++
		if fits {
			return
		}
		cutByte := byteOffset(short, cut)
		line := short[:cutByte]
		end := cut
		if l.w.collapse {
			for end > 0 && l.runes[end-1] == ' ' {
				end--
			}
			line = short[:byteOffset(short, end)]
		}
		l.relayOut(line, end)
		off += cutByte
	}
}

// shortText returns the short text of the start of rest that ivy lays out
// to find the first line: as many characters as the width holds at the
// ratio, or with the advances of minAdvance and the margin.
func (l *ivyLayout) shortText(rest string) string {
	width := l.w.width / l.w.fontSize
	short := rest
	if n := int(width * ratio); n < len(rest) {
		short = rest[:byteOffset(rest, n)]
	}
	sum, after := 0.0, 0
	for i, r := range short {
		if r == '\n' {
			break
		}
		if sum <= width {
			sum += minAdvance(r)
			continue
		}
		if after++; after > wideTextMargin {
			return short[:i]
		}
	}
	return short
}

// minAdvance is minAdvance of ivy: the least advance of a character, in
// em.
func minAdvance(r rune) float64 {
	switch {
	case r < 0x3000:
		return 0.25
	case r >= 0x3400 && r <= 0x4dbf, r >= 0x4e00 && r <= 0x9fff, r >= 0xac00 && r <= 0xd7a3,
		r >= 0xf900 && r <= 0xfaff, r >= 0x20000 && r <= 0x3ffff:
		return 0.9
	case r >= 0x3041 && r <= 0x30ff && r != 0x30a0 && r != 0x30fb:
		return 0.75
	case r <= 0x303f, r >= 0xff01 && r <= 0xff60, r >= 0xffe0 && r <= 0xffe6:
		return 0.5
	}
	return 0.25
}

// byteOffset returns the offset in bytes of the rune n of s.
func byteOffset(s string, n int) int {
	i := 0
	for off := range s {
		if i == n {
			return off
		}
		i++
	}
	return len(s)
}

// decode decodes text into the runes of l, and returns their number.
func (l *ivyLayout) decode(text string) int {
	l.runes = l.runes[:0]
	for _, r := range text {
		l.runes = append(l.runes, r)
	}
	return len(l.runes)
}

// shape shapes a text as shapeText of ivy does, and keeps the advances and
// the flags of its clusters. It returns the number of runes of the text.
func (l *ivyLayout) shape(text string) int {
	n := l.decode(text)
	l.adv = resize(l.adv, n+1)
	l.start = resize(l.start, n+1)
	l.unsafe = resize(l.unsafe, n+1)
	for _, item := range l.seg.Split(l.w.input(l.runes), l.w.faces) {
		item.Size = fixed.I(int(item.Face.Upem()))
		out := l.shaper.ShapeInto(item, l.glyphs[:0])
		l.glyphs = out.Glyphs
		scale := l.w.fontSize / float64(item.Face.Upem()) / 64
		for _, g := range out.Glyphs {
			ci := g.TextIndex()
			if !l.start[ci] {
				l.start[ci] = true
				l.unsafe[ci] = g.UnsafeToBreak()
			}
			l.adv[ci] += float64(g.Advance) * scale
		}
	}
	l.start[n] = true
	l.shaped += n
	return n
}

// lineBreaks sets the line breaks of a text, as lineBreaks of ivy does.
func (l *ivyLayout) lineBreaks(text string) {
	n := l.decode(text)
	l.breaks = resize(l.breaks, n+1)
	l.breaker.Init(l.runes)
	lines := l.breaker.LineIterator()
	for lines.Next() {
		line := lines.Line()
		if end := line.Offset + len(line.Text); end > 0 && end < n {
			l.breaks[end] = true
		}
	}
}

// firstLine returns the number of runes of the first line of the n runes
// of the last text, and whether they all fit in the width. A space at the
// end of a line hangs past the width. A word that is wider than the line
// goes to its next break.
func (l *ivyLayout) firstLine(n int) (cut int, fits bool) {
	x, last := 0.0, 0
	for i := range n {
		if i > 0 && l.breaks[i] {
			last = i
		}
		x += l.adv[i]
		if x <= l.w.width || l.runes[i] == ' ' {
			continue
		}
		if last > 0 {
			return last, false
		}
		for j := i + 1; j < n; j++ {
			if l.breaks[j] {
				return j, false
			}
		}
		return n, true
	}
	return n, true
}

// relayOut lays out the text of the first line, whose end is the rune end
// of the last text, as firstLineMetrics of ivy does.
func (l *ivyLayout) relayOut(line string, end int) {
	if end > 0 && l.start[end] && !l.unsafe[end] {
		n := l.decode(line)
		_ = l.seg.Split(l.w.input(l.runes[:n]), l.w.faces)
		return
	}
	l.shape(line)
}

// resize returns s with n items set to zero.
func resize[T any](s []T, n int) []T {
	if cap(s) < n {
		return make([]T, n)
	}
	s = s[:n]
	clear(s)
	return s
}

// TestWorkloads makes sure that each workload loads, and that the model of
// ivy breaks each paragraph into lines that hold all its characters.
func TestWorkloads(t *testing.T) {
	for _, spec := range workloadSpecs {
		t.Run(spec.name, func(t *testing.T) {
			w, err := spec.load()
			if err != nil {
				t.Skipf("skipping %s: %v", spec.name, err)
			}
			if w.chars == 0 {
				t.Fatal("got no characters")
			}
			l := &ivyLayout{w: w}
			for _, p := range w.paragraphs[:min(len(w.paragraphs), 20)] {
				l.layOut(p)
			}
			if l.lines == 0 || l.shaped < utf8.RuneCountInString(w.paragraphs[0]) {
				t.Fatalf("got %d lines and %d shaped characters", l.lines, l.shaped)
			}
		})
	}
}
