// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package shaping

import (
	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/harfbuzz"
	ucd "github.com/xo/okage/typesetting/internal/unicodedata"
	"github.com/xo/okage/typesetting/language"
	"github.com/xo/okage/typesetting/segmenter"
)

// okage: this file is okage's own. go-text has no Paragraph (D77).

// Paragraph is a paragraph of text that is shaped once, for the lines that
// a program breaks it into. [Paragraph.Init] splits the paragraph into runs
// and shapes each run. [Paragraph.Line] then gives the glyphs of a range of
// runes, such as a line. It takes them from the shaped paragraph, and it
// shapes again only the glyphs next to a cut that is not safe to break, as
// [Glyph.UnsafeToBreak] tells. The glyphs are the glyphs that a new shaping
// of the range gives, but a glyph taken from the paragraph keeps the flags
// of the paragraph. [Paragraph.LineBreak] gives the line breaks of the
// paragraph, which it computes once.
//
// A Paragraph keeps its memory for the next paragraph, so a program can use
// one Paragraph for all its paragraphs. A Paragraph is not safe for
// concurrent use.
//
// okage: D77.
type Paragraph struct {
	// input is the input of Init, with Text set to text. faces and prepare
	// are the arguments of Init.
	input   Input
	faces   Fontmap
	prepare func(*Input)
	// item is the run that prepare changes. It is in p, so that a run
	// does not move to the heap for each call of prepare.
	item Input

	text    []rune
	split   Segmenter
	shaper  HarfbuzzShaper
	breaker segmenter.Segmenter

	// runs are the runs of the paragraph, and glyphs holds the glyphs of
	// all the runs.
	runs   []paragraphRun
	glyphs []Glyph
	// clusters holds clusterStart, clusterUnsafe and clusterConcat for
	// each rune.
	clusters []uint8
	// breaks holds lineBreak and mandatoryBreak for each rune and for the
	// end of the text, when hasBreaks is true.
	breaks    []uint8
	hasBreaks bool

	// outs are the outputs of the last call of Line, and lineGlyphs holds
	// their glyphs. ends holds the end of the glyphs of each output in
	// lineGlyphs.
	outs       []Output
	lineGlyphs []Glyph
	ends       []int
	// piece is the memory of the shaper for a part of a run. left and
	// right hold the glyphs of the parts at the ends of a run.
	piece, left, right []Glyph

	// shaped is the number of runes that the shaper shaped since Init.
	shaped int
	// level0 is true when the paragraph is horizontal, from left to right,
	// and the bidi algorithm gives level 0 to each of its runes.
	level0 bool

	// okage: D72. When ownSplit is true, Init and Line split the runes
	// with splitRange, from the properties of each rune in info, and not
	// with [Segmenter.Split]. lang and langOK are the language of the
	// input as Split resolves it. scriptRuns, faceRuns and delims are the
	// memory of splitRange.
	ownSplit bool
	info     []runeInfo
	// okage: D87. nextScript and nextFace hold, for each rune, the next
	// rune from it that can change the split by script or the split by
	// face, or the end of the text. splitRange skips the runes before it.
	nextScript, nextFace []int32
	lang                 language.LangID
	langOK               bool
	scriptRuns, faceRuns []Input
	delims               []delimEntry
}

// runeInfo holds the properties of a rune that [Segmenter.Split] reads.
//
// okage: D72.
type runeInfo struct {
	// face is the face that the Fontmap gives for the rune, when ignore is
	// false.
	face *font.Face
	// script is the script of the rune.
	script language.Script
	// delim is the index of the rune in pairedDelims when its script is
	// not strong, or else -1.
	delim int16
	// ignore is true when the rune does not change the face, as
	// ignoreFaceChange says.
	ignore bool
}

// paragraphRun is a run of a Paragraph.
type paragraphRun struct {
	// split is the run of the split, and input is the run after prepare.
	split, input Input
	// out is the output of the run, without its glyphs, which are
	// glyphs[g0:g1] of the Paragraph.
	out    Output
	g0, g1 int
	// monotone is true when the clusters of the glyphs only go up or only
	// go down, and reverse is true when they go down, as in a run from
	// right to left.
	monotone, reverse bool
}

// The flags of Paragraph.clusters.
const (
	// clusterStart tells that a cluster starts at the rune.
	clusterStart uint8 = 1 << iota
	// clusterUnsafe tells that the cluster that starts at the rune is
	// unsafe to break.
	clusterUnsafe
	// clusterConcat tells that the cluster that starts at the rune is
	// unsafe to concat: the flag unsafe-to-concat of HarfBuzz.
	clusterConcat
)

// glyphFlags are the flags of HarfBuzz in the mask of a glyph that Line
// reads. Only the shaper of a Paragraph sets the flag unsafe-to-concat, so
// Line removes it from the glyphs that it gives.
const glyphFlags = harfbuzz.GlyphUnsafeToBreak | harfbuzz.GlyphUnsafeToConcat

// The flags of Paragraph.breaks.
const (
	// lineBreak tells that a line can break before the rune.
	lineBreak uint8 = 1 << iota
	// mandatoryBreak tells that a line must break before the rune.
	mandatoryBreak
)

// contextLength is the number of runes before and after a run that the
// shaper reads as its context. It is contextLength of harfbuzz.
const contextLength = 5

// Init shapes a paragraph, and drops the paragraph that p held before.
//
// The paragraph is all of input.Text. Init splits it with
// [Segmenter.Split] and faces, and shapes each run with
// [HarfbuzzShaper.Shape]. Init ignores input.RunStart and input.RunEnd.
// When prepare is not nil, Init calls it on each run of the split before it
// shapes the run, for example to set the size or the features of the run
// for its face. prepare must not change Text, RunStart or RunEnd. faces
// must give the same face for a rune each time, and prepare the same
// changes for the same run, since [Paragraph.Line] uses them again.
//
// okage: D72. For a paragraph from left to right with no bidi runs, Init
// keeps the script and the face of each rune, and splits the paragraph
// from them with the steps of Split. The split is the same. Line splits a
// range of runes from them too. Init does not do this when faces is a
// [FontmapScript].
//
// Init copies input.Text, so the caller can change it after Init returns.
func (p *Paragraph) Init(input Input, faces Fontmap, prepare func(*Input)) {
	p.text = append(p.text[:0], input.Text...)
	input.Text, input.RunStart, input.RunEnd = p.text, 0, len(p.text)
	p.input, p.faces, p.prepare = input, faces, prepare
	p.hasBreaks = false
	p.shaped = 0
	p.shaper.concat = true
	p.level0 = !input.Direction.IsVertical() && input.Direction.Progression() == di.FromTopLeft &&
		!needsBidi(p.text)
	p.clusters = zeroed(p.clusters, len(p.text)+1)
	clear(p.runs)
	p.runs = p.runs[:0]
	p.glyphs = p.glyphs[:0]
	p.initSplit()
	var splits []Input
	if p.ownSplit {
		splits = p.splitRange(0, len(p.text))
	} else {
		splits = p.split.Split(input, faces)
	}
	for _, split := range splits {
		item := p.prepared(split)
		// The shaper writes the glyphs after the glyphs of the runs before
		// when p.glyphs has the room for them.
		g0 := len(p.glyphs)
		buf := p.glyphs[g0:]
		out := p.shaper.ShapeInto(item, buf)
		if len(out.Glyphs) <= cap(buf) {
			p.glyphs = p.glyphs[:g0+len(out.Glyphs)]
		} else {
			p.glyphs = append(p.glyphs, out.Glyphs...)
		}
		p.shaped += item.RunEnd - item.RunStart
		run := paragraphRun{split: split, input: item, out: out, g0: g0, g1: len(p.glyphs)}
		run.out.Glyphs = nil
		up, down := true, true
		glyphs := p.glyphs[g0:]
		for i, g := range glyphs {
			p.clusters[g.ClusterIndex] |= clusterStart
			if g.UnsafeToBreak() {
				p.clusters[g.ClusterIndex] |= clusterUnsafe
			}
			if g.Mask&harfbuzz.GlyphUnsafeToConcat != 0 {
				p.clusters[g.ClusterIndex] |= clusterConcat
			}
			if i > 0 {
				up = up && glyphs[i-1].ClusterIndex <= g.ClusterIndex
				down = down && glyphs[i-1].ClusterIndex >= g.ClusterIndex
			}
		}
		run.monotone, run.reverse = up || down, !up
		p.runs = append(p.runs, run)
	}
}

// Len returns the number of runes of the paragraph.
func (p *Paragraph) Len() int { return len(p.text) }

// ShapedRunes returns the number of runes that the shaper shaped since
// Init: the runes of the paragraph, and the runes that Line shaped again.
// A program can use it to measure how much Line reuses.
func (p *Paragraph) ShapedRunes() int { return p.shaped }

// LineBreak reports whether a line can break before the rune i of the
// paragraph, and whether it must break there. i is from 0 to [Paragraph.Len],
// which is the end of the paragraph. The breaks are the breaks of
// [segmenter.Segmenter.LineIterator] for the paragraph: a line never
// breaks at 0, and it always breaks at the end. LineBreak computes the
// breaks of the paragraph at its first call after Init.
func (p *Paragraph) LineBreak(i int) (ok, mandatory bool) {
	if !p.hasBreaks {
		p.computeBreaks()
	}
	f := p.breaks[i]
	return f&lineBreak != 0, f&mandatoryBreak != 0
}

// computeBreaks computes the line breaks of the paragraph.
func (p *Paragraph) computeBreaks() {
	p.breaks = zeroed(p.breaks, len(p.text)+1)
	p.hasBreaks = true
	if len(p.text) == 0 {
		return
	}
	p.breaker.Init(p.text)
	lines := p.breaker.LineIterator()
	for lines.Next() {
		line := lines.Line()
		f := lineBreak
		if line.IsMandatoryBreak {
			f |= mandatoryBreak
		}
		p.breaks[line.Offset+len(line.Text)] = f
	}
}

// Line returns the outputs of the runes start to end of the paragraph. They
// are the outputs that a program gets when it splits the same input with
// Text set to these runes alone, calls prepare on each run, and shapes each
// run with [HarfbuzzShaper.Shape]. So the glyphs, their clusters and the
// runes of each output count from start.
//
// Line splits the runes again, because a run of the paragraph can split in
// another way at a cut. It skips the split when the runes are in one run of
// a paragraph from left to right that has no bidi runs, and the split of
// the runes is sure to give that run. Otherwise, for such a paragraph, it
// splits the runes from the script and the face of each rune that Init
// kept, and does not look them up again (D72). It takes the glyphs of each
// run from the shaped paragraph when the run of the paragraph has the same
// face, script, direction, language, size and features, and when the cut
// at each end is safe to break. For a script that joins its letters, such as Arabic, a
// cut where the shaper reads other runes around the run than in the
// paragraph is not safe either.
//
// At a cut that is not safe, Line follows the method that HarfBuzz gives
// for its flag unsafe-to-concat, which the shaper of a Paragraph also
// sets. Line finds the next cluster of the paragraph that is safe to
// concat, and shapes the runes from the cut to that cluster, and one
// cluster more. If the cluster is safe to concat in this new shaping too,
// Line joins the new glyphs before the cluster to the glyphs of the
// paragraph from the cluster on. Otherwise it tries the next cluster. The
// end of a line is the same, toward the start of the line. When no cluster
// joins, Line shapes the run again whole.
//
// Line trusts the flags of HarfBuzz. HarfBuzz misses a few clusters that
// are unsafe to break, such as a character that the font does not have,
// which it decomposes in another way when a mark is elsewhere in the text
// (D70). Line can then give other glyphs than a new shaping.
//
// The mask of a glyph that Line takes from the paragraph holds the flag
// unsafe-to-break of the paragraph. Near a cut, a new shaping of the range
// can set the flag on other clusters, because a lookup can read the runes
// after the cut in the paragraph and the end of the text in the range.
// HarfBuzz does not keep the flags when it splits a text either: its
// option --verify does not compare them. A glyph that Line shapes again
// has the flags of its new shaping.
//
// The outputs and their glyphs are valid until the next call of Line or
// Init. Line panics unless 0 <= start <= end <= [Paragraph.Len].
func (p *Paragraph) Line(start, end int) []Output {
	if start < 0 || end > len(p.text) || start > end {
		panic("shaping: Paragraph.Line: range out of bounds")
	}
	in := p.input
	in.Text, in.RunStart, in.RunEnd = p.text[start:end], 0, end-start
	clear(p.outs)
	p.outs = p.outs[:0]
	p.ends = p.ends[:0]
	p.lineGlyphs = p.lineGlyphs[:0]
	switch item, ok := p.splitInRun(start, end); {
	case ok:
		p.lineRun(p.prepared(item), start, end)
	case p.ownSplit && start < end:
		// okage: D72.
		for _, item := range p.splitRange(start, end) {
			p.lineRun(p.prepared(item), start, end)
		}
	default:
		for _, item := range p.split.Split(in, p.faces) {
			p.lineRun(p.prepared(item), start, end)
		}
	}
	for i := range p.lineGlyphs {
		p.lineGlyphs[i].Mask &^= harfbuzz.GlyphUnsafeToConcat
	}
	g0 := 0
	for i, g1 := range p.ends {
		p.outs[i].Glyphs = p.lineGlyphs[g0:g1:g1]
		g0 = g1
	}
	return p.outs
}

// splitInRun returns the one run that [Segmenter.Split] gives for the
// runes start to end, when it can tell it without the split: the runes are
// in one run of the paragraph, and the run of the split of the runes alone
// is that run, for these reasons:
//
//  1. The bidi algorithm gives level 0 to each rune of the paragraph, so it
//     does to the runes too.
//  2. The run has one strong script, or none. The runes have the script of
//     the run when they hold a rune of that script, or when the run has no
//     strong script. A paired punctuation only takes a script that the run
//     already has.
//  3. The faces gave the face of the run to each rune of the run that can
//     change the face. The first rune of the split takes the face of the
//     input when it cannot change the face, so the face of the run must be
//     the face of the input, or the first rune must be able to change it.
//
// The faces must give the same face for a rune each time, as a Fontmap
// that keeps the face of each rune does.
func (p *Paragraph) splitInRun(start, end int) (Input, bool) {
	if !p.level0 || start == end {
		return Input{}, false
	}
	run := p.runAt(start)
	if end > run.split.RunEnd {
		return Input{}, false
	}
	item := run.split
	if item.Script != language.Common && !hasScript(p.text[start:end], item.Script) {
		return Input{}, false
	}
	if item.Face != p.input.Face && ignoreFaceChange(p.text[start]) {
		return Input{}, false
	}
	item.Text, item.RunStart, item.RunEnd = p.text[start:end], 0, end-start
	return item, true
}

// hasScript reports whether the first rune of text with a strong script
// has the script s.
func hasScript(text []rune, s language.Script) bool {
	for _, r := range text {
		if rs := language.LookupScript(r); rs.Strong() {
			return rs == s
		}
	}
	return false
}

// prepared returns a run after prepare.
func (p *Paragraph) prepared(item Input) Input {
	if p.prepare == nil {
		return item
	}
	p.item = item
	p.prepare(&p.item)
	item = p.item
	p.item = Input{}
	return item
}

// lineRun adds the output of a run of the split of the runes start to end
// to the outputs of Line.
func (p *Paragraph) lineRun(item Input, start, end int) {
	a, b := start+item.RunStart, start+item.RunEnd
	if a == b {
		p.shapeRun(item)
		return
	}
	run := p.runAt(a)
	if !run.monotone || b > run.input.RunEnd || !sameShaping(&item, &run.input) {
		p.shapeRun(item)
		return
	}
	// The shaper of a script that joins its letters reads the runes around
	// a run. An end of the run reads the same runes as in the paragraph
	// when at least contextLength runes of the range are on the other
	// side, or when the paragraph ends there too. The runes around an end
	// also do not change the glyphs when the rune on either side of the
	// end never joins, because the shaper stops at the first rune that is
	// not transparent.
	other := !joinsLetters(item.Script)
	leftContext := other || a-start >= contextLength || start == 0 ||
		nonJoining(p.text[a-1]) || nonJoining(p.text[a])
	rightContext := other || end-b >= contextLength || end == len(p.text) ||
		nonJoining(p.text[b-1]) || nonJoining(p.text[b])
	c1, c2 := a, b
	p.left, p.right = p.left[:0], p.right[:0]
	if !leftContext || (a > run.input.RunStart && !p.safe(a)) {
		if c1 = p.spliceLeft(item, a, b, start); c1 == b {
			p.shapeRun(item)
			return
		}
	}
	if !rightContext || (b < run.input.RunEnd && !p.safe(b)) {
		if c2 = p.spliceRight(item, a, c1, b, start); c2 == c1 {
			p.shapeRun(item)
			return
		}
	}

	g0 := len(p.lineGlyphs)
	if run.reverse {
		p.lineGlyphs = append(p.lineGlyphs, p.right...)
	} else {
		p.lineGlyphs = append(p.lineGlyphs, p.left...)
	}
	// okage: D81. One copy of the glyphs, then the clusters move in place.
	g1 := len(p.lineGlyphs)
	p.lineGlyphs = append(p.lineGlyphs, p.runGlyphs(run, c1, c2)...)
	for i := range p.lineGlyphs[g1:] {
		p.lineGlyphs[g1+i].ClusterIndex -= start
	}
	g2 := len(p.lineGlyphs)
	if run.reverse {
		p.lineGlyphs = append(p.lineGlyphs, p.left...)
	} else {
		p.lineGlyphs = append(p.lineGlyphs, p.right...)
	}
	out := run.out
	out.Glyphs = p.lineGlyphs[g0:]
	out.Runes = Range{Offset: item.RunStart, Count: item.RunEnd - item.RunStart}
	out.Level = item.Level
	countLineClusters(out.Glyphs, g1-g0, g2-g0, c1-start, &item, run.reverse)
	out.RecalculateAll()
	out.Glyphs = nil
	p.outs = append(p.outs, out)
	p.ends = append(p.ends, len(p.lineGlyphs))
}

// countLineClusters sets the counts of the clusters of the glyphs of a run
// of Line, as countClusters does for the whole run. glyphs[g1:g2] are the
// glyphs that Line takes from the paragraph, from the cluster c1, which
// counts from the start of the line. The glyphs before and after them are
// the parts that Line shaped again.
//
// The glyphs from the paragraph keep the counts that countClusters gave
// them in the paragraph. Each of their clusters is whole, and the cluster
// next to each end starts at the same rune as in the paragraph, so
// countClusters gives them the same counts again. So only the parts at the
// ends need countClusters, each with the cluster next to it.
//
// okage: D87.
func countLineClusters(glyphs []Glyph, g1, g2, c1 int, item *Input, reverse bool) {
	dir := item.Direction.Progression()
	if g1 == g2 || reverse != (dir == di.TowardTopLeft) {
		countClusters(glyphs, item.RunEnd, dir)
		return
	}
	// The left part holds the clusters before c1, and the glyphs from the
	// paragraph start at c1. The right part ends at the end of the run.
	left, right := glyphs[:g1], glyphs[g2:]
	if reverse {
		left, right = right, left
	}
	countClusters(left, c1, dir)
	countClusters(right, item.RunEnd, dir)
}

// spliceLeft shapes the start of the part a to b of a run, which starts
// at a cut that is not safe to break. It returns the cluster c where the
// glyphs of the paragraph take over, with the glyphs of a to c in p.left.
// It returns b when no cluster in the first half of the part can take
// over, since the shaper then shapes the part whole for less.
func (p *Paragraph) spliceLeft(item Input, a, b, start int) int {
	half := a + (b-a)/2
	for c := p.nextCluster(a, b, unsafeConcat); c <= half; c = p.nextCluster(c, b, unsafeConcat) {
		piece := p.shapePiece(item, a, p.nextCluster(c, b, 0), start)
		if concatAt(piece, c-start) {
			p.left = appendClusters(p.left[:0], piece, a-start, c-start)
			return c
		}
	}
	return b
}

// spliceRight shapes the end of the part c1 to b of a run, which starts
// at a and ends at a cut that is not safe to break. It returns the cluster
// c where the glyphs of the paragraph stop, with the glyphs of c to b in
// p.right. It returns c1 when no cluster after c1 and in the last half of
// the part can stop them.
func (p *Paragraph) spliceRight(item Input, a, c1, b, start int) int {
	half := b - (b-a)/2
	for c := p.prevCluster(b, c1, unsafeConcat); c > c1 && c >= half; c = p.prevCluster(c, c1, unsafeConcat) {
		piece := p.shapePiece(item, p.prevCluster(c, c1, 0), b, start)
		if concatAt(piece, c-start) {
			p.right = appendClusters(p.right[:0], piece, c-start, b-start)
			return c
		}
	}
	return c1
}

// concatAt reports whether a cluster starts at the rune c in glyphs, and
// whether it is safe to break and to concat.
func concatAt(glyphs []Glyph, c int) bool {
	found := false
	for _, g := range glyphs {
		if g.ClusterIndex == c {
			if g.Mask&glyphFlags != 0 {
				return false
			}
			found = true
		}
	}
	return found
}

// shapePiece shapes the runes a to b of a run of the split of Line, whose
// text starts at the rune start of the paragraph. The glyphs are valid
// until the next call of shapePiece or shapeRun.
func (p *Paragraph) shapePiece(item Input, a, b, start int) []Glyph {
	item.RunStart, item.RunEnd = a-start, b-start
	out := p.shaper.ShapeInto(item, p.piece[:0])
	p.piece = out.Glyphs
	p.shaped += b - a
	return out.Glyphs
}

// shapeRun shapes a run of the split of Line whole, and adds its output to
// the outputs of Line.
func (p *Paragraph) shapeRun(item Input) {
	out := p.shaper.ShapeInto(item, p.piece[:0])
	p.piece = out.Glyphs
	p.shaped += item.RunEnd - item.RunStart
	p.lineGlyphs = append(p.lineGlyphs, out.Glyphs...)
	out.Glyphs = nil
	p.outs = append(p.outs, out)
	p.ends = append(p.ends, len(p.lineGlyphs))
}

// runAt returns the run of the paragraph that holds the rune i.
func (p *Paragraph) runAt(i int) *paragraphRun {
	lo, hi := 0, len(p.runs)-1
	for lo < hi {
		m := int(uint(lo+hi+1) >> 1)
		if p.runs[m].input.RunStart <= i {
			lo = m
		} else {
			hi = m - 1
		}
	}
	return &p.runs[lo]
}

// unsafeConcat is the flags of a cluster that is not safe to concat.
const unsafeConcat = clusterUnsafe | clusterConcat

// safe reports whether a cluster starts at the rune i, and whether it is
// safe to break.
func (p *Paragraph) safe(i int) bool {
	return p.clusters[i]&(clusterStart|clusterUnsafe) == clusterStart
}

// nextCluster returns the first rune after i and before limit where a
// cluster starts that has none of the flags unsafe, or limit.
func (p *Paragraph) nextCluster(i, limit int, unsafe uint8) int {
	for i++; i < limit; i++ {
		if p.clusters[i]&(clusterStart|unsafe) == clusterStart {
			return i
		}
	}
	return limit
}

// prevCluster returns the last rune before i and after limit where a
// cluster starts that has none of the flags unsafe, or limit.
func (p *Paragraph) prevCluster(i, limit int, unsafe uint8) int {
	for i--; i > limit; i-- {
		if p.clusters[i]&(clusterStart|unsafe) == clusterStart {
			return i
		}
	}
	return limit
}

// runGlyphs returns the glyphs of the clusters lo to hi of a run, whose
// clusters are monotone.
func (p *Paragraph) runGlyphs(run *paragraphRun, lo, hi int) []Glyph {
	glyphs := p.glyphs[run.g0:run.g1]
	if run.reverse {
		return glyphs[searchBelow(glyphs, hi):searchBelow(glyphs, lo)]
	}
	return glyphs[searchFrom(glyphs, lo):searchFrom(glyphs, hi)]
}

// searchFrom returns the first glyph whose cluster is c or more, in glyphs
// whose clusters go up.
func searchFrom(glyphs []Glyph, c int) int {
	lo, hi := 0, len(glyphs)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if glyphs[m].ClusterIndex < c {
			lo = m + 1
		} else {
			hi = m
		}
	}
	return lo
}

// searchBelow returns the first glyph whose cluster is less than c, in
// glyphs whose clusters go down.
func searchBelow(glyphs []Glyph, c int) int {
	lo, hi := 0, len(glyphs)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if glyphs[m].ClusterIndex >= c {
			lo = m + 1
		} else {
			hi = m
		}
	}
	return lo
}

// appendClusters appends to dst the glyphs of glyphs whose clusters are
// lo to hi, in their order.
func appendClusters(dst, glyphs []Glyph, lo, hi int) []Glyph {
	for _, g := range glyphs {
		if g.ClusterIndex >= lo && g.ClusterIndex < hi {
			dst = append(dst, g)
		}
	}
	return dst
}

// sameShaping reports whether the shaper gives the same glyphs to the
// same runes of two runs: the runs have the same face, script, direction,
// language, size and features.
func sameShaping(a, b *Input) bool {
	if a.Face != b.Face || a.Script != b.Script || a.Direction != b.Direction ||
		a.Language != b.Language || a.Size != b.Size || len(a.FontFeatures) != len(b.FontFeatures) {
		return false
	}
	for i, f := range a.FontFeatures {
		if f != b.FontFeatures[i] {
			return false
		}
	}
	return true
}

// joinsLetters reports whether the shaper joins the letters of a script,
// and so reads the runes around a run. It is hasArabicJoining of harfbuzz.
func joinsLetters(script language.Script) bool {
	switch script {
	case language.Adlam, language.Arabic, language.Chorasmian, language.Hanifi_Rohingya, language.Mandaic,
		language.Manichaean, language.Mongolian, language.Nko, language.Old_Uyghur, language.Phags_Pa,
		language.Psalter_Pahlavi, language.Sogdian, language.Syriac:
		return true
	}
	return false
}

// nonJoining reports whether r never joins a letter and is not
// transparent to the joining of letters. The table of the joining types of
// harfbuzz starts at U+0600, and a rune that is not in it is transparent
// when it is a mark or a format character.
func nonJoining(r rune) bool {
	if r >= 0x0600 {
		return false
	}
	switch ucd.LookupGeneralCategory(r) {
	case ucd.Mn, ucd.Me, ucd.Cf:
		return false
	}
	return true
}

// zeroed returns s with n items set to zero, in the memory of s when it
// has the capacity.
func zeroed[T any](s []T, n int) []T {
	if cap(s) < n {
		return make([]T, n)
	}
	s = s[:n]
	clear(s)
	return s
}
