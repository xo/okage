//go:build goexperiment.simd && !okage_scalar && !tinygo && (arm64 || wasm)

package wide

import "simd/archsimd"

// This file holds the helpers of the SIMD code on arm64 and wasm, where a
// vector holds 128 bits: the loads, and the operations that must give the
// bits of the scalar code. The suffix of each name is the number of lanes.
// widegen does not write this file. Some helpers serve only SIMD code that
// the table of widegen turns off, and widegen -all turns it on.

// ldF4 loads 4 float32 lanes from s.
func ldF4(s []float32) archsimd.Float32x4 {
	return archsimd.LoadFloat32x4Array((*[4]float32)(s))
}

// ldI4 loads 4 int32 lanes from s.
func ldI4(s []int32) archsimd.Int32x4 {
	return archsimd.LoadInt32x4Array((*[4]int32)(s))
}

// ldU4 loads 4 uint32 lanes from s.
func ldU4(s []uint32) archsimd.Uint32x4 {
	return archsimd.LoadUint32x4Array((*[4]uint32)(s))
}

// maskF4 gives the float lanes of m: all bits 1 where m is true, and all
// bits 0 where it is false.
func maskF4(m archsimd.Mask32x4) archsimd.Float32x4 {
	return m.ToInt32x4().ToBits().BitsToFloat32()
}

// vMin4 is fasterMin on each lane: y where y < x, and x otherwise, also for
// NaN and for 0 and -0. It compares and selects. The Min of NEON and of
// SIMD128 gives NaN for a NaN lane. VMINPS on amd64 has the rule of fasterMin
// for its two operands, but the Go compiler marks Min as commutative and can
// swap them.
func vMin4(x, y archsimd.Float32x4) archsimd.Float32x4 {
	return y.IfElse(y.Less(x), x)
}

// vMax4 is fasterMax on each lane: y where x < y, and x otherwise. It
// compares and selects, for the reasons of vMin4.
func vMax4(x, y archsimd.Float32x4) archsimd.Float32x4 {
	return y.IfElse(x.Less(y), x)
}

// vFloor4 is the scalar Floor of F32x4 and F32x8 on each lane. It truncates
// to an int32 with vTrunc4, converts back and subtracts 1 where that is
// larger than the lane.
func vFloor4(x archsimd.Float32x4) archsimd.Float32x4 {
	rt := vTrunc4(x).ConvertToFloat32()
	one := archsimd.BroadcastFloat32x4(1).ToBits()
	return rt.Sub(maskF4(rt.Greater(x)).ToBits().And(one).BitsToFloat32())
}

// vRound4 is roundF32x4 on each lane. The instruction rounds half to even,
// as the scalar code does. Two cases differ, and vRound4 fixes them: the
// scalar code gives +0 for -0.5 where the instruction gives -0, and it keeps
// the bits of NaN where the instruction can change them.
func vRound4(x archsimd.Float32x4) archsimd.Float32x4 {
	r := x.Round()
	negHalf := x.Equal(archsimd.BroadcastFloat32x4(-0.5)).ToInt32x4().ToBits()
	r = r.ToBits().AndNot(negHalf).BitsToFloat32()
	return r.IfElse(x.Equal(x), x)
}

// vTrunc4 is TruncI32 on each lane. FCVTZS on arm64 and
// i32x4.trunc_sat_f32x4_s on wasm saturate and give 0 for NaN, as the scalar
// code does.
func vTrunc4(x archsimd.Float32x4) archsimd.Int32x4 {
	return x.ConvertToInt32()
}

// ldH8 loads 8 uint16 lanes from s.
func ldH8(s []uint16) archsimd.Uint16x8 {
	return archsimd.LoadUint16x8Array((*[8]uint16)(s))
}
