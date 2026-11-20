// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/xo/okage"
)

// quadPNG returns a PNG image of 2 by 2 pixels: red, green, blue and white,
// row by row.
func quadPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	img.Set(1, 0, color.NRGBA{0, 255, 0, 255})
	img.Set(0, 1, color.NRGBA{0, 0, 255, 255})
	img.Set(1, 1, color.NRGBA{255, 255, 255, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// imageDoc returns a Lottie document of 20 by 20 pixels with one image layer
// of the opacity opacity. Its asset is 20 by 20, with the directory dir and
// the file name name.
func imageDoc(dir, name string, opacity int) []byte {
	return sizedImageDoc(dir, name, opacity, 20, 20)
}

// sizedImageDoc is imageDoc with an asset of w by h.
func sizedImageDoc(dir, name string, opacity, w, h int) []byte {
	return fmt.Appendf(nil, `{
		"v": "5.7.0", "fr": 30, "ip": 0, "op": 10, "w": 20, "h": 20,
		"assets": [{"id": "img", "w": %d, "h": %d, "u": %q, "p": %q, "e": 0}],
		"layers": [{
			"ty": 2, "ind": 1, "ip": 0, "op": 10, "st": 0, "refId": "img",
			"ks": {
				"a": {"a": 0, "k": [0, 0]}, "p": {"a": 0, "k": [0, 0]},
				"s": {"a": 0, "k": [100, 100]}, "r": {"a": 0, "k": 0},
				"o": {"a": 0, "k": %d}
			}
		}]
	}`, w, h, dir, name, opacity)
}

// dataURL returns data as a data URL of the media type mime.
func dataURL(mime string, data []byte) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// pixelAt returns the premultiplied color of the pixel (x, y) of p.
func pixelAt(p *image.RGBA, x, y int) [4]uint8 {
	d := p.Pix[p.PixOffset(x, y):]
	return [4]uint8{d[0], d[1], d[2], d[3]}
}

// renderImage draws frame 0 of c onto a new pixmap of its size, with a sink
// of the options opts.
func renderImage(t *testing.T, c *Composition, opts ...SinkOption) *image.RGBA {
	t.Helper()
	pm, ok := okage.NewPixmap(uint32(c.Width), uint32(c.Height))
	if !ok {
		t.Fatal("no pixmap")
	}
	m := pm.AsMut()
	if err := NewRenderer().Append(c, 0, IdentityAffine, 1, NewPixmapSink(&m, opts...)); err != nil {
		t.Fatal(err)
	}
	return pm.AsRGBA()
}

// checkQuad checks that the corners of p show the four colors of quadPNG,
// scaled to the size of p, with the alpha a.
func checkQuad(t *testing.T, p *image.RGBA, a uint8) {
	t.Helper()
	w, h := p.Rect.Dx(), p.Rect.Dy()
	want := map[[2]int][4]uint8{
		{1, 1}:         {a, 0, 0, a},
		{w - 2, 1}:     {0, a, 0, a},
		{1, h - 2}:     {0, 0, a, a},
		{w - 2, h - 2}: {a, a, a, a},
	}
	for xy, c := range want {
		got := pixelAt(p, xy[0], xy[1])
		for i := range 4 {
			if d := int(got[i]) - int(c[i]); d < -1 || d > 1 {
				t.Errorf("pixel %v is %v, want %v", xy, got, c)
				break
			}
		}
	}
}

// TestDrawImageDataURL checks that an embedded PNG image fills the size of
// its asset, with the opacity of its layer.
func TestDrawImageDataURL(t *testing.T) {
	for _, opacity := range []int{100, 50} {
		c, err := Parse(imageDoc("", dataURL("image/png", quadPNG(t)), opacity))
		if err != nil {
			t.Fatal(err)
		}
		a := uint8(255)
		if opacity == 50 {
			a = 128
		}
		checkQuad(t, renderImage(t, c), a)
	}
}

// TestDrawImageKeepsAspect checks that an image keeps its aspect ratio in an
// asset of another shape, as ThorVG draws it: the square image fits in the
// height of an asset of 20 by 10, at its top-left corner.
func TestDrawImageKeepsAspect(t *testing.T) {
	c, err := Parse(sizedImageDoc("", dataURL("image/png", quadPNG(t)), 100, 20, 10))
	if err != nil {
		t.Fatal(err)
	}
	p := renderImage(t, c)
	tests := []struct {
		x, y int
		want [4]uint8
	}{
		{1, 1, [4]uint8{255, 0, 0, 255}},
		{8, 8, [4]uint8{255, 255, 255, 255}},
		{15, 5, [4]uint8{}},
		{5, 15, [4]uint8{}},
	}
	for _, test := range tests {
		if got := pixelAt(p, test.x, test.y); got != test.want {
			t.Errorf("pixel (%d, %d) is %v, want %v", test.x, test.y, got, test.want)
		}
	}
}

// TestDrawImageFS checks that an image that the asset names by a path comes
// from the files of WithFS, and that it draws nothing without them.
func TestDrawImageFS(t *testing.T) {
	c, err := Parse(imageDoc("/images/", "quad.png", 100))
	if err != nil {
		t.Fatal(err)
	}
	if p := renderImage(t, c); pixelAt(p, 10, 10) != [4]uint8{} {
		t.Errorf("without files, pixel (10, 10) is %v, want transparent", pixelAt(p, 10, 10))
	}
	fsys := fstest.MapFS{"images/quad.png": {Data: quadPNG(t)}}
	checkQuad(t, renderImage(t, c, WithFS(fsys)), 255)
}

// TestDrawImageDotLottie checks that Load and Composition.Image read the
// image of a dotLottie archive from the archive.
func TestDrawImageDotLottie(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := []struct {
		name string
		data []byte
	}{
		{"manifest.json", []byte(`{"animations": [{"id": "a"}]}`)},
		{"animations/a.json", imageDoc("/images/", "img_0.png", 100)},
		{"images/img_0.png", quadPNG(t)},
	}
	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(f.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	c, err := Load(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{20, 40} {
		img, err := c.Image(0, size, size)
		if err != nil {
			t.Fatal(err)
		}
		checkQuad(t, img, 255)
	}
}

// TestDrawImageBad checks that an image that does not decode draws nothing,
// and that the sink logs it once for all the frames.
func TestDrawImageBad(t *testing.T) {
	c, err := Parse(imageDoc("", dataURL("image/png", []byte("not a png")), 100))
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	pm, _ := okage.NewPixmap(20, 20)
	m := pm.AsMut()
	sink := NewPixmapSink(&m, WithLogger(slog.New(slog.NewTextHandler(&log, nil))))
	for frame := range 3 {
		if err := NewRenderer().Append(c, float64(frame), IdentityAffine, 1, sink); err != nil {
			t.Fatal(err)
		}
	}
	if p := pixelAt(pm.AsRGBA(), 10, 10); p != [4]uint8{} {
		t.Errorf("pixel (10, 10) is %v, want transparent", p)
	}
	if n := strings.Count(log.String(), "cannot draw an image asset"); n != 1 {
		t.Errorf("the sink logged %d times, want 1:\n%s", n, log.String())
	}
}

// webpQuad is a lossless WebP image of 2 by 2 pixels: red, green, blue and
// white with an alpha of 128, row by row.
const webpQuad = "UklGRjAAAABXRUJQVlA4TCMAAAAvAUAAEB8gEEjeHzqN+RcQFPwfnYCg6LrlImYPwg0YIvofAgA="

// TestDecodeImage checks the decoding of each format into premultiplied
// pixels.
func TestDecodeImage(t *testing.T) {
	gray := image.NewGray(image.Rect(0, 0, 8, 8))
	for i := range gray.Pix {
		gray.Pix[i] = 100
	}
	var jpg, gf bytes.Buffer
	if err := jpeg.Encode(&jpg, gray, nil); err != nil {
		t.Fatal(err)
	}
	pal := image.NewPaletted(image.Rect(0, 0, 2, 1), color.Palette{color.RGBA{0, 0, 0, 0}, color.RGBA{0, 0, 255, 255}})
	pal.Pix[1] = 1
	if err := gif.Encode(&gf, pal, nil); err != nil {
		t.Fatal(err)
	}
	webp, err := base64.StdEncoding.DecodeString(webpQuad)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		data []byte
		x, y int
		want [4]uint8
	}{
		{"png", quadPNG(t), 1, 0, [4]uint8{0, 255, 0, 255}},
		{"jpeg", jpg.Bytes(), 4, 4, [4]uint8{100, 100, 100, 255}},
		{"gif transparent", gf.Bytes(), 0, 0, [4]uint8{}},
		{"gif", gf.Bytes(), 1, 0, [4]uint8{0, 0, 255, 255}},
		{"webp", webp, 0, 0, [4]uint8{255, 0, 0, 255}},
		{"webp alpha", webp, 1, 1, [4]uint8{128, 128, 128, 128}},
	}
	for _, test := range tests {
		p, err := decodeImage(test.data)
		if err != nil {
			t.Errorf("%s: %v", test.name, err)
			continue
		}
		if got := pixelAt(p.AsRGBA(), test.x, test.y); got != test.want {
			t.Errorf("%s: pixel (%d, %d) is %v, want %v", test.name, test.x, test.y, got, test.want)
		}
	}
	if _, err := decodeImage([]byte("BM")); err == nil {
		t.Error("an unknown format decodes")
	}
}

// TestDecodeDataURL checks base64 with white space and no padding, the
// percent encoding, and a URL with no comma.
func TestDecodeDataURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"data:image/png;base64,aGVs\n bG8", "hello"},
		{"data:image/png;BASE64,aGVsbG8=", "hello"},
		{"data:,a%20b", "a b"},
	}
	for _, test := range tests {
		got, err := decodeDataURL(test.in)
		if err != nil || string(got) != test.want {
			t.Errorf("decodeDataURL(%q) = %q, %v, want %q", test.in, got, err, test.want)
		}
	}
	if _, err := decodeDataURL("data:image/png;base64"); err == nil {
		t.Error("a data url with no comma decodes")
	}
}
