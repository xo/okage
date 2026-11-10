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
	appendGroupPaths(pb, text.Flattened(), path.IdentityTransform())
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
