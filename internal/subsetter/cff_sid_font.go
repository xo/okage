// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

// This file ports subsetter src/cff/sid_font.rs.

// sidMetadata is SIDMetadata in subsetter src/cff/sid_font.rs. Metadata
// required for handling SID-keyed fonts. Make the default metadata with
// defaultSIDMetadata.
type sidMetadata struct {
	localSubrs      index
	privateDictData []byte
}

// defaultSIDMetadata is the derived Default of SIDMetadata in subsetter
// src/cff/sid_font.rs.
func defaultSIDMetadata() sidMetadata {
	return sidMetadata{localSubrs: defaultIndex()}
}

// parseSIDMetadata is parse_sid_metadata in subsetter src/cff/sid_font.rs.
// The parsing logic was taken from ttf-parser.
func parseSIDMetadata(data []byte, topDict *topDictData) sidMetadata {
	if !topDict.hasPrivate {
		return defaultSIDMetadata()
	}
	privateDictRange := topDict.private
	metadata := defaultSIDMetadata()
	privateDictData, ok := sliceRange(data, privateDictRange)
	if !ok {
		return defaultSIDMetadata()
	}

	if subrsOffset, ok := parseSubrOffset(privateDictData); ok {
		start := privateDictRange.start + subrsOffset
		subrsData, ok := sliceAt(data, start)
		if !ok {
			return defaultSIDMetadata()
		}
		r := newReader(subrsData)
		metadata.localSubrs, ok = parseIndex(&r)
		if !ok {
			return defaultSIDMetadata()
		}
	} else {
		metadata.localSubrs = defaultIndex()
	}

	metadata.privateDictData = privateDictData
	return metadata
}

// generateFDIndex is generate_fd_index in subsetter src/cff/sid_font.rs. It
// writes the FD INDEX for SID-keyed fonts. They all get mapped to the font
// DICT 0.
func generateFDIndex(gidRemapper *GlyphRemapper, w *writer) {
	// Format
	w.writeU8(3)
	// nRanges
	w.writeU16(1)
	// first
	w.writeU16(0)
	// fd index
	w.writeU8(0)
	// sentinel
	w.writeU16(gidRemapper.NumGids())
}
