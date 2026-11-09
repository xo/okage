package oracle

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"strconv"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// num is a float32 in a scene. It is written to JSON as the shortest decimal
// of a float64 that gives back the float32. serde_json can read such a
// decimal one unit in the last place of a float64 off, and the conversion to
// f32 in the oracle still gives the same float32.
type num float32

// MarshalJSON satisfies the json.Marshaler interface.
func (n num) MarshalJSON() ([]byte, error) {
	return strconv.AppendFloat(nil, float64(n), 'g', -1, 64), nil
}

// UnmarshalJSON satisfies the json.Unmarshaler interface. It reads a float64
// and converts it to a float32, as the oracle does.
func (n *num) UnmarshalJSON(b []byte) error {
	v, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return err
	}
	*n = num(float32(v))
	return nil
}

// The verbs of a segment, the first value of each segment of a path.
const (
	verbMove num = iota
	verbLine
	verbQuad
	verbCubic
	verbClose
)

// segment is a segment of a path: its verb, then its points.
type segment []num

// scene is a scene of the renderer. The oracle reads the same JSON.
type scene struct {
	Width   uint32     `json:"width"`
	Height  uint32     `json:"height"`
	Sources []source   `json:"sources,omitempty"`
	Masks   []maskSpec `json:"masks,omitempty"`
	Ops     []op       `json:"ops"`
	Name    string     `json:"name"`
}

// source is a pixmap of random pixels.
type source struct {
	Width  uint32 `json:"width"`
	Height uint32 `json:"height"`
	Seed   uint32 `json:"seed"`
}

// maskSpec is a mask: from a source, or from a path. Intersect, if set,
// intersects the mask with a second path.
type maskSpec struct {
	FromSource *int      `json:"from_source,omitempty"`
	Luminance  bool      `json:"luminance"`
	Path       []segment `json:"path,omitempty"`
	EvenOdd    bool      `json:"even_odd"`
	AntiAlias  bool      `json:"anti_alias"`
	Transform  [6]num    `json:"transform"`
	Intersect  []segment `json:"intersect,omitempty"`
	Invert     bool      `json:"invert"`
}

// op is one operation on the pixmap. Kind selects the fields that count:
// fill_path, stroke_path, fill_rect, draw_pixmap, apply_mask or fill.
type op struct {
	Kind      string      `json:"kind"`
	Path      []segment   `json:"path,omitempty"`
	Paint     *paintSpec  `json:"paint,omitempty"`
	EvenOdd   bool        `json:"even_odd"`
	Transform [6]num      `json:"transform"`
	Mask      *int        `json:"mask,omitempty"`
	Stroke    *strokeSpec `json:"stroke,omitempty"`
	Rect      *[4]num     `json:"rect,omitempty"`
	X         int32       `json:"x"`
	Y         int32       `json:"y"`
	Source    int         `json:"source"`
	Opacity   num         `json:"opacity"`
	BlendMode uint8       `json:"blend_mode"`
	Quality   uint8       `json:"quality"`
	Color     *[4]num     `json:"color,omitempty"`
}

// paintSpec is a paint. Opacity, if set, is applied to the shader.
type paintSpec struct {
	Shader     shaderSpec `json:"shader"`
	BlendMode  uint8      `json:"blend_mode"`
	AntiAlias  bool       `json:"anti_alias"`
	ForceHQ    bool       `json:"force_hq"`
	Colorspace uint8      `json:"colorspace"`
	Opacity    *num       `json:"opacity,omitempty"`
}

// shaderSpec is a shader. Kind selects the fields that count: solid, linear,
// radial, sweep or pattern.
type shaderSpec struct {
	Kind        string   `json:"kind"`
	Color       *[4]num  `json:"color,omitempty"`
	Start       *[2]num  `json:"start,omitempty"`
	End         *[2]num  `json:"end,omitempty"`
	Center      *[2]num  `json:"center,omitempty"`
	StartRadius num      `json:"start_radius"`
	EndRadius   num      `json:"end_radius"`
	StartAngle  num      `json:"start_angle"`
	EndAngle    num      `json:"end_angle"`
	Stops       [][5]num `json:"stops,omitempty"`
	Spread      uint8    `json:"spread"`
	Transform   [6]num   `json:"transform"`
	Source      int      `json:"source"`
	Quality     uint8    `json:"quality"`
	Opacity     num      `json:"opacity"`
}

// dashSpec is a dash: the intervals and the offset. JSON writes it as a
// list of two values, as the tuple of the oracle reads it.
type dashSpec struct {
	Array  []num
	Offset num
}

// MarshalJSON satisfies the json.Marshaler interface.
func (d dashSpec) MarshalJSON() ([]byte, error) {
	b := []byte("[[")
	for i, v := range d.Array {
		if i > 0 {
			b = append(b, ',')
		}
		b = strconv.AppendFloat(b, float64(v), 'g', -1, 64)
	}
	b = append(b, "],"...)
	b = strconv.AppendFloat(b, float64(d.Offset), 'g', -1, 64)
	return append(b, ']'), nil
}

// UnmarshalJSON satisfies the json.Unmarshaler interface.
func (d *dashSpec) UnmarshalJSON(b []byte) error {
	var raw [2]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if err := json.Unmarshal(raw[0], &d.Array); err != nil {
		return err
	}
	return json.Unmarshal(raw[1], &d.Offset)
}

// strokeSpec is a stroke.
type strokeSpec struct {
	Width num       `json:"width"`
	Miter num       `json:"miter"`
	Cap   uint8     `json:"cap"`
	Join  uint8     `json:"join"`
	Dash  *dashSpec `json:"dash,omitempty"`
}

// hasCubic reports whether a path of the scene has a cubic curve. D26 allows
// such a scene to differ from the oracle.
func (s *scene) hasCubic() bool {
	cubic := func(segs []segment) bool {
		for _, seg := range segs {
			if seg[0] == verbCubic {
				return true
			}
		}
		return false
	}
	for _, m := range s.Masks {
		if cubic(m.Path) || cubic(m.Intersect) {
			return true
		}
	}
	for _, o := range s.Ops {
		if cubic(o.Path) {
			return true
		}
	}
	return false
}

func transform(t [6]num) okage.Transform {
	return okage.TransformFromRow(float32(t[0]), float32(t[1]), float32(t[2]), float32(t[3]), float32(t[4]), float32(t[5]))
}

// buildPath builds a path from segments, as path in the oracle does.
func buildPath(segs []segment) (*okage.Path, bool) {
	pb := okage.NewPathBuilder()
	for _, s := range segs {
		switch s[0] {
		case verbMove:
			pb.MoveTo(float32(s[1]), float32(s[2]))
		case verbLine:
			pb.LineTo(float32(s[1]), float32(s[2]))
		case verbQuad:
			pb.QuadTo(float32(s[1]), float32(s[2]), float32(s[3]), float32(s[4]))
		case verbCubic:
			pb.CubicTo(float32(s[1]), float32(s[2]), float32(s[3]), float32(s[4]), float32(s[5]), float32(s[6]))
		default:
			pb.Close()
		}
	}
	return pb.Finish()
}

// blends holds the blend modes in the order of the Rust enum.
var blends = []okage.BlendMode{
	okage.BlendModeClear, okage.BlendModeSource, okage.BlendModeDestination,
	okage.BlendModeSourceOver, okage.BlendModeDestinationOver, okage.BlendModeSourceIn,
	okage.BlendModeDestinationIn, okage.BlendModeSourceOut, okage.BlendModeDestinationOut,
	okage.BlendModeSourceAtop, okage.BlendModeDestinationAtop, okage.BlendModeXor,
	okage.BlendModePlus, okage.BlendModeModulate, okage.BlendModeScreen,
	okage.BlendModeOverlay, okage.BlendModeDarken, okage.BlendModeLighten,
	okage.BlendModeColorDodge, okage.BlendModeColorBurn, okage.BlendModeHardLight,
	okage.BlendModeSoftLight, okage.BlendModeDifference, okage.BlendModeExclusion,
	okage.BlendModeMultiply, okage.BlendModeHue, okage.BlendModeSaturation,
	okage.BlendModeColor, okage.BlendModeLuminosity,
}

func stops(s [][5]num) []okage.GradientStop {
	out := make([]okage.GradientStop, 0, len(s))
	for _, v := range s {
		c, ok := okage.ColorFromRGBA(float32(v[1]), float32(v[2]), float32(v[3]), float32(v[4]))
		if !ok {
			panic("a color of a gradient stop is out of range")
		}
		out = append(out, okage.NewGradientStop(float32(v[0]), c))
	}
	return out
}

// makeSource makes the pixmap of random pixels of s, with the generator of
// the oracle.
func makeSource(s source) *okage.Pixmap {
	n := int(s.Width * s.Height)
	data := make([]byte, n*4)
	h := s.Seed
	for i := range n {
		h = h*1664525 + 1013904223
		a := h >> 24
		if (h>>3)&3 == 0 {
			a = 255
		}
		data[i*4] = uint8(((h >> 16) & 255) * a / 255)
		data[i*4+1] = uint8(((h >> 8) & 255) * a / 255)
		data[i*4+2] = uint8((h & 255) * a / 255)
		data[i*4+3] = uint8(a)
	}
	size, _ := okage.IntSizeFromWH(s.Width, s.Height)
	p, ok := okage.PixmapFromVec(data, size)
	if !ok {
		panic("a source has a bad size")
	}
	return p
}

func point(p *[2]num) okage.Point {
	return okage.PointFromXY(float32(p[0]), float32(p[1]))
}

func makeShader(s *shaderSpec, sources []*okage.Pixmap) (okage.Shader, bool) {
	switch s.Kind {
	case "solid":
		c, ok := okage.ColorFromRGBA(float32(s.Color[0]), float32(s.Color[1]), float32(s.Color[2]), float32(s.Color[3]))
		return okage.SolidColor(c), ok
	case "linear":
		return okage.NewLinearGradient(point(s.Start), point(s.End), stops(s.Stops), okage.SpreadMode(s.Spread), transform(s.Transform))
	case "radial":
		return okage.NewRadialGradient(point(s.Start), float32(s.StartRadius), point(s.End), float32(s.EndRadius),
			stops(s.Stops), okage.SpreadMode(s.Spread), transform(s.Transform))
	case "sweep":
		return okage.NewSweepGradient(point(s.Center), float32(s.StartAngle), float32(s.EndAngle),
			stops(s.Stops), okage.SpreadMode(s.Spread), transform(s.Transform))
	case "pattern":
		return okage.NewPattern(sources[s.Source].AsRef(), okage.SpreadMode(s.Spread), okage.FilterQuality(s.Quality),
			float32(s.Opacity), transform(s.Transform)), true
	}
	panic("unknown shader " + s.Kind)
}

func makePaint(p *paintSpec, sources []*okage.Pixmap) (okage.Paint, bool) {
	paint := okage.DefaultPaint()
	sh, ok := makeShader(&p.Shader, sources)
	if !ok {
		return paint, false
	}
	paint.Shader = sh
	if p.Opacity != nil {
		paint.Shader = paint.Shader.ApplyOpacity(float32(*p.Opacity))
	}
	paint.BlendMode = blends[p.BlendMode]
	paint.AntiAlias = p.AntiAlias
	paint.ForceHQPipeline = p.ForceHQ
	paint.Colorspace = okage.ColorSpace(p.Colorspace)
	return paint, true
}

func makeStroke(s *strokeSpec) okage.Stroke {
	st := okage.DefaultStroke()
	st.Width = float32(s.Width)
	st.MiterLimit = float32(s.Miter)
	st.LineCap = okage.LineCap(s.Cap)
	st.LineJoin = okage.LineJoin(s.Join)
	if s.Dash != nil {
		a := make([]float32, len(s.Dash.Array))
		for i, v := range s.Dash.Array {
			a[i] = float32(v)
		}
		st.Dash, _ = okage.NewStrokeDash(a, float32(s.Dash.Offset))
	}
	return st
}

func fillRule(evenOdd bool) okage.FillRule {
	if evenOdd {
		return okage.FillRuleEvenOdd
	}
	return okage.FillRuleWinding
}

func maskAt(i *int, masks []*okage.Mask) *okage.Mask {
	if i == nil {
		return nil
	}
	return masks[*i]
}

// render renders a scene with okage. It gives the same bytes as render in
// the oracle: the pixmap, then each mask.
func render(sc *scene) []byte {
	pixmap, ok := okage.NewPixmap(sc.Width, sc.Height)
	if !ok {
		return nil
	}
	var sources []*okage.Pixmap
	for _, s := range sc.Sources {
		sources = append(sources, makeSource(s))
	}
	var masks []*okage.Mask
	for _, m := range sc.Masks {
		var mask *okage.Mask
		if m.FromSource != nil {
			mt := okage.MaskTypeAlpha
			if m.Luminance {
				mt = okage.MaskTypeLuminance
			}
			mask = okage.MaskFromPixmap(sources[*m.FromSource].AsRef(), mt)
		} else {
			mask, ok = okage.NewMask(sc.Width, sc.Height)
			if !ok {
				return nil
			}
			if p, ok := buildPath(m.Path); ok {
				mask.FillPath(p, fillRule(m.EvenOdd), m.AntiAlias, transform(m.Transform))
			}
		}
		if p, ok := buildPath(m.Intersect); ok {
			mask.IntersectPath(p, fillRule(m.EvenOdd), m.AntiAlias, transform(m.Transform))
		}
		if m.Invert {
			mask.Invert()
		}
		masks = append(masks, mask)
	}
	for i := range sc.Ops {
		o := &sc.Ops[i]
		switch o.Kind {
		case "fill_path":
			p, ok1 := buildPath(o.Path)
			paint, ok2 := makePaint(o.Paint, sources)
			if !ok1 || !ok2 {
				continue
			}
			pixmap.FillPath(p, &paint, fillRule(o.EvenOdd), transform(o.Transform), maskAt(o.Mask, masks))
		case "stroke_path":
			p, ok1 := buildPath(o.Path)
			paint, ok2 := makePaint(o.Paint, sources)
			if !ok1 || !ok2 {
				continue
			}
			st := makeStroke(o.Stroke)
			pixmap.StrokePath(p, &paint, &st, transform(o.Transform), maskAt(o.Mask, masks))
		case "fill_rect":
			r, ok1 := okage.RectFromXYWH(float32(o.Rect[0]), float32(o.Rect[1]), float32(o.Rect[2]), float32(o.Rect[3]))
			paint, ok2 := makePaint(o.Paint, sources)
			if !ok1 || !ok2 {
				continue
			}
			pixmap.FillRect(r, &paint, transform(o.Transform), maskAt(o.Mask, masks))
		case "draw_pixmap":
			pp := okage.PixmapPaint{Opacity: float32(o.Opacity), BlendMode: blends[o.BlendMode], Quality: okage.FilterQuality(o.Quality)}
			pixmap.DrawPixmap(o.X, o.Y, sources[o.Source].AsRef(), &pp, transform(o.Transform), maskAt(o.Mask, masks))
		case "apply_mask":
			pixmap.ApplyMask(masks[*o.Mask])
		case "fill":
			c, _ := okage.ColorFromRGBA(float32(o.Color[0]), float32(o.Color[1]), float32(o.Color[2]), float32(o.Color[3]))
			pixmap.Fill(c)
		default:
			panic("unknown op " + o.Kind)
		}
	}
	out := bytes.Clone(pixmap.Data())
	for _, m := range masks {
		out = append(out, m.Data()...)
	}
	return out
}

// pathQuestion is a question of the package path: stroke, dash or transform
// a path.
type pathQuestion struct {
	PathOp    string      `json:"path_op"`
	Path      []segment   `json:"path"`
	Stroke    *strokeSpec `json:"stroke,omitempty"`
	ResScale  num         `json:"res_scale"`
	Transform [6]num      `json:"transform"`
	Name      string      `json:"name"`
}

// hasCubic reports whether the path of the question has a cubic curve.
func (q *pathQuestion) hasCubic() bool {
	for _, seg := range q.Path {
		if seg[0] == verbCubic {
			return true
		}
	}
	return false
}

// answer answers a question of the package path with okage. It gives the
// same bytes as answer_path in the oracle.
func (q *pathQuestion) answer() []byte {
	p, ok := buildPath(q.Path)
	if !ok {
		return nil
	}
	var result *path.Path
	switch q.PathOp {
	case "stroke":
		st := makeStroke(q.Stroke)
		result, ok = p.Stroke(&st, float32(q.ResScale))
	case "dash":
		st := makeStroke(q.Stroke)
		if st.Dash == nil {
			return nil
		}
		result, ok = p.Dash(st.Dash, float32(q.ResScale))
	case "transform":
		result, ok = p.Transform(transform(q.Transform))
	default:
		panic("unknown path_op " + q.PathOp)
	}
	if !ok {
		return nil
	}
	return writePath(result)
}

// writePath writes a path as write_path in the oracle does.
func writePath(p *path.Path) []byte {
	le := binary.LittleEndian
	var out []byte
	f32 := func(v float32) {
		out = le.AppendUint32(out, math.Float32bits(v))
	}
	rect := func(r path.Rect) {
		f32(r.Left())
		f32(r.Top())
		f32(r.Right())
		f32(r.Bottom())
	}
	out = le.AppendUint32(out, uint32(len(p.Verbs())))
	for _, v := range p.Verbs() {
		out = append(out, byte(v))
	}
	out = le.AppendUint32(out, uint32(len(p.Points())))
	for _, pt := range p.Points() {
		f32(pt.X)
		f32(pt.Y)
	}
	rect(p.Bounds())
	if r, ok := p.ComputeTightBounds(); ok {
		out = append(out, 1)
		rect(r)
	} else {
		out = append(out, 0)
	}
	return out
}
