package crmath

// This file holds the correctly rounded functions of an f64 that the port of
// kurbo and svgtypes needs: sin, cos, tan, atan2, pow, ln and log2. Rust
// calls the C library for each one (D26).
//
// Each function handles the special values (zeros, infinities and NaN) as C99
// Annex F says, and computes every other value with math/big at the precision
// wprec. A correctly rounded f64 needs about 120 bits in the worst known
// cases, so wprec leaves a wide margin. The argument of sin, cos and tan is
// reduced with a value of pi that has enough bits for the exponent of the
// argument, so that a large argument also gives the right result.

import (
	"math"
	"math/big"
	"sync"
)

// wprec is the working precision in bits of the f64 functions of this file.
const wprec = 192

// piPrec is the precision in bits of the value of pi that reduces the
// argument of sin, cos and tan. The largest exponent of an f64 is 1023, so
// the reduction of any finite argument keeps more than wprec bits.
const piPrec = 1400

// wide holds the constants of the f64 functions of this file. They are made
// on first use.
var wide struct {
	once sync.Once
	pi   *big.Float // pi, at piPrec bits
	ln2  *big.Float // ln 2, at wprec bits
}

// initWide makes the constants of the f64 functions of this file.
func initWide() {
	wide.once.Do(func() {
		// pi = 16 atan(1/5) - 4 atan(1/239)
		p := uint(piPrec + 64)
		a := atanSeries(quoP(fP(1, p), fP(5, p), p), p)
		b := atanSeries(quoP(fP(1, p), fP(239, p), p), p)
		pi := subP(mulP(a, fP(16, p), p), mulP(b, fP(4, p), p), p)
		wide.pi = new(big.Float).SetPrec(piPrec).Set(pi)
		// ln 2 = 2 atanh(1/3)
		q := uint(wprec + 64)
		l := atanhSeries(quoP(fP(1, q), fP(3, q), q), q)
		wide.ln2 = new(big.Float).SetPrec(wprec).SetMantExp(l, 1)
	})
}

// Sin64 is sin for an f64, correctly rounded.
func Sin64(x float64) float64 {
	if x != x || math.IsInf(x, 0) {
		return math.NaN()
	}
	if x == 0 {
		return x
	}
	s, _ := sinCosBig(x)
	f, _ := s.Float64()
	return f
}

// cos64 is cos for an f64, correctly rounded. Cos64 calls it.
func cos64(x float64) float64 {
	if x != x || math.IsInf(x, 0) {
		return math.NaN()
	}
	_, c := sinCosBig(x)
	f, _ := c.Float64()
	return f
}

// Tan64 is tan for an f64, correctly rounded.
func Tan64(x float64) float64 {
	if x != x || math.IsInf(x, 0) {
		return math.NaN()
	}
	if x == 0 {
		return x
	}
	s, c := sinCosBig(x)
	f, _ := quoP(s, c, wprec).Float64()
	return f
}

// Atan2_64 is atan2(y, x) for an f64, correctly rounded.
func Atan2_64(y, x float64) float64 {
	if y != y || x != x || y == 0 || x == 0 || math.IsInf(y, 0) || math.IsInf(x, 0) {
		// math.Atan2 follows C99 Annex F for these values. Each result is
		// zero, NaN or a multiple of pi/4 that Go rounds correctly as a
		// constant.
		return math.Atan2(y, x)
	}
	f, _ := atan2Big(y, x).Float64()
	return f
}

// atan2Big is atan2(y, x) with math/big, for a finite non-zero x and y.
func atan2Big(y, x float64) *big.Float {
	initWide()
	t := quoP(fP(y, wprec), fP(x, wprec), wprec)
	r := atanBigP(t)
	if x < 0 {
		pi := new(big.Float).SetPrec(wprec).Set(wide.pi)
		if y > 0 {
			r = addP(r, pi, wprec)
		} else {
			r = subP(r, pi, wprec)
		}
	}
	return r
}

// Pow64 is pow for an f64, correctly rounded.
func Pow64(x, y float64) float64 {
	if x != x || y != y || x == 0 || y == 0 || math.IsInf(x, 0) || math.IsInf(y, 0) || x == 1 || y == 1 {
		// math.Pow follows C99 Annex F for these values, and each result is
		// exact.
		return math.Pow(x, y)
	}
	neg := false
	if x < 0 {
		if y != math.Trunc(y) {
			return math.NaN()
		}
		// y is an integer. The result is negative if y is odd.
		if math.Abs(y) < 1<<53 && int64(y)&1 == 1 {
			neg = true
		}
		x = -x
	}
	var r *big.Float
	if y == math.Trunc(y) && math.Abs(y) <= 64 {
		r = powIntBig(x, int(y))
	} else {
		initWide()
		e := mulP(lnBigP(x), fP(y, wprec), wprec)
		// exp(e) overflows or underflows an f64 when |e| is larger than
		// about 745. Stop early, so that the exponent stays small.
		if e.Cmp(fP(1000, wprec)) > 0 {
			r = fP(math.Inf(1), wprec)
		} else if e.Cmp(fP(-1000, wprec)) < 0 {
			r = fP(0, wprec)
		} else {
			r = expBigP(e)
		}
	}
	if neg {
		r.Neg(r)
	}
	f, _ := r.Float64()
	return f
}

// Ln64 is ln, the natural logarithm, for an f64, correctly rounded.
func Ln64(x float64) float64 {
	switch {
	case x != x || x < 0:
		return math.NaN()
	case x == 0:
		return math.Inf(-1)
	case math.IsInf(x, 1):
		return x
	case x == 1:
		return 0
	}
	initWide()
	f, _ := lnBigP(x).Float64()
	return f
}

// Log2_64 is log2 for an f64, correctly rounded.
func Log2_64(x float64) float64 {
	switch {
	case x != x || x < 0:
		return math.NaN()
	case x == 0:
		return math.Inf(-1)
	case math.IsInf(x, 1):
		return x
	}
	// The result is exact for a power of two.
	frac, exp := math.Frexp(x)
	if frac == 0.5 {
		return float64(exp - 1)
	}
	initWide()
	f, _ := quoP(lnBigP(x), wide.ln2, wprec).Float64()
	return f
}

// fP returns x as a *big.Float of the precision p.
func fP(x float64, p uint) *big.Float {
	return new(big.Float).SetPrec(p).SetFloat64(x)
}

func addP(a, b *big.Float, p uint) *big.Float { return new(big.Float).SetPrec(p).Add(a, b) }
func subP(a, b *big.Float, p uint) *big.Float { return new(big.Float).SetPrec(p).Sub(a, b) }
func mulP(a, b *big.Float, p uint) *big.Float { return new(big.Float).SetPrec(p).Mul(a, b) }
func quoP(a, b *big.Float, p uint) *big.Float { return new(big.Float).SetPrec(p).Quo(a, b) }

// negligible reports whether the term t no longer changes the sum s at the
// precision p. The test is relative to s.
func negligible(t, s *big.Float, p uint) bool {
	if t.Sign() == 0 {
		return true
	}
	if s.Sign() == 0 {
		return false
	}
	return t.MantExp(nil) < s.MantExp(nil)-int(p)-8
}

// atanSeries is the Taylor series of atan, for |x| below 0.5, at the precision
// p.
func atanSeries(x *big.Float, p uint) *big.Float {
	x2 := mulP(x, x, p)
	term := new(big.Float).SetPrec(p).Set(x)
	sum := new(big.Float).SetPrec(p).Set(x)
	for k := 1; ; k++ {
		term = mulP(term, x2, p)
		term.Neg(term)
		t := quoP(term, fP(float64(2*k+1), p), p)
		if negligible(t, sum, p) {
			return sum
		}
		sum = addP(sum, t, p)
	}
}

// atanhSeries is the Taylor series of atanh, for |x| below 0.5, at the
// precision p.
func atanhSeries(x *big.Float, p uint) *big.Float {
	x2 := mulP(x, x, p)
	term := new(big.Float).SetPrec(p).Set(x)
	sum := new(big.Float).SetPrec(p).Set(x)
	for k := 1; ; k++ {
		term = mulP(term, x2, p)
		t := quoP(term, fP(float64(2*k+1), p), p)
		if negligible(t, sum, p) {
			return sum
		}
		sum = addP(sum, t, p)
	}
}

// atanBigP is atan at the precision wprec. For |x| > 1 it uses
// atan(x) = sign(x) pi/2 - atan(1/x). Then it reduces x with
// atan(x) = 2 atan(x/(1+sqrt(1+x*x))) until |x| is at most 1/8.
func atanBigP(x *big.Float) *big.Float {
	one := fP(1, wprec)
	if new(big.Float).Abs(x).Cmp(one) > 0 {
		halfPi := new(big.Float).SetPrec(wprec).SetMantExp(wide.pi, -1)
		r := atanBigP(quoP(one, x, wprec))
		if x.Sign() > 0 {
			return subP(halfPi, r, wprec)
		}
		halfPi.Neg(halfPi)
		return subP(halfPi, r, wprec)
	}
	n := 0
	eighth := fP(0.125, wprec)
	for new(big.Float).Abs(x).Cmp(eighth) > 0 {
		s := new(big.Float).SetPrec(wprec).Sqrt(addP(one, mulP(x, x, wprec), wprec))
		x = quoP(x, addP(one, s, wprec), wprec)
		n++
	}
	r := atanSeries(x, wprec)
	return new(big.Float).SetPrec(wprec).SetMantExp(r, n)
}

// sinCosBig returns sin(x) and cos(x) at the precision wprec, for a finite x.
//
// It reduces x to r = x - k pi/2, with |r| at most about pi/4, and computes
// the Taylor series of sin and cos of r. The quadrant k mod 4 then chooses
// the signs and the order of the two results.
func sinCosBig(x float64) (*big.Float, *big.Float) {
	initWide()
	_, e := math.Frexp(x)
	// The reduction needs the bits of pi up to the exponent of x, and wprec
	// bits after the point, with a margin for the cancellation of x - k pi/2.
	p := uint(wprec + 128)
	if e > 0 {
		p += uint(e)
	}
	halfPi := new(big.Float).SetPrec(p).SetMantExp(wide.pi, -1)
	bx := fP(x, p)
	q := quoP(bx, halfPi, p)
	// k is q rounded to the nearest integer.
	if q.Sign() >= 0 {
		q = addP(q, fP(0.5, p), p)
	} else {
		q = subP(q, fP(0.5, p), p)
	}
	k, _ := q.Int(nil)
	r := subP(bx, mulP(new(big.Float).SetPrec(p).SetInt(k), halfPi, p), p)
	r = new(big.Float).SetPrec(wprec).Set(r)

	s := sinSeries(r)
	c := cosSeries(r)
	switch new(big.Int).And(k, big.NewInt(3)).Int64() {
	case 0:
		return s, c
	case 1:
		return c, s.Neg(s)
	case 2:
		return s.Neg(s), c.Neg(c)
	default:
		return c.Neg(c), s
	}
}

// sinSeries is the Taylor series of sin, for |r| at most about pi/4.
func sinSeries(r *big.Float) *big.Float {
	if r.Sign() == 0 {
		return fP(0, wprec)
	}
	r2 := mulP(r, r, wprec)
	term := new(big.Float).SetPrec(wprec).Set(r)
	sum := new(big.Float).SetPrec(wprec).Set(r)
	for k := 1; ; k++ {
		term = mulP(term, r2, wprec)
		term = quoP(term, fP(float64((2*k)*(2*k+1)), wprec), wprec)
		term.Neg(term)
		if negligible(term, sum, wprec) {
			return sum
		}
		sum = addP(sum, term, wprec)
	}
}

// cosSeries is the Taylor series of cos, for |r| at most about pi/4.
func cosSeries(r *big.Float) *big.Float {
	r2 := mulP(r, r, wprec)
	term := fP(1, wprec)
	sum := fP(1, wprec)
	for k := 1; ; k++ {
		term = mulP(term, r2, wprec)
		term = quoP(term, fP(float64((2*k-1)*(2*k)), wprec), wprec)
		term.Neg(term)
		if negligible(term, sum, wprec) {
			return sum
		}
		sum = addP(sum, term, wprec)
	}
}

// lnBigP is ln(x) at the precision wprec, for a finite positive x.
//
// It writes x = m * 2^e with m in [sqrt(1/2), sqrt(2)), so that the sum
// 2 atanh((m-1)/(m+1)) + e ln 2 has no cancellation when x is close to 1.
func lnBigP(x float64) *big.Float {
	frac, e := math.Frexp(x)
	if frac < math.Sqrt2/2 {
		frac *= 2
		e--
	}
	m := fP(frac, wprec)
	one := fP(1, wprec)
	t := quoP(subP(m, one, wprec), addP(m, one, wprec), wprec)
	l := new(big.Float).SetPrec(wprec)
	if t.Sign() != 0 {
		l.SetMantExp(atanhSeries(t, wprec), 1)
	}
	return addP(l, mulP(wide.ln2, fP(float64(e), wprec), wprec), wprec)
}

// expBigP is exp(x) at the precision wprec, for |x| at most 1000. It reduces
// x to k ln 2 + r with |r| at most ln 2 / 2.
func expBigP(x *big.Float) *big.Float {
	q := quoP(x, wide.ln2, wprec)
	if q.Sign() >= 0 {
		q = addP(q, fP(0.5, wprec), wprec)
	} else {
		q = subP(q, fP(0.5, wprec), wprec)
	}
	k, _ := q.Int64()
	r := subP(x, mulP(fP(float64(k), wprec), wide.ln2, wprec), wprec)
	term := fP(1, wprec)
	sum := fP(1, wprec)
	for i := 1; ; i++ {
		term = quoP(mulP(term, r, wprec), fP(float64(i), wprec), wprec)
		if negligible(term, sum, wprec) {
			return new(big.Float).SetPrec(wprec).SetMantExp(sum, int(k))
		}
		sum = addP(sum, term, wprec)
	}
}

// powIntBig is x^n for a finite positive x and an integer n with |n| at most
// 64. For a positive n the product is exact, because the precision holds
// every bit of it, so a result that is the middle of two f64 values rounds
// as it must. For a negative n it divides 1 by the exact product.
func powIntBig(x float64, n int) *big.Float {
	neg := n < 0
	if neg {
		n = -n
	}
	p := uint(53*n + 64)
	r := fP(1, p)
	b := fP(x, p)
	for range n {
		r = mulP(r, b, p)
	}
	if neg {
		return quoP(fP(1, wprec), r, wprec)
	}
	return r
}
