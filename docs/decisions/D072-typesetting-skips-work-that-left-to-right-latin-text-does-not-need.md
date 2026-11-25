# D72. typesetting skips work that left-to-right Latin text does not need

Status: Decided, amended by D81 and D92.

ivy lays out books with `typesetting`, the copy of go-text in okage (D52).
On 2026-11-20 a profile of ivy put 23% to 26% of the time of its layout in
`typesetting`, spread over the lookups of Unicode properties, the line
breaker, the bidi algorithm and the shaper ("typesetting" in
`docs/BACKLOG.md`). Most of these books hold only left-to-right text, and
most of their characters are below U+0100. The maintainers want the copy to
skip the work that such text does not need, with the same results.

## The decision

1. `typesetting` first applies the pending commits of go-text (D52). The
   changes of this decision come after them, and are measured with the
   benchmarks of `test/textbench` before and after each change.
2. The bidi algorithm is skipped for a paragraph that holds no character of
   the bidi classes R, AL or AN, and no bidi control (the classes LRE, RLE,
   LRO, RLO, PDF, LRI, RLI, FSI and PDI), when the caller does not ask for a
   right-to-left base direction. Such a paragraph can hold neutral
   characters, such as quotes and punctuation, which the shortcut of go-text
   does not allow. The result is
   the result that the full algorithm gives for such a paragraph: one run at
   level 0. A test compares both paths on the test files of the bidi
   algorithm and on random text.
3. The lookups of the general category, the bidi class, the grapheme break
   property and the script have a fast path for a rune below U+0100. The
   fast path reads a table that the same data as the general lookup makes.
   A test compares the fast path with the general lookup for each rune below
   U+0100.
4. `shaping` reuses its buffers between the lines of a paragraph and between
   the paragraphs that one `LineWrapper` wraps, where it allocates them
   again today. The normalizer of `harfbuzz` stops allocating for each call
   of the shaper. The output does not change.
5. The text is not decoded from UTF-8 once for all stages, and
   `typesetting` does not use SIMD. Both stay in "typesetting" in
   `docs/BACKLOG.md`. The first changes too much
   of the copy, and the gain of the second is small.
6. On 2026-11-20 the maintainers added two changes, after D77:
   `SplitByFace` and the split by script of `shaping` skip the work for a run
   that one face and one script cover, and `language.LookupScript` finds the
   script of a rune above U+00FF without a binary search over the whole
   table, for example with a table of the blocks of Unicode. Both give the
   same results, with a test that compares them with the old code.
7. Each change has a comment that starts with `okage:` and names this
   decision. A change at the calls of `shaping` is preferred to a change
   inside a package of go-text.
8. The output of every test of `typesetting`, `svg` and `pdf` stays the
   same.

## When upstream changes

If go-text adds a fast path that does the same work, follow these steps:

1. Apply the commit of go-text as D52 says.
2. Remove the change of this decision that the commit replaces, and keep
   the test that compares the results.
3. Run the benchmarks of okage, and record the result in the commit
   message.

## Why

These changes remove work that has no effect on the result for most text,
and each one is small enough to keep the copy close to go-text. The models
that the maintainers consulted on 2026-11-20 ranked them first among the
ideas of the backlog, after the pending commits of go-text.
