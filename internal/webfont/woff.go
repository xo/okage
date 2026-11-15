package webfont

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"
)

// convertWOFF converts a font in WOFF 1.0 into an sfnt file. Each table that
// is shorter than its original length is inflated with zlib.
//
// See https://www.w3.org/TR/WOFF/.
func convertWOFF(data []byte) ([]byte, error) {
	const headerSize, entrySize = 44, 20
	if len(data) < headerSize {
		return nil, errorf("woff", "the header is cut short")
	}
	be := binary.BigEndian
	flavor := be.Uint32(data[4:])
	if int(be.Uint32(data[8:])) != len(data) {
		return nil, errorf("woff", "the length in the header is not the length of the data")
	}
	numTables := int(be.Uint16(data[12:]))
	if numTables == 0 {
		return nil, errorf("woff", "no tables")
	}
	if len(data) < headerSize+numTables*entrySize {
		return nil, errorf("woff", "the table directory is cut short")
	}
	tables := make([]table, numTables)
	font := sfntFont{flavor: flavor, tables: make([]int, numTables)}
	for i := range numTables {
		e := data[headerSize+i*entrySize:]
		tag := be.Uint32(e)
		offset := int(be.Uint32(e[4:]))
		compLength := int(be.Uint32(e[8:]))
		origLength := int(be.Uint32(e[12:]))
		if offset < 0 || compLength < 0 || offset > len(data) || compLength > len(data)-offset || compLength > origLength {
			return nil, errorf("woff", "table %d is out of range", i)
		}
		b := data[offset : offset+compLength]
		if compLength < origLength {
			r, err := zlib.NewReader(bytes.NewReader(b))
			if err != nil {
				return nil, errorf("woff", "table %d: %w", i, err)
			}
			// Read one byte more than the original length, to find a
			// table that inflates to more.
			if b, err = io.ReadAll(io.LimitReader(r, int64(origLength)+1)); err != nil {
				return nil, errorf("woff", "table %d: %w", i, err)
			}
			if len(b) != origLength {
				return nil, errorf("woff", "table %d inflates to %d bytes, not %d", i, len(b), origLength)
			}
		}
		tables[i] = table{tag: tag, data: b}
		font.tables[i] = i
	}
	return writeSFNT([]sfntFont{font}, tables, false), nil
}
