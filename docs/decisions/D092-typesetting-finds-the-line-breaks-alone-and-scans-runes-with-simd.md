# D92. typesetting finds the line breaks alone and scans runes with SIMD

Status: Decided, amends D72.

Items 1 and 2 of "typesetting" in [`../BACKLOG.md`](../BACKLOG.md) asked for
more speed in the text code of `typesetting`, the copy of go-text in okage
(D52). Item 1 asked to try the candidates of "What remains" in D87. Item 2
asked to use SIMD where the work is a scan or a loop over an array (D17,
D28), at three places: the scan for the skip of the bidi algorithm of D72,
a scan for a run of ASCII for the fast paths of D72, and the scale and the
rounding of the advances and the offsets in `harfbuzz`. The maintainers
asked for this work on 2026-11-25. This decision records what was kept,
with the numbers, and what was tried and dropped.

## What the profiles found

On 2026-11-25 a profile of `BenchmarkEndToEndOnce` of `test/textbench` on
the two books found these costs, after D90. Each share is of the time of
`BenchmarkEndToEndOnce`.

1. `Buffer.AddRunes` of `harfbuzz` appends each rune to the glyphs with
   `append`, one call for each rune. This took 3% to 5% of the time.
2. `Paragraph.LineBreak` computes the breaks of the paragraph with
   `segmenter.Segmenter.Init` and the line iterator. `Init` copies the
   text and applies the rules of the graphemes, the words and the lines to
   each rune. `Paragraph` reads only the line breaks. The breaks took 12%
   to 17% of the time.
3. `needsBidi`, the test of D72 for the skip of the bidi algorithm, looks
   up the bidi class of each rune. It took 3% of the time of `pg24264`,
   whose runes are almost all above U+00FF.
4. `Paragraph.Init` looks up the script, the paired delimiter and the
   general category of each rune. The lookup of the general category walks
   five levels of a table for a rune above U+00FF, where
   `LookupSegmentProperties` of D83 walks two levels. The loop took 3% of
   the time, without the faces of the fontmap of the benchmark.

## The decision

These changes are kept. Each one has a comment that starts with `okage:`
and names this decision.

1. `Buffer.AddRunes` grows the slices of the glyphs and the positions once
   for the whole item, and fills the new glyphs in place. The buffer grows
   as before, as `hb_buffer_add_codepoints` of HarfBuzz 15.1.0 does (D73).
2. The new function `segmenter.LineBreaks` computes only the line breaks
   of a paragraph, and writes them into a slice that the caller gives,
   with the flags `LineBreakAllowed` and `LineBreakMandatory`. It applies
   the rules of the lines of `computeBreakAttributes` alone. The rules of
   the graphemes and the words change no state that the rules of the lines
   read, so the breaks are the breaks of the line iterator after `Init`.
   `Paragraph.LineBreak` uses it, and `Paragraph` keeps no `Segmenter`.
   `TestLineBreaks` compares it with the line iterator on the texts of the
   three test files of Unicode, on 20,000 random texts of their runes, and
   on the empty text.
3. The new type `wide.RuneRanges` holds up to 8 ranges of runes. Its method
   `IndexOutside` gives the index of the first rune of a text that is in
   none of the ranges. In the SIMD build it tests 8 runes at a time with
   `Int32x8` on amd64, or two `Int32x4` on arm64 and wasm, and the scalar
   code tests the rest. It uses the SIMD code only for a text of 32 runes
   or more, where it is faster. On amd64 it clears the upper bits of the
   vector registers when it ends, as "The upper bits of the vector
   registers" in [`../SIMD.md`](../SIMD.md) says. Without the clear, the
   plain float code after it made `BenchmarkSplit` 5% to 8% slower.
   `TestRuneRanges` compares it with a plain test of each range in both
   builds.
4. `needsBidi` skips the runes of `noBidiRanges` with `IndexOutside`, and
   looks up the bidi class of each other rune as before. The 7 ranges hold
   no rune of the classes R, AL and AN, no bidi control and no paragraph
   separator. They cover ASCII from U+001F, Latin, Greek, Cyrillic,
   Armenian, the scripts from U+08E3 to U+200E, the general punctuation
   around the bidi controls, the CJK ideographs, Hangul, and the half width
   and full width forms. `TestNoBidiRanges` checks each rune of the ranges
   against the lookup, and `TestNeedsBidiScan` compares `needsBidi` with
   the lookup of each rune on random texts of each length up to 40.
5. `LookupGeneralCategory` of `unicodedata` reads the general category of
   a rune above U+00FF from the table of `LookupSegmentProperties`.
   `TestGeneralCategoryTable` compares it with the old lookup for each
   rune.
6. `Paragraph.Init` takes the script, the paired delimiter and the flag
   of `ignoreFaceChange` of a rune below U+0080 from a table of 128 items,
   which the same functions fill. The test of D87 compares the runs of
   `Paragraph` with the runs of `Segmenter.Split`.
7. Rule 8 of the hard rules holds. Only the files of `internal/wide`
   carry a build tag. `typesetting` calls `RuneRanges`, and gives the
   same results in both builds and with TinyGo.

This decision amends item 5 of D72, which said that `typesetting` does not
use SIMD. It uses SIMD in one place now, through `internal/wide`.

## The measurements

The machine had 32 CPUs and other work on them. So each benchmark of the
old code ran next to the same benchmark of the new code, in 20 pairs on 4
CPUs, and the old code ran first in every other pair. The numbers are the
medians of the ratios of the pairs. A change of 1% or less is within the
noise of these runs.

On 2026-11-25 each kept change was measured against all the changes
without it, on the two books:

| Change left out | Once `pg24264` | Once `pg1342` | EndToEnd `pg24264` | EndToEnd `pg1342` |
| --- | --- | --- | --- | --- |
| 1. `AddRunes` in one growth | -1.5% | -2.8% | -2.4% | -2.7% |
| 2. `LineBreaks` for `Paragraph` | -8.7% | -7.4% | (+2.7%) | (-4.9%) |
| 4. The scan of `needsBidi` | -0.9% | -0.6% | -3.9% | -0.8% |
| 5. The general category from the table of D83 | -1.4% | -0.2% | -2.1% | 0.0% |
| 6. The table of the runes below U+0080 | (+0.9%) | -1.3% | (-0.9%) | (-2.0%) |

"Once" is `BenchmarkEndToEndOnce` and "EndToEnd" is `BenchmarkEndToEnd`.
A number in parentheses is for a benchmark that does not run the changed
code, so it shows the noise of these runs: up to 3% for
`BenchmarkEndToEnd` and 1% for `BenchmarkEndToEndOnce`. Changes 2 and 6
change only `Paragraph`. Change 4 also runs in `Segmenter.Split`, for each
line of `BenchmarkEndToEnd`.

Two changes were also measured alone, in a benchmark of their own loop on
the lines of the two books, in one process:

- `needsBidi` took 3.4 ms for all the lines of `pg24264.txt` before the
  change, 2.3 ms with the scalar code of `IndexOutside`, and 1.0 ms to
  1.3 ms with the SIMD code. On the lines of `pg1342-images.html`, it took
  1.2 ms before, 0.57 ms with the scalar code and 0.65 ms with the SIMD
  code, because many of those lines are shorter than 32 runes.
- The loop of `Paragraph.Init` over the runes, without the faces, took 7.1
  ms for the lines of `pg1342-images.html` before change 6 and 4.3 ms after
  it. Change 5 took it from 11.3 ms to 10.1 ms on `pg24264.txt`.

All the changes together, against the base `ba729ba`, in 15 pairs:

| Benchmark | Scalar build | SIMD build |
| --- | --- | --- |
| Once `pg24264` | 140 to 129 ns/char, -12.3% | 132 to 117 ns/char, -11.3% |
| Once `pg1342` | 142 to 127 ns/char, -10.4% | 135 to 121 ns/char, -10.6% |
| EndToEnd `pg24264` | 236 to 219 ns/char, -7.1% | 207 to 192 ns/char, -7.0% |
| EndToEnd `pg1342` | 243 to 232 ns/char, -3.7% | 231 to 222 ns/char, -3.6% |

On the small workloads, `BenchmarkEndToEndOnce` took 9.7% to 9.9% less
time on `english`, 6.7% to 7.3% on `emoji`, 3.3% to 5.3% on `devanagari`,
and 1.6% to 3.9% on `arabic`, `mixed` and `variable`.
`BenchmarkEndToEnd` took 0% to 6.4% less time on them. Each range covers
both builds.

## What was tried and dropped

- A scan for a run of ASCII in the loop of `Paragraph.Init`, with
  `IndexOutside`, in place of the test `r < 0x80` for each rune. Each rune
  of the run still needs its own item of the table and its face, so the
  scan only adds a pass. On the lines of `pg1342-images.html`, the loop
  took 5.2 ms with the scan in the scalar build and 4.8 ms in the SIMD
  build, where the test for each rune took 4.2 ms. The fast paths of D72
  below U+0100 are one read of a table each, and a scan cannot remove the
  read. No other place of the text code was found where a run of ASCII
  skips work.
- One pass in `Paragraph.Line` that moves the clusters of the glyphs from
  the paragraph and sums their advances and their bounds, in place of the
  loop over the clusters and `RecalculateAll`. It made
  `BenchmarkEndToEndOnce` 6.6% slower on `pg1342`, in 30 pairs, and did not
  change `pg24264`.
- SIMD for the scale and the rounding of the advances in `harfbuzz`. A
  benchmark of the loop measured 1.5 ns for `roundf` of an advance, and
  2.3 ns with the scale of `emScaleX`, for each glyph. That is 1.7% of the
  time of a glyph of `pg1342` at most. A batch needs a second pass over the
  glyphs. The scale of D90 multiplies two values of 64 bits, and
  `internal/wide` has no vector of 64-bit integers. The rounding of D90
  adds one half in a `float64`. So a SIMD batch can save less than 1.7%,
  which the benchmarks cannot measure on this machine, and it adds two
  paths to code that must match HarfBuzz at each scale. This decision
  leaves it out. The offsets are scaled in the lookups of GPOS, one value
  at a time, and form no array.

## The output

The output of every test stays the same. On 2026-11-25 these tests passed:

- Every test of `typesetting` and of `internal/wide`, in the scalar build,
  in the SIMD build and with TinyGo.
- Every test of `test/textbench`, and `TestScale` and the run with
  `-harfbuzz.all` of `test/harfbuzz`, which gives the glyphs of uharfbuzz
  for each line of `perf_reference`. On 2026-11-25 the system of the
  machine updated HarfBuzz to 15.2.0, and these tests skip with that
  version. They ran with `LD_LIBRARY_PATH` set to the `libharfbuzz` of the
  package of HarfBuzz 15.1.0 in the cache of the package manager.
- A hash of each line break, of the outputs of `Paragraph.Line` for random
  ranges of each paragraph, and of a new shaping of each run of each
  paragraph at two sizes, with the glyphs, the positions, the extents and
  the counts of the clusters. The hash was the same before and after the
  changes for each workload of `test/textbench`, in both builds.

## What changes for ivy

ivy can call `segmenter.LineBreaks` where it asks the segmenter for the
line breaks of a text and does not read its graphemes or its words.
`BenchmarkEndToEnd`, which models the path of ivy without `Paragraph`,
still calls `Init` and the line iterator.

## What remains

After the changes, a profile of `BenchmarkEndToEndOnce` puts most of the
time in the shaper, as in D87. These candidates remain:

- The extents of the glyphs of `pg24264`, which miss the cache of D87 for
  many of its few thousand glyphs.
- The rules of the line breaks themselves, now that `LineBreaks` applies
  them alone.
- The faces of the fontmap of the benchmark take 5% of the time of
  `BenchmarkEndToEndOnce`. That is code of the caller, not of
  `typesetting`.

## When upstream changes

If go-text makes one of these changes in its own way, follow these steps:

1. Apply the commit of go-text as D52 says.
2. Remove the change of this decision that the commit replaces, and keep
   its test.
3. Run `test/textbench`, and record the result in the commit message.

## Why

Each change removes work that does not change the result, and each one is
small, so the copy stays close to go-text. The scan of the runes is the
one place of the text code where the work is a loop over an array that
SIMD makes faster. The other places of item 2 are a lookup for each rune
or a value for each glyph, where SIMD saves less than the noise.
