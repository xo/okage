package okage_test

// This file tests SpreadModeDecal, which ports no upstream test (D55).

import (
	"math"
	"strconv"
	"testing"

	"github.com/xo/okage"
)

// decalStops are the stops of the tests of the decal mode: two stops that
// are not opaque, and three stops that are not evenly spaced.
var decalStops = [][]okage.GradientStop{
	{
		okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
		okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
	},
	{
		okage.NewGradientStop(0.2, okage.ColorFromRGBA8(250, 0, 0, 255)),
		okage.NewGradientStop(0.3, okage.ColorFromRGBA8(0, 250, 0, 255)),
		okage.NewGradientStop(0.9, okage.ColorFromRGBA8(0, 0, 250, 255)),
	},
}

// fillDecal fills a pixmap of 200 by 200 with the gradient that makeShader
// makes in the decal mode, and with the same gradient in the pad mode, in the
// highp pipeline.
func fillDecal(t *testing.T, makeShader func(okage.SpreadMode) (okage.Shader, bool)) (decal, pad *okage.Pixmap) {
	t.Helper()
	rect := must(okage.RectFromLTRB(0, 0, 200, 200))
	paint := okage.DefaultPaint()
	paint.ForceHQPipeline = true

	decal = must(okage.NewPixmap(200, 200))
	paint.Shader = must(makeShader(okage.SpreadModeDecal))
	if paint.Shader.IsOpaque() {
		t.Errorf("a gradient in the decal mode is opaque")
	}
	decal.FillRect(rect, &paint, okage.IdentityTransform(), nil)

	pad = must(okage.NewPixmap(200, 200))
	paint.Shader = must(makeShader(okage.SpreadModePad))
	pad.FillRect(rect, &paint, okage.IdentityTransform(), nil)
	return decal, pad
}

// checkDecal compares the pixels of decal and pad. inside returns 1 for a
// pixel where t is in 0..=1, 0 for a pixel where it is not, and -1 for a
// pixel too near a limit to tell. A pixel inside must be the pixel of pad,
// and a pixel outside must be transparent.
func checkDecal(t *testing.T, decal, pad *okage.Pixmap, inside func(x, y float64) int) {
	t.Helper()
	var in, out int
	for y := range uint32(200) {
		for x := range uint32(200) {
			got := must(decal.Pixel(x, y))
			switch inside(float64(x)+0.5, float64(y)+0.5) {
			case 1:
				in++
				if want := must(pad.Pixel(x, y)); got != want {
					t.Fatalf("the pixel at %d,%d is %v, want %v", x, y, got, want)
				}
			case 0:
				out++
				if got != okage.PremultipliedColorU8Transparent {
					t.Fatalf("the pixel at %d,%d is %v, want transparent", x, y, got)
				}
			}
		}
	}
	if in == 0 || out == 0 {
		t.Fatalf("%d pixels are inside and %d are outside, want some of each", in, out)
	}
}

// side returns 1 when v is in lo..=hi, 0 when it is not, and -1 when it is
// within 0.01 of a limit.
func side(v, lo, hi float64) int {
	switch {
	case math.Abs(v-lo) < 0.01 || math.Abs(v-hi) < 0.01:
		return -1
	case v >= lo && v <= hi:
		return 1
	}
	return 0
}

func TestGradientsDecalLinear(t *testing.T) {
	for i, stops := range decalStops {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			decal, pad := fillDecal(t, func(mode okage.SpreadMode) (okage.Shader, bool) {
				return okage.NewLinearGradient(okage.PointFromXY(50, 0), okage.PointFromXY(150, 0), stops, mode, okage.IdentityTransform())
			})
			checkDecal(t, decal, pad, func(x, _ float64) int { return side(x, 50, 150) })
		})
	}
}

func TestGradientsDecalLinearTransform(t *testing.T) {
	ts := okage.TransformFromRotateAt(30, 100, 100)
	decal, pad := fillDecal(t, func(mode okage.SpreadMode) (okage.Shader, bool) {
		return okage.NewLinearGradient(okage.PointFromXY(50, 100), okage.PointFromXY(150, 100), decalStops[0], mode, ts)
	})
	inv := must(ts.Invert())
	checkDecal(t, decal, pad, func(x, y float64) int {
		u := float64(inv.SX)*x + float64(inv.KX)*y + float64(inv.TX)
		return side(u, 50, 150)
	})
}

func TestGradientsDecalRadial(t *testing.T) {
	for i, stops := range decalStops {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			decal, pad := fillDecal(t, func(mode okage.SpreadMode) (okage.Shader, bool) {
				return okage.NewRadialGradient(okage.PointFromXY(100, 100), 0, okage.PointFromXY(100, 100), 60, stops, mode, okage.IdentityTransform())
			})
			checkDecal(t, decal, pad, func(x, y float64) int { return side(math.Hypot(x-100, y-100), 0, 60) })
		})
	}
}

func TestGradientsDecalRadialConcentric(t *testing.T) {
	decal, pad := fillDecal(t, func(mode okage.SpreadMode) (okage.Shader, bool) {
		return okage.NewRadialGradient(okage.PointFromXY(100, 100), 30, okage.PointFromXY(100, 100), 80, decalStops[1], mode, okage.IdentityTransform())
	})
	checkDecal(t, decal, pad, func(x, y float64) int { return side(math.Hypot(x-100, y-100), 30, 80) })
}

func TestGradientsDecalTwoPointConical(t *testing.T) {
	// The focal point is inside the end circle, so t is in 0..=1 inside the
	// end circle and greater than 1 outside it.
	decal, pad := fillDecal(t, func(mode okage.SpreadMode) (okage.Shader, bool) {
		return okage.NewRadialGradient(okage.PointFromXY(80, 90), 0, okage.PointFromXY(100, 100), 70, decalStops[0], mode, okage.IdentityTransform())
	})
	checkDecal(t, decal, pad, func(x, y float64) int { return side(math.Hypot(x-100, y-100), -1, 70) })
}

func TestGradientsDecalSweep(t *testing.T) {
	for i, stops := range decalStops {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			decal, pad := fillDecal(t, func(mode okage.SpreadMode) (okage.Shader, bool) {
				return okage.NewSweepGradient(okage.PointFromXY(100, 100), 90, 270, stops, mode, okage.IdentityTransform())
			})
			checkDecal(t, decal, pad, func(x, y float64) int {
				angle := math.Atan2(y-100, x-100) * 180 / math.Pi
				if angle < 0 {
					angle += 360
				}
				return side(angle, 90, 270)
			})
		})
	}
}

// TestGradientsDecalDegenerate makes a linear gradient whose start and end
// are the same point. In the decal mode, it is transparent.
func TestGradientsDecalDegenerate(t *testing.T) {
	shader := must(okage.NewLinearGradient(okage.PointFromXY(50, 50), okage.PointFromXY(50, 50), decalStops[0], okage.SpreadModeDecal, okage.IdentityTransform()))
	if shader != okage.SolidColor(okage.ColorTransparent) {
		t.Errorf("got %v, want a transparent SolidColor", shader)
	}
}

func TestSpreadModeDecalString(t *testing.T) {
	if got := okage.SpreadModeDecal.String(); got != "Decal" {
		t.Errorf("got %q, want %q", got, "Decal")
	}
}
