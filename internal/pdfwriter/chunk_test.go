// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"slices"
	"strings"
	"testing"
)

// The test test_chunk of pdf-writer src/chunk.rs needs the writers of
// src/font.rs. It is ported with that file. The tests below test how the
// port finishes its writers, which the borrow checker and Drop do in Rust.

// mustPanic fails the test if f does not panic with a message that holds
// want.
func mustPanic(t *testing.T, want string, f func()) {
	t.Helper()
	defer func() {
		t.Helper()
		r := recover()
		s, _ := r.(string)
		if !strings.Contains(s, want) {
			t.Errorf("the panic is %v, want %q", r, want)
		}
	}()
	f()
}

// TestFinishOrder writes nested writers without a call to Finish. Each call
// on a parent finishes its children, and the next object finishes the rest,
// as Rust drops them.
func TestFinishOrder(t *testing.T) {
	c := NewChunkWithSettings(Settings{Pretty: false})
	d := c.Indirect(NewRef(1)).Dict()
	inner := d.Insert(Name("A")).Array()
	inner.Item(Int(1))
	deeper := inner.Push().Dict()
	deeper.Pair(Name("X"), Int(2))
	d.Pair(Name("B"), Int(3))
	d.Insert(Name("C")).Dict().Pair(Name("Y"), Int(4))
	c.Indirect(NewRef(2)).Primitive(Null{})
	want := "1 0 obj\n<</A[1<</X 2>>]/B 3/C<</Y 4>>>>\nendobj\n2 0 obj\nnull\nendobj\n"
	if got := string(c.AsBytes()); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !slices.Equal(slices.Collect(c.Refs()), []Ref{NewRef(1), NewRef(2)}) {
		t.Errorf("the references are %v", slices.Collect(c.Refs()))
	}
}

// TestFinishTwice makes sure that Finish finishes the children first, and
// that a second Finish does nothing.
func TestFinishTwice(t *testing.T) {
	c := NewChunkWithSettings(Settings{Pretty: false})
	a := c.Indirect(NewRef(1)).Array()
	a.Push().Array().Item(Int(1))
	a.Finish()
	a.Finish()
	if got, want := string(c.AsBytes()), "1 0 obj\n[[1]]\nendobj\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestMisuse makes sure that a write that the borrow checker of Rust forbids
// panics.
func TestMisuse(t *testing.T) {
	mustPanic(t, "the Dict is finished", func() {
		c := NewChunk()
		d := c.Indirect(NewRef(1)).Dict()
		c.Indirect(NewRef(2)).Primitive(Null{})
		d.Pair(Name("A"), Int(1))
	})
	mustPanic(t, "the Array is finished", func() {
		c := NewChunk()
		a := c.Indirect(NewRef(1)).Array()
		a.Finish()
		a.Item(Int(1))
	})
	mustPanic(t, "an Obj writes", func() {
		c := NewChunk()
		obj := c.Indirect(NewRef(1))
		obj.Primitive(Int(1))
		obj.Primitive(Int(2))
	})
	mustPanic(t, "an Obj writes", func() {
		c := NewChunk()
		d := c.Indirect(NewRef(1)).Dict()
		obj := d.Insert(Name("A"))
		d.Pair(Name("B"), Int(1))
		obj.Primitive(Int(2))
	})
	mustPanic(t, "an Obj writes", func() {
		c := NewChunk()
		d := c.Indirect(NewRef(1)).Dict()
		obj := d.Insert(Name("A"))
		d.Finish()
		obj.Primitive(Int(2))
	})
	mustPanic(t, "indirect reference out of valid range", func() { NewRef(0) })
	mustPanic(t, "assertion failed", func() {
		c := NewChunk()
		startStream(c.Indirect(NewRef(1)).Dict().Insert(Name("S")), nil)
	})
}

// TestExtend makes sure that Chunk.Extend finishes the writers of both
// chunks and moves the offsets, and that Pdf.Finish lists the objects of
// the other chunk.
func TestExtend(t *testing.T) {
	w := NewPdf()
	w.Indirect(NewRef(1)).Dict().Pair(Name("A"), NewRef(2))
	c := NewChunk()
	c.Indirect(NewRef(2)).Array().Item(Int(1))
	w.Extend(c)
	testBytes(t,
		w.Finish(),
		"%PDF-1.7\n%\x80\x80\x80\x80\n",
		"1 0 obj",
		"<<\n  /A 2 0 R\n>>",
		"endobj\n",
		"2 0 obj",
		"[1]",
		"endobj\n",
		"xref",
		"0 3",
		"0000000000 65535 f\r",
		"0000000016 00000 n\r",
		"0000000049 00000 n\r",
		"trailer",
		"<<\n  /Size 3\n>>",
		"startxref\n69\n%%EOF",
	)
}

// TestClone makes sure that a clone does not share bytes with its chunk.
func TestClone(t *testing.T) {
	c := NewChunk()
	c.Indirect(NewRef(1)).Primitive(Int(1))
	d := c.Clone()
	d.Indirect(NewRef(2)).Primitive(Int(2))
	if c.Len() == d.Len() || len(slices.Collect(c.Refs())) != 1 {
		t.Errorf("the clone shares its bytes: %q and %q", c.AsBytes(), d.AsBytes())
	}
}

// TestTrees writes a name tree and a number tree, as krilla writes the
// destinations and the page labels.
func TestTrees(t *testing.T) {
	c := NewChunkWithSettings(Settings{Pretty: false})
	ChunkNameTree[Ref](c, NewRef(1)).Limits(Name("a"), Name("b")).Names().Insert(Str("a"), NewRef(3))
	n := ChunkNumberTree[Ref](c, NewRef(2))
	n.Kids().Items(NewRef(4))
	n.Limits(0, 9)
	n.Nums().Insert(0, NewRef(5)).Insert(9, NewRef(6))
	want := "1 0 obj\n<</Limits[/a/b]/Names[(a)3 0 R]>>\nendobj\n" +
		"2 0 obj\n<</Kids[4 0 R]/Limits[0 9]/Nums[0 5 0 R 9 6 0 R]>>\nendobj\n"
	if got := string(c.AsBytes()); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestDecodeParms writes each pair of a DecodeParms dictionary, and makes
// sure that the checks of upstream panic.
func TestDecodeParms(t *testing.T) {
	c := NewChunkWithSettings(Settings{Pretty: false})
	s := c.Stream(NewRef(1), []byte("x"))
	s.Filter(FilterFlateDecode)
	s.DecodeParms().Predictor(PredictorPngOptimum).Colors(3).BitsPerComponent(8).Columns(4).EarlyChange(false).
		K(-1).EndOfLine(true).EncodedByteAlign(true).Rows(2).EndOfBlock(false).BlackIs1(true).
		DamagedRowsBeforeError(1).Jbig2Globals(NewRef(2)).ColorTransform(true).CryptType().Name(Name("Std"))
	want := "1 0 obj\n<</Length 1/Filter/FlateDecode/DecodeParms<</Predictor 15/Colors 3/BitsPerComponent 8" +
		"/Columns 4/EarlyChange 0/K -1/EndOfLine true/EncodedByteAlign true/Rows 2/EndOfBlock false" +
		"/BlackIs1 true/DamagedRowsBeforeError 1/JBIG2Globals 2 0 R/ColorTransform 1" +
		"/Type/CryptFilterDecodeParms/Name/Std>>>>\nstream\nx\nendstream\nendobj\n"
	if got := string(c.AsBytes()); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	mustPanic(t, "`Colors` must be greater than 0", func() {
		NewChunk().Stream(NewRef(1), nil).DecodeParms().Colors(0)
	})
	mustPanic(t, "`BitsPerComponent` must be one of", func() {
		NewChunk().Stream(NewRef(1), nil).DecodeParms().BitsPerComponent(3)
	})
}

// TestTextStrWithLang writes a text string with two languages.
func TestTextStrWithLang(t *testing.T) {
	en, ok := LanguageIdentifierFromLang("en")
	if !ok {
		t.Fatal("en is not a language")
	}
	deCH, ok := LanguageIdentifierFromLangRegion("de", "CH")
	if !ok {
		t.Fatal("de CH is not a language and a region")
	}
	if _, ok := LanguageIdentifierFromLang("e-"); ok {
		t.Error("e- is a language")
	}
	testPrimitive(t, TextStrWithLang{{en, "Hi"}, {deCH, "ä"}},
		"<FEFF001B0065006E001B00480069001B00640065004300480"+"01B00E4>")
}
