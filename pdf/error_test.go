// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"errors"
	"testing"

	"github.com/xo/okage"
)

// TestKrillaErrorImplementsError is the test krilla_error_implements_error of
// krilla src/error.rs. The text of a limit error is the Display text of
// upstream after "writing pdf: ", and the error is its sentinel error, as
// the source of upstream.
func TestKrillaErrorImplementsError(t *testing.T) {
	err := limitTooLongArray.err()
	if got, want := err.Error(), "writing pdf: PDF version limit exceeded: an array exceeded the maximum allowed length"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !errors.Is(err, okage.ErrPDFTooLongArray) {
		t.Errorf("%v is not ErrPDFTooLongArray", err)
	}
	for e, want := range map[limitError]error{
		limitTooLargeFloat:     okage.ErrPDFTooLargeFloat,
		limitTooLongArray:      okage.ErrPDFTooLongArray,
		limitTooLongDictionary: okage.ErrPDFTooLongDictionary,
	} {
		if got := e.err(); !errors.Is(got, want) {
			t.Errorf("the limit %d gives %v, want %v", e, got, want)
		}
	}
}

// TestValidationFailed checks the text of KrillaError::Validation.
func TestValidationFailed(t *testing.T) {
	one := []validationEntry{{err: validationError{kind: validationTransparency}}}
	two := []validationEntry{one[0], {err: validationError{kind: validationNoDocumentTitle}}}
	for _, c := range []struct {
		errors []validationEntry
		want   string
	}{
		{one, "writing pdf: validation failed with 1 error"},
		{two, "writing pdf: validation failed with 2 errors"},
	} {
		err := validationFailed(c.errors)
		if err.Error() != c.want {
			t.Errorf("got %q, want %q", err, c.want)
		}
		if !errors.Is(err, okage.ErrPDFValidation) {
			t.Errorf("%v is not ErrPDFValidation", err)
		}
	}
}

// TestFontError checks the text of KrillaError::Font, which names the font
// when it has a PostScript name, and the impl Debug for Font of krilla
// src/text/font.rs, which String ports.
func TestFontError(t *testing.T) {
	named := &Font{info: &fontInfo{postscriptName: "NotoSans-Regular", hasPostscriptName: true, index: 2}}
	unnamed := &Font{info: &fontInfo{}}
	for _, c := range []struct {
		font        *Font
		err, string string
	}{
		{
			named,
			"writing pdf: failed to embed font NotoSans-Regular: failed to subset font",
			`Font { postscript_name: Some("NotoSans-Regular"), index: 2, .. }`,
		},
		{
			unnamed,
			"writing pdf: failed to embed font: failed to subset font",
			"Font { postscript_name: None, index: 0, .. }",
		},
	} {
		err := fontError(c.font, "failed to subset font")
		if err.Error() != c.err {
			t.Errorf("got %q, want %q", err, c.err)
		}
		if !errors.Is(err, okage.ErrPDFFont) {
			t.Errorf("%v is not ErrPDFFont", err)
		}
		if got := c.font.String(); got != c.string {
			t.Errorf("String gives %q, want %q", got, c.string)
		}
	}
}

// TestLocationText checks write_location of krilla src/error.rs.
func TestLocationText(t *testing.T) {
	if got := locationText(0); got != "" {
		t.Errorf("no location gives %q", got)
	}
	if got := locationText(42); got != " at location 42" {
		t.Errorf("the location 42 gives %q", got)
	}
}
