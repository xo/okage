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
		if (ex == 0 && ey == 0) || l.dirty.empty() {
			return l.dirty
		}
		// ThorVG blurs the pixels of the layer that the canvas holds, and
		// repeats the pixels at the edge of that region outside it (D82).
		// So the blur reads the pixels around the canvas from a copy that
		// repeats the edge of the canvas.
		outer := pixRect{l.dirty.x0 - blurPad(sx), l.dirty.y0 - blurPad(sy), l.dirty.x1 + blurPad(sx), l.dirty.y1 + blurPad(sy)}
		buf := copyRegionClamped(l.pixmap, outer)
		boxblur.Apply(sx, sy, buf, uint32(outer.x1-outer.x0), uint32(outer.y1-outer.y0))
		region := outer.intersect(s.canvas())
		pasteRegionFrom(l.pixmap, region, buf, outer)
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

// copyRegionClamped returns the RGBA bytes of the pixels of p in r, where r
// can reach outside p. A pixel outside p repeats the nearest pixel at the
// edge of p, as _gaussianEdgeExtend of ThorVG reads it (D82).
func copyRegionClamped(p *okage.Pixmap, r pixRect) []byte {
	w := r.x1 - r.x0
	pw, ph := int(p.Width()), int(p.Height())
	data := p.Data()
	stride := pw * 4
	buf := make([]byte, w*(r.y1-r.y0)*4)
	for y := r.y0; y < r.y1; y++ {
		row := data[min(max(y, 0), ph-1)*stride:]
		out := buf[(y-r.y0)*w*4:]
		x := r.x0
		for ; x < min(0, r.x1); x++ {
			copy(out[(x-r.x0)*4:(x-r.x0+1)*4], row[:4])
		}
		if x0, x1 := max(r.x0, 0), min(r.x1, pw); x0 < x1 {
			copy(out[(x0-r.x0)*4:], row[x0*4:x1*4])
			x = x1
		}
		for x = max(x, pw); x < r.x1; x++ {
			copy(out[(x-r.x0)*4:(x-r.x0+1)*4], row[(pw-1)*4:pw*4])
		}
	}
	return buf
}

// pasteRegionFrom writes the pixels of r from buf, which holds the RGBA
// bytes of the pixels of outer, to the pixels of p. outer holds r.
func pasteRegionFrom(p *okage.Pixmap, r pixRect, buf []byte, outer pixRect) {
	w := outer.x1 - outer.x0
	data := p.DataMut()
	stride := int(p.Width()) * 4
	for y := r.y0; y < r.y1; y++ {
		i := ((y-outer.y0)*w + r.x0 - outer.x0) * 4
		copy(data[y*stride+r.x0*4:y*stride+r.x1*4], buf[i:i+(r.x1-r.x0)*4])
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

	// The alpha of the layer, blurred, in the pixels of blurred. The blur
	// repeats the pixels at the edge of the canvas outside it, as ThorVG
	// does (D82). The shadow moves only the pixels of the canvas.
	pad := blurPad(sigma)
	blurred := pixRect{l.dirty.x0 - pad, l.dirty.y0 - pad, l.dirty.x1 + pad, l.dirty.y1 + pad}
	inside := blurred.intersect(s.canvas())
	bw, bh := blurred.x1-blurred.x0, blurred.y1-blurred.y0
	alpha := copyRegionClamped(l.pixmap, blurred)
	for i := 0; i < len(alpha); i += 4 {
		alpha[i], alpha[i+1], alpha[i+2] = 0, 0, 0
	}
	if extends > 0 {
		boxblur.Apply(sigma, sigma, alpha, uint32(bw), uint32(bh))
	}
	moved := pixRect{inside.x0 + ox, inside.y0 + oy, inside.x1 + ox, inside.y1 + oy}
	region := l.dirty.union(moved).intersect(s.canvas())
	data := l.pixmap.DataMut()
	stride := int(s.width) * 4
	for y := region.y0; y < region.y1; y++ {
		py := y - oy
		for x := region.x0; x < region.x1; x++ {
			px := x - ox
			sa := 0
			if px >= inside.x0 && py >= inside.y0 && px < inside.x1 && py < inside.y1 {
				sa = div255(int(alpha[((py-blurred.y0)*bw+px-blurred.x0)*4+3]) * a8)
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
