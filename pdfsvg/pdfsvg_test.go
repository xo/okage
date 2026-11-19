// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfsvg

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
	"github.com/xo/okage/svg"
)

// The tests of this file draw SVG files of the tests of resvg with Draw. The
// test module compares each file with krilla-svg in test/krilla, through
// the oracle (D22, D62). These tests make sure that each part of the port
// writes a file, and that a file never makes Draw panic.

// testsDir is the folder of the tests of resvg.
var testsDir = filepath.Join("..", "svg", "testdata", "tests")

// fontDB holds the font of the tests of resvg that the texts of these tests
// use.
var fontDB = sync.OnceValue(func() *svg.FontDatabase {
	db := svg.NewFontDatabase()
	data, err := os.ReadFile(filepath.Join("..", "svg", "testdata", "fonts", "NotoSans-Regular.ttf"))
	if err != nil {
		panic(err)
	}
	db.LoadFontData(data)
	db.SetSansSerifFamily("Noto Sans")
	db.SetSerifFamily("Noto Sans")
	return db
})

// drawFile parses SVG data, draws the tree onto a page of its size, and
// returns the file, without compression, so that a test can search its
// objects.
func drawFile(tb testing.TB, data []byte, dir string, opts ...Option) ([]byte, error) {
	tb.Helper()
	tree, err := svg.FromData(data, svg.WithResourcesDir(dir), svg.WithFontDatabase(fontDB()))
	if err != nil {
		return nil, err
	}
	d := pdf.NewDocument(pdf.WithCompression(false), pdf.WithPretty(true), pdf.WithSVGGlyphs(RenderGlyph))
	ps, ok := pdf.PageSettingsFromWH(tree.Size().Width(), tree.Size().Height())
	if !ok {
		tb.Fatalf("the size %v gives no page", tree.Size())
	}
	page := d.StartPage(ps)
	s := page.Surface()
	if !Draw(s, tree, tree.Size(), opts...) {
		tb.Fatal("Draw returns false")
	}
	s.Finish()
	page.Finish()
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// TestDraw draws a file for each part of the port, and looks for the
// objects that the part writes.
func TestDraw(t *testing.T) {
	for _, c := range []struct {
		file string
		want []string
	}{
		{"shapes/rect/simple-case", []string{"h\nf\n", "/Type /Page"}},
		{"painting/stroke-dasharray/comma-ws-separator", []string{" d\n"}},
		{"paint-servers/linearGradient/default-attributes", []string{"/ShadingType 2"}},
		{"paint-servers/radialGradient/fr=0.5", []string{"/ShadingType 3"}},
		{"paint-servers/pattern/simple-case", []string{"/PatternType 1"}},
		{"masking/mask/simple-case", []string{"/SMask", "/S /Luminosity"}},
		{"masking/clipPath/simple-case", []string{"W\nn\n"}},
		{"masking/clipPath/clip-path-on-child", []string{"/S /Alpha"}},
		{"painting/mix-blend-mode/multiply", []string{"/BM /Multiply"}},
		{"painting/mix-blend-mode/plus-lighter", []string{"/Subtype /Image"}},
		{"structure/image/embedded-png", []string{"/Subtype /Image"}},
		{"structure/image/embedded-svg", []string{"W\nn\n"}},
		{"filters/feGaussianBlur/simple-case", []string{"/Subtype /Image"}},
		{"text/text/simple-case", []string{"/CIDFontType2", "/ToUnicode"}},
	} {
		t.Run(c.file, func(t *testing.T) {
			name := filepath.Join(testsDir, filepath.FromSlash(c.file)+".svg")
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			out, err := drawFile(t, data, filepath.Dir(name))
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range c.want {
				if !bytes.Contains(out, []byte(w)) {
					t.Errorf("the file has no %q", w)
				}
			}
		})
	}
}

// TestDrawOutlined draws text with WithEmbedText(false), which draws the
// glyphs as paths and embeds no font.
func TestDrawOutlined(t *testing.T) {
	name := filepath.Join(testsDir, "text", "text", "simple-case.svg")
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	out, err := drawFile(t, data, filepath.Dir(name), WithEmbedText(false), WithFilterScale(1))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("/CIDFontType2")) {
		t.Error("the text is embedded as a font")
	}
}

// TestNeedsPlusLighter finds the groups that the raster fallback draws for
// the blend mode PlusLighter.
func TestNeedsPlusLighter(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		want bool
	}{
		{"none", `<rect width="10" height="10" style="mix-blend-mode:multiply"/>`, false},
		{"child", `<rect width="10" height="10" style="mix-blend-mode:plus-lighter"/>`, true},
		{"in a group", `<g><rect width="10" height="10"/><g style="mix-blend-mode:plus-lighter"><rect width="5" height="5"/></g></g>`, true},
		{"isolated", `<g opacity="0.5"><rect width="10" height="10"/><rect width="5" height="5" style="mix-blend-mode:plus-lighter"/></g>`, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			tree, err := svg.FromStr(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10">` + c.body + `</svg>`)
			if err != nil {
				t.Fatal(err)
			}
			if got := needsPlusLighter(tree.Root()); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// TestRenderGlyph draws the documents of SVG glyphs, compressed and not,
// with and without the ID of the glyph, as a table SVG holds them.
func TestRenderGlyph(t *testing.T) {
	doc := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` +
		`<rect id="glyph7" width="50" height="50" fill="currentColor"/></svg>`
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write([]byte(doc)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name   string
		data   []byte
		glyph  pdf.GlyphID
		upem   float32
		want   bool
		inPage string
	}{
		{"plain", []byte(doc), 7, 1000, true, " l\n"},
		{"gzip", gz.Bytes(), 7, 1000, true, " l\n"},
		{"whole document", []byte(doc), 8, 1000, true, " l\n"},
		{"bad gzip", []byte{0x1f, 0x8b, 0}, 7, 1000, false, ""},
		{"not UTF-8", []byte{0xff, 0xfe}, 7, 1000, false, ""},
		{"not XML", []byte("<svg"), 7, 1000, false, ""},
		{"no units per em", []byte(doc), 7, 0, false, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := pdf.NewDocument(pdf.WithCompression(false), pdf.WithPretty(true))
			ps, _ := pdf.PageSettingsFromWH(100, 100)
			page := d.StartPage(ps)
			s := page.Surface()
			if got := RenderGlyph(c.data, pdf.NewColorRGB(255, 0, 0), c.glyph, c.upem, c.upem, s); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
			s.Finish()
			page.Finish()
			var buf bytes.Buffer
			if err := d.Finish(&buf); err != nil {
				t.Fatal(err)
			}
			if c.inPage != "" && !bytes.Contains(buf.Bytes(), []byte(c.inPage)) {
				t.Errorf("the page has no %q", c.inPage)
			}
		})
	}
}

// TestRectToClipPath makes sure that the clip path of a rectangle has its
// four corners.
func TestRectToClipPath(t *testing.T) {
	r, _ := path.RectFromXYWH(1, 2, 3, 4)
	p := rectToClipPath(r)
	if b := p.Bounds(); b != r {
		t.Errorf("the bounds are %v, want %v", b, r)
	}
}

// FuzzDraw parses SVG data and draws the tree. Draw must not panic or hang
// for any tree that svg parses, and the document must finish. The seeds are
// one file in ten of the tests of resvg.
func FuzzDraw(f *testing.F) {
	i := 0
	err := filepath.WalkDir(testsDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".svg") {
			return nil
		}
		i++
		if i%10 != 0 {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		f.Add(data)
		return nil
	})
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		tree, err := svg.FromData(data, svg.WithFontDatabase(fontDB()))
		if err != nil {
			return
		}
		// A page of PDF is at most 14400 units on each side, and a large
		// tree only takes longer.
		size := tree.Size()
		if size.Width() > 2000 || size.Height() > 2000 {
			return
		}
		d := pdf.NewDocument(pdf.WithSVGGlyphs(RenderGlyph))
		ps, ok := pdf.PageSettingsFromWH(size.Width(), size.Height())
		if !ok {
			return
		}
		page := d.StartPage(ps)
		s := page.Surface()
		Draw(s, tree, size, WithFilterScale(1))
		s.Finish()
		page.Finish()
		var buf bytes.Buffer
		_ = d.Finish(&buf)
	})
}
