package wide

import "testing"

// f32x4Vec builds and reads an F32x4 for the runners of wide_test.go.
var f32x4Vec = floatVec[F32x4]{
	width: 4,
	from:  func(l []float32) F32x4 { return F32x4(l) },
	lanes: func(v F32x4) []float32 { return v[:] },
}

func i32x4Lanes(v I32x4) []int32 { return v[:] }

func TestSplatF32x4(t *testing.T) {
	for _, n := range []float32{0, negZero, 1.5, nan, -inf} {
		got := SplatF32x4(n)
		for i := range got {
			checkF32Lane(t, "SplatF32x4", i, n, got[i], n)
		}
	}
}

func TestF32x4Floor(t *testing.T) {
	runUnary(t, f32x4Vec, "Floor", floorTruncCases, F32x4.Floor)
}

func TestF32x4Abs(t *testing.T) {
	runUnaryBits(t, f32x4Vec, "Abs", absCases, F32x4.Abs)
}

func TestF32x4Max(t *testing.T) {
	runBinary(t, f32x4Vec, "Max", maxCases, F32x4.Max)
}

func TestF32x4Min(t *testing.T) {
	runBinary(t, f32x4Vec, "Min", minCases, F32x4.Min)
}

func TestF32x4CmpEq(t *testing.T) {
	runCmp(t, f32x4Vec, "CmpEq", func(c cmpF32) bool { return c.eq }, F32x4.CmpEq)
}

func TestF32x4CmpNe(t *testing.T) {
	runCmp(t, f32x4Vec, "CmpNe", func(c cmpF32) bool { return c.ne }, F32x4.CmpNe)
}

func TestF32x4CmpGe(t *testing.T) {
	runCmp(t, f32x4Vec, "CmpGe", func(c cmpF32) bool { return c.ge }, F32x4.CmpGe)
}

func TestF32x4CmpGt(t *testing.T) {
	runCmp(t, f32x4Vec, "CmpGt", func(c cmpF32) bool { return c.gt }, F32x4.CmpGt)
}

func TestF32x4CmpLe(t *testing.T) {
	runCmp(t, f32x4Vec, "CmpLe", func(c cmpF32) bool { return c.le }, F32x4.CmpLe)
}

func TestF32x4CmpLt(t *testing.T) {
	runCmp(t, f32x4Vec, "CmpLt", func(c cmpF32) bool { return c.lt }, F32x4.CmpLt)
}

func TestF32x4Blend(t *testing.T) {
	runBlend(t, f32x4Vec, F32x4.Blend)
}

func TestF32x4Round(t *testing.T) {
	runUnary(t, f32x4Vec, "Round", roundCases, F32x4.Round)
}

func TestF32x4RoundInt(t *testing.T) {
	runToI32(t, f32x4Vec, "RoundInt", roundIntCases, F32x4.RoundInt, i32x4Lanes)
}

func TestF32x4TruncInt(t *testing.T) {
	runToI32(t, f32x4Vec, "TruncInt", truncIntCases, F32x4.TruncInt, i32x4Lanes)
}

func TestF32x4RecipFast(t *testing.T) {
	runUnary(t, f32x4Vec, "RecipFast", recipFastCases, F32x4.RecipFast)
}

func TestF32x4RecipSqrt(t *testing.T) {
	runUnary(t, f32x4Vec, "RecipSqrt", recipSqrtCases, F32x4.RecipSqrt)
}

func TestF32x4Sqrt(t *testing.T) {
	runUnary(t, f32x4Vec, "Sqrt", sqrtCases, F32x4.Sqrt)
}

func TestF32x4Add(t *testing.T) {
	runBinary(t, f32x4Vec, "Add", addCases, F32x4.Add)
}

func TestF32x4Sub(t *testing.T) {
	runBinary(t, f32x4Vec, "Sub", subCases, F32x4.Sub)
}

func TestF32x4Mul(t *testing.T) {
	runBinary(t, f32x4Vec, "Mul", mulCases, F32x4.Mul)
}

func TestF32x4Div(t *testing.T) {
	runBinary(t, f32x4Vec, "Div", divCases, F32x4.Div)
}

func TestF32x4And(t *testing.T) {
	runBinaryBits(t, f32x4Vec, "And", andCases, F32x4.And)
}

func TestF32x4Or(t *testing.T) {
	runBinaryBits(t, f32x4Vec, "Or", orCases, F32x4.Or)
}

func TestF32x4Xor(t *testing.T) {
	runBinaryBits(t, f32x4Vec, "Xor", xorCases, F32x4.Xor)
}

func TestF32x4Neg(t *testing.T) {
	runUnary(t, f32x4Vec, "Neg", negCases, F32x4.Neg)
}

func TestF32x4Not(t *testing.T) {
	runUnaryBits(t, f32x4Vec, "Not", notCases, F32x4.Not)
}

func TestF32x4Equal(t *testing.T) {
	runEqual(t, f32x4Vec, F32x4.Equal)
}
