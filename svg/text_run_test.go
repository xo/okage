package svg

import (
	"errors"
	"fmt"
	"html"
	"slices"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// This file tests the text API of text_run.go (D37).

// findText returns the first Text node of g and of its groups.
func findText(g *Group) *Text {
	for _, child := range g.Children() {
		switch c := child.(type) {
		case *Text:
			return c
		case *Group:
			if t := findText(c); t != nil {
				return t
			}
		}
	}
	return nil
}

// textOutline returns the outline of the flattened group of text as one
// path, or nil.
func textOutline(text *Text) *path.Path {
	pb := path.NewPathBuilder()
	(&TextRun{text: text}).AppendPath(pb, 0, 0)
	p, _ := pb.Finish()
	return p
}

// TestShapeTextMatchesParser checks that ShapeText gives the box and the
// outlines of the <text> element with the same attributes, in the same
// fonts, with fallback, kerning, weights and styles.
func TestShapeTextMatchesParser(t *testing.T) {
	db := writeFontDB()
	for _, c := range []struct {
		text  string
		style TextStyle
		attrs string
	}{
		{"Hello, world", TextStyle{Family: "Noto Sans", Size: 16}, `font-family="Noto Sans" font-size="16"`},
		{"AVA Wo", TextStyle{Family: "sans-serif", Size: 11}, `font-family="sans-serif" font-size="11"`},
		{"Bold 123", TextStyle{Family: "Noto Sans", Size: 20, Weight: 700}, `font-family="Noto Sans" font-size="20" font-weight="700"`},
		{"Italic", TextStyle{Family: "Noto Sans", Size: 12, Style: FontStyleItalic}, `font-family="Noto Sans" font-size="12" font-style="italic"`},
		{"Serif", TextStyle{Family: "No Such Font, serif", Size: 14}, `font-family="No Such Font, serif" font-size="14"`},
		{"Default", TextStyle{Size: 10}, `font-size="10"`},
		{"Latin \u0939\u093f\u0928\u094d\u0926\u0940 \u65e5\u672c\u8a9e", TextStyle{Family: "Noto Sans", Size: 18}, `font-family="Noto Sans" font-size="18"`},
		{"<&>", TextStyle{Family: "monospace", Size: 9.5}, `font-family="monospace" font-size="9.5"`},
		{"Thin", TextStyle{Family: "Noto Sans", Size: 30, Weight: 100}, `font-family="Noto Sans" font-size="30" font-weight="100"`},
	} {
		t.Run(c.text, func(t *testing.T) {
			run, err := ShapeText(c.text, c.style, WithFontDatabase(db))
			if err != nil {
				t.Fatal(err)
			}
			data := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="400" height="100"><text x="0" y="0" %s>%s</text></svg>`, c.attrs, html.EscapeString(c.text))
			tree, err := FromStr(data, WithFontDatabase(db))
			if err != nil {
				t.Fatal(err)
			}
			want := findText(tree.Root())
			if want == nil {
				t.Fatal("the parser made no Text node")
			}
			if run.Bounds() != want.BoundingBox() {
				t.Errorf("Bounds() = %v, want %v", run.Bounds(), want.BoundingBox())
			}
			if run.Width() != want.BoundingBox().Width() || run.Width() <= 0 {
				t.Errorf("Width() = %g, want %g", run.Width(), want.BoundingBox().Width())
			}
			got, wantPath := textOutline(run.Text()), textOutline(want)
			if got == nil || wantPath == nil {
				t.Fatal("no outline")
			}
			if !slices.Equal(got.Verbs(), wantPath.Verbs()) || !slices.Equal(got.Points(), wantPath.Points()) {
				t.Error("the outline differs from the outline of the parser")
			}
		})
	}
}

// TestShapeTextAppendPath checks that AppendPath moves the outline to the
// origin that it is given.
func TestShapeTextAppendPath(t *testing.T) {
	run, err := ShapeText("Wx", TextStyle{Family: "Noto Sans", Size: 20}, WithFontDatabase(writeFontDB()))
	if err != nil {
		t.Fatal(err)
	}
	a, b := path.NewPathBuilder(), path.NewPathBuilder()
	run.AppendPath(a, 0, 0)
	run.AppendPath(b, 100, 50)
	pa, _ := a.Finish()
	pb, _ := b.Finish()
	if pa == nil || pb == nil {
		t.Fatal("no outline")
	}
	ba, bb := pa.Bounds(), pb.Bounds()
	if bb.Left()-ba.Left() != 100 || bb.Top()-ba.Top() != 50 {
		t.Errorf("the bounds %v moved to %v, want a move of (100, 50)", ba, bb)
	}
	// The glyphs sit on the baseline: most of them is above y = 0.
	if ba.Top() >= 0 || ba.Bottom() > 1 {
		t.Errorf("the bounds %v are not on the baseline at y = 0", ba)
	}
}

// TestShapeTextErrors checks the errors of ShapeText and the empty run.
func TestShapeTextErrors(t *testing.T) {
	db := writeFontDB()
	if _, err := ShapeText("x", TextStyle{Size: 0}, WithFontDatabase(db)); !errors.Is(err, okage.ErrSVGTextInvalidSize) {
		t.Errorf("a size of 0 gives %v, want ErrSVGTextInvalidSize", err)
	}
	if _, err := ShapeText("x", TextStyle{Size: 10}); !errors.Is(err, okage.ErrSVGTextNoFont) {
		t.Errorf("an empty database gives %v, want ErrSVGTextNoFont", err)
	}
	run, err := ShapeText("", TextStyle{Size: 10}, WithFontDatabase(db))
	if err != nil || run.Width() != 0 || run.Text() != nil {
		t.Errorf("an empty text gives %v, %v, want an empty run", run, err)
	}
	pb := path.NewPathBuilder()
	run.AppendPath(pb, 1, 2)
	if !pb.IsEmpty() {
		t.Error("an empty run appended a path")
	}
}

// TestRendererShapeText checks that a renderer shapes with its fonts and its
// generic families.
func TestRendererShapeText(t *testing.T) {
	r := New(testFonts()...)
	got, err := r.ShapeText("Label", TextStyle{Family: "serif", Size: 12})
	if err != nil {
		t.Fatal(err)
	}
	want, err := ShapeText("Label", TextStyle{Family: "Noto Serif", Size: 12}, WithFontDatabase(writeFontDB()))
	if err != nil {
		t.Fatal(err)
	}
	if got.Width() != want.Width() || !slices.Equal(textOutline(got.Text()).Points(), textOutline(want.Text()).Points()) {
		t.Error("the serif of the renderer is not Noto Serif")
	}
}

// TestShapeTextClusters checks the clusters of a run: their bytes in the
// order of the text, their positions and advances, and their glyphs (D57).
func TestShapeTextClusters(t *testing.T) {
	db, ids := testFontDB(t, "RobotoFlex.subset.ttf")
	// Roboto Flex draws "fi" as one ligature, so "fib" has two clusters.
	run, err := ShapeText("fib", TextStyle{Family: "Roboto Flex", Size: 100}, WithFontDatabase(db))
	if err != nil {
		t.Fatal(err)
	}
	clusters := run.Clusters()
	if len(clusters) != 2 {
		t.Fatalf("got %d clusters, want 2", len(clusters))
	}
	var x float32
	for i, c := range clusters {
		want := [2]int{0, 2}
		if i == 1 {
			want = [2]int{2, 3}
		}
		if c.Start != want[0] || c.End != want[1] {
			t.Errorf("cluster %d is the bytes %d to %d, want %v", i, c.Start, c.End, want)
		}
		if c.X != x {
			t.Errorf("cluster %d at %v, want %v", i, c.X, x)
		}
		if c.Font != ids[0] || len(c.Glyphs) != 1 || c.Advance <= 0 {
			t.Errorf("cluster %d = %+v, want one glyph of the font", i, c)
		}
		if ts := c.Glyphs[0].Transform(); ts.TX != c.X {
			t.Errorf("cluster %d: the glyph is at %v, want %v", i, ts.TX, c.X)
		}
		x += c.Advance
	}
	if abs32(x-run.Width()) > 1e-3 {
		t.Errorf("the advances add up to %v, want the width %v", x, run.Width())
	}
	if (&TextRun{}).Clusters() != nil {
		t.Error("an empty run has clusters")
	}
}

// TestShapeTextClustersRTL checks that the clusters of a run from right to
// left are in the order of the text, with the first one at the right.
func TestShapeTextClustersRTL(t *testing.T) {
	db, _ := testFontDB(t, "Amiri-Regular.ttf")
	text := "אב" // Hebrew alef and bet, which Amiri draws.
	run, err := ShapeText(text, TextStyle{Family: "Amiri", Size: 100}, WithFontDatabase(db))
	if err != nil {
		t.Fatal(err)
	}
	clusters := run.Clusters()
	if len(clusters) != 2 {
		t.Fatalf("got %d clusters, want 2", len(clusters))
	}
	if clusters[0].Start != 0 || clusters[0].End != 2 || clusters[1].Start != 2 || clusters[1].End != len(text) {
		t.Errorf("clusters = %+v, want the bytes of each letter", clusters)
	}
	if clusters[0].X <= clusters[1].X {
		t.Errorf("the first letter is at %v and the second at %v, want the first at the right", clusters[0].X, clusters[1].X)
	}
}

// TestShapeTextSpacing checks the letter spacing and the word spacing of
// TextStyle.
func TestShapeTextSpacing(t *testing.T) {
	db, _ := testFontDB(t, "NotoSans-Regular.ttf")
	width := func(text string, style TextStyle) float32 {
		t.Helper()
		style.Family, style.Size = "Noto Sans", 100
		run, err := ShapeText(text, style, WithFontDatabase(db))
		if err != nil {
			t.Fatal(err)
		}
		return run.Width()
	}
	base := width("ab cd", TextStyle{})
	// The letter spacing goes after each cluster but the last.
	if got, want := width("ab cd", TextStyle{LetterSpacing: 2}), base+4*2; abs32(got-want) > 1e-3 {
		t.Errorf("letter spacing: width %v, want %v", got, want)
	}
	if got, want := width("ab cd", TextStyle{WordSpacing: 7}), base+7; abs32(got-want) > 1e-3 {
		t.Errorf("word spacing: width %v, want %v", got, want)
	}
}

// TestShapeTextFeaturesAndLanguage checks that the features and the language
// of TextStyle reach the shaper.
func TestShapeTextFeaturesAndLanguage(t *testing.T) {
	db, _ := testFontDB(t, "NotoSerif-Regular.ttf", "NotoSans-Regular.ttf")
	glyphs := func(text string, style TextStyle) []GlyphID {
		t.Helper()
		style.Size = 100
		run, err := ShapeText(text, style, WithFontDatabase(db))
		if err != nil {
			t.Fatal(err)
		}
		var ids []GlyphID
		for _, c := range run.Clusters() {
			for _, g := range c.Glyphs {
				ids = append(ids, g.ID)
			}
		}
		return ids
	}
	// Noto Serif draws "fi" as a ligature, unless liga is off.
	if got := glyphs("fi", TextStyle{Family: "Noto Serif"}); len(got) != 1 {
		t.Errorf("fi: %v, want the ligature", got)
	}
	noLiga := []FontFeature{{Tag: [4]byte{'l', 'i', 'g', 'a'}, Value: 0}}
	if got := glyphs("fi", TextStyle{Family: "Noto Serif", Features: noLiga}); len(got) != 2 {
		t.Errorf("fi without liga: %v, want two glyphs", got)
	}
	// Noto Sans draws the Serbian form of б for the language sr.
	ru, sr := glyphs("б", TextStyle{Family: "Noto Sans"}), glyphs("б", TextStyle{Family: "Noto Sans", Language: "sr"})
	if len(ru) != 1 || len(sr) != 1 || ru[0] == sr[0] {
		t.Errorf("б: %v without a language and %v for sr, want two glyphs", ru, sr)
	}
}

// TestShapeTextStretchAndVariations checks that the weight, the stretch and
// the variations of TextStyle set the axes of a variable font, as
// AutoFontVariations maps them.
func TestShapeTextStretchAndVariations(t *testing.T) {
	db, _ := testFontDB(t, "RobotoFlex.subset.ttf")
	width := func(style TextStyle) float32 {
		t.Helper()
		style.Family, style.Size = "Roboto Flex", 100
		run, err := ShapeText("ab", style, WithFontDatabase(db))
		if err != nil {
			t.Fatal(err)
		}
		return run.Width()
	}
	wdth := [4]byte{'w', 'd', 't', 'h'}
	normal := width(TextStyle{})
	if width(TextStyle{Weight: 700}) <= normal {
		t.Error("the weight 700 is not wider")
	}
	condensed := width(TextStyle{Stretch: 75})
	if condensed >= normal {
		t.Error("the stretch 75 is not narrower")
	}
	if got := width(TextStyle{Variations: []FontVariation{NewFontVariation(wdth, 75)}}); got != condensed {
		t.Errorf("wdth 75: width %v, want %v as the stretch 75", got, condensed)
	}
	// The nearest keyword of 80 is condensed, 75.
	if got := width(TextStyle{Stretch: 80}); got != condensed {
		t.Errorf("stretch 80: width %v, want %v as condensed", got, condensed)
	}
	// Variations win over the stretch.
	if got := width(TextStyle{Stretch: 75, Variations: []FontVariation{NewFontVariation(wdth, 100)}}); got != normal {
		t.Errorf("stretch 75 with wdth 100: width %v, want %v", got, normal)
	}
}

// TestAutoFontVariations checks the axes of AutoFontVariations, and that it
// does not change its argument.
func TestAutoFontVariations(t *testing.T) {
	wght, wdth := [4]byte{'w', 'g', 'h', 't'}, [4]byte{'w', 'd', 't', 'h'}
	ital, slnt := [4]byte{'i', 't', 'a', 'l'}, [4]byte{'s', 'l', 'n', 't'}
	for _, c := range []struct {
		in      []FontVariation
		weight  uint16
		stretch FontStretch
		style   FontStyle
		want    []FontVariation
	}{
		{nil, 400, FontStretchNormal, FontStyleNormal, []FontVariation{{wght, 400}}},
		{nil, 700, FontStretchCondensed, FontStyleItalic, []FontVariation{{wght, 700}, {wdth, 75}, {ital, 1}}},
		{nil, 300, FontStretchUltraExpanded, FontStyleOblique, []FontVariation{{wght, 300}, {wdth, 200}, {slnt, -12}}},
		{[]FontVariation{{wght, 550}, {slnt, -5}}, 700, FontStretchNormal, FontStyleOblique, []FontVariation{{wght, 550}, {slnt, -5}}},
	} {
		in := make([]FontVariation, len(c.in), len(c.in)+4)
		copy(in, c.in)
		got := AutoFontVariations(in, c.weight, c.stretch, c.style)
		if !slices.Equal(got, c.want) {
			t.Errorf("AutoFontVariations(%v, %d, %v, %v) = %v, want %v", c.in, c.weight, c.stretch, c.style, got, c.want)
		}
		// The spare capacity of in stays empty, so the result does not
		// share it.
		if !slices.Equal(in, c.in) || in[:len(in)+1][len(in)] != (FontVariation{}) {
			t.Errorf("AutoFontVariations changed its argument")
		}
	}
}
