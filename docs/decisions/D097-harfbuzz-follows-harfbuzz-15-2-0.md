# D97. harfbuzz follows HarfBuzz 15.2.0

Status: Decided, amends D73, D90 and D93.

## Context

D73 made `typesetting/harfbuzz`, the shaper of the copy of go-text (D52),
give the same output as HarfBuzz 15.1.0. The oracle in `test/harfbuzz`
compares the two with uharfbuzz. HarfBuzz 15.2.0 came out on 2026-11-20. On
2026-11-25 the maintainers decided to move the oracle of okage from
HarfBuzz 15.1.0 to 15.2.0. The oracle now uses the HarfBuzz 15.2.0 of the
system, through python-uharfbuzz 0.57.0.

With only the version of the oracle changed, five tests failed:
`TestUnsafeToConcat` on one text of `NotoNastaliqUrdu-Regular.ttf`, and the
four tests that run out of operations, because HarfBuzz 15.2.0 raised its
default work limits.

## The decision

`harfbuzz` follows HarfBuzz 15.2.0. okage read the source of both versions
and the NEWS of 15.2.0 with `gh api`, and ported each change of 15.1.0 to
15.2.0 that touches code that the copy has. Each change has a comment that
starts with `okage:` and names D97. These are the changes:

1. The work limits of `hb-limits.hh`. The buffer has 65,536 operations for
   each character, and at least 524,288. Its length can grow to 512 times
   the text. The default limit of operations is 0x3FFFFFFF. A subtable of
   morx or kerx no longer takes the length of the buffer for its pass. A
   pass of a lookup of GSUB or GPOS takes one operation for each 16
   glyphs. The skipping iterator takes one operation for each 32 glyphs
   that it skips. A lookup record that does not recurse takes one
   operation. The walks over a ligature, over the marks after a ligature,
   from a mark to its base in `propagateAttachmentOffsets`, and the search
   for the base of a mark each take operations. The lookups, the subtables
   and the walk of the attachments stop when the buffer is in error.
2. A lookup per syllable of GSUB stops at a glyph of another syllable, also
   when the lookup skips such a glyph. The serial of a syllable has 4
   bits, and 15.1.0 matched a glyph 15 syllables later. The syllable machines of
   Indic, Khmer, Myanmar and USE record the cut of a syllable of more than
   64 glyphs, so that the walks before and after the substitutions see the
   same pieces.
3. The search for the base of a mark in MarkBasePos and MarkLigPos stops at
   the first glyph that the lookup does not skip. Before, a mark attached
   to an earlier base when a feature on a range did not apply to the
   nearest base.
4. A decomposition that adds marks to a text with no marks makes the
   normalizer compose again, in the Indic, Khmer, Myanmar and USE shapers.
5. A global request of a feature wins over the earlier requests on ranges
   for the same tag. In AAT, the later of two requests for the same
   feature wins, and the end of a request removes that request.
6. A character that the font does not have takes the glyph `NotFound` of
   the buffer also before a variation selector.
7. The fallback positioning of marks places a mark from the x offset of its
   base. The last bit before the global bit of the map can be allocated.
8. The kerning machine of kern and of the pair subtables of kerx: the value
   -0x8000 of a cross-stream subtable resets the offset across the line, a
   value across the line is scaled on that axis, and the sums are clamped.
   A next glyph that the kerning does not skip makes the pair without the
   skipping iterator.
9. The glyphs after a cross-stream kern of kern or kerx are unsafe to break.
   At the levels of characters, a merge of graphemes in the output marks
   the glyphs unsafe to break.
10. The feature vkrn has a mask when the caller asks for it, so the table
    kern applies in vertical text when GPOS does not have vkrn.
11. kerx applies its subtables of pairs and classes that are not
    cross-stream together with GPOS, when GPOS has no feature of kerning.
    The marks then stay zeroed, and `hasGposMark` is false when GPOS does
    not apply. So the Hebrew shaper composes presentation forms in a font
    whose feature mark is only under the script DFLT.
12. The Indic shaper does not apply init to a first syllable after a letter
    or a mark of the context before the buffer.
13. The USE shaper gives the category SPACE to a space separator that is
    not a consonant placeholder, so a space is not the base of a dependent
    vowel. A generic base, a symbol cluster, a space and a ZWNJ end a word
    for the forms isol, init, medi and fina, with the flags of HarfBuzz.
    The generator of the USE table of go-text is not in this repository,
    so `useCategory` gives SPACE to the 16 characters, and the table keeps
    the other categories.
14. The Khmer shaper moves Coeng Ro before the base only when it is the
    first or the second coeng of the syllable.
15. The Hangul shaper takes a dotted circle as the base of a tone mark, and
    an inserted dotted circle takes its Unicode properties, so it keeps its
    advance in a font without GDEF. The same holds for the dotted circle of
    a prohibited vowel sequence. A default ignorable, such as ZWJ, between
    the vowels of such a sequence does not stop the match.
16. The Arabic shaper centers the tiles of stch on a short word and on an
    exact fit.
17. trak subtracts the tracking from the advance in vertical text, and
    clamps the sums. A subtable of format 4 of kerx leaves a glyph
    unattached when the attachment does not fit in 16 bits.
18. The anchors of GPOS are rounded and clamped to 32 bits, and the
    negation of a cursive offset saturates.
19. A subtable of morx or kerx whose actions need a glyph that the buffer
    does not hold is skipped (`collect_second_glyphs`). The segments of a
    lookup of format 4 are collected as those of format 2.

`typesetting/testdata` keeps the files of go-text byte for byte (D76). The
test of the in-house tests of HarfBuzz takes the line of HarfBuzz 15.2.0 in
place of one line of `arabic-stch.tests`, whose result 15.2.0 changed.

## What okage skips

- The cluster level graphemes, level 3, and the fixes for it. The copy has
  no such level. The merge of graphemes at the other levels is ported.
- The rounding of scaled glyph extents and the clamps of synthetic bold
  and slant in `hb-font.hh`. The copy computes the extents and the
  emboldening with its own arithmetic from go-text, which did not follow
  15.1.0 either. Only values near the limits of 32 bits change.
- `digest_second`, the pair filter of kern and kerx, the cache of the mark
  filtering sets, the fast paths of PairPos and of the decoding of UTF, and
  the other changes that make HarfBuzz faster and give the same output.
  `digest_second` skips a lookup, and with it the operations of its pass,
  so a text near the work limit can end in error in `harfbuzz` and not in
  HarfBuzz. No test text is near that limit.
- The work budgets of the closure, the dependency graph, the subsetter, the
  instancer and the renderers, and the checks of the sanitizer. The copy
  has none of them.
- `hb_font_create_without_funcs_or_fail`, the extender glyphs of JSTF,
  CBDT format 19, the CoreText and FreeType font functions, HarfRust, and
  the fixes of `hb_style_get_value`, `hb_face_collect_unicodes`, nested
  components of glyf with anchor points, and `hb_buffer_get_glyph_positions`.
  They are API additions or code that the copy does not have.
- The canonical form of the tags of ltag, which 15.2.0 moved into
  `hb_language_matches_string` with the same result.

## Tests

`test/harfbuzz` checks for HarfBuzz 15.2.0. The four tests of the limits
now make the buffer run out of the operations of 15.2.0: 65,535 records,
600,000 values of kerx, four features that ask about 32,000 rules, and an
insertion subtable whose insertions are not in the subtable. The tests of
D73 in `typesetting/harfbuzz` use the same fonts and the output of
HarfBuzz 15.2.0. `hb152_test.go` adds a test for each change that a font
built by hand or a font of `perf_reference` can show: changes 2 to 15, and
the tracking of change 17. Each of these tests fails when its change is
taken out. `shape.py` gained the options `--direction`, `--feature`,
`--not-found`, `--cluster-level` and `--item-offset` for them. The test of
change 16 is the line of `arabic-stch.tests`. These parts have no test:
the Hebrew part of change 11, because no test font has Hebrew
presentation forms, the last bit of the map in change 7, the attachment of
more than 16 bits of change 17, the clamps of change 18, which need values
near the limits of 32 bits, and change 19, which gives the same output.

All of `test/harfbuzz` passes against HarfBuzz 15.2.0, with
`-harfbuzz.all`: each of the 413,399 lines of `perf_reference`,
`TestScale`, the tests of kerx and the new tests.

`BenchmarkEndToEndOnce` of `test/textbench`, with the code before and
after, 10 runs of each in turns, gives these medians in milliseconds:

| Workload | Before | After |
| --- | --- | --- |
| pg24264 | 120.8 | 124.0 |
| pg1342 | 91.6 | 90.1 |
| english | 1.47 | 1.48 |
| arabic | 10.08 | 9.88 |
| mixed | 4.41 | 4.90 |
| devanagari | 3.91 | 3.87 |
| emoji | 1.94 | 1.97 |
| variable | 4.77 | 4.95 |

The machine ran other work, and the runs of mixed and variable spread from
4.1 to 9.2 milliseconds. Eight more runs of 2 seconds each gave the same
times before and after for both. The speed of D92 holds.

## Amendments

- D73: `harfbuzz` follows HarfBuzz 15.2.0, with the work limits of 15.2.0.
  The statements of D73 that measured against 15.1.0 stay as they are.
- D90: the oracle of the scale is HarfBuzz 15.2.0, which scales as 15.1.0
  did.
- D93: the flags unsafe-to-break and unsafe-to-concat are those of
  HarfBuzz 15.2.0.
