// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/parser/mask.rs.

// convertMask is convert in usvg src/parser/mask.rs.
//
// Port: a nil objectBBox is None.
func convertMask(
	node svgNode,
	state *converterState,
	objectBBox *NonZeroRect,
	cache *converterCache,
) (*Mask, bool) {
	// A `mask` attribute must reference a `mask` element.
	if !node.hasTagName(EIdMask) {
		return nil, false
	}

	units, ok := nodeAttribute(node, AIdMaskUnits, fromValueUnits)
	if !ok {
		units = unitsObjectBoundingBox
	}

	contentUnits, ok := nodeAttribute(node, AIdMaskContentUnits, fromValueUnits)
	if !ok {
		contentUnits = unitsUserSpaceOnUse
	}

	// Check if this element was already converted.
	//
	// Only `userSpaceOnUse` masks can be shared,
	// because `objectBoundingBox` one will be converted into user one
	// and will become node-specific.
	cacheable := units == unitsUserSpaceOnUse && contentUnits == unitsUserSpaceOnUse
	if cacheable {
		if mask, ok := cache.masks[node.elementID()]; ok {
			return mask, true
		}
	}

	rect, ok := path.NonZeroRectFromXYWH(
		node.convertLength(AIdX, units, state, svgtypes.NewLength(-10.0, svgtypes.LengthUnitPercent)),
		node.convertLength(AIdY, units, state, svgtypes.NewLength(-10.0, svgtypes.LengthUnitPercent)),
		node.convertLength(AIdWidth, units, state, svgtypes.NewLength(120.0, svgtypes.LengthUnitPercent)),
		node.convertLength(AIdHeight, units, state, svgtypes.NewLength(120.0, svgtypes.LengthUnitPercent)),
	)
	rect, ok = logNone(rect, ok, func() {
		state.opt.warnf("Mask '%s' has an invalid size. Skipped.", node.elementID())
	})
	if !ok {
		return nil, false
	}

	maskAll := false
	if units == unitsObjectBoundingBox {
		if objectBBox != nil {
			rect = rect.BBoxTransform(*objectBBox)
		} else {
			// When mask units are `objectBoundingBox` and bbox is zero-sized - the whole
			// element should be masked.
			// Technically an UB, but this is what Chrome and Firefox do.
			maskAll = true
		}
	}

	id, ok := newNonEmptyString(node.elementID())
	if !ok {
		return nil, false
	}
	// Generate ID only when we're parsing `objectBoundingBox` mask for the second time.
	if _, ok := cache.masks[id.get()]; !cacheable && ok {
		id = cache.genMaskID()
	}
	idCopy := id.get()

	if maskAll {
		mask := &Mask{
			id:   id,
			rect: rect,
			kind: MaskTypeLuminance,
			mask: nil,
			root: *emptyGroup(),
		}
		cache.masks[idCopy] = mask
		return mask, true
	}

	// Resolve linked mask.
	var linked *Mask
	if link, ok := node.attributeNode(AIdMask); ok {
		linked, ok = convertMask(link, state, objectBBox, cache)

		// Linked `mask` must be valid.
		if !ok {
			return nil, false
		}
	}

	kind := MaskTypeLuminance
	if v, ok := node.attributeStr(AIdMaskType); ok && v == "alpha" {
		kind = MaskTypeAlpha
	}

	mask := &Mask{
		id:   id,
		rect: rect,
		kind: kind,
		mask: linked,
		root: *emptyGroup(),
	}

	// To emulate content `objectBoundingBox` units we have to put
	// mask children into a group with a transform.
	var subroot *Group
	if contentUnits == unitsObjectBoundingBox {
		if objectBBox == nil {
			state.opt.warnf("Masking of zero-sized shapes is not allowed.")
			return nil, false
		}

		g := emptyGroup()
		g.transform = path.TransformFromBBox(*objectBBox)
		// Make sure to set `abs_transform`, because it must propagate to all children.
		g.absTransform = g.transform

		subroot = g
	}

	{
		// Prefer `subroot` to `mask.root`.
		realRoot := subroot
		if realRoot == nil {
			realRoot = &mask.root
		}
		convertChildren(node, state, cache, realRoot)

		// A mask without children at this point is invalid.
		// Only masks with zero bbox and `objectBoundingBox` can be empty.
		if !realRoot.HasChildren() {
			return nil, false
		}
	}

	if subroot != nil {
		subroot.calculateBoundingBoxes()
		mask.root.children = append(mask.root.children, subroot)
	}

	mask.root.calculateBoundingBoxes()
	updatePaintServers(
		&mask.root,
		path.DefaultTransform(),
		nil,
		nil,
		cache,
	)

	cache.masks[idCopy] = mask
	return mask, true
}
