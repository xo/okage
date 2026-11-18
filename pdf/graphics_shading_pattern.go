// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import "github.com/xo/okage/internal/pdfwriter"

// shadingPattern is ShadingPattern in krilla
// src/graphics/shading_pattern.rs, with its Repr. Upstream holds the Repr in
// an Arc, and the port holds it as a value.
type shadingPattern struct {
	shadingFunction  shadingFunction
	shadingTransform Transform
}

// newShadingPattern is ShadingPattern::new in krilla
// src/graphics/shading_pattern.rs.
func newShadingPattern(gradientProperties gradientProperties, shadingTransform Transform) shadingPattern {
	return shadingPattern{
		// CTM doesn't need to be included to calculate the domain of the
		// shading function.
		shadingFunction:  newShadingFunction(gradientProperties, false),
		shadingTransform: shadingTransform,
	}
}

// typeID names the type for sipHash.
func (shadingPattern) typeID() string { return "shadingPattern" }

// hash is the impl Hash for Repr in krilla src/graphics/shading_pattern.rs.
func (p shadingPattern) hash(h *hasher) {
	p.shadingFunction.hash(h)
	h.transform(p.shadingTransform)
}

// serialize is Cacheable::serialize for ShadingPattern in krilla
// src/graphics/shading_pattern.rs.
func (p shadingPattern) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	shadingRef := sc.registerCacheable(cc, p.shadingFunction)
	chunk := cc.nonStream.patterns
	sp := chunk.ShadingPattern(rootRef)
	sp.Pair(pdfwriter.Name("Shading"), shadingRef)
	sp.Matrix(transformToPDF(p.shadingTransform))

	sp.Finish()
}

// newResource is Resourceable for ShadingPattern in krilla
// src/graphics/shading_pattern.rs.
func (shadingPattern) newResource(ref pdfwriter.Ref) patternResource {
	return patternResource{ref}
}
