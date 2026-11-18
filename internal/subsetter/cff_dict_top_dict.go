// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

// This file ports subsetter src/cff/dict/top_dict.rs. The parsing logic was
// adapted from ttf-parser.

// topDictData is TopDictData in subsetter src/cff/dict/top_dict.rs. A field
// whose flag has* is false is None.
type topDictData struct {
	charset        uint64
	hasCharset     bool
	charStrings    uint64
	hasCharStrings bool
	private        dataRange
	hasPrivate     bool
	fdArray        uint64
	hasFDArray     bool
	fdSelect       uint64
	hasFDSelect    bool
	notice         stringID
	hasNotice      bool
	copyright      stringID
	hasCopyright   bool
	fontName       stringID
	hasFontName    bool
	hasROS         bool
	fontMatrix     [6]number
	hasFontMatrix  bool
	fontBBox       [4]number
	hasFontBBox    bool
}

// parseTopDictIndex is parse_top_dict_index in subsetter
// src/cff/dict/top_dict.rs.
func parseTopDictIndex(r *reader) (topDictData, bool) {
	var topDict topDictData

	index, ok := parseIndex(r)
	if !ok {
		return topDictData{}, false
	}

	// The Top DICT INDEX should have only one dictionary in CFF fonts.
	data, ok := index.get(0)
	if !ok {
		return topDictData{}, false
	}

	dictParser := newDictionaryParser(data)

	for {
		operator, ok := dictParser.parseNext()
		if !ok {
			break
		}
		switch operator {
		// We only need to preserve the copyrights and font name.
		case opNotice:
			topDict.notice, ok = dictParser.parseSID()
			topDict.hasNotice = true
		case opCopyright:
			topDict.copyright, ok = dictParser.parseSID()
			topDict.hasCopyright = true
		case opFontName:
			topDict.fontName, ok = dictParser.parseSID()
			topDict.hasFontName = true
		case opCharset:
			topDict.charset, ok = dictParser.parseOffset()
			topDict.hasCharset = true
		case opEncoding:
			// We don't care about encoding since we convert to CID-keyed
			// font anyway.
		case opCharStrings:
			topDict.charStrings, ok = dictParser.parseOffset()
			topDict.hasCharStrings = true
		case opPrivate:
			topDict.private, ok = dictParser.parseRange()
			topDict.hasPrivate = true
		case opROS:
			// We will rewrite the ROS, so no need to grab it from here. But
			// we need to register it, so we know we are dealing with a
			// CID-keyed font.
			topDict.hasROS = true
		case opFDArray:
			topDict.fdArray, ok = dictParser.parseOffset()
			topDict.hasFDArray = true
		case opFDSelect:
			topDict.fdSelect, ok = dictParser.parseOffset()
			topDict.hasFDSelect = true
		case opFontMatrix:
			topDict.fontMatrix, ok = dictParser.parseFontMatrix()
			topDict.hasFontMatrix = true
		case opFontBBox:
			topDict.fontBBox, ok = dictParser.parseFontBBox()
			topDict.hasFontBBox = true
		}
		if !ok {
			return topDictData{}, false
		}
	}

	return topDict, true
}

// rewriteTopDictIndex is rewrite_top_dict_index in subsetter
// src/cff/dict/top_dict.rs. It rewrites the top dict. Implementation is
// based on what ghostscript seems to keep when rewriting a font subset.
func rewriteTopDictIndex(topDictData *topDictData, offsets *offsetsCFF, sidRemapper *sidRemapper, w *writer) error {
	subW := newWriter()

	// ROS.
	adobe, ok := sidRemapper.get([]byte("Adobe"))
	if !ok {
		return errSubset
	}
	numberFromI32(int32(adobe)).write(subW)
	identity, ok := sidRemapper.get([]byte("Identity"))
	if !ok {
		return errSubset
	}
	numberFromI32(int32(identity)).write(subW)
	numberZero().write(subW)
	opROS.write(subW)

	// Copyright notices.
	if topDictData.hasCopyright {
		if copyright, ok := sidRemapper.getNewSID(topDictData.copyright); ok {
			numberFromI32(int32(copyright)).write(subW)
			opCopyright.write(subW)
		}
	}

	if topDictData.hasNotice {
		if notice, ok := sidRemapper.getNewSID(topDictData.notice); ok {
			numberFromI32(int32(notice)).write(subW)
			opNotice.write(subW)
		}
	}

	// Font name.
	if topDictData.hasFontName {
		if fontName, ok := sidRemapper.getNewSID(topDictData.fontName); ok {
			numberFromI32(int32(fontName)).write(subW)
			opFontName.write(subW)
		}
	}

	// See https://bugs.ghostscript.com/show_bug.cgi?id=690724#c12 and https://leahneukirchen.org/blog/archive/2022/10/50-blank-pages-or-black-box-debugging-of-pdf-rendering-in-printers.html
	// We assume that if at least one font dict has a matrix, all of them do.
	// Case 1: Top DICT MATRIX is some, FONT DICT MATRIX is some -> supplied, supplied
	// Case 2: Top DICT MATRIX is none, FONT DICT MATRIX is some -> (0.001, 0, 0, 0.001, 0, 0), supplied * 1000
	// Case 3: Top DICT MATRIX is some, FONT DICT MATRIX is none -> supplied, (1, 0, 0, 1, 0, 0)
	// Case 4: Top DICT MATRIX is none, FONT DICT MATRIX is none -> (0.001, 0, 0, 0.001, 0 0), (1, 0, 0, 1, 0, 0)
	fontMatrix := [6]number{
		numberFromF32(0.001),
		numberZero(),
		numberZero(),
		numberFromF32(0.001),
		numberZero(),
		numberZero(),
	}
	if topDictData.hasFontMatrix {
		fontMatrix = topDictData.fontMatrix
	}
	if err := writeNumbers(subW, fontMatrix[:]); err != nil {
		return err
	}
	opFontMatrix.write(subW)

	// Write a default font bbox, if it does not exist.
	var fontBBox [4]number
	if topDictData.hasFontBBox {
		fontBBox = topDictData.fontBBox
	}
	if err := writeNumbers(subW, fontBBox[:]); err != nil {
		return err
	}
	opFontBBox.write(subW)

	// Note: When writing the offsets, we need to add the current length of w
	// AND subW.
	// Charset
	offsets.charsetOffset.updateLocation(subW.len() + w.len())
	dummyValue.writeAs5Bytes(subW)
	opCharset.write(subW)

	// Charstrings
	offsets.charStringsOffset.updateLocation(subW.len() + w.len())
	dummyValue.writeAs5Bytes(subW)
	opCharStrings.write(subW)

	numberFromI32(0xFFFF).write(subW)
	opCIDCount.write(subW)

	// Note: Previously, we wrote those two entries directly after ROS.
	// However, for some reason not known to me, Apple Preview does not like
	// show the CFF font at all if that's the case. This is why we now write
	// the offsets in the very end.

	// FD array.
	offsets.fdArrayOffset.updateLocation(subW.len() + w.len())
	dummyValue.writeAs5Bytes(subW)
	opFDArray.write(subW)

	// FD select.
	offsets.fdSelectOffset.updateLocation(subW.len() + w.len())
	dummyValue.writeAs5Bytes(subW)
	opFDSelect.write(subW)

	finished := subW.finish()

	// TOP DICT INDEX always has size 1 in CFF.
	index, err := createIndex([][]byte{finished})
	if err != nil {
		return err
	}

	// The offsets we calculated before were calculated under the assumption
	// that the contents of subW will be appended directly to w. However,
	// when we create an index, the INDEX header data will be appended in the
	// beginning, meaning that we need to adjust the offsets to account for
	// that.
	offsets.charsetOffset.adjustLocation(index.headerSize)
	offsets.charStringsOffset.adjustLocation(index.headerSize)
	offsets.fdArrayOffset.adjustLocation(index.headerSize)
	offsets.fdSelectOffset.adjustLocation(index.headerSize)

	index.write(w)

	return nil
}
