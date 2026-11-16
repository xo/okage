// Copyright 2013-2020 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package tinycss2

import (
	"strconv"
	"strings"
)

// ParseNth is parse_nth in tinycss2 tinycss2/nth.py. It parses <An+B>, as
// found in :nth-child() and the related pseudo-classes of selectors.
//
// tinycss2 holds no full selector parser, but it holds this bit of syntax, as
// it is tricky to define on top of a CSS tokenizer.
//
// It returns a and b, and false if the input is invalid. The tokenizer drops
// the comments of a string input.
func ParseNth[T Input](input T) (a, b int64, ok bool) {
	tokens := toTokenIterator(input, newOptions([]Option{WithSkipComments(true)}))
	token := nextSignificant(tokens)
	if token == nil {
		return 0, 0, false
	}
	switch t := token.(type) {
	case *NumberToken:
		if t.IsInteger {
			return parseEnd(tokens, 0, t.IntValue)
		}
	case *DimensionToken:
		if t.IsInteger {
			unit := t.LowerUnit
			if unit == "n" {
				return parseB(tokens, t.IntValue)
			} else if unit == "n-" {
				return parseSignlessB(tokens, t.IntValue, -1)
			} else if digits, ok := matchNDashDigits(unit); ok {
				return parseEnd(tokens, t.IntValue, digits)
			}
		}
	case *IdentToken:
		ident := t.LowerValue
		switch {
		case ident == "even":
			return parseEnd(tokens, 2, 0)
		case ident == "odd":
			return parseEnd(tokens, 2, 1)
		case ident == "n":
			return parseB(tokens, 1)
		case ident == "-n":
			return parseB(tokens, -1)
		case ident == "n-":
			return parseSignlessB(tokens, 1, -1)
		case ident == "-n-":
			return parseSignlessB(tokens, -1, -1)
		case strings.HasPrefix(ident, "-"):
			if digits, ok := matchNDashDigits(ident[1:]); ok {
				return parseEnd(tokens, -1, digits)
			}
		default:
			if digits, ok := matchNDashDigits(ident); ok {
				return parseEnd(tokens, 1, digits)
			}
		}
	case *LiteralToken:
		if t.Value == "+" {
			// Whitespace after an initial '+' is invalid, so the next token
			// is read without skipping it. nil is used when the iterator is
			// exhausted.
			if ident, ok := tokens.next().(*IdentToken); ok {
				switch v := ident.LowerValue; v {
				case "n":
					return parseB(tokens, 1)
				case "n-":
					return parseSignlessB(tokens, 1, -1)
				default:
					if digits, ok := matchNDashDigits(v); ok {
						return parseEnd(tokens, 1, digits)
					}
				}
			}
		}
	}
	return 0, 0, false
}

// SerializeNth is serialize_nth in tinycss2 tinycss2/nth.py. It serializes
// <An+B> as an "an+b" string.
func SerializeNth(a, b int64) string {
	var an string
	switch {
	case a == 1:
		an = "n"
	case a == -1:
		an = "-n"
	case a != 0:
		an = strconv.FormatInt(a, 10) + "n"
	}
	var bs string
	switch {
	case a == 0:
		bs = strconv.FormatInt(b, 10)
	case b > 0:
		bs = "+" + strconv.FormatInt(b, 10)
	case b < 0:
		bs = strconv.FormatInt(b, 10)
	}
	return an + bs
}

// parseB is parse_b in tinycss2 tinycss2/nth.py.
func parseB(tokens *tokenIter, a int64) (int64, int64, bool) {
	token := nextSignificant(tokens)
	switch {
	case token == nil:
		return a, 0, true
	case IsLiteral(token, "+"):
		return parseSignlessB(tokens, a, 1)
	case IsLiteral(token, "-"):
		return parseSignlessB(tokens, a, -1)
	}
	if t, ok := token.(*NumberToken); ok && t.IsInteger &&
		(t.Representation[0] == '-' || t.Representation[0] == '+') {
		return parseEnd(tokens, a, t.IntValue)
	}
	return 0, 0, false
}

// parseSignlessB is parse_signless_b in tinycss2 tinycss2/nth.py.
func parseSignlessB(tokens *tokenIter, a, bSign int64) (int64, int64, bool) {
	token := nextSignificant(tokens)
	if t, ok := token.(*NumberToken); ok && t.IsInteger &&
		t.Representation[0] != '-' && t.Representation[0] != '+' {
		return parseEnd(tokens, a, bSign*t.IntValue)
	}
	return 0, 0, false
}

// parseEnd is parse_end in tinycss2 tinycss2/nth.py.
func parseEnd(tokens *tokenIter, a, b int64) (int64, int64, bool) {
	if nextSignificant(tokens) == nil {
		return a, b, true
	}
	return 0, 0, false
}

// matchNDashDigits is N_DASH_DIGITS_RE.match in tinycss2 tinycss2/nth.py,
// the regular expression `^n(-[0-9]+)$`. It returns the integer of the group.
// The $ of Python also matches before a newline at the end of the string.
func matchNDashDigits(s string) (int64, bool) {
	s = strings.TrimSuffix(s, "\n")
	if len(s) < 3 || s[0] != 'n' || s[1] != '-' {
		return 0, false
	}
	for i := 2; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	return parseIntSaturating(s[1:]), true
}
