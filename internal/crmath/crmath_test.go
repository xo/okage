package crmath

import (
	"math"
	"math/rand/v2"
	"testing"
)

// TestSpecialValues checks the results that are exact or defined.
func TestSpecialValues(t *testing.T) {
	if got := Acos64(1); got != 0 {
		t.Errorf("Acos64(1) = %v, want 0", got)
	}
	if got := Acos64(-1); got != math.Pi {
		t.Errorf("Acos64(-1) = %v, want %v", got, math.Pi)
	}
	if got := Acos64(0); got != math.Pi/2 {
		t.Errorf("Acos64(0) = %v, want %v", got, math.Pi/2)
	}
	if got := Cos64(0); got != 1 {
		t.Errorf("Cos64(0) = %v, want 1", got)
	}
	if got := Acos64(2); got == got {
		t.Errorf("Acos64(2) = %v, want NaN", got)
	}
	if got := Cos32(0); got != 1 {
		t.Errorf("Cos32(0) = %v, want 1", got)
	}
	if got := Sin32(0); got != 0 {
		t.Errorf("Sin32(0) = %v, want 0", got)
	}
	if got := Pow32(8, 1.0/3); got != 2 {
		t.Errorf("Pow32(8, 1/3) = %v, want 2", got)
	}
	if got := Pow32(0, 2); got != 0 {
		t.Errorf("Pow32(0, 2) = %v, want 0", got)
	}
}

// TestF32AgreesWithBig checks that the fast path of each f32 function gives
// the result of math/big, on random inputs.
func TestF32AgreesWithBig(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for range 2000 {
		x := float32(r.Float64()*2 - 1)
		if want, _ := acosBig(newFloat(float64(x))).Float32(); Acos32(x) != want {
			t.Errorf("Acos32(%v) = %v, want %v", x, Acos32(x), want)
		}
		y := float32((r.Float64()*2 - 1) * 10)
		if want, _ := cosBig(newFloat(float64(y))).Float32(); Cos32(y) != want {
			t.Errorf("Cos32(%v) = %v, want %v", y, Cos32(y), want)
		}
		if want, _ := sinBig(newFloat(float64(y))).Float32(); Sin32(y) != want {
			t.Errorf("Sin32(%v) = %v, want %v", y, Sin32(y), want)
		}
		p := float32(r.Float64() * 4)
		initConsts()
		if want, _ := expBig(mul(lnBig(newFloat(float64(p))), newFloat(float64(float32(1.0/3))))).Float32(); Pow32(p, 1.0/3) != want {
			t.Errorf("Pow32(%v, 1/3) = %v, want %v", p, Pow32(p, 1.0/3), want)
		}
	}
}

// TestF64KnownValues checks results that glibc 2.44 gives correctly
// rounded. math.Acos of Go gives 0.18366521709006278 for the first input, three
// units in the last place away.
func TestF64KnownValues(t *testing.T) {
	if got, want := Acos64(0.9831809036248165), 0.18366521709006287; got != want {
		t.Errorf("Acos64(0.9831809036248165) = %v, want %v", got, want)
	}
	if got, want := Acos64(0.5), 1.0471975511965979; got != want {
		t.Errorf("Acos64(0.5) = %v, want %v", got, want)
	}
	if got, want := Cos64(1), 0.5403023058681398; got != want {
		t.Errorf("Cos64(1) = %v, want %v", got, want)
	}
}

func BenchmarkAcos64(b *testing.B) {
	for i := range b.N {
		Acos64(float64(i%1000)/1000 - 0.5)
	}
}

func BenchmarkCos32(b *testing.B) {
	for i := range b.N {
		Cos32(float32(i%1000) / 100)
	}
}

// TestAtan2_32 checks Atan2_32 against atan2f of glibc 2.44, which gave the
// same bits as Atan2_32 on 200,000 random inputs on 2026-11-09, and against
// math/big on random inputs.
func TestAtan2_32(t *testing.T) {
	for _, c := range []struct {
		y, x float32
		want uint32
	}{
		{1, 2, 0x3eed6338},
		{-3, -4, 0xc01fe0bb},
		{1, -1, 0x4016cbe4},
		{0, -1, math.Float32bits(float32(math.Pi))},
		{0, 1, 0},
	} {
		if got := Atan2_32(c.y, c.x); math.Float32bits(got) != c.want {
			t.Errorf("Atan2_32(%v, %v) = %v, want %v", c.y, c.x, got, math.Float32frombits(c.want))
		}
	}
	if got := Atan2_32(float32(math.NaN()), 1); got == got {
		t.Errorf("Atan2_32(NaN, 1) = %v, want NaN", got)
	}
	r := rand.New(rand.NewPCG(5, 6))
	for range 2000 {
		y := float32((r.Float64()*2 - 1) * 100)
		x := float32((r.Float64()*2 - 1) * 100)
		if y == 0 || x == 0 {
			continue
		}
		if want, _ := atan2Big(float64(y), float64(x)).Float32(); Atan2_32(y, x) != want {
			t.Errorf("Atan2_32(%v, %v) = %v, want %v", y, x, Atan2_32(y, x), want)
		}
	}
}
