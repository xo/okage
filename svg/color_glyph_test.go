package svg

import (
	"encoding/binary"
	"image/color"
	"slices"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// This file tests FontDatabase.ColorGlyph of color_glyph.go (D57).

// renderColorGlyph draws the tree of a color glyph at the size s with its
// origin at (x, y) onto a new pixmap of w by h pixels.
func renderColorGlyph(t *testing.T, tree *Tree, s, x, y float32, w, h uint32) *okage.Pixmap {
	t.Helper()
	pm, ok := okage.NewPixmap(w, h)
	if !ok {
		t.Fatal("no pixmap")
	}
	m := pm.AsMut()
	Render(tree, path.TransformFromRow(s, 0, 0, s, x, y), &m)
	return pm
}

// maxDiff returns the largest difference of a byte of a and of b.
func maxDiff(a, b []byte) int {
	d := 0
	for i := range a {
		d = max(d, abs(int(a[i])-int(b[i])))
	}
	return d
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// TestColorGlyph checks that ColorGlyph draws a glyph of COLRv1 and a glyph
// of the table SVG as RenderColorGlyphs of ShapeText draws them.
func TestColorGlyph(t *testing.T) {
	for _, c := range []struct {
		file  string
		emoji rune
	}{
		{"NotoColorEmojiCOLR.subset.ttf", '\U0001F436'},
		{"TwitterColorEmoji.subset.ttf", '\U0001F600'},
	} {
		t.Run(c.file, func(t *testing.T) {
			db, ids := testFontDB(t, c.file)
			emoji := c.emoji
			g, ok := db.NominalGlyph(ids[0], emoji)
			if !ok {
				t.Fatal("no glyph for the emoji")
			}
			tree, ok := db.ColorGlyph(ids[0], g, ColorGlyphStyle{})
			if !ok {
				t.Fatal("ColorGlyph is false")
			}
			if s := tree.Size(); s.Width() != 1 || s.Height() != 1 {
				t.Errorf("Size() = %v, want 1 by 1", s)
			}
			got := renderColorGlyph(t, tree, 20, 10, 30, 40, 40)

			face, _ := db.Face(ids[0])
			family := `"` + face.Families[0].Name + `"`
			run, err := ShapeText(string(emoji), TextStyle{Family: family, Size: 20}, WithFontDatabase(db))
			if err != nil {
				t.Fatal(err)
			}
			want, _ := okage.NewPixmap(40, 40)
			wm := want.AsMut()
			run.RenderColorGlyphs(&wm, path.TransformFromTranslate(10, 30))

			if !slices.ContainsFunc(want.Data(), func(b byte) bool { return b != 0 }) {
				t.Fatal("RenderColorGlyphs drew nothing")
			}
			// The scale of the tree is 20 times 1/upem, and that of the
			// run is 20/upem, so an edge can differ by a little.
			if d := maxDiff(got.Data(), want.Data()); d > 2 {
				t.Errorf("ColorGlyph differs from RenderColorGlyphs by %d", d)
			}
		})
	}
}

// TestColorGlyphNone checks that ColorGlyph is false for a glyph without
// color and for a face that does not exist.
func TestColorGlyphNone(t *testing.T) {
	db, ids := testFontDB(t, "NotoSans-Regular.ttf", "NotoColorEmojiCBDT.subset.ttf")
	g, _ := db.NominalGlyph(ids[0], 'A')
	if _, ok := db.ColorGlyph(ids[0], g, ColorGlyphStyle{}); ok {
		t.Error("ColorGlyph is true for an outline")
	}
	g, ok := db.NominalGlyph(ids[1], '\U0001F600')
	if !ok {
		t.Fatal("no glyph for the emoji")
	}
	if _, ok := db.ColorGlyph(ids[1], g, ColorGlyphStyle{}); ok {
		t.Error("ColorGlyph is true for a bitmap glyph of CBDT")
	}
	if _, ok := db.ColorGlyph(ids[1]+1, 1, ColorGlyphStyle{}); ok {
		t.Error("ColorGlyph is true for a face that does not exist")
	}
}

// colrV0 returns a table COLR of version 0 where the base glyph base is one
// layer: the glyph layer in the color entry index of the palette.
func colrV0(base, layer, index uint16) []byte {
	b := binary.BigEndian
	var t []byte
	t = b.AppendUint16(t, 0)  // version
	t = b.AppendUint16(t, 1)  // numBaseGlyphRecords
	t = b.AppendUint32(t, 14) // baseGlyphRecordsOffset
	t = b.AppendUint32(t, 20) // layerRecordsOffset
	t = b.AppendUint16(t, 1)  // numLayerRecords
	t = b.AppendUint16(t, base)
	t = b.AppendUint16(t, 0) // firstLayerIndex
	t = b.AppendUint16(t, 1) // numLayers
	t = b.AppendUint16(t, layer)
	t = b.AppendUint16(t, index)
	return t
}

// TestColorGlyphPalette checks a glyph of COLRv0 with each palette of CPAL,
// with a palette that the font does not have, and with the color of the
// text.
func TestColorGlyphPalette(t *testing.T) {
	db, ids := testFontDB(t, "NotoSans-Regular.ttf")
	g, _ := db.NominalGlyph(ids[0], 'H')
	pf, ok := db.withFaceData(ids[0])
	if !ok {
		t.Fatal("no font")
	}
	red := tables.ColorRecord{Red: 255, Alpha: 255}
	blue := tables.ColorRecord{Blue: 255, Alpha: 255}
	for _, c := range []struct {
		name    string
		index   uint16
		palette uint16
		fg      color.Color
		want    [4]byte
	}{
		{"default palette", 0, 0, nil, [4]byte{255, 0, 0, 255}},
		{"second palette", 0, 1, nil, [4]byte{0, 0, 255, 255}},
		{"missing palette", 0, 9, nil, [4]byte{255, 0, 0, 255}},
		{"foreground", 0xFFFF, 1, color.RGBA{G: 255, A: 255}, [4]byte{0, 255, 0, 255}},
		{"black foreground", 0xFFFF, 0, nil, [4]byte{0, 0, 0, 255}},
		{"missing entry", 7, 0, color.RGBA{G: 255, A: 255}, [4]byte{0, 255, 0, 255}},
	} {
		t.Run(c.name, func(t *testing.T) {
			colr, err := tables.ParseCOLR(colrV0(uint16(g), uint16(g), c.index))
			if err != nil {
				t.Fatal(err)
			}
			f := *pf.font
			f.COLR = &colr
			f.CPAL = font.CPAL{{red}, {blue}}
			old := pf.font
			pf.font = &f
			defer func() { pf.font = old }()

			tree, ok := db.ColorGlyph(ids[0], g, ColorGlyphStyle{Palette: c.palette, Foreground: c.fg})
			if !ok {
				t.Fatal("ColorGlyph is false")
			}
			pm := renderColorGlyph(t, tree, 40, 4, 36, 40, 40)
			data := pm.Data()
			found := false
			for i := 0; i < len(data); i += 4 {
				if px := [4]byte(data[i : i+4]); px[3] == 255 {
					if px != c.want {
						t.Fatalf("a pixel is %v, want %v", px, c.want)
					}
					found = true
				}
			}
			if !found {
				t.Error("no opaque pixel")
			}
		})
	}
}
