// Copyright 2019 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "testing"

// usvg src/parser/units.rs has no unit tests. This test checks the port of
// f32::powi, a product of squares in f32.

func TestPowi32(t *testing.T) {
	for _, c := range []struct {
		a    float32
		b    int32
		want float32
	}{
		{1.2, 0, 1},
		{1.2, 1, 1.2},
		{1.2, 2, float32(1.2) * float32(1.2)},
		{1.2, 3, float32(float32(1.2)*float32(1.2)) * float32(1.2)},
		{1.2, -1, 1 / float32(1.2)},
		{1.2, -3, 1 / (float32(float32(1.2)*float32(1.2)) * float32(1.2))},
		{2, 10, 1024},
	} {
		if got := powi32(c.a, c.b); got != c.want {
			t.Errorf("powi32(%v, %d) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
