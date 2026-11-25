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
	"github.com/xo/okage/internal/xmpwriter"
)

// This file ports krilla src/interchange/metadata/mod.rs: the metadata of the
// information dictionary, of the catalog and of the XMP metadata (D99). The
// modules custom.rs, extension_schema.rs, predefined.rs, relax_ng.rs and
// xmp.rs of the folder src/interchange/metadata are metadata_custom.go,
// metadata_extension_schema.go, metadata_predefined.go,
// metadata_relax_ng.go and metadata_xmp.go.
//
// Setting document metadata.
//
// PDF allows for the inclusion of metadata in a PDF document. To do so,
// create a Metadata object, set the data, and then include it in the document
// via Document.SetMetadata.

// Metadata is Metadata in krilla src/interchange/metadata/mod.rs. Each field
// of upstream that is an Option is a pointer here, and nil is None. A method
// that sets a field returns a copy of the metadata, as the builder of upstream
// does. The copy shares no slice and no map with the metadata it came from,
// so a slice that a method extends is copied first.
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
	trapped         *bool
	// customXMPProperties and customXMPSchemas hold the custom XMP metadata
	// of CustomXMPProperty and CustomXMPSchema.
	customXMPProperties []Property
	customXMPSchemas    []Schema
	// customFields holds the custom fields of the information dictionary.
	// Upstream keeps them in a BTreeMap, so the port writes them in the
	// order of their names.
	customFields map[string]string
}

// NewMetadata is Metadata::new in krilla src/interchange/metadata/mod.rs.
//
// Create new metadata.
func NewMetadata() Metadata {
	return Metadata{}
}

// Title is Metadata::title in krilla src/interchange/metadata/mod.rs. An empty
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
// src/interchange/metadata/mod.rs. An empty description leaves the description
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

// Keywords is Metadata::keywords in krilla src/interchange/metadata/mod.rs. An
// empty list leaves the keywords as they are.
//
// The keywords that describe the document.
func (m Metadata) Keywords(keywords []string) Metadata {
	if len(keywords) != 0 {
		m.keywords = slices.Clone(keywords)
	}
	return m
}

// Language is Metadata::language in krilla src/interchange/metadata/mod.rs. An
// empty language leaves the language as it is.
//
// The main language of the document, as an RFC 3066 language tag.
//
// This property is required for some export modes, like for example
// PDF/A-3a.
func (m Metadata) Language(language string) Metadata {
	if language != "" {
		m.language = new(language)
	}
	return m
}

// Creator is Metadata::creator in krilla src/interchange/metadata/mod.rs. An
// empty creator leaves the creator as it is.
//
// The creator tool of the document.
func (m Metadata) Creator(creator string) Metadata {
	if creator != "" {
		m.creator = new(creator)
	}
	return m
}

// Producer is Metadata::producer in krilla src/interchange/metadata/mod.rs. An
// empty producer leaves the producer as it is.
//
// The producer tool of the document.
func (m Metadata) Producer(producer string) Metadata {
	if producer != "" {
		m.producer = new(producer)
	}
	return m
}

// Authors is Metadata::authors in krilla src/interchange/metadata/mod.rs. An
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
// src/interchange/metadata/mod.rs.
//
// The creation date of the document.
func (m Metadata) CreationDate(creationDate DateTime) Metadata {
	m.creationDate = &creationDate
	return m
}

// DocumentID is Metadata::document_id in krilla
// src/interchange/metadata/mod.rs.
//
// A document ID.
//
// This attribute will be used as an identifier for identifying different
// versions of the same document.
func (m Metadata) DocumentID(documentID string) Metadata {
	m.documentID = new(documentID)
	return m
}

// VersionID is Metadata::version_id in krilla src/interchange/metadata/mod.rs.
// It goes only into the XMP metadata.
//
// A version ID.
//
// This attribute identifies a specific version of the document.
func (m Metadata) VersionID(versionID string) Metadata {
	m.versionID = new(versionID)
	return m
}

// TextDirection is Metadata::text_direction in krilla
// src/interchange/metadata/mod.rs.
//
// The main text direction of the document.
func (m Metadata) TextDirection(textDirection TextDirectionMetadata) Metadata {
	m.textDirection = &textDirection
	return m
}

// PageLayout is Metadata::page_layout in krilla
// src/interchange/metadata/mod.rs.
//
// How the viewer should lay out the pages.
func (m Metadata) PageLayout(pageLayout PageLayout) Metadata {
	m.pageLayout = &pageLayout
	return m
}

// DisplayDocTitle is Metadata::display_doc_title in krilla
// src/interchange/metadata/mod.rs.
//
// Whether the viewer should show the title of the document in its title bar,
// rather than the file name.
//
// Validators that require it set it regardless.
func (m Metadata) DisplayDocTitle(displayDocTitle bool) Metadata {
	m.displayDocTitle = displayDocTitle
	return m
}

// Trapped is Metadata::trapped in krilla src/interchange/metadata/mod.rs.
//
// Whether the document has been trapped.
//
// If this is not set, the trapping status is unknown.
func (m Metadata) Trapped(trapped bool) Metadata {
	m.trapped = &trapped
	return m
}

// CustomXMPProperty is Metadata::custom_xmp_property in krilla
// src/interchange/metadata/mod.rs.
//
// Add a custom XMP property.
func (m Metadata) CustomXMPProperty(property Property) Metadata {
	m.customXMPProperties = append(slices.Clip(m.customXMPProperties), property)
	return m
}

// CustomXMPSchema is Metadata::custom_xmp_schema in krilla
// src/interchange/metadata/mod.rs.
//
// Add a schema describing the properties of a custom XMP namespace.
func (m Metadata) CustomXMPSchema(schema Schema) Metadata {
	m.customXMPSchemas = append(slices.Clip(m.customXMPSchemas), schema)
	return m
}

// CustomField is Metadata::custom_field in krilla
// src/interchange/metadata/mod.rs. It returns false where upstream returns
// None, with the metadata as it was.
//
// Add a custom field to the document information dictionary.
//
// If a field with the same name was already added, its value will be
// replaced. Returns false for empty names and names reserved for standard
// document information fields.
//
// Note: Custom fields are ignored when exporting to PDF/A-2, PDF/A-3 and
// PDF/A-4. PDF/A-2 and PDF/A-3 readers ignore the document information
// dictionary, and in PDF/A-4 it may only contain ModDate.
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
// src/interchange/metadata/mod.rs.
func (m *Metadata) hasDocumentInfo() bool {
	return m.title != nil ||
		m.producer != nil ||
		m.keywords != nil ||
		m.authors != nil ||
		m.creator != nil ||
		m.creationDate != nil ||
		m.description != nil ||
		m.trapped != nil ||
		len(m.customFields) != 0
}

// serializeXMPMetadata is Metadata::serialize_xmp_metadata in krilla
// src/interchange/metadata/mod.rs.
func (m *Metadata) serializeXMPMetadata(xmp *xmpwriter.Writer, sc *serializeContext, instanceID string) error {
	if m.title != nil {
		xmp.Title(xmpwriter.LangAlt{Value: *m.title})
	}

	if m.description != nil {
		xmp.Description(xmpwriter.LangAlt{Value: *m.description})
	}

	if m.keywords != nil {
		xmp.PDFKeywords(strings.Join(m.keywords, ", "))
	}

	if m.authors != nil {
		if sc.serializeSettings().pdfVersion() >= PDF20 {
			// PDF 2.0+ deprecates the document information dictionary, so
			// we can use the array here.
			xmp.Creator(m.authors...)
		} else {
			// Turns out that if the authors are given in both the document
			// information dictionary and the XMP metadata, Acrobat takes a
			// little bit of both: The first author from the document
			// information dictionary and the remaining authors from the XMP
			// metadata.
			//
			// To fix this for Acrobat, we could omit the remaining authors
			// or all metadata from the document information catalog (it is
			// optional) and only write XMP. However, not all other tools
			// (including Apple Preview) read the XMP data. This means we do
			// want to include all authors in the document information
			// dictionary.
			//
			// Thus, the only alternative is to fold all authors into a
			// single <rdf:li> in the XMP metadata. This is, in fact, exactly
			// what the PDF/A spec Part 1 section 6.7.3 has to say about the
			// matter. It's a bit weird to not use the array (and it makes
			// Acrobat show the author list in quotes), but there's not much
			// we can do about that.
			xmp.Creator(strings.Join(m.authors, ", "))
		}
	}

	if m.creator != nil {
		xmp.CreatorTool(*m.creator)
	}

	if m.producer != nil {
		xmp.Producer(*m.producer)
	}

	if m.versionID != nil {
		xmp.VersionID(*m.versionID)
	}

	if m.language != nil {
		xmp.Language(xmpwriter.LangID(*m.language))
	}

	if m.trapped != nil {
		xmp.Trapped(*m.trapped)
	}

	if m.creationDate != nil {
		date := xmpDate(*m.creationDate)
		xmp.ModifyDate(date)
		xmp.CreateDate(date)

		validators := sc.serializeSettings().validators()
		if validators.requiresFileProvenanceInformation() {
			history := xmp.History()
			saved := history.AddEvent()

			saved.Action(xmpwriter.ResourceEventActionSaved).When(date)

			if !validators.prohibitsInstanceIDInXMPMetadata() {
				saved.InstanceID(instanceID + "_source")
			}

			saved.Finish()

			converted := history.AddEvent()

			converted.Action(xmpwriter.ResourceEventActionConverted).When(date)

			if m.creator != nil {
				converted.SoftwareAgent(*m.creator)
			}

			if !validators.prohibitsInstanceIDInXMPMetadata() {
				converted.InstanceID(instanceID + "_source")
			}
			converted.Finish()
			history.Finish()
		}
	} else {
		sc.registerValidationError(validationError{kind: validationMissingDocumentDate})
	}

	return m.serializeCustomXMP(xmp, sc)
}

// relaxNGSchema is Metadata::relax_ng_schema in krilla
// src/interchange/metadata/mod.rs.
//
// A RELAX NG schema describing the XMP metadata of the document.
func (m *Metadata) relaxNGSchema() string {
	schemas := mergeSchemas(managedSchemas(), m.customXMPSchemas)
	var undescribed []undescribedProperty
	for i := range m.customXMPProperties {
		property := &m.customXMPProperties[i]
		uri := property.Namespace.uri
		described := slices.ContainsFunc(schemas, func(s Schema) bool {
			_, ok := s.get(property.Name)
			return s.namespace.uri == uri && ok
		})
		listed := slices.ContainsFunc(undescribed, func(u undescribedProperty) bool {
			return u.namespace.uri == uri && u.name == property.Name
		})
		if !described && !listed {
			undescribed = append(undescribed, undescribedProperty{namespace: property.Namespace, name: property.Name})
		}
	}

	return relaxNGSchema(schemas, undescribed)
}

// extensionSchemas is Metadata::extension_schemas in krilla
// src/interchange/metadata/mod.rs.
//
// The attached schemas that need a PDF/A extension schema, without the
// descriptions of predefined properties.
func (m *Metadata) extensionSchemas(validators validators) []Schema {
	xmp2005 := validators.usesXMP2005PredefinedSchemas()
	var schemas []Schema
	for _, schema := range m.customXMPSchemas {
		uri := schema.namespace.uri
		isPredefined := func(desc PropertyDescription) bool {
			return isPredefinedPDFAProperty(uri, desc.name, xmp2005)
		}
		if !slices.ContainsFunc(schema.properties, isPredefined) {
			schemas = append(schemas, schema)
			continue
		}

		schema.properties = slices.DeleteFunc(slices.Clone(schema.properties), isPredefined)
		if len(schema.properties) != 0 {
			schemas = append(schemas, schema)
		}
	}
	return schemas
}

// serializeDocumentInfo is Metadata::serialize_document_info in krilla
// src/interchange/metadata/mod.rs. Upstream starts the dictionary lazily, with
// a LazyCell, so that it writes no dictionary when it has no entry. The port
// starts it at the first entry in the same way. The reference is taken even
// when no entry is written, as upstream does.
//
// The validators of PDF/A-2 and PDF/A-3 leave out the custom fields.
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

		if m.trapped != nil {
			if *m.trapped {
				documentInfo().Trapped(pdfwriter.TrappingStatusTrapped)
			} else {
				documentInfo().Trapped(pdfwriter.TrappingStatusNotTrapped)
			}
		}
	}

	if m.creationDate != nil {
		documentInfo().ModifiedDate(pdfDate(*m.creationDate))
		documentInfo().CreationDate(pdfDate(*m.creationDate))
	}

	// See https://github.com/LaurenzV/krilla/pull/419#issuecomment-5142992957.
	archival, ok := config.validators.archival()
	if !ok || !archival.isA2A3() {
		names := make([]string, 0, len(m.customFields))
		for name := range m.customFields {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			documentInfo().Pair(pdfwriter.Name(name), pdfwriter.TextStr(m.customFields[name]))
		}
	}

	if info != nil {
		info.Finish()
	}
}

// isStandardDocumentInfoField is is_standard_document_info_field in krilla
// src/interchange/metadata/mod.rs.
func isStandardDocumentInfoField(name string) bool {
	switch name {
	case "Title", "Author", "Subject", "Keywords", "Creator", "Producer",
		"CreationDate", "ModDate", "Trapped":
		return true
	}
	return false
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

// DateTime is DateTime in krilla src/interchange/metadata/mod.rs. A field that
// is an Option of upstream has a flag for Some.
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

// NewDateTime is DateTime::new in krilla src/interchange/metadata/mod.rs.
//
// Create a new, minimal date. The year will be clamped within the range
// 0-9999.
func NewDateTime(year uint16) DateTime {
	return DateTime{year: min(year, 9999)}
}

// Month is DateTime::month in krilla src/interchange/metadata/mod.rs.
//
// Add the month field. It will be clamped within the range 1-12.
func (d DateTime) Month(month uint8) DateTime {
	d.month, d.hasMonth = min(max(month, 1), 12), true
	return d
}

// Day is DateTime::day in krilla src/interchange/metadata/mod.rs.
//
// Add the day field. It will be clamped within the range 1-31.
func (d DateTime) Day(day uint8) DateTime {
	d.day, d.hasDay = min(max(day, 1), 31), true
	return d
}

// Hour is DateTime::hour in krilla src/interchange/metadata/mod.rs.
//
// Add the hour field. It will be clamped within the range 0-23.
func (d DateTime) Hour(hour uint8) DateTime {
	d.hour, d.hasHour = min(hour, 23), true
	return d
}

// Minute is DateTime::minute in krilla src/interchange/metadata/mod.rs.
//
// Add the minute field. It will be clamped within the range 0-59.
func (d DateTime) Minute(minute uint8) DateTime {
	d.minute, d.hasMinute = min(minute, 59), true
	return d
}

// Second is DateTime::second in krilla src/interchange/metadata/mod.rs.
//
// Add the second field. It will be clamped within the range 0-59.
func (d DateTime) Second(second uint8) DateTime {
	d.second, d.hasSecond = min(second, 59), true
	return d
}

// UTCOffsetHour is DateTime::utc_offset_hour in krilla
// src/interchange/metadata/mod.rs.
//
// Add the offset from UTC in hours. If not specified, the time will be
// assumed to be local to the viewer's time zone. It will be clamped within
// the range -23-23.
func (d DateTime) UTCOffsetHour(hour int8) DateTime {
	d.utcOffsetHour, d.hasUTCOffsetHour = min(max(hour, -23), 23), true
	return d
}

// UTCOffsetMinute is DateTime::utc_offset_minute in krilla
// src/interchange/metadata/mod.rs.
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

// xmpDate is xmp_date in krilla src/interchange/metadata/mod.rs.
//
// Converts a datetime to an xmp-writer datetime.
func xmpDate(d DateTime) xmpwriter.DateTime {
	timezone := xmpwriter.Timezone{}
	if d.hasUTCOffsetHour {
		timezone = xmpwriter.Timezone{Local: true, Hour: d.utcOffsetHour, Minute: int8(d.utcOffsetMinute)}
	}

	// We always assume a full date with all fields because for some reason
	// Acrobat doesn't like PDF/A-1 files without everything set.
	return xmpwriter.NewDateTime(
		d.year,
		orU8(d.month, d.hasMonth, 1),
		orU8(d.day, d.hasDay, 1),
		orU8(d.hour, d.hasHour, 0),
		orU8(d.minute, d.hasMinute, 0),
		orU8(d.second, d.hasSecond, 0),
		timezone,
	)
}

// pdfDate is pdf_date in krilla src/interchange/metadata/mod.rs.
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
// src/interchange/metadata/mod.rs. The module text of krilla has a
// TextDirection of its own, for the direction of a run of text, so this one
// takes the name of its module as a suffix (D10).
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

// toPDF is TextDirection::to_pdf in krilla src/interchange/metadata/mod.rs. A
// value that is not one of the constants is right to left.
func (d TextDirectionMetadata) toPDF() pdfwriter.Direction {
	if d == TextDirectionMetadataLeftToRight {
		return pdfwriter.DirectionL2R
	}
	return pdfwriter.DirectionR2L
}

// PageLayout is PageLayout in krilla src/interchange/metadata/mod.rs.
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

// toPDF is PageLayout::to_pdf in krilla src/interchange/metadata/mod.rs. A
// value that is not one of the constants is a single page.
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
