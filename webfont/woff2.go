package webfont

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"

	"github.com/xo/okage/internal/brotli"
)

// woff2KnownTags is the table of known tags of section 4.1 of WOFF2. The
// low 6 bits of the flags of a table entry index it, and 63 means that the
// tag follows.
var woff2KnownTags = [63]string{
	"cmap", "head", "hhea", "hmtx", "maxp", "name", "OS/2", "post",
	"cvt ", "fpgm", "glyf", "loca", "prep", "CFF ", "VORG", "EBDT",
	"EBLC", "gasp", "hdmx", "kern", "LTSH", "PCLT", "VDMX", "vhea",
	"vmtx", "BASE", "GDEF", "GPOS", "GSUB", "EBSC", "JSTF", "MATH",
	"CBDT", "CBLC", "COLR", "CPAL", "SVG ", "sbix", "acnt", "avar",
	"bdat", "bloc", "bsln", "cvar", "fdsc", "feat", "fmtx", "fvar",
	"gvar", "hsty", "just", "lcar", "mort", "morx", "opbd", "prop",
	"trak", "Zapf", "Silf", "Glat", "Gloc", "Feat", "Sill",
}

// The tags of the tables that WOFF2 can transform, and of the tables that
// their reconstruction reads.
const (
	tagGlyf = 0x676c7966
	tagLoca = 0x6c6f6361
	tagHmtx = 0x686d7478
	tagHhea = 0x68686561
	tagMaxp = 0x6d617870
)

// woff2Entry is an entry of the table directory of WOFF2.
type woff2Entry struct {
	tag uint32
	// transformed reports whether the data of the table in the stream is
	// transformed: glyf and loca with the transform version 0, and hmtx
	// with the transform version 1.
	transformed bool
	origLength  uint32
	// length is the length of the table in the decompressed stream: its
	// transformLength when it is transformed, else its origLength.
	length uint32
	// data is the data of the table in the decompressed stream.
	data []byte
}

// woff2Reader reads the big-endian values of the header and the directories
// of WOFF2.
type woff2Reader struct {
	b   []byte
	err error
}

// fail records that the data is cut short or malformed.
func (r *woff2Reader) fail(f string, a ...any) {
	if r.err == nil {
		r.err = errorf("woff2", f, a...)
	}
}

// bytes returns the next n bytes, or nil when fewer remain.
func (r *woff2Reader) bytes(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > len(r.b) {
		r.fail("the data is cut short")
		return nil
	}
	b := r.b[:n]
	r.b = r.b[n:]
	return b
}

func (r *woff2Reader) u8() uint8 {
	if b := r.bytes(1); b != nil {
		return b[0]
	}
	return 0
}

func (r *woff2Reader) u16() uint16 {
	if b := r.bytes(2); b != nil {
		return binary.BigEndian.Uint16(b)
	}
	return 0
}

func (r *woff2Reader) u32() uint32 {
	if b := r.bytes(4); b != nil {
		return binary.BigEndian.Uint32(b)
	}
	return 0
}

// base128 reads a UIntBase128 of section 4.1 of WOFF2.
func (r *woff2Reader) base128() uint32 {
	var v uint32
	for i := range 5 {
		b := r.u8()
		if r.err != nil {
			return 0
		}
		// A leading zero is not allowed.
		if i == 0 && b == 0x80 {
			r.fail("a UIntBase128 has a leading zero")
			return 0
		}
		// The value must fit in 32 bits.
		if v&0xfe000000 != 0 {
			r.fail("a UIntBase128 overflows")
			return 0
		}
		v = v<<7 | uint32(b&0x7f)
		if b&0x80 == 0 {
			return v
		}
	}
	r.fail("a UIntBase128 is longer than 5 bytes")
	return 0
}

// u255 reads a 255UInt16 of section 6.1.1 of WOFF2.
func (r *woff2Reader) u255() uint16 {
	const oneMoreByteCode1, oneMoreByteCode2, wordCode, lowestUCode = 255, 254, 253, 253
	switch code := r.u8(); code {
	case wordCode:
		return r.u16()
	case oneMoreByteCode1:
		return uint16(r.u8()) + lowestUCode
	case oneMoreByteCode2:
		return uint16(r.u8()) + lowestUCode*2
	default:
		return uint16(code)
	}
}

// convertWOFF2 converts a font or a collection in WOFF2 into an sfnt file.
//
// See https://www.w3.org/TR/WOFF2/.
func convertWOFF2(data []byte) ([]byte, error) {
	const headerSize = 48
	r := &woff2Reader{b: data}
	r.u32() // signature
	flavor := r.u32()
	length := r.u32()
	numTables := int(r.u16())
	r.u16() // reserved
	r.u32() // totalSfntSize
	totalCompressedSize := r.u32()
	r.bytes(headerSize - 24)
	if r.err != nil {
		return nil, r.err
	}
	if int64(length) != int64(len(data)) {
		return nil, errorf("woff2", "the length in the header is not the length of the data")
	}
	if numTables == 0 {
		return nil, errorf("woff2", "no tables")
	}

	// The table directory.
	entries := make([]woff2Entry, numTables)
	var total uint64
	for i := range entries {
		e := &entries[i]
		flags := r.u8()
		if index := flags & 0x3f; index == 0x3f {
			e.tag = r.u32()
		} else {
			t := woff2KnownTags[index]
			e.tag = binary.BigEndian.Uint32([]byte(t))
		}
		version := flags >> 6
		e.origLength = r.base128()
		e.length = e.origLength
		switch e.tag {
		case tagGlyf, tagLoca:
			// For glyf and loca, the version 0 is the transform, and the
			// version 3 is the null transform.
			switch version {
			case 0:
				e.transformed = true
			case 3:
			default:
				r.fail("table %d has the unknown transform %d", i, version)
			}
		default:
			switch version {
			case 0:
			case 1:
				if e.tag != tagHmtx {
					r.fail("table %d has the unknown transform %d", i, version)
				}
				e.transformed = true
			default:
				r.fail("table %d has the unknown transform %d", i, version)
			}
		}
		if e.transformed {
			e.length = r.base128()
			if e.tag == tagLoca && e.length != 0 {
				r.fail("the transformed table loca is not empty")
			}
		}
		total += uint64(e.length)
	}
	if r.err != nil {
		return nil, r.err
	}

	// The collection directory, or the one font of the file.
	var fonts []sfntFont
	collection := flavor == sigCollection
	if collection {
		r.u32() // version
		numFonts := int(r.u255())
		if numFonts == 0 {
			return nil, errorf("woff2", "a collection has no fonts")
		}
		fonts = make([]sfntFont, numFonts)
		for i := range fonts {
			n := int(r.u255())
			fonts[i].flavor = r.u32()
			fonts[i].tables = make([]int, n)
			for j := range n {
				index := int(r.u255())
				if index >= numTables {
					r.fail("font %d names table %d, which does not exist", i, index)
				}
				fonts[i].tables[j] = index
			}
		}
		if r.err != nil {
			return nil, r.err
		}
	} else {
		font := sfntFont{flavor: flavor, tables: make([]int, numTables)}
		for i := range font.tables {
			font.tables[i] = i
		}
		fonts = []sfntFont{font}
	}

	// The compressed data, decompressed to exactly the sum of the lengths of
	// the tables.
	compressed := r.bytes(int(totalCompressedSize))
	if r.err != nil {
		return nil, r.err
	}
	if total > math.MaxInt32 {
		return nil, errorf("woff2", "the tables are too large")
	}
	stream, err := io.ReadAll(io.LimitReader(brotli.NewReader(bytes.NewReader(compressed)), int64(total)+1))
	if err != nil {
		return nil, errorf("woff2", "decompressing: %w", err)
	}
	if uint64(len(stream)) != total {
		return nil, errorf("woff2", "the tables decompress to %d bytes, not %d", len(stream), total)
	}
	for i := range entries {
		e := &entries[i]
		e.data = stream[:e.length]
		stream = stream[e.length:]
	}

	tables := make([]table, numTables)
	done := make([]bool, numTables)
	for _, f := range fonts {
		if err := reconstructFont(f, entries, tables, done); err != nil {
			return nil, err
		}
	}
	for i, e := range entries {
		if !done[i] {
			// A table that no font names is not written.
			tables[i] = table{tag: e.tag}
		}
	}
	return writeSFNT(fonts, tables, collection), nil
}

// reconstructFont fills tables with the tables of the font f, which entries
// describes. It reverses the transforms of glyf, loca and hmtx. done marks
// the tables that are already filled, by another font of a collection.
func reconstructFont(f sfntFont, entries []woff2Entry, tables []table, done []bool) error {
	find := func(tag uint32) int {
		for _, i := range f.tables {
			if entries[i].tag == tag {
				return i
			}
		}
		return -1
	}
	glyf, loca, hmtx := find(tagGlyf), find(tagLoca), find(tagHmtx)
	if glyf >= 0 && loca >= 0 && entries[glyf].transformed != entries[loca].transformed {
		return errorf("woff2", "only one of glyf and loca is transformed")
	}

	// xMins holds the xMin of each glyph, for the transform of hmtx.
	var xMins []int16
	if glyf >= 0 && entries[glyf].transformed {
		if loca < 0 {
			return errorf("woff2", "the transformed table glyf has no table loca")
		}
		g, l, mins, err := reconstructGlyf(entries[glyf].data)
		if err != nil {
			return err
		}
		if uint32(len(l)) != entries[loca].origLength {
			return errorf("woff2", "the table loca has %d bytes, not %d", len(l), entries[loca].origLength)
		}
		if !done[glyf] {
			tables[glyf] = table{tag: tagGlyf, data: g}
			tables[loca] = table{tag: tagLoca, data: l}
			done[glyf], done[loca] = true, true
		}
		xMins = mins
	}
	if hmtx >= 0 && entries[hmtx].transformed && !done[hmtx] {
		if xMins == nil {
			return errorf("woff2", "the transformed table hmtx has no transformed table glyf")
		}
		hhea, maxp := find(tagHhea), find(tagMaxp)
		if hhea < 0 || maxp < 0 {
			return errorf("woff2", "the transformed table hmtx has no table hhea or maxp")
		}
		b, err := reconstructHmtx(entries[hmtx].data, entries[hhea].data, entries[maxp].data, xMins)
		if err != nil {
			return err
		}
		if uint32(len(b)) != entries[hmtx].origLength {
			return errorf("woff2", "the table hmtx has %d bytes, not %d", len(b), entries[hmtx].origLength)
		}
		tables[hmtx] = table{tag: tagHmtx, data: b}
		done[hmtx] = true
	}
	for _, i := range f.tables {
		if done[i] {
			continue
		}
		e := entries[i]
		if e.transformed {
			return errorf("woff2", "the table %d is transformed and cannot be reconstructed", i)
		}
		tables[i] = table{tag: e.tag, data: e.data}
		done[i] = true
	}
	return nil
}

// reconstructHmtx reverses the transform of hmtx of section 5.4 of WOFF2. A
// left side bearing that the transform leaves out is the xMin of its glyph.
func reconstructHmtx(data, hhea, maxp []byte, xMins []int16) ([]byte, error) {
	if len(hhea) < 36 || len(maxp) < 6 {
		return nil, errorf("woff2", "the table hhea or maxp is cut short")
	}
	numHMetrics := int(binary.BigEndian.Uint16(hhea[34:]))
	numGlyphs := int(binary.BigEndian.Uint16(maxp[4:]))
	if numHMetrics == 0 || numHMetrics > numGlyphs || numGlyphs > len(xMins) {
		return nil, errorf("woff2", "the table hmtx has %d metrics for %d glyphs", numHMetrics, numGlyphs)
	}
	r := &woff2Reader{b: data}
	flags := r.u8()
	if flags&^3 != 0 {
		return nil, errorf("woff2", "the transformed table hmtx has the reserved flags %#x", flags)
	}
	advances := make([]uint16, numHMetrics)
	for i := range advances {
		advances[i] = r.u16()
	}
	lsbs := make([]int16, numGlyphs)
	for i := range numGlyphs {
		absent := flags&1 != 0
		if i >= numHMetrics {
			absent = flags&2 != 0
		}
		if absent {
			lsbs[i] = xMins[i]
		} else {
			lsbs[i] = int16(r.u16())
		}
	}
	if r.err != nil {
		return nil, r.err
	}
	out := make([]byte, 0, 4*numHMetrics+2*(numGlyphs-numHMetrics))
	for i := range numGlyphs {
		if i < numHMetrics {
			out = binary.BigEndian.AppendUint16(out, advances[i])
		}
		out = binary.BigEndian.AppendUint16(out, uint16(lsbs[i]))
	}
	return out, nil
}
