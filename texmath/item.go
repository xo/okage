package texmath

import (
	"github.com/xo/okage"
	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
)

// This file ports no file of go-tex/math. It holds the result of the layout
// of D78 section 3: a Box of items, with Draw for okage and Path and ItemPath
// for a writer of SVG. The source writes SVG text in its place.

// Box is a formula laid out. Its origin is the left end of its baseline, and
// y grows down, as in okage. Every length is in the unit of the size of the
// layout.
type Box struct {
	Width  float64 // the advance width
	Height float64 // the extent above the baseline
	Depth  float64 // the extent below the baseline
	Italic float64 // the italic correction of the last glyph, which Width includes (WithItalicCorrection)
	Items  []Item
}

// ItemKind is the kind of an Item.
type ItemKind uint8

// The kinds of an Item.
const (
	ItemGlyph ItemKind = iota // one glyph of a face
	ItemRule                  // a filled rectangle
	ItemPath                  // a filled path, such as the head of an arrow
)

// Item is one thing that a Box draws.
type Item struct {
	Kind      ItemKind
	Face      *font.Face      // ItemGlyph
	Glyph     font.GID        // ItemGlyph
	Size      float64         // ItemGlyph: the em size of the glyph
	X, Y      float64         // the origin of a glyph or a path, or the top left corner of a rule
	W, H      float64         // ItemRule
	Path      *okage.Path     // ItemPath, relative to X and Y
	Text      string          // ItemGlyph: the characters of the glyph, for a text layer. Empty on a part of an assembly after the first
	Color     okage.Color     // the color of the fill
	Transform okage.Transform // applied after X and Y. The identity, except under \scalebox and \reflectbox
}

// Draw fills each item into p, moved by t. paint gives the anti-aliasing and
// the blend mode, and the color of each item replaces its shader. paint is
// not changed.
func (b *Box) Draw(p *okage.Pixmap, t okage.Transform, paint *okage.Paint) {
	pt := *paint
	for i := range b.Items {
		path := b.ItemPath(i)
		if path == nil {
			continue
		}
		pt.SetColor(b.Items[i].Color)
		p.FillPath(path, &pt, okage.FillRuleWinding, t, nil)
	}
}

// Path returns the outline of every item as one path, in the unit of the
// size, with the baseline at y = 0. It reads the outlines from the faces of
// the items, and needs no FontDatabase. It returns nil for a box that draws
// nothing.
//
// The path is filled with the nonzero rule. A rule and the head of an arrow
// go clockwise on the screen, as the outer contours of a CFF font do, so an
// arrow head adds to the shaft under it.
func (b *Box) Path() *okage.Path {
	pb := okage.NewPathBuilder()
	for i := range b.Items {
		b.appendItem(pb, i)
	}
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// ItemPath returns the outline of the item i, for a caller that colors each
// item, such as a writer of SVG for a formula with colors. It returns nil when
// the item draws nothing or i is out of range.
func (b *Box) ItemPath(i int) *okage.Path {
	if i < 0 || i >= len(b.Items) {
		return nil
	}
	pb := okage.NewPathBuilder()
	b.appendItem(pb, i)
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// appendItem adds the outline of the item i to pb.
func (b *Box) appendItem(pb *okage.PathBuilder, i int) {
	it := &b.Items[i]
	switch it.Kind {
	case ItemGlyph:
		appendGlyph(pb, it)
	case ItemRule:
		if !(it.W > 0 && it.H > 0) {
			return
		}
		m := pointMapper{it: it}
		x0, y0 := m.apply(0, 0)
		x1, y1 := m.apply(it.W, 0)
		x2, y2 := m.apply(it.W, it.H)
		x3, y3 := m.apply(0, it.H)
		pb.MoveTo(x0, y0)
		pb.LineTo(x1, y1)
		pb.LineTo(x2, y2)
		pb.LineTo(x3, y3)
		pb.Close()
	case ItemPath:
		if it.Path == nil {
			return
		}
		ts := it.Transform.PreTranslate(float32(it.X), float32(it.Y))
		if p, ok := it.Path.Transform(ts); ok {
			pb.PushPath(p)
		}
	}
}

// pointMapper maps a point relative to the origin of an item to the space
// of the box: it adds X and Y, and applies the transform of the item.
type pointMapper struct{ it *Item }

func (m pointMapper) apply(x, y float64) (float32, float32) {
	x += m.it.X
	y += m.it.Y
	t := m.it.Transform
	if t.IsIdentity() {
		return float32(x), float32(y)
	}
	// The products are converted, so that the compiler does not fuse them
	// with the sums (D16).
	tx := float64(float64(t.SX)*x) + float64(float64(t.KX)*y) + float64(t.TX)
	ty := float64(float64(t.KY)*x) + float64(float64(t.SY)*y) + float64(t.TY)
	return float32(tx), float32(ty)
}

// appendGlyph adds the outline of a glyph item to pb. The outline is in font
// units with y up, so it is scaled by the size over the units per em, and
// flipped. A contour ends where the next one starts, so a contour is closed
// before each move and after the last segment.
func appendGlyph(pb *okage.PathBuilder, it *Item) {
	if it.Face == nil {
		return
	}
	outline, ok := it.Face.GlyphDataOutline(it.Glyph)
	if !ok || len(outline.Segments) == 0 {
		return
	}
	s := it.Size / float64(it.Face.Upem())
	m := pointMapper{it: it}
	pt := func(p ot.SegmentPoint) (float32, float32) {
		return m.apply(float64(float64(p.X)*s), -float64(float64(p.Y)*s))
	}
	for i, seg := range outline.Segments {
		a := seg.Args
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			if i != 0 {
				pb.Close()
			}
			x, y := pt(a[0])
			pb.MoveTo(x, y)
		case ot.SegmentOpLineTo:
			x, y := pt(a[0])
			pb.LineTo(x, y)
		case ot.SegmentOpQuadTo:
			x1, y1 := pt(a[0])
			x, y := pt(a[1])
			pb.QuadTo(x1, y1, x, y)
		case ot.SegmentOpCubeTo:
			x1, y1 := pt(a[0])
			x2, y2 := pt(a[1])
			x, y := pt(a[2])
			pb.CubicTo(x1, y1, x2, y2, x, y)
		}
	}
	pb.Close()
}
