// Copyright 2019 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

import "testing"

// The tests of kurbo kurbo/src/point.rs. The test display is not ported,
// because the subset has no Display of Point.

func TestPointArithmetic(t *testing.T) {
	if got, want := NewPoint(0., 0.).SubVec(NewVec2(10., 0.)), NewPoint(-10., 0.); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := NewPoint(0., 0.).Sub(NewPoint(-5., 101.)), NewVec2(5., -101.); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPointDistance(t *testing.T) {
	p1 := NewPoint(0., 10.)
	p2 := NewPoint(0., 5.)
	if got := p1.Distance(p2); got != 5. {
		t.Errorf("got %v, want 5", got)
	}

	p1 = NewPoint(-11., 1.)
	p2 = NewPoint(-7., -2.)
	if got := p1.Distance(p2); got != 5. {
		t.Errorf("got %v, want 5", got)
	}
}
