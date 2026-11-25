// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package pathbool computes the geometric boolean operations of paths: the
// union, the difference, the intersection and the exclusion of the areas
// that they fill. lottie uses it for the result of a merge path that a
// stroke or a trim path draws (D94). It ports no upstream code. Neither
// kurbo nor ThorVG has such operations.
//
// Combine flattens the curves of each operand with kurbo.Flatten, and snaps
// the points to a grid of integers, so that each test of the geometry is
// exact. It splits the edges where they cross or touch, and merges the edges
// that lie on each other. For each edge, it counts the winding number of
// each operand on both sides, with a vertical ray. An edge is part of the
// result when the operation gives the inside on one side and the outside on
// the other. Combine then links these edges into closed contours, with the
// inside on their left.
package pathbool

import (
	"math"
	"slices"

	"github.com/xo/okage/kurbo"
)

// Op is a boolean operation of Combine.
type Op uint8

// The values of Op.
const (
	// Union is the area that any operand covers.
	Union Op = iota
	// Difference is the area of the first operand without the areas of the
	// operands after it.
	Difference
	// Intersection is the area that all operands cover.
	Intersection
	// Xor is the area that an odd number of operands covers.
	Xor
)

// gridSteps is the number of steps of the grid in the tolerance of Combine.
// A point moves by at most half a step when Combine snaps it to the grid.
const gridSteps = 16

// maxGrid is the largest coordinate on the grid. With it, a cross product
// of two differences of doubled coordinates fits in an int64.
const maxGrid = 1 << 26

// maxRounds is the most rounds of splits. A point that snaps to the grid
// can make a new crossing, which the next round splits.
const maxRounds = 16

// point is a point on the grid.
type point struct {
	x, y int64
}

// less reports if p comes before q, by x and then by y.
func (p point) less(q point) bool {
	return p.x < q.x || p.x == q.x && p.y < q.y
}

// cross returns the cross product of the vectors from o to a and from o to
// b. It is positive when b lies to the left of the line from o to a.
func cross(o, a, b point) int64 {
	return (a.x-o.x)*(b.y-o.y) - (a.y-o.y)*(b.x-o.x)
}

// sign returns -1, 0 or 1 for the sign of v.
func sign(v int64) int {
	switch {
	case v < 0:
		return -1
	case v > 0:
		return 1
	}
	return 0
}

// edge is an edge from a to b, with a before b. wind holds the edges of each
// operand on it: +1 for each edge from a to b, and -1 for each edge from b to
// a.
type edge struct {
	a, b   point
	wind   []int32
	splits []point
}

// newEdge returns the edge from p to q of operand k of n operands.
func newEdge(p, q point, k, n int) edge {
	e := edge{a: p, b: q, wind: make([]int32, n)}
	e.wind[k] = 1
	if q.less(p) {
		e.a, e.b = q, p
		e.wind[k] = -1
	}
	return e
}

// vertical reports if e is vertical.
func (e *edge) vertical() bool {
	return e.a.x == e.b.x
}

// inside reports if p lies on e, between its ends. p lies on the line of e.
func (e *edge) inside(p point) bool {
	return e.a.less(p) && p.less(e.b)
}

// grid maps the points of the operands to the grid and back.
type grid struct {
	x0, y0, step float64
}

// snap returns the point of the grid nearest to p.
func (g grid) snap(p kurbo.Point) point {
	return point{int64(math.Round((p.X - g.x0) / g.step)), int64(math.Round((p.Y - g.y0) / g.step))}
}

// unsnap returns the point that p of the grid stands for.
func (g grid) unsnap(p point) kurbo.Point {
	return kurbo.NewPoint(g.x0+float64(float64(p.x)*g.step), g.y0+float64(float64(p.y)*g.step))
}

// Combine returns the outline of the area that the operands give when they
// combine with op, in their order, and appends it to dst. Each operand fills
// with the nonzero rule, and a subpath that is not closed closes. Combine
// flattens the curves with the tolerance tolerance, so the result holds
// only lines. Each contour of the result is closed and has the inside on
// its left in a coordinate system where y goes up, so a hole turns the
// other way. The contours do not cross.
func Combine(op Op, operands [][]kurbo.PathEl, tolerance float64, dst []kurbo.PathEl) []kurbo.PathEl {
	n := len(operands)
	if n == 0 {
		return dst
	}
	// Flatten each operand into closed polylines.
	var flat []kurbo.PathEl
	var polys [][]kurbo.Point
	var owner []int
	bounds := kurbo.Rect{X0: math.Inf(1), Y0: math.Inf(1), X1: math.Inf(-1), Y1: math.Inf(-1)}
	for k, operand := range operands {
		flat = kurbo.Flatten(operand, tolerance, flat[:0])
		var poly []kurbo.Point
		for _, el := range flat {
			switch el.Kind {
			case kurbo.PathElMoveTo, kurbo.PathElClosePath:
				if len(poly) > 2 {
					polys, owner = append(polys, poly), append(owner, k)
				}
				poly = nil
				if el.Kind == kurbo.PathElClosePath {
					continue
				}
			}
			p := el.P[0]
			if !p.IsFinite() {
				continue
			}
			poly = append(poly, p)
			bounds = kurbo.Rect{X0: min(bounds.X0, p.X), Y0: min(bounds.Y0, p.Y), X1: max(bounds.X1, p.X), Y1: max(bounds.Y1, p.Y)}
		}
		if len(poly) > 2 {
			polys, owner = append(polys, poly), append(owner, k)
		}
	}
	extent := max(bounds.X1-bounds.X0, bounds.Y1-bounds.Y0)
	if len(polys) == 0 || !(extent > 0) {
		return dst
	}
	g := grid{x0: bounds.X0, y0: bounds.Y0, step: tolerance / gridSteps}
	if !(g.step > 0) || extent/g.step > maxGrid {
		g.step = extent / maxGrid
	}

	// The edges of the polylines on the grid.
	var edges []edge
	for i, poly := range polys {
		for j := range poly {
			p, q := g.snap(poly[j]), g.snap(poly[(j+1)%len(poly)])
			if p != q {
				edges = append(edges, newEdge(p, q, owner[i], n))
			}
		}
	}
	for range maxRounds {
		if !findSplits(edges) {
			break
		}
		edges = splitEdges(edges)
	}
	edges = mergeEdges(edges, n)
	kept := keepEdges(op, edges, n)
	return linkEdges(g, kept, dst)
}

// findSplits adds to the splits of each edge of edges the points where
// another edge crosses or touches it, and reports if it added any.
func findSplits(edges []edge) bool {
	slices.SortFunc(edges, func(e, f edge) int {
		switch {
		case e.a.x < f.a.x:
			return -1
		case e.a.x > f.a.x:
			return 1
		}
		return 0
	})
	found := false
	add := func(e *edge, p point) {
		if p != e.a && p != e.b {
			e.splits = append(e.splits, p)
			found = true
		}
	}
	for i := range edges {
		e := &edges[i]
		eyMin, eyMax := min(e.a.y, e.b.y), max(e.a.y, e.b.y)
		for j := i + 1; j < len(edges) && edges[j].a.x <= e.b.x; j++ {
			f := &edges[j]
			if max(f.a.y, f.b.y) < eyMin || min(f.a.y, f.b.y) > eyMax {
				continue
			}
			d1, d2 := sign(cross(e.a, e.b, f.a)), sign(cross(e.a, e.b, f.b))
			d3, d4 := sign(cross(f.a, f.b, e.a)), sign(cross(f.a, f.b, e.b))
			if d1 == 0 && d2 == 0 {
				// The edges lie on one line. Each end of one that lies
				// inside the other splits it.
				for _, p := range [...]point{f.a, f.b} {
					if e.inside(p) {
						add(e, p)
					}
				}
				for _, p := range [...]point{e.a, e.b} {
					if f.inside(p) {
						add(f, p)
					}
				}
				continue
			}
			if d1 == 0 && e.inside(f.a) {
				add(e, f.a)
			}
			if d2 == 0 && e.inside(f.b) {
				add(e, f.b)
			}
			if d3 == 0 && f.inside(e.a) {
				add(f, e.a)
			}
			if d4 == 0 && f.inside(e.b) {
				add(f, e.b)
			}
			if d1*d2 < 0 && d3*d4 < 0 {
				p := crossing(e, f)
				add(e, p)
				add(f, p)
			}
		}
	}
	return found
}

// crossing returns the point of the grid nearest to the point where e and f
// cross.
func crossing(e, f *edge) point {
	ex, ey := float64(e.b.x-e.a.x), float64(e.b.y-e.a.y)
	fx, fy := float64(f.b.x-f.a.x), float64(f.b.y-f.a.y)
	gx, gy := float64(f.a.x-e.a.x), float64(f.a.y-e.a.y)
	t := (float64(gx*fy) - float64(gy*fx)) / (float64(ex*fy) - float64(ey*fx))
	return point{e.a.x + int64(math.Round(float64(t*ex))), e.a.y + int64(math.Round(float64(t*ey)))}
}

// splitEdges returns the edges of edges, split at their splits.
func splitEdges(edges []edge) []edge {
	var out []edge
	for _, e := range edges {
		if len(e.splits) == 0 {
			out = append(out, e)
			continue
		}
		dx, dy := e.b.x-e.a.x, e.b.y-e.a.y
		along := func(p point) int64 { return (p.x-e.a.x)*dx + (p.y-e.a.y)*dy }
		slices.SortFunc(e.splits, func(p, q point) int {
			if s := sign(along(p) - along(q)); s != 0 {
				return s
			}
			if p.less(q) {
				return -1
			}
			if q.less(p) {
				return 1
			}
			return 0
		})
		p := e.a
		for _, q := range append(slices.Compact(e.splits), e.b) {
			if q == p {
				continue
			}
			s := edge{a: p, b: q, wind: slices.Clone(e.wind)}
			if q.less(p) {
				s.a, s.b = q, p
				for k := range s.wind {
					s.wind[k] = -s.wind[k]
				}
			}
			out = append(out, s)
			p = q
		}
	}
	return out
}

// mergeEdges merges the edges of edges that have the same ends, and drops
// the edges whose windings cancel.
func mergeEdges(edges []edge, n int) []edge {
	index := make(map[[2]point]int, len(edges))
	var out []edge
	for _, e := range edges {
		key := [2]point{e.a, e.b}
		if i, ok := index[key]; ok {
			for k := range n {
				out[i].wind[k] += e.wind[k]
			}
			continue
		}
		index[key] = len(out)
		out = append(out, edge{a: e.a, b: e.b, wind: e.wind})
	}
	return slices.DeleteFunc(out, func(e edge) bool {
		return !slices.ContainsFunc(e.wind, func(w int32) bool { return w != 0 })
	})
}

// directed is an edge of the result, from a to b, with the inside on its
// left.
type directed struct {
	a, b point
}

// keepEdges returns the edges of edges that have the inside of the result
// of op on one side and the outside on the other, each with the inside on
// its left.
func keepEdges(op Op, edges []edge, n int) []directed {
	// Each column holds the edges that are not vertical and that cover a
	// part of its range of doubled x coordinates.
	x0, x1 := edges[0].a.x, edges[0].b.x
	for i := range edges {
		x0, x1 = min(x0, edges[i].a.x), max(x1, edges[i].b.x)
	}
	columns := max(1, int(math.Sqrt(float64(len(edges)))))
	width := (2*(x1-x0))/int64(columns) + 1
	column := func(x2 int64) int { return int((x2 - 2*x0) / width) }
	buckets := make([][]int32, columns)
	for i := range edges {
		e := &edges[i]
		if e.vertical() {
			continue
		}
		for c := column(2 * e.a.x); c <= column(2*e.b.x-1); c++ {
			buckets[c] = append(buckets[c], int32(i))
		}
	}

	right := make([]int32, n)
	left := make([]int32, n)
	var kept []directed
	for i := range edges {
		e := &edges[i]
		// The winding numbers at the middle of e, just below it, or just
		// to its right when it is vertical. That is the right side of e,
		// from a to b. Each edge that is not vertical and that lies below
		// that point crosses the vertical ray down from it. An edge
		// covers the x coordinates from its a to before its b, so that a
		// ray through an end of two edges counts one of them.
		px2, py2 := e.a.x+e.b.x, e.a.y+e.b.y
		if e.vertical() {
			px2 = 2 * e.a.x
		}
		clear(right)
		for _, j := range buckets[column(px2)] {
			f := &edges[j]
			if int(j) == i || 2*f.a.x > px2 || px2 >= 2*f.b.x {
				continue
			}
			if (f.b.x-f.a.x)*(py2-2*f.a.y)-(f.b.y-f.a.y)*(px2-2*f.a.x) > 0 {
				for k := range n {
					right[k] += f.wind[k]
				}
			}
		}
		for k := range n {
			left[k] = right[k] + e.wind[k]
		}
		inLeft, inRight := apply(op, left), apply(op, right)
		switch {
		case inLeft && !inRight:
			kept = append(kept, directed{e.a, e.b})
		case inRight && !inLeft:
			kept = append(kept, directed{e.b, e.a})
		}
	}
	return kept
}

// apply reports if the point with the winding numbers wind of the operands
// lies inside the result of op.
func apply(op Op, wind []int32) bool {
	in := wind[0] != 0
	for _, w := range wind[1:] {
		next := w != 0
		switch op {
		case Union:
			in = in || next
		case Difference:
			in = in && !next
		case Intersection:
			in = in && next
		case Xor:
			in = in != next
		}
	}
	return in
}

// linkEdges links the edges of kept into closed contours, appends them to
// dst as path elements in the coordinates of g, and returns dst. At a point
// where several edges of kept start, a contour takes the edge that turns
// the most to the left, so that it follows the boundary of one area.
func linkEdges(g grid, kept []directed, dst []kurbo.PathEl) []kurbo.PathEl {
	out := make(map[point][]int, len(kept))
	for i, e := range kept {
		out[e.a] = append(out[e.a], i)
	}
	used := make([]bool, len(kept))
	var contour []point
	for start := range kept {
		if used[start] {
			continue
		}
		contour = contour[:0]
		for i := start; !used[i]; {
			used[i] = true
			e := kept[i]
			contour = append(contour, e.a)
			// The next edge is the first one clockwise from the edge back
			// to e.a.
			bx, by := float64(e.a.x-e.b.x), float64(e.a.y-e.b.y)
			best, bestAngle := -1, 0.0
			for _, j := range out[e.b] {
				f := kept[j]
				fx, fy := float64(f.b.x-f.a.x), float64(f.b.y-f.a.y)
				angle := math.Atan2(float64(bx*fy)-float64(by*fx), float64(bx*fx)+float64(by*fy))
				// angle turns counterclockwise from the edge back, so the
				// clockwise angle is its negation, from 0 to 2 pi.
				cw := -angle
				if cw <= 0 {
					cw += 2 * math.Pi
				}
				if best < 0 || cw < bestAngle {
					best, bestAngle = j, cw
				}
			}
			if best < 0 {
				break
			}
			i = best
		}
		dst = appendContour(g, simplify(contour), dst)
	}
	return dst
}

// simplify removes the points of the closed contour that lie on the line
// between the points before and after them, in place, and returns it.
func simplify(contour []point) []point {
	for changed := true; changed && len(contour) > 2; {
		changed = false
		out := contour[:0]
		for i, p := range contour {
			prev := contour[(i+len(contour)-1)%len(contour)]
			if len(out) > 0 {
				prev = out[len(out)-1]
			}
			next := contour[(i+1)%len(contour)]
			if cross(prev, p, next) == 0 && (p.x-prev.x)*(next.x-p.x)+(p.y-prev.y)*(next.y-p.y) > 0 {
				changed = true
				continue
			}
			out = append(out, p)
		}
		contour = out
	}
	return contour
}

// appendContour appends the closed contour in the coordinates of g to dst.
func appendContour(g grid, contour []point, dst []kurbo.PathEl) []kurbo.PathEl {
	if len(contour) < 3 {
		return dst
	}
	dst = append(dst, kurbo.MoveTo(g.unsnap(contour[0])))
	for _, p := range contour[1:] {
		dst = append(dst, kurbo.LineTo(g.unsnap(p)))
	}
	return append(dst, kurbo.ClosePath())
}
