// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// The tests of this file port the tests of krilla tests/src that draw on a
// surface with solid paints: path.rs, stream.rs, mask.rs and graphic.rs.
// content.rs and surface.rs of upstream have no tests of their own.

// rectToPath is rect_to_path of krilla tests/src/main.rs.
func rectToPath(t *testing.T, x1, y1, x2, y2 float32) *Path {
	t.Helper()
	b := path.NewPathBuilder()
	b.MoveTo(x1, y1)
	b.LineTo(x2, y1)
	b.LineTo(x2, y2)
	b.LineTo(x1, y2)
	b.Close()
	p, ok := b.Finish()
	if !ok {
		t.Fatal("the path is empty")
	}
	return p
}

// opacity returns a NormalizedF32, as NormalizedF32::new(opacity).unwrap().
func opacity(t *testing.T, v float32) NormalizedF32 {
	t.Helper()
	n, ok := NewNormalizedF32(v)
	if !ok {
		t.Fatalf("the opacity %v is not valid", v)
	}
	return n
}

// solidFill returns a fill of a color with the default rule, as red_fill,
// green_fill, gray_fill and cmyk_fill of krilla tests/src/main.rs do.
func solidFill(t *testing.T, c Color, o float32) *Fill {
	t.Helper()
	return &Fill{Paint: c, Opacity: opacity(t, o), Rule: FillRuleNonZero}
}

func redFill(t *testing.T, o float32) *Fill {
	t.Helper()
	return solidFill(t, NewColorRGB(255, 0, 0), o)
}

func greenFill(t *testing.T, o float32) *Fill {
	t.Helper()
	return solidFill(t, NewColorRGB(0, 255, 0), o)
}

func blueFill(t *testing.T, o float32) *Fill {
	t.Helper()
	return solidFill(t, NewColorRGB(0, 0, 255), o)
}

// redStroke is red_stroke of krilla tests/src/main.rs.
func redStroke(t *testing.T, o, width float32) *Stroke {
	t.Helper()
	s := DefaultStroke()
	s.Paint = NewColorRGB(255, 0, 0)
	s.Opacity = opacity(t, o)
	s.Width = width
	return &s
}

// snapshotPage starts the page of a snapshot test of a page, as the macro
// snapshot of krilla does: a page of 200 by 200 with the settings 1.
func snapshotPage() (*Document, *Page) {
	d := NewDocument(settings1()...)
	ps, _ := PageSettingsFromWH(200, 200)
	return d, d.StartPage(ps)
}

// TestPathWithRGB is the snapshot test path_with_rgb of krilla
// tests/src/path.rs.
func TestPathWithRGB(t *testing.T) {
	d, page := snapshotPage()
	surface := page.Surface()
	surface.SetFill(redFill(t, 1))
	surface.DrawPath(rectToPath(t, 20, 20, 180, 180))
	checkSnapshot(t, "path_with_rgb", d)
}

// TestPathWithLuma is the snapshot test path_with_luma of krilla
// tests/src/path.rs.
func TestPathWithLuma(t *testing.T) {
	d, page := snapshotPage()
	surface := page.Surface()
	surface.SetFill(solidFill(t, NewColorLuma(127), 1))
	surface.DrawPath(rectToPath(t, 20, 20, 180, 180))
	checkSnapshot(t, "path_with_luma", d)
}

// TestPathWithRGBAndOpacity is the snapshot test path_with_rgb_and_opacity
// of krilla tests/src/path.rs.
func TestPathWithRGBAndOpacity(t *testing.T) {
	d, page := snapshotPage()
	surface := page.Surface()
	surface.SetFill(redFill(t, 0.5))
	surface.DrawPath(rectToPath(t, 20, 20, 180, 180))
	checkSnapshot(t, "path_with_rgb_and_opacity", d)
}

// TestPathWithCMYK is the snapshot test path_with_cmyk of krilla
// tests/src/path.rs.
func TestPathWithCMYK(t *testing.T) {
	d, page := snapshotPage()
	surface := page.Surface()
	surface.SetFill(solidFill(t, NewColorCMYK(0, 8, 252, 5), 1))
	surface.DrawPath(rectToPath(t, 20, 20, 180, 180))
	checkSnapshot(t, "path_with_cmyk", d)
}

// TestStreamNestedTransforms is the snapshot test stream_nested_transforms
// of krilla tests/src/stream.rs.
func TestStreamNestedTransforms(t *testing.T) {
	d, page := snapshotPage()
	surface := page.Surface()
	path1 := rectToPath(t, 0, 0, 100, 100)

	surface.PushTransform(path.TransformFromTranslate(50, 50))
	surface.SetFill(greenFill(t, 1))
	surface.DrawPath(path1)
	surface.PushTransform(path.TransformFromTranslate(100, 100))
	surface.SetFill(redFill(t, 1))
	surface.DrawPath(path1)

	surface.Pop()
	surface.Pop()
	checkSnapshot(t, "stream_nested_transforms", d)
}

// TestStreamReusedGraphicsState is the snapshot test
// stream_reused_graphics_state of krilla tests/src/stream.rs.
func TestStreamReusedGraphicsState(t *testing.T) {
	d, page := snapshotPage()
	surface := page.Surface()
	path1 := rectToPath(t, 0, 0, 100, 100)
	surface.SetFill(greenFill(t, 0.5))
	surface.DrawPath(path1)
	surface.PushBlendMode(BlendModeColorBurn)
	surface.SetFill(greenFill(t, 0.5))
	surface.DrawPath(path1)
	surface.Pop()
	surface.SetFill(greenFill(t, 0.5))
	surface.DrawPath(path1)
	checkSnapshot(t, "stream_reused_graphics_state", d)
}

// basicMask is basic_mask of krilla tests/src/main.rs.
func basicMask(t *testing.T, surface *Surface, maskType MaskType) *Mask {
	t.Helper()
	streamBuilder := surface.StreamBuilder()
	subSurface := streamBuilder.Surface()
	subSurface.SetFill(redFill(t, 0.2))
	subSurface.DrawPath(rectToPath(t, 20, 20, 180, 180))
	subSurface.Finish()
	return NewMask(streamBuilder.Finish(), maskType)
}

// TestMask is the snapshot test mask of krilla tests/src/mask.rs.
func TestMask(t *testing.T) {
	d, page := snapshotPage()
	surface := page.Surface()
	mask := basicMask(t, surface, MaskTypeAlpha)
	surface.PushMask(mask)
	surface.SetFill(greenFill(t, 0.5))
	surface.DrawPath(rectToPath(t, 0, 0, 100, 100))
	surface.Pop()
	checkSnapshot(t, "mask", d)
}

// TestGraphic is the snapshot test graphic of krilla tests/src/graphic.rs.
func TestGraphic(t *testing.T) {
	d, page := snapshotPage()
	surface := page.Surface()

	streamBuilder := surface.StreamBuilder()
	streamSurface := streamBuilder.Surface()
	streamSurface.SetFill(redFill(t, 0.5))
	streamSurface.DrawPath(rectToPath(t, 0, 0, 20, 20))
	streamSurface.Finish()
	graphic := NewGraphic(streamBuilder.Finish(), false)

	surface.DrawGraphic(graphic)
	surface.PushTransform(path.TransformFromTranslate(20, 20))
	surface.DrawGraphic(graphic)
	surface.Pop()

	surface.PushTransform(path.TransformFromTranslate(40, 40))
	surface.DrawGraphic(graphic)
	surface.Pop()
	checkSnapshot(t, "graphic", d)
}

// TestStreamCTM is the test stream_ctm of krilla tests/src/stream.rs.
func TestStreamCTM(t *testing.T) {
	d := NewDocument()
	ps, _ := PageSettingsFromWH(200, 200)
	page := d.StartPage(ps)
	surface := page.Surface()

	scale := path.TransformFromScale(2, 3)
	surface.PushTransform(scale)
	surface.PushIsolated()
	surface.PushTransform(path.TransformFromTranslate(5, 0))

	scaledTranslate := path.TransformFromRow(2, 0, 0, 3, 10, 0)
	if got := surface.CTM(); got != scaledTranslate {
		t.Errorf("the CTM is %v, want %v", got, scaledTranslate)
	}

	{
		streamBuilder := surface.StreamBuilder()
		subSurface := streamBuilder.Surface()
		if got := subSurface.CTM(); got != scaledTranslate {
			t.Errorf("the CTM of the stream is %v, want %v", got, scaledTranslate)
		}

		subSurface.PushTransform(path.TransformFromScale(0.5, 0.5))
		if got, want := subSurface.CTM(), path.TransformFromRow(1, 0, 0, 1.5, 10, 0); got != want {
			t.Errorf("the scaled CTM of the stream is %v, want %v", got, want)
		}
		subSurface.Pop()
		if got := subSurface.CTM(); got != scaledTranslate {
			t.Errorf("the CTM of the stream after the pop is %v, want %v", got, scaledTranslate)
		}
		subSurface.Finish()
	}

	surface.Pop()
	surface.Pop()
	if got := surface.CTM(); got != scale {
		t.Errorf("the CTM is %v, want %v", got, scale)
	}
	surface.Pop()
	if got := surface.CTM(); got != path.IdentityTransform() {
		t.Errorf("the CTM is %v, want the identity", got)
	}

	surface.Finish()
	page.Finish()
	if err := d.Finish(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}

// pageContent draws on the surface of a page and returns the content stream
// of the page and the error of Document.Finish.
func pageContent(t *testing.T, draw func(s *Surface)) (string, error) {
	t.Helper()
	d := NewDocument(WithCompression(false), WithPretty(true))
	ps, _ := PageSettingsFromWH(200, 200)
	page := d.StartPage(ps)
	s := page.Surface()
	draw(s)
	s.Finish()
	content := string(page.pageStream.content)
	page.Finish()
	return content, d.Finish(&bytes.Buffer{})
}

// countLines counts the lines that are line.
func countLines(lines []string, line string) int {
	n := 0
	for _, l := range lines {
		if l == line {
			n++
		}
	}
	return n
}

// TestDrawPathZeroSize checks the paths that draw_path of krilla
// src/content.rs leaves out: a path of no size, and the fill of a line.
func TestDrawPathZeroSize(t *testing.T) {
	line := func(x1, y1, x2, y2 float32) *Path {
		b := path.NewPathBuilder()
		b.MoveTo(x1, y1)
		b.LineTo(x2, y2)
		p, _ := b.Finish()
		return p
	}
	tests := []struct {
		name   string
		path   *Path
		fill   bool
		stroke *Stroke
		want   string
	}{
		{"point with a fill", line(10, 10, 10, 10), true, nil, ""},
		{"point with a butt stroke", line(10, 10, 10, 10), false, redStroke(t, 1, 10), ""},
		{"point with a round stroke", line(10, 10, 10, 10), false, func() *Stroke {
			s := redStroke(t, 1, 10)
			s.LineCap = LineCapRound
			return s
		}(), "1 J"},
		{"line with a fill", line(10, 10, 50, 50), true, nil, ""},
		{"line with a fill and a stroke", line(10, 10, 50, 50), true, redStroke(t, 1, 2), "S"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := pageContent(t, func(s *Surface) {
				if tt.fill {
					s.SetFill(redFill(t, 1))
				}
				s.SetStroke(tt.stroke)
				s.DrawPath(tt.path)
			})
			if err != nil {
				t.Fatal(err)
			}
			if tt.want == "" && content != "" {
				t.Errorf("the content is %q, want nothing", content)
			}
			if tt.want != "" && !strings.Contains(content, tt.want) {
				t.Errorf("the content %q does not hold %q", content, tt.want)
			}
			if strings.Contains(content, "\nf") || strings.Contains(content, "\nB") {
				t.Errorf("the content %q fills a line", content)
			}
		})
	}
}

// TestEmptyGradient checks that draw_path and draw_glyphs of krilla
// src/content.rs paint nothing for a gradient with no stop, as
// Paint::is_empty_gradient of src/graphics/paint.rs finds it.
func TestEmptyGradient(t *testing.T) {
	id := path.IdentityTransform()
	for _, tt := range []struct {
		paint Paint
		empty bool
	}{
		{LinearGradient{X2: 10, Transform: id}, true},
		{RadialGradient{Cr: 10, Transform: id}, true},
		{SweepGradient{EndAngle: 90, Transform: id}, true},
		{LinearGradient{X2: 10, Transform: id, Stops: []Stop{{Color: NewColorRGB(255, 0, 0), Opacity: 1}}}, false},
		{NewColorRGB(0, 0, 0), false},
		{Pattern{}, false},
		{MeshGradient{}, false},
	} {
		if got := paintIsEmptyGradient(tt.paint); got != tt.empty {
			t.Errorf("paintIsEmptyGradient of %T is %v", tt.paint, got)
		}
	}

	empty := LinearGradient{X2: 10, Transform: id}
	draw := func(s *Surface) {
		s.DrawPath(rectToPath(t, 0, 0, 10, 10))
		s.DrawGlyphs(path.PointFromXY(0, 50), []Glyph{{GlyphID: 36, TextRange: TextRange{0, 1}, XAdvance: 0.5}},
			testFont(t, "LibertinusSerif-Regular.otf"), "A", 12, false)
	}

	// A fill and a stroke with no stop paint nothing.
	content, err := pageContent(t, func(s *Surface) {
		s.SetFill(&Fill{Paint: empty, Opacity: 1})
		s.SetStroke(&Stroke{Paint: empty, Width: 1, MiterLimit: 10, Opacity: 1})
		draw(s)
	})
	if err != nil {
		t.Fatal(err)
	}
	if content != "" {
		t.Errorf("the content is %q, want nothing", content)
	}

	// The fill still paints when only the stroke has no stop, and the stroke
	// when only the fill has no stop.
	for _, tt := range []struct {
		name          string
		fill          *Fill
		stroke        *Stroke
		want, notWant string
	}{
		{"fill", redFill(t, 1), &Stroke{Paint: empty, Width: 1, MiterLimit: 10, Opacity: 1}, "\nf", "\nS"},
		{"stroke", &Fill{Paint: empty, Opacity: 1}, redStroke(t, 1, 10), "\nS", "\nf"},
	} {
		content, err := pageContent(t, func(s *Surface) {
			s.SetFill(tt.fill)
			s.SetStroke(tt.stroke)
			s.DrawPath(rectToPath(t, 0, 0, 10, 10))
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(content, tt.want) || strings.Contains(content, tt.notWant) {
			t.Errorf("the %s gives the content %q", tt.name, content)
		}
	}
}

// TestDrawPathOperators checks the operators of the fills and the strokes of
// draw_path of krilla src/content.rs, and the conversion of a quadratic curve
// into a cubic curve.
func TestDrawPathOperators(t *testing.T) {
	b := path.NewPathBuilder()
	b.MoveTo(0, 0)
	b.QuadTo(30, 0, 30, 30)
	b.LineTo(0, 30)
	b.Close()
	p, _ := b.Finish()

	dashed := DefaultStroke()
	dashed.Width = 2
	dashed.MiterLimit = 4
	dashed.LineCap = LineCapSquare
	dashed.LineJoin = LineJoinBevel
	dashed.Dash = &StrokeDash{Array: []float32{3, 1}, Offset: 0.5}
	dashed.Paint = NewColorLuma(51)

	tests := []struct {
		name   string
		fill   *Fill
		stroke *Stroke
		want   []string
	}{
		{"default fill", nil, nil, []string{"0 g", "0 0 m", "20 0 30 10 30 30 c", "0 30 l", "h", "f"}},
		{"even odd", &Fill{Paint: NewColorRGB(0, 0, 255), Opacity: 1, Rule: FillRuleEvenOdd}, nil, []string{"0 0 1 rg", "f*"}},
		{"stroke", nil, &dashed, []string{"0.2 G", "2 w", "4 M", "2 J", "2 j", "[3 1] 0.5 d", "S"}},
		{"fill and stroke", blueFill(t, 1), &dashed, []string{"0 0 1 rg", "0.2 G", "B"}},
		{"fill and stroke even odd", &Fill{Paint: NewColorCMYK(255, 0, 0, 0), Opacity: 1, Rule: FillRuleEvenOdd}, &dashed, []string{"1 0 0 0 k", "B*"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := pageContent(t, func(s *Surface) {
				s.SetFill(tt.fill)
				s.SetStroke(tt.stroke)
				s.DrawPath(p)
			})
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(content, "\n")
			for _, w := range tt.want {
				if countLines(lines, w) == 0 {
					t.Errorf("the content %q has no line %q", content, w)
				}
			}
		})
	}
}

// TestStrokeWithOpacity checks has_complex_fill_or_stroke of krilla
// src/surface.rs: a stroke with an opacity is drawn apart from its fill.
func TestStrokeWithOpacity(t *testing.T) {
	content, err := pageContent(t, func(s *Surface) {
		s.SetFill(greenFill(t, 1))
		s.SetStroke(redStroke(t, 0.5, 2))
		s.DrawPath(rectToPath(t, 10, 10, 50, 50))
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(content, "\nQ") != 2 || !strings.Contains(content, "\nf\n") || !strings.Contains(content, "\nS\n") {
		t.Errorf("the content %q does not fill and stroke apart", content)
	}
}

// TestQNestingLevel checks content_save_state of krilla src/content.rs: a
// nesting of more than 28 levels records TooHighQNestingLevel once, as the
// test validate_pdf_a_q_nesting_28 of krilla tests/src/validate.rs expects.
func TestQNestingLevel(t *testing.T) {
	for _, n := range []int{28, 29, 40} {
		d := NewDocument()
		page := d.StartPage(DefaultPageSettings())
		s := page.Surface()
		for range n {
			s.PushClipPath(rectToPath(t, 0, 0, 100, 100), FillRuleNonZero)
		}
		for range n {
			s.Pop()
		}
		s.Finish()
		var want []validationError
		if n > 28 {
			want = []validationError{{kind: validationTooHighQNestingLevel}}
		}
		got := page.pageStream.validationErrors
		if len(got) != len(want) || (len(want) != 0 && got[0] != want[0]) {
			t.Errorf("%d clip paths give the errors %v, want %v", n, got, want)
		}
		page.Finish()
	}
}

// TestPushPopErrors checks the errors that replace the panics of krilla
// src/surface.rs for pushes and pops that do not match.
func TestPushPopErrors(t *testing.T) {
	_, err := pageContent(t, func(s *Surface) {
		s.Pop()
	})
	if !errors.Is(err, okage.ErrPDFPushPop) {
		t.Errorf("a pop without a push gives %v", err)
	}

	content, err := pageContent(t, func(s *Surface) {
		s.PushClipPath(rectToPath(t, 0, 0, 10, 10), FillRuleEvenOdd)
		s.PushOpacity(opacity(t, 0.5))
		s.PushTransform(path.TransformFromScale(2, 2))
		s.DrawPath(rectToPath(t, 0, 0, 10, 10))
	})
	if !errors.Is(err, okage.ErrPDFPushPop) {
		t.Errorf("a push without a pop gives %v", err)
	}
	lines := strings.Split(content, "\n")
	if slices.Index(lines, "W*") < 0 || countLines(lines, "q") != countLines(lines, "Q") {
		t.Errorf("the content %q is not balanced", content)
	}

	_, err = pageContent(t, func(s *Surface) {
		s.PushTransform(path.TransformFromScale(3e38, 3e38))
		s.PushClipPath(rectToPath(t, 0, 0, 10, 10), FillRuleNonZero)
		s.Pop()
		s.Pop()
	})
	if !errors.Is(err, okage.ErrPDFInvalidClipPath) {
		t.Errorf("a clip path that is not finite gives %v", err)
	}
}

// TestPaintErrors checks the errors that replace the panics of krilla for a
// path to stroke with coordinates that are too large, for the stops of a
// gradient in two color spaces, and for a gradient that is not valid.
func TestPaintErrors(t *testing.T) {
	quad := func(x1, y1, x2, y2, x3, y3 float32) *Path {
		b := path.NewPathBuilder()
		b.MoveTo(x1, y1)
		b.QuadTo(x2, y2, x3, y3)
		p, ok := b.Finish()
		if !ok {
			t.Fatal("the path is empty")
		}
		return p
	}
	stops := func(a, b Color) []Stop {
		return []Stop{{Color: a, Opacity: 1}, {Offset: 1, Color: b, Opacity: 1}}
	}
	rgbStops := stops(NewColorRGB(255, 0, 0), NewColorRGB(0, 0, 255))
	singular := path.TransformFromScale(0, 1)
	// huge is a quadratic curve whose products overflow to NaN in
	// findQuadMaxCurvature of the stroker, where tiny-skia panics.
	huge := quad(2e37, -2e37, -1, 1e20, 1, 1e38)

	tests := []struct {
		name string
		draw func(s *Surface)
		want error
	}{
		{"stroke in a group", func(s *Surface) {
			s.PushIsolated()
			s.SetStroke(redStroke(t, 1, 2))
			s.DrawPath(huge)
			s.Pop()
		}, okage.ErrPDFInvalidPath},
		{"stroke of a gradient", func(s *Surface) {
			k := DefaultStroke()
			k.Paint = LinearGradient{X2: 10, Transform: path.IdentityTransform(), Stops: rgbStops}
			s.SetStroke(&k)
			s.DrawPath(quad(0, 0, 1e30, 1, 2e30, 0))
		}, okage.ErrPDFInvalidPath},
		// A path with a point that is NaN, whose stroke does not end in
		// tiny-skia, has no case here: PathBuilder.Finish refuses it
		// (D107). TestStrokeNotFinite of path covers the check of D74.
		{"wide stroke in a group", func(s *Surface) {
			s.PushIsolated()
			s.SetStroke(redStroke(t, 1, 2e18))
			s.DrawPath(rectToPath(t, 0, 0, 10, 10))
			s.Pop()
		}, okage.ErrPDFInvalidPath},
		{"solid stroke on the page", func(s *Surface) {
			// krilla does not stroke the path for the bbox here.
			s.SetStroke(redStroke(t, 1, 2))
			s.DrawPath(huge)
		}, nil},
		{"stroke that fits", func(s *Surface) {
			s.PushIsolated()
			s.SetStroke(redStroke(t, 1, 2))
			s.DrawPath(quad(0, 0, 1e17, 1e17, 2e17, 0))
			s.Pop()
		}, nil},
		{"mixed stops", func(s *Surface) {
			s.SetFill(&Fill{Paint: LinearGradient{
				X2: 10, Transform: path.IdentityTransform(),
				Stops: stops(NewColorRGB(255, 0, 0), NewColorCMYK(0, 0, 0, 255)),
			}, Opacity: 1})
			s.DrawPath(rectToPath(t, 0, 0, 10, 10))
		}, okage.ErrPDFMixedColorSpaces},
		{"mixed stops of a sweep", func(s *Surface) {
			s.SetFill(&Fill{Paint: SweepGradient{
				EndAngle: 90, Transform: path.IdentityTransform(),
				Stops: stops(NewColorLuma(0), NewColorRGB(0, 0, 255)),
			}, Opacity: 1})
			s.DrawPath(rectToPath(t, 0, 0, 10, 10))
		}, okage.ErrPDFMixedColorSpaces},
		{"no stop", func(s *Surface) {
			s.SetFill(&Fill{Paint: RadialGradient{Cr: 10, Transform: path.IdentityTransform()}, Opacity: 1})
			s.DrawPath(rectToPath(t, 0, 0, 10, 10))
		}, nil},
		{"no stop with a singular transform", func(s *Surface) {
			s.SetFill(&Fill{Paint: LinearGradient{X2: 10, Transform: singular, SpreadMethod: SpreadMethodReflect}, Opacity: 1})
			s.DrawPath(rectToPath(t, 0, 0, 10, 10))
		}, nil},
		{"singular transform", func(s *Surface) {
			s.SetFill(&Fill{Paint: LinearGradient{
				X2: 10, Transform: singular, SpreadMethod: SpreadMethodReflect, Stops: rgbStops,
			}, Opacity: 1})
			s.DrawPath(rectToPath(t, 0, 0, 10, 10))
		}, okage.ErrPDFInvalidGradient},
		{"singular transform of a pad gradient", func(s *Surface) {
			s.SetFill(&Fill{Paint: LinearGradient{X2: 10, Transform: singular, Stops: rgbStops}, Opacity: 1})
			s.DrawPath(rectToPath(t, 0, 0, 10, 10))
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pageContent(t, tt.draw)
			if tt.want == nil && err != nil || tt.want != nil && !errors.Is(err, tt.want) {
				t.Errorf("the error is %v, want %v", err, tt.want)
			}
		})
	}
}

// TestSurfaceFinish checks that a page finishes its surface, that a second
// surface of a page replaces the drawings of the first, and that a second
// Finish of a surface does nothing.
func TestSurfaceFinish(t *testing.T) {
	d := NewDocument(WithCompression(false))
	ps, _ := PageSettingsFromWH(200, 200)
	page := d.StartPage(ps)
	s := page.Surface()
	s.SetFill(redFill(t, 1))
	s.DrawPath(rectToPath(t, 0, 0, 10, 10))
	s2 := page.Surface()
	if !strings.Contains(string(page.pageStream.content), "1 0 0 rg") {
		t.Errorf("the first surface did not finish: %q", page.pageStream.content)
	}
	s.Finish()
	s2.SetFill(greenFill(t, 1))
	s2.DrawPath(rectToPath(t, 0, 0, 10, 10))
	page.Finish()
	s2.Finish()
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "0 1 0 rg") || strings.Contains(buf.String(), "1 0 0 rg") {
		t.Errorf("the page does not hold the drawings of the second surface only:\n%s", buf.String())
	}
}

// TestStreamBuilderSurfaceWithBBox checks StreamBuilder::surface_with_bbox
// of krilla src/stream.rs: the surface flips its y axis, and the bbox of the
// stream holds the bbox that it starts with.
func TestStreamBuilderSurfaceWithBBox(t *testing.T) {
	d := NewDocument(WithCompression(false), WithPretty(true))
	ps, _ := PageSettingsFromWH(200, 200)
	page := d.StartPage(ps)
	s := page.Surface()
	b := s.StreamBuilder()
	bbox, _ := path.RectFromXYWH(0, 0, 40, 20)
	sub := b.SurfaceWithBBox(bbox)
	sub.SetFill(redFill(t, 1))
	sub.DrawPath(rectToPath(t, 5, 5, 10, 10))
	sub.Finish()
	stream := b.Finish()
	if !strings.HasPrefix(string(stream.content), "q\n1 0 0 -1 0 20 cm\n") {
		t.Errorf("the stream %q does not flip its y axis", stream.content)
	}
	if stream.bbox != bbox {
		t.Errorf("the bbox is %v, want %v", stream.bbox, bbox)
	}
	s.Finish()
	page.Finish()
	if err := d.Finish(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}
