// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"math"
	"slices"

	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/parser/style.rs.

// fromValueLineCap is the impl FromValue for LineCap in usvg
// src/parser/style.rs.
func fromValueLineCap(_ svgNode, _ AId, value string) (LineCap, bool) {
	switch value {
	case "butt":
		return LineCapButt, true
	case "round":
		return LineCapRound, true
	case "square":
		return LineCapSquare, true
	}
	return 0, false
}

// fromValueLineJoin is the impl FromValue for LineJoin in usvg
// src/parser/style.rs.
func fromValueLineJoin(_ svgNode, _ AId, value string) (LineJoin, bool) {
	switch value {
	case "miter":
		return LineJoinMiter, true
	case "miter-clip":
		return LineJoinMiterClip, true
	case "round":
		return LineJoinRound, true
	case "bevel":
		return LineJoinBevel, true
	}
	return 0, false
}

// fromValueFillRule is the impl FromValue for FillRule in usvg
// src/parser/style.rs.
func fromValueFillRule(_ svgNode, _ AId, value string) (FillRule, bool) {
	switch value {
	case "nonzero":
		return FillRuleNonZero, true
	case "evenodd":
		return FillRuleEvenOdd, true
	}
	return 0, false
}

// resolveFill is resolve_fill in usvg src/parser/style.rs.
func resolveFill(
	node svgNode,
	hasBBox bool,
	state *converterState,
	cache *converterCache,
) (*Fill, bool) {
	if state.parentClipPath != nil {
		// A `clipPath` child can be filled only with a black color.
		rule, _ := nodeFindAttribute(node, AIdClipRule, fromValueFillRule)
		return &Fill{
			paint:          ColorBlack(),
			opacity:        path.NormalizedF32One,
			rule:           rule,
			contextElement: nil,
		}, true
	}

	subOpacity := path.NormalizedF32One
	var paint Paint
	var contextElement *contextElement
	if n, ok := findAncestorWithAttribute(node, AIdFill); ok {
		value, ok := n.attributeStr(AIdFill)
		if !ok {
			return nil, false
		}
		paint, contextElement, ok = convertPaint(
			node,
			value,
			AIdFill,
			hasBBox,
			state,
			&subOpacity,
			cache,
		)
		if !ok {
			return nil, false
		}
	} else {
		paint = ColorBlack()
	}

	fillOpacity, ok := nodeFindAttribute(node, AIdFillOpacity, fromValueOpacity)
	if !ok {
		fillOpacity = path.NormalizedF32One
	}

	rule, _ := nodeFindAttribute(node, AIdFillRule, fromValueFillRule)
	return &Fill{
		paint:          paint,
		opacity:        subOpacity.Mul(fillOpacity),
		rule:           rule,
		contextElement: contextElement,
	}, true
}

// findAncestorWithAttribute is the Rust expression
// `node.ancestors().find(|n| n.has_attribute(aid))`.
func findAncestorWithAttribute(node svgNode, aid AId) (svgNode, bool) {
	for n := range node.ancestors() {
		if n.hasAttribute(aid) {
			return n, true
		}
	}
	return svgNode{}, false
}

// resolveStroke is resolve_stroke in usvg src/parser/style.rs.
func resolveStroke(
	node svgNode,
	hasBBox bool,
	state *converterState,
	cache *converterCache,
) (*Stroke, bool) {
	if state.parentClipPath != nil {
		// A `clipPath` child cannot be stroked.
		return nil, false
	}

	subOpacity := path.NormalizedF32One
	n, ok := findAncestorWithAttribute(node, AIdStroke)
	if !ok {
		return nil, false
	}
	value, ok := n.attributeStr(AIdStroke)
	if !ok {
		return nil, false
	}

	paint, contextElement, ok := convertPaint(
		node,
		value,
		AIdStroke,
		hasBBox,
		state,
		&subOpacity,
		cache,
	)
	if !ok {
		return nil, false
	}

	width, ok := node.resolveValidLength(AIdStrokeWidth, state, 1.0)
	if !ok {
		return nil, false
	}

	// Must be bigger than 1.
	miterlimit, ok := nodeFindAttribute(node, AIdStrokeMiterlimit, fromValueF32)
	if !ok {
		miterlimit = 4.0
	}
	if miterlimit < 1.0 {
		miterlimit = 1.0
	}
	ml := NewStrokeMiterlimit(miterlimit)

	strokeOpacity, ok := nodeFindAttribute(node, AIdStrokeOpacity, fromValueOpacity)
	if !ok {
		strokeOpacity = path.NormalizedF32One
	}

	linecap, _ := nodeFindAttribute(node, AIdStrokeLinecap, fromValueLineCap)
	linejoin, _ := nodeFindAttribute(node, AIdStrokeLinejoin, fromValueLineJoin)
	stroke := &Stroke{
		paint:          paint,
		dasharray:      convDasharray(node, state),
		dashoffset:     node.resolveLength(AIdStrokeDashoffset, state, 0.0),
		miterlimit:     ml,
		opacity:        subOpacity.Mul(strokeOpacity),
		width:          width,
		linecap:        linecap,
		linejoin:       linejoin,
		contextElement: contextElement,
	}

	return stroke, true
}

// convertPaint is convert_paint in usvg src/parser/style.rs.
//
// Port: upstream returns Option<(Paint, Option<ContextElement>)>. Go returns
// the paint, the context element or nil, and false for None.
func convertPaint(
	node svgNode,
	value string,
	aid AId,
	hasBBox bool,
	state *converterState,
	opacity *Opacity,
	cache *converterCache,
) (Paint, *contextElement, bool) {
	paint, err := svgtypes.ParsePaint(value)
	if err != nil {
		switch aid {
		case AIdFill:
			state.opt.warnf(
				"Failed to parse fill value: '%s'. Fallback to black.",
				value,
			)
			paint = svgtypes.Paint{Kind: svgtypes.PaintColor, Color: svgtypes.ColorBlack()}
		case AIdStroke:
			state.opt.warnf(
				"Failed to parse stroke value: '%s'. Fallback to no stroke.",
				value,
			)
			return nil, nil, false
		default:
			return nil, nil, false
		}
	}

	switch paint.Kind {
	case svgtypes.PaintNone:
		return nil, nil, false
	case svgtypes.PaintInherit:
		return nil, nil, false // already resolved by svgtree
	case svgtypes.PaintContextFill:
		if state.contextElement == nil || state.contextElement.fill == nil {
			return nil, nil, false
		}
		f := state.contextElement.fill
		return f.paint, f.contextElement, true
	case svgtypes.PaintContextStroke:
		if state.contextElement == nil || state.contextElement.stroke == nil {
			return nil, nil, false
		}
		s := state.contextElement.stroke
		return s.paint, s.contextElement, true
	case svgtypes.PaintCurrentColor:
		svgColor, ok := nodeFindAttribute(node, AIdColor, fromValueColor)
		if !ok {
			svgColor = svgtypes.ColorBlack()
		}
		color, alpha := splitAlpha(svgColor)
		*opacity = alpha
		return color, nil, true
	case svgtypes.PaintColor:
		color, alpha := splitAlpha(paint.Color)
		*opacity = alpha
		return color, nil, true
	case svgtypes.PaintFuncIRI:
		link, ok := node.document().elementByID(paint.FuncIRI)
		if !ok {
			p, ok := fromFallback(node, paint.Fallback, paint.HasFallback, opacity)
			return p, nil, ok
		}
		tagName, ok := link.tagName()
		if !ok {
			panic("svg: called Option::unwrap() on a None value")
		}
		if !tagName.IsPaintServer() {
			state.opt.warnf("'%s' cannot be used to %s a shape.", tagName, aid)
			return nil, nil, false
		}
		soc, ok := convertPaintServer(link, state, cache)
		if !ok {
			p, ok := fromFallback(node, paint.Fallback, paint.HasFallback, opacity)
			return p, nil, ok
		}
		if soc.server != nil {
			// We can use a paint server node with ObjectBoundingBox units
			// for painting only when the shape itself has a bbox.
			//
			// See SVG spec 7.11 for details.
			if !hasBBox && paintUnits(soc.server) == unitsObjectBoundingBox {
				p, ok := fromFallback(node, paint.Fallback, paint.HasFallback, opacity)
				return p, nil, ok
			}
			return soc.server, nil, true
		}
		*opacity = soc.opacity
		return soc.color, nil, true
	}
	return nil, nil, false
}

// fromFallback is from_fallback in usvg src/parser/style.rs.
//
// Port: hasFallback is false for a fallback of None.
func fromFallback(
	node svgNode,
	fallback svgtypes.PaintFallback,
	hasFallback bool,
	opacity *Opacity,
) (Paint, bool) {
	if !hasFallback {
		return nil, false
	}
	switch fallback.Kind {
	case svgtypes.PaintFallbackNone:
		return nil, false
	case svgtypes.PaintFallbackCurrentColor:
		svgColor, ok := nodeFindAttribute(node, AIdColor, fromValueColor)
		if !ok {
			svgColor = svgtypes.ColorBlack()
		}
		color, alpha := splitAlpha(svgColor)
		*opacity = alpha
		return color, true
	case svgtypes.PaintFallbackColor:
		color, alpha := splitAlpha(fallback.Color)
		*opacity = alpha
		return color, true
	}
	return nil, false
}

// convDasharray is conv_dasharray in usvg src/parser/style.rs. It prepares
// the 'stroke-dasharray' according to:
// https://www.w3.org/TR/SVG11/painting.html#StrokeDasharrayProperty
//
// Port: nil is None.
func convDasharray(node svgNode, state *converterState) []float32 {
	n, ok := findAncestorWithAttribute(node, AIdStrokeDasharray)
	if !ok {
		return nil
	}
	list, ok := convertList(n, AIdStrokeDasharray, state)
	if !ok {
		return nil
	}

	// `A negative value is an error`
	if slices.ContainsFunc(list, func(n float32) bool { return math.Signbit(float64(n)) }) {
		return nil
	}

	// `If the sum of the values is zero, then the stroke is rendered
	// as if a value of none were specified.`
	{
		// no Iter::sum(), because of f64

		var sum float32
		for _, n := range list {
			sum += n
		}

		if ApproxEqUlps32(sum, 0.0, 4) {
			return nil
		}
	}

	// `If an odd number of values is provided, then the list of values
	// is repeated to yield an even number of values.`
	if len(list)%2 != 0 {
		tmpList := slices.Clone(list)
		tmpList = append(tmpList, list...)
		return tmpList
	}

	return list
}
