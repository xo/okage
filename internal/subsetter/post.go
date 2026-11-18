// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"iter"
	"math"
)

// This file ports subsetter src/post.rs.
//
// Subset the `post` table. The `post` table contains name information for
// glyphs needed for some PostScript printers. Only version 2 table contains
// actual custom names, so this is the only version that we need to subset.
// All we need to do is to extract the strings for all requested glyphs and
// write them into a new `post` table in the given order.

// subsetPost is subset in subsetter src/post.rs.
func subsetPost(ctx *context) error {
	post, ok := ctx.expectTable(tagPOST)
	if !ok {
		return errMalformedFont
	}
	r := newReader(post)

	version, ok := r.readU32()
	if !ok {
		return errMalformedFont
	}
	if version != 0x00020000 {
		ctx.push(tagPOST, post)
		return nil
	}

	table, ok := parseVersion2Table(post)
	if !ok {
		return errMalformedFont
	}
	var names [][]byte
	for name := range table.names() {
		names = append(names, name)
	}

	subPost := newWriter()
	subPost.extend(table.header)
	subPost.writeU16(ctx.mapper.NumGids())

	stringStorage := newWriter()
	stringIndex := 0

	for oldGid := range ctx.mapper.RemappedGids() {
		index, ok := table.glyphIndexes.get(oldGid)
		if !ok {
			return errMalformedFont
		}

		// IDs smaller than 258 refer to the names in the Macintosh TrueType
		// file.
		if index <= 257 {
			subPost.writeU16(index)
		} else {
			index := index - 258
			// Phetsarath-Regular.ttf from Google Fonts seems to have a wrong
			// name table. If name cannot be fetched, use empty name instead.
			var name []byte
			if int(index) < len(names) {
				name = names[index]
			}
			if len(name) > math.MaxUint8 {
				return errOverflow
			}
			nameLen := uint8(len(name))
			if stringIndex+258 > math.MaxUint16 {
				return errOverflow
			}
			subPost.writeU16(uint16(stringIndex + 258))

			stringStorage.writeU8(nameLen)
			stringStorage.extend(name)
			stringIndex++
		}
	}

	subPost.extend(stringStorage.finish())

	ctx.push(tagPOST, subPost.finish())
	return nil
}

// postNames is Names in subsetter src/post.rs. An iterator over glyph names.
//
// The `post` table doesn't provide the glyph names count, so we have to
// simply iterate over all of them to find it out.
func postNames(data []byte) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		offset := 0
		for offset < len(data) {
			length := int(data[offset])
			offset++

			// An empty name is an error.
			if length == 0 {
				return
			}

			if offset+length > len(data) {
				return
			}
			name := data[offset : offset+length]
			offset += length
			if !yield(name) {
				return
			}
		}
	}
}

// version2Table is Version2Table in subsetter src/post.rs. A version 2 post
// table.
type version2Table struct {
	header       []byte
	glyphIndexes lazyArray16[uint16]
	namesData    []byte
}

// parseVersion2Table is Version2Table::parse in subsetter src/post.rs. It
// parses a version 2 table.
func parseVersion2Table(data []byte) (version2Table, bool) {
	// Do not check the exact length, because some fonts include padding in
	// table's length in table records, which is incorrect.
	r := newReader(data)
	if version, ok := r.readU32(); len(data) < 32 || !ok || version != 0x00020000 {
		return version2Table{}, false
	}

	r = newReader(data)
	header, ok := r.readBytes(32)
	if !ok {
		return version2Table{}, false
	}

	indexesCount, ok := r.readU16()
	if !ok {
		return version2Table{}, false
	}
	glyphIndexes, ok := readArray16(&r, indexesCount, 2, (*reader).readU16)
	if !ok {
		return version2Table{}, false
	}
	namesData, ok := r.tail()
	if !ok {
		return version2Table{}, false
	}

	return version2Table{header: header, glyphIndexes: glyphIndexes, namesData: namesData}, true
}

// names is Version2Table::names in subsetter src/post.rs. It returns an
// iterator over glyph names.
//
// Default/predefined names are not included. Just the one in the font file.
func (t version2Table) names() iter.Seq[[]byte] {
	return postNames(t.namesData)
}
