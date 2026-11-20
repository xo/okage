// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports no file of velato. It holds the option of the PixmapSink
// that gives the fonts of a text layer, and the shaping of the characters
// that the list chars of a file does not hold (D79).

import (
	"strings"

	"golang.org/x/image/math/fixed"

	"github.com/xo/okage/kurbo"
	"github.com/xo/okage/svg"
	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/language"
	"github.com/xo/okage/typesetting/shaping"
)

// WithFontDatabase sets the fonts that the sink draws the characters of a
// text layer with, when the list chars of the file does not hold their
// glyph. The sink takes the face whose PostScript name is the name of the
// font of the text document, or else the face of the family and the style
// of that font in the font list of the file. Without this option, such a
// character draws nothing, and the sink logs it to the logger of WithLogger.
// velato has no such field, because it does not draw text (D79).
func WithFontDatabase(db *svg.FontDatabase) SinkOption {
	return func(s *PixmapSink) {
		s.fonts = db
	}
}

// textFace is a face of the font database, with its scale.
type textFace struct {
	face   *font.Face
	upem   float64
	ascent float64
}

// warnGlyphs logs each character of text, which the sink cannot draw with
// the font name.
func (s *PixmapSink) warnGlyphs(name string, text []rune) {
	if s.logger == nil {
		return
	}
	for _, c := range text {
		if c == ' ' {
			continue
		}
		s.logger.Warn("lottie: no glyph for a character of a text layer", "font", name, "char", string(c))
	}
}

// textFace returns the face of the font database for the font name, family
// and style, or false. It keeps each face for the next texts of the sink.
func (s *PixmapSink) textFace(name, family, style string) (*textFace, bool) {
	key := name + "\x00" + family + "\x00" + style
	if f, ok := s.faces[key]; ok {
		return f, f != nil
	}
	if s.faces == nil {
		s.faces = map[string]*textFace{}
	}
	f := s.loadTextFace(name, family, style)
	s.faces[key] = f
	return f, f != nil
}

// loadTextFace finds and loads the face of textFace, or returns nil.
func (s *PixmapSink) loadTextFace(name, family, style string) *textFace {
	id, found := svg.FontID(0), false
	for face := range s.fonts.Faces() {
		if name != "" && face.PostScriptName == name {
			id, found = face.ID, true
			break
		}
	}
	if !found {
		q := svg.FontQuery{
			Families: []svg.FontFamily{{Kind: svg.FontFamilyNamed, Name: family}},
			Weight:   fontWeight(style),
			Stretch:  svg.FontStretchNormal,
			Style:    svg.FontStyleNormal,
		}
		lower := strings.ToLower(style)
		switch {
		case strings.Contains(lower, "italic"):
			q.Style = svg.FontStyleItalic
		case strings.Contains(lower, "oblique"):
			q.Style = svg.FontStyleOblique
		}
		if id, found = s.fonts.Query(q); !found {
			return nil
		}
	}
	face, ok := s.fonts.NewFace(id, nil)
	if !ok {
		return nil
	}
	upem := float64(face.Upem())
	if !(upem > 0) {
		return nil
	}
	f := &textFace{face: face, upem: upem}
	if ext, ok := face.FontHExtents(); ok {
		f.ascent = float64(ext.Ascender)
	}
	return f
}

// fontWeight returns the weight of the name of a style, such as 700 for
// "Bold Italic". A style without a known word has the weight 400.
func fontWeight(style string) uint16 {
	s := strings.ToLower(strings.NewReplacer(" ", "", "-", "", "_", "").Replace(style))
	for _, w := range []struct {
		name   string
		weight uint16
	}{
		{"extralight", 200}, {"ultralight", 200}, {"semibold", 600}, {"demibold", 600},
		{"extrabold", 800}, {"ultrabold", 800}, {"thin", 100}, {"hairline", 100},
		{"light", 300}, {"medium", 500}, {"bold", 700}, {"black", 900}, {"heavy", 900},
	} {
		if strings.Contains(s, w.name) {
			return w.weight
		}
	}
	return 400
}

// shapeText shapes text with the face of textFace at size pixels, from the
// left to the right, as a textSink does. A cluster with a glyph that the face
// does not have draws nothing, and the sink logs it.
func (s *PixmapSink) shapeText(name, family, style string, text []rune, size float64) ([]shapedCluster, float64, bool) {
	if s.fonts == nil {
		s.warnGlyphs(name, text)
		return nil, 0, false
	}
	f, ok := s.textFace(name, family, style)
	if !ok {
		s.warnGlyphs(name, text)
		return nil, 0, false
	}
	script := language.Common
	for _, c := range text {
		if sc := language.LookupScript(c); sc.Strong() {
			script = sc
			break
		}
	}
	// Shape at the size of one em in font units, so that each value is in
	// font units.
	out := (&shaping.HarfbuzzShaper{}).Shape(shaping.Input{
		Text:      text,
		RunStart:  0,
		RunEnd:    len(text),
		Direction: di.DirectionLTR,
		Face:      f.face,
		Size:      fixed.Int26_6(f.upem * 64),
		Script:    script,
	})
	k := size / f.upem
	var clusters []shapedCluster
	pen := 0.0
	for i := 0; i < len(out.Glyphs); {
		first := out.Glyphs[i]
		start, n := first.TextIndex(), max(first.RunesCount(), 1)
		c := shapedCluster{text: string(text[start:min(start+n, len(text))])}
		missing := false
		x := pen
		for ; i < len(out.Glyphs) && out.Glyphs[i].TextIndex() == start; i++ {
			g := out.Glyphs[i]
			if g.GlyphID == 0 {
				missing = true
			}
			c.glyphs = append(c.glyphs, shapedGlyph{
				path:  glyphPath(f.face, g.GlyphID),
				scale: k,
				dx:    (x + fromFixed(g.XOffset) - pen) * k,
				dy:    -fromFixed(g.YOffset) * k,
			})
			x += fromFixed(g.Advance)
		}
		c.advance = (x - pen) * k
		pen = x
		if missing {
			s.warnGlyphs(name, []rune(c.text))
			c.glyphs = nil
		}
		clusters = append(clusters, c)
	}
	return clusters, f.ascent * k, true
}

// fromFixed converts a fixed point value of 26.6 bits.
func fromFixed(v fixed.Int26_6) float64 {
	return float64(v) / 64
}

// glyphPath returns the outline of the glyph gid of face, in font units with
// y up, with each contour closed, or nil for a glyph without an outline.
func glyphPath(face *font.Face, gid font.GID) []kurbo.PathEl {
	outline, ok := face.GlyphData(gid).(font.GlyphOutline)
	if !ok {
		return nil
	}
	var path []kurbo.PathEl
	point := func(p font.SegmentPoint) kurbo.Point {
		return kurbo.NewPoint(float64(p.X), float64(p.Y))
	}
	for _, seg := range outline.Segments {
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			if len(path) > 0 {
				path = append(path, kurbo.ClosePath())
			}
			path = append(path, kurbo.MoveTo(point(seg.Args[0])))
		case ot.SegmentOpLineTo:
			path = append(path, kurbo.LineTo(point(seg.Args[0])))
		case ot.SegmentOpQuadTo:
			path = append(path, kurbo.QuadTo(point(seg.Args[0]), point(seg.Args[1])))
		case ot.SegmentOpCubeTo:
			path = append(path, kurbo.CurveTo(point(seg.Args[0]), point(seg.Args[1]), point(seg.Args[2])))
		}
	}
	if len(path) > 0 {
		path = append(path, kurbo.ClosePath())
	}
	return path
}
