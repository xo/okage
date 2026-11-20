package crmath

import (
	"flag"
	"math"
	"math/big"
	"math/rand/v2"
	"testing"
)

// ddN is the number of random arguments of TestSinCosDD for each range.
var ddN = flag.Int("crmath.ddn", 3000, "random arguments of TestSinCosDD for each range")

// TestSinCosDD checks that each result of sin, cos and tan that the fast
// path accepts is the result of math/big, on random arguments in each range of the fast path,
// next to multiples of pi/2, and on the rows of MPFR. It also checks that the
// fast path decides most roundings, which is the reason it exists.
func TestSinCosDD(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	var args []float64
	for _, r := range [][2]float64{{-400, -20}, {-20, 0}, {0, 3}, {3, 19.9}} {
		for range *ddN {
			e := r[0] + rng.Float64()*(r[1]-r[0])
			x := math.Exp2(e)
			if rng.IntN(2) == 0 {
				x = -x
			}
			args = append(args, x)
		}
	}
	// Arguments next to k pi/2, where the reduction cancels the most bits.
	for range *ddN {
		k := float64(rng.IntN(1 << 20))
		x := math.Nextafter(k*(math.Pi/2), math.Inf(int(rng.IntN(2)*2-1)))
		for range rng.IntN(4) {
			x = math.Nextafter(x, 0)
		}
		args = append(args, x)
	}
	for _, v := range mpfrValues {
		if v.fn == "sin" || v.fn == "cos" {
			args = append(args, math.Float64frombits(v.x))
		}
	}

	accepted := 0
	for _, x := range args {
		s, c, okS, okC := sinCosDD(x)
		if !okS && !okC {
			continue
		}
		bs, bc := sinCosBig(x)
		if okS {
			accepted++
			if want, _ := bs.Float64(); s != want {
				t.Errorf("sinCosDD(%#016x) sin = %#016x, want %#016x", math.Float64bits(x), math.Float64bits(s), math.Float64bits(want))
			}
		}
		if okC {
			accepted++
			if want, _ := bc.Float64(); c != want {
				t.Errorf("sinCosDD(%#016x) cos = %#016x, want %#016x", math.Float64bits(x), math.Float64bits(c), math.Float64bits(want))
			}
		}
	}
	tanAccepted := 0
	for _, x := range args {
		got, ok := tanDD(x)
		if !ok {
			continue
		}
		tanAccepted++
		bs, bc := sinCosBig(x)
		if want, _ := quoP(bs, bc, wprec).Float64(); got != want {
			t.Errorf("tanDD(%#016x) = %#016x, want %#016x", math.Float64bits(x), math.Float64bits(got), math.Float64bits(want))
		}
	}
	if tanAccepted < len(args)*9/10 {
		t.Errorf("the fast path decided %d of %d roundings of tan, want at least 90 percent", tanAccepted, len(args))
	}
	if total := 2 * len(args); accepted < total*9/10 {
		t.Errorf("the fast path decided %d of %d roundings, want at least 90 percent", accepted, total)
	}
}

// TestSinCosDDRange checks that the fast path declines the arguments outside
// its range.
func TestSinCosDDRange(t *testing.T) {
	for _, x := range []float64{0x1p-401, -0x1p-500, 0x1p20, -0x1p30, 1e300} {
		if _, _, okS, okC := sinCosDD(x); okS || okC {
			t.Errorf("sinCosDD(%g) accepted an argument outside its range", x)
		}
	}
}

// TestPowDD checks that each result that the fast path of Pow64 accepts is
// the result of math/big, on random bases in the whole range of an f64 and
// on random, integer and small fractional exponents.
func TestPowDD(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	fracs := []float64{1.0 / 6.0, 0.5, 1.0 / 3.0, 2.2, 1 / 2.4, -0.5, 2, 3, -1, 7, 10}
	accepted, total := 0, 0
	for i := range 6 * *ddN {
		x := math.Exp2(-1070 + rng.Float64()*2090)
		if i%3 == 0 {
			// Bases close to 1, where ln x cancels the most bits.
			x = 1 + (rng.Float64()-0.5)*math.Exp2(-rng.Float64()*50)
		}
		var y float64
		switch i % 4 {
		case 0:
			y = fracs[rng.IntN(len(fracs))]
		case 1:
			y = (rng.Float64() - 0.5) * 40
		default:
			y = (rng.Float64() - 0.5) * math.Exp2(rng.Float64()*20)
		}
		if x == 1 || y == 0 || y == 1 {
			continue
		}
		if math.Abs(y*math.Log(x)) < ddMaxExp-1 {
			total++
		}
		got, ok := powDD(x, y)
		if !ok {
			continue
		}
		if math.Abs(y*math.Log(x)) < ddMaxExp-1 {
			accepted++
		}
		if want, _ := powBig(x, y).Float64(); got != want {
			t.Errorf("powDD(%#016x, %#016x) = %#016x, want %#016x", math.Float64bits(x), math.Float64bits(y), math.Float64bits(got), math.Float64bits(want))
		}
	}
	for _, v := range mpfrValues {
		if v.fn != "pow" {
			continue
		}
		x, y := math.Float64frombits(v.x), math.Float64frombits(v.y)
		if x <= 0 || math.IsInf(x, 0) || math.IsInf(y, 0) || x != x || y != y {
			continue
		}
		if got, ok := powDD(x, y); ok && math.Float64bits(got) != v.want {
			t.Errorf("powDD(%#016x, %#016x) = %#016x, want %#016x from MPFR", v.x, v.y, math.Float64bits(got), v.want)
		}
	}
	// Only the arguments in the range of the fast path count. math/big
	// computes a result that overflows or underflows.
	if accepted < total*9/10 {
		t.Errorf("the fast path of pow decided %d of %d roundings in its range, want at least 90 percent", accepted, total)
	}
}

// TestAtan2DD checks that each result that the fast path of Atan2_64 accepts
// is the result of math/big, in each quadrant and on ratios from 2^-500 to
// 2^500.
func TestAtan2DD(t *testing.T) {
	rng := rand.New(rand.NewPCG(5, 6))
	accepted, total := 0, 0
	for i := range 6 * *ddN {
		x := math.Exp2(-500 + rng.Float64()*1000)
		y := x * math.Exp2((rng.Float64()-0.5)*2*float64([]int{4, 60, 999}[i%3]))
		if rng.IntN(2) == 0 {
			x = -x
		}
		if rng.IntN(2) == 0 {
			y = -y
		}
		if r := math.Abs(y / x); r < ddAtanMin || r > ddAtanMax {
			continue
		}
		total++
		got, ok := atan2DD(y, x)
		if !ok {
			continue
		}
		accepted++
		if want, _ := atan2Big(y, x).Float64(); got != want {
			t.Errorf("atan2DD(%#016x, %#016x) = %#016x, want %#016x", math.Float64bits(y), math.Float64bits(x), math.Float64bits(got), math.Float64bits(want))
		}
	}
	for _, v := range mpfrValues {
		if v.fn != "atan2" {
			continue
		}
		x, y := math.Float64frombits(v.x), math.Float64frombits(v.y)
		if x == 0 || y == 0 || math.IsInf(x, 0) || math.IsInf(y, 0) || x != x || y != y {
			continue
		}
		if got, ok := atan2DD(y, x); ok && math.Float64bits(got) != v.want {
			t.Errorf("atan2DD(%#016x, %#016x) = %#016x, want %#016x from MPFR", v.x, v.y, math.Float64bits(got), v.want)
		}
	}
	if accepted < total*9/10 {
		t.Errorf("the fast path of atan2 decided %d of %d roundings, want at least 90 percent", accepted, total)
	}
}

// TestTinyArgs checks the results of Sin64, Cos64 and Tan64 below
// ddTinyArg against math/big.
func TestTinyArgs(t *testing.T) {
	for _, x := range []float64{0x1p-27 * 0.999, -0x1.fffffffffffffp-28, 0x1p-28, 1e-10, -3e-300, 5e-324} {
		s, c := sinCosBig(x)
		ws, _ := s.Float64()
		wc, _ := c.Float64()
		wt, _ := quoP(s, c, wprec).Float64()
		if Sin64(x) != ws || Cos64(x) != wc || Tan64(x) != wt {
			t.Errorf("x = %g: got %g, %g, %g, want %g, %g, %g", x, Sin64(x), Cos64(x), Tan64(x), ws, wc, wt)
		}
	}
}

// TestSinCos64 checks that SinCos64 gives Sin64 and Cos64, on random
// arguments in the whole range of an f64 and on the special values.
func TestSinCos64(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 8))
	args := []float64{0, math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.NaN(), 5e-324, 1e300}
	for range *ddN {
		x := math.Exp2(-1074 + rng.Float64()*2097)
		if rng.IntN(2) == 0 {
			x = -x
		}
		args = append(args, x)
	}
	for _, x := range args {
		s, c := SinCos64(x)
		ws, wc := Sin64(x), Cos64(x)
		if math.Float64bits(s) != math.Float64bits(ws) && !(s != s && ws != ws) ||
			math.Float64bits(c) != math.Float64bits(wc) && !(c != c && wc != wc) {
			t.Errorf("SinCos64(%g) = %g, %g, want %g, %g", x, s, c, ws, wc)
		}
	}
}

// lnHardArgs returns the hard arguments of ln and log2: the f64 values next
// to 1, the powers of two and their neighbors, sqrt(1/2) times a power of two
// and its neighbors, where the fast path changes e, the subnormal values, and
// the largest values.
func lnHardArgs(rng *rand.Rand) []float64 {
	args := []float64{5e-324, 0x1p-1022, math.Nextafter(0x1p-1022, 0), math.MaxFloat64, 0x1p1023}
	up, down := 1.0, 1.0
	for range 200 {
		up = math.Nextafter(up, 2)
		down = math.Nextafter(down, 0)
		args = append(args, up, down)
	}
	for range *ddN {
		// 1 + d with d from 2^-53 to 2^-1, and 1 - d/2.
		d := math.Exp2(-1 - rng.Float64()*52)
		args = append(args, 1+d, 1-d/2)
		p := math.Exp2(float64(rng.IntN(2098) - 1074))
		args = append(args, p, math.Nextafter(p, 0), math.Nextafter(p, math.Inf(1)))
		s := math.Ldexp(math.Sqrt2/2, rng.IntN(2040)-1015)
		args = append(args, s, math.Nextafter(s, 0), math.Nextafter(s, math.Inf(1)))
		args = append(args, math.Float64frombits(1+rng.Uint64N(1<<52-1)))
	}
	return args
}

// TestLnDD checks that each result that the fast paths of Ln64 and Log2_64
// accept is the result of math/big, on random arguments in the whole range of
// an f64, on the hard arguments of lnHardArgs and on the rows of MPFR. It also
// checks that Ln64 and Log2_64 give the result of math/big on each of these
// arguments.
func TestLnDD(t *testing.T) {
	rng := rand.New(rand.NewPCG(9, 10))
	args := lnHardArgs(rng)
	for range 4 * *ddN {
		args = append(args, math.Exp2(-1074+rng.Float64()*2098))
	}
	for _, v := range mpfrValues {
		if v.fn == "ln" || v.fn == "log2" {
			args = append(args, math.Float64frombits(v.x))
		}
	}
	accepted, total := 0, 0
	for _, x := range args {
		if x <= 0 || math.IsInf(x, 0) || x == 1 {
			continue
		}
		total += 2
		want := ln64Big(x)
		if got, ok := ln64DD(x); ok {
			accepted++
			if got != want {
				t.Errorf("ln64DD(%#016x) = %#016x, want %#016x", math.Float64bits(x), math.Float64bits(got), math.Float64bits(want))
			}
		}
		if got := Ln64(x); got != want {
			t.Errorf("Ln64(%#016x) = %#016x, want %#016x", math.Float64bits(x), math.Float64bits(got), math.Float64bits(want))
		}
		want = log2_64Big(x)
		if got, ok := log2DD(x); ok {
			accepted++
			if got != want {
				t.Errorf("log2DD(%#016x) = %#016x, want %#016x", math.Float64bits(x), math.Float64bits(got), math.Float64bits(want))
			}
		}
		if got := Log2_64(x); got != want {
			t.Errorf("Log2_64(%#016x) = %#016x, want %#016x", math.Float64bits(x), math.Float64bits(got), math.Float64bits(want))
		}
	}
	if accepted < total*9/10 {
		t.Errorf("the fast paths of ln and log2 decided %d of %d roundings, want at least 90 percent", accepted, total)
	}
}

// TestAcosDD checks that each result that the fast path of Acos64 accepts is
// the result of math/big, on random arguments in [-1, 1], next to 1, -1 and
// 0, on the subnormal values, and next to the values where pi/2 - x changes
// its rounding. It also checks that Acos64 gives the result of math/big on
// each of these arguments.
func TestAcosDD(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12))
	args := []float64{0, math.Copysign(0, -1), 0.5, -0.5, 5e-324, -5e-324, 0x1p-1022, -0x1p-1022, 0.9831809036248165}
	up, down := -1.0, 1.0
	for range 200 {
		up = math.Nextafter(up, 0)
		down = math.Nextafter(down, 0)
		args = append(args, up, down)
	}
	// Near 0, acos(x) = pi/2 - x to within 2^-150. It is the middle of two
	// f64 values when x = pi/2 - m, for m the middle of two f64 values next
	// to pi/2. Such an x has a fine step, so some of its neighbors give a
	// result too close to the middle for the fast path.
	initWide()
	halfPi := new(big.Float).SetPrec(wprec).SetMantExp(wide.pi, -1)
	hi, _ := halfPi.Float64()
	for _, j := range []float64{-3, -1, 1, 3} {
		m := addP(fP(hi, wprec), fP(j*0x1p-53, wprec), wprec)
		x, _ := subP(halfPi, m, wprec).Float64()
		args = append(args, x)
		up, down := x, x
		for range 100 {
			up = math.Nextafter(up, 1)
			down = math.Nextafter(down, -1)
			args = append(args, up, down)
		}
	}
	for range *ddN {
		args = append(args, rng.Float64()*2-1)
		// 1 - d and -1 + d with d from 2^-53 to 2^-1.
		d := math.Exp2(-1 - rng.Float64()*52)
		args = append(args, 1-d, -1+d)
		// A value near 0, the subnormal values too.
		s := math.Exp2(-1074 + rng.Float64()*1073)
		if rng.IntN(2) == 0 {
			s = -s
		}
		args = append(args, s)
	}
	accepted, total := 0, 0
	for _, x := range args {
		if x <= -1 || x >= 1 {
			continue
		}
		total++
		want := acos64Big(x)
		if got, ok := acosDD(x); ok {
			accepted++
			if got != want {
				t.Errorf("acosDD(%#016x) = %#016x, want %#016x", math.Float64bits(x), math.Float64bits(got), math.Float64bits(want))
			}
		}
		if got := Acos64(x); got != want {
			t.Errorf("Acos64(%#016x) = %#016x, want %#016x", math.Float64bits(x), math.Float64bits(got), math.Float64bits(want))
		}
	}
	if accepted < total*9/10 {
		t.Errorf("the fast path of acos decided %d of %d roundings, want at least 90 percent", accepted, total)
	}
}
