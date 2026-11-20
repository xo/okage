# D73. typesetting fixes the faults that go-text still has

Status: Decided.

`typesetting` copies go-text with its faults (D52). Four faults are known in
`docs/BACKLOG.md`, and go-text has each one up to its commit `bafa8730`. The
pending commits of go-text, which okage applied on 2026-11-20, fix none of
them:

1. `harfbuzz` places a mark on some ligatures of a short text of Amiri in
   another place than HarfBuzz 15.1.0.
2. `FuzzBreakOptions` finds that the line breaker of `shaping` and the
   segmenter count the graphemes of a text in other ways.
3. `FuzzE2EVariableSize` finds that `WrapParagraph` gives more runes than
   its input in one configuration.
4. `typesetting` cannot read an anchor of a lookup MarkLigPos of
   `NotoSans_variable.ttf`, and drops the whole table GPOS of the font
   ("The PDF writer" in `docs/BACKLOG.md`).

The maintainers want these faults fixed in okage.

On 2026-11-20 faults 2 and 3 turned out to be faults of the fuzz tests of
go-text, not of the library. For fault 2, UAX #14 allows a line break after
a space before a combining mark, and UAX #29 keeps the space and the mark in
one grapheme cluster, so a line can end inside a cluster. The test counted
the clusters of the line alone. For fault 3, the test counted the runes of
the truncator of the last line twice. okage fixed both tests, and changed no
code of the library.

## The decision

1. `typesetting` first applies the pending commits of go-text (D52). A fault
   that a commit of go-text fixes needs nothing more.
2. For each fault that remains, okage fixes it in the copy. The fix follows
   HarfBuzz for the shaper, the Unicode standard for the segmenter and the
   line breaker, and the OpenType specification for the parse of a font.
3. Each fix has a test that fails before the fix, and a comment that starts
   with `okage:` and names this decision.
4. A fuzz target that leaves out an input because of a fault, such as the
   marks of the Arabic alphabet of `FuzzUnsafeToBreak` (D70), takes that
   input again when the fault is fixed.
5. A fix that changes the output of a test of `svg` or `pdf` follows the
   oracle of that test: resvg for `svg` and krilla for `pdf`.

## The fixes

1. Fault 1: a lookup MarkBasePos or MarkLigPos of `harfbuzz` keeps the
   base glyph that it found for the next mark. go-text kept that base from
   one lookup to the next. HarfBuzz clears it when a lookup starts, in
   `set_lookup_mask`. In Amiri, GSUB adds a tatweel after a lam. One lookup
   attaches the mark to the tatweel, and a later lookup must skip the
   tatweel and attach the mark to the lam. With the base of the first
   lookup, the second lookup did not attach the mark. In a longer text, a
   later mark cleared the base, so the fault showed only in a short text.
   `harfbuzz` now clears the base when a lookup starts. `TestMarkBaseOfLookup`
   tests the two texts of the fault. A comparison shaped 120,000 random
   texts of the Arabic alphabet of `FuzzUnsafeToBreak`, with its marks.
   HarfBuzz 15.1.0 and `harfbuzz` give the same glyphs, clusters, advances
   and offsets for each text.
2. Fault 4: lookup 35 of `NotoSans_variable.ttf` covers 287 marks, but its
   MarkArray has 281 records, and 6 records have a null anchor offset. The
   font does not follow the OpenType specification, which gives each mark a
   record and an anchor. go-text read a null offset as the start of the
   MarkArray, so its count, 281, was the format of an anchor. The sanitizer
   of go-text also refused the counts that differ. Each error dropped the
   whole table GPOS. HarfBuzz reads such a font. A null offset is its Null
   anchor at (0, 0), and a mark without a record has its Null record, with
   the class 0. `typesetting` now does the same. `TestMarkArrayNullAnchor`,
   `TestNotoSansVariableGPOS` and `TestMarkWithoutRecord` test it, and
   `TestTextGoVariable` of `test/krilla` compares the font with krilla
   again.

## When upstream changes

If go-text fixes one of these faults, follow these steps:

1. Apply the commit of go-text as D52 says.
2. If the commit gives the same result, remove the fix of okage, and keep
   its test.
3. If the result is not the same, compare both with HarfBuzz or with the
   specification, and keep the one that matches. A maintainer decides when
   neither one matches.

## Why

The faults give wrong glyphs, wrong line breaks or no kerning, and the PDF
files of `pdf` then differ from the files of krilla. go-text has not fixed
them, and okage can fix them with small changes that keep a test.
