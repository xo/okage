// Copyright 2013-2020 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package tinycss2

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// Encoding is Encoding of webencodings, the character encoding of a
// stylesheet.
//
// The port decodes UTF-8, UTF-16LE, UTF-16BE, windows-1252 and
// x-user-defined, with all the labels that webencodings gives them. Python
// decodes every encoding of the Encoding Standard. For a label of another
// encoding, LookupEncoding returns nil, as for an unknown label (D50).
type Encoding struct {
	// Name is the name of the encoding in the Encoding Standard, such as
	// "utf-8" or "windows-1252".
	Name string
	// decode decodes data, and replaces each invalid sequence with U+FFFD.
	decode func(data []byte) string
}

// The names of the encodings of UTF-16.
const (
	nameUTF16LE = "utf-16le"
	nameUTF16BE = "utf-16be"
)

// The encodings that the port decodes.
var (
	// UTF8 is UTF8 of webencodings.
	UTF8     = &Encoding{"utf-8", decodeUTF8}
	utf16LE  = &Encoding{nameUTF16LE, func(data []byte) string { return decodeUTF16(data, false) }}
	utf16BE  = &Encoding{nameUTF16BE, func(data []byte) string { return decodeUTF16(data, true) }}
	win1252  = &Encoding{"windows-1252", decodeWindows1252}
	userDefd = &Encoding{"x-user-defined", decodeUserDefined}
)

// encodingLabels is the part of LABELS of webencodings for the encodings that
// the port decodes.
var encodingLabels = map[string]*Encoding{
	"unicode-1-1-utf-8": UTF8,
	"utf-8":             UTF8,
	"utf8":              UTF8,
	"ansi_x3.4-1968":    win1252,
	"ascii":             win1252,
	"cp1252":            win1252,
	"cp819":             win1252,
	"csisolatin1":       win1252,
	"ibm819":            win1252,
	"iso-8859-1":        win1252,
	"iso-ir-100":        win1252,
	"iso8859-1":         win1252,
	"iso88591":          win1252,
	"iso_8859-1":        win1252,
	"iso_8859-1:1987":   win1252,
	"l1":                win1252,
	"latin1":            win1252,
	"us-ascii":          win1252,
	"windows-1252":      win1252,
	"x-cp1252":          win1252,
	"utf-16be":          utf16BE,
	"utf-16":            utf16LE,
	"utf-16le":          utf16LE,
	"x-user-defined":    userDefd,
}

// LookupEncoding is lookup of webencodings. It returns the encoding of a
// label, or nil. It strips the ASCII whitespace of the label, and matches it
// in ASCII lower case.
func LookupEncoding(label string) *Encoding {
	label = asciiLower(strings.Trim(label, "\t\n\f\r "))
	return encodingLabels[label]
}

// DecodeStylesheetBytes is decode_stylesheet_bytes in tinycss2
// tinycss2/bytes.py. It finds the character encoding of a CSS stylesheet and
// decodes it, from a BOM, an @charset rule and the encoding meta-information.
//
// protocolEncoding is the encoding label of HTTP or of an equivalent
// protocol, such as the charset parameter of the Content-Type header, or the
// empty string. environmentEncoding is the environment encoding, or nil.
//
// It returns the decoded string and the encoding that it used.
func DecodeStylesheetBytes(cssBytes []byte, protocolEncoding string, environmentEncoding *Encoding) (string, *Encoding) {
	// https://drafts.csswg.org/css-syntax/#the-input-byte-stream
	if protocolEncoding != "" {
		if fallback := LookupEncoding(protocolEncoding); fallback != nil {
			return decode(cssBytes, fallback)
		}
	}
	if bytes.HasPrefix(cssBytes, []byte(`@charset "`)) {
		// 10 is len(b'@charset "')
		// 100 is arbitrary so that no encoding label is more than 100-10 bytes.
		endQuote := -1
		if i := bytes.IndexByte(cssBytes[10:min(100, len(cssBytes))], '"'); i != -1 {
			endQuote = 10 + i
		}
		if endQuote != -1 && bytes.HasPrefix(cssBytes[endQuote:], []byte(`";`)) {
			if fallback := LookupEncoding(decodeLatin1(cssBytes[10:endQuote])); fallback != nil {
				if fallback.Name == nameUTF16BE || fallback.Name == nameUTF16LE {
					return decode(cssBytes, UTF8)
				}
				return decode(cssBytes, fallback)
			}
		}
	}
	if environmentEncoding != nil {
		return decode(cssBytes, environmentEncoding)
	}
	return decode(cssBytes, UTF8)
}

// ParseStylesheetBytes is parse_stylesheet_bytes in tinycss2
// tinycss2/bytes.py. It parses a stylesheet from bytes, and finds its
// character encoding as web browsers do. Use it to read a file or a URL.
//
// It finds the encoding as DecodeStylesheetBytes does. The last fallback is
// UTF-8. The options work as in ParseStylesheet.
//
// It returns the rules, as ParseStylesheet does, and the encoding that it
// used. If the rules hold an @import rule, the encoding is the environment
// encoding of the imported stylesheet.
func ParseStylesheetBytes(cssBytes []byte, protocolEncoding string, environmentEncoding *Encoding, opts ...Option) ([]Node, *Encoding) {
	cssUnicode, encoding := DecodeStylesheetBytes(cssBytes, protocolEncoding, environmentEncoding)
	stylesheet := ParseStylesheet(cssUnicode, opts...)
	return stylesheet, encoding
}

// decode is decode of webencodings. A BOM wins over the fallback encoding,
// and decode removes it.
func decode(input []byte, fallback *Encoding) (string, *Encoding) {
	encoding := fallback
	switch {
	case bytes.HasPrefix(input, []byte{0xFF, 0xFE}):
		encoding, input = utf16LE, input[2:]
	case bytes.HasPrefix(input, []byte{0xFE, 0xFF}):
		encoding, input = utf16BE, input[2:]
	case bytes.HasPrefix(input, []byte{0xEF, 0xBB, 0xBF}):
		encoding, input = UTF8, input[3:]
	}
	return encoding.decode(input), encoding
}

// decodeLatin1 is bytes.decode('latin1') of Python.
func decodeLatin1(data []byte) string {
	var b strings.Builder
	for _, c := range data {
		b.WriteRune(rune(c))
	}
	return b.String()
}

// decodeUTF8 decodes UTF-8 as the codec utf-8 of Python does with the error
// handler "replace": one U+FFFD for each maximal invalid subpart.
func decodeUTF8(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	var b strings.Builder
	for i := 0; i < len(data); {
		c := data[i]
		if c < 0x80 {
			b.WriteByte(c)
			i++
			continue
		}
		var need int
		lower, upper := byte(0x80), byte(0xBF)
		switch {
		case c >= 0xC2 && c <= 0xDF:
			need = 1
		case c >= 0xE0 && c <= 0xEF:
			need = 2
			switch c {
			case 0xE0:
				lower = 0xA0
			case 0xED:
				upper = 0x9F
			}
		case c >= 0xF0 && c <= 0xF4:
			need = 3
			switch c {
			case 0xF0:
				lower = 0x90
			case 0xF4:
				upper = 0x8F
			}
		default:
			b.WriteRune(utf8.RuneError)
			i++
			continue
		}
		j := i + 1
		for k := range need {
			if j >= len(data) || data[j] < lower || data[j] > upper {
				break
			}
			if k == 0 {
				lower, upper = 0x80, 0xBF
			}
			j++
		}
		if j-i == need+1 {
			b.Write(data[i:j])
		} else {
			b.WriteRune(utf8.RuneError)
		}
		i = j
	}
	return b.String()
}

// decodeUTF16 decodes UTF-16 as the codecs utf-16-le and utf-16-be of Python
// do with the error handler "replace".
func decodeUTF16(data []byte, bigEndian bool) string {
	unit := func(i int) rune {
		if bigEndian {
			return rune(data[i])<<8 | rune(data[i+1])
		}
		return rune(data[i+1])<<8 | rune(data[i])
	}
	var b strings.Builder
	i := 0
	for i+1 < len(data) {
		u := unit(i)
		i += 2
		switch {
		case u < 0xD800 || u > 0xDFFF:
			b.WriteRune(u)
		case u <= 0xDBFF:
			if i+1 < len(data) {
				if v := unit(i); v >= 0xDC00 && v <= 0xDFFF {
					b.WriteRune(0x10000 + (u-0xD800)<<10 + (v - 0xDC00))
					i += 2
					continue
				}
			} else if i < len(data) {
				// A high surrogate and one byte: truncated data.
				b.WriteRune(utf8.RuneError)
				return b.String()
			}
			b.WriteRune(utf8.RuneError)
		default:
			b.WriteRune(utf8.RuneError)
		}
	}
	if i < len(data) {
		b.WriteRune(utf8.RuneError)
	}
	return b.String()
}

// windows1252High maps the bytes 0x80 to 0x9F of the codec cp1252 of Python.
// Python leaves 0x81, 0x8D, 0x8F, 0x90 and 0x9D undefined, so the error
// handler "replace" gives U+FFFD for them.
var windows1252High = [32]rune{
	0x20AC, 0xFFFD, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0xFFFD, 0x017D, 0xFFFD,
	0xFFFD, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0xFFFD, 0x017E, 0x0178,
}

// decodeWindows1252 decodes windows-1252 as the codec cp1252 of Python does.
func decodeWindows1252(data []byte) string {
	var b strings.Builder
	for _, c := range data {
		if c >= 0x80 && c <= 0x9F {
			b.WriteRune(windows1252High[c-0x80])
		} else {
			b.WriteRune(rune(c))
		}
	}
	return b.String()
}

// decodeUserDefined decodes x-user-defined as webencodings does.
func decodeUserDefined(data []byte) string {
	var b strings.Builder
	for _, c := range data {
		if c < 0x80 {
			b.WriteByte(c)
		} else {
			b.WriteRune(0xF700 + rune(c))
		}
	}
	return b.String()
}
