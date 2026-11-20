# D73. typesetting fixes the faults that go-text still has

Status: Decided.

`typesetting` copies go-text with its faults (D52). Four faults are known in
`docs/BACKLOG.md`, and go-text has each one up to its commit `bafa8730`. The
pending commits of go-text, which okage applied on 2026-11-20, fix none of
them:

1. `harfbuzz` places a mark on some ligatures of a short text of Amiri in
   another place than HarfBuzz 15.1.0.
2. `FuzzBreakOptions` finds that the line breaker of `shaping` and the
   segmenter count the graphemes of a text in other ways.
3. `FuzzE2EVariableSize` finds that `WrapParagraph` gives more runes than
   its input in one configuration.
4. `typesetting` cannot read an anchor of a lookup MarkLigPos of
   `NotoSans_variable.ttf`, and drops the whole table GPOS of the font
   ("The PDF writer" in `docs/BACKLOG.md`).

The maintainers want these faults fixed in okage.

On 2026-11-20 the maintainers added two faults that the work on the first
four found:

5. `typesetting` drops the whole table GSUB or GPOS when one subtable of a
   lookup has an error. HarfBuzz sets the offset that fails to null, up to
   32 offsets for each table, and keeps the rest of the table.
6. `BenchmarkShaping` of `harfbuzz` does not end on the text
   `perf_reference/texts/duployan.txt` in the font
   `NotoSansDuployan-Regular.otf`, in okage and in go-text `bafa8730`.

On 2026-11-20 faults 2 and 3 turned out to be faults of the fuzz tests of
go-text, not of the library. For fault 2, UAX #14 allows a line break after
a space before a combining mark, and UAX #29 keeps the space and the mark in
one grapheme cluster, so a line can end inside a cluster. The test counted
the clusters of the line alone. For fault 3, the test counted the runes of
the truncator of the last line twice. okage fixed both tests, and changed no
code of the library.

## The decision

1. `typesetting` first applies the pending commits of go-text (D52). A fault
   that a commit of go-text fixes needs nothing more.
2. For each fault that remains, okage fixes it in the copy. The fix follows
   HarfBuzz for the shaper, the Unicode standard for the segmenter and the
   line breaker, and the OpenType specification for the parse of a font.
3. Each fix has a test that fails before the fix, and a comment that starts
   with `okage:` and names this decision.
4. A fuzz target that leaves out an input because of a fault, such as the
   marks of the Arabic alphabet of `FuzzUnsafeToBreak` (D70), takes that
   input again when the fault is fixed.
5. A fix that changes the output of a test of `svg` or `pdf` follows the
   oracle of that test: resvg for `svg` and krilla for `pdf`.

## The fixes

1. Fault 1: a lookup MarkBasePos or MarkLigPos of `harfbuzz` keeps the
   base glyph that it found for the next mark. go-text kept that base from
   one lookup to the next. HarfBuzz clears it when a lookup starts, in
   `set_lookup_mask`. In Amiri, GSUB adds a tatweel after a lam. One lookup
   attaches the mark to the tatweel, and a later lookup must skip the
   tatweel and attach the mark to the lam. With the base of the first
   lookup, the second lookup did not attach the mark. In a longer text, a
   later mark cleared the base, so the fault showed only in a short text.
   `harfbuzz` now clears the base when a lookup starts. `TestMarkBaseOfLookup`
   tests the two texts of the fault. A comparison shaped 120,000 random
   texts of the Arabic alphabet of `FuzzUnsafeToBreak`, with its marks.
   HarfBuzz 15.1.0 and `harfbuzz` give the same glyphs, clusters, advances
   and offsets for each text.
2. Fault 4: lookup 35 of `NotoSans_variable.ttf` covers 287 marks, but its
   MarkArray has 281 records, and 6 records have a null anchor offset. The
   font does not follow the OpenType specification, which gives each mark a
   record and an anchor. go-text read a null offset as the start of the
   MarkArray, so its count, 281, was the format of an anchor. The sanitizer
   of go-text also refused the counts that differ. Each error dropped the
   whole table GPOS. HarfBuzz reads such a font. A null offset is its Null
   anchor at (0, 0), and a mark without a record has its Null record, with
   the class 0. `typesetting` now does the same. `TestMarkArrayNullAnchor`,
   `TestNotoSansVariableGPOS` and `TestMarkWithoutRecord` test it, and
   `TestTextGoVariable` of `test/krilla` compares the font with krilla
   again.
3. Fault 6: `NotoSansDuployan-Regular.otf` makes about 60 glyphs of each
   character. go-text gave the buffer 1024 operations for each character,
   at least 16,384. HarfBuzz 15.1.0 gives 4096, at least 65,536, in
   `hb-limits.hh`. The limit of the length of the buffer was 64 times the
   text, at least 16,384, and HarfBuzz gives 256 times, at least 65,536.
   Each line of `duployan.txt` used all its operations in go-text. A
   context lookup then applied without a move in the buffer, and the loop
   of the lookup did not end. HarfBuzz stops that loop when the buffer is
   not `successful`. `harfbuzz` now has the limits of HarfBuzz 15.1.0, and
   it stops the loop when the buffer has no operations left.
4. The comparison of fault 6 with HarfBuzz found one more fault. A reverse
   chaining lookup of GSUB works in place, without an output buffer.
   go-text read the glyphs before the current glyph from the output
   buffer, which still held the glyphs from before the last lookup.
   HarfBuzz reads the glyphs of the buffer, and `harfbuzz` now does the
   same. After these fixes, each of the 14 lines of `duployan.txt` gives
   the same glyphs, clusters, advances and offsets in `harfbuzz` and in
   HarfBuzz 15.1.0. The first 200 lines with text of
   `fa-thelittleprince.txt` give the same output in
   `NotoNastaliqUrdu-Regular.ttf` and `Gulzar-Regular.ttf`.
   `BenchmarkShaping` shapes all of `duployan.txt` in one buffer in 5.2
   seconds, and HarfBuzz takes 1.1 seconds. `TestShapeDuployan` and
   `TestShapeWithoutOperations` test the fixes. go-text counts some
   operations in other ways than HarfBuzz. So a text can still use all its
   operations in `harfbuzz` and not in HarfBuzz. 5 of 20,000 random texts
   of 2 to 15 characters of the Duployan block do, and their glyphs differ
   from HarfBuzz.
5. okage did not change the code for fault 5. HarfBuzz stopped setting an
   offset that fails to null in its commits `83dc4d1d` and `665afc08` of
   November 2025. Since then, HarfBuzz drops the whole table when a
   subtable has an error, as `typesetting` does. A test set the offset of
   the subtable of lookup 199 of GSUB in `Amiri-Regular.ttf` to a place
   after the end of the table. HarfBuzz 15.1.0 then reads no script of
   GSUB, and `harfbuzz` gives the same glyphs as HarfBuzz for two Arabic
   texts. The maintainers then decided fault 5, as the end of this part
   says.

On 2026-11-20 the maintainers added two more faults:

7. `harfbuzz` places the glyphs of `perf_reference/texts/fa-thelittleprince.txt`
   in the font `Amiri-Regular.ttf` of `perf_reference` in another place than
   HarfBuzz 15.1.0, on 115 of the first 200 lines. The glyphs and the
   clusters are the same.
8. `harfbuzz` counts the operations of a shaping in another way than
   HarfBuzz, so that a few texts of Duployan run out of operations in okage
   and not in HarfBuzz. `maxNestingLevel` is 6 in `harfbuzz` and 64 in
   HarfBuzz.

On 2026-11-20 the maintainers decided fault 5: when one subtable of GSUB or
GPOS is bad, `typesetting` follows HarfBuzz. HarfBuzz 15.1.0 drops the whole
table, as `typesetting` already does, so the code does not change. If
HarfBuzz changes this logic again, okage follows it.

On 2026-11-20 okage fixed faults 7 and 8:

6. Fault 7: lookup 74 of GPOS in `Amiri-Regular.ttf` of `perf_reference`
   holds a PairPos of format 2 with the class counts 80 and 67. Its two
   class definitions have 79 and 66 classes. The sanitizer of go-text
   required each count to be the number of classes of its class
   definition. So it dropped the whole table GPOS of the font, and the
   texts had no kerning. HarfBuzz
   requires only that the subtable holds the records of the counts. Its
   lookup does not apply to a class outside the counts. `typesetting` now
   does the same, with the methods `Class1Count` and `Class2Count` of
   `PairPosData2`. `TestPairPosClassCounts` of `font` and of `harfbuzz`
   test it. After the fix, each of the 771 lines with text of
   `fa-thelittleprince.txt` gives the same glyphs, clusters, advances and
   offsets in `harfbuzz` and in HarfBuzz 15.1.0. The same is true for each
   of the 7 fonts of `perf_reference` with each of the 7 texts, a line at
   a time: 413,399 lines.
7. Fault 8: `harfbuzz` now takes the operations where HarfBuzz 15.1.0
   takes them, and has its nesting level of 64:
   - `setGlyphFlags` returns at once for a range whose start, in the
     output buffer, is after its end. HarfBuzz does the same, because it
     computes the length of the range without a sign. go-text set the
     flags and took operations for them. This was the main cause.
   - Each nested lookup takes one operation. go-text took it only when the
     lookup did not apply. A nesting deeper than 64 stops the shaping.
   - A merge of clusters takes one operation for each glyph that it
     changes or extends over, also in the output buffer and when a glyph
     is deleted. It does not extend when the buffer has no operations
     left.
   - `maxNestingLevel` is 64. It also limits the chain of attachments that
     the offsets follow, as in HarfBuzz. go-text followed the whole chain.
   `TestShapeDuployanOperations`, `TestSetGlyphFlagsBeforeStart`,
   `TestRecurseOperations`, `TestMergeClustersOperations` and
   `TestAttachmentNesting` test the changes. A comparison shaped 20,000
   random texts of 2 to 15 characters from the 147 characters of the
   Duployan block, the format controls included. Before the fix, 3,119
   texts gave another output than HarfBuzz, and 2,410 of them other
   glyphs. After the fix, each text gives the same glyphs, clusters,
   advances and offsets as HarfBuzz. 547 more texts added one or two
   characters to the two texts that use the most operations. `harfbuzz`
   and HarfBuzz run out of operations on the same 12 of them, and on no
   other. `TestShapeWithoutOperations` now shapes one of the 12, because
   its earlier text no longer runs out. Each nested lookup now costs an
   operation, so a hostile font with deep nesting stops at the limit of
   operations. Item 4 of "typesetting" in `docs/BACKLOG.md` lists the
   differences from HarfBuzz that remain, which no test text shows. With
   the flags of HarfBuzz, `FuzzParagraphLine` of `shaping` finds a
   difference in the flag unsafe-to-break, which item 5 records.

On 2026-11-20 okage made `harfbuzz` follow HarfBuzz 15.1.0 at the five
places that the fix of faults 7 and 8 found different:

8. In a PairPos of format 2, a second glyph that ClassDef2 does not hold
   has the class 0, and the subtable applies. go-text did not apply the
   subtable, and tried the next one. `TestPairPosClassZero` tests it. The
   record of the class 0 then takes time, and a fast path of
   `PairPosData2.Record` for a record of the advance alone keeps the time
   of the shaping.
9. The buffer has the state `inError`, which is true when the buffer of
   HarfBuzz is not `successful`. The buffer is in error when it has no
   operations left, or when a nesting is too deep. The loops of the
   lookups, of the normalizer and of the shapers stop on an error where
   HarfBuzz stops. At the end of a lookup in error, `swapBuffers` drops
   the output of the lookup, as `sync` of HarfBuzz does. As in HarfBuzz,
   the output shares the array of the input until the output is longer
   than the input that it consumed. So the buffer keeps the glyphs that
   the lookup wrote into that array. go-text kept the output.
   `TestDropOutputWithoutOperations` and `TestSwapBuffersInError` test it.
10. The buffer is in error when its arrays must grow past the limit of
    the length. HarfBuzz compares the length with the limit only when its
    arrays grow, by half their length and 32 glyphs, so a buffer can hold
    more glyphs than the limit. `harfbuzz` keeps the length of the arrays
    of HarfBuzz, and no longer compares the length before each lookup.
    `TestBufferLengthLimit` tests it.
11. Each subtable of morx, kerx and kern takes the length of the buffer in
    operations, and the table stops when the buffer is in error. An
    insertion of morx takes its operations before it compares the count,
    and a control point action of kerx takes 512 operations.
    `TestMorxSubtableOperations` tests the operations of the subtables.
12. When the Indic shaper asks whether a lookup substitutes glyphs, the
    question takes one operation for the lookup and one for each of its
    subtables, one for each rule of a rule set, and one for each ligature
    of a ligature set. The answer is false when the buffer is in error.
    go-text took no operation. `TestIndicWouldSubstituteOperations` tests
    it.

On 2026-11-20 `test/textbench` compared the shaping before and after
these fixes, with 8 runs of each benchmark in turn. Most workloads are 1%
to 5% faster. Devanagari is 2% to 4% slower, because of the operations of
the questions of the Indic shaper, and the variable font is about 3%
slower, because of the records of the class 0.

The test module compares the font of each of these tests with
uharfbuzz 15.1.0, in `test/harfbuzz`. Each of the 413,399 lines of
`perf_reference` still gives the same glyphs, clusters, advances and
offsets as HarfBuzz. The work found three more differences, which
item 4 of "typesetting" in `docs/BACKLOG.md` records.

On 2026-11-20 the maintainers decided one of the three: when the Indic
shaper asks whether a context or a chained context of format 2 or 3
substitutes glyphs, `harfbuzz` does not test the coverage of the first
glyph, as in HarfBuzz 15.1.0. go-text tested it, and its test
`TestWouldApplyContextRequiresFirstGlyphCoverage` now asks for the answer
of HarfBuzz. `TestIndicFirstGlyphCoverage` tests it, in `harfbuzz` and in
`test/harfbuzz`.

On 2026-11-20 okage made `harfbuzz` follow HarfBuzz 15.1.0 at the other
two places of item 4:

13. `typesetting` refuses a table kern, kerx or morx with more than 1,024
    subtables, as `HB_AAT_MAX_SUBTABLES` of `hb-limits.hh`. In morx, the
    limit counts the subtables of all the chains. `ParseKern` returns an
    error before it allocates the subtables, so a table kern of Apple
    that declares 4 billion subtables no longer allocates memory for
    them. `font` drops a table kerx or morx with too many subtables.
    `typesetting` reads no table mort. `TestAATSubtableLimit` tests it.
14. A subtable of format 1 of kerx follows the code of HarfBuzz 15.1.0,
    which follows CoreText. The stack is a history of the last eight
    glyphs that the subtable pushed. Each list of actions walks it again
    from the last glyph, and does not empty it. A slot that no glyph
    filled holds the first glyph of the buffer. A list ends with the
    value 0xFFFF, or at the end of the values. An odd value does not end
    it. Each action takes one operation, and marks the glyphs from the
    changed glyph to the current glyph unsafe to break. The sum of a
    value and a position is clamped to the range of a position. The
    values of a subtable go up to the end of the subtable, and up to the
    end of the table for the last subtable, whose length HarfBuzz
    ignores. An entry whose list starts after the end of the values is no
    longer an error of the parse. As in HarfBuzz, such a list empties the
    stack when the subtable reads it.
15. A subtable of format 1 of kern keeps the stack of go-text, which a
    list empties, and whose list ends with an odd value, as in HarfBuzz.
    HarfBuzz ignores the flag Reset in kern and kerx. In kern, the bit
    0x2000 of the flags is a bit of the offset of the list, and go-text
    read it as Reset. `harfbuzz` now ignores the flag too.

`TestKerxFormat1History`, `TestKerxFormat1Operations`,
`TestKerxFormat1Bounds` and `TestKernFormat1Offset` test fixes 14 and 15,
in `harfbuzz` and in `test/harfbuzz`. `shape.py` of `test/harfbuzz` also
writes the flag unsafe-to-break when a test asks for it. A comparison
shaped 60,000 random texts of a, b, c and x in 3,000 random subtables of
format 1 of kerx, with the flags Push, Reset and DontAdvance, and lists
of random values and ends. `harfbuzz` and HarfBuzz gave the same glyphs,
clusters, advances, offsets and flags unsafe-to-break for each text. The
comparison left out the subtables that HarfBuzz skips and `harfbuzz`
does not, which item 4 of "typesetting" in `docs/BACKLOG.md` records. No
workload of `test/textbench` uses a table kern, kerx or morx, so their
times do not change.

## When upstream changes

If go-text fixes one of these faults, follow these steps:

1. Apply the commit of go-text as D52 says.
2. If the commit gives the same result, remove the fix of okage, and keep
   its test.
3. If the result is not the same, compare both with HarfBuzz or with the
   specification, and keep the one that matches. A maintainer decides when
   neither one matches.

## Why

The faults give wrong glyphs, wrong line breaks or no kerning, and the PDF
files of `pdf` then differ from the files of krilla. go-text has not fixed
them, and okage can fix them with small changes that keep a test.
