# D93. shaping gives the flag unsafe-to-concat, and Line shapes reversed runs again

Status: Decided, amends D70 and D77, amended by D97.

Items 3, 4 and 5 of "typesetting" in [`../BACKLOG.md`](../BACKLOG.md) were
open after D90:

- Item 3: `Shape` gives only the flag unsafe-to-break of HarfBuzz (D70).
  The flag unsafe-to-concat was only for the private shaper of
  `shaping.Paragraph`, and `Paragraph` removes it from the glyphs that it
  gives (D77).
- Item 4: in a run of tatweels and spaces of the script Common, Amiri
  forms a ligature of tatweels where a run starts, and the cluster is not
  marked unsafe to break. The fuzz target of `Paragraph` left the tatweel
  out of its alphabet.
- Item 5: `FuzzParagraphLine` found the text `fAbb !Z` with the cuts `0`,
  from left to right, where `Paragraph.Line` gives another advance than a
  new shaping.

The maintainers asked for this work on 2026-11-25. The oracle is
uharfbuzz with HarfBuzz 15.1.0 in `test/harfbuzz`.

## The flag unsafe-to-concat

HarfBuzz sets the flag unsafe-to-concat only when the buffer has the flag
`HB_BUFFER_FLAG_PRODUCE_UNSAFE_TO_CONCAT`, because it costs time. A
cluster is unsafe to concat when the text that breaks at its start can
give other glyphs if the two parts are shaped on their own and joined. A
program can then shape again only a few clusters around a line break, as
the comment of `harfbuzz.GlyphUnsafeToConcat` says.

1. `HarfbuzzShaper.SetUnsafeToConcat(on bool)` sets whether the shaper
   sets the buffer flag `harfbuzz.ProduceUnsafeToConcat` for each shaping.
   It is off by default, as in HarfBuzz. The glyphs and their positions
   are the same with the flag on and off.
2. `Glyph.UnsafeToConcat` reports whether the mask of a glyph holds
   `harfbuzz.GlyphUnsafeToConcat`. The comment of `Glyph.Mask` says that
   the mask holds the flag only after `SetUnsafeToConcat`.
3. `HarfbuzzShaper` has no constructor, and go-text configures it with the
   method `SetFontCacheSize`. So the new setting is a method of the same
   kind, not a functional option. Hard rule 11 is about the options of the
   code of okage, and a change of `typesetting` stays close to go-text
   (D52). On 2026-11-25 the maintainers kept the method, and hard rule 11
   of `AGENTS.md` now names this exception.
4. `Paragraph` does not change. Its shaper always sets the flag, and
   `Line` removes it from the glyphs that it gives (D77).

This replaces item 3 of "The flags" of D70.

## The flags follow HarfBuzz 15.1.0

`TestUnsafeToConcat` of `test/harfbuzz` compares the flags unsafe-to-break
and unsafe-to-concat of each glyph with HarfBuzz. `shape.py` takes the new
argument `--concat`. Before the changes below, 4 of the 49 pairs of a font
and a text of `perf_reference` gave other flags in their first 200 lines.
The glyphs and their positions were the same. go-text marked other glyphs
unsafe to concat than HarfBuzz at four places, and `harfbuzz` now follows
HarfBuzz there:

1. A ligature set of GSUB first finds the next glyph that the lookup does
   not skip. It then tries only the ligatures whose second component is
   that glyph. When it passes over another ligature, it marks the glyphs up
   to the next glyph unsafe to concat. go-text tried each ligature, and a
   ligature that did not match marked its glyphs. For example, Roboto has
   the ligatures ffi and fi for f. In "fi", go-text marked f and i, and
   HarfBuzz marks no glyph.
2. A rule set of a context or chained context lookup with more than 4
   rules does the same with the first two glyphs after the current glyph.
   When no glyph follows, it tries only the rules with no more input and
   no lookahead, and marks the glyphs up to the end of the buffer when no
   rule applies.
3. A chained context rule whose input does not match marks no glyph.
   HarfBuzz keeps the end of the range at the current glyph in that case.
   go-text marked the glyphs up to the glyph that did not match.
4. A reverse chaining lookup whose backtrack or lookahead does not match
   marks the glyphs that it read unsafe to concat. go-text marked no glyph.

The source of HarfBuzz 15.1.0 was not at hand. Changes 1 and 2 follow the
fast paths of `LigatureSet::apply`, `RuleSet::apply` and
`ChainRuleSet::apply` of HarfBuzz 8.5.0, which apply to sets of more than
4 items. Tests against HarfBuzz 15.1.0 then showed two differences from
8.5.0: a ligature set takes the fast path at each size, and a rule set
with no next glyph marks the glyphs up to the end.
`TestRuleSetUnsafeToConcat` tests each path with lookups built by hand.
After the changes, each of the 413,399 lines of `perf_reference` gives the
same flags as HarfBuzz 15.1.0, with `-harfbuzz.all`.
`TestUnsafeToConcatFlags` of `harfbuzz` and `TestSetUnsafeToConcat` of
`shaping` test the changes without the oracle.

## The direction of the shaper

HarfBuzz shapes a run in the direction of its script. When the direction
of the run differs, `ensureNativeDirection` of `harfbuzz` reverses the
buffer, and the glyphs are reversed again at the end. In a run from left
to right in a script from right to left, such as Arabic, HarfBuzz keeps
the direction of the run when the run holds a decimal digit or a regional
indicator and no letter. So the direction can depend on all the runes of
the run.

Item 5 has this cause. The text of the fuzz target is `(٢'')[0`, and the
split gives the run `'')[0` of the script Arabic, from left to right. It
holds a digit, so HarfBuzz shapes it from left to right. The line of the
runes 0 to 5 gives the run `'')`, which has no digit, so HarfBuzz shapes
it from right to left. Roboto kerns the two quotes, and the kerning goes to
the first quote in one direction and to the second quote in the other.
HarfBuzz 15.1.0 gives the same glyphs as `harfbuzz` for both texts, and no
flag marks the cut after `)`. A test of the flags as `hb-buffer-verify.cc`
does it, which shapes the two parts of the text at each cut that is safe
to break, also fails on HarfBuzz itself.

Item 4 is a fault of HarfBuzz in such a run too. A run of tatweels and
spaces has the script Common, which HarfBuzz sets from left to right. In a
paragraph from right to left, HarfBuzz reverses the run. Amiri makes
ligatures of up to seven tatweels, so in the reversed buffer they form
from the end of the run. A contextual lookup of the feature rlig then
changes the tatweel that is left at the start. For 15 tatweels, HarfBuzz
15.1.0 gives the same glyphs and flags as `harfbuzz`: the first tatweel
has the glyph 6628, the cut after it is safe to break, and the first
tatweel alone has the glyph 5992. The test of `hb-buffer-verify.cc`
fails at that cut on HarfBuzz itself. In a run of the script Arabic, set
from right to left, the ligatures form from the start, and the flags are
right.

`Paragraph.Line` now does not trust the flags of HarfBuzz for runes that
the shaper shapes against the direction of their run. It shapes such a
run again whole:

1. `shapedReversed` tells whether the shaper reverses the runes of a run,
   as `ensureNativeDirection` does. `Init` keeps it for each run of the
   paragraph, and `Line` computes it for the runes of each run of the
   line. When either one is true, `Line` shapes the run again whole.
2. When the direction depends on the runes, the part of the run in the
   line is numeric. `spliceLeft` and `spliceRight` skip a piece that is
   not numeric, because the shaper shapes it in the other direction.
3. `harfbuzz` exports `ScriptHorizontalDirection`, which is
   `hb_script_get_horizontal_direction` of HarfBuzz, for `shapedReversed`.

Such runs are rare in a paragraph. The bidi algorithm gives the letters of
a script from right to left an odd level, and the split gives the
neutral runes around them the script of their neighbors. A reversed run is
a run of neutral runes alone, such as digits and punctuation in a run of
the other direction. So the cost is small.

`FuzzParagraphLine` holds the tatweel in its alphabet again, and its seed
corpus holds the inputs of items 4 and 5. With the change, the fuzz target
ran 5 minutes on 6 workers without a failure. `TestShapedReversed` of
`shaping` tests `shapedReversed`. The item about U+01EC in D70 stays: it is
a fault of HarfBuzz in a run in its own direction.

## The cost

On 2026-11-25 `test/textbench` ran the old and the new code in turns, 10
runs each, with 20 iterations on 4 CPUs. The medians of the 24 benchmarks
of `BenchmarkShape`, `BenchmarkEndToEnd` and `BenchmarkEndToEndOnce`
changed by -3.5% to +1.2%. The fast path of the ligature sets tries fewer
ligatures, so `BenchmarkShape` is 2.8% faster on `pg1342` and 3.5% faster
on `english`. `BenchmarkEndToEndOnce` changed by +0.1% on `pg24264` and
by -2.5% on `pg1342`. It still shapes 1.000 rune for each rune of
`arabic`, so the workloads of `test/textbench` have no reversed run.

## When upstream changes

1. If go-text adds the fast paths of the rule sets and the ligature sets,
   or the flags of the reverse chaining lookups, compare them with this
   decision, and keep the behavior of HarfBuzz 15.1.0.
2. If HarfBuzz marks the clusters of a reversed run that a cut changes,
   `Line` can trust the flags of such a run again. Run `FuzzParagraphLine`
   with the tatweel first.
