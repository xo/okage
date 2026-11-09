// Package crmath gives correctly rounded results of the math functions that
// tiny-skia, kurbo and svgtypes call from the C library: acos, cos, sin, tan,
// atan2, pow, ln and log2 for an f64, and acos, cos, sin, atan2 and pow for
// an f32.
// A correctly rounded result is the exact value rounded to the nearest float,
// so it is the same on every platform.
//
// Rust calls the C library for these functions, and the C libraries differ.
// On 2026-11-09, glibc 2.44 gave the correctly rounded result of acos and cos
// for an f64 in 99.9 percent of 20,000 random inputs, and math.Acos and
// math.Cos of Go gave it in about 70 percent. See D26.
//
// The functions of an f32 compute the result for an f64 with the package math,
// and round it to an f32. If that result is too close to the middle of two
// f32 values to decide the rounding, they compute it again with math/big.
// Acos64 computes with math/big at 160 bits of precision, and the other
// functions of an f64, in f64.go, at 192 bits.
package crmath

import (
	"math"
	"math/big"
	"sync"
)

// prec is the precision in bits of the computations with math/big.
const prec = 160

// consts holds the constants of the computations with math/big. They are made
// on first use.
var consts struct {
	once sync.Once
	pi   *big.Float
	ln2  *big.Float
	eps  *big.Float
}

// initConsts makes the constants of the computations with math/big.
func initConsts() {
	consts.once.Do(func() {
		consts.eps = new(big.Float).SetPrec(prec).SetMantExp(newFloat(1), -prec-8)
		// pi = 16 atan(1/5) - 4 atan(1/239)
		a := atanSmall(quo(newFloat(1), newFloat(5)))
		b := atanSmall(quo(newFloat(1), newFloat(239)))
		consts.pi = sub(mul(a, newFloat(16)), mul(b, newFloat(4)))
		// ln 2 = 2 atanh(1/3)
		consts.ln2 = scale(atanhSmall(quo(newFloat(1), newFloat(3))), 1)
	})
}

// Acos64 is acos for an f64, correctly rounded.
func Acos64(x float64) float64 {
	switch {
	case x != x || x < -1 || x > 1:
		return math.NaN()
	case x == 1:
		return 0
	}
	f, _ := acosBig(newFloat(x)).Float64()
	return f
}

// Cos64 is cos for an f64, correctly rounded. It reduces the argument with
// as many bits of pi as the exponent of x needs, in f64.go.
func Cos64(x float64) float64 {
	return cos64(x)
}

// Acos32 is acosf for an f32, correctly rounded.
func Acos32(x float32) float32 {
	switch {
	case x != x || x < -1 || x > 1:
		return float32(math.NaN())
	case x == 1:
		return 0
	}
	if r, ok := round32(math.Acos(float64(x))); ok {
		return r
	}
	f, _ := acosBig(newFloat(float64(x))).Float32()
	return f
}

// Cos32 is cosf for an f32, correctly rounded.
func Cos32(x float32) float32 {
	if x != x || math.IsInf(float64(x), 0) {
		return float32(math.NaN())
	}
	if r, ok := round32(math.Cos(float64(x))); ok {
		return r
	}
	f, _ := cosBig(newFloat(float64(x))).Float32()
	return f
}

// Sin32 is sinf for an f32, correctly rounded.
func Sin32(x float32) float32 {
	if x != x || math.IsInf(float64(x), 0) {
		return float32(math.NaN())
	}
	if x == 0 {
		return x
	}
	if r, ok := round32(math.Sin(float64(x))); ok {
		return r
	}
	f, _ := sinBig(newFloat(float64(x))).Float32()
	return f
}

// Pow32 is powf for an f32, correctly rounded for a finite positive x. Other
// cases give the result of math.Pow, which follows the special cases of C.
func Pow32(x, y float32) float32 {
	r64 := math.Pow(float64(x), float64(y))
	if x <= 0 || math.IsInf(float64(x), 0) || x != x || y != y || math.IsInf(float64(y), 0) ||
		math.IsInf(r64, 0) || r64 == 0 || x == 1 || y == 0 {
		return float32(r64)
	}
	if r, ok := round32(r64); ok {
		return r
	}
	initConsts()
	f, _ := expBig(mul(lnBig(newFloat(float64(x))), newFloat(float64(y)))).Float32()
	return f
}

// Atan2_32 is atan2f for an f32, correctly rounded. usvg calls it to find the
// angle of a marker.
func Atan2_32(y, x float32) float32 {
	y64, x64 := float64(y), float64(x)
	if y != y || x != x || y == 0 || x == 0 || math.IsInf(y64, 0) || math.IsInf(x64, 0) {
		// math.Atan2 follows C99 Annex F for these values. Each result is
		// zero, NaN or a multiple of pi/4, which no f64 result puts on the
		// middle of two f32 values.
		return float32(math.Atan2(y64, x64))
	}
	if r, ok := round32(math.Atan2(y64, x64)); ok {
		return r
	}
	f, _ := atan2Big(y64, x64).Float32()
	return f
}

// round32 rounds r, a result for an f64 with an error of a few units in its
// last place, to an f32. It reports false if the rounding cannot be decided
// from r, because r is too close to the middle of two f32 values.
func round32(r float64) (float32, bool) {
	if r == 0 || math.IsInf(r, 0) || r != r {
		return float32(r), true
	}
	// eps is 8 units in the last place of r.
	eps := math.Abs(math.Nextafter(r, math.Inf(1))-r) * 8
	lo, hi := float32(r-eps), float32(r+eps)
	if lo != hi {
		return 0, false
	}
	return lo, true
}

// newFloat returns x as a *big.Float of the precision of the package.
func newFloat(x float64) *big.Float {
	return new(big.Float).SetPrec(prec).SetFloat64(x)
}

func add(a, b *big.Float) *big.Float { return new(big.Float).SetPrec(prec).Add(a, b) }
func sub(a, b *big.Float) *big.Float { return new(big.Float).SetPrec(prec).Sub(a, b) }
func mul(a, b *big.Float) *big.Float { return new(big.Float).SetPrec(prec).Mul(a, b) }
func quo(a, b *big.Float) *big.Float { return new(big.Float).SetPrec(prec).Quo(a, b) }

// scale returns a times 2 to the power e.
func scale(a *big.Float, e int) *big.Float {
	return new(big.Float).SetPrec(prec).SetMantExp(a, e)
}

// small reports whether |t| is below the precision of the package.
func small(t *big.Float) bool {
	return t.Sign() == 0 || new(big.Float).Abs(t).Cmp(consts.eps) < 0
}

// atanSmall is the Taylor series of atan, for |x| below 0.5.
func atanSmall(x *big.Float) *big.Float {
	eps := new(big.Float).SetPrec(prec).SetMantExp(newFloat(1), -prec-8)
	x2 := mul(x, x)
	term := new(big.Float).Copy(x)
	sum := new(big.Float).Copy(x)
	for k := 1; ; k++ {
		term = mul(term, x2)
		term.Neg(term)
		t := quo(term, newFloat(float64(2*k+1)))
		if t.Sign() == 0 || new(big.Float).Abs(t).Cmp(eps) < 0 {
			return sum
		}
		sum = add(sum, t)
	}
}

// atanhSmall is the Taylor series of atanh, for |x| below 0.5.
func atanhSmall(x *big.Float) *big.Float {
	eps := new(big.Float).SetPrec(prec).SetMantExp(newFloat(1), -prec-8)
	x2 := mul(x, x)
	term := new(big.Float).Copy(x)
	sum := new(big.Float).Copy(x)
	for k := 1; ; k++ {
		term = mul(term, x2)
		t := quo(term, newFloat(float64(2*k+1)))
		if t.Sign() == 0 || new(big.Float).Abs(t).Cmp(eps) < 0 {
			return sum
		}
		sum = add(sum, t)
	}
}

// atanBig is atan, with the reduction atan(x) = 2 atan(x/(1+sqrt(1+x*x))).
func atanBig(x *big.Float) *big.Float {
	n := 0
	tenth := newFloat(0.1)
	for new(big.Float).Abs(x).Cmp(tenth) > 0 {
		s := new(big.Float).SetPrec(prec).Sqrt(add(newFloat(1), mul(x, x)))
		x = quo(x, add(newFloat(1), s))
		n++
	}
	return scale(atanSmall(x), n)
}

// acosBig is acos(x) = 2 atan(sqrt((1-x)/(1+x))), for -1 <= x < 1.
func acosBig(x *big.Float) *big.Float {
	initConsts()
	if x.Cmp(newFloat(-1)) == 0 {
		return new(big.Float).Copy(consts.pi)
	}
	q := quo(sub(newFloat(1), x), add(newFloat(1), x))
	return scale(atanBig(new(big.Float).SetPrec(prec).Sqrt(q)), 1)
}

// cosBig is cos, with the argument reduced modulo 2 pi.
func cosBig(x *big.Float) *big.Float {
	initConsts()
	twoPi := scale(consts.pi, 1)
	k, _ := quo(x, twoPi).Int(nil)
	x = sub(x, mul(new(big.Float).SetPrec(prec).SetInt(k), twoPi))
	x2 := mul(x, x)
	term := newFloat(1)
	sum := newFloat(1)
	for k := 1; ; k++ {
		term = mul(term, x2)
		term = quo(term, newFloat(float64((2*k-1)*(2*k))))
		term.Neg(term)
		if small(term) {
			return sum
		}
		sum = add(sum, term)
	}
}

// sinBig is sin(x) = cos(x - pi/2).
func sinBig(x *big.Float) *big.Float {
	initConsts()
	return cosBig(sub(x, scale(consts.pi, -1)))
}

// lnBig is ln(x) = 2 atanh((m-1)/(m+1)) + e ln 2, for x = m * 2^e and a
// positive x.
func lnBig(x *big.Float) *big.Float {
	m := new(big.Float)
	e := x.MantExp(m)
	m.SetPrec(prec)
	t := quo(sub(m, newFloat(1)), add(m, newFloat(1)))
	return add(scale(atanhSmall(t), 1), mul(consts.ln2, newFloat(float64(e))))
}

// expBig is exp(x), with x reduced to k ln 2 + r.
func expBig(x *big.Float) *big.Float {
	k, _ := quo(x, consts.ln2).Int(nil)
	r := sub(x, mul(new(big.Float).SetPrec(prec).SetInt(k), consts.ln2))
	term := newFloat(1)
	sum := newFloat(1)
	for i := 1; ; i++ {
		term = quo(mul(term, r), newFloat(float64(i)))
		if small(term) {
			return scale(sum, int(k.Int64()))
		}
		sum = add(sum, term)
	}
}
