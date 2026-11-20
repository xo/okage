// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

import (
	"math"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"
)

// TestThorVGRightAngle checks which turns ThorVG draws without fading the
// edge of an image (D82).
func TestThorVGRightAngle(t *testing.T) {
	for deg, want := range map[float64]bool{0: true, 90: true, 180: true, -90: true, 270: true, 30: false, -125: false, 0.5: false} {
		a := kurbo.AffineScale(3).Mul(kurbo.AffineRotate(deg * math.Pi / 180))
		if got := thorvgRightAngle(a); got != want {
			t.Errorf("the turn of %v degrees gives %v, want %v", deg, got, want)
		}
	}
}

// TestThorVGFeather checks the fade of the edges of an image of 4 by 3
// pixels, with u and v at the middle of an image pixel.
func TestThorVGFeather(t *testing.T) {
	half := thorvgModf(0.5)
	for _, c := range []struct {
		iru, irv, want int
	}{
		{2, 2, 255},
		{1, 2, 255 - half},
		{4, 2, half},
		{2, 1, 255 - half},
		{2, 3, half},
		{4, 3, thorvgMultiply(half, half)},
	} {
		if got := thorvgFeather(c.iru, c.irv, half, half, 4, 3); got != c.want {
			t.Errorf("the image pixel (%d, %d) has the alpha %d, want %d", c.iru, c.irv, got, c.want)
		}
	}
}

// TestDrawTurnedImage checks that an opaque image of 4 by 4 pixels that is
// scaled by 10 and turned by 30 degrees is opaque inside, and half opaque
// half an image pixel inside its edge, as ThorVG draws it (D82). A turn of
// 90 degrees does not draw it this way.
func TestDrawTurnedImage(t *testing.T) {
	img, ok := okage.NewPixmap(4, 4)
	if !ok {
		t.Fatal("no pixmap")
	}
	img.Fill(okage.ColorWhite)
	pm, ok := okage.NewPixmap(100, 100)
	if !ok {
		t.Fatal("no pixmap")
	}
	m := pm.AsMut()
	s := NewPixmapSink(&m)
	turn := func(deg float64) kurbo.Affine {
		return kurbo.AffineTranslate(kurbo.NewVec2(50, 50)).Mul(kurbo.AffineRotate(deg * math.Pi / 180)).Mul(kurbo.AffineScale(10)).Mul(kurbo.AffineTranslate(kurbo.NewVec2(-2, -2)))
	}
	if s.drawTurnedImage(&m, img, turn(90), 1) {
		t.Fatal("a turn of 90 degrees draws a faded edge")
	}
	tr := turn(30)
	if !s.drawTurnedImage(&m, img, tr, 1) {
		t.Fatal("a turn of 30 degrees does not draw")
	}
	inv := tr.Inverse()
	checked := 0
	for y := range uint32(100) {
		for x := range uint32(100) {
			q := inv.MulPoint(kurbo.NewPoint(float64(x+1), float64(y+1)))
			a := pixel(t, pm, x, y)[3]
			switch {
			case q.X > 1.2 && q.X < 2.8 && q.Y > 1.2 && q.Y < 2.8:
				if a != 255 {
					t.Errorf("the inner pixel (%d, %d) has the alpha %d", x, y, a)
				}
				checked++
			case q.X > 0.4 && q.X < 0.6 && q.Y > 1.5 && q.Y < 2.5:
				if a < 90 || a > 165 {
					t.Errorf("the edge pixel (%d, %d) has the alpha %d", x, y, a)
				}
				checked++
			case q.X < -0.1 || q.Y < -0.1 || q.X > 4.1 || q.Y > 4.1:
				if a != 0 {
					t.Errorf("the outer pixel (%d, %d) has the alpha %d", x, y, a)
				}
			}
		}
	}
	if checked < 50 {
		t.Errorf("only %d pixels are checked", checked)
	}
}
