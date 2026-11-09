package svg

import (
	"bytes"
	"compress/gzip"
	"errors"
	"image"
	"image/color"
	"os"
	"sync"
	"testing"

	"github.com/xo/okage"
)

// This file tests the renderer of renderer.go (D36).

// testFonts returns the options of a renderer that loads the test fonts and
// no font of the system.
func testFonts(opts ...Option) []Option {
	return append([]Option{WithLoadSystemFonts(false), WithFontFS(os.DirFS("testdata/fonts"))}, opts...)
}

// renderByHand renders data as a program did before D36: it parses data,
// makes a pixmap, fills it and renders the tree.
func renderByHand(t *testing.T, data []byte, bg color.RGBA, opts ...Option) *image.RGBA {
	t.Helper()
	tree, err := FromData(data, opts...)
	if err != nil {
		t.Fatal(err)
	}
	size := tree.Size().ToIntSize()
	pixmap, ok := okage.NewPixmap(size.Width(), size.Height())
	if !ok {
		t.Fatal("NewPixmap failed")
	}
	img := pixmap.AsRGBA()
	for i := 0; i < len(img.Pix); i += 4 {
		copy(img.Pix[i:], []uint8{bg.R, bg.G, bg.B, bg.A})
	}
	mut := pixmap.AsMut()
	Render(tree, okage.IdentityTransform(), &mut)
	return img
}

const rendererTestSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40">
  <rect x="5" y="5" width="30" height="30" fill="seagreen" fill-opacity="0.5"/>
  <text x="40" y="28" font-size="20" font-family="No Such Font,serif">Hello</text>
</svg>`

// TestRendererMatchesFromData checks that a render gives the pixels of the
// parse and the render by hand, with the same fonts and background.
func TestRendererMatchesFromData(t *testing.T) {
	data := []byte(rendererTestSVG)
	bg := color.RGBA{R: 0x40, G: 0x40, B: 0x40, A: 0xff}
	got, err := New(testFonts(WithBackground(bg))...).Render(data)
	if err != nil {
		t.Fatal(err)
	}
	db := NewFontDatabase()
	db.LoadFontsFS(os.DirFS("testdata/fonts"), ".")
	db.SetSerifFamily("Noto Serif")
	db.SetSansSerifFamily("Noto Sans")
	want := renderByHand(t, data, bg, WithFontDatabase(db), WithFontFamily("Noto Serif"))
	if !bytes.Equal(got.Pix, want.Pix) || got.Rect != want.Rect {
		t.Error("Render differs from FromData and Render by hand")
	}
	if bytes.Equal(got.Pix, renderByHand(t, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40"><rect x="5" y="5" width="30" height="30" fill="seagreen" fill-opacity="0.5"/></svg>`), bg).Pix) {
		t.Error("Render drew no text")
	}
}

// TestRendererGenericFamilies checks the families that the renderer sets from
// the candidates, and that an option wins.
func TestRendererGenericFamilies(t *testing.T) {
	o := New(testFonts()...).options()
	for _, c := range []struct {
		kind FontFamilyKind
		want string
	}{
		{FontFamilySerif, "Noto Serif"},
		{FontFamilySansSerif, "Noto Sans"},
		// No candidate of these is a test font, so they keep the defaults of
		// fontdb.
		{FontFamilyMonospace, "Courier New"},
		{FontFamilyCursive, "Comic Sans MS"},
		{FontFamilyFantasy, "Impact"},
	} {
		if got := o.fontdb.FamilyName(FontFamily{Kind: c.kind}); got != c.want {
			t.Errorf("FamilyName(%v) = %q, want %q", c.kind, got, c.want)
		}
	}
	if o.fontFamily != "Noto Serif" {
		t.Errorf("fontFamily = %q, want Noto Serif", o.fontFamily)
	}

	o = New(testFonts(WithSerifFamily("Noto Sans"), WithFontFamily("Noto Mono"))...).options()
	if got := o.fontdb.FamilyName(FontFamily{Kind: FontFamilySerif}); got != "Noto Sans" {
		t.Errorf("FamilyName(serif) = %q, want the family of WithSerifFamily", got)
	}
	if o.fontFamily != "Noto Mono" {
		t.Errorf("fontFamily = %q, want the family of WithFontFamily", o.fontFamily)
	}

	// The renderer leaves a database of WithFontDatabase as it is.
	db := NewFontDatabase()
	db.LoadFontsFS(os.DirFS("testdata/fonts"), ".")
	o = New(WithFontDatabase(db)).options()
	if o.fontdb != db || db.FamilyName(FontFamily{Kind: FontFamilySerif}) != "Times New Roman" || o.fontFamily != "Times New Roman" {
		t.Error("the renderer changed the database of WithFontDatabase")
	}
}

// TestRendererIsLazy checks that New loads no font and applies no option.
func TestRendererIsLazy(t *testing.T) {
	r := New()
	if r.opt != nil {
		t.Error("New applied the options")
	}
	if defaultRenderer.opt != nil && !testing.Short() {
		t.Log("the default renderer was used by another test")
	}
}

// TestRendererScale checks the size and the scale of each mode, the
// transform and the errors.
func TestRendererScale(t *testing.T) {
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="50"><rect width="100" height="50" fill="red"/></svg>`)
	for _, c := range []struct {
		name string
		opts []Option
		w, h int
	}{
		{"none", nil, 100, 50},
		{"none width", []Option{WithWidth(30)}, 30, 50},
		{"best fit", []Option{WithWidth(30), WithHeight(30), WithScaleMode(ScaleBestFit)}, 30, 15},
		{"best fit height", []Option{WithHeight(25), WithScaleMode(ScaleBestFit)}, 50, 25},
		{"min width", []Option{WithWidth(200), WithScaleMode(ScaleMinWidth)}, 200, 100},
		{"min width larger", []Option{WithWidth(50), WithScaleMode(ScaleMinWidth)}, 100, 50},
		{"max height", []Option{WithHeight(10), WithScaleMode(ScaleMaxHeight)}, 20, 10},
	} {
		r := New(append([]Option{WithLoadSystemFonts(false)}, c.opts...)...)
		cfg, err := r.ParseConfig(data)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		img, err := r.Render(data)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if cfg.Width != c.w || cfg.Height != c.h || img.Rect.Dx() != c.w || img.Rect.Dy() != c.h {
			t.Errorf("%s: config %dx%d and image %v, want %dx%d", c.name, cfg.Width, cfg.Height, img.Rect, c.w, c.h)
		}
		// The rectangle fills the image in each mode.
		if got := img.RGBAAt(c.w-1, c.h-1); got != (color.RGBA{R: 255, A: 255}) {
			t.Errorf("%s: the last pixel is %v, want red", c.name, got)
		}
	}

	// A transform replaces the scale, and the size still comes from it.
	img, err := New(WithLoadSystemFonts(false), WithWidth(200), WithTransform(okage.TransformFromTranslate(50, 0))).Render(data)
	if err != nil {
		t.Fatal(err)
	}
	if img.Rect.Dx() != 200 || img.RGBAAt(49, 10).A != 0 || img.RGBAAt(149, 10).A != 255 || img.RGBAAt(151, 10).A != 0 {
		t.Error("WithTransform did not replace the scale")
	}

	if _, err := New(WithLoadSystemFonts(false), WithScaleMode(ScaleMaxWidth)).Render(data); !errors.Is(err, okage.ErrSVGInvalidRenderSize) {
		t.Errorf("a render to a width of 0 gives %v, want ErrSVGInvalidRenderSize", err)
	}
}

// TestScaleMode checks ScaleMode.Scale against the results of
// github.com/xo/resvg.
func TestScaleMode(t *testing.T) {
	for _, c := range []struct {
		mode                ScaleMode
		width, height, w, h uint
		gotW, gotH          int
		scaleX, scaleY      float32
	}{
		{ScaleNone, 100, 50, 0, 0, 100, 50, 1, 1},
		{ScaleNone, 100, 50, 200, 0, 200, 50, 2, 1},
		{ScaleNone, 100, 50, 0, 25, 100, 25, 1, 0.5},
		{ScaleMinWidth, 100, 50, 300, 0, 300, 150, 3, 3},
		{ScaleMinWidth, 100, 50, 30, 0, 100, 50, 1, 1},
		{ScaleMinHeight, 100, 50, 0, 75, 150, 75, 1.5, 1.5},
		{ScaleMaxWidth, 100, 50, 30, 0, 30, 15, 0.3, 0.3},
		{ScaleMaxWidth, 100, 50, 300, 0, 100, 50, 1, 1},
		{ScaleMaxHeight, 100, 50, 0, 20, 40, 20, 0.4, 0.4},
		{ScaleBestFit, 100, 50, 40, 40, 40, 20, 0.4, 0.4},
		{ScaleBestFit, 100, 50, 0, 10, 20, 10, 0.2, 0.2},
		{ScaleBestFit, 3, 7, 10, 0, 10, 23, 10.0 / 3, 10.0 / 3},
	} {
		w, h, sx, sy := c.mode.Scale(c.width, c.height, c.w, c.h)
		if w != c.gotW || h != c.gotH || sx != c.scaleX || sy != c.scaleY {
			t.Errorf("%d.Scale(%d, %d, %d, %d) = %d, %d, %g, %g, want %d, %d, %g, %g",
				c.mode, c.width, c.height, c.w, c.h, w, h, sx, sy, c.gotW, c.gotH, c.scaleX, c.scaleY)
		}
	}
}

// TestRendererGzip checks that a render reads SVGZ data.
func TestRendererGzip(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(rendererTestSVG)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	r := New(testFonts()...)
	got, err := r.Render(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	want, err := r.Render([]byte(rendererTestSVG))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Pix, want.Pix) {
		t.Error("the render of SVGZ data differs from the render of the plain data")
	}
}

// TestRendererConcurrent renders with one renderer from several goroutines.
// The race detector finds a write to shared state.
func TestRendererConcurrent(t *testing.T) {
	r := New(testFonts()...)
	want, err := r.Render([]byte(rendererTestSVG))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			got, err := r.Render([]byte(rendererTestSVG))
			if err != nil {
				t.Error(err)
				return
			}
			if !bytes.Equal(got.Pix, want.Pix) {
				t.Error("a concurrent render differs")
			}
		})
	}
	wg.Wait()
}

// TestRendererDocument checks that a document gives its tree, its size and
// the same image as Render, more than once.
func TestRendererDocument(t *testing.T) {
	r := New(testFonts()...)
	doc, err := r.Parse([]byte(rendererTestSVG))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Tree() == nil {
		t.Fatal("Tree() = nil")
	}
	cfg, err := doc.Config()
	if err != nil || cfg.Width != 120 || cfg.Height != 40 || cfg.ColorModel != color.RGBAModel {
		t.Errorf("Config() = %+v, %v, want 120x40 in RGBA", cfg, err)
	}
	a, err := doc.Render()
	if err != nil {
		t.Fatal(err)
	}
	b, err := doc.Render()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Error("two renders of a document differ")
	}
	if _, err := r.Parse([]byte("not svg")); err == nil {
		t.Error("Parse of data that is not SVG gives no error")
	}
}

// TestDecode checks that image.Decode finds the format svg, and renders with
// the fonts of the system.
func TestDecode(t *testing.T) {
	data := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" width="10" height="20"><rect width="10" height="20" fill="blue"/></svg>`)
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "svg" || cfg.Width != 10 || cfg.Height != 20 {
		t.Fatalf("DecodeConfig = %+v, %q, %v, want 10x20 svg", cfg, format, err)
	}
	img, format, err := image.Decode(bytes.NewReader(data[len(`<?xml version="1.0"?>`):]))
	if err != nil || format != "svg" {
		t.Fatalf("Decode = %q, %v, want svg", format, err)
	}
	if got := color.RGBAModel.Convert(img.At(5, 5)); got != (color.RGBA{B: 255, A: 255}) {
		t.Errorf("At(5, 5) = %v, want blue", got)
	}
}
