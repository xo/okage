# D84. typesetting computes the deltas of variable fonts in fixed point

Status: Decided, amends D65.

`internal/subsetter` makes the instances of variable fonts with
`typesetting`, the copy of go-text in okage (D52), where subsetter makes
them with skrifa (D62). `typesetting` computed the deltas of the variations
in float32, as go-text and HarfBuzz do. skrifa computes them in the 16.16
fixed-point numbers of OpenType, as FreeType does. So some points of the
instances of `test/krilla` differed by 1 unit from the instances of
subsetter. D65 accepted that within a tolerance: at most 0.1% of the glyphs
of the instances of a font can differ by more than 1 unit. Item 1 of "The
PDF writer" in [`../BACKLOG.md`](../BACKLOG.md) asked for the instances of
subsetter. This decision makes `typesetting` compute the deltas as skrifa
does. It changes the copy of go-text, so it needs the approval of the
maintainers (D52). On 2026-11-21 the maintainers accepted the three
commits, answer 1 of question 36 of [`../PLAN.md`](../PLAN.md). The
tolerance of D65 goes away, and `TestSubsetterVariable` requires the
instances of subsetter, byte for byte.

## What skrifa does

skrifa 0.42 and read-fonts 0.39, the versions of the `Cargo.lock` of
krilla that subsetter uses, compute these values. A number in 16.16 is an
int32 with 16 bits of fraction. Its product and its quotient are rounded to
the nearest number.

1. The coordinates of a location. skrifa converts the value of the user to
   16.16, normalizes it with a rounded division, maps it with the table
   avar in 16.16, and rounds it to 2.14 at the end. `typesetting` and
   HarfBuzz round the normalized value to 2.14 first, and then map it with
   avar.
2. The scalar of a tuple of gvar and of a region of an item variation
   store. skrifa multiplies the factors of the axes one at a time, with one
   rounded `mul_div` for each axis.
3. The deltas of gvar. Each delta of a tuple is the product of the delta of
   the font and the scalar, rounded. The deltas of the tuples are added in
   16.16. A point that a tuple gives no delta is interpolated between two
   points in 16.16, with a rounded quotient and a rounded product, as the
   IUP of FreeType does.
4. An outline without a scale. skrifa adds the delta of each point,
   rounded to an integer, to the point. A component gets the delta of its
   offset, rounded to an integer. The outline then moves by its first
   phantom point.
5. An item variation store, which HVAR, MVAR and the device tables of GPOS
   read. skrifa adds the products of the deltas and the scalars in 64 bits,
   and rounds the sum to an integer.
6. The blends of CFF2. skrifa runs the char string in 16.16, and rounds the
   product of each delta and its scalar.

HarfBuzz 15.1.0 computes the scalar of a region in float, and rounds the
delta of an advance of HVAR to an integer with `roundf`. HarfBuzz 8.5 also
computes the deltas of gvar and the blends of CFF2 in float. It rounds a normalized coordinate to 2.14 before
it maps the coordinate with avar, as `typesetting` does.

## The prototype

The branch of the prototype holds three commits. Each change of the copy
of go-text has a comment that starts with `okage:` and names this decision.

1. `852fee2` changes `typesetting`. The new type `tables.Fixed` holds a
   number in 16.16, with the rounded `Mul`, `Div` and `MulDiv` of
   font-types. `VariationRegion.EvaluateFixed` gives the scalar of a region
   as read-fonts does. The scalars of gvar, the deltas of each tuple, the
   interpolated deltas, the sum of `ItemVarStore.GetDelta` and the blends of
   CFF2 are in 16.16. A point of gvar gets the sum of its deltas at the
   end. `GetDelta` does not round its sum to an integer, so that a device
   table of GPOS keeps the fraction of its delta, as in HarfBuzz.
2. `65899d8` changes `internal/subsetter`. It computes the coordinates of
   a location as skrifa does, with the new `SegmentMaps.MapFixed` of
   `typesetting`. The normalization of `typesetting` for the shaper does
   not change, and still follows HarfBuzz.
3. `4a6103f` adds `Face.GlyphVariationDeltas` to `typesetting`, a new
   public method. It gives the deltas of gvar of a glyph in 16.16. The
   outline of `GlyphDataOutline` holds its coordinates in float32, which
   cannot hold a coordinate above 256 units with the exact fraction of its
   delta. `internal/subsetter` adds each delta, rounded, to its point, to
   the offset of its component and to the phantom points, as skrifa does.
   It takes the delta of HVAR from the table. A point of CFF2 still comes
   from the float32 of `CFF2.LoadGlyph`, and a delta of HVAR from the
   float32 of `GetDelta`. A float32 holds a number in 16.16 exactly only
   below 256 units. Above, a point of CFF2 very near an integer, or a delta
   of HVAR very near half a unit, can round the other way. No instance of
   `test/krilla` has such a value.

The conversions of D71 on the lines of the deltas of gvar, of
`GetDelta` and of the blends of CFF2 are gone, because these lines no longer
multiply floats. The new sums in float have an explicit conversion.

## The measurements

The measurements are of 2026-11-20, on an AMD Ryzen 9 9950X.

### The instances of test/krilla

`TestSubsetterVariable` makes 629 instances of four fonts. This table gives,
for each step, the instances that are the same as the instances of
subsetter, byte for byte, the points that differ by 1 unit, the glyphs that
differ by more than 1 unit, and the advances that differ by 1 unit.

| Font | Before | Commit 1 | Commit 2 alone | Commits 1 and 2 | Commits 1, 2 and 3 |
| --- | --- | --- | --- | --- | --- |
| `Cantarell-VF.otf`, CFF2, 110 cases | 106 the same, 471 points, 21 glyphs, 0 advances | 106, 471, 21, 0 | 110, 0, 0, 0 | 110, 0, 0, 0 | 110, 0, 0, 0 |
| `NotoSans-Regular_var.ttf`, 135 cases | 105, 4,184, 1, 67 | 116, 3,892, 1, 89 | 116, 914, 0, 20 | 127, 53, 0, 2 | 135, 0, 0, 0 |
| `RobotoFlex.subset.ttf`, 312 cases | 207, 705, 0, 15 | 271, 352, 0, 14 | 222, 432, 0, 1 | 303, 42, 0, 0 | 312, 0, 0, 0 |
| `Selawik-VF-Subset.ttf`, 72 cases | 72, 0, 0, 0 | 72, 0, 0, 0 | 72, 0, 0, 0 | 72, 0, 0, 0 | 72, 0, 0, 0 |

With the three commits, each of the 629 instances is the same as the
instance of subsetter, byte for byte. The coordinates alone give the
instances of the CFF2 font: its 21 glyphs with another number of quadratic
curves came from coordinates that differed by 1/16384. The last points
that differ with commits 1 and 2 have a coordinate above 256 units whose
fraction is very near half a unit. The float32 of the outline rounds it to
half a unit, and it then rounds to the other integer.

### The images of svg

`TestRender` and `TestExtra` of `svg` render 1,810 images of resvg. Each
pixmap is the same, byte for byte, before and after the three commits. The
14 tests of `text/font-variation-settings` that differ from resvg still
differ by 1 to 68 pixels, 576 pixels in all. The outline of `typesetting`
keeps the fraction of each delta. The new arithmetic moves a point by a
small fraction of a unit, which is far less than a pixel at the size of
these tests. The tests of the test module pass, with the same results.

skrifa rounds each point of a TrueType outline at a location to an
integer, as item 4 above says, and usvg draws that outline. A trial that
rounded the points of such an outline in `internal/glyphoutline` cut the
576 pixels of the 14 tests to 191. One test, `opsz-144`, went from 1 pixel
to 25. The prototype does not hold this trial.

### Shaping

`test/harfbuzz` still matches uharfbuzz on each of the 413,399 lines of
perf_reference. perf_reference shapes its variable font at the default
location only, where this decision changes nothing. A comparison for this
decision shaped the first 150 lines of each text of perf_reference in 8
variable fonts at 4 to 53 locations each. 966 of the 98,712 lines differ
from HarfBuzz 15.1.0, and they are the same lines before and after the
three commits. They come from other faults: the substitutions of the
feature variations of `RobotoFlex.subset.ttf` and `Commissioner-VF.ttf`,
and positions of GPOS that differ by 1 unit in `Mada-VF.ttf`,
`NotoSans-Regular_var.ttf` and `Estedad-VF.ttf`.

With this decision, `typesetting` follows skrifa for the deltas, and
HarfBuzz for the coordinates of the shaper and for the fraction of a delta
of GPOS.

### Speed

The benchmarks of `test/textbench` ran 8 times before and after the three
commits, one after the other. The workload `variable`, English in Roboto
Flex at a location, got faster: `BenchmarkShape` by 25%,
`BenchmarkEndToEnd` by 24% and `BenchmarkEndToEndOnce` by 23%, in a second
run of 12 times. The likely cause is that the scalar of a region in 16.16
skips each axis whose peak is 0 before it computes anything, and Roboto
Flex has 13 axes. The other
workloads do not use a variable font. Their medians moved by -3.6% to
+1.5%, inside the spread of the runs.

A benchmark for this decision drew the outline and took the advance of
each glyph of three variable fonts at a location. It got slower: by 18%
for `NotoSans-Regular_var.ttf`, by 7% for `RobotoFlex.subset.ttf` and by 8%
for the CFF2 font `Cantarell-VF.otf`. The deltas of gvar take a second
slice for their sum, and a product or a quotient in 16.16 takes more
instructions than in float32. The prototype does not try to make them
faster.

## The choices

1. Accept the three commits. The instances of `internal/subsetter` are the
   instances of subsetter, byte for byte. `typesetting` gets the new public
   method `Face.GlyphVariationDeltas`. `TestSubsetterVariable` can then
   require the same bytes, and the tolerance of D65 goes away.
2. Accept commits 1 and 2. No point differs by more than 1 unit, and 95
   points differ by 1 unit. The tolerance of D65 stays.
3. Accept commit 2 alone, and move `MapFixed` into `internal/subsetter`.
   The copy of go-text does not change. The CFF2 font gives the instances
   of subsetter, and 1,346 points of the TrueType fonts differ by 1 unit.
   The tolerance of D65 stays.
4. Accept none of them, and keep the tolerance of D65.

The recommendation is answer 1. It gives the instances of the oracle, and
the outlines and the shaping of `typesetting` do not change in a way that a
test can see.

The trial of the rounded outlines in `internal/glyphoutline` is a separate
question, and this decision does not ask it. The trial follows skrifa in
`svg` and `pdf`. It moves the outlines of `svg` away from the outlines of
HarfBuzz, and one test of resvg gets worse.

## When upstream changes

1. When okage applies a pending commit of go-text that changes one of these
   functions, keep the change of okage, as "typesetting" in
   [`../UPSTREAM.md`](../UPSTREAM.md) says.
2. If go-text computes the deltas in fixed point in the same way, follow
   go-text, and remove the comments `okage:` of this decision.
