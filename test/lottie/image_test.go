package lottie

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/xo/lottie"

	okagelottie "github.com/xo/okage/lottie"
)

// gradientPNG returns a PNG image of w by h pixels. Its red grows from left
// to right, its green from top to bottom, and its right half fades out, so a
// wrong position, scale or alpha changes the colors.
func gradientPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			a := 255
			if x >= w/2 {
				a -= (x - w/2 + 1) * 180 / w
			}
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 255 / w), uint8(y * 255 / h), 128, uint8(a)})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// imageCase is a Lottie document of 100 by 100 pixels and 30 frames with an
// image layer over a solid layer. The image layer moves, scales and has an
// opacity of 80.
type imageCase struct {
	// dir and name are the keys u and p of the asset, and embedded is its
	// key e.
	dir, name string
	embedded  int
	// w and h are the size of the asset.
	w, h int
	// rotate is the angle that the layer turns to, in degrees.
	rotate int
	// masked adds a diamond mask to the image layer.
	masked bool
}

// document returns the Lottie document of c.
func (c imageCase) document() []byte {
	mask := ""
	if c.masked {
		mask = fmt.Sprintf(`"hasMask": true, "masksProperties": [{"mode": "a", "o": {"a": 0, "k": 100},
			"pt": {"a": 0, "k": {"c": true, "v": [[%[1]d, -5], [%[3]d, %[2]d], [%[1]d, %[4]d], [-5, %[2]d]],
				"i": [[0, 0], [0, 0], [0, 0], [0, 0]], "o": [[0, 0], [0, 0], [0, 0], [0, 0]]}}}],`, c.w/2, c.h/2, c.w+5, c.h+5)
	}
	return fmt.Appendf(nil, `{
		"v": "5.7.0", "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
		"assets": [{"id": "img", "w": %d, "h": %d, "u": %q, "p": %q, "e": %d}],
		"layers": [{
			"ty": 2, "ind": 1, "ip": 0, "op": 30, "st": 0, "refId": "img", %s
			"ks": {
				"a": {"a": 0, "k": [%d, %d]},
				"p": {"a": 1, "k": [{"t": 0, "s": [50, 50], "i": {"x": 0.5, "y": 0.5}, "o": {"x": 0.5, "y": 0.5}}, {"t": 29, "s": [56, 44]}]},
				"s": {"a": 1, "k": [{"t": 0, "s": [100, 100], "i": {"x": [0.5, 0.5], "y": [0.5, 0.5]}, "o": {"x": [0.5, 0.5], "y": [0.5, 0.5]}}, {"t": 29, "s": [130, 90]}]},
				"r": {"a": 1, "k": [{"t": 0, "s": [0], "i": {"x": [0.5], "y": [0.5]}, "o": {"x": [0.5], "y": [0.5]}}, {"t": 29, "s": [%d]}]},
				"o": {"a": 0, "k": 80}
			}
		}, {
			"ty": 1, "ind": 2, "ip": 0, "op": 30, "st": 0, "sc": "#336699", "sw": 100, "sh": 100,
			"ks": {"a": {"a": 0, "k": [0, 0]}, "p": {"a": 0, "k": [0, 0]}, "s": {"a": 0, "k": [100, 100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}}
		}]
	}`, c.w, c.h, c.dir, c.name, c.embedded, mask, c.w/2, c.h/2, c.rotate)
}

// dotLottie returns a dotLottie archive with the document doc and the image
// file images/img_0.png.
func dotLottie(t *testing.T, doc, img []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range []struct {
		name string
		data []byte
	}{
		{"manifest.json", []byte(`{"version": "1", "animations": [{"id": "image"}]}`)},
		{"animations/image.json", doc},
		{"images/img_0.png", img},
	} {
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
	return buf.Bytes()
}

// TestThorVGImage compares the image assets of D75 with ThorVG: an embedded
// PNG image at its own size that turns, and a dotLottie archive with a PNG
// file of half the size of its asset, which ThorVG scales by 2 to fit in the
// asset. ThorVG, as github.com/xo/lottie runs it, reads only a Lottie
// document, and no file that it names. So for the archive, ThorVG draws the
// same document with the image embedded.
func TestThorVGImage(t *testing.T) {
	ctx := context.Background()
	r, err := lottie.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(ctx) }()

	img := gradientPNG(t, 60, 40)
	embedded := imageCase{name: "data:image/png;base64," + base64.StdEncoding.EncodeToString(img), embedded: 1, w: 60, h: 40, rotate: 35}.document()
	c, err := okagelottie.Parse(embedded)
	if err != nil {
		t.Fatal(err)
	}
	results := compareFrames(ctx, r, "embedded-png", c, embedded)

	// The asset is 60 by 50, so the image of 30 by 20 scales by 2, the
	// smaller of the two ratios, and keeps its aspect ratio.
	half := gradientPNG(t, 30, 20)
	archived := imageCase{dir: "/images/", name: "img_0.png", w: 60, h: 50, masked: true}
	archive := dotLottie(t, archived.document(), half)
	archived.dir, archived.name, archived.embedded = "", "data:image/png;base64,"+base64.StdEncoding.EncodeToString(half), 1
	c, err = okagelottie.Load(archive)
	if err != nil {
		t.Fatal(err)
	}
	results = append(results, compareFrames(ctx, r, "dotlottie-png", c, archived.document())...)
	for _, res := range results {
		if res.err == "" {
			t.Logf("%s frame %v: %.3f%% of the pixels differ, largest difference %d", res.name, res.frame, float64(res.differ)*100/float64(res.total), res.maxDiff)
		}
	}
	report(t, results)
}
