package svg

import (
	"image/color"

	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
	"github.com/xo/okage/typesetting/font"
)

// This file ports no upstream file. It gives a program the color glyphs of
// the tables COLR and SVG of a face as a Tree, which the flatten of the
// layout of text draws inside svg (D32, D39, D57).

// ColorGlyphStyle holds the inputs of FontDatabase.ColorGlyph.
type ColorGlyphStyle struct {
	// Variations are the values of the axes of a variable font, for a glyph
	// of COLRv1 whose paints vary. A glyph of the table SVG does not use
	// them.
	Variations []FontVariation

	// Palette is the index of the palette of the table CPAL for a glyph of
	// COLR. 0 is the default palette. A palette that the font does not have
	// is the default palette.
	Palette uint16

	// Foreground is the color of the text. A glyph of COLR takes it for the
	// palette entry 0xFFFF, and for an entry that the palette does not have.
	// nil is black, as for a <text> element.
	Foreground color.Color
}

// ColorGlyph returns the color glyph of the face id as a tree, for a glyph
// of the table COLR, version 0 or 1, or of the table SVG. It returns false
// for another glyph, such as a glyph with an outline only or a bitmap glyph
// of the tables CBDT and sbix, and when the face does not exist.
//
// The tree is in em units, and y grows down. The origin of the glyph is at
// (0, 0) on the baseline, and the size of the tree is 1 by 1. To draw the
// glyph at the font size s with its origin at (x, y), call Render with the
// transform TransformFromRow(s, 0, 0, s, x, y). The tree draws the glyph
// with the colors of the font, as the renderer draws it in a <text>
// element. A glyph of COLR comes first when the font has both tables, as in
// the renderer.
//
// Each call parses the glyph again and returns a new tree, which the caller
// can keep.
func (db *FontDatabase) ColorGlyph(id FontID, glyph GlyphID, style ColorGlyphStyle) (*Tree, bool) {
	pf, ok := db.withFaceData(id)
	if !ok {
		return nil, false
	}
	opt := newOptions()
	scale := 1 / float32(unitsPerEmOf(pf))

	var node Node
	var ts Transform
	if pf.font.COLR != nil {
		face := font.NewFace(pf.font)
		if key := filterVariations(pf, style.Variations); len(key) != 0 {
			face.SetVariations(toGoTextVariations(key))
		}
		if t, ok := colr(pf, face, glyph, int(style.Palette), foregroundColor(style.Foreground), opt); ok {
			// The outlines of COLR are in font units with y up, as
			// PositionedGlyph.ColrTransform maps them.
			node, ts = &t.root, path.TransformFromScale(scale, -scale)
		}
	}
	if node == nil {
		if n, ok := svg(pf, glyph, opt); ok {
			// An SVG glyph is in font units with y down, as
			// PositionedGlyph.SvgTransform maps it.
			node, ts = n, path.TransformFromScale(scale, scale)
		}
	}
	if node == nil {
		return nil, false
	}

	g := emptyGroup()
	g.transform = ts
	g.absTransform = ts
	g.children = []Node{node}
	size, _ := path.SizeFromWH(1, 1)
	tree := &Tree{size: size, root: emptyGroupValue()}
	tree.root.children = []Node{g}
	tree.collectPaintServers()
	tree.root.collectClipPaths(&tree.clipPaths)
	tree.root.collectMasks(&tree.masks)
	tree.root.collectFilters(&tree.filters)
	return tree, true
}

// foregroundColor returns the color of ColorGlyphStyle.Foreground, or black
// for nil.
func foregroundColor(c color.Color) svgtypes.Color {
	if c == nil {
		return svgtypes.NewColorRGBA(0, 0, 0, 255)
	}
	n, _ := color.NRGBAModel.Convert(c).(color.NRGBA)
	return svgtypes.NewColorRGBA(n.R, n.G, n.B, n.A)
}
