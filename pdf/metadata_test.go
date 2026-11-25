// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/okage"
)

// The tests of this file port the snapshot tests of krilla
// tests/src/metadata.rs and the test of src/interchange/metadata/mod.rs. A
// snapshot with XMP metadata is compared without its XMP stream, with
// checkSnapshotWithoutXMP, because the base port writes no XMP metadata
// (D62). The snapshot metadata_full_with_xmp is metadata_full with XMP
// metadata, and metadata_version_id_pdf_a1 needs the validator of PDF/A-1,
// so they are left out.

// datetime is datetime of krilla tests/src/metadata.rs.
func datetime() DateTime {
	return NewDateTime(2024).
		Month(11).
		Day(8).
		Hour(22).
		Minute(23).
		Second(18).
		UTCOffsetHour(1).
		UTCOffsetMinute(12)
}

// metadataImpl is metadata_impl of krilla tests/src/metadata.rs.
func metadataImpl(d *Document) {
	metadata := NewMetadata().
		CreationDate(datetime()).
		Description("A very interesting subject").
		Creator("krilla").
		Producer("krilla").
		Language("en").
		Keywords([]string{"keyword1", "keyword2", "keyword3"}).
		Title("An awesome title").
		Authors([]string{"John Doe", "Max Mustermann"}).
		TextDirection(TextDirectionMetadataLeftToRight).
		PageLayout(PageLayoutTwoColumnRight).
		DisplayDocTitle(true).
		Trapped(true)
	d.SetMetadata(metadata)
}

// TestMetadataEmpty is the snapshot test metadata_empty of krilla
// tests/src/metadata.rs.
func TestMetadataEmpty(t *testing.T) {
	d := NewDocument(settings1()...)
	d.SetMetadata(NewMetadata())
	checkSnapshot(t, "metadata_empty", d)
}

// TestMetadataFull is the snapshot test metadata_full of krilla
// tests/src/metadata.rs. The ID of the document is the hash of the version,
// the title and the authors.
func TestMetadataFull(t *testing.T) {
	d := NewDocument(settings1()...)
	metadataImpl(d)
	checkSnapshot(t, "metadata_full", d)
}

// customMetadata is custom_metadata of krilla tests/src/metadata.rs.
func customMetadata(t *testing.T) Metadata {
	t.Helper()
	m := NewMetadata().Title("Standard title").Trapped(false)
	for _, f := range [][2]string{
		{"CustomField", "old value"},
		{"CustomField", "new value"},
		{"Custom/Key", "Custom value"},
	} {
		var ok bool
		if m, ok = m.CustomField(f[0], f[1]); !ok {
			t.Fatalf("the custom field %q is not accepted", f[0])
		}
	}
	return m
}

// TestMetadataCustomFields is the snapshot test metadata_custom_fields of
// krilla tests/src/metadata.rs.
func TestMetadataCustomFields(t *testing.T) {
	d := NewDocument(settings1()...)
	d.SetMetadata(customMetadata(t))
	checkSnapshot(t, "metadata_custom_fields", d)
}

// TestMetadataVersionID is the snapshot test metadata_version_id of krilla
// tests/src/metadata.rs, with settings_5. The version ID goes only into the
// XMP metadata, so the file has no information dictionary.
func TestMetadataVersionID(t *testing.T) {
	d := NewDocument(settings1()...)
	d.SetMetadata(NewMetadata().VersionID("1.2.3"))
	checkSnapshotWithoutXMP(t, "metadata_version_id", d)
}

// TestMetadataPDF20Author is the snapshot test metadata_pdf_20_author of
// krilla tests/src/metadata.rs, with settings_30. PDF 2.0 writes the dates
// and not the authors into the information dictionary.
func TestMetadataPDF20Author(t *testing.T) {
	d := NewDocument(append(settings1(), WithPDFVersion(PDF20))...)
	d.SetMetadata(NewMetadata().
		Authors([]string{"John Doe", "Max Mustermann"}).
		CreationDate(datetime()))
	checkSnapshotWithoutXMP(t, "metadata_pdf_20_author", d)
}

// TestCustomFieldReturnsNoneForIgnoredNames is the test
// custom_field_returns_none_for_ignored_names of krilla
// src/interchange/metadata/mod.rs.
func TestCustomFieldReturnsNoneForIgnoredNames(t *testing.T) {
	if _, ok := NewMetadata().CustomField("", "value"); ok {
		t.Error("an empty name is accepted")
	}
	if _, ok := NewMetadata().CustomField("Title", "value"); ok {
		t.Error("the name Title is accepted")
	}
	m, ok := NewMetadata().CustomField("CustomField", "value")
	if !ok {
		t.Fatal("the name CustomField is not accepted")
	}
	if got := m.customFields["CustomField"]; got != "value" {
		t.Errorf("the field has the value %q, want %q", got, "value")
	}
}

// TestMetadataCopies checks that a method of Metadata gives a copy, which
// shares no slice and no map with the metadata it came from.
func TestMetadataCopies(t *testing.T) {
	keywords := []string{"a", "b"}
	m1 := NewMetadata().Keywords(keywords)
	keywords[0] = "changed"
	if m1.keywords[0] != "a" {
		t.Error("the keywords share the slice of the caller")
	}
	m2, _ := m1.CustomField("A", "1")
	m3, _ := m2.CustomField("B", "2")
	if len(m2.customFields) != 1 || len(m3.customFields) != 2 {
		t.Errorf("the custom fields are shared: %v and %v", m2.customFields, m3.customFields)
	}
}

// TestMetadataInfo checks the information dictionary for the other fields
// and versions: the document ID, a date with few fields, a language that
// is empty and so ignored, the trapping status, a page layout that needs
// PDF 1.5, and text in UTF-16.
func TestMetadataInfo(t *testing.T) {
	cases := []struct {
		name    string
		version PDFVersion
		m       Metadata
		want    []string
		notWant []string
	}{
		{
			name:    "document id",
			version: PDF17,
			m:       NewMetadata().DocumentID("id").Title("Ünïcödé"),
			want:    []string{"/Title <FEFF00DC006E00EF006300F6006400E9>", "/Info 5 0 R"},
		},
		{
			name:    "short date",
			version: PDF17,
			m:       NewMetadata().CreationDate(NewDateTime(65535).Month(0).Day(40).UTCOffsetHour(-50)),
			want:    []string{"/CreationDate (D:99990131000000-23'00)"},
		},
		{
			name:    "empty language",
			version: PDF17,
			m:       NewMetadata().Language(""),
			notWant: []string{"/Lang", "/Info"},
		},
		{
			name:    "empty language after a language",
			version: PDF17,
			m:       NewMetadata().Language("en").Language(""),
			want:    []string{"/Lang (en)"},
			notWant: []string{"/Lang ()"},
		},
		{
			name:    "not trapped",
			version: PDF17,
			m:       NewMetadata().Trapped(false),
			want:    []string{"/Trapped /False", "/Info"},
		},
		{
			name:    "trapped on PDF 2.0",
			version: PDF20,
			m:       NewMetadata().Trapped(true),
			notWant: []string{"/Trapped", "/Info"},
		},
		{
			name:    "two pages on PDF 1.4",
			version: PDF14,
			m:       NewMetadata().PageLayout(PageLayoutTwoPageLeft).TextDirection(TextDirectionMetadataRightToLeft),
			want:    []string{"/Direction /R2L"},
			notWant: []string{"/PageLayout", "/DisplayDocTitle"},
		},
		{
			name:    "two pages on PDF 1.5",
			version: PDF15,
			m:       NewMetadata().PageLayout(PageLayoutTwoPageRight),
			want:    []string{"/PageLayout /TwoPageRight"},
			notWant: []string{"/ViewerPreferences"},
		},
		{
			name:    "PDF 2.0 without dates",
			version: PDF20,
			m:       NewMetadata().Title("t").Authors([]string{"a"}),
			notWant: []string{"/Info", "/Title"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := NewDocument(append(settings1(), WithPDFVersion(c.version))...)
			d.SetMetadata(c.m)
			var buf bytes.Buffer
			if err := d.Finish(&buf); err != nil {
				t.Fatal(err)
			}
			for _, w := range c.want {
				if !bytes.Contains(buf.Bytes(), []byte(w)) {
					t.Errorf("the file has no %q:\n%s", w, buf.Bytes())
				}
			}
			for _, w := range c.notWant {
				if bytes.Contains(buf.Bytes(), []byte(w)) {
					t.Errorf("the file has %q:\n%s", w, buf.Bytes())
				}
			}
		})
	}
}

// snapshotObject matches an object of a file of the snapshots, xmpRef the
// key of the catalog that names the XMP metadata, and pdfName a name.
var (
	snapshotObject = regexp.MustCompile(`(?s)\d+ 0 obj\n.*?\nendobj\n`)
	xmpRef         = regexp.MustCompile(`\n  /Metadata \d+ 0 R`)
	pdfName        = regexp.MustCompile(`/[A-Za-z]+`)
)

// objectsWithoutXMP returns the objects of a file of the snapshots, without
// their numbers, without the object of the XMP metadata and without the key
// of the catalog that names it.
func objectsWithoutXMP(file []byte) []string {
	var objects []string
	for _, o := range snapshotObject.FindAllString(string(file), -1) {
		if strings.Contains(o, "/Type /Metadata") {
			continue
		}
		o = o[strings.Index(o, "\n")+1:]
		objects = append(objects, xmpRef.ReplaceAllString(o, ""))
	}
	return objects
}

// checkSnapshotWithoutXMP compares the objects of the file with the objects
// of the snapshot of krilla, for a snapshot with XMP metadata. The object of
// the XMP metadata of the snapshot shifts the numbers of the objects after
// it, so the objects are compared in their order, without their numbers.
// The trailers must have the same keys. The cross-reference table and the
// ID, which hashes the XMP metadata, are not compared.
func checkSnapshotWithoutXMP(t *testing.T, name string, d *Document) {
	t.Helper()
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "snapshots", name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	got, wantObjects := objectsWithoutXMP(buf.Bytes()), objectsWithoutXMP(want)
	if len(got) != len(wantObjects) {
		t.Fatalf("%s has %d objects without XMP metadata, want %d:\n%s", name, len(got), len(wantObjects), buf.Bytes())
	}
	for i := range got {
		if got[i] != wantObjects[i] {
			t.Errorf("%s differs in the object %d:\ngot  %q\nwant %q", name, i+1, got[i], wantObjects[i])
		}
	}
	trailerKeys := func(b []byte) string {
		trailer := b[bytes.LastIndex(b, []byte("trailer")):]
		return strings.Join(pdfName.FindAllString(string(trailer), -1), " ")
	}
	if g, w := trailerKeys(buf.Bytes()), trailerKeys(want); g != w {
		t.Errorf("%s has the trailer keys %s, want %s", name, g, w)
	}
}

// TestMetadataErrors checks that SetMetadata after Finish does nothing, and
// that Finish still returns its error.
func TestMetadataErrors(t *testing.T) {
	d := NewDocument(settings1()...)
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	d.SetMetadata(NewMetadata().Title("late"))
	if err := d.Finish(&buf); !errors.Is(err, okage.ErrPDFFinished) {
		t.Errorf("a second Finish gives %v, want ErrPDFFinished", err)
	}
}
