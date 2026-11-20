// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

import (
	"fmt"
	"math"
	"testing"
)

// effectFile returns a Lottie file of 100 by 100 pixels with a red square
// from 30 to 70 in a layer with the effect effect.
func effectFile(effect string) []byte {
	return fmt.Appendf(nil, `{"v": "5.7.0", "fr": 30, "ip": 0, "op": 10, "w": 100, "h": 100, "layers": [
		{"ty": 4, "ind": 1, "ip": 0, "op": 10, "st": 0, "ks": {"p": {"a": 0, "k": [0, 0]}}, "ef": [%s],
		 "shapes": [%s, %s]}
	]}`, effect, rect(50, 50, 40, 40), red)
}

// TestThorVGBlur checks the size of the blur of ThorVG for a few standard
// deviations.
func TestThorVGBlur(t *testing.T) {
	if s, e := thorvgBlur(0); s != 0 || e != 0 {
		t.Errorf("the sigma 0 gives %v and %d", s, e)
	}
	// For the variance 100, the boxes have the width 19 and the radius 9,
	// and two of them have the variance 2 * (19 * 19 - 1) / 12 = 60.
	if s, e := thorvgBlur(10); math.Abs(s-math.Sqrt(60)) > 1e-9 || e != 18 {
		t.Errorf("the sigma 10 gives %v and %d", s, e)
	}
}

// TestGaussianBlurEffect checks the Gaussian blur of a layer: it spreads the
// edge of the square in both directions, or only along x with the
// dimensions 2.
func TestGaussianBlurEffect(t *testing.T) {
	for _, dims := range []int{1, 2} {
		pm := render(t, effectFile(fmt.Sprintf(`{"ty": 29, "ef": [{"v": {"a": 0, "k": 30}}, {"v": {"a": 0, "k": %d}}, {"v": {"a": 0, "k": 0}}]}`, dims)), 0)
		// The center stays almost opaque, the pixels next to the left edge
		// are partly covered on both sides, and a far pixel stays
		// transparent.
		if a := pixel(t, pm, 50, 50)[3]; a < 250 {
			t.Errorf("dimensions %d: the center has the alpha %d", dims, a)
		}
		for _, x := range []uint32{27, 32} {
			if a := pixel(t, pm, x, 50)[3]; a == 0 || a == 255 {
				t.Errorf("dimensions %d: the pixel (%d, 50) has the alpha %d", dims, x, a)
			}
		}
		if a := pixel(t, pm, 5, 50)[3]; a != 0 {
			t.Errorf("dimensions %d: the far pixel has the alpha %d", dims, a)
		}
		above := pixel(t, pm, 50, 27)[3]
		if dims == 1 && (above == 0 || above == 255) {
			t.Errorf("dimensions 1: the pixel above the edge has the alpha %d", above)
		}
		if dims == 2 && above != 0 {
			t.Errorf("dimensions 2: the pixel above the edge has the alpha %d", above)
		}
	}
}

// TestGaussianBlurCanvasEdge checks that the Gaussian blur repeats the
// pixels at the edge of the canvas outside it, as ThorVG does (D82). A
// rectangle that covers the left half of the canvas stays opaque at the left
// edge, and the blur still spreads its right edge.
func TestGaussianBlurCanvasEdge(t *testing.T) {
	data := fmt.Appendf(nil, `{"v": "5.7.0", "fr": 30, "ip": 0, "op": 10, "w": 100, "h": 100, "layers": [
		{"ty": 4, "ind": 1, "ip": 0, "op": 10, "st": 0, "ks": {"p": {"a": 0, "k": [0, 0]}},
		 "ef": [{"ty": 29, "ef": [{"v": {"a": 0, "k": 30}}, {"v": {"a": 0, "k": 1}}, {"v": {"a": 0, "k": 0}}]}],
		 "shapes": [%s, %s]}
	]}`, rect(10, 50, 100, 140), red)
	pm := render(t, data, 0)
	for _, y := range []uint32{0, 50, 99} {
		if a := pixel(t, pm, 0, y)[3]; a < 250 {
			t.Errorf("the pixel (0, %d) has the alpha %d", y, a)
		}
	}
	if a := pixel(t, pm, 60, 50)[3]; a == 0 || a == 255 {
		t.Errorf("the pixel (60, 50) has the alpha %d", a)
	}
}

// TestDropShadowEffect checks the drop shadow of a layer: at the angle 180
// and the distance 15, the shadow lies 15 pixels below the square, in its
// color and its opacity, and the square stays above it.
func TestDropShadowEffect(t *testing.T) {
	pm := render(t, effectFile(`{"ty": 25, "ef": [{"v": {"a": 0, "k": [0, 0, 1, 1]}}, {"v": {"a": 0, "k": 255}},
		{"v": {"a": 0, "k": 180}}, {"v": {"a": 0, "k": 15}}, {"v": {"a": 0, "k": 0}}]}`), 0)
	for _, c := range []struct {
		x, y uint32
		want [4]uint8
	}{
		{50, 50, [4]uint8{255, 0, 0, 255}},
		{50, 80, [4]uint8{0, 0, 255, 255}},
		{50, 90, [4]uint8{0, 0, 0, 0}},
		{50, 25, [4]uint8{0, 0, 0, 0}},
	} {
		if p := pixel(t, pm, c.x, c.y); p != c.want {
			t.Errorf("the pixel (%d, %d) is %v, want %v", c.x, c.y, p, c.want)
		}
	}
	// With a softness and half the opacity, the shadow is lighter and
	// spreads.
	pm = render(t, effectFile(`{"ty": 25, "ef": [{"v": {"a": 0, "k": [0, 0, 1, 1]}}, {"v": {"a": 0, "k": 128}},
		{"v": {"a": 0, "k": 180}}, {"v": {"a": 0, "k": 15}}, {"v": {"a": 0, "k": 20}}]}`), 0)
	if a := pixel(t, pm, 50, 78)[3]; a < 100 || a > 140 {
		t.Errorf("the soft shadow has the alpha %d", a)
	}
	if a := pixel(t, pm, 50, 87)[3]; a == 0 || a >= 128 {
		t.Errorf("the edge of the soft shadow has the alpha %d", a)
	}
}
