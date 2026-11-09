// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"cmp"
	"log/slog"
	"math"
	"slices"

	"github.com/xo/okage/path"
)

// fillPathPath is fill_path in tiny-skia src/scan/path.rs. The name takes the
// suffix of its module, because src/scan/path_aa.rs has a function of the
// same name (D10).
//
// Port: logger is the logger of D20, which buildEdges logs to. A nil logger
// logs nothing.
func fillPathPath(p *Path, fillRule FillRule, clip *screenIntRect, b blitter, logger *slog.Logger) {
	ir, ok := conservativeRoundToInt(p.Bounds())
	if !ok {
		return
	}

	pathContainedInClip := false
	if bounds, ok := intRectToScreenIntRect(ir); ok {
		pathContainedInClip = clip.Contains(bounds)
	}
	// If bounds cannot be converted into ScreenIntRect,
	// the path is out of clip.

	// TODO: SkScanClipper

	fillPathImplPath(
		p,
		fillRule,
		clip,
		ir.Y(),
		ir.Bottom(),
		0,
		pathContainedInClip,
		b,
		logger,
	)
}

// conservativeRoundToInt is conservative_round_to_int in tiny-skia
// src/scan/path.rs.
//
// Conservative rounding function, which effectively nudges the int-rect to be
// slightly larger than Rect::round() produces. This is a safety-net for the
// scan-converter, which inspects the returned int-rect, and can
// disable clipping (for speed) if it thinks all of the edges will fit inside
// the clip's bounds. The scan-converter introduces slight numeric errors due
// to accumulated += of the slope, so this function is used to return a
// conservatively large int-bounds, and thus we will only disable clipping if
// we are sure the edges will stay in-bounds.
func conservativeRoundToInt(src Rect) (IntRect, bool) {
	// We must use `from_ltrb`, otherwise rounding will be incorrect.
	return path.IntRectFromLTRB(
		roundDownToInt(src.Left()),
		roundDownToInt(src.Top()),
		roundUpToInt(src.Right()),
		roundUpToInt(src.Bottom()),
	)
}

// conservativeRoundBias is CONSERVATIVE_ROUND_BIAS in tiny-skia
// src/scan/path.rs.
//
// Bias used for conservative rounding of float rects to int rects, to nudge
// the irects a little larger, so we do not "think" a path's bounds are inside a
// clip, when (due to numeric drift in the scan-converter) we can walk beyond
// the predicted limits.
//
// This value has been determined trial and error: pick the smallest value
// (after the 0.5) that fixes any problematic cases (e.g. crbug.com/844457)
// NOTE: cubics appear to be the main reason for needing this slop. If we can
// (perhaps) have a more accurate walker for cubics, we can reduce
// this fudge factor.
const conservativeRoundBias float64 = 0.5 + 1.5/float64(fdot6One)

// roundDownToInt is round_down_to_int in tiny-skia src/scan/path.rs.
//
// Round the value down. This is used to round the top and left of a
// rectangle, and corresponds to the way the scan converter treats the top and
// left edges. It has a slight bias to make the "rounded" int smaller than a
// normal round, to create a more conservative int-bounds (larger) from a float
// rect.
func roundDownToInt(x float32) int32 {
	xx := float64(x)
	xx -= conservativeRoundBias
	return path.SaturateFromF64(math.Ceil(xx))
}

// roundUpToInt is round_up_to_int in tiny-skia src/scan/path.rs.
//
// Round the value up. This is used to round the right and bottom of a
// rectangle. It has a slight bias to make the "rounded" int smaller than a
// normal round, to create a more conservative int-bounds (larger) from a float
// rect.
func roundUpToInt(x float32) int32 {
	xx := float64(x)
	xx += conservativeRoundBias
	return path.SaturateFromF64(math.Floor(xx))
}

// edgeHeadY is EDGE_HEAD_Y, a constant of fill_path_impl in tiny-skia
// src/scan/path.rs.
const edgeHeadY int32 = math.MinInt32

// edgeTailY is EDGE_TAIL_Y, a constant of fill_path_impl in tiny-skia
// src/scan/path.rs.
const edgeTailY int32 = math.MaxInt32

// fillPathImplPath is fill_path_impl in tiny-skia src/scan/path.rs. The name
// takes the suffix of its module, because src/scan/path_aa.rs has a function
// of the same name (D10).
//
// Port: logger is the logger of D20, which buildEdges logs to.
func fillPathImplPath(
	p *Path,
	fillRule FillRule,
	clipRect *screenIntRect,
	startY int32,
	stopY int32,
	shiftEdgesUp int32,
	pathContainedInClip bool,
	b blitter,
	logger *slog.Logger,
) {
	shiftedClip, ok := newShiftedIntRect(clipRect, shiftEdgesUp)
	if !ok {
		return
	}

	var clip *shiftedIntRect
	if !pathContainedInClip {
		clip = &shiftedClip
	}
	edges, ok := buildEdges(p, clip, shiftEdgesUp, logger)
	if !ok {
		return // no edges to render, just return
	}

	slices.SortStableFunc(edges, func(a, b edge) int {
		valueA := a.asLine().firstY
		valueB := b.asLine().firstY

		if valueA == valueB {
			valueA = a.asLine().x
			valueB = b.asLine().x
		}

		return cmp.Compare(valueA, valueB)
	})

	for i := range edges {
		// 0 will be set later, so start with 1.
		edges[i].asLine().prev = someU32(uint32(i) + 0)
		edges[i].asLine().next = someU32(uint32(i) + 2)
	}

	edges = slices.Insert(edges, 0, edge{
		kind: edgeKindLine,
		line: lineEdge{
			next:   someU32(1),
			x:      math.MinInt32,
			firstY: edgeHeadY,
		},
	})

	edges = append(edges, edge{
		kind: edgeKindLine,
		line: lineEdge{
			prev:   someU32(uint32(len(edges)) - 1),
			firstY: edgeTailY,
		},
	})

	startY <<= shiftEdgesUp
	stopY <<= shiftEdgesUp

	top := int32(shiftedClip.shiftedRect().Y())
	if !pathContainedInClip && startY < top {
		startY = top
	}

	bottom := int32(shiftedClip.shiftedRect().Bottom())
	if !pathContainedInClip && stopY > bottom {
		stopY = bottom
	}

	if startY < 0 {
		return
	}
	if stopY < 0 {
		return
	}

	// TODO: walk_simple_edges

	walkEdges(
		fillRule,
		uint32(startY),
		uint32(stopY),
		shiftedClip.shiftedRect().Right(),
		edges,
		b,
	)
}

// walkEdges is walk_edges in tiny-skia src/scan/path.rs.
//
// TODO: simplify!
func walkEdges(fillRule FillRule, startY, stopY, rightClip uint32, edges []edge, b blitter) {
	currY := startY
	windingMask := int32(-1)
	if fillRule == FillRuleEvenOdd {
		windingMask = 1
	}

	for {
		w := int32(0)
		left := uint32(0)
		prevX := edges[0].asLine().x

		currIdx := int(edges[0].asLine().next.unwrap())
		for edges[currIdx].asLine().firstY <= int32(currY) {
			x := uint32(fdot16RoundToI32(edges[currIdx].asLine().x)) // TODO: check

			if (w & windingMask) == 0 {
				// we are starting interval
				left = x
			}

			w += int32(edges[currIdx].asLine().winding)

			if (w & windingMask) == 0 {
				// we finished an interval
				if width := x - left; width != 0 {
					b.blitH(left, currY, width)
				}
			}

			nextIdx := edges[currIdx].asLine().next.unwrap()
			var newX fDot16

			if edges[currIdx].asLine().lastY == int32(currY) {
				// are we done with this edge?
				switch e := &edges[currIdx]; e.kind {
				case edgeKindLine:
					removeEdge(currIdx, edges)
				case edgeKindQuadratic:
					if e.quad.curveCount > 0 && e.quad.update() {
						newX = e.quad.line.x

						if newX < prevX {
							// ripple current edge backwards until it is x-sorted
							backwardInsertEdgeBasedOnX(currIdx, edges)
						} else {
							prevX = newX
						}
					} else {
						removeEdge(currIdx, edges)
					}
				case edgeKindCubic:
					if e.cubic.curveCount < 0 && e.cubic.update() {
						newX = e.cubic.line.x

						if newX < prevX {
							// ripple current edge backwards until it is x-sorted
							backwardInsertEdgeBasedOnX(currIdx, edges)
						} else {
							prevX = newX
						}
					} else {
						removeEdge(currIdx, edges)
					}
				}
			} else {
				line := edges[currIdx].asLine()
				newX = line.x + line.dx
				line.x = newX

				if newX < prevX {
					// ripple current edge backwards until it is x-sorted
					backwardInsertEdgeBasedOnX(currIdx, edges)
				} else {
					prevX = newX
				}
			}

			currIdx = int(nextIdx)
		}

		if (w & windingMask) != 0 {
			// was our right-edge culled away?
			if width := rightClip - left; width != 0 {
				b.blitH(left, currY, width)
			}
		}

		currY++
		if currY >= stopY {
			break
		}

		// now current edge points to the first edge with a Yint larger than curr_y
		insertNewEdges(currIdx, int32(currY), edges)
	}
}

// removeEdge is remove_edge in tiny-skia src/scan/path.rs.
func removeEdge(currIdx int, edges []edge) {
	prev := edges[currIdx].asLine().prev.unwrap()
	next := edges[currIdx].asLine().next.unwrap()

	edges[prev].asLine().next = someU32(next)
	edges[next].asLine().prev = someU32(prev)
}

// backwardInsertEdgeBasedOnX is backward_insert_edge_based_on_x in tiny-skia
// src/scan/path.rs.
func backwardInsertEdgeBasedOnX(currIdx int, edges []edge) {
	x := edges[currIdx].asLine().x
	prevIdx := int(edges[currIdx].asLine().prev.unwrap())
	for prevIdx != 0 {
		if edges[prevIdx].asLine().x > x {
			prevIdx = int(edges[prevIdx].asLine().prev.unwrap())
		} else {
			break
		}
	}

	nextIdx := int(edges[prevIdx].asLine().next.unwrap())
	if nextIdx != currIdx {
		removeEdge(currIdx, edges)
		insertEdgeAfter(currIdx, prevIdx, edges)
	}
}

// insertEdgeAfter is insert_edge_after in tiny-skia src/scan/path.rs.
func insertEdgeAfter(currIdx, afterIdx int, edges []edge) {
	edges[currIdx].asLine().prev = someU32(uint32(afterIdx))
	edges[currIdx].asLine().next = edges[afterIdx].asLine().next

	afterNextIdx := int(edges[afterIdx].asLine().next.unwrap())
	edges[afterNextIdx].asLine().prev = someU32(uint32(currIdx))
	edges[afterIdx].asLine().next = someU32(uint32(currIdx))
}

// backwardInsertStart is backward_insert_start in tiny-skia src/scan/path.rs.
//
// Start from the right side, searching backwards for the point to begin the
// new edge list insertion, marching forwards from here. The implementation
// can start from the left of the prior insertion, and search to the
// right, or with some additional caching, binary search the starting point.
// More work can be done to determine optimal new edge insertion.
func backwardInsertStart(prevIdx int, x fDot16, edges []edge) int {
	for {
		prev := edges[prevIdx].asLine().prev
		if !prev.ok {
			break
		}
		prevIdx = int(prev.v)
		if edges[prevIdx].asLine().x <= x {
			break
		}
	}

	return prevIdx
}

// insertNewEdges is insert_new_edges in tiny-skia src/scan/path.rs.
func insertNewEdges(newIdx int, currY int32, edges []edge) {
	if edges[newIdx].asLine().firstY != currY {
		return
	}

	prevIdx := int(edges[newIdx].asLine().prev.unwrap())
	if edges[prevIdx].asLine().x <= edges[newIdx].asLine().x {
		return
	}

	// find first x pos to insert
	startIdx := backwardInsertStart(prevIdx, edges[newIdx].asLine().x, edges)
	// insert the lot, fixing up the links as we go
	for {
		nextIdx := int(edges[newIdx].asLine().next.unwrap())
		keepEdge := false
		for {
			afterIdx := int(edges[startIdx].asLine().next.unwrap())
			if afterIdx == newIdx {
				keepEdge = true
				break
			}

			if edges[afterIdx].asLine().x >= edges[newIdx].asLine().x {
				break
			}

			startIdx = afterIdx
		}

		if !keepEdge {
			removeEdge(newIdx, edges)
			insertEdgeAfter(newIdx, startIdx, edges)
		}

		startIdx = newIdx
		newIdx = nextIdx

		if edges[newIdx].asLine().firstY != currY {
			break
		}
	}
}
