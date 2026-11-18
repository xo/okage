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

// The tests invalid_combination_1 to invalid_combination_3,
// multi_validator_pdf_a3b_pdf_ua1 and multi_validator_pdfa2a_pdfua1 of krilla
// src/configure/mod.rs need the validators, which the base port leaves out
// (D62). The tests of src/configure/version.rs need the ICC profiles of the
// images.

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
	for k := validationTooLongString; k <= validationRequiresNewerPDFVersion; k++ {
		if _, ok := v.prohibits(validationError{kind: k}); ok {
			t.Errorf("%s is prohibited", k)
		}
		if k.String() == "" || k.String() == "ValidationError(?)" {
			t.Errorf("the kind %d has no name", k)
		}
	}
	if (validationRequiresNewerPDFVersion + 1).String() != "ValidationError(?)" {
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
