// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

import (
	"math"
	"testing"
)

// The tests of kurbo kurbo/src/line.rs. The tests line_reversed,
// line_midpoint, line_is_finite and line_nearest are not ported, because the
// subset has no Line::reversed, Line::midpoint, Line::is_finite and no
// ParamCurveNearest.

func TestLineArclen(t *testing.T) {
	l := NewLine(NewPoint(0.0, 0.0), NewPoint(1.0, 1.0))
	trueLen := math.Sqrt(2.0)
	epsilon := 1e-9
	if !(l.Arclen(epsilon)-trueLen < epsilon) {
		t.Errorf("arclen = %v, want %v", l.Arclen(epsilon), trueLen)
	}

	tt := l.InvArclen(trueLen/3.0, epsilon)
	if !(math.Abs(tt-1.0/3.0) < epsilon) {
		t.Errorf("inv_arclen = %v, want 1/3", tt)
	}
}
