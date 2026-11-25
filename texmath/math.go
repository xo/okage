// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"math"
	"slices"

	"github.com/xo/okage"
	"github.com/xo/okage/typesetting/font"
)

// This file is the layout half of math.go of the source: the styles, the
// space between atoms, the boxes, the scripts, the limits, the fractions,
// the radicals, the delimiters that grow, the accents, the bars, the arrows
// and the grid of the matrices. The source writes SVG text, and this file
// makes the items of item.go. The writer of SVG of the source, document and
// documentTight, is not here: Box.Path and Box.ItemPath of item.go take its
// place.
//
// okage: every length is in exact font units, scaled by the size, which is a
// float64. The source asks go-opentype for a face at a whole number of
// pixels, and go-opentype rounds each value to a whole pixel (D78 section
// 6). Where the source multiplies and adds, this file writes
// float64(x*y) + z, so that the compiler does not fuse the two (D16). The
// compiler turns x/2 into x*0.5, so this file also writes float64(x/2) + z
// (D71). A scan of the assembly for arm64, s390x, ppc64le, riscv64 and
// loong64 finds no fused instruction in texmath.

// ── style ───────────────────────────────────────────────────────────────────

// scriptOf returns the style of a script of the style s: script style for
// display and text style, scriptscript style for the others. It is
// style.script of the source.
//
// okage: TeX has three sizes, and scriptscript is the smallest. The source
// sets a script at the script size of the size in force, so a script of a
// script is 70% of a script, and each level is smaller again
// (math.go:163).
func scriptOf(s Style) Style {
	if s <= Text {
		return Script
	}
	return ScriptScript
}

// fracInner returns the style of the numerator and the denominator of a
// fraction in the style s. TeX moves them one style down (Appendix G, rule
// 15b): a fraction in display style sets its parts in text style, at the same
// size, and a fraction in text style sets them in script style.
func fracInner(s Style) Style {
	switch s {
	case Display:
		return Text
	case Text:
		return Script
	default:
		return ScriptScript
	}
}

// cellStyle returns the style of a cell of a grid in the style s. It is
// style.inner of the source, which keeps the size and drops display style.
func cellStyle(s Style) Style {
	if s == Display {
		return Text
	}
	return s
}

// px returns the size of the style s: the size of the layout for display and
// text style, and that size scaled by ScriptPercentScaleDown or
// ScriptScriptPercentScaleDown for the scripts. It is e.scriptSize and
// e.scriptScriptSize of the source, with the size of the layout as their
// argument.
//
// okage: the source truncates the size of a script to a whole pixel, and
// uses the size in force, not the text size (math.go:277 and math.go:290).
func (l *layout) px(s Style) float64 {
	switch s {
	case Script:
		return float64(l.size*float64(l.face.MathConstant(font.ScriptPercentScaleDown))) / 100
	case ScriptScript:
		return float64(l.size*float64(l.face.MathConstant(font.ScriptScriptPercentScaleDown))) / 100
	}
	return l.size
}

// ── atom classes & boxes ────────────────────────────────────────────────────

// box is a fragment that is laid out. Its origin is the left end of its
// baseline, and y grows down. h is the extent above the baseline, d the depth
// below it and w the advance width, in the unit of the size. cls is the class
// of the atom, for the space between atoms. char reports a box of one glyph
// of a character, which is not an extended shape of the MATH table: the
// scripts of such a box do not take the drops of their baselines.
//
// okage: ic is the italic correction of the last glyph of the box, from the
// MATH table. glyph reports a box of one glyph of a character, or its
// variant or assembly, whose italic correction moves its scripts, as TeX
// Appendix G, rules 13a and 18, says. gid and gpx are that glyph and its size,
// when the box is one glyph, for the top accent attachment and the math
// kerning: kern reports that they are set, and gy is the baseline of the
// glyph over the baseline of the box. A list of one atom keeps them.
type box struct {
	w, h, d float64
	cls     Class
	char    bool
	items   []Item

	ic    float64
	glyph bool
	kern  bool
	gid   font.GID
	gpx   float64
	gy    float64
}

func newBox(cls Class) *box { return &box{cls: cls} }

// place moves the items of src into dst, translated by (dx, dy). It is place
// of the source, which wrote a <g transform="translate">. src gives up its
// items, so a box is placed once.
func place(dst, src *box, dx, dy float64) {
	if len(src.items) == 0 {
		return
	}
	translateItems(src.items, dx, dy)
	if dst.items == nil {
		dst.items = src.items
	} else {
		dst.items = append(dst.items, src.items...)
	}
	src.items = nil
}

// translateItems moves items by (dx, dy). An item with a transform keeps its
// X and Y, and the translation goes after its transform.
func translateItems(items []Item, dx, dy float64) {
	if dx == 0 && dy == 0 {
		return
	}
	for i := range items {
		it := &items[i]
		if it.Transform.IsIdentity() {
			it.X += dx
			it.Y += dy
		} else {
			it.Transform = it.Transform.PostTranslate(float32(dx), float32(dy))
		}
	}
}

// rule adds a filled rectangle (x, y, w, h) to dst. It is rule of the
// source, which wrote a <rect>.
func (l *layout) rule(dst *box, x, y, w, h float64) {
	if !l.count() {
		return
	}
	dst.items = append(dst.items, Item{Kind: ItemRule, X: x, Y: y, W: w, H: h, Color: l.color, Transform: okage.IdentityTransform()})
}

// path adds the filled path p to dst. The source writes an SVG path for the
// head of an arrow and for the hook of \xhookrightarrow.
func (l *layout) path(dst *box, p *okage.Path) {
	if p == nil || !l.count() {
		return
	}
	dst.items = append(dst.items, Item{Kind: ItemPath, Path: p, Color: l.color, Transform: okage.IdentityTransform()})
}

// ── engine ──────────────────────────────────────────────────────────────────

// scale returns the units of the size for each font unit at the size px.
func (l *layout) scale(px float64) float64 { return px / l.upem }

// mc reads a MATH constant, scaled to the size px.
func (l *layout) mc(which font.MathConstant, px float64) float64 {
	return float64(float64(l.face.MathConstant(which)) * l.scale(px))
}

func (l *layout) axis(px float64) float64 { return l.mc(font.AxisHeight, px) }

// glyphBox lays out the character r as a box of the class cls. It returns
// false when the face has no glyph for r.
func (l *layout) glyphBox(r rune, px float64, cls Class) (*box, bool) {
	gid, ok := l.face.NominalGlyph(r)
	if !ok {
		return nil, false
	}
	return l.gidBox(gid, px, cls, string(r)), true
}

// gidBox makes a box for the glyph gid. text is the character of the glyph,
// for a text layer, and empty for a part of an assembly after the first.
func (l *layout) gidBox(gid font.GID, px float64, cls Class, text string) *box {
	h, dp := l.gidVExtent(gid, px)
	b := &box{w: float64(float64(l.face.HorizontalAdvance(gid)) * l.scale(px)), h: h, d: dp, cls: cls,
		glyph: true, kern: true, gid: gid, gpx: px}
	if !l.cfg.noItalic {
		b.ic = float64(float64(l.face.MathItalicsCorrection(gid)) * l.scale(px))
	}
	if l.count() {
		b.items = []Item{{Kind: ItemGlyph, Face: l.face, Glyph: gid, Size: px, Text: text, Color: l.color, Transform: okage.IdentityTransform()}}
	}
	return b
}

// gidVExtent returns the extent of the ink of a glyph above (h) and below (d)
// the baseline. Both are at least zero: a box holds its own baseline. gidInk
// gives where the ink is.
func (l *layout) gidVExtent(gid font.GID, px float64) (h, d float64) {
	top, bot := l.gidInk(gid, px)
	return max(top, 0), -min(bot, 0)
}

// gidInk returns the highest and the lowest ink of a glyph above the
// baseline, with y up, neither clamped to zero. For a spacing accent both are
// positive.
//
// okage: the ink is the bounds of the curves of the outline, from the
// extents of typesetting. go-opentype stores a CFF outline as lines, and the
// source takes the ink from their points.
func (l *layout) gidInk(gid font.GID, px float64) (top, bot float64) {
	e, ok := l.face.GlyphExtents(gid)
	if !ok {
		return 0, 0
	}
	s := l.scale(px)
	return float64(float64(e.YBearing) * s), float64((float64(e.YBearing) + float64(e.Height)) * s)
}

// thin, med and thick are the spaces of TeX in mu: \thinmuskip,
// \medmuskip and \thickmuskip, the values of plain.tex (tex.web §764).
const (
	thin  = 3
	med   = 4
	thick = 5
)

// spaceTable is the table of TeX Appendix G, in mu. tight marks the entries
// that are not in parentheses in the table of the TeXbook, page 170: the
// only spaces of script and scriptscript style.
var spaceTable = [8][8]uint8{
	Ord:   {Op: thin, Bin: med, Rel: thick, Inner: thin},
	Op:    {Ord: thin, Op: thin, Rel: thick, Inner: thin},
	Bin:   {Ord: med, Op: med, Open: med, Inner: med},
	Rel:   {Ord: thick, Op: thick, Open: thick, Inner: thick},
	Close: {Op: thin, Bin: med, Rel: thick, Inner: thin},
	Punct: {Ord: thin, Op: thin, Rel: thin, Open: thin, Close: thin, Punct: thin, Inner: thin},
	Inner: {Ord: thin, Op: thin, Bin: med, Rel: thick, Open: thin, Punct: thin, Inner: thin},
}

var tightTable = [8][8]bool{
	Ord:   {Op: true},
	Op:    {Ord: true, Op: true},
	Close: {Op: true},
	Inner: {Op: true},
}

// interAtom returns the space between two atoms of the classes l and r, in
// the style s, at the size px. A mu is px/18.
//
// okage: in script and scriptscript style, TeX keeps only the thin spaces
// next to an Op. The source drops every space in a script (math.go:163), and
// keeps every space in a fraction in text style, whose parts are at the
// script size (math.go:191).
func interAtom(l, r Class, s Style, px float64) float64 {
	if l > Inner || r > Inner {
		return 0
	}
	if s >= Script && !tightTable[l][r] {
		return 0
	}
	return float64(float64(spaceTable[l][r]) * (px / 18))
}

// entry is one item of an hlist. A space is not an atom: it takes no part in
// the classes, as a kern or glue of TeX.
type entry struct {
	b    *box
	st   Style // the style of the item, for the space before it
	atom bool
}

// hlist sets the entries left to right on one baseline, with the space
// between the atoms, and sets a Bin as an Ord where a binary operator cannot
// stand.
//
// okage: as in TeX (Appendix G, rules 5 and 6, and tex.web §729), a Bin is
// an Ord also before a Rel, a Close or a Punct, and at the end of the list.
// The source sets it as an Ord only at the start and after a Bin, Op, Rel,
// Open or Punct (math.go:443). A space between two atoms is not an atom. The
// source makes each space an Ord atom, so the − of a=\,-b stayed a Bin, and
// \int\!\int took the spaces of an Ord (parse.go:1001 and mukern.go:33).
//
//go:noinline
func (l *layout) hlist(items []entry) *box {
	out := newBox(Ord)
	if len(items) == 0 {
		return out
	}
	var prev *box // the previous atom
	for _, it := range items {
		if !it.atom {
			continue
		}
		b := it.b
		switch b.cls {
		case Bin:
			if prev == nil {
				b.cls = Ord
			} else {
				switch prev.cls {
				case Bin, Op, Rel, Open, Punct:
					b.cls = Ord
				}
			}
		case Rel, Close, Punct:
			if prev != nil && prev.cls == Bin {
				prev.cls = Ord
			}
		}
		prev = b
	}
	if prev != nil && prev.cls == Bin {
		prev.cls = Ord
	}
	x := 0.0
	prevCls, hasPrev := Ord, false
	for i, it := range items {
		if it.atom {
			if hasPrev {
				x += interAtom(prevCls, it.b.cls, it.st, l.px(it.st))
			}
			prevCls, hasPrev = it.b.cls, true
		}
		place(out, it.b, x, 0)
		x += it.b.w
		out.h = max(out.h, it.b.h)
		out.d = max(out.d, it.b.d)
		// okage: the italic correction of a character follows it, as the
		// kern of TeX Appendix G, rule 17, and the margin of KaTeX. The
		// italic correction of the last atom is that of the box.
		if it.atom && it.b.glyph {
			x += it.b.ic
		}
		if it.atom && i == len(items)-1 {
			out.ic = it.b.ic
		}
	}
	out.w = x
	if len(items) == 1 && items[0].atom {
		b := items[0].b
		out.cls = b.cls
		out.kern, out.gid, out.gpx, out.gy = b.kern, b.gid, b.gpx, b.gy
	}
	return out
}

// attachScripts sets a superscript and a subscript on a nucleus, by TeX
// Appendix G, rule 18, with the constants of the MATH table, as the OpenType
// specification gives them. SpaceAfterScript, \scriptspace of TeX, follows
// the scripts.
//
// okage: the source places the superscript at the larger of
// SuperscriptShiftUp and the height of the nucleus less
// SuperscriptBaselineDropMax, and the subscript at the larger of
// SubscriptShiftDown and the depth of the nucleus, each on its own
// (math.go:475 and math.go:485). It leaves out SuperscriptBottomMin,
// SubscriptBaselineDropMin, SubscriptTopMax, SubSuperscriptGapMin,
// SuperscriptBottomMaxWithSubscript and SpaceAfterScript, and it takes the
// drops also under one character, so a deep
// superscript or a tall subscript ran into the nucleus, and a superscript
// and a subscript could touch.
//
// okage: the superscript of a nucleus of one glyph is the italic correction
// of the glyph to the right of the subscript, as TeX Appendix G, rule 18,
// and the OpenType specification say. The math kerning of the two glyphs
// that face each other then moves each script (mathKernSup and
// mathKernSub). Both are additions of phase 26 of D78.
//
//go:noinline
func (l *layout) attachScripts(nuc, sup, sub *box, st Style) *box {
	px := l.px(st)
	out := newBox(nuc.cls)
	out.w, out.h, out.d = nuc.w, nuc.h, nuc.d
	x := nuc.w
	supX, subX := x, x
	if nuc.glyph {
		supX += nuc.ic
	}
	// The drops of the baselines apply to a nucleus that is a box, and not
	// to one glyph of a character (Appendix G, rule 18a, and the OpenType
	// specification of SuperscriptBaselineDropMax).
	dropUp, dropDown := nuc.h-l.mc(font.SuperscriptBaselineDropMax, px), nuc.d+l.mc(font.SubscriptBaselineDropMin, px)
	if nuc.char {
		dropUp, dropDown = 0, 0
	}
	var u, v float64 // the shift of the superscript up and of the subscript down
	if sup != nil {
		u = max(l.mc(font.SuperscriptShiftUp, px), dropUp, sup.d+l.mc(font.SuperscriptBottomMin, px))
	}
	if sub != nil {
		v = max(l.mc(font.SubscriptShiftDown, px), dropDown)
		if sup == nil {
			v = max(v, sub.h-l.mc(font.SubscriptTopMax, px))
		}
	}
	if sup != nil && sub != nil {
		// The gap between the bottom of the superscript and the top of the
		// subscript is at least SubSuperscriptGapMin. The subscript goes
		// down, and then both go up while the bottom of the superscript is
		// below SuperscriptBottomMaxWithSubscript.
		if gap := (u - sup.d) - (sub.h - v); gap < l.mc(font.SubSuperscriptGapMin, px) {
			v += l.mc(font.SubSuperscriptGapMin, px) - gap
			if psi := l.mc(font.SuperscriptBottomMaxWithSubscript, px) - (u - sup.d); psi > 0 {
				u += psi
				v -= psi
			}
		}
	}
	space := l.mc(font.SpaceAfterScript, px)
	place(out, nuc, 0, 0)
	// okage: a subscript whose top is over the baseline counts in the
	// height, and a superscript whose bottom is under it in the depth. The
	// source counts only the height of the superscript and the depth of the
	// subscript (math.go:477 and math.go:487), so ^1 had no depth and _2 no
	// height.
	if sup != nil {
		supX += l.mathKernSup(nuc, sup, u)
		out.h = max(out.h, u+sup.h)
		out.d = max(out.d, sup.d-u)
		out.w = max(out.w, supX+sup.w+space)
		place(out, sup, supX, -u)
	}
	if sub != nil {
		subX += l.mathKernSub(nuc, sub, v)
		out.d = max(out.d, v+sub.d)
		out.h = max(out.h, sub.h-v)
		out.w = max(out.w, subX+sub.w+space)
		place(out, sub, subX, v)
	}
	return out
}

// mathKernSup returns the math kerning of the superscript sup, shifted up by
// u, on the nucleus nuc, or 0 when nuc is not one glyph. The kern of the top
// right corner of the nucleus and of the bottom left corner of the
// superscript are added at two heights: the top of the nucleus and the bottom
// of the superscript. The smaller sum is the kern, as LuaTeX computes it from
// the OpenType specification. A superscript that is not one glyph adds no
// kern of its own.
func (l *layout) mathKernSup(nuc, sup *box, u float64) float64 {
	if l.cfg.noKern || !nuc.kern {
		return 0
	}
	k := math.Inf(1)
	for _, h := range [2]float64{nuc.h, u - sup.d} {
		k = min(k, l.glyphKern(nuc, font.MathKernTopRight, h)+l.glyphKern(sup, font.MathKernBottomLeft, h-u))
	}
	return k
}

// mathKernSub returns the math kerning of the subscript sub, shifted down by
// v, on the nucleus nuc, as mathKernSup does, with the bottom right corner of
// the nucleus and the top left corner of the subscript, at the top of the
// subscript and the bottom of the nucleus.
func (l *layout) mathKernSub(nuc, sub *box, v float64) float64 {
	if l.cfg.noKern || !nuc.kern {
		return 0
	}
	k := math.Inf(1)
	for _, h := range [2]float64{sub.h - v, -nuc.d} {
		k = min(k, l.glyphKern(nuc, font.MathKernBottomRight, h)+l.glyphKern(sub, font.MathKernTopLeft, h+v))
	}
	return k
}

// glyphKern returns the math kern of the corner c of the glyph of b at the
// height h over its baseline, in the unit of the size, or 0 when b is not one
// glyph.
func (l *layout) glyphKern(b *box, c font.MathKernCorner, h float64) float64 {
	if !b.kern {
		return 0
	}
	s := l.scale(b.gpx)
	return float64(float64(l.face.MathKern(b.gid, c, float32((h-b.gy)/s))) * s)
}

// attachLimits sets an upper and a lower limit, centered, over and under a
// large operator, by TeX Appendix G, rule 13a, with the constants of the
// MATH table.
//
// okage: the baseline of a limit is at least UpperLimitBaselineRiseMin over
// the operator, or LowerLimitBaselineDropMin under it. The source takes only
// the gaps, UpperLimitGapMin and LowerLimitGapMin (math.go:512 and
// math.go:517).
//
//go:noinline
func (l *layout) attachLimits(op, over, under *box, st Style) *box {
	px := l.px(st)
	w := op.w
	if over != nil {
		w = max(w, over.w)
	}
	if under != nil {
		w = max(w, under.w)
	}
	out := newBox(Op)
	out.w = w
	out.h, out.d = op.h, op.d
	// okage: the upper limit of an operator of one glyph is half of its
	// italic correction to the right, and the lower limit half of it to the
	// left, as TeX Appendix G, rule 13a, and the OpenType specification say.
	// This is an addition of phase 26 of D78.
	shift := 0.0
	if op.glyph {
		shift = op.ic / 2
	}
	place(out, op, (w-op.w)/2, 0)
	if over != nil {
		base := op.h + max(l.mc(font.UpperLimitBaselineRiseMin, px), l.mc(font.UpperLimitGapMin, px)+over.d)
		out.h = base + over.h
		place(out, over, float64((w-over.w)/2)+shift, -base)
	}
	if under != nil {
		base := op.d + max(l.mc(font.LowerLimitBaselineDropMin, px), l.mc(font.LowerLimitGapMin, px)+under.h)
		out.d = base + under.d
		place(out, under, float64((w-under.w)/2)-shift, base)
	}
	return out
}

// fraction stacks num over den with a rule on the math axis, by TeX Appendix
// G, rule 15, with the constants of the MATH table.
//
// pad is \nulldelimiterspace, 1.2pt at 10pt, so 0.12 em. TeX puts the two
// delimiters of the noad beside the fraction, and \frac names none: each is
// an empty box of that width (tex.web:13930), outside the vlist that holds
// the rule. So the rule spans the wider of the numerator and the
// denominator, and not the padding.
//
// okage: the baseline of the numerator is at least
// FractionNumeratorShiftUp over the baseline, and the baseline of the
// denominator at least FractionDenominatorShiftDown under it, with the
// constants of display style in display style. The source takes only the
// gaps (math.go:553 and math.go:555), so a fraction was too flat. align
// sets a numerator that is narrower than den at one side of it, for
// \cfrac[l] and \cfrac[r] (D95).
//
//go:noinline
func (l *layout) fraction(num, den *box, st Style, align Align) *box {
	if align != AlignCenter {
		num = alignNumerator(num, den, align)
	}
	px := l.px(st)
	axis := l.axis(px)
	rt := l.mc(font.FractionRuleThickness, px)
	numShift := l.mc(font.FractionNumeratorShiftUp, px)
	denShift := l.mc(font.FractionDenominatorShiftDown, px)
	gapN := l.mc(font.FractionNumeratorGapMin, px)
	gapD := l.mc(font.FractionDenominatorGapMin, px)
	if st == Display {
		numShift = l.mc(font.FractionNumeratorDisplayStyleShiftUp, px)
		denShift = l.mc(font.FractionDenominatorDisplayStyleShiftDown, px)
		gapN = l.mc(font.FractionNumDisplayStyleGapMin, px)
		gapD = l.mc(font.FractionDenomDisplayStyleGapMin, px)
	}
	pad := float64(px * 0.12)
	inner := max(num.w, den.w)
	w := inner + float64(2*pad)
	out := newBox(Inner)
	out.w = w
	numBase := -max(numShift, axis+float64(rt/2)+gapN+num.d)
	denBase := max(denShift, -axis+float64(rt/2)+gapD+den.h)
	out.h = -numBase + num.h
	out.d = denBase + den.d
	place(out, num, (w-num.w)/2, numBase)
	place(out, den, (w-den.w)/2, denBase)
	l.rule(out, pad, -axis-float64(rt/2), inner, rt)
	return out
}

// binom stacks num over den without a rule: the body of \binom, which the
// caller puts in parentheses, and of \atop. It is the stack of TeX Appendix
// G, rule 15, with StackTopShiftUp, StackBottomShiftDown and StackGapMin of
// the MATH table: when the gap between the two is below the minimum, each
// moves away by half of what is missing.
//
// okage: the source takes the gaps of a fraction, each widened by the
// thickness of the rule (math.go:570), and none of the constants of a
// stack.
//
//go:noinline
func (l *layout) binom(num, den *box, st Style) *box {
	px := l.px(st)
	up := l.mc(font.StackTopShiftUp, px)
	down := l.mc(font.StackBottomShiftDown, px)
	gapMin := l.mc(font.StackGapMin, px)
	if st == Display {
		up = l.mc(font.StackTopDisplayStyleShiftUp, px)
		down = l.mc(font.StackBottomDisplayStyleShiftDown, px)
		gapMin = l.mc(font.StackDisplayStyleGapMin, px)
	}
	if gap := (up - num.d) - (den.h - down); gap < gapMin {
		up += float64((gapMin - gap) / 2)
		down += float64((gapMin - gap) / 2)
	}
	pad := float64(px * 0.12)
	w := max(num.w, den.w) + float64(2*pad)
	out := newBox(Inner)
	out.w = w
	out.h = up + num.h
	out.d = down + den.d
	place(out, num, (w-num.w)/2, -up)
	place(out, den, (w-den.w)/2, down)
	return out
}

// xArrow builds an arrow that grows to the width of its labels, such as
// \xrightarrow: the label over the shaft and the optional label under it,
// both in script style. leftHead and rightHead choose the heads, hooked adds
// a hook at the tail, and mapsto a bar at the tail. The arrow is centered on
// the math axis, and its class is Rel. sup and sub can be nil.
//
// okage: the shaft, the heads and the axis are those of the style in force.
// The source draws them at the size and on the axis of the script style of
// the labels (parse.go:581), so the arrow sat below the math axis. The hook
// is a filled outline, so that Box.Path holds every item. The source strokes
// an SVG arc.
//
//go:noinline
func (l *layout) xArrow(sup, sub *box, leftHead, rightHead, hooked, mapsto bool, st Style) *box {
	px := l.px(st)
	axisY := -l.axis(px)
	rt := max(l.mc(font.FractionRuleThickness, px), float64(px*0.045))
	headLen := float64(px * 0.42)
	headHW := float64(px * 0.24)
	pad := float64(px * 0.30)
	minLen := float64(px * 1.4)

	labelW := 0.0
	if sup != nil {
		labelW = max(labelW, sup.w)
	}
	if sub != nil {
		labelW = max(labelW, sub.w)
	}
	w := max(labelW+float64(2*pad), minLen)

	out := newBox(Rel)
	out.w = w

	// The shaft, cut at each end that carries a head.
	x0, x1 := 0.0, w
	if leftHead || mapsto {
		x0 = headLen
	}
	if rightHead {
		x1 = w - headLen
	}
	l.rule(out, x0, axisY-float64(rt/2), x1-x0, rt)
	if rightHead {
		l.path(out, arrowHead(w, w-headLen, axisY, headHW))
	}
	if leftHead {
		l.path(out, arrowHead(0, headLen, axisY, headHW))
	}
	if mapsto {
		// \xmapsto: a vertical bar at the left end.
		l.rule(out, 0, axisY-headHW, rt, float64(2*headHW))
	}
	if hooked {
		// A small half circle at the tail, the end without the head.
		hr := float64(headHW * 0.85)
		if rightHead {
			l.path(out, hook(headLen, axisY, hr, rt, true))
		} else {
			l.path(out, hook(w-headLen, axisY, hr, rt, false))
		}
	}

	arrowTopY := axisY - headHW
	arrowBotY := axisY + headHW
	out.h = -arrowTopY
	out.d = arrowBotY
	if hooked {
		// okage: the hook hangs below the shaft, down to the axis plus its
		// diameter and half its thickness, and the depth holds it. The source
		// leaves it out (math.go:660).
		out.d = max(out.d, axisY+float64(2*float64(headHW*0.85))+float64(rt/2))
	}
	gap := float64(px * 0.12)
	if sup != nil {
		by := arrowTopY - gap - sup.d
		out.h = max(out.h, -by+sup.h)
		place(out, sup, (w-sup.w)/2, by)
	}
	if sub != nil {
		by := arrowBotY + gap + sub.h
		out.d = max(out.d, by+sub.d)
		place(out, sub, (w-sub.w)/2, by)
	}
	out.d = max(out.d, 0)
	return out
}

// arrowHead returns the triangle of a head with its tip at (tipX, y), its
// back at backX and a half width hw. The points go clockwise on the screen,
// as the rectangle of a rule does, so that the head and the shaft that it
// covers add up in a path that is filled with the nonzero rule.
func arrowHead(tipX, backX, y, hw float64) *okage.Path {
	pb := okage.NewPathBuilder()
	pb.MoveTo(float32(tipX), float32(y))
	if tipX > backX {
		pb.LineTo(float32(backX), float32(y+hw))
		pb.LineTo(float32(backX), float32(y-hw))
	} else {
		pb.LineTo(float32(backX), float32(y-hw))
		pb.LineTo(float32(backX), float32(y+hw))
	}
	pb.Close()
	p, _ := pb.Finish()
	return p
}

// hook returns the outline of the half circle of radius r that the source
// strokes with the width rt, from (x, y) down to (x, y+2r). left is the arc
// that bulges to the left, the SVG arc with the sweep flag 0.
func hook(x, y, r, rt float64, left bool) *okage.Path {
	cy := y + r
	ro, ri := r+float64(rt/2), max(r-float64(rt/2), 0)
	dir := 1.0
	if left {
		dir = -1
	}
	pb := okage.NewPathBuilder()
	// The outer arc from the top to the bottom, then the inner arc back.
	pb.MoveTo(float32(x), float32(cy-ro))
	halfCircle(pb, x, cy, ro, dir, 1)
	pb.LineTo(float32(x), float32(cy+ri))
	halfCircle(pb, x, cy, ri, dir, -1)
	pb.Close()
	p, _ := pb.Finish()
	return p
}

// halfCircle adds two cubic curves for the half circle of center (cx, cy)
// and radius r, from the top to the bottom when down is 1, and back when it
// is -1. side is -1 for the half on the left and 1 for the half on the
// right.
func halfCircle(pb *okage.PathBuilder, cx, cy, r, side, down float64) {
	// k is the distance of the control points of a quarter circle.
	const k = 0.5522847498307936
	kr := float64(k * r)
	sy := float64(down * r)
	skr := float64(down * kr)
	xk, xr := cx+float64(side*kr), cx+float64(side*r)
	pb.CubicTo(float32(xk), float32(cy-sy), float32(xr), float32(cy-skr), float32(xr), float32(cy))
	pb.CubicTo(float32(xr), float32(cy+skr), float32(xk), float32(cy+sy), float32(cx), float32(cy+sy))
}

// xArrowSpec returns the heads and the ornaments of the arrow that an
// extensible arrow draws, by the character of XArrow.Arrow, and false for
// another character. The source chooses them by the name of the command.
func xArrowSpec(arrow rune) (leftHead, rightHead, hooked, mapsto, ok bool) {
	switch arrow {
	case '→', '⇒', '⇀', '⇁':
		return false, true, false, false, true
	case '←', '⇐', '↼', '↽':
		return true, false, false, false, true
	case '↔', '⇔':
		return true, true, false, false, true
	case '↪':
		return false, true, true, false, true
	case '↩':
		return true, false, true, false, true
	case '↦':
		return false, true, false, true, true
	}
	return false, false, false, false, false
}

// overUnder centers a small box over or under a base, as \overset,
// \underset and \stackrel do.
//
//go:noinline
func (l *layout) overUnder(base, extra *box, over bool, st Style, cls Class) *box {
	out := newBox(cls)
	out.w = max(base.w, extra.w)
	gap := float64(l.px(st) * 0.1)
	if over {
		ty := -(base.h + gap + extra.d)
		out.h = -ty + extra.h
		out.d = base.d
		place(out, base, (out.w-base.w)/2, 0)
		place(out, extra, (out.w-extra.w)/2, ty)
	} else {
		ty := base.d + gap + extra.h
		out.h = base.h
		out.d = ty + extra.d
		place(out, base, (out.w-base.w)/2, 0)
		place(out, extra, (out.w-extra.w)/2, ty)
	}
	return out
}

// boxed draws a frame around b, for \boxed. n gives the background and the
// color of the frame of \colorbox and \fcolorbox, and \colorbox has no frame.
// The background is a rule behind b, of the size of the frame.
//
//go:noinline
func (l *layout) boxed(b *box, st Style, n *Boxed) *box {
	px := l.px(st)
	pad := float64(px * 0.15)
	rt := max(l.mc(font.FractionRuleThickness, px), float64(px*0.05))
	out := newBox(Ord)
	out.w = b.w + float64(2*pad)
	top, bot := -(b.h + pad), b.d+pad
	out.h, out.d = b.h+pad, b.d+pad
	saved := l.color
	if n.Background != nil {
		l.color = *n.Background
		l.rule(out, 0, top, out.w, bot-top)
		l.color = saved
	}
	place(out, b, pad, 0)
	if n.NoFrame {
		return out
	}
	if n.Border != nil {
		l.color = *n.Border
	}
	l.rule(out, 0, top, out.w, rt)          // top
	l.rule(out, 0, bot-rt, out.w, rt)       // bottom
	l.rule(out, 0, top, rt, bot-top)        // left
	l.rule(out, out.w-rt, top, rt, bot-top) // right
	l.color = saved
	return out
}

// brace sets a horizontal curly brace over or under b, for \overbrace and
// \underbrace: U+23DE or U+23DF, grown to the width of b with the horizontal
// variants and the horizontal assembly of the MATH table. The ink of the brace
// is 0.1 em from b, as KaTeX sets it, and the box goes 0.3 em past the ink,
// as the \kern3\p@ of \overbrace and \underbrace in plain.tex. The box is an
// Op, which takes its scripts as limits.
//
// okage: the source draws a rule in place of the brace. A face without the
// glyph still gets the rule. r is the character of the brace, such as the
// square bracket of \overbracket, or 0 for the curly brace (D95).
//
//go:noinline
func (l *layout) brace(b *box, r rune, over bool, st Style) *box {
	px := l.px(st)
	if r == 0 {
		r = '⏟'
		if over {
			r = '⏞'
		}
	}
	if _, ok := l.face.NominalGlyph(r); !ok {
		var out *box
		if over {
			out = l.overline(b, st)
		} else {
			out = l.underline(b, st)
		}
		out.cls = Op
		return out
	}
	return l.stretchOver(b, r, over, float64(px*0.1), float64(px*0.3), Op, px)
}

// stretchOver sets the character r over or under b, grown to the width of b
// with the horizontal variants and the horizontal assembly of the MATH
// table. The ink of r is gap from b, and the box goes beyond past the ink.
// The box has the class cls. brace calls it for the braces, and
// layoutStretch for \overgroup and its kin (D95).
func (l *layout) stretchOver(b *box, r rune, over bool, gap, beyond float64, cls Class, px float64) *box {
	br := l.stretchHorizontal(r, b.w, px, Ord)
	// The ink of the brace, from its items, with y up.
	top, bot := math.Inf(-1), math.Inf(1)
	for _, it := range br.items {
		t, d := l.gidInk(it.Glyph, px)
		top, bot = max(top, t-it.Y), min(bot, d-it.Y)
	}
	if len(br.items) == 0 {
		top, bot = 0, 0
	}
	out := newBox(cls)
	out.w = max(b.w, br.w)
	bw, brw := b.w, br.w
	out.h, out.d = b.h, b.d
	place(out, b, (out.w-bw)/2, 0)
	if over {
		dy := -(b.h + gap - bot) // the bottom of the ink gap over b
		out.h = b.h + gap + (top - bot) + beyond
		place(out, br, (out.w-brw)/2, dy)
	} else {
		dy := b.d + gap + top // the top of the ink gap under b
		out.d = b.d + gap + (top - bot) + beyond
		place(out, br, (out.w-brw)/2, dy)
	}
	return out
}

// attachBraceLabels sets the labels of \overbrace and \underbrace, a
// superscript and a subscript, centered over and under the box of the brace,
// 0.2 em from it, as KaTeX sets them. They are the limits of an Op in each
// style, as \mathop with \limits in plain.tex. This is an addition of phase
// 26 of D78.
//
//go:noinline
func (l *layout) attachBraceLabels(br, over, under *box, st Style) *box {
	gap := float64(l.px(st) * 0.2)
	w := br.w
	if over != nil {
		w = max(w, over.w)
	}
	if under != nil {
		w = max(w, under.w)
	}
	out := newBox(Op)
	out.w = w
	out.h, out.d = br.h, br.d
	bw := br.w
	place(out, br, (w-bw)/2, 0)
	if over != nil {
		base := out.h + gap + over.d
		out.h = base + over.h
		place(out, over, (w-over.w)/2, -base)
	}
	if under != nil {
		base := out.d + gap + under.h
		out.d = base + under.d
		place(out, under, (w-under.w)/2, base)
	}
	return out
}

// phantom keeps the size of b and draws nothing: the width, the height and
// the depth for \phantom, the width for \hphantom (vertical false), and the
// height and the depth for \vphantom (horizontal false).
func (l *layout) phantom(b *box, horizontal, vertical bool) *box {
	out := newBox(Ord)
	if horizontal {
		out.w = b.w
	}
	if vertical {
		out.h, out.d = b.h, b.d
	}
	return out
}

// negate sets a slash over b, centered on its height and its depth, for
// \not. The box keeps the class of b.
//
//go:noinline
func (l *layout) negate(b *box, st Style) *box {
	out := newBox(b.cls)
	out.w, out.h, out.d = b.w, b.h, b.d
	sl := l.mustGlyph('/', l.px(st), Ord)
	dx := (b.w - sl.w) / 2
	dy := float64((b.d-b.h)/2) - float64((sl.d-sl.h)/2) // the middle of the slash on the middle of the atom
	place(out, b, 0, 0)
	out.h = max(out.h, -dy+sl.h)
	out.d = max(out.d, dy+sl.d)
	place(out, sl, dx, dy)
	return out
}

// overline draws a rule over b.
//
//go:noinline
func (l *layout) overline(b *box, st Style) *box {
	px := l.px(st)
	gap := l.mc(font.OverbarVerticalGap, px)
	rt := l.mc(font.OverbarRuleThickness, px)
	asc := l.mc(font.OverbarExtraAscender, px)
	out := newBox(Ord)
	out.w, out.d = b.w, b.d
	top := b.h + gap
	out.h = top + rt + asc
	place(out, b, 0, 0)
	l.rule(out, 0, -(top + rt), out.w, rt)
	return out
}

// underline draws a rule under b.
//
//go:noinline
func (l *layout) underline(b *box, st Style) *box {
	px := l.px(st)
	gap := l.mc(font.UnderbarVerticalGap, px)
	rt := l.mc(font.UnderbarRuleThickness, px)
	desc := l.mc(font.UnderbarExtraDescender, px)
	out := newBox(Ord)
	out.w, out.h = b.w, b.h
	bot := b.d + gap
	out.d = bot + rt + desc
	place(out, b, 0, 0)
	l.rule(out, 0, bot, out.w, rt)
	return out
}

// overArrow sets an arrow over b that spans its width: \overrightarrow,
// \overleftarrow and \overleftrightarrow. The shaft runs the whole width,
// and each end that is asked for carries a head over it.
//
//go:noinline
func (l *layout) overArrow(b *box, left, right bool, st Style) *box {
	px := l.px(st)
	gap := l.mc(font.OverbarVerticalGap, px)
	rt := max(l.mc(font.OverbarRuleThickness, px), float64(px*0.045))
	asc := l.mc(font.OverbarExtraAscender, px)
	headLen := float64(px * 0.42)
	headHW := float64(px * 0.24)
	out := newBox(Ord)
	out.w, out.d = b.w, b.d
	top := b.h + gap
	y := -(top + headHW) // the center line of the arrow, clear of the nucleus
	out.h = top + float64(2*headHW) + asc
	place(out, b, 0, 0)
	l.rule(out, 0, y-float64(rt/2), out.w, rt)
	if right {
		l.path(out, arrowHead(out.w, out.w-headLen, y, headHW))
	}
	if left {
		l.path(out, arrowHead(0, headLen, y, headHW))
	}
	return out
}

// combiningAccents maps the spacing accent of each accent command to its
// combining mark, which carries the top accent attachment and the horizontal
// variants of the MATH table.
var combiningAccents = map[rune]rune{
	'`': 0x300, '´': 0x301, 'ˆ': 0x302, '˜': 0x303, '¯': 0x304, '˘': 0x306,
	'˙': 0x307, '¨': 0x308, '˚': 0x30A, 'ˇ': 0x30C,
}

// accent sets the accent acc over the nucleus. The bottom of the ink of the
// accent sits a small gap over the nucleus. stretch grows the accent to the
// width of the nucleus, as \widehat.
//
// okage: the accent is the combining mark of the face, and its attachment
// point sits over the attachment point of a nucleus of one glyph, from the
// MathTopAccentAttachment of the MATH table (WithAccentAttachment). The
// source centers the spacing accent on the advance of the nucleus. A
// stretched accent takes the horizontal variants of the mark, or its
// horizontal assembly, as TeX Appendix G, rule 12, says: the widest variant
// that is not wider than the nucleus. The source does not stretch an accent.
// Both are additions of phase 26 of D78.
//
//go:noinline
func (l *layout) accent(nuc *box, acc rune, stretch bool, st Style) *box {
	px := l.px(st)
	attach := !l.cfg.noAttach
	r := acc
	if attach || stretch {
		if c, ok := combiningAccents[acc]; ok {
			if _, ok := l.face.NominalGlyph(c); ok {
				r = c
			}
		}
	}
	var ab *box
	if stretch {
		ab = l.wideAccent(r, nuc.w, px)
	} else {
		ab = l.mustGlyph(r, px, Ord)
	}
	out := newBox(Ord)
	out.w, out.d = nuc.w, nuc.d
	ax := (nuc.w - ab.w) / 2
	if attach || stretch {
		ax = l.attachment(nuc, attach) - l.attachment(ab, true)
	}
	gap := float64(px * 0.04)
	inkBot := 0.0
	if len(ab.items) > 0 {
		inkBot = ab.h
		for _, it := range ab.items {
			_, bot := l.gidInk(it.Glyph, px)
			inkBot = min(inkBot, bot-it.Y)
		}
	}
	ty := -(nuc.h + gap) + inkBot
	out.h = -ty + ab.h
	place(out, nuc, 0, 0)
	place(out, ab, ax, ty)
	return out
}

// attachment returns where an accent goes over b, from the left end of its
// baseline: the top accent attachment of its glyph when b is one glyph and
// useGlyph is set, the middle of the ink of a glyph with no advance, such as
// a combining mark, and the middle of the advance of b in the other cases.
func (l *layout) attachment(b *box, useGlyph bool) float64 {
	if !useGlyph || !b.kern {
		return b.w / 2
	}
	s := l.scale(b.gpx)
	if a, ok := l.face.MathTopAccentAttachment(b.gid); ok {
		return float64(float64(a) * s)
	}
	if b.w == 0 {
		if e, ok := l.face.GlyphExtents(b.gid); ok {
			return float64((float64(e.XBearing) + float64(float64(e.Width)/2)) * s)
		}
	}
	return b.w / 2
}

// radical sets \sqrt{body} with a radical sign that grows, and an optional
// degree, which can be nil.
//
// The degree is placed by three MATH constants: its bottom is
// RadicalDegreeBottomRaisePercent of the ascender of the sign over the
// baseline, with RadicalKernBeforeDegree before it and a negative
// RadicalKernAfterDegree after it. plain.tex raises it by .6(\ht0-\dp0) and
// puts \mskip5mu and \mskip-10mu around it. So the degree sits inside the
// height of the sign.
//
//go:noinline
func (l *layout) radical(body, index *box, st Style) *box {
	px := l.px(st)
	rt := l.mc(font.RadicalRuleThickness, px)
	gap := l.mc(font.RadicalVerticalGap, px)
	if st == Display {
		gap = l.mc(font.RadicalDisplayStyleVerticalGap, px)
	}
	asc := l.mc(font.RadicalExtraAscender, px)
	target := body.h + body.d + gap + rt
	sign := l.stretchVertical('√', target, px, Ord)
	// okage: a sign that is taller than the target opens the gap by half
	// of the excess, so that the body is centered in the sign (Appendix G,
	// rule 11, and the OpenType specification of RadicalVerticalGap). The
	// source puts all the excess below the body (math.go:887).
	if excess := sign.h + sign.d - target; excess > 0 {
		gap += float64(excess / 2)
	}
	out := newBox(Ord)
	ruleY := -(body.h + gap + rt)
	x := 0.0
	degreeTop := 0.0
	if index != nil {
		x += l.mc(font.RadicalKernBeforeDegree, px)
		ascender := -ruleY // the top of the ink of the sign, over the baseline
		raise := float64(float64(l.face.MathConstant(font.RadicalDegreeBottomRaisePercent)) / 100 * ascender)
		degreeTop = raise + index.d + index.h
		iw := index.w
		place(out, index, x, -(raise + index.d))
		x = max(x+iw+l.mc(font.RadicalKernAfterDegree, px), 0)
	}
	// The top of the ink of the sign meets the rule. Its tick then reaches
	// about the depth of the body.
	ty := ruleY + sign.h
	sw, sd := sign.w, sign.d
	place(out, sign, x, ty)
	x += sw
	bw, bd := body.w, body.d
	place(out, body, x, 0)
	l.rule(out, x, ruleY, bw, rt)
	out.w = x + bw
	out.h = max(-ruleY+rt+asc, degreeTop)
	out.d = max(bd, ty+sd)
	return out
}

// stretchVertical returns the box of the character r grown to at least the
// height target, from the vertical size variants of the MATH table and, when
// they are too small, from its glyph assembly. The box is on the baseline, not
// centered.
//
// okage: a character without a glyph gives an empty box, as mustGlyph does.
// The source draws the glyph .notdef (math.go:933).
//
//go:noinline
func (l *layout) stretchVertical(r rune, target, px float64, cls Class) *box {
	gid, ok := l.face.NominalGlyph(r)
	if !ok {
		l.missing(r)
		return newBox(cls)
	}
	text := string(r)
	best := gid
	s := l.scale(px)
	for _, v := range l.face.MathGlyphVariants(gid, true) {
		best = v.Glyph
		if float64(float64(v.Advance)*s) >= target {
			return l.gidBox(best, px, cls, text)
		}
	}
	// The variants do not reach it. An assembly can reach any height, and
	// without one the largest variant is all that the face has.
	if asm, ok := l.face.MathGlyphAssembly(gid, true); ok {
		if b := l.assembleVertical(asm, target, px, cls, text); b != nil {
			// okage: an assembly has the italic correction of its recipe.
			b.glyph = true
			if !l.cfg.noItalic {
				b.ic = float64(float64(asm.ItalicsCorrection) * s)
			}
			return b
		}
	}
	return l.gidBox(best, px, cls, text)
}

// maxRepeats is the number of times that an assembly repeats its extenders
// at most, as in the source.
const maxRepeats = 64

// assemblyFit solves an assembly for the size target: the count n of the
// repeats of its extenders and the overlap ov of each joint. ok is false when
// the recipe cannot grow.
//
// The overlap is solved for: each joint overlaps at least
// MinConnectorOverlap, and at most the shorter connector of its two parts.
// The count of repeats is the smallest that reaches the target at the
// smallest overlap, and then the overlap is spread evenly so that the
// assembly lands as close to the target as the joints allow.
func (l *layout) assemblyFit(asm font.MathGlyphAssembly, target, px float64) (n int, ov float64, ok bool) {
	if len(asm.Parts) == 0 {
		return 0, 0, false
	}
	s := l.scale(px)
	minOv := float64(float64(l.face.MathMinConnectorOverlap()) * s)
	// The tightest joint sets how far the stack can be pressed. A repeated
	// extender also joins itself.
	maxOv := math.Inf(1)
	for i := 0; i+1 < len(asm.Parts); i++ {
		o := float64(min(float64(asm.Parts[i].EndConnectorLength), float64(asm.Parts[i+1].StartConnectorLength)) * s)
		maxOv = min(maxOv, o)
	}
	for _, p := range asm.Parts {
		if p.Extender {
			maxOv = min(maxOv, float64(min(float64(p.EndConnectorLength), float64(p.StartConnectorLength))*s))
		}
	}
	if math.IsInf(maxOv, 1) || maxOv < minOv {
		maxOv = minOv
	}
	fixed, ext := 0.0, 0.0
	nFixed, nExt := 0, 0
	for _, p := range asm.Parts {
		if p.Extender {
			ext += float64(float64(p.FullAdvance) * s)
			nExt++
			continue
		}
		fixed += float64(float64(p.FullAdvance) * s)
		nFixed++
	}
	if nExt == 0 || ext <= float64(minOv*float64(nExt)) {
		// No extender, or one that adds nothing once its overlap is paid: the
		// recipe cannot grow.
		return 0, 0, false
	}
	// The smallest count of repeats that reaches the target, at the smallest
	// overlap, which gives the largest size for a count.
	k := nFixed
	adv := fixed
	for n < maxRepeats && adv-float64(minOv*float64(k-1)) < target {
		n++
		k = nFixed + n*nExt
		adv = fixed + float64(float64(n)*ext)
	}
	// Then the even overlap that lands closest to the target from above.
	ov = minOv
	if k > 1 {
		ov = max(minOv, min(maxOv, (adv-target)/float64(k-1)))
	}
	return n, ov, true
}

// assembleVertical stacks the parts of an assembly until they cover target,
// with the extenders repeated, and returns nil when the recipe cannot grow.
// The parts go from the bottom to the top, as the OpenType specification
// says for MathGlyphAssembly, and assemblyFit gives the repeats and the
// overlap.
//
// okage: the first part is at the bottom. The source stacks the parts from
// the last one up, so the top part of a recipe was at the bottom, and a
// parenthesis of parts was upside down (math.go:1038).
//
//go:noinline
func (l *layout) assembleVertical(asm font.MathGlyphAssembly, target, px float64, cls Class, text string) *box {
	n, ov, ok := l.assemblyFit(asm, target, px)
	if !ok {
		return nil
	}
	s := l.scale(px)
	out := newBox(cls)
	y := 0.0 // y grows down: the parts go from the bottom up
	first := true
	w := 0.0
	for _, p := range asm.Parts {
		reps := 1
		if p.Extender {
			reps = n
		}
		for range reps {
			g := l.gidBox(p.Glyph, px, cls, text)
			text = ""
			if !first {
				y += ov
			}
			first = false
			y -= float64(float64(p.FullAdvance) * s)
			w = max(w, g.w)
			place(out, g, 0, y)
		}
	}
	// The stack is built up from the baseline, so it is all above it.
	// axisCentre then puts it on the axis, as it does for a variant.
	out.w = w
	out.h = -y
	out.d = 0
	return out
}

// assembleHorizontal sets the parts of an assembly from the left to the
// right until they cover target, as assembleVertical does upward. It returns
// nil when the recipe cannot grow. This is an addition of phase 26 of D78,
// for \overbrace, \underbrace and the wide accents.
//
//go:noinline
func (l *layout) assembleHorizontal(asm font.MathGlyphAssembly, target, px float64, cls Class, text string) *box {
	n, ov, ok := l.assemblyFit(asm, target, px)
	if !ok {
		return nil
	}
	s := l.scale(px)
	out := newBox(cls)
	x := 0.0
	first := true
	for _, p := range asm.Parts {
		reps := 1
		if p.Extender {
			reps = n
		}
		for range reps {
			g := l.gidBox(p.Glyph, px, cls, text)
			text = ""
			if !first {
				x -= ov
			}
			first = false
			out.h, out.d = max(out.h, g.h), max(out.d, g.d)
			place(out, g, x, 0)
			x += float64(float64(p.FullAdvance) * s)
		}
	}
	out.w = x
	return out
}

// stretchHorizontal returns the box of the character r grown to at least the
// width target, from the horizontal size variants of the MATH table and, when
// they are too narrow, from its horizontal glyph assembly. It is
// stretchVertical in the other direction, an addition of phase 26 of D78.
//
//go:noinline
func (l *layout) stretchHorizontal(r rune, target, px float64, cls Class) *box {
	gid, ok := l.face.NominalGlyph(r)
	if !ok {
		l.missing(r)
		return newBox(cls)
	}
	text := string(r)
	best := gid
	s := l.scale(px)
	for _, v := range l.face.MathGlyphVariants(gid, false) {
		best = v.Glyph
		if float64(float64(v.Advance)*s) >= target {
			return l.gidBox(best, px, cls, text)
		}
	}
	if asm, ok := l.face.MathGlyphAssembly(gid, false); ok {
		if b := l.assembleHorizontal(asm, target, px, cls, text); b != nil {
			return b
		}
	}
	return l.gidBox(best, px, cls, text)
}

// axisCentre moves a box so that the middle of its height sits on the math
// axis, for the delimiters that grow, which straddle the axis.
//
//go:noinline
func (l *layout) axisCentre(b *box, px float64) *box {
	dy := float64((b.h-b.d)/2) - l.axis(px)
	out := newBox(b.cls)
	out.w = b.w
	out.ic, out.glyph, out.kern, out.gid, out.gpx, out.gy = b.ic, b.glyph, b.kern, b.gid, b.gpx, b.gy-dy
	out.h = b.h - dy
	out.d = max(b.d+dy, 0)
	place(out, b, 0, dy)
	return out
}

// delimiterFactor and delimiterShortfall are \delimiterfactor and
// \delimitershortfall of plain.tex: 901, and 5pt at 10pt, so 0.5 em.
const (
	delimiterFactor    = 901
	delimiterShortfall = 0.5
)

// delimited puts the delimiters open and close, which grow, around inner. A
// rune of 0 is no delimiter.
//
// okage: the delimiters cover at least 901/1000 of the height around the
// axis, and miss at most 0.5 em of it, as TeX Appendix G, rule 19, says
// with \delimiterfactor and \delimitershortfall. The source grows them to
// the whole height (math.go:1083), so a delimiter was often a size too
// large.
//
//go:noinline
func (l *layout) delimited(inner *box, open, close rune, st Style) *box {
	px := l.px(st)
	target := l.delimTarget(inner, px)
	out := newBox(Inner)
	x := 0.0
	add := func(b *box) {
		out.h = max(out.h, b.h)
		out.d = max(out.d, b.d)
		w := b.w
		place(out, b, x, 0)
		x += w
	}
	if open != 0 {
		add(l.axisCentre(l.stretchVertical(open, target, px, Open), px))
	}
	add(inner)
	if close != 0 {
		add(l.axisCentre(l.stretchVertical(close, target, px, Close), px))
	}
	out.w = x
	return out
}

// delimTarget returns the height of the delimiters around inner at the size
// px, by the rule of delimited.
func (l *layout) delimTarget(inner *box, px float64) float64 {
	delta := 2 * max(inner.h-l.axis(px), inner.d+l.axis(px))
	return max(float64(delta*delimiterFactor)/1000, delta-float64(delimiterShortfall*px))
}

// gridOpts holds the settings of gridLayout.
type gridOpts struct {
	aligns []Align   // the alignment of each column, centered when short
	gaps   []float64 // the gap to the right of each column, colGap when short
	colGap float64   // the gap between columns, and the margin of a vertical rule
	vrules []int     // the column gaps, 0 to the column count, that carry a vertical rule
	hrules []int     // the row gaps, 0 to the row count, that carry a horizontal rule
	hdash  []bool    // for each rule of hrules, whether it is dashed
	lead   leading   // the model of the space between the rows, see leading.go
	axisPx float64   // the size of the axis and of the vertical rules
}

// gridLayout sets rows and columns on a grid centered on the math axis, with
// an alignment for each column and optional vertical rules between the
// columns. It is the core of matrix, array, aligned, gathered, smallmatrix
// and \substack.
//
// The vertical model of TeX for anything on \array, which is each matrix and
// cases: \@arstrut puts a strut of .7\baselineskip by .3\baselineskip in each
// row (latex.ltx:12103), and the rows stack on a grid of baselines. The pitch
// has a floor of \baselineskip and opens only for a row that is taller.
//
//go:noinline
func (l *layout) gridLayout(rows [][]*box, o gridOpts) *box {
	ncol := 0
	for _, r := range rows {
		ncol = max(ncol, len(r))
	}
	align := func(j int) Align {
		if j < len(o.aligns) {
			return o.aligns[j]
		}
		return AlignCenter
	}
	gap := func(j int) float64 { // the gap to the right of column j
		if j < len(o.gaps) {
			return o.gaps[j]
		}
		return o.colGap
	}
	colW := make([]float64, ncol)
	rowH := make([]float64, len(rows))
	rowD := make([]float64, len(rows))
	for i, r := range rows {
		for j, c := range r {
			colW[j] = max(colW[j], c.w)
			rowH[i] = max(rowH[i], c.h)
			rowD[i] = max(rowD[i], c.d)
		}
	}
	sh, sd := o.lead.strut()
	for i := range rows {
		rowH[i] = max(rowH[i], sh)
		rowD[i] = max(rowD[i], sd)
	}
	// rowSep[i] separates the row i from the row i+1. The struts come first,
	// because the glue of tex.web §679 reads the extents.
	rowSep := make([]float64, 0, len(rows))
	for i := 0; i+1 < len(rows); i++ {
		rowSep = append(rowSep, o.lead.gap(rowD[i], rowH[i+1]))
	}
	totalH := 0.0
	for i := range rows {
		totalH += rowH[i] + rowD[i]
		if i > 0 {
			totalH += rowSep[i-1]
		}
	}
	// okage: a horizontal rule of \hline adds its thickness between two rows,
	// as \noalign{\hrule} of LaTeX does. The source has no \hline.
	hrt := 0.0
	if len(o.hrules) > 0 {
		hrt = l.mc(font.FractionRuleThickness, o.axisPx)
		totalH += float64(float64(len(o.hrules)) * hrt)
	}
	hasRule := func(g int) bool { return slices.Contains(o.vrules, g) }
	x0 := 0.0
	if hasRule(0) {
		x0 = o.colGap // room for a rule at the left edge
	}
	colStart := make([]float64, ncol)
	x := x0
	for j := range ncol {
		colStart[j] = x
		x += colW[j]
		if j < ncol-1 {
			x += gap(j)
		}
	}
	rightEnd := x
	grid := newBox(Inner)
	axis := l.axis(o.axisPx)
	grid.h = float64(totalH/2) + axis
	grid.d = float64(totalH/2) - axis
	// The vertical rules first, behind the cells.
	if len(o.vrules) > 0 {
		rt := l.mc(font.FractionRuleThickness, o.axisPx)
		for _, g := range o.vrules {
			var rx float64
			switch {
			case g <= 0:
				rx = x0 / 2
			case g >= ncol:
				rx = rightEnd + float64(o.colGap/2)
			default:
				rx = colStart[g] - float64(gap(g-1)/2)
			}
			l.rule(grid, rx-float64(rt/2), -grid.h, rt, grid.h+grid.d)
		}
	}
	y := -float64(totalH/2) - axis
	gw := rightEnd
	if hasRule(ncol) {
		gw += o.colGap
	}
	hline := func(i int) {
		for k, g := range o.hrules {
			if g != i {
				continue
			}
			if k < len(o.hdash) && o.hdash[k] {
				l.dashedRule(grid, 0, y, gw, hrt, float64(hdashLength*o.axisPx))
			} else {
				l.rule(grid, 0, y, gw, hrt)
			}
			y += hrt
		}
	}
	for i, r := range rows {
		hline(i)
		y += rowH[i]
		for j, c := range r {
			var off float64
			switch align(j) {
			case AlignRight:
				off = colW[j] - c.w
			case AlignCenter:
				off = (colW[j] - c.w) / 2
			}
			place(grid, c, colStart[j]+off, y)
		}
		y += rowD[i]
		if i+1 < len(rows) {
			y += rowSep[i]
		}
	}
	hline(len(rows))
	grid.w = gw
	return grid
}

// hdashLength is the length of a dash of \hdashline and of the gap after it,
// in em: the 4pt of \dashlinedash and \dashlinegap of the package arydshln
// at 10pt (D95).
const hdashLength = 0.4

// dashedRule draws a dashed horizontal rule of the width w and the
// thickness h at (x, y): dashes of the length dash, with gaps of the same
// length. The last dash is cut at the end of the rule.
func (l *layout) dashedRule(dst *box, x, y, w, h, dash float64) {
	if !(dash > 0) {
		l.rule(dst, x, y, w, h)
		return
	}
	for i := 0; ; i++ {
		x0 := float64(float64(2*i) * dash)
		if x0 >= w {
			return
		}
		l.rule(dst, x+x0, y, min(dash, w-x0), h)
	}
}

// alignedAligns returns the columns of aligned and split: right and left in
// turn, in pairs joined at the &.
func alignedAligns(ncol int) []Align {
	a := make([]Align, ncol)
	for j := range a {
		if j%2 == 0 {
			a[j] = AlignRight
		} else {
			a[j] = AlignLeft
		}
	}
	return a
}

// alignedGaps gives small gaps inside each pair of columns and wider gaps
// between the pairs.
func alignedGaps(ncol int, px float64) []float64 {
	if ncol <= 1 {
		return nil
	}
	g := make([]float64, ncol-1)
	for j := range g {
		if j%2 == 0 { // between the right and the left half of a pair
			g[j] = float64(px * 0.17)
		} else { // between two pairs
			g[j] = float64(px * 0.9)
		}
	}
	return g
}

// bl is \baselineskip for cells set at the size p. The layout gets a size
// and not a leading, so it takes the ratio of \normalsize in size10.clo of
// LaTeX, \@setfontsize\normalsize\@xpt{12}: 1.2.
func bl(p float64) float64 { return float64(1.2 * p) }

// jot is \jot, the leading that \openup adds between the rows of a display
// of several lines: 3pt at 10pt, which aligned and gathered of amsmath use.
func jot(p float64) float64 { return float64(0.3 * p) }

// finishEnv lays out the rows of an environment by its kind. cst is the
// style of the cells, and st the style in force where the environment
// opens.
//
//go:noinline
func (l *layout) finishEnv(n *Array, rows [][]*box, cst, st Style) *box {
	ncol := 0
	for _, r := range rows {
		ncol = max(ncol, len(r))
	}
	p := l.px(cst)
	outer := l.px(st)
	switch n.Kind {
	case ArrayArray:
		return l.gridLayout(rows, gridOpts{aligns: n.Align, colGap: float64(p * 0.5),
			lead: strutLeading{baselineskip: bl(p)}, vrules: n.VRules, hrules: n.HRules, hdash: n.HDashed, axisPx: outer})
	// aligned and gathered take the same grid and \jot: \openup\jot of
	// amsmath opens each row of a display by \jot. Measured with tectonic,
	// both have one row of 12pt, the plain strut, and a pitch of 15pt.
	case ArrayAligned:
		return l.gridLayout(rows, gridOpts{aligns: alignedAligns(ncol), gaps: alignedGaps(ncol, p),
			colGap: float64(p * 0.5), lead: strutLeading{baselineskip: bl(p), jot: jot(p)}, axisPx: outer})
	case ArrayGathered:
		return l.gridLayout(rows, gridOpts{colGap: float64(p * 0.6),
			lead: strutLeading{baselineskip: bl(p), jot: jot(p)}, axisPx: outer})
	// smallmatrix has no strut: it sets its own leading in \ex@ and takes the
	// glue of §679 (amsmath.sty:1045-1047, \baselineskip6\ex@
	// \lineskip1.5\ex@ \lineskiplimit\lineskip). \ex@ is computed at the size
	// where the environment opens, not at the script size of its cells.
	case ArraySmallMatrix:
		ex := exAt(outer)
		// The gaps are in mu of the outer size: the \thickspace and the two \,
		// of the template are outside the $\m@th\scriptstyle##$ of each cell.
		mu := outer / 18
		grid := l.gridLayout(rows, gridOpts{colGap: float64(5 * mu),
			lead: glueLeading{baselineskip: float64(6 * ex), lineskip: float64(1.5 * ex), lineskiplimit: float64(1.5 * ex)}, axisPx: outer})
		// The environment is an Ord atom on both sides: \vcenter is an Ord.
		grid.cls = Ord
		// \null\, before and \, after (amsmath.sty:1045 and :1052).
		return l.hlist([]entry{
			{b: l.kern(float64(3 * mu)), st: st},
			{b: grid, st: st, atom: true},
			{b: l.kern(float64(3 * mu)), st: st},
		})
	case ArraySubstack:
		grid := l.gridLayout(rows, gridOpts{lead: flatLeading{rowGap: float64(p * 0.18)}, axisPx: outer})
		grid.cls = Ord
		return grid
	default: // the matrix family
		grid := l.gridLayout(rows, gridOpts{colGap: float64(p * 0.6),
			lead: strutLeading{baselineskip: bl(p), stretch: n.Stretch}, hrules: n.HRules, hdash: n.HDashed, axisPx: outer})
		if n.Open == 0 && n.Close == 0 {
			return grid
		}
		return l.delimited(grid, n.Open, n.Close, st)
	}
}

// displayOperator grows a large operator for display style. TeX takes one
// step up the charlist of the font (tex.web:14685-14691). A font with a MATH
// table gives the vertical variants of the glyph and the constant
// DisplayOperatorMinHeight, so the variant is the first one that reaches
// that height. An operator that already reaches it stays.
//
//go:noinline
func (l *layout) displayOperator(r rune, px float64, cls Class, base *box) *box {
	target := l.mc(font.DisplayOperatorMinHeight, px)
	if base.h+base.d >= target {
		return base
	}
	return l.stretchVertical(r, target, px, cls)
}

// wideAccent returns the accent r stretched for a nucleus of the width w: the
// widest horizontal variant that is not wider than w, as TeX Appendix G, rule
// 12, chooses a successor of an accent. When the widest variant is narrower
// than w, the horizontal assembly grows to w, if the face has one. This is
// an addition of phase 26 of D78.
//
//go:noinline
func (l *layout) wideAccent(r rune, w, px float64) *box {
	gid, ok := l.face.NominalGlyph(r)
	if !ok {
		l.missing(r)
		return newBox(Ord)
	}
	s := l.scale(px)
	best := gid
	short := true // the widest variant is narrower than w
	for _, v := range l.face.MathGlyphVariants(gid, false) {
		if float64(float64(v.Advance)*s) > w {
			short = false
			break
		}
		best = v.Glyph
	}
	if short {
		if asm, ok := l.face.MathGlyphAssembly(gid, false); ok {
			if b := l.assembleHorizontal(asm, w, px, Ord, string(r)); b != nil {
				return b
			}
		}
	}
	return l.gidBox(best, px, Ord, string(r))
}
