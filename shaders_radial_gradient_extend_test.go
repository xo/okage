package okage_test

// This file tests NewRadialGradientExtend, which ports no upstream test
// (D64). The tests compare each pixel with the rule of the radial shadings of
// ISO 32000-1, 8.7.4.5.4, which the test computes in float64.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"testing"

	"github.com/xo/okage"
)

// extendCase is the geometry of a radial gradient in a test.
type extendCase struct {
	name   string
	x0, y0 float64
	r0     float64
	x1, y1 float64
	r1     float64
}

// extendCases are the gradients of the tests. They cover circles that do
// not nest, circles that nest, a cone that narrows, concentric circles, equal
// radii, a start circle that touches the end circle on the inside, and two
// circles of radius 0.
var extendCases = []extendCase{
	{"apart", 60, 100, 20, 140, 100, 40},
	{"apart-vertical", 100, 150, 30, 110, 60, 15},
	{"narrows", 70, 100, 50, 140, 90, 10},
	{"cone", 80, 100, 10, 140, 100, 40},
	{"nested", 80, 90, 0, 100, 100, 80},
	{"nested-start", 100, 100, 70, 120, 110, 20},
	{"concentric", 100, 100, 20, 100, 100, 70},
	{"concentric-reversed", 100, 100, 70, 100, 100, 20},
	{"equal-radii", 50, 80, 30, 150, 120, 30},
	{"touching", 100, 100, 0, 140, 100, 40},
	{"points", 50, 50, 0, 150, 150, 0},
}

// extendStops are the stops of the tests: two stops that are not opaque,
// and three stops that are not evenly spaced.
var extendStops = []struct {
	pos  []float64
	rgba [][4]uint8
}{
	{
		[]float64{0, 1},
		[][4]uint8{{50, 127, 150, 200}, {220, 140, 75, 180}},
	},
	{
		[]float64{0.2, 0.3, 0.9},
		[][4]uint8{{250, 0, 0, 255}, {0, 250, 0, 255}, {0, 0, 250, 255}},
	},
}

// gradientStops makes the stops of okage for the stops i of extendStops.
func gradientStops(i int) []okage.GradientStop {
	s := extendStops[i]
	stops := make([]okage.GradientStop, len(s.pos))
	for j, p := range s.pos {
		c := s.rgba[j]
		stops[j] = okage.NewGradientStop(float32(p), okage.ColorFromRGBA8(c[0], c[1], c[2], c[3]))
	}
	return stops
}

// refExtendT gives the t of the point (x, y) in the space of the gradient
// by the rule of PDF: the largest t whose circle passes through the point and
// has a radius of at least 0, in 0..=1, or below 0 with extendStart, or above
// 1 with extendEnd. It returns false when the point has no such t. unsure is
// true when the point is too near a limit for float32 to give the same
// answer.
func refExtendT(c extendCase, extendStart, extendEnd bool, x, y float64) (t float64, ok, unsure bool) {
	// The same space as NewRadialGradientExtend: the start center at the
	// origin, and the largest of the distance and the radii is 1.
	k := 1 / max(math.Hypot(c.x1-c.x0, c.y1-c.y0), c.r0, c.r1)
	cx, cy := (c.x1-c.x0)*k, (c.y1-c.y0)*k
	r0, dr := c.r0*k, (c.r1-c.r0)*k
	px, py := (x-c.x0)*k, (y-c.y0)*k

	a := cx*cx + cy*cy - dr*dr
	if math.Abs(a) <= 1.0/4096 {
		a = 0
	}
	b := px*cx + py*cy + r0*dr
	cc := px*px + py*py - r0*r0

	var roots []float64
	if a == 0 {
		if math.Abs(b) < 1e-4 {
			return 0, false, true
		}
		roots = []float64{cc / (2 * b)}
	} else {
		disc := b*b - a*cc
		if math.Abs(disc) < 1e-4*(b*b+math.Abs(a*cc))+1e-6 {
			unsure = true
		}
		if disc < 0 {
			return 0, false, unsure
		}
		sq := math.Sqrt(disc)
		roots = []float64{(b + sq) / a, (b - sq) / a}
		if roots[0] < roots[1] {
			roots[0], roots[1] = roots[1], roots[0]
		}
	}
	for _, s := range roots {
		r := r0 + s*dr
		if math.Abs(s) < 1e-3 || math.Abs(s-1) < 1e-3 || math.Abs(r) < 1e-3 {
			unsure = true
		}
		if (extendStart || s >= 0) && (extendEnd || s <= 1) && r >= 0 {
			return s, true, unsure
		}
	}
	return 0, false, unsure
}

// refExtendColor gives the premultiplied color of the stops i of
// extendStops at t, with the opacity, as the bytes that the pipeline stores.
func refExtendColor(i int, t, opacity float64) [4]uint8 {
	s := extendStops[i]
	t = min(max(t, 0), 1)
	var c [4]float64
	switch {
	case t <= s.pos[0]:
		c = rgbaF(s.rgba[0])
	case t >= s.pos[len(s.pos)-1]:
		c = rgbaF(s.rgba[len(s.rgba)-1])
	default:
		for j := 1; j < len(s.pos); j++ {
			if t <= s.pos[j] {
				u := (t - s.pos[j-1]) / (s.pos[j] - s.pos[j-1])
				c0, c1 := rgbaF(s.rgba[j-1]), rgbaF(s.rgba[j])
				for n := range c {
					c[n] = c0[n] + u*(c1[n]-c0[n])
				}
				break
			}
		}
	}
	c[3] *= opacity
	var out [4]uint8
	for n := range out {
		v := c[n]
		if n < 3 {
			v *= c[3]
		}
		out[n] = uint8(math.Round(min(max(v, 0), 1) * 255))
	}
	return out
}

// rgbaF gives the channels of c in 0..=1.
func rgbaF(c [4]uint8) [4]float64 {
	return [4]float64{float64(c[0]) / 255, float64(c[1]) / 255, float64(c[2]) / 255, float64(c[3]) / 255}
}

// fillExtend fills a pixmap of 200 by 200 with shader.
func fillExtend(t *testing.T, shader okage.Shader) *okage.Pixmap {
	t.Helper()
	if shader.IsOpaque() {
		t.Errorf("a gradient of NewRadialGradientExtend is opaque")
	}
	pixmap := must(okage.NewPixmap(200, 200))
	paint := okage.DefaultPaint()
	paint.Shader = shader
	pixmap.FillRect(must(okage.RectFromLTRB(0, 0, 200, 200)), &paint, okage.IdentityTransform(), nil)
	return pixmap
}

// checkExtend compares each pixel of pixmap with the reference. inv maps a
// pixel to the space of the gradient. A channel can differ by 1. It returns
// the number of pixels that have a t, that have none, and that the test
// skips because they are too near a limit.
func checkExtend(
	t *testing.T, pixmap *okage.Pixmap, c extendCase, stops int,
	extendStart, extendEnd bool, inv okage.Transform, opacity float64,
) (in, out, unsure int) {
	t.Helper()
	for y := range uint32(200) {
		for x := range uint32(200) {
			px, py := float64(x)+0.5, float64(y)+0.5
			gx := float64(inv.SX)*px + float64(inv.KX)*py + float64(inv.TX)
			gy := float64(inv.KY)*px + float64(inv.SY)*py + float64(inv.TY)
			tt, ok, u := refExtendT(c, extendStart, extendEnd, gx, gy)
			if u {
				unsure++
				continue
			}
			var want [4]uint8
			if ok {
				in++
				want = refExtendColor(stops, tt, opacity)
			} else {
				out++
			}
			p := must(pixmap.Pixel(x, y))
			got := [4]uint8{p.Red(), p.Green(), p.Blue(), p.Alpha()}
			for n := range got {
				if d := int(got[n]) - int(want[n]); d < -1 || d > 1 {
					t.Fatalf("the pixel at %d,%d is %v, want %v (t %v, has t %v)", x, y, got, want, tt, ok)
				}
			}
		}
	}
	return in, out, unsure
}

// extendName names a combination of the extend flags.
func extendName(extendStart, extendEnd bool) string {
	return fmt.Sprintf("extend-%t-%t", extendStart, extendEnd)
}

func TestRadialGradientExtend(t *testing.T) {
	for _, c := range extendCases {
		for stops := range extendStops {
			for _, extendStart := range []bool{false, true} {
				for _, extendEnd := range []bool{false, true} {
					name := fmt.Sprintf("%s/%d/%s", c.name, stops, extendName(extendStart, extendEnd))
					t.Run(name, func(t *testing.T) {
						shader := must(okage.NewRadialGradientExtend(
							okage.PointFromXY(float32(c.x0), float32(c.y0)), float32(c.r0),
							okage.PointFromXY(float32(c.x1), float32(c.y1)), float32(c.r1),
							gradientStops(stops), extendStart, extendEnd, okage.IdentityTransform()))
						pixmap := fillExtend(t, shader)
						in, out, unsure := checkExtend(t, pixmap, c, stops, extendStart, extendEnd, okage.IdentityTransform(), 1)
						if out == 0 && !extendStart && !extendEnd {
							t.Errorf("every pixel has a t, want some without one")
						}
						if unsure > 200*200/20 {
							t.Errorf("the test skips %d pixels, want at most %d", unsure, 200*200/20)
						}
						if c.name != "points" && in == 0 {
							t.Errorf("no pixel has a t")
						}
						if c.name == "points" && in != 0 {
							t.Errorf("%d pixels have a t, want 0 for circles of radius 0", in)
						}
					})
				}
			}
		}
	}
}

// TestRadialGradientExtendDiffers shows a case where the rule of PDF and the
// rule of tiny-skia give other colors. The circles do not nest. Inside the
// end circle, the largest t is more than 1, and a smaller t is in 0..=1. The
// decal mode of NewRadialGradient paints nothing there.
func TestRadialGradientExtendDiffers(t *testing.T) {
	c := extendCases[0]
	start := okage.PointFromXY(float32(c.x0), float32(c.y0))
	end := okage.PointFromXY(float32(c.x1), float32(c.y1))
	pdf := fillExtend(t, must(okage.NewRadialGradientExtend(start, float32(c.r0), end, float32(c.r1), gradientStops(0), false, false, okage.IdentityTransform())))
	decal := fillExtend(t, must(okage.NewRadialGradient(start, float32(c.r0), end, float32(c.r1), gradientStops(0), okage.SpreadModeDecal, okage.IdentityTransform())))
	// The point 150, 100 is on the circles of t 0.7 and of t 11/6.
	tt, ok, unsure := refExtendT(c, false, false, 150.5, 100.5)
	if !ok || unsure || tt > 1 {
		t.Fatalf("the t at 150,100 is %v, %v, %v", tt, ok, unsure)
	}
	p, d := must(pdf.Pixel(150, 100)), must(decal.Pixel(150, 100))
	want := refExtendColor(0, tt, 1)
	if got := [4]uint8{p.Red(), p.Green(), p.Blue(), p.Alpha()}; got != want {
		t.Errorf("the pixel at 150,100 is %v, want %v", got, want)
	}
	if d != okage.PremultipliedColorU8Transparent {
		t.Errorf("the pixel of the decal gradient at 150,100 is %v, want transparent", d)
	}
}

// TestRadialGradientExtendPad compares the gradient with both extend flags
// with a gradient of NewRadialGradient in the pad mode. When the circles
// nest, the two rules give the same t. The test skips the pixels near a
// limit, and allows 1 in each channel.
func TestRadialGradientExtendPad(t *testing.T) {
	for _, i := range []int{4, 5, 6, 7} {
		c := extendCases[i]
		t.Run(c.name, func(t *testing.T) {
			start := okage.PointFromXY(float32(c.x0), float32(c.y0))
			end := okage.PointFromXY(float32(c.x1), float32(c.y1))
			pdf := fillExtend(t, must(okage.NewRadialGradientExtend(start, float32(c.r0), end, float32(c.r1), gradientStops(1), true, true, okage.IdentityTransform())))
			pad := fillExtend(t, must(okage.NewRadialGradient(start, float32(c.r0), end, float32(c.r1), gradientStops(1), okage.SpreadModePad, okage.IdentityTransform())))
			var same int
			for y := range uint32(200) {
				for x := range uint32(200) {
					if _, _, unsure := refExtendT(c, true, true, float64(x)+0.5, float64(y)+0.5); unsure {
						continue
					}
					a, b := must(pdf.Pixel(x, y)), must(pad.Pixel(x, y))
					ga := [4]int{int(a.Red()), int(a.Green()), int(a.Blue()), int(a.Alpha())}
					gb := [4]int{int(b.Red()), int(b.Green()), int(b.Blue()), int(b.Alpha())}
					for n := range ga {
						if d := ga[n] - gb[n]; d < -1 || d > 1 {
							t.Fatalf("the pixel at %d,%d is %v, and %v in the pad mode", x, y, ga, gb)
						}
					}
					same++
				}
			}
			if same < 200*200*9/10 {
				t.Errorf("the test compares %d pixels, want most of them", same)
			}
		})
	}
}

// TestRadialGradientExtendTransform turns and scales the gradient, through
// the transform of the constructor and through Shader.Transform, and makes it
// half transparent.
func TestRadialGradientExtendTransform(t *testing.T) {
	c := extendCases[0]
	ts := okage.TransformFromRotateAt(30, 100, 100).PreScale(1.2, 0.8)
	shift := okage.TransformFromTranslate(10, -5)
	for _, extendStart := range []bool{false, true} {
		for _, extendEnd := range []bool{false, true} {
			t.Run(extendName(extendStart, extendEnd), func(t *testing.T) {
				shader := must(okage.NewRadialGradientExtend(
					okage.PointFromXY(float32(c.x0), float32(c.y0)), float32(c.r0),
					okage.PointFromXY(float32(c.x1), float32(c.y1)), float32(c.r1),
					gradientStops(1), extendStart, extendEnd, ts))
				shader = shader.Transform(shift).ApplyOpacity(0.5)
				pixmap := fillExtend(t, shader)
				inv := must(ts.PostConcat(shift).Invert())
				in, _, _ := checkExtend(t, pixmap, c, 1, extendStart, extendEnd, inv, 0.5)
				if in == 0 {
					t.Errorf("no pixel has a t")
				}
			})
		}
	}
}

// TestRadialGradientExtendOneStop makes a gradient with one stop. It has the
// color of the stop where a t exists, and is transparent elsewhere.
func TestRadialGradientExtendOneStop(t *testing.T) {
	c := extendCases[0]
	red := okage.ColorFromRGBA8(255, 0, 0, 255)
	shader := must(okage.NewRadialGradientExtend(
		okage.PointFromXY(float32(c.x0), float32(c.y0)), float32(c.r0),
		okage.PointFromXY(float32(c.x1), float32(c.y1)), float32(c.r1),
		[]okage.GradientStop{okage.NewGradientStop(0.5, red)}, false, false, okage.IdentityTransform()))
	pixmap := fillExtend(t, shader)
	if got := must(pixmap.Pixel(80, 100)); got != must(okage.PremultipliedColorU8FromRGBA(255, 0, 0, 255)) {
		t.Errorf("the pixel at 80,100 is %v, want red", got)
	}
	if got := must(pixmap.Pixel(5, 5)); got != okage.PremultipliedColorU8Transparent {
		t.Errorf("the pixel at 5,5 is %v, want transparent", got)
	}
}

func TestRadialGradientExtendFails(t *testing.T) {
	p := okage.PointFromXY
	stops := gradientStops(0)
	id := okage.IdentityTransform()
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	tests := []struct {
		name   string
		start  okage.Point
		r0     float32
		end    okage.Point
		r1     float32
		stops  []okage.GradientStop
		matrix okage.Transform
	}{
		{"no stops", p(0, 0), 1, p(10, 0), 5, nil, id},
		{"negative start radius", p(0, 0), -1, p(10, 0), 5, stops, id},
		{"negative end radius", p(0, 0), 1, p(10, 0), -5, stops, id},
		{"NaN radius", p(0, 0), nan, p(10, 0), 5, stops, id},
		{"infinite radius", p(0, 0), 1, p(10, 0), inf, stops, id},
		{"NaN point", p(nan, 0), 1, p(10, 0), 5, stops, id},
		{"infinite point", p(0, 0), 1, p(inf, 0), 5, stops, id},
		{"same circle", p(10, 10), 5, p(10, 10), 5, stops, id},
		{"same point", p(10, 10), 0, p(10, 10), 0, stops, id},
		{"not invertible", p(0, 0), 1, p(10, 0), 5, stops, okage.TransformFromRow(1, 1, 1, 1, 0, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if s, ok := okage.NewRadialGradientExtend(tt.start, tt.r0, tt.end, tt.r1, tt.stops, true, false, tt.matrix); ok {
				t.Errorf("got %v, want false", s)
			}
		})
	}
}

// TestRadialGradientExtendBytes checks the bytes of every case of
// TestRadialGradientExtend. The scalar build, the SIMD build and each
// architecture must give these bytes (D16).
func TestRadialGradientExtendBytes(t *testing.T) {
	h := sha256.New()
	for _, c := range extendCases {
		for stops := range extendStops {
			for _, extendStart := range []bool{false, true} {
				for _, extendEnd := range []bool{false, true} {
					shader := must(okage.NewRadialGradientExtend(
						okage.PointFromXY(float32(c.x0), float32(c.y0)), float32(c.r0),
						okage.PointFromXY(float32(c.x1), float32(c.y1)), float32(c.r1),
						gradientStops(stops), extendStart, extendEnd,
						okage.TransformFromRotateAt(20, 100, 100)))
					h.Write(fillExtend(t, shader).Data())
				}
			}
		}
	}
	const want = "7db0d8ca2f5b61e7935cbb9b3814d216c1a801fa61564cfb4b9b104755f837b2"
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		t.Errorf("the hash of the pixels is %s, want %s", got, want)
	}
}
