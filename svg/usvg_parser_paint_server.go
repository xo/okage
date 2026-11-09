// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/parser/paint_server.rs, without the items of the
// feature text.
//
// Port: upstream keeps a paint server in an Arc, and Arc::get_mut changes a
// paint server in place only when no other Arc holds it. Go has no reference
// count, so converterCache.paintIsUnique takes the place of Arc::get_mut. See
// usvg_parser_converter.go.

// serverOrColor is ServerOrColor in usvg src/parser/paint_server.rs.
//
// Port: the case Server holds a paint, and the case Color a color and an
// opacity, so the enum becomes a struct (D27). A non-nil server is the case
// Server.
type serverOrColor struct {
	// server is the paint of the case Server, or nil for the case Color.
	server Paint
	// color and opacity are the fields of the case Color.
	color   Color
	opacity Opacity
}

// convertPaintServer is convert in usvg src/parser/paint_server.rs.
func convertPaintServer(node svgNode, state *converterState, cache *converterCache) (serverOrColor, bool) {
	// Check for existing.
	if paint, ok := cache.paint[node.elementID()]; ok {
		return serverOrColor{server: paint}, true
	}

	// Unwrap is safe, because we already checked for is_paint_server().
	tagName, ok := node.tagName()
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	var paint serverOrColor
	switch tagName {
	case EIdLinearGradient:
		paint, ok = convertLinear(node, state)
	case EIdRadialGradient:
		paint, ok = convertRadial(node, state)
	case EIdPattern:
		paint, ok = convertPattern(node, state, cache)
	default:
		panic("svg: internal error: entered unreachable code")
	}

	if ok && paint.server != nil {
		cache.paint[node.elementID()] = paint.server
	}

	return paint, ok
}

// convertLinear is convert_linear in usvg src/parser/paint_server.rs.
func convertLinear(node svgNode, state *converterState) (serverOrColor, bool) {
	id, ok := newNonEmptyString(node.elementID())
	if !ok {
		return serverOrColor{}, false
	}

	withStops, ok := findGradientWithStops(node)
	if !ok {
		return serverOrColor{}, false
	}
	stops := convertStops(withStops, state)
	if len(stops) < 2 {
		return stopsToColor(stops)
	}

	units := convertUnits(node, AIdGradientUnits, unitsObjectBoundingBox)
	transform := resolveAttr(node, AIdGradientTransform).resolveTransform(AIdGradientTransform, state)

	gradient := &LinearGradient{
		x1: resolveNumber(node, AIdX1, units, state, svgtypes.LengthZero()),
		y1: resolveNumber(node, AIdY1, units, state, svgtypes.LengthZero()),
		x2: resolveNumber(
			node,
			AIdX2,
			units,
			state,
			svgtypes.NewLength(100.0, svgtypes.LengthUnitPercent),
		),
		y2:           resolveNumber(node, AIdY2, units, state, svgtypes.LengthZero()),
		id:           id,
		units:        units,
		transform:    transform,
		spreadMethod: convertSpreadMethod(node),
		stops:        stops,
	}

	return serverOrColor{server: gradient}, true
}

// convertRadial is convert_radial in usvg src/parser/paint_server.rs.
func convertRadial(node svgNode, state *converterState) (serverOrColor, bool) {
	id, ok := newNonEmptyString(node.elementID())
	if !ok {
		return serverOrColor{}, false
	}

	withStops, ok := findGradientWithStops(node)
	if !ok {
		return serverOrColor{}, false
	}
	stops := convertStops(withStops, state)
	if len(stops) < 2 {
		return stopsToColor(stops)
	}

	units := convertUnits(node, AIdGradientUnits, unitsObjectBoundingBox)
	r := resolveNumber(node, AIdR, units, state, svgtypes.NewLength(50.0, svgtypes.LengthUnitPercent))
	fr := resolveNumber(node, AIdFr, units, state, svgtypes.LengthZero())

	// 'A value of zero will cause the area to be painted as a single color
	// using the color and opacity of the last gradient stop.'
	//
	// https://www.w3.org/TR/SVG11/pservers.html#RadialGradientElementRAttribute
	if !isValidLength32(r) {
		stop := stops[len(stops)-1]
		return serverOrColor{
			color:   stop.color,
			opacity: stop.opacity,
		}, true
	}

	spreadMethod := convertSpreadMethod(node)
	cx := resolveNumber(
		node,
		AIdCx,
		units,
		state,
		svgtypes.NewLength(50.0, svgtypes.LengthUnitPercent),
	)
	cy := resolveNumber(
		node,
		AIdCy,
		units,
		state,
		svgtypes.NewLength(50.0, svgtypes.LengthUnitPercent),
	)
	fx := resolveNumber(node, AIdFx, units, state, svgtypes.NewLengthNumber(float64(cx)))
	fy := resolveNumber(node, AIdFy, units, state, svgtypes.NewLengthNumber(float64(cy)))
	transform := resolveAttr(node, AIdGradientTransform).resolveTransform(AIdGradientTransform, state)

	rPositive, ok := path.NewPositiveF32(r)
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	frPositive, ok := path.NewPositiveF32(fr)
	if !ok {
		frPositive = 0
	}
	gradient := &RadialGradient{
		cx:           cx,
		cy:           cy,
		r:            rPositive,
		fx:           fx,
		fy:           fy,
		fr:           frPositive,
		id:           id,
		units:        units,
		transform:    transform,
		spreadMethod: spreadMethod,
		stops:        stops,
	}

	return serverOrColor{server: gradient}, true
}

// convertPattern is convert_pattern in usvg src/parser/paint_server.rs.
func convertPattern(
	node svgNode,
	state *converterState,
	cache *converterCache,
) (serverOrColor, bool) {
	nodeWithChildren, ok := findPatternWithChildren(node)
	if !ok {
		return serverOrColor{}, false
	}

	id, ok := newNonEmptyString(node.elementID())
	if !ok {
		return serverOrColor{}, false
	}

	var vb *viewBox
	{
		n1 := resolveAttr(node, AIdViewBox)
		n2 := resolveAttr(node, AIdPreserveAspectRatio)
		if r, ok := n1.parseViewbox(); ok {
			aspect, ok := nodeAttribute(n2, AIdPreserveAspectRatio, fromValueAspectRatio)
			if !ok {
				aspect = svgtypes.DefaultAspectRatio()
			}
			vb = &viewBox{
				rect:   r,
				aspect: aspect,
			}
		}
	}

	units := convertUnits(node, AIdPatternUnits, unitsObjectBoundingBox)
	contentUnits := convertUnits(node, AIdPatternContentUnits, unitsUserSpaceOnUse)

	transform := resolveAttr(node, AIdPatternTransform).resolveTransform(AIdPatternTransform, state)

	rect, ok := path.NonZeroRectFromXYWH(
		resolveNumber(node, AIdX, units, state, svgtypes.LengthZero()),
		resolveNumber(node, AIdY, units, state, svgtypes.LengthZero()),
		resolveNumber(node, AIdWidth, units, state, svgtypes.LengthZero()),
		resolveNumber(node, AIdHeight, units, state, svgtypes.LengthZero()),
	)
	rect, ok = logNone(rect, ok, func() {
		state.opt.warnf(
			"Pattern '%s' has an invalid size. Skipped.",
			node.elementID(),
		)
	})
	if !ok {
		return serverOrColor{}, false
	}

	patt := &Pattern{
		id:           id,
		units:        units,
		contentUnits: contentUnits,
		transform:    transform,
		rect:         rect,
		viewBox:      vb,
		root:         *emptyGroup(),
	}

	// We can apply viewbox transform only for user space coordinates.
	// Otherwise we need a bounding box, which is unknown at this point.
	if patt.viewBox != nil &&
		patt.units == unitsUserSpaceOnUse &&
		patt.contentUnits == unitsUserSpaceOnUse {
		g := emptyGroup()
		g.transform = vb.toTransform(rect.Size())
		g.absTransform = g.transform

		convertChildren(nodeWithChildren, state, cache, g)
		if !g.HasChildren() {
			return serverOrColor{}, false
		}

		g.calculateBoundingBoxes()
		patt.root.children = append(patt.root.children, g)
	} else {
		convertChildren(nodeWithChildren, state, cache, &patt.root)
		if !patt.root.HasChildren() {
			return serverOrColor{}, false
		}
	}

	patt.root.calculateBoundingBoxes()
	updatePaintServers(&patt.root, path.DefaultTransform(), nil, nil, cache)

	return serverOrColor{server: patt}, true
}

// convertSpreadMethod is convert_spread_method in usvg
// src/parser/paint_server.rs.
func convertSpreadMethod(node svgNode) SpreadMethod {
	node = resolveAttr(node, AIdSpreadMethod)
	m, _ := nodeAttribute(node, AIdSpreadMethod, fromValueSpreadMethod)
	return m
}

// convertUnits is convert_units in usvg src/parser/paint_server.rs.
func convertUnits(node svgNode, name AId, def units) units {
	node = resolveAttr(node, name)
	u, ok := nodeAttribute(node, name, fromValueUnits)
	if !ok {
		return def
	}
	return u
}

// findGradientWithStops is find_gradient_with_stops in usvg
// src/parser/paint_server.rs.
func findGradientWithStops(node svgNode) (svgNode, bool) {
	for link := range node.hrefIter() {
		tagName, ok := link.tagName()
		if !ok {
			panic("svg: called Option::unwrap() on a None value")
		}
		if !tagName.IsGradient() {
			node.doc.opt.warnf(
				"Gradient '%s' cannot reference '%s' via 'xlink:href'.",
				node.elementID(),
				tagName,
			)
			return svgNode{}, false
		}

		for n := range link.children() {
			if n.hasTagName(EIdStop) {
				return link, true
			}
		}
	}

	return svgNode{}, false
}

// findPatternWithChildren is find_pattern_with_children in usvg
// src/parser/paint_server.rs.
func findPatternWithChildren(node svgNode) (svgNode, bool) {
	for link := range node.hrefIter() {
		if !link.hasTagName(EIdPattern) {
			tagName, ok := link.tagName()
			if !ok {
				panic("svg: called Option::unwrap() on a None value")
			}
			node.doc.opt.warnf(
				"Pattern '%s' cannot reference '%s' via 'xlink:href'.",
				node.elementID(),
				tagName,
			)
			return svgNode{}, false
		}

		if link.hasChildren() {
			return link, true
		}
	}

	return svgNode{}, false
}

// convertStops is convert_stops in usvg src/parser/paint_server.rs.
//
// Port: Go takes the state for the logger of the options (D20).
func convertStops(grad svgNode, state *converterState) []Stop {
	var stops []Stop

	{
		prevOffset := svgtypes.LengthZero()
		for stop := range grad.children() {
			tagName, ok := stop.tagName()
			if !ok || tagName != EIdStop {
				if !ok {
					panic("svg: called Option::unwrap() on a None value")
				}
				state.opt.warnf("Invalid gradient child: '%s'.", tagName)
				continue
			}

			// `number` can be either a number or a percentage.
			offset, ok := stop.attributeLength(AIdOffset)
			if !ok {
				offset = prevOffset
			}
			var offsetNumber float64
			switch offset.Unit {
			case svgtypes.LengthUnitNone:
				offsetNumber = offset.Number
			case svgtypes.LengthUnitPercent:
				offsetNumber = offset.Number / 100.0
			default:
				offsetNumber = prevOffset.Number
			}
			prevOffset = svgtypes.NewLengthNumber(offsetNumber)
			offset32 := f32Bound(0.0, float32(offsetNumber), 1.0)

			var svgColor svgtypes.Color
			if value, ok := stop.attributeStr(AIdStopColor); ok {
				if value == "currentColor" {
					svgColor, ok = nodeFindAttribute(stop, AIdColor, fromValueColor)
					if !ok {
						svgColor = svgtypes.ColorBlack()
					}
				} else if c, err := svgtypes.ParseColor(value); err == nil {
					svgColor = c
				} else {
					state.opt.warnf("Failed to parse stop-color value: '%s'.", value)
					svgColor = svgtypes.ColorBlack()
				}
			} else {
				svgColor = svgtypes.ColorBlack()
			}
			color, opacity := splitAlpha(svgColor)

			stopOpacity, ok := stop.attributeOpacity(AIdStopOpacity)
			if !ok {
				stopOpacity = path.NormalizedF32One
			}
			stops = append(stops, Stop{
				offset:  path.NormalizedF32Clamped(offset32),
				color:   color,
				opacity: opacity.Mul(stopOpacity),
			})
		}
	}

	// Remove stops with equal offset.
	//
	//nolint:dupword // the example of upstream repeats an offset
	// Example:
	// offset="0.5"
	// offset="0.7"
	// offset="0.7" <-- this one should be removed
	// offset="0.7"
	// offset="0.9"
	if len(stops) >= 3 {
		i := 0
		for i < len(stops)-2 {
			offset1 := stops[i+0].offset.Get()
			offset2 := stops[i+1].offset.Get()
			offset3 := stops[i+2].offset.Get()

			if ApproxEqUlps32(offset1, offset2, 4) && ApproxEqUlps32(offset2, offset3, 4) {
				// Remove offset in the middle.
				stops = append(stops[:i+1], stops[i+2:]...)
			} else {
				i++
			}
		}
	}

	// Remove zeros.
	//
	//nolint:dupword // the example of upstream repeats an offset
	// From:
	// offset="0.0"
	// offset="0.0"
	// offset="0.7"
	//
	// To:
	// offset="0.0"
	// offset="0.00000001"
	// offset="0.7"
	if len(stops) >= 2 {
		i := 0
		for i < len(stops)-1 {
			offset1 := stops[i+0].offset.Get()
			offset2 := stops[i+1].offset.Get()

			if ApproxEqUlps32(offset1, 0.0, 4) && ApproxEqUlps32(offset2, 0.0, 4) {
				stops[i+1].offset = path.NormalizedF32Clamped(offset1 + epsilonF32)
			}

			i++
		}
	}

	// Shift equal offsets.
	//
	//nolint:dupword // the example of upstream repeats an offset
	// From:
	// offset="0.5"
	// offset="0.7"
	// offset="0.7"
	//
	// To:
	// offset="0.5"
	// offset="0.699999999"
	// offset="0.7"
	{
		i := 1
		for i < len(stops) {
			offset1 := stops[i-1].offset.Get()
			offset2 := stops[i-0].offset.Get()

			// Next offset must be smaller then previous.
			if offset1 > offset2 || ApproxEqUlps32(offset1, offset2, 4) {
				// Make previous offset a bit smaller.
				newOffset := offset1 - epsilonF32
				stops[i-1].offset = path.NormalizedF32Clamped(newOffset)
				stops[i-0].offset = path.NormalizedF32Clamped(offset1)
			}

			i++
		}
	}

	return stops
}

// epsilonF32 is f32::EPSILON of Rust.
const epsilonF32 float32 = 1.1920929e-07

// resolveNumber is resolve_number in usvg src/parser/paint_server.rs.
func resolveNumber(
	node svgNode,
	name AId,
	units units,
	state *converterState,
	def svgtypes.Length,
) float32 {
	return resolveAttr(node, name).convertLength(name, units, state, def)
}

// resolveAttr is resolve_attr in usvg src/parser/paint_server.rs.
func resolveAttr(node svgNode, name AId) svgNode {
	if node.hasAttribute(name) {
		return node
	}

	tagName, ok := node.tagName()
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	switch tagName {
	case EIdLinearGradient:
		return resolveLgAttr(node, name)
	case EIdRadialGradient:
		return resolveRgAttr(node, name)
	case EIdPattern:
		return resolvePatternAttr(node, name)
	case EIdFilter:
		return resolveFilterAttr(node, name)
	}
	return node
}

// resolveLgAttr is resolve_lg_attr in usvg src/parser/paint_server.rs.
func resolveLgAttr(node svgNode, name AId) svgNode {
	for link := range node.hrefIter() {
		tagName, ok := link.tagName()
		if !ok {
			return node
		}

		isLg := tagName == EIdLinearGradient
		isGradient := isLg || tagName == EIdRadialGradient
		switch {
		// Coordinates can be resolved only from
		// ref element with the same type.
		case (name == AIdX1 || name == AIdY1 || name == AIdX2 || name == AIdY2) && isLg,
			// Other attributes can be resolved
			// from any kind of gradient.
			(name == AIdGradientUnits || name == AIdSpreadMethod || name == AIdGradientTransform) && isGradient:
			if link.hasAttribute(name) {
				return link
			}
		default:
			return node
		}
	}

	return node
}

// resolveRgAttr is resolve_rg_attr in usvg src/parser/paint_server.rs.
func resolveRgAttr(node svgNode, name AId) svgNode {
	for link := range node.hrefIter() {
		tagName, ok := link.tagName()
		if !ok {
			return node
		}

		isRg := tagName == EIdRadialGradient
		isGradient := isRg || tagName == EIdLinearGradient
		switch {
		// Coordinates can be resolved only from
		// ref element with the same type.
		case (name == AIdCx || name == AIdCy || name == AIdR || name == AIdFx || name == AIdFy || name == AIdFr) && isRg,
			// Other attributes can be resolved
			// from any kind of gradient.
			(name == AIdGradientUnits || name == AIdSpreadMethod || name == AIdGradientTransform) && isGradient:
			if link.hasAttribute(name) {
				return link
			}
		default:
			return node
		}
	}

	return node
}

// resolvePatternAttr is resolve_pattern_attr in usvg
// src/parser/paint_server.rs.
func resolvePatternAttr(node svgNode, name AId) svgNode {
	for link := range node.hrefIter() {
		tagName, ok := link.tagName()
		if !ok {
			return node
		}

		if tagName != EIdPattern {
			break
		}

		if link.hasAttribute(name) {
			return link
		}
	}

	return node
}

// resolveFilterAttr is resolve_filter_attr in usvg
// src/parser/paint_server.rs.
func resolveFilterAttr(node svgNode, aid AId) svgNode {
	for link := range node.hrefIter() {
		tagName, ok := link.tagName()
		if !ok {
			return node
		}

		if tagName != EIdFilter {
			break
		}

		if link.hasAttribute(aid) {
			return link
		}
	}

	return node
}

// stopsToColor is stops_to_color in usvg src/parser/paint_server.rs.
func stopsToColor(stops []Stop) (serverOrColor, bool) {
	if len(stops) == 0 {
		return serverOrColor{}, false
	}
	return serverOrColor{
		color:   stops[0].color,
		opacity: stops[0].opacity,
	}, true
}

// updatePaintServers is update_paint_servers in usvg
// src/parser/paint_server.rs.
//
// Update paints servers by doing the following:
// 1. Replace context fills/strokes that are linked to
// a use node with their actual values.
// 2. Convert all object units to UserSpaceOnUse
//
// Port: a nil rect is None.
func updatePaintServers(
	group *Group,
	contextTransform Transform,
	contextBBox *Rect,
	textBBox *Rect,
	cache *converterCache,
) {
	for _, child := range group.children {
		// Set context transform and bbox if applicable if the
		// current group is a use node.
		ctxTransform, ctxBBox := contextTransform, contextBBox
		if group.isContextElement {
			ctxTransform, ctxBBox = group.absTransform, nil
			if bbox, ok := group.boundingBox.get(); ok {
				ctxBBox = &bbox
			}
		}

		nodeToUserCoordinates(child, ctxTransform, ctxBBox, textBBox, cache)
	}
}

// nodeToUserCoordinates is node_to_user_coordinates in usvg
// src/parser/paint_server.rs.
//
// When parsing clipPaths, masks and filters we already know group's bounding box.
// But with gradients and patterns we don't, because we have to know text bounding box
// before we even parsed it. Which is impossible.
// Therefore our only choice is to parse gradients and patterns preserving their units
// and then replace them with `userSpaceOnUse` after the whole tree parsing is finished.
// So while gradients and patterns do still store their units,
// they are not exposed in the public API and for the caller they are always `userSpaceOnUse`.
//
// Port: a nil rect is None.
func nodeToUserCoordinates(
	node Node,
	contextTransform Transform,
	contextBBox *Rect,
	textBBox *Rect,
	cache *converterCache,
) {
	switch n := node.(type) {
	case *Group:
		updatePaintServers(n, contextTransform, contextBBox, textBBox, cache)
	case *Path:
		// Paths inside `Text::flattened` are special and must use text's bounding box
		// instead of their own.
		bbox := n.boundingBox
		if textBBox != nil {
			bbox = *textBBox
		}

		processFill(
			&n.fill,
			n.absTransform,
			contextTransform,
			contextBBox,
			bbox,
			cache,
		)
		processStroke(
			&n.stroke,
			n.absTransform,
			contextTransform,
			contextBBox,
			bbox,
			cache,
		)
	case *Image:
		if svg, ok := n.kind.(ImageKindSVG); ok {
			updatePaintServers(&svg.Tree.root, contextTransform, contextBBox, nil, cache)
		}
	case *Text:
		// By the SVG spec, `tspan` doesn't have a bbox and uses the parent `text` bbox.
		// Therefore we have to use text's bbox when converting tspan and flatted text
		// paint servers.
		bbox := n.boundingBox

		// We need to update three things:
		// 1. The fills/strokes of the original elements in the usvg tree.
		// 2. The fills/strokes of the layouted elements of the text.
		// 3. The fills/strokes of the outlined text.

		// 1.
		for i := range n.chunks {
			chunk := &n.chunks[i]
			for j := range chunk.spans {
				span := &chunk.spans[j]
				processFill(
					&span.fill,
					n.absTransform,
					contextTransform,
					contextBBox,
					bbox,
					cache,
				)
				processStroke(
					&span.stroke,
					n.absTransform,
					contextTransform,
					contextBBox,
					bbox,
					cache,
				)
				processTextDecoration(span.decoration.underline, bbox, cache)
				processTextDecoration(span.decoration.overline, bbox, cache)
				processTextDecoration(span.decoration.lineThrough, bbox, cache)
			}
		}

		// 2.
		for i := range n.layouted {
			span := &n.layouted[i]
			processFill(
				&span.Fill,
				n.absTransform,
				contextTransform,
				contextBBox,
				bbox,
				cache,
			)
			processStroke(
				&span.Stroke,
				n.absTransform,
				contextTransform,
				contextBBox,
				bbox,
				cache,
			)

			processDecoration := func(p *Path) {
				processFill(
					&p.fill,
					n.absTransform,
					contextTransform,
					contextBBox,
					bbox,
					cache,
				)
				processStroke(
					&p.stroke,
					n.absTransform,
					contextTransform,
					contextBBox,
					bbox,
					cache,
				)
			}

			if span.Overline != nil {
				processDecoration(span.Overline)
			}

			if span.Underline != nil {
				processDecoration(span.Underline)
			}

			if span.LineThrough != nil {
				processDecoration(span.LineThrough)
			}
		}

		// 3.
		if n.flattened != nil {
			updatePaintServers(
				n.flattened,
				contextTransform,
				contextBBox,
				&bbox,
				cache,
			)
		}
	}
}

// processFill is process_fill in usvg src/parser/paint_server.rs.
//
// Port: a nil *Fill is None, and a nil rect is None.
func processFill(
	fill **Fill,
	pathTransform Transform,
	contextTransform Transform,
	contextBBox *Rect,
	bbox Rect,
	cache *converterCache,
) {
	ok := false
	if f := *fill; f != nil {
		// Path context elements (i.e. for  markers) have already been resolved,
		// so we only care about use nodes.
		ok = processPaint(
			&f.paint,
			f.contextElement != nil && f.contextElement.kind == contextElementUseNode,
			contextTransform,
			contextBBox,
			pathTransform,
			bbox,
			cache,
		)
	}
	if !ok {
		*fill = nil
	}
}

// processStroke is process_stroke in usvg src/parser/paint_server.rs.
//
// Port: a nil *Stroke is None, and a nil rect is None.
func processStroke(
	stroke **Stroke,
	pathTransform Transform,
	contextTransform Transform,
	contextBBox *Rect,
	bbox Rect,
	cache *converterCache,
) {
	ok := false
	if s := *stroke; s != nil {
		// Path context elements (i.e. for  markers) have already been resolved,
		// so we only care about use nodes.
		ok = processPaint(
			&s.paint,
			s.contextElement != nil && s.contextElement.kind == contextElementUseNode,
			contextTransform,
			contextBBox,
			pathTransform,
			bbox,
			cache,
		)
	}
	if !ok {
		*stroke = nil
	}
}

// processContextPaint is process_context_paint in usvg
// src/parser/paint_server.rs.
//
// Port: upstream returns Option<()>. Go returns false for None.
func processContextPaint(
	paint *Paint,
	contextTransform Transform,
	pathTransform Transform,
	cache *converterCache,
) bool {
	// The idea is the following: We have a certain context element that has
	// a transform A, and further below in the tree we have for example a path
	// whose paint has a transform C. In order to get from A to C, there is some
	// transformation matrix B such that A x B = C. We now need to figure out
	// a way to get from C back to A, so that the transformation of the paint
	// matches the one from the context element, even if B was applied. How
	// do we do that? We calculate CxB^(-1), which will overall then have
	// the same effect as A. How do we calculate B^(-1)?
	// --> (A^(-1)xC)^(-1)
	inv, ok := contextTransform.Invert()
	if !ok {
		return false
	}
	revTransform, ok := inv.PreConcat(pathTransform).Invert()
	if !ok {
		return false
	}

	switch p := (*paint).(type) {
	case Color:
	case *LinearGradient:
		transform := p.transform.PostConcat(revTransform)
		newPaint := &LinearGradient{
			x1:           p.x1,
			y1:           p.y1,
			x2:           p.x2,
			y2:           p.y2,
			id:           cache.genLinearGradientID(),
			units:        p.units,
			transform:    transform,
			spreadMethod: p.spreadMethod,
			stops:        p.stops,
		}
		cache.replacePaint(*paint, newPaint)
		*paint = newPaint
	case *RadialGradient:
		transform := p.transform.PostConcat(revTransform)
		newPaint := &RadialGradient{
			cx:           p.cx,
			cy:           p.cy,
			r:            p.r,
			fx:           p.fx,
			fy:           p.fy,
			fr:           p.fr,
			id:           cache.genRadialGradientID(),
			units:        p.units,
			transform:    transform,
			spreadMethod: p.spreadMethod,
			stops:        p.stops,
		}
		cache.replacePaint(*paint, newPaint)
		*paint = newPaint
	case *Pattern:
		transform := p.transform.PostConcat(revTransform)
		newPaint := &Pattern{
			id:           cache.genPatternID(),
			units:        p.units,
			contentUnits: p.contentUnits,
			transform:    transform,
			rect:         p.rect,
			viewBox:      p.viewBox,
			root:         p.root.cloneGroup(),
		}
		cache.replacePaint(*paint, newPaint)
		*paint = newPaint
	}

	return true
}

// processPaint is process_paint in usvg src/parser/paint_server.rs.
//
// Port: a nil rect is None.
func processPaint(
	paint *Paint,
	hasContext bool,
	contextTransform Transform,
	contextBBox *Rect,
	pathTransform Transform,
	bbox Rect,
	cache *converterCache,
) bool {
	if paintUnits(*paint) == unitsObjectBoundingBox ||
		paintContentUnits(*paint) == unitsObjectBoundingBox {
		if hasContext {
			if contextBBox == nil {
				return false
			}
			bbox = *contextBBox
		}

		if !paintToUserCoordinates(paint, bbox, cache) {
			return false
		}
	}

	if hasContext {
		processContextPaint(paint, contextTransform, pathTransform, cache)
	}

	return true
}

// processTextDecoration is process_text_decoration in usvg
// src/parser/paint_server.rs.
//
// Port: a nil style is None.
func processTextDecoration(style *TextDecorationStyle, bbox Rect, cache *converterCache) {
	if style != nil {
		processFill(
			&style.fill,
			path.DefaultTransform(),
			path.DefaultTransform(),
			nil,
			bbox,
			cache,
		)
		processStroke(
			&style.stroke,
			path.DefaultTransform(),
			path.DefaultTransform(),
			nil,
			bbox,
			cache,
		)
	}
}

// paintToUserCoordinates is Paint::to_user_coordinates in usvg
// src/parser/paint_server.rs.
//
// Port: upstream returns Option<()>. Go returns false for None. Where
// upstream changes a paint server in place when Arc::get_mut allows it, Go
// does so when cache.paintIsUnique reports that no other paint holds it.
func paintToUserCoordinates(paint *Paint, bbox Rect, cache *converterCache) bool {
	name := "Gradient"
	if _, ok := (*paint).(*Pattern); ok {
		name = "Pattern"
	}
	nzBBox, ok := bbox.ToNonZeroRect()
	nzBBox, ok = logNone(nzBBox, ok, func() {
		cache.opt.warnf("%s on zero-sized shapes is not allowed.", name)
	})
	if !ok {
		return false
	}

	// `Arc::get_mut()` allow us to modify some paint servers in-place.
	// This reduces the amount of cloning and preserves the original ID as well.
	switch p := (*paint).(type) {
	case Color: // unreachable
	case *LinearGradient:
		transform := p.transform.PostConcat(path.TransformFromBBox(nzBBox))
		if cache.paintIsUnique(p) {
			p.transform = transform
			p.units = unitsUserSpaceOnUse
		} else {
			newPaint := &LinearGradient{
				x1:           p.x1,
				y1:           p.y1,
				x2:           p.x2,
				y2:           p.y2,
				id:           cache.genLinearGradientID(),
				units:        unitsUserSpaceOnUse,
				transform:    transform,
				spreadMethod: p.spreadMethod,
				stops:        p.stops,
			}
			cache.replacePaint(p, newPaint)
			*paint = newPaint
		}
	case *RadialGradient:
		transform := p.transform.PostConcat(path.TransformFromBBox(nzBBox))
		if cache.paintIsUnique(p) {
			p.transform = transform
			p.units = unitsUserSpaceOnUse
		} else {
			newPaint := &RadialGradient{
				cx:           p.cx,
				cy:           p.cy,
				r:            p.r,
				fx:           p.fx,
				fy:           p.fy,
				fr:           p.fr,
				id:           cache.genRadialGradientID(),
				units:        unitsUserSpaceOnUse,
				transform:    transform,
				spreadMethod: p.spreadMethod,
				stops:        p.stops,
			}
			cache.replacePaint(p, newPaint)
			*paint = newPaint
		}
	case *Pattern:
		rect := p.rect
		if p.units == unitsObjectBoundingBox {
			rect = p.rect.BBoxTransform(nzBBox)
		}

		if cache.paintIsUnique(p) {
			p.rect = rect
			p.units = unitsUserSpaceOnUse

			if p.contentUnits == unitsObjectBoundingBox && p.viewBox == nil {
				// No need to shift patterns.
				transform := path.TransformFromScale(nzBBox.Width(), nzBBox.Height())
				pushPatternTransform(&p.root, transform)
			}

			if p.viewBox != nil {
				pushPatternTransform(&p.root, p.viewBox.toTransform(rect.Size()))
			}

			p.contentUnits = unitsUserSpaceOnUse
		} else {
			var root Group
			if p.contentUnits == unitsObjectBoundingBox && p.viewBox == nil {
				// No need to shift patterns.
				transform := path.TransformFromScale(nzBBox.Width(), nzBBox.Height())

				g := p.root.cloneGroup()
				pushPatternTransform(&g, transform)
				root = g
			} else {
				root = p.root.cloneGroup()
			}

			if p.viewBox != nil {
				pushPatternTransform(&root, p.viewBox.toTransform(rect.Size()))
			}

			newPaint := &Pattern{
				id:           cache.genPatternID(),
				units:        unitsUserSpaceOnUse,
				contentUnits: unitsUserSpaceOnUse,
				transform:    p.transform,
				rect:         rect,
				viewBox:      p.viewBox,
				root:         root,
			}
			cache.replacePaint(p, newPaint)
			*paint = newPaint
		}
	}

	return true
}

// pushPatternTransform is push_pattern_transform in usvg
// src/parser/paint_server.rs.
func pushPatternTransform(root *Group, transform Transform) {
	// TODO: we should update abs_transform in all descendants as well
	g := new(Group)
	*g = *root
	*root = *emptyGroup()
	g.transform = transform
	g.absTransform = transform

	root.children = append(root.children, g)
	root.calculateBoundingBoxes()
}

// paintUnits is Paint::units in usvg src/parser/paint_server.rs.
func paintUnits(p Paint) units {
	switch p := p.(type) {
	case Color:
		return unitsUserSpaceOnUse
	case *LinearGradient:
		return p.units
	case *RadialGradient:
		return p.units
	case *Pattern:
		return p.units
	}
	return unitsUserSpaceOnUse
}

// paintContentUnits is Paint::content_units in usvg
// src/parser/paint_server.rs.
func paintContentUnits(p Paint) units {
	if patt, ok := p.(*Pattern); ok {
		return patt.contentUnits
	}
	return unitsUserSpaceOnUse
}
