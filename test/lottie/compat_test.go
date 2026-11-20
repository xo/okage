package lottie

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/lottie"

	okagelottie "github.com/xo/okage/lottie"
	"github.com/xo/okage/svg"
)

// compatCase is a Lottie document that okage draws, and the document that
// ThorVG draws to compare with it. When thorvg is nil, ThorVG draws doc.
type compatCase struct {
	name        string
	doc, thorvg []byte
}

// compareCases compares each case of cases with ThorVG, with the sink
// options opts, and logs and reports the results.
func compareCases(t *testing.T, cases []compatCase, opts ...okagelottie.SinkOption) {
	t.Helper()
	ctx := context.Background()
	r, err := lottie.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(ctx) }()
	var results []result
	for _, tc := range cases {
		c, err := okagelottie.Parse(tc.doc)
		if err != nil {
			t.Fatal(err)
		}
		other := tc.thorvg
		if other == nil {
			other = tc.doc
		}
		results = append(results, compareFrames(ctx, r, tc.name, c, other, opts...)...)
	}
	for _, res := range results {
		if res.err == "" {
			t.Logf("%s frame %v: %.3f%% of the pixels differ, largest difference %d", res.name, res.frame, float64(res.differ)*100/float64(res.total), res.maxDiff)
		}
	}
	report(t, results)
}

// edgeLayer returns a shape layer with a rectangle that reaches past the
// left, the top and the bottom of a canvas of 400 by 300 pixels, and a
// circle at the right edge, with the effects effects. The layer moves to the
// right, so the part of the rectangle outside the canvas shrinks.
func edgeLayer(effects string) string {
	return fmt.Sprintf(`{
		"ty": 4, "ind": 1, "ip": 0, "op": 60, "st": 0, "ef": [%s],
		"ks": {"a": {"a": 0, "k": [0, 0]}, "p": {"a": 1, "k": [{"t": 0, "s": [0, 0], "i": {"x": 0.5, "y": 0.5}, "o": {"x": 0.5, "y": 0.5}}, {"t": 59, "s": [40, 0]}]},
			"s": {"a": 0, "k": [100, 100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}},
		"shapes": [
			{"ty": "rc", "p": {"a": 0, "k": [60, 150]}, "s": {"a": 0, "k": [200, 360]}, "r": {"a": 0, "k": 0}},
			{"ty": "el", "p": {"a": 0, "k": [390, 150]}, "s": {"a": 0, "k": [120, 120]}},
			{"ty": "fl", "c": {"a": 0, "k": [0.9, 0.5, 0.1, 1]}, "o": {"a": 0, "k": 100}, "r": 1}
		]
	}`, effects)
}

// TestThorVGBlurEdge compares the Gaussian blur and the drop shadow of a
// layer whose shapes reach past the edges of the canvas with ThorVG (D82).
// ThorVG repeats the pixels at the edge of the canvas outside it, so the
// blur does not fade the shapes at the edge. ThorVG does the same with the
// key "Repeat Edge Pixels" of the blur off and on.
func TestThorVGBlurEdge(t *testing.T) {
	var cases []compatCase
	for _, tc := range []struct {
		name, effects string
	}{
		{"blur-edge", fmt.Sprintf(`{"ty": 29, "en": 1, "ef": [%s, %s, %s]}`, effectParam("30"), effectParam("1"), effectParam("0"))},
		{"blur-edge-repeat", fmt.Sprintf(`{"ty": 29, "en": 1, "ef": [%s, %s, %s]}`, effectParam("30"), effectParam("1"), effectParam("1"))},
		{"blur-edge-vertical", fmt.Sprintf(`{"ty": 29, "en": 1, "ef": [%s, %s, %s]}`, effectParam("40"), effectParam("3"), effectParam("0"))},
		{"shadow-edge", fmt.Sprintf(`{"ty": 25, "en": 1, "ef": [%s, %s, %s, %s, %s]}`,
			effectParam("[0.1, 0.1, 0.4, 1]"), effectParam("200"), effectParam("135"), effectParam("20"), effectParam("30"))},
	} {
		cases = append(cases, compatCase{name: tc.name, doc: textDocument(`{"list": []}`, "[]", edgeLayer(tc.effects))})
	}
	compareCases(t, cases)
}

// TestThorVGTracking compares the tracking of a text that the sink shapes
// with the fonts of WithFontDatabase with ThorVG (D82). ThorVG multiplies
// each advance by 1 plus the tracking over the ascent of the font. The file
// embeds a font of svg/testdata/fonts for ThorVG as a data URL, and okage
// reads the same font from a font database.
func TestThorVGTracking(t *testing.T) {
	ttf, err := os.ReadFile(filepath.Join("..", "..", "svg", "testdata", "fonts", "NotoSans-Regular.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	db := svg.NewFontDatabase()
	db.LoadFontData(ttf)
	noto := fmt.Sprintf(`{"list": [{"fName": "NotoSans-Regular", "fFamily": "Noto Sans", "fStyle": "Regular", "ascent": 75, "origin": 3,
		"fPath": "data:font/ttf;base64,%s"}]}`, base64.StdEncoding.EncodeToString(ttf))
	doc := textDocument(noto, "[]",
		textLayer(1, 200, 60,
			textDoc("NotoSans-Regular", "Wide tracking", `, "j": 2, "tr": 200`),
			textDoc("NotoSans-Regular", "Tight tracking", `, "j": 2, "tr": -60`)),
		textLayer(2, 20, 140,
			textDoc("NotoSans-Regular", "Left 100", `, "j": 0, "tr": 100, "sc": [0.9, 0.2, 0.1], "sw": 1.5`),
			textDoc("NotoSans-Regular", "Left 400", `, "j": 0, "tr": 400`)),
		textLayer(3, 380, 220,
			textDoc("NotoSans-Regular", "Right 150", `, "j": 1, "tr": 150`),
			textDoc("NotoSans-Regular", "Right -30", `, "j": 1, "tr": -30`)))
	compareCases(t, []compatCase{{name: "tracking-font", doc: doc}}, okagelottie.WithFontDatabase(db))
}

// TestThorVGTurnedImage compares an image asset that is scaled up and
// turned with ThorVG (D82). ThorVG fades the edge of such an image over one
// pixel of the image. The image of 16 by 12 pixels fills an asset of 53 by
// 40 pixels, so it scales by about 3.3, and the layer turns to 50 degrees,
// or back to -125 degrees. The third image turns at its own size.
func TestThorVGTurnedImage(t *testing.T) {
	var cases []compatCase
	small := gradientPNG(t, 16, 12)
	for _, tc := range []struct {
		name   string
		img    []byte
		w, h   int
		rotate int
	}{
		{"image-turned-3.3", small, 53, 40, 50},
		{"image-turned-3.3-back", small, 53, 40, -125},
		{"image-turned-1", gradientPNG(t, 60, 40), 60, 40, 35},
	} {
		c := imageCase{name: "data:image/png;base64," + base64.StdEncoding.EncodeToString(tc.img), embedded: 1, w: tc.w, h: tc.h, rotate: tc.rotate}
		cases = append(cases, compatCase{name: tc.name, doc: c.document()})
	}
	compareCases(t, cases)
}
