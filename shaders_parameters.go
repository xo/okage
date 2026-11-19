package okage

import "slices"

// This file ports no upstream file. It gives the parameters of each shader,
// so that a writer of another format, such as the package pdf, can write the
// shader in that format (D68). tiny-skia keeps them private.

// Position gives the position of the stop, from 0 to 1.
func (s GradientStop) Position() float32 {
	return s.position.Get()
}

// Color gives the color of the stop.
func (s GradientStop) Color() Color {
	return s.color
}

// GradientParameters are the parameters of a linear, radial or sweep
// gradient, which the method Parameters of each gradient gives (D68).
type GradientParameters struct {
	// Start is the start point of a linear gradient, the center of the start
	// circle of a radial gradient, or the center of a sweep gradient.
	Start Point
	// End is the end point of a linear gradient, or the center of the end
	// circle of a radial gradient.
	End Point
	// StartRadius and EndRadius are the radii of the circles of a radial
	// gradient.
	StartRadius float32
	EndRadius   float32
	// StartAngle and EndAngle are the angles of a sweep gradient, in
	// degrees.
	StartAngle float32
	EndAngle   float32
	// Stops are the stops as the shader draws them. The first stop is at 0,
	// the last stop is at 1, and the positions do not go down. An opacity
	// that ApplyOpacity gave is in the alpha of each color.
	Stops []GradientStop
	// Mode is the spread mode. It is SpreadModePad for a gradient of
	// NewRadialGradientExtend.
	Mode SpreadMode
	// Extend is true for a gradient of NewRadialGradientExtend. ExtendStart
	// and ExtendEnd are then its flags (D64).
	Extend      bool
	ExtendStart bool
	ExtendEnd   bool
	// Transform maps the space of the gradient to the space of the user.
	Transform Transform
}

// Parameters gives the parameters of the linear gradient. The points come
// from the inverse of the transform that tiny-skia keeps, so they can differ
// from the points of NewLinearGradient in the last bits.
func (g LinearGradient) Parameters() GradientParameters {
	p := g.base.parameters()
	if inv, ok := g.base.pointsToUnit.Invert(); ok {
		p.Start = PointFromXY(0, 0)
		p.End = PointFromXY(1, 0)
		inv.MapPoint(&p.Start)
		inv.MapPoint(&p.End)
	}
	return p
}

// Parameters gives the parameters of the radial gradient: the circles that
// its constructor took. For a degenerate gradient of NewRadialGradient that
// draws a ring, the start circle has the radius 0 and the stops give the
// ring.
func (g RadialGradient) Parameters() GradientParameters {
	p := g.base.parameters()
	c := g.gradientType.circles
	p.Start, p.StartRadius = c.start, c.startRadius
	p.End, p.EndRadius = c.end, c.endRadius
	if g.gradientType.kind == gradientTypeExtend {
		p.Extend = true
		p.ExtendStart = g.gradientType.extend.extendStart
		p.ExtendEnd = g.gradientType.extend.extendEnd
	}
	return p
}

// Parameters gives the parameters of the sweep gradient. The angles come
// from the values that tiny-skia keeps, so they can differ from the angles
// of NewSweepGradient in the last bits.
func (g SweepGradient) Parameters() GradientParameters {
	p := g.base.parameters()
	// The transform to the unit space of a sweep gradient is a translation.
	p.Start = PointFromXY(-g.base.pointsToUnit.TX, -g.base.pointsToUnit.TY)
	p.StartAngle = float32(g.t0 * 360)
	p.EndAngle = float32(g.t1 * 360)
	return p
}

// parameters gives the parameters that each gradient has.
func (g *gradient) parameters() GradientParameters {
	return GradientParameters{
		Stops:     slices.Clone(g.stops),
		Mode:      g.tileMode,
		Transform: g.transform,
	}
}

// radialCircles holds the circles of a radial gradient, as its constructor
// took them (D68).
type radialCircles struct {
	start, end             Point
	startRadius, endRadius float32
}

// MeshParameters are the parameters of a MeshGradient, which
// MeshGradient.Parameters gives (D68).
type MeshParameters struct {
	// Triangles are the triangles that NewMeshGradient kept, in their
	// order. The colors are the colors of the caller, without Opacity.
	Triangles []MeshTriangle
	// Opacity is the opacity that ApplyOpacity gave, from 0 to 1.
	Opacity float32
	// Transform maps the space of the triangles to the space of the user.
	Transform Transform
}

// Parameters gives the parameters of the mesh. It copies the triangles.
func (m MeshGradient) Parameters() MeshParameters {
	p := MeshParameters{Opacity: m.opacity, Transform: m.transform}
	if m.mesh == nil {
		return p
	}
	p.Triangles = make([]MeshTriangle, len(m.mesh.points)/3)
	for i := range p.Triangles {
		for k := range 3 {
			p.Triangles[i][k] = MeshVertex{Point: m.mesh.points[3*i+k], Color: m.mesh.vertexColors[3*i+k]}
		}
	}
	return p
}

// PatternParameters are the parameters of a Pattern, which
// Pattern.Parameters gives (D68).
type PatternParameters struct {
	// Pixmap is the pixmap of the pattern. It shares the pixels of the
	// pattern.
	Pixmap PixmapRef
	// Mode is the spread mode.
	Mode SpreadMode
	// Quality is the filter quality.
	Quality FilterQuality
	// Opacity is the opacity, from 0 to 1.
	Opacity float32
	// Transform maps the space of the pixmap to the space of the user.
	Transform Transform
}

// Parameters gives the parameters of the pattern.
func (p Pattern) Parameters() PatternParameters {
	return PatternParameters{
		Pixmap:    p.pixmap,
		Mode:      p.spreadMode,
		Quality:   p.quality,
		Opacity:   p.opacity.Get(),
		Transform: p.transform,
	}
}
