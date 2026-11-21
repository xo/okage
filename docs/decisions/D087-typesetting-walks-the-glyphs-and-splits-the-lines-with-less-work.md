# D87. typesetting walks the glyphs and splits the lines with less work

Status: Proposed.

D81 and D83 made the text code of `typesetting`, the copy of go-text in
okage (D52), faster for the books that ivy lays out. "What remains" in D83
named three candidates: the loop of `applyForward` over the glyphs for each
lookup, the split of each line in `Paragraph.Line`, and the scale and the
rounding of the advances and the extents. This decision proposes eight
changes that follow these candidates. Six of them change the copy of
go-text, so they need the approval of the maintainers (D52). Question 38 of
[`../PLAN.md`](../PLAN.md) asks for it. Changes 5 and 6 change okage's own
code of `Paragraph` (D72, D77).

## What the profiles found

On 2026-11-21 a profile of `BenchmarkEndToEndOnce` of `test/textbench`
after D83 found these costs. Each share is of the time of
`BenchmarkEndToEndOnce` on the book.

1. For each lookup, `applyForward` reads each glyph of the run and tests it
   against the digest of the lookup, the mask and the glyph properties.
   This loop took 8% of the time of `pg1342`. The run of `pg1342` reads 7.4
   million glyphs in this loop for 0.7 million characters, and the digest
   rejects 87% of them.
2. For each glyph that passes the digest, the accelerator of the lookup
   tries each subtable, and the context and pair lookups test the glyphs
   after it with the skipping iterator. Two methods there took their value
   by copy: `maySkip` copied the whole matcher for each glyph, and `apply`
   of a subtable copied the subtable.
3. The shaper adds the origin of each glyph to its offsets before GPOS, and
   subtracts it again after GPOS. The faces of go-text always give the
   origin 0, 0, so these two loops do nothing unless the font has an
   embolden. With the loop of `positionDefault`, they took 2% to 3% of the
   time of both books.
4. The shaping package asks for the extents of each glyph that it shapes.
   `GlyphExtents` scales each of the four values with a multiply, a divide
   and a rounding. It took 5% of the time of both books.
5. `Paragraph.Line` splits each line again, rune by rune, in two passes:
   by script and by face. The split took 6% of the time of `pg24264`.
6. `Paragraph.Line` counts the runes and the glyphs of each cluster of each
   run of the line again, also for the glyphs that it copies from the
   paragraph. This took 2% of the time of `pg24264`.

## The decision

1. `applyForward` of `harfbuzz` keeps the digest, the mask and the
   properties of the lookup in local variables, and does the first test of
   `checkGlyphProperty` itself. The Go compiler does not inline
   `checkGlyphProperty`, whose cost is 88 where the budget is 80.
2. `maySkip` of the matcher of the skipping iterator takes a pointer.
3. `positionDefault` and `positionComplex` skip the loops over the origins
   of the glyphs when the font has no embolden. A test makes sure that the
   faces give the origin 0, 0 for each glyph of some fonts.
4. Each `harfbuzz.Font` keeps a cache of the scaled extents of 4096 glyphs,
   at the index of the low 12 bits of the glyph. An item holds the bits of
   the extents that the face gives, and serves a glyph only when the face
   gives the same bits again. The scale gives the same result for the same
   bits, so an item never gives a wrong value, also when two glyphs share an
   item, or when the face changes its variations. The cache keeps the
   scales, the slant and the embolden of its items, and empties itself when
   one of them changes. The cache takes 144 KiB for each `harfbuzz.Font`.
   A test compares it with the extents without the cache for each glyph of
   three fonts, at several scales, with a slant and an embolden, and at two
   variations of a variable font.
5. `Paragraph.Init` marks, for each rune, the next rune that can change the
   split by script and the next rune that can change the split by face.
   When the run of the split has the script of the last strong rune, or the
   face of the last rune that can change the face, `splitRange` jumps to the
   next such rune. The runes between them do not change the runs. The two
   marks take 8 bytes for each rune of the paragraph. A test compares the
   split with `Segmenter.Split` on 600 random texts of long words, each in
   one script. Another test makes sure that the marks of one text are right.
6. `Paragraph.Line` keeps the counts of the clusters that it copies from
   the paragraph, and counts only the clusters that it shaped again at the
   ends of a run. The counts are the counts that `countClusters` gives for
   the whole run. When the clusters of the run do not go in the direction
   of the run, it counts the whole run as before.
7. `mayHave` of the set digest tests the three filters of HarfBuzz in one
   expression with no branch. A test compares it with the three tests for
   random digests and for each glyph.
8. `apply` of the accelerator of a lookup does not copy each subtable, and
   `apply` of a subtable takes a pointer.
9. Each change has a comment that starts with `okage:` and names this
   decision. The changes of the lookups (1, 2, 7 and 8), the origins (3),
   the extents (4) and `Paragraph` (5 and 6) are four commits, so the
   maintainers can take some of them and not others.
10. The output of every test stays the same. On 2026-11-21 these tests
    passed:
    - Every test of `typesetting`, of `test/textbench` and of
      `test/harfbuzz`.
    - `test/harfbuzz` with `-harfbuzz.all`, which gives the glyphs of
      uharfbuzz for each of the 413,399 lines of `perf_reference`.
    - A hash of each line break, of the outputs of `Paragraph.Line` for a
      third of the lines and for random ranges of each paragraph, and of a
      new shaping of each run of each paragraph, with the glyphs, the
      positions, the extents and the counts of the clusters. The hash was
      the same before and after the changes for each workload of
      `test/textbench`.
    - The fuzz target `FuzzParagraphLine` ran for 84 seconds with changes
      1 to 6. It found one fault, which the code before the changes has
      too. Item 6 of "typesetting" in the backlog records it.

## The measurements

On 2026-11-21 `test/textbench` measured each change against the build
before it, on a machine with 32 CPUs. Each benchmark of the old code ran
next to the same benchmark of the new code, in 10 pairs on 4 CPUs, and the
old one ran first in every other pair. benchstat gave the change of the
medians.

| Change | Once `pg24264` | Once `pg1342` | EndToEnd `pg24264` | EndToEnd `pg1342` |
| --- | --- | --- | --- | --- |
| 1. The loop of `applyForward` | -0.5% | -1.9% | -0.7% | -1.8% |
| 2. `maySkip` by pointer | -0.9% | -1.2% | +0.3% | -1.1% |
| 3. No loops over the origins | -2.1% | -2.5% | -2.9% | -3.0% |
| 4. The cache of the extents | -2.1% | -2.9% | -1.9% | -2.0% |
| 5. The skips of the split | -1.7% | +0.1% | -0.5% | -0.2% |
| 6. The counts of the clusters | -1.1% | -1.9% | 0.0% | -0.4% |
| 7. `mayHave` with no branch | +0.1% | -2.0% | -2.3% | -1.1% |
| 8. The subtables by pointer | -0.4% | -2.4% | -0.1% | -1.4% |

"Once" is `BenchmarkEndToEndOnce` and "EndToEnd" is `BenchmarkEndToEnd`. A
change of 1% or less is within the noise of these runs. Changes 5 and 6
cannot change `BenchmarkEndToEnd`, which does not use `Paragraph`.

All eight changes together, measured in the same way against the base
`5e860c6`, made `BenchmarkEndToEndOnce` 10.2% faster on `pg24264`, from 151
to 135 ns/char, and 14.1% faster on `pg1342`, from 160 to 137 ns/char.
`BenchmarkEndToEnd` went from 226 to 212 ns/char on `pg24264` (-6.4%) and
from 261 to 234 ns/char on `pg1342` (-10.1%). On the small workloads, in 6
pairs, `BenchmarkEndToEndOnce` took 10.1% less time on `english`, 9.1% on
`emoji` and on `mixed`, 8.7% on `arabic`, 8.3% on `devanagari` and 2.8% on
`variable`. `BenchmarkEndToEnd` took 3.7% to 8.8% less time on them.

Three other forms gave less, and this decision leaves them out:

- A cache of the scaled advances, next to the cache of the extents. With
  it, change 4 gave -0.7%, -1.6%, +0.2% and -2.2%, less than the cache of
  the extents alone. The face already keeps the advances, and the second
  cache read more memory.
- A cache of the extents of 256 glyphs, as the caches of HarfBuzz hold. It
  gave -3.3% on `pg1342`, but +2.7% on `pg24264`, whose text in Chinese
  uses a few thousand glyphs.
- A pass of its own for the marks of change 5, after the loop of
  `initSplit`. The profile put 2.7% of the time of `pg24264` in it, and
  0.8% when the loop of `initSplit` marks the runes.

Two other forms do not work with Go 1.27. `checkGlyphProperty` in one
expression has the cost 84, and `roundf` with no branch has the cost 118,
so the compiler inlines neither of them.

## What remains

After the changes, a profile of `BenchmarkEndToEndOnce` puts 47% of the
time of `pg24264` and 63% of `pg1342` in the shaper, and 16% of each in
the line breaks. The loop of `applyForward`, with the lookups that it
applies, takes 40% of `pg1342`. `Paragraph.Line` takes 15% of `pg24264`.
`initSplit` takes 10% of `pg24264`, and most of it is the fontmap of the
benchmark. These are the next candidates:

- The extents still take 6% of the time of `pg24264`, which uses a few
  thousand glyphs, and the scale of the advances takes 2% of each book.
  Item 2 of "typesetting" in the backlog lists these scales for SIMD.
- The line breaks of `Paragraph.LineBreak`, which the segmenter computes
  once for each paragraph.
- The copy of the glyphs of a line in `Paragraph.Line`, and the sum of
  their advances and extents in `RecalculateAll`.

## When upstream changes

If go-text or HarfBuzz makes one of these changes in its own way, follow
these steps:

1. Apply the commit of go-text as D52 says.
2. Remove the change of this decision that the commit replaces, and keep
   its test.
3. Run `test/textbench`, and record the result in the commit message.

## Why

Each change removes work that does not change the result, and each one is a
few lines, so the copy stays close to go-text. Change 4 is the one change
that takes memory: 144 KiB for each `harfbuzz.Font`. Together the changes
make the path of ivy with `Paragraph` faster on both books, with the same
glyphs.
