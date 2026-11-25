# D100. svg follows the font model of read-fonts 0.46

Status: Decided, amends D32, amended by D101.

The resvg commit `04291b74` moved usvg to the font model of read-fonts
0.46, which skrifa 0.49 exports. `svg` followed its rules on go-text, and
three differences went to `docs/BACKLOG.md` under phase 10:

1. read-fonts keeps a metric with its MVAR delta as an F48Dot16, a number
   with 48 bits of integer and 16 bits of fraction. Go kept it as a float32.
2. The walk of the paint graph of COLRv1 in `internal/colr` ported skrifa
   0.44 (D32, D62). usvg walks the graph with the model of read-fonts 0.46.
3. read-fonts reads the strikes of the table EBDT. Go left them out, because
   usvg draws none of their glyphs.

On 2026-11-25 the maintainers chose to close the three differences, and not
to accept them.

## The oracle

read-fonts 0.46.0 and skrifa 0.49.0 are the tags `read-fonts-v0.46.0` and
`skrifa-v0.49.0` of the fontations repository, which is checked out at
`upstream/write-fonts`. Both tags name the same commit, `47a30161`. A small
Rust program, outside the repository, read fonts with that commit through
skrifa, as usvg does, and printed these values:

- each metric that `resolveFont` reads, as the raw value of its F48Dot16.
- the strikes of a font, and the bitmap that `select_bitmap` finds for each
  glyph.
- each call that `ColorGlyph::paint` makes on a `PaintSink`, with each
  float32 as the hex of its bits.

A probe test in Go printed the same lines, and a script compared them. The
probe is not in the repository. The tests that this decision adds hold the
values that the program printed.

## The decision

1. A metric is an `f48Dot16` in `svg/usvg_text_layout.go`, the port of
   F48Dot16 of font-types. `metricDeltas` adds the delta of MVAR in the
   48.16 format, and `resolveFont` converts a metric to a float32 only
   where usvg calls `to_f32`. `itemVariationDelta` ports
   `ItemVariationStore::compute_delta` of read-fonts 0.46, which adds the
   products in the 48.16 format and does not round the sum.
   `mvarMetricDelta` finds the record with the binary search of the Rust
   standard library.
2. `colr.PaintModel` in `internal/colr/model.go` ports the walk of
   read-fonts 0.46: `ColorGlyph::paint` of `src/model/glyph/color.rs`, and
   `traversal.rs`, `instance.rs` and `decycler.rs` of
   `src/model/glyph/color`. `svg` calls it. `pdf` still calls `colr.Paint`,
   the walk of skrifa 0.44, because krilla uses skrifa 0.44. The two walks
   share the brush of a fill, which both versions compute in the same way.
3. `bitmapStrikes` in `svg/usvg_text_cache.go` reads the strikes of EBDT
   after those of sbix and CBDT, as `BitmapStrikes::new` of read-fonts does.
   `bdtGlyph` ports `Strike::get`, `bitmap_data` and
   `BitmapGlyph::from_bdt` for CBDT and EBDT. A glyph of mask or BGRA
   pixels is a glyph, and `raster` does not draw it, as usvg does not.
4. No code of `typesetting` changes, so no comment `okage:` names this
   decision.

## What each change does

1. The metrics. With the same normalized coordinates, the Go metrics are
   the same as those of read-fonts, bit for bit, for 10 variable fonts at 14
   locations each. The fonts include RobotoFlex, Source Serif, Noto Sans,
   Estedad, Mada and a CFF2 font. The old code gave the same values for
   these fonts, because a float32 holds a delta below 256 units exactly. A
   larger delta shows the difference. `TestMVARDeltaF48Dot16` holds the MVAR
   table of a font that the script made from `SourceSans-VF.ttf` with
   fontTools: a delta of 32767 for "undo" and an underline position of
   -16285. At the coordinate 8193, read-fonts gives 100.49994, which usvg
   rounds to 100. The old code gave 100.5, which rounds to 101.
2. The walk. On the 3845 color glyphs of Noto Color Emoji, the 335 glyphs of
   Coral Pixels and the subset of the tests of resvg, both walks make the
   same calls as read-fonts. On the test font of COLRv1 of fontations,
   `test_glyphs-glyf_colr_1_variable.ttf`, the new walk makes the same calls
   as read-fonts for each of the 201 glyphs, at the default location and at
   four other locations. The old walk differs on the glyphs 178 and 179,
   which are cycles: it finds the cycle at another depth. Both fail, and
   usvg then draws no color glyph. go-text cannot parse the deltas of 32
   bits of that font, so the script made a copy whose deltas fit in 16 bits.
   `TestPaintModelReadFonts` holds a table that the script made, with these
   differences from skrifa 0.44:
   - A PaintGlyph whose chain of transforms ends in a PaintColrLayers is a
     clip, a transform and a fill for each layer. skrifa 0.44 gave a
     FillGlyph for each layer.
   - A variation index with no DeltaSetIndexMap names the outer index in
     its high 16 bits. skrifa 0.44 took the outer index 0.
   - A delta is an F48Dot16, applied as font-types applies it.
   - A walk visits at most 16384 paints, and the decycler compares a paint
     with the paint at half the depth only. The old Go walk compared it
     with each paint on the stack.

   `TestPaintSkrifa` makes sure that `colr.Paint` keeps the calls of skrifa
   0.44 for `pdf`. The tests of `pdf` against krilla do not change.
3. EBDT. KacstQurn has two strikes of EBDT, at 16 and 21 pixels per em. For
   each of its first 400 glyphs, `selectBitmap` finds the same strike, size
   and bearings as read-fonts. The results for the CBDT fonts and the sbix
   fonts of the tests do not change. usvg draws the outline of a glyph of
   EBDT, before and after. The CBDT fonts and the sbix fonts of the tests
   give the same bitmaps as read-fonts too. A strike of mask pixels in CBDT
   now hides a PNG in a smaller strike, as in read-fonts. No font of the
   tests has such a strike.

The tests of `svg` against resvg pass with each change.

## What stays different

These differences are outside the three items. `docs/BACKLOG.md` keeps the
first two under phase 10:

1. skrifa normalizes the coordinate of an axis in the 16.16 format and maps
   it with avar before it rounds it to 2.14. go-text normalizes it in
   float32, as HarfBuzz does. The coordinate of a face of `svg` can differ by
   one unit of 2.14, for example -7995 for -7994 at the weight 333 of
   `CFF2-VF.otf`. `internal/subsetter` already normalizes as skrifa does.
2. go-text cannot parse an ItemVariationData whose deltas have 32 bits, the
   flag LONG_WORDS. It then loads no table COLR, and the font has no color
   glyphs. read-fonts reads such deltas.
3. read-fonts takes the offset of a paint as its id for the decycler. go-text
   gives no offsets, so both walks take the index of a layer, or the glyph
   of a base glyph, as D32 says.
