// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"
	"image"
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// This file ports no file of krilla. It holds the fill through a stencil of 8
// bits and the raster fallback, which the device of ivy needs and krilla does
// not have (D62 items 10 and 11, D68).

// FillStencil fills the rectangle from 0, 0 to size with the fill of the
// surface, through a stencil mask of 8 bits. The pixels of mask cover the
// rectangle as the pixels of an image of DrawImage do, and the alpha of each
// pixel is its coverage. This is the fill of a glyph bitmap or of a stencil
// mask of okage. With interpolate, a viewer can smooth the mask when it
// scales it. It is not in krilla (D68).
//
// When the fill is a color and each pixel of the mask is 0 or 255, the file
// holds the mask as an image mask of 1 bit for each pixel, which the viewer
// fills with the color (D69). Otherwise, the file holds the mask as a gray
// image in a soft mask of the type luminosity. Without a fill, the surface
// fills with black, as DrawPath does. The stroke of the surface does not
// apply. A nil mask records ErrPDFImage, which Document.Finish returns. A
// mask with no pixel draws nothing.
func (s *Surface) FillStencil(mask *image.Alpha, size Size, interpolate bool) {
	if mask == nil {
		s.sc.setError(fmt.Errorf("%w: the stencil is nil", okage.ErrPDFImage))
		return
	}
	if mask.Rect.Empty() {
		return
	}
	rect, ok := path.RectFromXYWH(0, 0, size.Width(), size.Height())
	if !ok {
		return
	}
	fill := DefaultFill()
	if s.fill != nil {
		fill = *s.fill
	}
	if _, ok := fill.Paint.(Color); ok && isBinaryStencil(mask) {
		// The limit of the gray image of the soft mask.
		if err := checkImageBytes(mask.Rect.Dx(), mask.Rect.Dy(), 1); err != nil {
			s.sc.setError(imageError(err))
			return
		}
		s.bd.getMut().drawImageMask(newImageMask(mask, interpolate), size, &fill, s.sc, s.cc)
		return
	}

	gray := &image.Gray{Pix: mask.Pix, Stride: mask.Stride, Rect: mask.Rect}
	img, err := ImageFromImage(gray, interpolate)
	if err != nil {
		s.sc.setError(err)
		return
	}

	b := s.StreamBuilder()
	ms := b.Surface()
	ms.DrawImage(img, size)
	ms.Finish()

	s.PushMask(NewMask(b.Finish(), MaskTypeLuminosity))
	s.bd.getMut().drawPath(path.PathFromRect(rect), &fill, nil, s.sc, s.cc)
	s.Pop()
}

// isBinaryStencil reports whether each pixel of the stencil is 0 or 255.
func isBinaryStencil(mask *image.Alpha) bool {
	w := mask.Rect.Dx()
	for y := range mask.Rect.Dy() {
		i := y * mask.Stride
		for _, a := range mask.Pix[i : i+w] {
			if a != 0 && a != 0xff {
				return false
			}
		}
	}
	return true
}

// imageMask is an image mask of 1 bit for each pixel. A bit of 0 marks a
// pixel that the viewer fills with the current color, and a bit of 1 a pixel
// that it leaves, as the default Decode array [0 1] of an image mask says.
// Each row starts at a byte.
type imageMask struct {
	width, height int
	bits          []byte
	interpolate   bool
}

// newImageMask makes the image mask of a stencil of 0 and 255. A pixel of
// 255 is filled.
func newImageMask(mask *image.Alpha, interpolate bool) imageMask {
	w, h := mask.Rect.Dx(), mask.Rect.Dy()
	row := (w + 7) / 8
	bits := make([]byte, row*h)
	for y := range h {
		src := mask.Pix[y*mask.Stride:]
		dst := bits[y*row:]
		for x := range w {
			if src[x] == 0 {
				dst[x/8] |= 0x80 >> (x % 8)
			}
		}
	}
	return imageMask{width: w, height: h, bits: bits, interpolate: interpolate}
}

// typeID names the type for sipHash.
func (imageMask) typeID() string { return "imageMask" }

// hash writes the size, the flag and the bits of the mask.
func (m imageMask) hash(h *hasher) {
	h.usize(m.width)
	h.usize(m.height)
	h.bool(m.interpolate)
	h.bytes(m.bits)
}

// serialize writes the mask as an image XObject with ImageMask true.
func (m imageMask) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	if m.interpolate {
		sc.registerValidationError(validationError{kind: validationImageInterpolation, location: sc.location})
	}
	chunk := sc.newChunk()
	stream := newFilterStreamAutoCompressed(m.bits).finish(sc.serializeSettings())
	x := chunk.ImageXObject(rootRef, stream.encodedData())
	stream.writeFilters(x.Dict)
	x.Width(int32(m.width))
	x.Height(int32(m.height))
	x.ImageMask(true)
	x.BitsPerComponent(1)
	if m.interpolate {
		x.Interpolate(true)
	}
	x.Finish()
	cc.streams.images = append(cc.streams.images, chunkResult{chunk: chunk})
}

// newResource gives the XObject resource of the mask.
func (imageMask) newResource(ref pdfwriter.Ref) xObjectResource {
	return xObjectResource{ref}
}

// drawImageMask fills an image mask with the color of fill, and its opacity,
// in the rectangle from 0, 0 to size. It places the mask as drawImage places
// an image.
func (b *contentBuilder) drawImageMask(m imageMask, size Size, fill *Fill, sc *serializeContext, cc *chunkContainer) {
	unit, _ := path.RectFromXYWH(0, 0, 1, 1)
	b.applyIsolatedOp(
		func(sb *contentBuilder) {
			sb.setFillOpacity(fill.Opacity)
			// Scale the mask from 1x1 to the actual dimensions.
			transform := path.TransformFromRow(size.Width(), 0, 0, -size.Height(), 0, size.Height())
			sb.concatTransform(transform)
			sb.expandBBox(unit)
		},
		func(sb *contentBuilder, sc *serializeContext, cc *chunkContainer) {
			sb.contentSetFillProperties(unit, fill, sc, cc)
			name := sb.rdBuilder.registerResource(registerResourceable(sc, cc, m))
			sb.content.XObject(pdfwriter.Name(name))
		},
		sc,
		cc,
	)
}

// maxRasterPixels is the largest number of pixels on a side of the pixmap of
// DrawRaster, PIXEL_THRESHOLD in krilla-svg src/filter.rs.
const maxRasterPixels = 5000

// RasterOption is an option of Surface.DrawRaster (D20).
type RasterOption func(*rasterOptions)

// rasterOptions holds the options of DrawRaster.
type rasterOptions struct {
	area    Rect
	hasArea bool
}

// WithRasterArea gives DrawRaster the area of its rectangle on the page,
// from which it takes the resolution of the pixmap. Without it, the area is
// the rectangle under the current transform. pdfsvg gives the area that
// render of krilla-svg src/filter.rs computes: the bounding box of a group
// under the current transform, and then under the transform of the group.
func WithRasterArea(area Rect) RasterOption {
	return func(o *rasterOptions) {
		o.area, o.hasArea = area, true
	}
}

// DrawRaster draws a part of the surface with okage, and draws the pixmap as
// an image in its place. It is the fallback of D62 item 10, for a part that
// PDF cannot hold, such as a blend mode that PDF does not have. It is render
// in krilla-svg src/filter.rs, which draws a filter of SVG in this way, and
// pdfsvg draws its filters with it. It is not in krilla (D68).
//
// rect is the area to draw, in the user space of the surface. The pixmap has
// the resolution of rect on the page, as the current transform gives it, in
// pixels for each unit of the page, times scale. WithRasterArea gives the
// area on the page in place of the current transform. A side of the pixmap
// has at most 5000 pixels. draw gets a transparent pixmap and the transform
// from the user space of the surface to the pixels of the pixmap. It draws
// onto the pixmap, and the surface then draws the pixmap as an image into
// rect.
//
// The scale and the limit of 5000 pixels compare as f32::min and f32::max of
// Rust, which take the other value for a NaN, so a scale that is NaN gives
// the largest pixmap, as in krilla-svg. DrawRaster returns false and draws
// nothing when rect or scale give no pixel.
func (s *Surface) DrawRaster(rect Rect, scale float32, draw func(pm *okage.Pixmap, t Transform), opts ...RasterOption) bool {
	var o rasterOptions
	for _, opt := range opts {
		opt(&o)
	}
	actual := o.area
	if !o.hasArea {
		var ok bool
		if actual, ok = rect.Transform(s.CTM()); !ok {
			return false
		}
	}
	// Calculate the necessary scale in the x/y direction, and take the
	// maximum of that.
	rasterScale := maxF32(actual.Width()/rect.Width(), actual.Height()/rect.Height()) * scale
	// Let's try to avoid generating images that have more than 5000 pixels
	// in either direction. Take whichever is smaller.
	rasterScale = minF32(rasterScale, minF32(maxRasterPixels/rect.Width(), maxRasterPixels/rect.Height()))
	size, ok := path.SizeFromWH(rect.Width()*rasterScale, rect.Height()*rasterScale)
	if !ok {
		return false
	}
	w := math.Round(float64(size.Width()))
	h := math.Round(float64(size.Height()))
	if !(w >= 1 && h >= 1 && w <= maxRasterPixels+1 && h <= maxRasterPixels+1) {
		return false
	}
	pm, ok := okage.NewPixmap(uint32(w), uint32(h))
	if !ok {
		return false
	}
	t := path.TransformFromScale(rasterScale, rasterScale).
		PreConcat(path.TransformFromTranslate(-rect.X(), -rect.Y()))
	draw(pm, t)

	pixels := pm.Pixels()
	data := make([]byte, 0, 4*pixels.Len())
	for i := range pixels.Len() {
		c := pixels.At(i).Demultiply()
		data = append(data, c.Red(), c.Green(), c.Blue(), c.Alpha())
	}
	img := ImageFromRGBA8(data, pm.Width(), pm.Height())
	imageSize, ok := path.SizeFromWH(rect.Width(), rect.Height())
	if !ok {
		return false
	}
	s.PushTransform(path.TransformFromTranslate(rect.X(), rect.Y()))
	s.DrawImage(img, imageSize)
	s.Pop()
	return true
}

// minF32 is f32::min of Rust, which returns the other value for a NaN.
func minF32(a, b float32) float32 {
	switch {
	case a != a:
		return b
	case b != b:
		return a
	}
	return min(a, b)
}

// maxF32 is f32::max of Rust, which returns the other value for a NaN.
func maxF32(a, b float32) float32 {
	switch {
	case a != a:
		return b
	case b != b:
		return a
	}
	return max(a, b)
}
