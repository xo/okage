// Code generated from pipeline_lowp.go for TestLowpFused. DO NOT EDIT.

package okage

import "github.com/xo/okage/internal/wide"

var _ = wide.SplatU16x16

func refLowpSourceAtop(p *lowpPipeline) {
	p.r = div255(p.r.Mul(p.da).Add(p.dr.Mul(invLowp(p.a))))
	p.g = div255(p.g.Mul(p.da).Add(p.dg.Mul(invLowp(p.a))))
	p.b = div255(p.b.Mul(p.da).Add(p.db.Mul(invLowp(p.a))))
	p.a = div255(p.a.Mul(p.da).Add(p.da.Mul(invLowp(p.a))))

}

func refLowpDestinationAtop(p *lowpPipeline) {
	p.r = div255(p.dr.Mul(p.a).Add(p.r.Mul(invLowp(p.da))))
	p.g = div255(p.dg.Mul(p.a).Add(p.g.Mul(invLowp(p.da))))
	p.b = div255(p.db.Mul(p.a).Add(p.b.Mul(invLowp(p.da))))
	p.a = div255(p.da.Mul(p.a).Add(p.a.Mul(invLowp(p.da))))

}

func refLowpSourceIn(p *lowpPipeline) {
	p.r = div255(p.r.Mul(p.da))
	p.g = div255(p.g.Mul(p.da))
	p.b = div255(p.b.Mul(p.da))
	p.a = div255(p.a.Mul(p.da))

}

func refLowpDestinationIn(p *lowpPipeline) {
	p.r = div255(p.dr.Mul(p.a))
	p.g = div255(p.dg.Mul(p.a))
	p.b = div255(p.db.Mul(p.a))
	p.a = div255(p.da.Mul(p.a))

}

func refLowpSourceOut(p *lowpPipeline) {
	p.r = div255(p.r.Mul(invLowp(p.da)))
	p.g = div255(p.g.Mul(invLowp(p.da)))
	p.b = div255(p.b.Mul(invLowp(p.da)))
	p.a = div255(p.a.Mul(invLowp(p.da)))

}

func refLowpDestinationOut(p *lowpPipeline) {
	p.r = div255(p.dr.Mul(invLowp(p.a)))
	p.g = div255(p.dg.Mul(invLowp(p.a)))
	p.b = div255(p.db.Mul(invLowp(p.a)))
	p.a = div255(p.da.Mul(invLowp(p.a)))

}

func refLowpSourceOver(p *lowpPipeline) {
	p.r = p.r.Add(div255(p.dr.Mul(invLowp(p.a))))
	p.g = p.g.Add(div255(p.dg.Mul(invLowp(p.a))))
	p.b = p.b.Add(div255(p.db.Mul(invLowp(p.a))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

}

func refLowpDestinationOver(p *lowpPipeline) {
	p.r = p.dr.Add(div255(p.r.Mul(invLowp(p.da))))
	p.g = p.dg.Add(div255(p.g.Mul(invLowp(p.da))))
	p.b = p.db.Add(div255(p.b.Mul(invLowp(p.da))))
	p.a = p.da.Add(div255(p.a.Mul(invLowp(p.da))))

}

func refLowpModulate(p *lowpPipeline) {
	p.r = div255(p.r.Mul(p.dr))
	p.g = div255(p.g.Mul(p.dg))
	p.b = div255(p.b.Mul(p.db))
	p.a = div255(p.a.Mul(p.da))

}

func refLowpMultiply(p *lowpPipeline) {
	p.r = div255(p.r.Mul(invLowp(p.da)).Add(p.dr.Mul(invLowp(p.a))).Add(p.r.Mul(p.dr)))
	p.g = div255(p.g.Mul(invLowp(p.da)).Add(p.dg.Mul(invLowp(p.a))).Add(p.g.Mul(p.dg)))
	p.b = div255(p.b.Mul(invLowp(p.da)).Add(p.db.Mul(invLowp(p.a))).Add(p.b.Mul(p.db)))
	p.a = div255(p.a.Mul(invLowp(p.da)).Add(p.da.Mul(invLowp(p.a))).Add(p.a.Mul(p.da)))

}

func refLowpScreen(p *lowpPipeline) {
	p.r = p.r.Add(p.dr).Sub(div255(p.r.Mul(p.dr)))
	p.g = p.g.Add(p.dg).Sub(div255(p.g.Mul(p.dg)))
	p.b = p.b.Add(p.db).Sub(div255(p.b.Mul(p.db)))
	p.a = p.a.Add(p.da).Sub(div255(p.a.Mul(p.da)))

}

func refLowpXor(p *lowpPipeline) {
	p.r = div255(p.r.Mul(invLowp(p.da)).Add(p.dr.Mul(invLowp(p.a))))
	p.g = div255(p.g.Mul(invLowp(p.da)).Add(p.dg.Mul(invLowp(p.a))))
	p.b = div255(p.b.Mul(invLowp(p.da)).Add(p.db.Mul(invLowp(p.a))))
	p.a = div255(p.a.Mul(invLowp(p.da)).Add(p.da.Mul(invLowp(p.a))))

}

func refLowpPlus(p *lowpPipeline) {
	p.r = p.r.Add(p.dr).Min(wide.SplatU16x16(255))
	p.g = p.g.Add(p.dg).Min(wide.SplatU16x16(255))
	p.b = p.b.Add(p.db).Min(wide.SplatU16x16(255))
	p.a = p.a.Add(p.da).Min(wide.SplatU16x16(255))

}

func refLowpDarken(p *lowpPipeline) {
	// The same logic applied to color, and source_over for alpha.
	p.r = p.r.Add(p.dr).Sub(div255(p.r.Mul(p.da).Max(p.dr.Mul(p.a))))
	p.g = p.g.Add(p.dg).Sub(div255(p.g.Mul(p.da).Max(p.dg.Mul(p.a))))
	p.b = p.b.Add(p.db).Sub(div255(p.b.Mul(p.da).Max(p.db.Mul(p.a))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

}

func refLowpLighten(p *lowpPipeline) {
	// The same logic applied to color, and source_over for alpha.
	p.r = p.r.Add(p.dr).Sub(div255(p.r.Mul(p.da).Min(p.dr.Mul(p.a))))
	p.g = p.g.Add(p.dg).Sub(div255(p.g.Mul(p.da).Min(p.dg.Mul(p.a))))
	p.b = p.b.Add(p.db).Sub(div255(p.b.Mul(p.da).Min(p.db.Mul(p.a))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

}

func refLowpExclusion(p *lowpPipeline) {
	// The same logic applied to color, and source_over for alpha.
	p.r = p.r.Add(p.dr).Sub(wide.SplatU16x16(2).Mul(div255(p.r.Mul(p.dr))))
	p.g = p.g.Add(p.dg).Sub(wide.SplatU16x16(2).Mul(div255(p.g.Mul(p.dg))))
	p.b = p.b.Add(p.db).Sub(wide.SplatU16x16(2).Mul(div255(p.b.Mul(p.db))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

}

func refLowpDifference(p *lowpPipeline) {
	// The same logic applied to color, and source_over for alpha.
	p.r = p.r.Add(p.dr).Sub(wide.SplatU16x16(2).Mul(div255(p.r.Mul(p.da).Min(p.dr.Mul(p.a)))))
	p.g = p.g.Add(p.dg).Sub(wide.SplatU16x16(2).Mul(div255(p.g.Mul(p.da).Min(p.dg.Mul(p.a)))))
	p.b = p.b.Add(p.db).Sub(wide.SplatU16x16(2).Mul(div255(p.b.Mul(p.da).Min(p.db.Mul(p.a)))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

}

func refLowpHardLight(p *lowpPipeline) {
	// The same logic applied to color, and source_over for alpha.
	p.r = div255(p.r.Mul(invLowp(p.da)).Add(p.dr.Mul(invLowp(p.a))).Add(p.r.Add(p.r).CmpLe(p.a).Blend(wide.SplatU16x16(2).Mul(p.r).Mul(p.dr), p.a.Mul(p.da).Sub(wide.SplatU16x16(2).Mul(p.a.Sub(p.r)).Mul(p.da.Sub(p.dr))))))
	p.g = div255(p.g.Mul(invLowp(p.da)).Add(p.dg.Mul(invLowp(p.a))).Add(p.g.Add(p.g).CmpLe(p.a).Blend(wide.SplatU16x16(2).Mul(p.g).Mul(p.dg), p.a.Mul(p.da).Sub(wide.SplatU16x16(2).Mul(p.a.Sub(p.g)).Mul(p.da.Sub(p.dg))))))
	p.b = div255(p.b.Mul(invLowp(p.da)).Add(p.db.Mul(invLowp(p.a))).Add(p.b.Add(p.b).CmpLe(p.a).Blend(wide.SplatU16x16(2).Mul(p.b).Mul(p.db), p.a.Mul(p.da).Sub(wide.SplatU16x16(2).Mul(p.a.Sub(p.b)).Mul(p.da.Sub(p.db))))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

}

func refLowpOverlay(p *lowpPipeline) {
	// The same logic applied to color, and source_over for alpha.
	p.r = div255(p.r.Mul(invLowp(p.da)).Add(p.dr.Mul(invLowp(p.a))).Add(p.dr.Add(p.dr).CmpLe(p.da).Blend(wide.SplatU16x16(2).Mul(p.r).Mul(p.dr), p.a.Mul(p.da).Sub(wide.SplatU16x16(2).Mul(p.a.Sub(p.r)).Mul(p.da.Sub(p.dr))))))
	p.g = div255(p.g.Mul(invLowp(p.da)).Add(p.dg.Mul(invLowp(p.a))).Add(p.dg.Add(p.dg).CmpLe(p.da).Blend(wide.SplatU16x16(2).Mul(p.g).Mul(p.dg), p.a.Mul(p.da).Sub(wide.SplatU16x16(2).Mul(p.a.Sub(p.g)).Mul(p.da.Sub(p.dg))))))
	p.b = div255(p.b.Mul(invLowp(p.da)).Add(p.db.Mul(invLowp(p.a))).Add(p.db.Add(p.db).CmpLe(p.da).Blend(wide.SplatU16x16(2).Mul(p.b).Mul(p.db), p.a.Mul(p.da).Sub(wide.SplatU16x16(2).Mul(p.a.Sub(p.b)).Mul(p.da.Sub(p.db))))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

}

func refLowpPremultiply(p *lowpPipeline) {
	p.r = div255(p.r.Mul(p.a))
	p.g = div255(p.g.Mul(p.a))
	p.b = div255(p.b.Mul(p.a))

}

func refLowpMaskU8(p *lowpPipeline) {
	offset := p.maskCtx.offset(p.dx, p.dy)

	var c wide.U16x16
	for i := range p.tail {
		c[i] = uint16(p.maskCtx.data[offset+i])
	}

	if c.Equal(wide.U16x16{}) {
		return
	}

	p.r = div255(p.r.Mul(c))
	p.g = div255(p.g.Mul(c))
	p.b = div255(p.b.Mul(c))
	p.a = div255(p.a.Mul(c))

}

func refLowpScaleU8(p *lowpPipeline) {
	// Load u8xTail and cast it to u16x16.
	data := p.aaMaskCtx.copyAtXY(p.dx, p.dy, p.tail)
	c := wide.U16x16{
		uint16(data[0]),
		uint16(data[1]),
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
	}

	p.r = div255(p.r.Mul(c))
	p.g = div255(p.g.Mul(c))
	p.b = div255(p.b.Mul(c))
	p.a = div255(p.a.Mul(c))

}

func refLowpLerpU8(p *lowpPipeline) {
	// Load u8xTail and cast it to u16x16.
	data := p.aaMaskCtx.copyAtXY(p.dx, p.dy, p.tail)
	c := wide.U16x16{
		uint16(data[0]),
		uint16(data[1]),
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
	}

	p.r = lerpLowp(p.dr, p.r, c)
	p.g = lerpLowp(p.dg, p.g, c)
	p.b = lerpLowp(p.db, p.b, c)
	p.a = lerpLowp(p.da, p.a, c)

}

func refLowpScale1Float(p *lowpPipeline) {
	c := fromFloat(p.ctx.currentCoverage)
	p.r = div255(p.r.Mul(c))
	p.g = div255(p.g.Mul(c))
	p.b = div255(p.b.Mul(c))
	p.a = div255(p.a.Mul(c))

}

func refLowpLerp1Float(p *lowpPipeline) {
	c := fromFloat(p.ctx.currentCoverage)
	p.r = lerpLowp(p.dr, p.r, c)
	p.g = lerpLowp(p.dg, p.g, c)
	p.b = lerpLowp(p.db, p.b, c)
	p.a = lerpLowp(p.da, p.a, c)

}

func refLowpSourceOverRgba(p *lowpPipeline) {
	pixels := p.pixmap.slice16AtXY(p.dx, p.dy)
	load8888Lowp(pixels, &p.dr, &p.dg, &p.db, &p.da)
	p.r = p.r.Add(div255(p.dr.Mul(invLowp(p.a))))
	p.g = p.g.Add(div255(p.dg.Mul(invLowp(p.a))))
	p.b = p.b.Add(div255(p.db.Mul(invLowp(p.a))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))
	store8888Lowp(&p.r, &p.g, &p.b, &p.a, pixels)

}

func refLowpSourceOverRgbaTail(p *lowpPipeline) {
	pixels := p.pixmap.sliceAtXY(p.dx, p.dy)
	load8888TailLowp(p.tail, pixels, &p.dr, &p.dg, &p.db, &p.da)
	p.r = p.r.Add(div255(p.dr.Mul(invLowp(p.a))))
	p.g = p.g.Add(div255(p.dg.Mul(invLowp(p.a))))
	p.b = p.b.Add(div255(p.db.Mul(invLowp(p.a))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))
	store8888TailLowp(&p.r, &p.g, &p.b, &p.a, p.tail, pixels)

}
