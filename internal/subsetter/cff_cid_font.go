// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

// This file ports subsetter src/cff/cid_font.rs. The parsing logic was taken
// from ttf-parser.

// parseCIDMetadata is parse_cid_metadata in subsetter src/cff/cid_font.rs.
// It parses CID metadata from a font.
func parseCIDMetadata(data []byte, topDict *topDictData, numberOfGlyphs uint16) (cidMetadata, bool) {
	if !topDict.hasFDArray || !topDict.hasFDSelect {
		return cidMetadata{}, false
	}

	r := newReaderAtOffset(data, topDict.fdArray)
	fdArray, ok := parseIndex(&r)
	if !ok {
		return cidMetadata{}, false
	}
	s := newReaderAtOffset(data, topDict.fdSelect)
	fdSelect, ok := parseFDSelect(numberOfGlyphs, &s)
	if !ok {
		return cidMetadata{}, false
	}
	metadata := cidMetadata{fdArray: fdArray, fdSelect: fdSelect}

	for fontDictData := range metadata.fdArray.all() {
		fd, ok := parseFontDict(data, fontDictData)
		if !ok {
			return cidMetadata{}, false
		}
		metadata.fontDicts = append(metadata.fontDicts, fd)
	}

	return metadata, true
}

// parseFDSelect is parse_fd_select in subsetter src/cff/cid_font.rs.
func parseFDSelect(numberOfGlyphs uint16, r *reader) (fdSelect, bool) {
	format, ok := r.readU8()
	if !ok {
		return fdSelect{}, false
	}
	switch format {
	case 0:
		array, ok := readArray16(r, numberOfGlyphs, 1, (*reader).readU8)
		if !ok {
			return fdSelect{}, false
		}
		return fdSelect{format0: array}, true
	case 3:
		tail, ok := r.tail()
		if !ok {
			return fdSelect{}, false
		}
		return fdSelect{format3: tail, isFormat3: true}, true
	}
	return fdSelect{}, false
}

// cidMetadata is CIDMetadata in subsetter src/cff/cid_font.rs. Metadata
// necessary for processing CID-keyed fonts.
type cidMetadata struct {
	fontDicts []fontDict
	fdArray   index
	fdSelect  fdSelect
}

// fdSelect is FDSelect in subsetter src/cff/cid_font.rs. It is
// FDSelect::Format3 with the data format3 when isFormat3 is true, and
// FDSelect::Format0 with the array format0 when it is false. The zero value
// is the derived Default of upstream, Format0 with an empty array.
type fdSelect struct {
	format0   lazyArray16[uint8]
	format3   []byte
	isFormat3 bool
}

// fontDictIndex is FDSelect::font_dict_index in subsetter
// src/cff/cid_font.rs. It returns the font dict index for a glyph.
func (s fdSelect) fontDictIndex(glyphID uint16) (uint8, bool) {
	if !s.isFormat3 {
		return s.format0.get(glyphID)
	}
	r := newReader(s.format3)
	numberOfRanges, ok := r.readU16()
	if !ok || numberOfRanges == 0 {
		return 0, false
	}

	if numberOfRanges == 0xFFFF {
		return 0, false
	}
	numberOfRanges++

	prevFirstGlyph, ok := r.readU16()
	if !ok {
		return 0, false
	}
	prevIndex, ok := r.readU8()
	if !ok {
		return 0, false
	}
	for range numberOfRanges - 1 {
		currFirstGlyph, ok := r.readU16()
		if !ok {
			return 0, false
		}
		if glyphID >= prevFirstGlyph && glyphID < currFirstGlyph {
			return prevIndex, true
		}
		if prevIndex, ok = r.readU8(); !ok {
			return 0, false
		}

		prevFirstGlyph = currFirstGlyph
	}

	return 0, false
}

// rewriteFDIndex is rewrite_fd_index in subsetter src/cff/cid_font.rs. It
// rewrites the FD INDEX for CID-keyed font.
func rewriteFDIndex(gidRemapper *GlyphRemapper, fdSelect fdSelect, fdRemapper *fontDictRemapper, w *writer) error {
	// We always use format 0, since it's the simplest.
	w.writeU8(0)

	for gid := range gidRemapper.RemappedGids() {
		oldFD, ok := fdSelect.fontDictIndex(gid)
		if !ok {
			return errMalformedFont
		}
		newFD, ok := fdRemapper.get(oldFD)
		if !ok {
			return errSubset
		}
		w.writeU8(newFD)
	}

	return nil
}
