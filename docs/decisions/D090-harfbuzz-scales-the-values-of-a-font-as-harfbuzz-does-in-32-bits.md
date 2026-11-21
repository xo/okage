# D90. harfbuzz scales the values of a font as HarfBuzz does, in 32 bits

Status: Decided, amends D81.

## Context

Fix 37 of D73 made `Kerning` of `tables.KerxData6` read a long value of a
subtable of format 6 of kerx with its 32 bits, as HarfBuzz 15.1.0 does. A
long value is a kerning value of 32 bits, which a subtable of format 6 holds
when bit 0 of its flags is set. `KernPair` of the interface
`font.SimpleKerns` returns an `int16`. So the kerning of `harfbuzz` still
keeps the low 16 bits of such a value, as go-text did. Item 4 of
"typesetting" in [`../BACKLOG.md`](../BACKLOG.md) leaves the choice to a
maintainer. No text of `perf_reference` and no font of the tests has such a
value. On 2026-11-21 the maintainers accepted option 1 of this decision,
answer 1 of question 41 of [`../PLAN.md`](../PLAN.md).

On 2026-11-21 the source of HarfBuzz 15.1.0 was read for this choice, at
`hb-kern.hh`, `hb-font.hh` and `hb-aat-layout-kerx-table.hh`. Each
`get_kerning` of kerx and kern returns an `int`. `hb_kern_machine_t` gives
that value to `em_scale_x` or `em_scale_y`, which take an `int32_t`. So
HarfBuzz keeps all 32 bits of a long value.

The same source shows a second difference, in the scale. HarfBuzz scales a
value of the font with `em_mult`:

```c
x_mult = (int64_t) x_scale * 0x10000 / upem;
hb_position_t em_mult (int32_t v, int64_t mult)
{ return (hb_position_t) ((v * mult + 32768) >> 16); }
```

This rounds the result to the nearest unit. `emScaleX` and `emScaleY` of
`harfbuzz` compute `Position(v) * f.XScale / f.faceUpem`, as go-text does.
This rounds the result toward 0. The two give the same value when the scale
is the units per em of the font. `NewFont` and `test/harfbuzz` use that
scale. So the 413,399 lines of `perf_reference` match. At another
scale, a value can differ by one unit. A negative kerning then becomes one
unit less negative. `emScaleX` scales the values of GPOS, the anchors and
the actions of kerx, the kerning of kern and kerx, and the carets of GDEF.

`shaping` sets the scale to the size of the text in units of 1/64, for
example 768 for a size of 12. So each `Paragraph` that ivy lays out has this
difference. On 2026-11-21 a test shaped Roboto with "AVATAWAYTo.LTA" at the
scales 1000, 1500 and 768. okage gave an advance one unit larger than
uharfbuzz 15.1.0 for 7, 7 and 4 of the 14 glyphs. With the rounding
of `em_mult`, all 14 glyphs matched at each scale. The test then shaped
each of the 28,248 lines of `pg24264` at the scale 768:

| Font | Glyphs that change | Lines that change | Largest change of a line |
| --- | --- | --- | --- |
| Roboto-Regular | 445 of 896,498 (0.05%) | 225 (0.8%) | 7/64 |
| Amiri-Regular | 667 of 896,490 (0.07%) | 256 (0.9%) | 12/64 |

The two parts touch the same functions. A path of 32 bits needs `emScaleX`
to take an `int32`, and the rounding of `em_mult` changes the body of
`emScaleX`.

## The decision

1. `emScaleX` and `emScaleY` of `harfbuzz` take an `int32`, and compute
   `em_mult` of HarfBuzz 15.1.0. The multiplier is
   `int64(scale) * 0x10000 / int64(upem)`, and the result is
   `Position((int64(v)*mult + 32768) >> 16)`. `Font` keeps its public
   fields `XScale` and `YScale`, and `shaping` sets them before each shape.
   So `Font` keeps the multiplier of each axis with its scale, as
   `x_mult` and `y_mult` of `hb_font_t`, and computes it again when the
   scale changes. The callers that pass an `int16` convert it, and the
   value does not change.
2. `KernPair` of `font.SimpleKerns` returns an `int32`. `Kern0`, `Kern2`
   and `Kern3` return their values of 16 bits as an `int32`. `Kern6`
   returns the value of `tables.KerxData6.Kerning` with all its bits. The
   function `kern` of `ot_kern.go` passes the value to `emScaleX` and
   `emScaleY` without a cast. The accelerators of kerx formats 0, 2 and 6
   in `harfbuzz` return it without a cast. This changes a public
   interface of the copy of go-text. No package of okage and no code of
   ivy implements `SimpleKerns`, and only `harfbuzz` and the tests of
   `font` call `KernPair`.

Each part has a test in `test/harfbuzz` that compares the glyphs with
uharfbuzz 15.1.0. `TestScale` shapes at the scales 768 and 1000: Roboto with
its GPOS, the first 30 lines of each text of `perf_reference` in each of its
seven fonts, and Roboto with kerx formats 0, 1 and 4 and with a table kern.
`TestKerxFormat6LongValues` uses a font built by hand with the long values
-100,000, 70,000 and 0x12345. `shape.py` takes the argument `--scale`.
`TestEmMult` of `harfbuzz` compares `emMult` with values of `em_mult`, and
checks that a `Font` computes the multiplier again when its scale changes.

## What the work found

With parts 1 and 2, `TestScale` still found glyphs one unit away from
HarfBuzz in four fonts. The answer to question 41 asks for a match at each
scale, so the work also makes these three changes. Each one gives the same
value at the units per em of the font, so `perf_reference` still matches:

3. `roundf` of `harfbuzz` rounds as `_hb_roundf` of HarfBuzz 15.1.0. In
   `hb-algs.hh`, HarfBuzz puts `floor((double) x + .5)` in place of
   `roundf`. So a half rounds toward +infinity, and -25.5 gives -25. The
   `roundf` of D81 rounds a half away from 0, as `math.Round` does. The
   cursive attachments of NotoNastaliqUrdu gave -26 in place of -25 at the
   scale 768. This amends item 2 of D81. `roundf` stays a sum and a
   conversion without a call, with one more compare for a negative value.
   `TestRoundf` compares it with `math.Floor` of the value plus one half,
   and a run over each float32 found no difference.
4. `emScalef` and `emFscale` multiply the value by the scale divided by
   the units per em, as `em_multf` and `em_fmult` do with `x_multf` and
   `y_multf`. go-text multiplied the value by the scale first. The two
   orders give the same float32 when the units per em are a power of 2.
   For 1,000 units per em and the sizes from 6 to 72, 4,058 of the
   4,390,912 values of 16 bits round to another unit.
5. `GlyphHAdvance` rounds the advance in the units of the font with
   `roundf`, clamps it at 0, and scales it with `emScaleX`, as
   `hb_ot_get_glyph_h_advances` and `get_advance_with_var_unscaled` do. The
   vertical advance does the same with `emScaleY`. go-text scaled the
   advance as a float, which gave Gulzar an advance of 337 in place of 336
   at the scale 768.

## The options

1. Parts 1 and 2. This matches HarfBuzz 15.1.0 at each scale and for each
   long value.
2. Part 1, and an optional interface for part 2. A new interface
   `font.LongKerns` has the method `KernPair32`, and only `Kern6`
   implements it. The function `kern` uses it when the subtable has it.
   `SimpleKerns` stays as go-text has it. Each pair of glyphs then pays for
   one more check of a type.
3. Part 1 alone. A long value of format 6 keeps its low 16 bits, as a
   deliberate difference from HarfBuzz. UPSTREAM.md records it.
4. Neither part. The positions of ivy stay as they are now.

## What changes for ivy

The decision changes the positions of the glyphs that ivy lays out, by at
most one unit of 1/64 for each value of the font. On 2026-11-21 a test
shaped each of the 28,248 lines of `pg24264` at the scale 768 with the code
of `main` and with this decision:

| Font | Units per em | Glyphs that change | Lines that change | Largest change of a line |
| --- | --- | --- | --- | --- |
| Roboto-Regular | 2048 | 445 of 896,498 (0.05%) | 225 (0.8%) | 7/64 |
| Amiri-Regular | 2048 | 667 of 896,490 (0.07%) | 256 (0.9%) | 12/64 |
| SourceSerifVariable-Roman | 1000 | 2,992 of 896,489 (0.33%) | 308 (1.1%) | 17/64 |

In rare cases, a line breaks at another place. The hashes of the output of
ivy change. Part 2 changes no glyph of a known font.

With 10 runs of the old and the new code in turns, on 4 CPUs, the 24
benchmarks of `test/textbench` changed by -2.3% to +2.9%, except
`BenchmarkEndToEnd/emoji`, at +5.4%. `BenchmarkEndToEndOnce` changed by
+0.0% on `pg1342` and by +1.3% on `pg24264`. A first version computed the
multiplier on each call, and made `BenchmarkEndToEndOnce/english` 7%
slower.

## When upstream changes

go-text has the same `emScaleX`, `emScalef`, `roundf`, `GlyphHAdvance` and
`SimpleKerns`. If go-text changes one of them, compare its change with this
decision, and keep the behavior of HarfBuzz.

## The recommendation

The recommendation is option 1. The rounding of `em_mult` is a fault of the
copy of go-text, in the kind that D73 fixes. Each part is a few lines with
a test against HarfBuzz, and the change of the interface reaches no caller
outside okage.
