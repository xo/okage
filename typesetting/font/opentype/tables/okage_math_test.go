// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

// okage: this file is not in go-text. It tests the reader of the table MATH
// of D78. The test module compares each value of the table with fontTools.

import (
	"encoding/binary"
	"errors"
	"os"
	"testing"
)

// stixTwoMath is the font of texmath/stixtwomath, which has a MATH table.
const stixTwoMath = "../../../../texmath/stixtwomath/stixtwomath.otf"

// rawTable returns the table tag of the font file at name. It reads the
// table directory of the file, so that the test needs no loader.
func rawTable(t testing.TB, name, tag string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	n := int(binary.BigEndian.Uint16(data[4:]))
	for i := range n {
		rec := data[12+16*i:]
		if string(rec[:4]) == tag {
			off, size := binary.BigEndian.Uint32(rec[8:]), binary.BigEndian.Uint32(rec[12:])
			return data[off : off+size]
		}
	}
	t.Fatalf("%s has no table %s", name, tag)
	return nil
}

// TestParseMATHStixTwoMath reads the table MATH of STIX Two Math, and checks
// the sizes of its subtables and some values. The values come from
// fontTools.
func TestParseMATHStixTwoMath(t *testing.T) {
	m, n, err := ParseMATH(rawTable(t, stixTwoMath, "MATH"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 10 || m.MajorVersion != 1 || m.MinorVersion != 0 {
		t.Errorf("the header is %d bytes, version %d.%d, want 10 bytes, version 1.0", n, m.MajorVersion, m.MinorVersion)
	}
	mc := m.MathConstants
	if mc.ScriptPercentScaleDown != 70 || mc.ScriptScriptPercentScaleDown != 55 ||
		mc.DelimitedSubFormulaMinHeight != 1325 || mc.DisplayOperatorMinHeight != 1800 {
		t.Errorf("the first constants are %d, %d, %d, %d, want 70, 55, 1325, 1800", mc.ScriptPercentScaleDown,
			mc.ScriptScriptPercentScaleDown, mc.DelimitedSubFormulaMinHeight, mc.DisplayOperatorMinHeight)
	}
	if got := mc.Records[1].Value; got != 258 {
		t.Errorf("AxisHeight is %d, want 258", got)
	}
	gi := m.MathGlyphInfo
	for _, test := range []struct {
		name      string
		got, want int
	}{
		{"italic corrections", len(gi.MathItalicsCorrectionInfo.Values), 889},
		{"top accent attachments", len(gi.MathTopAccentAttachment.Values), 2652},
		{"extended shapes", gi.ExtendedShapeCoverage.Len(), 494},
		{"kerned glyphs", len(gi.MathKernInfo.Records), 233},
		{"minimum connector overlap", int(m.MathVariants.MinConnectorOverlap), 100},
		{"vertical constructions", len(m.MathVariants.VertGlyphConstructions), 118},
		{"horizontal constructions", len(m.MathVariants.HorizGlyphConstructions), 47},
	} {
		if test.got != test.want {
			t.Errorf("the table has %d %s, want %d", test.got, test.name, test.want)
		}
	}
	for i, c := range m.MathVariants.VertGlyphConstructions {
		if len(c.Variants) == 0 && c.GlyphAssembly == nil {
			t.Errorf("the vertical construction %d is empty", i)
		}
	}
}

// TestParseMATHTruncated parses prefixes of the table of STIX Two Math. A
// prefix gives an error or a table, and never a panic.
func TestParseMATHTruncated(t *testing.T) {
	raw := rawTable(t, stixTwoMath, "MATH")
	step := 1
	for n := 0; n < len(raw); n += step {
		if n >= 512 {
			step = 211
		}
		if _, _, err := ParseMATH(raw[:n]); err == nil && n < 214 {
			t.Errorf("a prefix of %d bytes parses, but MathConstants needs 214 bytes", n)
		}
	}
}

// mathHeader returns the header of a table MATH whose MathGlyphInfo is at
// glyphInfo, with no constants and no variants.
func mathHeader(glyphInfo uint16) []byte {
	return binary.BigEndian.AppendUint16([]byte{0, 1, 0, 0, 0, 0}, glyphInfo)[:8:8]
}

// kernTable returns a table MATH with count kerned glyphs. The four corners
// of record i point to the MathKern at offset(i, corner) from the start of
// MathKernInfo, in a block of size bytes of value fill that follows the
// records.
func kernTable(count, size int, fill byte, offset func(i, corner int) int) []byte {
	b := append(mathHeader(10), 0, 0) // no MathVariants
	// MathGlyphInfo at 10: only MathKernInfo, at 8 bytes from its start.
	b = append(b, 0, 0, 0, 0, 0, 0, 0, 8)
	// MathKernInfo at 18: the coverage, the count and the records.
	cov := 4 + 8*count
	b = binary.BigEndian.AppendUint16(b, uint16(cov))
	b = binary.BigEndian.AppendUint16(b, uint16(count))
	for i := range count {
		for c := range 4 {
			b = binary.BigEndian.AppendUint16(b, uint16(cov+4+offset(i, c)))
		}
	}
	b = append(b, 0, 1, 0, 0) // a coverage of format 1 with no glyph
	for range size {
		b = append(b, fill)
	}
	return b
}

// TestParseMATHSharedKern reads a table whose kern records all point to one
// MathKern. The parser reads it once, so the table parses.
func TestParseMATHSharedKern(t *testing.T) {
	// The MathKern holds 1,000 heights and 1,001 kern values: 8,006 bytes.
	raw := kernTable(2000, 8006, 0, func(int, int) int { return 0 })
	binary.BigEndian.PutUint16(raw[len(raw)-8006:], 1000)
	m, _, err := ParseMATH(raw)
	if err != nil {
		t.Fatal(err)
	}
	r := m.MathGlyphInfo.MathKernInfo.Records
	if len(r) != 2000 || len(r[1999][3].CorrectionHeights) != 1000 || len(r[1999][3].KernValues) != 1001 {
		t.Errorf("the table has %d records, the last one with %d heights", len(r), len(r[len(r)-1][3].CorrectionHeights))
	}
}

// TestParseMATHOverlappingKerns reads a table whose kern records point to
// many MathKern tables that overlap. Each one is large, so the parser can
// decode much more than the table holds. It must stop with an error.
func TestParseMATHOverlappingKerns(t *testing.T) {
	// Each MathKern at an even offset of the block claims 0x1010 heights,
	// which is 32,900 bytes of records.
	raw := kernTable(1000, 40000, 0x10, func(i, c int) int { return 2 * ((4*i + c) % 3000) })
	_, _, err := ParseMATH(raw)
	if !errors.Is(err, errMathBudget) {
		t.Errorf("the error is %v, want %v", err, errMathBudget)
	}
}

// TestParseMATHErrors checks that a malformed table gives an error.
func TestParseMATHErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  []byte
	}{
		{"empty", nil},
		{"version 2", []byte{0, 2, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"constants out of range", []byte{0, 1, 0, 0, 0, 10, 0, 0, 0, 0}},
		{"glyph info out of range", []byte{0, 1, 0, 0, 0, 0, 0, 10, 0, 0}},
		{"variants out of range", []byte{0, 1, 0, 0, 0, 0, 0, 0, 0, 200}},
		{"kern count out of range", kernTable(1, 2, 0xFF, func(int, int) int { return 0 })},
		{"kern offset out of range", kernTable(1, 2, 0, func(int, int) int { return 100 })},
	} {
		if _, _, err := ParseMATH(test.raw); err == nil {
			t.Errorf("%s: the table parses", test.name)
		}
	}
}

// TestMathDevice reads a value record with a VariationIndex table and one
// with a device table for sizes in pixels.
func TestMathDevice(t *testing.T) {
	raw := mathHeader(10)
	raw = append(raw, 0, 0) // no MathVariants
	// MathGlyphInfo at 10: MathItalicsCorrectionInfo at 8 from its start.
	raw = append(raw, 0, 8, 0, 0, 0, 0, 0, 0)
	// MathItalicsCorrectionInfo at 18: coverage at 12, two records.
	raw = append(raw, 0, 12, 0, 2)
	raw = append(raw, 0, 50, 0, 20)             // 50, device at 20
	raw = append(raw, 0xFF, 0xF6, 0, 26)        // -10, device at 26
	raw = append(raw, 0, 1, 0, 2, 0, 3, 0, 4)   // coverage of glyphs 3 and 4
	raw = append(raw, 0, 7, 0, 9, 0x80, 0)      // VariationIndex 7, 9
	raw = append(raw, 0, 12, 0, 13, 0, 1, 0, 0) // sizes 12 to 13, format 1
	m, _, err := ParseMATH(raw)
	if err != nil {
		t.Fatal(err)
	}
	info := m.MathGlyphInfo.MathItalicsCorrectionInfo
	a, okA := info.Value(3)
	b, okB := info.Value(4)
	if _, ok := info.Value(5); ok || !okA || !okB {
		t.Fatalf("the coverage gives %v, %v and a record for the glyph 5", okA, okB)
	}
	if index, ok := a.Device.VariationIndex(); a.Value != 50 || !ok || index != (VariationStoreIndex{7, 9}) {
		t.Errorf("the first record is %+v", a)
	}
	if _, ok := b.Device.VariationIndex(); b.Value != -10 || ok || b.Device != (MathDevice{12, 13, 1}) {
		t.Errorf("the second record is %+v", b)
	}
}

// FuzzParseMATH gives random bytes to ParseMATH, with the table of STIX Two
// Math as its seed. It must never panic.
func FuzzParseMATH(f *testing.F) {
	f.Add(rawTable(f, stixTwoMath, "MATH"))
	f.Add(kernTable(4, 64, 1, func(i, c int) int { return 2 * (i + c) }))
	f.Fuzz(func(t *testing.T, raw []byte) {
		m, _, err := ParseMATH(raw)
		if err != nil {
			return
		}
		for g := range GlyphID(64) {
			m.MathGlyphInfo.MathItalicsCorrectionInfo.Value(g)
			m.MathGlyphInfo.MathTopAccentAttachment.Value(g)
			m.MathGlyphInfo.MathKernInfo.Record(g)
			m.MathVariants.Construction(g, true)
			m.MathVariants.Construction(g, false)
		}
	})
}
