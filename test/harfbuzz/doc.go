// Package harfbuzz holds the oracle of typesetting/harfbuzz, the shaper of
// the copy of go-text in okage (D52, D73). It has no code of its own.
//
// uharfbuzz, the Python binding of HarfBuzz, shapes each text with
// shape.py, and the tests compare its glyphs, clusters, advances and
// offsets with the output of harfbuzz. The tests need uharfbuzz with
// HarfBuzz 15.1.0, the version that D73 follows:
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
//
// A test skips when python3, uharfbuzz with HarfBuzz 15.1.0, or the folder
// typesetting/testdata is missing. To install uharfbuzz, run:
//
//	pip install uharfbuzz==0.57.0
package harfbuzz
