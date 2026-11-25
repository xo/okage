# D81. typesetting skips repeated work in the shaper

Status: Decided, amends D72, amended by D83 and D90.

D72 and D77 made the text code of `typesetting`, the copy of go-text in
okage (D52), faster for the books that ivy lays out. Item 1 of
"typesetting" in [`../BACKLOG.md`](../BACKLOG.md) then named the next step.
A profile of `BenchmarkEndToEndOnce` of `test/textbench` put 53% of the
time of `pg24264` and 67% of `pg1342` in the shaper, 20% and 13% in the
line breaks, and 12% and 5% in the split. This decision makes six small
changes that remove work which the shaper repeats for each glyph or for
each run. Five of them change the copy of go-text, so they needed the
approval of the maintainers (D52). On 2026-11-20 the maintainers accepted
all six, answer 1 of question 33 of [`../PLAN.md`](../PLAN.md).

## What the profiles found

On 2026-11-20 a profile of `BenchmarkEndToEndOnce` and `BenchmarkEndToEnd`
on the two books found these costs in the shaper. Each share is of the time
of `BenchmarkEndToEndOnce`.

1. The scale and the rounding of positions in `harfbuzz` (`emScalef` and
   `roundf`) took 8% of `pg24264` and 7% of `pg1342`. The shaper scaled the
   horizontal origin of each glyph three times, and that origin is always
   0, 0 in go-text. `roundf` called `math.Round`, which Go does not turn
   into one instruction.
2. The cache of shape plans took 4% of `pg24264`. It compared the key with
   each plan of the face by value, and so copied the whole plan for each
   call of `Shape`. `pg24264` has many short runs, because its Latin
   punctuation is in another font than its ideographs.
3. The lookups of the cmap took 6% of `pg24264`. The cache of the cmap of a
   `Face` holds 256 runes, and a text of Chinese uses a few thousand.
4. The kerning of `pg1342` took 12% of its time, in a PairPos of format 2.
   Its records were decoded into a full `ValueRecord`, though they hold
   only an advance. The binary searches of the coverage and the class
   definitions of GSUB and GPOS took 10%, part of it in the kerning.
5. `Paragraph.Line` of D77 took 3% of its time to append the glyphs of a
   line one by one.

The text is not decoded from UTF-8 in `typesetting`. Its API takes runes,
and a program that uses `Paragraph` decodes each paragraph once.
`BenchmarkEndToEndOnce` has no decode, and in `BenchmarkEndToEnd` the
decode of the model of ivy takes 3% of the time of `pg24264` and less of
`pg1342`. So the candidate "decode the UTF-8 of a text once" of the backlog
is work for ivy, not for `typesetting`.

## The decision

1. `getGlyphHOriginWithFallback` of `harfbuzz` skips the scale when the
   face gives the origin 0, 0. The scale of 0 is 0 for each upem from 16 to
   16384, which the table head allows.
2. `roundf` of `harfbuzz` adds one half away from zero and truncates, for a
   value that fits in an int32. A float32 has 24 bits, so the sum in a
   float64 is exact, and the result is the result of `math.Round`. Any
   other value, NaN and the infinities take `math.Round`, as before,
   because the conversion of such a value depends on the platform. D16 and
   D71 hold: no multiply joins the add. A test compares the two on one
   float32 in 97 and on the values next to each half and each limit. A run
   over all 2^32 values on amd64 found no difference. D90 changes the
   rounding of a half to `floor(x + .5)`, as HarfBuzz 15.1.0 does.
3. `equal` and `userFeaturesMatch` of the shape plan take pointers.
4. The cache of the cmap of a `Face` holds 4096 runes, in a type of okage,
   `cmapCache4K`, which is `cache21_19_8` of go-text with more entries. It
   takes 16 KiB for each `Face`, where the old cache took 1 KiB. A test
   compares `NominalGlyph` with the cmap for each rune.
5. `PairPosData2.Record` reads the advance alone when the value formats are
   `XAdvance` and 0, and gives the record that `parseValueRecord` gives. A
   test compares the two.
6. `Paragraph.Line` appends the glyphs of a run in one copy, and then moves
   their clusters. This code is okage's own (D77).
7. Each change has a comment that starts with `okage:` and names this
   decision, and each one is a commit of its own, so the maintainers can
   take some of them and not others.
8. The output of every test of `typesetting`, `svg`, `pdf` and the test
   module stays the same. On 2026-11-20 a hash of the glyphs, the positions,
   the flags and the extents of each line of the seven texts of
   `perf_reference`, in its fonts, at three sizes, in three directions,
   with and without the flag unsafe-to-concat, was the same before and
   after the changes. A hash of each line of each workload of
   `test/textbench`, in both of its models of ivy, was the same too.
9. This decision amends item 5 of D72. A decode of UTF-8 once for all
   stages is not work for `typesetting`, and it leaves the backlog of
   `typesetting`.

## The measurements

On 2026-11-20 `test/textbench` measured each change alone against the
base `4c93b59`. The machine had other work, so each benchmark of the old
code ran next to the same benchmark of the new code, in 10 pairs on 4
CPUs, and the old one ran first in every other pair. Each number is the
median of the ratios of the pairs.

| Change | Once `pg24264` | Once `pg1342` | EndToEnd `pg24264` | EndToEnd `pg1342` |
| --- | --- | --- | --- | --- |
| 1. The origin 0, 0 | -4.4% | -5.0% | -5.4% | -4.2% |
| 2. `roundf` | -3.5% | -3.1% | -4.8% | -2.8% |
| 3. The plans by pointer | -5.9% | -1.7% | -7.2% | -1.7% |
| 4. The cmap cache | -4.9% | -1.1% | -4.9% | +0.1% |
| 5. The kerning record | -0.5% | -2.6% | -1.0% | -1.6% |
| 6. `Paragraph.Line` | -2.0% | -3.3% | -0.6% | -0.3% |

"Once" is `BenchmarkEndToEndOnce` and "EndToEnd" is `BenchmarkEndToEnd`. A
change of 1% or less is within the noise of these runs. Change 6 cannot
change `BenchmarkEndToEnd`, which does not use `Paragraph`.

All six changes together, measured in the same way, made
`BenchmarkEndToEndOnce` 17.4% faster on `pg24264`, from 210 to 173
ns/char, and 11.8% faster on `pg1342`, from 220 to 193 ns/char.
`BenchmarkEndToEnd` went from 307 to 259 ns/char on `pg24264` (-15.5%)
and from 351 to 318 ns/char on `pg1342` (-8.9%). On the small workloads,
in 6 pairs, `BenchmarkEndToEndOnce` took 14% less time on `english`, 12% on
`emoji`, 8% on `mixed`, 5% on `devanagari` and 2.5% on `arabic`.
`variable` changed by +1.3%, within the noise. `BenchmarkEndToEnd` changed
by about the same amounts.

Two other candidates gave no gain, and this decision leaves them out:

- The normalizer of `harfbuzz` stops its short circuit at the first rune
  that the font does not have, as HarfBuzz does, and decomposes the rest of
  the run one rune at a time. A version that goes back to the short circuit
  after that rune changed the four numbers by -1.1% to +0.4%.
- The binary searches of `Coverage1`, `Coverage2` and `ClassDef2` without
  `sort.Search` and its closure changed them by -1.1% to +0.7%. The Go
  compiler already inlines the closure.

## What remains

After the changes, a profile of `BenchmarkEndToEndOnce` puts 46% of the
time of `pg24264` and 70% of `pg1342` in the shaper, 23% and 13% in the
line breaks, and 7% and 5% in the split.
These are the next candidates. Each one is a larger change:

- The lookups of the Unicode properties of each rune in
  `segmenter.computeBreakAttributes`: the general category, the line break
  class, the grapheme break, the word break and others, each in a table of
  its own.
- The coverage and the class definitions of GPOS. HarfBuzz keeps a small
  cache of their results for each lookup, and go-text does not.
- The scale and the rounding of the advances and the extents. D92 measured
  them for SIMD and left them out.

## When upstream changes

If go-text or HarfBuzz makes one of these changes in its own way, follow
these steps:

1. Apply the commit of go-text as D52 says.
2. Remove the change of this decision that the commit replaces, and keep
   its test.
3. Run `test/textbench`, and record the result in the commit message.

## Why

Each change removes work that has no effect on the result, and each one is
a few lines, so the copy stays close to go-text. Together they take 12% to
17% of the time of the path of ivy with `Paragraph`, with the same glyphs.
