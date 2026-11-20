package crmath

// This file holds the fast path of Acos64. It computes
// acos(x) = 2 atan(sqrt((1-x)/(1+x))), the formula of acosBig, in the
// double-double arithmetic of sincos_dd.go, with the atan of atan2_dd.go. It
// bounds the error of that result. When every value within the bound rounds
// to the same f64, that f64 is the correctly rounded result. Otherwise Acos64
// computes the result again with math/big. The fast path never changes a
// result.
//
// The fast path covers -1 < x < 1, the subnormal values too. 1-x and 1+x are
// exact as double-doubles, so the relative error of the quotient does not
// grow when x comes near 1 or -1. The argument t of atan is between about
// 2^-27 and 2^27, in the range of atanDD.

import "math"

// ddAcosRelErr bounds the relative error of acos(x) that comes from the error
// of t. Analysis gives about 2^-102 for the relative error of t, and an error
// of t gives at most the same relative error of atan(t). The bound is wider,
// so that a result is never accepted when it is wrong. The fast path adds it
// to the bounds of atanDD.
const ddAcosRelErr = 0x1p-94

// ddSqrt returns the double-double square root of (ah, al), for a positive
// ah. It corrects s = sqrt(ah) with one step of Newton's method:
// sqrt(a) = s + (a - s^2) / (2s), where twoProd gives s^2 exactly.
func ddSqrt(ah, al float64) (float64, float64) {
	s := math.Sqrt(ah)
	ph, pl := twoProd(s, s)
	rh, rl := ddAdd(ah, al, -ph, -pl)
	return quickTwoSum(s, (rh+rl)/(2*s))
}

// acosDD returns acos(x), correctly rounded, and true, when the fast path
// decides the rounding, for -1 < x < 1.
func acosDD(x float64) (float64, bool) {
	initDDAtan()
	nh, nl := twoSum(1, -x)
	dh, dl := twoSum(1, x)
	qh, ql := ddDiv(nh, nl, dh, dl)
	th, tl := ddSqrt(qh, ql)
	ah, al, abs := atanDD(th, tl)
	// 2 atan(t) doubles each part and each bound exactly.
	ah, al, abs = 2*ah, 2*al, 2*abs
	return ddRound(ah, al, float64(math.Abs(ah)*(ddAtanRelErr+ddAcosRelErr))+abs)
}
