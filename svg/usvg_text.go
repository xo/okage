// Copyright 2024 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"log/slog"
	"slices"
	"strings"
)

// This file ports usvg src/text/mod.rs.
//
// Port: the font database of usvg_text_fontdb.go takes the place of the
// crate fontdb, and go-text takes the place of skrifa (D9).

// opsz is OPSZ in usvg src/text/mod.rs. It is the optical sizing variation
// axis tag.
var opsz = [4]byte{'o', 'p', 's', 'z'}

// GlyphID is GlyphId in usvg src/text/mod.rs. It is the ID of a glyph within
// a font.
//
// Port: the impl From<GlyphId> for skrifa::GlyphId has no port. go-text
// takes a glyph as a font.GID, a uint32, which is the conversion.
type GlyphID uint32

// FontSelectionFn is FontSelectionFn in usvg src/text/mod.rs. It is a
// shorthand for [FontResolver]'s font selection function.
//
// This function receives a font specification (families + a style, weight,
// stretch triple) and a font database and should return the ID of the font
// that shall be used (if any).
//
// In the basic case, the function will search the existing fonts in the
// database to find a good match, e.g. via
// [`Database::query`](fontdb::Database::query). This is what the [default
// implementation](FontResolver::default_font_selector) does.
//
// Users with more complex requirements can mutate the database to load
// additional fonts dynamically. It is important that the database is only
// mutated additively. Removing fonts or replacing the entire database will
// break things.
//
// Port: upstream passes `&mut Arc<Database>`, and a function calls
// Arc::make_mut before it changes the database. Go passes the copy of the
// database that the conversion of one tree owns, so a function can load a
// font into it with FontDatabase.LoadFontData. The font is then in the
// database of the tree, Tree.FontDB, and not in the database of the options.
type FontSelectionFn func(font *Font, fontdb *FontDatabase) (FontID, bool)

// FallbackSelectionFn is FallbackSelectionFn in usvg src/text/mod.rs. It is a
// shorthand for [FontResolver]'s fallback selection function.
//
// This function receives a specific character, a list of already used fonts,
// and a font database. It should return the ID of a font that
//   - is not any of the already used fonts
//   - is as close as possible to the first already used font (if any)
//   - supports the given character
//
// The function can search the existing database, but can also load additional
// fonts dynamically. See the documentation of [`FontSelectionFn`] for more
// details.
type FallbackSelectionFn func(c rune, excludeFonts []FontID, fontdb *FontDatabase) (FontID, bool)

// FontResolver is FontResolver in usvg src/text/mod.rs. It is a font resolver
// for `<text>` elements.
//
// This type can be useful if you want to have an alternative font handling to
// the default one. By default, only fonts specified upfront in
// [`Options::fontdb`](crate::Options::fontdb) will be used. This type allows
// you to load additional fonts on-demand and customize the font selection
// process.
//
// Port: a nil function is the default function of upstream,
// DefaultFontSelector or DefaultFallbackSelector, with the logger of the
// options. The zero value is the impl Default of upstream.
type FontResolver struct {
	// Resolver function that will be used when selecting a specific font
	// for a generic [`Font`] specification.
	SelectFont FontSelectionFn

	// Resolver function that will be used when selecting a fallback font for a
	// character.
	SelectFallback FallbackSelectionFn
}

// selectFont calls the field SelectFont of r, or the default selector with
// the logger of opt when it is nil.
func (r *FontResolver) selectFont(font *Font, fontdb *FontDatabase, opt *options) (FontID, bool) {
	if r.SelectFont != nil {
		return r.SelectFont(font, fontdb)
	}
	return DefaultFontSelector(opt.logger)(font, fontdb)
}

// selectFallback calls the field SelectFallback of r, or the default
// selector with the logger of opt when it is nil.
func (r *FontResolver) selectFallback(c rune, excludeFonts []FontID, fontdb *FontDatabase, opt *options) (FontID, bool) {
	if r.SelectFallback != nil {
		return r.SelectFallback(c, excludeFonts, fontdb)
	}
	return DefaultFallbackSelector(opt.logger)(c, excludeFonts, fontdb)
}

// DefaultFontSelector is FontResolver::default_font_selector in usvg
// src/text/mod.rs. It creates a default font selection resolver.
//
// The default implementation forwards to
// [`query`](fontdb::Database::query) on the font database specified in the
// [`Options`](crate::Options).
//
// It logs its warning to logger, which can be nil (D20).
func DefaultFontSelector(logger *slog.Logger) FontSelectionFn {
	return func(font *Font, fontdb *FontDatabase) (FontID, bool) {
		// Port: fontdb::Family has the cases of FontFamily, so the names
		// are the families themselves.
		nameList := make([]FontFamily, 0, len(font.families)+1)
		nameList = append(nameList, font.families...)

		// Use the default font as fallback.
		nameList = append(nameList, FontFamily{Kind: FontFamilySerif})

		query := FontQuery{
			Families: nameList,
			Weight:   font.weight,
			Stretch:  font.stretch,
			Style:    font.style,
		}

		id, ok := fontdb.Query(query)
		if !ok && logger != nil {
			names := make([]string, len(font.families))
			for i, f := range font.families {
				names[i] = f.String()
			}
			logger.Warn("No match for '" + strings.Join(names, ", ") + "' font-family.")
		}

		return id, ok
	}
}

// DefaultFallbackSelector is FontResolver::default_fallback_selector in usvg
// src/text/mod.rs. It creates a default font fallback selection resolver.
//
// The default implementation searches through the entire `fontdb`
// to find a font that has the correct style and supports the character.
//
// It logs its warning to logger, which can be nil (D20).
func DefaultFallbackSelector(logger *slog.Logger) FallbackSelectionFn {
	return func(c rune, excludeFonts []FontID, fontdb *FontDatabase) (FontID, bool) {
		baseFontID := excludeFonts[0]
		baseFace, ok := fontdb.Face(baseFontID)
		if !ok {
			return 0, false
		}

		// Iterate over fonts and check if any of them support the specified char.
		for face := range fontdb.Faces() {
			// Ignore fonts, that were used for shaping already.
			if slices.Contains(excludeFonts, face.ID) {
				continue
			}

			// Check that the new face has the same style.
			if baseFace.Style != face.Style &&
				baseFace.Weight != face.Weight &&
				baseFace.Stretch != face.Stretch {
				continue
			}

			if !databaseHasChar(fontdb, face.ID, c) {
				continue
			}

			baseFamily := englishFamily(baseFace)
			newFamily := englishFamily(face)

			if logger != nil {
				logger.Warn("Fallback from " + baseFamily + " to " + newFamily + ".")
			}
			return face.ID, true
		}

		return 0, false
	}
}

// englishFamily is the family of face in English (United States), or its
// first family, as FontResolver::default_fallback_selector in usvg
// src/text/mod.rs finds it.
func englishFamily(face *FaceInfo) string {
	for _, f := range face.Families {
		if f.Language == FontLanguageEnglishUnitedStates {
			return f.Name
		}
	}
	return face.Families[0].Name
}

// String is the impl Debug for FontResolver in usvg src/text/mod.rs.
func (r FontResolver) String() string {
	return "FontResolver { .. }"
}

// convertText is convert in usvg src/text/mod.rs. It converts a text into its
// paths. This is done in two steps:
//  1. We convert the text into glyphs and position them according to the rules specified
//     in the SVG specification. While doing so, we also calculate the text bbox (which
//     is not based on the outlines of a glyph, but instead the glyph metrics as well
//     as decoration spans).
//  2. We convert all of the positioned glyphs into outlines.
func convertText(text *Text, resolver *FontResolver, cache *converterCache) bool {
	textFragments, bbox, ok := layoutText(text, resolver, cache.fontdb, cache.shapingFonts, cache.opt)
	if !ok {
		return false
	}
	text.layouted = textFragments
	text.boundingBox = bbox.ToRect()
	absBBox, ok := bbox.Transform(text.absTransform)
	if !ok {
		return false
	}
	text.absBoundingBox = absBBox.ToRect()

	group, strokeBBox, ok := flatten(text, cache)
	if !ok {
		return false
	}
	text.flattened = group
	text.strokeBoundingBox = strokeBBox.ToRect()
	absStrokeBBox, ok := strokeBBox.Transform(text.absTransform)
	if !ok {
		return false
	}
	text.absStrokeBoundingBox = absStrokeBBox.ToRect()

	return true
}
