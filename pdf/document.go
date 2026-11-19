// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"
	"io"

	"github.com/xo/okage"
)

// Location is Location in krilla src/surface.rs, a NonZeroU64. The port
// declares it here, because Document.SetLocation takes it. Zero is no
// location, as the None of an Option<Location> of upstream.
//
// A location that a program gives to the drawing operations that follow, so
// that an error of the export names where it comes from.
type Location uint64

// Document is Document in krilla src/document.rs.
//
// A PDF document.
//
// Creating PDFs always requires some kind of global state to keep track of
// different aspects in the creation process, meaning that it is not possible
// to generate multiple pages at the same time. Instead, you add pages one
// after the other with the method StartPage. Rust makes sure that a page is
// finished before the next one starts. Go cannot, so StartPage and Finish
// finish the page that is open.
//
// The methods SetMetadata, SetOutline and RegisterNamedDestination are in
// metadata.go, outline.go and destination.go. The base port leaves out
// embed_pdf_pages, set_field_tree, set_tag_tree and embed_file (D62).
type Document struct {
	sc *serializeContext
	cc *chunkContainer
	// page is the page that is open, or nil.
	page     *Page
	finished bool
}

// NewDocument is Document::new_with in krilla src/document.rs. The options
// are the fields of SerializeSettings, and no option gives the settings of
// Document::new.
//
// Create a new document with custom serialize settings.
func NewDocument(opts ...Option) *Document {
	settings := defaultSerializeSettings()
	for _, opt := range opts {
		opt(&settings)
	}
	sc := newSerializeContext(settings)
	return &Document{
		sc: sc,
		cc: newChunkContainer(sc),
	}
}

// StartPage is Document::start_page_with in krilla src/document.rs. It
// finishes the page that is open first. DefaultPageSettings gives the
// settings of Document::start_page. It panics after Finish, as Rust cannot
// call a method on a document that it has finished.
//
// Start a new page with specific page settings.
func (d *Document) StartPage(pageSettings PageSettings) *Page {
	if d.finished {
		panic("pdf: StartPage called after Finish")
	}
	if d.page != nil {
		d.page.Finish()
	}
	pageIndex := len(d.sc.pageInfos)
	d.page = newPage(d, pageIndex, pageSettings)
	return d.page
}

// SetLocation is Document::set_location in krilla src/document.rs.
//
// Set the location that should be assumed for subsequent operations.
func (d *Document) SetLocation(location Location) {
	d.sc.setLocation(location)
}

// ResetLocation is Document::reset_location in krilla src/document.rs.
//
// Reset the location that should be assumed for subsequent operations.
func (d *Document) ResetLocation() {
	d.sc.resetLocation()
}

// Finish is Document::finish in krilla src/document.rs. krilla returns the
// bytes of the file. The port writes them to w (D62). It finishes the page
// that is open first. A document without pages gets one empty page of the
// default settings, as in upstream. A second call returns ErrPDFFinished.
//
// Attempt to export the document to a PDF file.
func (d *Document) Finish(w io.Writer) error {
	if d.finished {
		return okage.ErrPDFFinished
	}
	if err := d.sc.serializeSettings().err; err != nil {
		d.finished = true
		return err
	}
	if d.page != nil {
		d.page.Finish()
	}
	// Write empty page if none has been created yet.
	if len(d.sc.pageInfos) == 0 {
		d.StartPage(DefaultPageSettings()).Finish()
	}
	d.finished = true
	if d.sc.err != nil {
		return d.sc.err
	}

	pdf, err := d.sc.finish(d.cc)
	if err != nil {
		return err
	}
	if _, err := w.Write(pdf.Finish()); err != nil {
		return fmt.Errorf("writing pdf: %w", err)
	}
	return nil
}
