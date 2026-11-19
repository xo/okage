package krilla

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	gobuild "go/build"
	"hash/crc32"
	"image"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage/pdf"
)

// The tests of this file compare the images of the package pdf with krilla,
// through the question "krilla" of the oracle (the gate of phase 15 of the
// PDF writer in docs/PLAN.md). Each scene draws images with draw_image, and
// the module image of the oracle makes the custom images, the PNG files
// without a decode and the JPEG files with a profile. The objects of the two
// files must be the same after the reader decodes the streams. When no
// stream is compressed, the bytes must be the same. When the oracle gives an
// error, pdf must give an error too.

// customSpec is the field "custom" of an image, which the module image of
// the oracle reads.
type customSpec struct {
	Color      string `json:"color"`
	Alpha      string `json:"alpha,omitempty"`
	Bits       int    `json:"bits"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	ColorSpace string `json:"color_space"`
}

// imageCase is a scene with images. The images of the scene and of goImages
// are the same images: the oracle reads the first, and pdf the second.
type imageCase struct {
	name     string
	settings map[string]any
	images   []map[string]any
	goImages []func() (*pdf.Image, error)
	pages    []pageSpec
}

// goImageScene builds the PDF file of an image case with pdf. It makes the
// images, then the calls of the surface with goRun.
func goImageScene(c imageCase) ([]byte, error) {
	images := make([]*pdf.Image, len(c.goImages))
	for i, f := range c.goImages {
		img, err := f()
		if err != nil {
			return nil, err
		}
		images[i] = img
	}
	opts, err := goOptions(c.settings)
	if err != nil {
		return nil, err
	}
	d := pdf.NewDocument(opts...)
	for i, p := range c.pages {
		ps, err := goPageSettings(p)
		if err != nil {
			return nil, err
		}
		page := d.StartPage(ps)
		surface := page.Surface()
		err = goRun(surface, p.Ops, &sceneResources{images: images})
		surface.Finish()
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", i, err)
		}
		page.Finish()
	}
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// goOptions makes the options of the settings of a scene, as goScene does.
func goOptions(settings map[string]any) ([]pdf.Option, error) {
	var opts []pdf.Option
	for k, v := range settings {
		flag, _ := v.(bool)
		switch k {
		case "pretty":
			opts = append(opts, pdf.WithPretty(flag))
		case "ascii_compatible":
			opts = append(opts, pdf.WithASCIICompatible(flag))
		case "compress_content_streams":
			opts = append(opts, pdf.WithCompression(flag))
		case "no_device_cs":
			opts = append(opts, pdf.WithNoDeviceCS(flag))
		case "xmp_metadata":
		case "version":
			versions := map[any]pdf.PDFVersion{
				"1.4": pdf.PDF14, "1.5": pdf.PDF15, "1.6": pdf.PDF16, "1.7": pdf.PDF17, "2.0": pdf.PDF20,
			}
			opts = append(opts, pdf.WithPDFVersion(versions[v]))
		default:
			return nil, fmt.Errorf("the setting %q is not ported", k)
		}
	}
	return opts, nil
}

// fileImage is an image of a file for both sides. kind is the kind of the
// oracle: "png", "png_native", "jpeg", "gif" or "webp".
func fileImage(file, kind string, interpolate bool) (map[string]any, func() (*pdf.Image, error)) {
	spec := map[string]any{"file": file, "kind": kind, "interpolate": interpolate}
	return spec, func() (*pdf.Image, error) {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		switch kind {
		case "png":
			return pdf.ImageFromPNG(data, interpolate)
		case "png_native":
			return pdf.ImageFromPNGNative(data, interpolate)
		case "jpeg":
			return pdf.ImageFromJPEG(data, interpolate)
		case "gif":
			return pdf.ImageFromGIF(data, interpolate)
		default:
			return pdf.ImageFromWebP(data, interpolate)
		}
	}
}

// imageFiles returns the files of the tests of pdf, and the PNG, GIF, JPEG
// and WebP files of the tests of Go and of golang.org/x/image.
func imageFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	glob := func(pattern string) {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	images, err := filepath.Abs(filepath.Join(root, "pdf", "testdata", "images"))
	if err != nil {
		t.Fatal(err)
	}
	glob(filepath.Join(images, "*.*"))
	glob(filepath.Join(images, "png_native", "*.png"))
	goroot := gobuild.Default.GOROOT
	glob(filepath.Join(goroot, "src", "image", "png", "testdata", "pngsuite", "*.png"))
	glob(filepath.Join(goroot, "src", "image", "png", "testdata", "*.png"))
	glob(filepath.Join(goroot, "src", "image", "testdata", "*.*"))
	if out, err := exec.CommandContext(t.Context(), "go", "list", "-m", "-f", "{{.Dir}}", "golang.org/x/image").Output(); err == nil {
		glob(filepath.Join(strings.TrimSpace(string(out)), "testdata", "*.webp"))
	}
	var kept []string
	for _, f := range files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".png", ".jpg", ".jpeg", ".gif", ".webp":
			kept = append(kept, f)
		}
	}
	return kept
}

// kindOf returns the kind of a file by its extension.
func kindOf(file string) string {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".png":
		return "png"
	case ".jpg", ".jpeg":
		return "jpeg"
	case ".gif":
		return "gif"
	}
	return "webp"
}

// drawPage is a page of 200 by 200 that draws each image in a list.
func drawPage(ops ...op) pageSpec {
	return pageSpec{Width: 200, Height: 200, Ops: ops}
}

// imageCases returns the scenes of the images.
func imageCases(t *testing.T) []imageCase {
	t.Helper()
	pretty := map[string]any{"xmp_metadata": false, "pretty": true, "ascii_compatible": true, "compress_content_streams": false}
	settingsList := []map[string]any{
		pretty,
		{"xmp_metadata": false},
		{"xmp_metadata": false, "no_device_cs": true},
		{"xmp_metadata": false, "version": "1.4", "pretty": true, "compress_content_streams": false},
		{"xmp_metadata": false, "version": "2.0", "ascii_compatible": true},
	}
	var cases []imageCase
	for _, file := range imageFiles(t) {
		kind := kindOf(file)
		kinds := []string{kind}
		if kind == "png" {
			kinds = append(kinds, "png_native")
		}
		for _, k := range kinds {
			for i, settings := range settingsList {
				interpolate := i == 1
				spec, goImg := fileImage(file, k, interpolate)
				cases = append(cases, imageCase{
					name:     fmt.Sprintf("%s %s %d", filepath.Base(file), k, i),
					settings: settings,
					images:   []map[string]any{spec},
					goImages: []func() (*pdf.Image, error){goImg},
					pages:    []pageSpec{drawPage(drawImage(0, 150, 100))},
				})
			}
		}
	}

	// Two images, drawn under the calls of the surface, on two pages. Each
	// image is written once.
	images, err := filepath.Abs(filepath.Join(root, "pdf", "testdata", "images"))
	if err != nil {
		t.Fatal(err)
	}
	a, aGo := fileImage(filepath.Join(images, "rgba8.png"), "png", false)
	b, bGo := fileImage(filepath.Join(images, "cmyk.jpg"), "jpeg", false)
	ops := []op{
		pushTransform(0.5, 0.2, -0.2, 0.5, 30, 10), drawImage(0, 100, 100), pop(),
		pushOpacity(0.5), drawImage(1, 80, 40), pop(),
		pushClipPath(rect(10, 10, 50, 50), "evenodd"), drawImage(0, 200, 200), pop(),
		pushIsolated(), pushBlendMode("multiply"), drawImage(1, 20, 20), pop(), pop(),
	}
	for i, settings := range settingsList {
		cases = append(cases, imageCase{
			name:     fmt.Sprintf("two images %d", i),
			settings: settings,
			images:   []map[string]any{a, b},
			goImages: []func() (*pdf.Image, error){aGo, bGo},
			pages:    []pageSpec{drawPage(ops...), drawPage(drawImage(1, 10, 10), drawImage(0, 10, 10))},
		})
	}

	// The custom images of Go, and the RGBA images of from_rgba8.
	r := rand.New(rand.NewPCG(15, 1))
	for i, img := range customImages(r) {
		spec := customOf(img)
		for j, settings := range settingsList {
			cases = append(cases, imageCase{
				name:     fmt.Sprintf("custom %d %T %d", i, img, j),
				settings: settings,
				images:   []map[string]any{{"custom": spec, "interpolate": j == 1}},
				goImages: []func() (*pdf.Image, error){func() (*pdf.Image, error) { return pdf.ImageFromImage(img, j == 1) }},
				pages:    []pageSpec{drawPage(drawImage(0, 100, 120))},
			})
		}
	}
	for i := range 4 {
		w, h := 1+r.IntN(30), 1+r.IntN(30)
		nrgba := image.NewNRGBA(image.Rect(0, 0, w, h))
		for j := range nrgba.Pix {
			nrgba.Pix[j] = uint8(r.IntN(256))
			if i%2 == 0 && j%4 == 3 {
				nrgba.Pix[j] = 255
			}
		}
		pix := nrgba.Pix
		cases = append(cases, imageCase{
			name:     fmt.Sprintf("rgba %d", i),
			settings: pretty,
			images:   []map[string]any{{"rgba": base64.StdEncoding.EncodeToString(pix), "width": w, "height": h}},
			goImages: []func() (*pdf.Image, error){func() (*pdf.Image, error) {
				return pdf.ImageFromRGBA8(pix, uint32(w), uint32(h)), nil
			}},
			pages: []pageSpec{drawPage(drawImage(0, 100, 100))},
		})
	}
	return cases
}

// customImages returns random images of Go in the color models that
// ImageFromImage reads.
func customImages(r *rand.Rand) []image.Image {
	rect := func() image.Rectangle { return image.Rect(0, 0, 1+r.IntN(20), 1+r.IntN(20)) }
	fill := func(pix []byte) {
		for i := range pix {
			pix[i] = uint8(r.IntN(256))
		}
	}
	gray := image.NewGray(rect())
	fill(gray.Pix)
	gray16 := image.NewGray16(rect())
	fill(gray16.Pix)
	cmyk := image.NewCMYK(rect())
	fill(cmyk.Pix)
	nrgba := image.NewNRGBA(rect())
	fill(nrgba.Pix)
	nrgba64 := image.NewNRGBA64(rect())
	fill(nrgba64.Pix)
	opaque := image.NewNRGBA(rect())
	fill(opaque.Pix)
	for i := 3; i < len(opaque.Pix); i += 4 {
		opaque.Pix[i] = 255
	}
	return []image.Image{gray, gray16, cmyk, nrgba, nrgba64, opaque}
}

// customOf gives the samples of an image of customImages, for the field
// "custom" of the oracle.
func customOf(img image.Image) customSpec {
	b := img.Bounds()
	c := customSpec{Bits: 8, Width: b.Dx(), Height: b.Dy(), ColorSpace: "rgb"}
	enc := base64.StdEncoding.EncodeToString
	switch m := img.(type) {
	case *image.Gray:
		c.ColorSpace, c.Color = "luma", enc(m.Pix)
	case *image.Gray16:
		c.ColorSpace, c.Bits, c.Color = "luma", 16, enc(m.Pix)
	case *image.CMYK:
		c.ColorSpace, c.Color = "cmyk", enc(m.Pix)
	case *image.NRGBA:
		var col, alpha []byte
		for i := 0; i < len(m.Pix); i += 4 {
			col = append(col, m.Pix[i:i+3]...)
			alpha = append(alpha, m.Pix[i+3])
		}
		c.Color = enc(col)
		if !m.Opaque() {
			c.Alpha = enc(alpha)
		}
	case *image.NRGBA64:
		var col, alpha []byte
		for i := 0; i < len(m.Pix); i += 8 {
			col = append(col, m.Pix[i:i+6]...)
			alpha = append(alpha, m.Pix[i+6:i+8]...)
		}
		c.Bits, c.Color = 16, enc(col)
		if !m.Opaque() {
			c.Alpha = enc(alpha)
		}
	}
	return c
}

// compareImageCase compares the file of pdf for an image case with the
// answer of the oracle. When the oracle gives an error, pdf must give an
// error too.
func compareImageCase(t *testing.T, c imageCase, a answer) {
	t.Helper()
	got, err := goImageScene(c)
	if a.Err != "" {
		if err == nil {
			t.Errorf("the oracle gives the error %q, and pdf gives no error", a.Err)
		}
		return
	}
	if err != nil {
		t.Fatalf("pdf gives the error %v, and the oracle gives no error", err)
	}
	want := a.Data
	compress, ok := c.settings["compress_content_streams"].(bool)
	if ok && !compress && bytes.Equal(got, want) {
		return
	}
	gotFile, err := readPDF(got)
	if err != nil {
		t.Fatalf("reading the file of pdf: %v", err)
	}
	wantFile, err := readPDF(want)
	if err != nil {
		t.Fatalf("reading the file of krilla: %v", err)
	}
	broken := sameBrokenStreams(gotFile, wantFile)
	if d := comparePDF(gotFile, wantFile); len(d) != 0 {
		t.Errorf("%d differences:\n%s", len(d), strings.Join(d[:min(len(d), 10)], "\n"))
	}
	// qpdf reports a stream that does not decode.
	if !broken {
		qpdfCheck(t, writeTemp(t, "go.pdf", got))
	}
}

// sameBrokenStreams accepts the streams that do not decode in both files
// and hold the same bytes, such as the data of a PNG file with a fault that
// both sides copy without a decode. It returns true when it finds one.
func sameBrokenStreams(got, want *pdfFile) bool {
	found := false
	for i := range min(len(got.Objects), len(want.Objects)) {
		g, w := got.Objects[i], want.Objects[i]
		if g.DecodeErr != nil && w.DecodeErr != nil && bytes.Equal(g.Raw, w.Raw) {
			g.DecodeErr, w.DecodeErr = nil, nil
			found = true
		}
	}
	return found
}

// TestImageGo compares the files of pdf with the files of krilla for the
// image cases.
func TestImageGo(t *testing.T) {
	cases := imageCases(t)
	questions := make([]any, len(cases))
	for i, c := range cases {
		questions[i] = map[string]any{"krilla": map[string]any{"settings": c.settings, "images": c.images, "pages": c.pages}}
	}
	answers := ask(t, questions...)
	errs := 0
	for i, c := range cases {
		if answers[i].Err != "" {
			errs++
		}
		t.Run(c.name, func(t *testing.T) {
			compareImageCase(t, c, answers[i])
		})
	}
	t.Logf("%d cases, %d with an error of both sides", len(cases), errs)
}

// TestImageGoBroken compares pdf with krilla for PNG files with random
// changes to the bytes of their chunks IDAT, whose CRC is fixed, so that
// the decoders find the faults. Both sides must accept the same files, and
// a file must give the same objects.
func TestImageGoBroken(t *testing.T) {
	images, err := filepath.Abs(filepath.Join(root, "pdf", "testdata", "images", "png_native"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(images, "*.png"))
	if err != nil {
		t.Fatal(err)
	}
	r := rand.New(rand.NewPCG(15, 2))
	dir := t.TempDir()
	var cases []imageCase
	for i, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for j := range 3 {
			broken := breakIDAT(r, data)
			name := filepath.Join(dir, fmt.Sprintf("%d-%d.png", i, j))
			if err := os.WriteFile(name, broken, 0o644); err != nil {
				t.Fatal(err)
			}
			spec, goImg := fileImage(name, "png", false)
			cases = append(cases, imageCase{
				name:     fmt.Sprintf("%s %d", filepath.Base(f), j),
				settings: map[string]any{"xmp_metadata": false},
				images:   []map[string]any{spec},
				goImages: []func() (*pdf.Image, error){goImg},
				pages:    []pageSpec{drawPage(drawImage(0, 100, 100))},
			})
		}
	}
	questions := make([]any, len(cases))
	for i, c := range cases {
		questions[i] = map[string]any{"krilla": map[string]any{"settings": c.settings, "images": c.images, "pages": c.pages}}
	}
	answers := ask(t, questions...)
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			compareImageCase(t, c, answers[i])
		})
	}
}

// breakIDAT changes a few bytes of the first chunk IDAT of a PNG file, and
// fixes the CRC of the chunk.
func breakIDAT(r *rand.Rand, data []byte) []byte {
	out := append([]byte{}, data...)
	i := bytes.Index(out, []byte("IDAT"))
	if i < 4 {
		return out
	}
	n := int(binary.BigEndian.Uint32(out[i-4:]))
	body := out[i+4 : i+4+n]
	for range 1 + r.IntN(3) {
		if len(body) > 0 {
			body[r.IntN(len(body))] = uint8(r.IntN(256))
		}
	}
	binary.BigEndian.PutUint32(out[i+4+n:], crc32.ChecksumIEEE(out[i:i+4+n]))
	return out
}
