package lottie

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/lottie"

	okagelottie "github.com/xo/okage/lottie"
	"github.com/xo/okage/svg"
)

// glyphShape returns the shapes of a glyph of the list chars: one group
// with a path for each contour of contours, closed.
func glyphShape(contours ...[][2]int) string {
	var paths []string
	for _, c := range contours {
		var v, z []string
		for _, p := range c {
			v = append(v, fmt.Sprintf("[%d, %d]", p[0], p[1]))
			z = append(z, "[0, 0]")
		}
		paths = append(paths, fmt.Sprintf(`{"ty": "sh", "ks": {"a": 0, "k": {"c": true, "v": [%s], "i": [%s], "o": [%s]}}}`,
			strings.Join(v, ", "), strings.Join(z, ", "), strings.Join(z, ", ")))
	}
	return fmt.Sprintf(`[{"ty": "gr", "it": [%s]}]`, strings.Join(paths, ", "))
}

// testChars is the list chars of the font "Shapes": the glyphs I, L, T and
// O and a space, at the size 100, with the baseline at y = 0.
var testChars = func() string {
	glyphs := []struct {
		ch     string
		w      int
		shapes string
	}{
		{"I", 40, glyphShape([][2]int{{10, -70}, {30, -70}, {30, 0}, {10, 0}})},
		{"L", 65, glyphShape([][2]int{{10, -70}, {25, -70}, {25, -15}, {60, -15}, {60, 0}, {10, 0}})},
		{"T", 70, glyphShape([][2]int{{5, -70}, {65, -70}, {65, -55}, {42, -55}, {42, 0}, {28, 0}, {28, -55}, {5, -55}})},
		{"O", 80, glyphShape([][2]int{{10, -70}, {70, -70}, {70, 0}, {10, 0}}, [][2]int{{25, -55}, {25, -15}, {55, -15}, {55, -55}})},
		{" ", 30, "[]"},
	}
	var list []string
	for _, g := range glyphs {
		list = append(list, fmt.Sprintf(`{"ch": %q, "size": 100, "style": "Regular", "w": %d, "fFamily": "Shapes", "data": {"shapes": %s}}`, g.ch, g.w, g.shapes))
	}
	return "[" + strings.Join(list, ", ") + "]"
}()

// textLayer returns a text layer of 60 frames at (x, y) with the documents
// docs, which change at the frames 0 and 30.
func textLayer(ind, x, y int, docs ...string) string {
	var keyframes []string
	for i, d := range docs {
		keyframes = append(keyframes, fmt.Sprintf(`{"t": %d, "s": %s}`, i*30, d))
	}
	return fmt.Sprintf(`{
		"ty": 5, "ind": %d, "ip": 0, "op": 60, "st": 0,
		"ks": {"a": {"a": 0, "k": [0, 0]}, "p": {"a": 0, "k": [%d, %d]}, "s": {"a": 0, "k": [100, 100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}},
		"t": {"d": {"k": [%s]}, "a": [], "m": {"g": 1, "a": {"a": 0, "k": [0, 0]}}, "p": {}}
	}`, ind, x, y, strings.Join(keyframes, ", "))
}

// textDoc returns a text document of the font font with the extra keys
// extra.
func textDoc(font, text string, extra string) string {
	return fmt.Sprintf(`{"t": %q, "f": %q, "s": 36, "fc": [0.1, 0.3, 0.8], "lh": 44, "j": 0, "tr": 0%s}`, text, font, extra)
}

// textDocument returns a Lottie document of 400 by 300 pixels with the fonts
// fonts, the glyphs chars and the layers layers, over a white solid layer.
func textDocument(fonts, chars string, layers ...string) []byte {
	layers = append(layers, `{"ty": 1, "ind": 99, "ip": 0, "op": 60, "st": 0, "sc": "#ffffff", "sw": 400, "sh": 300,
		"ks": {"a": {"a": 0, "k": [0, 0]}, "p": {"a": 0, "k": [0, 0]}, "s": {"a": 0, "k": [100, 100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}}}`)
	return fmt.Appendf(nil, `{"v": "5.7.0", "fr": 30, "ip": 0, "op": 60, "w": 400, "h": 300,
		"fonts": %s, "chars": %s, "layers": [%s]}`, fonts, chars, strings.Join(layers, ", "))
}

// TestThorVGText compares text layers with ThorVG (D79). The first file
// draws the glyphs of the list chars: two lines that are centered, with a
// stroke, a tracking and a line height, a box text that is aligned to the
// right and breaks its lines, a character without a glyph, and documents
// that change at the frame 30. The second file has no list chars, and its
// font is a font of svg/testdata/fonts, which the file embeds as a data URL
// for ThorVG, and which okage reads from a font database. Its text has no
// tracking, because ThorVG scales the advances of a font that it shapes
// itself by a factor, and okage adds the tracking to each advance, as for
// the glyphs of the list chars.
func TestThorVGText(t *testing.T) {
	ctx := context.Background()
	r, err := lottie.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(ctx) }()

	shapes := `{"list": [{"fName": "Shapes-Regular", "fFamily": "Shapes", "fStyle": "Regular", "ascent": 72, "origin": 0}]}`
	chars := textDocument(shapes, testChars,
		textLayer(1, 200, 70,
			textDoc("Shapes-Regular", "TILT\rLOT IT", `, "j": 2, "tr": 50, "sc": [0.9, 0.2, 0.1], "sw": 3, "of": true`),
			textDoc("Shapes-Regular", "LOTTO\rIT XL", `, "j": 2, "tr": -20, "sc": [0.9, 0.2, 0.1], "sw": 2, "of": false`)),
		textLayer(2, 200, 150,
			textDoc("Shapes-Regular", "LIT LOT TILT OIL TOIL", `, "j": 1, "ps": [-150, 0], "sz": [180, 140], "ls": 4`)),
	)
	c, err := okagelottie.Parse(chars)
	if err != nil {
		t.Fatal(err)
	}
	results := compareFrames(ctx, r, "text-chars", c, chars)

	ttf, err := os.ReadFile(filepath.Join("..", "..", "svg", "testdata", "fonts", "NotoSans-Regular.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	db := svg.NewFontDatabase()
	db.LoadFontData(ttf)
	noto := fmt.Sprintf(`{"list": [{"fName": "NotoSans-Regular", "fFamily": "Noto Sans", "fStyle": "Regular", "ascent": 75, "origin": 3,
		"fPath": "data:font/ttf;base64,%s"}]}`, base64.StdEncoding.EncodeToString(ttf))
	font := textDocument(noto, "[]",
		textLayer(1, 200, 90,
			textDoc("NotoSans-Regular", "Lottie text\rin two lines", `, "j": 2, "sc": [0.9, 0.2, 0.1], "sw": 1.5`),
			textDoc("NotoSans-Regular", "Right aligned", `, "j": 1`)))
	c, err = okagelottie.Parse(font)
	if err != nil {
		t.Fatal(err)
	}
	fontResults := compareFrames(ctx, r, "text-font", c, font, okagelottie.WithFontDatabase(db))
	results = append(results, fontResults...)
	for _, res := range results {
		if res.err == "" {
			t.Logf("%s frame %v: %.3f%% of the pixels differ, largest difference %d", res.name, res.frame, float64(res.differ)*100/float64(res.total), res.maxDiff)
		}
	}
	report(t, results)
}

// effectLayer returns a shape layer with a rectangle and a circle, and the
// effects effects.
func effectLayer(effects string) string {
	return fmt.Sprintf(`{
		"ty": 4, "ind": 1, "ip": 0, "op": 60, "st": 0, "ef": [%s],
		"ks": {"a": {"a": 0, "k": [0, 0]}, "p": {"a": 0, "k": [200, 150]}, "s": {"a": 0, "k": [100, 100]},
			"r": {"a": 1, "k": [{"t": 0, "s": [0], "i": {"x": [0.5], "y": [0.5]}, "o": {"x": [0.5], "y": [0.5]}}, {"t": 59, "s": [40]}]}, "o": {"a": 0, "k": 100}},
		"shapes": [
			{"ty": "rc", "p": {"a": 0, "k": [-40, 0]}, "s": {"a": 0, "k": [100, 70]}, "r": {"a": 0, "k": 0}},
			{"ty": "el", "p": {"a": 0, "k": [50, -20]}, "s": {"a": 0, "k": [80, 80]}},
			{"ty": "fl", "c": {"a": 0, "k": [0.9, 0.5, 0.1, 1]}, "o": {"a": 0, "k": 100}, "r": 1}
		]
	}`, effects)
}

// effectParam returns a parameter of an effect with the value v.
func effectParam(v string) string {
	return fmt.Sprintf(`{"ty": 0, "v": {"a": 0, "k": %s}}`, v)
}

// TestThorVGEffects compares the Gaussian blur and the drop shadow of a layer
// with ThorVG (D79). The layer turns, so the shadow turns with it.
func TestThorVGEffects(t *testing.T) {
	ctx := context.Background()
	r, err := lottie.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(ctx) }()
	cases := []struct {
		name, effects string
	}{
		{"blur", fmt.Sprintf(`{"ty": 29, "en": 1, "ef": [%s, %s, %s]}`, effectParam("12"), effectParam("1"), effectParam("0"))},
		{"blur-horizontal", fmt.Sprintf(`{"ty": 29, "en": 1, "ef": [%s, %s, %s]}`, effectParam("20"), effectParam("2"), effectParam("0"))},
		{"shadow", fmt.Sprintf(`{"ty": 25, "en": 1, "ef": [%s, %s, %s, %s, %s]}`,
			effectParam("[0.1, 0.1, 0.4, 1]"), effectParam("160"), effectParam("135"), effectParam("14"), effectParam("10"))},
		{"shadow-hard", fmt.Sprintf(`{"ty": 25, "en": 1, "ef": [%s, %s, %s, %s, %s]}`,
			effectParam("[0, 0, 0, 1]"), effectParam("255"), effectParam("60"), effectParam("9"), effectParam("0"))},
	}
	var results []result
	for _, tc := range cases {
		doc := textDocument(`{"list": []}`, "[]", effectLayer(tc.effects))
		c, err := okagelottie.Parse(doc)
		if err != nil {
			t.Fatal(err)
		}
		results = append(results, compareFrames(ctx, r, tc.name, c, doc)...)
	}
	for _, res := range results {
		if res.err == "" {
			t.Logf("%s frame %v: %.3f%% of the pixels differ, largest difference %d", res.name, res.frame, float64(res.differ)*100/float64(res.total), res.maxDiff)
		}
	}
	report(t, results)
}
