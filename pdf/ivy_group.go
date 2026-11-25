// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

// This file ports no file of krilla. It holds the transparency groups and the
// stroke adjustment that the device of ivy needs and krilla does not have
// (D62 item 11, D68).

// okagePushInstruction is a push of a surface that is not in krilla. Pop calls
// its method pop.
type okagePushInstruction interface {
	pushInstruction
	pop(s *Surface)
}

// Group is a transparency group of PDF, which Surface.PushGroup starts. It is
// not in krilla (D68).
//
// The zero Group draws its content at an opacity of 0. Set Opacity to
// NormalizedF32One for a group without transparency.
type Group struct {
	// Opacity is the opacity of the group, the entries CA and ca of its
	// graphics state.
	Opacity NormalizedF32
	// BlendMode is the blend mode of the group onto its backdrop.
	BlendMode BlendMode
	// Isolated starts the group on a transparent backdrop, the flag I of
	// the group. Without it, the content of the group blends with the
	// content under the group.
	Isolated bool
	// Knockout makes each object of the group draw over the backdrop of the
	// group, and not over the earlier objects of the group. This is the
	// flag K of the group (ISO 32000-2, 11.4.8).
	Knockout bool
	// Mask is the soft mask of the group, or nil.
	Mask *Mask
}

// pushInstructionGroup is the push of PushGroup.
type pushInstructionGroup struct {
	group Group
}

func (pushInstructionGroup) isPushInstruction() {}

// PushGroup starts a transparency group. The content that the surface draws
// until the matching Pop goes into one form XObject with a group of the
// transparency type, which has the flags I and K of the group. Pop draws the
// XObject with a graphics state that holds the opacity, the blend mode and
// the soft mask of the group. It is not in krilla (D68).
//
// krilla draws an opacity, a mask and an isolated group each as an XObject
// of its own. One group of PushGroup holds all of them, so that a group that
// is not isolated, or that is a knockout group, keeps its meaning in PDF.
func (s *Surface) PushGroup(g Group) {
	s.pushInstructions = append(s.pushInstructions, pushInstructionGroup{group: g})
	s.pushSubBuilder()
}

// pop draws the XObject of the group.
func (i pushInstructionGroup) pop(s *Surface) {
	g := i.group
	stream := s.popSubBuilder()
	b := s.bd.getMut()
	state := newExtGState()
	if g.Opacity != NormalizedF32One {
		state = state.strokingAlpha(g.Opacity).nonStrokingAlpha(g.Opacity)
	}
	if g.BlendMode != BlendModeNormal {
		state = state.blendMode(g.BlendMode.toPDF())
	}
	if g.Mask != nil {
		state = state.mask(g.Mask.coverContent(stream), s.sc, s.cc)
		b.usesMask = true
	}
	b.drawXObject(s.sc, s.cc, newGroupXObject(stream, g.Isolated, g.Knockout), state)
}

// newGroupXObject is newXObject for a group of PushGroup. The XObject always
// has a group of the transparency type, with the flag K of a knockout group.
func newGroupXObject(stream *Stream, isolated, knockout bool) xObject {
	x := newXObject(stream, isolated, groupColorSpaceRGB, nil)
	if !knockout {
		return x
	}
	r := x.p.value
	r.knockout = true
	return xObject{p: newPrehashed(r)}
}

// pushInstructionStrokeAdjustment is the push of PushStrokeAdjustment.
type pushInstructionStrokeAdjustment struct{}

func (pushInstructionStrokeAdjustment) isPushInstruction() {}

// PushStrokeAdjustment sets the automatic stroke adjustment of PDF, the
// entry SA of the graphics state, for the strokes until the matching Pop. A
// viewer then draws a thin line with a width of at least one pixel, and puts
// its edges on the borders of the pixels, as Paint.StrokeAdjust of okage
// does (D54). It is not in krilla (D68).
func (s *Surface) PushStrokeAdjustment(adjust bool) {
	s.pushInstructions = append(s.pushInstructions, pushInstructionStrokeAdjustment{})
	b := s.bd.getMut()
	b.saveGraphicsState()
	b.graphicsStates.combine(newExtGState().strokeAdjustment(adjust))
}

// pop restores the graphics state of the surface.
func (pushInstructionStrokeAdjustment) pop(s *Surface) {
	s.bd.getMut().restoreGraphicsState()
}

// strokeAdjustment returns a copy of the graphics state with the flag SA. It
// is not in krilla (D68).
func (g extGState) strokeAdjustment(adjust bool) extGState {
	g.r.strokeAdjust = adjust
	g.r.hasStrokeAdjust = true
	return g
}
