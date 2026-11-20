package krilla

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/xo/okage/internal/crmath"
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
)

// The tests of this file compare the surface and the graphics of the package
// pdf with krilla, through the question "krilla" of the oracle (the gate of
// phase 14 of the PDF writer in docs/PLAN.md): the paths, the fills, the
// strokes, the transforms, the clips, the opacities, the blend modes, the
// isolated groups, the masks, the colors, the gradients, the patterns and the
// ICC color spaces. image_test.go compares the images, and text_test.go the
// text, with the same calls of the surface. The objects of the two files must be the same after the reader decodes the
// streams. When no stream is compressed, the bytes must be the same, with
// the ID of the document. qpdf must accept each file of pdf.

// opSpec is a call of the surface, as the oracle reads it in scene.rs. goRun
// decodes the calls of a scene from JSON, so that it reads each number as
// the oracle reads it.
type opSpec struct {
	textSpec

	Op        string      `json:"op"`
	Fill      *fillSpec   `json:"fill"`
	Stroke    *strokeSpec `json:"stroke"`
	Path      [][]float64 `json:"path"`
	Transform *[6]float32 `json:"transform"`
	Mode      string      `json:"mode"`
	Rule      *string     `json:"rule"`
	MaskType  string      `json:"mask_type"`
	Ops       []opSpec    `json:"ops"`
	BBox      *[4]float32 `json:"bbox"`
	Opacity   *float32    `json:"opacity"`
	Image     *int        `json:"image"`
	Width     float32     `json:"width"`
	Height    float32     `json:"height"`

	// The fields of the call draw_svg, with Width and Height.
	File        string     `json:"file"`
	EmbedText   *bool      `json:"embed_text"`
	FilterScale *float32   `json:"filter_scale"`
	Fonts       *fontsSpec `json:"fonts"`
}

type fillSpec struct {
	Paint   *paintSpec `json:"paint"`
	Opacity *float32   `json:"opacity"`
	Rule    *string    `json:"rule"`
}

type strokeSpec struct {
	Paint      *paintSpec `json:"paint"`
	Width      *float32   `json:"width"`
	MiterLimit *float32   `json:"miter_limit"`
	LineCap    *string    `json:"line_cap"`
	LineJoin   *string    `json:"line_join"`
	Opacity    *float32   `json:"opacity"`
	Dash       *struct {
		Array  []float32 `json:"array"`
		Offset float32   `json:"offset"`
	} `json:"dash"`
}

type paintSpec struct {
	RGB     *[3]uint8     `json:"rgb"`
	Luma    *uint8        `json:"luma"`
	CMYK    *[4]uint8     `json:"cmyk"`
	Linear  *gradientSpec `json:"linear"`
	Radial  *gradientSpec `json:"radial"`
	Sweep   *gradientSpec `json:"sweep"`
	Pattern *patternSpec  `json:"pattern"`
}

// gradientSpec holds the fields of the three gradients of the oracle.
type gradientSpec struct {
	X1         float32     `json:"x1"`
	Y1         float32     `json:"y1"`
	X2         float32     `json:"x2"`
	Y2         float32     `json:"y2"`
	Fx         float32     `json:"fx"`
	Fy         float32     `json:"fy"`
	Fr         float32     `json:"fr"`
	Cx         float32     `json:"cx"`
	Cy         float32     `json:"cy"`
	Cr         float32     `json:"cr"`
	StartAngle float32     `json:"start_angle"`
	EndAngle   float32     `json:"end_angle"`
	Transform  *[6]float32 `json:"transform"`
	Spread     *string     `json:"spread"`
	Stops      []stopSpec  `json:"stops"`
	AntiAlias  bool        `json:"anti_alias"`
}

type stopSpec struct {
	Offset  float32   `json:"offset"`
	Color   paintSpec `json:"color"`
	Opacity *float32  `json:"opacity"`
}

type patternSpec struct {
	Ops       []opSpec    `json:"ops"`
	Width     float32     `json:"width"`
	Height    float32     `json:"height"`
	Transform *[6]float32 `json:"transform"`
}

// goRun makes the calls of a list on a surface, as run of the oracle does.
// Each push of the list must have its pop in the same list. If a call fails,
// goRun pops what the list pushed, so that the surface can finish.
func goRun(s *pdf.Surface, ops []op, res *sceneResources) error {
	data, err := json.Marshal(ops)
	if err != nil {
		return err
	}
	var specs []opSpec
	if err := json.Unmarshal(data, &specs); err != nil {
		return err
	}
	return goRunSpecs(s, specs, res)
}

func goRunSpecs(s *pdf.Surface, ops []opSpec, res *sceneResources) error {
	depth := 0
	for _, o := range ops {
		if err := goStep(s, o, &depth, res); err != nil {
			for range depth {
				s.Pop()
			}
			return err
		}
	}
	if depth != 0 {
		for range depth {
			s.Pop()
		}
		return fmt.Errorf("%d pushes have no pop", depth)
	}
	return nil
}

func goStep(s *pdf.Surface, o opSpec, depth *int, res *sceneResources) error {
	switch o.Op {
	case "set_fill":
		if o.Fill == nil {
			s.SetFill(nil)
			return nil
		}
		paint, err := goPaint(s, o.Fill.Paint, res)
		if err != nil {
			return err
		}
		opacity, err := goNormalized(o.Fill.Opacity)
		if err != nil {
			return err
		}
		rule, err := goFillRule(o.Fill.Rule)
		if err != nil {
			return err
		}
		s.SetFill(&pdf.Fill{Paint: paint, Opacity: opacity, Rule: rule})
	case "set_stroke":
		if o.Stroke == nil {
			s.SetStroke(nil)
			return nil
		}
		k := pdf.DefaultStroke()
		paint, err := goPaint(s, o.Stroke.Paint, res)
		if err != nil {
			return err
		}
		k.Paint = paint
		if o.Stroke.Width != nil {
			k.Width = *o.Stroke.Width
		}
		if o.Stroke.MiterLimit != nil {
			k.MiterLimit = *o.Stroke.MiterLimit
		}
		if c := o.Stroke.LineCap; c != nil {
			caps := map[string]pdf.LineCap{"butt": pdf.LineCapButt, "round": pdf.LineCapRound, "square": pdf.LineCapSquare}
			lc, ok := caps[*c]
			if !ok {
				return fmt.Errorf("the line cap %q is not known", *c)
			}
			k.LineCap = lc
		}
		if j := o.Stroke.LineJoin; j != nil {
			joins := map[string]pdf.LineJoin{"miter": pdf.LineJoinMiter, "round": pdf.LineJoinRound, "bevel": pdf.LineJoinBevel}
			lj, ok := joins[*j]
			if !ok {
				return fmt.Errorf("the line join %q is not known", *j)
			}
			k.LineJoin = lj
		}
		if k.Opacity, err = goNormalized(o.Stroke.Opacity); err != nil {
			return err
		}
		if d := o.Stroke.Dash; d != nil {
			k.Dash = &pdf.StrokeDash{Array: d.Array, Offset: d.Offset}
		}
		s.SetStroke(&k)
	case "draw_path":
		p, err := goPath(o.Path)
		if err != nil {
			return err
		}
		s.DrawPath(p)
	case "push_transform":
		if o.Transform == nil {
			return errors.New("push_transform has no transform")
		}
		t := o.Transform
		s.PushTransform(path.TransformFromRow(t[0], t[1], t[2], t[3], t[4], t[5]))
		*depth++
	case "push_blend_mode":
		mode, ok := surfaceBlendModes[o.Mode]
		if !ok {
			return fmt.Errorf("the blend mode %q is not known", o.Mode)
		}
		s.PushBlendMode(mode)
		*depth++
	case "push_clip_path":
		p, err := goPath(o.Path)
		if err != nil {
			return err
		}
		rule, err := goFillRule(o.Rule)
		if err != nil {
			return err
		}
		s.PushClipPath(p, rule)
		*depth++
	case "push_mask":
		maskTypes := map[string]pdf.MaskType{"luminosity": pdf.MaskTypeLuminosity, "alpha": pdf.MaskTypeAlpha}
		maskType, ok := maskTypes[o.MaskType]
		if !ok {
			return fmt.Errorf("the mask type %q is not known", o.MaskType)
		}
		sb := s.StreamBuilder()
		var sub *pdf.Surface
		if b := o.BBox; b != nil {
			r, ok := path.RectFromXYWH(b[0], b[1], b[2], b[3])
			if !ok {
				return errors.New("the box of a mask is not valid")
			}
			sub = sb.SurfaceWithBBox(r)
		} else {
			sub = sb.Surface()
		}
		err := goRunSpecs(sub, o.Ops, res)
		sub.Finish()
		stream := sb.Finish()
		if err != nil {
			return err
		}
		s.PushMask(pdf.NewMask(stream, maskType))
		*depth++
	case "push_opacity":
		opacity, err := goNormalized(o.Opacity)
		if err != nil {
			return err
		}
		s.PushOpacity(opacity)
		*depth++
	case "push_isolated":
		s.PushIsolated()
		*depth++
	case "pop":
		if *depth == 0 {
			return errors.New("a pop has no push in its list")
		}
		s.Pop()
		*depth--
	case "draw_image":
		if o.Image == nil || res == nil || *o.Image < 0 || *o.Image >= len(res.images) {
			return errors.New("the image of draw_image does not exist")
		}
		size, ok := path.SizeFromWH(o.Width, o.Height)
		if !ok {
			return errors.New("the size of an image is not valid")
		}
		s.DrawImage(res.images[*o.Image], size)
	case "draw_glyphs", "draw_text":
		return goDrawText(s, o, res)
	case "draw_svg":
		return goDrawSVG(s, o)
	default:
		return fmt.Errorf("the call %q is not ported", o.Op)
	}
	return nil
}

// surfaceBlendModes holds the blend modes of the oracle by name.
var surfaceBlendModes = map[string]pdf.BlendMode{
	"normal": pdf.BlendModeNormal, "multiply": pdf.BlendModeMultiply, "screen": pdf.BlendModeScreen,
	"overlay": pdf.BlendModeOverlay, "darken": pdf.BlendModeDarken, "lighten": pdf.BlendModeLighten,
	"color_dodge": pdf.BlendModeColorDodge, "color_burn": pdf.BlendModeColorBurn,
	"hard_light": pdf.BlendModeHardLight, "soft_light": pdf.BlendModeSoftLight,
	"difference": pdf.BlendModeDifference, "exclusion": pdf.BlendModeExclusion, "hue": pdf.BlendModeHue,
	"saturation": pdf.BlendModeSaturation, "color": pdf.BlendModeColor, "luminosity": pdf.BlendModeLuminosity,
}

// goPaint makes a paint as paint of the oracle does. No paint is black. A
// pattern draws its calls into a stream of the surface.
func goPaint(s *pdf.Surface, p *paintSpec, res *sceneResources) (pdf.Paint, error) {
	switch {
	case p == nil:
		return pdf.ColorLumaBlack(), nil
	case p.RGB != nil:
		return pdf.NewColorRGB(p.RGB[0], p.RGB[1], p.RGB[2]), nil
	case p.Luma != nil:
		return pdf.NewColorLuma(*p.Luma), nil
	case p.CMYK != nil:
		return pdf.NewColorCMYK(p.CMYK[0], p.CMYK[1], p.CMYK[2], p.CMYK[3]), nil
	case p.Linear != nil:
		g := p.Linear
		spread, stops, err := goGradient(g)
		if err != nil {
			return nil, err
		}
		return pdf.LinearGradient{
			X1: g.X1, Y1: g.Y1, X2: g.X2, Y2: g.Y2, Transform: goTransform(g.Transform),
			SpreadMethod: spread, Stops: stops, AntiAlias: g.AntiAlias,
		}, nil
	case p.Radial != nil:
		g := p.Radial
		spread, stops, err := goGradient(g)
		if err != nil {
			return nil, err
		}
		return pdf.RadialGradient{
			Fx: g.Fx, Fy: g.Fy, Fr: g.Fr, Cx: g.Cx, Cy: g.Cy, Cr: g.Cr, Transform: goTransform(g.Transform),
			SpreadMethod: spread, Stops: stops, AntiAlias: g.AntiAlias,
		}, nil
	case p.Sweep != nil:
		g := p.Sweep
		spread, stops, err := goGradient(g)
		if err != nil {
			return nil, err
		}
		return pdf.SweepGradient{
			Cx: g.Cx, Cy: g.Cy, StartAngle: g.StartAngle, EndAngle: g.EndAngle, Transform: goTransform(g.Transform),
			SpreadMethod: spread, Stops: stops, AntiAlias: g.AntiAlias,
		}, nil
	case p.Pattern != nil:
		sb := s.StreamBuilder()
		sub := sb.Surface()
		err := goRunSpecs(sub, p.Pattern.Ops, res)
		sub.Finish()
		stream := sb.Finish()
		if err != nil {
			return nil, err
		}
		return pdf.Pattern{
			Stream: stream, Transform: goTransform(p.Pattern.Transform),
			Width: p.Pattern.Width, Height: p.Pattern.Height,
		}, nil
	}
	return nil, errors.New("the paint is not known")
}

// goGradient is spread and stops of the oracle.
func goGradient(g *gradientSpec) (pdf.SpreadMethod, []pdf.Stop, error) {
	var spread pdf.SpreadMethod
	switch {
	case g.Spread == nil || *g.Spread == "pad":
		spread = pdf.SpreadMethodPad
	case *g.Spread == "reflect":
		spread = pdf.SpreadMethodReflect
	case *g.Spread == "repeat":
		spread = pdf.SpreadMethodRepeat
	default:
		return 0, nil, fmt.Errorf("the spread method %q is not known", *g.Spread)
	}
	stops := make([]pdf.Stop, 0, len(g.Stops))
	for _, st := range g.Stops {
		offset, err := goNormalized(&st.Offset)
		if err != nil {
			return 0, nil, err
		}
		color, err := goPaint(nil, &st.Color, nil)
		if err != nil {
			return 0, nil, err
		}
		c, ok := color.(pdf.Color)
		if !ok {
			return 0, nil, errors.New("the color of a stop is not a color")
		}
		opacity, err := goNormalized(st.Opacity)
		if err != nil {
			return 0, nil, err
		}
		stops = append(stops, pdf.Stop{Offset: offset, Color: c, Opacity: opacity})
	}
	return spread, stops, nil
}

// goTransform is transform of the oracle. No transform is the identity.
func goTransform(t *[6]float32) pdf.Transform {
	if t == nil {
		return path.IdentityTransform()
	}
	return path.TransformFromRow(t[0], t[1], t[2], t[3], t[4], t[5])
}

// goNormalized is normalized of the oracle. No value is 1.
func goNormalized(v *float32) (pdf.NormalizedF32, error) {
	if v == nil {
		return pdf.NormalizedF32One, nil
	}
	n, ok := pdf.NewNormalizedF32(*v)
	if !ok {
		return 0, fmt.Errorf("%v is not between 0 and 1", *v)
	}
	return n, nil
}

// goFillRule is fill_rule of the oracle.
func goFillRule(r *string) (pdf.FillRule, error) {
	if r == nil || *r == "nonzero" {
		return pdf.FillRuleNonZero, nil
	}
	if *r == "evenodd" {
		return pdf.FillRuleEvenOdd, nil
	}
	return 0, fmt.Errorf("the fill rule %q is not known", *r)
}

// goPath is path of the oracle.
func goPath(segments [][]float64) (*pdf.Path, error) {
	b := path.NewPathBuilder()
	for _, s := range segments {
		v := make([]float32, len(s))
		for i, x := range s {
			v[i] = float32(x)
		}
		want := map[int]int{0: 3, 1: 3, 2: 5, 3: 7, 4: 1}
		if len(v) == 0 || want[int(v[0])] != len(v) {
			return nil, fmt.Errorf("the segment %v is not known", s)
		}
		switch int(v[0]) {
		case 0:
			b.MoveTo(v[1], v[2])
		case 1:
			b.LineTo(v[1], v[2])
		case 2:
			b.QuadTo(v[1], v[2], v[3], v[4])
		case 3:
			b.CubicTo(v[1], v[2], v[3], v[4], v[5], v[6])
		default:
			b.Close()
		}
	}
	p, ok := b.Finish()
	if !ok {
		return nil, errors.New("the path is empty")
	}
	return p, nil
}

// surfaceCases returns the scenes of the surface.
func surfaceCases() []documentCase {
	var cases []documentCase
	pretty := map[string]any{"xmp_metadata": false, "pretty": true, "ascii_compatible": true, "compress_content_streams": false}
	add := func(name string, ops ...op) {
		cases = append(cases, documentCase{name, scene{Settings: pretty, Pages: []pageSpec{{Width: 200, Height: 200, Ops: ops}}}})
	}
	// stroke makes a stroke from pairs of a key and a value.
	stroke := func(kv ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[fmt.Sprint(kv[i])] = kv[i+1]
		}
		return m
	}
	curve := []seg{{0, 20, 20}, {2, 100, 0, 180, 20}, {3, 200, 100, 100, 200, 20, 180}, {4}}
	star := []seg{{0, 100, 10}, {1, 160, 190}, {1, 10, 70}, {1, 190, 70}, {1, 40, 190}, {4}}
	twoRects := append(rect(10, 10, 100, 100), rect(50, 50, 100, 100)...)

	// The paths and the fills.
	add("the default fill", drawPath(rect(20, 20, 160, 160)))
	add("rgb", setFill(rgb(255, 0, 0), 1, "nonzero"), drawPath(rect(20, 20, 160, 160)))
	add("luma", setFill(luma(127), 1, "nonzero"), drawPath(rect(20, 20, 160, 160)))
	add("cmyk", setFill(cmyk(0, 8, 252, 5), 1, "nonzero"), drawPath(rect(20, 20, 160, 160)))
	add("fill opacity", setFill(rgb(255, 0, 0), 0.5, "nonzero"), drawPath(rect(20, 20, 160, 160)))
	add("zero opacity", setFill(rgb(255, 0, 0), 0, "nonzero"), drawPath(rect(20, 20, 160, 160)))
	add("even odd", setFill(rgb(0, 0, 255), 1, "evenodd"), drawPath(star))
	add("nonzero", setFill(rgb(0, 0, 255), 1, "nonzero"), drawPath(star))
	add("two subpaths", setFill(luma(30), 0.7, "evenodd"), drawPath(twoRects))
	add("curves", setFill(rgb(10, 200, 30), 1, "nonzero"), drawPath(curve))
	add("fractions", setFill(rgb(1, 2, 3), 0.123, "nonzero"),
		drawPath([]seg{{0, 0.1, 0.2}, {1, 33.333, 0.2}, {2, 50.5, 70.25, 0.125, 99.999}, {4}}))
	add("open path", setFill(rgb(0, 0, 0), 1, "nonzero"), drawPath([]seg{{0, 10, 10}, {1, 100, 10}, {1, 100, 100}}))
	add("a line is not filled", setFill(rgb(0, 0, 0), 1, "nonzero"), drawPath([]seg{{0, 10, 10}, {1, 100, 100}}))
	add("a point is not drawn", setFill(rgb(0, 0, 0), 1, "nonzero"), drawPath([]seg{{0, 10, 10}, {1, 10, 10}}))
	add("a flat rectangle is not filled", setFill(rgb(0, 0, 0), 1, "nonzero"), drawPath(rect(10, 10, 100, 0)))
	add("no fill and no stroke", noFill(), noStroke(), drawPath(rect(10, 10, 50, 50)))
	add("a fill that repeats", setFill(rgb(0, 255, 0), 0.5, "nonzero"), drawPath(rect(0, 0, 50, 50)),
		setFill(rgb(255, 0, 0), 0.5, "nonzero"), drawPath(rect(50, 50, 50, 50)),
		setFill(rgb(0, 255, 0), 0.5, "nonzero"), drawPath(rect(100, 100, 50, 50)))

	// The strokes.
	add("default stroke", noFill(), setStroke(stroke()), drawPath(rect(20, 20, 160, 160)))
	add("stroke width", noFill(), setStroke(stroke("paint", rgb(255, 0, 0), "width", 7.5)), drawPath(curve))
	add("hairline", noFill(), setStroke(stroke("paint", rgb(255, 0, 0), "width", 0)), drawPath(curve))
	for _, c := range []string{"butt", "round", "square"} {
		for _, j := range []string{"miter", "round", "bevel"} {
			add("stroke "+c+" "+j, noFill(), setStroke(stroke("paint", luma(80), "width", 9, "line_cap", c, "line_join", j, "miter_limit", 2.5)),
				drawPath([]seg{{0, 20, 20}, {1, 180, 40}, {1, 30, 180}}))
		}
	}
	add("stroke dash", noFill(), setStroke(stroke("paint", cmyk(10, 20, 30, 40), "width", 3,
		"dash", map[string]any{"array": []float32{6, 3, 1, 3}, "offset": 2.5})), drawPath(curve))
	add("stroke opacity", noFill(), setStroke(stroke("paint", rgb(0, 0, 255), "width", 4, "opacity", 0.4)), drawPath(curve))
	add("fill and stroke", setFill(rgb(255, 255, 0), 1, "nonzero"),
		setStroke(stroke("paint", rgb(0, 0, 255), "width", 4)), drawPath(star))
	add("fill and stroke even odd", setFill(luma(200), 0.5, "evenodd"),
		setStroke(stroke("paint", cmyk(0, 0, 0, 255), "width", 2, "line_join", "round")), drawPath(star))
	add("fill and stroke with opacity", setFill(rgb(255, 255, 0), 1, "nonzero"),
		setStroke(stroke("paint", rgb(0, 0, 255), "width", 4, "opacity", 0.5)), drawPath(star))
	add("a line with a stroke", setFill(rgb(0, 0, 0), 1, "nonzero"),
		setStroke(stroke("paint", rgb(255, 0, 0), "width", 5)), drawPath([]seg{{0, 10, 10}, {1, 100, 100}}))
	for _, c := range []string{"butt", "round", "square"} {
		add("a point with a "+c+" stroke", noFill(), setStroke(stroke("paint", rgb(255, 0, 0), "width", 10, "line_cap", c)),
			drawPath([]seg{{0, 40, 40}, {1, 40, 40}}), drawPath([]seg{{0, 80, 80}, {1, 80, 100}}))
	}

	// The transforms.
	add("transforms", pushTransform(1, 0, 0, 1, 50, 50), setFill(rgb(0, 255, 0), 1, "nonzero"),
		drawPath(rect(0, 0, 100, 100)), pushTransform(1, 0, 0, 1, 100, 100), setFill(rgb(255, 0, 0), 1, "nonzero"),
		drawPath(rect(0, 0, 100, 100)), pop(), pop())
	add("scale skew rotate", pushTransform(2, 0.5, -0.25, 1.5, 10, 20), setFill(luma(60), 1, "nonzero"),
		drawPath(curve), pushTransform(0.70710677, 0.70710677, -0.70710677, 0.70710677, 0, 0),
		setStroke(stroke("width", 2)), drawPath(rect(0, 0, 30, 30)), pop(), pop())
	add("identity transform", pushTransform(1, 0, 0, 1, 0, 0), drawPath(rect(5, 5, 10, 10)), pop())
	add("mirror", pushTransform(-1, 0, 0, 1, 200, 0), setFill(rgb(9, 99, 199), 1, "nonzero"), drawPath(curve), pop())

	// The clips.
	add("clip nonzero", pushClipPath(star, "nonzero"), setFill(rgb(255, 0, 0), 1, "nonzero"),
		drawPath(rect(0, 0, 200, 200)), pop())
	add("clip even odd", pushClipPath(star, "evenodd"), setFill(rgb(255, 0, 0), 1, "nonzero"),
		drawPath(rect(0, 0, 200, 200)), pop())
	add("nested clips", pushClipPath(rect(10, 10, 150, 150), "nonzero"), pushTransform(1, 0, 0, 1, 30, 30),
		pushClipPath(curve, "evenodd"), setFill(luma(0), 1, "nonzero"), drawPath(rect(0, 0, 200, 200)),
		pop(), pop(), drawPath(rect(0, 0, 20, 20)), pop())
	add("a clip under a scale", pushTransform(0.5, 0, 0, 0.5, 0, 0), pushClipPath(curve, "nonzero"),
		drawPath(rect(0, 0, 400, 400)), pop(), pop())
	var deep []op
	for range 30 {
		deep = append(deep, pushClipPath(rect(0, 0, 100, 100), "nonzero"))
	}
	deep = append(deep, drawPath(rect(0, 0, 10, 10)))
	for range 30 {
		deep = append(deep, pop())
	}
	add("thirty clips", deep...)

	// The opacities, the isolated groups and the blend modes.
	add("opacity", pushOpacity(0.5), setFill(rgb(255, 0, 0), 1, "nonzero"), drawPath(rect(10, 10, 100, 100)), pop())
	add("opacity 1", pushOpacity(1), setFill(rgb(255, 0, 0), 1, "nonzero"), drawPath(rect(10, 10, 100, 100)), pop())
	add("nested opacities", pushTransform(1, 0, 0, 1, 20, 20), pushOpacity(0.5),
		setFill(rgb(255, 0, 0), 0.8, "nonzero"), drawPath(rect(10, 10, 100, 100)), pushOpacity(0.25),
		pushTransform(2, 0, 0, 2, 0, 0), setStroke(stroke("paint", rgb(0, 0, 255), "width", 3, "opacity", 0.5)),
		drawPath(rect(10, 10, 20, 20)), pop(), pop(), pop(), pop())
	add("an empty opacity", pushOpacity(0.5), pop())
	add("isolated", pushIsolated(), setFill(rgb(0, 128, 0), 1, "nonzero"), drawPath(curve), pop())
	add("nested isolated groups", pushIsolated(), pushTransform(1, 0, 0, 1, 50, 0), pushIsolated(),
		drawPath(rect(0, 0, 50, 50)), pop(), pop(), drawPath(rect(100, 100, 50, 50)), pop())
	for name := range surfaceBlendModes {
		add("blend mode "+name, setFill(rgb(0, 0, 255), 1, "nonzero"), drawPath(rect(0, 0, 120, 120)),
			pushBlendMode(name), setFill(rgb(255, 128, 0), 0.75, "nonzero"), drawPath(rect(60, 60, 120, 120)), pop())
	}
	add("blend modes that nest", pushBlendMode("multiply"), pushOpacity(0.5), pushBlendMode("screen"),
		setFill(rgb(1, 2, 3), 0.5, "nonzero"), drawPath(rect(0, 0, 100, 100)), pop(),
		drawPath(rect(50, 50, 100, 100)), pop(), pop())

	// The masks.
	for _, mt := range []string{"luminosity", "alpha"} {
		add("mask "+mt, pushMask(mt, setFill(rgb(255, 0, 0), 0.2, "nonzero"), drawPath(rect(20, 20, 160, 160))),
			setFill(rgb(0, 255, 0), 0.5, "nonzero"), drawPath(rect(0, 0, 100, 100)), pop())
	}
	masked := pushMask("luminosity", setFill(luma(200), 1, "nonzero"), drawPath(curve))
	masked["bbox"] = []float32{0, 0, 150, 120}
	add("mask with a bbox", pushTransform(1, 0, 0, 1, 20, 10), masked, setFill(rgb(0, 0, 0), 1, "nonzero"),
		drawPath(rect(0, 0, 200, 200)), pop(), pop())
	add("nested masks", pushMask("alpha", setFill(rgb(0, 0, 0), 0.5, "nonzero"), drawPath(rect(0, 0, 150, 150))),
		pushTransform(1, 0, 0, 1, 25, 25),
		pushMask("luminosity", pushIsolated(), setFill(luma(255), 1, "nonzero"), drawPath(star), pop()),
		drawPath(rect(0, 0, 200, 200)), pop(), pop(), pop())

	// The gradients. Each gradient fills or strokes the shape of the scene.
	stops2 := []map[string]any{stop(0.2, rgb(255, 0, 0), 1), stop(0.8, rgb(255, 255, 0), 1)}
	stopsOpacity := []map[string]any{stop(0.2, rgb(255, 0, 0), 1), stop(0.8, rgb(255, 255, 0), 0.5)}
	lumaStops := []map[string]any{stop(0.1, luma(0), 0.3), stop(0.5, luma(128), 1), stop(0.9, luma(255), 0.7)}
	cmykStops := []map[string]any{stop(0.25, cmyk(0, 50, 100, 0), 1), stop(0.75, cmyk(100, 0, 0, 20), 1)}
	rgbStops := []map[string]any{stop(0, rgb(255, 0, 0), 1), stop(0.4, rgb(0, 255, 0), 1), stop(1, rgb(0, 0, 255), 1)}
	var manyStops []map[string]any
	for i := range 120 {
		o := float32(1)
		if i%5 == 0 {
			o = 0.5
		}
		manyStops = append(manyStops, stop(float32(float64(i)/119), rgb(i*2%256, (255-i)%256, i*7%256), o))
	}
	linearAt := func(x1, y1, x2, y2 float32, spread string, stops []map[string]any) map[string]any {
		return gradient("linear", map[string]any{"x1": x1, "y1": y1, "x2": x2, "y2": y2}, spread, nil, stops)
	}
	radialAt := func(fx, fy, fr, cx, cy, cr float32, transform []float32, stops []map[string]any) map[string]any {
		return gradient("radial", map[string]any{"fx": fx, "fy": fy, "fr": fr, "cx": cx, "cy": cy, "cr": cr}, "pad", transform, stops)
	}
	sweepAt := func(cx, cy, start, end float32, spread string, stops []map[string]any) map[string]any {
		return gradient("sweep", map[string]any{"cx": cx, "cy": cy, "start_angle": start, "end_angle": end}, spread, nil, stops)
	}
	for _, spread := range []string{"pad", "reflect", "repeat"} {
		add("linear "+spread, setFill(linearAt(50, 0, 150, 0, spread, stops2), 1, "nonzero"), drawPath(rect(20, 20, 160, 160)))
		add("linear "+spread+" with opacities", setFill(linearAt(50, 10, 150, 110, spread, stopsOpacity), 1, "nonzero"),
			drawPath(curve))
		add("sweep "+spread, setFill(sweepAt(100, 100, 10, 300, spread, lumaStops), 1, "nonzero"), drawPath(rect(20, 20, 160, 160)))
	}
	lr := gradient("linear", map[string]any{"x1": 50, "y1": 10, "x2": 150, "y2": 110}, "reflect", []float32{1, 0, 0, 1, 5, 7}, stops2)
	add("linear reflect translated", setFill(lr, 1, "nonzero"), drawPath(rect(20, 20, 140, 140)))
	add("linear cmyk", setFill(linearAt(20, 20, 140, 100, "pad", cmykStops), 1, "nonzero"), drawPath(rect(20, 20, 140, 140)))
	add("linear anti-aliased", setFill(gradient("linear", map[string]any{"x1": 0, "y1": 0, "x2": 200, "y2": 200, "anti_alias": true},
		"pad", nil, rgbStops), 1, "nonzero"), drawPath(star))
	add("radial", setFill(radialAt(70, 70, 5, 80, 80, 60, []float32{2, 0, 0, 1, -50, 0}, lumaStops), 0.5, "nonzero"),
		drawPath(rect(20, 20, 140, 140)))
	add("radial opaque", setFill(radialAt(80, 80, 0, 80, 80, 60, nil, rgbStops), 1, "nonzero"), drawPath(rect(20, 20, 140, 140)))
	add("radial transformed", pushTransform(0.5, 0.2, -0.2, 0.5, 30, 10),
		setFill(radialAt(80, 80, 0, 80, 80, 60, nil, rgbStops), 1, "nonzero"), drawPath(rect(20, 20, 140, 140)), pop())
	add("sweep with many stops", setFill(sweepAt(80, 80, 0, 120, "repeat", manyStops), 1, "nonzero"), drawPath(rect(20, 20, 140, 140)))
	add("linear with many stops", setFill(linearAt(0, 0, 200, 50, "pad", manyStops), 1, "nonzero"), drawPath(rect(20, 20, 140, 140)))
	add("one stop", setFill(radialAt(120, 120, 60, 100, 100, 30, nil, []map[string]any{stop(0.5, rgb(255, 0, 0), 1)}), 1, "nonzero"),
		drawPath(rect(20, 20, 160, 160)))
	add("one stop with an opacity", setFill(linearAt(0, 0, 10, 0, "repeat", []map[string]any{stop(0.5, cmyk(1, 2, 3, 4), 0.25)}), 1, "nonzero"),
		drawPath(rect(20, 20, 160, 160)))
	noStops := []map[string]any{}
	add("no stop", setFill(linearAt(0, 0, 10, 0, "reflect", noStops), 1, "nonzero"), drawPath(rect(20, 20, 160, 160)))
	add("no stop in the stroke", setFill(rgb(0, 0, 255), 1, "nonzero"),
		setStroke(stroke("paint", sweepAt(100, 100, 0, 360, "pad", noStops), "width", 8)), drawPath(star))
	add("no stop in the fill", setFill(radialAt(100, 100, 0, 100, 100, 90, nil, noStops), 1, "nonzero"),
		setStroke(stroke("paint", rgb(255, 0, 0), "width", 4)), drawPath(curve))
	add("gradient stroke", noFill(), setStroke(stroke("paint", linearAt(0, 0, 200, 0, "pad", rgbStops), "width", 12)), drawPath(curve))
	add("gradient stroke with opacities", noFill(),
		setStroke(stroke("paint", sweepAt(100, 100, 0, 360, "pad", lumaStops), "width", 8, "line_join", "round")), drawPath(star))
	add("gradient fill and stroke", setFill(radialAt(100, 100, 0, 100, 100, 90, nil, stopsOpacity), 1, "evenodd"),
		setStroke(stroke("paint", linearAt(0, 0, 0, 200, "reflect", cmykStops), "width", 6, "opacity", 0.5)), drawPath(star))
	add("gradient in a group", pushIsolated(), pushOpacity(0.5),
		setFill(linearAt(0, 0, 100, 100, "repeat", stopsOpacity), 1, "nonzero"), drawPath(rect(10, 10, 180, 180)), pop(), pop())
	add("gradient in a mask", pushMask("luminosity", setFill(linearAt(0, 0, 200, 0, "pad", lumaStops), 1, "nonzero"),
		drawPath(rect(0, 0, 200, 200))), setFill(rgb(0, 0, 255), 1, "nonzero"), drawPath(rect(20, 20, 160, 160)), pop())

	// The patterns. basic is basic_pattern_stream of krilla
	// tests/src/main.rs.
	basic := []op{setFill(rgb(255, 0, 0), 1, "nonzero"), drawPath(rect(0, 0, 10, 10)), pushTransform(1, 0, 0, 1, 10, 10),
		setFill(rgb(0, 255, 0), 1, "nonzero"), drawPath(rect(0, 0, 10, 10)), pop()}
	add("pattern", setFill(pattern(20, 20, nil, basic...), 0.5, "nonzero"), drawPath(rect(20, 20, 140, 140)))
	add("pattern tiling basic", pushTransform(1, 0, 0, 1, -20, -20),
		setFill(pattern(20, 20, nil, basic...), 0.5, "nonzero"), drawPath(rect(20, 20, 160, 160)), pop())
	add("opaque pattern", setFill(pattern(20, 20, nil, basic...), 1, "nonzero"), drawPath(star))
	add("pattern transformed", pushTransform(1.5, 0, 0, 1.5, 10, 0),
		setFill(pattern(15, 25, []float32{0.8, 0.3, -0.3, 0.8, 5, 5}, basic...), 1, "nonzero"), drawPath(curve), pop())
	add("pattern stroke", noFill(), setStroke(stroke("paint", pattern(20, 20, nil, basic...), "width", 10, "opacity", 0.6)), drawPath(curve))
	add("pattern of a gradient", setFill(pattern(40, 40, nil, setFill(linearAt(0, 0, 40, 40, "pad", stopsOpacity), 1, "nonzero"),
		drawPath(rect(0, 0, 40, 40))), 1, "nonzero"), drawPath(rect(10, 10, 180, 180)))
	add("pattern of a pattern", setFill(pattern(40, 40, nil, setFill(pattern(10, 10, nil, basic...), 0.5, "nonzero"),
		drawPath(rect(0, 0, 30, 30))), 0.8, "nonzero"), drawPath(rect(10, 10, 180, 180)))
	add("pattern in a mask", pushMask("alpha", setFill(pattern(20, 20, nil, basic...), 0.7, "nonzero"),
		drawPath(rect(0, 0, 200, 200))), setFill(rgb(0, 0, 0), 1, "nonzero"), drawPath(rect(20, 20, 160, 160)), pop())

	// The ICC color spaces, with no_device_cs.
	icc := func(name string, extra map[string]any, ops ...op) {
		settings := map[string]any{"xmp_metadata": false, "pretty": true, "ascii_compatible": true,
			"compress_content_streams": false, "no_device_cs": true}
		maps.Copy(settings, extra)
		cases = append(cases, documentCase{name, scene{Settings: settings, Pages: []pageSpec{{Width: 200, Height: 200, Ops: ops}}}})
	}
	iccOps := []op{setFill(rgb(255, 0, 0), 1, "nonzero"), drawPath(rect(0, 0, 100, 100)),
		setFill(luma(100), 0.5, "nonzero"), drawPath(rect(50, 50, 100, 100)),
		setFill(cmyk(10, 20, 30, 40), 1, "nonzero"), drawPath(rect(100, 100, 100, 100)),
		noFill(), setStroke(stroke("paint", rgb(0, 0, 255), "width", 3)), drawPath(curve)}
	gradientOps := []op{setFill(linearAt(0, 0, 200, 0, "pad", stopsOpacity), 1, "nonzero"), drawPath(rect(0, 0, 100, 100)),
		setFill(sweepAt(100, 100, 0, 270, "reflect", lumaStops), 1, "nonzero"), drawPath(rect(100, 0, 100, 100)),
		setFill(radialAt(50, 150, 0, 50, 150, 50, nil, cmykStops), 1, "nonzero"), drawPath(rect(0, 100, 100, 100))}
	for _, v := range []string{"1.4", "1.7", "2.0"} {
		icc("icc colors "+v, map[string]any{"version": v}, iccOps...)
		icc("icc gradients "+v, map[string]any{"version": v}, gradientOps...)
	}
	icc("icc pattern and mask", nil, pushMask("luminosity", setFill(luma(200), 1, "nonzero"), drawPath(curve)),
		setFill(pattern(20, 20, nil, basic...), 0.5, "nonzero"), drawPath(rect(0, 0, 200, 200)), pop())

	// More pages and other settings.
	pages := []pageSpec{
		{Width: 100, Height: 50, Ops: []op{setFill(rgb(255, 0, 0), 0.5, "nonzero"), drawPath(rect(0, 0, 50, 50))}},
		{Width: 300, Height: 300, Ops: []op{pushOpacity(0.5), setFill(rgb(255, 0, 0), 0.5, "nonzero"), drawPath(rect(0, 0, 50, 50)), pop()}},
		{Width: 120, Height: 80},
		{MediaBox: xywh(-50, -50, 200, 200), Ops: []op{pushClipPath(curve, "nonzero"), drawPath(rect(-50, -50, 300, 300)), pop()}},
	}
	for _, settings := range []map[string]any{
		pretty,
		{"xmp_metadata": false},
		{"xmp_metadata": false, "pretty": false, "compress_content_streams": false},
		{"xmp_metadata": false, "version": "1.4", "pretty": true, "compress_content_streams": false},
		{"xmp_metadata": false, "version": "2.0", "pretty": true, "compress_content_streams": false},
		{"xmp_metadata": false, "version": "2.0", "ascii_compatible": true},
		{"xmp_metadata": false, "no_device_cs": true},
	} {
		cases = append(cases, documentCase{fmt.Sprintf("pages %v", settings), scene{Settings: settings, Pages: pages}})
	}
	return cases
}

// gradient is a gradient of a scene: the kind "linear", "radial" or
// "sweep", its geometry, its spread method, its transform or nil, and its
// stops.
func gradient(kind string, fields map[string]any, spread string, transform []float32, stops []map[string]any) map[string]any {
	g := map[string]any{"spread": spread, "stops": stops}
	maps.Copy(g, fields)
	if transform != nil {
		g["transform"] = transform
	}
	return map[string]any{kind: g}
}

// pattern is a pattern of a scene, whose calls draw its stream.
func pattern(width, height float32, transform []float32, ops ...op) map[string]any {
	p := map[string]any{"width": width, "height": height, "ops": ops}
	if transform != nil {
		p["transform"] = transform
	}
	return map[string]any{"pattern": p}
}

// randomOps makes a random list of calls of the surface with every paint,
// with a pop for each push.
func randomOps(r *rand.Rand, depth int) []op {
	coord := func() float64 { return float64(float32(r.Float64()*240 - 20)) }
	unit := func() float32 { return float32(r.IntN(11)) / 10 }
	color := func() map[string]any {
		switch r.IntN(3) {
		case 0:
			return rgb(r.IntN(256), r.IntN(256), r.IntN(256))
		case 1:
			return luma(r.IntN(256))
		default:
			return cmyk(r.IntN(256), r.IntN(256), r.IntN(256), r.IntN(256))
		}
	}
	// paint makes a color, or one time in four a gradient or a pattern. The
	// stops of a gradient have colors of one color space, because krilla
	// panics for two.
	paint := func() map[string]any {
		if r.IntN(4) != 0 {
			return color()
		}
		spread := []string{"pad", "reflect", "repeat"}[r.IntN(3)]
		var transform []float32
		if r.IntN(2) == 0 {
			transform = []float32{float32(r.Float64() + 0.5), float32(r.Float64() - 0.5), float32(r.Float64() - 0.5),
				float32(r.Float64() + 0.5), float32(coord() / 4), float32(coord() / 4)}
		}
		var stops []map[string]any
		model := r.IntN(3)
		for range 1 + r.IntN(5) {
			var c map[string]any
			switch model {
			case 0:
				c = rgb(r.IntN(256), r.IntN(256), r.IntN(256))
			case 1:
				c = luma(r.IntN(256))
			default:
				c = cmyk(r.IntN(256), r.IntN(256), r.IntN(256), r.IntN(256))
			}
			stops = append(stops, stop(unit(), c, unit()))
		}
		switch k := r.IntN(4); {
		case k == 0:
			return gradient("linear", map[string]any{"x1": coord(), "y1": coord(), "x2": coord(), "y2": coord(),
				"anti_alias": r.IntN(2) == 0}, spread, transform, stops)
		case k == 1:
			return gradient("radial", map[string]any{"fx": coord(), "fy": coord(), "fr": float32(r.IntN(40)),
				"cx": coord(), "cy": coord(), "cr": float32(1 + r.IntN(100))}, "pad", transform, stops)
		case k == 2 || depth >= 3:
			return gradient("sweep", map[string]any{"cx": coord(), "cy": coord(), "start_angle": float32(r.IntN(360)),
				"end_angle": float32(r.IntN(720) - 180)}, spread, transform, stops)
		default:
			return pattern(float32(5+r.IntN(40)), float32(5+r.IntN(40)), transform, randomOps(r, depth+1)...)
		}
	}
	rule := func() string { return []string{"nonzero", "evenodd"}[r.IntN(2)] }
	randomPath := func() []seg {
		p := []seg{{0, coord(), coord()}}
		for range 1 + r.IntN(5) {
			switch r.IntN(5) {
			case 0, 1:
				p = append(p, seg{1, coord(), coord()})
			case 2:
				p = append(p, seg{2, coord(), coord(), coord(), coord()})
			case 3:
				p = append(p, seg{3, coord(), coord(), coord(), coord(), coord(), coord()})
			default:
				p = append(p, seg{4}, seg{0, coord(), coord()}, seg{1, coord(), coord()})
			}
		}
		if r.IntN(2) == 0 {
			p = append(p, seg{4})
		}
		return p
	}
	modes := make([]string, 0, len(surfaceBlendModes))
	for name := range surfaceBlendModes {
		modes = append(modes, name)
	}
	var ops []op
	for range 1 + r.IntN(8) {
		switch k := r.IntN(14); {
		case k < 2:
			if r.IntN(5) == 0 {
				ops = append(ops, noFill())
			} else {
				ops = append(ops, setFill(paint(), unit(), rule()))
			}
		case k < 4:
			if r.IntN(5) == 0 {
				ops = append(ops, noStroke())
				continue
			}
			s := map[string]any{
				"paint": paint(), "width": float32(r.IntN(40)) / 4, "opacity": unit(),
				"line_cap":    []string{"butt", "round", "square"}[r.IntN(3)],
				"line_join":   []string{"miter", "round", "bevel"}[r.IntN(3)],
				"miter_limit": float32(1 + r.IntN(12)),
			}
			if r.IntN(3) == 0 {
				s["dash"] = map[string]any{"array": []float32{float32(1 + r.IntN(8)), float32(1 + r.IntN(8))}, "offset": float32(r.IntN(5))}
			}
			ops = append(ops, setStroke(s))
		case k < 8:
			ops = append(ops, drawPath(randomPath()))
		case depth >= 3:
			ops = append(ops, drawPath(randomPath()))
		default:
			var push op
			switch r.IntN(6) {
			case 0:
				a := float32(r.Float64()*4 - 2)
				push = pushTransform(a, float32(r.Float64()-0.5), float32(r.Float64()-0.5), float32(r.Float64()*2+0.1),
					float32(coord()/4), float32(coord()/4))
			case 1:
				push = pushBlendMode(modes[r.IntN(len(modes))])
			case 2:
				push = pushClipPath(randomPath(), rule())
			case 3:
				push = pushOpacity(unit())
			case 4:
				push = pushIsolated()
			default:
				push = pushMask([]string{"luminosity", "alpha"}[r.IntN(2)], randomOps(r, depth+1)...)
			}
			ops = append(ops, push)
			ops = append(ops, randomOps(r, depth+1)...)
			ops = append(ops, pop())
		}
	}
	return ops
}

// compareScene compares the file of pdf for a scene with the answer of the
// oracle. The files can differ only where glibc does not give the correctly
// rounded rotation of a linear gradient, as libmRotations finds (D26).
func compareScene(t *testing.T, c documentCase, a answer) {
	t.Helper()
	if a.Err != "" {
		t.Fatalf("the oracle gives an error: %s", a.Err)
	}
	got, err := goScene(c.scene)
	if err != nil {
		t.Fatal(err)
	}
	want := a.Data
	var diffs []string
	// krilla compresses the ICC profiles of no_device_cs and the fonts in
	// every case, so their bytes differ (D62).
	compress, ok := c.scene.Settings["compress_content_streams"].(bool)
	noDeviceCS, _ := c.scene.Settings["no_device_cs"].(bool)
	if ok && !compress && !noDeviceCS && len(c.scene.Fonts) == 0 && !bytes.Equal(got, want) {
		diffs = append(diffs, "the bytes differ: "+firstDiff(got, want))
	}
	gotFile, err := readPDF(got)
	if err != nil {
		t.Fatalf("reading the file of pdf: %v", err)
	}
	wantFile, err := readPDF(want)
	if err != nil {
		t.Fatalf("reading the file of krilla: %v", err)
	}
	if d := comparePDF(gotFile, wantFile); len(d) != 0 {
		diffs = append(diffs, fmt.Sprintf("%d differences:\n%s", len(d), strings.Join(d[:min(len(d), 10)], "\n")))
	}
	if len(diffs) != 0 {
		if n := libmRotations(t, c.scene); n != 0 {
			t.Logf("glibc does not round the rotation of %d linear gradients correctly, so the files can differ (D26): %s", n, diffs[0])
		} else {
			for _, d := range diffs {
				t.Error(d)
			}
		}
	}
	qpdfCheck(t, writeTemp(t, "go.pdf", got))
}

// pointTsQuestion is the question "point_ts" of the oracle.
type pointTsQuestion struct {
	PointTs [][4]float32 `json:"point_ts"`
}

// libmRotations counts the linear gradients of a scene whose rotation in
// pdf differs from the rotation of the oracle. krilla rotates a linear
// gradient with the spread method reflect or repeat with atan2, sin and cos
// of the C library. pdf computes them correctly rounded, and glibc misses
// the correctly rounded result of sinf and cosf for about 1 percent of the
// angles (D26).
func libmRotations(t *testing.T, sc scene) int {
	t.Helper()
	var lines [][4]float32
	var walk func(ops []opSpec)
	paint := func(p *paintSpec) {
		switch {
		case p == nil:
		case p.Linear != nil && p.Linear.Spread != nil && *p.Linear.Spread != "pad":
			lines = append(lines, [4]float32{p.Linear.X1, p.Linear.Y1, p.Linear.X2, p.Linear.Y2})
		case p.Pattern != nil:
			walk(p.Pattern.Ops)
		}
	}
	walk = func(ops []opSpec) {
		for _, o := range ops {
			if o.Fill != nil {
				paint(o.Fill.Paint)
			}
			if o.Stroke != nil {
				paint(o.Stroke.Paint)
			}
			walk(o.Ops)
		}
	}
	for _, p := range sc.Pages {
		data, err := json.Marshal(p.Ops)
		if err != nil {
			t.Fatal(err)
		}
		var ops []opSpec
		if err := json.Unmarshal(data, &ops); err != nil {
			t.Fatal(err)
		}
		walk(ops)
	}
	if len(lines) == 0 {
		return 0
	}
	a := ask(t, pointTsQuestion{PointTs: lines})[0]
	if a.Err != "" || len(a.Data) != 24*len(lines) {
		t.Fatalf("the oracle gives a wrong answer to point_ts: %q", a.Err)
	}
	n := 0
	for i, l := range lines {
		// get_point_ts of krilla src/graphics/shading_function.rs.
		const degsPerRad float32 = 57.2957795130823208767981548141051703
		angle := crmath.Atan2_32(l[3]-l[1], l[2]-l[0]) * degsPerRad
		ts := path.TransformFromRotateAt(angle, l[0], l[1])
		for j, v := range []float32{ts.SX, ts.KY, ts.KX, ts.SY, ts.TX, ts.TY} {
			if math.Float32bits(v) != binary.LittleEndian.Uint32(a.Data[24*i+4*j:]) {
				n++
				break
			}
		}
	}
	return n
}

// TestSurfaceGo compares the files of pdf with the files of krilla for the
// scenes of surfaceCases.
func TestSurfaceGo(t *testing.T) {
	cases := surfaceCases()
	questions := make([]any, len(cases))
	for i, c := range cases {
		questions[i] = sceneQuestion{Krilla: c.scene}
	}
	answers := ask(t, questions...)
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			compareScene(t, c, answers[i])
		})
	}
}

// TestSurfaceGoRandom compares the files of pdf with the files of krilla
// for random scenes with every paint.
func TestSurfaceGoRandom(t *testing.T) {
	n := 300
	if testing.Short() {
		n = 30
	}
	r := rand.New(rand.NewPCG(5, 14))
	cases := make([]documentCase, n)
	for i := range cases {
		settings := map[string]any{"xmp_metadata": false, "pretty": r.IntN(2) == 0, "compress_content_streams": r.IntN(4) == 0}
		var pages []pageSpec
		for range 1 + r.IntN(2) {
			pages = append(pages, pageSpec{Width: 200, Height: 200, Ops: randomOps(r, 0)})
		}
		cases[i] = documentCase{fmt.Sprintf("random %d", i), scene{Settings: settings, Pages: pages}}
	}
	questions := make([]any, len(cases))
	for i, c := range cases {
		questions[i] = sceneQuestion{Krilla: c.scene}
	}
	answers := ask(t, questions...)
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			compareScene(t, c, answers[i])
		})
	}
}

// TestSurfaceGoErrors checks that pdf gives the errors of the oracle for the
// pushes and the pops of a list that do not match.
func TestSurfaceGoErrors(t *testing.T) {
	for _, ops := range [][]op{{pushOpacity(0.5)}, {pop()}} {
		s := scene{Settings: map[string]any{"xmp_metadata": false}, Pages: []pageSpec{{Width: 10, Height: 10, Ops: ops}}}
		a := ask(t, sceneQuestion{Krilla: s})[0]
		if a.Err == "" {
			t.Errorf("the oracle gives no error for %v", ops)
		}
		if _, err := goScene(s); err == nil {
			t.Errorf("pdf gives no error for %v", ops)
		}
	}
}
