# D45. `svg` loads WOFF, WOFF2 and EOT fonts, and reads their names

Status: Decided, amends D7, D9 and D13, amended by D51 and D91.

The image viewer iv shows a preview of a font file. It used a package on
tdewolff/canvas for that, which adds 6.9 MB to its release build. iv can draw
the preview with `ShapeText` (D37), but two parts are missing in okage. The
first is a font in a web format: WOFF, WOFF2 or EOT. go-text reads WOFF, but
not WOFF2 or EOT. The second is a string of the name table, such as the
version or the sample text of the font. The maintainers want this work in
okage, not in iv, because other programs, such as zuzu, can use it too.

## The decision

1. `FontDatabase.LoadFontData` accepts a font in one of these formats:

   | Format | What okage does |
   | --- | --- |
   | TrueType, OpenType, or a collection of them | Loads the bytes as they are, as before |
   | WOFF 1.0 | Inflates each table with `compress/zlib` and writes an sfnt file |
   | WOFF2 | Decompresses the tables with brotli, reverses the transforms of the tables `glyf`, `loca` and `hmtx`, and writes an sfnt file or an sfnt collection |
   | EOT | Removes the header, and reverses the XOR of the font data when the header sets it |

   An sfnt file is a TrueType or OpenType font file. `FaceData` gives the
   sfnt bytes, not the bytes of the web format. An EOT file that uses
   MicroType Express compression does not load. Such files are rare, and
   D91 decides that okage does not read them.
2. `ConvertFontData(data []byte) ([]byte, error)` in `svg` gives the sfnt
   bytes of a font in one of these formats, for a program that needs the
   bytes and not a database. It returns sfnt bytes as they are.
3. `FontDatabase.FaceName(id FontID, nameID uint16) (string, bool)` returns
   the string of a name ID of the name table of a face. It takes the record
   in English (United States) first, then another Unicode record, then a
   Macintosh record in Roman.
4. `LoadFontsFS`, `LoadSystemFonts` and `LoadUserFonts` still load only the
   files that fontdb loads: `.ttf`, `.ttc`, `.otf` and `.otc`.
5. The root package adds two sentinel errors (D21). `ErrFontData` is for font
   data that is malformed. `ErrFontUnsupported` is for data in no format of
   this list, and for an EOT file with MicroType Express compression.
6. `internal/brotli` holds the brotli decoder of `github.com/dsnet/compress`
   at the commit `39efe44ab707`, copied into okage. It is not a dependency, so
   D6 does not apply to it. The copy has these changes:
   - It has only the decoder.
   - The helpers that it took from `internal` and `internal/errors` of
     `github.com/dsnet/compress` are in the package.
   - The dictionary of brotli, 122,784 bytes, is the file `dictionary.bin`,
     which `go:embed` reads. It is not a string in the source.
   - Its errors wrap `okage.ErrFontData`.
7. `internal/brotli` keeps the BSD-3-Clause license of
   `github.com/dsnet/compress`, in a `LICENSE` file in its folder (D13).
8. `internal/webfont`, now `webfont` (D51), holds the conversion of WOFF,
   WOFF2 and EOT. It ports no upstream. It follows the WOFF 1.0 and WOFF2
   recommendations of the W3C and the EOT submission to the W3C.

## What stays in iv

iv draws the preview itself: the lines of text, their sizes, the margins and
the colors. It reads the family, the style, the version and the sample text
with `FaceInfo` and `FaceName`, and it shapes the text with `ShapeText`.

## When upstream changes

`internal/brotli` has no upstream that okage follows. If a fault is found in
the decoder, fix it in okage. If `github.com/dsnet/compress` fixes the same
fault, compare the fix with the fix of okage.

If fontdb or go-text adds a reader of WOFF2 or EOT, follow these steps:

1. Compare its result with the result of `webfont` (D51) on the fonts of
   the tests.
2. If both give the same faces, a maintainer decides whether okage keeps its
   own code or uses the other one. Record the result here or in a new
   decision.

## Why

The standard library has no brotli decoder. Of the two decoders in pure Go,
`github.com/dsnet/compress/brotli` adds about 460 KB to a program, and
`github.com/andybalholm/brotli` adds about 2.5 MB. Both gave the same bytes on
the brotli data of 16 WOFF2 files. The module `github.com/dsnet/compress` has
requirements that its brotli package does not use, and its `go.mod` names Go
1.9, so they all go into the `go.sum` of okage. A copy avoids that. A copy also
keeps the dictionary as 120 KB of binary data, not as a 748 KB string in the
source.
