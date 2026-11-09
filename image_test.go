package okage

import (
	"image"
	"image/color"
	"image/draw"
	"testing"
)

// The tests of this file port no upstream test. They check the APIs of D19.

// TestAsRGBA checks that AsRGBA shares the bytes of the pixmap.
func TestAsRGBA(t *testing.T) {
	p, _ := NewPixmap(3, 2)
	img := p.AsRGBA()
	if img.Bounds() != image.Rect(0, 0, 3, 2) {
		t.Errorf("bounds %v", img.Bounds())
	}
	img.SetRGBA(2, 1, color.RGBA{R: 10, G: 20, B: 30, A: 40})
	if got, _ := p.Pixel(2, 1); got != premultipliedColorU8FromRGBAUnchecked(10, 20, 30, 40) {
		t.Errorf("Pixel(2, 1) = %v", got)
	}
	p.PixelsMut().Set(0, premultipliedColorU8FromRGBAUnchecked(1, 2, 3, 4))
	if got := img.RGBAAt(0, 0); got != (color.RGBA{R: 1, G: 2, B: 3, A: 4}) {
		t.Errorf("RGBAAt(0, 0) = %v", got)
	}
}

// TestPixmapImage checks the methods of image.Image and draw.Image.
func TestPixmapImage(t *testing.T) {
	p, _ := NewPixmap(4, 4)
	if p.ColorModel() != color.RGBAModel {
		t.Error("ColorModel is not color.RGBAModel")
	}
	if p.Bounds() != image.Rect(0, 0, 4, 4) {
		t.Errorf("bounds %v", p.Bounds())
	}

	p.Set(1, 2, color.NRGBA{R: 255, G: 0, B: 0, A: 128})
	if got := p.At(1, 2); got != (color.RGBA{R: 128, G: 0, B: 0, A: 128}) {
		t.Errorf("At(1, 2) = %v", got)
	}
	if got := p.At(-1, 0); got != (color.RGBA{}) {
		t.Errorf("At(-1, 0) = %v", got)
	}
	p.Set(4, 0, color.White)
	p.Set(0, -1, color.White)

	draw.Draw(p, image.Rect(2, 2, 4, 4), image.NewUniform(color.RGBA{R: 1, G: 2, B: 3, A: 255}), image.Point{}, draw.Src)
	if got, _ := p.Pixel(3, 3); got != premultipliedColorU8FromRGBAUnchecked(1, 2, 3, 255) {
		t.Errorf("Pixel(3, 3) = %v", got)
	}
	if got, _ := p.Pixel(1, 1); got != premultipliedColorU8FromRGBAUnchecked(0, 0, 0, 0) {
		t.Errorf("Pixel(1, 1) = %v", got)
	}
}
