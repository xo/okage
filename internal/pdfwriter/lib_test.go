// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"math"
	"testing"
)

// The tests of pdf-writer src/lib.rs.

// slice is the function slice of the tests in pdf-writer src/lib.rs.
//
// Return the slice of bytes written during the execution of f.
func slice(f func(w *Pdf), settings Settings) []byte {
	w := NewPdfWithSettings(settings)
	start := w.Len()
	f(w)
	end := w.Len()
	buf := w.Finish()
	return append([]byte(nil), buf[start:end]...)
}

// sliceObj is the function slice_obj of the tests in pdf-writer src/lib.rs.
//
// Return the slice of bytes written for an object.
func sliceObj(f func(obj Obj), settings Settings) []byte {
	buf := slice(func(w *Pdf) { f(w.Indirect(NewRef(1))) }, settings)
	if settings.Pretty {
		return buf[8 : len(buf)-9]
	}
	return buf[8 : len(buf)-8]
}

// TestMinimal is test_minimal in pdf-writer src/lib.rs.
func TestMinimal(t *testing.T) {
	w := NewPdf()
	testBytes(t,
		w.Finish(),
		"%PDF-1.7\n%\x80\x80\x80\x80\n",
		"xref\n0 1\n0000000000 65535 f\r",
		"trailer\n<<\n  /Size 1\n>>",
		"startxref\n16\n%%EOF",
	)
}

// TestXrefFreeListShort is test_xref_free_list_short in pdf-writer
// src/lib.rs.
func TestXrefFreeListShort(t *testing.T) {
	w := NewPdf()
	w.Indirect(NewRef(1)).Primitive(Int(1))
	w.Indirect(NewRef(2)).Primitive(Int(2))
	testBytes(t,
		w.Finish(),
		"%PDF-1.7\n%\x80\x80\x80\x80\n",
		"1 0 obj\n1\nendobj\n",
		"2 0 obj\n2\nendobj\n",
		"xref",
		"0 3",
		"0000000000 65535 f\r",
		"0000000016 00000 n\r",
		"0000000034 00000 n\r",
		"trailer",
		"<<\n  /Size 3\n>>",
		"startxref\n52\n%%EOF",
	)
}

// TestXrefFreeListLong is test_xref_free_list_long in pdf-writer
// src/lib.rs.
func TestXrefFreeListLong(t *testing.T) {
	w := NewPdf()
	w.SetVersion(1, 4)
	w.Indirect(NewRef(1)).Primitive(Int(1))
	w.Indirect(NewRef(2)).Primitive(Int(2))
	w.Indirect(NewRef(5)).Primitive(Int(5))
	testBytes(t,
		w.Finish(),
		"%PDF-1.4\n%\x80\x80\x80\x80\n",
		"1 0 obj\n1\nendobj\n",
		"2 0 obj\n2\nendobj\n",
		"5 0 obj\n5\nendobj\n",
		"xref",
		"0 6",
		"0000000003 65535 f\r",
		"0000000016 00000 n\r",
		"0000000034 00000 n\r",
		"0000000004 00000 f\r",
		"0000000000 00000 f\r",
		"0000000052 00000 n\r",
		"trailer",
		"<<\n  /Size 6\n>>",
		"startxref\n70\n%%EOF",
	)
}

// TestXrefFreeListDuplicate is test_xref_free_list_duplicate in pdf-writer
// src/lib.rs.
func TestXrefFreeListDuplicate(t *testing.T) {
	defer func() {
		if r := recover(); r != "duplicate indirect reference id: 3" {
			t.Errorf("the panic is %v", r)
		}
	}()
	w := NewPdf()
	w.Indirect(NewRef(3)).Primitive(Int(1))
	w.Indirect(NewRef(5)).Primitive(Int(2))
	w.Indirect(NewRef(13)).Primitive(Int(1))
	w.Indirect(NewRef(3)).Primitive(Int(1))
	w.Indirect(NewRef(6)).Primitive(Int(2))
	w.Finish()
}

// TestBinaryMarker is test_binary_marker in pdf-writer src/lib.rs.
func TestBinaryMarker(t *testing.T) {
	w := NewPdf()
	w.SetBinaryMarker([4]byte{'A', 'B', 'C', 'D'})
	testBytes(t,
		w.Finish(),
		"%PDF-1.7\n%ABCD\n",
		"xref\n0 1\n0000000000 65535 f\r",
		"trailer\n<<\n  /Size 1\n>>",
		"startxref\n16\n%%EOF",
	)
}

// TestFieldWidth is field_width in pdf-writer src/lib.rs.
func TestFieldWidth(t *testing.T) {
	for _, c := range []struct {
		offset uint
		want   uint32
	}{
		{128, 1},
		{255, 1},
		{256, 2},
		{math.MaxUint16, 2},
		{math.MaxUint16 + 1, 3},
		{math.MaxUint32, 4},
	} {
		if got := determineFieldWidth(c.offset); got != c.want {
			t.Errorf("determineFieldWidth(%d) = %d, want %d", c.offset, got, c.want)
		}
	}
}

// TestXrefStream is test_xref_stream in pdf-writer src/lib.rs.
func TestXrefStream(t *testing.T) {
	w := NewPdf()
	w.Indirect(NewRef(1)).Primitive(Int(1))
	w.Indirect(NewRef(2)).Primitive(Int(2))
	w.Indirect(NewRef(5)).Primitive(Int(5))
	testBytes(t,
		w.FinishWithXRefStream(NewRef(6)),
		"%PDF-1.7\n%\x80\x80\x80\x80\n",
		"1 0 obj\n1\nendobj\n",
		"2 0 obj\n2\nendobj\n",
		"5 0 obj\n5\nendobj\n",
		"6 0 obj\n<<\n  /Length 28\n  /Type /XRef\n  /Size 7\n  /W [1 1 2]\n>>\nstream",
		// [0, 3, 255, 255], [1, 16, 0, 0], [1, 34, 0, 0], [0, 4, 0, 0], [0, 0, 0, 0], [1, 52, 0, 0], [1, 70, 0, 0]
		"\x00\x03\xFF\xFF\x01\x10\x00\x00\x01\x22\x00\x00\x00\x04\x00\x00\x00\x00\x00\x00\x01\x34\x00\x00\x01\x46\x00\x00",
		"endstream\nendobj\n",
		"startxref\n70\n%%EOF",
	)
}

// TestXrefStreamSingleFilter is test_xref_stream_single_filter in
// pdf-writer src/lib.rs.
func TestXrefStreamSingleFilter(t *testing.T) {
	w := NewPdf()
	w.Indirect(NewRef(1)).Primitive(Int(1))
	testBytes(t,
		w.FinishWithXRefStreamAndFilter(NewRef(2), func([]byte) ([]byte, XRefFilter) {
			return []byte("ABCDEFGH"), XRefFilterSingle(FilterFlateDecode)
		}),
		"%PDF-1.7\n%\x80\x80\x80\x80\n",
		"1 0 obj\n1\nendobj\n",
		"2 0 obj\n<<\n  /Length 8\n  /Type /XRef\n  /Filter /FlateDecode\n  /Size 3\n  /W [1 1 2]\n>>\nstream",
		"ABCDEFGH",
		"endstream\nendobj\n",
		"startxref\n34\n%%EOF",
	)
}

// TestXrefStreamMultipleFilters is test_xref_stream_multiple_filters in
// pdf-writer src/lib.rs.
func TestXrefStreamMultipleFilters(t *testing.T) {
	w := NewPdf()
	w.Indirect(NewRef(1)).Primitive(Int(1))
	testBytes(t,
		w.FinishWithXRefStreamAndFilter(NewRef(2), func([]byte) ([]byte, XRefFilter) {
			return []byte("ABCDEFGH"), XRefFilterMultiple(FilterASCIIHexDecode, FilterFlateDecode)
		}),
		"%PDF-1.7\n%\x80\x80\x80\x80\n",
		"1 0 obj\n1\nendobj\n",
		"2 0 obj\n<<\n  /Length 8\n  /Type /XRef\n  /Filter [/ASCIIHexDecode /FlateDecode]\n  /Size 3\n  /W [1 1 2]\n>>\nstream",
		"ABCDEFGH",
		"endstream\nendobj\n",
		"startxref\n34\n%%EOF",
	)
}

// TestXrefWidth2 is test_xref_width2 in pdf-writer src/lib.rs.
func TestXrefWidth2(t *testing.T) {
	w := NewPdf()
	zeros := make([]byte, 256)
	for i := range zeros {
		zeros[i] = '0'
	}
	w.Stream(NewRef(1), zeros)
	w.Indirect(NewRef(2)).Primitive(Int(1))
	testBytes(t,
		w.FinishWithXRefStream(NewRef(3)),
		"%PDF-1.7\n%\x80\x80\x80\x80\n",
		"1 0 obj\n<<\n  /Length 256\n>>\nstream",
		string(zeros),
		"endstream\nendobj\n",
		"2 0 obj\n1\nendobj\n",
		"3 0 obj\n<<\n  /Length 20\n  /Type /XRef\n  /Size 4\n  /W [1 2 2]\n>>\nstream",
		// [0, 0, 0, 255, 255], [1, 0, 16, 0, 0], [1, 0, 34, 0, 0], [1, 1, 32, 0, 0]
		"\x00\x00\x00\xFF\xFF\x01\x00\x10\x00\x00\x01\x01\x46\x00\x00\x01\x01\x58\x00\x00",
		"endstream\nendobj\n",
		"startxref\n344\n%%EOF",
	)
}

// TestTrailer makes sure that the catalog, the document information and the
// file identifier go into the trailer, as TrailerData::write_into_dict
// writes them. The writers Catalog and DocumentInfo of src/structure.rs
// start on the objects that startCatalog and startDocumentInfo give.
func TestTrailer(t *testing.T) {
	w := NewPdfWithSettings(Settings{Pretty: false})
	w.SetFileID([]byte("a"), []byte("\xff"))
	w.startCatalog(NewRef(1)).Dict().Pair(Name("Type"), Name("Catalog"))
	w.startDocumentInfo(NewRef(2)).Dict().Pair(Name("Title"), TextStr("T"))
	testBytes(t,
		w.Finish(),
		"%PDF-1.7\n%\x80\x80\x80\x80\n",
		"1 0 obj",
		"<</Type/Catalog>>",
		"endobj",
		"2 0 obj",
		"<</Title(T)>>",
		"endobj",
		"xref",
		"0 3",
		"0000000000 65535 f\r",
		"0000000016 00000 n\r",
		"0000000049 00000 n\r",
		"trailer",
		"<</Size 3/Root 1 0 R/Info 2 0 R/ID[(a)<FF>]>>",
		"startxref\n78\n%%EOF",
	)
}
