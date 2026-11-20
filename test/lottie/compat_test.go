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

// animatedTextLayer returns a text layer of 60 frames at (x, y) with the
// document doc, the text animators animators, the key m align, the key p
// path and the masks masks. An empty string leaves a key out.
func animatedTextLayer(ind, x, y int, doc, animators, align, path, masks string) string {
	keys := fmt.Sprintf(`"d": {"k": [{"t": 0, "s": %s}]}, "a": [%s]`, doc, animators)
	if align != "" {
		keys += `, "m": ` + align
	}
	if path != "" {
		keys += `, "p": ` + path
	}
	extra := ""
	if masks != "" {
		extra = `, "hasMask": true, "masksProperties": [` + masks + `]`
	}
	return fmt.Sprintf(`{
		"ty": 5, "ind": %d, "ip": 0, "op": 60, "st": 0%s,
		"ks": {"a": {"a": 0, "k": [0, 0]}, "p": {"a": 0, "k": [%d, %d]}, "s": {"a": 0, "k": [100, 100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}},
		"t": {%s}
	}`, ind, extra, x, y, keys)
}

// animated returns a property that goes from the value from at the frame 0
// to the value to at the frame 59.
func animated(from, to string) string {
	return fmt.Sprintf(`{"a": 1, "k": [{"t": 0, "s": [%s], "i": {"x": [0.5], "y": [0.5]}, "o": {"x": [0.5], "y": [0.5]}}, {"t": 59, "s": [%s]}]}`, from, to)
}

// static returns a property with the value v.
func static(v string) string {
	return fmt.Sprintf(`{"a": 0, "k": %s}`, v)
}

// textAnimator returns a text animator with the keys selector of its range
// selector and the keys style of its style.
func textAnimator(selector, style string) string {
	return fmt.Sprintf(`{"s": {%s}, "a": {%s}}`, selector, style)
}

// shapesFont is the font list with the font "Shapes" of testChars.
const shapesFont = `{"list": [{"fName": "Shapes-Regular", "fFamily": "Shapes", "fStyle": "Regular", "ascent": 72, "origin": 0}]}`

// shapesDoc returns a text document of the font "Shapes" with the extra
// keys extra.
func shapesDoc(text, extra string) string {
	return textDoc("Shapes-Regular", text, extra)
}

// TestThorVGTextAnimators compares the text animators of a text layer in a
// font with glyphs in the list chars with ThorVG (D85). In each file, the
// range of a range selector moves over the text, and the file checks a
// part of the animators:
//
//   - text-animator-move: the shape square, with the position, the
//     opacity and the fill color, and a second animator whose amount
//     changes, for the characters without the spaces, with no smoothness.
//   - text-animator-turn: the shapes triangle and ramp down, with the scale
//     and the rotation, for a group of each character with an anchor, and
//     for a group of each word.
//   - text-animator-stroke: the shapes round and smooth, with the stroke
//     width, color and opacity, the fill opacity, the ease of the range,
//     the units of glyphs, and a group of each line.
//   - text-animator-spacing: the shape ramp up, with the tracking and the
//     line spacing of the animator, for the words, in a box text that
//     breaks its lines.
func TestThorVGTextAnimators(t *testing.T) {
	cases := []compatCase{
		{name: "text-animator-move", doc: textDocument(shapesFont, testChars,
			animatedTextLayer(1, 30, 80, shapesDoc("TILT LOT", ""),
				textAnimator(`"t": 0, "xe": `+static("0")+`, "ne": `+static("0")+`, "a": `+static("100")+`, "b": 1, "rn": 0, "sh": 1, "s": `+animated("0", "100")+`, "e": `+animated("25", "100")+`, "o": `+static("0")+`, "r": 1`,
					`"p": `+static("[0, -30, 0]")+`, "o": `+static("30")+`, "fc": `+static("[0.9, 0.1, 0.1, 1]")),
				"", "", ""),
			animatedTextLayer(2, 30, 200, shapesDoc("LOT TILT IT", `, "sc": [0.1, 0.6, 0.2], "sw": 2, "of": true`),
				textAnimator(`"a": `+animated("0", "100")+`, "b": 2, "sh": 1, "s": `+static("20")+`, "e": `+static("70")+`, "sm": `+static("0"),
					`"p": `+static("[10, 15]")+`, "fc": `+static("[0.9, 0.7, 0.1]")),
				"", "", "")),
		},
		{name: "text-animator-turn", doc: textDocument(shapesFont, testChars,
			animatedTextLayer(1, 40, 90, shapesDoc("TILT LOT", ""),
				textAnimator(`"a": `+static("100")+`, "sh": 4, "s": `+animated("-40", "60")+`, "e": `+animated("10", "110"),
					`"s": `+static("[160, 120, 100]")+`, "r": `+static("40")),
				`{"g": 1, "a": `+static("[30, -35]")+`}`, "", ""),
			animatedTextLayer(2, 40, 220, shapesDoc("LIT TOIL LOT", ""),
				textAnimator(`"a": `+static("100")+`, "sh": 3, "s": `+static("0")+`, "e": `+animated("30", "100"),
					`"r": `+static("-25")+`, "p": `+static("[0, 10]")),
				`{"g": 2, "a": `+static("[0, 0]")+`}`, "", "")),
		},
		{name: "text-animator-stroke", doc: textDocument(shapesFont, testChars,
			animatedTextLayer(1, 30, 80, shapesDoc("TILT LOT\rLOT TILT", `, "sc": [0.1, 0.1, 0.1], "sw": 1`),
				textAnimator(`"a": `+static("100")+`, "sh": 5, "r": 2, "s": `+animated("0", "8")+`, "e": `+animated("6", "14"),
					`"sw": `+static("6")+`, "sc": `+static("[0.9, 0.1, 0.6]")+`, "so": `+static("60")+`, "fo": `+static("40"))+", "+
					textAnimator(`"a": `+static("80")+`, "sh": 6, "xe": `+static("60")+`, "ne": `+static("-40")+`, "s": `+static("10")+`, "e": `+animated("40", "100"),
						`"s": `+static("[100, 140]")),
				`{"g": 3, "a": `+static("[0, -20]")+`}`, "", "")),
		},
		{name: "text-animator-spacing", doc: textDocument(shapesFont, testChars,
			animatedTextLayer(1, 20, 20, shapesDoc("LIT LOT TILT OIL TOIL", `, "ps": [0, 0], "sz": [300, 260]`),
				textAnimator(`"a": `+static("100")+`, "sh": 2, "b": 3, "s": `+animated("0", "40")+`, "e": `+animated("40", "100"),
					`"t": `+animated("10", "60")+`, "ls": `+static("30")),
				"", "", "")),
		},
	}
	compareCases(t, cases)
}

// TestThorVGTextCaps compares the capitals of a text document, the key
// ca, with ThorVG (D85). With all capitals, a small letter draws the glyph
// of its capital, and with small capitals it draws it at 70 percent of the
// size, as _searchGlyph of ThorVG says. The build of ThorVG that
// github.com/xo/lottie runs draws no small letter at all when the key ca is
// not 0, because _searchGlyph keeps a pointer to a local variable after its
// scope ends. So ThorVG draws a file that gives the same image without the
// key ca: the capitals in place of the small letters, and for small
// capitals, a text of only small letters at 70 percent of the size, with 70
// percent of the stroke width.
func TestThorVGTextCaps(t *testing.T) {
	cases := []compatCase{
		{
			name: "text-caps-all",
			doc: textDocument(shapesFont, testChars,
				textLayer(1, 200, 70, shapesDoc("Tilt Lot\rTOIL it", `, "j": 2, "ca": 1, "tr": 40`)),
				textLayer(2, 20, 190, shapesDoc("Lit lot oil", `, "ca": 1, "ps": [0, 0], "sz": [200, 100]`))),
			thorvg: textDocument(shapesFont, testChars,
				textLayer(1, 200, 70, shapesDoc("TILT LOT\rTOIL IT", `, "j": 2, "tr": 40`)),
				textLayer(2, 20, 190, shapesDoc("LIT LOT OIL", `, "ps": [0, 0], "sz": [200, 100]`))),
		},
		{
			name: "text-caps-small",
			doc: textDocument(shapesFont, testChars,
				textLayer(1, 200, 70, shapesDoc("tilt\rlot", `, "j": 2, "ca": 2, "tr": 40, "sc": [0.9, 0.2, 0.1], "sw": 2`)),
				textLayer(2, 380, 220, shapesDoc("toil", `, "j": 1, "ca": 2, "s": 60`))),
			thorvg: textDocument(shapesFont, testChars,
				textLayer(1, 200, 70, shapesDoc("TILT\rLOT", `, "j": 2, "s": 25.2, "tr": 40, "sc": [0.9, 0.2, 0.1], "sw": 1.4`)),
				textLayer(2, 380, 220, shapesDoc("TOIL", `, "j": 1, "s": 42`))),
		},
	}
	compareCases(t, cases)
}

// TestThorVGTextPath compares a text on a path, the key p of a text layer,
// with ThorVG (D85). The path is the first mask of the layer, of the mode
// none, so it does not cut the layer. ThorVG puts the middle of each glyph
// on the path at the cursor plus the first margin, and does not turn the
// glyph. The first file has an open curve and a margin that moves the text
// past the end of the path, and the second a closed path and a margin that
// starts below 0.
func TestThorVGTextPath(t *testing.T) {
	open := `{"mode": "n", "inv": false, "o": {"a": 0, "k": 100}, "x": {"a": 0, "k": 0},
		"pt": {"a": 0, "k": {"c": false, "v": [[0, 100], [150, 0], [330, 60]], "i": [[0, 0], [-60, 0], [-40, -40]], "o": [[40, -80], [60, 0], [0, 0]]}}}`
	closed := `{"mode": "n", "inv": false, "o": {"a": 0, "k": 100}, "x": {"a": 0, "k": 0},
		"pt": {"a": 0, "k": {"c": true, "v": [[0, 0], [160, 0], [160, 120], [0, 120]], "i": [[0, 0], [0, 0], [0, 0], [0, 0]], "o": [[0, 0], [0, 0], [0, 0], [0, 0]]}}}`
	cases := []compatCase{
		{name: "text-path-open", doc: textDocument(shapesFont, testChars,
			animatedTextLayer(1, 30, 60, shapesDoc("TILT LOT IT", ""), "", "", `{"m": 0, "f": `+animated("0", "260")+`}`, open))},
		{name: "text-path-closed", doc: textDocument(shapesFont, testChars,
			animatedTextLayer(1, 120, 120, shapesDoc("LOT TILT OIL", `, "sc": [0.9, 0.2, 0.1], "sw": 2`), "", "", `{"m": 0, "f": `+animated("-120", "300")+`}`, closed))},
	}
	compareCases(t, cases)
}

// TestThorVGTextAnimatorsFont compares the text animators of a text that
// the sink shapes with the fonts of WithFontDatabase with ThorVG (D85).
// ThorVG takes the share of the first glyph for the whole text, and uses
// only the opacity, the fill color, and the stroke width and color of each
// animator.
func TestThorVGTextAnimatorsFont(t *testing.T) {
	ttf, err := os.ReadFile(filepath.Join("..", "..", "svg", "testdata", "fonts", "NotoSans-Regular.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	db := svg.NewFontDatabase()
	db.LoadFontData(ttf)
	noto := fmt.Sprintf(`{"list": [{"fName": "NotoSans-Regular", "fFamily": "Noto Sans", "fStyle": "Regular", "ascent": 75, "origin": 3,
		"fPath": "data:font/ttf;base64,%s"}]}`, base64.StdEncoding.EncodeToString(ttf))
	doc := textDocument(noto, "[]",
		animatedTextLayer(1, 200, 90, textDoc("NotoSans-Regular", "Animated text", `, "j": 2`),
			textAnimator(`"a": `+static("100")+`, "sh": 1, "s": `+animated("50", "0")+`, "e": `+static("100"),
				`"fc": `+static("[0.9, 0.1, 0.1]")+`, "o": `+static("50")+`, "p": `+static("[0, -40]")),
			"", "", ""),
		animatedTextLayer(2, 200, 200, textDoc("NotoSans-Regular", "With a stroke", `, "j": 2, "sc": [0.1, 0.1, 0.1], "sw": 1`),
			textAnimator(`"a": `+animated("0", "100")+`, "sh": 2, "s": `+static("0")+`, "e": `+static("100"),
				`"sw": `+static("4")+`, "sc": `+static("[0.2, 0.7, 0.2]")+`, "fc": `+static("[0.1, 0.2, 0.9]")+`, "r": `+static("30")),
			"", "", ""))
	compareCases(t, []compatCase{{name: "text-animator-font", doc: doc}}, okagelottie.WithFontDatabase(db))
}
