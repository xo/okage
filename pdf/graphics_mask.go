// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import "github.com/xo/okage/internal/pdfwriter"

// Mask is Mask in krilla src/graphics/mask.rs. Surface.PushMask takes it.
//
// A mask. Can be a luminance mask or an alpha mask.
//
// IMPORTANT: Note that you must only use a mask in the document that you
// created it with! If you use it in a different document, you will end up
// with an invalid PDF file.
type Mask struct {
	// stream is the stream of the mask.
	stream *Stream
	// maskType is the type of the mask.
	maskType MaskType
	// customBBox is a custom bbox of the mask, or nil. The only reason we
	// need this is that for gradients with transparencies, we create a
	// custom mask where we call the shading operator. In this case, we want
	// to manually set the bbox of the underlying XObject to match the shape
	// that the gradient is being applied to.
	customBBox *Rect

	// backdrop is the backdrop color BC of a luminosity mask, or nil. It is
	// not in krilla (D68).
	backdrop *ColorRGB
	// transfer is the transfer function TR of the mask, or nil. It is not
	// in krilla (D68).
	transfer *[256]uint8
}

// NewMask is Mask::new in krilla src/graphics/mask.rs.
//
// Create a new mask. stream contains the content description of the mask,
// and maskType indicates the type of mask.
func NewMask(stream *Stream, maskType MaskType) *Mask {
	return &Mask{
		stream:     stream,
		maskType:   maskType,
		customBBox: nil,
	}
}

// newMaskFromShading is Mask::new_from_shading in krilla
// src/graphics/mask.rs. It returns nil, which is None, when each stop is
// opaque.
//
// Create a new mask for a shading to encode the opacity channels.
func newMaskFromShading(gradientProperties gradientProperties, shadingTransform Transform, bbox Rect, sc *serializeContext, cc *chunkContainer) *Mask {
	var stops []Stop
	switch gp := gradientProperties.(type) {
	case radialAxialGradient:
		stops = gp.stops
	case postScriptGradient:
		stops = gp.stops
	}
	opaque := true
	for _, s := range stops {
		if s.Opacity.Get() != 1.0 {
			opaque = false
			break
		}
	}
	if opaque {
		return nil
	}

	shadingFunction := newShadingFunction(gradientProperties, true)

	shadingStream := shadingMaskStream(sc, cc, shadingTransform, shadingFunction)

	return &Mask{
		stream:     shadingStream,
		maskType:   MaskTypeLuminosity,
		customBBox: &bbox,
	}
}

// MaskType is MaskType in krilla src/graphics/mask.rs.
//
// A mask type.
type MaskType uint8

// The values of MaskType.
const (
	// MaskTypeLuminosity is a luminosity mask.
	MaskTypeLuminosity MaskType = iota
	// MaskTypeAlpha is an alpha mask.
	MaskTypeAlpha
)

// toName is MaskType::to_name in krilla src/graphics/mask.rs.
//
// Return the PDF name of the mask type.
func (t MaskType) toName() pdfwriter.Name {
	if t == MaskTypeAlpha {
		return pdfwriter.Name("Alpha")
	}
	return pdfwriter.Name("Luminosity")
}

// typeID names the type for sipHash.
func (*Mask) typeID() string { return "Mask" }

// hash is the derived Hash of Mask in krilla src/graphics/mask.rs.
func (m *Mask) hash(h *hasher) {
	m.stream.hash(h)
	h.usize(int(m.maskType))
	if m.customBBox == nil {
		h.usize(0)
	} else {
		h.usize(1)
		h.rect(*m.customBBox)
	}
	// okage: the backdrop and the transfer function add to the hash only
	// when they are set, so that the hash of every other mask stays the hash
	// of upstream (D68).
	m.hashBackdropTransfer(h)
}

// serialize is Cacheable::serialize for Mask in krilla
// src/graphics/mask.rs. It writes the stream as a transparency group once,
// and the dictionary of the soft mask.
func (m *Mask) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	xObject := sc.registerCacheable(cc, newXObject(m.stream, false, true, m.customBBox))

	chunk := cc.nonStream.masks
	dict := chunk.Indirect(rootRef).Dict()
	dict.Pair(pdfwriter.Name("Type"), pdfwriter.Name("Mask"))
	dict.Pair(pdfwriter.Name("S"), m.maskType.toName())
	dict.Pair(pdfwriter.Name("G"), xObject)
	// okage: the backdrop BC and the transfer function TR (D68).
	m.writeBackdropTransfer(sc, cc, dict)

	dict.Finish()
}

// newResource is Resourceable for Mask in krilla src/graphics/mask.rs.
func (*Mask) newResource(ref pdfwriter.Ref) xObjectResource {
	return xObjectResource{ref}
}
