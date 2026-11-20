package krilla

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
	"github.com/xo/okage/typesetting/font"
)

// The tests of this file compare the text of the package pdf with krilla,
// through the question "krilla" of the oracle (the gate of phase 16 of the
// PDF writer in docs/PLAN.md): the CID fonts with their subsets, the tags of
// their names, their widths and their ToUnicode maps, the ActualText spans,
// the Type 3 fonts of the COLR glyphs and of the bitmap glyphs of sbix and
// CBDT, the SVG glyphs that pdfsvg draws, and the shaping of draw_text. The
// objects of the two files must be the same after the reader decodes the
// streams, so the subsets of the fonts are the bytes of krilla.

// textSpec holds the fields of the calls draw_glyphs and draw_text of the
// oracle, in opSpec.
type textSpec struct {
	Start     *[2]float32 `json:"start"`
	Font      *int        `json:"font"`
	Size      float32     `json:"size"`
	Text      string      `json:"text"`
	Outlined  bool        `json:"outlined"`
	Glyphs    []glyphSpec `json:"glyphs"`
	Direction *string     `json:"direction"`
}

// glyphSpec is a glyph of draw_glyphs, as the oracle reads it.
type glyphSpec struct {
	ID       uint32  `json:"id"`
	Range    [2]int  `json:"range"`
	XAdvance float32 `json:"x_advance"`
	XOffset  float32 `json:"x_offset"`
	YOffset  float32 `json:"y_offset"`
	YAdvance float32 `json:"y_advance"`
}

// sceneResources is Resources of the oracle: the fonts and the images of a
// scene. goResources reads the fonts as answer_scene of the oracle reads
// them, and goImageScene makes the images.
type sceneResources struct {
	fonts  []*pdf.Font
	images []*pdf.Image
}

// goResources reads the fonts of a scene with pdf.NewFont, as the oracle
// reads them with Font::new_variable.
func goResources(s scene) (*sceneResources, error) {
	res := &sceneResources{}
	for _, f := range s.Fonts {
		data, err := os.ReadFile(f.File)
		if err != nil {
			return nil, err
		}
		var coords []pdf.VariationCoord
		for _, c := range f.Coords {
			tag, ok := pdf.TagFromString(c.Tag)
			if !ok {
				return nil, fmt.Errorf("the tag %q is not valid", c.Tag)
			}
			coords = append(coords, pdf.VariationCoord{Tag: tag, Value: c.Value})
		}
		font, err := pdf.NewFont(data, int(f.Index), pdf.WithVariations(coords...))
		if err != nil {
			return nil, err
		}
		res.fonts = append(res.fonts, font)
	}
	return res, nil
}

// goDrawText makes the calls draw_glyphs and draw_text of the oracle.
func goDrawText(s *pdf.Surface, o opSpec, res *sceneResources) error {
	if o.Start == nil || o.Font == nil {
		return fmt.Errorf("%s needs a start and a font", o.Op)
	}
	if res == nil || *o.Font < 0 || *o.Font >= len(res.fonts) {
		return fmt.Errorf("the font %d does not exist", *o.Font)
	}
	font := res.fonts[*o.Font]
	start := path.PointFromXY(o.Start[0], o.Start[1])
	if o.Op == "draw_text" {
		dirs := map[string]pdf.TextDirection{
			"auto": pdf.TextDirectionAuto, "ltr": pdf.TextDirectionLeftToRight,
			"rtl": pdf.TextDirectionRightToLeft, "ttb": pdf.TextDirectionTopToBottom,
			"btt": pdf.TextDirectionBottomToTop,
		}
		dir := pdf.TextDirectionAuto
		if o.Direction != nil {
			d, ok := dirs[*o.Direction]
			if !ok {
				return fmt.Errorf("the direction %q is not known", *o.Direction)
			}
			dir = d
		}
		s.DrawText(start, font, o.Size, o.Text, o.Outlined, dir)
		return nil
	}
	glyphs := make([]pdf.Glyph, len(o.Glyphs))
	for i, g := range o.Glyphs {
		glyphs[i] = pdf.Glyph{
			GlyphID:   pdf.GlyphID(g.ID),
			TextRange: pdf.TextRange{Start: g.Range[0], End: g.Range[1]},
			XAdvance:  g.XAdvance,
			XOffset:   g.XOffset,
			YOffset:   g.YOffset,
			YAdvance:  g.YAdvance,
		}
	}
	s.DrawGlyphs(start, glyphs, font, o.Text, o.Size, o.Outlined)
	return nil
}

// textFont is a font of testdata/fonts of pdf, which holds copies of the
// fonts of krilla assets/fonts.
func textFont(t *testing.T, name string, coords ...coord) fontSpec {
	t.Helper()
	return fontSpec{File: abs(t, "pdf", "testdata", "fonts", name), Coords: coords}
}

// krillaFont is a font of the checkout of krilla.
func krillaFont(t *testing.T, name string, coords ...coord) fontSpec {
	t.Helper()
	return fontSpec{File: abs(t, "upstream", "krilla", "assets", "fonts", name), Coords: coords}
}

// svgFont is a font of the tests of svg.
func svgFont(t *testing.T, name string, coords ...coord) fontSpec {
	t.Helper()
	return fontSpec{File: abs(t, "svg", "testdata", "fonts", name), Coords: coords}
}

// textSettings are the settings of the snapshots of krilla without the SVG
// glyphs. A scene with the SVG glyphs sets svg_glyphs in extra.
func textSettings(extra map[string]any) map[string]any {
	s := map[string]any{"xmp_metadata": false, "pretty": true, "ascii_compatible": true, "compress_content_streams": false, "svg_glyphs": false}
	maps.Copy(s, extra)
	return s
}

// withDirection gives a call draw_text a direction.
func withDirection(o op, dir string) op {
	o["direction"] = dir
	return o
}

// outlined draws the text of a call as paths.
func outlined(o op) op {
	o["outlined"] = true
	return o
}

// placedGlyph is a glyph of draw_glyphs with all its offsets and advances.
func placedGlyph(id uint32, start, end int, xAdvance, xOffset, yOffset, yAdvance float32) map[string]any {
	return map[string]any{"id": id, "range": []int{start, end}, "x_advance": xAdvance,
		"x_offset": xOffset, "y_offset": yOffset, "y_advance": yAdvance}
}

// textCases returns the scenes of the text.
func textCases(t *testing.T) []documentCase {
	t.Helper()
	var cases []documentCase
	add := func(name string, settings map[string]any, fonts []fontSpec, ops ...op) {
		cases = append(cases, documentCase{name, scene{Settings: textSettings(settings), Fonts: fonts, Pages: []pageSpec{{Width: 200, Height: 200, Ops: ops}}}})
	}
	one := func(f fontSpec) []fontSpec { return []fontSpec{f} }

	// The snapshots of tests/src/text.rs and tests/src/font.rs.
	add("text_direction_auto", nil, one(textFont(t, "NotoSansArabic-Regular.ttf")), drawText(0, 100, 0, 32, "مرحبا بالعالم"))
	add("text_simple_cff", nil, one(textFont(t, "LatinModernRoman-Regular.otf")), drawText(0, 50, 0, 16, "A line of text."))
	add("text_simple_ttf", nil, one(textFont(t, "NotoSans-Regular.ttf")), drawText(0, 50, 0, 16, "A line of text."))
	devanagari := one(textFont(t, "NotoSansDevanagari-Regular.ttf"))
	add("text_complex", nil, devanagari, drawText(0, 50, 0, 16, "यह कुछ जटिल पाठ है."))
	add("text_complex_2", nil, devanagari, drawText(0, 50, 0, 16, "यु॒धा नर॑ ऋ॒ष्वा"))
	add("text_complex_3", nil, devanagari, drawText(0, 50, 0, 12, "आ रु॒क्मैरा यु॒धा नर॑ ऋ॒ष्वा ऋ॒ष्टीर॑सृक्षत ।"))
	add("text_complex_4", nil, devanagari, drawText(0, 50, 0, 10, "अन्वे॑नाँ॒ अह॑ वि॒द्युतो॑ म॒रुतो॒ जज्झ॑तीरव भनर॑र्त॒ त्मना॑ दि॒वः ॥"))
	add("text_small_caps", nil, one(textFont(t, "LibertinusSerif-Regular.otf")),
		drawGlyphs(0, 50, 0, 12, "Tt", glyph(2464, 0, 1, 0.529), glyph(2464, 1, 2, 0.529)))
	noto := one(textFont(t, "NotoSans-Regular.ttf"))
	add("text_fill", nil, noto, drawText(0, 50, 0, 16, "hi there"))
	add("text_stroke", nil, noto, setStroke(map[string]any{}), drawText(0, 50, 0, 16, "hi there"))
	add("font_wrong_metrics", nil, one(textFont(t, "NotoSerifSC_subset1.ttf")), drawText(0, 25, 0, 25, "智")) //nolint:gosmopolitan // the text of the test of krilla
	add("font_wrong_metrics_2", nil, one(textFont(t, "DejaVuSansMono.ttf")), drawText(0, 25, 0, 25, "H"))
	add("font_wrong_metrics_3", nil, one(textFont(t, "NewCMMath-Regular.otf")), drawText(0, 50, 0, 25, "a + b = c"))

	// The snapshots of the Type 3 fonts, without the SVG glyphs: the Type 3
	// font draws the outline of each glyph of Twitter Color Emoji.
	twitter := one(textFont(t, "TwitterColorEmoji.subset.ttf"))
	add("font_type3_color_glyphs", nil, twitter, drawText(0, 25, 0, 25, "😀😃"))
	add("font_type3_pdf_14", map[string]any{"version": "1.4"}, twitter, drawText(0, 25, 0, 25, "😀"))
	add("font_type3_with_not_def", nil, twitter, drawText(0, 100, 0, 32, "H😄"))

	// The same snapshots with the SVG glyphs, which krilla-svg and pdfsvg
	// draw, as the tests of krilla draw them.
	svgGlyphs := map[string]any{"svg_glyphs": true}
	add("font_type3_color_glyphs with SVG glyphs", svgGlyphs, twitter, drawText(0, 25, 0, 25, "😀😃"))
	add("font_type3_pdf_14 with SVG glyphs", map[string]any{"svg_glyphs": true, "version": "1.4"}, twitter, drawText(0, 25, 0, 25, "😀"))
	add("font_type3_with_not_def with SVG glyphs", svgGlyphs, twitter, drawText(0, 100, 0, 32, "H😄"))

	// The directions of tests/src/text.rs.
	cjk := one(krillaFont(t, "NotoSansCJKsc-Regular.otf"))
	add("text_direction_ltr", nil, one(textFont(t, "NotoSans-Regular.ttf")), withDirection(drawText(0, 20, 0, 16, "Hello world"), "ltr"))
	add("text_direction_rtl", nil, one(textFont(t, "NotoSansArabic-Regular.ttf")), withDirection(drawText(0, 20, 0, 16, "مرحبا بالعالم"), "rtl"))
	add("text_direction_ttb", nil, cjk, withDirection(drawText(10, 0, 0, 16, "你好这是一段则是文字"), "ttb"))   //nolint:gosmopolitan // the text of the test of krilla
	add("text_direction_btt", nil, cjk, withDirection(drawText(10, 200, 0, 16, "你好这是一段则是文字"), "btt")) //nolint:gosmopolitan // the text of the test of krilla
	add("text_two_fonts", nil, []fontSpec{textFont(t, "NotoSans-Regular.ttf"), krillaFont(t, "NotoSansCJKsc-Regular.otf")},
		drawText(0, 50, 0, 16, "hi there"), drawText(0, 20, 1, 16, "你好")) //nolint:gosmopolitan // the text of the test of krilla

	// The fills and the strokes of text_with_fill_impl and
	// text_with_stroke_impl, as glyphs and as outlines, with the COLR
	// glyphs of Noto Color Emoji.
	stops := []map[string]any{stop(0.1, rgb(255, 0, 0), 1), stop(0.3, rgb(0, 255, 0), 1), stop(0.6, rgb(0, 0, 255), 1), stop(0.9, rgb(0, 0, 0), 0.5)}
	grad := func(spread string) map[string]any {
		return gradient("linear", map[string]any{"x1": 50, "y1": 0, "x2": 150, "y2": 0}, spread, nil, stops)
	}
	fillFonts := []fontSpec{textFont(t, "NotoSans-Regular.ttf"), textFont(t, "NotoColorEmoji.COLR.subset.ttf")}
	for _, out := range []bool{false, true} {
		draw := func(o op) op {
			if out {
				return outlined(o)
			}
			return o
		}
		add(fmt.Sprintf("text with fill outlined %v", out), nil, fillFonts,
			setFill(rgb(255, 0, 0), 0.5, "nonzero"), draw(drawText(2, 19, 0, 20, "red outlined text")),
			setFill(rgb(0, 0, 255), 0.8, "nonzero"), draw(drawText(2, 39, 0, 20, "blue outlined text")),
			setFill(grad("pad"), 1, "nonzero"), draw(drawText(2, 59, 0, 20, "gradient text")),
			setFill(rgb(0, 0, 255), 0.8, "nonzero"), draw(drawText(2, 79, 1, 20, "😄😁😆")),
			setFill(grad("reflect"), 1, "nonzero"), draw(drawText(2, 99, 0, 20, "longer gradient text with repeat")),
		)
		add(fmt.Sprintf("text with stroke outlined %v", out), nil, fillFonts,
			noFill(), setStroke(map[string]any{"paint": rgb(255, 0, 0), "opacity": 0.5}), draw(drawText(2, 29, 0, 30, "red outlined text")),
			setStroke(map[string]any{"paint": rgb(0, 0, 255), "opacity": 0.8}), draw(drawText(2, 59, 0, 30, "blue outlined text")),
			setStroke(map[string]any{"paint": grad("pad")}), draw(drawText(2, 89, 0, 30, "gradient text")),
			setFill(rgb(0, 255, 0), 1, "nonzero"), setStroke(map[string]any{"paint": rgb(0, 0, 0), "width": 0.5}),
			draw(drawText(2, 119, 1, 30, "😄😁😆")),
		)
	}

	// The COLR glyphs of Noto Color Emoji, in a Type 3 font of more than 256
	// glyphs, inside a group whose bbox matters.
	emojis, err := os.ReadFile(abs(t, "upstream", "krilla", "assets", "emojis.txt"))
	if err != nil {
		t.Skipf("the checkout of krilla is missing: %v", err)
	}
	add("colr emojis", nil, one(textFont(t, "NotoColorEmoji.COLR.subset.ttf")),
		pushIsolated(), drawText(0, 30, 0, 16, strings.TrimSpace(string(emojis))), pop())
	colr := []fontSpec{krillaFont(t, "colr_test_glyphs.ttf"), krillaFont(t, "COLR_extra.ttf")}
	var colrGlyphs []map[string]any
	for id := uint32(1); id < 220; id++ {
		colrGlyphs = append(colrGlyphs, glyph(id, 0, 0, 1))
	}
	add("colr test glyphs", nil, colr,
		setFill(rgb(0, 128, 0), 1, "nonzero"), drawGlyphs(0, 30, 0, 8, "", colrGlyphs...),
		drawGlyphs(0, 80, 1, 30, "", glyph(1, 0, 0, 1), glyph(2, 0, 0, 1), glyph(3, 0, 0, 1), glyph(4, 0, 0, 1)))

	// Glyphs with offsets, y advances, the same glyph for two texts, two
	// glyphs of one cluster, the glyph 0 and reversed clusters.
	add("glyph offsets", nil, noto,
		drawGlyphs(10, 50, 0, 20, "abc",
			placedGlyph(68, 0, 1, 0.6, 0.1, 0, 0), placedGlyph(69, 1, 2, 0.6, 0, 0.2, 0),
			placedGlyph(70, 2, 3, 0.5, 0, 0, 0.3)),
		drawGlyphs(10, 100, 0, 20, "aéb",
			glyph(68, 0, 1, 0.6), glyph(68, 1, 3, 0.6), glyph(69, 3, 4, 0.6), glyph(0, 3, 4, 0.5)),
		drawGlyphs(10, 150, 0, 20, "ab c",
			glyph(70, 3, 4, 0.5), glyph(3, 2, 3, 0.3), glyph(69, 1, 2, 0.6), glyph(68, 0, 1, 0.6)))
	add("variable font cff2", nil, []fontSpec{
		krillaFont(t, "Cantarell-VF.otf", coord{"wght", 400}),
		krillaFont(t, "Cantarell-VF.otf", coord{"wght", 100}),
		krillaFont(t, "Cantarell-VF.otf", coord{"wght", 900}),
	},
		drawText(4, 17, 0, 16, "I love variable fonts!"), drawText(4, 37, 1, 16, "I love variable fonts!"),
		drawText(4, 57, 2, 16, "I love variable fonts!"))
	add("zalgo", nil, noto, drawText(2, 85, 0, 32, "z͈̤̭͖̉͑́a̳ͫ́̇͑̽͒ͯlͨ͗̍̀̍̔̀ģ͔̫̫̄o̗̠͔̦͆̏̓͢"),
		outlined(drawText(2, 185, 0, 32, "z͈̤̭͖̉͑́a̳ͫ́̇͑̽͒ͯlͨ͗̍̀̍̔̀ģ͔̫̫̄o̗̠͔̦͆̏̓͢")))
	add("text in a pattern and a mask", nil, noto,
		setFill(pattern(20, 20, nil, drawText(0, 15, 0, 12, "ab")), 1, "nonzero"),
		drawText(0, 50, 0, 30, "patterned"),
		pushMask("luminosity", setFill(rgb(255, 255, 255), 1, "nonzero"), drawText(0, 100, 0, 40, "mask")),
		drawPath(rect(0, 60, 200, 60)), pop(),
		pushOpacity(0.5), drawText(0, 150, 0, 20, "half"), pop())

	// The bitmap glyphs of the tables sbix and CBDT, which the Type 3 fonts
	// draw as images: font_sbix_grid_glyf, font_sbix_grid_cff and
	// font_noto_color_emoji_cbdt of tests/src/font.rs.
	for _, c := range [][2]string{{"font_sbix_grid_glyf", "sbix.ttf"}, {"font_sbix_grid_cff", "sbix.otf"}} {
		cases = append(cases, documentCase{c[0], scene{Settings: textSettings(nil), Fonts: one(textFont(t, c[1])),
			Pages: []pageSpec{{Width: 288, Height: 240, Ops: sbixGrid()}}}})
	}
	cases = append(cases, allGlyphsCase(t, "font_noto_color_emoji_cbdt", textFont(t, "NotoColorEmoji.CBDT.subset.ttf"), string(emojis)))
	return cases
}

// sbixGrid makes the calls of draw_sbix_grid of krilla tests/src/font.rs:
// a grid, and the glyphs 2 to 29 of the font in its cells.
func sbixGrid() []op {
	var grid []seg
	for column := range 7 {
		grid = append(grid, rect(min(float64(column)*48, 287), 0, 1, 240)...)
	}
	for row := range 6 {
		grid = append(grid, rect(0, min(float64(row)*48, 239), 288, 1)...)
	}
	ops := []op{setFill(rgb(219, 226, 237), 1, "nonzero"), drawPath(grid), setFill(rgb(0, 0, 0), 1, "nonzero")}
	for i := range 28 {
		x, y := float32(i%6)*48-34, float32(i/6)*48+106
		ops = append(ops, drawGlyphs(x, y, 0, 64, "", glyph(uint32(i+2), 0, 0, 0)))
	}
	return ops
}

// allGlyphsCase makes the scene of all_glyphs_to_pdf of krilla
// tests/src/main.rs, without the cycle of colors: the glyph of each
// character of text that the cmap of the font has, in cells of 40 points
// on a page 400 points wide.
func allGlyphsCase(t *testing.T, name string, f fontSpec, text string) documentCase {
	t.Helper()
	data, err := os.ReadFile(f.File)
	if err != nil {
		t.Fatal(err)
	}
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	const width, size = 400, 40
	var ops []op
	n := 0
	for _, c := range text {
		gid, ok := face.NominalGlyph(c)
		if !ok {
			continue
		}
		col, row := n%(width/size), n/(width/size)
		ch := string(c)
		ops = append(ops, pushTransform(1, 0, 0, 1, float32(col*size), float32((row+1)*size)),
			setFill(rgb(0, 0, 0), 1, "nonzero"),
			drawGlyphs(0, 0, 0, size, ch, glyph(uint32(gid), 0, len(ch), 0)), pop())
		n++
	}
	height := float32((n + width/size - 1) / (width / size) * size)
	return documentCase{name, scene{Settings: textSettings(nil), Fonts: []fontSpec{f},
		Pages: []pageSpec{{Width: width, Height: height, Ops: ops}}}}
}

// subsetterFont is a font of the tests of internal/subsetter.
func subsetterFont(t *testing.T, name string, coords ...coord) fontSpec {
	t.Helper()
	return fontSpec{File: abs(t, "internal", "subsetter", "testdata", name), Coords: coords}
}

// TestTextGoVariable compares the instances of a variable font with
// krilla, as the test text_variable_font of krilla does with
// NotoSans_variable.ttf, and with RobotoFlex of the tests of svg, which has
// more axes. NotoSans-Regular_var.ttf of the tests of internal/subsetter is
// the same file as NotoSans_variable.ttf. typesetting computes the deltas
// of the variations as skrifa does, so each object must be the same, with
// the font files, the widths, the bounding boxes and the tags of the names
// (D84). The coordinates are the ends and the default of each axis.
func TestTextGoVariable(t *testing.T) {
	text := "I love variable fonts!"
	for _, c := range []struct {
		name string
		load func(*testing.T, string, ...coord) fontSpec
		font string
		axes [][]coord
	}{
		{"NotoSans", subsetterFont, "NotoSans-Regular_var.ttf", [][]coord{
			{{"wght", 400}}, {{"wght", 100}}, {{"wght", 900}}, {{"wght", 900}, {"wdth", 62.5}},
		}},
		{"RobotoFlex", svgFont, "RobotoFlex.subset.ttf", [][]coord{
			{{"wght", 400}}, {{"wght", 100}}, {{"wght", 1000}}, {{"wght", 1000}, {"wdth", 151}},
			{{"wdth", 25}}, {{"opsz", 144}, {"wght", 100}},
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			var fonts []fontSpec
			var ops []op
			for i, coords := range c.axes {
				fonts = append(fonts, c.load(t, c.font, coords...))
				ops = append(ops, drawText(4, float32(17+20*i), i, 16, text))
			}
			sc := scene{Settings: textSettings(nil), Fonts: fonts, Pages: []pageSpec{{Width: 200, Height: 200, Ops: ops}}}
			a := ask(t, sceneQuestion{Krilla: sc})[0]
			if a.Err != "" {
				t.Fatalf("the oracle gives an error: %s", a.Err)
			}
			got, err := goScene(sc)
			if err != nil {
				t.Fatal(err)
			}
			gotFile, err := readPDF(got)
			if err != nil {
				t.Fatal(err)
			}
			wantFile, err := readPDF(a.Data)
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range comparePDF(gotFile, wantFile) {
				t.Error(d)
			}
		})
	}
}

// TestTextGo compares the files of pdf with the files of krilla for the
// scenes of textCases.
func TestTextGo(t *testing.T) {
	cases := textCases(t)
	questions := make([]any, len(cases))
	for i, c := range cases {
		questions[i] = sceneQuestion{Krilla: c.scene}
	}
	answers := ask(t, questions...)
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			compareScene(t, c, answers[i])
		})
	}
}

// randomGlyphRun makes a random text and the glyphs of a random run over it,
// with clusters of one or more characters and glyphs that share a
// cluster.
func randomGlyphRun(r *rand.Rand) (string, []map[string]any) {
	const chars = "abcdefgh ijk.,-é"
	var b strings.Builder
	for range 1 + r.IntN(12) {
		c := []rune(chars)[r.IntN(utf8.RuneCountInString(chars))]
		b.WriteRune(c)
	}
	text := b.String()
	var bounds []int
	for i := range text {
		bounds = append(bounds, i)
	}
	bounds = append(bounds, len(text))
	var glyphs []map[string]any
	for i := 0; i+1 < len(bounds); {
		n := 1 + r.IntN(2)
		end := min(i+n, len(bounds)-1)
		for range 1 + r.IntN(2) {
			id := uint32(r.IntN(90))
			glyphs = append(glyphs, placedGlyph(id, bounds[i], bounds[end],
				float32(r.IntN(8))/10, float32(r.IntN(3)-1)/10, float32(r.IntN(3)-1)/10, float32(r.IntN(2))/10))
		}
		i = end
	}
	if r.IntN(4) == 0 {
		for i, j := 0, len(glyphs)-1; i < j; i, j = i+1, j-1 {
			glyphs[i], glyphs[j] = glyphs[j], glyphs[i]
		}
	}
	return text, glyphs
}

// TestTextGoRandom compares the files of pdf with the files of krilla for
// random runs of glyphs, with fills, strokes and groups.
func TestTextGoRandom(t *testing.T) {
	n := 200
	if testing.Short() {
		n = 20
	}
	cases := textRandomCases(t, n)
	questions := make([]any, len(cases))
	for i, c := range cases {
		questions[i] = sceneQuestion{Krilla: c.scene}
	}
	answers := ask(t, questions...)
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			compareScene(t, c, answers[i])
		})
	}
}

// textRandomCases makes n random scenes of runs of glyphs.
func textRandomCases(t *testing.T, n int) []documentCase {
	t.Helper()
	r := rand.New(rand.NewPCG(16, 3))
	fonts := []fontSpec{
		textFont(t, "NotoSans-Regular.ttf"), textFont(t, "LatinModernRoman-Regular.otf"),
		textFont(t, "NotoColorEmoji.COLR.subset.ttf"), textFont(t, "TwitterColorEmoji.subset.ttf"),
	}
	paints := []map[string]any{rgb(255, 0, 0), luma(100), cmyk(0, 100, 0, 0),
		gradient("linear", map[string]any{"x1": 0, "y1": 0, "x2": 200, "y2": 0}, "pad", nil,
			[]map[string]any{stop(0, rgb(255, 0, 0), 1), stop(1, rgb(0, 0, 255), 0.5)})}
	cases := make([]documentCase, n)
	for i := range cases {
		var ops []op
		for range 1 + r.IntN(5) {
			switch r.IntN(4) {
			case 0:
				ops = append(ops, setFill(paints[r.IntN(len(paints))], float32(5+r.IntN(6))/10, "nonzero"))
			case 1:
				ops = append(ops, setStroke(map[string]any{"paint": paints[r.IntN(len(paints))], "width": float32(r.IntN(3))}))
			case 2:
				ops = append(ops, noStroke())
			}
			text, glyphs := randomGlyphRun(r)
			o := drawGlyphs(float32(r.IntN(100)), float32(20+r.IntN(160)), r.IntN(len(fonts)), float32(8+r.IntN(30)), text, glyphs...)
			if r.IntN(5) == 0 {
				o = outlined(o)
			}
			if r.IntN(4) == 0 {
				ops = append(ops, pushIsolated(), o, pop())
			} else {
				ops = append(ops, o)
			}
		}
		settings := map[string]any{"compress_content_streams": r.IntN(3) == 0}
		cases[i] = documentCase{fmt.Sprintf("random %d", i), scene{Settings: textSettings(settings), Fonts: fonts, Pages: []pageSpec{{Width: 200, Height: 200, Ops: ops}}}}
	}
	return cases
}

// TestTextGoErrors checks the errors of pdf for the glyphs where krilla
// panics.
func TestTextGoErrors(t *testing.T) {
	s := scene{Settings: textSettings(nil), Fonts: []fontSpec{textFont(t, "NotoSans-Regular.ttf")}}
	for _, o := range []op{
		drawGlyphs(0, 20, 0, 12, "ab", glyph(68, 0, 3, 0.5)),
		drawGlyphs(0, 20, 0, 12, "é", glyph(68, 1, 2, 0.5)),
		drawGlyphs(0, 20, 0, 12, "a", glyph(70000, 0, 1, 0.5)),
	} {
		s.Pages = []pageSpec{{Width: 10, Height: 10, Ops: []op{o}}}
		if a := ask(t, sceneQuestion{Krilla: s})[0]; a.Err == "" {
			t.Errorf("the oracle gives no error for %v", o)
		}
		if _, err := goScene(s); err == nil || !errors.Is(err, okage.ErrPDFInvalidGlyph) {
			t.Errorf("pdf gives the error %v for %v", err, o)
		}
	}
}
