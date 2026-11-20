// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"fmt"
	"math"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/typesetting/font"
)

// This file ports rowgrid_test.go of the source: the vertical models of the
// grid in leading.go, and the delimiters that grow with their content.
//
// The source measures at 10px, and okage at the size 10, where one point is
// one unit, as in the source. The values of tectonic, a TeX engine, are the
// values that the comments of the source give. Where okage still gives the
// value of tectonic, the test compares with it. Where it does not, the test
// computes the exact value again from the font, and the comment says how far
// it is from tectonic.

// rowgridTotal returns the height plus the depth of tex at the size 10 in
// text style.
func rowgridTotal(t *testing.T, tex string) float64 {
	t.Helper()
	b := mustLayout(t, tex, 10, Text)
	return b.Height + b.Depth
}

// rowgridLayout returns a layout of STIX Two Math at the size 10, for a test
// that calls a function of the layout itself.
func rowgridLayout(t *testing.T) *layout {
	t.Helper()
	face := testFace(t)
	return &layout{face: face, size: 10, upem: float64(face.Upem()), cfg: newConfig(nil), color: okage.ColorBlack}
}

// The model of TeX for anything on \array, each matrix and cases: \@arstrut
// puts a strut of \arraystretch times .7 and .3 of \baselineskip in each row
// (latex.ltx:12103), and the rows touch. \jot, where an environment has one,
// is the only space between them.
//
// okage: the delimiters of pmatrix, bmatrix and vmatrix cover 901/1000 of the
// rows, as \delimiterfactor of TeX says, so they no longer reach past the
// struts, and the totals are those of tectonic. cases is 14.40 at one row,
// the height of its rows, where tectonic gives 18.00 and the source 16.67:
// the brace of STIX Two Math that covers 901/1000 of 14.40 is 14.28 tall, so
// it does not set the total. At three rows, cases is 43.20, the value of
// tectonic, where the source gives 45.00 with a brace that covers the whole
// height.
func TestRowsStackOnStrutsWithNoInterlineGlue(t *testing.T) {
	for _, c := range []struct {
		name       string
		one, three string
		wantOne    float64
		wantThree  float64
	}{
		{"array", `\begin{array}{l}x\end{array}`, `\begin{array}{l}x\\x\\x\end{array}`, 12, 36},
		{"matrix", `\begin{matrix}x\end{matrix}`, `\begin{matrix}x\\x\\x\end{matrix}`, 12, 36},
		{"pmatrix", `\begin{pmatrix}x\end{pmatrix}`, `\begin{pmatrix}x\\x\\x\end{pmatrix}`, 12, 36},
		{"bmatrix", `\begin{bmatrix}x\end{bmatrix}`, `\begin{bmatrix}x\\x\\x\end{bmatrix}`, 12, 36},
		{"vmatrix", `\begin{vmatrix}x\end{vmatrix}`, `\begin{vmatrix}x\\x\\x\end{vmatrix}`, 12, 36},
		// cases has \arraystretch 1.2, so its rows are 14.40 each.
		{"cases", `\begin{cases}x & y\end{cases}`,
			`\begin{cases}x & y\\x & y\\x & y\end{cases}`, 14.40, 43.20},
		// \jot adds 3pt at 10pt between the rows: 12 + 2 x 15.
		{"aligned", `\begin{aligned}x &= y\end{aligned}`,
			`\begin{aligned}x &= y\\x &= y\\x &= y\end{aligned}`, 12, 42},
		{"gathered", `\begin{gathered}x\end{gathered}`,
			`\begin{gathered}x\\x\\x\end{gathered}`, 12, 42},
	} {
		t.Run(c.name, func(t *testing.T) {
			if v := rowgridTotal(t, c.one); math.Abs(v-c.wantOne) > 0.05 {
				t.Errorf("one row = %.3f, want %.2f", v, c.wantOne)
			}
			if v := rowgridTotal(t, c.three); math.Abs(v-c.wantThree) > 0.05 {
				t.Errorf("three rows = %.3f, want %.2f", v, c.wantThree)
			}
		})
	}
}

// cases has \arraystretch 1.2 (amsmath.sty:1106), so its rows are 14.40 and
// three of them are 43.20: the pitch follows the stretch in proportion,
// because the rows touch and the strut is the pitch. The interline glue of
// tex.web §679 gives the same value at a stretch of 1 only, and 15.40 for each
// row at 1.2. The model is checked on the struts themselves, where no
// delimiter can reach it.
func TestTheStretchedEnvironmentTracksItsStrutLinearly(t *testing.T) {
	for _, c := range []struct {
		stretch, perRow float64
	}{{1.0, 12.00}, {1.2, 14.40}, {1.5, 18.00}} {
		h, d := strutLeading{baselineskip: bl(10), stretch: c.stretch}.strut()
		if got := h + d; math.Abs(got-c.perRow) > 0.005 {
			t.Errorf("strut at \\arraystretch %g = %.3f, tectonic gives %.2f per row",
				c.stretch, got, c.perRow)
		}
		// The rows touch: n rows are n struts and nothing else.
		if gap := (strutLeading{baselineskip: bl(10), stretch: c.stretch}).gap(d, h); gap != 0 {
			t.Errorf("strutted rows must touch, gap = %.3f at stretch %g", gap, c.stretch)
		}
	}
	// okage: the three rows of cases are 43.20, the value of tectonic. The
	// source gives 45.00, a brace that covers the whole height of the rows.
	if v := rowgridTotal(t, `\begin{cases}x & y\\x & y\\x & y\end{cases}`); math.Abs(v-43.20) > 0.05 {
		t.Errorf("cases three rows = %.3f, tectonic gives 43.20", v)
	}
	if v := rowgridTotal(t, `\begin{pmatrix}x\\x\\x\end{pmatrix}`); math.Abs(v-36) > 0.05 {
		t.Errorf("pmatrix three rows = %.3f, tectonic gives 36.00", v)
	}
}

// A row taller than the strut takes its own height: the strut is a floor,
// not a fixed pitch.
//
// okage: the expected total is computed again from the font: two rows of
// struts, 12.00 each, and the height and the depth of the fraction, laid out
// alone in text style, the style of the cells. The fraction of STIX Two Math
// is taller than the one of tectonic, because the MATH table gives
// FractionNumeratorShiftUp and FractionDenominatorShiftDown of 0.585 em, so the
// row opens the pitch by 8.42 where tectonic opens it by 1.95. The source,
// which takes the gaps of a fraction only, opens it by 2.20.
func TestATallRowTakesItsOwnHeight(t *testing.T) {
	flat := rowgridTotal(t, `\begin{pmatrix}x\\x\\x\end{pmatrix}`) - rowgridTotal(t, `\begin{pmatrix}x\\x\end{pmatrix}`)
	if math.Abs(flat-12) > 0.05 {
		t.Errorf("flat row added = %.3f, tectonic gives 12.00", flat)
	}
	frac := mustLayout(t, `\frac{\frac{a}{b}}{c}`, 10, Text)
	sh, sd := strutLeading{baselineskip: bl(10)}.strut()
	want := 2*(sh+sd) + max(frac.Height, sh) + max(frac.Depth, sd)
	for _, env := range []string{"matrix", "pmatrix"} {
		tex := `\begin{` + env + `}x\\x\\\frac{\frac{a}{b}}{c}\end{` + env + `}`
		if got := rowgridTotal(t, tex); !near(got, want) {
			t.Errorf("%s = %.4f, want %.4f", tex, got, want)
		}
	}
	t.Logf("a tall row opens the pitch by %.3f, tectonic by 1.95", want-36)
}

// okage: the source pins the total of cases at one row to 16.67, a brace that
// is taller than the rows. With \delimiterfactor, the brace covers 901/1000 of
// the rows only. This checks that the brace is chosen as that rule says: the
// smallest variant of the brace that reaches 901/1000 of the height around
// the axis, centered on the axis, and that it does not reach past the rows.
func TestCasesOneRowIsFlooredByItsBrace(t *testing.T) {
	face := testFace(t)
	const rows = 14.40
	axis := mathConst(face, font.AxisHeight, 10)
	h, d := rows/2+axis, rows/2-axis // the rows, centered on the axis
	delta := 2 * max(h-axis, d+axis)
	target := max(float64(delta*901)/1000, delta-5)
	brace := rowgridVariant(t, face, '{', target, 10)
	if brace/2+axis > h || brace/2-axis > d {
		t.Errorf("the brace of %.3f reaches past the rows %.3f/%.3f", brace, h, d)
	}
	if got := rowgridTotal(t, `\begin{cases}x & y\end{cases}`); !near(got, rows) {
		t.Errorf("cases one row = %.4f, want the rows, %.2f (tectonic 18.00)", got, rows)
	}
}

// rowgridVariant returns the height of the first vertical variant of r that
// reaches target at size, as stretchVertical chooses it.
func rowgridVariant(t *testing.T, face *font.Face, r rune, target, size float64) float64 {
	t.Helper()
	s := size / float64(face.Upem())
	for _, v := range face.MathGlyphVariants(glyphOf(t, face, r), true) {
		if a := float64(float64(v.Advance) * s); a >= target {
			return a
		}
	}
	t.Fatalf("no variant of %q reaches %.3f", r, target)
	return 0
}

// smallmatrix takes the interline glue of TeX, not a strut: amsmath.sty:1045-1047
// sets \baselineskip6\ex@ \lineskip1.5\ex@ \lineskiplimit\lineskip, so §679
// decides each gap from the extents of the rows. The cells are \rule boxes,
// whose height and depth are given, so the face is not involved. The values
// are those of tectonic, at 10pt, where \ex@ is 1.
//
// The rows of 5pt separate the models: a grid that always moves by
// \baselineskip gives 11.0, and §679 gives 11.5, because the gap is under
// \lineskiplimit and \lineskip replaces it.
func TestSmallmatrixTakesInterlineGlue(t *testing.T) {
	for _, c := range []struct {
		tex  string
		want float64
	}{
		{`\begin{smallmatrix}\rule{1pt}{1pt}\\\rule{1pt}{1pt}\end{smallmatrix}`, 7.0},
		{`\begin{smallmatrix}\rule{1pt}{1pt}\\\rule{1pt}{1pt}\\\rule{1pt}{1pt}\end{smallmatrix}`, 13.0},
		{`\begin{smallmatrix}\rule{1pt}{4pt}\\\rule{1pt}{4pt}\end{smallmatrix}`, 10.0},
		{`\begin{smallmatrix}\rule{1pt}{4.5pt}\\\rule{1pt}{4.5pt}\end{smallmatrix}`, 10.5},
		{`\begin{smallmatrix}\rule{1pt}{5pt}\\\rule{1pt}{5pt}\end{smallmatrix}`, 11.5},
		{`\begin{smallmatrix}\rule{1pt}{10pt}\\\rule{1pt}{10pt}\end{smallmatrix}`, 21.5},
		{`\begin{smallmatrix}\rule{1pt}{10pt}\\\rule{1pt}{10pt}\\\rule{1pt}{10pt}\end{smallmatrix}`, 33.0},
	} {
		if v := rowgridTotal(t, c.tex); math.Abs(v-c.want) > 0.01 {
			t.Errorf("%s = %.4f, tectonic gives %.4f", c.tex, v, c.want)
		}
	}
}

// The pitch of rows that fit in the leading is \baselineskip, 6\ex@, for any
// rows. It is checked on three heights of rows that take the same branch.
func TestSmallmatrixPitchIsSixExAt(t *testing.T) {
	want := 6 * exAt(10)
	for _, row := range []string{`\rule{1pt}{1pt}`, `\rule{1pt}{2pt}`, `\rule{1pt}{4pt}`} {
		two := rowgridTotal(t, `\begin{smallmatrix}`+row+`\\`+row+`\end{smallmatrix}`)
		three := rowgridTotal(t, `\begin{smallmatrix}`+row+`\\`+row+`\\`+row+`\end{smallmatrix}`)
		if p := three - two; math.Abs(p-want) > 1e-9 {
			t.Errorf("rows of %s: pitch %.4f, 6\\ex@ is %.4f", row, p, want)
		}
	}
}

// On glyphs, the totals of smallmatrix differ from tectonic, and the face
// makes the whole difference, not the grid. The axis moves the height against
// the depth, and the x of the script size is the x of the face.
//
// okage: each value is computed again from the font. The axis is AxisHeight,
// 2.58 at 10, where the source rounds it to 3 pixels and tectonic gives 2.50.
// The script size is 7, as in tectonic. A grid is centered on the axis, so
// the height of a grid of one row less the axis is half of the row:
// 2(h - axis) is the height plus the depth of the x of the script size.
func TestSmallmatrixResidualIsTheFaceNotTheGrid(t *testing.T) {
	face := testFace(t)
	axis := mathConst(face, font.AxisHeight, 10)
	if want := 2.58; !near(axis, want) {
		t.Errorf("axis = %.4f, want %.4f (tectonic 2.5000)", axis, want)
	}
	if got := scriptSize(face, 10); got != 7 {
		t.Errorf("script size = %v, tectonic uses 7", got)
	}
	_, xh, xd := glyphMetrics(t, face, mathItalic('x'), 7)
	b := mustLayout(t, `\begin{smallmatrix}x\end{smallmatrix}`, 10, Text)
	if T := 2 * (b.Height - axis); !near(T, xh+xd) {
		t.Errorf("x at script size = %.4f, want %.4f (tectonic 3.0138)", T, xh+xd)
	}
}

// The horizontal spaces of smallmatrix are in mu of the outer size, not of
// the script size of the cells: the \thickspace and the two \, of the template
// are outside the $\m@th\scriptstyle##$ of each cell (amsmath.sty:1045-1052).
// The cells are \rule{1pt}{1pt}, so the face is not involved. R is one such
// rule and S a smallmatrix of one cell. The environment is an Ord, so no space
// between atoms comes at its edges.
//
// The values are those of tectonic. TeX converts mu in scaled points and
// truncates, so 5mu is 2.7770 where 5/18 x 10 is 2.7778: 0.002 bounds that.
func TestSmallmatrixSpacesInMuAtTheOuterSize(t *testing.T) {
	const R = `\rule{1pt}{1pt}`
	const S = `\begin{smallmatrix}` + R + `\end{smallmatrix}`
	for _, c := range []struct {
		tex  string
		want float64
	}{
		{S, 4.3332},
		{R + S, 5.3332},
		{S + R, 5.3332},
		{R + S + R, 6.3332},
		{`\begin{smallmatrix}` + R + `&` + R + `\end{smallmatrix}`, 8.1102},
		{`\begin{smallmatrix}` + R + `&` + R + `&` + R + `\end{smallmatrix}`, 11.8872},
	} {
		if w := mustLayout(t, c.tex, 10, Text).Width; math.Abs(w-c.want) > 0.002 {
			t.Errorf("%s width = %.4f, tectonic gives %.4f", c.tex, w, c.want)
		}
	}
}

// rowgridAssembly returns the height of the assembly of the vertical brace
// of STIX Two Math that reaches target at size, computed from the parts of
// its recipe: the smallest count of repeats of the extenders, zero or more,
// that reaches the target at the smallest overlap, and then the even overlap, between the
// smallest overlap and the tightest joint, that lands closest to the target.
func rowgridAssembly(t *testing.T, face *font.Face, target, size float64) float64 {
	t.Helper()
	asm, ok := face.MathGlyphAssembly(glyphOf(t, face, '{'), true)
	if !ok {
		t.Fatal("the brace has no assembly")
	}
	s := size / float64(face.Upem())
	minOv := float64(float64(face.MathMinConnectorOverlap()) * s)
	maxOv := math.Inf(1)
	var fixed, ext float64
	var nFixed, nExt int
	for i, p := range asm.Parts {
		if i+1 < len(asm.Parts) {
			maxOv = min(maxOv, float64(min(float64(p.EndConnectorLength), float64(asm.Parts[i+1].StartConnectorLength))*s))
		}
		a := float64(float64(p.FullAdvance) * s)
		if p.Extender {
			maxOv = min(maxOv, float64(min(float64(p.EndConnectorLength), float64(p.StartConnectorLength))*s))
			ext += a
			nExt++
		} else {
			fixed += a
			nFixed++
		}
	}
	for n := 0; ; n++ {
		k := nFixed + n*nExt
		adv := fixed + float64(float64(n)*ext)
		if adv-float64(minOv*float64(k-1)) >= target {
			ov := max(minOv, min(maxOv, (adv-target)/float64(k-1)))
			return adv - float64(ov*float64(k-1))
		}
	}
}

// The delimiter that grows does not stop growing: past the largest variant,
// the assembly of the MATH table builds a brace of any height. The source
// pins the totals of \left\lbrace\rule{1pt}{H}\right. at 10pt. tectonic gives
// 42.5, 49.5, 61.5, 73.5, 114.5, 193.5 and 392.5 for the contents of 24, 28,
// 34, 40, 60, 100 and 200pt.
//
// okage: the expected values are computed again from the font. The brace
// covers 901/1000 of twice the larger extent around the axis, and misses at
// most 5pt of it (\delimiterfactor and \delimitershortfall). Its depth is half
// its height less the axis, because it is centered on the axis, and the rule
// has no depth. okage gives 40.72, 48.34, 60.34, 72.34, 112.34, 192.34 and
// 392.34, each at most 2.2 from tectonic.
func TestTheExtensibleDelimiterDoesNotStopGrowing(t *testing.T) {
	face := testFace(t)
	axis := mathConst(face, font.AxisHeight, 10)
	tectonic := map[float64]float64{24: 42.5, 28: 49.5, 34: 61.5, 40: 73.5, 60: 114.501, 100: 193.501, 200: 392.502}
	for _, content := range []float64{24, 28, 34, 40, 60, 100, 200} {
		delta := 2 * max(content-axis, axis)
		target := max(float64(delta*901)/1000, delta-5)
		brace := rowgridAssembly(t, face, target, 10)
		tex := fmt.Sprintf(`\left\lbrace\rule{1pt}{%gpt}\right.`, content)
		b := mustLayout(t, tex, 10, Text)
		if wantD := brace/2 - axis; math.Abs(b.Depth-wantD) > 1e-9 {
			t.Errorf("content %gpt: delimiter depth %.6f, want %.6f", content, b.Depth, wantD)
		}
		if math.Abs(b.Height-max(content, brace/2+axis)) > 1e-9 {
			t.Errorf("content %gpt: height %.6f, want %.6f", content, b.Height, max(content, brace/2+axis))
		}
		if got := b.Height + b.Depth; math.Abs(got-tectonic[content]) > 2.2 {
			t.Errorf("content %gpt: total %.3f, more than 2.2 from tectonic, %.3f", content, got, tectonic[content])
		}
	}
}

// Past the reach of the variants, each increase of the content makes the
// delimiter larger. The depth belongs to the delimiter alone: the rule has
// none.
func TestTheDelimiterGrowsStrictlyWithItsContent(t *testing.T) {
	prev := 0.0
	for _, h := range []float64{24, 40, 60, 100, 160, 200} {
		tex := fmt.Sprintf(`\left\lbrace\rule{1pt}{%gpt}\right.`, h)
		d := mustLayout(t, tex, 10, Text).Depth
		if d <= prev {
			t.Errorf("content %gpt: delimiter depth %.4f, no greater than %.4f at the "+
				"previous height: the delimiter has stopped growing", h, d, prev)
		}
		prev = d
	}
}

// The stack of an assembly follows its own arithmetic: k parts that advance
// adv in all, joined at the overlap ov, reach adv - (k-1)ov. The brace has 5
// parts that advance 6499 font units, 64.99 at 10, with joints that press
// from 1.00 to 5.00, so one repeat spans 44.99 to 60.99. A target in that
// range is met by pressing the joints, not by more parts.
//
// okage: the values are in exact font units. The source gives 65.00 and 1,
// rounded to pixels.
func TestTheAssemblyIsStackedByItsOwnArithmetic(t *testing.T) {
	l := rowgridLayout(t)
	asm, ok := l.face.MathGlyphAssembly(glyphOf(t, l.face, '{'), true)
	if !ok {
		t.Fatal("the face has no assembly for the brace")
	}
	if len(asm.Parts) != 5 || l.face.MathMinConnectorOverlap() != 100 {
		t.Fatalf("the recipe changed: %d parts, smallest overlap %v; compute the bounds again",
			len(asm.Parts), l.face.MathMinConnectorOverlap())
	}
	adv := 0.0
	for _, p := range asm.Parts {
		adv += float64(p.FullAdvance)
	}
	if adv != 6499 {
		t.Fatalf("the parts advance %v font units, the bounds below take 6499", adv)
	}
	for _, target := range []float64{45, 50, 55, 60.9} {
		b := l.assembleVertical(asm, target, 10, Open, "{")
		if b == nil {
			t.Fatalf("target %.2f: no assembly", target)
		}
		if b.h < target-1e-9 {
			t.Errorf("target %.2f: assembled to %.3f, which does not reach it", target, b.h)
		}
		if !near(b.h, target) {
			t.Errorf("target %.2f: assembled to %.6f, the joints can press to it exactly", target, b.h)
		}
	}
}

// The guards of assembleVertical, which STIX Two Math does not reach, with
// recipes that reach them. A recipe whose extender adds nothing once its
// overlap is paid would make the search loop without an end.
//
// okage: the smallest overlap is the one of the face, 100 font units, and not
// a field of the recipe, as MathMinConnectorOverlap of typesetting gives it.
// The recipes are in font units. The source has a case for a nil recipe,
// which has no meaning here: MathGlyphAssembly returns a value and a flag.
func TestTheAssemblyRefusesRecipesItCannotUse(t *testing.T) {
	l := rowgridLayout(t)
	part := func(adv, start, end float32, ext bool) font.MathGlyphPart {
		g, _ := l.face.NominalGlyph('|')
		return font.MathGlyphPart{Glyph: g, FullAdvance: adv, StartConnectorLength: start, EndConnectorLength: end, Extender: ext}
	}
	for _, c := range []struct {
		name  string
		parts []font.MathGlyphPart
		want  bool // an assembly is expected
	}{
		{"no parts", nil, false},
		{"no extender", []font.MathGlyphPart{part(1000, 0, 500, false), part(1000, 500, 0, false)}, false},
		// The extender advances what its overlap costs, so a repeat adds nothing.
		{"extender that pays for itself", []font.MathGlyphPart{part(1000, 0, 1000, false),
			part(100, 1000, 1000, true), part(1000, 1000, 0, false)}, false},
		// The extender first, where its join with itself is the tightest.
		{"extender at the end of the chain", []font.MathGlyphPart{part(2000, 100, 800, true),
			part(2000, 800, 0, false)}, true},
		// Connectors below the smallest overlap, so the floor of maxOv applies.
		{"tight extender", []font.MathGlyphPart{part(2000, 0, 900, false),
			part(2000, 50, 50, true), part(2000, 900, 0, false)}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := l.assembleVertical(font.MathGlyphAssembly{Parts: c.parts}, 100, 10, Open, "|")
			if got := b != nil; got != c.want {
				t.Errorf("assembly built = %v, want %v", got, c.want)
			}
			if b != nil && b.h < 100 {
				t.Errorf("assembled height %.3f, below the target of 100", b.h)
			}
		})
	}
}
