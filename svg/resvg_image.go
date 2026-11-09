// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"

	"golang.org/x/image/webp"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// This file ports resvg src/image.rs, with the feature raster-images.
//
// Port: the function render of upstream collides with the function render of
// other modules, so it takes the name of its module, renderImage (D10). The
// functions of the module raster_images are in this file, after the others.
// Each function that logs takes the options of Render last, for the logger
// of D20. A function that returns Option<()> returns a bool, and false is
// None.
//
// okage ports no crate that decodes an image (D7). image/png, through
// okage.DecodePNG, image/jpeg, image/gif and golang.org/x/image/webp decode
// the images in place of the crates png, zune-jpeg, gif and image-webp. The
// pixels can differ from upstream where a decoder converts colors in another
// way, and D31 measures the difference.

// renderImage is render in resvg src/image.rs.
func renderImage(
	img *Image,
	transform Transform,
	pixmap *okage.PixmapMut,
	opt *options,
) {
	if !img.IsVisible() {
		return
	}

	renderInner(img.Kind(), transform, img.RenderingMode(), pixmap, opt)
}

// renderInner is render_inner in resvg src/image.rs.
func renderInner(
	imageKind ImageKind,
	transform Transform,
	renderingMode ImageRendering,
	pixmap *okage.PixmapMut,
	opt *options,
) {
	switch k := imageKind.(type) {
	case ImageKindSVG:
		renderVector(k.Tree, transform, pixmap, opt)
	default:
		renderRaster(imageKind, transform, renderingMode, pixmap, opt)
	}
}

// renderVector is render_vector in resvg src/image.rs.
func renderVector(
	tree *Tree,
	transform Transform,
	pixmap *okage.PixmapMut,
	opt *options,
) bool {
	subPixmap := mustPixmap(newPixmap(pixmap.Width(), pixmap.Height(), opt))
	m := subPixmap.AsMut()
	Render(tree, transform, &m, WithLogger(opt.logger))
	paint := okage.DefaultPixmapPaint()
	pixmap.DrawPixmap(
		0,
		0,
		subPixmap.AsRef(),
		&paint,
		path.DefaultTransform(),
		nil,
	)

	return true
}

// decodeRaster is raster_images::decode_raster in resvg src/image.rs.
func decodeRaster(img ImageKind, opt *options) (*okage.Pixmap, bool) {
	switch data := img.(type) {
	case ImageKindSVG:
		return nil, false
	case ImageKindJPEG:
		p, ok := decodeJPEG(data, opt)
		return logNone(p, ok, func() { opt.warnf("Failed to decode a JPEG image.") })
	case ImageKindPNG:
		p, ok := decodePNG(data, opt)
		return logNone(p, ok, func() { opt.warnf("Failed to decode a PNG image.") })
	case ImageKindGIF:
		p, ok := decodeGIF(data, opt)
		return logNone(p, ok, func() { opt.warnf("Failed to decode a GIF image.") })
	case ImageKindWEBP:
		p, ok := decodeWEBP(data, opt)
		return logNone(p, ok, func() { opt.warnf("Failed to decode a WebP image.") })
	}
	return nil, false
}

// decodePNG is raster_images::decode_png in resvg src/image.rs.
func decodePNG(data []byte, opt *options) (*okage.Pixmap, bool) {
	p, err := okage.DecodePNG(data, okage.WithLogger(opt.logger))
	if err != nil {
		return nil, false
	}
	return p, true
}

// decodeJPEG is raster_images::decode_jpeg in resvg src/image.rs.
//
// Port: image/jpeg decodes the image in place of zune-jpeg (D7), and
// imageToRGBA converts it to RGBA, which upstream asks zune-jpeg to do.
func decodeJPEG(data []byte, opt *options) (*okage.Pixmap, bool) {
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}

	imgData := imageToRGBA(img)

	b := img.Bounds()
	size, ok := path.IntSizeFromWH(uint32(b.Dx()), uint32(b.Dy()))
	if !ok {
		return nil, false
	}
	return okage.PixmapFromVec(imgData, size, okage.WithLogger(opt.logger))
}

// decodeGIF is raster_images::decode_gif in resvg src/image.rs.
//
// Port: image/gif decodes the first frame in place of the crate gif (D7). The
// frame has the bounds of the first frame, as the buffer of upstream does.
func decodeGIF(data []byte, opt *options) (*okage.Pixmap, bool) {
	firstFrame, err := gif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}

	b := firstFrame.Bounds()
	size, ok := path.IntSizeFromWH(uint32(b.Dx()), uint32(b.Dy()))
	if !ok {
		return nil, false
	}

	w, h := size.Dimensions()
	pixmap, ok := newPixmap(w, h, opt)
	if !ok {
		return nil, false
	}
	rgbaToPixmap(imageToRGBA(firstFrame), pixmap)
	return pixmap, true
}

// decodeWEBP is raster_images::decode_webp in resvg src/image.rs.
//
// Port: golang.org/x/image/webp decodes the image in place of the crate
// image-webp (D7). An image has alpha when the decoder gives a type with an
// alpha channel, as image-webp reports with has_alpha.
func decodeWEBP(data []byte, opt *options) (*okage.Pixmap, bool) {
	img, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}

	b := img.Bounds()
	w, h := uint32(b.Dx()), uint32(b.Dy())
	pixmap, ok := newPixmap(w, h, opt)
	if !ok {
		return nil, false
	}

	firstFrame := imageToRGBA(img)
	if _, ok := img.(*image.YCbCr); !ok {
		rgbaToPixmap(firstFrame, pixmap)
	} else {
		rgb := make([]byte, 0, len(firstFrame)/4*3)
		for i := 0; i+4 <= len(firstFrame); i += 4 {
			rgb = append(rgb, firstFrame[i], firstFrame[i+1], firstFrame[i+2])
		}
		rgbToPixmap(rgb, pixmap)
	}

	return pixmap, true
}

// rgbToPixmap is raster_images::rgb_to_pixmap in resvg src/image.rs.
func rgbToPixmap(data []byte, pixmap *okage.Pixmap) {
	i := 0
	dst := pixmap.DataMut()
	for j := 0; j+3 <= len(data); j += 3 {
		dst[i+0] = data[j+0]
		dst[i+1] = data[j+1]
		dst[i+2] = data[j+2]
		dst[i+3] = 255

		i += okage.BytesPerPixel
	}
}

// rgbaToPixmap is raster_images::rgba_to_pixmap in resvg src/image.rs.
func rgbaToPixmap(data []byte, pixmap *okage.Pixmap) {
	i := 0
	dst := pixmap.DataMut()
	for j := 0; j+4 <= len(data); j += 4 {
		r, g, b, a8 := data[j], data[j+1], data[j+2], data[j+3]
		a := float64(a8) / 255.0
		dst[i+0] = asU8F64(float64(float64(r)*a) + 0.5)
		dst[i+1] = asU8F64(float64(float64(g)*a) + 0.5)
		dst[i+2] = asU8F64(float64(float64(b)*a) + 0.5)
		dst[i+3] = a8

		i += okage.BytesPerPixel
	}
}

// renderRaster is raster_images::render_raster in resvg src/image.rs.
func renderRaster(
	img ImageKind,
	transform Transform,
	renderingMode ImageRendering,
	pixmap *okage.PixmapMut,
	opt *options,
) bool {
	raster, ok := decodeRaster(img, opt)
	if !ok {
		return false
	}

	size, ok := path.SizeFromWH(float32(raster.Width()), float32(raster.Height()))
	if !ok {
		return false
	}
	rect, ok := size.ToRect(0.0, 0.0)
	if !ok {
		return false
	}

	var quality okage.FilterQuality
	switch renderingMode {
	case ImageRenderingOptimizeQuality:
		quality = okage.FilterQualityBicubic
	case ImageRenderingOptimizeSpeed:
		quality = okage.FilterQualityNearest
	case ImageRenderingSmooth:
		quality = okage.FilterQualityBilinear
	case ImageRenderingHighQuality:
		quality = okage.FilterQualityBicubic
	case ImageRenderingCrispEdges:
		quality = okage.FilterQualityNearest
	case ImageRenderingPixelated:
		quality = okage.FilterQualityNearest
	}

	pattern := okage.NewPattern(
		raster.AsRef(),
		okage.SpreadModePad,
		quality,
		1.0,
		path.DefaultTransform(),
	)
	paint := okage.DefaultPaint()
	paint.Shader = pattern

	pixmap.FillRect(rect, &paint, transform, nil)

	return true
}

// imageToRGBA returns the pixels of img as RGBA bytes with straight alpha,
// row by row. It ports no upstream function. It takes the place of the
// output color space RGBA of the decoders of upstream. A YCbCr image, which
// image/jpeg and golang.org/x/image/webp give, converts with
// color.YCbCrToRGB, and every other image with color.NRGBAModel.
func imageToRGBA(img image.Image) []byte {
	b := img.Bounds()
	data := make([]byte, 0, b.Dx()*b.Dy()*4)
	if ycc, ok := img.(*image.YCbCr); ok {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				c := ycc.YCbCrAt(x, y)
				r, g, bl := color.YCbCrToRGB(c.Y, c.Cb, c.Cr)
				data = append(data, r, g, bl, 255)
			}
		}
		return data
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c, _ := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			data = append(data, c.R, c.G, c.B, c.A)
		}
	}
	return data
}

// asU8F64 is the Rust cast "f as u8" for an f64. It truncates toward zero and
// saturates, and NaN gives 0.
func asU8F64(f float64) uint8 {
	switch {
	case f != f || f <= 0:
		return 0
	case f >= 255:
		return 255
	}
	return uint8(f)
}
