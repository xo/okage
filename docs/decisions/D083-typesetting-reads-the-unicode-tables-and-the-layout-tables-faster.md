# D83. typesetting reads the Unicode tables and the layout tables faster

Status: Decided, amends D81.

D81 made the shaper of `typesetting`, the copy of go-text in okage (D52),
faster for the books that ivy lays out. "What remains" in D81 named the
next candidates: the lookups of the Unicode properties of each rune in the
line breaker, and a cache of the coverage and the class definitions of
GPOS, as HarfBuzz has. This decision makes five changes that follow
these candidates. Each one changes the copy of go-text, so each one needs
the approval of the maintainers (D52). On 2026-11-21 the maintainers
accepted all five, answer 1 of question 35 of [`../PLAN.md`](../PLAN.md).

## What the profiles found

On 2026-11-20 a profile of `BenchmarkEndToEndOnce` of `test/textbench`
after D81 found these costs. Each share is of the time of
`BenchmarkEndToEndOnce` on the book.

1. The segmenter looks up six properties of each rune: the general
   category, the line break class, the grapheme break, the word break, the
   Indic conjunct break and the extended pictographic flag. Each property
   has a table of its own, and each lookup reads five levels of its table,
   one after the other. The start of each iteration of the segmenter,
   which holds these lookups, took 9% of the time of `pg24264` and 7% of
   `pg1342`.
2. The kerning of `pg1342` in Noto Sans is a lookup with a PairPos subtable
   of format 1 and one of format 2. For each glyph, the shaper searched the
   coverage of each subtable, and then the two class definitions of format
   2, each with a binary search. The searches of the coverage of GPOS took
   6% of the time, and the searches of the classes took 4%.
3. The shaper reads the glyph class of each glyph from the table GDEF with
   a binary search, once for each run and again after each substitution.
   This took 5% of the time of `pg24264` and 4% of `pg1342`.

HarfBuzz has caches for such lookups. In HarfBuzz 15.1.0, the lookup
accelerator gives each of the first 8 subtables of a lookup a cache of its
own (`external_cache_create` in `hb-ot-layout-gsubgpos.hh`). A subtable
keeps the coverage or the classes of 256 glyphs in it
(`hb_ot_layout_mapping_cache_t`), for example the classes of a context
lookup of format 2. The accelerator of GDEF keeps the glyph properties of
256 glyphs (`glyph_props_cache` in `GDEF.hh`).

## The decision

1. The segmenter finds the six properties of a rune in one lookup.
   `unicodedata.LookupSegmentProperties` reads a table of two levels: one
   byte for each block of 256 runes gives one of 192 distinct blocks, and
   each rune of a block gives one of 210 distinct sets of properties. The
   table takes 53 KiB in the binary, and the sets take 3.4 KiB of memory.
   The cursor of the segmenter keeps the properties of the next rune for
   the next iteration, so it does one lookup for each rune where it did
   six. The test `TestGenerateSegmentTable` writes the table from the
   tables of go-text with the flag `-unicodedata.generate`. Without the
   flag, it fails when the table is not the one that the tables of go-text
   give. `TestSegmentProperties` compares the table with the six lookups
   for each rune and for runes outside the range of Unicode.
2. Each of the first 8 PairPos subtables of format 2 of a lookup keeps a
   cache of its coverage index and of its two classes, in the way of the
   caches of HarfBuzz. Each cache holds 256 glyphs, at the index of the
   low 8 bits of the glyph. An item holds the whole glyph and its value, so a glyph never
   takes the value of another glyph. A coverage index of 0xFFFF or more is
   not kept. The caches take 3 KiB for each such subtable of each
   `harfbuzz.Font`.
3. Each `harfbuzz.Font` keeps a cache of the glyph properties of GDEF of
   256 glyphs, as HarfBuzz does. It takes 1 KiB for each `harfbuzz.Font`.
4. Each of the first 8 PairPos subtables of format 1 of a lookup keeps the
   cache of its coverage index too. Noto Sans holds its kerning by pairs
   before its kerning by classes, so each glyph searched the coverage of
   format 1 first. The cache takes 1 KiB for each such subtable of each `harfbuzz.Font`.
5. The cache of the glyph properties holds 4096 glyphs, where HarfBuzz
   holds 256. A text in Chinese uses a few thousand glyphs, and the cache
   of 256 glyphs missed most of them. It takes 16 KiB for each
   `harfbuzz.Font` in place of 1 KiB. The cmap cache of a `Face` of D81
   has the same size for the same reason.
6. A test compares each cache with the lookup in the table for each glyph
   of some fonts, twice, so that the second pass reads the cache. Each
   change has a comment that starts with `okage:` and names this decision.
   Each one is a commit of its own, in the order of this list, so the
   maintainers can take some of them and not others. Change 5 needs change
   3, and the caches of changes 2, 3 and 4 share one type.
7. The output of every test stays the same. On 2026-11-20, before and
   after each change, a hash was the same for each of these outputs:
   - The glyphs, the positions, the flags and the extents of each line of
     the seven texts of `perf_reference`, in each of its seven fonts, at
     three sizes, in three directions, with and without the flag
     unsafe-to-concat.
   - The line breaks, the graphemes and the words that the segmenter gives
     for each line of these texts and for each whole text.
   - Each output, line break and cut of each line of each workload of
     `test/textbench`, in both of its models of ivy.

   `test/harfbuzz` with `-harfbuzz.all` gives the glyphs of uharfbuzz for
   each of the 413,399 lines of `perf_reference` after the changes.

## The measurements

On 2026-11-20 `test/textbench` measured each change against the commit
before it. The machine had other work, so each benchmark of the old code
ran next to the same benchmark of the new code, in 10 pairs on 4 CPUs, and
the old one ran first in every other pair. Each number is the median of
the ratios of the pairs.

| Change | Once `pg24264` | Once `pg1342` | EndToEnd `pg24264` | EndToEnd `pg1342` |
| --- | --- | --- | --- | --- |
| 1. The table of the segmenter | -5.8% | -3.5% | -6.7% | -3.7% |
| 2. The cache of PairPos format 2 | +0.6% | -6.6% | -2.1% | -8.8% |
| 3. The cache of GDEF | -0.9% | -1.7% | -1.0% | -4.2% |
| 4. The cache of PairPos format 1 | -1.2% | -4.9% | +0.1% | -3.0% |
| 5. The cache of GDEF of 4096 glyphs | -2.7% | +0.6% | -2.4% | -0.2% |

"Once" is `BenchmarkEndToEndOnce` and "EndToEnd" is `BenchmarkEndToEnd`. A
change of 1% or less is within the noise of these runs. Change 1 also made
`BenchmarkSegmenter` 28% faster on `pg24264` and 25% faster on `pg1342`.

All five changes together, measured in the same way against the base
`75ea4b7`, made `BenchmarkEndToEndOnce` 10.3% faster on `pg24264`, from 163
to 146 ns/char, and 16.6% faster on `pg1342`, from 187 to 156 ns/char.
`BenchmarkEndToEnd` went from 245 to 218 ns/char on `pg24264` (-10.9%) and
from 306 to 251 ns/char on `pg1342` (-18.0%). `BenchmarkShape` went 9.4%
faster on `pg24264` and 19.3% faster on `pg1342`. On the small workloads,
in 6 pairs, `BenchmarkEndToEndOnce` took 16.8% less time on `english`,
12.0% on `emoji`, 7.3% on `mixed`, 4.2% on `devanagari`, 2.8% on
`variable` and 2.2% on `arabic`. `BenchmarkEndToEnd` changed by about the
same amounts.

One other candidate gave no gain, and this decision leaves it out.
`LookupGeneralCategory` read the general category of a rune above U+00FF
from the table of change 1. The shaper and the split look up the general
category of each rune. The change gave -0.2% to -1.1% on the four numbers.
It was a commit after change 1, so both binaries of the measurements of
changes 2 to 5 held it. The measurement of all five changes does not hold
it.

## What remains

After the changes, a profile of `BenchmarkEndToEndOnce` puts 49% of the
time of `pg24264` and 69% of `pg1342` in the shaper, and 15% and 12% in
the line breaks. `Paragraph.Line` takes 15% of `pg24264`, and half of it
is the split of each line. These are the next candidates:

- The loop of `applyForward` over the glyphs for each lookup, with its
  lookups in the digests, takes 11% of the time of `pg1342`.
- The split of each line in `Paragraph.Line`, which is okage's own code
  (D77), takes 7% of `pg24264`.
- The scale and the rounding of the advances and the extents, which item 2
  of "typesetting" in the backlog still lists for SIMD.

## When upstream changes

If go-text changes its tables of Unicode, or makes one of these changes in
its own way, follow these steps:

1. Apply the commit of go-text as D52 says.
2. If the tables of Unicode change, `TestGenerateSegmentTable` fails. Run
   `go test -run TestGenerateSegmentTable ./typesetting/internal/unicodedata/ -args -unicodedata.generate`
   to write the table again, then run the tests.
3. Remove the change of this decision that the commit replaces, and keep
   its test.
4. Run `test/textbench`, and record the result in the commit message.

## Why

Each change keeps the result of a lookup that the code does again for each
rune or for each glyph, and gives the same glyphs. Changes 2, 3 and 4 follow
the caches of HarfBuzz. Change 5 makes a cache larger than the cache of
HarfBuzz, because the Chinese book showed a gain.
