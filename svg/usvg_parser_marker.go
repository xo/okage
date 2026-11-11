// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"math"

	"github.com/xo/okage/internal/crmath"
	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/parser/marker.rs.

// segmentKind is the case of a segment.
type segmentKind uint8

// The cases of the enum Segment in usvg src/parser/marker.rs.
const (
	segmentMoveTo segmentKind = iota
	segmentLineTo
	segmentCubicTo
	segmentClose
)

// segment is Segment in usvg src/parser/marker.rs.
//
// Similar to `tiny_skia_path::PathSegment`, but without the `QuadTo`.
//
// Port: each case holds points, so the enum becomes a struct with a kind
// (D27). MoveTo and LineTo hold one point in p[0], CubicTo holds three, and
// Close holds none.
type segment struct {
	kind segmentKind
	p    [3]path.Point
}

// isValid is is_valid in usvg src/parser/marker.rs.
func isValid(node svgNode) bool {
	// `marker-*` attributes cannot be set on shapes inside a `clipPath`.
	for n := range node.ancestors() {
		if n.hasTagName(EIdClipPath) {
			return false
		}
	}

	_, start := nodeFindAttribute(node, AIdMarkerStart, fromValueSvgNode)
	_, mid := nodeFindAttribute(node, AIdMarkerMid, fromValueSvgNode)
	_, end := nodeFindAttribute(node, AIdMarkerEnd, fromValueSvgNode)
	return start || mid || end
}

// convertMarker is convert in usvg src/parser/marker.rs.
func convertMarker(
	node svgNode,
	p *path.Path,
	state *converterState,
	cache *converterCache,
	parent *Group,
) {
	list := [...]struct {
		aid  AId
		kind markerKind
	}{
		{AIdMarkerStart, markerKindStart},
		{AIdMarkerMid, markerKindMiddle},
		{AIdMarkerEnd, markerKindEnd},
	}

	for _, item := range list {
		var marker svgNode
		hasMarker := false
		if link, ok := nodeFindAttribute(node, item.aid, fromValueSvgNode); ok {
			if link.hasTagName(EIdMarker) {
				marker, hasMarker = link, true
			}
		}

		if hasMarker {
			resolveMarker(node, p, marker, item.kind, state, cache, parent)
		}
	}
}

// markerKind is MarkerKind in usvg src/parser/marker.rs.
type markerKind uint8

// The values of markerKind.
const (
	markerKindStart markerKind = iota
	markerKindMiddle
	markerKindEnd
)

// markerOrientationKind is the case of a markerOrientation.
type markerOrientationKind uint8

// The cases of the enum MarkerOrientation in usvg src/parser/marker.rs.
const (
	markerOrientationAuto markerOrientationKind = iota
	markerOrientationAutoStartReverse
	markerOrientationAngle
)

// markerOrientation is MarkerOrientation in usvg src/parser/marker.rs.
//
// Port: only the case Angle holds data, so the enum becomes a struct with a
// kind (D27).
type markerOrientation struct {
	kind markerOrientationKind
	// angle is the value of the case Angle.
	angle float32
}

// resolveMarker is resolve in usvg src/parser/marker.rs.
//
// Port: upstream returns Option<()>, which no caller reads. Go returns
// nothing.
func resolveMarker(
	shapeNode svgNode,
	p *path.Path,
	markerNode svgNode,
	kind markerKind,
	state *converterState,
	cache *converterCache,
	parent *Group,
) {
	strokeScaleValue, ok := strokeScale(shapeNode, markerNode, state)
	if !ok {
		return
	}
	scale := strokeScaleValue.Get()

	r, ok := convertRectMarker(markerNode, state)
	if !ok {
		return
	}

	viewBoxTs := path.DefaultTransform()
	if vb, ok := markerNode.parseViewbox(); ok {
		aspect, ok := nodeAttribute(markerNode, AIdPreserveAspectRatio, fromValueAspectRatio)
		if !ok {
			aspect = svgtypes.DefaultAspectRatio()
		}
		viewBoxTs = viewBox{
			rect:   vb,
			aspect: aspect,
		}.toTransform(r.Size())
	}

	var hasOverflow bool
	{
		overflow, ok := markerNode.attributeStr(AIdOverflow)
		// `overflow` is `hidden` by default.
		hasOverflow = !ok || overflow == "hidden" || overflow == "scroll"
	}

	var clipPath *ClipPath
	if hasOverflow {
		inv, ok := viewBoxTs.Invert()
		if !ok {
			return
		}
		clipRect, ok := r.Size().ToNonZeroRect(0.0, 0.0).Transform(inv)
		if !ok {
			return
		}

		clipPath = emptyClipPath(cache.genClipPathID())

		clipShape := newSimplePath(path.PathFromRect(clipRect.ToRect()))
		fill := DefaultFill()
		clipShape.fill = &fill

		clipPath.root.children = append(clipPath.root.children, clipShape)
	}

	// TODO: avoid allocation
	segments := make([]segment, 0, p.Len())
	prev := path.PointZero()
	prevMove := path.PointZero()
	for it := p.Segments(); ; {
		seg, ok := it.Next()
		if !ok {
			break
		}
		switch seg.Kind {
		case path.PathSegmentMoveTo:
			pt := seg.Points[0]
			segments = append(segments, segment{kind: segmentMoveTo, p: [3]path.Point{pt}})
			prev = pt
			prevMove = pt
		case path.PathSegmentLineTo:
			pt := seg.Points[0]
			segments = append(segments, segment{kind: segmentLineTo, p: [3]path.Point{pt}})
			prev = pt
		case path.PathSegmentQuadTo:
			p1, p2, pt := quadToCurve(prev, seg.Points[0], seg.Points[1])
			segments = append(segments, segment{kind: segmentCubicTo, p: [3]path.Point{p1, p2, pt}})
			prev = pt
		case path.PathSegmentCubicTo:
			pt := seg.Points[2]
			segments = append(segments, segment{kind: segmentCubicTo, p: seg.Points})
			prev = pt
		case path.PathSegmentClose:
			segments = append(segments, segment{kind: segmentClose})
			prev = prevMove
		}
	}

	drawMarker := func(pt path.Point, idx int) {
		ts := path.TransformFromTranslate(pt.X, pt.Y)

		var angle float32
		orientation := convertOrientation(markerNode)
		switch {
		case orientation.kind == markerOrientationAutoStartReverse && idx == 0:
			angle = float32(math.Mod(float64(calcVertexAngle(segments, idx)+180.0), 360.0))
		case orientation.kind == markerOrientationAuto || orientation.kind == markerOrientationAutoStartReverse:
			angle = calcVertexAngle(segments, idx)
		default:
			angle = orientation.angle
		}

		if !ApproxZeroUlps32(angle, 4) {
			ts = ts.PreRotate(angle)
		}

		sx, sy := viewBoxTs.GetScale()
		ts = ts.PreScale(sx*scale, sy*scale)
		ts = ts.PreTranslate(-r.X(), -r.Y())

		// TODO: do not create a group when no clipPath
		g := emptyGroup()
		g.transform = ts
		g.absTransform = parent.absTransform.PreConcat(ts)
		g.clipPath = clipPath

		markerState := state.clone()
		markerState.insideMarker = true
		convertChildren(markerNode, markerState, cache, g)

		if g.HasChildren() {
			parent.children = append(parent.children, g)
		}
	}

	drawMarkers(segments, kind, drawMarker)
}

// strokeScale is stroke_scale in usvg src/parser/marker.rs.
func strokeScale(
	pathNode svgNode,
	markerNode svgNode,
	state *converterState,
) (NonZeroPositiveF32, bool) {
	if v, ok := markerNode.attributeStr(AIdMarkerUnits); ok && v == "userSpaceOnUse" { //nolint:goconst // an SVG keyword, as upstream writes it
		return path.NewNonZeroPositiveF32(1.0)
	}
	return pathNode.resolveValidLength(AIdStrokeWidth, state, 1.0)
}

// drawMarkers is draw_markers in usvg src/parser/marker.rs.
func drawMarkers(p []segment, kind markerKind, drawMarker func(path.Point, int)) {
	switch kind {
	case markerKindStart:
		if len(p) > 0 && p[0].kind == segmentMoveTo {
			drawMarker(p[0].p[0], 0)
		}
	case markerKindMiddle:
		total := len(p) - 1
		i := 1
		for i < total {
			var pt path.Point
			switch p[i].kind {
			case segmentMoveTo, segmentLineTo:
				pt = p[i].p[0]
			case segmentCubicTo:
				pt = p[i].p[2]
			default:
				i++
				continue
			}

			drawMarker(pt, i)

			i++
		}
	case markerKindEnd:
		idx := len(p) - 1
		if len(p) == 0 {
			return
		}
		last := p[len(p)-1]
		switch last.kind {
		case segmentLineTo:
			drawMarker(last.p[0], idx)
		case segmentCubicTo:
			drawMarker(last.p[2], idx)
		case segmentClose:
			pt := getSubpathStart(p, idx)
			drawMarker(pt, idx)
		}
	}
}

// calcVertexAngle is calc_vertex_angle in usvg src/parser/marker.rs.
func calcVertexAngle(p []segment, idx int) float32 {
	if idx == 0 {
		// First segment.

		seg1 := p[0]
		seg2 := p[1]

		switch {
		case seg1.kind == segmentMoveTo && seg2.kind == segmentLineTo:
			pm, pt := seg1.p[0], seg2.p[0]
			return calcLineAngle(pm.X, pm.Y, pt.X, pt.Y)
		case seg1.kind == segmentMoveTo && seg2.kind == segmentCubicTo:
			pm, p1, pt := seg1.p[0], seg2.p[0], seg2.p[2]
			if ApproxEqUlps32(pm.X, p1.X, 4) && ApproxEqUlps32(pm.Y, p1.Y, 4) {
				return calcLineAngle(pm.X, pm.Y, pt.X, pt.Y)
			}
			return calcLineAngle(pm.X, pm.Y, p1.X, p1.Y)
		}
		return 0.0
	} else if idx == len(p)-1 {
		// Last segment.

		seg1 := p[idx-1]
		seg2 := p[idx]

		switch {
		case seg2.kind == segmentMoveTo:
			return 0.0 // unreachable
		case seg2.kind == segmentLineTo:
			pt := seg2.p[0]
			prev := getPrevVertex(p, idx)
			return calcLineAngle(prev.X, prev.Y, pt.X, pt.Y)
		case seg2.kind == segmentCubicTo:
			p1, p2, pt := seg2.p[0], seg2.p[1], seg2.p[2]
			if ApproxEqUlps32(p2.X, pt.X, 4) && ApproxEqUlps32(p2.Y, pt.Y, 4) {
				return calcLineAngle(p1.X, p1.Y, pt.X, pt.Y)
			}
			return calcLineAngle(p2.X, p2.Y, pt.X, pt.Y)
		case seg1.kind == segmentLineTo && seg2.kind == segmentClose:
			pt := seg1.p[0]
			next := getSubpathStart(p, idx)
			return calcLineAngle(pt.X, pt.Y, next.X, next.Y)
		case seg1.kind == segmentCubicTo && seg2.kind == segmentClose:
			p2, pt := seg1.p[1], seg1.p[2]
			prev := getPrevVertex(p, idx)
			next := getSubpathStart(p, idx)
			return calcCurvesAngle(
				prev.X, prev.Y, p2.X, p2.Y, pt.X, pt.Y, next.X, next.Y, next.X, next.Y,
			)
		}
		return 0.0 // (_, Segment::Close)
	}

	// Middle segments.

	seg1 := p[idx]
	seg2 := p[idx+1]

	// TODO: Not sure if there is a better way.
	switch {
	case seg1.kind == segmentMoveTo && seg2.kind == segmentLineTo:
		pm, pt := seg1.p[0], seg2.p[0]
		return calcLineAngle(pm.X, pm.Y, pt.X, pt.Y)
	case seg1.kind == segmentMoveTo && seg2.kind == segmentCubicTo:
		pm, p1 := seg1.p[0], seg2.p[0]
		return calcLineAngle(pm.X, pm.Y, p1.X, p1.Y)
	case seg1.kind == segmentLineTo && seg2.kind == segmentLineTo:
		p1, p2 := seg1.p[0], seg2.p[0]
		prev := getPrevVertex(p, idx)
		return calcAngle(prev.X, prev.Y, p1.X, p1.Y, p1.X, p1.Y, p2.X, p2.Y)
	case seg1.kind == segmentCubicTo && seg2.kind == segmentCubicTo:
		c1P2, c1P := seg1.p[1], seg1.p[2]
		c2P1, c2P := seg2.p[0], seg2.p[2]
		prev := getPrevVertex(p, idx)
		return calcCurvesAngle(
			prev.X, prev.Y, c1P2.X, c1P2.Y, c1P.X, c1P.Y, c2P1.X, c2P1.Y, c2P.X,
			c2P.Y,
		)
	case seg1.kind == segmentLineTo && seg2.kind == segmentCubicTo:
		pl := seg1.p[0]
		p1, pt := seg2.p[0], seg2.p[2]
		prev := getPrevVertex(p, idx)
		return calcCurvesAngle(
			prev.X, prev.Y, prev.X, prev.Y, pl.X, pl.Y, p1.X, p1.Y, pt.X, pt.Y,
		)
	case seg1.kind == segmentCubicTo && seg2.kind == segmentLineTo:
		p2, pt := seg1.p[1], seg1.p[2]
		pl := seg2.p[0]
		prev := getPrevVertex(p, idx)
		return calcCurvesAngle(prev.X, prev.Y, p2.X, p2.Y, pt.X, pt.Y, pl.X, pl.Y, pl.X, pl.Y)
	case seg1.kind == segmentLineTo && seg2.kind == segmentMoveTo:
		pt := seg1.p[0]
		prev := getPrevVertex(p, idx)
		return calcLineAngle(prev.X, prev.Y, pt.X, pt.Y)
	case seg1.kind == segmentCubicTo && seg2.kind == segmentMoveTo:
		p2, pt := seg1.p[1], seg1.p[2]
		if ApproxEqUlps32(pt.X, p2.X, 4) && ApproxEqUlps32(pt.Y, p2.Y, 4) {
			prev := getPrevVertex(p, idx)
			return calcLineAngle(prev.X, prev.Y, pt.X, pt.Y)
		}
		return calcLineAngle(p2.X, p2.Y, pt.X, pt.Y)
	case seg1.kind == segmentLineTo && seg2.kind == segmentClose:
		pt := seg1.p[0]
		prev := getPrevVertex(p, idx)
		next := getSubpathStart(p, idx)
		return calcAngle(prev.X, prev.Y, pt.X, pt.Y, pt.X, pt.Y, next.X, next.Y)
	case seg2.kind == segmentClose:
		prev := getPrevVertex(p, idx)
		next := getSubpathStart(p, idx)
		return calcLineAngle(prev.X, prev.Y, next.X, next.Y)
	}
	// (_, Segment::MoveTo(_)) | (Segment::Close, _)
	return 0.0
}

// calcLineAngle is calc_line_angle in usvg src/parser/marker.rs.
func calcLineAngle(x1, y1, x2, y2 float32) float32 {
	return calcAngle(x1, y1, x2, y2, x1, y1, x2, y2)
}

// calcCurvesAngle is calc_curves_angle in usvg src/parser/marker.rs.
func calcCurvesAngle(
	px, py float32, // previous vertex
	cx1, cy1 float32, // previous control point
	x, y float32, // current vertex
	cx2, cy2 float32, // next control point
	nx, ny float32, // next vertex
) float32 {
	switch {
	case ApproxEqUlps32(cx1, x, 4) && ApproxEqUlps32(cy1, y, 4):
		return calcAngle(px, py, x, y, x, y, cx2, cy2)
	case ApproxEqUlps32(x, cx2, 4) && ApproxEqUlps32(y, cy2, 4):
		return calcAngle(cx1, cy1, x, y, x, y, nx, ny)
	}
	return calcAngle(cx1, cy1, x, y, x, y, cx2, cy2)
}

// The constants of std::f32::consts of Rust that calc_angle uses.
const (
	pi32       float32 = math.Pi
	fracPi2F32 float32 = math.Pi / 2
	// degsPerRadF32 is the constant of f32::to_degrees of Rust.
	degsPerRadF32 float32 = 57.2957795130823208767981548141051703
)

// calcAngle is calc_angle in usvg src/parser/marker.rs.
func calcAngle(x1, y1, x2, y2, x3, y3, x4, y4 float32) float32 {
	normalize := func(rad float32) float32 {
		v := float32(math.Mod(float64(rad), float64(pi32*2.0)))
		if v < 0.0 {
			return v + pi32*2.0
		}
		return v
	}

	vectorAngle := func(vx, vy float32) float32 {
		rad := crmath.Atan2_32(vy, vx)
		if rad != rad {
			return 0.0
		}
		return normalize(rad)
	}

	inA := vectorAngle(x2-x1, y2-y1)
	outA := vectorAngle(x4-x3, y4-y3)
	d := (outA - inA) * 0.5

	angle := inA + d
	if fracPi2F32 < float32(math.Abs(float64(d))) {
		angle -= pi32
	}

	return normalize(angle) * degsPerRadF32
}

// getSubpathStart is get_subpath_start in usvg src/parser/marker.rs.
func getSubpathStart(segments []segment, idx int) path.Point {
	offset := len(segments) - idx
	for i := len(segments) - 1 - offset; i >= 0; i-- {
		if segments[i].kind == segmentMoveTo {
			return segments[i].p[0]
		}
	}

	return path.PointZero()
}

// getPrevVertex is get_prev_vertex in usvg src/parser/marker.rs.
func getPrevVertex(segments []segment, idx int) path.Point {
	seg := segments[idx-1]
	switch seg.kind {
	case segmentMoveTo, segmentLineTo:
		return seg.p[0]
	case segmentCubicTo:
		return seg.p[2]
	}
	return getSubpathStart(segments, idx)
}

// convertRectMarker is convert_rect in usvg src/parser/marker.rs.
func convertRectMarker(node svgNode, state *converterState) (NonZeroRect, bool) {
	return path.NonZeroRectFromXYWH(
		node.convertUserLength(AIdRefX, state, svgtypes.LengthZero()),
		node.convertUserLength(AIdRefY, state, svgtypes.LengthZero()),
		node.convertUserLength(AIdMarkerWidth, state, svgtypes.NewLengthNumber(3.0)),
		node.convertUserLength(AIdMarkerHeight, state, svgtypes.NewLengthNumber(3.0)),
	)
}

// convertOrientation is convert_orientation in usvg src/parser/marker.rs.
func convertOrientation(node svgNode) markerOrientation {
	v, ok := node.attributeStr(AIdOrient)
	switch {
	case ok && v == "auto": //nolint:goconst // an SVG keyword, as upstream writes it
		return markerOrientation{kind: markerOrientationAuto}
	case ok && v == "auto-start-reverse":
		return markerOrientation{kind: markerOrientationAutoStartReverse}
	}
	if angle, ok := nodeAttribute(node, AIdOrient, fromValueAngle); ok {
		return markerOrientation{kind: markerOrientationAngle, angle: float32(angle.ToDegrees())}
	}
	return markerOrientation{kind: markerOrientationAngle, angle: 0.0}
}

// quadToCurve is quad_to_curve in usvg src/parser/marker.rs.
func quadToCurve(prev, p1, p path.Point) (path.Point, path.Point, path.Point) {
	calc := func(n1, n2 float32) float32 {
		return (n1 + n2*2.0) / 3.0
	}

	return path.PointFromXY(calc(prev.X, p1.X), calc(prev.Y, p1.Y)),
		path.PointFromXY(calc(p.X, p1.X), calc(p.Y, p1.Y)),
		p
}
