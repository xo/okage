// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package font

// okage: this file is not in go-text. It tests the API of the table MATH of
// D78. The test module compares each value of the table with fontTools.

import (
	"bytes"
	"encoding/binary"
	"os"
	"slices"
	"testing"

	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// stixTwoMath is the font of texmath/stixtwomath, which has a MATH table.
const stixTwoMath = "../../texmath/stixtwomath/stixtwomath.otf"

// loadFace parses the font file at name.
func loadFace(t testing.TB, data []byte) *Face {
	t.Helper()
	face, err := ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return face
}

// readFile reads the file at name.
func readFile(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// glyph returns the glyph of r in face.
func glyph(t *testing.T, face *Face, r rune) GID {
	t.Helper()
	g, ok := face.NominalGlyph(r)
	if !ok {
		t.Fatalf("the font has no glyph for %q", r)
	}
	return g
}

// TestMathStixTwoMath reads values of the table MATH of STIX Two Math. The
// values come from fontTools.
func TestMathStixTwoMath(t *testing.T) {
	face := loadFace(t, readFile(t, stixTwoMath))
	if !face.HasMath() {
		t.Fatal("STIX Two Math has no MATH table")
	}
	for _, test := range []struct {
		c    MathConstant
		want float32
	}{
		{ScriptPercentScaleDown, 70},
		{ScriptScriptPercentScaleDown, 55},
		{DelimitedSubFormulaMinHeight, 1325},
		{DisplayOperatorMinHeight, 1800},
		{MathLeading, 150},
		{AxisHeight, 258},
		{AccentBaseHeight, 480},
		{FlattenedAccentBaseHeight, 656},
		{MathConstantCount, 0},
	} {
		if got := face.MathConstant(test.c); got != test.want {
			t.Errorf("the constant %d is %v, want %v", test.c, got, test.want)
		}
	}
	if got := face.MathMinConnectorOverlap(); got != 100 {
		t.Errorf("the minimum connector overlap is %v, want 100", got)
	}

	// The parenthesis has size variants and an assembly, from the smallest
	// variant up.
	paren := glyph(t, face, '(')
	variants := face.MathGlyphVariants(paren, true)
	if len(variants) < 2 || variants[0].Glyph != paren || !slices.IsSortedFunc(variants, func(a, b MathGlyphVariant) int {
		return int(a.Advance - b.Advance)
	}) {
		t.Errorf("the variants of ( are %v", variants)
	}
	asm, ok := face.MathGlyphAssembly(paren, true)
	if !ok || len(asm.Parts) < 3 || !slices.ContainsFunc(asm.Parts, func(p MathGlyphPart) bool { return p.Extender }) {
		t.Errorf("the assembly of ( is %+v, %v", asm, ok)
	}
	if _, ok := face.MathGlyphAssembly(paren, false); ok {
		t.Error("( has a horizontal assembly")
	}
	if v := face.MathGlyphVariants(glyph(t, face, 'x'), true); v != nil {
		t.Errorf("x has the variants %v", v)
	}

	// The italic f leans to the right, so it has an italic correction, a
	// top accent attachment and no variants.
	f := glyph(t, face, '\U0001D453')
	if got := face.MathItalicsCorrection(f); got <= 0 {
		t.Errorf("the italic correction of the italic f is %v", got)
	}
	if _, ok := face.MathTopAccentAttachment(f); !ok {
		t.Error("the italic f has no top accent attachment")
	}
	if !face.IsMathExtendedShape(glyph(t, face, '∑')) || face.IsMathExtendedShape(f) {
		t.Error("the extended shapes are wrong")
	}
}

// mathFace returns a face whose font holds only the table MATH m.
func mathFace(m *tables.MATH) *Face {
	return &Face{Font: &Font{MATH: m}}
}

// TestMathKern checks the choice of a kern value at the boundaries of the
// correction heights, as HarfBuzz chooses it.
func TestMathKern(t *testing.T) {
	rec := func(v int16) tables.MathValueRecord { return tables.MathValueRecord{Value: v} }
	m := &tables.MATH{}
	m.MathGlyphInfo.MathKernInfo = tables.MathKernInfo{
		Coverage: tables.Coverage1{Glyphs: []tables.GlyphID{5}},
		Records: []tables.MathKernInfoRecord{{
			tables.MathKern{},
			tables.MathKern{
				CorrectionHeights: []tables.MathValueRecord{rec(10), rec(20)},
				KernValues:        []tables.MathValueRecord{rec(1), rec(2), rec(3)},
			},
		}},
	}
	face := mathFace(m)
	for _, test := range []struct {
		height float32
		want   float32
	}{
		{-100, 1}, {9.5, 1}, {10, 2}, {15, 2}, {20, 3}, {1000, 3},
	} {
		if got := face.MathKern(5, MathKernTopLeft, test.height); got != test.want {
			t.Errorf("the kern at %v is %v, want %v", test.height, got, test.want)
		}
	}
	if got := face.MathKern(5, MathKernTopRight, 15); got != 0 {
		t.Errorf("a corner without kerning gives %v", got)
	}
	if got := face.MathKern(6, MathKernTopLeft, 15); got != 0 {
		t.Errorf("a glyph without kerning gives %v", got)
	}
	if got := face.MathKern(5, MathKernCorner(4), 15); got != 0 {
		t.Errorf("a corner out of range gives %v", got)
	}
}

// TestMathVariation checks that a value with a VariationIndex table gets
// its delta from the item variation store of GDEF, at the coordinates of
// the face.
func TestMathVariation(t *testing.T) {
	m := &tables.MATH{}
	m.MathConstants.Records[AxisHeight-MathLeading] = tables.MathValueRecord{
		Value:        250,
		DeviceOffset: 1,
		Device:       tables.MathDevice{First: 0, Second: 1, Format: 0x8000},
	}
	face := mathFace(m)
	face.GDEF.ItemVarStore = tables.ItemVarStore{
		VariationRegionList: tables.VariationRegionList{VariationRegions: []tables.VariationRegion{{
			RegionAxes: []tables.RegionAxisCoordinates{{StartCoord: 0, PeakCoord: 1 << 14, EndCoord: 1 << 14}},
		}}},
		ItemVariationDatas: []tables.ItemVariationData{{
			RegionIndexes: []uint16{0},
			DeltaSets:     [][]int32{{0}, {40}},
		}},
	}
	if got := face.MathConstant(AxisHeight); got != 250 {
		t.Errorf("AxisHeight without coordinates is %v, want 250", got)
	}
	face.SetCoords([]tables.Coord{1 << 13})
	if got := face.MathConstant(AxisHeight); got != 270 {
		t.Errorf("AxisHeight at half the axis is %v, want 270", got)
	}
}

// TestMathAbsent checks that a font without a MATH table gives the zero
// value of each method.
func TestMathAbsent(t *testing.T) {
	face := loadFace(t, readFile(t, "testdata/Roboto-Regular.ttf"))
	checkNoMath(t, face)
}

// checkNoMath checks that face has no MATH table.
func checkNoMath(t *testing.T, face *Face) {
	t.Helper()
	if face.HasMath() {
		t.Fatal("the font has a MATH table")
	}
	if face.MathConstant(AxisHeight) != 0 || face.MathItalicsCorrection(1) != 0 ||
		face.MathKern(1, MathKernTopRight, 0) != 0 || face.MathMinConnectorOverlap() != 0 ||
		face.IsMathExtendedShape(1) || face.MathGlyphVariants(1, true) != nil {
		t.Error("a method gives a value")
	}
	if _, ok := face.MathTopAccentAttachment(1); ok {
		t.Error("MathTopAccentAttachment gives a value")
	}
	if _, ok := face.MathGlyphAssembly(1, true); ok {
		t.Error("MathGlyphAssembly gives a value")
	}
}

// withMATH returns the font data with the table MATH replaced by raw. It
// keeps the tables that NewFont needs, and the outlines.
func withMATH(t testing.TB, data, raw []byte) []byte {
	t.Helper()
	ld, err := ot.NewLoader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var out []ot.Table
	for _, tag := range ld.Tables() {
		if tag == ot.MustNewTag("MATH") {
			continue
		}
		content, err := ld.RawTable(tag)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, ot.Table{Tag: tag, Content: content})
	}
	out = append(out, ot.Table{Tag: ot.MustNewTag("MATH"), Content: raw})
	return ot.WriteOpentype(out, ld.Type)
}

// TestMathMalformed checks that NewFont leaves a malformed MATH table out,
// and still loads the font.
func TestMathMalformed(t *testing.T) {
	data := readFile(t, stixTwoMath)
	ld, err := ot.NewLoader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ld.RawTable(ot.MustNewTag("MATH"))
	if err != nil {
		t.Fatal(err)
	}
	bad := slices.Clone(raw)
	binary.BigEndian.PutUint16(bad, 2) // version 2.0
	face := loadFace(t, withMATH(t, data, bad))
	checkNoMath(t, face)
	if _, ok := face.NominalGlyph('x'); !ok {
		t.Error("the font lost its cmap")
	}
	if !loadFace(t, withMATH(t, data, raw)).HasMath() {
		t.Error("the font with its own MATH table has none")
	}
}

// FuzzMATHFace loads a font whose MATH table the fuzzer changes, and calls
// each method of the API with a random glyph. It must never panic or hang.
func FuzzMATHFace(f *testing.F) {
	data := readFile(f, stixTwoMath)
	ld, err := ot.NewLoader(bytes.NewReader(data))
	if err != nil {
		f.Fatal(err)
	}
	// The base font holds the tables that NewFont needs, without the
	// outlines, so that a run is fast.
	var base []ot.Table
	for _, name := range []string{"cmap", "head", "maxp", "hhea", "hmtx"} {
		content, err := ld.RawTable(ot.MustNewTag(name))
		if err != nil {
			f.Fatal(err)
		}
		base = append(base, ot.Table{Tag: ot.MustNewTag(name), Content: content})
	}
	raw, err := ld.RawTable(ot.MustNewTag("MATH"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(raw, uint16(3), float32(500), true)
	f.Add(raw[:300], uint16(1000), float32(-1), false)
	f.Fuzz(func(t *testing.T, raw []byte, g uint16, height float32, vertical bool) {
		tabs := append(slices.Clone(base), ot.Table{Tag: ot.MustNewTag("MATH"), Content: raw})
		face, err := ParseTTF(bytes.NewReader(ot.WriteOpentype(tabs, ot.TrueType)))
		if err != nil {
			t.Fatal(err)
		}
		gid := GID(g)
		for c := range MathConstantCount + 1 {
			face.MathConstant(c)
		}
		face.HasMath()
		face.MathItalicsCorrection(gid)
		face.MathTopAccentAttachment(gid)
		face.IsMathExtendedShape(gid)
		for corner := range MathKernBottomLeft + 2 {
			face.MathKern(gid, corner, height)
		}
		face.MathMinConnectorOverlap()
		face.MathGlyphVariants(gid, vertical)
		face.MathGlyphAssembly(gid, vertical)
	})
}
