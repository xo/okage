// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"testing"
)

// The tests of pdf-writer src/annotations.rs.

// TestAnnotations is test_annotations in pdf-writer src/annotations.rs.
func TestAnnotations(t *testing.T) {
	testBytes(t,
		slice(func(w *Pdf) {
			w.Annotation(NewRef(1)).Rect(NewRect(0.0, 0.0, 1.0, 1.0))
			w.Annotation(NewRef(2)).Rect(NewRect(1.0, 1.0, 0.0, 0.0))
		}, DefaultSettings()),
		"1 0 obj",
		"<<",
		"  /Type /Annot",
		"  /Rect [0 0 1 1]",
		">>",
		"endobj\n",
		"2 0 obj",
		"<<",
		"  /Type /Annot",
		"  /Rect [1 1 0 0]",
		">>",
		"endobj\n\n",
	)
}
