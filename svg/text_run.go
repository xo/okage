package svg

import (
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
}

// TextRun is a run of text that ShapeText laid out. Its origin is the start
// of the run on the baseline. y grows down, as in SVG.
type TextRun struct {
	text *Text
	opt  *options
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
	font := Font{
		families:   families,
		style:      style.Style,
		stretch:    FontStretchNormal,
		weight:     weight,
		variations: autoFontVariations(nil, weight, FontStretchNormal, style.Style),
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
			}},
			textFlow: TextFlow{Kind: TextFlowLinear},
			text:     text,
		}},
		absTransform: path.IdentityTransform(),
		flattened:    emptyGroup(),
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
	return &TextRun{text: t, opt: opt}, nil
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
