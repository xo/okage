package krilla

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// The helpers of this file run the outside programs of D62: MuPDF `mutool
// draw` and Poppler `pdftoppm` render a page, and `qpdf --check` looks for
// faults in a file. A test that calls one skips when the program is missing.

// needProgram returns the path of a program, or skips the test.
func needProgram(t *testing.T, name string) string {
	t.Helper()
	p, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s is not installed", name)
	}
	return p
}

// writeTemp writes the data of a PDF file into the temporary folder of the
// test, and returns its path.
func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// renderMuPDF renders a page of a PDF file with `mutool draw`, at a
// resolution in dots for each inch. The first page is 1.
func renderMuPDF(t *testing.T, file string, page int, dpi float64) *image.NRGBA {
	t.Helper()
	mutool := needProgram(t, "mutool")
	out := filepath.Join(t.TempDir(), "mupdf.png")
	cmd := exec.CommandContext(t.Context(), mutool, "draw", "-q",
		"-r", strconv.FormatFloat(dpi, 'f', -1, 64), "-F", "png", "-o", out,
		file, strconv.Itoa(page))
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mutool draw %s: %v\n%s", file, err, b)
	}
	if len(b) > 0 {
		t.Logf("mutool draw %s: %s", file, b)
	}
	return readPNG(t, out)
}

// renderPoppler renders a page of a PDF file with `pdftoppm`, at a
// resolution in dots for each inch. The first page is 1.
func renderPoppler(t *testing.T, file string, page int, dpi float64) *image.NRGBA {
	t.Helper()
	pdftoppm := needProgram(t, "pdftoppm")
	prefix := filepath.Join(t.TempDir(), "poppler")
	r := strconv.FormatFloat(dpi, 'f', -1, 64)
	p := strconv.Itoa(page)
	cmd := exec.CommandContext(t.Context(), pdftoppm, "-png", "-r", r,
		"-f", p, "-l", p, "-singlefile", file, prefix)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pdftoppm %s: %v\n%s", file, err, b)
	}
	if len(b) > 0 {
		t.Logf("pdftoppm %s: %s", file, b)
	}
	return readPNG(t, prefix+".png")
}

// readPNG reads a PNG file into an NRGBA image.
func readPNG(t *testing.T, name string) *image.NRGBA {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // the file is only read
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decoding %s: %v", name, err)
	}
	if n, ok := img.(*image.NRGBA); ok {
		return n
	}
	n := image.NewNRGBA(img.Bounds())
	draw.Draw(n, n.Bounds(), img, img.Bounds().Min, draw.Src)
	return n
}

// qpdfCheck runs `qpdf --check` on a PDF file. The test fails when qpdf
// finds an error or a warning.
func qpdfCheck(t *testing.T, file string) {
	t.Helper()
	qpdf := needProgram(t, "qpdf")
	b, err := exec.CommandContext(t.Context(), qpdf, "--check", file).CombinedOutput()
	if err != nil {
		t.Errorf("qpdf --check %s: %v\n%s", file, err, b)
	}
}

// diffPixels counts the pixels of two images that differ, with the metric
// of D31: a pixel differs when one of its four channels differs by more
// than threshold, or when only one of the images has it. D31 uses the
// threshold 1, and a comparison with another anti-aliasing uses 32.
func diffPixels(a, b *image.NRGBA, threshold int) int {
	ab, bb := a.Bounds(), b.Bounds()
	w, h := max(ab.Dx(), bb.Dx()), max(ab.Dy(), bb.Dy())
	n := 0
	for y := range h {
		for x := range w {
			pa := image.Pt(ab.Min.X+x, ab.Min.Y+y)
			pb := image.Pt(bb.Min.X+x, bb.Min.Y+y)
			if !pa.In(ab) || !pb.In(bb) {
				n++
				continue
			}
			ia, ib := a.PixOffset(pa.X, pa.Y), b.PixOffset(pb.X, pb.Y)
			for c := range 4 {
				d := int(a.Pix[ia+c]) - int(b.Pix[ib+c])
				if d > threshold || -d > threshold {
					n++
					break
				}
			}
		}
	}
	return n
}

// describeImage gives the size of an image and the number of its pixels
// that are not white, for a message of a test.
func describeImage(img *image.NRGBA) string {
	n := 0
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 255 || img.Pix[i+1] != 255 || img.Pix[i+2] != 255 {
			n++
		}
	}
	return fmt.Sprintf("%dx%d with %d pixels that are not white", img.Bounds().Dx(), img.Bounds().Dy(), n)
}
