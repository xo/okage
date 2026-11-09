package crmath

import (
	"math"
	"math/big"
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

// hypotfValues holds results of hypotf of glibc 2.44 for random inputs, as
// bits: x, y and the result. A C program made them on 2026-11-09. On
// 2,000,000 random inputs, glibc gave the result of the exact square root
// rounded to an f32 each time.
var hypotfValues = [][3]uint32{
	{0x13bc5da9, 0x0ccf92d9, 0x13bc5da9},
	{0x437ba7c1, 0x43177290, 0x4392db22},
	{0x4d469201, 0x77383831, 0x77383831},
	{0x210dca65, 0x20d4302a, 0x213116cb},
	{0x416fc184, 0x686e07f3, 0x686e07f3},
	{0x16053510, 0x160065bb, 0x16390381},
	{0x46f0b221, 0x05b46391, 0x46f0b221},
	{0x141ba7a6, 0x0a7b2196, 0x141ba7a6},
	{0x08c14416, 0x08411276, 0x08d808db},
	{0x05a28dba, 0x5e5875f3, 0x5e5875f3},
	{0x4c314911, 0x4b0ef764, 0x4c34da35},
	{0x50f64425, 0x4ea53c20, 0x50f67b8e},
	{0x4acc3fb7, 0x4a80dbf0, 0x4af18004},
	{0x5c9ecb2a, 0x77e471ac, 0x77e471ac},
	{0x0ac8a5a1, 0x0956217b, 0x0aca6ca8},
	{0x5369774d, 0x4bd4ec7b, 0x5369774d},
	{0x77bd615a, 0x775f61a0, 0x77dbdce7},
	{0x6bec1402, 0x36481d40, 0x6bec1402},
	{0x51edc12c, 0x50335455, 0x51eecf19},
	{0x1e41f6fd, 0x4d9ea0c0, 0x4d9ea0c0},
	{0x28d48425, 0x28cd44d7, 0x2913bb47},
	{0x725ba365, 0x4b8f7c5a, 0x725ba365},
	{0x299384c7, 0x29060dd7, 0x29a20833},
	{0x434bed6a, 0x1d6d4036, 0x434bed6a},
	{0x3a08209e, 0x39fb804e, 0x3a39522f},
	{0x09c26ee6, 0x32303199, 0x32303199},
	{0x0af8ed02, 0x09e9b216, 0x0affb0a9},
	{0x3bc64155, 0x597304ad, 0x597304ad},
	{0x2e9a27a3, 0x2e7d43f2, 0x2ec77f8e},
	{0x00c09d6f, 0x0ed542ce, 0x0ed542ce},
	{0x205f9b8e, 0x1f6047ce, 0x206687c4},
	{0x6b1c5bbd, 0x21130c49, 0x6b1c5bbd},
	{0x06d663ec, 0x06aa85ea, 0x0708f7e7},
	{0x078083f1, 0x044ffdb3, 0x07808692},
	{0x6425ead0, 0x640d49a0, 0x6459ec74},
	{0x1958ca10, 0x4292065d, 0x4292065d},
	{0x3270e452, 0x3231969b, 0x3295a369},
	{0x29499996, 0x4a7055c4, 0x4a7055c4},
	{0x1866fe54, 0x16e98ac4, 0x1868d4b2},
	{0x4786835c, 0x1597867d, 0x4786835c},
}

// TestHypot32 checks Hypot32 against glibc, against math/big and on the
// special values.
func TestHypot32(t *testing.T) {
	for _, v := range hypotfValues {
		x, y := math.Float32frombits(v[0]), math.Float32frombits(v[1])
		if got := Hypot32(x, y); math.Float32bits(got) != v[2] {
			t.Errorf("Hypot32(%v, %v) = %v, want %v", x, y, got, math.Float32frombits(v[2]))
		}
	}
	inf, nan := float32(math.Inf(1)), float32(math.NaN())
	if got := Hypot32(nan, inf); got != inf {
		t.Errorf("Hypot32(NaN, Inf) = %v, want +Inf", got)
	}
	if got := Hypot32(nan, 1); got == got {
		t.Errorf("Hypot32(NaN, 1) = %v, want NaN", got)
	}
	if got := Hypot32(3, -4); got != 5 {
		t.Errorf("Hypot32(3, -4) = %v, want 5", got)
	}
	r := rand.New(rand.NewPCG(7, 8))
	for range 20000 {
		x := float32((r.Float64()*2 - 1) * 100)
		y := float32((r.Float64()*2 - 1) * 100)
		bx, by := newFloat(float64(x)), newFloat(float64(y))
		want, _ := new(big.Float).SetPrec(prec).Sqrt(add(mul(bx, bx), mul(by, by))).Float32()
		if got := Hypot32(x, y); got != want {
			t.Errorf("Hypot32(%v, %v) = %v, want %v", x, y, got, want)
		}
	}
}

// tanfValues holds results of tanf of glibc 2.44 for random inputs, as bits:
// x and the result. A C program made them on 2026-11-09 with cc -lm. glibc
// gives the correctly rounded result of tanf.
var tanfValues = [][2]uint32{
	{0xc01608a0, 0x3f831685},
	{0x3fcc4dc4, 0xc21de6b0},
	{0xc017d7ee, 0x3f77c204},
	{0xbf908206, 0xc0074d4e},
	{0x4044e14a, 0xbd8602eb},
	{0xbe969470, 0xbe9b13b5},
	{0x3fe41950, 0xc0953c91},
	{0x3f845008, 0x3fd6f20c},
	{0x3fe21810, 0xc0a189ee},
	{0x3febff10, 0xc064a524},
	{0xc064504d, 0xbee838b5},
	{0x4058aab2, 0x3e7ebefa},
	{0xc05a0fae, 0xbe8b4920},
	{0x405b7eda, 0x3e97afc6},
	{0x4044c0aa, 0xbd8a1b87},
	{0xc0510075, 0xbdff630e},
	{0xc051f939, 0xbe0f83a7},
	{0xbfad4582, 0xc09110d6},
	{0x3fbe42f4, 0x413d2c28},
	{0x404e7934, 0x3dad9520},
	{0xc04f7d33, 0xbdce5cc2},
	{0x3fb33658, 0x40b9a37c},
	{0xbff4298e, 0x4036d45f},
	{0xbfe879ce, 0x407f850e},
	{0x407ba05c, 0x3f81339b},
	{0xc00674fc, 0x3fda69fb},
	{0xbfc47f60, 0xc1e04052},
	{0x3fae8258, 0x40980a2c},
	{0x3ff70088, 0xc02a9881},
	{0xc00a7100, 0x3fbe33f3},
	{0x445b169c, 0xbe1be484},
	{0x446016d6, 0x3fc8a0ba},
	{0x44568f9f, 0x3f2b242e},
	{0xc4333241, 0xbf0c8cba},
	{0xc3c0f14c, 0x3f1641ff},
	{0x441cd3a1, 0xbfcca1e7},
	{0x4351b083, 0xbf830f51},
	{0xc46b184a, 0xbfdbba1c},
	{0xc2e48a59, 0xc0184177},
	{0xc3ae5c41, 0xbb816793},
}

// TestTan32 checks Tan32 against glibc, against math/big and on the special
// values.
func TestTan32(t *testing.T) {
	for _, v := range tanfValues {
		x := math.Float32frombits(v[0])
		if got := Tan32(x); math.Float32bits(got) != v[1] {
			t.Errorf("Tan32(%v) = %v, want %v", x, got, math.Float32frombits(v[1]))
		}
	}
	if got := Tan32(float32(math.Inf(1))); got == got {
		t.Errorf("Tan32(+Inf) = %v, want NaN", got)
	}
	if got := Tan32(float32(math.Copysign(0, -1))); got != 0 || !math.Signbit(float64(got)) {
		t.Errorf("Tan32(-0) = %v, want -0", got)
	}
	r := rand.New(rand.NewPCG(9, 10))
	for range 2000 {
		x := float32((r.Float64()*2 - 1) * 10)
		s, c := sinCosBig(float64(x))
		want, _ := quoP(s, c, wprec).Float32()
		if got := Tan32(x); got != want {
			t.Errorf("Tan32(%v) = %v, want %v", x, got, want)
		}
	}
}
