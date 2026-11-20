// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import "github.com/xo/okage/internal/pdfwriter"

// xObjectRepr is Repr in krilla src/graphics/xobject.rs. customBBox is nil
// for None.
type xObjectRepr struct {
	stream                      *Stream
	isolated                    bool
	transparencyGroupColorSpace bool
	customBBox                  *Rect
	// knockout is the flag K of the group. It is not in krilla, and only
	// Surface.PushGroup sets it (D68).
	knockout bool
}

// typeID names the type for sipHash.
func (xObjectRepr) typeID() string { return "xObjectRepr" }

// hash is the derived Hash of Repr in krilla src/graphics/xobject.rs.
func (r xObjectRepr) hash(h *hasher) {
	r.stream.hash(h)
	h.bool(r.isolated)
	h.bool(r.transparencyGroupColorSpace)
	if r.customBBox == nil {
		h.usize(0)
	} else {
		h.usize(1)
		h.rect(*r.customBBox)
	}
	// okage: a knockout group adds its flag, so that the hash of every
	// other XObject stays the hash of upstream (D68).
	if r.knockout {
		h.bool(true)
	}
}

// xObject is XObject in krilla src/graphics/xobject.rs. It holds its Repr
// with its hash, as the Arc<Prehashed<Repr>> of upstream.
type xObject struct {
	p prehashed[xObjectRepr]
}

// newXObject is XObject::new in krilla src/graphics/xobject.rs. A nil
// customBBox is None.
func newXObject(stream *Stream, isolated, transparencyGroupColorSpace bool, customBBox *Rect) xObject {
	// In case a mask was invoked in the content stream, we _always_ create a
	// new transparency group. Please see
	// <https://github.com/typst/typst/issues/5509>. Just to provide a brief
	// explanation: I have not found any mention of a transparency group
	// being required when using a soft mask in the PDF spec. However, Apple
	// Preview (and only them!) have a weird bug where if you transform an
	// XObject with a soft mask but not transparency group, the transform
	// gets applied twice to the mask, resulting in rendering issues. Using a
	// transparency group in this case seems to fix the issue.
	// TODO: Apply group transparency to page as well.
	transparencyGroupColorSpace = transparencyGroupColorSpace || stream.usesMask

	if customBBox != nil {
		r := *customBBox
		customBBox = &r
	}
	return xObject{p: newPrehashed(xObjectRepr{
		stream:                      stream,
		isolated:                    isolated,
		transparencyGroupColorSpace: transparencyGroupColorSpace,
		customBBox:                  customBBox,
	})}
}

// isEmpty is XObject::is_empty in krilla src/graphics/xobject.rs.
func (x xObject) isEmpty() bool {
	return x.p.value.stream.isEmpty()
}

// bbox is XObject::bbox in krilla src/graphics/xobject.rs.
func (x xObject) bbox() Rect {
	if x.p.value.customBBox != nil {
		return *x.p.value.customBBox
	}
	return x.p.value.stream.bbox
}

// typeID names the type for sipHash.
func (xObject) typeID() string { return "xObject" }

// hash is the derived Hash of XObject in krilla src/graphics/xobject.rs.
func (x xObject) hash(h *hasher) {
	x.p.write(h)
}

// serialize is Cacheable::serialize for XObject in krilla
// src/graphics/xobject.rs. The struct parents of upstream are for the tags,
// which the base port leaves out (D62).
func (x xObject) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	r := x.p.value
	chunk := sc.newChunk()

	for _, validationError := range r.stream.validationErrors {
		sc.registerValidationError(validationError)
	}

	useTransparencyGroup := r.isolated || r.transparencyGroupColorSpace

	if useTransparencyGroup {
		sc.registerValidationError(validationError{kind: validationTransparency, location: sc.location})
	}

	serializeSettings := sc.serializeSettings()

	// "Ordinarily, the CS entry may be present only for isolated
	// transparency groups (those for which I is true), and even then it is
	// optional. However, this entry shall be present in the group attributes
	// dictionary for any transparency group XObject that has no parent group
	// or page from which to inherit."
	//
	// So while we don't technically always need to write it, let's just
	// play it save and always add a group color space in case we have a
	// transparency group.
	var transparencyGroupCS *maybeDeviceColorSpace
	if useTransparencyGroup {
		cs := sc.registerColorSpace(cc, colorSpaceRGB(serializeSettings.noDeviceCS))
		transparencyGroupCS = &cs
	}

	// okage: the resources come first, so that Document.Finish can write
	// the stream again when the subset of one of its fonts fails. They are
	// in another chunk, so the bytes do not change. write keeps no pointer
	// to the stream, whose content the garbage collector can free (D80).
	resources := r.stream.resourceDictionary.writeResources(sc, cc.nonStream.resourceDictionaries)
	bbox := x.bbox()
	isolated, knockout := r.isolated, r.knockout
	write := func(chunk *pdfwriter.Chunk, content []byte) {
		xObjectStream := newFilterStreamFromContentStream(content, serializeSettings).finish(serializeSettings)
		xObj := chunk.FormXObject(rootRef, xObjectStream.encodedData())
		xObjectStream.writeFilters(xObj.Dict)

		resources.write(xObj)
		xObj.BBox(rectToPDFRect(bbox))

		if useTransparencyGroup {
			group := xObj.Group()
			transparency := group.Transparency()

			if isolated {
				transparency.Isolated(isolated)
			}

			// okage: the flag K of a knockout group (D68).
			if knockout {
				transparency.Knockout(true)
			}

			if transparencyGroupCS != nil {
				pdfCS := transparency.Insert(pdfwriter.Name("CS"))

				switch transparencyGroupCS.kind {
				case maybeDeviceRGB:
					pdfCS.Primitive(pdfwriter.Name(deviceRGB))
				case maybeDeviceResource:
					// Can only be SRGB.
					pdfCS.Primitive(transparencyGroupCS.cs.getRef())
				default:
					panic("unreachable")
				}
			}

			// Upstream also finishes transparency, which is a reference to
			// group, so that call does nothing.
			group.Finish()
		}

		xObj.Finish()
	}
	write(chunk, r.stream.content)
	sc.registerTextStream(chunk, &r.stream.resourceDictionary, write)
	cc.streams.xObjects = append(cc.streams.xObjects, chunk)
}

// newResource is Resourceable for XObject in krilla src/graphics/xobject.rs.
func (xObject) newResource(ref pdfwriter.Ref) xObjectResource {
	return xObjectResource{ref}
}
