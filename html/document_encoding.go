package html

// This file ports no upstream file. It holds what tinyhtml5 takes from the
// package webencodings and from the codecs of Python (D49): the labels of the
// encodings of the WHATWG Encoding Standard, the lookup of a label, and the
// decoders. html decodes UTF-8, UTF-16LE, UTF-16BE, windows-1252 and
// x-user-defined. A program decodes the other encodings with
// WithCharsetReader.

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// encodingLabel is a label of an encoding and the name of the encoding.
type encodingLabel struct {
	label string
	name  string
}

// encodingLabels is LABELS of webencodings 0.5.1, sorted by label. It is the
// table of the labels of the WHATWG Encoding Standard.
var encodingLabels = []encodingLabel{
	{"866", "ibm866"},
	{"ansi_x3.4-1968", "windows-1252"},
	{"arabic", "iso-8859-6"},
	{"ascii", "windows-1252"},
	{"asmo-708", "iso-8859-6"},
	{"big5", "big5"},
	{"big5-hkscs", "big5"},
	{"chinese", "gbk"},
	{"cn-big5", "big5"},
	{"cp1250", "windows-1250"},
	{"cp1251", "windows-1251"},
	{"cp1252", "windows-1252"},
	{"cp1253", "windows-1253"},
	{"cp1254", "windows-1254"},
	{"cp1255", "windows-1255"},
	{"cp1256", "windows-1256"},
	{"cp1257", "windows-1257"},
	{"cp1258", "windows-1258"},
	{"cp819", "windows-1252"},
	{"cp866", "ibm866"},
	{"csbig5", "big5"},
	{"cseuckr", "euc-kr"},
	{"cseucpkdfmtjapanese", "euc-jp"},
	{"csgb2312", "gbk"},
	{"csibm866", "ibm866"},
	{"csiso2022jp", "iso-2022-jp"},
	{"csiso2022kr", "iso-2022-kr"},
	{"csiso58gb231280", "gbk"},
	{"csiso88596e", "iso-8859-6"},
	{"csiso88596i", "iso-8859-6"},
	{"csiso88598e", "iso-8859-8"},
	{"csiso88598i", "iso-8859-8-i"},
	{"csisolatin1", "windows-1252"},
	{"csisolatin2", "iso-8859-2"},
	{"csisolatin3", "iso-8859-3"},
	{"csisolatin4", "iso-8859-4"},
	{"csisolatin5", "windows-1254"},
	{"csisolatin6", "iso-8859-10"},
	{"csisolatin9", "iso-8859-15"},
	{"csisolatinarabic", "iso-8859-6"},
	{"csisolatincyrillic", "iso-8859-5"},
	{"csisolatingreek", "iso-8859-7"},
	{"csisolatinhebrew", "iso-8859-8"},
	{"cskoi8r", "koi8-r"},
	{"csksc56011987", "euc-kr"},
	{"csmacintosh", "macintosh"},
	{"csshiftjis", "shift_jis"},
	{"cyrillic", "iso-8859-5"},
	{"dos-874", "windows-874"},
	{"ecma-114", "iso-8859-6"},
	{"ecma-118", "iso-8859-7"},
	{"elot_928", "iso-8859-7"},
	{"euc-jp", "euc-jp"},
	{"euc-kr", "euc-kr"},
	{"gb18030", "gb18030"},
	{"gb2312", "gbk"},
	{"gb_2312", "gbk"},
	{"gb_2312-80", "gbk"},
	{"gbk", "gbk"},
	{"greek", "iso-8859-7"},
	{"greek8", "iso-8859-7"},
	{"hebrew", "iso-8859-8"},
	{"hz-gb-2312", "hz-gb-2312"},
	{"ibm819", "windows-1252"},
	{"ibm866", "ibm866"},
	{"iso-2022-jp", "iso-2022-jp"},
	{"iso-2022-kr", "iso-2022-kr"},
	{"iso-8859-1", "windows-1252"},
	{"iso-8859-10", "iso-8859-10"},
	{"iso-8859-11", "windows-874"},
	{"iso-8859-13", "iso-8859-13"},
	{"iso-8859-14", "iso-8859-14"},
	{"iso-8859-15", "iso-8859-15"},
	{"iso-8859-16", "iso-8859-16"},
	{"iso-8859-2", "iso-8859-2"},
	{"iso-8859-3", "iso-8859-3"},
	{"iso-8859-4", "iso-8859-4"},
	{"iso-8859-5", "iso-8859-5"},
	{"iso-8859-6", "iso-8859-6"},
	{"iso-8859-6-e", "iso-8859-6"},
	{"iso-8859-6-i", "iso-8859-6"},
	{"iso-8859-7", "iso-8859-7"},
	{"iso-8859-8", "iso-8859-8"},
	{"iso-8859-8-e", "iso-8859-8"},
	{"iso-8859-8-i", "iso-8859-8-i"},
	{"iso-8859-9", "windows-1254"},
	{"iso-ir-100", "windows-1252"},
	{"iso-ir-101", "iso-8859-2"},
	{"iso-ir-109", "iso-8859-3"},
	{"iso-ir-110", "iso-8859-4"},
	{"iso-ir-126", "iso-8859-7"},
	{"iso-ir-127", "iso-8859-6"},
	{"iso-ir-138", "iso-8859-8"},
	{"iso-ir-144", "iso-8859-5"},
	{"iso-ir-148", "windows-1254"},
	{"iso-ir-149", "euc-kr"},
	{"iso-ir-157", "iso-8859-10"},
	{"iso-ir-58", "gbk"},
	{"iso8859-1", "windows-1252"},
	{"iso8859-10", "iso-8859-10"},
	{"iso8859-11", "windows-874"},
	{"iso8859-13", "iso-8859-13"},
	{"iso8859-14", "iso-8859-14"},
	{"iso8859-15", "iso-8859-15"},
	{"iso8859-2", "iso-8859-2"},
	{"iso8859-3", "iso-8859-3"},
	{"iso8859-4", "iso-8859-4"},
	{"iso8859-5", "iso-8859-5"},
	{"iso8859-6", "iso-8859-6"},
	{"iso8859-7", "iso-8859-7"},
	{"iso8859-8", "iso-8859-8"},
	{"iso8859-9", "windows-1254"},
	{"iso88591", "windows-1252"},
	{"iso885910", "iso-8859-10"},
	{"iso885911", "windows-874"},
	{"iso885913", "iso-8859-13"},
	{"iso885914", "iso-8859-14"},
	{"iso885915", "iso-8859-15"},
	{"iso88592", "iso-8859-2"},
	{"iso88593", "iso-8859-3"},
	{"iso88594", "iso-8859-4"},
	{"iso88595", "iso-8859-5"},
	{"iso88596", "iso-8859-6"},
	{"iso88597", "iso-8859-7"},
	{"iso88598", "iso-8859-8"},
	{"iso88599", "windows-1254"},
	{"iso_8859-1", "windows-1252"},
	{"iso_8859-15", "iso-8859-15"},
	{"iso_8859-1:1987", "windows-1252"},
	{"iso_8859-2", "iso-8859-2"},
	{"iso_8859-2:1987", "iso-8859-2"},
	{"iso_8859-3", "iso-8859-3"},
	{"iso_8859-3:1988", "iso-8859-3"},
	{"iso_8859-4", "iso-8859-4"},
	{"iso_8859-4:1988", "iso-8859-4"},
	{"iso_8859-5", "iso-8859-5"},
	{"iso_8859-5:1988", "iso-8859-5"},
	{"iso_8859-6", "iso-8859-6"},
	{"iso_8859-6:1987", "iso-8859-6"},
	{"iso_8859-7", "iso-8859-7"},
	{"iso_8859-7:1987", "iso-8859-7"},
	{"iso_8859-8", "iso-8859-8"},
	{"iso_8859-8:1988", "iso-8859-8"},
	{"iso_8859-9", "windows-1254"},
	{"iso_8859-9:1989", "windows-1254"},
	{"koi", "koi8-r"},
	{"koi8", "koi8-r"},
	{"koi8-r", "koi8-r"},
	{"koi8-u", "koi8-u"},
	{"koi8_r", "koi8-r"},
	{"korean", "euc-kr"},
	{"ks_c_5601-1987", "euc-kr"},
	{"ks_c_5601-1989", "euc-kr"},
	{"ksc5601", "euc-kr"},
	{"ksc_5601", "euc-kr"},
	{"l1", "windows-1252"},
	{"l2", "iso-8859-2"},
	{"l3", "iso-8859-3"},
	{"l4", "iso-8859-4"},
	{"l5", "windows-1254"},
	{"l6", "iso-8859-10"},
	{"l9", "iso-8859-15"},
	{"latin1", "windows-1252"},
	{"latin2", "iso-8859-2"},
	{"latin3", "iso-8859-3"},
	{"latin4", "iso-8859-4"},
	{"latin5", "windows-1254"},
	{"latin6", "iso-8859-10"},
	{"logical", "iso-8859-8-i"},
	{"mac", "macintosh"},
	{"macintosh", "macintosh"},
	{"ms_kanji", "shift_jis"},
	{"shift-jis", "shift_jis"},
	{"shift_jis", "shift_jis"},
	{"sjis", "shift_jis"},
	{"sun_eu_greek", "iso-8859-7"},
	{"tis-620", "windows-874"},
	{"unicode-1-1-utf-8", "utf-8"},
	{"us-ascii", "windows-1252"},
	{"utf-16", "utf-16le"},
	{"utf-16be", "utf-16be"},
	{"utf-16le", "utf-16le"},
	{"utf-8", "utf-8"},
	{"utf8", "utf-8"},
	{"visual", "iso-8859-8"},
	{"windows-1250", "windows-1250"},
	{"windows-1251", "windows-1251"},
	{"windows-1252", "windows-1252"},
	{"windows-1253", "windows-1253"},
	{"windows-1254", "windows-1254"},
	{"windows-1255", "windows-1255"},
	{"windows-1256", "windows-1256"},
	{"windows-1257", "windows-1257"},
	{"windows-1258", "windows-1258"},
	{"windows-31j", "shift_jis"},
	{"windows-874", "windows-874"},
	{"windows-949", "euc-kr"},
	{"x-cp1250", "windows-1250"},
	{"x-cp1251", "windows-1251"},
	{"x-cp1252", "windows-1252"},
	{"x-cp1253", "windows-1253"},
	{"x-cp1254", "windows-1254"},
	{"x-cp1255", "windows-1255"},
	{"x-cp1256", "windows-1256"},
	{"x-cp1257", "windows-1257"},
	{"x-cp1258", "windows-1258"},
	{"x-euc-jp", "euc-jp"},
	{"x-gbk", "gbk"},
	{"x-mac-cyrillic", "x-mac-cyrillic"},
	{"x-mac-roman", "macintosh"},
	{"x-mac-ukrainian", "x-mac-cyrillic"},
	{"x-sjis", "shift_jis"},
	{"x-user-defined", "x-user-defined"},
	{"x-x-big5", "big5"},
}

// The names of the encodings that html decodes.
const (
	encodingUTF8         = "utf-8"
	encodingUTF16LE      = "utf-16le"
	encodingUTF16BE      = "utf-16be"
	encodingWindows1252  = "windows-1252"
	encodingXUserDefined = "x-user-defined"
)

// lookupEncoding is lookup in webencodings, the "get an encoding" algorithm
// of the WHATWG Encoding Standard. It returns the name of the encoding of a
// label, or false for an unknown label.
func lookupEncoding(label string) (string, bool) {
	// Only strip ASCII whitespace: U+0009, U+000A, U+000C, U+000D, and U+0020.
	label = asciiUpperToLower(strings.Trim(label, "\t\n\f\r "))
	i, ok := slices.BinarySearchFunc(encodingLabels, label, func(e encodingLabel, l string) int {
		return strings.Compare(e.label, l)
	})
	if !ok {
		return "", false
	}
	return encodingLabels[i].name, true
}

// decodeBytes decodes data with the encoding name. Each malformed sequence
// becomes U+FFFD, as the "replace" errors of Python do. It returns false for
// an encoding that html does not decode.
func decodeBytes(name string, data []byte) (string, bool) {
	switch name {
	case encodingUTF8:
		return decodeUTF8(data), true
	case encodingUTF16LE:
		return decodeUTF16(data, false), true
	case encodingUTF16BE:
		return decodeUTF16(data, true), true
	case encodingWindows1252:
		return decodeWindows1252(data), true
	case encodingXUserDefined:
		return decodeXUserDefined(data), true
	}
	return "", false
}

// decodeUTF8 is the UTF-8 decoder of the WHATWG Encoding Standard. Each
// maximal part of a sequence that is not valid becomes one U+FFFD.
func decodeUTF8(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	var b strings.Builder
	b.Grow(len(data) + len(data)/2)
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r != utf8.RuneError || size > 1 {
			b.WriteRune(r)
			i += size
			continue
		}
		b.WriteRune(utf8.RuneError)
		i += utf8MaximalSubpart(data[i:])
	}
	return b.String()
}

// utf8MaximalSubpart returns the length of the start of data that is the
// start of a valid sequence of UTF-8, and at least 1. data does not start
// with a valid sequence.
func utf8MaximalSubpart(data []byte) int {
	lo, hi := byte(0x80), byte(0xBF)
	var need int
	switch c := data[0]; {
	case 0xC2 <= c && c <= 0xDF:
		need = 1
	case c == 0xE0:
		lo, need = 0xA0, 2
	case c == 0xED:
		hi, need = 0x9F, 2
	case 0xE1 <= c && c <= 0xEF:
		need = 2
	case c == 0xF0:
		lo, need = 0x90, 3
	case 0xF1 <= c && c <= 0xF3:
		need = 3
	case c == 0xF4:
		hi, need = 0x8F, 3
	default:
		return 1
	}
	n := 1
	for ; n <= need && n < len(data); n++ {
		if data[n] < lo || data[n] > hi {
			break
		}
		lo, hi = 0x80, 0xBF
	}
	return n
}

// decodeUTF16 is the UTF-16LE or the UTF-16BE decoder of the WHATWG Encoding
// Standard.
func decodeUTF16(data []byte, be bool) string {
	var b strings.Builder
	b.Grow(len(data))
	lead := rune(-1)
	for i := 0; i+1 < len(data); i += 2 {
		u := rune(data[i]) | rune(data[i+1])<<8
		if be {
			u = rune(data[i])<<8 | rune(data[i+1])
		}
		if lead >= 0 {
			if 0xDC00 <= u && u <= 0xDFFF {
				b.WriteRune(0x10000 + (lead-0xD800)<<10 + (u - 0xDC00))
				lead = -1
				continue
			}
			b.WriteRune(utf8.RuneError)
			lead = -1
		}
		switch {
		case 0xD800 <= u && u <= 0xDBFF:
			lead = u
		case 0xDC00 <= u && u <= 0xDFFF:
			b.WriteRune(utf8.RuneError)
		default:
			b.WriteRune(u)
		}
	}
	if lead >= 0 || len(data)%2 == 1 {
		b.WriteRune(utf8.RuneError)
	}
	return b.String()
}

// decodeWindows1252 is the windows-1252 decoder of the WHATWG Encoding
// Standard. The codec cp1252 of Python gives U+FFFD for the bytes 0x81,
// 0x8D, 0x8F, 0x90 and 0x9D, and the standard gives the control character of
// the same number.
func decodeWindows1252(data []byte) string {
	var b strings.Builder
	b.Grow(len(data) + len(data)/2)
	for _, c := range data {
		switch {
		case c < 0x80:
			b.WriteByte(c)
		case c < 0xA0:
			b.WriteRune(windows1252High[c-0x80])
		default:
			b.WriteRune(rune(c))
		}
	}
	return b.String()
}

// decodeXUserDefined is the x-user-defined decoder of the WHATWG Encoding
// Standard.
func decodeXUserDefined(data []byte) string {
	var b strings.Builder
	b.Grow(len(data) + len(data)/2)
	for _, c := range data {
		if c < 0x80 {
			b.WriteByte(c)
			continue
		}
		b.WriteRune(0xF780 + rune(c) - 0x80)
	}
	return b.String()
}
