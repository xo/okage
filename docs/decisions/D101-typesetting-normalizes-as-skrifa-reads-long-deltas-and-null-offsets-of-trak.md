# D101. typesetting normalizes as skrifa, reads long deltas and the null offsets of trak

Status: Decided, amends D84 and D100.

On 2026-11-25 the maintainers chose four changes to `typesetting`, the
copy of go-text in okage (D52). Each change of the copy has a comment that
starts with `okage:` and names this decision. Three changes are done. The
fourth, the generator of the USE table, stopped, because the Unicode data
files that it reads are not on the disk.

## 1. The coordinates of a variable font

`Font.NormalizeVariations` normalized the coordinate of an axis in
float32, rounded it to 2.14, and then mapped it with avar in 2.14. skrifa
and read-fonts convert the value of the user to the 16.16 format,
normalize it with a rounded division, map it with avar in 16.16, and round
it to 2.14 at the end, in `Fvar::user_to_normalized`. D84 ported that rule
to `axesLocation` of `internal/subsetter`, and D100 left the difference in
`svg` to phase 10 of the backlog.

`NormalizeVariations` now follows the rule of skrifa. The code is shared:
`tables.FixedFromFloat64`, `Fixed.SaturatingSub` and
`VariationAxisRecord.NormalizeFixed` in
`typesetting/font/opentype/tables/okage_fixed.go` are the code that
`internal/subsetter` had, and `axesLocation` calls them. `Face.SetVariations`,
which `svg` and `pdf` call, and `harfbuzz.Font.SetVarCoordsDesign` use the
new rule. The avar table of version 2 is still not read, as before.

A small Rust program read five fonts of `typesetting/testdata` with
read-fonts at the tag `skrifa-v0.49.0` of fontations, and printed the
coordinate of the first axis over its range, in steps of 1/8 or 1/16:
31,205 values of `CFF2-VF.otf`, `NotoSansCJKjp-VF.otf`, `Selawik-VF.ttf`,
`Commissioner-VF.ttf` and `SourceSans-VF.ttf`. The new rule gives each of
them. The old rule differed by one unit of 2.14 on 7,253 of them, for
example -16351 for -16350 at the weight 200.75 of `CFF2-VF.otf`.
`TestNormalizeVariationsSkrifa` holds ten such values.

HarfBuzz 15.2.0 normalizes in float, scales the value to 16.16, maps it with
avar in float, and rounds it to 2.14. On the same 31,205 values, it gives
the value of skrifa on all but 8, for example 5885 for 5884 at the weight
595.625 of `CFF2-VF.otf`. So the shaper is now closer to HarfBuzz than with
the old rule. The tests of the shaper against HarfBuzz pass, among them the
246 lines with variations of the tests of HarfBuzz.

Four test values of go-text change to the values of skrifa, which are also
those of HarfBuzz 15.2.0: -16117 for the weight 206 and 1530 for 450 of
`CFF2-VF.otf`, and 2674 for 301 and 6423 for 401.2 of
`NotoSansCJKjp-VF.otf`. `TestCFF2Var` compares extents that HarfBuzz made
at its older coordinate 2672 for the weight 301, so it now takes that
coordinate as it is.

## 2. The deltas of 32 bits of an item variation store

go-text returned an error for an ItemVariationData with the flag
LONG_WORDS. The table that holds the store, such as COLR, HVAR or GDEF,
was then left out of the font. The copy now reads such deltas, as
`DeltaSetIter` of read-fonts and `VarData::get_delta` of HarfBuzz do. A
word delta has 4 bytes and another delta 2 bytes. `ItemVariationData.DeltaSets`
holds an `int32` for each delta, where go-text holds an `int16`.

D100 compared the walk of COLRv1 with read-fonts on a copy of
`test_glyphs-glyf_colr_1_variable.ttf` of fontations whose deltas fit in 16
bits. The original font is now in `internal/colr/testdata`, with its
licenses. Its item variation data 2 has the deltas 46875, 48241 and 49151
for the axis ROTA. The Rust program printed the calls of
`ColorGlyph::paint` for each of the 201 color glyphs at six locations: the
default, each axis at 0.5, each axis at -0.5, ROTA at 1, ROTA at 1/3, and
random coordinates. `colr.PaintModel` makes the same calls at each location.
`TestPaintModelFontations` holds the SHA-256 of the calls at the first
five, and `TestPaintModelLongWords` the calls of the glyphs 99 to 102,
which rotate by an angle with a delta of 32 bits.
`TestColorGlyphLongWords` of `svg` draws the glyph 99, which had no color
glyph before.

## 3. The null offsets of the table trak

In HarfBuzz 15.2.0, the offset of the size table of a TrackData and the
offset of the values of a TrackTableEntry are never null: the offset 0 is
the start of the table trak. go-text ignored a null offset. A track with a
null offset to its values then had no values, and `value` indexed the
empty slice, which panicked. The parser now reads both offsets as HarfBuzz
does. `GetTracking` also gives the mean of the two values when two tracks
have the same value, as HarfBuzz does, where go-text divided by zero.

`TestTrakNullOffsets` of `test/harfbuzz` builds five tables trak from
Roboto: a null offset to the values with one size and with four sizes, a
null offset to the sizes, both null, and two tracks of the value 0. harfbuzz
gives the glyphs of HarfBuzz 15.2.0 for each. The first table panicked
before, and the last gave an advance of about -2^31.

## 4. The generator of the USE table

The maintainers asked for a Go generator of the USE table, as a port of
`gen-use-table.py` of HarfBuzz 15.2.0 or of the generator of go-text, so
that the table gives the category SPACE that D97 gives by hand in
`useCategory`. go-text has such a generator, `writer_use.go` of
typesetting-utils, which is in the module cache. It reads the files
`IndicSyllabicCategory.txt`, `IndicPositionalCategory.txt`, `Blocks.txt`,
`DerivedCoreProperties.txt`, `UnicodeData.txt` and others of the Unicode
Character Database. It downloads them from unicode.org. HarfBuzz also takes
them as files that its scripts download.

No copy of these files of go-text or of HarfBuzz is on the disk. The disk
holds only the files of Microsoft for USE, `IndicSyllabicCategory-Additional.txt`
and `IndicPositionalCategory-Additional.txt`, and old copies of other
projects at Unicode versions that do not match. The generators of
`typesetting` read no Unicode data files either. They build from the
generated tables. So this item stopped, and `useCategory` keeps the
mapping of D97. The maintainers must choose a source for the Unicode data
files. `docs/BACKLOG.md` holds the item.

## The checks

The tests of `svg` against resvg, the PDF oracle of krilla and subsetter
in `test/krilla`, and all of `test/harfbuzz` with `-harfbuzz.all` pass
against HarfBuzz 15.2.0.

A delta of an item variation store now takes 4 bytes in place of 2.
`BenchmarkEndToEndOnce` of `test/textbench`, with the code before and
after, 10 runs of each in turns, gives the same times. The medians in
milliseconds are 1.34 and 1.35 for english, 8.46 and 8.38 for arabic, and
4.31 and 4.26 for variable.

## Amendments

- D84: the normalization of `typesetting` for the shaper follows skrifa,
  not HarfBuzz. It is the one rule of the copy, which `internal/subsetter`
  shares.
- D100: the first two differences of "What stays different" are closed.
  The third, the id of a paint for the decycler, stays.
