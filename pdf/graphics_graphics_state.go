// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// extGStateRepr is Repr in krilla src/graphics/graphics_state.rs, the inner
// representation of an external graphics state. Each Option of upstream is
// a value and a flag that says whether it is set.
type extGStateRepr struct {
	// nonStrokingAlpha is the non-stroking alpha.
	nonStrokingAlpha    NormalizedF32
	hasNonStrokingAlpha bool
	// strokingAlpha is the stroking alpha.
	strokingAlpha    NormalizedF32
	hasStrokingAlpha bool
	// blendMode is the blend mode.
	blendMode    pdfwriter.BlendMode
	hasBlendMode bool
	// mask is the reference of an active mask.
	mask    pdfwriter.Ref
	hasMask bool
}

// extGState is ExtGState in krilla src/graphics/graphics_state.rs. Upstream
// holds its Repr in an Arc and copies it on a change with Arc::make_mut. The
// port holds the Repr as a value, so a copy of an extGState is a new state,
// and a method returns the changed copy, as the builder of upstream does.
//
// A graphics state containing information about the current stroking alpha,
// the current non-stroking alpha, the current blend mode and the current
// mask.
type extGState struct {
	r extGStateRepr
}

// newExtGState is ExtGState::new in krilla src/graphics/graphics_state.rs.
//
// Create a new, empty graphics state.
func newExtGState() extGState {
	return extGState{}
}

// strokingAlpha is ExtGState::stroking_alpha in krilla
// src/graphics/graphics_state.rs.
//
// Create a new graphics state with a stroking alpha.
func (g extGState) strokingAlpha(strokingAlpha NormalizedF32) extGState {
	g.r.strokingAlpha = strokingAlpha
	g.r.hasStrokingAlpha = true
	return g
}

// nonStrokingAlpha is ExtGState::non_stroking_alpha in krilla
// src/graphics/graphics_state.rs.
//
// Create a new graphics state with a non-stroking alpha.
func (g extGState) nonStrokingAlpha(nonStrokingAlpha NormalizedF32) extGState {
	g.r.nonStrokingAlpha = nonStrokingAlpha
	g.r.hasNonStrokingAlpha = true
	return g
}

// blendMode is ExtGState::blend_mode in krilla
// src/graphics/graphics_state.rs.
//
// Create a new graphics state with a blend mode.
func (g extGState) blendMode(blendMode pdfwriter.BlendMode) extGState {
	g.r.blendMode = blendMode
	g.r.hasBlendMode = true
	return g
}

// mask is ExtGState::mask in krilla src/graphics/graphics_state.rs. It
// writes the mask once, and keeps its reference.
//
// Create a new graphics state with a mask.
func (g extGState) mask(mask *Mask, sc *serializeContext, cc *chunkContainer) extGState {
	maskRef := sc.registerCacheable(cc, mask)
	g.r.mask = maskRef
	g.r.hasMask = true
	return g
}

// empty is ExtGState::empty in krilla src/graphics/graphics_state.rs.
//
// Check whether the graphics state is empty.
func (g extGState) empty() bool {
	return !g.r.hasMask &&
		!g.r.hasStrokingAlpha &&
		!g.r.hasNonStrokingAlpha &&
		!g.r.hasBlendMode
}

// combine is ExtGState::combine in krilla src/graphics/graphics_state.rs.
//
// Integrate another graphics state into the current one. This is done by
// replacing all active properties of the other graphics state in the current
// graphics state, while leaving the inactive ones unchanged.
func (g *extGState) combine(other extGState) {
	if other.r.hasStrokingAlpha {
		g.r.strokingAlpha = other.r.strokingAlpha
		g.r.hasStrokingAlpha = true
	}

	if other.r.hasNonStrokingAlpha {
		g.r.nonStrokingAlpha = other.r.nonStrokingAlpha
		g.r.hasNonStrokingAlpha = true
	}

	if other.r.hasBlendMode {
		g.r.blendMode = other.r.blendMode
		g.r.hasBlendMode = true
	}

	if other.r.hasMask {
		g.r.mask = other.r.mask
		g.r.hasMask = true
	}
}

// typeID names the type for sipHash.
func (extGState) typeID() string { return "extGState" }

// hash is the derived Hash of ExtGState in krilla
// src/graphics/graphics_state.rs: each Option, in the order of Repr.
func (g extGState) hash(h *hasher) {
	if g.r.hasNonStrokingAlpha {
		h.usize(1)
		hashF32Bits(h, g.r.nonStrokingAlpha.Get())
	} else {
		h.usize(0)
	}
	if g.r.hasStrokingAlpha {
		h.usize(1)
		hashF32Bits(h, g.r.strokingAlpha.Get())
	} else {
		h.usize(0)
	}
	if g.r.hasBlendMode {
		h.usize(1)
		h.usize(int(g.r.blendMode))
	} else {
		h.usize(0)
	}
	if g.r.hasMask {
		h.usize(1)
		h.i32(g.r.mask.Get())
	} else {
		h.usize(0)
	}
}

// serialize is Cacheable::serialize for ExtGState in krilla
// src/graphics/graphics_state.rs. An alpha that is not 1, a blend mode that
// is not normal and a mask record the error Transparency.
func (g extGState) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	chunk := cc.nonStream.extGStates

	extSt := chunk.ExtGraphics(rootRef)
	if g.r.hasNonStrokingAlpha {
		nsa := g.r.nonStrokingAlpha
		if nsa != NormalizedF32One {
			sc.registerValidationError(validationError{kind: validationTransparency, location: sc.location})
		}

		extSt.NonStrokingAlpha(nsa.Get())
	}

	if g.r.hasStrokingAlpha {
		sa := g.r.strokingAlpha
		if sa != NormalizedF32One {
			sc.registerValidationError(validationError{kind: validationTransparency, location: sc.location})
		}

		extSt.StrokingAlpha(sa.Get())
	}

	if g.r.hasBlendMode {
		bm := g.r.blendMode
		if bm != pdfwriter.BlendModeNormal {
			sc.registerValidationError(validationError{kind: validationTransparency, location: sc.location})
		}

		extSt.BlendMode(bm)
	}

	if g.r.hasMask {
		sc.registerValidationError(validationError{kind: validationTransparency, location: sc.location})

		extSt.Pair(pdfwriter.Name("SMask"), g.r.mask)
	}

	extSt.Finish()
}

// newResource is Resourceable for ExtGState in krilla
// src/graphics/graphics_state.rs.
func (extGState) newResource(ref pdfwriter.Ref) extGStateResource {
	return extGStateResource{ref}
}

// graphicsState is GraphicsState in krilla src/graphics/graphics_state.rs.
// The derived Hash of upstream hashes no graphics state in the cache of the
// serializer, so it has no method hash.
//
// A simulation of the PDF graphics state, so that we can write our
// transforms and graphics state all at once when adding an image or a path
// instead of having to use save_state and restore_state excessively.
type graphicsState struct {
	// state is the field ext_g_state of upstream. The method extGState
	// gives it.
	state extGState
	ctm   Transform
}

// defaultGraphicsState is the impl Default for GraphicsState in krilla
// src/graphics/graphics_state.rs.
func defaultGraphicsState() graphicsState {
	return graphicsState{
		state: newExtGState(),
		ctm:   path.IdentityTransform(),
	}
}

// combine is GraphicsState::combine in krilla
// src/graphics/graphics_state.rs.
func (s *graphicsState) combine(other extGState) {
	s.state.combine(other)
}

// concatTransform is GraphicsState::concat_transform in krilla
// src/graphics/graphics_state.rs.
func (s *graphicsState) concatTransform(transform Transform) {
	s.ctm = s.ctm.PreConcat(transform)
}

// transform is GraphicsState::transform in krilla
// src/graphics/graphics_state.rs.
func (s *graphicsState) transform() Transform {
	return s.ctm
}

// extGState is GraphicsState::ext_g_state in krilla
// src/graphics/graphics_state.rs.
func (s *graphicsState) extGState() extGState {
	return s.state
}

// graphicsStates is GraphicsStates in krilla
// src/graphics/graphics_state.rs.
//
// A collection of graphics states, simulates the stack-like structure used.
type graphicsStates struct {
	graphicsStates []graphicsState
}

// newGraphicsStates is GraphicsStates::new in krilla
// src/graphics/graphics_state.rs.
func newGraphicsStates() *graphicsStates {
	return &graphicsStates{
		graphicsStates: []graphicsState{defaultGraphicsState()},
	}
}

// cur is GraphicsStates::cur and GraphicsStates::cur_mut in krilla
// src/graphics/graphics_state.rs. A content builder pops no more states than
// it saves, so the stack is never empty.
func (s *graphicsStates) cur() *graphicsState {
	return &s.graphicsStates[len(s.graphicsStates)-1]
}

// saveState is GraphicsStates::save_state in krilla
// src/graphics/graphics_state.rs.
func (s *graphicsStates) saveState() {
	state := *s.cur()
	s.graphicsStates = append(s.graphicsStates, state)
}

// restoreState is GraphicsStates::restore_state in krilla
// src/graphics/graphics_state.rs.
func (s *graphicsStates) restoreState() {
	s.graphicsStates = s.graphicsStates[:len(s.graphicsStates)-1]
}

// combine is GraphicsStates::combine in krilla
// src/graphics/graphics_state.rs.
func (s *graphicsStates) combine(other extGState) {
	s.cur().combine(other)
}

// transform is GraphicsStates::transform in krilla
// src/graphics/graphics_state.rs.
func (s *graphicsStates) transform(transform Transform) {
	s.cur().concatTransform(transform)
}

// transformBBox is GraphicsStates::transform_bbox in krilla
// src/graphics/graphics_state.rs. krilla unwraps the transformed rectangle,
// and panics when it is not valid, such as for a transform that is not
// finite. The port returns false in that case, and the caller decides.
//
// Important: This does not take the root transform of the corresponding
// ContentBuilder into account, because we want it to be in krilla
// coordinates, not in PDF coordinates.
func (s *graphicsStates) transformBBox(bbox Rect) (Rect, bool) {
	return bbox.Transform(s.cur().transform())
}
