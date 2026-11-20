// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports no file of velato. It holds the model of a text layer, its
// conversion from the schema, and its layout into glyphs (D79). The layout
// follows updateLocalFont in src/loaders/lottie/tvgLottieBuilder.cpp of
// ThorVG, in pixels instead of the units of the glyphs.

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
	// Tracking is the space after each glyph, in thousandths of an em.
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
	// glyph is the glyph of the list chars, or nil.
	glyph *Glyph
	// shaped are the glyphs that the sink shaped for the unit.
	shaped []shapedGlyph
	// x is the position of the unit in its line, after the layout.
	x float64
}

// shapedGlyph is a glyph that a textSink shaped: its outline in font units
// with y up, the pixels per font unit, and its offset from the origin of its
// cluster, in pixels with y down.
type shapedGlyph struct {
	path   []kurbo.PathEl
	scale  float64
	dx, dy float64
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

// renderText draws the glyphs of a text layer at frame onto sink, under
// transform and with alpha (D79). Each glyph of the list chars draws its
// outline. The sink shapes the other characters, when it is a textSink. The
// lines and the box follow updateLocalFont of ThorVG.
func renderText(animation *Composition, keyframes []TextKeyframe, frame float64, transform kurbo.Affine, alpha float64, sink Sink) {
	doc := textAt(keyframes, frame)
	if doc == nil || doc.Text == "" || !(doc.Size > 0) {
		return
	}
	font := animation.Fonts[doc.Font]
	scale := doc.Size / 100
	tracking := doc.Tracking * 0.1 * scale
	ascent := 0.0
	if font != nil {
		ascent = font.Ascent * scale
	}
	shaper, _ := sink.(textSink)
	line := 0
	for _, paragraph := range splitLines(doc.Text) {
		units, shapedAscent := textUnits(paragraph, font, doc, scale, tracking, shaper)
		if font == nil {
			ascent = shapedAscent
		}
		// Break the paragraph into lines, as updateLocalFont does for a box
		// text.
		start, cursor := 0, 0.0
		for i := 0; i <= len(units); {
			if i == len(units) {
				drawTextLine(units[start:i], cursor, line, ascent, doc, transform, alpha, sink)
				line++
				break
			}
			u := &units[i]
			if u.space && doc.BoxSize.Width > 0 && cursor+u.advance+nextWordWidth(units[i+1:]) >= doc.BoxSize.Width {
				drawTextLine(units[start:i], cursor, line, ascent, doc, transform, alpha, sink)
				line++
				i++
				start, cursor = i, 0
				continue
			}
			if !u.drawn {
				i++
				continue
			}
			if doc.BoxSize.Width > 0 && cursor > 0 && cursor+u.advance > doc.BoxSize.Width {
				drawTextLine(units[start:i], cursor, line, ascent, doc, transform, alpha, sink)
				line++
				start, cursor = i, 0
				continue
			}
			u.x = cursor
			cursor += u.advance
			i++
		}
	}
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
// shaped font in pixels, or 0.
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
				units = append(units, textUnit{space: c == ' '})
			}
		}
		for _, c := range clusters {
			u := textUnit{space: c.text == " ", drawn: len(c.glyphs) > 0, shaped: c.glyphs}
			if u.drawn {
				u.advance = c.advance + tracking
			}
			units = append(units, u)
		}
		pending = pending[:0]
	}
	for len(text) > 0 {
		if g := font.glyph(text); g != nil && g.Text != "" {
			flush()
			units = append(units, textUnit{
				space:   text[0] == ' ',
				drawn:   true,
				advance: float64(g.Width*scale) + tracking,
				glyph:   g,
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

// drawTextLine draws the units of the line number line, whose width is
// width, as updateLocalFont places a line: the left of the line at the left
// of the box plus Justify of the free space of the box, and the baseline at
// the top of the box plus the ascent, at most the height of the box, minus
// the baseline shift.
func drawTextLine(units []textUnit, width float64, line int, ascent float64, doc *TextDocument, transform kurbo.Affine, alpha float64, sink Sink) {
	ascent = math.Min(ascent, doc.BoxSize.Height)
	x0 := doc.BoxPosition.X + doc.Justify*(doc.BoxSize.Width-width)
	y0 := doc.BoxPosition.Y + ascent - doc.BaselineShift + float64(line)*doc.LineHeight
	fill := solidBrush(doc.Fill.MultiplyAlpha(float32(alpha)))
	stroke := solidBrush(doc.Stroke.MultiplyAlpha(float32(alpha)))
	scale := doc.Size / 100
	for i := range units {
		u := &units[i]
		if !u.drawn {
			continue
		}
		if g := u.glyph; g != nil {
			if len(g.Path) == 0 {
				continue
			}
			t := transform.Mul(kurbo.AffineTranslate(kurbo.NewVec2(x0+u.x, y0))).Mul(kurbo.AffineScale(scale))
			drawTextGlyph(sink, t, g.Path, doc.StrokeWidth/scale, doc.StrokeBelow, &fill, &stroke)
			continue
		}
		for _, g := range u.shaped {
			if len(g.path) == 0 || !(g.scale > 0) {
				continue
			}
			t := transform.Mul(kurbo.AffineTranslate(kurbo.NewVec2(x0+u.x+g.dx, y0+g.dy))).Mul(kurbo.AffineScaleNonUniform(g.scale, -g.scale))
			drawTextGlyph(sink, t, g.path, doc.StrokeWidth/g.scale, doc.StrokeBelow, &fill, &stroke)
		}
	}
}

// drawTextGlyph fills the outline path of a glyph under transform with
// fill, and strokes it with stroke when width is larger than 0, below the
// fill when below is true. The stroke has the round joins and the square caps
// of textShape of ThorVG.
func drawTextGlyph(sink Sink, transform kurbo.Affine, path []kurbo.PathEl, width float64, below bool, fill, stroke *Brush) {
	var st *Stroke
	if width > 0 {
		st = &Stroke{Width: width, Join: JoinRound, MiterLimit: 4, StartCap: CapSquare, EndCap: CapSquare}
	}
	if st != nil && below {
		sink.Draw(st, transform, stroke, path)
	}
	sink.Draw(nil, transform, fill, path)
	if st != nil && !below {
		sink.Draw(st, transform, stroke, path)
	}
}
