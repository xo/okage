// Copyright 2023 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "github.com/xo/okage/path"

// This file ports resvg src/geom.rs.

// fitToRect is fit_to_rect in resvg src/geom.rs. It fits the current rect
// into the specified bounds.
func fitToRect(r, bounds path.IntRect) (path.IntRect, bool) {
	left := r.Left()
	if left < bounds.Left() {
		left = bounds.Left()
	}

	top := r.Top()
	if top < bounds.Top() {
		top = bounds.Top()
	}

	right := r.Right()
	if right > bounds.Right() {
		right = bounds.Right()
	}

	bottom := r.Bottom()
	if bottom > bounds.Bottom() {
		bottom = bounds.Bottom()
	}

	return path.IntRectFromLTRB(left, top, right, bottom)
}
