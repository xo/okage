// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file holds the Sink that draws with okage (D41). It ports no velato
// file. It does what the sinks of velato for Vello do, with the parts of
// okage that svg uses for a group.

import (
	"math"
	"sync"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/kurbo"
	"github.com/xo/okage/path"
)

// PixmapSink is a Sink that draws onto an okage pixmap. Each layer is a
// pixmap of the size of the target, from a pool. A layer keeps the rectangle
// that its draws touch, and PopLayer composites only that rectangle onto the
// layer below, with its blend mode, its opacity and its clip. A clip that is
// a rectangle on the pixel grid needs no mask.
type PixmapSink struct {
	target *okage.PixmapMut
	width  uint32
	height uint32
	stack  []*sinkLayer
}

// pixRect is a rectangle of pixels, from (x0, y0) to (x1, y1), exclusive. It
// is empty when x0 >= x1 or y0 >= y1.
type pixRect struct {
	x0, y0, x1, y1 int
}

// empty reports if r holds no pixel.
func (r pixRect) empty() bool {
	return r.x0 >= r.x1 || r.y0 >= r.y1
}

// union returns the smallest rectangle that holds r and o.
func (r pixRect) union(o pixRect) pixRect {
	if r.empty() {
		return o
	}
	if o.empty() {
		return r
	}
	return pixRect{min(r.x0, o.x0), min(r.y0, o.y0), max(r.x1, o.x1), max(r.y1, o.y1)}
}

// intersect returns the pixels that r and o share.
func (r pixRect) intersect(o pixRect) pixRect {
	return pixRect{max(r.x0, o.x0), max(r.y0, o.y0), min(r.x1, o.x1), min(r.y1, o.y1)}
}

// sinkLayer is a layer of a PixmapSink.
type sinkLayer struct {
	pixmap *okage.Pixmap
	blend  BlendMode
	alpha  float32
	// mask is the clip of the layer, or nil when the clip is the rectangle
	// clip.
	mask *okage.Mask
	// clip holds the pixels that the clip can keep.
	clip pixRect
	// dirty holds the pixels that the draws into the layer touch.
	dirty pixRect
	// filter is the effect of a filter layer, which has no clip, or nil.
	filter *FilterEffect
}

// NewPixmapSink returns a sink that draws onto target.
func NewPixmapSink(target *okage.PixmapMut) *PixmapSink {
	return &PixmapSink{target: target, width: target.Width(), height: target.Height()}
}

// canvas returns the rectangle of the target.
func (s *PixmapSink) canvas() pixRect {
	return pixRect{0, 0, int(s.width), int(s.height)}
}

// current returns the pixmap that draws go to, or nil inside a layer whose
// clip is empty, where nothing shows.
func (s *PixmapSink) current() *okage.PixmapMut {
	if len(s.stack) == 0 {
		return s.target
	}
	p := s.stack[len(s.stack)-1].pixmap
	if p == nil {
		return nil
	}
	m := p.AsMut()
	return &m
}

// touch adds r to the dirty rectangle of the current layer.
func (s *PixmapSink) touch(r pixRect) {
	if len(s.stack) == 0 {
		return
	}
	l := s.stack[len(s.stack)-1]
	l.dirty = l.dirty.union(r.intersect(s.canvas()))
}

// pixmapPool holds pixmaps for the layers of each PixmapSink. A pixmap in
// the pool is transparent.
var pixmapPool sync.Pool

// acquire returns a transparent pixmap of the size of the target.
func (s *PixmapSink) acquire() *okage.Pixmap {
	if p, ok := pixmapPool.Get().(*okage.Pixmap); ok {
		if p.Width() == s.width && p.Height() == s.height {
			return p
		}
	}
	p, ok := okage.NewPixmap(s.width, s.height)
	if !ok {
		return nil
	}
	return p
}

// release clears the pixels of r in p, and puts p in the pool.
func release(p *okage.Pixmap, r pixRect) {
	if !r.empty() {
		data := p.DataMut()
		stride := int(p.Width()) * 4
		for y := r.y0; y < r.y1; y++ {
			clear(data[y*stride+r.x0*4 : y*stride+r.x1*4])
		}
	}
	pixmapPool.Put(p)
}

// PushLayer starts a layer that composites with blend and alpha, clipped to
// shape under transform.
func (s *PixmapSink) PushLayer(blend BlendMode, alpha float32, transform kurbo.Affine, shape []kurbo.PathEl) {
	mask, clip := s.clipOf(transform, shape)
	var pixmap *okage.Pixmap
	if !clip.empty() && s.current() != nil {
		pixmap = s.acquire()
	}
	s.stack = append(s.stack, &sinkLayer{
		pixmap: pixmap,
		blend:  blend,
		alpha:  alpha,
		mask:   mask,
		clip:   clip,
	})
}

// PushClipLayer starts a layer clipped to shape under transform.
func (s *PixmapSink) PushClipLayer(transform kurbo.Affine, shape []kurbo.PathEl) {
	s.PushLayer(blendMix(MixNormal), 1.0, transform, shape)
}

// PopLayer ends the last layer, and composites it onto the layer below.
func (s *PixmapSink) PopLayer() {
	if len(s.stack) == 0 {
		return
	}
	layer := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	if layer.pixmap == nil {
		return
	}
	if s.current() == nil {
		release(layer.pixmap, layer.dirty)
		return
	}
	defer release(layer.pixmap, layer.dirty)
	if layer.filter != nil {
		applyFilter(layer.pixmap, layer.filter, layer.dirty)
	}
	// A mode that keeps the target where the layer is transparent changes
	// only the dirty pixels. Any other mode changes each pixel of the clip.
	region := layer.dirty.intersect(layer.clip)
	if !keepsTarget(layer.blend) {
		region = layer.clip
	}
	if region.empty() {
		return
	}
	s.touch(region)
	if layer.blend.Mix != MixNormal {
		paint := okage.DefaultPixmapPaint()
		paint.BlendMode = okageBlendMode(layer.blend)
		paint.Opacity = clampF32(layer.alpha)
		mask := layer.mask
		if mask == nil {
			mask = s.rectMask(layer.clip)
		}
		s.current().DrawPixmap(0, 0, layer.pixmap.AsRef(), &paint, path.IdentityTransform(), mask)
		return
	}
	var maskData []byte
	if layer.mask != nil {
		maskData = layer.mask.Data()
	}
	dst := s.current().DataMut()
	width := int(s.width)
	inner := layer.dirty.intersect(layer.clip)
	if !keepsTarget(layer.blend) {
		// Where the layer is transparent, these modes give a transparent
		// pixel.
		clearOutside(dst, width, region, inner)
	}
	if !inner.empty() {
		composite(dst, layer.pixmap.Data(), maskData, width, inner, layer.blend.Compose, clampF32(layer.alpha))
	}
}

// clearOutside makes each pixel of region outside inner transparent. The
// modes that call it give a transparent pixel where the layer is
// transparent, whatever the coverage of the clip, because the coverage scales
// the layer, as tiny-skia does.
func clearOutside(dst []byte, width int, region, inner pixRect) {
	for y := region.y0; y < region.y1; y++ {
		row := dst[y*width*4 : (y+1)*width*4]
		if y < inner.y0 || y >= inner.y1 || inner.empty() {
			clear(row[region.x0*4 : region.x1*4])
			continue
		}
		if region.x0 < inner.x0 {
			clear(row[region.x0*4 : inner.x0*4])
		}
		if inner.x1 < region.x1 {
			clear(row[inner.x1*4 : region.x1*4])
		}
	}
}

// keepsTarget reports if the compose mode of b leaves the target as it is
// where the layer is transparent.
func keepsTarget(b BlendMode) bool {
	if b.Mix != MixNormal {
		return true
	}
	switch b.Compose {
	case ComposeSrcIn, ComposeSrcOut, ComposeDestIn, ComposeDestAtop, ComposeCopy, ComposeClear:
		return false
	}
	return true
}

// div255 divides x, at most 255*255, by 255, rounded.
func div255(x int) int {
	x += 128
	return (x + x>>8) >> 8
}

// composite composites the premultiplied pixels src onto dst in region, with
// the Porter-Duff mode compose. alpha scales src, and so does the coverage
// in mask, when mask is not nil, as the stage mask_u8 of tiny-skia does.
func composite(dst, src, mask []byte, width int, region pixRect, compose Compose, alpha float32) {
	a8 := int(alpha*255 + 0.5)
	keep := keepsTarget(BlendMode{Compose: compose})
	for y := region.y0; y < region.y1; y++ {
		base := y * width
		for x := region.x0; x < region.x1; x++ {
			f := a8
			if mask != nil {
				f = div255(a8 * int(mask[base+x]))
			}
			i := (base + x) * 4
			sa := int(src[i+3])
			if keep && (sa == 0 || f == 0) {
				continue
			}
			s0, s1, s2 := int(src[i]), int(src[i+1]), int(src[i+2])
			if f < 255 {
				s0, s1, s2, sa = div255(s0*f), div255(s1*f), div255(s2*f), div255(sa*f)
			}
			d0, d1, d2, da := int(dst[i]), int(dst[i+1]), int(dst[i+2]), int(dst[i+3])
			switch compose {
			case ComposeSrcOver:
				if sa == 255 {
					dst[i], dst[i+1], dst[i+2], dst[i+3] = uint8(s0), uint8(s1), uint8(s2), 255
					continue
				}
				k := 255 - sa
				dst[i], dst[i+1], dst[i+2], dst[i+3] = uint8(s0+div255(d0*k)), uint8(s1+div255(d1*k)), uint8(s2+div255(d2*k)), uint8(sa+div255(da*k))
				continue
			case ComposeSrcIn, ComposeSrcOut:
				k := da
				if compose == ComposeSrcOut {
					k = 255 - da
				}
				dst[i], dst[i+1], dst[i+2], dst[i+3] = uint8(div255(s0*k)), uint8(div255(s1*k)), uint8(div255(s2*k)), uint8(div255(sa*k))
				continue
			case ComposeDestIn, ComposeDestOut:
				k := sa
				if compose == ComposeDestOut {
					k = 255 - sa
				}
				dst[i], dst[i+1], dst[i+2], dst[i+3] = uint8(div255(d0*k)), uint8(div255(d1*k)), uint8(div255(d2*k)), uint8(div255(da*k))
				continue
			}
			sp := [4]int{s0, s1, s2, sa}
			dp := [4]int{d0, d1, d2, da}
			for c := range 4 {
				sv, dv := sp[c], dp[c]
				var r int
				switch compose {
				case ComposeXor:
					r = div255(sv*(255-da)) + div255(dv*(255-sa))
				case ComposeDestOver:
					r = dv + div255(sv*(255-da))
				case ComposeSrcAtop:
					r = div255(sv*da) + div255(dv*(255-sa))
				case ComposeDestAtop:
					r = div255(dv*sa) + div255(sv*(255-da))
				case ComposePlus, ComposePlusLighter:
					r = min(255, sv+dv)
				case ComposeCopy:
					r = sv
				case ComposeClear:
					r = 0
				case ComposeDest:
					r = dv
				}
				dst[i+c] = uint8(min(r, 255))
			}
		}
	}
}

// applyFilter applies the effect of a Fill or Tint filter to the pixels of
// pixmap in region, as FilterEffect describes them.
func applyFilter(pixmap *okage.Pixmap, f *FilterEffect, region pixRect) {
	data := pixmap.DataMut()
	width := int(pixmap.Width())
	for y := region.y0; y < region.y1; y++ {
		for x := region.x0; x < region.x1; x++ {
			i := (y*width + x) * 4
			a := float32(data[i+3]) / 255
			if a == 0 {
				continue
			}
			var r, g, b, na float32
			switch f.Kind {
			case EffectFill:
				// Replaces source RGB; output alpha is source alpha times
				// effect opacity.
				r, g, b, na = f.Color[0], f.Color[1], f.Color[2], a*f.Color[3]
			case EffectTint:
				// Changes source RGB while keeping source alpha: maps the
				// luminance from black to white, mixed by the amount.
				sr, sg, sb := float32(data[i])/255/a, float32(data[i+1])/255/a, float32(data[i+2])/255/a
				l := 0.2126*sr + 0.7152*sg + 0.0722*sb
				am := float32(f.Amount)
				tr := f.Black[0] + (f.White[0]-f.Black[0])*l
				tg := f.Black[1] + (f.White[1]-f.Black[1])*l
				tb := f.Black[2] + (f.White[2]-f.Black[2])*l
				r, g, b, na = sr+(tr-sr)*am, sg+(tg-sg)*am, sb+(tb-sb)*am, a
			default:
				return
			}
			na = clampF32(na)
			data[i] = uint8(clampF32(r)*na*255 + 0.5)
			data[i+1] = uint8(clampF32(g)*na*255 + 0.5)
			data[i+2] = uint8(clampF32(b)*na*255 + 0.5)
			data[i+3] = uint8(na*255 + 0.5)
		}
	}
}

// clipOf returns the clip of shape under transform: a mask, or nil when the
// shape is a rectangle on the pixel grid, and the rectangle of the pixels
// that the clip can keep.
func (s *PixmapSink) clipOf(transform kurbo.Affine, shape []kurbo.PathEl) (*okage.Mask, pixRect) {
	if r, ok := gridRect(transform, shape); ok {
		return nil, r.intersect(s.canvas())
	}
	bounds := s.bounds(transform, shape, 0).intersect(s.canvas())
	if bounds.empty() {
		return nil, bounds
	}
	p := buildPath(shape)
	if p == nil {
		return nil, pixRect{}
	}
	mask, ok := okage.NewMask(s.width, s.height)
	if !ok {
		return nil, pixRect{}
	}
	mask.FillPath(p, okage.FillRuleWinding, true, okageTransform(transform))
	return mask, bounds
}

// gridRect reports if shape under transform is a rectangle whose sides lie
// on the pixel grid, and returns its pixels.
func gridRect(transform kurbo.Affine, shape []kurbo.PathEl) (pixRect, bool) {
	if transform[1] != 0 || transform[2] != 0 {
		return pixRect{}, false
	}
	n := len(shape)
	if n == 5 && shape[4].Kind == kurbo.PathElClosePath {
		n = 4
	}
	if n != 4 || shape[0].Kind != kurbo.PathElMoveTo {
		return pixRect{}, false
	}
	var pts [4]kurbo.Point
	for i := range 4 {
		if i > 0 && shape[i].Kind != kurbo.PathElLineTo {
			return pixRect{}, false
		}
		pts[i] = transform.MulPoint(shape[i].P[0])
	}
	// The sides alternate between horizontal and vertical.
	horizontal := pts[0].Y == pts[1].Y
	for i := range 4 {
		a, b := pts[i], pts[(i+1)%4]
		if (i%2 == 0) == horizontal {
			if a.Y != b.Y {
				return pixRect{}, false
			}
		} else if a.X != b.X {
			return pixRect{}, false
		}
	}
	x0, x1 := math.Min(pts[0].X, pts[2].X), math.Max(pts[0].X, pts[2].X)
	y0, y1 := math.Min(pts[0].Y, pts[2].Y), math.Max(pts[0].Y, pts[2].Y)
	for _, v := range []float64{x0, x1, y0, y1} {
		if v != math.Round(v) {
			return pixRect{}, false
		}
	}
	const limit = 1 << 30
	return pixRect{int(math.Max(x0, -limit)), int(math.Max(y0, -limit)), int(math.Min(x1, limit)), int(math.Min(y1, limit))}, true
}

// bounds returns the pixels that shape under transform can touch, with the
// shape grown by pad in its own coordinates.
func (s *PixmapSink) bounds(transform kurbo.Affine, shape []kurbo.PathEl, pad float64) pixRect {
	first := true
	var x0, y0, x1, y1 float64
	for _, el := range shape {
		n := map[kurbo.PathElKind]int{kurbo.PathElMoveTo: 1, kurbo.PathElLineTo: 1, kurbo.PathElQuadTo: 2, kurbo.PathElCurveTo: 3}[el.Kind]
		for _, p := range el.P[:n] {
			q := transform.MulPoint(p)
			if first {
				x0, y0, x1, y1, first = q.X, q.Y, q.X, q.Y, false
				continue
			}
			x0, y0 = math.Min(x0, q.X), math.Min(y0, q.Y)
			x1, y1 = math.Max(x1, q.X), math.Max(y1, q.Y)
		}
	}
	if first {
		return pixRect{}
	}
	// The pad grows by the largest scale of the transform, and 2 pixels of
	// antialiasing.
	scale := math.Sqrt(math.Max(transform[0]*transform[0]+transform[1]*transform[1], transform[2]*transform[2]+transform[3]*transform[3]))
	g := pad*scale + 2
	const limit = 1 << 30
	if !(x0 > -limit) || !(y0 > -limit) || !(x1 < limit) || !(y1 < limit) {
		return s.canvas()
	}
	return pixRect{int(math.Floor(x0 - g)), int(math.Floor(y0 - g)), int(math.Ceil(x1 + g)), int(math.Ceil(y1 + g))}
}

// rectMask returns a mask of the pixels of r.
func (s *PixmapSink) rectMask(r pixRect) *okage.Mask {
	if r == s.canvas() {
		return nil
	}
	mask, ok := okage.NewMask(s.width, s.height)
	if !ok {
		return nil
	}
	data := mask.DataMut()
	for y := r.y0; y < r.y1; y++ {
		for x := r.x0; x < r.x1; x++ {
			data[y*int(s.width)+x] = 255
		}
	}
	return mask
}

// PushFilter starts a layer for a Fill or a Tint effect. It returns
// FilterUnsupported for the Gaussian blur and the drop shadow, as the sinks
// of velato for Vello do for every effect (D41, D43).
func (s *PixmapSink) PushFilter(_ kurbo.Affine, effect *FilterEffect) FilterLayerResult {
	if effect.Kind != EffectFill && effect.Kind != EffectTint {
		return FilterUnsupported
	}
	if s.current() == nil {
		return FilterUnsupported
	}
	pixmap := s.acquire()
	if pixmap == nil {
		return FilterUnsupported
	}
	f := *effect
	s.stack = append(s.stack, &sinkLayer{pixmap: pixmap, blend: blendMix(MixNormal), alpha: 1, clip: s.canvas(), filter: &f})
	return FilterPushed
}

// Draw fills shape, or strokes it when stroke is not nil, with brush under
// transform. A fill uses the nonzero rule, as the sinks of velato do.
func (s *PixmapSink) Draw(stroke *Stroke, transform kurbo.Affine, brush *Brush, shape []kurbo.PathEl) {
	p := buildPath(shape)
	if p == nil {
		return
	}
	ts := okageTransform(transform)
	shader, ok := okageShader(brush)
	if !ok {
		return
	}
	target := s.current()
	if target == nil {
		return
	}
	pad := 0.0
	if stroke != nil {
		pad = stroke.Width / 2 * math.Max(stroke.MiterLimit, 1)
	}
	s.touch(s.bounds(transform, shape, pad))
	paint := okage.DefaultPaint()
	paint.Shader = shader
	paint.AntiAlias = true
	if stroke == nil {
		target.FillPath(p, &paint, okage.FillRuleWinding, ts, nil)
		return
	}
	st := path.DefaultStroke()
	st.Width = float32(stroke.Width)
	st.MiterLimit = float32(stroke.MiterLimit)
	switch stroke.StartCap {
	case CapRound:
		st.LineCap = path.LineCapRound
	case CapSquare:
		st.LineCap = path.LineCapSquare
	default:
		st.LineCap = path.LineCapButt
	}
	switch stroke.Join {
	case JoinRound:
		st.LineJoin = path.LineJoinRound
	case JoinMiter:
		st.LineJoin = path.LineJoinMiter
	default:
		st.LineJoin = path.LineJoinBevel
	}
	if len(stroke.Dash) > 0 {
		dash := make([]float32, 0, len(stroke.Dash)*2)
		for _, d := range stroke.Dash {
			dash = append(dash, float32(d))
		}
		if len(dash)%2 != 0 {
			dash = append(dash, dash...)
		}
		if sd, ok := path.NewStrokeDash(dash, float32(stroke.DashOffset)); ok {
			st.Dash = sd
		}
	}
	target.StrokePath(p, &paint, &st, ts, nil)
}

// DrawImage does nothing: the sink does not load image assets.
func (s *PixmapSink) DrawImage(*ImageAsset, kurbo.Affine, float64) {}

// BeginLayerGroup does nothing.
func (s *PixmapSink) BeginLayerGroup(string, int) {}

// EndLayerGroup does nothing.
func (s *PixmapSink) EndLayerGroup() {}

// buildPath returns the okage path of elements, or nil for an empty path.
func buildPath(elements []kurbo.PathEl) *okage.Path {
	pb := path.NewPathBuilder()
	for _, el := range elements {
		switch el.Kind {
		case kurbo.PathElMoveTo:
			pb.MoveTo(float32(el.P[0].X), float32(el.P[0].Y))
		case kurbo.PathElLineTo:
			pb.LineTo(float32(el.P[0].X), float32(el.P[0].Y))
		case kurbo.PathElQuadTo:
			pb.QuadTo(float32(el.P[0].X), float32(el.P[0].Y), float32(el.P[1].X), float32(el.P[1].Y))
		case kurbo.PathElCurveTo:
			pb.CubicTo(float32(el.P[0].X), float32(el.P[0].Y), float32(el.P[1].X), float32(el.P[1].Y), float32(el.P[2].X), float32(el.P[2].Y))
		case kurbo.PathElClosePath:
			pb.Close()
		}
	}
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// okageTransform converts an affine transform.
func okageTransform(a kurbo.Affine) okage.Transform {
	return path.TransformFromRow(float32(a[0]), float32(a[1]), float32(a[2]), float32(a[3]), float32(a[4]), float32(a[5]))
}

// okageColor converts a color, with each component clamped to [0, 1].
func okageColor(c Color) okage.Color {
	return okage.ColorFromRGBAUnchecked(clampF32(c[0]), clampF32(c[1]), clampF32(c[2]), clampF32(c[3]))
}

// clampF32 clamps x to [0, 1], with 0 for NaN.
func clampF32(x float32) float32 {
	if !(x > 0) {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// okageShader converts a brush to a shader in the coordinates of the path.
// FillPath and StrokePath apply the transform of the path to the shader.
func okageShader(brush *Brush) (okage.Shader, bool) {
	ts := path.IdentityTransform()
	if brush.Kind == BrushSolid {
		return okage.SolidColor(okageColor(brush.Color)), true
	}
	g := &brush.Gradient
	if len(g.Stops) == 0 {
		return nil, false
	}
	stops := make([]okage.GradientStop, len(g.Stops))
	for i, st := range g.Stops {
		stops[i] = okage.NewGradientStop(st.Offset, okageColor(st.Color))
	}
	start := path.PointFromXY(float32(g.Start.X), float32(g.Start.Y))
	if g.Kind == GradientRadial {
		focal := path.PointFromXY(float32(g.Focal.X), float32(g.Focal.Y))
		shader, ok := okage.NewRadialGradient(focal, 0, start, g.Radius, stops, okage.SpreadModePad, ts)
		if !ok {
			return okage.SolidColor(okageColor(g.Stops[len(g.Stops)-1].Color)), true
		}
		return shader, true
	}
	end := path.PointFromXY(float32(g.End.X), float32(g.End.Y))
	shader, ok := okage.NewLinearGradient(start, end, stops, okage.SpreadModePad, ts)
	if !ok {
		return okage.SolidColor(okageColor(g.Stops[len(g.Stops)-1].Color)), true
	}
	return shader, true
}

// okageBlendMode converts a blend mode. A mode other than Normal wins over
// the compose mode.
func okageBlendMode(b BlendMode) okage.BlendMode {
	switch b.Mix {
	case MixMultiply:
		return okage.BlendModeMultiply
	case MixScreen:
		return okage.BlendModeScreen
	case MixOverlay:
		return okage.BlendModeOverlay
	case MixDarken:
		return okage.BlendModeDarken
	case MixLighten:
		return okage.BlendModeLighten
	case MixColorDodge:
		return okage.BlendModeColorDodge
	case MixColorBurn:
		return okage.BlendModeColorBurn
	case MixHardLight:
		return okage.BlendModeHardLight
	case MixSoftLight:
		return okage.BlendModeSoftLight
	case MixDifference:
		return okage.BlendModeDifference
	case MixExclusion:
		return okage.BlendModeExclusion
	case MixHue:
		return okage.BlendModeHue
	case MixSaturation:
		return okage.BlendModeSaturation
	case MixColor:
		return okage.BlendModeColor
	case MixLuminosity:
		return okage.BlendModeLuminosity
	}
	switch b.Compose {
	case ComposeClear:
		return okage.BlendModeClear
	case ComposeCopy:
		return okage.BlendModeSource
	case ComposeDest:
		return okage.BlendModeDestination
	case ComposeDestOver:
		return okage.BlendModeDestinationOver
	case ComposeSrcIn:
		return okage.BlendModeSourceIn
	case ComposeDestIn:
		return okage.BlendModeDestinationIn
	case ComposeSrcOut:
		return okage.BlendModeSourceOut
	case ComposeDestOut:
		return okage.BlendModeDestinationOut
	case ComposeSrcAtop:
		return okage.BlendModeSourceAtop
	case ComposeDestAtop:
		return okage.BlendModeDestinationAtop
	case ComposeXor:
		return okage.BlendModeXor
	case ComposePlus, ComposePlusLighter:
		return okage.BlendModePlus
	}
	return okage.BlendModeSourceOver
}

// RenderFrame renders the frame of c onto pixmap under transform, with a
// new Renderer and a PixmapSink.
func RenderFrame(c *Composition, frame float64, transform kurbo.Affine, pixmap *okage.PixmapMut) error {
	return NewRenderer().Append(c, frame, transform, 1.0, NewPixmapSink(pixmap))
}
