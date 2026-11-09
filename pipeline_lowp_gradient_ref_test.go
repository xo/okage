package okage

import (
	"math"

	"github.com/xo/okage/internal/wide"
)

// This file holds the bodies of the float stages and helpers of
// pipeline_lowp.go before they were written as one loop on the lanes (D33).
// TestLowpGradientFused compares each stage with its old body, bit for bit. A
// stage here does not call the next stage.

// refLowpGradSeedShader is the body of seedShader before D33.
func refLowpGradSeedShader(p *lowpPipeline) {
	iota := wide.F32x16{
		wide.F32x8{0.5, 1.5, 2.5, 3.5, 4.5, 5.5, 6.5, 7.5},
		wide.F32x8{8.5, 9.5, 10.5, 11.5, 12.5, 13.5, 14.5, 15.5},
	}

	x := wide.SplatF32x16(float32(p.dx)).Add(iota)
	y := wide.SplatF32x16(float32(p.dy) + 0.5)
	refLowpGradSplit(&x, &p.r, &p.g)
	refLowpGradSplit(&y, &p.b, &p.a)
}

// refLowpGradTransform is the body of transform before D33.
func refLowpGradTransform(p *lowpPipeline) {
	ts := &p.ctx.transform

	x := refLowpGradJoin(&p.r, &p.g)
	y := refLowpGradJoin(&p.b, &p.a)

	nx := refLowpGradMad(x, wide.SplatF32x16(ts.SX), refLowpGradMad(y, wide.SplatF32x16(ts.KX), wide.SplatF32x16(ts.TX)))
	ny := refLowpGradMad(x, wide.SplatF32x16(ts.KY), refLowpGradMad(y, wide.SplatF32x16(ts.SY), wide.SplatF32x16(ts.TY)))

	refLowpGradSplit(&nx, &p.r, &p.g)
	refLowpGradSplit(&ny, &p.b, &p.a)
}

// refLowpGradPadX1 is the body of padX1 before D33.
func refLowpGradPadX1(p *lowpPipeline) {
	x := refLowpGradJoin(&p.r, &p.g)
	x = x.Normalize()
	refLowpGradSplit(&x, &p.r, &p.g)
}

// refLowpGradReflectX1 is the body of reflectX1 before D33.
func refLowpGradReflectX1(p *lowpPipeline) {
	x := refLowpGradJoin(&p.r, &p.g)
	one := wide.SplatF32x16(1.0)
	t := x.Sub(one).Mul(wide.SplatF32x16(0.5)).Floor()
	x = x.Sub(one).
		Sub(t.Add(t)). // two(t)
		Sub(one).
		Abs().Normalize()
	refLowpGradSplit(&x, &p.r, &p.g)
}

// refLowpGradRepeatX1 is the body of repeatX1 before D33.
func refLowpGradRepeatX1(p *lowpPipeline) {
	x := refLowpGradJoin(&p.r, &p.g)
	x = x.Sub(x.Floor()).Normalize()
	refLowpGradSplit(&x, &p.r, &p.g)
}

// refLowpGradGradient is the body of gradient before D33.
func refLowpGradGradient(p *lowpPipeline) {
	ctx := &p.ctx.gradient

	t := refLowpGradJoin(&p.r, &p.g)
	var idx wide.U16x16
	for i := 1; i < ctx.len; i++ {
		tt := ctx.tValues[i].Get()
		t0 := t[0]
		t1 := t[1]
		idx[0] += b2u16(t0[0] >= tt)
		idx[1] += b2u16(t0[1] >= tt)
		idx[2] += b2u16(t0[2] >= tt)
		idx[3] += b2u16(t0[3] >= tt)
		idx[4] += b2u16(t0[4] >= tt)
		idx[5] += b2u16(t0[5] >= tt)
		idx[6] += b2u16(t0[6] >= tt)
		idx[7] += b2u16(t0[7] >= tt)
		idx[8] += b2u16(t1[0] >= tt)
		idx[9] += b2u16(t1[1] >= tt)
		idx[10] += b2u16(t1[2] >= tt)
		idx[11] += b2u16(t1[3] >= tt)
		idx[12] += b2u16(t1[4] >= tt)
		idx[13] += b2u16(t1[5] >= tt)
		idx[14] += b2u16(t1[6] >= tt)
		idx[15] += b2u16(t1[7] >= tt)
	}
	refLowpGradGradientLookup(ctx, &idx, t, &p.r, &p.g, &p.b, &p.a)
}

// refLowpGradEvenlySpaced2StopGradient is the body of
// evenlySpaced2StopGradient before D33.
func refLowpGradEvenlySpaced2StopGradient(p *lowpPipeline) {
	ctx := &p.ctx.evenlySpaced2StopGradient

	t := refLowpGradJoin(&p.r, &p.g)
	refLowpGradRoundF32ToU16(
		refLowpGradMad(t, wide.SplatF32x16(ctx.factor.r), wide.SplatF32x16(ctx.bias.r)),
		refLowpGradMad(t, wide.SplatF32x16(ctx.factor.g), wide.SplatF32x16(ctx.bias.g)),
		refLowpGradMad(t, wide.SplatF32x16(ctx.factor.b), wide.SplatF32x16(ctx.bias.b)),
		refLowpGradMad(t, wide.SplatF32x16(ctx.factor.a), wide.SplatF32x16(ctx.bias.a)),
		&p.r, &p.g, &p.b, &p.a,
	)
}

// refLowpGradXyToRadius is the body of xyToRadius before D33.
func refLowpGradXyToRadius(p *lowpPipeline) {
	x := refLowpGradJoin(&p.r, &p.g)
	y := refLowpGradJoin(&p.b, &p.a)
	x = x.Mul(x).Add(y.Mul(y)).Sqrt()
	refLowpGradSplit(&x, &p.r, &p.g)
	refLowpGradSplit(&y, &p.b, &p.a)
}

// refLowpGradGradientLookup is the body of gradientLookupLowp before D33.
func refLowpGradGradientLookup(
	ctx *gradientCtx, idx *wide.U16x16, t wide.F32x16,
	r, g, b, a *wide.U16x16,
) {
	var fr, fg, fb, fa, br, bg, bb, ba wide.F32x16
	for i := range lowpStageWidth {
		f := &ctx.factors[idx[i]]
		fr[i/8][i%8] = f.r
		fg[i/8][i%8] = f.g
		fb[i/8][i%8] = f.b
		fa[i/8][i%8] = f.a
	}
	for i := range lowpStageWidth {
		c := &ctx.biases[idx[i]]
		br[i/8][i%8] = c.r
		bg[i/8][i%8] = c.g
		bb[i/8][i%8] = c.b
		ba[i/8][i%8] = c.a
	}

	refLowpGradRoundF32ToU16(
		refLowpGradMad(t, fr, br),
		refLowpGradMad(t, fg, bg),
		refLowpGradMad(t, fb, bb),
		refLowpGradMad(t, fa, ba),
		r, g, b, a,
	)
}

// refLowpGradRoundF32ToU16 is the body of roundF32ToU16 before D33.
func refLowpGradRoundF32ToU16(
	rf, gf, bf, af wide.F32x16,
	r, g, b, a *wide.U16x16,
) {
	c255 := wide.SplatF32x16(255.0)
	half := wide.SplatF32x16(0.5)
	rf = rf.Normalize().Mul(c255).Add(half)
	gf = gf.Normalize().Mul(c255).Add(half)
	bf = bf.Normalize().Mul(c255).Add(half)
	af = af.Mul(c255).Add(half)

	rf.SaveToU16x16(r)
	gf.SaveToU16x16(g)
	bf.SaveToU16x16(b)
	af.SaveToU16x16(a)
}

// refLowpGradSplit is the body of split before D33.
func refLowpGradSplit(v *wide.F32x16, lo, hi *wide.U16x16) {
	for i := range 8 {
		bits := math.Float32bits(v[0][i])
		lo[2*i] = uint16(bits)
		lo[2*i+1] = uint16(bits >> 16)
	}
	for i := range 8 {
		bits := math.Float32bits(v[1][i])
		hi[2*i] = uint16(bits)
		hi[2*i+1] = uint16(bits >> 16)
	}
}

// refLowpGradJoin is the body of join before D33.
func refLowpGradJoin(lo, hi *wide.U16x16) wide.F32x16 {
	var v wide.F32x16
	for i := range 8 {
		v[0][i] = math.Float32frombits(uint32(lo[2*i]) | uint32(lo[2*i+1])<<16)
	}
	for i := range 8 {
		v[1][i] = math.Float32frombits(uint32(hi[2*i]) | uint32(hi[2*i+1])<<16)
	}
	return v
}

// refLowpGradMad is the body of madLowp before D33.
func refLowpGradMad(f, m, a wide.F32x16) wide.F32x16 {
	return f.Mul(m).Add(a)
}
