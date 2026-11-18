// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"compress/zlib"
	"slices"

	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// Stream is Stream in krilla src/stream.rs. The field xobject_identifier of
// upstream is for the tags, which the base port leaves out (D62).
//
// Drawing to a non-page context.
//
// In most cases, all you need to do is to construct a document, add pages to
// it and draw on the surface of each page. A stream is a "sub-surface" that
// you draw on independently of the main page contents. There are two such
// situations: the contents of a mask, and the contents of a pattern. A
// StreamBuilder of the current surface gives the surface of a stream, and
// StreamBuilder.Finish returns the Stream, with which you then create a
// pattern or a mask.
type Stream struct {
	content []byte
	bbox    Rect
	// validationErrors must reach the serializer. Each object that uses a
	// stream passes them on: a mask, a tiling pattern, a page and an
	// XObject.
	validationErrors   []validationError
	resourceDictionary resourceDictionary
	usesMask           bool
}

// newStream is Stream::new in krilla src/stream.rs.
func newStream(content []byte, bbox Rect, validationErrors []validationError, resourceDictionary resourceDictionary, usesMask bool) *Stream {
	return &Stream{
		content:            content,
		bbox:               bbox,
		validationErrors:   validationErrors,
		resourceDictionary: resourceDictionary,
		usesMask:           usesMask,
	}
}

// isEmpty is Stream::is_empty in krilla src/stream.rs.
func (s *Stream) isEmpty() bool {
	return len(s.content) == 0
}

// emptyStream is Stream::empty in krilla src/stream.rs.
func emptyStream() *Stream {
	bbox, _ := path.RectFromXYWH(0, 0, 0, 0)
	return &Stream{
		bbox:               bbox,
		resourceDictionary: newResourceDictionaryBuilder().finish(),
	}
}

// hash is the derived Hash of Stream in krilla src/stream.rs. A pattern and a
// mask hash their stream, so that the serializer writes a stream once.
func (s *Stream) hash(h *hasher) {
	h.bytes(s.content)
	h.rect(s.bbox)
	h.usize(len(s.validationErrors))
	for _, e := range s.validationErrors {
		e.hash(h)
	}
	s.resourceDictionary.hash(h)
	h.bool(s.usesMask)
	// The Option<XObjectTagIdentifier> of upstream, always None.
	h.usize(0)
}

// StreamBuilder is StreamBuilder in krilla src/stream.rs. The methods
// Surface and SurfaceWithBBox of upstream make a Surface, and the port of
// surface.rs adds them, as the package comment says.
//
// A builder to create streams.
type StreamBuilder struct {
	sc     *serializeContext
	cc     *chunkContainer
	stream *Stream
	// baseTransform is the transform that the stream is assumed to be drawn
	// with, relative to the outermost surface.
	baseTransform Transform
}

// newStreamBuilder is StreamBuilder::new in krilla src/stream.rs.
func newStreamBuilder(sc *serializeContext, cc *chunkContainer, baseTransform Transform) *StreamBuilder {
	return &StreamBuilder{
		sc:            sc,
		cc:            cc,
		stream:        emptyStream(),
		baseTransform: baseTransform,
	}
}

// Finish is StreamBuilder::finish in krilla src/stream.rs.
//
// Turn the stream builder into a stream.
func (b *StreamBuilder) Finish() *Stream {
	return b.stream
}

// streamFilter is StreamFilter in krilla src/stream.rs.
//
// A PDF stream filter.
type streamFilter uint8

// The cases of StreamFilter.
const (
	filterFlate streamFilter = iota
	// filterFlateMemoized is StreamFilter::FlateMemoized. krilla memoizes
	// the compression with comemo when that feature is on. The port
	// compresses each time.
	filterFlateMemoized
	filterASCIIHex
	// filterDCT is StreamFilter::Dct, for the data of a JPEG file, which is
	// already encoded.
	filterDCT
)

// pngPredictor is DecodeParams::PngPredictor in krilla src/stream.rs.
type pngPredictor struct {
	colors           int32
	bitsPerComponent int32
	columns          int32
}

// write is DecodeParams::write in krilla src/stream.rs.
func (p pngPredictor) write(dict *pdfwriter.Dict) {
	dict.Pair(pdfwriter.Name("Predictor"), pdfwriter.Int(15))
	dict.Pair(pdfwriter.Name("Colors"), pdfwriter.Int(p.colors))
	dict.Pair(pdfwriter.Name("BitsPerComponent"), pdfwriter.Int(p.bitsPerComponent))
	dict.Pair(pdfwriter.Name("Columns"), pdfwriter.Int(p.columns))
	dict.Finish()
}

// filter is Filter in krilla src/stream.rs. DecodeParams has one case, so
// decodeParams is a pointer to it, and nil is None.
type filter struct {
	kind         streamFilter
	decodeParams *pngPredictor
}

// toName is StreamFilter::to_name in krilla src/stream.rs.
func (f streamFilter) toName() pdfwriter.Name {
	switch f {
	case filterASCIIHex:
		return pdfwriter.Name("ASCIIHexDecode")
	case filterDCT:
		return pdfwriter.Name("DCTDecode")
	default:
		return pdfwriter.Name("FlateDecode")
	}
}

// isBinary is StreamFilter::is_binary in krilla src/stream.rs.
func (f streamFilter) isBinary() bool {
	return f != filterASCIIHex
}

// apply is StreamFilter::apply in krilla src/stream.rs. krilla panics for DCT,
// which it never applies.
func (f streamFilter) apply(content []byte) []byte {
	switch f {
	case filterFlate, filterFlateMemoized:
		return deflateEncode(content)
	case filterASCIIHex:
		return hexEncode(content)
	default:
		panic("can't apply dct decode")
	}
}

// filterStreamBuilder is FilterStreamBuilder in krilla src/stream.rs. The
// enum StreamFilters of upstream is the slice filters: None, Single and
// Multiple are a slice of 0, 1 and more filters, in the order in which they
// were added.
type filterStreamBuilder struct {
	content  []byte
	filters  []filter
	isBinary bool
}

// filtersAreBinary is StreamFilters::is_binary in krilla src/stream.rs.
func filtersAreBinary(filters []filter) bool {
	if len(filters) == 0 {
		return false
	}
	return filters[len(filters)-1].kind.isBinary()
}

// emptyFilterStream is FilterStreamBuilder::empty in krilla src/stream.rs.
func emptyFilterStream(content []byte) *filterStreamBuilder {
	return &filterStreamBuilder{content: content}
}

// binaryFilterStream is FilterStreamBuilder::binary in krilla src/stream.rs.
func binaryFilterStream(content []byte) *filterStreamBuilder {
	return &filterStreamBuilder{content: content, isBinary: true}
}

// newFilterStreamFromContentStream is
// FilterStreamBuilder::new_from_content_stream in krilla src/stream.rs.
func newFilterStreamFromContentStream(content []byte, settings *serializeSettings) *filterStreamBuilder {
	fs := emptyFilterStream(content)
	if settings.compressContentStreams {
		fs.addFilter(filterFlateMemoized)
	}
	return fs
}

// newFilterStreamFromDeflated is FilterStreamBuilder::new_from_deflated in
// krilla src/stream.rs.
func newFilterStreamFromDeflated(content []byte) *filterStreamBuilder {
	fs := emptyFilterStream(content)
	fs.addUnappliedFilter(filterFlate)
	return fs
}

// newFilterStreamFromPNGData is FilterStreamBuilder::new_from_png_data in
// krilla src/stream.rs.
func newFilterStreamFromPNGData(content []byte, colors, bitsPerComponent, columns int32) *filterStreamBuilder {
	fs := emptyFilterStream(content)
	fs.filters = append(fs.filters, filter{
		kind: filterFlate,
		decodeParams: &pngPredictor{
			colors:           colors,
			bitsPerComponent: bitsPerComponent,
			columns:          columns,
		},
	})
	return fs
}

// newFilterStreamFromBinaryData is FilterStreamBuilder::new_from_binary_data
// in krilla src/stream.rs.
func newFilterStreamFromBinaryData(content []byte) *filterStreamBuilder {
	fs := binaryFilterStream(content)
	fs.addFilter(filterFlate)
	return fs
}

// newFilterStreamAutoCompressed is FilterStreamBuilder::new_auto_compressed
// in krilla src/stream.rs. It keeps the compressed data only when it is at
// most 75 percent of the size of the data.
func newFilterStreamAutoCompressed(content []byte) *filterStreamBuilder {
	fs := newFilterStreamFromBinaryData(content)
	const maxCompressedSize = 75
	if len(content) == 0 || 100*len(fs.content)/len(content) > maxCompressedSize {
		return binaryFilterStream(content)
	}
	return fs
}

// newFilterStreamFromUncompressed is
// FilterStreamBuilder::new_from_uncompressed in krilla src/stream.rs.
func newFilterStreamFromUncompressed(content []byte) *filterStreamBuilder {
	return binaryFilterStream(content)
}

// newFilterStreamFromJPEGData is FilterStreamBuilder::new_from_jpeg_data in
// krilla src/stream.rs.
func newFilterStreamFromJPEGData(content []byte) *filterStreamBuilder {
	fs := emptyFilterStream(content)
	// JPEG data already is DCT encoded.
	fs.addUnappliedFilter(filterDCT)
	return fs
}

// finish is FilterStreamBuilder::finish in krilla src/stream.rs. When the
// file must be ASCII, it encodes binary data in hex.
func (b *filterStreamBuilder) finish(settings *serializeSettings) filterStream {
	if settings.asciiCompatible && len(b.content) != 0 &&
		(filtersAreBinary(b.filters) || (b.isBinary && !isASCII(b.content))) {
		b.addFilter(filterASCIIHex)
	}
	return filterStream{content: b.content, filters: b.filters}
}

// addFilter is FilterStreamBuilder::add_filter in krilla src/stream.rs.
func (b *filterStreamBuilder) addFilter(f streamFilter) {
	b.content = f.apply(b.content)
	b.filters = append(b.filters, filter{kind: f})
}

// addUnappliedFilter is FilterStreamBuilder::add_unapplied_filter in krilla
// src/stream.rs.
func (b *filterStreamBuilder) addUnappliedFilter(f streamFilter) {
	b.filters = append(b.filters, filter{kind: f})
}

// isASCII reports whether each byte is ASCII, as the method is_ascii of a
// byte of Rust.
func isASCII(content []byte) bool {
	for _, c := range content {
		if c >= 0x80 {
			return false
		}
	}
	return true
}

// filterStream is FilterStream in krilla src/stream.rs.
type filterStream struct {
	content []byte
	filters []filter
}

// encodedData is FilterStream::encoded_data in krilla src/stream.rs.
func (s filterStream) encodedData() []byte {
	return s.content
}

// writeFilters is FilterStream::write_filters in krilla src/stream.rs. It
// writes the filters in the reverse order of their application, as a reader
// applies the first filter of the array first.
func (s filterStream) writeFilters(dict *pdfwriter.Dict) {
	switch len(s.filters) {
	case 0:
	case 1:
		f := s.filters[0]
		dict.Pair(pdfwriter.Name("Filter"), f.kind.toName())
		if f.decodeParams != nil {
			f.decodeParams.write(dict.Insert(pdfwriter.Name("DecodeParms")).Dict())
		}
	default:
		names := dict.Insert(pdfwriter.Name("Filter")).Array()
		for _, f := range slices.Backward(s.filters) {
			names.Item(f.kind.toName())
		}
		names.Finish()
		hasParams := false
		for _, f := range s.filters {
			if f.decodeParams != nil {
				hasParams = true
			}
		}
		if hasParams {
			params := dict.Insert(pdfwriter.Name("DecodeParms")).Array()
			for _, f := range slices.Backward(s.filters) {
				if p := f.decodeParams; p != nil {
					p.write(params.Push().Dict())
				} else {
					params.Push().Primitive(pdfwriter.Null{})
				}
			}
			params.Finish()
		}
	}
}

// deflateEncode is deflate_encode in krilla src/stream.rs. It compresses with
// zlib at the level 6, as krilla does with flate2. The bytes differ from the
// bytes of flate2, so the tests compare the decoded data (D62).
func deflateEncode(data []byte) []byte {
	var buf bytes.Buffer
	e := deflateEncoder(&buf)
	// A write to a bytes.Buffer cannot fail.
	_, _ = e.Write(data)
	_ = e.Close()
	return buf.Bytes()
}

// deflateEncoder is deflate_encoder in krilla src/stream.rs.
func deflateEncoder(buf *bytes.Buffer) *zlib.Writer {
	const compressionLevel = 6
	// The level is valid, so NewWriterLevel returns no error.
	e, _ := zlib.NewWriterLevel(buf, compressionLevel)
	return e
}

// hexEncode is hex_encode in krilla src/stream.rs. It writes two capital hex
// digits for each byte, and a new line after each 35 bytes.
func hexEncode(data []byte) []byte {
	const columns = 35
	const digits = "0123456789ABCDEF"
	output := make([]byte, 0, 2*len(data)+len(data)/columns)
	for index, b := range data {
		output = append(output, digits[b>>4], digits[b&0x0f])
		if index%columns == columns-1 {
			output = append(output, '\n')
		}
	}
	return output
}
