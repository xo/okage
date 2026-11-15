package webfont

import (
	"encoding/binary"

	"github.com/xo/okage"
)

// The fields of the header of an EOT file that the conversion reads. The
// header is little-endian.
const (
	eotMagicOffset = 34
	eotMagic       = 0x504c
	eotMinHeader   = 36

	// eotFlagCompressed is TTEMBED_TTCOMPRESSED: the font data uses
	// MicroType Express compression.
	eotFlagCompressed = 0x00000004
	// eotFlagXOR is TTEMBED_XORENCRYPTDATA: each byte of the font data is
	// XORed with 0x50.
	eotFlagXOR = 0x10000000
)

// isEOT reports whether data starts with the header of an EOT file: the magic
// number at its place, and a size that is the size of data.
func isEOT(data []byte) bool {
	if len(data) < eotMinHeader {
		return false
	}
	le := binary.LittleEndian
	return le.Uint16(data[eotMagicOffset:]) == eotMagic && int64(le.Uint32(data)) == int64(len(data))
}

// convertEOT returns the font data of an EOT file. The font data is at the
// end of the file, after the header. It returns okage.ErrFontUnsupported for
// font data with MicroType Express compression.
//
// See https://www.w3.org/submissions/EOT/.
func convertEOT(data []byte) ([]byte, error) {
	le := binary.LittleEndian
	eotSize := int64(le.Uint32(data))
	fontSize := int64(le.Uint32(data[4:]))
	flags := le.Uint32(data[12:])
	if fontSize == 0 || fontSize > eotSize-eotMinHeader {
		return nil, errorf("eot", "the size of the font data is out of range")
	}
	if flags&eotFlagCompressed != 0 {
		return nil, okage.ErrFontUnsupported
	}
	font := data[eotSize-fontSize : eotSize]
	if flags&eotFlagXOR != 0 {
		font = append([]byte(nil), font...)
		for i := range font {
			font[i] ^= 0x50
		}
	}
	if DetectFormat(font) != FormatSFNT {
		return nil, errorf("eot", "the font data is not a TrueType or OpenType font")
	}
	return font, nil
}
