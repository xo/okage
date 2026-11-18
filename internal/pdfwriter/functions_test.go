// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"testing"
)

// The tests of pdf-writer src/functions.rs.

// TestPostScriptEncoding is test_post_script_encoding in pdf-writer
// src/functions.rs.
func TestPostScriptEncoding(t *testing.T) {
	ops := []PostScriptOp{
		PostScriptOpReal(3.0),
		PostScriptOpReal(2.0),
		PostScriptOpMul,
		PostScriptOpExch,
		PostScriptOpDup,
		PostScriptOpReal(0.0),
		PostScriptOpGe,
		PostScriptOpIfElse([]PostScriptOp{PostScriptOpReal(1.0), PostScriptOpAdd}, []PostScriptOp{PostScriptOpNeg}),
		PostScriptOpAdd,
	}

	testBytes(t, PostScriptOpEncode(ops).AsSlice(),
		"{ 3.0 2.0 mul exch dup 0.0 ge { 1.0 add } {neg} ifelse add }")
}
