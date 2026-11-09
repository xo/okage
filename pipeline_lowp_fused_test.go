package okage

import (
	"math/rand/v2"
	"testing"

	"github.com/xo/okage/internal/wide"
)

// TestLowpFused checks that each stage of lowp that D33 writes as one loop on
// the lanes gives the same bits as the methods of the vector types, which
// pipeline_lowp_ref_test.go keeps. The channels hold random values in 0..255,
// as the pipeline holds them, and random values in 0..65535, which a stage
// must also wrap as upstream does.
func TestLowpFused(t *testing.T) {
	stages := []struct {
		name  string
		fused func(*lowpPipeline)
		ref   func(*lowpPipeline)
	}{
		{"sourceAtop", (*lowpPipeline).sourceAtop, refLowpSourceAtop},
		{"destinationAtop", (*lowpPipeline).destinationAtop, refLowpDestinationAtop},
		{"sourceIn", (*lowpPipeline).sourceIn, refLowpSourceIn},
		{"destinationIn", (*lowpPipeline).destinationIn, refLowpDestinationIn},
		{"sourceOut", (*lowpPipeline).sourceOut, refLowpSourceOut},
		{"destinationOut", (*lowpPipeline).destinationOut, refLowpDestinationOut},
		{"sourceOver", (*lowpPipeline).sourceOver, refLowpSourceOver},
		{"destinationOver", (*lowpPipeline).destinationOver, refLowpDestinationOver},
		{"modulate", (*lowpPipeline).modulate, refLowpModulate},
		{"multiply", (*lowpPipeline).multiply, refLowpMultiply},
		{"screen", (*lowpPipeline).screen, refLowpScreen},
		{"xor", (*lowpPipeline).xor, refLowpXor},
		{"plus", (*lowpPipeline).plus, refLowpPlus},
		{"darken", (*lowpPipeline).darken, refLowpDarken},
		{"lighten", (*lowpPipeline).lighten, refLowpLighten},
		{"exclusion", (*lowpPipeline).exclusion, refLowpExclusion},
		{"difference", (*lowpPipeline).difference, refLowpDifference},
		{"hardLight", (*lowpPipeline).hardLight, refLowpHardLight},
		{"overlay", (*lowpPipeline).overlay, refLowpOverlay},
		{"premultiply", (*lowpPipeline).premultiply, refLowpPremultiply},
		{"scale1Float", (*lowpPipeline).scale1Float, refLowpScale1Float},
		{"lerp1Float", (*lowpPipeline).lerp1Float, refLowpLerp1Float},
	}
	r := rand.New(rand.NewPCG(1, 33))
	for _, st := range stages {
		for n := range 20000 {
			limit := 256
			if n%4 == 3 {
				limit = 65536
			}
			ctx := &context{}
			ctx.currentCoverage = r.Float32()
			var p lowpPipeline
			p.ctx = ctx
			for _, v := range []*wide.U16x16{&p.r, &p.g, &p.b, &p.a, &p.dr, &p.dg, &p.db, &p.da} {
				for i := range v {
					v[i] = uint16(r.IntN(limit))
				}
			}
			q := p
			p.functions = []func(*lowpPipeline){(*lowpPipeline).justReturn}
			st.fused(&p)
			st.ref(&q)
			if p.r != q.r || p.g != q.g || p.b != q.b || p.a != q.a {
				t.Fatalf("%s: case %d differs\nfused %v %v %v %v\nref   %v %v %v %v", st.name, n, p.r, p.g, p.b, p.a, q.r, q.g, q.b, q.a)
			}
		}
	}
}
