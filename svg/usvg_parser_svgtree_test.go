// Copyright 2021 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"slices"
	"strings"
	"testing"

	"github.com/xo/okage/internal/roxmltree"
	"github.com/xo/okage/internal/svgtypes"
)

// usvg src/parser/svgtree has no unit tests. These tests check the document,
// the attribute parsers and the trimming of text.

func mustParseTree(t *testing.T, text string) *document {
	t.Helper()
	opt := xmlTestOptions()
	xml, err := roxmltree.ParseWithOptions(text, opt)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseTree(xml, "", DefaultMaxDepth, newOptions())
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// xmlTestOptions returns the XML options of Tree::from_str.
func xmlTestOptions() roxmltree.ParsingOptions {
	opt := roxmltree.DefaultParsingOptions()
	opt.AllowDTD = true
	return opt
}

func TestDocument(t *testing.T) {
	doc := mustParseTree(t, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">
  <linearGradient id="a" xlink:href="#b"/>
  <linearGradient id="b" xlink:href="#c"/>
  <linearGradient id="c" xlink:href="#a"/>
  <g fill="red" opacity="0.5"><rect id="r" x="1.5" fill="inherit" opacity="inherit"/></g>
  <text xml:space="default">  Hello  <tspan> big </tspan> world </text>
</svg>`)

	root := doc.rootElement()
	if !root.hasTagName(EIdSvg) {
		t.Fatal("the root element is not svg")
	}

	a, ok := doc.elementByID("a")
	if !ok {
		t.Fatal("no element a")
	}
	var ids []string
	for n := range a.hrefIter() {
		ids = append(ids, n.elementID())
	}
	if !slices.Equal(ids, []string{"a", "b", "c"}) {
		t.Errorf("hrefIter = %v", ids)
	}

	r, _ := doc.elementByID("r")
	if v, ok := r.attributeF32(AIdX); !ok || v != 1.5 {
		t.Errorf("x = %v, %v", v, ok)
	}
	if v, ok := r.attributeStr(AIdFill); !ok || v != "red" {
		t.Errorf("fill = %q, %v", v, ok)
	}
	if v, ok := r.attributeStr(AIdOpacity); !ok || v != "0.5" {
		t.Errorf("opacity = %q, %v", v, ok)
	}
	if v, ok := nodeFindAttribute(r, AIdFill, fromValueStr); !ok || v != "red" {
		t.Errorf("find fill = %q, %v", v, ok)
	}

	var texts []string
	for n := range doc.descendants() {
		if n.isText() {
			texts = append(texts, n.text())
		}
	}
	if !slices.Equal(texts, []string{"Hello ", "big", " world"}) {
		t.Errorf("texts = %q", texts)
	}

	if s := doc.String(); !strings.HasPrefix(s, "Document [\n    Element {\n        tag_name: svg\n") {
		t.Errorf("String() = %s", s)
	}
}

func TestFromValue(t *testing.T) {
	var n svgNode
	check := func(name string, got, want any, ok bool) {
		t.Helper()
		if !ok {
			t.Errorf("%s: not parsed", name)
			return
		}
		if gs, ws := toString(got), toString(want); gs != ws {
			t.Errorf("%s = %v, want %v", name, gs, ws)
		}
	}
	u, ok := fromValueUnits(n, AIdX, "objectBoundingBox")
	check("units", u, unitsObjectBoundingBox, ok)
	a, ok := fromValueAngle(n, AIdX, "90deg")
	check("angle", a.ToDegrees(), 90.0, ok)
	eb, ok := fromValueEnableBackground(n, AIdX, "accumulate")
	check("enable-background", eb.Kind, svgtypes.EnableBackgroundAccumulate, ok)
	p, ok := fromValuePaint(n, AIdX, "url(#a) none")
	check("paint", p.FuncIRI, "a", ok)
	nl, ok := fromValueNumberList(n, AIdX, "1 2.5,3")
	check("numbers", nl, []float32{1, 2.5, 3}, ok)
	if _, ok := fromValueNumberList(n, AIdX, "1 x"); ok {
		t.Error("numbers: parsed an invalid list")
	}
	ll, ok := fromValueLengthList(n, AIdX, "1 2%")
	check("lengths", len(ll), 2, ok)
	if _, ok := fromValueLengthList(n, AIdX, "1 x"); ok {
		t.Error("lengths: parsed an invalid list")
	}
	sm, ok := fromValueSpreadMethod(n, AIdX, "reflect")
	check("spreadMethod", sm, SpreadMethodReflect, ok)
	tr, ok := fromValueTextRendering(n, AIdX, "geometricPrecision")
	check("text-rendering", tr, TextRenderingGeometricPrecision, ok)
	ir, ok := fromValueImageRendering(n, AIdX, "pixelated")
	check("image-rendering", ir, ImageRenderingPixelated, ok)
	if _, ok := fromValueBlendMode(n, AIdX, "plus-lighter"); ok {
		t.Error("plus-lighter is only for mix-blend-mode")
	}
	bm, ok := fromValueBlendMode(n, AIdMixBlendMode, "plus-lighter")
	check("mix-blend-mode", bm, BlendModePlusLighter, ok)
	o, ok := fromValueOpacity(n, AIdX, "150%")
	check("opacity", o.Get(), float32(1), ok)
	if _, ok := fromValueOpacity(n, AIdX, "1mm"); ok {
		t.Error("opacity: parsed a length")
	}
	ts, ok := fromValueTransform(n, AIdX, "scale(0)")
	check("transform", ts.IsIdentity(), true, ok)
}

func toString(v any) string {
	var b strings.Builder
	switch v := v.(type) {
	case []float32:
		for _, f := range v {
			b.WriteString(fmtF32(f) + " ")
		}
	default:
		b.WriteString(strings.TrimSpace(strings.ReplaceAll(fmtAny(v), "\n", " ")))
	}
	return b.String()
}

func fmtAny(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case float64:
		return fmtF32(float32(v))
	case float32:
		return fmtF32(v)
	case int:
		return fmtF32(float32(v))
	case bool:
		if v {
			return "true"
		}
		return "false"
	case interface{ String() string }:
		return v.String()
	}
	return "?"
}

func TestTrimText(t *testing.T) {
	for _, c := range []struct {
		in    string
		space xmlSpace
		want  string
	}{
		{"a \t\n  b", xmlSpaceDefault, "a b"},
		{"a \t\n  b", xmlSpacePreserve, "a     b"},
		{"é\ré", xmlSpaceDefault, "é é"},
	} {
		if got := trimText(c.in, c.space); got != c.want {
			t.Errorf("trimText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
