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
// Phases 14, 15 and 16 of "The PDF writer" in docs/PLAN.md are done. They
// port the document, the pages, the serializer, the surface, the graphics,
// the images, the text, the metadata, the outline and the links. These files
// are ported: geom.rs, num.rs, util.rs, error.rs, data.rs, lib.rs,
// stream.rs, resource.rs, chunk_container.rs, serialize.rs, document.rs,
// page.rs, configure/, content.rs, surface.rs, graphics/ without
// graphics/separation.rs, text/ without the standard fonts,
// interchange/metadata.rs, interchange/outline.rs, interactive/action.rs,
// interactive/annotation.rs and interactive/destination.rs.
//
// Each file of graphics/ is a file graphics_<name>.go, and the ICC profiles
// of krilla are in the folder icc. graphics/image/mod.rs and png.rs are
// graphics_image.go and graphics_image_png.go. graphics_image_decode.go
// reads the headers of the image files in place of the decoders of
// upstream. The section Text names the files of text/. metadata.rs,
// outline.rs, destination.rs, action.rs and annotation.rs are metadata.go,
// outline.go, destination.go, action.go and annotation.go, without the
// forms. Document.SetMetadata, Document.SetOutline,
// Document.RegisterNamedDestination and Page.AddAnnotation take them.
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
//     surface, of the text, of the links and of the outline:
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
//     - ErrPDFInvalidGradient for a gradient with a transform that cannot
//     be inverted where krilla needs its inverse.
//     - ErrPDFImage for an image that does not decode, or a nil image, and
//     ErrPDFSixteenBitImage for an image of 16 bits in PDF 1.4.
//     - ErrPDFDuplicateNamedDestination for a name with two different
//     destinations, which krilla returns too.
//     - ErrPDFInvalidDestination for a destination to a page that does not
//     exist.
//     - ErrPDFInvalidAnnotation for a link annotation with no target, with
//     quadrilaterals that give no rectangle, or with a rectangle that does
//     not flip into the coordinates of PDF.
//     - ErrPDFInvalidGlyph for a glyph whose range is not a range of its
//     text, or whose ID is above 65535 in a CID font.
//     - ErrPDFFont for a font that Document.Finish cannot embed, which
//     krilla returns too.
//  4. The cache of the serializer keys each object with a hash. sipHash
//     hashes a name of the type, then the value, as sip_hash of upstream
//     hashes a type_id, then the value. stableHash128 gives the hash of
//     upstream, byte for byte, for the values that go into the file, such as
//     the ID of the document.
//
// # How a phase adds its files
//
// Phases 14, 15 and 16 added their files by these rules, and a later phase
// follows them too:
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
// the error of a decode. The text, the metadata, the outline and the links
// add their fields to chunkContainer and globalObjects, and their steps to
// serializeContext.finish and chunkContainer.finish. The annotations of interactive/annotation.rs
// satisfy the interface annotation of page.go, and Page.AddAnnotation
// appends them to Page.annotations.
//
// # Text
//
// Phase 16 ports these files of src/text: mod.rs, font.rs, cid.rs,
// group.rs, type3.rs, shape.rs and glyph/. Each file is a file
// text_<name>.go, such as text_cid.go and text_glyph_colr.go, and mod.rs is
// text.go. content.go holds the methods of contentBuilder for the text,
// and surface.go holds DrawGlyphs and DrawText.
//
// A Font holds the bytes of a TrueType, OpenType or collection font, with
// the coordinates of its variations. Surface.DrawGlyphs draws the glyphs
// that the caller places, with the text of their clusters, which the
// ToUnicode maps and the ActualText spans hold (D62, question 27).
// Surface.DrawText shapes a text with typesetting/harfbuzz first. A glyph
// with outlines and no color data goes into the CID font of its font, which
// holds the subset of internal/subsetter. A glyph with COLR, sbix, CBDT or
// SVG data goes into a Type 3 font. internal/colr walks the paint graph of
// a COLR glyph (D32).
//
// The port differs from krilla in these ways:
//
//  1. skrifa reads the metrics, the outlines and the bitmap strikes of a
//     font in upstream. The port reads them with typesetting and with the
//     bytes of the tables, by the rules of skrifa 0.44 (D52, D62). The
//     outlines take the order of skrifa, and for a CFF or CFF2 font, the
//     rules of NopFilterSink of read-fonts.
//  2. krilla returns None from Font::new for data that it cannot read.
//     NewFont returns an error that wraps okage.ErrPDFInvalidFont. A font
//     that is not TrueType or OpenType gives this error, and the caller
//     draws its glyphs as paths (question 25). KrillaError::Font is
//     okage.ErrPDFFont.
//  3. krilla panics for a glyph whose range is not a range of the text, and
//     for a glyph ID above 65535 in a CID font. DrawGlyphs then records
//     okage.ErrPDFInvalidGlyph, which Document.Finish returns, and draws
//     nothing.
//  4. harfrust sets no language when it guesses the properties of a text,
//     and typesetting takes the language of the locale. naiveShape sets no
//     language, as harfrust does.
//  5. typesetting computes the deltas of HVAR in float32, and skrifa in
//     fixed point. At a location of a variable font that is not exact in
//     F2Dot14, a width of the CID font can differ by 1 unit (D65).
//  6. The standard fonts of PDF are left out. Only the forms of krilla use
//     them, and the base port has no forms (D62).
//  7. WithSVGGlyphs is the field render_svg_glyph_fn of SerializeSettings.
//     Its default is nil, which draws no SVG glyph, as the default function
//     of upstream does. pdfsvg.RenderGlyph draws the SVG glyphs, as
//     render_svg_glyph of krilla-svg does.
//
// # The additions for ivy
//
// Phase 18 adds what the PDF device of ivy needs and krilla does not have
// (D62 item 11, D68). The files pdf/ivy_*.go hold them:
//
//   - Surface.PushGroup draws a transparency group with the flags I and K,
//     and an opacity, a blend mode and a soft mask in one graphics state.
//   - Mask.WithBackdrop and Mask.WithTransfer write the entries BC and TR
//     of a soft mask.
//   - The field Extend of LinearGradient and RadialGradient writes the
//     entry Extend of the shading, for the decal spread mode of okage and
//     the gradients of okage.NewRadialGradientExtend.
//   - The paint MeshGradient is a shading of type 4.
//   - Surface.FillStencil fills through a stencil of 8 bits. A stencil of 0
//     and 255 with a color is an image mask (D69).
//   - Surface.PushStrokeAdjustment writes the entry SA.
//   - Surface.PaintFromShader converts a shader of okage into a paint.
//   - Surface.DrawRaster draws a part of a page with okage, and draws the
//     pixmap as an image (D62 item 10).
//   - A stitching function of more than 256 functions becomes a tree of
//     stitching functions, because MuPDF 1.28 reads no more (D69).
//
// A change to a file of the port for them has a comment that starts with
// okage: and names D68 or D69. Each one keeps the bytes and the hashes of
// every input of krilla, except a stitching function of more than 256
// functions. MuPDF does not read such a function (D69).
package pdf
