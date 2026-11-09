// Copyright 2019 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// This file ports resvg src/mask.rs.
//
// Port: the function apply of upstream collides with the function apply of
// other modules, so it takes the name of its module, applyMask (D10).

// applyMask is apply in resvg src/mask.rs.
func applyMask(
	mask *Mask,
	ctx *context,
	transform Transform,
	pixmap *okage.Pixmap,
) {
	if len(mask.Root().Children()) == 0 {
		pixmap.Fill(okage.ColorTransparent)
		return
	}

	maskPixmap := mustPixmap(newPixmap(pixmap.Width(), pixmap.Height(), ctx.opt))

	{
		// TODO: only when needed
		// Mask has to be clipped by mask.region
		alphaMask, ok := okage.NewMask(pixmap.Width(), pixmap.Height(), okage.WithLogger(ctx.opt.logger))
		if !ok {
			panic("svg: called Option::unwrap() on a None value")
		}
		alphaMask.FillPath(
			path.PathFromRect(mask.Rect().ToRect()),
			okage.FillRuleWinding,
			true,
			transform,
		)

		m := maskPixmap.AsMut()
		renderNodes(mask.Root(), ctx, transform, &m)

		maskPixmap.ApplyMask(alphaMask)
	}

	if mask, ok := mask.Mask(); ok {
		applyMask(mask, ctx, transform, pixmap)
	}

	var maskType okage.MaskType
	switch mask.Kind() {
	case MaskTypeLuminance:
		maskType = okage.MaskTypeLuminance
	case MaskTypeAlpha:
		maskType = okage.MaskTypeAlpha
	}

	m := okage.MaskFromPixmap(maskPixmap.AsRef(), maskType, okage.WithLogger(ctx.opt.logger))
	pixmap.ApplyMask(m)
}
