// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"errors"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/pdfwriter"
)

// The tests invalid_combination_2, invalid_combination_3,
// multi_validator_pdf_a3b_pdf_ua1 and multi_validator_pdfa2a_pdfua1 of krilla
// src/configure/mod.rs need the validator of PDF/UA, which pdf does not have
// (D99). The tests of src/configure/version.rs need the ICC profiles of the
// images.

// TestInvalidCombination1 is the test invalid_combination_1 of krilla
// src/configure/mod.rs.
func TestInvalidCombination1(t *testing.T) {
	// A1_B max is PDF 1.4; explicit PDF 1.7 is out of range.
	_, err := configurationBuilder{}.withVersion(PDF17).withArchivalValidator(ArchivalA1B).finish()
	if !errors.Is(err, okage.ErrPDFVersion) {
		t.Errorf("got %v, want ErrPDFVersion", err)
	}
}

// TestArchivalVersions checks that a validator without a version gives the
// highest version that the validator allows, as ConfigurationBuilder::finish
// of krilla src/configure/mod.rs does.
func TestArchivalVersions(t *testing.T) {
	for a := ArchivalA1A; a <= ArchivalA4E; a++ {
		config, err := configurationBuilder{}.withArchivalValidator(a).finish()
		if err != nil {
			t.Fatalf("%s: %v", a, err)
		}
		if config.version != a.max() {
			t.Errorf("%s gives %s, want %s", a, config.version, a.max())
		}
		if got, ok := config.validators.archival(); !ok || got != a {
			t.Errorf("%s gives the validator %s", a, got)
		}
	}
	if _, err := (configurationBuilder{}).withArchivalValidator(ArchivalA4E + 1).finish(); !errors.Is(err, okage.ErrPDFVersion) {
		t.Errorf("an unknown validator gives %v", err)
	}
	if got := ArchivalA2U.String(); got != "PDF/A-2u" {
		t.Errorf("ArchivalA2U is %q", got)
	}
}

// TestEmptyValidators is the test empty_validators of krilla
// src/configure/mod.rs.
func TestEmptyValidators(t *testing.T) {
	config, err := configurationBuilder{}.finish()
	if err != nil {
		t.Fatal(err)
	}
	if !config.validators.isEmpty() {
		t.Error("the validators are not empty")
	}
	if config.version != PDF17 {
		t.Errorf("the version is %s", config.version)
	}
}

// TestDefaultConfig is the test default_config of krilla
// src/configure/mod.rs.
func TestDefaultConfig(t *testing.T) {
	config, err := configurationBuilder{}.finish()
	if err != nil {
		t.Fatal(err)
	}
	if config != defaultConfiguration() {
		t.Errorf("got %+v, want %+v", config, defaultConfiguration())
	}
}

// TestWithVersion sets each version, and a version that pdf does not know.
func TestWithVersion(t *testing.T) {
	for v := PDFVersionMin; v <= PDFVersionMax; v++ {
		config, err := configurationBuilder{}.withVersion(v).finish()
		if err != nil || config.version != v {
			t.Errorf("the version %s gives %+v, %v", v, config, err)
		}
	}
	_, err := configurationBuilder{}.withVersion(PDF20 + 1).finish()
	if !errors.Is(err, okage.ErrPDFVersion) {
		t.Errorf("an unknown version gives %v", err)
	}

	d := NewDocument(WithPDFVersion(PDF20 + 1))
	if err := d.Finish(&bytes.Buffer{}); !errors.Is(err, okage.ErrPDFVersion) {
		t.Errorf("Finish gives %v, want ErrPDFVersion", err)
	}
}

// TestPDFVersion checks the methods of PdfVersion of krilla
// src/configure/version.rs.
func TestPDFVersion(t *testing.T) {
	cases := []struct {
		v          PDFVersion
		name       string
		header     string
		deprecated bool
	}{
		{PDF14, "PDF 1.4", "%PDF-1.4", false},
		{PDF15, "PDF 1.5", "%PDF-1.5", false},
		{PDF16, "PDF 1.6", "%PDF-1.6", false},
		{PDF17, "PDF 1.7", "%PDF-1.7", false},
		{PDF20, "PDF 2.0", "%PDF-2.0", true},
	}
	for _, c := range cases {
		if got := c.v.String(); got != c.name {
			t.Errorf("String gives %q, want %q", got, c.name)
		}
		pdf := pdfwriter.NewPdf()
		c.v.setVersion(pdf)
		if got := pdf.AsBytes(); !bytes.HasPrefix(got, []byte(c.header)) {
			t.Errorf("%s gives the header %q", c.name, got[:min(len(got), 8)])
		}
		if c.v.deprecatesProcSets() != c.deprecated || c.v.deprecatesCIDSet() != c.deprecated {
			t.Errorf("%s deprecates the procedure sets and the CIDSet: %t, %t", c.name, c.v.deprecatesProcSets(), c.v.deprecatesCIDSet())
		}
	}
	if got := (PDF20 + 1).String(); got != "PDFVersion(5)" {
		t.Errorf("an unknown version gives %q", got)
	}
}

// TestValidators checks that no validator prohibits an error, as an empty
// Validators of upstream does.
func TestValidators(t *testing.T) {
	var v validators
	for k := validationTooLongString; k <= validationUnsupportedXMPValueType; k++ {
		if _, ok := v.prohibits(validationError{kind: k}); ok {
			t.Errorf("%s is prohibited", k)
		}
		if k.String() == "" || k.String() == "ValidationError(?)" {
			t.Errorf("the kind %d has no name", k)
		}
	}
	if (validationUnsupportedXMPValueType + 1).String() != "ValidationError(?)" {
		t.Error("an unknown kind has a name")
	}
	if v.requiresNoDeviceCS() || v.requiresTagging() || v.requiresBinaryHeader() || v.hasAccessibility() {
		t.Error("an empty set of validators requires something")
	}
	for f, want := range map[versionedFeature]PDFVersion{
		featureStructureOrderTabbing:        PDF15,
		featureHeaderFooterArtifactSubtypes: PDF17,
		featureTableHeaderScope:             PDF15,
	} {
		if got := f.minimumPDFVersion(); got != want {
			t.Errorf("the feature %d needs %s, want %s", f, got, want)
		}
	}
}
