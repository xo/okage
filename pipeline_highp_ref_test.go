package okage

import (
	"math"

	"github.com/xo/okage/internal/wide"
)

// This file holds the bodies of the stages and helpers of pipeline_highp.go
// before they were written as one loop on the lanes (D33). TestHighpFused
// compares each stage with its old body, bit for bit. A stage here does not
// call the next stage.

var _ = math.Float32bits

// refHighpMoveSourceToDestination is the body of moveSourceToDestination before D33.
func refHighpMoveSourceToDestination(p *highpPipeline) {
	p.dr = p.r
	p.dg = p.g
	p.db = p.b
	p.da = p.a
}

// refHighpPremultiply is the body of premultiply before D33.
func refHighpPremultiply(p *highpPipeline) {
	p.r = p.r.Mul(p.a)
	p.g = p.g.Mul(p.a)
	p.b = p.b.Mul(p.a)
}

// refHighpMoveDestinationToSource is the body of moveDestinationToSource before D33.
func refHighpMoveDestinationToSource(p *highpPipeline) {
	p.r = p.dr
	p.g = p.dg
	p.b = p.db
	p.a = p.da
}

// refHighpClamp0 is the body of clamp0 before D33.
func refHighpClamp0(p *highpPipeline) {
	p.r = p.r.Max(wide.F32x8{})
	p.g = p.g.Max(wide.F32x8{})
	p.b = p.b.Max(wide.F32x8{})
	p.a = p.a.Max(wide.F32x8{})
}

// refHighpClampA is the body of clampA before D33.
func refHighpClampA(p *highpPipeline) {
	p.r = p.r.Min(wide.SplatF32x8(1.0))
	p.g = p.g.Min(wide.SplatF32x8(1.0))
	p.b = p.b.Min(wide.SplatF32x8(1.0))
	p.a = p.a.Min(wide.SplatF32x8(1.0))
}

// refHighpUniformColor is the body of uniformColor before D33.
func refHighpUniformColor(p *highpPipeline) {
	ctx := &p.ctx.uniformColor
	p.r = wide.SplatF32x8(ctx.r)
	p.g = wide.SplatF32x8(ctx.g)
	p.b = wide.SplatF32x8(ctx.b)
	p.a = wide.SplatF32x8(ctx.a)
}

// refHighpSeedShader is the body of seedShader before D33.
func refHighpSeedShader(p *highpPipeline) {
	iota := wide.F32x8{0.5, 1.5, 2.5, 3.5, 4.5, 5.5, 6.5, 7.5}

	p.r = wide.SplatF32x8(float32(p.dx)).Add(iota)
	p.g = wide.SplatF32x8(float32(p.dy) + 0.5)
	p.b = wide.SplatF32x8(1.0)
	p.a = wide.F32x8{}

	p.dr = wide.F32x8{}
	p.dg = wide.F32x8{}
	p.db = wide.F32x8{}
	p.da = wide.F32x8{}
}

// refHighpLoadDst is the body of loadDst before D33.
func refHighpLoadDst(p *highpPipeline) {
	refHighpLoad8888Highp(p.pixmapDst.slice4AtXY(p.dx, p.dy), &p.dr, &p.dg, &p.db, &p.da)
}

// refHighpLoadDstTail is the body of loadDstTail before D33.
func refHighpLoadDstTail(p *highpPipeline) {
	refHighpLoad8888TailHighp(p.tail, p.pixmapDst.sliceAtXY(p.dx, p.dy), &p.dr, &p.dg, &p.db, &p.da)
}

// refHighpStore is the body of store before D33.
func refHighpStore(p *highpPipeline) {
	refHighpStore8888Highp(&p.r, &p.g, &p.b, &p.a, p.pixmapDst.slice4AtXY(p.dx, p.dy))
}

// refHighpStoreTail is the body of storeTail before D33.
func refHighpStoreTail(p *highpPipeline) {
	refHighpStore8888TailHighp(&p.r, &p.g, &p.b, &p.a, p.tail, p.pixmapDst.sliceAtXY(p.dx, p.dy))
}

// refHighpGather is the body of gather before D33.
func refHighpGather(p *highpPipeline) {
	ix := refHighpGatherIx(p.pixmapSrc, p.r, p.g)
	pixels := p.pixmapSrc.gather(ix)
	refHighpLoad8888Highp(&pixels, &p.r, &p.g, &p.b, &p.a)
}

// refHighpGatherIx is the body of gatherIx before D33.
func refHighpGatherIx(pixmap PixmapRef, x, y wide.F32x8) wide.U32x8 {
	// Exclusive -> inclusive.
	w := ulpSub(float32(pixmap.Width()))
	h := ulpSub(float32(pixmap.Height()))
	x = x.Max(wide.F32x8{}).Min(wide.SplatF32x8(w))
	y = y.Max(wide.F32x8{}).Min(wide.SplatF32x8(h))

	return y.TruncInt().Mul(wide.SplatI32x8(int32(pixmap.Width()))).Add(x.TruncInt()).ToU32x8Bitcast()
}

// refHighpMaskU8 is the body of maskU8 before D33.
func refHighpMaskU8(p *highpPipeline) {
	offset := p.maskCtx.offset(p.dx, p.dy)
	var c wide.F32x8
	for i := range p.tail {
		c[i] = float32(p.maskCtx.data[offset+i])
	}
	c = c.Div(wide.SplatF32x8(255.0))

	if c.Equal(wide.F32x8{}) {
		return
	}

	p.r = p.r.Mul(c)
	p.g = p.g.Mul(c)
	p.b = p.b.Mul(c)
	p.a = p.a.Mul(c)
}

// refHighpScaleU8 is the body of scaleU8 before D33.
func refHighpScaleU8(p *highpPipeline) {
	// Load u8xTail and cast it to f32x8.
	data := p.aaMaskCtx.copyAtXY(p.dx, p.dy, p.tail)
	c := wide.F32x8{float32(data[0]), float32(data[1]), 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}
	c = c.Div(wide.SplatF32x8(255.0))

	p.r = p.r.Mul(c)
	p.g = p.g.Mul(c)
	p.b = p.b.Mul(c)
	p.a = p.a.Mul(c)
}

// refHighpLerpU8 is the body of lerpU8 before D33.
func refHighpLerpU8(p *highpPipeline) {
	// Load u8xTail and cast it to f32x8.
	data := p.aaMaskCtx.copyAtXY(p.dx, p.dy, p.tail)
	c := wide.F32x8{float32(data[0]), float32(data[1]), 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}
	c = c.Div(wide.SplatF32x8(255.0))

	p.r = refHighpLerpHighp(p.dr, p.r, c)
	p.g = refHighpLerpHighp(p.dg, p.g, c)
	p.b = refHighpLerpHighp(p.db, p.b, c)
	p.a = refHighpLerpHighp(p.da, p.a, c)
}

// refHighpScale1Float is the body of scale1Float before D33.
func refHighpScale1Float(p *highpPipeline) {
	c := wide.SplatF32x8(p.ctx.currentCoverage)
	p.r = p.r.Mul(c)
	p.g = p.g.Mul(c)
	p.b = p.b.Mul(c)
	p.a = p.a.Mul(c)
}

// refHighpLerp1Float is the body of lerp1Float before D33.
func refHighpLerp1Float(p *highpPipeline) {
	c := wide.SplatF32x8(p.ctx.currentCoverage)
	p.r = refHighpLerpHighp(p.dr, p.r, c)
	p.g = refHighpLerpHighp(p.dg, p.g, c)
	p.b = refHighpLerpHighp(p.db, p.b, c)
	p.a = refHighpLerpHighp(p.da, p.a, c)
}

// refHighpClear is the body of clear before D33.
func refHighpClear(p *highpPipeline) {
	p.r = wide.F32x8{}
	p.g = wide.F32x8{}
	p.b = wide.F32x8{}
	p.a = wide.F32x8{}
}

// refHighpBlendFn is the body of blendFn before D33.
func refHighpBlendFn(p *highpPipeline, f func(s, d, sa, da wide.F32x8) wide.F32x8) {
	p.r = f(p.r, p.dr, p.a, p.da)
	p.g = f(p.g, p.dg, p.a, p.da)
	p.b = f(p.b, p.db, p.a, p.da)
	p.a = f(p.a, p.da, p.a, p.da)
}

// refHighpSourceAtop is the body of sourceAtop before D33.
func refHighpSourceAtop(p *highpPipeline) { refHighpBlendFn(p, refHighpSourceAtopHighp) }

// refHighpSourceAtopHighp is the body of sourceAtopHighp before D33.
func refHighpSourceAtopHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Mul(da).Add(d.Mul(refHighpInvHighp(sa)))
}

// refHighpDestinationAtop is the body of destinationAtop before D33.
func refHighpDestinationAtop(p *highpPipeline) { refHighpBlendFn(p, refHighpDestinationAtopHighp) }

// refHighpDestinationAtopHighp is the body of destinationAtopHighp before D33.
func refHighpDestinationAtopHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return d.Mul(sa).Add(s.Mul(refHighpInvHighp(da)))
}

// refHighpSourceIn is the body of sourceIn before D33.
func refHighpSourceIn(p *highpPipeline) { refHighpBlendFn(p, refHighpSourceInHighp) }

// refHighpSourceInHighp is the body of sourceInHighp before D33.
func refHighpSourceInHighp(s, _, _, da wide.F32x8) wide.F32x8 {
	return s.Mul(da)
}

// refHighpDestinationIn is the body of destinationIn before D33.
func refHighpDestinationIn(p *highpPipeline) { refHighpBlendFn(p, refHighpDestinationInHighp) }

// refHighpDestinationInHighp is the body of destinationInHighp before D33.
func refHighpDestinationInHighp(_, d, sa, _ wide.F32x8) wide.F32x8 {
	return d.Mul(sa)
}

// refHighpSourceOut is the body of sourceOut before D33.
func refHighpSourceOut(p *highpPipeline) { refHighpBlendFn(p, refHighpSourceOutHighp) }

// refHighpSourceOutHighp is the body of sourceOutHighp before D33.
func refHighpSourceOutHighp(s, _, _, da wide.F32x8) wide.F32x8 {
	return s.Mul(refHighpInvHighp(da))
}

// refHighpDestinationOut is the body of destinationOut before D33.
func refHighpDestinationOut(p *highpPipeline) { refHighpBlendFn(p, refHighpDestinationOutHighp) }

// refHighpDestinationOutHighp is the body of destinationOutHighp before D33.
func refHighpDestinationOutHighp(_, d, sa, _ wide.F32x8) wide.F32x8 {
	return d.Mul(refHighpInvHighp(sa))
}

// refHighpSourceOver is the body of sourceOver before D33.
func refHighpSourceOver(p *highpPipeline) { refHighpBlendFn(p, refHighpSourceOverHighp) }

// refHighpSourceOverHighp is the body of sourceOverHighp before D33.
func refHighpSourceOverHighp(s, d, sa, _ wide.F32x8) wide.F32x8 {
	return refHighpMadHighp(d, refHighpInvHighp(sa), s)
}

// refHighpDestinationOver is the body of destinationOver before D33.
func refHighpDestinationOver(p *highpPipeline) { refHighpBlendFn(p, refHighpDestinationOverHighp) }

// refHighpDestinationOverHighp is the body of destinationOverHighp before D33.
func refHighpDestinationOverHighp(s, d, _, da wide.F32x8) wide.F32x8 {
	return refHighpMadHighp(s, refHighpInvHighp(da), d)
}

// refHighpModulate is the body of modulate before D33.
func refHighpModulate(p *highpPipeline) { refHighpBlendFn(p, refHighpModulateHighp) }

// refHighpModulateHighp is the body of modulateHighp before D33.
func refHighpModulateHighp(s, d, _, _ wide.F32x8) wide.F32x8 {
	return s.Mul(d)
}

// refHighpMultiply is the body of multiply before D33.
func refHighpMultiply(p *highpPipeline) { refHighpBlendFn(p, refHighpMultiplyHighp) }

// refHighpMultiplyHighp is the body of multiplyHighp before D33.
func refHighpMultiplyHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Mul(refHighpInvHighp(da)).Add(d.Mul(refHighpInvHighp(sa))).Add(s.Mul(d))
}

// refHighpScreen is the body of screen before D33.
func refHighpScreen(p *highpPipeline) { refHighpBlendFn(p, refHighpScreenHighp) }

// refHighpScreenHighp is the body of screenHighp before D33.
func refHighpScreenHighp(s, d, _, _ wide.F32x8) wide.F32x8 {
	return s.Add(d).Sub(s.Mul(d))
}

// refHighpXor is the body of xor before D33.
func refHighpXor(p *highpPipeline) { refHighpBlendFn(p, refHighpXorHighp) }

// refHighpXorHighp is the body of xorHighp before D33.
func refHighpXorHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Mul(refHighpInvHighp(da)).Add(d.Mul(refHighpInvHighp(sa)))
}

// refHighpPlus is the body of plus before D33.
func refHighpPlus(p *highpPipeline) { refHighpBlendFn(p, refHighpPlusHighp) }

// refHighpPlusHighp is the body of plusHighp before D33.
func refHighpPlusHighp(s, d, _, _ wide.F32x8) wide.F32x8 {
	return s.Add(d).Min(wide.SplatF32x8(1.0))
}

// refHighpBlendFn2 is the body of blendFn2 before D33.
func refHighpBlendFn2(p *highpPipeline, f func(s, d, sa, da wide.F32x8) wide.F32x8) {
	// The same logic applied to color, and source_over for alpha.
	p.r = f(p.r, p.dr, p.a, p.da)
	p.g = f(p.g, p.dg, p.a, p.da)
	p.b = f(p.b, p.db, p.a, p.da)
	p.a = refHighpMadHighp(p.da, refHighpInvHighp(p.a), p.a)
}

// refHighpDarken is the body of darken before D33.
func refHighpDarken(p *highpPipeline) { refHighpBlendFn2(p, refHighpDarkenHighp) }

// refHighpDarkenHighp is the body of darkenHighp before D33.
func refHighpDarkenHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Add(d).Sub(s.Mul(da).Max(d.Mul(sa)))
}

// refHighpLighten is the body of lighten before D33.
func refHighpLighten(p *highpPipeline) { refHighpBlendFn2(p, refHighpLightenHighp) }

// refHighpLightenHighp is the body of lightenHighp before D33.
func refHighpLightenHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Add(d).Sub(s.Mul(da).Min(d.Mul(sa)))
}

// refHighpDifference is the body of difference before D33.
func refHighpDifference(p *highpPipeline) { refHighpBlendFn2(p, refHighpDifferenceHighp) }

// refHighpDifferenceHighp is the body of differenceHighp before D33.
func refHighpDifferenceHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Add(d).Sub(refHighpTwo(s.Mul(da).Min(d.Mul(sa))))
}

// refHighpExclusion is the body of exclusion before D33.
func refHighpExclusion(p *highpPipeline) { refHighpBlendFn2(p, refHighpExclusionHighp) }

// refHighpExclusionHighp is the body of exclusionHighp before D33.
func refHighpExclusionHighp(s, d, _, _ wide.F32x8) wide.F32x8 {
	return s.Add(d).Sub(refHighpTwo(s.Mul(d)))
}

// refHighpColorBurn is the body of colorBurn before D33.
func refHighpColorBurn(p *highpPipeline) { refHighpBlendFn2(p, refHighpColorBurnHighp) }

// refHighpColorBurnHighp is the body of colorBurnHighp before D33.
func refHighpColorBurnHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return d.CmpEq(da).Blend(
		d.Add(s.Mul(refHighpInvHighp(da))),
		s.CmpEq(wide.F32x8{}).Blend(
			d.Mul(refHighpInvHighp(sa)),
			sa.Mul(da.Sub(da.Min(da.Sub(d).Mul(sa).Mul(s.RecipFast())))).Add(s.Mul(refHighpInvHighp(da))).Add(d.Mul(refHighpInvHighp(sa))),
		),
	)
}

// refHighpColorDodge is the body of colorDodge before D33.
func refHighpColorDodge(p *highpPipeline) { refHighpBlendFn2(p, refHighpColorDodgeHighp) }

// refHighpColorDodgeHighp is the body of colorDodgeHighp before D33.
func refHighpColorDodgeHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return d.CmpEq(wide.F32x8{}).Blend(
		s.Mul(refHighpInvHighp(da)),
		s.CmpEq(sa).Blend(
			s.Add(d.Mul(refHighpInvHighp(sa))),
			sa.Mul(da.Min(d.Mul(sa).Mul(sa.Sub(s).RecipFast()))).Add(s.Mul(refHighpInvHighp(da))).Add(d.Mul(refHighpInvHighp(sa))),
		),
	)
}

// refHighpHardLight is the body of hardLight before D33.
func refHighpHardLight(p *highpPipeline) { refHighpBlendFn2(p, refHighpHardLightHighp) }

// refHighpHardLightHighp is the body of hardLightHighp before D33.
func refHighpHardLightHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Mul(refHighpInvHighp(da)).Add(d.Mul(refHighpInvHighp(sa))).Add(refHighpTwo(s).CmpLe(sa).Blend(
		refHighpTwo(s.Mul(d)),
		sa.Mul(da).Sub(refHighpTwo(da.Sub(d).Mul(sa.Sub(s)))),
	))
}

// refHighpOverlay is the body of overlay before D33.
func refHighpOverlay(p *highpPipeline) { refHighpBlendFn2(p, refHighpOverlayHighp) }

// refHighpOverlayHighp is the body of overlayHighp before D33.
func refHighpOverlayHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Mul(refHighpInvHighp(da)).Add(d.Mul(refHighpInvHighp(sa))).Add(refHighpTwo(d).CmpLe(da).Blend(
		refHighpTwo(s.Mul(d)),
		sa.Mul(da).Sub(refHighpTwo(da.Sub(d).Mul(sa.Sub(s)))),
	))
}

// refHighpSoftLight is the body of softLight before D33.
func refHighpSoftLight(p *highpPipeline) { refHighpBlendFn2(p, refHighpSoftLightHighp) }

// refHighpSoftLightHighp is the body of softLightHighp before D33.
func refHighpSoftLightHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	one := wide.SplatF32x8(1.0)
	m := da.CmpGt(wide.F32x8{}).Blend(d.Div(da), wide.F32x8{})
	s2 := refHighpTwo(s)
	m4 := refHighpTwo(refHighpTwo(m))

	// The logic forks three ways:
	//    1. dark src?
	//    2. light src, dark dst?
	//    3. light src, light dst?
	darkSrc := d.Mul(sa.Add(s2.Sub(sa).Mul(one.Sub(m))))
	darkDst := m4.Mul(m4).Add(m4).Mul(m.Sub(one)).Add(wide.SplatF32x8(7.0).Mul(m))
	liteDst := m.Sqrt().Sub(m)
	liteSrc := d.Mul(sa).Add(da.Mul(s2.Sub(sa)).
		Mul(refHighpTwo(refHighpTwo(d)).CmpLe(da).Blend(darkDst, liteDst))) // 2 or 3?

	return s.Mul(refHighpInvHighp(da)).Add(d.Mul(refHighpInvHighp(sa))).Add(s2.CmpLe(sa).Blend(darkSrc, liteSrc)) // 1 or (2 or 3)?
}

// refHighpBlendFn3 is the body of blendFn3 before D33.
func refHighpBlendFn3(p *highpPipeline, f func(r, g, b, a, dr, dg, db, da wide.F32x8) (wide.F32x8, wide.F32x8, wide.F32x8, wide.F32x8)) {
	tr, tg, tb, ta := f(p.r, p.g, p.b, p.a, p.dr, p.dg, p.db, p.da)
	p.r = tr
	p.g = tg
	p.b = tb
	p.a = ta
}

// refHighpHue is the body of hue before D33.
func refHighpHue(p *highpPipeline) { refHighpBlendFn3(p, refHighpHueK) }

// refHighpHueK is the body of hueK before D33.
func refHighpHueK(r, g, b, a, dr, dg, db, da wide.F32x8) (wide.F32x8, wide.F32x8, wide.F32x8, wide.F32x8) {
	rr := r.Mul(a)
	gg := g.Mul(a)
	bb := b.Mul(a)

	refHighpSetSat(&rr, &gg, &bb, refHighpSat(dr, dg, db).Mul(a))
	refHighpSetLum(&rr, &gg, &bb, refHighpLum(dr, dg, db).Mul(a))
	refHighpClipColor(&rr, &gg, &bb, a.Mul(da))

	r = r.Mul(refHighpInvHighp(da)).Add(dr.Mul(refHighpInvHighp(a))).Add(rr)
	g = g.Mul(refHighpInvHighp(da)).Add(dg.Mul(refHighpInvHighp(a))).Add(gg)
	b = b.Mul(refHighpInvHighp(da)).Add(db.Mul(refHighpInvHighp(a))).Add(bb)
	a = a.Add(da).Sub(a.Mul(da))

	return r, g, b, a
}

// refHighpSaturation is the body of saturation before D33.
func refHighpSaturation(p *highpPipeline) { refHighpBlendFn3(p, refHighpSaturationK) }

// refHighpSaturationK is the body of saturationK before D33.
func refHighpSaturationK(r, g, b, a, dr, dg, db, da wide.F32x8) (wide.F32x8, wide.F32x8, wide.F32x8, wide.F32x8) {
	rr := dr.Mul(a)
	gg := dg.Mul(a)
	bb := db.Mul(a)

	refHighpSetSat(&rr, &gg, &bb, refHighpSat(r, g, b).Mul(da))
	refHighpSetLum(&rr, &gg, &bb, refHighpLum(dr, dg, db).Mul(a)) // (This is not redundant.)
	refHighpClipColor(&rr, &gg, &bb, a.Mul(da))

	r = r.Mul(refHighpInvHighp(da)).Add(dr.Mul(refHighpInvHighp(a))).Add(rr)
	g = g.Mul(refHighpInvHighp(da)).Add(dg.Mul(refHighpInvHighp(a))).Add(gg)
	b = b.Mul(refHighpInvHighp(da)).Add(db.Mul(refHighpInvHighp(a))).Add(bb)
	a = a.Add(da).Sub(a.Mul(da))

	return r, g, b, a
}

// refHighpColor is the body of color before D33.
func refHighpColor(p *highpPipeline) { refHighpBlendFn3(p, refHighpColorK) }

// refHighpColorK is the body of colorK before D33.
func refHighpColorK(r, g, b, a, dr, dg, db, da wide.F32x8) (wide.F32x8, wide.F32x8, wide.F32x8, wide.F32x8) {
	rr := r.Mul(da)
	gg := g.Mul(da)
	bb := b.Mul(da)

	refHighpSetLum(&rr, &gg, &bb, refHighpLum(dr, dg, db).Mul(a))
	refHighpClipColor(&rr, &gg, &bb, a.Mul(da))

	r = r.Mul(refHighpInvHighp(da)).Add(dr.Mul(refHighpInvHighp(a))).Add(rr)
	g = g.Mul(refHighpInvHighp(da)).Add(dg.Mul(refHighpInvHighp(a))).Add(gg)
	b = b.Mul(refHighpInvHighp(da)).Add(db.Mul(refHighpInvHighp(a))).Add(bb)
	a = a.Add(da).Sub(a.Mul(da))

	return r, g, b, a
}

// refHighpLuminosity is the body of luminosity before D33.
func refHighpLuminosity(p *highpPipeline) { refHighpBlendFn3(p, refHighpLuminosityK) }

// refHighpLuminosityK is the body of luminosityK before D33.
func refHighpLuminosityK(r, g, b, a, dr, dg, db, da wide.F32x8) (wide.F32x8, wide.F32x8, wide.F32x8, wide.F32x8) {
	rr := dr.Mul(a)
	gg := dg.Mul(a)
	bb := db.Mul(a)

	refHighpSetLum(&rr, &gg, &bb, refHighpLum(r, g, b).Mul(da))
	refHighpClipColor(&rr, &gg, &bb, a.Mul(da))

	r = r.Mul(refHighpInvHighp(da)).Add(dr.Mul(refHighpInvHighp(a))).Add(rr)
	g = g.Mul(refHighpInvHighp(da)).Add(dg.Mul(refHighpInvHighp(a))).Add(gg)
	b = b.Mul(refHighpInvHighp(da)).Add(db.Mul(refHighpInvHighp(a))).Add(bb)
	a = a.Add(da).Sub(a.Mul(da))

	return r, g, b, a
}

// refHighpSat is the body of sat before D33.
func refHighpSat(r, g, b wide.F32x8) wide.F32x8 {
	return r.Max(g.Max(b)).Sub(r.Min(g.Min(b)))
}

// refHighpLum is the body of lum before D33.
func refHighpLum(r, g, b wide.F32x8) wide.F32x8 {
	return r.Mul(wide.SplatF32x8(0.30)).Add(g.Mul(wide.SplatF32x8(0.59))).Add(b.Mul(wide.SplatF32x8(0.11)))
}

// refHighpSetSat is the body of setSat before D33.
func refHighpSetSat(r, g, b *wide.F32x8, s wide.F32x8) {
	mn := r.Min(g.Min(*b))
	mx := r.Max(g.Max(*b))
	sat := mx.Sub(mn)

	// Map min channel to 0, max channel to s, and scale the middle proportionally.
	//
	// Port: upstream has a closure scale. Go writes it out for each channel.
	isZero := sat.CmpEq(wide.F32x8{})
	*r = isZero.Blend(wide.F32x8{}, r.Sub(mn).Mul(s).Div(sat))
	*g = isZero.Blend(wide.F32x8{}, g.Sub(mn).Mul(s).Div(sat))
	*b = isZero.Blend(wide.F32x8{}, b.Sub(mn).Mul(s).Div(sat))
}

// refHighpSetLum is the body of setLum before D33.
func refHighpSetLum(r, g, b *wide.F32x8, l wide.F32x8) {
	diff := l.Sub(refHighpLum(*r, *g, *b))
	*r = r.Add(diff)
	*g = g.Add(diff)
	*b = b.Add(diff)
}

// refHighpClipColor is the body of clipColor before D33.
func refHighpClipColor(r, g, b *wide.F32x8, a wide.F32x8) {
	mn := r.Min(g.Min(*b))
	mx := r.Max(g.Max(*b))
	l := refHighpLum(*r, *g, *b)

	*r = refHighpClipColorChannel(*r, mn, mx, l, a)
	*g = refHighpClipColorChannel(*g, mn, mx, l, a)
	*b = refHighpClipColorChannel(*b, mn, mx, l, a)
}

// refHighpClipColorChannel is the body of clipColorChannel before D33.
func refHighpClipColorChannel(c, mn, mx, l, a wide.F32x8) wide.F32x8 {
	c = mx.CmpGe(wide.F32x8{}).Blend(c, l.Add(c.Sub(l).Mul(l).Div(l.Sub(mn))))
	c = mx.CmpGt(a).Blend(l.Add(c.Sub(l).Mul(a.Sub(l)).Div(mx.Sub(l))), c)
	c = c.Max(wide.F32x8{}) // Sometimes without this we can dip just a little negative.
	return c
}

// refHighpSourceOverRgba is the body of sourceOverRgba before D33.
func refHighpSourceOverRgba(p *highpPipeline) {
	pixels := p.pixmapDst.slice4AtXY(p.dx, p.dy)
	refHighpLoad8888Highp(pixels, &p.dr, &p.dg, &p.db, &p.da)
	p.r = refHighpMadHighp(p.dr, refHighpInvHighp(p.a), p.r)
	p.g = refHighpMadHighp(p.dg, refHighpInvHighp(p.a), p.g)
	p.b = refHighpMadHighp(p.db, refHighpInvHighp(p.a), p.b)
	p.a = refHighpMadHighp(p.da, refHighpInvHighp(p.a), p.a)
	refHighpStore8888Highp(&p.r, &p.g, &p.b, &p.a, pixels)
}

// refHighpSourceOverRgbaTail is the body of sourceOverRgbaTail before D33.
func refHighpSourceOverRgbaTail(p *highpPipeline) {
	pixels := p.pixmapDst.sliceAtXY(p.dx, p.dy)
	refHighpLoad8888TailHighp(p.tail, pixels, &p.dr, &p.dg, &p.db, &p.da)
	p.r = refHighpMadHighp(p.dr, refHighpInvHighp(p.a), p.r)
	p.g = refHighpMadHighp(p.dg, refHighpInvHighp(p.a), p.g)
	p.b = refHighpMadHighp(p.db, refHighpInvHighp(p.a), p.b)
	p.a = refHighpMadHighp(p.da, refHighpInvHighp(p.a), p.a)
	refHighpStore8888TailHighp(&p.r, &p.g, &p.b, &p.a, p.tail, pixels)
}

// refHighpTransform is the body of transform before D33.
func refHighpTransform(p *highpPipeline) {
	ts := &p.ctx.transform

	tr := refHighpMadHighp(p.r, wide.SplatF32x8(ts.SX), refHighpMadHighp(p.g, wide.SplatF32x8(ts.KX), wide.SplatF32x8(ts.TX)))
	tg := refHighpMadHighp(p.r, wide.SplatF32x8(ts.KY), refHighpMadHighp(p.g, wide.SplatF32x8(ts.SY), wide.SplatF32x8(ts.TY)))
	p.r = tr
	p.g = tg
}

// refHighpReflect is the body of reflect before D33.
func refHighpReflect(p *highpPipeline) {
	ctx := &p.ctx.limitX
	p.r = refHighpExclusiveReflect(p.r, ctx.scale, ctx.invScale)

	ctx = &p.ctx.limitY
	p.g = refHighpExclusiveReflect(p.g, ctx.scale, ctx.invScale)
}

// refHighpExclusiveReflect is the body of exclusiveReflect before D33.
func refHighpExclusiveReflect(v wide.F32x8, limit, invLimit float32) wide.F32x8 {
	limitV := wide.SplatF32x8(limit)
	invLimitV := wide.SplatF32x8(invLimit)
	return v.Sub(limitV).Sub(limitV.Add(limitV).
		Mul(v.Sub(limitV).Mul(invLimitV.Mul(wide.SplatF32x8(0.5))).Floor())).Sub(limitV).Abs()
}

// refHighpRepeat is the body of repeat before D33.
func refHighpRepeat(p *highpPipeline) {
	ctx := &p.ctx.limitX
	p.r = refHighpExclusiveRepeat(p.r, ctx.scale, ctx.invScale)

	ctx = &p.ctx.limitY
	p.g = refHighpExclusiveRepeat(p.g, ctx.scale, ctx.invScale)
}

// refHighpExclusiveRepeat is the body of exclusiveRepeat before D33.
func refHighpExclusiveRepeat(v wide.F32x8, limit, invLimit float32) wide.F32x8 {
	return v.Sub(v.Mul(wide.SplatF32x8(invLimit)).Floor().Mul(wide.SplatF32x8(limit)))
}

// refHighpBilinear is the body of bilinear before D33.
func refHighpBilinear(p *highpPipeline) {
	x := p.r
	fx := x.Add(wide.SplatF32x8(0.5)).Fract()
	y := p.g
	fy := y.Add(wide.SplatF32x8(0.5)).Fract()
	one := wide.SplatF32x8(1.0)
	wx := [2]wide.F32x8{one.Sub(fx), fx}
	wy := [2]wide.F32x8{one.Sub(fy), fy}

	refHighpSampler2x2(p.pixmapSrc, &p.ctx.sampler, x, y, &wx, &wy, &p.r, &p.g, &p.b, &p.a)
}

// refHighpBicubic is the body of bicubic before D33.
func refHighpBicubic(p *highpPipeline) {
	x := p.r
	fx := x.Add(wide.SplatF32x8(0.5)).Fract()
	y := p.g
	fy := y.Add(wide.SplatF32x8(0.5)).Fract()
	one := wide.SplatF32x8(1.0)
	wx := [4]wide.F32x8{refHighpBicubicFar(one.Sub(fx)), refHighpBicubicNear(one.Sub(fx)), refHighpBicubicNear(fx), refHighpBicubicFar(fx)}
	wy := [4]wide.F32x8{refHighpBicubicFar(one.Sub(fy)), refHighpBicubicNear(one.Sub(fy)), refHighpBicubicNear(fy), refHighpBicubicFar(fy)}

	refHighpSampler4x4(p.pixmapSrc, &p.ctx.sampler, x, y, &wx, &wy, &p.r, &p.g, &p.b, &p.a)
}

// refHighpBicubicNear is the body of bicubicNear before D33.
func refHighpBicubicNear(t wide.F32x8) wide.F32x8 {
	// 1/18 + 9/18t + 27/18t^2 - 21/18t^3 == t ( t ( -21/18t + 27/18) + 9/18) + 1/18
	return refHighpMadHighp(
		t,
		refHighpMadHighp(t,
			refHighpMadHighp(
				wide.SplatF32x8(-21.0/18.0),
				t,
				wide.SplatF32x8(27.0/18.0),
			),
			wide.SplatF32x8(9.0/18.0),
		),
		wide.SplatF32x8(1.0/18.0),
	)
}

// refHighpBicubicFar is the body of bicubicFar before D33.
func refHighpBicubicFar(t wide.F32x8) wide.F32x8 {
	// 0/18 + 0/18*t - 6/18t^2 + 7/18t^3 == t^2 (7/18t - 6/18)
	return t.Mul(t).Mul(refHighpMadHighp(wide.SplatF32x8(7.0/18.0), t, wide.SplatF32x8(-6.0/18.0)))
}

// refHighpSampler2x2 is the body of sampler2x2 before D33.
func refHighpSampler2x2(
	pixmap PixmapRef,
	ctx *samplerCtx,
	cx, cy wide.F32x8,
	wx, wy *[2]wide.F32x8,
	r, g, b, a *wide.F32x8,
) {
	*r = wide.F32x8{}
	*g = wide.F32x8{}
	*b = wide.F32x8{}
	*a = wide.F32x8{}

	one := wide.SplatF32x8(1.0)
	const start = -0.5
	y := cy.Add(wide.SplatF32x8(start))
	for j := range 2 {
		x := cx.Add(wide.SplatF32x8(start))
		for i := range 2 {
			var rr, gg, bb, aa wide.F32x8
			refHighpSample(pixmap, ctx, x, y, &rr, &gg, &bb, &aa)

			w := wx[i].Mul(wy[j])
			*r = refHighpMadHighp(w, rr, *r)
			*g = refHighpMadHighp(w, gg, *g)
			*b = refHighpMadHighp(w, bb, *b)
			*a = refHighpMadHighp(w, aa, *a)

			x = x.Add(one)
		}

		y = y.Add(one)
	}
}

// refHighpSampler4x4 is the body of sampler4x4 before D33.
func refHighpSampler4x4(
	pixmap PixmapRef,
	ctx *samplerCtx,
	cx, cy wide.F32x8,
	wx, wy *[4]wide.F32x8,
	r, g, b, a *wide.F32x8,
) {
	*r = wide.F32x8{}
	*g = wide.F32x8{}
	*b = wide.F32x8{}
	*a = wide.F32x8{}

	one := wide.SplatF32x8(1.0)
	const start = -1.5
	y := cy.Add(wide.SplatF32x8(start))
	for j := range 4 {
		x := cx.Add(wide.SplatF32x8(start))
		for i := range 4 {
			var rr, gg, bb, aa wide.F32x8
			refHighpSample(pixmap, ctx, x, y, &rr, &gg, &bb, &aa)

			w := wx[i].Mul(wy[j])
			*r = refHighpMadHighp(w, rr, *r)
			*g = refHighpMadHighp(w, gg, *g)
			*b = refHighpMadHighp(w, bb, *b)
			*a = refHighpMadHighp(w, aa, *a)

			x = x.Add(one)
		}

		y = y.Add(one)
	}
}

// refHighpSample is the body of sample before D33.
func refHighpSample(
	pixmap PixmapRef, ctx *samplerCtx, x, y wide.F32x8,
	r, g, b, a *wide.F32x8,
) {
	x = refHighpTile(x, ctx.spreadMode, float32(pixmap.Width()), ctx.invWidth)
	y = refHighpTile(y, ctx.spreadMode, float32(pixmap.Height()), ctx.invHeight)

	ix := refHighpGatherIx(pixmap, x, y)
	pixels := pixmap.gather(ix)
	refHighpLoad8888Highp(&pixels, r, g, b, a)
}

// refHighpTile is the body of tile before D33.
func refHighpTile(v wide.F32x8, mode SpreadMode, limit, invLimit float32) wide.F32x8 {
	switch mode {
	case SpreadModeRepeat:
		return refHighpExclusiveRepeat(v, limit, invLimit)
	case SpreadModeReflect:
		return refHighpExclusiveReflect(v, limit, invLimit)
	}
	// SpreadModePad
	return v
}

// refHighpPadX1 is the body of padX1 before D33.
func refHighpPadX1(p *highpPipeline) {
	p.r = p.r.Normalize()
}

// refHighpReflectX1 is the body of reflectX1 before D33.
func refHighpReflectX1(p *highpPipeline) {
	one := wide.SplatF32x8(1.0)
	p.r = p.r.Sub(one).
		Sub(refHighpTwo(p.r.Sub(one).Mul(wide.SplatF32x8(0.5)).Floor())).
		Sub(one).
		Abs().Normalize()
}

// refHighpRepeatX1 is the body of repeatX1 before D33.
func refHighpRepeatX1(p *highpPipeline) {
	p.r = p.r.Sub(p.r.Floor()).Normalize()
}

// refHighpGradient is the body of gradient before D33.
func refHighpGradient(p *highpPipeline) {
	ctx := &p.ctx.gradient

	// N.B. The loop starts at 1 because idx 0 is the color to use before the first stop.
	t := p.r
	var idx wide.U32x8
	for i := 1; i < ctx.len; i++ {
		tt := ctx.tValues[i].Get()
		n := wide.U32x8{
			b2u32(t[0] >= tt),
			b2u32(t[1] >= tt),
			b2u32(t[2] >= tt),
			b2u32(t[3] >= tt),
			b2u32(t[4] >= tt),
			b2u32(t[5] >= tt),
			b2u32(t[6] >= tt),
			b2u32(t[7] >= tt),
		}
		idx = idx.Add(n)
	}
	refHighpGradientLookupHighp(ctx, &idx, p.r, &p.r, &p.g, &p.b, &p.a)
}

// refHighpGradientLookupHighp is the body of gradientLookupHighp before D33.
func refHighpGradientLookupHighp(
	ctx *gradientCtx, idx *wide.U32x8, t wide.F32x8,
	r, g, b, a *wide.F32x8,
) {
	// Surprisingly, but bound checking does not affect the performance.
	// And since `idx` can contain any number, we leave it in place.
	//
	// Port: the macro gather! of upstream reads one field for each lane. Go
	// reads all the fields of a lane at once, which gives the same values.
	var fr, fg, fb, fa, br, bg, bb, ba wide.F32x8
	for i := range highpStageWidth {
		f := &ctx.factors[idx[i]]
		fr[i] = f.r
		fg[i] = f.g
		fb[i] = f.b
		fa[i] = f.a
	}
	for i := range highpStageWidth {
		c := &ctx.biases[idx[i]]
		br[i] = c.r
		bg[i] = c.g
		bb[i] = c.b
		ba[i] = c.a
	}

	*r = refHighpMadHighp(t, fr, br)
	*g = refHighpMadHighp(t, fg, bg)
	*b = refHighpMadHighp(t, fb, bb)
	*a = refHighpMadHighp(t, fa, ba)
}

// refHighpEvenlySpaced2StopGradient is the body of evenlySpaced2StopGradient before D33.
func refHighpEvenlySpaced2StopGradient(p *highpPipeline) {
	ctx := &p.ctx.evenlySpaced2StopGradient

	t := p.r
	p.r = refHighpMadHighp(t, wide.SplatF32x8(ctx.factor.r), wide.SplatF32x8(ctx.bias.r))
	p.g = refHighpMadHighp(t, wide.SplatF32x8(ctx.factor.g), wide.SplatF32x8(ctx.bias.g))
	p.b = refHighpMadHighp(t, wide.SplatF32x8(ctx.factor.b), wide.SplatF32x8(ctx.bias.b))
	p.a = refHighpMadHighp(t, wide.SplatF32x8(ctx.factor.a), wide.SplatF32x8(ctx.bias.a))
}

// refHighpXyToUnitAngle is the body of xyToUnitAngle before D33.
func refHighpXyToUnitAngle(p *highpPipeline) {
	x := p.r
	y := p.g
	xAbs := x.Abs()
	yAbs := y.Abs()
	slope := xAbs.Min(yAbs).Div(xAbs.Max(yAbs))
	s := slope.Mul(slope)
	// Use a 7th degree polynomial to approximate atan.
	// This was generated using sollya.gforge.inria.fr.
	// A float optimized polynomial was generated using the following command.
	// P1 = fpminimax((1/(2*Pi))*atan(x),[|1,3,5,7|],[|24...|],[2^(-40),1],relative);
	phi := slope.Mul(
		wide.SplatF32x8(0.15912117063999176025390625).Add(
			s.Mul(wide.SplatF32x8(-5.185396969318389892578125e-2).Add(
				s.Mul(wide.SplatF32x8(2.476101927459239959716796875e-2).Add(
					s.Mul(wide.SplatF32x8(-7.0547382347285747528076171875e-3))))))))
	phi = xAbs.CmpLt(yAbs).Blend(wide.SplatF32x8(0.25).Sub(phi), phi)
	phi = x.
		CmpLt(wide.SplatF32x8(0.0)).
		Blend(wide.SplatF32x8(0.5).Sub(phi), phi)
	phi = y.
		CmpLt(wide.SplatF32x8(0.0)).
		Blend(wide.SplatF32x8(1.0).Sub(phi), phi)
	phi = phi.CmpNe(phi).Blend(wide.SplatF32x8(0.0), phi)
	p.r = phi
}

// refHighpXyToRadius is the body of xyToRadius before D33.
func refHighpXyToRadius(p *highpPipeline) {
	x2 := p.r.Mul(p.r)
	y2 := p.g.Mul(p.g)
	p.r = x2.Add(y2).Sqrt()
}

// refHighpXyTo2PtConicalFocalOnCircle is the body of xyTo2PtConicalFocalOnCircle before D33.
func refHighpXyTo2PtConicalFocalOnCircle(p *highpPipeline) {
	x := p.r
	y := p.g
	p.r = x.Add(y.Mul(y).Div(x))
}

// refHighpXyTo2PtConicalWellBehaved is the body of xyTo2PtConicalWellBehaved before D33.
func refHighpXyTo2PtConicalWellBehaved(p *highpPipeline) {
	ctx := &p.ctx.twoPointConicalGradient

	x := p.r
	y := p.g
	p.r = x.Mul(x).Add(y.Mul(y)).Sqrt().Sub(x.Mul(wide.SplatF32x8(ctx.p0)))
}

// refHighpXyTo2PtConicalGreater is the body of xyTo2PtConicalGreater before D33.
func refHighpXyTo2PtConicalGreater(p *highpPipeline) {
	ctx := &p.ctx.twoPointConicalGradient

	x := p.r
	y := p.g
	p.r = x.Mul(x).Sub(y.Mul(y)).Sqrt().Sub(x.Mul(wide.SplatF32x8(ctx.p0)))
}

// refHighpXyTo2PtConicalSmaller is the body of xyTo2PtConicalSmaller before D33.
func refHighpXyTo2PtConicalSmaller(p *highpPipeline) {
	ctx := &p.ctx.twoPointConicalGradient

	x := p.r
	y := p.g
	p.r = x.Mul(x).Sub(y.Mul(y)).Sqrt().Neg().Sub(x.Mul(wide.SplatF32x8(ctx.p0)))
}

// refHighpXyTo2PtConicalStrip is the body of xyTo2PtConicalStrip before D33.
func refHighpXyTo2PtConicalStrip(p *highpPipeline) {
	ctx := &p.ctx.twoPointConicalGradient

	x := p.r
	y := p.g
	p.r = x.Add(wide.SplatF32x8(ctx.p0).Sub(y.Mul(y)).Sqrt())
}

// refHighpMask2PtConicalNan is the body of mask2PtConicalNan before D33.
func refHighpMask2PtConicalNan(p *highpPipeline) {
	ctx := &p.ctx.twoPointConicalGradient

	t := p.r
	isDegenerate := t.CmpNe(t)
	p.r = isDegenerate.Blend(wide.F32x8{}, t)
	ctx.mask = condToMask(isDegenerate.ToU32x8Bitcast().Not())
}

// refHighpMask2PtConicalDegenerates is the body of mask2PtConicalDegenerates before D33.
func refHighpMask2PtConicalDegenerates(p *highpPipeline) {
	ctx := &p.ctx.twoPointConicalGradient

	t := p.r
	isDegenerate := t.CmpLe(wide.F32x8{}).Or(t.CmpNe(t))
	p.r = isDegenerate.Blend(wide.F32x8{}, t)
	ctx.mask = condToMask(isDegenerate.ToU32x8Bitcast().Not())
}

// refHighpApplyVectorMask is the body of applyVectorMask before D33.
func refHighpApplyVectorMask(p *highpPipeline) {
	ctx := &p.ctx.twoPointConicalGradient

	p.r = p.r.ToU32x8Bitcast().And(ctx.mask).ToF32x8Bitcast()
	p.g = p.g.ToU32x8Bitcast().And(ctx.mask).ToF32x8Bitcast()
	p.b = p.b.ToU32x8Bitcast().And(ctx.mask).ToF32x8Bitcast()
	p.a = p.a.ToU32x8Bitcast().And(ctx.mask).ToF32x8Bitcast()
}

// refHighpAlter2PtConicalCompensateFocal is the body of alter2PtConicalCompensateFocal before D33.
func refHighpAlter2PtConicalCompensateFocal(p *highpPipeline) {
	ctx := &p.ctx.twoPointConicalGradient

	p.r = p.r.Add(wide.SplatF32x8(ctx.p1))
}

// refHighpAlter2PtConicalUnswap is the body of alter2PtConicalUnswap before D33.
func refHighpAlter2PtConicalUnswap(p *highpPipeline) {
	p.r = wide.SplatF32x8(1.0).Sub(p.r)
}

// refHighpNegateX is the body of negateX before D33.
func refHighpNegateX(p *highpPipeline) {
	p.r = p.r.Neg()
}

// refHighpApplyConcentricScaleBias is the body of applyConcentricScaleBias before D33.
func refHighpApplyConcentricScaleBias(p *highpPipeline) {
	ctx := &p.ctx.twoPointConicalGradient

	// Apply t = t * scale + bias for concentric gradients
	x := p.r
	p.r = x.Mul(wide.SplatF32x8(ctx.p0)).Add(wide.SplatF32x8(ctx.p1))
}

// refHighpGammaExpand2 is the body of gammaExpand2 before D33.
func refHighpGammaExpand2(p *highpPipeline) {
	p.r = p.r.Mul(p.r)
	p.g = p.g.Mul(p.g)
	p.b = p.b.Mul(p.b)
}

// refHighpGammaExpandDst2 is the body of gammaExpandDst2 before D33.
func refHighpGammaExpandDst2(p *highpPipeline) {
	p.dr = p.dr.Mul(p.dr)
	p.dg = p.dg.Mul(p.dg)
	p.db = p.db.Mul(p.db)
}

// refHighpGammaCompress2 is the body of gammaCompress2 before D33.
func refHighpGammaCompress2(p *highpPipeline) {
	p.r = p.r.Sqrt()
	p.g = p.g.Sqrt()
	p.b = p.b.Sqrt()
}

// refHighpGammaExpand22 is the body of gammaExpand22 before D33.
func refHighpGammaExpand22(p *highpPipeline) {
	p.r = p.r.Powf(2.2)
	p.g = p.g.Powf(2.2)
	p.b = p.b.Powf(2.2)
}

// refHighpGammaExpandDst22 is the body of gammaExpandDst22 before D33.
func refHighpGammaExpandDst22(p *highpPipeline) {
	p.dr = p.dr.Powf(2.2)
	p.dg = p.dg.Powf(2.2)
	p.db = p.db.Powf(2.2)
}

// refHighpGammaCompress22 is the body of gammaCompress22 before D33.
func refHighpGammaCompress22(p *highpPipeline) {
	p.r = p.r.Powf(0.45454545)
	p.g = p.g.Powf(0.45454545)
	p.b = p.b.Powf(0.45454545)
}

// refHighpSrgbExpand is the body of srgbExpand before D33.
func refHighpSrgbExpand(x wide.F32x8) wide.F32x8 {
	small := x.CmpLe(wide.SplatF32x8(0.04045))
	linear := x.Div(wide.SplatF32x8(12.92))
	exp := x.Add(wide.SplatF32x8(0.055)).Div(wide.SplatF32x8(1.055)).Powf(2.4)
	return small.Blend(linear, exp)
}

// refHighpSrgbCompress is the body of srgbCompress before D33.
func refHighpSrgbCompress(x wide.F32x8) wide.F32x8 {
	small := x.CmpLe(wide.SplatF32x8(0.0031308))
	linear := x.Mul(wide.SplatF32x8(12.92))
	exp := x.Powf(0.416666666).Mul(wide.SplatF32x8(1.055)).Sub(wide.SplatF32x8(0.055))
	return small.Blend(linear, exp)
}

// refHighpGammaExpandSrgb is the body of gammaExpandSrgb before D33.
func refHighpGammaExpandSrgb(p *highpPipeline) {
	p.r = refHighpSrgbExpand(p.r)
	p.g = refHighpSrgbExpand(p.g)
	p.b = refHighpSrgbExpand(p.b)
}

// refHighpGammaExpandDstSrgb is the body of gammaExpandDstSrgb before D33.
func refHighpGammaExpandDstSrgb(p *highpPipeline) {
	p.dr = refHighpSrgbExpand(p.dr)
	p.dg = refHighpSrgbExpand(p.dg)
	p.db = refHighpSrgbExpand(p.db)
}

// refHighpGammaCompressSrgb is the body of gammaCompressSrgb before D33.
func refHighpGammaCompressSrgb(p *highpPipeline) {
	p.r = refHighpSrgbCompress(p.r)
	p.g = refHighpSrgbCompress(p.g)
	p.b = refHighpSrgbCompress(p.b)
}

// refHighpLoad8888Highp is the body of load8888Highp before D33.
func refHighpLoad8888Highp(
	data *[highpStageWidth * BytesPerPixel]byte,
	r, g, b, a *wide.F32x8,
) {
	// Surprisingly, `f32 * FACTOR` is way faster than `f32x8 * f32x8::splat(FACTOR)`.

	const factor float32 = 1.0 / 255.0

	for i := range highpStageWidth {
		r[i] = float32(data[i*BytesPerPixel]) * factor
		g[i] = float32(data[i*BytesPerPixel+1]) * factor
		b[i] = float32(data[i*BytesPerPixel+2]) * factor
		a[i] = float32(data[i*BytesPerPixel+3]) * factor
	}
}

// refHighpLoad8888TailHighp is the body of load8888TailHighp before D33.
func refHighpLoad8888TailHighp(
	tail int, data PixelSlice,
	r, g, b, a *wide.F32x8,
) {
	// Fill a dummy array with `tail` values. `tail` is always in a 1..STAGE_WIDTH-1 range.
	// This way we can reuse the `load_8888_` method and remove any branches.
	var tmp [highpStageWidth * BytesPerPixel]byte
	copy(tmp[0:tail*BytesPerPixel], data[0:tail*BytesPerPixel])
	refHighpLoad8888Highp(&tmp, r, g, b, a)
}

// refHighpStore8888Highp is the body of store8888Highp before D33.
func refHighpStore8888Highp(
	r, g, b, a *wide.F32x8,
	data *[highpStageWidth * BytesPerPixel]byte,
) {
	ri := refHighpUnnorm(r)
	gi := refHighpUnnorm(g)
	bi := refHighpUnnorm(b)
	ai := refHighpUnnorm(a)

	for i := range highpStageWidth {
		data[i*BytesPerPixel] = uint8(ri[i])
		data[i*BytesPerPixel+1] = uint8(gi[i])
		data[i*BytesPerPixel+2] = uint8(bi[i])
		data[i*BytesPerPixel+3] = uint8(ai[i])
	}
}

// refHighpStore8888TailHighp is the body of store8888TailHighp before D33.
func refHighpStore8888TailHighp(
	r, g, b, a *wide.F32x8,
	tail int, data PixelSlice,
) {
	ri := refHighpUnnorm(r)
	gi := refHighpUnnorm(g)
	bi := refHighpUnnorm(b)
	ai := refHighpUnnorm(a)

	// This is better than `for i in 0..tail`, because this way the compiler
	// knows that we have only 4 steps and slices access is guarantee to be valid.
	// This removes bounds checking and a possible panic call.
	for i := range highpStageWidth {
		data.Set(i, premultipliedColorU8FromRGBAUnchecked(
			uint8(ri[i]), uint8(gi[i]), uint8(bi[i]), uint8(ai[i]),
		))

		if i+1 == tail {
			break
		}
	}
}

// refHighpUnnorm is the body of unnorm before D33.
func refHighpUnnorm(v *wide.F32x8) wide.I32x8 {
	return v.Max(wide.F32x8{}).Min(wide.SplatF32x8(1.0)).Mul(wide.SplatF32x8(255.0)).RoundInt()
}

// refHighpInvHighp is the body of invHighp before D33.
func refHighpInvHighp(v wide.F32x8) wide.F32x8 {
	return wide.SplatF32x8(1.0).Sub(v)
}

// refHighpTwo is the body of two before D33.
func refHighpTwo(v wide.F32x8) wide.F32x8 {
	return v.Add(v)
}

// refHighpMadHighp is the body of madHighp before D33.
func refHighpMadHighp(f, m, a wide.F32x8) wide.F32x8 {
	return f.Mul(m).Add(a)
}

// refHighpLerpHighp is the body of lerpHighp before D33.
func refHighpLerpHighp(from, to, t wide.F32x8) wide.F32x8 {
	return refHighpMadHighp(to.Sub(from), t, from)
}
