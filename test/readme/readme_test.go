// Package readme tests the examples of README.md. Each example is a function
// of this file, between a line "// readme:<name>" and a line
// "// readme:end". TestReadme makes sure that README.md holds the code of
// each example in a block of Go code, and that each example draws the image
// docs/images/<name>.png. Run the test with -readme.update to write the
// images again.
package readme

import (
	"bytes"
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/lottie"
	"github.com/xo/okage/pdf"
	"github.com/xo/okage/pdfsvg"
	"github.com/xo/okage/svg"
	"github.com/xo/okage/texmath"
	"github.com/xo/okage/texmath/stixtwomath"
	"github.com/xo/okage/typesetting/font"
)

var update = flag.Bool("readme.update", false, "write the images of README.md again")

// root is the root of the repository.
const root = "../.."

func shapes(out *image.Image) error {
	// readme:shapes
	pixmap, _ := okage.NewPixmap(480, 240)
	pixmap.Fill(okage.ColorFromRGBA8(255, 255, 255, 255))

	// Fill a circle with a radial gradient.
	circle, _ := okage.PathFromCircle(120, 120, 96)
	paint := okage.DefaultPaint()
	paint.AntiAlias = true
	paint.Shader, _ = okage.NewRadialGradient(
		okage.PointFromXY(84, 84), 0,
		okage.PointFromXY(120, 120), 96,
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(255, 214, 102, 255)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(232, 89, 60, 255)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	)
	pixmap.FillPath(circle, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	// Stroke a curve with round dashes.
	pb := okage.NewPathBuilder()
	pb.MoveTo(250, 200)
	pb.CubicTo(290, 20, 400, 20, 450, 200)
	curve, _ := pb.Finish()
	paint.SetColorRGBA8(59, 113, 96, 255)
	stroke := okage.DefaultStroke()
	stroke.Width = 10
	stroke.LineCap = okage.LineCapRound
	stroke.Dash, _ = okage.NewStrokeDash([]float32{18, 18}, 0)
	pixmap.StrokePath(curve, &paint, &stroke, okage.IdentityTransform(), nil)

	if err := pixmap.SavePNG("shapes.png"); err != nil {
		return err
	}
	// readme:end
	*out = pixmap
	return nil
}

func logo(out *image.Image) error {
	// readme:svg
	data, err := os.ReadFile("docs/okage.svg")
	if err != nil {
		return err
	}
	r := svg.New(svg.WithWidth(480))
	img, err := r.Render(data)
	if err != nil {
		return err
	}
	// readme:end
	*out = img
	return nil
}

func text(out *image.Image) error {
	latin, arabic, japanese := fonts[0], fonts[1], fonts[2]
	// readme:text
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="480" height="80">
  <rect width="100%" height="100%" fill="white"/>
  <text x="240" y="52" font-family="Noto Sans" font-size="32"
        text-anchor="middle" fill="#3B7160">Hello, مرحبا, こんにちは</text>
</svg>`)
	r := svg.New(
		svg.WithLoadSystemFonts(false),
		svg.WithFontFile(latin),
		svg.WithFontFile(arabic),
		svg.WithFontFile(japanese),
	)
	img, err := r.Render(data)
	if err != nil {
		return err
	}
	// readme:end
	*out = img
	return nil
}

func formula(out *image.Image) error {
	// readme:math
	f, err := stixtwomath.Font()
	if err != nil {
		return err
	}
	tex := `x = \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}`
	box, err := texmath.LayoutTeX(tex, font.NewFace(f), 40, texmath.Display)
	if err != nil {
		return err
	}
	w := uint32(math.Ceil(box.Width)) + 32
	h := uint32(math.Ceil(box.Height+box.Depth)) + 32
	pixmap, _ := okage.NewPixmap(w, h)
	pixmap.Fill(okage.ColorFromRGBA8(255, 255, 255, 255))
	paint := okage.DefaultPaint()
	paint.AntiAlias = true
	box.Draw(pixmap, okage.TransformFromTranslate(16, 16+float32(box.Height)), &paint)
	// readme:end
	*out = pixmap
	return nil
}

func animation(out *image.Image) error {
	data, err := os.ReadFile("lottie/testdata/1f525-fire.json")
	if err != nil {
		return err
	}
	strip := image.NewRGBA(image.Rect(0, 0, 4*160, 160))
	draw.Draw(strip, strip.Bounds(), image.White, image.Point{}, draw.Src)
	c, err := lottie.Load(data)
	if err != nil {
		return err
	}
	for i := range 4 {
		img, err := c.Image(i*c.FrameCount()/4, 160, 160)
		if err != nil {
			return err
		}
		draw.Draw(strip, img.Bounds().Add(image.Pt(i*160, 0)), img, image.Point{}, draw.Over)
	}
	*out = strip
	return nil
}

// The examples of lottie and pdf in README.md, which draw no image here.
func lottieExample() (image.Image, error) {
	data, _ := os.ReadFile("lottie/testdata/1f525-fire.json")
	// readme:lottie
	c, err := lottie.Load(data)
	if err != nil {
		return nil, err
	}
	// Draw the middle frame at 256 by 256 pixels.
	img, err := c.Image(c.FrameCount()/2, 256, 256)
	if err != nil {
		return nil, err
	}
	// readme:end
	return img, nil
}

func pdfExample() ([]byte, error) {
	data, _ := os.ReadFile("docs/okage.svg")
	var w bytes.Buffer
	// readme:pdf
	tree, err := svg.FromData(data)
	if err != nil {
		return nil, err
	}
	d := pdf.NewDocument(pdf.WithSVGGlyphs(pdfsvg.RenderGlyph))
	ps, _ := pdf.PageSettingsFromWH(tree.Size().Width(), tree.Size().Height())
	page := d.StartPage(ps)
	pdfsvg.Draw(page.Surface(), tree, tree.Size())
	page.Finish()
	if err := d.Finish(&w); err != nil {
		return nil, err
	}
	// readme:end
	return w.Bytes(), nil
}

// fonts are the fonts of the example of text: Noto Sans, Amiri and M PLUS
// 1p, from svg/testdata/fonts.
var fonts [3][]byte

// examples are the examples that draw an image, by name.
var examples = []struct {
	name string
	draw func(*image.Image) error
}{
	{"shapes", shapes},
	{"svg", logo},
	{"text", text},
	{"math", formula},
	{"lottie", animation},
}

func TestReadme(t *testing.T) {
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	readme := string(readFile(t, filepath.Join(abs, "README.md")))
	source := string(readFile(t, "readme_test.go"))
	blocks := goBlocks(readme)
	for name, code := range snippets(source) {
		if !strings.Contains(blocks, code) {
			t.Errorf("README.md has no block of Go code with the example %s:\n%s", name, code)
		}
	}

	for i, name := range []string{"NotoSans-Regular.ttf", "Amiri-Regular.ttf", "MPLUS1p-Regular.ttf"} {
		fonts[i] = readFile(t, filepath.Join(abs, "svg", "testdata", "fonts", name))
	}
	// The examples read their files from the root of the repository, and
	// the example of shapes saves shapes.png, so they run in a copy.
	dir := t.TempDir()
	for _, name := range []string{"docs/okage.svg", "lottie/testdata/1f525-fire.json"} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), readFile(t, filepath.Join(abs, name)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)

	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			var img image.Image
			if err := ex.draw(&img); err != nil {
				t.Fatal(err)
			}
			name := filepath.Join(abs, "docs", "images", ex.name+".png")
			if *update {
				var buf bytes.Buffer
				if err := png.Encode(&buf, img); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(name, buf.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := png.Decode(bytes.NewReader(readFile(t, name)))
			if err != nil {
				t.Fatal(err)
			}
			if !samePixels(img, want) {
				t.Errorf("the example %s does not draw %s. Run the test with -readme.update", ex.name, name)
			}
		})
	}

	if _, err := lottieExample(); err != nil {
		t.Error(err)
	}
	data, err := pdfExample()
	if err != nil {
		t.Error(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Error("the example of pdf writes no PDF file")
	}
}

// snippets returns the code of each example of source, by name, without the
// tab of the function body.
func snippets(source string) map[string]string {
	m := map[string]string{}
	name, lines := "", []string(nil)
	for line := range strings.SplitSeq(source, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "// readme:end":
			m[name] = strings.Join(lines, "\n")
			name, lines = "", nil
		case name == "" && strings.HasPrefix(trimmed, "// readme:"):
			name = strings.TrimPrefix(trimmed, "// readme:")
		case name != "":
			lines = append(lines, strings.TrimPrefix(line, "\t"))
		}
	}
	return m
}

// goBlocks returns the blocks of Go code of a Markdown document, one after
// the other.
func goBlocks(doc string) string {
	var b strings.Builder
	in := false
	for line := range strings.SplitSeq(doc, "\n") {
		switch {
		case !in && strings.HasPrefix(line, "```go"):
			in = true
		case in && strings.HasPrefix(line, "```"):
			in = false
			b.WriteString("\n")
		case in:
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

// samePixels reports whether a and b have the same size and the same
// non-premultiplied colors.
func samePixels(a, b image.Image) bool {
	if a.Bounds().Size() != b.Bounds().Size() {
		return false
	}
	ab, bb := a.Bounds(), b.Bounds()
	for y := range ab.Dy() {
		for x := range ab.Dx() {
			c1, _ := color.NRGBAModel.Convert(a.At(ab.Min.X+x, ab.Min.Y+y)).(color.NRGBA)
			c2, _ := color.NRGBAModel.Convert(b.At(bb.Min.X+x, bb.Min.Y+y)).(color.NRGBA)
			if c1 != c2 && (c1.A != 0 || c2.A != 0) {
				return false
			}
		}
	}
	return true
}

func readFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
