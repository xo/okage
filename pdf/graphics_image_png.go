// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// This file ports krilla src/graphics/image/png.rs, which reads the chunks
// of a PNG file. krilla copies the data of the chunks IDAT of some PNG files
// into the PDF file without a decode, and the filter of PDF undoes the
// predictors of PNG. The types of the crate png that upstream takes, ChunkType,
// BitDepth and ColorType, are in this file, with the functions of the crate
// that the file calls.

// pngColorType is png::ColorType: the color type of the header of a PNG
// file.
type pngColorType uint8

// The values of pngColorType, as the header of a PNG file holds them.
const (
	pngGrayscale      pngColorType = 0
	pngRGB            pngColorType = 2
	pngIndexed        pngColorType = 3
	pngGrayscaleAlpha pngColorType = 4
	pngRGBA           pngColorType = 6
)

// pngColorTypeFromU8 is ColorType::from_u8 of the crate png.
func pngColorTypeFromU8(v uint8) (pngColorType, bool) {
	switch c := pngColorType(v); c {
	case pngGrayscale, pngRGB, pngIndexed, pngGrayscaleAlpha, pngRGBA:
		return c, true
	}
	return 0, false
}

// samples is ColorType::samples of the crate png: the number of samples of a
// pixel.
func (c pngColorType) samples() int32 {
	switch c {
	case pngRGB:
		return 3
	case pngGrayscaleAlpha:
		return 2
	case pngRGBA:
		return 4
	default:
		return 1
	}
}

// pngBitDepthFromU8 is BitDepth::from_u8 of the crate png. A bit depth is
// the number of bits of a sample, 1, 2, 4, 8 or 16.
func pngBitDepthFromU8(v uint8) (uint8, bool) {
	switch v {
	case 1, 2, 4, 8, 16:
		return v, true
	}
	return 0, false
}

// pngChunkType is chunk::ChunkType of the crate png: the four bytes of the
// type of a chunk.
type pngChunkType [4]byte

// The types of the chunks that png.rs reads, from chunk of the crate png.
var (
	pngChunkIHDR = pngChunkType{'I', 'H', 'D', 'R'}
	pngChunkPLTE = pngChunkType{'P', 'L', 'T', 'E'}
	pngChunkIDAT = pngChunkType{'I', 'D', 'A', 'T'}
	pngChunkIEND = pngChunkType{'I', 'E', 'N', 'D'}
	pngChunkTRNS = pngChunkType{'t', 'R', 'N', 'S'}
	pngChunkICCP = pngChunkType{'i', 'C', 'C', 'P'}
)

// isCritical is chunk::is_critical of the crate png. A chunk is critical when
// the first letter of its type is a capital letter.
func (t pngChunkType) isCritical() bool {
	return t[0]&32 == 0
}

// pngData is PngData in krilla src/graphics/image/png.rs: the data of the
// chunks IDAT of a PNG file, with what the PDF file needs to read it.
type pngData struct {
	idat       []byte
	bitDepth   uint8
	colorType  pngColorType
	hasPalette bool
	palette    []byte
}

// newPNGData is PngData::new in krilla src/graphics/image/png.rs. It returns
// false when the PNG file cannot go into the PDF file without a decode: an
// image that is interlaced, that has an alpha channel or a chunk tRNS, a
// critical chunk that is not known, or a CRC of a critical chunk that is not
// valid.
func newPNGData(data []byte) (pngData, bool) {
	chunks, err := newPNGChunks(data)
	if err != nil {
		return pngData{}, false
	}
	ihdr, ok, err := chunks.next()
	if !ok || err != nil {
		return pngData{}, false
	}
	if ihdr.kind != pngChunkIHDR || !ihdr.hasValidCRC() {
		return pngData{}, false
	}

	header, ok := newPNGHeader(ihdr.data)
	if !ok {
		return pngData{}, false
	}

	if header.interlaced ||
		(header.colorType != pngGrayscale && header.colorType != pngRGB && header.colorType != pngIndexed) {
		return pngData{}, false
	}

	var idat []byte
	hasIDAT := false
	var palette []byte
	hasPalette := false
	paletteSeen := false
	reachedIEND := false

	for {
		pngChunk, ok, err := chunks.next()
		if !ok {
			break
		}
		if err != nil {
			return pngData{}, false
		}

		if pngChunk.kind.isCritical() && !pngChunk.hasValidCRC() {
			return pngData{}, false
		}

		switch {
		case pngChunk.kind == pngChunkIHDR || pngChunk.kind == pngChunkTRNS:
			return pngData{}, false
		case pngChunk.kind == pngChunkPLTE:
			var maxLen int
			switch header.colorType {
			case pngIndexed:
				maxLen = 3 * (1 << header.bitDepth)
			case pngRGB:
				maxLen = 3 * 256
			default:
				return pngData{}, false
			}

			if paletteSeen ||
				hasIDAT ||
				len(pngChunk.data) == 0 ||
				len(pngChunk.data)%3 != 0 ||
				len(pngChunk.data) > maxLen {
				return pngData{}, false
			}

			paletteSeen = true

			// See 4.1.2: https://www.libpng.org/pub/png/spec/1.2/PNG-Chunks.html
			// For RGB images, we can safely ignore it.
			if header.colorType == pngIndexed {
				palette = append([]byte(nil), pngChunk.data...)
				hasPalette = true
			}
		case pngChunk.kind == pngChunkIDAT:
			hasIDAT = true
			idat = append(idat, pngChunk.data...)
		case pngChunk.kind == pngChunkIEND:
			reachedIEND = true
		case pngChunk.kind.isCritical():
			return pngData{}, false
		}
		if reachedIEND {
			break
		}
	}

	if header.colorType == pngIndexed && !hasPalette {
		return pngData{}, false
	}

	if !reachedIEND || !hasIDAT {
		return pngData{}, false
	}
	return pngData{
		// Note: For performance reasons we don't inflate and validate whether
		// the actual length of the data is correct. This does mean that
		// invalid data might be embedded in case the original PNG was
		// corrupted.
		idat:       idat,
		bitDepth:   header.bitDepth,
		colorType:  header.colorType,
		hasPalette: hasPalette,
		palette:    palette,
	}, true
}

// pngHeader is Header in krilla src/graphics/image/png.rs, the fields of the
// chunk IHDR that png.rs reads.
type pngHeader struct {
	bitDepth   uint8
	colorType  pngColorType
	interlaced bool
}

// newPNGHeader is Header::new in krilla src/graphics/image/png.rs. Like
// upstream, it does not check that the bit depth fits the color type.
func newPNGHeader(data []byte) (pngHeader, bool) {
	if len(data) != 13 {
		return pngHeader{}, false
	}
	width := binary.BigEndian.Uint32(data[0:4])
	height := binary.BigEndian.Uint32(data[4:8])
	bitDepth, ok := pngBitDepthFromU8(data[8])
	if !ok {
		return pngHeader{}, false
	}
	colorType, ok := pngColorTypeFromU8(data[9])
	if !ok {
		return pngHeader{}, false
	}

	if width == 0 || height == 0 || data[10] != 0 || data[11] != 0 {
		return pngHeader{}, false
	}

	var interlaced bool
	switch data[12] {
	case 0:
	case 1:
		interlaced = true
	default:
		return pngHeader{}, false
	}

	return pngHeader{
		bitDepth:   bitDepth,
		colorType:  colorType,
		interlaced: interlaced,
	}, true
}

// pngMagic is PNG_MAGIC in krilla src/graphics/image/png.rs, the signature
// of a PNG file.
const pngMagic = "\x89\x50\x4E\x47\x0D\x0A\x1A\x0A"

// pngChunk is PngChunk in krilla src/graphics/image/png.rs.
type pngChunk struct {
	kind pngChunkType
	data []byte
	crc  uint32
}

// hasValidCRC is PngChunk::has_valid_crc in krilla
// src/graphics/image/png.rs. The CRC covers the type and the data of the
// chunk.
func (c pngChunk) hasValidCRC() bool {
	crc := crc32.Update(0, crc32.IEEETable, c.kind[:])
	crc = crc32.Update(crc, crc32.IEEETable, c.data)
	return crc == c.crc
}

// pngChunks is Chunks in krilla src/graphics/image/png.rs, an iterator of
// the chunks of a PNG file.
type pngChunks struct {
	reader pngReader
	failed bool
}

// errPNGChunkTooShort is the error of a chunk that the data cuts off.
var errPNGChunkTooShort = errors.New("chunk is too short")

// newPNGChunks is Chunks::new in krilla src/graphics/image/png.rs. It reads
// the signature of the file.
func newPNGChunks(data []byte) (*pngChunks, error) {
	reader := pngReader{data: data}

	if b, ok := reader.read(len(pngMagic)); !ok || string(b) != pngMagic {
		return nil, errors.New("invalid PNG signature")
	}

	return &pngChunks{reader: reader}, nil
}

// readChunk is Chunks::read_chunk in krilla src/graphics/image/png.rs.
func (c *pngChunks) readChunk() (pngChunk, error) {
	chunkLen, ok := c.reader.readU32()
	if !ok {
		return pngChunk{}, errPNGChunkTooShort
	}
	kind, ok := c.reader.readU32()
	if !ok {
		return pngChunk{}, errPNGChunkTooShort
	}
	data, ok := c.reader.read(int(chunkLen))
	if !ok {
		return pngChunk{}, errPNGChunkTooShort
	}
	crc, ok := c.reader.readU32()
	if !ok {
		return pngChunk{}, errPNGChunkTooShort
	}

	var k pngChunkType
	binary.BigEndian.PutUint32(k[:], kind)
	return pngChunk{kind: k, data: data, crc: crc}, nil
}

// next is Iterator::next for Chunks in krilla src/graphics/image/png.rs. It
// returns false at the end of the data, or after an error.
func (c *pngChunks) next() (pngChunk, bool, error) {
	if c.failed || len(c.reader.data) == 0 {
		return pngChunk{}, false, nil
	}

	chunk, err := c.readChunk()
	c.failed = err != nil
	return chunk, true, err
}

// pngReader is Reader in krilla src/graphics/image/png.rs.
type pngReader struct {
	data []byte
}

// read is Reader::read in krilla src/graphics/image/png.rs. A length that
// does not fit an int, which a chunk on a machine of 32 bits can give, is
// longer than the data.
func (r *pngReader) read(n int) ([]byte, bool) {
	if n < 0 || n > len(r.data) {
		return nil, false
	}
	b := r.data[:n]
	r.data = r.data[n:]
	return b, true
}

// readU32 is Reader::read_u32 in krilla src/graphics/image/png.rs.
func (r *pngReader) readU32() (uint32, bool) {
	b, ok := r.read(4)
	if !ok {
		return 0, false
	}
	return binary.BigEndian.Uint32(b), true
}
