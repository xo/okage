// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package font

// okage: this file is not in go-text. It tests the deltas of gvar in 16.16
// of D84.

import (
	"testing"

	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// TestInferDeltaFixed is the test interpolate of skrifa
// src/outline/glyf/deltas.rs, which takes the example of inferred deltas of
// the gvar chapter of the OpenType specification. The point (260, 700) lies
// between the points (245, 630) and (305, 680), whose deltas are (28, -62)
// and (-42, -57).
func TestInferDeltaFixed(t *testing.T) {
	fx := func(i int32) tables.Fixed { return tables.FixedFromInt(i) }
	// 10.4999237060547 in the 16.16 format, as skrifa gives it.
	if got := inferDelta(260, 245, 305, fx(28), fx(-42)); got != 688123 {
		t.Errorf("the delta x is %d, want 688123", got)
	}
	if got := inferDelta(700, 630, 680, fx(-62), fx(-57)); got != fx(-57) {
		t.Errorf("the delta y is %d, want %d", got, fx(-57))
	}
	// Two reference points at the same coordinate with other deltas give
	// no delta.
	if got := inferDelta(10, 20, 20, fx(1), fx(2)); got != 0 {
		t.Errorf("the delta is %d, want 0", got)
	}
}
