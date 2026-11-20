# D70. shaping gives the flags of HarfBuzz and reuses its glyphs

Status: Decided.

ivy lays out its documents with `typesetting/shaping`, the copy of go-text
in okage (D52). On 2026-11-19 the layout of
`gutenberg/html/pg1342-images.html` spent half of its time in the shaper.
The layout of ivy breaks a text into lines as WeasyPrint does. It shapes
the start of a text to find the first line, shapes the first line again,
and then shapes the rest of the text for the next line. So it shapes each
character about 4.5 times. Each call of the shaper also allocates a new
slice of glyphs, and HarfBuzz allocates in its lookups. The shaper made
23% of the bytes that ivy allocated for this document.

HarfBuzz marks each glyph whose cluster is unsafe to break. A cluster is
safe to break when the text that breaks at its start, shaped as two parts,
gives the same glyphs as the whole text. A layout can then take the glyphs
of a line from the glyphs of the whole text, and shape the text only once.
The harfbuzz package of go-text computes these flags, and `shaping`
copies them into `Glyph.Mask`, but go-text does not say so. Each change of
`typesetting` has a comment that starts with `okage:` and names this
decision.

## The flags

1. The comment of `Glyph.Mask` says that it holds the glyph flags of
   HarfBuzz for the cluster of the glyph. The function `propagateFlags` of
   `harfbuzz` leaves only the flags in the mask, and gives each glyph of a
   cluster the same flags, as HarfBuzz does. So the output does not change.
2. `Glyph.UnsafeToBreak` reports whether the mask holds
   `harfbuzz.GlyphUnsafeToBreak`, the flag `HB_GLYPH_FLAG_UNSAFE_TO_BREAK`
   of HarfBuzz.
3. The shaper does not set the buffer flag `ProduceUnsafeToConcat`, so the
   mask never holds `GlyphUnsafeToConcat`. okage adds no option for it
   until a program needs it.

## The glyphs that the shaper reuses

1. `HarfbuzzShaper.ShapeInto(input Input, buf []Glyph) Output` is
   `HarfbuzzShaper.Shape`, but it writes the glyphs into the memory of
   `buf` when `buf` has the capacity for them. A caller that reads the
   glyphs of each output before it shapes the next text gives the glyphs of
   the last output as `buf`. The shaper then allocates no glyphs.
2. `Shape` and `ShapeNoExtents` do not change.

## The commits of go-text that okage applies out of order

go-text fixed most of the allocations of its lookups in five commits.
`typesetting` applies the commits of go-text in order (D52), and they were
pending. okage applies them now, in the order of the ledger, as D67 did for
the fixes of malformed input:

1. `c4f9c9e6`, which keeps the fallback plan of Arabic on the plan of the
   shaper.
2. `09d8f6df`, which makes the matchers of the contexts values, not
   closures.
3. `11522e58`, which stops the allocations of the nested lookups.
4. `11785748`, which reads the value records of pair positioning without
   an allocation.
5. `64922d2b`, which reads the anchors of the base, ligature and mark2
   glyphs without an interface value.

Each commit applies cleanly, and its row in `docs/upstream/typesetting.tsv`
is `ported`. The column `matched` does not move, because older commits are
still pending (D12). The test files of `11522e58` and `64922d2b` are not in
the copy, so they are left out (D52). After these commits, 69 commits are
pending.

## The tests

1. `TestUnsafeToBreak` shapes texts in Roboto, Ubuntu Mono and Amiri,
   Latin and Arabic. At the start of each cluster that is safe to break, it
   shapes the text in two parts, each with the rest of the text as its
   context. The glyphs of the parts must be the glyphs of the whole text.
   The option `--verify` of `hb-shape` makes sure of the same.
2. `FuzzUnsafeToBreak` does the same for a text that the fuzzer makes from
   an alphabet that each font has.
3. `TestUnsafeToBreakKerning` tests the flag of a pair that Roboto kerns.
4. `TestShapeInto` makes sure that `ShapeInto` gives the output of `Shape` in
   the memory of `buf`, with fewer allocations.
5. `BenchmarkShape` measures `Shape` and `ShapeInto` on a paragraph.

## How the flags match HarfBuzz

On 2026-11-19 a program outside the repository shaped a corpus with
`shaping` and with HarfBuzz 15.1.0 through cgo, and compared each glyph.
The corpus is 400 paragraphs of `pg1342-images.html` in ten Latin fonts,
60 paragraphs of `pg24264.txt` in Droid Sans Fallback, and texts in
Arabic, Hebrew, Devanagari, Bengali, Thai, Khmer and Myanmar in Noto fonts
and Amiri. Its 761,000 glyphs have the same glyph, cluster, advance,
offsets and flags in both. The same program shaped the corpus with okage
v0.17.0 and with this change, and each field of each output is the same.

Two faults stay:

1. HarfBuzz itself does not mark every cluster that is unsafe to break.
   For example, it decomposes U+01EC in Amiri in one way when a mark is
   elsewhere in the text, and in another way when no mark is, and the flag
   does not show it. `shaping` follows HarfBuzz.
2. `FuzzUnsafeToBreak` found a fault of go-text that okage has from
   v0.3.5: in Amiri, the position of a mark such as U+064E on some
   ligatures, such as lam and kaf, differs from HarfBuzz when the text is
   short. The flags are not the cause. D73 fixes it, and the Arabic
   alphabet of the fuzz test holds the marks again.

## The cost

`BenchmarkShape` shapes a paragraph of 2,500 characters in Roboto. Before
this change, `Shape` made 6,591 allocations and 237,402 bytes in 1.01 ms.
After it, `Shape` makes 4 allocations and 205,153 bytes in 0.92 ms, and
`ShapeInto` makes 3 allocations and 447 bytes in 0.80 ms.

## When upstream changes

1. If go-text gives the flags or a buffer of glyphs in another way, follow
   go-text, and remove the comment `okage:` and the item of this decision.
2. When okage applies the commits of go-text in order, the five commits
   above are already `ported`, and okage skips them.

On 2026-11-19 the maintainers accepted this decision, and answered
question 30 with answer 1.
