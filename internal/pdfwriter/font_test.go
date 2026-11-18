// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"strings"
	"testing"
)

// TestChunk is test_chunk in pdf-writer src/chunk.rs. It waited for the
// writers of src/font.rs.
func TestChunk(t *testing.T) {
	w := NewPdf()
	font := w.Type3Font(NewRef(1))
	c := NewChunk()
	c.FontDescriptor(NewRef(2)).Name(Name("MyFont"))
	font.FontDescriptor(NewRef(2))
	font.Finish()
	w.Extend(c)
	testBytes(t,
		w.Finish(),
		"%PDF-1.7\n%\x80\x80\x80\x80\n",
		"1 0 obj",
		"<<\n  /Type /Font\n  /Subtype /Type3\n  /FontDescriptor 2 0 R\n>>",
		"endobj\n",
		"2 0 obj",
		"<<\n  /Type /FontDescriptor\n  /FontName /MyFont\n>>",
		"endobj\n",
		"xref",
		"0 3",
		"0000000000 65535 f\r",
		"0000000016 00000 n\r",
		"0000000094 00000 n\r",
		"trailer",
		"<<\n  /Size 3\n>>",
		"startxref\n160\n%%EOF",
	)
}

// TestUnicodeCmap writes a map with one byte for each glyph, and a map with
// two bytes. The test module compares both kinds with pdf-writer, byte for
// byte. This test makes sure that a range ends after 100 lines, and that a
// character outside the BMP takes two code units of UTF-16.
func TestUnicodeCmap(t *testing.T) {
	info := SystemInfo{Registry: Str("Adobe"), Ordering: Str("Identity"), Supplement: 0}

	small := NewUnicodeCmap[uint8](Name("Custom"), info)
	small.Pair(0x41, 'A')
	b := small.Finish()
	got := string(b.AsSlice())
	for _, want := range []string{
		"%%Title: (Custom Adobe Identity 0)\n",
		"/WMode 0 def\n",
		"1 begincodespacerange\n<00> <FF>\nendcodespacerange\n",
		"1 beginbfchar\n<41> <0041>\nendbfchar\nendcmap\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the map with one byte has no %q:\n%s", want, got)
		}
	}
	if !strings.HasSuffix(got, "%%EndResource\n%%EOF") {
		t.Errorf("the map ends with %q", got[len(got)-20:])
	}

	large := NewUnicodeCmapWithWritingMode[uint16](Name("Custom"), info, WModeVertical)
	for g := range uint16(101) {
		large.Pair(g, 'a')
	}
	large.PairWithMultiple(0x1234, '😀', 'f')
	b = large.Finish()
	got = string(b.AsSlice())
	for _, want := range []string{
		"/WMode 1 def\n",
		"<0000> <FFFF>\n",
		"100 beginbfchar\n<0000> <0061>\n",
		"<0063> <0061>\nendbfchar\n2 beginbfchar\n<0064> <0061>\n<1234> <D83DDE000066>\nendbfchar\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the map with two bytes has no %q:\n%s", want, got)
		}
	}
}
