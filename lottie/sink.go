// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file holds the Sink that draws with okage (D41). It ports no velato
// file. It does what the sinks of velato for Vello do, with the parts of
// okage that svg uses for a group.

import (
	"github.com/xo/okage"
	"github.com/xo/okage/internal/kurbo"
	"github.com/xo/okage/path"
)

// PixmapSink is a Sink that draws onto an okage pixmap. Each layer is a
// pixmap of the size of the target. PopLayer composites it onto the layer
// below with its blend mode, its opacity and its clip.
type PixmapSink struct {
	target *okage.PixmapMut
	width  uint32
	height uint32
	stack  []*sinkLayer
}

// sinkLayer is a layer of a PixmapSink.
type sinkLayer struct {
	pixmap *okage.Pixmap
	mode   okage.BlendMode
	alpha  float32
	mask   *okage.Mask
	// filter is the effect of a filter layer, which has no clip, or nil.
	filter *FilterEffect
}

// NewPixmapSink returns a sink that draws onto target.
func NewPixmapSink(target *okage.PixmapMut) *PixmapSink {
	return &PixmapSink{target: target, width: target.Width(), height: target.Height()}
}

// current returns the pixmap that draws go to.
func (s *PixmapSink) current() *okage.PixmapMut {
	if len(s.stack) == 0 {
		return s.target
	}
	m := s.stack[len(s.stack)-1].pixmap.AsMut()
	return &m
}

// PushLayer starts a layer that composites with blend and alpha, clipped to
// shape under transform.
func (s *PixmapSink) PushLayer(blend BlendMode, alpha float32, transform kurbo.Affine, shape []kurbo.PathEl) {
	pixmap, ok := okage.NewPixmap(s.width, s.height)
	if !ok {
		pixmap = nil
	}
	s.stack = append(s.stack, &sinkLayer{
		pixmap: pixmap,
		mode:   okageBlendMode(blend),
		alpha:  alpha,
		mask:   s.clipMask(transform, shape),
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
	if layer.pixmap == nil || (layer.mask == nil && layer.filter == nil) {
		return
	}
	if layer.filter != nil {
		applyFilter(layer.pixmap, layer.filter)
	}
	paint := okage.DefaultPixmapPaint()
	paint.BlendMode = layer.mode
	paint.Opacity = clampF32(layer.alpha)
	s.current().DrawPixmap(0, 0, layer.pixmap.AsRef(), &paint, path.IdentityTransform(), layer.mask)
}

// applyFilter applies the effect of a Fill or Tint filter to the pixels of
// pixmap, as FilterEffect describes them.
func applyFilter(pixmap *okage.Pixmap, f *FilterEffect) {
	data := pixmap.DataMut()
	for i := 0; i+3 < len(data); i += 4 {
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

// clipMask returns the mask of shape under transform, or nil when the shape
// is empty.
func (s *PixmapSink) clipMask(transform kurbo.Affine, shape []kurbo.PathEl) *okage.Mask {
	p := buildPath(shape)
	if p == nil {
		return nil
	}
	mask, ok := okage.NewMask(s.width, s.height)
	if !ok {
		return nil
	}
	mask.FillPath(p, okage.FillRuleWinding, true, okageTransform(transform))
	return mask
}

// PushFilter starts a layer for a Fill or a Tint effect. It returns
// FilterUnsupported for the Gaussian blur and the drop shadow, as the sinks
// of velato for Vello do for every effect (D41, D43).
func (s *PixmapSink) PushFilter(_ kurbo.Affine, effect *FilterEffect) FilterLayerResult {
	if effect.Kind != EffectFill && effect.Kind != EffectTint {
		return FilterUnsupported
	}
	pixmap, ok := okage.NewPixmap(s.width, s.height)
	if !ok {
		return FilterUnsupported
	}
	f := *effect
	s.stack = append(s.stack, &sinkLayer{pixmap: pixmap, mode: okage.BlendModeSourceOver, alpha: 1, filter: &f})
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
	paint := okage.DefaultPaint()
	paint.Shader = shader
	paint.AntiAlias = true
	if stroke == nil {
		s.current().FillPath(p, &paint, okage.FillRuleWinding, ts, nil)
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
	s.current().StrokePath(p, &paint, &st, ts, nil)
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
