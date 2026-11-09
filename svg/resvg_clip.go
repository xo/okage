// Copyright 2019 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// This file ports resvg src/clip.rs.
//
// Port: the function apply of upstream collides with the function apply of
// other modules, so it takes the name of its module, applyClip (D10). Each
// function takes the options of Render last, for the logger of D20. A
// function that returns Option<()> returns a bool, and false is None.

// applyClip is apply in resvg src/clip.rs.
func applyClip(
	clip *ClipPath,
	transform Transform,
	pixmap *okage.Pixmap,
	opt *options,
) {
	clipPixmap := mustPixmap(newPixmap(pixmap.Width(), pixmap.Height(), opt))
	clipPixmap.Fill(okage.ColorBlack)

	m := clipPixmap.AsMut()
	drawChildren(
		clip.Root(),
		okage.BlendModeClear,
		transform.PreConcat(clip.Transform()),
		&m,
		opt,
	)

	if clip, ok := clip.ClipPath(); ok {
		applyClip(clip, transform, pixmap, opt)
	}

	mask := okage.MaskFromPixmap(clipPixmap.AsRef(), okage.MaskTypeAlpha, okage.WithLogger(opt.logger))
	mask.Invert()
	pixmap.ApplyMask(mask)
}

// drawChildren is draw_children in resvg src/clip.rs.
func drawChildren(
	parent *Group,
	mode okage.BlendMode,
	transform Transform,
	pixmap *okage.PixmapMut,
	opt *options,
) {
	for _, child := range parent.Children() {
		switch c := child.(type) {
		case *Path:
			if !c.IsVisible() {
				continue
			}

			// We could use any values here. They will not be used anyway.
			maxBBox, ok := path.IntRectFromXYWH(0, 0, 1, 1)
			if !ok {
				panic("svg: called Option::unwrap() on a None value")
			}
			ctx := &context{
				maxBBox: maxBBox,
				opt:     opt,
			}

			fillPath(c, mode, ctx, transform, pixmap)
		case *Text:
			drawChildren(c.Flattened(), mode, transform, pixmap, opt)
		case *Group:
			transform := transform.PreConcat(c.Transform())

			if clip, ok := c.ClipPath(); ok {
				// If a `clipPath` child also has a `clip-path`
				// then we should render this child on a new canvas,
				// clip it, and only then draw it to the `clipPath`.
				clipGroup(c, clip, transform, pixmap, opt)
			} else {
				drawChildren(c, mode, transform, pixmap, opt)
			}
		}
	}
}

// clipGroup is clip_group in resvg src/clip.rs.
func clipGroup(
	children *Group,
	clip *ClipPath,
	transform Transform,
	pixmap *okage.PixmapMut,
	opt *options,
) bool {
	clipPixmap := mustPixmap(newPixmap(pixmap.Width(), pixmap.Height(), opt))

	m := clipPixmap.AsMut()
	drawChildren(
		children,
		okage.BlendModeSourceOver,
		transform,
		&m,
		opt,
	)
	applyClip(clip, transform, clipPixmap, opt)

	paint := okage.DefaultPixmapPaint()
	paint.BlendMode = okage.BlendModeXor
	pixmap.DrawPixmap(
		0,
		0,
		clipPixmap.AsRef(),
		&paint,
		path.IdentityTransform(),
		nil,
	)

	return true
}
