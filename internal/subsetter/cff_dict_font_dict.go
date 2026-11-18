// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

// This file ports subsetter src/cff/dict/font_dict.rs. The parsing logic was
// adapted from ttf-parser.

// fontDict is FontDict in subsetter src/cff/dict/font_dict.rs. A font DICT.
// Make the default font DICT with defaultFontDict.
type fontDict struct {
	// localSubrs holds the local subroutines that are linked in the font
	// DICT.
	localSubrs index
	// privateDict is the underlying data of the private dict.
	privateDict []byte
	// fontName is the StringID of the font name in this font DICT. It is None
	// when hasFontName is false.
	fontName    stringID
	hasFontName bool
	// fontMatrix is the font matrix. It is None when hasFontMatrix is false.
	fontMatrix    [6]number
	hasFontMatrix bool
}

// defaultFontDict is the derived Default of FontDict in subsetter
// src/cff/dict/font_dict.rs.
func defaultFontDict() fontDict {
	return fontDict{localSubrs: defaultIndex()}
}

// parseFontDict is parse_font_dict in subsetter src/cff/dict/font_dict.rs.
func parseFontDict(fontData, fontDictData []byte) (fontDict, bool) {
	fd := defaultFontDict()

	dictParser := newDictionaryParser(fontDictData)
	for {
		operator, ok := dictParser.parseNext()
		if !ok {
			break
		}
		switch operator {
		case opPrivate:
			privateDictRange, ok := dictParser.parseRange()
			if !ok {
				return fontDict{}, false
			}
			privateDictData, ok := sliceRange(fontData, privateDictRange)
			if !ok {
				return fontDict{}, false
			}
			fd.privateDict = privateDictData
			if subrsOffset, ok := parseSubrOffset(privateDictData); ok {
				start := privateDictRange.start + subrsOffset
				subrsData, ok := sliceAt(fontData, start)
				if !ok {
					return fontDict{}, false
				}
				r := newReader(subrsData)
				fd.localSubrs, ok = parseIndex(&r)
				if !ok {
					return fontDict{}, false
				}
			} else {
				fd.localSubrs = defaultIndex()
			}
		case opFontName:
			fd.fontName, ok = dictParser.parseSID()
			if !ok {
				return fontDict{}, false
			}
			fd.hasFontName = true
		case opFontMatrix:
			fd.fontMatrix, ok = dictParser.parseFontMatrix()
			if !ok {
				return fontDict{}, false
			}
			fd.hasFontMatrix = true
		}
	}

	return fd, true
}

// rewriteFontDictIndex is rewrite_font_dict_index in subsetter
// src/cff/dict/font_dict.rs. It rewrites the font DICT INDEX for CID-keyed
// fonts.
func rewriteFontDictIndex(fdRemapper *fontDictRemapper, sidRemapper *sidRemapper, offsets *offsetsCFF, metadata *cidMetadata, w *writer, topDictIsMissingFontMatrix bool) error {
	var dicts [][]byte

	newDF := 0
	for oldDF := range fdRemapper.sortedIter() {
		if int(oldDF) >= len(metadata.fontDicts) {
			return errSubset
		}
		dict := &metadata.fontDicts[oldDF]
		w := newWriter()

		// See comment in `rewriteTopDictIndex`.
		fontMatrix := [6]number{
			numberOne(),
			numberZero(),
			numberZero(),
			numberOne(),
			numberZero(),
			numberZero(),
		}
		if dict.hasFontMatrix {
			fontMatrix = dict.fontMatrix
			if topDictIsMissingFontMatrix {
				scale := [6]number{
					numberFromF32(1000.0),
					numberZero(),
					numberZero(),
					numberFromF32(1000.0),
					numberZero(),
					numberZero(),
				}
				fontMatrix = combineNumbers(fontMatrix, scale)
			}
		}
		if err := writeNumbers(w, fontMatrix[:]); err != nil {
			return err
		}
		opFontMatrix.write(w)

		// Write the length and offset of the private dict.
		// Private dicts have already been written, so the offsets are
		// already correct. This means that these two offsets are a bit
		// special compared to the others, since we never use the `location`
		// field of the offset and we don't overwrite it like we do for the
		// others.
		if newDF >= len(offsets.privateDictsLens) {
			return errSubset
		}
		offsets.privateDictsLens[newDF].value.writeAs5Bytes(w)

		if newDF >= len(offsets.privateDictsOffsets) {
			return errSubset
		}
		offsets.privateDictsOffsets[newDF].value.writeAs5Bytes(w)
		opPrivate.write(w)

		if dict.hasFontName {
			if fontName, ok := sidRemapper.getNewSID(dict.fontName); ok {
				numberFromI32(int32(fontName)).write(w)
				opFontName.write(w)
			}
		}

		dicts = append(dicts, w.finish())
		newDF++
	}

	index, err := createIndex(dicts)
	if err != nil {
		return err
	}
	index.write(w)

	return nil
}

// generateFontDictIndex is generate_font_dict_index in subsetter
// src/cff/dict/font_dict.rs. It generates a new font DICT INDEX for
// SID-keyed fonts.
func generateFontDictIndex(offsets *offsetsCFF, w *writer) error {
	subW := newWriter()

	// See comment in `rewriteTopDictIndex`
	for _, n := range []number{
		numberOne(),
		numberZero(),
		numberZero(),
		numberOne(),
		numberZero(),
		numberZero(),
	} {
		n.write(subW)
	}
	opFontMatrix.write(subW)

	// Write the length and offset of the private dict.
	// Private dicts have already been written, so the offsets are already
	// correct. This means that these two offsets are a bit special compared
	// to the others, since we never use the `location` field of the offset
	// and we don't overwrite it like we do for the others.
	if len(offsets.privateDictsLens) == 0 {
		return errSubset
	}
	offsets.privateDictsLens[0].value.writeAs5Bytes(subW)

	if len(offsets.privateDictsOffsets) == 0 {
		return errSubset
	}
	offsets.privateDictsOffsets[0].value.writeAs5Bytes(subW)

	opPrivate.write(subW)
	index, err := createIndex([][]byte{subW.finish()})
	if err != nil {
		return err
	}
	index.write(w)

	// TODO: Maybe write a font name as well? But shouldn't matter.
	return nil
}
