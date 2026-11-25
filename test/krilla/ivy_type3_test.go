package krilla

import (
	"bytes"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
)

// The scenes of this file are the gate of the image fonts of D89. Each one
// draws glyphs of pdf.NewImageFont with pdf.Surface.DrawGlyphs, and okage
// draws the same pixels with a pixmap for each glyph. A pixel of a glyph is
// 5 units of the page or more, because Poppler smooths an image that it
// scales up by less than 4 times (D68). TestImageFontText makes sure that
// pdftotext extracts the text of each scene, and that the reader finds the
// shape glyphs.

// imageFontUnitsPerEm is the size of an em of the image fonts of the scenes.
// A pixel of a mask glyph is one unit of the font.
const imageFontUnitsPerEm = 10

// imageFontSize is the size of the text of the scenes, so a unit of the
// font is 5 units of the page.
const imageFontSize = 50

// imageGlyphTransform maps the pixels of an image of h pixels, each s units
// of the font, onto the units of the font, with the bottom of the image on
// the baseline.
func imageGlyphTransform(s float32, h int) path.Transform {
	return path.TransformFromRow(s, 0, 0, -s, 0, s*float32(h))
}

// maskShapes returns the masks of the glyphs a, b and c: a ring, a cross and
// a triangle, of 8 by 8 pixels.
func maskShapes() []*image.Alpha {
	shapes := []func(x, y int) bool{
		func(x, y int) bool {
			return x == 0 || y == 0 || x == 7 || y == 7 || x == 1 || y == 1 || x == 6 || y == 6
		},
		func(x, y int) bool { return x == 3 || x == 4 || y == 3 || y == 4 },
		func(x, y int) bool { return x <= y },
	}
	out := make([]*image.Alpha, len(shapes))
	for i, in := range shapes {
		m := image.NewAlpha(image.Rect(0, 0, 8, 8))
		for y := range 8 {
			for x := range 8 {
				if in(x, y) {
					m.Pix[y*m.Stride+x] = 0xff
				}
			}
		}
		out[i] = m
	}
	return out
}

// colorGlyphImage returns the image of the color glyph: 4 by 4 pixels in
// four colors.
func colorGlyphImage() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	colors := []color.NRGBA{{220, 40, 40, 255}, {40, 160, 60, 255}, {40, 60, 220, 255}, {240, 200, 40, 255}}
	for y := range 4 {
		for x := range 4 {
			img.SetNRGBA(x, y, colors[(x/2)+2*(y/2)])
		}
	}
	return img
}

// imageFontGlyph is a glyph of a scene: a mask or a color image, with its
// transform in the units of the font.
type imageFontGlyph struct {
	mask      *image.Alpha
	image     *image.NRGBA
	transform path.Transform
	advance   float32
}

// imageFontLine is a line of glyphs of a scene, with its start and its text.
// Glyph i of the line is the cluster of the byte i of the text.
type imageFontLine struct {
	x, y   float32
	ids    []int
	text   string
	shader func(t *testing.T) okage.Shader
}

// imageFontScene makes a scene of the glyphs and the lines.
func imageFontScene(name string, glyphs []imageFontGlyph, lines func() []imageFontLine) ivyScene {
	return ivyScene{
		name: name,
		okage: func(t *testing.T, pm *okage.Pixmap) {
			t.Helper()
			scale := float32(imageFontSize) / imageFontUnitsPerEm
			for _, l := range lines() {
				x := l.x
				for _, id := range l.ids {
					g := glyphs[id]
					ts := path.TransformFromTranslate(x, l.y).
						PreConcat(path.TransformFromScale(scale, -scale)).
						PreConcat(g.transform)
					imageFontOkageGlyph(t, pm, g, ts, l.shader(t))
					x += float32(g.advance * scale)
				}
			}
		},
		pdf: func(t *testing.T, s *pdf.Surface) {
			t.Helper()
			font := imageFontPDF(t, glyphs)
			for _, l := range lines() {
				paint, opacity, ok := s.PaintFromShader(l.shader(t))
				if !ok {
					t.Fatal("the shader does not convert")
				}
				s.SetFill(&pdf.Fill{Paint: paint, Opacity: opacity})
				run := make([]pdf.Glyph, len(l.ids))
				for i, id := range l.ids {
					run[i] = pdf.Glyph{
						GlyphID:   pdf.GlyphID(id),
						TextRange: pdf.TextRange{Start: i, End: i + 1},
						XAdvance:  glyphs[id].advance / imageFontUnitsPerEm,
					}
				}
				s.DrawGlyphs(path.PointFromXY(l.x, l.y), run, font, l.text, imageFontSize, false)
			}
		},
	}
}

// imageFontPDF makes the image font of the glyphs.
func imageFontPDF(t *testing.T, glyphs []imageFontGlyph) *pdf.Font {
	t.Helper()
	font, err := pdf.NewImageFont(imageFontUnitsPerEm, pdf.WithImageFontName("Gate"))
	if err != nil {
		t.Fatal(err)
	}
	for i, g := range glyphs {
		ig := pdf.ImageGlyph{Advance: g.advance, Mask: g.mask, Transform: g.transform}
		if g.image != nil {
			img, err := pdf.ImageFromImage(g.image, false)
			if err != nil {
				t.Fatal(err)
			}
			ig.Mask, ig.Image = nil, img
		}
		id, err := font.AddImageGlyph(ig)
		if err != nil || int(id) != i {
			t.Fatalf("adding the glyph %d: %d, %v", i, id, err)
		}
	}
	return font
}

// imageFontOkageGlyph draws a glyph with okage: a color image as a pixmap,
// and a mask as the fill of the shader through it. The pixels are not
// smoothed.
func imageFontOkageGlyph(t *testing.T, pm *okage.Pixmap, g imageFontGlyph, ts path.Transform, shader okage.Shader) {
	t.Helper()
	pp := okage.DefaultPixmapPaint()
	pp.Quality = okage.FilterQualityNearest
	if g.image != nil {
		b := g.image.Rect
		gp, _ := okage.NewPixmap(uint32(b.Dx()), uint32(b.Dy()))
		px := gp.PixelsMut()
		for i := range b.Dx() * b.Dy() {
			c := g.image.Pix[4*i:]
			px.Set(i, okage.ColorFromRGBA8(c[0], c[1], c[2], c[3]).Premultiply().ToColorU8())
		}
		pm.DrawPixmap(0, 0, gp.AsRef(), &pp, ts, nil)
		return
	}
	b := g.mask.Rect
	sp, _ := okage.NewPixmap(uint32(b.Dx()), uint32(b.Dy()))
	px := sp.PixelsMut()
	for y := range b.Dy() {
		for x := range b.Dx() {
			c, _ := okage.PremultipliedColorU8FromRGBA(0, 0, 0, g.mask.Pix[y*g.mask.Stride+x])
			px.Set(y*b.Dx()+x, c)
		}
	}
	layer, _ := okage.NewPixmap(ivySize, ivySize)
	layer.DrawPixmap(0, 0, sp.AsRef(), &pp, ts, nil)
	mask := okage.MaskFromPixmap(layer.AsRef(), okage.MaskTypeAlpha)
	fillOkage(pm, rectPath(t, 0, 0, ivySize, ivySize), shader, mask)
}

// solid returns a shader of one color.
func solid(c okage.Color) func(*testing.T) okage.Shader {
	return func(*testing.T) okage.Shader { return okage.SolidColor(c) }
}

// maskGlyphs returns the glyphs a, b and c, each 9 units wide.
func maskGlyphs() []imageFontGlyph {
	var glyphs []imageFontGlyph
	for _, m := range maskShapes() {
		glyphs = append(glyphs, imageFontGlyph{mask: m, transform: imageGlyphTransform(1, 8), advance: 9})
	}
	return glyphs
}

// manyGlyphs returns 300 mask glyphs of 2 by 2 pixels, each 2 units wide,
// with one of the 15 patterns that fill a pixel or more.
func manyGlyphs() []imageFontGlyph {
	glyphs := make([]imageFontGlyph, 300)
	for i := range glyphs {
		m := image.NewAlpha(image.Rect(0, 0, 2, 2))
		bits := i%15 + 1
		for p := range 4 {
			if bits&(1<<p) != 0 {
				m.Pix[p] = 0xff
			}
		}
		glyphs[i] = imageFontGlyph{mask: m, transform: imageGlyphTransform(1, 2), advance: 2}
	}
	return glyphs
}

// manyText returns the text of the line i of the scene of many glyphs:
// one letter for each of its 20 glyphs.
func manyText(i int) string {
	var b strings.Builder
	for j := range 20 {
		b.WriteByte('a' + byte((i*20+j)%26))
	}
	return b.String()
}

// imageFontScenes are the scenes of the image fonts: mask glyphs in two
// colors, mask glyphs with a gradient, color glyphs with mask glyphs, and
// 300 glyphs, which take two Type 3 fonts.
func imageFontScenes() []ivyScene {
	gradient := func(t *testing.T) okage.Shader {
		t.Helper()
		st := stops([]float32{0, 1}, rgba(220, 40, 40, 255), rgba(40, 60, 220, 255))
		return must(t)(okage.NewLinearGradient(okage.PointFromXY(10, 0), okage.PointFromXY(190, 0), st,
			okage.SpreadModePad, okage.IdentityTransform()))
	}
	colorGlyphs := append(maskGlyphs(), imageFontGlyph{image: colorGlyphImage(), transform: imageGlyphTransform(2, 4), advance: 9})
	return []ivyScene{
		imageFontScene("image_font_masks", maskGlyphs(), func() []imageFontLine {
			return []imageFontLine{
				{x: 10, y: 70, ids: []int{0, 1, 2, 0}, text: "abca", shader: solid(rgba(200, 30, 30, 255))},
				{x: 10, y: 150, ids: []int{2, 1, 0, 1}, text: "cbab", shader: solid(rgba(30, 60, 200, 255))},
			}
		}),
		imageFontScene("image_font_gradient", maskGlyphs(), func() []imageFontLine {
			return []imageFontLine{
				{x: 10, y: 70, ids: []int{0, 1, 2, 0}, text: "abca", shader: gradient},
				{x: 10, y: 150, ids: []int{1, 1, 2, 2}, text: "bbcc", shader: gradient},
			}
		}),
		imageFontScene("image_font_color", colorGlyphs, func() []imageFontLine {
			return []imageFontLine{
				{x: 10, y: 70, ids: []int{3, 0, 3, 1}, text: "xaxb", shader: solid(rgba(30, 30, 30, 255))},
				{x: 10, y: 150, ids: []int{2, 3, 3, 0}, text: "cxxa", shader: solid(rgba(30, 150, 60, 255))},
			}
		}),
		imageFontScene("image_font_many", manyGlyphs(), func() []imageFontLine {
			lines := make([]imageFontLine, 15)
			for i := range lines {
				ids := make([]int, 20)
				for j := range ids {
					ids[j] = i*20 + j
				}
				lines[i] = imageFontLine{x: 0, y: float32(20 + 12*i), ids: ids, text: manyText(i), shader: solid(rgba(30, 30, 30, 255))}
			}
			return lines
		}),
	}
}

// TestImageFontText extracts the text of each scene of the image fonts with
// pdftotext, and reads the Type 3 fonts of each file with the reader.
func TestImageFontText(t *testing.T) {
	// A gradient draws each glyph on the page, with no text, because Poppler
	// fills the whole page for a shape glyph with a gradient (D89).
	want := map[string]string{
		"image_font_masks":    "abca cbab",
		"image_font_gradient": "",
		"image_font_color":    "xaxb cxxa",
	}
	var many []string
	for i := range 15 {
		many = append(many, manyText(i))
	}
	want["image_font_many"] = strings.Join(many, " ")
	fonts := map[string]int{"image_font_many": 2, "image_font_gradient": 0}
	for _, sc := range imageFontScenes() {
		t.Run(sc.name, func(t *testing.T) {
			data := ivyPDF(t, sc)
			f := checkFile(t, sc.name+".pdf", data, ivySize, ivySize)
			type3, shape := 0, 0
			for _, o := range f.Objects {
				if dict, ok := o.Value.(*pdfDict); ok && dict.get("Subtype") == pdfName("Type3") {
					type3++
				}
				if o.IsStream && bytes.Contains(o.Data, []byte(" d1\n")) {
					shape++
				}
			}
			n, ok := fonts[sc.name]
			if !ok {
				n = 1
			}
			if type3 != n {
				t.Errorf("the file has %d Type 3 fonts, want %d", type3, n)
			}
			if (shape != 0) != (n != 0) {
				t.Errorf("the file has %d shape glyphs in %d Type 3 fonts", shape, n)
			}
			file := writeTemp(t, sc.name+".pdf", data)
			if got := strings.Join(strings.Fields(pdfText(t, file)), " "); got != want[sc.name] {
				t.Errorf("pdftotext extracts %q, want %q", got, want[sc.name])
			}
		})
	}
}
