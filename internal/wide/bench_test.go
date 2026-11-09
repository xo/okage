package wide

import "testing"

// The benchmarks of this file measure a few methods of each type. Each one
// runs as a sub-benchmark named after the code that the methods run: "scalar"
// in the scalar build, "simd" in the SIMD build, and "fallback" for the
// scalar code of the SIMD build, which the second run of TestMain gives.
// Each loop feeds the result back as the next input, so that the compiler
// cannot remove the call. The loops call the methods directly, without a
// function value, so that the compiler can inline them as in the pipeline.

var (
	sinkF32x4  F32x4
	sinkF32x8  F32x8
	sinkF32x16 F32x16
	sinkI32x8  I32x8
	sinkU32x8  U32x8
	sinkU16x16 U16x16
)

var (
	benchX8  = F32x8{0.5, 1.25, -2, 3.5, 100, -0.75, 7, 0.125}
	benchY8  = F32x8{1.0000001, 0.9999999, 1.0000001, 0.9999999, 1.0000001, 0.9999999, 1.0000001, 0.9999999}
	benchM8  = F32x8{f32From(maskBits), 0, f32From(maskBits), 0, 0, f32From(maskBits), 0, f32From(maskBits)}
	benchX16 = U16x16{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	benchY16 = U16x16{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	benchM16 = U16x16{0xffff, 0, 0xffff, 0, 0xffff, 0, 0xffff, 0, 0, 0xffff, 0, 0xffff, 0, 0xffff, 0, 0xffff}
)

func BenchmarkF32x8Add(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := benchX8
		for b.Loop() {
			v = v.Add(benchY8)
		}
		sinkF32x8 = v
	})
}

func BenchmarkF32x8Mul(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := benchX8
		for b.Loop() {
			v = v.Mul(benchY8)
		}
		sinkF32x8 = v
	})
}

func BenchmarkF32x8Div(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := benchX8
		for b.Loop() {
			v = v.Div(benchY8)
		}
		sinkF32x8 = v
	})
}

func BenchmarkF32x8Min(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := benchX8
		for b.Loop() {
			v = v.Min(benchY8)
		}
		sinkF32x8 = v
	})
}

func BenchmarkF32x8Max(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := benchX8
		for b.Loop() {
			v = v.Max(benchY8)
		}
		sinkF32x8 = v
	})
}

func BenchmarkF32x8Blend(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := benchX8
		for b.Loop() {
			v = benchM8.Blend(benchY8, v)
		}
		sinkF32x8 = v
	})
}

func BenchmarkF32x8Round(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := benchX8
		for b.Loop() {
			v = v.Round().Add(benchY8)
		}
		sinkF32x8 = v
	})
}

func BenchmarkF32x8Sqrt(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := benchX8
		for b.Loop() {
			v = v.Sqrt()
		}
		sinkF32x8 = v
	})
}

func BenchmarkF32x8Powf(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := F32x8{0.5, 0.25, 0.75, 0.1, 0.9, 0.01, 0.6, 0.333}
		for b.Loop() {
			v = v.Powf(1.0001)
		}
		sinkF32x8 = v
	})
}

func BenchmarkF32x4Add(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := F32x4(benchX8[:4])
		for b.Loop() {
			v = v.Add(F32x4(benchY8[:4]))
		}
		sinkF32x4 = v
	})
}

func BenchmarkF32x16Mul(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := F32x16{benchX8, benchX8}
		for b.Loop() {
			v = v.Mul(F32x16{benchY8, benchY8})
		}
		sinkF32x16 = v
	})
}

func BenchmarkI32x8Mul(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := I32x8{1, 2, 3, 4, 5, 6, 7, 8}
		for b.Loop() {
			v = v.Mul(SplatI32x8(3))
		}
		sinkI32x8 = v
	})
}

func BenchmarkU32x8Shl(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := U32x8{1, 2, 3, 4, 5, 6, 7, 8}
		for b.Loop() {
			v = v.Shl(3).Add(v)
		}
		sinkU32x8 = v
	})
}

func BenchmarkU16x16Add(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := benchX16
		for b.Loop() {
			v = v.Add(benchY16)
		}
		sinkU16x16 = v
	})
}

func BenchmarkU16x16Mul(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := benchX16
		for b.Loop() {
			v = v.Mul(benchX16)
		}
		sinkU16x16 = v
	})
}

func BenchmarkU16x16Div(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := SplatU16x16(0xffff)
		for b.Loop() {
			v = v.Div(benchY16)
		}
		sinkU16x16 = v
	})
}

func BenchmarkU16x16Blend(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := benchX16
		for b.Loop() {
			v = benchM16.Blend(benchY16, v)
		}
		sinkU16x16 = v
	})
}

func BenchmarkF32x8Floor(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := benchX8
		for b.Loop() {
			v = v.Floor().Add(benchY8)
		}
		sinkF32x8 = v
	})
}

func BenchmarkF32x8Normalize(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := benchX8
		for b.Loop() {
			v = v.Normalize().Add(benchY8)
		}
		sinkF32x8 = v
	})
}

func BenchmarkF32x4RoundInt(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := F32x4(benchX8[:4])
		var r I32x4
		for b.Loop() {
			r = v.RoundInt()
			v[0] = float32(r[0])
		}
		sinkF32x4 = v
	})
}

func BenchmarkF32x16Floor(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := F32x16{benchX8, benchX8}
		for b.Loop() {
			v = v.Floor().Add(F32x16{benchY8, benchY8})
		}
		sinkF32x16 = v
	})
}

func BenchmarkF32x16Sqrt(b *testing.B) {
	b.Run(implName(), func(b *testing.B) {
		v := F32x16{benchX8, benchX8}
		for b.Loop() {
			v = v.Sqrt()
		}
		sinkF32x16 = v
	})
}
