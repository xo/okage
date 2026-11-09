// Copyright 2024 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import (
	"slices"
	"testing"

	"github.com/xo/okage"
)

// The tests of svgtypes src/font.rs.

func named(text string) FontFamily {
	return FontFamily{Kind: FontFamilyNamed, Name: text}
}

var (
	serif     = FontFamily{Kind: FontFamilySerif}
	sansSerif = FontFamily{Kind: FontFamilySansSerif}
	fantasy   = FontFamily{Kind: FontFamilyFantasy}
	monospace = FontFamily{Kind: FontFamilyMonospace}
	cursive   = FontFamily{Kind: FontFamilyCursive}
)

func TestParseFontFamilies(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []FontFamily
	}{
		{"font_family_1", "Times New Roman", []FontFamily{named("Times New Roman")}},
		{"font_family_2", "serif", []FontFamily{serif}},
		{"font_family_3", "sans-serif", []FontFamily{sansSerif}},
		{"font_family_4", "cursive", []FontFamily{cursive}},
		{"font_family_5", "fantasy", []FontFamily{fantasy}},
		{"font_family_6", "monospace", []FontFamily{monospace}},
		{"font_family_7", "'Times New Roman'", []FontFamily{named("Times New Roman")}},
		{"font_family_8", "'Times New Roman', sans-serif", []FontFamily{named("Times New Roman"), sansSerif}},
		{"font_family_9", "'Times New Roman', sans-serif", []FontFamily{named("Times New Roman"), sansSerif}},
		{"font_family_10", "Arial, sans-serif, 'fantasy'", []FontFamily{named("Arial"), sansSerif, named("fantasy")}},
		{"font_family_11", "    Arial  , monospace  , 'fantasy'", []FontFamily{named("Arial"), monospace, named("fantasy")}},
		{"font_family_12", "Times    New Roman", []FontFamily{named("Times New Roman")}},
		{
			"font_family_13", "\"Times New Roman\", sans-serif, sans-serif, \"Arial\"",
			[]FontFamily{named("Times New Roman"), sansSerif, sansSerif, named("Arial")},
		},
		{"font_family_14", "Times New Roman,,,Arial", []FontFamily{named("Times New Roman"), named("Arial")}},
		{
			"font_family_15", "简体中文,sans-serif  , ,\"日本語フォント\",Arial", //nolint:gosmopolitan // the text of the upstream test
			[]FontFamily{named("简体中文"), sansSerif, named("日本語フォント"), named("Arial")}, //nolint:gosmopolitan // the text of the upstream test
		},
		{"font_family_16", "", []FontFamily{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFontFamilies(tt.text)
			if err != nil || !slices.Equal(got, tt.want) {
				t.Errorf("ParseFontFamilies(%q) = %v, %v, want %v", tt.text, got, err, tt.want)
			}
		})
	}
}

func TestParseFontFamiliesErr(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"font_family_err_1", "Red/Black, sans-serif", "invalid ident"},
		{"font_family_err_2", "\"Lucida\" Grande, sans-serif", "unexpected data at position 10"},
		{"font_family_err_3", "Ahem!, sans-serif", "invalid ident"},
		{"font_family_err_4", "test@foo, sans-serif", "invalid ident"},
		{"font_family_err_5", "#POUND, sans-serif", "invalid ident"},
		{"font_family_err_6", "Hawaii 5-0, sans-serif", "invalid ident"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFontFamilies(tt.text)
			errText(t, err, tt.want)
		})
	}
}

func TestParseFontShorthand(t *testing.T) {
	tests := []struct {
		name string
		text string
		want FontShorthand
	}{
		{"font_shorthand_1", "12pt/14pt sans-serif", FontShorthand{"", "", "", "", "12pt", "sans-serif"}},
		{"font_shorthand_2", "80% sans-serif", FontShorthand{"", "", "", "", "80%", "sans-serif"}},
		{"font_shorthand_3", "bold italic large Palatino, serif", FontShorthand{"italic", "", "bold", "", "large", "Palatino, serif"}},
		{"font_shorthand_4", "x-large/110% \"new century schoolbook\", serif", FontShorthand{"", "", "", "", "x-large", "\"new century schoolbook\", serif"}},
		{"font_shorthand_5", "normal small-caps 120%/120% fantasy", FontShorthand{"", "small-caps", "", "", "120%", "fantasy"}},
		{"font_shorthand_6", "condensed oblique 12pt \"Helvetica Neue\", serif", FontShorthand{"oblique", "", "", "condensed", "12pt", "\"Helvetica Neue\", serif"}},
		{"font_shorthand_7", "italic 500 2em sans-serif, 'Noto Sans'", FontShorthand{"italic", "", "500", "", "2em", "sans-serif, 'Noto Sans'"}},
		{"font_shorthand_8", "xx-large 'Noto Sans'", FontShorthand{"", "", "", "", "xx-large", "'Noto Sans'"}},
		{"font_shorthand_9", "small-caps normal normal italic xx-small Times", FontShorthand{"italic", "small-caps", "", "", "xx-small", "Times"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFontShorthand(tt.text)
			if err != nil || got != tt.want {
				t.Errorf("ParseFontShorthand(%q) = %+v, %v, want %+v", tt.text, got, err, tt.want)
			}
		})
	}
}

func TestParseFontShorthandErr(t *testing.T) {
	tests := []struct {
		name string
		text string
		want error
	}{
		{"font_shorthand_err_1", "", okage.ErrSVGTypesUnexpectedEndOfStream},
		{"font_shorthand_err_2", "Noto Sans", errUnexpectedData(0)},
		{"font_shorthand_err_3", "12pt  ", okage.ErrSVGTypesUnexpectedEndOfStream},
		{"font_shorthand_err_4", "something 12pt 'Noto Sans'", errUnexpectedData(0)},
		{"font_shorthand_err_5", "'Noto Sans' 13pt", errUnexpectedData(0)},
		{"font_shorthand_err_6", "small-caps normal normal normal italic xx-large Times", errUnexpectedData(32)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFontShorthand(tt.text)
			if err == nil || err.Error() != tt.want.Error() {
				t.Errorf("ParseFontShorthand(%q) error %v, want %v", tt.text, err, tt.want)
			}
		})
	}
}
