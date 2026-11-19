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

// This file ports krilla-svg src/mask.rs.

// renderMask is render in krilla-svg src/mask.rs. It takes the suffix of
// its module, because other modules have a function of the same name (D10).
// It returns the number of pushes onto the surface, which the caller pops.
//
// Render a usvg Mask into a surface.
func renderMask(mask *svg.Mask, surface *pdf.Surface, pc *processContext) int {
	streamBuilder := surface.StreamBuilder()
	subSurface := streamBuilder.Surface()
	popCount := 0
	if subMask, ok := mask.Mask(); ok {
		popCount += renderMask(subMask, subSurface, pc)
	}

	// The rectangle of a NonZeroRect is always valid, where upstream
	// unwraps it.
	rect := mask.Rect().ToRect()
	r, _ := path.RectFromLTRB(rect.Left(), rect.Top(), rect.Right(), rect.Bottom())
	clipPath := rectToClipPath(r)

	subSurface.PushClipPath(clipPath, pdf.FillRuleNonZero)
	popCount++
	renderRoot(mask.Root(), subSurface, pc)

	for range popCount {
		subSurface.Pop()
	}

	subSurface.Finish()
	stream := streamBuilder.Finish()

	surface.PushMask(pdf.NewMask(stream, convertMaskType(mask.Kind())))
	return 1
}
