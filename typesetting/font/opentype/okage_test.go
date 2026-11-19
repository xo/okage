// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package opentype

// okage: this file is not in go-text. It tests the limits of D61 with small
// fonts that it builds, and the ResourceSlicer of go-text with a font of
// font/testdata, because the tests of go-text read typesetting-utils (D52).

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"runtime"
	"testing"
)

// hugeLength is the length of a table that the tests claim. A buffer of
// this length is almost 4 GB.
const hugeLength = 0xfffffff0

// testOTF returns an OpenType font with one table, whose entry in the table
// directory claims length bytes. The font holds four bytes of the table.
func testOTF(length uint32) []byte {
	b := make([]byte, otfHeaderSize+otfEntrySize+4)
	binary.BigEndian.PutUint32(b[0:], uint32(TrueType))
	binary.BigEndian.PutUint16(b[4:], 1)
	e := b[otfHeaderSize:]
	binary.BigEndian.PutUint32(e[0:], uint32(MustNewTag("head")))
	binary.BigEndian.PutUint32(e[8:], otfHeaderSize+otfEntrySize)
	binary.BigEndian.PutUint32(e[12:], length)
	return b
}

// testWOFFTable is the table of testWOFF. zlib makes it smaller, so the
// loader reads it as a compressed table.
var testWOFFTable = bytes.Repeat([]byte{7}, 1024)

// testWOFF returns a WOFF font with one compressed table, whose entry in the
// table directory claims zLength uncompressed bytes. The table holds the
// bytes of testWOFFTable.
func testWOFF(t *testing.T, zLength uint32) []byte {
	t.Helper()
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	if _, err := w.Write(testWOFFTable); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, woffHeaderSize+woffEntrySize, woffHeaderSize+woffEntrySize+z.Len())
	binary.BigEndian.PutUint32(b[0:], uint32(signatureWOFF))
	binary.BigEndian.PutUint32(b[4:], uint32(TrueType))
	binary.BigEndian.PutUint16(b[12:], 1)
	e := b[woffHeaderSize:]
	binary.BigEndian.PutUint32(e[0:], uint32(MustNewTag("head")))
	binary.BigEndian.PutUint32(e[4:], woffHeaderSize+woffEntrySize)
	binary.BigEndian.PutUint32(e[8:], uint32(z.Len()))
	binary.BigEndian.PutUint32(e[12:], zLength)
	return append(b, z.Bytes()...)
}

// allocated returns the number of bytes that f allocates.
func allocated(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestHugeTable(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"otf", testOTF(hugeLength), io.EOF},
		{"woff", testWOFF(t, hugeLength), io.ErrUnexpectedEOF},
	}
	for _, test := range tests {
		resources := []struct {
			name string
			res  func() Resource
		}{
			{"reader", func() Resource { return bytes.NewReader(test.data) }},
			{"slicer", func() Resource { return NewSlicedResource(test.data) }},
		}
		for _, r := range resources {
			t.Run(test.name+"/"+r.name, func(t *testing.T) {
				ld, err := NewLoader(r.res())
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				var tableErr error
				n := allocated(func() {
					_, tableErr = ld.RawTable(MustNewTag("head"))
				})
				if tableErr == nil {
					t.Fatal("expected an error")
				}
				if r.name == "reader" && !errors.Is(tableErr, test.want) {
					t.Errorf("expected %v, got: %v", test.want, tableErr)
				}
				if n > 1<<20 {
					t.Errorf("expected less than 1 MB of allocations, got: %d bytes", n)
				}
			})
		}
	}
}

func TestTableEnd(t *testing.T) {
	// The table ends at the end of the font, so the loader reads it.
	ld, err := NewLoader(bytes.NewReader(testOTF(4)))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	table, err := ld.RawTable(MustNewTag("head"))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(table) != 4 {
		t.Errorf("expected 4 bytes, got: %d", len(table))
	}
	// The table ends one byte after the end of the font.
	ld, err = NewLoader(bytes.NewReader(testOTF(5)))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if _, err := ld.RawTable(MustNewTag("head")); !errors.Is(err, io.EOF) {
		t.Errorf("expected %v, got: %v", io.EOF, err)
	}
}

func TestWOFFTable(t *testing.T) {
	want := testWOFFTable
	for _, n := range []uint32{1024, 1025} {
		ld, err := NewLoader(bytes.NewReader(testWOFF(t, n)))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		table, err := ld.RawTable(MustNewTag("head"))
		switch {
		case n == 1024 && (err != nil || !bytes.Equal(table, want)):
			t.Errorf("expected %v, got: %v, %v", want, table, err)
		case n == 1025 && !errors.Is(err, io.ErrUnexpectedEOF):
			t.Errorf("expected %v, got: %v", io.ErrUnexpectedEOF, err)
		}
		// A buffer that is large enough is used as it is.
		dst := make([]byte, 1100)
		table, err = ld.RawTableTo(MustNewTag("head"), dst)
		if n == 1024 && (err != nil || !bytes.Equal(table, want) || &table[0] != &dst[0]) {
			t.Errorf("expected %v in dst, got: %v, %v", want, table, err)
		}
	}
}

func TestSlicedResourceRoboto(t *testing.T) {
	data, err := os.ReadFile("../testdata/Roboto-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	want, err := NewLoader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	got, err := NewLoader(NewSlicedResource(data))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	for _, tag := range want.Tables() {
		w, err := want.RawTable(tag)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		g, err := got.RawTable(tag)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !bytes.Equal(g, w) {
			t.Errorf("table %s: expected the same bytes", tag)
		}
		if len(g) != 0 && cap(g) != len(g) {
			t.Errorf("table %s: expected a capacity of %d, got: %d", tag, len(g), cap(g))
		}
	}
}

// TestWriteTTFKeepsInput checks that WriteTTF does not write into the data
// after a table, and that it computes the checksum of a table whose length
// is not a multiple of four with zero padding. The commit f7b0a668 of
// go-text fixes it, and its test reads typesetting-utils (D65, D67).
func TestWriteTTFKeepsInput(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 0xAA, 0xBB, 0xCC}
	out := WriteTTF([]Table{{Content: data[:5], Tag: MustNewTag("abcd")}})
	if want := []byte{1, 2, 3, 4, 5, 0xAA, 0xBB, 0xCC}; !bytes.Equal(data, want) {
		t.Errorf("WriteTTF changes its input to % x", data)
	}
	if got, want := binary.BigEndian.Uint32(out[16:]), uint32(0x01020304+0x05000000); got != want {
		t.Errorf("the checksum is %#x, want %#x", got, want)
	}
}
