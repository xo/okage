package crmath

// This file holds the fast path of Sin64 and Cos64. It computes sin and cos
// in double-double arithmetic, where a value is the unevaluated sum hi + lo of
// two f64 values, with about 106 bits of precision. It also bounds the error
// of that result. When every value within the bound rounds to the same f64,
// that f64 is the correctly rounded result. Otherwise the caller computes the
// result again with math/big, in f64.go. The fast path never changes a
// result. It only decides when math/big is not needed.
//
// The fast path covers 2^-400 <= |x| < 2^20. The argument is reduced to
// r = x - k pi/2 with four parts of pi/2. The first three parts have 33 bits
// each, so that k times each one is exact for k < 2^21.

import (
	"math"
	"math/big"
	"sync"
)

// The range of the fast path. Outside it, the result comes from math/big.
const (
	ddMinArg = 0x1p-400
	ddMaxArg = 0x1p20
)

// ddTinyArg is the bound below which sin(x) and tan(x) round to x, and
// cos(x) rounds to 1. For |x| < 2^-27, the next term of each series is below
// 2^-55 |x|, or 2^-55 for cos, which is less than half an ulp.
const ddTinyArg = 0x1p-27

// The bounds of the error of the fast path. Analysis gives about 2^-99 for
// the relative error of the series. The reduction adds an error of about
// 2^-104 |r|, for the double-double sums, and below 2^-130, for the part of
// pi/2 after the four parts. Each bound is wider, so that a result is never
// accepted when it is wrong.
const (
	ddRelErr    = 0x1p-92
	ddRedRelErr = 0x1p-94
	ddRedAbsErr = 0x1p-124
)

// ddTerms is the number of terms of the series of sin and cos after the
// first. For |r| <= 1, the first term that is left out is below 2^-117.
const ddTerms = 15

// dd holds the constants of the fast path. They are made on first use.
var dd struct {
	once sync.Once
	// pio2 holds pi/2 = pio2[0] + pio2[1] + pio2[2] + pio2[3]. The first
	// three parts have at most 33 significant bits.
	pio2 [4]float64
	// sinC[n] is (-1)^(n+1) / (2n+3)!, and cosC[n] is (-1)^(n+1) / (2n+2)!,
	// each as hi and lo.
	sinC, cosC [ddTerms][2]float64
}

// initDD makes the constants of the fast path from the value of pi of
// f64.go.
func initDD() {
	dd.once.Do(func() {
		initWide()
		p := uint(400)
		rest := new(big.Float).SetPrec(p).SetMantExp(wide.pi, -1)
		for i := range 3 {
			part := new(big.Float).SetPrec(33).SetMode(big.ToZero).Set(rest)
			dd.pio2[i], _ = part.Float64()
			rest = subP(rest, part, p)
		}
		dd.pio2[3], _ = rest.Float64()

		fact := fP(1, p)
		for n := 1; n <= 2*ddTerms+1; n++ {
			fact = mulP(fact, fP(float64(n), p), p)
			if n < 2 {
				continue
			}
			c := quoP(fP(1, p), fact, p)
			if n%4 == 2 || n%4 == 3 {
				c.Neg(c)
			}
			hi, _ := c.Float64()
			lo, _ := subP(c, fP(hi, p), p).Float64()
			if n%2 == 1 {
				dd.sinC[(n-3)/2] = [2]float64{hi, lo}
			} else {
				dd.cosC[(n-2)/2] = [2]float64{hi, lo}
			}
		}
	})
}

// twoSum returns s and e with s + e = a + b exactly, and s = fl(a + b).
func twoSum(a, b float64) (float64, float64) {
	s := a + b
	bb := s - a
	return s, (a - (s - bb)) + (b - bb)
}

// quickTwoSum is twoSum for |a| >= |b|.
func quickTwoSum(a, b float64) (float64, float64) {
	s := a + b
	return s, b - (s - a)
}

// twoProd returns p and e with p + e = a * b exactly, and p = fl(a * b).
// math.FMA rounds once, so its result is the exact error of p.
func twoProd(a, b float64) (float64, float64) {
	p := a * b
	return p, math.FMA(a, b, -p)
}

// ddAdd returns the double-double sum of (ah, al) and (bh, bl).
func ddAdd(ah, al, bh, bl float64) (float64, float64) {
	s, e := twoSum(ah, bh)
	t, f := twoSum(al, bl)
	e += t
	s, e = quickTwoSum(s, e)
	e += f
	return quickTwoSum(s, e)
}

// ddMul returns the double-double product of (ah, al) and (bh, bl). Each
// product is converted to an f64 before the sum, so that the compiler does
// not fuse it with the addition (D16).
func ddMul(ah, al, bh, bl float64) (float64, float64) {
	p, e := twoProd(ah, bh)
	e += float64(ah*bl) + float64(al*bh)
	return quickTwoSum(p, e)
}

// ddSeries returns the sum of c[n] r2^(n+1) for n from 0, plus 1, in Horner's
// form, as a double-double.
func ddSeries(c *[ddTerms][2]float64, r2h, r2l float64) (float64, float64) {
	ph, pl := c[ddTerms-1][0], c[ddTerms-1][1]
	for n := ddTerms - 2; n >= 0; n-- {
		ph, pl = ddMul(ph, pl, r2h, r2l)
		ph, pl = ddAdd(ph, pl, c[n][0], c[n][1])
	}
	ph, pl = ddMul(ph, pl, r2h, r2l)
	return ddAdd(1, 0, ph, pl)
}

// ddRound returns the f64 that hi + lo rounds to, and true, when every value
// within err of hi + lo rounds to the same f64.
func ddRound(hi, lo, err float64) (float64, bool) {
	a := hi + (lo - err)
	b := hi + (lo + err)
	return a, a == b
}

// ddValue is a double-double value hi + lo, with a bound err of the distance
// to the exact value.
type ddValue struct {
	hi, lo, err float64
}

// round returns the f64 that v rounds to, and true, when every value within
// v.err of v rounds to the same f64.
func (v ddValue) round() (float64, bool) {
	return ddRound(v.hi, v.lo, v.err)
}

// sinCosDD returns sin(x) and cos(x), correctly rounded, for a finite x. okS
// and okC report whether the fast path decided each rounding. When one is
// false, that result must come from math/big.
func sinCosDD(x float64) (s, c float64, okS, okC bool) {
	sv, cv, ok := sinCosDDValue(x)
	if !ok {
		return 0, 0, false, false
	}
	s, okS = sv.round()
	c, okC = cv.round()
	return s, c, okS, okC
}

// tanDD returns tan(x), correctly rounded, and true, when the fast path
// decides the rounding, for a finite x.
func tanDD(x float64) (float64, bool) {
	sv, cv, ok := sinCosDDValue(x)
	if !ok || sv.hi == 0 || cv.hi == 0 {
		return 0, false
	}
	// q = s / c in double-double: q1 = s / c rounded, then the remainder
	// s - q1 c divided by c.
	q1 := sv.hi / cv.hi
	ph, pl := ddMul(q1, 0, cv.hi, cv.lo)
	rh, _ := ddAdd(sv.hi, sv.lo, -ph, -pl)
	q2 := rh / cv.hi
	qh, ql := quickTwoSum(q1, q2)
	// The relative error of q is the sum of the relative errors of s and c,
	// and of the division.
	rel := sv.err/math.Abs(sv.hi) + cv.err/math.Abs(cv.hi) + ddRelErr
	return ddRound(qh, ql, math.Abs(qh)*rel)
}

// sinCosDDValue returns sin(x) and cos(x) as double-double values with their
// error bounds, and true, when x is in the range of the fast path.
func sinCosDDValue(x float64) (ddValue, ddValue, bool) {
	ax := math.Abs(x)
	if ax < ddMinArg || ax >= ddMaxArg {
		return ddValue{}, ddValue{}, false
	}
	initDD()

	// r = x - k pi/2. x - k pio2[0] is exact by the lemma of Sterbenz, and
	// each product of k with one of the first three parts is exact.
	k := math.Round(x * (2 / math.Pi))
	rh, rl := x, 0.0
	abs := 0.0
	if k != 0 {
		rh, rl = twoSum(x-float64(k*dd.pio2[0]), -(k * dd.pio2[1]))
		rh, rl = ddAdd(rh, rl, -(k * dd.pio2[2]), 0)
		rh, rl = ddAdd(rh, rl, -(k * dd.pio2[3]), 0)
		// sin and cos have a slope of at most 1, so an error of r is at most
		// the same error of the result.
		abs = math.Abs(rh)*ddRedRelErr + ddRedAbsErr
	}

	r2h, r2l := ddMul(rh, rl, rh, rl)
	sh, sl := ddSeries(&dd.sinC, r2h, r2l)
	sh, sl = ddMul(sh, sl, rh, rl)
	ch, cl := ddSeries(&dd.cosC, r2h, r2l)

	// The quadrant k mod 4 chooses the signs and the order of the results.
	switch int64(k) & 3 {
	case 1:
		sh, sl, ch, cl = ch, cl, -sh, -sl
	case 2:
		sh, sl, ch, cl = -sh, -sl, -ch, -cl
	case 3:
		sh, sl, ch, cl = -ch, -cl, sh, sl
	}
	sv := ddValue{sh, sl, math.Abs(sh)*ddRelErr + abs}
	cv := ddValue{ch, cl, math.Abs(ch)*ddRelErr + abs}
	return sv, cv, true
}
