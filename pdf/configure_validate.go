// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import "github.com/xo/okage/internal/siphasher"

// The validators of PDF/A and PDF/UA of krilla src/configure/validate.rs are
// not in the base port (D62). The serializer still records each validation
// error where upstream records it, so that a later decision adds the
// validators without a change to the code that records the errors. With no
// validator, no validator prohibits an error, and the serializer drops it, as
// upstream does with an empty set of validators.

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
	// validationEmbeddedFile is ValidationError::EmbeddedFile. The embedded
	// files are not in the base port.
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
// as the hash of the font, so that the value stays comparable. The cases of
// the separation colors and of the embedded files carry types that the base
// port leaves out.
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

// validators is Validators in krilla src/configure/validate.rs, with no
// validator. Upstream holds an optional validator of PDF/A and one of
// PDF/UA. The methods below are the ones that the serializer calls. Each one
// gives the answer of upstream for an empty set. A unit that calls another
// method of Validators adds it, with the same answer.
//
// Collection of validators with at most one validator for each standard.
type validators struct{}

// prohibits is Validators::prohibits in krilla src/configure/validate.rs. No
// validator prohibits an error, so it returns false.
//
// Returns a filtered Validators containing only validators that prohibit the
// given error, or None if no validator prohibits it.
func (v validators) prohibits(validationError) (validators, bool) {
	return v, false
}

// isEmpty is Validators::is_empty in krilla src/configure/validate.rs.
//
// Returns true if no validators are set.
func (validators) isEmpty() bool {
	return true
}

// requiresNoDeviceCS is Validators::requires_no_device_cs in krilla
// src/configure/validate.rs.
//
// Force sRGB profiles for DeviceGray and DeviceRgb colorspaces.
func (validators) requiresNoDeviceCS() bool {
	return false
}

// requiresTagging is Validators::requires_tagging in krilla
// src/configure/validate.rs.
//
// Whether Tagged PDF must be enabled.
func (validators) requiresTagging() bool {
	return false
}

// requiresBinaryHeader is Validators::requires_binary_header in krilla
// src/configure/validate.rs.
//
// Whether a non-printable file header is mandatory.
func (validators) requiresBinaryHeader() bool {
	return false
}

// hasAccessibility is Validators::accessibility in krilla
// src/configure/validate.rs, which returns the validator of PDF/UA, if set.
// The port says whether one is set.
func (validators) hasAccessibility() bool {
	return false
}
