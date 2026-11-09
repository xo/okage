// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"math"

	"github.com/xo/okage/internal/kurbo"
	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/parser/shapes.rs.
//
// Port: an Arc<Path> of upstream is a *path.Path.

// convertShapes is convert in usvg src/parser/shapes.rs.
func convertShapes(node svgNode, state *converterState) (*path.Path, bool) {
	tag, ok := node.tagName()
	if !ok {
		return nil, false
	}
	switch tag {
	case EIdRect:
		return convertRect(node, state)
	case EIdCircle:
		return convertCircle(node, state)
	case EIdEllipse:
		return convertEllipse(node, state)
	case EIdLine:
		return convertLine(node, state)
	case EIdPolyline:
		return convertPolyline(node)
	case EIdPolygon:
		return convertPolygon(node)
	case EIdPath:
		return convertPathShapes(node)
	}
	return nil, false
}

// convertPathShapes is convert_path in usvg src/parser/shapes.rs.
//
// Port: the Go name takes the module as a suffix, because
// src/parser/converter.rs has a convert_path too (D10).
func convertPathShapes(node svgNode) (*path.Path, bool) {
	value, ok := node.attributeStr(AIdD)
	if !ok {
		return nil, false
	}
	builder := path.NewPathBuilder()
	p := svgtypes.NewSimplifyingPathParser(value)
	for {
		segment, ok, err := p.Next()
		if err != nil {
			node.doc.opt.warnf("Error during path parsing: %v", err)
			break
		}
		if !ok {
			break
		}

		switch segment.Kind {
		case svgtypes.SimplePathSegmentMoveTo:
			builder.MoveTo(float32(segment.X), float32(segment.Y))
		case svgtypes.SimplePathSegmentLineTo:
			builder.LineTo(float32(segment.X), float32(segment.Y))
		case svgtypes.SimplePathSegmentQuadratic:
			builder.QuadTo(float32(segment.X1), float32(segment.Y1), float32(segment.X), float32(segment.Y))
		case svgtypes.SimplePathSegmentCurveTo:
			builder.CubicTo(
				float32(segment.X1), float32(segment.Y1), float32(segment.X2), float32(segment.Y2), float32(segment.X), float32(segment.Y),
			)
		case svgtypes.SimplePathSegmentClosePath:
			builder.Close()
		}
	}

	return builder.Finish()
}

// convertRect is convert_rect in usvg src/parser/shapes.rs.
func convertRect(node svgNode, state *converterState) (*path.Path, bool) {
	// 'width' and 'height' attributes must be positive and non-zero.
	width := node.convertUserLength(AIdWidth, state, svgtypes.LengthZero())
	height := node.convertUserLength(AIdHeight, state, svgtypes.LengthZero())
	if !isValidLength32(width) {
		state.opt.warnf(
			"Rect '%s' has an invalid 'width' value. Skipped.",
			node.elementID(),
		)
		return nil, false
	}
	if !isValidLength32(height) {
		state.opt.warnf(
			"Rect '%s' has an invalid 'height' value. Skipped.",
			node.elementID(),
		)
		return nil, false
	}

	x := node.convertUserLength(AIdX, state, svgtypes.LengthZero())
	y := node.convertUserLength(AIdY, state, svgtypes.LengthZero())

	rx, ry := resolveRxRy(node, state)

	// Clamp rx/ry to the half of the width/height.
	//
	// Should be done only after resolving.
	if rx > width/2.0 {
		rx = width / 2.0
	}
	if ry > height/2.0 {
		ry = height / 2.0
	}

	// Conversion according to https://www.w3.org/TR/SVG11/shapes.html#RectElement
	var p *path.Path
	if ApproxEqUlps32(rx, 0.0, 4) {
		r, ok := path.RectFromXYWH(x, y, width, height)
		if !ok {
			return nil, false
		}
		p = path.PathFromRect(r)
	} else {
		builder := path.NewPathBuilder()
		builder.MoveTo(x+rx, y)

		builder.LineTo(x+width-rx, y)
		arcTo(builder, rx, ry, 0.0, false, true, x+width, y+ry)

		builder.LineTo(x+width, y+height-ry)
		arcTo(builder, rx, ry, 0.0, false, true, x+width-rx, y+height)

		builder.LineTo(x+rx, y+height)
		arcTo(builder, rx, ry, 0.0, false, true, x, y+height-ry)

		builder.LineTo(x, y+ry)
		arcTo(builder, rx, ry, 0.0, false, true, x+rx, y)

		builder.Close()

		var ok bool
		p, ok = builder.Finish()
		if !ok {
			return nil, false
		}
	}

	return p, true
}

// resolveRxRy is resolve_rx_ry in usvg src/parser/shapes.rs.
func resolveRxRy(node svgNode, state *converterState) (float32, float32) {
	rxOpt, hasRx := node.attributeLength(AIdRx)
	ryOpt, hasRy := node.attributeLength(AIdRy)

	// Remove negative values first.
	if hasRx && math.Signbit(rxOpt.Number) {
		hasRx = false
	}
	if hasRy && math.Signbit(ryOpt.Number) {
		hasRy = false
	}

	// Resolve.
	switch {
	case !hasRx && !hasRy:
		return 0.0, 0.0
	case hasRx && !hasRy:
		rx := convertUserLength(rxOpt, node, AIdRx, state)
		return rx, rx
	case !hasRx && hasRy:
		ry := convertUserLength(ryOpt, node, AIdRy, state)
		return ry, ry
	}
	rx := convertUserLength(rxOpt, node, AIdRx, state)
	ry := convertUserLength(ryOpt, node, AIdRy, state)
	return rx, ry
}

// convertLine is convert_line in usvg src/parser/shapes.rs.
func convertLine(node svgNode, state *converterState) (*path.Path, bool) {
	x1 := node.convertUserLength(AIdX1, state, svgtypes.LengthZero())
	y1 := node.convertUserLength(AIdY1, state, svgtypes.LengthZero())
	x2 := node.convertUserLength(AIdX2, state, svgtypes.LengthZero())
	y2 := node.convertUserLength(AIdY2, state, svgtypes.LengthZero())

	builder := path.NewPathBuilder()
	builder.MoveTo(x1, y1)
	builder.LineTo(x2, y2)
	return builder.Finish()
}

// convertPolyline is convert_polyline in usvg src/parser/shapes.rs.
func convertPolyline(node svgNode) (*path.Path, bool) {
	builder, ok := pointsToPath(node, "Polyline")
	if !ok {
		return nil, false
	}
	return builder.Finish()
}

// convertPolygon is convert_polygon in usvg src/parser/shapes.rs.
func convertPolygon(node svgNode) (*path.Path, bool) {
	builder, ok := pointsToPath(node, "Polygon")
	if !ok {
		return nil, false
	}
	builder.Close()
	return builder.Finish()
}

// pointsToPath is points_to_path in usvg src/parser/shapes.rs.
func pointsToPath(node svgNode, eid string) (*path.PathBuilder, bool) {
	builder := path.NewPathBuilder()
	text, ok := node.attributeStr(AIdPoints)
	if !ok {
		node.doc.opt.warnf(
			"%s '%s' has an invalid 'points' value. Skipped.",
			eid,
			node.elementID(),
		)
		return nil, false
	}
	for x, y := range svgtypes.NewPointsParser(text).All() {
		if builder.IsEmpty() {
			builder.MoveTo(float32(x), float32(y))
		} else {
			builder.LineTo(float32(x), float32(y))
		}
	}

	// 'polyline' and 'polygon' elements must contain at least 2 points.
	if builder.Len() < 2 {
		node.doc.opt.warnf(
			"%s '%s' has less than 2 points. Skipped.",
			eid,
			node.elementID(),
		)
		return nil, false
	}

	return builder, true
}

// convertCircle is convert_circle in usvg src/parser/shapes.rs.
func convertCircle(node svgNode, state *converterState) (*path.Path, bool) {
	cx := node.convertUserLength(AIdCx, state, svgtypes.LengthZero())
	cy := node.convertUserLength(AIdCy, state, svgtypes.LengthZero())
	r := node.convertUserLength(AIdR, state, svgtypes.LengthZero())

	if !isValidLength32(r) {
		state.opt.warnf(
			"Circle '%s' has an invalid 'r' value. Skipped.",
			node.elementID(),
		)
		return nil, false
	}

	return ellipseToPath(cx, cy, r, r)
}

// convertEllipse is convert_ellipse in usvg src/parser/shapes.rs.
func convertEllipse(node svgNode, state *converterState) (*path.Path, bool) {
	cx := node.convertUserLength(AIdCx, state, svgtypes.LengthZero())
	cy := node.convertUserLength(AIdCy, state, svgtypes.LengthZero())
	rx, ry := resolveRxRy(node, state)

	if !isValidLength32(rx) {
		state.opt.warnf(
			"Ellipse '%s' has an invalid 'rx' value. Skipped.",
			node.elementID(),
		)
		return nil, false
	}

	if !isValidLength32(ry) {
		state.opt.warnf(
			"Ellipse '%s' has an invalid 'ry' value. Skipped.",
			node.elementID(),
		)
		return nil, false
	}

	return ellipseToPath(cx, cy, rx, ry)
}

// ellipseToPath is ellipse_to_path in usvg src/parser/shapes.rs.
func ellipseToPath(cx, cy, rx, ry float32) (*path.Path, bool) {
	builder := path.NewPathBuilder()
	builder.MoveTo(cx+rx, cy)
	arcTo(builder, rx, ry, 0.0, false, true, cx, cy+ry)
	arcTo(builder, rx, ry, 0.0, false, true, cx-rx, cy)
	arcTo(builder, rx, ry, 0.0, false, true, cx, cy-ry)
	arcTo(builder, rx, ry, 0.0, false, true, cx+rx, cy)
	builder.Close()
	return builder.Finish()
}

// arcTo is PathBuilderExt::arc_to for tiny_skia_path::PathBuilder in usvg
// src/parser/shapes.rs.
//
// Port: the private trait PathBuilderExt becomes a function that takes the
// builder.
func arcTo(
	pb *path.PathBuilder,
	rx float32,
	ry float32,
	xAxisRotation float32, //nolint:unparam // a parameter of the Rust trait method
	largeArc bool, //nolint:unparam // a parameter of the Rust trait method
	sweep bool, //nolint:unparam // a parameter of the Rust trait method
	x float32,
	y float32,
) {
	prev, ok := pb.LastPoint()
	if !ok {
		return
	}

	svgArc := kurbo.SvgArc{
		From:      kurbo.NewPoint(float64(prev.X), float64(prev.Y)),
		To:        kurbo.NewPoint(float64(x), float64(y)),
		Radii:     kurbo.NewVec2(float64(rx), float64(ry)),
		XRotation: float64(xAxisRotation) * (math.Pi / 180.0),
		LargeArc:  largeArc,
		Sweep:     sweep,
	}

	arc, ok := kurbo.FromSvgArc(&svgArc)
	if !ok {
		pb.LineTo(x, y)
		return
	}
	arc.ToCubicBeziers(0.1, func(p1, p2, p kurbo.Point) {
		pb.CubicTo(
			float32(p1.X),
			float32(p1.Y),
			float32(p2.X),
			float32(p2.Y),
			float32(p.X),
			float32(p.Y),
		)
	})
}
