// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfsvg

import (
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
	"github.com/xo/okage/svg"
)

// This file ports krilla-svg src/group.rs, with the raster fallback of the
// blend mode PlusLighter (D62).

// renderGroup is render in krilla-svg src/group.rs. It takes the suffix of
// its module, because other modules have a function of the same name (D10).
func renderGroup(group *svg.Group, surface *pdf.Surface, pc *processContext) {
	if len(group.Filters()) != 0 {
		renderFilter(group, surface, pc)
		return
	}

	// okage: a group whose blending needs PlusLighter is drawn by the raster
	// fallback (D62).
	if group.ShouldIsolate() && needsPlusLighter(group) {
		renderPlusLighter(group, surface, pc)
		return
	}

	popCount := 0

	surface.PushTransform(group.Transform())
	popCount++

	if clipPath, ok := group.ClipPath(); ok {
		popCount += renderClipPath(group, clipPath, surface, pc)
	}

	if mask, ok := group.Mask(); ok {
		popCount += renderMask(mask, surface, pc)
	}

	blendMode := convertBlendMode(group.BlendMode())
	surface.PushBlendMode(blendMode)
	surface.PushOpacity(group.Opacity())
	popCount += 2

	// There is the `group.should_isolate` method which also returns true if
	// there is a clip path, since strictly speaking, just a clip path
	// also requires layer isolation. However, clip paths are very common
	// and it would be wasteful to generate new isolated XObjects for that
	// (especially since it shouldn't make any visual difference), so we
	// don't include this here.
	shouldIsolate := group.Isolate() ||
		group.Opacity() != path.NormalizedF32One ||
		hasMask(group) ||
		group.BlendMode() != svg.BlendModeNormal

	if shouldIsolate {
		surface.PushIsolated()
		popCount++
	}

	for _, child := range group.Children() {
		renderNodeGroup(child, surface, pc)
	}

	for range popCount {
		surface.Pop()
	}
}

// hasMask is group.mask().is_some() of upstream.
func hasMask(group *svg.Group) bool {
	_, ok := group.Mask()
	return ok
}

// renderNodeGroup is render_node in krilla-svg src/group.rs. It takes the
// suffix of its module, because lib.rs has a function of the same name
// (D10).
func renderNodeGroup(node svg.Node, surface *pdf.Surface, pc *processContext) {
	switch n := node.(type) {
	case *svg.Group:
		renderGroup(n, surface, pc)
	case *svg.Path:
		renderPath(n, surface, pc)
	case *svg.Image:
		renderImage(n, surface, pc)
	case *svg.Text:
		renderText(n, surface, pc)
	}
}

// renderRoot draws the root of a tree or of a subtree, such as the root of
// a pattern, a mask or a clip path. Upstream calls render of group.rs, and
// the port calls renderRoot in its place.
//
// okage: usvg 0.47, which krilla-svg 0.8.1 requires, has no blend mode
// PlusLighter, and PDF has none. A group blends with the groups before it,
// up to the closest group that svg isolates, or up to the root. So when a
// group in that range has PlusLighter, the port draws the whole range with
// okage into an image, with the code of the filters (D62).
func renderRoot(group *svg.Group, surface *pdf.Surface, pc *processContext) {
	if len(group.Filters()) == 0 && needsPlusLighter(group) {
		renderPlusLighter(group, surface, pc)
		return
	}
	renderGroup(group, surface, pc)
}

// needsPlusLighter reports whether a child of the group, or of a child that
// svg does not isolate, has the blend mode PlusLighter. Such a child blends
// with the content of the group.
func needsPlusLighter(group *svg.Group) bool {
	for _, child := range group.Children() {
		g, ok := child.(*svg.Group)
		if !ok {
			continue
		}
		if g.BlendMode() == svg.BlendModePlusLighter {
			return true
		}
		if !g.ShouldIsolate() && needsPlusLighter(g) {
			return true
		}
	}
	return false
}

// renderPlusLighter draws a group with the raster fallback of the filters.
// The image holds the transform, the clip path, the mask and the opacity of
// the group. The blend mode of the group is a mode of PDF, because the
// group that holds it is drawn first, so the surface blends the image with
// it. A blend mode of PDF over a transparent backdrop gives the source, so
// the image does not hold it.
func renderPlusLighter(group *svg.Group, surface *pdf.Surface, pc *processContext) {
	blendMode := convertBlendMode(group.BlendMode())
	if blendMode != pdf.BlendModeNormal {
		surface.PushBlendMode(blendMode)
	}
	renderFilter(group, surface, pc)
	if blendMode != pdf.BlendModeNormal {
		surface.Pop()
	}
}
