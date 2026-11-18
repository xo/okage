// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

// This file ports subsetter src/cff/dict/private_dict.rs. The parsing logic
// was adapted from ttf-parser.

// parseSubrOffset is parse_subr_offset in subsetter
// src/cff/dict/private_dict.rs. It parses the subroutine offset from a
// private dict.
func parseSubrOffset(data []byte) (uint64, bool) {
	dictParser := newDictionaryParser(data)

	for {
		operator, ok := dictParser.parseNext()
		if !ok {
			break
		}
		if operator == opSubrs {
			return dictParser.parseOffset()
		}
	}

	return 0, false
}

// rewriteCIDPrivateDicts is rewrite_cid_private_dicts in subsetter
// src/cff/dict/private_dict.rs. It writes the private dicts of a CID font
// for each font dict.
func rewriteCIDPrivateDicts(fdRemapper *fontDictRemapper, offsets *offsetsCFF, metadata *cidMetadata, w *writer) error {
	newDF := 0
	for oldDF := range fdRemapper.sortedIter() {
		if int(oldDF) >= len(metadata.fontDicts) {
			return errSubset
		}
		fontDict := &metadata.fontDicts[oldDF]
		if err := rewritePrivateDict(offsets, fontDict.privateDict, w, newDF); err != nil {
			return err
		}
		newDF++
	}

	return nil
}

// rewritePrivateDict is rewrite_private_dict in subsetter
// src/cff/dict/private_dict.rs.
func rewritePrivateDict(offsets *offsetsCFF, privateDictData []byte, w *writer, dictIndex int) error {
	privateDictOffset := w.len()

	dictParser := newDictionaryParser(privateDictData)

	subW := newWriter()

	// We just make sure that no subroutine offset gets written, all other
	// operators stay the same.
	for {
		operator, ok := dictParser.parseNext()
		if !ok {
			break
		}
		if operator == opSubrs {
			// We don't have any subroutines, so don't rewrite this DICT
			// entry.
			continue
		}
		if !dictParser.parseOperands() {
			return errMalformedFont
		}
		if err := writeNumbers(subW, dictParser.operandsSlice()); err != nil {
			return err
		}
		operator.write(subW)
	}

	data := subW.finish()

	privateDictLen := len(data)

	if dictIndex >= len(offsets.privateDictsLens) {
		return errSubset
	}
	if err := offsets.privateDictsLens[dictIndex].updateValue(privateDictLen); err != nil {
		return err
	}

	if dictIndex >= len(offsets.privateDictsOffsets) {
		return errSubset
	}
	if err := offsets.privateDictsOffsets[dictIndex].updateValue(privateDictOffset); err != nil {
		return err
	}

	w.extend(data)

	return nil
}
