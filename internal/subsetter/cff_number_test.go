// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"bytes"
	"math"
	"slices"
	"testing"
)

// This file ports the tests of subsetter src/cff/number.rs, and adds tests
// of the conversions that the port writes in Go.

func TestU24(t *testing.T) {
	nums := []uint32{0, 45, 345, 54045, 32849324, 16777215}

	for _, num := range nums {
		wrapped := u24(num)

		w := newWriter()
		wrapped.write(w)
		first := w.finish()

		r := newReader(first)
		rewritten, ok := readU24(&r)
		if !ok {
			t.Fatalf("%d: readU24 fails", num)
		}
		w = newWriter()
		rewritten.write(w)
		second := w.finish()

		if !bytes.Equal(first, second) {
			t.Errorf("%d: % x and % x", num, first, second)
		}
	}
}

// checkIntegerRoundtrip is the body of the tests size*_roundtrip in
// subsetter src/cff/number.rs.
func checkIntegerRoundtrip(t *testing.T, nums []int32, size int) {
	t.Helper()
	for _, num := range nums {
		integer := integerNumber(num)
		w := newWriter()
		integer.write(w)
		buffer := w.finish()
		reader := newReader(buffer)

		reparsed, ok := parseIntegerNumber(&reader)
		if !ok {
			t.Fatalf("%d: parseIntegerNumber fails on % x", num, buffer)
		}
		w = newWriter()
		reparsed.write(w)
		b := w.finish()
		if len(b) != size {
			t.Errorf("%d: %d bytes, want %d", num, len(b), size)
		}
		if int32(reparsed) != num {
			t.Errorf("%d: reparsed as %d", num, reparsed)
		}
	}
}

func TestSize1Roundtrip(t *testing.T) {
	checkIntegerRoundtrip(t, []int32{0, 1, -1, 93, 107, -107}, 1)
}

func TestSize2Roundtrip(t *testing.T) {
	checkIntegerRoundtrip(t, []int32{108, -108, 255, -255, 349, -349, 845, -845, 1131, -1131}, 2)
}

func TestSize3Roundtrip(t *testing.T) {
	checkIntegerRoundtrip(t, []int32{1132, -1132, 2450, -2450, 4096, -4096, 8965, -8965, 32767, -32768}, 3)
}

func TestSize5Roundtrip(t *testing.T) {
	checkIntegerRoundtrip(t, []int32{32768, -32769, math.MaxInt32, math.MinInt32}, 5)
}

func TestParseFloat(t *testing.T) {
	num := []byte{0x1E, 0xE2, 0x49, 0x32, 0xA1, 0x2C, 0x2F}
	r := newReader(num)
	real, ok := parseRealNumber(&r)
	if !ok || real != -249.3212 {
		t.Errorf("got %v, %v", real, ok)
	}
}

func TestFloatRoundtrip(t *testing.T) {
	nums := []float32{0.58, -0.21, 3.98, 16.49, 159.18, 5906.2}

	for _, num := range nums {
		float := realNumber(num)
		w := newWriter()
		float.write(w)
		buffer := w.finish()
		reader := newReader(buffer)

		reparsed, ok := parseRealNumber(&reader)
		if !ok || float32(reparsed) != num {
			t.Errorf("%v: reparsed as %v, %v", num, reparsed, ok)
		}
	}
}

func TestFixed(t *testing.T) {
	num := []byte{255, 154, 104, 120, 40}

	r := newReader(num)
	parsed, ok := parseFixedNumber(&r)
	if !ok {
		t.Fatal("parseFixedNumber fails")
	}

	w := newWriter()
	parsed.write(w)

	if got := w.finish(); !bytes.Equal(got, num) {
		t.Errorf("got % x", got)
	}
}

// TestParseRealNumberText checks the real numbers whose text Rust and
// strconv.ParseFloat read the same way, and the ones that both reject.
func TestParseRealNumberText(t *testing.T) {
	for _, tc := range []struct {
		nibbles []byte
		want    float32
		ok      bool
	}{
		{[]byte{1, 0xa, 5}, 1.5, true},
		{[]byte{0xa, 5}, 0.5, true},
		{[]byte{1, 0xa}, 1, true},
		{[]byte{0xe, 0xa, 2, 5}, -0.25, true},
		{[]byte{1, 0xb, 3}, 1000, true},
		{[]byte{2, 0xc, 2}, 0.02, true},
		{[]byte{1, 0xb, 9, 9}, float32(math.Inf(1)), true},
		{[]byte{0xe, 1, 0xb, 9, 9}, float32(math.Inf(-1)), true},
		{[]byte{1, 0xc, 9, 9}, 0, true},
		{[]byte{}, 0, false},
		{[]byte{0xa}, 0, false},
		{[]byte{0xe}, 0, false},
		{[]byte{1, 0xb}, 0, false},
		{[]byte{0xb, 1}, 0, false},
		{[]byte{1, 0xe, 2}, 0, false},
		{[]byte{1, 0xa, 2, 0xa, 3}, 0, false},
		{[]byte{1, 0xc, 0xe, 2}, 0, false},
		{[]byte{1, 0xd}, 0, false},
	} {
		data := []byte{30}
		nibbles := append(slices.Clone(tc.nibbles), 0xf)
		if len(nibbles)%2 != 0 {
			nibbles = append(nibbles, 0xf)
		}
		for i := 0; i < len(nibbles); i += 2 {
			data = append(data, nibbles[i]<<4|nibbles[i+1])
		}
		r := newReader(data)
		got, ok := parseRealNumber(&r)
		if ok != tc.ok || (ok && float32(got) != tc.want) {
			t.Errorf("% x: got %v, %v, want %v, %v", data, got, ok, tc.want, tc.ok)
		}
	}
	// The text of a number holds at most 64 bytes.
	data := append([]byte{30}, bytes.Repeat([]byte{0x11}, 32)...)
	r := newReader(append(data, 0xff))
	if _, ok := parseRealNumber(&r); !ok {
		t.Error("a real number of 64 digits does not parse")
	}
	r = newReader(append(data, 0x1f))
	if _, ok := parseRealNumber(&r); ok {
		t.Error("a real number of 65 digits parses")
	}
}

// TestWriteRealNumber checks the text of real numbers, which is the impl
// Display of f32 of Rust.
func TestWriteRealNumber(t *testing.T) {
	for _, tc := range []struct {
		f    float32
		want []byte
	}{
		{0.001, []byte{30, 0x0a, 0x00, 0x1f}},
		{1000, []byte{30, 0x10, 0x00, 0xff}},
		{-1.5, []byte{30, 0xe1, 0xa5, 0xff}},
		{float32(math.Copysign(0, -1)), []byte{30, 0xe0, 0xff}},
		{1e-7, []byte{30, 0x0a, 0x00, 0x00, 0x00, 0x1f}},
		{1e10, []byte{30, 0x10, 0x00, 0x00, 0x00, 0x00, 0x0f}},
	} {
		w := newWriter()
		realNumber(tc.f).write(w)
		if got := w.finish(); !bytes.Equal(got, tc.want) {
			t.Errorf("%v: got % x, want % x", tc.f, got, tc.want)
		}
	}
	if err := writeNumbers(newWriter(), []number{numberFromF32(float32(math.NaN()))}); err == nil {
		t.Error("writeNumbers writes NaN")
	}
}

func TestNumberAsI32(t *testing.T) {
	for _, tc := range []struct {
		n    number
		want int32
		ok   bool
	}{
		{numberFromI32(-5), -5, true},
		{numberFromF32(3), 3, true},
		{numberFromF32(3.5), 0, false},
		{numberFromF32(1e20), math.MaxInt32, true},
		{numberFromF32(-1e20), math.MinInt32, true},
		{numberFromF32(float32(math.Inf(1))), 0, false},
		{numberFromF32(float32(math.NaN())), 0, false},
		{number{kind: numberFixed, fixed: 3 << 16}, 3, true},
		{number{kind: numberFixed, fixed: 3<<16 + 1}, 0, false},
	} {
		got, ok := tc.n.asI32()
		if got != tc.want || ok != tc.ok {
			t.Errorf("%v: got %d, %v", tc.n, got, ok)
		}
	}
	if _, ok := numberFromI32(-1).asU32(); ok {
		t.Error("asU32 gives a value for -1")
	}
}

func TestCombineNumbers(t *testing.T) {
	m := [6]number{numberFromF32(0.001), numberZero(), numberZero(), numberFromF32(0.002), numberFromI32(3), numberFromI32(4)}
	scale := [6]number{numberFromF32(1000), numberZero(), numberZero(), numberFromF32(1000), numberZero(), numberZero()}
	got := combineNumbers(m, scale)
	want := [6]float32{1, 0, 0, 2, 3, 4}
	for i := range got {
		if got[i].kind != numberReal || float32(got[i].real) != want[i] {
			t.Errorf("%d: got %v", i, got[i])
		}
	}
}
