// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfsvg

import (
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
	"github.com/xo/okage/svg"
)

// This file ports krilla-svg src/path.rs.

// renderPath is render in krilla-svg src/path.rs. It takes the suffix of
// its module, because other modules have a function of the same name (D10).
//
// Render a path into a surface.
func renderPath(p *svg.Path, surface *pdf.Surface, pc *processContext) {
	if !p.IsVisible() {
		return
	}

	fill, _ := p.Fill()
	stroke, _ := p.Stroke()
	switch p.PaintOrder() {
	case svg.PaintOrderFillAndStroke:
		drawPath(p, fill, stroke, surface, pc)
	case svg.PaintOrderStrokeAndFill:
		drawPath(p, nil, stroke, surface, pc)
		drawPath(p, fill, nil, surface, pc)
	}
}

// drawPath is draw_path in krilla-svg src/path.rs. A nil fill or stroke is
// None.
//
// Render a filled and/or stroked path into a surface.
func drawPath(p *svg.Path, fill *svg.Fill, stroke *svg.Stroke, surface *pdf.Surface, pc *processContext) {
	var f *pdf.Fill
	if fill != nil {
		f = convertFill(fill, surface, pc, path.IdentityTransform())
	}

	var s *pdf.Stroke
	if stroke != nil {
		s = convertStroke(stroke, surface, pc, path.IdentityTransform())
	}

	// Otherwise krilla will fill with black by default.
	if f != nil || s != nil {
		surface.SetFill(f)
		surface.SetStroke(s)

		if kp := pathToKrilla(p); kp != nil {
			surface.DrawPath(kp)
		}
	}
}
