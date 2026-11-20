// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports no file of velato. It draws an image asset that turns by an
// angle that is not a multiple of 90 degrees, as rasterTexmapPolygon of
// ThorVG draws it in src/renderer/cpu_engine/tvgSwRasterTexmap.h (D82).

import (
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"
)

// thorvgAtan2 is atan2 of src/common/tvgMath.cpp of ThorVG, a polynomial
// approximation in float32.
func thorvgAtan2(y, x float32) float32 {
	if y == 0 && x == 0 {
		return 0
	}
	ax, ay := float32(math.Abs(float64(x))), float32(math.Abs(float64(y)))
	a := min(ax, ay) / max(ax, ay)
	s := float32(a * a)
	r := float32(float32(float32(float32(float32(float32(-0.0464964749*s)+0.15931422)*s)-0.327622764)*s)*a) + a
	if ay > ax {
		r = 1.57079637 - r
	}
	if x < 0 {
		r = 3.14159274 - r
	}
	if y < 0 {
		return -r
	}
	return r
}

// thorvgRightAngle reports if the transform t turns by a multiple of 90
// degrees, as rightAngle of src/common/tvgMath.h of ThorVG does.
func thorvgRightAngle(t kurbo.Affine) bool {
	const eps = 1e-6
	r := float32(math.Abs(float64(thorvgAtan2(float32(t[1]), float32(t[0])))))
	zero := func(v float32) bool { return math.Abs(float64(v)) <= eps }
	return zero(r) || zero(r-math.Pi/2) || zero(r-math.Pi)
}

// thorvgMultiply is MULTIPLY of src/renderer/tvgRender.h of ThorVG.
func thorvgMultiply(c, a int) int {
	return (c*a + 0xff) >> 8
}

// thorvgFeather is _feathering of src/renderer/cpu_engine/tvgSwRasterTexmap.h
// of ThorVG: the alpha that fades the pixels of the first and the last row
// and column of an image of sw by sh pixels. iru and irv are the column and
// the row of the image pixel plus 1, and ar and ab are 255 minus the
// fraction of u and v in 256ths.
func thorvgFeather(iru, irv, ar, ab, sw, sh int) int {
	switch {
	case irv == 1:
		switch iru {
		case 1:
			return 255 - thorvgMultiply(ar, ab)
		case sw:
			return thorvgMultiply(ar, 255-ab)
		}
		return 255 - ab
	case irv == sh:
		switch iru {
		case 1:
			return thorvgMultiply(255-ar, ab)
		case sw:
			return thorvgMultiply(ar, ab)
		}
		return ab
	case iru == 1:
		return 255 - ar
	case iru == sw:
		return ar
	}
	return 255
}

// thorvgModf is _modf of src/renderer/cpu_engine/tvgSwRasterTexmap.h of
// ThorVG: 255 minus the fraction of v in 256ths.
func thorvgModf(v float32) int {
	return 255 - (int(float32(v*256)) & 255)
}

// drawTurnedImage draws the image p onto target under the transform t, with
// its opacity scaled by alpha, as rasterTexmapPolygon of ThorVG draws an
// image whose transform turns it by an angle that is not a multiple of 90
// degrees (D82). It returns false, and draws nothing, for any other
// transform.
//
// ThorVG draws a pixel when the image holds the bottom-right corner of the
// pixel. It samples the image there with a bilinear filter that puts the
// center of an image pixel at its top-left corner, and it fades the first
// and the last row and column of the image from transparent at the edge of
// the image to opaque one image pixel inside it. The edge of the image has
// no other antialiasing.
func (s *PixmapSink) drawTurnedImage(target *okage.PixmapMut, p *okage.Pixmap, t kurbo.Affine, alpha float64) bool {
	if thorvgRightAngle(t) || !(t.Determinant() != 0) {
		return false
	}
	sw, sh := int(p.Width()), int(p.Height())
	box := s.bounds(t, kurbo.NewRect(0, 0, float64(sw), float64(sh)).PathElements(0.1), 0).intersect(s.canvas())
	if box.empty() {
		return true
	}
	s.touch(box)
	inv := t.Inverse()
	opacity := int(float32(clampF32(float32(alpha))*255) + 0.5)
	src := p.Data()
	dst := target.DataMut()
	stride := int(s.width) * 4
	for y := box.y0; y < box.y1; y++ {
		for x := box.x0; x < box.x1; x++ {
			q := inv.MulPoint(kurbo.NewPoint(float64(x+1), float64(y+1)))
			u, v := float32(q.X), float32(q.Y)
			if !(u >= 0) || !(v >= 0) || u >= float32(sw) || v >= float32(sh) {
				continue
			}
			uu, vv := int(u), int(v)
			ar, ab := thorvgModf(u), thorvgModf(v)
			iru, irv := uu+1, vv+1
			px := texel(src, sw, uu, vv)
			if iru < sw {
				px = interpolate(px, texel(src, sw, iru, vv), ar)
			}
			if irv < sh {
				px2 := texel(src, sw, uu, irv)
				if iru < sw {
					px2 = interpolate(px2, texel(src, sw, iru, irv), ar)
				}
				px = interpolate(px, px2, ab)
			}
			if opacity < 255 {
				px = alphaBlend(px, opacity)
			}
			if f := thorvgFeather(iru, irv, ar, ab, sw, sh); f < 255 {
				px = alphaBlend(px, f)
			}
			i := y*stride + x*4
			d := alphaBlend([4]int{int(dst[i]), int(dst[i+1]), int(dst[i+2]), int(dst[i+3])}, 255-px[3])
			for c := range 4 {
				dst[i+c] = uint8(min(255, px[c]+d[c]))
			}
		}
	}
	return true
}

// texel returns the premultiplied channels of the pixel (x, y) of the RGBA
// bytes data of an image that is w pixels wide.
func texel(data []byte, w, x, y int) [4]int {
	i := (y*w + x) * 4
	return [4]int{int(data[i]), int(data[i+1]), int(data[i+2]), int(data[i+3])}
}

// interpolate is INTERPOLATE of src/renderer/cpu_engine/tvgSwCommon.h of
// ThorVG for each channel: d plus (s minus d) times a over 256.
func interpolate(s, d [4]int, a int) [4]int {
	var out [4]int
	for c := range 4 {
		out[c] = (((s[c]-d[c])*a)>>8 + d[c]) & 0xff
	}
	return out
}

// alphaBlend is ALPHA_BLEND of src/renderer/cpu_engine/tvgSwCommon.h of
// ThorVG for each channel: c times a plus 1, over 256.
func alphaBlend(c [4]int, a int) [4]int {
	var out [4]int
	for i := range 4 {
		out[i] = (c[i] * (a + 1)) >> 8
	}
	return out
}
