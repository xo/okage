// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"testing"
)

// The one test of pdf-writer src/structure.rs, test_max_heading_name, tests
// StructRole2 of tagged PDF, which has no port. The tests below test the
// example of the package comment of src/lib.rs, and the asserts and the
// methods that take the writer by value in Rust.

// TestMinimalExample writes the example "Minimal example" of the package
// comment in pdf-writer src/lib.rs: a document with a single, empty A4
// page.
func TestMinimalExample(t *testing.T) {
	catalogID := NewRef(1)
	pageTreeID := NewRef(2)
	pageID := NewRef(3)
	testBytes(t,
		slice(func(pdf *Pdf) {
			pdf.Catalog(catalogID).Pages(pageTreeID)
			pdf.Pages(pageTreeID).Kids(pageID).Count(1)
			pdf.Page(pageID).
				Parent(pageTreeID).
				MediaBox(NewRect(0.0, 0.0, 595.0, 842.0)).
				Resources()
		}, DefaultSettings()),
		"1 0 obj",
		"<<",
		"  /Type /Catalog",
		"  /Pages 2 0 R",
		">>",
		"endobj\n",
		"2 0 obj",
		"<<",
		"  /Type /Pages",
		"  /Kids [3 0 R]",
		"  /Count 1",
		">>",
		"endobj\n",
		"3 0 obj",
		"<<",
		"  /Type /Page",
		"  /Parent 2 0 R",
		"  /MediaBox [0 0 595 842]",
		"  /Resources <<>>",
		">>",
		"endobj\n\n",
	)
}

// TestDestinationFinishes makes sure that a method that takes a Destination
// by value in Rust finishes the array, so that no item can follow it.
func TestDestinationFinishes(t *testing.T) {
	c := NewChunkWithSettings(Settings{Pretty: false})
	d := c.Destination(NewRef(1)).Page(NewRef(2))
	zoom := float32(1.5)
	d.XYZ(10, 20.5, &zoom)
	mustPanic(t, "the Array is finished", func() { d.Item(Int(1)) })
	c.Destination(NewRef(3)).Page(NewRef(2)).XYZ(0, 1, nil)
	item := c.OutlineItem(NewRef(4))
	item.Dest().Page(NewRef(2)).FitRect(NewRect(1, 2, 3, 4))
	item.Title(TextStr("A"))
	want := "1 0 obj\n[2 0 R/XYZ 10 20.5 1.5]\nendobj\n" +
		"3 0 obj\n[2 0 R/XYZ 0 1 0]\nendobj\n" +
		"4 0 obj\n<</Dest[2 0 R/FitR 1 2 3 4]/Title(A)>>\nendobj\n"
	if got := string(c.AsBytes()); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestAsserts tests the asserts of upstream, which panic with the message of
// upstream.
func TestAsserts(t *testing.T) {
	c := NewChunk()
	mustPanic(t, "visible outline count must not be negative", func() {
		c.Outline(NewRef(1)).Count(-1)
	})
	pdf := NewPdf()
	mustPanic(t, "mode must not full screen", func() {
		pdf.Catalog(NewRef(1)).ViewerPreferences().NonFullScreenPageMode(PageModeFullScreen)
	})
}
