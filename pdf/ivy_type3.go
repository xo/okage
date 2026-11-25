// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"
	"image"
	"math"
	"sync"
	"sync/atomic"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// This file ports no file of krilla. It holds the image fonts of D89: a
// *Font whose glyphs are images that the caller gives, which DrawGlyphs
// draws into Type 3 fonts with their text. The device of ivy needs them for
// the bitmap glyphs of the Type 3 fonts of the files that it reads, and for
// the color glyphs of its layout.

// maxImageGlyphs is the largest number of glyphs of an image font. A glyph
// ID of a run of DrawGlyphs is at most 65535 (CheckGlyphs).
const maxImageGlyphs = 1 << 16

// imageFontCount gives each image font its number, from which the hash of
// the font comes. So two image fonts never share their Type 3 fonts in a
// document.
var imageFontCount atomic.Uint64

// ImageFontOption is an option of NewImageFont (D20).
type ImageFontOption func(*imageFontOptions)

// imageFontOptions holds the options of NewImageFont.
type imageFontOptions struct {
	name string
}

// WithImageFontName sets the name of the Type 3 fonts of an image font in
// the file. The default is "unknown", as krilla names a font without a
// PostScript name. A name of more than 115 bytes is cut, as krilla cuts the
// name of a Type 3 font.
func WithImageFontName(name string) ImageFontOption {
	return func(o *imageFontOptions) {
		o.name = name
	}
}

// ImageGlyph is one glyph of an image font. Exactly one of Mask and Image is
// set.
type ImageGlyph struct {
	// Advance is the advance of the glyph, in the units of the font.
	Advance float32
	// Mask is a stencil of 0 and 255. A pixel of 255 takes the fill of the
	// text. The glyph is a shape glyph, with the operator d1. A mask with
	// no pixel draws nothing, as a space does.
	Mask *image.Alpha
	// Image is the image of a color glyph, with the operator d0. The glyph
	// keeps the colors of the image.
	Image *Image
	// Transform maps the pixels of the mask or of the image, with y down,
	// onto the units of the font, with y up. The pixel 0, 0 is the top left
	// corner of the image.
	Transform Transform
}

// imageGlyph is a glyph of an image font as the font keeps it.
type imageGlyph struct {
	advance   float32
	mask      *imageMask
	image     *Image
	size      Size
	transform Transform
	// bbox is the area of the glyph in the units of the font. hasBBox is
	// false for a glyph that draws nothing.
	bbox    Rect
	hasBBox bool
}

// imageFont is the part of a *Font of NewImageFont.
type imageFont struct {
	unitsPerEm float32

	// mu guards glyphs and bbox, because a program can add glyphs while
	// another goroutine draws with the font.
	mu     sync.Mutex
	glyphs []imageGlyph
	// bbox holds the areas of the glyphs, and the em square.
	bbox Rect
}

// imageFontKey is the value whose hash is the hash of an image font.
type imageFontKey uint64

// typeID names the type for sipHash.
func (imageFontKey) typeID() string { return "imageFontKey" }

// hash writes the number of the font.
func (k imageFontKey) hash(h *hasher) {
	h.u64(uint64(k))
}

// NewImageFont returns a font whose glyphs are images that the caller gives
// with AddImageGlyph. unitsPerEm is the size of an em in the units of the
// glyphs. DrawGlyphs draws its glyphs into Type 3 fonts, with the text that
// a reader can copy or search. The font has no character map, so DrawText
// does not draw it. It is not in krilla (D89).
//
// It returns an error that wraps okage.ErrPDFInvalidFont for a unitsPerEm
// that is not finite or not above 0.
func NewImageFont(unitsPerEm float32, opts ...ImageFontOption) (*Font, error) {
	if !(unitsPerEm > 0) || math.IsInf(float64(unitsPerEm), 1) {
		return nil, fmt.Errorf("%w: the units per em %v of an image font are not finite and above 0", okage.ErrPDFInvalidFont, unitsPerEm)
	}
	var o imageFontOptions
	for _, opt := range opts {
		opt(&o)
	}
	em, ok := path.RectFromXYWH(0, 0, unitsPerEm, unitsPerEm)
	if !ok {
		return nil, fmt.Errorf("%w: the units per em %v of an image font are too large", okage.ErrPDFInvalidFont, unitsPerEm)
	}
	f := &Font{
		img: &imageFont{unitsPerEm: unitsPerEm, bbox: em},
		info: &fontInfo{
			postscriptName:    o.name,
			hasPostscriptName: o.name != "",
			weight:            400,
			stretch:           1,
			globalBBox:        em,
		},
	}
	f.hash = sipHash(imageFontKey(imageFontCount.Add(1)))
	return f, nil
}

// AddImageGlyph adds a glyph to an image font, and returns its ID. The IDs
// count from 0, in the order of the calls. A program adds a glyph before it
// draws it, and can add glyphs until Document.Finish. The font copies the
// mask, and keeps the image. It is not in krilla (D89).
//
// It returns an error that wraps okage.ErrPDFInvalidGlyph for a glyph with
// both or neither of Mask and Image, for an advance or a transform that is
// not finite, for a mask with a pixel that is not 0 or 255, and when the
// font already holds 65,536 glyphs. It returns an error that wraps
// okage.ErrPDFImage for a mask that is larger than the limit of
// Surface.FillStencil, and okage.ErrPDFInvalidFont for a font of NewFont.
func (f *Font) AddImageGlyph(g ImageGlyph) (GlyphID, error) {
	if f.img == nil {
		return 0, fmt.Errorf("%w: AddImageGlyph needs a font of NewImageFont", okage.ErrPDFInvalidFont)
	}
	if (g.Mask == nil) == (g.Image == nil) {
		return 0, fmt.Errorf("%w: an image glyph needs exactly one of a mask and an image", okage.ErrPDFInvalidGlyph)
	}
	if !isFiniteF32(g.Advance) || !g.Transform.IsFinite() {
		return 0, fmt.Errorf("%w: the advance or the transform of an image glyph is not finite", okage.ErrPDFInvalidGlyph)
	}
	ig := imageGlyph{advance: g.Advance, transform: g.Transform}
	if g.Mask != nil {
		w, h := g.Mask.Rect.Dx(), g.Mask.Rect.Dy()
		if err := checkImageBytes(w, h, 1); err != nil {
			return 0, imageError(err)
		}
		if !isBinaryStencil(g.Mask) {
			return 0, fmt.Errorf("%w: the mask of an image glyph has a pixel that is not 0 or 255", okage.ErrPDFInvalidGlyph)
		}
		if !g.Mask.Rect.Empty() {
			m := newImageMask(g.Mask, false)
			ig.mask = &m
			ig.size, _ = path.SizeFromWH(float32(w), float32(h))
		}
	} else {
		w, h := g.Image.Size()
		ig.image = g.Image
		ig.size, ig.hasBBox = path.SizeFromWH(float32(w), float32(h))
		if !ig.hasBBox {
			ig.image = nil
		}
	}
	if ig.mask != nil || ig.image != nil {
		r, _ := ig.size.ToRect(0, 0)
		ig.bbox, ig.hasBBox = r.Transform(ig.transform)
	}

	f.img.mu.Lock()
	defer f.img.mu.Unlock()
	if len(f.img.glyphs) >= maxImageGlyphs {
		return 0, fmt.Errorf("%w: an image font holds at most %d glyphs", okage.ErrPDFInvalidGlyph, maxImageGlyphs)
	}
	if ig.hasBBox {
		rectExpand(&f.img.bbox, ig.bbox)
	}
	f.img.glyphs = append(f.img.glyphs, ig)
	return GlyphID(len(f.img.glyphs) - 1), nil
}

// isFiniteF32 reports whether v is neither infinite nor NaN.
func isFiniteF32(v float32) bool {
	return !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0)
}

// glyph returns a glyph of the font, or false for an ID that the font does
// not hold.
func (f *imageFont) glyph(id GlyphID) (imageGlyph, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if uint64(id) >= uint64(len(f.glyphs)) {
		return imageGlyph{}, false
	}
	return f.glyphs[id], true
}

// fontBBox returns the areas of the glyphs, with the em square.
func (f *imageFont) fontBBox() Rect {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bbox
}

// drawImageGlyph draws a glyph of an image font onto the surface.
// baseTransform maps the units of the font, with y up, onto the surface, as
// drawColorGlyph takes it. It fills a mask through FillStencil, with the
// fill of the surface. It returns false for a glyph that the font does not
// hold, or that draws nothing.
func drawImageGlyph(font *Font, glyph GlyphID, baseTransform Transform, surface *Surface) bool {
	g, ok := font.img.glyph(glyph)
	if !ok || !g.hasBBox {
		return false
	}
	ts := baseTransform.PreConcat(g.transform)
	surface.PushTransform(ts)
	if g.mask != nil {
		// The paint of the fill stays in the space of the text, as the
		// paint of an outline of drawOutlineGlyph does.
		saved := surface.fill
		if saved != nil {
			if inv, ok := ts.Invert(); ok {
				fill := *saved
				fill.Paint = paintPreConcat(fill.Paint, inv)
				surface.fill = &fill
			}
		}
		surface.FillStencil(g.mask.alpha(), g.size, false)
		surface.fill = saved
	} else {
		surface.DrawImage(g.image, g.size)
	}
	surface.Pop()
	return true
}

// paintPreConcat returns the paint with ts before its transform. A color has
// no transform.
func paintPreConcat(p Paint, ts Transform) Paint {
	switch q := p.(type) {
	case LinearGradient:
		q.Transform = ts.PreConcat(q.Transform)
		return q
	case RadialGradient:
		q.Transform = ts.PreConcat(q.Transform)
		return q
	case SweepGradient:
		q.Transform = ts.PreConcat(q.Transform)
		return q
	case Pattern:
		q.Transform = ts.PreConcat(q.Transform)
		return q
	case MeshGradient:
		q.Transform = ts.PreConcat(q.Transform)
		return q
	}
	return p
}

// alpha returns the stencil of the mask, with 255 for each pixel that the
// viewer fills.
func (m imageMask) alpha() *image.Alpha {
	a := image.NewAlpha(image.Rect(0, 0, m.width, m.height))
	row := (m.width + 7) / 8
	for y := range m.height {
		for x := range m.width {
			if m.bits[y*row+x/8]&(0x80>>(x%8)) == 0 {
				a.Pix[y*a.Stride+x] = 0xff
			}
		}
	}
	return a
}

// isMaskGlyph reports whether the font holds the glyph as a mask glyph,
// which is a shape glyph in its Type 3 font.
func (f *imageFont) isMaskGlyph(id GlyphID) bool {
	g, ok := f.glyph(id)
	return ok && g.image == nil
}

// writeShapeGlyph writes the content of a mask glyph of an image font: the
// operator d1 with the width and the bbox of the glyph, and the image mask,
// with no color. The viewer fills it with the fill of the text. It returns
// the bbox of the glyph in the units of the font, or false for a glyph that
// draws nothing.
func writeShapeGlyph(font *Font, glyph GlyphID, sc *serializeContext, cc *chunkContainer, rd *resourceDictionaryBuilder, content *pdfwriter.Content) (Rect, bool) {
	g, _ := font.img.glyph(glyph)
	if !g.hasBBox {
		content.StartShapeGlyph(g.advance, 0, 0, 0, 0)
		return Rect{}, false
	}
	b := g.bbox
	content.StartShapeGlyph(g.advance, b.Left(), b.Top(), b.Right(), b.Bottom())
	content.SaveState()
	// Map the unit square of the image onto its pixels, with the first row
	// at the top, and the pixels onto the units of the font.
	unit := path.TransformFromRow(g.size.Width(), 0, 0, -g.size.Height(), 0, g.size.Height())
	content.Transform(transformToPDF(g.transform.PreConcat(unit)))
	name := rd.registerResource(registerResourceable(sc, cc, *g.mask))
	content.XObject(pdfwriter.Name(name))
	content.RestoreState()
	return b, true
}

// drawImageFontGlyphs draws the glyphs of an image font as DrawGlyphs draws
// text with only a fill. A Type 3 font is always filled, so a stroke without
// a fill gives its paint and its opacity to the fill.
//
// Poppler 26.11 fills the whole page when a gradient or a pattern fills a
// shape glyph of an image mask. So a fill that is not a color draws each
// glyph as an image on the page, through FillStencil for a mask, and the
// run has no text.
func (s *Surface) drawImageFontGlyphs(start Point, glyphs []Glyph, font *Font, text string, fontSize float32) {
	fill := DefaultFill()
	switch {
	case s.fill != nil:
		fill = *s.fill
	case s.stroke != nil:
		fill.Paint = s.stroke.Paint
		fill.Opacity = s.stroke.Opacity
	}
	if _, ok := fill.Paint.(Color); !ok {
		saved := s.fill
		s.fill = &fill
		s.outlineGlyphs(glyphs, ColorRGB{}, start, font, fontSize)
		s.fill = saved
		return
	}
	s.bd.getMut().drawGlyphs(start, s.sc, s.cc, &fill, nil, ColorRGB{}, glyphs, font, text, fontSize)
}
