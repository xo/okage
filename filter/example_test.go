// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package filter_test

import (
	"fmt"
	"image"
	"image/color"

	"github.com/xo/okage"
	"github.com/xo/okage/filter"
)

// This example draws a soft shadow: it fills the silhouette of a shape into
// a mask, blurs the mask, and fills a pixmap with a color through the mask.
func ExampleBlurMask() {
	rect, _ := okage.RectFromXYWH(20, 20, 40, 30)
	shape := okage.PathFromRect(rect)
	mask, _ := okage.NewMask(80, 70)
	mask.FillPath(shape, okage.FillRuleWinding, true, okage.IdentityTransform())

	// A standard deviation of 4 pixels takes the box blur of resvg.
	filter.BlurMask(mask, 4, 4)

	pixmap, _ := okage.NewPixmap(80, 70)
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(0, 0, 0, 128)
	all, _ := okage.RectFromXYWH(0, 0, 80, 70)
	pixmap.FillRect(all, &paint, okage.IdentityTransform(), mask)

	row := mask.Data()[35*80:]
	fmt.Println(row[10], row[16], row[20], row[24], row[40])
	// Output:
	// 4 57 139 214 255
}

// This example blurs a pixmap with a small standard deviation, which takes
// the IIR blur of resvg.
func ExampleBlur() {
	pixmap, _ := okage.NewPixmap(30, 30)
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(200, 50, 50, 255)
	square, _ := okage.RectFromXYWH(10, 10, 10, 10)
	pixmap.FillRect(square, &paint, okage.IdentityTransform(), nil)

	filter.Blur(pixmap, 1.5, 1.5)

	for _, x := range []uint32{8, 10, 15} {
		c, _ := pixmap.Pixel(x, 15)
		fmt.Println(c.Red(), c.Green(), c.Blue(), c.Alpha())
	}
	// Output:
	// 26 6 6 34
	// 131 32 32 168
	// 197 49 49 252
}

// This example makes a picture gray and then tints it with two colors, as a
// duotone does. Each row of a matrix gives a channel from the channels R, G,
// B and A, from 0 to 1, and a constant.
func ExampleApplyColorMatrixNRGBA() {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 128, B: 0, A: 255})
	img.SetNRGBA(1, 0, color.NRGBA{R: 30, G: 60, B: 200, A: 128})

	// The luminance with the weights of ivy, in each channel.
	gray := filter.NewColorMatrix([20]float32{
		0.299, 0.587, 0.114, 0, 0,
		0.299, 0.587, 0.114, 0, 0,
		0.299, 0.587, 0.114, 0, 0,
		0, 0, 0, 1, 0,
	})
	filter.ApplyColorMatrixNRGBA(img, gray)
	fmt.Printf("%v\n", img.Pix)

	// A duotone from dark blue (0, 0, 0.4) to cream (1, 0.95, 0.8) by the
	// gray in the channel R.
	duotone := filter.NewColorMatrix([20]float32{
		1, 0, 0, 0, 0,
		0.95, 0, 0, 0, 0,
		0.4, 0, 0, 0, 0.4,
		0, 0, 0, 1, 0,
	})
	filter.ApplyColorMatrixNRGBA(img, duotone)
	fmt.Printf("%v\n", img.Pix)
	// Output:
	// [151 151 151 255 66 66 66 128]
	// [151 143 162 255 66 62 128 128]
}

// This example changes the contrast and the brightness of a picture with a
// linear transfer function, and then makes it black and white. A discrete
// function of 256 values maps each byte of a channel to its own value, so it
// can put the threshold of the black and white at any byte.
func ExampleApplyComponentTransferNRGBA() {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	copy(img.Pix, []byte{40, 40, 40, 255, 100, 100, 100, 255, 200, 200, 200, 255})

	// A contrast factor of 1.5 around the middle gray, and a brightness of
	// 0.1: 1.5 * (c - 0.5) + 0.5 + 0.1.
	lum := filter.TransferLinear(1.5, -0.15)
	filter.ApplyComponentTransferNRGBA(img, lum, lum, lum, filter.TransferFunction{})
	fmt.Printf("%v\n", img.Pix)

	// White from the byte 120 up.
	values := make([]float32, 256)
	for i := 120; i < 256; i++ {
		values[i] = 1
	}
	biLevel := filter.TransferDiscrete(values)
	filter.ApplyComponentTransferNRGBA(img, biLevel, biLevel, biLevel, filter.TransferFunction{})
	fmt.Printf("%v\n", img.Pix)
	// Output:
	// [21 21 21 255 111 111 111 255 255 255 255 255]
	// [0 0 0 255 0 0 0 255 255 255 255 255]
}
