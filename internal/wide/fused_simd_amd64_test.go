//go:build goexperiment.simd && !okage_scalar && !tinygo

package wide

import (
	"bytes"
	"math"
	"math/rand/v2"
	"testing"
)

// TestFusedRowSIMD runs the SIMD code and the scalar code of each fused
// program of D98 on random rows, and makes sure that both give the same
// bytes. TestFusedPrograms of the root package compares the fused programs
// with the stages.
func TestFusedRowSIMD(t *testing.T) {
	if !useSIMD {
		t.Skip("the CPU has no AVX2")
	}
	t.Cleanup(func() { useSIMD = true })
	float := func(rng *rand.Rand, scale float32) float32 {
		switch rng.IntN(40) {
		case 0:
			return float32(math.NaN())
		case 1:
			return float32(math.Inf(1 - 2*rng.IntN(2)))
		case 2:
			return (rng.Float32() - 0.5) * 1e30
		}
		return (rng.Float32()*2 - 0.5) * scale
	}
	rng := rand.New(rand.NewPCG(98, 1))
	for prog := FusedNone + 1; prog <= FusedLinearPad2Lowp; prog++ {
		for range 20000 {
			n := 1 + rng.IntN(70)
			px := make([]byte, 4*n)
			for i := range px {
				px[i] = byte(rng.Uint32())
			}
			mask := make([]byte, n)
			for i := range mask {
				if rng.IntN(3) > 0 {
					mask[i] = byte(rng.Uint32())
				}
			}
			c := &FusedCtx{
				RGBA:      [4]uint16{uint16(rng.IntN(256)), uint16(rng.IntN(256)), uint16(rng.IntN(256)), uint16(rng.IntN(256))},
				Coverage:  uint16(rng.IntN(256)),
				Transform: [3]float32{float(rng, 0.02), float(rng, 0.02), float(rng, 2)},
			}
			for i := range 4 {
				c.Factor[i] = float(rng, 2)
				c.Bias[i] = float(rng, 1)
			}
			x, y := rng.IntN(5000), rng.IntN(5000)
			want := bytes.Clone(px)
			useSIMD = false
			FusedRow(prog, c, want, mask, x, y, n)
			useSIMD = true
			FusedRow(prog, c, px, mask, x, y, n)
			if !bytes.Equal(px, want) {
				t.Fatalf("program %d: the SIMD code gives %v, the scalar code %v, ctx %+v", prog, px, want, c)
			}
		}
	}
}
