// Copyright 2022 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"github.com/xo/okage/path"
)

// This file ports usvg src/text/flatten.rs.
//
// Port: the glyphs come from the font cache of usvg_text_cache.go, where
// go-text takes the place of skrifa (D9).

// resolveRenderingMode is resolve_rendering_mode in usvg
// src/text/flatten.rs.
func resolveRenderingMode(text *Text) ShapeRendering {
	switch text.renderingMode {
	case TextRenderingOptimizeSpeed:
		return ShapeRenderingCrispEdges
	case TextRenderingOptimizeLegibility:
		return ShapeRenderingGeometricPrecision
	}
	return ShapeRenderingGeometricPrecision
}

// pushOutlinePaths is push_outline_paths in usvg src/text/flatten.rs.
func pushOutlinePaths(
	span *Span,
	builder **path.PathBuilder,
	newChildren *[]Node,
	renderingMode ShapeRendering,
	absTransform Transform,
) {
	b := *builder
	*builder = path.NewPathBuilder()

	if data, ok := b.Finish(); ok {
		p := newPath(
			"",
			span.Visible,
			cloneFill(span.Fill),
			cloneStroke(span.Stroke),
			span.PaintOrder,
			renderingMode,
			data,
			absTransform,
		)
		*newChildren = append(*newChildren, p)
	}
}

// flatten is flatten in usvg src/text/flatten.rs.
func flatten(text *Text, fonts *fontCache) (*Group, NonZeroRect, bool) {
	var newChildren []Node

	absTransform := text.absTransform
	renderingMode := resolveRenderingMode(text)

	for i := range text.layouted {
		span := &text.layouted[i]
		if span.Overline != nil {
			p := clonePath(span.Overline)
			p.renderingMode = renderingMode
			newChildren = append(newChildren, p)
		}

		if span.Underline != nil {
			p := clonePath(span.Underline)
			p.renderingMode = renderingMode
			newChildren = append(newChildren, p)
		}

		// Instead of always processing each glyph separately, we always collect
		// as many outline glyphs as possible by pushing them into the span_builder
		// and only if we encounter a different glyph, or we reach the very end of the
		// span to we push the actual outline paths into new_children. This way, we don't need
		// to create a new path for every glyph if we have many consecutive glyphs
		// with just outlines (which is the most common case).
		spanBuilder := path.NewPathBuilder()

		variations := withOpticalSizing(
			span.Variations,
			span.FontOpticalSizing,
			span.FontSize.Get(),
		)

		for j := range span.PositionedGlyphs {
			glyph := &span.PositionedGlyphs[j]

			// A (best-effort conversion of a) COLR glyph.
			if tree, ok := fonts.colr(glyph.Font, glyph.ID, variations); ok {
				group := emptyGroup()
				group.transform = glyph.ColrTransform()
				// TODO: Probably need to update abs_transform of children? Same
				// for SVG and bitmap glyphs.
				root := tree.root.cloneGroup()
				group.children = append(group.children, root)

				newChildren = append(newChildren, group)
			} else if node, ok := fonts.svg(glyph.Font, glyph.ID); ok {
				// An SVG glyph. Will return the usvg node containing the glyph descriptions.
				pushOutlinePaths(
					span,
					&spanBuilder,
					&newChildren,
					renderingMode,
					absTransform,
				)

				group := emptyGroup()
				group.transform = glyph.SvgTransform()
				group.children = append(group.children, cloneNode(node))

				newChildren = append(newChildren, group)
			} else if img, ok := fonts.raster(glyph.Font, glyph.ID); ok {
				// A bitmap glyph.
				pushOutlinePaths(
					span,
					&spanBuilder,
					&newChildren,
					renderingMode,
					absTransform,
				)

				var transform Transform
				if img.isSbix {
					transform = glyph.SbixTransform(
						float32(img.x),
						float32(img.y),
						img.bearingX,
						img.bearingY,
						float32(img.pixelsPerEm),
						img.image.size.Height(),
					)
				} else {
					transform = glyph.CbdtTransform(float32(img.x), float32(img.y), float32(img.pixelsPerEm))
				}

				group := emptyGroup()
				group.transform = transform
				image := img.image
				group.children = append(group.children, &image)

				newChildren = append(newChildren, group)
			} else {
				outline, ok := fonts.outline(glyph.Font, glyph.ID, variations)

				if ok {
					if outline, ok := outline.Clone().Transform(glyph.OutlineTransform()); ok {
						spanBuilder.PushPath(outline)
					}
				}
			}
		}

		pushOutlinePaths(
			span,
			&spanBuilder,
			&newChildren,
			renderingMode,
			absTransform,
		)

		if span.LineThrough != nil {
			p := clonePath(span.LineThrough)
			p.renderingMode = renderingMode
			newChildren = append(newChildren, p)
		}
	}

	group := emptyGroup()
	group.id = text.id

	group.children = append(group.children, newChildren...)

	groupStrokeBBox, ok := group.StrokeBoundingBox()
	if !ok {
		return nil, NonZeroRect{}, false
	}
	strokeBBox, ok := groupStrokeBBox.ToNonZeroRect()
	if !ok {
		return nil, NonZeroRect{}, false
	}
	return group, strokeBBox, true
}

// cloneNode is the derived Clone for Node in usvg src/tree/mod.rs: a copy of
// the node, with a copy of each fill and stroke, as cloneGroup makes it.
func cloneNode(node Node) Node {
	g := &Group{children: []Node{node}}
	return g.cloneGroup().children[0]
}
