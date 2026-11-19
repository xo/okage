package krilla

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
)

// The tests of this file are the gate of phase 18 of the PDF writer in
// docs/PLAN.md (D68). Each scene draws one addition of D62 item 11, or the
// fallback of item 10, twice: okage draws it into a pixmap, and pdf writes it
// into a PDF file. MuPDF and Poppler render the file at 72 dots for each
// inch, so that a pixel is a unit of the page. A pixel differs when a channel
// differs by more than 32 (D31). The viewers anti-alias the edges in their
// own way, so a scene passes when at most 1 percent of its pixels differ.
// qpdf must accept each file.

// ivySize is the width and the height of the page of a scene.
const ivySize = 200

// ivyScene is a scene of the gate.
type ivyScene struct {
	name string
	// okage draws the scene onto a white pixmap.
	okage func(t *testing.T, pm *okage.Pixmap)
	// pdf draws the scene onto the surface of a page.
	pdf func(t *testing.T, s *pdf.Surface)
	// limit is the largest share of the pixels that can differ, if it is not
	// the 1 percent of the gate. A scene that sets it says why.
	limit float64
	// known holds the reason of a fault of a viewer, by the name of the
	// viewer. The test logs the pixels that differ in that viewer, and does
	// not fail.
	known map[string]string
}

// TestIvyAdditions is the gate of phase 18.
func TestIvyAdditions(t *testing.T) {
	out := os.Getenv("OKAGE_IVY_OUT")
	for _, sc := range ivyScenes() {
		t.Run(sc.name, func(t *testing.T) {
			t.Helper()
			want := ivyOkage(t, sc)
			data := ivyPDF(t, sc)
			file := writeTemp(t, sc.name+".pdf", data)
			if out != "" {
				ivySave(t, out, sc.name, data, want)
			}
			qpdfCheck(t, file)
			limit := sc.limit
			if limit == 0 {
				limit = 0.01
			}
			for _, r := range []struct {
				name   string
				render func(*testing.T, string, int, float64) *image.NRGBA
			}{
				{"mutool", renderMuPDF},
				{"pdftoppm", renderPoppler},
			} {
				t.Run(r.name, func(t *testing.T) {
					t.Helper()
					got := r.render(t, file, 1, 72)
					if out != "" {
						ivySavePNG(t, filepath.Join(out, sc.name+"_"+r.name+".png"), got)
					}
					n := diffPixels(got, want, 32)
					total := ivySize * ivySize
					t.Logf("%d of %d pixels differ by more than 32", n, total)
					if reason, ok := sc.known[r.name]; ok {
						t.Logf("a known fault of %s: %s", r.name, reason)
						return
					}
					if float64(n) > limit*float64(total) {
						t.Errorf("%d of %d pixels differ by more than 32, above %.1f percent", n, total, 100*limit)
					}
				})
			}
		})
	}
}

// ivySave writes the PDF file and the image of okage of a scene into a
// folder, for a person who looks at them.
func ivySave(t *testing.T, dir, name string, data []byte, want *image.NRGBA) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".pdf"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	pm, ok := okage.NewPixmap(ivySize, ivySize)
	if !ok {
		t.Fatal("no pixmap")
	}
	copy(pm.DataMut(), want.Pix)
	if err := pm.SavePNG(filepath.Join(dir, name+".png")); err != nil {
		t.Fatal(err)
	}
}

// ivySavePNG writes an image as a PNG file.
func ivySavePNG(t *testing.T, name string, img image.Image) {
	t.Helper()
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // a test
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// ivyOkage draws the okage side of a scene onto a white pixmap.
func ivyOkage(t *testing.T, sc ivyScene) *image.NRGBA {
	t.Helper()
	pm, ok := okage.NewPixmap(ivySize, ivySize)
	if !ok {
		t.Fatal("no pixmap")
	}
	pm.Fill(okage.ColorWhite)
	sc.okage(t, pm)
	img := image.NewNRGBA(image.Rect(0, 0, ivySize, ivySize))
	pixels := pm.Pixels()
	for i := range pixels.Len() {
		c := pixels.At(i).Demultiply()
		copy(img.Pix[4*i:], []byte{c.Red(), c.Green(), c.Blue(), c.Alpha()})
	}
	return img
}

// ivyPDF writes the pdf side of a scene as a file of one page.
func ivyPDF(t *testing.T, sc ivyScene) []byte {
	t.Helper()
	d := pdf.NewDocument()
	ps, ok := pdf.PageSettingsFromWH(ivySize, ivySize)
	if !ok {
		t.Fatal("no page settings")
	}
	page := d.StartPage(ps)
	s := page.Surface()
	sc.pdf(t, s)
	s.Finish()
	page.Finish()
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatalf("finishing the document: %v", err)
	}
	return buf.Bytes()
}

// circle returns the path of a circle.
func circle(t *testing.T, cx, cy, r float32) *path.Path {
	t.Helper()
	p, ok := path.PathFromCircle(cx, cy, r)
	if !ok {
		t.Fatal("no circle")
	}
	return p
}

// rectPath returns the path of a rectangle.
func rectPath(t *testing.T, x, y, w, h float32) *path.Path {
	t.Helper()
	r, ok := path.RectFromXYWH(x, y, w, h)
	if !ok {
		t.Fatal("no rectangle")
	}
	return path.PathFromRect(r)
}

// polygon returns the closed path through points, given as x and y pairs.
func polygon(t *testing.T, xy ...float32) *path.Path {
	t.Helper()
	b := path.NewPathBuilder()
	b.MoveTo(xy[0], xy[1])
	for i := 2; i+1 < len(xy); i += 2 {
		b.LineTo(xy[i], xy[i+1])
	}
	b.Close()
	p, ok := b.Finish()
	if !ok {
		t.Fatal("no polygon")
	}
	return p
}

// rgba returns a color of okage from bytes.
func rgba(r, g, b, a uint8) okage.Color {
	return okage.ColorFromRGBA8(r, g, b, a)
}

// fillOkage fills a path on a pixmap with a shader.
func fillOkage(pm *okage.Pixmap, p *path.Path, shader okage.Shader, mask *okage.Mask) {
	paint := okage.DefaultPaint()
	paint.Shader = shader
	pm.FillPath(p, &paint, okage.FillRuleWinding, okage.IdentityTransform(), mask)
}

// fillPDF fills a path on a surface with a shader, through
// Surface.PaintFromShader.
func fillPDF(t *testing.T, s *pdf.Surface, p *path.Path, shader okage.Shader) {
	t.Helper()
	paint, opacity, ok := s.PaintFromShader(shader)
	if !ok {
		t.Fatalf("the shader %T does not convert", shader)
	}
	s.SetFill(&pdf.Fill{Paint: paint, Opacity: opacity, Rule: pdf.FillRuleNonZero})
	s.SetStroke(nil)
	s.DrawPath(p)
}

// stops returns the stops of a gradient from positions and colors.
func stops(pos []float32, colors ...okage.Color) []okage.GradientStop {
	out := make([]okage.GradientStop, len(pos))
	for i := range pos {
		out[i] = okage.NewGradientStop(pos[i], colors[i])
	}
	return out
}

// must returns a function that returns the shader of a constructor, or
// fails the test.
func must(t *testing.T) func(s okage.Shader, ok bool) okage.Shader {
	t.Helper()
	return func(s okage.Shader, ok bool) okage.Shader {
		t.Helper()
		if !ok {
			t.Fatal("no shader")
		}
		return s
	}
}

// ivyScenes returns the scenes of the gate.
func ivyScenes() []ivyScene {
	var scenes []ivyScene
	scenes = append(scenes, groupScenes()...)
	scenes = append(scenes, maskScenes()...)
	scenes = append(scenes, shaderScenes()...)
	scenes = append(scenes, stencilScenes()...)
	scenes = append(scenes, strokeAdjustScenes()...)
	scenes = append(scenes, rasterScenes()...)
	return scenes
}

// knockoutElements are the elements of the knockout groups: two circles that
// overlap, each with an opacity.
func knockoutElements(t *testing.T) ([]*path.Path, []okage.Color) {
	t.Helper()
	return []*path.Path{circle(t, 80, 100, 50), circle(t, 120, 100, 50)},
		[]okage.Color{rgba(220, 30, 30, 153), rgba(30, 30, 220, 153)}
}

// groupScenes draw the transparency groups: knockout groups that are
// isolated and that are not, and a group with an opacity, a blend mode and a
// mask in one graphics state.
func groupScenes() []ivyScene {
	backdrop := func(t *testing.T) (*path.Path, okage.Color) {
		t.Helper()
		return rectPath(t, 20, 60, 160, 80), rgba(40, 180, 60, 255)
	}
	// Poppler 26.11 draws a knockout group as a group without the flag K.
	known := map[string]string{"pdftoppm": "no knockout groups"}
	return []ivyScene{{
		name:  "knockout_isolated",
		known: known,
		okage: func(t *testing.T, pm *okage.Pixmap) {
			t.Helper()
			paths, colors := knockoutElements(t)
			layer, _ := okage.NewPixmap(ivySize, ivySize)
			for i, p := range paths {
				elem, _ := okage.NewPixmap(ivySize, ivySize)
				fillOkage(elem, p, okage.SolidColor(colors[i]), nil)
				shape, _ := okage.NewMask(ivySize, ivySize)
				shape.FillPath(p, okage.FillRuleWinding, true, okage.IdentityTransform())
				layer.Knockout(elem.AsRef(), okage.PixmapRef{}, shape)
			}
			pp := okage.DefaultPixmapPaint()
			pm.DrawPixmap(0, 0, layer.AsRef(), &pp, okage.IdentityTransform(), nil)
		},
		pdf: func(t *testing.T, s *pdf.Surface) {
			t.Helper()
			paths, colors := knockoutElements(t)
			s.PushGroup(pdf.Group{Opacity: pdf.NormalizedF32One, Isolated: true, Knockout: true})
			for i, p := range paths {
				fillPDF(t, s, p, okage.SolidColor(colors[i]))
			}
			s.Pop()
		},
	}, {
		// A knockout group that is not isolated draws each element over the
		// backdrop of the group, as D56 composites it.
		name:  "knockout_backdrop",
		known: known,
		okage: func(t *testing.T, pm *okage.Pixmap) {
			t.Helper()
			bp, bc := backdrop(t)
			fillOkage(pm, bp, okage.SolidColor(bc), nil)
			initial := pm.Clone()
			layer := pm.Clone()
			alpha, _ := okage.NewPixmap(ivySize, ivySize)
			paths, colors := knockoutElements(t)
			for i, p := range paths {
				shape, _ := okage.NewMask(ivySize, ivySize)
				shape.FillPath(p, okage.FillRuleWinding, true, okage.IdentityTransform())
				elem := initial.Clone()
				fillOkage(elem, p, okage.SolidColor(colors[i]), nil)
				layer.Knockout(elem.AsRef(), initial.AsRef(), shape)
				elemAlpha, _ := okage.NewPixmap(ivySize, ivySize)
				fillOkage(elemAlpha, p, okage.SolidColor(colors[i]), nil)
				alpha.Knockout(elemAlpha.AsRef(), okage.PixmapRef{}, shape)
			}
			layer.RemoveBackdrop(initial.AsRef(), alpha.AsRef())
			pp := okage.DefaultPixmapPaint()
			pm.DrawPixmap(0, 0, layer.AsRef(), &pp, okage.IdentityTransform(), nil)
		},
		pdf: func(t *testing.T, s *pdf.Surface) {
			t.Helper()
			bp, bc := backdrop(t)
			fillPDF(t, s, bp, okage.SolidColor(bc))
			paths, colors := knockoutElements(t)
			s.PushGroup(pdf.Group{Opacity: pdf.NormalizedF32One, Knockout: true})
			for i, p := range paths {
				fillPDF(t, s, p, okage.SolidColor(colors[i]))
			}
			s.Pop()
		},
	}, {
		// One group holds an opacity, the blend mode multiply and an alpha
		// mask.
		name: "group_state",
		okage: func(t *testing.T, pm *okage.Pixmap) {
			t.Helper()
			bp, bc := backdrop(t)
			fillOkage(pm, bp, okage.SolidColor(bc), nil)
			layer, _ := okage.NewPixmap(ivySize, ivySize)
			fillOkage(layer, circle(t, 70, 100, 50), okage.SolidColor(rgba(230, 200, 30, 255)), nil)
			fillOkage(layer, circle(t, 130, 100, 50), okage.SolidColor(rgba(200, 40, 200, 255)), nil)
			content, _ := okage.NewPixmap(ivySize, ivySize)
			fillOkage(content, rectPath(t, 0, 0, ivySize, 120), okage.SolidColor(rgba(0, 0, 0, 255)), nil)
			mask := okage.MaskFromPixmap(content.AsRef(), okage.MaskTypeAlpha)
			pp := okage.DefaultPixmapPaint()
			pp.Opacity = 0.8
			pp.BlendMode = okage.BlendModeMultiply
			pm.DrawPixmap(0, 0, layer.AsRef(), &pp, okage.IdentityTransform(), mask)
		},
		pdf: func(t *testing.T, s *pdf.Surface) {
			t.Helper()
			bp, bc := backdrop(t)
			fillPDF(t, s, bp, okage.SolidColor(bc))
			b := s.StreamBuilder()
			ms := b.Surface()
			fillPDF(t, ms, rectPath(t, 0, 0, ivySize, 120), okage.SolidColor(rgba(0, 0, 0, 255)))
			ms.Finish()
			o, _ := pdf.NewNormalizedF32(0.8)
			s.PushGroup(pdf.Group{
				Opacity:   o,
				BlendMode: pdf.BlendModeMultiply,
				Isolated:  true,
				Mask:      pdf.NewMask(b.Finish(), pdf.MaskTypeAlpha),
			})
			fillPDF(t, s, circle(t, 70, 100, 50), okage.SolidColor(rgba(230, 200, 30, 255)))
			fillPDF(t, s, circle(t, 130, 100, 50), okage.SolidColor(rgba(200, 40, 200, 255)))
			s.Pop()
		},
	}}
}

// maskTransfer is the transfer function of the mask scenes. It takes the
// square root of each value.
func maskTransfer() *[256]uint8 {
	var table [256]uint8
	for i := range table {
		table[i] = uint8(math.Round(255 * math.Sqrt(float64(i)/255)))
	}
	return &table
}

// maskScenes draw a luminosity mask with a backdrop color and a transfer
// function. The content of the mask is gray, and the backdrop too, because
// PDF and okage weigh red, green and blue in other ways (D62).
func maskScenes() []ivyScene {
	content := func(t *testing.T) (*path.Path, okage.Color, *path.Path, okage.Color) {
		t.Helper()
		return circle(t, 100, 100, 60), rgba(200, 200, 200, 255), rectPath(t, 20, 20, 60, 60), rgba(40, 40, 40, 160)
	}
	backdrop := rgba(64, 64, 64, 255)
	scene := func(name string, bc bool, tr bool) ivyScene {
		return ivyScene{
			name: name,
			okage: func(t *testing.T, pm *okage.Pixmap) {
				t.Helper()
				p1, c1, p2, c2 := content(t)
				mc, _ := okage.NewPixmap(ivySize, ivySize)
				fillOkage(mc, p1, okage.SolidColor(c1), nil)
				fillOkage(mc, p2, okage.SolidColor(c2), nil)
				b := okage.ColorBlack
				if bc {
					b = backdrop
				}
				mask := okage.MaskFromLuminance(mc.AsRef(), b)
				if tr {
					mask.ApplyTransfer(maskTransfer())
				}
				fillOkage(pm, rectPath(t, 0, 0, ivySize, ivySize), okage.SolidColor(rgba(200, 30, 30, 255)), mask)
			},
			pdf: func(t *testing.T, s *pdf.Surface) {
				t.Helper()
				p1, c1, p2, c2 := content(t)
				b := s.StreamBuilder()
				ms := b.Surface()
				fillPDF(t, ms, p1, okage.SolidColor(c1))
				fillPDF(t, ms, p2, okage.SolidColor(c2))
				ms.Finish()
				mask := pdf.NewMask(b.Finish(), pdf.MaskTypeLuminosity)
				if bc {
					mask = mask.WithBackdrop(pdf.NewColorRGB(64, 64, 64))
				}
				if tr {
					mask = mask.WithTransfer(maskTransfer())
				}
				s.PushMask(mask)
				fillPDF(t, s, rectPath(t, 0, 0, ivySize, ivySize), okage.SolidColor(rgba(200, 30, 30, 255)))
				s.Pop()
			},
		}
	}
	return []ivyScene{
		scene("mask_backdrop", true, false),
		scene("mask_transfer", false, true),
		scene("mask_backdrop_transfer", true, true),
	}
}

// withLimit sets the limit of a scene.
func withLimit(limit float64, sc ivyScene) ivyScene {
	sc.limit = limit
	return sc
}

// shaderScene is a scene that fills paths with shaders, which the pdf side
// converts with Surface.PaintFromShader.
func shaderScene(name string, fills func(t *testing.T) ([]*path.Path, []okage.Shader)) ivyScene {
	return ivyScene{
		name: name,
		okage: func(t *testing.T, pm *okage.Pixmap) {
			t.Helper()
			paths, shaders := fills(t)
			for i, p := range paths {
				fillOkage(pm, p, shaders[i], nil)
			}
		},
		pdf: func(t *testing.T, s *pdf.Surface) {
			t.Helper()
			paths, shaders := fills(t)
			for i, p := range paths {
				fillPDF(t, s, p, shaders[i])
			}
		},
	}
}

// quadrants returns the four quadrants of the page, with a margin.
func quadrants(t *testing.T) []*path.Path {
	t.Helper()
	return []*path.Path{rectPath(t, 5, 5, 90, 90), rectPath(t, 105, 5, 90, 90), rectPath(t, 5, 105, 90, 90), rectPath(t, 105, 105, 90, 90)}
}

// Poppler 26.11 smooths an image that it scales up by less than 4 times,
// even with Interpolate false. The patterns and the stencils of the scenes
// thus scale their pixels by 4 or more.

// shaderScenes draw each kind of shader of okage through the conversion:
// colors, linear, radial and sweep gradients, the decal mode, the radial
// gradients of NewRadialGradientExtend, meshes and patterns.
func shaderScenes() []ivyScene {
	red, green, blue := rgba(220, 40, 40, 255), rgba(40, 200, 60, 255), rgba(40, 60, 220, 255)
	pos3 := []float32{0, 0.4, 1}
	return []ivyScene{
		shaderScene("shader_colors", func(t *testing.T) ([]*path.Path, []okage.Shader) {
			t.Helper()
			return quadrants(t), []okage.Shader{
				okage.SolidColor(red), okage.SolidColor(rgba(40, 200, 60, 128)),
				okage.SolidColor(rgba(40, 60, 220, 60)), okage.SolidColor(rgba(0, 0, 0, 255)),
			}
		}),
		shaderScene("shader_linear", func(t *testing.T) ([]*path.Path, []okage.Shader) {
			t.Helper()
			st := stops(pos3, red, green, blue)
			pt := okage.PointFromXY
			id := okage.IdentityTransform()
			skew := okage.TransformFromRow(1, 0.2, 0.1, 1, 0, 0)
			return quadrants(t), []okage.Shader{
				must(t)(okage.NewLinearGradient(pt(20, 20), pt(80, 80), st, okage.SpreadModePad, id)),
				must(t)(okage.NewLinearGradient(pt(130, 50), pt(160, 50), st, okage.SpreadModeRepeat, id)),
				must(t)(okage.NewLinearGradient(pt(20, 120), pt(50, 140), st, okage.SpreadModeReflect, skew)),
				must(t)(okage.NewLinearGradient(pt(130, 150), pt(170, 150), st, okage.SpreadModeDecal, id)),
			}
		}),
		shaderScene("shader_linear_alpha", func(t *testing.T) ([]*path.Path, []okage.Shader) {
			t.Helper()
			st := stops([]float32{0, 1}, rgba(220, 40, 40, 255), rgba(40, 60, 220, 40))
			pt := okage.PointFromXY
			return []*path.Path{rectPath(t, 10, 10, 180, 180)}, []okage.Shader{
				must(t)(okage.NewLinearGradient(pt(10, 10), pt(190, 190), st, okage.SpreadModePad, okage.IdentityTransform())),
			}
		}),
		shaderScene("shader_radial", func(t *testing.T) ([]*path.Path, []okage.Shader) {
			t.Helper()
			st := stops(pos3, red, green, blue)
			pt := okage.PointFromXY
			id := okage.IdentityTransform()
			return quadrants(t), []okage.Shader{
				must(t)(okage.NewRadialGradient(pt(50, 50), 0, pt(50, 50), 40, st, okage.SpreadModePad, id)),
				must(t)(okage.NewRadialGradient(pt(140, 40), 5, pt(150, 50), 40, st, okage.SpreadModePad, id)),
				must(t)(okage.NewRadialGradient(pt(50, 150), 10, pt(50, 150), 35, st, okage.SpreadModeDecal, id)),
				must(t)(okage.NewRadialGradient(pt(150, 150), 0, pt(150, 150), 30, st, okage.SpreadModePad,
					okage.TransformFromRow(1.4, 0, 0, 0.8, -60, 30))),
			}
		}),
		// The edges of the cones are edges of the shadings, which no viewer
		// anti-aliases. MuPDF and Poppler put them in other pixels than
		// okage along 8 lines, which is 1.1 percent of the pixels.
		withLimit(0.015, shaderScene("shader_radial_extend", func(t *testing.T) ([]*path.Path, []okage.Shader) {
			t.Helper()
			// Circles that do not nest, with each pair of flags. The rule
			// of PDF and the rule of tiny-skia differ for them (D64).
			st := stops(pos3, red, green, blue)
			var shaders []okage.Shader
			for i, f := range [][2]bool{{true, true}, {true, false}, {false, true}, {false, false}} {
				x := float32(5 + 100*(i%2))
				y := float32(5 + 100*(i/2))
				shaders = append(shaders, must(t)(okage.NewRadialGradientExtend(
					okage.PointFromXY(x+25, y+45), 12, okage.PointFromXY(x+60, y+45), 25,
					st, f[0], f[1], okage.IdentityTransform())))
			}
			return quadrants(t), shaders
		})),
		shaderScene("shader_sweep", func(t *testing.T) ([]*path.Path, []okage.Shader) {
			t.Helper()
			st := stops(pos3, red, green, blue)
			pt := okage.PointFromXY
			id := okage.IdentityTransform()
			return quadrants(t)[:3], []okage.Shader{
				must(t)(okage.NewSweepGradient(pt(50, 50), 0, 360, st, okage.SpreadModePad, id)),
				must(t)(okage.NewSweepGradient(pt(150, 50), 30, 120, st, okage.SpreadModeRepeat, id)),
				must(t)(okage.NewSweepGradient(pt(50, 150), 45, 270, st, okage.SpreadModeReflect, id)),
			}
		}),
		shaderScene("shader_mesh", func(t *testing.T) ([]*path.Path, []okage.Shader) {
			t.Helper()
			v := func(x, y float32, c okage.Color) okage.MeshVertex {
				return okage.MeshVertex{Point: okage.PointFromXY(x, y), Color: c}
			}
			opaque := must(t)(okage.NewMeshGradient([]okage.MeshTriangle{
				{v(10, 10, red), v(90, 10, green), v(10, 90, blue)},
				{v(90, 10, green), v(90, 90, rgba(230, 220, 40, 255)), v(10, 90, blue)},
			}, okage.IdentityTransform()))
			alpha := must(t)(okage.NewMeshGradient([]okage.MeshTriangle{
				{v(0, 0, rgba(220, 40, 40, 255)), v(80, 0, rgba(40, 200, 60, 80)), v(0, 80, rgba(40, 60, 220, 200))},
			}, okage.TransformFromTranslate(110, 10)))
			turned := must(t)(okage.NewMeshGradient([]okage.MeshTriangle{
				{v(0, 0, red), v(60, 0, green), v(30, 50, blue)},
			}, okage.TransformFromRow(1, 0.3, -0.3, 1, 60, 110)))
			return []*path.Path{
				rectPath(t, 10, 10, 80, 80),
				polygon(t, 110, 10, 190, 10, 110, 90),
				polygon(t, 60, 110, 120, 128, 105, 186),
			}, []okage.Shader{opaque, alpha.ApplyOpacity(0.9), turned}
		}),
		shaderScene("shader_pattern", func(t *testing.T) ([]*path.Path, []okage.Shader) {
			t.Helper()
			tile := checker(t)
			return quadrants(t)[:3], []okage.Shader{
				okage.NewPattern(tile.AsRef(), okage.SpreadModeRepeat, okage.FilterQualityNearest, 1,
					okage.TransformFromRow(4, 0, 0, 4, 5, 5)),
				okage.NewPattern(tile.AsRef(), okage.SpreadModeReflect, okage.FilterQualityNearest, 0.7,
					okage.TransformFromRow(4, 0, 0, 5, 105, 5)),
				okage.NewPattern(tile.AsRef(), okage.SpreadModeRepeat, okage.FilterQualityNearest, 1,
					okage.TransformFromRow(5, 0, 0, 5, 5, 105)),
			}
		}),
	}
}

// checker returns a pixmap of 6 by 4 pixels with colors that differ in each
// corner, so that a mirror shows.
func checker(t *testing.T) *okage.Pixmap {
	t.Helper()
	pm, ok := okage.NewPixmap(6, 4)
	if !ok {
		t.Fatal("no pixmap")
	}
	px := pm.PixelsMut()
	for y := range 4 {
		for x := range 6 {
			c := rgba(uint8(40*x), uint8(60*y), 200, 255)
			if (x+y)%2 == 0 {
				c = rgba(250, 240, 220, 255)
			}
			px.Set(y*6+x, c.Premultiply().ToColorU8())
		}
	}
	return pm
}

// stencil returns a stencil of 20 by 20 pixels: a ring with alphas that
// grow from the left to the right.
func stencil() *image.Alpha {
	m := image.NewAlpha(image.Rect(0, 0, 20, 20))
	for y := range 20 {
		for x := range 20 {
			d := math.Hypot(float64(x)-9.5, float64(y)-9.5)
			if d > 4 && d < 9 {
				m.Pix[y*m.Stride+x] = uint8(60 + 10*x)
			}
		}
	}
	return m
}

// stencilScenes draw a stencil of 8 bits with a color and with a gradient,
// at 5 units for each pixel of the stencil.
func stencilScenes() []ivyScene {
	scene := func(name string, shader func(t *testing.T) okage.Shader) ivyScene {
		return ivyScene{
			name: name,
			okage: func(t *testing.T, pm *okage.Pixmap) {
				t.Helper()
				// The fill of ivy: the shader through a mask of the stencil,
				// with the pixels of the stencil scaled by 5.
				m := stencil()
				sp, _ := okage.NewPixmap(20, 20)
				px := sp.PixelsMut()
				for i, a := range m.Pix {
					c, _ := okage.PremultipliedColorU8FromRGBA(0, 0, 0, a)
					px.Set(i, c)
				}
				layer, _ := okage.NewPixmap(ivySize, ivySize)
				pp := okage.DefaultPixmapPaint()
				pp.Quality = okage.FilterQualityNearest
				layer.DrawPixmap(0, 0, sp.AsRef(), &pp, okage.TransformFromRow(5, 0, 0, 5, 50, 50), nil)
				mask := okage.MaskFromPixmap(layer.AsRef(), okage.MaskTypeAlpha)
				fillOkage(pm, rectPath(t, 50, 50, 100, 100), shader(t), mask)
			},
			pdf: func(t *testing.T, s *pdf.Surface) {
				t.Helper()
				// The shader is in the space of the page, and the stencil
				// in the space of the translation.
				sh := shader(t).Transform(okage.TransformFromTranslate(-50, -50))
				paint, opacity, ok := s.PaintFromShader(sh)
				if !ok {
					t.Fatal("the shader does not convert")
				}
				s.SetFill(&pdf.Fill{Paint: paint, Opacity: opacity})
				s.PushTransform(okage.TransformFromTranslate(50, 50))
				size, _ := path.SizeFromWH(100, 100)
				s.FillStencil(stencil(), size, false)
				s.Pop()
			},
		}
	}
	return []ivyScene{
		scene("stencil_color", func(*testing.T) okage.Shader { return okage.SolidColor(rgba(30, 60, 200, 255)) }),
		scene("stencil_gradient", func(t *testing.T) okage.Shader {
			t.Helper()
			st := stops([]float32{0, 1}, rgba(220, 40, 40, 255), rgba(40, 60, 220, 255))
			return must(t)(okage.NewLinearGradient(okage.PointFromXY(50, 50), okage.PointFromXY(150, 150), st,
				okage.SpreadModePad, okage.IdentityTransform()))
		}),
	}
}

// strokeAdjustLines returns thin horizontal and vertical lines that lie
// across the borders of the pixels.
func strokeAdjustLines(t *testing.T) []*path.Path {
	t.Helper()
	var lines []*path.Path
	for i := range 8 {
		y := 20.5 + float32(i)*20.25
		b := path.NewPathBuilder()
		b.MoveTo(20, y)
		b.LineTo(180, y)
		b.MoveTo(y, 20)
		b.LineTo(y, 180)
		p, ok := b.Finish()
		if !ok {
			t.Fatal("no line")
		}
		lines = append(lines, p)
	}
	return lines
}

// strokeAdjustScenes stroke thin lines with the stroke adjustment of PDF and
// of okage (D54).
func strokeAdjustScenes() []ivyScene {
	// MuPDF 1.28.6 ignores the entry SA. Poppler 26.11 rounds each edge of
	// a line to the pixels, so a line of 1.6 units is 1 or 2 pixels wide by
	// its place. okage rounds the width first, to 2 pixels (D54).
	scene := func(name string, width float32, known map[string]string) ivyScene {
		return ivyScene{
			name:  name,
			known: known,
			okage: func(t *testing.T, pm *okage.Pixmap) {
				t.Helper()
				paint := okage.DefaultPaint()
				paint.StrokeAdjust = true
				st := okage.DefaultStroke()
				st.Width = width
				for _, p := range strokeAdjustLines(t) {
					pm.StrokePath(p, &paint, &st, okage.IdentityTransform(), nil)
				}
			},
			pdf: func(t *testing.T, s *pdf.Surface) {
				t.Helper()
				st := pdf.DefaultStroke()
				st.Paint = pdf.NewColorRGB(0, 0, 0)
				st.Width = width
				s.SetFill(nil)
				s.SetStroke(&st)
				s.PushStrokeAdjustment(true)
				for _, p := range strokeAdjustLines(t) {
					s.DrawPath(p)
				}
				s.Pop()
			},
		}
	}
	mupdf := "no stroke adjustment"
	return []ivyScene{
		scene("stroke_adjust_thin", 0.6, map[string]string{"mutool": mupdf}),
		scene("stroke_adjust_wide", 1.6, map[string]string{"mutool": mupdf, "pdftoppm": "another rounding of the width"}),
	}
}

// rasterScenes draw a part of the page with the fallback: a group with the
// blend mode destination out, which PDF does not have, over a backdrop.
func rasterScenes() []ivyScene {
	draw := func(t *testing.T, pm *okage.Pixmap, ts okage.Transform) {
		t.Helper()
		paint := okage.DefaultPaint()
		paint.SetColor(rgba(220, 120, 30, 255))
		pm.FillPath(circle(t, 100, 100, 70), &paint, okage.FillRuleWinding, ts, nil)
		paint.BlendMode = okage.BlendModeDestinationOut
		paint.SetColor(rgba(0, 0, 0, 200))
		pm.FillPath(circle(t, 120, 90, 35), &paint, okage.FillRuleWinding, ts, nil)
	}
	return []ivyScene{{
		name: "raster_fallback",
		// Poppler 26.11 smooths the image of the fallback, which it draws
		// at its own size, and the edges of the circles get other colors.
		known: map[string]string{"pdftoppm": "a smoothed image"},
		okage: func(t *testing.T, pm *okage.Pixmap) {
			t.Helper()
			fillOkage(pm, rectPath(t, 0, 80, ivySize, 40), okage.SolidColor(rgba(30, 60, 200, 255)), nil)
			layer, _ := okage.NewPixmap(ivySize, ivySize)
			draw(t, layer, okage.IdentityTransform())
			pp := okage.DefaultPixmapPaint()
			pm.DrawPixmap(0, 0, layer.AsRef(), &pp, okage.IdentityTransform(), nil)
		},
		pdf: func(t *testing.T, s *pdf.Surface) {
			t.Helper()
			fillPDF(t, s, rectPath(t, 0, 80, ivySize, 40), okage.SolidColor(rgba(30, 60, 200, 255)))
			r, _ := path.RectFromXYWH(20, 20, 160, 160)
			if !s.DrawRaster(r, 1, func(pm *okage.Pixmap, ts okage.Transform) { draw(t, pm, ts) }) {
				t.Fatal("DrawRaster drew nothing")
			}
		},
	}}
}
