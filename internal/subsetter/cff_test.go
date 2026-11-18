// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"bytes"
	"encoding/binary"
	"errors"
	"slices"
	"testing"
	"time"
)

// subsetter src/cff has no tests other than those of number.rs and
// remapper.rs. These tests check the parts of the CFF code that the fonts of
// TestSubsets reach rarely. The test module compares the subsets of the CFF
// fonts with the oracle.

// TestIndex writes INDEXes with createIndex, and reads them back.
func TestIndex(t *testing.T) {
	for _, items := range [][][]byte{
		{},
		{nil},
		{[]byte("a"), nil, []byte("bc")},
		{bytes.Repeat([]byte{1}, 300)},
		{bytes.Repeat([]byte{2}, 70000)},
	} {
		owned, err := createIndex(items)
		if err != nil {
			t.Fatal(err)
		}
		r := newReader(owned.data)
		x, ok := parseIndex(&r)
		if !ok || !r.atEnd() {
			t.Fatalf("%d items: parseIndex gives %v, at %d of %d", len(items), ok, r.offset(), len(owned.data))
		}
		got := slices.Collect(x.all())
		if len(got) != len(items) {
			t.Fatalf("%d items: got %d", len(items), len(got))
		}
		for i := range got {
			if !bytes.Equal(got[i], items[i]) {
				t.Errorf("%d items: item %d differs", len(items), i)
			}
		}
		r = newReader(owned.data)
		if !skipIndex(&r) || !r.atEnd() {
			t.Errorf("%d items: skipIndex stops at %d", len(items), r.offset())
		}
	}
	if _, err := createIndex(make([][]byte, 0x10000)); !errors.Is(err, errOverflow) {
		t.Errorf("an INDEX of 65536 items gives the error %v", err)
	}
	// An offset of 0 ends the items, as in Rust.
	r := newReader([]byte{0, 2, 1, 1, 0, 2, 'a'})
	x, ok := parseIndex(&r)
	if !ok {
		t.Fatal("parseIndex fails")
	}
	if got := slices.Collect(x.all()); len(got) != 0 {
		t.Errorf("got %q", got)
	}
}

// TestDictionaryParser checks the operands of a DICT, and the limit of 48
// operands.
func TestDictionaryParser(t *testing.T) {
	var data []byte
	for range 50 {
		data = append(data, 139)
	}
	data = append(data, 6, 140, 12, 7, 30, 0x1a, 0x5f, 18)
	p := newDictionaryParser(data)
	op, ok := p.parseNext()
	if !ok || op != 6 {
		t.Fatalf("got %v, %v", op, ok)
	}
	if !p.parseOperands() || len(p.operandsSlice()) != 48 {
		t.Errorf("the operands are %v", p.operandsSlice())
	}
	op, ok = p.parseNext()
	if !ok || op != opFontMatrix || op.String() != "127" {
		t.Fatalf("got %v, %v", op, ok)
	}
	if v, ok := p.parseOffset(); !ok || v != 1 {
		t.Errorf("parseOffset = %d, %v", v, ok)
	}
	if _, ok := p.parseNext(); !ok {
		t.Fatal("no third operator")
	}
	if !p.parseOperands() || len(p.operandsSlice()) != 1 || p.operandsSlice()[0].asF64() != 1.5 {
		t.Errorf("the operands are %v", p.operandsSlice())
	}
	if _, ok := p.parseNext(); ok {
		t.Error("a fourth operator")
	}
}

// TestDecompile decompiles charstrings that call subroutines, with hint
// masks, and compiles them.
func TestDecompile(t *testing.T) {
	// The global subroutine 0 draws a line and returns. The local
	// subroutine 0 ends the glyph.
	gsubrs := [][]byte{{139, 140, 5, 11}}
	lsubrs := [][]byte{{14}}
	charstring := []byte{
		// A width and a horizontal stem, then a hint mask with an implicit
		// vertical stem.
		150, 139, 149, 1,
		139, 149, 19, 0xC0,
		// rmoveto, then the call of the global subroutine 0 with the bias
		// 107.
		139, 139, 21,
		32, 29,
		// The call of the local subroutine 0 ends the glyph, and the bytes
		// after it are not read.
		32, 10,
		0,
	}
	d := newDecompiler(newSubroutineHandler(gsubrs), newSubroutineHandler(lsubrs))
	p, err := d.decompile(charstring)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{150, 139, 149, 1, 139, 149, 19, 0xC0, 139, 139, 21, 139, 140, 5, 14}
	if got := p.compile(); !bytes.Equal(got, want) {
		t.Errorf("got % x, want % x", got, want)
	}
	if got, want := p.String(), "11 0 10 op(1)\n0 10 op(19) 11000000\n0 0 op(21)\n0 1 op(5)\nop(14)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestDecompileErrors checks the errors of the decompiler.
func TestDecompileErrors(t *testing.T) {
	recursive := [][]byte{{32, 10}}
	for _, tc := range []struct {
		name       string
		charstring []byte
		want       error
	}{
		{"seac", []byte{139, 139, 139, 139, 14}, errUnimplemented},
		{"seac with a width", []byte{139, 139, 139, 139, 139, 14}, errUnimplemented},
		{"endchar with 3 arguments", []byte{139, 139, 139, 14}, nil},
		{"recursion", []byte{32, 10}, errCFF},
		{"a call without index", []byte{10}, errCFF},
		{"a call out of range", []byte{33, 10}, errCFF},
		{"a call with a fraction", []byte{255, 0x80, 0, 0x80, 0, 10}, errCFF},
		{"an unknown operator", []byte{2}, errCFF},
		{"an unknown two-byte operator", []byte{12, 3}, errCFF},
		{"a two-byte operator at the end", []byte{12}, errCFF},
		{"a short number", []byte{28, 1}, errCFF},
		{"a short hint mask", []byte{139, 139, 1, 19}, errCFF},
		{"49 arguments", bytes.Repeat([]byte{139}, 49), errCFF},
		{"48 arguments", bytes.Repeat([]byte{139}, 48), nil},
	} {
		d := newDecompiler(newSubroutineHandler(nil), newSubroutineHandler(recursive))
		_, err := d.decompile(tc.charstring)
		if !errors.Is(err, tc.want) || (tc.want == nil) != (err == nil) {
			t.Errorf("%s: got the error %v, want %v", tc.name, err, tc.want)
		}
	}
}

// TestFDSelect reads the font DICT of glyphs in an FDSelect of the format 3.
func TestFDSelect(t *testing.T) {
	data := []byte{3, 0, 2, 0, 0, 5, 0, 3, 7, 0, 10, 0xFF}
	r := newReader(data)
	s, ok := parseFDSelect(10, &r)
	if !ok {
		t.Fatal("parseFDSelect fails")
	}
	for glyph, want := range []int{5, 5, 5, 7, 7, 7, 7, 7, 7, 7, -1} {
		got, ok := s.fontDictIndex(uint16(glyph))
		if (want < 0) == ok || (ok && int(got) != want) {
			t.Errorf("glyph %d: got %d, %v", glyph, got, ok)
		}
	}
	r = newReader([]byte{0, 1, 2})
	s, ok = parseFDSelect(2, &r)
	if !ok {
		t.Fatal("parseFDSelect fails")
	}
	if got, ok := s.fontDictIndex(1); !ok || got != 2 {
		t.Errorf("got %d, %v", got, ok)
	}
	if _, ok := s.fontDictIndex(2); ok {
		t.Error("the glyph 2 has a font DICT")
	}
	r = newReader([]byte{1})
	if _, ok := parseFDSelect(2, &r); ok {
		t.Error("the format 1 parses")
	}
}

// TestUnapplyBias checks the bias of the subroutines at the limits of an
// int32.
func TestUnapplyBias(t *testing.T) {
	for _, tc := range []struct {
		index int32
		bias  uint16
		want  uint32
		ok    bool
	}{
		{-107, 107, 0, true},
		{-108, 107, 0, false},
		{2147483647, 107, 0, false},
		{2147483647 - 32768, 32768, 2147483647, true},
	} {
		got, ok := unapplyBias(tc.index, tc.bias)
		if got != tc.want || ok != tc.ok {
			t.Errorf("unapplyBias(%d, %d) = %d, %v", tc.index, tc.bias, got, ok)
		}
	}
	for _, tc := range []struct {
		n    uint32
		want uint16
	}{{0, 107}, {1239, 107}, {1240, 1131}, {33899, 1131}, {33900, 32768}} {
		if got := calcSubroutineBias(tc.n); got != tc.want {
			t.Errorf("calcSubroutineBias(%d) = %d", tc.n, got)
		}
	}
}

// TestSubsetCFFFonts subsets the CFF fonts of testdata: glyph 0 only, the
// glyph whose endchar is in a subroutine, and a glyph that is not in the
// font.
func TestSubsetCFFFonts(t *testing.T) {
	for _, tc := range []struct {
		font   string
		glyphs []uint16
		want   error
	}{
		{"LatinModernRoman-Regular.otf", nil, nil},
		{"EndcharInSubroutine.otf", []uint16{1}, nil},
		{"NotoSansCJKsc-Bold-subset1.otf", []uint16{1}, nil},
		{"NewCMMath-Regular.otf", []uint16{60000}, errMalformedFont},
	} {
		data := readTestFont(t, tc.font)
		out, err := Subset(data, 0, NewGlyphRemapperFromGlyphs(tc.glyphs))
		if !errors.Is(err, tc.want) || (tc.want == nil) != (err == nil) {
			t.Errorf("%s: got the error %v, want %v", tc.font, err, tc.want)
			continue
		}
		if err != nil {
			continue
		}
		f, err := parse(out, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !f.hasTable(tagCFF) || f.hasTable(tagGLYF) {
			t.Errorf("%s: the subset has the wrong tables", tc.font)
		}
	}
}

// TestDecompileLimits checks the limits of the work of the decompiler for
// one glyph, which upstream does not have. The font has 63 nested local
// subroutines, the most that the depth limit of 64 lets a glyph reach, and
// each one calls the next one twice. Upstream runs 2^63 subroutines for its
// glyph 1, and the port stops at the limit of the operators. With 64 nested
// subroutines, upstream and the port stop at the depth limit at once.
func TestDecompileLimits(t *testing.T) {
	line := []byte{139, 140, 5, 11}
	font := buildCFFFont(t, [][]byte{{14}, {32, 10, 14}}, nestedSubrs(63, line))

	start := time.Now()
	_, err := Subset(font, 0, NewGlyphRemapperFromGlyphs([]uint16{1}))
	if !errors.Is(err, errMalformedFont) {
		t.Errorf("63 levels: got the error %v", err)
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("63 levels: the subset takes %v", d)
	}
	if _, err := Subset(font, 0, NewGlyphRemapper()); err != nil {
		t.Errorf("63 levels, the glyph 0: %v", err)
	}
	font = buildCFFFont(t, [][]byte{{14}, {32, 10, 14}}, nestedSubrs(64, line))
	if _, err := Subset(font, 0, NewGlyphRemapperFromGlyphs([]uint16{1})); !errors.Is(err, errCFF) {
		t.Errorf("64 levels: got the error %v", err)
	}

	// 16 levels give 2^16 calls of a subroutine of 49 instructions, more
	// than the limit of the program, with fewer operators than their
	// limit.
	long := append(bytes.Repeat([]byte{139}, 48), 5, 11)
	d := newDecompiler(newSubroutineHandler(nil), newSubroutineHandler(nestedSubrs(16, long)))
	if _, err := d.decompile([]byte{32, 10, 14}); !errors.Is(err, errMalformedFont) {
		t.Errorf("16 levels: got the error %v", err)
	}
	if d.operators > maxDecompiledOperators {
		t.Errorf("16 levels: %d operators", d.operators)
	}

	// 16 levels of lines are within both limits.
	d = newDecompiler(newSubroutineHandler(nil), newSubroutineHandler(nestedSubrs(16, line)))
	p, err := d.decompile([]byte{32, 10, 14})
	if err != nil {
		t.Fatalf("16 levels of lines: %v", err)
	}
	if n := len(p.compile()); n != 3<<15+1 {
		t.Errorf("16 levels of lines: %d bytes", n)
	}
}

// nestedSubrs returns n local subroutines. The subroutine i calls the
// subroutine i+1 twice, with the bias 107, and the last one is last.
func nestedSubrs(n int, last []byte) [][]byte {
	var subrs [][]byte
	for i := range n - 1 {
		subrs = append(subrs, []byte{byte(i + 1 + 32), 10, byte(i + 1 + 32), 10, 11})
	}
	return append(subrs, last)
}

// buildCFFFont returns an OpenType font with a SID-keyed CFF table that
// holds the charstrings and the local subroutines, and the tables head,
// hhea, hmtx and maxp.
func buildCFFFont(t *testing.T, charStrings, subrs [][]byte) []byte {
	t.Helper()
	index := func(items [][]byte) []byte {
		x, err := createIndex(items)
		if err != nil {
			t.Fatal(err)
		}
		return x.data
	}
	int5 := func(v int) []byte {
		w := newWriter()
		integerNumber(v).writeAs5Bytes(w)
		return w.finish()
	}
	// The Private DICT holds only the Subrs offset, relative to itself.
	private := append(int5(6), 19)
	build := func(charStringsAt, privateAt int) []byte {
		top := append(int5(charStringsAt), 17)
		top = append(top, int5(len(private))...)
		top = append(append(top, int5(privateAt)...), 18)
		cff := []byte{1, 0, 4, 4}
		cff = append(cff, index([][]byte{[]byte("Test")})...)
		cff = append(cff, index([][]byte{top})...)
		cff = append(cff, index(nil)...)
		cff = append(cff, index(nil)...)
		cff = append(cff, index(charStrings)...)
		cff = append(cff, private...)
		return append(cff, index(subrs)...)
	}
	first := build(0, 0)
	privateAt := len(first) - len(index(subrs)) - len(private)
	charStringsAt := privateAt - len(index(charStrings))
	cff := build(charStringsAt, privateAt)

	n := uint16(len(charStrings))
	head := make([]byte, 54)
	binary.BigEndian.PutUint32(head, 0x00010000)
	binary.BigEndian.PutUint16(head[18:], 1000)
	hhea := make([]byte, 36)
	binary.BigEndian.PutUint32(hhea, 0x00010000)
	binary.BigEndian.PutUint16(hhea[34:], n)
	var hmtx []byte
	for i := range n {
		hmtx = binary.BigEndian.AppendUint16(hmtx, 500+i)
		hmtx = binary.BigEndian.AppendUint16(hmtx, i)
	}
	maxp := make([]byte, 6)
	binary.BigEndian.PutUint32(maxp, 0x00005000)
	binary.BigEndian.PutUint16(maxp[4:], n)
	return buildSfnt([]table{
		{tagCFF, cff},
		{tagHEAD, head},
		{tagHHEA, hhea},
		{tagHMTX, hmtx},
		{tagMAXP, maxp},
	})
}
