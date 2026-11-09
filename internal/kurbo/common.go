// Copyright 2018 the Kurbo Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package kurbo

// This file ports the subset of kurbo kurbo/src/common.rs that svgtypes and
// usvg need: the float functions of the trait FloatFuncs, the ITP solver and
// the tables of Legendre-Gauss quadrature that the arc length of a cubic
// Bezier uses.
//
// usvg turns on the feature std of kurbo, so the methods of FloatFuncs are
// the methods of f64 in the Rust standard library. Go cannot add a method to
// float64, so each one is a function. The functions that Rust takes from the
// C library are the correctly rounded functions of internal/crmath (D26).

import (
	"math"

	"github.com/xo/okage/internal/crmath"
)

// Signum is FloatFuncs::signum in kurbo kurbo/src/common.rs, which is
// f64::signum. It is 1 for a positive x and for +0, -1 for a negative x and
// for -0, and NaN for NaN.
func Signum(x float64) float64 {
	if x != x {
		return math.NaN()
	}
	return math.Copysign(1, x)
}

// Abs is FloatFuncs::abs in kurbo kurbo/src/common.rs.
func Abs(x float64) float64 {
	return math.Abs(x)
}

// Atan2 is FloatFuncs::atan2 in kurbo kurbo/src/common.rs. It is
// atan2(y, x), with y the receiver of the Rust method, correctly rounded.
func Atan2(y, x float64) float64 {
	return crmath.Atan2_64(y, x)
}

// Ceil is FloatFuncs::ceil in kurbo kurbo/src/common.rs.
func Ceil(x float64) float64 {
	return math.Ceil(x)
}

// Cos is FloatFuncs::cos in kurbo kurbo/src/common.rs, correctly rounded.
func Cos(x float64) float64 {
	return crmath.Cos64(x)
}

// Copysign is FloatFuncs::copysign in kurbo kurbo/src/common.rs.
func Copysign(x, sign float64) float64 {
	return math.Copysign(x, sign)
}

// Floor is FloatFuncs::floor in kurbo kurbo/src/common.rs.
func Floor(x float64) float64 {
	return math.Floor(x)
}

// Ln is FloatFuncs::ln in kurbo kurbo/src/common.rs, correctly rounded.
func Ln(x float64) float64 {
	return crmath.Ln64(x)
}

// Log2 is FloatFuncs::log2 in kurbo kurbo/src/common.rs, correctly rounded.
func Log2(x float64) float64 {
	return crmath.Log2_64(x)
}

// Powi is FloatFuncs::powi in kurbo kurbo/src/common.rs, which is f64::powi.
//
// Rust computes powi with repeated multiplication, not with pow. For a
// constant n, LLVM expands it into the multiplications of the binary method,
// and for any other n it calls __powidf2 of compiler-rt, which does the same
// multiplications in the same order. This is __powidf2: x*x for 2, and
// x*(x*x) for 3.
func Powi(x float64, n int32) float64 {
	recip := n < 0
	r := 1.0
	for {
		if n&1 != 0 {
			r *= x
		}
		n /= 2
		if n == 0 {
			break
		}
		x *= x
	}
	if recip {
		return 1 / r
	}
	return r
}

// Powf is FloatFuncs::powf in kurbo kurbo/src/common.rs, correctly rounded.
func Powf(x, n float64) float64 {
	return crmath.Pow64(x, n)
}

// Round is FloatFuncs::round in kurbo kurbo/src/common.rs. It rounds half
// away from zero, as math.Round does.
func Round(x float64) float64 {
	return math.Round(x)
}

// Sin is FloatFuncs::sin in kurbo kurbo/src/common.rs, correctly rounded.
func Sin(x float64) float64 {
	return crmath.Sin64(x)
}

// SinCos is FloatFuncs::sin_cos in kurbo kurbo/src/common.rs. The Rust
// standard library computes it as (sin(x), cos(x)).
func SinCos(x float64) (float64, float64) {
	return Sin(x), Cos(x)
}

// Sqrt is FloatFuncs::sqrt in kurbo kurbo/src/common.rs.
func Sqrt(x float64) float64 {
	return math.Sqrt(x)
}

// Tan is FloatFuncs::tan in kurbo kurbo/src/common.rs, correctly rounded.
func Tan(x float64) float64 {
	return crmath.Tan64(x)
}

// Trunc is FloatFuncs::trunc in kurbo kurbo/src/common.rs.
func Trunc(x float64) float64 {
	return math.Trunc(x)
}

// SolveItp is solve_itp in kurbo kurbo/src/common.rs.
//
// Solve an arbitrary function for a zero-crossing.
//
// This uses the [ITP method], as described in the paper
// [An Enhancement of the Bisection Method Average Performance Preserving Minmax Optimality].
//
// The values of `ya` and `yb` are given as arguments rather than
// computed from `f`, as the values may already be known, or they may
// be less expensive to compute as special cases.
//
// It is assumed that `ya < 0.0` and `yb > 0.0`, otherwise unexpected
// results may occur.
//
// The value of `epsilon` must be larger than 2^-63 times `b - a`,
// otherwise integer overflow may occur. The `a` and `b` parameters
// represent the lower and upper bounds of the bracket searched for a
// solution.
//
// The ITP method has tuning parameters. This implementation hardwires
// k2 to 2, both because it avoids an expensive floating point
// exponentiation, and because this value has been tested to work well
// with curve fitting problems.
//
// The `n0` parameter controls the relative impact of the bisection and
// secant components. When it is 0, the number of iterations is
// guaranteed to be no more than the number required by bisection (thus,
// this method is strictly superior to bisection). However, when the
// function is smooth, a value of 1 gives the secant method more of a
// chance to engage, so the average number of iterations is likely
// lower, though there can be one more iteration than bisection in the
// worst case.
//
// The `k1` parameter is harder to characterize, and interested users
// are referred to the paper, as well as encouraged to do empirical
// testing. To match the paper, a value of `0.2 / (b - a)` is
// suggested, and this is confirmed to give good results.
//
// When the function is monotonic, the returned result is guaranteed to
// be within `epsilon` of the zero crossing. For more detailed analysis,
// again see the paper.
//
// [ITP method]: https://en.wikipedia.org/wiki/ITP_Method
// [An Enhancement of the Bisection Method Average Performance Preserving Minmax Optimality]: https://dl.acm.org/doi/10.1145/3423597
func SolveItp(f func(float64) float64, a, b, epsilon float64, n0 uint, k1, ya, yb float64) float64 {
	a, b, _ = solveItpFallible(func(x float64) (float64, error) {
		return f(x), nil
	}, a, b, epsilon, n0, k1, ya, yb)

	// Return the midpoint of the bracket returned by `solve_itp_fallible`. If that found an exact
	// zero, it returns `(a, a)`, and the following will result in exactly `a` (except for harmless
	// negative zero subtlety or when `a + b` overflows ~1e308).
	return 0.5 * (a + b)
}

// solveItpFallible is solve_itp_fallible in kurbo kurbo/src/common.rs.
//
// A variant ITP solver that allows fallible functions.
//
// Another difference: it returns the bracket that contains the root,
// which may be important if the function has a discontinuity.
func solveItpFallible(f func(float64) (float64, error), a, b, epsilon float64, n0 uint, k1, ya, yb float64) (float64, float64, error) {
	n12 := f64ToUsize(maxF64(Ceil(Log2((b-a)/epsilon))-1.0, 0.0))
	nmax := n0 + n12
	// Rust shifts a u64 by nmax. A release build masks the shift to the
	// width of the type.
	scaledEpsilon := epsilon * float64(uint64(1)<<(nmax&63))
	for b-a > 2.0*epsilon {
		x12 := 0.5 * (a + b)
		r := scaledEpsilon - float64(0.5*(b-a))
		xf := (float64(yb*a) - float64(ya*b)) / (yb - ya)
		sigma := x12 - xf
		// This has k2 = 2 hardwired for efficiency.
		delta := k1 * Powi(b-a, 2)
		var xt float64
		if delta <= Abs(x12-xf) {
			xt = xf + Copysign(delta, sigma)
		} else {
			xt = x12
		}
		var xitp float64
		if Abs(xt-x12) <= r {
			xitp = xt
		} else {
			xitp = x12 - Copysign(r, sigma)
		}
		yitp, err := f(xitp)
		if err != nil {
			return 0, 0, err
		}
		switch {
		case yitp > 0.0:
			b = xitp
			yb = yitp
		case yitp < 0.0:
			a = xitp
			ya = yitp
		default:
			return xitp, xitp, nil
		}
		scaledEpsilon *= 0.5
	}
	return a, b, nil
}

// Tables of Legendre-Gauss quadrature coefficients, adapted from:
// <https://pomax.github.io/bezierinfo/legendre-gauss.html>
//
// Each pair is a weight and an abscissa, (wi, xi).

// GaussLegendreCoeffs8 is GAUSS_LEGENDRE_COEFFS_8 in kurbo kurbo/src/common.rs.
var GaussLegendreCoeffs8 = [][2]float64{
	{0.3626837833783620, -0.1834346424956498},
	{0.3626837833783620, 0.1834346424956498},
	{0.3137066458778873, -0.5255324099163290},
	{0.3137066458778873, 0.5255324099163290},
	{0.2223810344533745, -0.7966664774136267},
	{0.2223810344533745, 0.7966664774136267},
	{0.1012285362903763, -0.9602898564975363},
	{0.1012285362903763, 0.9602898564975363},
}

// GaussLegendreCoeffs8Half is GAUSS_LEGENDRE_COEFFS_8_HALF in kurbo
// kurbo/src/common.rs.
var GaussLegendreCoeffs8Half = [][2]float64{
	{0.3626837833783620, 0.1834346424956498},
	{0.3137066458778873, 0.5255324099163290},
	{0.2223810344533745, 0.7966664774136267},
	{0.1012285362903763, 0.9602898564975363},
}

// GaussLegendreCoeffs16Half is GAUSS_LEGENDRE_COEFFS_16_HALF in kurbo
// kurbo/src/common.rs.
//
// Just the positive x_i values.
var GaussLegendreCoeffs16Half = [][2]float64{
	{0.1894506104550685, 0.0950125098376374},
	{0.1826034150449236, 0.2816035507792589},
	{0.1691565193950025, 0.4580167776572274},
	{0.1495959888165767, 0.6178762444026438},
	{0.1246289712555339, 0.7554044083550030},
	{0.0951585116824928, 0.8656312023878318},
	{0.0622535239386479, 0.9445750230732326},
	{0.0271524594117541, 0.9894009349916499},
}

// GaussLegendreCoeffs24Half is GAUSS_LEGENDRE_COEFFS_24_HALF in kurbo
// kurbo/src/common.rs.
var GaussLegendreCoeffs24Half = [][2]float64{
	{0.1279381953467522, 0.0640568928626056},
	{0.1258374563468283, 0.1911188674736163},
	{0.1216704729278034, 0.3150426796961634},
	{0.1155056680537256, 0.4337935076260451},
	{0.1074442701159656, 0.5454214713888396},
	{0.0976186521041139, 0.6480936519369755},
	{0.0861901615319533, 0.7401241915785544},
	{0.0733464814110803, 0.8200019859739029},
	{0.0592985849154368, 0.8864155270044011},
	{0.0442774388174198, 0.9382745520027328},
	{0.0285313886289337, 0.9747285559713095},
	{0.0123412297999872, 0.9951872199970213},
}

// The functions below have no Rust source. They are methods of f64 in the
// Rust core library, with rules that the Go builtins do not have.

// maxF64 is f64::max. If one operand is NaN, it returns the other one.
//
// It follows the code that rustc 1.97.1 makes for x86-64, as maxF32 in path
// does: the result is x if x > y, and y if not. If y is NaN, the result is x.
// So for two zeros of different signs, the result is y.
func maxF64(x, y float64) float64 {
	if y != y {
		return x
	}
	if x > y {
		return x
	}
	return y
}

// minF64 is f64::min. If one operand is NaN, it returns the other one.
//
// It follows the code that rustc 1.97.1 makes for x86-64, as minF32 in path
// does: the result is x if x < y, and y if not. If y is NaN, the result is x.
// So for two zeros of different signs, the result is y.
func minF64(x, y float64) float64 {
	if y != y {
		return x
	}
	if x < y {
		return x
	}
	return y
}

// f64ToUsize is the conversion `x as usize` of Rust, which saturates: NaN
// and a negative x give 0, and a value above the largest usize gives the
// largest usize.
func f64ToUsize(x float64) uint {
	switch {
	case x != x || x <= 0:
		return 0
	case x >= float64(^uint(0)):
		return ^uint(0)
	}
	return uint(x)
}

// negZero is -0.0. The sum of an iterator of f64 in Rust starts at it.
var negZero = math.Copysign(0, -1)

// remF64 is the operator % of f64, which is fmod in C. Its result is exact,
// and math.Mod computes it exactly.
func remF64(x, y float64) float64 {
	return math.Mod(x, y)
}
