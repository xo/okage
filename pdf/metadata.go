// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"maps"
	"slices"
	"strings"

	"github.com/xo/okage/internal/pdfwriter"
)

// This file ports krilla src/interchange/metadata.rs: the metadata of the
// information dictionary and of the catalog. The base port has no XMP
// metadata (D62), so serialize_xmp_metadata and xmp_date have no port. The
// part of serialize_xmp_metadata that registers a validation error is in
// chunkContainer.finish, as finishMetadata.
//
// Setting document metadata.
//
// PDF allows for the inclusion of metadata in a PDF document. To do so, you
// can simply create a Metadata object, set the data, and then include it in
// the document via Document.SetMetadata.

// Metadata is Metadata in krilla src/interchange/metadata.rs. Each field of
// upstream that is an Option is a pointer here, and nil is None. A method
// that sets a field returns a copy of the metadata, as the builder of
// upstream does. The copy shares no slice and no map with the metadata it
// came from.
//
// Metadata for a PDF document.
type Metadata struct {
	title           *string
	description     *string
	creator         *string
	producer        *string
	keywords        []string
	authors         []string
	documentID      *string
	versionID       *string
	language        *string
	creationDate    *DateTime
	textDirection   *TextDirectionMetadata
	pageLayout      *PageLayout
	displayDocTitle bool
	// customFields holds the custom fields of the information dictionary.
	// Upstream keeps them in a BTreeMap, so the port writes them in the
	// order of their names.
	customFields map[string]string
}

// NewMetadata is Metadata::new in krilla src/interchange/metadata.rs.
//
// Create new metadata.
func NewMetadata() Metadata {
	return Metadata{}
}

// Title is Metadata::title in krilla src/interchange/metadata.rs. An empty
// title leaves the title as it is.
//
// The title of the document.
func (m Metadata) Title(title string) Metadata {
	if title != "" {
		m.title = new(title)
	}
	return m
}

// Description is Metadata::description in krilla
// src/interchange/metadata.rs. An empty description leaves the description
// as it is.
//
// The description of the document.
//
// This should be a short, human-readable abstract, summary, or description
// of the topic of the document.
func (m Metadata) Description(description string) Metadata {
	if description != "" {
		m.description = new(description)
	}
	return m
}

// Keywords is Metadata::keywords in krilla src/interchange/metadata.rs. An
// empty list leaves the keywords as they are.
//
// The keywords that describe the document.
func (m Metadata) Keywords(keywords []string) Metadata {
	if len(keywords) != 0 {
		m.keywords = slices.Clone(keywords)
	}
	return m
}

// Language is Metadata::language in krilla src/interchange/metadata.rs. An
// empty language is set too, as upstream does.
//
// The main language of the document, as an RFC 3066 language tag.
//
// This property is required for some export modes, like for example
// PDF/A-3a.
func (m Metadata) Language(language string) Metadata {
	m.language = new(language)
	return m
}

// Creator is Metadata::creator in krilla src/interchange/metadata.rs. An
// empty creator leaves the creator as it is.
//
// The creator tool of the document.
func (m Metadata) Creator(creator string) Metadata {
	if creator != "" {
		m.creator = new(creator)
	}
	return m
}

// Producer is Metadata::producer in krilla src/interchange/metadata.rs. An
// empty producer leaves the producer as it is.
//
// The producer tool of the document.
func (m Metadata) Producer(producer string) Metadata {
	if producer != "" {
		m.producer = new(producer)
	}
	return m
}

// Authors is Metadata::authors in krilla src/interchange/metadata.rs. An
// empty list leaves the authors as they are.
//
// The authors of the document.
func (m Metadata) Authors(authors []string) Metadata {
	if len(authors) != 0 {
		m.authors = slices.Clone(authors)
	}
	return m
}

// CreationDate is Metadata::creation_date in krilla
// src/interchange/metadata.rs.
//
// The creation date of the document.
func (m Metadata) CreationDate(creationDate DateTime) Metadata {
	m.creationDate = &creationDate
	return m
}

// DocumentID is Metadata::document_id in krilla src/interchange/metadata.rs.
//
// A document ID.
//
// This attribute will be used as an identifier for identifying different
// versions of the same document.
func (m Metadata) DocumentID(documentID string) Metadata {
	m.documentID = new(documentID)
	return m
}

// VersionID is Metadata::version_id in krilla src/interchange/metadata.rs.
// Upstream writes it only to the XMP metadata, so the base port does not
// write it (D62).
//
// A version ID.
//
// This attribute identifies a specific version of the document.
func (m Metadata) VersionID(versionID string) Metadata {
	m.versionID = new(versionID)
	return m
}

// TextDirection is Metadata::text_direction in krilla
// src/interchange/metadata.rs.
//
// The main text direction of the document.
func (m Metadata) TextDirection(textDirection TextDirectionMetadata) Metadata {
	m.textDirection = &textDirection
	return m
}

// PageLayout is Metadata::page_layout in krilla
// src/interchange/metadata.rs.
//
// How the viewer should lay out the pages.
func (m Metadata) PageLayout(pageLayout PageLayout) Metadata {
	m.pageLayout = &pageLayout
	return m
}

// DisplayDocTitle is Metadata::display_doc_title in krilla
// src/interchange/metadata.rs.
//
// Whether the viewer should show the title of the document in its title bar,
// rather than the file name.
//
// Validators that require it set it regardless.
func (m Metadata) DisplayDocTitle(displayDocTitle bool) Metadata {
	m.displayDocTitle = displayDocTitle
	return m
}

// CustomField is Metadata::custom_field in krilla
// src/interchange/metadata.rs. It returns false where upstream returns None,
// with the metadata as it was.
//
// Add a custom field to the document information dictionary.
//
// If a field with the same name was already added, its value will be
// replaced. Returns false for empty names and names reserved for standard
// document information fields.
//
// WARNING: Custom fields will currently be ignored when exporting to PDF/A-2
// and PDF/A-3 and PDF/A-4. This is a temporary limitation that will be lifted
// in the future.
func (m Metadata) CustomField(name, value string) (Metadata, bool) {
	if name == "" || isStandardDocumentInfoField(name) {
		return m, false
	}
	fields := make(map[string]string, len(m.customFields)+1)
	maps.Copy(fields, m.customFields)
	fields[name] = value
	m.customFields = fields
	return m, true
}

// hasDocumentInfo is Metadata::has_document_info in krilla
// src/interchange/metadata.rs.
func (m *Metadata) hasDocumentInfo() bool {
	return m.title != nil ||
		m.producer != nil ||
		m.keywords != nil ||
		m.authors != nil ||
		m.creator != nil ||
		m.creationDate != nil ||
		m.description != nil ||
		len(m.customFields) != 0
}

// serializeDocumentInfo is Metadata::serialize_document_info in krilla
// src/interchange/metadata.rs. Upstream starts the dictionary lazily, with a
// LazyCell, so that it writes no dictionary when it has no entry. The port
// starts it at the first entry in the same way. The reference is taken
// even when no entry is written, as upstream does.
//
// The validators of PDF/A-2 and PDF/A-3 leave out the custom fields. The base
// port has no validator (D62), so it writes them always.
func (m *Metadata) serializeDocumentInfo(ref *pdfwriter.Ref, pdf *pdfwriter.Pdf, config configuration) {
	if config.validators.prohibitsInfoDict() {
		return
	}
	if !m.hasDocumentInfo() {
		return
	}

	infoRef := ref.Bump()
	var info *pdfwriter.DocumentInfo
	documentInfo := func() *pdfwriter.DocumentInfo {
		if info == nil {
			info = pdf.DocumentInfo(infoRef)
		}
		return info
	}

	// All of those are deprecated in PDF 2.0 and will only be written to the
	// XMP metadata.
	if config.version < PDF20 {
		if m.title != nil {
			documentInfo().Title(pdfwriter.TextStr(*m.title))
		}
		if m.description != nil {
			documentInfo().Subject(pdfwriter.TextStr(*m.description))
		}
		if m.keywords != nil {
			documentInfo().Keywords(pdfwriter.TextStr(strings.Join(m.keywords, ", ")))
		}
		if m.authors != nil {
			documentInfo().Author(pdfwriter.TextStr(strings.Join(m.authors, ", ")))
		}
		if m.creator != nil {
			documentInfo().Creator(pdfwriter.TextStr(*m.creator))
		}
		if m.producer != nil {
			documentInfo().Producer(pdfwriter.TextStr(*m.producer))
		}
	}

	if m.creationDate != nil {
		documentInfo().ModifiedDate(pdfDate(*m.creationDate))
		documentInfo().CreationDate(pdfDate(*m.creationDate))
	}

	names := make([]string, 0, len(m.customFields))
	for name := range m.customFields {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		documentInfo().Pair(pdfwriter.Name(name), pdfwriter.TextStr(m.customFields[name]))
	}

	if info != nil {
		info.Finish()
	}
}

// isStandardDocumentInfoField is is_standard_document_info_field in krilla
// src/interchange/metadata.rs.
func isStandardDocumentInfoField(name string) bool {
	switch name {
	case "Title", "Author", "Subject", "Keywords", "Creator", "Producer",
		"CreationDate", "ModDate", "Trapped":
		return true
	}
	return false
}

// finishMetadata is the part of ChunkContainer::finish and
// Metadata::serialize_xmp_metadata in krilla src/chunk_container.rs and
// src/interchange/metadata.rs that does not write XMP metadata. It registers
// the validation error of a document without a creation date. Upstream
// registers it for each document with metadata, also when it writes no XMP
// metadata.
func finishMetadata(sc *serializeContext, m *Metadata) {
	if m != nil && m.creationDate == nil {
		sc.registerValidationError(validationError{kind: validationMissingDocumentDate})
	}
}

// metadataDocumentID is the part of ChunkContainer::finish in krilla
// src/chunk_container.rs that gives the first ID of the document. It is a
// hash of the version and the ID of the metadata, or of the version, the
// title and the authors. Without them, it is the instance ID.
func metadataDocumentID(sc *serializeContext, m *Metadata, instanceID string) string {
	if m == nil {
		return instanceID
	}
	version := sc.serializeSettings().pdfVersion().String()
	switch {
	case m.documentID != nil:
		// The hash of the tuple (&str, &String).
		return stableHashBase64(func(h *hasher) {
			h.str(version)
			h.str(*m.documentID)
		})
	case m.title != nil && m.authors != nil:
		// The hash of the tuple (&str, &Option<String>, &Option<Vec<String>>):
		// the case Some of each Option, then its value. A Vec writes its
		// length first.
		return stableHashBase64(func(h *hasher) {
			h.str(version)
			h.usize(1)
			h.str(*m.title)
			h.usize(1)
			h.usize(len(m.authors))
			for _, a := range m.authors {
				h.str(a)
			}
		})
	}
	return instanceID
}

// serializeCatalogMetadata is the part of ChunkContainer::finish in krilla
// src/chunk_container.rs that writes the viewer preferences and the page
// layout of the metadata into the catalog.
func serializeCatalogMetadata(sc *serializeContext, catalog *pdfwriter.Catalog, m *Metadata) {
	writeDocTitle := sc.serializeSettings().validators().requiresDisplayDocTitle() ||
		(m != nil && m.displayDocTitle)
	var textDirection *TextDirectionMetadata
	if m != nil {
		textDirection = m.textDirection
	}

	if writeDocTitle || textDirection != nil {
		vp := catalog.ViewerPreferences()
		if writeDocTitle {
			vp.DisplayDocTitle(true)
		}
		if textDirection != nil {
			vp.Direction(textDirection.toPDF())
		}
		vp.Finish()
	}

	if m != nil && m.pageLayout != nil {
		layout := *m.pageLayout
		// TwoPageLeft and TwoPageRight are only available PDF 1.5+.
		if sc.serializeSettings().pdfVersion() >= PDF15 ||
			(layout != PageLayoutTwoPageLeft && layout != PageLayoutTwoPageRight) {
			catalog.PageLayout(layout.toPDF())
		}
	}
}

// DateTime is DateTime in krilla src/interchange/metadata.rs. A field that is
// an Option of upstream has a flag for Some.
//
// A datetime. Invalid values will be clamped.
type DateTime struct {
	// year is the year (0-9999).
	year uint16
	// month is the month (1-12).
	month    uint8
	hasMonth bool
	// day is the day (1-31).
	day    uint8
	hasDay bool
	// hour is the hour (0-23).
	hour    uint8
	hasHour bool
	// minute is the minute (0-59).
	minute    uint8
	hasMinute bool
	// second is the second (0-59).
	second    uint8
	hasSecond bool
	// utcOffsetHour is the hour offset from UTC (-23 through 23).
	utcOffsetHour    int8
	hasUTCOffsetHour bool
	// utcOffsetMinute is the minute offset from UTC (0-59). It carries over
	// the sign from utcOffsetHour.
	utcOffsetMinute uint8
}

// NewDateTime is DateTime::new in krilla src/interchange/metadata.rs.
//
// Create a new, minimal date. The year will be clamped within the range
// 0-9999.
func NewDateTime(year uint16) DateTime {
	return DateTime{year: min(year, 9999)}
}

// Month is DateTime::month in krilla src/interchange/metadata.rs.
//
// Add the month field. It will be clamped within the range 1-12.
func (d DateTime) Month(month uint8) DateTime {
	d.month, d.hasMonth = min(max(month, 1), 12), true
	return d
}

// Day is DateTime::day in krilla src/interchange/metadata.rs.
//
// Add the day field. It will be clamped within the range 1-31.
func (d DateTime) Day(day uint8) DateTime {
	d.day, d.hasDay = min(max(day, 1), 31), true
	return d
}

// Hour is DateTime::hour in krilla src/interchange/metadata.rs.
//
// Add the hour field. It will be clamped within the range 0-23.
func (d DateTime) Hour(hour uint8) DateTime {
	d.hour, d.hasHour = min(hour, 23), true
	return d
}

// Minute is DateTime::minute in krilla src/interchange/metadata.rs.
//
// Add the minute field. It will be clamped within the range 0-59.
func (d DateTime) Minute(minute uint8) DateTime {
	d.minute, d.hasMinute = min(minute, 59), true
	return d
}

// Second is DateTime::second in krilla src/interchange/metadata.rs.
//
// Add the second field. It will be clamped within the range 0-59.
func (d DateTime) Second(second uint8) DateTime {
	d.second, d.hasSecond = min(second, 59), true
	return d
}

// UTCOffsetHour is DateTime::utc_offset_hour in krilla
// src/interchange/metadata.rs.
//
// Add the offset from UTC in hours. If not specified, the time will be
// assumed to be local to the viewer's time zone. It will be clamped within
// the range -23-23.
func (d DateTime) UTCOffsetHour(hour int8) DateTime {
	d.utcOffsetHour, d.hasUTCOffsetHour = min(max(hour, -23), 23), true
	return d
}

// UTCOffsetMinute is DateTime::utc_offset_minute in krilla
// src/interchange/metadata.rs.
//
// Add the offset from UTC in minutes. This will have the same sign as set in
// UTCOffsetHour. It will be clamped within the range 0-59.
func (d DateTime) UTCOffsetMinute(minute uint8) DateTime {
	d.utcOffsetMinute = min(minute, 59)
	return d
}

// orU8 returns v when ok is true, and def when it is false, as
// Option::unwrap_or does.
func orU8(v uint8, ok bool, def uint8) uint8 {
	if ok {
		return v
	}
	return def
}

// pdfDate is pdf_date in krilla src/interchange/metadata.rs.
//
// Converts a datetime to a pdf-writer date.
func pdfDate(d DateTime) pdfwriter.Date {
	// We always assume a full date with all fields because for some reason
	// Acrobat doesn't like PDF/A-1 files without everything set.
	var offset int8
	if d.hasUTCOffsetHour {
		offset = d.utcOffsetHour
	}
	return pdfwriter.NewDate(d.year).
		Month(orU8(d.month, d.hasMonth, 1)).
		Day(orU8(d.day, d.hasDay, 1)).
		Hour(orU8(d.hour, d.hasHour, 0)).
		Minute(orU8(d.minute, d.hasMinute, 0)).
		Second(orU8(d.second, d.hasSecond, 0)).
		UTCOffsetHour(offset).
		UTCOffsetMinute(d.utcOffsetMinute)
}

// TextDirectionMetadata is TextDirection in krilla
// src/interchange/metadata.rs. The module text of krilla has a TextDirection
// of its own, for the direction of a run of text, so this one takes the name
// of its module as a suffix (D10).
//
// The main text direction of the document.
type TextDirectionMetadata uint8

// The values of TextDirectionMetadata.
const (
	// TextDirectionMetadataLeftToRight is TextDirection::LeftToRight.
	TextDirectionMetadataLeftToRight TextDirectionMetadata = iota
	// TextDirectionMetadataRightToLeft is TextDirection::RightToLeft.
	TextDirectionMetadataRightToLeft
)

// toPDF is TextDirection::to_pdf in krilla src/interchange/metadata.rs. A
// value that is not one of the constants is right to left.
func (d TextDirectionMetadata) toPDF() pdfwriter.Direction {
	if d == TextDirectionMetadataLeftToRight {
		return pdfwriter.DirectionL2R
	}
	return pdfwriter.DirectionR2L
}

// PageLayout is PageLayout in krilla src/interchange/metadata.rs.
//
// How the viewer should lay out the pages.
type PageLayout uint8

// The values of PageLayout.
const (
	// PageLayoutSinglePage is PageLayout::SinglePage. Only a single page at
	// a time.
	PageLayoutSinglePage PageLayout = iota
	// PageLayoutOneColumn is PageLayout::OneColumn. A single, continuously
	// scrolling column of pages.
	PageLayoutOneColumn
	// PageLayoutTwoColumnLeft is PageLayout::TwoColumnLeft. Two continuously
	// scrolling columns of pages, laid out with odd-numbered pages on the
	// left.
	PageLayoutTwoColumnLeft
	// PageLayoutTwoColumnRight is PageLayout::TwoColumnRight. Two
	// continuously scrolling columns of pages, laid out with odd-numbered
	// pages on the right (like in a left-bound book).
	PageLayoutTwoColumnRight
	// PageLayoutTwoPageLeft is PageLayout::TwoPageLeft. Only two pages are
	// visible at a time, laid out with odd-numbered pages on the left. PDF
	// 1.5+.
	PageLayoutTwoPageLeft
	// PageLayoutTwoPageRight is PageLayout::TwoPageRight. Only two pages are
	// visible at a time, laid out with odd-numbered pages on the right (like
	// in a left-bound book). PDF 1.5+.
	PageLayoutTwoPageRight
)

// toPDF is PageLayout::to_pdf in krilla src/interchange/metadata.rs. A value
// that is not one of the constants is a single page.
func (l PageLayout) toPDF() pdfwriter.PageLayout {
	switch l {
	case PageLayoutOneColumn:
		return pdfwriter.PageLayoutOneColumn
	case PageLayoutTwoColumnLeft:
		return pdfwriter.PageLayoutTwoColumnLeft
	case PageLayoutTwoColumnRight:
		return pdfwriter.PageLayoutTwoColumnRight
	case PageLayoutTwoPageLeft:
		return pdfwriter.PageLayoutTwoPageLeft
	case PageLayoutTwoPageRight:
		return pdfwriter.PageLayoutTwoPageRight
	}
	return pdfwriter.PageLayoutSinglePage
}

// SetMetadata is Document::set_metadata in krilla src/document.rs.
//
// Set the metadata of the document.
func (d *Document) SetMetadata(metadata Metadata) {
	d.cc.metadata = &metadata
}

// prohibitsInfoDict is Validators::prohibits_info_dict in krilla
// src/configure/validate.rs.
func (validators) prohibitsInfoDict() bool {
	return false
}

// requiresDisplayDocTitle is Validators::requires_display_doc_title in
// krilla src/configure/validate.rs.
func (validators) requiresDisplayDocTitle() bool {
	return false
}
