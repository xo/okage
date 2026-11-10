package crmath

// This file holds the fast path of Pow64. It computes x^y as exp(y ln x) in
// the double-double arithmetic of sincos_dd.go, and bounds the error of that
// result. When every value within the bound rounds to the same f64, that f64
// is the correctly rounded result. Otherwise Pow64 computes the result again
// with math/big. The fast path never changes a result.
//
// The fast path covers a finite positive x and a finite y with
// |y ln x| <= 650, so that the result and its low part stay normal f64
// values.

import (
	"math"
	"math/big"
	"sync"
)

// ddMaxExp is the largest |y ln x| of the fast path.
const ddMaxExp = 650

// The number of terms of the series of the fast path. For |t| <= 0.1716, the
// first term of the series of atanh that is left out is below 2^-120. For
// |r| <= 0.35, the first term of the series of exp that is left out is below
// 2^-120.
const (
	ddLnTerms  = 24
	ddExpTerms = 27
)

// The bounds of the error of the fast path. Analysis gives about 2^-100 for
// the relative error of ln x and of y ln x, about 2^-96 for the absolute
// error of the reduction of the argument of exp, and about 2^-100 for the
// relative error of the series of exp. Each bound is wider, so that a result
// is never accepted when it is wrong.
const (
	ddPowZRelErr = 0x1p-94
	ddPowZAbsErr = 0x1p-92
	ddPowRelErr  = 0x1p-92
)

// ddp holds the constants of the fast path of Pow64. They are made on first
// use.
var ddp struct {
	once sync.Once
	// ln2 is ln 2 as hi and lo.
	ln2 [2]float64
	// atanhC[n] is 1 / (2n+1), and expC[n] is 1 / n!, each as hi and lo.
	atanhC [ddLnTerms][2]float64
	expC   [ddExpTerms][2]float64
}

// ddSplit returns v as the hi and lo parts of a double-double.
func ddSplit(v *big.Float) [2]float64 {
	hi, _ := v.Float64()
	lo, _ := subP(v, fP(hi, wprec), wprec).Float64()
	return [2]float64{hi, lo}
}

// initDDPow makes the constants of the fast path of Pow64.
func initDDPow() {
	ddp.once.Do(func() {
		initWide()
		p := uint(wprec)
		ddp.ln2 = ddSplit(wide.ln2)
		for n := range ddLnTerms {
			ddp.atanhC[n] = ddSplit(quoP(fP(1, p), fP(float64(2*n+1), p), p))
		}
		fact := fP(1, p)
		for n := range ddExpTerms {
			if n > 0 {
				fact = mulP(fact, fP(float64(n), p), p)
			}
			ddp.expC[n] = ddSplit(quoP(fP(1, p), fact, p))
		}
	})
}

// ddHorner returns the sum of c[n] v^n for n from 0, in Horner's form, as a
// double-double.
func ddHorner(c [][2]float64, vh, vl float64) (float64, float64) {
	ph, pl := c[len(c)-1][0], c[len(c)-1][1]
	for n := len(c) - 2; n >= 0; n-- {
		ph, pl = ddMul(ph, pl, vh, vl)
		ph, pl = ddAdd(ph, pl, c[n][0], c[n][1])
	}
	return ph, pl
}

// ddDiv returns the double-double quotient of (ah, al) by (bh, bl).
func ddDiv(ah, al, bh, bl float64) (float64, float64) {
	q1 := ah / bh
	ph, pl := ddMul(q1, 0, bh, bl)
	rh, rl := ddAdd(ah, al, -ph, -pl)
	q2 := (rh + rl) / bh
	return quickTwoSum(q1, q2)
}

// powDD returns x^y, correctly rounded, and true, when the fast path decides
// the rounding, for a finite positive x and a finite y.
func powDD(x, y float64) (float64, bool) {
	initDDPow()

	// ln x = e ln 2 + 2 atanh(t), with x = m 2^e, m in [sqrt(1/2), sqrt(2))
	// and t = (m-1)/(m+1). m-1 is exact by the lemma of Sterbenz.
	m, e := math.Frexp(x)
	if m < math.Sqrt2/2 {
		m *= 2
		e--
	}
	dh, dl := twoSum(m, 1)
	th, tl := ddDiv(m-1, 0, dh, dl)
	t2h, t2l := ddMul(th, tl, th, tl)
	sh, sl := ddHorner(ddp.atanhC[:], t2h, t2l)
	sh, sl = ddMul(sh, sl, 2*th, 2*tl)
	fe := float64(e)
	eh, el := twoProd(fe, ddp.ln2[0])
	el += float64(fe * ddp.ln2[1])
	lh, ll := ddAdd(eh, el, sh, sl)

	// z = y ln x.
	zh, zl := ddMul(y, 0, lh, ll)
	if !(math.Abs(zh) <= ddMaxExp) {
		return 0, false
	}
	zErr := math.Abs(zh)*ddPowZRelErr + ddPowZAbsErr

	// exp(z) = 2^k exp(r), with r = z - k ln 2 and |r| <= ln 2 / 2.
	k := math.Round(zh / ddp.ln2[0])
	kh, kl := twoProd(k, ddp.ln2[0])
	kl += float64(k * ddp.ln2[1])
	rh, rl := ddAdd(zh, zl, -kh, -kl)
	vh, vl := ddHorner(ddp.expC[:], rh, rl)
	vh = math.Ldexp(vh, int(k))
	vl = math.Ldexp(vl, int(k))

	// An error d of z gives a relative error of about d in exp(z).
	return ddRound(vh, vl, math.Abs(vh)*(zErr*1.01+ddPowRelErr))
}
