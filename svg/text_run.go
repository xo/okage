package svg

import (
	"slices"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports no upstream file. It holds the text API of D37: ShapeText
// lays out one run of text with the layout of text of usvg, and returns its
// size and its outlines, so that a program can draw text without SVG data.
// It builds the same Text node that the parser builds for a <text> element
// with the same attributes, and calls the same layout and the same flatten.
//
// D57 adds the stretch, the variations, the features, the language and the
// spacing of a run to TextStyle, and the clusters of a run.

// TextStyle is the font of a run of text for ShapeText.
type TextStyle struct {
	// Family is a list of font families, as the CSS property font-family
	// gives it, such as `"Noto Sans", Arial, sans-serif`. The empty string is
	// the default family of the options, as for a <text> element without
	// font-family.
	Family string

	// Size is the font size in user units. It must be larger than 0.
	Size float32

	// Weight is the font weight, from 1 to 1000. 0 is 400, the normal
	// weight.
	Weight uint16

	// Style is the font style.
	Style FontStyle

	// Stretch is the font stretch, as a percentage of the normal width, as
	// the CSS property font-stretch gives it. It goes from 50 to 200. 0 is
	// 100, the normal width. The query of the font takes the keyword of
	// font-stretch that is nearest, such as semi-condensed for 87.5.
	Stretch float32

	// Variations are the values of the axes of a variable font, as the CSS
	// property font-variation-settings gives them. ShapeText adds the axes
	// wght, wdth, ital and slnt for the weight, the stretch and the style,
	// when Variations does not set them, as AutoFontVariations does.
	Variations []FontVariation

	// Features are the OpenType features of the run, as the CSS property
	// font-feature-settings gives them, such as tnum with the value 1. The
	// value 0 turns a feature off, such as liga or kern.
	Features []FontFeature

	// Language is the language of the text, as a BCP 47 tag such as "tr" or
	// "zh-Hant". The shaper selects the glyphs of a font for that language.
	// The empty string is no language, as for a <text> element.
	Language string

	// LetterSpacing is the space in user units that ShapeText adds after
	// each cluster but the last, as the CSS property letter-spacing does. A
	// script whose letters join, such as Arabic, gets no letter spacing.
	LetterSpacing float32

	// WordSpacing is the space in user units that ShapeText adds after each
	// space between words, as the CSS property word-spacing does.
	WordSpacing float32
}

// FontFeature is an OpenType feature of a run of text and its value, such as
// the feature liga with the value 0, which turns the ligatures off.
//
// Port: usvg has no such type. ShapeText gives the features to the shaper
// (D57).
type FontFeature struct {
	// Tag is the tag of the feature, such as liga.
	Tag [4]byte

	// Value is the value of the feature. 0 turns the feature off, and 1
	// turns it on. A feature with alternates, such as salt, takes the number
	// of an alternate.
	Value uint32
}

// TextCluster is a cluster of a TextRun: the characters that the shaper draws
// together, with their glyphs. y grows down, as in SVG.
//
// Port: usvg has no such type. TextRun.Clusters gives the clusters (D57).
type TextCluster struct {
	// Start and End are the range of the cluster in the bytes of the text
	// of the run.
	Start int
	End   int

	// X is the position of the cluster on the baseline, from the origin of
	// the run, in user units.
	X float32

	// Advance is the advance of the cluster in user units, with the letter
	// spacing and the word spacing.
	Advance float32

	// Font is the face of the glyphs of the cluster. A cluster can take a
	// face of the fallback, and so the clusters of one run can have other
	// faces.
	Font FontID

	// Glyphs are the glyphs of the cluster. The method Transform of each
	// glyph places it in the run. Glyphs is empty when a negative letter
	// spacing removed the glyphs of the cluster.
	Glyphs []PositionedGlyph
}

// TextRun is a run of text that ShapeText laid out. Its origin is the start
// of the run on the baseline. y grows down, as in SVG.
type TextRun struct {
	text     *Text
	opt      *options
	clusters []TextCluster
}

// ShapeText lays out text in one line, from an origin at (0, 0) on the
// baseline, with the fonts, the fallback and the shaping of the parser and
// the options opts. Use WithFontDatabase for the fonts, as for FromData.
//
// The text is laid out as it is. A <text> element of SVG data collapses its
// white space first, so pass text without leading, trailing or repeated
// spaces to get the same result as the element.
//
// ShapeText returns okage.ErrSVGTextInvalidSize when style.Size is not larger
// than 0, and okage.ErrSVGTextNoFont when no font of the database draws the
// text. An empty text gives an empty run.
func ShapeText(text string, style TextStyle, opts ...Option) (*TextRun, error) {
	return shapeTextRun(text, style, newOptions(opts...))
}

// ShapeText lays out text as the package function ShapeText does, with the
// options and the font database of the renderer. The first call of the
// renderer loads the fonts of the system, as Render does.
func (r *Renderer) ShapeText(text string, style TextStyle) (*TextRun, error) {
	opt := *r.options()
	opt.fontdbOwned = false
	return shapeTextRun(text, style, &opt)
}

// shapeTextRun is ShapeText with the options already made.
func shapeTextRun(text string, style TextStyle, opt *options) (*TextRun, error) {
	fontSize, ok := path.NewNonZeroPositiveF32(style.Size)
	if !ok {
		return nil, okage.ErrSVGTextInvalidSize
	}
	if text == "" {
		return &TextRun{}, nil
	}

	families, err := svgtypes.ParseFontFamilies(style.Family)
	if err != nil || len(families) == 0 {
		families = []FontFamily{{Kind: FontFamilyNamed, Name: opt.fontFamily}}
	}
	weight := style.Weight
	switch {
	case weight == 0:
		weight = 400
	case weight > 1000:
		weight = 1000
	}
	stretch := stretchFromPercent(style.Stretch)
	font := Font{
		families:   families,
		style:      style.Style,
		stretch:    stretch,
		weight:     weight,
		variations: autoFontVariations(slices.Clone(style.Variations), weight, stretch, style.Style),
	}

	// The span and the chunk of a <text x="0" y="0"> element with these
	// attributes and no other.
	fill := DefaultFill()
	n := utf8.RuneCountInString(text)
	var zero float32
	t := &Text{
		renderingMode: opt.textRendering,
		dx:            make([]float32, n),
		dy:            make([]float32, n),
		rotate:        make([]float32, n),
		writingMode:   WritingModeLeftToRight,
		chunks: []TextChunk{{
			x:      &zero,
			y:      &zero,
			anchor: TextAnchorStart,
			spans: []TextSpan{{
				start:             0,
				end:               len(text),
				fill:              &fill,
				paintOrder:        PaintOrderFillAndStroke,
				font:              font,
				fontSize:          fontSize,
				applyKerning:      true,
				fontOpticalSizing: FontOpticalSizingAuto,
				visible:           true,
				dominantBaseline:  DominantBaselineAuto,
				alignmentBaseline: AlignmentBaselineAuto,
				lengthAdjust:      LengthAdjustSpacing,
				letterSpacing:     style.LetterSpacing,
				wordSpacing:       style.WordSpacing,
				features:          slices.Clone(style.Features),
				language:          style.Language,
			}},
			textFlow: TextFlow{Kind: TextFlowLinear},
			text:     text,
		}},
		absTransform: path.IdentityTransform(),
		flattened:    emptyGroup(),
		keepClusters: true,
	}

	fonts := newFontCache(opt.fontdb, opt)
	layouted, bbox, ok := layoutText(t, &opt.fontResolver, fonts, opt)
	if !ok {
		return nil, okage.ErrSVGTextNoFont
	}
	t.layouted = layouted
	t.boundingBox = bbox.ToRect()
	t.absBoundingBox = t.boundingBox
	group, strokeBBox, ok := flatten(t, fonts)
	if !ok {
		return nil, okage.ErrSVGTextNoFont
	}
	t.flattened = group
	t.strokeBoundingBox = strokeBBox.ToRect()
	t.absStrokeBoundingBox = t.strokeBoundingBox
	clusters := t.clusters
	t.keepClusters, t.clusters = false, nil
	return &TextRun{text: t, opt: opt, clusters: logicalClusters(clusters, len(text))}, nil
}

// stretchFromPercent returns the keyword of font-stretch that is nearest to
// the percentage p of TextStyle.Stretch. 0 is the normal width.
func stretchFromPercent(p float32) FontStretch {
	if p <= 0 {
		return FontStretchNormal
	}
	percents := [...]float32{50, 62.5, 75, 87.5, 100, 112.5, 125, 150, 200}
	best := FontStretchNormal
	for i, v := range percents {
		if abs32(p-v) < abs32(p-percents[best]) {
			best = FontStretch(i)
		}
	}
	return best
}

// newTextCluster returns the cluster of the run for a cluster of the layout,
// whose glyphs have their transforms. spanTs is the transform of its span.
func newTextCluster(gc *glyphCluster, spanTs Transform) TextCluster {
	ts := spanTs.PreConcat(gc.fullTransform())
	c := TextCluster{
		Start:   gc.byteIdx.value(),
		X:       ts.TX,
		Advance: gc.advance,
		Glyphs:  gc.glyphs,
	}
	if len(gc.glyphs) != 0 {
		c.Font = gc.glyphs[0].Font
	}
	return c
}

// logicalClusters sorts the clusters of the layout, which are in visual
// order, into the order of the text, and sets the end of each one to the
// start of the next. The last one ends at n, the length of the text.
func logicalClusters(clusters []TextCluster, n int) []TextCluster {
	slices.SortStableFunc(clusters, func(a, b TextCluster) int { return a.Start - b.Start })
	for i := range clusters {
		clusters[i].End = n
		if i+1 < len(clusters) {
			clusters[i].End = clusters[i+1].Start
		}
	}
	return clusters
}

// Width returns the advance width of the run, from the metrics of its
// glyphs. It is the width of Bounds.
func (r *TextRun) Width() float32 {
	if r.text == nil {
		return 0
	}
	return r.text.boundingBox.Width()
}

// Bounds returns the box of the run from the metrics of its glyphs: from the
// origin to the advance width, and from the ascent above the baseline to the
// descent below it, as Text.BoundingBox gives it.
func (r *TextRun) Bounds() Rect {
	if r.text == nil {
		return Rect{}
	}
	return r.text.boundingBox
}

// Clusters returns the clusters of the run in the order of the text, with the
// range of bytes, the position, the advance and the glyphs of each one. The
// caller must not change them. An empty run has no clusters.
func (r *TextRun) Clusters() []TextCluster {
	return r.clusters
}

// AutoFontVariations returns variations with the axes of a variable font
// for the CSS properties font-weight, font-stretch and font-style, as usvg
// sets them for a <text> element. It adds each axis that variations does not
// set:
//
//   - wght, the weight.
//   - wdth, the percentage of the stretch, when the stretch is not normal.
//   - ital with the value 1, when the style is italic.
//   - slnt with the value -12, when the style is oblique.
//
// A program that makes a face with FontDatabase.NewFace gets the same axes
// as ShapeText and the parser. AutoFontVariations does not change
// variations.
func AutoFontVariations(variations []FontVariation, weight uint16, stretch FontStretch, style FontStyle) []FontVariation {
	return autoFontVariations(slices.Clone(variations), weight, stretch, style)
}

// Text returns the Text node of the run, with its layout and its flattened
// group, or nil for an empty run.
func (r *TextRun) Text() *Text {
	return r.text
}

// AppendPath appends the outlines of the glyphs of the run, and of its
// underline and overline, to pb, with the origin of the run at (x, y). Fill
// the path with the nonzero rule. A color glyph, such as an emoji of a color
// font, has no outline here. Draw it with RenderColorGlyphs. AppendPath does
// not change the run, so a program can call it more than once.
func (r *TextRun) AppendPath(pb *path.PathBuilder, x, y float32) {
	if r.text == nil {
		return
	}
	g := r.text.flattened
	ts := path.TransformFromTranslate(x, y).PreConcat(g.Transform())
	for _, child := range g.Children() {
		// The outlines are the paths of the group. Each color glyph is a
		// group of its own (D39).
		if c, ok := child.(*Path); ok {
			if p, ok := c.Data().Clone().Transform(ts); ok {
				pb.PushPath(p)
			}
		}
	}
}

// HasColorGlyphs reports if the run has a glyph that a font draws as an image
// or in color, from the tables COLR, SVG, CBDT or sbix of the font.
func (r *TextRun) HasColorGlyphs() bool {
	if r.text == nil {
		return false
	}
	for _, child := range r.text.flattened.Children() {
		if _, ok := child.(*Group); ok {
			return true
		}
	}
	return false
}

// RenderColorGlyphs draws the color glyphs of the run onto pixmap, with the
// origin of the run at the origin of transform. It draws nothing for the
// other glyphs, which AppendPath gives. A program that fills the outlines and
// then calls RenderColorGlyphs with the same origin draws the whole run.
func (r *TextRun) RenderColorGlyphs(pixmap *okage.PixmapMut, transform Transform) {
	if r.text == nil {
		return
	}
	g := r.text.flattened
	ctx := &context{maxBBox: maxFilterBBox(pixmap.Width(), pixmap.Height()), opt: r.opt}
	ts := transform.PreConcat(g.Transform())
	for _, child := range g.Children() {
		if c, ok := child.(*Group); ok {
			renderNode(c, ctx, ts, pixmap)
		}
	}
}
