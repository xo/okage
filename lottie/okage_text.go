// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports no file of velato. It holds the model of a text layer, its
// conversion from the schema, and its layout into glyphs (D79, D85). The
// layout follows updateLocalFont in src/loaders/lottie/tvgLottieBuilder.cpp
// of ThorVG, in pixels instead of the units of the glyphs.

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/xo/okage/kurbo"
)

// TextDocument is a text document of a text layer, at one keyframe.
type TextDocument struct {
	// Text is the text. A carriage return, a line feed or the character 3
	// starts a new line.
	Text string
	// Font is the name of the font, the key of Composition.Fonts.
	Font string
	// Size is the font size in pixels.
	Size float64
	// Justify is the share of the free space of a line on the left of the
	// line: 0 for the left, 0.5 for the center and 1 for the right.
	Justify float64
	// Caps is 1 when a small letter draws the glyph of its capital, and 2
	// when it draws it at 70 percent of the size (D85).
	Caps int
	// Tracking is the space after each glyph of the list chars, in
	// thousandths of an em. For a glyph that the sink shapes, it scales the
	// advance instead, as ThorVG does (D82).
	Tracking float64
	// LineHeight is the distance from one baseline to the next, in pixels.
	LineHeight float64
	// BaselineShift moves the baseline up, in pixels.
	BaselineShift float64
	// Fill is the color of the glyphs, and Stroke the color of their
	// outline, both opaque.
	Fill   Color
	Stroke Color
	// StrokeWidth is the width of the outline, in pixels. 0 is no outline.
	StrokeWidth float64
	// StrokeBelow draws the outline below the fill.
	StrokeBelow bool
	// BoxPosition and BoxSize are the box of a box text. BoxSize is zero
	// for a text without a box.
	BoxPosition kurbo.Point
	BoxSize     kurbo.Size
}

// TextKeyframe is a text document from the frame Frame on.
type TextKeyframe struct {
	Frame    float64
	Document TextDocument
}

// Font is a font of the font list of a file.
type Font struct {
	// Name, Family and Style are the keys fName, fFamily and fStyle.
	Name   string
	Family string
	Style  string
	// Ascent is the height above the baseline, in hundredths of the font
	// size.
	Ascent float64
	// Glyphs are the glyphs of the list chars that have the family and the
	// style of the font, in the order of the file.
	Glyphs []Glyph
}

// Glyph is a glyph of the list chars of a file.
type Glyph struct {
	// Text is the text that the glyph draws.
	Text string
	// Width is the advance of the glyph at the font size 100.
	Width float64
	// Path is the outline of the glyph at the font size 100, with y down
	// and the baseline at y = 0.
	Path []kurbo.PathEl
}

// glyph returns the first glyph of f whose text starts text, or nil.
func (f *Font) glyph(text string) *Glyph {
	if f == nil {
		return nil
	}
	for i := range f.Glyphs {
		if g := &f.Glyphs[i]; strings.HasPrefix(text, g.Text) {
			return g
		}
	}
	return nil
}

// textAt returns the document of the keyframes at frame: the last document
// whose frame is at or before frame, or the first one, as ThorVG picks it.
func textAt(keyframes []TextKeyframe, frame float64) *TextDocument {
	if len(keyframes) == 0 {
		return nil
	}
	d := &keyframes[0].Document
	for i := 1; i < len(keyframes); i++ {
		if keyframes[i].Frame > frame {
			break
		}
		d = &keyframes[i].Document
	}
	return d
}

// convFonts converts the font list and the glyphs of a file. A glyph whose
// family and style match no font is dropped, as ThorVG drops it.
func (c *converter) convFonts(source *Animation) map[string]*Font {
	if source.Fonts == nil || len(source.Fonts.List) == 0 {
		return nil
	}
	fonts := make(map[string]*Font, len(source.Fonts.List))
	list := make([]*Font, 0, len(source.Fonts.List))
	for _, f := range source.Fonts.List {
		font := &Font{Name: f.Name, Family: f.Family, Style: f.Style, Ascent: f.Ascent}
		list = append(list, font)
		if _, ok := fonts[f.Name]; !ok {
			fonts[f.Name] = font
		}
	}
	for i := range source.Chars {
		ch := &source.Chars[i]
		if ch.Char == "" {
			continue
		}
		var shapes []Shape
		for j := range ch.Data.Shapes {
			if s, ok := c.convShape(&ch.Data.Shapes[j]); ok {
				shapes = append(shapes, s)
			}
		}
		var path []kurbo.PathEl
		for _, p := range evaluatePaths(shapes, kurbo.IdentityAffine, 0) {
			path = append(path, p...)
		}
		for _, font := range list {
			if font.Family == ch.Family && font.Style == ch.Style {
				font.Glyphs = append(font.Glyphs, Glyph{Text: ch.Char, Width: ch.Width, Path: path})
				break
			}
		}
	}
	return fonts
}

// convText converts the text documents of a text layer.
func convText(source *TextLayer) []TextKeyframe {
	keyframes := make([]TextKeyframe, 0, len(source.Text.Document.Keyframes))
	for _, k := range source.Text.Document.Keyframes {
		d := &k.Document
		doc := TextDocument{
			Text:          d.Text,
			Font:          d.FontName,
			Size:          d.Size,
			Caps:          d.Caps,
			Tracking:      d.Tracking,
			LineHeight:    d.LineHeight,
			BaselineShift: d.BaselineShift,
			Fill:          textColor(d.FillColor),
			Stroke:        textColor(d.StrokeColor),
			StrokeWidth:   d.StrokeWidth,
			StrokeBelow:   d.StrokeOverFill != nil && !*d.StrokeOverFill,
		}
		switch d.Justify {
		case 1:
			doc.Justify = 1
		case 2:
			doc.Justify = 0.5
		}
		if len(d.BoxPosition) >= 2 {
			doc.BoxPosition = kurbo.NewPoint(d.BoxPosition[0], d.BoxPosition[1])
		}
		if len(d.BoxSize) >= 2 {
			doc.BoxSize = kurbo.NewSize(d.BoxSize[0], d.BoxSize[1])
		}
		keyframes = append(keyframes, TextKeyframe{Frame: k.Time, Document: doc})
	}
	return keyframes
}

// textColor converts the color of a text document, red, green and blue from
// 0 to 1, as ThorVG rounds it to 8 bits. A missing component is 0.
func textColor(v []float64) Color {
	var c Color
	for i := range min(len(v), 3) {
		x := math.RoundToEven(v[i] * 255)
		c[i] = float32(math.Max(0, math.Min(x, 255)) / 255)
	}
	c[3] = 1
	return c
}

// textUnit is a glyph or a cluster of a line of text, with its advance in
// pixels.
type textUnit struct {
	// space reports if the unit is a space, where a box text can break.
	space bool
	// drawn reports if the unit has a glyph. A unit without one has no
	// advance and draws nothing.
	drawn   bool
	advance float64
	// bytes is the length of the text of the unit in bytes, which the
	// range selectors count (D85).
	bytes int
	// capScale scales a glyph of the list chars that small capitals draw
	// as a capital (D85). It is 1 for any other unit.
	capScale float64
	// glyph is the glyph of the list chars, or nil.
	glyph *Glyph
	// shaped are the glyphs that the sink shaped for the unit.
	shaped []shapedGlyph
	// x and y are the position of the unit in its text, in pixels, after
	// the layout.
	x, y float64
	// group is the group of the unit for the text animators, and style its
	// style after them (D85).
	group int
	style textStyle
	// place maps the pixels of the unit, from its origin, into the pixels
	// of its line.
	place kurbo.Affine
}

// shapedGlyph is a glyph that a textSink shaped: its outline in font units
// with y up, the pixels per font unit, and its offset from the origin of its
// cluster, in pixels with y down. pen is the part of dx that the advances of
// the glyphs before it in its cluster give.
type shapedGlyph struct {
	path   []kurbo.PathEl
	scale  float64
	dx, dy float64
	pen    float64
}

// shapedCluster is a cluster of characters that a textSink shaped.
type shapedCluster struct {
	// text holds the characters of the cluster.
	text string
	// advance is the advance of the cluster in pixels.
	advance float64
	// glyphs are the glyphs of the cluster. A cluster that the font cannot
	// draw has none.
	glyphs []shapedGlyph
}

// textSink is a Sink that shapes the characters that the list chars of a
// file does not hold (D79). PixmapSink implements it with the fonts of
// WithFontDatabase.
type textSink interface {
	// shapeText shapes text with the font that name, family and style
	// describe, at size pixels. It returns the clusters in the order of the
	// text, and the ascent of the font in pixels. Without a font for text, it
	// logs each character and returns false.
	shapeText(name, family, style string, text []rune, size float64) ([]shapedCluster, float64, bool)
}

// textGroup is a group of glyphs that turn about one point (D85): the
// glyphs of one character, word or line, as the key m of the layer says.
// The first glyph of the group that an animator selects sets its
// transform, as updateTextRange of ThorVG does.
type textGroup struct {
	set       bool
	transform kurbo.Affine
}

// textLayout is the state of the layout of a text at a frame, as
// RenderText of ThorVG holds it. The positions are in pixels.
type textLayout struct {
	doc   *TextDocument
	frame float64
	scale float64
	// anim is the text animation of a text in a font with glyphs in the
	// list chars, or nil. ThorVG draws the animators of such a text glyph
	// by glyph.
	anim *TextAnimation
	// base is the style of each unit before the animators of anim, or the
	// style of a text in a font that the sink shapes, after its animators.
	base textStyle
	// follow is the path that the text follows, or nil, and firstMargin
	// the margin before its first glyph, in the units of a glyph.
	follow      *textFollow
	firstMargin float64
	// total is the length of the text in bytes. line, space and idx count
	// the lines, the spaces and the bytes before the current unit.
	total            float32
	line, space, idx int
	cursor           kurbo.Point
	// lineSpace is the largest line spacing of the animators in the
	// current line, and totalLineSpace the sum for the lines before it.
	lineSpace, totalLineSpace float64
	// groups are the groups of the text, and the last one is the current
	// group.
	groups []textGroup
	ascent float64

	transform kurbo.Affine
	alpha     float64
	sink      Sink
}

// renderText draws the glyphs of layer, a text layer, at frame onto sink,
// under transform and with alpha (D79). Each glyph of the list chars draws
// its outline. The sink shapes the other characters, when it is a
// textSink. The lines and the box follow updateLocalFont of ThorVG, and the
// text animators and the path of the text follow updateTextRange and
// updateURLFont (D85).
func renderText(animation *Composition, layer *Layer, frame float64, transform kurbo.Affine, alpha float64, sink Sink) {
	doc := textAt(layer.Content.Text, frame)
	if doc == nil || doc.Text == "" || !(doc.Size > 0) {
		return
	}
	font := animation.Fonts[doc.Font]
	scale := doc.Size / 100
	tracking := doc.Tracking * 0.1 * scale
	l := &textLayout{
		doc:       doc,
		frame:     frame,
		scale:     scale,
		total:     float32(len(doc.Text)),
		transform: transform,
		alpha:     alpha,
		sink:      sink,
	}
	anim := layer.Content.TextAnimation
	if font != nil {
		l.ascent = font.Ascent * scale
	}
	if font != nil && len(font.Glyphs) > 0 {
		// ThorVG draws the animators and the path glyph by glyph only for
		// a font with glyphs in the list chars.
		l.anim = anim
		l.base = newTextStyle(doc)
		if anim != nil && anim.Path != nil && anim.Path.Mask < len(layer.Masks) {
			path := layer.Masks[anim.Path.Mask].Geometry.Evaluate(frame, nil)
			l.follow = newTextFollow(path, scale)
			l.firstMargin = anim.Path.FirstMargin.Evaluate(frame) / scale
		}
	} else {
		l.base = wholeTextStyle(anim, doc, frame)
	}
	l.groups = append(l.groups, textGroup{})
	group := TextGroupCharacters
	if l.anim != nil {
		group = l.anim.Group
	}
	box := doc.BoxSize.Width
	shaper, _ := sink.(textSink)
	for _, paragraph := range splitLines(doc.Text) {
		units, shapedAscent := textUnits(paragraph, font, doc, scale, tracking, shaper)
		if font == nil {
			l.ascent = shapedAscent
		}
		// Break the paragraph into lines, as updateLocalFont does for a box
		// text.
		start := 0
		for i := 0; i <= len(units); {
			if i == len(units) {
				l.endLine(units[start:i])
				break
			}
			u := &units[i]
			if u.space && box > 0 && l.cursor.X+u.advance+nextWordWidth(units[i+1:]) >= box {
				l.endLine(units[start:i])
				i++
				start = i
				continue
			}
			if u.space {
				l.space++
				if group == TextGroupWords {
					l.groups = append(l.groups, textGroup{})
				}
			}
			if !u.drawn {
				l.idx += u.bytes
				i++
				continue
			}
			if box > 0 && l.cursor.X > 0 && l.cursor.X+u.advance > box {
				l.endLine(units[start:i])
				start = i
				continue
			}
			if group == TextGroupCharacters || group == TextGroupAll {
				l.groups = append(l.groups, textGroup{})
			}
			l.place(u)
			l.cursor.X += u.advance
			l.idx += u.bytes
			i++
		}
	}
}

// place sets the position, the group, the style and the transform of the
// unit u at the cursor. The animators that select u move the cursor by
// their tracking first.
func (l *textLayout) place(u *textUnit) {
	st := l.base
	if l.anim != nil {
		st = l.animate()
	}
	u.x, u.y = l.cursor.X, l.cursor.Y
	u.group = len(l.groups) - 1
	u.style = st
	c := u.capScale
	switch {
	case st.grouped:
		g := &l.groups[u.group]
		if !g.set {
			g.set, g.transform = true, l.groupTransform(&st)
		}
		e := g.transform
		u.place = e.Mul(kurbo.Affine{st.scale.X * c, 0, 0, st.scale.Y * c, st.translate.X + u.x - e[4], st.translate.Y + u.y - e[5]})
	case l.follow != nil:
		// _commit of ThorVG puts the middle of the glyph on the path, and
		// does not turn it.
		w := u.advance / l.scale * 0.5
		if u.glyph != nil {
			w = u.glyph.Width * 0.5
		}
		p := l.follow.position(u.x/l.scale + w + l.firstMargin)
		u.place = kurbo.Affine{c, 0, 0, c, (p.X - float64(w*c)) * l.scale, p.Y * l.scale}
	default:
		u.place = kurbo.Affine{c, 0, 0, c, u.x, u.y}
	}
}

// animate returns the style of the unit at the cursor after the animators,
// as updateTextRange of ThorVG computes it. It moves the cursor by the
// tracking of the animators, and keeps the largest line spacing.
func (l *textLayout) animate() textStyle {
	st := l.base
	frame := l.frame
	for i := range l.anim.Animators {
		a := &l.anim.Animators[i]
		idx := l.idx
		switch a.Based {
		case TextBasedCharactersNoSpace:
			idx -= l.space
		case TextBasedWords:
			idx = l.line + l.space
		case TextBasedLines:
			idx = l.line
		}
		f32 := a.factor(frame, l.total, float32(idx))
		if float32(math.Abs(float64(f32))) < textEpsilon {
			continue
		}
		f := float64(f32)
		st.grouped = true
		p := a.Position.Evaluate(frame)
		st.translate = kurbo.Vec2{X: float64(f*p.X) + st.translate.X, Y: float64(f*p.Y) + st.translate.Y}
		s := a.Scale.Evaluate(frame)
		st.scale.X *= float64(f*(float64(s.X*0.01)-1)) + 1
		st.scale.Y *= float64(f*(float64(s.Y*0.01)-1)) + 1
		st.rotate += float64(f * a.Rotation.Evaluate(frame))
		st.opacity = mixOpacity(st.opacity, opacity255(a.Opacity.Evaluate(frame)), f32)
		a.mixColors(&st, frame, f32)
		st.fillOpacity = mixOpacity(st.fillOpacity, opacity255(a.FillOpacity.Evaluate(frame)), f32)
		if a.StrokeWidth != nil {
			st.strokeWidth = f * a.StrokeWidth.Evaluate(frame)
		}
		if st.strokeWidth > 0 {
			st.strokeOpacity = mixOpacity(st.strokeOpacity, opacity255(a.StrokeOpacity.Evaluate(frame)), f32)
		}
		l.cursor.X += float64(float64(f*a.Tracking.Evaluate(frame)) * l.scale)
		if spacing := f * a.LineSpacing.Evaluate(frame); spacing > l.lineSpace {
			l.lineSpace = spacing
		}
	}
	return st
}

// groupTransform returns the transform of a group whose first selected
// glyph has the style st and is at the cursor, as updateTextRange of
// ThorVG sets the transform of a line scene: the cursor, plus the anchor,
// turned by the rotation about a pivot, plus the offset. ThorVG takes the x
// of the anchor for both terms of the x of the pivot, and the y for both
// terms of the y, and okage does the same.
func (l *textLayout) groupTransform(st *textStyle) kurbo.Affine {
	a := l.anim.Anchor.Evaluate(l.frame)
	ax, ay := a.X*l.scale, a.Y*l.scale
	e11, e12, e21, e22 := 1.0, 0.0, 0.0, 1.0
	if st.rotate != 0 {
		sin, cos := math.Sincos(st.rotate * math.Pi / 180)
		e11, e12, e21, e22 = cos, -sin, sin, cos
	}
	e13 := l.cursor.X + ax
	e13 += float64(-ax*e11) + float64(-ax*e12)
	e13 += st.translate.X
	e23 := l.cursor.Y + ay
	e23 += float64(-ay*e21) + float64(-ay*e22)
	e23 += st.translate.Y
	return kurbo.Affine{e11, e21, e12, e22, e13, e23}
}

// endLine draws the units of the current line, and moves the cursor to
// the start of the next line.
func (l *textLayout) endLine(units []textUnit) {
	l.drawLine(units)
	l.line++
	l.totalLineSpace += l.lineSpace
	l.lineSpace = 0
	l.cursor = kurbo.NewPoint(0, float64(float64(l.line)*l.doc.LineHeight)+l.totalLineSpace)
	l.groups = append(l.groups, textGroup{})
}

// splitLines splits text at each carriage return, line feed and character
// 3. An empty line stays, so that it moves the next lines down.
func splitLines(text string) []string {
	var lines []string
	for {
		i := strings.IndexAny(text, "\r\n\x03")
		if i < 0 {
			return append(lines, text)
		}
		lines = append(lines, text[:i])
		text = text[i+1:]
	}
}

// nextWordWidth returns the advance of the units up to the next space, as
// _nextWordWidth of ThorVG does.
func nextWordWidth(units []textUnit) float64 {
	w := 0.0
	for _, u := range units {
		if u.space {
			break
		}
		w += u.advance
	}
	return w
}

// textUnits returns the units of one line of text: a glyph of font for
// each glyph that the list chars holds, and the clusters that shaper
// shapes for the other characters. It also returns the ascent of the
// shaped font in pixels, or 0. When the document has capitals, a small
// letter of ASCII draws the glyph of its capital, at 70 percent of the size
// for small capitals, as _searchGlyph of ThorVG does (D85). The text that
// shaper shapes does not change, as ThorVG does not change a text in a font
// that it shapes.
func textUnits(text string, font *Font, doc *TextDocument, scale, tracking float64, shaper textSink) ([]textUnit, float64) {
	var units []textUnit
	var pending []rune
	ascent := 0.0
	flush := func() {
		if len(pending) == 0 {
			return
		}
		var clusters []shapedCluster
		ok := false
		if shaper != nil {
			family, style := doc.Font, ""
			if font != nil {
				family, style = font.Family, font.Style
			}
			var a float64
			clusters, a, ok = shaper.shapeText(doc.Font, family, style, pending, doc.Size)
			if ok {
				ascent = a
			}
		}
		if !ok {
			for _, c := range pending {
				units = append(units, textUnit{space: c == ' ', bytes: utf8.RuneLen(c), capScale: 1})
			}
		}
		// ThorVG multiplies each advance of a font that it shapes by the
		// factor 1 plus the tracking over the ascent of the font, and does
		// not add the tracking (D82). It keeps the factor 1 when the factor
		// is negative.
		spacing := 1.0
		if ascent > 0 {
			spacing = 1 + tracking/ascent
		}
		if spacing < 0 {
			spacing = 1
		}
		for _, c := range clusters {
			u := textUnit{space: c.text == " ", drawn: len(c.glyphs) > 0, bytes: len(c.text), capScale: 1, shaped: c.glyphs}
			if u.drawn {
				u.advance = c.advance * spacing
				for i := range u.shaped {
					g := &u.shaped[i]
					g.dx += float64(g.pen * (spacing - 1))
				}
			}
			units = append(units, u)
		}
		pending = pending[:0]
	}
	for len(text) > 0 {
		look, capScale := text, 1.0
		if doc.Caps != 0 && 'a' <= text[0] && text[0] <= 'z' {
			look = string(text[0]-'a'+'A') + text[1:]
			if doc.Caps == 2 {
				capScale = 0.7
			}
		}
		if g := font.glyph(look); g != nil && g.Text != "" {
			flush()
			units = append(units, textUnit{
				space:    text[0] == ' ',
				drawn:    true,
				advance:  (float64(g.Width*scale) + tracking) * capScale,
				bytes:    len(g.Text),
				capScale: capScale,
				glyph:    g,
			})
			text = text[len(g.Text):]
			continue
		}
		c, n := utf8.DecodeRuneInString(text)
		pending = append(pending, c)
		text = text[n:]
	}
	flush()
	return units, ascent
}

// drawLine draws the units of the current line, as updateLocalFont places
// a line: the left of the line at the left of the box plus Justify of the
// free space of the box, and the baseline at the top of the box plus the
// ascent, at most the height of the box, minus the baseline shift. A unit
// that an animator selects draws with the other such units of its group,
// when the group ends, as the line scene of ThorVG draws them.
func (l *textLayout) drawLine(units []textUnit) {
	doc := l.doc
	ascent := math.Min(l.ascent, doc.BoxSize.Height)
	x0 := doc.BoxPosition.X + float64(doc.Justify*(doc.BoxSize.Width-l.cursor.X))
	y0 := doc.BoxPosition.Y + ascent - doc.BaselineShift
	origin := kurbo.AffineTranslate(kurbo.NewVec2(x0, y0))
	group := -1
	var grouped []*textUnit
	flush := func() {
		for _, u := range grouped {
			l.drawUnit(u, origin)
		}
		grouped = grouped[:0]
	}
	for i := range units {
		u := &units[i]
		if !u.drawn {
			continue
		}
		if u.group != group {
			flush()
			group = u.group
		}
		if u.style.grouped {
			grouped = append(grouped, u)
			continue
		}
		l.drawUnit(u, origin)
	}
	flush()
}

// drawUnit draws the glyphs of the unit u of a line whose origin is
// origin.
func (l *textLayout) drawUnit(u *textUnit, origin kurbo.Affine) {
	doc := l.doc
	fill, stroke := u.style.brushes(l.alpha)
	// textShape of ThorVG sets the round join only for a document with a
	// stroke. An animator that adds a stroke keeps the bevel join.
	join := JoinBevel
	if doc.StrokeWidth > 0 {
		join = JoinRound
	}
	m := origin.Mul(u.place)
	if g := u.glyph; g != nil {
		if len(g.Path) == 0 {
			return
		}
		t := l.transform.Mul(m).Mul(kurbo.AffineScale(l.scale))
		drawTextGlyph(l.sink, t, g.Path, u.style.strokeWidth/l.scale, join, doc.StrokeBelow, &fill, &stroke)
		return
	}
	for _, g := range u.shaped {
		if len(g.path) == 0 || !(g.scale > 0) {
			continue
		}
		t := l.transform.Mul(m.Mul(kurbo.AffineTranslate(kurbo.NewVec2(g.dx, g.dy)))).Mul(kurbo.AffineScaleNonUniform(g.scale, -g.scale))
		drawTextGlyph(l.sink, t, g.path, u.style.strokeWidth/g.scale, join, doc.StrokeBelow, &fill, &stroke)
	}
}

// drawTextGlyph fills the outline path of a glyph under transform with
// fill, and strokes it with stroke when width is larger than 0, below the
// fill when below is true. The stroke has the given join and the square caps
// of textShape of ThorVG.
func drawTextGlyph(sink Sink, transform kurbo.Affine, path []kurbo.PathEl, width float64, join Join, below bool, fill, stroke *Brush) {
	var st *Stroke
	if width > 0 {
		st = &Stroke{Width: width, Join: join, MiterLimit: 4, StartCap: CapSquare, EndCap: CapSquare}
	}
	if st != nil && below {
		sink.Draw(st, transform, stroke, path)
	}
	sink.Draw(nil, transform, fill, path)
	if st != nil && !below {
		sink.Draw(st, transform, stroke, path)
	}
}
