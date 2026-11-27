package pdf

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// The tests of this file cover the invisible text of D106.

// invisibleRun draws the text with the font as invisible text at 10, 50.
func invisibleRun(s *Surface, font *Font, text string) {
	glyphs := naiveShape(text, font, TextDirectionAuto)
	s.DrawInvisibleGlyphs(path.PointFromXY(10, 50), glyphs, font, text, 16)
}

// TestDrawInvisibleGlyphs checks that an invisible run writes its text in
// the text rendering mode 3, with no color and no graphics state, even with
// a fill and a stroke of the surface.
func TestDrawInvisibleGlyphs(t *testing.T) {
	font := testFont(t, "NotoSans-Regular.ttf")
	content, err := pageContent(t, func(s *Surface) {
		s.SetFill(redFill(t, 0.5))
		s.SetStroke(redStroke(t, 0.5, 2))
		invisibleRun(s, font, "hi there")
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(content, "\n")
	if n := countLines(lines, "3 Tr"); n != 1 {
		t.Errorf("expected one line 3 Tr, got %d:\n%s", n, content)
	}
	if !strings.Contains(content, "BT\n3 Tr\n/f0 16 Tf\n") || !strings.Contains(content, "TJ\nET\n") {
		t.Errorf("expected a text object in the mode 3:\n%s", content)
	}
	for _, op := range []string{" gs", " rg", " RG", " sc", " scn", " SC", " SCN", " w", " cs", " CS"} {
		for _, l := range lines {
			if strings.HasSuffix(l, op) {
				t.Errorf("expected no operator %q, got the line %q:\n%s", strings.TrimSpace(op), l, content)
			}
		}
	}
}

// TestDrawInvisibleGlyphsKeepsState checks that an invisible run changes no
// state of the runs after it: the content of a visible run after it is the
// content of the visible run alone.
func TestDrawInvisibleGlyphsKeepsState(t *testing.T) {
	font := testFont(t, "NotoSans-Regular.ttf")
	visible := func(s *Surface) {
		s.SetFill(redFill(t, 0.5))
		s.DrawText(path.PointFromXY(10, 100), font, 16, "hi there", false, TextDirectionAuto)
	}
	alone, err := pageContent(t, visible)
	if err != nil {
		t.Fatal(err)
	}
	both, err := pageContent(t, func(s *Surface) {
		invisibleRun(s, font, "hi there")
		visible(s)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(both, alone) {
		t.Errorf("expected the visible run to end the page unchanged:\n%s\nwant the end:\n%s", both, alone)
	}
	if !strings.Contains(alone, "0 Tr") || strings.Contains(alone, "3 Tr") {
		t.Errorf("expected the visible run in the mode 0:\n%s", alone)
	}
}

// TestDrawInvisibleGlyphsText checks that the file embeds the font and
// maps the glyphs of an invisible run to their text, and that a second
// text of a glyph takes a second CID, as in a visible run (D102).
func TestDrawInvisibleGlyphsText(t *testing.T) {
	font := testFont(t, "NotoSans-Regular.ttf")
	d := NewDocument(WithCompression(false), WithPretty(true))
	ps, _ := PageSettingsFromWH(200, 200)
	s := d.StartPage(ps).Surface()
	glyphs := []Glyph{
		{GlyphID: 36, TextRange: TextRange{0, 1}, XAdvance: 0.5},
		{GlyphID: 36, TextRange: TextRange{1, 2}, XAdvance: 0.5},
	}
	s.DrawInvisibleGlyphs(path.PointFromXY(0, 50), glyphs, font, "ab", 12)
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	for _, want := range []string{"/FontFile2", "/ToUnicode", "\n3 Tr\n", "<0001> <0061>", "/CIDToGIDMap"} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("expected %q in the file", want)
		}
	}
	if bytes.Contains(data, []byte("/ActualText")) {
		t.Error("expected no ActualText")
	}
}

// TestDrawInvisibleGlyphsType3 checks that a run of a font of Type 3, a
// font of color glyphs or an image font, stays in the mode 3, where a
// visible run of such a font is always filled.
func TestDrawInvisibleGlyphsType3(t *testing.T) {
	img, err := NewImageFont(10)
	if err != nil {
		t.Fatal(err)
	}
	id, err := img.AddImageGlyph(ImageGlyph{Advance: 9, Mask: testMask(7, 8), Transform: path.IdentityTransform()})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		font   *Font
		glyphs []Glyph
		text   string
	}{
		{"COLR", testFont(t, "NotoColorEmoji.COLR.subset.ttf"), nil, "😀"},
		{"image", img, []Glyph{{GlyphID: id, TextRange: TextRange{0, 1}, XAdvance: 0.5}}, "a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content, err := pageContent(t, func(s *Surface) {
				glyphs := tc.glyphs
				if glyphs == nil {
					glyphs = naiveShape(tc.text, tc.font, TextDirectionAuto)
				}
				s.DrawInvisibleGlyphs(path.PointFromXY(10, 50), glyphs, tc.font, tc.text, 16)
			})
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(content, "\n")
			if countLines(lines, "3 Tr") != 1 || countLines(lines, "0 Tr") != 0 {
				t.Errorf("expected the mode 3 only:\n%s", content)
			}
		})
	}
}

// TestDrawInvisibleGlyphsFallback checks that an invisible run of a font
// whose subset fails stays invisible text, and draws no path (D80).
func TestDrawInvisibleGlyphsFallback(t *testing.T) {
	font, err := NewFont(brokenFont(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDocument(WithCompression(false), WithPretty(true))
	ps, _ := PageSettingsFromWH(200, 200)
	invisibleRun(d.StartPage(ps).Surface(), font, "hi there")
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	text := buf.String()
	if strings.Contains(text, "/FontFile2") {
		t.Error("expected no font program")
	}
	if !strings.Contains(text, "\n3 Tr\n") {
		t.Error("expected the mode 3")
	}
	for _, op := range []string{"\nf\n", "\nS\n", "\nB\n"} {
		if strings.Contains(text, op) {
			t.Errorf("expected no path, got %q", strings.TrimSpace(op))
		}
	}
}

// TestDrawInvisibleGlyphsInvalid checks that a glyph outside its text gives
// the error of CheckGlyphs, as DrawGlyphs does.
func TestDrawInvisibleGlyphsInvalid(t *testing.T) {
	font := testFont(t, "NotoSans-Regular.ttf")
	content, err := pageContent(t, func(s *Surface) {
		glyphs := []Glyph{{GlyphID: 36, TextRange: TextRange{0, 5}, XAdvance: 0.5}}
		s.DrawInvisibleGlyphs(path.PointFromXY(10, 50), glyphs, font, "ab", 16)
	})
	if !errors.Is(err, okage.ErrPDFInvalidGlyph) {
		t.Errorf("expected %v, got %v", okage.ErrPDFInvalidGlyph, err)
	}
	if strings.Contains(content, "BT") {
		t.Errorf("expected no text object:\n%s", content)
	}
}
