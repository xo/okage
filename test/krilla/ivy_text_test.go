package krilla

import (
	"bytes"
	"image"
	"os"
	"os/exec"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
	"github.com/xo/okage/typesetting/font"
)

// nominalGlyphs gives the glyph of each character of text from the cmap of
// the font, with its advance, one cluster for each character.
func nominalGlyphs(t *testing.T, data []byte, text string) []pdf.Glyph {
	t.Helper()
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var glyphs []pdf.Glyph
	for i, c := range text {
		gid, ok := face.NominalGlyph(c)
		if !ok {
			t.Fatalf("the font has no glyph for %q", c)
		}
		glyphs = append(glyphs, pdf.Glyph{
			GlyphID:   pdf.GlyphID(gid),
			TextRange: pdf.TextRange{Start: i, End: i + utf8.RuneLen(c)},
			XAdvance:  face.HorizontalAdvance(gid) / float32(face.Upem()),
		})
	}
	return glyphs
}

// inkPixels counts the pixels of an image that are not white.
func inkPixels(img *image.NRGBA) int {
	n := 0
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if img.Pix[i] < 250 || img.Pix[i+1] < 250 || img.Pix[i+2] < 250 {
			n++
		}
	}
	return n
}

// mupdfText extracts the text of a PDF file with MuPDF `mutool draw`.
func mupdfText(t *testing.T, file string) string {
	t.Helper()
	mutool := needProgram(t, "mutool")
	b, err := exec.CommandContext(t.Context(), mutool, "draw", "-q", "-F", "txt", file).Output()
	if err != nil {
		t.Fatalf("mutool draw %s: %v", file, err)
	}
	return string(b)
}

// TestInvisibleText draws the text of a TrueType font, of a CFF font and
// of a font of color glyphs with Surface.DrawInvisibleGlyphs (D106). MuPDF
// and Poppler paint no pixel of the page, and pdftotext and MuPDF extract
// the text.
func TestInvisibleText(t *testing.T) {
	d := pdf.NewDocument()
	ps, _ := pdf.PageSettingsFromWH(300, 200)
	s := d.StartPage(ps).Surface()
	for i, c := range []struct {
		font string
		text string
	}{
		{"NotoSans-Regular.ttf", "hidden words"},
		{"LatinModernRoman-Regular.otf", "more words"},
		{"NotoColorEmoji.COLR.subset.ttf", "😀"},
	} {
		data, err := os.ReadFile(abs(t, "pdf", "testdata", "fonts", c.font))
		if err != nil {
			t.Fatal(err)
		}
		font, err := pdf.NewFont(data, 0)
		if err != nil {
			t.Fatal(err)
		}
		glyphs := nominalGlyphs(t, data, c.text)
		s.DrawInvisibleGlyphs(path.PointFromXY(10, float32(50+50*i)), glyphs, font, c.text, 24)
	}
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	// checkFile fails on a blank page, so the test reads the file and runs
	// qpdf itself.
	f, err := readPDF(buf.Bytes())
	if err != nil {
		t.Fatalf("reading the PDF file: %v", err)
	}
	for _, o := range f.Objects {
		if o.DecodeErr != nil {
			t.Errorf("object %d: %v", o.Ref.Num, o.DecodeErr)
		}
	}
	file := writeTemp(t, "invisible.pdf", buf.Bytes())
	qpdfCheck(t, file)

	for _, r := range []struct {
		name   string
		render func(*testing.T, string, int, float64) *image.NRGBA
	}{
		{"mutool", renderMuPDF},
		{"pdftoppm", renderPoppler},
	} {
		if n := inkPixels(r.render(t, file, 1, 72)); n != 0 {
			t.Errorf("%s paints %d pixels, want none", r.name, n)
		}
	}
	want := "hidden words more words 😀"
	if got := strings.Join(strings.Fields(pdfText(t, file)), " "); got != want {
		t.Errorf("pdftotext extracts %q, want %q", got, want)
	}
	if got := strings.Join(strings.Fields(mupdfText(t, file)), " "); got != want {
		t.Errorf("mutool extracts %q, want %q", got, want)
	}
}
