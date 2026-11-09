// Copyright 2011 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"log/slog"
	"sync"

	"github.com/xo/okage/path"
)

// combine is Combine in tiny-skia src/edge_builder.rs.
type combine uint8

// The values of combine.
const (
	combineNo combine = iota
	combinePartial
	combineTotal
)

// shiftedIntRect is ShiftedIntRect in tiny-skia src/edge_builder.rs.
type shiftedIntRect struct {
	shifted screenIntRect
	shift   int32
}

// newShiftedIntRect is ShiftedIntRect::new in tiny-skia src/edge_builder.rs.
func newShiftedIntRect(rect *screenIntRect, shift int32) (shiftedIntRect, bool) {
	shifted, ok := screenIntRectFromXYWH(
		rect.X()<<shift,
		rect.Y()<<shift,
		rect.Width()<<shift,
		rect.Height()<<shift,
	)
	if !ok {
		return shiftedIntRect{}, false
	}
	return shiftedIntRect{shifted: shifted, shift: shift}, true
}

// shiftedRect is ShiftedIntRect::shifted in tiny-skia src/edge_builder.rs.
//
// Port: the field has the name shifted, so the method has another name.
func (r *shiftedIntRect) shiftedRect() *screenIntRect {
	return &r.shifted
}

// recover is ShiftedIntRect::recover in tiny-skia src/edge_builder.rs.
func (r *shiftedIntRect) recover() screenIntRect {
	rect, ok := screenIntRectFromXYWH(
		r.shifted.X()>>r.shift,
		r.shifted.Y()>>r.shift,
		r.shifted.Width()>>r.shift,
		r.shifted.Height()>>r.shift,
	)
	if !ok {
		// cannot fail, because the original rect was valid
		panic("called `Option::unwrap()` on a `None` value")
	}
	return rect
}

// basicEdgeBuilder is BasicEdgeBuilder in tiny-skia src/edge_builder.rs.
type basicEdgeBuilder struct {
	edges     []edge
	clipShift int32
}

// newBasicEdgeBuilder is BasicEdgeBuilder::new in tiny-skia
// src/edge_builder.rs.
//
// Port: the edges come from edgePool, so that a fill does not allocate a new
// buffer for each path. The capacity is 64 or more, as upstream asks.
func newBasicEdgeBuilder(clipShift int32) *basicEdgeBuilder {
	return &basicEdgeBuilder{
		edges:     getEdges(), // TODO: stack array + fallback
		clipShift: clipShift,
	}
}

// edgePool holds the edge buffers of fills that are done. It ports no item of
// upstream: Rust allocates a Vec for each fill, which is cheap there, and in
// Go the garbage of a buffer for each path is a large part of the time of a
// file with many paths.
var edgePool = sync.Pool{
	New: func() any {
		s := make([]edge, 0, 64)
		return &s
	},
}

// getEdges returns an empty edge buffer from edgePool.
func getEdges() []edge {
	if s, ok := edgePool.Get().(*[]edge); ok {
		return (*s)[:0]
	}
	return make([]edge, 0, 64)
}

// putEdges returns the buffer of edges to edgePool. The caller must not use
// edges afterward.
func putEdges(edges []edge) {
	if cap(edges) == 0 {
		return
	}
	edges = edges[:0]
	edgePool.Put(&edges)
}

// buildEdges is BasicEdgeBuilder::build_edges in tiny-skia
// src/edge_builder.rs.
//
// Skia returns a linked list here, but it is a nightmare to use in Rust, so
// we are mimicking it with Vec.
//
// Port: upstream logs a warning with log::warn!. Go logs it to logger, as D20
// says. A nil logger logs nothing.
func buildEdges(p *Path, clip *shiftedIntRect, clipShift int32, logger *slog.Logger) ([]edge, bool) {
	// If we are convex, then we need both edges, even if the right edge is past the clip.
	// let can_cull_to_the_right = !path.isConvex();
	canCullToTheRight := false // TODO: this

	builder := newBasicEdgeBuilder(clipShift)
	if !builder.build(p, clip, canCullToTheRight) {
		warn(logger, "infinite or NaN segments detected during edges building")
		putEdges(builder.edges)
		return nil, false
	}

	if len(builder.edges) < 2 {
		putEdges(builder.edges)
		return nil, false
	}

	return builder.edges, true
}

// build is BasicEdgeBuilder::build in tiny-skia src/edge_builder.rs.
//
// TODO: build_poly.
func (b *basicEdgeBuilder) build(p *Path, clip *shiftedIntRect, canCullToTheRight bool) bool {
	if clip != nil {
		clipRect := clip.recover().ToRect()
		it := newEdgeClipperIter(p, clipRect, canCullToTheRight)
		for {
			edges, ok := it.next()
			if !ok {
				break
			}
			for _, e := range edges.slice() {
				switch e.kind {
				case pathEdgeKindLineTo:
					if !e.points[0].IsFinite() || !e.points[1].IsFinite() {
						return false
					}

					b.pushLine(&[2]Point{e.points[0], e.points[1]})
				case pathEdgeKindQuadTo:
					if !e.points[0].IsFinite() || !e.points[1].IsFinite() || !e.points[2].IsFinite() {
						return false
					}

					b.pushQuad(e.points[0:3])
				case pathEdgeKindCubicTo:
					if !e.points[0].IsFinite() ||
						!e.points[1].IsFinite() ||
						!e.points[2].IsFinite() ||
						!e.points[3].IsFinite() {
						return false
					}

					b.pushCubic(e.points[0:4])
				}
			}
		}
	} else {
		it := edgeIter(p)
		for {
			e, ok := it.next()
			if !ok {
				break
			}
			switch e.kind {
			case pathEdgeKindLineTo:
				b.pushLine(&[2]Point{e.points[0], e.points[1]})
			case pathEdgeKindQuadTo:
				points := [3]Point{e.points[0], e.points[1], e.points[2]}
				var monoX [5]Point
				n := chopQuadAtYExtrema(&points, &monoX)
				for i := 0; i <= n; i++ {
					b.pushQuad(monoX[i*2:])
				}
			case pathEdgeKindCubicTo:
				points := [4]Point{e.points[0], e.points[1], e.points[2], e.points[3]}
				var monoY [10]Point
				n := chopCubicAtYExtrema(&points, &monoY)
				for i := 0; i <= n; i++ {
					b.pushCubic(monoY[i*3:])
				}
			}
		}
	}

	return true
}

// pushLine is BasicEdgeBuilder::push_line in tiny-skia src/edge_builder.rs.
func (b *basicEdgeBuilder) pushLine(points *[2]Point) {
	e, ok := newLineEdge(points[0], points[1], b.clipShift)
	if !ok {
		return
	}

	c := combineNo
	if e.isVertical() && len(b.edges) != 0 {
		if last := &b.edges[len(b.edges)-1]; last.kind == edgeKindLine {
			c = combineVertical(&e, &last.line)
		}
	}

	switch c {
	case combineTotal:
		b.edges = b.edges[:len(b.edges)-1]
	case combinePartial:
	case combineNo:
		b.edges = append(b.edges, edge{kind: edgeKindLine, line: e})
	}
}

// pushQuad is BasicEdgeBuilder::push_quad in tiny-skia src/edge_builder.rs.
func (b *basicEdgeBuilder) pushQuad(points []Point) {
	if e, ok := newQuadraticEdge(points, b.clipShift); ok {
		b.edges = append(b.edges, edge{kind: edgeKindQuadratic, quad: e})
	}
}

// pushCubic is BasicEdgeBuilder::push_cubic in tiny-skia src/edge_builder.rs.
func (b *basicEdgeBuilder) pushCubic(points []Point) {
	if e, ok := newCubicEdge(points, b.clipShift); ok {
		b.edges = append(b.edges, edge{kind: edgeKindCubic, cubic: e})
	}
}

// combineVertical is combine_vertical in tiny-skia src/edge_builder.rs.
func combineVertical(e, last *lineEdge) combine {
	if last.dx != 0 || e.x != last.x {
		return combineNo
	}

	if e.winding == last.winding {
		switch {
		case e.lastY+1 == last.firstY:
			last.firstY = e.firstY
			return combinePartial
		case e.firstY == last.lastY+1:
			last.lastY = e.lastY
			return combinePartial
		}
		return combineNo
	}

	if e.firstY == last.firstY {
		switch {
		case e.lastY == last.lastY:
			return combineTotal
		case e.lastY < last.lastY:
			last.firstY = e.lastY + 1
			return combinePartial
		}
		last.firstY = last.lastY + 1
		last.lastY = e.lastY
		last.winding = e.winding
		return combinePartial
	}

	if e.lastY == last.lastY {
		if e.firstY > last.firstY {
			last.lastY = e.firstY - 1
		} else {
			last.lastY = last.firstY - 1
			last.firstY = e.firstY
			last.winding = e.winding
		}

		return combinePartial
	}

	return combineNo
}

// edgeIter is edge_iter in tiny-skia src/edge_builder.rs.
func edgeIter(p *Path) pathEdgeIter {
	return pathEdgeIter{
		path:           p,
		verbIndex:      0,
		pointsIndex:    0,
		moveTo:         path.PointZero(),
		needsCloseLine: false,
	}
}

// pathEdgeKind is the variant of the enum PathEdge in tiny-skia
// src/edge_builder.rs.
type pathEdgeKind uint8

// The values of pathEdgeKind.
const (
	pathEdgeKindLineTo pathEdgeKind = iota
	pathEdgeKindQuadTo
	pathEdgeKindCubicTo
)

// pathEdge is PathEdge in tiny-skia src/edge_builder.rs.
//
// Port: the Rust enum carries data, so it becomes a struct with a kind field,
// as path.PathSegment does. points holds two points for LineTo, three for
// QuadTo and four for CubicTo. The other points are zero, so that two edges
// compare equal with ==.
type pathEdge struct {
	kind   pathEdgeKind
	points [4]Point
}

// pathEdgeLineTo is PathEdge::LineTo in tiny-skia src/edge_builder.rs.
func pathEdgeLineTo(p0, p1 Point) pathEdge {
	return pathEdge{kind: pathEdgeKindLineTo, points: [4]Point{p0, p1}}
}

// pathEdgeQuadTo is PathEdge::QuadTo in tiny-skia src/edge_builder.rs.
func pathEdgeQuadTo(p0, p1, p2 Point) pathEdge {
	return pathEdge{kind: pathEdgeKindQuadTo, points: [4]Point{p0, p1, p2}}
}

// pathEdgeCubicTo is PathEdge::CubicTo in tiny-skia src/edge_builder.rs.
func pathEdgeCubicTo(p0, p1, p2, p3 Point) pathEdge {
	return pathEdge{kind: pathEdgeKindCubicTo, points: [4]Point{p0, p1, p2, p3}}
}

// pathEdgeIter is PathEdgeIter in tiny-skia src/edge_builder.rs.
//
// Lightweight variant of PathIter that only returns segments (e.g.
// lines/quads).
//
// Does not return Move or Close. Always "auto-closes" each contour.
type pathEdgeIter struct {
	path           *Path
	verbIndex      int
	pointsIndex    int
	moveTo         Point
	needsCloseLine bool
}

// closeLine is PathEdgeIter::close_line in tiny-skia src/edge_builder.rs.
func (it *pathEdgeIter) closeLine() (pathEdge, bool) {
	it.needsCloseLine = false

	e := pathEdgeLineTo(it.path.Points()[it.pointsIndex-1], it.moveTo)
	return e, true
}

// next is the Iterator impl of PathEdgeIter in tiny-skia src/edge_builder.rs.
//
// Port: upstream calls self.next() again after a Move or a Close that ends no
// contour. Go loops instead, with the same result.
func (it *pathEdgeIter) next() (pathEdge, bool) {
	for {
		verbs := it.path.Verbs()
		points := it.path.Points()
		if it.verbIndex >= len(verbs) {
			if it.needsCloseLine {
				return it.closeLine()
			}
			return pathEdge{}, false
		}

		verb := verbs[it.verbIndex]
		it.verbIndex++

		switch verb {
		case path.PathVerbMove:
			if it.needsCloseLine {
				res, ok := it.closeLine()
				it.moveTo = points[it.pointsIndex]
				it.pointsIndex++
				return res, ok
			}

			it.moveTo = points[it.pointsIndex]
			it.pointsIndex++
			continue
		case path.PathVerbClose:
			if it.needsCloseLine {
				return it.closeLine()
			}

			continue
		}

		// Actual edge.
		it.needsCloseLine = true

		var e pathEdge
		switch verb {
		case path.PathVerbLine:
			e = pathEdgeLineTo(
				points[it.pointsIndex-1],
				points[it.pointsIndex+0],
			)
			it.pointsIndex++
		case path.PathVerbQuad:
			e = pathEdgeQuadTo(
				points[it.pointsIndex-1],
				points[it.pointsIndex+0],
				points[it.pointsIndex+1],
			)
			it.pointsIndex += 2
		case path.PathVerbCubic:
			e = pathEdgeCubicTo(
				points[it.pointsIndex-1],
				points[it.pointsIndex+0],
				points[it.pointsIndex+1],
				points[it.pointsIndex+2],
			)
			it.pointsIndex += 3
		default:
			panic("internal error: entered unreachable code")
		}

		return e, true
	}
}
