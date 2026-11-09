// Copyright 2019 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/parser/use_node.rs.

// convertUseNode is convert in usvg src/parser/use_node.rs.
func convertUseNode(
	node svgNode,
	state *converterState,
	cache *converterCache,
	parent *Group,
) {
	child, ok := node.firstChild()
	if !ok {
		return
	}

	if state.parentClipPath != nil && child.hasTagName(EIdSymbol) {
		// Ignore `symbol` referenced by `use` inside a `clipPath`.
		// It will be ignored later anyway, but this will prevent
		// a redundant `clipPath` creation (which is required for `symbol`).
		return
	}

	useState := state.clone()
	ctx := &contextFillStroke{}
	if f, ok := resolveFill(node, true, state, cache); ok {
		f.contextElement = &contextElement{kind: contextElementUseNode}
		ctx.fill = f
	}
	if s, ok := resolveStroke(node, true, state, cache); ok {
		s.contextElement = &contextElement{kind: contextElementUseNode}
		ctx.stroke = s
	}
	useState.contextElement = ctx

	// We require an original transformation to setup 'clipPath'.
	origTs := node.resolveTransform(AIdTransform, state)
	newTs := path.DefaultTransform()

	{
		x := node.convertUserLength(AIdX, useState, svgtypes.LengthZero())
		y := node.convertUserLength(AIdY, useState, svgtypes.LengthZero())
		newTs = newTs.PreTranslate(x, y)
	}

	linkedToSymbol := child.hasTagName(EIdSymbol)

	if linkedToSymbol {
		// If a `use` element has a width/height attribute and references a symbol
		// then relative units (like percentages) should be resolved relative
		// to the width/height of the `use` element, and not the original SVG.
		// This is why we need to (potentially) adapt the view box here.
		{
			def := svgtypes.NewLength(100.0, svgtypes.LengthUnitPercent)
			x := useState.viewBox.X()
			y := useState.viewBox.Y()

			width := useState.viewBox.Width()
			if node.hasAttribute(AIdWidth) {
				width = node.convertUserLength(AIdWidth, useState, def)
			}

			height := useState.viewBox.Height()
			if node.hasAttribute(AIdHeight) {
				height = node.convertUserLength(AIdHeight, useState, def)
			}

			// Fail silently if the rect is not valid.
			if r, ok := path.NonZeroRectFromXYWH(x, y, width, height); ok {
				useState.viewBox = r
			}
		}

		if ts, ok := viewboxTransform(node, child, useState); ok {
			newTs = newTs.PreConcat(ts)
		}

		if clipRect, ok := getClipRect(node, child, useState); ok {
			g := clipElement(node, clipRect, origTs, useState, cache)
			g.absTransform = parent.absTransform

			// Make group for `use`.
			if g2, ok := convertGroup(node, useState, true, cache, g, func(cache *converterCache, g2 *Group) {
				convertChildrenUseNode(child, newTs, useState, cache, false, g2)
			}); ok {
				// We must reset transform, because it was already set
				// to the group with clip-path.
				g.isContextElement = true
				g2.id = "" // Prevent ID duplication.
				g2.transform = path.DefaultTransform()
				g.children = append(g.children, g2)
			}

			if len(g.children) == 0 {
				return
			}

			g.calculateBoundingBoxes()
			parent.children = append(parent.children, g)
			return
		}
	}

	origTs = origTs.PreConcat(newTs)

	if linkedToSymbol {
		// Make group for `use`.
		if g, ok := convertGroup(node, useState, false, cache, parent, func(cache *converterCache, g *Group) {
			convertChildrenUseNode(child, origTs, useState, cache, false, g)
		}); ok {
			g.isContextElement = true
			g.transform = path.DefaultTransform()
			parent.children = append(parent.children, g)
		}
	} else {
		linkedToSvg := child.hasTagName(EIdSvg)
		if linkedToSvg {
			// When a `use` element references a `svg` element,
			// we have to remember `use` element size and use it
			// instead of `svg` element size.

			def := svgtypes.NewLength(100.0, svgtypes.LengthUnitPercent)
			// As per usual, the SVG spec doesn't clarify this edge case,
			// but it seems like `use` size has to be reset by each `use`.
			// Meaning if we have two nested `use` elements, where one had set `width` and
			// other set `height`, we have to ignore the first `width`.
			//
			// Example:
			// <use id="use1" xlink:href="#use2" width="100"/>
			// <use id="use2" xlink:href="#svg2" height="100"/>
			// <svg id="svg2" x="40" y="40" width="80" height="80" xmlns="http://www.w3.org/2000/svg"/>
			//
			// In this case `svg2` size is 80x100 and not 100x100.
			useState.hasUseWidth, useState.hasUseHeight = false, false

			// Width and height can be set independently.
			if node.hasAttribute(AIdWidth) {
				useState.useWidth = node.convertUserLength(AIdWidth, useState, def)
				useState.hasUseWidth = true
			}
			if node.hasAttribute(AIdHeight) {
				useState.useHeight = node.convertUserLength(AIdHeight, useState, def)
				useState.hasUseHeight = true
			}

			convertChildrenUseNode(node, origTs, useState, cache, true, parent)
		} else {
			convertChildrenUseNode(node, origTs, useState, cache, true, parent)
		}
	}
}

// convertSvg is convert_svg in usvg src/parser/use_node.rs.
func convertSvg(
	node svgNode,
	state *converterState,
	cache *converterCache,
	parent *Group,
) {
	// We require original transformation to setup 'clipPath'.
	origTs := node.resolveTransform(AIdTransform, state)
	newTs := path.DefaultTransform()

	x := node.convertUserLength(AIdX, state, svgtypes.LengthZero())
	y := node.convertUserLength(AIdY, state, svgtypes.LengthZero())
	newTs = newTs.PreTranslate(x, y)

	if ts, ok := viewboxTransform(node, node, state); ok {
		newTs = newTs.PreConcat(ts)
	}

	// We have to create a new state which would have its viewBox set to the current SVG element.
	// Note that we're not updating State::size - it's a completely different property.
	newState := state.clone()
	if vb, ok := node.parseViewbox(); ok {
		newState.viewBox = vb
	} else {
		// No `viewBox` attribute? Then use `x`, `y`, `width` and `height` instead.
		w, h := useNodeSize(node, state)

		// If attributes `width` and/or `height` are provided on the `use` element,
		// then these values will override the corresponding attributes
		// on the `svg` in the generated tree.
		if state.hasUseWidth {
			w = state.useWidth
		}
		if state.hasUseHeight {
			h = state.useHeight
		}

		if r, ok := path.NonZeroRectFromXYWH(x, y, w, h); ok {
			newState.viewBox = r
		} else {
			newState.viewBox = state.viewBox
		}
	}

	if clipRect, ok := getClipRect(node, node, state); ok {
		g := clipElement(node, clipRect, origTs, state, cache)
		g.absTransform = parent.absTransform.PreConcat(origTs)
		convertChildrenUseNode(node, newTs, newState, cache, false, g)
		g.calculateBoundingBoxes()
		parent.children = append(parent.children, g)
	} else {
		origTs = origTs.PreConcat(newTs)
		convertChildrenUseNode(node, origTs, newState, cache, false, parent)
	}
}

// clipElement is clip_element in usvg src/parser/use_node.rs.
func clipElement(
	node svgNode,
	clipRect NonZeroRect,
	transform Transform,
	state *converterState,
	cache *converterCache,
) *Group {
	// We can't set `clip-path` on the element itself,
	// because it will be affected by a possible transform.
	// So we have to create an additional group.

	// Emulate a new viewport via clipPath.
	//
	// From:
	// <defs/>
	// <elem/>
	//
	// To:
	// <defs>
	//   <clipPath id="clipPath1">
	//     <rect/>
	//   </clipPath>
	// </defs>
	// <g clip-path="ulr(#clipPath1)">
	//   <elem/>
	// </g>

	clipPath := emptyClipPath(cache.genClipPathID())

	p, ok := newSimplePath(path.PathFromRect(clipRect.ToRect()))
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	fill := DefaultFill()
	p.fill = &fill
	clipPath.root.children = append(clipPath.root.children, p)
	clipPath.root.calculateBoundingBoxes()

	// Nodes generated by markers must not have an ID. Otherwise we would have duplicates.
	id := ""
	if len(state.parentMarkers) == 0 {
		id = node.elementID()
	}

	g := emptyGroup()
	g.id = id
	g.transform = transform
	g.clipPath = clipPath
	return g
}

// convertChildrenUseNode is convert_children in usvg src/parser/use_node.rs.
func convertChildrenUseNode(
	node svgNode,
	transform Transform,
	state *converterState,
	cache *converterCache,
	isContextElement bool,
	parent *Group,
) {
	// Cache the parent absolute transform.
	parentAbsTransform := parent.absTransform

	required := !transform.IsIdentity()

	g, ok := convertGroup(node, state, required, cache, parent, func(cache *converterCache, g *Group) {
		// Make sure they are correct before converting children.
		g.transform = transform
		g.absTransform = parentAbsTransform.PreConcat(transform)
		g.isContextElement = isContextElement

		if state.parentClipPath != nil {
			convertClipPathElements(node, state, cache, g)
		} else {
			convertChildren(node, state, cache, g)
		}
	})
	if !ok {
		return
	}

	parent.children = append(parent.children, g)
}

// getClipRect is get_clip_rect in usvg src/parser/use_node.rs.
func getClipRect(
	useNode svgNode,
	symbolNode svgNode,
	state *converterState,
) (NonZeroRect, bool) {
	// No need to clip elements with overflow:visible.
	if v, ok := symbolNode.attributeStr(AIdOverflow); ok && (v == "visible" || v == "auto") { //nolint:goconst // an SVG keyword, as upstream writes it
		return NonZeroRect{}, false
	}

	// A nested `svg` with only the `viewBox` attribute and no "rectangle" (x, y, width, height)
	// should not be clipped.
	if useNode.hasTagName(EIdSvg) {
		// Nested `svg` referenced by `use` still should be clipped, but by `use` bounds.
		if !state.hasUseWidth && !state.hasUseHeight {
			if !useNode.hasAttribute(AIdWidth) || !useNode.hasAttribute(AIdHeight) {
				return NonZeroRect{}, false
			}
		}
	}

	x := useNode.convertUserLength(AIdX, state, svgtypes.LengthZero())
	y := useNode.convertUserLength(AIdY, state, svgtypes.LengthZero())
	w, h := useNodeSize(useNode, state)

	if useNode.hasTagName(EIdSvg) {
		// If attributes `width` and/or `height` are provided on the `use` element,
		// then these values will override the corresponding attributes
		// on the `svg` in the generated tree.
		if state.hasUseWidth {
			w = state.useWidth
		}
		if state.hasUseHeight {
			h = state.useHeight
		}
	}

	if !isValidLength32(w) || !isValidLength32(h) {
		return NonZeroRect{}, false
	}

	return path.NonZeroRectFromXYWH(x, y, w, h)
}

// useNodeSize is use_node_size in usvg src/parser/use_node.rs.
func useNodeSize(node svgNode, state *converterState) (float32, float32) {
	def := svgtypes.NewLength(100.0, svgtypes.LengthUnitPercent)
	w := node.convertUserLength(AIdWidth, state, def)
	h := node.convertUserLength(AIdHeight, state, def)
	return w, h
}

// viewboxTransform is viewbox_transform in usvg src/parser/use_node.rs.
func viewboxTransform(
	node svgNode,
	linked svgNode,
	state *converterState,
) (Transform, bool) {
	w, h := useNodeSize(node, state)

	if node.hasTagName(EIdSvg) {
		// If attributes `width` and/or `height` are provided on the `use` element,
		// then these values will override the corresponding attributes
		// on the `svg` in the generated tree.
		if state.hasUseWidth {
			w = state.useWidth
		}
		if state.hasUseHeight {
			h = state.useHeight
		}
	}

	size, ok := path.SizeFromWH(w, h)
	if !ok {
		return Transform{}, false
	}
	rect, ok := linked.parseViewbox()
	if !ok {
		return Transform{}, false
	}
	aspect, ok := nodeAttribute(linked, AIdPreserveAspectRatio, fromValueAspectRatio)
	if !ok {
		aspect = svgtypes.DefaultAspectRatio()
	}
	vb := viewBox{rect: rect, aspect: aspect}

	return vb.toTransform(size), true
}
