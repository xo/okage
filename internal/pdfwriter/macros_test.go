// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"bytes"
	"testing"
)

// The macros of the tests in pdf-writer src/macros.rs.

// testBytes is the macro test! in pdf-writer src/macros.rs.
//
// Test that buf is the same as the result of concatenating the strings.
func testBytes(t *testing.T, buf []byte, lines ...string) {
	t.Helper()
	expected := []byte(joinLines(lines))
	if !bytes.Equal(buf, expected) {
		t.Errorf("=========== EXPECTED =============\n%s\n============= FOUND ==============\n%s\n=============================\n%q\n%q",
			expected, buf, expected, buf)
	}
}

// joinLines joins the lines with a newline, as the macro test! does.
func joinLines(lines []string) string {
	var b bytes.Buffer
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(l)
	}
	return b.String()
}

// testObj is the macro test_obj! in pdf-writer src/macros.rs.
//
// Test how an object is written.
func testObj(t *testing.T, write func(obj Obj), lines ...string) {
	t.Helper()
	testBytes(t, sliceObj(write, DefaultSettings()), lines...)
}

// testObjNoPretty is the macro test_obj_no_pretty! in pdf-writer
// src/macros.rs.
//
// Test how an object is written, without pretty-printing.
func testObjNoPretty(t *testing.T, write func(obj Obj), lines ...string) {
	t.Helper()
	testBytes(t, sliceObj(write, Settings{Pretty: false}), lines...)
}

// testPrimitive is the macro test_primitive! in pdf-writer src/macros.rs.
//
// Test how a primitive object is written.
func testPrimitive(t *testing.T, value Primitive, lines ...string) {
	t.Helper()
	testObj(t, func(obj Obj) { obj.Primitive(value) }, lines...)
}
