// Copyright 2020 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "github.com/xo/okage/internal/boxblur"

// This file ports resvg src/filter/box_blur.rs. The port is in the package
// internal/boxblur, so that lottie blurs with it too (D79).

// applyFilterBoxBlur is apply in resvg src/filter/box_blur.rs. It applies a
// box blur to the premultiplied pixels of src.
func applyFilterBoxBlur(sigmaX, sigmaY float64, src imageRefMut) {
	boxblur.Apply(sigmaX, sigmaY, src.data, src.width, src.height)
}
