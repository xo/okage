// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"errors"
	"image"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// The tests of this file check the objects that the additions for ivy write
// (D68). The test module renders each addition with MuPDF and Poppler, and
// compares the pixels with okage, in test/krilla/ivy_test.go.

// ivyFile draws onto one page of 200 by 200 units, and returns the file
// without compression, and the error of Document.Finish.
func ivyFile(t *testing.T, draw func(s *Surface)) (string, error) {
	t.Helper()
	d := NewDocument(WithCompression(false), WithPretty(true))
	ps, ok := PageSettingsFromWH(200, 200)
	if !ok {
		t.Fatal("no page settings")
	}
	page := d.StartPage(ps)
	s := page.Surface()
	draw(s)
	s.Finish()
	page.Finish()
	var buf bytes.Buffer
	err := d.Finish(&buf)
	return buf.String(), err
}

// mustIvyFile is ivyFile for a file without an error.
func mustIvyFile(t *testing.T, draw func(s *Surface)) string {
	t.Helper()
	file, err := ivyFile(t, draw)
	if err != nil {
		t.Fatalf("finishing the document: %v", err)
	}
	return file
}

// wantMatch fails when the file does not match each pattern.
func wantMatch(t *testing.T, file string, patterns ...string) {
	t.Helper()
	for _, p := range patterns {
		if !regexp.MustCompile(p).MatchString(file) {
			t.Errorf("the file does not match %q", p)
		}
	}
}

// wantNoMatch fails when the file matches a pattern.
func wantNoMatch(t *testing.T, file string, patterns ...string) {
	t.Helper()
	for _, p := range patterns {
		if regexp.MustCompile(p).MatchString(file) {
			t.Errorf("the file matches %q", p)
		}
	}
}

func TestPushGroup(t *testing.T) {
	cases := []struct {
		name    string
		group   Group
		want    []string
		notWant []string
	}{
		{
			"knockout",
			Group{Opacity: NormalizedF32One, Isolated: true, Knockout: true},
			[]string{`/S /Transparency`, `/I true`, `/K true`},
			[]string{`/ca `, `/BM `},
		},
		{
			"not isolated",
			Group{Opacity: NormalizedF32One, Knockout: true},
			[]string{`/S /Transparency`, `/K true`},
			[]string{`/I true`},
		},
		{
			"state",
			Group{Opacity: NormalizedF32(0.5), BlendMode: BlendModeMultiply},
			[]string{`/S /Transparency`, `/ca 0.5`, `/CA 0.5`, `/BM /Multiply`},
			[]string{`/K true`, `/I true`},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			file := mustIvyFile(t, func(s *Surface) {
				s.PushGroup(c.group)
				s.SetFill(redFill(t, 1))
				s.DrawPath(rectToPath(t, 10, 10, 90, 90))
				s.Pop()
			})
			wantMatch(t, file, c.want...)
			wantNoMatch(t, file, c.notWant...)
		})
	}
	t.Run("mask", func(t *testing.T) {
		file := mustIvyFile(t, func(s *Surface) {
			b := s.StreamBuilder()
			ms := b.Surface()
			ms.SetFill(greenFill(t, 1))
			ms.DrawPath(rectToPath(t, 0, 0, 50, 50))
			ms.Finish()
			s.PushGroup(Group{Opacity: NormalizedF32One, Mask: NewMask(b.Finish(), MaskTypeAlpha)})
			s.SetFill(redFill(t, 1))
			s.DrawPath(rectToPath(t, 10, 10, 90, 90))
			s.Pop()
		})
		wantMatch(t, file, `/SMask \d+ 0 R`, `/S /Alpha`)
	})
	t.Run("the hash of the flag", func(t *testing.T) {
		// A knockout group and a group with the same content are two
		// XObjects.
		file := mustIvyFile(t, func(s *Surface) {
			for _, k := range []bool{false, true} {
				s.PushGroup(Group{Opacity: NormalizedF32One, Knockout: k})
				s.SetFill(redFill(t, 1))
				s.DrawPath(rectToPath(t, 10, 10, 90, 90))
				s.Pop()
			}
		})
		if n := strings.Count(file, "/Subtype /Form"); n != 2 {
			t.Errorf("the file has %d form XObjects, want 2", n)
		}
	})
}

func TestPushStrokeAdjustment(t *testing.T) {
	file := mustIvyFile(t, func(s *Surface) {
		s.SetFill(nil)
		s.SetStroke(redStroke(t, 1, 0.5))
		s.PushStrokeAdjustment(true)
		s.DrawPath(rectToPath(t, 10, 10, 90, 90))
		s.Pop()
		s.DrawPath(rectToPath(t, 20, 20, 80, 80))
	})
	wantMatch(t, file, `/SA true`)
	// The second path, after the pop, draws without the graphics state.
	if n := strings.Count(file, " gs\n"); n != 1 {
		t.Errorf("the content sets %d graphics states, want 1", n)
	}
}

func TestMaskBackdropTransfer(t *testing.T) {
	var table [256]uint8
	for i := range table {
		table[i] = uint8(255 - i)
	}
	draw := func(mask func(*Mask) *Mask) string {
		return mustIvyFile(t, func(s *Surface) {
			b := s.StreamBuilder()
			ms := b.Surface()
			ms.SetFill(greenFill(t, 1))
			ms.DrawPath(rectToPath(t, 0, 0, 50, 50))
			ms.Finish()
			s.PushMask(mask(NewMask(b.Finish(), MaskTypeLuminosity)))
			s.SetFill(redFill(t, 1))
			s.DrawPath(rectToPath(t, 10, 10, 190, 190))
			s.Pop()
		})
	}
	file := draw(func(m *Mask) *Mask { return m.WithBackdrop(NewColorRGB(255, 0, 0)).WithTransfer(&table) })
	wantMatch(t, file, `/BC \[1 0 0\]`, `/TR \d+ 0 R`, `/FunctionType 0`, `/Size \[256\]`,
		`/BitsPerSample 8`, `/Domain \[0 1\]`, `/Range \[0 1\]`)
	// The group of the mask covers the content that it masks.
	wantMatch(t, file, `/BBox \[10 10 190 190\]`)
	file = draw(func(m *Mask) *Mask { return m.WithTransfer(&table).WithTransfer(nil) })
	wantNoMatch(t, file, `/BC `, `/TR `, `/FunctionType 0`)
	// An alpha mask has no backdrop.
	file = mustIvyFile(t, func(s *Surface) {
		b := s.StreamBuilder()
		ms := b.Surface()
		ms.DrawPath(rectToPath(t, 0, 0, 50, 50))
		ms.Finish()
		s.PushMask(NewMask(b.Finish(), MaskTypeAlpha).WithBackdrop(NewColorRGB(255, 0, 0)))
		s.DrawPath(rectToPath(t, 10, 10, 90, 90))
		s.Pop()
	})
	wantNoMatch(t, file, `/BC `)
}

func TestExtendMode(t *testing.T) {
	stops := []Stop{
		{Offset: NormalizedF32Zero, Color: NewColorRGB(255, 0, 0), Opacity: NormalizedF32One},
		{Offset: NormalizedF32One, Color: NewColorRGB(0, 0, 255), Opacity: NormalizedF32One},
	}
	for _, c := range []struct {
		extend ExtendMode
		want   string
	}{
		{ExtendBoth, `/Extend \[true true\]`},
		{ExtendStart, `/Extend \[true false\]`},
		{ExtendEnd, `/Extend \[false true\]`},
		{ExtendNone, `/Extend \[false false\]`},
	} {
		file := mustIvyFile(t, func(s *Surface) {
			s.SetFill(&Fill{Paint: LinearGradient{
				X1: 10, Y1: 10, X2: 90, Y2: 10, Transform: path.IdentityTransform(), Stops: stops, Extend: c.extend,
			}, Opacity: NormalizedF32One})
			s.DrawPath(rectToPath(t, 0, 0, 100, 100))
			s.SetFill(&Fill{Paint: RadialGradient{
				Fx: 50, Fy: 50, Fr: 5, Cx: 60, Cy: 50, Cr: 40, Transform: path.IdentityTransform(), Stops: stops, Extend: c.extend,
			}, Opacity: NormalizedF32One})
			s.DrawPath(rectToPath(t, 0, 100, 100, 200))
		})
		if n := len(regexp.MustCompile(c.want).FindAllString(file, -1)); n != 2 {
			t.Errorf("%d: the file has %d matches of %q, want 2", c.extend, n, c.want)
		}
	}
}

// meshTriangles returns two triangles of a square, with an opacity at one
// corner.
func meshTriangles(opacity NormalizedF32) []MeshTriangle {
	v := func(x, y float32, c Color, o NormalizedF32) MeshVertex {
		return MeshVertex{Point: path.PointFromXY(x, y), Color: c, Opacity: o}
	}
	red, green, blue := NewColorRGB(255, 0, 0), NewColorRGB(0, 255, 0), NewColorRGB(0, 0, 255)
	return []MeshTriangle{
		{v(10, 10, red, opacity), v(90, 10, green, NormalizedF32One), v(10, 90, blue, NormalizedF32One)},
		{v(90, 10, green, NormalizedF32One), v(90, 90, red, NormalizedF32One), v(10, 90, blue, NormalizedF32One)},
	}
}

func TestMeshGradient(t *testing.T) {
	draw := func(m MeshGradient, stroke bool) (string, error) {
		return ivyFile(t, func(s *Surface) {
			if stroke {
				st := DefaultStroke()
				st.Paint = m
				s.SetFill(nil)
				s.SetStroke(&st)
			} else {
				s.SetFill(&Fill{Paint: m, Opacity: NormalizedF32One})
			}
			s.DrawPath(rectToPath(t, 10, 10, 90, 90))
		})
	}
	file, err := draw(MeshGradient{Triangles: meshTriangles(NormalizedF32One), Transform: path.IdentityTransform()}, false)
	if err != nil {
		t.Fatal(err)
	}
	wantMatch(t, file, `/ShadingType 4`, `/BitsPerCoordinate 32`, `/BitsPerComponent 16`, `/BitsPerFlag 8`,
		`/Decode \[10 90 10 90 0 1 0 1 0 1\]`, `/ColorSpace /DeviceRGB`, `/PatternType 2`)
	wantNoMatch(t, file, `/SMask`)
	if n := strings.Count(file, "/ShadingType 4"); n != 1 {
		t.Errorf("the file has %d shadings, want 1", n)
	}

	// A fill with an opacity draws in a group with a soft mask of the
	// opacities, in gray. The group of the mask is a gray transparency
	// group, as krilla writes the mask of a gradient since 63233fb4 (D89).
	file, err = draw(MeshGradient{Triangles: meshTriangles(NormalizedF32(0.5)), Transform: path.IdentityTransform()}, false)
	if err != nil {
		t.Fatal(err)
	}
	wantMatch(t, file, `/SMask \d+ 0 R`, `/S /Luminosity`, `/ColorSpace /DeviceGray`, `/Decode \[10 90 10 90 0 1\]`,
		`/S /Transparency\s+/CS /DeviceGray`)
	if n := strings.Count(file, "/ShadingType 4"); n != 2 {
		t.Errorf("the file has %d shadings, want 2", n)
	}
	// A stroke with an opacity sets the soft mask in its graphics state.
	file, err = draw(MeshGradient{Triangles: meshTriangles(NormalizedF32(0.5)), Transform: path.IdentityTransform()}, true)
	if err != nil {
		t.Fatal(err)
	}
	wantMatch(t, file, `/SMask \d+ 0 R`, `/ColorSpace /DeviceGray`, `/S /Transparency\s+/CS /DeviceGray`)

	// The faults of a mesh.
	bad := meshTriangles(NormalizedF32One)
	bad[1][2].Color = nil
	notFinite := meshTriangles(NormalizedF32One)
	notFinite[0][0].Point = path.PointFromXY(float32(posInf()), 0)
	mixed := meshTriangles(NormalizedF32One)
	mixed[1][0].Color = NewColorCMYK(0, 0, 0, 255)
	for _, c := range []struct {
		name string
		mesh []MeshTriangle
		want error
	}{
		{"empty", nil, okage.ErrPDFInvalidGradient},
		{"no color", bad, okage.ErrPDFInvalidGradient},
		{"not finite", notFinite, okage.ErrPDFInvalidGradient},
		{"mixed", mixed, okage.ErrPDFMixedColorSpaces},
	} {
		_, err := draw(MeshGradient{Triangles: c.mesh, Transform: path.IdentityTransform()}, false)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: the error is %v, want %v", c.name, err, c.want)
		}
	}
}

// posInf returns the positive infinity.
func posInf() float64 {
	var zero float64
	return 1 / zero
}

func TestMeshCoordinate(t *testing.T) {
	lo, hi := meshRange(10, 90)
	if lo != 10 || hi != 90 {
		t.Errorf("the range of 10 to 90 is %v to %v", lo, hi)
	}
	if lo, hi := meshRange(5, 5); lo != 5 || hi != 6 {
		t.Errorf("the range of 5 to 5 is %v to %v", lo, hi)
	}
	if lo, hi := meshRange(0.1, 0.3); !(float64(float32(lo)) == lo && lo <= 0.1 && hi >= 0.3) {
		t.Errorf("the range of 0.1 to 0.3 is %v to %v", lo, hi)
	}
	for _, c := range []struct {
		v    float64
		want uint32
	}{{10, 0}, {90, 0xFFFFFFFF}, {50, 0x80000000}, {-1, 0}, {100, 0xFFFFFFFF}} {
		if got := meshCoordinate(c.v, 10, 90); got != c.want {
			t.Errorf("meshCoordinate(%v) = %#x, want %#x", c.v, got, c.want)
		}
	}
	for _, c := range []struct {
		v    float32
		want uint16
	}{{0, 0}, {1, 0xFFFF}, {0.5, 0x8000}, {-1, 0}, {2, 0xFFFF}} {
		if got := meshComponent(c.v); got != c.want {
			t.Errorf("meshComponent(%v) = %#x, want %#x", c.v, got, c.want)
		}
	}
}

func TestFillStencil(t *testing.T) {
	m := image.NewAlpha(image.Rect(0, 0, 4, 3))
	for i := range m.Pix {
		m.Pix[i] = uint8(20 * i)
	}
	size, _ := path.SizeFromWH(40, 30)
	file := mustIvyFile(t, func(s *Surface) {
		s.SetFill(redFill(t, 1))
		s.FillStencil(m, size, true)
		s.FillStencil(image.NewAlpha(image.Rect(0, 0, 0, 0)), size, false)
	})
	wantMatch(t, file, `/S /Luminosity`, `/Subtype /Image`, `/Width 4`, `/Height 3`,
		`/ColorSpace /DeviceGray`, `/Interpolate true`, `1 0 0 rg`)
	if n := strings.Count(file, "/Subtype /Image"); n != 1 {
		t.Errorf("the file has %d images, want 1", n)
	}
	_, err := ivyFile(t, func(s *Surface) { s.FillStencil(nil, size, false) })
	if !errors.Is(err, okage.ErrPDFImage) {
		t.Errorf("the error of a nil stencil is %v", err)
	}
}

func TestDrawRaster(t *testing.T) {
	r, _ := path.RectFromXYWH(10, 20, 30, 40)
	var got okage.Transform
	var w, h uint32
	file := mustIvyFile(t, func(s *Surface) {
		s.PushTransform(path.TransformFromScale(2, 2))
		ok := s.DrawRaster(r, 1.5, func(pm *okage.Pixmap, ts okage.Transform) {
			got, w, h = ts, pm.Width(), pm.Height()
			pm.Fill(okage.ColorFromRGBA8(255, 0, 0, 128))
		})
		s.Pop()
		if !ok {
			t.Error("DrawRaster drew nothing")
		}
	})
	// The page scales by 2, and the scale is 1.5.
	if w != 90 || h != 120 {
		t.Errorf("the pixmap is %dx%d, want 90x120", w, h)
	}
	if want := path.TransformFromRow(3, 0, 0, 3, -30, -60); got != want {
		t.Errorf("the transform is %v, want %v", got, want)
	}
	wantMatch(t, file, `/Width 90`, `/Height 120`, `/SMask \d+ 0 R`)

	// The pixmap has at most 5000 pixels on a side.
	_ = mustIvyFile(t, func(s *Surface) {
		s.DrawRaster(r, 1000, func(pm *okage.Pixmap, _ okage.Transform) {
			w, h = pm.Width(), pm.Height()
		})
	})
	if w != 3750 || h != 5000 {
		t.Errorf("the large pixmap is %dx%d, want 3750x5000", w, h)
	}
	_ = mustIvyFile(t, func(s *Surface) {
		if s.DrawRaster(r, 0, func(*okage.Pixmap, okage.Transform) { t.Error("draw for a scale of 0") }) {
			t.Error("DrawRaster drew for a scale of 0")
		}
	})

	// A scale that is NaN gives the largest pixmap, as f32::min of Rust
	// does in krilla-svg.
	_ = mustIvyFile(t, func(s *Surface) {
		s.DrawRaster(r, float32(math.NaN()), func(pm *okage.Pixmap, _ okage.Transform) {
			w, h = pm.Width(), pm.Height()
		})
	})
	if w != 3750 || h != 5000 {
		t.Errorf("the pixmap of a NaN scale is %dx%d, want 3750x5000", w, h)
	}

	// WithRasterArea gives the area on the page in place of the current
	// transform.
	area, _ := path.RectFromXYWH(0, 0, 120, 40)
	_ = mustIvyFile(t, func(s *Surface) {
		s.PushTransform(path.TransformFromScale(2, 2))
		s.DrawRaster(r, 1, func(pm *okage.Pixmap, _ okage.Transform) {
			w, h = pm.Width(), pm.Height()
		}, WithRasterArea(area))
		s.Pop()
	})
	// The area is 4 times as wide as r, and as high.
	if w != 120 || h != 160 {
		t.Errorf("the pixmap of the area is %dx%d, want 120x160", w, h)
	}
}

func TestPaintFromShader(t *testing.T) {
	red := okage.ColorFromRGBA8(255, 0, 0, 255)
	blue := okage.ColorFromRGBA8(0, 0, 255, 128)
	stops := []okage.GradientStop{okage.NewGradientStop(0.2, red), okage.NewGradientStop(1, blue)}
	pt := okage.PointFromXY
	id := okage.IdentityTransform()
	shader := func(s okage.Shader, ok bool) okage.Shader {
		t.Helper()
		if !ok {
			t.Fatal("no shader")
		}
		return s
	}
	pm, _ := okage.NewPixmap(2, 2)
	v := func(x, y float32, c okage.Color) okage.MeshVertex { return okage.MeshVertex{Point: pt(x, y), Color: c} }
	mesh := shader(okage.NewMeshGradient([]okage.MeshTriangle{{v(0, 0, red), v(10, 0, blue), v(0, 10, red)}}, id))

	_ = mustIvyFile(t, func(s *Surface) {
		check := func(sh okage.Shader, want Paint, wantOpacity NormalizedF32) {
			t.Helper()
			got, opacity, ok := s.PaintFromShader(sh)
			if !ok {
				t.Errorf("%T does not convert", sh)
				return
			}
			if sipHash(paintKey{got}) != sipHash(paintKey{want}) || opacity != wantOpacity {
				t.Errorf("%T converts to %#v, %v, want %#v, %v", sh, got, opacity, want, wantOpacity)
			}
		}
		wantStops := []Stop{
			{Offset: NormalizedF32Zero, Color: NewColorRGB(255, 0, 0), Opacity: NormalizedF32One},
			{Offset: NormalizedF32(0.2), Color: NewColorRGB(255, 0, 0), Opacity: NormalizedF32One},
			{Offset: NormalizedF32One, Color: NewColorRGB(0, 0, 255), Opacity: NormalizedF32(float32(128) / 255)},
		}
		check(okage.SolidColor(blue), NewColorRGB(0, 0, 255), NormalizedF32(float32(128)/255))
		check(shader(okage.NewLinearGradient(pt(0, 0), pt(10, 0), stops, okage.SpreadModeDecal, id)),
			LinearGradient{X2: 10, Transform: id, Stops: wantStops, Extend: ExtendNone}, NormalizedF32One)
		check(shader(okage.NewLinearGradient(pt(0, 0), pt(10, 0), stops, okage.SpreadModeReflect, id)),
			LinearGradient{X2: 10, Transform: id, Stops: wantStops, SpreadMethod: SpreadMethodReflect}, NormalizedF32One)
		check(shader(okage.NewRadialGradient(pt(1, 2), 3, pt(4, 5), 6, stops, okage.SpreadModePad, id)),
			RadialGradient{Fx: 1, Fy: 2, Fr: 3, Cx: 4, Cy: 5, Cr: 6, Transform: id, Stops: wantStops}, NormalizedF32One)
		check(shader(okage.NewRadialGradientExtend(pt(1, 2), 3, pt(4, 5), 6, stops, false, true, id)),
			RadialGradient{Fx: 1, Fy: 2, Fr: 3, Cx: 4, Cy: 5, Cr: 6, Transform: id, Stops: wantStops, Extend: ExtendEnd}, NormalizedF32One)
		check(shader(okage.NewSweepGradient(pt(1, 2), 0, 90, stops, okage.SpreadModeRepeat, id)),
			SweepGradient{Cx: 1, Cy: 2, EndAngle: 90, Transform: id, Stops: wantStops, SpreadMethod: SpreadMethodRepeat}, NormalizedF32One)
		check(mesh.ApplyOpacity(0.5), MeshGradient{Transform: id, Triangles: []MeshTriangle{{
			{Point: pt(0, 0), Color: NewColorRGB(255, 0, 0), Opacity: NormalizedF32(0.5)},
			{Point: pt(10, 0), Color: NewColorRGB(0, 0, 255), Opacity: NormalizedF32(float32(64) / 255)},
			{Point: pt(0, 10), Color: NewColorRGB(255, 0, 0), Opacity: NormalizedF32(0.5)},
		}}}, NormalizedF32One)

		for _, sh := range []okage.Shader{
			shader(okage.NewRadialGradient(pt(1, 2), 3, pt(4, 5), 6, stops, okage.SpreadModeRepeat, id)),
			shader(okage.NewSweepGradient(pt(1, 2), 0, 90, stops, okage.SpreadModeDecal, id)),
			okage.NewPattern(pm.AsRef(), okage.SpreadModePad, okage.FilterQualityNearest, 1, id),
			okage.NewPattern(pm.AsRef(), okage.SpreadModeDecal, okage.FilterQualityNearest, 1, id),
			nil,
		} {
			if _, _, ok := s.PaintFromShader(sh); ok {
				t.Errorf("%T converts", sh)
			}
		}
		for _, mode := range []okage.SpreadMode{okage.SpreadModeRepeat, okage.SpreadModeReflect} {
			p, opacity, ok := s.PaintFromShader(okage.NewPattern(pm.AsRef(), mode, okage.FilterQualityBicubic, 0.5, id))
			pat, isPattern := p.(Pattern)
			if !ok || !isPattern || opacity != NormalizedF32(0.5) {
				t.Errorf("%v: the pattern converts to %#v, %v, %v", mode, p, opacity, ok)
				continue
			}
			want := float32(2)
			if mode == okage.SpreadModeReflect {
				want = 4
			}
			if pat.Width != want || pat.Height != want {
				t.Errorf("%v: the tile is %vx%v, want %vx%v", mode, pat.Width, pat.Height, want, want)
			}
		}
	})
}

// paintKey hashes a paint for a comparison.
type paintKey struct {
	p Paint
}

func (paintKey) typeID() string { return "paintKey" }

func (k paintKey) hash(h *hasher) { k.p.hashPaint(h) }

func TestFillStencilImageMask(t *testing.T) {
	m := image.NewAlpha(image.Rect(0, 0, 10, 2))
	for i := range m.Pix {
		if i%3 == 0 {
			m.Pix[i] = 0xff
		}
	}
	size, _ := path.SizeFromWH(40, 30)
	file := mustIvyFile(t, func(s *Surface) {
		s.SetFill(redFill(t, 0.5))
		s.FillStencil(m, size, false)
	})
	wantMatch(t, file, `/Subtype /Image`, `/ImageMask true`, `/BitsPerComponent 1`, `/Width 10`, `/Height 2`,
		`/ca 0.5`, `1 0 0 rg`, `/x\d+ Do`)
	wantNoMatch(t, file, `/S /Luminosity`, `/ColorSpace /DeviceGray`, `/Interpolate`)

	// A pixel between 0 and 255, or a fill that is not a color, takes the
	// soft mask.
	m.Pix[1] = 0x80
	file = mustIvyFile(t, func(s *Surface) { s.FillStencil(m, size, true) })
	wantMatch(t, file, `/S /Luminosity`)
	wantNoMatch(t, file, `/ImageMask`)
	m.Pix[1] = 0
	stops := []Stop{
		{Offset: NormalizedF32Zero, Color: NewColorRGB(255, 0, 0), Opacity: NormalizedF32One},
		{Offset: NormalizedF32One, Color: NewColorRGB(0, 0, 255), Opacity: NormalizedF32One},
	}
	file = mustIvyFile(t, func(s *Surface) {
		s.SetFill(&Fill{Paint: LinearGradient{X2: 40, Transform: path.IdentityTransform(), Stops: stops}, Opacity: NormalizedF32One})
		s.FillStencil(m, size, false)
	})
	wantMatch(t, file, `/S /Luminosity`)
	wantNoMatch(t, file, `/ImageMask`)
}

func TestNewImageMask(t *testing.T) {
	m := image.NewAlpha(image.Rect(0, 0, 10, 3))
	m.Pix[0], m.Pix[12], m.Pix[28] = 0xff, 0xff, 0xff
	// A sub-image has a stride that is not its width. Its pixels 1, 0 and
	// 7, 1 are filled, and the bits that pad a row are 0.
	sub, ok := m.SubImage(image.Rect(1, 1, 10, 3)).(*image.Alpha)
	if !ok {
		t.Fatal("no sub-image")
	}
	got := newImageMask(sub, false)
	want := []byte{0xbf, 0x80, 0xfe, 0x80}
	if got.width != 9 || got.height != 2 || !bytes.Equal(got.bits, want) {
		t.Errorf("the mask is %d by %d with %x, want 9 by 2 with %x", got.width, got.height, got.bits, want)
	}
	if !isBinaryStencil(sub) {
		t.Error("the stencil is not binary")
	}
	m.Pix[29] = 1
	if isBinaryStencil(sub) {
		t.Error("the stencil is binary")
	}
}

func TestManyStops(t *testing.T) {
	for _, c := range []struct {
		stops, stitching int
	}{
		{257, 1},
		{258, 3},
		{600, 4},
		{70000, 277},
	} {
		stops := make([]Stop, c.stops)
		for i := range stops {
			v := uint8(i)
			stops[i] = Stop{
				Offset:  path.NormalizedF32Clamped(float32(i) / float32(c.stops-1)),
				Color:   NewColorRGB(v, 255-v, 0),
				Opacity: path.NormalizedF32Clamped(float32(i%2+1) / 2),
			}
		}
		file := mustIvyFile(t, func(s *Surface) {
			s.SetFill(&Fill{Paint: LinearGradient{X2: 200, Transform: path.IdentityTransform(), Stops: stops}, Opacity: NormalizedF32One})
			s.DrawPath(rectToPath(t, 0, 0, 200, 200))
		})
		// The colors and the opacities each have a tree.
		if n := strings.Count(file, "/FunctionType 3"); n != 2*c.stitching {
			t.Errorf("%d stops: the file has %d stitching functions, want %d", c.stops, n, 2*c.stitching)
		}
		if n := strings.Count(file, "/FunctionType 2"); n != 2*(c.stops-1) {
			t.Errorf("%d stops: the file has %d exponential functions, want %d", c.stops, n, 2*(c.stops-1))
		}
		for _, f := range regexp.MustCompile(`/Functions \[([^\]]*)\]`).FindAllStringSubmatch(file, -1) {
			if n := strings.Count(f[1], " R"); n > maxStitchedFunctions {
				t.Errorf("%d stops: a stitching function has %d functions", c.stops, n)
			}
		}
	}
}
