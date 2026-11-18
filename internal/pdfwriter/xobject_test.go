// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import "testing"

// TestChunkLimits is test_chunk_limits in pdf-writer src/buf.rs. It waited
// for the writers of src/xobject.rs.
func TestChunkLimits(t *testing.T) {
	limits := NewLimits()

	chunk := NewChunk()
	xObject := chunk.FormXObject(NewRef(1), []byte{})
	xObject.BBox(NewRect(4.0, 6.0, 22.1, 31.0))
	xObject.Finish()

	limits.Merge(chunk.Limits())

	want := Limits{int: 31, real: 22.1, nameLen: 7, strLen: 0, arrayLen: 4, dictEntries: 4}
	if limits != want {
		t.Errorf("the limits are %+v, want %+v", limits, want)
	}
}
