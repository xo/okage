// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"strconv"

	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/internal/siphasher"
	"github.com/xo/okage/internal/xmpwriter"
)

// This file ports krilla src/configure/validate.rs, with the validators of
// PDF/A (D99). The validator of PDF/UA, Accessibility::UA1, is left out.
// It requires the tags of PDF/UA, which pdf does not have (D62), so each
// document with it fails with ValidationError::MissingTagging. The
// separation colors are left out too, so ValidationStore has no port, and
// no error InconsistentSeparationFallback is registered.
//
// Exporting with a specific PDF conformance level.
//
// PDF defines a number of additional conformance levels that restrict the
// features of PDF that can be used to a specific subset.
//
// krilla writes the file in a way that conforms to the given standard, for
// example with the metadata that the standard requires. For the aspects that
// depend on the input, krilla validates the input, records each violation,
// and Document.Finish returns an error that wraps okage.ErrPDFValidation if
// there is at least one. Some requirements are semantic and cannot be
// validated. The caller must make sure that they are fulfilled.

// validationErrorKind is the case of the enum ValidationError in krilla
// src/configure/validate.rs.
type validationErrorKind uint8

// The cases of ValidationError, in the order of upstream.
const (
	// validationTooLongString is ValidationError::TooLongString. There was a
	// string that was longer than the maximum allowed length (32767).
	validationTooLongString validationErrorKind = iota
	// validationTooLongName is ValidationError::TooLongName. There was a name
	// that was longer than the maximum allowed length (127).
	validationTooLongName
	// validationTooLongArray is ValidationError::TooLongArray. There was an
	// array that was longer than the maximum allowed length (8191).
	validationTooLongArray
	// validationTooLongDictionary is ValidationError::TooLongDictionary.
	// There was a dictionary with more entries than the maximum allowed
	// (4095).
	validationTooLongDictionary
	// validationTooLargeFloat is ValidationError::TooLargeFloat. There was a
	// float that is higher than the maximum allowed (32767).
	validationTooLargeFloat
	// validationTooManyIndirectObjects is
	// ValidationError::TooManyIndirectObjects. The PDF exceeds the upper
	// limit for indirect objects (8388607).
	validationTooManyIndirectObjects
	// validationTooHighQNestingLevel is ValidationError::TooHighQNestingLevel.
	// The PDF contains a content stream that exceeds maximum allowed q/Q
	// nesting level (28).
	validationTooHighQNestingLevel
	// validationContainsPostScript is ValidationError::ContainsPostScript,
	// with a location. The PDF contains PostScript code.
	validationContainsPostScript
	// validationMissingCMYKProfile is ValidationError::MissingCMYKProfile. No
	// CMYK ICC profile was provided, even though one is necessary.
	validationMissingCMYKProfile
	// validationInconsistentSeparationFallback is
	// ValidationError::InconsistentSeparationFallback. The separation colors
	// are not in the base port.
	validationInconsistentSeparationFallback
	// validationContainsNotDefGlyph is ValidationError::ContainsNotDefGlyph,
	// with a font, a location and the text of the glyph. The .notdef glyph
	// was used.
	validationContainsNotDefGlyph
	// validationNoCodepointMapping is ValidationError::NoCodepointMapping,
	// with a font, a glyph and a location. A glyph was mapped to no
	// codepoint at all.
	validationNoCodepointMapping
	// validationInvalidCodepointMapping is
	// ValidationError::InvalidCodepointMapping, with a font, a glyph, a
	// character and a location. A glyph was mapped either to the codepoint
	// 0x0, 0xFEFF or 0xFFFE.
	validationInvalidCodepointMapping
	// validationUnicodePrivateArea is ValidationError::UnicodePrivateArea,
	// with a font, a glyph, a character and a location. A glyph was mapped
	// to a codepoint in the Unicode private use area.
	validationUnicodePrivateArea
	// validationRestrictedLicense is ValidationError::RestrictedLicense, with
	// a font. A font has a license that requires explicit permission of the
	// legal owner for embedding.
	validationRestrictedLicense
	// validationNoDocumentLanguage is ValidationError::NoDocumentLanguage. No
	// document language was set via the metadata.
	validationNoDocumentLanguage
	// validationNoDocumentTitle is ValidationError::NoDocumentTitle. No title
	// was provided for the document.
	validationNoDocumentTitle
	// validationMissingAltText is ValidationError::MissingAltText, with a
	// location. A figure or formula is missing an alt text.
	validationMissingAltText
	// validationMissingHeadingTitle is ValidationError::MissingHeadingTitle.
	// A heading is missing a title.
	validationMissingHeadingTitle
	// validationMissingDocumentOutline is
	// ValidationError::MissingDocumentOutline. The document does not contain
	// an outline.
	validationMissingDocumentOutline
	// validationMissingAnnotationAltText is
	// ValidationError::MissingAnnotationAltText, with a location. An
	// annotation is missing an alt text.
	validationMissingAnnotationAltText
	// validationMissingFieldAltName is ValidationError::MissingFieldAltName,
	// with a location. A form field is missing an alt name.
	validationMissingFieldAltName
	// validationMissingDocumentDate is ValidationError::MissingDocumentDate.
	// The date of the document is missing.
	validationMissingDocumentDate
	// validationTransparency is ValidationError::Transparency, with a
	// location. The PDF contains transparency.
	validationTransparency
	// validationImageInterpolation is ValidationError::ImageInterpolation,
	// with a location. The PDF contains an image with interpolate set to
	// true.
	validationImageInterpolation
	// validationEmbeddedFile is ValidationError::EmbeddedFile, with an
	// EmbedError and a location. The PDF contains an embedded file.
	validationEmbeddedFile
	// validationMissingTagging is ValidationError::MissingTagging. The PDF
	// contains no tagging.
	validationMissingTagging
	// validationAnnotationHasConditionalAppearance is
	// ValidationError::AnnotationHasConditionalAppearance, with a location.
	// The PDF contains an annotation with a rollover or down appearance.
	validationAnnotationHasConditionalAppearance
	// validationContainsMutatingAction is
	// ValidationError::ContainsMutatingAction, with a location. The PDF
	// contains an action that can mutate the appearance of the document.
	validationContainsMutatingAction
	// validationContainsAdditionalActions is
	// ValidationError::ContainsAdditionalActions, with a location. The PDF
	// contains an annotation or field that has an additional-actions
	// dictionary.
	validationContainsAdditionalActions
	// validationEmbeddedPDF is ValidationError::EmbeddedPDF. The embedded PDF
	// files are not in the base port.
	validationEmbeddedPDF
	// validationRequiresNewerPDFVersion is
	// ValidationError::RequiresNewerPdfVersion, with a feature and a
	// location. A feature only available in a later PDF version was
	// required.
	validationRequiresNewerPDFVersion
	// validationMissingXMPPropertyDescription is
	// ValidationError::MissingXmpPropertyDescription, with a namespace URI and
	// a property name. A custom XMP property was used in PDF/A export without
	// a matching PDF/A extension schema description.
	validationMissingXMPPropertyDescription
	// validationUnsupportedXMPValueType is
	// ValidationError::UnsupportedXmpValueType, with a namespace URI and a
	// property name. A custom XMP property or structure field was described
	// with a value type that PDF/A extension schemas can't express.
	validationUnsupportedXMPValueType
)

// validationErrorNames holds the Rust name of each case of ValidationError.
var validationErrorNames = [...]string{
	validationTooLongString:                      "TooLongString",
	validationTooLongName:                        "TooLongName",
	validationTooLongArray:                       "TooLongArray",
	validationTooLongDictionary:                  "TooLongDictionary",
	validationTooLargeFloat:                      "TooLargeFloat",
	validationTooManyIndirectObjects:             "TooManyIndirectObjects",
	validationTooHighQNestingLevel:               "TooHighQNestingLevel",
	validationContainsPostScript:                 "ContainsPostScript",
	validationMissingCMYKProfile:                 "MissingCMYKProfile",
	validationInconsistentSeparationFallback:     "InconsistentSeparationFallback",
	validationContainsNotDefGlyph:                "ContainsNotDefGlyph",
	validationNoCodepointMapping:                 "NoCodepointMapping",
	validationInvalidCodepointMapping:            "InvalidCodepointMapping",
	validationUnicodePrivateArea:                 "UnicodePrivateArea",
	validationRestrictedLicense:                  "RestrictedLicense",
	validationNoDocumentLanguage:                 "NoDocumentLanguage",
	validationNoDocumentTitle:                    "NoDocumentTitle",
	validationMissingAltText:                     "MissingAltText",
	validationMissingHeadingTitle:                "MissingHeadingTitle",
	validationMissingDocumentOutline:             "MissingDocumentOutline",
	validationMissingAnnotationAltText:           "MissingAnnotationAltText",
	validationMissingFieldAltName:                "MissingFieldAltName",
	validationMissingDocumentDate:                "MissingDocumentDate",
	validationTransparency:                       "Transparency",
	validationImageInterpolation:                 "ImageInterpolation",
	validationEmbeddedFile:                       "EmbeddedFile",
	validationMissingTagging:                     "MissingTagging",
	validationAnnotationHasConditionalAppearance: "AnnotationHasConditionalAppearance",
	validationContainsMutatingAction:             "ContainsMutatingAction",
	validationContainsAdditionalActions:          "ContainsAdditionalActions",
	validationEmbeddedPDF:                        "EmbeddedPDF",
	validationRequiresNewerPDFVersion:            "RequiresNewerPdfVersion",
	validationMissingXMPPropertyDescription:      "MissingXmpPropertyDescription",
	validationUnsupportedXMPValueType:            "UnsupportedXmpValueType",
}

// String returns the Rust name of the case.
func (k validationErrorKind) String() string {
	if int(k) < len(validationErrorNames) {
		return validationErrorNames[k]
	}
	return "ValidationError(?)"
}

// validationError is ValidationError in krilla src/configure/validate.rs. A
// Rust enum with data becomes one struct with the fields of each case, and a
// case sets only its own fields. The value is comparable, as the derived Eq
// and Hash of upstream make it, so that the serializer removes the errors
// that repeat.
//
// The cases of the text carry a Font of krilla, which the field font holds
// as the hash of the font, so that the value stays comparable. The case of
// the separation colors carries a type that pdf leaves out.
//
// An error that occurred during validation.
type validationError struct {
	kind validationErrorKind
	// location is the Option<Location> of the cases that have one. Zero is
	// None.
	location Location
	// feature is the VersionedFeature of RequiresNewerPdfVersion.
	feature versionedFeature
	// glyph is the GlyphId of the cases of the text.
	glyph uint32
	// char is the char of InvalidCodepointMapping and UnicodePrivateArea.
	char rune
	// text is the String of ContainsNotDefGlyph.
	text string
	// font is the hash of the Font of the cases of the text, as
	// Font::eq of upstream compares it.
	font siphasher.Hash128
	// embed is the EmbedError of EmbeddedFile.
	embed embedError
	// namespaceURI and propertyName are the fields of
	// MissingXmpPropertyDescription and UnsupportedXmpValueType.
	namespaceURI string
	propertyName string
}

// hash is the derived Hash of ValidationError in krilla
// src/configure/validate.rs. A Stream hashes its validation errors. Each
// field of the struct is written, which keeps two errors that differ apart.
func (e validationError) hash(h *hasher) {
	h.usize(int(e.kind))
	h.u64(uint64(e.location))
	h.usize(int(e.feature))
	h.u32(e.glyph)
	h.u32(uint32(e.char))
	h.str(e.text)
	h.u128(e.font)
	h.usize(int(e.embed))
	h.str(e.namespaceURI)
	h.str(e.propertyName)
}

// versionedFeature is VersionedFeature in krilla src/configure/validate.rs.
//
// Features that may require a later PDF version than the current one.
type versionedFeature uint8

// The cases of VersionedFeature.
const (
	// featureStructureOrderTabbing is
	// VersionedFeature::StructureOrderTabbing. Tabbing through the document
	// according to the structure order.
	featureStructureOrderTabbing versionedFeature = iota
	// featureHeaderFooterArtifactSubtypes is
	// VersionedFeature::HeaderFooterArtifactSubtypes. Header and footer
	// artifact subtypes.
	featureHeaderFooterArtifactSubtypes
	// featureTableHeaderScope is VersionedFeature::TableHeaderScope. Scope
	// attribute for table header cells.
	featureTableHeaderScope
)

// minimumPDFVersion is VersionedFeature::minimum_pdf_version in krilla
// src/configure/validate.rs.
//
// Get the minimum PDF version required for this feature.
func (f versionedFeature) minimumPDFVersion() PDFVersion {
	switch f {
	case featureStructureOrderTabbing:
		return PDF15
	case featureHeaderFooterArtifactSubtypes:
		return PDF17
	default:
		return PDF15
	}
}

// validators is Validators in krilla src/configure/validate.rs, with the
// validator of PDF/A only. A zero archival is None.
//
// Collection of validators with at most one validator for each standard.
type validators struct {
	a Archival
}

// prohibits is Validators::prohibits in krilla src/configure/validate.rs.
// It returns false where upstream returns None.
//
// Returns a filtered Validators containing only validators that prohibit
// the given error, or None if no validator prohibits it.
func (v validators) prohibits(err validationError) (validators, bool) {
	if v.a != 0 && v.a.prohibits(err) {
		return v, true
	}
	return validators{}, false
}

// isEmpty is Validators::is_empty in krilla src/configure/validate.rs.
//
// Returns true if no validators are set.
func (v validators) isEmpty() bool {
	return v.a == 0
}

// archival is Validators::archival in krilla src/configure/validate.rs. It
// returns false where upstream returns None.
//
// Returns the PDF/A validator, if set.
func (v validators) archival() (Archival, bool) {
	return v.a, v.a != 0
}

// hasAccessibility is Validators::accessibility in krilla
// src/configure/validate.rs, which returns the validator of PDF/UA, if set.
// pdf has no validator of PDF/UA, so it returns false.
func (validators) hasAccessibility() bool {
	return false
}

// requiresCodepointMappings is Validators::requires_codepoint_mappings in
// krilla src/configure/validate.rs.
//
// Whether the font must supply valid Unicode code points for each of the
// drawn glyphs.
func (v validators) requiresCodepointMappings() bool {
	return v.a != 0 && v.a.requiresCodepointMappings()
}

// requiresDisplayDocTitle is Validators::requires_display_doc_title in
// krilla src/configure/validate.rs. Only the validator of PDF/UA requires
// it, so it returns false.
//
// Force the DisplayDocTitle flag set.
func (validators) requiresDisplayDocTitle() bool {
	return false
}

// requiresNoDeviceCS is Validators::requires_no_device_cs in krilla
// src/configure/validate.rs.
//
// Force sRGB profiles for DeviceGray and DeviceRgb colorspaces.
func (v validators) requiresNoDeviceCS() bool {
	return v.a != 0 && v.a.requiresNoDeviceCS()
}

// requiresAnnotationFlags is Validators::requires_annotation_flags in krilla
// src/configure/validate.rs.
//
// Force the Print flag set and the Hidden, Invisible, ToggleNoView, and
// NoView flags unset.
func (v validators) requiresAnnotationFlags() bool {
	return v.a != 0 && v.a.requiresAnnotationFlags()
}

// requiresTagging is Validators::requires_tagging in krilla
// src/configure/validate.rs.
//
// Whether Tagged PDF must be enabled.
func (v validators) requiresTagging() bool {
	return v.a != 0 && v.a.requiresTagging()
}

// requiresXMPMetadata is Validators::requires_xmp_metadata in krilla
// src/configure/validate.rs.
//
// Whether XMP metadata must be written.
func (v validators) requiresXMPMetadata() bool {
	return v.a != 0 && v.a.requiresXMPMetadata()
}

// requiresXMPMetadataExtensionSchema is
// Validators::requires_xmp_metadata_extension_schema in krilla
// src/configure/validate.rs.
//
// Whether any extension schemata should be described using the
// "pdfaSchema" namespace.
func (v validators) requiresXMPMetadataExtensionSchema() bool {
	return v.a != 0 && v.a.requiresXMPMetadataExtensionSchema()
}

// usesXMP2005PredefinedSchemas is
// Validators::uses_xmp_2005_predefined_schemas in krilla
// src/configure/validate.rs.
//
// Whether the predefined XMP schemas are from the larger XMP 2005 set
// (PDF/A-2 and PDF/A-3) rather than the smaller XMP 2004 set (PDF/A-1).
func (v validators) usesXMP2005PredefinedSchemas() bool {
	return v.a != 0 && v.a.usesXMP2005PredefinedSchemas()
}

// prohibitsInstanceIDInXMPMetadata is
// Validators::prohibits_instance_id_in_xmp_metadata in krilla
// src/configure/validate.rs.
//
// Whether the instanceID field is allowed in XMP.
func (v validators) prohibitsInstanceIDInXMPMetadata() bool {
	return v.a != 0 && v.a.prohibitsInstanceIDInXMPMetadata()
}

// requiresFileProvenanceInformation is
// Validators::requires_file_provenance_information in krilla
// src/configure/validate.rs.
//
// Whether the xmpMM:History entry is required.
func (v validators) requiresFileProvenanceInformation() bool {
	return v.a != 0 && v.a.requiresFileProvenanceInformation()
}

// prohibitsInfoDict is Validators::prohibits_info_dict in krilla
// src/configure/validate.rs.
//
// Whether the /Info dictionary is allowed in the file trailer.
func (v validators) prohibitsInfoDict() bool {
	return v.a != 0 && v.a.prohibitsInfoDict()
}

// requiresBinaryHeader is Validators::requires_binary_header in krilla
// src/configure/validate.rs.
//
// Whether a non-printable file header is mandatory.
func (v validators) requiresBinaryHeader() bool {
	return v.a != 0 && v.a.requiresBinaryHeader()
}

// requiresEmbeddedFilesWhenEmpty is
// Validators::requires_embedded_files_when_empty in krilla
// src/configure/validate.rs.
//
// Whether the EmbeddedFiles key in the name dictionary of the document
// catalog dictionary should be written even if empty.
func (v validators) requiresEmbeddedFilesWhenEmpty() bool {
	return v.a != 0 && v.a.requiresEmbeddedFilesWhenEmpty()
}

// specifiesAssociatedFiles is Validators::specifies_associated_files in
// krilla src/configure/validate.rs.
//
// Whether any of these standards explicitly specifies the /AF key.
//
// The /AF key may be supported by the underlying PDF version instead:
// Starting at PDF 2.0, the key is specified by ISO 32000 and does not need
// to be added by PDF/A.
func (v validators) specifiesAssociatedFiles() bool {
	return v.a != 0 && v.a.specifiesAssociatedFiles()
}

// outputIntent is Validators::output_intent in krilla
// src/configure/validate.rs. It returns false where upstream returns None.
func (v validators) outputIntent() (pdfwriter.OutputIntentSubtype, bool) {
	if v.a == 0 {
		return pdfwriter.OutputIntentSubtype{}, false
	}
	return v.a.outputIntent(), true
}

// recommendsXMPRelaxNGSchema is Validators::recommends_xmp_relax_ng_schema
// in krilla src/configure/validate.rs.
//
// Whether the XMP metadata should be described by a RELAX NG schema in an
// associated file.
func (v validators) recommendsXMPRelaxNGSchema() bool {
	return v.a != 0 && v.a.recommendsXMPRelaxNGSchema()
}

// extensionSchemas is Validators::extension_schemas in krilla
// src/configure/validate.rs.
//
// The PDF/A extension schemas describing the properties krilla writes
// itself.
func (v validators) extensionSchemas() []Schema {
	var schemas []Schema
	if v.requiresXMPMetadataExtensionSchema() {
		schemas = append(schemas, v.a.extensionSchemas()...)
	}
	return schemas
}

// writeXMP is Validators::write_xmp in krilla src/configure/validate.rs.
func (v validators) writeXMP(xmp *xmpwriter.Writer, userSchemas []Schema) {
	if v.requiresXMPMetadataExtensionSchema() {
		w := xmp.ExtensionSchemas()
		writeExtensionSchemas(w, v.extensionSchemas(), userSchemas)
		w.Finish()
	}

	if v.a != 0 {
		v.a.writeXMP(xmp)
	}
}

// max is Validators::max in krilla src/configure/validate.rs.
//
// Returns the maximum PDF version allowed by all active validators.
func (v validators) max() PDFVersion {
	if v.a == 0 {
		return PDFVersionMax
	}
	return v.a.max()
}

// min is Validators::min in krilla src/configure/validate.rs. It returns
// false where upstream returns None.
//
// Returns the minimum PDF version required by all active validators, if
// any.
func (v validators) min() (PDFVersion, bool) {
	if v.a == 0 {
		return 0, false
	}
	return v.a.min(), true
}

// Archival is Archival in krilla src/configure/validate.rs. The zero value
// is no validator, so the constants start at 1. WithArchivalValidator sets
// it.
//
// A PDF/A conformance level.
type Archival uint8

// The values of Archival.
const (
	// ArchivalA1A is Archival::A1_A, the validator for the PDF/A-1a
	// standard. It requires the tags of PDF/A, which pdf does not have, so
	// each document with it fails with the validation error MissingTagging
	// (D99).
	ArchivalA1A Archival = iota + 1
	// ArchivalA1B is Archival::A1_B, the validator for the PDF/A-1b
	// standard.
	ArchivalA1B
	// ArchivalA2A is Archival::A2_A, the validator for the PDF/A-2a
	// standard. It requires the tags of PDF/A, which pdf does not have.
	ArchivalA2A
	// ArchivalA2B is Archival::A2_B, the validator for the PDF/A-2b
	// standard. You should only use fonts that are legally embeddable in a
	// file for unlimited, universal rendering.
	ArchivalA2B
	// ArchivalA2U is Archival::A2_U, the validator for the PDF/A-2u
	// standard. It has all requirements of PDF/A-2b.
	ArchivalA2U
	// ArchivalA3A is Archival::A3_A, the validator for the PDF/A-3a
	// standard. It requires the tags of PDF/A, which pdf does not have.
	ArchivalA3A
	// ArchivalA3B is Archival::A3_B, the validator for the PDF/A-3b
	// standard. It has all requirements of PDF/A-2b.
	ArchivalA3B
	// ArchivalA3U is Archival::A3_U, the validator for the PDF/A-3u
	// standard. It has all requirements of PDF/A-2b.
	ArchivalA3U
	// ArchivalA4 is Archival::A4, the validator for the PDF/A-4 standard.
	// While not required, it's recommended to enable tagging.
	ArchivalA4
	// ArchivalA4F is Archival::A4F, the validator for the PDF/A-4f
	// standard. It has all requirements of PDF/A-4.
	ArchivalA4F
	// ArchivalA4E is Archival::A4E, the validator for the PDF/A-4e
	// standard. It has all requirements of PDF/A-4.
	ArchivalA4E
)

// isA1 reports whether the validator is one of PDF/A-1.
func (a Archival) isA1() bool { return a == ArchivalA1A || a == ArchivalA1B }

// isA2A3 reports whether the validator is one of PDF/A-2 or PDF/A-3.
func (a Archival) isA2A3() bool { return a >= ArchivalA2A && a <= ArchivalA3U }

// isA2 reports whether the validator is one of PDF/A-2.
func (a Archival) isA2() bool { return a >= ArchivalA2A && a <= ArchivalA2U }

// isA3 reports whether the validator is one of PDF/A-3.
func (a Archival) isA3() bool { return a >= ArchivalA3A && a <= ArchivalA3U }

// prohibits is Archival::prohibits in krilla src/configure/validate.rs. The
// arms of the match of upstream become one switch for each group of
// profiles. A case that no arm names is allowed.
func (a Archival) prohibits(err validationError) bool {
	k := err.kind
	if k == validationMissingXMPPropertyDescription || k == validationUnsupportedXMPValueType {
		return a.requiresXMPMetadataExtensionSchema()
	}
	switch {
	case a.isA1():
		switch k {
		// Forbidden under all PDF/A-1 profiles.
		case validationTooLongString, validationTooLongName, validationTooLongArray,
			validationTooLongDictionary, validationTooLargeFloat, validationTooManyIndirectObjects,
			validationTooHighQNestingLevel, validationContainsPostScript, validationMissingCMYKProfile,
			validationRestrictedLicense, validationMissingDocumentDate, validationTransparency,
			validationImageInterpolation, validationAnnotationHasConditionalAppearance,
			validationContainsMutatingAction, validationContainsAdditionalActions, validationEmbeddedPDF:
			return true
		case validationEmbeddedFile:
			// Only the existence of an embedded file is forbidden.
			return err.embed == embedErrorExistence
		// Forbidden under PDF/A-1a but allowed under PDF/A-1b.
		case validationContainsNotDefGlyph, validationNoCodepointMapping, validationNoDocumentLanguage,
			validationMissingAltText, validationMissingAnnotationAltText, validationMissingFieldAltName,
			validationMissingTagging:
			return a == ArchivalA1A
		}
		// Allowed under all PDF/A-1 profiles.
		return false
	case a.isA2A3():
		switch k {
		// Forbidden under all PDF/A-2 and PDF/A-3 profiles.
		case validationTooLongString, validationTooLongName, validationTooManyIndirectObjects,
			validationTooHighQNestingLevel, validationContainsPostScript, validationMissingCMYKProfile,
			validationInconsistentSeparationFallback, validationContainsNotDefGlyph,
			validationRestrictedLicense, validationMissingDocumentDate, validationImageInterpolation,
			validationAnnotationHasConditionalAppearance, validationContainsMutatingAction,
			validationContainsAdditionalActions, validationEmbeddedPDF:
			return true
		case validationEmbeddedFile:
			if err.embed == embedErrorExistence {
				// Forbidden under PDF/A-2 but allowed under PDF/A-3.
				return a.isA2()
			}
			// Forbidden under PDF/A-3 but allowed under PDF/A-2.
			return a.isA3()
		// Forbidden under PDF/A-2 and PDF/A-3 accessible profiles.
		case validationUnicodePrivateArea, validationNoDocumentLanguage, validationMissingAltText,
			validationMissingAnnotationAltText, validationMissingFieldAltName, validationMissingTagging:
			return a == ArchivalA2A || a == ArchivalA3A
		// Forbidden under PDF/A-2 and PDF/A-3 accessible and Unicode
		// profiles.
		case validationNoCodepointMapping, validationInvalidCodepointMapping:
			return a == ArchivalA2A || a == ArchivalA2U || a == ArchivalA3A || a == ArchivalA3U
		}
		// Allowed under all PDF/A-2 and PDF/A-3 profiles.
		return false
	default:
		switch k {
		// Forbidden under all PDF/A-4 profiles.
		case validationMissingCMYKProfile, validationInconsistentSeparationFallback,
			validationContainsNotDefGlyph, validationNoCodepointMapping,
			validationInvalidCodepointMapping, validationUnicodePrivateArea,
			validationRestrictedLicense, validationMissingDocumentDate, validationImageInterpolation,
			validationAnnotationHasConditionalAppearance, validationContainsMutatingAction,
			validationEmbeddedPDF:
			return true
		case validationEmbeddedFile:
			// The existence of an embedded file and a missing description
			// are forbidden under PDF/A-4 but allowed under the other
			// PDF/A-4 profiles. A missing date or MIME type is allowed.
			if err.embed == embedErrorExistence || err.embed == embedErrorMissingDescription {
				return a == ArchivalA4
			}
			return false
		}
		// Allowed under all PDF/A-4 profiles.
		return false
	}
}

// requiresCodepointMappings is Archival::requires_codepoint_mappings in
// krilla src/configure/validate.rs.
func (a Archival) requiresCodepointMappings() bool {
	return a != ArchivalA1B && a != ArchivalA2B && a != ArchivalA3B
}

// requiresNoDeviceCS is Archival::requires_no_device_cs in krilla
// src/configure/validate.rs.
func (Archival) requiresNoDeviceCS() bool { return true }

// requiresAnnotationFlags is Archival::requires_annotation_flags in krilla
// src/configure/validate.rs.
func (Archival) requiresAnnotationFlags() bool { return true }

// requiresTagging is Archival::requires_tagging in krilla
// src/configure/validate.rs.
func (a Archival) requiresTagging() bool {
	return a == ArchivalA1A || a == ArchivalA2A || a == ArchivalA3A
}

// requiresXMPMetadata is Archival::requires_xmp_metadata in krilla
// src/configure/validate.rs.
func (Archival) requiresXMPMetadata() bool { return true }

// requiresXMPMetadataExtensionSchema is
// Archival::requires_xmp_metadata_extension_schema in krilla
// src/configure/validate.rs.
func (a Archival) requiresXMPMetadataExtensionSchema() bool {
	// Clause 6.7.2.3 of PDF/A-4 recommends ("should") a RELAX NG
	// definition of its metadata contents to be embedded as an associated
	// file. It no longer uses the inline schema definition using the
	// "pdfaSchema" namespaces for extension schemata.
	return a.isA1() || a.isA2A3()
}

// usesXMP2005PredefinedSchemas is
// Archival::uses_xmp_2005_predefined_schemas in krilla
// src/configure/validate.rs.
//
// Whether the predefined XMP schemas follow the XMP 2005 specification
// (PDF/A-2 and PDF/A-3) rather than the smaller XMP 2004 set (PDF/A-1).
func (a Archival) usesXMP2005PredefinedSchemas() bool {
	return a.isA2A3()
}

// prohibitsInstanceIDInXMPMetadata is
// Archival::prohibits_instance_id_in_xmp_metadata in krilla
// src/configure/validate.rs.
func (a Archival) prohibitsInstanceIDInXMPMetadata() bool {
	return a.isA1()
}

// requiresFileProvenanceInformation is
// Archival::requires_file_provenance_information in krilla
// src/configure/validate.rs.
func (Archival) requiresFileProvenanceInformation() bool { return true }

// prohibitsInfoDict is Archival::prohibits_info_dict in krilla
// src/configure/validate.rs.
func (a Archival) prohibitsInfoDict() bool {
	return a >= ArchivalA4
}

// requiresBinaryHeader is Archival::requires_binary_header in krilla
// src/configure/validate.rs.
func (Archival) requiresBinaryHeader() bool { return true }

// requiresEmbeddedFilesWhenEmpty is
// Archival::requires_embedded_files_when_empty in krilla
// src/configure/validate.rs.
func (a Archival) requiresEmbeddedFilesWhenEmpty() bool {
	return a == ArchivalA4F
}

// recommendsXMPRelaxNGSchema is Archival::recommends_xmp_relax_ng_schema in
// krilla src/configure/validate.rs.
//
// Whether the XMP metadata should be described by a RELAX NG schema in an
// associated file.
func (a Archival) recommendsXMPRelaxNGSchema() bool {
	return a == ArchivalA4F || a == ArchivalA4E
}

// specifiesAssociatedFiles is Archival::specifies_associated_files in
// krilla src/configure/validate.rs.
//
// Whether this standard explicitly specifies the /AF key.
func (a Archival) specifiesAssociatedFiles() bool {
	return a.isA3()
}

// outputIntent is Archival::output_intent in krilla
// src/configure/validate.rs.
func (Archival) outputIntent() pdfwriter.OutputIntentSubtype {
	return pdfwriter.OutputIntentSubtypePDFA
}

// writeXMP is Archival::write_xmp in krilla src/configure/validate.rs.
func (a Archival) writeXMP(xmp *xmpwriter.Writer) {
	switch a {
	case ArchivalA1A:
		xmp.PDFAPart(1)
		xmp.PDFAConformance("A")
	case ArchivalA1B:
		xmp.PDFAPart(1)
		xmp.PDFAConformance("B")
	case ArchivalA2A:
		xmp.PDFAPart(2)
		xmp.PDFAConformance("A")
	case ArchivalA2B:
		xmp.PDFAPart(2)
		xmp.PDFAConformance("B")
	case ArchivalA2U:
		xmp.PDFAPart(2)
		xmp.PDFAConformance("U")
	case ArchivalA3A:
		xmp.PDFAPart(3)
		xmp.PDFAConformance("A")
	case ArchivalA3B:
		xmp.PDFAPart(3)
		xmp.PDFAConformance("B")
	case ArchivalA3U:
		xmp.PDFAPart(3)
		xmp.PDFAConformance("U")
	case ArchivalA4:
		xmp.PDFAPart(4)
		xmp.PDFARev(2020)
	case ArchivalA4F:
		xmp.PDFAPart(4)
		xmp.PDFARev(2020)
		xmp.PDFAConformance("F")
	case ArchivalA4E:
		xmp.PDFAPart(4)
		xmp.PDFARev(2020)
		xmp.PDFAConformance("E")
	}
}

// extensionSchemas is Archival::extension_schemas in krilla
// src/configure/validate.rs.
func (a Archival) extensionSchemas() []Schema {
	if !a.requiresXMPMetadataExtensionSchema() {
		return nil
	}

	return []Schema{
		builtinSchema(xmpwriter.NamespaceXmpMedia).Property(
			NewPropertyDescription(
				"InstanceID",
				SimpleTypeText,
				"UUID based identifier for specific incarnation of a document",
			).Internal(),
		),
		builtinSchema(xmpwriter.NamespaceAdobePdf).
			Property(NewPropertyDescription(
				"Keywords",
				SimpleTypeText,
				"Keywords associated with the document",
			)).
			Property(NewPropertyDescription(
				"PDFVersion",
				SimpleTypeText,
				"Version of the PDF specification to which the document conforms",
			).Internal()).
			Property(NewPropertyDescription(
				"Producer",
				SimpleTypeText,
				"Name of the application that created the PDF document",
			).Internal()).
			Property(NewPropertyDescription(
				"Trapped",
				SimpleTypeText,
				"Whether the document has been trapped",
			).Internal()),
	}
}

// archivalNames holds the text of each validator of PDF/A.
var archivalNames = [...]string{
	ArchivalA1A: "PDF/A-1a",
	ArchivalA1B: "PDF/A-1b",
	ArchivalA2A: "PDF/A-2a",
	ArchivalA2B: "PDF/A-2b",
	ArchivalA2U: "PDF/A-2u",
	ArchivalA3A: "PDF/A-3a",
	ArchivalA3B: "PDF/A-3b",
	ArchivalA3U: "PDF/A-3u",
	ArchivalA4:  "PDF/A-4",
	ArchivalA4F: "PDF/A-4f",
	ArchivalA4E: "PDF/A-4e",
}

// String is Archival::as_str in krilla src/configure/validate.rs. A value
// that is not one of the constants gives Archival(n).
//
// Returns a human-readable string representation of the conformance level.
func (a Archival) String() string {
	if a == 0 || int(a) >= len(archivalNames) {
		return "Archival(" + strconv.Itoa(int(a)) + ")"
	}
	return archivalNames[a]
}

// min is Archival::min in krilla src/configure/validate.rs. Each validator
// of PDF/A has a minimum, so it returns the version only.
//
// Minimum PDF version required to use this standard, if any.
func (a Archival) min() PDFVersion {
	if a >= ArchivalA4 {
		return PDF20
	}
	// PDF/A-1 through 3 require XMP /Metadata streams, which require PDF
	// 1.4.
	return PDF14
}

// max is Archival::max in krilla src/configure/validate.rs.
//
// Maximum PDF version this standard can be used with.
func (a Archival) max() PDFVersion {
	switch {
	case a.isA1():
		return PDF14
	case a.isA2A3():
		return PDF17
	default:
		return PDF20
	}
}

// builtinSchema is builtin_schema in krilla src/configure/validate.rs.
func builtinSchema(namespace xmpwriter.Namespace) Schema {
	return NewSchema(NewNamespace(namespace.Prefix(), namespace.URL())).
		Name(namespace.Name() + " schema")
}
