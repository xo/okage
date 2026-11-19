# D65. typesetting fixes the variable fonts of the subsetter

Status: Decided, amended by D67.

`internal/subsetter` makes the instances of variable fonts with
`typesetting`, where subsetter makes them with skrifa (D62). Unit 4 of phase
13 found that `typesetting` gave wrong instances of CFF2 fonts, rounded the
map of the table avar another way than skrifa, wrote into the data of its
caller, and panicked on some malformed fonts. On 2026-11-18 the maintainers
decided to fix `typesetting` rather than work around it in
`internal/subsetter`. This decision records the commit of go-text that
okage applies out of order, and each difference from go-text. Each change of
`typesetting` has a comment that starts with `okage:` and names this
decision (D52).

## The CFF2 char strings

A CFF2 char string has no operator `return`. A subroutine ends at the end of
its data, and the caller goes on after the call. `CFF2.LoadGlyph` stopped at
the end of the first subroutine that it called. It also left the mask of a
hintmask at the end of a subroutine in place, and read it as a number. So
most CFF2 fonts gave wrong outlines.

1. okage applies the commit `14b50442` of go-text out of order. It fixes
   both faults, and other panics and faults of `font/cff`.
2. Its row in `docs/upstream/typesetting.tsv` is `ported`. The column
   `matched` does not move, because older commits are still pending (D12).
   Its test file reads `typesetting-utils`, so the copy leaves it out (D52).
3. The commit drops the widths of the char strings of CFF, next to the seac
   of D59. okage applies these two hunks by hand, and keeps the seac.

## The map of avar

`tables.SegmentMaps.Map` maps a normalized coordinate with the table avar.
go-text rounds the product of the interpolation, and then truncates the
quotient of its division. skrifa and FreeType interpolate in 16.16, with a
rounded division, and round the result to 2.14.

1. `Map` interpolates as skrifa does. A coordinate between two values of the
   map can no longer be 1/16384 lower than the coordinate of skrifa.
2. With this change, each instance of `Selawik-VF-Subset.ttf` in
   `test/krilla` is the same as the instance of subsetter, byte for byte.

## The checksum of WriteTTF

D67 applies the commit `f7b0a668` of go-text, which fixes the checksum of
`opentype.WriteTTF` as this decision did. okage follows go-text, and its
own change is gone.

## The panics on malformed fonts

ivy reads fonts from strangers. The fuzz test of `internal/subsetter` found
panics of `typesetting`. go-text fixes some of them in pending commits, and
okage fixes the others.

1. okage applies four commits of go-text out of order, in their order:
   `d674f8ed`, `a4526591`, `160e691e` and `001e002b`. They check the point
   numbers of gvar, reject a tuple of gvar and a table avar whose number of
   axes differs from fvar, and apply cleanly. Their rows in
   `docs/upstream/typesetting.tsv` are `ported`. Their tests read
   `typesetting-utils`, so the copy leaves them out (D52).

okage changes these functions:

1. `tables.ParseGlyf` sliced the table glyf with each range of the table
   loca. A range that is out of the table now gives a glyph without an
   outline, as in read-fonts and FreeType.
2. `tables.Hmtx.Advance` read the last long metric of a table without a long
   metric. Such a glyph now has the advance 0.
3. `tables.ParseHmtx` made a slice of a negative length when the table hhea
   gives more long metrics than the table maxp gives glyphs. It now returns
   an error for a negative count. `font.NewFont` gives it a count of 0
   side bearings in that case, so the font keeps its advances.
4. `CFF2.LoadGlyph` read the regions of the item variation store of CFF2
   with each region index of its item variation data. An index out of the
   list of regions now gives an error. The commit `c6965dda` of go-text,
   which D67 applies, guards the same indexes in `ItemVarStore.GetDelta`
   only.
5. The tests in `typesetting/font/opentype/okage_test.go` and
   `typesetting/font/opentype/tables/okage_test.go` cover the first three
   changes. `FuzzSubset` of `internal/subsetter` covers the fourth.

## The tolerance of the instances

skrifa computes the deltas of the variations in fixed point, and
`typesetting` in float32. A point of an instance can differ by 1 unit, and
in a CFF2 font a point of a cubic curve that differs by 1 unit can change the
number of quadratic curves of the curve. The maintainers accepted that the
comparison with subsetter in `test/krilla` fails only when more than 0.1% of
the glyphs of the instances of a font differ by more than 1 unit. Item 5 of
"Testing" under "The PDF writer" in [`PLAN.md`](../PLAN.md) records it.

## When upstream changes

1. When okage applies a pending commit of go-text that changes one of these
   functions, keep the change of okage, as "typesetting" in
   [`UPSTREAM.md`](../UPSTREAM.md) says.
2. If go-text fixes a fault in the same way, follow go-text, and remove the
   comment `okage:` and the item of this decision.
