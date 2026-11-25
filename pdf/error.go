// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"

	"github.com/xo/okage"
)

// The enum KrillaError of krilla src/error.rs becomes the sentinel errors
// ErrPDF of the root package (D21, D62). A case with data wraps its sentinel
// error with the data, as validationFailed does. The function of a case can
// be in the file of its unit: fontError of KrillaError::Font is in
// text_cid.go, the error of DuplicateNamedDestination is in destination.go,
// and the errors of KrillaError::Image and SixteenBitImage are in
// graphics_image.go, and XMPError of KrillaError::Xmp is in metadata_xmp.go.
// KrillaError::StandardFont, and the cases of the tags and of embedded PDF
// files, are not in pdf (D62).

// limitError is LimitError in krilla src/error.rs.
//
// A limit imposed by the selected PDF version.
type limitError uint8

// The cases of LimitError.
const (
	// limitTooLargeFloat is LimitError::TooLargeFloat. A float exceeded the
	// maximum allowed size for the PDF version.
	limitTooLargeFloat limitError = iota
	// limitTooLongArray is LimitError::TooLongArray. An array exceeded the
	// maximum allowed length for the PDF version.
	limitTooLongArray
	// limitTooLongDictionary is LimitError::TooLongDictionary. A dictionary
	// exceeded the maximum allowed number of entries for the PDF version.
	limitTooLongDictionary
)

// err is KrillaError::Limit of krilla src/error.rs. It returns the sentinel
// error of the case, whose text is the Display text of upstream.
func (e limitError) err() error {
	switch e {
	case limitTooLargeFloat:
		return okage.ErrPDFTooLargeFloat
	case limitTooLongArray:
		return okage.ErrPDFTooLongArray
	default:
		return okage.ErrPDFTooLongDictionary
	}
}

// validationFailedError is KrillaError::Validation of krilla src/error.rs: the
// validation errors with the validators that raised them. It wraps
// ErrPDFValidation, and its text gives the number of errors, as the Display
// text of upstream does. The list of errors is not public (D99).
type validationFailedError struct {
	entries []validationEntry
}

// Error gives the text of ErrPDFValidation with the number of errors.
func (e *validationFailedError) Error() string {
	count := len(e.entries)
	noun := "errors"
	if count == 1 {
		noun = "error"
	}
	return fmt.Sprintf("%s with %d %s", okage.ErrPDFValidation, count, noun)
}

// Unwrap returns ErrPDFValidation.
func (e *validationFailedError) Unwrap() error {
	return okage.ErrPDFValidation
}

// validationFailed returns KrillaError::Validation of krilla src/error.rs
// for the errors.
func validationFailed(errors []validationEntry) error {
	return &validationFailedError{entries: errors}
}

// locationText is write_location of krilla src/error.rs. It gives the text
// that follows the message of an error at a location, or nothing when the
// location is not set.
func locationText(location Location) string {
	if location == 0 {
		return ""
	}
	return fmt.Sprintf(" at location %d", uint64(location))
}
