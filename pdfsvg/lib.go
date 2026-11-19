// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfsvg

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
	"github.com/xo/okage/roxmltree"
	"github.com/xo/okage/svg"
)

// This file ports krilla-svg src/lib.rs. The documentation of the crate is
// the package comment in doc.go.

// svgSettings is SvgSettings in krilla-svg src/lib.rs. The options
// WithEmbedText and WithFilterScale set its fields (D20).
//
// Settings that should be applied when converting a SVG.
type svgSettings struct {
	// embedText is whether text should be embedded as properly selectable
	// text. Otherwise, it will be drawn as outlined paths instead.
	embedText bool
	// filterScale is how much filters, which will be converted to bitmaps,
	// should be scaled. Higher values mean better quality, but also bigger
	// file sizes.
	filterScale float32
}

// defaultSVGSettings is the impl Default for SvgSettings in krilla-svg
// src/lib.rs.
func defaultSVGSettings() svgSettings {
	return svgSettings{
		embedText:   true,
		filterScale: 4.0,
	}
}

// Option is an option of Draw. Each option sets a field of SvgSettings of
// krilla-svg (D20).
type Option func(*svgSettings)

// WithEmbedText is the field embed_text of SvgSettings in krilla-svg
// src/lib.rs. The default is true.
//
// Whether text should be embedded as properly selectable text. Otherwise,
// it will be drawn as outlined paths instead.
func WithEmbedText(embed bool) Option {
	return func(s *svgSettings) {
		s.embedText = embed
	}
}

// WithFilterScale is the field filter_scale of SvgSettings in krilla-svg
// src/lib.rs. The default is 4. The raster fallback of a group that PDF
// cannot express takes the same scale (D62).
//
// How much filters, which will be converted to bitmaps, should be scaled.
// Higher values mean better quality, but also bigger file sizes.
func WithFilterScale(scale float32) Option {
	return func(s *svgSettings) {
		s.filterScale = scale
	}
}

// Draw is SurfaceExt::draw_svg in krilla-svg src/lib.rs. It returns false
// where upstream returns None. Upstream always returns Some, and so does
// Draw, except for a tree whose size gives no rectangle, where upstream
// panics.
//
// Draw a tree of svg onto a surface with the given size and settings.
func Draw(s *pdf.Surface, tree *svg.Tree, size pdf.Size, opts ...Option) bool {
	settings := defaultSVGSettings()
	for _, opt := range opts {
		opt(&settings)
	}

	// okage: krilla-svg unwraps the rectangle of the tree, and panics for a
	// size that gives none. Draw returns false first (D62).
	r, ok := path.RectFromXYWH(0, 0, tree.Size().Width(), tree.Size().Height())
	if !ok {
		return false
	}

	oldFill := s.GetFill()
	oldStroke := s.GetStroke()

	transform := path.TransformFromScale(
		size.Width()/tree.Size().Width(),
		size.Height()/tree.Size().Height(),
	)
	s.PushTransform(transform)
	s.PushClipPath(rectToClipPath(r), pdf.FillRuleNonZero)
	renderTree(tree, settings, s)
	s.Pop()
	s.Pop()

	s.SetFill(oldFill)
	s.SetStroke(oldStroke)

	return true
}

// processContext is ProcessContext in krilla-svg src/lib.rs.
type processContext struct {
	fonts       *fonts
	svgSettings svgSettings
}

// newProcessContext is ProcessContext::new in krilla-svg src/lib.rs.
func newProcessContext(treeFontDB *svg.FontDatabase, settings svgSettings) *processContext {
	return &processContext{
		fonts:       newFonts(treeFontDB),
		svgSettings: settings,
	}
}

// renderTree is render_tree in krilla-svg src/lib.rs. Upstream clones the
// font database of the tree, because fontdb needs a mutable database to
// share the data of a face. The database of svg does not, so the port
// takes the database of the tree.
func renderTree(tree *svg.Tree, settings svgSettings, surface *pdf.Surface) {
	fc := newProcessContext(tree.FontDB(), settings)
	renderRoot(tree.Root(), surface, fc)
}

// renderNodeLib is render_node in krilla-svg src/lib.rs. It takes the suffix
// of its module, because group.rs has a function of the same name (D10).
func renderNodeLib(node svg.Node, treeFontDB *svg.FontDatabase, settings svgSettings, surface *pdf.Surface) {
	fc := newProcessContext(treeFontDB, settings)
	// okage: the node is drawn alone, so a group is the root of its own
	// blending, as renderRoot says (D62).
	if g, ok := node.(*svg.Group); ok {
		renderRoot(g, surface, fc)
		return
	}
	renderNodeGroup(node, surface, fc)
}

// RenderGlyph is render_svg_glyph in krilla-svg src/lib.rs. It has the type
// pdf.SVGGlyphFunc, and a program gives it to pdf.WithSVGGlyphs. It returns
// false where upstream returns None. Upstream panics where width or height
// is not positive, and RenderGlyph returns false.
//
// Render an SVG glyph from an OpenType font into a surface. You can plug
// this function into pdf.WithSVGGlyphs.
func RenderGlyph(data []byte, contextColor pdf.ColorRGB, glyph pdf.GlyphID, width, height float32, surface *pdf.Surface) bool {
	settings := defaultSVGSettings()

	defaultSize, ok := path.SizeFromWH(width, height)
	if !ok {
		return false
	}

	if bytes.HasPrefix(data, []byte{0x1f, 0x8b}) {
		decoded, err := gunzip(data)
		if err != nil {
			return false
		}
		data = decoded
	}

	if !utf8.Valid(data) {
		return false
	}
	xml := string(data)
	// Incredibly hacky, but hopefully that's enough for SVG glyphs.
	hasViewBox := strings.Contains(xml, "viewBox")
	document, err := roxmltree.Parse(xml)
	if err != nil {
		return false
	}

	// Reparsing every time might be pretty slow in some cases, because Noto
	// Color Emoji for example contains hundreds of glyphs in the same SVG
	// document, meaning that we have to reparse it every time. However,
	// Twitter Color Emoji does have each glyph in a separate SVG document,
	// and since we use COLRv1 for Noto Color Emoji anyway, this is good
	// enough.
	tree, err := svg.FromXMLTree(document,
		svg.WithStyleSheet(fmt.Sprintf("svg { color: rgb(%d, %d, %d) }",
			contextColor.Red(), contextColor.Green(), contextColor.Blue())),
		svg.WithDefaultSize(defaultSize),
	)
	if err != nil {
		return false
	}

	applyScale := defaultSize != tree.Size() && hasViewBox

	// From the specification:
	//
	// The size of the initial viewport for the SVG document is the em
	// square: height and width both equal to head.unitsPerEm. If a viewBox
	// attribute is specified on the <svg> element with width or height
	// values different from the unitsPerEm value, this will have the effect
	// of a scale transformation on the SVG “user” coordinate system.
	if applyScale {
		scale := min(defaultSize.Width()/tree.Size().Width(),
			defaultSize.Height()/tree.Size().Height())
		surface.PushTransform(path.TransformFromScale(scale, scale))
	}

	if node, ok := tree.NodeByID(fmt.Sprintf("glyph%d", uint32(glyph))); ok {
		renderNodeLib(node, tree.FontDB(), settings, surface)
	} else {
		// Twitter Color Emoji SVGs contain the glyph ID on the root element,
		// which isn't saved by usvg. So in this case, we simply draw the
		// whole document.
		renderTree(tree, settings, surface)
	}

	if applyScale {
		surface.Pop()
	}

	return true
}

// gunzip is flate2::read::GzDecoder with read_to_end, which RenderGlyph
// calls for a compressed document. GzDecoder reads one member of a gzip
// file, so the reader does too.
func gunzip(data []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("reading the gzip header: %w", err)
	}
	zr.Multistream(false)
	b, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("decompressing the document: %w", err)
	}
	return b, nil
}
