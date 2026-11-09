//go:build goexperiment.simd && !okage_scalar && !tinygo

package wide

import "simd/archsimd"

// This file holds the helpers of the SIMD code on amd64: the loads, and the
// operations that must give the bits of the scalar code. The suffix of each
// name is the number of lanes. widegen does not write this file. Some
// helpers serve only SIMD code that the table of widegen turns off, and
// widegen -all turns it on.

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

// vTrunc4 is TruncI32 on each lane. VCVTTPS2DQ gives 0x80000000 for NaN and
// for a value that does not fit. vTrunc4 gives 0x7fffffff for a value of
// 2^31 or more, and 0 for NaN, as the scalar code does.
func vTrunc4(x archsimd.Float32x4) archsimd.Int32x4 {
	r := x.ConvertToInt32()
	big := x.GreaterEqual(archsimd.BroadcastFloat32x4(2147483648)).ToInt32x4()
	notNaN := x.Equal(x).ToInt32x4()
	return r.Xor(big).And(notNaN)
}

// ldF8 loads 8 float32 lanes from s.
func ldF8(s []float32) archsimd.Float32x8 {
	return archsimd.LoadFloat32x8Array((*[8]float32)(s))
}

// ldI8 loads 8 int32 lanes from s.
func ldI8(s []int32) archsimd.Int32x8 {
	return archsimd.LoadInt32x8Array((*[8]int32)(s))
}

// ldU8 loads 8 uint32 lanes from s.
func ldU8(s []uint32) archsimd.Uint32x8 {
	return archsimd.LoadUint32x8Array((*[8]uint32)(s))
}

// maskF8 gives the float lanes of m: all bits 1 where m is true, and all
// bits 0 where it is false.
func maskF8(m archsimd.Mask32x8) archsimd.Float32x8 {
	return m.ToInt32x8().ToBits().BitsToFloat32()
}

// vMin8 is fasterMin on each lane: y where y < x, and x otherwise, also for
// NaN and for 0 and -0. It compares and selects. The Min of NEON and of
// SIMD128 gives NaN for a NaN lane. VMINPS on amd64 has the rule of fasterMin
// for its two operands, but the Go compiler marks Min as commutative and can
// swap them.
func vMin8(x, y archsimd.Float32x8) archsimd.Float32x8 {
	return y.IfElse(y.Less(x), x)
}

// vMax8 is fasterMax on each lane: y where x < y, and x otherwise. It
// compares and selects, for the reasons of vMin8.
func vMax8(x, y archsimd.Float32x8) archsimd.Float32x8 {
	return y.IfElse(x.Less(y), x)
}

// vFloor8 is the scalar Floor of F32x4 and F32x8 on each lane. It truncates
// to an int32 with vTrunc8, converts back and subtracts 1 where that is
// larger than the lane.
func vFloor8(x archsimd.Float32x8) archsimd.Float32x8 {
	rt := vTrunc8(x).ConvertToFloat32()
	one := archsimd.BroadcastFloat32x8(1).ToBits()
	return rt.Sub(maskF8(rt.Greater(x)).ToBits().And(one).BitsToFloat32())
}

// vRound8 is roundF32x4 on each lane. The instruction rounds half to even,
// as the scalar code does. Two cases differ, and vRound8 fixes them: the
// scalar code gives +0 for -0.5 where the instruction gives -0, and it keeps
// the bits of NaN where the instruction can change them.
func vRound8(x archsimd.Float32x8) archsimd.Float32x8 {
	r := x.Round()
	negHalf := x.Equal(archsimd.BroadcastFloat32x8(-0.5)).ToInt32x8().ToBits()
	r = r.ToBits().AndNot(negHalf).BitsToFloat32()
	return r.IfElse(x.Equal(x), x)
}

// vTrunc8 is TruncI32 on each lane. VCVTTPS2DQ gives 0x80000000 for NaN and
// for a value that does not fit. vTrunc8 gives 0x7fffffff for a value of
// 2^31 or more, and 0 for NaN, as the scalar code does.
func vTrunc8(x archsimd.Float32x8) archsimd.Int32x8 {
	r := x.ConvertToInt32()
	big := x.GreaterEqual(archsimd.BroadcastFloat32x8(2147483648)).ToInt32x8()
	notNaN := x.Equal(x).ToInt32x8()
	return r.Xor(big).And(notNaN)
}

// ldH16 loads 16 uint16 lanes from s.
func ldH16(s []uint16) archsimd.Uint16x16 {
	return archsimd.LoadUint16x16Array((*[16]uint16)(s))
}
