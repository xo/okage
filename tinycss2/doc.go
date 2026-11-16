// Copyright 2013-2020 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// Package tinycss2 is a port of the Python package tinycss2 1.5.1, a
// low-level CSS parser and generator. It parses strings into tokens, blocks
// and rules, and serializes them back to CSS (D50). It is public, so that ivy
// can port the layout of WeasyPrint, which reads its CSS with tinycss2.
//
// # How the Python types map to Go
//
// Each class of tinycss2/ast.py is a struct of the same name, and a pointer
// to it implements Node. Node.Type returns the class attribute type of
// upstream, such as "ident" or "{} block", so that a port can switch on it as
// upstream code does. Each attribute is a field in MixedCaps: value is Value,
// lower_value is LowerValue, int_value is IntValue, is_integer is IsInteger,
// lower_unit is LowerUnit, arguments is Arguments, content is Content and
// prelude is Prelude. source_line and source_column are the fields
// SourceLine and SourceColumn of the embedded Location.
//
//   - A Python list of component values is a []Node.
//   - None, where upstream returns it for a node, is a nil Node.
//   - The None of AtRule.content is a nil Content. An empty block gives an
//     empty Content that is not nil.
//   - The None of int_value is IsInteger false.
//   - The comparison `token == ','` of a LiteralToken is IsLiteral(token, ",").
//   - The functions that take a string or an iterable of component values
//     take an Input, which is a string or a []Node.
//   - The keyword arguments skip_comments and skip_whitespace are the
//     options WithSkipComments and WithSkipWhitespace.
//   - parse_color of color3.py, color4.py and color5.py are ParseColor3,
//     ParseColor4 and ParseColor5. RGBA is RGBA, and the Color of color4.py
//     and color5.py is Color. The string 'currentColor' is a second result
//     of true.
//   - parse_nth returns a, b and true, or false for None.
//   - webencodings.Encoding is *Encoding, and lookup is LookupEncoding.
//
// # Differences from upstream
//
// The port keeps the behavior of upstream, with its faults, except where
// Python raises an exception on bad input, or where Go cannot hold the same
// value (D50):
//
//   - The tokenizer nests at most DefaultMaxDepth blocks and functions, or
//     the value of WithMaxDepth. A deeper block becomes a ParseError of the
//     kind "too-deep", and its content is dropped.
//   - An escape of a surrogate code point gives U+FFFD, as a Go string
//     cannot hold a lone surrogate.
//   - An integer out of the range of int64 saturates in IntValue.
//   - Serialization writes nothing for a ParseError of a kind that upstream
//     cannot serialize, where upstream raises TypeError.
//   - The color parsers return no color where upstream raises an exception.
//   - The bytes of a stylesheet decode from UTF-8, UTF-16, windows-1252 and
//     x-user-defined only. Another encoding label counts as unknown.
package tinycss2

// Version is VERSION of tinycss2, the version that the package ports.
const Version = "1.5.1"
