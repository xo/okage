// Copyright 2024 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports velato src/runtime/model/animated.rs and
// src/runtime/model/spline.rs.

import (
	"math"

	"github.com/xo/okage/internal/kurbo"
	"github.com/xo/okage/internal/lottiecompat"
)

// PositionKind is the variant of Position in velato
// src/runtime/model/animated.rs.
type PositionKind uint8

// The variants of Position.
const (
	PositionValue PositionKind = iota
	PositionSplit
	PositionSpatial
)

// Position is Position in velato src/runtime/model/animated.rs (D27).
type Position struct {
	Kind    PositionKind
	Value   Value[kurbo.Point]
	X, Y    Value[float64]
	Spatial *SpatialPosition
}

// IsFixed is Position::is_fixed in velato src/runtime/model/animated.rs.
func (p *Position) IsFixed() bool {
	switch p.Kind {
	case PositionValue:
		return p.Value.IsFixed()
	case PositionSplit:
		return p.X.IsFixed() && p.Y.IsFixed()
	}
	return false
}

// Evaluate is Position::evaluate in velato src/runtime/model/animated.rs.
func (p *Position) Evaluate(frame float64) kurbo.Point {
	switch p.Kind {
	case PositionValue:
		return p.Value.Evaluate(frame)
	case PositionSplit:
		return kurbo.NewPoint(p.X.Evaluate(frame), p.Y.Evaluate(frame))
	}
	return p.Spatial.Evaluate(frame)
}

// AnimatedTransform is Transform in velato src/runtime/model/animated.rs.
type AnimatedTransform struct {
	// Anchor point.
	Anchor Position
	// Translation.
	Position Position
	// Rotation angle.
	Rotation Value[float64]
	// Scale factor.
	Scale Value[kurbo.Vec2]
	// Skew factor.
	Skew Value[float64]
	// Skew angle, 0 skews along the x axis, 90 along the y axis.
	SkewAngle Value[float64]
}

// IsFixed is Transform::is_fixed in velato src/runtime/model/animated.rs.
func (t *AnimatedTransform) IsFixed() bool {
	return t.Anchor.IsFixed() && t.Position.IsFixed() && t.Rotation.IsFixed() &&
		t.Scale.IsFixed() && t.Skew.IsFixed() && t.SkewAngle.IsFixed()
}

// Evaluate is Transform::evaluate in velato src/runtime/model/animated.rs.
func (t *AnimatedTransform) Evaluate(frame float64) kurbo.Affine {
	return t.Components(frame).Matrix()
}

// Components is Transform::components in velato
// src/runtime/model/animated.rs.
func (t *AnimatedTransform) Components(frame float64) TransformComponents {
	return TransformComponents{
		Anchor:    t.Anchor.Evaluate(frame),
		Position:  t.Position.Evaluate(frame),
		Rotation:  t.Rotation.Evaluate(frame),
		Scale:     t.Scale.Evaluate(frame),
		Skew:      t.Skew.Evaluate(frame),
		SkewAngle: t.SkewAngle.Evaluate(frame),
	}
}

// intoModel is Transform::into_model in velato
// src/runtime/model/animated.rs.
func (t *AnimatedTransform) intoModel() Transform {
	return Transform{Animated: t}
}

// AnimatedEllipse is Ellipse in velato src/runtime/model/animated.rs.
type AnimatedEllipse struct {
	// True if the ellipse is drawn in CCW order.
	IsCCW bool
	// Position of the ellipse.
	Position Position
	// Size of the ellipse.
	Size Value[kurbo.Size]
}

// Evaluate is Ellipse::evaluate in velato src/runtime/model/animated.rs,
// without the feature lottie-shape-paths.
func (e *AnimatedEllipse) Evaluate(frame float64, path []kurbo.PathEl) []kurbo.PathEl {
	center := e.Position.Evaluate(frame)
	size := e.Size.Evaluate(frame)
	ellipse := kurbo.NewEllipse(center, kurbo.NewVec2(size.Width*0.5, size.Height*0.5), 0.0)
	return append(path, ellipse.PathElements(0.1)...)
}

// AnimatedRect is Rect in velato src/runtime/model/animated.rs.
type AnimatedRect struct {
	// True if the rect is drawn in CCW order.
	IsCCW bool
	// Position of the rectangle.
	Position Position
	// Size of the rectangle.
	Size Value[kurbo.Size]
	// Radius of the rectangle corners.
	CornerRadius Value[float64]
}

// Evaluate is Rect::evaluate in velato src/runtime/model/animated.rs,
// without the feature lottie-shape-paths.
func (r *AnimatedRect) Evaluate(frame float64, path []kurbo.PathEl) []kurbo.PathEl {
	center := r.Position.Evaluate(frame)
	size := r.Size.Evaluate(frame)
	rect := kurbo.NewRoundedRect(
		center.X-size.Width*0.5,
		center.Y-size.Height*0.5,
		center.X+size.Width*0.5,
		center.Y+size.Height*0.5,
		r.CornerRadius.Evaluate(frame),
	)
	return append(path, rect.PathElements(0.1)...)
}

// AnimatedStar is Star in velato src/runtime/model/animated.rs: a star or a
// polygon.
type AnimatedStar struct {
	IsPolygon      bool
	Direction      float64
	Position       Position
	InnerRadius    Value[float64]
	InnerRoundness Value[float64]
	OuterRadius    Value[float64]
	OuterRoundness Value[float64]
	Rotation       Value[float64]
	Points         Value[float64]
}

// IsFixed is Star::is_fixed in velato src/runtime/model/animated.rs.
func (s *AnimatedStar) IsFixed() bool {
	return s.Position.IsFixed() && s.InnerRadius.IsFixed() && s.InnerRoundness.IsFixed() &&
		s.OuterRadius.IsFixed() && s.OuterRoundness.IsFixed() && s.Rotation.IsFixed() && s.Points.IsFixed()
}

// Evaluate is Star::evaluate in velato src/runtime/model/animated.rs. It
// appends the star or polygon at frame to path.
func (s *AnimatedStar) Evaluate(frame float64, path []kurbo.PathEl) []kurbo.PathEl {
	center := s.Position.Evaluate(frame)
	numPoints := roundToUsize(s.Points.Evaluate(frame))
	if numPoints == 0 {
		return path
	}
	rotationDeg := s.Rotation.Evaluate(frame)
	outerRadius := s.OuterRadius.Evaluate(frame)
	outerRoundness := s.OuterRoundness.Evaluate(frame) / 100.0

	isStar := !s.IsPolygon
	innerRadius, innerRoundness := outerRadius, 0.0
	if isStar {
		innerRadius = s.InnerRadius.Evaluate(frame)
		innerRoundness = s.InnerRoundness.Evaluate(frame) / 100.0
	}

	anglePerPoint := 2 * math.Pi / float64(numPoints)
	halfAngle := anglePerPoint / 2.0
	rotationRad := toRadians(rotationDeg) - math.Pi/2

	totalVertices := numPoints
	vertexAngleStep := anglePerPoint
	if isStar {
		totalVertices = numPoints * 2
		vertexAngleStep = halfAngle
	}

	type vertex struct{ p, in, out kurbo.Point }
	vertices := make([]vertex, 0, totalVertices)
	currentAngle := rotationRad

	for i := range totalVertices {
		radius, roundness := outerRadius, outerRoundness
		if isStar && i%2 != 0 {
			radius, roundness = innerRadius, innerRoundness
		}

		x := center.X + radius*kurbo.Cos(currentAngle)
		y := center.Y + radius*kurbo.Sin(currentAngle)

		inPt, outPt := kurbo.NewPoint(x, y), kurbo.NewPoint(x, y)
		if math.Abs(roundness) > 1e-6 {
			tangentLen := radius * kurbo.Tan(vertexAngleStep/2.0) * roundness * 4.0 / 3.0
			perpX := -kurbo.Sin(currentAngle)
			perpY := kurbo.Cos(currentAngle)
			inPt = kurbo.NewPoint(x-tangentLen*perpX, y-tangentLen*perpY)
			outPt = kurbo.NewPoint(x+tangentLen*perpX, y+tangentLen*perpY)
		}

		vertices = append(vertices, vertex{kurbo.NewPoint(x, y), inPt, outPt})
		currentAngle += vertexAngleStep
	}

	if len(vertices) == 0 {
		return path
	}

	near := func(a, b kurbo.Point) bool {
		return math.Abs(a.X-b.X) < 1e-6 && math.Abs(a.Y-b.Y) < 1e-6
	}
	path = append(path, kurbo.MoveTo(vertices[0].p))
	for i := 1; i < len(vertices); i++ {
		prev, curr := vertices[i-1], vertices[i]
		if near(prev.out, prev.p) && near(curr.in, curr.p) {
			path = append(path, kurbo.LineTo(curr.p))
		} else {
			path = append(path, kurbo.CurveTo(prev.out, curr.in, curr.p))
		}
	}
	last, first := vertices[len(vertices)-1], vertices[0]
	if near(last.out, last.p) && near(first.in, first.p) {
		return append(path, kurbo.ClosePath())
	}
	return append(path, kurbo.CurveTo(last.out, first.in, first.p), kurbo.ClosePath())
}

// roundToUsize is f64::round followed by the cast "as usize": a negative or
// NaN value gives 0.
func roundToUsize(x float64) int {
	r := math.Round(x)
	if !(r > 0) {
		return 0
	}
	if r >= math.MaxInt {
		return math.MaxInt
	}
	return int(r)
}

// intoModel is Star::into_model in velato src/runtime/model/animated.rs.
func (s *AnimatedStar) intoModel() Geometry {
	if s.IsFixed() {
		return Geometry{Kind: GeometryFixed, Path: s.Evaluate(0.0, nil)}
	}
	return Geometry{Kind: GeometryStar, Star: s}
}

// AnimatedSpline is Spline in velato src/runtime/model/animated.rs: an
// animated cubic spline.
type AnimatedSpline struct {
	// True if the spline is closed.
	IsClosed bool
	// Collection of times.
	Times []Time
	// Collection of splines for each time.
	Values []SplineKeyframeValues
}

// SplineKeyframeValues is SplineKeyframeValues in velato
// src/runtime/model/animated.rs. When End is nil, the next keyframe's start
// is used.
type SplineKeyframeValues struct {
	Start []kurbo.Point
	End   []kurbo.Point
	// Port: whether the start and the end of this keyframe are closed
	// (D43).
	StartClosed, EndClosed bool
}

// Evaluate is Spline::evaluate in velato src/runtime/model/animated.rs. It
// appends the spline at frame to path.
func (s *AnimatedSpline) Evaluate(frame float64, path []kurbo.PathEl) ([]kurbo.PathEl, bool) {
	ix, t, easing, hold, ok := framesAndWeight(s.Times, frame)
	if !ok {
		return path, false
	}
	if hold {
		t = 0.0
	}
	// Port: velato blends the shapes with the linear time, and ignores the
	// easing. okage eases the time, as ThorVG does (D43).
	if !lottiecompat.Velato {
		t = tweenF64(0, 1, t, easing)
	}
	if ix[0] >= len(s.Values) || ix[1] >= len(s.Values) {
		return path, false
	}
	from, to := s.Values[ix[0]], s.Values[ix[1]]
	toSlice := to.Start
	if from.End != nil {
		toSlice = from.End
	}
	closed := s.IsClosed
	if !lottiecompat.Velato {
		// Port: velato closes the shape when any keyframe is closed, and
		// blends any two keyframes. okage closes the shape as its keyframe
		// is closed. When one keyframe is closed and the other is open,
		// ThorVG cannot blend them, because their points differ in number,
		// and it keeps the first keyframe. okage does the same (D43).
		toClosed := to.StartClosed
		if from.End != nil {
			toClosed = from.EndClosed
		}
		if from.StartClosed != toClosed {
			t = 0
		}
		closed = from.StartClosed
	}
	n := min(len(from.Start), len(toSlice))
	get := func(i int) kurbo.Point {
		// TODO: This definitely shouldn't be a lerp
		return from.Start[i].Lerp(toSlice[i], t)
	}
	path = splineToPath(n, get, closed, path)
	return path, true
}

// splineToPath is SplineToPath::to_path in velato
// src/runtime/model/spline.rs. The spline has n points, three for each
// vertex: the vertex, its in tangent and its out tangent, which get gives.
func splineToPath(n int, get func(int) kurbo.Point, isClosed bool, path []kurbo.PathEl) []kurbo.PathEl {
	if n == 0 {
		return path
	}
	path = append(path, kurbo.MoveTo(get(0)))
	nVertices := n / 3
	addElement := func(fromVertex, toVertex int) {
		fromIndex := 3 * fromVertex
		toIndex := 3 * toVertex
		p0 := get(fromIndex)
		p1 := get(toIndex)
		c0 := get(fromIndex + 2)
		c0.X += p0.X
		c0.Y += p0.Y
		c1 := get(toIndex + 1)
		c1.X += p1.X
		c1.Y += p1.Y
		if c0 == p0 && c1 == p1 {
			path = append(path, kurbo.LineTo(p1))
		} else {
			path = append(path, kurbo.CurveTo(c0, c1, p1))
		}
	}
	for i := 1; i < nVertices; i++ {
		addElement(i-1, i)
	}
	if isClosed && nVertices != 0 {
		addElement(nVertices-1, 0)
		path = append(path, kurbo.ClosePath())
	}
	return path
}

// AnimatedRepeater is Repeater in velato src/runtime/model/animated.rs.
type AnimatedRepeater struct {
	// Number of times elements are repeated.
	Copies Value[float64]
	// Offset applied to each element.
	Offset Value[float64]
	// Anchor point.
	AnchorPoint Position
	// Translation.
	Position Position
	// Rotation in degrees.
	Rotation Value[float64]
	// Scale.
	Scale Value[kurbo.Vec2]
	// Opacity of the first element.
	StartOpacity Value[float64]
	// Opacity of the last element.
	EndOpacity Value[float64]
	// How copies are stacked relative to the original shape.
	Composite RepeaterComposite
}

// IsFixed is Repeater::is_fixed in velato src/runtime/model/animated.rs.
func (r *AnimatedRepeater) IsFixed() bool {
	return r.Copies.IsFixed() && r.Offset.IsFixed() && r.AnchorPoint.IsFixed() &&
		r.Position.IsFixed() && r.Rotation.IsFixed() && r.Scale.IsFixed() &&
		r.StartOpacity.IsFixed() && r.EndOpacity.IsFixed()
}

// Evaluate is Repeater::evaluate in velato src/runtime/model/animated.rs.
func (r *AnimatedRepeater) Evaluate(frame float64) FixedRepeater {
	return FixedRepeater{
		Copies:       roundToUsize(r.Copies.Evaluate(frame)),
		Offset:       r.Offset.Evaluate(frame),
		AnchorPoint:  r.AnchorPoint.Evaluate(frame),
		Position:     r.Position.Evaluate(frame),
		Rotation:     r.Rotation.Evaluate(frame),
		Scale:        r.Scale.Evaluate(frame),
		StartOpacity: r.StartOpacity.Evaluate(frame),
		EndOpacity:   r.EndOpacity.Evaluate(frame),
		Composite:    r.Composite,
	}
}

// intoModel is Repeater::into_model in velato
// src/runtime/model/animated.rs.
func (r *AnimatedRepeater) intoModel() RepeaterValue {
	if r.IsFixed() {
		return RepeaterValue{Fixed: r.Evaluate(0.0)}
	}
	return RepeaterValue{Animated: r}
}

// AnimatedStroke is Stroke in velato src/runtime/model/animated.rs.
type AnimatedStroke struct {
	// Width of the stroke.
	Width Value[float64]
	// Join style.
	Join Join
	// Limit for miter joins, or nil.
	MiterLimit *float64
	// Cap style.
	Cap Cap
	// Port: the lengths of the dashes and the gaps, in their order, and the
	// offset of the pattern (D43).
	Dashes     []Value[float64]
	DashOffset Value[float64]
}

// Evaluate is Stroke::evaluate in velato src/runtime/model/animated.rs.
func (s *AnimatedStroke) Evaluate(frame float64) Stroke {
	stroke := newStroke(s.Width.Evaluate(frame))
	stroke.StartCap, stroke.EndCap = s.Cap, s.Cap
	stroke.Join = s.Join
	if s.MiterLimit != nil {
		stroke.MiterLimit = *s.MiterLimit
	}
	if len(s.Dashes) > 0 {
		stroke.Dash = make([]float64, len(s.Dashes))
		for i := range s.Dashes {
			stroke.Dash[i] = s.Dashes[i].Evaluate(frame)
		}
		stroke.DashOffset = s.DashOffset.Evaluate(frame)
	}
	return stroke
}

// intoModel is Stroke::into_model in velato src/runtime/model/animated.rs.
func (s *AnimatedStroke) intoModel() StrokeValue {
	fixed := s.Width.IsFixed() && s.DashOffset.IsFixed()
	for i := range s.Dashes {
		fixed = fixed && s.Dashes[i].IsFixed()
	}
	if fixed {
		return StrokeValue{Fixed: s.Evaluate(0.0)}
	}
	return StrokeValue{Animated: s}
}

// AnimatedGradient is Gradient in velato src/runtime/model/animated.rs.
type AnimatedGradient struct {
	// True if the gradient is radial.
	IsRadial bool
	// Starting point.
	StartPoint Value[kurbo.Point]
	// Ending point.
	EndPoint Value[kurbo.Point]
	// Stop offsets and color values.
	Stops ColorStopsValue
	// Port: the highlight length, in percent of the radius, and the
	// highlight angle in degrees, of a radial gradient (D43).
	HighlightLength Value[float64]
	HighlightAngle  Value[float64]
}

// IsFixed is Gradient::is_fixed in velato src/runtime/model/animated.rs.
func (g *AnimatedGradient) IsFixed() bool {
	return g.StartPoint.IsFixed() && g.EndPoint.IsFixed() && g.Stops.IsFixed() &&
		g.HighlightLength.IsFixed() && g.HighlightAngle.IsFixed()
}

// Evaluate is Gradient::evaluate in velato src/runtime/model/animated.rs.
func (g *AnimatedGradient) Evaluate(frame float64) Brush {
	start := g.StartPoint.Evaluate(frame)
	end := g.EndPoint.Evaluate(frame)
	stops := g.Stops.Evaluate(frame)
	if g.IsRadial {
		radius := end.ToVec2().Sub(start.ToVec2()).Hypot()
		focal := start
		if !lottiecompat.Velato {
			// Port: velato takes the distance from the start to the end as
			// the radius. okage takes the approximation of the distance that
			// ThorVG computes, and the focal point of the highlight as
			// ThorVG places it (D43).
			w, h := math.Abs(end.X-start.X), math.Abs(end.Y-start.Y)
			if w > h {
				radius = w + 0.375*h
			} else {
				radius = h + 0.375*w
			}
			if progress := g.HighlightLength.Evaluate(frame) * 0.01; progress != 0 {
				angle := end.Sub(start).Atan2() + toRadians(g.HighlightAngle.Evaluate(frame))
				focal = kurbo.NewPoint(start.X+kurbo.Cos(angle)*progress*radius, start.Y+kurbo.Sin(angle)*progress*radius)
			}
		}
		return Brush{Kind: BrushGradient, Gradient: Gradient{Kind: GradientRadial, Start: start, End: start, Radius: float32(radius), Stops: stops, Focal: focal}}
	}
	return Brush{Kind: BrushGradient, Gradient: Gradient{Kind: GradientLinear, Start: start, End: end, Stops: stops}}
}

// AnimatedColorStops is ColorStops in velato
// src/runtime/model/animated.rs. Each value is the raw gradient data of a
// keyframe, with Count color stops.
//
// Port: velato keeps five numbers for each color stop, with the alpha that
// conv_stops gives, and tweens them. okage keeps the raw data, tweens it,
// and then makes the stops with gradientStops (D43).
type AnimatedColorStops struct {
	Frames []Time
	Values [][]float64
	Count  int
	// velato is true when Values holds five numbers for each stop, as
	// velato makes them (internal/lottiecompat).
	velato bool
}

// Evaluate is ColorStops::evaluate in velato src/runtime/model/animated.rs.
func (c *AnimatedColorStops) Evaluate(frame float64) ColorStops {
	if c.velato {
		stops, _ := c.evaluateVelato(frame)
		return stops
	}
	ix, t, easing, hold, ok := framesAndWeight(c.Frames, frame)
	if !ok || ix[0] >= len(c.Values) || ix[1] >= len(c.Values) {
		return nil
	}
	if hold {
		t = 0.0
	}
	v0, v1 := c.Values[ix[0]], c.Values[ix[1]]
	raw := make([]float64, min(len(v0), len(v1)))
	for i := range raw {
		raw[i] = tweenF64(v0[i], v1[i], t, easing)
	}
	return gradientStops(raw, c.Count)
}

// evaluateVelato is ColorStops::evaluate_inner in velato
// src/runtime/model/animated.rs, without the change of D43.
func (c *AnimatedColorStops) evaluateVelato(frame float64) (ColorStops, bool) {
	ix, t, easing, hold, ok := framesAndWeight(c.Frames, frame)
	if !ok || ix[0] >= len(c.Values) || ix[1] >= len(c.Values) {
		return nil, false
	}
	v0, v1 := c.Values[ix[0]], c.Values[ix[1]]
	var stops ColorStops
	for i := range c.Count {
		j := i * 5
		if j+4 >= len(v0) || j+4 >= len(v1) {
			return nil, false
		}
		offset := tweenF64(v0[j], v1[j], t, easing)
		tt := t
		if hold {
			tt = 0.0
		}
		stops = append(stops, ColorStop{
			Offset: float32(offset),
			Color: Color{
				float32(tweenF64(v0[j+1], v1[j+1], tt, easing)),
				float32(tweenF64(v0[j+2], v1[j+2], tt, easing)),
				float32(tweenF64(v0[j+3], v1[j+3], tt, easing)),
				float32(tweenF64(v0[j+4], v1[j+4], tt, easing)),
			},
		})
	}
	return stops, true
}

// AnimatedBrush is Brush in velato src/runtime/model/animated.rs: a solid
// color, or a gradient (D27).
type AnimatedBrush struct {
	IsGradient bool
	Solid      Value[Color]
	Gradient   AnimatedGradient
}

// IsFixed is Brush::is_fixed in velato src/runtime/model/animated.rs.
func (b *AnimatedBrush) IsFixed() bool {
	if b.IsGradient {
		return b.Gradient.IsFixed()
	}
	return b.Solid.IsFixed()
}

// Evaluate is Brush::evaluate in velato src/runtime/model/animated.rs.
func (b *AnimatedBrush) Evaluate(alpha, frame float64) Brush {
	if b.IsGradient {
		return b.Gradient.Evaluate(frame)
	}
	return solidBrush(b.Solid.EvaluateOr(frame, ColorTransparent).MultiplyAlpha(float32(alpha)))
}

// intoModel is Brush::into_model in velato src/runtime/model/animated.rs.
func (b *AnimatedBrush) intoModel() BrushValue {
	if b.IsFixed() {
		return BrushValue{Fixed: b.Evaluate(1.0, 0.0)}
	}
	return BrushValue{Animated: b}
}

// AnimatedTrim is Trim in velato src/runtime/model/animated.rs.
type AnimatedTrim struct {
	Mode TrimMode
	// Start of the visible segment (0.0 to 100.0).
	Start Value[float64]
	// End of the visible segment (0.0 to 100.0).
	End Value[float64]
	// Offset rotation in degrees.
	Offset Value[float64]
}

// Evaluate is Trim::evaluate in velato src/runtime/model/animated.rs.
func (t *AnimatedTrim) Evaluate(frame float64) FixedTrim {
	return FixedTrim{
		Mode:   t.Mode,
		Start:  t.Start.Evaluate(frame),
		End:    t.End.Evaluate(frame),
		Offset: t.Offset.Evaluate(frame),
	}
}

// intoModel is Trim::into_model in velato src/runtime/model/animated.rs.
func (t *AnimatedTrim) intoModel() TrimValue {
	if t.Start.IsFixed() && t.End.IsFixed() && t.Offset.IsFixed() {
		return TrimValue{Fixed: t.Evaluate(0.0)}
	}
	return TrimValue{Animated: t}
}
