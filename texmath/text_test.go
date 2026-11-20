package texmath

import (
	"os"
	"runtime"
	"testing"

	"golang.org/x/image/math/fixed"

	"github.com/xo/okage"
	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/language"
	"github.com/xo/okage/typesetting/shaping"
)

// This file tests the text face and the color commands of phase 26 of D78.

// textFace returns a face of a text font of svg/testdata/fonts, which okage
// holds with its license. On wasip1, the runtime can give the folder texmath
// as the root of the files, so that ../svg is out of reach. The font is then
// at /svg/testdata/fonts when the runtime mounts the folder svg at /svg, and
// the test skips when it does not.
func textFace(tb testing.TB, name string) *font.Face {
	tb.Helper()
	f, err := os.Open("../svg/testdata/fonts/" + name)
	if err != nil && runtime.GOOS == "wasip1" {
		f, err = os.Open("/svg/testdata/fonts/" + name)
		if err != nil {
			tb.Skipf("the font %s is not there: %v", name, err)
		}
	}
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	face, err := font.ParseTTF(f)
	if err != nil {
		tb.Fatal(err)
	}
	return face
}

// firstGlyphs returns the glyphs of the glyph items of b.
func firstGlyphs(b *Box) []font.GID {
	var out []font.GID
	for _, it := range b.Items {
		if it.Kind == ItemGlyph {
			out = append(out, it.Glyph)
		}
	}
	return out
}

// TestTextWithoutAFace checks that a text command without a text face takes
// the math alphabet of its style from the math face.
func TestTextWithoutAFace(t *testing.T) {
	face := testFace(t)
	for _, c := range []struct {
		tex  string
		want rune
	}{
		{`\text{A}`, 'A'},
		{`\textrm{A}`, 'A'},
		{`\textbf{A}`, 0x1D400},
		{`\textit{A}`, 0x1D434},
		{`\textbf{\textit{A}}`, 0x1D468},
		{`\textsf{A}`, 0x1D5A0},
		{`\textsf{\textbf{H}}`, 0x1D5DB},
		{`\textsf{\textit{A}}`, 0x1D608},
		{`\textsf{\textbf{\textit{A}}}`, 0x1D63C},
		{`\texttt{A}`, 0x1D670},
		{`\texttt{\textbf{A}}`, 0x1D670},
		{`\emph{A}`, 0x1D434},
		{`\textit{\emph{A}}`, 'A'},
		{`\textbf{\textmd{A}}`, 'A'},
		{`\textit{\textup{A}}`, 'A'},
		{`\textbf{\textnormal{A}}`, 'A'},
		{`\textbf{\text{A}}`, 0x1D400},
		{`\textsf{\textrm{A}}`, 'A'},
		{`\mathbf{\text{A}}`, 'A'},
	} {
		b := mustLayout(t, c.tex, 10, Text)
		if got := firstGlyphs(b); len(got) != 1 || got[0] != glyphOf(t, face, c.want) {
			t.Errorf("%s: the glyphs %v, want the glyph of %U", c.tex, got, c.want)
		}
	}
	// The spaces of \textbf stay.
	b := mustLayout(t, `\textbf{a b}`, 10, Text)
	if n := len(firstGlyphs(b)); n != 3 {
		t.Errorf(`\textbf{a b} has %d glyphs, want 3`, n)
	}
}

// TestTextFace checks that a TextRun is shaped with the face of its style,
// and that a style without a face takes the math face.
func TestTextFace(t *testing.T) {
	face := testFace(t)
	regular := textFace(t, "NotoSans-Regular.ttf")
	bold := textFace(t, "NotoSans-Bold.ttf")
	italic := textFace(t, "NotoSans-Italic.ttf")
	mono := textFace(t, "NotoMono-Regular.ttf")
	opts := []Option{WithTextFace(regular, bold, italic, nil), WithTextFamily(FamilyMono, mono, nil, nil, nil)}
	for _, c := range []struct {
		tex  string
		face *font.Face
	}{
		{`\text{AV}`, regular},
		{`\textbf{AV}`, bold},
		{`\textit{AV}`, italic},
		{`\texttt{AV}`, mono},
		{`\textbf{\textit{A}}`, face},
		{`\textsf{A}`, face},
	} {
		b, err := LayoutTeX(c.tex, face, 10, Text, opts...)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range b.Items {
			if it.Face != c.face {
				t.Errorf("%s: an item of the wrong face", c.tex)
			}
		}
	}
	// The items are the glyphs of the shaper, with its kerning and its
	// ligatures, at the size of the layout.
	for _, s := range []string{"AV", "To fit, office", "Ab0 xy"} {
		b, err := LayoutTeX(`\text{`+s+`}`, face, 12, Text, opts...)
		if err != nil {
			t.Fatal(err)
		}
		text := []rune(s)
		out := (&shaping.HarfbuzzShaper{}).Shape(shaping.Input{Text: text, RunEnd: len(text), Direction: di.DirectionLTR,
			Face: regular, Size: fixed.Int26_6(int32(regular.Upem()) * 64), Script: language.Latin})
		if len(out.Glyphs) != len(b.Items) {
			t.Fatalf("%q: %d items, want the %d glyphs of the shaper", s, len(b.Items), len(out.Glyphs))
		}
		k := 12 / float64(regular.Upem())
		pen := 0.0
		for i, g := range out.Glyphs {
			if it := b.Items[i]; it.Glyph != g.GlyphID || !near(it.X, pen+float64(float64(g.XOffset)/64*k)) || it.Size != 12 {
				t.Errorf("%q: item %d is %+v, want the glyph %d at %v", s, i, it, g.GlyphID, pen)
			}
			pen += float64(float64(g.Advance) / 64 * k)
		}
		if !near(b.Width, pen) {
			t.Errorf("%q: the width %v, want %v", s, b.Width, pen)
		}
	}
	// NotoSans kerns AV, so the run is narrower than its two advances.
	b, err := LayoutTeX(`\text{AV}`, face, 10, Text, opts...)
	if err != nil {
		t.Fatal(err)
	}
	k := 10 / float64(regular.Upem())
	sum := float64(float64(regular.HorizontalAdvance(glyphOf(t, regular, 'A'))+regular.HorizontalAdvance(glyphOf(t, regular, 'V'))) * k)
	if !(b.Width < sum) {
		t.Errorf(`\text{AV} is %v wide, want less than %v`, b.Width, sum)
	}
	if b.Items[0].Text != "A" || b.Items[1].Text != "V" {
		t.Errorf("the texts of the items are %q and %q", b.Items[0].Text, b.Items[1].Text)
	}
	// A character that the face has no glyph for takes the math face.
	b, err = LayoutTeX(`\text{a∀}`, face, 10, Text, WithTextFace(textFace(t, "NotoSansDevanagari-Regular.ttf"), nil, nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Items) != 2 || b.Items[1].Face != face || b.Items[1].Glyph != glyphOf(t, face, '∀') {
		t.Errorf(`\text{a∀} gives %+v, want ∀ from the math face`, b.Items)
	}
}

// TestColor checks \color and \textcolor.
func TestColor(t *testing.T) {
	face := testFace(t)
	red := okage.ColorFromRGBA8(255, 0, 0, 255)
	green := okage.ColorFromRGBA8(0x22, 0x8b, 0x22, 255)
	b := mustLayout(t, `a {\color{red} b c} d`, 10, Text)
	want := []okage.Color{okage.ColorBlack, red, red, okage.ColorBlack}
	for i, it := range b.Items {
		if it.Color != want[i] {
			t.Errorf(`item %d of \color has the color %v, want %v`, i, it.Color, want[i])
		}
	}
	for _, tex := range []string{`\textcolor{#228B22}{x}`, `\textcolor{#228b22ff}{x}`, `\textcolor{228B22}{x}`, `\color{ForestGreen}x`} {
		if b := mustLayout(t, tex, 10, Text); b.Items[0].Color != green {
			t.Errorf("%s has the color %v, want %v", tex, b.Items[0].Color, green)
		}
	}
	if b := mustLayout(t, `\textcolor{#f00}{x}`, 10, Text); b.Items[0].Color != red {
		t.Errorf(`\textcolor{#f00} has the color %v`, b.Items[0].Color)
	}
	// The items of a color join the list for the space between atoms.
	for _, c := range [][2]string{
		{`a+b`, `a\textcolor{red}{+}b`},
		{`a+b`, `a\color{red}+b`},
		{`a+b=c`, `a\color{red}+b=c`},
		{`{a+b}`, `{a\color{red}+b}`},
	} {
		if w, got := mustLayout(t, c[0], 10, Text).Width, mustLayout(t, c[1], 10, Text).Width; !near(w, got) {
			t.Errorf("%s is %v wide, want %v as %s", c[1], got, w, c[0])
		}
	}
	for _, tex := range []string{`\color{nosuch}x`, `\textcolor{#12}{x}`, `\color`, `\textcolor{red}`} {
		if _, err := Parse(tex); err == nil {
			t.Errorf("%s parses", tex)
		}
	}
	_ = face
}

// TestColorbox checks \colorbox, \fcolorbox and \fbox.
func TestColorbox(t *testing.T) {
	aqua := okage.ColorFromRGBA8(0, 255, 255, 255)
	red := okage.ColorFromRGBA8(255, 0, 0, 255)
	b := mustLayout(t, `\colorbox{aqua}{A b}`, 10, Text)
	rules := itemsOf(b, ItemRule)
	if len(rules) != 1 || rules[0].Color != aqua || b.Items[0].Kind != ItemRule {
		t.Errorf(`\colorbox gives the rules %+v, want one aqua rule behind the text`, rules)
	}
	if len(itemsOf(b, ItemGlyph)) != 3 {
		t.Errorf(`\colorbox{aqua}{A b}: the text is not text`)
	}
	if r := rules[0]; !near(r.W, b.Width) || !near(r.H, b.Height+b.Depth) {
		t.Errorf(`\colorbox: the background %v by %v, want the box %v by %v`, r.W, r.H, b.Width, b.Height+b.Depth)
	}
	b = mustLayout(t, `\fcolorbox{red}{aqua}{A}`, 10, Text)
	rules = itemsOf(b, ItemRule)
	if len(rules) != 5 || rules[0].Color != aqua {
		t.Fatalf(`\fcolorbox gives %d rules`, len(rules))
	}
	for _, r := range rules[1:] {
		if r.Color != red {
			t.Errorf(`a rule of the frame of \fcolorbox has the color %v`, r.Color)
		}
	}
	if b.Items[1].Color != okage.ColorBlack {
		t.Errorf(`the text of \fcolorbox has the color %v`, b.Items[1].Color)
	}
	b = mustLayout(t, `\fbox{A}`, 10, Text)
	if n := len(itemsOf(b, ItemRule)); n != 4 {
		t.Errorf(`\fbox has %d rules, want 4`, n)
	}
	if w, got := mustLayout(t, `\boxed{\text{A}}`, 10, Text).Width, b.Width; !near(w, got) {
		t.Errorf(`\fbox{A} is %v wide, and \boxed{\text{A}} %v`, got, w)
	}
}
