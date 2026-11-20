// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports no file of velato. It draws the Gaussian blur and the drop
// shadow of a layer in the PixmapSink, as ThorVG draws them in
// src/renderer/cpu_engine/tvgSwPostEffect.cpp, with the box blur of the
// filters of svg (D79).

import (
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/boxblur"
)

// thorvgBlurLevels is the number of box blurs that ThorVG applies at the
// quality 50, its default: int(3 * (50 - 1) / 100) + 1.
const thorvgBlurLevels = 2

// thorvgBlur returns the size of the blur of ThorVG for the standard
// deviation sigma in pixels: the standard deviation of its box blurs
// together, and the number of pixels that they spread a pixel by. Both are 0
// when ThorVG does not blur. ThorVG computes the sizes of three boxes, as
// _gaussianInit does, and applies the first thorvgBlurLevels of them. The
// variance of a box of the radius r is ((2r+1)^2 - 1) / 12, and the variances
// of the boxes add up.
func thorvgBlur(sigma float64) (float64, int) {
	const maxLevel = 3
	v := float32(sigma * sigma)
	if !(v > 1.1920929e-07) {
		return 0, 0
	}
	wl := int(float32(math.Sqrt(float64(float32(12*v)/maxLevel + 1))))
	if wl%2 == 0 {
		wl--
	}
	wu := wl + 2
	mi := (float32(12*v) - float32(maxLevel*wl*wl) - float32(4*maxLevel*wl) - 3*maxLevel) / float32(-4*wl-4)
	m := int(mi + 0.5)
	extends, variance := 0, 0.0
	for i := range thorvgBlurLevels {
		w := wu
		if i < m {
			w = wl
		}
		r := (w - 1) / 2
		extends += r
		variance += float64((2*r+1)*(2*r+1)-1) / 12
	}
	if extends == 0 {
		return 0, 0
	}
	return math.Sqrt(variance), extends
}

// applyFilter applies the effect of the filter layer l to its pixels, and
// returns the pixels that the result can touch.
func (s *PixmapSink) applyFilter(l *sinkLayer) pixRect {
	f := l.filter
	t := l.transform
	// ThorVG scales an effect by the length of the first row of the
	// transform.
	scale := math.Sqrt(float64(t[0]*t[0]) + float64(t[2]*t[2]))
	switch f.Kind {
	case EffectGaussianBlur:
		sx, ex := thorvgBlur(f.StdDeviation.X * scale)
		sy, ey := thorvgBlur(f.StdDeviation.Y * scale)
		if ex == 0 && ey == 0 {
			return l.dirty
		}
		region := s.grow(l.dirty, blurPad(sx), blurPad(sy))
		buf := copyRegion(l.pixmap, region)
		boxblur.Apply(sx, sy, buf, uint32(region.x1-region.x0), uint32(region.y1-region.y0))
		pasteRegion(l.pixmap, region, buf)
		return region
	case EffectDropShadow:
		return s.dropShadow(l, scale)
	}
	applyColorFilter(l.pixmap, f, l.dirty)
	return l.dirty
}

// blurPad returns the pixels that the box blur of svg spreads a pixel by,
// for the standard deviation sigma. Its five boxes spread a pixel by less
// than 4 sigma.
func blurPad(sigma float64) int {
	if sigma == 0 {
		return 0
	}
	return int(math.Ceil(4*sigma)) + 2
}

// grow returns r grown by dx and dy pixels on each side, within the canvas.
func (s *PixmapSink) grow(r pixRect, dx, dy int) pixRect {
	return pixRect{r.x0 - dx, r.y0 - dy, r.x1 + dx, r.y1 + dy}.intersect(s.canvas())
}

// copyRegion returns the RGBA bytes of the pixels of p in r.
func copyRegion(p *okage.Pixmap, r pixRect) []byte {
	w := r.x1 - r.x0
	data := p.Data()
	stride := int(p.Width()) * 4
	buf := make([]byte, w*(r.y1-r.y0)*4)
	for y := r.y0; y < r.y1; y++ {
		copy(buf[(y-r.y0)*w*4:], data[y*stride+r.x0*4:y*stride+r.x1*4])
	}
	return buf
}

// pasteRegion writes the RGBA bytes of buf to the pixels of p in r.
func pasteRegion(p *okage.Pixmap, r pixRect, buf []byte) {
	w := r.x1 - r.x0
	data := p.DataMut()
	stride := int(p.Width()) * 4
	for y := r.y0; y < r.y1; y++ {
		copy(data[y*stride+r.x0*4:y*stride+r.x1*4], buf[(y-r.y0)*w*4:(y-r.y0+1)*w*4])
	}
}

// dropShadow draws the drop shadow of the filter layer l below its pixels,
// as effectDropShadow of ThorVG does, and returns the pixels that the result
// can touch. The shadow is the alpha of the layer, blurred, in the color and
// the opacity of the effect, and moved by the offset of the effect. ThorVG
// scales the offset by scale, turns it by the angle of the transform, and
// cuts it to whole pixels.
func (s *PixmapSink) dropShadow(l *sinkLayer, scale float64) pixRect {
	f := l.filter
	t := l.transform
	a8 := int(float32(clampF32(f.Color[3])*255) + 1e-3)
	if a8 == 0 || l.dirty.empty() {
		return l.dirty
	}
	var color [3]int
	for i := range color {
		color[i] = int(math.RoundToEven(float64(clampF32(f.Color[i]) * 255)))
	}
	// ThorVG takes the absolute angle of the transform.
	theta := math.Abs(math.Atan2(t[1], t[0]))
	sin, cos := math.Sincos(theta)
	ox := int(scale * (f.Offset.X*cos - f.Offset.Y*sin))
	oy := int(scale * (f.Offset.X*sin + f.Offset.Y*cos))
	sigma, extends := thorvgBlur(f.ShadowStdDeviation * scale)

	// The alpha of the layer, blurred, in the pixels of blurred.
	pad := blurPad(sigma)
	blurred := s.grow(l.dirty, pad, pad)
	bw, bh := blurred.x1-blurred.x0, blurred.y1-blurred.y0
	alpha := copyRegion(l.pixmap, blurred)
	for i := 0; i < len(alpha); i += 4 {
		alpha[i], alpha[i+1], alpha[i+2] = 0, 0, 0
	}
	if extends > 0 {
		boxblur.Apply(sigma, sigma, alpha, uint32(bw), uint32(bh))
	}
	moved := pixRect{blurred.x0 + ox, blurred.y0 + oy, blurred.x1 + ox, blurred.y1 + oy}
	region := l.dirty.union(moved).intersect(s.canvas())
	data := l.pixmap.DataMut()
	stride := int(s.width) * 4
	for y := region.y0; y < region.y1; y++ {
		sy := y - oy - blurred.y0
		for x := region.x0; x < region.x1; x++ {
			sx := x - ox - blurred.x0
			sa := 0
			if sx >= 0 && sy >= 0 && sx < bw && sy < bh {
				sa = div255(int(alpha[(sy*bw+sx)*4+3]) * a8)
			}
			if sa == 0 {
				continue
			}
			i := y*stride + x*4
			k := 255 - int(data[i+3])
			for c := range 3 {
				data[i+c] = uint8(min(255, int(data[i+c])+div255(div255(color[c]*sa)*k)))
			}
			data[i+3] = uint8(min(255, int(data[i+3])+div255(sa*k)))
		}
	}
	return region
}
