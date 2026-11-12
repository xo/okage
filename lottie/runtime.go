// Copyright 2024 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports velato src/runtime/mod.rs.

import (
	"fmt"

	"github.com/xo/okage"
)

// Composition is Composition in velato src/runtime/mod.rs: the model of a
// Lottie file.
type Composition struct {
	// Frames in which the animation is active: [Frames[0], Frames[1]).
	Frames [2]float64
	// Frames per second.
	FrameRate float64
	// Width of the animation.
	Width uint
	// Height of the animation.
	Height uint
	// Precomposed layers that can be instanced, by the id of the asset.
	Assets map[string][]Layer
	// Raster image assets, keyed by the identifier of a layer's refId.
	Images map[string]ImageAsset
	// Collection of layers.
	Layers []Layer
}

// Parse is Composition::from_slice in velato src/runtime/mod.rs: the
// composition of the Lottie file in data.
func Parse(data []byte) (*Composition, error) {
	source, err := ParseAnimation(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", okage.ErrLottieParse, err)
	}
	return convAnimation(source)
}
