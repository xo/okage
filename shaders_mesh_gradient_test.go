package okage_test

// This file tests MeshGradient, which ports no upstream file (D55).

import (
	"fmt"
	"math"
	"testing"

	"github.com/xo/okage"
)

// meshGrid returns a mesh of n by n squares over the square from 0 to size,
// each square split into two triangles, with the diagonals in turn. color
// gives the color of each corner of the grid.
func meshGrid(n int, size float32, color func(i, j int) okage.Color) []okage.MeshTriangle {
	vertex := func(i, j int) okage.MeshVertex {
		return okage.MeshVertex{
			Point: okage.PointFromXY(size*float32(i)/float32(n), size*float32(j)/float32(n)),
			Color: color(i, j),
		}
	}
	var triangles []okage.MeshTriangle
	for j := range n {
		for i := range n {
			a, b, c, d := vertex(i, j), vertex(i+1, j), vertex(i+1, j+1), vertex(i, j+1)
			if (i+j)%2 == 0 {
				triangles = append(triangles, okage.MeshTriangle{a, b, c}, okage.MeshTriangle{a, c, d})
			} else {
				triangles = append(triangles, okage.MeshTriangle{a, b, d}, okage.MeshTriangle{b, c, d})
			}
		}
	}
	return triangles
}

// meshTransform turns and scales a mesh of the size 100 into the middle of a
// pixmap of 200 by 200.
func meshTransform() okage.Transform {
	return okage.TransformFromRotateAt(23, 100, 100).
		PreTranslate(30, 25).
		PreScale(1.37, 1.21)
}

// TestMeshGradientSolid fills a mesh of one color, and a path of its outline
// with the same color. Each fill must give the same bytes, with each blend
// mode, mask, color space and opacity. The colors of a mesh thus do not leak
// at the edges of the triangles, and the antialiased outline has the color of
// the mesh.
func TestMeshGradientSolid(t *testing.T) {
	outline := okage.PathFromRect(must(okage.RectFromLTRB(0, 0, 100, 100)))
	mask := must(okage.NewMask(200, 200))
	mask.FillPath(must(okage.PathFromCircle(90, 110, 70)), okage.FillRuleWinding, true, okage.IdentityTransform())
	for _, color := range []okage.Color{
		okage.ColorFromRGBA8(51, 102, 153, 255),
		okage.ColorFromRGBA8(200, 30, 90, 170),
	} {
		triangles := meshGrid(5, 100, func(int, int) okage.Color { return color })
		for _, aa := range []bool{true, false} {
			for _, blend := range []okage.BlendMode{okage.BlendModeSourceOver, okage.BlendModeMultiply, okage.BlendModeSource} {
				for _, cs := range []okage.ColorSpace{okage.ColorSpaceLinear, okage.ColorSpaceSimpleSRGB} {
					for _, opacity := range []float32{1, 0.6} {
						for _, m := range []*okage.Mask{nil, mask} {
							name := fmt.Sprintf("%v/aa=%t/%v/%v/%v/mask=%t", color, aa, blend, cs, opacity, m != nil)
							if blend == okage.BlendModeSource && cs != okage.ColorSpaceLinear && m == nil {
								// The fill of a SolidColor in this case sets the
								// bytes of the color, and does not convert it to
								// the color space, as tiny-skia does.
								continue
							}
							t.Run(name, func(t *testing.T) {
								want := must(okage.NewPixmap(200, 200))
								want.Fill(okage.ColorFromRGBA8(40, 180, 60, 200))
								got := want.Clone()

								paint := okage.DefaultPaint()
								paint.AntiAlias = aa
								paint.BlendMode = blend
								paint.Colorspace = cs
								paint.ForceHQPipeline = true
								paint.Shader = okage.SolidColor(color).ApplyOpacity(opacity)
								want.FillPath(outline, &paint, okage.FillRuleWinding, meshTransform(), m)

								paint.Shader = must(okage.NewMeshGradient(triangles, okage.IdentityTransform())).ApplyOpacity(opacity)
								got.FillPath(outline, &paint, okage.FillRuleWinding, meshTransform(), m)
								assertPixmapEqual(t, got, want)
							})
						}
					}
				}
			}
		}
	}
}

// TestMeshGradientNoSeams fills a turned mesh of many colors with an
// antialiased path of its outline. The alpha of each pixel must be the
// coverage of the path, so no pixel inside the mesh lets the background
// through.
func TestMeshGradientNoSeams(t *testing.T) {
	triangles := meshGrid(7, 100, func(i, j int) okage.Color {
		return okage.ColorFromRGBA8(uint8(i*36), uint8(j*36), uint8((i*j*17)%256), 255)
	})
	outline := okage.PathFromRect(must(okage.RectFromLTRB(0, 0, 100, 100)))

	paint := okage.DefaultPaint()
	paint.ForceHQPipeline = true
	want := must(okage.NewPixmap(200, 200))
	want.FillPath(outline, &paint, okage.FillRuleWinding, meshTransform(), nil)

	paint.Shader = must(okage.NewMeshGradient(triangles, okage.IdentityTransform()))
	got := must(okage.NewPixmap(200, 200))
	got.FillPath(outline, &paint, okage.FillRuleWinding, meshTransform(), nil)

	gotData, wantData := got.Data(), want.Data()
	for i := 3; i < len(gotData); i += 4 {
		if gotData[i] != wantData[i] {
			p := i / 4
			t.Fatalf("the alpha at %d,%d is %d, want %d", p%200, p/200, gotData[i], wantData[i])
		}
	}
}

// TestMeshGradientInterpolation fills a triangle whose red grows with x and
// whose green grows with y. Each pixel inside must have the color at its
// center, and each pixel that does not overlap the triangle must be
// transparent.
func TestMeshGradientInterpolation(t *testing.T) {
	triangles := []okage.MeshTriangle{{
		{Point: okage.PointFromXY(64, 0), Color: okage.ColorFromRGBA8(255, 0, 0, 255)},
		{Point: okage.PointFromXY(0, 0), Color: okage.ColorFromRGBA8(0, 0, 0, 255)},
		{Point: okage.PointFromXY(0, 64), Color: okage.ColorFromRGBA8(0, 255, 0, 255)},
	}}
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewMeshGradient(triangles, okage.IdentityTransform()))
	pixmap := must(okage.NewPixmap(70, 70))
	pixmap.FillRect(must(okage.RectFromLTRB(0, 0, 70, 70)), &paint, okage.IdentityTransform(), nil)

	for y := range 70 {
		for x := range 70 {
			got := must(pixmap.Pixel(uint32(x), uint32(y)))
			var want [4]uint8
			switch {
			case x+y <= 62 && x < 64 && y < 64:
				r := math.Round((float64(x) + 0.5) / 64 * 255)
				g := math.Round((float64(y) + 0.5) / 64 * 255)
				want = [4]uint8{uint8(r), uint8(g), 0, 255}
			case x+y >= 64:
			default:
				continue
			}
			if c := [4]uint8{got.Red(), got.Green(), got.Blue(), got.Alpha()}; c != want {
				t.Fatalf("the color at %d,%d is %v, want %v", x, y, c, want)
			}
		}
	}
}

// TestMeshGradientOrder fills two triangles that overlap. The later triangle
// covers the earlier one.
func TestMeshGradientOrder(t *testing.T) {
	red := okage.ColorFromRGBA8(255, 0, 0, 255)
	blue := okage.ColorFromRGBA8(0, 0, 255, 255)
	triangle := func(x, y float32, c okage.Color) okage.MeshTriangle {
		return okage.MeshTriangle{
			{Point: okage.PointFromXY(x, y), Color: c},
			{Point: okage.PointFromXY(x+20, y), Color: c},
			{Point: okage.PointFromXY(x, y+20), Color: c},
		}
	}
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewMeshGradient([]okage.MeshTriangle{
		triangle(0, 0, red),
		triangle(5, 5, blue),
	}, okage.IdentityTransform()))
	pixmap := must(okage.NewPixmap(30, 30))
	pixmap.FillRect(must(okage.RectFromLTRB(0, 0, 30, 30)), &paint, okage.IdentityTransform(), nil)

	for _, tc := range []struct {
		x, y uint32
		want okage.Color
	}{
		{1, 1, red},
		{6, 6, blue},
		{8, 7, blue},
		{17, 1, red},
		{23, 6, blue},
	} {
		got := must(pixmap.Pixel(tc.x, tc.y))
		if want := tc.want.Premultiply().ToColorU8(); got != want {
			t.Errorf("the color at %d,%d is %v, want %v", tc.x, tc.y, got, want)
		}
	}
	if got := must(pixmap.Pixel(28, 28)); got.Alpha() != 0 {
		t.Errorf("the color at 28,28 is %v, want transparent", got)
	}
}

// TestMeshGradientTransform makes a mesh with a transform, and the same mesh
// with the transform of the shader. Both must give the same bytes.
func TestMeshGradientTransform(t *testing.T) {
	triangles := meshGrid(3, 100, func(i, j int) okage.Color {
		return okage.ColorFromRGBA8(uint8(i*80), 90, uint8(j*80), uint8(255-i*20))
	})
	outline := okage.PathFromRect(must(okage.RectFromLTRB(0, 0, 100, 100)))

	paint := okage.DefaultPaint()
	paint.Shader = must(okage.NewMeshGradient(triangles, meshTransform()))
	want := must(okage.NewPixmap(200, 200))
	want.FillPath(outline, &paint, okage.FillRuleWinding, meshTransform(), nil)

	paint.Shader = must(okage.NewMeshGradient(triangles, okage.IdentityTransform())).Transform(meshTransform())
	got := must(okage.NewPixmap(200, 200))
	got.FillPath(outline, &paint, okage.FillRuleWinding, meshTransform(), nil)
	assertPixmapEqual(t, got, want)
}

// TestMeshGradientManyTriangles fills a mesh of triangles that each cover
// the whole mesh. The grid gets coarser, and the last triangle gives each
// color.
func TestMeshGradientManyTriangles(t *testing.T) {
	var triangles []okage.MeshTriangle
	for i := range 3000 {
		c := okage.ColorFromRGBA8(uint8(i), uint8(i>>8), 0, 255)
		o := float32(i%7) / 7
		triangles = append(triangles, okage.MeshTriangle{
			{Point: okage.PointFromXY(-o, -o), Color: c},
			{Point: okage.PointFromXY(200+o, 0), Color: c},
			{Point: okage.PointFromXY(0, 200+o), Color: c},
		})
	}
	paint := okage.DefaultPaint()
	paint.Shader = must(okage.NewMeshGradient(triangles, okage.IdentityTransform()))
	pixmap := must(okage.NewPixmap(40, 40))
	pixmap.FillRect(must(okage.RectFromLTRB(0, 0, 40, 40)), &paint, okage.IdentityTransform(), nil)
	last := 2999
	want := okage.ColorFromRGBA8(uint8(last), uint8(last>>8), 0, 255).Premultiply().ToColorU8()
	if got := must(pixmap.Pixel(20, 20)); got != want {
		t.Errorf("the color at 20,20 is %v, want %v", got, want)
	}
}

// TestNewMeshGradientRefuses makes meshes that NewMeshGradient refuses.
func TestNewMeshGradientRefuses(t *testing.T) {
	c := okage.ColorBlack
	nan := float32(math.NaN())
	line := okage.MeshTriangle{
		{Point: okage.PointFromXY(0, 0), Color: c},
		{Point: okage.PointFromXY(5, 5), Color: c},
		{Point: okage.PointFromXY(10, 10), Color: c},
	}
	notFinite := okage.MeshTriangle{
		{Point: okage.PointFromXY(0, 0), Color: c},
		{Point: okage.PointFromXY(nan, 5), Color: c},
		{Point: okage.PointFromXY(10, 0), Color: c},
	}
	good := okage.MeshTriangle{
		{Point: okage.PointFromXY(0, 0), Color: c},
		{Point: okage.PointFromXY(10, 0), Color: c},
		{Point: okage.PointFromXY(0, 10), Color: c},
	}
	for _, tc := range []struct {
		name      string
		triangles []okage.MeshTriangle
		transform okage.Transform
		ok        bool
	}{
		{"empty", nil, okage.IdentityTransform(), false},
		{"line", []okage.MeshTriangle{line}, okage.IdentityTransform(), false},
		{"not finite", []okage.MeshTriangle{notFinite}, okage.IdentityTransform(), false},
		{"transform", []okage.MeshTriangle{good}, okage.TransformFromRow(1, 1, 1, 1, 0, 0), false},
		{"skips", []okage.MeshTriangle{line, notFinite, good}, okage.IdentityTransform(), true},
	} {
		shader, ok := okage.NewMeshGradient(tc.triangles, tc.transform)
		if ok != tc.ok {
			t.Errorf("%s: got %t, want %t", tc.name, ok, tc.ok)
		}
		if ok && shader.IsOpaque() {
			t.Errorf("%s: the shader is opaque", tc.name)
		}
	}
}

// BenchmarkMeshGradient fills a mesh of 800 triangles.
func BenchmarkMeshGradient(b *testing.B) {
	triangles := meshGrid(20, 100, func(i, j int) okage.Color {
		return okage.ColorFromRGBA8(uint8(i*12), uint8(j*12), 128, 255)
	})
	outline := okage.PathFromRect(must(okage.RectFromLTRB(0, 0, 100, 100)))
	paint := okage.DefaultPaint()
	paint.Shader = must(okage.NewMeshGradient(triangles, okage.IdentityTransform()))
	pixmap := must(okage.NewPixmap(200, 200))
	for b.Loop() {
		pixmap.FillPath(outline, &paint, okage.FillRuleWinding, meshTransform(), nil)
	}
}
