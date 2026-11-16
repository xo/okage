// Package webfont converts a font in WOFF, WOFF2 or EOT into an sfnt file,
// the format of a TrueType or OpenType font file (D45). It is public, so that
// ivy and other programs can read fonts in a web format (D51). It ports no
// upstream. It follows the WOFF 1.0 and WOFF2 recommendations of the W3C and
// the EOT submission to the W3C.
package webfont

import (
	"cmp"
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/xo/okage"
)

// Format is the format of font data.
type Format int

// The formats of font data.
const (
	// FormatUnknown is data in no format of this list.
	FormatUnknown Format = iota
	// FormatSFNT is a TrueType or OpenType font, or a collection of them.
	FormatSFNT
	// FormatWOFF is a font in WOFF 1.0.
	FormatWOFF
	// FormatWOFF2 is a font or a collection in WOFF2.
	FormatWOFF2
	// FormatEOT is a font in EOT.
	FormatEOT
)

// The signatures at the start of font data, as a big-endian uint32.
const (
	sigTrueType   = 0x00010000
	sigOpenType   = 0x4f54544f // OTTO
	sigAppleTrue  = 0x74727565 // true
	sigPostScript = 0x74797031 // typ1
	sigCollection = 0x74746366 // ttcf
	sigWOFF       = 0x774f4646 // wOFF
	sigWOFF2      = 0x774f4632 // wOF2
)

// DetectFormat returns the format of data, from its first bytes.
func DetectFormat(data []byte) Format {
	if len(data) < 4 {
		return FormatUnknown
	}
	switch binary.BigEndian.Uint32(data) {
	case sigTrueType, sigOpenType, sigAppleTrue, sigPostScript, sigCollection:
		return FormatSFNT
	case sigWOFF:
		return FormatWOFF
	case sigWOFF2:
		return FormatWOFF2
	}
	if isEOT(data) {
		return FormatEOT
	}
	return FormatUnknown
}

// Convert returns the sfnt bytes of a font in WOFF, WOFF2 or EOT. It returns
// data as it is when data is already an sfnt file. It returns an error that
// wraps okage.ErrFontData for malformed data, and okage.ErrFontUnsupported for
// data in another format.
func Convert(data []byte) ([]byte, error) {
	switch DetectFormat(data) {
	case FormatSFNT:
		return data, nil
	case FormatWOFF:
		return convertWOFF(data)
	case FormatWOFF2:
		return convertWOFF2(data)
	case FormatEOT:
		return convertEOT(data)
	}
	return nil, okage.ErrFontUnsupported
}

// errorf returns an error that wraps okage.ErrFontData, with the format
// name and the text of f and a.
func errorf(format, f string, a ...any) error {
	return fmt.Errorf("%w: %s: "+f, append([]any{okage.ErrFontData, format}, a...)...)
}

// table is a table of an sfnt file.
type table struct {
	tag  uint32
	data []byte
}

// sfntFont is one font of an sfnt file: its signature and the indexes of its
// tables in the list of tables of the file.
type sfntFont struct {
	flavor uint32
	tables []int
}

// checksum returns the checksum of the data of a table: the sum of its
// big-endian uint32 values, with the last one padded with zeros.
func checksum(b []byte) uint32 {
	var sum uint32
	for len(b) >= 4 {
		sum += binary.BigEndian.Uint32(b)
		b = b[4:]
	}
	if len(b) > 0 {
		var last [4]byte
		copy(last[:], b)
		sum += binary.BigEndian.Uint32(last[:])
	}
	return sum
}

// pad4 returns n rounded up to a multiple of 4.
func pad4(n int) int {
	return (n + 3) &^ 3
}

// tagHead is the tag of the table head.
const tagHead = 0x68656164

// writeSFNT writes an sfnt file of the fonts, whose tables are in tables. It
// writes a collection when there is more than one font, or when collection
// is true. Each table is written once, after the table directories, and
// fonts that share a table point to the same data. For a single font, it
// sets checkSumAdjustment in the table head.
func writeSFNT(fonts []sfntFont, tables []table, collection bool) []byte {
	collection = collection || len(fonts) > 1
	size := 0
	if collection {
		size = 12 + 4*len(fonts)
	}
	dirOffsets := make([]int, len(fonts))
	for i, f := range fonts {
		dirOffsets[i] = size
		size += 12 + 16*len(f.tables)
	}
	offsets := make([]int, len(tables))
	for i, t := range tables {
		size = pad4(size)
		offsets[i] = size
		size += len(t.data)
	}
	out := make([]byte, pad4(size))
	be := binary.BigEndian
	if collection {
		be.PutUint32(out, sigCollection)
		be.PutUint32(out[4:], 0x00010000)
		be.PutUint32(out[8:], uint32(len(fonts)))
		for i := range fonts {
			be.PutUint32(out[12+4*i:], uint32(dirOffsets[i]))
		}
	}
	headIndex := -1
	for i, t := range tables {
		copy(out[offsets[i]:], t.data)
		if t.tag == tagHead && len(t.data) >= 12 {
			// The checksum of head is computed with checkSumAdjustment
			// set to 0.
			be.PutUint32(out[offsets[i]+8:], 0)
			headIndex = i
		}
	}
	for i, f := range fonts {
		// The directory lists the tables in the order of their tags.
		order := slices.Clone(f.tables)
		slices.SortFunc(order, func(a, b int) int {
			return cmp.Compare(tables[a].tag, tables[b].tag)
		})
		d := out[dirOffsets[i]:]
		n := len(order)
		entrySelector := 0
		for 2<<entrySelector <= n {
			entrySelector++
		}
		searchRange := 16 << entrySelector
		be.PutUint32(d, f.flavor)
		be.PutUint16(d[4:], uint16(n))
		be.PutUint16(d[6:], uint16(searchRange))
		be.PutUint16(d[8:], uint16(entrySelector))
		be.PutUint16(d[10:], uint16(16*n-searchRange))
		for j, ti := range order {
			e := d[12+16*j:]
			t := tables[ti]
			be.PutUint32(e, t.tag)
			be.PutUint32(e[4:], checksum(out[offsets[ti]:offsets[ti]+len(t.data)]))
			be.PutUint32(e[8:], uint32(offsets[ti]))
			be.PutUint32(e[12:], uint32(len(t.data)))
		}
	}
	if !collection && headIndex >= 0 {
		be.PutUint32(out[offsets[headIndex]+8:], 0xb1b0afba-checksum(out))
	}
	return out
}
