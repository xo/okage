// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"testing"
)

// The tests of pdf-writer src/object.rs.

// TestPrimitiveObjects is test_primitive_objects in pdf-writer
// src/object.rs.
func TestPrimitiveObjects(t *testing.T) {
	// Test really simple objects.
	testPrimitive(t, Bool(true), "true")
	testPrimitive(t, Bool(false), "false")
	testPrimitive(t, Int(78), "78")
	testPrimitive(t, Real(4.22), "4.22")
	testPrimitive(t, Real(1.184e-7), "0.0000001184")
	testPrimitive(t, Real(4.2e13), "42000000000000")
	testPrimitive(t, NewRef(7), "7 0 R")
	testPrimitive(t, Null{}, "null")

	// Test strings.
	testPrimitive(t, Str("Hello, World!"), "(Hello, World!)")
	testPrimitive(t, Str("()"), `(())`)
	testPrimitive(t, Str(")()"), `(\)\(\))`)
	testPrimitive(t, Str("()(())"), `(()(()))`)
	testPrimitive(t, Str("(()))"), `(\(\(\)\)\))`)
	testPrimitive(t, Str("\\"), `(\\)`)
	testPrimitive(t, Str("\n\ta"), `(\n\ta)`)
	testPrimitive(t, Str(`\n`), `(\\n)`)
	testPrimitive(t, Str("a\x14b"), `(a\024b)`)
	testPrimitive(t, Str("\xFF\xAA"), "<FFAA>")
	testPrimitive(t, Str("\x0A\x7F\x1F"), `(\n\177\037)`)

	// Test text strings.
	testPrimitive(t, TextStr("Hallo"), "(Hallo)")
	testPrimitive(t, TextStr("😀!"), "<FEFFD83DDE000021>")

	// Test names.
	testPrimitive(t, Name("Filter"), "/Filter")
	testPrimitive(t, Name("A B"), `/A#20B`)
	testPrimitive(t, Name("~+c"), `/~+c`)
	testPrimitive(t, Name("/A-B"), `/#2FA-B`)
	testPrimitive(t, Name("<A>"), `/#3CA#3E`)
	testPrimitive(t, Name("#"), `/#23`)
	testPrimitive(t, Name("\n"), `/#0A`)
}

// TestDates is test_dates in pdf-writer src/object.rs.
func TestDates(t *testing.T) {
	testPrimitive(t, NewDate(2021), "(D:2021)")
	testPrimitive(t, NewDate(2021).Month(30), "(D:202112)")

	date := NewDate(2020).Month(3).Day(17).Hour(1).Minute(2).Second(3)
	testPrimitive(t, date, "(D:20200317010203)")
	testPrimitive(t, date.UTCOffsetHour(0), "(D:20200317010203Z)")
	testPrimitive(t, date.UTCOffsetHour(4), "(D:20200317010203+04'00)")
	testPrimitive(t,
		date.UTCOffsetHour(-17).UTCOffsetMinute(10),
		"(D:20200317010203-17'10)",
	)
}

// TestArrays is test_arrays in pdf-writer src/object.rs.
func TestArrays(t *testing.T) {
	testObj(t, func(obj Obj) { obj.Array() }, "[]")
	testObj(t, func(obj Obj) { obj.Array().Item(Int(12)).Item(Null{}) }, "[12 null]")
	testObj(t, func(obj Obj) { NewTypedArray[Int](obj.Array()).Items(1, 2, 3) }, "[1 2 3]")
	testObj(t,
		func(obj Obj) {
			array := obj.Array()
			NewTypedArray[Int](array.Push().Array()).Items(1, 2)
			array.Item(Int(3))
		},
		"[[1 2] 3]",
	)
}

// TestDicts is test_dicts in pdf-writer src/object.rs.
func TestDicts(t *testing.T) {
	testObj(t, func(obj Obj) { obj.Dict() }, "<<>>")
	testObj(t,
		func(obj Obj) { obj.Dict().Pair(Name("Quality"), Name("Good")) },
		"<<\n  /Quality /Good\n>>",
	)
	testObj(t,
		func(obj Obj) {
			obj.Dict().Pair(Name("A"), Int(1)).Pair(Name("B"), Int(2))
		},
		"<<\n  /A 1\n  /B 2\n>>",
	)
}

// TestStreams is test_streams in pdf-writer src/object.rs.
func TestStreams(t *testing.T) {
	w := NewPdf()
	w.Stream(NewRef(1), []byte("Hi there!")).Filter(FilterCrypt)
	testBytes(t,
		w.Finish(),
		"%PDF-1.7\n%\x80\x80\x80\x80\n",
		"1 0 obj",
		"<<\n  /Length 9\n  /Filter /Crypt\n>>",
		"stream",
		"Hi there!",
		"endstream",
		"endobj\n",
		"xref",
		"0 2",
		"0000000000 65535 f\r",
		"0000000016 00000 n\r",
		"trailer",
		"<<\n  /Size 2\n>>",
		"startxref\n94\n%%EOF",
	)
}

// TestArraysNoPretty is test_arrays_no_pretty in pdf-writer src/object.rs.
func TestArraysNoPretty(t *testing.T) {
	testObjNoPretty(t, func(obj Obj) { obj.Array() }, "[]")
	testObjNoPretty(t,
		func(obj Obj) {
			arr := obj.Array()

			arr.Item(Int(12)).
				Item(Name("Hi")).
				Item(Name("Hi2")).
				Item(Bool(false)).
				Item(TextStr("A string"))

			arr.Push().Dict().Pair(Name("Test1"), Int(23))

			arr.Item(Null{}).Item(Real(23.40))
		},
		"[12/Hi/Hi2 false(A string)<</Test1 23>>null 23.4]",
	)
}

// TestDictsNoPretty is test_dicts_no_pretty in pdf-writer src/object.rs.
func TestDictsNoPretty(t *testing.T) {
	testObjNoPretty(t, func(obj Obj) { obj.Dict() }, "<<>>")
	testObjNoPretty(t,
		func(obj Obj) {
			dict := obj.Dict()
			dict.Pair(Name("Key1"), Int(12)).
				Pair(Name("Key2"), Name("Hi")).
				Pair(Name("Key3"), Bool(false)).
				Pair(Name("Key4"), TextStr("A string"))
			dict.Insert(Name("Key5")).Array().Item(Int(23)).Item(Name("Value"))
			dict.Pair(Name("Key6"), Null{}).
				Pair(Name("Key7"), Real(23.40))
		},
		"<</Key1 12/Key2/Hi/Key3 false/Key4(A string)/Key5[23/Value]/Key6 null/Key7 23.4>>",
	)
}
