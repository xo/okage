// Copyright 2013-2020 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package tinycss2

// This file ports no upstream file. It holds the options of the functions
// that parse, which are keyword arguments in upstream (D20), the limit of the
// nesting depth (D50), and the small helpers of webencodings that tinycss2
// imports.

// DefaultMaxDepth is the default of WithMaxDepth: the most blocks and
// functions that the tokenizer nests in each other.
const DefaultMaxDepth = 256

// Input is the type of the input of the functions that parse. Upstream takes
// a string or an iterable of component values.
type Input interface {
	string | []Node
}

// Option is an option of the functions that parse. It changes the
// configuration of one call.
type Option func(*options)

// options is the configuration that the options set.
type options struct {
	// skipComments is the argument skip_comments of upstream.
	skipComments bool
	// skipWhitespace is the argument skip_whitespace of upstream.
	skipWhitespace bool
	// maxDepth is the most blocks and functions that the tokenizer nests.
	maxDepth int
}

// WithSkipComments is the argument skip_comments of upstream. If v is true
// and the input is a string, the tokenizer drops every comment. If v is true,
// the functions that parse a list also drop the comments at the top level of
// the list.
func WithSkipComments(v bool) Option {
	return func(o *options) {
		o.skipComments = v
	}
}

// WithSkipWhitespace is the argument skip_whitespace of upstream. If v is
// true, the functions that parse a list drop the whitespace at the top level
// of the list. The whitespace in the value of a declaration, and in the
// prelude and the content of a rule, stays.
func WithSkipWhitespace(v bool) Option {
	return func(o *options) {
		o.skipWhitespace = v
	}
}

// WithMaxDepth sets the most blocks and functions that the tokenizer nests
// in each other. Upstream has no such limit. At the limit, the tokenizer
// adds a ParseError of the kind "too-deep" in place of the block or the
// function that is too deep, and drops its content. A value of 0 or less
// sets DefaultMaxDepth.
func WithMaxDepth(n int) Option {
	return func(o *options) {
		o.maxDepth = n
	}
}

// newOptions applies opts to a new configuration and returns it.
func newOptions(opts []Option) options {
	o := options{maxDepth: DefaultMaxDepth}
	for _, opt := range opts {
		opt(&o)
	}
	if o.maxDepth <= 0 {
		o.maxDepth = DefaultMaxDepth
	}
	return o
}

// asciiLower is ascii_lower of webencodings. It maps only the ASCII letters
// A to Z to lower case.
func asciiLower(s string) string {
	for i := range len(s) {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if b[j] >= 'A' && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// ASCIILower is ascii_lower of webencodings, which tinycss2 and cssselect2
// use. It maps only the ASCII letters A to Z to lower case.
func ASCIILower(s string) string {
	return asciiLower(s)
}

// isASCIILetter reports whether c is an ASCII letter.
func isASCIILetter(c rune) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// isDigit reports whether c is an ASCII digit.
func isDigit(c rune) bool {
	return c >= '0' && c <= '9'
}

// isHexDigit reports whether c is an ASCII hexadecimal digit.
func isHexDigit(c rune) bool {
	return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// isNameChar reports whether c is in the set of upstream
// 'abcdefghijklmnopqrstuvwxyz-_0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ', or is
// not ASCII.
func isNameChar(c rune) bool {
	return isASCIILetter(c) || isDigit(c) || c == '-' || c == '_' || c > 0x7F
}
