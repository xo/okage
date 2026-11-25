// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

// Graphic is Graphic in krilla src/graphics/graphic.rs. Surface.DrawGraphic
// draws it.
//
// A cacheable graphic. You can use this for large graphics objects which you
// want to reuse in multiple locations in the same document. Embedding the
// same graphic multiple times will ensure that it is only actually written
// once in the PDF document, which might lead to better file sizes.
//
// IMPORTANT: Note that you must only use a graphic in the document that you
// created it with! If you use it in a different document, you will end up
// with an invalid PDF file.
type Graphic struct {
	// xObject is the stream of the graphic.
	xObject xObject
}

// NewGraphic is Graphic::new in krilla src/graphics/graphic.rs.
//
// Create a new graphic. stream contains the content description of the
// graphic. isolated indicates whether the contents of the graphic should be
// isolated from wherever the graphic is invoked.
func NewGraphic(stream *Stream, isolated bool) Graphic {
	return Graphic{xObject: newXObject(stream, isolated, groupColorSpaceNone, nil)}
}
