package okage

import (
	"math"
	"testing"
)

// This file ports no upstream test. It checks the parameters of the shaders
// (D68).

// as returns the shader as a T, or fails the test.
func as[T Shader](t *testing.T, s Shader) T {
	t.Helper()
	v, ok := s.(T)
	if !ok {
		t.Fatalf("the shader is a %T", s)
	}
	return v
}

// nearPoint reports whether two points are within 1e-4 of each other.
func nearPoint(a, b Point) bool {
	return math.Abs(float64(a.X-b.X)) < 1e-4 && math.Abs(float64(a.Y-b.Y)) < 1e-4
}

func TestGradientParameters(t *testing.T) {
	red := ColorFromRGBA8(255, 0, 0, 255)
	blue := ColorFromRGBA8(0, 0, 255, 255)
	stops := []GradientStop{NewGradientStop(0.25, red), NewGradientStop(0.75, blue)}
	ts := TransformFromRow(2, 0, 0, 3, 4, 5)

	shader := func(s Shader, ok bool) Shader {
		t.Helper()
		if !ok {
			t.Fatal("no shader")
		}
		return s
	}

	lg := as[LinearGradient](t, shader(NewLinearGradient(PointFromXY(10, 20), PointFromXY(110, 70), stops, SpreadModeReflect, ts)))
	p := as[LinearGradient](t, lg.Transform(TransformFromTranslate(1, 2)).ApplyOpacity(0.5)).Parameters()
	if !nearPoint(p.Start, PointFromXY(10, 20)) || !nearPoint(p.End, PointFromXY(110, 70)) {
		t.Errorf("the points are %v and %v", p.Start, p.End)
	}
	if p.Mode != SpreadModeReflect || p.Transform != ts.PostConcat(TransformFromTranslate(1, 2)) {
		t.Errorf("the mode is %v and the transform %v", p.Mode, p.Transform)
	}
	// The stops get a stop at 0 and at 1, and the opacity.
	if len(p.Stops) != 4 || p.Stops[0].Position() != 0 || p.Stops[3].Position() != 1 ||
		p.Stops[1].Position() != 0.25 || p.Stops[1].Color().Alpha() != 0.5 || p.Stops[2].Color().Blue() != 1 {
		t.Errorf("the stops are %v", p.Stops)
	}

	for _, c := range []struct {
		name                   string
		shader                 Shader
		start, end             Point
		startRadius, endRadius float32
		extend, xs, xe         bool
	}{
		{"focal", shader(NewRadialGradient(PointFromXY(10, 20), 0, PointFromXY(30, 20), 50, stops, SpreadModePad, ts)),
			PointFromXY(10, 20), PointFromXY(30, 20), 0, 50, false, false, false},
		{"strip", shader(NewRadialGradient(PointFromXY(10, 20), 5, PointFromXY(30, 20), 5, stops, SpreadModePad, ts)),
			PointFromXY(10, 20), PointFromXY(30, 20), 5, 5, false, false, false},
		{"concentric", shader(NewRadialGradient(PointFromXY(10, 20), 5, PointFromXY(10, 20), 40, stops, SpreadModeDecal, ts)),
			PointFromXY(10, 20), PointFromXY(10, 20), 5, 40, false, false, false},
		{"radial", shader(NewRadialGradient(PointFromXY(10, 20), 0, PointFromXY(10, 20), 40, stops, SpreadModeRepeat, ts)),
			PointFromXY(10, 20), PointFromXY(10, 20), 0, 40, false, false, false},
		{"extend", shader(NewRadialGradientExtend(PointFromXY(10, 20), 3, PointFromXY(30, 40), 9, stops, false, true, ts)),
			PointFromXY(10, 20), PointFromXY(30, 40), 3, 9, true, false, true},
	} {
		p := as[RadialGradient](t, c.shader).Parameters()
		if p.Start != c.start || p.End != c.end || p.StartRadius != c.startRadius || p.EndRadius != c.endRadius ||
			p.Extend != c.extend || p.ExtendStart != c.xs || p.ExtendEnd != c.xe || p.Transform != ts {
			t.Errorf("%s: the parameters are %+v", c.name, p)
		}
	}

	sg := as[SweepGradient](t, shader(NewSweepGradient(PointFromXY(10, 20), 45, 270, stops, SpreadModeRepeat, ts)))
	p = sg.Parameters()
	if p.Start != PointFromXY(10, 20) || p.StartAngle != 45 || p.EndAngle != 270 || p.Mode != SpreadModeRepeat {
		t.Errorf("the sweep has %+v", p)
	}
}

func TestMeshParameters(t *testing.T) {
	v := func(x, y float32, c Color) MeshVertex { return MeshVertex{Point: PointFromXY(x, y), Color: c} }
	red := ColorFromRGBA8(255, 0, 0, 255)
	tris := []MeshTriangle{
		{v(0, 0, red), v(10, 0, ColorBlack), v(0, 10, ColorWhite)},
		// A triangle with no area, which NewMeshGradient skips.
		{v(0, 0, red), v(10, 0, red), v(20, 0, red)},
		{v(5, 5, ColorWhite), v(15, 5, red), v(5, 15, ColorBlack)},
	}
	s, ok := NewMeshGradient(tris, TransformFromTranslate(3, 4))
	if !ok {
		t.Fatal("no mesh")
	}
	p := as[MeshGradient](t, s.ApplyOpacity(0.25)).Parameters()
	if len(p.Triangles) != 2 || p.Triangles[0] != tris[0] || p.Triangles[1] != tris[2] {
		t.Errorf("the triangles are %v", p.Triangles)
	}
	if p.Opacity != 0.25 || p.Transform != TransformFromTranslate(3, 4) {
		t.Errorf("the opacity is %v and the transform %v", p.Opacity, p.Transform)
	}
	if p := (MeshGradient{}).Parameters(); p.Triangles != nil {
		t.Errorf("the zero mesh has %v", p.Triangles)
	}
}

func TestPatternParameters(t *testing.T) {
	pm, _ := NewPixmap(3, 2)
	ts := TransformFromScale(2, 2)
	p := as[Pattern](t, NewPattern(pm.AsRef(), SpreadModeReflect, FilterQualityBilinear, 0.5, ts).ApplyOpacity(0.5)).Parameters()
	if p.Pixmap.Width() != 3 || p.Pixmap.Height() != 2 || p.Mode != SpreadModeReflect ||
		p.Quality != FilterQualityBilinear || p.Opacity != 0.25 || p.Transform != ts {
		t.Errorf("the pattern has %+v", p)
	}
}
