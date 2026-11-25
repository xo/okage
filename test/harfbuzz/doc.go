// Package harfbuzz holds the oracle of typesetting/harfbuzz, the shaper of
// the copy of go-text in okage (D52, D73). It has no code of its own.
//
// uharfbuzz, the Python binding of HarfBuzz, shapes each text with
// shape.py, and the tests compare its glyphs, clusters, advances and
// offsets with the output of harfbuzz. The tests need uharfbuzz with
// HarfBuzz 15.2.0, the version that D97 follows, from D73:
//
//   - The tests of D73 build fonts from Roboto-Regular.ttf of
//     typesetting/font/testdata and from the fonts of perf_reference, with
//     tables GSUB, GPOS, morx, kerx or kern that they write by hand. Each
//     font makes the buffer run out of operations or grow past its limit
//     of length, makes a subtable apply, or makes a table of too many
//     subtables, at a place where harfbuzz differed from HarfBuzz. Some
//     tests also compare the flag unsafe-to-break of each glyph.
//   - TestPerfReference shapes the texts of perf_reference in its fonts, a
//     line at a time, from typesetting/testdata. By default it shapes the
//     first 200 lines of each text. With the flag -harfbuzz.all, it shapes
//     all 413,399 lines, which takes about a minute.
//   - TestScale and TestKerxFormat6LongValues shape at scales other than
//     the units per em of the font, and with long kerning values of kerx
//     format 6 (D90).
//   - TestUnsafeToConcat and TestRuleSetUnsafeToConcat compare the flags
//     unsafe-to-break and unsafe-to-concat of each glyph (D93).
//   - The tests of D97 compare the changes of HarfBuzz 15.2.0 that D97
//     ports, such as a global request of a feature after a request on a
//     range, the kerning across the line of kern, the USE shaper at a
//     space, and the dotted circle of a prohibited vowel sequence with a
//     ZWJ.
//   - TestTrakNullOffsets compares tables trak whose offset to the sizes
//     or to the values of a track is null, which HarfBuzz reads as the
//     start of the table (D101).
//   - TestUSESpaceSeparators compares the USE shaper at each space
//     separator, which the table that usegen generates puts in the
//     category SPACE (D101).
//
// A test skips when python3, uharfbuzz with HarfBuzz 15.2.0, or the folder
// typesetting/testdata is missing. uharfbuzz 0.57.0 built on the HarfBuzz
// 15.2.0 of the system works, such as the packages harfbuzz 15.2.0-1 and
// python-uharfbuzz 0.57.0 of Arch Linux (D97).
package harfbuzz
