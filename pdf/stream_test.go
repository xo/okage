// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"compress/zlib"
	"io"
	"strings"
	"testing"

	"github.com/xo/okage/internal/pdfwriter"
)

// writeStream writes a filter stream as the object 1 of a chunk without
// pretty printing, and returns the bytes of the chunk.
func writeStream(fs filterStream) string {
	chunk := pdfwriter.NewChunkWithSettings(pdfwriter.Settings{Pretty: false})
	s := chunk.Stream(pdfwriter.NewRef(1), fs.encodedData())
	fs.writeFilters(s.Dict)
	s.Finish()
	return string(chunk.AsBytes())
}

// inflate decodes the data of a FlateDecode filter.
func inflate(t *testing.T, data []byte) []byte {
	t.Helper()
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestHexEncode checks hex_encode of krilla src/stream.rs: two capital
// digits for each byte, and a new line after each 35 bytes.
func TestHexEncode(t *testing.T) {
	if got := string(hexEncode([]byte{0x00, 0xab, 0x7f})); got != "00AB7F" {
		t.Errorf("got %q", got)
	}
	data := bytes.Repeat([]byte{0xff}, 71)
	want := strings.Repeat("FF", 35) + "\n" + strings.Repeat("FF", 35) + "\n" + "FF"
	if got := string(hexEncode(data)); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := hexEncode(nil); len(got) != 0 {
		t.Errorf("no data gives %q", got)
	}
}

// TestFilterStream checks the filters of FilterStreamBuilder in krilla
// src/stream.rs, with each setting that changes them.
func TestFilterStream(t *testing.T) {
	content := []byte("0 0 m 10 10 l S")
	compressed := &serializeSettings{compressContentStreams: true}
	ascii := &serializeSettings{compressContentStreams: true, asciiCompatible: true}
	plain := &serializeSettings{}

	fs := newFilterStreamFromContentStream(content, compressed).finish(compressed)
	if got := inflate(t, fs.encodedData()); !bytes.Equal(got, content) {
		t.Errorf("the compressed content is %q", got)
	}
	if out := writeStream(fs); !strings.Contains(out, "/Filter/FlateDecode>>") {
		t.Errorf("the stream is %q", out)
	}

	fs = newFilterStreamFromContentStream(content, ascii).finish(ascii)
	out := writeStream(fs)
	if !strings.Contains(out, "/Filter[/ASCIIHexDecode/FlateDecode]>>") {
		t.Errorf("the ASCII stream is %q", out)
	}
	for _, c := range fs.encodedData() {
		if c >= 0x80 {
			t.Fatalf("the ASCII stream holds the byte %#x", c)
		}
	}

	fs = newFilterStreamFromContentStream(content, plain).finish(plain)
	if !bytes.Equal(fs.encodedData(), content) || strings.Contains(writeStream(fs), "/Filter") {
		t.Errorf("an uncompressed stream is %q", writeStream(fs))
	}

	// An empty stream gets no hex filter.
	fs = newFilterStreamFromContentStream(nil, &serializeSettings{asciiCompatible: true}).finish(&serializeSettings{asciiCompatible: true})
	if len(fs.filters) != 0 {
		t.Errorf("an empty stream has the filters %v", fs.filters)
	}

	// Binary data that is not compressed gets the hex filter only when it
	// holds a byte that is not ASCII.
	for _, c := range []struct {
		data    []byte
		filters int
	}{
		{[]byte("ascii"), 0},
		{[]byte{0x80, 1}, 1},
	} {
		fs := newFilterStreamFromUncompressed(c.data).finish(&serializeSettings{asciiCompatible: true})
		if len(fs.filters) != c.filters {
			t.Errorf("the data %q has %d filters, want %d", c.data, len(fs.filters), c.filters)
		}
	}

	fs = newFilterStreamFromDeflated([]byte{1, 2}).finish(plain)
	if !bytes.Equal(fs.encodedData(), []byte{1, 2}) || !strings.Contains(writeStream(fs), "/Filter/FlateDecode>>") {
		t.Errorf("deflated data gives %q", writeStream(fs))
	}

	fs = newFilterStreamFromJPEGData([]byte{0xff, 0xd8}).finish(ascii)
	if out := writeStream(fs); !strings.Contains(out, "/Filter[/ASCIIHexDecode/DCTDecode]>>") {
		t.Errorf("JPEG data in an ASCII file gives %q", out)
	}

	fs = newFilterStreamFromPNGData([]byte{1}, 3, 8, 4).finish(plain)
	if out := writeStream(fs); !strings.Contains(out, "/Filter/FlateDecode/DecodeParms<</Predictor 15/Colors 3/BitsPerComponent 8/Columns 4>>>>") {
		t.Errorf("PNG data gives %q", out)
	}
	fs = newFilterStreamFromPNGData([]byte{1}, 1, 16, 2).finish(ascii)
	if out := writeStream(fs); !strings.Contains(out, "/Filter[/ASCIIHexDecode/FlateDecode]/DecodeParms[null<</Predictor 15/Colors 1/BitsPerComponent 16/Columns 2>>]>>") {
		t.Errorf("PNG data in an ASCII file gives %q", out)
	}
}

// TestAutoCompressed checks FilterStreamBuilder::new_auto_compressed of
// krilla src/stream.rs, which keeps the compressed data only when it is at
// most 75 percent of the size of the data.
func TestAutoCompressed(t *testing.T) {
	repeated := bytes.Repeat([]byte("abcd"), 100)
	fs := newFilterStreamAutoCompressed(repeated)
	if len(fs.filters) != 1 || !bytes.Equal(inflate(t, fs.content), repeated) {
		t.Errorf("repeated data has the filters %v", fs.filters)
	}
	for _, data := range [][]byte{nil, []byte("x")} {
		fs := newFilterStreamAutoCompressed(data)
		if len(fs.filters) != 0 || !bytes.Equal(fs.content, data) || !fs.isBinary {
			t.Errorf("the data %q gives %+v", data, fs)
		}
	}
	fs = newFilterStreamFromBinaryData([]byte("x"))
	if len(fs.filters) != 1 || !fs.isBinary {
		t.Errorf("binary data gives %+v", fs)
	}
}

// TestDCTPanics checks that the filter DCT is never applied, as upstream
// panics.
func TestDCTPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("applying DCT does not panic")
		}
	}()
	filterDCT.apply([]byte{1})
}

// TestStream checks the empty stream and the hash of a stream.
func TestStream(t *testing.T) {
	s := emptyStream()
	if !s.isEmpty() || s.bbox.Width() != 0 || s.bbox.Height() != 0 {
		t.Errorf("the empty stream is %+v", s)
	}
	b := newStreamBuilder(nil, nil, Transform{})
	if got := b.Finish(); !got.isEmpty() {
		t.Error("a new stream builder gives a stream that is not empty")
	}
	rd := newResourceDictionaryBuilder()
	rd.registerResource(fontResource{pdfwriter.NewRef(3)})
	other := newStream([]byte("q Q"), s.bbox, []validationError{{kind: validationTransparency}}, rd.finish(), true)
	if other.isEmpty() {
		t.Error("a stream with content is empty")
	}
	hash := func(s *Stream) [16]byte {
		h := newHasher()
		s.hash(h)
		return h.finish128().AsBytes()
	}
	if hash(s) == hash(other) || hash(s) != hash(emptyStream()) {
		t.Error("the hash of a stream does not follow its fields")
	}
	copied := *other
	copied.usesMask = false
	if hash(other) == hash(&copied) {
		t.Error("the hash of a stream does not hold usesMask")
	}
}
