# D77. shaping shapes a paragraph once for its lines

Status: Decided, amends D70.

ivy breaks a text into lines as WeasyPrint does. For each line, it shapes a
text that starts at the start of the line, finds the break, and takes the
glyphs of the line. Since D70, it takes the glyphs of a line from an earlier
output when the cut is safe to break. On 2026-11-20, after D72,
`test/textbench` measured two costs that remain in this path:

1. The shaper takes 51% of the time of `BenchmarkEndToEnd` on `pg24264` and
   69% on `pg1342`. The benchmark shapes 1.52 and 1.73 characters for each
   character of the book, because a text that starts or ends inside an
   earlier text is shaped again.
2. `segmenter.computeBreakAttributes` takes 19% and 13% of the time,
   because the program asks for the break attributes of the same paragraph
   again for each line.

The maintainers want `shaping` to do both once for each paragraph.

## The decision

1. `shaping` adds a type that holds a paragraph shaped once: its runes, its
   runs with their glyphs, and its break attributes. A program makes it once
   for each paragraph, with the faces, the size and the direction of each
   run, as for `Shape`.
2. The type gives the glyphs of a range of runes of the paragraph, such as
   one line. It takes the glyphs from the shaped paragraph where the cut at
   each end of the range is safe to break, as the flag unsafe-to-break of
   HarfBuzz says (D70). It shapes again only the glyphs next to a cut that is
   not safe to break, and gives the same glyphs as a new shaping of the
   range. The flag unsafe-to-break alone is not enough for this: in Amiri, a
   hamza changes its glyph with the text before it. So the private shaper of
   the type also sets `ProduceUnsafeToConcat`, and follows the method of
   HarfBuzz for the flag unsafe-to-concat at a cut that is not safe. The
   type removes that flag from the glyphs that it gives, so D70 item 3 still
   holds for the glyphs that a program sees. The glyphs and their positions
   are the same as a new shaping of the range, but the flags of a glyph can
   differ near a cut, because a glyph that the type takes from the paragraph
   keeps the flags of the paragraph. HarfBuzz does not promise the same flags
   either: its own checks of a split ignore them.
3. The type gives the break opportunities of the paragraph, from break
   attributes that it computes once. A program that finds the breaks of
   each line asks the type, not the segmenter.
4. The type reuses its buffers between paragraphs, as D72 does for the
   line wrapper.
5. The new code is okage's own, in `shaping`. It does not change the code of
   go-text in the other packages. Each part has a comment that starts with
   `okage:` and names this decision.
6. A test compares the glyphs that the type gives for each range with a new
   shaping of that range, on the texts of `test/textbench`, on random cuts,
   and on texts of Arabic, Devanagari and emoji, where many cuts are not
   safe to break. A fuzz target compares them on random texts and cuts.
7. `test/textbench` measures the type with a benchmark of the path of ivy
   that uses it. The result goes in the commit message.

## When upstream changes

If go-text adds a type that shapes a paragraph once, or a cache of break
attributes for a paragraph, follow these steps:

1. Apply the commit of go-text as D52 says.
2. If it gives the same glyphs and the same breaks, make the type of this
   decision use it, or remove the type and tell ivy.
3. If it does not give the same result, a maintainer decides which one okage
   keeps.

## Why

A shaped paragraph is the unit that both costs share, and HarfBuzz gives the
flags that make it safe to reuse its glyphs. One type in `shaping` lets each
program that breaks lines use it, and keeps the code of go-text as it is.
