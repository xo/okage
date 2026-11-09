package wide

import (
	"math"
	"testing"
)

func TestTruncI32(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	tests := []struct {
		in   float32
		want int32
	}{
		{0, 0},
		{1.9, 1},
		{-1.9, -1},
		{nan, 0},
		{inf, math.MaxInt32},
		{-inf, math.MinInt32},
		{3e9, math.MaxInt32},
		{-3e9, math.MinInt32},
		{2147483520, 2147483520},
	}
	for _, test := range tests {
		if got := TruncI32(test.in); got != test.want {
			t.Errorf("TruncI32(%v) = %d, want %d", test.in, got, test.want)
		}
	}
}

func TestTruncU32(t *testing.T) {
	tests := []struct {
		in   float32
		want uint32
	}{
		{-1, 0},
		{float32(math.NaN()), 0},
		{5e9, math.MaxUint32},
		{7.5, 7},
	}
	for _, test := range tests {
		if got := TruncU32(test.in); got != test.want {
			t.Errorf("TruncU32(%v) = %d, want %d", test.in, got, test.want)
		}
	}
}

func TestTruncU8(t *testing.T) {
	tests := []struct {
		in   float32
		want uint8
	}{
		{-1, 0},
		{254.9, 254},
		{300, 255},
		{float32(math.NaN()), 0},
	}
	for _, test := range tests {
		if got := TruncU8(test.in); got != test.want {
			t.Errorf("TruncU8(%v) = %d, want %d", test.in, got, test.want)
		}
	}
}

func TestTrunc64I32(t *testing.T) {
	tests := []struct {
		in   float64
		want int32
	}{
		{2147483647.9, math.MaxInt32},
		{-2147483649, math.MinInt32},
		{math.NaN(), 0},
		{-7.9, -7},
	}
	for _, test := range tests {
		if got := Trunc64I32(test.in); got != test.want {
			t.Errorf("Trunc64I32(%v) = %d, want %d", test.in, got, test.want)
		}
	}
}

func TestTruncI64(t *testing.T) {
	tests := []struct {
		in   float32
		want int64
	}{
		{9223371487098961920, 9223371487098961920},
		{-9223371487098961920, -9223371487098961920},
		{float32(math.Inf(1)), math.MaxInt64},
		{float32(math.NaN()), 0},
		{4611686018427387904, 4611686018427387904},
	}
	for _, test := range tests {
		if got := TruncI64(test.in); got != test.want {
			t.Errorf("TruncI64(%v) = %d, want %d", test.in, got, test.want)
		}
	}
}

func TestTrunc64I64(t *testing.T) {
	tests := []struct {
		in   float64
		want int64
	}{
		{9223372036854774784, 9223372036854774784},
		{-9223372036854774784, -9223372036854774784},
		{math.NaN(), 0},
		{1e300, math.MaxInt64},
	}
	for _, test := range tests {
		if got := Trunc64I64(test.in); got != test.want {
			t.Errorf("Trunc64I64(%v) = %d, want %d", test.in, got, test.want)
		}
	}
}
