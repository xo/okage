// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import "github.com/xo/okage/internal/pdfwriter"

// tilingPattern is TilingPattern in krilla src/graphics/tiling_pattern.rs.
type tilingPattern struct {
	stream      *Stream
	transform   Transform
	baseOpacity NormalizedF32
	width       float32
	height      float32
}

// newTilingPattern is TilingPattern::new in krilla
// src/graphics/tiling_pattern.rs.
func newTilingPattern(stream *Stream, transform Transform, baseOpacity NormalizedF32, width, height float32, sc *serializeContext, cc *chunkContainer) tilingPattern {
	// stroke/fill opacity doesn't work consistently across different viewers
	// for patterns, so instead we simulate it ourselves.
	patternStream := stream
	if baseOpacity != NormalizedF32One {
		patternStream = opacifiedPatternStream(sc, cc, baseOpacity, stream)
	}

	return tilingPattern{
		stream:      patternStream,
		transform:   transform,
		baseOpacity: baseOpacity,
		width:       width,
		height:      height,
	}
}

// typeID names the type for sipHash.
func (tilingPattern) typeID() string { return "tilingPattern" }

// hash is the impl Hash for TilingPattern in krilla
// src/graphics/tiling_pattern.rs.
func (p tilingPattern) hash(h *hasher) {
	p.stream.hash(h)
	h.transform(p.transform)
	hashF32Bits(h, p.baseOpacity.Get())
	hashF32Bits(h, p.width)
	hashF32Bits(h, p.height)
}

// serialize is Cacheable::serialize for TilingPattern in krilla
// src/graphics/tiling_pattern.rs. One tile has the size of the pattern, and
// the tiles have no space between them.
func (p tilingPattern) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	chunk := sc.newChunk()

	for _, validationError := range p.stream.validationErrors {
		sc.registerValidationError(validationError)
	}

	// okage: the resources come first, so that Document.Finish can write
	// the stream again when the subset of one of its fonts fails. They are
	// in another chunk, so the bytes do not change. write keeps no pointer
	// to the stream, whose content the garbage collector can free (D80).
	resources := p.stream.resourceDictionary.writeResources(sc, cc.nonStream.resourceDictionaries)
	width, height, transform := p.width, p.height, p.transform
	write := func(chunk *pdfwriter.Chunk, content []byte) {
		patternStream := newFilterStreamFromContentStream(content, sc.serializeSettings()).
			finish(sc.serializeSettings())
		tp := chunk.TilingPattern(rootRef, patternStream.encodedData())
		patternStream.writeFilters(tp.Dict)

		resources.write(tp)

		finalBBox := pdfwriter.NewRect(0.0, 0.0, width, height)

		tp.TilingType(pdfwriter.TilingTypeConstantSpacing).
			PaintType(pdfwriter.PaintTypeColored).
			BBox(finalBBox).
			Matrix(transformToPDF(transform)).
			XStep(finalBBox.X2 - finalBBox.X1).
			YStep(finalBBox.Y2 - finalBBox.Y1)

		tp.Finish()
	}
	write(chunk, p.stream.content)
	sc.registerTextStream(chunk, &p.stream.resourceDictionary, write)
	cc.streams.patterns = append(cc.streams.patterns, chunk)
}

// newResource is Resourceable for TilingPattern in krilla
// src/graphics/tiling_pattern.rs.
func (tilingPattern) newResource(ref pdfwriter.Ref) patternResource {
	return patternResource{ref}
}
