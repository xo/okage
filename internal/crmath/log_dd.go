package crmath

// This file holds the fast paths of Ln64 and Log2_64. They compute ln x in
// the double-double arithmetic of sincos_dd.go, as the fast path of Pow64
// does, and bound the error of that result. When every value within the
// bound rounds to the same f64, that f64 is the correctly rounded result.
// Otherwise Ln64 or Log2_64 computes the result again with math/big. The fast
// path never changes a result.
//
// The fast paths cover every finite positive x, the subnormal values too.
// x = m 2^e, with m in [sqrt(1/2), sqrt(2)), gives ln x = e ln 2 + 2 atanh(t)
// with t = (m-1)/(m+1). The relative error of 2 atanh(t) does not grow when x
// comes near 1, because m-1 is exact. When e is not 0, |e ln 2| is at least
// twice |2 atanh(t)|, so the sum cancels at most one bit.

import "math"

// The bounds of the relative error of the fast paths. Analysis gives about
// 2^-100 for ln x, and about 2^-99 for log2 x, which also multiplies by
// 1 / ln 2 and adds e. Each bound is wider, so that a result is never
// accepted when it is wrong.
const (
	ddLnRelErr   = 0x1p-92
	ddLog2RelErr = 0x1p-92
)

// ln64DD returns ln x, correctly rounded, and true, when the fast path
// decides the rounding, for a finite positive x other than 1.
func ln64DD(x float64) (float64, bool) {
	initDDPow()
	lh, ll := lnDD(x)
	return ddRound(lh, ll, float64(math.Abs(lh)*ddLnRelErr))
}

// log2DD returns log2 x, correctly rounded, and true, when the fast path
// decides the rounding, for a finite positive x.
//
// log2 x = e + 2 atanh(t) / ln 2. e is exact, and the second term is at most
// 1/2 in magnitude, so the sum cancels at most one bit when e is not 0.
func log2DD(x float64) (float64, bool) {
	initDDPow()
	sh, sl, e := lnParts(x)
	vh, vl := ddMul(sh, sl, ddp.invLn2[0], ddp.invLn2[1])
	vh, vl = ddAdd(float64(e), 0, vh, vl)
	return ddRound(vh, vl, float64(math.Abs(vh)*ddLog2RelErr))
}
