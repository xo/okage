// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"errors"
	"image"
	"image/color"
	"math"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// The tests of this file check the objects of the image fonts of D89. The
// test module renders them with MuPDF and Poppler, in
// test/krilla/ivy_test.go.

// testMask returns a stencil of w by h pixels with a filled frame.
func testMask(w, h int) *image.Alpha {
	m := image.NewAlpha(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			if x == 0 || y == 0 || x == w-1 || y == h-1 {
				m.Pix[y*m.Stride+x] = 0xff
			}
		}
	}
	return m
}

// testImageFont returns an image font of 10 units per em with a mask glyph
// 0, a color glyph 1 and an empty glyph 2.
func testImageFont(t *testing.T, opts ...ImageFontOption) *Font {
	t.Helper()
	f, err := NewImageFont(10, opts...)
	if err != nil {
		t.Fatal(err)
	}
	// A pixel is one unit, and the top of the image is at 8 units.
	ts := path.TransformFromRow(1, 0, 0, -1, 1, 8)
	if id, err := f.AddImageGlyph(ImageGlyph{Advance: 9, Mask: testMask(7, 8), Transform: ts}); err != nil || id != 0 {
		t.Fatalf("adding the mask glyph: %d, %v", id, err)
	}
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for i := range 16 {
		src.Set(i%4, i/4, color.NRGBA{R: 0, G: 0x80, B: 0xff, A: 0xff})
	}
	img, err := ImageFromImage(src, false)
	if err != nil {
		t.Fatal(err)
	}
	ts2 := path.TransformFromRow(2, 0, 0, -2, 0, 8)
	if id, err := f.AddImageGlyph(ImageGlyph{Advance: 10, Image: img, Transform: ts2}); err != nil || id != 1 {
		t.Fatalf("adding the color glyph: %d, %v", id, err)
	}
	if id, err := f.AddImageGlyph(ImageGlyph{Advance: 4, Mask: image.NewAlpha(image.Rectangle{}), Transform: ts}); err != nil || id != 2 {
		t.Fatalf("adding the empty glyph: %d, %v", id, err)
	}
	return f
}

// imageFontRun returns the glyphs of the text "ab c", with the glyphs 0, 1, 2
// and 0.
func imageFontRun() []Glyph {
	ids := []GlyphID{0, 1, 2, 0}
	adv := []float32{0.9, 1, 0.4, 0.9}
	glyphs := make([]Glyph, len(ids))
	for i := range ids {
		glyphs[i] = Glyph{GlyphID: ids[i], TextRange: TextRange{Start: i, End: i + 1}, XAdvance: adv[i]}
	}
	return glyphs
}

func TestImageFont(t *testing.T) {
	f := testImageFont(t, WithImageFontName("Bitmap"))
	start := path.PointFromXY(10, 50)
	file := mustIvyFile(t, func(s *Surface) {
		s.SetFill(redFill(t, 1))
		s.DrawGlyphs(start, imageFontRun(), f, "ab c", 20, false)
		// Another color takes the same codes.
		fill := &Fill{Paint: NewColorRGB(0, 0, 255), Opacity: NormalizedF32One}
		s.SetFill(fill)
		s.DrawGlyphs(path.PointFromXY(10, 100), imageFontRun(), f, "ab c", 20, false)
	})
	wantMatch(t, file,
		`/Subtype /Type3`,
		`/Name /Bitmap`,
		`/FirstChar 0`, `/LastChar 2`,
		`/Widths \[9 10 4\]`,
		`/FontMatrix \[0\.1 0 0 0\.1 0 0\]`,
		`9 0 1 0 8 8 d1`,
		`4 0 0 0 0 0 d1`,
		`10 0 d0`,
		`/ImageMask true`,
		`<00> 1 beginbfchar|<00> <0061>`,
		`<01> <0062>`,
	)
	if n := strings.Count(file, "/Subtype /Type3"); n != 1 {
		t.Errorf("the file has %d Type 3 fonts, want 1", n)
	}
	if n := strings.Count(file, " d1"); n != 2 {
		t.Errorf("the file has %d shape glyphs, want 2", n)
	}
	// The shape glyph sets no color.
	re := regexp.MustCompile(`(?s)d1(.*?)endstream`)
	for _, m := range re.FindAllStringSubmatch(file, -1) {
		if regexp.MustCompile(` (rg|RG|g|G|k|K|sc|scn|SC|SCN|cs|CS)\n`).MatchString(m[1]) {
			t.Errorf("a shape glyph sets a color: %q", m[1])
		}
	}
	wantNoMatch(t, file, `/CIDFontType`)
}

func TestImageFontMany(t *testing.T) {
	f, err := NewImageFont(1)
	if err != nil {
		t.Fatal(err)
	}
	var glyphs []Glyph
	var text strings.Builder
	for i := range 300 {
		id, err := f.AddImageGlyph(ImageGlyph{Advance: 1, Mask: testMask(2, 2), Transform: path.IdentityTransform()})
		if err != nil {
			t.Fatal(err)
		}
		glyphs = append(glyphs, Glyph{GlyphID: id, TextRange: TextRange{Start: i, End: i + 1}, XAdvance: 1})
		text.WriteByte('a' + byte(i%26))
	}
	file := mustIvyFile(t, func(s *Surface) {
		s.DrawGlyphs(path.PointFromXY(0, 100), glyphs, f, text.String(), 0.5, false)
	})
	if n := strings.Count(file, "/Subtype /Type3"); n != 2 {
		t.Errorf("the file has %d Type 3 fonts, want 2", n)
	}
	wantMatch(t, file, `/Name /unknown`)
}

func TestImageFontPaints(t *testing.T) {
	f := testImageFont(t)
	stops := []Stop{
		{Offset: NormalizedF32Zero, Color: NewColorRGB(255, 0, 0), Opacity: NormalizedF32One},
		{Offset: NormalizedF32One, Color: NewColorRGB(0, 0, 255), Opacity: NormalizedF32One},
	}
	gradient := &Fill{Paint: LinearGradient{X2: 100, Transform: path.IdentityTransform(), Stops: stops}, Opacity: NormalizedF32One}

	// A gradient fills each mask through a soft mask, with no text, because
	// Poppler does not fill a shape glyph with a gradient.
	file := mustIvyFile(t, func(s *Surface) {
		s.SetFill(gradient)
		s.DrawGlyphs(path.PointFromXY(10, 50), imageFontRun(), f, "ab c", 20, false)
	})
	wantMatch(t, file, `/S /Luminosity`, `/ShadingType 2`)
	wantNoMatch(t, file, `/Type3`, `BT`)

	// A stroke without a fill fills the text with the paint of the stroke.
	file = mustIvyFile(t, func(s *Surface) {
		s.SetFill(nil)
		s.SetStroke(&Stroke{Paint: NewColorRGB(0, 255, 0), Width: 1, Opacity: NormalizedF32One})
		s.DrawGlyphs(path.PointFromXY(10, 50), imageFontRun(), f, "ab c", 20, false)
	})
	wantMatch(t, file, `0 1 0 rg`, `0 Tr`)
	wantNoMatch(t, file, `[12] Tr`)

	// Outlined glyphs draw the masks and the image on the page, with no
	// text.
	file = mustIvyFile(t, func(s *Surface) {
		s.SetFill(redFill(t, 1))
		s.DrawGlyphs(path.PointFromXY(10, 50), imageFontRun(), f, "ab c", 20, true)
	})
	wantMatch(t, file, `/ImageMask true`, `1 0 0 rg`)
	wantNoMatch(t, file, `/Type3`, `BT`)

	// DrawText cannot shape the text.
	_, err := ivyFile(t, func(s *Surface) {
		s.DrawText(path.PointFromXY(10, 50), f, 20, "ab", false, TextDirectionLeftToRight)
	})
	if !errors.Is(err, okage.ErrPDFInvalidFont) {
		t.Errorf("DrawText gives %v, want ErrPDFInvalidFont", err)
	}
}

func TestImageFontErrors(t *testing.T) {
	for _, upem := range []float32{0, -1, float32(math.NaN()), float32(math.Inf(1))} {
		if _, err := NewImageFont(upem); !errors.Is(err, okage.ErrPDFInvalidFont) {
			t.Errorf("NewImageFont(%v) gives %v, want ErrPDFInvalidFont", upem, err)
		}
	}
	f, err := NewImageFont(1000)
	if err != nil {
		t.Fatal(err)
	}
	src := image.NewGray(image.Rect(0, 0, 1, 1))
	img, err := ImageFromImage(src, false)
	if err != nil {
		t.Fatal(err)
	}
	gray := testMask(3, 3)
	gray.Pix[4] = 0x80
	id := path.IdentityTransform()
	nan := float32(math.NaN())
	cases := []struct {
		name string
		g    ImageGlyph
		want error
	}{
		{"neither", ImageGlyph{Transform: id}, okage.ErrPDFInvalidGlyph},
		{"both", ImageGlyph{Mask: testMask(1, 1), Image: img, Transform: id}, okage.ErrPDFInvalidGlyph},
		{"advance", ImageGlyph{Advance: nan, Mask: testMask(1, 1), Transform: id}, okage.ErrPDFInvalidGlyph},
		{"transform", ImageGlyph{Mask: testMask(1, 1), Transform: path.TransformFromScale(nan, 1)}, okage.ErrPDFInvalidGlyph},
		{"gray", ImageGlyph{Mask: gray, Transform: id}, okage.ErrPDFInvalidGlyph},
		{"large", ImageGlyph{Mask: &image.Alpha{Rect: image.Rect(0, 0, 1<<15, 1<<14), Stride: 1 << 15}, Transform: id}, okage.ErrPDFImage},
	}
	for _, c := range cases {
		if _, err := f.AddImageGlyph(c.g); !errors.Is(err, c.want) {
			t.Errorf("%s: AddImageGlyph gives %v, want %v", c.name, err, c.want)
		}
	}
	font, err := NewFont(brokenFont(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := font.AddImageGlyph(ImageGlyph{Mask: testMask(1, 1), Transform: id}); !errors.Is(err, okage.ErrPDFInvalidFont) {
		t.Errorf("AddImageGlyph on a font of NewFont gives %v, want ErrPDFInvalidFont", err)
	}
	for range maxImageGlyphs {
		if _, err := f.AddImageGlyph(ImageGlyph{Mask: &image.Alpha{}, Transform: id}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.AddImageGlyph(ImageGlyph{Mask: &image.Alpha{}, Transform: id}); !errors.Is(err, okage.ErrPDFInvalidGlyph) {
		t.Errorf("the glyph 65537 gives %v, want ErrPDFInvalidGlyph", err)
	}
}

func TestImageFontConcurrent(t *testing.T) {
	f, err := NewImageFont(10)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for range 50 {
				if _, err := f.AddImageGlyph(ImageGlyph{Advance: 1, Mask: testMask(2, 2), Transform: path.IdentityTransform()}); err != nil {
					t.Error(err)
				}
				_ = f.bbox()
			}
		})
	}
	wg.Wait()
	if n := f.numGlyphs(); n != 200 {
		t.Errorf("the font has %d glyphs, want 200", n)
	}
	// Two image fonts never share a font.
	g, err := NewImageFont(10)
	if err != nil {
		t.Fatal(err)
	}
	if f.hash == g.hash {
		t.Error("two image fonts have the same hash")
	}
}
