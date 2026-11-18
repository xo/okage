# D57. `svg` gives the faces, the clusters and the color glyphs of text

Status: Decided, amends D37.

ivy lays out the text of its documents with a port of WeasyPrint. It shapes
the text with `typesetting` itself, and it takes its fonts from a
`FontDatabase` of `svg`. To do that, ivy parses each face a second time,
and it reads the cmap of a font again for each lookup of a fallback. It
also keeps its own copies of the metrics of a face and of the map from a
CSS weight to the axes of a variable font. ivy cannot draw a color glyph of
the tables COLR and SVG, which `svg` draws only inside its renderer. On
2026-11-18 the maintainers asked for an API in `svg` that gives these parts
to a program.

## The decision

1. `FontDatabase.FaceFont(id)` returns the `*font.Font` of `typesetting` that
   the database parses once for each font file and keeps. The layout of text
   of `svg` reads the same font. A `*font.Font` is safe for concurrent use.
2. `FontDatabase.NewFace(id, variations)` returns a new `*font.Face` of that
   font. It keeps only the axes that the font has, and it limits each value
   to the range of its axis, as the font instance of the layout does. A
   `*font.Face` is not safe for concurrent use, so each call returns a new
   face, and the caller keeps it.
3. `FontDatabase.NominalGlyph(id, c)` looks up a character in the cmap that
   the database reads once for each font file, from the tables cmap and OS/2
   only. It does not parse the whole face. `DefaultFallbackSelector` looks up
   each character with it. The selector read the same cmap before, through a
   private method, so its result does not change.
4. `FontDatabase.FaceMetrics(id, variations)` returns a `FontMetrics` in font
   units: the units per em, the ascent, the descent, the x-height, the
   position and the thickness of the underline, the position of the line
   through, and the offsets of a subscript and a superscript. These are the
   metrics of the resolved font of the layout, which follow the rules of
   skrifa.
5. `AutoFontVariations(variations, weight, stretch, style)` returns the axes
   `wght`, `wdth`, `ital` and `slnt` for a CSS weight, stretch and style, as
   usvg maps them for a `<text>` element. It adds only an axis that
   `variations` does not set, and it does not change its argument. It is the
   function `autoFontVariations` of D37.
6. `TextStyle` of `ShapeText` has these new fields. The zero value of each
   field gives the result of D37, so a program that uses D37 gets the same
   runs.

   | Field | What it gives `ShapeText` |
   | --- | --- |
   | `Stretch` | The CSS font-stretch, as a percentage. 0 is 100. The query of the font takes the keyword that is nearest, and the axis `wdth` takes the percentage of that keyword, as usvg does |
   | `Variations` | The CSS font-variation-settings. `ShapeText` adds the axes of `AutoFontVariations` |
   | `Features` | The CSS font-feature-settings, as a list of `FontFeature`: a tag and a value. They apply to the whole run, after the features `smcp` and `kern` of usvg |
   | `Language` | A BCP 47 tag, which the shaper uses to select the glyphs of a language |
   | `LetterSpacing`, `WordSpacing` | The letter spacing and the word spacing of usvg, in user units |

7. `TextRun.Clusters()` returns the clusters of the run in the order of the
   text. A `TextCluster` holds its range of bytes in the text, its position
   on the baseline, its advance with the spacing, its face, and its glyphs
   as `PositionedGlyph` values.
8. `FontDatabase.ColorGlyph(id, glyph, style)` returns a glyph of the table
   COLR, version 0 or 1, or of the table SVG, as a `*Tree`. The tree is in em
   units, y grows down, and the origin of the glyph is at (0, 0) on the
   baseline. `ColorGlyphStyle` holds the variations, the index of the palette
   of CPAL, and the color of the text. A glyph of COLR takes that color for
   the palette entry 0xFFFF, and for an entry that the palette does not
   have. A glyph of COLR comes first when a font has both tables, as in the
   renderer. A bitmap glyph of CBDT or sbix gives false, and the program
   draws it itself.
9. `TextStyle` and `ColorGlyphStyle` are inputs of one call, not options, as
   `TextStyle` of D37 is (D20).
10. The code of items 1 to 4 is in `svg/font_face.go`, the code of item 8 is
    in `svg/color_glyph.go`, and the code of items 5 to 7 is in
    `svg/text_run.go`. These files port no upstream file. `typesetting` does
    not change.

## The changes of the ported files

The changes add inputs that the parser does not give, so a `<text>` element
of SVG data draws as before:

1. `shapeText` and `shapeTextWithFont` in `svg/usvg_text_layout.go` take the
   features and the language of a span. usvg sets only the features `smcp`
   and `kern`, and no language. `TextSpan` holds the two fields, and the
   parser leaves them empty.
2. `layoutText` keeps the clusters of the layout in two fields of `Text`
   when `ShapeText` asks for them.
3. `colr` in `svg/usvg_text_cache.go` takes a palette and the color of the
   text. The glyph painter in `svg/usvg_text_colr.go` takes that palette, or
   the first palette when the font does not have it. usvg always takes the
   palette 0 and black, and the font cache of the renderer gives those.

Each change has a comment that starts with "Port:" and names D57. When a
resvg commit changes one of these functions, port the commit and keep the
inputs of D57. If upstream adds the same input, such as a language for the
shaper, remove the code of D57 for that input.

## Why

A program that shapes text with `typesetting` and takes its fonts from a
`FontDatabase` needs the parsed faces, the cmaps and the metrics that the
database already holds. Without this API, it parses each face again and
keeps a copy of rules that `svg` already has, such as the axes of a variable
font. These copies can fall out of step with `svg`, and the same text then
gets other glyphs in the program and in `svg`.

`ColorGlyph` gives a color glyph as a tree, because the renderer already
draws a tree. A program draws the glyph with `Render` onto a pixmap, at the
size and the place of the glyph.

`ShapeText` still makes a new font cache for each call, and the shaper of
each font with it. A program that shapes each text of a document can keep
its own shaper, and use items 1 to 5 for its fonts.
