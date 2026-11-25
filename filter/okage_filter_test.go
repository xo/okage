// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package filter

import (
	"bytes"
	"image"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// randomShape returns the bytes of an image of width by height pixels with
// bpp bytes for each pixel, which are 0 outside a random rectangle and
// random inside it.
func randomShape(r *rand.Rand, bpp, width, height int) []byte {
	data := make([]byte, width*height*bpp)
	x0, y0 := r.IntN(width), r.IntN(height)
	x1, y1 := x0+1+r.IntN(width-x0), y0+1+r.IntN(height-y0)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			a := byte(r.IntN(256))
			i := (y*width + x) * bpp
			data[i+bpp-1] = a
			for c := range bpp - 1 {
				data[i+c] = byte(r.IntN(int(a) + 1))
			}
		}
	}
	return data
}

// TestBoxBlurRegion makes sure that the blur of the box of the pixels that
// are not 0 gives the bytes of a blur of the whole image.
func TestBoxBlurRegion(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 400 {
		bpp := []int{1, 4}[i%2]
		width, height := 1+r.IntN(90), 1+r.IntN(90)
		sx, sy := r.Float64()*12, r.Float64()*12
		if i%7 == 0 {
			sx = 0
		}
		want := randomShape(r, bpp, width, height)
		got := append([]byte(nil), want...)
		boxBlur(want, bpp, width, height, sx, sy)
		boxBlurRegion(got, bpp, width, height, sx, sy)
		if !bytes.Equal(got, want) {
			t.Fatalf("case %d: bpp %d, %dx%d, sigma %v %v: the bytes differ", i, bpp, width, height, sx, sy)
		}
	}
}

// TestBlurMaskMatchesBlur makes sure that BlurMask gives the alpha of Blur,
// with the box blur and with the IIR blur.
func TestBlurMaskMatchesBlur(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for i := range 300 {
		width, height := 1+r.IntN(70), 1+r.IntN(70)
		sx, sy := r.Float64()*6, r.Float64()*6
		var opts []Option
		if i%3 == 0 {
			opts = append(opts, WithBoxBlur())
		}
		rgba := randomShape(r, 4, width, height)
		alpha := make([]byte, width*height)
		for j := range alpha {
			alpha[j] = rgba[j*4+3]
		}
		size, ok := path.IntSizeFromWH(uint32(width), uint32(height))
		if !ok {
			t.Fatal("size")
		}
		p, ok := okage.PixmapFromVec(rgba, size)
		if !ok {
			t.Fatal("pixmap")
		}
		m, ok := okage.MaskFromVec(alpha, size)
		if !ok {
			t.Fatal("mask")
		}
		Blur(p, sx, sy, opts...)
		BlurMask(m, sx, sy, opts...)
		got, data := m.Data(), p.Data()
		for j := range got {
			if got[j] != data[j*4+3] {
				t.Fatalf("case %d: %dx%d, sigma %v %v: pixel %d is %d, want %d", i, width, height, sx, sy, j, got[j], data[j*4+3])
			}
		}
	}
}

// TestBlurOddSigmas makes sure that a sigma that is NaN, infinite, negative
// or huge does not panic, and that a sigma that does not blur leaves the
// pixels.
func TestBlurOddSigmas(t *testing.T) {
	for _, s := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -3, 0, 0.01, 1e300, 3e9} {
		p, ok := okage.NewPixmap(17, 9)
		if !ok {
			t.Fatal("pixmap")
		}
		p.Fill(okage.ColorFromRGBA8(10, 20, 30, 40))
		before := append([]byte(nil), p.Data()...)
		Blur(p, s, s)
		Blur(p, s, s, WithBoxBlur())
		rect, ok := okage.IntRectFromXYWH(2, 2, 5, 5)
		if !ok {
			t.Fatal("rect")
		}
		m, ok := okage.NewRectMask(17, 9, rect)
		if !ok {
			t.Fatal("mask")
		}
		BlurMask(m, s, s)
		if (math.IsNaN(s) || s < 0.05) && !bytes.Equal(before, p.Data()) {
			t.Errorf("sigma %v changed the pixels", s)
		}
	}
	Blur(nil, 3, 3)
	BlurMask(nil, 3, 3)
}

// TestNRGBAMatchesPixmap makes sure that the functions on an image.NRGBA
// give the bytes of the functions on a pixmap where each pixel is opaque.
func TestNRGBAMatchesPixmap(t *testing.T) {
	matrices := []ColorMatrix{
		{},
		NewColorMatrix([20]float32{0.2, 0.3, 0.5, 0, 0.1, 0, 1, 0, 0, 0, 0.7, 0, 0.1, 0, -0.1, 0, 0, 0, 1, 0}),
		SaturateMatrix(0.3),
		SaturateMatrix(-1),
		HueRotateMatrix(73),
	}
	funcs := [][4]TransferFunction{
		{},
		{TransferTable([]float32{0, 0.8, 0.2}), TransferDiscrete([]float32{1, 0, 0.5}), TransferLinear(1.5, -0.2), TransferGamma(2, 0.5, 0.1)},
		{TransferTable(nil), TransferDiscrete(nil), TransferLinear(0, 0.4), {}},
	}
	r := rand.New(rand.NewPCG(5, 6))
	const w, h = 13, 7
	src := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range src.Pix {
		src.Pix[i] = byte(r.IntN(256))
		if i%4 == 3 {
			src.Pix[i] = 255
		}
	}
	pixmap := func() *okage.Pixmap {
		size, _ := path.IntSizeFromWH(w, h)
		p, ok := okage.PixmapFromVec(append([]byte(nil), src.Pix...), size)
		if !ok {
			t.Fatal("pixmap")
		}
		return p
	}
	for _, opts := range [][]Option{nil, {WithLinearRGB()}} {
		for i, m := range matrices {
			p := pixmap()
			img := image.NewNRGBA(src.Rect)
			copy(img.Pix, src.Pix)
			ApplyColorMatrix(p, m, opts...)
			ApplyColorMatrixNRGBA(img, m, opts...)
			if !bytes.Equal(p.Data(), img.Pix) {
				t.Errorf("matrix %d, %d options: the bytes differ", i, len(opts))
			}
		}
		for i, f := range funcs {
			p := pixmap()
			img := image.NewNRGBA(src.Rect)
			copy(img.Pix, src.Pix)
			ApplyComponentTransfer(p, f[0], f[1], f[2], f[3], opts...)
			ApplyComponentTransferNRGBA(img, f[0], f[1], f[2], f[3], opts...)
			if !bytes.Equal(p.Data(), img.Pix) {
				t.Errorf("functions %d, %d options: the bytes differ", i, len(opts))
			}
		}
	}
}

// TestNRGBASubImage makes sure that the functions on an image.NRGBA change
// only the pixels of its rectangle.
func TestNRGBASubImage(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for i := range img.Pix {
		img.Pix[i] = 100
	}
	sub, ok := img.SubImage(image.Rect(2, 3, 5, 6)).(*image.NRGBA)
	if !ok {
		t.Fatal("sub image")
	}
	ApplyColorMatrixNRGBA(sub, LuminanceToAlphaMatrix())
	for y := range 8 {
		for x := range 8 {
			c := img.NRGBAAt(x, y)
			inside := image.Pt(x, y).In(sub.Rect)
			if inside && (c.R != 0 || c.G != 0 || c.B != 0) {
				t.Errorf("pixel %d,%d inside is %v", x, y, c)
			}
			if !inside && c.R != 100 {
				t.Errorf("pixel %d,%d outside is %v", x, y, c)
			}
		}
	}
	ApplyColorMatrixNRGBA(nil, SaturateMatrix(0))
	ApplyComponentTransferNRGBA(nil, TransferFunction{}, TransferFunction{}, TransferFunction{}, TransferFunction{})
}

// TestTransferValuesCopied makes sure that a change of the values that a
// function took does not change the function.
func TestTransferValuesCopied(t *testing.T) {
	values := []float32{0, 1}
	f := TransferTable(values)
	values[0] = 1
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	ApplyComponentTransferNRGBA(img, f, f, f, TransferFunction{})
	if img.Pix[0] != 0 {
		t.Errorf("channel %d, want 0", img.Pix[0])
	}
}
