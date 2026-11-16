// Copyright 2013-2020 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package tinycss2

import (
	"fmt"
	"strings"
)

// Serialize is serialize in tinycss2 tinycss2/serializer.py. It serializes
// nodes to CSS syntax.
//
// Use it for component values, instead of Node.Serialize on each node, as it
// takes care of corner cases such as ";" between declarations, and
// consecutive identifiers that would otherwise parse back as the same token.
func Serialize(nodes []Node) string {
	var b strings.Builder
	serializeTo(nodes, &b)
	return b.String()
}

// SerializeIdentifier is serialize_identifier in tinycss2
// tinycss2/serializer.py. It serializes any string as a CSS identifier: the
// result parses as an IdentToken whose Value is value.
//
// Upstream raises IndexError for the empty string. The port returns the empty
// string for it (D50).
func SerializeIdentifier(value string) string {
	if value == "-" {
		return `\-`
	}

	if strings.HasPrefix(value, "--") {
		return "--" + serializeName(value[2:])
	}

	var result strings.Builder
	if strings.HasPrefix(value, "-") {
		result.WriteByte('-')
		value = value[1:]
	}
	if value == "" {
		return result.String()
	}
	runes := []rune(value)
	c := runes[0]
	switch {
	case isASCIILetter(c) || c == '_' || c > 0x7F:
		result.WriteRune(c)
	case c == '\n':
		result.WriteString(`\A `)
	case c == '\r':
		result.WriteString(`\D `)
	case c == '\f':
		result.WriteString(`\C `)
	case c >= '0' && c <= '9':
		fmt.Fprintf(&result, "\\%X ", c)
	default:
		result.WriteByte('\\')
		result.WriteRune(c)
	}
	result.WriteString(serializeName(string(runes[1:])))
	return result.String()
}

// serializeName is serialize_name in tinycss2 tinycss2/serializer.py.
func serializeName(value string) string {
	var b strings.Builder
	for _, c := range value {
		switch {
		case isNameChar(c):
			b.WriteRune(c)
		case c == '\n':
			b.WriteString(`\A `)
		case c == '\r':
			b.WriteString(`\D `)
		case c == '\f':
			b.WriteString(`\C `)
		default:
			b.WriteByte('\\')
			b.WriteRune(c)
		}
	}
	return b.String()
}

// serializeStringValue is serialize_string_value in tinycss2
// tinycss2/serializer.py.
func serializeStringValue(value string) string {
	var b strings.Builder
	for _, c := range value {
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\A `)
		case '\r':
			b.WriteString(`\D `)
		case '\f':
			b.WriteString(`\C `)
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}

// serializeURL is serialize_url in tinycss2 tinycss2/serializer.py.
func serializeURL(value string) string {
	var b strings.Builder
	for _, c := range value {
		switch c {
		case '\'':
			b.WriteString(`\'`)
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case ' ':
			b.WriteString(`\ `)
		case '\t':
			b.WriteString(`\9 `)
		case '\n':
			b.WriteString(`\A `)
		case '\r':
			b.WriteString(`\D `)
		case '\f':
			b.WriteString(`\C `)
		case '(':
			b.WriteString(`\(`)
		case ')':
			b.WriteString(`\)`)
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}

// serializeTo is _serialize_to in tinycss2 tinycss2/serializer.py. It
// serializes a list of nodes to CSS syntax, and writes the chunks to b.
func serializeTo(nodes []Node, b *strings.Builder) {
	previousType := ""
	for _, node := range nodes {
		serializationType := node.Type()
		if t, ok := node.(*LiteralToken); ok {
			serializationType = t.Value
		}
		if badPairs[[2]string{previousType, serializationType}] {
			b.WriteString("/**/")
		} else if previousType == `\` {
			ws, ok := node.(*WhitespaceToken)
			if !ok || !strings.HasPrefix(ws.Value, "\n") {
				b.WriteByte('\n')
			}
		}
		node.serializeTo(b)
		if serializationType == TypeDeclaration {
			b.WriteByte(';')
		}
		previousType = serializationType
	}
}

// badPairs is BAD_PAIRS in tinycss2 tinycss2/serializer.py, from
// https://drafts.csswg.org/css-syntax/#serialization-tables.
var badPairs = makeBadPairs()

// makeBadPairs builds BAD_PAIRS.
func makeBadPairs() map[[2]string]bool {
	m := map[[2]string]bool{}
	add := func(as, bs []string) {
		for _, a := range as {
			for _, b := range bs {
				m[[2]string{a, b}] = true
			}
		}
	}
	add([]string{TypeIdent, TypeAtKeyword, TypeHash, TypeDimension, "#", "-", TypeNumber},
		[]string{TypeIdent, TypeFunction, TypeURL, TypeNumber, TypePercentage, TypeDimension, TypeUnicodeRange})
	add([]string{TypeIdent, TypeAtKeyword, TypeHash, TypeDimension},
		[]string{"-", "-->"})
	add([]string{"#", "-", TypeNumber, "@"},
		[]string{TypeIdent, TypeFunction, TypeURL})
	add([]string{TypeUnicodeRange, ".", "+"},
		[]string{TypeNumber, TypePercentage, TypeDimension})
	add([]string{"@"},
		[]string{TypeIdent, TypeFunction, TypeURL, TypeUnicodeRange, "-"})
	add([]string{TypeUnicodeRange},
		[]string{TypeIdent, TypeFunction, "?"})
	add([]string{"$", "*", "^", "~", "|"},
		[]string{"="})
	add([]string{TypeIdent}, []string{TypeParenthesesBlock})
	add([]string{"|"}, []string{"|"})
	add([]string{"/"}, []string{"*"})
	return m
}
