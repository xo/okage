// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"math"
	"testing"
)

// The tests test_content_limits and test_chunk_limits of pdf-writer
// src/buf.rs need the writers of src/content.rs and src/xobject.rs. They are
// ported with those files.

// TestPushFloat compares the numbers that the port writes with the numbers
// that pdf-writer writes. A Rust program wrote each f32 of the table with
// Obj::primitive into a chunk with pretty off, in a release build. The
// table covers each branch of push_float and push_decimal: whole numbers
// that fit an i32, the numbers that ryu writes, and the numbers that the
// Display of Rust writes.
func TestPushFloat(t *testing.T) {
	for _, c := range []struct {
		bits uint32
		want string
	}{
		{0x00000000, "0"},
		{0x80000000, "0"},
		{0x3f000000, "0.5"},
		{0xbfc00000, "-1.5"},
		{0x4f32d05e, "3000000000.0"},
		{0xcf32d05e, "-3000000000.0"},
		{0x4f000000, "2147483647"},
		{0xcf000000, "-2147483648"},
		{0xcf000001, "-2147484000.0"},
		{0x7fc00000, "NaN"},
		{0x7f800000, "inf"},
		{0xff800000, "-inf"},
		{0x33d6bf95, "0.0000001"},
		{0x358637bd, "0.000001"},
		{0x35c9539c, "0.0000015"},
		{0x358637be, "0.0000010000001"},
		{0x5368d4a5, "1000000000000"},
		{0x5368d4a5, "1000000000000"},
		{0x551184e7, "10000000000000"},
		{0x7149f2ca, "1000000000000000000000000000000"},
		{0x71977617, "1500000000000000000000000000000"},
		{0x7f7fffff, "340282350000000000000000000000000000000"},
		{0x00800000, "0.000000000000000000000000000000000000011754944"},
		{0x00000001, "0.000000000000000000000000000000000000000000001"},
		{0x47f12065, "123456.79"},
		{0x3dcccccd, "0.1"},
		{0x3e4ccccd, "0.2"},
		{0x3e99999a, "0.3"},
		{0x3eaaaaab, "0.33333334"},
		{0x4b800000, "16777216"},
		{0x4b000000, "8388608"},
		{0x501502f9, "10000000000.0"},
		{0x51e5f18c, "123450000000.0"},
		{0xb900f990, "-0.000123"},
		{0x35865a19, "0.000001001"},
		{0x47c34fff, "99999.99"},
		{0x40870a3d, "4.22"},
		{0x41bb3333, "23.4"},
		{0x33fe4317, "0.0000001184"},
		{0x5618cb8c, "42000000000000"},
	} {
		f := math.Float32frombits(c.bits)
		got := sliceObj(func(obj Obj) { obj.Primitive(Real(f)) }, Settings{Pretty: false})
		if string(got) != c.want {
			t.Errorf("%g (0x%08x): got %q, want %q", f, c.bits, got, c.want)
		}
	}
}

// TestLimits makes sure that the limits keep the largest absolute values,
// that NaN does not replace a real, and that the absolute value of
// math.MinInt32 wraps, as in a release build of Rust.
func TestLimits(t *testing.T) {
	c := NewChunk()
	arr := c.Indirect(NewRef(1)).Array()
	arr.Items(Int(-7), Real(-2.5), Real(float32(math.NaN())), Name("Abc"), Str("abcd"))
	arr.Push().Dict().Pair(Name("A"), Int(1)).Pair(Name("B"), Int(2))
	want := Limits{int: 7, real: 2.5, nameLen: 3, strLen: 4, arrayLen: 6, dictEntries: 2}
	if got := c.Limits(); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	c.Indirect(NewRef(2)).Primitive(Int(math.MinInt32))
	if got := c.Limits().Int(); got != 7 {
		t.Errorf("the limit of the integers is %d, want 7", got)
	}
	other := NewLimits()
	other.registerReal(float32(math.Inf(-1)))
	c.MergeLimits(other)
	if got := c.Limits().Real(); !math.IsInf(float64(got), 1) {
		t.Errorf("the limit of the reals is %g, want +Inf", got)
	}
}

// TestBufPushes writes with the functions of Buf that the writers of later
// files call: the hex digits of the strings of a CMap, the octal escapes of
// a string, and the bytes of another buffer with its limits.
func TestBufPushes(t *testing.T) {
	b := newBuf()
	b.pushHex(0x0F)
	b.pushHexU16(0xBEEF)
	b.push(' ')
	b.pushOctal(0o377)
	b.pushDecimal(2)
	other := newBufWithCapacity(8)
	other.pushVal(Name("Abc"))
	b.extendBuf(&other)
	if got, want := string(b.AsSlice()), "0FBEEF 3772.0/Abc"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := b.Limits(); got.NameLen() != 3 || got.Real() != 2 {
		t.Errorf("the limits are %+v", got)
	}
}
