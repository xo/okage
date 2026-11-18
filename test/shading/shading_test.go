package shading

import (
	"bytes"
	"fmt"
	"image"
	imgcolor "image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/xo/okage"
)

// size is the width and the height of the page, in points and in pixels.
const size = 200

// tolerance is the largest difference of a channel that the test accepts for
// a pixel.
const tolerance = 8

// maxDiff is the part of the pixels that can differ by more than tolerance.
// These are the pixels at the edges of the shading, which MuPDF
// anti-aliases.
const maxDiff = 0.02

// circles is the geometry of a radial shading: the key Coords of PDF.
type circles struct {
	name                   string
	x0, y0, r0, x1, y1, r1 float32
}

// cases are the shadings of the tests. The first three have circles that do
// not nest, which the rule of PDF and the rule of tiny-skia draw in other
// ways.
var cases = []circles{
	{"apart", 60, 100, 20, 140, 100, 40},
	{"narrows", 70, 100, 50, 140, 90, 10},
	{"cone", 80, 100, 10, 140, 100, 40},
	{"nested", 80, 90, 0, 100, 100, 80},
	{"concentric-reversed", 100, 100, 70, 100, 100, 20},
}

// The colors at t 0 and at t 1, opaque, as PDF gives them to the function of
// type 2.
var (
	c0 = [3]uint8{230, 40, 20}
	c1 = [3]uint8{20, 60, 220}
)

func TestMuPDF(t *testing.T) {
	mutool, err := exec.LookPath("mutool")
	if err != nil {
		t.Skip("mutool is not installed")
	}
	for _, c := range cases {
		for _, extend := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
			t.Run(fmt.Sprintf("%s/extend-%t-%t", c.name, extend[0], extend[1]), func(t *testing.T) {
				want := renderMuPDF(t, mutool, writePDF(t, c, extend))
				got := renderOkage(t, c, extend)
				var diff int
				for y := range size {
					for x := range size {
						if !near(got.NRGBAAt(x, y), want.NRGBAAt(x, y)) {
							diff++
						}
					}
				}
				t.Logf("%d of %d pixels differ by more than %d", diff, size*size, tolerance)
				if float64(diff) > maxDiff*size*size {
					t.Errorf("%d pixels differ by more than %d, want at most %d", diff, tolerance, int(maxDiff*size*size))
				}
			})
		}
	}
}

// near reports whether each channel of a and b differs by tolerance at most.
func near(a, b imgcolor.NRGBA) bool {
	for _, d := range [...]int{
		int(a.R) - int(b.R), int(a.G) - int(b.G), int(a.B) - int(b.B), int(a.A) - int(b.A),
	} {
		if d < -tolerance || d > tolerance {
			return false
		}
	}
	return true
}

// renderOkage fills a white pixmap with the gradient of c.
func renderOkage(t *testing.T, c circles, extend [2]bool) *image.NRGBA {
	t.Helper()
	pixmap, ok := okage.NewPixmap(size, size)
	if !ok {
		t.Fatal("making the pixmap")
	}
	pixmap.Fill(okage.ColorWhite)
	stops := []okage.GradientStop{
		okage.NewGradientStop(0, okage.ColorFromRGBA8(c0[0], c0[1], c0[2], 255)),
		okage.NewGradientStop(1, okage.ColorFromRGBA8(c1[0], c1[1], c1[2], 255)),
	}
	shader, ok := okage.NewRadialGradientExtend(
		okage.PointFromXY(c.x0, c.y0), c.r0, okage.PointFromXY(c.x1, c.y1), c.r1,
		stops, extend[0], extend[1], okage.IdentityTransform())
	if !ok {
		t.Fatal("making the gradient")
	}
	paint := okage.DefaultPaint()
	paint.Shader = shader
	rect, ok := okage.RectFromXYWH(0, 0, size, size)
	if !ok {
		t.Fatal("making the rectangle")
	}
	pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	copy(img.Pix, pixmap.Data())
	return img
}

// writePDF writes a PDF file with one page that paints the shading of c, and
// returns its path. The page flips the axis y, so that the coordinates of the
// shading are the coordinates of the pixels of okage.
func writePDF(t *testing.T, c circles, extend [2]bool) string {
	t.Helper()
	content := fmt.Sprintf("1 0 0 -1 0 %d cm /Sh0 sh", size)
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %d %d] "+
			"/Resources << /Shading << /Sh0 5 0 R >> >> /Contents 4 0 R >>", size, size),
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		fmt.Sprintf("<< /ShadingType 3 /ColorSpace /DeviceRGB /Coords [%g %g %g %g %g %g] "+
			"/Extend [%t %t] /Function << /FunctionType 2 /Domain [0 1] "+
			"/C0 [%s] /C1 [%s] /N 1 >> >>",
			c.x0, c.y0, c.r0, c.x1, c.y1, c.r1, extend[0], extend[1], rgb(c0), rgb(c1)),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n")
	offsets := make([]int, len(objects))
	for i, o := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	name := filepath.Join(t.TempDir(), "shading.pdf")
	if err := os.WriteFile(name, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

// rgb writes a color as three numbers in 0..=1.
func rgb(c [3]uint8) string {
	return fmt.Sprintf("%g %g %g", float64(c[0])/255, float64(c[1])/255, float64(c[2])/255)
}

// renderMuPDF renders the page of a PDF file with mutool draw at 72 dots
// for each inch.
func renderMuPDF(t *testing.T, mutool, file string) *image.NRGBA {
	t.Helper()
	out := filepath.Join(t.TempDir(), "mupdf.png")
	cmd := exec.CommandContext(t.Context(), mutool, "draw", "-q", "-r", "72", "-F", "png", "-o", out, file, "1")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mutool draw %s: %v\n%s", file, err, b)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // the file is only read
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decoding %s: %v", out, err)
	}
	n := image.NewNRGBA(img.Bounds())
	draw.Draw(n, n.Bounds(), img, img.Bounds().Min, draw.Src)
	if n.Bounds().Dx() != size || n.Bounds().Dy() != size {
		t.Fatalf("mutool draws %v, want %d by %d", n.Bounds(), size, size)
	}
	return n
}
