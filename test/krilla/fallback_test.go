package krilla

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
)

// The test of this file covers the fallback of D80: when the subset of a
// font fails, pdf draws its glyphs as paths and keeps its text as invisible
// text, where krilla returns an error.

// brokenFont writes NotoSans-Regular.ttf of pdf/testdata/fonts with a table
// post that subsetter refuses into the temporary folder of the test, and
// returns its path. The table has the version 2 and no glyph index, so
// subsetter finds no name for a glyph and returns Error::MalformedFont. The
// outlines of the font do not change.
func brokenFont(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(abs(t, "pdf", "testdata", "fonts", "NotoSans-Regular.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	f, ok := openFont(data, 0)
	if !ok {
		t.Fatal("NotoSans-Regular.ttf does not open")
	}
	post := f.table("post")
	if len(post) < 34 || binary.BigEndian.Uint32(post) != 0x00020000 {
		t.Fatal("NotoSans-Regular.ttf has no table post of the version 2")
	}
	binary.BigEndian.PutUint16(post[32:], 0)
	p := filepath.Join(t.TempDir(), "NotoSans-Broken.ttf")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// fallbackScene draws "hi there" with the font file: filled, stroked, and
// filled in an isolated group.
func fallbackScene(file string) scene {
	return scene{
		Fonts: []fontSpec{{File: file}},
		Pages: []pageSpec{{Width: 200, Height: 200, Ops: []op{
			drawText(10, 50, 0, 16, "hi there"),
			setStroke(map[string]any{}),
			drawText(10, 100, 0, 16, "hi there"),
			{"op": "set_stroke"},
			pushIsolated(),
			drawText(10, 150, 0, 16, "hi there"),
			pop(),
		}}},
	}
}

// pdfText extracts the text of a PDF file with Poppler `pdftotext`.
func pdfText(t *testing.T, file string) string {
	t.Helper()
	pdftotext := needProgram(t, "pdftotext")
	b, err := exec.CommandContext(t.Context(), pdftotext, "-q", file, "-").Output()
	if err != nil {
		t.Fatalf("pdftotext %s: %v", file, err)
	}
	return string(b)
}

// TestSubsetFallback draws text with a font whose subset fails. krilla
// returns an error. pdf returns the same kind of error with the fallback
// off, and with the fallback on, it writes a file that the reader reads,
// that MuPDF and Poppler render as the file of krilla with the font that
// does not fail, and whose text pdftotext extracts.
func TestSubsetFallback(t *testing.T) {
	broken := brokenFont(t)
	good := abs(t, "pdf", "testdata", "fonts", "NotoSans-Regular.ttf")
	answers := ask(t, sceneQuestion{Krilla: fallbackScene(broken)}, sceneQuestion{Krilla: fallbackScene(good)})
	if !strings.Contains(answers[0].Err, "failed to subset font") {
		t.Errorf("krilla gives %q for the font, want an error of the subset", answers[0].Err)
	}
	if answers[1].Err != "" {
		t.Fatalf("krilla gives an error for the font that does not fail: %s", answers[1].Err)
	}

	// The fallback off.
	data, err := os.ReadFile(broken)
	if err != nil {
		t.Fatal(err)
	}
	font, err := pdf.NewFont(data, 0)
	if err != nil {
		t.Fatal(err)
	}
	d := pdf.NewDocument(pdf.WithSubsetFallback(false))
	ps, _ := pdf.PageSettingsFromWH(200, 200)
	d.StartPage(ps).Surface().DrawText(path.PointFromXY(10, 50), font, 16, "hi there", false, pdf.TextDirectionAuto)
	if err := d.Finish(&bytes.Buffer{}); !errors.Is(err, okage.ErrPDFFont) || !errors.Is(err, okage.ErrSubsetMalformedFont) {
		t.Errorf("Finish without the fallback gives %v, want %v", err, okage.ErrPDFFont)
	}

	// The fallback on.
	got, err := goScene(fallbackScene(broken))
	if err != nil {
		t.Fatal(err)
	}
	f := checkFile(t, "fallback.pdf", got, 200, 200)
	for _, o := range f.Objects {
		dict, _ := o.Value.(*pdfDict)
		if dict.get("FontFile2") != nil || dict.get("FontFile3") != nil {
			t.Errorf("object %d embeds a font", o.Ref.Num)
		}
	}
	var content []byte
	for _, o := range f.Objects {
		if o.IsStream && bytes.Contains(o.Data, []byte(" Tr")) {
			content = append(content, o.Data...)
		}
	}
	for _, want := range []string{"3 Tr", "\nf", "\nS"} {
		if !bytes.Contains(content, []byte(want)) {
			t.Errorf("the content streams have no %q", want)
		}
	}

	file := writeTemp(t, "fallback.pdf", got)
	want := writeTemp(t, "krilla.pdf", answers[1].Data)
	for _, r := range []struct {
		name   string
		render func(*testing.T, string, int, float64) *image.NRGBA
	}{
		{"mutool krilla", renderMuPDF},
		{"pdftoppm krilla", renderPoppler},
	} {
		t.Run(r.name, func(t *testing.T) {
			a, b := r.render(t, file, 1, 72), r.render(t, want, 1, 72)
			n := diffPixels(a, b, 32)
			t.Logf("%d of %d pixels differ from the file of krilla", n, 200*200)
			if n > 200*200/100 {
				t.Errorf("%d pixels differ from the file of krilla, above 1 percent", n)
			}
		})
	}
	if text := strings.Fields(pdfText(t, file)); strings.Join(text, " ") != "hi there hi there hi there" {
		t.Errorf("pdftotext extracts %q", text)
	}
}
