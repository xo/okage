// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"
)

// barChars is the list chars of the font "Bars": the glyph I, a bar from x
// 10 to 30 and y -70 to 0 with the advance 40, and a space with the advance
// 30, at the size 100.
const barChars = `[
	{"ch": "I", "size": 100, "style": "Regular", "w": 40, "fFamily": "Bars", "data": {"shapes": [{"ty": "gr", "it": [
		{"ty": "sh", "ks": {"a": 0, "k": {"c": true, "v": [[10, -70], [30, -70], [30, 0], [10, 0]], "i": [[0, 0], [0, 0], [0, 0], [0, 0]], "o": [[0, 0], [0, 0], [0, 0], [0, 0]]}}}
	]}]}},
	{"ch": " ", "size": 100, "style": "Regular", "w": 30, "fFamily": "Bars", "data": {}}
]`

// barFonts is the font list with the font "Bars".
const barFonts = `{"list": [{"fName": "Bars-Regular", "fFamily": "Bars", "fStyle": "Regular", "ascent": 70, "origin": 0}]}`

// textFile returns a Lottie file of 100 by 100 pixels with the fonts fonts,
// the glyphs chars and a text layer at (x, y) with the documents docs, which
// change at the frames 0 and 5.
func textFile(fonts, chars string, x, y int, docs ...string) []byte {
	var keyframes []string
	for i, d := range docs {
		keyframes = append(keyframes, fmt.Sprintf(`{"t": %d, "s": %s}`, i*5, d))
	}
	return fmt.Appendf(nil, `{"v": "5.7.0", "fr": 30, "ip": 0, "op": 10, "w": 100, "h": 100, "fonts": %s, "chars": %s, "layers": [
		{"ty": 5, "ind": 1, "ip": 0, "op": 10, "st": 0, "ks": {"p": {"a": 0, "k": [%d, %d]}}, "t": {"d": {"k": [%s]}}}
	]}`, fonts, chars, x, y, strings.Join(keyframes, ", "))
}

// drawn reports if the pixel (x, y) of pm is opaque, and fails when it is
// neither opaque nor transparent.
func drawn(t *testing.T, pm *okage.Pixmap, x, y uint32) bool {
	t.Helper()
	switch a := pixel(t, pm, x, y)[3]; a {
	case 0:
		return false
	case 255:
		return true
	default:
		t.Fatalf("the pixel (%d, %d) has the alpha %d", x, y, a)
	}
	return false
}

// checkDrawn checks the pixels of want, true where a glyph covers the pixel.
func checkDrawn(t *testing.T, pm *okage.Pixmap, want map[[2]uint32]bool) {
	t.Helper()
	for p, w := range want {
		if got := drawn(t, pm, p[0], p[1]); got != w {
			t.Errorf("the pixel (%d, %d) is drawn: %v, want %v", p[0], p[1], got, w)
		}
	}
}

// TestTextLayerDeserialize checks the schema of a text layer, of the font
// list and of the glyphs.
func TestTextLayerDeserialize(t *testing.T) {
	a, err := ParseAnimation(textFile(barFonts, barChars, 0, 0,
		`{"t": "I\rI", "f": "Bars-Regular", "s": 50, "j": 2, "tr": 10, "lh": 30, "ls": 2, "fc": [1, 0, 0], "sc": [0, 0, 1], "sw": 2, "of": false, "ps": [1, 2], "sz": [3, 4]}`))
	if err != nil {
		t.Fatal(err)
	}
	l := a.Layers[0].Text
	if l == nil || len(l.Text.Document.Keyframes) != 1 {
		t.Fatalf("got %+v", a.Layers[0])
	}
	d := l.Text.Document.Keyframes[0].Document
	if d.Text != "I\rI" || d.FontName != "Bars-Regular" || d.Size != 50 || d.Justify != 2 || d.Tracking != 10 ||
		d.LineHeight != 30 || d.BaselineShift != 2 || d.StrokeWidth != 2 || *d.StrokeOverFill ||
		len(d.FillColor) != 3 || len(d.BoxPosition) != 2 || d.BoxSize[1] != 4 {
		t.Errorf("got %+v", d)
	}
	if len(a.Fonts.List) != 1 || a.Fonts.List[0].Ascent != 70 || len(a.Chars) != 2 || a.Chars[0].Width != 40 {
		t.Errorf("got %+v and %+v", a.Fonts, a.Chars)
	}
	var tl AnyLayer
	if err := json.Unmarshal([]byte(`{"ty": 5, "ip": 0, "op": 1, "ks": {"p": {"k": [0, 0]}}}`), &tl); err == nil {
		t.Error("a text layer without the key t gave no error")
	}
}

// TestTextChars checks the glyphs of the list chars: the size, the
// tracking, the centered lines, the line height and the document that
// changes at the frame 5.
func TestTextChars(t *testing.T) {
	data := textFile(barFonts, barChars, 50, 40,
		// At the size 50, each bar is 10 pixels wide and 35 high, with an
		// advance of 20 and 5 more of tracking. The line of two bars is 50
		// wide, so it starts at x = 25.
		`{"t": "II\rI", "f": "Bars-Regular", "s": 50, "j": 2, "tr": 100, "lh": 45, "fc": [0, 0, 1]}`,
		`{"t": "I", "f": "Bars-Regular", "s": 50, "fc": [0, 0, 1]}`)
	pm := render(t, data, 0)
	checkDrawn(t, pm, map[[2]uint32]bool{
		{32, 20}: true, {40, 20}: false, {57, 20}: true, {65, 20}: false,
		// The second line is 25 wide, and starts at x = 37.5.
		{45, 65}: true, {35, 65}: false, {32, 65}: false,
	})
	if p := pixel(t, pm, 32, 20); p != [4]uint8{0, 0, 255, 255} {
		t.Errorf("the glyph is %v, want blue", p)
	}
	// At the frame 5, the text starts at x = 50 on the left.
	pm = render(t, data, 5)
	checkDrawn(t, pm, map[[2]uint32]bool{{57, 20}: true, {32, 20}: false, {45, 65}: false})
}

// TestTextBox checks a box text: it breaks a line at a space where the next
// word does not fit, aligns each line to the right of the box, and puts the
// first baseline at the top of the box plus the ascent.
func TestTextBox(t *testing.T) {
	// At the size 50, the word "II" is 40 wide, and the box is 70 wide, so
	// the second word goes to the next line. The ascent is 35.
	data := textFile(barFonts, barChars, 0, 0,
		`{"t": "II II", "f": "Bars-Regular", "s": 50, "j": 1, "lh": 40, "fc": [0, 0, 1], "ps": [20, 10], "sz": [70, 80]}`)
	pm := render(t, data, 0)
	// Each line is 40 wide and starts at x = 20 + 70 - 40 = 50. The first
	// line goes from y = 10 to 45, and the second from 50 to 85.
	checkDrawn(t, pm, map[[2]uint32]bool{
		{57, 30}: true, {77, 30}: true, {40, 30}: false,
		{57, 70}: true, {77, 70}: true, {57, 47}: false,
	})
}

// TestTextStroke checks the stroke of a text, over the fill and below it.
func TestTextStroke(t *testing.T) {
	for _, over := range []bool{true, false} {
		data := textFile(barFonts, barChars, 40, 60,
			fmt.Sprintf(`{"t": "I", "f": "Bars-Regular", "s": 100, "fc": [0, 0, 1], "sc": [1, 0, 0], "sw": 6, "of": %v}`, over))
		pm := render(t, data, 0)
		// The bar goes from x = 50 to 70. The stroke covers 3 pixels on each
		// side of its edge.
		if p := pixel(t, pm, 48, 30); p != [4]uint8{255, 0, 0, 255} {
			t.Errorf("over %v: the stroke outside is %v, want red", over, p)
		}
		want := [4]uint8{0, 0, 255, 255}
		if over {
			want = [4]uint8{255, 0, 0, 255}
		}
		if p := pixel(t, pm, 51, 30); p != want {
			t.Errorf("over %v: the stroke inside is %v, want %v", over, p, want)
		}
	}
}

// TestTextMissingGlyph checks that a character without a glyph draws
// nothing, takes no space, and goes to the logger.
func TestTextMissingGlyph(t *testing.T) {
	data := textFile(barFonts, barChars, 0, 50, `{"t": "XI", "f": "Bars-Regular", "s": 50, "fc": [0, 0, 1]}`)
	c, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	pm, _ := okage.NewPixmap(100, 100)
	m := pm.AsMut()
	if err := NewRenderer().Append(c, 0, IdentityAffine, 1, NewPixmapSink(&m, WithLogger(slog.New(slog.NewTextHandler(&buf, nil))))); err != nil {
		t.Fatal(err)
	}
	checkDrawn(t, pm, map[[2]uint32]bool{{7, 30}: true, {27, 30}: false})
	if !strings.Contains(buf.String(), "char=X") {
		t.Errorf("the log is %q", buf.String())
	}
}

// TestTextFont checks a text without the list chars, which the sink shapes
// with the fonts of WithFontDatabase, and which draws nothing without it.
func TestTextFont(t *testing.T) {
	ttf, err := os.ReadFile(filepath.Join("..", "svg", "testdata", "fonts", "NotoSans-Regular.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	db := svg.NewFontDatabase()
	db.LoadFontData(ttf)
	fonts := `{"list": [{"fName": "NotoSans-Regular", "fFamily": "Noto Sans", "fStyle": "Regular", "ascent": 75, "origin": 3}]}`
	data := textFile(fonts, "[]", 10, 70, `{"t": "HH", "f": "NotoSans-Regular", "s": 60, "fc": [0, 0, 1]}`)
	c, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, withFonts := range []bool{true, false} {
		var buf bytes.Buffer
		opts := []SinkOption{WithLogger(slog.New(slog.NewTextHandler(&buf, nil)))}
		if withFonts {
			opts = append(opts, WithFontDatabase(db))
		}
		pm, _ := okage.NewPixmap(100, 100)
		m := pm.AsMut()
		if err := NewRenderer().Append(c, 0, IdentityAffine, 1, NewPixmapSink(&m, opts...)); err != nil {
			t.Fatal(err)
		}
		// The two letters H stand on the baseline at y = 70, start after
		// x = 10, and are less than the size high.
		n := 0
		for y := range uint32(100) {
			for x := range uint32(100) {
				if pixel(t, pm, x, y)[3] == 0 {
					continue
				}
				n++
				if x < 10 || y < 10 || y > 71 {
					t.Fatalf("with fonts %v: the pixel (%d, %d) is drawn", withFonts, x, y)
				}
			}
		}
		if withFonts && (n < 300 || buf.Len() != 0) {
			t.Errorf("with fonts: %d pixels are drawn, and the log is %q", n, buf.String())
		}
		if !withFonts && (n != 0 || strings.Count(buf.String(), "char=H") != 2) {
			t.Errorf("without fonts: %d pixels are drawn, and the log is %q", n, buf.String())
		}
	}
	// The stems of the first H are on the baseline, at the size of 60 and
	// a width of about 0.65 em.
	pm, _ := okage.NewPixmap(100, 100)
	m := pm.AsMut()
	if err := NewRenderer().Append(c, 0, IdentityAffine, 1, NewPixmapSink(&m, WithFontDatabase(db))); err != nil {
		t.Fatal(err)
	}
	if p := pixel(t, pm, 20, 60); p != [4]uint8{0, 0, 255, 255} {
		t.Errorf("the left stem is %v, want blue", p)
	}
}

// TestFontWeight checks the weight of the name of a style.
func TestFontWeight(t *testing.T) {
	for style, want := range map[string]uint16{
		"Regular": 400, "Bold Italic": 700, "SemiBold": 600, "Extra-Light": 200, "Black": 900, "Light Italic": 300,
	} {
		if got := fontWeight(style); got != want {
			t.Errorf("%q has the weight %d, want %d", style, got, want)
		}
	}
}

// fixedShaper is a textSink that gives each character a cluster of two
// glyphs with the advances 30 and 20, and the ascent 40.
type fixedShaper struct{}

func (fixedShaper) shapeText(_, _, _ string, text []rune, _ float64) ([]shapedCluster, float64, bool) {
	var clusters []shapedCluster
	for _, c := range text {
		clusters = append(clusters, shapedCluster{text: string(c), advance: 50, glyphs: []shapedGlyph{
			{scale: 1, dx: 2, pen: 0},
			{scale: 1, dx: 33, pen: 30},
		}})
	}
	return clusters, 40, true
}

// TestShapedTracking checks the tracking of a text that the sink shapes
// (D82). As ThorVG does, it scales each advance by 1 plus the tracking over
// the ascent, and keeps the factor 1 when that factor is negative. At the
// size 100, the tracking 200 is 20 pixels, and so the factor is 1.5.
func TestShapedTracking(t *testing.T) {
	for _, c := range []struct {
		tracking, advance, dx float64
	}{
		{0, 50, 33},
		{200, 75, 48},
		{-100, 37.5, 25.5},
		{-500, 50, 33},
	} {
		doc := &TextDocument{Font: "Shaped", Size: 100, Tracking: c.tracking}
		units, ascent := textUnits("ab", nil, doc, 1, c.tracking*0.1, fixedShaper{})
		if len(units) != 2 || ascent != 40 {
			t.Fatalf("tracking %v: %d units and the ascent %v", c.tracking, len(units), ascent)
		}
		for _, u := range units {
			if u.advance != c.advance || u.shaped[0].dx != 2 || u.shaped[1].dx != c.dx {
				t.Errorf("tracking %v: the advance %v and the offsets %v and %v, want %v, 2 and %v",
					c.tracking, u.advance, u.shaped[0].dx, u.shaped[1].dx, c.advance, c.dx)
			}
		}
	}
}
