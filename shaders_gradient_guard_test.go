package okage

import (
	"math"
	"testing"
)

// This file ports no upstream test. It checks the guard of
// averageGradientColor, where tiny-skia panics (D68).

func TestAverageGradientColorOutOfOrder(t *testing.T) {
	red := ColorFromRGBA8(255, 0, 0, 255)
	green := ColorFromRGBA8(0, 255, 0, 255)
	blue := ColorFromRGBA8(0, 0, 255, 255)
	nan := float32(math.NaN())
	p := PointFromXY(10, 10)
	for _, c := range []struct {
		name  string
		stops []GradientStop
		want  [3]float32
	}{
		// The positions become 0.9, 0.9 and 0.9: red to 0.9, then green.
		{"out of order", []GradientStop{NewGradientStop(0.9, red), NewGradientStop(0.1, blue), NewGradientStop(0.5, green)},
			[3]float32{0.9, 0.1, 0}},
		// A position that is NaN is 0, so the positions become 0.6 and
		// 0.6.
		{"NaN", []GradientStop{NewGradientStop(0.6, red), NewGradientStop(nan, blue)},
			[3]float32{0.6, 0, 0.4}},
		// Stops in order keep the result of tiny-skia: the mean of red and
		// blue.
		{"in order", []GradientStop{NewGradientStop(0, red), NewGradientStop(1, blue)},
			[3]float32{0.5, 0, 0.5}},
	} {
		for _, mode := range []SpreadMode{SpreadModeRepeat, SpreadModeReflect} {
			s, ok := NewLinearGradient(p, p, c.stops, mode, IdentityTransform())
			sc, isColor := s.(SolidColor)
			if !ok || !isColor {
				t.Errorf("%s, %v: the gradient is %T, %v", c.name, mode, s, ok)
				continue
			}
			got := Color(sc)
			for i, v := range []float32{got.Red(), got.Green(), got.Blue()} {
				if math.Abs(float64(v-c.want[i])) > 1e-6 {
					t.Errorf("%s, %v: the color is %v, want %v", c.name, mode, got, c.want)
					break
				}
			}
			if got.Alpha() != 1 {
				t.Errorf("%s, %v: the alpha is %v", c.name, mode, got.Alpha())
			}
		}
	}
}

func TestAverageGradientColorRounding(t *testing.T) {
	// Many stops of white can sum to a little more than 1 in float32. The
	// color stays in 0..=1, where tiny-skia panics.
	var stops []GradientStop
	for i := range 100 {
		stops = append(stops, NewGradientStop(float32(i)/99, ColorWhite))
	}
	c := averageGradientColor(stops)
	if c.Red() < 0.999 || c.Red() > 1 || c.Alpha() > 1 {
		t.Errorf("the color is %v", c)
	}
}
