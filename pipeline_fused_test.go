package okage

import (
	"bytes"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/xo/okage/internal/wide"
)

// This file ports no upstream file. It tests the fused programs of D98.

// fusedCase is a program of stages that matches a fused program.
type fusedCase struct {
	name   string
	prog   wide.FusedProgram
	highp  bool
	stages []stage
}

// fusedCases lists a program of stages for each fused program, and the
// other programs of stages that match one.
var fusedCases = []fusedCase{
	{"solid over", wide.FusedSolidOverLowp, false, []stage{stageUniformColor, stageSourceOverRgba}},
	{"solid over with load and store", wide.FusedSolidOverLowp, false, []stage{stageUniformColor, stageLoadDestination, stageSourceOver, stageStore}},
	{"solid coverage over", wide.FusedSolidCoverageOverLowp, false, []stage{stageUniformColor, stageScale1Float, stageLoadDestination, stageSourceOver, stageStore}},
	{"solid mask over", wide.FusedSolidMaskOverLowp, false, []stage{stageUniformColor, stageMaskU8, stageLoadDestination, stageSourceOver, stageStore}},
	{"solid mask coverage over", wide.FusedSolidMaskCoverageOverLowp, false, []stage{stageUniformColor, stageMaskU8, stageScale1Float, stageLoadDestination, stageSourceOver, stageStore}},
	{"linear pad highp", wide.FusedLinearPad2Highp, true, []stage{stageSeedShader, stageTransform, stagePadX1, stageEvenlySpaced2StopGradient, stagePremultiply, stageSourceOverRgba}},
	{"linear pad highp identity", wide.FusedLinearPad2Highp, true, []stage{stageSeedShader, stagePadX1, stageEvenlySpaced2StopGradient, stagePremultiply, stageSourceOverRgba}},
	{"linear pad lowp", wide.FusedLinearPad2Lowp, false, []stage{stageSeedShader, stageTransform, stagePadX1, stageEvenlySpaced2StopGradient, stagePremultiply, stageSourceOverRgba}},
	{"linear pad lowp identity", wide.FusedLinearPad2Lowp, false, []stage{stageSeedShader, stagePadX1, stageEvenlySpaced2StopGradient, stagePremultiply, stageSourceOverRgba}},
}

// fusedFloat returns a random float for a context of a fused program:
// mostly a value near 0..1, and at times a large value, an infinity or
// NaN.
func fusedFloat(rng *rand.Rand, scale float32) float32 {
	switch rng.IntN(40) {
	case 0:
		return float32(math.NaN())
	case 1:
		return float32(math.Inf(1 - 2*rng.IntN(2)))
	case 2:
		return (rng.Float32() - 0.5) * 1e30
	case 3:
		return 0
	}
	return (rng.Float32()*2 - 0.5) * scale
}

// TestFusedPrograms runs each fused program and its stages on random
// pixmaps, rects, masks, colors, coverages and gradients, and makes sure
// that both give the same bytes. It runs the SIMD code of the fused
// programs in the SIMD build on amd64, and their scalar code otherwise.
func TestFusedPrograms(t *testing.T) {
	n := 3000
	if testing.Short() {
		n = 300
	}
	t.Cleanup(func() { useFused = true })
	for ci, tc := range fusedCases {
		t.Run(tc.name, func(t *testing.T) {
			rng := rand.New(rand.NewPCG(uint64(ci), 98))
			for iter := range n {
				w := 1 + rng.IntN(70)
				h := 1 + rng.IntN(3)
				pixmap, ok := NewPixmap(uint32(w), uint32(h))
				if !ok {
					t.Fatal("NewPixmap failed")
				}
				data := pixmap.DataMut()
				for i := range data {
					data[i] = byte(rng.Uint32())
				}
				x := rng.IntN(w)
				y := rng.IntN(h)
				rw := 1 + rng.IntN(w-x)
				rh := 1 + rng.IntN(h-y)
				// A hairline can give a rect one column past the right
				// side, which draws on the next row (see blitColumns).
				if y+rh < h && rng.IntN(8) == 0 {
					rw = w - x + 1
				}
				rect, ok := screenIntRectFromXYWH(uint32(x), uint32(y), uint32(rw), uint32(rh))
				if !ok {
					t.Fatal("screenIntRectFromXYWH failed")
				}

				mask := make([]byte, w*h+1)
				for i := range mask {
					if rng.IntN(3) > 0 {
						mask[i] = byte(rng.Uint32())
					}
				}
				if rng.IntN(4) == 0 {
					clear(mask)
				}
				mc := maskCtx{data: mask, realWidth: uint32(w)}

				b := newRasterPipelineBuilder()
				b.setForceHqPipeline(tc.highp)
				for _, s := range tc.stages {
					b.push(s)
				}
				for i := range b.ctx.uniformColor.rgba {
					b.ctx.uniformColor.rgba[i] = uint16(rng.IntN(256))
				}
				b.ctx.currentCoverage = rng.Float32()
				if rng.IntN(20) == 0 {
					b.ctx.currentCoverage = fusedFloat(rng, 2)
				}
				b.ctx.transform = Transform{
					SX: fusedFloat(rng, 0.02), KX: fusedFloat(rng, 0.02), TX: fusedFloat(rng, 2),
					KY: fusedFloat(rng, 1), SY: fusedFloat(rng, 1), TY: fusedFloat(rng, 1),
				}
				g := &b.ctx.evenlySpaced2StopGradient
				g.factor = newGradientColor(fusedFloat(rng, 2), fusedFloat(rng, 2), fusedFloat(rng, 2), fusedFloat(rng, 2))
				g.bias = newGradientColor(fusedFloat(rng, 1), fusedFloat(rng, 1), fusedFloat(rng, 1), fusedFloat(rng, 1))
				rp := b.compile()
				if rp.fused != tc.prog {
					t.Fatalf("the stages match the fused program %d, want %d", rp.fused, tc.prog)
				}

				want := bytes.Clone(data)
				wantPixmap, ok := PixmapFromVec(want, pixmap.Size())
				if !ok {
					t.Fatal("PixmapFromVec failed")
				}
				src := PixmapRef{}
				useFused = false
				wm := wantPixmap.AsMut()
				wsub := wm.asSubpixmap()
				rp.run(&rect, aaMaskCtx{}, mc, src, &wsub)
				useFused = true
				m := pixmap.AsMut()
				sub := m.asSubpixmap()
				rp.run(&rect, aaMaskCtx{}, mc, src, &sub)
				if !bytes.Equal(pixmap.Data(), wantPixmap.Data()) {
					t.Fatalf("case %d: the fused program gives other bytes than the stages, rect %v, ctx %+v", iter, rect, rp.fusedCtx)
				}
			}
		})
	}
}

// TestFusedMatch makes sure that a program of stages that differs from each
// fused program, or a color that does not fit in 8 bits, matches no fused
// program.
func TestFusedMatch(t *testing.T) {
	b := newRasterPipelineBuilder()
	b.push(stageUniformColor)
	b.push(stageSourceOverRgba)
	b.ctx.uniformColor.rgba = [4]uint16{256, 0, 0, 255}
	if rp := b.compile(); rp.fused != wide.FusedNone {
		t.Errorf("a color with a channel of 256 matches the fused program %d", rp.fused)
	}
	b = newRasterPipelineBuilder()
	b.setForceHqPipeline(true)
	b.push(stageUniformColor)
	b.push(stageSourceOverRgba)
	if rp := b.compile(); rp.fused != wide.FusedNone {
		t.Errorf("a highp solid fill matches the fused program %d", rp.fused)
	}
	b = newRasterPipelineBuilder()
	for _, s := range []stage{stageSeedShader, stageTransform, stageRepeatX1, stageEvenlySpaced2StopGradient, stagePremultiply, stageSourceOverRgba} {
		b.push(s)
	}
	if rp := b.compile(); rp.fused != wide.FusedNone {
		t.Errorf("a gradient with the spread mode repeat matches the fused program %d", rp.fused)
	}
}
