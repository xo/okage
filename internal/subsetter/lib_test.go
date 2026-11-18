// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xo/okage"
)

// This file ports the tests of subsetter src/lib.rs, and adds tests of the
// seams and of the closure of glyf.go.

// TestMalformedCFFReturnsError is malformed_cff_returns_error in subsetter
// src/lib.rs. Until unit 3 of phase 13 ports cff::subset, the seam returns
// okage.ErrSubsetUnimplemented.
func TestMalformedCFFReturnsError(t *testing.T) {
	var font []byte
	font = binary.BigEndian.AppendUint32(font, 0x4F54544F)
	font = binary.BigEndian.AppendUint16(font, 1)
	font = binary.BigEndian.AppendUint16(font, 0)
	font = binary.BigEndian.AppendUint16(font, 0)
	font = binary.BigEndian.AppendUint16(font, 0)
	font = append(font, "CFF "...)
	font = binary.BigEndian.AppendUint32(font, 0)
	font = binary.BigEndian.AppendUint32(font, 28)
	font = binary.BigEndian.AppendUint32(font, 4)
	font = append(font, 2, 0, 4, 4)

	_, err := Subset(font, 0, NewGlyphRemapper())

	want := error(errCFF)
	if subsetCFFTable == nil {
		want = errUnimplemented
	}
	if !errors.Is(err, want) {
		t.Errorf("got the error %v, want %v", err, want)
	}
}

// TestTableRecordWithInvalidBoundsReturnsNone is
// table_record_with_invalid_bounds_returns_none in subsetter src/lib.rs.
func TestTableRecordWithInvalidBoundsReturnsNone(t *testing.T) {
	f := &face{
		data: nil,
		records: []tableRecord{{
			tag:      tagHEAD,
			checksum: 0,
			offset:   math.MaxUint32,
			length:   math.MaxUint32,
		}},
	}

	if data, ok := f.table(tagHEAD); ok {
		t.Errorf("got the table % x", data)
	}
}

func TestTag(t *testing.T) {
	tag, ok := ParseTag("wght")
	if !ok || tag != NewTag([4]byte{'w', 'g', 'h', 't'}) || tag.Get() != [4]byte{'w', 'g', 'h', 't'} {
		t.Errorf("ParseTag(wght) = %v, %v", tag, ok)
	}
	if _, ok := ParseTag("wgh"); ok {
		t.Error("ParseTag accepts a tag of 3 bytes")
	}
	if s := fmt.Sprintf("%v %#v %v", tagCVT, tagHEAD, Tag{0xff, 0, 0, 0}); s != "cvt  Tag(head) ..." {
		t.Errorf("the tags print as %q", s)
	}
	if tagCFF.compare(tagCFF2) >= 0 || tagGLYF.compare(tagGLYF) != 0 || tagPOST.compare(tagNAME) <= 0 {
		t.Error("the tags do not compare as their bytes")
	}
}

// TestBinarySearchRecords compares binarySearchRecords with a linear search
// on sorted records, and checks a record that the search of Rust finds in
// records that are not sorted.
func TestBinarySearchRecords(t *testing.T) {
	var records []tableRecord
	for i := range 40 {
		records = append(records, tableRecord{tag: Tag{'a', byte('a' + i), 0, 0}, offset: uint32(i)})
	}
	for n := range len(records) + 1 {
		for i := range n + 1 {
			tag := Tag{'a', byte('a' + i), 0, 0}
			got, ok := binarySearchRecords(records[:n], tag)
			if want := i < n; ok != want || (ok && got != i) {
				t.Errorf("searching %v in %d records gives %d, %v", tag, n, got, ok)
			}
		}
	}
	// [b, a, c]: Rust probes c, then a, and does not find b.
	unsorted := []tableRecord{{tag: Tag{'b'}}, {tag: Tag{'a'}}, {tag: Tag{'c'}}}
	if _, ok := binarySearchRecords(unsorted, Tag{'b'}); ok {
		t.Error("the search finds b in [b, a, c]")
	}
	if i, ok := binarySearchRecords(unsorted, Tag{'a'}); !ok || i != 1 {
		t.Errorf("the search finds a in [b, a, c] at %d, %v", i, ok)
	}
}

// TestVariations checks the seam of the instances of variable fonts: the
// default location gives the subset of Subset, and another location returns
// okage.ErrSubsetUnimplemented until unit 4 of phase 13 is ported.
func TestVariations(t *testing.T) {
	// The font folder of svg is the folder that the packages share (D63).
	data, err := os.ReadFile(filepath.Join("..", "..", "svg", "testdata", "fonts", "RobotoFlex.subset.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	m := NewGlyphRemapperFromGlyphs([]uint16{1, 2, 3})
	want, err := Subset(data, 0, m)
	if err != nil {
		t.Fatal(err)
	}
	for _, vars := range [][]Variation{
		nil,
		{{Tag{'w', 'g', 'h', 't'}, 400}},
		{{Tag{'z', 'z', 'z', 'z'}, 900}},
		{{Tag{'w', 'g', 'h', 't'}, 900}, {Tag{'w', 'g', 'h', 't'}, 400}},
	} {
		got, err := SubsetWithVariations(data, 0, vars, m)
		if err != nil {
			t.Errorf("%v: %v", vars, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%v: the subset is not the subset of Subset", vars)
		}
	}
	_, err = SubsetWithVariations(data, 0, []Variation{{Tag{'w', 'g', 'h', 't'}, 900}}, m)
	if interjectGlyph == nil && !errors.Is(err, okage.ErrSubsetUnimplemented) {
		t.Errorf("a location that is not the default gives the error %v", err)
	}
	if _, err := SubsetWithVariations(data, 1, nil, m); !errors.Is(err, okage.ErrSubsetMalformedFont) {
		t.Errorf("the index 1 of a font that is not a collection gives the error %v", err)
	}
}

// TestSubsetDoesNotChange checks that Subset does not change the remapper
// and the data that it gets.
func TestSubsetDoesNotChange(t *testing.T) {
	data := readTestFont(t, "NotoSans-Regular.ttf")
	orig := slices.Clone(data)
	// The glyph 120 of Noto Sans is a composite glyph, whose components
	// the closure adds to the subset.
	m := NewGlyphRemapperFromGlyphs([]uint16{120})
	if _, err := Subset(data, 0, m); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, orig) {
		t.Error("Subset changes the data of the font")
	}
	if got := slices.Collect(m.RemappedGids()); !slices.Equal(got, []uint16{0, 120}) {
		t.Errorf("Subset changes the remapper to %v", got)
	}
}

// TestClosure compares the closure of glyf.go with the closure of upstream
// on fonts of composite glyphs that refer to each other.
func TestClosure(t *testing.T) {
	// Each glyph 1 to k refers to every glyph 1 to k. Upstream pushes about
	// k*k/2 glyphs on its stack, and the closure of the port removes the
	// entries that it skips.
	const k = 600
	all := make([]uint16, k)
	for i := range all {
		all[i] = uint16(i + 1)
	}
	shared := compositeGlyph(all...)
	glyphs := [][]byte{nil}
	for range k {
		glyphs = append(glyphs, shared)
	}
	dense := buildTrueType(glyphs)
	checkClosure(t, "dense", dense, []uint16{1})
	if _, err := Subset(dense, 0, NewGlyphRemapperFromGlyphs([]uint16{1})); err != nil {
		t.Errorf("dense: %v", err)
	}

	// A chain of glyphs with cycles and repeated components. The glyph 900
	// is not in the font.
	glyphs = [][]byte{
		nil,
		compositeGlyph(2, 3, 2, 4),
		compositeGlyph(1, 5, 5),
		compositeGlyph(6, 2),
		simpleGlyph(),
		compositeGlyph(3, 7, 900),
		compositeGlyph(5),
		compositeGlyph(7, 1),
	}
	for _, start := range [][]uint16{{1}, {7}, {4, 6}, {6, 4}, {3, 1}} {
		checkClosure(t, fmt.Sprint("chain ", start), buildTrueType(glyphs), start)
	}
}

// checkClosure compares the order of the remapper after the closure of
// glyf.go with the order after upstreamClosure.
func checkClosure(t *testing.T, name string, font []byte, start []uint16) {
	t.Helper()
	f, err := parse(font, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := NewGlyphRemapperFromGlyphs(start)
	if err := closure(f, got); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	want := NewGlyphRemapperFromGlyphs(start)
	upstreamClosure(t, f, want)
	if !got.Equal(want) {
		t.Errorf("%s: the closure gives %v, and upstream gives %v", name,
			slices.Collect(got.RemappedGids()), slices.Collect(want.RemappedGids()))
	}
}

// upstreamClosure is closure in subsetter src/glyf.rs, as upstream writes
// it, without the changes of the port.
func upstreamClosure(t *testing.T, f *face, glyphRemapper *GlyphRemapper) {
	t.Helper()
	table, ok := newTableGlyf(f)
	if !ok {
		t.Fatal("no glyf table")
	}
	processGlyphs := slices.Collect(glyphRemapper.RemappedGids())
	for len(processGlyphs) > 0 {
		glyph := processGlyphs[len(processGlyphs)-1]
		processGlyphs = processGlyphs[:len(processGlyphs)-1]
		glyphRemapper.Remap(glyph)
		glyphData, ok := table.glyphData(glyph)
		if !ok || len(glyphData) == 0 {
			continue
		}
		if int16(binary.BigEndian.Uint16(glyphData)) < 0 {
			components, _ := componentGlyphs(glyphData)
			for component := range components {
				if _, ok := glyphRemapper.Get(component); !ok {
					processGlyphs = append(processGlyphs, component)
				}
			}
		}
	}
}

func TestCompactGlyphStack(t *testing.T) {
	var processed glyphSet
	processed.add(4)
	got := compactGlyphStack([]uint16{1, 2, 3, 2, 4, 1, 5, 4, 3}, &processed)
	// The copy nearest to the top of each glyph that is not processed stays.
	if want := []uint16{2, 1, 5, 3}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestClosureOverflow checks that a closure that needs more than 65535
// glyphs returns okage.ErrSubsetOverflow, where upstream panics.
func TestClosureOverflow(t *testing.T) {
	// The glyph 1 refers to the glyphs 0 to 65535, and the loca table has
	// no entry for them.
	comps := make([]uint16, 1<<16)
	for i := range comps {
		comps[i] = uint16(i)
	}
	font := buildTrueType([][]byte{nil, compositeGlyph(comps...)})
	if _, err := Subset(font, 0, NewGlyphRemapperFromGlyphs([]uint16{1})); !errors.Is(err, okage.ErrSubsetOverflow) {
		t.Errorf("got the error %v", err)
	}
}

// readTestFont reads a font of testdata.
func readTestFont(tb testing.TB, name string) []byte {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		tb.Fatal(err)
	}
	return data
}

// simpleGlyph returns a simple glyph with one contour of one point.
func simpleGlyph() []byte {
	var g []byte
	for _, v := range []uint16{1, 0, 0, 10, 10, 0, 0} {
		g = binary.BigEndian.AppendUint16(g, v)
	}
	// One flag: on the curve, and both coordinates are 0.
	return append(g, 0x31)
}

// compositeGlyph returns a composite glyph with the components, each with
// two arguments of one byte and no scale.
func compositeGlyph(components ...uint16) []byte {
	var g []byte
	for _, v := range []uint16{0xFFFF, 0, 0, 10, 10} {
		g = binary.BigEndian.AppendUint16(g, v)
	}
	for i, c := range components {
		var flags uint16
		if i < len(components)-1 {
			flags |= moreComponents
		}
		g = binary.BigEndian.AppendUint16(g, flags)
		g = binary.BigEndian.AppendUint16(g, c)
		g = binary.BigEndian.AppendUint16(g, 0)
	}
	return g
}

// buildTrueType returns a TrueType font with the glyphs, and the tables
// head, hhea, hmtx, maxp, loca and glyf. A glyph can share the data of the
// glyph before it: the loca table then gives the same range for both.
func buildTrueType(glyphs [][]byte) []byte {
	loca := buildLoca(glyphs)
	glyf := buildGlyf(glyphs)

	n := uint16(len(glyphs))
	head := make([]byte, 54)
	binary.BigEndian.PutUint32(head, 0x00010000)
	binary.BigEndian.PutUint16(head[18:], 1000)
	binary.BigEndian.PutUint16(head[50:], 1)
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
		{tagGLYF, glyf},
		{tagHEAD, head},
		{tagHHEA, hhea},
		{tagHMTX, hmtx},
		{tagLOCA, loca},
		{tagMAXP, maxp},
	})
}

// buildGlyf returns the glyf table of the glyphs. A glyph that shares the
// data of the glyph before it adds nothing.
func buildGlyf(glyphs [][]byte) []byte {
	var glyf []byte
	for i, g := range glyphs {
		if sharesData(glyphs, i) {
			continue
		}
		glyf = append(glyf, g...)
	}
	return glyf
}

// buildLoca returns the long loca table of the glyphs.
func buildLoca(glyphs [][]byte) []byte {
	var loca []byte
	at := 0
	for i, g := range glyphs {
		if sharesData(glyphs, i) {
			at -= len(g)
		}
		loca = binary.BigEndian.AppendUint32(loca, uint32(at))
		at += len(g)
	}
	return binary.BigEndian.AppendUint32(loca, uint32(at))
}

// sharesData tells whether the glyph i has the same data as the glyph
// before it.
func sharesData(glyphs [][]byte, i int) bool {
	return i > 0 && len(glyphs[i]) > 0 && len(glyphs[i-1]) > 0 && &glyphs[i][0] == &glyphs[i-1][0]
}

// buildSfnt returns a font with the tables, which must be sorted by tag.
func buildSfnt(tables []table) []byte {
	w := newWriter()
	fontFlavorTrueType.write(w)
	w.writeU16(uint16(len(tables)))
	w.writeU16(0)
	w.writeU16(0)
	w.writeU16(0)
	offset := 12 + 16*len(tables)
	for _, t := range tables {
		tableRecord{tag: t.tag, offset: uint32(offset), length: uint32(len(t.data))}.write(w)
		offset += (len(t.data) + 3) &^ 3
	}
	for _, t := range tables {
		w.extend(t.data)
		w.align(4)
	}
	return w.finish()
}
