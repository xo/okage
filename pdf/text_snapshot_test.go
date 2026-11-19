// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/okage/path"
)

// The tests of this file write the snapshots of krilla for text and fonts:
// the snapshot tests of tests/src/text.rs and tests/src/font.rs. Each font
// file is a stream that krilla compresses, so the tests compare the decoded
// streams with checkSnapshotDecoded. The subsets of the fonts and the tags
// of their names are then the bytes of krilla.

// testFont reads a font of testdata/fonts, the fonts of krilla
// assets/fonts.
func testFont(tb testing.TB, name string, opts ...FontOption) *Font {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "fonts", name))
	if err != nil {
		tb.Fatal(err)
	}
	f, err := NewFont(data, 0, opts...)
	if err != nil {
		tb.Fatal(err)
	}
	return f
}

// textSnapshotPage starts a document with settings_1 and a page of 200 by
// 200, as the macro snapshot of krilla does, and gives the surface of the
// page.
// Document.Finish finishes the page and its surface.
func textSnapshotPage(t *testing.T, opts ...Option) (*Document, *Surface) {
	t.Helper()
	d := NewDocument(append(settings1(), opts...)...)
	ps, _ := PageSettingsFromWH(200, 200)
	return d, d.StartPage(ps).Surface()
}

// TestTextDirectionAuto is the snapshot test text_direction_auto of krilla
// tests/src/text.rs.
func TestTextDirectionAuto(t *testing.T) {
	d, s := textSnapshotPage(t)
	s.DrawText(path.PointFromXY(0, 100), testFont(t, "NotoSansArabic-Regular.ttf"), 32, "مرحبا بالعالم", false, TextDirectionAuto)
	checkSnapshotDecoded(t, "text_direction_auto", d)
}

// simpleTextImpl is simple_text_impl of krilla tests/src/text.rs.
func simpleTextImpl(t *testing.T, name, font string) {
	t.Helper()
	d, s := textSnapshotPage(t)
	s.DrawText(path.PointFromXY(0, 50), testFont(t, font), 16, "A line of text.", false, TextDirectionAuto)
	checkSnapshotDecoded(t, name, d)
}

// TestTextSimpleCFF is the snapshot test text_simple_cff of krilla
// tests/src/text.rs.
func TestTextSimpleCFF(t *testing.T) {
	simpleTextImpl(t, "text_simple_cff", "LatinModernRoman-Regular.otf")
}

// TestTextSimpleTTF is the snapshot test text_simple_ttf of krilla
// tests/src/text.rs.
func TestTextSimpleTTF(t *testing.T) {
	simpleTextImpl(t, "text_simple_ttf", "NotoSans-Regular.ttf")
}

// TestTextComplex is the snapshot tests text_complex to text_complex_4 of
// krilla tests/src/text.rs.
func TestTextComplex(t *testing.T) {
	for _, c := range []struct {
		name string
		size float32
		text string
	}{
		{"text_complex", 16, "यह कुछ जटिल पाठ है."},
		{"text_complex_2", 16, "यु॒धा नर॑ ऋ॒ष्वा"},
		{"text_complex_3", 12, "आ रु॒क्मैरा यु॒धा नर॑ ऋ॒ष्वा ऋ॒ष्टीर॑सृक्षत ।"},
		{"text_complex_4", 10, "अन्वे॑नाँ॒ अह॑ वि॒द्युतो॑ म॒रुतो॒ जज्झ॑तीरव भनर॑र्त॒ त्मना॑ दि॒वः ॥"},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, s := textSnapshotPage(t)
			s.DrawText(path.PointFromXY(0, 50), testFont(t, "NotoSansDevanagari-Regular.ttf"), c.size, c.text, false, TextDirectionAuto)
			checkSnapshotDecoded(t, c.name, d)
		})
	}
}

// TestTextSmallCaps is the snapshot test text_small_caps of krilla
// tests/src/text.rs. It tests https://github.com/typst/typst/issues/5654.
func TestTextSmallCaps(t *testing.T) {
	glyphs := []Glyph{
		{GlyphID: 2464, TextRange: TextRange{0, 1}, XAdvance: 0.529},
		{GlyphID: 2464, TextRange: TextRange{1, 2}, XAdvance: 0.529},
	}
	d, s := textSnapshotPage(t)
	s.DrawGlyphs(path.PointFromXY(0, 50), glyphs, testFont(t, "LibertinusSerif-Regular.otf"), "Tt", 12, false)
	checkSnapshotDecoded(t, "text_small_caps", d)
}

// TestTextFill is the snapshot test text_fill of krilla tests/src/text.rs.
func TestTextFill(t *testing.T) {
	d, s := textSnapshotPage(t)
	s.DrawText(path.PointFromXY(0, 50), testFont(t, "NotoSans-Regular.ttf"), 16, "hi there", false, TextDirectionAuto)
	checkSnapshotDecoded(t, "text_fill", d)
}

// TestTextStroke is the snapshot test text_stroke of krilla
// tests/src/text.rs.
func TestTextStroke(t *testing.T) {
	d, s := textSnapshotPage(t)
	stroke := DefaultStroke()
	s.SetStroke(&stroke)
	s.DrawText(path.PointFromXY(0, 50), testFont(t, "NotoSans-Regular.ttf"), 16, "hi there", false, TextDirectionAuto)
	checkSnapshotDecoded(t, "text_stroke", d)
}

// TestFontWrongMetrics is the snapshot tests font_wrong_metrics to
// font_wrong_metrics_3 of krilla tests/src/font.rs. See
// https://github.com/typst/typst/issues/6185 and
// https://github.com/typst/typst/issues/6671.
func TestFontWrongMetrics(t *testing.T) {
	for _, c := range []struct {
		name string
		font string
		y    float32
		text string
	}{
		{"font_wrong_metrics", "NotoSerifSC_subset1.ttf", 25, "智"}, //nolint:gosmopolitan // the text of the test of krilla
		{"font_wrong_metrics_2", "DejaVuSansMono.ttf", 25, "H"},
		{"font_wrong_metrics_3", "NewCMMath-Regular.otf", 50, "a + b = c"},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, s := textSnapshotPage(t)
			s.DrawText(path.PointFromXY(0, c.y), testFont(t, c.font), 25, c.text, false, TextDirectionAuto)
			checkSnapshotDecoded(t, c.name, d)
		})
	}
}

// TestFontType3 is the snapshot tests font_type3_color_glyphs,
// font_type3_pdf_14 and font_type3_with_not_def of krilla
// tests/src/font.rs. krilla draws the SVG glyphs of Twitter Color Emoji with
// render_svg_glyph of krilla-svg, which pdfsvg ports in phase 17. Until
// then the tests skip, and the oracle of test/krilla compares the Type 3
// fonts of these scenes without the SVG glyphs.
func TestFontType3(t *testing.T) {
	for _, c := range []struct {
		name string
		opts []Option
		y    float32
		size float32
		text string
	}{
		{"font_type3_color_glyphs", nil, 25, 25, "😀😃"},
		{"font_type3_pdf_14", []Option{WithPDFVersion(PDF14)}, 25, 25, "😀"},
		{"font_type3_with_not_def", nil, 100, 32, "H😄"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Skip("the SVG glyphs need pdfsvg, which phase 17 ports")
			d, s := textSnapshotPage(t, c.opts...)
			s.DrawText(path.PointFromXY(0, c.y), testFont(t, "TwitterColorEmoji.subset.ttf"), c.size, c.text, false, TextDirectionAuto)
			checkSnapshotDecoded(t, c.name, d)
		})
	}
}

// TestType3MoreThan256Glyphs is the test type3_more_than_256_glyphs of
// krilla src/text/type3.rs.
func TestType3MoreThan256Glyphs(t *testing.T) {
	font := testFont(t, "NotoColorEmoji.COLR.subset.ttf")
	t3 := newType3FontMapper(font)

	for i := GlyphID(2); i < 258; i++ {
		t3.addGlyph(coloredGlyph{glyphID: i, contextColor: ColorRGBBlack()})
	}

	if len(t3.fonts) != 1 {
		t.Fatalf("got %d fonts, want 1", len(t3.fonts))
	}
	if got := t3.fonts[0].addGlyph(coloredGlyph{glyphID: 20, contextColor: ColorRGBBlack()}); got != 18 {
		t.Errorf("got the gid %d, want 18", got)
	}

	t3.addGlyph(coloredGlyph{glyphID: 512, contextColor: ColorRGBBlack()})
	if len(t3.fonts) != 2 {
		t.Fatalf("got %d fonts, want 2", len(t3.fonts))
	}
}
