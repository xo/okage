// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfsvg

import (
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
	"github.com/xo/okage/svg"
)

// This file ports krilla-svg src/filter.rs. It is the raster fallback of
// D62, item 10, in pdfsvg: the filters of SVG, and the groups that need the
// blend mode PlusLighter, which group.go gives it.

// renderFilter is render in krilla-svg src/filter.rs. It takes the suffix
// of its module, because other modules have a function of the same name
// (D10). It draws the group with svg, where upstream draws it with resvg.
//
// Render a filter into a surface by rasterizing it with svg and drawing the
// image.
//
// Returns false if converting the filter was unsuccessful.
func renderFilter(group *svg.Group, surface *pdf.Surface, pc *processContext) bool { //nolint:unparam // upstream returns Option<()>
	// okage: the layer bounding box of a group of usvg 0.47 is not
	// optional. The tree of svg has none for a group with no content, which
	// draws nothing (D62).
	groupLayerBBox, ok := group.LayerBoundingBox()
	if !ok {
		return false
	}
	layerBBox, ok := groupLayerBBox.Transform(group.Transform())
	if !ok {
		return false
	}

	// Find out what dimensions the SVG will actually have in user space
	// units inside of the PDF.
	actualCTM, ok := groupLayerBBox.Transform(surface.CTM())
	if !ok {
		return false
	}
	actualBBox, ok := actualCTM.Transform(group.Transform())
	if !ok {
		return false
	}
	// Calculate the necessary scale in the x/y direction, and take the
	// maximum of that.
	scale := maxF32(
		actualBBox.Width()/layerBBox.Width(),
		actualBBox.Height()/layerBBox.Height(),
	) * pc.svgSettings.filterScale

	// Let's try to avoid generating images that have more than 5000 pixels
	// in either direction.
	const pixelThreshold float32 = 5000.0
	// Take the minimum of that.
	maxScale := minF32(
		pixelThreshold/layerBBox.Width(),
		pixelThreshold/layerBBox.Height(),
	)

	// Take whichever is smaller.
	rasterScale := minF32(scale, maxScale)

	pixmapSize, ok := path.SizeFromWH(
		layerBBox.Width()*rasterScale,
		layerBBox.Height()*rasterScale,
	)
	if !ok {
		return false
	}
	width := wide.TruncU32(roundF32(pixmapSize.Width()))
	height := wide.TruncU32(roundF32(pixmapSize.Height()))

	pixmap, ok := okage.NewPixmap(width, height)
	if !ok {
		return false
	}

	// okage: the absolute layer bounding box is not optional in usvg 0.47
	// either (D62).
	absLayerBBox, ok := group.AbsLayerBoundingBox()
	if !ok {
		return false
	}
	initialTransform := path.TransformFromScale(rasterScale, rasterScale).
		PreConcat(path.TransformFromTranslate(-layerBBox.X(), -layerBBox.Y())).
		// This one is a hack because RenderNode will take the absolute layer
		// bbox into consideration and translate by -layer_bbox.x() and
		// -layer_bbox.y(), but we don't want that, so we inverse it.
		PreConcat(path.TransformFromTranslate(absLayerBBox.X(), absLayerBBox.Y()))

	pm := pixmap.AsMut()
	svg.RenderNode(group, initialTransform, &pm)

	pixels := pixmap.Pixels()
	demultiplied := make([]byte, 0, 4*pixels.Len())
	for i := range pixels.Len() {
		c := pixels.At(i).Demultiply()
		demultiplied = append(demultiplied, c.Red(), c.Green(), c.Blue(), c.Alpha())
	}

	image := pdf.ImageFromRGBA8(demultiplied, width, height)
	size, ok := path.SizeFromWH(layerBBox.Width(), layerBBox.Height())
	if !ok {
		return false
	}

	surface.PushTransform(path.TransformFromTranslate(layerBBox.X(), layerBBox.Y()))
	surface.DrawImage(image, size)
	surface.Pop()

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

// roundF32 is f32::round of Rust, which rounds half away from zero.
func roundF32(f float32) float32 {
	return float32(math.Round(float64(f)))
}
