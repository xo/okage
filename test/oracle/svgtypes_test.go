package oracle

import (
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/xo/okage/internal/roxmltree"
	"github.com/xo/okage/internal/svgtypes"
)

type svgtypesQuestion struct {
	Parser string `json:"svgtypes"`
	Value  string `json:"value"`
}

// attrParsers maps the name of an SVG attribute or property to the parsers
// of svgtypes that read its value. A name can have more than one parser, so
// that each parser sees many values.
var attrParsers = map[string][]string{
	"d":                   {"path", "simple_path"},
	"points":              {"points", "number_list"},
	"transform":           {"transform", "transform_list"},
	"gradientTransform":   {"transform", "transform_list"},
	"patternTransform":    {"transform", "transform_list"},
	"transform-origin":    {"transform_origin"},
	"viewBox":             {"viewbox"},
	"preserveAspectRatio": {"aspect_ratio"},
	"fill":                {"paint"},
	"stroke":              {"paint"},
	"color":               {"color"},
	"stop-color":          {"color"},
	"flood-color":         {"color"},
	"lighting-color":      {"color"},
	"filter":              {"filter", "funciri"},
	"clip-path":           {"funciri"},
	"mask":                {"funciri"},
	"marker-start":        {"funciri"},
	"marker-mid":          {"funciri"},
	"marker-end":          {"funciri"},
	"href":                {"iri"},
	"paint-order":         {"paint_order"},
	"font-family":         {"font_families"},
	"font":                {"font_shorthand"},
	"enable-background":   {"enable_background"},
	"orient":              {"angle"},
	"stroke-dasharray":    {"length_list", "number_list"},
	"x":                   {"length", "length_list"},
	"y":                   {"length", "length_list"},
	"dx":                  {"length", "length_list"},
	"dy":                  {"length", "length_list"},
	"rotate":              {"number_list"},
	"width":               {"length"},
	"height":              {"length"},
	"cx":                  {"length"},
	"cy":                  {"length"},
	"r":                   {"length"},
	"rx":                  {"length"},
	"ry":                  {"length"},
	"fx":                  {"length"},
	"fy":                  {"length"},
	"fr":                  {"length"},
	"x1":                  {"length"},
	"y1":                  {"length"},
	"x2":                  {"length"},
	"y2":                  {"length"},
	"refX":                {"length"},
	"refY":                {"length"},
	"markerWidth":         {"length"},
	"markerHeight":        {"length"},
	"stroke-width":        {"length"},
	"stroke-dashoffset":   {"length"},
	"font-size":           {"length"},
	"letter-spacing":      {"length"},
	"word-spacing":        {"length"},
	"baseline-shift":      {"length"},
	"textLength":          {"length"},
	"startOffset":         {"length"},
	"offset":              {"number", "length"},
	"opacity":             {"number"},
	"fill-opacity":        {"number"},
	"stroke-opacity":      {"number"},
	"stop-opacity":        {"number"},
	"flood-opacity":       {"number"},
	"stroke-miterlimit":   {"number"},
	"k1":                  {"number"},
	"k2":                  {"number"},
	"k3":                  {"number"},
	"k4":                  {"number"},
	"scale":               {"number"},
	"seed":                {"number"},
	"azimuth":             {"number"},
	"elevation":           {"number"},
	"surfaceScale":        {"number"},
	"diffuseConstant":     {"number"},
	"specularConstant":    {"number"},
	"specularExponent":    {"number"},
	"limitingConeAngle":   {"number"},
	"divisor":             {"number"},
	"bias":                {"number"},
	"numOctaves":          {"number"},
	"values":              {"number_list"},
	"kernelMatrix":        {"number_list"},
	"tableValues":         {"number_list"},
	"stdDeviation":        {"number_list"},
	"baseFrequency":       {"number_list"},
	"order":               {"number_list"},
	"radius":              {"number_list"},
	"kernelUnitLength":    {"number_list"},
}

// svgtypesQuestions returns the questions of the attributes of each element
// of each file, and of the declarations of each style attribute. A value is
// asked once for each parser.
func svgtypesQuestions(files []string) []svgtypesQuestion {
	seen := map[svgtypesQuestion]bool{}
	var questions []svgtypesQuestion
	add := func(name, value string) {
		for _, p := range attrParsers[name] {
			q := svgtypesQuestion{Parser: p, Value: value}
			if !seen[q] {
				seen[q] = true
				questions = append(questions, q)
			}
		}
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil || !utf8.Valid(b) {
			continue
		}
		doc, err := roxmltree.ParseWithOptions(string(b), xmlOptions())
		if err != nil {
			continue
		}
		for n := range doc.Root().Descendants() {
			for a := range n.Attributes() {
				add(a.Name(), a.Value())
				if a.Name() == "style" {
					for decl := range strings.SplitSeq(a.Value(), ";") {
						if name, value, ok := strings.Cut(decl, ":"); ok {
							add(strings.TrimSpace(name), strings.TrimSpace(value))
						}
					}
				}
			}
		}
	}
	return questions
}

// TestSvgtypes parses the value of each attribute of the SVG files of the
// tests of resvg with okage and with svgtypes, and compares every bit of the
// results (phase 6).
func TestSvgtypes(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}
	questions := svgtypesQuestions(svgFiles(t))
	answers := ask(t, questions)
	diffs := 0
	byParser := map[string]int{}
	for i, q := range questions {
		got, err := safely(func() []byte { return svgtypesAnswer(q) })
		if err != nil {
			got = []byte("panic\n")
		}
		byParser[q.Parser]++
		if string(got) != string(answers[i]) {
			diffs++
			if diffs <= 20 {
				t.Errorf("%s %q differs\nokage:\n%s\nsvgtypes:\n%s", q.Parser, q.Value, got, answers[i])
			}
		}
	}
	names := make([]string, 0, len(byParser))
	for name := range byParser {
		names = append(names, name)
	}
	slices.Sort(names)
	var counts strings.Builder
	for _, name := range names {
		fmt.Fprintf(&counts, " %s=%d", name, byParser[name])
	}
	t.Logf("%d values, %d differ:%s", len(questions), diffs, counts.String())
}

func hx(x float64) string {
	return fmt.Sprintf("%016x", math.Float64bits(x))
}

// qs writes a string as the hex digits of its bytes, as the oracle does.
func qs(s string) string {
	return fmt.Sprintf("x%x", s)
}

var lengthUnits = [...]string{"none", "em", "ex", "px", "in", "cm", "mm", "pt", "pc", "%"}

func length(l svgtypes.Length) string {
	return hx(l.Number) + " " + lengthUnits[l.Unit]
}

var angleUnits = [...]string{"deg", "grad", "rad", "turn"}

func angle(a svgtypes.Angle) string {
	return hx(a.Number) + " " + angleUnits[a.Unit]
}

func color(c svgtypes.Color) string {
	return fmt.Sprintf("%d %d %d %d", c.Red, c.Green, c.Blue, c.Alpha)
}

func optStr(s string) string {
	if s == "" {
		return "-"
	}
	return qs(s)
}

func absRel(abs bool) string {
	if abs {
		return "A"
	}
	return "R"
}

func pathSegment(s svgtypes.PathSegment) string {
	a := absRel(s.Abs)
	switch s.Kind {
	case svgtypes.PathSegmentMoveTo:
		return fmt.Sprintf("M %s %s %s", a, hx(s.X), hx(s.Y))
	case svgtypes.PathSegmentLineTo:
		return fmt.Sprintf("L %s %s %s", a, hx(s.X), hx(s.Y))
	case svgtypes.PathSegmentHorizontalLineTo:
		return fmt.Sprintf("H %s %s", a, hx(s.X))
	case svgtypes.PathSegmentVerticalLineTo:
		return fmt.Sprintf("V %s %s", a, hx(s.Y))
	case svgtypes.PathSegmentCurveTo:
		return fmt.Sprintf("C %s %s %s %s %s %s %s", a, hx(s.X1), hx(s.Y1), hx(s.X2), hx(s.Y2), hx(s.X), hx(s.Y))
	case svgtypes.PathSegmentSmoothCurveTo:
		return fmt.Sprintf("S %s %s %s %s %s", a, hx(s.X2), hx(s.Y2), hx(s.X), hx(s.Y))
	case svgtypes.PathSegmentQuadratic:
		return fmt.Sprintf("Q %s %s %s %s %s", a, hx(s.X1), hx(s.Y1), hx(s.X), hx(s.Y))
	case svgtypes.PathSegmentSmoothQuadratic:
		return fmt.Sprintf("T %s %s %s", a, hx(s.X), hx(s.Y))
	case svgtypes.PathSegmentEllipticalArc:
		return fmt.Sprintf("A %s %s %s %s %t %t %s %s", a, hx(s.Rx), hx(s.Ry), hx(s.XAxisRotation),
			s.LargeArc, s.Sweep, hx(s.X), hx(s.Y))
	default:
		return "Z " + a
	}
}

func simpleSegment(s svgtypes.SimplePathSegment) string {
	switch s.Kind {
	case svgtypes.SimplePathSegmentMoveTo:
		return fmt.Sprintf("M %s %s", hx(s.X), hx(s.Y))
	case svgtypes.SimplePathSegmentLineTo:
		return fmt.Sprintf("L %s %s", hx(s.X), hx(s.Y))
	case svgtypes.SimplePathSegmentCurveTo:
		return fmt.Sprintf("C %s %s %s %s %s %s", hx(s.X1), hx(s.Y1), hx(s.X2), hx(s.Y2), hx(s.X), hx(s.Y))
	case svgtypes.SimplePathSegmentQuadratic:
		return fmt.Sprintf("Q %s %s %s %s", hx(s.X1), hx(s.Y1), hx(s.X), hx(s.Y))
	default:
		return "Z"
	}
}

func transformToken(t svgtypes.TransformListToken) string {
	switch t.Kind {
	case svgtypes.TransformListTokenMatrix:
		return fmt.Sprintf("matrix %s %s %s %s %s %s", hx(t.A), hx(t.B), hx(t.C), hx(t.D), hx(t.E), hx(t.F))
	case svgtypes.TransformListTokenTranslate:
		return fmt.Sprintf("translate %s %s", hx(t.Tx), hx(t.Ty))
	case svgtypes.TransformListTokenScale:
		return fmt.Sprintf("scale %s %s", hx(t.Sx), hx(t.Sy))
	case svgtypes.TransformListTokenRotate:
		return "rotate " + hx(t.Angle)
	case svgtypes.TransformListTokenSkewX:
		return "skewX " + hx(t.Angle)
	default:
		return "skewY " + hx(t.Angle)
	}
}

var filterNames = [...]string{
	svgtypes.FilterValueBrightness: "brightness",
	svgtypes.FilterValueContrast:   "contrast",
	svgtypes.FilterValueGrayscale:  "grayscale",
	svgtypes.FilterValueInvert:     "invert",
	svgtypes.FilterValueOpacity:    "opacity",
	svgtypes.FilterValueSepia:      "sepia",
	svgtypes.FilterValueSaturate:   "saturate",
	svgtypes.FilterValueURL:        "",
}

func filterValue(v svgtypes.FilterValue) string {
	switch v.Kind {
	case svgtypes.FilterValueBlur:
		return "blur " + length(v.Length)
	case svgtypes.FilterValueDropShadow:
		c := "-"
		if v.HasColor {
			c = color(v.Color)
		}
		return fmt.Sprintf("drop-shadow %s %s %s %s", c, length(v.Dx), length(v.Dy), length(v.StdDev))
	case svgtypes.FilterValueHueRotate:
		return "hue-rotate " + angle(v.Angle)
	case svgtypes.FilterValueURL:
		return "url " + qs(v.URL)
	default:
		return filterNames[v.Kind] + " " + hx(v.Number)
	}
}

func paintFallback(f svgtypes.PaintFallback) string {
	switch f.Kind {
	case svgtypes.PaintFallbackNone:
		return "none"
	case svgtypes.PaintFallbackCurrentColor:
		return "currentColor"
	default:
		return "color " + color(f.Color)
	}
}

func paint(p svgtypes.Paint) string {
	switch p.Kind {
	case svgtypes.PaintNone:
		return "none"
	case svgtypes.PaintInherit:
		return "inherit"
	case svgtypes.PaintCurrentColor:
		return "currentColor"
	case svgtypes.PaintColor:
		return "color " + color(p.Color)
	case svgtypes.PaintFuncIRI:
		fb := "-"
		if p.HasFallback {
			fb = paintFallback(p.Fallback)
		}
		return "funciri " + qs(p.FuncIRI) + " " + fb
	case svgtypes.PaintContextFill:
		return "context-fill"
	default:
		return "context-stroke"
	}
}

var aligns = [...]string{
	"none", "xMinYMin", "xMidYMin", "xMaxYMin", "xMinYMid",
	"xMidYMid", "xMaxYMid", "xMinYMax", "xMidYMax", "xMaxYMax",
}

var paintOrderKinds = [...]string{"fill", "stroke", "markers"}

var fontFamilies = [...]string{"serif", "sans-serif", "cursive", "fantasy", "monospace"}

func fontFamily(f svgtypes.FontFamily) string {
	if f.Kind == svgtypes.FontFamilyNamed {
		return "named " + qs(f.Name)
	}
	return fontFamilies[f.Kind]
}

var directionalPositions = [...]string{"top", "center", "bottom", "right", "left"}

// one writes the result of a parser that gives one value.
func one[T any](v T, err error, f func(T) string) []string {
	if err != nil {
		return []string{"err"}
	}
	return []string{f(v)}
}

// list writes the results of a parser that gives a list of values.
func list[T any](next func() (T, bool, error), f func(T) string) []string {
	var lines []string
	for {
		v, ok, err := next()
		switch {
		case !ok:
			return lines
		case err != nil:
			lines = append(lines, "err")
		default:
			lines = append(lines, f(v))
		}
	}
}

// svgtypesAnswer is the answer of okage to an svgtypes question, in the
// form of the oracle.
func svgtypesAnswer(q svgtypesQuestion) []byte {
	v := q.Value
	var lines []string
	switch q.Parser {
	case "number":
		n, err := svgtypes.ParseNumber(v)
		lines = one(n, err, func(n svgtypes.Number) string { return hx(float64(n)) })
	case "number_list":
		lines = list(svgtypes.NewNumberListParser(v).Next, hx)
	case "length":
		l, err := svgtypes.ParseLength(v)
		lines = one(l, err, length)
	case "length_list":
		lines = list(svgtypes.NewLengthListParser(v).Next, length)
	case "angle":
		a, err := svgtypes.ParseAngle(v)
		lines = one(a, err, angle)
	case "color":
		c, err := svgtypes.ParseColor(v)
		lines = one(c, err, color)
	case "paint":
		p, err := svgtypes.ParsePaint(v)
		lines = one(p, err, paint)
	case "filter":
		lines = list(svgtypes.NewFilterValueListParser(v).Next, filterValue)
	case "path":
		lines = list(svgtypes.NewPathParser(v).Next, pathSegment)
	case "simple_path":
		lines = list(svgtypes.NewSimplifyingPathParser(v).Next, simpleSegment)
	case "points":
		for x, y := range svgtypes.NewPointsParser(v).All() {
			lines = append(lines, hx(x)+" "+hx(y))
		}
	case "transform":
		ts, err := svgtypes.ParseTransform(v)
		lines = one(ts, err, func(t svgtypes.Transform) string {
			return strings.Join([]string{hx(t.A), hx(t.B), hx(t.C), hx(t.D), hx(t.E), hx(t.F)}, " ")
		})
	case "transform_list":
		lines = list(svgtypes.NewTransformListParser(v).Next, transformToken)
	case "transform_origin":
		o, err := svgtypes.ParseTransformOrigin(v)
		lines = one(o, err, func(o svgtypes.TransformOrigin) string {
			return length(o.XOffset) + " / " + length(o.YOffset) + " / " + length(o.ZOffset)
		})
	case "viewbox":
		b, err := svgtypes.ParseViewBox(v)
		lines = one(b, err, func(b svgtypes.ViewBox) string {
			return strings.Join([]string{hx(b.X), hx(b.Y), hx(b.W), hx(b.H)}, " ")
		})
	case "aspect_ratio":
		a, err := svgtypes.ParseAspectRatio(v)
		lines = one(a, err, func(a svgtypes.AspectRatio) string {
			return fmt.Sprintf("%t %s %t", a.Defer, aligns[a.Align], a.Slice)
		})
	case "paint_order":
		p := svgtypes.ParsePaintOrder(v)
		lines = []string{paintOrderKinds[p.Order[0]] + " " + paintOrderKinds[p.Order[1]] + " " + paintOrderKinds[p.Order[2]]}
	case "font_families":
		fs, err := svgtypes.ParseFontFamilies(v)
		lines = one(fs, err, func(fs []svgtypes.FontFamily) string {
			s := make([]string, len(fs))
			for i, f := range fs {
				s[i] = fontFamily(f)
			}
			return strings.Join(s, ", ")
		})
	case "font_shorthand":
		f, err := svgtypes.ParseFontShorthand(v)
		lines = one(f, err, func(f svgtypes.FontShorthand) string {
			return strings.Join([]string{
				optStr(f.FontStyle), optStr(f.FontVariant), optStr(f.FontWeight),
				optStr(f.FontStretch), qs(f.FontSize), qs(f.FontFamily),
			}, " ")
		})
	case "iri":
		i, err := svgtypes.ParseIRI(v)
		lines = one(i, err, func(i svgtypes.IRI) string { return qs(string(i)) })
	case "funciri":
		i, err := svgtypes.ParseFuncIRI(v)
		lines = one(i, err, func(i svgtypes.FuncIRI) string { return qs(string(i)) })
	case "enable_background":
		e, err := svgtypes.ParseEnableBackground(v)
		lines = one(e, err, func(e svgtypes.EnableBackground) string {
			switch e.Kind {
			case svgtypes.EnableBackgroundAccumulate:
				return "accumulate"
			case svgtypes.EnableBackgroundNew:
				return "new"
			default:
				return "new " + hx(e.X) + " " + hx(e.Y) + " " + hx(e.Width) + " " + hx(e.Height)
			}
		})
	case "directional_position":
		d, err := svgtypes.ParseDirectionalPosition(v)
		lines = one(d, err, func(d svgtypes.DirectionalPosition) string { return directionalPositions[d] })
	default:
		return nil
	}
	var s strings.Builder
	for _, line := range lines {
		s.WriteString(line)
		s.WriteByte('\n')
	}
	return []byte(s.String())
}

// usesLibm reports whether the answer to q needs sin, cos or tan: a path
// with an arc that the simplifying parser turns into curves, or a transform
// with a rotation or a skew. The math functions of okage are correctly
// rounded and glibc is not, so D26 allows 1 percent of such values to
// differ.
func usesLibm(q svgtypesQuestion) bool {
	switch q.Parser {
	case "simple_path":
		return strings.ContainsAny(q.Value, "Aa")
	case "transform":
		return strings.Contains(q.Value, "rotate") || strings.Contains(q.Value, "skew")
	}
	return false
}

// hugeArc reports whether q asks the simplifying parser for a path with an
// arc and a number that has an exponent or more than 8 digits. kurbo, and so
// okage, split such an arc into a number of curves that can be near 2^64, and
// neither ends in a useful time (BACKLOG.md).
func hugeArc(q svgtypesQuestion) bool {
	if q.Parser != "simple_path" || !strings.ContainsAny(q.Value, "Aa") {
		return false
	}
	if strings.ContainsAny(q.Value, "eE") {
		return true
	}
	digits := 0
	for _, c := range q.Value {
		switch {
		case c >= '0' && c <= '9':
			digits++
			if digits > 8 {
				return true
			}
		case c != '.':
			digits = 0
		}
	}
	return false
}

// TestSvgtypesMutated parses random mutations of the values of the SVG
// files of the tests of resvg: a character deleted, a character of the
// syntax inserted, or a digit changed. The flag -oracle.n sets the number of
// mutations of each parser.
func TestSvgtypesMutated(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}
	base := svgtypesQuestions(svgFiles(t))
	byParser := map[string][]svgtypesQuestion{}
	for _, q := range base {
		byParser[q.Parser] = append(byParser[q.Parser], q)
	}
	r := rand.New(rand.NewPCG(*flagSeed, 7))
	const syntax = "0123456789.-+eE,; ()#%aAzZmMlLhHvVcCsSqQtTxyrgbdpinu/"
	var questions []svgtypesQuestion
	parsers := make([]string, 0, len(byParser))
	for p := range byParser {
		parsers = append(parsers, p)
	}
	slices.Sort(parsers)
	for _, p := range parsers {
		qs := byParser[p]
		for range *flagN {
			q := qs[r.IntN(len(qs))]
			b := []byte(q.Value)
			for range 1 + r.IntN(3) {
				at := r.IntN(len(b) + 1)
				switch r.IntN(3) {
				case 0:
					if at < len(b) {
						b = append(b[:at:at], b[at+1:]...)
					}
				case 1:
					b = append(b[:at:at], append([]byte{syntax[r.IntN(len(syntax))]}, b[at:]...)...)
				default:
					if at < len(b) {
						b[at] = byte('0' + r.IntN(10))
					}
				}
			}
			mq := svgtypesQuestion{Parser: p, Value: string(b)}
			if utf8.Valid(b) && !hugeArc(mq) {
				questions = append(questions, mq)
			}
		}
	}
	answers := ask(t, questions)
	diffs, libm, libmDiffs := 0, 0, 0
	for i, q := range questions {
		got, err := safely(func() []byte { return svgtypesAnswer(q) })
		if err != nil {
			got = []byte("panic\n")
		}
		uses := usesLibm(q)
		if uses {
			libm++
		}
		if string(got) == string(answers[i]) {
			continue
		}
		if uses {
			libmDiffs++
			continue
		}
		diffs++
		if diffs <= 20 {
			t.Errorf("%s %q differs\nokage:\n%s\nsvgtypes:\n%s", q.Parser, q.Value, got, answers[i])
		}
	}
	if libm > 0 && float64(libmDiffs) > maxCubicDiffs*float64(libm) {
		t.Errorf("%d of %d values that need sin, cos or tan differ, more than D26 allows", libmDiffs, libm)
	}
	t.Logf("%d values, %d differ; %d need sin, cos or tan, and %d of them differ", len(questions), diffs, libm, libmDiffs)
}
