// Copyright 2024 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/font.rs.

import (
	"strings"
	"unicode/utf8"

	"github.com/xo/okage"
)

// ParseFontFamilies is parse_font_families in svgtypes src/font.rs.
//
// Parses a list of font families and generic families from a string.
func ParseFontFamilies(text string) ([]FontFamily, error) {
	s := NewStream(text)
	fontFamilies, err := s.ParseFontFamilies()
	if err != nil {
		return nil, err
	}

	s.SkipSpaces()
	if !s.AtEnd() {
		return nil, errUnexpectedData(s.CalcCharPos())
	}

	return fontFamilies, nil
}

// FontFamilyKind is the case of the enum FontFamily in svgtypes src/font.rs
// (D27).
type FontFamilyKind uint8

// The cases of FontFamily.
const (
	// FontFamilySerif is FontFamily::Serif.
	//
	// A serif font.
	FontFamilySerif FontFamilyKind = iota
	// FontFamilySansSerif is FontFamily::SansSerif.
	//
	// A sans-serif font.
	FontFamilySansSerif
	// FontFamilyCursive is FontFamily::Cursive.
	//
	// A cursive font.
	FontFamilyCursive
	// FontFamilyFantasy is FontFamily::Fantasy.
	//
	// A fantasy font.
	FontFamilyFantasy
	// FontFamilyMonospace is FontFamily::Monospace.
	//
	// A monospace font.
	FontFamilyMonospace
	// FontFamilyNamed is FontFamily::Named.
	//
	// A custom named font.
	FontFamilyNamed
)

// FontFamily is FontFamily in svgtypes src/font.rs. The Rust enum has data in
// one case, so it is a struct with a kind (D27). Name holds the data of the
// case Named.
//
// A type of font family.
type FontFamily struct {
	Kind FontFamilyKind
	Name string
}

// String is the impl Display for FontFamily in svgtypes src/font.rs.
func (f FontFamily) String() string {
	switch f.Kind {
	case FontFamilyMonospace:
		return "monospace"
	case FontFamilySerif:
		return "serif"
	case FontFamilySansSerif:
		return "sans-serif"
	case FontFamilyCursive:
		return "cursive"
	case FontFamilyFantasy:
		return "fantasy"
	default:
		return "\"" + f.Name + "\""
	}
}

// ParseFontFamilies is Stream::parse_font_families in svgtypes src/font.rs.
func (s *Stream) ParseFontFamilies() ([]FontFamily, error) {
	var families []FontFamily

	for !s.AtEnd() {
		s.SkipSpaces()

		var family FontFamily
		ch, err := s.CurrByte()
		if err != nil {
			return nil, err
		}
		if ch == '\'' || ch == '"' {
			res, err := s.ParseQuotedString()
			if err != nil {
				return nil, err
			}
			family = FontFamily{Kind: FontFamilyNamed, Name: res}
		} else {
			var idents []string

			for !s.AtEnd() {
				c, _ := utf8.DecodeRuneInString(s.SliceTail())
				if c != ',' {
					ident, err := s.ParseIdent()
					if err != nil {
						return nil, err
					}
					idents = append(idents, ident)
					s.SkipSpaces()
				} else {
					break
				}
			}

			joined := strings.Join(idents, " ")

			// TODO: No CSS keyword must be matched as a family name...
			switch joined {
			case "serif":
				family = FontFamily{Kind: FontFamilySerif}
			case "sans-serif":
				family = FontFamily{Kind: FontFamilySansSerif}
			case "cursive":
				family = FontFamily{Kind: FontFamilyCursive}
			case "fantasy":
				family = FontFamily{Kind: FontFamilyFantasy}
			case "monospace":
				family = FontFamily{Kind: FontFamilyMonospace}
			default:
				family = FontFamily{Kind: FontFamilyNamed, Name: joined}
			}
		}

		families = append(families, family)

		if b, err := s.CurrByte(); err == nil {
			if b == ',' {
				s.Advance(1)
			} else {
				break
			}
		}
	}

	filtered := make([]FontFamily, 0, len(families))
	for _, f := range families {
		if f.Kind != FontFamilyNamed || f.Name != "" {
			filtered = append(filtered, f)
		}
	}

	return filtered, nil
}

// FontShorthand is FontShorthand in svgtypes src/font.rs. An Option<&str> of
// Rust is a string that is empty where Rust has None. A value that upstream
// sets is a keyword, so it is never empty.
//
// The values of a [`font` shorthand](https://www.w3.org/TR/css-fonts-3/#font-prop).
type FontShorthand struct {
	// The font style.
	FontStyle string
	// The font variant.
	FontVariant string
	// The font weight.
	FontWeight string
	// The font stretch.
	FontStretch string
	// The font size.
	FontSize string
	// The font family.
	FontFamily string
}

// ParseFontShorthand is FontShorthand::from_str in svgtypes src/font.rs.
//
// Parses the `font` shorthand from a string.
//
// [font]: https://www.w3.org/TR/css-fonts-3/#font-prop
func ParseFontShorthand(text string) (FontShorthand, error) {
	stream := NewStream(text)
	stream.SkipSpaces()

	prevPos := stream.Pos()

	var fontStyle, fontVariant, fontWeight, fontStretch string

loop:
	for range 4 {
		ident := stream.ConsumeASCIIIdent()

		switch ident {
		// TODO: Reuse actual parsers to prevent duplication.
		// We ignore normal because it's ambiguous to which it belongs and all
		// other attributes need to be reset anyway.
		case "normal":
		case "small-caps":
			fontVariant = ident
		case "italic", "oblique":
			fontStyle = ident
		case "bold", "bolder", "lighter", "100", "200", "300", "400", "500", "600",
			"700", "800", "900":
			fontWeight = ident
		case "ultra-condensed", "extra-condensed", "condensed", "semi-condensed",
			"semi-expanded", "expanded", "extra-expanded", "ultra-expanded":
			fontStretch = ident
		default:
			// Not one of the 4 properties, so we backtrack and then start
			// passing font size and family.
			stream = NewStream(text)
			stream.Advance(prevPos)
			break loop
		}

		stream.SkipSpaces()
		prevPos = stream.Pos()
	}

	prevPos = stream.Pos()
	c, err := stream.CurrByte()
	if err != nil {
		return FontShorthand{}, err
	}
	if isDigit(c) {
		// A font size such as '15pt'.
		if _, err := stream.ParseLength(); err != nil {
			return FontShorthand{}, err
		}
	} else {
		// A font size like 'xx-large'.
		size := stream.ConsumeASCIIIdent()

		switch size {
		case "xx-small", "x-small", "small", "medium", "large", "x-large", "xx-large",
			"larger", "smaller":
		default:
			return FontShorthand{}, errUnexpectedData(prevPos)
		}
	}

	fontSize := stream.SliceBack(prevPos)
	stream.SkipSpaces()

	c, err = stream.CurrByte()
	if err != nil {
		return FontShorthand{}, err
	}
	if c == '/' {
		// We should ignore line height since it has no effect in SVG.
		stream.Advance(1)
		stream.SkipSpaces()
		if _, err := stream.ParseLength(); err != nil {
			return FontShorthand{}, err
		}
		stream.SkipSpaces()
	}

	if stream.AtEnd() {
		return FontShorthand{}, okage.ErrSVGTypesUnexpectedEndOfStream
	}

	fontFamily := stream.SliceTail()

	return FontShorthand{
		FontStyle:   fontStyle,
		FontVariant: fontVariant,
		FontWeight:  fontWeight,
		FontStretch: fontStretch,
		FontSize:    fontSize,
		FontFamily:  fontFamily,
	}, nil
}
