// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/funciri.rs.

import (
	"strings"
	"unicode"

	"github.com/xo/okage"
)

// IRI is IRI in svgtypes src/funciri.rs.
//
// Representation of the [`<IRI>`] type.
//
// [`<IRI>`]: https://www.w3.org/TR/SVG11/types.html#DataTypeIRI
type IRI string

// ParseIRI is IRI::from_str in svgtypes src/funciri.rs.
//
// Parsers a `IRI` from a string.
//
// By the SVG spec, the ID must contain only [Name] characters,
// but since no one fallows this it will parse any characters.
//
// [Name]: https://www.w3.org/TR/xml/#NT-Name
func ParseIRI(text string) (IRI, error) {
	s := NewStream(text)
	link, err := s.ParseIRI()
	if err != nil {
		return "", err
	}
	s.SkipSpaces()
	if !s.AtEnd() {
		return "", errUnexpectedData(s.CalcCharPos())
	}

	return IRI(link), nil
}

// FuncIRI is FuncIRI in svgtypes src/funciri.rs.
//
// Representation of the [`<FuncIRI>`] type.
//
// [`<FuncIRI>`]: https://www.w3.org/TR/SVG11/types.html#DataTypeFuncIRI
type FuncIRI string

// ParseFuncIRI is FuncIRI::from_str in svgtypes src/funciri.rs.
//
// Parsers a `FuncIRI` from a string.
//
// By the SVG spec, the ID must contain only [Name] characters,
// but since no one fallows this it will parse any characters.
//
// [Name]: https://www.w3.org/TR/xml/#NT-Name
func ParseFuncIRI(text string) (FuncIRI, error) {
	s := NewStream(text)
	link, err := s.ParseFuncIRI()
	if err != nil {
		return "", err
	}
	s.SkipSpaces()
	if !s.AtEnd() {
		return "", errUnexpectedData(s.CalcCharPos())
	}

	return FuncIRI(link), nil
}

// ParseIRI is Stream::parse_iri in svgtypes src/funciri.rs.
func (s *Stream) ParseIRI() (string, error) {
	s.SkipSpaces()
	if err := s.ConsumeByte('#'); err != nil {
		return "", err
	}
	link := s.ConsumeBytes(func(_ *Stream, c byte) bool { return c != ' ' })
	if link == "" {
		return "", okage.ErrSVGTypesInvalidValue
	}
	return link, nil
}

// ParseFuncIRI is Stream::parse_func_iri in svgtypes src/funciri.rs.
func (s *Stream) ParseFuncIRI() (string, error) {
	s.SkipSpaces()
	if err := s.ConsumeString("url("); err != nil {
		return "", err
	}
	s.SkipSpaces()

	var quote byte
	hasQuote := false
	if c, err := s.CurrByte(); err == nil && (c == '\'' || c == '"') {
		quote, hasQuote = c, true
	}
	if hasQuote {
		s.Advance(1)
		s.SkipSpaces()
	}
	if err := s.ConsumeByte('#'); err != nil {
		return "", err
	}
	var link string
	if hasQuote {
		// str::trim_end of Rust trims the chars with the property
		// White_Space, which is unicode.IsSpace in Go.
		link = strings.TrimRightFunc(s.ConsumeBytes(func(_ *Stream, c byte) bool { return c != quote }), unicode.IsSpace)
	} else {
		link = s.ConsumeBytes(func(_ *Stream, c byte) bool { return c != ' ' && c != ')' })
	}
	if link == "" {
		return "", okage.ErrSVGTypesInvalidValue
	}
	// Non-paired quotes is an error.
	if strings.ContainsRune(link, '\'') || strings.ContainsRune(link, '"') {
		return "", okage.ErrSVGTypesInvalidValue
	}
	s.SkipSpaces()
	if hasQuote {
		if err := s.ConsumeByte(quote); err != nil {
			return "", err
		}
		s.SkipSpaces()
	}
	if err := s.ConsumeByte(')'); err != nil {
		return "", err
	}
	return link, nil
}
