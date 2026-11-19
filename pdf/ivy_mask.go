// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import "github.com/xo/okage/internal/pdfwriter"

// This file ports no file of krilla. It holds the backdrop color and the
// transfer function of a soft mask, which the device of ivy needs and krilla
// does not have (D62 item 11, D68).

// WithBackdrop returns a copy of the mask with the backdrop color BC. A
// viewer composites the content of a luminosity mask over an opaque backdrop
// of this color, as okage.MaskFromLuminance does (D56). The group of the mask
// is in the RGB color space, so the color is an RGB color. A mask of the type
// alpha ignores the backdrop. It is not in krilla (D68).
func (m *Mask) WithBackdrop(c ColorRGB) *Mask {
	n := *m
	n.backdrop = &c
	return &n
}

// WithTransfer returns a copy of the mask with the transfer function TR. A
// viewer changes each value v of the mask, from 0 to 255, to table[v], as
// okage.Mask.ApplyTransfer does (D56). The file holds the table as a sampled
// function of 256 samples of 8 bits. A nil table removes the transfer
// function. It is not in krilla (D68).
func (m *Mask) WithTransfer(table *[256]uint8) *Mask {
	n := *m
	n.transfer = nil
	if table != nil {
		t := *table
		n.transfer = &t
	}
	return &n
}

// coverContent returns the mask with a bbox that covers its own content and
// the content that it masks, when the mask has a backdrop or a transfer
// function. Outside the bbox of its group, a mask has the value of the
// backdrop after the transfer function. Poppler 26.11 takes the backdrop
// there without the transfer function, so the bbox of the group covers the
// content. Both streams are in the user space of the push of the mask.
func (m *Mask) coverContent(content *Stream) *Mask {
	if m.backdrop == nil && m.transfer == nil {
		return m
	}
	bbox := m.stream.bbox
	if m.customBBox != nil {
		bbox = *m.customBBox
	}
	rectExpand(&bbox, content.bbox)
	n := *m
	n.customBBox = &bbox
	return &n
}

// hashBackdropTransfer writes the backdrop and the transfer function of the
// mask, when they are set.
func (m *Mask) hashBackdropTransfer(h *hasher) {
	if m.backdrop != nil {
		h.usize(1)
		m.backdrop.hash(h)
	}
	if m.transfer != nil {
		h.usize(2)
		h.bytes(m.transfer[:])
	}
}

// writeBackdropTransfer writes the entries BC and TR of the dictionary of the
// soft mask.
func (m *Mask) writeBackdropTransfer(sc *serializeContext, cc *chunkContainer, dict *pdfwriter.Dict) {
	if m.backdrop != nil && m.maskType == MaskTypeLuminosity {
		a := dict.Insert(pdfwriter.Name("BC")).Array()
		for _, v := range m.backdrop.toPDFColor() {
			a.Item(pdfwriter.Real(v))
		}
		a.Finish()
	}
	if m.transfer != nil {
		ref := sc.registerCacheable(cc, transferFunction(*m.transfer))
		dict.Pair(pdfwriter.Name("TR"), ref)
	}
}

// transferFunction is the transfer function of a soft mask: a sampled
// function of one input and one output, with a sample of 8 bits for each
// value of the mask.
type transferFunction [256]uint8

// typeID names the type for sipHash.
func (transferFunction) typeID() string { return "transferFunction" }

// hash writes the samples.
func (f transferFunction) hash(h *hasher) {
	h.bytes(f[:])
}

// serialize writes the function as a stream.
func (f transferFunction) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	chunk := sc.newChunk()
	stream := newFilterStreamAutoCompressed(f[:]).finish(sc.serializeSettings())
	fn := chunk.SampledFunction(rootRef, stream.encodedData())
	stream.writeFilters(fn.Dict)
	fn.Domain(0, 1).Range(0, 1).Size(256).BitsPerSample(8)
	fn.Finish()
	cc.streams.shadingFunctions = append(cc.streams.shadingFunctions, chunk)
}
