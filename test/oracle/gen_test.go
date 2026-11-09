package oracle

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// gen makes random scenes and random questions of the package path. Every
// value is a float32, so that okage and the oracle read the same numbers.
type gen struct {
	r *rand.Rand
}

// f returns a random value in [lo, hi). One value in four is rounded to a
// quarter, so that edges fall on the borders of pixels and of sub-pixels.
func (g *gen) f(lo, hi float64) num {
	v := float32(lo + g.r.Float64()*(hi-lo))
	if g.r.IntN(4) == 0 {
		v = float32(math.Round(float64(v)*4) / 4)
	}
	return num(v)
}

// unit returns a random value in [0, 1).
func (g *gen) unit() num {
	return num(float32(g.r.Float64()))
}

var identity = [6]num{1, 0, 0, 1, 0, 0}

func (g *gen) transform(w, h float64) [6]num {
	switch g.r.IntN(6) {
	case 0, 1:
		return identity
	case 2:
		return [6]num{1, 0, 0, 1, g.f(-5, 5), g.f(-5, 5)}
	case 3:
		return [6]num{g.f(0.3, 2), 0, 0, g.f(0.3, 2), g.f(-5, 5), g.f(-5, 5)}
	case 4:
		a := g.r.Float64() * 2 * math.Pi
		s := float64(g.f(0.5, 1.5))
		c, si := num(math.Cos(a)*s), num(math.Sin(a)*s)
		return [6]num{c, si, -si, c, num(w / 2), num(h / 2)}
	default:
		return [6]num{g.f(0.5, 1.5), g.f(-0.5, 0.5), g.f(-0.5, 0.5), g.f(0.5, 1.5), g.f(-5, 5), g.f(-5, 5)}
	}
}

func (g *gen) color() *[4]num {
	a := g.unit()
	if g.r.IntN(3) == 0 {
		a = 1
	}
	return &[4]num{g.unit(), g.unit(), g.unit(), a}
}

func (g *gen) stops() [][5]num {
	n := 2 + g.r.IntN(4)
	if g.r.IntN(8) == 0 {
		n = 1
	}
	out := make([][5]num, 0, n)
	pos := num(0)
	for i := range n {
		var p num
		switch g.r.IntN(4) {
		case 0:
			p = num(float32(i) / float32(max(n-1, 1)))
		case 1:
			p = pos // a hard stop
		default:
			p = pos + g.unit()*0.5
		}
		if g.r.IntN(10) == 0 {
			p = g.f(-0.2, 1.2)
		}
		pos = min(p, 1)
		c := g.color()
		out = append(out, [5]num{p, c[0], c[1], c[2], c[3]})
	}
	return out
}

func (g *gen) shader(w, h float64, nsrc int) shaderSpec {
	k := g.r.IntN(5)
	if nsrc == 0 && k == 4 {
		k = 0
	}
	switch k {
	case 0:
		return shaderSpec{Kind: "solid", Color: g.color(), Transform: identity}
	case 1:
		s := [2]num{g.f(-10, w+10), g.f(-10, h+10)}
		e := [2]num{g.f(-10, w+10), g.f(-10, h+10)}
		sp := uint8(g.r.IntN(3))
		if g.r.IntN(10) == 0 {
			// A degenerate gradient uses Pad only, because tiny-skia panics
			// in average_gradient_color for some stops with the other modes.
			e = s
			sp = 0
		}
		return shaderSpec{Kind: "linear", Start: &s, End: &e, Stops: g.stops(), Spread: sp, Transform: g.transform(w, h)}
	case 2:
		c0 := [2]num{g.f(0, w), g.f(0, h)}
		c1 := c0
		r0 := num(0)
		r1 := g.f(1, w)
		switch g.r.IntN(7) {
		case 0: // simple radial
		case 1: // concentric
			r0 = g.f(0.5, w/2)
		case 2: // strip
			c1 = [2]num{g.f(0, w), g.f(0, h)}
			r0 = r1
		case 3: // natively focal
			c1 = [2]num{g.f(0, w), g.f(0, h)}
		case 4: // focal on circle
			c1 = [2]num{c0[0] + 10, c0[1]}
			r1 = 10
		default: // general two point
			c1 = [2]num{g.f(0, w), g.f(0, h)}
			r0 = g.f(0, w/2)
		}
		if g.r.IntN(2) == 0 {
			c0, c1 = c1, c0
			r0, r1 = r1, r0
		}
		return shaderSpec{
			Kind: "radial", Start: &c0, StartRadius: r0, End: &c1, EndRadius: r1,
			Stops: g.stops(), Spread: uint8(g.r.IntN(3)), Transform: g.transform(w, h),
		}
	case 3:
		c := [2]num{g.f(0, w), g.f(0, h)}
		a0 := g.f(-90, 270)
		a1 := a0 + g.f(0, 360)
		switch g.r.IntN(5) {
		case 0:
			a0, a1 = 0, 360
		case 1:
			a1 = a0
		}
		return shaderSpec{
			Kind: "sweep", Center: &c, StartAngle: a0, EndAngle: a1,
			Stops: g.stops(), Spread: uint8(g.r.IntN(3)), Transform: g.transform(w, h),
		}
	default:
		o := num(1)
		if g.r.IntN(2) == 0 {
			o = g.unit()
		}
		return shaderSpec{
			Kind: "pattern", Source: g.r.IntN(nsrc), Spread: uint8(g.r.IntN(3)),
			Quality: uint8(g.r.IntN(3)), Opacity: o, Transform: g.transform(w, h),
		}
	}
}

func (g *gen) paint(w, h float64, nsrc int) *paintSpec {
	p := &paintSpec{
		Shader:    g.shader(w, h, nsrc),
		BlendMode: 3, // SourceOver
		AntiAlias: g.r.IntN(3) != 0,
		ForceHQ:   g.r.IntN(3) == 0,
	}
	if g.r.IntN(3) == 0 {
		p.BlendMode = uint8(g.r.IntN(len(blends)))
	}
	if g.r.IntN(5) == 0 {
		p.Colorspace = uint8(1 + g.r.IntN(3))
	}
	if g.r.IntN(6) == 0 {
		o := g.unit()
		p.Opacity = &o
	}
	return p
}

// path returns a random path of one or two contours in a box a little
// larger than w by h. Each segment is a line, a quad or a cubic. If cubics
// is false, the path has no cubic.
func (g *gen) path(w, h float64, cubics bool) []segment {
	var segs []segment
	nc := 1 + g.r.IntN(2)
	for range nc {
		segs = append(segs, segment{verbMove, g.f(-5, w+5), g.f(-5, h+5)})
		n := 2 + g.r.IntN(5)
		for range n {
			k := g.r.IntN(4)
			if !cubics && k == 3 {
				k = 2
			}
			switch k {
			case 0, 1:
				segs = append(segs, segment{verbLine, g.f(-5, w+5), g.f(-5, h+5)})
			case 2:
				segs = append(segs, segment{verbQuad, g.f(-5, w+5), g.f(-5, h+5), g.f(-5, w+5), g.f(-5, h+5)})
			default:
				segs = append(segs, segment{
					verbCubic, g.f(-5, w+5), g.f(-5, h+5), g.f(-5, w+5), g.f(-5, h+5), g.f(-5, w+5), g.f(-5, h+5),
				})
			}
		}
		if g.r.IntN(2) == 0 {
			segs = append(segs, segment{verbClose})
		}
	}
	return segs
}

// circle returns a circle of four cubic curves.
func circle(cx, cy, r num) []segment {
	k := 0.5522847498 * r
	return []segment{
		{verbMove, cx + r, cy},
		{verbCubic, cx + r, cy + k, cx + k, cy + r, cx, cy + r},
		{verbCubic, cx - k, cy + r, cx - r, cy + k, cx - r, cy},
		{verbCubic, cx - r, cy - k, cx - k, cy - r, cx, cy - r},
		{verbCubic, cx + k, cy - r, cx + r, cy - k, cx + r, cy},
		{verbClose},
	}
}

// stroke returns a random stroke. If dash is true, it always has a dash.
func (g *gen) stroke(dash bool) *strokeSpec {
	widths := []num{0, 0.3, 0.7, 1, 1.5, 3, 7.5}
	st := &strokeSpec{
		Width: widths[g.r.IntN(len(widths))],
		Miter: 4,
		Cap:   uint8(g.r.IntN(3)),
		Join:  uint8(g.r.IntN(4)),
	}
	if g.r.IntN(4) == 0 {
		st.Miter = g.f(1, 10)
	}
	if dash || g.r.IntN(4) == 0 {
		st.Dash = &dashSpec{Array: []num{g.f(1, 6), g.f(1, 6)}, Offset: g.f(0, 5)}
		if g.r.IntN(3) == 0 {
			st.Dash.Array = append(st.Dash.Array, g.f(0.5, 4), g.f(0.5, 4))
		}
	}
	return st
}

// scene returns a random scene. One scene in two has no cubic curve, so
// that both classes of D26 have many scenes.
func (g *gen) scene(i int) scene {
	w := uint32(8 + g.r.IntN(60))
	h := uint32(8 + g.r.IntN(50))
	fw, fh := float64(w), float64(h)
	cubics := g.r.IntN(2) == 0
	sc := scene{Width: w, Height: h, Name: fmt.Sprintf("random-%d", i)}
	nsrc := g.r.IntN(3)
	for range nsrc {
		sc.Sources = append(sc.Sources, source{Width: uint32(1 + g.r.IntN(20)), Height: uint32(1 + g.r.IntN(20)), Seed: g.r.Uint32()})
	}
	nm := 0
	if g.r.IntN(3) == 0 {
		nm = 1 + g.r.IntN(2)
	}
	for range nm {
		m := maskSpec{Transform: identity}
		if nsrc > 0 && g.r.IntN(3) == 0 {
			// A mask from a source with the size of the pixmap.
			sc.Sources = append(sc.Sources, source{Width: w, Height: h, Seed: g.r.Uint32()})
			m.FromSource = new(len(sc.Sources) - 1)
			m.Luminance = g.r.IntN(2) == 0
		} else {
			m.Path = g.path(fw, fh, cubics)
			m.EvenOdd = g.r.IntN(2) == 0
			m.AntiAlias = g.r.IntN(3) != 0
			m.Transform = g.transform(fw, fh)
		}
		if cubics && g.r.IntN(4) == 0 {
			m.Intersect = circle(num(fw/2), num(fh/2), g.f(2, fw/2))
		}
		m.Invert = g.r.IntN(4) == 0
		sc.Masks = append(sc.Masks, m)
	}
	nsrc = len(sc.Sources)
	if g.r.IntN(2) == 0 {
		sc.Ops = append(sc.Ops, op{Kind: "fill", Color: g.color(), Transform: identity})
	}
	nops := 1 + g.r.IntN(4)
	for range nops {
		var mask *int
		if nm > 0 && g.r.IntN(2) == 0 {
			mask = new(g.r.IntN(nm))
		}
		o := op{Transform: identity, Mask: mask}
		k := g.r.IntN(6)
		if nsrc == 0 && k == 4 {
			k = 0
		}
		switch k {
		case 0, 1:
			o.Kind = "fill_path"
			o.Path = g.path(fw, fh, cubics)
			if cubics && g.r.IntN(3) == 0 {
				o.Path = circle(g.f(0, fw), g.f(0, fh), g.f(1, fw))
			}
			o.Paint = g.paint(fw, fh, nsrc)
			o.EvenOdd = g.r.IntN(2) == 0
			o.Transform = g.transform(fw, fh)
		case 2:
			o.Kind = "stroke_path"
			o.Path = g.path(fw, fh, cubics)
			o.Paint = g.paint(fw, fh, nsrc)
			o.Stroke = g.stroke(false)
			o.Transform = g.transform(fw, fh)
		case 3:
			o.Kind = "fill_rect"
			o.Rect = &[4]num{g.f(-5, fw), g.f(-5, fh), g.f(0.5, fw), g.f(0.5, fh)}
			o.Paint = g.paint(fw, fh, nsrc)
			if g.r.IntN(2) == 0 {
				o.Transform = g.transform(fw, fh)
			}
		case 4:
			o.Kind = "draw_pixmap"
			o.X = int32(g.r.IntN(int(w)) - 5)
			o.Y = int32(g.r.IntN(int(h)) - 5)
			o.Source = g.r.IntN(nsrc)
			o.Opacity = 1
			if g.r.IntN(2) == 0 {
				o.Opacity = g.unit()
			}
			o.BlendMode = 3
			if g.r.IntN(3) == 0 {
				o.BlendMode = uint8(g.r.IntN(len(blends)))
			}
			o.Quality = uint8(g.r.IntN(3))
			if g.r.IntN(2) == 0 {
				o.Transform = g.transform(fw, fh)
			}
		default:
			// apply_mask needs a mask with the size of the pixmap.
			if j := g.sameSizeMask(&sc); nm > 0 && j >= 0 && g.r.IntN(2) == 0 {
				o = op{Kind: "apply_mask", Mask: new(j), Transform: identity}
				break
			}
			o.Kind = "fill_path"
			o.Path = g.path(fw, fh, cubics)
			o.Paint = g.paint(fw, fh, nsrc)
		}
		sc.Ops = append(sc.Ops, o)
	}
	return sc
}

// sameSizeMask returns the index of the first mask of sc with the size of
// the pixmap, or -1.
func (g *gen) sameSizeMask(sc *scene) int {
	for j, m := range sc.Masks {
		if m.FromSource == nil {
			return j
		}
		if s := sc.Sources[*m.FromSource]; s.Width == sc.Width && s.Height == sc.Height {
			return j
		}
	}
	return -1
}

// pathQuestion returns a random question of the package path. One question
// in two has no cubic curve.
func (g *gen) pathQuestion(i int) pathQuestion {
	q := pathQuestion{
		Path:      g.path(100, 100, g.r.IntN(2) == 0),
		ResScale:  []num{1, 0.5, 2, 4, 0.25}[g.r.IntN(5)],
		Transform: identity,
		Name:      fmt.Sprintf("path-%d", i),
	}
	switch g.r.IntN(3) {
	case 0:
		q.PathOp = "stroke"
		q.Stroke = g.stroke(false)
	case 1:
		q.PathOp = "dash"
		q.Stroke = g.stroke(true)
	default:
		q.PathOp = "transform"
		q.Transform = g.transform(100, 100)
	}
	return q
}

// structured returns scenes that cover each blend mode, each pattern mode
// and the tiler of large pixmaps, which random scenes reach seldom.
func structured() []scene {
	var out []scene
	bg := func() []op {
		lin := shaderSpec{
			Kind: "linear", Start: &[2]num{0, 0}, End: &[2]num{37, 29},
			Stops:     [][5]num{{0, 1, 0, 0, 1}, {0.5, 0, 1, 0, 0.5}, {1, 0, 0, 1, 0.8}},
			Transform: identity,
		}
		return []op{
			{Kind: "fill", Color: &[4]num{0.2, 0.5, 0.6, 0.8}, Transform: identity},
			{Kind: "fill_rect", Rect: &[4]num{3, 2, 30, 20}, Paint: &paintSpec{Shader: lin, BlendMode: 3, AntiAlias: true}, Transform: identity},
		}
	}
	square := []segment{{verbMove, 6.5, 3.25}, {verbLine, 30.75, 4}, {verbQuad, 33, 15, 29.5, 26}, {verbLine, 5, 25.5}, {verbClose}}
	// Each blend mode, with lowp and highp, solid and gradient, with and
	// without anti-aliasing.
	for bm := range blends {
		for _, hq := range []bool{false, true} {
			for _, grad := range []bool{false, true} {
				for _, aa := range []bool{false, true} {
					sh := shaderSpec{Kind: "solid", Color: &[4]num{0.86, 0.55, 0.29, 0.7}, Transform: identity}
					if grad {
						sh = shaderSpec{
							Kind: "radial", Start: &[2]num{18, 14}, End: &[2]num{18, 14}, EndRadius: 15,
							Stops: [][5]num{{0, 1, 1, 0, 1}, {1, 0, 0.5, 1, 0.4}}, Transform: identity,
						}
					}
					paint := &paintSpec{Shader: sh, BlendMode: uint8(bm), AntiAlias: aa, ForceHQ: hq}
					ops := append(bg(),
						op{Kind: "fill_path", Path: square, Paint: paint, Transform: identity},
						op{Kind: "fill_rect", Rect: &[4]num{1.5, 20.25, 33.25, 6}, Paint: paint, Transform: identity},
					)
					out = append(out, scene{Width: 37, Height: 29, Ops: ops, Name: fmt.Sprintf("blend-%v-hq%v-grad%v-aa%v", blends[bm], hq, grad, aa)})
				}
			}
		}
	}
	// Patterns: each spread mode and filter quality, with a set of
	// transforms.
	tss := [][6]num{identity, {1, 0, 0, 1, 3, 2}, {1, 0, 0, 1, 2.5, 1.25}, {2.5, 0, 0, 1.75, 1, 1}, {0.866, 0.5, -0.5, 0.866, 10, 2}, {1.25, 0.3, -0.2, 0.9, 0, 0}}
	for sp := range 3 {
		for q := range 3 {
			for ti, t := range tss {
				for _, opacity := range []num{1, 0.6} {
					for _, cs := range []uint8{0, 2} {
						sh := shaderSpec{Kind: "pattern", Source: 0, Spread: uint8(sp), Quality: uint8(q), Opacity: opacity, Transform: t}
						ops := append(bg(),
							op{Kind: "fill_path", Path: square, Paint: &paintSpec{Shader: sh, BlendMode: 3, AntiAlias: true, Colorspace: cs}, Transform: identity},
							op{Kind: "draw_pixmap", X: 2, Y: 3, Source: 1, Opacity: opacity, BlendMode: 3, Quality: uint8(q), Transform: t},
						)
						out = append(out, scene{
							Width: 37, Height: 29, Sources: []source{{7, 5, 12345}, {9, 6, 999}}, Ops: ops,
							Name: fmt.Sprintf("pattern-s%d-q%d-t%d-o%v-cs%d", sp, q, ti, opacity, cs),
						})
					}
				}
			}
		}
	}
	// The tiler of pixmaps wider than 8191 pixels.
	for i, w := range []uint32{8200, 8300} {
		sh := shaderSpec{
			Kind: "linear", Start: &[2]num{0, 0}, End: &[2]num{8300, 0},
			Stops: [][5]num{{0, 1, 0, 0, 1}, {1, 0, 0, 1, 1}}, Spread: 1, Transform: identity,
		}
		ops := []op{
			{
				Kind: "fill_path", Path: []segment{{verbMove, 10, 1}, {verbLine, 8250, 3}, {verbLine, 8100, 11}, {verbClose}},
				Paint: &paintSpec{Shader: sh, BlendMode: 3, AntiAlias: i == 0}, Transform: identity, Mask: new(0),
			},
			{
				Kind: "stroke_path", Path: []segment{{verbMove, 0, 6}, {verbLine, 8290, 7}},
				Paint:  &paintSpec{Shader: sh, BlendMode: 3, AntiAlias: true},
				Stroke: &strokeSpec{Width: 0.5, Miter: 4}, Transform: identity, Mask: new(0),
			},
			{
				Kind: "stroke_path", Path: []segment{{verbMove, 0, 9}, {verbLine, 8290, 2}},
				Paint:  &paintSpec{Shader: sh, BlendMode: 3, AntiAlias: false},
				Stroke: &strokeSpec{Width: 0, Miter: 4}, Transform: [6]num{1, 0, 0, 1, 0.5, 0.5},
			},
		}
		masks := []maskSpec{{
			Path:      []segment{{verbMove, 0, 0}, {verbLine, 8300, 0}, {verbLine, 8000, 12}, {verbLine, 0, 12}, {verbClose}},
			AntiAlias: true, Transform: identity,
		}}
		out = append(out, scene{Width: w, Height: 12, Masks: masks, Ops: ops, Name: fmt.Sprintf("tiler-%d", w)})
	}
	return out
}
