package okage

import (
	"testing"

	"github.com/xo/okage/internal/wide"
)

// TestLowpGradientFused runs each float stage of lowp, which D33 writes as
// one loop on the lanes, and its old body in
// pipeline_lowp_gradient_ref_test.go on the same random states. It compares
// the channels bit for bit. The pairs r, g and b, a hold the bits of the
// floats x and y, as split writes them.
func TestLowpGradientFused(t *testing.T) {
	tests := []struct {
		name  string
		fused func(*lowpPipeline)
		ref   func(*lowpPipeline)
	}{
		{"seedShader", (*lowpPipeline).seedShader, refLowpGradSeedShader},
		{"transform", (*lowpPipeline).transform, refLowpGradTransform},
		{"padX1", (*lowpPipeline).padX1, refLowpGradPadX1},
		{"reflectX1", (*lowpPipeline).reflectX1, refLowpGradReflectX1},
		{"repeatX1", (*lowpPipeline).repeatX1, refLowpGradRepeatX1},
		{"gradient", (*lowpPipeline).gradient, refLowpGradGradient},
		{"evenlySpaced2StopGradient", (*lowpPipeline).evenlySpaced2StopGradient, refLowpGradEvenlySpaced2StopGradient},
		{"xyToRadius", (*lowpPipeline).xyToRadius, refLowpGradXyToRadius},
	}

	rounds := highpFusedRounds
	if testing.Short() {
		rounds /= 10
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fr := newFusedRand(uint64(len(test.name)) + 1000)
			for round := range rounds {
				s := fr.newFusedState()
				var base lowpPipeline
				x := wide.F32x16{fr.f32x8(), fr.f32x8()}
				y := wide.F32x16{fr.f32x8(), fr.f32x8()}
				refLowpGradSplit(&x, &base.r, &base.g)
				refLowpGradSplit(&y, &base.b, &base.a)
				for _, c := range []*wide.U16x16{&base.dr, &base.dg, &base.db, &base.da} {
					for i := range c {
						c[i] = uint16(fr.r.Uint32())
					}
				}

				run := func(f func(*lowpPipeline)) *lowpPipeline {
					ctx := s.ctx
					p := base
					p.functions = []func(*lowpPipeline){(*lowpPipeline).justReturn}
					p.ctx = &ctx
					p.tail = s.tail
					p.dx = s.dx
					p.dy = s.dy
					f(&p)
					return &p
				}
				pf := run(test.fused)
				pr := run(test.ref)

				names := [...]string{"r", "g", "b", "a", "dr", "dg", "db", "da"}
				got := [...]*wide.U16x16{&pf.r, &pf.g, &pf.b, &pf.a, &pf.dr, &pf.dg, &pf.db, &pf.da}
				want := [...]*wide.U16x16{&pr.r, &pr.g, &pr.b, &pr.a, &pr.dr, &pr.dg, &pr.db, &pr.da}
				for i := range got {
					if *got[i] != *want[i] && !sameNaNHalves(test.name, i, pf, pr) {
						t.Fatalf("round %d: %s = %v, want %v; x %v, y %v",
							round, names[i], *got[i], *want[i], x, y)
					}
				}
			}
		})
	}
}

// sameNaNHalves reports whether the channels of pf and pr that hold the
// floats of a stage that keeps a float, such as transform, differ only where
// both floats are NaN. See sameF32x8.
func sameNaNHalves(name string, channel int, pf, pr *lowpPipeline) bool {
	switch name {
	case "gradient", "evenlySpaced2StopGradient":
		return false
	}
	if channel >= 4 {
		return false
	}
	lo, hi := &pf.r, &pf.g
	wlo, whi := &pr.r, &pr.g
	if channel >= 2 {
		lo, hi = &pf.b, &pf.a
		wlo, whi = &pr.b, &pr.a
	}
	got := refLowpGradJoin(lo, hi)
	want := refLowpGradJoin(wlo, whi)
	for h := range got {
		if _, ok := sameF32x8(&got[h], &want[h]); !ok {
			return false
		}
	}
	return true
}
