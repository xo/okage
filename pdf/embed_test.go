// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The tests of this file port the tests of krilla tests/src/embed.rs and of
// src/interchange/embed.rs (D99). testdata/files holds emojis.txt and the
// SVG file of assets/svgs of krilla, byte for byte, and the images are the
// files of testdata/images. The test embedded_file_pdf_a2 needs a tag tree,
// which pdf does not have (D62), so it is left out. The tests
// embedded_file_before_metadata and embedded_file_pdf_a3b_missing_date set
// an empty tag tree in upstream. Without it, PDF/A-3b does not prohibit the
// missing tags, so the results are the same.

// readTestFile reads a file of testdata.
func readTestFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// embeddedTestFile makes a file of the tests of krilla tests/src/embed.rs.
func embeddedTestFile(t *testing.T, path, mimeType, description string, kind AssociationKind, data []byte) EmbeddedFile {
	t.Helper()
	m, ok := NewMIMEType(mimeType)
	if !ok {
		t.Fatalf("the mime type %q is not valid", mimeType)
	}
	date := NewDateTime(2001)
	compress := false
	return EmbeddedFile{
		Path:             path,
		MIMEType:         m,
		Description:      &description,
		AssociationKind:  kind,
		Data:             data,
		ModificationDate: &date,
		Compress:         &compress,
	}
}

// file1 is file_1 of krilla tests/src/embed.rs.
func file1(t *testing.T) EmbeddedFile {
	t.Helper()
	return embeddedTestFile(t, "emojis.txt", "text/txt", "The description of the file.",
		AssociationKindSupplement, readTestFile(t, "files/emojis.txt"))
}

// file2 is file_2 of krilla tests/src/embed.rs.
func file2(t *testing.T) EmbeddedFile {
	t.Helper()
	return embeddedTestFile(t, "image.svg", "image/svg+xml", "A nice SVG image!",
		AssociationKindSupplement, readTestFile(t, "files/resvg_structure_svg_nested_svg_with_rect.svg"))
}

// file3 is file_3 of krilla tests/src/embed.rs.
func file3(t *testing.T) EmbeddedFile {
	t.Helper()
	return embeddedTestFile(t, "rgb8.png", "image/png", "A nice picture.",
		AssociationKindUnspecified, readTestFile(t, "images/rgb8.png"))
}

// file4 is file_4 of krilla tests/src/embed.rs.
func file4(t *testing.T) EmbeddedFile {
	t.Helper()
	return embeddedTestFile(t, "rgb8.gif", "image/gif", "A nice gif.",
		AssociationKindUnspecified, readTestFile(t, "images/rgb8.gif"))
}

// emptyFile is empty_file of krilla tests/src/embed.rs.
func emptyFile(t *testing.T) EmbeddedFile {
	t.Helper()
	f := embeddedTestFile(t, "empty_file", "text/plain", "A zero-byte file.", AssociationKindSupplement, nil)
	f.Compress = nil
	return f
}

// metadata1 is metadata_1 of krilla tests/src/main.rs.
func metadata1() Metadata {
	return NewMetadata().Language("en").CreationDate(NewDateTime(2001))
}

// TestEmbeddedFileSnapshots ports the snapshot tests of embedded files of
// krilla tests/src/embed.rs. A compressed file is compared with
// checkSnapshotDecoded, because the compressed bytes differ.
func TestEmbeddedFileSnapshots(t *testing.T) {
	yes := true
	tests := []struct {
		name    string
		opts    []Option
		decoded bool
		files   func(t *testing.T) []EmbeddedFile
	}{
		{"embedded_file", nil, false, func(t *testing.T) []EmbeddedFile { t.Helper(); return []EmbeddedFile{file1(t)} }},
		{"embedded_file_with_compression", nil, true, func(t *testing.T) []EmbeddedFile {
			t.Helper()
			f := file1(t)
			f.Compress = &yes
			return []EmbeddedFile{f}
		}},
		{"embedded_file_with_auto_compression_success", nil, true, func(t *testing.T) []EmbeddedFile {
			t.Helper()
			f := file1(t)
			f.Compress = nil
			return []EmbeddedFile{f}
		}},
		{"embedded_file_with_auto_compression_fail", nil, false, func(t *testing.T) []EmbeddedFile {
			t.Helper()
			f := file4(t)
			f.Compress = nil
			return []EmbeddedFile{f}
		}},
		{"embedded_file_multiple", nil, false, func(t *testing.T) []EmbeddedFile {
			t.Helper()
			return []EmbeddedFile{file1(t), file2(t), file3(t)}
		}},
		{"embedded_zero_byte_file", nil, true, func(t *testing.T) []EmbeddedFile { t.Helper(); return []EmbeddedFile{emptyFile(t)} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDocument(append(settings1(), tt.opts...)...)
			for _, f := range tt.files(t) {
				d.EmbedFile(f)
			}
			if tt.decoded {
				checkSnapshotDecoded(t, tt.name, d)
			} else {
				checkSnapshot(t, tt.name, d)
			}
		})
	}
}

// TestEmbeddedFilePDF20 is the snapshot test embedded_file_pdf_20 of krilla
// tests/src/embed.rs, with settings_25. PDF 2.0 supports associated files,
// so we expect them to appear.
func TestEmbeddedFilePDF20(t *testing.T) {
	d := NewDocument(append(settings1(), WithPDFVersion(PDF20))...)
	d.SetMetadata(metadata1())
	d.EmbedFile(file1(t))
	checkSnapshot(t, "embedded_file_pdf_20", d)
}

// TestEmbeddedFileDuplicate is the test embedded_file_duplicate of krilla
// tests/src/embed.rs.
func TestEmbeddedFileDuplicate(t *testing.T) {
	d := NewDocument()
	f1 := file1(t)
	f2 := file2(t)
	f2.Path = f1.Path

	if !d.EmbedFile(f1) {
		t.Error("the first file is not embedded")
	}
	if d.EmbedFile(f2) {
		t.Error("the second file with the same name is embedded")
	}
}

// TestEmbeddedFileBeforeMetadata is the test embedded_file_before_metadata
// of krilla tests/src/embed.rs. See
// https://github.com/typst/typst/issues/6758.
func TestEmbeddedFileBeforeMetadata(t *testing.T) {
	d := NewDocument(settingsArchival(ArchivalA3B)...)
	d.EmbedFile(file1(t))
	d.SetMetadata(metadata1())
	if err := d.Finish(&bytes.Buffer{}); err != nil {
		t.Error(err)
	}
}

// TestEmbeddedFilePDFA3BMissingDate is the test
// embedded_file_pdf_a3b_missing_date of krilla tests/src/embed.rs.
func TestEmbeddedFilePDFA3BMissingDate(t *testing.T) {
	d := NewDocument(settingsArchival(ArchivalA3B)...)
	d.SetMetadata(metadata1())
	f1 := file1(t)
	f1.ModificationDate = nil
	d.EmbedFile(f1)

	errs := validationErrorsOf(t, d.Finish(&bytes.Buffer{}))
	want := []validationError{{kind: validationEmbeddedFile, embed: embedErrorMissingDate}}
	if !slices.Equal(errs, want) {
		t.Errorf("got %+v, want %+v", errs, want)
	}
}

// TestValidMIMETypes is the test valid_mime_types of krilla
// src/interchange/embed.rs.
func TestValidMIMETypes(t *testing.T) {
	for _, m := range []string{
		"text/plain", "application/pdf", "image/jpeg", "application/octet-stream", "text/html",
		"application/json", "image/png", "video/mp4", "audio/mpeg", "application/vnd.ms-excel",
		"text/css", "application/x-www-form-urlencoded", "multipart/form-data",
		"application/xml+svg", "text/plain+charset",
	} {
		if !validMIMEType(m) {
			t.Errorf("%q is not valid", m)
		}
	}
}

// TestInvalidMIMETypes is the test invalid_mime_types of krilla
// src/interchange/embed.rs.
func TestInvalidMIMETypes(t *testing.T) {
	for _, m := range []string{
		"source", "text/", "/plain", "", "text", "text/plain/extra", "text plain", `text\plain`,
		"text@plain", "text/plain*",
	} {
		if validMIMEType(m) {
			t.Errorf("%q is valid", m)
		}
	}
}
