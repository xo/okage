// Copyright 2025 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "github.com/xo/okage/path"

// This file ports usvg src/text/transform.rs.
//
// Port: skrifa::color::Transform becomes colrTransform, the transform of the
// paint graph of a COLR glyph in usvg_text_colr_paint.go (D9).

// skrifaToTspTransform is skrifa_to_tsp_transform in usvg
// src/text/transform.rs.
func skrifaToTspTransform(t colrTransform) Transform {
	return path.TransformFromRow(t.xx, t.yx, t.xy, t.yy, t.dx, t.dy)
}

// tspToSkrifaTransform is tsp_to_skrifa_transform in usvg
// src/text/transform.rs.
func tspToSkrifaTransform(t Transform) colrTransform {
	return colrTransform{
		xx: t.SX,
		yx: t.KY,
		xy: t.KX,
		yy: t.SY,
		dx: t.TX,
		dy: t.TY,
	}
}
