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
