// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfsvg

import (
	"github.com/xo/okage"
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
// okage: Surface.DrawRaster holds the rest of render: the scale, the
// pixmap, the limit of 5000 pixels and the image. The writer uses one code
// for both (D62 item 10, D68).
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

	// okage: the absolute layer bounding box is not optional in usvg 0.47
	// either (D62).
	absLayerBBox, ok := group.AbsLayerBoundingBox()
	if !ok {
		return false
	}

	return surface.DrawRaster(exactRect(layerBBox), pc.svgSettings.filterScale, func(pixmap *okage.Pixmap, t pdf.Transform) {
		initialTransform := t.
			// This one is a hack because RenderNode will take the absolute
			// layer bbox into consideration and translate by
			// -layer_bbox.x() and -layer_bbox.y(), but we don't want that,
			// so we inverse it.
			PreConcat(path.TransformFromTranslate(absLayerBBox.X(), absLayerBBox.Y()))

		pm := pixmap.AsMut()
		svg.RenderNode(group, initialTransform, &pm)
	}, pdf.WithRasterArea(exactRect(actualBBox)))
}

// exactRect gives the Rect with the edges of r. NonZeroRect.ToRect makes the
// right and the bottom edges from the width and the height, which can round
// them to other values.
func exactRect(r path.NonZeroRect) pdf.Rect {
	rect, _ := path.RectFromLTRB(r.Left(), r.Top(), r.Right(), r.Bottom())
	return rect
}
