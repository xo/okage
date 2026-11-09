// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/okage"
)

// errText checks that err is not nil and that its message ends with want,
// the Display text of the upstream error. The message of an error of okage
// starts with "parsing svg value: ", and the message of an error with data
// can add the name of its case before the text of upstream.
func errText(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Errorf("got no error, want %q", want)
		return
	}
	if !strings.HasSuffix(err.Error(), want) {
		t.Errorf("got error %q, want %q", err.Error(), want)
	}
}

// TestErrorsIs checks that each error with data wraps its sentinel error, and
// that the message ends with the Display text of upstream.
func TestErrorsIs(t *testing.T) {
	tests := []struct {
		err      error
		sentinel okage.Error
		want     string
	}{
		{errUnexpectedData(5), okage.ErrSVGTypesUnexpectedData, "parsing svg value: unexpected data at position 5"},
		{errInvalidNumber(1), okage.ErrSVGTypesInvalidNumber, "parsing svg value: invalid number at position 1"},
		{errInvalidChar([]byte{'?', '('}, 1), okage.ErrSVGTypesInvalidChar, "parsing svg value: invalid char: expected '(' not '?' at position 1"},
		{errInvalidString([]string{"url ", "url("}, 1), okage.ErrSVGTypesInvalidString, "parsing svg value: invalid string: expected 'url(' not 'url ' at position 1"},
		{errPercentageValue(6), okage.ErrSVGTypesFilterPercentageValue, "parsing svg value: a percentage value detected at position 6"},
	}
	for _, tt := range tests {
		if !errors.Is(tt.err, tt.sentinel) {
			t.Errorf("errors.Is(%v, %v) = false", tt.err, tt.sentinel)
		}
		if tt.err.Error() != tt.want {
			t.Errorf("got %q, want %q", tt.err.Error(), tt.want)
		}
	}
}
