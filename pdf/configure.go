// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"

	"github.com/xo/okage"
)

// The validators of PDF/A and PDF/UA are not in the base port (D62), so a
// configuration holds a version and an empty set of validators. The types
// Configuration and ConfigurationBuilder of upstream are public. The port
// keeps them unexported until a decision adds the validators, and the option
// WithPDFVersion sets the version through them.

// configuration is Configuration in krilla src/configure/mod.rs.
//
// A configuration of validator and PDF version.
type configuration struct {
	validators validators
	version    PDFVersion
}

// defaultConfiguration is the Default of Configuration in krilla
// src/configure/mod.rs.
func defaultConfiguration() configuration {
	return configuration{version: PDF17}
}

// configurationBuilder is ConfigurationBuilder in krilla
// src/configure/mod.rs, without the validators. The methods set_validator,
// with_archival_validator and with_accessibility_validator wait for them.
//
// A configuration of validator and PDF version.
type configurationBuilder struct {
	version    PDFVersion
	hasVersion bool
}

// withVersion is ConfigurationBuilder::with_version in krilla
// src/configure/mod.rs.
//
// Set the PDF version, overwriting the current one if already set.
func (b configurationBuilder) withVersion(version PDFVersion) configurationBuilder {
	b.version = version
	b.hasVersion = true
	return b
}

// finish is ConfigurationBuilder::finish in krilla src/configure/mod.rs.
// Upstream narrows the range of the versions to the range that each
// validator allows. With no validator, the range is PDFVersionMin to
// PDFVersionMax. So the port has only the error
// ConfigurationError::VersionDoesNotMatchValidatorsRange, as ErrPDFVersion,
// for a version that is not one of the constants of PDFVersion. The error
// NoOverlappingValidatorsRange needs two validators.
//
// Build the Configuration, returning an error if the validators and version
// are incompatible.
func (b configurationBuilder) finish() (configuration, error) {
	var validators validators
	low, high := PDFVersionMin, PDFVersionMax
	switch {
	case b.hasVersion && low <= b.version && b.version <= high:
		return configuration{validators: validators, version: b.version}, nil
	case b.hasVersion:
		return configuration{}, fmt.Errorf("%w: %s", okage.ErrPDFVersion, b.version)
	default:
		return configuration{validators: validators, version: defaultConfiguration().version}, nil
	}
}
