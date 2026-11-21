// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package harfbuzz

import (
	"math"
	"testing"
)

// okage: the tests of D81.

// TestRoundf compares roundf with floor(x + .5), the roundf of HarfBuzz
// 15.1.0 (D90), on one float32 in 97, on the values next to each half and
// to each limit of an int32, and on NaN and the infinities. A run over
// every float32 gave no difference.
func TestRoundf(t *testing.T) {
	check := func(f float32) {
		t.Helper()
		if got, want := roundf(f), Position(math.Floor(float64(f)+0.5)); got != want {
			t.Fatalf("roundf(%v), bits %#x: got %d, want %d", f, math.Float32bits(f), got, want)
		}
	}
	for b := uint64(0); b < 1<<32; b += 97 {
		check(math.Float32frombits(uint32(b)))
	}
	for k := -1000; k <= 1000; k++ {
		half := float32(k) + 0.5
		check(half)
		check(math.Nextafter32(half, 0))
		check(math.Nextafter32(half, float32(2*k)))
	}
	for _, f := range []float32{
		0, float32(math.Copysign(0, -1)), 1 << 31, -(1 << 31), 1 << 23, -(1 << 23), 1 << 24,
		math.MaxFloat32, -math.MaxFloat32, math.SmallestNonzeroFloat32,
		float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN()),
	} {
		check(f)
		check(math.Nextafter32(f, 0))
		check(math.Nextafter32(f, float32(math.Inf(1))))
		check(math.Nextafter32(f, float32(math.Inf(-1))))
	}
}
