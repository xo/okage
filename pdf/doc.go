// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package pdf is a port of the Rust crate krilla, at the commit e3459d06
// (D62). It writes PDF files.
//
// krilla is a high-level library that creates PDF files. It builds on the
// crate pdf-writer, which okage ports into internal/pdfwriter. It hides the
// details of PDF behind primitives such as fills, strokes, gradients, glyphs
// and images.
//
// A program starts with a Document. The Document starts the pages, and each
// Page gives a surface to draw on. Document.Finish writes the file. The file
// is in memory until Finish, because the subset of a font needs every glyph
// of the document (D62):
//
//	d := pdf.NewDocument(pdf.WithPDFVersion(pdf.PDF17))
//	ps, _ := pdf.PageSettingsFromWH(200, 200)
//	page := d.StartPage(ps)
//	// Draw on the surface of the page.
//	page.Finish()
//	if err := d.Finish(w); err != nil {
//		return err
//	}
//
// # The state of the port
//
// Phase 14 of "The PDF writer" in docs/PLAN.md is done. It ports the
// document, the pages, the serializer, the surface and the graphics without
// the images. These files are ported: geom.rs, num.rs, util.rs, error.rs,
// data.rs, lib.rs, stream.rs, resource.rs, chunk_container.rs,
// serialize.rs, document.rs, page.rs, configure/, content.rs, surface.rs and
// graphics/ without graphics/image/ and graphics/separation.rs. Each file
// of graphics/ is a file graphics_<name>.go, and the ICC profiles of krilla
// are in the folder icc. The images, the text, the metadata, the outline and
// the links come in later phases. content.go and surface.go name the parts
// of the text and the images that phases 15 and 16 add to them.
//
// The file lib.rs of upstream holds the documentation of the crate, which is
// this comment. The file data.rs holds the type Data, a shared slice of
// bytes. A []byte does that work in Go, so it has no port.
//
// The base port leaves out the tags of PDF/UA, the forms, the validators of
// PDF/A and PDF/UA with their XMP metadata, the embedded files, the
// separation colors and the pages of other PDF files (D62). krilla writes
// XMP metadata by default. pdf writes none.
//
// # How the port is built
//
// Rust borrows the state of the document in each page and each surface, and
// it drops a page or a surface at the end of its scope. Go has neither, so
// the port does this:
//
//  1. serializeContext is SerializeContext of upstream, and chunkContainer is
//     ChunkContainer. A Page, a StreamBuilder and a surface hold pointers to
//     both, as the borrows of upstream do.
//  2. A method Finish runs the impl Drop of upstream. Document.StartPage and
//     Document.Finish finish the page that is open. Page.Finish finishes the
//     surface that is open, with the function in Page.finishSurface.
//  3. A panic of krilla that a caller can cause becomes an error that wraps
//     a sentinel error of the root package, such as ErrPDFInvalidBox (D21,
//     D62). A call of a surface records its error in the serializeContext,
//     and Document.Finish returns the first one. These are the errors of a
//     surface:
//     - ErrPDFPushPop for a pop without a push, or a push without a pop.
//     - ErrPDFInvalidClipPath for a clip path that a transform makes too
//     large.
//     - ErrPDFInvalidStroke for a stroke whose width is negative or NaN.
//     - ErrPDFInvalidPath for a path to stroke whose coordinates, with the
//     width of the stroke, have a magnitude above 2^60, when krilla
//     computes the bbox of the stroke. The stroker of krilla can panic
//     there (D66).
//     - ErrPDFMixedColorSpaces for a gradient whose stops have colors with
//     different numbers of components, such as RGB and CMYK (D66).
//     - ErrPDFInvalidGradient for a gradient with no stop, or with a
//     transform that cannot be inverted where krilla needs its inverse.
//  4. The cache of the serializer keys each object with a hash. sipHash
//     hashes a name of the type, then the value, as sip_hash of upstream
//     hashes a type_id, then the value. stableHash128 gives the hash of
//     upstream, byte for byte, for the values that go into the file, such as
//     the ID of the document.
//
// # How a later phase adds its files
//
// Phases 15 and 16 port the images, the text, the metadata, the outline and
// the links. They add their files by these rules, which units 5 and 6 of
// phase 14 followed for the surface and the graphics:
//
//  1. A type that krilla writes once for each value, such as a graphics
//     state, a shading or a pattern, satisfies cacheable: the methods typeID,
//     hash and serialize. A type that a content stream names as a resource
//     also satisfies resourceable, with the method newResource that gives
//     one of the types of resource.go, such as extGStateResource. The
//     method hash writes the fields in the order of the derived Hash of
//     upstream, with the methods of hasher, such as h.rect and h.transform
//     for a Rect and a Transform. registerResourceable writes the object
//     once and returns its resource. The test type testGraphicsState in
//     resource_test.go is the pattern.
//  2. The method serialize of an object writes into the chunk of its kind
//     in chunkContainer: cc.nonStream.extGStates, colorSpaces, masks,
//     shadingFunctions and patterns for the objects without a stream, and
//     cc.streams.shadingFunctions, patterns, xObjects and iccProfiles for
//     the streams. It makes the bytes of a stream with a filterStreamBuilder,
//     such as newFilterStreamFromContentStream, and writes the filters with
//     filterStream.writeFilters.
//  3. A method of a type of an earlier file that needs a type of a later
//     file goes in the later file, because a Go method can be in any file of
//     its package. For example, surface.go holds Page.Surface,
//     StreamBuilder.Surface, StreamBuilder.SurfaceWithBBox and
//     Surface.StreamBuilder. graphics_color.go holds
//     serializeContext.registerColorSpace and the type
//     maybeDeviceColorSpace of serialize.rs. graphics_icc.go holds
//     PDFVersion.rgbICC and PDFVersion.greyICC of configure/version.rs. A
//     unit that needs another method of Validators adds it to validators, in
//     its own file, with the answer of an empty set of validators.
//  4. A surface gives its stream to the code that made it with a function,
//     as the finish_fn of upstream. The number of marked content IDs of
//     upstream is for the tags, so the function takes the stream only.
//
// The images of graphics/image add their chunks to cc.streams.images, with
// the error of a decode. The text, the metadata, the outline and the links add their
// fields to chunkContainer and globalObjects, and their steps to
// serializeContext.finish and chunkContainer.finish, where the comments of
// those functions name them. The annotations of interactive/annotation.rs
// satisfy the interface annotation of page.go, and Page.AddAnnotation
// appends them to Page.annotations.
package pdf
