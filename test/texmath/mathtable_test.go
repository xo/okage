package texmath_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// The fonts with a MATH table whose values the test compares with
// fontTools. Each one is in the repository with its license.
var mathFonts = []string{
	"../../texmath/stixtwomath/stixtwomath.otf",
	"../../pdf/testdata/fonts/NewCMMath-Regular.otf",
	"../../typesetting/testdata/opentype/common/DejaVuSans.ttf",
}

// ftRecord is a MathValueRecord as mathtable.py writes it: the value, and
// the start size, the end size and the delta format of its device table.
type ftRecord [4]int

// ftKern is a MathKern as mathtable.py writes it.
type ftKern struct {
	Heights []ftRecord `json:"heights"`
	Kerns   []ftRecord `json:"kerns"`
}

// ftConstruction is a MathGlyphConstruction as mathtable.py writes it.
type ftConstruction struct {
	Variants [][2]int `json:"variants"`
	Assembly *struct {
		Italics ftRecord `json:"italics"`
		Parts   [][5]int `json:"parts"`
	} `json:"assembly"`
}

// ftMath is the table MATH as mathtable.py writes it. A map is keyed by the
// glyph ID, in decimal.
type ftMath struct {
	Math      bool                  `json:"math"`
	Glyphs    int                   `json:"glyphs"`
	Version   int                   `json:"version"`
	Constants []ftRecord            `json:"constants"`
	Italics   map[string]ftRecord   `json:"italics"`
	Accents   map[string]ftRecord   `json:"accents"`
	Extended  []int                 `json:"extended"`
	Kerns     map[string][4]*ftKern `json:"kerns"`
	Variants  *struct {
		Overlap    int                       `json:"overlap"`
		Vertical   map[string]ftConstruction `json:"vertical"`
		Horizontal map[string]ftConstruction `json:"horizontal"`
	} `json:"variants"`
}

// readFontTools runs mathtable.py on the font at name. It skips the test
// when python3 or fontTools is missing.
func readFontTools(t *testing.T, name string) ftMath {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is missing")
	}
	if err := exec.CommandContext(t.Context(), python, "-I", "-c", "import fontTools").Run(); err != nil {
		t.Skip("fontTools is missing")
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(t.Context(), python, "-I", "mathtable.py", name)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running mathtable.py on %s: %v\n%s", name, err, stderr.Bytes())
	}
	var m ftMath
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("reading the output of mathtable.py: %v", err)
	}
	return m
}

// record returns r in the form of mathtable.py.
func record(r tables.MathValueRecord) ftRecord {
	return ftRecord{int(r.Value), int(r.Device.First), int(r.Device.Second), int(r.Device.Format)}
}

// TestMATHMatchesFontTools holds the gate of phase 21 of D78. Each value of
// the table MATH of each font is the value that fontTools reads, both in
// the tables of typesetting and through the API of font.Face. The faces
// have no variations, so a value of the API is the value of the table.
func TestMATHMatchesFontTools(t *testing.T) {
	t.Parallel()
	for _, name := range mathFonts {
		t.Run(filepath.Base(name), func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(name)
			if os.IsNotExist(err) {
				t.Skipf("%s is missing", name)
			}
			if err != nil {
				t.Fatal(err)
			}
			want := readFontTools(t, name)
			if !want.Math {
				t.Fatalf("fontTools finds no MATH table in %s", name)
			}
			face, err := font.ParseTTF(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if !face.HasMath() {
				t.Fatal("typesetting finds no MATH table")
			}
			c := comparison{t: t, face: face, m: face.MATH}
			c.version(want.Version)
			c.constants(want.Constants)
			c.glyphInfo(want)
			c.variants(want)
			t.Logf("%d values compared", c.n)
		})
	}
}

// comparison compares one font with the output of mathtable.py, and counts
// the values that it compares.
type comparison struct {
	t    *testing.T
	face *font.Face
	m    *tables.MATH
	n    int
}

// equal reports an error when got is not want.
func equal[T comparable](c *comparison, what string, got, want T) {
	c.t.Helper()
	c.n++
	if got != want {
		c.t.Errorf("%s is %v, want %v", what, got, want)
	}
}

// glyphs returns the glyph IDs of the keys of m.
func glyphs[V any](t *testing.T, m map[string]V) map[font.GID]V {
	t.Helper()
	out := make(map[font.GID]V, len(m))
	for k, v := range m {
		g, err := strconv.ParseUint(k, 10, 16)
		if err != nil {
			t.Fatal(err)
		}
		out[font.GID(g)] = v
	}
	return out
}

func (c *comparison) version(want int) {
	equal(c, "the version", int(c.m.MajorVersion)<<16|int(c.m.MinorVersion), want)
}

func (c *comparison) constants(want []ftRecord) {
	if len(want) != int(font.MathConstantCount) {
		c.t.Fatalf("fontTools gives %d constants", len(want))
	}
	mc := c.m.MathConstants
	equal(c, "ScriptPercentScaleDown", int(mc.ScriptPercentScaleDown), want[0][0])
	equal(c, "ScriptScriptPercentScaleDown", int(mc.ScriptScriptPercentScaleDown), want[1][0])
	equal(c, "DelimitedSubFormulaMinHeight", int(mc.DelimitedSubFormulaMinHeight), want[2][0])
	equal(c, "DisplayOperatorMinHeight", int(mc.DisplayOperatorMinHeight), want[3][0])
	for i, r := range mc.Records {
		equal(c, fmt.Sprintf("the constant %d", i+4), record(r), want[i+4])
	}
	equal(c, "RadicalDegreeBottomRaisePercent", int(mc.RadicalDegreeBottomRaisePercent), want[55][0])
	for k := range font.MathConstantCount {
		equal(c, fmt.Sprintf("MathConstant(%d)", k), c.face.MathConstant(k), float32(want[k][0]))
	}
}

func (c *comparison) glyphInfo(want ftMath) {
	gi := c.m.MathGlyphInfo
	n := want.Glyphs + 16 // and some glyphs that the font does not have

	italics := glyphs(c.t, want.Italics)
	accents := glyphs(c.t, want.Accents)
	kerns := glyphs(c.t, want.Kerns)
	extended := map[font.GID]bool{}
	for _, g := range want.Extended {
		extended[font.GID(g)] = true
	}
	equal(c, "the count of italic corrections", len(gi.MathItalicsCorrectionInfo.Values), len(italics))
	equal(c, "the count of top accent attachments", len(gi.MathTopAccentAttachment.Values), len(accents))
	equal(c, "the count of kerned glyphs", len(gi.MathKernInfo.Records), len(kerns))
	if gi.ExtendedShapeCoverage != nil || want.Extended != nil {
		equal(c, "the count of extended shapes", gi.ExtendedShapeCoverage.Len(), len(want.Extended))
	}

	for g := range font.GID(n) {
		r, ok := gi.MathItalicsCorrectionInfo.Value(tables.GlyphID(g))
		w, wok := italics[g]
		equal(c, fmt.Sprintf("the italic correction of the glyph %d", g), record(r), w)
		equal(c, fmt.Sprintf("the coverage of the italic correction of the glyph %d", g), ok, wok)
		equal(c, fmt.Sprintf("MathItalicsCorrection(%d)", g), c.face.MathItalicsCorrection(g), float32(w[0]))

		r, ok = gi.MathTopAccentAttachment.Value(tables.GlyphID(g))
		w, wok = accents[g]
		equal(c, fmt.Sprintf("the top accent attachment of the glyph %d", g), record(r), w)
		equal(c, fmt.Sprintf("the coverage of the top accent attachment of the glyph %d", g), ok, wok)
		v, ok := c.face.MathTopAccentAttachment(g)
		equal(c, fmt.Sprintf("MathTopAccentAttachment(%d)", g), v, float32(w[0]))
		equal(c, fmt.Sprintf("the result of MathTopAccentAttachment(%d)", g), ok, wok)

		equal(c, fmt.Sprintf("IsMathExtendedShape(%d)", g), c.face.IsMathExtendedShape(g), extended[g])

		rec, ok := gi.MathKernInfo.Record(tables.GlyphID(g))
		wk, wok := kerns[g]
		equal(c, fmt.Sprintf("the coverage of the kerning of the glyph %d", g), ok, wok)
		for corner := range 4 {
			c.kern(g, font.MathKernCorner(corner), rec[corner], wk[corner])
		}
	}
}

// kern compares the kerning of one corner of g.
func (c *comparison) kern(g font.GID, corner font.MathKernCorner, got tables.MathKern, want *ftKern) {
	what := fmt.Sprintf("the kerning of the corner %d of the glyph %d", corner, g)
	if want == nil {
		equal(c, what+": its count", len(got.KernValues), 0)
		equal(c, what+": MathKern", c.face.MathKern(g, corner, 0), 0)
		return
	}
	equal(c, what+": its count of heights", len(got.CorrectionHeights), len(want.Heights))
	equal(c, what+": its count of kerns", len(got.KernValues), len(want.Kerns))
	if len(got.CorrectionHeights) != len(want.Heights) || len(got.KernValues) != len(want.Kerns) {
		return
	}
	for i, h := range got.CorrectionHeights {
		equal(c, fmt.Sprintf("%s: the height %d", what, i), record(h), want.Heights[i])
	}
	for i, k := range got.KernValues {
		equal(c, fmt.Sprintf("%s: the kern %d", what, i), record(k), want.Kerns[i])
	}
	// Below the first height, the kern is the first one. At each height
	// and above it, the kern is the next one.
	if len(want.Heights) == 0 {
		equal(c, what+": MathKern", c.face.MathKern(g, corner, 0), float32(want.Kerns[0][0]))
		return
	}
	equal(c, what+": MathKern below the heights", c.face.MathKern(g, corner, float32(want.Heights[0][0]-1)), float32(want.Kerns[0][0]))
	for i, h := range want.Heights {
		equal(c, fmt.Sprintf("%s: MathKern at the height %d", what, i), c.face.MathKern(g, corner, float32(h[0])), float32(want.Kerns[i+1][0]))
	}
}

func (c *comparison) variants(want ftMath) {
	if want.Variants == nil {
		c.t.Fatal("fontTools finds no MathVariants")
	}
	equal(c, "MinConnectorOverlap", int(c.m.MathVariants.MinConnectorOverlap), want.Variants.Overlap)
	equal(c, "MathMinConnectorOverlap", c.face.MathMinConnectorOverlap(), float32(want.Variants.Overlap))
	for _, dir := range []struct {
		vertical bool
		want     map[string]ftConstruction
		count    int
	}{
		{true, want.Variants.Vertical, len(c.m.MathVariants.VertGlyphConstructions)},
		{false, want.Variants.Horizontal, len(c.m.MathVariants.HorizGlyphConstructions)},
	} {
		constructions := glyphs(c.t, dir.want)
		equal(c, fmt.Sprintf("the count of the constructions, vertical %v", dir.vertical), dir.count, len(constructions))
		for g := range font.GID(want.Glyphs + 16) {
			c.construction(g, dir.vertical, constructions[g])
		}
	}
}

// construction compares the construction of g in one direction.
func (c *comparison) construction(g font.GID, vertical bool, want ftConstruction) {
	what := fmt.Sprintf("the construction of the glyph %d, vertical %v", g, vertical)
	got, _ := c.m.MathVariants.Construction(tables.GlyphID(g), vertical)
	variants := c.face.MathGlyphVariants(g, vertical)
	equal(c, what+": its count of variants", len(got.Variants), len(want.Variants))
	equal(c, what+": the count of MathGlyphVariants", len(variants), len(want.Variants))
	if len(got.Variants) == len(want.Variants) && len(variants) == len(want.Variants) {
		for i, v := range want.Variants {
			equal(c, fmt.Sprintf("%s: the variant %d", what, i), [2]int{int(got.Variants[i].VariantGlyph), int(got.Variants[i].AdvanceMeasurement)}, v)
			equal(c, fmt.Sprintf("%s: MathGlyphVariants %d", what, i), variants[i], font.MathGlyphVariant{Glyph: font.GID(v[0]), Advance: float32(v[1])})
		}
	}
	asm, ok := c.face.MathGlyphAssembly(g, vertical)
	equal(c, what+": the presence of its assembly", got.GlyphAssembly != nil, want.Assembly != nil)
	equal(c, what+": the result of MathGlyphAssembly", ok, want.Assembly != nil)
	if want.Assembly == nil || got.GlyphAssembly == nil {
		return
	}
	equal(c, what+": the italic correction of its assembly", record(got.GlyphAssembly.ItalicsCorrection), want.Assembly.Italics)
	equal(c, what+": the italic correction of MathGlyphAssembly", asm.ItalicsCorrection, float32(want.Assembly.Italics[0]))
	equal(c, what+": its count of parts", len(got.GlyphAssembly.Parts), len(want.Assembly.Parts))
	if len(got.GlyphAssembly.Parts) != len(want.Assembly.Parts) || len(asm.Parts) != len(want.Assembly.Parts) {
		return
	}
	for i, p := range got.GlyphAssembly.Parts {
		w := want.Assembly.Parts[i]
		equal(c, fmt.Sprintf("%s: the part %d", what, i),
			[5]int{int(p.GlyphID), int(p.StartConnectorLength), int(p.EndConnectorLength), int(p.FullAdvance), int(p.PartFlags)}, w)
		equal(c, fmt.Sprintf("%s: the part %d of MathGlyphAssembly", what, i), asm.Parts[i], font.MathGlyphPart{
			Glyph:                font.GID(w[0]),
			StartConnectorLength: float32(w[1]),
			EndConnectorLength:   float32(w[2]),
			FullAdvance:          float32(w[3]),
			Extender:             w[4]&1 != 0,
		})
	}
}
