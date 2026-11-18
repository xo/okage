// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"testing"
)

// The tests of pdf-writer src/renumber.rs.

// TestRenumber is test_renumber in pdf-writer src/renumber.rs.
func TestRenumber(t *testing.T) {
	c := NewChunk()

	c.Indirect(NewRef(4)).
		Dict().
		Insert(Name("Nested")).
		Dict().
		Pair(Name("stream"), Bool(true)).
		Pair(Name("Nonself"), NewRef(8)).
		Pair(Name("R"), Bool(false)).
		Pair(Name("Safe"), Int(10)).
		Pair(Name("False"), TextStr("17 0 R")).
		Pair(Name("Unsafe"), NewRef(17)).
		Pair(Name("Unsafe"), TextStr("()(And (More) 17 0 R())"))

	// Manually write an untidy object.
	c.buf.finishFrom(0)
	c.offsets = append(c.offsets, refOffset{NewRef(8), len(c.buf.inner)})
	c.buf.extendString("8  3  obj\n<</Fmt false/Niceness(4 0\nR-)")
	c.buf.extendString("/beginobj/endobj%4 0 R\n")
	c.buf.extendString("/Me 8 3  R/Unknown 11 0  R/R[4  0\nR]>>%\n\nendobj")

	c.Stream(NewRef(17), []byte("1 0 R 2 0 R 3 0 R 4 0 R")).
		Pair(Name("Ok"), TextStr(")4 0 R")).
		Pair(Name("Self"), NewRef(17))

	alloc := NewRef(1)
	m := map[Ref]Ref{}
	r := c.Renumber(func(old Ref) Ref {
		if old == NewRef(11) {
			return old
		}
		if n, ok := m[old]; ok {
			return n
		}
		n := alloc.Bump()
		m[old] = n
		return n
	})

	testBytes(t,
		r.AsBytes(),
		"1 0 obj",
		"<<",
		"  /Nested <<",
		"    /stream true",
		"    /Nonself 2 0 R",
		"    /R false",
		"    /Safe 10",
		"    /False (17 0 R)",
		"    /Unsafe 3 0 R",
		"    /Unsafe (()(And (More) 17 0 R()))",
		"  >>",
		">>",
		"endobj",
		"",
		"2 3 obj",
		"<</Fmt false/Niceness(4 0",
		"R-)/beginobj/endobj%4 0 R",
		"/Me 2 3 R/Unknown 11 0 R/R[1 0 R]>>%",
		"endobj",
		"",
		"3 0 obj",
		"<<",
		"  /Length 23",
		`  /Ok (\)4 0 R)`,
		"  /Self 3 0 R",
		">>",
		"stream",
		"1 0 R 2 0 R 3 0 R 4 0 R",
		"endstream",
		"endobj\n\n",
	)
}

// FuzzRenumber gives the renumbering random objects. It must not panic, and
// a mapping that keeps each number must give back each object that it
// finds.
func FuzzRenumber(f *testing.F) {
	f.Add([]byte("<</A 1 0 R/B (2 0 R)>>"), []byte("8  3  obj\n<</Me 8 3  R%4 0 R\n>>\nendobj"))
	f.Add([]byte("[1 0 R 2 0 R]"), []byte("1 0 obj\n<<>>\nstream\n1 0 R\nendstream\nendobj\n"))
	f.Add([]byte("(\\) 3 0 R"), []byte("obj endobj 0 R 99999999999 0 R"))
	f.Fuzz(func(t *testing.T, a, b []byte) {
		c := NewChunkWithSettings(Settings{Pretty: false})
		c.Stream(NewRef(1), a)
		c.buf.finishFrom(0)
		c.offsets = append(c.offsets, refOffset{NewRef(2), len(c.buf.inner)})
		c.buf.extend(b)
		c.Indirect(NewRef(3)).Primitive(Str(a))
		r := c.Renumber(func(old Ref) Ref { return NewRef(old.Get()%1000 + 1) })
		_ = r.AsBytes()
		same := c.Renumber(func(old Ref) Ref { return old })
		n := 0
		for range same.Refs() {
			n++
		}
		if n < 2 {
			t.Errorf("the renumbering keeps %d objects, want at least 2", n)
		}
	})
}
