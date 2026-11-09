package okage

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// highpFusedRounds is the number of random states on which TestHighpFused
// runs each stage.
const highpFusedRounds = 10000

// fusedRand gives the random values of TestHighpFused and of
// TestLowpGradientFused.
type fusedRand struct {
	r *rand.Rand
}

// newFusedRand gives a fusedRand with a fixed seed, so that a failure
// repeats.
func newFusedRand(seed uint64) *fusedRand {
	return &fusedRand{r: rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))}
}

// f32 gives a random float32. Most values are in [0, 1], and the others are
// out of that range, near a rounding boundary, 0, -0, denormal, an infinity
// or NaN.
func (fr *fusedRand) f32() float32 {
	r := fr.r
	switch n := r.IntN(100); {
	case n < 30:
		return r.Float32()
	case n < 40:
		special := [...]float32{0, float32(math.Copysign(0, -1)), 1, 0.5, 0.25, 2, -1, 255, 1.0 / 255}
		return special[r.IntN(len(special))]
	case n < 50:
		// k/255, a value that a pixel gives, and its neighbors.
		v := float32(r.IntN(256)) / 255
		return nudge(v, r.IntN(5)-2)
	case n < 58:
		// A value whose product with 255 is near a half.
		v := (float32(r.IntN(256)) + 0.5) / 255
		return nudge(v, r.IntN(5)-2)
	case n < 70:
		return r.Float32()*5 - 2
	case n < 78:
		v := float32(math.Pow(10, r.Float64()*12-3))
		if r.IntN(2) == 0 {
			v = -v
		}
		return v
	case n < 84:
		special := [...]float32{
			float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1)),
			math.Float32frombits(0x7fc00001), math.Float32frombits(0xffc00000),
		}
		return special[r.IntN(len(special))]
	case n < 88:
		v := math.Float32frombits(uint32(r.IntN(1 << 23)))
		if r.IntN(2) == 0 {
			v = -v
		}
		return v
	case n < 94:
		// An integer or a half, as a coordinate of a pixel.
		return float32(r.IntN(64)-16) + float32(r.IntN(2))*0.5
	}
	return math.Float32frombits(r.Uint32())
}

// nudge moves v by n ulps.
func nudge(v float32, n int) float32 {
	for ; n > 0; n-- {
		v = math.Nextafter32(v, float32(math.Inf(1)))
	}
	for ; n < 0; n++ {
		v = math.Nextafter32(v, float32(math.Inf(-1)))
	}
	return v
}

// f32x8 gives a random F32x8.
func (fr *fusedRand) f32x8() wide.F32x8 {
	var v wide.F32x8
	for i := range v {
		v[i] = fr.f32()
	}
	return v
}

// mix copies some lanes of each vector of vs from the same lane of another
// one, so that the branches of a stage that compare two channels run.
func (fr *fusedRand) mix(vs []*wide.F32x8) {
	for _, v := range vs {
		for i := range v {
			if fr.r.IntN(5) == 0 {
				v[i] = vs[fr.r.IntN(len(vs))][i]
			}
		}
	}
}

// fusedState is the random state of one round of TestHighpFused: a context,
// a source pixmap, a destination pixmap and the masks.
type fusedState struct {
	ctx    context
	src    []byte
	srcW   uint32
	srcH   uint32
	dst    []byte
	mask   []byte
	aaMask aaMaskCtx
	dx     int
	dy     int
	tail   int
}

// fusedDstWidth is the width of the destination pixmap of TestHighpFused.
// It is two stages wide, so that a stage can start at any x of the first.
const fusedDstWidth = 2 * lowpStageWidth

// newFusedState gives a random fusedState.
func (fr *fusedRand) newFusedState() *fusedState {
	r := fr.r
	s := &fusedState{}
	c := &s.ctx

	c.currentCoverage = fr.f32()
	c.uniformColor = uniformColorCtx{r: fr.f32(), g: fr.f32(), b: fr.f32(), a: fr.f32()}
	for i := range c.uniformColor.rgba {
		c.uniformColor.rgba[i] = uint16(r.IntN(256))
	}
	c.evenlySpaced2StopGradient = evenlySpaced2StopGradientCtx{
		factor: gradientColor{fr.f32(), fr.f32(), fr.f32(), fr.f32()},
		bias:   gradientColor{fr.f32(), fr.f32(), fr.f32(), fr.f32()},
	}

	n := 2 + r.IntN(7)
	c.gradient.len = n
	c.gradient.factors = make([]gradientColor, 16)
	c.gradient.biases = make([]gradientColor, 16)
	for i := range n {
		c.gradient.factors[i] = gradientColor{fr.f32(), fr.f32(), fr.f32(), fr.f32()}
		c.gradient.biases[i] = gradientColor{fr.f32(), fr.f32(), fr.f32(), fr.f32()}
	}
	c.gradient.tValues = make([]path.NormalizedF32, n)
	for i := 1; i < n; i++ {
		t := float32(c.gradient.tValues[i-1]) + r.Float32()*(1-float32(c.gradient.tValues[i-1]))
		c.gradient.tValues[i] = path.NormalizedF32Clamped(t)
	}

	for i := range c.twoPointConicalGradient.mask {
		if r.IntN(2) == 0 {
			c.twoPointConicalGradient.mask[i] = ^uint32(0)
		}
	}
	c.twoPointConicalGradient.p0 = fr.f32()
	c.twoPointConicalGradient.p1 = fr.f32()

	s.srcW = uint32(1 + r.IntN(9))
	s.srcH = uint32(1 + r.IntN(9))
	s.src = make([]byte, s.srcW*s.srcH*BytesPerPixel)
	for i := range s.src {
		s.src[i] = uint8(r.IntN(256))
	}

	c.sampler = samplerCtx{
		spreadMode: SpreadMode(r.IntN(3)),
		invWidth:   1 / float32(s.srcW),
		invHeight:  1 / float32(s.srcH),
	}
	c.limitX = tileCtx{scale: float32(s.srcW), invScale: 1 / float32(s.srcW)}
	c.limitY = tileCtx{scale: float32(s.srcH), invScale: 1 / float32(s.srcH)}
	if r.IntN(4) == 0 {
		v := fr.f32()
		c.limitX = tileCtx{scale: v, invScale: 1 / v}
		v = fr.f32()
		c.limitY = tileCtx{scale: v, invScale: 1 / v}
	}

	c.transform = path.Transform{
		SX: fr.f32(), KY: fr.f32(), KX: fr.f32(), SY: fr.f32(), TX: fr.f32(), TY: fr.f32(),
	}

	s.dst = make([]byte, fusedDstWidth*2*BytesPerPixel)
	for i := range s.dst {
		s.dst[i] = uint8(r.IntN(256))
	}
	s.mask = make([]byte, fusedDstWidth*2)
	for i := range s.mask {
		s.mask[i] = uint8(r.IntN(256))
		if r.IntN(3) == 0 {
			s.mask[i] = 0
		}
	}

	s.dx = r.IntN(lowpStageWidth + 1)
	s.dy = r.IntN(2)
	s.tail = 1 + r.IntN(highpStageWidth)

	s.aaMask.pixels = [2]uint8{uint8(r.IntN(256)), uint8(r.IntN(256))}
	s.aaMask.stride = 0
	switch r.IntN(3) {
	case 0:
		s.aaMask.shift = uint(s.dx)
		s.tail = 1
	case 1:
		s.aaMask.shift = uint(s.dx)
		s.tail = 2
	default:
		s.aaMask.shift = uint(s.dx) - 1
		s.tail = 1
	}
	if r.IntN(2) == 0 {
		s.tail = 1 + r.IntN(highpStageWidth)
	}
	return s
}

// highpPipeline gives a highpPipeline on a copy of the state, with the
// channels of base. Its only next stage is justReturn.
func (s *fusedState) highpPipeline(t *testing.T, base *highpPipeline) (*highpPipeline, *context, []byte) {
	t.Helper()
	ctx := s.ctx
	dst := append([]byte(nil), s.dst...)
	src, ok := PixmapRefFromBytes(s.src, s.srcW, s.srcH)
	if !ok {
		t.Fatalf("PixmapRefFromBytes(%d, %d) failed", s.srcW, s.srcH)
	}
	size, ok := path.IntSizeFromWH(fusedDstWidth, 2)
	if !ok {
		t.Fatal("IntSizeFromWH failed")
	}
	p := *base
	p.index = 0
	p.functions = []func(*highpPipeline){(*highpPipeline).justReturn}
	p.pixmapSrc = src
	p.pixmapDst = &subPixmapMut{data: dst, size: size, realWidth: fusedDstWidth}
	p.ctx = &ctx
	p.maskCtx = maskCtx{data: s.mask, realWidth: fusedDstWidth}
	p.aaMaskCtx = s.aaMask
	p.tail = s.tail
	p.dx = s.dx
	p.dy = s.dy
	return &p, &ctx, dst
}

// sameF32x8 reports the first lane where a and b have different bits. Two
// NaN lanes are the same, whatever their bits, as in the tests of
// internal/wide: the Go compiler can swap the operands of an add or a
// multiply, and so give the NaN of the other operand. A NaN gives 0 in
// unnorm, so the bits of a NaN do not reach a pixel.
func sameF32x8(a, b *wide.F32x8) (int, bool) {
	for i := range a {
		if a[i] != a[i] && b[i] != b[i] {
			continue
		}
		if math.Float32bits(a[i]) != math.Float32bits(b[i]) {
			return i, false
		}
	}
	return 0, true
}

// TestHighpFused runs each stage of highp, which D33 writes as one loop on
// the lanes, and its old body in pipeline_highp_ref_test.go on the same
// random states. It compares the channels, the context and the destination
// pixmap bit for bit.
func TestHighpFused(t *testing.T) {
	tests := []struct {
		name  string
		fused func(*highpPipeline)
		ref   func(*highpPipeline)
	}{
		{"moveSourceToDestination", (*highpPipeline).moveSourceToDestination, refHighpMoveSourceToDestination},
		{"premultiply", (*highpPipeline).premultiply, refHighpPremultiply},
		{"moveDestinationToSource", (*highpPipeline).moveDestinationToSource, refHighpMoveDestinationToSource},
		{"clamp0", (*highpPipeline).clamp0, refHighpClamp0},
		{"clampA", (*highpPipeline).clampA, refHighpClampA},
		{"uniformColor", (*highpPipeline).uniformColor, refHighpUniformColor},
		{"seedShader", (*highpPipeline).seedShader, refHighpSeedShader},
		{"loadDst", (*highpPipeline).loadDst, refHighpLoadDst},
		{"loadDstTail", (*highpPipeline).loadDstTail, refHighpLoadDstTail},
		{"store", (*highpPipeline).store, refHighpStore},
		{"storeTail", (*highpPipeline).storeTail, refHighpStoreTail},
		{"gather", (*highpPipeline).gather, refHighpGather},
		{"maskU8", (*highpPipeline).maskU8, refHighpMaskU8},
		{"scaleU8", (*highpPipeline).scaleU8, refHighpScaleU8},
		{"lerpU8", (*highpPipeline).lerpU8, refHighpLerpU8},
		{"scale1Float", (*highpPipeline).scale1Float, refHighpScale1Float},
		{"lerp1Float", (*highpPipeline).lerp1Float, refHighpLerp1Float},
		{"clear", (*highpPipeline).clear, refHighpClear},
		{"sourceAtop", (*highpPipeline).sourceAtop, refHighpSourceAtop},
		{"destinationAtop", (*highpPipeline).destinationAtop, refHighpDestinationAtop},
		{"sourceIn", (*highpPipeline).sourceIn, refHighpSourceIn},
		{"destinationIn", (*highpPipeline).destinationIn, refHighpDestinationIn},
		{"sourceOut", (*highpPipeline).sourceOut, refHighpSourceOut},
		{"destinationOut", (*highpPipeline).destinationOut, refHighpDestinationOut},
		{"sourceOver", (*highpPipeline).sourceOver, refHighpSourceOver},
		{"destinationOver", (*highpPipeline).destinationOver, refHighpDestinationOver},
		{"modulate", (*highpPipeline).modulate, refHighpModulate},
		{"multiply", (*highpPipeline).multiply, refHighpMultiply},
		{"screen", (*highpPipeline).screen, refHighpScreen},
		{"xor", (*highpPipeline).xor, refHighpXor},
		{"plus", (*highpPipeline).plus, refHighpPlus},
		{"darken", (*highpPipeline).darken, refHighpDarken},
		{"lighten", (*highpPipeline).lighten, refHighpLighten},
		{"difference", (*highpPipeline).difference, refHighpDifference},
		{"exclusion", (*highpPipeline).exclusion, refHighpExclusion},
		{"colorBurn", (*highpPipeline).colorBurn, refHighpColorBurn},
		{"colorDodge", (*highpPipeline).colorDodge, refHighpColorDodge},
		{"hardLight", (*highpPipeline).hardLight, refHighpHardLight},
		{"overlay", (*highpPipeline).overlay, refHighpOverlay},
		{"softLight", (*highpPipeline).softLight, refHighpSoftLight},
		{"hue", (*highpPipeline).hue, refHighpHue},
		{"saturation", (*highpPipeline).saturation, refHighpSaturation},
		{"color", (*highpPipeline).color, refHighpColor},
		{"luminosity", (*highpPipeline).luminosity, refHighpLuminosity},
		{"sourceOverRgba", (*highpPipeline).sourceOverRgba, refHighpSourceOverRgba},
		{"sourceOverRgbaTail", (*highpPipeline).sourceOverRgbaTail, refHighpSourceOverRgbaTail},
		{"transform", (*highpPipeline).transform, refHighpTransform},
		{"reflect", (*highpPipeline).reflect, refHighpReflect},
		{"repeat", (*highpPipeline).repeat, refHighpRepeat},
		{"bilinear", (*highpPipeline).bilinear, refHighpBilinear},
		{"bicubic", (*highpPipeline).bicubic, refHighpBicubic},
		{"padX1", (*highpPipeline).padX1, refHighpPadX1},
		{"reflectX1", (*highpPipeline).reflectX1, refHighpReflectX1},
		{"repeatX1", (*highpPipeline).repeatX1, refHighpRepeatX1},
		{"gradient", (*highpPipeline).gradient, refHighpGradient},
		{"evenlySpaced2StopGradient", (*highpPipeline).evenlySpaced2StopGradient, refHighpEvenlySpaced2StopGradient},
		{"xyToUnitAngle", (*highpPipeline).xyToUnitAngle, refHighpXyToUnitAngle},
		{"xyToRadius", (*highpPipeline).xyToRadius, refHighpXyToRadius},
		{"xyTo2PtConicalFocalOnCircle", (*highpPipeline).xyTo2PtConicalFocalOnCircle, refHighpXyTo2PtConicalFocalOnCircle},
		{"xyTo2PtConicalWellBehaved", (*highpPipeline).xyTo2PtConicalWellBehaved, refHighpXyTo2PtConicalWellBehaved},
		{"xyTo2PtConicalGreater", (*highpPipeline).xyTo2PtConicalGreater, refHighpXyTo2PtConicalGreater},
		{"xyTo2PtConicalSmaller", (*highpPipeline).xyTo2PtConicalSmaller, refHighpXyTo2PtConicalSmaller},
		{"xyTo2PtConicalStrip", (*highpPipeline).xyTo2PtConicalStrip, refHighpXyTo2PtConicalStrip},
		{"mask2PtConicalNan", (*highpPipeline).mask2PtConicalNan, refHighpMask2PtConicalNan},
		{"mask2PtConicalDegenerates", (*highpPipeline).mask2PtConicalDegenerates, refHighpMask2PtConicalDegenerates},
		{"applyVectorMask", (*highpPipeline).applyVectorMask, refHighpApplyVectorMask},
		{"alter2PtConicalCompensateFocal", (*highpPipeline).alter2PtConicalCompensateFocal, refHighpAlter2PtConicalCompensateFocal},
		{"alter2PtConicalUnswap", (*highpPipeline).alter2PtConicalUnswap, refHighpAlter2PtConicalUnswap},
		{"negateX", (*highpPipeline).negateX, refHighpNegateX},
		{"applyConcentricScaleBias", (*highpPipeline).applyConcentricScaleBias, refHighpApplyConcentricScaleBias},
		{"gammaExpand2", (*highpPipeline).gammaExpand2, refHighpGammaExpand2},
		{"gammaExpandDst2", (*highpPipeline).gammaExpandDst2, refHighpGammaExpandDst2},
		{"gammaCompress2", (*highpPipeline).gammaCompress2, refHighpGammaCompress2},
		{"gammaExpand22", (*highpPipeline).gammaExpand22, refHighpGammaExpand22},
		{"gammaExpandDst22", (*highpPipeline).gammaExpandDst22, refHighpGammaExpandDst22},
		{"gammaCompress22", (*highpPipeline).gammaCompress22, refHighpGammaCompress22},
		{"gammaExpandSrgb", (*highpPipeline).gammaExpandSrgb, refHighpGammaExpandSrgb},
		{"gammaExpandDstSrgb", (*highpPipeline).gammaExpandDstSrgb, refHighpGammaExpandDstSrgb},
		{"gammaCompressSrgb", (*highpPipeline).gammaCompressSrgb, refHighpGammaCompressSrgb},
	}

	rounds := highpFusedRounds
	if testing.Short() {
		rounds /= 10
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fr := newFusedRand(uint64(len(test.name)))
			for round := range rounds {
				s := fr.newFusedState()
				var base highpPipeline
				channels := []*wide.F32x8{&base.r, &base.g, &base.b, &base.a, &base.dr, &base.dg, &base.db, &base.da}
				for _, c := range channels {
					*c = fr.f32x8()
				}
				fr.mix(channels)

				pf, ctxF, dstF := s.highpPipeline(t, &base)
				pr, ctxR, dstR := s.highpPipeline(t, &base)
				test.fused(pf)
				test.ref(pr)

				names := [...]string{"r", "g", "b", "a", "dr", "dg", "db", "da"}
				got := [...]*wide.F32x8{&pf.r, &pf.g, &pf.b, &pf.a, &pf.dr, &pf.dg, &pf.db, &pf.da}
				want := [...]*wide.F32x8{&pr.r, &pr.g, &pr.b, &pr.a, &pr.dr, &pr.dg, &pr.db, &pr.da}
				for i := range got {
					if lane, ok := sameF32x8(got[i], want[i]); !ok {
						t.Fatalf("round %d: %s[%d] = %v (%#08x), want %v (%#08x); input %+v",
							round, names[i], lane, got[i][lane], math.Float32bits(got[i][lane]),
							want[i][lane], math.Float32bits(want[i][lane]), base)
					}
				}
				if ctxF.twoPointConicalGradient.mask != ctxR.twoPointConicalGradient.mask {
					t.Fatalf("round %d: mask = %v, want %v", round,
						ctxF.twoPointConicalGradient.mask, ctxR.twoPointConicalGradient.mask)
				}
				for i := range dstF {
					if dstF[i] != dstR[i] {
						t.Fatalf("round %d: dst[%d] = %d, want %d", round, i, dstF[i], dstR[i])
					}
				}
			}
		})
	}
}
