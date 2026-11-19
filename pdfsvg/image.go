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

// This file ports krilla-svg src/image.rs.

// renderImage is render in krilla-svg src/image.rs. It takes the suffix of
// its module, because other modules have a function of the same name (D10).
//
// Render an image into a surface.
//
// Returns false if the image could not be rendered.
func renderImage(image *svg.Image, surface *pdf.Surface, pc *processContext) bool {
	if !image.IsVisible() {
		return true
	}

	// The size of an image of usvg is always valid, where upstream unwraps
	// it.
	size, _ := path.SizeFromWH(image.Size().Width(), image.Size().Height())

	switch kind := image.Kind().(type) {
	case svg.ImageKindJPEG:
		img, err := pdf.ImageFromJPEG(kind, false)
		if err != nil {
			return false
		}
		surface.DrawImage(img, size)
	case svg.ImageKindPNG:
		img, err := pdf.ImageFromPNG(kind, false)
		if err != nil {
			return false
		}
		surface.DrawImage(img, size)
	case svg.ImageKindGIF:
		img, err := pdf.ImageFromGIF(kind, false)
		if err != nil {
			return false
		}
		surface.DrawImage(img, size)
	case svg.ImageKindWEBP:
		img, err := pdf.ImageFromWebP(kind, false)
		if err != nil {
			return false
		}
		surface.DrawImage(img, size)
	case svg.ImageKindSVG:
		r, ok := path.RectFromXYWH(0.0, 0.0, kind.Tree.Size().Width(), kind.Tree.Size().Height())
		if !ok {
			return false
		}
		surface.PushClipPath(rectToClipPath(r), pdf.FillRuleNonZero)
		renderRoot(kind.Tree.Root(), surface, pc)
		surface.Pop()
	}

	return true
}
