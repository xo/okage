// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"

	"github.com/xo/okage"
)

// This file ports krilla src/configure/mod.rs. The types Configuration and
// ConfigurationBuilder of upstream are public. The port keeps them
// unexported, and the options WithPDFVersion and WithArchivalValidator set
// the version and the validator through them. NewDocument builds the
// configuration after the last option, so the order of the options does
// not matter (D99). pdf has no validator of PDF/UA, so the builder has no
// method with_accessibility_validator.
//
// Configuring PDF version and export mode.

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
// src/configure/mod.rs. The field validators of upstream, a
// ValidatorsBuilder, is the validator of PDF/A. Zero is None.
//
// A configuration of validator and PDF version.
type configurationBuilder struct {
	archival   Archival
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

// withArchivalValidator is ConfigurationBuilder::with_archival_validator in
// krilla src/configure/mod.rs.
//
// Set the PDF/A validator, overwriting the current one if already set.
func (b configurationBuilder) withArchivalValidator(archival Archival) configurationBuilder {
	b.archival = archival
	return b
}

// finish is ConfigurationBuilder::finish in krilla src/configure/mod.rs.
// ConfigurationError::VersionDoesNotMatchValidatorsRange is ErrPDFVersion,
// also for a version or a validator that is not one of the constants. The
// error NoOverlappingValidatorsRange needs two validators, which pdf does
// not have.
//
// Build the Configuration, returning an error if the validators and version
// are incompatible.
func (b configurationBuilder) finish() (configuration, error) {
	if b.archival != 0 && (b.archival < ArchivalA1A || b.archival > ArchivalA4E) {
		return configuration{}, fmt.Errorf("%w: %s", okage.ErrPDFVersion, b.archival)
	}
	validators := validators{a: b.archival}

	low, ok := validators.min()
	if !ok {
		low = PDFVersionMin
	}
	high := validators.max()
	switch {
	case b.hasVersion && low <= b.version && b.version <= high:
		return configuration{validators: validators, version: b.version}, nil
	case b.hasVersion:
		return configuration{}, fmt.Errorf("%w: %s", okage.ErrPDFVersion, b.version)
	case !validators.isEmpty():
		return configuration{validators: validators, version: high}, nil
	default:
		return configuration{validators: validators, version: defaultConfiguration().version}, nil
	}
}
