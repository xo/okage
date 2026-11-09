package okage

import (
	"image"
	"image/color"
	"image/draw"
)

// This file ports no upstream file. It holds the APIs of D19, which make a
// Pixmap work with the package image of the standard library. The bytes of a
// Pixmap have the layout of an image.RGBA: four bytes for each pixel, in the
// order red, green, blue and alpha, premultiplied by the alpha. So no API of
// this file converts a pixel.

// The interfaces of the package image that a Pixmap implements.
var (
	_ image.Image = (*Pixmap)(nil)
	_ draw.Image  = (*Pixmap)(nil)
)

// AsRGBA returns an *image.RGBA that shares the bytes of the pixmap. A change
// to one is a change to the other. Its bounds start at (0, 0).
func (p *Pixmap) AsRGBA() *image.RGBA {
	return &image.RGBA{
		Pix:    p.data,
		Stride: int(p.Width()) * BytesPerPixel,
		Rect:   image.Rect(0, 0, int(p.Width()), int(p.Height())),
	}
}

// ColorModel satisfies the image.Image interface. It returns
// color.RGBAModel, because the pixels are premultiplied 8-bit RGBA.
func (p *Pixmap) ColorModel() color.Model {
	return color.RGBAModel
}

// Bounds satisfies the image.Image interface. The bounds start at (0, 0).
func (p *Pixmap) Bounds() image.Rectangle {
	return image.Rect(0, 0, int(p.Width()), int(p.Height()))
}

// At satisfies the image.Image interface. It returns the premultiplied color
// of the pixel at (x, y) as a color.RGBA. It returns a transparent color for a
// position outside the bounds.
func (p *Pixmap) At(x, y int) color.Color {
	i, ok := p.pixOffset(x, y)
	if !ok {
		return color.RGBA{}
	}
	s := p.data[i : i+BytesPerPixel : i+BytesPerPixel]
	return color.RGBA{R: s[0], G: s[1], B: s[2], A: s[3]}
}

// Set satisfies the draw.Image interface. It converts c to a premultiplied
// color.RGBA and stores it at (x, y). It does nothing for a position outside
// the bounds.
func (p *Pixmap) Set(x, y int, c color.Color) {
	i, ok := p.pixOffset(x, y)
	if !ok {
		return
	}
	v, _ := color.RGBAModel.Convert(c).(color.RGBA)
	s := p.data[i : i+BytesPerPixel : i+BytesPerPixel]
	s[0] = v.R
	s[1] = v.G
	s[2] = v.B
	s[3] = v.A
}

// pixOffset returns the index of the first byte of the pixel at (x, y), and
// false if the position is outside the bounds.
func (p *Pixmap) pixOffset(x, y int) (int, bool) {
	if x < 0 || y < 0 || x >= int(p.Width()) || y >= int(p.Height()) {
		return 0, false
	}
	return (y*int(p.Width()) + x) * BytesPerPixel, true
}
