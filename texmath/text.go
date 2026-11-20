package texmath

import (
	"golang.org/x/image/math/fixed"

	"github.com/xo/okage"
	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/language"
	"github.com/xo/okage/typesetting/shaping"
)

// This file ports no file of go-tex/math. It holds the text face of phase 26
// of D78: the options that give the faces of text, the text commands \text,
// \textbf, \textit, \textsf, \texttt, \textrm, \textup, \textmd,
// \textnormal and \emph, and the layout of a TextRun. The source sets \text
// with the upright glyphs of the math face, and has none of the other
// commands.

// WithTextFace sets the faces of the roman family of text, for \text and a
// TextRun: the regular, the bold, the italic and the bold italic face. A nil
// face is not set. The layout shapes a run of text with typesetting/shaping,
// so the ligatures and the kerning of the face apply. A style whose face is
// not set takes the glyphs of the math alphabets of the math face, as the
// layout does with no text face at all: the upright, bold, italic and bold
// italic letters and digits, and the other characters of the math face. The
// source has no text face.
func WithTextFace(regular, bold, italic, boldItalic *font.Face) Option {
	return WithTextFamily(FamilyRoman, regular, bold, italic, boldItalic)
}

// WithTextFamily sets the faces of the family f of text, as WithTextFace does
// for the roman family: \textsf takes FamilySans, and \texttt FamilyMono. A
// style whose face is not set takes the sans-serif or the monospace alphabet
// of the math face. An unknown family is ignored.
func WithTextFamily(f Family, regular, bold, italic, boldItalic *font.Face) Option {
	return func(c *config) {
		if f > FamilyMono {
			return
		}
		c.textFaces[f] = [4]*font.Face{regular, bold, italic, boldItalic}
	}
}

// textFace returns the face that the options give for a run of the family f
// in bold and italic, or nil.
func (c *config) textFace(f Family, bold, italic bool) *font.Face {
	if f > FamilyMono {
		f = FamilyRoman
	}
	i := 0
	if bold {
		i++
	}
	if italic {
		i += 2
	}
	return c.textFaces[f][i]
}

// textAlphabet returns the alphabet of text inside st: st itself when it is
// already text, so that \text in \textbf stays bold, and the regular roman
// text otherwise.
func textAlphabet(st alphabet) alphabet {
	if st.text {
		return st
	}
	return alphabet{set: true, text: true, variant: Upright}
}

// run makes a TextRun of s in the text alphabet a.
func (a alphabet) run(s string) TextRun {
	return TextRun{Text: s, Bold: a.bold, Italic: a.italic, Family: a.family}
}

// parseTextFont parses \text and its kin, \textbf, \textit, \textsf,
// \texttt, \textrm, \textup, \textmd, \textnormal and \emph. Each sets its
// argument as text, and changes one part of the text style in force, as
// LaTeX does: \textbf in \textit is bold italic, \emph turns italic on or
// off, and \textnormal goes back to the regular roman style. \text, \mbox,
// \hbox and \textrm in math start from the regular roman style.
func (p *parser) parseTextFont(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	a := textAlphabet(st)
	switch cmd.text {
	case "textbf":
		a.bold = true
	case "textmd":
		a.bold = false
	case "textit":
		a.italic = true
	case "textup":
		a.italic = false
	case "emph":
		a.italic = !a.italic
	case "textrm":
		a.family = FamilyRoman
	case "textsf":
		a.family = FamilySans
	case "texttt":
		a.family = FamilyMono
	case "textnormal":
		a = textAlphabet(alphabet{})
	}
	b, r, err := p.parseGroupArg(toks, a)
	if err != nil {
		return nil, nil, err
	}
	return p.group(b), r, nil
}

// textVariant returns the math alphabet that stands for a text style when
// the options give no face for it. Unicode has no bold or italic monospace
// alphabet, so each monospace style is Monospace.
func textVariant(f Family, bold, italic bool) Variant {
	switch f {
	case FamilySans:
		switch {
		case bold && italic:
			return SansSerifBoldItalic
		case bold:
			return SansSerifBold
		case italic:
			return SansSerifItalic
		}
		return SansSerif
	case FamilyMono:
		return Monospace
	}
	switch {
	case bold && italic:
		return BoldItalic
	case bold:
		return Bold
	case italic:
		return Italic
	}
	return Upright
}

// textRun lays out a TextRun. With a face of WithTextFace or
// WithTextFamily for its style, the run is shaped with that face. Without
// one, its characters take the math alphabet of textVariant, and the other
// characters, the spaces among them, the glyphs of the math face, as the
// source sets \text.
//
//go:noinline
func (l *layout) textRun(node Node, st Style) *box {
	n, _ := node.(TextRun)
	px := l.px(st)
	if f := l.cfg.textFace(n.Family, n.Bold, n.Italic); f != nil {
		return l.shapeText(n.Text, f, px)
	}
	return l.opName(n.Text, textVariant(n.Family, n.Bold, n.Italic), px)
}

// shapeText shapes s with the text face f at the size px. The shaper works at
// the size of one em in font units, so that each advance and offset is an
// exact number of font units, and the layout scales them. Each glyph is an
// item, and the first glyph of a cluster carries the characters of the
// cluster. A character that f has no glyph for takes the glyph of the math
// face, or an empty box when that face has none either.
//
//go:noinline
func (l *layout) shapeText(s string, f *font.Face, px float64) *box {
	text := []rune(s)
	out := newBox(Ord)
	if len(text) == 0 {
		return out
	}
	script := language.Latin
	for _, c := range text {
		if sc := language.LookupScript(c); sc.Strong() {
			script = sc
			break
		}
	}
	shaped := l.shaper.Shape(shaping.Input{
		Text:      text,
		RunStart:  0,
		RunEnd:    len(text),
		Direction: di.DirectionLTR,
		Face:      f,
		Size:      fixed.Int26_6(int32(f.Upem()) * 64),
		Script:    script,
	})
	k := px / float64(f.Upem())
	x := 0.0 // in the unit of the size, after the fallbacks of the math face
	cluster := -1
	for _, g := range shaped.Glyphs {
		first := g.TextIndex() != cluster
		cluster = g.TextIndex()
		var chars string
		if first {
			end := min(cluster+max(g.RunesCount(), 1), len(text))
			chars = string(text[cluster:end])
		}
		if g.GlyphID == 0 {
			// The face has no glyph: take the glyphs of the math face for the
			// characters of the cluster, once.
			if first {
				for _, c := range chars {
					b := l.mustGlyph(c, px, Ord)
					bw := b.w
					out.h, out.d = max(out.h, b.h), max(out.d, b.d)
					place(out, b, x, 0)
					x += bw
				}
			}
			continue
		}
		dx := float64(float64(g.XOffset) / 64 * k)
		dy := -float64(float64(g.YOffset) / 64 * k)
		if top, bot := faceInk(f, g.GlyphID, k); top != 0 || bot != 0 {
			out.h = max(out.h, top-dy)
			out.d = max(out.d, -(bot - dy))
		}
		if l.count() {
			out.items = append(out.items, Item{Kind: ItemGlyph, Face: f, Glyph: g.GlyphID, Size: px,
				X: x + dx, Y: dy, Text: chars, Color: l.color, Transform: okage.IdentityTransform()})
		}
		x += float64(float64(g.Advance) / 64 * k)
	}
	out.w = x
	return out
}

// faceInk returns the highest and the lowest ink of the glyph g of the face f
// above the baseline, with y up, at the scale k of the size for each font
// unit. It is gidInk for any face.
func faceInk(f *font.Face, g font.GID, k float64) (top, bot float64) {
	e, ok := f.GlyphExtents(g)
	if !ok {
		return 0, 0
	}
	return float64(float64(e.YBearing) * k), float64((float64(e.YBearing) + float64(e.Height)) * k)
}
