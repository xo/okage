// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"github.com/xo/okage/internal/pdfwriter"
)

// refChunk is the Option<(Ref, Chunk)> of the fields of NonStreamChunks in
// krilla src/chunk_container.rs: a chunk with the reference of its root
// object. nil is None.
type refChunk struct {
	ref   pdfwriter.Ref
	chunk *pdfwriter.Chunk
}

// chunkResult is the Deferred<KrillaResult<Chunk>> of the images in krilla
// src/chunk_container.rs. The port makes the chunk at once, so it holds the
// chunk or the error that the chunk gave.
type chunkResult struct {
	chunk *pdfwriter.Chunk
	err   error
}

// chunkContainer is ChunkContainer in krilla src/chunk_container.rs. The
// fields of upstream for the forms, the tags, the embedded files and the
// embedded PDF files are not in the base port (D62). The port of the
// metadata of interchange/metadata.rs adds the field metadata.
//
// Collects all chunks that we create while building the PDF and then writes
// them out in an orderly manner.
type chunkContainer struct {
	streams   streamChunks
	nonStream nonStreamChunks
}

// streamChunks is StreamChunks in krilla src/chunk_container.rs. The deferred
// chunks of the pages of upstream are chunks.
type streamChunks struct {
	fonts            []*pdfwriter.Chunk
	shadingFunctions []*pdfwriter.Chunk
	patterns         []*pdfwriter.Chunk
	pages            []*pdfwriter.Chunk
	iccProfiles      []*pdfwriter.Chunk
	xObjects         []*pdfwriter.Chunk
	images           []chunkResult
}

// nonStreamChunks is NonStreamChunks in krilla src/chunk_container.rs.
type nonStreamChunks struct {
	pageTree      *refChunk
	outline       *refChunk
	pageLabelTree *refChunk
	pageLabels    *pdfwriter.Chunk
	annotations   *pdfwriter.Chunk
	colorSpaces   *pdfwriter.Chunk
	destinations  *pdfwriter.Chunk
	extGStates    *pdfwriter.Chunk
	// resourceDictionaries holds the resource dictionaries of the pages and
	// the streams.
	resourceDictionaries *pdfwriter.Chunk
	masks                *pdfwriter.Chunk
	fonts                *pdfwriter.Chunk
	shadingFunctions     *pdfwriter.Chunk
	patterns             *pdfwriter.Chunk
	pages                *pdfwriter.Chunk
}

// newChunkContainer is ChunkContainer::new in krilla src/chunk_container.rs.
func newChunkContainer(sc *serializeContext) *chunkContainer {
	return &chunkContainer{
		nonStream: nonStreamChunks{
			pageLabels:           sc.newChunk(),
			annotations:          sc.newChunk(),
			colorSpaces:          sc.newChunk(),
			destinations:         sc.newChunk(),
			extGStates:           sc.newChunk(),
			resourceDictionaries: sc.newChunk(),
			masks:                sc.newChunk(),
			fonts:                sc.newChunk(),
			shadingFunctions:     sc.newChunk(),
			patterns:             sc.newChunk(),
			pages:                sc.newChunk(),
		},
	}
}

// finish is ChunkContainer::finish in krilla src/chunk_container.rs. It
// numbers the objects of the chunks in the order of the file, writes the
// chunks, and writes the catalog.
//
// The port leaves out the parts of the base port that later units add: the
// information dictionary and the language of the metadata, the viewer
// preferences, the page layout and the named destinations. It leaves out the
// parts that the base port does not have: the XMP metadata, the forms, the
// output intents, the tags, the embedded files and the associated files
// (D62).
func (c *chunkContainer) finish(sc *serializeContext) (*pdfwriter.Pdf, error) {
	remappedRef := pdfwriter.NewRef(1)
	remapper := map[pdfwriter.Ref]pdfwriter.Ref{}

	// Allows us to estimate the capacity we will need for the new PDF.
	chunksByteLen := 0

	// This traverses the chunks in the order that we will write them to the
	// PDF and assigns new references as we go. This gives us the advantage
	// that the PDF will be numbered with monotonically increasing numbers,
	// which, while it is not a strict requirement for a valid PDF, makes it
	// a lot cleaner and might make implementing features like object streams
	// easier down the road.
	//
	// It also allows us to estimate the capacity we will need for the new
	// PDF.
	err := c.visit(func(chunk *pdfwriter.Chunk) {
		for objectRef := range chunk.Refs() {
			remapper[objectRef] = remappedRef.Bump()
		}
		chunksByteLen += chunk.Len()
	})
	if err != nil {
		return nil, err
	}

	// Chunk length is not an exact number because the length might change
	// as we renumber, so we add a bit of a padding by multiplying with 1.1.
	// The 200 is additional padding for the document catalog. This
	// hopefully allows us to avoid re-allocations in the general case, and
	// thus give us better performance.
	capacity := int(float32(chunksByteLen)*1.1 + 200.0)
	pdf := sc.newPDFWithCapacity(capacity)
	sc.serializeSettings().pdfVersion().setVersion(pdf)

	if sc.serializeSettings().asciiCompatible &&
		!sc.serializeSettings().validators().requiresBinaryHeader() {
		pdf.SetBinaryMarker([4]byte{'A', 'A', 'A', 'A'})
	}

	// Write the chunks in all the fields.
	err = c.visit(func(chunk *pdfwriter.Chunk) {
		chunk.RenumberInto(&pdf.Chunk, func(old pdfwriter.Ref) pdfwriter.Ref {
			return remapper[old]
		})
	})
	if err != nil {
		return nil, err
	}

	// The port of the metadata registers NoDocumentTitle only when the
	// metadata has no title, and writes the information dictionary here.
	sc.registerValidationError(validationError{kind: validationNoDocumentTitle})

	instanceID := stableHashBase64(func(h *hasher) {
		h.bytes(pdf.AsBytes())
	})
	// The document ID of upstream comes from the metadata when it has an ID,
	// or a title and authors. Without metadata, it is the instance ID.
	documentID := instanceID

	pdf.SetFileID([]byte(documentID), []byte(instanceID))

	catalogRef := remappedRef.Bump()
	catalog := pdf.Catalog(catalogRef)
	pageTree := c.nonStream.pageTree
	if pageTree == nil {
		panic("page tree should exist")
	}
	catalog.Pages(remapper[pageTree.ref])

	if pl := c.nonStream.pageLabelTree; pl != nil {
		catalog.Pair(pdfwriter.Name("PageLabels"), remapper[pl.ref])
	}

	// The port of the metadata writes the language of the document here.
	sc.registerValidationError(validationError{kind: validationNoDocumentLanguage})

	if ol := c.nonStream.outline; ol != nil {
		catalog.Outlines(remapper[ol.ref])
	}

	catalog.Finish()

	return pdf, nil
}

// visit is the trait Visit in krilla src/chunk_container.rs, for
// ChunkContainer. It calls f for each chunk, in the order of the file: the
// chunks without streams, then the chunks with streams.
func (c *chunkContainer) visit(f func(*pdfwriter.Chunk)) error {
	c.nonStream.visit(f)
	return c.streams.visit(f)
}

// visit is Visit for StreamChunks in krilla src/chunk_container.rs. The
// error of an image stops the visit, as the KrillaResult of upstream does.
func (s *streamChunks) visit(f func(*pdfwriter.Chunk)) error {
	visitChunks(s.fonts, f)
	visitChunks(s.shadingFunctions, f)
	visitChunks(s.patterns, f)
	visitChunks(s.pages, f)
	visitChunks(s.iccProfiles, f)
	visitChunks(s.xObjects, f)
	for _, image := range s.images {
		if image.err != nil {
			return image.err
		}
		f(image.chunk)
	}
	return nil
}

// visit is Visit for NonStreamChunks in krilla src/chunk_container.rs.
func (n *nonStreamChunks) visit(f func(*pdfwriter.Chunk)) {
	n.pageTree.visit(f)
	n.outline.visit(f)
	n.pageLabelTree.visit(f)
	f(n.pageLabels)
	f(n.annotations)
	f(n.colorSpaces)
	f(n.destinations)
	f(n.extGStates)
	f(n.resourceDictionaries)
	f(n.masks)
	f(n.fonts)
	f(n.shadingFunctions)
	f(n.patterns)
	f(n.pages)
}

// visit is Visit for Option<(Ref, Chunk)> in krilla src/chunk_container.rs.
func (r *refChunk) visit(f func(*pdfwriter.Chunk)) {
	if r != nil {
		f(r.chunk)
	}
}

// visitChunks is Visit for Vec<Chunk> in krilla src/chunk_container.rs.
func visitChunks(chunks []*pdfwriter.Chunk, f func(*pdfwriter.Chunk)) {
	for _, chunk := range chunks {
		f(chunk)
	}
}
