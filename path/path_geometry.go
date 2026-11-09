// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import "github.com/xo/okage/internal/crmath"

// This file is the port of the public module path_geometry of tiny-skia-path,
// a collection of functions that work with Bezier paths. Upstream says that it
// is mainly for internal use, and that a user must not rely on it.

// QuadCoeff is QuadCoeff in tiny-skia-path path/src/path_geometry.rs.
//
// use for : eval(t) == A * t^2 + B * t + C.
type QuadCoeff struct {
	A F32x2
	B F32x2
	C F32x2
}

// QuadCoeffFromPoints is QuadCoeff::from_points in tiny-skia-path
// path/src/path_geometry.rs.
func QuadCoeffFromPoints(points *[3]Point) QuadCoeff {
	c := points[0].ToF32x2()
	p1 := points[1].ToF32x2()
	p2 := points[2].ToF32x2()
	b := times2(p1.Sub(c))
	a := p2.Sub(times2(p1)).Add(c)

	return QuadCoeff{A: a, B: b, C: c}
}

// Eval is QuadCoeff::eval in tiny-skia-path path/src/path_geometry.rs.
func (q QuadCoeff) Eval(t F32x2) F32x2 {
	return q.A.Mul(t).Add(q.B).Mul(t).Add(q.C)
}

// CubicCoeff is CubicCoeff in tiny-skia-path path/src/path_geometry.rs.
type CubicCoeff struct {
	A F32x2
	B F32x2
	C F32x2
	D F32x2
}

// CubicCoeffFromPoints is CubicCoeff::from_points in tiny-skia-path
// path/src/path_geometry.rs.
func CubicCoeffFromPoints(points *[4]Point) CubicCoeff {
	p0 := points[0].ToF32x2()
	p1 := points[1].ToF32x2()
	p2 := points[2].ToF32x2()
	p3 := points[3].ToF32x2()
	three := F32x2Splat(3.0)

	return CubicCoeff{
		A: p3.Add(three.Mul(p1.Sub(p2))).Sub(p0),
		B: three.Mul(p2.Sub(times2(p1)).Add(p0)),
		C: three.Mul(p1.Sub(p0)),
		D: p0,
	}
}

// Eval is CubicCoeff::eval in tiny-skia-path path/src/path_geometry.rs.
func (c CubicCoeff) Eval(t F32x2) F32x2 {
	return c.A.Mul(t).Add(c.B).Mul(t).Add(c.C).Mul(t).Add(c.D)
}

// NewTValues is new_t_values in tiny-skia-path path/src/path_geometry.rs.
//
// TODO: to a custom type?
func NewTValues() [3]NormalizedF32Exclusive {
	return [3]NormalizedF32Exclusive{
		NormalizedF32ExclusiveAny,
		NormalizedF32ExclusiveAny,
		NormalizedF32ExclusiveAny,
	}
}

// ChopQuadAt is chop_quad_at in tiny-skia-path path/src/path_geometry.rs.
func ChopQuadAt(src []Point, t NormalizedF32Exclusive, dst *[5]Point) {
	p0 := src[0].ToF32x2()
	p1 := src[1].ToF32x2()
	p2 := src[2].ToF32x2()
	tt := F32x2Splat(t.Get())

	p01 := interpPathGeometry(p0, p1, tt)
	p12 := interpPathGeometry(p1, p2, tt)

	dst[0] = PointFromF32x2(p0)
	dst[1] = PointFromF32x2(p01)
	dst[2] = PointFromF32x2(interpPathGeometry(p01, p12, tt))
	dst[3] = PointFromF32x2(p12)
	dst[4] = PointFromF32x2(p2)
}

// FindUnitQuadRoots is find_unit_quad_roots in tiny-skia-path
// path/src/path_geometry.rs.
//
// From Numerical Recipes in C.
//
//	Q = -1/2 (B + sign(B) sqrt[B*B - 4*A*C])
//	x1 = Q / A
//	x2 = C / Q
func FindUnitQuadRoots(a, b, c float32, roots *[3]NormalizedF32Exclusive) int {
	if a == 0.0 {
		if r, ok := ValidUnitDivide(-c, b); ok {
			roots[0] = r
			return 1
		}
		return 0
	}

	// use doubles so we don't overflow temporarily trying to compute R
	dr := float64(float64(b)*float64(b)) - float64(4.0*float64(a)*float64(c))
	if dr < 0.0 {
		return 0
	}
	dr = sqrtF64(dr)
	r := float32(dr)
	if !isFinite(r) {
		return 0
	}

	var q float32
	if b < 0.0 {
		q = -(b - r) / 2.0
	} else {
		q = -(b + r) / 2.0
	}

	rootsOffset := 0
	if r, ok := ValidUnitDivide(q, a); ok {
		roots[rootsOffset] = r
		rootsOffset++
	}

	if r, ok := ValidUnitDivide(c, q); ok {
		roots[rootsOffset] = r
		rootsOffset++
	}

	if rootsOffset == 2 {
		if roots[0].Get() > roots[1].Get() {
			roots[0], roots[1] = roots[1], roots[0]
		} else if roots[0] == roots[1] {
			// nearly-equal?
			rootsOffset-- // skip the double root
		}
	}

	return rootsOffset
}

// ChopCubicAt2 is chop_cubic_at2 in tiny-skia-path path/src/path_geometry.rs.
func ChopCubicAt2(src *[4]Point, t NormalizedF32Exclusive, dst []Point) {
	p0 := src[0].ToF32x2()
	p1 := src[1].ToF32x2()
	p2 := src[2].ToF32x2()
	p3 := src[3].ToF32x2()
	tt := F32x2Splat(t.Get())

	ab := interpPathGeometry(p0, p1, tt)
	bc := interpPathGeometry(p1, p2, tt)
	cd := interpPathGeometry(p2, p3, tt)
	abc := interpPathGeometry(ab, bc, tt)
	bcd := interpPathGeometry(bc, cd, tt)
	abcd := interpPathGeometry(abc, bcd, tt)

	dst[0] = PointFromF32x2(p0)
	dst[1] = PointFromF32x2(ab)
	dst[2] = PointFromF32x2(abc)
	dst[3] = PointFromF32x2(abcd)
	dst[4] = PointFromF32x2(bcd)
	dst[5] = PointFromF32x2(cd)
	dst[6] = PointFromF32x2(p3)
}

// findQuadExtrema is find_quad_extrema in tiny-skia-path
// path/src/path_geometry.rs.
//
// Quad'(t) = At + B, where
// A = 2(a - 2b + c)
// B = 2(b - a)
// Solve for t, only if it fits between 0 < t < 1.
func findQuadExtrema(a, b, c float32) (NormalizedF32Exclusive, bool) {
	// At + B == 0
	// t = -B / A
	return ValidUnitDivide(a-b, a-b-b+c)
}

// ValidUnitDivide is valid_unit_divide in tiny-skia-path
// path/src/path_geometry.rs.
func ValidUnitDivide(numer, denom float32) (NormalizedF32Exclusive, bool) {
	if numer < 0.0 {
		numer = -numer
		denom = -denom
	}

	if denom == 0.0 || numer == 0.0 || numer >= denom {
		return 0, false
	}

	r := numer / denom
	return NewNormalizedF32Exclusive(r)
}

// interpPathGeometry is interp in tiny-skia-path path/src/path_geometry.rs.
// The name takes the suffix of its module, because dash.rs also has an interp
// (D10).
func interpPathGeometry(v0, v1, t F32x2) F32x2 {
	return v0.Add(v1.Sub(v0).Mul(t))
}

// times2 is times_2 in tiny-skia-path path/src/path_geometry.rs.
func times2(value F32x2) F32x2 {
	return value.Add(value)
}

// findQuadMaxCurvature is find_quad_max_curvature in tiny-skia-path
// path/src/path_geometry.rs.
//
//	F(t)    = a (1 - t) ^ 2 + 2 b t (1 - t) + c t ^ 2
//	F'(t)   = 2 (b - a) + 2 (a - 2b + c) t
//	F''(t)  = 2 (a - 2b + c)
//
//	A = 2 (b - a)
//	B = 2 (a - 2b + c)
//
// Maximum curvature for a quadratic means solving
//
//	Fx' Fx'' + Fy' Fy'' = 0
//
//	t = - (Ax Bx + Ay By) / (Bx ^ 2 + By ^ 2)
func findQuadMaxCurvature(src *[3]Point) NormalizedF32 {
	ax := src[1].X - src[0].X
	ay := src[1].Y - src[0].Y
	bx := src[0].X - src[1].X - src[1].X + src[2].X
	by := src[0].Y - src[1].Y - src[1].Y + src[2].Y

	numer := -(float32(ax*bx) + float32(ay*by))
	denom := float32(bx*bx) + float32(by*by)
	if denom < 0.0 {
		numer = -numer
		denom = -denom
	}

	if numer <= 0.0 {
		return NormalizedF32Zero
	}

	if numer >= denom {
		// Also catches denom=0
		return NormalizedF32One
	}

	t := numer / denom
	n, ok := NewNormalizedF32(t)
	if !ok {
		panic("path: find_quad_max_curvature: NormalizedF32::new returned None")
	}
	return n
}

// evalQuadAt is eval_quad_at in tiny-skia-path path/src/path_geometry.rs.
func evalQuadAt(src *[3]Point, t NormalizedF32) Point {
	return PointFromF32x2(QuadCoeffFromPoints(src).Eval(F32x2Splat(t.Get())))
}

// evalQuadTangentAt is eval_quad_tangent_at in tiny-skia-path
// path/src/path_geometry.rs.
func evalQuadTangentAt(src *[3]Point, tol NormalizedF32) Point {
	// The derivative equation is 2(b - a +(a - 2b +c)t). This returns a
	// zero tangent vector when t is 0 or 1, and the control point is equal
	// to the end point. In this case, use the quad end points to compute the tangent.
	if (tol == NormalizedF32Zero && src[0] == src[1]) ||
		(tol == NormalizedF32One && src[1] == src[2]) {
		return src[2].Sub(src[0])
	}

	p0 := src[0].ToF32x2()
	p1 := src[1].ToF32x2()
	p2 := src[2].ToF32x2()

	b := p1.Sub(p0)
	a := p2.Sub(p1).Sub(b)
	t := a.Mul(F32x2Splat(tol.Get())).Add(b)

	return PointFromF32x2(t.Add(t))
}

// FindCubicMaxCurvature is find_cubic_max_curvature in tiny-skia-path
// path/src/path_geometry.rs. It returns the part of tValues that holds the
// result.
//
// Looking for
//
//	F' dot F'' == 0
//
//	A = b - a
//	B = c - 2b + a
//	C = d - 3c + 3b - a
//
//	F' = 3Ct^2 + 6Bt + 3A
//	F'' = 6Ct + 6B
//
//	F' dot F'' -> CCt^3 + 3BCt^2 + (2BB + CA)t + AB
func FindCubicMaxCurvature(src *[4]Point, tValues *[3]NormalizedF32) []NormalizedF32 {
	coeffX := formulateF1DotF2(&[4]float32{src[0].X, src[1].X, src[2].X, src[3].X})
	coeffY := formulateF1DotF2(&[4]float32{src[0].Y, src[1].Y, src[2].Y, src[3].Y})

	for i := range 4 {
		coeffX[i] += coeffY[i]
	}

	n := solveCubicPoly(&coeffX, tValues)
	return tValues[0:n]
}

// formulateF1DotF2 is formulate_f1_dot_f2 in tiny-skia-path
// path/src/path_geometry.rs.
//
// Looking for
//
//	F' dot F'' == 0
//
//	A = b - a
//	B = c - 2b + a
//	C = d - 3c + 3b - a
//
//	F' = 3Ct^2 + 6Bt + 3A
//	F'' = 6Ct + 6B
//
//	F' dot F'' -> CCt^3 + 3BCt^2 + (2BB + CA)t + AB
func formulateF1DotF2(src *[4]float32) [4]float32 {
	a := src[1] - src[0]
	b := src[2] - float32(2.0*src[1]) + src[0]
	c := src[3] + float32(3.0*(src[1]-src[2])) - src[0]

	return [4]float32{
		float32(c * c),
		float32(3.0 * b * c),
		float32(2.0*b*b) + float32(c*a),
		float32(a * b),
	}
}

// solveCubicPoly is solve_cubic_poly in tiny-skia-path
// path/src/path_geometry.rs.
//
// Solve coeff(t) == 0, returning the number of roots that lie within 0 < t < 1.
// coeff[0]t^3 + coeff[1]t^2 + coeff[2]t + coeff[3]
//
// Eliminates repeated roots (so that all t_values are distinct, and are always
// in increasing order.
func solveCubicPoly(coeff *[4]float32, tValues *[3]NormalizedF32) int {
	if IsNearlyZero(coeff[0]) {
		// we're just a quadratic
		tmpT := NewTValues()
		count := FindUnitQuadRoots(coeff[1], coeff[2], coeff[3], &tmpT)
		for i := range count {
			tValues[i] = tmpT[i].ToNormalized()
		}

		return count
	}

	inva := Invert(coeff[0])
	a := float32(coeff[1] * inva)
	b := float32(coeff[2] * inva)
	c := float32(coeff[3] * inva)

	q := (float32(a*a) - float32(b*3.0)) / 9.0
	r := (float32(2.0*a*a*a) - float32(9.0*a*b) + float32(27.0*c)) / 54.0

	q3 := float32(q * q * q)
	r2MinusQ3 := float32(r*r) - q3
	adiv3 := a / 3.0

	if r2MinusQ3 < 0.0 {
		// we have 3 real roots
		// the divide/root can, due to finite precisions, be slightly outside of -1...1
		theta := acosF32(Bound(r/sqrtF32(q3), -1.0, 1.0))
		neg2RootQ := float32(-2.0 * sqrtF32(q))

		tValues[0] = NormalizedF32Clamped(float32(neg2RootQ*cosF32(theta/3.0)) - adiv3)
		tValues[1] = NormalizedF32Clamped(
			float32(neg2RootQ*cosF32((theta+2.0*floatPi)/3.0)) - adiv3,
		)
		tValues[2] = NormalizedF32Clamped(
			float32(neg2RootQ*cosF32((theta-2.0*floatPi)/3.0)) - adiv3,
		)

		// now sort the roots
		sortArray3(tValues)
		return collapseDuplicates3(tValues)
	}
	// we have 1 real root
	a = absF32(r) + sqrtF32(r2MinusQ3)
	a = scalarCubeRoot(a)
	if r > 0.0 {
		a = -a
	}

	if a != 0.0 {
		a += q / a
	}

	tValues[0] = NormalizedF32Clamped(a - adiv3)
	return 1
}

// sortArray3 is sort_array3 in tiny-skia-path path/src/path_geometry.rs.
func sortArray3(array *[3]NormalizedF32) {
	if array[0] > array[1] {
		array[0], array[1] = array[1], array[0]
	}

	if array[1] > array[2] {
		array[1], array[2] = array[2], array[1]
	}

	if array[0] > array[1] {
		array[0], array[1] = array[1], array[0]
	}
}

// collapseDuplicates3 is collapse_duplicates3 in tiny-skia-path
// path/src/path_geometry.rs.
func collapseDuplicates3(array *[3]NormalizedF32) int {
	n := 3

	if array[1] == array[2] {
		n = 2
	}

	if array[0] == array[1] {
		n = 1
	}

	return n
}

// scalarCubeRoot is scalar_cube_root in tiny-skia-path
// path/src/path_geometry.rs.
func scalarCubeRoot(x float32) float32 {
	return powF32(x, 0.3333333)
}

// evalCubicPosAt is eval_cubic_pos_at in tiny-skia-path
// path/src/path_geometry.rs.
//
// This is SkEvalCubicAt split into three functions.
func evalCubicPosAt(src *[4]Point, t NormalizedF32) Point {
	return PointFromF32x2(CubicCoeffFromPoints(src).Eval(F32x2Splat(t.Get())))
}

// evalCubicTangentAt is eval_cubic_tangent_at in tiny-skia-path
// path/src/path_geometry.rs.
//
// This is SkEvalCubicAt split into three functions.
func evalCubicTangentAt(src *[4]Point, t NormalizedF32) Point {
	// The derivative equation returns a zero tangent vector when t is 0 or 1, and the
	// adjacent control point is equal to the end point. In this case, use the
	// next control point or the end points to compute the tangent.
	if (t.Get() == 0.0 && src[0] == src[1]) || (t.Get() == 1.0 && src[2] == src[3]) {
		var tangent Point
		if t.Get() == 0.0 {
			tangent = src[2].Sub(src[0])
		} else {
			tangent = src[3].Sub(src[1])
		}

		if tangent.X == 0.0 && tangent.Y == 0.0 {
			tangent = src[3].Sub(src[0])
		}

		return tangent
	}
	return evalCubicDerivative(src, t)
}

// evalCubicDerivative is eval_cubic_derivative in tiny-skia-path
// path/src/path_geometry.rs.
func evalCubicDerivative(src *[4]Point, t NormalizedF32) Point {
	p0 := src[0].ToF32x2()
	p1 := src[1].ToF32x2()
	p2 := src[2].ToF32x2()
	p3 := src[3].ToF32x2()

	coeff := QuadCoeff{
		A: p3.Add(F32x2Splat(3.0).Mul(p1.Sub(p2))).Sub(p0),
		B: times2(p2.Sub(times2(p1)).Add(p0)),
		C: p1.Sub(p0),
	}

	return PointFromF32x2(coeff.Eval(F32x2Splat(t.Get())))
}

// findCubicExtrema is find_cubic_extrema in tiny-skia-path
// path/src/path_geometry.rs.
//
// Cubic'(t) = At^2 + Bt + C, where
// A = 3(-a + 3(b - c) + d)
// B = 6(a - 2b + c)
// C = 3(b - a)
// Solve for t, keeping only those that fit between 0 < t < 1.
func findCubicExtrema(a, b, c, d float32, tValues *[3]NormalizedF32Exclusive) int {
	// we divide A,B,C by 3 to simplify
	aa := d - a + float32(3.0*(b-c))
	bb := float32(2.0 * (a - b - b + c))
	cc := b - a

	return FindUnitQuadRoots(aa, bb, cc, tValues)
}

// findCubicInflections is find_cubic_inflections in tiny-skia-path
// path/src/path_geometry.rs.
//
// http://www.faculty.idc.ac.il/arik/quality/appendixA.html
//
// Inflection means that curvature is zero.
//
//	Curvature is [F' x F''] / [F'^3]
//	So we solve F'x X F''y - F'y X F''y == 0
//	After some canceling of the cubic term, we get
//	A = b - a
//	B = c - 2b + a
//	C = d - 3c + 3b - a
//	(BxCy - ByCx)t^2 + (AxCy - AyCx)t + AxBy - AyBx == 0
func findCubicInflections(src *[4]Point, tValues *[3]NormalizedF32Exclusive) []NormalizedF32Exclusive {
	ax := src[1].X - src[0].X
	ay := src[1].Y - src[0].Y
	bx := src[2].X - float32(2.0*src[1].X) + src[0].X
	by := src[2].Y - float32(2.0*src[1].Y) + src[0].Y
	cx := src[3].X + float32(3.0*(src[1].X-src[2].X)) - src[0].X
	cy := src[3].Y + float32(3.0*(src[1].Y-src[2].Y)) - src[0].Y

	n := FindUnitQuadRoots(
		float32(bx*cy)-float32(by*cx),
		float32(ax*cy)-float32(ay*cx),
		float32(ax*by)-float32(ay*bx),
		tValues,
	)

	return tValues[0:n]
}

// findCubicCusp is find_cubic_cusp in tiny-skia-path
// path/src/path_geometry.rs.
//
// Return location (in t) of cubic cusp, if there is one.
// Note that classify cubic code does not reliably return all cusp'd cubics, so
// it is not called here.
func findCubicCusp(src *[4]Point) (NormalizedF32Exclusive, bool) {
	// When the adjacent control point matches the end point, it behaves as if
	// the cubic has a cusp: there's a point of max curvature where the derivative
	// goes to zero. Ideally, this would be where t is zero or one, but math
	// error makes not so. It is not uncommon to create cubics this way; skip them.
	if src[0] == src[1] {
		return 0, false
	}

	if src[2] == src[3] {
		return 0, false
	}

	// Cubics only have a cusp if the line segments formed by the control and end points cross.
	// Detect crossing if line ends are on opposite sides of plane formed by the other line.
	if onSameSide(src, 0, 2) || onSameSide(src, 2, 0) {
		return 0, false
	}

	// Cubics may have multiple points of maximum curvature, although at most only
	// one is a cusp.
	tValues := [3]NormalizedF32{NormalizedF32Zero, NormalizedF32Zero, NormalizedF32Zero}
	maxCurvature := FindCubicMaxCurvature(src, &tValues)
	for _, testT := range maxCurvature {
		if 0.0 >= testT.Get() || testT.Get() >= 1.0 {
			// no need to consider max curvature on the end
			continue
		}

		// A cusp is at the max curvature, and also has a derivative close to zero.
		// Choose the 'close to zero' meaning by comparing the derivative length
		// with the overall cubic size.
		dPt := evalCubicDerivative(src, testT)
		dPtMagnitude := dPt.lengthSqd()
		precision := calcCubicPrecision(src)
		if dPtMagnitude < precision {
			// All three max curvature t values may be close to the cusp;
			// return the first one.
			return NormalizedF32ExclusiveBounded(testT.Get()), true
		}
	}

	return 0, false
}

// onSameSide is on_same_side in tiny-skia-path path/src/path_geometry.rs.
//
// Returns true if both points src[testIndex], src[testIndex+1] are in the same half plane defined
// by the line segment src[lineIndex], src[lineIndex+1].
func onSameSide(src *[4]Point, testIndex, lineIndex int) bool {
	origin := src[lineIndex]
	line := src[lineIndex+1].Sub(origin)
	var crosses [2]float32
	for index := range 2 {
		testLine := src[testIndex+index].Sub(origin)
		crosses[index] = line.Cross(testLine)
	}

	return float32(crosses[0]*crosses[1]) >= 0.0
}

// calcCubicPrecision is calc_cubic_precision in tiny-skia-path
// path/src/path_geometry.rs.
//
// Returns a constant proportional to the dimensions of the cubic.
// Constant found through experimentation -- maybe there's a better way....
func calcCubicPrecision(src *[4]Point) float32 {
	return float32((src[1].distanceToSqd(src[0]) +
		src[2].distanceToSqd(src[1]) +
		src[3].distanceToSqd(src[2])) *
		1e-8)
}

// conic is Conic in tiny-skia-path path/src/path_geometry.rs.
type conic struct {
	points [3]Point
	weight float32
}

// newConic is Conic::new in tiny-skia-path path/src/path_geometry.rs.
func newConic(pt0, pt1, pt2 Point, weight float32) conic {
	return conic{
		points: [3]Point{pt0, pt1, pt2},
		weight: weight,
	}
}

// conicFromPoints is Conic::from_points in tiny-skia-path
// path/src/path_geometry.rs.
func conicFromPoints(points []Point, weight float32) conic {
	return conic{
		points: [3]Point{points[0], points[1], points[2]},
		weight: weight,
	}
}

// computeQuadPow2 is Conic::compute_quad_pow2 in tiny-skia-path
// path/src/path_geometry.rs.
func (c *conic) computeQuadPow2(tolerance float32) (uint8, bool) {
	if tolerance < 0.0 || !isFinite(tolerance) {
		return 0, false
	}

	if !c.points[0].IsFinite() || !c.points[1].IsFinite() || !c.points[2].IsFinite() {
		return 0, false
	}

	// Limit the number of suggested quads to approximate a conic
	const maxConicToQuadPow2 = 4

	// "High order approximation of conic sections by quadratic splines"
	// by Michael Floater, 1993
	a := c.weight - 1.0
	k := a / float32(4.0*(2.0+a))
	x := float32(k * (c.points[0].X - float32(2.0*c.points[1].X) + c.points[2].X))
	y := float32(k * (c.points[0].Y - float32(2.0*c.points[1].Y) + c.points[2].Y))

	err := sqrtF32(float32(x*x) + float32(y*y))
	var pow2 uint8
	for range maxConicToQuadPow2 {
		if err <= tolerance {
			break
		}

		err = float32(err * 0.25)
		pow2++
	}

	// Unlike Skia, we always expect `pow2` to be at least 1.
	// Otherwise it produces ugly results.
	return max(pow2, 1), true
}

// chopIntoQuadsPow2 is Conic::chop_into_quads_pow2 in tiny-skia-path
// path/src/path_geometry.rs.
//
// Chop this conic into N quads, stored continuously in pts[], where
// N = 1 << pow2. The amount of storage needed is (1 + 2 * N).
func (c *conic) chopIntoQuadsPow2(pow2 uint8, points []Point) uint8 {
	points[0] = c.points[0]
	subdivide(c, points[1:], pow2)

	quadCount := 1 << pow2
	ptCount := 2*quadCount + 1
	anyNotFinite := false
	for i := 0; i < ptCount && i < len(points); i++ {
		if !points[i].IsFinite() {
			anyNotFinite = true
			break
		}
	}
	if anyNotFinite {
		// if we generated a non-finite, pin ourselves to the middle of the hull,
		// as our first and last are already on the first/last pts of the hull.
		for i := 1; i < ptCount-1 && i < len(points); i++ {
			points[i] = c.points[1]
		}
	}

	return 1 << pow2
}

// chop is Conic::chop in tiny-skia-path path/src/path_geometry.rs.
func (c *conic) chop() (conic, conic) {
	scale := F32x2Splat(Invert(1.0 + c.weight))
	newW := subdivideWeightValue(c.weight)

	p0 := c.points[0].ToF32x2()
	p1 := c.points[1].ToF32x2()
	p2 := c.points[2].ToF32x2()
	ww := F32x2Splat(c.weight)

	wp1 := ww.Mul(p1)
	m := p0.Add(times2(wp1)).Add(p2).Mul(scale).Mul(F32x2Splat(0.5))
	mPt := PointFromF32x2(m)
	if !mPt.IsFinite() {
		wD := float64(c.weight)
		w2 := float64(wD * 2.0)
		scaleHalf := float64(1.0/(1.0+wD)) * 0.5
		mPt.X = float32((float64(c.points[0].X) +
			float64(w2*float64(c.points[1].X)) +
			float64(c.points[2].X)) *
			scaleHalf)

		mPt.Y = float32((float64(c.points[0].Y) +
			float64(w2*float64(c.points[1].Y)) +
			float64(c.points[2].Y)) *
			scaleHalf)
	}

	return conic{
			points: [3]Point{c.points[0], PointFromF32x2(p0.Add(wp1).Mul(scale)), mPt},
			weight: newW,
		},
		conic{
			points: [3]Point{mPt, PointFromF32x2(wp1.Add(p2).Mul(scale)), c.points[2]},
			weight: newW,
		}
}

// buildUnitArc is Conic::build_unit_arc in tiny-skia-path
// path/src/path_geometry.rs. It returns the part of dst that holds the
// result, or false if the result is empty.
func buildUnitArc(
	uStart Point,
	uStop Point,
	dir pathDirection,
	userTransform Transform,
	dst *[5]conic,
) ([]conic, bool) {
	// rotate by x,y so that u_start is (1.0)
	x := uStart.Dot(uStop)
	y := uStart.Cross(uStop)

	absY := absF32(y)

	// check for (effectively) coincident vectors
	// this can happen if our angle is nearly 0 or nearly 180 (y == 0)
	// ... we use the dot-prod to distinguish between 0 and 180 (x > 0)
	if absY <= ScalarNearlyZero &&
		x > 0.0 &&
		((y >= 0.0 && dir == pathDirectionCW) || (y <= 0.0 && dir == pathDirectionCCW)) {
		return nil, false
	}

	if dir == pathDirectionCCW {
		y = -y
	}

	// We decide to use 1-conic per quadrant of a circle. What quadrant does [xy] lie in?
	//      0 == [0  .. 90)
	//      1 == [90 ..180)
	//      2 == [180..270)
	//      3 == [270..360)
	//
	quadrant := 0
	if y == 0.0 {
		quadrant = 2 // 180
	} else if x == 0.0 {
		if y > 0.0 {
			quadrant = 1 // 90
		} else {
			quadrant = 3 // 270
		}
	} else {
		if y < 0.0 {
			quadrant += 2
		}

		if (x < 0.0) != (y < 0.0) {
			quadrant++
		}
	}

	quadrantPoints := [8]Point{
		PointFromXY(1.0, 0.0),
		PointFromXY(1.0, 1.0),
		PointFromXY(0.0, 1.0),
		PointFromXY(-1.0, 1.0),
		PointFromXY(-1.0, 0.0),
		PointFromXY(-1.0, -1.0),
		PointFromXY(0.0, -1.0),
		PointFromXY(1.0, -1.0),
	}

	const quadrantWeight = ScalarRoot2Over2

	conicCount := quadrant
	for i := range conicCount {
		dst[i] = conicFromPoints(quadrantPoints[i*2:], quadrantWeight)
	}

	// Now compute any remaining (sub-90-degree) arc for the last conic
	finalPt := PointFromXY(x, y)
	lastQ := quadrantPoints[quadrant*2] // will already be a unit-vector
	dot := lastQ.Dot(finalPt)

	if dot < 1.0 {
		offCurve := PointFromXY(lastQ.X+x, lastQ.Y+y)
		// compute the bisector vector, and then rescale to be the off-curve point.
		// we compute its length from cos(theta/2) = length / 1, using half-angle identity we get
		// length = sqrt(2 / (1 + cos(theta)). We already have cos() when to computed the dot.
		// This is nice, since our computed weight is cos(theta/2) as well!
		cosThetaOver2 := sqrtF32((1.0 + dot) / 2.0)
		offCurve.SetLength(Invert(cosThetaOver2))
		if !lastQ.almostEqual(offCurve) {
			dst[conicCount] = newConic(lastQ, offCurve, finalPt, cosThetaOver2)
			conicCount++
		}
	}

	// now handle counter-clockwise and the initial unitStart rotation
	transform := transformFromSinCos(uStart.Y, uStart.X)
	if dir == pathDirectionCCW {
		transform = transform.PreScale(1.0, -1.0)
	}

	transform = transform.PostConcat(userTransform)

	for i := range conicCount {
		transform.MapPoints(dst[i].points[:])
	}

	if conicCount == 0 {
		return nil, false
	}
	return dst[0:conicCount], true
}

// subdivideWeightValue is subdivide_weight_value in tiny-skia-path
// path/src/path_geometry.rs.
func subdivideWeightValue(w float32) float32 {
	return sqrtF32(0.5 + float32(w*0.5))
}

// subdivide is subdivide in tiny-skia-path path/src/path_geometry.rs.
func subdivide(src *conic, points []Point, level uint8) []Point {
	if level == 0 {
		points[0] = src.points[1]
		points[1] = src.points[2]
		return points[2:]
	}
	dst0, dst1 := src.chop()

	startY := src.points[0].Y
	endY := src.points[2].Y
	if between(startY, src.points[1].Y, endY) {
		// If the input is monotonic and the output is not, the scan converter hangs.
		// Ensure that the chopped conics maintain their y-order.
		midY := dst0.points[2].Y
		if !between(startY, midY, endY) {
			// If the computed midpoint is outside the ends, move it to the closer one.
			var closerY float32
			if absF32(midY-startY) < absF32(midY-endY) {
				closerY = startY
			} else {
				closerY = endY
			}
			dst0.points[2].Y = closerY
			dst1.points[0].Y = closerY
		}

		if !between(startY, dst0.points[1].Y, dst0.points[2].Y) {
			// If the 1st control is not between the start and end, put it at the start.
			// This also reduces the quad to a line.
			dst0.points[1].Y = startY
		}

		if !between(dst1.points[0].Y, dst1.points[1].Y, endY) {
			// If the 2nd control is not between the start and end, put it at the end.
			// This also reduces the quad to a line.
			dst1.points[1].Y = endY
		}
	}

	level--
	points = subdivide(&dst0, points, level)
	return subdivide(&dst1, points, level)
}

// between is between in tiny-skia-path path/src/path_geometry.rs.
//
// This was originally developed and tested for pathops: see SkOpTypes.h
// returns true if (a <= b <= c) || (a >= b >= c).
func between(a, b, c float32) bool {
	return float32((a-b)*(c-b)) <= 0.0
}

// autoConicToQuads is AutoConicToQuads in tiny-skia-path
// path/src/path_geometry.rs.
type autoConicToQuads struct {
	points [64]Point
	len    uint8 // the number of quads
}

// computeAutoConicToQuads is AutoConicToQuads::compute in tiny-skia-path
// path/src/path_geometry.rs.
func computeAutoConicToQuads(pt0, pt1, pt2 Point, weight float32) (autoConicToQuads, bool) {
	c := newConic(pt0, pt1, pt2, weight)
	pow2, ok := c.computeQuadPow2(0.25)
	if !ok {
		return autoConicToQuads{}, false
	}
	var points [64]Point
	n := c.chopIntoQuadsPow2(pow2, points[:])
	return autoConicToQuads{points: points, len: n}, true
}

// The helpers below port no upstream function. They give the result of the
// methods of f32 in the Rust standard library, which upstream calls.

// acosF32 is f32::acos. Rust calls acosf of the C library. okage gives the
// correctly rounded result, which is the same on every platform (D26).
func acosF32(x float32) float32 {
	return crmath.Acos32(x)
}

// powF32 is f32::powf. Rust calls powf of the C library. okage gives the
// correctly rounded result, which is the same on every platform (D26).
func powF32(x, y float32) float32 {
	return crmath.Pow32(x, y)
}
