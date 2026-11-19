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

// This file ports krilla-svg src/clip_path.rs.

// renderClipPath is render in krilla-svg src/clip_path.rs. It takes the
// suffix of its module, because other modules have a function of the same
// name (D10). It returns the number of pushes onto the surface, which the
// caller pops.
//
// Render a clip path into a surface.
func renderClipPath(group *svg.Group, clipPath *svg.ClipPath, surface *pdf.Surface, pc *processContext) int {
	// Unfortunately, clip paths are a bit tricky to deal with, the reason
	// being that clip paths in SVGs can be much more complex than in PDF. In
	// SVG, clip paths can have transforms, as well as nested clip paths. The
	// objects inside of the clip path can have transforms as well, making it
	// even more difficult to deal with. Because in PDF, once you start a clip
	// path operation you cannot interrupt it, because once you pop the
	// current graphics state, the clip path will be lost since it is part of
	// the current graphics state. However, if we have various transforms on
	// the children, we need to be able to push/pop the graphics state, so
	// that the children's transforms don't affect each other. Initially,
	// because of this, clip paths were only implemented using soft masks,
	// but Safari has a couple of issues with rendering them properly. Not to
	// mention the fact that soft masks are obviously also more expensive.
	// Because of this, we proceed the following way: We first check whether
	// the clip path itself is too "complex" (complex being that it fulfills
	// some attributes that make it impossible to represent them in our
	// current setup using just native PDF clip paths. If it is too complex,
	// we fall back to using soft masks. However, if it is simple enough, we
	// just use the normal clip path operator in PDF. It should be noted that
	// in reality, only very few SVGs seem to have such complex clipping paths
	// (they are not even rendered correctly by all online converters that
	// were tested), so in most real-life scenarios, the simple version should
	// suffice. But in order to conform with the SVG specification, we also
	// handle the case of more complex clipping paths, even if this means that
	// Safari will in some cases not display them correctly.
	isSimple := isSimpleClipPath(clipPath.Root())
	clipRules := collectClipRules(clipPath.Root())

	popCount := 0

	if isSimple &&
		(allClipRules(clipRules, svg.FillRuleNonZero) ||
			// For even odd, there must be at most one shape in the group,
			// because overlapping shapes with evenodd render differently in
			// PDF
			(allClipRules(clipRules, svg.FillRuleEvenOdd) && len(clipRules) == 1)) {
		rule := svg.FillRuleNonZero
		if len(clipRules) != 0 {
			rule = clipRules[0]
		}
		clips := createClipPath(clipPath, rule)

		for _, c := range clips {
			surface.PushClipPath(c.path, c.rule)
			popCount++
		}
	} else {
		popCount += renderComplex(group, clipPath, surface, pc)
	}

	return popCount
}

// allClipRules is clip_rules.iter().all(|f| *f == rule) of upstream.
func allClipRules(clipRules []svg.FillRule, rule svg.FillRule) bool {
	for _, r := range clipRules {
		if r != rule {
			return false
		}
	}
	return true
}

// clip is the tuple (Path, FillRule) that create_clip_path returns.
type clip struct {
	path *path.Path
	rule pdf.FillRule
}

// createClipPath is create_clip_path in krilla-svg src/clip_path.rs.
//
// Create a simple clip path.
func createClipPath(clipPath *svg.ClipPath, clipRule svg.FillRule) []clip {
	var clips []clip
	if inner, ok := clipPath.ClipPath(); ok {
		clips = append(clips, createClipPath(inner, clipRule)...)
	}

	// Just a dummy operation, so that in case the clip path only has hidden
	// children the clip path will still be applied and everything will be
	// hidden.
	pathBuilder := path.NewPathBuilder()
	pathBuilder.MoveTo(0.0, 0.0)

	baseTransform := clipPath.Transform()
	extendSegmentsFromGroup(clipPath.Root(), baseTransform, pathBuilder)

	p, ok := pathBuilder.Finish()
	if !ok {
		builder := path.NewPathBuilder()
		builder.MoveTo(0.0, 0.0)
		builder.LineTo(0.0, 0.0)
		p, _ = builder.Finish()
	}
	clips = append(clips, clip{path: p, rule: convertFillRule(clipRule)})
	return clips
}

// extendSegmentsFromGroup is extend_segments_from_group in krilla-svg
// src/clip_path.rs.
//
// Collect the paths of a group so that they can be used in the clip path.
func extendSegmentsFromGroup(group *svg.Group, transform path.Transform, pathBuilder *path.PathBuilder) {
	for _, child := range group.Children() {
		switch n := child.(type) {
		case *svg.Path:
			if !n.IsVisible() {
				continue
			}
			for segment := range n.Data().Segments().All() {
				switch segment.Kind {
				case path.PathSegmentMoveTo:
					p := segment.Points[0]
					transform.MapPoint(&p)
					pathBuilder.MoveTo(p.X, p.Y)
				case path.PathSegmentLineTo:
					p := segment.Points[0]
					transform.MapPoint(&p)
					pathBuilder.LineTo(p.X, p.Y)
				case path.PathSegmentQuadTo:
					points := []path.Point{segment.Points[0], segment.Points[1]}
					transform.MapPoints(points)
					pathBuilder.QuadTo(points[0].X, points[0].Y, points[1].X, points[1].Y)
				case path.PathSegmentCubicTo:
					points := []path.Point{segment.Points[0], segment.Points[1], segment.Points[2]}
					transform.MapPoints(points)
					pathBuilder.CubicTo(
						points[0].X,
						points[0].Y,
						points[1].X,
						points[1].Y,
						points[2].X,
						points[2].Y,
					)
				case path.PathSegmentClose:
					pathBuilder.Close()
				}
			}
		case *svg.Group:
			groupTransform := transform.PreConcat(n.Transform())
			extendSegmentsFromGroup(n, groupTransform, pathBuilder)
		case *svg.Text:
			// We could in theory preserve text in clip paths by using the
			// appropriate rendering mode, but for now we just use the
			// flattened version.
			if f := n.Flattened(); f != nil {
				extendSegmentsFromGroup(f, transform, pathBuilder)
			}
		}
		// Images are not valid in a clip path.
	}
}

// isSimpleClipPath is is_simple_clip_path in krilla-svg src/clip_path.rs.
//
// Check if the clip path is simple, i.e. it can be translated into a PDF
// clip path.
func isSimpleClipPath(group *svg.Group) bool {
	for _, n := range group.Children() {
		if g, ok := n.(*svg.Group); ok {
			// We can only intersect one clipping path with another one,
			// meaning that we can convert nested clip paths if a second clip
			// path is defined on the clip path itself, but not if it is
			// defined on a child.
			if _, has := g.ClipPath(); has || !isSimpleClipPath(g) {
				return false
			}
		}
	}
	return true
}

// collectClipRules is collect_clip_rules in krilla-svg src/clip_path.rs.
//
// Collect the filling rules used in the clip path.
func collectClipRules(group *svg.Group) []svg.FillRule {
	var clipRules []svg.FillRule
	for _, n := range group.Children() {
		switch c := n.(type) {
		case *svg.Path:
			if fill, ok := c.Fill(); ok {
				clipRules = append(clipRules, fill.Rule())
			}
		case *svg.Text:
			if f := c.Flattened(); f != nil {
				clipRules = append(clipRules, collectClipRules(f)...)
			}
		case *svg.Group:
			clipRules = append(clipRules, collectClipRules(c)...)
		}
	}

	return clipRules
}

// renderComplex is render_complex in krilla-svg src/clip_path.rs.
//
// Render a clip path by using alpha masks.
func renderComplex(parent *svg.Group, clipPath *svg.ClipPath, surface *pdf.Surface, pc *processContext) int {
	streamBuilder := surface.StreamBuilder()
	subSurface := streamBuilder.Surface()

	popCount := 0

	if inner, ok := clipPath.ClipPath(); ok {
		popCount += renderClipPath(parent, inner, subSurface, pc)
	}

	subSurface.PushTransform(clipPath.Transform())
	popCount++
	renderRoot(clipPath.Root(), subSurface, pc)

	for range popCount {
		subSurface.Pop()
	}

	subSurface.Finish()
	stream := streamBuilder.Finish()

	surface.PushMask(pdf.NewMask(stream, pdf.MaskTypeAlpha))
	return 1
}
