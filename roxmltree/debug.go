package roxmltree

// This file ports no file of roxmltree. It holds the fmt::Debug of str and
// char of the Rust core library, which the Display of Error and the AST dump
// of tests/ast.rs print.

import (
	"strconv"
	"strings"
	"unicode"
)

// rustDebugStr is fmt::Debug for str of the Rust core library. It quotes s
// with double quotes and escapes it as char::escape_debug_ext does, with
// escape_grapheme_extended and escape_double_quote.
//
// Port: Rust decides what is printable and what is a grapheme extender with
// its own Unicode tables, and Go uses those of the package unicode. The two
// can follow different versions of Unicode.
func rustDebugStr(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		rustEscapeDebug(&b, c, false, true)
	}
	b.WriteByte('"')
	return b.String()
}

// rustDebugChar is fmt::Debug for char of the Rust core library. It quotes c
// with single quotes and escapes it as char::escape_debug_ext does, with
// escape_grapheme_extended and escape_single_quote.
func rustDebugChar(c rune) string {
	var b strings.Builder
	b.WriteByte('\'')
	rustEscapeDebug(&b, c, true, false)
	b.WriteByte('\'')
	return b.String()
}

// rustNone is the Debug text of None.
const rustNone = "None"

// rustDebugOptionStr is fmt::Debug for Option<&str> of the Rust core
// library: None, or Some with the quoted string.
func rustDebugOptionStr(s string, ok bool) string {
	if !ok {
		return rustNone
	}
	return "Some(" + rustDebugStr(s) + ")"
}

// rustEscapeDebug is char::escape_debug_ext of the Rust core library, with
// escape_grapheme_extended always on.
func rustEscapeDebug(b *strings.Builder, c rune, escapeSingleQuote, escapeDoubleQuote bool) {
	switch {
	case c == 0:
		b.WriteString(`\0`)
	case c == '\t':
		b.WriteString(`\t`)
	case c == '\r':
		b.WriteString(`\r`)
	case c == '\n':
		b.WriteString(`\n`)
	case c == '\\':
		b.WriteString(`\\`)
	case c == '"' && escapeDoubleQuote:
		b.WriteString(`\"`)
	case c == '\'' && escapeSingleQuote:
		b.WriteString(`\'`)
	case isGraphemeExtended(c) || !unicode.IsPrint(c):
		b.WriteString(`\u{`)
		b.WriteString(strconv.FormatUint(uint64(c), 16))
		b.WriteByte('}')
	default:
		b.WriteRune(c)
	}
}

// isGraphemeExtended is char::is_grapheme_extended of the Rust core library:
// the Unicode property Grapheme_Extend, which is the categories Mn and Me and
// the property Other_Grapheme_Extend.
func isGraphemeExtended(c rune) bool {
	if c < 0x300 {
		return false
	}
	return unicode.In(c, unicode.Mn, unicode.Me, unicode.Other_Grapheme_Extend)
}
