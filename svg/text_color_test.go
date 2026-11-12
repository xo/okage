package svg

import (
	"os"
	"slices"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// colorFontOptions returns the options of a database with Noto Sans and the
// color font file, with Noto Sans as the default family.
func colorFontOptions(t *testing.T, file string) []Option {
	t.Helper()
	opts := []Option{WithFontFamily("Noto Sans")}
	for _, name := range []string{"NotoSans-Regular.ttf", file} {
		data, err := os.ReadFile("testdata/fonts/" + name)
		if err != nil {
			t.Fatal(err)
		}
		opts = append(opts, WithFontFile(data))
	}
	return opts
}

// glyphIDs returns the text and the glyph ID of each glyph of run.
func glyphIDs(run *TextRun) map[string]GlyphID {
	ids := map[string]GlyphID{}
	for _, span := range run.Text().Layouted() {
		for _, g := range span.PositionedGlyphs {
			ids[g.Text] = g.ID
		}
	}
	return ids
}

// TestShapeTextAppendPathKeepsRun checks that AppendPath does not move the
// outlines of the run, so that a second call gives the same path.
func TestShapeTextAppendPathKeepsRun(t *testing.T) {
	run, err := ShapeText("Wx", TextStyle{Family: "Noto Sans", Size: 20}, WithFontDatabase(writeFontDB()))
	if err != nil {
		t.Fatal(err)
	}
	var got [2][]path.Point
	for i := range got {
		pb := path.NewPathBuilder()
		run.AppendPath(pb, 100, 50)
		p, _ := pb.Finish()
		if p == nil {
			t.Fatal("no outline")
		}
		got[i] = p.Points()
	}
	if !slices.Equal(got[0], got[1]) {
		t.Error("a second AppendPath gave other points")
	}
}

// TestShapeTextColorGlyphs checks that RenderColorGlyphs draws the color
// glyphs of COLR and CBDT fonts, after a fallback from Noto Sans, as the
// renderer draws the same <text> element, and that AppendPath leaves them
// out.
func TestShapeTextColorGlyphs(t *testing.T) {
	for _, c := range []struct {
		file, emoji string
	}{
		{"NotoColorEmojiCOLR.subset.ttf", "\U0001F436"},
		{"NotoColorEmojiCBDT.subset.ttf", "\U0001F600"},
	} {
		t.Run(c.file, func(t *testing.T) {
			opts := colorFontOptions(t, c.file)
			run, err := ShapeText(c.emoji, TextStyle{Size: 20}, opts...)
			if err != nil {
				t.Fatal(err)
			}
			if !run.HasColorGlyphs() {
				t.Fatal("HasColorGlyphs is false")
			}
			if glyphIDs(run)[c.emoji] == 0 {
				t.Fatal("no fallback to the color font")
			}
			pb := path.NewPathBuilder()
			run.AppendPath(pb, 10, 30)
			if !pb.IsEmpty() {
				t.Error("AppendPath appended the color glyph")
			}

			got, ok := okage.NewPixmap(40, 40)
			if !ok {
				t.Fatal("no pixmap")
			}
			m := got.AsMut()
			run.RenderColorGlyphs(&m, path.TransformFromTranslate(10, 30))

			tree, err := FromStr(`<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40">`+
				`<text x="10" y="30" font-size="20">`+c.emoji+`</text></svg>`, opts...)
			if err != nil {
				t.Fatal(err)
			}
			want, ok := okage.NewPixmap(40, 40)
			if !ok {
				t.Fatal("no pixmap")
			}
			wm := want.AsMut()
			Render(tree, path.IdentityTransform(), &wm, opts...)

			if !slices.ContainsFunc(want.Data(), func(b byte) bool { return b != 0 }) {
				t.Fatal("the renderer drew nothing")
			}
			if !slices.Equal(got.Data(), want.Data()) {
				t.Error("RenderColorGlyphs differs from the renderer")
			}
		})
	}
}

// TestShapeTextColorGlyphsNone checks a run without color glyphs.
func TestShapeTextColorGlyphsNone(t *testing.T) {
	run, err := ShapeText("Wx", TextStyle{Family: "Noto Sans", Size: 20}, WithFontDatabase(writeFontDB()))
	if err != nil {
		t.Fatal(err)
	}
	if run.HasColorGlyphs() {
		t.Error("HasColorGlyphs is true")
	}
	pm, ok := okage.NewPixmap(8, 8)
	if !ok {
		t.Fatal("no pixmap")
	}
	m := pm.AsMut()
	run.RenderColorGlyphs(&m, path.IdentityTransform())
	(&TextRun{}).RenderColorGlyphs(&m, path.IdentityTransform())
	if slices.ContainsFunc(pm.Data(), func(b byte) bool { return b != 0 }) {
		t.Error("RenderColorGlyphs drew a glyph without color")
	}
}

// TestShapeTextFallbackPastMissing checks that a character that no font draws
// does not stop the fallback of the characters after it (D39).
func TestShapeTextFallbackPastMissing(t *testing.T) {
	// The COLR font has the dog and not the grinning face, and Noto Sans has
	// neither.
	const missing, dog = "\U0001F600", "\U0001F436"
	opts := colorFontOptions(t, "NotoColorEmojiCOLR.subset.ttf")
	run, err := ShapeText("A"+missing+dog+"B", TextStyle{Size: 20}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	ids := glyphIDs(run)
	if ids[missing] != 0 {
		t.Errorf("the missing character has the glyph %d, want 0", ids[missing])
	}
	if ids[dog] == 0 || ids["A"] == 0 || ids["B"] == 0 {
		t.Errorf("the glyphs are %v, want a glyph for each character but %s", ids, missing)
	}
}

// TestShapeTextFallbackSequence checks that an emoji sequence, which is
// several glyphs in Noto Sans and one glyph in the color font, falls back
// with the text around it, whichever font comes first (D39).
func TestShapeTextFallbackSequence(t *testing.T) {
	// The rainbow flag is a ZWJ sequence of four characters.
	const flag = "\U0001F3F3\uFE0F\u200D\U0001F308"
	opts := colorFontOptions(t, "TwitterColorEmoji.subset.ttf")
	for _, family := range []string{"Noto Sans", "Twitter Color Emoji"} {
		t.Run(family, func(t *testing.T) {
			run, err := ShapeText("a "+flag+" \U0001F600", TextStyle{Family: family, Size: 20}, opts...)
			if err != nil {
				t.Fatal(err)
			}
			ids := glyphIDs(run)
			for _, s := range []string{"a", flag, "\U0001F600"} {
				if ids[s] == 0 {
					t.Errorf("%q has no glyph in %v", s, ids)
				}
			}
			if !run.HasColorGlyphs() {
				t.Error("HasColorGlyphs is false")
			}
		})
	}
}

// TestShapeTextPresentationNoEmojiFont checks that a text glyph with U+FE0F,
// which no emoji font draws, stays in the text font, and that the run does
// not move to another font (D40).
func TestShapeTextPresentationNoEmojiFont(t *testing.T) {
	const copyright = "\u00A9\uFE0F"
	opts := colorFontOptions(t, "NotoColorEmojiCOLR.subset.ttf")
	run, err := ShapeText("a"+copyright+"b", TextStyle{Size: 20}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	ids := glyphIDs(run)
	for _, s := range []string{"a", "b"} {
		if ids[s] == 0 {
			t.Errorf("%q has no glyph in %v", s, ids)
		}
	}
	if run.HasColorGlyphs() {
		t.Error("HasColorGlyphs is true")
	}
}
