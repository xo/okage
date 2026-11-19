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
// The file holds the mask as a gray image in a soft mask of the type
// luminosity. Without a fill, the surface fills with black, as DrawPath
// does. The stroke of the surface does not apply. A nil mask records
// ErrPDFImage, which Document.Finish returns. A mask with no pixel draws
// nothing.
func (s *Surface) FillStencil(mask *image.Alpha, size Size, interpolate bool) {
	if mask == nil {
		s.sc.setError(fmt.Errorf("%w: the stencil is nil", okage.ErrPDFImage))
		return
	}
	if mask.Rect.Empty() {
		return
	}
	gray := &image.Gray{Pix: mask.Pix, Stride: mask.Stride, Rect: mask.Rect}
	img, err := ImageFromImage(gray, interpolate)
	if err != nil {
		s.sc.setError(err)
		return
	}
	rect, ok := path.RectFromXYWH(0, 0, size.Width(), size.Height())
	if !ok {
		return
	}

	b := s.StreamBuilder()
	ms := b.Surface()
	ms.DrawImage(img, size)
	ms.Finish()

	fill := DefaultFill()
	if s.fill != nil {
		fill = *s.fill
	}
	s.PushMask(NewMask(b.Finish(), MaskTypeLuminosity))
	s.bd.getMut().drawPath(path.PathFromRect(rect), &fill, nil, s.sc, s.cc)
	s.Pop()
}

// maxRasterPixels is the largest number of pixels on a side of the pixmap of
// DrawRaster, PIXEL_THRESHOLD in krilla-svg src/filter.rs.
const maxRasterPixels = 5000

// DrawRaster draws a part of the surface with okage, and draws the pixmap as
// an image in its place. It is the fallback of D62 item 10, for a part that
// PDF cannot hold, such as a blend mode that PDF does not have. It follows
// render in krilla-svg src/filter.rs, which draws a filter of SVG in this
// way. It is not in krilla (D68).
//
// rect is the area to draw, in the user space of the surface. The pixmap has
// the resolution of rect on the page, as the current transform gives it, in
// pixels for each unit of the page, times scale. A side of the pixmap has at
// most 5000 pixels. draw gets a transparent pixmap and the transform from
// the user space of the surface to the pixels of the pixmap. It draws onto
// the pixmap, and the surface then draws the pixmap as an image into rect.
//
// DrawRaster returns false and draws nothing when rect or scale give no
// pixel.
func (s *Surface) DrawRaster(rect Rect, scale float32, draw func(pm *okage.Pixmap, t Transform)) bool {
	actual, ok := rect.Transform(s.CTM())
	if !ok {
		return false
	}
	// The scale in the x and y directions on the page, and the larger one.
	rasterScale := max(actual.Width()/rect.Width(), actual.Height()/rect.Height()) * scale
	// Do not make a pixmap with more than 5000 pixels on a side.
	rasterScale = min(rasterScale, min(maxRasterPixels/rect.Width(), maxRasterPixels/rect.Height()))
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
