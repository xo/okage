// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/parser/clippath.rs.

// convertClipPath is convert in usvg src/parser/clippath.rs.
//
// Port: a nil objectBBox is None.
func convertClipPath(
	node svgNode,
	state *converterState,
	objectBBox *NonZeroRect,
	cache *converterCache,
) (*ClipPath, bool) {
	// A `clip-path` attribute must reference a `clipPath` element.
	if !node.hasTagName(EIdClipPath) {
		return nil, false
	}

	// The whole clip path should be ignored when a transform is invalid.
	transform, ok := resolveClipPathTransform(node, state)
	if !ok {
		return nil, false
	}

	units, ok := nodeAttribute(node, AIdClipPathUnits, fromValueUnits)
	if !ok {
		units = unitsUserSpaceOnUse
	}

	// Check if this element was already converted.
	//
	// Only `userSpaceOnUse` clipPaths can be shared,
	// because `objectBoundingBox` one will be converted into user one
	// and will become node-specific.
	cacheable := units == unitsUserSpaceOnUse
	if cacheable {
		if clip, ok := cache.clipPaths[node.elementID()]; ok {
			return clip, true
		}
	}

	if units == unitsObjectBoundingBox {
		if objectBBox == nil {
			state.opt.warnf("Clipping of zero-sized shapes is not allowed.")
			return nil, false
		}

		ts := path.TransformFromBBox(*objectBBox)
		transform = transform.PreConcat(ts)
	}

	// Resolve linked clip path.
	var clipPath *ClipPath
	if link, ok := node.attributeNode(AIdClipPath); ok {
		clipPath, ok = convertClipPath(link, state, objectBBox, cache)

		// Linked `clipPath` must be valid.
		if !ok {
			return nil, false
		}
	}

	id, ok := newNonEmptyString(node.elementID())
	if !ok {
		return nil, false
	}
	// Generate ID only when we're parsing `objectBoundingBox` clip for the second time.
	if _, ok := cache.clipPaths[id.get()]; !cacheable && ok {
		id = cache.genClipPathID()
	}
	idCopy := id.get()

	clip := &ClipPath{
		id:        id,
		transform: transform,
		clipPath:  clipPath,
		root:      *emptyGroup(),
	}

	clipState := state.clone()
	clipState.parentClipPath = &node
	convertClipPathElements(node, clipState, cache, &clip.root)

	if clip.root.HasChildren() {
		clip.root.calculateBoundingBoxes()
		cache.clipPaths[idCopy] = clip
		return clip, true
	}
	// A clip path without children is invalid.
	return nil, false
}

// resolveClipPathTransform is resolve_clip_path_transform in usvg
// src/parser/clippath.rs.
func resolveClipPathTransform(node svgNode, state *converterState) (Transform, bool) {
	// Do not use Node::attribute::<Transform>, because it will always
	// return a valid transform.

	value, ok := node.attributeStr(AIdTransform)
	if !ok {
		return path.DefaultTransform(), true
	}

	ts, err := svgtypes.ParseTransform(value)
	if err != nil {
		state.opt.warnf("Failed to parse %s value: '%s'.", AIdTransform, value)
		return Transform{}, false
	}

	t := path.TransformFromRow(
		float32(ts.A),
		float32(ts.B),
		float32(ts.C),
		float32(ts.D),
		float32(ts.E),
		float32(ts.F),
	)

	if t.IsValid() {
		return node.resolveTransform(AIdTransform, state), true
	}
	return Transform{}, false
}
