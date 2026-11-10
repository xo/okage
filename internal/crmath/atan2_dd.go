package crmath

// This file holds the fast path of Atan2_64. It computes atan2(y, x) in the
// double-double arithmetic of sincos_dd.go, and bounds the error of that
// result. When every value within the bound rounds to the same f64, that f64
// is the correctly rounded result. Otherwise Atan2_64 computes the result
// again with math/big. The fast path never changes a result.
//
// With t = |y| / |x|, the fast path uses atan(t) = pi/2 - atan(1/t) for
// t > 1, then atan(t) = atan(c) + atan((t - c) / (1 + t c)) with c the
// nearest multiple of 1/8, so that the argument of the series is at most
// about 1/16. It covers 2^-500 <= t <= 2^500.

import (
	"math"
	"math/big"
	"sync"
)

// The range of t of the fast path.
const (
	ddAtanMin = 0x1p-500
	ddAtanMax = 0x1p500
)

// ddAtanTerms is the number of terms of the series of atan. For |u| <= 1/16,
// the first term that is left out is below 2^-120.
const ddAtanTerms = 15

// The bounds of the error of the fast path. Analysis gives about 2^-100 for
// the relative error of the series and of t, and about 2^-102 for the
// absolute error that the reductions add. Each bound is wider, so that a
// result is never accepted when it is wrong.
const (
	ddAtanRelErr = 0x1p-92
	ddAtanAbsErr = 0x1p-94
)

// dda holds the constants of the fast path of Atan2_64. They are made on
// first use.
var dda struct {
	once sync.Once
	// pi and halfPi are pi and pi/2 as hi and lo.
	pi, halfPi [2]float64
	// atanK[k] is atan(k/8) as hi and lo.
	atanK [9][2]float64
	// atanC[n] is (-1)^n / (2n+1) as hi and lo.
	atanC [ddAtanTerms][2]float64
}

// initDDAtan makes the constants of the fast path of Atan2_64.
func initDDAtan() {
	dda.once.Do(func() {
		initWide()
		p := uint(wprec)
		pi := new(big.Float).SetPrec(p).Set(wide.pi)
		dda.pi = ddSplit(pi)
		dda.halfPi = ddSplit(new(big.Float).SetPrec(p).SetMantExp(pi, -1))
		for k := 1; k <= 8; k++ {
			dda.atanK[k] = ddSplit(atanBigP(fP(float64(k)/8, p)))
		}
		for n := range ddAtanTerms {
			c := quoP(fP(1, p), fP(float64(2*n+1), p), p)
			if n%2 == 1 {
				c.Neg(c)
			}
			dda.atanC[n] = ddSplit(c)
		}
	})
}

// atan2DD returns atan2(y, x), correctly rounded, and true, when the fast
// path decides the rounding, for a finite non-zero x and y.
func atan2DD(y, x float64) (float64, bool) {
	ax, ay := math.Abs(x), math.Abs(y)
	if ay/ax < ddAtanMin || ay/ax > ddAtanMax {
		return 0, false
	}
	initDDAtan()

	th, tl := ddDiv(ay, 0, ax, 0)
	recip := th > 1
	if recip {
		th, tl = ddDiv(1, 0, th, tl)
	}
	// c = k/8 is exact, and |t - c| <= 1/16, so |u| <= 1/16.
	k := math.Round(8 * th)
	uh, ul := th, tl
	if k != 0 {
		c := k / 8
		nh, nl := ddAdd(th, tl, -c, 0)
		dh, dl := ddMul(th, tl, c, 0)
		dh, dl = ddAdd(1, 0, dh, dl)
		uh, ul = ddDiv(nh, nl, dh, dl)
	}
	u2h, u2l := ddMul(uh, ul, uh, ul)
	ah, al := ddHorner(dda.atanC[:], u2h, u2l)
	ah, al = ddMul(ah, al, uh, ul)

	abs := 0.0
	if k != 0 {
		ah, al = ddAdd(dda.atanK[int(k)][0], dda.atanK[int(k)][1], ah, al)
		abs = ddAtanAbsErr
	}
	if recip {
		ah, al = ddAdd(dda.halfPi[0], dda.halfPi[1], -ah, -al)
		abs = ddAtanAbsErr
	}
	if x < 0 {
		ah, al = ddAdd(dda.pi[0], dda.pi[1], -ah, -al)
		abs = ddAtanAbsErr
	}
	if y < 0 {
		ah, al = -ah, -al
	}
	return ddRound(ah, al, math.Abs(ah)*ddAtanRelErr+abs)
}
